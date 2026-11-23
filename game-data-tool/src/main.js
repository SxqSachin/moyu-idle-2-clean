import { createApp, computed, nextTick, onMounted, reactive, ref, watch } from "vue"
import * as echarts from "echarts"
import { TABLES } from "./core/schemaRegistry"
import { validateCatalog } from "./core/validation"
import { loadCatalog, requestGameDataDir, saveCatalog } from "./core/dataSourceAdapter"
import { runFormula } from "./core/formulaEngine"
import "./styles.css"

const CN = {
  id: "ID",
  nameKey: "名称Key",
  descKey: "描述Key",
  type: "类型",
  slot: "槽位",
  duration: "耗时(秒)",
  economyIndustryTier: "产业层级",
  economyDifficultyMultiplier: "难度系数",
  itemReward: "产出物",
  costs: "消耗",
  economyManualPricing: "手工定价",
  economyManualValue: "手工价值",
  baseAttrs: "基础属性",
  attrs: "属性"
}

function clone(v) { return JSON.parse(JSON.stringify(v)) }
function isObject(v) { return !!v && typeof v === "object" && !Array.isArray(v) }
function labelOf(key) { return CN[key] || key }

const JsonFieldEditor = {
  props: ["modelValue"],
  emits: ["update:modelValue"],
  methods: {
    update(v) { this.$emit("update:modelValue", v) },
    addObjectKey(value) {
      const key = window.prompt("新增字段名")
      if (!key) return
      if (Object.prototype.hasOwnProperty.call(value, key)) return
      this.update({ ...value, [key]: "" })
    }
  },
  template: `
    <div class="json-field">
      <template v-if="Array.isArray(modelValue)">
        <div class="json-block">
          <div class="json-header"><span>数组</span><button @click="update([...(modelValue||[]), ''])">新增元素</button></div>
          <div class="json-row" v-for="(it, idx) in modelValue" :key="idx">
            <span class="json-key">[{{idx}}]</span>
            <JsonFieldEditor :modelValue="it" @update:modelValue="(v)=>{const c=[...modelValue]; c[idx]=v; update(c)}" />
            <button @click="()=>{const c=[...modelValue]; c.splice(idx,1); update(c)}">删</button>
          </div>
        </div>
      </template>
      <template v-else-if="modelValue && typeof modelValue === 'object'">
        <div class="json-block">
          <div class="json-header"><span>对象</span><button @click="addObjectKey(modelValue)">新增字段</button></div>
          <div class="json-row" v-for="(v, k) in modelValue" :key="k">
            <span class="json-key">{{k}}</span>
            <JsonFieldEditor :modelValue="v" @update:modelValue="(nv)=>update({...modelValue,[k]:nv})" />
            <button @click="()=>{const c={...modelValue}; delete c[k]; update(c)}">删</button>
          </div>
        </div>
      </template>
      <input v-else-if="typeof modelValue === 'number'" type="number" :value="modelValue" @input="update(Number($event.target.value))" />
      <input v-else-if="typeof modelValue === 'boolean'" type="checkbox" :checked="modelValue" @change="update(Boolean($event.target.checked))" />
      <input v-else type="text" :value="modelValue ?? ''" @input="update($event.target.value)" />
    </div>
  `
}

