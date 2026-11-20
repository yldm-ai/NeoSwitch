package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/yldm-ai/neo-switch/internal/daemon"
	"github.com/yldm-ai/neo-switch/internal/provider"
)

// isolateBackendEnv clears every input resolveBackend reads, points the state at a temp dir, and makes agy available or not.
func isolateBackendEnv(t *testing.T, agyInstalled bool) string {
	t.Helper()
	for _, key := range []string{backendEnv, legacyACPEnv, pluginKeyEnv, pluginAntigravityEnv, "GEMINI_API_KEY", "GOOGLE_API_KEY", "GOOGLE_GENAI_USE_VERTEXAI", "GOOGLE_GENAI_USE_ENTERPRISE", "OPENAI_API_KEY", "OPENAI_MODEL", "OPENAI_BASE_URL"} {
		t.Setenv(key, "")
	}
	dir := t.TempDir()
	t.Setenv("NEOSWITCH_STATE_DIR", filepath.Join(dir, "state"))
	agy := filepath.Join(dir, "agy")
	if agyInstalled {
		if err := os.WriteFile(agy, []byte("#!/bin/sh\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("ANTIGRAVITY_CLI_PATH", agy)
	return filepath.Join(dir, "state")
}

func mustResolve(t *testing.T) string {
	t.Helper()
	kind, _, err := resolveBackend()
	if err != nil {
		t.Fatal(err)
	}
	return kind
}

func TestLoginChoiceWinsOverGeminiKeyInEnvironment(t *testing.T) {
	state := isolateBackendEnv(t, true)
	// A key exported for other tools, as many shells have.
	t.Setenv("GEMINI_API_KEY", "for-another-tool")
	if got := mustResolve(t); got != backendGemini {
		t.Fatalf("without a saved choice the key selects gemini, got %q", got)
	}
	if _, err := daemon.SaveBackend(state, backendAntigravity); err != nil {
		t.Fatal(err)
	}
	if got := mustResolve(t); got != backendAntigravity {
		t.Fatalf("after /neoswitch:login the saved choice must win, got %q", got)
	}
	t.Setenv(backendEnv, backendGemini)
	if got := mustResolve(t); got != backendGemini {
		t.Fatalf("%s must override the saved choice, got %q", backendEnv, got)
	}
}

func TestAutomaticBackend(t *testing.T) {
	isolateBackendEnv(t, true)
	if got := mustResolve(t); got != backendAntigravity {
		t.Fatalf("agy without Gemini credentials selects antigravity, got %q", got)
	}
	t.Setenv("GOOGLE_GENAI_USE_VERTEXAI", "true")
	if got := mustResolve(t); got != backendGemini {
		t.Fatalf("Vertex AI credentials select gemini, got %q", got)
	}
}

func TestNoBackendExplainsBothPaths(t *testing.T) {
	isolateBackendEnv(t, false)
	if _, _, err := resolveBackend(); !errors.Is(err, errNoBackend) {
		t.Fatalf("expected errNoBackend, got %v", err)
	}
}

func TestLegacyPluginOptionsAreHonoured(t *testing.T) {
	isolateBackendEnv(t, false)
	t.Setenv(pluginKeyEnv, "saved-in-plugin-settings")
	if got := mustResolve(t); got != backendGemini {
		t.Fatalf("a key saved by an earlier plugin version selects gemini, got %q", got)
	}
	t.Setenv(pluginAntigravityEnv, "true")
	if got := mustResolve(t); got != backendAntigravity {
		t.Fatalf("the earlier OAuth toggle selects antigravity, got %q", got)
	}
}

func TestBackendCommandSavesAndClearsChoice(t *testing.T) {
	isolateBackendEnv(t, false)
	t.Setenv("GEMINI_API_KEY", "key")
	if err := backendCommand([]string{"antigravity"}); err != nil {
		t.Fatal(err)
	}
	if got := mustResolve(t); got != backendAntigravity {
		t.Fatalf("got %q", got)
	}
	if err := backendCommand([]string{"auto"}); err != nil {
		t.Fatal(err)
	}
	if got := mustResolve(t); got != backendGemini {
		t.Fatalf("auto must clear the saved choice, got %q", got)
	}
	if err := backendCommand([]string{"anthropic"}); err == nil {
		t.Fatal("an unknown backend must be rejected")
	}
	t.Setenv(backendEnv, "anthropic")
	if _, _, err := resolveBackend(); err == nil {
		t.Fatalf("an unknown %s must be rejected", backendEnv)
	}
}

func TestOpenAIIsOptInOnly(t *testing.T) {
	isolateBackendEnv(t, true)
	t.Setenv("OPENAI_API_KEY", "sk-test-not-a-real-key")
	t.Setenv("OPENAI_MODEL", "test-model")
	if got := mustResolve(t); got != backendAntigravity {
		t.Fatalf("an OpenAI key must not change automatic selection, got %q", got)
	}
	t.Setenv("GEMINI_API_KEY", "key")
	if got := mustResolve(t); got != backendGemini {
		t.Fatalf("an OpenAI key must not change automatic selection, got %q", got)
	}
	if err := backendCommand([]string{"openai"}); err != nil {
		t.Fatal(err)
	}
	if got := mustResolve(t); got != backendOpenAI {
		t.Fatalf("`neoswitch backend openai` must select openai, got %q", got)
	}
	if err := backendCommand([]string{"auto"}); err != nil {
		t.Fatal(err)
	}
	t.Setenv(backendEnv, "OpenAI")
	if got := mustResolve(t); got != backendOpenAI {
		t.Fatalf("%s=openai must select openai, got %q", backendEnv, got)
	}
}

func TestOpenAIBackendRequiresModel(t *testing.T) {
	isolateBackendEnv(t, false)
	t.Setenv(backendEnv, backendOpenAI)
	t.Setenv("OPENAI_API_KEY", "sk-test-not-a-real-key")
	_, _, _, err := configuredBackend(context.Background())
	if err == nil || !strings.Contains(err.Error(), "OPENAI_MODEL") {
		t.Fatalf("a missing OPENAI_MODEL must be reported, got %v", err)
	}
	if strings.Contains(err.Error(), "sk-test") {
		t.Fatal("the error must not echo the key")
	}
	t.Setenv("OPENAI_MODEL", "test-model")
	_, model, label, err := configuredBackend(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if model != "test-model" || label != backendLabel(backendOpenAI, "") {
		t.Fatalf("model %q label %q", model, label)
	}
}

func TestOnRefusesWithoutBackend(t *testing.T) {
	state := isolateBackendEnv(t, false)
	project := t.TempDir()
	if err := on([]string{project}); !errors.Is(err, errNoBackend) {
		t.Fatalf("expected errNoBackend, got %v", err)
	}
	for _, path := range []string{filepath.Join(project, ".claude", "settings.local.json"), filepath.Join(state, "proxy.json")} {
		if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("%s was written: %v", path, err)
		}
	}
	if message := errNoBackend.Error(); !strings.Contains(message, "/neoswitch:login") || !strings.Contains(message, "/neoswitch:key") || strings.Contains(message, "export") {
		t.Fatalf("errNoBackend must name the slash command and no shell step: %q", message)
	}
}

func TestOnExcludesSettingsFromGit(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}
	isolateBackendEnv(t, true)
	// A global excludes file on the machine running the tests must not decide the outcome.
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	repo := t.TempDir()
	if out, err := exec.Command("git", "init", "-q", repo).CombinedOutput(); err != nil {
		t.Fatalf("git init: %v %s", err, out)
	}
	settings := filepath.Join(".claude", "settings.local.json")
	for _, project := range []string{repo, filepath.Join(repo, "sub", "app")} {
		if err := os.MkdirAll(project, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := on([]string{project}); err != nil {
			t.Fatal(err)
		}
		if !isGitIgnored(project, settings) {
			t.Fatalf("%s: settings are not ignored after on", project)
		}
		// Running on again must not add the pattern twice.
		if err := on([]string{project}); err != nil {
			t.Fatal(err)
		}
	}
	data, err := os.ReadFile(filepath.Join(repo, ".git", "info", "exclude"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"/.claude/settings.local.json", "/sub/app/.claude/settings.local.json"} {
		if n := strings.Count("\n"+string(data), "\n"+want+"\n"); n != 1 {
			t.Fatalf("exclude has %q %d times:\n%s", want, n, data)
		}
	}
}

func TestStatusAnswersInPlainLines(t *testing.T) {
	for _, tc := range []struct {
		name    string
		setup   func(t *testing.T, project string)
		want    []string
		notWant []string
	}{
		{
			name: "off without a backend",
			want: []string{"This project uses Claude.", "/neoswitch:login"},
		},
		{
			name:  "off with agy",
			setup: func(t *testing.T, project string) { installAgy(t) },
			want:  []string{"This project uses Claude.", "Run /neoswitch:on to switch it to Gemini 4 Neon."},
		},
		{
			name: "on, this session not yet",
			setup: func(t *testing.T, project string) {
				installAgy(t)
				mustOn(t, project)
			},
			want: []string{"this session still uses Claude", "Start a new Claude Code session here to use Gemini 4 Neon."},
		},
		{
			name: "on, agy removed afterwards",
			setup: func(t *testing.T, project string) {
				installAgy(t)
				mustOn(t, project)
				t.Setenv("ANTIGRAVITY_CLI_PATH", filepath.Join(t.TempDir(), "agy"))
			},
			want:    []string{"cannot reach a model", "/neoswitch:login", "/neoswitch:off"},
			notWant: []string{"Gemini 4 Neon"},
		},
		{
			name: "on, state removed afterwards",
			setup: func(t *testing.T, project string) {
				installAgy(t)
				mustOn(t, project)
				if err := os.Remove(filepath.Join(os.Getenv("NEOSWITCH_STATE_DIR"), "proxy.json")); err != nil {
					t.Fatal(err)
				}
			},
			want: []string{"earlier NeoSwitch setup", "/neoswitch:on", "/neoswitch:off"},
		},
		{
			name: "on, state damaged and another project turned on since",
			setup: func(t *testing.T, project string) {
				installAgy(t)
				mustOn(t, project)
				if err := os.WriteFile(filepath.Join(os.Getenv("NEOSWITCH_STATE_DIR"), "proxy.json"), []byte("{"), 0o600); err != nil {
					t.Fatal(err)
				}
				mustOn(t, t.TempDir())
			},
			want: []string{"earlier NeoSwitch setup", "/neoswitch:on"},
		},
		{
			name: "this session routed, service not running",
			setup: func(t *testing.T, project string) {
				installAgy(t)
				mustOn(t, project)
				state, _, _ := daemon.LoadState(os.Getenv("NEOSWITCH_STATE_DIR"))
				t.Setenv("ANTHROPIC_BASE_URL", state.BaseURL())
			},
			want: []string{"cannot answer", "Start a new Claude Code session here to restart it"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			isolateBackendEnv(t, false)
			t.Setenv("ANTHROPIC_BASE_URL", "")
			project := t.TempDir()
			if tc.setup != nil {
				tc.setup(t, project)
			}
			var out bytes.Buffer
			if err := printStatus(&out, project, false); err != nil {
				t.Fatal(err)
			}
			text := out.String()
			if lines := strings.Count(text, "\n"); lines < 2 || lines > 3 {
				t.Fatalf("want two or three lines, got:\n%s", text)
			}
			for _, word := range append([]string{"127.0.0.1", "daemon", "ACP", "-> gemini"}, tc.notWant...) {
				if strings.Contains(text, word) {
					t.Fatalf("output contains %q:\n%s", word, text)
				}
			}
			for _, want := range tc.want {
				if !strings.Contains(text, want) {
					t.Fatalf("output lacks %q:\n%s", want, text)
				}
			}
			out.Reset()
			if err := printStatus(&out, project, true); err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(out.String(), "version:  ") {
				t.Fatalf("verbose output lacks the details:\n%s", out.String())
			}
		})
	}
}

func installAgy(t *testing.T) {
	t.Helper()
	agy := filepath.Join(t.TempDir(), "agy")
	if err := os.WriteFile(agy, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("ANTIGRAVITY_CLI_PATH", agy)
}

func mustOn(t *testing.T, project string) {
	t.Helper()
	if err := on([]string{project}); err != nil {
		t.Fatal(err)
	}
}

func TestAttachNotice(t *testing.T) {
	agy := backendLabel(backendAntigravity, "")
	for _, tc := range []struct {
		name    string
		notice  string
		label   string
		info    daemon.Info
		running bool
		want    string
	}{
		{"success names the model", "", agy, daemon.Info{Backend: agy, Version: version}, true, "this session uses Gemini 4 Neon through your Google account (Antigravity sign-in). Ignore the Claude model name in the header. The first reply can take a while."},
		{"api key", "", "Gemini Developer API", daemon.Info{Backend: "Gemini Developer API", Version: version}, true, "through a Gemini API key"},
		{"earlier backend", "", "Gemini Developer API", daemon.Info{Backend: agy, Version: version}, true, "still share the earlier setup (your Google account (Antigravity sign-in))"},
		{"earlier version", "", agy, daemon.Info{Backend: agy, Version: "v0.0.1"}, true, "still use the earlier NeoSwitch version"},
		{"daemon without version", "", agy, daemon.Info{Backend: agy}, true, "Close all Claude Code windows"},
		{"gateway notice only", "NeoSwitch: gateway.", agy, daemon.Info{Backend: agy, Version: version}, true, "NeoSwitch: gateway."},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := attachNotice(tc.notice, provider.DefaultModel, tc.label, tc.info, tc.running)
			if !strings.Contains(got, tc.want) {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
			if tc.notice != "" && strings.Contains(got, "this session uses") {
				t.Fatalf("a notice must not be followed by a success claim: %q", got)
			}
		})
	}
}

func TestSessionBlockedNamesNeoSwitchOnce(t *testing.T) {
	for _, problem := range []string{errNoBackend.Error(), "the proxy stopped"} {
		message := sessionBlocked(problem)
		if !strings.HasPrefix(message, "NeoSwitch") || strings.Count(message, "NeoSwitch") != 1 {
			t.Fatalf("sessionBlocked(%q) = %q, want NeoSwitch named once at the start", problem, message)
		}
	}
}

func TestSessionStartProblemsNameTheFix(t *testing.T) {
	isolateBackendEnv(t, false)
	t.Setenv(backendEnv, backendAntigravity)
	_, _, _, err := configuredBackend(context.Background())
	message := sessionBlocked(err.Error())
	for _, want := range []string{"Antigravity CLI from Google", "/neoswitch:off"} {
		if !strings.Contains(message, want) {
			t.Fatalf("%q lacks %q", message, want)
		}
	}
	for _, jargon := range []string{"PATH", "exec", "stat ", "`neoswitch"} {
		if strings.Contains(message, jargon) {
			t.Fatalf("%q contains %q", message, jargon)
		}
	}
}

func captureStdout(t *testing.T, f func() error) string {
	t.Helper()
	read, write, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	saved := os.Stdout
	os.Stdout = write
	err = f()
	os.Stdout = saved
	_ = write.Close()
	data, _ := io.ReadAll(read)
	if err != nil {
		t.Fatalf("%v (output %s)", err, data)
	}
	return string(data)
}

func TestAttachReportsAnEarlierSetup(t *testing.T) {
	for _, tc := range []struct {
		name, gateway, want string
	}{
		{"NeoSwitch entries for an earlier address", "0", "earlier NeoSwitch setup"},
		{"a local gateway of the user", "", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			isolateBackendEnv(t, true)
			t.Setenv("ANTHROPIC_BASE_URL", "http://127.0.0.1:1")
			t.Setenv(daemon.EnvUseGateway, tc.gateway)
			project := t.TempDir()
			t.Setenv("CLAUDE_PROJECT_DIR", project)
			out := captureStdout(t, func() error { return attach([]string{"--pid", "4242"}) })
			if tc.want == "" {
				if out != "" {
					t.Fatalf("unexpected message: %s", out)
				}
				return
			}
			for _, want := range []string{`"systemMessage"`, "NeoSwitch: ", tc.want, "/neoswitch:on", "/neoswitch:off"} {
				if !strings.Contains(out, want) {
					t.Fatalf("output lacks %q: %s", want, out)
				}
			}
			// The hook must not rewrite the settings; the user picks on or off.
			if _, err := os.Stat(filepath.Join(project, ".claude")); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("attach wrote settings: %v", err)
			}
		})
	}
}

func readEnv(t *testing.T, project string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(project, ".claude", "settings.local.json"))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	var settings map[string]any
	if err := json.Unmarshal(data, &settings); err != nil {
		t.Fatal(err)
	}
	env, _ := settings["env"].(map[string]any)
	return env
}

func TestOffAllRestoresEveryProject(t *testing.T) {
	state := isolateBackendEnv(t, true)
	first, second, removed, never := t.TempDir(), t.TempDir(), filepath.Join(t.TempDir(), "gone"), t.TempDir()
	for _, project := range []string{first, second, removed} {
		if err := os.MkdirAll(project, 0o755); err != nil {
			t.Fatal(err)
		}
		mustOn(t, project)
	}
	if err := os.RemoveAll(removed); err != nil {
		t.Fatal(err)
	}
	// second keeps the address of a state that was removed since, which off must still clean up.
	if err := os.Remove(filepath.Join(state, "proxy.json")); err != nil {
		t.Fatal(err)
	}
	mustOn(t, first)
	out := captureStdout(t, func() error { return off([]string{"all", never}) })
	for _, want := range []string{first, second} {
		if !strings.Contains(out, want) {
			t.Fatalf("output lacks %s:\n%s", want, out)
		}
	}
	for _, unwanted := range []string{removed, never} {
		if strings.Contains(out, unwanted) {
			t.Fatalf("output lists %s:\n%s", unwanted, out)
		}
	}
	for _, project := range []string{first, second} {
		if env := readEnv(t, project); len(env) != 0 {
			t.Fatalf("%s still has %v", project, env)
		}
	}
	if projects, _ := daemon.Projects(state); len(projects) != 0 {
		t.Fatalf("registry not cleared: %v", projects)
	}
	if out := captureStdout(t, func() error { return off([]string{"all", never}) }); !strings.Contains(out, "was not on in any project") {
		t.Fatalf("second run: %s", out)
	}
}

func TestOffOneProject(t *testing.T) {
	state := isolateBackendEnv(t, true)
	project, other := t.TempDir(), t.TempDir()
	mustOn(t, project)
	mustOn(t, other)
	if out := captureStdout(t, func() error { return off([]string{project}) }); !strings.Contains(out, "is off for") {
		t.Fatalf("got %s", out)
	}
	if projects, _ := daemon.Projects(state); strings.Join(projects, ",") != other {
		t.Fatalf("registry = %v", projects)
	}
	if out := captureStdout(t, func() error { return off([]string{project}) }); !strings.Contains(out, "was not on") {
		t.Fatalf("got %s", out)
	}
	if err := off([]string{"some", project}); err == nil {
		t.Fatal("an unknown argument must be rejected")
	}
}

func TestSavedGeminiKey(t *testing.T) {
	state := isolateBackendEnv(t, true)
	if err := daemon.SaveGeminiAPIKey(state, "saved-key-0123456789012345678901234"); err != nil {
		t.Fatal(err)
	}
	if got := mustResolve(t); got != backendGemini {
		t.Fatalf("a saved key selects gemini without a saved choice, got %q", got)
	}
	if _, _, _, err := configuredBackend(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got := os.Getenv("GEMINI_API_KEY"); got != "saved-key-0123456789012345678901234" {
		t.Fatalf("the saved key must reach the Gemini client, got %q", got)
	}
	t.Setenv("GEMINI_API_KEY", "from-environment")
	if _, _, _, err := configuredBackend(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got := os.Getenv("GEMINI_API_KEY"); got != "from-environment" {
		t.Fatalf("the environment must win over the saved key, got %q", got)
	}
}

func TestLastSetupCommandDecidesBackend(t *testing.T) {
	state := isolateBackendEnv(t, true)
	t.Setenv("GEMINI_API_KEY", "for-another-tool")
	// /neoswitch:login saves antigravity.
	if _, err := daemon.SaveBackend(state, backendAntigravity); err != nil {
		t.Fatal(err)
	}
	if got := mustResolve(t); got != backendAntigravity {
		t.Fatalf("after login got %q", got)
	}
	// /neoswitch:key saves gemini.
	if data, status := saveGeminiKeyAndEnable("AIza0123456789012345678901234567890abcd", t.TempDir()); status != http.StatusOK || data.EnableError != "" {
		t.Fatalf("key setup = %d %+v", status, data)
	}
	if got := mustResolve(t); got != backendGemini {
		t.Fatalf("after /neoswitch:key got %q", got)
	}
	t.Setenv(backendEnv, backendOpenAI)
	if got := mustResolve(t); got != backendOpenAI {
		t.Fatalf("%s must still override, got %q", backendEnv, got)
	}
}

func TestTermsWarningPointsToKey(t *testing.T) {
	for _, want := range []string{"may suspend", "accept that risk", "/neoswitch:key"} {
		if !strings.Contains(antigravityTermsWarning, want) {
			t.Fatalf("warning lacks %q", want)
		}
	}
	if strings.Contains(strings.ToLower(antigravityTermsWarning), "default") {
		t.Fatal("the warning must not call OAuth the default")
	}
}

func TestKeyPage(t *testing.T) {
	const key = "AIzaSyTest0123456789abcdefghijklmnopqrs"
	state := isolateBackendEnv(t, true)
	project := t.TempDir()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	served := make(chan error, 1)
	urls := make(chan string, 1)
	go func() {
		served <- runKeyPage(listener, "secret-token", project, time.Minute, func(url string) { urls <- url })
	}()
	url := <-urls
	origin := "http://" + listener.Addr().String()
	post := func(target, origin, value string) (int, string) {
		t.Helper()
		req, _ := http.NewRequest(http.MethodPost, target, strings.NewReader("key="+value))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		if origin != "" {
			req.Header.Set("Origin", origin)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		return resp.StatusCode, string(body)
	}

	resp, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK || !strings.Contains(string(body), `type="password"`) || !strings.Contains(string(body), geminiKeyURL) || resp.Header.Get("Referrer-Policy") != "same-origin" {
		t.Fatalf("form = %d %v\n%s", resp.StatusCode, resp.Header, body)
	}
	for _, tc := range []struct {
		name, target, origin, value string
		status                      int
	}{
		{"wrong path", origin + "/guess", origin, key, http.StatusNotFound},
		{"no origin", url, "", key, http.StatusForbidden},
		{"other origin", url, "http://evil.example", key, http.StatusForbidden},
		{"not a key", url, origin, "hello", http.StatusBadRequest},
	} {
		if status, body := post(tc.target, tc.origin, tc.value); status != tc.status || strings.Contains(body, key) {
			t.Fatalf("%s: status %d, want %d\n%s", tc.name, status, tc.status, body)
		}
	}
	if saved, _ := daemon.LoadGeminiAPIKey(state); saved != "" {
		t.Fatal("a rejected request saved a key")
	}
	if status, page := post(url, origin, key); status != http.StatusOK || !strings.Contains(page, "/exit") || strings.Contains(page, key) {
		t.Fatalf("save = %d\n%s", status, page)
	}
	select {
	case err := <-served:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the page kept running after the key was saved")
	}
	if saved, _ := daemon.LoadGeminiAPIKey(state); saved != key {
		t.Fatalf("saved key = %q", saved)
	}
	if loaded, _, _ := daemon.LoadState(state); loaded.Backend != backendGemini {
		t.Fatalf("backend = %q", loaded.Backend)
	}
	if env := readEnv(t, project); env["ANTHROPIC_BASE_URL"] == nil {
		t.Fatalf("project not turned on: %v", env)
	}
	if projects, _ := daemon.Projects(state); len(projects) != 1 || projects[0] != project {
		t.Fatalf("registry = %v", projects)
	}
}

// A proxy that is already running keeps the key and backend it started with, so saving a key must ask for every window to close, whichever backend that proxy serves.
func TestSaveKeyAsksToCloseWindowsWhileProxyRuns(t *testing.T) {
	const key = "AIzaSyTest0123456789abcdefghijklmnopqrs"
	for _, tc := range []struct {
		name    string
		running bool
	}{
		{"no proxy", false},
		{"proxy running", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			state := isolateBackendEnv(t, true)
			if tc.running {
				proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.URL.Path != "/neoswitch/status" || r.Header.Get("Authorization") != "Bearer tok" {
						http.NotFound(w, r)
						return
					}
					_ = json.NewEncoder(w).Encode(daemon.Info{Sessions: 1, Backend: backendLabel(backendGemini, "")})
				}))
				t.Cleanup(proxy.Close)
				if err := os.MkdirAll(state, 0o700); err != nil {
					t.Fatal(err)
				}
				saved, _ := json.Marshal(daemon.State{Listen: strings.TrimPrefix(proxy.URL, "http://"), Token: "tok"})
				if err := os.WriteFile(filepath.Join(state, "proxy.json"), saved, 0o600); err != nil {
					t.Fatal(err)
				}
			}
			data, status := saveGeminiKeyAndEnable(key, t.TempDir())
			if status != http.StatusOK || data.Restart != tc.running {
				t.Fatalf("status %d, restart %v, want %v: %+v", status, data.Restart, tc.running, data)
			}
			var page bytes.Buffer
			if err := keyPageTemplate.Execute(&page, data); err != nil {
				t.Fatal(err)
			}
			if got := strings.Contains(page.String(), "Close all Claude Code windows"); got != tc.running {
				t.Fatalf("page asks to close all windows = %v, want %v\n%s", got, tc.running, page.String())
			}
			if got := strings.Contains(page.String(), "/exit"); got == tc.running {
				t.Fatalf("page mentions /exit = %v, want %v\n%s", got, !tc.running, page.String())
			}
		})
	}
}

func TestKeyPageGivesUp(t *testing.T) {
	isolateBackendEnv(t, true)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	if err := runKeyPage(listener, "t", t.TempDir(), 50*time.Millisecond, func(string) {}); err != nil {
		t.Fatal(err)
	}
}

func TestPlausibleGeminiKey(t *testing.T) {
	for key, want := range map[string]bool{
		"AIzaSyTest0123456789abcdefghijklmnopqrs": true,
		"short": false,
		"AIzaSy Test0123456789abcdefghijklmnopqrs":   false,
		"export GEMINI_API_KEY=AIzaSyTest0123456789": false,
		strings.Repeat("a", 201):                     false,
	} {
		if got := plausibleGeminiKey(key); got != want {
			t.Errorf("%q: got %v", key, got)
		}
	}
}
