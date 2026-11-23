<template>
  <div class="page-shell md:p-10">
    <div class="mx-auto max-w-5xl space-y-6">
          <div class="space-y-1">
            <h1 class="text-2xl font-bold">{{ t("instructions.pageTitle") }}</h1>
            <p class="text-sm app-muted">{{ t("instructions.pageSubtitle") }}</p>
          </div>
        <InstructionsContent />
      <el-button @click="goHome">{{ t("instructions.backHome") }}</el-button>
    </div>
  </div>
</template>

<script setup lang="ts">
import "@/style.css"
import { useRouter } from "vue-router"
import { useI18n } from "vue-i18n"
import InstructionsContent from "@components/common/InstructionsContent.vue"

const router = useRouter()
const { t } = useI18n()

function goHome() {
  router.push("/")
}
</script>
