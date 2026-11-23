<template>
  <LabeledFrameCard :hide-border="hideBorder" :label="hideLabel ? undefined : label">
    <div class="empty-card-mark">+</div>
  </LabeledFrameCard>
</template>

<script setup lang="ts">
import LabeledFrameCard from "@components/game/common/LabeledFrameCard.vue"

withDefaults(
  defineProps<{
    label?: string
    hideBorder?: boolean
    hideLabel?: boolean
  }>(),
  {
    label: "",
    hideBorder: false,
    hideLabel: false
  }
)
</script>

<style scoped>
.empty-card-mark {
  width: 30px;
  height: 30px;
  border: 1px dashed var(--el-border-color);
  border-radius: 999px;
  display: flex;
  align-items: center;
  justify-content: center;
  color: var(--el-text-color-secondary);
  font-size: 20px;
  line-height: 1;
}
</style>
