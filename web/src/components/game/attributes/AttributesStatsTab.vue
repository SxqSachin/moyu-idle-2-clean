<template>
  <div class="attribute-stats-panel" data-tutorial="attribute-stats">
    <el-tabs v-model="activeGroupTab" class="attribute-group-tabs" data-tutorial="attribute-group-tabs">
      <el-tab-pane
        v-for="group in categorizedRows"
        :key="group.key"
        :label="t(group.titleKey)"
        :name="group.key"
      >
        <section
          class="attribute-group"
          :class="`attribute-group--${group.key}`"
          :data-tutorial="group.key === 'battle' ? 'battle-stats' : 'work-stats'"
        >
          <div class="attribute-group__layout">
            <section v-if="group.primarySection" class="attribute-panel attribute-panel--primary">
              <div class="attribute-panel__header">
                <div>
                  <div class="attribute-panel__title">{{ t(group.primarySection.titleKey) }}</div>
                </div>
                <div v-if="group.key === 'battle' && showAllocateButton" class="attribute-panel__actions">
                  <el-button
                    size="small"
                    type="primary"
                    data-tutorial="attribute-allocate-button"
                    @click="$emit('open-allocate-dialog')"
                  >
                    {{ t("game.battlePointAllocateAction") }}
                  </el-button>
                </div>
              </div>
              <div class="attribute-primary-grid">
                <div
                  v-for="row in group.primarySection.rows"
                  :key="`attr-primary-${row.key}`"
                  class="attribute-primary-grid__item"
                  :class="{ 'attribute-primary-grid__item--full-row': group.key === 'battle' && row.key === 'battle' }"
                >
                  <AttributeStatCard
                    :row="row"
                    variant="default"
                    emphasis="primary"
                    reserve-progress-space
                    display-level-as-value
                    :show-progress="group.key !== 'battle' || row.key === 'battle'"
                  />
                </div>
              </div>
            </section>

            <section v-if="group.derivedSections.length" class="attribute-panel attribute-panel--derived">
              <div class="attribute-panel__header">
                <div>
                  <div class="attribute-panel__title">{{ t("game.attrDerivedGroup") }}</div>
                </div>
              </div>
              <div class="attribute-derived-grid">
                <section
                  v-for="section in group.derivedSections"
                  :key="section.key"
                  class="attribute-derived-section"
                >
                  <div class="attribute-derived-section__header">
                    <span>{{ t(section.titleKey) }}</span>
                  </div>
                  <div v-if="visibleSectionColumns(section).length" class="attribute-derived-section__columns">
                    <div
                      v-for="column in visibleSectionColumns(section)"
                      :key="column.key"
                      class="attribute-derived-section__column"
                    >
                      <div class="attribute-derived-section__column-title">{{ t(column.titleKey) }}</div>
                      <div class="attribute-derived-section__list attribute-derived-section__list--typed">
                        <AttributeStatCard
                          v-for="cell in column.cells"
                          :key="`attr-${section.key}-${column.key}-${cell.key}`"
                          :row="cell.value!"
                          variant="metric"
                          emphasis="normal"
                        />
                      </div>
                    </div>
                  </div>
                  <div v-else class="attribute-derived-section__list">
                    <AttributeStatCard
                      v-for="row in section.rows"
                      :key="`attr-${section.key}-${row.key}`"
                      :row="row"
                      variant="metric"
                      emphasis="normal"
                    />
                  </div>
                </section>
              </div>
            </section>
          </div>
          <div v-if="group.key === 'battle' && basicAttackEffectGroups.length" class="attribute-basic-attack">
            <div class="attribute-basic-attack__title">{{ t("game.battleUnitBasicAttack") }}</div>
            <BasicAttackDetailBlocks :groups="basicAttackEffectGroups" :details="[]" />
          </div>
        </section>
      </el-tab-pane>
    </el-tabs>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from "vue-i18n"
import { computed, ref, watchEffect } from "vue"
import type { AttributeRow } from "@/composables/game/attributes/useAttributeRows"
import AttributeStatCard from "@components/game/attributes/AttributeStatCard.vue"
import BasicAttackDetailBlocks from "@components/game/common/BasicAttackDetailBlocks.vue"
import { evaluateAmountFormulaPreview, localizeAmountFormulaText, localizeTargetCountMaxText, localizeTargetCountText } from "@services/effect-formula-display"
import { buildCharacterAttributeGroups } from "@services/attribute-layout"

