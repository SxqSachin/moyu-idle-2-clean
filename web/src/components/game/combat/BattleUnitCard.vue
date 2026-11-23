<template>
  <el-popover trigger="hover" :width="340" :hide-after="0">
    <template #reference>
      <div
        :data-battle-unit-id="unitId"
        :class="[
          'battle-unit-card w-[120px] md:w-[156px] rounded-lg border border-[var(--app-border)] bg-[var(--el-bg-color-overlay)] px-2 py-1.5 text-[11px] shadow-sm transition-all duration-200',
          unit.alive === false ? 'battle-unit-dead' : '',
          isActive ? 'battle-unit-active' : '',
          isHovered ? 'battle-unit-hovered-order' : ''
        ]"
        @click.stop="openDetailDialog"
      >
        <div class="flex items-center justify-between gap-2">
          <div class="min-w-0 flex-1">
            <div class="truncate font-medium" :title="unitName">{{ unitName }}</div>
            <div v-if="battleCardBuffList.length" class="mt-1 flex flex-wrap items-center gap-1">
              <BuffQuickRef
                v-for="buff in battleCardBuffList"
                :key="`card-buff-${buff.buffId}`"
                :buff-id="buff.buffId"
                :remain-turns="buff.remainTurns"
                :payload="buff.payload"
                :stack-count="buff.stackCount"
                icon-only
              />
            </div>
          </div>
          <span v-if="enemyLevel > 0" class="shrink-0 text-[10px] app-muted">Lv.{{ enemyLevel }}</span>
        </div>

        <div class="mt-1.5 space-y-1.5">
          <div>
            <div class="mb-0.5 flex items-center justify-between text-[10px]">
              <span class="app-muted">HP</span>
              <span v-if="immutableHealth">{{ t("game.combatTrial.immutable") }}</span>
              <span v-else>{{ formatStat(unit.hp) }} / {{ formatStat(unit.maxHp) }}</span>
            </div>
            <div class="h-1.5 rounded bg-[rgba(0,0,0,0.1)] overflow-hidden">
              <div
                class="h-full rounded bg-rose-600 transition-all duration-300"
                :style="{ width: `${hpPercent}%` }"
              />
            </div>
          </div>

          <div>
            <div class="mb-0.5 flex items-center justify-between text-[10px]">
              <span class="app-muted">MP</span>
              <span>{{ formatStat(unit.mp) }} / {{ formatStat(unit.maxMp) }}</span>
            </div>
            <div class="h-1.5 rounded bg-[rgba(0,0,0,0.1)] overflow-hidden">
              <div
                class="h-full rounded bg-blue-600 transition-all duration-300"
                :style="{ width: `${mpPercent}%` }"
              />
            </div>
          </div>
        </div>
      </div>
    </template>
    <div class="battle-unit-popover-content space-y-2" :style="{ maxHeight: 'min(60vh, 420px)', overflowY: 'auto', paddingRight: '4px' }">
      <div>
        <div class="font-medium">{{ t("game.battleUnitStats") }}</div>
        <div v-if="statsPreviewList.length" class="battle-unit-preview-stats">
          <div
            v-for="row in statsPreviewList"
            :key="`preview-stat-${row.key}`"
            class="battle-unit-stat-row"
            :title="`${row.name}: ${row.value}${row.valueSuffix || ''}`"
          >
            <AttributeIcon :attr-key="row.key" :alt="row.name" :size="14" />
            <span class="battle-unit-stat-row__name">{{ row.name }}</span>
            <span class="battle-unit-stat-row__value">{{ row.value }}<span v-if="row.valueSuffix" class="app-muted">{{ row.valueSuffix }}</span></span>
          </div>
        </div>
        <div v-if="statsOverflowCount > 0" class="mt-1 text-[11px] app-muted">
          +{{ statsOverflowCount }}
        </div>
        <div v-else-if="!statsPreviewList.length" class="mt-1 app-muted">{{ t("game.battleUnitEmpty") }}</div>
      </div>

      <div>
        <div class="font-medium">{{ t("game.battleUnitBasicAttack") }}</div>
        <div class="mt-1 app-muted">
          <BasicAttackDetailBlocks
            :groups="basicAttackEffectGroups"
            :details="basicAttackDetails"
            compact
            :max-groups="1"
            :max-rows-per-group="4"
          />
        </div>
      </div>

      <div v-if="skillPreviewList.length">
        <div class="font-medium">{{ t("game.battleUnitSkills") }}</div>
        <div v-if="skillPreviewList.length" class="mt-1 flex flex-row flex-wrap gap-1">
          <div v-for="skill in skillPreviewList" :key="`skill-${skill.id}`">
            <SkillQuickRef
              :skill-id="skill.id"
              show-icon
              :remaining-cooldown="skill.cooldownRemain"
              :total-cooldown="skill.cooldownTurns"
              :unit-attributes="props.unit.attributes || props.unit.stats || null"
              :unit-skill-level="skill.level"
              :runtime-skill="skill.runtimeSkill"
            />
          </div>
        </div>
        <div v-if="skillOverflowCount > 0" class="mt-1 text-[11px] app-muted">
          +{{ skillOverflowCount }}
        </div>
        <div v-else-if="!skillPreviewList.length" class="mt-1 app-muted">{{ t("game.battleUnitEmpty") }}</div>
      </div>

      <div v-if="battleItemPreviewList.length">
        <div class="font-medium">{{ t("game.battleItemBagTitle") }}</div>
        <div class="mt-1 grid gap-1">
          <div v-for="entry in battleItemPreviewList" :key="`battle-item-${entry.itemId}`" class="flex items-center gap-2 flex-wrap">
            <ItemQuickRef :item-id="entry.itemId" />
            <span class="app-muted">x{{ entry.quantity }} · CD {{ entry.cooldownRemain }}/{{ entry.cooldownTurns }}</span>
          </div>
        </div>
        <div v-if="battleItemOverflowCount > 0" class="mt-1 text-[11px] app-muted">
          +{{ battleItemOverflowCount }}
        </div>
      </div>


      <!-- <div>
        <div class="font-medium">{{ t("game.battleUnitEquipment") }}</div>
        <div v-if="equipmentList.length" class="mt-1 grid gap-1">
          <div v-for="(line, index) in equipmentList" :key="`equipment-${index}`">{{ line }}</div>
        </div>
        <div v-else class="mt-1 app-muted">{{ t("game.battleUnitEmpty") }}</div>
      </div> -->

      <div v-if="buffPreviewList.length">
        <div class="font-medium">{{ t("game.battleUnitBuffs") }}</div>
        <div v-if="buffPreviewList.length" class="mt-1 grid gap-1">
          <div v-for="buff in buffPreviewList" :key="`buff-${buff.buffId}`">
            <BuffQuickRef :buff-id="buff.buffId" :remain-turns="buff.remainTurns" :payload="buff.payload" :stack-count="buff.stackCount" />
          </div>
        </div>
        <div v-if="buffOverflowCount > 0" class="mt-1 text-[11px] app-muted">
          +{{ buffOverflowCount }}
        </div>
        <div v-else-if="!buffPreviewList.length" class="mt-1 app-muted">{{ t("game.battleUnitEmpty") }}</div>
      </div>

      <div class="pt-1 text-[11px] app-muted opacity-70">
        {{ t("game.battleEnemyQuickRefHint") }}
      </div>
    </div>
  </el-popover>

  <GameDialog
    v-model="detailDialogVisible"
    :title="unitName"
    width="420px"
    destroy-on-close
  >
    <div class="space-y-3 text-xs overflow-auto pr-1">
      <div>
        <div class="font-medium">{{ t("game.battleUnitStats") }}</div>
        <div v-if="statsPrimaryRows.length || statsDialogSections.length" class="battle-unit-detail-stats">
          <section v-if="statsPrimaryRows.length" class="battle-unit-stat-card">
            <div class="battle-unit-stat-card__header">
              <span>{{ t("game.attrGrowthLevelable") }}</span>
            </div>
            <div class="battle-unit-stat-card__grid">
              <div
                v-for="row in statsPrimaryRows"
                :key="`dialog-primary-stat-${row.key}`"
                class="battle-unit-stat-row battle-unit-stat-row--dialog"
                :title="`${row.name}: ${row.value}${row.valueSuffix || ''}`"
              >
                <AttributeIcon :attr-key="row.key" :alt="row.name" :size="14" />
                <AttributeValueWithTooltip
                  :name="row.name"
                  :value="row.value"
                  :value-suffix="row.valueSuffix"
                  :composition-lines="row.breakdownLines"
                  :impact-lines="[]"
                />
              </div>
            </div>
          </section>

          <section
            v-for="section in statsDialogSections"
            :key="`dialog-stat-section-${section.key}`"
            class="battle-unit-stat-card"
          >
            <div class="battle-unit-stat-card__header">
              <span>{{ t(section.titleKey) }}</span>
            </div>
            <div v-if="visibleStatColumns(section).length" class="battle-unit-stat-columns">
              <div
                v-for="column in visibleStatColumns(section)"
                :key="`dialog-stat-col-${section.key}-${column.key}`"
                class="battle-unit-stat-column"
              >
                <div class="battle-unit-stat-column__title">{{ t(column.titleKey) }}</div>
                <div class="battle-unit-stat-list">
                  <div
                    v-for="cell in column.cells"
                    :key="`dialog-stat-cell-${section.key}-${column.key}-${cell.key}`"
                    class="battle-unit-stat-row battle-unit-stat-row--dialog"
                    :title="`${cell.value!.name}: ${cell.value!.value}${cell.value!.valueSuffix || ''}`"
                  >
                    <AttributeIcon :attr-key="cell.value!.key" :alt="cell.value!.name" :size="14" />
                    <AttributeValueWithTooltip
                      :name="cell.value!.name"
                      :value="cell.value!.value"
                      :value-suffix="cell.value!.valueSuffix"
                      :composition-lines="cell.value!.breakdownLines"
                      :impact-lines="[]"
                    />
                  </div>
                </div>
              </div>
            </div>
            <div v-else class="battle-unit-stat-list">
              <div
                v-for="row in section.rows"
                :key="`dialog-stat-${section.key}-${row.key}`"
                class="battle-unit-stat-row battle-unit-stat-row--dialog"
                :title="`${row.name}: ${row.value}${row.valueSuffix || ''}`"
              >
                <AttributeIcon :attr-key="row.key" :alt="row.name" :size="14" />
                  <AttributeValueWithTooltip
                    :name="row.name"
                    :value="row.value"
                    :value-suffix="row.valueSuffix"
                    :composition-lines="row.breakdownLines"
                    :impact-lines="[]"
                  />
              </div>
            </div>
          </section>
        </div>
        <div v-else class="mt-1 app-muted">{{ t("game.battleUnitEmpty") }}</div>
      </div>

      <div>
        <div class="font-medium">{{ t("game.battleUnitBasicAttack") }}</div>
        <div class="mt-1 app-muted">
          <BasicAttackDetailBlocks :groups="basicAttackEffectGroups" :details="basicAttackDetails" />
        </div>
      </div>

      <div>
        <div class="font-medium">{{ t("game.battleUnitSkills") }}</div>
        <div v-if="skillList.length" class="mt-1 flex flex-row flex-wrap gap-1">
          <div v-for="skill in skillList" :key="`dialog-skill-${skill.id}`">
            <SkillQuickRef
              :skill-id="skill.id"
              show-icon
              :remaining-cooldown="skill.cooldownRemain"
              :total-cooldown="skill.cooldownTurns"
              :unit-attributes="props.unit.attributes || props.unit.stats || null"
              :unit-skill-level="skill.level"
              :runtime-skill="skill.runtimeSkill"
            />
          </div>
        </div>
        <div v-else class="mt-1 app-muted">{{ t("game.battleUnitEmpty") }}</div>
      </div>

      <div>
        <div class="font-medium">{{ t("game.battleItemBagTitle") }}</div>
        <div v-if="battleItemList.length" class="mt-1 flex flex-wrap gap-2">
          <div v-for="entry in battleItemList" :key="`dialog-battle-item-${entry.itemId}`" class="flex items-center gap-2 flex-wrap">
            <ItemQuickRef :item-id="entry.itemId" />
            <span class="app-muted">x{{ entry.quantity }} · CD {{ entry.cooldownRemain }}/{{ entry.cooldownTurns }}</span>
          </div>
        </div>
        <div v-else class="mt-1 app-muted">{{ t("game.battleUnitEmpty") }}</div>
      </div>

      <div>
        <div class="font-medium">{{ t("game.battleUnitBuffs") }}</div>
        <div v-if="buffList.length" class="mt-1 flex flex-wrap gap-2">
          <div v-for="buff in buffList" :key="`dialog-buff-${buff.buffId}`">
            <BuffQuickRef :buff-id="buff.buffId" :remain-turns="buff.remainTurns" :payload="buff.payload" :stack-count="buff.stackCount" />
          </div>
        </div>
        <div v-else class="mt-1 app-muted">{{ t("game.battleUnitEmpty") }}</div>
      </div>
    </div>
  </GameDialog>
