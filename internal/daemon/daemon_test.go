package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestEnsureStateIsStableAndPrivate(t *testing.T) {
	dir := t.TempDir()
	first, err := EnsureState(dir)
	if err != nil {
		t.Fatal(err)
	}
	second, err := EnsureState(dir)
	if err != nil || second != first {
		t.Fatalf("state changed: %+v %+v %v", first, second, err)
	}
	info, err := os.Stat(statePath(dir))
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("state file mode = %v %v", info.Mode(), err)
	}
}

func TestEnableAndDisablePreserveOtherSettings(t *testing.T) {
	project := t.TempDir()
	s := State{Listen: "127.0.0.1:4711", Token: "tok"}
	path := settingsPath(project)
	_ = os.MkdirAll(filepath.Dir(path), 0o755)
	_ = os.WriteFile(path, []byte(`{"permissions":{"allow":["Bash(ls)"]},"env":{"FOO":"bar"}}`), 0o644)

	if err := Enable(project, s); err != nil {
		t.Fatal(err)
	}
	if on, _ := Enabled(project, s); !on {
		t.Fatal("not enabled")
	}
	var settings map[string]any
	data, _ := os.ReadFile(path)
	_ = json.Unmarshal(data, &settings)
	env := settings["env"].(map[string]any)
	if env["FOO"] != "bar" || env[envBaseURL] != "http://127.0.0.1:4711" || env[envAuthToken] != "tok" || settings["permissions"] == nil {
		t.Fatalf("settings after enable = %s", data)
	}
	// Without this, a session started by a gateway launcher keeps the launcher's ANTHROPIC_BASE_URL and bypasses the proxy.
	if env[EnvUseGateway] != "0" {
		t.Fatalf("enable must turn Claude Code gateway mode off: %s", data)
	}

	if changed, err := Disable(project, s); err != nil || !changed {
		t.Fatalf("disable = %v %v", changed, err)
	}
	data, _ = os.ReadFile(path)
	settings = map[string]any{}
	_ = json.Unmarshal(data, &settings)
	env = settings["env"].(map[string]any)
	if len(env) != 1 || env["FOO"] != "bar" || settings["permissions"] == nil {
		t.Fatalf("settings after disable = %s", data)
	}
}

func TestDisableLeavesForeignBaseURL(t *testing.T) {
	for name, content := range map[string]string{
		"remote gateway": `{"env":{"ANTHROPIC_BASE_URL":"https://gateway.example","ANTHROPIC_AUTH_TOKEN":"theirs"}}`,
		// A local gateway the user set up has a loopback URL but not NeoSwitch's second mark.
		"local gateway":         `{"env":{"ANTHROPIC_BASE_URL":"http://127.0.0.1:9000","ANTHROPIC_AUTH_TOKEN":"theirs"}}`,
		"local gateway, mode 1": `{"env":{"ANTHROPIC_BASE_URL":"http://127.0.0.1:9000","ANTHROPIC_AUTH_TOKEN":"theirs","CLAUDE_CODE_USE_GATEWAY":"1"}}`,
	} {
		t.Run(name, func(t *testing.T) {
			project := t.TempDir()
			path := settingsPath(project)
			_ = os.MkdirAll(filepath.Dir(path), 0o755)
			_ = os.WriteFile(path, []byte(content), 0o644)
			s := State{Listen: "127.0.0.1:1", Token: "tok"}
			if changed, err := Disable(project, s); err != nil || changed {
				t.Fatalf("disable = %v %v", changed, err)
			}
			if data, _ := os.ReadFile(path); string(data) != content {
				t.Fatalf("foreign gateway changed: %s", data)
			}
			if stale, err := StaleAddress(project, s); err != nil || stale != "" {
				t.Fatalf("foreign gateway reported as stale: %q %v", stale, err)
			}
		})
	}
}

func TestDisableRemovesEntriesOfAnEarlierState(t *testing.T) {
	earlier := State{Listen: "127.0.0.1:4711", Token: "old"}
	for name, current := range map[string]State{
		"new state": {Listen: "127.0.0.1:4712", Token: "new"},
		"no state":  {},
	} {
		t.Run(name, func(t *testing.T) {
			project := t.TempDir()
			if err := Enable(project, earlier); err != nil {
				t.Fatal(err)
			}
			if on, _ := Enabled(project, current); on {
				t.Fatal("an earlier address must not count as enabled")
			}
			if stale, err := StaleAddress(project, current); err != nil || stale != earlier.BaseURL() {
				t.Fatalf("stale = %q %v", stale, err)
			}
			if changed, err := Disable(project, current); err != nil || !changed {
				t.Fatalf("disable = %v %v", changed, err)
			}
			data, _ := os.ReadFile(settingsPath(project))
			if strings.Contains(string(data), "ANTHROPIC") || strings.Contains(string(data), EnvUseGateway) {
				t.Fatalf("entries left: %s", data)
			}
		})
	}
}

