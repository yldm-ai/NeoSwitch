package main

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/yldm-ai/neo-switch/internal/provider"
)

// fakeInstaller serves handler as Google's installer over local TLS and returns the isolated HOME and a download counter. HOME, PATH and the state directory are isolated so the real agy, the real installer and the user's shell profiles are never touched.
func fakeInstaller(t *testing.T, handler http.HandlerFunc) (home string, downloads *atomic.Int32) {
	t.Helper()
	downloads = new(atomic.Int32)
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		downloads.Add(1)
		handler(w, r)
	}))
	t.Cleanup(server.Close)
	client := server.Client()
	client.CheckRedirect = httpsOnlyRedirects
	previousURL, previousClient := antigravityInstallerURL, antigravityInstallerClient
	antigravityInstallerURL, antigravityInstallerClient = server.URL+"/install.sh", client
	t.Cleanup(func() { antigravityInstallerURL, antigravityInstallerClient = previousURL, previousClient })
	home = t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("NEOSWITCH_STATE_DIR", filepath.Join(home, "state"))
	t.Setenv("ANTIGRAVITY_CLI_PATH", "")
	// bash must stay reachable; agy must not be.
	t.Setenv("PATH", "/usr/bin:/bin")
	if _, err := provider.FindAntigravityCLI(); err == nil {
		t.Skip("an agy in /usr/bin or /bin would hide the install")
	}
	return home, downloads
}

func serveScript(script string) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(script)) }
}

// placeAgy is the part of a fake installer that puts a working agy, one that answers --version, where Google's installer does.
const placeAgy = `mkdir -p "$HOME/.local/bin"
printf '#!/bin/sh\necho 1.0.0\n' > "$HOME/.local/bin/agy"
chmod +x "$HOME/.local/bin/agy"
`

const fakeInstallScript = "#!/bin/bash\nset -eu\n" + placeAgy

func TestInstallAntigravityCLIRunsGooglesInstaller(t *testing.T) {
	home, downloads := fakeInstaller(t, serveScript(fakeInstallScript))
	path, err := installAntigravityCLI(context.Background())
	want := filepath.Join(home, ".local", "bin", "agy")
	if err != nil || path != want || downloads.Load() != 1 {
		t.Fatalf("installAntigravityCLI() = %q, %v after %d downloads; want %q", path, err, downloads.Load(), want)
	}
}

func TestInstallAntigravityCLIFailuresNameTheManualStep(t *testing.T) {
	for _, tc := range []struct {
		name    string
		handler http.HandlerFunc
	}{
		{"download refused", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNotFound) }},
		{"captive portal page", serveScript("<html>sign in to the Wi-Fi</html>")},
		{"oversized download", serveScript("#!/bin/bash\n" + strings.Repeat("#", maxInstallerSize))},
		{"redirect to plain HTTP", func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, "http://example.invalid/install.sh", http.StatusFound)
		}},
		{"installer fails", serveScript("#!/bin/bash\nexit 3\n")},
		{"installer puts agy elsewhere", serveScript("#!/bin/bash\nexit 0\n")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			home, _ := fakeInstaller(t, tc.handler)
			_, err := installAntigravityCLI(context.Background())
			if !errors.Is(err, provider.ErrAntigravityCLIMissing) || !strings.Contains(err.Error(), "curl -fsSL https://antigravity.google/cli/install.sh | bash") || !strings.Contains(err.Error(), "/neoswitch:login") {
				t.Fatalf("got %v", err)
			}
			if _, statErr := os.Stat(filepath.Join(home, ".local", "bin", "agy")); !os.IsNotExist(statErr) {
				t.Fatalf("agy exists after a failed install: %v", statErr)
			}
		})
	}
}

func TestInstallerRefusesPlainHTTPURL(t *testing.T) {
	fakeInstaller(t, serveScript(fakeInstallScript))
	antigravityInstallerURL = strings.Replace(antigravityInstallerURL, "https://", "http://", 1)
	if _, err := installAntigravityCLI(context.Background()); err == nil || !strings.Contains(err.Error(), "non-HTTPS") {
		t.Fatalf("got %v", err)
	}
}

