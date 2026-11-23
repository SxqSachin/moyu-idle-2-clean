<template>
  <div class="affix-popover-content">
    <div class="affix-popover-name">{{ affixName }}</div>
    <!-- <div v-if="affixScore" class="affix-popover-score">
      <strong>{{ affixScore.score }}</strong>
    </div> -->
    <div v-if="affixDesc" class="affix-popover-desc">{{ affixDesc }}</div>
    <div v-if="limitLines.length > 0" class="affix-popover-limits">
      <div v-for="(line, idx) in limitLines" :key="`affix-popover-limit-${idx}`" class="affix-popover-limit-line">{{ line }}</div>
    </div>
    <div v-if="rarityRows.length === 0" class="affix-popover-empty">{{ t("game.itemDetailAffixNoExtraEffect") }}</div>
    <div v-for="row in rarityRows" :key="`affix-popover-rarity-${row.rarityId}`" class="affix-popover-row">
      <div class="affix-popover-rarity">
        <RarityText :rarity-id="row.rarityId" />
        <span v-if="row.weight !== null" class="affix-popover-weight">
          {{ t("game.itemDetailAffixRollWeight", { weight: formatWeight(row.weight), percent: formatChance(row.chance) }) }}
        </span>
      </div>
      <div class="affix-popover-lines">
        <div
          v-for="(line, idx) in row.lines"
          :key="`affix-popover-line-${row.rarityId}-${idx}`"
          class="affix-popover-line-row"
        >
          <span class="affix-popover-line-label">{{ line.label }}</span>
          <span v-if="line.skillIds?.length" class="affix-popover-line-value affix-popover-skill-list">
            <SkillQuickRef
              v-for="skillId in line.skillIds"
              :key="`affix-popover-skill-${row.rarityId}-${skillId}`"
              :skill-id="skillId"
            />
          </span>
          <span v-else class="affix-popover-line-value">{{ line.value }}</span>
        </div>
        <div v-if="row.hasHidden" class="affix-popover-hook-line" style="display:flex;justify-content:flex-end;">
          <button style="font-size:0.7rem;opacity:0.6;background:transparent;border:0;cursor:pointer;padding:0;" @click="showAllHooksByRarity[row.rarityId] = !showAllHooksByRarity[row.rarityId]">{{ showAllHooksByRarity[row.rarityId] ? t("game.itemDetailBattleHooksShowLess") : t("game.itemDetailBattleHooksShowAll") }}</button>
        </div>
        <template v-for="hook in (showAllHooksByRarity[row.rarityId] ? row.allHooks : row.allHooks.filter((n) => !n.hidden))" :key="`affix-popover-hook-${row.rarityId}-${hook.key}`">
          <div class="affix-popover-hook-line affix-popover-hook-trigger-line">
            {{ hook.triggerText }}
          </div>
          <div
            v-if="hook.conditions.length > 0"
            class="affix-popover-hook-line affix-popover-hook-label-line"
          >
            <span>{{ t("game.itemDetailBattleHookIf") }}</span>
          </div>
          <div
            v-for="condition in hook.conditions"
            :key="`affix-popover-hook-${row.rarityId}-${hook.key}-condition-${condition.key}`"
            class="affix-popover-hook-line affix-popover-hook-content-line"
          >
            <span class="affix-popover-hook-bullet">·</span>{{ condition.text }}
          </div>
          <div
            v-if="hook.conditions.length > 0"
            class="affix-popover-hook-line affix-popover-hook-label-line"
          >
            {{ t("game.itemDetailBattleHookThen") }}
          </div>
          <div
            v-for="effect in hook.effects"
            :key="`affix-popover-hook-${row.rarityId}-${hook.key}-${effect.key}`"
            class="affix-popover-hook-line affix-popover-hook-content-line"
          >
            <span class="affix-popover-hook-bullet">·</span>
            <template v-if="effect.kind === 'add_buff' || effect.kind === 'remove_buff'">
              <span>{{ t(effect.kind === 'add_buff' ? "game.itemDetailBattleHookEffectAddBuffPrefix" : "game.itemDetailBattleHookEffectRemoveBuffPrefix", { target: effect.targetText }) }}</span>
              <BuffQuickRef :buff-id="effect.buffId" />
              <span v-if="effect.kind === 'add_buff' && effect.turns !== null">{{ t("game.itemDetailBattleHookEffectTurnsSuffix", { turns: effect.turns }) }}</span>
              <span v-if="effect.kind === 'add_buff' && effect.procChanceText">{{ effect.procChanceText }}</span>
              <span v-if="effect.kind === 'remove_buff' && effect.removeStack">{{ t("game.itemDetailBattleHookEffectRemoveBuffOneStack") }}</span>
            </template>
            <template v-else-if="effect.kind === 'heal'">
              <span>{{ t("game.itemDetailBattleHookEffectHeal", { target: effect.targetText, formula: effect.formulaText }) }}</span>
              <span v-if="effect.procChanceText">{{ effect.procChanceText }}</span>
            </template>
            <template v-else-if="effect.kind === 'summon'">
              <span>{{ t("game.itemDetailBattleHookEffectSummonPrefix", { target: effect.targetText }) }}</span>
              <EnemyUnitQuickRef :enemy-id="effect.unitId" />
              <span>{{ effect.durationText }}</span>
              <span v-if="effect.countText">{{ effect.countText }}</span>
              <span v-if="effect.procChanceText">{{ effect.procChanceText }}</span>
              <div v-if="effect.statBonusEntries.length" class="basis-full pl-4 pt-1">
                <div v-for="bonus in effect.statBonusEntries" :key="`affix-popover-hook-${row.rarityId}-${hook.key}-${effect.key}-${bonus.key}`">
                  <span class="font-medium">{{ bonus.label }}:</span>
                  <span>{{ bonus.value }}</span>
                </div>
              </div>
            </template>
            <template v-else-if="effect.kind === 'mirror_cast_skill'">
              <span>{{ effect.descriptionText }}</span>
              <div v-if="effect.filterText" class="affix-popover-hook-subline">{{ effect.filterText }}</div>
            </template>
            <template v-else-if="effect.kind === 'basic_attack_marked_target'">
              <span>{{ t("game.itemDetailBattleHookEffectBasicAttackMarkedTargetPrefix") }}</span>
              <BuffEffectQuickRef :effect-kind="effect.effectKind" />
              <span>{{ t("game.itemDetailBattleHookEffectBasicAttackMarkedTargetSuffix") }}</span>
              <span v-if="effect.procChanceText">{{ effect.procChanceText }}</span>
            </template>
            <template v-else>
            {{ formatHookEffectFallback(effect) }}
            </template>
          </div>
        </template>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, reactive } from "vue"
