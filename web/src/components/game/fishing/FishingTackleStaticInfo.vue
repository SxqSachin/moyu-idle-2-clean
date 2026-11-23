<template>
  <section v-if="definition" class="fishing-tackle-info">
    <header class="fishing-tackle-info__header">
      <span>{{ t("game.fishing.tackleStats") }}</span>
      <strong>{{ slotLabel }}</strong>
    </header>

    <div class="fishing-tackle-info__methods">
      <span>{{ t("game.fishing.tackleSuitableMethods") }}</span>
      <strong>{{ methodLabels }}</strong>
    </div>

    <div v-if="attributeEntries.length" class="fishing-tackle-info__attributes">
      <StatusTag
        v-for="entry in attributeEntries"
        :key="entry.attrKey"
        :attr-key="entry.attrKey"
        :value="entry.value"
        signed
      />
    </div>

    <dl v-if="metricEntries.length" class="fishing-tackle-info__metrics">
      <div v-for="entry in metricEntries" :key="entry.key">
        <dt>
          {{ entry.label }}
          <el-tooltip v-if="entry.hint" :content="entry.hint" placement="top">
            <button type="button" class="fishing-tackle-info__help" :aria-label="entry.hint">
              <Icon icon="lucide:circle-help" aria-hidden="true" />
            </button>
          </el-tooltip>
        </dt>
        <dd :class="entry.tone ? `is-${entry.tone}` : ''">{{ entry.value }}</dd>
      </div>
    </dl>

    <div v-if="preferenceEntries.length" class="fishing-tackle-info__preferences">
      <span>{{ t("game.fishing.tacklePreferences") }}</span>
      <div>
        <b
          v-for="entry in preferenceEntries"
          :key="entry.key"
          :class="entry.value < 0 ? 'is-negative' : 'is-positive'"
        >
          {{ entry.label }} {{ formatSignedPercent(entry.value) }}
        </b>
      </div>
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed } from "vue"
import { Icon } from "@iconify/vue"
import { useI18n } from "vue-i18n"
import StatusTag from "@components/game/attribute/StatusTag.vue"
import { getGameDataTable } from "@services/game-data-runtime"
import { getLocalizedFishingTackleSecondaryCategoryLabel } from "@services/fishing-tackle-grouping"
import {
  getFishSpeciesDisplay,
  getFishingWaterDisplay,
  localizedFishText
} from "@services/fish-instance-display"
import type { FishingMethod, FishingTackleDefinition } from "@services/fishing"

const props = defineProps<{
  itemId: string
  effectiveAttrs?: Record<string, number>
}>()

const { t, locale } = useI18n()

const definition = computed(() =>
  getGameDataTable<FishingTackleDefinition>("fishing_tackle")
    .find((row) => row.itemId === props.itemId) || null
)

const slotLabel = computed(() => {
  const row = definition.value
  if (!row) return ""
  return getLocalizedFishingTackleSecondaryCategoryLabel(t, row)
})

const methodLabels = computed(() =>
  (definition.value?.methods || []).map(methodLabel).join(" / ")
)

const attributeEntries = computed(() => {
  const row = definition.value
  if (!row) return []
  const candidates: Array<{ attrKey: string; fallback?: number }> = [
    { attrKey: "fishing_control", fallback: row.control },
    { attrKey: "fishing_sensitivity", fallback: row.sensitivity },
    { attrKey: "fishing_efficiency", fallback: row.efficiency },
    { attrKey: "fishing_power" },
    { attrKey: "fishing_casting", fallback: row.casting },
    { attrKey: "fishing_hooking", fallback: row.hooking }
  ]
  return candidates.flatMap((entry) => {
    const effectiveValue = props.effectiveAttrs?.[entry.attrKey]
    const value = Number.isFinite(Number(effectiveValue)) ? Number(effectiveValue) : Number(entry.fallback || 0)
    return Number.isFinite(value) && value !== 0 ? [{ attrKey: entry.attrKey, value }] : []
  })
})

const metricEntries = computed(() => {
  const row = definition.value
  if (!row) return []
  const entries: Array<{ key: string; label: string; value: string; hint?: string; tone?: "positive" | "negative" }> = []
  if (Number(row.maxLoadKg) > 0) {
    entries.push({ key: "load", label: t("game.fishing.tackleMaxLoad"), value: `${formatNumber(row.maxLoadKg)} kg`, hint: t("game.fishing.tackleMaxLoadHint") })
  }
  if (Number.isFinite(Number(row.biteMultiplier)) && Number(row.biteMultiplier) !== 1) {
    const delta = Number(row.biteMultiplier) - 1
    entries.push({
      key: "bite",
      label: t("game.fishing.tackleBiteRate"),
      value: formatSignedPercent(delta),
      tone: delta < 0 ? "negative" : "positive"
    })
  }
  if (Number(row.mutationChanceMultiplier) > 1) {
    entries.push({
      key: "special-fish",
      label: t("game.fishing.tackleSpecialFishChance"),
      value: t("game.fishing.tackleSpecialFishChanceHigher"),
      hint: t("game.fishing.tackleSpecialFishChanceHint"),
      tone: "positive"
    })
  }
  if (Number(row.wearPerFight) > 0) {
    entries.push({
      key: "wear",
      label: t("game.fishing.tackleBaseWear"),
      value: formatUnsignedPercent(Number(row.wearPerFight))
    })
  }
  return entries
})

