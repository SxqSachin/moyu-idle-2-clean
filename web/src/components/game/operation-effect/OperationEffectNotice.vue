<template>
  <section class="operation-effect-notice" :aria-label="t('game.fishCuisine.context.title')" data-testid="operation-effect-notice">
    <details class="operation-effect-notice__disclosure">
    <summary>
      <Icon icon="lucide:sparkles" />
      <strong>{{ t('game.fishCuisine.context.title') }}</strong>
      <span v-if="initialized && !stale" class="operation-effect-notice__count">{{ applicable.length }}</span>
      <Icon icon="lucide:chevron-down" class="operation-effect-notice__chevron" />
    </summary>
    <template v-if="initialized && !stale">
      <OperationEffectList v-if="applicable.length" :items="applicable" :now-ms="nowMs" compact />
      <p v-else class="operation-effect-notice__status">{{ t('game.fishCuisine.context.empty') }}</p>
      <details v-if="unavailable.length" class="operation-effect-notice__unavailable">
        <summary>{{ t('game.fishCuisine.context.unavailable', { count: unavailable.length }) }}</summary>
        <div v-for="row in unavailable" :key="row.effect.grantId" class="operation-effect-notice__reason">
          <span>{{ te(row.effect.nameKey) ? t(row.effect.nameKey) : row.effect.definitionId }}</span>
          <small>{{ t(`game.fishCuisine.context.${row.reason}`) }}</small>
        </div>
      </details>
      <p v-if="applicable.length" class="operation-effect-notice__hint">{{ t('game.fishCuisine.context.startBoundary') }}</p>
    </template>
    </details>
    <p v-if="!initialized || stale" class="operation-effect-notice__status" role="status">
      {{ t(stale ? 'game.fishCuisine.context.stale' : 'game.fishCuisine.context.loading') }}
    </p>
  </section>
</template>

<script setup lang="ts">
import { Icon } from "@iconify/vue"
import { useI18n } from "vue-i18n"
import { useOperationEffectContext } from "@/composables/game/useOperationEffectContext"
import type { OperationEffectContext } from "@services/operation-effect-context"
import OperationEffectList from "./OperationEffectList.vue"

const props = defineProps<{ context: OperationEffectContext }>()
const { t, te } = useI18n()
const { initialized, stale, nowMs, applicable, unavailable } = useOperationEffectContext(() => props.context)
</script>

<style scoped>
.operation-effect-notice { min-width: 0; padding: 12px 0; border-block: 1px solid var(--app-border); margin-block: 8px; }
.operation-effect-notice__disclosure > summary { display: flex; align-items: center; gap: 7px; color: var(--app-text); font-size: 13px; cursor: pointer; list-style: none; }
.operation-effect-notice__disclosure > summary::-webkit-details-marker { display: none; }
.operation-effect-notice__disclosure > summary:focus-visible { outline: 2px solid var(--app-primary); outline-offset: 3px; }
.operation-effect-notice__disclosure > summary > svg { color: var(--app-primary); flex: 0 0 auto; }
.operation-effect-notice__chevron { margin-left: auto; }
.operation-effect-notice__count + .operation-effect-notice__chevron { margin-left: 0; }
.operation-effect-notice__disclosure[open] > summary .operation-effect-notice__chevron { transform: rotate(180deg); }
.operation-effect-notice__count { margin-left: auto; color: var(--app-primary); font-variant-numeric: tabular-nums; }
.operation-effect-notice__status, .operation-effect-notice__hint { margin: 7px 0 0; font-size: 12px; line-height: 1.5; color: var(--app-text-secondary); overflow-wrap: anywhere; }
.operation-effect-notice__hint { color: var(--app-muted); }
.operation-effect-notice__unavailable { margin-top: 9px; font-size: 12px; color: var(--app-muted); }
.operation-effect-notice__unavailable summary { cursor: pointer; padding-block: 5px; }
.operation-effect-notice__unavailable summary:focus-visible { outline: 2px solid var(--app-primary); outline-offset: 2px; }
.operation-effect-notice__reason { display: grid; gap: 3px; padding-block: 6px; }
.operation-effect-notice__reason small { font-size: 12px; overflow-wrap: anywhere; }
</style>