</template>

<script setup lang="ts">
import { computed, ref } from "vue"
import { useI18n } from "vue-i18n"
import SkillQuickRef from "@components/game/skill/SkillQuickRef.vue"
import ItemQuickRef from "@components/game/item/QuickRef.vue"
import BuffQuickRef from "@components/game/buff/BuffQuickRef.vue"
import AttributeIcon from "@components/game/icon/AttributeIcon.vue"
import AttributeValueWithTooltip from "@components/game/attributes/AttributeValueWithTooltip.vue"
import BasicAttackDetailBlocks from "@components/game/common/BasicAttackDetailBlocks.vue"
import type { AttributeRow } from "@/composables/game/attributes/useAttributeRows"
import { getSkillDefinition } from "@services/skill-definition"
import { getBuffDefinition, getBuffEffectRule } from "@services/buff-definition"
import { formatAttributeValueByKey } from "@/services/attribute-display"
import { formatDecimal } from "@utils/number-format"
import {
  buildBattleDetailAttributeSections,
  getScenePreviewLimit,
  sortAttributeKeysForScene,
  type AttributeLayoutSection
} from "@services/attribute-layout"
import { DEFENSE_K } from "@/config/shared-global-config"
import { evaluateAmountFormulaPreview, localizeAmountFormulaText, localizeEffectAttr, localizeTargetCountMaxText, localizeTargetCountText } from "@services/effect-formula-display"
import GameDialog from "@components/game/common/GameDialog.vue"

