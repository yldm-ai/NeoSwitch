package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"syscall"
	"time"

	"github.com/yldm-ai/neo-switch/internal/daemon"
	"github.com/yldm-ai/neo-switch/internal/provider"
)

// antigravityInstallerURL is Google's official agy installer for macOS and Linux, the one https://antigravity.google/download gives. It installs agy into ~/.local/bin and adds that directory to the shell profiles.
var antigravityInstallerURL = "https://antigravity.google/cli/install.sh"

// antigravityInstallerClient fetches the installer. Every hop must be HTTPS, so the script that runs always comes from a server that proved it is Google's; tests swap in a client that trusts their local TLS server.
var antigravityInstallerClient = &http.Client{
	Timeout:       time.Minute,
	CheckRedirect: httpsOnlyRedirects,
}

// antigravityInstallTimeout bounds one install, counted from when it holds the install lock; the agy download is over 200 MB.
var antigravityInstallTimeout = 10 * time.Minute

// maxInstallerSize caps the script download. Google's installer is under 10 KB; anything near this size is not that script.
const maxInstallerSize = 1 << 20

// shellProfiles are the files, relative to HOME, that Google's installer may add its PATH line to. NeoSwitch does not edit them; it reports which ones changed so the installer's edit is never silent. A directory entry counts as changed when a file in it is added or removed.
var shellProfiles = []string{".zshrc", ".zprofile", ".zshenv", ".bashrc", ".bash_profile", ".bash_login", ".profile", ".config/fish/config.fish", ".config/fish/conf.d"}

// installerEnvNames are the only variables the downloaded installer sees: what it needs to find the home directory, the shell and its profiles, temporary space, the network through a proxy, and custom CA certificates. Everything else, credentials above all, stays out, whatever it is called.
var installerEnvNames = []string{
	"HOME", "USER", "LOGNAME", "SHELL", "PATH", "TMPDIR", "LANG", "TERM", "ZDOTDIR",
	"HTTP_PROXY", "HTTPS_PROXY", "NO_PROXY", "ALL_PROXY", "http_proxy", "https_proxy", "no_proxy", "all_proxy",
	"SSL_CERT_FILE", "SSL_CERT_DIR", "CURL_CA_BUNDLE",
}

// antigravityManualInstall is the step a user takes when NeoSwitch cannot install agy itself.
func antigravityManualInstall() string {
	if runtime.GOOS == "darwin" || runtime.GOOS == "linux" {
		return "install it with `curl -fsSL https://antigravity.google/cli/install.sh | bash` (or from https://antigravity.google/download), then run /neoswitch:login again"
	}
	return "install it from https://antigravity.google/download, then run /neoswitch:login again"
}

func httpsOnlyRedirects(request *http.Request, via []*http.Request) error {
	if len(via) >= 5 {
		return errors.New("too many redirects")
	}
	if request.URL.Scheme != "https" {
		return fmt.Errorf("refused a redirect to non-HTTPS %s", request.URL.Redacted())
	}
	return nil
}

