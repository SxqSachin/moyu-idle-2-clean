<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue';
import * as echarts from 'echarts/core';
import { DataZoomComponent, GridComponent, LegendComponent, TooltipComponent } from 'echarts/components';
import { LineChart } from 'echarts/charts';
import { CanvasRenderer } from 'echarts/renderers';
import {
  loadAttributeExpProfiles,
  saveAttributeExpProfiles,
  type AttributeExpProfileRow,
  type GameDataDirectoryHandle
} from '../services/gameDataFs';

echarts.use([GridComponent, TooltipComponent, DataZoomComponent, LegendComponent, LineChart, CanvasRenderer]);

const props = defineProps<{
  gameDataDir: GameDataDirectoryHandle | null;
}>();

const emit = defineEmits<{
  (e: 'status', message: string): void;
}>();

type CurveParams = {
  maxLevel: number;
  earlyStart: number;
  earlyEnd: number;
  earlyPower: number;
  midStart: number;
  midEnd: number;
  midPower: number;
  lateStart: number;
  lateEnd: number;
  latePower: number;
  enemyExp: number;
  killMinutes: number;
  dailyHours: number;
  futureMultiplier: number;
  previewTotalExp: number;
};

type Progress = {
  level: number;
  exp: number;
  nextExp: number;
  progressRatio: number;
};

const params = reactive<CurveParams>({
  maxLevel: 500,
  earlyStart: 50,
  earlyEnd: 35000,
  earlyPower: 0.48,
  midStart: 40000,
  midEnd: 105000,
  midPower: 0.82,
  lateStart: 105000,
  lateEnd: 4185288,
  latePower: 0.73,
  enemyExp: 1317,
  killMinutes: 10,
  dailyHours: 24,
  futureMultiplier: 3,
  previewTotalExp: 8420303
});

