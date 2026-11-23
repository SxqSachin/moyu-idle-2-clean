<template>
  <div class="battle-ops">
    <el-card class="admin-card">
      <template #header>
        <div class="panel-header">
          <div>
            <div class="font-semibold">战斗运维</div>
            <div class="admin-muted text-sm">{{ runtimeLabel }}</div>
          </div>
          <el-tooltip content="刷新运行状态" placement="bottom">
            <el-button
              circle
              :icon="Refresh"
              :loading="loadingMetrics"
              aria-label="刷新运行状态"
              @click="loadMetrics"
            />
          </el-tooltip>
        </div>
      </template>

      <div v-loading="loadingMetrics" class="panel-content">
        <el-alert
          title="请先完成 Server 与 Battle Worker 发布。重置会中断当前 Run，并让房间从恢复流程重新进入第 1 波。"
          type="warning"
          :closable="false"
          show-icon
        />

        <el-descriptions class="runtime-grid" :column="4" border size="small">
          <el-descriptions-item label="专精树">
            {{ specialization?.treeId || "未加载" }}
          </el-descriptions-item>
          <el-descriptions-item label="规则版本">
            {{ specialization?.rulesetVersion || "-" }}
          </el-descriptions-item>
          <el-descriptions-item label="平衡版本">
            <el-tag v-if="specialization" type="success" effect="plain">
              {{ specialization.balanceRevision }}
            </el-tag>
            <el-tag v-else type="danger" effect="plain">不可用</el-tag>
          </el-descriptions-item>
          <el-descriptions-item label="Battle Worker">
            <el-tag v-if="workerRuntime?.ready" type="success" effect="plain">
              {{ workerRuntime.matchingWorkers }}/{{ workerRuntime.workerCount }} 已就绪
            </el-tag>
            <el-tag v-else type="danger" effect="plain">
              {{ workerRuntime?.workerCount || 0 }} 个，版本未就绪
            </el-tag>
          </el-descriptions-item>
        </el-descriptions>

        <el-alert
          v-if="workerRuntime && !workerRuntime.ready"
          title="Battle Worker 尚未全部加载当前专精版本，重置操作已锁定。"
          type="error"
          :closable="false"
          show-icon
        />

        <section class="operation-section">
          <div class="section-heading">
            <div>
              <strong>重置活跃战斗</strong>
              <span>当前战斗记录写为 interrupted，不计作玩家战败。</span>
            </div>
          </div>

          <el-checkbox-group v-model="selectedStatuses" class="status-options">
            <el-checkbox-button
              v-for="option in statusOptions"
              :key="option.value"
              :value="option.value"
            >
              {{ option.label }}
            </el-checkbox-button>
          </el-checkbox-group>

          <div class="operation-actions">
            <el-button
              type="danger"
              :icon="RefreshRight"
              :loading="reinitializing"
              :disabled="!specialization || !workerRuntime?.ready || selectedStatuses.length === 0"
              @click="confirmReinitialize"
            >
              重置所选战斗
            </el-button>
            <span v-if="selectedStatuses.length === 0" class="validation-text">至少选择一种房间状态</span>
          </div>
        </section>

        <el-alert
          v-if="lastResult"
          class="result-alert"
          :title="`已重置 ${lastResult.reinitRooms} 个房间，中断 ${lastResult.interruptedRuns} 场运行`"
          :description="resultDescription"
          type="success"
          :closable="false"
          show-icon
        />

        <el-collapse v-if="metrics" class="metrics-detail">
          <el-collapse-item title="运行指标" name="metrics">
            <pre>{{ JSON.stringify(metrics, null, 2) }}</pre>
          </el-collapse-item>
        </el-collapse>
      </div>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from "vue"
import { ElMessage, ElMessageBox } from "element-plus"
import { Refresh, RefreshRight } from "@element-plus/icons-vue"
import {
  fetchBattleMetrics,
  reinitializeBattleRooms,
  type BattleActiveRoomStatus,
  type BattleMetricsResult,
  type BattleRoomsReinitializeResult
} from "@/services/admin-battle"

