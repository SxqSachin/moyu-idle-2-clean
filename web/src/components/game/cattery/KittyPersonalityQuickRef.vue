<template>
  <el-popover
    v-model:visible="popoverVisible"
    placement="top"
    :width="360"
    trigger="hover"
    :persistent="false"
    :show-after="180"
  >
    <template #reference>
      <el-tag
        :disable-transitions="true"
        :class="{ 'is-loading': loading }"
      >
        {{ personalityName }}
      </el-tag>
    </template>
    <div class="space-y-2 text-xs leading-5">
      <div class="font-semibold">{{ personalityName }}</div>
      <div class="app-muted">{{ personalityDesc }}</div>
      <div class="space-y-2">
        <template v-for="block in effectBlocks" :key="block.key">
          <div v-if="block.kind === 'empty'" class="app-muted">
            {{ t("game.kittyPersonalityQuickRefNoEffect") }}
          </div>

          <div v-else-if="block.kind === 'attr'" class="space-y-1">
            <div class="font-medium">{{ t("game.kittyPersonalityQuickRefAttr") }}</div>
            <div
              v-for="entry in block.entries"
              :key="`${block.key}-${entry.attrKey}`"
              class="pl-3"
            >
              {{ localizeAttr(entry.attrKey) }} {{ formatSignedNumber(entry.value) }}
            </div>
          </div>

          <div v-else-if="block.kind === 'position'" class="space-y-1">
            <div class="font-medium">{{ t("game.kittyPersonalityQuickRefBattlePosition") }}</div>
            <div class="pl-3">{{ localizePosition(block.position) }}</div>
          </div>

          <div v-else-if="block.kind === 'hook'" class="space-y-2">
            <div class="font-medium">{{ t("game.itemDetailBattleHooks") }}</div>
            <div
              v-for="hook in block.hooks"
              :key="`${block.key}-${hook.key}`"
              class="rounded border border-[var(--el-border-color-lighter)] px-2 py-1"
            >
              <div class="font-medium">{{ hook.triggerText }}</div>
              <div v-if="hook.conditions.length > 0" class="mt-1 space-y-1">
                <div class="app-muted">{{ t("game.itemDetailBattleHookIf") }}</div>
                <div
                  v-for="condition in hook.conditions"
                  :key="`${block.key}-${hook.key}-${condition.key}`"
                  class="pl-3"
                >
                  {{ condition.text }}
                </div>
              </div>
              <div class="mt-1 space-y-1">
                <div v-if="hook.conditions.length > 0" class="app-muted">{{ t("game.itemDetailBattleHookThen") }}</div>
                <div
                  v-for="effect in hook.effects"
                  :key="`${block.key}-${hook.key}-${effect.key}`"
                  class="pl-3"
                >
                  <template v-if="effect.kind === 'add_buff' || effect.kind === 'remove_buff'">
                    <span>{{ t(effect.kind === 'add_buff' ? "game.itemDetailBattleHookEffectAddBuffPrefix" : "game.itemDetailBattleHookEffectRemoveBuffPrefix", { target: effect.targetText }) }}</span>
                    <BuffQuickRef :buff-id="effect.buffId" />
                    <span v-if="effect.kind === 'add_buff' && effect.turns !== null">
                      {{ t("game.itemDetailBattleHookEffectTurnsSuffix", { turns: effect.turns }) }}
                    </span>
                    <span v-if="effect.kind === 'add_buff' && effect.procChanceText">
                      {{ effect.procChanceText }}
                    </span>
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
                  </template>
                  <template v-else-if="effect.kind === 'mirror_cast_skill'">
                    <span>{{ effect.descriptionText }}</span>
                    <div v-if="effect.filterText" class="app-muted">{{ effect.filterText }}</div>
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
              </div>
            </div>
          </div>

          <div v-else-if="block.kind === 'mutation'" class="space-y-2">
            <div
              v-for="entry in block.entries"
              :key="`${block.key}-${entry.key}`"
              class="rounded border border-[var(--el-border-color-lighter)] px-2 py-1"
            >
              <div class="font-medium">
                {{ entry.scopeLabel }}
                <template v-if="entry.skillIds.length > 0">
                  ·
                  <span v-for="(skillId, skillIndex) in entry.skillIds" :key="`${entry.key}-skill-${skillId}`">
                    <SkillQuickRef :skill-id="skillId" />
                    <span v-if="skillIndex < entry.skillIds.length - 1"> / </span>
                  </span>
                </template>
              </div>
              <div class="mt-1 space-y-1">
                <div v-if="entry.selectorText" class="pl-3">
                  <span class="app-muted">{{ t("game.itemDetailAffixMutationFieldEffect") }}:</span>
                  <span class="ml-1">{{ entry.selectorText }}</span>
                </div>
                <div v-if="entry.targetCountDelta !== null" class="pl-3">
                  <span class="app-muted">{{ t("game.itemDetailAffixMutationFieldTargetCountDelta") }}:</span>
                  <span class="ml-1">{{ formatSignedNumber(entry.targetCountDelta) }}</span>
                </div>
                <div v-if="entry.amountBaseAddDelta !== null" class="pl-3">
                  <span class="app-muted">{{ t("game.itemDetailAffixMutationFieldAmountBaseAddDelta") }}:</span>
                  <span class="ml-1">{{ formatSignedNumber(entry.amountBaseAddDelta) }}</span>
                </div>
                <div v-if="entry.amountTermsText" class="pl-3">
                  <span class="app-muted">{{ t("game.itemDetailAffixMutationFieldAmountTermsAppend") }}:</span>
                  <span class="ml-1">{{ entry.amountTermsText }}</span>
                </div>
                <div v-if="entry.statusAppend.length > 0" class="pl-3">
                  <span class="app-muted">{{ t("game.itemDetailAffixMutationFieldStatusAppend") }}:</span>
                  <span v-for="(buffId, buffIndex) in entry.statusAppend" :key="`${entry.key}-add-${buffId}`">
                    <span v-if="buffIndex === 0" class="ml-1" />
                    <BuffQuickRef :buff-id="buffId" />
                    <span v-if="buffIndex < entry.statusAppend.length - 1"> / </span>
                  </span>
                </div>
                <div v-if="entry.statusRemove.length > 0" class="pl-3">
                  <span class="app-muted">{{ t("game.itemDetailAffixMutationFieldStatusRemove") }}:</span>
                  <span v-for="(buffId, buffIndex) in entry.statusRemove" :key="`${entry.key}-remove-${buffId}`">
                    <span v-if="buffIndex === 0" class="ml-1" />
                    <BuffQuickRef :buff-id="buffId" />
                    <span v-if="buffIndex < entry.statusRemove.length - 1"> / </span>
                  </span>
                </div>
                <div
                  v-for="appendStatus in entry.appendStatuses"
                  :key="`${entry.key}-${appendStatus.key}`"
                  class="pl-3"
                >
                  <span class="app-muted">{{ t("game.itemDetailAffixMutationFieldAppendEffects") }}:</span>
                  <span class="ml-1">{{ appendStatus.targetText }}</span>
                  <span v-for="(buffId, buffIndex) in appendStatus.statusIds" :key="`${appendStatus.key}-${buffId}`">
                    <span v-if="buffIndex === 0" class="ml-1" />
                    <BuffQuickRef :buff-id="buffId" />
                    <span v-if="buffIndex < appendStatus.statusIds.length - 1"> / </span>
                  </span>
                  <span v-if="appendStatus.turns !== null">
                    {{ t("game.itemDetailBattleHookEffectTurnsSuffix", { turns: formatNumber(appendStatus.turns) }) }}
                  </span>
                </div>
                <div v-if="entry.minTargetCount !== null" class="pl-3">
                  <span class="app-muted">{{ t("game.itemDetailAffixMutationFieldTargetCountMin") }}:</span>
                  <span class="ml-1">{{ formatNumber(entry.minTargetCount) }}</span>
                </div>
                <div v-if="entry.maxTargetCount !== null" class="pl-3">
                  <span class="app-muted">{{ t("game.itemDetailAffixMutationFieldTargetCountMax") }}:</span>
                  <span class="ml-1">{{ formatNumber(entry.maxTargetCount) }}</span>
                </div>
              </div>
            </div>
          </div>
        </template>
      </div>
    </div>
  </el-popover>
