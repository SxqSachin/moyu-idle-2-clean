<template>
  <div class="page-shell space-y-4">
    <el-card class="app-card app-card--no-border game-page-frame" shadow="never" data-tutorial="building-page">
      <template #header>
        <GamePageHeader
          title-key="game.menu.building"
          tutorial-id="building"
          :visual-src="buildingVisual.src"
          :visual-tone="buildingVisual.tone"
          visual-key="building"
        />
      </template>
      <div class="mb-4 flex items-center gap-2" data-tutorial="building-actor">
        <span class="text-sm app-muted">{{ t("game.actionExecutor") }}:</span>
        <el-select v-model="selectedActorKey" style="max-width: 280px">
          <el-option
            v-for="option in actorOptions"
            :key="option.key"
            :label="option.label"
            :value="option.key"
          >
            <div class="flex items-center gap-1">
              <span>{{ option.label }}</span>
              <el-tooltip v-if="option.actorType === 'kitty'" :content="kittyExecutorHint" placement="top">
                <span class="cursor-help text-[var(--app-muted)]">ⓘ</span>
              </el-tooltip>
            </div>
          </el-option>
        </el-select>
      </div>
      <div v-if="buildingRows.length" class="grid gap-4 md:grid-cols-2 xl:grid-cols-3" data-tutorial="building-list">
        <el-card v-for="row in buildingRows" :key="row.id" class="border border-[var(--app-border)]" shadow="never">
          <div class="space-y-3">
            <div class="flex items-center justify-between">
              <div class="font-semibold">{{ row.name }}</div>
              <div class="text-xs app-muted">{{ t("game.buildingLevel") }} {{ row.currentLevel }} / {{ row.maxLevel }}</div>
            </div>

            <div v-if="row.maxed" class="text-xs text-[var(--app-muted)]">
              {{ t("game.buildingMaxed") }}
            </div>
            <template v-else>
              <div class="text-xs app-muted">
                {{ t("game.buildingUpgradeTo", { level: row.nextLevel }) }}
              </div>

              <div class="space-y-2">
                <div class="text-xs app-muted">{{ t("game.actionDuration") }}: {{ formatDurationI18n(resolveActorDurationSeconds(row.nextDurationS), t) }}</div>
                <div v-if="row.costEntries.length" class="flex flex-wrap gap-2">
                  <QuickRefWithRequire
                    v-for="cost in row.costEntries"
                    :key="`${row.id}-${cost.itemId}`"
                    :item-id="cost.itemId"
                    :required-count="cost.quantity"
                  />
                </div>
                <div v-if="row.statusEntries.length" class="flex flex-wrap gap-2">
                  <StatusRequireTag
                    v-for="requirement in row.statusEntries"
                    :key="`${row.id}-${requirement.attrKey}`"
                    :attr-key="requirement.attrKey"
                    :required-value="requirement.required"
                  />
                </div>
                <div v-if="row.id === 'fish_pond' && row.nextBonuses" class="pond-bonus-preview">
                  <span><b>{{ row.nextBonuses.fishCapacity || 0 }}</b>{{ t("game.fishing.pondMainZone") }}</span>
                  <span><b>{{ row.nextBonuses.nurseryCapacity || 0 }}</b>{{ t("game.fishing.pondNurseryZone") }}</span>
                  <span><b>{{ row.nextBonuses.biomassKg || 0 }} kg</b>{{ t("game.fishing.pondBiomass") }}</span>
                  <span><b>{{ row.nextBonuses.breedingPairs || 0 }}</b>{{ t("game.fishing.pondPairCapacity") }}</span>
                  <span><b>{{ row.nextBonuses.feederNutrition || 0 }}</b>{{ t("game.fishing.pondFeeder") }}</span>
                </div>
              </div>

              <div class="flex gap-2 pt-2">
                <el-button
                  class="w-full"
                  type="primary"
                  :disabled="row.runDisabled"
                  @click="runNow(row.id)"
                >
                  {{ t("game.actionRunNow") }}
                </el-button>
                <el-button
                  class="w-full"
                  :disabled="row.queueDisabled"
                  @click="enqueue(row.id)"
                >
                  {{ t("game.actionEnqueue") }}
                </el-button>
              </div>
            </template>
          </div>
        </el-card>
      </div>
      <div v-else class="text-sm app-muted">{{ t("game.actionEmpty") }}</div>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, watch } from "vue"
