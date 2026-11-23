<template>
  <div
    v-if="source"
    class="inherit-source-cell"
    :class="[`is-${side}`, `decision-${decision}`, { 'is-effect': source.componentKind === 'effect' }]"
  >
    <div class="inherit-source-cell__main">
      <div class="inherit-source-cell__identity">
        <QuickRef v-if="source.itemId" :item-id="source.itemId" />
        <AffixQuickRef
          v-else-if="source.affixId"
          :affix-id="source.affixId"
          :slot="source.affixSlot || null"
          :show-instance-details="false"
        />
        <el-tag v-else size="small" effect="plain" type="info">{{ source.sourceLabel }}</el-tag>
        <span v-if="source.sourceNote" class="inherit-source-cell__note" :title="source.sourceNote">{{ source.sourceNote }}</span>
      </div>

      <div class="inherit-source-cell__value" :title="source.rangeText">
        <span v-if="source.attr" class="inherit-source-cell__attr" :class="valueToneClass">
          <AttributeIcon :attr-key="source.attr" :size="14" />
          <span>{{ source.attrLabel }}</span>
          <strong>{{ source.valueText }}</strong>
        </span>
        <span v-else class="inherit-source-cell__attr is-effect">{{ source.effectLabel || effectLabel }}</span>
      </div>

      <div v-if="!source.attr" class="inherit-source-cell__range">{{ effectHint }}</div>

      <div v-if="source.compositionItemIds?.length" class="inherit-source-cell__composition">
        <span>{{ t("alchemy.inherit.comboParts") }}</span>
        <QuickRef v-for="itemId in source.compositionItemIds" :key="itemId" :item-id="itemId" />
      </div>
    </div>

    <div class="inherit-source-cell__decision-rail">
      <el-radio-group
        class="inherit-source-cell__decision"
        size="small"
        :model-value="decision"
        @update:model-value="(value) => emit('update:decision', value as SourceDecision)"
      >
        <el-popover
          v-for="option in decisionOptions"
          :key="option.value"
          placement="left"
          :width="260"
          trigger="hover"
          :show-after="180"
        >
          <template #reference>
            <el-radio-button :value="option.value">
              <span class="inherit-source-cell__decision-label">
                <span>{{ option.label }}</span>
                <el-icon class="inherit-help-icon"><QuestionFilled /></el-icon>
              </span>
            </el-radio-button>
          </template>
          <div class="inherit-source-cell__decision-popover">
            <strong>{{ option.label }}</strong>
            <span>{{ decisionHint(source, option.value) }}</span>
          </div>
        </el-popover>
      </el-radio-group>
    </div>
  </div>
  <div v-else class="inherit-source-cell is-empty">
    {{ emptyText }}
  </div>
</template>

<script setup lang="ts">
import { computed } from "vue"
import { useI18n } from "vue-i18n"
import { QuestionFilled } from "@element-plus/icons-vue"
import AttributeIcon from "@components/game/icon/AttributeIcon.vue"
import AffixQuickRef from "@components/game/affix/AffixQuickRef.vue"
import QuickRef from "@components/game/item/QuickRef.vue"
import type { EquipmentAffixSlot } from "@/types/equipment"

type SourceDecision = "inherit" | "suppress" | "ignore"

type DecisionOption = {
  label: string
  value: SourceDecision
}

type InheritSourceCellSource = {
  itemId?: string
  affixId?: string
  affixSlot?: EquipmentAffixSlot | null
  sourceLabel: string
  sourceNote?: string
  effectLabel?: string
  compositionItemIds?: string[]
  attr: string
  attrLabel: string
  componentKind: "attr" | "effect"
  currentValue: number
  valueText: string
  rangeText: string
}

const props = defineProps<{
  source: InheritSourceCellSource | null
  side: "father" | "mother"
  decision: SourceDecision
  decisionOptions: DecisionOption[]
  decisionHint: (source: InheritSourceCellSource, decision: SourceDecision) => string
  emptyText: string
}>()

const emit = defineEmits<{
  (e: "update:decision", value: SourceDecision): void
}>()

const { t } = useI18n()
const effectLabel = computed(() => t("alchemy.inherit.effectSource"))
const effectHint = computed(() => t("alchemy.inherit.effectSourceHint"))
const valueToneClass = computed(() => {
  const value = Number(props.source?.currentValue ?? 0)
  if (!Number.isFinite(value) || value === 0) return "is-neutral-value"
  return value > 0 ? "is-positive-value" : "is-negative-value"
})
</script>