const props = withDefaults(defineProps<{
  rows: AttributeRow[]
  basicAttack?: Record<string, unknown> | null
  unitAttributes?: Record<string, number> | null
  visibleGroupKeys?: Array<"work" | "battle">
  splitWorkSectionsIntoTwoColumns?: boolean
  showAllocateButton?: boolean
}>(), {
  showAllocateButton: true
})

defineEmits<{
  (e: "refresh"): void
  (e: "open-allocate-dialog"): void
}>()

const { t } = useI18n()
const activeGroupTab = ref<"work" | "battle">("work")

type BasicAttackDetail = {
  key: string
  label: string
  value: string
  buffIds?: string[]
  labelHint?: string
}

type TargetCountInput = Parameters<typeof localizeTargetCountText>[0]
type AttributeSection = ReturnType<typeof buildCharacterAttributeGroups>[number]["sections"][number]

const basicAttackEffectGroups = computed(() => {
  const rawProfile = props.basicAttack && typeof props.basicAttack === "object" ? props.basicAttack : null
  const runtimeEffects = Array.isArray(rawProfile?.effects) ? (rawProfile.effects as Array<Record<string, unknown>>) : []
  const effects = runtimeEffects.length > 0 ? runtimeEffects : buildLegacyPreviewEffects(rawProfile)
  return effects.map((effect, index) => {
    const targeting = effect?.targeting && typeof effect.targeting === "object" ? (effect.targeting as Record<string, unknown>) : {}
    const details: BasicAttackDetail[] = [
      {
        key: "kind",
        label: t("game.skillEffectKindLabel"),
        value: t(`game.skillEffectKind.${String(effect?.kind || "damage")}`)
      },
      {
        key: "target-direction",
        label: t("game.battleUnitBasicAttackTargetDirection"),
        value: localizeTargetPositions(targeting.targetPositions)
      },
      {
        key: "target-seek",
        label: t("game.battleUnitBasicAttackTargetSeek"),
        value: t(`game.targetSeekStrategy.${String(targeting.secondarySeek || "front_to_rear")}`)
      },
      {
        key: "target-mode",
        label: t("game.skillTargetMode"),
        value: t(`game.skillTargetScope.${String(targeting.targetScope || "enemy")}`)
      },
      {
        key: "target-count",
        label: t("game.skillTargetBaseCount"),
        value: localizeTargetCountText(asTargetCountInput(targeting.targetCount), t)
      },
      // {
      //   key: "raw-config",
      //   label: t("game.skillEffectRawConfig"),
      //   value: formatRawJson(effect)
      // }
    ]
    const targetCountMax = localizeTargetCountMaxText(asTargetCountInput(targeting.targetCount), t)
    if (targetCountMax !== null) {
      details.push({
        key: "target-count-max",
        label: t("game.skillTargetMaxCount"),
        value: targetCountMax,
        labelHint: t("game.skillTargetMaxCountHint")
      })
    }
    if (effect?.kind === "damage" || effect?.kind === "heal") {
      const amountFormula = effect?.amountFormula && typeof effect.amountFormula === "object"
        ? (effect.amountFormula as Record<string, unknown>)
        : effect?.damageSpec && typeof effect.damageSpec === "object" && (effect.damageSpec as Record<string, unknown>).amountFormula && typeof (effect.damageSpec as Record<string, unknown>).amountFormula === "object"
          ? ((effect.damageSpec as Record<string, unknown>).amountFormula as Record<string, unknown>)
          : null
      const preview = evaluateAmountFormulaPreview(
        { baseAdd: Number(amountFormula?.baseAdd || 0), terms: Array.isArray(amountFormula?.terms) ? amountFormula.terms : [] },
        props.unitAttributes || null,
        1
      )
      const formulaText = localizeAmountFormulaText(amountFormula?.baseAdd, amountFormula?.terms, t)
      details.push({
        key: "formula",
        label: t("game.battleUnitBasicAttackFormula"),
        value: preview === null ? formulaText : `${formulaText} (${formatFloat(preview)})`
      })
    }
    if (effect?.kind === "status") {
      const payload = effect?.statusPayload && typeof effect.statusPayload === "object"
        ? (effect.statusPayload as Record<string, unknown>)
        : {}
      const statusIds = Array.isArray(payload.statusIds)
        ? payload.statusIds.map((value) => String(value || "").trim()).filter(Boolean)
        : []
      if (statusIds.length > 0) {
        details.push({
          key: "status-ids",
          label: t("game.skillEffectStatusIds"),
          value: "-",
          buffIds: statusIds
        })
      }
      const baseTurns = Math.max(0, Math.trunc(Number(payload.durationBaseTurns || 0)))
      const perLevel = Number(payload.durationPerSkillLevelTurns || 0)
      const duration = Number.isFinite(perLevel) && perLevel !== 0
        ? `${baseTurns} + ${formatFloat(perLevel)} / Lv`
        : String(baseTurns)
      details.push({
        key: "status-duration",
        label: t("game.skillEffectStatusDuration"),
        value: duration
      })
    }
    return {
      key: `effect-${index + 1}`,
      title: `${t("game.skillEffectTitle")} ${index + 1}`,
      details
    }
  })
})