type BattleSkill = {
  id?: string
  cooldown?: number
  cooldownTurns?: number
  cooldownRemain?: number
  level?: number
}

type BasicAttackFormulaTerm = {
  attrKey?: string
  coefficient?: number
}

type BasicAttackProfile = {
  damageType?: string
  targetDirection?: string
  targetSeekStrategy?: string
  formula?: BasicAttackFormulaTerm[]
  damageSpec?: {
    amountFormula?: {
      terms?: BasicAttackFormulaTerm[]
    }
  }
  effects?: Array<{
    kind?: string
    damageType?: string
    targeting?: {
      targetPositions?: string[]
      secondarySeek?: string
      targetScope?: string
      selectRule?: string
      selectAttrKey?: string
      fallbackSelfIfEmpty?: boolean
      targetCount?: {
        base?: number
        formula?: Array<{ attrKey?: string; coefficient?: number; perSkillLevel?: number }>
      }
      targetPredicates?: Array<{ kind?: string }>
    }
    onlyWhenNoTargetFromEffects?: number[]
    reuseTargetsFromEffect?: number
    amountFormula?: {
      baseAdd?: number
      terms?: BasicAttackFormulaTerm[]
    }
    damageSpec?: {
      amountFormula?: {
        baseAdd?: number
        terms?: BasicAttackFormulaTerm[]
      }
    }
    statusPayload?: {
      statusIds?: string[]
      durationBaseTurns?: number
      durationPerSkillLevelTurns?: number
    }
    mpCost?: {
      baseAdd?: number
      terms?: BasicAttackFormulaTerm[]
    }
  }>
}

type BattleUnit = {
  unitId?: string | number
  name?: string
  nameKey?: string
  hp?: number
  maxHp?: number
  mp?: number
  maxMp?: number
  alive?: boolean
  corpseConsumed?: boolean
  stats?: Record<string, number>
  attributes?: Record<string, number>
  skillIds?: string[]
  skillCooldowns?: Record<string, number>
  itemCooldowns?: Record<string, number>
  itemQuantities?: Record<string, number>
  skills?: BattleSkill[]
  battleItems?: Array<{
    itemId?: string
    quantity?: number
    cooldownTurns?: number
    cooldownRemain?: number
  }>
  basicAttack?: BasicAttackProfile
  equipmentDigest?: string[]
  equipmentInstanceIds?: number[]
  buffs?: Array<{ buffId?: string; remainTurns?: number; payload?: Record<string, unknown>; triggered?: number }>
  [key: string]: unknown
}

type BasicAttackDetail = {
  key: string
  label: string
  value: string
  buffIds?: string[]
  labelHint?: string
}

const props = defineProps<{
  unitId: string
  unit: BattleUnit
  resolveName: (unit: Record<string, unknown>) => string
  isActive?: boolean
  isHovered?: boolean
  immutableHealth?: boolean
}>()

const { t } = useI18n()

