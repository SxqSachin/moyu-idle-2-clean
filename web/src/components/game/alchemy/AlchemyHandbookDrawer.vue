<template>
  <el-drawer v-model="visible" direction="rtl" size="360px" :title="t('alchemy.handbook.title')" append-to-body>
    <div class="alchemy-handbook">
      <AlchemyCodexPanel />
      <section>
        <h3>{{ t("alchemy.handbook.groups", { done: knownGroups.size, total: groups.length }) }}</h3>
        <div class="alchemy-handbook__list">
          <el-tag v-for="group in groups" :key="group.id" :type="knownGroups.has(group.id) ? 'success' : 'info'" effect="plain">
            {{ knownGroups.has(group.id) ? t(group.nameKey) : t("alchemy.handbook.unknown", { index: groupIndex(group.id) }) }}
          </el-tag>
        </div>
      </section>

      <section>
        <h3>{{ t("alchemy.handbook.traits", { done: knownTraits.size, total: traits.length }) }}</h3>
        <div class="alchemy-handbook__list">
          <el-tag v-for="trait in traits" :key="trait.id" :type="knownTraits.has(trait.id) ? 'success' : 'info'" effect="plain">
            {{ knownTraits.has(trait.id) ? t(trait.nameKey) : t("alchemy.handbook.unknown", { index: traitIndex(trait.id) }) }}
          </el-tag>
        </div>
      </section>

      <section>
        <h3>{{ t("alchemy.handbook.combos", { done: knownCombos.size, total: combos.length }) }}</h3>
        <div class="alchemy-handbook__list">
          <button
            v-for="combo in combos"
            :key="combo.id"
            class="alchemy-handbook__combo"
            :class="{ 'alchemy-handbook__combo--known': knownCombos.has(combo.id) }"
            type="button"
            :disabled="!knownCombos.has(combo.id)"
            @click="selectCombo(combo.id)"
          >
            <strong>{{ knownCombos.has(combo.id) ? t(combo.nameKey) : t("alchemy.handbook.unknown", { index: comboIndex(combo.id) }) }}</strong>
            <small v-if="knownCombos.has(combo.id)">{{ comboInputsText(combo) }}</small>
          </button>
        </div>
      </section>
    </div>
  </el-drawer>
</template>

<script setup lang="ts">
import { computed } from "vue"
import { useI18n } from "vue-i18n"
import {
  listAlchemyComboRecipes,
  listAlchemyGroups,
  listAlchemyTraits,
  type AlchemyDiscoveries
} from "@services/alchemy-data"
import AlchemyCodexPanel from "@components/game/alchemy/AlchemyCodexPanel.vue"

const visible = defineModel<boolean>({ default: false })
const props = withDefaults(defineProps<{ discoveries?: AlchemyDiscoveries | null }>(), { discoveries: null })
const emit = defineEmits<{
  (e: "select-combo", comboId: string): void
}>()

const { t } = useI18n()

const groups = listAlchemyGroups()
const traits = listAlchemyTraits()
const combos = listAlchemyComboRecipes()
const knownGroups = computed(() => new Set(props.discoveries?.groups || []))
const knownTraits = computed(() => new Set(props.discoveries?.traits || []))
const knownCombos = computed(() => new Set(props.discoveries?.combos || []))

function groupIndex(id: string): number {
  return groups.findIndex((row) => row.id === id) + 1
}

function traitIndex(id: string): number {
  return traits.findIndex((row) => row.id === id) + 1
}

function comboIndex(id: string): number {
  return combos.findIndex((row) => row.id === id) + 1
}

function comboInputsText(combo: { inputs: Record<string, number> }): string {
  return Object.entries(combo.inputs)
    .map(([itemId, qty]) => `${t(`item.${itemId}.name`, itemId)} x${qty}`)
    .join(" / ")
}

function selectCombo(comboId: string) {
  emit("select-combo", comboId)
}
</script>

<style scoped>
.alchemy-handbook {
  display: grid;
  gap: 18px;
}

.alchemy-handbook section {
  display: grid;
  gap: 8px;
}

.alchemy-handbook h3 {
  margin: 0;
  font-size: 14px;
  font-weight: 700;
  color: var(--app-text);
}

.alchemy-handbook__list {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}

.alchemy-handbook__combo {
  display: grid;
  gap: 2px;
  min-width: 0;
  max-width: 100%;
  border: 1px solid var(--app-border);
  border-radius: 8px;
  background: var(--app-surface, transparent);
  color: var(--app-muted);
  padding: 7px 9px;
  text-align: left;
}

.alchemy-handbook__combo--known {
  color: var(--el-color-success);
  cursor: pointer;
}

.alchemy-handbook__combo:disabled {
  cursor: not-allowed;
  opacity: 0.68;
}

.alchemy-handbook__combo strong {
  font-size: 12px;
}

.alchemy-handbook__combo small {
  color: var(--app-muted);
  font-size: 11px;
  line-height: 1.35;
}
</style>
