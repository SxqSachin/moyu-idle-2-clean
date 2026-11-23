<template>
  <div class="fish-filter-controls">
    <el-input :model-value="modelValue.search" clearable :placeholder="t('game.fishing.fishSelectorSearch')" @update:model-value="update('search', $event)">
      <template #prefix><Icon icon="lucide:search" /></template>
    </el-input>
    <el-select :model-value="modelValue.species" filterable :placeholder="t('game.fishing.fishSelectorAllSpecies')" :aria-label="t('game.fishing.species')" @update:model-value="update('species', $event)">
      <el-option value="" :label="t('game.fishing.fishSelectorAllSpecies')" />
      <el-option v-for="row in species" :key="row.id" :value="row.id" :label="localizedFishingText(row.names, locale) || row.id" />
    </el-select>
    <el-select :model-value="modelValue.rarity" :placeholder="t('game.fishing.fishSelectorAllRarities')" :aria-label="t('game.fishing.fishSelectorRarity')" @update:model-value="update('rarity', $event)">
      <el-option value="" :label="t('game.fishing.fishSelectorAllRarities')" />
      <el-option v-for="rarity in rarities" :key="rarity" :value="rarity" :label="t(getRarityNameKey(rarity))"><RarityText :rarity-id="rarity" /></el-option>
    </el-select>
    <el-select :model-value="modelValue.mutation" :placeholder="t('game.fishing.fishSelectorAllMutations')" :aria-label="t('game.fishing.fishSelectorMutation')" @update:model-value="update('mutation', $event)">
      <el-option value="" :label="t('game.fishing.fishSelectorAllMutations')" />
      <el-option value="any" :label="t('game.fishing.fishSelectorAnyMutation')" />
      <el-option value="none" :label="t('game.fishing.fishSelectorNoMutation')" />
      <el-option v-for="mutation in mutations" :key="mutation.id" :value="mutation.id" :label="t(mutation.nameKey)" />
    </el-select>
    <el-select :model-value="modelValue.health" :aria-label="t('game.fishing.health')" @update:model-value="update('health', $event)">
      <el-option value="all" :label="t('game.fishing.fishSelectorAllHealth')" />
      <el-option value="healthy" :label="t('game.fishing.fishSelectorHealthy')" />
      <el-option value="attention" :label="t('game.fishing.fishSelectorNeedsCare')" />
    </el-select>
    <div class="fish-filter-controls__sort">
      <el-select :model-value="modelValue.sort" :aria-label="t('game.fishing.fishSelectorSort')" @update:model-value="update('sort', $event)">
        <el-option value="recent" :label="t('game.fishing.fishSelectorSortRecent')" />
        <el-option v-for="key in sortKeys" :key="key.value" :value="key.value" :label="t(key.label)" />
      </el-select>
      <el-tooltip :content="t('game.fishing.fishSelectorToggleSort')">
        <el-button circle :aria-label="t('game.fishing.fishSelectorToggleSort')" @click="update('direction', modelValue.direction === 'desc' ? 'asc' : 'desc')"><Icon :icon="modelValue.direction === 'desc' ? 'lucide:arrow-down-wide-narrow' : 'lucide:arrow-up-narrow-wide'" /></el-button>
      </el-tooltip>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from "vue"
import { Icon } from "@iconify/vue"
import { useI18n } from "vue-i18n"
import RarityText from "@components/game/common/RarityText.vue"
import { getRarityNameKey } from "@services/item-definition"
import { fishFilterRarities, type FishFilters } from "@services/fish-filters"
import { localizedFishingText, type FishingMutationDefinition, type FishingSpecies } from "@services/fishing"

const props = defineProps<{ modelValue: FishFilters; species: FishingSpecies[]; mutations: FishingMutationDefinition[] }>()
const emit = defineEmits<{ (event: "update:modelValue", value: FishFilters): void }>()
const { t, locale } = useI18n()
const rarities = computed(() => fishFilterRarities())
const sortKeys = [
  { value: "rarity", label: "game.fishing.fishSelectorSortRarity" },
  { value: "weight", label: "game.fishing.fishSelectorSortWeight" },
  { value: "length", label: "game.fishing.fishSelectorSortLength" },
  { value: "health", label: "game.fishing.fishSelectorSortHealth" },
  { value: "name", label: "game.fishing.fishSelectorSortName" }
]
function update<K extends keyof FishFilters>(key: K, value: FishFilters[K]) {
  emit("update:modelValue", { ...props.modelValue, [key]: value })
}
</script>

<style scoped>
.fish-filter-controls { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 8px; align-items: center; }
.fish-filter-controls > *, .fish-filter-controls__sort > .el-select { min-width: 0; width: 100%; }
.fish-filter-controls__sort { display: flex; gap: 8px; align-items: center; }
@media (max-width: 600px) { .fish-filter-controls { grid-template-columns: repeat(2, minmax(0, 1fr)); }.fish-filter-controls > :first-child, .fish-filter-controls__sort { grid-column: 1 / -1; } }
</style>