const LEGACY_BATTLE_EXP_CURVE = [
  3, 7, 12, 19, 29, 40, 52, 67, 85, 104, 126, 150, 176, 205, 237, 271,
  308, 348, 390, 436, 485, 537, 593, 651, 714, 779, 849, 922, 999, 1080, 1165, 1254,
  1347, 1445, 1546, 1653, 1764, 1880, 2000, 2125, 2256, 2391, 2532, 2678, 2829, 2986, 3148, 3316,
  3490, 3670, 3856, 4048, 4246, 4450, 4661, 4879, 5103, 5334, 5571, 5816, 6068, 6327, 6593, 6867,
  7148, 7437, 7734, 8038, 8351, 8672, 9000, 9337, 9683, 10037, 10400, 10772, 11152, 11542, 11940, 12348,
  12765, 13192, 13628, 14074, 14530, 14996, 15472, 15958, 16454, 16961, 17479, 18007, 18545, 19095, 19656, 20228,
  20811, 21406, 22012, 22630, 23260, 23901, 24555, 25220, 25898, 26589, 27292, 28007, 28736, 29477, 30231, 30999,
  31779, 32573, 33381, 34202, 35038, 35887, 36750, 37627, 38519, 39425, 40345, 41281, 42231, 43196, 44176, 45171,
  46182, 47209, 48250, 49308, 50382, 51471, 52577, 53699, 54837, 55992, 57164, 58352, 59557, 60780, 62019, 63276,
  64551, 65843, 67153, 68481, 69826, 71190, 72573, 73973, 75393, 76831, 78287, 79763, 81258, 82773, 84306, 85859,
  87432, 89025, 90637, 92270, 93923, 95596, 97290, 99005, 100740, 102496, 104274, 106072, 107892, 109734, 111597, 113481,
  115388, 117317, 119268, 121241, 123237, 125256, 127297, 129361, 131449, 133559, 135693, 137850, 140031, 142236, 144465, 146718,
  148995, 151296, 153622, 155972, 158348, 160748, 163173, 165624, 168100, 170602, 173129, 175682, 178261, 180866, 183498, 186156,
  188840, 239315, 243034, 246794, 250597, 254441, 258327, 262256, 266228, 270244, 274302, 278405, 282551, 286742, 290977, 295258,
  299583, 303954, 308371, 312833, 317342, 321898, 326501, 331151, 335848, 340593, 345386, 350228, 355118, 360058, 365046, 370085,
  375173, 380311, 385500, 390739, 396030, 401372, 406766, 412212, 417710, 423260, 428864, 434521, 440231, 445995, 451813, 457686,
  463613, 469596, 475634, 481727, 487877, 494083, 500345, 506665, 513041, 519475, 525967, 532518, 539126, 545794, 552521, 559307,
  566153, 573059, 580025, 587053, 594141, 601290, 608502, 615775, 623110, 630509, 637970, 645495, 653083, 660735, 668451, 676232,
  684078, 691990, 699967, 708009, 716118, 724294, 732537, 740846, 749224, 757669, 766182, 774765, 783416, 792136, 800926, 809785,
  818715, 827716, 836787, 845930, 855145, 864431, 873789, 883221, 892725, 902302, 911953, 921678, 931478, 941352, 951301, 961326,
  971426, 981602, 991855, 1002184, 1012590, 1023074, 1033636, 1044276, 1054994, 1065791, 1076667, 1087623, 1098659, 1109775, 1120972, 1132249,
  1143608, 1155048, 1166571, 1178176, 1189863, 1201634, 1213488, 1225425, 1237447, 1249554, 1261745, 1274022, 1286384, 1298832, 1311366, 1323987,
  1336695, 1349491, 1362374, 1375345, 1388405, 1401553, 1414791, 1428118, 1441536, 1455043, 1468642, 1482331, 1496111, 1509984, 1523949, 1538006,
  1552156, 1566399, 1580736, 1595166, 1609692, 1624312, 1639027, 1653837, 1668743, 1683746, 1698845, 1714041, 1729335, 1744726, 1760216, 1775803,
  1791490, 1807276, 1823162, 1839147, 1855233, 1871420, 1887708, 1904097, 1920588, 1937182, 1953878, 1970678, 1987580, 2004587, 2021698, 2038913,
  3660164, 3694699, 3729476, 3764495, 3799759, 3835268, 3871024, 3907027, 3943278, 3979779, 4016531, 4053535, 4090792, 4128303, 4166069, 4204092,
  4242372, 4280911, 4319710, 4358770, 4398092, 4437677, 4477527, 4517642, 4558024, 4598674, 4639592, 4680781, 4722242, 4763975, 4805981, 4848263,
  4890820, 4933655, 4976768, 5020161, 5063834, 5107789, 5152028, 5196550, 5241358, 5286453, 5331836, 5377508, 5423470, 5469723, 5516269, 5563109,
  5610244, 5657675, 5705404, 5753432, 5801759, 5850388, 5899319, 5948553, 5998093, 6047938, 6098091, 6148552, 6199323, 6250405, 6301799, 6353507,
  6405529, 6457867, 6510523, 6563497, 6616790, 6670405, 6724342, 6778602, 6833187, 6888098, 6943337, 6998904, 7054800, 7111028, 7167588, 7224482,
  7281711, 7339276, 7397178, 7455420, 7514001, 7572924, 7632189, 7691798, 7751753, 7812054, 7872703, 7933701, 7995050, 8056750, 8118804, 8181211,
  8243975, 8307096, 8370575, 8434414
];

const busy = ref(false);
const localStatus = ref('旧曲线已内置为调整前 battle 基线；写入时会覆盖 source/json 中 battle 的 500 行。');
const reqChartEl = ref<HTMLDivElement | null>(null);
const cumulativeChartEl = ref<HTMLDivElement | null>(null);
let reqChart: echarts.ECharts | null = null;
let cumulativeChart: echarts.ECharts | null = null;

function clampNumber(value: number, min: number, max: number) {
  if (!Number.isFinite(value)) return min;
  return Math.max(min, Math.min(max, value));
}

function safeRatio(start: number, end: number) {
  return Math.max(0.000001, end) / Math.max(0.000001, start);
}

