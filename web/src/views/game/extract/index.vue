<template>
  <EquipmentAffixOperationShell
    :title="t('game.itemDetailDetachAffix')"
    :loading="loading"
    :target-label="targetLabel"
    :notice-title="detachDestroyedNotice ? t('game.affixDetachEquipmentDestroyed') : ''"
    notice-type="warning"
    @back="goWarehouse"
    @refresh="handleRefresh"
    @notice-close="detachDestroyedNotice = false"
  >
    <AffixDetachPanel :detail="detail" :target-instance-id="target?.instanceId ?? null" @submitted="handleSubmitted" />
  </EquipmentAffixOperationShell>
</template>

<script setup lang="ts">
import { computed, ref } from "vue"
import { ElMessage } from "element-plus"
import AffixDetachPanel from "@components/game/equipment-affix/AffixDetachPanel.vue"
import EquipmentAffixOperationShell from "@components/game/equipment-affix/EquipmentAffixOperationShell.vue"
import { useEquipmentAffixOperation } from "@composables/game/useEquipmentAffixOperation"

const { t, loading, detail, target, itemLabel, goWarehouse, refreshAll, clearDestroyedTarget } = useEquipmentAffixOperation()
const detachDestroyedNotice = ref(false)

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

async function handleSubmitted(result?: { equipmentInstanceId?: number; destroyed?: boolean }) {
  try {
    if (result?.destroyed) {
      detachDestroyedNotice.value = true
      await clearDestroyedTarget(result.equipmentInstanceId)
      return
    }
    detachDestroyedNotice.value = false
    await handleRefresh()
  } catch (err) {
    ElMessage.error(err instanceof Error ? err.message : t("game.itemOperationFailed"))
  }
}
</script>
