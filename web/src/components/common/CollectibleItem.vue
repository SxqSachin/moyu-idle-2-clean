<template>
  <el-card body-class="flex items-start items-stretch gap-2 " shadow="hover">
    <div class="shrink-0">
      <ItemIcon :item-id="itemId" :size="iconSize" />
    </div>
    <div class="min-w-0 flex flex-col justify-between">
      <div class="flex flex-col">
        <div class="flex items-center gap-1.5">
          <span class="truncate text-sm font-medium leading-5">{{ displayName }}</span>
          <el-tag v-if="level > 0" size="small" type="success">Lv.{{ level }}</el-tag>
        </div>
        <div v-if="displayDesc" class="mt-0.5 truncate text-xs leading-4 text-[var(--app-muted)]">{{ displayDesc }}</div>
      </div>

      <slot />
      <div v-if="displayDate" class="mt-1 text-[10px] leading-3 text-[var(--app-muted)]">{{ t("collectible.obtainedAt") }}: {{ displayDate }}</div>
    </div>
  </el-card>
</template>

<script setup lang="ts">
import { computed } from "vue"
import { useI18n } from "vue-i18n"
import ItemIcon from "@components/game/icon/ItemIcon.vue"

const props = withDefaults(
  defineProps<{
    itemId: string
    nameKey: string
    descKey?: string
    fallbackName?: string
    obtainedAt?: string
    iconSize?: number
    showDate?: boolean
    level?: number
  }>(),
  {
    descKey: "",
    fallbackName: "",
    obtainedAt: "",
    iconSize: 40,
    showDate: true,
    level: 0
  }
)

const { t } = useI18n()

const displayName = computed(() => {
  const key = String(props.nameKey || "").trim()
  if (!key) return props.fallbackName || props.itemId
  const translated = t(key, props.fallbackName || "")
  if (!translated || translated === key) return props.fallbackName || props.itemId
  return translated
})

const displayDesc = computed(() => {
  const key = String(props.descKey || "").trim()
  if (!key) return ""
  const translated = t(key, "")
  if (!translated || translated === key) return ""
  return translated
})

const displayDate = computed(() => {
  if (!props.showDate || !props.obtainedAt) return ""
  const date = new Date(props.obtainedAt)
  if (Number.isNaN(date.getTime())) return ""
  return date.toLocaleDateString()
})
</script>