function battleExpRequired(level: number, p = params): number {
  const maxLevel = Math.max(201, Math.round(p.maxLevel));
  if (level >= maxLevel) return 0;
  if (level < 100) {
    const x = clampNumber((level - 1) / 98, 0, 1);
    return Math.max(1, Math.round(p.earlyStart * Math.pow(safeRatio(p.earlyStart, p.earlyEnd), Math.pow(x, p.earlyPower))));
  }
  if (level < 200) {
    const x = clampNumber((level - 100) / 99, 0, 1);
    return Math.max(1, Math.round(p.midStart * Math.pow(safeRatio(p.midStart, p.midEnd), Math.pow(x, p.midPower))));
  }
  const x = clampNumber((level - 200) / Math.max(1, maxLevel - 201), 0, 1);
  return Math.max(1, Math.round(p.lateStart * Math.pow(safeRatio(p.lateStart, p.lateEnd), Math.pow(x, p.latePower))));
}

function buildCurveRows(p = params): AttributeExpProfileRow[] {
  const maxLevel = Math.max(1, Math.round(p.maxLevel));
  const rows: AttributeExpProfileRow[] = [];
  for (let level = 1; level <= maxLevel; level += 1) {
    rows.push({
      id: 'battle',
      attr: 'battle',
      level,
      expRequired: level >= maxLevel ? 0 : battleExpRequired(level, p)
    });
  }
  return rows;
}

const newBattleRows = computed(() => buildCurveRows());
const newCurve = computed(() => newBattleRows.value.map((row) => row.expRequired));
const oldBattleRows = computed(() =>
  LEGACY_BATTLE_EXP_CURVE.map((expRequired, index) => ({
    id: 'battle',
    attr: 'battle',
    level: index + 1,
    expRequired
  }))
);
const oldCurve = computed(() => {
  const out = new Map<number, number>();
  for (const row of oldBattleRows.value) out.set(row.level, row.expRequired);
  return out;
});

function cumulativeBefore(curve: number[], level: number): number {
  let total = 0;
  for (let index = 0; index < Math.max(0, level - 1) && index < curve.length; index += 1) {
    total += Math.max(0, curve[index] || 0);
  }
  return total;
}

function progressFromTotal(curve: number[], totalExp: number, maxLevel = params.maxLevel): Progress {
  let remaining = Math.max(0, totalExp || 0);
  const cap = Math.max(1, Math.round(maxLevel));
  for (let level = 1; level < cap; level += 1) {
    const required = Math.max(0, curve[level - 1] || 0);
    if (required <= 0) return { level, exp: 0, nextExp: 0, progressRatio: 0 };
    if (remaining < required) {
      return { level, exp: remaining, nextExp: required, progressRatio: required > 0 ? remaining / required : 0 };
    }
    remaining -= required;
  }
  return { level: cap, exp: 0, nextExp: 0, progressRatio: 1 };
}

const totalExpToCap = computed(() => cumulativeBefore(newCurve.value, params.maxLevel));
const expPerDay = computed(() => {
  const killsPerDay = params.killMinutes > 0 ? (params.dailyHours * 60) / params.killMinutes : 0;
  return Math.max(0, params.enemyExp * killsPerDay);
});
const futureExpPerDay = computed(() => expPerDay.value * Math.max(0, params.futureMultiplier));

const keyLevels = computed(() => {
  const levels = [1, 24, 49, 74, 99, 100, 124, 149, 174, 199, 200, 224, 249, 299, 349, 399, 449, 499]
    .filter((level) => level <= params.maxLevel);
  return levels.map((level) => {
    const req = battleExpRequired(level);
    const oldReq = oldCurve.value.get(level) ?? 0;
    return {
      level,
      targetLevel: Math.min(level + 1, params.maxLevel),
      req,
      oldReq,
      cumulative: cumulativeBefore(newCurve.value, Math.min(level + 1, params.maxLevel)),
      levelsPerDay: req > 0 ? expPerDay.value / req : 0,
      futureLevelsPerDay: req > 0 ? futureExpPerDay.value / req : 0,
      killsPerLevel: params.enemyExp > 0 ? req / params.enemyExp : 0,
      hoursPerLevel: expPerDay.value > 0 ? req / expPerDay.value * params.dailyHours : 0
    };
  });
});

