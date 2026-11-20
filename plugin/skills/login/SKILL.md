---
description: Sign in with your Google account through Google's Antigravity sign-in and switch this project to Gemini 4 Neon. Google may suspend accounts used this way
disable-model-invocation: true
allowed-tools: Bash(${CLAUDE_PLUGIN_ROOT}/plugin/neoswitch.sh *)
---

!`${CLAUDE_PLUGIN_ROOT}/plugin/neoswitch.sh antigravity login && ${CLAUDE_PLUGIN_ROOT}/plugin/neoswitch.sh on "${CLAUDE_PROJECT_DIR}"`

Tell the user the result above in their language, briefly. Do not run any other command.
