<template>
  <el-dialog
    v-bind="attrs"
    append-to-body
    :modal-class="mergedModalClass"
  >
    <template v-for="(_, name) in $slots" #[name]="slotProps">
      <slot :name="name" v-bind="slotProps || {}" />
    </template>
  </el-dialog>
</template>

<script setup lang="ts">
import { computed, useAttrs } from "vue"

defineOptions({
  inheritAttrs: false
})

const props = defineProps<{
  modalClass?: string
}>()

const attrs = useAttrs()

const mergedModalClass = computed(() => {
  const classes = new Set(["game-dialog-overlay"])
  for (const className of (props.modalClass || "").split(/\s+/)) {
    const normalized = className.trim()
    if (normalized) classes.add(normalized)
  }
  return Array.from(classes).join(" ")
})
</script>