const unitName = computed(() => props.resolveName(props.unit as Record<string, unknown>))
const detailDialogVisible = ref(false)
const isActive = computed(() => Boolean(props.isActive))
const isHovered = computed(() => Boolean(props.isHovered))
const immutableHealth = computed(() => Boolean(props.immutableHealth))
const hpPercent = computed(() => toPercent(props.unit.hp, props.unit.maxHp))
const mpPercent = computed(() => toPercent(props.unit.mp, props.unit.maxMp))
const unitAttributes = computed(() => (props.unit.attributes || props.unit.stats || {}) as Record<string, number>)
const enemyLevel = computed(() => {
  const side = String(props.unit.side || "")
  if (side !== "enemy") return 0
  return Math.trunc(Number((props.unit as Record<string, unknown>).enemyLevel || 0))
})
const previewSkillLimit = 4
const previewBattleItemLimit = 4
const previewBuffLimit = 4

const DEFENSE_REDUCTION_KEYS = new Set(["physics_defence", "magical_defence"])
const BATTLE_STAT_SOURCE_TRIGGERS = [
  "on_other_unit_cast_skill",
  "on_before_deal_damage",
  "on_after_deal_damage",
  "on_before_take_damage",
  "on_after_take_damage",
  "on_before_heal",
  "on_turn_start",
  "on_turn_end",
  "while_active",
  "on_attach",
  "on_detach",
  "on_kill",
  "on_unit_death"
]

function localizeWithFallback(key: string, fallback: string, params?: Record<string, unknown>) {
  const localized = params ? t(key, params) : t(key)
  return localized !== key && localized.trim() !== "" ? localized : fallback
}

function localizeBattleStatSource(sourceKey: string): string {
  const normalized = String(sourceKey || "").trim()
  if (!normalized) return "-"
  if (normalized === "base") return localizeWithFallback("game.attrSourceBase", "Base")
  if (normalized === "__derived__") return localizeWithFallback("game.attrSourceDerived", "Derived")

  const rule = getBuffEffectRule(normalized)
  if (rule) {
    return localizeBattleBuffSource(rule.buffId, rule.trigger)
  }

  for (const trigger of BATTLE_STAT_SOURCE_TRIGGERS) {
    const suffix = `_${trigger}`
    if (!normalized.endsWith(suffix)) continue
    const buffId = normalized.slice(0, -suffix.length)
    if (!buffId) continue
    return localizeBattleBuffSource(buffId, trigger)
  }

  return normalized
}

function localizeBattleBuffSource(buffId: string, trigger: string): string {
  const buffName = localizeWithFallback(`buff.${buffId}.name`, buffId)
  const triggerName = localizeWithFallback(`game.buffTrigger_${trigger}`, trigger)
  return localizeWithFallback("game.attrSourceBattleBuff", `${buffName} (${triggerName})`, {
    source: buffName,
    trigger: triggerName
  })
}

const getStatsList = (type: 'dialog' | 'popover') => {
  const stats = props.unit.attributes || props.unit.stats || {}
  const statSources: Record<string, Record<string, unknown>> =
    (props.unit.statSources as Record<string, Record<string, unknown>> | undefined) ?? {}
  const baseStats: Record<string, number> = (props.unit.baseStats as Record<string, number> | undefined) ?? {}
  const keys = sortAttributeKeysForScene(
    (type == 'dialog' ? "battle_unit_card" : 'battle_unit_card_popover'),
    Object.keys(stats).filter((key) => Number.isFinite(Number(stats[key]))),
    "battle"
  )
  const rows = keys.map((key) => {
    const value = Number(stats[key] ?? 0)
    return {
      key,
      value
    }
  })
  return rows.map(({ key, value }) => {
      const attrKey = `attr.${key}.name`
      const localized = t(attrKey)
      const breakdownLines: string[] = []
      const baseValue = Number(baseStats[key] ?? NaN)
      if (Number.isFinite(baseValue)) {
        breakdownLines.push(`- ${localizeBattleStatSource("base")}: ${formatAttributeValueByKey(key, baseValue)}`)
      }
      const sourceRow = statSources[key]
      if (sourceRow && typeof sourceRow === "object") {
        for (const [sourceKey, sourceValue] of Object.entries(sourceRow)) {
          const sourceNum = Number(sourceValue)
          if (!Number.isFinite(sourceNum) || sourceNum === 0) continue
          breakdownLines.push(`- ${localizeBattleStatSource(sourceKey)}: ${formatAttributeValueByKey(key, sourceNum)}`)
        }
      }
      const rawValue = Number(value)
      let valueSuffix: string | undefined
      if (DEFENSE_REDUCTION_KEYS.has(key) && rawValue > 0) {
        const pct = (rawValue / (DEFENSE_K + rawValue)) * 100
        valueSuffix = ` (${formatDecimal(pct, 1)}%)`
      }
      return {
        key,
        name: localized !== attrKey ? localized : key,
        desc: "",
        value: formatAttributeValueByKey(key, rawValue),
        valueSuffix,
        rawValue,
        compositionLines: breakdownLines,
        impactLines: [],
        breakdownLines,
        category: "battle",
        growthType: "static",
        order: 0,
        level: 1,
        exp: 0,
        nextExp: 0,
        progress: 0,
        skillPointLevel: 0,
        allowBattlePointLevelUp: false
      }
    })
}

const statsList = computed<AttributeRow[]>(() => {
  return getStatsList("dialog")
})

const statsPreviewList = computed(() => {
  return getStatsList('popover').slice(0, getScenePreviewLimit("battle_unit_card_popover", 9))
})
const statsOverflowCount = computed(() => Math.max(0, statsList.value.length - statsPreviewList.value.length))

const statsPrimaryRows = computed(() => {
  const rowMap = new Map(statsList.value.map((row) => [row.key.toLowerCase(), row]))
  return sortAttributeKeysForScene("battle_unit_card", statsList.value.map((row) => row.key), "battlePrimary")
    .map((key) => rowMap.get(key.toLowerCase()))
    .filter((row): row is AttributeRow => Boolean(row))
})

