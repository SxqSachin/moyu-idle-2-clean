<script setup lang="ts">
import { computed, reactive } from 'vue'
import type { CombatDifficultyLevel } from '../model/types'
import {
  DEFAULT_DIFFICULTY_LEVELS,
  difficultyAuthoringGuardrailsFromSharedGlobalConfig,
  difficultyConfigFromRaw,
  sharedGlobalConfigWithDifficulty,
  type RawSharedConfig
} from '../model/sharedScoring'
import {
  DEFAULT_ENEMY_DIFFICULTY_AUTHORING_GUARDRAILS,
  parseEnemyDifficultyAuthoringGuardrails
} from '../model/enemyDifficultyGuardrails'
import sharedGlobalConfig from '../../../packages/game-config/source/shared-global-config.json'

const emit = defineEmits<{
  (e: 'save', config: RawSharedConfig): void
}>()

const levels = reactive<CombatDifficultyLevel[]>(
  difficultyConfigFromRaw((sharedGlobalConfig as RawSharedConfig).combatDifficulty?.levels)
)
const guardrails = reactive(
  difficultyAuthoringGuardrailsFromSharedGlobalConfig(sharedGlobalConfig as RawSharedConfig)
)

const CHART_W = 600
const CHART_H = 360
const PAD = { top: 20, right: 20, bottom: 40, left: 50 }
const INNER_W = CHART_W - PAD.left - PAD.right
const INNER_H = CHART_H - PAD.top - PAD.bottom
const MAX_DIFFICULTY_MULTIPLIER = 6

function curveValue(lvl: CombatDifficultyLevel): number {
  const curve = lvl.statMultiplierCurve
  const base = Number(curve?.base ?? 1)
  const step = Number(curve?.step ?? 0)
  const overrides = curve?.overrides as Record<string, unknown> | undefined
  const overrideVal = overrides?.[String(lvl.level)]
  if (overrideVal != null) return Number(overrideVal)
  return base + step * (lvl.level - 1)
}

function setCurveValue(lvl: CombatDifficultyLevel, value: number) {
  const safeValue = Number.isFinite(value) ? Math.max(0.1, Math.min(MAX_DIFFICULTY_MULTIPLIER, value)) : 1
  const curve = { ...lvl.statMultiplierCurve }
  const overrides = { ...(curve.overrides as Record<string, unknown> || {}) } as Record<string, unknown>
  overrides[String(lvl.level)] = safeValue
  curve.overrides = overrides
  lvl.statMultiplierCurve = curve
}

function toSvgX(level: number) {
  return PAD.left + ((level - 1) / 9) * INNER_W
}
function toSvgY(value: number) {
  const minY = 0
  const maxY = chartMaxY.value
  return PAD.top + (1 - (value - minY) / (maxY - minY)) * INNER_H
}

const maxDifficultyMultiplier = computed(() => Math.max(...levels.map(curveValue)))
const chartMaxY = computed(() => Math.max(6, Math.ceil(maxDifficultyMultiplier.value + 0.5)))
const yGridValues = computed(() => Array.from(
  { length: 7 },
  (_, index) => Number((chartMaxY.value * index / 6).toFixed(2))
))
const enemyTypeRows = [
  { key: 'normal', label: '普通' },
  { key: 'elite', label: '精英' },
  { key: 'boss', label: '首领' },
  { key: 'world_boss', label: '世界首领' }
]
const chartPoints = computed(() =>
  levels.map((lvl) => ({
    level: lvl.level,
    value: curveValue(lvl),
    x: toSvgX(lvl.level),
    y: toSvgY(curveValue(lvl)),
  }))
)

const polylinePoints = computed(() =>
  chartPoints.value.map((p) => `${p.x.toFixed(1)},${p.y.toFixed(1)}`).join(' ')
)