import { ElMessage } from "element-plus"
import { useI18n } from "vue-i18n"
import { getGameDataChunk } from "@services/game-data-runtime"
import StatusRequireTag from "@components/game/attribute/StatusRequireTag.vue"
import QuickRefWithRequire from "@components/game/item/QuickRefWithRequire.vue"
import { useActionSubmit } from "@composables/game/action/useActionSubmit"
import { useInventorySummary } from "@composables/game/useInventorySummary"
import { useAttributeStats } from "@composables/game/attributes/useAttributeStats"
import { usePlayerStore } from "@stores/player"
import { KITTY_STATUS_EXP_MULTIPLIER } from "@/config/shared-global-config"
import { formatDurationI18n } from "@/utils/duration-format"
import GamePageHeader from "@components/game/common/GamePageHeader.vue"
import { gamePageVisuals } from "@services/game-page-visuals"

type BuildingDefinition = {
  id: string
  iconId?: string
  nameKey?: string
  descKey?: string
  order?: number
  maxLevel?: number
  baseDurationS?: number
  stepDurationS?: number
  baseCosts?: Record<string, number>
  stepCosts?: Record<string, number>
  baseStatusRequirement?: Record<string, number>
  stepStatusRequirement?: Record<string, number>
  levels?: BuildingLevelDefinition[]
}

type BuildingLevelDefinition = {
  level: number
  durationS: number
  costs: Record<string, number>
  statusRequirement: Record<string, number>
  bonuses: Record<string, number>
}

const { t } = useI18n()
const buildingVisual = gamePageVisuals.building
const playerStore = usePlayerStore()
const { getInventoryQuantity } = useInventorySummary()
const { normalizedStatusValue } = useAttributeStats()
const { submit } = useActionSubmit()
const selectedActorKey = ref("player:0")
const actorOptions = computed<Array<{ key: string; label: string; actorType: "player" | "kitty"; actorId: number }>>(() => {
  const options: Array<{ key: string; label: string; actorType: "player" | "kitty"; actorId: number }> = [
    { key: "player:0", label: t("game.executorPlayer"), actorType: "player", actorId: 0 }
  ]
  for (const kitty of playerStore.kitties || []) {
    options.push({
      key: `kitty:${kitty.id}`,
      label: `${t("game.executorKitty")}: ${kitty.name}`,
      actorType: "kitty",
      actorId: kitty.id
    })
  }
  return options
})
const selectedActor = computed(() => {
  return actorOptions.value.find((row) => row.key === selectedActorKey.value) || actorOptions.value[0]
})
const kittyExecutorHint = computed(() => {
  if (KITTY_STATUS_EXP_MULTIPLIER <= 0) {
    return t("game.executorKittyTooltipNoExp")
  }
  return t("game.executorKittyTooltip", {
    expMultiplier: KITTY_STATUS_EXP_MULTIPLIER.toFixed(2)
  })
})

watch(actorOptions, (options) => {
  if (!options.some((row) => row.key === selectedActorKey.value)) {
    selectedActorKey.value = "player:0"
  }
})

