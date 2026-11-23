<template>
  <div class="skill-quick-ref-content gap-4">
    <div class="skill-quick-ref-header">
      <div class="skill-quick-ref-title">{{ skillName }}</div>
      <div v-if="skillTypeLabel" class="skill-quick-ref-type-wrap">
        <el-tag size="small" effect="plain" :disable-transitions="true" :style="tagStyle">
          {{ skillTypeLabel }}
        </el-tag>
      </div>

      <div v-if="skillDesc" class="skill-quick-ref-desc">{{ skillDesc }}</div>
      <!-- <div v-if="skillScore" class="skill-quick-ref-score">
        {{ skillScore.score }} 分 / {{ skillScore.goldValue }} 金
      </div>
      <div v-if="skillScoreBreakdown.length" class="skill-quick-ref-score-breakdown">
        <span v-for="entry in skillScoreBreakdown" :key="entry">{{ entry }}</span>
      </div> -->
    </div>

    <div v-for="section in renderedDetailSections" :key="section.key" class="detail-surface">
      <div v-if="section.title" class="skill-quick-ref-section-title detail-section-title">{{ section.title }}</div>
      <div class="skill-quick-ref-section detail-section grid-cols-2">
        <div
          v-for="(row, rowIndex) in section.rows"
          :key="`${section.key}-${row.label}-${rowIndex}`"
          class="skill-detail-row flex flex-row flex-wrap gap-2 items-center"
          :class="{ 'is-changed': row.changed }"
        >
          <div class="skill-quick-ref-label detail-row-label">
            {{ row.label }}:
            <el-tooltip v-if="row.labelHint" :content="row.labelHint" placement="top">
              <el-icon class="skill-quick-ref-label-hint detail-row-hint"><QuestionFilled /></el-icon>
            </el-tooltip>
          </div>
          <div class="skill-quick-ref-value detail-row-value">
            <div v-if="row.changed" class="skill-detail-row__original">
              <span class="skill-detail-row__original-label">{{ t("skillForge.preview.originalValue") }}</span>
              <div v-if="row.originalBuffIds?.length" class="skill-quick-ref-buff-list detail-buff-list">
                <BuffQuickRef v-for="buffId in row.originalBuffIds" :key="`${section.key}-${row.label}-original-${buffId}`" :buff-id="buffId" :remain-turns="-1" />
              </div>
              <span v-else>
                {{ row.originalValue }}
                <span v-if="row.originalPreview" class="skill-quick-ref-value-preview detail-row-preview"> {{ row.originalPreview }}</span>
              </span>
            </div>
            <div class="skill-detail-row__current">
              <span v-if="row.changed" class="skill-detail-row__current-label">{{ t("skillForge.preview.modifiedValue") }}</span>
              <div v-if="row.buffIds?.length" class="skill-quick-ref-buff-list detail-buff-list">
                <BuffQuickRef v-for="buffId in row.buffIds" :key="`${section.key}-${row.label}-${buffId}`" :buff-id="buffId" :remain-turns="-1" />
              </div>
              <span v-else>
                {{ row.value }}
                <span v-if="row.preview" class="skill-quick-ref-value-preview detail-row-preview"> {{ row.preview }}</span>
              </span>
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from "vue"
import { useI18n } from "vue-i18n"
import { QuestionFilled } from "@element-plus/icons-vue"
import BuffQuickRef from "@components/game/buff/BuffQuickRef.vue"
import {
  getLocalizedRuntimeSkillName,
  getLocalizedSkillName,
  getSkillDefinition,
  getSkillTypeDefinition
} from "@services/skill-definition"
import { describeWeaponClassList } from "@services/weapon-class"
import { evaluateAmountFormulaPreview, evaluateTargetCountPreview, localizeTargetCountMaxText, localizeTargetCountText } from "@services/effect-formula-display"
import { buildEffectDetailSections } from "@services/effect-detail-renderer"
import { scoreSkillDefinition } from "@services/combat-scoring"