// The downloaded script sees only the allowlisted environment: no credentials, whatever they are called.
func TestInstallerRunsWithoutCredentials(t *testing.T) {
	home, _ := fakeInstaller(t, serveScript(`#!/bin/bash
set -eu
env > "$HOME/installer-env"
`+placeAgy))
	t.Setenv("ANTHROPIC_AUTH_TOKEN", "leak-1")
	t.Setenv("CLAUDE_CODE_OAUTH_TOKEN", "leak-2")
	t.Setenv("GEMINI_API_KEY", "leak-3")
	t.Setenv("MYSQL_PWD", "leak-4")
	t.Setenv("DATABASE_URL", "postgres://u:leak-5@db/x")
	t.Setenv("HTTPS_PROXY", "http://proxy.invalid:3128")
	t.Setenv("LC_ALL", "C")
	if _, err := installAntigravityCLI(context.Background()); err != nil {
		t.Fatal(err)
	}
	env, err := os.ReadFile(filepath.Join(home, "installer-env"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(env), "leak-") {
		t.Fatalf("installer saw a credential:\n%s", env)
	}
	for _, want := range []string{"HTTPS_PROXY=http://proxy.invalid:3128", "HOME=" + home, "LC_ALL=C", "PATH="} {
		if !strings.Contains(string(env), want) {
			t.Fatalf("installer lost %q:\n%s", want, env)
		}
	}
}

// Google's installer treats any file at its target as installed, so a failed run must not leave one behind.
func TestFailedInstallLeavesNoAgy(t *testing.T) {
	for _, tc := range []struct{ name, script string }{
		{"installer fails after placing agy", "#!/bin/bash\n" + placeAgy + "exit 3\n"},
		{"placed agy does not run", "#!/bin/bash\nmkdir -p \"$HOME/.local/bin\"\nprintf '#!/bin/sh\\nexit 1\\n' > \"$HOME/.local/bin/agy\"\nchmod +x \"$HOME/.local/bin/agy\"\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			home, _ := fakeInstaller(t, serveScript(tc.script))
			if _, err := installAntigravityCLI(context.Background()); !errors.Is(err, provider.ErrAntigravityCLIMissing) {
				t.Fatalf("got %v", err)
			}
			if _, err := os.Lstat(filepath.Join(home, ".local", "bin", "agy")); !os.IsNotExist(err) {
				t.Fatalf("agy left behind: %v", err)
			}
		})
	}
}

