<template>
  <span class="game-keyword-text">
    <template v-for="part in parts" :key="part.key">
      <span v-if="part.kind === 'text'">{{ part.text }}</span>
      <GameKeywordTag v-else :keyword="part.keyword" :label="part.text" />
    </template>
  </span>
</template>

<script setup lang="ts">
import { computed } from "vue"
import GameKeywordTag from "@components/game/common/GameKeywordTag.vue"
import { buildGameKeywordTextParts, type GameKeywordDisplay } from "@services/game-keyword"

const props = defineProps<{
  text: string
  keywords: GameKeywordDisplay[]
}>()

const parts = computed(() => buildGameKeywordTextParts(props.text, props.keywords))
</script>

<style scoped>
.game-keyword-text {
  overflow-wrap: anywhere;
}
</style>
