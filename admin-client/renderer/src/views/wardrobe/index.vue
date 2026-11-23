<template>
  <div class="space-y-4">
    <el-card class="admin-card">
      <template #header><div class="font-semibold">衣柜管理</div></template>

      <div class="mb-4 grid gap-3 md:grid-cols-3">
        <el-input v-model="queryUserId" placeholder="User ID" @keyup.enter="loadWardrobe" />
        <el-button type="primary" @click="loadWardrobe" :loading="loading">查询</el-button>
      </div>

      <div v-if="wardrobe" class="space-y-4">
        <div>
          <div class="mb-2 text-sm font-medium">拥有 ({{ wardrobe.owned.length }})</div>
          <el-table :data="wardrobe.owned" size="small" border>
            <el-table-column prop="category" label="品类" width="140" />
            <el-table-column prop="itemId" label="物品ID">
              <template #default="{ row }">{{ formatItemId(row) }}</template>
            </el-table-column>
            <el-table-column prop="source" label="来源" width="160" />
            <el-table-column prop="obtainedAt" label="获取时间" width="180">
              <template #default="{ row }">{{ formatDate(row.obtainedAt) }}</template>
            </el-table-column>
            <el-table-column label="操作" width="80">
              <template #default="{ row }">
                <el-button size="small" type="danger" text @click="handleRevoke(row.category, row.itemId)">撤销</el-button>
              </template>
            </el-table-column>
          </el-table>
        </div>

        <div>
          <div class="mb-2 text-sm font-medium">装备中 ({{ wardrobe.equipped.length }})</div>
          <el-table :data="wardrobe.equipped" size="small" border>
            <el-table-column prop="ownerType" label="主体" width="100" />
            <el-table-column prop="ownerId" label="主体ID" width="100" />
            <el-table-column prop="slotId" label="槽位" width="180" />
            <el-table-column prop="itemId" label="物品ID">
              <template #default="{ row }">{{ formatItemId(row) }}</template>
            </el-table-column>
          </el-table>
        </div>
      </div>
    </el-card>

    <el-card class="admin-card">
      <template #header><div class="font-semibold">发放称号</div></template>

      <div class="grid gap-3 md:grid-cols-2">
        <el-input v-model="grantForm.userId" placeholder="User ID" />
        <el-select v-model="grantForm.category" placeholder="选择品类" @change="onCategoryChange">
          <el-option label="称号前缀" value="title_prefix" />
          <el-option label="称号后缀" value="title_suffix" />
        </el-select>
        <el-select
          v-model="grantForm.itemId"
          placeholder="选择称号"
          filterable
          allow-create
          class="md:col-span-2"
        >
          <el-option
            v-for="item in itemOptions"
            :key="item.id"
            :label="`${item.id} (${item.rarity})`"
            :value="item.id"
          />
        </el-select>
        <el-input v-model="grantForm.source" placeholder="来源 (如 admin_manual)" class="md:col-span-2" />
      </div>
      <div class="mt-4 flex gap-2">
        <el-button type="primary" @click="handleGrant" :loading="granting">发放</el-button>
      </div>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted } from "vue"
import { ElMessage } from "element-plus"
import {
  listWardrobe,
  grantCosmetic,
  revokeCosmetic,
  loadTitlePrefixes,
  loadTitleSuffixes,
  type WardrobeListResponse,
  type TitleEntry
} from "@/services/admin-wardrobe"

const queryUserId = ref("")
const wardrobe = ref<WardrobeListResponse | null>(null)
const loading = ref(false)
const granting = ref(false)

const grantForm = reactive({
  userId: "",
  category: "title_prefix",
  itemId: "",
  source: "admin_manual"
})

const prefixes = ref<TitleEntry[]>([])
const suffixes = ref<TitleEntry[]>([])

const itemOptions = computed<TitleEntry[]>(() => {
  if (grantForm.category === "title_prefix") return prefixes.value
  if (grantForm.category === "title_suffix") return suffixes.value
  return []
})

onMounted(async () => {
  try {
    const [p, s] = await Promise.all([loadTitlePrefixes(), loadTitleSuffixes()])
    prefixes.value = p.filter((e) => !e.disabled).sort((a, b) => a.order - b.order)
    suffixes.value = s.filter((e) => !e.disabled).sort((a, b) => a.order - b.order)
  } catch {
    // game-data files may not exist yet
  }
})

function onCategoryChange() {
  grantForm.itemId = ""
}

function formatItemId(row: { category: string; itemId: string }): string {
  const cat = row.category
  const id = row.itemId
  if (cat === "title_prefix") {
    const entry = prefixes.value.find((e) => e.id === id)
    return entry ? `${entry.id} (${(entry as any).nameKey || ""})` : id
  }
  if (cat === "title_suffix") {
    const entry = suffixes.value.find((e) => e.id === id)
    return entry ? `${entry.id} (${(entry as any).nameKey || ""})` : id
  }
  return id
}

function formatDate(raw: string) {
  if (!raw) return ""
  try {
    return new Date(raw).toLocaleString()
  } catch {
    return raw
  }
}

async function loadWardrobe() {
  const uid = Number(queryUserId.value)
  if (!uid || uid <= 0) {
    ElMessage.warning("请输入有效的 User ID")
    return
  }
  loading.value = true
  try {
    wardrobe.value = await listWardrobe(uid)
  } catch (err) {
    ElMessage.error(err instanceof Error ? err.message : "查询失败")
    wardrobe.value = null
  } finally {
    loading.value = false
  }
}

async function handleGrant() {
  const uid = Number(grantForm.userId)
  if (!uid || uid <= 0) {
    ElMessage.warning("请输入有效的 User ID")
    return
  }
  if (!grantForm.itemId.trim()) {
    ElMessage.warning("请选择或输入物品ID")
    return
  }
  granting.value = true
  try {
    const result = await grantCosmetic({
      userId: uid,
      category: grantForm.category,
      itemId: grantForm.itemId.trim(),
      source: grantForm.source.trim() || "admin_manual"
    })
    ElMessage.success(result.alreadyOwned ? "已拥有，跳过" : "发放成功")
    if (Number(queryUserId.value) === uid) await loadWardrobe()
  } catch (err) {
    ElMessage.error(err instanceof Error ? err.message : "发放失败")
  } finally {
    granting.value = false
  }
}

async function handleRevoke(category: string, itemId: string) {
  const uid = Number(queryUserId.value)
  if (!uid || uid <= 0) return
  try {
    await revokeCosmetic({ userId: uid, category, itemId })
    ElMessage.success("撤销成功")
    await loadWardrobe()
  } catch (err) {
    ElMessage.error(err instanceof Error ? err.message : "撤销失败")
  }
}
</script>