const preferenceEntries = computed(() => {
  const row = definition.value
  if (!row) return []
  const entries: Array<{ key: string; label: string; value: number }> = []
  for (const [waterId, rawValue] of Object.entries(row.waterBiteModifiers || {})) {
    const value = Number(rawValue)
    if (!Number.isFinite(value) || value === 0) continue
    const water = getFishingWaterDisplay(waterId)
    const name = localizedFishText(water?.names, locale.value, waterId)
    entries.push({ key: `water-${waterId}`, label: t("game.fishing.tackleWaterPreference", { name }), value })
  }
  for (const [speciesId, rawValue] of Object.entries(row.speciesWeightModifiers || {})) {
    const value = Number(rawValue)
    if (!Number.isFinite(value) || value === 0) continue
    const species = getFishSpeciesDisplay(speciesId)
    const name = localizedFishText(species?.names, locale.value, speciesId)
    entries.push({ key: `species-${speciesId}`, label: t("game.fishing.tackleSpeciesPreference", { name }), value })
  }
  return entries
})

function methodLabel(method: FishingMethod) {
  return t(`game.fishing.method_${method}`)
}

function formatNumber(value: number) {
  return Number(value).toFixed(2).replace(/\.00$/, "").replace(/(\.\d)0$/, "$1")
}

function formatSignedPercent(value: number) {
  const percent = Math.round(Number(value) * 1000) / 10
  return `${percent > 0 ? "+" : ""}${formatNumber(percent)}%`
}

function formatUnsignedPercent(value: number) {
  return `${formatNumber(Math.round(Number(value) * 1000) / 10)}%`
}
</script>

<style scoped>
.fishing-tackle-info {
  display: grid;
  gap: 8px;
  padding-block: 10px;
  border-block: 1px solid var(--app-border);
}

.fishing-tackle-info__header,
.fishing-tackle-info__methods {
  display: flex;
  min-width: 0;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.fishing-tackle-info__header span,
.fishing-tackle-info__methods span,
.fishing-tackle-info__preferences > span {
  color: var(--app-muted);
  font-size: 12px;
}

.fishing-tackle-info__header strong {
  color: var(--el-color-primary);
  font-size: 12px;
}

.fishing-tackle-info__methods strong {
  min-width: 0;
  color: var(--app-text);
  font-size: 12px;
  text-align: right;
  overflow-wrap: anywhere;
}

.fishing-tackle-info__attributes,
.fishing-tackle-info__preferences > div {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}

.fishing-tackle-info__metrics {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 1px;
  margin: 0;
  border: 1px solid var(--app-border);
  background: var(--app-border);
}

.fishing-tackle-info__metrics > div {
  display: flex;
  min-width: 0;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  padding: 6px 8px;
  background: var(--app-surface);
  font-size: 12px;
}

.fishing-tackle-info__metrics > div:last-child:nth-child(odd) {
  grid-column: 1 / -1;
}

.fishing-tackle-info__metrics dt {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  color: var(--app-muted);
}

.fishing-tackle-info__help {
  display: grid;
  place-items: center;
  flex: none;
  width: 18px;
  height: 18px;
  padding: 0;
  border: 0;
  background: transparent;
  color: inherit;
  cursor: help;
}

.fishing-tackle-info__help svg {
  font-size: 13px;
}

.fishing-tackle-info__metrics dd {
  margin: 0;
  color: var(--app-text);
  font-weight: 700;
}

.fishing-tackle-info__metrics dd.is-positive,
.fishing-tackle-info__preferences b.is-positive {
  color: var(--el-color-success);
}

.fishing-tackle-info__metrics dd.is-negative,
.fishing-tackle-info__preferences b.is-negative {
  color: var(--el-color-danger);
}

.fishing-tackle-info__preferences {
  display: grid;
  gap: 5px;
}

.fishing-tackle-info__preferences b {
  display: inline-flex;
  min-height: 24px;
  align-items: center;
  padding: 2px 7px;
  border: 1px solid currentColor;
  border-radius: 4px;
  font-size: 11px;
  font-weight: 650;
}
</style>
