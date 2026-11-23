<template>
  <div class="operation-effect-list" :class="{ 'operation-effect-list--compact': compact }" role="list">
    <article
      v-for="effect in items"
      :key="effect.grantId"
      class="operation-effect-list__item"
      role="listitem"
    >
      <div class="operation-effect-list__icon" aria-hidden="true">
        <GameIcon
          v-if="effect.iconId || effect.sourceItemId"
          :icon-id="effect.iconId || `item.${effect.sourceItemId}`"
          :size="34"
          fallback="✦"
        />
        <Icon v-else icon="lucide:sparkles" />
      </div>

      <div class="operation-effect-list__content">
        <div class="operation-effect-list__heading">
          <strong>{{ localizedText(effect.nameKey, effect.definitionId) }}</strong>
          <span class="operation-effect-list__remaining">
            <Icon :icon="effect.lifetimeKind === 'duration' ? 'lucide:timer' : 'lucide:circle-dot-dashed'" />
            {{ remainingText(effect) }}
          </span>
        </div>

        <div v-if="modifierLines(effect).length" class="operation-effect-list__modifiers">
          <span v-for="line in modifierLines(effect)" :key="line">
            <Icon :icon="domainIcon(effect.domain)" />
            {{ line }}
          </span>
        </div>

        <div v-if="!compact" class="operation-effect-list__meta">
          <span>
            <Icon :icon="domainIcon(effect.domain)" />
            {{ t(`game.operationEffects.domain.${effect.domain}`) }}
          </span>
          <span>
            <Icon :icon="targetIcon(effect.targetKind)" />
            {{ targetText(effect) }}
          </span>
        </div>

        <div
          v-if="effect.lifetimeKind === 'duration'"
          class="operation-effect-list__progress"
          role="progressbar"
          :aria-label="remainingText(effect)"
          :aria-valuenow="durationPercent(effect)"
          aria-valuemin="0"
          aria-valuemax="100"
        >
          <span :style="{ width: `${durationPercent(effect)}%` }" />
        </div>
      </div>
    </article>
  </div>
</template>

<script setup lang="ts">
import { Icon } from "@iconify/vue"
import { useI18n } from "vue-i18n"
import GameIcon from "@components/game/icon/Icon.vue"
import { operationEffectModifierLines } from "@services/operation-effect-display"
import { usePlayerStore } from "@stores/player"
import { formatDurationI18n } from "@/utils/duration-format"
import type {
  OperationEffectDomain,
  OperationEffectTargetKind,
  OperationEffectView
} from "@services/operation-effects"

const props = defineProps<{
  items: OperationEffectView[]
  nowMs: number
  compact?: boolean
}>()

const { t, te } = useI18n()
const playerStore = usePlayerStore()

function localizedText(key: string | undefined, fallback: string): string {
  const normalized = String(key || "").trim()
  return normalized && te(normalized) ? t(normalized) : fallback
}

function timestamp(value: unknown): number {
  const parsed = Date.parse(String(value || ""))
  return Number.isFinite(parsed) ? parsed : 0
}

function remainingText(effect: OperationEffectView): string {
  if (effect.lifetimeKind === "charges") {
    return t("game.operationEffects.remainingCharges", {
      count: Math.max(0, Math.trunc(Number(effect.remainingCharges || 0)))
    })
  }
  const remainingSeconds = Math.max(0, Math.ceil((timestamp(effect.validUntil) - props.nowMs) / 1000))
  return t("game.operationEffects.remainingTime", {
    time: formatDurationI18n(remainingSeconds, t)
  })
}

function durationPercent(effect: OperationEffectView): number {
  const from = timestamp(effect.validFrom)
  const until = timestamp(effect.validUntil)
  if (!from || until <= from) return 0
  const percent = ((until - props.nowMs) / (until - from)) * 100
  return Math.max(0, Math.min(100, Math.round(percent)))
}

function modifierLines(effect: OperationEffectView): string[] {
  return operationEffectModifierLines(effect.kind, effect.params || {}, t, false)
}

function targetText(effect: OperationEffectView): string {
  switch (effect.targetKind) {
    case "player":
      return t("game.operationEffects.targetPlayer")
    case "kitty": {
      const kitty = playerStore.kitties.find((candidate) => Number(candidate.id || 0) === Number(effect.targetActorId || 0))
      const name = String(kitty?.name || "").trim() || `#${effect.targetActorId || 0}`
      return t("game.operationEffects.targetKitty", { name })
    }
    case "all_kitties":
      return t("game.operationEffects.targetAllKitties")
    case "player_and_kitties":
      return t("game.operationEffects.targetPlayerAndKitties")
    default:
      return String(effect.targetKind || "")
  }
}

