<template>
  <GameDialog
    v-model="dialogVisible"
    :title="t('game.battleEnemyQuickRefDialogTitle')"
    width="640px"
    destroy-on-close
  >
    <EnemyUnitQuickRefDetailContent
      :enemy-id="dialogStore.enemyId"
      :difficulty-context="dialogStore.difficultyContext"
    />
  </GameDialog>
</template>

<script setup lang="ts">
import { computed } from "vue"
import { useI18n } from "vue-i18n"
import EnemyUnitQuickRefDetailContent from "@components/game/combat/EnemyUnitQuickRefDetailContent.vue"
import { useEnemyUnitQuickRefDialogStore } from "@stores/enemy-unit-quick-ref-dialog"
import GameDialog from "@components/game/common/GameDialog.vue"

const { t } = useI18n()
const dialogStore = useEnemyUnitQuickRefDialogStore()

const dialogVisible = computed({
  get() {
    return dialogStore.visible
  },
  set(value: boolean) {
    if (!value) dialogStore.close()
  }
})
</script>
