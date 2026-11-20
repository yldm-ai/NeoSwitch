#!/bin/sh
# SessionStart hook. Claude Code waits for it before the first model request, so when this project is routed through NeoSwitch the shared daemon is running (and this session registered with it) before it is needed. Sessions not routed through a local proxy return immediately without building anything.
case "${ANTHROPIC_BASE_URL:-}" in
http://127.0.0.1:*) ;;
*) exit 0 ;;
esac
exec "$(dirname -- "$0")/neoswitch.sh" attach --pid "$1"
