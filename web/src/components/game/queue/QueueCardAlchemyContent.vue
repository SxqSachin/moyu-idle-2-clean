<template>
  <div class="queue-alchemy">
    <div v-if="alchemy.subKind === 'convert'" class="queue-alchemy__row">
      <span class="queue-alchemy__label">{{ t("alchemy.queue.convertSource") }}</span>
      <QuickRef v-if="alchemy.sourceItemId" :item-id="alchemy.sourceItemId" :count="alchemy.sourceCount || 1" />
    </div>

    <div v-else-if="alchemy.subKind === 'bench'" class="queue-alchemy__stack">
      <div v-if="alchemy.carrierInstanceId" class="queue-alchemy__row">
        <span class="queue-alchemy__label">{{ t("alchemy.queue.benchCarrier") }}</span>
        <Card v-if="alchemy.carrierItemId" :item-id="alchemy.carrierItemId" :instance-id="alchemy.carrierInstanceId" />
        <span v-else class="queue-alchemy__meta">#{{ alchemy.carrierInstanceId }}</span>
      </div>
      <div v-if="materialEntries.length > 0" class="queue-alchemy__row queue-alchemy__row--wrap">
        <span class="queue-alchemy__label">{{ t("alchemy.queue.benchMaterials") }}</span>
        <QuickRef v-for="entry in materialEntries" :key="entry.itemId" :item-id="entry.itemId" :count="entry.count" />
      </div>
      <div v-if="alchemy.comboRecipeId" class="queue-alchemy__row">
        <span class="queue-alchemy__label">{{ t("alchemy.queue.benchCombo") }}</span>
        <span class="queue-alchemy__meta">{{ comboRecipeName }}</span>
      </div>
    </div>

    <div v-else-if="alchemy.subKind === 'transmute'" class="queue-alchemy__row">
      <span class="queue-alchemy__label">{{ t("alchemy.queue.transmuteItem") }}</span>
      <QuickRef v-if="alchemy.itemId" :item-id="alchemy.itemId" :count="alchemy.totalCount || 1" />
    </div>

    <div v-else-if="alchemy.subKind === 'inherit'" class="queue-alchemy__stack">
      <div class="queue-alchemy__row">
        <span class="queue-alchemy__label">{{ t("alchemy.queue.inheritParents") }}</span>
        <span class="queue-alchemy__meta">#{{ alchemy.fatherInstanceId }} / #{{ alchemy.motherInstanceId }}</span>
      </div>
      <div v-if="alchemy.mediumItemId" class="queue-alchemy__row">
        <span class="queue-alchemy__label">{{ t("alchemy.queue.inheritMedium") }}</span>
        <QuickRef :item-id="alchemy.mediumItemId" :count="1" />
      </div>
      <div v-if="alchemy.carrierInstanceId" class="queue-alchemy__row">
        <span class="queue-alchemy__label">{{ t("alchemy.queue.inheritCarrier") }}</span>
        <span class="queue-alchemy__meta">#{{ alchemy.carrierInstanceId }}</span>
      </div>
      <div class="queue-alchemy__row">
        <span class="queue-alchemy__label">{{ t("alchemy.queue.inheritGenes") }}</span>
        <span class="queue-alchemy__meta">{{ t("alchemy.queue.inheritGeneCounts", { include: alchemy.inheritGeneCount || 0, suppress: alchemy.suppressGeneCount || 0 }) }}</span>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from "vue"
import { useI18n } from "vue-i18n"
import QuickRef from "@components/game/item/QuickRef.vue"
import Card from "../item/Card.vue"
import type { QueueCard, QueueCardDisplayAlchemy } from "@components/game/queue/types"
import { listAlchemyComboRecipes } from "@services/alchemy-data"

const props = defineProps<{
  alchemy: QueueCardDisplayAlchemy
  card: QueueCard
}>()

const { t } = useI18n()

const materialEntries = computed(() => {
  if (!props.alchemy.materials) return []
  return Object.entries(props.alchemy.materials)
    .filter(([, count]) => Number(count) > 0)
    .map(([itemId, count]) => ({ itemId, count: Number(count) }))
})

const comboRecipeName = computed(() => {
  const id = String(props.alchemy.comboRecipeId || "").trim()
  if (!id) return ""
  const combo = listAlchemyComboRecipes().find((row) => row.id === id)
  return combo ? t(combo.nameKey, id) : id
})
</script>

<style scoped>
.queue-alchemy {
  display: grid;
  gap: 8px;
  margin-top: 10px;
  font-size: 12px;
}

.queue-alchemy__row {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 6px 8px;
}

.queue-alchemy__stack {
  display: grid;
  gap: 8px;
}

.queue-alchemy__label {
  color: var(--app-muted);
  font-weight: 600;
}

.queue-alchemy__meta {
  color: var(--app-muted);
}
</style>
