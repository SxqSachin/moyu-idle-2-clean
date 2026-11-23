<script setup lang="ts">
import { computed, ref } from 'vue'
import { repConfig, saveRepConfig, resetRepConfig, computeLevels } from '../config/cat-village-rep-config'

const CHART_W = 520
const CHART_H = 300
const PAD = { top: 16, right: 24, bottom: 40, left: 60 }
const INNER_W = CHART_W - PAD.left - PAD.right
const INNER_H = CHART_H - PAD.top - PAD.bottom

const levels = computed(() => computeLevels(repConfig))
const cumulative = computed(() => {
  let cum = 0
  return levels.value.map(l => { cum += l.xpRequired; return { ...l, cumXp: cum } })
})

const maxXpPerLevel = computed(() => Math.max(...levels.value.map(l => l.xpRequired), 1))
const maxCumXp = computed(() => cumulative.value[cumulative.value.length - 1]?.cumXp ?? 1)
const maxL = computed(() => levels.value.length)

function svgX(level: number) {
  return PAD.left + ((level - 1) / Math.max(maxL.value - 1, 1)) * INNER_W
}
function barY(xp: number) {
  return PAD.top + (1 - xp / maxXpPerLevel.value) * INNER_H
}
function cumY(xp: number) {
  return PAD.top + (1 - xp / maxCumXp.value) * INNER_H
}

const cumPolyline = computed(() =>
  cumulative.value.map(p => `${svgX(p.level).toFixed(1)},${cumY(p.cumXp).toFixed(1)}`).join(' ')
)

const yTicksCum = computed(() => {
  const n = 5
  return Array.from({ length: n + 1 }, (_, i) => {
    const v = (maxCumXp.value * i) / n
    return { v, y: cumY(v), label: v >= 1000 ? `${(v / 1000).toFixed(1)}k` : String(Math.round(v)) }
  })
})

const barW = computed(() => Math.max(4, Math.min(28, (INNER_W / Math.max(maxL.value, 1)) * 0.6)))

const showExport = ref(false)

const exportJson = computed(() => JSON.stringify({ repLevels: levels.value }, null, 2))

function onInput() { saveRepConfig() }
</script>