func TestDisableWithoutSettingsWritesNothing(t *testing.T) {
	project := t.TempDir()
	if changed, err := Disable(project, State{Listen: "127.0.0.1:1", Token: "t"}); err != nil || changed {
		t.Fatalf("disable = %v %v", changed, err)
	}
	if _, err := os.Stat(filepath.Join(project, ".claude")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("disable created settings: %v", err)
	}
}

func TestEnableRefusesInvalidSettings(t *testing.T) {
	project := t.TempDir()
	path := settingsPath(project)
	_ = os.MkdirAll(filepath.Dir(path), 0o755)
	_ = os.WriteFile(path, []byte(`{"env": `), 0o644)
	if err := Enable(project, State{Listen: "127.0.0.1:1", Token: "t"}); err == nil {
		t.Fatal("must not overwrite unparseable settings")
	}
	if data, _ := os.ReadFile(path); string(data) != `{"env": ` {
		t.Fatalf("file modified: %q", data)
	}
}

func TestInvalidSettingsNameTheLine(t *testing.T) {
	for name, tc := range map[string]struct{ content, want string }{
		"trailing comma": {"{\n  \"env\": {\n    \"A\": \"1\",\n  }\n}\n", "line 4 has a formatting mistake"},
		"truncated":      {"{\n  \"env\": ", "line 2 has a formatting mistake"},
		"not an object":  {"[1, 2]", "must hold a single JSON object"},
		"env not object": {`{"env": "x"}`, `"env" entry is not a JSON object`},
	} {
		t.Run(name, func(t *testing.T) {
			project := t.TempDir()
			path := settingsPath(project)
			_ = os.MkdirAll(filepath.Dir(path), 0o755)
			_ = os.WriteFile(path, []byte(tc.content), 0o644)
			err := Enable(project, State{Listen: "127.0.0.1:1", Token: "t"})
			if err == nil || !strings.Contains(err.Error(), tc.want) || strings.Contains(err.Error(), "invalid character") {
				t.Fatalf("got %v, want %q", err, tc.want)
			}
		})
	}
}

func TestCorruptStateIsSetAside(t *testing.T) {
	for name, content := range map[string]string{"truncated": "{", "no token": `{"listen":"127.0.0.1:1"}`} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			_ = os.WriteFile(statePath(dir), []byte(content), 0o600)
			if _, ok, err := LoadState(dir); err != nil || ok {
				t.Fatalf("damaged state must read as missing: %v %v", ok, err)
			}
			if data, err := os.ReadFile(statePath(dir) + ".bad"); err != nil || string(data) != content {
				t.Fatalf("damaged state not kept aside: %q %v", data, err)
			}
			if _, err := EnsureState(dir); err != nil {
				t.Fatal(err)
			}
			if _, ok, err := LoadState(dir); err != nil || !ok {
				t.Fatalf("new state not usable: %v %v", ok, err)
			}
		})
	}
}

func TestProjectRegistry(t *testing.T) {
	dir := t.TempDir()
	for _, project := range []string{"/a", "/b", "/a", "/c"} {
		if err := RecordProject(dir, project); err != nil {
			t.Fatal(err)
		}
	}
	if got, _ := Projects(dir); strings.Join(got, ",") != "/a,/b,/c" {
		t.Fatalf("projects = %v", got)
	}
	if err := ForgetProjects(dir, "/b", "/missing"); err != nil {
		t.Fatal(err)
	}
	if got, _ := Projects(dir); strings.Join(got, ",") != "/a,/c" {
		t.Fatalf("projects = %v", got)
	}
	_ = os.WriteFile(projectsPath(dir), []byte("{"), 0o600)
	if got, err := Projects(dir); err != nil || len(got) != 0 {
		t.Fatalf("a damaged registry must read as empty: %v %v", got, err)
	}
}

func TestAttachFailsFastWhenAnotherProgramHoldsThePort(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	s := State{Listen: listener.Addr().String(), Token: "t"}
	start := time.Now()
	err = Attach(s, 100, func() error { t.Fatal("must not spawn a daemon that cannot listen"); return nil })
	var busy *PortInUseError
	if !errors.As(err, &busy) || busy.Listen != s.Listen {
		t.Fatalf("got %v", err)
	}
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Fatalf("took %s", elapsed)
	}
}

