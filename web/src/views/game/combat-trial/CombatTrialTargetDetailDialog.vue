<template>
  <GameDialog
    :model-value="modelValue"
    :title="targetName"
    width="680px"
    destroy-on-close
    @update:model-value="emit('update:modelValue', $event)"
  >
    <EnemyUnitQuickRefDetailContent
      v-if="definition && unit"
      :enemy-id="unit.enemyId || definition.id"
      :definition-override="definition"
      :name-override="targetName"
      :description-override="targetDescription"
      :show-level="unit.kind === 'authored_enemy'"
      :show-experience="false"
      :show-difficulty="false"
      :show-drops="false"
      :fallback-basic-attack="unit.behavior !== 'passive'"
      :project-derived-stats="unit.kind === 'authored_enemy' && !hasResolvedStats"
      :include-configured-stats="unit.kind === 'authored_enemy'"
      :stat-value-overrides="statValueOverrides"
      :empty-attack-mode-label="unit.behavior === 'passive' ? t('game.combatTrial.passive') : ''"
    />
  </GameDialog>
</template>

<script setup lang="ts">
import { computed } from "vue"
import { useI18n } from "vue-i18n"
import GameDialog from "@components/game/common/GameDialog.vue"
import EnemyUnitQuickRefDetailContent from "@components/game/combat/EnemyUnitQuickRefDetailContent.vue"
import { getEnemyDefinition, type EnemyDefinition } from "@services/enemy-definition"
import type { CombatTrialEncounterUnit, CombatTrialFormat } from "@services/combat-trial"

const props = defineProps<{
  modelValue: boolean
  format: CombatTrialFormat | null
  unit: CombatTrialEncounterUnit | null
}>()

const emit = defineEmits<{
  (event: "update:modelValue", value: boolean): void
}>()

const { t, locale } = useI18n()

const targetName = computed(() => {
  const unit = props.unit
  if (!unit) return t("game.combatTrial.opponents")
  return localized(unit.nameKey, unit.enemyId || unit.id)
})

const targetDescription = computed(() => {
  const unit = props.unit
  const format = props.format
  if (!unit || !format) return ""
  return [
    localized(format.nameKey, format.id),
    targetDifficultyLabel.value,
    unit.kind === "scripted_ally" ? t("game.combatTrial.scriptedAlly") : unit.behavior === "passive" ? t("game.combatTrial.passive") : t("game.combatTrial.active"),
    unit.healthMode === "immutable_full" ? t("game.combatTrial.immutable") : t("game.combatTrial.normalHealth")
  ].filter(Boolean).join(" · ")
})

const targetDifficultyLabel = computed(() => {
  const unitID = String(props.unit?.id || "").trim()
  if (!unitID) return ""
  const format = props.format
  const wave = format?.encounter?.waves?.find((entry) => entry.units.some((unit) => unit.id === unitID))
  const level = Math.max(0, Math.trunc(Number(wave?.enemyStatProfile?.difficultyLevel ?? format?.encounter?.enemyStatProfile?.difficultyLevel ?? 0)))
  return level > 0 ? `N${level}` : ""
})

const hasResolvedStats = computed(() => Object.keys(props.unit?.resolvedStats || {}).length > 0)

const definition = computed<EnemyDefinition | null>(() => {
  const unit = props.unit
  if (!unit) return null
  const authored = unit.enemyId ? getEnemyDefinition(unit.enemyId) : undefined
  if (authored) {
    return {
      ...authored,
      position: unit.position,
      stats: hasResolvedStats.value ? { ...(unit.resolvedStats || {}) } : { ...authored.stats },
      drops: [],
      skillIds: [...authored.skillIds],
      initialBuffIds: [...authored.initialBuffIds],
      equipment: authored.equipment.map((entry) => ({
        ...entry,
        baseSnapshot: entry.baseSnapshot ? { ...entry.baseSnapshot } : undefined,
        affixes: entry.affixes.map((affix) => ({ ...affix }))
      }))
    }
  }
  const defense = Math.max(0, Number(unit.defense ?? props.format?.defenseK ?? props.format?.dummy?.defense ?? 0))
  const maxHp = Math.max(1, Number(unit.maxHp || 1))
  const damageResist = Math.max(0, Number(unit.damageResist || 0))
  const criticalDamageResist = Math.max(0, Number(unit.criticalDamageResist || 0))
  const stats: Record<string, number> = {
    hp: maxHp,
    speed: unit.behavior === "passive" ? 1 : 0
  }
  if (defense > 0) {
    stats.physics_defence = defense
    stats.magical_defence = defense
  }
  if (damageResist > 0) {
    stats.physics_damage_resist = damageResist
    stats.magical_damage_resist = damageResist
  }
  if (criticalDamageResist > 0) stats.crit_damage_resist = criticalDamageResist
  return {
    id: unit.id,
    name: targetName.value,
    nameKey: String(unit.nameKey || ""),
    descKey: "",
    position: unit.position,
    level: 1,
    exp: 0,
    stats,
    drops: [],
    skillIds: [],
    initialBuffIds: [],
    equipment: []
  }
})

const statValueOverrides = computed<Record<string, string>>(() => {
  const unit = props.unit
  if (unit?.healthMode === "immutable_full") return { hp: "∞" }
  if (unit?.kind !== "scripted_ally") return {}
  const maxHp = Math.max(1, Number(unit.maxHp || 1))
  const currentHp = Math.max(1, maxHp * Math.max(0, Math.min(1, Number(unit.initialHpRatio || 1))))
  const formatter = new Intl.NumberFormat(locale.value, { maximumFractionDigits: 0 })
  return { hp: `${formatter.format(currentHp)} / ${formatter.format(maxHp)}` }
})

function localized(key: string | undefined, fallback: string): string {
  const normalized = String(key || "").trim()
  if (!normalized) return fallback
  const translated = t(normalized)
  return translated && translated !== normalized ? translated : fallback
}
</script>
