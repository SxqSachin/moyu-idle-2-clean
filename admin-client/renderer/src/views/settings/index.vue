<template>
  <div class="grid gap-6 xl:grid-cols-[0.95fr_1.05fr]">
    <el-card class="admin-card">
      <template #header><div class="font-semibold">环境列表</div></template>
      <div class="mb-4 flex gap-2">
        <el-button type="primary" @click="createProfile">新建环境</el-button>
        <el-button @click="reloadProfiles">刷新</el-button>
        <el-button type="success" @click="testConnection">测试连接</el-button>
      </div>
      <el-table :data="profiles" stripe>
        <el-table-column prop="profileName" label="Name" min-width="140" />
        <el-table-column prop="serverBaseURL" label="Base URL" min-width="220" />
        <el-table-column label="Active" width="90">
          <template #default="{ row }">{{ row.profileId === activeProfileId ? "Yes" : "" }}</template>
        </el-table-column>
        <el-table-column label="操作" width="180">
          <template #default="{ row }">
            <div class="flex gap-2">
              <el-button size="small" @click="editProfile(row.profileId)">编辑</el-button>
              <el-button size="small" type="warning" @click="setActive(row.profileId)">启用</el-button>
              <el-button size="small" type="danger" @click="removeProfile(row.profileId)">删除</el-button>
            </div>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <el-card class="admin-card">
      <template #header><div class="font-semibold">环境配置</div></template>
      <div class="grid gap-3">
        <el-input v-model="form.profileName" placeholder="Profile Name" />
        <el-input v-model="form.serverBaseURL" placeholder="https://your-server.com" />
        <el-input v-model="form.releaseToken" show-password placeholder="Release Token" />
        <el-input-number v-model="form.timeoutMs" :min="1000" :max="60000" />
        <el-switch v-model="form.rememberToken" active-text="Remember Token" />
        <div class="flex gap-2">
          <el-button type="primary" @click="saveProfile">保存</el-button>
        </div>
      </div>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from "vue"
import { ElMessage } from "element-plus"
import type { AdminProfile } from "@shared/types"
import { getAdminBridge } from "@/services/bridge"

const profiles = ref<AdminProfile[]>([])
const activeProfileId = ref("")
const form = reactive<AdminProfile>({
  profileId: "",
  profileName: "",
  serverBaseURL: "",
  releaseToken: "",
  rememberToken: true,
  timeoutMs: 10000
})

onMounted(() => {
  reloadProfiles().catch((error) => {
    ElMessage.error(error instanceof Error ? error.message : "加载环境失败")
  })
})

async function reloadProfiles() {
  const config = await getAdminBridge().getProfiles()
  profiles.value = config.profiles || []
  activeProfileId.value = config.activeProfileId || ""
}

function createProfile() {
  Object.assign(form, {
    profileId: "",
    profileName: "",
    serverBaseURL: "",
    releaseToken: "",
    rememberToken: true,
    timeoutMs: 10000
  })
}

function editProfile(profileId: string) {
  const current = profiles.value.find((item) => item.profileId === profileId)
  if (!current) return
  Object.assign(form, current)
}

async function saveProfile() {
  try {
    await getAdminBridge().saveProfile({ ...form })
    ElMessage.success("保存成功")
    await reloadProfiles()
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : "保存失败")
  }
}

async function setActive(profileId: string) {
  try {
    await getAdminBridge().setActiveProfile(profileId)
    ElMessage.success("已切换环境")
    await reloadProfiles()
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : "切换失败")
  }
}

async function removeProfile(profileId: string) {
  try {
    await getAdminBridge().deleteProfile(profileId)
    ElMessage.success("已删除环境")
    await reloadProfiles()
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : "删除失败")
  }
}

async function testConnection() {
  try {
    await getAdminBridge().testConnection(form.profileId || activeProfileId.value)
    ElMessage.success("连接成功")
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : "连接失败")
  }
}
</script>