function resetToDefaults() {
  DEFAULT_DIFFICULTY_LEVELS.forEach((def, i) => {
    Object.assign(levels[i], {
      labelKey: def.labelKey,
      statMultiplierCurve: { ...def.statMultiplierCurve, overrides: { ...(def.statMultiplierCurve.overrides as Record<string, unknown> || {}) } },
      battleExpBonus: def.battleExpBonus,
      battleDropRateBonus: def.battleDropRateBonus,
    })
  })
  Object.assign(guardrails, {
    minEnemyLevel: DEFAULT_ENEMY_DIFFICULTY_AUTHORING_GUARDRAILS.minEnemyLevel,
    defenseRetainRatio: DEFAULT_ENEMY_DIFFICULTY_AUTHORING_GUARDRAILS.defenseRetainRatio,
    defenseBaseCapsByEnemyType: { ...DEFAULT_ENEMY_DIFFICULTY_AUTHORING_GUARDRAILS.defenseBaseCapsByEnemyType },
    critDamageResistRetainRatio: DEFAULT_ENEMY_DIFFICULTY_AUTHORING_GUARDRAILS.critDamageResistRetainRatio,
    critDamageResistBaseFloorsByEnemyType: { ...DEFAULT_ENEMY_DIFFICULTY_AUTHORING_GUARDRAILS.critDamageResistBaseFloorsByEnemyType },
    critDamageResistBaseCapsByEnemyType: { ...DEFAULT_ENEMY_DIFFICULTY_AUTHORING_GUARDRAILS.critDamageResistBaseCapsByEnemyType }
  })
}

function commitSave() {
  const raw = { ...(sharedGlobalConfig as RawSharedConfig) }
  emit('save', sharedGlobalConfigWithDifficulty(raw, levels, parseEnemyDifficultyAuthoringGuardrails({
    minEnemyLevel: guardrails.minEnemyLevel,
    defenseRetainRatio: guardrails.defenseRetainRatio,
    defenseBaseCapsByEnemyType: { ...guardrails.defenseBaseCapsByEnemyType },
    critDamageResistRetainRatio: guardrails.critDamageResistRetainRatio,
    critDamageResistBaseFloorsByEnemyType: { ...guardrails.critDamageResistBaseFloorsByEnemyType },
    critDamageResistBaseCapsByEnemyType: { ...guardrails.critDamageResistBaseCapsByEnemyType }
  })))
}

const LEVEL_COLORS = [
  '#60a5fa', '#34d399', '#fbbf24', '#f97316', '#ef4444',
  '#a78bfa', '#ec4899', '#06b6d4', '#84cc16', '#f472b6'
]
</script>