const statusOptions: Array<{ value: BattleActiveRoomStatus; label: string }> = [
  { value: "entering", label: "入场中" },
  { value: "playing", label: "战斗中" },
  { value: "recovering", label: "恢复中" }
]

const metrics = ref<BattleMetricsResult | null>(null)
const selectedStatuses = ref<BattleActiveRoomStatus[]>(statusOptions.map((option) => option.value))
const lastResult = ref<BattleRoomsReinitializeResult | null>(null)
const loadingMetrics = ref(false)
const reinitializing = ref(false)

const specialization = computed(() => metrics.value?.runtime?.specialization || null)
const workerRuntime = computed(() => metrics.value?.runtime?.battleWorkers || null)
const runtimeLabel = computed(() => {
  const appEnv = metrics.value?.runtime?.appEnv
  if (!appEnv) return "等待读取 Server 运行状态"
  return `${appEnv} · Server 已加载的游戏数据`
})
const resultDescription = computed(() => {
  const version = lastResult.value?.specialization
  return version
    ? `${version.rulesetVersion} · balanceRevision ${version.balanceRevision} · ${lastResult.value?.battleWorkers.matchingWorkers || 0} 个 Worker 已校验`
    : "本次响应未包含专精版本"
})

async function loadMetrics() {
  loadingMetrics.value = true
  try {
    metrics.value = await fetchBattleMetrics()
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : "读取战斗运行状态失败")
  } finally {
    loadingMetrics.value = false
  }
}

function isDialogDismissed(error: unknown) {
  return error === "cancel" || error === "close"
}

async function confirmReinitialize() {
  const expected = specialization.value
  if (!expected || selectedStatuses.value.length === 0) return

  try {
    await ElMessageBox.prompt(
      `将中断所选活跃战斗。请输入当前平衡版本 ${expected.balanceRevision} 继续。`,
      "确认重置战斗",
      {
        type: "warning",
        confirmButtonText: "确认重置",
        cancelButtonText: "取消",
        confirmButtonClass: "el-button--danger",
        inputPlaceholder: String(expected.balanceRevision),
        inputValidator: (value) => value.trim() === String(expected.balanceRevision) || `请输入 ${expected.balanceRevision}`
      }
    )
    reinitializing.value = true
    lastResult.value = await reinitializeBattleRooms({
      statuses: [...selectedStatuses.value],
      expectedSpecialization: { ...expected }
    })
    ElMessage.success(`已重置 ${lastResult.value.reinitRooms} 个战斗房间`)
    await loadMetrics()
  } catch (error) {
    if (!isDialogDismissed(error)) {
      ElMessage.error(error instanceof Error ? error.message : "重置战斗失败")
    }
  } finally {
    reinitializing.value = false
  }
}

onMounted(loadMetrics)
</script>

<style scoped>
.battle-ops {
  display: grid;
  gap: 16px;
}

.panel-header,
.operation-actions,
.section-heading > div {
  display: flex;
  align-items: center;
}

.panel-header {
  justify-content: space-between;
  gap: 16px;
}

.panel-content {
  display: grid;
  gap: 18px;
}

.runtime-grid {
  width: 100%;
}

.operation-section {
  display: grid;
  gap: 14px;
  padding-top: 2px;
}

.section-heading > div {
  flex-wrap: wrap;
  gap: 8px 14px;
}

.section-heading span,
.validation-text {
  color: var(--el-text-color-secondary);
  font-size: 12px;
}

.status-options {
  display: flex;
  flex-wrap: wrap;
}

.operation-actions {
  flex-wrap: wrap;
  gap: 10px;
}

.result-alert {
  margin-top: 2px;
}

.metrics-detail pre {
  max-height: 360px;
  margin: 0;
  overflow: auto;
  padding: 12px;
  border: 1px solid var(--el-border-color-lighter);
  border-radius: 6px;
  background: #111827;
  color: #e5e7eb;
  font-size: 12px;
  line-height: 1.5;
}

@media (max-width: 900px) {
  .runtime-grid :deep(.el-descriptions__body) {
    overflow-x: auto;
  }
}
</style>
