<template>
  <div class="space-y-4">
    <div>
      <div class="text-base font-semibold">{{ enemyName }}</div>
      <div class="text-xs app-muted">{{ enemyDesc || "-" }}</div>
    </div>

    <div class="enemy-meta-grid">
      <div class="enemy-meta-item">
        <span class="enemy-meta-item__label">{{ t("game.battleRoomPosition") }}</span>
        <span class="enemy-meta-item__value">{{ positionLabel }}</span>
      </div>
      <div v-if="showLevel" class="enemy-meta-item">
        <span class="enemy-meta-item__label">{{ t("game.level") }}</span>
        <span class="enemy-meta-item__value">{{ definition?.level ?? "-" }}</span>
      </div>
      <div v-if="showExperience" class="enemy-meta-item">
        <span class="enemy-meta-item__label">{{ t("game.battleEnemyInfoExp") }}</span>
        <span class="enemy-meta-item__value">{{ expLabel }}</span>
      </div>
      <DifficultySelector
        v-if="showDifficulty && difficultyContext"
        v-model="selectedDifficultyLevel"
        :world-difficulty-settings="difficultyContext.worldDifficultySettings"
        compact
      />
    </div>

    <div>
      <div class="text-xs app-muted mb-2">{{ t("game.battleUnitStats") }}</div>
      <div v-if="primaryStatRows.length || detailStatSections.length" class="enemy-stat-area">
        <section v-if="primaryStatRows.length" class="enemy-stat-card enemy-stat-card--primary">
          <div class="enemy-stat-card__header">
            <span>{{ t("game.attrGrowthLevelable") }}</span>
          </div>
          <div class="enemy-stat-primary-grid">
            <div
              v-for="row in primaryStatRows"
              :key="`enemy-primary-stat-${row.key}`"
              class="enemy-stat-row"
              :title="`${row.name}: ${row.value}${row.valueSuffix || ''}`"
            >
              <AttributeIcon :attr-key="row.key" :alt="row.name" :size="14" />
              <span class="enemy-stat-row__name">{{ row.name }}</span>
              <span class="enemy-stat-row__value">{{ row.value }}<span v-if="row.valueSuffix" class="app-muted">{{ row.valueSuffix }}</span></span>
            </div>
          </div>
        </section>

        <div v-if="detailStatSections.length" class="enemy-stat-group-grid">
          <section
            v-for="section in detailStatSections"
            :key="section.key"
            class="enemy-stat-card"
          >
            <div class="enemy-stat-card__header">
              <span>{{ t(section.titleKey) }}</span>
            </div>
            <div v-if="visibleStatColumns(section).length" class="enemy-stat-column-grid">
              <div
                v-for="column in visibleStatColumns(section)"
                :key="column.key"
                class="enemy-stat-column"
              >
                <div class="enemy-stat-column__title">{{ t(column.titleKey) }}</div>
                <div class="enemy-stat-list">
                  <div
                    v-for="cell in column.cells"
                    :key="`enemy-stat-cell-${section.key}-${column.key}-${cell.key}`"
                    class="enemy-stat-row"
                    :title="`${cell.value!.name}: ${cell.value!.value}${cell.value!.valueSuffix || ''}`"
                  >
                    <AttributeIcon :attr-key="cell.value!.key" :alt="cell.value!.name" :size="14" />
                    <span class="enemy-stat-row__name">{{ cell.value!.name }}</span>
                    <span class="enemy-stat-row__value">{{ cell.value!.value }}<span v-if="cell.value!.valueSuffix" class="app-muted">{{ cell.value!.valueSuffix }}</span></span>
                  </div>
                </div>
              </div>
            </div>
            <div v-else class="enemy-stat-list">
              <div
                v-for="row in section.rows"
                :key="`enemy-stat-${section.key}-${row.key}`"
                class="enemy-stat-row"
                :title="`${row.name}: ${row.value}${row.valueSuffix || ''}`"
              >
                <AttributeIcon :attr-key="row.key" :alt="row.name" :size="14" />
                <span class="enemy-stat-row__name">{{ row.name }}</span>
                <span class="enemy-stat-row__value">{{ row.value }}<span v-if="row.valueSuffix" class="app-muted">{{ row.valueSuffix }}</span></span>
              </div>
            </div>
          </section>
        </div>
      </div>
      <div v-else class="text-xs app-muted">-</div>
    </div>

    <div v-if="initialBuffs.length">
      <div class="text-xs app-muted mb-2">{{ t("game.battleEnemyInfoInitialBuffs") }}</div>
      <div class="flex flex-wrap gap-2">
        <BuffQuickRef
          v-for="row in initialBuffs"
          :key="`enemy-initial-buff-${enemyId}-${row.buffId}`"
          :buff-id="row.buffId"
          :remain-turns="row.remainTurns"
        />
      </div>
    </div>

    <div>
      <div class="text-xs app-muted mb-2">{{ t("game.battleUnitSkills") }}</div>
      <div v-if="skillIds.length" class="flex flex-wrap gap-2">
        <SkillQuickRef
          v-for="skillId in skillIds"
          :key="`enemy-skill-${enemyId}-${skillId}`"
          :skill-id="skillId"
          show-icon
          :unit-attributes="displayStats"
        />
      </div>
      <div v-else class="text-xs app-muted">-</div>
    </div>

    <div>
      <div class="text-xs app-muted mb-2">{{ t("game.battleUnitAttackMode") }}</div>
      <div v-if="equipmentRows.length" class="space-y-2">
        <div
          v-for="row in equipmentRows"
          :key="`enemy-equipment-${enemyId}-${row.templateItemId}`"
          class="rounded border border-[var(--app-border)] p-2 text-xs space-y-2"
        >
          <div class="mb-2 flex items-center gap-2 flex-wrap">
            <QuickRef :item-id="row.templateItemId" />
            <span v-if="row.enhanceLevel > 0" class="app-muted">+{{ row.enhanceLevel }}</span>
          </div>
          <EquipmentStaticInfo
            v-if="row.definition"
            :equipment-definition="row.definition"
            :equipped-effect-attrs="row.baseSnapshot"
          />
        </div>
      </div>
      <div v-else class="text-xs app-muted">{{ emptyAttackModeLabel || t("game.battleUnitDefaultAttackMode") }}</div>
    </div>

    <div v-if="showDrops">
      <div class="text-xs app-muted mb-2">{{ t("game.battleEnemyInfoDrops") }}</div>
      <div v-if="drops.length" class="space-y-2">
        <div v-for="drop in drops" :key="`drop-${drop.itemId}`" class="rounded border border-[var(--app-border)] p-2 text-xs">
          <div class="flex items-center gap-2 flex-wrap">
            <QuickRef :item-id="drop.itemId" />
            <span class="app-muted">{{ t("game.battleEnemyDropRate", { percent: formatPercent(drop.percent) }) }}</span>
          </div>
          <div class="mt-1 app-muted">{{ t("game.battleEnemyDropRange", { min: drop.min, max: drop.max }) }}</div>
        </div>
      </div>
      <div v-else class="text-xs app-muted">{{t("game.battleEnemyInfoDropsEmpty")}}</div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, watch } from "vue"
