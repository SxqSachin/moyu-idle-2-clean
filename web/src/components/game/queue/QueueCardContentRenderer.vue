<template>
    <QueueCardEnhanceContent
      v-if="(card.display?.kind === 'equipment_enhance' || card.actionId === 'equipment_enhance') && card.display?.enhance"
      :enhance="card.display.enhance"
    />
    <QueueCardBuildingContent
      v-else-if="(card.display?.kind === 'building_upgrade' || card.actionId === 'building_upgrade') && card.display?.building"
      :building="card.display.building"
    />
    <QueueCardAlchemyContent
      v-else-if="card.display?.alchemy && (card.display.kind?.startsWith('alchemy.') || card.actionId?.startsWith('alchemy.'))"
      :alchemy="card.display.alchemy"
      :card="card"
    />
    <QueueCardFishingContent
      v-else-if="card.display?.fishing && (card.display.kind === 'fishing.trip' || card.actionId === 'fishing.trip')"
      :fishing="card.display.fishing"
      :card="card"
    />
</template>

<script setup lang="ts">
import QueueCardEnhanceContent from "@components/game/queue/QueueCardEnhanceContent.vue"
import QueueCardBuildingContent from "@components/game/queue/QueueCardBuildingContent.vue"
import QueueCardAlchemyContent from "@components/game/queue/QueueCardAlchemyContent.vue"
import QueueCardFishingContent from "@components/game/queue/QueueCardFishingContent.vue"
import type { QueueCard } from "@components/game/queue/types"

defineProps<{
  card: QueueCard
}>()
</script>