function asTargetCountInput(value: unknown): TargetCountInput {
  if (!value || typeof value !== "object") return null
  return value as TargetCountInput
}

function buildLegacyPreviewEffects(rawProfile: Record<string, unknown> | null): Array<Record<string, unknown>> {
  if (!rawProfile) return []
  const formula = Array.isArray(rawProfile.formula) ? rawProfile.formula : []
  const damageSpec = rawProfile.damageSpec && typeof rawProfile.damageSpec === "object"
    ? (rawProfile.damageSpec as Record<string, unknown>)
    : null
  const amountFormulaFromSpec = damageSpec?.amountFormula && typeof damageSpec.amountFormula === "object"
    ? (damageSpec.amountFormula as Record<string, unknown>)
    : null
  if (formula.length === 0 && !amountFormulaFromSpec) return []
  return [
    {
      kind: "damage",
      damageType: String(rawProfile.damageType || "physical"),
      targeting: {
        targetPositions: [String(rawProfile.targetDirection || "vanguard")],
        secondarySeek: String(rawProfile.targetSeekStrategy || "front_to_rear"),
        targetScope: "enemy",
        targetCount: { base: 1, formula: [] }
      },
      amountFormula: amountFormulaFromSpec || { baseAdd: 0, terms: formula },
      damageSpec: damageSpec || undefined
    }
  ]
}

const categorizedRows = computed(() => {
  const visibleGroupKeys = props.visibleGroupKeys || ["work", "battle"]
  const groups = buildCharacterAttributeGroups(props.rows)
  const work = groups.find((row) => row.key === "work")
  const battle = groups.find((row) => row.key === "battle")
  return [work, battle]
    .filter((row): row is NonNullable<typeof row> =>
      Boolean(row) && visibleGroupKeys.includes(row!.key as "work" | "battle")
    )
    .map((group) => ({
      ...group,
      sections: group.sections.filter((section) => section.rows.length > 0)
    }))
    .map((group) => {
      const primary = group.sections.find((section) => section.kind === "primary")
      const derived = group.sections.filter((section) => section.kind === "derived")
      return {
        ...group,
        primarySection: primary,
        derivedSections: derived
      }
    })
    .filter((group) => group.sections.length > 0)
})

function visibleSectionColumns(section: AttributeSection) {
  const titleKeys = section.columnTitleKeys || []
  if (titleKeys.length === 0) return []
  return section.columns
    .map((cells, index) => ({
      key: `${section.key}-${index}`,
      titleKey: titleKeys[index] || "",
      cells: cells.filter((cell) => cell.value && !cell.placeholder)
    }))
    .filter((column) => column.titleKey && column.cells.length > 0)
}

watchEffect(() => {
  const visible = categorizedRows.value
  if (visible.length <= 0) return
  if (visible.some((group) => group.key === activeGroupTab.value)) return
  activeGroupTab.value = visible[0].key as "work" | "battle"
})