<template>
  <div class="difficulty-editor-root">
    <div class="editor-layout">
      <!-- Left: Param table -->
      <aside class="param-panel">
        <div class="param-section">
          <div class="param-section-title">难度参数编辑</div>
          <div class="level-table">
            <div class="level-table-header">
              <span class="col-level">等级</span>
              <span class="col-mult">属性倍率</span>
              <span class="col-exp">经验+倍</span>
              <span class="col-drop">掉落+倍</span>
            </div>
            <div v-for="(lvl, i) in levels" :key="lvl.level" class="level-row">
              <span class="col-level font-mono">{{ lvl.level }}</span>
              <input class="col-mult input-num" type="number" min="0.1" :max="MAX_DIFFICULTY_MULTIPLIER" step="0.05"
                :value="curveValue(lvl)"
                @input="setCurveValue(lvl, Number(($event.target as HTMLInputElement).value))"
              />
              <input class="col-exp input-num" type="number" min="0" max="20" step="0.1"
                v-model.number="lvl.battleExpBonus"
              />
              <input class="col-drop input-num" type="number" min="0" max="10" step="0.05"
                v-model.number="lvl.battleDropRateBonus"
              />
            </div>
          </div>
        </div>

        <div class="param-section">
          <div class="param-section-title">敌人防御约束</div>
          <div class="guardrail-ratios">
            <label>启用等级
              <input class="input-num" type="number" min="1" step="1" v-model.number="guardrails.minEnemyLevel" />
            </label>
            <label>派生防御保留
              <input class="input-num" type="number" min="0" max="1" step="0.01" v-model.number="guardrails.defenseRetainRatio" />
            </label>
            <label>派生爆抗保留
              <input class="input-num" type="number" min="0" max="1" step="0.005" v-model.number="guardrails.critDamageResistRetainRatio" />
            </label>
          </div>
          <div class="guardrail-table">
            <div class="guardrail-head">
              <span>类型</span><span>防御上限</span><span>爆抗下限</span><span>爆抗上限</span><span>Lv10范围</span>
            </div>
            <div v-for="row in enemyTypeRows" :key="row.key" class="guardrail-row">
              <span>{{ row.label }}</span>
              <input class="input-num" type="number" min="0" step="5" v-model.number="guardrails.defenseBaseCapsByEnemyType[row.key]" />
              <input class="input-num" type="number" min="0" max="1" step="0.005" v-model.number="guardrails.critDamageResistBaseFloorsByEnemyType[row.key]" />
              <input class="input-num" type="number" min="0" max="1" step="0.005" v-model.number="guardrails.critDamageResistBaseCapsByEnemyType[row.key]" />
              <strong>{{ (guardrails.critDamageResistBaseFloorsByEnemyType[row.key] * maxDifficultyMultiplier * 100).toFixed(0) }}-{{ (guardrails.critDamageResistBaseCapsByEnemyType[row.key] * maxDifficultyMultiplier * 100).toFixed(0) }}%</strong>
            </div>
          </div>
        </div>

        <div class="param-actions">
          <button class="btn-secondary" @click="resetToDefaults">重置默认值</button>
          <button class="btn-primary" @click="commitSave">保存到文件</button>
        </div>
      </aside>

      <!-- Right: Chart -->
      <div class="chart-area">
        <svg :viewBox="`0 0 ${CHART_W} ${CHART_H}`" class="chart-svg" xmlns="http://www.w3.org/2000/svg">
          <!-- Grid -->
          <g class="grid">
            <line v-for="y in yGridValues" :key="`gy${y}`"
              :x1="PAD.left" :x2="PAD.left + INNER_W"
              :y1="toSvgY(y)" :y2="toSvgY(y)"
              stroke="#334155" stroke-width="1" :stroke-dasharray="y === 0 || y === yGridValues[yGridValues.length-1] ? 'none' : '4,3'"
            />
            <line v-for="lvl in levels" :key="`gx${lvl.level}`"
              :x1="toSvgX(lvl.level)" :x2="toSvgX(lvl.level)"
              :y1="PAD.top" :y2="PAD.top + INNER_H"
              stroke="#334155" stroke-width="1" stroke-dasharray="4,3"
            />
          </g>

          <!-- Y axis labels -->
          <g class="y-labels">
            <text v-for="y in yGridValues" :key="`yl${y}`"
              :x="PAD.left - 6" :y="toSvgY(y) + 4"
              text-anchor="end" font-size="10" fill="#94a3b8"
            >{{ y.toFixed(1) }}x</text>
          </g>

          <!-- X axis labels -->
          <g class="x-labels">
            <text v-for="lvl in levels" :key="`xl${lvl.level}`"
              :x="toSvgX(lvl.level)" :y="PAD.top + INNER_H + 16"
              text-anchor="middle" font-size="10" fill="#94a3b8"
            >Lv{{ lvl.level }}</text>
          </g>

          <!-- Axis titles -->
          <text :x="PAD.left + INNER_W / 2" :y="CHART_H - 4"
            text-anchor="middle" font-size="11" fill="#64748b">副本难度等级</text>
          <text :x="14" :y="PAD.top + INNER_H / 2"
            text-anchor="middle" font-size="11" fill="#64748b"
            transform="rotate(-90,14,180)">敌人属性倍率</text>

          <!-- Curve polyline -->
          <polyline
            :points="polylinePoints"
            fill="none" stroke="#60a5fa" stroke-width="2.5" stroke-linejoin="round"
          />

          <!-- Data points -->
          <g v-for="p in chartPoints" :key="p.level">
            <circle :cx="p.x" :cy="p.y" r="5" fill="#60a5fa" stroke="#0f172a" stroke-width="2" />
            <text :x="p.x" :y="p.y - 10" text-anchor="middle" font-size="10" fill="#e2e8f0">
              {{ p.value.toFixed(2) }}x
            </text>
          </g>
        </svg>

        <!-- Legend -->
        <div class="legend">
          <div class="legend-item">
            <span class="legend-dot" style="background:#60a5fa"></span>
            <span>属性倍率曲线</span>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.difficulty-editor-root {
  padding: 16px;
  color: #e2e8f0;
  font-size: 13px;
}
.editor-layout {
  display: flex;
  gap: 24px;
  align-items: flex-start;
}
.param-panel {
  flex: 0 0 520px;
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
.level-table-header,
.level-row {
  display: grid;
  grid-template-columns: 40px 80px 70px 70px;
  gap: 6px;
  align-items: center;
  font-size: 12px;
}
.guardrail-ratios {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 8px;
}
.guardrail-ratios label {
  display: grid;
  gap: 4px;
  color: #94a3b8;
  font-size: 11px;
}
.guardrail-head,
.guardrail-row {
  display: grid;
  grid-template-columns: 72px repeat(4, minmax(0, 1fr));
  gap: 6px;
  align-items: center;
}
.guardrail-head {
  margin-top: 4px;
  color: #64748b;
  font-size: 11px;
  font-weight: 600;
}
.guardrail-row {
  margin-top: 4px;
  color: #94a3b8;
}
.guardrail-row strong {
  color: #e2e8f0;
  text-align: right;
}
.level-table-header {
  color: #64748b;
  font-weight: 600;
  margin-bottom: 4px;
}
.level-row {
  margin-bottom: 3px;
}
.col-level { font-size: 12px; color: #94a3b8; }
.font-mono { font-family: monospace; }
.input-num {
  background: #0f172a;
  border: 1px solid #334155;
  border-radius: 4px;
  color: #f8fafc;
  padding: 3px 6px;
  font-size: 12px;
  width: 100%;
  box-sizing: border-box;
}
.input-text {
  background: #0f172a;
  border: 1px solid #334155;
  border-radius: 4px;
  color: #f8fafc;
  padding: 3px 6px;
  font-size: 12px;
  width: 100%;
  box-sizing: border-box;
}
.special-drop-group {
  border-top: 1px solid #334155;
  padding-top: 8px;
  margin-top: 4px;
}
.sd-level-label {
  font-size: 11px;
  color: #94a3b8;
  margin-bottom: 4px;
}
.sd-row {
  display: grid;
  grid-template-columns: 1fr 50px 60px 24px;
  gap: 4px;
  align-items: center;
  margin-bottom: 3px;
}
.sd-count, .sd-pct { width: 100%; }
.btn-icon {
  background: transparent;
  border: none;
  color: #ef4444;
  cursor: pointer;
  font-size: 12px;
  padding: 2px;
}
.btn-add {
  background: transparent;
  border: 1px dashed #334155;
  border-radius: 4px;
  color: #60a5fa;
  cursor: pointer;
  font-size: 11px;
  padding: 3px 8px;
  margin-top: 4px;
}
.btn-add:hover {
  border-color: #60a5fa;
}
.param-actions {
  display: flex;
  gap: 8px;
}
.btn-secondary, .btn-primary {
  flex: 1;
  padding: 6px 10px;
  border: 1px solid #334155;
  border-radius: 6px;
  cursor: pointer;
  font-size: 12px;
  transition: border-color 0.15s;
}
.btn-secondary {
  background: #1e293b;
  color: #94a3b8;
}
.btn-primary {
  background: #2563eb;
  color: #fff;
  border-color: #2563eb;
}
.btn-secondary:hover { border-color: #60a5fa; color: #e2e8f0; }
.btn-primary:hover { background: #1d4ed8; border-color: #1d4ed8; }

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
</style>