import { useI18n } from "vue-i18n"
import { storeToRefs } from "pinia"
import SkillQuickRef from "@components/game/skill/SkillQuickRef.vue"
import QuickRef from "@components/game/item/QuickRef.vue"
import EquipmentStaticInfo from "@components/game/item/EquipmentStaticInfo.vue"
import AttributeIcon from "@components/game/icon/AttributeIcon.vue"
import BuffQuickRef from "@components/game/buff/BuffQuickRef.vue"
import DifficultySelector from "@components/game/combat/DifficultySelector.vue"
import type { AttributeRow } from "@/composables/game/attributes/useAttributeRows"
import { formatAttributeValueByKey } from "@services/attribute-display"
import {
  buildBattleDetailAttributeSections,
  getConfiguredColumnsForScene,
  sortAttributeKeysForScene,
  type AttributeLayoutSection
} from "@services/attribute-layout"
import { projectDerivedFinalStats } from "@services/attribute-derived-impact"
import {
  getEnemyDefinition,
  getLocalizedEnemyDesc,
  getLocalizedEnemyName,
  type EnemyDefinition
} from "@services/enemy-definition"
import { getBuffDefinition } from "@services/buff-definition"
import { getEquipmentItemDefinition } from "@services/item-definition"
import { usePlayerStore } from "@stores/player"
import { DEFENSE_K, ENEMY_HP_REGEN_RATIO } from "@/config/shared-global-config"
import { formatDecimal } from "@utils/number-format"
import { projectBattleEnemyDifficultyStats } from "@services/battle-difficulty"
import type { EnemyQuickRefDifficultyContext } from "@stores/enemy-unit-quick-ref-dialog"