import { useI18n } from "vue-i18n"
import RarityText from "@components/game/common/RarityText.vue"
import BuffEffectQuickRef from "@components/game/buff/BuffEffectQuickRef.vue"
import BuffQuickRef from "@components/game/buff/BuffQuickRef.vue"
import EnemyUnitQuickRef from "@components/game/combat/EnemyUnitQuickRef.vue"
import SkillQuickRef from "@components/game/skill/SkillQuickRef.vue"
import { getAttributeUnit } from "@services/attribute-display"
import { buildBattleHookDescriptions } from "@services/battle-hook-display"
import { buildAffixMutationDisplayLines, type AffixMutationDisplayLine } from "@services/affix-mutation-display"
import { resolveAffixMaxEffectiveLimits } from "@services/equipment-affix-display"
import { getAffixDefinition, listRarityDefinitions } from "@services/item-definition"
import { scoreAffixDefinition } from "@services/combat-scoring"

const props = defineProps<{
  affixId: string
  rarityWeights?: Record<string, number> | null
}>()

const { t } = useI18n()
const showAllHooksByRarity = reactive<Record<string, boolean>>({})
type AffixDisplayLine = { label: string; value: string }
type AffixMutationLine = AffixDisplayLine & Pick<AffixMutationDisplayLine, "skillIds">