type DetailRow = {
  label: string
  value: string
  preview?: string
  buffIds?: string[]
  labelHint?: string
  changed?: boolean
  originalValue?: string
  originalPreview?: string
  originalBuffIds?: string[]
}

type DetailSection = {
  key: string
  title: string
  rows: DetailRow[]
}

const props = withDefaults(
  defineProps<{
    skillId: string
    remainingCooldown?: number | null
    totalCooldown?: number | null
    unitAttributes?: Record<string, number> | null
    unitSkillLevel?: number | null
    unitSkillExp?: number | null
    runtimeSkill?: Record<string, unknown> | null
    baselineSkill?: Record<string, unknown> | null
  }>(),
  {
    remainingCooldown: null,
    totalCooldown: null,
    unitAttributes: null,
    unitSkillLevel: null,
    unitSkillExp: null,
    runtimeSkill: null,
    baselineSkill: null
  }
)

const { t } = useI18n()
const normalizedSkillID = computed(() => String(props.skillId || "").trim())
const runtimeSkill = computed(() => {
  if (!props.runtimeSkill || typeof props.runtimeSkill !== "object" || Array.isArray(props.runtimeSkill)) return null
  return props.runtimeSkill as Record<string, unknown>
})
const skillDefinition = computed(() => getSkillDefinition(normalizedSkillID.value))
const skillType = computed(() => getSkillTypeDefinition(String(runtimeSkill.value?.category || skillDefinition.value?.category || "")))
const skillName = computed(() => {
  return getLocalizedRuntimeSkillName(t, normalizedSkillID.value, runtimeSkill.value)
})
const skillScore = computed(() => scoreSkillDefinition(runtimeSkill.value || skillDefinition.value as unknown as Record<string, unknown> | null))
const skillScoreBreakdown = computed(() => {
  return (skillScore.value?.sections || [])
    .slice(0, 3)
    .map((section) => `${section.label} ${section.score}分`)
})
const skillDesc = computed(() => {
  const descKey = String(runtimeSkill.value?.descKey || skillDefinition.value?.descKey || `skill.${normalizedSkillID.value}.desc`)
  const localized = t(descKey)
  if (localized !== descKey && localized.trim() !== "") return localized
  return t("game.skillDescriptionUnknown")
})
const skillTypeLabel = computed(() => {
  if (!skillType.value) return ""
  const localized = t(skillType.value.nameKey)
  if (localized !== skillType.value.nameKey && localized.trim() !== "") return localized
  return skillType.value.id
})
const baseCooldown = computed(() => Math.max(0, Math.trunc(Number(runtimeSkill.value?.cooldown ?? skillDefinition.value?.cooldown ?? 0))))
const customForge = computed(() => {
  const raw = runtimeSkill.value?.customForge || skillDefinition.value?.customForge
  if (!raw || typeof raw !== "object" || Array.isArray(raw)) return null
  const row = raw as Record<string, unknown>
  const budget = row.budget && typeof row.budget === "object" && !Array.isArray(row.budget)
    ? row.budget as Record<string, unknown>
    : {}
  return {
    sourceKind: String(row.sourceKind || "").trim(),
    sourceSkillId: String(row.sourceSkillId || "").trim(),
    budget: {
      used: Number(budget.used || 0),
      limit: Number(budget.limit || 0),
      refund: Number(budget.refund || 0),
      failChance: Number(budget.failChance || 0)
    }
  }
})
const resolvedTargeting = computed(() => {
  const runtime = runtimeSkill.value?.targeting
  if (runtime && typeof runtime === "object" && !Array.isArray(runtime)) {
    return runtime as NonNullable<typeof skillDefinition.value>["targeting"]
  }
  return skillDefinition.value?.targeting
})
const resolvedEffects = computed(() => {
  const runtime = runtimeSkill.value?.effects
  if (Array.isArray(runtime)) return runtime as NonNullable<typeof skillDefinition.value>["effects"]
  return Array.isArray(skillDefinition.value?.effects) ? skillDefinition.value.effects : []
})
const resolvedCastGuards = computed(() => {
  const runtime = runtimeSkill.value?.castGuards
  if (Array.isArray(runtime)) return runtime as NonNullable<typeof skillDefinition.value>["castGuards"]
  return skillDefinition.value?.castGuards || []
})
const resolvedStatusRequirement = computed<Record<string, number>>(() => {
  const raw = runtimeSkill.value?.statusRequirement ?? skillDefinition.value?.statusRequirement
  if (!raw || typeof raw !== "object" || Array.isArray(raw)) return {}
  return Object.fromEntries(Object.entries(raw as Record<string, unknown>)
    .map(([key, value]) => [key, Number(value)] as const)
    .filter(([, value]) => Number.isFinite(value) && value > 0))
})
const resolvedWeaponRequirement = computed<string[]>(() => {
  const raw = runtimeSkill.value?.weaponRequirement ?? skillDefinition.value?.weaponRequirement
  return Array.isArray(raw) ? raw.map((value) => String(value || "").trim()).filter(Boolean) : []
})
const totalCooldown = computed(() => {
  if (props.totalCooldown === null || props.totalCooldown === undefined) {
    const runtimeCooldown = Number(runtimeSkill.value?.cooldownTurns ?? runtimeSkill.value?.cooldown ?? Number.NaN)
    if (Number.isFinite(runtimeCooldown)) return Math.max(0, Math.trunc(runtimeCooldown))
    return baseCooldown.value
  }
  return Math.max(0, Math.trunc(Number(props.totalCooldown || 0)))
})
const tagStyle = computed(() => {
  const color = skillType.value?.color || ""
  if (!color) return {}
  return {
    borderColor: color,
    color
  }
})

