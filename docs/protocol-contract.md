# NeoSwitch protocol contract (MVP)

## Scope

NeoSwitch presents the subset of the Anthropic Messages API that Claude Code needs and translates it to the Gemini `generateContent` REST shape. The proxy is local-only by default and is not a general-purpose multi-provider gateway.

## Endpoints

| Method | Path | Contract |
|---|---|---|
| GET | `/healthz` | Returns `200 {"status":"ok"}` when the process is serving. It does not call Gemini. |
| GET | `/v1/models` | Returns the configured model as an Anthropic-style model entry. |
| POST | `/v1/messages` | Accepts an Anthropic Messages request and returns JSON or Anthropic SSE. |

## Request invariants

- `model` is accepted for Claude Code compatibility, but the backend model is selected by `GEMINI_MODEL`.
- `messages` must be non-empty and contain only `user` and `assistant` roles.
- Content blocks may be `text`, `image`, `tool_use`, or `tool_result`.
- `tools` are converted to Gemini function declarations.
- `max_tokens` is required by the Anthropic contract and becomes `maxOutputTokens`.
- Unknown content blocks are rejected with `400`; they are never silently discarded.
- Requests are bounded by the server timeout and are never retried after a response has started.

## Response invariants

- Every response has `type: "message"`, `role: "assistant"`, `model`, `content`, `stop_reason`, and `usage`.
- A Gemini function call becomes an Anthropic `tool_use` block with a stable ID for that response.
- A response containing one or more tool calls has `stop_reason: "tool_use"`.
- A normal completed response has `stop_reason: "end_turn"`.
- Gemini usage metadata is copied when available; missing usage is reported as zero rather than fabricated.
- Buffered streaming emits the canonical event order:

```text
message_start
content_block_start*
content_block_delta*
content_block_stop*
message_delta
message_stop
```

## Failure policy

- Missing credentials fail before an upstream request is made.
- Gemini 4xx/5xx responses are returned as Anthropic-compatible `502` errors with the provider status preserved in the message.
- No automatic model fallback is enabled in the MVP.
- No retry occurs after the first SSE byte is written.
- Logs contain request metadata and provider status only; prompts, tool arguments, API keys, and responses are not logged.

## Compatibility matrix

| Scenario | MVP status | Acceptance check |
|---|---|---|
| Text, non-streaming | Supported | Response text and usage preserved |
| Text, `stream: true` | Supported, buffered | Canonical SSE event order |
| System prompt | Supported | Gemini system instruction populated |
| Single tool call | Supported | Tool name, arguments, and ID preserved |
| Parallel tool calls | Translation supported | Each call receives its own ID |
| Tool result turn | Supported | Function response references prior tool name |
| Image input | Supported | Base64 inline data and MIME type preserved |
| Native upstream streaming | Not yet | Separate milestone; must pass tool-call replay tests |
| Session resume | Launcher-compatible | Resume through `neoswitch claude` |
| Consumer OAuth / Antigravity | Not supported | Explicitly outside stable core |
