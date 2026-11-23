<template>
  <ArenaReplayDialog
    battle-kind="combat_trial"
    :model-value="state.visible"
    :run-id="state.runId"
    :share-token="state.shareToken"
    :live="state.live"
    :version="state.version"
    @update:model-value="handleVisibility"
  />
</template>

<script setup lang="ts">
import ArenaReplayDialog from "@views/game/arena/ArenaReplayDialog.vue"
import { closeCombatTrialReplay, combatTrialReplayViewerState as state } from "@services/combat-trial-replay-viewer"

function handleVisibility(visible: boolean) {
  if (!visible) closeCombatTrialReplay()
}
</script>