const detailSections = computed<DetailSection[]>(() => {
  const definition = skillDefinition.value
  if (!definition && !runtimeSkill.value) {
    return []
  }
  const sections: DetailSection[] = []
  sections.push({
    key: "base",
    title: t("game.skillBaseInfo"),
    rows: compactRows([
      row(t("game.level"), String(resolveSkillLevel())),
      row(
        t("game.skillCost"),
        formatCost(
          Number(runtimeSkill.value?.hpCost ?? definition?.hpCost ?? 0),
          Number(runtimeSkill.value?.mpCost ?? definition?.mpCost ?? 0)
        )
      ),
      row(t("game.battleUnitCooldown"), String(totalCooldown.value)),
      row(t("game.skillTargetInfo"), describeTargeting(resolvedTargeting.value)),
      row(
        t("game.skillTargetBaseCount"),
        localizeTargetCountText(resolvedTargeting.value?.targetCount, t),
        resolveTargetCountPreview(resolvedTargeting.value?.targetCount)
      ),
      describeTargetCountMax(resolvedTargeting.value?.targetCount),
      row(t("game.skillTargetMode"), describeTargetScope(resolvedTargeting.value?.targetScope || "enemy")),
      describeTargetPredicates(resolvedTargeting.value),
      describeFallbackSelf(resolvedTargeting.value),
      describeCastGuards(resolvedCastGuards.value)
    ])
  })

  sections.push(
    ...buildEffectDetailSections({
      effects: resolvedEffects.value,
      t,
      unitAttributes: props.unitAttributes,
      skillLevel: resolveSkillLevel()
    })
  )
  if (customForge.value) {
    const metadata = customForge.value
    const source = metadata.sourceSkillId
      ? getLocalizedSkillName(t, metadata.sourceSkillId, metadata.sourceSkillId)
      : t("skillForge.draft.blank")
    sections.push({
      key: "custom-forge",
      title: t("skillForge.public.title"),
      rows: compactRows([
        row(t("skillForge.public.source"), source),
        row(t("skillForge.preview.budget"), `${formatForgeNumber(metadata.budget.used)} / ${formatForgeNumber(metadata.budget.limit)}`),
        row(t("skillForge.preview.refund"), formatForgeNumber(metadata.budget.refund)),
        row(t("skillForge.preview.failChance"), `${(Math.max(0, metadata.budget.failChance) * 100).toFixed(1)}%`)
      ])
    })
  }
  sections.push({
    key: "requirements",
    title: t("game.skillRequirements"),
    rows: compactRows([
      row(t("game.skillStatusRequirementInfo"), describeStatusRequirement(resolvedStatusRequirement.value)),
      row(t("game.skillWeaponRequirementInfo"), describeWeaponRequirement(resolvedWeaponRequirement.value))
    ])
  })
  return sections.filter((section) => section.rows.length > 0)
})

