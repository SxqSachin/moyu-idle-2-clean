<template>
  <el-table :data="orders" size="small" border>
    <el-table-column :label="t('game.marketQuantity')" width="120">
      <template #default="{ row }">
        {{ row.remainingQuantity }} / {{ row.quantity }}
      </template>
    </el-table-column>
    <el-table-column :label="t('game.marketPrice')" width="110">
      <template #default="{ row }">
        {{ formatNumberWithThousands(row.price) }}
      </template>
    </el-table-column>
    <el-table-column :label="t('game.marketAction')" min-width="110">
      <template #default="{ row, $index }">
        <div v-if="row.status === 'active' && row.buyerId !== userId" class="flex items-center">
          <el-button
            v-if="bestFillOrderId > 0 && row.id === bestFillOrderId"
            size="small"
            type="success"
            :disabled="inventorySellableForBuyOrder(row.itemId) <= 0"
            @click="$emit('fill', row.id)"
          >
            {{ t("game.marketSellToBuyOrder") }}
          </el-button>
          <el-button
            v-else-if="bestFillOrderId > 0 && $index > 0"
            size="small"
            text
            :disabled="inventorySellableForBuyOrder(row.itemId) <= 0"
            @click="$emit('fill', bestFillOrderId)"
          >
            ↑
          </el-button>
          <span v-else class="app-muted">-</span>
        </div>
        <el-button v-else-if="row.status === 'active' && row.buyerId === userId" size="small" type="warning" @click="$emit('cancel', row.id)">
          {{ t("game.marketCancelBuyOrder") }}
        </el-button>
        <span v-else class="app-muted">-</span>
      </template>
    </el-table-column>
  </el-table>
</template>

<script setup lang="ts">
import { computed } from "vue"
import { useI18n } from "vue-i18n"
import { formatNumberWithThousands } from "@utils/number-format"

type BuyOrder = {
  id: number
  buyerId: number
  itemId: string
  quantity: number
  remainingQuantity: number
  price: number
  status: string
}

const props = defineProps<{
  orders: BuyOrder[]
  userId: number
  inventorySellableForBuyOrder: (itemId: string) => number
}>()

defineEmits<{
  (e: "fill", orderId: number): void
  (e: "cancel", orderId: number): void
}>()

const { t } = useI18n()
const bestFillOrderId = computed(() => {
  const first = props.orders.find((row) => row.status === "active" && row.buyerId !== props.userId)
  return first?.id || 0
})
</script>