const statsDialogSections = computed(() => {
  const primaryKeySet = new Set(statsPrimaryRows.value.map((row) => row.key.toLowerCase()))
  const rowMap = new Map(statsList.value.map((row) => [row.key.toLowerCase(), row]))
  const secondaryRows = sortAttributeKeysForScene("battle_unit_card", statsList.value.map((row) => row.key), "battle")
    .map((key) => rowMap.get(key.toLowerCase()))
    .filter((row): row is AttributeRow => Boolean(row) && !primaryKeySet.has(row.key.toLowerCase()))
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

const skillList = computed(() => {
  const cooldownMap = props.unit.skillCooldowns || {}
  const legacySkills = Array.isArray(props.unit.skills) ? props.unit.skills : []
  const legacySkillMap = new Map(legacySkills.map((skill) => [String(skill.id || "").trim(), skill]))
  const skillIDs = Array.isArray(props.unit.skillIds)
    ? props.unit.skillIds.map((item) => String(item || "").trim()).filter(Boolean)
    : []
  if (skillIDs.length > 0) {
    return skillIDs.map((skillID) => {
      const legacy = legacySkillMap.get(skillID)
      const cooldownRemain = Math.max(0, Math.trunc(Number(cooldownMap[skillID] ?? legacy?.cooldownRemain ?? 0)))
      const cooldownTurns = Math.max(
        0,
        Math.trunc(Number(legacy?.cooldownTurns ?? legacy?.cooldown ?? getSkillDefinition(skillID)?.cooldown ?? 0))
      )
      return {
        id: skillID,
        cooldownTurns,
        cooldownRemain,
        level: Math.max(1, Math.trunc(Number(legacy?.level ?? 1))),
        runtimeSkill: legacy && typeof legacy === "object" ? (legacy as Record<string, unknown>) : null
      }
    })
  }
  const skills = Array.isArray(props.unit.skills) ? props.unit.skills : []
  return skills
    .map((skill) => ({
      id: String(skill.id || ""),
      cooldownTurns: Math.max(0, Math.trunc(Number(skill.cooldownTurns || 0))),
      cooldownRemain: Math.max(0, Math.trunc(Number(skill.cooldownRemain || 0))),
      level: Math.max(1, Math.trunc(Number(skill.level ?? 1))),
      runtimeSkill: skill && typeof skill === "object" ? (skill as Record<string, unknown>) : null
    }))
    .filter((skill) => skill.id)
})

const skillPreviewList = computed(() => skillList.value.slice(0, previewSkillLimit))
const skillOverflowCount = computed(() => Math.max(0, skillList.value.length - skillPreviewList.value.length))

const battleItemList = computed(() => {
  const itemRows = Array.isArray(props.unit.battleItems) ? props.unit.battleItems : []
  if (itemRows.length > 0) {
    return itemRows
      .map((item) => ({
        itemId: String(item.itemId || "").trim(),
        quantity: Math.max(0, Math.trunc(Number(item.quantity || 0))),
        cooldownTurns: Math.max(0, Math.trunc(Number(item.cooldownTurns || 0))),
        cooldownRemain: Math.max(0, Math.trunc(Number(item.cooldownRemain || 0)))
      }))
      .filter((item) => item.itemId && item.quantity > 0)
  }
  const quantityMap = props.unit.itemQuantities || {}
  const cooldownMap = props.unit.itemCooldowns || {}
  const itemIDs = Array.from(
    new Set([
      ...Object.keys(quantityMap).map((key) => String(key || "").trim()),
      ...Object.keys(cooldownMap).map((key) => String(key || "").trim())
    ])
  ).filter(Boolean)
  return itemIDs.map((itemId) => ({
    itemId,
    quantity: Math.max(0, Math.trunc(Number(quantityMap[itemId] || 0))),
    cooldownTurns: Math.max(0, Math.trunc(Number(cooldownMap[itemId] || 0))),
    cooldownRemain: Math.max(0, Math.trunc(Number(cooldownMap[itemId] || 0)))
  })).filter((item) => item.quantity > 0)
})

const battleItemPreviewList = computed(() => battleItemList.value.slice(0, previewBattleItemLimit))
const battleItemOverflowCount = computed(() => Math.max(0, battleItemList.value.length - battleItemPreviewList.value.length))

const equipmentList = computed(() => {
  const instanceIDs = Array.isArray(props.unit.equipmentInstanceIds) ? props.unit.equipmentInstanceIds : []
  if (instanceIDs.length > 0) {
    return instanceIDs
      .map((item) => Math.trunc(Number(item)))
      .filter((item) => Number.isFinite(item) && item > 0)
      .map((item) => `#${item}`)
  }
  const digestRows = Array.isArray(props.unit.equipmentDigest) ? props.unit.equipmentDigest : []
  return digestRows.map((row) => {
    const [templateID, enhanceLevel] = String(row || "").split("+", 2)
    const trimmedID = templateID.trim()
    if (!trimmedID) return String(row || "")
    const itemKey = `item.${trimmedID}.name`
    const localized = t(itemKey)
    const itemName = localized !== itemKey ? localized : trimmedID
    const level = Number(enhanceLevel || 0)
    if (!Number.isFinite(level) || level <= 0) return itemName
    return `${itemName} +${Math.trunc(level)}`
  })
})

const buffList = computed(() => {
  const buffs = Array.isArray(props.unit.buffs) ? props.unit.buffs : []
  const rows: Array<{ buffId: string; remainTurns?: number; payload: Record<string, unknown>; stackCount: number }> = []
  const seen = new Map<string, number>()
  for (const buff of buffs) {
    const buffId = String(buff.buffId || "").trim()
    if (!buffId) continue
    const definition = getBuffDefinition(buffId)
    const isStacking = definition?.stackPolicy === "stack"
    const remainTurns = Number(buff.remainTurns ?? -1)
    const payload = buff.payload && typeof buff.payload === "object" ? buff.payload : null
    const triggered = Number(buff.triggered ?? 0)
    const spentCharges = Number.isFinite(triggered) ? Math.max(0, Math.trunc(triggered)) : 0
    const remainingCharges = definition && definition.maxCharges > 1
      ? Math.max(1, definition.maxCharges - spentCharges)
      : 1
    if (isStacking) {
      const idx = seen.get(buffId)
      if (idx !== undefined) {
        rows[idx].stackCount += 1
      } else {
        seen.set(buffId, rows.length)
        rows.push({
          buffId,
          remainTurns: Number.isFinite(remainTurns) ? Math.trunc(remainTurns) : undefined,
          payload: payload || {},
          stackCount: 1
        })
      }
    } else {
      rows.push({
        buffId,
        remainTurns: Number.isFinite(remainTurns) ? Math.trunc(remainTurns) : undefined,
        payload: payload || {},
        stackCount: remainingCharges
      })
    }
  }
  return rows
})

const buffPreviewList = computed(() => buffList.value.slice(0, previewBuffLimit))
const buffOverflowCount = computed(() => Math.max(0, buffList.value.length - buffPreviewList.value.length))
const battleCardBuffList = computed(() => {
  return buffList.value.filter((buff) => {
    const definition = getBuffDefinition(buff.buffId)
    return Boolean(definition?.showInBattleCard) && String(definition?.icon || "").trim() !== ""
  })
})

const basicAttackSummary = computed(() => {
  const rawProfile = (props.unit.basicAttack && typeof props.unit.basicAttack === "object"
    ? props.unit.basicAttack
    : null) as BasicAttackProfile | null
  const rawDamageType = String(rawProfile?.damageType || "physical").trim().toLowerCase()
  const damageType = rawDamageType === "magical" ? "magical" : "physical"
  const targetDirection = String(rawProfile?.targetDirection || "vanguard").trim().toLowerCase()
  const targetSeek = String(rawProfile?.targetSeekStrategy || "front_to_rear").trim().toLowerCase()
  const parsedFormula = resolveBasicAttackFormulaTerms(rawProfile)
  const formulaTerms = parsedFormula.length > 0
    ? parsedFormula
    : [{ attrKey: "physics_damage", coefficient: 1 }]
  const isFallback = parsedFormula.length === 0
  const evaluatedTerms = formulaTerms.map((term) => {
    const attrValue = Number(unitAttributes.value[term.attrKey] ?? 0)
    const coefficient = Number(term.coefficient)
    const termValue = attrValue * coefficient
    const attrLabel = localizeEffectAttr(term.attrKey, t)
    return {
      attrLabel,
      attrValue,
      coefficient,
      termValue
    }
  })

  function normalizeBasicAttackFormulaTerms(rawTerms: BasicAttackFormulaTerm[] | undefined) {
    if (!Array.isArray(rawTerms)) return []
    return rawTerms
      .map((term) => ({
        attrKey: String(term?.attrKey || "").trim(),
        coefficient: Number(term?.coefficient ?? NaN)
      }))
      .filter((term) => term.attrKey && Number.isFinite(term.coefficient))
  }

  function resolveBasicAttackFormulaTerms(rawProfile: BasicAttackProfile | null) {
    const directTerms = normalizeBasicAttackFormulaTerms(rawProfile?.formula)
    if (directTerms.length > 0) return directTerms

    const damageSpecTerms = normalizeBasicAttackFormulaTerms(rawProfile?.damageSpec?.amountFormula?.terms)
    if (damageSpecTerms.length > 0) return damageSpecTerms

    const effects = Array.isArray(rawProfile?.effects) ? rawProfile?.effects : []
    for (const effect of effects) {
      const kind = String(effect?.kind || "").trim().toLowerCase()
      if (kind && kind !== "damage") continue
      const effectTerms = normalizeBasicAttackFormulaTerms(effect?.amountFormula?.terms)
      if (effectTerms.length > 0) return effectTerms
      const effectSpecTerms = normalizeBasicAttackFormulaTerms(effect?.damageSpec?.amountFormula?.terms)
      if (effectSpecTerms.length > 0) return effectSpecTerms
    }
    return []
  }

  const total = evaluatedTerms.reduce((sum, term) => sum + term.termValue, 0)
  const formulaLine = evaluatedTerms.map((term) => `${formatFloat(term.coefficient)}×${term.attrLabel}`).join(" + ")
  const termLines = evaluatedTerms.map((term) => {
    return t("game.battleUnitBasicAttackTermLine", {
      attr: term.attrLabel,
      value: formatFloat(term.attrValue),
      coefficient: formatFloat(term.coefficient),
      result: formatFloat(term.termValue)
    })
  })
  return {
    damageTypeLabel: t(`game.damageType.${damageType}`),
    targetDirectionLabel: t(`game.targetDirection.${targetDirection}`),
    targetSeekLabel: t(`game.targetSeekStrategy.${targetSeek}`),
    formulaLine,
    totalValue: formatFloat(total),
    termLines,
    isFallback
  }
})

const basicAttackEffectGroups = computed(() => {
  const rawProfile = (props.unit.basicAttack && typeof props.unit.basicAttack === "object"
    ? props.unit.basicAttack
    : null) as BasicAttackProfile | null
  const runtimeEffects = Array.isArray(rawProfile?.effects) ? rawProfile.effects : []
  if (runtimeEffects.length > 0) {
    return runtimeEffects.map((effect, index) => {
      const targeting = effect?.targeting || {}
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
          key: "select-rule",
          label: t("game.skillSelectRuleLabel"),
          value: localizeSelectRule(String(targeting.selectRule || "first"), String(targeting.selectAttrKey || ""))
        },
        {
          key: "target-count",
          label: t("game.skillTargetBaseCount"),
          value: localizeTargetCountText(targeting.targetCount, t)
        }
      ]
      const targetCountMax = localizeTargetCountMaxText(targeting.targetCount, t)
      if (targetCountMax !== null) {
        details.push({
          key: "target-count-max",
          label: t("game.skillTargetMaxCount"),
          value: targetCountMax,
          labelHint: t("game.skillTargetMaxCountHint")
        })
      }
      if (Array.isArray(targeting.targetPredicates) && targeting.targetPredicates.some((p) => p?.kind === "target_hp_not_full")) {
        const hpNotFullKey = "game.targetPredicateHpNotFull"
        const hpNotFull = t(hpNotFullKey)
        const conditionsKey = "game.skillTargetConditions"
        const conditions = t(conditionsKey)
        details.push({
          key: "target-predicate-hp-not-full",
          label: conditions === conditionsKey ? "目标条件" : conditions,
          value: hpNotFull === hpNotFullKey ? "生命值不满" : hpNotFull
        })
      }
      if (targeting.fallbackSelfIfEmpty) {
        details.push({
          key: "fallback-self-if-empty",
          label: t("game.skillFallbackSelfIfEmpty"),
          value: t("game.yes")
        })
      }
      if (Number.isFinite(Number(effect?.reuseTargetsFromEffect)) && Number(effect?.reuseTargetsFromEffect) > 0) {
        details.push({
          key: "reuse-targets",
          label: t("game.effectReuseTargetsFromEffect"),
          value: t("game.effectRefIndex", { index: Math.trunc(Number(effect.reuseTargetsFromEffect)) })
        })
      }
      if (effect?.kind === "damage" || effect?.kind === "heal") {
        const amountFormula = effect?.amountFormula || effect?.damageSpec?.amountFormula
        const formulaText = localizeAmountFormulaText(amountFormula?.baseAdd, amountFormula?.terms, t)
        const previewValue = evaluateAmountFormulaPreview(
          { baseAdd: Number(amountFormula?.baseAdd || 0), terms: Array.isArray(amountFormula?.terms) ? amountFormula.terms : [] },
          unitAttributes.value,
          1
        )
        const preview = previewValue === null ? undefined : `(${formatFloat(previewValue)})`
        details.push(
          {
            key: "damage-type",
            label: t("game.battleUnitBasicAttackDamageType"),
            value: t(`game.damageType.${String(effect?.damageType || "physical")}`)
          },
          {
            key: "formula",
            label: t("game.battleUnitBasicAttackFormula"),
            value: preview ? `${formulaText} ${preview}` : formulaText
          }
        )
      }
      if (effect?.kind === "heal" && effect?.mpCost) {
        const mpCostBase = Number(effect.mpCost.baseAdd || 0)
        const mpCostTerms = Array.isArray(effect.mpCost.terms) ? effect.mpCost.terms : []
        if (mpCostBase !== 0 || mpCostTerms.length > 0) {
          const formulaText = localizeAmountFormulaText(mpCostBase, mpCostTerms, t)
          const labelKey = "game.itemDetailBasicAttackMpCost"
          const localized = t(labelKey)
          const hintKey = "game.itemDetailBasicAttackMpCostHint"
          const hint = t(hintKey)
          details.push({
            key: "mp-cost",
            label: localized === labelKey ? "魔法消耗" : localized,
            value: formulaText,
            labelHint: hint === hintKey ? "魔法不足时此效果被跳过，触发后续备选效果（如有）" : hint
          })
        }
      }
      if (effect?.kind === "status" && Array.isArray(effect?.statusPayload?.statusIds) && effect.statusPayload.statusIds.length > 0) {
        details.push({
          key: "status-ids",
          label: t("game.skillEffectStatusIds"),
          value: "-",
          buffIds: effect.statusPayload.statusIds
        })
        const baseTurns = Math.max(0, Math.trunc(Number(effect.statusPayload.durationBaseTurns || 0)))
        const perLevel = Number(effect.statusPayload.durationPerSkillLevelTurns || 0)
        const duration = perLevel ? `${baseTurns} + ${formatFloat(perLevel)}/Lv` : String(baseTurns)
        details.push({
          key: "status-duration",
          label: t("game.skillEffectStatusDuration"),
          value: duration
        })
      }
      // details.push({
      //   key: "raw-config",
      //   label: t("game.skillEffectRawConfig"),
      //   value: formatRawJson(effect)
      // })
      const onlyWhenRefs = Array.isArray(effect?.onlyWhenNoTargetFromEffects) ? effect.onlyWhenNoTargetFromEffects : undefined
      const mpCostGatedRefs = onlyWhenRefs
        ? onlyWhenRefs.filter((ref) => {
            const refEffect = runtimeEffects[ref - 1]
            if (!refEffect?.mpCost) return false
            const base = Number(refEffect.mpCost.baseAdd || 0)
            const terms = Array.isArray(refEffect.mpCost.terms) ? refEffect.mpCost.terms : []
            return base !== 0 || terms.length > 0
          })
        : undefined
      return {
        key: `effect-${index + 1}`,
        title: `${t("game.skillEffectTitle")} ${index + 1}`,
        details,
        linkage: {
          onlyWhenNoTargetFromEffects: onlyWhenRefs,
          reuseTargetsFromEffect: Number.isFinite(Number(effect?.reuseTargetsFromEffect)) ? Number(effect.reuseTargetsFromEffect) : undefined,
          mpCostGatedRefs: mpCostGatedRefs && mpCostGatedRefs.length > 0 ? mpCostGatedRefs : undefined
        }
      }
    })
  }

  const formulaValue = basicAttackSummary.value.termLines.length > 0
    ? `${basicAttackSummary.value.formulaLine}\n${basicAttackSummary.value.termLines.join("\n")}`
    : basicAttackSummary.value.formulaLine
  const details: BasicAttackDetail[] = [
    {
      key: "damage-type",
      label: t("game.battleUnitBasicAttackDamageType"),
      value: basicAttackSummary.value.damageTypeLabel
    },
    {
      key: "target-direction",
      label: t("game.battleUnitBasicAttackTargetDirection"),
      value: basicAttackSummary.value.targetDirectionLabel
    },
    {
      key: "target-seek",
      label: t("game.battleUnitBasicAttackTargetSeek"),
      value: basicAttackSummary.value.targetSeekLabel
    },
    {
      key: "formula",
      label: t("game.battleUnitBasicAttackFormula"),
      value: formulaValue
    }
  ]
  return [
    {
      key: "effect-1",
      title: `${t("game.skillEffectTitle")} 1`,
      details
    }
  ]
})

