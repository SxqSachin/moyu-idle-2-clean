<template>
  <span v-if="!actionId">{{ renderedText }}</span>
  <span v-else class="inline-flex flex-wrap items-center gap-1">
    <template v-for="(part, i) in textParts" :key="i">
      <ActionQuickRef v-if="part === ACTION_SLOT" :action-id="actionId" />
      <span v-else>{{ part }}</span>
    </template>
  </span>
</template>

<script setup lang="ts">
import { computed } from "vue"
import { useI18n } from "vue-i18n"
import ActionQuickRef from "@components/game/action/ActionQuickRef.vue"

const ACTION_SLOT = "__action__"

const props = defineProps<{
  contentKey: string
  contentParams?: Record<string, unknown>
  actionId?: string
}>()

const { t } = useI18n()

const renderedText = computed(() => {
  const key = props.contentKey?.trim()
  if (!key) return "-"
  return t(key, props.contentParams ?? {})
})

const textParts = computed(() => {
  if (!props.actionId) return [renderedText.value]
  const marker = `[${props.actionId}]`
  const raw = renderedText.value
  const segments = raw.split(marker)
  const result: string[] = []
  segments.forEach((seg, i) => {
    if (seg) result.push(seg)
    if (i < segments.length - 1) result.push(ACTION_SLOT)
  })
  return result
})
</script>
