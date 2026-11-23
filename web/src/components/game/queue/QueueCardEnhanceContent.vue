<template>
  <div class="flex flex-col gap-2 text-xs mt-2">
    <div class="flex flex-wrap items-center gap-2">
      <span class="text-gray-400">{{ t("game.queueEnhanceTargetLevel") }}:</span>
      <span class="font-medium">+{{ enhance.targetEnhanceLevel }}</span>
      <span v-if="enhance.useProtect" class="text-gray-400">
        {{ t("game.queueEnhanceProtect") }}: {{ t("game.yes") }}
      </span>
      <QuickRef v-if="enhance.useProtect && enhance.protectItemId" :item-id="enhance.protectItemId" />
    </div>

    <div class="flex flex-row items-center gap-2">
      <span class="text-gray-400">{{ t('game.queueEnhanceItemInstance')  }}:</span>
      <Card :item-id="enhance.templateItemId" :instance-id="enhance.equipmentInstanceId"></Card>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from "vue-i18n"
import QuickRef from "@components/game/item/QuickRef.vue"
import Card from "../item/Card.vue";
import type { QueueCardDisplayEnhance } from "@components/game/queue/types"

defineProps<{
  enhance: QueueCardDisplayEnhance
}>()

const { t } = useI18n()
</script>
