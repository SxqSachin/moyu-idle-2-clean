<script setup lang="ts">
import { computed, ref } from 'vue'
import {
  enhanceConfig,
  computeSuccessRate,
  resetEnhanceConfig,
  RARITIES,
  RARITY_LABELS,
  RARITY_COLORS,
  MAX_LEVEL_COLORS,
  type RarityKey,
} from '../config/enhance-config'

const CHART_W = 520
const CHART_H = 320
const PAD = { top: 16, right: 16, bottom: 40, left: 48 }
const INNER_W = CHART_W - PAD.left - PAD.right
const INNER_H = CHART_H - PAD.top - PAD.bottom

const SAMPLE_MAX_LEVELS = [10, 15, 20, 25, 30, 40, 50]
const POINTS = 80

type ChartMode = 'byMaxLevel' | 'byRarity'
const mode = ref<ChartMode>('byMaxLevel')
const fixedRarity = ref<RarityKey>('common')
const fixedMaxLevel = ref(25)

function toSvgX(progress: number) {
  return PAD.left + progress * INNER_W
}
function toSvgY(rate: number) {
  return PAD.top + (1 - rate) * INNER_H
}

function buildPolyline(rarity: string, maxLevel: number): string {
  const pts: string[] = []
  for (let i = 0; i <= POINTS; i++) {
    const progress = i / POINTS
    const lvl = 1 + progress * (maxLevel - 1)
    const rate = computeSuccessRate(rarity, Math.round(lvl), maxLevel)
    pts.push(`${toSvgX(progress).toFixed(1)},${toSvgY(rate).toFixed(1)}`)
  }
  return pts.join(' ')
}

const byMaxLevelLines = computed(() =>
  SAMPLE_MAX_LEVELS.map((ml) => ({
    maxLevel: ml,
    color: MAX_LEVEL_COLORS[ml] ?? '#888',
    points: buildPolyline(fixedRarity.value, ml),
    endRate: computeSuccessRate(fixedRarity.value, ml, ml),
  }))
)

const byRarityLines = computed(() =>
  RARITIES.map((r) => ({
    rarity: r,
    label: RARITY_LABELS[r],
    color: RARITY_COLORS[r],
    points: buildPolyline(r, fixedMaxLevel.value),
    endRate: computeSuccessRate(r, fixedMaxLevel.value, fixedMaxLevel.value),
  }))
)

const yGridLines = [0, 0.25, 0.5, 0.75, 1.0]
const xGridLines = [0, 0.25, 0.5, 0.75, 1.0]

const exportCode = computed(() => {
  const c = enhanceConfig
  return `// enhance_runtime_cache.go
minSuccessRate := clampFloat(${c.minRateBase}-(float64(maxLevel)-15)*(${c.minRateScale}/35), ${c.minRateFloor}, ${c.minRateBase})
curvePower := clampFloat(${c.curvePowerBase}-(float64(maxLevel)-15)*(${c.curvePowerScale}/35), ${c.curvePowerMin}, ${c.curvePowerBase})

// equipment-enhance.ts
const minSuccessRate = clampRate(Math.max(${c.minRateFloor}, Math.min(${c.minRateBase}, ${c.minRateBase} - (maxLevel - 15) * (${c.minRateScale} / 35))))
const curvePower = Math.max(${c.curvePowerMin}, Math.min(${c.curvePowerBase}, ${c.curvePowerBase} - (maxLevel - 15) * (${c.curvePowerScale} / 35)))

// rarityFailMultiplier
${RARITIES.map((r) => `"${r}": ${c.rarityFailMultiplier[r]}`).join('\n')}`
})

const showExport = ref(false)
</script>

