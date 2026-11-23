<template>

    <div v-if="baseAttrs">
      <div class="item-popover-label">{{ t("game.itemDetailEquipmentEffect") }}</div>
      <div class="flex flex-row gap-2">
        <template v-for="(val, attr) in baseAttrs">
          <StatusTag :attr-key="attr" :value="val"></StatusTag>
        </template>
      </div>
    </div>
</template>

<script setup lang="ts">
import { EquipmentItemInstance } from '@/types/equipment';
import { computed } from 'vue';
import { useI18n } from 'vue-i18n';
import StatusTag from '../attribute/StatusTag.vue';

const { t } = useI18n()


const props = defineProps<{
  baseAttrs: Record<string, number>
}>()



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