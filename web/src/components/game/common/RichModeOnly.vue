<template>
  <slot v-if="!store.playerSimpleMode" />
</template>

<script setup lang="ts">
import { useGameStore } from "@stores/game"

const store = useGameStore()
</script>