const props = withDefaults(defineProps<{
  enemyId?: string
  definitionOverride?: EnemyDefinition | null
  nameOverride?: string
  descriptionOverride?: string
  difficultyContext?: EnemyQuickRefDifficultyContext | null
  showLevel?: boolean
  showExperience?: boolean
  showDifficulty?: boolean
  showDrops?: boolean
  fallbackBasicAttack?: boolean
  projectDerivedStats?: boolean
  includeConfiguredStats?: boolean
  statValueOverrides?: Record<string, string>
  emptyAttackModeLabel?: string
}>(), {
  enemyId: "",
  definitionOverride: null,
  nameOverride: "",
  showLevel: true,
  showExperience: true,
  showDifficulty: true,
  showDrops: true,
  fallbackBasicAttack: true,
  projectDerivedStats: true,
  includeConfiguredStats: true,
  emptyAttackModeLabel: ""
})

const { t } = useI18n()
const playerStore = usePlayerStore()
const { formula } = storeToRefs(playerStore)
const selectedDifficultyLevel = ref(0)
const enemyId = computed(() => String(props.enemyId || "").trim())
const difficultyContext = computed(() => props.difficultyContext || null)
const definition = computed(() => props.definitionOverride || getEnemyDefinition(enemyId.value))
const enemyName = computed(() => String(props.nameOverride || "").trim() || getLocalizedEnemyName(t, enemyId.value, enemyId.value))
const enemyDesc = computed(() => props.descriptionOverride !== undefined
  ? String(props.descriptionOverride || "").trim()
  : getLocalizedEnemyDesc(t, enemyId.value, ""))
const showLevel = computed(() => props.showLevel)
const showExperience = computed(() => props.showExperience)
const showDifficulty = computed(() => props.showDifficulty)
const showDrops = computed(() => props.showDrops)
const emptyAttackModeLabel = computed(() => String(props.emptyAttackModeLabel || "").trim())
const expLabel = computed(() => {
  const value = Number(definition.value?.exp || 0)
  if (!Number.isFinite(value)) return "0"
  return String(Math.max(0, Math.trunc(value)))
})
const positionLabel = computed(() => {
  const key = `game.battlePosition${capitalize(definition.value?.position || "vanguard")}`
  const text = t(key)
  return text === key ? String(definition.value?.position || "vanguard") : text
})

const skillIds = computed(() => {
  const rows = definition.value?.skillIds || []
  if (rows.length > 0) return rows
  return props.fallbackBasicAttack ? ["basic_attack"] : []
})

const initialBuffs = computed(() => {
  return (definition.value?.initialBuffIds || []).map((buffId) => {
    const def = getBuffDefinition(buffId)
    const dur = Number(def?.durationTurns ?? 0)
    const remainTurns = Number.isFinite(dur) && dur > 0 ? Math.trunc(dur) : -1
    return { buffId, remainTurns }
  })
})