</template>

<script setup lang="ts">
import { computed, ref } from "vue"
import { useI18n } from "vue-i18n"
import { KITTY_PERSONALITIES } from "@/config/shared-global-config"
import { buildBattleHookDescriptions } from "@/services/battle-hook-display"
import BuffEffectQuickRef from "@components/game/buff/BuffEffectQuickRef.vue"
import BuffQuickRef from "@components/game/buff/BuffQuickRef.vue"
import SkillQuickRef from "@components/game/skill/SkillQuickRef.vue"
import EnemyUnitQuickRef from "@components/game/combat/EnemyUnitQuickRef.vue"

type HookDescription = ReturnType<typeof buildBattleHookDescriptions>[number]
type AppendStatusSummary = {
  key: string
  targetText: string
  statusIds: string[]
  turns: number | null
}
type MutationSummary = {
  key: string
  scopeLabel: string
  skillIds: string[]
  selectorText: string
  targetCountDelta: number | null
  amountBaseAddDelta: number | null
  amountTermsText: string
  statusAppend: string[]
  statusRemove: string[]
  appendStatuses: AppendStatusSummary[]
  minTargetCount: number | null
  maxTargetCount: number | null
}
type PersonalityEffectBlock =
  | { key: string; kind: "empty" }
  | { key: string; kind: "attr"; entries: Array<{ attrKey: string; value: number }> }
  | { key: string; kind: "position"; position: string }
  | { key: string; kind: "hook"; hooks: HookDescription[] }
  | { key: string; kind: "mutation"; entries: MutationSummary[] }

