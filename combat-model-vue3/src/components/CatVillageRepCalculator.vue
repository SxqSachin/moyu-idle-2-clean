<script setup lang="ts">
import { computed, ref } from 'vue'
import { computeRepLevel, PROD_REP_LEVELS } from '../config/cat-village-rep-config'

const totalRepInput = ref<number | null>(null)

const result = computed(() => {
  if (totalRepInput.value === null || totalRepInput.value < 0) return null
  return computeRepLevel(totalRepInput.value)
})

const maxLevel = PROD_REP_LEVELS.length
const maxCumXp = PROD_REP_LEVELS.reduce((s, l) => s + l.xpRequired, 0)

const progressPercent = computed(() => {
  if (!result.value || result.value.nextLevelXp <= 0) return 100
  return Math.min(100, Math.round((result.value.levelReputation / result.value.nextLevelXp) * 100))
})
</script>

<template>
  <div class="calc-root">
    <div class="calc-card">
      <h3 class="calc-title">声望等级计算器</h3>
      <p class="calc-desc">输入数据库中的 <code>reputation</code> 值，推算玩家声望等级（逻辑与后端 ComputeRepLevel 一致）</p>

      <label class="input-row">
        <span class="input-label">累计声望值</span>
        <input
          type="number"
          v-model.number="totalRepInput"
          min="0"
          step="1"
          placeholder="例如：153"
          class="rep-input"
        />
      </label>

      <div v-if="result" class="result-panel">
        <div class="result-level">
          <span class="level-badge">Lv.{{ result.level }}</span>
          <span v-if="result.level === maxLevel" class="max-tag">满级</span>
        </div>

        <div v-if="result.nextLevelXp > 0" class="progress-section">
          <div class="progress-bar">
            <div class="progress-fill" :style="{ width: progressPercent + '%' }"></div>
          </div>
          <div class="progress-text">
            {{ result.levelReputation.toLocaleString() }} / {{ result.nextLevelXp.toLocaleString() }}
            <span class="progress-pct">({{ progressPercent }}%)</span>
          </div>
        </div>

        <table class="result-table">
          <tbody>
            <tr><td>累计声望</td><td class="num">{{ result.totalReputation.toLocaleString() }}</td></tr>
            <tr><td>当前等级</td><td class="num">Lv.{{ result.level }}</td></tr>
            <tr v-if="result.nextLevelXp > 0"><td>本级进度</td><td class="num">{{ result.levelReputation.toLocaleString() }} / {{ result.nextLevelXp.toLocaleString() }}</td></tr>
            <tr v-if="result.nextLevelXp > 0"><td>升级还需</td><td class="num">{{ (result.nextLevelXp - result.levelReputation).toLocaleString() }}</td></tr>
          </tbody>
        </table>
      </div>

      <div v-else-if="totalRepInput !== null && totalRepInput < 0" class="result-empty">
        请输入大于等于 0 的值
      </div>
      <div v-else class="result-empty">
        输入声望值后显示结果
      </div>
    </div>

    <!-- Quick reference table -->
    <details class="ref-section">
      <summary class="ref-title">等级阈值参考表</summary>
      <table class="ref-table">
        <thead>
          <tr><th>等级</th><th>本级所需</th><th>累计总声望</th></tr>
        </thead>
        <tbody>
          <tr v-for="(lvl, idx) in PROD_REP_LEVELS" :key="lvl.level"
              :class="{ 'highlight-row': result && lvl.level === result.level + 1 }">
            <td>Lv.{{ lvl.level }}</td>
            <td class="num">{{ lvl.xpRequired }}</td>
            <td class="num">{{ PROD_REP_LEVELS.slice(0, idx + 1).reduce((s, l) => s + l.xpRequired, 0) }}</td>
          </tr>
        </tbody>
      </table>
    </details>

    <div class="ref-section">
      <div class="ref-title-static">关键等级快速对照</div>
      <table class="ref-table">
        <thead>
          <tr><th>等级</th><th>所需累计声望</th></tr>
        </thead>
        <tbody>
          <tr v-for="row in [5, 10, 15, 20]" :key="row">
            <td>Lv.{{ row }}</td>
            <td class="num">{{ PROD_REP_LEVELS.slice(0, row).reduce((s, l) => s + l.xpRequired, 0).toLocaleString() }}</td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>

<style scoped>
.calc-root {
  padding: 16px;
  max-width: 520px;
  color: #e2e8f0;
  font-size: 13px;
}
.calc-card {
  background: #1e293b;
  border-radius: 8px;
  padding: 16px;
}
.calc-title {
  margin: 0 0 4px;
  font-size: 15px;
  color: #f8fafc;
}
.calc-desc {
  margin: 0 0 16px;
  font-size: 12px;
  color: #64748b;
  line-height: 1.5;
}
.calc-desc code {
  color: #60a5fa;
  font-size: 11px;
}
.input-row {
  display: flex;
  flex-direction: column;
  gap: 6px;
  margin-bottom: 16px;
}
.input-label {
  font-size: 12px;
  color: #94a3b8;
}
.rep-input {
  padding: 8px 12px;
  border-radius: 6px;
  border: 1px solid #334155;
  background: #0f172a;
  color: #f8fafc;
  font-size: 16px;
  font-family: monospace;
  outline: none;
  transition: border-color 0.15s;
}
.rep-input:focus {
  border-color: #60a5fa;
}
.result-panel {
  display: flex;
  flex-direction: column;
  gap: 12px;
}
.result-level {
  display: flex;
  align-items: center;
  gap: 10px;
}
.level-badge {
  font-size: 22px;
  font-weight: 700;
  color: #60a5fa;
}
.max-tag {
  font-size: 11px;
  background: #f59e0b;
  color: #0f172a;
  padding: 2px 8px;
  border-radius: 4px;
  font-weight: 600;
}
.progress-section {
  display: flex;
  flex-direction: column;
  gap: 4px;
}
.progress-bar {
  height: 8px;
  background: #0f172a;
  border-radius: 4px;
  overflow: hidden;
}
.progress-fill {
  height: 100%;
  background: #60a5fa;
  border-radius: 4px;
  transition: width 0.2s;
}
.progress-text {
  font-size: 12px;
  color: #94a3b8;
}
.progress-pct {
  color: #60a5fa;
  font-weight: 600;
}
.result-table {
  border-collapse: collapse;
  font-size: 12px;
}
.result-table td {
  padding: 3px 12px 3px 0;
  color: #94a3b8;
}
.result-table .num {
  font-family: monospace;
  color: #f8fafc;
  text-align: right;
}
.result-empty {
  font-size: 12px;
  color: #475569;
  padding: 12px 0;
}
.ref-section {
  margin-top: 16px;
}
.ref-title {
  cursor: pointer;
  font-size: 13px;
  color: #64748b;
  margin-bottom: 8px;
}
.ref-title-static {
  font-size: 13px;
  color: #64748b;
  margin-bottom: 8px;
}
.ref-table {
  border-collapse: collapse;
  font-size: 12px;
  width: 100%;
}
.ref-table th, .ref-table td {
  padding: 3px 12px;
  text-align: left;
  border-bottom: 1px solid #1e293b;
  color: #94a3b8;
}
.ref-table th {
  color: #64748b;
  font-weight: 500;
  font-size: 11px;
}
.ref-table .num {
  font-family: monospace;
  text-align: right;
}
.highlight-row td {
  color: #60a5fa;
  font-weight: 600;
}
</style>