const baselineSkill = computed(() => {
  if (!props.baselineSkill || typeof props.baselineSkill !== "object" || Array.isArray(props.baselineSkill)) return null
  return props.baselineSkill as Record<string, unknown>
})

const renderedDetailSections = computed<DetailSection[]>(() => {
  if (!baselineSkill.value) return detailSections.value
  return compareDetailSections(detailSections.value, buildComparableDetailSections(baselineSkill.value))
})

function buildComparableDetailSections(runtime: Record<string, unknown>): DetailSection[] {
  const definition = skillDefinition.value
  const targeting = resolveObject(runtime.targeting ?? definition?.targeting) as NonNullable<typeof definition>["targeting"] | undefined
  const effects = Array.isArray(runtime.effects)
    ? runtime.effects as NonNullable<typeof definition>["effects"]
    : Array.isArray(definition?.effects) ? definition.effects : []
  const castGuards = Array.isArray(runtime.castGuards)
    ? runtime.castGuards as NonNullable<typeof definition>["castGuards"]
    : definition?.castGuards || []
  const statusRequirement = resolvePositiveNumberRecord(runtime.statusRequirement ?? definition?.statusRequirement)
  const weaponRequirement = resolveStringList(runtime.weaponRequirement ?? definition?.weaponRequirement)
  const cooldown = resolveCooldown(runtime, definition as unknown as Record<string, unknown> | undefined)

  const sections: DetailSection[] = [{
    key: "base",
    title: t("game.skillBaseInfo"),
    rows: compactRows([
      row(t("game.level"), String(resolveSkillLevel())),
      row(t("game.skillCost"), formatCost(
        Number(runtime.hpCost ?? definition?.hpCost ?? 0),
        Number(runtime.mpCost ?? definition?.mpCost ?? 0)
      )),
      row(t("game.battleUnitCooldown"), String(cooldown)),
      row(t("game.skillTargetInfo"), describeTargeting(targeting)),
      row(t("game.skillTargetBaseCount"), localizeTargetCountText(targeting?.targetCount, t), resolveTargetCountPreview(targeting?.targetCount)),
      describeTargetCountMax(targeting?.targetCount),
      row(t("game.skillTargetMode"), describeTargetScope(targeting?.targetScope || "enemy")),
      describeTargetPredicates(targeting),
      describeFallbackSelf(targeting),
      describeCastGuards(castGuards)
    ])
  }]
  sections.push(...buildEffectDetailSections({
    effects,
    t,
    unitAttributes: props.unitAttributes,
    skillLevel: resolveSkillLevel()
  }))
  sections.push({
    key: "requirements",
    title: t("game.skillRequirements"),
    rows: compactRows([
      row(t("game.skillStatusRequirementInfo"), describeStatusRequirement(statusRequirement)),
      row(t("game.skillWeaponRequirementInfo"), describeWeaponRequirement(weaponRequirement))
    ])
  })
  return sections.filter((section) => section.rows.length > 0)
}

function compareDetailSections(current: DetailSection[], original: DetailSection[]): DetailSection[] {
  const originalSections = new Map(original.map((section) => [section.key, section]))
  const currentKeys = new Set(current.map((section) => section.key))
  const compared = current.map((section) => {
    if (section.key === "custom-forge") return section
    const originalSection = originalSections.get(section.key)
    return {
      ...section,
      rows: compareDetailRows(section.rows, originalSection?.rows || [])
    }
  })
  for (const originalSection of original) {
    if (currentKeys.has(originalSection.key)) continue
    compared.push({
      ...originalSection,
      rows: originalSection.rows.map((originalRow) => changedRow(
        { label: originalRow.label, value: t("skillForge.preview.noValue"), labelHint: originalRow.labelHint },
        originalRow
      ))
    })
  }
  return compared
}

