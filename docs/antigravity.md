# Antigravity account sessions

NeoSwitch does not read, export, or replay the OAuth/token profile used by Antigravity. The profile belongs to the official `agy` client and is stored through the platform's secure credential layer.

The safe diagnostic path is:

```bash
neoswitch antigravity probe
```

To start or switch the account session, run:

```bash
neoswitch antigravity login
```

This delegates the ACP authentication flow to the official `agy` CLI and keeps its credentials in the platform keyring. The command does not read or print the resulting token or open the interactive `agy` TUI. In an SSH session, `agy` prints a URL and an authorization code for completing the same flow in a local browser. `neoswitch antigravity oauth` is accepted as an alias.

To use the authenticated Neon account from the Claude Code plugin, run `/neoswitch:login`. The plugin signs in through `agy acp`, enables the current project's route, and starts using the `gemini-4-neon` session model after the next Claude Code session. OAuth credentials remain inside `agy` and are never read by NeoSwitch. The **Use Antigravity OAuth via ACP** option can force this route when another Gemini credential is present.

The probe launches the official CLI in headless JSON mode with a fixed, non-sensitive prompt. It captures the result in memory, checks for an exact success response, and prints only whether the session is usable. It never prints `stdout`, `stderr`, an account email, a token, or usage details.

The ACP route uses `agy` as the model-side agent process. NeoSwitch sends the translated Claude request through an ACP session, selects `gemini-4-neon`, and translates the returned text or Claude tool call back to the local Anthropic-compatible proxy. OAuth credentials remain owned by `agy`.

If the probe fails, run `neoswitch antigravity login` and retry the probe. Do not copy the token into `GEMINI_API_KEY`.

The stable NeoSwitch path remains:

```text
Claude Code → NeoSwitch Anthropic-compatible proxy → Gemini API/Vertex API → Gemini 4 Neon
```

The ACP route is opt-in through the plugin setting because ACP is an agent protocol and does not expose the raw Gemini HTTP contract. The adapter asks the Neon agent to return Claude-compatible text or tool-call JSON, while Claude Code continues to receive Anthropic-compatible responses from NeoSwitch.
