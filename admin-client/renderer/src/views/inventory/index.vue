<template>
  <el-card class="admin-card">
    <template #header><div class="font-semibold">库存管理（预留）</div></template>
    <div class="grid gap-3 md:grid-cols-2">
      <el-input v-model="form.userId" placeholder="User ID" />
      <el-input v-model="form.itemId" placeholder="Item ID" />
      <el-input v-model="form.quantity" placeholder="Quantity" />
      <el-select v-model="form.grantMode">
        <el-option label="template" value="template" />
        <el-option label="enhanced" value="enhanced" />
      </el-select>
      <el-input v-model="form.enhanceLevel" placeholder="Enhance Level" />
    </div>
    <div class="mt-4">
      <el-button type="primary" @click="submit">发放物品</el-button>
    </div>
  </el-card>
</template>

<script setup lang="ts">
import { reactive } from "vue"
import { ElMessage } from "element-plus"
import { grantInventory } from "@/services/admin-inventory"

const form = reactive({
  userId: "",
  itemId: "",
  quantity: "1",
  grantMode: "template",
  enhanceLevel: ""
})

async function submit() {
  try {
    await grantInventory({
      userId: Number(form.userId),
      itemId: form.itemId.trim(),
      quantity: Number(form.quantity),
      grantMode: form.grantMode,
      enhanceLevel: form.enhanceLevel ? Number(form.enhanceLevel) : undefined
    })
    ElMessage.success("发放成功")
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : "发放失败")
  }
}
</script>
