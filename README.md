# NeoSwitch

[简体中文](README.zh-CN.md)

[![CI](https://github.com/yldm-ai/NeoSwitch/actions/workflows/test.yml/badge.svg)](https://github.com/yldm-ai/NeoSwitch/actions/workflows/test.yml) [![Go Reference](https://pkg.go.dev/badge/github.com/yldm-ai/NeoSwitch.svg)](https://pkg.go.dev/github.com/yldm-ai/NeoSwitch) [![License](https://img.shields.io/github/license/yldm-ai/NeoSwitch)](https://github.com/yldm-ai/NeoSwitch/blob/main/LICENSE)

NeoSwitch lets Claude Code use Gemini 4 Neon through a local Anthropic-compatible proxy. It supports two upstream credential paths:

- Antigravity OAuth through the official local `agy acp` process.
- Gemini Developer API or Vertex AI credentials through the official `google.golang.org/genai` SDK.

The recommended setup for Claude Code is the Antigravity OAuth plugin flow. OAuth credentials stay inside `agy`. NeoSwitch does not read or export the token.

## Use Claude Code with Neon

### Prerequisites

- Claude Code.
- The official Antigravity CLI, available as `agy` on your PATH.
- Go. The plugin builds the bundled NeoSwitch binary on first use.

### Install the plugin

Run these commands inside Claude Code:

```text
/plugin marketplace add yldm-ai/NeoSwitch
/plugin install neoswitch@neoswitch
```

If the plugin was already installed, update it from a shell after a new release:

```bash
claude plugin marketplace update neoswitch
claude plugin update neoswitch@neoswitch
```

Restart Claude Code after updating. The current plugin version is required for the ACP login flow.

### Sign in and enable Neon

In the project where you want to use Neon, run:

```text
/neoswitch:login
```

The command performs the following steps:

1. Starts the ACP authentication flow owned by `agy`.
2. Opens the browser when Google OAuth is required.
3. Enables NeoSwitch routing for the current project.
4. Selects `gemini-4-neon` for the ACP session.

Close and reopen Claude Code in that project after the command completes. Check the route with:

```text
/neoswitch:status
```

The status should show `NeoSwitch ... -> gemini-4-neon`. Claude Code can still display its requested label, such as `claude-opus-5-5`, in the session header. The upstream route is shown by `/neoswitch:status`.

`/neoswitch:antigravity-login` is an alias for `/neoswitch:login`. `/neoswitch:on` and `/neoswitch:off` remain available when you want to change routing without logging in.

## Other credential paths

### Gemini API key

```bash
export GEMINI_API_KEY='your-key'
go run ./cmd/neoswitch claude
```

The plugin can also receive the key through `/plugin` → `neoswitch` → **Gemini API key**. The key is stored by Claude Code in its secure plugin configuration.

### Vertex AI

```bash
export GOOGLE_GENAI_USE_VERTEXAI=true
export GOOGLE_CLOUD_PROJECT=my-project
export GOOGLE_CLOUD_LOCATION=global
go run ./cmd/neoswitch claude
```

The Google Cloud SDK's Application Default Credentials must already be available to the process.

## Plugin commands

| Command | Effect |
|---|---|
| `/neoswitch:login` | Authenticate with Antigravity OAuth, enable this project's Neon route, and use ACP |
| `/neoswitch:on` | Enable routing for new Claude Code sessions in this project |
| `/neoswitch:status` | Show the project route and daemon state |
| `/neoswitch:off` | Disable routing for new sessions in this project |

After changing routing, start a new Claude Code session. The current session keeps its existing connection.

## How the plugin works

`/neoswitch:on` writes only `ANTHROPIC_BASE_URL` and `ANTHROPIC_AUTH_TOKEN` to the project's `.claude/settings.local.json`. The token authorizes the loopback proxy and is not an Antigravity credential.

The SessionStart hook starts one local daemon and registers the Claude Code process before its first model request. The daemon exits after all registered Claude Code processes have exited. The proxy listens on loopback by default.

With the OAuth route, the daemon starts the official `agy acp` subprocess. NeoSwitch sends the translated request through an ACP session and translates the returned answer or Claude-compatible tool-call JSON back to the local Anthropic-compatible endpoint. OAuth credentials and browser callbacks remain owned by `agy`.

The optional plugin setting **Use Antigravity OAuth via ACP** forces the ACP route when another Gemini credential is also present. A normal `/neoswitch:login` automatically uses ACP when no Gemini credential is configured.

## Standalone CLI

Build the binary or run it from the repository:

```bash
go build -o neoswitch ./cmd/neoswitch
go install ./cmd/neoswitch
```

Common commands:

```bash
neoswitch antigravity login   # ACP OAuth login through the official agy client
neoswitch antigravity probe   # verify the saved agy session without printing credentials
neoswitch doctor              # check the selected backend and Claude CLI
neoswitch models              # print the configured model
neoswitch on [project]        # enable routing for a project
neoswitch off [project]       # disable routing for a project
neoswitch status [project]    # show routing and daemon state
```

To run the local proxy for another Anthropic-compatible client:

```bash
export NEOSWITCH_AUTH_TOKEN="$(openssl rand -hex 24)"
GEMINI_API_KEY='your-key' neoswitch serve
```

Set both `ANTHROPIC_BASE_URL` and `ANTHROPIC_AUTH_TOKEN` in the client. `neoswitch claude` configures those variables automatically with an ephemeral port and token.

## Troubleshooting

| Symptom | Fix |
|---|---|
| `/neoswitch:login` is unknown | Update the plugin, restart Claude Code, and run `/reload-plugins`. |
| `agy: command not found` | Install the official Antigravity CLI or set `ANTIGRAVITY_CLI_PATH`. |
| `no authorization code was entered` | Update to the latest plugin. The current flow uses ACP `authenticate` and does not read shell stdin. |
| `ECONNREFUSED` on a new session | Run `/neoswitch:status`. If the route is enabled, inspect the daemon message and restart the session. Run `/neoswitch:off` from a session using Anthropic, then enable it again after fixing the credential. |
| The session starts with no Gemini credentials | Run `/neoswitch:login`, or configure a Gemini API key or Vertex AI ADC. |
| The plugin build fails | Install Go and restart Claude Code. |

## Compatibility boundary

NeoSwitch uses the official Gemini SDK or the unmodified local `agy acp` process. It does not extract OAuth tokens, call Antigravity private HTTP APIs, relay credentials to a remote proxy, modify hosts files, or silently fall back to another model.

Claude Code remains responsible for the session, permissions, MCP, and tool loop. Anthropic does not document routing Claude Code to non-Claude models, so Claude Code updates can change behavior that NeoSwitch relies on.

See [docs/protocol-contract.md](docs/protocol-contract.md) for the endpoint contract and compatibility matrix. See [docs/antigravity.md](docs/antigravity.md) for ACP authentication details.

## Development

```bash
go test ./...
go test -race ./...
go vet ./...
```

MIT License
