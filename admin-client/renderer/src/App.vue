<template>
  <div class="admin-shell">
    <el-container class="app-layout">
      <el-aside width="220px" class="app-aside">
        <div class="app-brand">
          <div class="text-lg font-semibold">Admin Client</div>
          <div class="text-sm text-[#909399]">{{ activeProfileLabel }}</div>
        </div>
        <el-menu :default-active="$route.path" router>
          <el-menu-item index="/risk">风险观测</el-menu-item>
          <el-menu-item index="/battle">战斗运维</el-menu-item>
          <el-menu-item index="/users">用户管理</el-menu-item>
          <el-menu-item index="/wardrobe">衣柜管理</el-menu-item>
          <el-menu-item index="/settings">环境设置</el-menu-item>
        </el-menu>
      </el-aside>
      <el-container>
        <el-header class="app-header">
          <div class="text-base font-semibold">{{ pageTitle }}</div>
          <div class="card-toolbar">
            <el-button size="small" @click="reloadWindow">刷新</el-button>
            <el-button size="small" @click="refreshOperations">最近操作</el-button>
          </div>
        </el-header>
        <el-main class="app-main">
          <router-view />
        </el-main>
      </el-container>
    </el-container>

    <el-drawer v-model="operationsVisible" title="最近操作" size="460px">
      <el-timeline>
        <el-timeline-item v-for="item in recentOperations" :key="item.id" :timestamp="item.at">
          <div class="font-medium">{{ item.moduleName }} · {{ item.method }}</div>
          <div class="admin-muted text-sm">{{ item.profileName }} · {{ item.resourcePath }}</div>
        </el-timeline-item>
      </el-timeline>
    </el-drawer>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from "vue"
import { useRoute } from "vue-router"
import type { AdminOperationLog, AdminProfile } from "@shared/types"
import { getAdminBridge } from "@/services/bridge"

const route = useRoute()
const activeProfile = ref<AdminProfile | null>(null)
const recentOperations = ref<AdminOperationLog[]>([])
const operationsVisible = ref(false)

const titleMap: Record<string, string> = {
  "/risk": "风险观测",
  "/battle": "战斗运维",
  "/users": "用户管理",
  "/wardrobe": "衣柜管理",
  "/settings": "环境设置"
}

const pageTitle = computed(() => titleMap[route.path] || "Admin")
const activeProfileLabel = computed(() =>
  activeProfile.value ? `${activeProfile.value.profileName} · ${activeProfile.value.serverBaseURL}` : "未配置环境"
)

onMounted(async () => {
  try {
    activeProfile.value = await getAdminBridge().getActiveProfile()
  } catch {
    activeProfile.value = null
  }
})

async function refreshOperations() {
  recentOperations.value = await getAdminBridge().getRecentOperations()
  operationsVisible.value = true
}

function reloadWindow() {
  window.location.reload()
}
</script>
