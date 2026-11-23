<template>
  <el-card class="admin-card">
    <template #header><div class="font-semibold">广播发布 / Release Announce</div></template>
    <div class="grid gap-3 md:grid-cols-2">
      <el-input v-model="form.releaseId" placeholder="releaseId" />
      <el-select v-model="form.kind">
        <el-option label="soft" value="soft" />
        <el-option label="force" value="force" />
      </el-select>
      <el-input v-model="form.messageI18nKey" placeholder="messageI18nKey" />
      <el-input-number v-model="form.recommendedDelaySec" :min="1" :max="900" />
      <el-input v-model="form.minAcceptedBuild" placeholder="minAcceptedBuild" />
      <el-input-number v-model="form.forceAtMs" :min="0" />
    </div>
    <div class="mt-4 flex gap-2">
      <el-button type="primary" @click="submit">发布</el-button>
    </div>
  </el-card>
</template>

<script setup lang="ts">
import { reactive } from "vue"
import { ElMessage } from "element-plus"
import { announceRelease } from "@/services/admin-release"

const form = reactive({
  releaseId: `admin-${Date.now()}`,
  kind: "soft" as "soft" | "force",
  messageI18nKey: "game.clientUpdateAvailable",
  recommendedDelaySec: 120,
  forceAtMs: 0,
  minAcceptedBuild: ""
})

async function submit() {
  try {
    await announceRelease({ ...form })
    ElMessage.success("发布成功")
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : "发布失败")
  }
}
</script>
