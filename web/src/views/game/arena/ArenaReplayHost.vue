<template>
  <ArenaReplayDialog
    :model-value="state.visible"
    :challenge-id="state.challengeId"
    :share-token="state.shareToken"
    :live="state.live"
    :version="state.version"
    @update:model-value="handleVisibility"
  />
</template>

<script setup lang="ts">
import ArenaReplayDialog from "./ArenaReplayDialog.vue"
import { arenaReplayViewerState as state, closeArenaReplay } from "@services/arena-replay-viewer"

function handleVisibility(visible: boolean) {
  if (!visible) closeArenaReplay()
}
</script>