<template>
  <div class="enhance-chart-root">
    <div class="enhance-layout">
      <!-- Left: params -->
      <aside class="param-panel">
        <div class="param-section">
          <div class="param-section-title">底限参数 (minSuccessRate)</div>
          <label class="param-row">
            <span class="param-label">基准值 <code>minRateBase</code></span>
            <input type="range" min="0.10" max="0.80" step="0.01" v-model.number="enhanceConfig.minRateBase" />
            <span class="param-val">{{ enhanceConfig.minRateBase.toFixed(2) }}</span>
          </label>
          <label class="param-row">
            <span class="param-label">衰减系数 <code>minRateScale</code></span>
            <input type="range" min="0.10" max="1.20" step="0.01" v-model.number="enhanceConfig.minRateScale" />
            <span class="param-val">{{ enhanceConfig.minRateScale.toFixed(2) }}/35</span>
          </label>
          <label class="param-row">
            <span class="param-label">最低下限 <code>minRateFloor</code></span>
            <input type="range" min="0.01" max="0.40" step="0.01" v-model.number="enhanceConfig.minRateFloor" />
            <span class="param-val">{{ enhanceConfig.minRateFloor.toFixed(2) }}</span>
          </label>
        </div>

        <div class="param-section">
          <div class="param-section-title">曲线参数 (curvePower)</div>
          <label class="param-row">
            <span class="param-label">基准值 <code>curvePowerBase</code></span>
            <input type="range" min="0.50" max="3.00" step="0.05" v-model.number="enhanceConfig.curvePowerBase" />
            <span class="param-val">{{ enhanceConfig.curvePowerBase.toFixed(2) }}</span>
          </label>
          <label class="param-row">
            <span class="param-label">衰减系数 <code>curvePowerScale</code></span>
            <input type="range" min="0.00" max="2.00" step="0.05" v-model.number="enhanceConfig.curvePowerScale" />
            <span class="param-val">{{ enhanceConfig.curvePowerScale.toFixed(2) }}/35</span>
          </label>
          <label class="param-row">
            <span class="param-label">最低下限 <code>curvePowerMin</code></span>
            <input type="range" min="0.10" max="2.00" step="0.05" v-model.number="enhanceConfig.curvePowerMin" />
            <span class="param-val">{{ enhanceConfig.curvePowerMin.toFixed(2) }}</span>
          </label>
        </div>

        <div class="param-section">
          <div class="param-section-title">稀有度失败倍率</div>
          <label v-for="r in RARITIES" :key="r" class="param-row rarity-row">
            <span class="rarity-dot" :style="{ background: RARITY_COLORS[r] }"></span>
            <span class="param-label rarity-label">{{ RARITY_LABELS[r] }}</span>
            <input type="range" min="1.00" max="2.00" step="0.01"
              v-model.number="enhanceConfig.rarityFailMultiplier[r]" />
            <span class="param-val">× {{ enhanceConfig.rarityFailMultiplier[r].toFixed(2) }}</span>
          </label>
        </div>

        <div class="param-actions">
          <button class="btn-secondary" @click="resetEnhanceConfig">重置默认值</button>
          <button class="btn-secondary" @click="showExport = !showExport">{{ showExport ? '隐藏' : '导出' }}代码片段</button>
        </div>
      </aside>

      <!-- Right: chart -->
      <div class="chart-area">
        <div class="chart-toolbar">
          <div class="mode-tabs">
            <button :class="['tab', { active: mode === 'byMaxLevel' }]" @click="mode = 'byMaxLevel'">按最大等级</button>
            <button :class="['tab', { active: mode === 'byRarity' }]" @click="mode = 'byRarity'">按稀有度</button>
          </div>

          <div v-if="mode === 'byMaxLevel'" class="inline-ctrl">
            <span>固定稀有度：</span>
            <select v-model="fixedRarity">
              <option v-for="r in RARITIES" :key="r" :value="r">{{ RARITY_LABELS[r] }}</option>
            </select>
          </div>
          <div v-else class="inline-ctrl">
            <span>固定最大等级：</span>
            <input type="number" min="5" max="100" step="1" v-model.number="fixedMaxLevel" style="width:64px" />
          </div>
        </div>

        <svg
          :viewBox="`0 0 ${CHART_W} ${CHART_H}`"
          class="chart-svg"
          xmlns="http://www.w3.org/2000/svg"
        >
          <!-- Grid -->
          <g class="grid">
            <line v-for="y in yGridLines" :key="`gy${y}`"
              :x1="PAD.left" :x2="PAD.left + INNER_W"
              :y1="toSvgY(y)" :y2="toSvgY(y)"
              stroke="#334155" stroke-width="1" :stroke-dasharray="y === 0 || y === 1 ? 'none' : '4,3'"
            />
            <line v-for="x in xGridLines" :key="`gx${x}`"
              :x1="toSvgX(x)" :x2="toSvgX(x)"
              :y1="PAD.top" :y2="PAD.top + INNER_H"
              stroke="#334155" stroke-width="1" :stroke-dasharray="x === 0 || x === 1 ? 'none' : '4,3'"
            />
          </g>

          <!-- Y axis labels -->
          <g class="y-labels">
            <text v-for="y in yGridLines" :key="`yl${y}`"
              :x="PAD.left - 6" :y="toSvgY(y) + 4"
              text-anchor="end" font-size="10" fill="#94a3b8"
            >{{ (y * 100).toFixed(0) }}%</text>
          </g>

          <!-- X axis labels -->
          <g class="x-labels">
            <text v-for="x in xGridLines" :key="`xl${x}`"
              :x="toSvgX(x)" :y="PAD.top + INNER_H + 16"
              text-anchor="middle" font-size="10" fill="#94a3b8"
            >{{ (x * 100).toFixed(0) }}%</text>
          </g>

          <!-- Axis titles -->
          <text :x="PAD.left + INNER_W / 2" :y="CHART_H - 4"
            text-anchor="middle" font-size="11" fill="#64748b">强化进度 (level / maxLevel)</text>
          <text :x="12" :y="PAD.top + INNER_H / 2"
            text-anchor="middle" font-size="11" fill="#64748b"
            transform="rotate(-90,12,160)">成功率</text>

          <!-- Lines: byMaxLevel -->
          <g v-if="mode === 'byMaxLevel'">
            <polyline v-for="line in byMaxLevelLines" :key="line.maxLevel"
              :points="line.points"
              fill="none" :stroke="line.color" stroke-width="2" stroke-linejoin="round"
            />
            <!-- End label -->
            <g v-for="line in byMaxLevelLines" :key="`el${line.maxLevel}`">
              <circle :cx="toSvgX(1)" :cy="toSvgY(line.endRate)" r="3" :fill="line.color" />
            </g>
          </g>

          <!-- Lines: byRarity -->
          <g v-if="mode === 'byRarity'">
            <polyline v-for="line in byRarityLines" :key="line.rarity"
              :points="line.points"
              fill="none" :stroke="line.color" stroke-width="2" stroke-linejoin="round"
            />
            <g v-for="line in byRarityLines" :key="`er${line.rarity}`">
              <circle :cx="toSvgX(1)" :cy="toSvgY(line.endRate)" r="3" :fill="line.color" />
            </g>
          </g>
        </svg>

        <!-- Legend -->
        <div class="legend">
          <template v-if="mode === 'byMaxLevel'">
            <div v-for="line in byMaxLevelLines" :key="line.maxLevel" class="legend-item">
              <span class="legend-dot" :style="{ background: line.color }"></span>
              <span>max {{ line.maxLevel }}</span>
              <span class="legend-end">末级 {{ (line.endRate * 100).toFixed(1) }}%</span>
            </div>
          </template>
          <template v-else>
            <div v-for="line in byRarityLines" :key="line.rarity" class="legend-item">
              <span class="legend-dot" :style="{ background: line.color }"></span>
              <span>{{ line.label }}</span>
              <span class="legend-end">末级 {{ (line.endRate * 100).toFixed(1) }}%</span>
            </div>
          </template>
        </div>

        <!-- Export snippet -->
        <div v-if="showExport" class="export-box">
          <pre>{{ exportCode }}</pre>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.enhance-chart-root {
  padding: 16px;
  color: #e2e8f0;
  font-size: 13px;
}