const normalizedAffixID = computed(() => normalizeAffixID(props.affixId))
const definition = computed(() => getAffixDefinition(normalizedAffixID.value))
const affixName = computed(() => localizeAffixName(normalizedAffixID.value))
const affixDesc = computed(() => localizeAffixDesc(normalizedAffixID.value))
const affixScore = computed(() => scoreAffixDefinition(normalizedAffixID.value))
const limitLines = computed(() => {
  const limits = resolveAffixMaxEffectiveLimits(definition.value)
  const lines: string[] = []
  if (limits.maxEffectivePerEquipment !== null) {
    lines.push(t("game.itemDetailAffixLimitEquipment", { count: limits.maxEffectivePerEquipment }))
  }
  if (limits.maxEffectivePerBody !== null) {
    lines.push(t("game.itemDetailAffixLimitBody", { count: limits.maxEffectivePerBody }))
  }
  return lines
})

const rarityRows = computed(() => {
  const def = definition.value
  if (!def) return []
  const weightedRarities = resolveWeightedRarities(def.allowedRarities)
  const rarityIDs = weightedRarities.length > 0
    ? weightedRarities.map((row) => row.rarityId)
    : def.allowedRarities.length > 0
      ? def.allowedRarities
      : listRarityDefinitions()
          .map((row) => row.id)
          .filter((id) => String(id || "").trim() !== "")
  const weightByRarity = new Map(weightedRarities.map((row) => [row.rarityId, row]))
  return rarityIDs.map((rarityID) => {
    const allHooks = buildRarityHookDescriptions(rarityID)
    const hasHidden = allHooks.some((n) => n.hidden)
    return {
      rarityId: rarityID,
      weight: weightByRarity.get(rarityID)?.weight ?? null,
      chance: weightByRarity.get(rarityID)?.chance ?? null,
      lines: buildRarityEffectLines(rarityID),
      allHooks,
      hasHidden
    }
  })
})

function resolveWeightedRarities(allowedRarities: string[]): Array<{ rarityId: string; weight: number; chance: number }> {
  const weights = props.rarityWeights
  if (!weights || typeof weights !== "object" || Array.isArray(weights)) return []
  const allowedSet = new Set((allowedRarities || []).map((id) => String(id || "").trim()).filter(Boolean))
  const rows = Object.entries(weights)
    .map(([rarityId, rawWeight]) => ({ rarityId: String(rarityId || "").trim(), weight: Number(rawWeight) }))
    .filter((row) => row.rarityId && Number.isFinite(row.weight) && row.weight > 0)
    .filter((row) => allowedSet.size === 0 || allowedSet.has(row.rarityId))
  const orderByRarity = new Map(listRarityDefinitions().map((row, index) => [row.id, row.order || index + 1]))
  rows.sort((a, b) => {
    const orderA = orderByRarity.get(a.rarityId) ?? Number.MAX_SAFE_INTEGER
    const orderB = orderByRarity.get(b.rarityId) ?? Number.MAX_SAFE_INTEGER
    return (orderA - orderB) || a.rarityId.localeCompare(b.rarityId)
  })
  const total = rows.reduce((sum, row) => sum + row.weight, 0)
  if (total <= 0) return []
  return rows.map((row) => ({ ...row, chance: row.weight / total }))
}

function normalizeAffixID(affixID: string): string {
  const normalized = String(affixID || "").trim()
  if (normalized.startsWith("affix.")) return normalized.slice("affix.".length)
  return normalized
}

function localizeAffixName(affixID: string): string {
  const keys = [`affix.${affixID}.name`, `${affixID}.name`].filter(Boolean)
  for (const key of keys) {
    const localized = t(key)
    if (localized !== key && localized.trim() !== "") return localized
  }
  return affixID
}

