<template>
  <div class="grid gap-6">
    <el-card class="admin-card">
      <template #header><div class="font-semibold">猫舍管理（预留）</div></template>
      <div class="grid gap-3 md:grid-cols-3">
        <el-input v-model="triggerForm.userId" placeholder="User ID" />
        <el-input v-model="triggerForm.kittyId" placeholder="Kitty ID" />
        <el-select v-model="triggerForm.triggerType">
          <el-option label="diary" value="diary" />
          <el-option label="encounter" value="encounter" />
        </el-select>
      </div>
      <div class="mt-4 flex gap-2">
        <el-button type="primary" @click="submitTrigger">触发</el-button>
      </div>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { reactive } from "vue"
import { ElMessage } from "element-plus"
import { triggerCattery } from "@/services/admin-cattery"

const triggerForm = reactive({
  userId: "",
  kittyId: "",
  triggerType: "diary"
})

async function submitTrigger() {
  try {
    await triggerCattery({
      userId: Number(triggerForm.userId),
      kittyId: Number(triggerForm.kittyId),
      triggerType: triggerForm.triggerType
    })
    ElMessage.success("触发成功")
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : "触发失败")
  }
}
</script>
