<template>
  <div class="space-y-1">
    <div v-if="!items.length" class="py-2 text-xs text-[var(--app-muted)]">
      {{ t("wardrobe.empty") }}
    </div>
    <div
      v-for="item in items"
      :key="item.id"
      class="flex cursor-pointer items-center justify-between rounded-md border px-2 py-1.5 text-xs transition-colors"
      :class="[
        item.id === equippedId
          ? 'border-[var(--app-accent)] bg-[var(--app-accent-bg,rgba(59,130,246,0.08))] font-semibold'
          : 'border-[var(--app-border)] hover:border-[var(--app-accent)]/50',
        !item.owned ? 'opacity-50' : ''
      ]"
      @click="item.owned && item.id !== equippedId && $emit('select', item.id)"
    >
      <TitleQuickRef :category="category" :item-id="item.id" />
      <div class="flex items-center gap-1">
        <span v-if="item.id === equippedId" class="text-[10px] text-[var(--app-accent)]">
          {{ t("wardrobe.equipped") }}
        </span>
        <el-button
          v-if="item.id === equippedId"
          size="small"
          text
          type="danger"
          @click.stop="$emit('unequip')"
        >
          {{ t("wardrobe.unequip") }}
        </el-button>
        <span v-if="!item.owned" class="text-[10px] text-[var(--app-muted)]">
          {{ t("wardrobe.locked") }}
        </span>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from "vue-i18n"
import TitleQuickRef from "@components/game/wardrobe/TitleQuickRef.vue"

type TitleItem = {
  id: string
  nameKey: string
  rarity: string
  label: string
  owned: boolean
}

defineProps<{
  items: TitleItem[]
  equippedId: string
  category: "title_prefix" | "title_suffix"
}>()

defineEmits<{
  select: [id: string]
  unequip: []
}>()

const { t } = useI18n()

</script>