function localizeTargetPositions(positions: unknown) {
  const rows = Array.isArray(positions) ? positions : []
  if (rows.length === 0) return t("game.targetDirection.vanguard")
  return rows.map((position) => t(`game.targetDirection.${String(position || "vanguard")}`)).join(" / ")
}

function formatFloat(value: number) {
  if (!Number.isFinite(value)) return "0"
  const normalized = Math.round(value * 100) / 100
  if (Number.isInteger(normalized)) return String(normalized)
  return normalized.toFixed(2).replace(/\.0+$/, "").replace(/(\.\d*[1-9])0+$/, "$1")
}

function formatRawJson(value: unknown): string {
  try {
    return JSON.stringify(value, null, 2)
  } catch {
    return String(value ?? "")
  }
}
</script>

<style scoped>
.attribute-stats-panel {
  display: grid;
  gap: 10px;
  font-size: 14px;
}

.attribute-group-tabs :deep(.el-tabs__header) {
  margin: 0 0 10px;
}

.attribute-group-tabs :deep(.el-tabs__nav-wrap::after) {
  height: 1px;
  background: color-mix(in srgb, var(--app-border) 78%, transparent);
}

.attribute-group {
  display: grid;
  gap: 10px;
  min-width: 0;
}

.attribute-group__layout {
  display: grid;
  gap: 12px;
  min-width: 0;
}

.attribute-panel {
  display: grid;
  min-width: 0;
  overflow: hidden;
  border: 1px solid color-mix(in srgb, var(--app-border) 76%, transparent);
  border-radius: 8px;
  background: color-mix(in srgb, var(--app-surface) 88%, var(--app-surface-elevated));
}

.attribute-panel__header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  min-width: 0;
  border-bottom: 1px solid color-mix(in srgb, var(--app-border) 70%, transparent);
  padding: 9px 12px;
}

.attribute-panel__actions {
  display: flex;
  min-width: 0;
  flex: none;
  flex-wrap: wrap;
  align-items: center;
  justify-content: flex-end;
  gap: 8px;
}

.attribute-panel__title,
.attribute-basic-attack__title {
  color: var(--app-text);
  font-size: 13px;
  font-weight: 700;
  line-height: 16px;
}

.attribute-primary-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(176px, 1fr));
  gap: 8px;
  min-width: 0;
  padding: 10px;
}

.attribute-primary-grid__item {
  min-width: 0;
}

.attribute-primary-grid__item--full-row {
  grid-column: 1 / -1;
}

.attribute-derived-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(260px, 1fr));
  gap: 10px;
  min-width: 0;
  padding: 10px;
}

.attribute-derived-section {
  display: grid;
  align-content: start;
  min-width: 0;
  overflow: hidden;
  border: 1px solid color-mix(in srgb, var(--app-border) 64%, transparent);
  border-radius: 6px;
  background: color-mix(in srgb, var(--app-surface-elevated) 42%, transparent);
}

.attribute-derived-section__header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  min-width: 0;
  border-bottom: 1px solid color-mix(in srgb, var(--app-border) 62%, transparent);
  padding: 7px 9px;
  color: var(--app-muted);
  font-size: 12px;
  font-weight: 700;
  line-height: 16px;
}

.attribute-derived-section__header span:first-child {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.attribute-derived-section__list {
  display: grid;
  min-width: 0;
  padding: 0 9px;
}

.attribute-derived-section__columns {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(180px, 1fr));
  gap: 0 12px;
  min-width: 0;
  padding: 0 9px;
}

.attribute-derived-section__column {
  display: grid;
  align-content: start;
  min-width: 0;
}

.attribute-derived-section__column-title {
  padding: 7px 0 3px;
  color: var(--app-muted);
  font-size: 11px;
  font-weight: 700;
  line-height: 15px;
}

.attribute-derived-section__list--typed {
  padding: 0;
}

.attribute-basic-attack {
  display: grid;
  gap: 8px;
  border: 1px solid color-mix(in srgb, var(--app-border) 70%, transparent);
  border-radius: 6px;
  padding: 10px;
  background: color-mix(in srgb, var(--app-surface-elevated) 56%, transparent);
}

@media (max-width: 640px) {
  .attribute-primary-grid,
  .attribute-derived-grid {
    grid-template-columns: minmax(0, 1fr);
  }
}
</style>