function localizeAffixDesc(affixID: string): string {
  const keys = [`affix.${affixID}.desc`, `${affixID}.desc`].filter(Boolean)
  for (const key of keys) {
    const localized = t(key)
    if (localized !== key && localized.trim() !== "") return localized
  }
  return ""
}

function buildRarityEffectLines(rarityID: string): AffixMutationLine[] {
  const def = definition.value
  if (!def) return []
  const effectParams = resolveAffixEffectParamsByRarity(def.effectParams || {}, rarityID)
  const lines: AffixMutationLine[] = []

  const rawAttrMods = effectParams.attrMods
  if (rawAttrMods && typeof rawAttrMods === "object" && !Array.isArray(rawAttrMods)) {
    for (const [attrKey, raw] of Object.entries(rawAttrMods as Record<string, unknown>)) {
      const range = resolveNumericRange(raw)
      if (!range) continue
      lines.push({
        label: localizeAttr(attrKey),
        value: formatSignedRangeByAttr(attrKey, range.min, range.max, def.valuePrecision)
      })
    }
  }

  const mutationLines = buildRarityMutationLines(effectParams)
  lines.push(...mutationLines)

  if (lines.length === 0 && buildRarityHookDescriptions(rarityID).filter((n) => !n.hidden).length === 0) {
    return [{ label: t("game.itemDetailAffixNoExtraEffect"), value: "-" }]
  }
  return lines
}

function buildRarityMutationLines(effectParams: Record<string, unknown>): AffixMutationLine[] {
  return buildAffixMutationDisplayLines(effectParams.effectMutations, t)
}

function buildRarityHookDescriptions(rarityID: string) {
  const def = definition.value
  if (!def) return []
  const effectParams = resolveAffixEffectParamsByRarity(def.effectParams || {}, rarityID)
  return buildBattleHookDescriptions(effectParams.battleHooks, t)
}

function formatHookEffectFallback(effect: { text?: string; formulaText?: string }): string {
  return String(effect.text || effect.formulaText || "")
}

function localizeAttr(attrKey: string): string {
  const key = `attr.${attrKey}.name`
  const localized = t(key)
  return localized === key ? attrKey : localized
}

function resolveAffixEffectParamsByRarity(effectParams: Record<string, unknown>, rarityID: string): Record<string, unknown> {
  const out: Record<string, unknown> = {}
  for (const [key, raw] of Object.entries(effectParams || {})) {
    out[key] = selectRarityScopedParam(raw, rarityID)
  }
  return out
}

function selectRarityScopedParam(raw: unknown, rarityID: string): unknown {
  if (!raw || typeof raw !== "object" || Array.isArray(raw)) return raw
  const node = raw as Record<string, unknown>
  const normalizedRarity = String(rarityID || "").trim()
  if (normalizedRarity && node[normalizedRarity] !== undefined) return node[normalizedRarity]
  if (node.default !== undefined) return node.default
  if (node["*"] !== undefined) return node["*"]
  return raw
}

function resolveNumericRange(raw: unknown): { min: number; max: number } | null {
  const direct = toFiniteNumber(raw)
  if (direct !== null) return { min: direct, max: direct }
  if (!raw || typeof raw !== "object" || Array.isArray(raw)) return null
  const node = raw as Record<string, unknown>
  const minRaw = toFiniteNumber(node.min)
  const maxRaw = toFiniteNumber(node.max)
  if (minRaw === null && maxRaw === null) return null
  const min = minRaw === null ? maxRaw! : minRaw
  const max = maxRaw === null ? minRaw! : maxRaw
  if (max < min) return { min: max, max: min }
  return { min, max }
}

function toFiniteNumber(raw: unknown): number | null {
  if (typeof raw === "number" && Number.isFinite(raw)) return raw
  if (typeof raw === "string" && raw.trim() !== "") {
    const parsed = Number(raw)
    if (Number.isFinite(parsed)) return parsed
  }
  return null
}

