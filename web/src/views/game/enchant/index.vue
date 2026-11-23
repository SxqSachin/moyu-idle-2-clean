<template>
  <EquipmentAffixOperationShell
    :title="t('game.enchant')"
    :loading="loading"
    :target-label="targetLabel"
    @back="goWarehouse"
    @refresh="handleRefresh"
  >
    <AffixEnchantPanel :detail="detail" :target-instance-id="target?.instanceId ?? null" @submitted="handleSubmitted" />
  </EquipmentAffixOperationShell>
</template>

<script setup lang="ts">
import { computed } from "vue"
import { ElMessage } from "element-plus"
import AffixEnchantPanel from "@components/game/equipment-affix/AffixEnchantPanel.vue"
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
