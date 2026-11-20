#!/bin/sh
# Runs the NeoSwitch CLI bundled with this plugin. The plugin cache holds the Go source, so the binary is built on first use (and rebuilt when the source is newer) into the user's cache directory.
set -eu
root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
cache="${XDG_CACHE_HOME:-$HOME/.cache}/neoswitch"
bin="$cache/neoswitch-$(printf '%s' "$root" | cksum | cut -d' ' -f1)"
if [ ! -x "$bin" ] || [ -n "$(find "$root/cmd" "$root/internal" "$root/go.mod" "$root/go.sum" -newer "$bin" \( -name '*.go' -o -name 'go.*' \) -print 2>/dev/null | head -n 1)" ]; then
	if ! command -v go >/dev/null 2>&1; then
		echo "NeoSwitch builds itself with Go on first use; install Go from https://go.dev/dl and try again." >&2
		exit 1
	fi
	mkdir -p "$cache"
	(cd "$root" && go build -trimpath -o "$bin.tmp" ./cmd/neoswitch) >&2
	mv "$bin.tmp" "$bin"
fi
exec "$bin" "$@"
