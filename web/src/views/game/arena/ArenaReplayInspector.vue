<template>
  <section class="replay-inspector" :aria-label="t('game.arena.replayAnalysis')">
    <header class="replay-inspector__header">
      <span><Icon icon="lucide:chart-no-axes-combined" /></span>
      <div>
        <strong>{{ t("game.arena.replayAnalysis") }}</strong>
        <small>{{ t("game.arena.replayAnalysisHint") }}</small>
      </div>
    </header>

    <el-tabs v-model="tab" class="replay-inspector__tabs">
      <el-tab-pane :label="t('game.arena.replayCurrentTurn')" name="turn">
        <div v-if="selectedTurn" class="replay-turn-insight">
          <div class="replay-turn-insight__identity">
            <span :class="`replay-turn-insight__side replay-turn-insight__side--${selectedTurn.side || 'neutral'}`">
              {{ sideLabel(selectedTurn.side) }}
            </span>
            <strong>{{ actorLabel(selectedTurn.actorName) }}</strong>
            <small>{{ skillLabel(selectedTurn) }}</small>
          </div>

          <dl class="replay-turn-insight__metrics">
            <div><dt>{{ t("game.arena.replayDamage") }}</dt><dd class="is-damage">{{ formatNumber(selectedTurn.damage) }}</dd></div>
            <div><dt>{{ t("game.arena.replayHealing") }}</dt><dd class="is-healing">{{ formatNumber(selectedTurn.healing) }}</dd></div>
            <div><dt>{{ t("game.arena.replayCriticals") }}</dt><dd>{{ selectedTurn.criticalHits }}</dd></div>
            <div><dt>{{ t("game.arena.replayKnockouts") }}</dt><dd>{{ selectedTurn.knockouts }}</dd></div>
            <div><dt>{{ t("game.arena.replayStatusChanges") }}</dt><dd>{{ selectedTurn.statusChanges }}</dd></div>
          </dl>

          <div class="replay-turn-insight__tags">
            <el-tag v-if="selectedTurn.preparationTurn" size="small" type="success" effect="plain">{{ t("game.arena.replayPreparationTurn") }}</el-tag>
            <el-tag v-if="selectedTurn.extraTurn" size="small" type="warning" effect="plain">{{ t("game.arena.replayExtraTurn") }}</el-tag>
            <el-tag v-if="selectedTurn.syncedTurn" size="small" type="info" effect="plain">{{ t("game.arena.replaySyncedTurn") }}</el-tag>
            <el-tag v-if="selectedTurn.keyMoment" size="small" type="danger" effect="plain">{{ t("game.arena.replayKeyMoment") }}</el-tag>
          </div>
        </div>

        <div v-else class="replay-inspector__empty">
          <Icon icon="lucide:mouse-pointer-click" />
          <span>{{ t("game.arena.replayNoTurnSelected") }}</span>
        </div>
      </el-tab-pane>

      <el-tab-pane :label="t('game.arena.replayContribution')" name="contribution">
        <div class="replay-contribution">
          <div class="replay-side-comparison">
            <div v-for="side in sideRows" :key="side.key" :class="`replay-side-comparison__row replay-side-comparison__row--${side.key}`">
              <strong>{{ side.label }}</strong>
              <span><Icon icon="lucide:burst" />{{ formatNumber(side.damage) }}</span>
              <span><Icon icon="lucide:heart-pulse" />{{ formatNumber(side.healing) }}</span>
              <span><Icon icon="lucide:circle-x" />{{ side.knockouts }}</span>
            </div>
          </div>

          <div v-if="analysis.actors.length > 0" class="replay-actor-list">
            <div v-for="actor in analysis.actors" :key="actor.actorId" class="replay-actor-row">
              <div class="replay-actor-row__heading">
                <span :class="`replay-actor-row__marker replay-actor-row__marker--${actor.side || 'neutral'}`" />
                <strong>{{ actorLabel(actor.actorName) }}</strong>
                <small>{{ t("game.arena.replayActorTurns", { count: actor.turns }) }}</small>
              </div>
              <div class="replay-actor-row__bar" aria-hidden="true">
                <i :style="{ width: `${impactPercent(actor)}%` }" />
              </div>
              <div class="replay-actor-row__values">
                <span class="is-damage"><Icon icon="lucide:burst" />{{ formatNumber(actor.damage) }}</span>
                <span class="is-healing"><Icon icon="lucide:heart-pulse" />{{ formatNumber(actor.healing) }}</span>
                <span v-if="actor.knockouts > 0"><Icon icon="lucide:circle-x" />{{ actor.knockouts }}</span>
              </div>
            </div>
          </div>

          <div v-else class="replay-inspector__empty">
            <Icon icon="lucide:chart-no-axes-column" />
            <span>{{ t("game.arena.replayNoContribution") }}</span>
          </div>
        </div>
      </el-tab-pane>
    </el-tabs>
  </section>
