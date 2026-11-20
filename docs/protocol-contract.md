# NeoSwitch protocol contract

## Scope

NeoSwitch presents the subset of the Anthropic Messages API that Claude Code needs and translates it to Gemini `generateContent` / `streamGenerateContent` through the official `google.golang.org/genai` SDK. The proxy is local-only by default and is not a general-purpose multi-provider gateway.

## Endpoints

| Method | Path | Contract |
|---|---|---|
| GET | `/healthz` | Returns `200 {"status":"ok"}` when the process is serving. It does not call Gemini. |
| GET | `/v1/models` | Returns the configured model as an Anthropic-style model entry. |
| POST | `/v1/messages` | Accepts an Anthropic Messages request and returns JSON, or Anthropic SSE when `stream` is true. |
| POST | `/v1/messages/count_tokens` | Returns `{"input_tokens": n}` for the same request shape (no `max_tokens` required). |

Query strings such as `?beta=true` are ignored. Every POST endpoint requires the configured token as `Authorization: Bearer`, `x-api-key` or `X-NeoSwitch-Token` when a token is set.

## Credentials

genai resolves credentials from the standard environment: `GEMINI_API_KEY` (or `GOOGLE_API_KEY`) for the Gemini Developer API, or `GOOGLE_GENAI_USE_VERTEXAI=true` with `GOOGLE_CLOUD_PROJECT`, `GOOGLE_CLOUD_LOCATION` and Application Default Credentials for Vertex AI. The Developer API is called on `v1beta` unless `GOOGLE_GENAI_API_VERSION` is set. `GEMINI_BASE_URL` overrides the API host; a trailing `/v1beta` or `/v1` is accepted and stripped.

## Request translation

- `model` is accepted for Claude Code compatibility; the backend model is selected by `GEMINI_MODEL`.
- `system` and mid-conversation `role: "system"` messages (Claude Code sends its environment block this way) become Gemini `systemInstruction` text, in order.
- `user` / `assistant` messages become `user` / `model` contents. Messages that translate to no parts are dropped, because Gemini rejects empty contents.
- Content blocks:

| Anthropic block | Gemini |
|---|---|
| `text` | text part; empty text is dropped |
| `image` (base64) | `inlineData` |
| `document` (base64, e.g. PDF) | `inlineData` |
| `document` (text source) | text part |
| `tool_use` | `functionCall`, with the thought signature and call ID recovered from the tool ID |
| `tool_result` | `functionResponse` with `{"output": text}` or `{"error": text}` when `is_error`; images inside become multimodal `functionResponse.parts` |
| `thinking`, `redacted_thinking` | dropped: Claude's signed reasoning has no Gemini representation |
| anything else, URL sources | rejected with `400` |

- Tools are sent as one Gemini tool with every function declaration. `input_schema` is passed unmodified as `parametersJsonSchema` (full JSON Schema), not the OpenAPI-subset `parameters` field, which rejects keywords Claude Code schemas use (`$schema`, `additionalProperties`, `allOf`, `exclusiveMinimum`, ...). Server tools (typed `web_search_*`, `code_execution_*`, ...) are rejected.
- `tool_choice` `auto` / `any` / `none` / `tool` map to function calling modes `AUTO` / `ANY` / `NONE` / `ANY` restricted to the named function.
- `max_tokens`, `temperature`, `top_p`, `top_k` and `stop_sequences` map to the generation config. `thinking`, `metadata`, `context_management` and `cache_control` are ignored.
- `output_config.effort` (`low`, `medium`, `high`, `xhigh`, `max`; Claude Code sends `medium` by default) maps to Gemini thinking levels `LOW` / `MEDIUM` / `HIGH` / `XHIGH` / `XHIGH`. The Antigravity ACP route applies it as `agy`'s `thought_level` when the session offers that level. The Gemini API / Vertex AI route sends it as `generationConfig.thinkingConfig.thinkingLevel`. The levels have not been verified against the live API for each model, so if Gemini rejects a request with HTTP `400` whose message names the thinking configuration, NeoSwitch resends it once without the level and stops sending a level for that model until the daemon restarts. For streams the resend happens only when the rejection arrives before the first chunk. No request without an effort carries a thinking config. Unknown effort values are ignored.

## Tool call IDs and thought signatures

Gemini 3+ attaches a `thoughtSignature` to function calls and rejects a later request whose history replays the call without it (finish reason `MISSING_THOUGHT_SIGNATURE`). Claude Code keeps only Anthropic blocks in its history, so NeoSwitch embeds the signature and Gemini's call ID in the `tool_use` ID it mints: `toolu_ns_` followed by base64url JSON. The ID is restricted to `[A-Za-z0-9_-]`, needs no server-side state, and survives proxy restarts and `claude --resume`. Tool IDs not minted by NeoSwitch carry no signature.

## Response translation