function targetIcon(kind: OperationEffectTargetKind): string {
  if (kind === "kitty") return "lucide:cat"
  if (kind === "all_kitties" || kind === "player_and_kitties") return "lucide:users"
  return "lucide:user"
}

function domainIcon(domain: OperationEffectDomain): string {
  switch (domain) {
    case "life_action":
      return "lucide:hammer"
    case "enhancement":
      return "lucide:anvil"
    case "enchantment":
      return "lucide:wand-sparkles"
    case "alchemy":
      return "lucide:flask-conical"
    case "fishing_cast":
      return "lucide:fish-symbol"
    case "building":
      return "lucide:house"
    default:
      return "lucide:sparkles"
  }
}
</script>

<style scoped>
.operation-effect-list {
  max-height: min(430px, 58vh);
  overflow-y: auto;
  overscroll-behavior: contain;
}

.operation-effect-list--compact { max-height: none; overflow: visible; }
.operation-effect-list--compact .operation-effect-list__item { grid-template-columns: 36px minmax(0, 1fr); gap: 9px; padding: 10px 0; }
.operation-effect-list--compact .operation-effect-list__icon { width: 36px; height: 36px; border: 0; background: none; }
.operation-effect-list--compact .operation-effect-list__heading { flex-wrap: wrap; gap: 4px 10px; font-size: 13px; }
.operation-effect-list--compact .operation-effect-list__modifiers { margin: 5px 0 0; font-size: 12px; }

.operation-effect-list__item {
  display: grid;
  grid-template-columns: 44px minmax(0, 1fr);
  gap: 11px;
  padding: 13px 2px;
  border-bottom: 1px solid var(--app-border);
}

.operation-effect-list__item:first-child {
  padding-top: 4px;
}

.operation-effect-list__item:last-child {
  padding-bottom: 4px;
  border-bottom: 0;
}

.operation-effect-list__icon {
  display: inline-flex;
  width: 44px;
  height: 44px;
  align-items: center;
  justify-content: center;
  color: var(--app-primary);
  font-size: 21px;
}

.operation-effect-list__content {
  min-width: 0;
}

.operation-effect-list__heading {
  display: flex;
  min-width: 0;
  align-items: baseline;
  flex-wrap: wrap;
  justify-content: space-between;
  gap: 10px;
  color: var(--app-text);
  font-size: 14px;
  line-height: 1.35;
}

.operation-effect-list__heading strong {
  min-width: 0;
  overflow-wrap: anywhere;
}

.operation-effect-list__remaining {
  display: inline-flex;
  flex: 0 0 auto;
  align-items: center;
  gap: 4px;
  color: var(--app-primary);
  font-size: 12px;
  font-variant-numeric: tabular-nums;
  white-space: nowrap;
}

.operation-effect-list__remaining svg,
.operation-effect-list__meta svg {
  flex: 0 0 auto;
  font-size: 13px;
}

.operation-effect-list__modifiers {
  display: grid;
  gap: 4px;
  margin: 7px 0 8px;
  color: var(--app-text);
  font-size: 12px;
  line-height: 1.45;
}

.operation-effect-list__modifiers span {
  display: flex;
  min-width: 0;
  align-items: flex-start;
  gap: 5px;
  overflow-wrap: anywhere;
}

.operation-effect-list__modifiers svg {
  flex: 0 0 auto;
  margin-top: 2px;
  color: var(--app-primary);
  font-size: 12px;
}

.operation-effect-list__meta {
  display: flex;
  flex-wrap: wrap;
  gap: 5px 13px;
  color: var(--app-muted);
  font-size: 11px;
}

.operation-effect-list__meta span {
  display: inline-flex;
  min-width: 0;
  align-items: center;
  gap: 4px;
}

.operation-effect-list__progress {
  height: 3px;
  margin-top: 9px;
  overflow: hidden;
  border-radius: 2px;
  background: var(--app-border);
}

.operation-effect-list__progress span {
  display: block;
  height: 100%;
  border-radius: inherit;
  background: var(--app-primary);
  transition: width 240ms linear;
}
</style>
