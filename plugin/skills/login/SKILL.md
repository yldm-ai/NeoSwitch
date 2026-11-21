---
description: Sign in with your Google account through Google's Antigravity sign-in, installing Google's agy CLI first when it is missing, and switch this project to Gemini 4 Neon. It opens a browser and also shows a link, so a computer reached over SSH signs in from any device by pasting back the code; run it again to switch Google accounts. Google may suspend accounts used this way
argument-hint: "[code | cancel]"
disable-model-invocation: true
allowed-tools: Bash(${CLAUDE_PLUGIN_ROOT}/plugin/neoswitch.sh *)
---

!`${CLAUDE_PLUGIN_ROOT}/plugin/neoswitch.sh antigravity login --on "${CLAUDE_PROJECT_DIR}" '$ARGUMENTS'`

Tell the user the result above in their language, briefly. When it contains a sign-in link, keep its order: first that they can finish in the browser window if one opened, then the link, copied exactly, character for character, alone on its own line and not as a Markdown link, because a single changed character breaks the sign-in; then which command to run with the code. Do not run any other command.

The only exception: if the command above did not run because the user pasted the full address of the sign-in page instead of the code, take the value of the address's `code` parameter and run `${CLAUDE_PLUGIN_ROOT}/plugin/neoswitch.sh antigravity login --on "${CLAUDE_PROJECT_DIR}" '<that code>'` once, then report its result the same way.