const recomputePreview = computed(() => {
  const oldCurveArray = oldBattleRows.value.length
    ? Array.from({ length: params.maxLevel }, (_, index) => oldCurve.value.get(index + 1) ?? 0)
    : newCurve.value;
  const oldProgress = progressFromTotal(oldCurveArray, params.previewTotalExp);
  const newProgress = progressFromTotal(newCurve.value, params.previewTotalExp);
  return { oldProgress, newProgress, levelDelta: newProgress.level - oldProgress.level };
});

const chartRows = computed(() => {
  const out: Array<{ level: number; targetLevel: number; req: number; cumulative: number; oldReq: number; oldCumulative: number; ratio: number | null }> = [];
  const maxLevel = Math.max(1, Math.round(params.maxLevel));
  let newTotal = 0;
  let oldTotal = 0;
  for (let level = 1; level < maxLevel; level += 1) {
    const req = battleExpRequired(level);
    const oldReq = oldCurve.value.get(level) ?? 0;
    newTotal += req;
    oldTotal += Math.max(0, oldReq);
    out.push({
      level,
      targetLevel: level + 1,
      req,
      cumulative: newTotal,
      oldReq,
      oldCumulative: oldTotal,
      ratio: oldReq > 0 ? req / oldReq : null
    });
  }
  return out;
});

function formatNumber(value: number, digits = 0) {
  if (!Number.isFinite(value)) return '-';
  return value.toLocaleString('zh-CN', { maximumFractionDigits: digits, minimumFractionDigits: 0 });
}

function formatDecimal(value: number, digits = 2) {
  if (!Number.isFinite(value)) return '-';
  return value.toLocaleString('zh-CN', { maximumFractionDigits: digits, minimumFractionDigits: digits });
}

function axisNumber(value: number) {
  if (value >= 100000000) return `${formatDecimal(value / 100000000, 1)}亿`;
  if (value >= 10000) return `${formatDecimal(value / 10000, 1)}万`;
  return formatNumber(value);
}

function tooltipNumber(value: unknown) {
  return typeof value === 'number' ? formatNumber(value, 2) : '-';
}

const commonChartStyle = {
  backgroundColor: '#0b1120',
  textStyle: { color: '#cbd5e1' },
  grid: { left: 72, right: 24, top: 44, bottom: 72 },
  dataZoom: [
    { type: 'inside', throttle: 30 },
    { type: 'slider', height: 24, bottom: 24, borderColor: '#334155', fillerColor: 'rgba(56,189,248,0.18)' }
  ]
};

const reqChartOption = computed(() => ({
  ...commonChartStyle,
  color: ['#38bdf8', '#94a3b8', '#f59e0b'],
  tooltip: {
    trigger: 'axis',
    axisPointer: { type: 'cross' },
    valueFormatter: tooltipNumber
  },
  legend: { top: 8, textStyle: { color: '#cbd5e1' } },
  xAxis: {
    type: 'value',
    name: '升级到等级',
    min: 2,
    max: Math.max(2, params.maxLevel),
    axisLabel: { color: '#94a3b8', formatter: (value: number) => `Lv${value}` },
    splitLine: { lineStyle: { color: '#26364d' } }
  },
  yAxis: [
    {
      type: 'log',
      name: '单级经验',
      min: 1,
      axisLabel: { color: '#94a3b8', formatter: axisNumber },
      splitLine: { lineStyle: { color: '#26364d' } }
    },
    {
      type: 'value',
      name: '新/旧',
      min: 0,
      axisLabel: { color: '#94a3b8', formatter: (value: number) => `${formatDecimal(value, 1)}x` },
      splitLine: { show: false }
    }
  ],
  series: [
    {
      name: '新单级经验',
      type: 'line',
      showSymbol: false,
      smooth: true,
      data: chartRows.value.map((row) => [row.targetLevel, row.req])
    },
    {
      name: '旧单级经验',
      type: 'line',
      showSymbol: false,
      smooth: true,
      lineStyle: { type: 'dashed' },
      data: chartRows.value.filter((row) => row.oldReq > 0).map((row) => [row.targetLevel, row.oldReq])
    },
    {
      name: '新/旧比例',
      type: 'line',
      yAxisIndex: 1,
      showSymbol: false,
      smooth: true,
      data: chartRows.value.filter((row) => row.ratio != null).map((row) => [row.targetLevel, row.ratio])
    }
  ]
}));

