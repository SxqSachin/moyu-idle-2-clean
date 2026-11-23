<template>
  <section class="inherit-preview" data-tutorial="alchemy-inherit-submit">
    <div class="inherit-section-head">
      <div>
        <div class="inherit-section-head__title">{{ t("alchemy.inherit.expectedResult") }}</div>
        <div class="inherit-section-head__hint">{{ t("alchemy.inherit.expectedResultHint") }}</div>
      </div>
    </div>
    <div class="inherit-preview__grid">
      <div class="inherit-preview__card">
        <span>{{ t("alchemy.inherit.previewGuaranteed") }}</span>
        <strong>{{ t("alchemy.inherit.previewGuaranteedValue", { inherit: inheritUsed, suppress: suppressUsed }) }}</strong>
      </div>
      <div class="inherit-preview__card">
        <span>{{ t("alchemy.inherit.previewLineage") }}</span>
        <strong>{{ expectedLineageText }}</strong>
      </div>
      <div class="inherit-preview__card">
        <span>{{ t("alchemy.inherit.previewFusion") }}</span>
        <strong>{{ t("alchemy.inherit.previewFusionValue", { hybrid: hybridPotentialCount, inbreed: inbreedRiskCount }) }}</strong>
      </div>
      <div class="inherit-preview__card">
        <span>{{ t("alchemy.inherit.previewIgnore") }}</span>
        <strong>{{ expectedIgnoreText }}</strong>
      </div>
    </div>
    <div class="inherit-affix-preview">
      <div class="inherit-affix-preview__head">
        <div>
          <div class="inherit-affix-preview__title">{{ t("alchemy.inherit.affixPreviewTitle") }}</div>
          <div class="inherit-affix-preview__hint">{{ t("alchemy.inherit.affixPreviewHint") }}</div>
        </div>
        <el-tag v-if="affixPreviewHasOrdinarySource" type="warning" effect="plain" size="small">
          {{ t("alchemy.inherit.orphanAffix") }}
        </el-tag>
      </div>
      <div class="inherit-affix-preview__body">
        <div class="inherit-affix-preview__name">{{ t("alchemy.inherit.affixPreviewName") }}</div>
        <div v-if="affixPreviewLines.length > 0" class="inherit-affix-preview__sections">
          <section
            v-for="section in affixPreviewSections"
            :key="section.key"
            class="inherit-affix-preview__section"
            :class="`is-${section.key}`"
          >
            <div class="inherit-affix-preview__section-title">{{ section.label }}</div>
            <div class="inherit-affix-preview__lines">
              <div
                v-for="line in section.lines"
                :key="line.key"
                class="inherit-affix-preview__line"
                :class="[`inherit-affix-preview__line--${line.status}`, `inherit-affix-preview__line--${line.tone}`]"
              >
                <div class="inherit-affix-preview__main">
                  <AttributeIcon v-if="line.attr" :attr-key="line.attr" :alt="line.label" :size="14" />
                  <span class="inherit-affix-preview__label">{{ line.label }}</span>
                  <strong v-if="line.valueText" class="inherit-affix-preview__value">{{ line.valueText }}</strong>
                </div>
                <div class="inherit-affix-preview__state">
                  <el-popover placement="top" trigger="hover" :width="190" popper-class="inherit-affix-preview-popover">
                    <template #reference>
                      <el-tag :type="tagType(line.status)" effect="plain" size="small">
                        {{ line.probabilityText || line.statusText }}
                      </el-tag>
                    </template>
                    <div class="inherit-affix-preview__state-popover">
                      <strong>{{ line.statusText }}</strong>
                      <span>{{ line.probabilityHint }}</span>
                    </div>
                  </el-popover>
                </div>
                <div v-if="line.projectionText || line.riskText || line.sourceRefs.length || line.sourceText || line.ignoreText || line.hint" class="inherit-affix-preview__details">
                  <span v-if="line.projectionText" class="inherit-affix-preview__detail inherit-affix-preview__detail--projection">
                    {{ line.projectionText }}
                  </span>
                  <span v-if="line.riskText" class="inherit-affix-preview__detail inherit-affix-preview__detail--risk">
                    {{ line.riskText }}
                  </span>
                  <span v-if="line.sourceRefs.length > 0" class="inherit-affix-preview__detail inherit-affix-preview__detail--source">
                    <span class="inherit-affix-preview__detail-label">{{ t("alchemy.inherit.affixPreviewFromLabel") }}</span>
                    <QuickRef
                      v-for="source in line.sourceRefs"
                      :key="source.key"
                      class="inherit-affix-preview__quick-ref"
                      :item-id="source.itemId"
                    />
                  </span>
                  <span v-else-if="line.sourceText" class="inherit-affix-preview__detail">
                    {{ line.sourceText }}
                  </span>
                  <span v-if="line.ignoreText" class="inherit-affix-preview__detail inherit-affix-preview__detail--ignore">
                    {{ line.ignoreText }}
                  </span>
                  <span v-if="line.hint" class="inherit-affix-preview__detail">
                    {{ line.hint }}
                  </span>
                </div>
              </div>
            </div>
          </section>
        </div>
        <div v-else class="inherit-affix-preview__empty">{{ t("alchemy.inherit.affixPreviewEmpty") }}</div>
        <div class="inherit-affix-preview__foot">{{ affixPreviewLineageText }}</div>
      </div>
    </div>
  </section>

  <section v-if="lastResult" class="inherit-preview inherit-result">
    <div class="inherit-section-head">
      <div>
        <div class="inherit-section-head__title">{{ t("alchemy.inherit.actualResult") }}</div>
        <div class="inherit-section-head__hint">{{ t("alchemy.inherit.actualResultHint") }}</div>
      </div>
    </div>
    <div class="inherit-result__body">
      <Card
        v-if="resultInstance"
        :item-id="resultInstance.templateItemId"
        :instance-id="resultInstance.instanceId"
        :quantity="resultInstance.quantity"
        :width="58"
        :height="58"
        :icon-size="34"
      />
      <AffixQuickRef
        v-if="resultAffix?.affixId"
        class="inherit-result__affix"
        :affix-id="resultAffix.affixId"
        :slot="resultAffix"
        :show-instance-details="true"
      />
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed } from "vue"
import { useI18n } from "vue-i18n"
import AttributeIcon from "@components/game/icon/AttributeIcon.vue"
import AffixQuickRef from "@components/game/affix/AffixQuickRef.vue"
import Card from "@components/game/item/Card.vue"
import QuickRef from "@components/game/item/QuickRef.vue"
import type { EquipmentAffixSlot, InventoryInstance } from "@/types/equipment"