const props = withDefaults(
  defineProps<{
    personalityId: string
    loading?: boolean
  }>(),
  {
    loading: false
  }
)

const { t } = useI18n()
const popoverVisible = ref(false)

const normalizedPersonalityID = computed(() => String(props.personalityId || "").trim().toLowerCase())
const profile = computed(() => KITTY_PERSONALITIES.find((row) => row.id === normalizedPersonalityID.value))
const personalityName = computed(() => {
  const id = normalizedPersonalityID.value
  const key = profile.value?.nameKey || `kitty.personality.${id}.name`
  const localized = t(key)
  return localized === key ? id : localized
})
const personalityDesc = computed(() => {
  const id = normalizedPersonalityID.value
  const key = profile.value?.descKey || `kitty.personality.${id}.desc`
  const localized = t(key)
  return localized === key ? "" : localized
})
const effectBlocks = computed<PersonalityEffectBlock[]>(() => {
  const out: PersonalityEffectBlock[] = []
  for (const effect of profile.value?.effects || []) {
    if (effect.kind === "attr_modifier") {
      const entries = Object.entries(effect.attrs || {})
        .filter(([, value]) => Number.isFinite(Number(value)) && Number(value) !== 0)
        .sort(([left], [right]) => left.localeCompare(right))
      if (entries.length > 0) {
        out.push({
          key: `attr-${out.length + 1}`,
          kind: "attr",
          entries: entries.map(([attrKey, value]) => ({ attrKey, value: Number(value) }))
        })
      }
      continue
    }
    if (effect.kind === "battle_position_lock") {
      out.push({
        key: `position-${out.length + 1}`,
        kind: "position",
        position: effect.position
      })
      continue
    }
    if (effect.kind === "battle_hook") {
      const allHooks = buildBattleHookDescriptions(effect.battleHooks || [], t)
      const hooks = allHooks.filter((n) => !n.hidden)
      if (hooks.length > 0) {
        out.push({
          key: `hook-${out.length + 1}`,
          kind: "hook",
          hooks
        })
      }
      continue
    }
    if (effect.kind === "effect_mutation") {
      const entries = mutationSummaryEntries(effect.effectMutations || [])
      if (entries.length > 0) {
        out.push({
          key: `mutation-${out.length + 1}`,
          kind: "mutation",
          entries
        })
      }
    }
  }
  if (out.length === 0) {
    out.push({ key: "empty", kind: "empty" })
  }
  return out
})