const App = {
  components: { JsonFieldEditor },
  setup() {
    const state = reactive({
      dirReady: false, gameDataDir: null, table: "items", selectedIndex: -1,
      catalog: { items: [], actions: [], equipment_templates: [], enemies: [] },
      issues: [], formulaErrors: [], status: "未连接目录",
      history: [], historyIndex: -1,
      economy: {
        goldPerSecond: 1,
        expPerSecond: 1 / 8,
        attrWeightsText: "{\"atk\":1.5,\"def\":1,\"hp\":0.05}",
        itemValueFormula: "(Number(item.economyBaseValue)||0)+attrScore",
        actionValueFormula: "((rewardValue-costValue)/Math.max(duration,1))*goldPerSecond + expPerSecond"
      }
    })
    const chartEl = ref(null)
    let chart = null
    const currentRows = computed(() => state.catalog[state.table] || [])
    const selectedRow = computed(() => (state.selectedIndex >= 0 ? currentRows.value[state.selectedIndex] || null : null))
    const rowKeys = computed(() => Object.keys(selectedRow.value || {}))
    const scalarKeys = computed(() => rowKeys.value.filter((k) => {
      if (state.table === "actions" && (k === "itemReward" || k === "costs")) return false
      const v = selectedRow.value?.[k]
      return !isObject(v) && !Array.isArray(v)
    }))
    const complexKeys = computed(() => rowKeys.value.filter((k) => {
      if (state.table === "actions" && (k === "itemReward" || k === "costs")) return false
      const v = selectedRow.value?.[k]
      return isObject(v) || Array.isArray(v)
    }))
    const moduleSummaries = computed(() => TABLES.map((name) => ({ name, count: (state.catalog[name] || []).length })))
    const itemOptions = computed(() => (state.catalog.items || []).map((x) => x.id).filter(Boolean))
    const econInsights = reactive({ itemValues: {}, actionValues: {}, actionCostBreakdown: {}, actionRewardBreakdown: {} })

    function pushHistory() {
      state.history = state.history.slice(0, state.historyIndex + 1)
      state.history.push(clone(state.catalog))
      state.historyIndex = state.history.length - 1
    }
    function restoreHistory(i) {
      if (i < 0 || i >= state.history.length) return
      state.catalog = clone(state.history[i]); state.historyIndex = i; refreshSelection()
    }
    function refreshSelection() {
      const rows = currentRows.value
      if (!rows.length) state.selectedIndex = -1
      else if (state.selectedIndex < 0 || state.selectedIndex >= rows.length) state.selectedIndex = 0
      nextTick(renderChart)
    }
    async function connectDir() {
      try {
        state.gameDataDir = await requestGameDataDir()
        state.catalog = await loadCatalog(state.gameDataDir)
        state.dirReady = true
        pushHistory()
        state.status = "已加载数据"
        refreshSelection()
      } catch (e) { state.status = `目录授权失败: ${e.message || e}` }
    }
    function updateSelectedRow(row) {
      if (state.selectedIndex < 0) return
      const rows = clone(currentRows.value); rows[state.selectedIndex] = row
      state.catalog[state.table] = rows
      pushHistory()
      nextTick(renderChart)
    }
    function addRow() {
      state.catalog[state.table].push({ id: `${state.table}_new_${Date.now()}` }); pushHistory(); refreshSelection()
      state.selectedIndex = state.catalog[state.table].length - 1
    }
    function removeRow() {
      if (state.selectedIndex < 0) return
      state.catalog[state.table].splice(state.selectedIndex, 1); pushHistory(); refreshSelection()
    }
    function readWeights() { try { return JSON.parse(state.economy.attrWeightsText) } catch { return {} } }
    function attrScore(obj) {
      const w = readWeights(); let s = 0; const src = obj || {}
      for (const [k, v] of Object.entries(w)) s += Number(src[k] || 0) * Number(v || 0)
      return s
    }
    function readCountValue(v) {
      if (typeof v === "number") return v
      return Number(v?.count || v?.range?.min || 0) * Number(v?.percent ?? 1)
    }
    function sumItemMap(map, itemValueById) {
      if (!isObject(map)) return 0
      let sum = 0
      for (const [id, val] of Object.entries(map)) {
        const price = Number(itemValueById[id] || 0)
        if (typeof val === "number") sum += val * price
        else sum += Number(val?.count || val?.range?.min || 0) * Number(val?.percent ?? 1) * price
      }
      return sum
    }
    const producerActionMap = computed(() => {
      const out = {}
      for (const action of state.catalog.actions || []) {
        for (const itemId of Object.keys(action.itemReward || {})) {
          if (!out[itemId]) out[itemId] = []
          out[itemId].push(action.id)
        }
      }
      return out
    })
    async function predictActionValue(action, itemValueById) {
      const rewardValue = sumItemMap(action.itemReward, itemValueById)
      const costValue = sumItemMap(action.costs, itemValueById)
      const duration = Number(action.duration || action.baseDuration || 1)
      const industryTier = Number(action.economyIndustryTier || 1)
      const difficultyMultiplier = Number(action.economyDifficultyMultiplier || 1)
      return Number(await runFormula(state.economy.actionValueFormula, {
        action, rewardValue, costValue, duration, industryTier, difficultyMultiplier,
        goldPerSecond: Number(state.economy.goldPerSecond || 1),
        expPerSecond: Number(state.economy.expPerSecond || (1 / 8))
      }, 180) || 0)
    }
    async function recomputeEconomy() {
      const actions = state.catalog.actions || []
      const items = state.catalog.items || []
      const itemValues = {}
      for (const item of items) {
        const manual = Boolean(item.economyManualPricing)
        if (manual) {
          itemValues[item.id] = Number(item.economyManualValue || 0)
          continue
        }
        itemValues[item.id] = 0
      }
      for (let i = 0; i < 4; i++) {
        const actionValues = {}
        const actionCostBreakdown = {}
        const actionRewardBreakdown = {}
        for (const action of actions) {
          const id = action.id || ""
          const costs = {}
          for (const [itemId, val] of Object.entries(action.costs || {})) {
            const qty = readCountValue(val)
            costs[itemId] = { qty, unitValue: Number(itemValues[itemId] || 0), value: qty * Number(itemValues[itemId] || 0) }
          }
          const rewards = {}
          for (const [itemId, val] of Object.entries(action.itemReward || {})) {
            const qty = readCountValue(val)
            rewards[itemId] = { qty, unitValue: Number(itemValues[itemId] || 0), value: qty * Number(itemValues[itemId] || 0) }
          }
          actionCostBreakdown[id] = costs
          actionRewardBreakdown[id] = rewards
          actionValues[id] = await predictActionValue(action, itemValues).catch(() => 0)
        }
        for (const item of items) {
          if (item.economyManualPricing) continue
          const producers = actions.filter((a) => isObject(a.itemReward) && Object.prototype.hasOwnProperty.call(a.itemReward, item.id))
          if (!producers.length) {
            try { itemValues[item.id] = Number(await runFormula(state.economy.itemValueFormula, { item, attrScore: attrScore(item.baseAttrs || {}) }, 120) || 0) }
            catch { itemValues[item.id] = 0 }
            continue
          }
          const implied = producers.map((a) => {
            const rewardQty = readCountValue(a.itemReward[item.id])
            const total = Number(actionValues[a.id] || 0)
            return rewardQty > 0 ? total / rewardQty : total
          })
          itemValues[item.id] = implied.reduce((s, x) => s + x, 0) / Math.max(implied.length, 1)
        }
        econInsights.actionValues = actionValues
        econInsights.actionCostBreakdown = actionCostBreakdown
        econInsights.actionRewardBreakdown = actionRewardBreakdown
      }
      econInsights.itemValues = itemValues
    }
    async function validateAll() {
      state.issues = validateCatalog(state.catalog)
      state.formulaErrors = []
      try {
        await recomputeEconomy()
      } catch (e) { state.formulaErrors.push(String(e?.message || e)) }
      state.status = (state.issues.length || state.formulaErrors.length) ? `校验失败: ${state.issues.length + state.formulaErrors.length}` : "校验通过"
    }
    async function saveAll() {
      await validateAll()
      if (state.issues.length || state.formulaErrors.length) return
      await saveCatalog(state.gameDataDir, state.catalog)
      state.status = "保存成功"
    }

    async function renderChart() {
      if (!chartEl.value) return
      if (!chart) chart = echarts.init(chartEl.value)
      const labels = currentRows.value.map((r, i) => r.id || `#${i + 1}`)
      let values = currentRows.value.map(() => 0)
      await recomputeEconomy()
      if (state.table === "items") values = currentRows.value.map((r) => Number(econInsights.itemValues[r.id] || 0))
      else if (state.table === "actions") values = currentRows.value.map((r) => Number(econInsights.actionValues[r.id] || 0))
      else values = currentRows.value.map((r) => attrScore(r.baseAttrs || r.attrs || {}))
      chart.setOption({ xAxis: { type: "category", data: labels }, yAxis: { type: "value" }, grid: { left: 40, right: 20, top: 20, bottom: 45 }, tooltip: { trigger: "axis" }, series: [{ type: "line", smooth: true, data: values }] })
    }

    function setField(key, value) { if (!selectedRow.value) return; updateSelectedRow({ ...selectedRow.value, [key]: value }) }
    function ensureObjectField(key) { if (!isObject(selectedRow.value?.[key])) setField(key, {}) }
    function setMapValue(field, itemId, value) {
      ensureObjectField(field)
      const row = clone(selectedRow.value); row[field][itemId] = Number(value || 0); updateSelectedRow(row)
    }
    function removeMapItem(field, itemId) {
      const row = clone(selectedRow.value); if (row[field]) delete row[field][itemId]; updateSelectedRow(row)
    }
    function addMapItem(field) {
      const pick = window.prompt("输入 item id")
      if (!pick) return
      ensureObjectField(field)
      const row = clone(selectedRow.value); row[field][pick] = 1; updateSelectedRow(row)
    }

    watch(() => state.table, refreshSelection)
    watch(() => state.catalog, () => nextTick(renderChart), { deep: true })
    onMounted(() => renderChart())

    const selectedItemValue = computed(() => Number(econInsights.itemValues[selectedRow.value?.id] || 0))
    const selectedActionValue = computed(() => Number(econInsights.actionValues[selectedRow.value?.id] || 0))
    return { state, TABLES, currentRows, selectedRow, rowKeys, scalarKeys, complexKeys, moduleSummaries, itemOptions, producerActionMap, econInsights, selectedItemValue, selectedActionValue, connectDir, restoreHistory, addRow, removeRow, setField, validateAll, saveAll, selectRow: (i) => { state.selectedIndex = i; nextTick(renderChart) }, addMapItem, removeMapItem, setMapValue, labelOf, updateSelectedRow }
  },
  template: `
  <div class="page">
    <header class="topbar">
      <h1>Game Data Tool</h1>
      <div class="topbar-actions">
        <button @click="connectDir">授权并加载</button><button :disabled="!state.dirReady" @click="validateAll">全量校验</button><button :disabled="!state.dirReady" @click="saveAll">保存</button>
        <button :disabled="state.historyIndex<=0" @click="restoreHistory(state.historyIndex-1)">撤销</button><button :disabled="state.historyIndex>=state.history.length-1" @click="restoreHistory(state.historyIndex+1)">重做</button>
      </div><p class="status">{{ state.status }}</p>
    </header>
    <div class="layout">
      <aside class="left"><h3>模块</h3><ul class="row-list"><li v-for="x in moduleSummaries" :key="x.name" :class="{active:state.table===x.name}" @click="state.table=x.name">{{x.name}} ({{x.count}})</li></ul><div class="list-actions"><button @click="addRow">新增</button><button @click="removeRow" :disabled="state.selectedIndex<0">删除</button></div><h3>列表</h3><ul class="row-list"><li v-for="(row,idx) in currentRows" :key="row.id||idx" :class="{active:idx===state.selectedIndex}" @click="selectRow(idx)">{{row.id||'(无id)'}}</li></ul></aside>
      <main class="center">
        <section class="editor" v-if="selectedRow">
          <h2>字段编辑</h2>
          <div class="compact-grid">
            <div v-for="key in scalarKeys" :key="'s'+key">
              <label>{{labelOf(key)}}</label>
              <input v-if="typeof selectedRow[key] !== 'boolean'" :type="typeof selectedRow[key] === 'number' ? 'number' : 'text'" :value="selectedRow[key] ?? ''" @input="setField(key, typeof selectedRow[key] === 'number' ? Number($event.target.value) : $event.target.value)" />
              <input v-else type="checkbox" :checked="selectedRow[key]" @change="setField(key, Boolean($event.target.checked))" />
            </div>
          </div>
          <div v-for="key in complexKeys" :key="'c'+key" class="complex-block">
            <label>{{labelOf(key)}}</label>
            <JsonFieldEditor :modelValue="selectedRow[key]" @update:modelValue="(v)=>setField(key,v)" />
          </div>
          <template v-if="state.table==='actions'">
            <h3>产出物(itemReward)</h3>
            <div class="map-table"><div class="map-row value-row" v-for="(v,k) in (selectedRow.itemReward||{})" :key="'r'+k"><input list="items-list" :value="k" @change="removeMapItem('itemReward',k); setMapValue('itemReward',$event.target.value,v)" /><input type="number" :value="typeof v==='number'?v:(v?.count||0)" @input="setMapValue('itemReward',k,$event.target.value)" /><span class="value-note">预测价值: {{ (econInsights.actionRewardBreakdown[selectedRow.id]?.[k]?.value ?? 0).toFixed(3) }}</span><button @click="removeMapItem('itemReward',k)">删</button></div><button @click="addMapItem('itemReward')">添加产出</button></div>
            <h3>消耗(costs)</h3>
            <div class="map-table"><div class="map-row value-row" v-for="(v,k) in (selectedRow.costs||{})" :key="'c'+k"><input list="items-list" :value="k" @change="removeMapItem('costs',k); setMapValue('costs',$event.target.value,v)" /><input type="number" :value="typeof v==='number'?v:(v?.count||0)" @input="setMapValue('costs',k,$event.target.value)" /><span class="value-note">消耗价值: {{ (econInsights.actionCostBreakdown[selectedRow.id]?.[k]?.value ?? 0).toFixed(3) }}</span><button @click="removeMapItem('costs',k)">删</button></div><button @click="addMapItem('costs')">添加消耗</button></div>
            <p class="hint">动作预测总价值: {{ selectedActionValue.toFixed(3) }}</p>
          </template>
          <template v-if="state.table==='items'">
            <h3>来源 Action</h3><p class="hint">{{ (producerActionMap[selectedRow.id]||[]).join(', ') || '无' }}</p>
            <div class="compact-grid">
              <div><label>联动价值</label><input :value="selectedItemValue.toFixed(3)" disabled /></div>
              <div><label>手工定价</label><input type="checkbox" :checked="Boolean(selectedRow.economyManualPricing)" @change="setField('economyManualPricing', Boolean($event.target.checked))" /></div>
              <div><label>手工价值</label><input type="number" :value="selectedRow.economyManualValue ?? 0" @input="setField('economyManualValue', Number($event.target.value))" :disabled="!selectedRow.economyManualPricing" /></div>
            </div>
          </template>
        </section>
        <section class="editor" v-else><p>当前模块没有数据。</p></section>
        <section class="chart"><h2>价值/强度曲线</h2><div ref="chartEl" class="chart-box"></div></section>
      </main>
      <aside class="right">
        <section><h3>经济与强度基准</h3><label>金币基准(每秒)</label><input type="number" v-model.number="state.economy.goldPerSecond" /><label>经验基准(每秒)</label><input type="number" v-model.number="state.economy.expPerSecond" /><label>属性价值权重(JSON)</label><textarea v-model="state.economy.attrWeightsText"></textarea><label>Item估值公式</label><textarea v-model="state.economy.itemValueFormula"></textarea><label>Action估值公式</label><textarea v-model="state.economy.actionValueFormula"></textarea></section>
        <section><h3>问题</h3><ul class="issues"><li v-for="(x,i) in state.issues" :key="'i'+i">{{x.table}}/{{x.field}} {{x.message}}</li><li v-for="(x,i) in state.formulaErrors" :key="'f'+i">{{x}}</li></ul></section>
      </aside>
    </div>
    <datalist id="items-list"><option v-for="id in itemOptions" :key="id" :value="id"></option></datalist>
  </div>
  `
}

createApp(App).mount("#app")
