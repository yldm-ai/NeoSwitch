package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"html/template"
	"net"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/yldm-ai/neo-switch/internal/daemon"
)

const (
	// keyPageTimeout is how long the key page waits for the user before it gives up.
	keyPageTimeout = 10 * time.Minute
	// keyPageTokenEnv passes the page's secret path to the page server. The environment, unlike the command line, is not visible to other users of the computer.
	keyPageTokenEnv = "NEOSWITCH_KEY_PAGE_TOKEN"
	// geminiKeyURL is where Google AI Studio creates Gemini API keys.
	geminiKeyURL = "https://aistudio.google.com/apikey"
)

// openURL opens url in the default browser. Tests replace it so no browser starts.
var openURL = func(url string) error {
	var name string
	switch runtime.GOOS {
	case "darwin":
		name = "open"
	case "linux":
		name = "xdg-open"
	default:
		return errors.ErrUnsupported
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return exec.CommandContext(ctx, name, url).Run()
}

// keyCommand lets the user enter a Gemini API key in a one-time page on this computer, so the key never passes through Claude Code's chat or a model. The page is served by a detached child process: the /neoswitch:key skill shows its output only once the command exits, and the user needs that output, the page address, while the page is open.
func keyCommand(args []string) error {
	if len(args) > 0 && args[0] == "--serve" {
		return serveKeyPage(args[1:])
	}
	if len(args) > 1 {
		return errors.New("usage: neoswitch key [project]")
	}
	project, err := projectDir(args)
	if err != nil {
		return err
	}
	token, err := randomToken()
	if err != nil {
		return err
	}
	self, err := os.Executable()
	if err != nil {
		return err
	}
	cmd := exec.Command(self, "key", "--serve", project)
	cmd.Env = append(os.Environ(), keyPageTokenEnv+"="+token)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	detach(cmd)
	if err := cmd.Start(); err != nil {
		return err
	}
	lines := make(chan string, 1)
	go func() {
		line, _ := bufio.NewReader(stdout).ReadString('\n')
		lines <- strings.TrimSpace(line)
	}()
	var line string
	select {
	case line = <-lines:
	case <-time.After(10 * time.Second):
		_ = cmd.Process.Kill()
	}
	_ = cmd.Process.Release()
	url, ok := strings.CutPrefix(line, "ready ")
	if !ok {
		reason := strings.TrimPrefix(line, "error ")
		if reason == "" {
			reason = "it did not start"
		}
		return fmt.Errorf("NeoSwitch could not open the page for your Gemini API key (%s). Try /neoswitch:key again", reason)
	}
	if err := openURL(url); err != nil {
		fmt.Println("Open this address in your browser to paste your Gemini API key:")
	} else {
		fmt.Println("A page opened in your browser where you can paste your Gemini API key. If it did not open, open this address in your browser:")
	}
	fmt.Println(url)
	fmt.Println("The key goes from that page straight to NeoSwitch on this computer, not through the chat. The page closes after 10 minutes.")
	if proxyRunning() {
		fmt.Println("After you save the key, close all Claude Code windows and open them again. While any of them is open, NeoSwitch keeps the earlier setup.")
	} else {
		fmt.Println("After you save the key, type /exit and start claude again.")
	}
	return nil
}

// proxyRunning reports whether the shared NeoSwitch proxy is running. It reads the backend and the Gemini key once at start and keeps them until every Claude Code window using it has closed, so a saved key reaches no session before then.
func proxyRunning() bool {
	dir, err := daemon.Dir()
	if err != nil {
		return false
	}
	state, ok, err := daemon.LoadState(dir)
	if err != nil || !ok {
		return false
	}
	_, running := daemon.Status(state)
	return running
}

// serveKeyPage is the detached page server started by keyCommand. It reports one line on stdout, "ready <url>" or "error <reason>", and then writes nothing more there, because keyCommand has stopped reading.
func serveKeyPage(args []string) error {
	report := func(line string) {
		fmt.Println(line)
		_ = os.Stdout.Close()
	}
	token := os.Getenv(keyPageTokenEnv)
	if len(args) != 1 || token == "" {
		report("error the page server was started without its project or token")
		return nil
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		report("error no free port on this computer")
		return nil
	}
	return runKeyPage(listener, token, args[0], keyPageTimeout, func(url string) { report("ready " + url) })
}

// runKeyPage serves the key page on listener until a key was saved or timeout passed. ready receives the page address once it is served.
func runKeyPage(listener net.Listener, token, project string, timeout time.Duration, ready func(url string)) error {
	page := &keyPage{addr: listener.Addr().String(), token: token, project: project, done: make(chan struct{})}
	server := &http.Server{Handler: page, ReadHeaderTimeout: 10 * time.Second}
	served := make(chan error, 1)
	go func() { served <- server.Serve(listener) }()
	ready(page.url())
	select {
	case err := <-served:
		return err
	case <-page.done:
	case <-time.After(timeout):
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	// Shutdown lets the request that saved the key finish its success page.
	return server.Shutdown(ctx)
}

// keyPage serves the form at a secret path and accepts one key posted from it.
type keyPage struct {
	addr    string
	token   string
	project string
	done    chan struct{}

	mu    sync.Mutex
	saved bool
}

func (p *keyPage) url() string { return "http://" + p.addr + "/" + p.token }

func (p *keyPage) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Another site's page can reach this port through a DNS name that resolves to 127.0.0.1; its requests carry that name as Host.
	if r.Host != p.addr || r.URL.Path != "/"+p.token {
		http.NotFound(w, r)
		return
	}
	header := w.Header()
	header.Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; form-action 'self'; frame-ancestors 'none'")
	// The page address is its secret, so the link to Google AI Studio must not send it along. same-origin rather than no-referrer: under no-referrer browsers send "Origin: null" with the form's POST, which post() rejects.
	header.Set("Referrer-Policy", "same-origin")
	header.Set("Cache-Control", "no-store")
	header.Set("Content-Type", "text/html; charset=utf-8")
	switch r.Method {
	case http.MethodGet:
		p.render(w, http.StatusOK, keyPageData{Project: p.project})
	case http.MethodPost:
		p.post(w, r)
	default:
		header.Set("Allow", "GET, POST")
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (p *keyPage) post(w http.ResponseWriter, r *http.Request) {
	// Browsers send Origin with every form POST; only this page itself may post the key.
	if r.Header.Get("Origin") != "http://"+p.addr {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte("This address only accepts a key typed into the NeoSwitch page."))
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 4<<10)
	if err := r.ParseForm(); err != nil {
		p.render(w, http.StatusBadRequest, keyPageData{Project: p.project, Error: "That did not look like a Gemini API key. Copy the whole key and try again."})
		return
	}
	key := strings.TrimSpace(r.PostForm.Get("key"))
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.saved {
		p.render(w, http.StatusGone, keyPageData{Project: p.project, Saved: true})
		return
	}
	if !plausibleGeminiKey(key) {
		p.render(w, http.StatusBadRequest, keyPageData{Project: p.project, Error: "That did not look like a Gemini API key. Copy the whole key and try again."})
		return
	}
	data, status := saveGeminiKeyAndEnable(key, p.project)
	if status == http.StatusOK {
		p.saved = true
		defer close(p.done)
	}
	p.render(w, status, data)
}

// saveGeminiKeyAndEnable saves the key, makes the Gemini API the backend, as signing in makes Antigravity the backend, and turns the project on like /neoswitch:on.
func saveGeminiKeyAndEnable(key, project string) (keyPageData, int) {
	data := keyPageData{Project: project}
	dir, err := daemon.Dir()
	if err == nil {
		err = daemon.SaveGeminiAPIKey(dir, key)
	}
	if err == nil {
		_, err = daemon.SaveBackend(dir, backendGemini)
	}
	if err != nil {
		data.Error = "NeoSwitch could not save the key on this computer (" + err.Error() + "). Try again, or run /neoswitch:key again in Claude Code."
		return data, http.StatusInternalServerError
	}
	data.Saved = true
	// A running proxy keeps the earlier key or backend, whichever it served, so the new key needs every window closed rather than one /exit.
	data.Restart = proxyRunning()
	if data.GitNote, err = enableProject(project); err != nil {
		data.EnableError = sentence(err.Error())
	}
	return data, http.StatusOK
}

// plausibleGeminiKey checks the shape of a key loosely, to catch pasting the wrong text, without rejecting key formats Google may introduce.
func plausibleGeminiKey(key string) bool {
	if len(key) < 30 || len(key) > 200 {
		return false
	}
	for _, c := range key {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-' || c == '_') {
			return false
		}
	}
	return true
}

type keyPageData struct {
	Project     string
	Error       string
	Saved       bool
	EnableError string
	GitNote     string
	KeyURL      string
	// Restart means a running proxy still serves the earlier setup, so every Claude Code window must close before the key is used.
	Restart bool
}

func (p *keyPage) render(w http.ResponseWriter, status int, data keyPageData) {
	data.KeyURL = geminiKeyURL
	w.WriteHeader(status)
	_ = keyPageTemplate.Execute(w, data)
}

var keyPageTemplate = template.Must(template.New("key").Parse(`<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>NeoSwitch: Gemini API key</title>
<style>
body { font: 16px/1.5 system-ui, sans-serif; max-width: 34rem; margin: 3rem auto; padding: 0 1rem; color: #1d1d1f; }
h1 { font-size: 1.5rem; }
label { display: block; font-weight: 600; margin: 1.5rem 0 .4rem; }
input { width: 100%; box-sizing: border-box; font: inherit; padding: .5rem; border: 1px solid #888; border-radius: 6px; }
button { margin-top: 1rem; font: inherit; padding: .5rem 1.2rem; border: 0; border-radius: 6px; background: #1a73e8; color: #fff; cursor: pointer; }
.error { color: #b3261e; }
code { background: #f1f1f1; padding: 0 .3rem; border-radius: 4px; }
@media (prefers-color-scheme: dark) { body { background: #1d1d1f; color: #f5f5f7; } code { background: #333; } a { color: #8ab4f8; } .error { color: #f28b82; } }
</style>
</head>
<body>
{{if .Saved}}
<h1>Key saved</h1>
{{if .EnableError}}
<p class="error" role="alert">NeoSwitch could not switch {{.Project}} to Gemini 4 Neon: {{.EnableError}}</p>
<p>After fixing that, run <code>/neoswitch:on</code> in Claude Code.</p>
{{else}}
{{if .Restart}}
<p>Claude Code in {{.Project}} uses Gemini 4 Neon with this key once every open Claude Code window has closed. Until then NeoSwitch keeps the earlier setup.</p>
<p>Close all Claude Code windows, then start <code>claude</code> again. You can close this page.</p>
{{else}}
<p>Claude Code in {{.Project}} now uses Gemini 4 Neon with your Gemini API key, starting with its next session.</p>
<p>In Claude Code, type <code>/exit</code>, then start <code>claude</code> again. You can close this page.</p>
{{end}}
{{if .GitNote}}<p>{{.GitNote}}</p>{{end}}
{{end}}
{{else}}
<h1>Use a Gemini API key</h1>
<p>Paste your Gemini API key to use Gemini 4 Neon in Claude Code for {{.Project}}. NeoSwitch keeps the key on this computer only; it does not go through the chat.</p>
<p>No key yet? <a href="{{.KeyURL}}" target="_blank" rel="noopener noreferrer">Create one in Google AI Studio</a>, then copy it here.</p>
<form method="post">
<label for="key">Gemini API key</label>
<input id="key" name="key" type="password" autocomplete="off" spellcheck="false" required autofocus>
{{if .Error}}<p class="error" role="alert">{{.Error}}</p>{{end}}
<button type="submit">Save key</button>
</form>
{{end}}
</body>
</html>
`))
