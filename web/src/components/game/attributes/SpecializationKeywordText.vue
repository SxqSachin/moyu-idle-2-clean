<template>
  <span class="specialization-keyword-text">
    <template v-for="part in parts" :key="part.key">
      <span v-if="part.kind === 'text'">{{ part.text }}</span>
      <span
        v-else-if="part.keyword.buffIds.length > 0"
        class="specialization-keyword-text__buffs"
        :aria-label="part.keyword.name"
      >
        <BuffQuickRef
          v-for="buffId in part.keyword.buffIds"
          :key="`${part.key}-${buffId}`"
          :buff-id="buffId"
        />
      </span>
      <GameKeywordTag v-else :keyword="part.keyword" :label="part.text" />
    </template>
  </span>
</template>

<script setup lang="ts">
import { computed } from "vue"
import BuffQuickRef from "@components/game/buff/BuffQuickRef.vue"
import GameKeywordTag from "@components/game/common/GameKeywordTag.vue"
import {
  buildSpecializationKeywordTextParts,
  type ResolvedSpecializationKeyword
} from "@services/specialization-keywords"

const props = defineProps<{
  text: string
  keywords: ResolvedSpecializationKeyword[]
}>()

const parts = computed(() => buildSpecializationKeywordTextParts(props.text, props.keywords))
</script>

<style scoped>
.specialization-keyword-text {
  overflow-wrap: anywhere;
}

.specialization-keyword-text__buffs {
  display: inline-flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 4px;
  margin: 0 2px;
  vertical-align: middle;
}
</style>