function roundByPrecision(value: number, precision: number): number {
  if (precision <= 0) return Math.round(value)
  const pow = 10 ** precision
  return Math.round(value * pow) / pow
}

function formatNumber(value: number, precision: number): string {
  const rounded = roundByPrecision(value, precision)
  const fixed = precision > 0 ? rounded.toFixed(precision) : String(rounded)
  return fixed.replace(/\.0+$/, "").replace(/(\.\d*?)0+$/, "$1")
}

function formatSignedNumber(value: number, precision: number): string {
  const text = formatNumber(value, precision)
  return value >= 0 ? `+${text}` : text
}

function formatSignedRange(min: number, max: number, precision: number, asPercent: boolean): string {
  const factor = asPercent ? 100 : 1
  const left = min * factor
  const right = max * factor
  if (Math.abs(left - right) < 1e-9) {
    return formatSignedNumber(left, precision)
  }
  return `${formatSignedNumber(left, precision)}~${formatSignedNumber(right, precision)}`
}

function formatSignedRangeByAttr(attrKey: string, min: number, max: number, precision: number): string {
  const unit = getAttributeUnit(attrKey)
  return formatSignedRange(min, max, precision, unit === "percent")
}

function formatWeight(value: number): string {
  return formatNumber(value, Number.isInteger(value) ? 0 : 2)
}

function formatChance(value: number | null): string {
  if (!Number.isFinite(Number(value))) return "0"
  return formatNumber(Number(value) * 100, 1)
}
</script>

<style scoped>
.affix-popover-content {
  min-width: 280px;
  max-width: 420px;
  display: grid;
  gap: 8px;
}

.affix-popover-name {
  font-size: 14px;
  font-weight: 600;
}

.affix-popover-desc {
  font-size: 12px;
  color: var(--el-text-color-secondary);
  white-space: pre-wrap;
  word-break: break-word;
}

.affix-popover-score {
  display: flex;
  align-items: baseline;
  gap: 6px;
  font-size: 12px;
  color: var(--el-color-success-dark-2);
}

.affix-popover-score strong {
  font-size: 15px;
}

.affix-popover-empty {
  font-size: 12px;
  color: var(--el-text-color-secondary);
}

.affix-popover-limits {
  font-size: 12px;
  color: var(--el-color-warning-dark-2);
  display: grid;
  gap: 2px;
}

.affix-popover-limit-line {
  line-height: 1.4;
}

.affix-popover-row {
  border: 1px solid color-mix(in srgb, var(--el-text-color-primary) 10%, transparent);
  border-radius: 8px;
  padding: 6px 8px;
  display: grid;
  gap: 4px;
}

.affix-popover-rarity {
  font-size: 12px;
  display: flex;
  align-items: center;
  gap: 6px;
  flex-wrap: wrap;
}

.affix-popover-weight {
  color: var(--el-text-color-secondary);
}

.affix-popover-lines {
  font-size: 12px;
  color: var(--el-text-color-regular);
  display: grid;
  gap: 2px;
}

.affix-popover-line-row {
  display: grid;
  grid-template-columns: 88px 1fr;
  gap: 6px;
}

.affix-popover-line-label {
  color: var(--el-text-color-secondary);
}

.affix-popover-line-value {
  white-space: pre-wrap;
  word-break: break-word;
}

.affix-popover-skill-list {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 4px;
  white-space: normal;
}

.affix-popover-hook-line {
  display: flex;
  align-items: center;
  gap: 6px;
  flex-wrap: wrap;
}

.affix-popover-hook-trigger-line {
  font-weight: 600;
}

.affix-popover-hook-label-line {
  color: var(--el-text-color-secondary);
  padding-left: 8px;
}

.affix-popover-hook-content-line {
  padding-left: 16px;
}

.affix-popover-hook-bullet {
  color: var(--el-text-color-secondary);
}

.affix-popover-hook-subline {
  color: var(--el-text-color-secondary);
  padding-left: 16px;
}
</style>
