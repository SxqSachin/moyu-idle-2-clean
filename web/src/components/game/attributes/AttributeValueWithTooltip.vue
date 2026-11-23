<template>
  <div class="inline-flex items-center gap-2">
    <span>{{ name }}:</span>
    <el-popover
      v-if="Boolean(desc) || compositionLines.length > 0 || impactLines.length > 0"
      placement="top"
      trigger="hover"
      :width="260"
    >
      <template #reference>
        <span class="cursor-help">{{ value }}<span v-if="valueSuffix" class="app-muted">{{ valueSuffix }}</span></span>
      </template>
      <div class="attribute-tooltip-content">
        <div v-if="desc">
          <div class="attribute-tooltip-title">{{ t("game.attrTooltipDescTitle") }}</div>
          <div class="attribute-tooltip-muted">{{ desc }}</div>
        </div>
        <div v-if="compositionLines.length > 0">
          <div class="attribute-tooltip-title">{{ t("game.attrTooltipCompositionTitle") }}</div>
          <div>{{ compositionLines.join("\n") }}</div>
        </div>
        <div v-if="impactLines.length > 0" class="flex flex-col gap-1">
          <div class="attribute-tooltip-title">{{ t("game.attrTooltipImpactTitle") }}</div>
          <div class="">
            <div>{{ t("game.attrPerPointImpactTitle") }}</div>
            <div v-for="str in impactLines">
              - {{ str }}
            </div>
          </div>
        </div>
      </div>
    </el-popover>
    <span v-else>{{ value }}<span v-if="valueSuffix" class="app-muted">{{ valueSuffix }}</span></span>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from "vue-i18n"

const { t } = useI18n()

defineProps<{
  name: string
  desc?: string
  value: string
  valueSuffix?: string
  compositionLines: string[]
  impactLines: string[]
}>()
</script>

<style scoped>
.attribute-tooltip-content {
  display: grid;
  gap: 6px;
  color: var(--el-text-color-regular);
  font-size: 12px;
  white-space: pre-wrap;
}

.attribute-tooltip-title {
  color: var(--app-text);
  font-weight: 600;
}

.attribute-tooltip-muted {
  color: var(--app-muted);
}
</style>