const cumulativeChartOption = computed(() => ({
  ...commonChartStyle,
  color: ['#22c55e', '#94a3b8'],
  tooltip: {
    trigger: 'axis',
    axisPointer: { type: 'cross' },
    valueFormatter: tooltipNumber
  },
  legend: { top: 8, textStyle: { color: '#cbd5e1' } },
  xAxis: {
    type: 'value',
    name: '等级',
    min: 2,
    max: Math.max(2, params.maxLevel),
    axisLabel: { color: '#94a3b8', formatter: (value: number) => `Lv${value}` },
    splitLine: { lineStyle: { color: '#26364d' } }
  },
  yAxis: {
    type: 'log',
    name: '累计经验',
    min: 1,
    axisLabel: { color: '#94a3b8', formatter: axisNumber },
    splitLine: { lineStyle: { color: '#26364d' } }
  },
  series: [
    {
      name: '新累计经验',
      type: 'line',
      showSymbol: false,
      smooth: true,
      data: chartRows.value.map((row) => [row.targetLevel, row.cumulative])
    },
    {
      name: '旧累计经验',
      type: 'line',
      showSymbol: false,
      smooth: true,
      lineStyle: { type: 'dashed' },
      data: chartRows.value.filter((row) => row.oldCumulative > 0).map((row) => [row.targetLevel, row.oldCumulative])
    }
  ]
}));

function renderCharts() {
  reqChart?.setOption(reqChartOption.value, true);
  cumulativeChart?.setOption(cumulativeChartOption.value, true);
}

function resizeCharts() {
  reqChart?.resize();
  cumulativeChart?.resize();
}

async function saveBattleCurve() {
  if (!props.gameDataDir) {
    localStatus.value = '请先授权 packages/game-data 目录。';
    return;
  }
  busy.value = true;
  try {
    const existing = await loadAttributeExpProfiles(props.gameDataDir);
    const withoutBattle = existing.filter((row) => row.id !== 'battle');
    const next = [...withoutBattle, ...newBattleRows.value].sort((a, b) => a.id.localeCompare(b.id) || a.level - b.level);
    await saveAttributeExpProfiles(props.gameDataDir, next);
    localStatus.value = `已写入 source/json/attribute_exp_profiles.json：battle ${newBattleRows.value.length} 行，保留其他 ${withoutBattle.length} 行。`;
    emit('status', localStatus.value);
  } catch (error) {
    localStatus.value = `写入失败: ${error instanceof Error ? error.message : String(error)}`;
  } finally {
    busy.value = false;
  }
}

function resetDefaults() {
  Object.assign(params, {
    maxLevel: 500,
    earlyStart: 50,
    earlyEnd: 35000,
    earlyPower: 0.48,
    midStart: 40000,
    midEnd: 105000,
    midPower: 0.82,
    lateStart: 105000,
    lateEnd: 4185288,
    latePower: 0.73,
    enemyExp: 1317,
    killMinutes: 10,
    dailyHours: 24,
    futureMultiplier: 3
  });
}

watch([reqChartOption, cumulativeChartOption], renderCharts, { deep: true });

onMounted(() => {
  void nextTick(() => {
    if (reqChartEl.value) reqChart = echarts.init(reqChartEl.value);
    if (cumulativeChartEl.value) cumulativeChart = echarts.init(cumulativeChartEl.value);
    renderCharts();
    window.addEventListener('resize', resizeCharts);
  });
});

onBeforeUnmount(() => {
  window.removeEventListener('resize', resizeCharts);
  reqChart?.dispose();
  cumulativeChart?.dispose();
  reqChart = null;
  cumulativeChart = null;
});
</script>