function compareDetailRows(current: DetailRow[], original: DetailRow[]): DetailRow[] {
  const originalByLabel = new Map(original.map((row) => [row.label, row]))
  const currentLabels = new Set(current.map((row) => row.label))
  const compared = current.map((currentRow) => {
    const originalRow = originalByLabel.get(currentRow.label)
    if (originalRow && detailRowSignature(currentRow) === detailRowSignature(originalRow)) return currentRow
    return changedRow(currentRow, originalRow || { label: currentRow.label, value: t("skillForge.preview.noValue") })
  })
  for (const originalRow of original) {
    if (currentLabels.has(originalRow.label)) continue
    compared.push(changedRow(
      { label: originalRow.label, value: t("skillForge.preview.noValue"), labelHint: originalRow.labelHint },
      originalRow
    ))
  }
  return compared
}

function changedRow(current: DetailRow, original: DetailRow): DetailRow {
  return {
    ...current,
    changed: true,
    originalValue: original.value || t("skillForge.preview.noValue"),
    originalPreview: original.preview,
    originalBuffIds: original.buffIds ? [...original.buffIds] : undefined
  }
}

function detailRowSignature(row: DetailRow): string {
  return JSON.stringify({ value: row.value || "", preview: row.preview || "", buffIds: row.buffIds || [] })
}

function resolveObject(value: unknown): Record<string, any> | undefined {
  if (!value || typeof value !== "object" || Array.isArray(value)) return undefined
  return value as Record<string, any>
}

function resolvePositiveNumberRecord(value: unknown): Record<string, number> {
  const record = resolveObject(value)
  if (!record) return {}
  return Object.fromEntries(Object.entries(record)
    .map(([key, raw]) => [key, Number(raw)] as const)
    .filter(([, number]) => Number.isFinite(number) && number > 0))
}

function resolveStringList(value: unknown): string[] {
  return Array.isArray(value) ? value.map((entry) => String(entry || "").trim()).filter(Boolean) : []
}

function resolveCooldown(runtime: Record<string, unknown>, definition?: Record<string, unknown>): number {
  const raw = Number(runtime.cooldownTurns ?? runtime.cooldown ?? definition?.cooldown ?? 0)
  return Number.isFinite(raw) ? Math.max(0, Math.trunc(raw)) : 0
}

function describeTargeting(targeting: NonNullable<typeof skillDefinition.value>["targeting"] | undefined): string {
  if (!targeting) return t("game.skillFormulaUnknown")
  const positions = formatTargetPositions(targeting.targetPositions)
  const seek = localizeSecondarySeek(targeting.secondarySeek)
  return [positions, seek].filter(Boolean).join(" · ")
}

function describeTargetScope(scope: string): string {
  return localizeTargetScope(scope)
}

function describeSelectRule(rule: string, attrKey: string): string {
  if (rule === "attr_max" || rule === "attr_min") {
    return `${localizeSelectRule(rule)}: ${localizeAttr(attrKey)}`
  }
  return localizeSelectRule(rule)
}

function describeFallbackSelf(targeting: NonNullable<typeof skillDefinition.value>["targeting"] | undefined): DetailRow | null {
  if (!targeting) return null
  if (!targeting.fallbackSelfIfEmpty) return null
  return row(t("game.skillFallbackSelfIfEmpty"), t("game.yes"))
}

function describeTargetPredicates(targeting: NonNullable<typeof skillDefinition.value>["targeting"] | undefined): DetailRow | null {
  const predicates = Array.isArray(targeting?.targetPredicates) ? targeting.targetPredicates : []
  if (predicates.length === 0) return null
  const text = predicates
    .map((predicate) => describeTargetPredicate(predicate))
    .filter((part): part is string => Boolean(part))
    .join(" · ")
  if (!text) return null
  return row(t("game.skillTargetPredicates"), text)
}

