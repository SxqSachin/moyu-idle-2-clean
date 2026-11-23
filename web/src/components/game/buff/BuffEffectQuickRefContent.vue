<template>
  <div class="buff-effect-quick-ref-content">
    <div class="buff-effect-quick-ref-content__title">{{ titleText }}</div>
    <div class="buff-effect-quick-ref-content__desc">{{ t("game.buffEffectQuickRefDesc") }}</div>
    <div v-if="buffs.length > 0" class="buff-effect-quick-ref-content__list">
      <BuffQuickRef
        v-for="buff in buffs"
        :key="buff.id"
        :buff-id="buff.id"
        :remain-turns="-1"
      />
    </div>
    <div v-else class="buff-effect-quick-ref-content__empty">{{ t("game.buffEffectQuickRefEmpty") }}</div>
  </div>
</template>

<script setup lang="ts">
import { computed } from "vue"
import { useI18n } from "vue-i18n"
import BuffQuickRef from "@components/game/buff/BuffQuickRef.vue"
import { listBuffDefinitionsByEffectKind } from "@services/buff-definition"

const props = defineProps<{
  effectKind: string
}>()

const { t } = useI18n()
const normalizedEffectKind = computed(() => String(props.effectKind || "").trim())
const titleText = computed(() => {
  const key = `game.buffEffectQuickRefTitle.${normalizedEffectKind.value}`
  const localized = t(key)
  if (localized !== key) return localized
  const kindKey = `game.buffEffectKind_${normalizedEffectKind.value}`
  const kindLabel = t(kindKey)
  return kindLabel === kindKey ? normalizedEffectKind.value : kindLabel
})
const buffs = computed(() => listBuffDefinitionsByEffectKind(normalizedEffectKind.value))
</script>

<style scoped>
.buff-effect-quick-ref-content {
  display: grid;
  gap: 8px;
  max-width: 100%;
  font-size: 12px;
}

.buff-effect-quick-ref-content__title {
  font-weight: 700;
  color: var(--el-text-color-primary);
}

.buff-effect-quick-ref-content__desc,
.buff-effect-quick-ref-content__empty {
  color: var(--el-text-color-secondary);
  line-height: 1.45;
}

.buff-effect-quick-ref-content__list {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  max-width: 100%;
}
</style>
