<template>
  <el-popover
    placement="top-start"
    :width="400"
    trigger="hover"
    :show-after="160"
    :hide-after="100"
    :persistent="false"
    :popper-style="{ maxWidth: 'calc(100vw - 24px)' }"
    popper-class="arena-format-rules-popper"
  >
    <template #reference><slot /></template>
    <ArenaFormatRulesContent :format="format" />
  </el-popover>
</template>

<script setup lang="ts">
import type { ArenaFormat } from "@services/arena"
import ArenaFormatRulesContent from "./ArenaFormatRulesContent.vue"

defineProps<{
  format: ArenaFormat
}>()
</script>