- Text parts become `text` blocks; `thought` parts are skipped; each `functionCall` becomes a `tool_use` block. Responses containing a tool call have `stop_reason: "tool_use"`.
- Finish reasons: `STOP` → `end_turn` / `tool_use`; `MAX_TOKENS` → `max_tokens`; safety, recitation and blocklist reasons → `refusal`. A blocked prompt (no candidates, `promptFeedback.blockReason`) is a `refusal` with empty content. Finish reasons that mean the turn failed (`MALFORMED_FUNCTION_CALL`, `UNEXPECTED_TOOL_CALL`, `MISSING_THOUGHT_SIGNATURE`, `OTHER`, ...) are errors, never a silent `end_turn`.
- Usage: `input_tokens` = prompt tokens minus cached tokens, `cache_read_input_tokens` = cached tokens, `output_tokens` = candidate plus thinking tokens.

## Streaming

`stream: true` relays Gemini's SSE stream as it arrives. NeoSwitch waits for the first Gemini chunk before writing anything, so failures before generation starts keep a real HTTP status. Event order:

```text
message_start
ping
(content_block_start content_block_delta+ content_block_stop)*
message_delta
message_stop
```

Consecutive text chunks share one text block. Gemini delivers each function call whole (argument streaming is not requested), so a `tool_use` block is emitted as start, a single complete `input_json_delta`, stop. A failure after streaming started ends the stream with an Anthropic `error` event and no `message_stop`.

## Token counting

On Vertex AI the system instruction and tools are counted natively. The Gemini Developer API countTokens call accepts contents only, so NeoSwitch prepends the system instruction and the serialized tool declarations as contents; the result is an estimate within the framing tokens Gemini adds around them.

## Failure policy

| Upstream | NeoSwitch response |
|---|---|
| translation error (unsupported block, unknown tool_use_id) | `400 invalid_request_error`, before any upstream call |
| Gemini 400 | `400 invalid_request_error`; token-limit errors are prefixed `prompt is too long:` so Claude Code offers compaction |
| Gemini 401 / 403 / 404 / 413 | same status, `authentication_error` / `permission_error` / `not_found_error` / `request_too_large` |
| Gemini 429 | `429 rate_limit_error` |
| Gemini 503 | `529 overloaded_error` |
| other Gemini 5xx | same status, `api_error` |
| no response within 10 minutes | `504 api_error` |
| network failure, unusable response | `502 api_error` |

- Missing credentials fail at startup.
- No automatic retry or model fallback: Claude Code owns retries; genai's retry option is left unset.
- Logs contain no prompts, tool arguments, credentials or responses.

## Compatibility matrix

| Scenario | Status | Acceptance check |
|---|---|---|
| Text, non-streaming and streaming | Supported | `server` tests; deltas merged into one block; e2e `text` |
| System prompt and system messages | Supported | `translate` tests |
| Claude Code built-in tool schemas | Supported | `testdata/claude-code-tools.json`, captured from Claude Code 2.1.329, passes unmodified; every e2e scenario sends the tools Claude Code really declares |
| Tool call → tool result → answer | Supported | `provider` wire test replays the signature through genai serialization; e2e `read`, `edit and write` and `bash` check the result content, the replayed signature and the file changed on disk |
| Parallel tool calls | Supported | Each call gets its own ID; results are sent in call order (`translate` test, e2e `parallel tool calls`) |
| Images and PDFs, including inside tool results | Supported | `translate` tests |
| Upstream errors | Supported | Status mapping table test |
| `count_tokens` | Supported, estimated on the Developer API | `provider` and `server` tests |
| Session resume | Supported | Stateless tool IDs; e2e `resume` checks that `claude -p --resume` replays the earlier call with its signature and its result |
| Claude Code permissions | Supported | e2e `permission denial`: a refused Bash call reaches the model as an `error` function response and the command does not run |
| Hooks | Supported | e2e `hooks`: project `PreToolUse` and `PostToolUse` hooks fire for a model tool call and receive the NeoSwitch tool ID |
| Skills | Supported | e2e `skill`: a project skill is advertised, invoked through the Skill tool and its instructions reach the model |
| Subagents | Supported | e2e `subagent`: the Agent tool's subagent sends its own request and its answer returns to the parent |
| MCP tools | Supported | e2e `mcp`: a deferred MCP tool from `claude mcp serve` is loaded with ToolSearch and called |
| Antigravity OAuth via ACP | Supported as an opt-in local adapter | Uses the official `agy acp` process through `github.com/coder/acp-go-sdk` and never reads OAuth tokens. Concurrent sessions, streamed text, usage and stop reasons are passed through. `provider` tests drive a scripted ACP server for concurrency, streaming with split escapes, tool calls, images, cancellation, agent exit and a stuck agent. Images are sent when `agy` advertises them; PDFs are marked as omitted |

The e2e scenarios live in `e2e/` behind the `e2e` build tag and run in CI. Each runs the real Claude Code CLI (`claude -p`, with a temporary home and config directory) through an in-process NeoSwitch proxy against a scripted Gemini server, which returns function calls with thought signatures and answers 400 when the next request does not carry what Claude Code should have sent back. They need no Gemini key or Anthropic login and cover the Gemini API path only, not the Antigravity ACP route. Run them with `go test -tags e2e ./e2e/`; `CLAUDE_CLI_PATH` selects the Claude Code executable.
