# NeoSwitch

NeoSwitch is a local, compatibility-first bridge that runs Claude Code against Gemini 4 Neon through an official Gemini API endpoint.

The first release deliberately keeps the surface small: Claude Code remains the agent runtime, while NeoSwitch owns only the Anthropic Messages to Gemini translation boundary. It listens on loopback, keeps credentials in the process environment, and does not write prompts or responses to disk.

## Status

This repository contains the first protocol-contract MVP. It supports:

- `neoswitch serve` — run the local Anthropic-compatible proxy.
- `neoswitch claude` — start Claude Code through an ephemeral local proxy.
- text, images, tools, tool results, non-streaming responses, and buffered Anthropic SSE responses.
- Gemini `generateContent` using `gemini-4-neon` by default.

Streaming is intentionally buffered in this MVP. The proxy emits a valid Anthropic SSE sequence only after Gemini returns a complete response; this avoids partial tool-call replay while the compatibility contract is being validated. Native upstream streaming is a separate, tested milestone.

## Quick start

Set an official Gemini API key, then run Claude Code through NeoSwitch:

```bash
export GEMINI_API_KEY='…'
go run ./cmd/neoswitch claude
```

To use another model or Claude executable:

```bash
GEMINI_MODEL=gemini-4-neon CLAUDE_CLI_PATH=claude go run ./cmd/neoswitch claude
```

Run the proxy directly when integrating another Anthropic client:

```bash
GEMINI_API_KEY='…' go run ./cmd/neoswitch serve
```

The proxy binds to `127.0.0.1` by default. Set `NEOSWITCH_LISTEN` to another loopback address only when needed. `GEMINI_BASE_URL` can point at a compatible Google endpoint; the default is the Gemini Developer API.

## Stability boundary

NeoSwitch uses official Gemini API credentials. Consumer OAuth flows, Antigravity scraping, hosts-file interception, and remote credential relays are intentionally outside the stable core. If the model is not available to the configured Google account, NeoSwitch fails with an explicit upstream error instead of silently switching models.

See [docs/protocol-contract.md](docs/protocol-contract.md) for the request/response contract and the compatibility test matrix.
