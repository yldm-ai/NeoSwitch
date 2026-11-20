# Antigravity account sessions

> [!WARNING]
> The [Google Antigravity Terms of Service](https://antigravity.google/terms) (section 6) treat accessing Antigravity through third-party software, tools or services as a breach and say Google may suspend or terminate the Antigravity and Gemini CLI accounts involved. Routing Claude Code through Antigravity OAuth falls under that rule. Use this route only if you accept that risk; otherwise use a Gemini API key or Vertex AI.

NeoSwitch does not read, export, or replay the OAuth/token profile used by Antigravity. The profile belongs to the official `agy` client and is stored through the platform's secure credential layer.

The safe diagnostic path is:

```bash
neoswitch antigravity probe
```

To start or switch the account session, run:

```bash
neoswitch antigravity login
```

This delegates ACP's protocol-driven authentication flow to the official `agy` CLI and keeps its credentials in the platform keyring. The command does not read or print the resulting token, wait for stdin, or open the interactive `agy` TUI. `agy` owns the browser callback. `neoswitch antigravity oauth` is accepted as an alias.

To use the authenticated Neon account from the Claude Code plugin, run `/neoswitch:login`. The plugin signs in through `agy acp`, enables the current project's route, and starts using the `gemini-4-neon` session model after the next Claude Code session. OAuth credentials remain inside `agy` and are never read by NeoSwitch. The **Use Antigravity OAuth via ACP** option can force this route when another Gemini credential is present.

The probe launches the official CLI in headless JSON mode with a fixed, non-sensitive prompt. It captures the result in memory, checks for an exact success response, and prints only whether the session is usable. It never prints `stdout`, `stderr`, an account email, a token, or usage details.

The ACP route uses `agy` as the model-side agent process. NeoSwitch sends the translated Claude request through an ACP session, selects `gemini-4-neon`, and translates the returned text or Claude tool call back to the local Anthropic-compatible proxy. OAuth credentials remain owned by `agy`.

`agy` is a full coding agent with its own file, shell and web tools. NeoSwitch starts it without `--dangerously-skip-permissions` and rejects every tool permission request it sends, so those tools never run; tools run only in Claude Code, under its permission rules.

Images, including screenshots returned by Claude Code tools, are sent as ACP image blocks when `agy` advertises image prompts. PDFs and other attachments cannot travel over ACP; the prompt marks each one as omitted so the model knows it was there.

One `agy acp` process serves the daemon, and every request runs in its own ACP session. `agy` runs sessions concurrently, so subagents, background requests and several Claude Code sessions do not wait for each other. Text answers stream to Claude Code as `agy` generates them; tool calls arrive whole.

Claude Code's effort setting (`/effort`, `--effort`, `medium` by default) selects `agy`'s thought level for each request: `low`, `medium`, `high` and `xhigh` map directly and `max` uses `xhigh`. The thought level dominates the wait before the first token, so `low` answers simple questions noticeably faster. When `agy` does not offer a level, the request runs at `agy`'s default.

Answers report the input and output tokens `agy` bills, so Claude Code's context usage and automatic compaction see real numbers. `agy` adds its own agent instructions (about 14k tokens) to every prompt. That overhead is included in the reported usage, and `count_tokens` adds it to its estimate once the first answer has reported it.

Each request is bounded by the proxy's 10-minute upstream timeout. When the timeout expires or Claude Code disconnects, NeoSwitch cancels that session in `agy` and leaves the others running. If `agy` produced no output at all for a request that timed out, it is treated as stuck and restarted for the next request.

If the probe fails, run `neoswitch antigravity login` and retry the probe. Do not copy the token into `GEMINI_API_KEY`.

The stable NeoSwitch path remains:

```text
Claude Code → NeoSwitch Anthropic-compatible proxy → Gemini API/Vertex API → Gemini 4 Neon
```

The ACP route is opt-in through the plugin setting because ACP is an agent protocol and does not expose the raw Gemini HTTP contract. The adapter asks the Neon agent to return Claude-compatible text or tool-call JSON, while Claude Code continues to receive Anthropic-compatible responses from NeoSwitch.