const basicAttackDetails = computed<BasicAttackDetail[]>(() => [])

function localizeTargetPositions(positions: string[] | undefined) {
  const rows = Array.isArray(positions) ? positions : []
  if (rows.length === 0) return t("game.targetDirection.vanguard")
  return rows.map((position) => t(`game.targetDirection.${String(position || "vanguard")}`)).join(" / ")
}

function localizeSelectRule(rule: string, attrKey: string): string {
  const ruleKey = `game.skillSelectRule.${rule}`
  const localizedRule = t(ruleKey) !== ruleKey ? t(ruleKey) : rule
  if (rule === "attr_max" || rule === "attr_min") {
    const i18nKey = `attr.${attrKey}.name`
    const attrName = t(i18nKey) !== i18nKey ? t(i18nKey) : attrKey
    return `${localizedRule}: ${attrName}`
  }
  return localizedRule
}

function formatStat(value: number | undefined) {
  if (typeof value !== "number" || !Number.isFinite(value)) return "0"
  return String(Math.ceil(value))
}

function formatFloat(value: number) {
  if (!Number.isFinite(value)) return "0"
  const normalized = Math.round(value * 100) / 100
  if (Number.isInteger(normalized)) return String(normalized)
  return normalized.toFixed(2).replace(/\.0+$/, "").replace(/(\.\d*[1-9])0+$/, "$1")
}

