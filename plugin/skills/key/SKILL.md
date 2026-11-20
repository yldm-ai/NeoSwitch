---
description: Use a Gemini API key, the setup without the account risk of /neoswitch:login. You paste the key in a page that opens in your browser, and this project switches to Gemini 4 Neon
disable-model-invocation: true
allowed-tools: Bash(${CLAUDE_PLUGIN_ROOT}/plugin/neoswitch.sh *)
---

!`${CLAUDE_PLUGIN_ROOT}/plugin/neoswitch.sh key "${CLAUDE_PROJECT_DIR}"`

Tell the user the result above in their language, briefly, and repeat the address if one is shown. The key is entered only in that page: never ask for it, and if the user pastes it here, tell them to use the page instead. Do not run any other command.
