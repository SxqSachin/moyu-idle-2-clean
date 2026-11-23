<template>
  <div class="node-inspector" data-testid="specialization-tooltip-card">
    <header class="node-inspector__header">
      <div>
        <div class="node-inspector__schools">
          <el-tag v-for="school in schoolNames" :key="school" effect="plain" size="small">{{ school }}</el-tag>
        </div>
        <h2>{{ displayName }}</h2>
      </div>
      <el-button
        text
        circle
        :icon="Close"
        :aria-label="t('game.specialization.closeDetails')"
        @click.stop="emit('close')"
      />
    </header>

    <dl class="node-inspector__metrics">
      <div>
        <dt>{{ t("game.specialization.rank") }}</dt>
        <dd>{{ nodeState.rank }} / {{ nodeState.maxRank }}</dd>
      </div>
      <div>
        <dt>{{ t("game.specialization.nextCost") }}</dt>
        <dd>{{ nodeState.rank >= nodeState.maxRank ? "-" : nodeState.nextCost }}</dd>
      </div>
    </dl>

    <section class="node-inspector__summary" data-testid="specialization-effect-summary">
      <div v-if="nodeState.rank > 0" class="node-inspector__effect-level">
        <strong>{{ t("game.specialization.currentRankEffect", { rank: nodeState.rank }) }}</strong>
        <ul v-if="currentEffectSummaryLines.length" class="node-inspector__summary-list">
          <li v-for="line in currentEffectSummaryLines" :key="`current-${line}`">
            <SpecializationKeywordText :text="line" :keywords="keywords" />
          </li>
        </ul>
        <p v-else class="node-inspector__muted">{{ t("game.specialization.noEffects") }}</p>
      </div>
      <div v-if="nodeState.rank < nodeState.maxRank" class="node-inspector__effect-level is-next">
        <strong>
          {{ nodeState.rank === 0
            ? t("game.specialization.unlockEffect")
            : t("game.specialization.nextRankEffect", { rank: nodeState.rank + 1 }) }}
        </strong>
        <ul v-if="nextEffectSummaryLines.length" class="node-inspector__summary-list">
          <li v-for="line in nextEffectSummaryLines" :key="`next-${line}`">
            <SpecializationKeywordText :text="line" :keywords="keywords" />
          </li>
        </ul>
        <p v-else class="node-inspector__muted">{{ t("game.specialization.noEffects") }}</p>
      </div>
    </section>

    <section class="node-inspector__route">
      <h3>{{ t("game.specialization.routeRequirements") }}</h3>
      <ul v-if="requirementLines.length" class="node-inspector__list">
        <li v-for="line in requirementLines" :key="line">
          {{ line }}
        </li>
      </ul>
      <p v-else class="node-inspector__muted">{{ t("game.specialization.noRequirements") }}</p>
      <ul v-if="nodeState.unmetReasonCodes.length" class="node-inspector__reasons">
        <li v-for="code in nodeState.unmetReasonCodes" :key="code">
          {{ reasonLabel(code) }}
        </li>
      </ul>
    </section>

    <details class="node-inspector__technical" data-testid="specialization-technical-details">
      <summary>
        <span>
          {{ nodeState.rank < nodeState.maxRank
            ? t("game.specialization.technicalNextDetails", { rank: nodeState.rank + 1 })
            : t("game.specialization.technicalCurrentDetails", { rank: nodeState.rank }) }}
        </span>
        <small>{{ t("game.specialization.technicalLineCount", { count: technicalEffectLines.length }) }}</small>
      </summary>
      <ul v-if="technicalEffectLines.length" class="node-inspector__list">
        <li v-for="line in technicalEffectLines" :key="line">
          <SpecializationKeywordText :text="line" :keywords="keywords" />
        </li>
      </ul>
      <p v-else class="node-inspector__muted">{{ t("game.specialization.noEffects") }}</p>
    </details>

    <el-button
      class="node-inspector__allocate"
      type="primary"
      :disabled="!nodeState.canAllocate || nodeState.rank >= nodeState.maxRank || busy"
      :loading="busy"
      @click.stop="emit('allocate')"
    >
      {{ nodeState.rank >= nodeState.maxRank
        ? t("game.specialization.learned")
        : t("game.specialization.learnRank", { rank: nodeState.rank + 1, cost: nodeState.nextCost }) }}
    </el-button>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from "vue-i18n"