<template>
  <div class="rep-chart-root">
    <div class="rep-layout">
      <!-- Left: params -->
      <aside class="param-panel">
        <div class="param-section">
          <div class="param-section-title">声望曲线参数</div>

          <label class="param-row">
            <span class="param-label">最大等级 <code>maxLevel</code></span>
            <input type="range" min="3" max="30" step="1" v-model.number="repConfig.maxLevel" @input="onInput" />
            <span class="param-val">{{ repConfig.maxLevel }}</span>
          </label>

          <label class="param-row">
            <span class="param-label">1级所需 <code>baseXp</code></span>
            <input type="range" min="10" max="2000" step="10" v-model.number="repConfig.baseXp" @input="onInput" />
            <span class="param-val">{{ repConfig.baseXp }}</span>
          </label>

          <label class="param-row">
            <span class="param-label">每级倍率 <code>growthFactor</code></span>
            <input type="range" min="1.0" max="3.0" step="0.05" v-model.number="repConfig.growthFactor" @input="onInput" />
            <span class="param-val">× {{ repConfig.growthFactor.toFixed(2) }}</span>
          </label>
        </div>

        <div class="param-section">
          <div class="param-section-title">公式</div>
          <div class="formula-desc">
            第 N 级所需 = <code>baseXp × growthFactor^(N-1)</code>
          </div>
          <div class="formula-example">
            例：第1级 = {{ repConfig.baseXp }} &nbsp;|&nbsp;
            第{{ repConfig.maxLevel }}级 = {{ levels[levels.length - 1]?.xpRequired.toLocaleString() }}
          </div>
          <div class="formula-example">
            满级累计 = {{ maxCumXp.toLocaleString() }}
          </div>
        </div>

        <div class="param-actions">
          <button class="btn-secondary" @click="resetRepConfig">重置默认值</button>
          <button class="btn-secondary" @click="showExport = !showExport">
            {{ showExport ? '隐藏' : '导出' }}配置
          </button>
        </div>

        <div v-if="showExport" class="export-box">
          <div class="export-hint">
            将以下内容写入：<br>
            <code>packages/game-config/source/cat-village-config.json</code>
          </div>
          <pre>{{ exportJson }}</pre>
        </div>
      </aside>

      <!-- Right: chart -->
      <div class="chart-area">
        <svg :viewBox="`0 0 ${CHART_W} ${CHART_H}`" class="chart-svg" xmlns="http://www.w3.org/2000/svg">
          <!-- Grid -->
          <line v-for="tick in yTicksCum" :key="tick.v"
            :x1="PAD.left" :x2="PAD.left + INNER_W"
            :y1="tick.y" :y2="tick.y"
            stroke="#1e293b" stroke-width="1"
            :stroke-dasharray="tick.v === 0 ? 'none' : '4,3'"
          />

          <!-- Bars (XP per level) -->
          <rect
            v-for="p in cumulative" :key="`bar-${p.level}`"
            :x="svgX(p.level) - barW / 2"
            :y="barY(p.xpRequired)"
            :width="barW"
            :height="PAD.top + INNER_H - barY(p.xpRequired)"
            fill="#1e3a5f"
            rx="2"
          />

          <!-- Cumulative line -->
          <polyline v-if="cumulative.length > 1"
            :points="cumPolyline"
            fill="none" stroke="#60a5fa" stroke-width="2"
            stroke-linejoin="round" stroke-linecap="round"
          />

          <!-- Points -->
          <circle v-for="p in cumulative" :key="`pt-${p.level}`"
            :cx="svgX(p.level)" :cy="cumY(p.cumXp)"
            r="3.5" fill="#60a5fa" stroke="#0f172a" stroke-width="1.5"
          />

          <!-- Y labels (cumulative) -->
          <text v-for="tick in yTicksCum" :key="`yl-${tick.v}`"
            :x="PAD.left - 6" :y="tick.y + 4"
            text-anchor="end" font-size="10" fill="#475569"
          >{{ tick.label }}</text>

          <!-- X labels (level) -->
          <text v-for="p in cumulative" :key="`xl-${p.level}`"
            :x="svgX(p.level)" :y="PAD.top + INNER_H + 16"
            text-anchor="middle" font-size="10" fill="#64748b"
          >{{ p.level }}</text>

          <!-- Axis titles -->
          <text :x="PAD.left + INNER_W / 2" :y="CHART_H - 4"
            text-anchor="middle" font-size="11" fill="#475569">等级</text>
          <text :x="10" :y="PAD.top + INNER_H / 2"
            text-anchor="middle" font-size="11" fill="#475569"
            :transform="`rotate(-90,10,${PAD.top + INNER_H / 2})`">累计声望</text>
        </svg>

        <div class="legend">
          <span class="legend-bar"></span><span>每级所需（柱）</span>
          <span class="legend-line"></span><span>累计声望（线）</span>
        </div>

        <!-- Data table -->
        <div class="data-table">
          <table>
            <thead>
              <tr><th>等级</th><th>本级所需</th><th>累计总声望</th></tr>
            </thead>
            <tbody>
              <tr v-for="p in cumulative" :key="p.level">
                <td>Lv.{{ p.level }}</td>
                <td>{{ p.xpRequired.toLocaleString() }}</td>
                <td>{{ p.cumXp.toLocaleString() }}</td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.rep-chart-root {
  padding: 16px;
  color: #e2e8f0;
  font-size: 13px;
}
.rep-layout {
  display: flex;
  gap: 24px;
  align-items: flex-start;
}
.param-panel {
  flex: 0 0 280px;
  display: flex;
  flex-direction: column;
  gap: 16px;
}
.param-section {
  background: #1e293b;
  border-radius: 8px;
  padding: 12px;
  display: flex;
  flex-direction: column;
  gap: 8px;
}
.param-section-title {
  font-size: 11px;
  font-weight: 600;
  color: #64748b;
  text-transform: uppercase;
  letter-spacing: 0.05em;
  margin-bottom: 4px;
}
.param-row {
  display: grid;
  grid-template-columns: 1fr auto auto;
  align-items: center;
  gap: 8px;
}
.param-label {
  font-size: 12px;
  color: #94a3b8;
}
.param-label code {
  font-size: 10px;
  color: #60a5fa;
  margin-left: 4px;
}
.param-val {
  font-size: 12px;
  font-family: monospace;
  color: #f8fafc;
  min-width: 52px;
  text-align: right;
}
input[type="range"] {
  width: 100px;
  accent-color: #60a5fa;
}
.formula-desc {
  font-size: 12px;
  color: #94a3b8;
}
.formula-desc code { color: #60a5fa; }
.formula-example {
  font-size: 11px;
  color: #64748b;
  font-family: monospace;
}
.param-actions {
  display: flex;
  gap: 8px;
}
.btn-secondary {
  flex: 1;
  padding: 6px 10px;
  background: #1e293b;
  border: 1px solid #334155;
  border-radius: 6px;
  color: #94a3b8;
  cursor: pointer;
  font-size: 12px;
  transition: border-color 0.15s;
}
.btn-secondary:hover {
  border-color: #60a5fa;
  color: #e2e8f0;
}
.export-box {
  background: #0f172a;
  border: 1px solid #334155;
  border-radius: 8px;
  padding: 12px;
  display: flex;
  flex-direction: column;
  gap: 8px;
}
.export-hint {
  font-size: 11px;
  color: #64748b;
  line-height: 1.6;
}
.export-hint code {
  color: #60a5fa;
  font-size: 10px;
  word-break: break-all;
}
.export-box pre {
  margin: 0;
  font-size: 11px;
  color: #94a3b8;
  white-space: pre-wrap;
  word-break: break-all;
  max-height: 300px;
  overflow-y: auto;
}
.chart-area {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 12px;
}
.chart-svg {
  width: 100%;
  background: #0f172a;
  border-radius: 8px;
  border: 1px solid #1e293b;
}
.legend {
  display: flex;
  align-items: center;
  gap: 10px;
  font-size: 11px;
  color: #64748b;
}
.legend-bar {
  display: inline-block;
  width: 12px;
  height: 12px;
  background: #1e3a5f;
  border-radius: 2px;
}
.legend-line {
  display: inline-block;
  width: 20px;
  height: 2px;
  background: #60a5fa;
  border-radius: 1px;
  margin-left: 8px;
}
.data-table { overflow-x: auto; }
.data-table table {
  border-collapse: collapse;
  font-size: 12px;
  width: 100%;
  max-width: 380px;
}
.data-table th, .data-table td {
  padding: 4px 12px;
  text-align: left;
  border-bottom: 1px solid #1e293b;
  color: #94a3b8;
}
.data-table th {
  color: #64748b;
  font-weight: 500;
  font-size: 11px;
}
</style>
