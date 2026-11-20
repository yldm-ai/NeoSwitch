package daemon

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
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
	return editSettings(project, func(env map[string]any) {
		env[envBaseURL] = s.BaseURL()
		env[envAuthToken] = s.Token
		env[EnvUseGateway] = gatewayOff
	})
}

// Disable removes the env entries Enable wrote. Entries the user changed to point elsewhere are left alone.
func Disable(project string, s State) error {
	return editSettings(project, func(env map[string]any) {
		if env[envBaseURL] == s.BaseURL() {
			delete(env, envBaseURL)
			if env[envAuthToken] == s.Token {
				delete(env, envAuthToken)
			}
			if env[EnvUseGateway] == gatewayOff {
				delete(env, EnvUseGateway)
			}
		}
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

func editSettings(project string, edit func(env map[string]any)) error {
	settings, err := readSettings(project)
	if err != nil {
		return err
	}
	env, ok := settings["env"].(map[string]any)
	if !ok {
		if _, present := settings["env"]; present {
			return fmt.Errorf("NeoSwitch cannot change %s: its \"env\" entry is not a JSON object ({ ... }). Fix the file and try again", settingsPath(project))
		}
		env = map[string]any{}
	}
	edit(env)
	if len(env) == 0 {
		delete(settings, "env")
	} else {
		settings["env"] = env
	}
	path := settingsPath(project)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, _ := json.MarshalIndent(settings, "", "  ")
	// Write through a temp file so a crash never leaves Claude Code with a truncated settings file.
	tmp := path + ".neoswitch.tmp"
	if err := os.WriteFile(tmp, append(data, '\n'), 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