import { Close } from "@element-plus/icons-vue"
import SpecializationKeywordText from "@components/game/attributes/SpecializationKeywordText.vue"
import type { ResolvedSpecializationKeyword } from "@services/specialization-keywords"
import type { SpecializationNodeState } from "@services/specialization"

defineProps<{
  nodeState: SpecializationNodeState
  displayName: string
  schoolNames: string[]
  keywords: ResolvedSpecializationKeyword[]
  currentEffectSummaryLines: string[]
  nextEffectSummaryLines: string[]
  technicalEffectLines: string[]
  requirementLines: string[]
  busy: boolean
}>()

const emit = defineEmits<{
  (e: "allocate"): void
  (e: "close"): void
}>()

const { t } = useI18n()

function reasonLabel(code: string) {
  const localized = (key: string, fallback: string) => {
    const value = t(key)
    return value === key ? fallback : value
  }
  const key = `game.specialization.unmet.${code}`
  return localized(key, code)
}
</script>

<style scoped>
.node-inspector {
  display: grid;
  max-height: min(62vh, 560px);
  gap: 10px;
  overflow: auto;
  padding-right: 3px;
}

@media (max-width: 767px) {
  .node-inspector {
    max-height: min(54vh, 456px);
  }
}

.node-inspector__header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
}

.node-inspector__header > div {
  display: flex;
  min-width: 0;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.node-inspector__header h2 {
  margin: 0;
  overflow-wrap: anywhere;
  font-size: 17px;
  line-height: 1.25;
  letter-spacing: 0;
}

.node-inspector__muted {
  margin: 0;
  color: var(--app-text-muted);
  font-size: 13px;
  line-height: 1.6;
}

.node-inspector__schools {
  display: flex;
  gap: 4px;
  flex-wrap: wrap;
}

.node-inspector__metrics {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  margin: 0;
  border-block: 1px solid var(--app-border);
}

.node-inspector__metrics div {
  display: flex;
  min-width: 0;
  align-items: baseline;
  gap: 6px;
  padding: 6px 0;
}

.node-inspector__metrics div + div {
  border-left: 1px solid var(--app-border);
  padding-left: 12px;
}

.node-inspector__metrics dt {
  color: var(--app-text-muted);
  font-size: 11px;
  line-height: 1.3;
}

.node-inspector__metrics dd {
  margin: 0;
  font-size: 14px;
  font-weight: 700;
  line-height: 1.2;
}

.node-inspector section {
  display: grid;
  gap: 8px;
}

.node-inspector section h3 {
  margin: 0;
  font-size: 12px;
  text-transform: none;
  letter-spacing: 0;
}

.node-inspector__list,
.node-inspector__reasons {
  display: grid;
  gap: 7px;
  margin: 0;
  padding-left: 18px;
  color: var(--app-text-muted);
  font-size: 12px;
  line-height: 1.5;
}

.node-inspector__summary-list {
  display: grid;
  gap: 10px;
  margin: 0;
  padding: 0;
  list-style: none;
  color: var(--app-text);
  font-size: 13px;
  line-height: 1.65;
}

.node-inspector__effect-level {
  display: grid;
  gap: 7px;
}

.node-inspector__effect-level + .node-inspector__effect-level {
  border-top: 1px dashed var(--app-border);
  padding-top: 10px;
}

.node-inspector__effect-level > strong {
  color: var(--app-text-secondary);
  font-size: 11px;
  font-weight: 700;
}

.node-inspector__effect-level.is-next > strong {
  color: var(--app-primary);
}

.node-inspector__summary-list li {
  position: relative;
  padding-left: 14px;
}

.node-inspector__summary-list li::before {
  position: absolute;
  top: 0.72em;
  left: 0;
  width: 5px;
  height: 5px;
  border-radius: 50%;
  background: var(--app-primary);
  content: "";
}

.node-inspector__route {
  border-top: 1px solid var(--app-border);
  padding-top: 12px;
}

.node-inspector__technical {
  border-top: 1px solid var(--app-border);
  padding-top: 11px;
}

.node-inspector__technical summary {
  display: flex;
  cursor: pointer;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  color: var(--app-text-secondary);
  font-size: 12px;
  font-weight: 600;
  list-style-position: inside;
}

.node-inspector__technical summary small {
  flex: 0 0 auto;
  color: var(--app-text-muted);
  font-size: 10px;
  font-weight: 400;
}

.node-inspector__technical[open] summary {
  margin-bottom: 12px;
}

.node-inspector__reasons {
  color: var(--game-danger);
}

.node-inspector__allocate {
  width: 100%;
  min-height: 40px;
}
</style>
