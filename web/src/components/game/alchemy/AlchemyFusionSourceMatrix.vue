<template>
  <section class="inherit-matrix-section" data-tutorial="alchemy-inherit-sources">
    <div class="inherit-section-head">
      <div>
        <div class="inherit-section-head__title">{{ t("alchemy.inherit.sourcePool") }}</div>
        <div class="inherit-section-head__hint">{{ t("alchemy.inherit.sourcePoolHintChoice") }}</div>
      </div>
      <el-tag v-if="kindMismatch" type="danger" effect="plain">{{ t("alchemy.inherit.kindMismatch") }}</el-tag>
      <el-tag v-else-if="selectedKind" type="success" effect="plain">{{ t(`alchemy.kind.${selectedKind}`, selectedKind) }}</el-tag>
    </div>

    <div v-if="sourceGroups.length === 0" class="inherit-empty">{{ t("alchemy.inherit.noSources") }}</div>
    <div v-else class="inherit-source-sections">
      <section v-for="section in sourceSections" :key="section.key" class="inherit-source-section" :class="`is-${section.key}`">
        <div class="inherit-source-section__head">
          <span>{{ section.label }}</span>
          <small>{{ section.groups.length }}</small>
        </div>
        <article v-for="group in section.groups" :key="group.key" class="inherit-source-group">
          <div class="inherit-source-group__head">
            <div class="inherit-source-group__title">
              <AttributeIcon v-if="group.attr" :attr-key="group.attr" :size="16" />
              <span>{{ group.label }}</span>
            </div>
            <span>{{ group.rows.length }}</span>
          </div>
          <div v-for="row in group.rows" :key="row.key" class="inherit-source-row">
            <AlchemyInheritSourceCell
              :source="row.father"
              side="father"
              :decision="row.father ? decisionFor(row.father) : 'ignore'"
              :decision-options="decisionOptions"
              :decision-hint="decisionHint"
              :empty-text="t('alchemy.inherit.noSourceHere')"
              @update:decision="(decision) => row.father && emit('set-decision', row.father, decision)"
            />
            <div class="inherit-relation" :class="`is-${row.relation}`">
              <el-popover placement="top" :width="240" trigger="hover" :show-after="180">
                <template #reference>
                  <el-tag :type="relationTagType(row.relation)" effect="plain" size="small">{{ relationLabel(row.relation) }}</el-tag>
                </template>
                <div class="inherit-relation-popover">
                  <strong>{{ relationLabel(row.relation) }}</strong>
                  <span>{{ relationHint(row.relation) }}</span>
                </div>
              </el-popover>
            </div>
            <AlchemyInheritSourceCell
              :source="row.mother"
              side="mother"
              :decision="row.mother ? decisionFor(row.mother) : 'ignore'"
              :decision-options="decisionOptions"
              :decision-hint="decisionHint"
              :empty-text="t('alchemy.inherit.noSourceHere')"
              @update:decision="(decision) => row.mother && emit('set-decision', row.mother, decision)"
            />
          </div>
        </article>
      </section>
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed } from "vue"
import { useI18n } from "vue-i18n"
import AttributeIcon from "@components/game/icon/AttributeIcon.vue"
import AlchemyInheritSourceCell from "./AlchemyInheritSourceCell.vue"
import type { EquipmentAffixSlot } from "@/types/equipment"

type SourceDecision = "inherit" | "suppress" | "ignore"
type ParentSide = "father" | "mother"
type SourceComponentKind = "attr" | "effect"
type SourceRelation = "hybrid" | "inbreed" | "single"
type TagType = "success" | "warning" | "info" | "primary" | "danger"
type GroupTone = "positive" | "negative" | "neutral"

type DecisionOption = {
  label: string
  value: SourceDecision
}

type InheritSourceRow = {
  choiceKey: string
  parent: ParentSide
  componentIndex: number
  sourceKey: string
  attr: string
  attrLabel: string
  componentKind: SourceComponentKind
  sourceLabel: string
  sourceNote?: string
  effectLabel?: string
  compositionItemIds?: string[]
  itemId?: string
  affixId?: string
  affixSlot?: EquipmentAffixSlot | null
  currentValue: number
  valueText: string
  rangeText: string
}

type SourceMatrixRow = {
  key: string
  attr: string
  sourceKey: string
  father: InheritSourceRow | null
  mother: InheritSourceRow | null
  relation: SourceRelation
}

type SourceGroup = {
  key: string
  attr: string
  label: string
  rows: SourceMatrixRow[]
}

type SourceSection = {
  key: GroupTone
  label: string
  groups: SourceGroup[]
}

