<template>
    <div v-if="instance">
      <div class="item-popover-label">{{ t("game.itemDetailInstanceState") }}</div>
      <div class="flex flex-col">
        <div>
            <span class="item-popover-value">
                {{ t('game.enhanceLevel') }}:
            </span>
            <span class="item-popover-value">
                Lv{{ instance.enhanceLevel || 0 }}
            </span>
        </div>
        <div>
            <span class="item-popover-value">
                {{ t('game.durabilityLabel') }}:
            </span>
            <span class="item-popover-value">
                {{ instance.durability || 0 }} / {{ instance.maxDurability || 0 }}
            </span>
        </div>
        <div>
        </div>
      </div>
    </div>
</template>

<script setup lang="ts">
import { EquipmentItemInstance } from '@/types/equipment';
import { computed } from 'vue';
import { useI18n } from 'vue-i18n';

const { t } = useI18n()


const props = defineProps<{
    instance: EquipmentItemInstance
}>()


const itemStateLabel = computed(() => {
  const state = props.instance.state
  if (!state) return t("game.itemDetailUnknown")
  const key = `game.itemState.${state}`
  const localized = t(key)
  return localized === key ? state : localized
})


</script>

<style scoped>
@reference "#app-style";

.item-popover-label {
  @apply text-xs text-gray-500 dark:text-gray-400;
}

.item-popover-value {
  @apply text-xs text-gray-700 dark:text-gray-300;
  white-space: pre-wrap;
  word-break: break-word;
}
</style>