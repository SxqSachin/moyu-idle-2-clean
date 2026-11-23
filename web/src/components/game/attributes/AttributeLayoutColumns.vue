<template>
  <div
    class="attribute-layout-columns"
    :class="`attribute-layout-columns--${layout || 'default'}`"
    :style="{ '--attribute-column-count': String(normalizedColumns.length || 1) }"
  >
    <div v-for="(column, columnIndex) in normalizedColumns" :key="`attr-col-${columnIndex}`" class="attribute-layout-column">
      <template v-for="cell in column" :key="`attr-cell-${columnIndex}-${cell.key}`">
        <AttributeStatCard
          v-if="cell.value && !cell.placeholder"
          :row="cell.value"
          :variant="cardVariant || 'default'"
          :emphasis="emphasis || 'normal'"
        />
        <div v-else class="attribute-layout-placeholder" />
      </template>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from "vue"
import type { AttributeRow } from "@/composables/game/attributes/useAttributeRows"
import type { AttributeLayoutCell } from "@services/attribute-layout"
import AttributeStatCard from "@components/game/attributes/AttributeStatCard.vue"

const props = defineProps<{
  columns: Array<Array<AttributeLayoutCell<AttributeRow>>>
  cardVariant?: "default" | "split"
  emphasis?: "normal" | "primary"
  layout?: "default" | "primary" | "dense"
}>()

const normalizedColumns = computed(() => {
  if (props.columns.length > 0) return props.columns
  return [[]]
})
</script>

<style scoped>
.attribute-layout-columns {
  --attribute-column-count: 1;
  display: grid;
  grid-template-columns: minmax(0, 1fr);
  gap: 8px;
}

.attribute-layout-column {
  display: grid;
  align-content: start;
  gap: 8px;
  min-width: 0;
}

.attribute-layout-placeholder {
  min-height: 56px;
  border: 1px solid transparent;
  border-radius: 8px;
}

@media (min-width: 768px) {
  .attribute-layout-columns {
    grid-template-columns: repeat(var(--attribute-column-count), minmax(0, 1fr));
  }
}

</style>
