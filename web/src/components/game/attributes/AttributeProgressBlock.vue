<template>
  <template v-if="growthType !== 'static'">
    <div class="text-xs app-muted">{{ levelLabel }} {{ level }}</div>
    <el-progress :percentage="progress" :stroke-width="8" />
    <div class="text-xs app-muted">{{ expText }} / {{ nextExpText }}</div>
  </template>
</template>

<script setup lang="ts">
import { computed } from "vue"
import { formatDecimal } from "@utils/number-format"

const props = defineProps<{
  growthType: string
  levelLabel: string
  level: number
  progress: number
  exp: number
  nextExp: number
}>()

const expText = computed(() => formatDecimal(Number(props.exp || 0), 2))
const nextExpText = computed(() => formatDecimal(Number(props.nextExp || 0), 2))

</script>
