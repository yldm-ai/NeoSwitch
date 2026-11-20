package daemon

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	envBaseURL   = "ANTHROPIC_BASE_URL"
	envAuthToken = "ANTHROPIC_AUTH_TOKEN"
	// EnvUseGateway is Claude Code's gateway mode switch, set by gateway launchers such as `everyapi use claude`. In gateway mode a session takes ANTHROPIC_BASE_URL from its process environment over project settings, so the proxy entries alone would be ignored silently while the SessionStart hook, which does see the project settings, still attaches. Enable therefore turns gateway mode off for the project.
	EnvUseGateway = "CLAUDE_CODE_USE_GATEWAY"
	gatewayOff    = "0"
)

// settingsPath is the per-user, per-project Claude Code settings file. It is the narrowest scope that can set ANTHROPIC_BASE_URL: other projects and other people's checkouts keep talking to Anthropic.
func settingsPath(project string) string {
	return filepath.Join(project, ".claude", "settings.local.json")
}

// Enable points Claude Code sessions in project at the shared proxy by adding its env entries to settings.local.json. Every other key in the file is preserved.
func Enable(project string, s State) error {
	_, err := editSettings(project, func(env map[string]any) bool {
		env[envBaseURL] = s.BaseURL()
		env[envAuthToken] = s.Token
		env[EnvUseGateway] = gatewayOff
		return true
	})
	return err
}

// Disable removes the env entries Enable wrote and reports whether there were any. Besides entries for s's address it removes those for an earlier proxy address, recognized by both marks Enable leaves (see IsProxyEntry), as left behind when the state was reset. Entries the user changed to point elsewhere are left alone.
func Disable(project string, s State) (bool, error) {
	return editSettings(project, func(env map[string]any) bool {
		switch {
		case env[envBaseURL] == s.BaseURL():
			delete(env, envBaseURL)
			if env[envAuthToken] == s.Token {
				delete(env, envAuthToken)
			}
			if env[EnvUseGateway] == gatewayOff {
				delete(env, EnvUseGateway)
			}
		case proxyEntry(env):
			delete(env, envBaseURL)
			delete(env, envAuthToken)
			delete(env, EnvUseGateway)
		default:
			return false
		}
		return true
	})
}

// Enabled reports whether project's settings point at the shared proxy.
func Enabled(project string, s State) (bool, error) {
	settings, err := readSettings(project)
	if err != nil {
		return false, err
	}
	env, _ := settings["env"].(map[string]any)
	return env[envBaseURL] == s.BaseURL(), nil
}

// StaleAddress returns the proxy address project's settings point at when it is NeoSwitch's but not s's current one, and "" otherwise. Sessions in such a project cannot reach NeoSwitch; running on again fixes it. Pass a zero State when there is none.
func StaleAddress(project string, s State) (string, error) {
	settings, err := readSettings(project)
	if err != nil {
		return "", err
	}
	env, _ := settings["env"].(map[string]any)
	if env[envBaseURL] == s.BaseURL() || !proxyEntry(env) {
		return "", nil
	}
	return env[envBaseURL].(string), nil
}

// IsProxyEntry reports whether a base URL and gateway setting look like the entries Enable writes, for any proxy address: a loopback base URL together with gateway mode turned off. Both marks are required, so a local gateway the user set up is not mistaken for NeoSwitch.
func IsProxyEntry(baseURL, useGateway string) bool {
	return strings.HasPrefix(baseURL, "http://127.0.0.1:") && useGateway == gatewayOff
}

func proxyEntry(env map[string]any) bool {
	baseURL, _ := env[envBaseURL].(string)
	useGateway, _ := env[EnvUseGateway].(string)
	return IsProxyEntry(baseURL, useGateway)
}

func readSettings(project string) (map[string]any, error) {
	data, err := os.ReadFile(settingsPath(project))
	if errors.Is(err, os.ErrNotExist) {
		return map[string]any{}, nil
	}
	if err != nil {
		return nil, err
	}
	settings := map[string]any{}
	if len(bytes.TrimSpace(data)) == 0 {
		return settings, nil
	}
	if err := json.Unmarshal(data, &settings); err != nil {
		return nil, invalidSettings(settingsPath(project), data, err)
	}
	return settings, nil
}

// invalidSettings explains a settings file NeoSwitch cannot parse, pointing at the line of a syntax error, so the user can fix it without knowing JSON error messages. The file is never rewritten in that state.
func invalidSettings(path string, data []byte, err error) error {
	var syntax *json.SyntaxError
	if errors.As(err, &syntax) {
		offset := min(int(syntax.Offset), len(data))
		line := 1 + bytes.Count(data[:offset], []byte("\n"))
		return fmt.Errorf("NeoSwitch cannot read %s: line %d has a formatting mistake, such as a missing or extra comma. Fix that line and try again", path, line)
	}
	return fmt.Errorf("NeoSwitch cannot read %s: it must hold a single JSON object ({ ... }). Fix the file and try again", path)
}

// editSettings applies edit to the env entries of project's settings and writes the file when edit reports a change.
func editSettings(project string, edit func(env map[string]any) bool) (bool, error) {
	settings, err := readSettings(project)
	if err != nil {
		return false, err
	}
	env, ok := settings["env"].(map[string]any)
	if !ok {
		if _, present := settings["env"]; present {
			return false, fmt.Errorf("NeoSwitch cannot change %s: its \"env\" entry is not a JSON object ({ ... }). Fix the file and try again", settingsPath(project))
		}
		env = map[string]any{}
	}
	if !edit(env) {
		return false, nil
	}
	if len(env) == 0 {
		delete(settings, "env")
	} else {
		settings["env"] = env
	}
	path := settingsPath(project)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return false, err
	}
	data, _ := json.MarshalIndent(settings, "", "  ")
	// Write through a temp file so a crash never leaves Claude Code with a truncated settings file.
	tmp := path + ".neoswitch.tmp"
	if err := os.WriteFile(tmp, append(data, '\n'), 0o600); err != nil {
		return false, err
	}
	if err := os.Rename(tmp, path); err != nil {
		return false, err
	}
	return true, nil
}