const drops = computed(() => definition.value?.drops || [])
const equipmentRows = computed(() => (definition.value?.equipment || [])
  .map((entry) => ({
    ...entry,
    definition: getEquipmentItemDefinition(entry.templateItemId)
  }))
)
const displayStats = computed(() => {
  const stats = props.projectDerivedStats
    ? projectDerivedFinalStats(definition.value?.stats || {}, formula.value?.derived)
    : { ...(definition.value?.stats || {}) }
  if (stats.hp_regen && stats.hp_regen > 0 && ENEMY_HP_REGEN_RATIO < 1) {
    stats.hp_regen = Math.round(stats.hp_regen * ENEMY_HP_REGEN_RATIO * 10000) / 10000
  }
  return projectBattleEnemyDifficultyStats(
    stats,
    selectedDifficultyLevel.value,
    difficultyContext.value?.worldDifficultySettings
  )
})

watch(() => difficultyContext.value?.worldId || "", () => {
  selectedDifficultyLevel.value = 0
})

const statsRows = computed<AttributeRow[]>(() => {
  const stats = displayStats.value
  const DEFENSE_REDUCTION_KEYS = new Set(["physics_defence", "magical_defence"])
  const orderedKeys = sortAttributeKeysForScene("battle_unit_card", resolvedBattleStatKeys(stats), "battle")
  return orderedKeys.map((key) => {
    const value = Number(stats[key] || 0)
    const hasValueOverride = Object.prototype.hasOwnProperty.call(props.statValueOverrides || {}, key)
    const i18nKey = key == "level" ? "game.level" : `attr.${key}.name`
    const localized = t(i18nKey)
    let valueSuffix: string | undefined
    if (!hasValueOverride && DEFENSE_REDUCTION_KEYS.has(key) && value > 0) {
      const pct = (value / (DEFENSE_K + value)) * 100
      valueSuffix = ` (${formatDecimal(pct, 1)}%)`
    }
    return {
      key,
      name: localized === i18nKey ? key : localized,
      desc: "",
      value: hasValueOverride ? String(props.statValueOverrides?.[key] || "") : formatAttributeValueByKey(key, value),
      valueSuffix,
      rawValue: value,
      skillPointLevel: 0,
      allowBattlePointLevelUp: false,
      compositionLines: [],
      impactLines: [],
      breakdownLines: [],
      growthType: "static",
      category: "battle",
      order: 0,
      level: 1,
      exp: 0,
      nextExp: 0,
      progress: 0
    }
  })
})
const primaryStatRows = computed(() => {
  const stats = displayStats.value
  const keys = resolvedBattleStatKeys(stats)
  const rowMap = new Map(statsRows.value.map((row) => [row.key.toLowerCase(), row]))
  return sortAttributeKeysForScene("battle_unit_card", keys, "battlePrimary")
    .map((key) => rowMap.get(key.toLowerCase()))
    .filter((row): row is AttributeRow => Boolean(row))
})

const detailStatSections = computed(() => {
  const stats = displayStats.value
  const keys = resolvedBattleStatKeys(stats)
  const rowMap = new Map(statsRows.value.map((row) => [row.key.toLowerCase(), row]))
  const primaryKeySet = new Set(primaryStatRows.value.map((row) => row.key.toLowerCase()))
  const secondaryRows = sortAttributeKeysForScene("battle_unit_card", keys, "battle")
    .map((key) => rowMap.get(key.toLowerCase()))
    .filter((row): row is AttributeRow => Boolean(row) && !primaryKeySet.has(row!.key.toLowerCase()))
  return buildBattleDetailAttributeSections(secondaryRows).filter((section) => section.rows.length > 0)
})