<template>
  <div class="battle-exp-root">
    <div class="header-row">
      <div>
        <h2>战斗经验曲线</h2>
        <p>生成 battle 的 attribute_exp_profiles 覆盖行，并模拟当前与未来副本效率。</p>
      </div>
      <div class="actions">
        <button class="btn-secondary" @click="resetDefaults">重置默认</button>
        <button class="btn-primary" :disabled="busy" @click="saveBattleCurve">写入 battle 曲线覆盖</button>
      </div>
    </div>

    <div class="status-line">{{ localStatus }}</div>

    <div class="layout">
      <aside class="panel params-panel">
        <div class="panel-title">曲线参数</div>
        <label><span>等级上限</span><input type="number" v-model.number="params.maxLevel" min="201" max="1000" step="1" /></label>
        <label><span>升到 Lv2</span><input type="number" v-model.number="params.earlyStart" min="1" step="1" /></label>
        <label><span>升到 Lv100</span><input type="number" v-model.number="params.earlyEnd" min="1" step="1000" /></label>
        <label><span>Lv1-100 power</span><input type="number" v-model.number="params.earlyPower" min="0.2" max="3" step="0.01" /></label>
        <label><span>升到 Lv101</span><input type="number" v-model.number="params.midStart" min="1" step="1000" /></label>
        <label><span>升到 Lv200</span><input type="number" v-model.number="params.midEnd" min="1" step="1000" /></label>
        <label><span>Lv100-200 power</span><input type="number" v-model.number="params.midPower" min="0.2" max="3" step="0.01" /></label>
        <label><span>升到 Lv201</span><input type="number" v-model.number="params.lateStart" min="1" step="1000" /></label>
        <label><span>升到 Lv{{ params.maxLevel }}</span><input type="number" v-model.number="params.lateEnd" min="1" step="10000" /></label>
        <label><span>Lv200-500 power</span><input type="number" v-model.number="params.latePower" min="0.2" max="4" step="0.01" /></label>

        <div class="panel-title spaced">效率模拟</div>
        <label><span>敌人经验</span><input type="number" v-model.number="params.enemyExp" min="0" step="1" /></label>
        <label><span>击杀分钟</span><input type="number" v-model.number="params.killMinutes" min="0.1" step="0.1" /></label>
        <label><span>每日小时</span><input type="number" v-model.number="params.dailyHours" min="0" max="24" step="0.5" /></label>
        <label><span>未来倍率</span><input type="number" v-model.number="params.futureMultiplier" min="0" step="0.1" /></label>
      </aside>

      <section class="main-area">
        <div class="stats-grid">
          <div class="stat">
            <span>当前效率/天</span>
            <strong>{{ formatNumber(expPerDay) }}</strong>
          </div>
          <div class="stat">
            <span>未来效率/天</span>
            <strong>{{ formatNumber(futureExpPerDay) }}</strong>
          </div>
          <div class="stat">
            <span>升至 Lv{{ params.maxLevel }}</span>
            <strong>{{ formatNumber(totalExpToCap) }}</strong>
          </div>
          <div class="stat">
            <span>将写入 battle 行</span>
            <strong>{{ newBattleRows.length }}</strong>
          </div>
        </div>

        <div class="chart-panel">
          <div class="chart-title">单级经验曲线</div>
          <div ref="reqChartEl" class="echart"></div>
        </div>

        <div class="chart-panel">
          <div class="chart-title">累计经验曲线</div>
          <div ref="cumulativeChartEl" class="echart"></div>
        </div>

        <div class="panel migration-panel">
          <div class="panel-title">累计经验重算预览</div>
          <label class="preview-input">
            <span>累计总经验</span>
            <input type="number" v-model.number="params.previewTotalExp" min="0" step="1000" />
          </label>
          <div class="migration-grid">
            <div><span>旧曲线等级</span><strong>Lv{{ recomputePreview.oldProgress.level }}</strong></div>
            <div><span>旧本级进度</span><strong>{{ formatDecimal(recomputePreview.oldProgress.progressRatio * 100, 2) }}%</strong></div>
            <div><span>新曲线等级</span><strong>Lv{{ recomputePreview.newProgress.level }}</strong></div>
            <div><span>等级变化</span><strong>{{ recomputePreview.levelDelta >= 0 ? '+' : '' }}{{ recomputePreview.levelDelta }}</strong></div>
          </div>
        </div>

        <div class="table-panel">
          <table>
            <thead>
              <tr>
                <th>升级到</th>
                <th>新单级</th>
                <th>旧单级</th>
                <th>新累计</th>
                <th>当前/天</th>
                <th>未来/天</th>
                <th>击杀/级</th>
                <th>小时/级</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="row in keyLevels" :key="row.level">
                <td>Lv{{ row.targetLevel }}</td>
                <td>{{ formatNumber(row.req) }}</td>
                <td>{{ row.oldReq ? formatNumber(row.oldReq) : '-' }}</td>
                <td>{{ formatNumber(row.cumulative) }}</td>
                <td>{{ formatDecimal(row.levelsPerDay) }}</td>
                <td>{{ formatDecimal(row.futureLevelsPerDay) }}</td>
                <td>{{ formatDecimal(row.killsPerLevel, 1) }}</td>
                <td>{{ formatDecimal(row.hoursPerLevel, 1) }}</td>
              </tr>
            </tbody>
          </table>
        </div>
      </section>
    </div>
  </div>
