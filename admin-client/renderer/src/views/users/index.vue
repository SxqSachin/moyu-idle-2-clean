<template>
  <div class="grid gap-6">
    <el-card class="admin-card">
      <template #header><div class="font-semibold">封禁操作</div></template>
      <div class="grid gap-3 md:grid-cols-2">
        <el-input v-model="banForm.userId" placeholder="User ID" />
        <el-input v-model="banForm.username" placeholder="Username" />
        <el-select v-model="banForm.reason" placeholder="Ban reason">
          <el-option v-for="reason in banReasons" :key="reason.key" :label="reason.key" :value="reason.key" />
        </el-select>
        <el-switch v-model="banForm.isPermanent" active-text="永久封禁" inactive-text="临时封禁" />
        <el-input v-model="banForm.banUntil" placeholder="banUntil (RFC3339)" :disabled="banForm.isPermanent" />
        <div class="flex gap-2">
          <el-button type="danger" @click="submitBan">封禁</el-button>
          <el-button @click="submitUnban">解封</el-button>
        </div>
      </div>
    </el-card>

    <el-card class="admin-card">
      <template #header>
        <div class="flex items-center justify-between">
          <div class="font-semibold">已封禁玩家</div>
          <div class="flex gap-2">
            <el-input v-model="searchQuery" placeholder="Search user/email/id" @keyup.enter="loadBannedUsers" />
            <el-button @click="loadBannedUsers">刷新</el-button>
          </div>
        </div>
      </template>
      <el-table :data="bannedUsers" stripe>
        <el-table-column prop="userId" label="UserID" width="110" />
        <el-table-column prop="username" label="Username" min-width="150" />
        <el-table-column prop="email" label="Email" min-width="190" />
        <el-table-column prop="banReason" label="Reason" width="160" />
        <el-table-column prop="banIsPermanent" label="Permanent" width="100">
          <template #default="{ row }">{{ row.banIsPermanent ? "Yes" : "No" }}</template>
        </el-table-column>
        <el-table-column prop="banUntil" label="Until" min-width="190" />
        <el-table-column label="操作" width="180">
          <template #default="{ row }">
            <div class="flex gap-2">
              <el-button size="small" @click="inspectUser(row.userId)">详情</el-button>
              <el-button size="small" type="success" @click="quickUnban(row.userId)">解封</el-button>
            </div>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <el-card class="admin-card" v-if="selectedUser">
      <template #header><div class="font-semibold">用户详情</div></template>
      <div class="grid gap-2 md:grid-cols-2">
        <div>ID: {{ selectedUser.userId }}</div>
        <div>Username: {{ selectedUser.username }}</div>
        <div>Email: {{ selectedUser.email }}</div>
        <div>Email Verified: {{ selectedUser.emailVerified ? "Yes" : "No" }}</div>
        <div>Ban Reason: {{ selectedUser.banReason || "-" }}</div>
        <div>Ban Until: {{ selectedUser.banUntil || "-" }}</div>
        <div>Created At: {{ selectedUser.createdAt }}</div>
      </div>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from "vue"
import { ElMessage } from "element-plus"
import type { AdminBanReason, AdminUserDetail } from "@shared/types"
import { banUser, fetchAdminUserDetail, fetchBanReasons, fetchBannedUsers, unbanUser } from "@/services/admin-users"

const banReasons = ref<AdminBanReason[]>([])
const bannedUsers = ref<AdminUserDetail[]>([])
const selectedUser = ref<AdminUserDetail | null>(null)
const searchQuery = ref("")
const banForm = reactive({
  userId: "",
  username: "",
  reason: "",
  isPermanent: true,
  banUntil: ""
})

onMounted(async () => {
  try {
    const reasonsPayload = await fetchBanReasons()
    banReasons.value = reasonsPayload.reasons || []
    banForm.reason = banReasons.value[0]?.key || ""
    await loadBannedUsers()
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : "加载用户管理数据失败")
  }
})

async function loadBannedUsers() {
  const payload = await fetchBannedUsers(searchQuery.value.trim())
  bannedUsers.value = payload.items || []
}

async function inspectUser(userId: number) {
  selectedUser.value = await fetchAdminUserDetail(userId)
}

async function submitBan() {
  try {
    await banUser({
      userId: banForm.userId ? Number(banForm.userId) : undefined,
      username: banForm.username.trim() || undefined,
      reason: banForm.reason,
      isPermanent: banForm.isPermanent,
      banUntil: banForm.isPermanent ? undefined : banForm.banUntil.trim()
    })
    ElMessage.success("封禁成功")
    await loadBannedUsers()
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : "封禁失败")
  }
}

async function submitUnban() {
  try {
    await unbanUser({
      userId: banForm.userId ? Number(banForm.userId) : undefined,
      username: banForm.username.trim() || undefined
    })
    ElMessage.success("解封成功")
    await loadBannedUsers()
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : "解封失败")
  }
}

async function quickUnban(userId: number) {
  await unbanUser({ userId })
  ElMessage.success("解封成功")
  await loadBannedUsers()
}
</script>
