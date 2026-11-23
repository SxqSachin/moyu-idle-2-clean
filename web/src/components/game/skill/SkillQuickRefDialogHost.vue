<template>
  <GameDialog
    v-model="dialogVisible"
    width="560px"
    destroy-on-close
  >
    <SkillQuickRefDetailContent
      :skill-id="dialogStore.skillId"
      :remaining-cooldown="dialogStore.remainingCooldown"
      :total-cooldown="dialogStore.totalCooldown"
      :unit-attributes="dialogStore.unitAttributes"
      :unit-skill-level="dialogStore.unitSkillLevel"
      :unit-skill-exp="dialogStore.unitSkillExp"
      :runtime-skill="dialogStore.runtimeSkill"
    />
  </GameDialog>
</template>

<script setup lang="ts">
import { computed } from "vue"
import SkillQuickRefDetailContent from "@components/game/skill/SkillQuickRefDetailContent.vue"
import { useSkillQuickRefDialogStore } from "@stores/skill-quick-ref-dialog"
import GameDialog from "@components/game/common/GameDialog.vue"

const dialogStore = useSkillQuickRefDialogStore()
const dialogVisible = computed({
  get() {
    return dialogStore.visible
  },
  set(value: boolean) {
    if (!value) {
      dialogStore.close()
    }
  }
})
</script>