function describeTargetPredicate(predicate: { kind?: string; actorAttr?: string; targetAttr?: string; operator?: string; enemyType?: string; statusId?: string }): string | null {
  const kind = String(predicate.kind || "").trim().toLowerCase()
  if (kind === "compare_attr") {
    const actorAttr = String(predicate.actorAttr || "").trim()
    const targetAttr = String(predicate.targetAttr || "").trim()
    if (!actorAttr || !targetAttr) return null
    return localizeCompareAttrPredicate(actorAttr, targetAttr, predicate.operator)
  }
  if (kind === "exclude_enemy_type") {
    const enemyType = String(predicate.enemyType || "").trim()
    if (!enemyType) return null
    return t("game.skillTargetPredicateExcludeEnemyTypeFriendly", { enemyType: localizeEnemyType(enemyType) })
  }
  if (kind === "require_target_has_status") {
    const statusId = String(predicate.statusId || "").trim()
    if (!statusId) return null
    return t("game.skillTargetPredicateRequireTargetHasStatusFriendly", { status: localizeBuffName(statusId) })
  }
  return null
}

function describeCastGuards(castGuards: NonNullable<typeof skillDefinition.value>["castGuards"] | undefined): DetailRow | null {
  const guards = Array.isArray(castGuards) ? castGuards : []
  if (guards.length === 0) return null
  const descriptions = guards
    .map((guard) => describeCastGuard(guard))
    .filter((text): text is string => Boolean(text))
  if (descriptions.length === 0) return null
  return row(t("game.skillCastGuards"), descriptions.join(" · "))
}

function describeCastGuard(guard: { kind?: string; effectKind?: string }): string | null {
  const kind = String(guard.kind || "").trim().toLowerCase()
  if (kind === "enemy_side_not_has_effect") {
    const effectKind = String(guard.effectKind || "").trim()
    if (!effectKind) return null
    return t("game.skillCastGuardEnemySideNotHasEffect", { effect: localizeBuffEffectKind(effectKind) })
  }
  if (kind === "ally_side_has_non_full_hp") {
    return t("game.skillCastGuardAllySideHasNonFullHp")
  }
  return null
}

function describeTargetCountMax(
  targetCount:
    | { base: number; formula: Array<{ attrKey: string; coefficient: number; perSkillLevel: number }>; max?: number }
    | undefined
): DetailRow | null {
  const targetCountMax = localizeTargetCountMaxText(targetCount, t)
  if (targetCountMax === null) return null
  return row(t("game.skillTargetMaxCount"), targetCountMax, undefined, t("game.skillTargetMaxCountHint"))
}

function describeStatusRequirement(requirement: Record<string, number>): string {
  const segments = Object.entries(requirement)
    .map(([attrKey, required]) => `${localizeAttr(attrKey)} ≥ ${required}`)
    .filter(Boolean)
  return segments.length > 0 ? segments.join(" · ") : t("game.skillFormulaUnknown")
}

function describeWeaponRequirement(requirements: string[]): string {
  const label = describeWeaponClassList(t, requirements)
  return label || t("game.skillFormulaUnknown")
}

function formatTargetPositions(positions: string[]): string {
  if (!positions.length) return t("game.skillFormulaUnknown")
  return positions.map((position) => localizeTargetPosition(position)).join(" / ")
}

function formatFormulaTerms(formula: Array<{ attrKey: string; coefficient: number; perSkillLevel: number }>): string {
  const skillLevel = resolveSkillLevel()
  const levelOffset = resolveSkillLevelOffset(skillLevel)
  const segments = (Array.isArray(formula) ? formula : [])
    .map((term) => {
      const attrKey = String(term?.attrKey || "").trim()
      const coefficient = Number(term?.coefficient)
      const perSkillLevel = Number(term?.perSkillLevel ?? 0)
      if (!attrKey || !Number.isFinite(coefficient) || !Number.isFinite(perSkillLevel)) return null
      return { attrKey, coefficient, perSkillLevel }
    })
    .filter((term): term is { attrKey: string; coefficient: number; perSkillLevel: number } => !!term)
    .map((term) => {
      const baseCoefficient = trimTrailingZeros(term.coefficient)
      const attrLabel = localizeAttr(term.attrKey)
      if (!term.perSkillLevel) {
        return `${baseCoefficient}×${attrLabel}`
      }
      if (levelOffset <= 0) {
        return `${baseCoefficient}×${attrLabel} (+${trimTrailingZeros(term.perSkillLevel)}/Lv)`
      }
      return `(${baseCoefficient} + ${trimTrailingZeros(term.perSkillLevel)+"/Lv"}×${levelOffset})×${attrLabel}`
    })
  return segments.join(" + ")
}

