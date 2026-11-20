---
description: Switch this project back to Claude. /neoswitch:off all does it for every project NeoSwitch switched; run that before removing the plugin
argument-hint: "[all]"
disable-model-invocation: true
allowed-tools: Bash(${CLAUDE_PLUGIN_ROOT}/plugin/neoswitch.sh *)
---

!`${CLAUDE_PLUGIN_ROOT}/plugin/neoswitch.sh off $ARGUMENTS "${CLAUDE_PROJECT_DIR}"`

Tell the user the result above in their language, briefly. Do not run any other command.
