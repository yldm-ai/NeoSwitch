package daemon

import (
	"context"
	"encoding/json"
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

	if err := Disable(project, s); err != nil {
		t.Fatal(err)
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
	project := t.TempDir()
	path := settingsPath(project)
	_ = os.MkdirAll(filepath.Dir(path), 0o755)
	_ = os.WriteFile(path, []byte(`{"env":{"ANTHROPIC_BASE_URL":"https://gateway.example","ANTHROPIC_AUTH_TOKEN":"theirs"}}`), 0o644)
	if err := Disable(project, State{Listen: "127.0.0.1:1", Token: "tok"}); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	if string(data) == "" || !json.Valid(data) {
		t.Fatal("settings destroyed")
	}
	var settings map[string]map[string]string
	_ = json.Unmarshal(data, &settings)
	if settings["env"]["ANTHROPIC_BASE_URL"] != "https://gateway.example" || settings["env"]["ANTHROPIC_AUTH_TOKEN"] != "theirs" {
		t.Fatalf("foreign gateway removed: %s", data)
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
			done <- Run(context.Background(), s, proxy, Options{StartupGrace: time.Minute, PollInterval: 20 * time.Millisecond, Alive: procs.Alive})
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
	if info, ok := Status(s); !ok || info.Sessions != 2 {
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
