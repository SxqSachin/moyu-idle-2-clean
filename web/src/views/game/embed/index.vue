<template>
  <EquipmentAffixOperationShell
    :title="t('game.itemDetailAttachAffix')"
    :loading="loading"
    :target-label="targetLabel"
    @back="goWarehouse"
    @refresh="handleRefresh"
  >
    <AffixAttachPanel :detail="detail" :target-instance-id="target?.instanceId ?? null" @submitted="handleSubmitted" />
  </EquipmentAffixOperationShell>
</template>

<script setup lang="ts">
import { computed } from "vue"
import { ElMessage } from "element-plus"
import AffixAttachPanel from "@components/game/equipment-affix/AffixAttachPanel.vue"
import EquipmentAffixOperationShell from "@components/game/equipment-affix/EquipmentAffixOperationShell.vue"
import { useEquipmentAffixOperation } from "@composables/game/useEquipmentAffixOperation"

const { t, loading, detail, target, itemLabel, goWarehouse, refreshAll } = useEquipmentAffixOperation()

const targetLabel = computed(() => {
  if (!target.value) return ""
  return `${itemLabel(target.value.templateItemId)} #${target.value.instanceId}`
})

async function handleRefresh() {
  try {
    await refreshAll()
  } catch (err) {
    ElMessage.error(err instanceof Error ? err.message : t("game.itemDetailLoadFailed"))
  }
}

async function handleSubmitted() {
  await handleRefresh()
}
</script>