const props = defineProps<{
  sourceGroups: SourceGroup[]
  selectedKind: string
  kindMismatch: boolean
  decisionOptions: DecisionOption[]
  decisionFor: (source: InheritSourceRow) => SourceDecision
  relationLabel: (relation: SourceRelation) => string
  relationHint: (relation: SourceRelation) => string
  relationTagType: (relation: SourceRelation) => TagType
  decisionHint: (source: InheritSourceRow, decision: SourceDecision) => string
}>()

const emit = defineEmits<{
  (e: "set-decision", source: InheritSourceRow, decision: SourceDecision): void
}>()

const { t } = useI18n()
const sourceSections = computed<SourceSection[]>(() => {
  const buckets: Record<GroupTone, SourceGroup[]> = {
    positive: [],
    negative: [],
    neutral: []
  }
  for (const group of props.sourceGroups) {
    buckets[groupTone(group)].push(group)
  }
  return (["positive", "negative", "neutral"] as GroupTone[])
    .filter((tone) => buckets[tone].length > 0)
    .map((tone) => ({
      key: tone,
      label: t(`alchemy.inherit.sourceSection.${tone}`),
      groups: buckets[tone]
    }))
})

function groupTone(group: SourceGroup): GroupTone {
  const values = group.rows.flatMap((row) => [row.father?.currentValue, row.mother?.currentValue])
    .filter((value): value is number => Number.isFinite(Number(value)))
  const hasPositive = values.some((value) => value > 0)
  const hasNegative = values.some((value) => value < 0)
  if (hasPositive && !hasNegative) return "positive"
  if (hasNegative && !hasPositive) return "negative"
  return "neutral"
}
</script>

<style scoped>
.inherit-matrix-section {
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
  font-size: 15px;
  font-weight: 800;
}

.inherit-section-head__hint {
  display: none;
}

.inherit-empty {
  padding: 14px;
  border: 1px dashed var(--app-border, #dde4ea);
  border-radius: 6px;
  color: var(--app-muted, #667085);
  font-size: 12px;
}

.inherit-source-sections {
  display: grid;
  gap: 12px;
}

.inherit-source-section {
  display: grid;
  gap: 7px;
}

.inherit-source-section__head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  padding: 3px 8px;
  border-left: 3px solid color-mix(in srgb, var(--app-muted, #667085) 52%, transparent);
  border-radius: 4px;
  background: color-mix(in srgb, var(--app-bg, #f6f8fb) 72%, transparent);
}

.inherit-source-section.is-positive .inherit-source-section__head {
  border-left-color: #047857;
  background: color-mix(in srgb, #10b981 7%, transparent);
}

.inherit-source-section.is-negative .inherit-source-section__head {
  border-left-color: #e11d48;
  background: color-mix(in srgb, #f43f5e 7%, transparent);
}

.inherit-source-section__head > span {
  color: var(--app-text, #1f2328);
  font-size: 12px;
  font-weight: 900;
}

.inherit-source-section__head > small {
  color: var(--app-muted, #667085);
  font-size: 11px;
  font-weight: 700;
}

.inherit-source-group {
  display: grid;
  gap: 6px;
  padding-top: 2px;
}

.inherit-source-group + .inherit-source-group {
  padding-top: 10px;
  border-top: 1px solid color-mix(in srgb, var(--app-border, #dde4ea) 70%, transparent);
}

.inherit-source-group__head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0 2px;
}

.inherit-source-group__title {
  min-width: 0;
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 13px;
  font-weight: 800;
}

.inherit-source-group__head > span {
  color: var(--app-muted, #667085);
  font-size: 12px;
}

.inherit-source-row {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 60px minmax(0, 1fr);
  gap: 5px;
  align-items: stretch;
}

.inherit-relation {
  min-width: 0;
  display: grid;
  align-content: center;
  justify-items: center;
  padding: 0;
  border-radius: 5px;
  background: transparent;
  text-align: center;
}

.inherit-relation :deep(.el-tag) {
  max-width: 58px;
  height: 20px;
  padding: 0 5px;
  border-radius: 999px;
  font-size: 10px;
  overflow: hidden;
  text-overflow: ellipsis;
}

.inherit-relation-popover {
  display: grid;
  gap: 5px;
  color: var(--app-text, #1f2328);
  font-size: 12px;
  line-height: 1.45;
}

.inherit-relation-popover span {
  color: var(--app-muted, #667085);
}

@media (max-width: 700px) {
  .inherit-source-row {
    grid-template-columns: 1fr;
  }

  .inherit-relation {
    justify-items: start;
    text-align: left;
  }
}
</style>