const buildingRows = computed(() => {
  const list = getCatalogBuildings(getGameDataChunk("buildings"))
  return list.map((building) => {
    const id = String(building.id || "").trim()
    const currentLevel = Math.max(0, Math.floor(Number(playerStore.buildings[id] || 0)))
    const maxLevel = Math.max(1, Math.floor(Number(building.maxLevel || 1)))
    const maxed = currentLevel >= maxLevel
    const nextLevel = Math.min(maxLevel, currentLevel + 1)
    const authoredLevel = building.levels?.find((entry) => entry.level === nextLevel)
    const nextDurationS = authoredLevel?.durationS ?? resolveCurveValue(building.baseDurationS, building.stepDurationS, currentLevel, 1)
    const nextCosts = authoredLevel?.costs ?? resolveCurveMap(building.baseCosts, building.stepCosts, currentLevel)
    const nextStatusRequirement = authoredLevel?.statusRequirement ?? resolveCurveMap(building.baseStatusRequirement, building.stepStatusRequirement, currentLevel)
    const statusEntries = Object.entries(nextStatusRequirement)
      .map(([attrKey, required]) => ({ attrKey, required: Math.max(0, Math.floor(required)) }))
      .filter((entry) => entry.required > 0)
    const costEntries = Object.entries(nextCosts)
      .map(([itemId, quantity]) => ({ itemId, quantity: Math.max(0, Math.floor(quantity)) }))
      .filter((entry) => entry.quantity > 0)

    const insufficientCosts = costEntries.filter((entry) => getInventoryQuantity(entry.itemId) < entry.quantity)
    const insufficientStatus = statusEntries.filter((entry) => {
      const current = Number(normalizedStatusValue.value[entry.attrKey] || 0)
      return current < entry.required
    })

    const nameKey = typeof building.nameKey === "string" ? building.nameKey : `building.${id}.name`
    const localizedName = t(nameKey)
    const name = localizedName === nameKey ? id : localizedName

    return {
      ...building,
      id,
      name,
      currentLevel,
      maxLevel,
      maxed,
      nextLevel,
      nextDurationS,
      nextBonuses: authoredLevel?.bonuses,
      statusEntries,
      costEntries,
      runDisabled: maxed || insufficientCosts.length > 0 || insufficientStatus.length > 0,
      queueDisabled: maxed,
      insufficientCosts,
      insufficientStatus
    }
  })
})

function getCatalogBuildings(catalog: unknown): BuildingDefinition[] {
  if (!catalog || typeof catalog !== "object") return []
  const rows = (catalog as { buildings?: unknown }).buildings
  if (!Array.isArray(rows)) return []
  return rows
    .filter((row): row is BuildingDefinition => !!row && typeof row === "object" && typeof (row as BuildingDefinition).id === "string")
    .sort((a, b) => Number(a.order || 0) - Number(b.order || 0))
}

function resolveCurveValue(baseRaw: number | undefined, stepRaw: number | undefined, level: number, minValue = 0) {
  const base = Number(baseRaw || 0)
  const step = Number(stepRaw || 0)
  const value = Math.floor(base + step * level)
  return Math.max(minValue, value)
}

function resolveActorDurationSeconds(baseSeconds: number) {
  return Math.max(1, Math.floor(Number(baseSeconds || 1)))
}

function resolveCurveMap(baseRaw?: Record<string, number>, stepRaw?: Record<string, number>, level = 0) {
  const out: Record<string, number> = {}
  for (const [key, value] of Object.entries(baseRaw || {})) {
    out[key] = Number(value || 0)
  }
  for (const [key, value] of Object.entries(stepRaw || {})) {
    out[key] = Number(out[key] || 0) + Number(value || 0) * level
  }
  for (const [key, value] of Object.entries(out)) {
    if (!Number.isFinite(value) || value <= 0) {
      delete out[key]
      continue
    }
    out[key] = Math.max(0, Math.floor(value))
  }
  return out
}

function runNow(buildingId: string) {
  const row = buildingRows.value.find((entry) => entry.id === buildingId)
  if (!row || row.runDisabled) {
    if (!row || row.maxed) {
      ElMessage.warning(t("game.buildingMaxed"))
      return
    }
    if (row.insufficientCosts.length > 0) {
      ElMessage.warning(t("game.error.actionInsufficientItems"))
      return
    }
    ElMessage.warning(t("game.error.statusRequirementNotMet"))
    return
  }
  submit("building_upgrade", "run", 1, { buildingId }, {
    actorType: selectedActor.value.actorType,
    actorId: selectedActor.value.actorId
  })
}

function enqueue(buildingId: string) {
  const row = buildingRows.value.find((entry) => entry.id === buildingId)
  if (!row || row.queueDisabled) {
    ElMessage.warning(t("game.buildingMaxed"))
    return
  }
  submit("building_upgrade", "queue", 1, { buildingId }, {
    actorType: selectedActor.value.actorType,
    actorId: selectedActor.value.actorId
  })
}
</script>

<style scoped>
.pond-bonus-preview { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 6px; padding-top: 4px; }
.pond-bonus-preview span { display: grid; gap: 1px; padding: 7px 8px; border: 1px solid var(--app-border); border-radius: 6px; color: var(--app-muted); font-size: 11px; }
.pond-bonus-preview b { color: var(--app-text); font-size: 13px; }
</style>
