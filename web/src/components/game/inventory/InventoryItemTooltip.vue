<template>
  <el-tooltip placement="top" effect="dark" :show-after="150">
    <template #content>
      <div class="space-y-2 max-w-[360px]">
        <div class="font-semibold">{{ itemLabel }}</div>
        <div class="text-xs opacity-80">Lv+{{ enhanceLevel }} · x{{ quantity }}</div>
        <div>
          <div class="text-xs opacity-80">{{ t("game.itemTooltip.usedIn") }}</div>
          <div v-if="insight.uses.length" class="text-xs">
            {{ insight.uses.map(formatFlowLabel).join(" / ") }}
          </div>
          <div v-else class="text-xs opacity-70">{{ t("game.itemTooltip.none") }}</div>
        </div>
        <div>
          <div class="text-xs opacity-80">{{ t("game.itemTooltip.sourcedFrom") }}</div>
          <div v-if="insight.sources.length" class="text-xs">
            {{ insight.sources.map(formatFlowLabel).join(" / ") }}
          </div>
          <div v-else class="text-xs opacity-70">{{ t("game.itemTooltip.none") }}</div>
        </div>
      </div>
    </template>
    <slot />
  </el-tooltip>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from "vue"
import { useI18n } from "vue-i18n"
import { useGameStore } from "@stores/game"
import { getItemInsightMap, type ItemFlowRef, type ItemInsight } from "@services/item-insights"

const props = defineProps<{
  itemId: string
  itemLabel: string
  quantity: number
  enhanceLevel: number
}>()

const { t } = useI18n()
const store = useGameStore()
const insightMap = ref<Record<string, ItemInsight>>({})

function emptyItemInsight(): ItemInsight {
  return {
    uses: [],
    sources: [],
    enemySources: [],
    catVillageCommissionSources: [],
    catteryCommissionSources: [],
    kittyJournalSources: []
  }
}

const insight = computed<ItemInsight>(() => insightMap.value[props.itemId] || emptyItemInsight())

function formatFlowLabel(flow: ItemFlowRef): string {
  const name = t(flow.nameKey)
  if (name !== flow.nameKey && name.trim() !== "") return name
  const menuLabelKey = `game.menu.${flow.menu}`
  const menuLabel = t(menuLabelKey)
  if (menuLabel !== menuLabelKey) return `${menuLabel}:${flow.actionId}`
  const mainTypeLabelKey = `action_main_type.${flow.menu}.name`
  const mainTypeLabel = t(mainTypeLabelKey)
  if (mainTypeLabel !== mainTypeLabelKey && mainTypeLabel.trim() !== "") return `${mainTypeLabel}:${flow.actionId}`
  return `${flow.menu}:${flow.actionId}`
}

onMounted(async () => {
  insightMap.value = await getItemInsightMap(store.token)
})
</script>