function resolveSkillLevel(): number {
  const rawLevel = Number(props.unitSkillLevel)
  if (!Number.isFinite(rawLevel)) return 1
  return Math.max(1, Math.trunc(rawLevel))
}

function resolveSkillLevelOffset(level: number): number {
  if (!Number.isFinite(level)) return 0
  return Math.max(0, Math.trunc(level) - 1)
}

function resolveTargetCountPreview(targetCount: { base: number; formula: Array<{ attrKey: string; coefficient: number; perSkillLevel: number }> } | undefined): string | undefined {
  if (!targetCount) return undefined
  const preview = evaluateTargetCountPreview(targetCount, props.unitAttributes, resolveSkillLevel())
  if (preview === null) return undefined
  return `(${preview})`
}

function resolveAmountFormulaPreview(formula?: { baseAdd: number; terms: Array<{ attrKey: string; coefficient: number; perSkillLevel: number }> }): string | undefined {
  if (!formula) return undefined
  const preview = evaluateAmountFormulaPreview(formula, props.unitAttributes, resolveSkillLevel())
  if (preview === null) return undefined
  return `(${trimTrailingZeros(preview)})`
}

function formatCost(hpCost: number, mpCost: number): string {
  const parts: string[] = []
  if (hpCost > 0) parts.push(`${t("game.skillCostHp")} ${hpCost}`)
  if (mpCost > 0) parts.push(`${t("game.skillCostMp")} ${mpCost}`)
  return parts.length > 0 ? parts.join(" · ") : t("game.skillNoCost")
}

function formatForgeNumber(value: number): string {
  if (!Number.isFinite(value)) return "0"
  return Number(value.toFixed(2)).toString()
}

function localizeTargetPosition(position: string): string {
  const key = `game.targetDirection.${position}`
  const localized = t(key)
  return localized === key ? position : localized
}

function localizeSecondarySeek(strategy: string): string {
  const key = `game.targetSeekStrategy.${strategy}`
  const localized = t(key)
  return localized === key ? strategy : localized
}

function localizeTargetScope(scope: string): string {
  const key = `game.skillTargetScope.${scope}`
  const localized = t(key)
  return localized === key ? scope : localized
}

function localizeSelectRule(rule: string): string {
  const key = `game.skillSelectRule.${rule}`
  const localized = t(key)
  return localized === key ? rule : localized
}

function localizeBuffEffectKind(effectKind: string): string {
  const key = `game.buffEffectKind_${effectKind}`
  const localized = t(key)
  return localized === key ? effectKind : localized
}

function localizeAttr(attrKey: string): string {
  const key = `attr.${attrKey}.name`
  const localized = t(key)
  return localized === key ? attrKey : localized
}

function localizeBuffName(buffId: string): string {
  const key = `buff.${buffId}.name`
  const localized = t(key)
  return localized === key ? buffId : localized
}

function localizeCompareOperator(raw: unknown): string {
  const value = String(raw || ">").trim()
  if (value === "gt") return ">"
  if (value === "gte") return ">="
  if (value === "lt") return "<"
  if (value === "lte") return "<="
  if (value === "eq" || value === "=") return "=="
  if (value === "ne") return "!="
  return value || ">"
}