type fakeProcs struct {
	mu    sync.Mutex
	alive map[int]bool
}

func (f *fakeProcs) Alive(pid int) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.alive[pid]
}

func (f *fakeProcs) set(pid int, alive bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.alive[pid] = alive
}

func testState(t *testing.T) State {
	t.Helper()
	s, err := EnsureState(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestDaemonServesUntilLastSessionExits(t *testing.T) {
	s := testState(t)
	procs := &fakeProcs{alive: map[int]bool{}}
	proxy := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusTeapot) })
	done := make(chan error, 1)
	spawn := func() error {
		go func() {
			done <- Run(context.Background(), s, proxy, Options{StartupGrace: time.Minute, PollInterval: 20 * time.Millisecond, Alive: procs.Alive, Version: "v9"})
		}()
		return nil
	}

	procs.set(100, true)
	procs.set(200, true)
	if err := Attach(s, 100, spawn); err != nil {
		t.Fatal(err)
	}
	if err := Attach(s, 200, func() error { t.Fatal("second attach must reuse the running daemon"); return nil }); err != nil {
		t.Fatal(err)
	}
	if info, ok := Status(s); !ok || info.Sessions != 2 || info.Version != "v9" {
		t.Fatalf("status = %+v %v", info, ok)
	}
	resp, err := http.Get(s.BaseURL() + "/v1/messages")
	if err != nil || resp.StatusCode != http.StatusTeapot {
		t.Fatalf("proxy not mounted: %v %v", resp, err)
	}
	resp.Body.Close()

	procs.set(100, false)
	time.Sleep(100 * time.Millisecond)
	if info, ok := Status(s); !ok || info.Sessions != 1 {
		t.Fatalf("after one exit: %+v %v", info, ok)
	}
	procs.set(200, false)
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("daemon kept running after the last session exited")
	}
}

func TestDaemonExitsWhenNobodyAttaches(t *testing.T) {
	s := testState(t)
	err := Run(context.Background(), s, http.NotFoundHandler(), Options{StartupGrace: 50 * time.Millisecond, PollInterval: 10 * time.Millisecond, Alive: func(int) bool { return true }})
	if err != nil {
		t.Fatal(err)
	}
}

func TestControlEndpointsRequireToken(t *testing.T) {
	s := testState(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = Run(ctx, s, http.NotFoundHandler(), Options{StartupGrace: time.Minute}) }()
	deadline := time.Now().Add(3 * time.Second)
	for {
		if _, ok := Status(s); ok {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("daemon did not start")
		}
		time.Sleep(20 * time.Millisecond)
	}
	if _, ok := Status(State{Listen: s.Listen, Token: "wrong"}); ok {
		t.Fatal("status answered without the token")
	}
	req, _ := http.NewRequest(http.MethodPost, s.BaseURL()+attachPath, strings.NewReader(`{"pid":4242}`))
	req.Header.Set("Authorization", "Bearer wrong")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("attach with wrong token = %d", resp.StatusCode)
	}
	if info, _ := Status(s); info.Sessions != 0 {
		t.Fatalf("unauthorized attach registered a session: %+v", info)
	}
}

func TestProcessAlive(t *testing.T) {
	if !processAlive(os.Getpid()) {
		t.Fatal("own process reported dead")
	}
	if processAlive(1 << 30) {
		t.Fatal("nonexistent pid reported alive")
	}
}

func TestSaveBackendKeepsAddressAndToken(t *testing.T) {
	dir := t.TempDir()
	first, err := EnsureState(dir)
	if err != nil {
		t.Fatal(err)
	}
	saved, err := SaveBackend(dir, "antigravity")
	if err != nil {
		t.Fatal(err)
	}
	// Enabled projects store the address and token, so recording a choice must not change them.
	if saved.Listen != first.Listen || saved.Token != first.Token {
		t.Fatalf("state changed: %+v -> %+v", first, saved)
	}
	loaded, ok, err := LoadState(dir)
	if err != nil || !ok || loaded.Backend != "antigravity" {
		t.Fatalf("loaded %+v %v %v", loaded, ok, err)
	}
	if _, err := SaveBackend(dir, ""); err != nil {
		t.Fatal(err)
	}
	if loaded, _, _ = LoadState(dir); loaded.Backend != "" {
		t.Fatalf("choice not cleared: %+v", loaded)
	}
}
