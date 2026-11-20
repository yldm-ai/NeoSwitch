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

This delegates the interactive browser OAuth flow to the official `agy` CLI and keeps its credentials in the platform keyring. The command does not read or print the resulting token. In an SSH session, `agy` prints a URL and an authorization code for completing the same flow in a local browser. `neoswitch antigravity oauth` is accepted as an alias.

The probe launches the official CLI in headless JSON mode with a fixed, non-sensitive prompt. It captures the result in memory, checks for an exact success response, and prints only whether the session is usable. It never prints `stdout`, `stderr`, an account email, a token, or usage details.

This command is a login/provider check, not a Claude Code backend. `agy -p` and `agy acp` run the Antigravity agent and its own tool/permission loop; they do not expose the raw Gemini model contract that NeoSwitch needs to preserve Claude Code's tool loop.

If the probe fails, run `neoswitch antigravity login` and retry the probe. Do not copy the token into `GEMINI_API_KEY`.

The stable NeoSwitch path remains:

```text
Claude Code → NeoSwitch Anthropic-compatible proxy → Gemini API/Vertex API → Gemini 4 Neon
```

An eventual `antigravity-experimental` adapter would need a separate compatibility contract and would never be enabled by default.