function toPercent(current: number | undefined, max: number | undefined) {
  const currentNum = Number(current || 0)
  const maxNum = Number(max || 0)
  if (!Number.isFinite(currentNum) || !Number.isFinite(maxNum) || maxNum <= 0) return 0
  return Math.max(0, Math.min(100, Math.round((currentNum / maxNum) * 100)))
}

function openDetailDialog() {
  detailDialogVisible.value = true
}

</script>

<style scoped>
.battle-unit-card {
  cursor: pointer;
}

.battle-unit-active {
  border-color: rgba(245, 158, 11, 0.9);
  box-shadow: 0 0 0 1px rgba(245, 158, 11, 0.45), 0 0 12px rgba(245, 158, 11, 0.35);
  animation: battle-unit-pulse 1.1s ease-in-out infinite;
}

.battle-unit-hovered-order {
  border-color: rgba(56, 189, 248, 0.95);
  box-shadow: 0 0 0 1px rgba(56, 189, 248, 0.5), 0 0 14px rgba(56, 189, 248, 0.35);
  animation: none;
}

@keyframes battle-unit-pulse {
  0% {
    box-shadow: 0 0 0 1px rgba(245, 158, 11, 0.36), 0 0 10px rgba(245, 158, 11, 0.26);
  }
  50% {
    box-shadow: 0 0 0 1px rgba(245, 158, 11, 0.58), 0 0 18px rgba(245, 158, 11, 0.5);
  }
  100% {
    box-shadow: 0 0 0 1px rgba(245, 158, 11, 0.36), 0 0 10px rgba(245, 158, 11, 0.26);
  }
}

