<template>
  <el-table :data="listings" size="small" border>
    <el-table-column :label="t('game.marketQuantity')" prop="quantity" width="90" />
    <el-table-column :label="t('game.marketPrice')" width="110">
      <template #default="{ row }">
        {{ formatNumberWithThousands(row.price) }}
      </template>
    </el-table-column>
    <el-table-column :label="t('game.marketAction')" min-width="110">
      <template #default="{ row, $index }">
        <div v-if="row.status === 'active' && row.sellerId !== userId" class="flex items-center">
          <el-button
            v-if="bestBuyListingId > 0 && row.id === bestBuyListingId"
            size="small"
            type="success"
            @click="$emit('buy', row.id)"
          >
            {{ t("game.marketBuyAction") }}
          </el-button>
          <el-button
            v-else-if="bestBuyListingId > 0 && $index > 0"
            size="small"
            text
            @click="$emit('buy', bestBuyListingId)"
          >
            ↑
          </el-button>
          <span v-else class="app-muted">-</span>
        </div>
        <el-button v-else-if="row.status === 'active' && row.sellerId === userId" size="small" type="warning" @click="$emit('cancel', row.id)">
          {{ t("game.marketCancelListingAction") }}
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

type Listing = {
  id: number
  sellerId: number
  quantity: number
  price: number
  status: string
}

defineEmits<{
  (e: "buy", listingId: number): void
  (e: "cancel", listingId: number): void
}>()

const { t } = useI18n()
const props = defineProps<{
  listings: Listing[]
  userId: number
}>()
const bestBuyListingId = computed(() => {
  const first = props.listings.find((row) => row.status === "active" && row.sellerId !== props.userId)
  return first?.id || 0
})
</script>
