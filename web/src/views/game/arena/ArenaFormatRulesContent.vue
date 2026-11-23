<template>
  <article class="arena-format-rule-detail">
    <header class="arena-format-rule-detail__header">
      <span class="arena-format-rule-detail__icon"><Icon :icon="normalizedFormat.icon" /></span>
      <div>
        <strong>{{ formatName }}</strong>
        <small>{{ t("game.arena.formatRuleDetails.subtitle") }}</small>
      </div>
    </header>

    <p class="arena-format-rule-detail__overview">{{ formatDescription }}</p>

    <div v-if="ruleViews.length > 0" class="arena-format-rule-detail__rules">
      <section v-for="rule in ruleViews" :key="rule.key" class="arena-format-rule-detail__rule">
        <header class="arena-format-rule-detail__event">
          <div>
            <small>{{ t("game.arena.formatRuleDetails.trigger") }}</small>
            <strong>{{ rule.event }}</strong>
          </div>
          <p><Icon icon="lucide:scan-search" />{{ rule.target }}</p>
        </header>
        <div class="arena-format-rule-detail__body">
          <div class="arena-format-rule-detail__effects">
            <Icon icon="lucide:sparkles" />
            <div>
              <span v-for="line in rule.effects" :key="line.key">{{ line.text }}</span>
            </div>
          </div>
          <p v-if="rule.settlement.length > 0" class="arena-format-rule-detail__settlement">
            <Icon icon="lucide:list-checks" />
            <span v-for="line in rule.settlement" :key="line.key">{{ line.text }}</span>
          </p>
        </div>
      </section>
    </div>

    <div v-else class="arena-format-rule-detail__empty">
      <Icon icon="lucide:file-question" />
      <span>{{ t("game.arena.formatRuleDetails.empty") }}</span>
    </div>
  </article>
</template>

<script setup lang="ts">
import { computed } from "vue"
import { Icon } from "@iconify/vue"
import { useI18n } from "vue-i18n"
import type { ArenaFormat } from "@services/arena"
import { buildArenaFormatRuleViews } from "@services/arena-format-display"

const props = defineProps<{
  format: ArenaFormat
}>()

const { t } = useI18n()

const normalizedFormat = computed<ArenaFormat>(() => props.format)
const formatName = computed(() => t(normalizedFormat.value.nameKey))
const formatDescription = computed(() => t(normalizedFormat.value.descKey))
const ruleViews = computed(() => buildArenaFormatRuleViews(normalizedFormat.value.rules, t))
</script>

<style scoped>
.arena-format-rule-detail {
  min-width: 0;
  display: grid;
  gap: 10px;
  color: var(--el-text-color-primary);
}

.arena-format-rule-detail__header {
  min-width: 0;
  display: flex;
  align-items: center;
  gap: 9px;
}

.arena-format-rule-detail__icon {
  width: 34px;
  height: 34px;
  display: grid;
  place-items: center;
  flex: none;
  border: 1px solid color-mix(in srgb, #c4740a 28%, var(--el-border-color));
  border-radius: 4px;
  color: #b86600;
  background: color-mix(in srgb, #f59e0b 9%, var(--el-bg-color));
}

.arena-format-rule-detail__header > div {
  min-width: 0;
  display: grid;
  gap: 2px;
}

.arena-format-rule-detail__header strong {
  font-size: 0.86rem;
}

.arena-format-rule-detail__header small,
.arena-format-rule-detail__overview {
  color: var(--app-muted);
  font-size: 0.7rem;
  line-height: 1.55;
}

.arena-format-rule-detail__overview {
  margin: 0;
}

.arena-format-rule-detail__rules {
  display: grid;
  gap: 8px;
}

.arena-format-rule-detail__rule {
  overflow: hidden;
  border: 1px solid var(--el-border-color-lighter);
  border-left: 3px solid #d48212;
  border-radius: 4px;
  background: var(--el-bg-color);
}

.arena-format-rule-detail__event {
  min-height: 36px;
  display: grid;
  grid-template-columns: minmax(112px, auto) minmax(0, 1fr);
  align-items: center;
  gap: 12px;
  padding: 6px 9px;
  border-bottom: 1px solid var(--el-border-color-extra-light);
  background: color-mix(in srgb, #f59e0b 6%, var(--el-bg-color));
}

.arena-format-rule-detail__event > div {
  min-width: 0;
  display: grid;
  gap: 1px;
}

.arena-format-rule-detail__event small {
  color: var(--app-muted);
  font-size: 0.61rem;
}

.arena-format-rule-detail__event strong {
  font-size: 0.72rem;
}

.arena-format-rule-detail__event p {
  min-width: 0;
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 5px;
  margin: 0;
  color: var(--app-muted);
  font-size: 0.65rem;
  line-height: 1.4;
  text-align: right;
}

.arena-format-rule-detail__event svg {
  flex: none;
  color: #b86600;
  font-size: 0.72rem;
}

.arena-format-rule-detail__body {
  display: grid;
  gap: 6px;
  padding: 8px 9px;
}

.arena-format-rule-detail__effects {
  min-width: 0;
  display: grid;
  grid-template-columns: 16px minmax(0, 1fr);
  gap: 6px;
}

.arena-format-rule-detail__effects > svg,
.arena-format-rule-detail__settlement > svg {
  margin-top: 1px;
  flex: none;
  color: #b86600;
  font-size: 0.76rem;
}

.arena-format-rule-detail__effects > div {
  min-width: 0;
  display: grid;
  gap: 2px;
  font-size: 0.7rem;
  line-height: 1.5;
}

.arena-format-rule-detail__settlement {
  min-width: 0;
  display: grid;
  grid-template-columns: 16px minmax(0, 1fr);
  gap: 6px;
  margin: 0;
  padding-top: 5px;
  border-top: 1px dashed var(--el-border-color-extra-light);
  color: var(--app-muted);
  font-size: 0.62rem;
  line-height: 1.45;
}

.arena-format-rule-detail__empty {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 9px;
  border-block: 1px solid var(--el-border-color-lighter);
  color: var(--app-muted);
  font-size: 0.7rem;
  line-height: 1.5;
}

.arena-format-rule-detail__empty svg {
  flex: none;
  color: #b86600;
}

@media (max-width: 520px) {
  .arena-format-rule-detail__event {
    grid-template-columns: 1fr;
    gap: 3px;
  }

  .arena-format-rule-detail__event p {
    justify-content: flex-start;
    text-align: left;
  }
}
</style>