</template>

<script setup lang="ts">
import { computed, ref } from "vue"
import { Icon } from "@iconify/vue"
import { useI18n } from "vue-i18n"
import { getLocalizedSkillName } from "@services/skill-definition"
import { formatNumberWithThousands } from "@utils/number-format"
import type {
  ArenaReplayActorContribution,
  ArenaReplayAnalysis,
  ArenaReplayTurnSummary
} from "@services/arena-replay-analysis"

const props = withDefaults(defineProps<{
  analysis: ArenaReplayAnalysis
  selectedTurn: ArenaReplayTurnSummary | null
  battleKind?: "arena" | "combat_trial"
}>(), {
  battleKind: "arena"
})

const { t } = useI18n()
const tab = ref<"turn" | "contribution">("turn")
const isCombatTrial = computed(() => props.battleKind === "combat_trial")
const maxImpact = computed(() => Math.max(1, ...props.analysis.actors.map((actor) => actor.damage + actor.healing)))
const sideRows = computed(() => [
  { key: "player", label: sideLabel("player"), ...props.analysis.player },
  { key: "enemy", label: sideLabel("enemy"), ...props.analysis.enemy }
])

function sideLabel(side: string): string {
  if (isCombatTrial.value) {
    return t(side === "enemy" ? "game.combatTrial.opponents" : "game.combatTrial.challengers")
  }
  return t(side === "enemy" ? "game.arena.defender" : "game.arena.attacker")
}

function skillLabel(turn: ArenaReplayTurnSummary): string {
  if (!turn.skillId && !turn.skillNameFallback) return t("game.arena.replayNoAction")
  return getLocalizedSkillName(t, turn.skillId, turn.skillNameFallback || turn.skillId)
}

function actorLabel(raw: string): string {
  const name = String(raw || "").trim()
  if (!name) return t("game.arena.unknownUnit")
  const translated = t(name)
  return translated && translated !== name ? translated : name
}

function impactPercent(actor: ArenaReplayActorContribution): number {
  return Math.max(3, Math.round(((actor.damage + actor.healing) / maxImpact.value) * 100))
}

function formatNumber(value: number): string {
  return formatNumberWithThousands(Math.round(Number(value) || 0))
}
</script>

<style scoped>
.replay-inspector {
  min-width: 0;
  max-height: min(650px, calc(100vh - 310px));
  overflow: hidden;
  border: 1px solid var(--el-border-color-lighter);
  border-radius: 6px;
  background: var(--el-bg-color);
}

.replay-inspector__header {
  min-height: 50px;
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px 10px;
  border-bottom: 1px solid var(--el-border-color-lighter);
  background: color-mix(in srgb, var(--el-fill-color-light) 68%, var(--el-bg-color));
}

.replay-inspector__header > span {
  width: 28px;
  height: 28px;
  display: grid;
  place-items: center;
  flex: none;
  border-radius: 4px;
  color: var(--el-color-success);
  background: color-mix(in srgb, var(--el-color-success) 10%, transparent);
}

.replay-inspector__header > div {
  min-width: 0;
  display: grid;
  gap: 1px;
}

.replay-inspector__header strong {
  font-size: 0.78rem;
}