<style scoped>
.inherit-source-cell {
  min-width: 0;
  display: grid;
  grid-template-columns: minmax(0, 1fr) 66px;
  gap: 6px;
  align-items: center;
  padding: 6px 8px;
  border: 1px solid color-mix(in srgb, var(--app-border, #dde4ea) 80%, transparent);
  border-radius: 6px;
  background: var(--app-surface, #fff);
  transition: border-color 120ms ease, background-color 120ms ease;
}

.inherit-source-cell:hover {
  border-color: color-mix(in srgb, var(--el-color-primary, #409eff) 24%, var(--app-border, #dde4ea));
  background: color-mix(in srgb, var(--app-surface, #fff) 92%, var(--app-bg, #f6f8fb));
}

.inherit-source-cell.decision-inherit {
  border-color: color-mix(in srgb, var(--el-color-success, #67c23a) 36%, var(--app-border, #dde4ea));
}

.inherit-source-cell.decision-suppress {
  border-color: color-mix(in srgb, var(--el-color-danger, #f56c6c) 34%, var(--app-border, #dde4ea));
}

.inherit-source-cell.decision-ignore {
  border-color: color-mix(in srgb, var(--el-color-primary, #409eff) 16%, var(--app-border, #dde4ea));
}

.inherit-source-cell.is-empty {
  min-height: 50px;
  place-items: center;
  color: var(--app-muted, #667085);
  font-size: 12px;
  background: color-mix(in srgb, var(--app-surface, #fff) 88%, #eef2f6);
}

.inherit-source-cell__main {
  min-width: 0;
  display: grid;
  gap: 4px;
}

.inherit-source-cell__identity,
.inherit-source-cell__value,
.inherit-source-cell__attr {
  min-width: 0;
  display: flex;
  align-items: center;
}

.inherit-source-cell__identity {
  gap: 5px;
}

.inherit-source-cell__identity :deep(.el-tag) {
  max-width: 100%;
  min-height: 20px;
}

.inherit-source-cell__identity :deep(.item-quick-ref-name),
.inherit-source-cell__identity :deep(.affix-quick-ref-content) {
  max-width: 150px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.inherit-source-cell__note {
  min-width: 0;
  color: var(--app-muted, #667085);
  font-size: 10px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.inherit-source-cell__value {
  gap: 6px;
  font-size: 12px;
}

.inherit-source-cell__attr {
  gap: 4px;
  color: var(--app-text, #1f2328);
  font-weight: 700;
}

.inherit-source-cell__attr.is-positive-value {
  color: #047857;
}

.inherit-source-cell__attr.is-negative-value {
  color: #e11d48;
}

.dark .inherit-source-cell__attr.is-positive-value {
  color: #6ee7b7;
}

.dark .inherit-source-cell__attr.is-negative-value {
  color: #fb7185;
}

.inherit-source-cell__attr.is-effect {
  color: var(--app-muted, #667085);
}

.inherit-source-cell__attr strong {
  flex: 0 0 auto;
  margin-left: auto;
  font-size: 12px;
  font-weight: 800;
}

.inherit-source-cell__range {
  min-width: 0;
  color: var(--app-muted, #667085);
  font-size: 11px;
  line-height: 1.35;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.inherit-source-cell__composition {
  min-width: 0;
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 3px;
  color: var(--app-muted, #667085);
  font-size: 11px;
  line-height: 1.35;
}

.inherit-source-cell__composition > span {
  flex: 0 0 auto;
  font-weight: 700;
}

.inherit-source-cell__composition :deep(.item-quick-ref) {
  min-height: 20px;
}

.inherit-source-cell__composition :deep(.item-quick-ref-name) {
  max-width: 92px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.inherit-source-cell__decision-rail {
  min-width: 0;
  display: grid;
  gap: 3px;
  align-content: center;
}

.inherit-source-cell__decision {
  display: grid;
  gap: 2px;
}

.inherit-source-cell__decision :deep(.el-radio-button),
.inherit-source-cell__decision :deep(.el-popover__reference-wrapper) {
  display: block;
}

.inherit-source-cell__decision :deep(.el-radio-button__inner) {
  width: 100%;
  min-height: 22px;
  padding: 3px 4px;
  border: 1px solid color-mix(in srgb, var(--el-border-color, #dcdfe6) 84%, transparent);
  border-radius: 4px;
  background: transparent;
  box-shadow: none;
  font-size: 10px;
  font-weight: 700;
  line-height: 1.2;
}

.inherit-source-cell__decision :deep(.el-radio-button:first-child .el-radio-button__inner),
.inherit-source-cell__decision :deep(.el-radio-button:last-child .el-radio-button__inner) {
  border-radius: 4px;
}

.inherit-source-cell__decision :deep(.el-radio-button + .el-radio-button .el-radio-button__inner) {
  margin-left: 0;
}

.inherit-source-cell__decision :deep(.el-radio-button__original-radio:checked + .el-radio-button__inner) {
  box-shadow: none;
}

.inherit-source-cell__decision-label {
  min-width: 0;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 2px;
}

.inherit-help-icon {
  flex: 0 0 auto;
  color: currentColor;
  font-size: 9px;
  opacity: 0.78;
  cursor: help;
}

.inherit-source-cell__decision-popover {
  display: grid;
  gap: 4px;
  color: var(--app-text, #1f2328);
  font-size: 12px;
  line-height: 1.45;
}

.inherit-source-cell__decision-popover span {
  color: var(--app-muted, #667085);
}

@media (max-width: 700px) {
  .inherit-source-cell {
    grid-template-columns: minmax(0, 1fr) 72px;
    gap: 8px;
  }

  .inherit-source-cell__decision :deep(.el-radio-button__inner) {
    min-height: 28px;
  }
}
</style>
