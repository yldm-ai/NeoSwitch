# NeoSwitch

**English** | [简体中文](i18n/README.zh-CN.md) | [繁體中文](i18n/README.zh-TW.md) | [日本語](i18n/README.ja.md) | [한국어](i18n/README.ko.md) | [Español](i18n/README.es.md) | [Français](i18n/README.fr.md) | [Deutsch](i18n/README.de.md) | [Português (Brasil)](i18n/README.pt-BR.md) | [Русский](i18n/README.ru.md)

[![CI](https://github.com/yldm-ai/neon-switch/actions/workflows/test.yml/badge.svg)](https://github.com/yldm-ai/neon-switch/actions/workflows/test.yml) [![Go Reference](https://pkg.go.dev/badge/github.com/yldm-ai/neon-switch.svg)](https://pkg.go.dev/github.com/yldm-ai/neon-switch) [![Go version](https://img.shields.io/github/go-mod/go-version/yldm-ai/neon-switch)](https://github.com/yldm-ai/neon-switch/blob/main/go.mod) [![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](https://github.com/yldm-ai/neon-switch/blob/main/LICENSE) [![Last commit](https://img.shields.io/github/last-commit/yldm-ai/neon-switch)](https://github.com/yldm-ai/neon-switch/commits/main) [![GitHub stars](https://img.shields.io/github/stars/yldm-ai/neon-switch)](https://github.com/yldm-ai/neon-switch/stargazers)

[![Claude Code plugin](https://img.shields.io/badge/Claude%20Code-plugin-D97757)](https://github.com/yldm-ai/neon-switch/blob/main/.claude-plugin/plugin.json) [![Model: Gemini 4 Neon](https://img.shields.io/badge/model-Gemini%204%20Neon-4285F4)](https://github.com/yldm-ai/neon-switch/blob/main/docs/protocol-contract.md) [![Platform: macOS and Linux](https://img.shields.io/badge/platform-macOS%20%7C%20Linux-lightgrey)](https://github.com/yldm-ai/neon-switch/releases) [![Proxy: local-only](https://img.shields.io/badge/proxy-local--only-2ea44f)](https://github.com/yldm-ai/neon-switch/blob/main/docs/protocol-contract.md)

**Keep Claude Code. Switch your models.**

NeoSwitch lets you use Gemini with Claude Code without changing the workflow you already know. Claude Code keeps its CLI, commands, permissions, tools, skills, MCP servers and hooks; NeoSwitch runs a small local proxy that translates its requests for the model behind it.

Today NeoSwitch routes Claude Code to Gemini 4 Neon. Other providers are planned and not yet supported.

## Why NeoSwitch

- **Keep your workflow**: you still run Claude Code. Only the model changes.
- **Per project**: turn routing on in one project and leave the others on Anthropic.
- **Local first**: the proxy listens on loopback only, uses a random access token, and logs no prompts, responses or credentials. No remote relay is involved.
- **Honest compatibility**: what works, what is partial and what is missing is listed [below](#compatibility).

## Choose a credential path

| Path | How it signs in | Notes |
|---|---|---|
| Antigravity OAuth | Google account through the official `agy` CLI | The plugin's default flow. Read the warning below first. |
| Gemini Developer API | `GEMINI_API_KEY` | Access to Gemini 4 Neon depends on your key. Check it with `neoswitch doctor`. |
| Vertex AI | Google Cloud Application Default Credentials | Access depends on your project and region. Check it with `neoswitch doctor`. |

> [!WARNING]
> The [Google Antigravity Terms of Service](https://antigravity.google/terms) (section 6) treat accessing Antigravity through third-party software, tools or services as a breach and say Google may suspend or terminate the Antigravity and Gemini CLI accounts involved. Routing Claude Code through Antigravity OAuth falls under that rule. Use it only if you accept that risk for the account you sign in with; otherwise use a Gemini API key or Vertex AI. `neoswitch doctor` and `neoswitch antigravity login` print the same warning.

OAuth credentials always stay inside `agy`. NeoSwitch does not read, copy or export the token.

## Use Claude Code with Neon

### Prerequisites

- Claude Code.
- For Antigravity OAuth: the official Antigravity CLI, available as `agy` on your PATH.
- Go. The plugin builds the bundled NeoSwitch binary on first use.

### Install the plugin

Run these commands inside Claude Code:

```text
/plugin marketplace add yldm-ai/neon-switch
/plugin install neoswitch@neoswitch
```

If the plugin was already installed, update it from a shell after a new release:

```bash
claude plugin marketplace update neoswitch
claude plugin update neoswitch@neoswitch
```

Restart Claude Code after updating.

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

To use a Gemini API key instead, export `GEMINI_API_KEY` before starting Claude Code and run `/neoswitch:on`; the plugin has no settings to fill in. If you signed in with `/neoswitch:login` before, also export `NEOSWITCH_BACKEND=gemini`, because the saved login choice wins otherwise.

Close and reopen Claude Code in that project. Check the route with:

```text
/neoswitch:status
```

The status should show `NeoSwitch ... -> gemini-4-neon`. Claude Code can still display its requested label, such as `claude-opus-5-5`, in the session header; the upstream route is the one `/neoswitch:status` shows.

`/neoswitch:antigravity-login` is an alias for `/neoswitch:login`. `/neoswitch:on` and `/neoswitch:off` change routing without signing in.

## Other credential paths

### Gemini API key

```bash
export GEMINI_API_KEY='your-key'
neoswitch doctor
go run ./cmd/neoswitch claude
```

### Vertex AI

```bash
export GOOGLE_GENAI_USE_VERTEXAI=true
export GOOGLE_CLOUD_PROJECT=my-project
export GOOGLE_CLOUD_LOCATION=global
neoswitch doctor
go run ./cmd/neoswitch claude
```

The Google Cloud SDK's Application Default Credentials must already be available to the process.

`neoswitch doctor` looks the model up and runs a one-line generation, then reports whether the credentials work or why they do not: rejected credentials, missing permission, region restriction, unknown model, quota, or an unavailable service.

## Plugin commands

| Command | Effect |
|---|---|
| `/neoswitch:login` | Authenticate with Antigravity OAuth, enable this project's Neon route, and use ACP |
| `/neoswitch:on` | Enable routing for new Claude Code sessions in this project |
| `/neoswitch:status` | Show the project route and daemon state |
| `/neoswitch:off` | Disable routing for new sessions in this project |

After changing routing, start a new Claude Code session. The current session keeps its existing connection.

## Compatibility

Claude Code still runs the session, permissions, tool execution, MCP servers, skills, hooks, `CLAUDE.md` and `/resume`; NeoSwitch only replaces the model behind them. How well those features work depends on how well the model follows Claude Code's tool protocol, which has been exercised in everyday coding use but is not covered by an automated end-to-end suite.

| Capability | Gemini API / Vertex AI | Antigravity OAuth (ACP) |
|---|---|---|
| Streaming text | Supported | Supported |
| Tool calls, including parallel calls | Native Gemini function calling | Partial: the model writes Claude tool-call JSON, which arrives whole instead of streamed |
| Multi-turn tool use and `/resume` | Supported; thought signatures are carried in the tool IDs | Supported |
| Images | Supported | Supported when `agy` offers image input |
| PDFs | Supported | Not supported; the model is told a file was omitted |
| `/effort` | Not applied yet | Mapped to `agy`'s thought level |
| Token usage and `count_tokens` | Reported; `count_tokens` is estimated on the Developer API | Reported, including about 14k tokens of `agy` instructions added to every prompt |
| Gemini 4 Neon access | Depends on your key or project; check with `neoswitch doctor` | Verified with a signed-in `agy` |

Not supported on either path: server tools such as web search and code execution, URL image and document sources, and Claude thinking blocks. `cache_control` and `context_management` are ignored. NeoSwitch never falls back to another model silently.

The full endpoint contract is in [docs/protocol-contract.md](docs/protocol-contract.md), and ACP details are in [docs/antigravity.md](docs/antigravity.md).

## How the plugin works

`/neoswitch:on` writes only `ANTHROPIC_BASE_URL` and `ANTHROPIC_AUTH_TOKEN` to the project's `.claude/settings.local.json`. The token authorizes the loopback proxy and is not an Antigravity credential.

The SessionStart hook starts one local daemon and registers the Claude Code process before its first model request. The daemon exits after all registered Claude Code processes have exited.

With the OAuth route, the daemon starts the official `agy acp` subprocess. NeoSwitch sends the translated request through an ACP session and translates the returned answer or tool-call JSON back to the local Anthropic-compatible endpoint. One `agy` process serves all sessions, and requests run concurrently in separate ACP sessions. `agy`'s own file, shell and web tools are refused on every request, so tools run only in Claude Code, under Claude Code's permission rules.

`/neoswitch:login` saves Antigravity OAuth as the backend, so a Gemini key exported for other tools does not take over. Without a saved choice, NeoSwitch uses Gemini credentials from the environment when present and Antigravity OAuth otherwise. `NEOSWITCH_BACKEND=antigravity|gemini|auto` overrides both, and `/neoswitch:status` shows the backend in use and why.

## Standalone CLI

Each tagged version publishes binaries for macOS (Apple Silicon and Intel), Linux (amd64 and arm64) and Windows (amd64), with SHA-256 checksums, on the [Releases](https://github.com/yldm-ai/neon-switch/releases) page. The Windows build is compiled but not yet tested. You can also build from source:

```bash
go build -o neoswitch ./cmd/neoswitch
go install ./cmd/neoswitch
```

Common commands:

```bash
neoswitch doctor              # check the Claude CLI and whether the credentials can use the model
neoswitch antigravity login   # ACP OAuth login through the official agy client
neoswitch antigravity probe   # verify the saved agy session without printing credentials
neoswitch models              # print the configured model
neoswitch on [project]        # enable routing for a project
neoswitch off [project]       # disable routing for a project
neoswitch status [project]    # show routing and daemon state
neoswitch version             # print the NeoSwitch version
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
| Not sure whether the credentials work | Run `neoswitch doctor`; it names the failing step and the cause. |
| `/neoswitch:login` is unknown | Update the plugin, restart Claude Code, and run `/reload-plugins`. |
| `agy: command not found` | Install the official Antigravity CLI or set `ANTIGRAVITY_CLI_PATH`. |
| `no authorization code was entered` | Update to the latest plugin. The current flow uses ACP `authenticate` and does not read shell stdin. |
| `ECONNREFUSED` on a new session | Run `/neoswitch:status`. If the route is enabled, inspect the daemon message and restart the session. Run `/neoswitch:off` from a session using Anthropic, then enable it again after fixing the credential. |
| The session reports that no backend is configured | Run `/neoswitch:login`, or export `GEMINI_API_KEY` or Vertex AI settings before starting Claude Code. |
| The plugin build fails | Install Go and restart Claude Code. |

## Compatibility boundary

NeoSwitch uses the official Gemini SDK or the unmodified local `agy acp` process. It does not extract OAuth tokens, call Antigravity private HTTP APIs, relay credentials to a remote proxy, or modify hosts files.

Anthropic does not document routing Claude Code to non-Claude models, so Claude Code updates can change behavior that NeoSwitch relies on.

## Roadmap

- More model providers, starting with OpenAI. Providers that already offer an Anthropic-compatible endpoint can be used by setting `ANTHROPIC_BASE_URL` directly and do not need NeoSwitch.
- Prebuilt binaries for the plugin, so it no longer needs Go.

## Development

```bash
go test ./...
go test -race ./...
go vet ./...
```

MIT License
