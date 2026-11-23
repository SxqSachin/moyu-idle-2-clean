<template>
  <el-card shadow="hover" body-class="p-3!">
    <div class="space-y-2 text-sm">
      <div class="text-xs app-muted">#{{ slotIndex + 1 }}</div>
      <template v-if="item">
        <div class="flex items-center gap-2">
          <Card :item-id="item.itemId" hide-label />
          <div class="min-w-0">
            <div class="font-medium truncate">{{ item.label }}</div>
            <div class="text-xs app-muted">
              {{ t("game.battleItemBagInBagCount") }}: {{ item.bagQuantity }} ·
              {{ t("game.battleItemBagInventoryCount") }}: {{ item.inventoryQuantity }}
            </div>
          </div>
        </div>
        <div class="flex items-center gap-2 w-full">
          <el-input-number
            class="w-full!"
            :model-value="quantity"
            :min="1"
            :max="Math.max(1, item.inventoryQuantity)"
            size="small"
            :step="1"
            :disabled="disabled || item.inventoryQuantity <= 0"
            @update:model-value="$emit('update:quantity', $event)"
          >
            <template #suffix>
              <span class="text-xs app-muted pr-2">
                / {{ item.inventoryQuantity }}
              </span>
            </template>
          </el-input-number>
          <el-button
            size="small"
            type="success"
            class="shrink-0 w-1/3"
            :disabled="disabled || item.inventoryQuantity <= 0"
            @click="$emit('add')"
          >
            {{ t("game.battleItemBagAddMore") }}
          </el-button>
        </div>
        <div class="flex flex-col items-center gap-2 w-full">
          <el-button size="small" type="danger" class="flex-1 w-full" :disabled="disabled" @click="$emit('takeBack')">
            {{ t("game.battleItemBagTakeBack") }}
          </el-button>
          <el-button v-if="showRuleButton" size="small" type="primary" class="flex-1 w-full ml-0!" :disabled="disabled" @click="$emit('rule')">
            {{ t("game.battleItemBagRuleConfigTitle") }}
            <template v-if="ruleCount > 0">({{ ruleCount }})</template>
          </el-button>
        </div>
      </template>
      <template v-else>
        <div class="text-xs app-muted">{{ t("game.battleItemBagSlotEmpty") }}</div>
        <EmptyCard :label="t('game.battleItemBagChooseItem')" @click="$emit('select')" />
      </template>
    </div>
  </el-card>
</template>

<script setup lang="ts">
import { useI18n } from "vue-i18n"
import Card from "@components/game/item/Card.vue"
import EmptyCard from "@components/game/item/EmptyCard.vue"

defineProps<{
  slotIndex: number
  item: {
    itemId: string
    label: string
    bagQuantity: number
    inventoryQuantity: number
  } | null
  quantity: number
  ruleCount: number
  disabled: boolean
  showRuleButton?: boolean
}>()

defineEmits<{
  (e: "update:quantity", value: unknown): void
  (e: "select"): void
  (e: "add"): void
  (e: "takeBack"): void
  (e: "rule"): void
}>()

const { t } = useI18n()
</script>