function localizeCompareAttrPredicate(actorAttrKey: string, targetAttrKey: string, rawOperator: unknown): string {
  const actor = localizeAttr(actorAttrKey)
  const target = localizeAttr(targetAttrKey)
  const op = localizeCompareOperator(rawOperator)
  const key = op === ">"
    ? "game.skillTargetPredicateCompareAttrGt"
    : op === ">="
      ? "game.skillTargetPredicateCompareAttrGte"
      : op === "<"
        ? "game.skillTargetPredicateCompareAttrLt"
        : op === "<="
          ? "game.skillTargetPredicateCompareAttrLte"
          : op === "=="
            ? "game.skillTargetPredicateCompareAttrEq"
            : "game.skillTargetPredicateCompareAttrNe"
  return t(key, { actor, target })
}

function localizeEnemyType(raw: string): string {
  const enemyType = raw.trim().toLowerCase()
  if (enemyType === "boss") return t("game.enemyTypeBoss")
  if (enemyType === "elite") return t("game.enemyTypeElite")
  if (enemyType === "normal") return t("game.enemyTypeNormal")
  return raw
}

function compactRows(rows: Array<DetailRow | null | undefined>): DetailRow[] {
  return rows.filter((row): row is DetailRow => {
    if (!row) return false
    if (Array.isArray(row.buffIds) && row.buffIds.length > 0) return true
    return String(row.value || "").trim() !== ""
  })
}

function row(label: string, value: string, preview?: string, labelHint?: string): DetailRow {
  return { label, value, preview, labelHint }
}

function rowBuffs(label: string, buffIds: string[]): DetailRow {
  return { label, value: "", buffIds }
}

function trimTrailingZeros(value: number): string {
  const normalized = Number(value)
  if (!Number.isFinite(normalized)) return "0"
  return normalized.toFixed(4).replace(/\.?0+$/, "")
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
@reference "#app-style";

.skill-quick-ref-content {
  display: grid;
  font-size: 12px;
}

.skill-quick-ref-header {
  display: grid;
  gap: 2px;
}

.skill-quick-ref-title {
  font-size: 16px;
  font-weight: 600;
}

.skill-quick-ref-desc {
  color: var(--el-text-color-secondary);
  display: inline-flex;
  align-items: center;
  gap: 4px;
}

.skill-quick-ref-score {
  color: var(--el-color-success);
  font-weight: 600;
}

.skill-quick-ref-score-breakdown {
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
  color: var(--el-text-color-secondary);
}

.skill-quick-ref-score-breakdown span {
  border: 1px solid var(--el-border-color-lighter);
  border-radius: 999px;
  padding: 1px 6px;
}

.skill-quick-ref-section { display: grid; }

.skill-detail-row {
  min-width: 0;
  padding: 2px 4px;
}

.skill-detail-row.is-changed {
  align-items: flex-start;
  padding: 7px 8px;
  border-left: 3px solid var(--el-color-warning);
  border-radius: 4px;
  background: color-mix(in srgb, var(--el-color-warning) 9%, transparent);
}

.skill-detail-row.is-changed .detail-row-label {
  padding-top: 2px;
  color: color-mix(in srgb, var(--el-color-warning) 72%, var(--el-text-color-primary));
  font-weight: 600;
}

.skill-detail-row.is-changed .detail-row-value {
  min-width: min(220px, 100%);
  display: grid;
  flex: 1;
  gap: 5px;
}

.skill-detail-row__original,
.skill-detail-row__current {
  min-width: 0;
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 5px;
  overflow-wrap: anywhere;
}

.skill-detail-row__original {
  color: var(--el-text-color-secondary);
  font-size: 11px;
}

.skill-detail-row__current {
  color: var(--el-text-color-primary);
  font-weight: 600;
}

.skill-detail-row__original-label,
.skill-detail-row__current-label {
  flex: 0 0 auto;
  padding: 1px 4px;
  border-radius: 3px;
  font-size: 10px;
  line-height: 1.4;
}

.skill-detail-row__original-label {
  color: var(--el-text-color-secondary);
  background: color-mix(in srgb, var(--el-text-color-secondary) 10%, transparent);
}

.skill-detail-row__current-label {
  color: color-mix(in srgb, var(--el-color-warning) 72%, var(--el-text-color-primary));
  background: color-mix(in srgb, var(--el-color-warning) 15%, transparent);
}
</style>