.battle-unit-dead {
  opacity: 0.45;
  filter: grayscale(0.9);
}

.battle-unit-dead.battle-unit-active {
  animation: none;
}

.battle-unit-preview-stats {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0 10px;
  margin-top: 4px;
  border: 1px solid color-mix(in srgb, var(--app-border) 72%, transparent);
  border-radius: 6px;
  padding: 4px 8px;
  background: color-mix(in srgb, var(--app-surface) 86%, var(--app-surface-elevated));
}

.battle-unit-popover-content {
  font-size: 11px;
  font-weight: 400;
  line-height: 15px;
}

.battle-unit-popover-content .font-medium,
.battle-unit-popover-content :deep(.font-medium),
.battle-unit-popover-content :deep(.font-semibold),
.battle-unit-popover-content :deep(.font-bold) {
  font-weight: 400;
}

.battle-unit-preview-stats .battle-unit-stat-row {
  padding: 4px 0;
  font-size: 11px;
  line-height: 14px;
}

.battle-unit-preview-stats .battle-unit-stat-row__name,
.battle-unit-preview-stats .battle-unit-stat-row__value {
  font-weight: 400;
}

.battle-unit-detail-stats {
  display: grid;
  gap: 8px;
  margin-top: 4px;
}

.battle-unit-stat-card {
  min-width: 0;
  overflow: hidden;
  border: 1px solid color-mix(in srgb, var(--app-border) 76%, transparent);
  border-radius: 6px;
  background: color-mix(in srgb, var(--app-surface) 88%, var(--app-surface-elevated));
}

.battle-unit-stat-card__header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  border-bottom: 1px solid color-mix(in srgb, var(--app-border) 64%, transparent);
  padding: 6px 8px;
  color: var(--app-muted);
  font-size: 12px;
  font-weight: 700;
  line-height: 16px;
}

.battle-unit-stat-card__header span:first-child {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.battle-unit-stat-card__grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(132px, 1fr));
  gap: 0 10px;
  padding: 2px 8px;
}

.battle-unit-stat-columns {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(120px, 1fr));
  gap: 0 10px;
  padding: 0 8px;
}

.battle-unit-stat-column {
  min-width: 0;
}

.battle-unit-stat-column__title {
  padding: 6px 0 2px;
  color: var(--app-muted);
  font-size: 11px;
  font-weight: 700;
  line-height: 14px;
}

.battle-unit-stat-list {
  display: grid;
  min-width: 0;
  padding: 2px 8px;
}

.battle-unit-stat-column .battle-unit-stat-list {
  padding: 0;
}

.battle-unit-stat-row {
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

.battle-unit-stat-row:last-child {
  border-bottom: 0;
}

.battle-unit-stat-row--dialog {
  grid-template-columns: auto minmax(0, 1fr);
}

.battle-unit-stat-row__name {
  min-width: 0;
  overflow: hidden;
  color: var(--app-text);
  font-weight: 600;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.battle-unit-stat-row__value {
  min-width: 0;
  color: var(--app-text);
  font-weight: 700;
  text-align: right;
  overflow-wrap: anywhere;
}

</style>