// A broken agy that was there before is the user's file: NeoSwitch names it instead of deleting it.
func TestBrokenExistingAgyIsNamedNotDeleted(t *testing.T) {
	home, _ := fakeInstaller(t, serveScript("#!/bin/bash\nexit 0\n"))
	target := filepath.Join(home, ".local", "bin", "agy")
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte("half a binary"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := installAntigravityCLI(context.Background())
	if err == nil || !strings.Contains(err.Error(), target) || !strings.Contains(err.Error(), "delete it") {
		t.Fatalf("got %v", err)
	}
	if _, err := os.Stat(target); err != nil {
		t.Fatalf("existing agy was removed: %v", err)
	}
}

func TestInstallerProfileEditsAreReported(t *testing.T) {
	home, _ := fakeInstaller(t, serveScript(fakeInstallScript+`echo 'export PATH="$HOME/.local/bin:$PATH"' >> "$HOME/.zshrc"
`))
	if err := os.WriteFile(filepath.Join(home, ".bashrc"), []byte("# untouched\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	before := snapshotProfiles(home)
	if _, err := installAntigravityCLI(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got, want := changedProfiles(home, before), []string{filepath.Join(home, ".zshrc")}; !slices.Equal(got, want) {
		t.Fatalf("changedProfiles = %v, want %v", got, want)
	}
}

// A stuck installer is stopped with everything it started, not left downloading in the background.
func TestInstallerTimeoutStopsItsChildren(t *testing.T) {
	testInstallerStopsItsChildren(t, func() context.Context {
		previous := antigravityInstallTimeout
		antigravityInstallTimeout = time.Second
		t.Cleanup(func() { antigravityInstallTimeout = previous })
		return context.Background()
	}, "did not finish")
}

// Cancelling the login, as Ctrl-C or SIGTERM do through the signal context, stops the installer the same way.
func TestCancelledInstallStopsItsChildren(t *testing.T) {
	testInstallerStopsItsChildren(t, func() context.Context {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		t.Cleanup(cancel)
		return ctx
	}, "stopped before it finished")
}

func testInstallerStopsItsChildren(t *testing.T, setup func() context.Context, want string) {
	home, _ := fakeInstaller(t, serveScript(`#!/bin/bash
sleep 30 &
echo $! > "$HOME/child.pid"
wait
`))
	ctx := setup()
	start := time.Now()
	_, err := installAntigravityCLI(ctx)
	if err == nil || !strings.Contains(err.Error(), want) || time.Since(start) > 15*time.Second {
		t.Fatalf("got %v after %v", err, time.Since(start))
	}
	pid, err := os.ReadFile(filepath.Join(home, "child.pid"))
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for exec.Command("kill", "-0", strings.TrimSpace(string(pid))).Run() == nil {
		if time.Now().After(deadline) {
			t.Fatalf("installer child %s still runs", strings.TrimSpace(string(pid)))
		}
		time.Sleep(50 * time.Millisecond)
	}
}

// Two logins at once run the installer once: the second waits for the first and finds agy.
func TestConcurrentInstallsRunTheInstallerOnce(t *testing.T) {
	home, _ := fakeInstaller(t, serveScript(`#!/bin/bash
set -eu
echo run >> "$HOME/runs"
sleep 1
`+placeAgy))
	var wg sync.WaitGroup
	errs := make([]error, 2)
	for i := range errs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, errs[i] = installAntigravityCLI(context.Background())
		}()
	}
	wg.Wait()
	if errs[0] != nil || errs[1] != nil {
		t.Fatalf("got %v", errs)
	}
	runs, err := os.ReadFile(filepath.Join(home, "runs"))
	if err != nil || strings.Count(string(runs), "run") != 1 {
		t.Fatalf("installer ran %q times (%v)", runs, err)
	}
}

// An installer process that outlives NeoSwitch, for example after Claude Code killed the command, keeps holding the lock, so a retried login cannot start a second installer next to it.
func TestInstallerProcessesKeepTheLock(t *testing.T) {
	fakeInstaller(t, serveScript("#!/bin/bash\nsleep 2 &\nexit 3\n"))
	if _, err := installAntigravityCLI(context.Background()); err == nil {
		t.Fatal("first install should fail")
	}
	start := time.Now()
	_, _ = installAntigravityCLI(context.Background())
	if waited := time.Since(start); waited < time.Second {
		t.Fatalf("second install started after %v, while the first installer's child still ran", waited)
	}
}

// An ANTIGRAVITY_CLI_PATH that names a missing agy is the user's own setup: login reports it and installs nothing.
func TestLoginDoesNotInstallOverAnExplicitPath(t *testing.T) {
	home, downloads := fakeInstaller(t, serveScript(fakeInstallScript))
	t.Setenv("ANTIGRAVITY_CLI_PATH", filepath.Join(t.TempDir(), "agy"))
	err := antigravityLogin(nil)
	if !errors.Is(err, provider.ErrAntigravityCLIMissing) || !strings.Contains(err.Error(), "ANTIGRAVITY_CLI_PATH") || downloads.Load() != 0 {
		t.Fatalf("got %v after %d downloads", err, downloads.Load())
	}
	if _, statErr := os.Stat(filepath.Join(home, ".local", "bin", "agy")); !os.IsNotExist(statErr) {
		t.Fatalf("agy was installed: %v", statErr)
	}
}
