# Security Policy

## Supported versions

Security fixes go into the latest release on the [Releases](https://github.com/yldm-ai/neon-switch/releases) page and the current `main` branch. Older releases are not patched; upgrade to the latest release to get fixes.

## Reporting a vulnerability

Report vulnerabilities privately through GitHub's private vulnerability reporting: <https://github.com/yldm-ai/neon-switch/security/advisories/new>. Only the repository maintainers can read the report.

Do not open a public issue, pull request or discussion for a suspected vulnerability. Include the NeoSwitch version (`neoswitch version`), your platform, the backend in use (Antigravity OAuth, Gemini API or Vertex AI), steps to reproduce, and the impact you expect. Remove API keys, tokens, prompts and other private data from logs before attaching them.

The maintainers follow up in the private advisory, where the fix and its disclosure are coordinated with you.

## Security model

- **Loopback-only proxy with a random token**: the plugin's shared daemon and `neoswitch claude` listen on `127.0.0.1` on a port picked at startup, and every request must carry a random token. The plugin's token is stored in the user-private state file `proxy.json` (mode `0600`) and in the project's `.claude/settings.local.json`; `neoswitch on` warns when that file is not git-ignored.
- **No conversation content or credentials in logs**: the daemon log records startup and error messages only. Prompts, responses, API keys and OAuth tokens are not logged; ACP diagnostics that can carry raw message text are dropped.
- **OAuth tokens stay in `agy`**: with Antigravity OAuth, sign-in runs through the official `agy` CLI's ACP `authenticate` flow, and `agy` keeps the credentials. NeoSwitch does not read, copy, export or relay them, and calls no private Antigravity HTTP API.
- **Tools run only in Claude Code**: `agy`'s own file, shell and web tools are refused on every request, so tool calls are executed by Claude Code under its permission rules.
- **No remote relay**: requests go from the local proxy straight to the Gemini API, Vertex AI or the local `agy` process.

## In scope

- Bypassing the proxy token, or the proxy accepting connections from other hosts in its default configuration.
- Prompts, responses or credentials written to logs, state files or other files NeoSwitch creates.
- NeoSwitch exposing, copying or forwarding Antigravity OAuth tokens, API keys or Application Default Credentials.
- `agy` tools running despite the refusal, or any other way a request makes NeoSwitch execute commands or access files.
- Changes to `.claude/settings.local.json` beyond the entries NeoSwitch documents.

## Out of scope

- Vulnerabilities in Claude Code, `agy`, the Gemini API, Vertex AI or the Google SDKs themselves; report those to their vendors.
- Account actions taken by Google under the Antigravity Terms of Service, which the README warns about.
- `neoswitch serve` run without `NEOSWITCH_AUTH_TOKEN` or with `NEOSWITCH_LISTEN` set to a non-loopback address; that exposure is the operator's choice.
- Attacks that require an attacker who already runs code as your user or can read your home directory.
- Model output quality, refusals or prompt injection in model responses; Claude Code's permission rules decide what tool calls may run.
