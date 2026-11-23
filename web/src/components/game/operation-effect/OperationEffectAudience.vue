<template>
  <p v-if="messageKey" class="operation-effect-audience">
    <Icon icon="lucide:users" />
    <span>{{ t(messageKey) }}</span>
  </p>
</template>

<script setup lang="ts">
import { Icon } from "@iconify/vue"
import { computed } from "vue"
import { useI18n } from "vue-i18n"
import type { OperationEffectLifetimeKind, OperationEffectTargetKind } from "@services/operation-effects"
const props = defineProps<{ targets: OperationEffectTargetKind[]; lifetimeKind: OperationEffectLifetimeKind }>()
const { t } = useI18n()
const messageKey = computed(() => {
  if (props.targets.length > 1) return props.lifetimeKind === 'duration' ? 'game.fishCuisine.selectIndividualAudience' : 'game.fishCuisine.selectAudience'
  if (props.targets.includes('player_and_kitties')) return props.lifetimeKind === 'charges' ? 'game.fishCuisine.sharedCharges' : 'game.fishCuisine.sharedDuration'
  return ''
})
</script>

<style scoped>
.operation-effect-audience { display: flex; gap: 5px; margin: 7px 0 0; color: var(--app-text-secondary); font-size: 12px; line-height: 1.5; }
.operation-effect-audience svg { flex: 0 0 auto; margin-top: 2px; color: var(--app-primary); }
.operation-effect-audience span { min-width: 0; overflow-wrap: anywhere; }
</style>