type AffixPreviewStatus = "fixed" | "possible" | "hybrid" | "risk" | "removed"
type AffixPreviewTone = "positive" | "negative" | "neutral"
type TagType = "success" | "warning" | "info" | "primary" | "danger"

type PreviewSourceRef = {
  key: string
  label: string
  itemId: string
}

type AffixPreviewLine = {
  key: string
  attr: string
  label: string
  valueText: string
  projectionText: string
  sourceText: string
  sourceRefs: PreviewSourceRef[]
  ignoreText: string
  status: AffixPreviewStatus
  statusText: string
  probabilityText: string
  probabilityHint: string
  riskText: string
  tone: AffixPreviewTone
  hint: string
  hasOrdinarySource: boolean
}

type InheritImmediateResult = {
  targetCarrierInstanceId?: number
  affix?: EquipmentAffixSlot | null
}

const props = defineProps<{
  inheritUsed: number
  suppressUsed: number
  expectedLineageText: string
  hybridPotentialCount: number
  inbreedRiskCount: number
  expectedIgnoreText: string
  affixPreviewLines: AffixPreviewLine[]
  affixPreviewHasOrdinarySource: boolean
  affixPreviewLineageText: string
  lastResult: InheritImmediateResult | null
  resultInstance: InventoryInstance | null
  tagType: (status: AffixPreviewStatus) => TagType
}>()

const { t } = useI18n()
const resultAffix = computed(() => {
  const slots = props.resultInstance?.affixes
  if (Array.isArray(slots) && slots.length > 0) return slots[0]
  return props.lastResult?.affix || null
})
const affixPreviewSections = computed(() => {
  const buckets: Record<AffixPreviewTone, AffixPreviewLine[]> = {
    positive: [],
    negative: [],
    neutral: []
  }
  for (const line of props.affixPreviewLines) buckets[line.tone || "neutral"].push(line)
  return (["positive", "negative", "neutral"] as AffixPreviewTone[])
    .filter((tone) => buckets[tone].length > 0)
    .map((tone) => ({
      key: tone,
      label: t(`alchemy.inherit.previewSection.${tone}`),
      lines: buckets[tone]
    }))
})
</script>