.replay-inspector__header small {
  overflow: hidden;
  color: var(--app-muted);
  font-size: 0.62rem;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.replay-inspector__tabs :deep(.el-tabs__header) {
  margin: 0;
  padding: 0 10px;
}

.replay-inspector__tabs :deep(.el-tabs__item) {
  height: 38px;
  padding-inline: 8px;
  font-size: 0.7rem;
}

.replay-inspector__tabs :deep(.el-tabs__content) {
  padding: 0 10px 10px;
}

.replay-turn-insight__identity {
  min-height: 78px;
  display: grid;
  align-content: center;
  gap: 4px;
  border-bottom: 1px solid var(--el-border-color-extra-light);
}

.replay-turn-insight__identity strong,
.replay-turn-insight__identity small {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.replay-turn-insight__identity strong {
  font-size: 0.82rem;
}

.replay-turn-insight__identity small {
  color: var(--app-muted);
  font-size: 0.65rem;
}

.replay-turn-insight__side {
  width: fit-content;
  color: var(--el-color-primary);
  font-size: 0.6rem;
  font-weight: 700;
}

.replay-turn-insight__side--enemy {
  color: var(--el-color-warning-dark-2);
}

.replay-turn-insight__metrics {
  margin: 0;
}

.replay-turn-insight__metrics > div {
  min-height: 34px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  border-bottom: 1px solid var(--el-border-color-extra-light);
}

.replay-turn-insight__metrics dt {
  color: var(--app-muted);
  font-size: 0.65rem;
}

.replay-turn-insight__metrics dd {
  margin: 0;
  font-size: 0.72rem;
  font-variant-numeric: tabular-nums;
  font-weight: 700;
}

.is-damage {
  color: var(--el-color-danger);
}

.is-healing {
  color: var(--el-color-success);
}

.replay-turn-insight__tags {
  display: flex;
  flex-wrap: wrap;
  gap: 5px;
  padding-top: 9px;
}

.replay-side-comparison {
  border-bottom: 1px solid var(--el-border-color-extra-light);
}

.replay-side-comparison__row {
  min-height: 38px;
  display: grid;
  grid-template-columns: minmax(56px, 1fr) repeat(3, auto);
  align-items: center;
  gap: 7px;
  color: var(--app-muted);
  font-size: 0.61rem;
  font-variant-numeric: tabular-nums;
}

.replay-side-comparison__row + .replay-side-comparison__row {
  border-top: 1px solid var(--el-border-color-extra-light);
}

.replay-side-comparison__row strong {
  color: var(--el-color-primary);
  font-size: 0.66rem;
}

.replay-side-comparison__row--enemy strong {
  color: var(--el-color-warning-dark-2);
}

.replay-side-comparison__row span,
.replay-actor-row__values span {
  display: inline-flex;
  align-items: center;
  gap: 3px;
}

.replay-actor-list {
  max-height: 430px;
  overflow-y: auto;
}

.replay-actor-row {
  display: grid;
  gap: 5px;
  padding: 9px 0;
  border-bottom: 1px solid var(--el-border-color-extra-light);
}

.replay-actor-row__heading {
  min-width: 0;
  display: grid;
  grid-template-columns: 7px minmax(0, 1fr) auto;
  align-items: center;
  gap: 6px;
}

.replay-actor-row__heading strong {
  overflow: hidden;
  font-size: 0.68rem;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.replay-actor-row__heading small {
  color: var(--app-muted);
  font-size: 0.58rem;
}

.replay-actor-row__marker {
  width: 7px;
  height: 7px;
  border-radius: 50%;
  background: var(--el-color-info);
}

.replay-actor-row__marker--player {
  background: var(--el-color-primary);
}

.replay-actor-row__marker--enemy {
  background: var(--el-color-warning);
}

.replay-actor-row__bar {
  height: 3px;
  overflow: hidden;
  background: var(--el-fill-color-darker);
}

.replay-actor-row__bar i {
  display: block;
  height: 100%;
  background: var(--el-color-primary);
}

.replay-actor-row__values {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  color: var(--app-muted);
  font-size: 0.59rem;
  font-variant-numeric: tabular-nums;
}

.replay-inspector__empty {
  min-height: 180px;
  display: grid;
  justify-items: center;
  align-content: center;
  gap: 7px;
  color: var(--app-muted);
  font-size: 0.68rem;
  text-align: center;
}

.replay-inspector__empty svg {
  font-size: 1.05rem;
}

@media (max-width: 1180px) and (min-width: 921px) {
  .replay-inspector__tabs :deep(.el-tabs__content) {
    padding-inline: 12px;
  }

  .replay-actor-list {
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
    column-gap: 18px;
  }
}
</style>