function localizeAttr(id: string): string {
  const key = `attr.${id}.name`
  const localized = t(key)
  return localized === key ? id : localized
}

function formatHookEffectFallback(effect: { text?: string; formulaText?: string }): string {
  return String(effect.text || effect.formulaText || "")
}

function localizePosition(position: string): string {
  const normalized = String(position || "").trim().toLowerCase()
  if (normalized === "midguard") return t("game.battlePositionMidguard")
  if (normalized === "rearguard") return t("game.battlePositionRearguard")
  return t("game.battlePositionVanguard")
}

function formatNumber(value: number): string {
  const num = Number(value || 0)
  if (!Number.isFinite(num)) return "0"
  if (Math.abs(num - Math.round(num)) < 1e-6) return String(Math.round(num))
  return num.toFixed(2).replace(/\.?0+$/, "")
}

function formatSignedNumber(value: number): string {
  const normalized = formatNumber(value)
  return value > 0 ? `+${normalized}` : normalized
}

function mutationSummaryEntries(rows: Record<string, unknown>[]): MutationSummary[] {
  const out: MutationSummary[] = []
  for (const row of rows) {
    if (!row || typeof row !== "object" || Array.isArray(row)) continue
    const scope = String(row.scope || "both").trim().toLowerCase()
    const selector = row.effectSelector && typeof row.effectSelector === "object" && !Array.isArray(row.effectSelector)
      ? row.effectSelector as Record<string, unknown>
      : {}
    const ops = row.ops && typeof row.ops === "object" && !Array.isArray(row.ops)
      ? row.ops as Record<string, unknown>
      : {}
    const targetCountDelta = toFiniteNumber(ops.targetCountDelta)
    const amountBaseAddDelta = toFiniteNumber(ops.amountBaseAddDelta)
    const amountTermsAppend = Array.isArray(ops.amountTermsAppend) ? ops.amountTermsAppend : []
    let amountTermsText = ""
    if (amountTermsAppend.length > 0) {
      amountTermsText = amountTermsAppend
        .map((term) => {
          if (!term || typeof term !== "object" || Array.isArray(term)) return ""
          const row = term as Record<string, unknown>
          const attrKey = String(row.attrKey || "").trim()
          const coefficient = toFiniteNumber(row.coefficient)
          const perLevel = toFiniteNumber(row.perSkillLevel)
          if (!attrKey || coefficient === null) return ""
          const attr = localizeAttr(attrKey)
          if (perLevel !== null && perLevel !== 0) return `${formatSignedNumber(coefficient)}×${attr} (${formatSignedNumber(perLevel)}/Lv)`
          return `${formatSignedNumber(coefficient)}×${attr}`
        })
        .filter(Boolean)
        .join(" + ")
    }
    const statusAppend = normalizeStringArray(ops.statusIdsAppend)
    const statusRemove = normalizeStringArray(ops.statusIdsRemove)
    const appendEffects = Array.isArray(ops.appendEffects) ? ops.appendEffects : []
    const appendStatuses = appendEffects
      .map((effect, index) => normalizeAppendStatusEffect(effect, index))
      .filter((entry): entry is AppendStatusSummary => !!entry)
    const min = row.constraints && typeof row.constraints === "object" && !Array.isArray(row.constraints)
      ? toFiniteNumber((row.constraints as Record<string, unknown>).targetCountMin)
      : null
    const max = row.constraints && typeof row.constraints === "object" && !Array.isArray(row.constraints)
      ? toFiniteNumber((row.constraints as Record<string, unknown>).targetCountMax)
      : null
    const normalizedTargetCountDelta = targetCountDelta !== null && targetCountDelta !== 0 ? targetCountDelta : null
    const normalizedAmountBaseAddDelta = amountBaseAddDelta !== null && amountBaseAddDelta !== 0 ? amountBaseAddDelta : null
    if (
      normalizedTargetCountDelta === null &&
      normalizedAmountBaseAddDelta === null &&
      !amountTermsText &&
      statusAppend.length === 0 &&
      statusRemove.length === 0 &&
      appendStatuses.length === 0 &&
      min === null &&
      max === null
    ) continue
    out.push({
      key: `mutation-${out.length + 1}`,
      scopeLabel: localizeMutationScope(scope),
      skillIds: normalizeStringArray(row.skillIds),
      selectorText: localizeMutationSelector(selector),
      targetCountDelta: normalizedTargetCountDelta,
      amountBaseAddDelta: normalizedAmountBaseAddDelta,
      amountTermsText,
      statusAppend,
      statusRemove,
      appendStatuses,
      minTargetCount: min,
      maxTargetCount: max
    })
  }
  return out
}

