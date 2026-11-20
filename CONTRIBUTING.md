# Contributing to NeoSwitch

Bug reports, fixes and documentation improvements are welcome. By taking part you agree to follow the [Code of Conduct](CODE_OF_CONDUCT.md). Report security issues privately as described in [SECURITY.md](SECURITY.md), not in public issues.

## Development setup

- Go 1.27 or newer, the version in [go.mod](go.mod).
- Claude Code, to try the plugin end to end.
- Optional: the official Antigravity CLI (`agy`) for the OAuth path, or a `GEMINI_API_KEY` or Vertex AI project for the Gemini path.

Build and run the CLI from the repository root:

```bash
go build -o neoswitch ./cmd/neoswitch
./neoswitch doctor
```

To try your checkout as a plugin, add the local directory as a marketplace inside Claude Code with `/plugin marketplace add /path/to/neon-switch`, then install `neoswitch@neoswitch`. The plugin builds the binary from that source on first use; after changing Go code, run `claude plugin marketplace update neoswitch` and `claude plugin update neoswitch@neoswitch`, then restart Claude Code.

## Before you open a pull request

Run the same checks as CI:

```bash
go test ./...
go test -race ./...
go vet ./...
```

- Add or update tests for behavior changes.
- Use [Conventional Commits](https://www.conventionalcommits.org/) for commit messages and pull request titles, for example `fix: ...`, `feat: ...`, `docs: ...`, `test: ...`, `chore: ...`.
- One issue per pull request. Write `Fixes #N` in the description; unrelated changes go in separate pull requests.
- Keep [README.md](README.md) and the nine translations in [i18n/](i18n/) in sync. A README change goes into every translation in the same pull request, with the same structure: the same headings, code blocks, tables and line count, and links that resolve from `i18n/`.
- Write one paragraph per line in Markdown and comments; do not hard-wrap text.
- Never commit credentials: API keys, OAuth tokens, `.env` files, Application Default Credentials, or a `.claude/settings.local.json` that contains the proxy token.

## Releases

Maintainers release by pushing a tag that starts with `v`, such as `v0.2.0`. The [release workflow](.github/workflows/release.yml) runs `go test -race ./...` and `go vet ./...`, builds the `neoswitch` binaries for macOS, Linux and Windows with the tag as the version, and publishes a GitHub Release with the binaries and a `checksums.txt` file of SHA-256 sums.
