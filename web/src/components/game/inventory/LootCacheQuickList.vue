<template>
  <div class="space-y-2">
      <el-card v-for="entry in entries" :key="entry.id" class="app-card">
        <div class="flex items-center justify-between gap-3 text-sm">
          <div class="space-y-1">
            <div class="inline-flex items-center gap-2">
              <ItemIcon :item-id="entry.templateItemId" />
              <span>{{ itemLabel(entry.templateItemId) }}</span>
            </div>
            <div class="app-muted">{{ formatTime(entry.expiresAt) }}</div>
          </div>
        <el-button size="small" type="warning" @click="$emit('claim', entry.id)">
          {{ claimLabel }}
        </el-button>
      </div>
    </el-card>
    <div v-if="entries.length === 0" class="text-sm app-muted">{{ emptyText }}</div>
  </div>
</template>

<script setup lang="ts">
import type { LootCacheEntry } from "@components/game/inventory/types"
import ItemIcon from "@components/game/icon/ItemIcon.vue"

defineProps<{
  entries: LootCacheEntry[]
  emptyText: string
  claimLabel: string
  itemLabel: (itemID: string) => string
}>()

defineEmits<{
  (e: "claim", entryID: number): void
}>()

function formatTime(raw: string): string {
  return new Date(raw).toLocaleString()
}
</script>