function localizeMutationSelector(selector: Record<string, unknown>): string {
  const mode = String(selector.mode || "").trim().toLowerCase()
  const indexes = normalizeIntegerArray(selector.indexes)
  if (mode === "index" && indexes.length > 0) {
    return indexes.map((index) => t("game.effectRefIndex", { index })).join(" / ")
  }
  if (mode === "all") return t("game.itemDetailAffixMutationSelectorModeValue.all")
  return ""
}

function localizeMutationScope(scope: string): string {
  if (scope === "basic_attack") return t("game.kittyPersonalityQuickRefMutationScopeBasicAttack")
  if (scope === "skill") return t("game.kittyPersonalityQuickRefMutationScopeSkill")
  return t("game.kittyPersonalityQuickRefMutationScopeBoth")
}

function normalizeAppendStatusEffect(effect: unknown, index: number): AppendStatusSummary | null {
  if (!effect || typeof effect !== "object" || Array.isArray(effect)) return null
  const row = effect as Record<string, unknown>
  const kind = String(row.kind || "").trim().toLowerCase()
  if (kind !== "status") return null
  const statusPayload = row.statusPayload && typeof row.statusPayload === "object" && !Array.isArray(row.statusPayload)
    ? row.statusPayload as Record<string, unknown>
    : {}
  const statusIds = normalizeStringArray(statusPayload.statusIds)
  const turns = toFiniteNumber(statusPayload.durationBaseTurns)
  const targeting = row.targeting && typeof row.targeting === "object" && !Array.isArray(row.targeting)
    ? row.targeting as Record<string, unknown>
    : {}
  const positions = normalizeStringArray(targeting.targetPositions).map(localizePositionToken)
  const target = positions.length > 0 ? positions.join("/") : t("game.kittyPersonalityQuickRefMutationEnemyTarget")
  return {
    key: `append-status-${index + 1}`,
    targetText: target,
    statusIds,
    turns: turns !== null && turns > 0 ? turns : null
  }
}

function localizePositionToken(position: string): string {
  const normalized = String(position || "").trim().toLowerCase()
  if (normalized === "midguard") return t("game.battlePositionMidguard")
  if (normalized === "rearguard") return t("game.battlePositionRearguard")
  if (normalized === "vanguard") return t("game.battlePositionVanguard")
  return position
}

function normalizeStringArray(raw: unknown): string[] {
  if (!Array.isArray(raw)) return []
  return raw.map((value) => String(value || "").trim()).filter(Boolean)
}

function normalizeIntegerArray(raw: unknown): number[] {
  if (!Array.isArray(raw)) return []
  return raw
    .map((value) => toFiniteNumber(value))
    .filter((value): value is number => value !== null)
    .map((value) => Math.trunc(value))
    .filter((value) => value > 0)
}

function toFiniteNumber(raw: unknown): number | null {
  const value = Number(raw)
  return Number.isFinite(value) ? value : null
}
</script>