</template>

<style scoped>
.battle-exp-root {
  padding: 16px;
  color: #e2e8f0;
  font-size: 13px;
}
.header-row {
  display: flex;
  justify-content: space-between;
  gap: 16px;
  align-items: flex-start;
}
h2 {
  margin: 0;
  font-size: 20px;
}
p {
  margin: 4px 0 0;
  color: #94a3b8;
}
.actions {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
  justify-content: flex-end;
}
button {
  border: 1px solid #334155;
  background: #0f172a;
  color: #e2e8f0;
  border-radius: 6px;
  padding: 7px 10px;
  cursor: pointer;
}
button:disabled {
  opacity: 0.55;
  cursor: not-allowed;
}
.btn-primary {
  background: #0ea5e9;
  border-color: #38bdf8;
  color: #04111d;
  font-weight: 700;
}
.btn-secondary {
  background: #1e293b;
}
.status-line {
  margin: 12px 0;
  color: #93c5fd;
  min-height: 18px;
}
.layout {
  display: grid;
  grid-template-columns: minmax(240px, 300px) minmax(0, 1fr);
  gap: 16px;
}
.panel,
.chart-panel,
.table-panel {
  background: #111827;
  border: 1px solid #26364d;
  border-radius: 8px;
  padding: 12px;
}
.params-panel {
  display: grid;
  gap: 8px;
  align-content: start;
}
.panel-title,
.chart-title {
  color: #94a3b8;
  font-size: 12px;
  font-weight: 700;
  margin-bottom: 8px;
}
.spaced {
  margin-top: 10px;
}
label {
  display: grid;
  grid-template-columns: 1fr 112px;
  align-items: center;
  gap: 8px;
}
label span {
  color: #cbd5e1;
}
input {
  min-width: 0;
  background: #020617;
  border: 1px solid #334155;
  color: #e2e8f0;
  border-radius: 6px;
  padding: 6px 8px;
}
.main-area {
  display: grid;
  gap: 14px;
  min-width: 0;
}
.stats-grid,
.migration-grid {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 10px;
}
.stat,
.migration-grid > div {
  background: #0f172a;
  border: 1px solid #26364d;
  border-radius: 8px;
  padding: 10px;
}
.stat span,
.migration-grid span {
  display: block;
  color: #94a3b8;
  font-size: 12px;
}
.stat strong,
.migration-grid strong {
  display: block;
  margin-top: 4px;
  font-size: 18px;
  color: #f8fafc;
}
.echart {
  display: block;
  width: 100%;
  height: 360px;
  min-height: 360px;
  background: #0b1120;
  border-radius: 6px;
}
.preview-input {
  max-width: 360px;
  margin-bottom: 10px;
}
table {
  width: 100%;
  border-collapse: collapse;
  font-size: 12px;
}
th,
td {
  padding: 7px 8px;
  border-bottom: 1px solid #26364d;
  text-align: right;
  white-space: nowrap;
}
th:first-child,
td:first-child {
  text-align: left;
}
th {
  color: #94a3b8;
  font-weight: 700;
}
td {
  color: #e2e8f0;
}
@media (max-width: 1100px) {
  .layout {
    grid-template-columns: 1fr;
  }
  .stats-grid,
  .migration-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}
</style>