// installAntigravityCLI runs Google's official installer and returns the installed agy. Signing in through Antigravity needs agy, and the user who runs /neoswitch:login has asked for exactly that, so a missing agy is installed instead of sending the user away to install it.
//
// Safety and robustness:
//   - The script is fetched over HTTPS only, downloaded whole before it runs (a cut connection never runs half a script), size- and shape-checked, and kept in a private temporary directory.
//   - It runs without stdin or a controlling terminal, so it can never wait on a prompt, and with an allowlisted environment that carries no credentials.
//   - One install runs at a time across processes. The installer's processes inherit the lock, so even if NeoSwitch is killed, no second installer starts while the first one still runs: both would write Google's single staging file.
//   - A timeout, Ctrl-C or SIGTERM stops the installer together with its child processes, and an agy left behind by a failed run is removed, because the installer treats any file at its target as already installed.
//   - The new agy must run before it is accepted, and every shell profile the installer changed is reported on stdout.
func installAntigravityCLI(ctx context.Context) (string, error) {
	target := provider.AntigravityCLIInstallPath()
	bash, err := exec.LookPath("bash")
	if target == "" || err != nil || !installSupported {
		return "", fmt.Errorf("%w and NeoSwitch cannot install it on this system: %s", provider.ErrAntigravityCLIMissing, antigravityManualInstall())
	}
	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)
	defer stop()

	stateDir, err := daemon.Dir()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(stateDir, 0o700); err != nil {
		return "", err
	}
	lock, err := lockFile(ctx, filepath.Join(stateDir, "agy-install.lock"))
	if err != nil {
		return "", fmt.Errorf("%w and NeoSwitch could not start the install (%v): %s", provider.ErrAntigravityCLIMissing, err, antigravityManualInstall())
	}
	defer lock.Close()
	// Another /neoswitch:login may have finished the install while this one waited for the lock.
	if path, err := provider.FindAntigravityCLI(); err == nil {
		return path, nil
	}
	stopped := ctx
	ctx, cancel := context.WithTimeout(ctx, antigravityInstallTimeout)
	defer cancel()

	fmt.Fprintf(os.Stderr, "NeoSwitch: Google's Antigravity command-line tool (agy) is not installed. Installing it with Google's official installer into %s (a download of about 230 MB) ...\n", filepath.Dir(target))
	dir, err := os.MkdirTemp("", "neoswitch-agy-install-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(dir)
	script := filepath.Join(dir, "install.sh")
	if err := downloadInstaller(ctx, script); err != nil {
		return "", fmt.Errorf("%w and NeoSwitch could not download Google's installer (%v): check the network, or %s", provider.ErrAntigravityCLIMissing, err, antigravityManualInstall())
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	_, statErr := os.Lstat(target)
	targetExisted := statErr == nil
	before := snapshotProfiles(home)
	install := exec.CommandContext(ctx, bash, script)
	install.Dir = dir
	install.Env = installerEnv(os.Environ())
	// The installer's progress goes to stderr so that stdout keeps only NeoSwitch's own result lines.
	install.Stdout = os.Stderr
	install.Stderr = os.Stderr
	install.ExtraFiles = []*os.File{lock}
	isolateInstaller(install)
	install.WaitDelay = 5 * time.Second
	runErr := install.Run()
	defer reportProfiles(changedProfiles(home, before))
	if runErr != nil {
		reason := runErr.Error()
		switch {
		case stopped.Err() != nil:
			reason = "it was stopped before it finished"
		case ctx.Err() != nil:
			reason = "it did not finish within " + antigravityInstallTimeout.String()
		}
		if !targetExisted {
			_ = os.Remove(target)
		}
		return "", fmt.Errorf("%w and Google's installer failed (%s): %s", provider.ErrAntigravityCLIMissing, reason, antigravityManualInstall())
	}
	path, err := provider.FindAntigravityCLI()
	if err == nil {
		err = checkAntigravityCLI(ctx, path)
	}
	if err != nil {
		if !targetExisted {
			_ = os.Remove(target)
			return "", fmt.Errorf("%w: Google's installer finished but left no working agy (%v); %s", provider.ErrAntigravityCLIMissing, err, antigravityManualInstall())
		}
		return "", fmt.Errorf("%w: %s exists but does not work (%v); delete it, then run /neoswitch:login again", provider.ErrAntigravityCLIMissing, target, err)
	}
	// The installer ends by asking to add its directory to PATH; NeoSwitch finds agy there without that, so the user need not act on it.
	fmt.Printf("ok: installed Google's Antigravity command-line tool (agy) at %s with Google's installer from %s; NeoSwitch finds it there, so you can ignore the installer's note about PATH\n", path, antigravityInstallerURL)
	return path, nil
}

// checkAntigravityCLI runs `agy --version`, which catches a truncated or wrong-platform binary before NeoSwitch relies on it.
func checkAntigravityCLI(ctx context.Context, path string) error {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	check := exec.CommandContext(ctx, path, "--version")
	check.Env = installerEnv(os.Environ())
	if out, err := check.Output(); err != nil || len(bytes.TrimSpace(out)) == 0 {
		if err == nil {
			err = errors.New("`agy --version` printed nothing")
		}
		return err
	}
	return nil
}

// downloadInstaller saves the installer script to path, refusing anything that is not a plausible shell script, such as a proxy's HTML error page.
func downloadInstaller(ctx context.Context, path string) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, antigravityInstallerURL, nil)
	if err != nil {
		return err
	}
	if request.URL.Scheme != "https" {
		return fmt.Errorf("refused non-HTTPS installer URL %s", request.URL.Redacted())
	}
	response, err := antigravityInstallerClient.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return errors.New(response.Status)
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, maxInstallerSize+1))
	if err != nil {
		return err
	}
	if len(body) > maxInstallerSize {
		return errors.New("the installer is unexpectedly large")
	}
	if !bytes.HasPrefix(body, []byte("#!")) || bytes.IndexByte(body, 0) >= 0 {
		return errors.New("the download is not a shell script")
	}
	return os.WriteFile(path, body, 0o600)
}

// installerEnv keeps only the allowlisted variables and the locale (LC_*, XDG_*) from environ.
func installerEnv(environ []string) []string {
	var env []string
	for _, kv := range environ {
		name, _, _ := strings.Cut(kv, "=")
		if slices.Contains(installerEnvNames, name) || strings.HasPrefix(name, "LC_") || strings.HasPrefix(name, "XDG_") {
			env = append(env, kv)
		}
	}
	return env
}

type profileState struct {
	size    int64
	modTime time.Time
	exists  bool
}

func profilePaths(home string) []string {
	paths := make([]string, 0, len(shellProfiles)+3)
	for _, name := range shellProfiles {
		paths = append(paths, filepath.Join(home, name))
	}
	if zdotdir := strings.TrimSpace(os.Getenv("ZDOTDIR")); zdotdir != "" && filepath.Clean(zdotdir) != filepath.Clean(home) {
		for _, name := range []string{".zshrc", ".zprofile", ".zshenv"} {
			paths = append(paths, filepath.Join(zdotdir, name))
		}
	}
	return paths
}

func snapshotProfiles(home string) map[string]profileState {
	states := make(map[string]profileState)
	for _, path := range profilePaths(home) {
		info, err := os.Stat(path)
		if err != nil {
			states[path] = profileState{}
			continue
		}
		states[path] = profileState{size: info.Size(), modTime: info.ModTime(), exists: true}
	}
	return states
}

func changedProfiles(home string, before map[string]profileState) []string {
	var changed []string
	for path, now := range snapshotProfiles(home) {
		if now != before[path] {
			changed = append(changed, path)
		}
	}
	slices.Sort(changed)
	return changed
}

func reportProfiles(changed []string) {
	if len(changed) > 0 {
		fmt.Printf("note: Google's installer changed %s to put ~/.local/bin on your PATH; NeoSwitch does not need that, and you may keep or remove the line it added\n", strings.Join(changed, ", "))
	}
}