.enhance-layout {
  display: flex;
  gap: 24px;
  align-items: flex-start;
}

/* Param panel */
.param-panel {
  flex: 0 0 300px;
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

.rarity-row {
  grid-template-columns: auto 1fr auto auto;
}

.param-label {
  font-size: 12px;
  color: #94a3b8;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.param-label code {
  font-size: 10px;
  color: #60a5fa;
  margin-left: 4px;
}

.rarity-label {
  font-size: 12px;
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

.rarity-dot {
  width: 10px;
  height: 10px;
  border-radius: 50%;
  flex-shrink: 0;
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

/* Chart area */
.chart-area {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.chart-toolbar {
  display: flex;
  align-items: center;
  gap: 16px;
}

.mode-tabs {
  display: flex;
  gap: 4px;
  background: #1e293b;
  border-radius: 6px;
  padding: 3px;
}

.tab {
  padding: 4px 12px;
  border: none;
  border-radius: 4px;
  cursor: pointer;
  font-size: 12px;
  background: transparent;
  color: #64748b;
  transition: background 0.15s, color 0.15s;
}

.tab.active {
  background: #334155;
  color: #f1f5f9;
}

.inline-ctrl {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 12px;
  color: #94a3b8;
}

.inline-ctrl select,
.inline-ctrl input[type="number"] {
  background: #1e293b;
  border: 1px solid #334155;
  border-radius: 4px;
  color: #e2e8f0;
  padding: 3px 6px;
  font-size: 12px;
}

.chart-svg {
  width: 100%;
  background: #0f172a;
  border-radius: 8px;
  border: 1px solid #1e293b;
}

/* Legend */
.legend {
  display: flex;
  flex-wrap: wrap;
  gap: 8px 16px;
}

.legend-item {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 12px;
  color: #94a3b8;
}

.legend-dot {
  width: 10px;
  height: 10px;
  border-radius: 50%;
  flex-shrink: 0;
}

.legend-end {
  color: #64748b;
  font-size: 11px;
}

/* Export */
.export-box {
  background: #0f172a;
  border: 1px solid #334155;
  border-radius: 8px;
  padding: 12px;
}

.export-box pre {
  margin: 0;
  font-size: 11px;
  color: #94a3b8;
  white-space: pre-wrap;
  word-break: break-all;
}
</style>