function visibleStatColumns(section: AttributeLayoutSection) {
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

function resolvedBattleStatKeys(stats: Record<string, number>) {
  const configuredKeys = props.includeConfiguredStats
    ? [
        ...getConfiguredColumnsForScene("battle_unit_card", "battlePrimary").flat(),
        ...getConfiguredColumnsForScene("battle_unit_card", "battle", { includeBattlePrimary: false }).flat()
      ]
    : []
  const keys = new Set<string>()
  for (const key of configuredKeys) {
    const normalized = String(key || "").trim()
    if (normalized && !normalized.startsWith("_")) keys.add(normalized)
  }
  for (const key of Object.keys(stats)) {
    const normalized = String(key || "").trim()
    if (normalized && !normalized.startsWith("_") && Number.isFinite(Number(stats[key]))) keys.add(normalized)
  }
  return Array.from(keys)
}

function formatPercent(value: number) {
  const numeric = Number(value)
  if (!Number.isFinite(numeric)) return "0"
  return (numeric * 100).toFixed(3).replace(/\\.0$/, "")
}

function capitalize(value: string) {
  const normalized = String(value || "")
  if (!normalized) return ""
  return normalized.slice(0, 1).toUpperCase() + normalized.slice(1)
}
</script>

<style scoped>
.enemy-meta-grid {
  display: flex;
  min-width: 0;
  align-items: center;
  flex-wrap: wrap;
  gap: 8px 18px;
  border-bottom: 1px solid color-mix(in srgb, var(--app-border) 64%, transparent);
  padding-bottom: 10px;
  font-size: 12px;
}

.enemy-meta-item {
  display: inline-flex;
  min-width: 0;
  align-items: baseline;
  gap: 6px;
}

.enemy-meta-item__label {
  color: var(--app-muted);
}

.enemy-meta-item__value {
  color: var(--app-text);
  font-weight: 700;
}

.enemy-stat-area {
  display: grid;
  gap: 8px;
}

.enemy-stat-group-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(210px, 1fr));
  gap: 8px;
}

.enemy-stat-card {
  min-width: 0;
  overflow: hidden;
  border: 1px solid color-mix(in srgb, var(--app-border) 76%, transparent);
  border-radius: 6px;
  background: color-mix(in srgb, var(--app-surface) 88%, var(--app-surface-elevated));
}

.enemy-stat-card__header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  border-bottom: 1px solid color-mix(in srgb, var(--app-border) 64%, transparent);
  padding: 6px 8px;
  color: var(--app-muted);
  font-size: 12px;
  font-weight: 400;
  line-height: 16px;
}

.enemy-stat-card__header span:first-child {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.enemy-stat-primary-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(132px, 1fr));
  gap: 0 10px;
  padding: 2px 8px;
}

.enemy-stat-column-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(120px, 1fr));
  gap: 0 10px;
  padding: 0 8px;
}

.enemy-stat-column {
  min-width: 0;
}

.enemy-stat-column__title {
  padding: 6px 0 2px;
  color: var(--app-muted);
  font-size: 11px;
  font-weight: 400;
  line-height: 14px;
}

.enemy-stat-list {
  display: grid;
  min-width: 0;
  padding: 2px 8px;
}

.enemy-stat-column .enemy-stat-list {
  padding: 0;
}

.enemy-stat-row {
  display: grid;
  grid-template-columns: auto minmax(0, 1fr) auto;
  align-items: center;
  gap: 6px;
  min-width: 0;
  border-bottom: 1px solid color-mix(in srgb, var(--app-border) 58%, transparent);
  padding: 6px 0;
  font-size: 12px;
  line-height: 16px;
}

.enemy-stat-row:last-child {
  border-bottom: 0;
}

.enemy-stat-row__name {
  min-width: 0;
  overflow: hidden;
  color: var(--app-text);
  font-weight: 400;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.enemy-stat-row__value {
  min-width: 0;
  color: var(--app-text);
  font-weight: 400;
  text-align: right;
  overflow-wrap: anywhere;
}
</style>
