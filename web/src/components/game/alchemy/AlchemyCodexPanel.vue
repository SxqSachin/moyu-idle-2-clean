<template>
  <div class="alchemy-codex">
    <section v-if="showFamilies" class="alchemy-codex__section">
      <div class="alchemy-codex__title">{{ t("alchemy.codex.families") }}</div>
      <div class="alchemy-codex__grid">
        <el-popover v-for="family in families" :key="family" placement="top" :width="340" trigger="hover">
          <template #reference>
            <button class="alchemy-codex__chip" type="button">
              <span>{{ t(`alchemy.family.${family}`, family) }}</span>
              <span class="alchemy-codex__chip-meta">{{ getAlchemyFamilyAttrs(family).length }}</span>
            </button>
          </template>
          <AlchemyFamilyPopoverContent :family="family" />
        </el-popover>
      </div>
    </section>

    <section class="alchemy-codex__section">
      <div class="alchemy-codex__title">{{ t("alchemy.codex.mediums") }}</div>
      <div class="alchemy-codex__cards">
        <article v-for="medium in mediums" :key="medium.id" class="alchemy-codex__card">
          <div class="alchemy-codex__medium-head">
            <Card :item-id="medium.itemId" :width="44" :height="44" hide-label />
            <div>
              <div class="alchemy-codex__card-title">{{ t(medium.nameKey) }}</div>
              <div class="alchemy-codex__card-item">{{ t(`item.${medium.itemId}.name`, medium.itemId) }}</div>
            </div>
          </div>
          <div class="alchemy-codex__card-desc">{{ t(medium.descKey) }}</div>
          <div class="alchemy-codex__stats">
            <span>{{ t("alchemy.codex.mediumConsume", { qty: medium.consumePerCraft }) }}</span>
            <span>{{ t("alchemy.codex.mediumLevel", { level: medium.minAlchemyLevel }) }}</span>
            <span>{{ t("alchemy.codex.mediumSlots", { slots: medium.modifiers.maxTraitSlots || 3 }) }}</span>
            <span v-for="line in mediumModifierLines(medium)" :key="`${medium.id}-${line}`">{{ line }}</span>
          </div>
          <div class="alchemy-codex__families">
            <el-popover v-for="(_, family) in medium.modifiers.familyAffinity" :key="`${medium.id}-${family}`" placement="top" :width="340" trigger="hover">
              <template #reference>
                <el-tag size="small" effect="plain">{{ t(`alchemy.family.${family}`, family) }}</el-tag>
              </template>
              <AlchemyFamilyPopoverContent :family="String(family)" />
            </el-popover>
          </div>
        </article>
      </div>
    </section>
  </div>
</template>

<script setup lang="ts">
import { computed } from "vue"
import { useI18n } from "vue-i18n"
import { listAlchemyMediums, type AlchemyMedium } from "@services/alchemy-data"
import { getAlchemyFamilyAttrs, listAlchemyFamilies } from "@services/alchemy-codex"
import AlchemyFamilyPopoverContent from "@components/game/alchemy/AlchemyFamilyPopoverContent.vue"
import Card from "@components/game/item/Card.vue"

const props = withDefaults(defineProps<{ showFamilies?: boolean }>(), {
  showFamilies: true
})
const { t } = useI18n()
const showFamilies = computed(() => props.showFamilies)
const families = computed(() => listAlchemyFamilies())
const mediums = computed(() => listAlchemyMediums())

function formatPercent(value: number): string {
  return `${(Number(value || 0) * 100).toFixed(2)}%`
}

function formatSignedPercent(value: number): string {
  const pct = Number(value || 0) * 100
  return `${pct >= 0 ? "+" : ""}${pct.toFixed(2)}%`
}

function mediumModifierLines(medium: AlchemyMedium): string[] {
  const lines: string[] = []
  if (medium.modifiers.valueMultiplier !== 1) {
    lines.push(t("alchemy.codex.mediumValue", { pct: formatSignedPercent(medium.modifiers.valueMultiplier - 1) }))
  }
  if (medium.modifiers.negativeMultiplier !== 1) {
    lines.push(t("alchemy.codex.mediumNegative", { pct: formatSignedPercent(medium.modifiers.negativeMultiplier - 1) }))
  }
  if (medium.modifiers.failChanceDelta !== 0) {
    lines.push(t("alchemy.codex.mediumFail", { pct: formatSignedPercent(medium.modifiers.failChanceDelta) }))
  }
  if (medium.modifiers.successFloor > 0) {
    lines.push(t("alchemy.codex.mediumFloor", { pct: formatPercent(medium.modifiers.successFloor) }))
  }
  if (!lines.length && !Object.keys(medium.modifiers.familyAffinity || {}).length) {
    lines.push(t("alchemy.codex.mediumNoExtra"))
  }
  return lines
}
</script>

<style scoped>
.alchemy-codex {
  display: grid;
  gap: 18px;
}

.alchemy-codex__section {
  display: grid;
  gap: 10px;
}

.alchemy-codex__title {
  font-size: 14px;
  font-weight: 800;
  color: var(--app-text);
}

.alchemy-codex__grid {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.alchemy-codex__chip {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  border: 1px solid var(--app-border);
  border-radius: 8px;
  background: var(--app-surface, transparent);
  color: var(--app-text);
  padding: 6px 9px;
  font-size: 13px;
  cursor: help;
}

.alchemy-codex__chip-meta {
  color: var(--app-muted);
  font-size: 12px;
}

.alchemy-codex__cards {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
  gap: 10px;
}

.alchemy-codex__card {
  display: grid;
  gap: 6px;
  border: 1px solid var(--app-border);
  border-radius: 8px;
  padding: 10px;
}

.alchemy-codex__medium-head {
  display: flex;
  align-items: center;
  gap: 10px;
  min-width: 0;
}

.alchemy-codex__card-title {
  font-size: 13px;
  font-weight: 800;
}

.alchemy-codex__card-item {
  color: var(--app-muted);
  font-size: 11px;
}

.alchemy-codex__card-desc {
  color: var(--app-muted);
  font-size: 12px;
  line-height: 1.45;
}

.alchemy-codex__stats {
  display: flex;
  flex-wrap: wrap;
  gap: 5px;
}

.alchemy-codex__stats span {
  border: 1px solid var(--app-border);
  border-radius: 6px;
  color: var(--app-text);
  background: var(--app-bg-mute, rgba(0, 0, 0, 0.03));
  padding: 3px 6px;
  font-size: 11px;
  line-height: 1.35;
}

.alchemy-codex__families {
  display: flex;
  flex-wrap: wrap;
  gap: 5px;
}
</style>