<style scoped>
.inherit-preview {
  display: grid;
  gap: 8px;
}

.inherit-section-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.inherit-section-head__title {
  font-size: 16px;
  font-weight: 800;
}

.inherit-section-head__hint {
  color: var(--app-muted, #667085);
  font-size: 12px;
  line-height: 1.35;
}

.inherit-preview__grid {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  padding: 6px;
  border: 1px solid color-mix(in srgb, var(--app-border, #dde4ea) 76%, transparent);
  border-radius: 6px;
  background: color-mix(in srgb, var(--app-bg, #f6f8fb) 64%, transparent);
}

.inherit-preview__card {
  min-width: 0;
  flex: 1 1 170px;
  display: flex;
  align-items: baseline;
  gap: 6px;
  padding: 3px 5px;
  border: 0;
  border-radius: 4px;
  background: transparent;
}

.inherit-preview__card span {
  flex: 0 0 auto;
  color: var(--app-muted, #667085);
  font-size: 11px;
  font-weight: 800;
}

.inherit-preview__card strong {
  display: block;
  min-width: 0;
  font-size: 12px;
  line-height: 1.35;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.inherit-affix-preview {
  display: grid;
  gap: 8px;
  padding: 8px;
  border: 1px solid var(--app-border, #dde4ea);
  border-radius: 6px;
  background: var(--app-surface, #fff);
}

.inherit-affix-preview__head {
  display: flex;
  align-items: start;
  justify-content: space-between;
  gap: 10px;
}

.inherit-affix-preview__title {
  color: var(--app-text, #1f2328);
  font-size: 13px;
  font-weight: 900;
}

.inherit-affix-preview__hint,
.inherit-affix-preview__foot {
  color: var(--app-muted, #667085);
  font-size: 11px;
  line-height: 1.45;
}

.inherit-affix-preview__body {
  display: grid;
  gap: 6px;
  padding: 8px 9px;
  border: 1px solid color-mix(in srgb, var(--app-border, #dde4ea) 82%, transparent);
  border-radius: 6px;
  background: color-mix(in srgb, var(--app-bg, #f7f8fa) 65%, transparent);
}

.inherit-affix-preview__name {
  color: var(--app-muted, #667085);
  font-size: 12px;
  font-weight: 900;
}

.inherit-affix-preview__sections {
  display: grid;
  gap: 8px;
}

.inherit-affix-preview__section {
  display: grid;
  gap: 4px;
}

.inherit-affix-preview__section + .inherit-affix-preview__section {
  padding-top: 7px;
  border-top: 1px solid color-mix(in srgb, var(--app-border, #dde4ea) 72%, transparent);
}

.inherit-affix-preview__section-title {
  color: var(--app-muted, #667085);
  font-size: 11px;
  font-weight: 900;
}

.inherit-affix-preview__section.is-positive .inherit-affix-preview__section-title {
  color: #047857;
}

.inherit-affix-preview__section.is-negative .inherit-affix-preview__section-title {
  color: #e11d48;
}

.inherit-affix-preview__lines {
  display: grid;
  gap: 4px;
}

.inherit-affix-preview__line {
  min-width: 0;
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto;
  gap: 5px 8px;
  align-items: center;
  padding: 5px 7px;
  border-radius: 5px;
  background: var(--app-surface, #fff);
  border-left: 3px solid color-mix(in srgb, var(--app-muted, #667085) 45%, transparent);
}

.inherit-affix-preview__line--fixed {
  border-left-color: var(--el-color-primary, #409eff);
}

.inherit-affix-preview__line--hybrid {
  border-left-color: var(--el-color-success, #67c23a);
}

.inherit-affix-preview__line--risk,
.inherit-affix-preview__line--removed {
  border-left-color: var(--el-color-danger, #f56c6c);
}

.inherit-affix-preview__line--possible {
  border-left-color: var(--el-color-warning, #e6a23c);
}

.inherit-affix-preview__details {
  grid-column: 1 / -1;
  display: flex;
  flex-wrap: wrap;
  gap: 4px 5px;
  padding-top: 1px;
}

.inherit-affix-preview__detail {
  display: inline-flex;
  align-items: center;
  min-height: 19px;
  padding: 1px 6px;
  border-radius: 999px;
  background: color-mix(in srgb, var(--app-muted, #667085) 8%, transparent);
  color: var(--app-muted, #667085);
  font-size: 11px;
  line-height: 1.35;
}

.inherit-affix-preview__detail--projection {
  background: transparent;
  color: var(--app-text, #1f2328);
  font-weight: 800;
  padding-left: 0;
}

.inherit-affix-preview__detail--risk {
  background: color-mix(in srgb, var(--el-color-danger, #f56c6c) 9%, transparent);
  color: #e11d48;
  font-weight: 800;
}

.inherit-affix-preview__detail--ignore {
  background: color-mix(in srgb, var(--el-color-warning, #e6a23c) 12%, transparent);
  color: #9a5b00;
}

.inherit-affix-preview__detail--source {
  gap: 3px;
  padding: 1px 4px 1px 6px;
}

.inherit-affix-preview__detail-label {
  flex: 0 0 auto;
}

.inherit-affix-preview__quick-ref {
  max-width: 140px;
}

.inherit-affix-preview__quick-ref :deep(.el-tag) {
  height: 18px;
  padding: 0 4px;
  border: 0;
  background: color-mix(in srgb, var(--el-color-primary, #409eff) 10%, transparent);
}

.inherit-affix-preview__quick-ref :deep(.item-quick-ref) {
  gap: 2px;
}

.inherit-affix-preview__quick-ref :deep(.item-quick-ref-name) {
  max-width: 96px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-size: 11px;
}

.inherit-affix-preview__quick-ref :deep(.item-quick-ref svg),
.inherit-affix-preview__quick-ref :deep(.item-quick-ref img) {
  width: 14px;
  height: 14px;
}

.inherit-affix-preview__state {
  display: inline-flex;
  align-items: center;
  justify-content: flex-end;
  gap: 5px;
  min-width: 0;
}

:global(.inherit-affix-preview-popover) {
  padding: 8px 9px;
  min-width: 0;
}

.inherit-affix-preview__main {
  min-width: 0;
  display: flex;
  align-items: center;
  gap: 5px;
}

.inherit-affix-preview__state-popover {
  display: grid;
  gap: 3px;
  line-height: 1.35;
}

.inherit-affix-preview__state-popover strong {
  color: var(--app-text, #1f2328);
  font-size: 12px;
  font-weight: 800;
}

.inherit-affix-preview__state-popover span {
  color: var(--app-muted, #667085);
  font-size: 11px;
}

.inherit-affix-preview__label {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-size: 12px;
  font-weight: 800;
}

.inherit-affix-preview__value {
  flex: 0 0 auto;
  font-size: 12px;
  font-weight: 900;
}

.inherit-affix-preview__line--positive .inherit-affix-preview__value {
  color: #047857;
}

.inherit-affix-preview__line--negative .inherit-affix-preview__value {
  color: #e11d48;
}

.inherit-affix-preview__line--removed .inherit-affix-preview__main {
  color: var(--app-muted, #667085);
  text-decoration: line-through;
}

.inherit-affix-preview__empty {
  padding: 8px;
  border: 1px dashed var(--app-border, #dde4ea);
  border-radius: 5px;
  color: var(--app-muted, #667085);
  font-size: 12px;
  text-align: center;
}

.inherit-result {
  padding: 9px;
  border: 1px solid color-mix(in srgb, var(--el-color-success, #67c23a) 34%, var(--app-border, #dde4ea));
  border-radius: 6px;
  background: color-mix(in srgb, var(--el-color-success, #67c23a) 4%, var(--app-surface, #fff));
}

.inherit-result__body {
  display: grid;
  grid-template-columns: auto minmax(0, 1fr);
  align-items: center;
  gap: 10px;
}

.inherit-result__affix {
  min-width: 0;
}

@media (max-width: 1100px) {
  .inherit-preview__grid {
    gap: 5px;
  }
}

@media (max-width: 700px) {
  .inherit-preview__card {
    flex-basis: 100%;
  }

  .inherit-affix-preview__head,
  .inherit-affix-preview__line,
  .inherit-result__body {
    grid-template-columns: 1fr;
  }

  .inherit-affix-preview__head {
    display: grid;
  }
}
</style>
