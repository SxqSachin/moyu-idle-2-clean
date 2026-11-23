<template>
  <el-tag v-if="isVisible" class="action-quick-ref" type="primary" :disable-transitions="true" @click.stop="openActionDetail">
    <span class="action-quick-ref-content">
      <TypeIcon :type="iconType" :id="iconID" :alt="actionLabel" :size="20" :fallback="iconFallback" />
      <span>{{ actionLabel }}</span>
    </span>
  </el-tag>
</template>

<script setup lang="ts">
import { computed } from "vue"
import { ElMessage } from "element-plus"
import { useI18n } from "vue-i18n"
import TypeIcon from "@components/game/icon/TypeIcon.vue"
import { getActionDefinition, isGenericActionVisible } from "@services/action-definition"
import { parseIconId, resolveFallbackIconByIconId } from "@services/item-icons"
import { useActionQuickRefDialogStore } from "@stores/action-quick-ref-dialog"

const props = defineProps<{
  actionId: string
}>()

const { t } = useI18n()
const dialogStore = useActionQuickRefDialogStore()

const actionDefinition = computed(() => getActionDefinition(props.actionId))
const isVisible = computed(() => isGenericActionVisible(actionDefinition.value))
const actionLabel = computed(() => {
  const action = actionDefinition.value
  if (action?.nameKey) {
    const localized = t(action.nameKey)
    if (localized !== action.nameKey && localized.trim() !== "") return localized
  }
  const fallbackKey = `action.${props.actionId}.name`
  const fallbackLocalized = t(fallbackKey)
  if (fallbackLocalized !== fallbackKey && fallbackLocalized.trim() !== "") return fallbackLocalized
  return props.actionId
})
const iconKey = computed(() => {
  const action = actionDefinition.value
  return action?.iconId || `action.${props.actionId}`
})
const parsedIcon = computed(() => {
  return parseIconId(iconKey.value)
})
const iconType = computed(() => parsedIcon.value?.type || "action")
const iconID = computed(() => parsedIcon.value?.id || props.actionId)
const iconFallback = computed(() => resolveFallbackIconByIconId(iconKey.value, "⚙️"))

function openActionDetail() {
  if (!isVisible.value) {
    ElMessage.warning(t("game.actionCatalogLoadFailed"))
    return
  }
  dialogStore.open({
    actionId: props.actionId,
    count: 1
  })
}
</script>

<style scoped>
.action-quick-ref {
  cursor: pointer;
  user-select: none;
}

.action-quick-ref-content {
  display: inline-flex;
  align-items: center;
  gap: 4px;
}
</style>
