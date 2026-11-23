<template>
  <div class="page-shell space-y-4">
    <el-card class="app-card">
      <template #header>
        <div class="flex items-center justify-between gap-2">
          <div class="font-bold">{{ title }}</div>
          <div class="flex gap-2">
            <el-button size="small" @click="$emit('back')">{{ t("game.backToWarehouse") }}</el-button>
            <el-button size="small" :disabled="!targetLabel" @click="$emit('refresh')">{{ t("game.refresh") }}</el-button>
          </div>
        </div>
      </template>

      <el-alert
        v-if="noticeTitle"
        class="mb-3"
        :title="noticeTitle"
        :type="noticeType"
        show-icon
        @close="$emit('notice-close')"
      />

      <div v-if="!targetLabel" class="text-sm app-muted">{{ t("game.equipmentOperationTargetMissing") }}</div>

      <div v-else v-loading="loading" class="space-y-3">
        <div class="text-sm">{{ t("game.itemDetailName") }}: <span class="font-medium">{{ targetLabel }}</span></div>
        <slot />
      </div>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from "vue-i18n"

withDefaults(defineProps<{
  title: string
  loading: boolean
  targetLabel: string
  noticeTitle?: string
  noticeType?: "success" | "warning" | "info" | "error"
}>(), {
  noticeTitle: "",
  noticeType: "warning"
})

defineEmits<{
  (e: "refresh"): void
  (e: "back"): void
  (e: "notice-close"): void
}>()

const { t } = useI18n()
</script>
