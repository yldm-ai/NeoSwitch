# NeoSwitch

[English](README.md) | [简体中文](README.zh-CN.md)

NeoSwitch is a local, compatibility-first bridge that runs Claude Code against Gemini 4 Neon through an official Gemini API endpoint.

The first release deliberately keeps the surface small: Claude Code remains the agent runtime, while NeoSwitch owns only the Anthropic Messages to Gemini translation boundary. It listens on loopback, keeps credentials in the process environment, and does not write prompts or responses to disk.

## Status

It supports:

- `neoswitch serve` — run the local Anthropic-compatible proxy.
- `neoswitch claude` — start Claude Code through an ephemeral local proxy.
- `neoswitch antigravity login` — complete the official `agy` browser OAuth sign-in.
- `neoswitch antigravity probe` — verify an existing `agy` login without reading its token; this is not a Claude Code backend.
- text, images, PDFs, Claude Code's tool schemas, multi-turn tool calls (including Gemini thought signatures), streaming and non-streaming responses, and `count_tokens`.
- the Gemini Developer API or Vertex AI through the official `google.golang.org/genai` SDK, using `gemini-4-neon` by default.

## Quick start

Set a Gemini API key, then run Claude Code through NeoSwitch:

```bash
export GEMINI_API_KEY='…'
go run ./cmd/neoswitch claude
```

To use another model or Claude executable:

```bash
GEMINI_MODEL=gemini-4-neon CLAUDE_CLI_PATH=claude go run ./cmd/neoswitch claude
```

To use Vertex AI with Application Default Credentials instead of an API key:

```bash
export GOOGLE_GENAI_USE_VERTEXAI=true GOOGLE_CLOUD_PROJECT=my-project GOOGLE_CLOUD_LOCATION=global
go run ./cmd/neoswitch claude
```

Run the proxy directly when integrating another Anthropic client:

```bash
export NEOSWITCH_AUTH_TOKEN="$(openssl rand -hex 24)"
GEMINI_API_KEY='…' go run ./cmd/neoswitch serve

# in the client's environment
ANTHROPIC_BASE_URL=http://127.0.0.1:8080 ANTHROPIC_AUTH_TOKEN="$NEOSWITCH_AUTH_TOKEN" claude
```

Always set both: a Claude Code client given only a base URL sends its saved claude.ai credentials to the proxy instead. `neoswitch claude` does this for you with a random port and a one-time token.

The proxy binds to `127.0.0.1` by default. Set `NEOSWITCH_LISTEN` to another loopback address only when needed. `GEMINI_BASE_URL` overrides the Gemini API host.

## Claude Code plugin

The plugin routes chosen projects to Gemini while plain `claude` keeps working everywhere else, with no wrapper command to remember.

```text
/plugin marketplace add yldm-ai/NeoSwitch
/plugin install neoswitch@neoswitch
```

Set the Gemini API key in the plugin's configuration (`/plugin`, then neoswitch). Claude Code keeps it in the macOS Keychain. Without it, NeoSwitch falls back to `GEMINI_API_KEY` or Vertex AI credentials from the environment Claude Code was started in.

| Command | Effect |
|---|---|
| `/neoswitch:on` | Route new sessions in this project to Gemini |
| `/neoswitch:status` | Show routing and whether the proxy is running |
| `/neoswitch:off` | Route new sessions in this project back to Anthropic |
| `/neoswitch:antigravity-login` | Sign in to the official Antigravity CLI with Google OAuth |

After `/neoswitch:on` or `/neoswitch:off`, start a new session in the project; the running session keeps its connection.

Run `/neoswitch:antigravity-login` in Claude Code to launch the official `agy` CLI and complete its browser-based Google OAuth flow. The login is stored by `agy` in the platform keyring. This authenticates the Antigravity CLI and does not switch NeoSwitch's Gemini backend to Antigravity.

How it works:

- `/neoswitch:on` adds `ANTHROPIC_BASE_URL` and `ANTHROPIC_AUTH_TOKEN` to the project's `.claude/settings.local.json`, pointing at one loopback proxy shared by all enabled projects. Other settings in the file are preserved, and `/neoswitch:off` removes only those two entries.
- A SessionStart hook starts the proxy before the session's first request and registers the Claude Code process with it. The proxy exits once every registered session has exited, so nothing keeps running after you close Claude Code.
- On first use the plugin builds the bundled source with Go, so Go must be installed. macOS and Linux are supported.

Things to know:

- `.claude/settings.local.json` then holds the proxy token. Keep that file out of version control; `/neoswitch:on` warns when git does not ignore it.
- Remote Control is unavailable in routed projects, because Claude Code disables it whenever `ANTHROPIC_BASE_URL` points away from Anthropic.
- Anthropic documents that it does not support routing Claude Code to non-Claude models, so Claude Code updates can change behavior NeoSwitch depends on.

## Stability boundary

NeoSwitch uses official Gemini API or Vertex AI credentials. Consumer OAuth flows, Antigravity scraping, hosts-file interception, and remote credential relays are intentionally outside the stable core. If the model is not available to the configured Google account, NeoSwitch fails with an explicit upstream error instead of silently switching models.

See [docs/protocol-contract.md](docs/protocol-contract.md) for the request/response contract and the compatibility test matrix.

Antigravity account sessions are intentionally kept outside the stable model path. See [docs/antigravity.md](docs/antigravity.md).
