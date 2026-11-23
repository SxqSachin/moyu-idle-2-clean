<script setup lang="ts">
import { computed, defineAsyncComponent, defineComponent, h, nextTick, onBeforeUnmount, onMounted, reactive, ref, toRaw, watch, type PropType } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import * as echarts from 'echarts/core';
import { GridComponent, TooltipComponent, DataZoomComponent, type GridComponentOption, type TooltipComponentOption, type DataZoomComponentOption } from 'echarts/components';
import { LineChart, type LineSeriesOption } from 'echarts/charts';
import { CanvasRenderer } from 'echarts/renderers';
import { SPECIAL_VIEWS } from './router';
import type { AttributeValue, Constants, DropPricingRarityRule, EnemyAttributeTemplates, EnemyTemplateRule, EnemyTierTemplateRule, ScoreBreakdownSection } from './model/types';
import {
  COMBAT_TABLES,
  EDITABLE_TABLES,
  TABLE_LABELS,
  asArray,
  asNumber,
  asString,
  clone,
  createEmptyCatalog,
  defaultEffect,
  defaultFormula,
  defaultTargeting,
  isRecord,
  makeDefaultRow,
  rowId,
  type Catalog,
  type CombatTable,
  type DataRow
} from './model/catalog';
import { scoreRow, type ScoringConfig } from './model/realScoring';
import {
  buildBalanceIssueList,
  diagnoseRowBalance,
  baselineAtLevel,
  generateEnemyStats,
  estimateTierParams,
  type BalanceDiagnostics,
  type CombatBalanceModel
} from './model/balance';
import {
  balanceModelFromSharedGlobalConfig,
  difficultyAuthoringGuardrailsFromSharedGlobalConfig,
  loadSharedScoringConfig,
  loadCombatBalanceModel,
  loadDifficultyAuthoringGuardrails,
  scoringConfigFromSharedGlobalConfig,
  sharedGlobalConfigWithScoring,
  type RawSharedConfig
} from './model/sharedScoring';
import { applyEnemyDifficultyGuardrails, buildEnemyDifficultyGuardrailPatch, projectEnemyDefensiveStats } from './model/enemyDifficultyGuardrails';
import { loadCatalog, requestGameDataDir, restoreGameDataDir, saveCatalog, type GameDataDirectoryHandle, type GameDataFileHandle } from './services/gameDataFs';
import { equipmentSecondaryClassIds } from './model/equipmentSecondaryClass';
import { localizeKey, t } from './model/i18n';
import EnhanceRateChart from './components/EnhanceRateChart.vue';
import CatVillageRepChart from './components/CatVillageRepChart.vue';
import CatVillageRepCalculator from './components/CatVillageRepCalculator.vue';
import DifficultyCurveEditor from './components/DifficultyCurveEditor.vue';
import BattleExpCurveDesigner from './components/BattleExpCurveDesigner.vue';
import OverviewPanel from './components/OverviewPanel.vue';
import PondFeedEditor from './components/PondFeedEditor.vue';
import FishFoodScalingEditor from './components/FishFoodScalingEditor.vue';
import InfusionCapacityParamsEditor from './components/InfusionCapacityParamsEditor.vue';
import FishFoodLifetimeBonusesEditor from './components/FishFoodLifetimeBonusesEditor.vue';
import { buildSkillOverview, buildEquipmentOverview } from './model/overview';
import { buildDropPricingReport } from './model/dropPricing';
import { validateSpecializationTrees } from './model/specialization';
import { createStableSkillEffectId, validateGlobalSkillEffectStableIds, validateSkillEffectStableIds } from './model/skillEffectIds';
import {
  DEFAULT_POWERED_EXPONENTIAL_DECAY,
  POWERED_EXPONENTIAL_DECAY_KIND,
  approximateHitDamageEquivalentCap,
  hitDamageEquivalentSum,
  hitDamageMultiplier,
  normalizeHitDamageScaling,
  validateHitDamageScaling
} from './model/hitDamageScaling';
import {
  configuredEnemyOnlyDamageReductionPierceFields,
  removeEnemyOnlyDamageReductionPierceFields,
  validateEnemyOnlyDamageReductionPierceSkill,
  validateEnemyOnlyTargetPrioritySkill,
  validatePlayerOwnedDamageSpecs,
  validatePlayerOwnedTargetPriority
} from './model/enemyOnlyDamageReductionPierce';

const SpecializationTreeDesigner = defineAsyncComponent(() => import('./components/SpecializationTreeDesigner.vue'));

echarts.use([GridComponent, TooltipComponent, DataZoomComponent, LineChart, CanvasRenderer]);

type EditableTable = CombatTable;
type ActiveView = EditableTable | 'scoring' | 'balance' | 'drop_pricing' | 'enhance' | 'cat_village_rep_curve' | 'cat_village_rep_calc' | 'difficulty' | 'battle_exp_curve' | 'alchemy_review' | 'alchemy_inherit_review';
type DropPricingChartOption = echarts.ComposeOption<GridComponentOption | TooltipComponentOption | DataZoomComponentOption | LineSeriesOption>;
type FieldKind = 'text' | 'number' | 'boolean' | 'select' | 'stringList' | 'numberMap' | 'rangeMap' | 'rangeMapOptions' | 'targeting' | 'effects' | 'formula' | 'json' | 'pondFeed' | 'enemyEquipment' | 'enemySkills' | 'enemyDrops' | 'buffConditions' | 'buffDslEffects' | 'battleHooks' | 'attrConversions' | 'enhanceCostCurve' | 'enhanceAttrCurve' | 'defaultAffixSlots' | 'enhanceAffixMilestonePools' | 'affixEffectParams' | 'castGuards' | 'kittyPersonalityEffects' | 'difficultySettings' | 'achievementCondition' | 'achievementRewards' | 'achievementStages' | 'alchemyGroupEntries' | 'alchemyMediumModifiers' | 'alchemyInheritModifiers' | 'alchemyValueRange' | 'alchemyAffixOutput' | 'alchemyLuckyTiers' | 'alchemyNameAffix' | 'alchemyTraitList';
interface FieldDef {
  key: string;
  label: string;
  kind: FieldKind;
  options?: () => string[];
  span?: number;
}

function getRowSearchText(table: EditableTable, row: DataRow, index: number): string {
  const key = `${table}:${index}:${rowId(row, '__no_id__')}`;
  const cached = rowSearchTextCache.get(key);
  if (cached) return cached;
  const next = buildRowSearchText(row);
  rowSearchTextCache.set(key, next);
  return next;
}

function clearRowSearchTextCache() {
  rowSearchTextCache.clear();
}

function applyDefaultCollapsedState(groups: Array<{ key: string }> = groupedRows.value) {
  for (const group of groups) collapsedGroups[group.key] = true;
}

function pruneCollapsedGroupState(groups: Array<{ key: string }> = groupedRows.value) {
  const keys = new Set(groups.map((group) => group.key));
  for (const key of Object.keys(collapsedGroups)) {
    if (!keys.has(key)) delete collapsedGroups[key];
  }
}

function resetCollapsedGroupState(groups: Array<{ key: string }> = groupedRows.value) {
  for (const key of Object.keys(collapsedGroups)) delete collapsedGroups[key];
  applyDefaultCollapsedState(groups);
}

function isGroupCollapsed(key: string): boolean {
  return collapsedGroups[key] !== false;
}

function toggleGroupCollapse(key: string) {
  collapsedGroups[key] = !isGroupCollapsed(key);
}

function fieldMetaText(field: FieldDef): string {
  if (editorTable.value === 'actions' && field.key === 'mainOutput') {
    const detail = selectedActionEconomy.value;
    if (!detail) return '';
    if (!detail.mainOutput) return '未设置主产出';
    if (detail.mainOutputExpectedCount <= 0) return '主产出期望数量为 0';
    return `≈ ${formatGoldDisplay(detail.mainOutputUnitGold)} 金/个`;
  }
  if (editorTable.value === 'equipment_templates' && selectedEquipmentEnhanceSummary.value) {
    if (field.key === 'enhanceAttrCurve') return `最终属性：${formatAttrMapSummary(selectedEquipmentEnhanceSummary.value.finalAttrs)}`;
    if (field.key === 'enhanceCostCurve') return `总消耗：${formatItemMapSummary(selectedEquipmentEnhanceSummary.value.totalCosts)}`;
  }
  return '';
}
interface FieldLink {
  label: string;
  table: EditableTable;
  id?: string;
  match?: (row: DataRow) => boolean;
}
interface ActionEconomySummary {
  mainOutput: string;
  mainOutputExpectedCount: number;
  baseLaborGold: number;
  totalCostGold: number;
  actionTotalGold: number;
  mainOutputUnitGold: number;
}
interface EquipmentEnhanceSummary {
  maxLevel: number;
  cumulativeEnhanceAttrs: Record<string, number>;
  finalAttrs: Record<string, number>;
  totalCosts: Record<string, number>;
  totalCostCount: number;
}

const POSITIONS = ['vanguard', 'midguard', 'rearguard'];
const SEEK_OPTIONS = ['front_to_rear', 'rear_to_front', 'random_alive', 'none'];
const TARGET_SCOPES = ['ally', 'enemy', 'all', 'self'];
const SIDE_OPTIONS = ['source', 'player', 'enemy'];
const SELECT_RULES = ['hp_max', 'hp_min', 'hp_percent_max', 'hp_percent_min', 'attr_max', 'attr_min', 'boss_first', 'first', 'random', 'self'];
const EFFECT_KINDS = ['damage', 'heal', 'resource', 'status', 'summon', 'weighted_random_per_target', 'corpse_explosion', 'adjust_skill_cooldown'];
const DAMAGE_TYPES = ['physical', 'magical'];
const TARGET_PREDICATE_KINDS = ['compare_attr', 'exclude_enemy_type', 'require_target_has_status', 'exclude_target_has_status', 'target_hp_not_full'];
const TARGET_COMPARE_OPERATORS = ['>', '>=', '<', '<=', '==', '!='];
const CAST_GUARD_KINDS = ['enemy_side_not_has_effect', 'ally_side_has_non_full_hp'];
const BUFF_TRIGGERS = ['while_active', 'on_attach', 'on_detach', 'on_enemy_refresh', 'on_turn_start', 'on_turn_end', 'on_before_take_damage', 'on_after_take_damage', 'on_before_deal_damage', 'on_after_deal_damage'];
const BUFF_CONDITION_KINDS = ['wave_interval_eq', 'self_position', 'ally_side_has_positions', 'self_weapon_class', 'source_weapon_class', 'self_hp_percent_lte', 'self_hp_percent_gte', 'source_hp_percent_lte', 'source_hp_percent_gte', 'target_hp_percent_lte', 'target_hp_percent_gte', 'self_mp_lt', 'self_mp_gte', 'self_skill_cooldown_count_gte', 'equipment_enhance_level_gte', 'incoming_damage_gte', 'incoming_damage_lte', 'incoming_heal_gte', 'incoming_heal_lte', 'has_buff', 'source_has_buff', 'target_has_buff', 'self_charges_remaining_gt', 'self_charges_remaining_gte', 'self_charges_remaining_lt', 'self_charges_remaining_lte', 'self_buff_count_gte', 'self_buff_count_gt', 'self_buff_count_lte', 'self_buff_count_lt', 'self_buff_count_eq', '__damage_types__'];
const BATTLE_HOOK_CONDITION_KINDS = [...BUFF_CONDITION_KINDS, 'target_buff_count_gte', 'target_distinct_source_buff_count_gte', 'player_party_equipped_unit_count_eq'];
const BUFF_EFFECT_KINDS = [
  'add_stat',
  'add_stat_percent',
  'damage',
  'heal',
  'resource',
  'add_skill_cooldown',
  'adjust_skill_cooldown',
  'add_buff',
  'remove_buff',
  'reduce_damage_percent',
  'set_damage_multiplier',
  'add_damage_percent',
  'add_damage_flat',
  'add_damage',
  'untargetable_by_enemy',
  'untargetable_by_ally',
  'treat_all_as_enemy',
  'camouflage_targeting_remap',
  'prefer_as_enemy_target',
  'avoid_as_enemy_target',
  'prefer_as_ally_target',
  'avoid_as_ally_target',
  'turn_action_suppressed',
  'skill_action_suppressed',
  'basic_attack_suppressed',
  'frontline_fixation',
  'position_override',
  'position_map',
  'advance_position',
  'side_override',
  'invert_relation_perspective',
  'damage_redirect_to_source',
  'marked_target',
  'skip_turn_order',
  'turn_ineligible',
  'trigger_synced_summon_turn',
  'suppress_mirror_cast_trigger',
  'extra_enemy_refresh'
];
const BUFF_RESOURCE_OPTIONS = ['mp', 'hp'];
const ENEMY_REFRESH_POOL_OPTIONS = ['current_world'];
const BATTLE_ENEMY_REFRESH_POOL_OPTIONS = ['current_wave_boss'];
const MIRROR_CAST_TRIGGER_SCOPES = ['all', 'ally', 'enemy'];
const MIRROR_CAST_TARGETING_MODES = ['reselect', 'reuse_original_targets', 'reuse_then_reselect'];
const BATTLE_HOOK_TRIGGERS = ['on_battle_start', 'on_wave_start', 'on_enemy_refresh', 'on_turn_start', 'on_turn_end', 'on_after_skill_cooldown_tick', 'on_other_unit_turn_end', 'on_before_deal_damage', 'on_after_deal_damage', 'on_after_apply_buff', 'on_after_deal_crit', 'on_after_deal_no_crit', 'on_after_basic_attack_deal_damage', 'on_before_take_damage', 'on_after_take_damage', 'on_ally_after_take_damage', 'on_after_deal_heal', 'on_kill', 'on_unit_death', 'on_after_summon', 'on_other_unit_cast_skill', 'on_before_probability_check', 'on_probability_success', 'on_probability_failure', 'on_battle_end'];
const BATTLE_HOOK_TARGET_MODES = ['self', 'source', 'targeting'];
const BUFF_HOOK_TARGET_MODES = ['self', 'source'];
const BATTLE_HOOK_EFFECT_KINDS = ['add_buff', 'remove_buff', 'remove_buff_by_kind', 'damage', 'heal', 'consume_mp', 'adjust_skill_cooldown', 'transfer_skill_cooldown', 'extra_turn', 'execute', 'runtime_damage_amp', 'add_runtime_damage_amp', 'add_damage_amp', 'basic_attack_marked_target', 'extra_enemy_refresh', 'summon', 'player_clone', 'mirror_cast_skill', 'add_probability', 'set_probability', 'force_probability_failure'];
const BATTLE_HOOK_BUFF_KINDS = ['buff', 'debuff'];
const SKILL_COOLDOWN_SELECTION_MODES = ['all', 'max_remaining'];
const BATTLE_HOOK_UNIT_FILTER_TYPES = ['summon', 'player', 'kitty', 'enemy'];
const BATTLE_HOOK_APPLICATION_KINDS = ['attach', 'refresh'];
const BATTLE_HOOK_TURN_KINDS = ['ordered', 'extra'];
const BATTLE_PROBABILITY_KINDS = ['equipment_proc', 'status_apply'];
const SUMMON_ON_CASTER_DEATH_OPTIONS = ['keep_until_duration_end', 'despawn_immediately'];
const SUMMON_ON_CAP_OVERFLOW_OPTIONS = ['replace_oldest', 'fail_new_summon'];
const SUMMON_ON_DISPEL_OPTIONS = ['despawn_immediately', 'ignore_dispel'];
const SUMMON_ON_DURATION_END_OPTIONS = ['despawn_immediately'];
const OPTION_LABEL_OVERRIDES: Record<string, string> = {
  on_battle_start: '战斗开始',
  on_wave_start: '每波战斗开始前',
  on_enemy_refresh: '创建本波敌人时',
  on_turn_start: '回合开始',
  on_turn_end: '回合结束',
  on_after_skill_cooldown_tick: '主动施法并完成冷却递减后',
  on_other_unit_turn_end: '其他单位回合结束（不含自身）',
  on_after_take_damage: '受击后',
  on_ally_after_take_damage: '友方受击后（不含自身）',
  on_before_deal_damage: '造成伤害前',
  on_after_deal_damage: '造成伤害后',
  on_after_apply_buff: '成功施加或刷新状态后',
  on_kill: '击杀目标后',
  on_unit_death: '任意单位阵亡后',
  on_before_take_damage: '受击前',
  on_after_summon: '召唤后',
  on_other_unit_cast_skill: '其他单位释放技能（不含自身）',
  on_before_probability_check: '概率判定前',
  on_probability_success: '概率判定成功后',
  on_probability_failure: '概率判定失败后',
  on_battle_end: '战斗结束',
  suppress_mirror_cast_trigger: '禁止触发镜像施法',
  add_buff: '施加状态',
  remove_buff: '移除状态',
  remove_buff_by_kind: '按类别移除状态',
  damage: '造成伤害',
  add_skill_cooldown: '追加技能冷却',
  transfer_skill_cooldown: '转移技能冷却',
  heal: '治疗',
  extra_turn: '立即行动一回合',
  runtime_damage_amp: '运行时增伤',
  add_runtime_damage_amp: '叠加运行时增伤',
  add_damage_amp: '叠加增伤',
  basic_attack_marked_target: '普攻随机标记目标',
  marked_target: '视作被标记',
  summon: '召唤',
  player_clone: '玩家克隆体（旧影）',
  add_probability: '调整概率',
  set_probability: '设置概率',
  force_probability_failure: '强制判定失败',
  extra_enemy_refresh: '额外刷新地图敌人',
  current_wave_boss: '本波已创建的 Boss',
  current_world: '当前地图普通敌人池',
  equipment_proc: '装备效果触发',
  status_apply: '状态施加',
  equipment_enhance_level: '装备强化等级',
  player_party_equipped_unit_count_eq: '队伍真实玩家装备数等于',
  ordered: '正常行动',
  extra: '额外行动',
  team_side: '按原始队伍阵营',
  keep_until_duration_end: '召唤者死亡后保留至持续结束',
  despawn_immediately: '立即离场',
  replace_oldest: '超上限替换最早召唤',
  fail_new_summon: '超上限时放弃本次召唤',
  ignore_dispel: '被驱散时忽略',
  all: '全体',
  self: '自身',
  ally: '友方',
  enemy: '敌方',
  source: '来源 / 跟随召唤者阵营',
  attach: '首次施加',
  refresh: '刷新或叠加',
  targeting: '按目标模板',
  reselect: '重新索敌',
  reuse_original_targets: '复用原目标',
  reuse_then_reselect: '优先复用，失败重选',
  wave_interval_eq: '波次间隔（每N波）',
  self_position: '自身站位',
  ally_side_has_positions: '己方覆盖站位',
  self_weapon_class: '自身装备武器类别',
  source_weapon_class: '来源装备武器类别',
  self_hp_percent_lte: '自身血量 ≤ 阈值',
  self_hp_percent_gte: '自身血量 ≥ 阈值',
  source_hp_percent_lte: '来源血量 ≤ 阈值',
  source_hp_percent_gte: '来源血量 ≥ 阈值',
  target_hp_percent_lte: '目标血量 ≤ 阈值',
  target_hp_percent_gte: '目标血量 ≥ 阈值',
  incoming_damage_gte: '受到伤害 ≥ 阈值',
  incoming_damage_lte: '受到伤害 ≤ 阈值',
  incoming_heal_gte: '受到治疗 ≥ 阈值',
  incoming_heal_lte: '受到治疗 ≤ 阈值',
  has_buff: '自身拥有状态',
  source_has_buff: '来源拥有状态',
  target_has_buff: '目标拥有状态',
  self_charges_remaining_gt: '层数剩余 > 阈值',
  self_charges_remaining_gte: '层数剩余 ≥ 阈值',
  self_charges_remaining_lt: '层数剩余 < 阈值',
  self_charges_remaining_lte: '层数剩余 ≤ 阈值',
  self_buff_count_gte: '自身状态层数 ≥ 阈值',
  self_buff_count_gt: '自身状态层数 > 阈值',
  self_buff_count_lte: '自身状态层数 ≤ 阈值',
  self_buff_count_lt: '自身状态层数 < 阈值',
  self_buff_count_eq: '自身状态层数 = 阈值',
  __damage_types__: '伤害类型匹配',
  // cast guard kinds
  enemy_side_not_has_effect: '敌方半场不存在效果',
  ally_side_has_non_full_hp: '友方存在非满血单位',
  // kitty journal personality grant match types
  category: '按类别',
  contentKey: '按内容Key',
  // personality grant categories
  'diary.idle': '悠闲日记',
  'diary.working': '工作日记',
  encounter: '相遇记录',
  skip_turn_order: '跳过行动队列',
  turn_ineligible: '完全无法获得回合',
  trigger_synced_summon_turn: '触发同步召唤物行动',
  mirror_cast_skill: '镜像释放技能'
};
const EFFECT_MUTATION_SCOPES = ['basic_attack', 'skill'];
const ENEMY_TYPES = ['normal', 'elite', 'boss'];
const EQUIP_TYPES = ['battle', 'work'];
const EQUIP_TARGETS = ['player', 'kitty'];
const BUFF_KINDS = ['buff', 'debuff'];
const BUFF_STACK_POLICIES = ['refresh_duration', 'stack'];
const EQUIPMENT_SLOT_ORDER = ['head', 'cloak', 'body', 'arm', 'legs', 'feet', 'right_hand', 'left_hand', 'accessory', 'accessory_1', 'accessory_2', 'accessory_3', 'accessory_4'];

const gameDataDir = ref<GameDataDirectoryHandle | null>(null);
const route = useRoute();
const router = useRouter();
const activeView = ref<ActiveView>(resolveViewFromRoute());
const selectedTable = ref<EditableTable>(resolveTableFromRoute());
const selectedIndex = ref(resolveIndexFromRoute());
const status = ref('未连接 packages/game-data');
const issues = ref<string[]>([]);
const saveBusy = ref(false);
const filterInputText = ref('');
const debouncedFilterText = ref('');
const sortByScore = ref(false);
const scoringConfigFile = ref<GameDataFileHandle | null>(null);
const collapsedGroups = reactive<Record<string, boolean>>({});
const dropSourceCollapsedWorlds = reactive<Record<string, boolean>>({});
const enemyTemplateBudgetOverride = ref<number | null>(null);
const enemyTemplateLevel = ref(1);

// 基础属性分根据等级自适应：max(1, floor(level^1.2))
// Level 1 = 1 点, Level 10 = 15, Level 50 = 109, Level 100 = 251
const enemyTemplateBudget = computed(() => {
  if (enemyTemplateBudgetOverride.value != null) return enemyTemplateBudgetOverride.value;
  const level = Math.max(1, enemyTemplateLevel.value || 1);
  return Math.max(1, Math.floor(Math.pow(level, 1.2)));
});
const enemyTemplateTier = ref('normal');
const enemyTemplateRole = ref('tank');
const enemyTemplateSpecies = ref('humanoid');
const enemyTemplateVariance = ref(0.18);
const comparePanels = ref<Array<{ key: string; table: EditableTable; index: number; top: number; left: number; z: number }>>([]);
const comparePanelDragging = ref<{ key: string; offsetX: number; offsetY: number } | null>(null);
const comparePanelZSeed = ref(100);
const FILTER_DEBOUNCE_MS = 120;
let filterDebounceTimer: ReturnType<typeof setTimeout> | null = null;
const rowSearchTextCache = new Map<string, string>();

const catalog = reactive<Catalog>(createEmptyCatalog());
const scoring = reactive<ScoringConfig>(loadSharedScoringConfig());
const balanceModel = reactive<CombatBalanceModel>(loadCombatBalanceModel());
const difficultyAuthoringGuardrails = reactive(loadDifficultyAuthoringGuardrails());

function isSpecialView(v: ActiveView): boolean {
  return v === 'scoring' || v === 'balance' || v === 'drop_pricing' || v === 'enhance' || v === 'cat_village_rep_curve' || v === 'cat_village_rep_calc' || v === 'difficulty' || v === 'battle_exp_curve' || v === 'alchemy_review' || v === 'alchemy_inherit_review';
}

function resolveViewFromRoute(): ActiveView {
  const table = String(route.params.table || '');
  if (!table || table === 'undefined') return 'skills';
  if ((SPECIAL_VIEWS as string[]).includes(table)) return table as ActiveView;
  if ((COMBAT_TABLES as readonly string[]).includes(table)) return table as ActiveView;
  return 'skills';
}
function resolveTableFromRoute(): EditableTable {
  const v = resolveViewFromRoute();
  return isSpecialView(v) ? 'skills' : v as EditableTable;
}
function resolveIndexFromRoute(): number {
  return 0;
}
function resolveIndexFromCatalog(table: EditableTable, targetId: string): number {
  const rows = catalog[table] ?? [];
  if (!targetId) return 0;
  const idx = rows.findIndex((row) => rowId(row) === targetId);
  return idx >= 0 ? idx : 0;
}
function routePathForView(table: ActiveView, id?: string): string {
  if (id) return `/${table}/${encodeURIComponent(id)}`;
  return `/${table}`;
}

const activeTable = computed<EditableTable>(() => isSpecialView(activeView.value) ? 'skills' : activeView.value as EditableTable);
const tableRows = computed(() => isSpecialView(activeView.value) ? [] : catalog[activeTable.value] ?? []);
const selectedTableRows = computed(() => catalog[selectedTable.value] ?? []);
const editorTable = computed<EditableTable>(() => selectedTable.value);
const searchQuery = computed(() => debouncedFilterText.value.trim().toLowerCase());
const filteredRows = computed(() => {
  const q = searchQuery.value;
  const table = activeTable.value;
  return tableRows.value
    .map((row, index) => ({ row, index }))
    .filter(({ row, index }) => !q || getRowSearchText(table, row, index).includes(q));
});
const groupedRows = computed(() => buildRowGroups(activeTable.value, filteredRows.value));
const shouldAutoExpandGroups = computed(() => Boolean(searchQuery.value) && filteredRows.value.length > 0 && filteredRows.value.length < 25);
const selectedRow = computed(() => selectedTableRows.value[selectedIndex.value] ?? null);

// 选中敌人时同步模板参数（level/type/race/class）
watch(selectedRow, (row) => {
  if (!row || editorTable.value !== 'enemies') return;
  const level = Number(row.level);
  if (Number.isFinite(level) && level > 0) enemyTemplateLevel.value = Math.round(level);
  const type = String(row.type || '').trim();
  if (type && scoring.enemyAttributeTemplates.tiers.some((t) => t.key === type)) {
    enemyTemplateTier.value = type;
  }
  const cls = String((row as Record<string, unknown>).class || '').trim();
  if (cls && scoring.enemyAttributeTemplates.roles.some((r) => r.key === cls)) {
    enemyTemplateRole.value = cls;
  }
  const race = String((row as Record<string, unknown>).race || '').trim();
  if (race && scoring.enemyAttributeTemplates.species.some((s) => s.key === race)) {
    enemyTemplateSpecies.value = race;
  }
  enemyTemplateBudgetOverride.value = null;
});
const selectedScore = computed(() => (selectedRow.value ? scoreRow(editorTable.value, selectedRow.value, catalog, scoring) : null));
const selectedRowOverview = computed(() => {
  if (!selectedRow.value) return null;
  if (editorTable.value === 'skills') return buildSkillOverview(selectedRow.value, scoring);
  if (editorTable.value === 'equipment_templates') return buildEquipmentOverview(selectedRow.value, scoring);
  return null;
});
const selectedBalanceDiagnostics = computed<BalanceDiagnostics | null>(() => (
  selectedRow.value ? diagnoseRowBalance(editorTable.value, selectedRow.value, catalog, scoring, balanceModel) : null
));
const balanceIssues = computed(() => buildBalanceIssueList(catalog, scoring, balanceModel).slice(0, 80));
const previewLevel = ref(50);
const previewTier = ref('normal');
const previewRole = ref('warrior');
const previewSpecies = ref('');
const expandedRoles = ref(new Set<string>());
const expandedSpecies = ref(new Set<string>());
const previewStats = computed(() => generateEnemyStats(previewLevel.value, previewTier.value, previewRole.value, previewSpecies.value, balanceModel, scoring));
const previewBaseline = computed(() => baselineAtLevel(balanceModel, previewLevel.value));
const previewTierData = computed(() => scoring.enemyAttributeTemplates.tiers.find((t) => t.key === previewTier.value));
const previewRoleData = computed(() => scoring.enemyAttributeTemplates.roles.find((r) => r.key === previewRole.value));
const previewSpeciesData = computed(() => scoring.enemyAttributeTemplates.species.find((s) => s.key === previewSpecies.value));
watch(
  () => [editorTable.value, selectedIndex.value, rowId(selectedRow.value ?? {})],
  () => {
    if (editorTable.value !== 'enemies' || !selectedRow.value) return;
    enemyTemplateLevel.value = Math.max(1, asNumber(selectedRow.value.level, enemyTemplateLevel.value || 1));
    const currentType = asString(selectedRow.value.type);
    const matchedTier = scoring.enemyAttributeTemplates.tiers.find((tier) => tier.enemyType === currentType || tier.key === currentType);
    if (matchedTier) enemyTemplateTier.value = matchedTier.key;
  }
);

watch(
  () => filterInputText.value,
  (value) => {
    if (filterDebounceTimer) clearTimeout(filterDebounceTimer);
    filterDebounceTimer = setTimeout(() => {
      debouncedFilterText.value = value;
    }, FILTER_DEBOUNCE_MS);
  }
);

watch(
  () => activeTable.value,
  () => {
    filterInputText.value = '';
    debouncedFilterText.value = '';
    clearRowSearchTextCache();
    resetCollapsedGroupState();
  },
  { immediate: true }
);

watch(
  () => searchQuery.value,
  (next, previous) => {
    if (previous && !next) applyDefaultCollapsedState();
  }
);

watch(
  () => [groupedRows.value, shouldAutoExpandGroups.value] as const,
  ([groups, autoExpand]) => {
    pruneCollapsedGroupState(groups);
    if (autoExpand) {
      for (const group of groups) collapsedGroups[group.key] = false;
      return;
    }
    for (const group of groups) {
      if (!(group.key in collapsedGroups)) collapsedGroups[group.key] = true;
    }
  },
  { immediate: true }
);
const attributeOptions = computed(() => {
  const fromCatalog = catalog.attributes.map((row) => rowId(row)).filter(Boolean);
  const fromScoring = scoring.attributes.map((row) => row.key);
  return Array.from(new Set([...fromCatalog, ...fromScoring])).sort();
});
const rarityOptions = computed(() => {
  const fromCatalog = catalog.rarity_definitions.map((row) => rowId(row)).filter(Boolean);
  const fromScoring = scoring.rarities.map((row) => row.key);
  return Array.from(new Set([...fromCatalog, ...fromScoring])).sort();
});
const skillOptions = computed(() => catalog.skills.map((row) => rowId(row)).filter(Boolean).sort());
const skillCategoryOptions = computed(() => catalog.skill_type.map((row) => rowId(row)).filter(Boolean).sort());
const buffOptions = computed(() => catalog.buff.map((row) => rowId(row)).filter(Boolean).sort());
const currentBuffEffects = computed(() => {
  if (editorTable.value !== 'buff') return [];
  const buffId = selectedRow.value ? rowId(selectedRow.value) : '';
  if (!buffId) return [];
  return catalog.buff_effects
    .map((row, index) => ({ row, index }))
    .filter(({ row }) => asString(row.buffId) === buffId);
});
const affixOptions = computed(() => catalog.affix_definitions.map((row) => rowId(row)).filter(Boolean).sort());
const equipmentOptions = computed(() => catalog.equipment_templates.map((row) => rowId(row)).filter(Boolean).sort());
const itemOptions = computed(() => catalog.items.map((row) => rowId(row)).filter(Boolean).sort());
const nonMythicItemOptions = computed(() => catalog.items
  .filter((row) => asString(row.rarity).toLowerCase() !== 'mythic')
  .map((row) => rowId(row))
  .filter(Boolean)
  .sort());
const buildingOptions = computed(() => catalog.buildings.map((row) => rowId(row)).filter(Boolean).sort());
const battleUsableItemOptions = computed(() => catalog.battle_usable_items.map((row) => rowId(row)).filter(Boolean).sort());
const itemTypeOptions = computed(() => Array.from(new Set(catalog.items.flatMap((row) => asArray(row.type).map((item) => asString(item)).filter(Boolean)))).sort());
const enemyOptions = computed(() => catalog.enemies.map((row) => rowId(row)).filter(Boolean).sort());
const fishingSpeciesOptions = computed(() => catalog.fishing_species.map((row) => rowId(row)).filter(Boolean).sort());
const weaponClassOptions = computed(() => catalog.weapon_classes.map((row) => rowId(row)).filter(Boolean).sort());
const equipmentSecondaryClassOptions = computed(() => equipmentSecondaryClassIds);
const actionMainTypeOptions = computed(() => Array.from(new Set(catalog.actions.map((row) => asString(row.mainType)).filter(Boolean))).sort());
const actionSubTypeOptions = computed(() => Array.from(new Set([
  ...catalog.actions.map((row) => asString(row.subType)).filter(Boolean),
  ...catalog.actions.map((row) => asString(row.sub_type)).filter(Boolean)
])).sort());
const actionIdOptions = computed(() => catalog.actions.map((row) => rowId(row)).filter(Boolean).sort());
const actionDurationMap = computed(() => {
  const map: Record<string, number> = {};
  catalog.actions.forEach((row) => {
    const id = asString(rowId(row));
    if (id) map[id] = asNumber(row.durationS, 0);
  });
  return map;
});
const actionMenuOptions = computed(() => Array.from(new Set(catalog.actions.map((row) => asString(row.menu)).filter(Boolean))).sort());
const slotOptions = computed(() => {
  const fromTemplates = catalog.equipment_templates.map((row) => asString(row.slot)).filter(Boolean);
  const fromScoring = scoring.slots.map((row) => row.key);
  const all = new Set([...EQUIPMENT_SLOT_ORDER, ...fromTemplates, ...fromScoring]);
  return Array.from(all).sort((a, b) => {
    const ia = EQUIPMENT_SLOT_ORDER.indexOf(a);
    const ib = EQUIPMENT_SLOT_ORDER.indexOf(b);
    if (ia >= 0 && ib >= 0) return ia - ib;
    if (ia >= 0) return -1;
    if (ib >= 0) return 1;
    return a.localeCompare(b);
  });
});
const enemyTemplateTierOptions = computed(() => scoring.enemyAttributeTemplates.tiers.map((row) => row.key));
const enemyTemplateRoleOptions = computed(() => scoring.enemyAttributeTemplates.roles.map((row) => row.key));
const enemyTemplateSpeciesOptions = computed(() => scoring.enemyAttributeTemplates.species.map((row) => row.key));

function asStringList(raw: unknown): string[] {
  if (Array.isArray(raw)) return raw.flatMap((value) => asStringList(value));
  if (typeof raw === 'string') return raw.split(',').map((value) => value.trim()).filter(Boolean);
  const value = asString(raw).trim();
  return value ? [value] : [];
}

function rowDisabled(raw: unknown): boolean {
  if (!isRecord(raw)) return false;
  const value = raw.disabled;
  if (typeof value === 'boolean') return value;
  if (typeof value === 'number') return value !== 0;
  if (typeof value === 'string') {
    const normalized = value.trim().toLowerCase();
    if (normalized === 'true' || normalized === '1') return true;
    if (normalized === 'false' || normalized === '0') return false;
  }
  return false;
}

function ensureDisableFlag(row: DataRow): DataRow {
  const next = { ...row };
  if (typeof next.disabled !== 'boolean') next.disabled = false;
  return next;
}

function worldEnemyIDs(row: DataRow): string[] {
  const lineupIDs = asArray(row.customLineupConf).flatMap((conf) => isRecord(conf) ? asStringList(conf.lineup) : []);
  return Array.from(new Set([...asStringList(row.enemyIds), ...asStringList(row.specialEnemyIds), ...lineupIDs]));
}

const enemyWorldGroups = computed(() => {
  const map = new Map<string, string[]>();
  for (const world of catalog.world) {
    const worldID = rowId(world);
    if (!worldID) continue;
    for (const enemyID of worldEnemyIDs(world)) {
      const list = map.get(enemyID) ?? [];
      if (!list.includes(worldID)) list.push(worldID);
      map.set(enemyID, list);
    }
  }
  return map;
});

type ItemDropSourceSummary = {
  enemyIds: string[];
  nonWorldBossEnemyIds: string[];
  enemyTypes: string[];
  worldIds: string[];
  hasNonWorldBoss: boolean;
  hasWorldBoss: boolean;
};

const itemDropSourceMap = computed(() => {
  const map = new Map<string, ItemDropSourceSummary>();
  const ensure = (itemId: string): ItemDropSourceSummary => {
    const existing = map.get(itemId);
    if (existing) return existing;
    const next: ItemDropSourceSummary = {
      enemyIds: [],
      nonWorldBossEnemyIds: [],
      enemyTypes: [],
      worldIds: [],
      hasNonWorldBoss: false,
      hasWorldBoss: false
    };
    map.set(itemId, next);
    return next;
  };
  for (const enemy of catalog.enemies) {
    const enemyId = rowId(enemy);
    if (!enemyId || !isRecord(enemy.drops)) continue;
    const enemyType = asString(enemy.type, 'normal') || 'normal';
    const worlds = enemyWorldGroups.value.get(enemyId) ?? [];
    for (const itemId of Object.keys(enemy.drops)) {
      if (!itemId || itemId === 'gold_coin') continue;
      const summary = ensure(itemId);
      if (!summary.enemyIds.includes(enemyId)) summary.enemyIds.push(enemyId);
      if (!summary.enemyTypes.includes(enemyType)) summary.enemyTypes.push(enemyType);
      for (const worldId of worlds) {
        if (!summary.worldIds.includes(worldId)) summary.worldIds.push(worldId);
      }
      if (enemyType === 'world_boss') {
        summary.hasWorldBoss = true;
      } else {
        summary.hasNonWorldBoss = true;
        if (!summary.nonWorldBossEnemyIds.includes(enemyId)) summary.nonWorldBossEnemyIds.push(enemyId);
      }
    }
  }
  return map;
});

const ITEM_TYPE_ORDER = ['skillbook', 'loot', 'material', 'equipment', 'special', 'collectible', 'food', 'trigger_item', 'currency', 'other'];
const ITEM_GROUP_ORDER = ['item:drop', 'item:skillbook', 'item:equipment', 'item:material', 'item:loot', 'item:collectible', 'item:food', 'item:special', 'item:trigger_item', 'item:currency', 'item:other'];
const RARITY_ORDER = ['common', 'uncommon', 'rare', 'epic', 'legendary', 'mythic'];

function itemTypeList(row: DataRow): string[] {
  return asArray(row.type).map((item) => asString(item).trim()).filter(Boolean);
}

function itemPrimaryType(row: DataRow): string {
  const types = itemTypeList(row);
  for (const type of ITEM_TYPE_ORDER) {
    if (types.includes(type)) return type;
  }
  if (row.isEquipment === true) return 'equipment';
  return types[0] || 'other';
}

function itemCategoryLabel(type: string): string {
  const labels: Record<string, string> = {
    skillbook: '技能书',
    equipment: '装备',
    material: '材料',
    loot: '战利品',
    collectible: '收藏品',
    special: '特殊物品',
    food: '食物',
    trigger_item: '触发道具',
    currency: '货币',
    other: '其他'
  };
  return labels[type] || optionLabel(type) || type;
}

function itemDropSource(row: DataRow): ItemDropSourceSummary | undefined {
  return itemDropSourceMap.value.get(rowId(row));
}

function worldLabel(worldID: string): string {
  const translated = t(`world.${worldID}.name`);
  return translated === `world.${worldID}.name` ? worldID : `${translated} (${worldID})`;
}

function groupOrderKey(table: EditableTable, key: string): number {
  if (table === 'items') {
    const index = ITEM_GROUP_ORDER.indexOf(key);
    return index >= 0 ? index : 1000;
  }
  if (table === 'skills') {
    const raw = key.replace(/^skill:/, '');
    const index = catalog.skill_type.findIndex((row) => rowId(row) === raw);
    return index >= 0 ? index : 1000;
  }
  if (table === 'equipment_templates' || table === 'actions') {
    const raw = key.replace(/^slot:/, '');
    const index = EQUIPMENT_SLOT_ORDER.indexOf(raw);
    return index >= 0 ? index : 1000;
  }
  if (table === 'enemies') {
    const raw = key.replace(/^world:/, '');
    const index = catalog.world.findIndex((row) => rowId(row) === raw);
    return index >= 0 ? index : 1000;
  }
  return 0;
}

function secondaryGroupOrderKey(key: string): number {
  if (key.startsWith('itemtype:')) {
    const raw = key.replace(/^itemtype:/, '');
    const index = ITEM_TYPE_ORDER.indexOf(raw);
    return index >= 0 ? index : 1000;
  }
  if (key.startsWith('rarity:')) {
    const raw = key.replace(/^rarity:/, '');
    const index = RARITY_ORDER.indexOf(raw);
    return index >= 0 ? index : 1000;
  }
  const raw = key.replace(/^secondary:/, '');
  const index = equipmentSecondaryClassIds.indexOf(raw);
  return index >= 0 ? index : 1000;
}

function buildRowGroups(table: EditableTable, rows: Array<{ row: DataRow; index: number }>) {
  type GroupRow = { row: DataRow; index: number };
  type GroupNode = {
    key: string;
    label: string;
    rows: GroupRow[];
    totalCount: number;
    totalScore: number;
    subGroups: GroupNode[];
  };
  const map = new Map<string, GroupNode>();
  const ensure = (key: string, label: string) => {
    const existing = map.get(key);
    if (existing) return existing;
    const group: GroupNode = { key, label, rows: [], totalCount: 0, totalScore: 0, subGroups: [] };
    map.set(key, group);
    return group;
  };
  const ensureSubGroup = (parent: GroupNode, key: string, label: string) => {
    const existing = parent.subGroups.find((group) => group.key === key);
    if (existing) return existing;
    const group: GroupNode = { key, label, rows: [], totalCount: 0, totalScore: 0, subGroups: [] };
    parent.subGroups.push(group);
    return group;
  };
  const scoreCache = new Map<number, number>();
  const scoreOf = (entry: GroupRow) => {
    const cached = scoreCache.get(entry.index);
    if (cached !== undefined) return cached;
    const next = scoreFor(table, entry.row).score;
    scoreCache.set(entry.index, next);
    return next;
  };
  const optionLabelCache = new Map<string, string>();
  const getOptionLabel = (value: string) => {
    const cached = optionLabelCache.get(value);
    if (cached) return cached;
    const next = optionLabel(value);
    optionLabelCache.set(value, next);
    return next;
  };
  const equipmentByID = table === 'actions'
    ? new Map(catalog.equipment_templates.map((row) => [rowId(row), row] as const))
    : null;
  for (const entry of rows) {
    let key = 'all';
    let label = TABLE_LABELS[table];
    let subKey = '';
    let subLabel = '';
    if (table === 'items') {
      const primaryType = itemPrimaryType(entry.row);
      const dropSource = itemDropSource(entry.row);
      if (dropSource?.hasNonWorldBoss) {
        key = 'item:drop';
        label = '怪物掉落';
        subKey = `itemtype:${primaryType}`;
        subLabel = itemCategoryLabel(primaryType);
      } else {
        key = `item:${primaryType}`;
        label = itemCategoryLabel(primaryType);
        const rarity = asString(entry.row.rarity) || 'unset';
        subKey = `rarity:${rarity}`;
        subLabel = rarity === 'unset' ? '未设置稀有度' : (getOptionLabel(rarity) || rarity);
      }
    } else if (table === 'equipment_templates') {
      const slot = asString(entry.row.slot) || 'other';
      key = `slot:${slot}`;
      label = getOptionLabel(slot);
      const secondary = asString(entry.row.equipmentSecondaryClass) || '';
      const weaponClasses = asStringList(entry.row.weaponClass);
      const secondaryID = secondary || weaponClasses[0] || 'other';
      subKey = `secondary:${secondaryID}`;
      subLabel = secondaryID === 'other' ? '其他' : getOptionLabel(secondaryID);
    } else if (table === 'skills') {
      const category = asString(entry.row.category) || 'other';
      key = `skill:${category}`;
      label = getOptionLabel(category);
    } else if (table === 'enemies') {
      const worlds = enemyWorldGroups.value.get(rowId(entry.row)) ?? [];
      const world = worlds[0] || 'unassigned';
      key = `world:${world}`;
      label = world === 'unassigned' ? '未分配' : worldLabel(world);
    } else if (table === 'actions') {
      const mainOutput = asString(entry.row.mainOutput);
      const equipmentTemplate = mainOutput ? (equipmentByID?.get(mainOutput) ?? null) : null;
      const slot = equipmentTemplate ? asString(equipmentTemplate.slot) || 'other' : '';
      const secondary = equipmentTemplate ? (asString(equipmentTemplate.equipmentSecondaryClass) || asStringList(equipmentTemplate.weaponClass)[0] || 'other') : '';
      if (equipmentTemplate) {
        key = `slot:${slot}`;
        label = getOptionLabel(slot);
        subKey = `secondary:${secondary}`;
        subLabel = secondary === 'other' ? '其他' : getOptionLabel(secondary);
      } else {
        const mainType = asString(entry.row.mainType) || 'other';
        key = `action:${mainType}`;
        label = getOptionLabel(mainType) || mainType;
      }
    }
    const group = ensure(key, label);
    if (subKey) {
      const subGroup = ensureSubGroup(group, subKey, subLabel);
      subGroup.rows.push(entry);
      subGroup.totalCount += 1;
      subGroup.totalScore += scoreOf(entry);
    } else {
      group.rows.push(entry);
      group.totalCount += 1;
      group.totalScore += scoreOf(entry);
    }
  }
  const groups = Array.from(map.values());
  for (const group of groups) {
    if (group.subGroups.length > 0) {
      for (const subGroup of group.subGroups) {
        if (sortByScore.value) {
          subGroup.rows.sort((a, b) => scoreOf(b) - scoreOf(a));
        }
        subGroup.totalScore = subGroup.rows.reduce((total, entry) => total + scoreOf(entry), 0);
      }
      if (sortByScore.value) {
        group.subGroups.sort((a, b) => b.totalScore - a.totalScore || a.label.localeCompare(b.label, 'zh-Hans-CN'));
      } else {
        group.subGroups.sort((a, b) => secondaryGroupOrderKey(a.key) - secondaryGroupOrderKey(b.key) || a.label.localeCompare(b.label, 'zh-Hans-CN'));
      }
      group.totalScore = group.subGroups.reduce((total, subGroup) => total + subGroup.totalScore, 0);
      group.totalCount = group.subGroups.reduce((total, subGroup) => total + subGroup.totalCount, 0);
    } else {
      if (sortByScore.value) {
        group.rows.sort((a, b) => scoreOf(b) - scoreOf(a));
      }
      group.totalScore = group.rows.reduce((total, entry) => total + scoreOf(entry), 0);
      group.totalCount = group.rows.length;
    }
  }
  return groups.sort((a, b) => groupOrderKey(table, a.key) - groupOrderKey(table, b.key) || a.label.localeCompare(b.label, 'zh-Hans-CN'));
}

function resolveRowMeta(table: EditableTable, row: DataRow): string {
  if (table === 'skills') {
    const category = asString(row.category) || 'other';
    return `技能类型：${optionLabel(category) || category}`;
  }
  if (table === 'equipment_templates') {
    const slot = asString(row.slot) || 'other';
    const secondary = asString(row.equipmentSecondaryClass) || asStringList(row.weaponClass)[0] || 'other';
    return `槽位：${optionLabel(slot) || slot} / 二级：${secondary === 'other' ? '其他' : optionLabel(secondary) || secondary}`;
  }
  if (table === 'actions') {
    const mainOutput = asString(row.mainOutput);
    if (mainOutput) {
      const outputLabel = optionLabel(mainOutput) || mainOutput;
      const equipmentTemplate = catalog.equipment_templates.find((template) => rowId(template) === mainOutput);
      if (equipmentTemplate) {
        const slot = asString(equipmentTemplate.slot) || 'other';
        const secondary = asString(equipmentTemplate.equipmentSecondaryClass) || asStringList(equipmentTemplate.weaponClass)[0] || 'other';
        return `主产出：${outputLabel} / ${optionLabel(slot) || slot} / ${secondary === 'other' ? '其他' : optionLabel(secondary) || secondary}`;
      }
      return `主产出：${outputLabel}`;
    }
    const mainType = asString(row.mainType) || 'other';
    return `主类型：${optionLabel(mainType) || mainType}`;
  }
  if (table === 'enemies') {
    return `类型：${asString(row.type) || 'unknown'} / 等级：${asNumber(row.level, 0)}`;
  }
  if (table === 'items') {
    const id = rowId(row);
    const types = itemTypeList(row).map((type) => itemCategoryLabel(type)).join('/');
    const rarity = asString(row.rarity) || '未设置';
    const source = itemDropSource(row);
    const parts = [
      `类型：${types || '其他'}`,
      `稀有度：${optionLabel(rarity) || rarity}`
    ];
    if (source?.hasNonWorldBoss) {
      const enemies = source.nonWorldBossEnemyIds.slice(0, 2).join(', ');
      const more = source.nonWorldBossEnemyIds.length > 2 ? ` +${source.nonWorldBossEnemyIds.length - 2}` : '';
      parts.push(`掉落：${enemies || id}${more}`);
    } else if (source?.hasWorldBoss) {
      parts.push('仅世界首领掉落');
    }
    const pricing = id ? dropPricingItemMap.value.get(id) : null;
    if (pricing?.skillbookAnchorEnemyType) {
      parts.push(`技能书锚定：${pricing.skillbookAnchorEnemyType} -> ${pricing.pricingRarity}`);
    }
    return parts.join(' / ');
  }
    if (table === 'affix_definitions') {
      return `效果：${asString(row.effectType) || 'unknown'} / 适用：${asString(row.type) || 'unknown'}`;
    }
    if (table === 'buildings') {
      return `最高等级：${asNumber(row.maxLevel, 1)} / 耗时：${asNumber(row.baseDurationS, 0)}+${asNumber(row.stepDurationS, 0)}s`;
    }
    return '';
}

function rowMetaLine(table: EditableTable, row: DataRow): string {
  return resolveRowMeta(table, row);
}

function tableFields(table: EditableTable): FieldDef[] {
  const commonText = (key: string, label: string, span = 1): FieldDef => ({ key, label, kind: 'text', span });
  const commonNumber = (key: string, label: string): FieldDef => ({ key, label, kind: 'number' });
  switch (table) {
    case 'actions':
      return [
        commonText('id', 'ID'),
        commonText('iconId', '图标ID'),
        { key: 'mainType', label: '主类型', kind: 'select', options: () => actionMainTypeOptions.value },
        { key: 'menu', label: '菜单', kind: 'select', options: () => actionMenuOptions.value },
        { key: 'subType', label: '子类型', kind: 'select', options: () => actionSubTypeOptions.value },
        commonNumber('durationS', '耗时'),
        { key: 'rarity', label: '稀有度', kind: 'select', options: () => rarityOptions.value },
        { key: 'costs', label: '消耗', kind: 'numberMap', options: () => itemOptions.value, span: 4 },
        { key: 'materialInstanceRequirements', label: '材料实例条件', kind: 'json', span: 4 },
        { key: 'itemReward', label: '产出', kind: 'json', span: 4 },
        { key: 'mainOutput', label: '主产出', kind: 'select', options: () => itemOptions.value, span: 4 },
        { key: 'statusRequirement', label: '属性门槛', kind: 'numberMap', options: () => attributeOptions.value, span: 2 },
        { key: 'statusExpReward', label: '属性经验', kind: 'numberMap', options: () => attributeOptions.value, span: 2 },
        commonNumber('economyIndustryTier', '行业阶级'),
        commonNumber('economyDifficultyMultiplier', '难度倍率'),
        { key: 'buildingRequirement', label: '建筑需求', kind: 'numberMap', options: () => buildingOptions.value, span: 2 }
      ];
    case 'operation_effects':
      return [
        commonText('id', 'ID'),
        commonText('rulesetVersion', '效果版本'),
        commonText('nameKey', '名称Key'),
        commonText('descKey', '描述Key'),
        commonText('iconId', '图标ID'),
        { key: 'domain', label: '业务领域', kind: 'select', options: () => ['life_action', 'alchemy', 'enhancement', 'enchantment', 'fishing_cast', 'building'] },
        { key: 'kind', label: '效果处理器', kind: 'select', options: () => [...new Set(catalog.operation_effects.map(row => asString(row.kind)).filter(Boolean))].sort(), span: 2 },
        { key: 'allowedTargets', label: '受益对象', kind: 'stringList', options: () => ['player', 'kitty', 'all_kitties', 'player_and_kitties'], span: 2 },
        commonNumber('priority', '优先级'),
        { key: 'lifetime', label: '次数或持续时间', kind: 'json', span: 2 },
        { key: 'stacking', label: '互斥与累计上限', kind: 'json', span: 2 },
        { key: 'selector', label: '动作筛选条件', kind: 'json', span: 2 },
        { key: 'params', label: '效果参数（比例使用小数）', kind: 'numberMap', span: 2 }
      ];
    case 'cattery_furniture':
      return [
        commonText('id', 'ID'), commonText('version', '规则版本'), commonText('nameKey', '名称Key'), commonText('iconId', '图标ID'),
        { key: 'service', label: '服务类型', kind: 'select', options: () => ['permanent', 'prepaid'] },
        commonNumber('capacitySeconds', '容量秒数（永久为0）'), commonNumber('refillSeconds', '每份秒数（永久为0）'),
        { key: 'installCosts', label: '一次性安置材料', kind: 'numberMap', options: () => itemOptions.value, span: 4 },
        { key: 'refillCosts', label: '每份维护材料（永久留空）', kind: 'numberMap', options: () => itemOptions.value, span: 4 },
        { key: 'rules', label: '效果规则（rateBps：100=1个百分点）', kind: 'json', span: 4 },
        { key: 'levels', label: '逐级升级：level / upgradeCosts / refillCostAdd / rules', kind: 'json', span: 4 },
      ];
    case 'fish_food_recipes':
      return [
        commonText('id', 'ID'),
        commonText('rulesetVersion', '配方规则版本'),
        { key: 'actionId', label: '制作动作', kind: 'select', options: () => catalog.actions.map((row) => rowId(row)).filter(Boolean).sort() },
        { key: 'outputItemId', label: '产出料理', kind: 'select', options: () => itemOptions.value },
        { key: 'kind', label: '料理类型', kind: 'select', options: () => ['grilled_fish', 'fish_soup'] },
        commonText('qualityRulesetVersion', '品质算法版本'),
        { key: 'allowSpecificIngredients', label: '允许指定食材', kind: 'boolean' },
        { key: 'parameterScaling', label: '各效果参数的食材权重', kind: 'json', span: 4 },
        { key: 'lifetimeScaling', label: '次数的食材成长（可选）', kind: 'json', span: 4 },
        { key: 'legacyEffectIds', label: '历史成品效果定义', kind: 'stringList', span: 4 },
        { key: 'replacedEffectIds', label: '替换旧成品效果', kind: 'stringList', span: 4 },
        { key: 'mutationLifetimeBonuses', label: '变异次数与时长奖励', kind: 'json', span: 4 },
        { key: 'fishIngredients', label: '鱼类食材', kind: 'json', span: 4 }
      ];
    case 'buildings':
      return [
        commonText('id', 'ID'),
        commonText('iconId', '图标ID'),
        commonText('nameKey', '名称Key(可选)'),
        commonText('descKey', '描述Key'),
        commonNumber('order', '排序'),
        commonNumber('maxLevel', '最高等级'),
        commonNumber('baseDurationS', '基础耗时(秒)'),
        commonNumber('stepDurationS', '每级增量耗时(秒)'),
        { key: 'baseCosts', label: '基础消耗', kind: 'numberMap', options: () => itemOptions.value, span: 2 },
        { key: 'stepCosts', label: '每级增量消耗', kind: 'numberMap', options: () => itemOptions.value, span: 2 },
        { key: 'baseStatusRequirement', label: '基础属性门槛', kind: 'numberMap', options: () => attributeOptions.value, span: 2 },
        { key: 'stepStatusRequirement', label: '每级增量属性门槛', kind: 'numberMap', options: () => attributeOptions.value, span: 2 }
      ];
    case 'skills':
      return [
        commonText('id', 'ID'),
        commonText('name', '名称'),
        commonText('icon', '图标'),
        { key: 'category', label: '技能类型', kind: 'select', options: () => catalog.skill_type.map((row) => rowId(row)).filter(Boolean) },
        commonNumber('hpCost', 'HP消耗'),
        commonNumber('mpCost', 'MP消耗'),
        commonNumber('cooldown', '冷却'),
        { key: 'enemyOnly', label: '仅供怪物使用', kind: 'boolean' },
        { key: 'statusRequirement', label: '属性门槛', kind: 'numberMap', options: () => attributeOptions.value, span: 2 },
        { key: 'weaponRequirement', label: '武器需求', kind: 'stringList', options: () => weaponClassOptions.value, span: 2 },
        { key: 'castGuards', label: '施法门禁', kind: 'castGuards', span: 4 },
        { key: 'effects', label: '效果列表', kind: 'effects', span: 4 }
      ];
    case 'equipment_templates':
      return [
        commonText('id', 'ID'),
        { key: 'equipType', label: '装备类型', kind: 'select', options: () => EQUIP_TYPES },
        { key: 'equipTargets', label: '佩戴对象', kind: 'stringList', options: () => EQUIP_TARGETS },
        { key: 'slot', label: '槽位', kind: 'select', options: () => slotOptions.value },
        { key: 'weaponClass', label: '武器类别', kind: 'stringList', options: () => weaponClassOptions.value },
        { key: 'equipmentSecondaryClass', label: '装备二级分类', kind: 'select', options: () => ['', ...equipmentSecondaryClassOptions.value] },
        { key: 'occupySlots', label: '占用槽位', kind: 'stringList', options: () => slotOptions.value },
        { key: 'statusRequirement', label: '属性门槛', kind: 'numberMap', options: () => attributeOptions.value, span: 4 },
        commonText('accessoryGroup', '饰品互斥组'),
        commonNumber('affixSlotCount', '词条槽数'),
        { key: 'affixAllowedIds', label: '词条白名单', kind: 'stringList', options: () => affixOptions.value, span: 2 },
        { key: 'baseAttrs', label: '基础属性', kind: 'numberMap', options: () => attributeOptions.value, span: 2 },
        commonNumber('maxDurability', '最大耐久'),
        commonNumber('decayRate', '耐久衰减'),
        commonNumber('enhanceMaxLevel', '强化上限'),
        commonNumber('enhanceDurationS', '强化耗时'),
        commonNumber('enhanceExp', '强化经验'),
        { key: 'enhanceProtectItemId', label: '保护道具', kind: 'stringList', options: () => itemOptions.value },
        { key: 'enhanceCostCurve', label: '强化消耗曲线', kind: 'enhanceCostCurve', span: 4 },
        { key: 'enhanceAttrCurve', label: '强化属性曲线', kind: 'enhanceAttrCurve', span: 4 },
        { key: 'defaultAffixSlots', label: '默认词条槽', kind: 'defaultAffixSlots', span: 4 },
        { key: 'enhanceAffixMilestonePools', label: '强化里程碑词条池', kind: 'enhanceAffixMilestonePools', span: 4 },
        { key: 'basicAttackEffects', label: '普攻效果', kind: 'effects', span: 4 },
        { key: 'battleHooks', label: '战斗钩子', kind: 'battleHooks', span: 4 },
        { key: 'attrConversions', label: '属性转换', kind: 'attrConversions', span: 4 }
      ];
    case 'affix_definitions':
      return [
        commonText('id', 'ID'),
        { key: 'effectType', label: '效果类型', kind: 'select', options: () => ['attr_bonus', 'effectMutation', 'battle_hook', 'battle_trigger'] },
        { key: 'type', label: '词条类型', kind: 'select', options: () => EQUIP_TYPES },
        { key: 'allowedRarities', label: '可用稀有度', kind: 'stringList', options: () => rarityOptions.value },
        commonNumber('valuePrecision', '数值精度'),
        commonNumber('manualScore', '手动基础评分'),
        commonNumber('maxEffectivePerEquipment', '单装备上限'),
        commonNumber('maxEffectivePerBody', '最多同时生效'),
        { key: 'active', label: '启用', kind: 'boolean' },
        { key: 'effectParams', label: '效果参数', kind: 'affixEffectParams', span: 4 },
        { key: 'smeltOverride', label: '熔炉覆盖奖励', kind: 'json', span: 4 }
      ];
    case 'buff':
      return [
        commonText('id', 'ID'),
        commonText('nameKey', '名称Key'),
        commonText('descKey', '描述Key'),
        commonText('icon', '图标'),
        { key: 'kind', label: '类型', kind: 'select', options: () => BUFF_KINDS },
        { key: 'type', label: '分类', kind: 'select', options: () => catalog.buff_type.map((row) => rowId(row)).filter(Boolean) },
        { key: 'stackPolicy', label: '叠加策略', kind: 'select', options: () => BUFF_STACK_POLICIES },
        commonNumber('durationTurns', '持续回合'),
        commonNumber('maxCharges', '最大充能数（0=不限）'),
        { key: 'permanent', label: '永久', kind: 'boolean' },
        { key: 'dispellable', label: '可驱散', kind: 'boolean' },
        { key: 'showInBattleCard', label: '显示在战斗卡片上', kind: 'boolean' }
      ];
    case 'buff_effects':
      return [
        commonText('id', 'ID'),
        { key: 'buffId', label: 'Buff', kind: 'select', options: () => buffOptions.value },
        { key: 'trigger', label: '触发器', kind: 'select', options: () => BUFF_TRIGGERS },
        commonNumber('priority', '优先级'),
        { key: 'conditions', label: '条件', kind: 'buffConditions', span: 4 },
        { key: 'effects', label: '效果', kind: 'buffDslEffects', span: 4 }
      ];
    case 'enemies':
      return [
        commonText('id', 'ID'),
        commonText('name', '名称'),
        { key: 'position', label: '站位', kind: 'select', options: () => POSITIONS },
        { key: 'type', label: '敌人类型', kind: 'select', options: () => ENEMY_TYPES },
        commonNumber('level', '等级'),
        commonNumber('exp', '经验'),
        { key: 'skillIds', label: '技能', kind: 'enemySkills', span: 4 },
        { key: 'equipment', label: '默认装备', kind: 'enemyEquipment', span: 4 },
        { key: 'drops', label: '掉落', kind: 'enemyDrops', span: 4 },
        { key: '__enemyAttrs', label: '属性矩阵', kind: 'json', span: 4 }
      ];
    case 'world':
      return [
        commonText('id', 'ID'),
        { key: 'enemyIds', label: '普通敌人池', kind: 'stringList', options: () => enemyOptions.value, span: 2 },
        { key: 'specialEnemyIds', label: '特殊敌人池', kind: 'stringList', options: () => enemyOptions.value, span: 2 },
        { key: 'specialWaves', label: '特殊波次', kind: 'json', span: 2 },
        { key: 'waveSchedules', label: '周期波次', kind: 'json', span: 4 },
        { key: 'normalEnemyCount', label: '普通敌人数', kind: 'json' },
        { key: 'extraEnemyPerPlayer', label: '每玩家额外敌人', kind: 'json' },
        commonNumber('entryDurationSeconds', '进入耗时'),
        { key: 'entryCosts', label: '进入消耗', kind: 'numberMap', options: () => itemOptions.value },
        {
          key: 'waveDrops',
          label: '额外掉落（格式：{ itemId: { percent, range: {min,max} } }）',
          kind: 'json',
          span: 4
        },
        { key: 'customLineupConf', label: '自定义阵容', kind: 'json', span: 4 },
        { key: 'difficultySettings', label: '难度设置', kind: 'difficultySettings', span: 4 }
      ];
    case 'arena_formats':
      return [
        commonText('id', 'ID'),
        commonText('baseFormatId', '基础赛制 ID（标准赛留空）'),
        commonNumber('schemaVersion', '结构版本'),
        commonText('rulesetVersion', '规则版本'),
        commonNumber('balanceRevision', '平衡修订'),
        commonText('nameKey', '名称 Key'),
        commonText('shortDescKey', '短描述 Key'),
        commonText('descKey', '完整描述 Key'),
        commonText('icon', '图标'),
        commonNumber('sortOrder', '排序'),
        { key: 'enabled', label: '开放', kind: 'boolean' },
        { key: 'triggers', label: '赛制触发器 DSL', kind: 'json', span: 4 }
      ];
    case 'combat_trial_metrics':
      return [
        commonText('id', 'ID'),
        commonNumber('schemaVersion', '结构版本'),
        commonText('nameKey', '名称 Key'),
        commonText('shortNameKey', '短名称 Key'),
        commonText('descKey', '描述 Key'),
        { key: 'eventType', label: '事件', kind: 'select', options: () => ['damage', 'heal', 'battle_end'] },
        { key: 'aggregation', label: '聚合', kind: 'select', options: () => ['sum', 'max', 'final'] },
        { key: 'valueSource', label: '结算值来源', kind: 'select', options: () => ['', 'player_main_actions', 'waves_cleared', 'objective_units_defeated', 'objective_units_remaining_hp'] },
        { key: 'sourceScope', label: '来源范围', kind: 'select', options: () => ['player_side', 'enemy_side', 'any'] },
        { key: 'targetScope', label: '目标范围', kind: 'select', options: () => ['objective_units', 'trial_dummies', 'player_side', 'enemy_side', 'any'] },
        { key: 'ranked', label: '进入排行榜', kind: 'boolean' },
        { key: 'rankDirection', label: '排名方向', kind: 'select', options: () => ['desc', 'asc'] },
        { key: 'eligibleEndReasons', label: '可入榜结束原因', kind: 'stringList', span: 2 },
        { key: 'valueFormat', label: '数值格式', kind: 'select', options: () => ['number', 'integer'] },
        commonNumber('sortOrder', '排序'),
        { key: 'enabled', label: '开放', kind: 'boolean' }
      ];
    case 'combat_trial_formats':
      return [
        commonText('id', 'ID'),
        commonText('baseFormatId', '基础项目 ID（独立项目留空）'),
        commonNumber('schemaVersion', '结构版本'),
        commonText('rulesetVersion', '规则版本'),
        commonNumber('balanceRevision', '平衡修订'),
        commonText('nameKey', '名称 Key'),
        commonText('shortDescKey', '短描述 Key'),
        commonText('descKey', '完整描述 Key'),
        commonText('icon', '图标'),
        commonNumber('sortOrder', '排序'),
        { key: 'enabled', label: '开放', kind: 'boolean' },
        { key: 'lineupRules', label: '出战阵容规则 DSL', kind: 'json', span: 2 },
        { key: 'metricIds', label: '计分指标', kind: 'stringList', options: () => catalog.combat_trial_metrics.map((row) => rowId(row)).filter(Boolean), span: 2 },
        { key: 'limits', label: '战斗限制 DSL', kind: 'json', span: 2 },
        { key: 'encounter', label: '遭遇阵容 DSL', kind: 'json', span: 4 },
        { key: 'endConditions', label: '结束条件 DSL', kind: 'json', span: 2 },
        { key: 'triggers', label: '赛制触发器 DSL', kind: 'json', span: 4 }
      ];
    case 'attributes':
      return [
        commonText('id', 'ID'),
        commonText('icon', 'Iconify图标'),
        commonText('iconId', '图标ID'),
        commonText('emojiIcon', 'Emoji'),
        commonNumber('baseDefault', '默认值'),
        { key: 'growthType', label: '成长类型', kind: 'select', options: () => ['levelable', 'static'] },
        { key: 'unit', label: '单位', kind: 'select', options: () => ['number', 'percent'] },
        commonNumber('order', '排序'),
        { key: 'category', label: '分类', kind: 'select', options: () => ['battle', 'work', 'other'] },
        commonText('expProfileId', '经验曲线'),
        { key: 'allowBattlePointLevelUp', label: '战斗点升级', kind: 'boolean' },
        commonText('relatedActionMainType', '关联行动主类'),
        commonText('relatedActionEffect', '关联行动效果')
      ];
    case 'rarity_definitions':
      return [commonText('id', 'ID'), commonNumber('order', '排序'), commonText('color', '颜色'), commonNumber('economyMultiplier', '经济倍率')];
    case 'kitty_personalities':
      return [
        commonText('id', 'ID'),
        commonText('nameKey', '名称Key'),
        commonText('descKey', '描述Key'),
        commonNumber('weight', '权重'),
        { key: 'effects', label: '性格效果 DSL', kind: 'kittyPersonalityEffects', span: 4 }
      ];
    case 'kitty_journal_personality_grants': {
      const journalCategoryOptions = () => ['diary.idle', 'diary.working', 'encounter'];
      const journalContentKeyOptions = () => [
        ...Array.from({ length: 20 }, (_, i) => `game.catteryJournal.diary.idleVariants.${i + 1}`),
        ...Array.from({ length: 20 }, (_, i) => `game.catteryJournal.diary.workingVariants.${i + 1}`),
        ...Array.from({ length: 20 }, (_, i) => `game.catteryJournal.encounter.metVariants.${i + 1}`)
      ];
      return [
        commonText('id', 'ID'),
        { key: 'matchType', label: '匹配类型', kind: 'select', options: () => ['category', 'contentKey'] },
        {
          key: 'matchValue',
          label: '匹配值',
          kind: 'select',
          options: () => asString(selectedRow.value?.matchType) === 'contentKey'
            ? journalContentKeyOptions()
            : journalCategoryOptions()
        },
        commonNumber('chance', '概率 (0~1)'),
        { key: 'personalityIds', label: '候选特性', kind: 'stringList', options: () => catalog.kitty_personalities.map((row) => rowId(row)).filter(Boolean), span: 2 }
      ];
    }
    case 'market_profiles':
      return [
        commonText('id', '规则 ID'),
        { key: 'disabled', label: '禁用', kind: 'boolean' },
        commonNumber('schemaVersion', '结构版本'),
        commonNumber('groupingVersion', '分组版本'),
        { key: 'uniqueByInstance', label: '独立实例展示', kind: 'boolean' },
        { key: 'catalogKeys', label: '目录字段', kind: 'json', span: 4 },
        { key: 'catalogSource', label: '目录来源（可选：table、namesPath、valuePaths；值路径与目录字段一一对应）', kind: 'json', span: 4 },
        { key: 'groupKeys', label: '同规格判定字段', kind: 'json', span: 4 },
        { key: 'presentation', label: '规格卡片字段', kind: 'json', span: 4 }
      ];
    case 'items':
      return [
        commonText('id', 'ID'),
        { key: 'type', label: '类型', kind: 'stringList', options: () => itemTypeOptions.value },
        commonText('category', '分类'),
        { key: 'rarity', label: '稀有度', kind: 'select', options: () => rarityOptions.value },
        { key: 'skillForgeEligible', label: '可用于技能炼成', kind: 'boolean' },
        { key: 'isEquipment', label: '装备', kind: 'boolean' },
        { key: 'stackable', label: '可堆叠', kind: 'boolean' },
        { key: 'carryableInCombat', label: '战斗携带', kind: 'boolean' },
        { key: 'marketListable', label: '市场上架', kind: 'boolean' },
        { key: 'marketPolicy', label: '独立交易策略（可选）', kind: 'json', span: 4 },
        commonText('iconId', '图标ID'),
        commonText('emojiIcon', 'Emoji'),
        commonText('nameKey', '名称Key'),
        commonText('descKey', '描述Key'),
        commonText('effectDescKey', '效果Key'),
        commonNumber('economyBaseValue', '经济基础价'),
        { key: 'useGrantSkillId', label: '使用授予技能', kind: 'select', options: () => skillOptions.value },
        { key: 'useGrantCollectibleId', label: '使用授予收藏品', kind: 'select', options: () => catalog.collectibles.map((row: DataRow) => rowId(row)).filter(Boolean).sort() },
        { key: 'battleUsableItemId', label: '战斗可用映射', kind: 'select', options: () => battleUsableItemOptions.value },
        commonNumber('useSkillExpGain', '使用技能经验'),
        { key: 'kittyFeed', label: '猫咪喂食配置', kind: 'json', span: 2 },
        { key: 'pondFeed', label: '池塘鱼粮 DSL', kind: 'pondFeed', span: 4 },
        { key: 'alchemy', label: '炼金元数据（转换权重/排除/命名）', kind: 'json', span: 2 },
        { key: 'baseAttrs', label: '基础属性', kind: 'numberMap', options: () => attributeOptions.value, span: 2 }
      ];
    case 'battle_usable_items': {
      const isTrig = !!selectedRow.value?.isTrigger;
      const baseFields: FieldDef[] = [
        commonText('id', 'ID'),
        { key: 'itemId', label: '物品ID', kind: 'select', options: () => itemOptions.value },
        { key: 'isTrigger', label: '被动触发物品', kind: 'boolean' }
      ];
      if (isTrig) {
        return [
          ...baseFields,
          { key: 'dungeonIds', label: '适用副本', kind: 'stringList', options: () => catalog.world.map((w: DataRow) => rowId(w)).filter(Boolean).sort(), span: 2 },
          { key: 'spawnEnemyIds', label: '召唤敌人池', kind: 'stringList', options: () => enemyOptions.value, span: 2 },
          commonNumber('minWave', '最早触发波次'),
          commonNumber('perWaveChance', '每波触发概率(0-1)'),
          commonNumber('intervalWaves', '间隔波次'),
          commonText('partyTriggerGroup', '队伍触发互斥组')
        ];
      }
      return [
        ...baseFields,
        commonNumber('cooldownTurns', '冷却回合'),
        { key: 'effects', label: '效果列表', kind: 'effects', span: 4 }
      ];
    }
    case 'skill_type':
    case 'buff_type':
      return [commonText('id', 'ID'), commonText('color', '颜色')];
    case 'weapon_classes':
      return [commonText('id', 'ID'), commonText('icon', '图标'), commonText('color', '颜色'), commonNumber('order', '排序')];
    case 'affix_enchant_rules':
      return [
        commonText('id', 'ID'),
        { key: 'toolItemId', label: '工具物品', kind: 'select', options: () => itemOptions.value },
        commonNumber('consumeCount', '消耗数量'),
        { key: 'costs', label: '额外消耗', kind: 'numberMap', options: () => itemOptions.value },
        commonNumber('failChance', '失败率'),
        { key: 'rarityWeights', label: '稀有度权重', kind: 'numberMap', options: () => rarityOptions.value },
        { key: 'allowedAffixIds', label: '允许词条', kind: 'stringList', options: () => affixOptions.value, span: 2 },
        { key: 'allowedAffixScopes', label: '允许范围', kind: 'json', span: 4 },
        { key: 'active', label: '启用', kind: 'boolean' }
      ];
    case 'affix_carrier_rules':
      return [
        commonText('id', 'ID'),
        { key: 'carrierItemId', label: '载体物品', kind: 'select', options: () => itemOptions.value },
        { key: 'allowedRarityIds', label: '允许稀有度', kind: 'stringList', options: () => rarityOptions.value },
        commonNumber('maxCustomAffixes', '自定义词条上限'),
        { key: 'allowedTraitKind', label: '允许特质类型', kind: 'select', options: () => ['both', 'combat', 'life'] },
        { key: 'active', label: '启用', kind: 'boolean' }
      ];
    case 'affix_smelt_rules':
      return [
        commonText('id', 'ID'),
        { key: 'rarityRewards', label: '稀有度奖励', kind: 'json', span: 4 },
        { key: 'active', label: '启用', kind: 'boolean' }
      ];
    case 'alchemy_groups':
      return [
        commonText('id', 'ID'),
        commonText('nameKey', '名称Key'),
        commonText('descKey', '描述Key'),
        commonNumber('minAlchemyLevel', '最低炼金等级'),
        commonNumber('goldCost', '金币消耗'),
        commonNumber('durationSec', '耗时秒数'),
        commonNumber('alchemyExp', '炼金经验'),
        commonNumber('failChanceBase', '基础失败率'),
        commonNumber('minFailChance', '最低失败率'),
        commonNumber('upgradeChanceBase', '升级基础概率'),
        { key: 'entries', label: '组内材料（itemId / 权重）', kind: 'alchemyGroupEntries', span: 4 },
        { key: 'upgradeEntries', label: '升级产物（itemId / 权重）', kind: 'alchemyGroupEntries', span: 4 },
        { key: 'disabled', label: '禁用', kind: 'boolean' }
      ];
    case 'item_traits':
      return [
        commonText('id', 'ID'),
        { key: 'itemId', label: '物品ID', kind: 'select', options: () => itemOptions.value },
        { key: 'nameAffix', label: '命名前后缀Key', kind: 'alchemyNameAffix', span: 2 },
        { key: 'traits', label: '特质列表', kind: 'alchemyTraitList', span: 4 },
        { key: 'disabled', label: '禁用', kind: 'boolean' }
      ];
    case 'alchemy_combo_recipes':
      return [
        commonText('id', 'ID'),
        commonText('nameKey', '名称Key'),
        commonText('descKey', '描述Key'),
        commonNumber('minAlchemyLevel', '最低炼金等级'),
        { key: 'kind', label: '特质类型', kind: 'select', options: () => ['combat', 'life'] },
        { key: 'inputs', label: '输入材料', kind: 'numberMap', options: () => itemOptions.value, span: 2 },
        { key: 'outputAffix', label: '输出词条', kind: 'alchemyAffixOutput', span: 4 },
        { key: 'outputRarity', label: '输出稀有度', kind: 'select', options: () => ['', ...rarityOptions.value] },
        commonNumber('failBaseChance', '基础失败率'),
        commonNumber('failChancePerLevelAbove', '每超等级失败率降低'),
        commonNumber('priority', '优先级'),
        { key: 'disabled', label: '禁用', kind: 'boolean' }
      ];
    case 'alchemy_transmute_rules':
      return [
        commonText('id', 'ID'),
        { key: 'itemIds', label: '指定物品', kind: 'stringList', options: () => itemOptions.value, span: 2 },
        commonNumber('priority', '优先级'),
        { key: 'default', label: '默认规则', kind: 'boolean' },
        { key: 'returnRatio', label: '金币返还比例（min/max）', kind: 'alchemyValueRange', span: 2 },
        commonNumber('failChanceBase', '基础失败率'),
        commonNumber('minFailChance', '最低失败率'),
        commonNumber('luckyChanceBase', '基础幸运概率'),
        commonNumber('durationSec', '耗时秒数'),
        commonNumber('alchemyExp', '炼金经验'),
        { key: 'luckyTiers', label: '幸运奖励档位', kind: 'alchemyLuckyTiers', span: 4 },
        commonNumber('minAlchemyLevel', '最低炼金等级'),
        { key: 'disabled', label: '禁用', kind: 'boolean' }
      ];
    case 'alchemy_mediums':
      return [
        commonText('id', 'ID'),
        { key: 'itemId', label: '物品ID', kind: 'select', options: () => itemOptions.value },
        commonText('nameKey', '名称Key'),
        commonText('descKey', '描述Key'),
        commonNumber('consumePerCraft', '每次合成消耗数量'),
        commonNumber('minAlchemyLevel', '最低炼金等级'),
        { key: 'modifiers', label: '修正器（值/副作用/失败率/保底/槽位/系别亲和）', kind: 'alchemyMediumModifiers', span: 4 },
        { key: 'active', label: '启用', kind: 'boolean' },
        { key: 'disabled', label: '禁用', kind: 'boolean' }
      ];
    case 'alchemy_inherit_mediums':
      return [
        commonText('id', 'ID'),
        { key: 'itemId', label: '物品ID', kind: 'select', options: () => itemOptions.value },
        commonText('nameKey', '名称Key'),
        commonText('descKey', '描述Key'),
        commonNumber('consumePerCraft', '每次融合消耗数量'),
        commonNumber('minAlchemyLevel', '最低炼金等级'),
        { key: 'modifiers', label: '融合修正器（保留/抑制/变异/倍率）', kind: 'alchemyInheritModifiers', span: 4 },
        { key: 'active', label: '启用', kind: 'boolean' },
        { key: 'disabled', label: '禁用', kind: 'boolean' }
      ];
    case 'cat_village_commissions':
      return [
        commonText('id', 'ID'),
        commonText('nameKey', '名称Key'),
        commonText('descKey', '描述Key'),
        commonNumber('weight', '权重'),
        commonNumber('minRepLevel', '最低声望等级'),
        commonNumber('reputationMin', '声望奖励(最低)'),
        commonNumber('reputationMax', '声望奖励(最高)'),
        { key: 'costOptions', label: '所需物资(多方案)', kind: 'rangeMapOptions', options: () => itemOptions.value, span: 4 },
        { key: 'costs', label: '所需物资(单方案)', kind: 'rangeMap', options: () => itemOptions.value, span: 4 },
        { key: 'rewardOptions', label: '奖励池(多方案)', kind: 'rangeMapOptions', options: () => itemOptions.value, span: 4 },
        { key: 'rewardPool', label: '奖励池(单方案)', kind: 'rangeMap', options: () => itemOptions.value, span: 4 },
        { key: 'isRumor', label: '传闻委托', kind: 'boolean' },
        commonText('rumorBadge', '传闻徽章ID'),
        commonText('rumorMutexGroup', '传闻互斥组'),
        { key: 'oncePerAccount', label: '账号唯一', kind: 'boolean' },
        { key: 'notRefreshable', label: '不可刷新', kind: 'boolean' },
        { key: 'notDeletable', label: '禁止放弃', kind: 'boolean' },
        commonNumber('pityThreshold', '保底次数(0=不保底)'),
        {
          key: 'clearCompletionIds',
          label: '完成后清除哪些传闻的完成记录（使其可重新触发）',
          kind: 'stringList',
          options: () => catalog.cat_village_commissions.map((r: DataRow) => rowId(r)).filter(Boolean).sort(),
          span: 2
        },
        { key: 'prereqRumorIds', label: '前置传闻IDs', kind: 'json', span: 2 }
      ];
    case 'collectibles':
      return [
        commonText('id', 'ID'),
        { key: 'itemId', label: '物品ID', kind: 'select', options: () => itemOptions.value },
        commonText('nameKey', '名称Key'),
        commonText('descKey', '描述Key'),
        { key: 'rarity', label: '稀有度', kind: 'select', options: () => rarityOptions.value },
        { key: 'categoryId', label: '分类', kind: 'select', options: () => catalog.collectible_categories.map((row) => rowId(row)).filter(Boolean).sort() },
        { key: 'playerMods', label: '玩家属性加成（不可升级时生效）', kind: 'numberMap', options: () => attributeOptions.value, span: 2 },
        { key: 'kittyMods', label: '猫咪属性加成', kind: 'numberMap', options: () => attributeOptions.value, span: 2 },
        { key: 'upgradeable', label: '可升级', kind: 'boolean' },
        { key: 'levelMaxLevel', label: '最高等级', kind: 'number' },
        { key: 'levelAttrCurve', label: '等级属性曲线', kind: 'enhanceAttrCurve', span: 4 },
        { key: 'disabled', label: '禁用', kind: 'boolean' }
      ];
    case 'title_prefixes':
      return [
        commonText('id', 'ID'),
        commonText('nameKey', '名称Key'),
        { key: 'rarity', label: '稀有度', kind: 'select', options: () => rarityOptions.value },
        { key: 'order', label: '排序', kind: 'number' }
      ];
    case 'title_suffixes':
      return [
        commonText('id', 'ID'),
        commonText('nameKey', '名称Key'),
        { key: 'rarity', label: '稀有度', kind: 'select', options: () => rarityOptions.value },
        { key: 'order', label: '排序', kind: 'number' }
      ];
    case 'achievement_categories':
      return [
        commonText('id', 'ID'),
        commonText('nameKey', '名称Key'),
        commonText('iconId', '图标ID'),
        { key: 'order', label: '排序', kind: 'number' },
        { key: 'hidden', label: '隐藏分类', kind: 'boolean' }
      ];
    case 'collectible_categories':
      return [
        commonText('id', 'ID'),
        commonText('nameKey', '名称Key'),
        { key: 'order', label: '排序', kind: 'number' }
      ];
    case 'fishing_species':
      return [
        commonText('id', 'ID'),
        commonText('waterId', '水域ID'),
        { key: 'rarity', label: '稀有度', kind: 'select', options: () => rarityOptions.value },
        { key: 'names', label: '多语言名称', kind: 'json', span: 4 },
        { key: 'disabled', label: '禁用', kind: 'boolean' }
      ];
    case 'achievements':
      return [
        commonText('id', 'ID'),
        { key: 'categoryId', label: '分类', kind: 'select', options: () => catalog.achievement_categories.map((row) => rowId(row)).filter(Boolean).sort() },
        commonText('nameKey', '名称Key'),
        commonText('descKey', '描述Key(可选)'),
        { key: 'rarity', label: '稀有度', kind: 'select', options: () => rarityOptions.value },
        commonText('iconId', '图标ID'),
        { key: 'order', label: '排序', kind: 'number' },
        { key: 'hidden', label: '隐藏成就', kind: 'boolean' },
        { key: 'stageProgressMode', label: '阶段进度', kind: 'select', options: () => ['sequential', 'cumulative'] },
        { key: 'prereqAchievementIds', label: '前置成就', kind: 'stringList', options: () => catalog.achievements.map((row) => rowId(row)).filter(Boolean).sort(), span: 2 },
        { key: 'condition', label: '完成条件', kind: 'achievementCondition', span: 4 },
        { key: 'rewards', label: '奖励', kind: 'achievementRewards', span: 4 },
        { key: 'stages', label: '阶段成就', kind: 'achievementStages', span: 4 }
      ];
    default:
      return [commonText('id', 'ID')];
  }
}

const activeFields = computed(() => tableFields(editorTable.value));
const knownKeys = computed(() => new Set(activeFields.value.map((field) => field.key).filter((key) => !key.startsWith('__'))));
const advancedEntries = computed(() => {
  const row = selectedRow.value;
  if (!row) return [];
  return Object.keys(row)
    .filter((key) => !knownKeys.value.has(key) && !(editorTable.value === 'enemies' && attributeOptions.value.includes(key)))
    .sort()
    .map((key) => ({ key, value: row[key] }));
});

function fieldContribution(field: FieldDef): string {
  const score = selectedScore.value;
  if (!score) return '';
  const detail = score.detail;
  const key = field.key;
  const value = (() => {
    if (editorTable.value === 'skills') {
      if (key === 'effects') return detail.effectBudget;
      if (key === 'targeting') return detail.targetingBudget;
      if (key === 'statusRequirement') return detail.requirementBudget;
      if (key === 'mpCost') return -detail.mpPenalty;
      if (key === 'cooldown') return -detail.cooldownPenalty;
    }
    if (editorTable.value === 'equipment_templates') {
      if (key === 'baseAttrs') return detail.baseAttrs;
      if (key === 'enhanceAttrCurve') return detail.enhanceAttrs;
      if (key === 'basicAttackEffects') return detail.basicAttack;
      if (key === 'battleHooks') return detail.battleHooks;
      if (key === 'affixSlotCount') return detail.affixSlots;
      if (key === 'slot') return detail.slotFactor;
    }
    if (editorTable.value === 'affix_definitions') {
      if (key === 'manualScore') return detail.manualScore;
      if (key === 'effectParams') return detail.attrBonus || detail.mutation || detail.battleHooks;
    }
    if (editorTable.value === 'enemies') {
      if (key === '__enemyAttrs') return detail.attrBudget;
      if (key === 'level') return detail.levelFloor;
      if (key === 'skillIds') return detail.skillBudget;
      if (key === 'equipment') return detail.equipmentBudget;
      if (key === 'type') return detail.tierFactor;
    }
    if (editorTable.value === 'world') {
      if (key === 'enemyIds') return detail.enemyPool;
      if (key === 'specialEnemyIds') return detail.specialPool;
      if (key === 'customLineupConf') return detail.lineupBudget;
    }
    return undefined;
  })();
  if (!Number.isFinite(Number(value)) || Number(value) === 0) return '';
  return `${Number(value) > 0 ? '+' : ''}${Number(value).toFixed(2).replace(/\.?0+$/, '')}分`;
}

function setCatalog(next: Catalog) {
  for (const table of COMBAT_TABLES) {
    const rows = Array.isArray(next[table]) ? next[table] : [];
    catalog[table] = (table === 'specialization_trees'
      ? rows.map((row) => ({ ...row }))
      : rows.map((row) => ensureDisableFlag(row))) as Catalog[typeof table];
  }
  clearRowSearchTextCache();
}

function applyScoringConfig(next: ScoringConfig) {
  Object.assign(scoring.constants, next.constants);
  scoring.attributes.splice(0, scoring.attributes.length, ...next.attributes);
  scoring.rarities.splice(0, scoring.rarities.length, ...next.rarities);
  scoring.slots.splice(0, scoring.slots.length, ...next.slots);
  scoring.enemyAttributeTemplates.tiers.splice(0, scoring.enemyAttributeTemplates.tiers.length, ...next.enemyAttributeTemplates.tiers);
  scoring.enemyAttributeTemplates.roles.splice(0, scoring.enemyAttributeTemplates.roles.length, ...next.enemyAttributeTemplates.roles);
  scoring.enemyAttributeTemplates.species.splice(0, scoring.enemyAttributeTemplates.species.length, ...next.enemyAttributeTemplates.species);
  Object.assign(scoring.dropPricing, next.dropPricing);
  scoring.dropPricing.rarityRules.splice(0, scoring.dropPricing.rarityRules.length, ...next.dropPricing.rarityRules);
}

function applyBalanceModel(next: CombatBalanceModel) {
  balanceModel.version = next.version;
  balanceModel.maxBattleLevel = next.maxBattleLevel;
  balanceModel.primaryPointCap = next.primaryPointCap;
  balanceModel.primaryPointAttrs.splice(0, balanceModel.primaryPointAttrs.length, ...next.primaryPointAttrs);
  balanceModel.defenseK = next.defenseK;
  balanceModel.expPerPower = next.expPerPower;
  balanceModel.teamExtraEnemyScale = next.teamExtraEnemyScale;
  balanceModel.teamRewardEfficiencyCap = next.teamRewardEfficiencyCap;
  balanceModel.comboBudgetMinShare = next.comboBudgetMinShare;
  balanceModel.comboBudgetMaxShare = next.comboBudgetMaxShare;
  balanceModel.levelBands.splice(0, balanceModel.levelBands.length, ...next.levelBands);
}

async function requestScoringConfigFile() {
  if (typeof window.showOpenFilePicker !== 'function') {
    issues.value = ['当前浏览器不支持选择 shared-global-config.json，请使用 Chrome/Edge 打开本工具。'];
    return;
  }
  const [handle] = await window.showOpenFilePicker({
    multiple: false,
    types: [{ description: 'Shared global config', accept: { 'application/json': ['.json'] } }]
  });
  scoringConfigFile.value = handle;
  const parsed = JSON.parse(await (await handle.getFile()).text()) as RawSharedConfig;
  applyScoringConfig(scoringConfigFromSharedGlobalConfig(parsed));
  applyBalanceModel(balanceModelFromSharedGlobalConfig(parsed));
  Object.assign(difficultyAuthoringGuardrails, difficultyAuthoringGuardrailsFromSharedGlobalConfig(parsed));
  status.value = '已加载 shared-global-config.json 评分/数值模型配置';
}

async function saveScoringConfigFile() {
  if (!scoringConfigFile.value) {
    await requestScoringConfigFile();
    if (!scoringConfigFile.value) return;
  }
  const file = await scoringConfigFile.value.getFile();
  const parsed = JSON.parse(await file.text()) as RawSharedConfig;
  const next = sharedGlobalConfigWithScoring(parsed, scoring, balanceModel);
  const writable = await scoringConfigFile.value.createWritable();
  await writable.write(`${JSON.stringify(next, null, 2)}\n`);
  await writable.close();
  status.value = '已保存 shared-global-config.json 评分配置';
}

const sharedGlobalConfigFileData = ref<RawSharedConfig | null>(null);

async function saveDifficultyConfig(next: RawSharedConfig) {
  if (!scoringConfigFile.value) {
    status.value = '请先通过评分配置页加载 shared-global-config.json';
    return;
  }
  try {
    const writable = await scoringConfigFile.value.createWritable();
    await writable.write(`${JSON.stringify(next, null, 2)}\n`);
    await writable.close();
    sharedGlobalConfigFileData.value = next;
    Object.assign(difficultyAuthoringGuardrails, difficultyAuthoringGuardrailsFromSharedGlobalConfig(next));
    status.value = '已保存 shared-global-config.json 副本难度配置';
  } catch (error) {
    status.value = `保存失败: ${error instanceof Error ? error.message : String(error)}`;
  }
}

void (async () => {
  if (scoringConfigFile.value) {
    const file = await scoringConfigFile.value.getFile();
    sharedGlobalConfigFileData.value = JSON.parse(await file.text()) as RawSharedConfig;
  }
})();

async function connectDir() {
  try {
    const dir = await requestGameDataDir();
    await loadGameDataDir(dir, '已加载 packages/game-data');
  } catch (error) {
    status.value = `加载失败: ${error instanceof Error ? error.message : String(error)}`;
  }
}

async function loadGameDataDir(dir: GameDataDirectoryHandle, message: string) {
  gameDataDir.value = null;
  const nextCatalog = await loadCatalog(dir);
  setCatalog(nextCatalog);
  gameDataDir.value = dir;
  selectedTable.value = activeTable.value;
  const idParam = String(route.params.id || '');
  selectedIndex.value = idParam ? resolveIndexFromCatalog(activeTable.value, decodeURIComponent(idParam)) : 0;
  status.value = message;
  validateAll();
}

async function restoreLastGameDataDir() {
  try {
    const dir = await restoreGameDataDir();
    if (!dir) return;
    await loadGameDataDir(dir, '已自动恢复 packages/game-data 授权');
  } catch {
    // 自动恢复失败时保持未连接状态，用户仍可手动授权。
  }
}

void restoreLastGameDataDir();

watch(
  () => [route.params.table, route.params.id],
  ([table, id]) => {
    const view = resolveViewFromRoute();
    if (activeView.value !== view) {
      activeView.value = view;
    }
    const tbl: EditableTable = isSpecialView(view) ? 'skills' : view as EditableTable;
    if (selectedTable.value !== tbl) {
      selectedTable.value = tbl;
    }
    const idStr = String(id || '');
    if (idStr) {
      const idx = resolveIndexFromCatalog(tbl, decodeURIComponent(idStr));
      if (selectedIndex.value !== idx) {
        selectedIndex.value = idx;
      }
    }
  }
);

async function saveAll() {
  if (!gameDataDir.value) {
    status.value = '请先授权 packages/game-data 目录';
    return;
  }
  validateAll();
  if (issues.value.length > 0) {
    status.value = `校验失败: ${issues.value.length} 个问题`;
    return;
  }
  saveBusy.value = true;
  try {
    await saveCatalog(gameDataDir.value, catalog);
    status.value = '已保存到 source/json';
  } catch (error) {
    status.value = `保存失败: ${error instanceof Error ? error.message : String(error)}`;
  } finally {
    saveBusy.value = false;
  }
}

function validateAll() {
  clearRowSearchTextCache();
  const next: string[] = [];
  const itemIds = new Set(catalog.items.map((row) => rowId(row)).filter(Boolean));
  const itemRarityById = new Map(catalog.items.map((row) => [rowId(row), asString(row.rarity).toLowerCase()] as const));
  const equipmentIds = new Set(catalog.equipment_templates.map((row) => rowId(row)).filter(Boolean));
  const validBattleHookFormulaAttrs = new Set([...attributeOptions.value, 'equipment_enhance_level', 'max_hp', 'max_mp', 'corpse_max_hp']);
  const validateFormulaNode = (label: string, formula: unknown) => {
    if (!isRecord(formula)) return;
    for (const term of asArray(formula.terms).filter(isRecord)) {
      const attrKey = asString(term.attrKey).trim();
      if (attrKey && !validBattleHookFormulaAttrs.has(attrKey)) next.push(`${label}: 公式属性未定义 ${attrKey}`);
      if (term.perEnhanceLevel !== undefined && !Number.isFinite(Number(term.perEnhanceLevel))) next.push(`${label}: perEnhanceLevel 必须是有限数值`);
    }
  };
  const validateSummonMaxCountFormulaNode = (label: string, formula: unknown) => {
    if (formula === undefined) return;
    if (!isRecord(formula)) {
      next.push(`${label}: 必须是公式`);
      return;
    }
    const baseAdd = Number(formula.baseAdd);
    const terms = asArray(formula.terms).filter(isRecord);
    if (!Number.isFinite(baseAdd)) next.push(`${label}: baseAdd 必须是有限数值`);
    if (!Array.isArray(formula.terms)) next.push(`${label}: terms 必须是数组`);
    if (baseAdd === 0 && terms.length === 0) next.push(`${label}: 公式不能为空`);
    validateFormulaNode(label, formula);
  };
  const validateSummonUnitSelectorNode = (label: string, payload: DataRow) => {
    const unitId = asString(payload.unitId).trim();
    if (!unitId) next.push(`${label}: summonPayload.unitId 不能为空`);
    if (unitId && !enemyOptions.value.includes(unitId)) next.push(`${label}: summonPayload.unitId 未定义 ${unitId}`);
  };
  const validateHookConditions = (label: string, conditions: unknown) => {
    for (const condition of asArray(conditions).filter(isRecord)) {
      const kind = asString(condition.type || (condition.damageTypes ? '__damage_types__' : '')).trim();
      if (!kind || !BATTLE_HOOK_CONDITION_KINDS.includes(kind)) next.push(`${label}: conditions.type 非法 ${kind || '(空)'}`);
      const buffId = asString(condition.buffId).trim();
      if (buffId && !buffOptions.value.includes(buffId)) next.push(`${label}: conditions.buffId 未定义 ${buffId}`);
      if (kind === 'self_weapon_class' || kind === 'source_weapon_class') {
        const weaponClasses = asStringList(condition.weaponClasses ?? condition.weaponClass);
        if (weaponClasses.length === 0) next.push(`${label}: conditions.weaponClasses 不能为空`);
        for (const weaponClass of weaponClasses) {
          if (!weaponClassOptions.value.includes(weaponClass)) next.push(`${label}: conditions.weaponClasses 未定义 ${weaponClass}`);
        }
      }
      if (kind === 'self_position' || kind === 'ally_side_has_positions') {
        const positions = asStringList(condition.positions ?? condition.position);
        if (positions.length === 0) next.push(`${label}: conditions.positions 不能为空`);
        for (const position of positions) {
          if (!POSITIONS.includes(position)) next.push(`${label}: conditions.positions 非法 ${position}`);
        }
      }
      if (kind === 'self_skill_cooldown_count_gte') {
        const categories = asStringList(condition.skillCategories);
        const value = asNumber(condition.value, 0);
        if (!Number.isInteger(value) || value < 1) next.push(`${label}: conditions.value 必须是至少为 1 的整数`);
        for (const category of categories) {
          if (!skillCategoryOptions.value.includes(category)) next.push(`${label}: conditions.skillCategories 未定义 ${category}`);
        }
      }
      if (kind === 'equipment_enhance_level_gte') {
        const value = asNumber(condition.value, -1);
        if (!Number.isInteger(value) || value < 0) next.push(`${label}: conditions.value 必须是非负整数`);
      }
      if (kind === 'player_party_equipped_unit_count_eq') {
        const itemId = asString(condition.itemId).trim();
        const value = asNumber(condition.value, -1);
        if (!equipmentIds.has(itemId)) next.push(`${label}: conditions.itemId 未定义装备 ${itemId || '(空)'}`);
        if (!Number.isInteger(value) || value < 0) next.push(`${label}: conditions.value 必须是非负整数`);
      }
      if (kind === 'target_buff_count_gte') {
        const value = asNumber(condition.value, 0);
        if (!buffId) next.push(`${label}: conditions.buffId 不能为空`);
        if (!Number.isInteger(value) || value < 1) next.push(`${label}: conditions.value 必须是至少为 1 的整数`);
      }
      if (kind === 'target_distinct_source_buff_count_gte') {
        const value = asNumber(condition.value, 0);
        if (!BATTLE_HOOK_BUFF_KINDS.includes(asString(condition.buffKind))) next.push(`${label}: conditions.buffKind 非法`);
        if (!Number.isInteger(value) || value < 1) next.push(`${label}: conditions.value 必须是至少为 1 的整数`);
      }
    }
  };
  const validateProbabilityFilters = (label: string, raw: unknown) => {
    if (!isRecord(raw)) return;
    if (raw.probabilityKinds !== undefined && !Array.isArray(raw.probabilityKinds)) next.push(`${label}: probabilityKinds 必须是数组`);
    for (const probabilityKind of asStringList(raw.probabilityKinds)) {
      if (!BATTLE_PROBABILITY_KINDS.includes(probabilityKind)) next.push(`${label}: probabilityKinds 非法 ${probabilityKind}`);
    }
    for (const key of ['sourceIds', 'excludeSourceIds']) {
      if (raw[key] !== undefined && !Array.isArray(raw[key])) next.push(`${label}: ${key} 必须是数组`);
    }
  };
  const validateBattleHookEffect = (label: string, effect: DataRow, enhanceMaxLevel = 0) => {
    const kind = asString(effect.kind || effect.type).trim();
    if (!kind || !BATTLE_HOOK_EFFECT_KINDS.includes(kind)) next.push(`${label}: effects.kind 非法 ${kind || '(空)'}`);
    if (kind === 'add_buff' || kind === 'remove_buff') {
      const buffId = asString(effect.buffId).trim();
      if (!buffId) next.push(`${label}: ${kind}.buffId 不能为空`);
      if (buffId && !buffOptions.value.includes(buffId)) next.push(`${label}: ${kind}.buffId 未定义 ${buffId}`);
      if (kind === 'add_buff') validateFormulaNode(`${label}: add_buff.procChanceFormula`, effect.procChanceFormula);
    }
    if (kind === 'remove_buff_by_kind') {
      if (!BATTLE_HOOK_BUFF_KINDS.includes(asString(effect.buffKind))) next.push(`${label}: remove_buff_by_kind.buffKind 非法`);
      if (!['max_remaining', 'id'].includes(asString(effect.selectionMode || 'max_remaining'))) next.push(`${label}: remove_buff_by_kind.selectionMode 非法`);
      const maxBuffs = asNumber(effect.maxBuffs, 1);
      if (!Number.isInteger(maxBuffs) || maxBuffs < 1) next.push(`${label}: remove_buff_by_kind.maxBuffs 必须是正整数`);
      validateFormulaNode(`${label}: remove_buff_by_kind.procChanceFormula`, effect.procChanceFormula);
    }
    if (kind === 'heal' || kind === 'consume_mp') validateFormulaNode(`${label}: ${kind}.amountFormula`, effect.amountFormula);
    if (kind === 'damage') {
      const damageType = asString(effect.damageType).trim();
      if (!DAMAGE_TYPES.includes(damageType)) next.push(`${label}: damage.damageType 非法 ${damageType || '(空)'}`);
      if (effect.amountFormulaByRelation !== undefined) {
        if (!isRecord(effect.amountFormulaByRelation)) {
          next.push(`${label}: damage.amountFormulaByRelation 必须是对象`);
        } else {
          validateFormulaNode(`${label}: damage.amountFormulaByRelation.ally`, effect.amountFormulaByRelation.ally);
          validateFormulaNode(`${label}: damage.amountFormulaByRelation.enemy`, effect.amountFormulaByRelation.enemy);
        }
        if (asString(effect.relationMode) !== 'team_side') next.push(`${label}: damage.relationMode 必须为 team_side`);
      } else {
        validateFormulaNode(`${label}: damage.amountFormula`, effect.amountFormula);
      }
      if (effect.relationMode !== undefined && asString(effect.relationMode) !== 'team_side') next.push(`${label}: damage.relationMode 非法 ${asString(effect.relationMode)}`);
      if (effect.simultaneous !== undefined && typeof effect.simultaneous !== 'boolean') next.push(`${label}: damage.simultaneous 必须是布尔值`);
      if (effect.dispatchEquipmentHooks !== undefined && typeof effect.dispatchEquipmentHooks !== 'boolean') next.push(`${label}: damage.dispatchEquipmentHooks 必须是布尔值`);
    }
    if (kind === 'adjust_skill_cooldown') {
      const amount = asNumber(effect.amount ?? effect.value, 0);
      if (!Number.isInteger(amount) || amount === 0) next.push(`${label}: adjust_skill_cooldown.amount 必须是非零整数`);
      const selectionMode = asString(effect.selectionMode || 'all');
      if (!SKILL_COOLDOWN_SELECTION_MODES.includes(selectionMode)) next.push(`${label}: adjust_skill_cooldown.selectionMode 非法 ${selectionMode}`);
      const maxSkills = asNumber(effect.maxSkills, 0);
      if (!Number.isInteger(maxSkills) || maxSkills < 0) next.push(`${label}: adjust_skill_cooldown.maxSkills 必须是非负整数`);
      for (const category of asStringList(effect.skillCategories)) {
        if (!skillCategoryOptions.value.includes(category)) next.push(`${label}: adjust_skill_cooldown.skillCategories 未定义 ${category}`);
      }
    }
    if (kind === 'transfer_skill_cooldown') {
      const amount = asNumber(effect.amount, 0);
      const from = isRecord(effect.from) ? effect.from : {};
      const to = isRecord(effect.to) ? effect.to : {};
      if (!Number.isInteger(amount) || amount < 1) next.push(`${label}: transfer_skill_cooldown.amount 必须是正整数`);
      if (effect.requireBoth !== true) next.push(`${label}: transfer_skill_cooldown.requireBoth 必须启用`);
      if (from.excludeContextSkill !== true) next.push(`${label}: transfer_skill_cooldown.from.excludeContextSkill 必须启用`);
      if (from.requireCooling !== true) next.push(`${label}: transfer_skill_cooldown.from.requireCooling 必须启用`);
      if (asString(from.selectionMode) !== 'max_remaining') next.push(`${label}: transfer_skill_cooldown.from.selectionMode 必须为 max_remaining`);
      if (asNumber(from.maxSkills, 0) !== 1) next.push(`${label}: transfer_skill_cooldown.from.maxSkills 必须为 1`);
      if (asString(to.selectionMode) !== 'context_skill') next.push(`${label}: transfer_skill_cooldown.to.selectionMode 必须为 context_skill`);
      for (const category of [...asStringList(from.skillCategories), ...asStringList(to.skillCategories)]) {
        if (!skillCategoryOptions.value.includes(category)) next.push(`${label}: transfer_skill_cooldown.skillCategories 未定义 ${category}`);
      }
      const waiverBuffId = asString(effect.waiveIncreaseBuffId).trim();
      if (waiverBuffId && !buffOptions.value.includes(waiverBuffId)) next.push(`${label}: transfer_skill_cooldown.waiveIncreaseBuffId 未定义 ${waiverBuffId}`);
      if (waiverBuffId && effect.consumeWaiverBuff !== true) next.push(`${label}: transfer_skill_cooldown.consumeWaiverBuff 必须启用`);
    }
    if (kind === 'extra_turn') {
      const targetMode = asString(effect.targetMode || 'self');
      const turnMode = asString(effect.turnMode || 'full');
      if (targetMode !== 'self') next.push(`${label}: extra_turn.targetMode 目前仅支持 self`);
      if (turnMode !== 'full') next.push(`${label}: extra_turn.turnMode 目前仅支持 full`);
    }
    if (kind === 'extra_enemy_refresh') {
      if (!BATTLE_ENEMY_REFRESH_POOL_OPTIONS.includes(asString(effect.enemyPool))) next.push(`${label}: extra_enemy_refresh.enemyPool 非法`);
      if (asString(effect.partyStackMode) !== 'highest') next.push(`${label}: extra_enemy_refresh.partyStackMode 必须为 highest`);
      if (asString(effect.settlementMode) !== 'normal') next.push(`${label}: extra_enemy_refresh.settlementMode 必须为 normal`);
      const maxCount = asNumber(effect.maxCount, 0);
      if (!Number.isInteger(maxCount) || maxCount < 1 || maxCount > 8) next.push(`${label}: extra_enemy_refresh.maxCount 必须是 1~8 的整数`);
      const formula = isRecord(effect.countFormula) ? effect.countFormula : {};
      const terms = asArray(formula.terms).filter(isRecord);
      if (!Number.isInteger(Number(formula.baseAdd)) || Number(formula.baseAdd) < 1 || terms.length !== 1) next.push(`${label}: extra_enemy_refresh.countFormula 必须包含基础数量和一个强化项`);
      const term = terms[0] || {};
      if (asString(term.attrKey) !== 'equipment_enhance_level' || asNumber(term.coefficient, 0) <= 0 || asNumber(term.perSkillLevel, 0) !== 0 || term.perEnhanceLevel !== undefined) next.push(`${label}: extra_enemy_refresh.countFormula 仅支持装备强化等级正向成长`);
      if (Number.isInteger(maxCount) && Number.isInteger(Number(formula.baseAdd)) && Number(formula.baseAdd) >= 1 && asNumber(term.coefficient, 0) > 0) {
        const formulaAtMax = Math.floor(Number(formula.baseAdd) + asNumber(term.coefficient, 0) * Math.max(0, enhanceMaxLevel));
        if (maxCount < formulaAtMax) next.push(`${label}: extra_enemy_refresh.maxCount 会截断强化数量曲线`);
      }
      for (const key of ['procChance', 'procChanceBase', 'procChanceMin', 'procChanceMax', 'procChanceFormula']) {
        if (effect[key] !== undefined) next.push(`${label}: extra_enemy_refresh 不支持 ${key}`);
      }
      validateFormulaNode(`${label}: extra_enemy_refresh.countFormula`, effect.countFormula);
    }
    if (kind === 'summon') {
      const payload = isRecord(effect.summonPayload) ? effect.summonPayload : effect;
      const unitId = asString(payload.unitId).trim();
      const durationTurns = Math.trunc(asNumber(payload.durationTurns, 0));
      const maxCountPerUnit = asNumber(payload.maxCountPerUnit, 0);
      const statsFormula = isRecord(payload.statsFormula) ? payload.statsFormula : {};
      if (!unitId) next.push(`${label}: summonPayload.unitId 不能为空`);
      if (unitId && !enemyOptions.value.includes(unitId)) next.push(`${label}: summonPayload.unitId 未定义 ${unitId}`);
      if (durationTurns < 0) next.push(`${label}: summonPayload.durationTurns 不能小于 0`);
      if (!Number.isInteger(maxCountPerUnit) || maxCountPerUnit < 0) next.push(`${label}: summonPayload.maxCountPerUnit 必须是非负整数`);
      validateFormulaNode(`${label}: summonPayload.countFormula`, payload.countFormula);
      validateFormulaNode(`${label}: summonPayload.durationFormula`, payload.durationFormula);
      validateSummonMaxCountFormulaNode(`${label}: summonPayload.maxCountFormula`, payload.maxCountFormula);
      for (const [attrKey, formula] of Object.entries(statsFormula)) {
        if (!attributeOptions.value.includes(attrKey)) next.push(`${label}: summonPayload.statsFormula 属性未定义 ${attrKey}`);
        if (!isRecord(formula)) next.push(`${label}: summonPayload.statsFormula.${attrKey} 公式非法`);
        validateFormulaNode(`${label}: summonPayload.statsFormula.${attrKey}`, formula);
      }
    }
    if (kind === 'player_clone') {
      if (Math.trunc(asNumber(effect.durationTurns, 1)) < 1) next.push(`${label}: player_clone.durationTurns 必须大于 0`);
      validateFormulaNode(`${label}: player_clone.durationFormula`, effect.durationFormula);
      const statsFormula = isRecord(effect.statsFormula) ? effect.statsFormula : {};
      for (const [attrKey, formula] of Object.entries(statsFormula)) {
        if (!attributeOptions.value.includes(attrKey)) next.push(`${label}: player_clone.statsFormula 属性未定义 ${attrKey}`);
        if (!isRecord(formula)) next.push(`${label}: player_clone.statsFormula.${attrKey} 公式非法`);
        validateFormulaNode(`${label}: player_clone.statsFormula.${attrKey}`, formula);
      }
    }
    if (kind === 'mirror_cast_skill') {
      for (const skillId of asStringList(effect.skillIds)) {
        if (!skillOptions.value.includes(skillId)) next.push(`${label}: mirror_cast_skill.skillIds 未定义 ${skillId}`);
      }
      for (const category of asStringList(effect.skillCategories)) {
        if (!skillCategoryOptions.value.includes(category)) next.push(`${label}: mirror_cast_skill.skillCategories 未定义 ${category}`);
      }
      for (const key of ['amountRatio', 'amountRatioBase', 'amountRatioPerEnhanceLevel', 'mpCostMultiplier', 'hpCostMultiplier']) {
        if (effect[key] !== undefined && (!Number.isFinite(Number(effect[key])) || Number(effect[key]) < 0)) next.push(`${label}: mirror_cast_skill.${key} 必须是非负数`);
      }
    }
    if (kind === 'execute') {
      const threshold = asNumber(effect.hpPercentThreshold, Number.NaN);
      const perEnhance = asNumber(effect.hpPercentPerEnhanceLevel, 0);
      if (!Number.isFinite(threshold) || threshold <= 0 || threshold >= 1) next.push(`${label}: execute.hpPercentThreshold 必须在 0~1 之间`);
      if (!Number.isFinite(perEnhance) || perEnhance < 0 || perEnhance >= 1) next.push(`${label}: execute.hpPercentPerEnhanceLevel 非法`);
    }
    if (kind === 'add_probability' || kind === 'set_probability') {
      const value = Number(effect.value);
      if (!Number.isFinite(value)) next.push(`${label}: ${kind}.value 必须是有限数值`);
      if (kind === 'set_probability' && (value < 0 || value > 1)) next.push(`${label}: set_probability.value 必须在 0~1 之间`);
      validateFormulaNode(`${label}: ${kind}.valueFormula`, effect.valueFormula ?? effect.amountFormula);
      validateProbabilityFilters(`${label}: ${kind}`, effect);
    }
    if (kind === 'force_probability_failure') validateProbabilityFilters(`${label}: force_probability_failure`, effect);
    if (effect.maxSuccessesPerTurn !== undefined) {
      const maxSuccesses = Number(effect.maxSuccessesPerTurn);
      if (!Number.isInteger(maxSuccesses) || maxSuccesses < 1) next.push(`${label}: maxSuccessesPerTurn 必须是正整数`);
    }
  };
  const validateApplyChance = (label: string, payload: DataRow) => {
    const hasChance = payload.applyChanceBase !== undefined
      || payload.applyChanceFormula !== undefined
      || payload.applyChancePerSkillLevel !== undefined
      || payload.applyChanceMin !== undefined
      || payload.applyChanceMax !== undefined;
    if (!hasChance) return;
    const base = Number(payload.applyChanceBase ?? 0);
    const min = Number(payload.applyChanceMin ?? 0);
    const max = Number(payload.applyChanceMax ?? 1);
    if (!Number.isFinite(base) || base < 0 || base > 1) next.push(`${label}: applyChanceBase 必须在 0~1 之间`);
    if (!Number.isFinite(min) || min < 0 || min > 1) next.push(`${label}: applyChanceMin 必须在 0~1 之间`);
    if (!Number.isFinite(max) || max < 0 || max > 1) next.push(`${label}: applyChanceMax 必须在 0~1 之间`);
    if (Number.isFinite(min) && Number.isFinite(max) && min > max) next.push(`${label}: 概率下限不能大于上限`);
    validateFormulaNode(`${label}: applyChanceFormula`, payload.applyChanceFormula);
  };
  const validateBattleSkillEffect = (label: string, effect: DataRow) => {
    const kind = asString(effect.kind).trim();
    if (!EFFECT_KINDS.includes(kind)) next.push(`${label}: effect.kind 非法 ${kind || '(空)'}`);
    if (kind === 'summon') {
      const payload = isRecord(effect.summonPayload) ? effect.summonPayload : {};
      validateSummonUnitSelectorNode(label, payload);
      validateFormulaNode(`${label}: summonPayload.countFormula`, payload.countFormula);
      validateFormulaNode(`${label}: summonPayload.durationFormula`, payload.durationFormula);
      validateSummonMaxCountFormulaNode(`${label}: summonPayload.maxCountFormula`, payload.maxCountFormula);
      const durationTurns = Number(payload.durationTurns ?? 0);
      if (!Number.isInteger(durationTurns) || durationTurns < 0) next.push(`${label}: summonPayload.durationTurns 必须是非负整数`);
      const maxCount = Number(payload.maxCountPerUnit ?? 0);
      if (!Number.isInteger(maxCount) || maxCount < 0) next.push(`${label}: summonPayload.maxCountPerUnit 必须是非负整数`);
    }
    if (kind === 'status') {
      const payload = isRecord(effect.statusPayload) ? effect.statusPayload : {};
      validateFormulaNode(`${label}: statusPayload.durationFormula`, payload.durationFormula);
      validateApplyChance(`${label}: statusPayload`, payload);
    }
    if (kind === 'weighted_random_per_target') {
      validateApplyChance(label, effect);
      const choices = asArray(effect.choices).filter(isRecord);
      if (choices.length === 0) next.push(`${label}: choices 不能为空`);
      const choiceIds = new Set<string>();
      let totalWeight = 0;
      for (const [choiceIndex, choice] of choices.entries()) {
        const choiceLabel = `${label}: choices #${choiceIndex + 1}`;
        const choiceId = asString(choice.id).trim();
        const statusId = asString(choice.statusId).trim();
        const weight = Number(choice.weight);
        const durationTurns = Number(choice.durationTurns);
        if (!choiceId) next.push(`${choiceLabel}: id 不能为空`);
        if (choiceId && choiceIds.has(choiceId)) next.push(`${label}: choices.id 重复 ${choiceId}`);
        choiceIds.add(choiceId);
        if (!Number.isFinite(weight) || weight <= 0) next.push(`${choiceLabel}: weight 必须大于 0`);
        else totalWeight += weight;
        if (!statusId) next.push(`${choiceLabel}: statusId 不能为空`);
        if (statusId && !buffOptions.value.includes(statusId)) next.push(`${choiceLabel}: statusId 未定义 ${statusId}`);
        if (!Number.isInteger(durationTurns) || durationTurns < 0) next.push(`${choiceLabel}: durationTurns 必须是非负整数`);
      }
      if (choices.length > 0 && totalWeight <= 0) next.push(`${label}: choices 总权重必须大于 0`);
    }
  };
  for (const table of EDITABLE_TABLES) {
    const ids = new Set<string>();
    for (let index = 0; index < catalog[table].length; index += 1) {
      const row = catalog[table][index];
      const id = rowId(row);
      if (!id) next.push(`${TABLE_LABELS[table]} #${index + 1}: 缺少 id`);
      if (id && ids.has(id)) next.push(`${TABLE_LABELS[table]}: 重复 id ${id}`);
      if (id) ids.add(id);
      if (table === 'skills' && asArray(row.effects).length === 0) next.push(`技能 ${id}: effects 不能为空`);
      if (table === 'skills') {
        if (row.enemyOnly !== undefined && typeof row.enemyOnly !== 'boolean') next.push(`技能 ${id}: enemyOnly 必须是布尔值`);
        next.push(...validateEnemyOnlyDamageReductionPierceSkill(row, buffOptions.value));
        next.push(...validateEnemyOnlyTargetPrioritySkill(row));
        next.push(...validateSkillEffectStableIds(id, asArray(row.effects)));
        for (const [effectIndex, effect] of asArray(row.effects).filter(isRecord).entries()) {
          const kind = asString(effect.kind).trim();
          const damageSpec = isRecord(effect.damageSpec) ? effect.damageSpec : {};
          next.push(...validateHitDamageScaling(damageSpec.hitDamageScaling, `技能 ${id}: effects #${effectIndex + 1}.damageSpec`));
          if (kind !== 'summon') continue;
          const payload = isRecord(effect.summonPayload) ? effect.summonPayload : {};
          const durationTurns = Math.trunc(asNumber(payload.durationTurns, 0));
          const maxCountPerUnit = asNumber(payload.maxCountPerUnit, 0);
          const statsFormula = isRecord(payload.statsFormula) ? payload.statsFormula : {};
          validateSummonUnitSelectorNode(`技能 ${id}`, payload);
          if (durationTurns < 0) next.push(`技能 ${id}: summonPayload.durationTurns 不能小于 0`);
          if (!Number.isInteger(maxCountPerUnit) || maxCountPerUnit < 0) next.push(`技能 ${id}: summonPayload.maxCountPerUnit 必须是非负整数`);
          validateSummonMaxCountFormulaNode(`技能 ${id}: summonPayload.maxCountFormula`, payload.maxCountFormula);
          for (const [attrKey, formula] of Object.entries(statsFormula)) {
            if (!attributeOptions.value.includes(attrKey)) next.push(`技能 ${id}: summonPayload.statsFormula 属性未定义 ${attrKey}`);
            if (!isRecord(formula)) next.push(`技能 ${id}: summonPayload.statsFormula.${attrKey} 公式非法`);
          }
          const statsKeys = Object.keys(statsFormula);
          if (new Set(statsKeys).size !== statsKeys.length) next.push(`技能 ${id}: summonPayload.statsFormula 存在重复属性`);
        }
      }
      if (table === 'actions') {
        const actionCosts = isRecord(row.costs) ? row.costs : {};
        for (const itemID of Object.keys(actionCosts)) {
          if (!itemOptions.value.includes(itemID)) next.push(`动作 ${id}: costs 引用了未定义物品 ${itemID}`);
        }
        if (row.materialInstanceRequirements !== undefined) {
          if (!isRecord(row.materialInstanceRequirements)) {
            next.push(`动作 ${id}: materialInstanceRequirements 必须是对象`);
          } else {
            for (const [itemID, rawRequirement] of Object.entries(row.materialInstanceRequirements)) {
              const label = `动作 ${id}: materialInstanceRequirements.${itemID}`;
              if (!Object.prototype.hasOwnProperty.call(actionCosts, itemID)) next.push(`${label} 不在 costs 中`);
              if (!equipmentIds.has(itemID)) next.push(`${label} 不是已定义装备`);
              if (!isRecord(rawRequirement)) {
                next.push(`${label} 必须是对象`);
                continue;
              }
              const enhanceLevel = Number(rawRequirement.enhanceLevel);
              if (!Number.isInteger(enhanceLevel) || enhanceLevel < 0) next.push(`${label}.enhanceLevel 必须是非负整数`);
              if (typeof rawRequirement.requireTemplateAffixes !== 'boolean') next.push(`${label}.requireTemplateAffixes 必须是布尔值`);
              if (rawRequirement.durability !== 'full') next.push(`${label}.durability 目前仅支持 full`);
            }
          }
        }
        for (const itemID of Object.keys(isRecord(row.itemReward) ? row.itemReward : {})) {
          if (!itemOptions.value.includes(itemID)) next.push(`动作 ${id}: itemReward 引用了未定义物品 ${itemID}`);
        }
        const mainOutput = asString(row.mainOutput);
        if (mainOutput && !itemOptions.value.includes(mainOutput)) next.push(`动作 ${id}: mainOutput 未定义 ${mainOutput}`);
        if (mainOutput && isRecord(row.itemReward) && !Object.prototype.hasOwnProperty.call(row.itemReward, mainOutput)) next.push(`动作 ${id}: mainOutput 不在 itemReward 中`);
        const outputItem = catalog.items.find((item) => rowId(item) === mainOutput);
        const outputFeed = outputItem && isRecord(outputItem.pondFeed) && Object.keys(outputItem.pondFeed).length > 0 ? outputItem.pondFeed : null;
        const feedOutputs = Object.keys(isRecord(row.itemReward) ? row.itemReward : {}).filter(itemID => { const item = catalog.items.find(item => rowId(item) === itemID); return item && isRecord(item.pondFeed) && Object.keys(item.pondFeed).length > 0; });
        if (feedOutputs.length && (feedOutputs.length !== 1 || feedOutputs[0] !== mainOutput)) next.push(`动作 ${id}: 鱼粮必须是唯一鱼粮产物并设为主产物`);
        if (outputFeed) {
          const buildingRequirement = isRecord(row.buildingRequirement) ? row.buildingRequirement : {};
          if (Object.prototype.hasOwnProperty.call(buildingRequirement, 'fish_pond')) next.push(`动作 ${id}: 普通鱼粮不得要求鱼塘建筑等级`);
          const rewards = isRecord(row.itemReward) ? row.itemReward : {};
          const rawReward = rewards[mainOutput];
          const outputCount = isRecord(rawReward) ? Number(rawReward.count) : Number(rawReward);
          if (!Number.isInteger(outputCount) || outputCount <= 0) next.push(`动作 ${id}: 鱼粮产量必须是正整数`);
          let finishedFeedInputs = 0;
          let inputValue = 0;
          let hasLootInput = false;
          let hasPlayerOutput = false;
          if (Object.keys(actionCosts).length === 0) next.push(`动作 ${id}: 鱼粮配方必须消耗材料`);
          for (const [itemID, quantity] of Object.entries(actionCosts)) {
            if (!Number.isInteger(Number(quantity)) || Number(quantity) < outputCount) next.push(`动作 ${id}: 产量不得超过材料 ${itemID} 的数量`);
            const ingredient = catalog.items.find((item) => rowId(item) === itemID);
            if (ingredient) {
              if (isRecord(ingredient.pondFeed) && Object.keys(ingredient.pondFeed).length > 0) finishedFeedInputs += 1;
              const isLoot = asStringList(ingredient.type).includes('loot') || asString(ingredient.category) === 'loot';
              if (isLoot) hasLootInput = true;
              if (!isLoot && !(isRecord(ingredient.pondFeed) && Object.keys(ingredient.pondFeed).length) && catalog.actions.some(action => !action.disabled && isRecord(action.itemReward) && Object.prototype.hasOwnProperty.call(action.itemReward, itemID))) hasPlayerOutput = true;
              inputValue += Number(quantity) * Number(ingredient.economyBaseValue || 0);
            }
          }
          if (finishedFeedInputs > 1) next.push(`动作 ${id}: 最多只能使用一种成品鱼粮`);
          const outputValue = outputCount * Number(outputItem?.economyBaseValue || 0);
          if (!Number.isFinite(outputValue) || outputValue <= 0 || inputValue < outputValue * 1.2) next.push(`动作 ${id}: 鱼粮材料基础价值至少应为产物的 120%`);
          if (asString(outputFeed.rulesetVersion) === 'pond-feed-v2' && (!hasLootInput || !hasPlayerOutput)) next.push(`动作 ${id}: pond-feed-v2 配方需要战利品和非成品鱼粮的玩家产出`);
        }
      }
      if (table === 'equipment_templates' && !asString(row.slot)) next.push(`装备 ${id}: slot 不能为空`);
      if (table === 'equipment_templates') {
        if (id && !itemIds.has(id)) next.push(`装备 ${id}: 缺少同 ID 物品`);
        next.push(...validatePlayerOwnedDamageSpecs(row.basicAttackEffects, `装备 ${id}: basicAttackEffects`));
        next.push(...validatePlayerOwnedTargetPriority(row.basicAttackEffects, `装备 ${id}: basicAttackEffects`));
        for (const [effectIndex, effect] of asArray(row.basicAttackEffects).filter(isRecord).entries()) {
          const damageSpec = isRecord(effect.damageSpec) ? effect.damageSpec : {};
          next.push(...validateHitDamageScaling(damageSpec.hitDamageScaling, `装备 ${id}: basicAttackEffects #${effectIndex + 1}.damageSpec`));
        }
        const basicAttackFormula = isRecord(row.basicAttackFormula) ? row.basicAttackFormula : {};
        const basicAttackDamageSpec = isRecord(basicAttackFormula.damageSpec) ? basicAttackFormula.damageSpec : null;
        next.push(...validateHitDamageScaling(basicAttackDamageSpec?.hitDamageScaling, `装备 ${id}: basicAttackFormula.damageSpec`));
        const forbiddenBasicAttackFields = configuredEnemyOnlyDamageReductionPierceFields(basicAttackDamageSpec);
        if (forbiddenBasicAttackFields.length > 0) {
          next.push(`装备 ${id}: basicAttackFormula.damageSpec 的 ${forbiddenBasicAttackFields.join('、')} 仅允许怪物专属技能使用`);
        }
        for (const target of asStringList(row.equipTargets)) {
          if (!EQUIP_TARGETS.includes(target)) next.push(`装备 ${id}: equipTargets 非法值 ${target}`);
        }
        const secondary = asString(row.equipmentSecondaryClass);
        const hasWeaponClass = asStringList(row.weaponClass).length > 0;
        if (secondary && !equipmentSecondaryClassOptions.value.includes(secondary)) {
          next.push(`装备 ${id}: equipmentSecondaryClass 未定义 ${secondary}`);
        }
        if (!hasWeaponClass && !secondary) {
          next.push(`装备 ${id}: 非武器装备缺少 equipmentSecondaryClass`);
        }
        const affixSlotCount = Math.max(0, Math.trunc(asNumber(row.affixSlotCount, 0)));
        const seenDefaultAffixSlots = new Set<number>();
        for (const [slotIndex, slot] of asArray(row.defaultAffixSlots).filter(isRecord).entries()) {
          const indexValue = Math.trunc(asNumber(slot.slotIndex, slotIndex));
          const affixId = asString(slot.affixId).trim();
          const rarityId = asString(slot.rarityId).trim();
          if (indexValue < 0) next.push(`装备 ${id}: defaultAffixSlots.slotIndex 不能小于 0`);
          if (affixSlotCount > 0 && indexValue >= affixSlotCount) next.push(`装备 ${id}: defaultAffixSlots.slotIndex 超出词条槽数 ${indexValue}`);
          if (seenDefaultAffixSlots.has(indexValue)) next.push(`装备 ${id}: defaultAffixSlots 存在重复槽位 ${indexValue}`);
          seenDefaultAffixSlots.add(indexValue);
          if (!affixId) next.push(`装备 ${id}: defaultAffixSlots.affixId 不能为空`);
          if (affixId && !affixOptions.value.includes(affixId)) next.push(`装备 ${id}: defaultAffixSlots.affixId 未定义 ${affixId}`);
          if (!rarityId) next.push(`装备 ${id}: defaultAffixSlots.rarityId 不能为空`);
          if (rarityId && !rarityOptions.value.includes(rarityId)) next.push(`装备 ${id}: defaultAffixSlots.rarityId 未定义 ${rarityId}`);
          if ('rollValue' in slot && !Number.isFinite(asNumber(slot.rollValue, Number.NaN))) next.push(`装备 ${id}: defaultAffixSlots.rollValue 非法`);
        }
        const milestonePools = isRecord(row.enhanceAffixMilestonePools) ? row.enhanceAffixMilestonePools : {};
        for (const [levelText, pool] of Object.entries(milestonePools)) {
          const level = Math.trunc(asNumber(levelText, Number.NaN));
          if (!Number.isFinite(level) || level < 1) next.push(`装备 ${id}: enhanceAffixMilestonePools 等级非法 ${levelText}`);
          if (asNumber(row.enhanceMaxLevel, 0) > 0 && level > asNumber(row.enhanceMaxLevel, 0)) next.push(`装备 ${id}: enhanceAffixMilestonePools 等级超过强化上限 ${levelText}`);
          if (!Array.isArray(pool)) next.push(`装备 ${id}: enhanceAffixMilestonePools.${levelText} 必须是数组`);
          for (const entry of asArray(pool).filter(isRecord)) {
            const affixId = asString(entry.affixId).trim();
            const rarityId = asString(entry.rarityId).trim();
            if (!affixId) next.push(`装备 ${id}: enhanceAffixMilestonePools.${levelText}.affixId 不能为空`);
            if (affixId && !affixOptions.value.includes(affixId)) next.push(`装备 ${id}: enhanceAffixMilestonePools.${levelText}.affixId 未定义 ${affixId}`);
            if (!rarityId) next.push(`装备 ${id}: enhanceAffixMilestonePools.${levelText}.rarityId 不能为空`);
            if (rarityId && !rarityOptions.value.includes(rarityId)) next.push(`装备 ${id}: enhanceAffixMilestonePools.${levelText}.rarityId 未定义 ${rarityId}`);
            if ('rollValue' in entry && !Number.isFinite(asNumber(entry.rollValue, Number.NaN))) next.push(`装备 ${id}: enhanceAffixMilestonePools.${levelText}.rollValue 非法`);
          }
        }
        next.push(...validateSkillEffectStableIds(`equipment:${id}`, asArray(row.basicAttackEffects)));
        for (const [effectIndex, effect] of asArray(row.basicAttackEffects).filter(isRecord).entries()) {
          validateBattleSkillEffect(`装备 ${id}: basicAttackEffects #${effectIndex + 1}`, effect);
        }
        for (const [hookIndex, hook] of asArray(row.battleHooks).filter(isRecord).entries()) {
          const hookLabel = `装备 ${id}: battleHooks #${hookIndex + 1}`;
          const trigger = asString(hook.trigger).trim();
          if (!trigger || !BATTLE_HOOK_TRIGGERS.includes(trigger)) next.push(`${hookLabel}: trigger 非法 ${trigger || '(空)'}`);
          for (const key of ['maxTriggersPerTurn', 'maxTriggersPerWave', 'maxTriggersPerBattle']) {
            if (hook[key] === undefined) continue;
            const limit = Number(hook[key]);
            if (!Number.isInteger(limit) || limit < 1) next.push(`${hookLabel}: ${key} 必须是正整数`);
          }
          if (hook.cooldownCompletedTurns !== undefined) {
            const cooldown = Number(hook.cooldownCompletedTurns);
            if (!Number.isInteger(cooldown) || cooldown < 1 || cooldown > 99) next.push(`${hookLabel}: cooldownCompletedTurns 必须为 1-99 的整数`);
          }
          if (hook.activationCost !== undefined) {
            if (!isRecord(hook.activationCost)) {
              next.push(`${hookLabel}: activationCost 必须是对象`);
            } else {
              const hpRatio = asNumber(hook.activationCost.maxHpRatio, 0);
              const mpRatio = asNumber(hook.activationCost.maxMpRatio, 0);
              if (hpRatio < 0 || hpRatio > 1 || mpRatio < 0 || mpRatio > 1 || (hpRatio === 0 && mpRatio === 0)) next.push(`${hookLabel}: activationCost 必须配置 0-1 之间的生命或魔法比例`);
            }
          }
          if (hook.sharedTriggerGroup !== undefined && !asString(hook.sharedTriggerGroup).trim()) next.push(`${hookLabel}: sharedTriggerGroup 不能为空`);
          if (hook.subjectFilter !== undefined) {
            if (!isRecord(hook.subjectFilter)) next.push(`${hookLabel}: subjectFilter 必须是对象`);
            const subjectFilter = isRecord(hook.subjectFilter) ? hook.subjectFilter : {};
            if (subjectFilter.requireAlive !== undefined && typeof subjectFilter.requireAlive !== 'boolean') next.push(`${hookLabel}: subjectFilter.requireAlive 必须是布尔值`);
            for (const key of ['includeUnitTypes', 'excludeUnitTypes']) {
              const values = subjectFilter[key];
              if (values !== undefined && !Array.isArray(values)) next.push(`${hookLabel}: subjectFilter.${key} 必须是数组`);
              for (const unitType of asStringList(values)) {
                if (!BATTLE_HOOK_UNIT_FILTER_TYPES.includes(unitType)) next.push(`${hookLabel}: subjectFilter.${key} 类型非法 ${unitType}`);
              }
            }
            for (const key of ['includeSummonTemplateIds', 'excludeSummonTemplateIds']) {
              const values = subjectFilter[key];
              if (values !== undefined && !Array.isArray(values)) next.push(`${hookLabel}: subjectFilter.${key} 必须是数组`);
              for (const templateId of asStringList(values)) {
                if (!enemyOptions.value.includes(templateId)) next.push(`${hookLabel}: subjectFilter.${key} 召唤模板未定义 ${templateId}`);
              }
            }
          }
          if (hook.eventFilter !== undefined) {
            if (!isRecord(hook.eventFilter)) next.push(`${hookLabel}: eventFilter 必须是对象`);
            const eventFilter = isRecord(hook.eventFilter) ? hook.eventFilter : {};
            if (eventFilter.excludeSelf !== undefined && typeof eventFilter.excludeSelf !== 'boolean') next.push(`${hookLabel}: eventFilter.excludeSelf 必须是布尔值`);
            const buffIds = asStringList(eventFilter.buffIds);
            if (eventFilter.buffIds !== undefined && buffIds.length === 0) next.push(`${hookLabel}: eventFilter.buffIds 不能为空`);
            for (const buffId of buffIds) {
              if (!buffOptions.value.includes(buffId)) next.push(`${hookLabel}: eventFilter.buffIds 未定义 ${buffId}`);
            }
            const applicationKinds = asStringList(eventFilter.applicationKinds);
            if (eventFilter.applicationKinds !== undefined && applicationKinds.length === 0) next.push(`${hookLabel}: eventFilter.applicationKinds 不能为空`);
            for (const applicationKind of applicationKinds) {
              if (!BATTLE_HOOK_APPLICATION_KINDS.includes(applicationKind)) next.push(`${hookLabel}: eventFilter.applicationKinds 非法 ${applicationKind}`);
            }
            const turnKinds = asStringList(eventFilter.turnKinds);
            if (eventFilter.turnKinds !== undefined && turnKinds.length === 0) next.push(`${hookLabel}: eventFilter.turnKinds 不能为空`);
            if (turnKinds.length > 0 && trigger !== 'on_turn_start' && trigger !== 'on_turn_end') next.push(`${hookLabel}: eventFilter.turnKinds 仅适用于回合触发器`);
            for (const turnKind of turnKinds) {
              if (!BATTLE_HOOK_TURN_KINDS.includes(turnKind)) next.push(`${hookLabel}: eventFilter.turnKinds 非法 ${turnKind}`);
            }
            if (eventFilter.includePreparation !== undefined && (typeof eventFilter.includePreparation !== 'boolean' || (trigger !== 'on_turn_start' && trigger !== 'on_turn_end'))) next.push(`${hookLabel}: eventFilter.includePreparation 非法`);
            if (eventFilter.includeSuppressedTurns !== undefined && (typeof eventFilter.includeSuppressedTurns !== 'boolean' || trigger !== 'on_turn_end')) next.push(`${hookLabel}: eventFilter.includeSuppressedTurns 非法`);
            validateProbabilityFilters(`${hookLabel}: eventFilter`, eventFilter);
            const hasProbabilityFilter = eventFilter.probabilityKinds !== undefined || eventFilter.sourceIds !== undefined || eventFilter.excludeSourceIds !== undefined;
            if (hasProbabilityFilter && !['on_before_probability_check', 'on_probability_success', 'on_probability_failure'].includes(trigger)) {
              next.push(`${hookLabel}: 概率 eventFilter 仅适用于概率触发器`);
            }
          }
          validateHookConditions(hookLabel, hook.conditions);
          const hookEffects = asArray(hook.effects).filter(isRecord);
          if (trigger === 'on_enemy_refresh') {
            if (hookEffects.length !== 1 || asArray(hook.conditions).length !== 0 || asString(hookEffects[0]?.kind || hookEffects[0]?.type) !== 'extra_enemy_refresh') {
              next.push(`${hookLabel}: on_enemy_refresh 必须是仅含一个 extra_enemy_refresh 的无条件专用钩子`);
            }
            for (const key of ['maxTriggersPerTurn', 'maxTriggersPerWave', 'maxTriggersPerBattle', 'cooldownCompletedTurns', 'activationCost', 'eventFilter', 'subjectFilter']) {
              if (hook[key] !== undefined) next.push(`${hookLabel}: on_enemy_refresh 不支持 ${key}`);
            }
          }
          for (const effect of hookEffects) {
            if (isRecord(effect.targeting) && Object.prototype.hasOwnProperty.call(effect.targeting, 'ignoreTargetPriority')) {
              next.push(`${hookLabel}: effects.targeting.ignoreTargetPriority 仅允许怪物专属技能使用`);
            }
            validateBattleHookEffect(hookLabel, effect, Math.max(0, Math.trunc(asNumber(row.enhanceMaxLevel, 0))));
            if (asString(effect.kind || effect.type) === 'extra_enemy_refresh' && trigger !== 'on_enemy_refresh') next.push(`${hookLabel}: extra_enemy_refresh 只能用于 on_enemy_refresh`);
          }
        }
      }
      if (table === 'items') {
        const battleID = asString(row.battleUsableItemId);
        if (battleID && !battleUsableItemOptions.value.includes(battleID)) next.push(`物品 ${id}: battleUsableItemId 未定义 ${battleID}`);
        const grantCollectibleID = asString(row.useGrantCollectibleId);
        if (grantCollectibleID) {
          const collectible = catalog.collectibles.find((entry) => rowId(entry) === grantCollectibleID);
          if (!collectible) {
            next.push(`物品 ${id}: useGrantCollectibleId 未定义 ${grantCollectibleID}`);
          } else if (!collectible.upgradeable && asString(collectible.itemId) !== id) {
            next.push(`物品 ${id}: useGrantCollectibleId 指向的收藏品 itemId 不匹配 ${grantCollectibleID}`);
          }
          if (!asStringList(row.type).includes('collectible')) next.push(`物品 ${id}: useGrantCollectibleId 需要 collectible 类型`);
        }
        const kittyFeed = isRecord(row.kittyFeed) ? row.kittyFeed : null;
        if (kittyFeed !== null) {
          const gain = Math.trunc(asNumber(kittyFeed.battleLevelCapGain, 0));
          if (gain > 0 && !asStringList(row.type).includes('food')) next.push(`物品 ${id}: kittyFeed 仅允许 food 类型`);
          if (gain < 0) next.push(`物品 ${id}: kittyFeed.battleLevelCapGain 不能小于 0`);
        }
        const pondFeed = isRecord(row.pondFeed) && Object.keys(row.pondFeed).length > 0 ? row.pondFeed : null;
        if (pondFeed !== null) {
          const allowedFeedFields = new Set(['nutrition', 'feedClass', 'rulesetVersion', 'consumePriority', 'conditions', 'effects', 'role', 'maxPortionsPerFishHour', 'delivery']);
          for (const key of Object.keys(pondFeed)) {
            if (!allowedFeedFields.has(key)) next.push(`物品 ${id}: pondFeed.${key} 不受支持`);
          }
          const nutrition = Number(pondFeed.nutrition);
          const targeted = pondFeed.delivery === 'targeted';
          if (pondFeed.delivery !== undefined && !['feeder', 'targeted'].includes(asString(pondFeed.delivery))) next.push(`物品 ${id}: 投喂方式非法`);
          if (targeted && (pondFeed.rulesetVersion !== 'pond-feed-v2' || pondFeed.role !== 'functional' || nutrition !== 0 || pondFeed.consumePriority !== undefined || pondFeed.maxPortionsPerFishHour !== undefined)) next.push(`物品 ${id}: 定向粮必须启用功能 DSL，营养为零，不配置料仓优先级或小时限量`);
          const feedClass = asString(pondFeed.feedClass);
          if (!Number.isFinite(nutrition) || nutrition < 0 || !targeted && nutrition === 0) next.push(`物品 ${id}: 料仓粮营养必须大于 0`);
          if (!['herbal', 'insect', 'protein', 'arcane', 'recycled'].includes(feedClass)) next.push(`物品 ${id}: pondFeed.feedClass 非法 ${feedClass || '(空)'}`);
          const hasDsl = ['rulesetVersion', 'consumePriority', 'conditions', 'effects'].some((key) => pondFeed[key] !== undefined);
          if (hasDsl && asString(pondFeed.rulesetVersion) !== 'pond-feed-v2') next.push(`物品 ${id}: pondFeed.rulesetVersion 必须是 pond-feed-v2`);
          const priority = Number(pondFeed.consumePriority ?? 0);
          if (!Number.isInteger(priority) || priority < -100 || priority > 100) next.push(`物品 ${id}: pondFeed.consumePriority 必须是 -100~100 的整数`);
          const conditionRows = pondFeed.conditions === undefined ? [] : Array.isArray(pondFeed.conditions) ? pondFeed.conditions : [];
          if (pondFeed.conditions !== undefined && !Array.isArray(pondFeed.conditions)) next.push(`物品 ${id}: pondFeed.conditions 必须是数组`);
          for (const [conditionIndex, condition] of conditionRows.entries()) {
            const label = `物品 ${id}: pondFeed.conditions #${conditionIndex + 1}`;
            if (!isRecord(condition)) { next.push(`${label} 必须是对象`); continue; }
            for (const key of Object.keys(condition)) {
              if (!['field', 'op', 'value'].includes(key)) next.push(`${label}.${key} 不受支持`);
            }
            const field = asString(condition.field);
            const op = asString(condition.op);
            const stringField = field === 'dietClass' || field === 'pondZone';
            const allowedOps = stringField ? ['eq', 'neq'] : ['eq', 'neq', 'lt', 'lte', 'gt', 'gte'];
            if (!allowedOps.includes(op)) next.push(`${label}.op 非法 ${op}`);
            if (!['dietClass', 'pondZone', 'health', 'satiety', 'condition'].includes(field)) next.push(`${label}.field 非法 ${field}`);
            if (stringField && typeof condition.value !== 'string') next.push(`${label}.value 必须是字符串`);
            if (field === 'dietClass' && !['herbal', 'insect', 'protein', 'arcane'].includes(asString(condition.value))) next.push(`${label}.value 食性类别非法`);
            if (field === 'pondZone' && !['main', 'nursery'].includes(asString(condition.value))) next.push(`${label}.value 池区非法`);
            if (!stringField) {
              const value = condition.value;
              const bounds = field === 'satiety' ? [0, 100] : field === 'condition' ? [0, 1.12] : [0, 1];
              if (typeof value !== 'number' || !Number.isFinite(value) || value < bounds[0] || value > bounds[1]) next.push(`${label}.value 超出允许范围 ${bounds[0]}~${bounds[1]}`);
            }
          }
          const effectRows = pondFeed.effects === undefined ? [] : Array.isArray(pondFeed.effects) ? pondFeed.effects : [];
          const role = asString(pondFeed.role, effectRows.length ? 'functional' : 'staple');
          if (role === 'functional' && !effectRows.some(effect => isRecord(effect) && Number(effect.valuePerPortion) > 0)) next.push(`物品 ${id}: 功能粮必须包含正面效果`);
          if (!['staple', 'functional'].includes(role) || role === 'staple' && effectRows.length || role === 'functional' && !effectRows.length) next.push(`物品 ${id}: 主粮不能配置效果，功能粮必须配置效果`);
          if (pondFeed.maxPortionsPerFishHour !== undefined && (role !== 'functional' || !Number.isFinite(Number(pondFeed.maxPortionsPerFishHour)) || Number(pondFeed.maxPortionsPerFishHour) <= 0 || Number(pondFeed.maxPortionsPerFishHour) > 0.5)) next.push(`物品 ${id}: 功能粮每小时份数须大于 0 且不超过 0.5`);
          if (effectRows.length > 4 || new Set(effectRows.filter(isRecord).map(effect => effect.stat)).size !== effectRows.length) next.push(`物品 ${id}: 最多四个效果，同一属性不能重复`);
          if (pondFeed.effects !== undefined && !Array.isArray(pondFeed.effects)) next.push(`物品 ${id}: pondFeed.effects 必须是数组`);
          for (const [effectIndex, effect] of effectRows.entries()) {
            const label = `物品 ${id}: pondFeed.effects #${effectIndex + 1}`;
            if (!isRecord(effect)) { next.push(`${label} 必须是对象`); continue; }
            for (const key of Object.keys(effect)) {
              if (!['kind', 'stat', 'mode', 'valuePerPortion', 'maxTargetMultiplier', 'floor', 'ceiling'].includes(key)) next.push(`${label}.${key} 不受支持`);
            }
            const stat = asString(effect.stat);
            const mode = asString(effect.mode);
            const value = Number(effect.valuePerPortion);
            if (asString(effect.kind) !== 'stat_delta') next.push(`${label}.kind 必须是 stat_delta`);
            if (!['lengthCm', 'weightKg', 'health', 'satiety', 'longevityDays', 'naturalProtectionDays'].includes(stat)) next.push(`${label}.stat 非法 ${stat}`);
            if (['longevityDays', 'naturalProtectionDays'].includes(stat) && (!targeted || mode !== 'flat' || value <= 0 || value > 36500 || effect.floor !== undefined || effect.ceiling !== undefined || effect.maxTargetMultiplier !== undefined)) next.push(`${label}: 延寿与护生必须是定向投喂，每份大于 0 且不超过 36500 生物日，不配置上下限`);
            if (!Number.isFinite(value) || value === 0) next.push(`${label}.valuePerPortion 必须是非零有限数值`);
            if (stat === 'lengthCm' || stat === 'weightKg') {
              const multiplier = Number(effect.maxTargetMultiplier);
              if (mode !== 'bonus_band' || value <= 0 || value > 0.2 || !Number.isFinite(multiplier) || multiplier <= 1 || multiplier > 1.15) next.push(`${label}: 成长效果需要不超过 0.2 的正每份值和 1~1.15 的目标倍率`);
              if (effect.floor !== undefined || effect.ceiling !== undefined) next.push(`${label}: 成长效果不支持上下限`);
            }
            if (stat === 'health' || stat === 'satiety') {
              const maximum = stat === 'satiety' ? 100 : 1;
              const maxMagnitude = stat === 'satiety' ? 25 : 0.1;
              if (mode !== 'flat' || Math.abs(value) > maxMagnitude || effect.maxTargetMultiplier !== undefined) next.push(`${label}: ${stat} 效果参数非法`);
              const floor = effect.floor;
              if (value < 0 && floor === undefined) next.push(`${label}: 负面效果必须配置下限`);
              const ceiling = effect.ceiling;
              if (floor !== undefined && (typeof floor !== 'number' || !Number.isFinite(floor) || floor < 0 || floor > maximum)) next.push(`${label}.floor 超出允许范围`);
              if (ceiling !== undefined && (typeof ceiling !== 'number' || !Number.isFinite(ceiling) || ceiling < 0 || ceiling > maximum)) next.push(`${label}.ceiling 超出允许范围`);
              if (typeof floor === 'number' && typeof ceiling === 'number' && floor > ceiling) next.push(`${label}: 上下限倒置`);
            }
          }
        }
        if (row.alchemy !== undefined && !isRecord(row.alchemy)) {
          next.push(`物品 ${id}: alchemy 必须是对象`);
        } else if (isRecord(row.alchemy)) {
          for (const key of ['convertWeight', 'convertWeightMultiplier']) {
            if (row.alchemy[key] === undefined) continue;
            const value = Number(row.alchemy[key]);
            if (!Number.isFinite(value) || value <= 0) next.push(`物品 ${id}: alchemy.${key} 必须是大于 0 的有限数值`);
          }
          if (row.alchemy.convertExcluded !== undefined && typeof row.alchemy.convertExcluded !== 'boolean') {
            next.push(`物品 ${id}: alchemy.convertExcluded 必须是布尔值`);
          }
        }
      }
      if (table === 'alchemy_groups') {
        const normalEntries = asArray(row.entries).filter(isRecord);
        const upgradeEntries = asArray(row.upgradeEntries).filter(isRecord);
        if (row.ultraRareChanceBase !== undefined || row.ultraRareEntries !== undefined) next.push(`炼金转换组 ${id}: 不支持额外超稀有产物分支`);
        if (row.entries !== undefined && !Array.isArray(row.entries)) next.push(`炼金转换组 ${id}: entries 必须是数组`);
        if (row.upgradeEntries !== undefined && !Array.isArray(row.upgradeEntries)) next.push(`炼金转换组 ${id}: upgradeEntries 必须是数组`);
        for (const [field, entries] of [['entries', normalEntries], ['upgradeEntries', upgradeEntries]] as const) {
          for (const entry of entries) {
            const itemId = asString(entry.itemId).trim();
            const weight = Number(entry.weight);
            if (!itemId || !itemIds.has(itemId)) next.push(`炼金转换组 ${id}: ${field}.itemId 未定义 ${itemId || '(空)'}`);
            if (field === 'upgradeEntries' && itemRarityById.get(itemId) === 'mythic') next.push(`炼金转换组 ${id}: 升级产物不能是神话物品 ${itemId}`);
            if (!Number.isFinite(weight) || weight <= 0) next.push(`炼金转换组 ${id}: ${field}.${itemId || '(空)'}.weight 必须大于 0`);
          }
        }
        const upgradeChance = Number(row.upgradeChanceBase ?? 0);
        if (!Number.isFinite(upgradeChance) || upgradeChance < 0 || upgradeChance > 0.25) next.push(`炼金转换组 ${id}: upgradeChanceBase 必须在 0~0.25 之间`);
      }
      if (table === 'alchemy_transmute_rules') {
        const scopedItemIds = asStringList(row.itemIds);
        if (row.itemIds !== undefined && !Array.isArray(row.itemIds)) next.push(`点金规则 ${id}: itemIds 必须是数组`);
        if (new Set(scopedItemIds).size !== scopedItemIds.length) next.push(`点金规则 ${id}: itemIds 存在重复物品`);
        for (const itemId of scopedItemIds) {
          if (!itemIds.has(itemId)) next.push(`点金规则 ${id}: itemIds 未定义 ${itemId}`);
        }
        if (row.default === true && scopedItemIds.length > 0) next.push(`点金规则 ${id}: 默认规则不能声明 itemIds`);
        if (row.default !== undefined && typeof row.default !== 'boolean') next.push(`点金规则 ${id}: default 必须是布尔值`);
        const priority = Number(row.priority ?? 0);
        if (!Number.isInteger(priority)) next.push(`点金规则 ${id}: priority 必须是整数`);
      }
      if (table === 'battle_usable_items') {
        const itemID = asString(row.itemId);
        if (itemID && !itemOptions.value.includes(itemID)) next.push(`战斗可用物品 ${id}: itemId 未定义 ${itemID}`);
        if (row.partyTriggerGroup !== undefined && typeof row.partyTriggerGroup !== 'string') next.push(`战斗可用物品 ${id}: partyTriggerGroup 必须是字符串`);
        if (isRecord(row.targeting) && Object.prototype.hasOwnProperty.call(row.targeting, 'ignoreTargetPriority')) {
          next.push(`战斗可用物品 ${id}: targeting.ignoreTargetPriority 仅允许怪物专属技能使用`);
        }
        next.push(...validatePlayerOwnedTargetPriority(row.effects, `战斗可用物品 ${id}: effects`));
        for (const [effectIndex, effect] of asArray(row.effects).filter(isRecord).entries()) {
          validateBattleSkillEffect(`战斗可用物品 ${id}: effects #${effectIndex + 1}`, effect);
        }
      }
      if (table === 'world') {
        const scheduleIds = new Set<string>();
        for (const [scheduleIndex, schedule] of asArray(row.waveSchedules).filter(isRecord).entries()) {
          const scheduleLabel = `副本 ${id}: waveSchedules #${scheduleIndex + 1}`;
          const scheduleId = asString(schedule.id).trim();
          const every = Number(schedule.every);
          const minCount = Number(schedule.minCount);
          const maxCount = Number(schedule.maxCount);
          if (!scheduleId) next.push(`${scheduleLabel}: id 不能为空`);
          if (scheduleId && scheduleIds.has(scheduleId)) next.push(`副本 ${id}: waveSchedules.id 重复 ${scheduleId}`);
          scheduleIds.add(scheduleId);
          if (!Number.isInteger(every) || every < 1) next.push(`${scheduleLabel}: every 必须是正整数`);
          if (!Number.isInteger(minCount) || minCount < 1) next.push(`${scheduleLabel}: minCount 必须是正整数`);
          if (!Number.isInteger(maxCount) || maxCount < minCount) next.push(`${scheduleLabel}: maxCount 必须是不小于 minCount 的整数`);
          const choices = asArray(schedule.enemyChoices).filter(isRecord);
          if (choices.length === 0) next.push(`${scheduleLabel}: enemyChoices 不能为空`);
          for (const [choiceIndex, choice] of choices.entries()) {
            const enemyId = asString(choice.enemyId).trim();
            const weight = Number(choice.weight);
            if (!enemyId || !enemyOptions.value.includes(enemyId)) next.push(`${scheduleLabel}: enemyChoices #${choiceIndex + 1} 敌人未定义 ${enemyId || '(空)'}`);
            if (!Number.isFinite(weight) || weight <= 0) next.push(`${scheduleLabel}: enemyChoices #${choiceIndex + 1} weight 必须大于 0`);
          }
        }
      }
      if (table === 'enemies' && asArray(row.skillIds).some((skill) => !skillOptions.value.includes(asString(skill)))) next.push(`敌人 ${id}: skillIds 存在未定义技能`);
      if (table === 'buff_effects' && row.buffId && !buffOptions.value.includes(asString(row.buffId))) next.push(`Buff效果 ${id}: buffId 未定义`);
      if (table === 'buff_effects') {
        for (const [effectIndex, effect] of asArray(row.effects).filter(isRecord).entries()) {
          const effectKind = asString(effect.kind || effect.type);
          if (effectKind === 'position_map') {
            const mapping = isRecord(effect.mapping) ? effect.mapping : {};
            for (const position of POSITIONS) {
              const mapped = asString(mapping[position]);
              if (!POSITIONS.includes(mapped)) next.push(`Buff效果 ${id} #${effectIndex + 1}: position_map.${position} 非法 ${mapped || '(空)'}`);
            }
          }
          if (effectKind === 'extra_enemy_refresh') {
            if (asString(row.trigger) !== 'on_enemy_refresh') next.push(`Buff效果 ${id} #${effectIndex + 1}: extra_enemy_refresh 只能用于 on_enemy_refresh`);
            if (!ENEMY_REFRESH_POOL_OPTIONS.includes(asString(effect.enemyPool))) next.push(`Buff效果 ${id} #${effectIndex + 1}: enemyPool 非法 ${asString(effect.enemyPool) || '(空)'}`);
            const count = Number(effect.count);
            if (!Number.isInteger(count) || count < 1 || count > 20) next.push(`Buff效果 ${id} #${effectIndex + 1}: count 必须是 1~20 的整数`);
          }
          if (effectKind === 'trigger_synced_summon_turn') {
            for (const templateId of asStringList(effect.includeSummonTemplateIds)) {
              if (!enemyOptions.value.includes(templateId)) next.push(`Buff效果 ${id} #${effectIndex + 1}: 同步召唤模板未定义 ${templateId}`);
            }
          }
        }
      }
      if (table === 'collectibles') {
        if (row.categoryId && !catalog.collectible_categories.some((category) => rowId(category) === asString(row.categoryId))) {
          next.push(`收藏品 ${id}: categoryId 未定义 ${asString(row.categoryId)}`);
        }
        if (!asString(row.rarity)) next.push(`收藏品 ${id}: rarity 必填`);
        if (asString(row.rarity) && !rarityOptions.value.includes(asString(row.rarity))) next.push(`收藏品 ${id}: rarity 未定义 ${asString(row.rarity)}`);
      }
      if (table === 'achievements') {
        const knownAchievementIds = new Set<string>();
        for (const achievement of catalog.achievements) {
          const achievementId = rowId(achievement);
          if (!achievementId) continue;
          const stages = asArray(achievement.stages).filter(isRecord);
          if (stages.length > 0) {
            stages.forEach((_, stageIndex) => knownAchievementIds.add(`${achievementId}__stage_${stageIndex + 1}`));
          } else {
            knownAchievementIds.add(achievementId);
          }
        }
        if (row.categoryId && !catalog.achievement_categories.some((category) => rowId(category) === asString(row.categoryId))) {
          next.push(`成就 ${id}: categoryId 未定义 ${asString(row.categoryId)}`);
        }
        if (!asString(row.rarity)) next.push(`成就 ${id}: rarity 必填`);
        if (asString(row.rarity) && !rarityOptions.value.includes(asString(row.rarity))) next.push(`成就 ${id}: rarity 未定义 ${asString(row.rarity)}`);
        for (const prereqID of asStringList(row.prereqAchievementIds)) {
          if (prereqID === id) next.push(`成就 ${id}: 前置成就不能指向自己`);
          if (!knownAchievementIds.has(prereqID)) next.push(`成就 ${id}: 前置成就未定义 ${prereqID}`);
        }
        const stages = asArray(row.stages).filter(isRecord);
        if (stages.length > 0) {
          const stageProgressMode = asString(row.stageProgressMode);
          if (stageProgressMode && !['sequential', 'cumulative'].includes(stageProgressMode)) next.push(`成就 ${id}: stageProgressMode 非法 ${stageProgressMode}`);
          stages.forEach((stage, stageIndex) => validateAchievementParts(next, `${id} 阶段${stageIndex + 1}`, stage));
        } else {
          validateAchievementParts(next, id, row);
        }
      }
    }
  }
  const activeTransmuteRules = catalog.alchemy_transmute_rules.filter((rule) => rule.disabled !== true);
  const activeDefaultTransmuteRules = activeTransmuteRules.filter((rule) => rule.default === true);
  if (activeDefaultTransmuteRules.length !== 1) next.push(`点金规则: 必须且只能有一个启用的默认规则，当前 ${activeDefaultTransmuteRules.length} 个`);
  const transmuteMatchByItemId = new Map<string, string>();
  for (const rule of activeTransmuteRules) {
    for (const itemId of asStringList(rule.itemIds)) {
      const previous = transmuteMatchByItemId.get(itemId);
      if (previous) next.push(`点金规则: ${itemId} 被 ${previous} 与 ${rowId(rule)} 重复匹配`);
      else transmuteMatchByItemId.set(itemId, rowId(rule));
    }
  }
  const specializationSkillEffects = catalog.skills.flatMap((row) => asArray(row.effects)).filter(isRecord);
  const specializationEquipmentEffects = catalog.equipment_templates.flatMap((row) => asArray(row.basicAttackEffects)).filter(isRecord);
  const specializationSourceEffects = [...specializationSkillEffects, ...specializationEquipmentEffects];
  for (const issue of validateSpecializationTrees(catalog.specialization_trees, {
    attributeIds: attributeOptions.value,
    itemIds: itemOptions.value,
    skillIds: catalog.skills.map((row) => rowId(row)).filter(Boolean),
    skillCategories: catalog.skills.map((row) => asString(row.category).trim()).filter(Boolean),
    weaponClasses: catalog.equipment_templates.flatMap((row) => asArray(row.weaponClass).map((entry) => asString(entry).trim())).filter(Boolean),
    buffIds: catalog.buff.map((row) => rowId(row)).filter(Boolean),
    buffKinds: catalog.buff.map((row) => asString(row.kind).trim()).filter(Boolean),
    skillEffectKinds: specializationSourceEffects.map((effect) => asString(effect.kind).trim()).filter(Boolean),
    effectIds: specializationSourceEffects.flatMap((effect) => [asString(effect.id).trim(), asString(effect.effectId).trim()]).filter(Boolean),
    effectKindsById: Object.fromEntries(specializationSourceEffects.flatMap((effect) => {
      const kind = asString(effect.kind).trim();
      return [asString(effect.id).trim(), asString(effect.effectId).trim()].filter(Boolean).map((id) => [id, kind]);
    })),
    equipmentEffectIds: specializationEquipmentEffects.flatMap((effect) => [asString(effect.id).trim(), asString(effect.effectId).trim()]).filter(Boolean)
  })) {
    next.push(issue.message);
  }
  next.push(...validateGlobalSkillEffectStableIds([
    ...catalog.skills,
    ...catalog.equipment_templates.map((row) => ({
      id: `equipment:${rowId(row)}`,
      effects: asArray(row.basicAttackEffects)
    }))
  ]));
  issues.value = next;
  if (next.length === 0 && gameDataDir.value) status.value = '校验通过';
}

function validateAchievementParts(next: string[], label: string, row: DataRow) {
  const condition = isRecord(row.condition) ? row.condition : {};
  const kind = asString(condition.kind);
  if (!ACHIEVEMENT_CONDITION_KINDS.includes(kind as typeof ACHIEVEMENT_CONDITION_KINDS[number])) next.push(`成就 ${label}: condition.kind 非法 ${kind}`);
  if (kind === 'kill_enemy' && !enemyOptions.value.includes(asString(condition.enemyId))) next.push(`成就 ${label}: enemyId 未定义 ${asString(condition.enemyId)}`);
  if (kind === 'catch_fish' && !fishingSpeciesOptions.value.includes(asString(condition.speciesId))) next.push(`成就 ${label}: speciesId 未定义 ${asString(condition.speciesId)}`);
  if (kind === 'complete_action' && !actionIdOptions.value.includes(asString(condition.actionId))) next.push(`成就 ${label}: actionId 未定义 ${asString(condition.actionId)}`);
  if (kind === 'complete_action' && !['any', 'player', 'kitty'].includes(asString(condition.actorType, 'any'))) next.push(`成就 ${label}: actorType 非法 ${asString(condition.actorType)}`);
  if ((kind === 'own_item' || kind === 'obtain_item') && !itemOptions.value.includes(asString(condition.itemId))) next.push(`成就 ${label}: itemId 未定义 ${asString(condition.itemId)}`);
  if (kind === 'complete_specific_commission' && !catalog.cat_village_commissions.some((commission) => rowId(commission) === asString(condition.commissionId))) next.push(`成就 ${label}: commissionId 未定义 ${asString(condition.commissionId)}`);
  if (kind === 'player_attr_reached' && !attributeOptions.value.includes(asString(condition.attrKey))) next.push(`成就 ${label}: attrKey 未定义 ${asString(condition.attrKey)}`);
  for (const reward of asArray(row.rewards).filter(isRecord)) {
    const rewardKind = asString(reward.kind);
    if (rewardKind === 'item' && !itemOptions.value.includes(asString(reward.itemId))) next.push(`成就 ${label}: 奖励物品未定义 ${asString(reward.itemId)}`);
    if (rewardKind === 'wardrobe') {
      const category = asString(reward.category);
      if (category === 'title_prefix' && !catalog.title_prefixes.some((title) => rowId(title) === asString(reward.itemId))) next.push(`成就 ${label}: 称号前缀奖励未定义 ${asString(reward.itemId)}`);
      if (category === 'title_suffix' && !catalog.title_suffixes.some((title) => rowId(title) === asString(reward.itemId))) next.push(`成就 ${label}: 称号后缀奖励未定义 ${asString(reward.itemId)}`);
      if (!['title_prefix', 'title_suffix'].includes(category)) next.push(`成就 ${label}: wardrobe.category 非法 ${category}`);
    }
    if (!['item', 'wardrobe'].includes(rewardKind)) next.push(`成就 ${label}: reward.kind 非法 ${rewardKind}`);
  }
}

function selectTable(table: ActiveView) {
  activeView.value = table;
  router.push(routePathForView(table));
}

function updateSpecializationTrees(rows: DataRow[]) {
  catalog.specialization_trees = clone(rows);
  clearRowSearchTextCache();
  validateAll();
}

function selectRow(index: number) {
  selectedTable.value = activeTable.value;
  selectedIndex.value = index;
  const row = catalog[activeTable.value]?.[index];
  const id = row ? rowId(row) : '';
  router.push(routePathForView(activeTable.value, id || undefined));
}

function jumpToRow(table: EditableTable, target: string | ((row: DataRow) => boolean)) {
  const rows = catalog[table] ?? [];
  const index = rows.findIndex((row) => typeof target === 'function' ? target(row) : rowId(row) === target);
  if (index < 0) {
    status.value = `未找到 ${TABLE_LABELS[table]}: ${typeof target === 'string' ? target : '关联数据'}`;
    return;
  }
  activeView.value = table;
  selectedTable.value = table;
  selectedIndex.value = index;
  filterInputText.value = '';
  debouncedFilterText.value = '';
  const id = rowId(rows[index]);
  router.push(routePathForView(table, id || undefined));
  status.value = `已跳转到 ${TABLE_LABELS[table]}: ${id}`;
}

function addRow() {
  catalog[activeTable.value].push(ensureDisableFlag(makeDefaultRow(activeTable.value)));
  selectedTable.value = activeTable.value;
  selectedIndex.value = catalog[activeTable.value].length - 1;
  clearRowSearchTextCache();
  validateAll();
}

function duplicateRow() {
  const row = selectedRow.value;
  if (!row) return;
  const copy = ensureDisableFlag(clone(row));
  copy.id = `${rowId(row, editorTable.value)}_copy`;
  catalog[editorTable.value].splice(selectedIndex.value + 1, 0, copy);
  selectedIndex.value += 1;
  clearRowSearchTextCache();
  validateAll();
}

function toggleSelectedRowDisabled() {
  if (!selectedRow.value) return;
  selectedRow.value.disabled = !rowDisabled(selectedRow.value);
  validateAll();
}

function removeRow() {
  if (!selectedRow.value) return;
  if (!confirm(`删除 ${rowId(selectedRow.value, '当前行')}？`)) return;
  const rows = catalog[editorTable.value];
  rows.splice(selectedIndex.value, 1);
  selectedIndex.value = rows.length === 0 ? 0 : Math.max(0, selectedIndex.value - 1);
  clearRowSearchTextCache();
  validateAll();
}

function updateField(key: string, value: unknown) {
  if (!selectedRow.value) return;
  const previousId = key === 'id' ? rowId(selectedRow.value) : '';
  selectedRow.value[key] = value;
  clearRowSearchTextCache();
  if (key === 'id') {
    applyIdDerivedTemplates(editorTable.value, selectedRow.value, previousId, rowId(selectedRow.value));
  }
  validateAll();
}

function applyIdDerivedTemplates(table: EditableTable, row: DataRow, previousId: string, currentId: string) {
  const id = currentId.trim();
  if (!id) return;
  const setTemplate = (key: string, nextTemplate: string, previousTemplate: string) => {
    const current = asString(row[key]).trim();
    if (!current || current === previousTemplate) row[key] = nextTemplate;
  };
  if (table === 'actions') {
    setTemplate('nameKey', `action.${id}.name`, `action.${previousId}.name`);
    setTemplate('descKey', `action.${id}.desc`, `action.${previousId}.desc`);
    setTemplate('iconId', `action.${id}`, `action.${previousId}`);
    return;
  }
  if (table === 'items') {
    setTemplate('nameKey', `item.${id}.name`, `item.${previousId}.name`);
    setTemplate('descKey', `item.${id}.desc`, `item.${previousId}.desc`);
    setTemplate('effectDescKey', `item.${id}.effect`, `item.${previousId}.effect`);
    setTemplate('iconId', `item.${id}`, `item.${previousId}`);
    return;
  }
  if (table === 'buff') {
    setTemplate('nameKey', `buff.${id}.name`, `buff.${previousId}.name`);
    setTemplate('descKey', `buff.${id}.desc`, `buff.${previousId}.desc`);
    return;
  }
  if (table === 'affix_definitions') {
    setTemplate('nameKey', `affix.${id}.name`, `affix.${previousId}.name`);
    setTemplate('descKey', `affix.${id}.desc`, `affix.${previousId}.desc`);
  }
}

function normalizeNumberMap(value: unknown): Record<string, number> {
  if (!isRecord(value)) return {};
  const out: Record<string, number> = {};
  for (const [key, raw] of Object.entries(value)) {
    if (!key) continue;
    const numberValue = isRecord(raw) ? asNumber(raw.count, asNumber(raw.value, NaN)) : asNumber(raw, NaN);
    if (!Number.isFinite(numberValue) || numberValue <= 0) continue;
    out[key] = numberValue;
  }
  return out;
}

function normalizeSignedNumberMap(value: unknown): Record<string, number> {
  if (!isRecord(value)) return {};
  const out: Record<string, number> = {};
  for (const [key, raw] of Object.entries(value)) {
    const normalizedKey = key.trim();
    if (!normalizedKey) continue;
    const numberValue = asNumber(raw, 0);
    if (!Number.isFinite(numberValue) || numberValue === 0) continue;
    out[normalizedKey] = numberValue;
  }
  return out;
}

function addMaps(base: Record<string, number>, delta: Record<string, number>): Record<string, number> {
  const out: Record<string, number> = { ...base };
  for (const [key, value] of Object.entries(delta)) {
    out[key] = (out[key] ?? 0) + value;
    if (out[key] === 0) delete out[key];
  }
  return out;
}

function enhanceCostAtLevel(curveRaw: unknown, level: number): Record<string, number> {
  if (!isRecord(curveRaw) || level <= 0) return {};
  const overrideLevelMap = isRecord(curveRaw.overrides) ? curveRaw.overrides[String(level)] : undefined;
  if (isRecord(overrideLevelMap)) return normalizeNumberMap(overrideLevelMap);
  const base = normalizeSignedNumberMap(curveRaw.base);
  const step = normalizeSignedNumberMap(curveRaw.step);
  const keys = new Set([...Object.keys(base), ...Object.keys(step)]);
  const out: Record<string, number> = {};
  for (const key of keys) {
    const value = (base[key] ?? 0) + (step[key] ?? 0) * (level - 1);
    if (!Number.isFinite(value) || value <= 0) continue;
    out[key] = value;
  }
  return out;
}

function enhanceAttrIncrementAtLevel(curveRaw: unknown, level: number): Record<string, number> {
  if (!isRecord(curveRaw) || level <= 0) return {};
  const overrideLevelMap = isRecord(curveRaw.overrides) ? curveRaw.overrides[String(level)] : undefined;
  if (isRecord(overrideLevelMap)) return normalizeSignedNumberMap(overrideLevelMap);
  const base = normalizeSignedNumberMap(curveRaw.base);
  const step = normalizeSignedNumberMap(curveRaw.step);
  const keys = new Set([...Object.keys(base), ...Object.keys(step)]);
  const out: Record<string, number> = {};
  for (const key of keys) {
    const value = level <= 1 ? (base[key] ?? 0) : (step[key] ?? 0);
    if (!Number.isFinite(value) || value === 0) continue;
    out[key] = value;
  }
  return out;
}

function summarizeEquipmentEnhance(row: DataRow | null): EquipmentEnhanceSummary | null {
  if (!row) return null;
  const maxLevel = Math.max(0, Math.floor(asNumber(row.enhanceMaxLevel, 0)));
  const baseAttrs = normalizeSignedNumberMap(row.baseAttrs);
  let cumulativeEnhanceAttrs: Record<string, number> = {};
  let totalCosts: Record<string, number> = {};
  for (let level = 1; level <= maxLevel; level += 1) {
    cumulativeEnhanceAttrs = addMaps(cumulativeEnhanceAttrs, enhanceAttrIncrementAtLevel(row.enhanceAttrCurve, level));
    totalCosts = addMaps(totalCosts, enhanceCostAtLevel(row.enhanceCostCurve, level));
  }
  const finalAttrs = addMaps(baseAttrs, cumulativeEnhanceAttrs);
  const totalCostCount = Object.values(totalCosts).reduce((sum, value) => sum + Math.max(0, asNumber(value, 0)), 0);
  return {
    maxLevel,
    cumulativeEnhanceAttrs,
    finalAttrs,
    totalCosts,
    totalCostCount
  };
}

const itemEconomyBaseValueMap = computed<Record<string, number>>(() => {
  const map: Record<string, number> = {};
  for (const row of catalog.items) {
    const id = rowId(row);
    if (!id) continue;
    map[id] = Math.max(0, asNumber(row.economyBaseValue, 0));
  }
  return map;
});

function expectedRewardCount(entry: unknown): number {
  if (typeof entry === 'number') return Math.max(0, entry);
  if (!isRecord(entry)) return 0;
  const percent = entry.percent == null ? 1 : Math.max(0, asNumber(entry.percent, 1));
  const range = isRecord(entry.range) ? entry.range : null;
  const amountFromRange = range ? (Math.max(0, asNumber(range.min, 0)) + Math.max(0, asNumber(range.max, asNumber(range.min, 0)))) / 2 : NaN;
  const amount = Number.isFinite(amountFromRange)
    ? amountFromRange
    : Math.max(0, asNumber(entry.count, asNumber(entry.value, 0)));
  return amount * percent;
}

function actionEconomySummary(row: DataRow | null): ActionEconomySummary | null {
  if (!row) return null;
  const costs = normalizeNumberMap(row.costs);
  const totalCostGold = Object.entries(costs).reduce((total, [itemID, qty]) => {
    const unit = Math.max(0, asNumber(itemEconomyBaseValueMap.value[itemID], 0));
    return total + qty * unit;
  }, 0);
  const goldPerSecond = Math.max(0, asNumber(scoring.constants.goldPerSecond, 1));
  const durationS = Math.max(0, asNumber(row.durationS, 0));
  const economyIndustryTier = Math.max(0, asNumber(row.economyIndustryTier, 1));
  const economyDifficultyMultiplier = Math.max(0, asNumber(row.economyDifficultyMultiplier, 1));
  const baseLaborGold = durationS * goldPerSecond * economyIndustryTier * economyDifficultyMultiplier;
  const mainOutput = asString(row.mainOutput);
  const itemReward = isRecord(row.itemReward) ? row.itemReward : {};
  const mainOutputExpectedCount = mainOutput ? expectedRewardCount(itemReward[mainOutput]) : 0;
  const actionTotalGold = totalCostGold + baseLaborGold;
  const mainOutputUnitGold = mainOutputExpectedCount > 0 ? actionTotalGold / mainOutputExpectedCount : 0;
  return {
    mainOutput,
    mainOutputExpectedCount,
    baseLaborGold,
    totalCostGold,
    actionTotalGold,
    mainOutputUnitGold
  };
}

function pickPrimaryOutputAction(itemID: string): DataRow | null {
  const candidates = catalog.actions
    .filter((row) => asString(row.mainOutput) === itemID && !rowDisabled(row))
    .sort((a, b) => asNumber(a.durationS, 0) - asNumber(b.durationS, 0) || rowId(a).localeCompare(rowId(b)));
  return candidates[0] ?? null;
}

function fillEnhanceCostCurveFromMainOutputAction() {
  if (editorTable.value !== 'equipment_templates' || !selectedRow.value) return;
  const itemID = rowId(selectedRow.value);
  if (!itemID) return;
  const action = pickPrimaryOutputAction(itemID);
  if (!action) {
    status.value = `未找到主产出为 ${itemID} 的动作`;
    return;
  }
  const costs = normalizeNumberMap(action.costs);
  if (Object.keys(costs).length === 0) {
    status.value = `动作 ${rowId(action)} 没有可用的消耗物资`;
    return;
  }
  const current = isRecord(selectedRow.value.enhanceCostCurve) ? selectedRow.value.enhanceCostCurve : {};
  selectedRow.value.enhanceCostCurve = {
    ...current,
    base: costs,
    step: costs
  };
  status.value = `已按动作 ${rowId(action)} 填充强化消耗基础/增量`;
  validateAll();
}

function fillEnhanceAttrCurveFromBaseAttrs() {
  if (!selectedRow.value) return;
  // Collectible: levelAttrCurve ← playerMods
  if (editorTable.value === 'collectibles') {
    const base = normalizeNumberMap(selectedRow.value.playerMods);
    if (Object.keys(base).length === 0) {
      status.value = '当前收藏品没有可用的玩家属性加成';
      return;
    }
    const current = isRecord(selectedRow.value.levelAttrCurve) ? selectedRow.value.levelAttrCurve : {};
    selectedRow.value.levelAttrCurve = { ...current, base, step: base };
    status.value = '已按玩家属性加成填充等级属性基础/增量';
    validateAll();
    return;
  }
  // Equipment: enhanceAttrCurve ← baseAttrs
  if (editorTable.value !== 'equipment_templates') return;
  const baseAttrs = normalizeNumberMap(selectedRow.value.baseAttrs);
  if (Object.keys(baseAttrs).length === 0) {
    status.value = '当前装备没有可用的基础属性';
    return;
  }
  const current = isRecord(selectedRow.value.enhanceAttrCurve) ? selectedRow.value.enhanceAttrCurve : {};
  selectedRow.value.enhanceAttrCurve = {
    ...current,
    base: baseAttrs,
    step: baseAttrs
  };
  status.value = '已按基础属性填充强化属性基础/增量';
  validateAll();
}

function updateAdvancedKey(key: string, value: unknown) {
  updateField(key, value);
}

function addAdvancedField() {
  const key = prompt('新增高级字段名');
  if (!key || !selectedRow.value || Object.prototype.hasOwnProperty.call(selectedRow.value, key)) return;
  selectedRow.value[key] = '';
}

function removeAdvancedField(key: string) {
  if (!selectedRow.value) return;
  delete selectedRow.value[key];
}

function updateEnemyAttr(attr: string, value: unknown) {
  if (!selectedRow.value) return;
  const num = asNumber(value, 0);
  if (num === 0) delete selectedRow.value[attr];
  else selectedRow.value[attr] = num;
  validateAll();
}

function fieldValue(row: DataRow | null, field: FieldDef) {
  if (!row) return undefined;
  if (field.key === '__enemyAttrs') return row;
  return row[field.key];
}

function scoringAttribute(key: string): AttributeValue | undefined {
  return scoring.attributes.find((attr) => attr.key === key);
}

function formatScoreNumber(value: number): string {
  if (!Number.isFinite(value)) return '0';
  if (Math.abs(value) >= 100) return String(Math.round(value));
  if (Math.abs(value) >= 10) return String(Math.round(value * 10) / 10);
  return String(Math.round(value * 100) / 100);
}

function formatPercent(value: number): string {
  if (!Number.isFinite(value)) return '0%';
  return `${formatScoreNumber(value * 100)}%`;
}

function formatDecimal(value: number, digits = 2): string {
  if (!Number.isFinite(value)) return '0';
  return value.toFixed(digits).replace(/\.0+$/, '').replace(/(\.\d*?)0+$/, '$1');
}

function balanceSeverityClass(severity: string): string {
  return `balance-${severity}`;
}

function balanceBaselineRows() {
  return balanceModel.levelBands;
}

function jumpToBalanceIssue(issue: BalanceDiagnostics) {
  jumpToRow(issue.table, issue.id);
}

function selectedBaselineText(): string {
  if (!selectedRow.value) return '';
  const level = Math.max(1, asNumber(selectedRow.value.level, 1));
  const baseline = baselineAtLevel(balanceModel, level);
  return `Lv.${level} 基准：DPR ${formatScoreNumber(baseline.playerDpr)} / HP ${formatScoreNumber(baseline.playerHp)} / MP ${formatScoreNumber(baseline.playerMp)} / 速度 ${formatScoreNumber(baseline.playerSpeed)}`;
}

function toggleExpandRole(key: string) {
  if (expandedRoles.value.has(key)) expandedRoles.value.delete(key);
  else expandedRoles.value.add(key);
}

function toggleExpandSpecies(key: string) {
  if (expandedSpecies.value.has(key)) expandedSpecies.value.delete(key);
  else expandedSpecies.value.add(key);
}

function templateWeightBar(weights: Record<string, number>): { key: string; width: number; label: string }[] {
  const key5 = ['hp', 'physics_damage', 'magical_damage', 'physics_defence', 'magical_defence', 'speed', 'mp'];
  const total = Object.values(weights).reduce((a, b) => a + Math.max(0, b), 0) || 1;
  return key5.map((k) => ({
    key: k,
    width: Math.round(Math.max(0, weights[k] ?? 0) / total * 100),
    label: k.replace('physics_', '物/').replace('magical_', '魔/').replace('_damage', 'DPS').replace('_defence', 'DEF').replace('hp', 'HP').replace('speed', '速').replace('mp', 'MP')
  }));
}

function radarPoints(weights: Record<string, number>, speciesWeights?: Record<string, number>): string {
  const dims = ['hp', 'physics_damage', 'magical_damage', 'physics_defence', 'magical_defence', 'speed', 'mp'];
  const merged: Record<string, number> = {};
  for (const k of dims) {
    merged[k] = Math.max(0, (weights[k] ?? 0) + (speciesWeights?.[k] ?? 0));
  }
  const max = Math.max(...Object.values(merged), 0.01);
  const cx = 60; const cy = 60; const r = 50;
  return dims.map((k, i) => {
    const angle = (i / dims.length) * Math.PI * 2 - Math.PI / 2;
    const scale = merged[k] / max;
    return `${cx + r * scale * Math.cos(angle)},${cy + r * scale * Math.sin(angle)}`;
  }).join(' ');
}

function radarRolePoints(weights: Record<string, number>): string {
  return radarPoints(weights);
}

function radarMergedPoints(roleWeights: Record<string, number>, speciesWeights: Record<string, number>): string {
  return radarPoints(roleWeights, speciesWeights);
}

function addWeightKey(weights: Record<string, number>) {
  const key = prompt('属性键名（如 crit_chance）：');
  if (key && key.trim()) weights[key.trim()] = 0;
}

function removeWeightKey(weights: Record<string, number>, key: string) {
  delete weights[key];
}

function attrLabel(key: string): string {
  return scoring.attributes.find((a) => a.key === key)?.label ?? key;
}

function addAttrToMap(map: Record<string, number>, key: string, defaultVal = 0) {
  if (key && !Object.prototype.hasOwnProperty.call(map, key)) map[key] = defaultVal;
}

function removeAttrFromMap(map: Record<string, number>, key: string) {
  delete map[key];
}

function addTemplateEntry(kind: 'tiers' | 'roles' | 'species') {
  const key = prompt('新条目 key（英文，如 my_role）：');
  if (!key || !key.trim()) return;
  const k = key.trim();
  const arr = scoring.enemyAttributeTemplates[kind] as Array<{ key: string }>;
  if (arr.some((e) => e.key === k)) return;
  const label = prompt('显示名称：') ?? k;
  if (kind === 'tiers') {
    (scoring.enemyAttributeTemplates.tiers as EnemyTierTemplateRule[]).push({
      key: k, label, enemyType: k, budgetMultiplier: 1, scoreMult: 1, rewardMult: 1,
      pressurePct: 0.15, targetRounds: { min: 4, max: 8 }
    });
  } else {
    (scoring.enemyAttributeTemplates[kind] as EnemyTemplateRule[]).push({
      key: k, label, weights: {}, preferredSkillTypes: []
    });
  }
}

function autoFillTierParams(tier: EnemyTierTemplateRule) {
  const tierType = tier.key as 'normal' | 'elite' | 'boss' | 'world_boss';
  const refLevel = 100; // 参考等级，用于计算建议值
  const estimate = estimateTierParams(refLevel, tierType);
  tier.targetRounds = estimate.targetRounds;
  tier.pressurePct = estimate.pressurePct;
  tier.budgetMultiplier = estimate.budgetMultiplier;
  tier.hpMultiplier = estimate.hpMultiplier;
}

function removeTemplateEntry(kind: 'tiers' | 'roles' | 'species', key: string) {
  const arr = scoring.enemyAttributeTemplates[kind] as Array<{ key: string }>;
  const idx = arr.findIndex((e) => e.key === key);
  if (idx >= 0) arr.splice(idx, 1);
}

function formatGoldDisplay(value: number): string {
  if (!Number.isFinite(value)) return '0.00';
  return (Math.round(value * 100) / 100).toFixed(2);
}

function formatSigned(value: number): string {
  if (!Number.isFinite(value)) return '0';
  const rounded = Math.round(value * 100) / 100;
  const text = Number.isInteger(rounded) ? String(rounded) : rounded.toFixed(2).replace(/\.?0+$/, '');
  return `${rounded > 0 ? '+' : ''}${text}`;
}

function formatAttrMapSummary(value: Record<string, number>): string {
  const entries = Object.entries(value).filter(([, raw]) => Number.isFinite(raw) && raw !== 0);
  if (!entries.length) return '-';
  return entries
    .sort(([a], [b]) => a.localeCompare(b))
    .map(([attr, raw]) => `${localizeKey(`attr.${attr}.name`, attr)} ${formatSigned(raw)}`)
    .join(' / ');
}

function formatItemMapSummary(value: Record<string, number>): string {
  const entries = Object.entries(value).filter(([, raw]) => Number.isFinite(raw) && raw > 0);
  if (!entries.length) return '-';
  return entries
    .sort(([a], [b]) => a.localeCompare(b))
    .map(([itemID, raw]) => `${localizeKey(`item.${itemID}.name`, itemID)} x${formatGoldDisplay(raw)}`)
    .join(' / ');
}

type AlchemyReviewEffect = {
  attr: string;
  label: string;
  min: number;
  max: number;
  totalText: string;
  perUnitText: string;
};

type AlchemyReviewRow = {
  itemId: string;
  itemName: string;
  rarity: string;
  kind: string;
  family: string;
  minAlchemyLevel: number;
  unitBasis: number;
  maxQty: number;
  fullMaxQty: number;
  failBaseChance: number;
  failQuantityCap: number;
  positiveEffects: AlchemyReviewEffect[];
  negativeEffects: AlchemyReviewEffect[];
};

type AlchemyComboReviewTemplate = AlchemyComboTemplateRow & {
  rangeText: string;
};

type AlchemyComboReviewRow = {
  id: string;
  name: string;
  minAlchemyLevel: number;
  outputRarity: string;
  inputsText: string;
  comboPowerText: string;
  templates: AlchemyComboReviewTemplate[];
};

type AlchemyInheritReviewRow = {
  id: string;
  itemId: string;
  itemName: string;
  consumePerCraft: number;
  minAlchemyLevel: number;
  inheritSlots: number;
  suppressSlots: number;
  ignoreKeepChance: number;
  positiveIgnoreKeepChanceBase: number;
  positiveIgnoreKeepChanceBonus: number;
  negativeIgnoreKeepChanceBase: number;
  negativeIgnoreSuppressChanceBonus: number;
  inheritCostText: string;
  suppressCostText: string;
  hybridText: string;
  inbreedText: string;
  mutationChance: number;
  scalingText: string;
};

const alchemyReviewPrecisionPercent = ref(0);
const alchemyReviewStabilityPercent = ref(0);
const alchemyReviewFillPercent = ref(100);
const alchemyReviewComboMultiplier = ref(1);
const alchemyInheritReviewLevel = ref(40);
const alchemyReviewPrecision = computed(() => asNumber(alchemyReviewPrecisionPercent.value, 0) / 100);
const alchemyReviewStability = computed(() => asNumber(alchemyReviewStabilityPercent.value, 0) / 100);
const alchemyReviewFillRatio = computed(() => Math.max(0, Math.min(100, asNumber(alchemyReviewFillPercent.value, 100))) / 100);
const alchemyReviewComboScale = computed(() => Math.max(1, Math.round(asNumber(alchemyReviewComboMultiplier.value, 1))));
const alchemyAttributeUnitMap = computed<Record<string, string>>(() => {
  const out: Record<string, string> = {};
  for (const row of catalog.attributes) {
    const id = rowId(row);
    if (!id) continue;
    out[id] = asString(row.unit, 'number');
  }
  return out;
});
const alchemyItemMap = computed(() => new Map(catalog.items.map((row) => [rowId(row), row] as const)));
const alchemyReviewRows = computed<AlchemyReviewRow[]>(() => {
  const rows: AlchemyReviewRow[] = [];
  for (const traitSet of catalog.item_traits) {
    if (!isRecord(traitSet)) continue;
    const itemId = asString(traitSet.itemId);
    if (!itemId) continue;
    const trait = asArray(traitSet.traits).find(isRecord);
    if (!trait) continue;
    const item = alchemyItemMap.value.get(itemId);
    const unitBasis = Math.max(1, Math.round(asNumber(trait.unitBasis, 1)));
    const maxBasisMultiplier = Math.max(1, Math.round(asNumber(trait.maxBasisMultiplier, 1)));
    const baseMaxQty = unitBasis * maxBasisMultiplier;
    const maxQty = alchemyReviewMaxQuantity(baseMaxQty, unitBasis);
    const reviewQty = alchemyReviewQuantity(maxQty);
    const effects = asArray(trait.effects).filter(isRecord).map((effect) => alchemyReviewEffect(effect, reviewQty)).filter((effect): effect is AlchemyReviewEffect => Boolean(effect));
    rows.push({
      itemId,
      itemName: item ? displayName(item) : localizeKey(`item.${itemId}.name`, itemId),
      rarity: asString(item?.rarity, 'common'),
      kind: asString(trait.kind, '-'),
      family: asString(trait.family, '-'),
      minAlchemyLevel: Math.max(1, Math.round(asNumber(trait.minAlchemyLevel, 1))),
      unitBasis,
      maxQty: reviewQty,
      fullMaxQty: maxQty,
      failBaseChance: asNumber(trait.failBaseChance, 0),
      failQuantityCap: asNumber(trait.failChanceQuantityCap, 0),
      positiveEffects: effects.filter((effect) => effect.max > 0),
      negativeEffects: effects.filter((effect) => effect.max <= 0)
    });
  }
  return rows.sort((a, b) =>
    rarityRank(b.rarity) - rarityRank(a.rarity) ||
    b.minAlchemyLevel - a.minAlchemyLevel ||
    a.itemName.localeCompare(b.itemName, 'zh-Hans-CN') ||
    a.itemId.localeCompare(b.itemId)
  );
});

const alchemyComboReviewRows = computed<AlchemyComboReviewRow[]>(() => {
  const scale = alchemyReviewComboScale.value;
  return catalog.alchemy_combo_recipes.filter(isRecord).map((combo) => {
    const id = rowId(combo);
    const output = isRecord(combo.outputAffix) ? combo.outputAffix : {};
    const params = isRecord(output.params) ? output.params : {};
    const range = isRecord(output.valueRange) ? output.valueRange : {};
    const baseMin = asNumber(range.min, 0) * scale;
    const baseMax = asNumber(range.max, baseMin) * scale;
    const [comboMinRaw, comboMaxRaw] = applyAlchemyReviewDerived(baseMin, baseMax);
    const comboMin = Math.min(comboMinRaw, comboMaxRaw);
    const comboMax = Math.max(comboMinRaw, comboMaxRaw);
    const templates = collectAlchemyComboTemplates(params).map((row) => {
      const minValue = row.source === 'comboMultiplier' ? row.base + scale * row.coeff : row.base + comboMin * row.coeff;
      const maxValue = row.source === 'comboMultiplier' ? row.base + scale * row.coeff : row.base + comboMax * row.coeff;
      return {
        ...row,
        rangeText: formatAlchemyComboTemplateRange(row, minValue, maxValue)
      };
    });
    return {
      id,
      name: localizeKey(asString(combo.nameKey, `alchemy.recipe.${id}.name`), id),
      minAlchemyLevel: Math.max(1, Math.round(asNumber(combo.minAlchemyLevel, 1))),
      outputRarity: asString(combo.outputRarity, '-'),
      inputsText: Object.entries(isRecord(combo.inputs) ? combo.inputs : {}).map(([itemId, qty]) => `${localizeKey(`item.${itemId}.name`, itemId)} x${qty}`).join(' / '),
      comboPowerText: formatAlchemyReviewRange('comboPower', comboMin, comboMax),
      templates
    };
  }).sort((a, b) => b.minAlchemyLevel - a.minAlchemyLevel || a.id.localeCompare(b.id));
});

const alchemyInheritReviewRows = computed<AlchemyInheritReviewRow[]>(() => {
  return catalog.alchemy_inherit_mediums.filter(isRecord).map((medium) => {
    const id = rowId(medium);
    const itemId = asString(medium.itemId);
    const item = alchemyItemMap.value.get(itemId);
    const mods = isRecord(medium.modifiers) ? medium.modifiers : {};
    const minLevel = Math.max(1, Math.round(asNumber(medium.minAlchemyLevel, 1)));
    const inheritCost = isRecord(mods.inheritCost) ? mods.inheritCost : {};
    const suppressCost = isRecord(mods.suppressCost) ? mods.suppressCost : {};
    const goodRange = isRecord(mods.inbreedGoodMultiplierRange) ? mods.inbreedGoodMultiplierRange : {};
    const badRange = isRecord(mods.inbreedBadMultiplierRange) ? mods.inbreedBadMultiplierRange : {};
    const precision = isRecord(mods.precisionScaling) ? mods.precisionScaling : {};
    const lucky = isRecord(mods.luckyScaling) ? mods.luckyScaling : {};
    const stability = isRecord(mods.stabilityScaling) ? mods.stabilityScaling : {};
    return {
      id,
      itemId,
      itemName: item ? displayName(item) : localizeKey(`item.${itemId}.name`, itemId),
      consumePerCraft: Math.max(1, Math.round(asNumber(medium.consumePerCraft, 1))),
      minAlchemyLevel: minLevel,
      inheritSlots: Math.max(0, Math.round(asNumber(mods.inheritSlots, 0))),
      suppressSlots: Math.max(0, Math.round(asNumber(mods.suppressSlots, 0))),
      ignoreKeepChance: Math.max(0, Math.min(1, asNumber(mods.ignoreKeepChance, 0))),
      positiveIgnoreKeepChanceBase: Math.max(0, Math.min(1, asNumber(mods.positiveIgnoreKeepChanceBase, 0.5))),
      positiveIgnoreKeepChanceBonus: Math.max(0, Math.min(1, asNumber(mods.positiveIgnoreKeepChanceBonus, 0))),
      negativeIgnoreKeepChanceBase: Math.max(0, Math.min(1, asNumber(mods.negativeIgnoreKeepChanceBase, 0.9))),
      negativeIgnoreSuppressChanceBonus: Math.max(0, Math.min(1, asNumber(mods.negativeIgnoreSuppressChanceBonus, 0))),
      inheritCostText: formatInheritCostText(inheritCost),
      suppressCostText: formatInheritCostText(suppressCost),
      hybridText: `融合除数 ${formatDecimal(asNumber(mods.hybridMergeDivisor, 1), 2)}；多来源按 1+(N-1)*(除数-1) 递减`,
      inbreedText: `畸变 ${formatPercent(asNumber(mods.inbreedMutationChance, 0))}；正向 ${formatPercent(asNumber(mods.inbreedGoodChance, 0))}；好 ${formatDecimal(asNumber(goodRange.min, 1), 2)}~${formatDecimal(asNumber(goodRange.max, 1), 2)}；坏 ${formatDecimal(asNumber(badRange.min, 1), 2)}~${formatDecimal(asNumber(badRange.max, 1), 2)}`,
      mutationChance: Math.max(0, Math.min(1, asNumber(mods.mutationChance, 0))),
      scalingText: `精度提高基础保留次数并降低除数；稳定每 100% 增加 1 次基础抑制，并按 100/(100+稳定) 调整正/负不干预概率；幸运 保留+${formatDecimal(asNumber(lucky.ignoreKeepChancePerPoint, 0), 3)}/点 正向畸变+${formatDecimal(asNumber(lucky.inbreedGoodChancePerPoint, 0), 3)}/点`
    };
  }).sort((a, b) => b.minAlchemyLevel - a.minAlchemyLevel || a.id.localeCompare(b.id));
});

function formatInheritCostText(cost: Record<string, unknown>): string {
  const gold = Math.max(0, Math.round(asNumber(cost.gold, 0)));
  const items = isRecord(cost.items) ? Object.entries(cost.items).filter(([, qty]) => asNumber(qty, 0) > 0).map(([itemId, qty]) => `${itemId} x${Math.round(asNumber(qty, 0))}`) : [];
  return [`金币 ${gold}`, ...items].join(' / ');
}

function alchemyReviewQuantity(maxQty: number): number {
  const cappedMax = Math.max(1, Math.round(maxQty));
  return Math.max(1, Math.min(cappedMax, Math.round(1 + (cappedMax - 1) * alchemyReviewFillRatio.value)));
}

function alchemyReviewMaxQuantity(baseMaxQty: number, unitBasis: number): number {
  const basis = Math.max(1, Math.round(unitBasis));
  const baseMax = Math.max(basis, Math.round(baseMaxQty));
  return Math.max(basis, Math.floor((baseMax * (1 + Math.max(0, alchemyReviewStability.value))) / basis) * basis);
}

function alchemyReviewEffect(effect: DataRow, qty: number): AlchemyReviewEffect | null {
  const attr = asString(effect.attr);
  const range = isRecord(effect.valuePerUnit) ? effect.valuePerUnit : {};
  if (!attr || !isRecord(range)) return null;
  const rawMin = asNumber(range.min, 0) * qty;
  const rawMax = asNumber(range.max, rawMin) * qty;
  const [scaledMin, scaledMax] = applyAlchemyReviewDerived(rawMin, rawMax);
  const min = Math.min(scaledMin, scaledMax);
  const max = Math.max(scaledMin, scaledMax);
  return {
    attr,
    label: localizeKey(`attr.${attr}.name`, attr),
    min,
    max,
    totalText: formatAlchemyReviewRange(attr, min, max),
    perUnitText: formatAlchemyReviewRange(attr, asNumber(range.min, 0), asNumber(range.max, 0))
  };
}

function applyAlchemyReviewDerived(minRaw: number, maxRaw: number): [number, number] {
  const lo = Math.min(minRaw, maxRaw);
  const hi = Math.max(minRaw, maxRaw);
  if (lo < 0 || hi <= 0) return [lo, hi];
  const precisionRatio = alchemyReviewSaturatingRatio(alchemyReviewPrecision.value);
  const stabilityRatio = alchemyReviewSaturatingRatio(alchemyReviewStability.value);
  const min = stabilityRatio >= 0 ? Math.max(lo, hi * stabilityRatio) : hi * stabilityRatio;
  const max = hi * Math.max(0.000001, 1 + precisionRatio + stabilityRatio * 0.25);
  return [Math.min(min, max), max];
}

function alchemyReviewSaturatingRatio(value: number): number {
  const numeric = asNumber(value, 0);
  if (!Number.isFinite(numeric)) return 0;
  return numeric / (1 + Math.abs(numeric));
}

function formatAlchemyReviewRange(attr: string, min: number, max: number): string {
  return `${formatAlchemyReviewValue(attr, min)} ~ ${formatAlchemyReviewValue(attr, max)}`;
}

function formatAlchemyReviewValue(attr: string, value: number): string {
  if (!Number.isFinite(value)) return '0';
  if (alchemyAttributeUnitMap.value[attr] === 'percent') return `${(value * 100).toFixed(2)}%`;
  const abs = Math.abs(value);
  if (abs >= 10) return (Math.round(value * 100) / 100).toFixed(2).replace(/\.?0+$/, '');
  if (abs >= 1) return (Math.round(value * 1000) / 1000).toFixed(3).replace(/\.?0+$/, '');
  return (Math.round(value * 10000) / 10000).toFixed(4).replace(/\.?0+$/, '');
}

function formatAlchemyComboTemplateRange(row: AlchemyComboTemplateRow, min: number, max: number): string {
  const lo = Math.min(min, max);
  const hi = Math.max(min, max);
  return `${formatAlchemyComboTemplateValue(row, lo)} ~ ${formatAlchemyComboTemplateValue(row, hi)}`;
}

function formatAlchemyComboTemplateValue(row: AlchemyComboTemplateRow, value: number): string {
  if (!Number.isFinite(value)) return '0';
  if (row.valueKind === 'chance') return `${(value * 100).toFixed(2)}%`;
  if (row.valueKind === 'attr' && row.attr) return formatAlchemyReviewValue(row.attr, value);
  return formatAlchemyReviewValue(row.pathText, value);
}

function rarityRank(rarity: string): number {
  const idx = RARITY_ORDER.indexOf(rarity);
  return idx >= 0 ? idx : 0;
}

const selectedActionEconomy = computed(() => (editorTable.value === 'actions' ? actionEconomySummary(selectedRow.value) : null));
const dropPricingReport = computed(() => buildDropPricingReport(catalog, scoring, balanceModel));
const dropPricingItemMap = computed(() => new Map(dropPricingReport.value.itemPrices.map((row) => [row.itemId, row] as const)));
const dropPricingPreviewRows = computed(() => dropPricingReport.value.itemPrices.slice(0, 80));
const dropPricingItemWorldGroups = computed(() => {
  const map = new Map<string, typeof dropPricingReport.value.itemPrices>();
  for (const row of dropPricingReport.value.itemPrices) {
    const worldId = row.mainWorldId || 'unassigned';
    const list = map.get(worldId) ?? [];
    list.push(row);
    map.set(worldId, list);
  }
  return Array.from(map.entries())
    .map(([worldId, rows]) => ({
      worldId,
      label: worldId === 'unassigned' ? '未分配' : worldLabel(worldId),
      rows: rows.sort((a, b) => b.suggestedValue - a.suggestedValue || a.itemId.localeCompare(b.itemId)),
      maxPrice: Math.max(0, ...rows.map((row) => row.suggestedValue))
    }))
    .sort((a, b) => {
      const worldRank = (worldId: string) => Number(worldId.match(/(\d+)/)?.[1] ?? 999);
      return worldRank(a.worldId) - worldRank(b.worldId) || a.worldId.localeCompare(b.worldId);
    });
});
const dropPricingChartRows = computed(() => {
  const worldRank = (worldId: string) => {
    const match = worldId.match(/(\d+)/);
    return match ? Number(match[1]) : 999;
  };
  return [...dropPricingReport.value.sourceRows]
    .sort((a, b) =>
      worldRank(a.worldId) - worldRank(b.worldId) ||
      a.worldId.localeCompare(b.worldId) ||
      a.enemyId.localeCompare(b.enemyId) ||
      a.itemId.localeCompare(b.itemId)
    )
    .slice(0, 80)
    .map((row, index) => ({
      ...row,
      chartIndex: index,
      label: localizeKey(`item.${row.itemId}.name`, row.itemName),
      worldLabel: worldLabel(row.worldId)
    }));
});
const dropPricingChartOption = computed<DropPricingChartOption>(() => {
  const rows = dropPricingChartRows.value;
  return {
    color: ['#3157d4'],
    grid: { left: 54, right: 24, top: 24, bottom: 142, containLabel: false },
    tooltip: {
      trigger: 'axis',
      axisPointer: { type: 'line' },
      formatter: (params: unknown) => {
        const item = Array.isArray(params) ? params[0] : params;
        const index = isRecord(item) ? Math.max(0, Math.trunc(asNumber(item.dataIndex, 0))) : 0;
        const row = rows[index];
        if (!row) return '';
        return [
          `<strong>${row.label}</strong>`,
          `${row.worldLabel}`,
          `${row.enemyId}`,
          `${formatGoldDisplay(row.cappedUnitPrice)} 金`
        ].join('<br/>');
      }
    },
    xAxis: {
      type: 'category',
      data: rows.map((row) => row.label),
      axisLabel: {
        rotate: 90,
        interval: 0,
        hideOverlap: false,
        color: '#50607d',
        fontSize: 11,
        margin: 12,
        width: 120,
        overflow: 'truncate'
      },
      axisTick: { alignWithLabel: true },
      axisLine: { lineStyle: { color: '#94a3b8' } }
    },
    yAxis: {
      type: 'value',
      axisLabel: {
        color: '#50607d',
        formatter: (value: number) => formatGoldDisplay(value)
      },
      splitLine: { lineStyle: { color: '#e5ebf6' } }
    },
    dataZoom: [
      { type: 'slider', xAxisIndex: 0, height: 20, bottom: 8, start: 0, end: rows.length > 24 ? Math.max(8, 2400 / rows.length) : 100 },
      { type: 'inside', xAxisIndex: 0 }
    ],
    series: [{
      type: 'line',
      smooth: false,
      symbol: 'circle',
      symbolSize: 7,
      data: rows.map((row) => row.cappedUnitPrice),
      lineStyle: { width: 2.2 },
      itemStyle: { color: '#047857' }
    }]
  };
});

function isDropSourceWorldCollapsed(worldId: string): boolean {
  return dropSourceCollapsedWorlds[worldId] !== false;
}

function toggleDropSourceWorld(worldId: string) {
  dropSourceCollapsedWorlds[worldId] = !isDropSourceWorldCollapsed(worldId);
}

function patchDropPricingItemValue(itemId: string, value: number) {
  const item = catalog.items.find((row) => rowId(row) === itemId);
  if (!item) {
    status.value = `未找到物品: ${itemId}`;
    return;
  }
  item.economyBaseValue = Math.max(0, value);
  clearRowSearchTextCache();
  validateAll();
}
const selectedItemMainOutputAction = computed(() => {
  if (editorTable.value !== 'items') return null;
  const itemID = rowId(selectedRow.value ?? {});
  if (!itemID) return null;
  return pickPrimaryOutputAction(itemID);
});
const selectedItemOutputEconomy = computed(() => actionEconomySummary(selectedItemMainOutputAction.value));
const selectedItemDropPricing = computed(() => {
  if (editorTable.value !== 'items') return null;
  const itemID = rowId(selectedRow.value ?? {});
  if (!itemID) return null;
  return dropPricingItemMap.value.get(itemID) ?? null;
});
const selectedEquipmentEnhanceSummary = computed(() => (
  editorTable.value === 'equipment_templates' ? summarizeEquipmentEnhance(selectedRow.value) : null
));

function jumpToSelectedItemMainOutputAction() {
  const actionID = rowId(selectedItemMainOutputAction.value ?? {});
  if (!actionID) return;
  jumpToRow('actions', actionID);
}

function jumpToSelectedItemDropEnemy() {
  const enemyID = selectedItemDropPricing.value?.mainEnemyId;
  if (!enemyID) return;
  jumpToRow('enemies', enemyID);
}

function dropPricingRiskClass(flag: string): string {
  if (flag === '建议过高') return 'danger';
  if (flag === '敌人偏离模板') return 'warn';
  if (flag === '占位价' || flag === '未定价') return 'muted';
  return '';
}

function dropPricingDeltaText(value: number | null): string {
  if (value == null || !Number.isFinite(value)) return '-';
  return formatPercent(value);
}

const DROP_PRICING_PARAM_META: Record<string, { label: string; help: string }> = {
  targetGoldPerSecond: { label: '战斗秒收益', help: '战斗时间的金币锚点。默认 1，表示每秒战斗时间按 1 金币估算。' },
  dropBudgetShare: { label: '掉落收益占比', help: '整波战斗收益里分配给怪物掉落物的比例。越高掉落建议价越高。' },
  defaultPlayerCount: { label: '默认队伍人数', help: '估算副本刷怪数量时使用的人数，会影响 extraEnemyPerPlayer。' },
  waveScorePerSecond: { label: '评分换算秒', help: '每秒战斗时间约等于多少敌人评分。数值越大，估算战斗时间越短，掉落预算越低。' },
  specialWaveHorizon: { label: '特殊波观察窗口', help: '估算固定/周期特殊波时观察多少波。窗口越大，低频 Boss 的期望出现次数越稳定。' },
  worldFloorGrowth: { label: '副本下限成长', help: '后续副本的软下限成长率，用来让高难副本基础材料略贵一点。' },
  worldFloorMaxMultiplier: { label: '副本下限最大倍率', help: '副本软下限成长的封顶倍率，避免高阶副本把普通材料抬得太高。' },
  enemyLevelPriceGrowth: { label: '敌人等级价格成长', help: '按敌人等级温和提高该来源的软下限和硬上限。只扩大价格空间，不直接扩大整波预算。' },
  enemyLevelPriceMaxMultiplier: { label: '等级价格最大倍率', help: '敌人等级价格成长的封顶倍率，控制高等级副本和低等级副本的价差不要过大。' },
  ignoreExistingPriceBelow: { label: '占位价阈值', help: '当前价格小于等于该值时视为占位价，不参与拟合误差。' },
  overpricedWarnRatio: { label: '过高风险阈值', help: '建议价超过当前价多少倍时标记为“建议过高”。默认 1.5 表示超过 150%。' },
  enemyTemplateDeviationWarnRatio: { label: '敌人偏离提示阈值', help: '实际敌人评分相对模板评分偏离过大时提示检查敌人数值。' }
};

function dropPricingParamMeta(key: string) {
  return DROP_PRICING_PARAM_META[key] ?? { label: key, help: key };
}

function patchDropPricingRarityRule(rule: DropPricingRarityRule, patch: Partial<DropPricingRarityRule>) {
  Object.assign(rule, patch);
}

// Derived stat formulas from global-config.json: targetAttr → [{source, coefficient}]
const ENEMY_DERIVED_FORMULAS: Record<string, Array<{ attrKey: string; coefficient: number }>> = {
  hp:                                  [{ attrKey: 'stamina', coefficient: 9 }, { attrKey: 'strength', coefficient: 1.2 }],
  mp:                                  [{ attrKey: 'intelligence', coefficient: 4 }],
  hp_regen:                            [{ attrKey: 'stamina', coefficient: 0.2 }],
  mp_regen:                            [{ attrKey: 'intelligence', coefficient: 0.1 }],
  speed:                               [{ attrKey: 'dexterity', coefficient: 0.45 }],
  physics_damage:                      [{ attrKey: 'strength', coefficient: 0.55 }, { attrKey: 'dexterity', coefficient: 0.25 }],
  magical_damage:                      [{ attrKey: 'intelligence', coefficient: 0.7 }, { attrKey: 'perception', coefficient: 0.3 }],
  physics_defence:                     [{ attrKey: 'stamina', coefficient: 0.45 }, { attrKey: 'strength', coefficient: 0.15 }],
  magical_defence:                     [{ attrKey: 'stamina', coefficient: 0.35 }, { attrKey: 'perception', coefficient: 0.5 }],
  crit_chance:                         [{ attrKey: 'dexterity', coefficient: 0.005 }, { attrKey: 'luck', coefficient: 0.007 }],
  crit_damage:                         [{ attrKey: 'dexterity', coefficient: 0.009 }, { attrKey: 'strength', coefficient: 0.003 }],
  magic_crit_chance:                   [{ attrKey: 'perception', coefficient: 0.004 }, { attrKey: 'luck', coefficient: 0.007 }],
  magic_crit_damage:                   [{ attrKey: 'perception', coefficient: 0.005 }, { attrKey: 'intelligence', coefficient: 0.0025 }],
  crit_damage_resist:                  [{ attrKey: 'strength', coefficient: 0.001 }, { attrKey: 'perception', coefficient: 0.001 }, { attrKey: 'stamina', coefficient: 0.002 }],
  physics_flat_defense_penetration:    [{ attrKey: 'strength', coefficient: 0.2 }],
  magicial_flat_defense_penetration:   [{ attrKey: 'intelligence', coefficient: 0.1 }, { attrKey: 'perception', coefficient: 0.15 }],
};

// Groups for enemy attribute panel — mirrors player character attribute screen layout
const ENEMY_ATTR_GROUPS: Array<{ key: string; label: string; attrs: string[] }> = [
  { key: 'primary',   label: '主属性',   attrs: ['strength', 'dexterity', 'intelligence', 'stamina', 'luck', 'perception'] },
  { key: 'survival',  label: '生存',     attrs: ['hp', 'hp_regen', 'physics_defence', 'magical_defence'] },
  { key: 'resource',  label: '资源',     attrs: ['mp', 'mp_regen'] },
  { key: 'output',    label: '输出',     attrs: ['physics_damage', 'magical_damage'] },
  { key: 'critical',  label: '暴击',     attrs: ['crit_chance', 'crit_damage', 'magic_crit_chance', 'magic_crit_damage', 'crit_damage_resist'] },
  { key: 'tempo',     label: '节奏',     attrs: ['speed'] },
  { key: 'amplify',   label: '乘区',     attrs: ['physics_damage_amplification', 'magical_damage_amplification', 'physics_damage_resist', 'magical_damage_resist'] },
  { key: 'penetrate', label: '穿透',     attrs: ['physics_flat_defense_penetration', 'magicial_flat_defense_penetration', 'physics_percent_defense_penetration', 'magicial_percent_defense_penetration'] },
];

function enemyDerivedContribution(row: DataRow, targetAttr: string): { contribution: number; parts: string } | null {
  const terms = ENEMY_DERIVED_FORMULAS[targetAttr];
  if (!terms?.length) return null;
  let total = 0;
  const parts: string[] = [];
  for (const { attrKey, coefficient } of terms) {
    const src = asNumber(row[attrKey], 0);
    if (src === 0) continue;
    const contrib = src * coefficient;
    total += contrib;
    const attrLabel = localizeKey(`attr.${attrKey}.name`, attrKey);
    const srcStr = `${attrLabel}${src}×${coefficient}`;
    parts.push(srcStr);
  }
  if (total === 0) return null;
  return { contribution: total, parts: parts.join(' + ') };
}

function enemyAttrScoreMeta(attrKey: string, rawValue: unknown): { unitLabel: string; totalLabel: string } {
  const rule = scoringAttribute(attrKey);
  const unitValue = Math.max(0, asNumber(rule?.unitValue, 0));
  const amount = asNumber(rawValue, 0);
  const total = amount * (rule?.displayAsPercent ? 100 : 1) * unitValue;
  return {
    unitLabel: `${rule?.displayAsPercent ? '每1%' : '每点'} ${formatScoreNumber(unitValue)}分`,
    totalLabel: `当前 ${formatScoreNumber(total)}分`
  };
}

function attributeCategory(attrKey: string): string {
  const category = asString(catalog.attributes.find((row) => rowId(row) === attrKey)?.category);
  if (category) return category;
  return scoringAttribute(attrKey)?.category ?? 'other';
}

function attributeGroupLabel(category: string): string {
  if (category === 'battle') return '战斗属性';
  if (category === 'work') return '生活属性';
  if (category === 'other') return '其他属性';
  return category;
}

function groupedAttributeOptions(): Array<{ key: string; label: string; attrs: string[] }> {
  const order = ['battle', 'work', 'other'];
  const map = new Map<string, string[]>();
  for (const attr of attributeOptions.value) {
    const category = attributeCategory(attr);
    const list = map.get(category) ?? [];
    list.push(attr);
    map.set(category, list);
  }
  return Array.from(map.entries())
    .map(([key, attrs]) => ({ key, label: attributeGroupLabel(key), attrs }))
    .sort((a, b) => {
      const ai = order.indexOf(a.key);
      const bi = order.indexOf(b.key);
      return (ai < 0 ? 999 : ai) - (bi < 0 ? 999 : bi) || a.label.localeCompare(b.label, 'zh-Hans-CN');
    });
}

function fieldLinks(field: FieldDef, row: DataRow, value: unknown): FieldLink[] {
  const id = asString(value);
  if (editorTable.value === 'items' && field.key === 'useGrantSkillId' && id) {
    return [{ label: '技能', table: 'skills', id }];
  }
  if (editorTable.value === 'items' && field.key === 'battleUsableItemId' && id) {
    return [{ label: '战斗物品', table: 'battle_usable_items', id }];
  }
  if (editorTable.value === 'items' && field.key === 'isEquipment' && Boolean(value)) {
    const equipmentID = rowId(row);
    if (equipmentID && equipmentOptions.value.includes(equipmentID)) {
      return [{ label: '装备', table: 'equipment_templates', id: equipmentID }];
    }
  }
  if (editorTable.value === 'battle_usable_items' && field.key === 'itemId' && id) {
    return [{ label: '物品', table: 'items', id }];
  }
  if (editorTable.value === 'actions' && field.key === 'mainOutput' && id) {
    return [{ label: '物品', table: 'items', id }];
  }
  if (editorTable.value === 'skills' && field.key === 'category' && id) {
    return [{ label: '类型', table: 'skill_type', id }];
  }
  if (editorTable.value === 'buff' && field.key === 'type') {
    const buffID = rowId(row);
    return [
      ...(id ? [{ label: '类型', table: 'buff_type' as EditableTable, id }] : []),
      ...(buffID ? [{ label: '效果', table: 'buff_effects' as EditableTable, match: (effectRow: DataRow) => asString(effectRow.buffId) === buffID }] : [])
    ];
  }
  if (editorTable.value === 'buff_effects' && field.key === 'buffId' && id) {
    return [{ label: 'Buff', table: 'buff', id }];
  }
  if (['enhanceProtectItemId', 'toolItemId', 'carrierItemId'].includes(field.key) && id) {
    return [{ label: '物品', table: 'items', id }];
  }
  return [];
}

function stringListLink(field: FieldDef): { table: EditableTable; label: string } | null {
  if (field.key === 'skillIds') return { table: 'skills', label: '技能' };
  if (field.key === 'weaponRequirement' || field.key === 'weaponClass') return { table: 'weapon_classes', label: '武器' };
  if (field.key === 'affixAllowedIds') return { table: 'affix_definitions', label: '词条' };
  if (field.key === 'enemyIds' || field.key === 'specialEnemyIds') return { table: 'enemies', label: '敌人' };
  if (field.key === 'type' && editorTable.value === 'items') return null;
  return null;
}

function buildRowSearchText(row: DataRow): string {
  const id = rowId(row);
  const nameKey = asString(row.nameKey);
  return [
    id,
    displayName(row),
    asString(row.name),
    nameKey,
    nameKey ? localizeKey(nameKey) : '',
    nameKey ? t(nameKey) : '',
    JSON.stringify(row)
  ]
    .filter(Boolean)
    .join(' ')
    .toLowerCase();
}

function displayName(row: DataRow) {
  const id = rowId(row, '(no id)');
  return localizeKey(row.nameKey)
    || localizeKey(`item.${id}.name`)
    || localizeKey(`skill.${id}.name`)
    || localizeKey(`buff.${id}.name`)
    || localizeKey(`affix.${id}.name`)
    || localizeKey(`building.${id}.name`)
    || asString(row.name)
    || id;
}

function comparePanelKey(table: EditableTable, index: number): string {
  return `${table}:${index}`;
}

function bringComparePanelToFront(key: string) {
  const panel = comparePanels.value.find((item) => item.key === key);
  if (!panel) return;
  comparePanelZSeed.value += 1;
  panel.z = comparePanelZSeed.value;
}

function toggleComparePanel(index: number, event: MouseEvent) {
  event.stopPropagation();
  const table = activeTable.value;
  const key = comparePanelKey(table, index);
  const existing = comparePanels.value.findIndex((panel) => panel.key === key);
  if (existing >= 0) {
    comparePanels.value.splice(existing, 1);
    return;
  }
  const trigger = event.currentTarget as HTMLElement | null;
  const rect = trigger?.getBoundingClientRect();
  const offset = comparePanels.value.length % 5;
  comparePanels.value.push({
    key,
    table,
    index,
    top: Math.max(12, Math.round((rect?.top ?? 80) + 8 + offset * 18)),
    left: Math.max(12, Math.round((rect?.right ?? 360) + 10 + offset * 18)),
    z: ++comparePanelZSeed.value
  });
}

function closeComparePanel(key: string) {
  comparePanels.value = comparePanels.value.filter((panel) => panel.key !== key);
}

function comparePanelRow(table: EditableTable, index: number): DataRow | null {
  return catalog[table]?.[index] ?? null;
}

function formatCompareValue(value: unknown): string {
  if (value === null || value === undefined) return '';
  if (typeof value === 'string') {
    const label = optionLabel(value);
    return label && label !== value ? `${label} (${value})` : value;
  }
  if (typeof value === 'number' || typeof value === 'boolean') return String(value);
  if (Array.isArray(value)) return value.map((item, index) => `${index + 1}. ${formatCompareValue(item)}`).join('\n');
  if (isRecord(value)) {
    return Object.entries(value)
      .map(([key, entryValue]) => {
        const keyLabel = optionLabel(key);
        const displayKey = keyLabel && keyLabel !== key ? `${keyLabel} (${key})` : key;
        return `${displayKey}: ${formatCompareValue(entryValue)}`;
      })
      .join('\n');
  }
  return String(value);
}

function comparePanelRows(table: EditableTable, index: number): Array<{ key: string; label: string; value: string; multiline: boolean }> {
  const row = comparePanelRow(table, index);
  if (!row) return [];
  const fields = tableFields(table).filter((field) => !field.key.startsWith('__'));
  const known = new Set(fields.map((field) => field.key));
  const rows = fields.map((field) => {
    const value = formatCompareValue(row[field.key]);
    return {
      key: field.key,
      label: `${field.label} (${field.key})`,
      value,
      multiline: value.includes('\n') || value.length > 64
    };
  });
  const advanced = Object.keys(row)
    .filter((key) => !known.has(key) && !(table === 'enemies' && attributeOptions.value.includes(key)))
    .sort()
    .map((key) => {
      const value = formatCompareValue(row[key]);
      return { key, label: key, value, multiline: value.includes('\n') || value.length > 64 };
    });
  return [...rows, ...advanced];
}

function onComparePanelDrag(event: MouseEvent) {
  if (!comparePanelDragging.value) return;
  const panel = comparePanels.value.find((item) => item.key === comparePanelDragging.value?.key);
  if (!panel) return;
  panel.left = Math.max(8, Math.round(event.clientX - comparePanelDragging.value.offsetX));
  panel.top = Math.max(8, Math.round(event.clientY - comparePanelDragging.value.offsetY));
}

function stopComparePanelDrag() {
  comparePanelDragging.value = null;
  window.removeEventListener('mousemove', onComparePanelDrag);
  window.removeEventListener('mouseup', stopComparePanelDrag);
}

function startComparePanelDrag(key: string, event: MouseEvent) {
  const target = event.target as HTMLElement | null;
  if (target?.closest('button')) return;
  bringComparePanelToFront(key);
  const panel = comparePanels.value.find((item) => item.key === key);
  if (!panel) return;
  comparePanelDragging.value = {
    key,
    offsetX: event.clientX - panel.left,
    offsetY: event.clientY - panel.top
  };
  window.addEventListener('mousemove', onComparePanelDrag);
  window.addEventListener('mouseup', stopComparePanelDrag);
}

onBeforeUnmount(() => {
  if (filterDebounceTimer) clearTimeout(filterDebounceTimer);
  stopComparePanelDrag();
});

function optionLabel(option: string) {
  const override = OPTION_LABEL_OVERRIDES[option];
  if (override) return `${override} (${option})`;
  const keys = [
    `attr.${option}.name`,
    `rarity.${option}.name`,
    `rarity.${option}`,
    `item.${option}.name`,
    `skill.${option}.name`,
    `skill_type.${option}.name`,
    `buff.${option}.name`,
    `affix.${option}.name`,
    `action.${option}.name`,
    `weapon_class.${option}.name`,
    `building.${option}.name`,
    `equipment_secondary_class.${option}.name`,
    `game.equipSlot.${option}`,
    `game.targetDirection.${option}`,
    `game.targetSeekStrategy.${option}`,
    `game.skillTargetScope.${option}`,
    `game.buffEffectKind_${option}`,
    `world.${option}.name`,
    `kitty.personality.${option}.name`
  ];
  for (const key of keys) {
    const translated = t(key);
    if (translated !== key) return `${translated} (${option})`;
  }
  // Content keys for diary/encounter variants
  if (option.startsWith('game.catteryJournal.')) {
    const translated = t(option);
    if (translated !== option) {
      const preview = translated.replace(/\{[^}]+\}/g, '…').slice(0, 28);
      return `${preview} (${option})`;
    }
  }
  return option;
}

function jsonKeyLabel(key: string) {
  const mapped: Record<string, string> = {
    targetCountDelta: 'game.itemDetailAffixMutationFieldTargetCountDelta',
    amountBaseAddDelta: 'game.itemDetailAffixMutationFieldAmountBaseAddDelta',
    amountTermsAppend: 'game.itemDetailAffixMutationFieldAmountTermsAppend',
    statusIdsAppend: 'game.itemDetailAffixMutationFieldStatusAppend',
    statusIdsRemove: 'game.itemDetailAffixMutationFieldStatusRemove',
    targetCountMin: 'game.itemDetailAffixMutationFieldTargetCountMin',
    targetCountMax: 'game.itemDetailAffixMutationFieldTargetCountMax',
    appendEffects: 'game.itemDetailAffixMutationFieldAppendEffects',
    attrKey: '属性',
    rarityId: '稀有度',
    allowedRarities: '可用稀有度',
    rarityWeights: '稀有度权重',
    targetCount: '目标数',
    targetPositions: '目标位置',
    secondarySeek: '二次搜索',
    targetScope: '目标阵营',
    selectRule: '选择规则',
    selectAttrKey: '选择属性',
    base: '基础值',
    formula: '公式',
    max: '最大值',
    min: '最小值',
    coefficient: '系数',
    perSkillLevel: '每级成长',
    rateBps: '比例基点（100=1个百分点）', upgradeCosts: '一次性升级材料', refillCostAdd: '维护材料增量', level: '目标等级'
  };
  const mappedValue = mapped[key];
  if (!mappedValue) return key;
  const translated = t(mappedValue);
  return translated === mappedValue ? mappedValue : translated;
}

function jsonValueOptions(path: string[]): string[] {
  const key = path[path.length - 1] ?? '';
  if (editorTable.value === 'cattery_furniture') {
    if (key === 'kind') return ['owner_experience','kitty_extra_loot','action_efficiency_add'];
    if (key === 'domain') return ['life_action','fishing_cast','battle_reward','action_start'];
    if (path.includes('mainTypes')) return actionMainTypeOptions.value;
    if (path.includes('actionIds')) return catalog.actions.map(row => rowId(row)).filter(Boolean);
  }
  if (key === 'templateItemId') return equipmentOptions.value;
  if (key === 'equippedSlot') return slotOptions.value;
  if (key === 'equipType') return EQUIP_TYPES;
  if (['attrKey', 'selectAttrKey'].includes(key)) return attributeOptions.value;
  if (['rarityId'].includes(key)) return rarityOptions.value;
  if (['affixId'].includes(key)) return affixOptions.value;
  if (['buffId', 'statusId'].includes(key)) return buffOptions.value;
  if (['skillId'].includes(key)) return skillOptions.value;
  if (['itemId', 'toolItemId', 'carrierItemId', 'mainOutput'].includes(key)) return itemOptions.value;
  if (['enemyId'].includes(key)) return enemyOptions.value;
  if (['targetPositions', 'position'].includes(key)) return POSITIONS;
  if (key === 'side' || key === 'targetSide') return SIDE_OPTIONS;
  if (key === 'secondarySeek') return SEEK_OPTIONS;
  if (key === 'targetScope') return TARGET_SCOPES;
  if (key === 'selectRule') return SELECT_RULES;
  if (path.includes('enemyIds') || path.includes('specialEnemyIds') || path.includes('lineup')) return enemyOptions.value;
  if (path.includes('skillIds')) return skillOptions.value;
  if (path.includes('allowedRarities') || path.includes('allowedRarityIds')) return rarityOptions.value;
  return [];
}

function jsonLinkForValue(path: string[], value: unknown): FieldLink | null {
  const key = path[path.length - 1] ?? '';
  const id = asString(value);
  if (!id) return null;
  if (key === 'skillId' || path.includes('skillIds')) return { label: '技能', table: 'skills', id };
  if (key === 'buffId' || key === 'statusId' || path.includes('statusIds')) return { label: 'Buff', table: 'buff', id };
  if (key === 'affixId') return { label: '词条', table: 'affix_definitions', id };
  if (key === 'templateItemId') return { label: '装备', table: 'equipment_templates', id };
  if (['itemId', 'toolItemId', 'carrierItemId', 'mainOutput'].includes(key) || path.includes('enhanceCostCurve')) return { label: '物品', table: 'items', id };
  if (key === 'enemyId' || path.includes('enemyIds') || path.includes('specialEnemyIds') || path.includes('lineup')) return { label: '敌人', table: 'enemies', id };
  return null;
}

function jsonKeyOptions(path: string[]): string[] {
  const parent = path[path.length - 1] ?? '';
  if (editorTable.value === 'cattery_furniture') {
    if (['upgradeCosts','refillCostAdd'].includes(parent)) return itemOptions.value;
    if (parent === 'selector') return ['mainTypes','subTypes','actionIds'];
  }
  if (path[0] === 'equipment' && path.length === 2) {
    return ['templateItemId', 'equippedSlot', 'equipType', 'enhanceLevel', 'baseSnapshot', 'affixes'];
  }
  if (path[0] === 'equipment' && parent === 'baseSnapshot') return attributeOptions.value;
  if (path[0] === 'equipment' && parent === 'attrMods') return attributeOptions.value;
  if (parent === 'drops' || parent === 'itemReward') return itemOptions.value;
  if (['rarityWeights', 'attrMods'].includes(parent)) return rarityOptions.value;
  if (parent === 'base' || parent === 'step') {
    if (path[0] === 'enhanceCostCurve') return itemOptions.value;
    return attributeOptions.value;
  }
  if (path.includes('attrMods') && path.length >= 2) return attributeOptions.value;
  if ((path[0] === 'enhanceAttrCurve' || path[0] === 'levelAttrCurve') && path.includes('overrides') && path.length >= 3) return attributeOptions.value;
  if (path[0] === 'enhanceCostCurve' && path.includes('overrides') && path.length >= 3) return itemOptions.value;
  return [];
}

function defaultJsonObjectValue(path: string[], key: string): unknown {
  const parent = path[path.length - 1] ?? '';
  if (editorTable.value === 'cattery_furniture') {
    if (['upgradeCosts','refillCostAdd'].includes(parent)) return 1;
    if (parent === 'selector') return [];
  }
  if (parent === 'drops') {
    return {
      percent: 0.01,
      range: { min: 1, max: 1 }
    };
  }
  if (parent === 'itemReward') return 1;
  if (parent === 'targetCount') {
    if (key === 'formula') return [];
    if (key === 'base') return 1;
    if (key === 'max') return 1;
  }
  if (path[0] === 'equipment' && path.length === 2) {
    if (key === 'baseSnapshot') return {};
    if (key === 'affixes') return [];
  }
  if (path[0] === 'equipment') {
    if (key === 'templateItemId') return equipmentOptions.value[0] ?? '';
    if (key === 'equippedSlot') return slotOptions.value[0] ?? 'right_hand';
    if (key === 'equipType') return 'battle';
    if (key === 'enhanceLevel' || key === 'slotIndex' || key === 'rollValue' || key === 'valuePrecision') return 0;
    if (key === 'baseSnapshot' || key === 'attrMods' || key === 'effectParams') return {};
    if (key === 'affixes') return [];
    if (key === 'affixId') return affixOptions.value[0] ?? '';
    if (key === 'rarityId') return rarityOptions.value[0] ?? '';
  }
  if (parent.endsWith('Formula') || parent === 'amountFormula') {
    if (key === 'baseAdd') return 0;
    if (key === 'terms') return [];
  }
  if (parent === 'range') return 1;
  if (path.includes('overrides') || ['attrMods', 'rarityWeights'].includes(parent)) return {};
  return '';
}

function defaultJsonArrayItem(path: string[], objectMode = false): unknown {
  const key = path[path.length - 1] ?? '';
  const parent = path[path.length - 2] ?? '';
  if (editorTable.value === 'cattery_furniture') {
    if (key === 'rules') return {kind:'action_efficiency_add',domain:'action_start',rateBps:2000,selector:{mainTypes:['collecting']}};
    if (key === 'levels') {
      const levels = Array.isArray(selectedRow.value?.levels) ? selectedRow.value.levels : [];
      const previous = levels[levels.length-1] as DataRow | undefined;
      return {level:levels.length+2,upgradeCosts:{wood_blank:160,iron:12},refillCostAdd:{},rules:structuredClone(previous?.rules ?? selectedRow.value?.rules ?? [])};
    }
    if (key === 'mainTypes') return 'collecting';
  }
  if (['effects', 'basicAttackEffects', 'appendEffects'].includes(key)) return defaultEffect('damage');
  if (key === 'equipment') {
    return {
      templateItemId: equipmentOptions.value[0] ?? '',
      equippedSlot: slotOptions.value[0] ?? 'right_hand',
      equipType: 'battle',
      enhanceLevel: 0,
      baseSnapshot: {},
      affixes: []
    };
  }
  if (key === 'affixes' && path[0] === 'equipment') {
    return {
      slotIndex: 0,
      affixId: affixOptions.value[0] ?? '',
      rarityId: rarityOptions.value[0] ?? '',
      rollValue: 1,
      attrMods: {},
      effectType: '',
      effectParams: {}
    };
  }
  if (key === 'terms' || key === 'formula') return { attrKey: attributeOptions.value[0] ?? '', coefficient: 1, perSkillLevel: 0 };
  if (key === 'targetPositions') return POSITIONS[0] ?? '';
  if (key === 'statusIds') return buffOptions.value[0] ?? '';
  if (key === 'skillIds') return skillOptions.value[0] ?? '';
  if (key === 'lineup' || key === 'enemyIds' || key === 'specialEnemyIds') return enemyOptions.value[0] ?? '';
  if (key === 'allowedRarities' || key === 'allowedRarityIds') return rarityOptions.value[0] ?? '';
  if (key === 'conditions') return {};
  if (key === 'customLineupConf') return { percent: 1, lineup: [] };
  if (key === 'specialWaves') return { wave: 1, enemyIds: [], minCount: 1, maxCount: 1 };
  if (key === 'waveSchedules') return { id: 'schedule_1', every: 3, priority: 0, enemyChoices: [], minCount: 1, maxCount: 1 };
  if (key === 'enemyChoices') return { enemyId: enemyOptions.value[0] ?? '', weight: 1 };
  if (key === 'battleHooks') {
    return {
      trigger: 'on_battle_start',
      priority: 0,
      conditions: [],
      effects: [{ kind: 'add_buff', buffId: buffOptions.value[0] ?? '', durationTurns: 1, targetMode: 'targeting', procChanceBase: 1, procChanceMin: 0, procChanceMax: 1, procChanceFormula: { baseAdd: 0, terms: [] } }]
    };
  }
  if (parent === 'battleHooks' && key === 'effects') return { kind: 'add_buff', buffId: buffOptions.value[0] ?? '', durationTurns: 1, targetMode: 'targeting', procChanceBase: 1, procChanceMin: 0, procChanceMax: 1, procChanceFormula: { baseAdd: 0, terms: [] } };
  return objectMode ? {} : '';
}

function scoreFor(table: EditableTable, row: DataRow) {
  return scoreRow(table, row, catalog, scoring);
}

function rangeText(label: string, minRaw: unknown, maxRaw: unknown): string {
  const min = Number(minRaw);
  const max = Number(maxRaw);
  if (!Number.isFinite(min) && !Number.isFinite(max)) return '';
  if (Math.max(Number.isFinite(min) ? min : 0, Number.isFinite(max) ? max : 0) <= 0) return '';
  return `${label}: ${Math.round(Number.isFinite(min) ? min : 0)}~${Math.round(Number.isFinite(max) ? max : min)}`;
}

function rowExtraScoreLine(table: EditableTable, row: DataRow): string {
  if (table === 'items') {
    const id = rowId(row);
    const pricing = id ? dropPricingItemMap.value.get(id) : null;
    if (!pricing) return '';
    const rarityText = pricing.catalogRarity !== pricing.pricingRarity ? ` / 定价稀有度 ${pricing.pricingRarity}` : '';
    return `参考 ${formatGoldDisplay(pricing.suggestedValue)}金${rarityText}`;
  }
  const detail = scoreFor(table, row).detail;
  if (table === 'equipment_templates') return rangeText('强化', detail.enhanceMin, detail.enhanceMax);
  if (table === 'affix_definitions') return rangeText('词条', detail.affixMin, detail.affixMax);
  return '';
}

function itemRowSideLine(row: DataRow): string {
  const id = rowId(row);
  const currentValue = Math.max(0, asNumber(row.economyBaseValue, 0));
  const pricing = id ? dropPricingItemMap.value.get(id) : null;
  if (pricing) return `当前 ${formatGoldDisplay(currentValue)}金\n参考 ${formatGoldDisplay(pricing.suggestedValue)}金`;
  if (currentValue > 0) return `当前 ${formatGoldDisplay(currentValue)}金`;
  const source = itemDropSource(row);
  if (source?.hasWorldBoss) return '世界首领\n不定价';
  return '-';
}

function effectSectionsForField(fieldKey: string): ScoreBreakdownSection[] {
  if (!selectedScore.value?.sections?.length) return [];
  if (fieldKey === 'effects' || fieldKey === 'basicAttackEffects') return selectedScore.value.sections;
  return [];
}

function templateOptionLabel(kind: keyof EnemyAttributeTemplates, key: string): string {
  const arr = scoring.enemyAttributeTemplates[kind];
  if (!Array.isArray(arr)) return key;
  const row = arr.find((entry) => entry.key === key);
  return row ? `${row.label} (${row.key})` : key;
}

function normalizeWeights(weights: Record<string, number>): Record<string, number> {
  const entries = Object.entries(weights)
    .map(([key, value]) => [key, Math.max(0, asNumber(value, 0))] as const)
    .filter(([key, value]) => key && value > 0 && scoring.attributes.some((attr) => attr.key === key && attr.unitValue > 0));
  const total = entries.reduce((sum, [, value]) => sum + value, 0);
  if (total <= 0) return {};
  return Object.fromEntries(entries.map(([key, value]) => [key, value / total]));
}

function selectedEnemyTemplateWeights(randomize = false): Record<string, number> {
  const role = scoring.enemyAttributeTemplates.roles.find((row) => row.key === enemyTemplateRole.value);
  const species = scoring.enemyAttributeTemplates.species.find((row) => row.key === enemyTemplateSpecies.value);
  const merged: Record<string, number> = {};
  for (const [key, value] of Object.entries(role?.weights ?? {})) merged[key] = (merged[key] ?? 0) + asNumber(value, 0);
  for (const [key, value] of Object.entries(species?.weights ?? {})) merged[key] = (merged[key] ?? 0) + asNumber(value, 0);
  if (randomize) {
    const variance = Math.max(0, Math.min(0.85, enemyTemplateVariance.value));
    for (const key of Object.keys(merged)) {
      const roll = 1 + (Math.random() * 2 - 1) * variance;
      merged[key] *= roll;
    }
  }
  return normalizeWeights(merged);
}

function selectedEnemyTemplateBudget(): number {
  const tier = scoring.enemyAttributeTemplates.tiers.find((row) => row.key === enemyTemplateTier.value);
  return Math.max(0, enemyTemplateBudget.value) * Math.max(0, tier?.budgetMultiplier ?? 1);
}

function formatEnemyTemplatePreview(): string {
  const weights = selectedEnemyTemplateWeights();
  return Object.entries(weights)
    .sort((a, b) => b[1] - a[1])
    .slice(0, 6)
    .map(([key, weight]) => `${optionLabel(key)} ${Math.round(weight * 100)}%`)
    .join(' / ');
}

function roundGeneratedAttrValue(attr: AttributeValue, value: number): number {
  const digits = attr.displayAsPercent ? 4 : Math.abs(value) >= 10 ? 0 : Math.abs(value) >= 1 ? 2 : 4;
  const base = 10 ** digits;
  return Math.round(value * base) / base;
}

function applyEnemyAttributeTemplate() {
  const row = selectedRow.value;
  if (!row || editorTable.value !== 'enemies') return;

  const level = Math.max(1, Math.trunc(enemyTemplateLevel.value || 1));
  const tierKey = enemyTemplateTier.value;
  const roleKey = enemyTemplateRole.value;
  const speciesKey = enemyTemplateSpecies.value;
  const tier = scoring.enemyAttributeTemplates.tiers.find((t) => t.key === tierKey);

  // 使用 generateEnemyStats 生成基础属性值
  const stats = generateEnemyStats(level, tierKey, roleKey, speciesKey, balanceModel, scoring);
  Object.assign(stats, buildEnemyDifficultyGuardrailPatch(
    { ...stats, level, type: tier?.enemyType || 'normal' },
    difficultyAuthoringGuardrails
  ));

  // 更新等级和类型
  row.level = level;
  if (tier?.enemyType) row.type = tier.enemyType;

  // 清除旧属性
  for (const attr of scoring.attributes) {
    if (Object.prototype.hasOwnProperty.call(row, attr.key)) delete row[attr.key];
  }

  // 应用新属性（generateEnemyStats 返回的已经是基础值，直接写入）
  for (const [key, value] of Object.entries(stats)) {
    if (value !== 0) row[key] = value;
  }

  // 计算建议经验
  validateAll();
  const score = scoreRow('enemies', row, catalog, scoring);
  const baseline = baselineAtLevel(balanceModel, level);
  const tierRewardMult = tier?.rewardMult ?? 1;
  const recommendedExp = Math.max(1, Math.round(score.score * balanceModel.expPerPower * baseline.expRate * tierRewardMult));

  // 填入建议经验
  row.exp = recommendedExp;
}

function applyEnemyDifficultyGuardrailsToCurrent() {
  const row = selectedRow.value;
  if (!row || editorTable.value !== 'enemies') return;
  applyEnemyDifficultyGuardrails(row, difficultyAuthoringGuardrails);
  validateAll();
}

const enemyTemplateDerivedStats = computed(() => {
  const level = Math.max(1, enemyTemplateLevel.value || 1);
  const tierKey = enemyTemplateTier.value;
  const roleKey = enemyTemplateRole.value;
  const speciesKey = enemyTemplateSpecies.value;
  const tier = scoring.enemyAttributeTemplates.tiers.find((entry) => entry.key === tierKey);

  // 使用 generateEnemyStats 生成基础属性值（与 apply 逻辑一致）
  const baseStats = generateEnemyStats(level, tierKey, roleKey, speciesKey, balanceModel, scoring);
  Object.assign(baseStats, buildEnemyDifficultyGuardrailPatch(
    { ...baseStats, level, type: tier?.enemyType || 'normal' },
    difficultyAuthoringGuardrails
  ));

  // 提取主属性
  const str = baseStats.strength ?? 0;
  const dex = baseStats.dexterity ?? 0;
  const int = baseStats.intelligence ?? 0;
  const per = baseStats.perception ?? 0;
  const sta = baseStats.stamina ?? 0;

  // 计算派生值（基础 + 派生）
  const derivedHp = sta * 9 + str * 1.2;
  const derivedMp = int * 4;
  const derivedSpeed = dex * 0.45;
  const derivedPhysicsDamage = str * 0.55 + dex * 0.25;
  const derivedMagicalDamage = int * 0.7 + per * 0.3;
  const defensive = projectEnemyDefensiveStats(baseStats);

  const baseHp = baseStats.hp ?? 0;
  const baseMp = baseStats.mp ?? 0;
  const baseSpeed = baseStats.speed ?? 0;
  const basePhysicsDamage = baseStats.physics_damage ?? 0;
  const baseMagicalDamage = baseStats.magical_damage ?? 0;
  return {
    strength: Math.round(str),
    dexterity: Math.round(dex),
    intelligence: Math.round(int),
    perception: Math.round(per),
    stamina: Math.round(sta),
    hp: Math.round(baseHp + derivedHp),
    mp: Math.round(baseMp + derivedMp),
    speed: Math.round(baseSpeed + derivedSpeed),
    physics_damage: Math.round(basePhysicsDamage + derivedPhysicsDamage),
    magical_damage: Math.round(baseMagicalDamage + derivedMagicalDamage),
    physics_defence: Math.round(defensive.physicsDefence),
    magical_defence: Math.round(defensive.magicalDefence),
    crit_damage_resist: defensive.critDamageResist
  };
});

// 实际敌人属性（从选中行提取，计算派生后的最终值）
const actualEnemyStats = computed(() => {
  if (editorTable.value !== 'enemies' || !selectedRow.value) return null;
  const row = selectedRow.value as Record<string, unknown>;

  // 提取主属性
  const getStat = (key: string): number => {
    const val = Number(row[key]);
    return Number.isFinite(val) ? val : 0;
  };

  const str = getStat('strength');
  const dex = getStat('dexterity');
  const int = getStat('intelligence');
  const per = getStat('perception');
  const sta = getStat('stamina');

  // 提取基础派生属性（存储在表中的值）
  const baseHp = getStat('hp');
  const baseMp = getStat('mp');
  const baseSpeed = getStat('speed');
  const basePhysicsDamage = getStat('physics_damage');
  const baseMagicalDamage = getStat('magical_damage');

  // 计算派生值（与服务器逻辑一致）
  const derivedHp = sta * 9 + str * 1.2;
  const derivedMp = int * 4;
  const derivedSpeed = dex * 0.45;
  const derivedPhysicsDamage = str * 0.55 + dex * 0.25;
  const derivedMagicalDamage = int * 0.7 + per * 0.3;
  const defensive = projectEnemyDefensiveStats(row);

  // 最终值 = 基础值 + 派生值
  const finalHp = baseHp + derivedHp;
  const finalMp = baseMp + derivedMp;
  const finalSpeed = baseSpeed + derivedSpeed;
  const finalPhysicsDamage = basePhysicsDamage + derivedPhysicsDamage;
  const finalMagicalDamage = baseMagicalDamage + derivedMagicalDamage;

  return {
    hp: Math.round(finalHp),
    mp: Math.round(finalMp),
    speed: Math.round(finalSpeed),
    physics_damage: Math.round(finalPhysicsDamage),
    magical_damage: Math.round(finalMagicalDamage),
    physics_defence: Math.round(defensive.physicsDefence),
    magical_defence: Math.round(defensive.magicalDefence),
    crit_damage_resist: defensive.critDamageResist,
    // 保留原始主属性用于调试
    strength: Math.round(str),
    dexterity: Math.round(dex),
    intelligence: Math.round(int),
    perception: Math.round(per),
    stamina: Math.round(sta)
  };
});

function addScoringAttribute() {
  scoring.attributes.push({ key: `attr_${scoring.attributes.length + 1}`, label: '新属性', category: 'custom', unitValue: 1 });
}

function removeScoringAttribute(index: number) {
  scoring.attributes.splice(index, 1);
}

function formatDiff(calculated: number, actual?: number): string {
  if (actual == null || !Number.isFinite(actual)) return '';
  const diff = actual - calculated;
  const pct = calculated > 0 ? (diff / calculated) * 100 : 0;
  const sign = diff > 0 ? '+' : '';
  return `${sign}${Math.round(diff)} (${sign}${pct.toFixed(1)}%)`;
}

function getDiffClass(calculated: number, actual?: number): string {
  if (actual == null || !Number.isFinite(actual)) return '';
  const pct = calculated > 0 ? Math.abs((actual - calculated) / calculated) * 100 : 0;
  if (pct < 10) return 'good';
  if (pct < 30) return 'warn';
  return 'danger';
}

function updateEnemyAttributeTemplates(value: unknown) {
  if (!isRecord(value)) return;
  const next = value as unknown as EnemyAttributeTemplates;
  scoring.enemyAttributeTemplates.tiers.splice(0, scoring.enemyAttributeTemplates.tiers.length, ...(Array.isArray(next.tiers) ? next.tiers : []));
  scoring.enemyAttributeTemplates.roles.splice(0, scoring.enemyAttributeTemplates.roles.length, ...(Array.isArray(next.roles) ? next.roles : []));
  scoring.enemyAttributeTemplates.species.splice(0, scoring.enemyAttributeTemplates.species.length, ...(Array.isArray(next.species) ? next.species : []));
}

const NumberInput = defineComponent({
  name: 'NumberInput',
  props: {
    modelValue: { type: null, required: false }
  },
  emits: ['update:modelValue'],
  setup(props, { emit }) {
    const focused = ref(false);
    const text = ref(formatNumberInput(props.modelValue));
    watch(
      () => props.modelValue,
      (value) => {
        if (!focused.value) text.value = formatNumberInput(value);
      }
    );
    const commit = () => {
      const value = text.value;
      if (isIncompleteNumberText(value)) {
        text.value = '0';
        emit('update:modelValue', 0);
        return;
      }
      const parsed = Number(value);
      if (!Number.isFinite(parsed)) {
        text.value = formatNumberInput(props.modelValue);
        return;
      }
      emit('update:modelValue', parsed);
      text.value = String(parsed);
    };
    const onInput = (event: Event) => {
      const input = event.target as HTMLInputElement;
      const next = (event.target as HTMLInputElement).value;
      if (!isAllowedNumberText(next)) {
        input.value = text.value;
        return;
      }
      text.value = next;
    };
    const onBlur = () => {
      focused.value = false;
      commit();
    };
    const onFocus = () => {
      focused.value = true;
    };
    const onKeydown = (event: KeyboardEvent) => {
      if (event.key === 'Enter') {
        event.preventDefault();
        commit();
        (event.target as HTMLInputElement).blur();
      } else if (event.key === 'Escape') {
        event.preventDefault();
        text.value = formatNumberInput(props.modelValue);
        (event.target as HTMLInputElement).blur();
      }
    };
    return { text, onInput, onBlur, onFocus, onKeydown };
  },
  template: `
    <input
      class="number-input"
      type="text"
      inputmode="decimal"
      :value="text"
      @focus="onFocus"
      @blur="onBlur"
      @input="onInput"
      @keydown="onKeydown"
      @wheel.prevent
    />
  `
});

function formatNumberInput(value: unknown): string {
  if (value === '' || value === null || value === undefined) return '';
  const parsed = Number(value);
  return Number.isFinite(parsed) ? String(parsed) : '';
}

function isIncompleteNumberText(value: string): boolean {
  const trimmed = value.trim();
  return trimmed === '' || trimmed === '-' || trimmed === '.' || trimmed === '-.';
}

function isAllowedNumberText(value: string): boolean {
  return /^-?(?:\d+)?(?:\.\d*)?$/.test(value.trim());
}

const JsonFieldEditor = defineComponent({
  name: 'JsonFieldEditor',
  components: { NumberInput },
  props: {
    modelValue: { type: null, required: false },
    options: { type: Array<string>, required: false, default: () => [] },
    path: { type: Array<string>, required: false, default: () => [] }
  },
  emits: ['update:modelValue'],
  setup(props, { emit }) {
    const update = (value: unknown) => emit('update:modelValue', value);
    const eventValue = (event: Event) => (event.target as HTMLInputElement | HTMLSelectElement).value;
    const eventChecked = (event: Event) => (event.target as HTMLInputElement).checked;
    const selectOptions = computed(() => jsonValueOptions(props.path as string[]).length ? jsonValueOptions(props.path as string[]) : props.options);
    const keySelectOptions = computed(() => jsonKeyOptions(props.path as string[]));
    const valueLink = computed(() => jsonLinkForValue(props.path as string[], props.modelValue));
    const childPath = (key: string | number) => [...(props.path as string[]), String(key)];
    const addKey = () => {
      const options = keySelectOptions.value;
      const key = options.length && isRecord(props.modelValue)
        ? options.find((option) => !Object.prototype.hasOwnProperty.call(props.modelValue, option)) ?? ''
        : prompt('新增字段名');
      if (!key || !isRecord(props.modelValue) || Object.prototype.hasOwnProperty.call(props.modelValue, key)) return;
      const path = props.path as string[];
      const value = defaultJsonObjectValue(path, key);
      update({ ...props.modelValue, [key]: value });
    };
    const addObjectKey = () => {
      const key = prompt('新增对象字段名');
      if (!key || !isRecord(props.modelValue) || Object.prototype.hasOwnProperty.call(props.modelValue, key)) return;
      update({ ...props.modelValue, [key]: defaultJsonObjectValue(props.path as string[], key) || {} });
    };
    const addArrayItem = (objectMode = false) => {
      update([...(Array.isArray(props.modelValue) ? props.modelValue : []), defaultJsonArrayItem(props.path as string[], objectMode)]);
    };
    const updateObjectKey = (oldKey: string, key: string) => {
      if (!isRecord(props.modelValue) || oldKey === key) return;
      const next = { ...props.modelValue };
      const value = next[oldKey];
      delete next[oldKey];
      if (key) next[key] = value;
      update(next);
    };
    return { update, addKey, addObjectKey, addArrayItem, updateObjectKey, eventValue, eventChecked, optionLabel, jsonKeyLabel, selectOptions, keySelectOptions, valueLink, childPath, jumpToRow, TABLE_LABELS };
  },
  template: `
    <div class="json-editor">
      <template v-if="Array.isArray(modelValue)">
        <div class="json-toolbar"><span>数组 {{ modelValue.length }}</span><div class="toolbar"><button class="ghost" @click="addArrayItem(false)">新增元素</button><button class="ghost" @click="addArrayItem(true)">新增对象</button></div></div>
        <div class="json-row" v-for="(item, index) in modelValue" :key="index">
          <span class="json-key">[{{ index }}]</span>
          <JsonFieldEditor :model-value="item" :options="options" :path="childPath(index)" @update:model-value="(value) => { const next = [...modelValue]; next[index] = value; update(next) }" />
          <button class="danger" @click="() => { const next = [...modelValue]; next.splice(index, 1); update(next) }">删</button>
        </div>
      </template>
      <template v-else-if="modelValue && typeof modelValue === 'object'">
        <div class="json-toolbar"><span>对象</span><div class="toolbar"><button class="ghost" @click="addKey">新增字段</button><button class="ghost" @click="addObjectKey">新增对象字段</button></div></div>
        <div class="json-row" v-for="(item, key) in modelValue" :key="key">
          <select v-if="keySelectOptions.length" class="json-key-select" :value="key" @change="updateObjectKey(String(key), eventValue($event))">
            <option v-for="option in keySelectOptions" :key="option" :value="option">{{ optionLabel(option) }}</option>
          </select>
          <span v-else class="json-key">{{ jsonKeyLabel(String(key)) }}</span>
          <JsonFieldEditor :model-value="item" :options="options" :path="childPath(String(key))" @update:model-value="(value) => update({ ...modelValue, [key]: value })" />
          <button class="danger" @click="() => { const next = { ...modelValue }; delete next[key]; update(next) }">删</button>
        </div>
      </template>
      <NumberInput v-else-if="typeof modelValue === 'number'" :model-value="modelValue" @update:model-value="update" />
      <label v-else-if="typeof modelValue === 'boolean'" class="checkbox-field"><input type="checkbox" :checked="modelValue" @change="update(eventChecked($event))" /> <span>{{ modelValue ? 'true' : 'false' }}</span></label>
      <input v-else-if="selectOptions.length === 0" type="text" :value="modelValue ?? ''" @input="update(eventValue($event))" />
      <div v-else class="linked-control">
        <select :value="modelValue ?? ''" @change="update(eventValue($event))">
          <option value=""></option>
          <option v-for="option in selectOptions" :key="option" :value="option">{{ optionLabel(option) }}</option>
        </select>
        <button v-if="valueLink" type="button" class="link-button" :title="'跳转到' + TABLE_LABELS[valueLink.table]" @click="jumpToRow(valueLink.table, valueLink.id || '')">{{ valueLink.label }}</button>
      </div>
    </div>
  `
});

const SearchSelect = defineComponent({
  name: 'SearchSelect',
  props: {
    modelValue: { type: String, default: '' },
    options: { type: Array as PropType<string[]>, default: () => [] },
    placeholder: { type: String, default: '搜索或选择' },
    allowEmpty: { type: Boolean, default: true }
  },
  emits: ['update:modelValue'],
  setup(props, { emit }) {
    const query = ref('');
    const focused = ref(false);
    const selectedLabel = computed(() => props.modelValue ? optionLabel(props.modelValue) : '');
    const filteredOptions = computed(() => {
      const raw = query.value.trim().toLowerCase();
      const base = props.options.filter(Boolean);
      if (!raw) return base.slice(0, 80);
      return base
        .filter((option) => `${option} ${optionLabel(option)}`.toLowerCase().includes(raw))
        .slice(0, 80);
    });
    const inputValue = computed(() => focused.value ? query.value : selectedLabel.value || props.modelValue);
    const choose = (value: string) => {
      emit('update:modelValue', value);
      query.value = '';
      focused.value = false;
    };
    const onFocus = () => {
      focused.value = true;
      query.value = '';
    };
    const onBlur = () => {
      setTimeout(() => {
        focused.value = false;
        query.value = '';
      }, 120);
    };
    const onInput = (event: Event) => {
      focused.value = true;
      query.value = (event.target as HTMLInputElement).value;
    };
    return { query, focused, filteredOptions, inputValue, choose, onFocus, onBlur, onInput, optionLabel };
  },
  template: `
    <div class="search-select">
      <input :value="inputValue" :placeholder="placeholder" @focus="onFocus" @blur="onBlur" @input="onInput" />
      <div v-if="focused" class="search-select-menu">
        <button v-if="allowEmpty" type="button" class="search-option" @mousedown.prevent="choose('')">清空</button>
        <button v-for="option in filteredOptions" :key="option" type="button" class="search-option" :class="{ active: option === modelValue }" @mousedown.prevent="choose(option)">
          <strong>{{ optionLabel(option) }}</strong>
          <span>{{ option }}</span>
        </button>
        <div v-if="filteredOptions.length === 0" class="search-empty">无匹配项</div>
      </div>
    </div>
  `
});

const ACHIEVEMENT_CONDITION_KINDS = [
  'kill_enemy',
  'catch_fish',
  'complete_action',
  'own_kitty',
  'own_item',
  'obtain_item',
  'enchant_count',
  'enhance_count',
  'complete_commission',
  'complete_rumor_commission',
  'complete_specific_commission',
  'player_attr_reached'
] as const;

const ACHIEVEMENT_CONDITION_LABELS: Record<string, string> = {
  kill_enemy: '击杀怪物',
  catch_fish: '捕获鱼种',
  complete_action: '完成动作',
  own_kitty: '拥有猫咪',
  own_item: '拥有物品',
  obtain_item: '获得物品',
  enchant_count: '附魔次数',
  enhance_count: '强化次数',
  complete_commission: '完成委托',
  complete_rumor_commission: '完成传闻委托',
  complete_specific_commission: '完成指定委托',
  player_attr_reached: '玩家属性达标'
};

function defaultAchievementCondition(kind: string): Record<string, unknown> {
  switch (kind) {
    case 'kill_enemy':
      return { kind, enemyId: '', count: 1 };
    case 'catch_fish':
      return { kind, speciesId: '', count: 1 };
    case 'complete_action':
      return { kind, actorType: 'player', actionId: '', count: 1 };
    case 'own_kitty':
      return { kind, count: 1 };
    case 'own_item':
      return { kind, itemId: '', quantity: 1 };
    case 'obtain_item':
      return { kind, itemId: '', quantity: 1 };
    case 'enhance_count':
      return { kind, count: 1, successfulOnly: false, itemId: '', minAfterLevel: 0 };
    case 'enchant_count':
      return { kind, count: 1, successfulOnly: false };
    case 'complete_specific_commission':
      return { kind, commissionId: '', count: 1 };
    case 'player_attr_reached':
      return { kind, attrKey: '', targetValue: 1, valueSource: 'base_level' };
    default:
      return { kind, count: 1 };
  }
}

const AchievementConditionEditor = defineComponent({
  name: 'AchievementConditionEditor',
  components: { SearchSelect },
  props: {
    modelValue: { type: Object as PropType<Record<string, unknown>>, default: () => ({}) },
    enemyOptions: { type: Array as PropType<string[]>, default: () => [] },
    fishingSpeciesOptions: { type: Array as PropType<string[]>, default: () => [] },
    actionOptions: { type: Array as PropType<string[]>, default: () => [] },
    actionDurationMap: { type: Object as PropType<Record<string, number>>, default: () => ({}) },
    itemOptions: { type: Array as PropType<string[]>, default: () => [] },
    commissionOptions: { type: Array as PropType<string[]>, default: () => [] },
    attrOptions: { type: Array as PropType<string[]>, default: () => [] },
    cumulativeSeconds: { type: Number, default: -1 }
  },
  emits: ['update:modelValue'],
  setup(props, { emit }) {
    const value = computed(() => isRecord(props.modelValue) ? props.modelValue : defaultAchievementCondition('complete_action'));
    const kind = computed(() => asString(value.value.kind, 'complete_action'));
    const patch = (next: Record<string, unknown>) => emit('update:modelValue', { ...value.value, ...next });
    const setKind = (nextKind: string) => emit('update:modelValue', defaultAchievementCondition(nextKind));
    const updateNumber = (key: string, raw: unknown, fallback = 0) => patch({ [key]: asNumber(raw, fallback) });
    const eventValue = (event: Event) => (event.target as HTMLInputElement | HTMLSelectElement).value;
    const eventChecked = (event: Event) => (event.target as HTMLInputElement).checked;
    const kindLabel = (nextKind: string) => ACHIEVEMENT_CONDITION_LABELS[nextKind] || nextKind;
    const actionDuration = computed(() => {
      const actionId = asString(value.value.actionId);
      return props.actionDurationMap?.[actionId] ?? 0;
    });
    const formatDuration = (sec: number) => {
      if (sec <= 0) return '';
      if (sec < 60) return sec + '秒';
      if (sec < 3600) return Math.floor(sec / 60) + '分' + (sec % 60) + '秒';
      if (sec < 86400) return (sec / 3600).toFixed(1) + '小时';
      return (sec / 86400).toFixed(1) + '天';
    };
    const actionTimeLabel = computed(() => {
      const dur = actionDuration.value;
      return dur > 0 ? '单次: ' + formatDuration(dur) : '';
    });
    const cumulativeTimeLabel = computed(() => {
      if (props.cumulativeSeconds <= 0) return '';
      return '累计: ' + formatDuration(props.cumulativeSeconds);
    });
    return { value, kind, patch, setKind, updateNumber, eventValue, eventChecked, kindLabel, ACHIEVEMENT_CONDITION_KINDS, actionTimeLabel, cumulativeTimeLabel };
  },
  template: `
    <div class="json-editor">
      <div class="json-row compact">
        <label>类型
          <select :value="kind" @change="setKind(eventValue($event))">
            <option v-for="item in ACHIEVEMENT_CONDITION_KINDS" :key="item" :value="item">{{ kindLabel(item) }}</option>
          </select>
        </label>
      </div>
      <div v-if="kind === 'kill_enemy'" class="json-row compact">
        <label>敌人<SearchSelect :model-value="String(value.enemyId || '')" :options="enemyOptions" @update:model-value="(v) => patch({ enemyId: v })" /></label>
        <label>次数<input type="number" :value="value.count ?? 1" @input="updateNumber('count', eventValue($event), 1)" /></label>
      </div>
      <div v-else-if="kind === 'catch_fish'" class="json-row compact">
        <label>鱼种<SearchSelect :model-value="String(value.speciesId || '')" :options="fishingSpeciesOptions" @update:model-value="(v) => patch({ speciesId: v })" /></label>
        <label>捕获数<input type="number" :value="value.count ?? 1" @input="updateNumber('count', eventValue($event), 1)" /></label>
      </div>
      <div v-else-if="kind === 'complete_action'" class="json-row compact">
        <label>队列
          <select :value="String(value.actorType || 'player')" @change="patch({ actorType: eventValue($event) })">
            <option value="player">玩家队列</option>
            <option value="kitty">猫咪队列</option>
            <option value="any">任意队列</option>
          </select>
        </label>
        <label>动作<SearchSelect :model-value="String(value.actionId || '')" :options="actionOptions" @update:model-value="(v) => patch({ actionId: v })" /></label>
        <span v-if="actionTimeLabel" class="meta-label time-label">{{ actionTimeLabel }}</span>
        <label>次数<input type="number" :value="value.count ?? 1" @input="updateNumber('count', eventValue($event), 1)" /></label>
        <span v-if="cumulativeTimeLabel" class="meta-label cumulative-label">{{ cumulativeTimeLabel }}</span>
      </div>
      <div v-else-if="kind === 'own_kitty'" class="json-row compact">
        <label>数量<input type="number" :value="value.count ?? 1" @input="updateNumber('count', eventValue($event), 1)" /></label>
      </div>
      <div v-else-if="kind === 'own_item'" class="json-row compact">
        <label>物品<SearchSelect :model-value="String(value.itemId || '')" :options="itemOptions" @update:model-value="(v) => patch({ itemId: v })" /></label>
        <label>数量<input type="number" :value="value.quantity ?? 1" @input="updateNumber('quantity', eventValue($event), 1)" /></label>
      </div>
      <div v-else-if="kind === 'obtain_item'" class="json-row compact">
        <label>物品<SearchSelect :model-value="String(value.itemId || '')" :options="itemOptions" @update:model-value="(v) => patch({ itemId: v })" /></label>
        <label>数量<input type="number" :value="value.quantity ?? 1" @input="updateNumber('quantity', eventValue($event), 1)" /></label>
        <label>最低强化<input type="number" :value="value.minEnhanceLevel ?? 0" @input="updateNumber('minEnhanceLevel', eventValue($event), 0)" /></label>
      </div>
      <div v-else-if="kind === 'enchant_count'" class="json-row compact">
        <label>次数<input type="number" :value="value.count ?? 1" @input="updateNumber('count', eventValue($event), 1)" /></label>
        <label><input type="checkbox" :checked="!!value.successfulOnly" @change="patch({ successfulOnly: eventChecked($event) })" /> 只统计成功</label>
      </div>
      <div v-else-if="kind === 'enhance_count'" class="json-row compact">
        <label>次数<input type="number" :value="value.count ?? 1" @input="updateNumber('count', eventValue($event), 1)" /></label>
        <label>物品<SearchSelect :model-value="String(value.itemId || '')" :options="itemOptions" @update:model-value="(v) => patch({ itemId: v })" /></label>
        <label>最低结果等级<input type="number" :value="value.minAfterLevel ?? 0" @input="updateNumber('minAfterLevel', eventValue($event), 0)" /></label>
        <label><input type="checkbox" :checked="!!value.successfulOnly" @change="patch({ successfulOnly: eventChecked($event) })" /> 只统计成功</label>
      </div>
      <div v-else-if="kind === 'complete_commission' || kind === 'complete_rumor_commission'" class="json-row compact">
        <label>次数<input type="number" :value="value.count ?? 1" @input="updateNumber('count', eventValue($event), 1)" /></label>
      </div>
      <div v-else-if="kind === 'complete_specific_commission'" class="json-row compact">
        <label>委托<SearchSelect :model-value="String(value.commissionId || '')" :options="commissionOptions" @update:model-value="(v) => patch({ commissionId: v })" /></label>
        <label>次数<input type="number" :value="value.count ?? 1" @input="updateNumber('count', eventValue($event), 1)" /></label>
      </div>
      <div v-else-if="kind === 'player_attr_reached'" class="json-row compact">
        <label>属性<SearchSelect :model-value="String(value.attrKey || '')" :options="attrOptions" @update:model-value="(v) => patch({ attrKey: v })" /></label>
        <label>目标值<input type="number" :value="value.targetValue ?? 1" @input="updateNumber('targetValue', eventValue($event), 1)" /></label>
        <label>取值
          <select :value="String(value.valueSource || 'base_level')" @change="patch({ valueSource: eventValue($event) })">
            <option value="base_level">基础等级</option>
            <option value="final_value">最终值</option>
            <option value="skill_point_level">战斗点等级</option>
          </select>
        </label>
      </div>
    </div>
  `
});

const AchievementRewardsEditor = defineComponent({
  name: 'AchievementRewardsEditor',
  components: { SearchSelect },
  props: {
    modelValue: { type: Array as PropType<Record<string, unknown>[]>, default: () => [] },
    itemOptions: { type: Array as PropType<string[]>, default: () => [] },
    titlePrefixOptions: { type: Array as PropType<string[]>, default: () => [] },
    titleSuffixOptions: { type: Array as PropType<string[]>, default: () => [] }
  },
  emits: ['update:modelValue'],
  setup(props, { emit }) {
    const rows = computed(() => asArray(props.modelValue).filter(isRecord));
    const updateAt = (index: number, patch: Record<string, unknown>) => {
      const next = rows.value.map((row) => ({ ...row }));
      next[index] = { ...next[index], ...patch };
      emit('update:modelValue', next);
    };
    const removeAt = (index: number) => emit('update:modelValue', rows.value.filter((_, i) => i !== index));
    const add = (kind: string) => emit('update:modelValue', [
      ...rows.value,
      kind === 'wardrobe'
        ? { kind: 'wardrobe', category: 'title_suffix', itemId: '' }
        : { kind: 'item', itemId: '', quantity: 1 }
    ]);
    const titleOptions = (row: Record<string, unknown>) => String(row.category || '') === 'title_prefix' ? props.titlePrefixOptions : props.titleSuffixOptions;
    const eventValue = (event: Event) => (event.target as HTMLInputElement | HTMLSelectElement).value;
    return { rows, updateAt, removeAt, add, titleOptions, eventValue, asNumber };
  },
  template: `
    <div class="array-editor">
      <div class="json-toolbar"><span>奖励 {{ rows.length }}</span><div class="toolbar"><button class="ghost" type="button" @click="add('item')">新增物品</button><button class="ghost" type="button" @click="add('wardrobe')">新增称号</button></div></div>
      <div class="json-row compact" v-for="(row, index) in rows" :key="index">
        <label>类型
          <select :value="String(row.kind || 'item')" @change="updateAt(index, eventValue($event) === 'wardrobe' ? { kind: 'wardrobe', category: 'title_suffix', itemId: '' } : { kind: 'item', itemId: '', quantity: 1 })">
            <option value="item">物品</option>
            <option value="wardrobe">称号</option>
          </select>
        </label>
        <template v-if="String(row.kind || 'item') === 'wardrobe'">
          <label>称号类型
            <select :value="String(row.category || 'title_suffix')" @change="updateAt(index, { category: eventValue($event), itemId: '' })">
              <option value="title_prefix">前缀</option>
              <option value="title_suffix">后缀</option>
            </select>
          </label>
          <label>称号<SearchSelect :model-value="String(row.itemId || '')" :options="titleOptions(row)" @update:model-value="(v) => updateAt(index, { itemId: v })" /></label>
        </template>
        <template v-else>
          <label>物品<SearchSelect :model-value="String(row.itemId || '')" :options="itemOptions" @update:model-value="(v) => updateAt(index, { itemId: v })" /></label>
          <label>数量<input type="number" :value="row.quantity ?? 1" @input="updateAt(index, { quantity: asNumber(eventValue($event), 1) })" /></label>
        </template>
        <button class="danger" type="button" @click="removeAt(index)">删除</button>
      </div>
    </div>
  `
});

const AchievementStagesEditor = defineComponent({
  name: 'AchievementStagesEditor',
  components: { AchievementConditionEditor, AchievementRewardsEditor },
  props: {
    modelValue: { type: Array as PropType<Record<string, unknown>[]>, default: () => [] },
    enemyOptions: { type: Array as PropType<string[]>, default: () => [] },
    fishingSpeciesOptions: { type: Array as PropType<string[]>, default: () => [] },
    actionOptions: { type: Array as PropType<string[]>, default: () => [] },
    actionDurationMap: { type: Object as PropType<Record<string, number>>, default: () => ({}) },
    stageProgressMode: { type: String, default: 'cumulative' },
    parentActionId: { type: String, default: '' },
    itemOptions: { type: Array as PropType<string[]>, default: () => [] },
    commissionOptions: { type: Array as PropType<string[]>, default: () => [] },
    attrOptions: { type: Array as PropType<string[]>, default: () => [] },
    rarityOptions: { type: Array as PropType<string[]>, default: () => [] },
    titlePrefixOptions: { type: Array as PropType<string[]>, default: () => [] },
    titleSuffixOptions: { type: Array as PropType<string[]>, default: () => [] }
  },
  emits: ['update:modelValue'],
  setup(props, { emit }) {
    const rows = computed(() => asArray(props.modelValue).filter(isRecord));
    const autoTotal = ref(0);
    const autoMode = ref('geo2');
    const autoDays = ref(0);
    const updateAt = (index: number, patch: Record<string, unknown>) => {
      const next = rows.value.map((row) => ({ ...row }));
      next[index] = { ...next[index], ...patch };
      emit('update:modelValue', next);
    };
    const add = () => emit('update:modelValue', [
      ...rows.value,
      {
        rarity: props.rarityOptions[0] || 'common',
        condition: defaultAchievementCondition('complete_action'),
        rewards: []
      }
    ]);
    const removeAt = (index: number) => emit('update:modelValue', rows.value.filter((_, i) => i !== index));
    const eventValue = (event: Event) => (event.target as HTMLInputElement | HTMLSelectElement).value;

    // Distribution delta-weights by mode (each = incremental work for that stage)
    // Cumulative counts are computed from prefix sums of these deltas.
    const DIST_MODES: Record<string, { label: string; deltas: (n: number) => number[] }> = {
      linear:    { label: '等差递增', deltas: (n) => Array.from({ length: n }, (_, i) => i + 1) },
      geo15:     { label: '1.5倍增',  deltas: (n) => Array.from({ length: n }, (_, i) => Math.pow(1.5, i)) },
      geo2:      { label: '2倍增',    deltas: (n) => Array.from({ length: n }, (_, i) => Math.pow(2, i)) },
      geo3:      { label: '3倍增',    deltas: (n) => Array.from({ length: n }, (_, i) => Math.pow(3, i)) },
      quadratic: { label: '平方递增', deltas: (n) => Array.from({ length: n }, (_, i) => (i + 1) ** 2) },
    };
    const distModeOptions = computed(() => Object.entries(DIST_MODES).map(([k, v]) => ({ value: k, label: v.label })));

    const niceRound = (n: number): number => {
      if (n <= 5) return Math.max(1, Math.round(n));
      if (n <= 50) return Math.round(n);
      if (n <= 500) return Math.round(n / 5) * 5;
      if (n <= 5000) return Math.round(n / 50) * 50;
      if (n <= 50000) return Math.round(n / 500) * 500;
      return Math.round(n / 1000) * 1000;
    };
    // Distribute total across N stages: deltas are per-stage work, output is cumulative counts
    const distribute = (total: number, mode: string): number[] => {
      const n = rows.value.length;
      if (n === 0 || total <= 0) return [];
      const deltaFn = DIST_MODES[mode]?.deltas || DIST_MODES.geo2.deltas;
      const rawDeltas = deltaFn(n);
      const deltaSum = rawDeltas.reduce((a, b) => a + b, 0);
      const base = total / deltaSum;
      // Cumulative counts = prefix sums of (delta_i * base)
      let cumDelta = 0;
      const counts = rawDeltas.map(d => { cumDelta += d; return niceRound(cumDelta * base); });
      // Ensure monotonically increasing
      for (let i = 1; i < counts.length; i++) {
        if (counts[i] <= counts[i - 1]) counts[i] = counts[i - 1] + 1;
      }
      return counts;
    };

    const applyDistribution = () => {
      const total = asNumber(autoTotal.value, 0);
      if (total <= 0 || rows.value.length === 0) return;
      const counts = distribute(total, autoMode.value);
      const next = rows.value.map((row, i) => ({
        ...row,
        condition: isRecord(row.condition) ? { ...row.condition, count: counts[i] } : row.condition
      }));
      emit('update:modelValue', next);
    };

    // Calculate count from days for action achievements
    const parentDuration = computed(() => {
      const aid = asString(props.parentActionId);
      return props.actionDurationMap?.[aid] ?? 0;
    });
    const daysToCount = computed(() => {
      const days = asNumber(autoDays.value, 0);
      const dur = parentDuration.value;
      if (days <= 0 || dur <= 0) return 0;
      return Math.ceil((days * 86400) / dur);
    });
    const applyDaysToCount = () => {
      const count = daysToCount.value;
      if (count <= 0) return;
      autoTotal.value = count;
      applyDistribution();
    };
    const formatSec = (sec: number) => {
      if (sec < 60) return sec + '秒';
      if (sec < 3600) return Math.floor(sec / 60) + '分' + (sec % 60) + '秒';
      if (sec < 86400) return (sec / 3600).toFixed(1) + '小时';
      return (sec / 86400).toFixed(1) + '天';
    };
    const parentDurationLabel = computed(() => {
      const dur = parentDuration.value;
      return dur > 0 ? formatSec(dur) : '';
    });

    const cumulativeSecondsFor = (index: number): number => {
      const row = rows.value[index];
      if (!row || !isRecord(row.condition)) return -1;
      const cond = row.condition;
      const kind = asString(cond.kind);
      const count = asNumber(cond.count, 0);
      if (kind !== 'complete_action') return -1;
      const actionId = asString(cond.actionId);
      const dur = props.actionDurationMap?.[actionId] ?? 0;
      if (dur <= 0 || count <= 0) return -1;
      const isCumulative = props.stageProgressMode !== 'sequential';
      if (isCumulative) return count * dur;
      let prevCount = 0;
      for (let i = index - 1; i >= 0; i--) {
        const prevCond = rows.value[i]?.condition;
        if (isRecord(prevCond) && asString(prevCond.kind) === kind) {
          prevCount = asNumber(prevCond.count, 0);
          break;
        }
      }
      return Math.max(0, (count - prevCount)) * dur;
    };
    return {
      rows, updateAt, add, removeAt, eventValue, asNumber, asArray, isRecord, defaultAchievementCondition,
      cumulativeSecondsFor, autoTotal, autoMode, autoDays, distModeOptions,
      applyDistribution, applyDaysToCount, daysToCount, parentDurationLabel, parentDuration
    };
  },
  template: `
    <div class="array-editor">
      <div class="auto-distribute-panel">
        <div class="auto-distribute-header">智能分布</div>
        <div class="auto-distribute-row">
          <label>总次数<input type="number" v-model.number="autoTotal" min="0" placeholder="输入总次数" /></label>
          <label>递增模式
            <select v-model="autoMode">
              <option v-for="m in distModeOptions" :key="m.value" :value="m.value">{{ m.label }}</option>
            </select>
          </label>
          <button class="ghost primary-btn" type="button" @click="applyDistribution" :disabled="!autoTotal || !rows.length">分布</button>
        </div>
        <div v-if="parentDuration > 0" class="auto-distribute-row">
          <span class="auto-meta">动作单次耗时: <strong>{{ parentDurationLabel }}</strong></span>
          <label>目标天数<input type="number" v-model.number="autoDays" min="0" step="0.5" placeholder="天" /></label>
          <span v-if="daysToCount > 0" class="auto-meta">= <strong>{{ daysToCount.toLocaleString() }}</strong> 次</span>
          <button class="ghost primary-btn" type="button" @click="applyDaysToCount" :disabled="!autoDays || !rows.length">按时间分布</button>
        </div>
      </div>
      <div class="json-toolbar">
        <span>阶段 {{ rows.length }}（每个阶段会展开为独立成就，进度模式: {{ stageProgressMode === 'sequential' ? '顺序' : '累计' }}）</span>
        <div class="toolbar"><button class="ghost" type="button" @click="add">新增阶段</button></div>
      </div>
      <div v-for="(row, index) in rows" :key="index" class="array-card">
        <div class="json-toolbar">
          <strong>阶段 {{ index + 1 }}</strong>
          <button class="danger" type="button" @click="removeAt(index)">删除阶段</button>
        </div>
        <div class="json-row compact">
          <label>名称Key(可选)<input :value="String(row.nameKey || '')" @input="updateAt(index, { nameKey: eventValue($event) })" /></label>
          <label>描述Key(可选)<input :value="String(row.descKey || '')" @input="updateAt(index, { descKey: eventValue($event) })" /></label>
          <label>稀有度
            <select :value="String(row.rarity || '')" @change="updateAt(index, { rarity: eventValue($event) })">
              <option value="">继承</option>
              <option v-for="rarity in rarityOptions" :key="rarity" :value="rarity">{{ rarity }}</option>
            </select>
          </label>
          <label>排序偏移<input type="number" :value="row.order ?? index" @input="updateAt(index, { order: asNumber(eventValue($event), index) })" /></label>
        </div>
        <AchievementConditionEditor
          :model-value="isRecord(row.condition) ? row.condition : defaultAchievementCondition('complete_action')"
          :enemy-options="enemyOptions"
          :fishing-species-options="fishingSpeciesOptions"
          :action-options="actionOptions"
          :action-duration-map="actionDurationMap"
          :cumulative-seconds="cumulativeSecondsFor(index)"
          :item-options="itemOptions"
          :commission-options="commissionOptions"
          :attr-options="attrOptions"
          @update:model-value="(v) => updateAt(index, { condition: v })"
        />
        <AchievementRewardsEditor
          :model-value="asArray(row.rewards).filter(isRecord)"
          :item-options="itemOptions"
          :title-prefix-options="titlePrefixOptions"
          :title-suffix-options="titleSuffixOptions"
          @update:model-value="(v) => updateAt(index, { rewards: v })"
        />
      </div>
    </div>
  `
});

const EnemyEquipmentEditor = defineComponent({
  name: 'EnemyEquipmentEditor',
  components: { JsonFieldEditor, NumberInput, SearchSelect },
  props: {
    modelValue: { type: Array as PropType<Record<string, unknown>[]>, default: () => [] },
    equipmentRows: { type: Array as PropType<DataRow[]>, default: () => [] },
    slots: { type: Array as PropType<string[]>, default: () => [] }
  },
  emits: ['update:modelValue'],
  setup(props, { emit }) {
    const update = (rows: Record<string, unknown>[]) => emit('update:modelValue', rows);
    const templateByID = computed(() => new Map(props.equipmentRows.map((row) => [rowId(row), row])));
    const slotList = computed(() => {
      const slots = new Set<string>(props.slots.map((slot) => asString(slot)).filter(Boolean));
      for (const row of props.equipmentRows) {
        const slot = asString(row.slot);
        if (slot) slots.add(slot);
        for (const occupySlot of asStringList(row.occupySlots)) slots.add(occupySlot);
      }
      return Array.from(slots).sort((a, b) => a.localeCompare(b));
    });
    const matchesSlot = (row: DataRow, slot: string) => {
      const normalizedSlot = asString(slot);
      if (!normalizedSlot) return true;
      const templateSlot = asString(row.slot);
      const occupySlots = asStringList(row.occupySlots);
      if (templateSlot === normalizedSlot || occupySlots.includes(normalizedSlot)) return true;
      if (templateSlot === 'weapon' && ['weapon', 'right_hand', 'left_hand'].includes(normalizedSlot)) return true;
      if (templateSlot === 'accessory' && normalizedSlot.startsWith('accessory')) return true;
      return false;
    };
    const equipmentForSlot = (slot: string) => props.equipmentRows.filter((row) => matchesSlot(row, slot));
    const attackEquipment = computed(() => props.equipmentRows.filter((row) => {
      const slot = asString(row.slot);
      if (slot === 'weapon' || slot === 'right_hand' || slot === 'left_hand') return true;
      if (asArray(row.basicAttackEffects).length > 0) return true;
      return isRecord(row.basicAttackFormula);
    }));
    const defaultSlotForTemplate = (template: DataRow | undefined) => {
      const slot = asString(template?.slot);
      if (slot === 'weapon') return 'right_hand';
      if (slot === 'accessory') return 'accessory_1';
      return slot || slotList.value[0] || 'right_hand';
    };
    const normalizeEntry = (entry: Record<string, unknown>, template: DataRow | undefined, slot = '') => ({
      ...entry,
      templateItemId: rowId(template) || asString(entry.templateItemId),
      equippedSlot: slot || asString(entry.equippedSlot) || defaultSlotForTemplate(template),
      equipType: asString(template?.equipType) || asString(entry.equipType) || 'battle',
      enhanceLevel: Math.max(0, asNumber(entry.enhanceLevel, 0)),
      baseSnapshot: isRecord(entry.baseSnapshot) ? entry.baseSnapshot : {},
      affixes: asArray(entry.affixes)
    });
    const patchAt = (index: number, patch: Record<string, unknown>) => {
      const next = props.modelValue.map((row) => ({ ...row }));
      next[index] = { ...(next[index] ?? {}), ...patch };
      update(next);
    };
    const selectSlot = (index: number, slot: string) => {
      const current = { ...(props.modelValue[index] ?? {}) };
      let template = templateByID.value.get(asString(current.templateItemId));
      if (!template || !matchesSlot(template, slot)) {
        template = equipmentForSlot(slot)[0];
      }
      const normalized = normalizeEntry(current, template, slot);
      patchAt(index, normalized);
    };
    const selectTemplate = (index: number, templateID: string) => {
      const current = { ...(props.modelValue[index] ?? {}) };
      const template = templateByID.value.get(templateID);
      const slot = asString(current.equippedSlot) || defaultSlotForTemplate(template);
      patchAt(index, normalizeEntry(current, template, slot));
    };
    const add = () => {
      const slot = slotList.value[0] || 'right_hand';
      const template = equipmentForSlot(slot)[0] ?? props.equipmentRows[0];
      update([...props.modelValue, normalizeEntry({}, template, slot)]);
    };
    const addWeapon = () => {
      const template = attackEquipment.value[0] ?? equipmentForSlot('right_hand')[0] ?? props.equipmentRows[0];
      update([...props.modelValue, normalizeEntry({}, template, defaultSlotForTemplate(template) || 'right_hand')]);
    };
    const removeAt = (index: number) => {
      const next = [...props.modelValue];
      next.splice(index, 1);
      update(next);
    };
    const jump = (templateID: string) => {
      if (!templateID) return;
      jumpToRow('equipment_templates', templateID);
    };
    const equipmentLabel = (row: DataRow) => {
      const id = rowId(row);
      const slot = asString(row.slot);
      const weaponClass = asStringList(row.weaponClass).join('/');
      const equipmentSecondaryClass = asString(row.equipmentSecondaryClass);
      const secondaryLabel = equipmentSecondaryClass
        ? localizeKey(`equipment_secondary_class.${equipmentSecondaryClass}.name`, equipmentSecondaryClass)
        : '';
      const suffix = [slot && optionLabel(slot), weaponClass, secondaryLabel].filter(Boolean).join(' · ');
      return `${displayName(row)} (${id})${suffix ? ` - ${suffix}` : ''}`;
    };
    return { EQUIP_TYPES, optionLabel, rowId, asString, asArray, slotList, equipmentForSlot, selectSlot, selectTemplate, add, addWeapon, removeAt, patchAt, jump, equipmentLabel };
  },
  template: `
    <div class="enemy-equipment-editor">
      <article class="nested-card" v-for="(entry, index) in modelValue" :key="index">
        <div class="nested-head">
          <strong>#{{ index + 1 }}</strong>
          <button class="danger" @click="removeAt(index)">删除</button>
        </div>
        <div class="form-grid cols-2">
          <label>部位
            <SearchSelect :model-value="asString(entry.equippedSlot)" :options="slotList" @update:model-value="(value) => selectSlot(index, value)" />
          </label>
          <label>装备
            <div class="linked-control">
              <SearchSelect :model-value="asString(entry.templateItemId)" :options="equipmentForSlot(asString(entry.equippedSlot)).map((row) => rowId(row))" @update:model-value="(value) => selectTemplate(index, value)" />
              <button v-if="entry.templateItemId" type="button" class="link-button" @click="jump(asString(entry.templateItemId))">装备</button>
            </div>
          </label>
          <label>装备类型
            <SearchSelect :model-value="asString(entry.equipType) || 'battle'" :options="EQUIP_TYPES" @update:model-value="(value) => patchAt(index, { equipType: value })" />
          </label>
          <label>强化等级<NumberInput :model-value="Number(entry.enhanceLevel || 0)" @update:model-value="(value) => patchAt(index, { enhanceLevel: value })" /></label>
        </div>
        <details class="advanced-section compact">
          <summary>高级装备实例字段</summary>
          <label class="wide">快照属性<JsonFieldEditor :model-value="entry.baseSnapshot || {}" :path="['equipment', String(index), 'baseSnapshot']" @update:model-value="(value) => patchAt(index, { baseSnapshot: value })" /></label>
          <label class="wide">词条<JsonFieldEditor :model-value="asArray(entry.affixes)" :path="['equipment', String(index), 'affixes']" @update:model-value="(value) => patchAt(index, { affixes: value })" /></label>
        </details>
      </article>
      <div class="toolbar">
        <button class="secondary" @click="addWeapon">新增武器/攻击</button>
        <button class="secondary" @click="add">新增装备</button>
      </div>
    </div>
  `
});

const EnemySkillEditor = defineComponent({
  name: 'EnemySkillEditor',
  components: { SearchSelect },
  props: {
    modelValue: { type: Array as PropType<string[]>, default: () => [] },
    options: { type: Array as PropType<string[]>, default: () => [] }
  },
  emits: ['update:modelValue'],
  setup(props, { emit }) {
    const normalizedRows = (rows: string[]) => rows.map((item) => String(item || '').trim()).filter(Boolean);
    const update = (rows: string[]) => emit('update:modelValue', normalizedRows(rows));
    const updateAt = (index: number, value: string) => {
      const next = [...props.modelValue];
      next[index] = value;
      update(next);
    };
    const addSkill = () => {
      const nextValue = props.options.find((option) => option !== 'basic_attack' && !props.modelValue.includes(option)) ?? props.options.find((option) => !props.modelValue.includes(option)) ?? '';
      update([...props.modelValue, nextValue]);
    };
    const addBasicAttack = () => {
      if (props.modelValue.includes('basic_attack')) return;
      update([...props.modelValue, 'basic_attack']);
    };
    const removeAt = (index: number) => {
      const next = [...props.modelValue];
      next.splice(index, 1);
      update(next);
    };
    const jump = (skillID: string) => {
      if (!skillID) return;
      jumpToRow('skills', skillID);
    };
    return { updateAt, addSkill, addBasicAttack, removeAt, jump };
  },
  template: `
    <div class="array-editor">
      <div class="json-row compact" v-for="(item, index) in modelValue" :key="index">
        <div class="linked-control">
          <SearchSelect :model-value="item" :options="options" @update:model-value="(value) => updateAt(index, value)" />
          <button v-if="item" type="button" class="link-button" @click="jump(String(item))">技能</button>
        </div>
        <button class="danger" @click="removeAt(index)">删</button>
      </div>
      <div class="toolbar">
        <button class="secondary" @click="addBasicAttack">新增普攻</button>
        <button class="secondary" @click="addSkill">新增技能</button>
      </div>
    </div>
  `
});

const EnemyDropEditor = defineComponent({
  name: 'EnemyDropEditor',
  components: { NumberInput, SearchSelect },
  props: {
    modelValue: { type: Object as PropType<Record<string, unknown>>, default: () => ({}) },
    itemOptions: { type: Array as PropType<string[]>, default: () => [] }
  },
  emits: ['update:modelValue'],
  setup(props, { emit }) {
    const entries = computed(() => Object.entries(props.modelValue ?? {}));
    const defaultDrop = () => ({ percent: 0.1, range: { min: 1, max: 1 } });
    const dropNode = (value: unknown) => {
      const node = isRecord(value) ? value : {};
      const range = isRecord(node.range) ? node.range : {};
      return {
        percent: Math.max(0, Math.min(1, asNumber(node.percent, 0.1))),
        range: {
          min: Math.max(0, asNumber(range.min, 1)),
          max: Math.max(0, asNumber(range.max, 1))
        }
      };
    };
    const update = (next: Record<string, unknown>) => emit('update:modelValue', next);
    const updateItem = (oldID: string, nextID: string) => {
      const normalized = String(nextID || '').trim();
      if (!normalized) return;
      const next = { ...(props.modelValue ?? {}) };
      const value = next[oldID] ?? defaultDrop();
      delete next[oldID];
      next[normalized] = value;
      update(next);
    };
    const patchDrop = (itemID: string, patch: Record<string, unknown>) => {
      const current = dropNode((props.modelValue ?? {})[itemID]);
      update({ ...(props.modelValue ?? {}), [itemID]: { ...current, ...patch } });
    };
    const patchRange = (itemID: string, patch: Record<string, unknown>) => {
      const current = dropNode((props.modelValue ?? {})[itemID]);
      patchDrop(itemID, { range: { ...current.range, ...patch } });
    };
    const add = () => {
      const itemID = props.itemOptions.find((option) => !Object.prototype.hasOwnProperty.call(props.modelValue ?? {}, option)) ?? '';
      if (!itemID) return;
      update({ ...(props.modelValue ?? {}), [itemID]: defaultDrop() });
    };
    const remove = (itemID: string) => {
      const next = { ...(props.modelValue ?? {}) };
      delete next[itemID];
      update(next);
    };
    const jump = (itemID: string) => {
      if (!itemID) return;
      jumpToRow('items', itemID);
    };
    return { entries, dropNode, updateItem, patchDrop, patchRange, add, remove, jump, itemOptions: props.itemOptions };
  },
  template: `
    <div class="enemy-drop-editor">
      <article class="nested-card" v-for="[itemID, rawDrop] in entries" :key="itemID">
        <div class="nested-head">
          <div class="linked-control">
            <SearchSelect :model-value="itemID" :options="itemOptions" @update:model-value="(value) => updateItem(itemID, value)" />
              <button type="button" class="link-button" @click="jump(itemID)">引用</button>
          </div>
          <button class="danger" @click="remove(itemID)">删除</button>
        </div>
        <div class="form-grid cols-2">
          <label>掉率(0~1)<NumberInput :model-value="dropNode(rawDrop).percent" @update:model-value="(value) => patchDrop(itemID, { percent: Math.max(0, Math.min(1, value)) })" /></label>
          <label>最小数量<NumberInput :model-value="dropNode(rawDrop).range.min" @update:model-value="(value) => patchRange(itemID, { min: Math.max(0, Math.floor(value)) })" /></label>
          <label>最大数量<NumberInput :model-value="dropNode(rawDrop).range.max" @update:model-value="(value) => patchRange(itemID, { max: Math.max(dropNode(rawDrop).range.min, Math.floor(value)) })" /></label>
        </div>
      </article>
      <button class="secondary" @click="add">新增掉落</button>
    </div>
  `
});

const StringListEditor = defineComponent({
  name: 'StringListEditor',
  components: { SearchSelect },
  props: {
    modelValue: { type: Array<string>, default: () => [] },
    options: { type: Array<string>, default: () => [] },
    linkTable: { type: String, default: '' },
    linkLabel: { type: String, default: '跳转' }
  },
  emits: ['update:modelValue'],
  setup(props, { emit }) {
    const eventValue = (event: Event) => (event.target as HTMLInputElement).value;
    const updateAt = (index: number, value: string) => {
      const next = [...props.modelValue];
      next[index] = value;
      emit('update:modelValue', next.map((item) => String(item).trim()).filter(Boolean));
    };
    const removeAt = (index: number) => {
      const next = [...props.modelValue];
      next.splice(index, 1);
      emit('update:modelValue', next);
    };
    const add = () => {
      const nextValue = props.options.find((option) => !props.modelValue.includes(option)) ?? '';
      emit('update:modelValue', [...props.modelValue, nextValue].filter((item, index, list) => item || index === list.length - 1));
    };
    const jump = (id: string) => {
      if (!props.linkTable || !id) return;
      jumpToRow(props.linkTable as EditableTable, id);
    };
    return { updateAt, removeAt, add, jump, eventValue, optionLabel, TABLE_LABELS };
  },
  template: `
    <div class="array-editor">
      <div class="json-row compact" v-for="(item, index) in modelValue" :key="index">
        <div v-if="options.length" class="linked-control">
          <SearchSelect :model-value="item" :options="options" @update:model-value="(value) => updateAt(index, value)" />
          <button v-if="linkTable && item" type="button" class="link-button" :title="'跳转到' + TABLE_LABELS[linkTable]" @click="jump(String(item))">{{ linkLabel }}</button>
        </div>
        <input v-else :value="item" @input="updateAt(index, eventValue($event))" />
        <button class="danger" @click="removeAt(index)">删</button>
      </div>
      <button class="secondary" @click="add">新增</button>
    </div>
  `
});

const NumberMapEditor = defineComponent({
  name: 'NumberMapEditor',
  components: { NumberInput, SearchSelect },
  props: {
    modelValue: { type: Object, default: () => ({}) },
    options: { type: Array<string>, default: () => [] },
    linkTable: { type: String, default: '' },
    linkLabel: { type: String, default: '跳转' },
    unitValueMap: { type: Object as PropType<Record<string, number>>, default: () => ({}) },
    showValueMeta: { type: Boolean, default: false }
  },
  emits: ['update:modelValue'],
  setup(props, { emit }) {
    const eventValue = (event: Event) => (event.target as HTMLInputElement).value;
    const entries = computed(() => Object.entries(props.modelValue ?? {}));
    const updateKey = (oldKey: string, key: string) => {
      const next = { ...(props.modelValue as Record<string, unknown>) };
      const value = next[oldKey];
      delete next[oldKey];
      if (key) next[key] = value;
      emit('update:modelValue', next);
    };
    const updateValue = (key: string, value: number) => emit('update:modelValue', { ...(props.modelValue as Record<string, unknown>), [key]: value });
    const remove = (key: string) => {
      const next = { ...(props.modelValue as Record<string, unknown>) };
      delete next[key];
      emit('update:modelValue', next);
    };
    const add = () => {
      const key = props.options.find((option) => !(option in (props.modelValue as Record<string, unknown>))) ?? '';
      emit('update:modelValue', { ...(props.modelValue as Record<string, unknown>), [key]: 0 });
    };
    const jump = (id: string) => {
      if (!props.linkTable || !id) return;
      jumpToRow(props.linkTable as EditableTable, id);
    };
    const valueMetaText = (key: string, value: unknown) => {
      if (!props.showValueMeta || !key) return '';
      const unit = Math.max(0, asNumber((props.unitValueMap as Record<string, unknown>)?.[key], 0));
      const total = Math.max(0, asNumber(value, 0)) * unit;
      return `单价 ${formatGoldDisplay(unit)} / 总价 ${formatGoldDisplay(total)}`;
    };
    return { entries, updateKey, updateValue, remove, add, jump, eventValue, TABLE_LABELS, valueMetaText };
  },
  template: `
    <div class="array-editor">
      <div :class="['json-row', 'compact', 'number-map-row', { 'with-meta': showValueMeta }]" v-for="[key, value] in entries" :key="key">
        <div v-if="options.length" class="linked-control">
          <SearchSelect :model-value="key" :options="options" @update:model-value="(value) => updateKey(key, value)" />
          <button v-if="linkTable && key" type="button" class="link-button" :title="'跳转到' + TABLE_LABELS[linkTable]" @click="jump(String(key))">{{ linkLabel }}</button>
        </div>
        <input v-else :value="key" @input="updateKey(key, eventValue($event))" />
        <NumberInput :model-value="value" @update:model-value="(next) => updateValue(key, next)" />
        <span v-if="showValueMeta && key" class="number-map-value-meta">{{ valueMetaText(String(key), value) }}</span>
        <button class="danger" @click="remove(key)">删</button>
      </div>
      <button class="secondary" @click="add">新增</button>
    </div>
  `
});

const RangeMapEditor = defineComponent({
  name: 'RangeMapEditor',
  components: { NumberInput, SearchSelect },
  props: {
    modelValue: { type: Object, default: () => ({}) },
    options: { type: Array as PropType<string[]>, default: () => [] }
  },
  emits: ['update:modelValue'],
  setup(props, { emit }) {
    const entries = computed(() => Object.entries(props.modelValue ?? {}).map(([k, v]) => {
      const range = (isRecord(v) ? v : {}) as Record<string, unknown>;
      return { key: k, min: asNumber(range.min, 0), max: asNumber(range.max, 0) };
    }));
    const updateKey = (oldKey: string, key: string) => {
      const next: Record<string, unknown> = { ...(props.modelValue as Record<string, unknown>) };
      const value = next[oldKey];
      delete next[oldKey];
      if (key) next[key] = value;
      emit('update:modelValue', next);
    };
    const updateRange = (key: string, field: 'min' | 'max', value: number) => {
      const current = isRecord((props.modelValue as Record<string, unknown>)[key])
        ? { ...(props.modelValue as Record<string, Record<string, unknown>>)[key] }
        : { min: 0, max: 0 };
      emit('update:modelValue', { ...(props.modelValue as Record<string, unknown>), [key]: { ...current, [field]: value } });
    };
    const remove = (key: string) => {
      const next: Record<string, unknown> = { ...(props.modelValue as Record<string, unknown>) };
      delete next[key];
      emit('update:modelValue', next);
    };
    const add = () => {
      const key = props.options.find((opt) => !(opt in (props.modelValue as Record<string, unknown>))) ?? '';
      emit('update:modelValue', { ...(props.modelValue as Record<string, unknown>), [key]: { min: 1, max: 1 } });
    };
    return { entries, updateKey, updateRange, remove, add };
  },
  template: `
    <div class="array-editor">
      <div class="json-row compact range-map-row" v-for="entry in entries" :key="entry.key">
        <div v-if="options.length" class="linked-control" style="min-width:140px">
          <SearchSelect :model-value="entry.key" :options="options" @update:model-value="(v) => updateKey(entry.key, v)" />
        </div>
        <input v-else :value="entry.key" @input="updateKey(entry.key, $event.target.value)" style="min-width:120px" />
        <label class="range-label">最低</label>
        <NumberInput :model-value="entry.min" @update:model-value="(v) => updateRange(entry.key, 'min', v)" style="width:72px" />
        <label class="range-label">最高</label>
        <NumberInput :model-value="entry.max" @update:model-value="(v) => updateRange(entry.key, 'max', v)" style="width:72px" />
        <button class="danger" @click="remove(entry.key)">删</button>
      </div>
      <button class="secondary" @click="add">新增</button>
    </div>
  `
});

const RangeMapOptionsEditor = defineComponent({
  name: 'RangeMapOptionsEditor',
  components: { RangeMapEditor },
  props: {
    modelValue: { type: Array, default: () => [] },
    options: { type: Array as PropType<string[]>, default: () => [] }
  },
  emits: ['update:modelValue'],
  setup(props, { emit }) {
    const groups = computed(() => asArray(props.modelValue).filter(isRecord) as Record<string, unknown>[]);
    const updateGroup = (index: number, value: Record<string, unknown>) => {
      const next = [...groups.value];
      next[index] = value;
      emit('update:modelValue', next);
    };
    const addGroup = () => emit('update:modelValue', [...groups.value, {}]);
    const removeGroup = (index: number) => {
      const next = [...groups.value];
      next.splice(index, 1);
      emit('update:modelValue', next);
    };
    return { groups, updateGroup, addGroup, removeGroup };
  },
  template: `
    <div class="array-editor">
      <div v-for="(group, index) in groups" :key="index" class="range-map-group">
        <div class="range-map-group-head">
          <span class="range-map-group-label">方案 {{ index + 1 }}</span>
          <button class="danger" style="padding:2px 8px;font-size:11px" @click="removeGroup(index)">删组</button>
        </div>
        <RangeMapEditor :model-value="group" :options="options" @update:model-value="(v) => updateGroup(index, v)" />
      </div>
      <button class="secondary" @click="addGroup">+ 新增方案</button>
    </div>
  `
});

const EnhanceCurveEditor = defineComponent({
  name: 'EnhanceCurveEditor',
  components: { NumberMapEditor, NumberInput },
  props: {
    modelValue: { type: Object as PropType<Record<string, unknown>>, default: () => ({}) },
    options: { type: Array as PropType<string[]>, default: () => [] },
    linkTable: { type: String, default: '' },
    valueLabel: { type: String, default: '数值' }
  },
  emits: ['update:modelValue'],
  setup(props, { emit }) {
    const mapNode = (value: unknown) => (isRecord(value) ? { ...value } : {});
    const overridesNode = computed(() => (isRecord(props.modelValue?.overrides) ? props.modelValue?.overrides as Record<string, unknown> : {}));
    const sortedOverrideLevels = computed(() => Object.keys(overridesNode.value).sort((a, b) => asNumber(a, 0) - asNumber(b, 0)));
    const patch = (patchNode: Record<string, unknown>) => {
      emit('update:modelValue', {
        base: mapNode(props.modelValue?.base),
        step: mapNode(props.modelValue?.step),
        overrides: { ...overridesNode.value },
        ...patchNode
      });
    };
    const updateBase = (value: Record<string, unknown>) => patch({ base: value });
    const updateStep = (value: Record<string, unknown>) => patch({ step: value });
    const updateOverride = (level: string, value: Record<string, unknown>) => {
      const next = { ...overridesNode.value, [level]: value };
      patch({ overrides: next });
    };
    const renameLevel = (oldLevel: string, nextLevel: number) => {
      const normalized = String(Math.max(1, Math.floor(asNumber(nextLevel, 1))));
      const next = { ...overridesNode.value };
      const value = next[oldLevel] ?? {};
      delete next[oldLevel];
      next[normalized] = isRecord(value) ? value : {};
      patch({ overrides: next });
    };
    const removeOverride = (level: string) => {
      const next = { ...overridesNode.value };
      delete next[level];
      patch({ overrides: next });
    };
    const addOverride = () => {
      const levels = sortedOverrideLevels.value.map((level) => asNumber(level, 0));
      const nextLevel = String(Math.max(1, (levels[levels.length - 1] ?? 0) + 1));
      if (Object.prototype.hasOwnProperty.call(overridesNode.value, nextLevel)) return;
      patch({ overrides: { ...overridesNode.value, [nextLevel]: {} } });
    };
    return { mapNode, overridesNode, sortedOverrideLevels, updateBase, updateStep, updateOverride, renameLevel, removeOverride, addOverride };
  },
  template: `
    <div class="buff-rule-editor">
      <article class="nested-card">
        <div class="nested-head"><strong>基础{{ valueLabel }}</strong></div>
        <NumberMapEditor :model-value="mapNode(modelValue.base)" :options="options" :link-table="linkTable" :link-label="linkTable === 'items' ? '物品' : '属性'" @update:model-value="updateBase" />
      </article>
      <article class="nested-card">
        <div class="nested-head"><strong>每级增量</strong></div>
        <NumberMapEditor :model-value="mapNode(modelValue.step)" :options="options" :link-table="linkTable" :link-label="linkTable === 'items' ? '物品' : '属性'" @update:model-value="updateStep" />
      </article>
      <article class="nested-card" v-for="level in sortedOverrideLevels" :key="level">
        <div class="nested-head">
          <strong>等级覆盖</strong>
          <button class="danger" @click="removeOverride(level)">删除</button>
        </div>
        <div class="form-grid cols-2">
          <label>等级<NumberInput :model-value="Number(level)" @update:model-value="(value) => renameLevel(level, value)" /></label>
          <label class="wide">{{ valueLabel }}<NumberMapEditor :model-value="mapNode(overridesNode[level])" :options="options" :link-table="linkTable" :link-label="linkTable === 'items' ? '物品' : '属性'" @update:model-value="(value) => updateOverride(level, value)" /></label>
        </div>
      </article>
      <button class="secondary" @click="addOverride">新增等级覆盖</button>
    </div>
  `
});

const DefaultAffixSlotsEditor = defineComponent({
  name: 'DefaultAffixSlotsEditor',
  components: { SearchSelect, NumberInput },
  props: {
    modelValue: { type: Array as PropType<unknown[]>, default: () => [] },
    affixOptions: { type: Array as PropType<string[]>, default: () => [] },
    rarityOptions: { type: Array as PropType<string[]>, default: () => [] },
    slotCount: { type: Number, default: 0 }
  },
  emits: ['update:modelValue'],
  setup(props, { emit }) {
    const rows = computed(() => asArray(props.modelValue).filter(isRecord) as DataRow[]);
    const normalizedRarities = computed(() => props.rarityOptions.length > 0 ? props.rarityOptions : ['common', 'uncommon', 'rare', 'epic', 'legendary']);
    const patchAt = (index: number, patch: DataRow) => {
      const next = rows.value.map((row) => ({ ...row }));
      next[index] = { ...next[index], ...patch };
      emit('update:modelValue', next);
    };
    const add = () => {
      const used = new Set(rows.value.map((row) => Math.trunc(asNumber(row.slotIndex, -1))));
      let slotIndex = 0;
      while (used.has(slotIndex) && (props.slotCount <= 0 || slotIndex < props.slotCount)) slotIndex += 1;
      emit('update:modelValue', [
        ...rows.value,
        {
          slotIndex,
          affixId: props.affixOptions[0] ?? '',
          rarityId: normalizedRarities.value.includes('rare') ? 'rare' : normalizedRarities.value[0] ?? '',
          rollValue: 1
        }
      ]);
    };
    const removeAt = (index: number) => {
      const next = rows.value.map((row) => ({ ...row }));
      next.splice(index, 1);
      emit('update:modelValue', next);
    };
    return { rows, normalizedRarities, patchAt, add, removeAt };
  },
  template: `
    <div class="buff-rule-editor">
      <article class="nested-card" v-for="(slot, index) in rows" :key="index">
        <div class="nested-head">
          <strong>默认槽 #{{ index + 1 }}</strong>
          <button class="danger" @click="removeAt(index)">删除</button>
        </div>
        <div class="form-grid cols-2">
          <label>槽位序号<NumberInput :model-value="Number(slot.slotIndex ?? index)" @update:model-value="(value) => patchAt(index, { slotIndex: Math.max(0, Math.trunc(Number(value) || 0)) })" /></label>
          <label>词条<SearchSelect :model-value="String(slot.affixId || '')" :options="affixOptions" :allow-empty="false" @update:model-value="(value) => patchAt(index, { affixId: value })" /></label>
          <label>稀有度<SearchSelect :model-value="String(slot.rarityId || '')" :options="normalizedRarities" :allow-empty="false" @update:model-value="(value) => patchAt(index, { rarityId: value })" /></label>
          <label>rollValue<NumberInput :model-value="Number(slot.rollValue ?? 1)" @update:model-value="(value) => patchAt(index, { rollValue: value })" /></label>
        </div>
      </article>
      <button class="secondary" @click="add">新增默认词条槽</button>
      <small class="dim" v-if="slotCount > 0">词条槽上限：{{ slotCount }}</small>
    </div>
  `
});

const EnhanceAffixMilestonePoolsEditor = defineComponent({
  name: 'EnhanceAffixMilestonePoolsEditor',
  components: { SearchSelect, NumberInput },
  props: {
    modelValue: { type: Object as PropType<Record<string, unknown>>, default: () => ({}) },
    affixOptions: { type: Array as PropType<string[]>, default: () => [] },
    rarityOptions: { type: Array as PropType<string[]>, default: () => [] },
    maxLevel: { type: Number, default: 0 }
  },
  emits: ['update:modelValue'],
  setup(props, { emit }) {
    const normalizedRarities = computed(() => props.rarityOptions.length > 0 ? props.rarityOptions : ['common', 'uncommon', 'rare', 'epic', 'legendary']);
    const rows = computed(() => Object.entries(isRecord(props.modelValue) ? props.modelValue : {})
      .map(([level, pool]) => ({ level, pool: asArray(pool).filter(isRecord) as DataRow[] }))
      .sort((a, b) => asNumber(a.level, 0) - asNumber(b.level, 0)));
    const rebuild = (nextRows: Array<{ level: string; pool: DataRow[] }>) => {
      const next: Record<string, unknown> = {};
      for (const row of nextRows) {
        const level = String(Math.max(1, Math.trunc(asNumber(row.level, 1))));
        next[level] = row.pool.map((entry) => ({ ...entry }));
      }
      emit('update:modelValue', next);
    };
    const renameLevel = (index: number, value: number) => {
      const nextRows = rows.value.map((row) => ({ level: row.level, pool: row.pool.map((entry) => ({ ...entry })) }));
      nextRows[index].level = String(Math.max(1, Math.trunc(Number(value) || 1)));
      rebuild(nextRows);
    };
    const addLevel = () => {
      const used = new Set(rows.value.map((row) => Math.trunc(asNumber(row.level, 0))));
      const preferred = [10, 20, 30, 40].find((level) => !used.has(level) && (props.maxLevel <= 0 || level <= props.maxLevel));
      const nextLevel = preferred ?? Math.max(1, (rows.value.map((row) => Math.trunc(asNumber(row.level, 0))).pop() ?? 0) + 1);
      rebuild([...rows.value, { level: String(nextLevel), pool: [] }]);
    };
    const removeLevel = (index: number) => {
      const nextRows = rows.value.map((row) => ({ level: row.level, pool: row.pool.map((entry) => ({ ...entry })) }));
      nextRows.splice(index, 1);
      rebuild(nextRows);
    };
    const patchEntry = (levelIndex: number, entryIndex: number, patch: DataRow) => {
      const nextRows = rows.value.map((row) => ({ level: row.level, pool: row.pool.map((entry) => ({ ...entry })) }));
      nextRows[levelIndex].pool[entryIndex] = { ...nextRows[levelIndex].pool[entryIndex], ...patch };
      rebuild(nextRows);
    };
    const addEntry = (levelIndex: number) => {
      const nextRows = rows.value.map((row) => ({ level: row.level, pool: row.pool.map((entry) => ({ ...entry })) }));
      nextRows[levelIndex].pool.push({
        affixId: props.affixOptions[0] ?? '',
        rarityId: normalizedRarities.value.includes('rare') ? 'rare' : normalizedRarities.value[0] ?? '',
        rollValue: 1
      });
      rebuild(nextRows);
    };
    const removeEntry = (levelIndex: number, entryIndex: number) => {
      const nextRows = rows.value.map((row) => ({ level: row.level, pool: row.pool.map((entry) => ({ ...entry })) }));
      nextRows[levelIndex].pool.splice(entryIndex, 1);
      rebuild(nextRows);
    };
    return { rows, normalizedRarities, renameLevel, addLevel, removeLevel, patchEntry, addEntry, removeEntry };
  },
  template: `
    <div class="buff-rule-editor">
      <article class="nested-card" v-for="(milestone, levelIndex) in rows" :key="milestone.level">
        <div class="nested-head">
          <strong>里程碑 {{ milestone.level }}</strong>
          <button class="danger" @click="removeLevel(levelIndex)">删除等级</button>
        </div>
        <div class="form-grid cols-2">
          <label>强化等级<NumberInput :model-value="Number(milestone.level)" @update:model-value="(value) => renameLevel(levelIndex, value)" /></label>
          <small class="dim" v-if="maxLevel > 0">强化上限：{{ maxLevel }}</small>
        </div>
        <div class="json-row compact" v-for="(entry, entryIndex) in milestone.pool" :key="entryIndex">
          <SearchSelect :model-value="String(entry.affixId || '')" :options="affixOptions" :allow-empty="false" @update:model-value="(value) => patchEntry(levelIndex, entryIndex, { affixId: value })" />
          <SearchSelect :model-value="String(entry.rarityId || '')" :options="normalizedRarities" :allow-empty="false" @update:model-value="(value) => patchEntry(levelIndex, entryIndex, { rarityId: value })" />
          <NumberInput :model-value="Number(entry.rollValue ?? 1)" @update:model-value="(value) => patchEntry(levelIndex, entryIndex, { rollValue: value })" />
          <button class="danger" @click="removeEntry(levelIndex, entryIndex)">删</button>
        </div>
        <button class="secondary" @click="addEntry(levelIndex)">新增候选词条</button>
      </article>
      <button class="secondary" @click="addLevel">新增里程碑等级</button>
    </div>
  `
});

const FormulaEditor = defineComponent({
  name: 'FormulaEditor',
  components: { JsonFieldEditor, NumberInput, SearchSelect },
  props: {
    modelValue: { type: Object, default: () => defaultFormula() },
    attrOptions: { type: Array<string>, default: () => [] },
    allowEnhanceScaling: { type: Boolean, default: false }
  },
  emits: ['update:modelValue'],
  setup(props, { emit }) {
    const formula = computed(() => (isRecord(props.modelValue) ? props.modelValue : defaultFormula()));
    const terms = computed(() => asArray(formula.value.terms));
    const baseAddTemplate = computed(() => {
      const baseAdd = formula.value.baseAdd;
      return isRecord(baseAdd) && ('alchemyValueCoeff' in baseAdd || 'alchemyMultiplierCoeff' in baseAdd) ? baseAdd : null;
    });
    const baseAddTemplateText = computed(() => {
      const node = baseAddTemplate.value;
      if (!node) return '';
      const source = 'alchemyMultiplierCoeff' in node ? 'comboMultiplier' : 'comboPower';
      const coeff = asNumber(firstDefined(node.alchemyValueCoeff, node.alchemyMultiplierCoeff), 0);
      return `结果 = ${asNumber(node.base, 0)} + ${source} x ${coeff}`;
    });
    const update = (patch: Record<string, unknown>) => emit('update:modelValue', { ...formula.value, ...patch });
    const updateTerm = (index: number, patch: Record<string, unknown>) => {
      const next = terms.value.map((term) => (isRecord(term) ? { ...term } : {}));
      next[index] = { ...next[index], ...patch };
      update({ terms: next });
    };
    const addTerm = () => update({ terms: [...terms.value, { attrKey: props.attrOptions[0] ?? '', coefficient: 1, perSkillLevel: 0, ...(props.allowEnhanceScaling ? { perEnhanceLevel: 0 } : {}) }] });
    const removeTerm = (index: number) => {
      const next = [...terms.value];
      next.splice(index, 1);
      update({ terms: next });
    };
    return { formula, terms, baseAddTemplate, baseAddTemplateText, update, updateTerm, addTerm, removeTerm };
  },
  template: `
    <div class="formula-editor">
      <label v-if="!baseAddTemplate">基础值<NumberInput :model-value="formula.baseAdd ?? 0" @update:model-value="(value) => update({ baseAdd: value })" /></label>
      <div v-else class="formula-template-lock">
        <span>基础值由组合强度模板接管</span>
        <code>{{ baseAddTemplateText }}</code>
      </div>
      <div class="json-row compact" v-for="(term, index) in terms" :key="index">
        <SearchSelect :model-value="String(term.attrKey ?? '')" :options="attrOptions" @update:model-value="(value) => updateTerm(index, { attrKey: value })" />
        <NumberInput :model-value="term.coefficient ?? 0" @update:model-value="(value) => updateTerm(index, { coefficient: value })" />
        <NumberInput :model-value="term.perSkillLevel ?? 0" @update:model-value="(value) => updateTerm(index, { perSkillLevel: value })" />
        <NumberInput v-if="allowEnhanceScaling" title="每强化等级追加到该属性系数" :model-value="term.perEnhanceLevel ?? 0" @update:model-value="(value) => updateTerm(index, { perEnhanceLevel: value })" />
        <button class="danger" @click="removeTerm(index)">删</button>
      </div>
      <button class="secondary" @click="addTerm">新增系数</button>
    </div>
  `
});

const SummonStatsFormulaEditor = defineComponent({
  name: 'SummonStatsFormulaEditor',
  components: { FormulaEditor, SearchSelect },
  props: {
    modelValue: { type: Object as PropType<Record<string, unknown>>, default: () => ({}) },
    attrOptions: { type: Array<string>, default: () => [] },
    formulaAttrOptions: { type: Array<string>, default: () => [] }
  },
  emits: ['update:modelValue'],
  setup(props, { emit }) {
    const rows = computed(() => {
      const source = isRecord(props.modelValue) ? props.modelValue : {};
      return Object.entries(source).map(([attrKey, formula]) => ({
        attrKey,
        formula: isRecord(formula) ? formula : defaultFormula()
      }));
    });
    const rebuild = (nextRows: Array<{ attrKey: string; formula: Record<string, unknown> }>) => {
      const next: Record<string, unknown> = {};
      for (const row of nextRows) {
        const attrKey = asString(row.attrKey).trim();
        if (!attrKey) continue;
        next[attrKey] = isRecord(row.formula) ? row.formula : defaultFormula();
      }
      emit('update:modelValue', next);
    };
    const patchRow = (index: number, patch: Partial<{ attrKey: string; formula: Record<string, unknown> }>) => {
      const nextRows = rows.value.map((row) => ({ ...row, formula: isRecord(row.formula) ? { ...row.formula } : defaultFormula() }));
      nextRows[index] = { ...nextRows[index], ...patch } as { attrKey: string; formula: Record<string, unknown> };
      rebuild(nextRows);
    };
    const add = () => {
      const nextRows = rows.value.map((row) => ({ ...row, formula: isRecord(row.formula) ? { ...row.formula } : defaultFormula() }));
      const used = new Set(nextRows.map((row) => row.attrKey));
      const fallback = props.attrOptions.find((attr) => !used.has(attr)) ?? props.attrOptions[0] ?? '';
      nextRows.push({ attrKey: fallback, formula: defaultFormula() });
      rebuild(nextRows);
    };
    const removeAt = (index: number) => {
      const nextRows = rows.value.map((row) => ({ ...row, formula: isRecord(row.formula) ? { ...row.formula } : defaultFormula() }));
      nextRows.splice(index, 1);
      rebuild(nextRows);
    };
    return { rows, patchRow, add, removeAt };
  },
  template: `
    <div class="effects-editor">
      <article class="nested-card" v-for="(row, index) in rows" :key="index">
        <div class="nested-head">
          <strong>属性公式 #{{ index + 1 }}</strong>
          <button class="danger" @click="removeAt(index)">删除</button>
        </div>
        <div class="form-grid cols-2">
          <label>属性<SearchSelect :model-value="String(row.attrKey || '')" :options="attrOptions" :allow-empty="false" @update:model-value="(value) => patchRow(index, { attrKey: value })" /></label>
          <label class="wide">公式<FormulaEditor :model-value="row.formula || { baseAdd: 0, terms: [] }" :attr-options="formulaAttrOptions" @update:model-value="(value) => patchRow(index, { formula: value })" /></label>
        </div>
      </article>
      <button class="secondary" @click="add">新增属性公式</button>
    </div>
  `
});

const TargetPredicatesEditor = defineComponent({
  name: 'TargetPredicatesEditor',
  components: { SearchSelect },
  props: {
    modelValue: { type: Array<unknown>, default: () => [] },
    attrOptions: { type: Array<string>, default: () => [] },
    buffOptions: { type: Array<string>, default: () => [] },
    skillOptions: { type: Array<string>, default: () => [] }
  },
  emits: ['update:modelValue'],
  setup(props, { emit }) {
    const updateAt = (index: number, value: unknown) => {
      const next = [...props.modelValue];
      next[index] = value;
      emit('update:modelValue', next);
    };
    const patchAt = (index: number, patch: Record<string, unknown>) => {
      const current = isRecord(props.modelValue[index]) ? props.modelValue[index] : {};
      updateAt(index, { ...current, ...patch });
    };
    const add = () => emit('update:modelValue', [...props.modelValue, { kind: 'compare_attr', actorAttr: '', operator: '>', targetAttr: '' }]);
    const removeAt = (index: number) => {
      const next = [...props.modelValue];
      next.splice(index, 1);
      emit('update:modelValue', next);
    };
    return { patchAt, add, removeAt, TARGET_PREDICATE_KINDS, TARGET_COMPARE_OPERATORS, ENEMY_TYPES };
  },
  template: `
    <div class="effects-editor">
      <article class="nested-card" v-for="(predicate, index) in modelValue" :key="index">
        <div class="nested-head">
          <strong>#{{ index + 1 }}</strong>
          <button class="danger" @click="removeAt(index)">删除</button>
        </div>
        <div class="form-grid cols-2">
          <label>类型<SearchSelect :model-value="String(predicate.kind || 'compare_attr')" :options="TARGET_PREDICATE_KINDS" :allow-empty="false" @update:model-value="(value) => patchAt(index, { kind: value })" /></label>
          <template v-if="predicate.kind === 'compare_attr'">
            <label>施法者属性<SearchSelect :model-value="String(predicate.actorAttr || '')" :options="attrOptions" @update:model-value="(value) => patchAt(index, { actorAttr: value })" /></label>
            <label>比较符<SearchSelect :model-value="String(predicate.operator || '>')" :options="TARGET_COMPARE_OPERATORS" :allow-empty="false" @update:model-value="(value) => patchAt(index, { operator: value })" /></label>
            <label class="wide">目标属性<SearchSelect :model-value="String(predicate.targetAttr || '')" :options="attrOptions" @update:model-value="(value) => patchAt(index, { targetAttr: value })" /></label>
          </template>
          <template v-else-if="predicate.kind === 'exclude_enemy_type'">
            <label class="wide">排除敌人类型<SearchSelect :model-value="String(predicate.enemyType || 'boss')" :options="ENEMY_TYPES" :allow-empty="false" @update:model-value="(value) => patchAt(index, { enemyType: value })" /></label>
          </template>
          <template v-else-if="predicate.kind === 'require_target_has_status' || predicate.kind === 'exclude_target_has_status'">
            <label class="wide">{{ predicate.kind === 'exclude_target_has_status' ? '排除状态' : '需要状态' }}<SearchSelect :model-value="String(predicate.statusId || '')" :options="buffOptions" @update:model-value="(value) => patchAt(index, { statusId: value })" /></label>
          </template>
          <p v-else class="empty-text">该谓词不需要额外参数。</p>
        </div>
      </article>
      <div class="toolbar">
        <button class="secondary" @click="add">新增谓词</button>
      </div>
    </div>
  `
});

const TargetingEditor = defineComponent({
  name: 'TargetingEditor',
  components: { StringListEditor, FormulaEditor, NumberInput, SearchSelect, TargetPredicatesEditor },
  props: {
    modelValue: { type: Object, default: () => defaultTargeting() },
    attrOptions: { type: Array<string>, default: () => [] },
    buffOptions: { type: Array<string>, default: () => [] },
    allowIgnoreTargetPriority: { type: Boolean, default: false }
  },
  emits: ['update:modelValue'],
  setup(props, { emit }) {
    const eventValue = (event: Event) => (event.target as HTMLInputElement | HTMLSelectElement).value;
    const eventChecked = (event: Event) => (event.target as HTMLInputElement).checked;
    const target = computed(() => (isRecord(props.modelValue) ? props.modelValue : defaultTargeting()));
    const targetCount = computed(() => (isRecord(target.value.targetCount) ? target.value.targetCount : { base: 1, formula: [], max: 0 }));
    const update = (patch: Record<string, unknown>) => emit('update:modelValue', { ...target.value, ...patch });
    const updateCount = (patch: Record<string, unknown>) => update({ targetCount: { ...targetCount.value, ...patch } });
    return { target, targetCount, update, updateCount, POSITIONS, SEEK_OPTIONS, TARGET_SCOPES, SELECT_RULES, eventValue, eventChecked, optionLabel };
  },
  template: `
    <div class="targeting-editor">
      <label>目标位置<StringListEditor :model-value="target.targetPositions || []" :options="POSITIONS" @update:model-value="(value) => update({ targetPositions: value })" /></label>
      <label>二次搜索<SearchSelect :model-value="String(target.secondarySeek || 'front_to_rear')" :options="SEEK_OPTIONS" :allow-empty="false" @update:model-value="(value) => update({ secondarySeek: value })" /></label>
      <label>目标阵营<SearchSelect :model-value="String(target.targetScope || 'enemy')" :options="TARGET_SCOPES" :allow-empty="false" @update:model-value="(value) => update({ targetScope: value })" /></label>
      <label>选择规则<SearchSelect :model-value="String(target.selectRule || 'random')" :options="SELECT_RULES" :allow-empty="false" @update:model-value="(value) => update({ selectRule: value })" /></label>
      <label>选择属性<SearchSelect :model-value="String(target.selectAttrKey || '')" :options="attrOptions" @update:model-value="(value) => update({ selectAttrKey: value })" /></label>
      <label class="checkbox-field"><input type="checkbox" :checked="Boolean(target.fallbackSelfIfEmpty)" @change="update({ fallbackSelfIfEmpty: eventChecked($event) })" />无目标时选自己</label>
      <label v-if="allowIgnoreTargetPriority" class="checkbox-field"><input type="checkbox" :checked="target.ignoreTargetPriority === true" @change="update({ ignoreTargetPriority: eventChecked($event) ? true : undefined })" />怪物专属：无视嘲讽与目标优先</label>
      <label>基础目标数<NumberInput :model-value="targetCount.base || 1" @update:model-value="(value) => updateCount({ base: value })" /></label>
      <label>最大目标数<NumberInput :model-value="targetCount.max || 0" @update:model-value="(value) => updateCount({ max: value > 0 ? value : 0 })" /></label>
      <label class="wide">目标数公式<FormulaEditor :model-value="{ baseAdd: 0, terms: targetCount.formula || [] }" :attr-options="attrOptions" @update:model-value="(value) => updateCount({ formula: value.terms || [] })" /></label>
      <label class="wide">目标谓词<TargetPredicatesEditor :model-value="target.targetPredicates || []" :attr-options="attrOptions" :buff-options="buffOptions" @update:model-value="(value) => update({ targetPredicates: value })" /></label>
    </div>
  `
});

const CastGuardsEditor = defineComponent({
  name: 'CastGuardsEditor',
  components: { SearchSelect },
  props: { modelValue: { type: Array<unknown>, default: () => [] } },
  emits: ['update:modelValue'],
  setup(props, { emit }) {
    const updateAt = (index: number, value: unknown) => {
      const next = [...props.modelValue];
      next[index] = value;
      emit('update:modelValue', next);
    };
    const patchAt = (index: number, patch: Record<string, unknown>) => {
      const current = isRecord(props.modelValue[index]) ? props.modelValue[index] : {};
      updateAt(index, { ...current, ...patch });
    };
    const add = () => emit('update:modelValue', [...props.modelValue, { kind: 'enemy_side_not_has_effect', effectKind: 'invert_relation_perspective' }]);
    const removeAt = (index: number) => {
      const next = [...props.modelValue];
      next.splice(index, 1);
      emit('update:modelValue', next);
    };
    return { patchAt, add, removeAt, CAST_GUARD_KINDS, BUFF_EFFECT_KINDS };
  },
  template: `
    <div class="effects-editor">
      <article class="nested-card" v-for="(guard, index) in modelValue" :key="index">
        <div class="nested-head">
          <strong>#{{ index + 1 }}</strong>
          <button class="danger" @click="removeAt(index)">删除</button>
        </div>
        <div class="form-grid cols-2">
          <label>门禁类型<SearchSelect :model-value="String(guard.kind || 'enemy_side_not_has_effect')" :options="CAST_GUARD_KINDS" :allow-empty="false" @update:model-value="(value) => patchAt(index, { kind: value })" /></label>
          <label v-if="guard.kind === 'enemy_side_not_has_effect'">EffectKind<SearchSelect :model-value="String(guard.effectKind || '')" :options="BUFF_EFFECT_KINDS" @update:model-value="(value) => patchAt(index, { effectKind: value })" /></label>
        </div>
      </article>
      <div class="toolbar">
        <button class="secondary" @click="add">新增门禁</button>
      </div>
    </div>
  `
});

function defaultDamageSpecNode(damageType = 'physical'): DataRow {
  return {
    damageType,
    amountFormula: defaultFormula(),
    hitCount: { base: 1, formula: [] },
    flatDefensePenetrationFormula: defaultFormula(),
    percentDefensePenetrationFormula: defaultFormula(),
    physicalFlatDefensePenetrationFormula: defaultFormula(),
    physicalPercentDefensePenetrationFormula: defaultFormula(),
    magicalFlatDefensePenetrationFormula: defaultFormula(),
    magicalPercentDefensePenetrationFormula: defaultFormula(),
    extraDamageAmplificationFormula: defaultFormula(),
    bonusAgainstTargetTypes: {}
  };
}

const DamageSpecEditor = defineComponent({
  name: 'DamageSpecEditor',
  components: { FormulaEditor, NumberInput, JsonFieldEditor, StringListEditor },
  props: {
    modelValue: { type: Object as PropType<DataRow | null | undefined>, default: () => ({}) },
    attrOptions: { type: Array<string>, default: () => [] },
    buffOptions: { type: Array<string>, default: () => [] },
    allowEnemyOnlyPierce: { type: Boolean, default: false }
  },
  emits: ['update:modelValue'],
  setup(props, { emit }) {
    const spec = computed(() => {
      const current = isRecord(props.modelValue) ? props.modelValue : {};
      return { ...defaultDamageSpecNode(String(current.damageType || 'physical')), ...current };
    });
    const eventValue = (event: Event) => (event.target as HTMLInputElement | HTMLSelectElement).value;
    const eventChecked = (event: Event) => (event.target as HTMLInputElement).checked;
    const patch = (patchNode: Record<string, unknown>) => emit('update:modelValue', { ...spec.value, ...patchNode });
    const patchFormula = (key: string, value: unknown) => patch({ [key]: value });
    const patchHitCount = (patchNode: Record<string, unknown>) => {
      const currentHitCount = isRecord(spec.value.hitCount) ? spec.value.hitCount : { base: 1, formula: [] };
      patch({ hitCount: { ...currentHitCount, ...patchNode } });
    };
    const hitCountFormula = computed(() => {
      const currentHitCount = isRecord(spec.value.hitCount) ? spec.value.hitCount : {};
      return { baseAdd: 0, terms: Array.isArray(currentHitCount.formula) ? currentHitCount.formula : [] };
    });
    const hitDamageScalingEnabled = computed(() => {
      const current = isRecord(spec.value.hitDamageScaling) ? spec.value.hitDamageScaling : {};
      return current.kind === POWERED_EXPONENTIAL_DECAY_KIND;
    });
    const toggleHitDamageScaling = (enabled: boolean) => {
      if (enabled) {
        patch({ hitDamageScaling: { ...DEFAULT_POWERED_EXPONENTIAL_DECAY } });
        return;
      }
      const { hitDamageScaling: _removed, ...next } = spec.value;
      emit('update:modelValue', next);
    };
    const patchHitDamageScaling = (patchNode: Record<string, unknown>) => {
      const current = normalizeHitDamageScaling(spec.value.hitDamageScaling) || DEFAULT_POWERED_EXPONENTIAL_DECAY;
      patch({ hitDamageScaling: { ...current, ...patchNode, kind: POWERED_EXPONENTIAL_DECAY_KIND } });
    };
    const hitDamageScalingPreview = computed(() => {
      const scaling = normalizeHitDamageScaling(spec.value.hitDamageScaling);
      if (!scaling) return '';
      const percent = (value: number) => `${(value * 100).toFixed(4).replace(/\.?0+$/, '')}%`;
      const cap = approximateHitDamageEquivalentCap(scaling);
      const capText = cap ? `${cap.converged ? '理论上限' : '前段累计'} ${cap.equivalentHits.toFixed(4)} 次完整伤害` : '';
      return `第2击 ${percent(hitDamageMultiplier(scaling, 2))} · 第12击 ${percent(hitDamageMultiplier(scaling, 12))} · 36击等效 ${hitDamageEquivalentSum(scaling, 36).toFixed(4)} · ${capText}`;
    });
    const pierceEnabled = computed(() => configuredEnemyOnlyDamageReductionPierceFields(props.modelValue).length > 0);
    const togglePierce = (enabled: boolean) => {
      if (!enabled) {
        emit('update:modelValue', removeEnemyOnlyDamageReductionPierceFields(props.modelValue));
        return;
      }
      patch({
        enemyOnlyDamageReductionPierce: 0.25,
        enemyOnlyDamageReductionPierceTargetStatuses: []
      });
    };
    const patchPierce = (value: unknown) => patch({
      enemyOnlyDamageReductionPierce: Math.max(0, Math.min(1, Number(value) || 0))
    });
    return {
      spec,
      eventValue,
      eventChecked,
      patch,
      patchFormula,
      patchHitCount,
      hitCountFormula,
      hitDamageScalingEnabled,
      toggleHitDamageScaling,
      patchHitDamageScaling,
      hitDamageScalingPreview,
      pierceEnabled,
      togglePierce,
      patchPierce,
      DAMAGE_TYPES
    };
  },
  template: `
    <div class="form-grid cols-2">
      <label>伤害类型
        <select :value="String(spec.damageType || 'physical')" @change="patch({ damageType: eventValue($event) })">
          <option v-for="type in DAMAGE_TYPES" :key="type" :value="type">{{ type }}</option>
        </select>
      </label>
      <label v-if="allowEnemyOnlyPierce" class="checkbox-field"><input type="checkbox" :checked="pierceEnabled" @change="togglePierce(eventChecked($event))" />怪物专属减伤穿透</label>
      <template v-if="allowEnemyOnlyPierce && pierceEnabled">
        <label>穿透减伤比例<NumberInput :model-value="Number(spec.enemyOnlyDamageReductionPierce || 0)" :min="0" :max="1" :step="0.05" @update:model-value="patchPierce" /></label>
        <label class="wide">触发状态（空=无条件）<StringListEditor :model-value="Array.isArray(spec.enemyOnlyDamageReductionPierceTargetStatuses) ? spec.enemyOnlyDamageReductionPierceTargetStatuses : []" :options="buffOptions" link-table="buff" link-label="状态" @update:model-value="(value) => patch({ enemyOnlyDamageReductionPierceTargetStatuses: value })" /></label>
      </template>
      <label>基础命中次数<NumberInput :model-value="Number(spec.hitCount?.base || 1)" @update:model-value="(value) => patchHitCount({ base: Math.max(1, Math.trunc(value || 1)) })" /></label>
      <label class="wide">命中次数公式<FormulaEditor :model-value="hitCountFormula" :attr-options="attrOptions" @update:model-value="(value) => patchHitCount({ formula: value.terms || [] })" /></label>
      <label class="checkbox-field wide"><input type="checkbox" :checked="hitDamageScalingEnabled" @change="toggleHitDamageScaling(eventChecked($event))" />启用逐击伤害衰减</label>
      <template v-if="hitDamageScalingEnabled">
        <label>第2击倍率<NumberInput :model-value="Number(spec.hitDamageScaling?.secondHitMultiplier ?? 0.998)" :min="0.000001" :max="0.999999" :step="0.0001" @update:model-value="(value) => patchHitDamageScaling({ secondHitMultiplier: Math.max(0.000001, Math.min(0.999999, Number(value) || 0.998)) })" /></label>
        <label>曲线幂次<NumberInput :model-value="Number(spec.hitDamageScaling?.curvePower ?? 2.1708)" :min="1" :max="3" :step="0.0001" @update:model-value="(value) => patchHitDamageScaling({ curvePower: Math.max(1, Math.min(3, Number(value) || 2.1708)) })" /></label>
        <div class="wide dim" style="font-size: 12px; line-height: 1.5;">规则：第 n 击倍率 = 第2击倍率 ^ ((n-1) ^ 曲线幂次)。{{ hitDamageScalingPreview }}</div>
      </template>
      <label class="wide">伤害数值公式<FormulaEditor :model-value="spec.amountFormula || {}" :attr-options="attrOptions" @update:model-value="(value) => patchFormula('amountFormula', value)" /></label>
      <label>固定穿透公式<FormulaEditor :model-value="spec.flatDefensePenetrationFormula || {}" :attr-options="attrOptions" @update:model-value="(value) => patchFormula('flatDefensePenetrationFormula', value)" /></label>
      <label>百分比穿透公式<FormulaEditor :model-value="spec.percentDefensePenetrationFormula || {}" :attr-options="attrOptions" @update:model-value="(value) => patchFormula('percentDefensePenetrationFormula', value)" /></label>
      <label>物理固定穿透公式<FormulaEditor :model-value="spec.physicalFlatDefensePenetrationFormula || {}" :attr-options="attrOptions" @update:model-value="(value) => patchFormula('physicalFlatDefensePenetrationFormula', value)" /></label>
      <label>物理百分比穿透公式<FormulaEditor :model-value="spec.physicalPercentDefensePenetrationFormula || {}" :attr-options="attrOptions" @update:model-value="(value) => patchFormula('physicalPercentDefensePenetrationFormula', value)" /></label>
      <label>法术固定穿透公式<FormulaEditor :model-value="spec.magicalFlatDefensePenetrationFormula || {}" :attr-options="attrOptions" @update:model-value="(value) => patchFormula('magicalFlatDefensePenetrationFormula', value)" /></label>
      <label>法术百分比穿透公式<FormulaEditor :model-value="spec.magicalPercentDefensePenetrationFormula || {}" :attr-options="attrOptions" @update:model-value="(value) => patchFormula('magicalPercentDefensePenetrationFormula', value)" /></label>
      <label class="wide">额外伤害增幅公式<FormulaEditor :model-value="spec.extraDamageAmplificationFormula || {}" :attr-options="attrOptions" @update:model-value="(value) => patchFormula('extraDamageAmplificationFormula', value)" /></label>
      <label class="wide">目标类型加成<JsonFieldEditor :model-value="spec.bonusAgainstTargetTypes || {}" :path="['bonusAgainstTargetTypes']" @update:model-value="(value) => patch({ bonusAgainstTargetTypes: value })" /></label>
    </div>
  `
});

const EffectsEditor = defineComponent({
  name: 'EffectsEditor',
  components: { TargetingEditor, FormulaEditor, StringListEditor, JsonFieldEditor, NumberInput, DamageSpecEditor, SummonStatsFormulaEditor, SearchSelect },
  props: {
    modelValue: { type: Array<unknown>, default: () => [] },
    stableIdOwner: { type: String, default: '' },
    attrOptions: { type: Array<string>, default: () => [] },
    buffOptions: { type: Array<string>, default: () => [] },
    enemyOptions: { type: Array<string>, default: () => [] },
    skillOptions: { type: Array<string>, default: () => [] },
    allowEnemyOnlyPierce: { type: Boolean, default: false },
    allowEnemyOnlyTargeting: { type: Boolean, default: false },
    scoreSections: { type: Array<ScoreBreakdownSection>, default: () => [] }
  },
  emits: ['update:modelValue'],
  setup(props, { emit }) {
    const eventValue = (event: Event) => (event.target as HTMLInputElement | HTMLSelectElement).value;
    const eventChecked = (event: Event) => (event.target as HTMLInputElement).checked;
    const updateAt = (index: number, value: unknown) => {
      const next = [...props.modelValue];
      next[index] = value;
      emit('update:modelValue', next);
    };
    const patchAt = (index: number, patch: Record<string, unknown>) => {
      const current = isRecord(props.modelValue[index]) ? props.modelValue[index] : {};
      updateAt(index, { ...current, ...patch });
    };
    const add = (kind: 'damage' | 'heal' | 'resource' | 'status' | 'summon' | 'weighted_random_per_target' | 'corpse_explosion' | 'adjust_skill_cooldown') => {
      const effect = defaultEffect(kind);
      if (props.stableIdOwner) effect.id = createStableSkillEffectId(props.stableIdOwner, kind, props.modelValue);
      emit('update:modelValue', [...props.modelValue, effect]);
    };
    const assignStableId = (index: number) => {
      const effect = isRecord(props.modelValue[index]) ? props.modelValue[index] : {};
      patchAt(index, {
        id: createStableSkillEffectId(props.stableIdOwner, effect.kind, props.modelValue.filter((_, effectIndex) => effectIndex !== index)),
        effectId: undefined
      });
    };
    const removeAt = (index: number) => {
      const next = [...props.modelValue];
      next.splice(index, 1);
      emit('update:modelValue', next);
    };
    const summonFormulaAttrOptions = computed(() => {
      const next = [...props.attrOptions];
      for (const attr of ['equipment_enhance_level', 'max_hp', 'max_mp']) {
        if (!next.includes(attr)) next.push(attr);
      }
      return next;
    });
    const corpseExplosionFormulaAttrOptions = computed(() => {
      const next = [...props.attrOptions];
      if (!next.includes('corpse_max_hp')) next.push('corpse_max_hp');
      return next;
    });
    const hasMaxCountFormula = (effect: DataRow) => {
      const payload = isRecord(effect.summonPayload) ? effect.summonPayload : {};
      return isRecord(payload.maxCountFormula);
    };
    const usesStatusProbability = (effect: DataRow) => {
      const payload = isRecord(effect.statusPayload) ? effect.statusPayload : {};
      return payload.applyChanceBase !== undefined || payload.applyChanceFormula !== undefined;
    };
    const toggleStatusProbability = (index: number, enabled: boolean) => {
      const effect = isRecord(props.modelValue[index]) ? props.modelValue[index] : {};
      const statusPayload = isRecord(effect.statusPayload) ? effect.statusPayload : {};
      patchAt(index, {
        statusPayload: enabled
          ? { ...statusPayload, applyChanceBase: 1, applyChancePerSkillLevel: 0, applyChanceMin: 0, applyChanceMax: 1, applyChanceFormula: { baseAdd: 0, terms: [] } }
          : { ...statusPayload, applyChanceBase: undefined, applyChancePerSkillLevel: undefined, applyChanceMin: undefined, applyChanceMax: undefined, applyChanceFormula: undefined }
      });
    };
    const usesEffectApplyProbability = (effect: DataRow) => effect.applyChanceBase !== undefined || effect.applyChanceFormula !== undefined;
    const toggleEffectApplyProbability = (index: number, enabled: boolean) => patchAt(index, enabled
      ? { applyChanceBase: 1, applyChancePerSkillLevel: 0, applyChanceMin: 0, applyChanceMax: 1, applyChanceFormula: { baseAdd: 0, terms: [] } }
      : { applyChanceBase: undefined, applyChancePerSkillLevel: undefined, applyChanceMin: undefined, applyChanceMax: undefined, applyChanceFormula: undefined });
    const weightedChoices = (effect: DataRow) => asArray(effect.choices).filter(isRecord);
    const patchWeightedChoice = (effectIndex: number, choiceIndex: number, patch: DataRow) => {
      const effect = isRecord(props.modelValue[effectIndex]) ? props.modelValue[effectIndex] : {};
      const choices = weightedChoices(effect).map((choice) => ({ ...choice }));
      choices[choiceIndex] = { ...(choices[choiceIndex] ?? {}), ...patch };
      patchAt(effectIndex, { choices });
    };
    const addWeightedChoice = (effectIndex: number) => {
      const effect = isRecord(props.modelValue[effectIndex]) ? props.modelValue[effectIndex] : {};
      const choices = weightedChoices(effect);
      const existing = new Set(choices.map((choice) => asString(choice.id)));
      let suffix = choices.length + 1;
      while (existing.has(`status_${suffix}`)) suffix += 1;
      patchAt(effectIndex, { choices: [...choices, { id: `status_${suffix}`, weight: 1, statusId: '', durationTurns: 1 }] });
    };
    const removeWeightedChoice = (effectIndex: number, choiceIndex: number) => {
      const effect = isRecord(props.modelValue[effectIndex]) ? props.modelValue[effectIndex] : {};
      const choices = weightedChoices(effect);
      choices.splice(choiceIndex, 1);
      patchAt(effectIndex, { choices });
    };
    return {
      updateAt,
      patchAt,
      assignStableId,
      add,
      removeAt,
      EFFECT_KINDS,
      DAMAGE_TYPES,
      eventValue,
      eventChecked,
      optionLabel,
      summonFormulaAttrOptions,
      corpseExplosionFormulaAttrOptions,
      hasMaxCountFormula,
      usesStatusProbability,
      toggleStatusProbability,
      usesEffectApplyProbability,
      toggleEffectApplyProbability,
      weightedChoices,
      patchWeightedChoice,
      addWeightedChoice,
      removeWeightedChoice,
      SUMMON_ON_CASTER_DEATH_OPTIONS,
      SUMMON_ON_CAP_OVERFLOW_OPTIONS,
      SUMMON_ON_DISPEL_OPTIONS,
      SUMMON_ON_DURATION_END_OPTIONS
    };
  },
  template: `
    <div class="effects-editor">
      <article class="nested-card" v-for="(effect, index) in modelValue" :key="effect.id || effect.effectId || index">
        <div class="nested-head">
          <strong>#{{ index + 1 }}</strong>
          <select :value="effect.kind || 'damage'" @change="patchAt(index, { kind: eventValue($event) })">
            <option v-for="kind in EFFECT_KINDS" :key="kind" :value="kind">{{ kind }}</option>
          </select>
          <span v-if="scoreSections[index]" class="score-chip">{{ scoreSections[index].score }}分</span>
          <button class="danger" @click="removeAt(index)">删除</button>
        </div>
        <div v-if="scoreSections[index]?.items?.length" class="effect-score-lines">
          <span v-for="(item, itemIndex) in scoreSections[index].items?.slice(0, 8)" :key="itemIndex">{{ item.label }} {{ item.score }}<small v-if="item.meta"> · {{ item.meta }}</small></span>
        </div>
        <div class="form-grid cols-2">
          <label v-if="stableIdOwner" class="wide">稳定效果 ID
            <div class="linked-control">
              <input :value="String(effect.id || effect.effectId || '')" @input="patchAt(index, { id: eventValue($event), effectId: undefined })" />
              <button type="button" class="link-button" @click="assignStableId(index)">生成</button>
            </div>
          </label>
          <label>目标<TargetingEditor :model-value="effect.targeting || {}" :attr-options="attrOptions" :buff-options="buffOptions" :allow-ignore-target-priority="allowEnemyOnlyTargeting" @update:model-value="(value) => patchAt(index, { targeting: value })" /></label>
          <template v-if="effect.kind === 'damage' || effect.kind === 'heal'">
            <label>伤害类型<select :value="effect.damageType || 'physical'" @change="patchAt(index, { damageType: eventValue($event) })"><option v-for="type in DAMAGE_TYPES" :key="type" :value="type">{{ type }}</option></select></label>
            <label>数值公式<FormulaEditor :model-value="effect.amountFormula || {}" :attr-options="attrOptions" @update:model-value="(value) => patchAt(index, { amountFormula: value })" /></label>
            <label v-if="effect.kind === 'damage'" class="wide">DamageSpec<DamageSpecEditor :model-value="effect.damageSpec || {}" :attr-options="attrOptions" :buff-options="buffOptions" :allow-enemy-only-pierce="allowEnemyOnlyPierce" @update:model-value="(value) => patchAt(index, { damageSpec: value })" /></label>
            <label v-if="effect.kind === 'heal'">MP消耗公式<FormulaEditor :model-value="effect.mpCost || { baseAdd: 0, terms: [] }" :attr-options="attrOptions" @update:model-value="(value) => patchAt(index, { mpCost: value && (Number(value.baseAdd) !== 0 || (value.terms || []).length > 0) ? value : undefined })" /></label>
          </template>
          <template v-else-if="effect.kind === 'resource'">
            <label>资源<select :value="effect.resource || 'mp'" @change="patchAt(index, { resource: eventValue($event) })"><option value="mp">mp</option><option value="hp">hp</option></select></label>
            <label>数值公式<FormulaEditor :model-value="effect.amountFormula || {}" :attr-options="attrOptions" @update:model-value="(value) => patchAt(index, { amountFormula: value })" /></label>
          </template>
          <template v-else-if="effect.kind === 'adjust_skill_cooldown'">
            <label>冷却调整<NumberInput :model-value="Number(effect.amount || 0)" @update:model-value="(value) => patchAt(index, { amount: Math.trunc(Number(value) || 0) })" /></label>
            <label class="checkbox-field"><input type="checkbox" :checked="Boolean(effect.includeBasic)" @change="patchAt(index, { includeBasic: eventChecked($event) })" />包含普攻</label>
            <label class="wide">指定技能（空=全部）<StringListEditor :model-value="Array.isArray(effect.skillIds) ? effect.skillIds : []" :options="skillOptions" @update:model-value="(value) => patchAt(index, { skillIds: value })" /></label>
            <label class="wide">排除技能<StringListEditor :model-value="Array.isArray(effect.excludeSkillIds) ? effect.excludeSkillIds : []" :options="skillOptions" @update:model-value="(value) => patchAt(index, { excludeSkillIds: value })" /></label>
          </template>
          <template v-else-if="effect.kind === 'summon'">
            <label>召唤单位<SearchSelect :model-value="String(effect.summonPayload?.unitId || '')" :options="enemyOptions" :allow-empty="false" @update:model-value="(value) => patchAt(index, { summonPayload: { ...(effect.summonPayload || {}), unitId: value } })" /></label>
            <label>召唤阵营<SearchSelect :model-value="String(effect.summonPayload?.side || 'player')" :options="SIDE_OPTIONS" :allow-empty="false" @update:model-value="(value) => patchAt(index, { summonPayload: { ...(effect.summonPayload || {}), side: value } })" /></label>
            <label>数量公式<FormulaEditor :model-value="effect.summonPayload?.countFormula || { baseAdd: 1, terms: [] }" :attr-options="summonFormulaAttrOptions" @update:model-value="(value) => patchAt(index, { summonPayload: { ...(effect.summonPayload || {}), countFormula: value } })" /></label>
            <label>持续回合公式<FormulaEditor :model-value="effect.summonPayload?.durationFormula || { baseAdd: 0, terms: [] }" :attr-options="summonFormulaAttrOptions" @update:model-value="(value) => patchAt(index, { summonPayload: { ...(effect.summonPayload || {}), durationFormula: value } })" /></label>
            <label>固定持续回合（兼容兜底）<NumberInput :model-value="Number(effect.summonPayload?.durationTurns || 0)" @update:model-value="(value) => patchAt(index, { summonPayload: { ...(effect.summonPayload || {}), durationTurns: Math.max(0, Math.trunc(Number(value) || 0)) } })" /></label>
            <label>固定数量上限（0=不限）<NumberInput :model-value="Number(effect.summonPayload?.maxCountPerUnit || 0)" @update:model-value="(value) => patchAt(index, { summonPayload: { ...(effect.summonPayload || {}), maxCountPerUnit: Math.max(0, Math.trunc(Number(value) || 0)) } })" /></label>
            <label class="checkbox-field"><input type="checkbox" :checked="hasMaxCountFormula(effect)" @change="patchAt(index, { summonPayload: { ...(effect.summonPayload || {}), maxCountFormula: eventChecked($event) ? { baseAdd: 1, terms: [] } : undefined } })" />使用数量上限公式</label>
            <label v-if="hasMaxCountFormula(effect)" class="wide">数量上限公式（配置后优先）<FormulaEditor :model-value="effect.summonPayload?.maxCountFormula" :attr-options="summonFormulaAttrOptions" @update:model-value="(value) => patchAt(index, { summonPayload: { ...(effect.summonPayload || {}), maxCountFormula: value } })" /></label>
            <label class="checkbox-field"><input type="checkbox" :checked="Boolean(effect.summonPayload?.persistAcrossBattles)" @change="patchAt(index, { summonPayload: { ...(effect.summonPayload || {}), persistAcrossBattles: eventChecked($event) } })" />跨战斗保留</label>
            <label>召唤者死亡策略<SearchSelect :model-value="String(effect.summonPayload?.onCasterDeath || 'keep_until_duration_end')" :options="SUMMON_ON_CASTER_DEATH_OPTIONS" :allow-empty="false" @update:model-value="(value) => patchAt(index, { summonPayload: { ...(effect.summonPayload || {}), onCasterDeath: value } })" /></label>
            <label>上限溢出策略<SearchSelect :model-value="String(effect.summonPayload?.onCapOverflow || 'replace_oldest')" :options="SUMMON_ON_CAP_OVERFLOW_OPTIONS" :allow-empty="false" @update:model-value="(value) => patchAt(index, { summonPayload: { ...(effect.summonPayload || {}), onCapOverflow: value } })" /></label>
            <label>驱散处理<SearchSelect :model-value="String(effect.summonPayload?.onDispel || 'despawn_immediately')" :options="SUMMON_ON_DISPEL_OPTIONS" :allow-empty="false" @update:model-value="(value) => patchAt(index, { summonPayload: { ...(effect.summonPayload || {}), onDispel: value } })" /></label>
            <label>持续结束处理<SearchSelect :model-value="String(effect.summonPayload?.onDurationEnd || 'despawn_immediately')" :options="SUMMON_ON_DURATION_END_OPTIONS" :allow-empty="false" @update:model-value="(value) => patchAt(index, { summonPayload: { ...(effect.summonPayload || {}), onDurationEnd: value } })" /></label>
            <label class="wide">属性增益公式（按属性）<SummonStatsFormulaEditor :model-value="effect.summonPayload?.statsFormula || {}" :attr-options="attrOptions" :formula-attr-options="summonFormulaAttrOptions" @update:model-value="(value) => patchAt(index, { summonPayload: { ...(effect.summonPayload || {}), statsFormula: value } })" /></label>
          </template>
          <template v-else-if="effect.kind === 'weighted_random_per_target'">
            <label class="checkbox-field"><input type="checkbox" :checked="usesEffectApplyProbability(effect)" @change="toggleEffectApplyProbability(index, eventChecked($event))" />使用概率判定</label>
            <template v-if="usesEffectApplyProbability(effect)">
              <label>基础概率<NumberInput :model-value="Number(effect.applyChanceBase ?? 1)" @update:model-value="(value) => patchAt(index, { applyChanceBase: value })" /></label>
              <label>每技能等级概率<NumberInput :model-value="Number(effect.applyChancePerSkillLevel ?? 0)" @update:model-value="(value) => patchAt(index, { applyChancePerSkillLevel: value })" /></label>
              <label>最小概率<NumberInput :model-value="Number(effect.applyChanceMin ?? 0)" @update:model-value="(value) => patchAt(index, { applyChanceMin: value })" /></label>
              <label>最大概率<NumberInput :model-value="Number(effect.applyChanceMax ?? 1)" @update:model-value="(value) => patchAt(index, { applyChanceMax: value })" /></label>
              <label class="wide">概率公式<FormulaEditor :model-value="effect.applyChanceFormula || { baseAdd: 0, terms: [] }" :attr-options="attrOptions" @update:model-value="(value) => patchAt(index, { applyChanceFormula: value })" /></label>
            </template>
            <div class="wide buff-rule-editor">
              <article class="nested-card" v-for="(choice, choiceIndex) in weightedChoices(effect)" :key="choice.id || choiceIndex">
                <div class="nested-head">
                  <strong>候选 #{{ choiceIndex + 1 }}</strong>
                  <button class="danger" @click="removeWeightedChoice(index, choiceIndex)">删除</button>
                </div>
                <div class="form-grid cols-2">
                  <label>候选ID<input :value="String(choice.id || '')" @input="patchWeightedChoice(index, choiceIndex, { id: eventValue($event) })" /></label>
                  <label>权重<NumberInput :model-value="Number(choice.weight ?? 1)" @update:model-value="(value) => patchWeightedChoice(index, choiceIndex, { weight: Math.max(0, Number(value) || 0) })" /></label>
                  <label>状态<SearchSelect :model-value="String(choice.statusId || '')" :options="buffOptions" @update:model-value="(value) => patchWeightedChoice(index, choiceIndex, { statusId: value })" /></label>
                  <label>持续回合<NumberInput :model-value="Number(choice.durationTurns ?? 1)" @update:model-value="(value) => patchWeightedChoice(index, choiceIndex, { durationTurns: Math.max(0, Math.trunc(Number(value) || 0)) })" /></label>
                  <label class="wide">Payload<JsonFieldEditor :model-value="choice.payload || {}" :path="['choices', choiceIndex, 'payload']" @update:model-value="(value) => patchWeightedChoice(index, choiceIndex, { payload: value })" /></label>
                </div>
              </article>
              <button class="secondary" @click="addWeightedChoice(index)">新增随机候选</button>
            </div>
          </template>
          <template v-else-if="effect.kind === 'corpse_explosion'">
            <label class="wide">尸体目标<TargetingEditor :model-value="effect.corpseTargeting || {}" :attr-options="attrOptions" :buff-options="buffOptions" :allow-ignore-target-priority="allowEnemyOnlyTargeting" @update:model-value="(value) => patchAt(index, { corpseTargeting: value })" /></label>
            <label>伤害类型<select :value="effect.damageType || 'magical'" @change="patchAt(index, { damageType: eventValue($event) })"><option v-for="type in DAMAGE_TYPES" :key="type" :value="type">{{ type }}</option></select></label>
            <label class="wide">尸爆伤害公式<FormulaEditor :model-value="effect.amountFormula || { baseAdd: 0, terms: [] }" :attr-options="corpseExplosionFormulaAttrOptions" @update:model-value="(value) => patchAt(index, { amountFormula: value })" /></label>
            <label class="checkbox-field"><input type="checkbox" :checked="effect.consumeCorpse !== false" @change="patchAt(index, { consumeCorpse: eventChecked($event) })" />消耗尸体</label>
          </template>
          <template v-else-if="effect.kind === 'status'">
            <label>状态列表<StringListEditor :model-value="effect.statusPayload?.statusIds || []" :options="buffOptions" @update:model-value="(value) => patchAt(index, { statusPayload: { ...(effect.statusPayload || {}), statusIds: value } })" /></label>
            <label>基础回合<NumberInput :model-value="effect.statusPayload?.durationBaseTurns || 0" @update:model-value="(value) => patchAt(index, { statusPayload: { ...(effect.statusPayload || {}), durationBaseTurns: value } })" /></label>
            <label>每级回合<NumberInput :model-value="effect.statusPayload?.durationPerSkillLevelTurns || 0" @update:model-value="(value) => patchAt(index, { statusPayload: { ...(effect.statusPayload || {}), durationPerSkillLevelTurns: value } })" /></label>
            <label class="wide">持续公式<FormulaEditor :model-value="effect.statusPayload?.durationFormula || { baseAdd: 0, terms: [] }" :attr-options="attrOptions" @update:model-value="(value) => patchAt(index, { statusPayload: { ...(effect.statusPayload || {}), durationFormula: value } })" /></label>
            <label class="checkbox-field"><input type="checkbox" :checked="usesStatusProbability(effect)" @change="toggleStatusProbability(index, eventChecked($event))" />使用概率判定</label>
            <template v-if="usesStatusProbability(effect)">
              <label>基础概率<NumberInput :model-value="Number(effect.statusPayload?.applyChanceBase ?? 1)" @update:model-value="(value) => patchAt(index, { statusPayload: { ...(effect.statusPayload || {}), applyChanceBase: value } })" /></label>
              <label>每技能等级概率<NumberInput :model-value="Number(effect.statusPayload?.applyChancePerSkillLevel ?? 0)" @update:model-value="(value) => patchAt(index, { statusPayload: { ...(effect.statusPayload || {}), applyChancePerSkillLevel: value } })" /></label>
              <label>最小概率<NumberInput :model-value="Number(effect.statusPayload?.applyChanceMin ?? 0)" @update:model-value="(value) => patchAt(index, { statusPayload: { ...(effect.statusPayload || {}), applyChanceMin: value } })" /></label>
              <label>最大概率<NumberInput :model-value="Number(effect.statusPayload?.applyChanceMax ?? 1)" @update:model-value="(value) => patchAt(index, { statusPayload: { ...(effect.statusPayload || {}), applyChanceMax: value } })" /></label>
              <label class="wide">概率公式<FormulaEditor :model-value="effect.statusPayload?.applyChanceFormula || { baseAdd: 0, terms: [] }" :attr-options="attrOptions" @update:model-value="(value) => patchAt(index, { statusPayload: { ...(effect.statusPayload || {}), applyChanceFormula: value } })" /></label>
              <label class="checkbox-field"><input type="checkbox" :checked="Boolean(effect.ignoreProbabilityHooks)" @change="patchAt(index, { ignoreProbabilityHooks: eventChecked($event) })" />忽略概率钩子</label>
            </template>
          </template>
          <label>复用目标序号<NumberInput :model-value="effect.reuseTargetsFromEffect || 0" @update:model-value="(value) => patchAt(index, { reuseTargetsFromEffect: value || undefined })" /></label>
          <label>无目标依赖<JsonFieldEditor :model-value="effect.onlyWhenNoTargetFromEffects || []" :path="['onlyWhenNoTargetFromEffects']" @update:model-value="(value) => patchAt(index, { onlyWhenNoTargetFromEffects: value })" /></label>
          <label>施法者需有状态<StringListEditor :model-value="effect.requiredCasterStatuses || []" :options="buffOptions" @update:model-value="(value) => patchAt(index, { requiredCasterStatuses: value })" /></label>
          <label>目标需有状态<StringListEditor :model-value="effect.requiredTargetStatuses || []" :options="buffOptions" @update:model-value="(value) => patchAt(index, { requiredTargetStatuses: value })" /></label>
          <label>命中消费状态<StringListEditor :model-value="effect.consumeStatusesOnHit || []" :options="buffOptions" @update:model-value="(value) => patchAt(index, { consumeStatusesOnHit: value })" /></label>
          <label>命中消费施法者状态<StringListEditor :model-value="effect.consumeCasterStatusesOnHit || []" :options="buffOptions" @update:model-value="(value) => patchAt(index, { consumeCasterStatusesOnHit: value })" /></label>
          <label class="checkbox-field"><input type="checkbox" :checked="Boolean(effect.ignoreUntargetable)" @change="patchAt(index, { ignoreUntargetable: eventChecked($event) })" />忽略不可选中</label>
        </div>
      </article>
      <div class="toolbar">
        <button class="secondary" @click="add('damage')">新增伤害</button>
        <button class="secondary" @click="add('heal')">新增治疗</button>
        <button class="secondary" @click="add('resource')">新增资源</button>
        <button class="secondary" @click="add('status')">新增状态</button>
        <button class="secondary" @click="add('summon')">新增召唤</button>
        <button class="secondary" @click="add('weighted_random_per_target')">新增逐目标随机状态</button>
        <button class="secondary" @click="add('corpse_explosion')">新增尸爆</button>
        <button class="secondary" @click="add('adjust_skill_cooldown')">新增冷却调整</button>
      </div>
    </div>
  `
});

function defaultBuffCondition(kind = 'self_hp_percent_lte'): DataRow {
  if (kind === '__damage_types__') return { damageTypes: ['physical'] };
  const base: DataRow = { type: kind };
  if (kind === 'has_buff' || kind === 'source_has_buff' || kind === 'target_has_buff') return { ...base, buffId: '' };
  if (kind === 'self_weapon_class' || kind === 'source_weapon_class') return { ...base, weaponClasses: ['bow'] };
  if (kind === 'self_position') return { ...base, positions: ['midguard'] };
  if (kind === 'ally_side_has_positions') return { ...base, positions: ['vanguard', 'midguard', 'rearguard'] };
  if (kind.includes('charges_remaining')) return { ...base, value: 1 };
  if (kind.startsWith('self_buff_count_')) return { ...base, buffId: '', value: 1 };
  if (kind === 'target_buff_count_gte') return { ...base, buffId: '', value: 3 };
  if (kind === 'target_distinct_source_buff_count_gte') return { ...base, buffKind: 'debuff', value: 3 };
  if (kind === 'wave_interval_eq') return { ...base, divisor: 3, value: 0 };
  if (kind === 'self_skill_cooldown_count_gte') return { ...base, value: 2, includeBasicAttack: false, skillCategories: ['arcane', 'perception', 'snowpact'] };
  if (kind === 'equipment_enhance_level_gte') return { ...base, value: 40 };
  if (kind === 'player_party_equipped_unit_count_eq') return { ...base, itemId: '', value: 1 };
  if (kind === 'self_mp_lt' || kind === 'self_mp_gte') return { ...base, value: 30 };
  return { ...base, value: kind.includes('hp_percent') ? 50 : 100 };
}

function defaultBuffDslEffect(kind = 'add_stat'): DataRow {
  switch (kind) {
    case 'add_stat':
    case 'add_stat_percent':
      return { kind, attr: '', value: kind === 'add_stat_percent' ? 0.1 : 1 };
    case 'damage':
      return { kind, damageType: 'physical', value: 0, amountFormula: defaultFormula() };
    case 'heal':
      return { kind, value: 10 };
    case 'resource':
      return { kind, resource: 'mp', value: 3 };
    case 'add_skill_cooldown':
      return { kind, value: 2, includeBasic: false, skillIds: [], excludeSkillIds: [], skillCategories: [], selectionMode: 'all', maxSkills: 0 };
    case 'adjust_skill_cooldown':
      return { kind, value: -1, includeBasic: false, skillIds: [], excludeSkillIds: [], skillCategories: [], selectionMode: 'all', maxSkills: 0 };
    case 'add_buff':
      return { kind, buffId: '', durationTurns: 1, targetMode: 'self' };
    case 'remove_buff':
      return { kind, buffId: '' };
    case 'reduce_damage_percent':
    case 'set_damage_multiplier':
    case 'add_damage_percent':
    case 'add_damage_flat':
    case 'add_damage':
      return { kind, value: kind === 'set_damage_multiplier' ? 1 : 0.1 };
    case 'advance_position':
      return { kind, steps: 1 };
    case 'position_map':
      return { kind, mapping: { vanguard: 'vanguard', midguard: 'midguard', rearguard: 'rearguard' } };
    case 'side_override':
      return { kind, side: 'enemy' };
    case 'trigger_synced_summon_turn':
      return { kind, includeSummonTemplateIds: [] };
    case 'extra_enemy_refresh':
      return { kind, enemyPool: 'current_world', count: 1 };
    default:
      return { kind };
  }
}

function patchKnownBuffEffect(kind: string, current: DataRow): DataRow {
  const defaults = defaultBuffDslEffect(kind);
  const keep: DataRow = { kind };
  for (const key of Object.keys(defaults)) {
    if (key === 'kind') continue;
    keep[key] = current[key] ?? defaults[key];
  }
  if (current.delayMs !== undefined) keep.delayMs = current.delayMs;
  if (current.payload !== undefined) keep.payload = current.payload;
  return keep;
}

function unknownBuffEffectKeys(effect: DataRow): string[] {
  const common = new Set(['kind', 'type', 'delayMs']);
  const byKind: Record<string, string[]> = {
    add_stat: ['attr', 'stat', 'value'],
    add_stat_percent: ['attr', 'stat', 'value', 'percent'],
    damage: ['damageType', 'value', 'amount', 'amountFormula'],
    heal: ['value'],
    resource: ['resource', 'value', 'valueScale', 'minimumHp', 'payloadValueKey', 'payloadValueCoeff'],
    add_skill_cooldown: ['value', 'turns', 'amount', 'includeBasic', 'includeBasicAttack', 'skillIds', 'skillIDs', 'skills', 'excludeSkillIds', 'excludeSkillIDs', 'excludeSkills', 'skillCategories', 'selectionMode', 'maxSkills'],
    adjust_skill_cooldown: ['value', 'turns', 'amount', 'includeBasic', 'includeBasicAttack', 'skillIds', 'skillIDs', 'skills', 'excludeSkillIds', 'excludeSkillIDs', 'excludeSkills', 'skillCategories', 'selectionMode', 'maxSkills'],
    add_buff: ['buffId', 'durationTurns', 'durationMs', 'targetMode', 'payload'],
    remove_buff: ['buffId'],
    reduce_damage_percent: ['value', 'percent'],
    set_damage_multiplier: ['value'],
    add_damage_percent: ['value', 'percent'],
    add_damage_flat: ['value'],
    add_damage: ['value'],
    untargetable_by_enemy: [],
    untargetable_by_ally: [],
    treat_all_as_enemy: [],
    camouflage_targeting_remap: [],
    prefer_as_enemy_target: [],
    avoid_as_enemy_target: [],
    prefer_as_ally_target: [],
    avoid_as_ally_target: [],
    turn_action_suppressed: [],
    skill_action_suppressed: [],
    basic_attack_suppressed: [],
    frontline_fixation: [],
    position_override: ['position'],
    position_map: ['mapping'],
    advance_position: ['steps'],
    side_override: ['side'],
    invert_relation_perspective: [],
    damage_redirect_to_source: ['percent'],
    trigger_synced_summon_turn: ['includeSummonTemplateIds'],
    extra_enemy_refresh: ['enemyPool', 'count']
  };
  const kind = asString(effect.kind || effect.type, 'add_stat');
  const known = new Set([...(byKind[kind] ?? []), ...common]);
  return Object.keys(effect).filter((key) => !known.has(key));
}

const BuffConditionsEditor = defineComponent({
  name: 'BuffConditionsEditor',
  components: { NumberInput, SearchSelect, StringListEditor },
  props: {
    modelValue: { type: Array<unknown>, default: () => [] },
    buffOptions: { type: Array<string>, default: () => [] },
    weaponClassOptions: { type: Array<string>, default: () => [] },
    skillCategoryOptions: { type: Array<string>, default: () => [] },
    equipmentOptions: { type: Array<string>, default: () => [] },
    conditionKindOptions: { type: Array<string>, default: () => BUFF_CONDITION_KINDS }
  },
  emits: ['update:modelValue'],
  setup(props, { emit }) {
    const eventChecked = (event: Event) => (event.target as HTMLInputElement).checked;
    const updateAt = (index: number, value: DataRow) => {
      const next = [...props.modelValue];
      next[index] = value;
      emit('update:modelValue', next);
    };
    const patchAt = (index: number, patch: DataRow) => {
      const current = isRecord(props.modelValue[index]) ? props.modelValue[index] : {};
      updateAt(index, { ...current, ...patch });
    };
    const changeKind = (index: number, kind: string) => updateAt(index, defaultBuffCondition(kind));
    const add = () => emit('update:modelValue', [...props.modelValue, defaultBuffCondition()]);
    const removeAt = (index: number) => {
      const next = [...props.modelValue];
      next.splice(index, 1);
      emit('update:modelValue', next);
    };
    const conditionKind = (condition: unknown) => {
      if (!isRecord(condition)) return 'self_hp_percent_lte';
      const kind = asString(condition.type || condition.kind);
      if (kind) return kind;
      return asArray(condition.damageTypes).length > 0 ? '__damage_types__' : 'self_hp_percent_lte';
    };
    return { BATTLE_HOOK_BUFF_KINDS, DAMAGE_TYPES, conditionKind, asStringList, eventChecked, patchAt, changeKind, add, removeAt };
  },
  template: `
    <div class="buff-rule-editor">
      <article class="nested-card" v-for="(condition, index) in modelValue" :key="index">
        <div class="nested-head">
          <strong>条件 #{{ index + 1 }}</strong>
          <SearchSelect :model-value="conditionKind(condition)" :options="conditionKindOptions" :allow-empty="false" @update:model-value="(value) => changeKind(index, value)" />
          <button class="danger" @click="removeAt(index)">删除</button>
        </div>
        <div class="form-grid cols-2">
          <template v-if="conditionKind(condition) === 'wave_interval_eq'">
            <label>每N波触发（除数）<NumberInput :model-value="Number(condition.divisor ?? 3)" @update:model-value="(value) => patchAt(index, { divisor: Math.max(1, Math.trunc(Number(value) || 3)) })" /></label>
            <label>余数（0=整除触发）<NumberInput :model-value="Number(condition.value ?? 0)" @update:model-value="(value) => patchAt(index, { value: Math.max(0, Math.trunc(Number(value) || 0)) })" /></label>
          </template>
          <label v-else-if="conditionKind(condition) === 'has_buff' || conditionKind(condition) === 'source_has_buff' || conditionKind(condition) === 'target_has_buff'">Buff
            <SearchSelect :model-value="String(condition.buffId || '')" :options="buffOptions" @update:model-value="(value) => patchAt(index, { buffId: value })" />
          </label>
          <label v-else-if="conditionKind(condition) === 'self_weapon_class' || conditionKind(condition) === 'source_weapon_class'" class="wide">武器类别
            <StringListEditor :model-value="asStringList(condition.weaponClasses ?? condition.weaponClass)" :options="weaponClassOptions" link-table="weapon_classes" @update:model-value="(value) => patchAt(index, { weaponClasses: value, weaponClass: undefined })" />
          </label>
          <label v-else-if="conditionKind(condition) === 'self_position' || conditionKind(condition) === 'ally_side_has_positions'" class="wide">站位
            <StringListEditor :model-value="asStringList(condition.positions ?? condition.position)" :options="POSITIONS" @update:model-value="(value) => patchAt(index, { positions: value, position: undefined })" />
          </label>
          <template v-else-if="conditionKind(condition).startsWith('self_buff_count_')">
            <label>状态ID<SearchSelect :model-value="String(condition.buffId || '')" :options="buffOptions" @update:model-value="(value) => patchAt(index, { buffId: value })" /></label>
            <label>阈值<NumberInput :model-value="condition.value ?? 1" @update:model-value="(value) => patchAt(index, { value })" /></label>
          </template>
          <template v-else-if="conditionKind(condition) === 'target_buff_count_gte'">
            <label>目标状态ID<SearchSelect :model-value="String(condition.buffId || '')" :options="buffOptions" @update:model-value="(value) => patchAt(index, { buffId: value })" /></label>
            <label>层数至少<NumberInput :model-value="condition.value ?? 3" @update:model-value="(value) => patchAt(index, { value: Math.max(1, Math.trunc(Number(value) || 1)) })" /></label>
          </template>
          <template v-else-if="conditionKind(condition) === 'target_distinct_source_buff_count_gte'">
            <label>状态类别<SearchSelect :model-value="String(condition.buffKind || 'debuff')" :options="BATTLE_HOOK_BUFF_KINDS" :allow-empty="false" @update:model-value="(value) => patchAt(index, { buffKind: value })" /></label>
            <label>来自自身的不同状态数至少<NumberInput :model-value="condition.value ?? 3" @update:model-value="(value) => patchAt(index, { value: Math.max(1, Math.trunc(Number(value) || 1)) })" /></label>
          </template>
          <template v-else-if="conditionKind(condition) === 'self_skill_cooldown_count_gte'">
            <label>冷却中技能数至少<NumberInput :model-value="Number(condition.value ?? 2)" @update:model-value="(value) => patchAt(index, { value: Math.max(1, Math.trunc(Number(value) || 1)) })" /></label>
            <label class="checkbox-field"><input type="checkbox" :checked="Boolean(condition.includeBasicAttack ?? condition.includeBasic)" @change="patchAt(index, { includeBasicAttack: eventChecked($event), includeBasic: undefined })" />包含普通攻击</label>
            <label class="wide">技能类型（空=全部）<StringListEditor :model-value="asStringList(condition.skillCategories)" :options="skillCategoryOptions" link-table="skill_type" link-label="技能类型" @update:model-value="(value) => patchAt(index, { skillCategories: value })" /></label>
          </template>
          <label v-else-if="conditionKind(condition) === 'equipment_enhance_level_gte'">装备强化等级至少<NumberInput :model-value="Number(condition.value ?? 40)" @update:model-value="(value) => patchAt(index, { value: Math.max(0, Math.trunc(Number(value) || 0)) })" /></label>
          <template v-else-if="conditionKind(condition) === 'player_party_equipped_unit_count_eq'">
            <label>装备<SearchSelect :model-value="String(condition.itemId || '')" :options="equipmentOptions" @update:model-value="(value) => patchAt(index, { itemId: value })" /></label>
            <label>玩家单位装备数等于<NumberInput :model-value="Number(condition.value ?? 1)" @update:model-value="(value) => patchAt(index, { value: Math.max(0, Math.trunc(Number(value) || 0)) })" /></label>
          </template>
          <label v-else-if="conditionKind(condition) === '__damage_types__'" class="wide">伤害类型
            <StringListEditor :model-value="Array.isArray(condition.damageTypes) ? condition.damageTypes : []" :options="DAMAGE_TYPES" @update:model-value="(value) => patchAt(index, { type: undefined, kind: undefined, damageTypes: value })" />
          </label>
          <label v-else>阈值<NumberInput :model-value="condition.value ?? 0" @update:model-value="(value) => patchAt(index, { value })" /></label>
          <label v-if="conditionKind(condition) !== '__damage_types__'" class="wide">伤害类型过滤（可选）
            <StringListEditor :model-value="Array.isArray(condition.damageTypes) ? condition.damageTypes : []" :options="DAMAGE_TYPES" @update:model-value="(value) => patchAt(index, { damageTypes: value })" />
          </label>
        </div>
      </article>
      <button class="secondary" @click="add">新增条件</button>
    </div>
  `
});

const BuffDslEffectsEditor = defineComponent({
  name: 'BuffDslEffectsEditor',
  components: { JsonFieldEditor, NumberInput, SearchSelect, FormulaEditor, StringListEditor },
  props: {
    modelValue: { type: Array<unknown>, default: () => [] },
    attrOptions: { type: Array<string>, default: () => [] },
    buffOptions: { type: Array<string>, default: () => [] },
    skillOptions: { type: Array<string>, default: () => [] },
    skillCategoryOptions: { type: Array<string>, default: () => [] },
    enemyOptions: { type: Array<string>, default: () => [] }
  },
  emits: ['update:modelValue'],
  setup(props, { emit }) {
    const eventChecked = (event: Event) => (event.target as HTMLInputElement).checked;
    const eventValue = (event: Event) => (event.target as HTMLInputElement | HTMLSelectElement).value;
    const updateAt = (index: number, value: DataRow) => {
      const next = [...props.modelValue];
      next[index] = value;
      emit('update:modelValue', next);
    };
    const patchAt = (index: number, patch: DataRow) => {
      const current = isRecord(props.modelValue[index]) ? props.modelValue[index] : {};
      updateAt(index, { ...current, ...patch });
    };
    const changeKind = (index: number, kind: string) => {
      const current = isRecord(props.modelValue[index]) ? props.modelValue[index] : {};
      updateAt(index, patchKnownBuffEffect(kind, current));
    };
    const add = (kind = 'add_stat') => emit('update:modelValue', [...props.modelValue, defaultBuffDslEffect(kind)]);
    const removeAt = (index: number) => {
      const next = [...props.modelValue];
      next.splice(index, 1);
      emit('update:modelValue', next);
    };
    const effectKind = (effect: unknown) => isRecord(effect) ? asString(effect.kind || effect.type, 'add_stat') : 'add_stat';
    const extraKeys = (effect: unknown) => isRecord(effect) ? unknownBuffEffectKeys(effect) : [];
    const patchExtra = (index: number, key: string, value: unknown) => patchAt(index, { [key]: value });
    return { BUFF_EFFECT_KINDS, BUFF_RESOURCE_OPTIONS, ENEMY_REFRESH_POOL_OPTIONS, DAMAGE_TYPES, POSITIONS, SIDE_OPTIONS, BUFF_HOOK_TARGET_MODES, SKILL_COOLDOWN_SELECTION_MODES, skillOptions: props.skillOptions, enemyOptions: props.enemyOptions, eventChecked, eventValue, effectKind, extraKeys, patchAt, patchExtra, changeKind, add, removeAt };
  },
  template: `
    <div class="buff-rule-editor">
      <article class="nested-card" v-for="(effect, index) in modelValue" :key="index">
        <div class="nested-head">
          <strong>效果 #{{ index + 1 }}</strong>
          <SearchSelect :model-value="effectKind(effect)" :options="BUFF_EFFECT_KINDS" :allow-empty="false" @update:model-value="(value) => changeKind(index, value)" />
          <button class="danger" @click="removeAt(index)">删除</button>
        </div>
        <div class="form-grid cols-2">
          <template v-if="effectKind(effect) === 'add_stat' || effectKind(effect) === 'add_stat_percent'">
            <label>属性<SearchSelect :model-value="String(effect.attr || effect.stat || '')" :options="attrOptions" @update:model-value="(value) => patchAt(index, { attr: value })" /></label>
            <label>{{ effectKind(effect) === 'add_stat_percent' ? '百分比' : '数值' }}<NumberInput :model-value="effect.percent ?? effect.value ?? 0" @update:model-value="(value) => patchAt(index, { value })" /></label>
          </template>
          <template v-else-if="effectKind(effect) === 'damage'">
            <label>伤害类型<SearchSelect :model-value="String(effect.damageType || 'physical')" :options="DAMAGE_TYPES" :allow-empty="false" @update:model-value="(value) => patchAt(index, { damageType: value })" /></label>
            <label>固定值（可选）<NumberInput :model-value="effect.value ?? 0" @update:model-value="(value) => patchAt(index, { value })" /></label>
            <label class="wide">数值公式<FormulaEditor :model-value="effect.amountFormula || {}" :attr-options="attrOptions" @update:model-value="(value) => patchAt(index, { amountFormula: value })" /></label>
          </template>
          <template v-else-if="effectKind(effect) === 'heal'">
            <label>治疗量<NumberInput :model-value="effect.value ?? 0" @update:model-value="(value) => patchAt(index, { value })" /></label>
          </template>
          <template v-else-if="effectKind(effect) === 'resource'">
            <label>资源<SearchSelect :model-value="String(effect.resource || 'mp')" :options="BUFF_RESOURCE_OPTIONS" :allow-empty="false" @update:model-value="(value) => patchAt(index, { resource: value })" /></label>
            <label>数值<NumberInput :model-value="effect.value ?? 0" @update:model-value="(value) => patchAt(index, { value })" /></label>
            <label>数值尺度<SearchSelect :model-value="String(effect.valueScale || '')" :options="['max_resource']" @update:model-value="(value) => patchAt(index, { valueScale: value || undefined })" /></label>
            <label v-if="String(effect.resource || 'mp') === 'hp'">最低生命<NumberInput :model-value="effect.minimumHp ?? 0" @update:model-value="(value) => patchAt(index, { minimumHp: Math.max(0, Number(value) || 0) })" /></label>
            <label>Payload 数值 Key<input :value="String(effect.payloadValueKey || '')" @input="(event) => patchAt(index, { payloadValueKey: eventValue(event) || undefined })" /></label>
            <label>Payload 系数<NumberInput :model-value="effect.payloadValueCoeff ?? 1" @update:model-value="(value) => patchAt(index, { payloadValueCoeff: value })" /></label>
          </template>
          <template v-else-if="effectKind(effect) === 'add_skill_cooldown' || effectKind(effect) === 'adjust_skill_cooldown'">
            <label>{{ effectKind(effect) === 'add_skill_cooldown' ? '追加冷却回合' : '冷却调整回合' }}<NumberInput :model-value="effect.turns ?? effect.value ?? (effectKind(effect) === 'add_skill_cooldown' ? 2 : -1)" @update:model-value="(value) => patchAt(index, { value: effectKind(effect) === 'add_skill_cooldown' ? Math.max(0, Math.trunc(Number(value) || 0)) : Math.trunc(Number(value) || 0) })" /></label>
            <label class="checkbox-row"><input type="checkbox" :checked="Boolean(effect.includeBasic || effect.includeBasicAttack)" @change="(event) => patchAt(index, { includeBasic: eventChecked(event) })" /> 包含普攻</label>
            <label class="wide">指定技能（空=全部非普攻）<StringListEditor :model-value="Array.isArray(effect.skillIds) ? effect.skillIds : []" :options="skillOptions" @update:model-value="(value) => patchAt(index, { skillIds: value })" /></label>
            <label class="wide">排除技能<StringListEditor :model-value="Array.isArray(effect.excludeSkillIds) ? effect.excludeSkillIds : []" :options="skillOptions" @update:model-value="(value) => patchAt(index, { excludeSkillIds: value })" /></label>
            <label class="wide">技能类型（空=全部）<StringListEditor :model-value="Array.isArray(effect.skillCategories) ? effect.skillCategories : []" :options="skillCategoryOptions" link-table="skill_type" link-label="技能类型" @update:model-value="(value) => patchAt(index, { skillCategories: value })" /></label>
            <label>选择策略<SearchSelect :model-value="String(effect.selectionMode || 'all')" :options="SKILL_COOLDOWN_SELECTION_MODES" :allow-empty="false" @update:model-value="(value) => patchAt(index, { selectionMode: value })" /></label>
            <label>最多影响技能数（0=不限）<NumberInput :model-value="Number(effect.maxSkills || 0)" @update:model-value="(value) => patchAt(index, { maxSkills: Math.max(0, Math.trunc(Number(value) || 0)) })" /></label>
          </template>
          <template v-else-if="effectKind(effect) === 'add_buff'">
            <label>Buff<SearchSelect :model-value="String(effect.buffId || '')" :options="buffOptions" @update:model-value="(value) => patchAt(index, { buffId: value })" /></label>
            <label>持续回合<NumberInput :model-value="effect.durationTurns ?? 1" @update:model-value="(value) => patchAt(index, { durationTurns: value })" /></label>
            <label>目标<SearchSelect :model-value="String(effect.targetMode || 'self')" :options="BUFF_HOOK_TARGET_MODES" :allow-empty="false" @update:model-value="(value) => patchAt(index, { targetMode: value })" /></label>
            <label class="wide">Payload<JsonFieldEditor :model-value="effect.payload || {}" :path="['payload']" @update:model-value="(value) => patchAt(index, { payload: value })" /></label>
          </template>
          <template v-else-if="effectKind(effect) === 'remove_buff'">
            <label>Buff<SearchSelect :model-value="String(effect.buffId || '')" :options="buffOptions" @update:model-value="(value) => patchAt(index, { buffId: value })" /></label>
          </template>
          <template v-else-if="['reduce_damage_percent', 'set_damage_multiplier', 'add_damage_percent', 'add_damage_flat', 'add_damage'].includes(effectKind(effect))">
            <label>数值<NumberInput :model-value="effect.percent ?? effect.value ?? 0" @update:model-value="(value) => patchAt(index, { value })" /></label>
          </template>
          <template v-else-if="effectKind(effect) === 'position_override'">
            <label>站位<SearchSelect :model-value="String(effect.position || 'vanguard')" :options="POSITIONS" :allow-empty="false" @update:model-value="(value) => patchAt(index, { position: value })" /></label>
          </template>
          <template v-else-if="effectKind(effect) === 'position_map'">
            <label>前排映射<SearchSelect :model-value="String(effect.mapping?.vanguard || 'vanguard')" :options="POSITIONS" :allow-empty="false" @update:model-value="(value) => patchAt(index, { mapping: { ...(effect.mapping || {}), vanguard: value } })" /></label>
            <label>中排映射<SearchSelect :model-value="String(effect.mapping?.midguard || 'midguard')" :options="POSITIONS" :allow-empty="false" @update:model-value="(value) => patchAt(index, { mapping: { ...(effect.mapping || {}), midguard: value } })" /></label>
            <label>后排映射<SearchSelect :model-value="String(effect.mapping?.rearguard || 'rearguard')" :options="POSITIONS" :allow-empty="false" @update:model-value="(value) => patchAt(index, { mapping: { ...(effect.mapping || {}), rearguard: value } })" /></label>
          </template>
          <template v-else-if="effectKind(effect) === 'advance_position'">
            <label>前移格数<NumberInput :model-value="effect.steps ?? 1" @update:model-value="(value) => patchAt(index, { steps: Math.max(1, Math.trunc(Number(value) || 1)) })" /></label>
          </template>
          <template v-else-if="effectKind(effect) === 'side_override'">
            <label>半场<SearchSelect :model-value="String(effect.side || 'enemy')" :options="SIDE_OPTIONS" :allow-empty="false" @update:model-value="(value) => patchAt(index, { side: value })" /></label>
          </template>
          <template v-else-if="effectKind(effect) === 'damage_redirect_to_source'">
            <label>转移比例<NumberInput :model-value="effect.percent ?? effect.value ?? 1" @update:model-value="(value) => patchAt(index, { percent: value })" /></label>
          </template>
          <template v-else-if="effectKind(effect) === 'trigger_synced_summon_turn'">
            <label class="wide">同步召唤模板（空=全部）<StringListEditor :model-value="Array.isArray(effect.includeSummonTemplateIds) ? effect.includeSummonTemplateIds : []" :options="enemyOptions" link-table="enemies" link-label="敌人/召唤模板" @update:model-value="(value) => patchAt(index, { includeSummonTemplateIds: value })" /></label>
          </template>
          <template v-else-if="effectKind(effect) === 'extra_enemy_refresh'">
            <label>敌人池<SearchSelect :model-value="String(effect.enemyPool || 'current_world')" :options="ENEMY_REFRESH_POOL_OPTIONS" :allow-empty="false" @update:model-value="(value) => patchAt(index, { enemyPool: value })" /></label>
            <label>额外数量<NumberInput :model-value="Number(effect.count || 1)" @update:model-value="(value) => patchAt(index, { count: Math.max(1, Math.trunc(Number(value) || 1)) })" /></label>
          </template>
          <p v-else class="empty-text">该效果不需要额外参数。</p>
          <label>延迟毫秒<NumberInput :model-value="effect.delayMs ?? 0" @update:model-value="(value) => patchAt(index, { delayMs: value || undefined })" /></label>
          <label v-for="key in extraKeys(effect)" :key="key" class="wide">{{ key }}<JsonFieldEditor :model-value="effect[key]" :path="[key]" @update:model-value="(value) => patchExtra(index, key, value)" /></label>
        </div>
      </article>
      <div class="toolbar">
        <button class="secondary" @click="add('add_stat')">新增属性加成</button>
        <button class="secondary" @click="add('add_stat_percent')">新增属性百分比</button>
        <button class="secondary" @click="add('damage')">新增伤害</button>
        <button class="secondary" @click="add('heal')">新增治疗</button>
        <button class="secondary" @click="add('add_buff')">新增施加状态</button>
        <button class="secondary" @click="add('remove_buff')">新增移除状态</button>
        <button class="secondary" @click="add('adjust_skill_cooldown')">新增冷却调整</button>
        <button class="secondary" @click="add('extra_enemy_refresh')">新增额外敌人刷新</button>
      </div>
    </div>
  `
});

const BattleHooksEditor = defineComponent({
  name: 'BattleHooksEditor',
  components: { TargetingEditor, SearchSelect, NumberInput, StringListEditor, BuffConditionsEditor, JsonFieldEditor, FormulaEditor, SummonStatsFormulaEditor, NumberMapEditor },
  props: {
    modelValue: { type: Array<unknown>, default: () => [] },
    buffOptions: { type: Array<string>, default: () => [] },
    attrOptions: { type: Array<string>, default: () => [] },
    enemyOptions: { type: Array<string>, default: () => [] },
    skillOptions: { type: Array<string>, default: () => [] },
    skillCategoryOptions: { type: Array<string>, default: () => [] },
    weaponClassOptions: { type: Array<string>, default: () => [] },
    equipmentOptions: { type: Array<string>, default: () => [] }
  },
  emits: ['update:modelValue'],
  setup(props, { emit }) {
    const defaultBattleHookEffect = (kind = 'add_buff', damageType = 'physical'): DataRow => {
      if (kind === 'remove_buff') return { kind, buffId: '', targetMode: 'self', removeStack: false };
      if (kind === 'remove_buff_by_kind') return { kind, buffKind: 'debuff', targetMode: 'source', dispellableOnly: true, selectionMode: 'max_remaining', maxBuffs: 1, procChanceBase: 1, procChanceMin: 0, procChanceMax: 1, procChanceFormula: { baseAdd: 0, terms: [] } };
      if (kind === 'damage') {
        return {
          kind,
          targetMode: 'targeting',
          damageType,
          targeting: { targetPositions: ['vanguard'], secondarySeek: 'front_to_rear', targetScope: 'enemy', selectRule: 'first', selectAttrKey: '', fallbackSelfIfEmpty: false, targetCount: { base: 1, formula: [], max: 1 } },
          amountFormula: { baseAdd: 0, terms: [] },
          procChanceBase: 1,
          dispatchEquipmentHooks: true
        };
      }
      if (kind === 'heal') {
        return {
          kind,
          targetMode: 'targeting',
          targeting: { targetPositions: ['vanguard', 'midguard', 'rearguard'], secondarySeek: 'front_to_rear', targetScope: 'ally', selectRule: 'all', selectAttrKey: '', fallbackSelfIfEmpty: false, targetCount: { base: 99, formula: [] } },
          amountFormula: { baseAdd: 8, terms: [] }
        };
      }
      if (kind === 'consume_mp') {
        return { kind, amountFormula: { baseAdd: 0, terms: [] }, requireFullAmount: false, abortHookOnFail: false };
      }
      if (kind === 'adjust_skill_cooldown') {
        return { kind, amount: -1, targetMode: 'self', includeBasic: false, skillIds: [], excludeSkillIds: [], skillCategories: [], selectionMode: 'all', maxSkills: 0 };
      }
      if (kind === 'transfer_skill_cooldown') {
        const skillCategories = ['arcane', 'perception', 'snowpact', 'holy', 'holy_cat'];
        return {
          kind,
          amount: 1,
          targetMode: 'self',
          requireBoth: true,
          from: { includeBasic: false, skillCategories, excludeContextSkill: true, requireCooling: true, selectionMode: 'max_remaining', maxSkills: 1 },
          to: { includeBasic: false, skillCategories, selectionMode: 'context_skill' },
          waiveIncreaseBuffId: '',
          consumeWaiverBuff: true
        };
      }
      if (kind === 'extra_turn') {
        return { kind, targetMode: 'self', turnMode: 'full' };
      }
      if (kind === 'mirror_cast_skill') {
        return {
          kind,
          triggerScope: 'all',
          targetingMode: 'reselect',
          amountRatioBase: 0.5,
          amountRatioPerEnhanceLevel: 0,
          mpCostMultiplier: 0.67,
          hpCostMultiplier: 1,
          includeBasicAttack: false,
          allowMirroredCastTrigger: false,
          applyCooldown: true,
          skillIds: [],
          skillCategories: []
        };
      }
      if (kind === 'execute') {
        return { kind, hpPercentThreshold: 0.10, hpPercentPerEnhanceLevel: 0.005, targetMode: 'source' };
      }
      if (kind === 'runtime_damage_amp' || kind === 'add_runtime_damage_amp' || kind === 'add_damage_amp') {
        return { kind, value: 0.1, valueFormula: { baseAdd: 0, terms: [] }, valueMin: 0, valueMax: 1, damageTypes: [damageType] };
      }
      if (kind === 'basic_attack_marked_target') {
        return { kind, procChanceBase: 0.5, procChanceMin: 0, procChanceMax: 1, procChanceFormula: { baseAdd: 0, terms: [] } };
      }
      if (kind === 'extra_enemy_refresh') {
        return {
          kind,
          enemyPool: 'current_wave_boss',
          countFormula: {
            baseAdd: 1,
            terms: [{ attrKey: 'equipment_enhance_level', coefficient: 0.05, perSkillLevel: 0 }]
          },
          maxCount: 3,
          partyStackMode: 'highest',
          settlementMode: 'normal'
        };
      }
      if (kind === 'add_probability' || kind === 'set_probability') {
        return {
          kind,
          value: kind === 'set_probability' ? 1 : 0.1,
          valueFormula: { baseAdd: 0, terms: [] },
          probabilityKinds: [],
          sourceIds: [],
          excludeSourceIds: []
        };
      }
      if (kind === 'force_probability_failure') {
        return { kind, probabilityKinds: [], sourceIds: [], excludeSourceIds: [] };
      }
      if (kind === 'summon') {
        return {
          kind,
          targetMode: 'self',
          summonPayload: {
            unitId: '',
            side: 'player',
            countFormula: { baseAdd: 1, terms: [] },
            durationFormula: { baseAdd: 0, terms: [] },
            durationTurns: 0,
            maxCountPerUnit: 0,
            persistAcrossBattles: false,
            onCasterDeath: 'keep_until_duration_end',
            onCapOverflow: 'replace_oldest',
            onDispel: 'despawn_immediately',
            onDurationEnd: 'despawn_immediately',
            statsFormula: {}
          }
        };
      }
      if (kind === 'player_clone') {
        return {
          kind,
          durationTurns: 5,
          durationFormula: { baseAdd: 5, terms: [] },
          maxCountPerUnit: 1,
          persistAcrossBattles: false,
          onCasterDeath: 'keep_until_duration_end',
          onCapOverflow: 'replace_oldest',
          onDispel: 'despawn_immediately',
          onDurationEnd: 'despawn_immediately',
          inherit: { skills: true, basicAttack: true, effectMutations: true, stats: 'all' },
          statModifiers: {},
          statsFormula: {}
        };
      }
      return { kind: 'add_buff', buffId: '', durationTurns: 1, targetMode: 'self', procChanceBase: 1, procChanceMin: 0, procChanceMax: 1, procChanceFormula: { baseAdd: 0, terms: [] } };
    };
    const updateAt = (index: number, value: DataRow) => {
      const next = [...props.modelValue];
      next[index] = value;
      emit('update:modelValue', next);
    };
    const patchAt = (index: number, patch: DataRow) => {
      const current = isRecord(props.modelValue[index]) ? props.modelValue[index] : {};
      updateAt(index, { ...current, ...patch });
    };
    const effects = (hook: unknown) => isRecord(hook) ? asArray(hook.effects).filter(isRecord) : [];
    const patchEffect = (hookIndex: number, effectIndex: number, patch: DataRow) => {
      const hook = isRecord(props.modelValue[hookIndex]) ? props.modelValue[hookIndex] : {};
      const nextEffects = effects(hook).map((effect) => ({ ...effect }));
      nextEffects[effectIndex] = { ...(nextEffects[effectIndex] ?? {}), ...patch };
      patchAt(hookIndex, { effects: nextEffects });
    };
    const effectKind = (effect: unknown) => isRecord(effect) ? asString(effect.kind || effect.type, 'add_buff') : 'add_buff';
    const isProbabilityTrigger = (trigger: unknown) => ['on_before_probability_check', 'on_probability_success', 'on_probability_failure'].includes(asString(trigger));
    const isProbabilityModifier = (effect: unknown) => ['add_probability', 'set_probability', 'force_probability_failure'].includes(effectKind(effect));
    const usesProcProbability = (effect: unknown) => isRecord(effect) && (
      effect.procChanceBase !== undefined
      || effect.procChanceFormula !== undefined
      || effect.probabilityGroup !== undefined
      || effect.maxSuccessesPerTurn !== undefined
      || effect.ignoreProbabilityHooks !== undefined
    );
    const relationDamageEnabled = (effect: unknown) => isRecord(effect) && isRecord(effect.amountFormulaByRelation);
    const toggleRelationDamage = (hookIndex: number, effectIndex: number, enabled: boolean) => {
      const hook = isRecord(props.modelValue[hookIndex]) ? props.modelValue[hookIndex] : {};
      const effect = effects(hook)[effectIndex] ?? {};
      if (enabled) {
        const formula = isRecord(effect.amountFormula) ? effect.amountFormula : { baseAdd: 0, terms: [] };
        patchEffect(hookIndex, effectIndex, {
          amountFormula: undefined,
          relationMode: 'team_side',
          amountFormulaByRelation: {
            ally: structuredClone(formula),
            enemy: structuredClone(formula)
          },
          simultaneous: true
        });
        return;
      }
      const formulas = isRecord(effect.amountFormulaByRelation) ? effect.amountFormulaByRelation : {};
      patchEffect(hookIndex, effectIndex, {
        amountFormula: isRecord(formulas.ally) ? structuredClone(formulas.ally) : { baseAdd: 0, terms: [] },
        amountFormulaByRelation: undefined,
        relationMode: undefined,
        simultaneous: undefined
      });
    };
    const hookFormulaAttrOptions = computed(() => {
      const next = [...props.attrOptions];
      if (!next.includes('equipment_enhance_level')) next.push('equipment_enhance_level');
      return next;
    });
    const hasMaxCountFormula = (effect: DataRow) => {
      const payload = isRecord(effect.summonPayload) ? effect.summonPayload : {};
      return isRecord(payload.maxCountFormula);
    };
    const changeEffectKind = (hookIndex: number, effectIndex: number, kind: string) => patchEffect(hookIndex, effectIndex, defaultBattleHookEffect(kind));
    const addEffect = (hookIndex: number, kind = 'add_buff', damageType = 'physical') => {
      const hook = isRecord(props.modelValue[hookIndex]) ? props.modelValue[hookIndex] : {};
      patchAt(hookIndex, { effects: [...effects(hook), defaultBattleHookEffect(kind, damageType)] });
    };
    const removeEffect = (hookIndex: number, effectIndex: number) => {
      const hook = isRecord(props.modelValue[hookIndex]) ? props.modelValue[hookIndex] : {};
      const nextEffects = effects(hook);
      nextEffects.splice(effectIndex, 1);
      patchAt(hookIndex, { effects: nextEffects });
    };
    const patchActivationCost = (hookIndex: number, key: 'maxHpRatio' | 'maxMpRatio', rawValue: unknown) => {
      const hook = isRecord(props.modelValue[hookIndex]) ? props.modelValue[hookIndex] : {};
      const current = isRecord(hook.activationCost) ? hook.activationCost : {};
      const next = { ...current, [key]: Math.max(0, Math.min(1, Number(rawValue) || 0)) };
      patchAt(hookIndex, {
        activationCost: Number(next.maxHpRatio || 0) > 0 || Number(next.maxMpRatio || 0) > 0 ? next : undefined
      });
    };
    const add = () => emit('update:modelValue', [...props.modelValue, { trigger: 'on_battle_start', priority: 0, conditions: [], effects: [defaultBattleHookEffect('add_buff')] }]);
    const removeAt = (index: number) => {
      const next = [...props.modelValue];
      next.splice(index, 1);
      emit('update:modelValue', next);
    };
    const eventChecked = (event: Event) => (event.target as HTMLInputElement).checked;
    const eventValue = (event: Event) => (event.target as HTMLInputElement | HTMLSelectElement).value;
    return {
      BATTLE_HOOK_TRIGGERS,
      BATTLE_HOOK_TARGET_MODES,
      BATTLE_HOOK_EFFECT_KINDS,
      BATTLE_HOOK_CONDITION_KINDS,
      BATTLE_HOOK_BUFF_KINDS,
      BATTLE_HOOK_UNIT_FILTER_TYPES,
      BATTLE_HOOK_APPLICATION_KINDS,
      BATTLE_HOOK_TURN_KINDS,
      BATTLE_PROBABILITY_KINDS,
      BATTLE_ENEMY_REFRESH_POOL_OPTIONS,
      MIRROR_CAST_TRIGGER_SCOPES,
      MIRROR_CAST_TARGETING_MODES,
      SKILL_COOLDOWN_SELECTION_MODES,
      hookFormulaAttrOptions,
      hasMaxCountFormula,
      effects,
      effectKind,
      isProbabilityTrigger,
      isProbabilityModifier,
      usesProcProbability,
      relationDamageEnabled,
      toggleRelationDamage,
      asStringList,
      eventChecked,
      eventValue,
      patchAt,
      patchEffect,
      patchActivationCost,
      changeEffectKind,
      addEffect,
      removeEffect,
      add,
      removeAt,
      SUMMON_ON_CASTER_DEATH_OPTIONS,
      SUMMON_ON_CAP_OVERFLOW_OPTIONS,
      SUMMON_ON_DISPEL_OPTIONS,
      SUMMON_ON_DURATION_END_OPTIONS
    };
  },
  template: `
    <div class="buff-rule-editor">
      <article class="nested-card" v-for="(hook, index) in modelValue" :key="hook.id || index">
        <div class="nested-head">
          <strong>钩子 #{{ index + 1 }}</strong>
          <label class="checkbox-field" style="margin-left:auto;margin-right:8px"><input type="checkbox" :checked="Boolean(hook.hideDescription)" @change="patchAt(index, { hideDescription: eventChecked($event) })" />隐藏文案</label>
          <button class="danger" @click="removeAt(index)">删除</button>
        </div>
        <div class="form-grid cols-2">
          <label>触发器<SearchSelect :model-value="String(hook.trigger || 'on_battle_start')" :options="BATTLE_HOOK_TRIGGERS" @update:model-value="(value) => patchAt(index, { trigger: value })" /></label>
          <label>优先级<NumberInput :model-value="Number(hook.priority || 0)" @update:model-value="(value) => patchAt(index, { priority: value })" /></label>
          <label>每回合最多触发（0=不限）<NumberInput :model-value="Number(hook.maxTriggersPerTurn || 0)" @update:model-value="(value) => patchAt(index, { maxTriggersPerTurn: Number(value) > 0 ? Math.trunc(Number(value)) : undefined })" /></label>
          <label>每波最多触发（0=不限）<NumberInput :model-value="Number(hook.maxTriggersPerWave || 0)" @update:model-value="(value) => patchAt(index, { maxTriggersPerWave: Number(value) > 0 ? Math.trunc(Number(value)) : undefined })" /></label>
          <label>每场最多触发（0=不限）<NumberInput :model-value="Number(hook.maxTriggersPerBattle || 0)" @update:model-value="(value) => patchAt(index, { maxTriggersPerBattle: Number(value) > 0 ? Math.trunc(Number(value)) : undefined })" /></label>
          <label>自身行动冷却（0=无）<NumberInput :model-value="Number(hook.cooldownCompletedTurns || 0)" @update:model-value="(value) => patchAt(index, { cooldownCompletedTurns: Number(value) > 0 ? Math.trunc(Number(value)) : undefined })" /></label>
          <label>触发生命代价（最大生命比例）<NumberInput :model-value="Number(hook.activationCost?.maxHpRatio || 0)" @update:model-value="(value) => patchActivationCost(index, 'maxHpRatio', value)" /></label>
          <label>触发魔法代价（最大魔法比例）<NumberInput :model-value="Number(hook.activationCost?.maxMpRatio || 0)" @update:model-value="(value) => patchActivationCost(index, 'maxMpRatio', value)" /></label>
          <label class="wide">共享触发组<input :value="String(hook.sharedTriggerGroup || '')" @input="patchAt(index, { sharedTriggerGroup: eventValue($event) || undefined })" /></label>
          <label class="checkbox-field"><input type="checkbox" :checked="Boolean(hook.subjectFilter?.requireAlive)" @change="patchAt(index, { subjectFilter: { ...(hook.subjectFilter || {}), requireAlive: eventChecked($event) } })" />事件目标需存活</label>
          <label class="wide">事件目标包含类型<StringListEditor :model-value="asStringList(hook.subjectFilter?.includeUnitTypes)" :options="BATTLE_HOOK_UNIT_FILTER_TYPES" @update:model-value="(value) => patchAt(index, { subjectFilter: { ...(hook.subjectFilter || {}), includeUnitTypes: value } })" /></label>
          <label class="wide">事件目标排除类型<StringListEditor :model-value="asStringList(hook.subjectFilter?.excludeUnitTypes)" :options="BATTLE_HOOK_UNIT_FILTER_TYPES" @update:model-value="(value) => patchAt(index, { subjectFilter: { ...(hook.subjectFilter || {}), excludeUnitTypes: value } })" /></label>
          <label class="wide">包含召唤模板<StringListEditor :model-value="asStringList(hook.subjectFilter?.includeSummonTemplateIds)" :options="enemyOptions" link-table="enemies" link-label="敌人/召唤模板" @update:model-value="(value) => patchAt(index, { subjectFilter: { ...(hook.subjectFilter || {}), includeSummonTemplateIds: value } })" /></label>
          <label class="wide">排除召唤模板<StringListEditor :model-value="asStringList(hook.subjectFilter?.excludeSummonTemplateIds)" :options="enemyOptions" link-table="enemies" link-label="敌人/召唤模板" @update:model-value="(value) => patchAt(index, { subjectFilter: { ...(hook.subjectFilter || {}), excludeSummonTemplateIds: value } })" /></label>
          <template v-if="hook.trigger === 'on_after_apply_buff'">
            <label class="wide">状态事件过滤<StringListEditor :model-value="asStringList(hook.eventFilter?.buffIds)" :options="buffOptions" @update:model-value="(value) => patchAt(index, { eventFilter: { ...(hook.eventFilter || {}), buffIds: value } })" /></label>
            <label class="wide">排除状态<StringListEditor :model-value="asStringList(hook.eventFilter?.excludeBuffIds)" :options="buffOptions" @update:model-value="(value) => patchAt(index, { eventFilter: { ...(hook.eventFilter || {}), excludeBuffIds: value } })" /></label>
            <label class="wide">状态类别过滤<StringListEditor :model-value="asStringList(hook.eventFilter?.buffKinds)" :options="BATTLE_HOOK_BUFF_KINDS" @update:model-value="(value) => patchAt(index, { eventFilter: { ...(hook.eventFilter || {}), buffKinds: value } })" /></label>
            <label class="wide">施加结果过滤<StringListEditor :model-value="asStringList(hook.eventFilter?.applicationKinds)" :options="BATTLE_HOOK_APPLICATION_KINDS" @update:model-value="(value) => patchAt(index, { eventFilter: { ...(hook.eventFilter || {}), applicationKinds: value } })" /></label>
            <label class="checkbox-field"><input type="checkbox" :checked="Boolean(hook.eventFilter?.firstBuffInstanceOnly)" @change="patchAt(index, { eventFilter: { ...(hook.eventFilter || {}), firstBuffInstanceOnly: eventChecked($event) } })" />仅状态首层附着</label>
            <label class="checkbox-field"><input type="checkbox" :checked="Boolean(hook.eventFilter?.excludeSelf)" @change="patchAt(index, { eventFilter: { ...(hook.eventFilter || {}), excludeSelf: eventChecked($event) } })" />排除对自身施加</label>
          </template>
          <template v-if="hook.trigger === 'on_turn_start' || hook.trigger === 'on_turn_end'">
            <label class="wide">计入的行动类型<StringListEditor :model-value="asStringList(hook.eventFilter?.turnKinds)" :options="BATTLE_HOOK_TURN_KINDS" @update:model-value="(value) => patchAt(index, { eventFilter: { ...(hook.eventFilter || {}), turnKinds: value } })" /></label>
            <label class="checkbox-field"><input type="checkbox" :checked="hook.eventFilter?.includePreparation !== false" @change="patchAt(index, { eventFilter: { ...(hook.eventFilter || {}), includePreparation: eventChecked($event) } })" />计入准备回合</label>
            <label v-if="hook.trigger === 'on_turn_end'" class="checkbox-field"><input type="checkbox" :checked="hook.eventFilter?.includeSuppressedTurns !== false" @change="patchAt(index, { eventFilter: { ...(hook.eventFilter || {}), includeSuppressedTurns: eventChecked($event) } })" />计入受控跳过行动</label>
          </template>
          <template v-if="isProbabilityTrigger(hook.trigger)">
            <label class="wide">概率类别<StringListEditor :model-value="asStringList(hook.eventFilter?.probabilityKinds)" :options="BATTLE_PROBABILITY_KINDS" @update:model-value="(value) => patchAt(index, { eventFilter: { ...(hook.eventFilter || {}), probabilityKinds: value } })" /></label>
            <label class="wide">仅监听来源ID（空=全部）<StringListEditor :model-value="asStringList(hook.eventFilter?.sourceIds)" :options="[]" @update:model-value="(value) => patchAt(index, { eventFilter: { ...(hook.eventFilter || {}), sourceIds: value } })" /></label>
            <label class="wide">排除来源ID<StringListEditor :model-value="asStringList(hook.eventFilter?.excludeSourceIds)" :options="[]" @update:model-value="(value) => patchAt(index, { eventFilter: { ...(hook.eventFilter || {}), excludeSourceIds: value } })" /></label>
          </template>
          <label class="wide">触发条件<BuffConditionsEditor :model-value="hook.conditions || []" :buff-options="buffOptions" :weapon-class-options="weaponClassOptions" :skill-category-options="skillCategoryOptions" :equipment-options="equipmentOptions" :condition-kind-options="BATTLE_HOOK_CONDITION_KINDS" @update:model-value="(value) => patchAt(index, { conditions: value })" /></label>
        </div>
        <div class="nested-card" v-for="(effect, effectIndex) in effects(hook)" :key="effectIndex">
          <div class="nested-head">
            <strong>效果 #{{ effectIndex + 1 }}</strong>
            <button class="danger" @click="removeEffect(index, effectIndex)">删除</button>
          </div>
          <div class="form-grid cols-2">
            <label>效果类型<SearchSelect :model-value="effectKind(effect)" :options="BATTLE_HOOK_EFFECT_KINDS" :allow-empty="false" @update:model-value="(value) => changeEffectKind(index, effectIndex, value)" /></label>
            <template v-if="isProbabilityModifier(effect)">
              <template v-if="effectKind(effect) !== 'force_probability_failure'">
                <label>{{ effectKind(effect) === 'set_probability' ? '设置概率' : '概率增量' }}<NumberInput :model-value="Number(effect.value ?? (effectKind(effect) === 'set_probability' ? 1 : 0.1))" @update:model-value="(value) => patchEffect(index, effectIndex, { value })" /></label>
                <label class="wide">强化缩放公式<FormulaEditor :model-value="effect.valueFormula || { baseAdd: 0, terms: [] }" :attr-options="hookFormulaAttrOptions" :allow-enhance-scaling="true" @update:model-value="(value) => patchEffect(index, effectIndex, { valueFormula: value })" /></label>
              </template>
              <label class="wide">概率类别（空=继承钩子）<StringListEditor :model-value="asStringList(effect.probabilityKinds)" :options="BATTLE_PROBABILITY_KINDS" @update:model-value="(value) => patchEffect(index, effectIndex, { probabilityKinds: value })" /></label>
              <label class="wide">仅作用来源ID<StringListEditor :model-value="asStringList(effect.sourceIds)" :options="[]" @update:model-value="(value) => patchEffect(index, effectIndex, { sourceIds: value })" /></label>
              <label class="wide">排除来源ID<StringListEditor :model-value="asStringList(effect.excludeSourceIds)" :options="[]" @update:model-value="(value) => patchEffect(index, effectIndex, { excludeSourceIds: value })" /></label>
            </template>
            <template v-else-if="effectKind(effect) === 'add_buff' || effectKind(effect) === 'remove_buff'">
              <label>Buff<SearchSelect :model-value="String(effect.buffId || '')" :options="buffOptions" @update:model-value="(value) => patchEffect(index, effectIndex, { buffId: value })" /></label>
              <label v-if="effectKind(effect) === 'add_buff'">持续回合<NumberInput :model-value="Number(effect.durationTurns || 1)" @update:model-value="(value) => patchEffect(index, effectIndex, { durationTurns: value })" /></label>
              <label v-if="effectKind(effect) === 'add_buff'">基础概率<NumberInput :model-value="Number(effect.procChanceBase ?? 1)" @update:model-value="(value) => patchEffect(index, effectIndex, { procChanceBase: value })" /></label>
              <label v-if="effectKind(effect) === 'add_buff'">最小概率<NumberInput :model-value="Number(effect.procChanceMin ?? 0)" @update:model-value="(value) => patchEffect(index, effectIndex, { procChanceMin: value })" /></label>
              <label v-if="effectKind(effect) === 'add_buff'">最大概率<NumberInput :model-value="Number(effect.procChanceMax ?? 1)" @update:model-value="(value) => patchEffect(index, effectIndex, { procChanceMax: value })" /></label>
              <label v-if="effectKind(effect) === 'remove_buff'" class="checkbox-field"><input type="checkbox" :checked="Boolean(effect.removeStack)" @change="patchEffect(index, effectIndex, { removeStack: eventChecked($event) })" />仅移除一层（叠加Buff）</label>
              <label>目标模式<SearchSelect :model-value="String(effect.targetMode || 'self')" :options="BATTLE_HOOK_TARGET_MODES" @update:model-value="(value) => patchEffect(index, effectIndex, { targetMode: value })" /></label>
              <label v-if="effect.targetMode === 'targeting'" class="wide">目标<TargetingEditor :model-value="effect.targeting || {}" :attr-options="attrOptions" :buff-options="buffOptions" @update:model-value="(value) => patchEffect(index, effectIndex, { targeting: value })" /></label>
              <label v-if="effectKind(effect) === 'add_buff'" class="wide">概率公式<FormulaEditor :model-value="effect.procChanceFormula || { baseAdd: 0, terms: [] }" :attr-options="hookFormulaAttrOptions" @update:model-value="(value) => patchEffect(index, effectIndex, { procChanceFormula: value })" /></label>
            </template>
            <template v-else-if="effectKind(effect) === 'remove_buff_by_kind'">
              <label>状态类别<SearchSelect :model-value="String(effect.buffKind || 'debuff')" :options="BATTLE_HOOK_BUFF_KINDS" :allow-empty="false" @update:model-value="(value) => patchEffect(index, effectIndex, { buffKind: value })" /></label>
              <label>目标模式<SearchSelect :model-value="String(effect.targetMode || 'source')" :options="BATTLE_HOOK_TARGET_MODES" @update:model-value="(value) => patchEffect(index, effectIndex, { targetMode: value })" /></label>
              <label>选择策略<SearchSelect :model-value="String(effect.selectionMode || 'max_remaining')" :options="['max_remaining', 'id']" :allow-empty="false" @update:model-value="(value) => patchEffect(index, effectIndex, { selectionMode: value })" /></label>
              <label>最多移除状态数<NumberInput :model-value="Number(effect.maxBuffs || 1)" @update:model-value="(value) => patchEffect(index, effectIndex, { maxBuffs: Math.max(1, Math.trunc(Number(value) || 1)) })" /></label>
              <label class="checkbox-field"><input type="checkbox" :checked="effect.dispellableOnly !== false" @change="patchEffect(index, effectIndex, { dispellableOnly: eventChecked($event) })" />仅移除可驱散状态</label>
              <label>基础概率<NumberInput :model-value="Number(effect.procChanceBase ?? 1)" @update:model-value="(value) => patchEffect(index, effectIndex, { procChanceBase: value })" /></label>
              <label>最小概率<NumberInput :model-value="Number(effect.procChanceMin ?? 0)" @update:model-value="(value) => patchEffect(index, effectIndex, { procChanceMin: value })" /></label>
              <label>最大概率<NumberInput :model-value="Number(effect.procChanceMax ?? 1)" @update:model-value="(value) => patchEffect(index, effectIndex, { procChanceMax: value })" /></label>
              <label class="wide">概率公式<FormulaEditor :model-value="effect.procChanceFormula || { baseAdd: 0, terms: [] }" :attr-options="hookFormulaAttrOptions" @update:model-value="(value) => patchEffect(index, effectIndex, { procChanceFormula: value })" /></label>
            </template>
            <template v-else-if="effectKind(effect) === 'damage'">
              <label>目标模式<SearchSelect :model-value="String(effect.targetMode || 'targeting')" :options="BATTLE_HOOK_TARGET_MODES" @update:model-value="(value) => patchEffect(index, effectIndex, { targetMode: value })" /></label>
              <label v-if="effect.targetMode === 'targeting' || !effect.targetMode" class="wide">目标<TargetingEditor :model-value="effect.targeting || {}" :attr-options="attrOptions" :buff-options="buffOptions" @update:model-value="(value) => patchEffect(index, effectIndex, { targeting: value })" /></label>
              <label>伤害类型<SearchSelect :model-value="String(effect.damageType || 'physical')" :options="DAMAGE_TYPES" :allow-empty="false" @update:model-value="(value) => patchEffect(index, effectIndex, { damageType: value })" /></label>
              <label class="checkbox-field"><input type="checkbox" :checked="relationDamageEnabled(effect)" @change="toggleRelationDamage(index, effectIndex, eventChecked($event))" />按队伍关系分别计算</label>
              <template v-if="relationDamageEnabled(effect)">
                <label>关系判定<SearchSelect :model-value="String(effect.relationMode || 'team_side')" :options="['team_side']" :allow-empty="false" @update:model-value="(value) => patchEffect(index, effectIndex, { relationMode: value })" /></label>
                <label class="checkbox-field"><input type="checkbox" :checked="Boolean(effect.simultaneous)" @change="patchEffect(index, effectIndex, { simultaneous: eventChecked($event) })" />按同一时点批量结算</label>
                <label class="wide">友军伤害公式<FormulaEditor :model-value="effect.amountFormulaByRelation?.ally || { baseAdd: 0, terms: [] }" :attr-options="hookFormulaAttrOptions" :allow-enhance-scaling="true" @update:model-value="(value) => patchEffect(index, effectIndex, { amountFormulaByRelation: { ...(effect.amountFormulaByRelation || {}), ally: value } })" /></label>
                <label class="wide">敌军伤害公式<FormulaEditor :model-value="effect.amountFormulaByRelation?.enemy || { baseAdd: 0, terms: [] }" :attr-options="hookFormulaAttrOptions" :allow-enhance-scaling="true" @update:model-value="(value) => patchEffect(index, effectIndex, { amountFormulaByRelation: { ...(effect.amountFormulaByRelation || {}), enemy: value } })" /></label>
              </template>
              <label v-else class="wide">伤害公式<FormulaEditor :model-value="effect.amountFormula || { baseAdd: 0, terms: [] }" :attr-options="hookFormulaAttrOptions" :allow-enhance-scaling="true" @update:model-value="(value) => patchEffect(index, effectIndex, { amountFormula: value })" /></label>
              <label>基础概率（可选）<NumberInput :model-value="Number(effect.procChanceBase ?? 1)" @update:model-value="(value) => patchEffect(index, effectIndex, { procChanceBase: value })" /></label>
              <label class="checkbox-field"><input type="checkbox" :checked="effect.dispatchEquipmentHooks !== false" @change="patchEffect(index, effectIndex, { dispatchEquipmentHooks: eventChecked($event) })" />继续派发装备伤害钩子</label>
            </template>
            <template v-else-if="effectKind(effect) === 'heal'">
              <label>目标模式<SearchSelect :model-value="String(effect.targetMode || 'targeting')" :options="BATTLE_HOOK_TARGET_MODES" @update:model-value="(value) => patchEffect(index, effectIndex, { targetMode: value })" /></label>
              <label v-if="effect.targetMode === 'targeting' || !effect.targetMode" class="wide">目标<TargetingEditor :model-value="effect.targeting || {}" :attr-options="attrOptions" :buff-options="buffOptions" @update:model-value="(value) => patchEffect(index, effectIndex, { targeting: value })" /></label>
              <label class="wide">治疗量公式<FormulaEditor :model-value="effect.amountFormula || { baseAdd: 0, terms: [] }" :attr-options="hookFormulaAttrOptions" @update:model-value="(value) => patchEffect(index, effectIndex, { amountFormula: value })" /></label>
              <label>基础概率（可选）<NumberInput :model-value="Number(effect.procChanceBase ?? 1)" @update:model-value="(value) => patchEffect(index, effectIndex, { procChanceBase: value })" /></label>
            </template>
            <template v-else-if="effectKind(effect) === 'extra_enemy_refresh'">
              <label>敌人池<SearchSelect :model-value="String(effect.enemyPool || 'current_wave_boss')" :options="BATTLE_ENEMY_REFRESH_POOL_OPTIONS" :allow-empty="false" @update:model-value="(value) => patchEffect(index, effectIndex, { enemyPool: value })" /></label>
              <label>队伍叠加策略<SearchSelect :model-value="String(effect.partyStackMode || 'highest')" :options="['highest']" :allow-empty="false" @update:model-value="(value) => patchEffect(index, effectIndex, { partyStackMode: value })" /></label>
              <label>结算方式<SearchSelect :model-value="String(effect.settlementMode || 'normal')" :options="['normal']" :allow-empty="false" @update:model-value="(value) => patchEffect(index, effectIndex, { settlementMode: value })" /></label>
              <label>额外 Boss 上限<NumberInput :model-value="Number(effect.maxCount || 3)" @update:model-value="(value) => patchEffect(index, effectIndex, { maxCount: Math.max(1, Math.min(8, Math.trunc(Number(value) || 1))) })" /></label>
              <label class="wide">额外 Boss 数量公式<FormulaEditor :model-value="effect.countFormula || { baseAdd: 1, terms: [{ attrKey: 'equipment_enhance_level', coefficient: 0.05, perSkillLevel: 0 }] }" :attr-options="['equipment_enhance_level']" @update:model-value="(value) => patchEffect(index, effectIndex, { countFormula: value })" /></label>
              <p class="empty-text wide">在本波常规敌人创建完成后，复制已创建 Boss 的最终属性并直接加入敌方阵容；不是召唤物，参与普通经验与掉落结算。</p>
            </template>
            <template v-else-if="effectKind(effect) === 'summon'">
              <label>召唤单位<SearchSelect :model-value="String(effect.summonPayload?.unitId || effect.unitId || '')" :options="enemyOptions" :allow-empty="false" @update:model-value="(value) => patchEffect(index, effectIndex, { summonPayload: { ...(effect.summonPayload || {}), unitId: value } })" /></label>
              <label>召唤阵营<SearchSelect :model-value="String(effect.summonPayload?.side || effect.side || 'player')" :options="SIDE_OPTIONS" :allow-empty="false" @update:model-value="(value) => patchEffect(index, effectIndex, { summonPayload: { ...(effect.summonPayload || {}), side: value } })" /></label>
              <label>基础概率<NumberInput :model-value="Number(effect.procChanceBase ?? 1)" @update:model-value="(value) => patchEffect(index, effectIndex, { procChanceBase: value })" /></label>
              <label>最小概率<NumberInput :model-value="Number(effect.procChanceMin ?? 0)" @update:model-value="(value) => patchEffect(index, effectIndex, { procChanceMin: value })" /></label>
              <label>最大概率<NumberInput :model-value="Number(effect.procChanceMax ?? 1)" @update:model-value="(value) => patchEffect(index, effectIndex, { procChanceMax: value })" /></label>
              <label>目标模式<SearchSelect :model-value="String(effect.targetMode || 'self')" :options="BATTLE_HOOK_TARGET_MODES" @update:model-value="(value) => patchEffect(index, effectIndex, { targetMode: value })" /></label>
              <label v-if="effect.targetMode === 'targeting'" class="wide">目标<TargetingEditor :model-value="effect.targeting || {}" :attr-options="attrOptions" :buff-options="buffOptions" @update:model-value="(value) => patchEffect(index, effectIndex, { targeting: value })" /></label>
              <label class="wide">概率公式<FormulaEditor :model-value="effect.procChanceFormula || { baseAdd: 0, terms: [] }" :attr-options="hookFormulaAttrOptions" @update:model-value="(value) => patchEffect(index, effectIndex, { procChanceFormula: value })" /></label>
              <label>数量公式<FormulaEditor :model-value="effect.summonPayload?.countFormula || effect.countFormula || { baseAdd: 1, terms: [] }" :attr-options="hookFormulaAttrOptions" @update:model-value="(value) => patchEffect(index, effectIndex, { summonPayload: { ...(effect.summonPayload || {}), countFormula: value } })" /></label>
              <label>持续回合公式<FormulaEditor :model-value="effect.summonPayload?.durationFormula || { baseAdd: 0, terms: [] }" :attr-options="hookFormulaAttrOptions" @update:model-value="(value) => patchEffect(index, effectIndex, { summonPayload: { ...(effect.summonPayload || {}), durationFormula: value } })" /></label>
              <label>固定持续回合（兼容兜底）<NumberInput :model-value="Number(effect.summonPayload?.durationTurns || effect.durationTurns || 0)" @update:model-value="(value) => patchEffect(index, effectIndex, { summonPayload: { ...(effect.summonPayload || {}), durationTurns: Math.max(0, Math.trunc(Number(value) || 0)) } })" /></label>
              <label>固定数量上限（0=不限）<NumberInput :model-value="Number(effect.summonPayload?.maxCountPerUnit || effect.maxCountPerUnit || 0)" @update:model-value="(value) => patchEffect(index, effectIndex, { summonPayload: { ...(effect.summonPayload || {}), maxCountPerUnit: Math.max(0, Math.trunc(Number(value) || 0)) } })" /></label>
              <label class="checkbox-field"><input type="checkbox" :checked="hasMaxCountFormula(effect)" @change="patchEffect(index, effectIndex, { summonPayload: { ...(effect.summonPayload || {}), maxCountFormula: eventChecked($event) ? { baseAdd: 1, terms: [] } : undefined } })" />使用数量上限公式</label>
              <label v-if="hasMaxCountFormula(effect)" class="wide">数量上限公式（配置后优先）<FormulaEditor :model-value="effect.summonPayload?.maxCountFormula" :attr-options="hookFormulaAttrOptions" @update:model-value="(value) => patchEffect(index, effectIndex, { summonPayload: { ...(effect.summonPayload || {}), maxCountFormula: value } })" /></label>
              <label class="checkbox-field"><input type="checkbox" :checked="Boolean(effect.summonPayload?.persistAcrossBattles || effect.persistAcrossBattles)" @change="patchEffect(index, effectIndex, { summonPayload: { ...(effect.summonPayload || {}), persistAcrossBattles: eventChecked($event) } })" />跨战斗保留</label>
              <label>召唤者死亡策略<SearchSelect :model-value="String(effect.summonPayload?.onCasterDeath || 'keep_until_duration_end')" :options="SUMMON_ON_CASTER_DEATH_OPTIONS" :allow-empty="false" @update:model-value="(value) => patchEffect(index, effectIndex, { summonPayload: { ...(effect.summonPayload || {}), onCasterDeath: value } })" /></label>
              <label>上限溢出策略<SearchSelect :model-value="String(effect.summonPayload?.onCapOverflow || 'replace_oldest')" :options="SUMMON_ON_CAP_OVERFLOW_OPTIONS" :allow-empty="false" @update:model-value="(value) => patchEffect(index, effectIndex, { summonPayload: { ...(effect.summonPayload || {}), onCapOverflow: value } })" /></label>
              <label>驱散处理<SearchSelect :model-value="String(effect.summonPayload?.onDispel || 'despawn_immediately')" :options="SUMMON_ON_DISPEL_OPTIONS" :allow-empty="false" @update:model-value="(value) => patchEffect(index, effectIndex, { summonPayload: { ...(effect.summonPayload || {}), onDispel: value } })" /></label>
              <label>持续结束处理<SearchSelect :model-value="String(effect.summonPayload?.onDurationEnd || 'despawn_immediately')" :options="SUMMON_ON_DURATION_END_OPTIONS" :allow-empty="false" @update:model-value="(value) => patchEffect(index, effectIndex, { summonPayload: { ...(effect.summonPayload || {}), onDurationEnd: value } })" /></label>
              <label class="wide">属性增益公式（按属性）<SummonStatsFormulaEditor :model-value="effect.summonPayload?.statsFormula || effect.statsFormula || {}" :attr-options="attrOptions" :formula-attr-options="hookFormulaAttrOptions" @update:model-value="(value) => patchEffect(index, effectIndex, { summonPayload: { ...(effect.summonPayload || {}), statsFormula: value } })" /></label>
            </template>
            <template v-else-if="effectKind(effect) === 'player_clone'">
              <label>名称后缀（nameSuffix）<input type="text" :value="String(effect.nameSuffix || '')" @input="(e) => patchEffect(index, effectIndex, { nameSuffix: e.target.value })" /></label>
              <label>名称后缀i18n键（nameSuffixKey）<input type="text" :value="String(effect.nameSuffixKey || '')" @input="(e) => patchEffect(index, effectIndex, { nameSuffixKey: e.target.value })" /></label>
              <label>名称分隔符（默认" - "）<input type="text" :value="String(effect.nameSeparator || '')" @input="(e) => patchEffect(index, effectIndex, { nameSeparator: e.target.value })" placeholder=" - " /></label>
              <label>持续回合<NumberInput :model-value="Number(effect.durationTurns ?? 5)" @update:model-value="(value) => patchEffect(index, effectIndex, { durationTurns: Math.max(1, Math.trunc(Number(value) || 1)) })" /></label>
              <label>单单位上限（0=不限）<NumberInput :model-value="Number(effect.maxCountPerUnit ?? 1)" @update:model-value="(value) => patchEffect(index, effectIndex, { maxCountPerUnit: Math.max(0, Math.trunc(Number(value) || 0)) })" /></label>
              <label>持续回合公式<FormulaEditor :model-value="effect.durationFormula || { baseAdd: 5, terms: [] }" :attr-options="hookFormulaAttrOptions" @update:model-value="(value) => patchEffect(index, effectIndex, { durationFormula: value })" /></label>
              <label class="checkbox-field"><input type="checkbox" :checked="Boolean(effect.persistAcrossBattles)" @change="patchEffect(index, effectIndex, { persistAcrossBattles: eventChecked($event) })" />跨战斗保留</label>
              <label>召唤者死亡策略<SearchSelect :model-value="String(effect.onCasterDeath || 'keep_until_duration_end')" :options="SUMMON_ON_CASTER_DEATH_OPTIONS" :allow-empty="false" @update:model-value="(value) => patchEffect(index, effectIndex, { onCasterDeath: value })" /></label>
              <label>上限溢出策略<SearchSelect :model-value="String(effect.onCapOverflow || 'replace_oldest')" :options="SUMMON_ON_CAP_OVERFLOW_OPTIONS" :allow-empty="false" @update:model-value="(value) => patchEffect(index, effectIndex, { onCapOverflow: value })" /></label>
              <label>驱散处理<SearchSelect :model-value="String(effect.onDispel || 'despawn_immediately')" :options="SUMMON_ON_DISPEL_OPTIONS" :allow-empty="false" @update:model-value="(value) => patchEffect(index, effectIndex, { onDispel: value })" /></label>
              <label>持续结束处理<SearchSelect :model-value="String(effect.onDurationEnd || 'despawn_immediately')" :options="SUMMON_ON_DURATION_END_OPTIONS" :allow-empty="false" @update:model-value="(value) => patchEffect(index, effectIndex, { onDurationEnd: value })" /></label>
              <div class="form-grid cols-2 wide" style="grid-column:1/-1;border:1px solid #aaa;padding:8px;border-radius:4px">
                <div style="grid-column:1/-1;font-weight:600;font-size:0.9em">继承配置（留空 = 全量继承）</div>
                <label class="checkbox-field"><input type="checkbox" :checked="Boolean((effect.inherit ?? {}).skills !== false)" @change="patchEffect(index, effectIndex, { inherit: { ...(effect.inherit || {}), skills: eventChecked($event) } })" />继承技能</label>
                <label class="checkbox-field"><input type="checkbox" :checked="Boolean((effect.inherit ?? {}).basicAttack !== false)" @change="patchEffect(index, effectIndex, { inherit: { ...(effect.inherit || {}), basicAttack: eventChecked($event) } })" />继承基础攻击（武器）</label>
                <label class="checkbox-field"><input type="checkbox" :checked="Boolean((effect.inherit ?? {}).effectMutations !== false)" @change="patchEffect(index, effectIndex, { inherit: { ...(effect.inherit || {}), effectMutations: eventChecked($event) } })" />继承词条效果变异</label>
                <label>属性继承模式
                  <select :value="Array.isArray((effect.inherit ?? {}).stats) ? 'select' : (String((effect.inherit ?? {}).stats || 'all'))"
                    @change="(e) => { const v = e.target.value; patchEffect(index, effectIndex, { inherit: { ...(effect.inherit || {}), stats: v === 'select' ? [] : v } }) }">
                    <option value="all">全量继承</option>
                    <option value="none">不继承</option>
                    <option value="select">指定属性</option>
                  </select>
                </label>
                <label v-if="Array.isArray((effect.inherit ?? {}).stats)" class="wide">指定继承属性
                  <StringListEditor :model-value="Array.isArray((effect.inherit ?? {}).stats) ? (effect.inherit ?? {}).stats : []" :options="attrOptions" @update:model-value="(value) => patchEffect(index, effectIndex, { inherit: { ...(effect.inherit || {}), stats: value } })" />
                </label>
              </div>
              <label class="wide">固定属性修正（statModifiers）<NumberMapEditor :model-value="effect.statModifiers || {}" :options="attrOptions" @update:model-value="(value) => patchEffect(index, effectIndex, { statModifiers: value })" /></label>
              <label class="wide">公式属性修正（statsFormula，支持强化等级）<SummonStatsFormulaEditor :model-value="effect.statsFormula || {}" :attr-options="attrOptions" :formula-attr-options="hookFormulaAttrOptions" @update:model-value="(value) => patchEffect(index, effectIndex, { statsFormula: value })" /></label>
            </template>
            <template v-else-if="effectKind(effect) === 'mirror_cast_skill'">
              <label>监听范围（全部不含自身）<SearchSelect :model-value="String(effect.triggerScope || 'all')" :options="MIRROR_CAST_TRIGGER_SCOPES" :allow-empty="false" @update:model-value="(value) => patchEffect(index, effectIndex, { triggerScope: value })" /></label>
              <label>目标策略<SearchSelect :model-value="String(effect.targetingMode || 'reselect')" :options="MIRROR_CAST_TARGETING_MODES" :allow-empty="false" @update:model-value="(value) => patchEffect(index, effectIndex, { targetingMode: value })" /></label>
              <label>基础效果倍率<NumberInput :model-value="Number(effect.amountRatioBase ?? effect.amountRatio ?? 1)" @update:model-value="(value) => patchEffect(index, effectIndex, { amountRatioBase: Math.max(0, Number(value) || 0), amountRatio: undefined })" /></label>
              <label>每级强化效果倍率<NumberInput :model-value="Number(effect.amountRatioPerEnhanceLevel ?? 0)" @update:model-value="(value) => patchEffect(index, effectIndex, { amountRatioPerEnhanceLevel: Math.max(0, Number(value) || 0) })" /></label>
              <label>MP消耗倍率<NumberInput :model-value="Number(effect.mpCostMultiplier ?? 1)" @update:model-value="(value) => patchEffect(index, effectIndex, { mpCostMultiplier: value })" /></label>
              <label>HP消耗倍率<NumberInput :model-value="Number(effect.hpCostMultiplier ?? 1)" @update:model-value="(value) => patchEffect(index, effectIndex, { hpCostMultiplier: value })" /></label>
              <label class="checkbox-field"><input type="checkbox" :checked="Boolean(effect.includeBasicAttack)" @change="patchEffect(index, effectIndex, { includeBasicAttack: eventChecked($event) })" />包含普通攻击</label>
              <label class="checkbox-field"><input type="checkbox" :checked="Boolean(effect.allowMirroredCastTrigger)" @change="patchEffect(index, effectIndex, { allowMirroredCastTrigger: eventChecked($event) })" />允许镜像继续触发</label>
              <label class="checkbox-field"><input type="checkbox" :checked="effect.applyCooldown !== false" @change="patchEffect(index, effectIndex, { applyCooldown: eventChecked($event) })" />触发技能冷却</label>
              <label class="wide">监听技能ID（空=全部）<StringListEditor :model-value="asStringList(effect.skillIds)" :options="skillOptions" link-table="skills" link-label="技能" @update:model-value="(value) => patchEffect(index, effectIndex, { skillIds: value })" /></label>
              <label class="wide">监听技能类型（空=全部）<StringListEditor :model-value="asStringList(effect.skillCategories)" :options="skillCategoryOptions" link-table="skill_type" link-label="技能类型" @update:model-value="(value) => patchEffect(index, effectIndex, { skillCategories: value })" /></label>
            </template>
            <template v-else-if="effectKind(effect) === 'consume_mp'">
              <label class="wide">消耗魔法值公式（支持强化等级）<FormulaEditor :model-value="effect.amountFormula || { baseAdd: 0, terms: [] }" :attr-options="hookFormulaAttrOptions" @update:model-value="(value) => patchEffect(index, effectIndex, { amountFormula: value })" /></label>
              <label class="checkbox-field"><input type="checkbox" :checked="Boolean(effect.requireFullAmount)" @change="patchEffect(index, effectIndex, { requireFullAmount: eventChecked($event) })" />要求法力足额</label>
              <label class="checkbox-field"><input type="checkbox" :checked="Boolean(effect.abortHookOnFail)" @change="patchEffect(index, effectIndex, { abortHookOnFail: eventChecked($event) })" />不足时中止后续效果</label>
            </template>
            <template v-else-if="effectKind(effect) === 'adjust_skill_cooldown'">
              <label>冷却调整回合<NumberInput :model-value="Number(effect.amount ?? effect.value ?? -1)" @update:model-value="(value) => patchEffect(index, effectIndex, { amount: Math.trunc(Number(value) || 0), value: undefined })" /></label>
              <label>目标模式<SearchSelect :model-value="String(effect.targetMode || 'self')" :options="BATTLE_HOOK_TARGET_MODES" :allow-empty="false" @update:model-value="(value) => patchEffect(index, effectIndex, { targetMode: value })" /></label>
              <label>选择策略<SearchSelect :model-value="String(effect.selectionMode || 'all')" :options="SKILL_COOLDOWN_SELECTION_MODES" :allow-empty="false" @update:model-value="(value) => patchEffect(index, effectIndex, { selectionMode: value })" /></label>
              <label>最多影响技能数（0=不限）<NumberInput :model-value="Number(effect.maxSkills || 0)" @update:model-value="(value) => patchEffect(index, effectIndex, { maxSkills: Math.max(0, Math.trunc(Number(value) || 0)) })" /></label>
              <label class="checkbox-field"><input type="checkbox" :checked="Boolean(effect.includeBasic ?? effect.includeBasicAttack)" @change="patchEffect(index, effectIndex, { includeBasic: eventChecked($event), includeBasicAttack: undefined })" />包含普通攻击</label>
              <label class="wide">技能ID（空=全部）<StringListEditor :model-value="asStringList(effect.skillIds)" :options="skillOptions" link-table="skills" link-label="技能" @update:model-value="(value) => patchEffect(index, effectIndex, { skillIds: value })" /></label>
              <label class="wide">排除技能ID<StringListEditor :model-value="asStringList(effect.excludeSkillIds)" :options="skillOptions" link-table="skills" link-label="技能" @update:model-value="(value) => patchEffect(index, effectIndex, { excludeSkillIds: value })" /></label>
              <label class="wide">技能类型（空=全部）<StringListEditor :model-value="asStringList(effect.skillCategories)" :options="skillCategoryOptions" link-table="skill_type" link-label="技能类型" @update:model-value="(value) => patchEffect(index, effectIndex, { skillCategories: value })" /></label>
            </template>
            <template v-else-if="effectKind(effect) === 'transfer_skill_cooldown'">
              <label>转移回合数<NumberInput :model-value="Number(effect.amount ?? 1)" @update:model-value="(value) => patchEffect(index, effectIndex, { amount: Math.max(1, Math.trunc(Number(value) || 1)) })" /></label>
              <label class="checkbox-field"><input type="checkbox" :checked="effect.requireBoth === true" @change="patchEffect(index, effectIndex, { requireBoth: eventChecked($event) })" />仅在两侧均合法时原子结算</label>
              <label class="wide">被缩短技能类型<StringListEditor :model-value="asStringList(effect.from?.skillCategories)" :options="skillCategoryOptions" link-table="skill_type" link-label="技能类型" @update:model-value="(value) => patchEffect(index, effectIndex, { from: { ...(effect.from || {}), skillCategories: value } })" /></label>
              <label>被缩短技能选择<SearchSelect :model-value="String(effect.from?.selectionMode || 'max_remaining')" :options="['max_remaining']" :allow-empty="false" @update:model-value="(value) => patchEffect(index, effectIndex, { from: { ...(effect.from || {}), selectionMode: value } })" /></label>
              <label class="checkbox-field"><input type="checkbox" :checked="effect.from?.excludeContextSkill === true" @change="patchEffect(index, effectIndex, { from: { ...(effect.from || {}), excludeContextSkill: eventChecked($event) } })" />排除本次释放技能</label>
              <label class="checkbox-field"><input type="checkbox" :checked="effect.from?.requireCooling === true" @change="patchEffect(index, effectIndex, { from: { ...(effect.from || {}), requireCooling: eventChecked($event) } })" />只选择冷却中的技能</label>
              <label class="checkbox-field"><input type="checkbox" :checked="Boolean(effect.from?.includeBasic ?? effect.from?.includeBasicAttack)" @change="patchEffect(index, effectIndex, { from: { ...(effect.from || {}), includeBasic: eventChecked($event), includeBasicAttack: undefined } })" />被缩短侧包含普攻</label>
              <label>最多选择技能数<NumberInput :model-value="Number(effect.from?.maxSkills ?? 1)" @update:model-value="(value) => patchEffect(index, effectIndex, { from: { ...(effect.from || {}), maxSkills: Math.max(1, Math.trunc(Number(value) || 1)) } })" /></label>
              <label class="wide">本次释放技能类型<StringListEditor :model-value="asStringList(effect.to?.skillCategories)" :options="skillCategoryOptions" link-table="skill_type" link-label="技能类型" @update:model-value="(value) => patchEffect(index, effectIndex, { to: { ...(effect.to || {}), skillCategories: value } })" /></label>
              <label class="checkbox-field"><input type="checkbox" :checked="Boolean(effect.to?.includeBasic ?? effect.to?.includeBasicAttack)" @change="patchEffect(index, effectIndex, { to: { ...(effect.to || {}), includeBasic: eventChecked($event), includeBasicAttack: undefined } })" />本次释放侧包含普攻</label>
              <label>免除增加所需状态<SearchSelect :model-value="String(effect.waiveIncreaseBuffId || '')" :options="buffOptions" @update:model-value="(value) => patchEffect(index, effectIndex, { waiveIncreaseBuffId: value })" /></label>
              <label class="checkbox-field"><input type="checkbox" :checked="effect.consumeWaiverBuff === true" @change="patchEffect(index, effectIndex, { consumeWaiverBuff: eventChecked($event) })" />触发后消耗免除状态</label>
            </template>
            <template v-else-if="effectKind(effect) === 'extra_turn'">
              <label>行动者<SearchSelect :model-value="String(effect.targetMode || 'self')" :options="['self']" :allow-empty="false" @update:model-value="(value) => patchEffect(index, effectIndex, { targetMode: value })" /></label>
              <label>行动模式<SearchSelect :model-value="String(effect.turnMode || 'full')" :options="['full']" :allow-empty="false" @update:model-value="(value) => patchEffect(index, effectIndex, { turnMode: value })" /></label>
            </template>
            <template v-else-if="effectKind(effect) === 'execute'">
              <label>基础斩杀阈值（血量百分比，如0.10=10%）<NumberInput :model-value="Number(effect.hpPercentThreshold ?? 0.10)" @update:model-value="(value) => patchEffect(index, effectIndex, { hpPercentThreshold: value })" /></label>
              <label>每强化等级提升（如0.005=+0.5%/级）<NumberInput :model-value="Number(effect.hpPercentPerEnhanceLevel ?? 0.005)" @update:model-value="(value) => patchEffect(index, effectIndex, { hpPercentPerEnhanceLevel: value })" /></label>
              <label>目标模式<SearchSelect :model-value="String(effect.targetMode || 'source')" :options="BATTLE_HOOK_TARGET_MODES" :allow-empty="false" @update:model-value="(value) => patchEffect(index, effectIndex, { targetMode: value })" /></label>
            </template>
            <template v-else-if="effectKind(effect) === 'basic_attack_marked_target'">
              <label>基础概率<NumberInput :model-value="Number(effect.procChanceBase ?? 0.5)" @update:model-value="(value) => patchEffect(index, effectIndex, { procChanceBase: value })" /></label>
              <label>最小概率<NumberInput :model-value="Number(effect.procChanceMin ?? 0)" @update:model-value="(value) => patchEffect(index, effectIndex, { procChanceMin: value })" /></label>
              <label>最大概率<NumberInput :model-value="Number(effect.procChanceMax ?? 1)" @update:model-value="(value) => patchEffect(index, effectIndex, { procChanceMax: value })" /></label>
              <label class="wide">概率公式<FormulaEditor :model-value="effect.procChanceFormula || { baseAdd: 0, terms: [] }" :attr-options="hookFormulaAttrOptions" @update:model-value="(value) => patchEffect(index, effectIndex, { procChanceFormula: value })" /></label>
            </template>
            <template v-else>
              <label>倍率<NumberInput :model-value="Number(effect.value || effect.percent || 0)" @update:model-value="(value) => patchEffect(index, effectIndex, { value })" /></label>
              <label>最小倍率<NumberInput :model-value="Number(effect.valueMin ?? 0)" @update:model-value="(value) => patchEffect(index, effectIndex, { valueMin: value })" /></label>
              <label>最大倍率<NumberInput :model-value="Number(effect.valueMax ?? 1)" @update:model-value="(value) => patchEffect(index, effectIndex, { valueMax: value })" /></label>
              <label class="wide">倍率公式<FormulaEditor :model-value="effect.valueFormula || { baseAdd: 0, terms: [] }" :attr-options="hookFormulaAttrOptions" @update:model-value="(value) => patchEffect(index, effectIndex, { valueFormula: value })" /></label>
              <label class="wide">伤害类型<StringListEditor :model-value="Array.isArray(effect.damageTypes) ? effect.damageTypes : []" :options="DAMAGE_TYPES" @update:model-value="(value) => patchEffect(index, effectIndex, { damageTypes: value })" /></label>
            </template>
            <template v-if="usesProcProbability(effect) && !isProbabilityModifier(effect)">
              <label>概率共享组<input :value="String(effect.probabilityGroup || '')" @input="patchEffect(index, effectIndex, { probabilityGroup: eventValue($event) || undefined })" /></label>
              <label>每回合最多成功（0=不限）<NumberInput :model-value="Number(effect.maxSuccessesPerTurn || 0)" @update:model-value="(value) => patchEffect(index, effectIndex, { maxSuccessesPerTurn: Number(value) > 0 ? Math.trunc(Number(value)) : undefined })" /></label>
              <label class="checkbox-field"><input type="checkbox" :checked="Boolean(effect.ignoreProbabilityHooks)" @change="patchEffect(index, effectIndex, { ignoreProbabilityHooks: eventChecked($event) })" />忽略概率钩子</label>
            </template>
          </div>
        </div>
        <div class="toolbar">
          <button class="secondary" @click="addEffect(index, 'add_buff')">新增施加状态</button>
          <button class="secondary" @click="addEffect(index, 'remove_buff')">新增移除状态</button>
          <button class="secondary" @click="addEffect(index, 'remove_buff_by_kind')">新增按类别移除</button>
          <button class="secondary" @click="addEffect(index, 'damage')">新增伤害</button>
          <button class="secondary" @click="addEffect(index, 'heal')">新增治疗</button>
          <button class="secondary" @click="addEffect(index, 'consume_mp')">新增消耗魔法</button>
          <button class="secondary" @click="addEffect(index, 'adjust_skill_cooldown')">新增冷却调整</button>
          <button class="secondary" @click="addEffect(index, 'transfer_skill_cooldown')">新增冷却转移</button>
          <button class="secondary" @click="addEffect(index, 'extra_turn')">新增立即行动</button>
          <button class="secondary" @click="addEffect(index, 'execute')">新增斩杀</button>
          <button class="secondary" @click="addEffect(index, 'runtime_damage_amp', 'physical')">新增物理增伤</button>
          <button class="secondary" @click="addEffect(index, 'runtime_damage_amp', 'magical')">新增魔法增伤</button>
          <button class="secondary" @click="addEffect(index, 'basic_attack_marked_target')">新增普攻标记目标</button>
          <button class="secondary" @click="addEffect(index, 'extra_enemy_refresh')">新增额外 Boss 刷新</button>
          <button class="secondary" @click="addEffect(index, 'summon')">新增召唤</button>
          <button class="secondary" @click="addEffect(index, 'player_clone')">新增玩家克隆体</button>
          <button class="secondary" @click="addEffect(index, 'mirror_cast_skill')">新增镜像施法</button>
          <button class="secondary" @click="addEffect(index, 'add_probability')">新增概率调整</button>
          <button class="secondary" @click="addEffect(index, 'set_probability')">新增概率设置</button>
          <button class="secondary" @click="addEffect(index, 'force_probability_failure')">新增强制失败</button>
        </div>
      </article>
      <button class="secondary" @click="add">新增战斗钩子</button>
    </div>
  `
});

const AttrModsEditor = defineComponent({
  name: 'AttrModsEditor',
  components: { SearchSelect, NumberInput },
  props: {
    modelValue: { type: Object as PropType<Record<string, unknown>>, default: () => ({}) },
    rarityOptions: { type: Array as PropType<string[]>, default: () => [] },
    attrOptions: { type: Array as PropType<string[]>, default: () => [] }
  },
  emits: ['update:modelValue'],
  setup(props, { emit }) {
    const rows = computed(() => {
      const out: Array<{ rarity: string; attr: string; min: number; max: number }> = [];
      for (const [rarity, attrs] of Object.entries(props.modelValue ?? {})) {
        if (!isRecord(attrs)) continue;
        for (const [attr, range] of Object.entries(attrs)) {
          const node = isRecord(range) ? range : {};
          out.push({ rarity, attr, min: asNumber(node.min, 0), max: asNumber(node.max, 0) });
        }
      }
      return out;
    });
    const rebuild = (nextRows: Array<{ rarity: string; attr: string; min: number; max: number }>) => {
      const next: Record<string, Record<string, { min: number; max: number }>> = {};
      for (const row of nextRows) {
        if (!row.rarity || !row.attr) continue;
        if (!next[row.rarity]) next[row.rarity] = {};
        next[row.rarity][row.attr] = { min: row.min, max: row.max };
      }
      emit('update:modelValue', next);
    };
    const patchAt = (index: number, patch: Partial<{ rarity: string; attr: string; min: number; max: number }>) => {
      const next = rows.value.map((row) => ({ ...row }));
      next[index] = { ...next[index], ...patch };
      rebuild(next);
    };
    const add = () => rebuild([...rows.value, { rarity: props.rarityOptions[0] ?? 'rare', attr: props.attrOptions[0] ?? '', min: 0, max: 1 }]);
    const removeAt = (index: number) => {
      const next = rows.value.map((row) => ({ ...row }));
      next.splice(index, 1);
      rebuild(next);
    };
    return { rows, patchAt, add, removeAt };
  },
  template: `
    <div class="buff-rule-editor">
      <article class="json-row compact attr-mod-row" v-for="(row, index) in rows" :key="index">
        <SearchSelect :model-value="row.rarity" :options="rarityOptions" @update:model-value="(value) => patchAt(index, { rarity: value })" />
        <SearchSelect :model-value="row.attr" :options="attrOptions" @update:model-value="(value) => patchAt(index, { attr: value })" />
        <NumberInput :model-value="row.min" @update:model-value="(value) => patchAt(index, { min: value })" />
        <NumberInput :model-value="row.max" @update:model-value="(value) => patchAt(index, { max: value })" />
        <button class="danger" @click="removeAt(index)">删</button>
      </article>
      <button class="secondary" @click="add">新增属性范围</button>
    </div>
  `
});

const EffectMutationsEditor = defineComponent({
  name: 'EffectMutationsEditor',
  components: { FormulaEditor, StringListEditor, SearchSelect, NumberInput },
  props: {
    modelValue: { type: Array<unknown>, default: () => [] },
    attrOptions: { type: Array<string>, default: () => [] },
    skillOptions: { type: Array<string>, default: () => [] },
    buffOptions: { type: Array<string>, default: () => [] }
  },
  emits: ['update:modelValue'],
  setup(props, { emit }) {
    const normalized = (value: unknown): DataRow => isRecord(value) ? value : { scope: 'basic_attack', skillIds: [], effectSelector: { mode: 'index', indexes: [1] }, ops: {}, constraints: {} };
    const patchAt = (index: number, patch: DataRow) => {
      const next = props.modelValue.map((row) => normalized(row));
      next[index] = { ...next[index], ...patch };
      emit('update:modelValue', next);
    };
    const patchOps = (index: number, patch: DataRow) => {
      const current = normalized(props.modelValue[index]);
      patchAt(index, { ops: { ...(isRecord(current.ops) ? current.ops : {}), ...patch } });
    };
    const patchConstraints = (index: number, patch: DataRow) => {
      const current = normalized(props.modelValue[index]);
      patchAt(index, { constraints: { ...(isRecord(current.constraints) ? current.constraints : {}), ...patch } });
    };
    const selectorIndexes = (mutation: unknown) => asArray(isRecord(mutation) && isRecord(mutation.effectSelector) ? mutation.effectSelector.indexes : [1]).map((value) => asNumber(value, 1));
    const ops = (mutation: unknown) => isRecord(mutation) && isRecord(mutation.ops) ? mutation.ops : {};
    const constraints = (mutation: unknown) => isRecord(mutation) && isRecord(mutation.constraints) ? mutation.constraints : {};
    const add = () => emit('update:modelValue', [...props.modelValue, normalized(null)]);
    const removeAt = (index: number) => {
      const next = [...props.modelValue];
      next.splice(index, 1);
      emit('update:modelValue', next);
    };
    return { EFFECT_MUTATION_SCOPES, normalized, selectorIndexes, ops, constraints, patchAt, patchOps, patchConstraints, add, removeAt, asStringList };
  },
  template: `
    <div class="buff-rule-editor">
      <article class="nested-card" v-for="(raw, index) in modelValue" :key="index">
        <div class="nested-head">
          <strong>变更 #{{ index + 1 }}</strong>
          <button class="danger" @click="removeAt(index)">删除</button>
        </div>
        <div class="form-grid cols-2">
          <label>作用范围<SearchSelect :model-value="String(normalized(raw).scope || 'basic_attack')" :options="EFFECT_MUTATION_SCOPES" @update:model-value="(value) => patchAt(index, { scope: value })" /></label>
          <label class="wide">指定技能<StringListEditor :model-value="asStringList(normalized(raw).skillIds)" :options="skillOptions" link-table="skills" link-label="技能" @update:model-value="(value) => patchAt(index, { skillIds: value })" /></label>
          <label>目标效果序号<NumberInput :model-value="selectorIndexes(raw)[0] || 1" @update:model-value="(value) => patchAt(index, { effectSelector: { mode: 'index', indexes: [value] } })" /></label>
          <label>目标数增量<NumberInput :model-value="Number(ops(raw).targetCountDelta || 0)" @update:model-value="(value) => patchOps(index, { targetCountDelta: value })" /></label>
          <label>基础值增量<NumberInput :model-value="Number(ops(raw).amountBaseAddDelta || 0)" @update:model-value="(value) => patchOps(index, { amountBaseAddDelta: value })" /></label>
          <label>最小目标数<NumberInput :model-value="Number(constraints(raw).targetCountMin || 0)" @update:model-value="(value) => patchConstraints(index, { targetCountMin: value })" /></label>
          <label>最大目标数<NumberInput :model-value="Number(constraints(raw).targetCountMax || 0)" @update:model-value="(value) => patchConstraints(index, { targetCountMax: value })" /></label>
          <label class="wide">追加公式<FormulaEditor :model-value="{ baseAdd: 0, terms: ops(raw).amountTermsAppend || [] }" :attr-options="attrOptions" @update:model-value="(value) => patchOps(index, { amountTermsAppend: value.terms || [] })" /></label>
          <label class="wide">追加状态<StringListEditor :model-value="asStringList(ops(raw).statusIdsAppend)" :options="buffOptions" link-table="buff" link-label="Buff" @update:model-value="(value) => patchOps(index, { statusIdsAppend: value })" /></label>
          <label class="wide">移除状态<StringListEditor :model-value="asStringList(ops(raw).statusIdsRemove)" :options="buffOptions" link-table="buff" link-label="Buff" @update:model-value="(value) => patchOps(index, { statusIdsRemove: value })" /></label>
        </div>
      </article>
      <button class="secondary" @click="add">新增效果变更</button>
    </div>
  `
});

const AttrConversionEditor = defineComponent({
  name: 'AttrConversionEditor',
  components: { FormulaEditor, SearchSelect, NumberInput },
  props: {
    modelValue: { type: Array as PropType<unknown[]>, default: () => [] },
    attrOptions: { type: Array as PropType<string[]>, default: () => [] }
  },
  emits: ['update:modelValue'],
  setup(props, { emit }) {
    const normalized = (v: unknown): Record<string, unknown> => isRecord(v) ? v : { fromAttr: '', toAttr: '', threshold: 1.0, ratioFormula: { baseAdd: 0.3, terms: [] } };
    const ratioFormula = (v: unknown) => { const f = normalized(v).ratioFormula; return isRecord(f) ? f : { baseAdd: 0.3, terms: [] }; };
    const patchAt = (index: number, patch: Record<string, unknown>) => {
      const next = asArray(props.modelValue).map((row) => normalized(row));
      next[index] = { ...next[index], ...patch };
      emit('update:modelValue', next);
    };
    const add = () => emit('update:modelValue', [...asArray(props.modelValue), normalized(null)]);
    const removeAt = (index: number) => {
      const next = [...asArray(props.modelValue)];
      next.splice(index, 1);
      emit('update:modelValue', next);
    };
    return { normalized, ratioFormula, patchAt, add, removeAt };
  },
  template: `
    <div class="buff-rule-editor">
      <article class="nested-card" v-for="(raw, index) in modelValue" :key="index">
        <div class="nested-head">
          <strong>转换规则 #{{ index + 1 }}</strong>
          <button class="danger" @click="removeAt(index)">删除</button>
        </div>
        <div class="form-grid cols-2">
          <label>来源属性（fromAttr）<SearchSelect :model-value="String(normalized(raw).fromAttr || '')" :options="attrOptions" @update:model-value="(value) => patchAt(index, { fromAttr: value })" /></label>
          <label>目标属性（toAttr）<SearchSelect :model-value="String(normalized(raw).toAttr || '')" :options="attrOptions" @update:model-value="(value) => patchAt(index, { toAttr: value })" /></label>
          <label>阈值（threshold）<NumberInput :model-value="Number(normalized(raw).threshold ?? 1.0)" @update:model-value="(value) => patchAt(index, { threshold: value })" /></label>
          <label class="wide">转化率公式（ratioFormula，支持 equipment_enhance_level）<FormulaEditor :model-value="ratioFormula(raw)" :attr-options="attrOptions" @update:model-value="(value) => patchAt(index, { ratioFormula: value })" /></label>
        </div>
      </article>
      <button class="secondary" @click="add">新增转换规则</button>
    </div>
  `
});

const RarityBattleHooksEditor = defineComponent({
  name: 'RarityBattleHooksEditor',
  components: { SearchSelect, BattleHooksEditor },
  props: {
    modelValue: { type: Object as PropType<Record<string, unknown>>, default: () => ({}) },
    rarityOptions: { type: Array as PropType<string[]>, default: () => [] },
    buffOptions: { type: Array as PropType<string[]>, default: () => [] },
    attrOptions: { type: Array as PropType<string[]>, default: () => [] },
    skillOptions: { type: Array as PropType<string[]>, default: () => [] },
    skillCategoryOptions: { type: Array as PropType<string[]>, default: () => [] }
  },
  emits: ['update:modelValue'],
  setup(props, { emit }) {
    const mapValue = computed(() => (isRecord(props.modelValue) ? props.modelValue : {}));
    const entries = computed(() => Object.entries(mapValue.value).filter(([, hooks]) => Array.isArray(hooks)));
    const emitMap = (next: Record<string, unknown>) => emit('update:modelValue', next);
    const updateKey = (oldKey: string, nextKey: string) => {
      const key = asString(nextKey).trim();
      if (!key) return;
      const next = { ...mapValue.value };
      const hooks = next[oldKey];
      delete next[oldKey];
      next[key] = Array.isArray(hooks) ? hooks : [];
      emitMap(next);
    };
    const updateHooks = (rarity: string, hooks: unknown[]) => emitMap({ ...mapValue.value, [rarity]: hooks });
    const remove = (rarity: string) => {
      const next = { ...mapValue.value };
      delete next[rarity];
      emitMap(next);
    };
    const add = () => {
      const rarity = props.rarityOptions.find((option) => !Object.prototype.hasOwnProperty.call(mapValue.value, option)) ?? 'rare';
      if (Object.prototype.hasOwnProperty.call(mapValue.value, rarity)) return;
      emitMap({ ...mapValue.value, [rarity]: [] });
    };
    return { entries, updateKey, updateHooks, remove, add };
  },
  template: `
    <div class="buff-rule-editor">
      <article class="nested-card" v-for="[rarity, hooks] in entries" :key="rarity">
        <div class="nested-head">
          <strong>稀有度</strong>
          <SearchSelect :model-value="rarity" :options="rarityOptions" :allow-empty="false" @update:model-value="(value) => updateKey(rarity, value)" />
          <button class="danger" @click="remove(rarity)">删除</button>
        </div>
        <BattleHooksEditor :model-value="Array.isArray(hooks) ? hooks : []" :buff-options="buffOptions" :attr-options="attrOptions" :skill-options="skillOptions" :skill-category-options="skillCategoryOptions" @update:model-value="(value) => updateHooks(rarity, value)" />
      </article>
      <button class="secondary" @click="add">新增稀有度钩子组</button>
    </div>
  `
});

const AffixEffectParamsEditor = defineComponent({
  name: 'AffixEffectParamsEditor',
  components: { AttrModsEditor, BattleHooksEditor, RarityBattleHooksEditor, EffectMutationsEditor, JsonFieldEditor },
  props: {
    modelValue: { type: Object as PropType<Record<string, unknown>>, default: () => ({}) },
    effectType: { type: String, default: 'attr_bonus' },
    rarityOptions: { type: Array as PropType<string[]>, default: () => [] },
    attrOptions: { type: Array as PropType<string[]>, default: () => [] },
    buffOptions: { type: Array as PropType<string[]>, default: () => [] },
    skillOptions: { type: Array as PropType<string[]>, default: () => [] },
    skillCategoryOptions: { type: Array as PropType<string[]>, default: () => [] }
  },
  emits: ['update:modelValue'],
  setup(props, { emit }) {
    const update = (patch: Record<string, unknown>) => emit('update:modelValue', { ...(props.modelValue ?? {}), ...patch });
    return { update };
  },
  template: `
    <div class="affix-param-editor">
      <AttrModsEditor
        v-if="effectType === 'attr_bonus'"
        :model-value="modelValue.attrMods || {}"
        :rarity-options="rarityOptions"
        :attr-options="attrOptions"
        @update:model-value="(value) => update({ attrMods: value })"
      />
      <RarityBattleHooksEditor
        v-else-if="(effectType === 'battle_trigger' || effectType === 'battle_hook') && modelValue.battleHooks && !Array.isArray(modelValue.battleHooks)"
        :model-value="modelValue.battleHooks || {}"
        :rarity-options="rarityOptions"
        :buff-options="buffOptions"
        :attr-options="attrOptions"
        :skill-options="skillOptions"
        :skill-category-options="skillCategoryOptions"
        @update:model-value="(value) => update({ battleHooks: value })"
      />
      <BattleHooksEditor
        v-else-if="effectType === 'battle_trigger' || effectType === 'battle_hook'"
        :model-value="modelValue.battleHooks || []"
        :buff-options="buffOptions"
        :attr-options="attrOptions"
        :skill-options="skillOptions"
        :skill-category-options="skillCategoryOptions"
        @update:model-value="(value) => update({ battleHooks: value })"
      />
      <EffectMutationsEditor
        v-else-if="effectType === 'effectMutation'"
        :model-value="modelValue.effectMutations || []"
        :attr-options="attrOptions"
        :skill-options="skillOptions"
        :buff-options="buffOptions"
        @update:model-value="(value) => update({ effectMutations: value })"
      />
      <JsonFieldEditor v-else :model-value="modelValue" :path="['effectParams']" @update:model-value="(value) => $emit('update:modelValue', value)" />
    </div>
  `
});

// ---------------------------------------------------------------------------
// Alchemy editors — specialized form widgets for the alchemy DSL tables.
// Designed for content authors (策划)so every nested struct in the alchemy
// catalog has a dedicated UI instead of a raw JSON blob.
// ---------------------------------------------------------------------------

const ALCHEMY_FAMILY_OPTIONS = ['gathering', 'mining', 'livestock', 'crafting', 'sewing', 'alchemy', 'physical', 'crit', 'magic_crit', 'arcane', 'shadow', 'brute', 'defense', 'apex'];
const ALCHEMY_KIND_OPTIONS = ['life', 'combat'];
const ALCHEMY_LUCKY_TIER_KINDS = ['gold_multiplier', 'extra_item'];

// {itemId, weight}[] — for alchemy_groups.entries
const AlchemyGroupEntriesEditor = defineComponent({
  name: 'AlchemyGroupEntriesEditor',
  components: { NumberInput, SearchSelect },
  props: {
    modelValue: { type: Array as PropType<unknown[]>, default: () => [] },
    itemOptions: { type: Array as PropType<string[]>, default: () => [] },
    itemValueMap: { type: Object as PropType<Record<string, number>>, default: () => ({}) }
  },
  emits: ['update:modelValue'],
  setup(props, { emit }) {
    const rows = computed(() => asArray(props.modelValue).filter(isRecord).map((row: Record<string, unknown>) => ({
      itemId: asString(row.itemId, ''),
      weight: asNumber(row.weight, 0)
    })));
    const totalWeight = computed(() => rows.value.reduce((sum, r) => sum + Math.max(0, r.weight), 0));
    const emitRows = (next: Array<{ itemId: string; weight: number }>) => emit('update:modelValue', next);
    const patchAt = (idx: number, patch: Partial<{ itemId: string; weight: number }>) => {
      const next = rows.value.map((r, i) => i === idx ? { ...r, ...patch } : r);
      emitRows(next);
    };
    const removeAt = (idx: number) => emitRows(rows.value.filter((_, i) => i !== idx));
    const add = () => emitRows([...rows.value, { itemId: '', weight: 1 }]);
    const fillByEconomy = () => {
      const next = rows.value.map((r) => {
        const v = Math.max(1, asNumber(props.itemValueMap[r.itemId], r.weight));
        return { ...r, weight: v };
      });
      emitRows(next);
    };
    return { rows, totalWeight, patchAt, removeAt, add, fillByEconomy };
  },
  template: `
    <div class="array-editor">
      <div class="json-row compact" v-for="(row, idx) in rows" :key="idx">
        <SearchSelect :model-value="row.itemId" :options="itemOptions" @update:model-value="(value) => patchAt(idx, { itemId: value })" />
        <label class="range-label">权重</label>
        <NumberInput :model-value="row.weight" :min="1" @update:model-value="(value) => patchAt(idx, { weight: Math.max(1, Number(value) || 1) })" style="width:88px" />
        <span class="number-map-value-meta" v-if="totalWeight > 0">{{ ((row.weight / totalWeight) * 100).toFixed(1) }}%</span>
        <button class="danger" @click="removeAt(idx)">删</button>
      </div>
      <div style="display:flex; gap:6px;">
        <button class="secondary" @click="add">新增</button>
        <button class="secondary" @click="fillByEconomy" title="将所有权重设为 economyBaseValue">按经济价值填权重</button>
      </div>
    </div>
  `
});

// modifiers: {valueMultiplier, negativeMultiplier, failChanceDelta, successFloor, maxTraitSlots, familyAffinity}
const AlchemyMediumModifiersEditor = defineComponent({
  name: 'AlchemyMediumModifiersEditor',
  components: { NumberInput, NumberMapEditor },
  props: {
    modelValue: { type: Object as PropType<Record<string, unknown>>, default: () => ({}) }
  },
  emits: ['update:modelValue'],
  setup(props, { emit }) {
    const data = computed(() => {
      const base = isRecord(props.modelValue) ? props.modelValue : {};
      return {
        valueMultiplier: asNumber(base.valueMultiplier, 1),
        negativeMultiplier: asNumber(base.negativeMultiplier, 1),
        failChanceDelta: asNumber(base.failChanceDelta, 0),
        successFloor: asNumber(base.successFloor, 0),
        maxTraitSlots: Math.max(0, asNumber(base.maxTraitSlots, 0)),
        familyAffinity: isRecord(base.familyAffinity) ? base.familyAffinity as Record<string, unknown> : {}
      };
    });
    const update = (patch: Record<string, unknown>) => emit('update:modelValue', { ...(props.modelValue ?? {}), ...patch });
    const eventValue = (event: Event) => (event.target as HTMLInputElement | HTMLSelectElement).value;
    const eventChecked = (event: Event) => (event.target as HTMLInputElement).checked;
    return { data, update, ALCHEMY_FAMILY_OPTIONS, itemOptions, eventValue, eventChecked };
  },
  template: `
    <div class="buff-rule-editor">
      <div class="form-grid cols-3">
        <label>正向值倍率<NumberInput :model-value="data.valueMultiplier" :step="0.05" @update:model-value="(v) => update({ valueMultiplier: Number(v) })" /></label>
        <label>副作用倍率<NumberInput :model-value="data.negativeMultiplier" :step="0.05" @update:model-value="(v) => update({ negativeMultiplier: Number(v) })" /></label>
        <label>失败率增减<NumberInput :model-value="data.failChanceDelta" :step="0.01" @update:model-value="(v) => update({ failChanceDelta: Number(v) })" /></label>
        <label>成功率保底<NumberInput :model-value="data.successFloor" :step="0.05" :min="0" @update:model-value="(v) => update({ successFloor: Math.max(0, Number(v)) })" /></label>
        <label>素材槽位上限<NumberInput :model-value="data.maxTraitSlots" :step="1" :min="0" @update:model-value="(v) => update({ maxTraitSlots: Math.max(0, Math.round(Number(v))) })" /></label>
      </div>
      <div class="param-section">
        <div class="param-section-title">系别亲和（family → 加成倍率）</div>
        <NumberMapEditor :model-value="data.familyAffinity" :options="ALCHEMY_FAMILY_OPTIONS" @update:model-value="(v) => update({ familyAffinity: v })" />
      </div>
    </div>
  `
});

const AlchemyInheritModifiersEditor = defineComponent({
  name: 'AlchemyInheritModifiersEditor',
  components: { NumberInput, NumberMapEditor },
  props: {
    modelValue: { type: Object as PropType<Record<string, unknown>>, default: () => ({}) }
  },
  emits: ['update:modelValue'],
  setup(props, { emit }) {
    const data = computed(() => {
      const base = isRecord(props.modelValue) ? props.modelValue : {};
      const inheritCost = isRecord(base.inheritCost) ? base.inheritCost : {};
      const suppressCost = isRecord(base.suppressCost) ? base.suppressCost : {};
      const goodRange = isRecord(base.inbreedGoodMultiplierRange) ? base.inbreedGoodMultiplierRange : {};
      const badRange = isRecord(base.inbreedBadMultiplierRange) ? base.inbreedBadMultiplierRange : {};
      const precisionScaling = isRecord(base.precisionScaling) ? base.precisionScaling : {};
      const luckyScaling = isRecord(base.luckyScaling) ? base.luckyScaling : {};
      const stabilityScaling = isRecord(base.stabilityScaling) ? base.stabilityScaling : {};
      return {
        inheritSlots: Math.max(0, Math.round(asNumber(base.inheritSlots, 3))),
        suppressSlots: Math.max(0, Math.round(asNumber(base.suppressSlots, 1))),
        ignoreKeepChance: asNumber(base.ignoreKeepChance, 0.1),
        positiveIgnoreKeepChanceBase: asNumber(base.positiveIgnoreKeepChanceBase, 0.5),
        positiveIgnoreKeepChanceBonus: asNumber(base.positiveIgnoreKeepChanceBonus, 0.1),
        negativeIgnoreKeepChanceBase: asNumber(base.negativeIgnoreKeepChanceBase, 0.9),
        negativeIgnoreSuppressChanceBonus: asNumber(base.negativeIgnoreSuppressChanceBonus, 0),
        inheritCostGold: Math.max(0, Math.round(asNumber(inheritCost.gold, 200))),
        inheritCostItems: isRecord(inheritCost.items) ? inheritCost.items as Record<string, unknown> : {},
        suppressCostGold: Math.max(0, Math.round(asNumber(suppressCost.gold, 500))),
        suppressCostItems: isRecord(suppressCost.items) ? suppressCost.items as Record<string, unknown> : {},
        hybridMergeDivisor: Math.max(1, asNumber(base.hybridMergeDivisor, 1.6)),
        inbreedMutationChance: asNumber(base.inbreedMutationChance, 0.8),
        inbreedGoodChance: asNumber(base.inbreedGoodChance, 0.05),
        inbreedGoodMin: asNumber(goodRange.min, 1.1),
        inbreedGoodMax: asNumber(goodRange.max, 1.35),
        inbreedBadMin: asNumber(badRange.min, 0.35),
        inbreedBadMax: asNumber(badRange.max, 0.8),
        mutationChance: asNumber(base.mutationChance, 0.08),
        precisionDivisorReductionPerPoint: asNumber(precisionScaling.hybridDivisorReductionPerPoint, 0.01),
        precisionDivisorReductionCap: asNumber(precisionScaling.hybridDivisorReductionCap, 0.12),
        luckyIgnoreKeepChancePerPoint: asNumber(luckyScaling.ignoreKeepChancePerPoint, 0.01),
        luckyIgnoreKeepChanceCap: asNumber(luckyScaling.ignoreKeepChanceCap, 0.1),
        luckyInbreedGoodChancePerPoint: asNumber(luckyScaling.inbreedGoodChancePerPoint, 0.003),
        luckyInbreedGoodChanceCap: asNumber(luckyScaling.inbreedGoodChanceCap, 0.05),
        stabilityMutationReductionPerPoint: asNumber(stabilityScaling.inbreedMutationChanceReductionPerPoint, 0.01),
        stabilityMutationReductionCap: asNumber(stabilityScaling.inbreedMutationChanceReductionCap, 0.12),
        stabilityBadMultiplierFloorBonusPerPoint: asNumber(stabilityScaling.badMultiplierFloorBonusPerPoint, 0.01),
        stabilityBadMultiplierFloorBonusCap: asNumber(stabilityScaling.badMultiplierFloorBonusCap, 0.12)
      };
    });
    const update = (patch: Record<string, unknown>) => emit('update:modelValue', { ...(props.modelValue ?? {}), ...patch });
    const updateCost = (key: 'inheritCost' | 'suppressCost', patch: Record<string, unknown>) => {
      const current = isRecord(props.modelValue?.[key]) ? props.modelValue?.[key] as Record<string, unknown> : {};
      update({ [key]: { ...current, ...patch } });
    };
    const updateRange = (key: 'inbreedGoodMultiplierRange' | 'inbreedBadMultiplierRange', patch: Record<string, unknown>) => {
      const current = isRecord(props.modelValue?.[key]) ? props.modelValue?.[key] as Record<string, unknown> : {};
      update({ [key]: { ...current, ...patch } });
    };
    const updateScaling = (key: 'precisionScaling' | 'luckyScaling' | 'stabilityScaling', patch: Record<string, unknown>) => {
      const current = isRecord(props.modelValue?.[key]) ? props.modelValue?.[key] as Record<string, unknown> : {};
      update({ [key]: { ...current, ...patch } });
    };
    return { data, update, updateCost, updateRange, updateScaling, itemOptions };
  },
  template: `
    <div class="buff-rule-editor">
      <div class="form-grid cols-3">
        <label>保留额度<NumberInput :model-value="data.inheritSlots" :step="1" :min="0" @update:model-value="(v) => update({ inheritSlots: Math.max(0, Math.round(Number(v))) })" /></label>
        <label>抑制额度<NumberInput :model-value="data.suppressSlots" :step="1" :min="0" @update:model-value="(v) => update({ suppressSlots: Math.max(0, Math.round(Number(v))) })" /></label>
        <label>不干预保留率<NumberInput :model-value="data.ignoreKeepChance" :step="0.01" :min="0" :max="1" @update:model-value="(v) => update({ ignoreKeepChance: Math.max(0, Math.min(1, Number(v))) })" /></label>
        <label>正面不干预基础保留<NumberInput :model-value="data.positiveIgnoreKeepChanceBase" :step="0.01" :min="0" :max="1" @update:model-value="(v) => update({ positiveIgnoreKeepChanceBase: Math.max(0, Math.min(1, Number(v))) })" /></label>
        <label>正面不干预保留加成<NumberInput :model-value="data.positiveIgnoreKeepChanceBonus" :step="0.01" :min="0" :max="1" @update:model-value="(v) => update({ positiveIgnoreKeepChanceBonus: Math.max(0, Math.min(1, Number(v))) })" /></label>
        <label>负面不干预基础保留<NumberInput :model-value="data.negativeIgnoreKeepChanceBase" :step="0.01" :min="0" :max="1" @update:model-value="(v) => update({ negativeIgnoreKeepChanceBase: Math.max(0, Math.min(1, Number(v))) })" /></label>
        <label>负面不干预丢弃加成<NumberInput :model-value="data.negativeIgnoreSuppressChanceBonus" :step="0.01" :min="0" :max="1" @update:model-value="(v) => update({ negativeIgnoreSuppressChanceBonus: Math.max(0, Math.min(1, Number(v))) })" /></label>
        <label>保留金币/条<NumberInput :model-value="data.inheritCostGold" :step="100" :min="0" @update:model-value="(v) => updateCost('inheritCost', { gold: Math.max(0, Math.round(Number(v))) })" /></label>
        <label>抑制金币/条<NumberInput :model-value="data.suppressCostGold" :step="100" :min="0" @update:model-value="(v) => updateCost('suppressCost', { gold: Math.max(0, Math.round(Number(v))) })" /></label>
        <label>异源融合除数<NumberInput :model-value="data.hybridMergeDivisor" :step="0.05" :min="1" @update:model-value="(v) => update({ hybridMergeDivisor: Math.max(1, Number(v)) })" /></label>
        <label>同源畸变率<NumberInput :model-value="data.inbreedMutationChance" :step="0.01" :min="0" :max="1" @update:model-value="(v) => update({ inbreedMutationChance: Math.max(0, Math.min(1, Number(v))) })" /></label>
        <label>畸变正向概率<NumberInput :model-value="data.inbreedGoodChance" :step="0.01" :min="0" :max="1" @update:model-value="(v) => update({ inbreedGoodChance: Math.max(0, Math.min(1, Number(v))) })" /></label>
        <label>变异率<NumberInput :model-value="data.mutationChance" :step="0.01" :min="0" @update:model-value="(v) => update({ mutationChance: Math.max(0, Number(v)) })" /></label>
        <label>正向倍率下限<NumberInput :model-value="data.inbreedGoodMin" :step="0.05" :min="0" @update:model-value="(v) => updateRange('inbreedGoodMultiplierRange', { min: Math.max(0, Number(v)) })" /></label>
        <label>正向倍率上限<NumberInput :model-value="data.inbreedGoodMax" :step="0.05" :min="0" @update:model-value="(v) => updateRange('inbreedGoodMultiplierRange', { max: Math.max(0, Number(v)) })" /></label>
        <label>负向倍率下限<NumberInput :model-value="data.inbreedBadMin" :step="0.05" :min="0" @update:model-value="(v) => updateRange('inbreedBadMultiplierRange', { min: Math.max(0, Number(v)) })" /></label>
        <label>负向倍率上限<NumberInput :model-value="data.inbreedBadMax" :step="0.05" :min="0" @update:model-value="(v) => updateRange('inbreedBadMultiplierRange', { max: Math.max(0, Number(v)) })" /></label>
      </div>
      <div class="param-section">
        <div class="param-section-title">保留道具费用（itemId → 数量）</div>
        <NumberMapEditor :model-value="data.inheritCostItems" :options="itemOptions" @update:model-value="(v) => updateCost('inheritCost', { items: v })" />
      </div>
      <div class="param-section">
        <div class="param-section-title">抑制道具费用（itemId → 数量）</div>
        <NumberMapEditor :model-value="data.suppressCostItems" :options="itemOptions" @update:model-value="(v) => updateCost('suppressCost', { items: v })" />
      </div>
      <div class="form-grid cols-3">
        <label>精度：融合除数降低/点<NumberInput :model-value="data.precisionDivisorReductionPerPoint" :step="0.001" :min="0" @update:model-value="(v) => updateScaling('precisionScaling', { hybridDivisorReductionPerPoint: Math.max(0, Number(v)) })" /></label>
        <label>精度：降低上限<NumberInput :model-value="data.precisionDivisorReductionCap" :step="0.01" :min="0" @update:model-value="(v) => updateScaling('precisionScaling', { hybridDivisorReductionCap: Math.max(0, Number(v)) })" /></label>
        <label>幸运：不干预保留/点<NumberInput :model-value="data.luckyIgnoreKeepChancePerPoint" :step="0.001" :min="0" @update:model-value="(v) => updateScaling('luckyScaling', { ignoreKeepChancePerPoint: Math.max(0, Number(v)) })" /></label>
        <label>幸运：不干预上限<NumberInput :model-value="data.luckyIgnoreKeepChanceCap" :step="0.01" :min="0" @update:model-value="(v) => updateScaling('luckyScaling', { ignoreKeepChanceCap: Math.max(0, Number(v)) })" /></label>
        <label>幸运：正向畸变/点<NumberInput :model-value="data.luckyInbreedGoodChancePerPoint" :step="0.001" :min="0" @update:model-value="(v) => updateScaling('luckyScaling', { inbreedGoodChancePerPoint: Math.max(0, Number(v)) })" /></label>
        <label>幸运：正向畸变上限<NumberInput :model-value="data.luckyInbreedGoodChanceCap" :step="0.01" :min="0" @update:model-value="(v) => updateScaling('luckyScaling', { inbreedGoodChanceCap: Math.max(0, Number(v)) })" /></label>
        <label>稳定：畸变降低/点<NumberInput :model-value="data.stabilityMutationReductionPerPoint" :step="0.001" :min="0" @update:model-value="(v) => updateScaling('stabilityScaling', { inbreedMutationChanceReductionPerPoint: Math.max(0, Number(v)) })" /></label>
        <label>稳定：畸变降低上限<NumberInput :model-value="data.stabilityMutationReductionCap" :step="0.01" :min="0" @update:model-value="(v) => updateScaling('stabilityScaling', { inbreedMutationChanceReductionCap: Math.max(0, Number(v)) })" /></label>
        <label>稳定：坏倍率底线/点<NumberInput :model-value="data.stabilityBadMultiplierFloorBonusPerPoint" :step="0.001" :min="0" @update:model-value="(v) => updateScaling('stabilityScaling', { badMultiplierFloorBonusPerPoint: Math.max(0, Number(v)) })" /></label>
        <label>稳定：坏倍率底线上限<NumberInput :model-value="data.stabilityBadMultiplierFloorBonusCap" :step="0.01" :min="0" @update:model-value="(v) => updateScaling('stabilityScaling', { badMultiplierFloorBonusCap: Math.max(0, Number(v)) })" /></label>
      </div>
    </div>
  `
});

// {min, max} or {minRatio, maxRatio} — generic two-number range
const AlchemyValueRangeEditor = defineComponent({
  name: 'AlchemyValueRangeEditor',
  components: { NumberInput },
  props: {
    modelValue: { type: Object as PropType<Record<string, unknown>>, default: () => ({}) },
    minKey: { type: String, default: 'min' },
    maxKey: { type: String, default: 'max' },
    step: { type: Number, default: 0.01 },
    label: { type: String, default: '' }
  },
  emits: ['update:modelValue'],
  setup(props, { emit }) {
    const minVal = computed(() => asNumber(isRecord(props.modelValue) ? props.modelValue[props.minKey] : 0, 0));
    const maxVal = computed(() => asNumber(isRecord(props.modelValue) ? props.modelValue[props.maxKey] : 0, 0));
    const update = (key: string, value: number) => emit('update:modelValue', { ...(isRecord(props.modelValue) ? props.modelValue : {}), [key]: value });
    return { minVal, maxVal, update };
  },
  template: `
    <div class="json-row compact">
      <span v-if="label" class="range-label">{{ label }}</span>
      <label class="range-label">最低</label>
      <NumberInput :model-value="minVal" :step="step" @update:model-value="(v) => update(minKey, Number(v))" style="width:96px" />
      <label class="range-label">最高</label>
      <NumberInput :model-value="maxVal" :step="step" @update:model-value="(v) => update(maxKey, Number(v))" style="width:96px" />
    </div>
  `
});

type AlchemyComboTemplateSource = 'comboPower' | 'comboMultiplier';
type AlchemyComboTemplateValueKind = 'attr' | 'chance' | 'number';
type AlchemyComboTemplateRow = {
  path: Array<string | number>;
  pathText: string;
  title: string;
  detail: string;
  sourceLabel: string;
  formulaText: string;
  valueKind: AlchemyComboTemplateValueKind;
  attr?: string;
  source: AlchemyComboTemplateSource;
  base: number;
  coeff: number;
};

function collectAlchemyComboTemplates(value: unknown, path: Array<string | number> = [], context?: DataRow): AlchemyComboTemplateRow[] {
  if (Array.isArray(value)) {
    return value.flatMap((item, index) => collectAlchemyComboTemplates(item, [...path, index], context));
  }
  if (!isRecord(value)) return [];
  const nextContext = asString(value.kind) ? value : context;
  if ('alchemyValueCoeff' in value || 'alchemyMultiplierCoeff' in value) {
    const source: AlchemyComboTemplateSource = 'alchemyMultiplierCoeff' in value ? 'comboMultiplier' : 'comboPower';
    const coeffKey = source === 'comboMultiplier' ? 'alchemyMultiplierCoeff' : 'alchemyValueCoeff';
    const pathText = formatAlchemyComboTemplatePath(path);
    const meta = alchemyComboTemplateMeta(path, nextContext);
    return [{
      path,
      pathText,
      ...meta,
      sourceLabel: source === 'comboMultiplier' ? '配方倍数 comboMultiplier' : '随机强度 comboPower',
      formulaText: source === 'comboMultiplier' ? '结果 = base + comboMultiplier x coeff' : '结果 = base + comboPower x coeff',
      source,
      base: asNumber(value.base, 0),
      coeff: asNumber(value[coeffKey], 0)
    }];
  }
  const out: AlchemyComboTemplateRow[] = [];
  for (const [key, child] of Object.entries(value)) {
    out.push(...collectAlchemyComboTemplates(child, [...path, key], nextContext));
  }
  return out;
}

function formatAlchemyComboTemplatePath(path: Array<string | number>): string {
  return path.map((node) => typeof node === 'number' ? `[${node}]` : String(node)).join('.').replace(/\.\[/g, '[');
}

function alchemyComboTemplateMeta(path: Array<string | number>, context?: DataRow): Pick<AlchemyComboTemplateRow, 'title' | 'detail' | 'valueKind' | 'attr'> {
  const attrModsIndex = path.findIndex((segment) => segment === 'attrMods');
  if (attrModsIndex >= 0 && typeof path[attrModsIndex + 1] === 'string') {
    const attr = String(path[attrModsIndex + 1]);
    return {
      title: `携带者属性：${alchemyAttrLabel(attr)}`,
      detail: '写入词条的 attrMods，直接影响装备携带者。',
      valueKind: 'attr',
      attr
    };
  }
  const statsIndex = path.findIndex((segment) => segment === 'statsFormula');
  if (statsIndex >= 0 && typeof path[statsIndex + 1] === 'string') {
    const attr = String(path[statsIndex + 1]);
    const payload = isRecord(context?.summonPayload) ? context.summonPayload : {};
    const unitId = asString(payload.unitId);
    const unitName = unitId ? alchemyEnemyLabel(unitId) : '召唤物';
    return {
      title: `召唤物属性：${unitName} · ${alchemyAttrLabel(attr)}`,
      detail: '写入 summonPayload.statsFormula，作为召唤单位的附加属性。',
      valueKind: 'attr',
      attr
    };
  }
  if (path.some((segment) => String(segment).toLowerCase().includes('chance'))) {
    return {
      title: '触发概率',
      detail: alchemyEffectContextDetail(context),
      valueKind: 'chance'
    };
  }
  if (path.some((segment) => segment === 'payload')) {
    return {
      title: `状态参数：${alchemyEffectContextDetail(context)}`,
      detail: '写入 Buff payload，Buff 效果运行时会读取该数值。',
      valueKind: 'number'
    };
  }
  return {
    title: '模板参数',
    detail: alchemyEffectContextDetail(context),
    valueKind: 'number'
  };
}

function alchemyEffectContextDetail(context?: DataRow): string {
  const kind = asString(context?.kind);
  const buffId = asString(context?.buffId);
  if (kind === 'summon') {
    const payload = isRecord(context?.summonPayload) ? context.summonPayload : {};
    const unitId = asString(payload.unitId);
    return unitId ? `召唤 ${alchemyEnemyLabel(unitId)}` : '召唤效果';
  }
  if (buffId) return `施加状态 ${alchemyBuffLabel(buffId)}`;
  return kind ? `${OPTION_LABEL_OVERRIDES[kind] ?? kind} 效果` : '组合效果参数';
}

function alchemyAttrLabel(attr: string): string {
  return localizeKey(`attr.${attr}.name`, attr);
}

function alchemyBuffLabel(buffId: string): string {
  const row = catalog.buff.find((entry) => rowId(entry) === buffId);
  return row ? displayName(row) : localizeKey(`buff.${buffId}.name`, buffId);
}

function alchemyEnemyLabel(enemyId: string): string {
  const row = catalog.enemies.find((entry) => rowId(entry) === enemyId);
  if (row) return localizeKey(asString(row.nameKey), '') || localizeKey(`enemy.${enemyId}.name`, enemyId) || asString(row.name) || enemyId;
  return localizeKey(`enemy.${enemyId}.name`, enemyId);
}

function patchAlchemyComboTemplate(value: unknown, path: Array<string | number>, patch: Partial<Pick<AlchemyComboTemplateRow, 'source' | 'base' | 'coeff'>>): unknown {
  const next = clone(value);
  let cursor: unknown = next;
  for (const segment of path.slice(0, -1)) {
    if (Array.isArray(cursor) && typeof segment === 'number') cursor = cursor[segment];
    else if (isRecord(cursor) && typeof segment === 'string') cursor = cursor[segment];
    else return next;
  }
  const leafKey = path[path.length - 1];
  const parent = cursor;
  if (leafKey === undefined) {
    return alchemyComboTemplateNode('comboPower', 0, 0, patch);
  }
  if (Array.isArray(parent) && typeof leafKey === 'number') {
    const current = isRecord(parent[leafKey]) ? parent[leafKey] : {};
    parent[leafKey] = alchemyComboTemplateNode(
      'alchemyMultiplierCoeff' in current ? 'comboMultiplier' : 'comboPower',
      asNumber(current.base, 0),
      asNumber(firstDefined(current.alchemyValueCoeff, current.alchemyMultiplierCoeff), 0),
      patch
    );
  } else if (isRecord(parent) && typeof leafKey === 'string') {
    const current = isRecord(parent[leafKey]) ? parent[leafKey] as Record<string, unknown> : {};
    parent[leafKey] = alchemyComboTemplateNode(
      'alchemyMultiplierCoeff' in current ? 'comboMultiplier' : 'comboPower',
      asNumber(current.base, 0),
      asNumber(firstDefined(current.alchemyValueCoeff, current.alchemyMultiplierCoeff), 0),
      patch
    );
  }
  return next;
}

function firstDefined(...values: unknown[]): unknown {
  return values.find((value) => value !== undefined);
}

function alchemyComboTemplateNode(source: AlchemyComboTemplateSource, base: number, coeff: number, patch: Partial<Pick<AlchemyComboTemplateRow, 'source' | 'base' | 'coeff'>>): Record<string, number> {
  const nextSource = patch.source || source;
  const nextBase = patch.base ?? base;
  const nextCoeff = patch.coeff ?? coeff;
  return nextSource === 'comboMultiplier'
    ? { base: nextBase, alchemyMultiplierCoeff: nextCoeff }
    : { base: nextBase, alchemyValueCoeff: nextCoeff };
}

const AlchemyComboParamTemplateEditor = defineComponent({
  name: 'AlchemyComboParamTemplateEditor',
  components: { NumberInput },
  props: {
    modelValue: { type: Object as PropType<Record<string, unknown>>, default: () => ({}) }
  },
  emits: ['update:modelValue'],
  setup(props, { emit }) {
    const rows = computed(() => collectAlchemyComboTemplates(props.modelValue));
    const updateRow = (row: AlchemyComboTemplateRow, patch: Partial<Pick<AlchemyComboTemplateRow, 'source' | 'base' | 'coeff'>>) => {
      emit('update:modelValue', patchAlchemyComboTemplate(props.modelValue, row.path, patch));
    };
    return { rows, updateRow };
  },
  template: `
    <div v-if="rows.length > 0" class="param-section">
      <div class="param-section-title">组合强度依赖（随机范围解析后写入）</div>
      <div class="alchemy-template-grid">
        <article v-for="row in rows" :key="row.pathText" class="alchemy-template-card">
          <div class="alchemy-template-path">
            <strong>{{ row.title }}</strong>
            <small>{{ row.detail }}</small>
            <code :title="row.pathText">{{ row.pathText }}</code>
          </div>
          <div class="alchemy-template-controls">
            <label>
              <span>依赖来源</span>
              <select :value="row.source" @change="(event) => updateRow(row, { source: event.target.value })">
                <option value="comboPower">comboPower 随机强度</option>
                <option value="comboMultiplier">comboMultiplier 配方倍数</option>
              </select>
            </label>
            <label>
              <span>基础值 base</span>
              <NumberInput :model-value="row.base" :step="0.01" @update:model-value="(value) => updateRow(row, { base: Number(value) || 0 })" />
            </label>
            <label>
              <span>系数 coeff</span>
              <NumberInput :model-value="row.coeff" :step="0.01" @update:model-value="(value) => updateRow(row, { coeff: Number(value) || 0 })" />
            </label>
          </div>
          <div class="alchemy-template-formula">
            {{ row.formulaText }}
          </div>
        </article>
      </div>
    </div>
  `
});

// outputAffix: {effectType, params: {attr}, valueRange: {min, max}}
const AlchemyAffixOutputEditor = defineComponent({
  name: 'AlchemyAffixOutputEditor',
  components: { NumberInput, SearchSelect, AlchemyValueRangeEditor, AffixEffectParamsEditor, AlchemyComboParamTemplateEditor },
  props: {
    modelValue: { type: Object as PropType<Record<string, unknown>>, default: () => ({}) },
    attrOptions: { type: Array as PropType<string[]>, default: () => [] },
    rarityOptions: { type: Array as PropType<string[]>, default: () => [] },
    buffOptions: { type: Array as PropType<string[]>, default: () => [] },
    skillOptions: { type: Array as PropType<string[]>, default: () => [] },
    skillCategoryOptions: { type: Array as PropType<string[]>, default: () => [] }
  },
  emits: ['update:modelValue'],
  setup(props, { emit }) {
    const data = computed(() => {
      const base = isRecord(props.modelValue) ? props.modelValue : {};
      const params = isRecord(base.params) ? base.params as Record<string, unknown> : {};
      const valueRange = isRecord(base.valueRange) ? base.valueRange as Record<string, unknown> : { min: 0, max: 0 };
      return {
        effectType: asString(base.effectType, 'add_stat'),
        attr: asString(params.attr, ''),
        params,
        valueRange,
        outputRarity: asString(base.outputRarity, '')
      };
    });
    const update = (patch: Record<string, unknown>) => emit('update:modelValue', { ...(props.modelValue ?? {}), ...patch });
    const updateParam = (attr: string) => update({ params: { ...data.value.params, attr } });
    const updateParams = (params: Record<string, unknown>) => update({ params });
    const updateRange = (next: Record<string, unknown>) => update({ valueRange: next });
    return { data, update, updateParam, updateParams, updateRange };
  },
  template: `
    <div class="buff-rule-editor">
      <div class="form-grid cols-3">
        <label>效果类型<SearchSelect :model-value="data.effectType" :options="['add_stat', 'attr_bonus', 'battle_trigger', 'battle_hook', 'effectMutation']" @update:model-value="(v) => update({ effectType: v })" /></label>
        <label v-if="data.effectType === 'add_stat'">目标属性<SearchSelect :model-value="data.attr" :options="attrOptions" @update:model-value="updateParam" /></label>
      </div>
      <div v-if="data.effectType === 'add_stat'" class="param-section">
        <div class="param-section-title">数值范围 valueRange</div>
        <AlchemyValueRangeEditor :model-value="data.valueRange" @update:model-value="updateRange" />
      </div>
      <div v-else class="param-section">
        <div class="param-section-title">高级 effectParams（复用词条编辑器）</div>
        <AffixEffectParamsEditor
          :model-value="data.params"
          :effect-type="data.effectType"
          :rarity-options="rarityOptions"
          :attr-options="attrOptions"
          :buff-options="buffOptions"
          :skill-options="skillOptions"
          :skill-category-options="skillCategoryOptions"
          @update:model-value="updateParams"
        />
        <AlchemyComboParamTemplateEditor :model-value="data.params" @update:model-value="updateParams" />
      </div>
    </div>
  `
});

// luckyTiers: list of {id, weight, kind, value?|itemId?|min?|max?}
const AlchemyLuckyTiersEditor = defineComponent({
  name: 'AlchemyLuckyTiersEditor',
  components: { NumberInput, SearchSelect },
  props: {
    modelValue: { type: Array as PropType<unknown[]>, default: () => [] },
    itemOptions: { type: Array as PropType<string[]>, default: () => [] }
  },
  emits: ['update:modelValue'],
  setup(props, { emit }) {
    const rows = computed(() => asArray(props.modelValue).filter(isRecord).map((row: Record<string, unknown>) => ({
      id: asString(row.id, ''),
      weight: asNumber(row.weight, 1),
      kind: asString(row.kind, 'gold_multiplier'),
      value: asNumber(row.value, 0),
      itemId: asString(row.itemId, ''),
      min: asNumber(row.min, 1),
      max: asNumber(row.max, 1)
    })));
    const totalWeight = computed(() => rows.value.reduce((sum, r) => sum + Math.max(0, r.weight), 0));
    const emitRows = (next: unknown[]) => emit('update:modelValue', next);
    const patchAt = (idx: number, patch: Record<string, unknown>) => {
      const next = rows.value.map((r, i) => {
        if (i !== idx) return r;
        return { ...r, ...patch };
      });
      // strip irrelevant fields per kind for cleanliness
      const clean = next.map((r) => {
        const base: Record<string, unknown> = { id: r.id, weight: r.weight, kind: r.kind };
        if (r.kind === 'gold_multiplier') {
          base.value = r.value;
        } else if (r.kind === 'extra_item') {
          base.itemId = r.itemId;
          base.min = r.min;
          base.max = r.max;
        }
        return base;
      });
      emitRows(clean);
    };
    const removeAt = (idx: number) => emitRows(rows.value.filter((_, i) => i !== idx));
    const add = () => emitRows([...rows.value, { id: '', weight: 10, kind: 'gold_multiplier', value: 2 }]);
    return { rows, totalWeight, patchAt, removeAt, add, ALCHEMY_LUCKY_TIER_KINDS };
  },
  template: `
    <div class="array-editor">
      <article class="nested-card" v-for="(row, idx) in rows" :key="idx">
        <div class="nested-head">
          <strong>档位 #{{ idx + 1 }} <span v-if="totalWeight > 0" class="number-map-value-meta">{{ ((row.weight / totalWeight) * 100).toFixed(1) }}%</span></strong>
          <button class="danger" @click="removeAt(idx)">删除</button>
        </div>
        <div class="form-grid cols-3">
          <label>ID<input :value="row.id" @input="patchAt(idx, { id: $event.target.value })" /></label>
          <label>权重<NumberInput :model-value="row.weight" :min="1" @update:model-value="(v) => patchAt(idx, { weight: Math.max(1, Number(v) || 1) })" /></label>
          <label>类型<SearchSelect :model-value="row.kind" :options="ALCHEMY_LUCKY_TIER_KINDS" @update:model-value="(v) => patchAt(idx, { kind: v })" /></label>
          <label v-if="row.kind === 'gold_multiplier'">金币倍率<NumberInput :model-value="row.value" :step="0.1" @update:model-value="(v) => patchAt(idx, { value: Number(v) })" /></label>
          <template v-else-if="row.kind === 'extra_item'">
            <label>物品<SearchSelect :model-value="row.itemId" :options="itemOptions" @update:model-value="(v) => patchAt(idx, { itemId: v })" /></label>
            <label>最少<NumberInput :model-value="row.min" :min="1" @update:model-value="(v) => patchAt(idx, { min: Math.max(1, Number(v) || 1) })" /></label>
            <label>最多<NumberInput :model-value="row.max" :min="1" @update:model-value="(v) => patchAt(idx, { max: Math.max(1, Number(v) || 1) })" /></label>
          </template>
        </div>
      </article>
      <button class="secondary" @click="add">新增档位</button>
    </div>
  `
});

// nameAffix: {prefixKey, suffixKey}
const AlchemyNameAffixEditor = defineComponent({
  name: 'AlchemyNameAffixEditor',
  props: {
    modelValue: { type: Object as PropType<Record<string, unknown>>, default: () => ({}) }
  },
  emits: ['update:modelValue'],
  setup(props, { emit }) {
    const prefixKey = computed(() => asString(isRecord(props.modelValue) ? props.modelValue.prefixKey : '', ''));
    const suffixKey = computed(() => asString(isRecord(props.modelValue) ? props.modelValue.suffixKey : '', ''));
    const update = (patch: Record<string, unknown>) => emit('update:modelValue', { ...(props.modelValue ?? {}), ...patch });
    return { prefixKey, suffixKey, update };
  },
  template: `
    <div class="form-grid cols-2">
      <label>前缀 i18n key<input :value="prefixKey" placeholder="alchemy.affix.prefix.xxx" @input="update({ prefixKey: $event.target.value })" /></label>
      <label>后缀 i18n key<input :value="suffixKey" placeholder="alchemy.affix.suffix.xxx" @input="update({ suffixKey: $event.target.value })" /></label>
    </div>
  `
});

// item_traits.traits: list of trait records
const AlchemyTraitListEditor = defineComponent({
  name: 'AlchemyTraitListEditor',
  components: { NumberInput, SearchSelect, AlchemyValueRangeEditor },
  props: {
    modelValue: { type: Array as PropType<unknown[]>, default: () => [] },
    attrOptions: { type: Array as PropType<string[]>, default: () => [] }
  },
  emits: ['update:modelValue'],
  setup(props, { emit }) {
    const rows = computed(() => asArray(props.modelValue).filter(isRecord).map((row: Record<string, unknown>) => ({
      traitId: asString(row.traitId, ''),
      kind: asString(row.kind, 'life'),
      family: asString(row.family, 'regen'),
      effectType: asString(row.effectType, 'add_stat'),
      effects: asArray(row.effects).filter(isRecord).map((e: Record<string, unknown>) => ({
        attr: asString(e.attr, ''),
        valuePerUnit: isRecord(e.valuePerUnit) ? e.valuePerUnit as Record<string, unknown> : { min: 0, max: 0 }
      })),
      unitBasis: Math.max(1, asNumber(row.unitBasis, 1)),
      maxBasisMultiplier: Math.max(1, asNumber(row.maxBasisMultiplier, 1)),
      minAlchemyLevel: Math.max(1, asNumber(row.minAlchemyLevel, 1)),
      failBaseChance: asNumber(row.failBaseChance, 0),
      failChancePerLevelAbove: asNumber(row.failChancePerLevelAbove, 0)
    })));
    const emitRows = (next: unknown[]) => emit('update:modelValue', next);
    const patchAt = (idx: number, patch: Record<string, unknown>) => {
      const next = rows.value.map((r, i) => i === idx ? { ...r, ...patch } : r);
      emitRows(next);
    };
    const removeAt = (idx: number) => emitRows(rows.value.filter((_, i) => i !== idx));
    const add = () => emitRows([...rows.value, {
      traitId: '',
      kind: 'life',
      family: 'regen',
      effectType: 'add_stat',
      effects: [{ attr: '', valuePerUnit: { min: 0, max: 0 } }],
      unitBasis: 1,
      maxBasisMultiplier: 100,
      minAlchemyLevel: 1,
      failBaseChance: 0.1,
      failChancePerLevelAbove: 0.005
    }]);
    const patchEffect = (traitIdx: number, effectIdx: number, patch: Record<string, unknown>) => {
      const trait = rows.value[traitIdx];
      const nextEffects = trait.effects.map((e, i) => i === effectIdx ? { ...e, ...patch } : e);
      patchAt(traitIdx, { effects: nextEffects });
    };
    const addEffect = (traitIdx: number) => {
      const trait = rows.value[traitIdx];
      patchAt(traitIdx, { effects: [...trait.effects, { attr: '', valuePerUnit: { min: 0, max: 0 } }] });
    };
    const removeEffect = (traitIdx: number, effectIdx: number) => {
      const trait = rows.value[traitIdx];
      patchAt(traitIdx, { effects: trait.effects.filter((_, i) => i !== effectIdx) });
    };
    return { rows, patchAt, removeAt, add, patchEffect, addEffect, removeEffect, ALCHEMY_KIND_OPTIONS, ALCHEMY_FAMILY_OPTIONS };
  },
  template: `
    <div class="array-editor">
      <article class="nested-card" v-for="(row, idx) in rows" :key="idx">
        <div class="nested-head">
          <strong>特质 #{{ idx + 1 }} {{ row.traitId ? '/ ' + row.traitId : '' }}</strong>
          <button class="danger" @click="removeAt(idx)">删除</button>
        </div>
        <div class="form-grid cols-3">
          <label>traitId<input :value="row.traitId" @input="patchAt(idx, { traitId: $event.target.value })" /></label>
          <label>类型<SearchSelect :model-value="row.kind" :options="ALCHEMY_KIND_OPTIONS" @update:model-value="(v) => patchAt(idx, { kind: v })" /></label>
          <label>系别<SearchSelect :model-value="row.family" :options="ALCHEMY_FAMILY_OPTIONS" @update:model-value="(v) => patchAt(idx, { family: v })" /></label>
          <label>效果类型<input :value="row.effectType" @input="patchAt(idx, { effectType: $event.target.value })" /></label>
          <label>每档物品数 unitBasis<NumberInput :model-value="row.unitBasis" :min="1" @update:model-value="(v) => patchAt(idx, { unitBasis: Math.max(1, Math.round(Number(v) || 1)) })" /></label>
          <label>最大档位 maxBasisMultiplier<NumberInput :model-value="row.maxBasisMultiplier" :min="1" @update:model-value="(v) => patchAt(idx, { maxBasisMultiplier: Math.max(1, Math.round(Number(v) || 1)) })" /></label>
          <label>最低炼金等级<NumberInput :model-value="row.minAlchemyLevel" :min="1" @update:model-value="(v) => patchAt(idx, { minAlchemyLevel: Math.max(1, Math.round(Number(v) || 1)) })" /></label>
          <label>基础失败率<NumberInput :model-value="row.failBaseChance" :step="0.01" :min="0" @update:model-value="(v) => patchAt(idx, { failBaseChance: Number(v) })" /></label>
          <label>每超等级降幅<NumberInput :model-value="row.failChancePerLevelAbove" :step="0.001" :min="0" @update:model-value="(v) => patchAt(idx, { failChancePerLevelAbove: Number(v) })" /></label>
        </div>
        <div class="param-section">
          <div class="param-section-title">每物品效果（effects）</div>
          <div class="json-row compact" v-for="(effect, ei) in row.effects" :key="ei">
            <SearchSelect :model-value="effect.attr" :options="attrOptions" @update:model-value="(v) => patchEffect(idx, ei, { attr: v })" />
            <AlchemyValueRangeEditor :model-value="effect.valuePerUnit" @update:model-value="(v) => patchEffect(idx, ei, { valuePerUnit: v })" />
            <button class="danger" @click="removeEffect(idx, ei)">删</button>
          </div>
          <button class="secondary" @click="addEffect(idx)">新增效果</button>
        </div>
      </article>
      <button class="secondary" @click="add">新增特质</button>
    </div>
  `
});

const KittyPersonalityEffectsEditor = defineComponent({
  name: 'KittyPersonalityEffectsEditor',
  components: { SearchSelect, NumberMapEditor, BattleHooksEditor, EffectMutationsEditor },
  props: {
    modelValue: { type: Array as PropType<unknown[]>, default: () => [] },
    attrOptions: { type: Array as PropType<string[]>, default: () => [] },
    buffOptions: { type: Array as PropType<string[]>, default: () => [] },
    skillOptions: { type: Array as PropType<string[]>, default: () => [] },
    skillCategoryOptions: { type: Array as PropType<string[]>, default: () => [] },
    enemyOptions: { type: Array as PropType<string[]>, default: () => [] }
  },
  emits: ['update:modelValue'],
  setup(props, { emit }) {
    const rows = computed(() => asArray(props.modelValue).map((row) => {
      const node = isRecord(row) ? row : {};
      const kind = asString(node.kind, 'attr_modifier');
      return {
        ...node,
        kind,
        attrs: isRecord(node.attrs) ? Object.fromEntries(
          Object.entries(node.attrs)
            .map(([key, value]) => [asString(key), asNumber(value, 0)])
            .filter(([key, value]) => key && Number.isFinite(value))
        ) : {},
        position: asString(node.position, 'vanguard'),
        battleHooks: asArray(node.battleHooks).filter(isRecord),
        effectMutations: asArray(node.effectMutations).filter(isRecord)
      };
    }));
    const emitRows = (next: Record<string, unknown>[]) => emit('update:modelValue', next);
    const defaultRowForKind = (kind: string) => {
      if (kind === 'battle_position_lock') return { kind, position: 'vanguard' };
      if (kind === 'battle_hook') return { kind, battleHooks: [] };
      if (kind === 'effect_mutation') return { kind, effectMutations: [] };
      return { kind: 'attr_modifier', attrs: {} };
    };
    const updateAt = (index: number, patch: Record<string, unknown>) => {
      const next = rows.value.map((row, i) => i === index ? { ...row, ...patch } : row);
      emitRows(next);
    };
    const changeKindAt = (index: number, kind: string) => {
      const next = rows.value.map((row, i) => i === index ? defaultRowForKind(kind) : row);
      emitRows(next);
    };
    const removeAt = (index: number) => emitRows(rows.value.filter((_, i) => i !== index));
    const add = () => emitRows([...rows.value, defaultRowForKind('attr_modifier')]);
    return {
      rows,
      updateAt,
      changeKindAt,
      removeAt,
      add,
      POSITION_OPTIONS: POSITIONS,
      EFFECT_KIND_OPTIONS: ['attr_modifier', 'battle_position_lock', 'battle_hook', 'effect_mutation']
    };
  },
  template: `
    <div class="buff-rule-editor">
      <article class="nested-card" v-for="(effect, index) in rows" :key="index">
        <div class="nested-head">
          <strong>效果 #{{ index + 1 }}</strong>
          <SearchSelect :model-value="String(effect.kind || '')" :options="EFFECT_KIND_OPTIONS" :allow-empty="false" @update:model-value="(value) => changeKindAt(index, value)" />
          <button class="danger" @click="removeAt(index)">删除</button>
        </div>
        <div class="nested-grid">
          <template v-if="effect.kind === 'attr_modifier'">
            <div class="nested-span-2">
              <span>属性修正</span>
              <NumberMapEditor
                :model-value="effect.attrs || {}"
                :options="attrOptions"
                @update:model-value="(value) => updateAt(index, { attrs: value })"
              />
            </div>
          </template>
          <template v-else-if="effect.kind === 'battle_position_lock'">
            <label>
              <span>站位锁定</span>
              <SearchSelect :model-value="String(effect.position || 'vanguard')" :options="POSITION_OPTIONS" :allow-empty="false" @update:model-value="(value) => updateAt(index, { position: value })" />
            </label>
          </template>
          <template v-else-if="effect.kind === 'battle_hook'">
            <div class="nested-span-2">
              <span>战斗钩子</span>
              <BattleHooksEditor
                :model-value="effect.battleHooks || []"
                :buff-options="buffOptions"
                :attr-options="attrOptions"
                :enemy-options="enemyOptions"
                :skill-options="skillOptions"
                :skill-category-options="skillCategoryOptions"
                @update:model-value="(value) => updateAt(index, { battleHooks: value })"
              />
            </div>
          </template>
          <template v-else>
            <div class="nested-span-2">
              <span>效果变更</span>
              <EffectMutationsEditor
                :model-value="effect.effectMutations || []"
                :attr-options="attrOptions"
                :skill-options="skillOptions"
                :buff-options="buffOptions"
                @update:model-value="(value) => updateAt(index, { effectMutations: value })"
              />
            </div>
          </template>
        </div>
      </article>
      <button class="secondary" @click="add">新增效果</button>
    </div>
  `
});

function renderField(field: FieldDef) {
  const row = selectedRow.value;
  if (!row) return null;
  if (field.key === '__enemyAttrs') {
    // Collect attrs actually present in the row or known via scoring (to show "extra" attrs not in fixed groups)
    const knownGroupAttrs = new Set(ENEMY_ATTR_GROUPS.flatMap((g) => g.attrs));
    const extraAttrs = attributeOptions.value.filter((a) => !knownGroupAttrs.has(a));
    const groups = extraAttrs.length > 0
      ? [...ENEMY_ATTR_GROUPS, { key: 'extra', label: '其他', attrs: extraAttrs }]
      : ENEMY_ATTR_GROUPS;

    return h('div', { class: 'attr-matrix-groups' }, groups.map((group) => {
      // only render groups that have at least one non-zero value or input, always show primary
      return h('section', { key: group.key, class: 'attr-matrix-group' }, [
        h('div', { class: 'attr-group-title' }, [
          h('strong', group.label)
        ]),
        h('div', { class: 'attr-matrix attr-matrix--player-style' }, group.attrs.map((attr) => {
          const rawBase = asNumber(row[attr], 0);
          const scoreMeta = enemyAttrScoreMeta(attr, rawBase);
          const derived = enemyDerivedContribution(row, attr);
          const scoringRule = scoringAttribute(attr);
          const isPercent = scoringRule?.displayAsPercent ?? false;
          const finalValue = rawBase + (derived?.contribution ?? 0);
          const attrLabel = localizeKey(`attr.${attr}.name`, attr);
          const emojiIcon = asString(catalog.attributes.find((r: DataRow) => rowId(r) === attr)?.emojiIcon);
          const fmtVal = (v: number) => isPercent ? (v * 100).toFixed(2) + '%' : formatScoreNumber(v);
          return h('label', { key: attr, class: 'attr-matrix-cell' }, [
            h('div', { class: 'attr-cell-header' },
              [emojiIcon ? h('span', { class: 'attr-emoji' }, emojiIcon) : null, h('span', { class: 'attr-name' }, attrLabel)].filter(Boolean)
            ),
            h(NumberInput, {
              modelValue: rawBase,
              step: isPercent ? 0.01 : 1,
              'onUpdate:modelValue': (value: number) => updateEnemyAttr(attr, value)
            }),
            ...(derived ? [
              h('div', { class: 'attr-derived-row', title: derived.parts }, [
                h('span', { class: 'attr-derived-label' }, '派生'),
                h('span', { class: 'attr-derived-value' }, `+${fmtVal(derived.contribution)}`),
                h('span', { class: 'attr-derived-total' }, `= ${fmtVal(finalValue)}`),
              ])
            ] : []),
            h('small', { class: 'attr-score-meta' }, scoreMeta.totalLabel),
          ]);
        }))
      ]);
    }));
  }
  const value = fieldValue(row, field);
  const options = field.options?.() ?? [];
  const update = (next: unknown) => updateField(field.key, next);
  if (field.kind === 'number') {
    return h(NumberInput, { modelValue: asNumber(value, 0), 'onUpdate:modelValue': update });
  }
  if (field.kind === 'boolean') {
    const checkbox = h('label', { class: 'checkbox-field' }, [
      h('input', { type: 'checkbox', checked: Boolean(value), onChange: (event: Event) => update((event.target as HTMLInputElement).checked) }),
      h('span', Boolean(value) ? 'true' : 'false')
    ]);
    const links = fieldLinks(field, row, value);
    if (!links.length) return checkbox;
    return h('div', { class: 'linked-control' }, [
      checkbox,
      ...links.map((link) => h('button', {
        key: link.label,
        type: 'button',
        class: 'link-button',
        title: `跳转到${TABLE_LABELS[link.table]}`,
        onClick: () => jumpToRow(link.table, link.match ?? link.id ?? '')
      }, link.label))
    ]);
  }
  if (field.kind === 'select') {
    const select = h(SearchSelect, { modelValue: asString(value), options, 'onUpdate:modelValue': update });
    const links = fieldLinks(field, row, value);
    if (!links.length) return select;
    return h('div', { class: 'linked-control' }, [
      select,
      ...links.map((link) => h('button', {
        key: link.label,
        type: 'button',
        class: 'link-button',
        title: `跳转到${TABLE_LABELS[link.table]}`,
        onClick: () => jumpToRow(link.table, link.match ?? link.id ?? '')
      }, link.label))
    ]);
  }
  if (field.kind === 'stringList') {
    const link = stringListLink(field);
    return h(StringListEditor, { modelValue: asStringList(value), options, linkTable: link?.table ?? '', linkLabel: link?.label ?? '跳转', 'onUpdate:modelValue': update });
  }
  if (field.kind === 'numberMap') {
    const itemMap = ['entryCosts', 'costs', 'baseCosts', 'stepCosts'].includes(field.key);
    const buildingMap = field.key === 'buildingRequirement';
    const actionCostMap = editorTable.value === 'actions' && field.key === 'costs';
    const linkTable = itemMap ? 'items' : (buildingMap ? 'buildings' : '');
    const linkLabel = itemMap ? '物品' : (buildingMap ? '建筑' : '跳转');
    return h(NumberMapEditor, {
      modelValue: isRecord(value) ? value : {},
      options,
      linkTable,
      linkLabel,
      unitValueMap: actionCostMap ? itemEconomyBaseValueMap.value : {},
      showValueMeta: actionCostMap,
      'onUpdate:modelValue': update
    });
  }
  if (field.kind === 'targeting') return h(TargetingEditor, { modelValue: isRecord(value) ? value : defaultTargeting(), attrOptions: attributeOptions.value, buffOptions: buffOptions.value, allowIgnoreTargetPriority: editorTable.value === 'skills', 'onUpdate:modelValue': update });
  if (field.kind === 'effects') return h(EffectsEditor, { modelValue: asArray(value), stableIdOwner: editorTable.value === 'skills' ? rowId(row) : editorTable.value === 'equipment_templates' && field.key === 'basicAttackEffects' ? `equipment_${rowId(row)}` : '', attrOptions: attributeOptions.value, buffOptions: buffOptions.value, enemyOptions: enemyOptions.value, skillOptions: skillOptions.value, allowEnemyOnlyPierce: editorTable.value === 'skills', allowEnemyOnlyTargeting: editorTable.value === 'skills', scoreSections: effectSectionsForField(field.key), 'onUpdate:modelValue': update });
  if (field.kind === 'castGuards') return h(CastGuardsEditor, { modelValue: asArray(value), 'onUpdate:modelValue': update });
  if (field.kind === 'formula') return h(FormulaEditor, { modelValue: isRecord(value) ? value : defaultFormula(), attrOptions: attributeOptions.value, 'onUpdate:modelValue': update });
  if (field.kind === 'enemySkills') return h(EnemySkillEditor, { modelValue: asStringList(value), options: skillOptions.value, 'onUpdate:modelValue': update });
  if (field.kind === 'enemyEquipment') return h(EnemyEquipmentEditor, { modelValue: asArray(value).filter(isRecord), equipmentRows: catalog.equipment_templates, slots: slotOptions.value, 'onUpdate:modelValue': update });
  if (field.kind === 'enemyDrops') return h(EnemyDropEditor, { modelValue: isRecord(value) ? value : {}, itemOptions: itemOptions.value, 'onUpdate:modelValue': update });
  if (field.kind === 'buffConditions') return h(BuffConditionsEditor, { modelValue: asArray(value), buffOptions: buffOptions.value, weaponClassOptions: weaponClassOptions.value, skillCategoryOptions: skillCategoryOptions.value, 'onUpdate:modelValue': update });
  if (field.kind === 'buffDslEffects') return h(BuffDslEffectsEditor, { modelValue: asArray(value), attrOptions: attributeOptions.value, buffOptions: buffOptions.value, skillOptions: skillOptions.value, skillCategoryOptions: skillCategoryOptions.value, enemyOptions: enemyOptions.value, 'onUpdate:modelValue': update });
  if (field.kind === 'battleHooks') return h(BattleHooksEditor, { modelValue: asArray(value), buffOptions: buffOptions.value, attrOptions: attributeOptions.value, enemyOptions: enemyOptions.value, skillOptions: skillOptions.value, skillCategoryOptions: skillCategoryOptions.value, weaponClassOptions: weaponClassOptions.value, equipmentOptions: equipmentOptions.value, 'onUpdate:modelValue': update });
  if (field.kind === 'attrConversions') return h(AttrConversionEditor, { modelValue: asArray(value), attrOptions: attributeOptions.value, 'onUpdate:modelValue': update });
  if (field.kind === 'defaultAffixSlots') return h(DefaultAffixSlotsEditor, { modelValue: asArray(value).filter(isRecord), affixOptions: affixOptions.value, rarityOptions: rarityOptions.value, slotCount: Math.max(0, Math.trunc(asNumber(row.affixSlotCount, 0))), 'onUpdate:modelValue': update });
  if (field.kind === 'enhanceAffixMilestonePools') return h(EnhanceAffixMilestonePoolsEditor, { modelValue: isRecord(value) ? value : {}, affixOptions: affixOptions.value, rarityOptions: rarityOptions.value, maxLevel: Math.max(0, Math.trunc(asNumber(row.enhanceMaxLevel, 0))), 'onUpdate:modelValue': update });
  if (field.kind === 'enhanceCostCurve') {
    return h('div', { class: 'buff-rule-editor' }, [
      h('div', { class: 'toolbar' }, [
        h('button', { type: 'button', class: 'secondary', onClick: fillEnhanceCostCurveFromMainOutputAction }, '按主产出动作填充基础/增量')
      ]),
      h(EnhanceCurveEditor, { modelValue: isRecord(value) ? value : {}, options: itemOptions.value, linkTable: 'items', valueLabel: '消耗', 'onUpdate:modelValue': update })
    ]);
  }
  if (field.kind === 'enhanceAttrCurve') {
    const buttonLabel = field.key === 'levelAttrCurve' ? '按玩家属性加成填充基础/增量' : '按基础属性填充基础/增量';
    return h('div', { class: 'buff-rule-editor' }, [
      h('div', { class: 'toolbar' }, [
        h('button', { type: 'button', class: 'secondary', onClick: fillEnhanceAttrCurveFromBaseAttrs }, buttonLabel)
      ]),
      h(EnhanceCurveEditor, { modelValue: isRecord(value) ? value : {}, options: attributeOptions.value, linkTable: '', valueLabel: '属性', 'onUpdate:modelValue': update })
    ]);
  }
  if (field.kind === 'affixEffectParams') return h(AffixEffectParamsEditor, {
    modelValue: isRecord(value) ? value : {},
    effectType: asString(row.effectType, 'attr_bonus'),
    rarityOptions: rarityOptions.value,
    attrOptions: attributeOptions.value,
    buffOptions: buffOptions.value,
    skillOptions: skillOptions.value,
    skillCategoryOptions: skillCategoryOptions.value,
    'onUpdate:modelValue': update
  });
  if (field.kind === 'kittyPersonalityEffects') return h(KittyPersonalityEffectsEditor, {
    modelValue: asArray(value),
    attrOptions: attributeOptions.value,
    buffOptions: buffOptions.value,
    skillOptions: skillOptions.value,
    skillCategoryOptions: skillCategoryOptions.value,
    enemyOptions: enemyOptions.value,
    'onUpdate:modelValue': update
  });
  if (field.kind === 'difficultySettings') return h(DifficultySettingsEditor, {
    modelValue: isRecord(value) ? value : {},
    attrOptions: attributeOptions.value,
    itemOptions: itemOptions.value,
    'onUpdate:modelValue': update
  });
  if (field.kind === 'achievementCondition') return h(AchievementConditionEditor, {
    modelValue: isRecord(value) ? value : defaultAchievementCondition('complete_action'),
    enemyOptions: enemyOptions.value,
    fishingSpeciesOptions: fishingSpeciesOptions.value,
    actionOptions: actionIdOptions.value,
    actionDurationMap: actionDurationMap.value,
    itemOptions: itemOptions.value,
    commissionOptions: catalog.cat_village_commissions.map((row) => rowId(row)).filter(Boolean).sort(),
    attrOptions: attributeOptions.value,
    'onUpdate:modelValue': update
  });
  if (field.kind === 'achievementRewards') return h(AchievementRewardsEditor, {
    modelValue: asArray(value).filter(isRecord),
    itemOptions: itemOptions.value,
    titlePrefixOptions: catalog.title_prefixes.map((row) => rowId(row)).filter(Boolean).sort(),
    titleSuffixOptions: catalog.title_suffixes.map((row) => rowId(row)).filter(Boolean).sort(),
    'onUpdate:modelValue': update
  });
  if (field.kind === 'achievementStages') return h(AchievementStagesEditor, {
    modelValue: asArray(value).filter(isRecord),
    enemyOptions: enemyOptions.value,
    fishingSpeciesOptions: fishingSpeciesOptions.value,
    actionOptions: actionIdOptions.value,
    actionDurationMap: actionDurationMap.value,
    stageProgressMode: asString(row.stageProgressMode, 'cumulative'),
    parentActionId: isRecord(row.condition) ? asString(row.condition.actionId) : '',
    itemOptions: itemOptions.value,
    commissionOptions: catalog.cat_village_commissions.map((row) => rowId(row)).filter(Boolean).sort(),
    attrOptions: attributeOptions.value,
    rarityOptions: rarityOptions.value,
    titlePrefixOptions: catalog.title_prefixes.map((row) => rowId(row)).filter(Boolean).sort(),
    titleSuffixOptions: catalog.title_suffixes.map((row) => rowId(row)).filter(Boolean).sort(),
    'onUpdate:modelValue': update
  });
  if (field.kind === 'rangeMap') {
    return h(RangeMapEditor, {
      modelValue: isRecord(value) ? value : {},
      options,
      'onUpdate:modelValue': update
    });
  }
  if (field.kind === 'rangeMapOptions') {
    return h(RangeMapOptionsEditor, {
      modelValue: Array.isArray(value) ? value : [],
      options,
      'onUpdate:modelValue': update
    });
  }
  if (field.kind === 'alchemyGroupEntries') {
    return h(AlchemyGroupEntriesEditor, {
      modelValue: Array.isArray(value) ? value : [],
      itemOptions: field.key === 'upgradeEntries' ? nonMythicItemOptions.value : itemOptions.value,
      itemValueMap: itemEconomyBaseValueMap.value,
      'onUpdate:modelValue': update
    });
  }
  if (field.kind === 'alchemyMediumModifiers') {
    return h(AlchemyMediumModifiersEditor, {
      modelValue: isRecord(value) ? value : {},
      'onUpdate:modelValue': update
    });
  }
  if (field.kind === 'alchemyInheritModifiers') {
    return h(AlchemyInheritModifiersEditor, {
      modelValue: isRecord(value) ? value : {},
      'onUpdate:modelValue': update
    });
  }
  if (field.kind === 'alchemyValueRange') {
    const minKey = field.key === 'returnRatio' ? 'minRatio' : 'min';
    const maxKey = field.key === 'returnRatio' ? 'maxRatio' : 'max';
    return h(AlchemyValueRangeEditor, {
      modelValue: isRecord(value) ? value : {},
      minKey,
      maxKey,
      step: 0.01,
      'onUpdate:modelValue': update
    });
  }
  if (field.kind === 'alchemyAffixOutput') {
    return h(AlchemyAffixOutputEditor, {
      modelValue: isRecord(value) ? value : {},
      attrOptions: attributeOptions.value,
      rarityOptions: rarityOptions.value,
      buffOptions: buffOptions.value,
      skillOptions: skillOptions.value,
      skillCategoryOptions: skillCategoryOptions.value,
      'onUpdate:modelValue': update
    });
  }
  if (field.kind === 'alchemyLuckyTiers') {
    return h(AlchemyLuckyTiersEditor, {
      modelValue: Array.isArray(value) ? value : [],
      itemOptions: itemOptions.value,
      'onUpdate:modelValue': update
    });
  }
  if (field.kind === 'alchemyNameAffix') {
    return h(AlchemyNameAffixEditor, {
      modelValue: isRecord(value) ? value : {},
      'onUpdate:modelValue': update
    });
  }
  if (field.kind === 'alchemyTraitList') {
    return h(AlchemyTraitListEditor, {
      modelValue: Array.isArray(value) ? value : [],
      attrOptions: attributeOptions.value,
      'onUpdate:modelValue': update
    });
  }
  if (field.kind === 'pondFeed') return h(PondFeedEditor, { modelValue: isRecord(value) ? value : {}, 'onUpdate:modelValue': update });
  if (field.key === 'params' && ['alchemy.infusion_bulk_capacity_floor_tradeoff', 'alchemy.infusion_precious_capacity_bonus'].includes(asString(row.kind))) return h(InfusionCapacityParamsEditor, { modelValue: isRecord(value) ? value : {}, kind: asString(row.kind), 'onUpdate:modelValue': update });
  if (field.key === 'parameterScaling') return h(FishFoodScalingEditor, { modelValue: isRecord(value) ? value : {}, 'onUpdate:modelValue': update });
  if (field.key === 'lifetimeScaling') return h(FishFoodScalingEditor, { modelValue: isRecord(value) ? value : {}, lifetime: true, 'onUpdate:modelValue': update });
  if (field.key === 'mutationLifetimeBonuses') return h(FishFoodLifetimeBonusesEditor, { modelValue: isRecord(value) ? value : {}, kind: asString(row.kind), 'onUpdate:modelValue': update });
  if (field.kind === 'json') return h(JsonFieldEditor, { modelValue: value ?? {}, options, path: [field.key], 'onUpdate:modelValue': update });
  return h('input', { value: asString(value), onInput: (event: Event) => update((event.target as HTMLInputElement).value) });
}

const DifficultySettingsEditor = defineComponent({
  name: 'DifficultySettingsEditor',
  components: { SearchSelect, NumberInput },
  props: {
    modelValue: { type: Object as PropType<Record<string, unknown>>, default: () => ({}) },
    attrOptions: { type: Array as PropType<string[]>, default: () => [] },
    itemOptions: { type: Array as PropType<string[]>, default: () => [] }
  },
  emits: ['update:modelValue'],
  setup(props, { emit }) {
    function cloneData() {
      const base = isRecord(props.modelValue) ? props.modelValue : {};
      return {
        maxLevel: asNumber(base.maxLevel, 10),
        statBonuses: asArray(base.statBonuses).filter(isRecord).map((r: Record<string, unknown>) => ({
          attrKey: asString(r.attrKey, ''),
          base: asNumber(r.base, 0),
          perLevelMult: asNumber(r.perLevelMult, 0)
        })),
        specialDrops: asArray(base.specialDrops).filter(isRecord).map((r: Record<string, unknown>) => ({
          itemId: asString(r.itemId, ''),
          percent: asNumber(r.percent, 0),
          rangeMin: asNumber(r.rangeMin, 1),
          rangeMax: asNumber(r.rangeMax, 1),
          minLevel: asNumber(r.minLevel, 1)
        }))
      };
    }
    const data = reactive(cloneData());
    let syncing = false;
    watch(() => props.modelValue, () => { if (!syncing) Object.assign(data, cloneData()); }, { deep: true });
    function emitNext() { syncing = true; emit('update:modelValue', { ...props.modelValue, ...toRaw(data) }); nextTick(() => { syncing = false; }); }
    function patchStatBonus(index: number, patch: Record<string, unknown>) {
      Object.assign(data.statBonuses[index], patch);
      emitNext();
    }
    function addStatBonus() { data.statBonuses.push({attrKey: '', base: 0, perLevelMult: 0}); emitNext(); }
    function removeStatBonus(index: number) { data.statBonuses.splice(index, 1); emitNext(); }
    function patchSpecialDrop(index: number, patch: Record<string, unknown>) {
      Object.assign(data.specialDrops[index], patch);
      emitNext();
    }
    function addSpecialDrop() { data.specialDrops.push({itemId: '', percent: 0, rangeMin: 1, rangeMax: 1, minLevel: 1}); emitNext(); }
    function removeSpecialDrop(index: number) { data.specialDrops.splice(index, 1); emitNext(); }
    return { data, emitNext, attrOptions: computed(() => props.attrOptions), itemOptions: computed(() => props.itemOptions),
      patchStatBonus, addStatBonus, removeStatBonus, patchSpecialDrop, addSpecialDrop, removeSpecialDrop };
  },
  template: `
    <div class="buff-rule-editor">
      <div class="param-section">
        <div class="param-section-title">最高难度等级</div>
        <NumberInput :model-value="data.maxLevel" @update:model-value="data.maxLevel=Math.max(1,Math.min(10,Number($event)));emitNext()" />
      </div>

      <div class="param-section">
        <div class="param-section-title">属性加成 (statBonuses)</div>
        <article class="nested-card" v-for="(sb, i) in data.statBonuses" :key="i">
          <div class="nested-head">
            <strong>属性 #{{ i + 1 }}</strong>
            <button class="danger" @click="removeStatBonus(i)">删除</button>
          </div>
          <div class="form-grid cols-3">
            <label>属性
              <SearchSelect :model-value="sb.attrKey" :options="attrOptions" @update:model-value="(v) => patchStatBonus(i, { attrKey: v })" />
            </label>
            <label>基础(%)
              <NumberInput :model-value="sb.base" @update:model-value="(v) => patchStatBonus(i, { base: Number(v) })" />
            </label>
            <label>每级乘数
              <NumberInput :model-value="sb.perLevelMult" @update:model-value="(v) => patchStatBonus(i, { perLevelMult: Number(v) })" />
            </label>
          </div>
        </article>
        <button class="secondary" @click="addStatBonus">+ 添加属性</button>
      </div>

      <div class="param-section">
        <div class="param-section-title">特殊掉落 (specialDrops)</div>
        <article class="nested-card" v-for="(sd, i) in data.specialDrops" :key="i">
          <div class="nested-head">
            <strong>掉落 #{{ i + 1 }}</strong>
            <button class="danger" @click="removeSpecialDrop(i)">删除</button>
          </div>
          <div class="form-grid cols-5">
            <label>物品
              <SearchSelect :model-value="sd.itemId" :options="itemOptions" @update:model-value="(v) => patchSpecialDrop(i, { itemId: v })" />
            </label>
            <label>概率(0~1)
              <NumberInput :model-value="sd.percent" @update:model-value="(v) => patchSpecialDrop(i, { percent: Math.max(0, Math.min(1, Number(v))) })" />
            </label>
            <label>最小数量
              <NumberInput :model-value="sd.rangeMin" @update:model-value="(v) => patchSpecialDrop(i, { rangeMin: Math.max(1, Math.floor(Number(v))) })" />
            </label>
            <label>最大数量
              <NumberInput :model-value="sd.rangeMax" @update:model-value="(v) => patchSpecialDrop(i, { rangeMax: Math.max(sd.rangeMin, Math.floor(Number(v))) })" />
            </label>
            <label>所需等级
              <NumberInput :model-value="sd.minLevel" @update:model-value="(v) => patchSpecialDrop(i, { minLevel: Math.max(1, Math.min(10, Number(v))) })" />
            </label>
          </div>
        </article>
        <button class="secondary" @click="addSpecialDrop">+ 添加掉落</button>
      </div>
    </div>`
});

const FieldRenderer = defineComponent({
  name: 'FieldRenderer',
  props: {
    field: { type: Object as PropType<FieldDef>, required: true }
  },
  setup(props) {
    return () => renderField(props.field);
  }
});

const EChartView = defineComponent({
  name: 'EChartView',
  props: {
    option: { type: Object as PropType<DropPricingChartOption>, required: true }
  },
  setup(props) {
    const el = ref<HTMLDivElement | null>(null);
    let chart: echarts.ECharts | null = null;
    const resize = () => chart?.resize();
    onMounted(() => {
      if (!el.value) return;
      chart = echarts.init(el.value);
      chart.setOption(props.option, true);
      window.addEventListener('resize', resize);
    });
    watch(
      () => props.option,
      (option) => chart?.setOption(option, true),
      { deep: true }
    );
    onBeforeUnmount(() => {
      window.removeEventListener('resize', resize);
      chart?.dispose();
      chart = null;
    });
    return () => h('div', { ref: el, class: 'drop-echart' });
  }
});
</script>

<template>
  <div class="app-shell">
    <aside class="sidebar">
      <div class="brand">
        <div class="logo">CM</div>
        <div>
          <h1>战斗数据编辑器</h1>
          <p>直接维护 packages/game-data/source/json</p>
        </div>
      </div>

      <div class="sidebar-actions top">
        <button @click="connectDir">授权并加载 game-data</button>
        <button class="secondary" :disabled="saveBusy" @click="saveAll">{{ saveBusy ? '保存中...' : '保存 source/json' }}</button>
        <button class="ghost" @click="validateAll">校验</button>
        <p class="status-text">{{ status }}</p>
      </div>

      <nav class="nav-list">
        <button class="nav-item" :class="{ active: activeView === 'scoring' }" @click="selectTable('scoring')">
          <span>评分配置</span>
          <small>shared</small>
        </button>
        <button class="nav-item" :class="{ active: activeView === 'balance' }" @click="selectTable('balance')">
          <span>数值模型</span>
          <small>{{ balanceIssues.length }}</small>
        </button>
        <button class="nav-item" :class="{ active: activeView === 'drop_pricing' }" @click="selectTable('drop_pricing')">
          <span>{{ t('combatModel.nav.dropPricing') }}</span>
          <small>{{ dropPricingReport.itemPrices.length }}</small>
        </button>
        <button class="nav-item" :class="{ active: activeView === 'enhance' }" @click="selectTable('enhance')">
          <span>强化概率</span>
          <small>curve</small>
        </button>
        <button class="nav-item" :class="{ active: activeView === 'cat_village_rep_curve' }" @click="selectTable('cat_village_rep_curve')">
          <span>猫村声望曲线</span>
          <small>curve</small>
        </button>
        <button class="nav-item" :class="{ active: activeView === 'cat_village_rep_calc' }" @click="selectTable('cat_village_rep_calc')">
          <span>声望等级计算</span>
          <small>calc</small>
        </button>
        <button class="nav-item" :class="{ active: activeView === 'difficulty' }" @click="selectTable('difficulty')">
          <span>副本难度曲线</span>
          <small>curve</small>
        </button>
        <button class="nav-item" :class="{ active: activeView === 'battle_exp_curve' }" @click="selectTable('battle_exp_curve')">
          <span>战斗经验曲线</span>
          <small>curve</small>
        </button>
        <button class="nav-item" :class="{ active: activeView === 'alchemy_review' }" @click="selectTable('alchemy_review')">
          <span>炼金素材审查</span>
          <small>{{ alchemyReviewRows.length }}</small>
        </button>
        <button class="nav-item" :class="{ active: activeView === 'alchemy_inherit_review' }" @click="selectTable('alchemy_inherit_review')">
          <span>炼金融合审查</span>
          <small>{{ alchemyInheritReviewRows.length }}</small>
        </button>
        <button v-for="table in EDITABLE_TABLES" :key="table" class="nav-item" :class="{ active: activeView === table }" @click="selectTable(table)">
          <span>{{ TABLE_LABELS[table] }}</span>
          <small>{{ catalog[table].length }}</small>
        </button>
      </nav>
    </aside>

    <main class="content">
      <section class="page">
        <section v-if="activeView === 'enhance'" class="settings-pane" style="padding:0">
          <EnhanceRateChart />
        </section>
        <section v-if="activeView === 'cat_village_rep_curve'" class="settings-pane" style="padding:0">
          <CatVillageRepChart />
        </section>
        <section v-if="activeView === 'cat_village_rep_calc'" class="settings-pane" style="padding:0">
          <CatVillageRepCalculator />
        </section>
        <section v-if="activeView === 'difficulty'" class="settings-pane" style="padding:0">
          <DifficultyCurveEditor @save="saveDifficultyConfig" />
        </section>
        <section v-if="activeView === 'battle_exp_curve'" class="settings-pane" style="padding:0">
          <BattleExpCurveDesigner :game-data-dir="gameDataDir" @status="(message: string) => (status = message)" />
        </section>
        <section v-if="activeView === 'alchemy_review'" class="settings-pane">
          <div class="section-title">
            <div>
              <h3>炼金素材填充审查</h3>
              <p class="status-text">按填充比例在 1 件到满量之间计算，精度用 x/(100%+|x|) 调整正面上限，稳定同公式调整正面下限并额外影响 25% 上限；负面效果保持原值。</p>
            </div>
            <div class="alchemy-review-toolbar">
              <label class="alchemy-review-fill">
                <span>填充比例</span>
                <input type="range" min="0" max="100" step="1" :value="alchemyReviewFillPercent" @input="(event: Event) => (alchemyReviewFillPercent = asNumber((event.target as HTMLInputElement).value, 100))" />
                <NumberInput :model-value="alchemyReviewFillPercent" @update:model-value="(value: number) => (alchemyReviewFillPercent = Math.max(0, Math.min(100, asNumber(value, 100))))" />
                <small>%</small>
              </label>
              <label>
                <span>炼金精度</span>
                <NumberInput :model-value="alchemyReviewPrecisionPercent" @update:model-value="(value: number) => (alchemyReviewPrecisionPercent = asNumber(value, 0))" />
                <small>%</small>
              </label>
              <label>
                <span>炼金稳定</span>
                <NumberInput :model-value="alchemyReviewStabilityPercent" @update:model-value="(value: number) => (alchemyReviewStabilityPercent = asNumber(value, 0))" />
                <small>%</small>
              </label>
            </div>
          </div>
          <div class="alchemy-review-table">
            <div class="alchemy-review-head">
              <span>素材</span>
              <span>层级</span>
              <span>用量/风险</span>
              <span>正面收益</span>
              <span>负面效果</span>
            </div>
            <div v-for="row in alchemyReviewRows" :key="row.itemId" class="alchemy-review-row">
              <span class="alchemy-review-item">
                <strong>{{ row.itemName }}</strong>
                <small>{{ row.itemId }}</small>
              </span>
              <span>
                <strong>{{ localizeKey(`rarity.${row.rarity}.name`, row.rarity) }}</strong>
                <small>{{ row.kind }} / {{ row.family }} / Lv.{{ row.minAlchemyLevel }}</small>
              </span>
              <span>
                <strong>{{ row.maxQty }} / {{ row.fullMaxQty }}</strong>
                <small>每档 {{ row.unitBasis }}，失败 {{ formatPercent(row.failBaseChance) }}，满量额外 ≤ {{ formatPercent(row.failQuantityCap) }}</small>
              </span>
              <span class="alchemy-review-effects">
                <em v-for="effect in row.positiveEffects" :key="effect.attr" class="good">
                  {{ effect.label }} <strong>{{ effect.totalText }}</strong><small>每件 {{ effect.perUnitText }}</small>
                </em>
                <small v-if="row.positiveEffects.length === 0">-</small>
              </span>
              <span class="alchemy-review-effects">
                <em v-for="effect in row.negativeEffects" :key="effect.attr" class="bad">
                  {{ effect.label }} <strong>{{ effect.totalText }}</strong><small>每件 {{ effect.perUnitText }}</small>
                </em>
                <small v-if="row.negativeEffects.length === 0">-</small>
              </span>
            </div>
          </div>
          <div class="section-title compact-title">
            <div>
              <h3>组合配方随机参数审查</h3>
              <p class="status-text">组合配方统一以 valueRange 随机出 comboPower；召唤属性、Buff payload、触发概率等模板节点会从 comboPower 或配方倍数解析。</p>
            </div>
            <div class="alchemy-review-toolbar">
              <label>
                <span>配方倍数</span>
                <NumberInput :model-value="alchemyReviewComboMultiplier" :min="1" :step="1" @update:model-value="(value: number) => (alchemyReviewComboMultiplier = Math.max(1, Math.round(asNumber(value, 1))))" />
                <small>x</small>
              </label>
            </div>
          </div>
          <div class="alchemy-combo-review-table">
            <div class="alchemy-combo-review-head">
              <span>组合</span>
              <span>输入</span>
              <span>comboPower</span>
              <span>依赖参数解析</span>
            </div>
            <div v-for="row in alchemyComboReviewRows" :key="row.id" class="alchemy-combo-review-row">
              <span>
                <strong>{{ row.name }}</strong>
                <small>{{ row.id }} / Lv.{{ row.minAlchemyLevel }} / {{ row.outputRarity }}</small>
              </span>
              <span><small>{{ row.inputsText }}</small></span>
              <span><strong>{{ row.comboPowerText }}</strong></span>
              <span class="alchemy-review-effects">
                <em v-for="template in row.templates" :key="template.pathText" class="combo">
                  <strong>{{ template.title }}</strong>
                  <small>{{ template.detail }}</small>
                  <small>{{ template.sourceLabel }}: base {{ template.base }}, coeff {{ template.coeff }}</small>
                  <small>{{ template.rangeText }}</small>
                </em>
                <small v-if="row.templates.length === 0">无模板参数</small>
              </span>
            </div>
          </div>
        </section>
        <section v-if="activeView === 'alchemy_inherit_review'" class="settings-pane">
          <div class="section-title">
            <div>
              <h3>炼金融合介质审查</h3>
              <p class="status-text">独立读取 alchemy_inherit_mediums，检查保留/抑制额度、费用、不干预保留率、融合递减、同源畸变和三属性缩放；普通 affix 参与后生成断代产物，继续生效但不能再次融合；不读取灌注介质字段。</p>
            </div>
            <div class="alchemy-review-toolbar">
              <label>
                <span>角色炼金等级</span>
                <NumberInput :model-value="alchemyInheritReviewLevel" :min="1" :step="1" @update:model-value="(value: number) => (alchemyInheritReviewLevel = Math.max(1, Math.round(asNumber(value, 40))))" />
                <small>Lv</small>
              </label>
            </div>
          </div>
          <div class="alchemy-combo-review-table">
            <div class="alchemy-combo-review-head">
              <span>介质</span>
              <span>额度/费用</span>
              <span>遗传概率</span>
              <span>融合与缩放</span>
            </div>
            <div v-for="row in alchemyInheritReviewRows" :key="row.id" class="alchemy-combo-review-row">
              <span>
                <strong>{{ row.itemName }}</strong>
                <small>{{ row.id }} / {{ row.itemId }} / x{{ row.consumePerCraft }} / Lv.{{ row.minAlchemyLevel }}</small>
              </span>
              <span>
                <strong>保留 {{ row.inheritSlots }} / 抑制 {{ row.suppressSlots }}</strong>
                <small>保留 {{ row.inheritCostText }}</small>
                <small>抑制 {{ row.suppressCostText }}</small>
              </span>
              <span>
                <strong>不干预保留 {{ formatPercent(row.ignoreKeepChance) }}</strong>
                <small>正面保留 +{{ formatPercent(row.positiveIgnoreKeepChanceBonus) }} / 负面丢弃 +{{ formatPercent(row.negativeIgnoreSuppressChanceBonus) }}</small>
                <small>变异 {{ formatPercent(row.mutationChance) }}</small>
                <small>{{ row.inbreedText }}</small>
              </span>
              <span class="alchemy-review-effects">
                <em class="combo">
                  <strong>异源融合</strong>
                  <small>{{ row.hybridText }}</small>
                </em>
                <em class="combo">
                  <strong>三属性</strong>
                  <small>{{ row.scalingText }}</small>
                </em>
              </span>
            </div>
          </div>
        </section>
        <section v-if="activeView === 'scoring'" class="settings-pane">
          <div class="section-title">
            <div>
              <h3>评分配置</h3>
              <p class="status-text">默认读取打包时的 shared-global-config.json；授权文件后可直接保存 combatScoring。</p>
            </div>
            <div class="toolbar">
              <button class="secondary" @click="requestScoringConfigFile">授权配置文件</button>
              <button @click="saveScoringConfigFile">保存评分配置</button>
              <button class="secondary" @click="addScoringAttribute">新增评分属性</button>
            </div>
          </div>
          <div class="rule-grid">
            <label v-for="(_, key) in scoring.constants" :key="key">
              <span>{{ key }}</span>
              <NumberInput :model-value="(scoring.constants as Constants)[key as keyof Constants]" @update:model-value="(value: number) => ((scoring.constants as Constants)[key as keyof Constants] = value)" />
            </label>
          </div>
          <div class="scoring-attrs">
            <div v-for="(attr, index) in scoring.attributes" :key="attr.key" class="attr-rule">
              <input v-model="(attr as AttributeValue).key" />
              <input v-model="(attr as AttributeValue).label" />
              <NumberInput :model-value="(attr as AttributeValue).unitValue" @update:model-value="(value: number) => ((attr as AttributeValue).unitValue = value)" />
              <label class="checkbox-field"><input type="checkbox" v-model="(attr as AttributeValue).displayAsPercent" />百分比</label>
              <button class="danger" @click="removeScoringAttribute(index)">删</button>
            </div>
          </div>
          <section class="advanced-section">
            <div class="section-title">
              <div>
                <h3>敌人属性模板</h3>
                <p class="status-text">等级/职业/种族模板已移至「战斗数值模型」页编辑，保存数值模型即可持久化。</p>
              </div>
            </div>
          </section>
        </section>
        <section v-if="activeView === 'drop_pricing'" class="settings-pane">
          <div class="section-title">
            <div>
              <h3>{{ t('combatModel.dropPricing.title') }}</h3>
              <p class="status-text">{{ t('combatModel.dropPricing.subtitle') }}</p>
            </div>
            <div class="toolbar">
              <button class="secondary" @click="requestScoringConfigFile">授权配置文件</button>
              <button @click="saveScoringConfigFile">保存模型参数</button>
              <button class="secondary" :disabled="saveBusy" @click="saveAll">{{ saveBusy ? '保存中...' : '保存物品价格' }}</button>
            </div>
          </div>
          <section class="advanced-section">
            <div class="rule-grid">
              <label v-for="key in ['targetGoldPerSecond','dropBudgetShare','defaultPlayerCount','waveScorePerSecond','specialWaveHorizon','worldFloorGrowth','worldFloorMaxMultiplier','enemyLevelPriceGrowth','enemyLevelPriceMaxMultiplier','ignoreExistingPriceBelow','overpricedWarnRatio','enemyTemplateDeviationWarnRatio']" :key="key">
                <span class="field-label-with-help">{{ dropPricingParamMeta(key).label }}<button type="button" class="help-dot" :title="dropPricingParamMeta(key).help">?</button></span>
                <NumberInput
                  :model-value="asNumber(scoring.dropPricing[key as keyof typeof scoring.dropPricing], 0)"
                  @update:model-value="(v: number) => {
                    if (key === 'defaultPlayerCount') scoring.dropPricing.defaultPlayerCount = Math.max(1, Math.round(v));
                    else if (key === 'waveScorePerSecond') scoring.dropPricing.waveScorePerSecond = Math.max(0.0001, v);
                    else if (key === 'specialWaveHorizon') scoring.dropPricing.specialWaveHorizon = Math.max(1, Math.round(v));
                    else if (key === 'worldFloorMaxMultiplier') scoring.dropPricing.worldFloorMaxMultiplier = Math.max(1, v);
                    else if (key === 'enemyLevelPriceMaxMultiplier') scoring.dropPricing.enemyLevelPriceMaxMultiplier = Math.max(1, v);
                    else if (key === 'overpricedWarnRatio') scoring.dropPricing.overpricedWarnRatio = Math.max(1, v);
                    else (scoring.dropPricing as Record<string, unknown>)[key] = Math.max(0, v);
                  }"
                />
              </label>
            </div>
            <div class="drop-pricing-summary">
              <span>{{ t('combatModel.dropPricing.priced') }} <strong>{{ dropPricingReport.fitSummary.pricedCount }}</strong></span>
              <span>{{ t('combatModel.dropPricing.comparable') }} <strong>{{ dropPricingReport.fitSummary.comparableCount }}</strong></span>
              <span>{{ t('combatModel.dropPricing.placeholder') }} <strong>{{ dropPricingReport.fitSummary.placeholderCount }}</strong></span>
              <span>MAE <strong>{{ formatGoldDisplay(dropPricingReport.fitSummary.mae) }}</strong></span>
              <span>MAPE <strong>{{ formatScoreNumber(dropPricingReport.fitSummary.mape) }}%</strong></span>
              <span>{{ t('combatModel.dropPricing.overpriced') }} <strong>{{ dropPricingReport.fitSummary.overpricedCount }}</strong></span>
            </div>
          </section>

          <section class="advanced-section">
            <div class="drop-type-table">
              <div class="drop-type-head"><span>{{ t('combatModel.dropPricing.enemyType') }}</span><span>{{ t('combatModel.dropPricing.dropBudgetWeight') }}</span></div>
              <div v-for="type in ['normal', 'elite', 'boss', 'world_boss']" :key="type" class="drop-type-row">
                <strong>{{ type === 'normal' ? t('combatModel.dropPricing.normal') : type === 'elite' ? t('combatModel.dropPricing.elite') : type === 'boss' ? t('combatModel.dropPricing.boss') : t('combatModel.dropPricing.worldBoss') }}</strong>
                <NumberInput
                  :model-value="scoring.dropPricing.enemyTypeWeights[type] ?? 1"
                  @update:model-value="(v: number) => (scoring.dropPricing.enemyTypeWeights[type] = Math.max(0, v))"
                />
              </div>
            </div>
            <div class="drop-rarity-table">
              <div class="drop-rarity-head"><span>{{ t('combatModel.dropPricing.rarity') }}</span><span>{{ t('combatModel.dropPricing.weight') }}</span><span>{{ t('combatModel.dropPricing.softFloor') }}</span><span>{{ t('combatModel.dropPricing.hardCap') }}</span><span>{{ t('combatModel.dropPricing.fit') }}</span></div>
              <div v-for="rule in scoring.dropPricing.rarityRules" :key="rule.key" class="drop-rarity-row">
                <strong>{{ optionLabel(rule.key) || rule.key }}</strong>
                <NumberInput :model-value="rule.weight" @update:model-value="(v: number) => patchDropPricingRarityRule(rule, { weight: Math.max(0, v) })" />
                <NumberInput :model-value="rule.softFloor" @update:model-value="(v: number) => patchDropPricingRarityRule(rule, { softFloor: Math.max(0, v) })" />
                <NumberInput :model-value="rule.hardCap" @update:model-value="(v: number) => patchDropPricingRarityRule(rule, { hardCap: Math.max(0, v) })" />
                <small>
                  n={{ dropPricingReport.fitSummary.groups.find((group) => group.key === rule.key)?.count ?? 0 }}
                  / MAPE {{ formatScoreNumber(dropPricingReport.fitSummary.groups.find((group) => group.key === rule.key)?.mape ?? 0) }}%
                </small>
              </div>
            </div>
          </section>

          <section class="advanced-section">
            <div class="drop-chart-card">
              <div class="drop-chart-title">
                <strong>{{ t('combatModel.dropPricing.sourceChartTitle') }}</strong>
                <small>{{ t('combatModel.dropPricing.sourceChartDesc') }}</small>
              </div>
              <div class="drop-chart-scroll">
                <EChartView v-if="dropPricingChartRows.length" :option="dropPricingChartOption" />
                <p v-else class="status-text">{{ t('combatModel.dropPricing.noSource') }}</p>
              </div>
            </div>
          </section>

          <section class="advanced-section">
            <div class="section-title">
              <div>
                <h3>{{ t('combatModel.dropPricing.item') }} / {{ t('combatModel.dropPricing.expected') }}</h3>
                <p class="status-text">{{ t('combatModel.dropPricing.sourceChartDesc') }}</p>
              </div>
            </div>
            <div class="drop-source-groups">
              <article v-for="group in dropPricingItemWorldGroups" :key="group.worldId" class="drop-source-group">
                <button type="button" class="drop-source-group-head" @click="toggleDropSourceWorld(group.worldId)">
                  <strong>{{ group.label }}</strong>
                  <small>{{ group.rows.length }} · {{ t('combatModel.dropPricing.maxPrice') }} {{ formatGoldDisplay(group.maxPrice) }}</small>
                  <span>{{ isDropSourceWorldCollapsed(group.worldId) ? '＋' : '－' }}</span>
                </button>
                <div v-if="!isDropSourceWorldCollapsed(group.worldId)" class="drop-pricing-table grouped-drop-pricing-table">
                  <div class="drop-pricing-head editable-drop-head">
                    <span>{{ t('combatModel.dropPricing.item') }}</span><span>{{ t('combatModel.dropPricing.current') }}</span><span>{{ t('combatModel.dropPricing.suggested') }}</span><span>{{ t('combatModel.dropPricing.expected') }}</span><span>{{ t('combatModel.dropPricing.source') }}</span><span>{{ t('combatModel.dropPricing.enemyScore') }}</span><span>{{ t('combatModel.dropPricing.risk') }}</span>
                  </div>
                  <div v-for="row in group.rows" :key="row.itemId" class="drop-pricing-row editable-drop-row">
                    <span>
                      <strong>{{ localizeKey(`item.${row.itemId}.name`, row.itemName) }}</strong>
                      <small>
                        {{ row.itemId }} · {{ row.pricingRarity }}
                        <template v-if="row.catalogRarity !== row.pricingRarity"> / {{ row.catalogRarity }}</template>
                        <template v-if="row.skillbookAnchorEnemyType"> / {{ row.skillbookAnchorEnemyType }}</template>
                      </small>
                    </span>
                    <span>{{ formatGoldDisplay(row.currentValue) }}</span>
                    <span>{{ formatGoldDisplay(row.suggestedValue) }}</span>
                    <span><NumberInput :model-value="row.currentValue" @update:model-value="(v: number) => patchDropPricingItemValue(row.itemId, v)" /></span>
                    <span>{{ row.mainWorldId }} / {{ row.mainEnemyId }}<small>{{ t('combatModel.dropPricing.level') }} {{ row.mainEnemyLevel }} · {{ t('combatModel.dropPricing.dropRate') }} {{ formatPercent(row.mainDropPercent) }} · {{ t('combatModel.dropPricing.sourceCount') }} {{ row.sourceCount }}</small></span>
                    <span>{{ formatScoreNumber(row.mainEnemyActualScore) }} / {{ formatScoreNumber(row.mainEnemyTemplateScore) }}<small>{{ dropPricingDeltaText(row.mainEnemyTemplateDeviation) }}</small></span>
                    <span class="risk-tags"><em v-for="flag in row.riskFlags" :key="flag" :class="dropPricingRiskClass(flag)">{{ flag }}</em><small v-if="row.riskFlags.length === 0">正常</small></span>
                  </div>
                </div>
              </article>
            </div>
          </section>
        </section>
        <section v-if="activeView === 'balance'" class="settings-pane">
          <div class="section-title">
            <div>
              <h3>战斗数值模型</h3>
              <p class="status-text">等级基准、递减防御 K、经验换算、组队收益和 combo 预算边界。</p>
            </div>
            <div class="toolbar">
              <button class="secondary" @click="requestScoringConfigFile">授权配置文件</button>
              <button @click="saveScoringConfigFile">保存数值模型</button>
            </div>
          </div>

          <div class="rule-grid">
            <label>
              <span>防御饱和常数 K</span>
              <small style="color:#73809a;font-size:10px">K防御=50%减免 (K=100时，100防御=50%减免)</small>
              <NumberInput :model-value="balanceModel.defenseK" @update:model-value="(v: number) => (balanceModel.defenseK = Math.max(1, v))" />
            </label>
            <label><span>maxBattleLevel</span><NumberInput :model-value="balanceModel.maxBattleLevel" @update:model-value="(v: number) => (balanceModel.maxBattleLevel = Math.max(1, Math.round(v)))" /></label>
            <label><span>primaryPointCap</span><NumberInput :model-value="balanceModel.primaryPointCap" @update:model-value="(v: number) => (balanceModel.primaryPointCap = Math.max(1, Math.round(v)))" /></label>
            <label><span>expPerPower</span><NumberInput :model-value="balanceModel.expPerPower" @update:model-value="(v: number) => (balanceModel.expPerPower = Math.max(0.0001, v))" /></label>
            <label><span>teamExtraEnemyScale</span><NumberInput :model-value="balanceModel.teamExtraEnemyScale" @update:model-value="(v: number) => (balanceModel.teamExtraEnemyScale = Math.max(0, v))" /></label>
            <label><span>teamRewardEfficiencyCap</span><NumberInput :model-value="balanceModel.teamRewardEfficiencyCap" @update:model-value="(v: number) => (balanceModel.teamRewardEfficiencyCap = Math.max(1, v))" /></label>
            <label><span>comboBudgetMinShare</span><NumberInput :model-value="balanceModel.comboBudgetMinShare" @update:model-value="(v: number) => (balanceModel.comboBudgetMinShare = Math.max(0, v))" /></label>
            <label><span>comboBudgetMaxShare</span><NumberInput :model-value="balanceModel.comboBudgetMaxShare" @update:model-value="(v: number) => (balanceModel.comboBudgetMaxShare = Math.max(balanceModel.comboBudgetMinShare, v))" /></label>
          </div>

          <section v-if="false" class="advanced-section">
            <div class="section-title">
              <div>
                <h3>掉落定价模型</h3>
                <p class="status-text">只生成参考价，不写入物品价格；模型按非世界首领掉落、期望刷怪权重和战斗秒收益保守估算。</p>
              </div>
            </div>
            <div class="rule-grid">
              <label>
                <span class="field-label-with-help">{{ dropPricingParamMeta('targetGoldPerSecond').label }}<button type="button" class="help-dot" :title="dropPricingParamMeta('targetGoldPerSecond').help">?</button></span>
                <NumberInput :model-value="scoring.dropPricing.targetGoldPerSecond" @update:model-value="(v: number) => (scoring.dropPricing.targetGoldPerSecond = Math.max(0, v))" />
              </label>
              <label>
                <span class="field-label-with-help">{{ dropPricingParamMeta('dropBudgetShare').label }}<button type="button" class="help-dot" :title="dropPricingParamMeta('dropBudgetShare').help">?</button></span>
                <NumberInput :model-value="scoring.dropPricing.dropBudgetShare" @update:model-value="(v: number) => (scoring.dropPricing.dropBudgetShare = Math.max(0, v))" />
              </label>
              <label>
                <span class="field-label-with-help">{{ dropPricingParamMeta('defaultPlayerCount').label }}<button type="button" class="help-dot" :title="dropPricingParamMeta('defaultPlayerCount').help">?</button></span>
                <NumberInput :model-value="scoring.dropPricing.defaultPlayerCount" @update:model-value="(v: number) => (scoring.dropPricing.defaultPlayerCount = Math.max(1, Math.round(v)))" />
              </label>
              <label>
                <span class="field-label-with-help">{{ dropPricingParamMeta('waveScorePerSecond').label }}<button type="button" class="help-dot" :title="dropPricingParamMeta('waveScorePerSecond').help">?</button></span>
                <NumberInput :model-value="scoring.dropPricing.waveScorePerSecond" @update:model-value="(v: number) => (scoring.dropPricing.waveScorePerSecond = Math.max(0.0001, v))" />
              </label>
              <label>
                <span class="field-label-with-help">{{ dropPricingParamMeta('specialWaveHorizon').label }}<button type="button" class="help-dot" :title="dropPricingParamMeta('specialWaveHorizon').help">?</button></span>
                <NumberInput :model-value="scoring.dropPricing.specialWaveHorizon" @update:model-value="(v: number) => (scoring.dropPricing.specialWaveHorizon = Math.max(1, Math.round(v)))" />
              </label>
              <label>
                <span class="field-label-with-help">{{ dropPricingParamMeta('worldFloorGrowth').label }}<button type="button" class="help-dot" :title="dropPricingParamMeta('worldFloorGrowth').help">?</button></span>
                <NumberInput :model-value="scoring.dropPricing.worldFloorGrowth" @update:model-value="(v: number) => (scoring.dropPricing.worldFloorGrowth = Math.max(0, v))" />
              </label>
              <label>
                <span class="field-label-with-help">{{ dropPricingParamMeta('worldFloorMaxMultiplier').label }}<button type="button" class="help-dot" :title="dropPricingParamMeta('worldFloorMaxMultiplier').help">?</button></span>
                <NumberInput :model-value="scoring.dropPricing.worldFloorMaxMultiplier" @update:model-value="(v: number) => (scoring.dropPricing.worldFloorMaxMultiplier = Math.max(1, v))" />
              </label>
              <label>
                <span class="field-label-with-help">{{ dropPricingParamMeta('enemyLevelPriceGrowth').label }}<button type="button" class="help-dot" :title="dropPricingParamMeta('enemyLevelPriceGrowth').help">?</button></span>
                <NumberInput :model-value="scoring.dropPricing.enemyLevelPriceGrowth" @update:model-value="(v: number) => (scoring.dropPricing.enemyLevelPriceGrowth = Math.max(0, v))" />
              </label>
              <label>
                <span class="field-label-with-help">{{ dropPricingParamMeta('enemyLevelPriceMaxMultiplier').label }}<button type="button" class="help-dot" :title="dropPricingParamMeta('enemyLevelPriceMaxMultiplier').help">?</button></span>
                <NumberInput :model-value="scoring.dropPricing.enemyLevelPriceMaxMultiplier" @update:model-value="(v: number) => (scoring.dropPricing.enemyLevelPriceMaxMultiplier = Math.max(1, v))" />
              </label>
              <label>
                <span class="field-label-with-help">{{ dropPricingParamMeta('ignoreExistingPriceBelow').label }}<button type="button" class="help-dot" :title="dropPricingParamMeta('ignoreExistingPriceBelow').help">?</button></span>
                <NumberInput :model-value="scoring.dropPricing.ignoreExistingPriceBelow" @update:model-value="(v: number) => (scoring.dropPricing.ignoreExistingPriceBelow = Math.max(0, v))" />
              </label>
              <label>
                <span class="field-label-with-help">{{ dropPricingParamMeta('overpricedWarnRatio').label }}<button type="button" class="help-dot" :title="dropPricingParamMeta('overpricedWarnRatio').help">?</button></span>
                <NumberInput :model-value="scoring.dropPricing.overpricedWarnRatio" @update:model-value="(v: number) => (scoring.dropPricing.overpricedWarnRatio = Math.max(1, v))" />
              </label>
              <label>
                <span class="field-label-with-help">{{ dropPricingParamMeta('enemyTemplateDeviationWarnRatio').label }}<button type="button" class="help-dot" :title="dropPricingParamMeta('enemyTemplateDeviationWarnRatio').help">?</button></span>
                <NumberInput :model-value="scoring.dropPricing.enemyTemplateDeviationWarnRatio" @update:model-value="(v: number) => (scoring.dropPricing.enemyTemplateDeviationWarnRatio = Math.max(0, v))" />
              </label>
            </div>
            <div class="drop-pricing-summary">
              <span>已定价 <strong>{{ dropPricingReport.fitSummary.pricedCount }}</strong></span>
              <span>参与拟合 <strong>{{ dropPricingReport.fitSummary.comparableCount }}</strong></span>
              <span>占位价 <strong>{{ dropPricingReport.fitSummary.placeholderCount }}</strong></span>
              <span>MAE <strong>{{ formatGoldDisplay(dropPricingReport.fitSummary.mae) }}</strong></span>
              <span>MAPE <strong>{{ formatScoreNumber(dropPricingReport.fitSummary.mape) }}%</strong></span>
              <span>过高风险 <strong>{{ dropPricingReport.fitSummary.overpricedCount }}</strong></span>
            </div>
            <div class="drop-type-table">
              <div class="drop-type-head"><span>敌人类型</span><span>掉落预算权重</span></div>
              <div v-for="type in ['normal', 'elite', 'boss', 'world_boss']" :key="type" class="drop-type-row">
                <strong>{{ type === 'normal' ? '普通' : type === 'elite' ? '精英' : type === 'boss' ? 'Boss' : '世界 Boss' }}</strong>
                <NumberInput
                  :model-value="scoring.dropPricing.enemyTypeWeights[type] ?? 1"
                  @update:model-value="(v: number) => (scoring.dropPricing.enemyTypeWeights[type] = Math.max(0, v))"
                />
              </div>
            </div>
            <div class="drop-rarity-table">
              <div class="drop-rarity-head"><span>稀有度</span><span>权重</span><span>软下限</span><span>硬上限</span><span>拟合</span></div>
              <div v-for="rule in scoring.dropPricing.rarityRules" :key="rule.key" class="drop-rarity-row">
                <strong>{{ optionLabel(rule.key) || rule.key }}</strong>
                <NumberInput :model-value="rule.weight" @update:model-value="(v: number) => patchDropPricingRarityRule(rule, { weight: Math.max(0, v) })" />
                <NumberInput :model-value="rule.softFloor" @update:model-value="(v: number) => patchDropPricingRarityRule(rule, { softFloor: Math.max(0, v) })" />
                <NumberInput :model-value="rule.hardCap" @update:model-value="(v: number) => patchDropPricingRarityRule(rule, { hardCap: Math.max(0, v) })" />
                <small>
                  n={{ dropPricingReport.fitSummary.groups.find((group) => group.key === rule.key)?.count ?? 0 }}
                  / MAPE {{ formatScoreNumber(dropPricingReport.fitSummary.groups.find((group) => group.key === rule.key)?.mape ?? 0) }}%
                </small>
              </div>
            </div>
            <div class="drop-chart-card">
              <div class="drop-chart-title">
                <strong>掉落来源价格折线图</strong>
                <small>x轴按副本 / 敌人 / 掉落物排序，y轴为来源参考价；同一物品最终仍按最低来源价锚定。</small>
              </div>
              <div class="drop-chart-scroll">
                <EChartView v-if="dropPricingChartRows.length" :option="dropPricingChartOption" />
                <p v-else class="status-text">暂无可展示的掉落来源。</p>
              </div>
            </div>
            <div class="drop-pricing-table">
              <div class="drop-pricing-head">
                <span>物品</span><span>当前</span><span>建议</span><span>差异</span><span>主来源</span><span>敌人评分</span><span>风险</span>
              </div>
              <div v-for="row in dropPricingPreviewRows" :key="row.itemId" class="drop-pricing-row">
                <span>
                  <strong>{{ localizeKey(`item.${row.itemId}.name`, row.itemName) }}</strong>
                  <small>
                    {{ row.itemId }} · 定价 {{ row.pricingRarity }}
                    <template v-if="row.catalogRarity !== row.pricingRarity"> / 原始 {{ row.catalogRarity }}</template>
                    <template v-if="row.skillbookAnchorEnemyType"> / 锚定 {{ row.skillbookAnchorEnemyType }}</template>
                  </small>
                </span>
                <span>{{ formatGoldDisplay(row.currentValue) }}</span>
                <span>{{ formatGoldDisplay(row.suggestedValue) }}</span>
                <span>{{ formatGoldDisplay(row.delta) }} / {{ dropPricingDeltaText(row.deltaRatio) }}</span>
                <span>{{ row.mainWorldId }} / {{ row.mainEnemyId }}<small>等级 {{ row.mainEnemyLevel }} · 掉率 {{ formatPercent(row.mainDropPercent) }} · 类型 x{{ formatScoreNumber(row.mainEnemyTypeWeight) }} · 等级 x{{ formatScoreNumber(row.mainEnemyLevelPriceMultiplier) }} · 来源 {{ row.sourceCount }}</small></span>
                <span>{{ formatScoreNumber(row.mainEnemyActualScore) }} / {{ formatScoreNumber(row.mainEnemyTemplateScore) }}<small>偏离 {{ dropPricingDeltaText(row.mainEnemyTemplateDeviation) }}</small></span>
                <span class="risk-tags"><em v-for="flag in row.riskFlags" :key="flag" :class="dropPricingRiskClass(flag)">{{ flag }}</em><small v-if="row.riskFlags.length === 0">正常</small></span>
              </div>
            </div>
          </section>

          <section class="advanced-section">
            <div class="section-title">
              <div>
                <h3>等级基准曲线</h3>
                <p class="status-text">用于倒推敌人生命、经验和属性派生强度。</p>
              </div>
            </div>
            <div class="balance-table">
              <div class="balance-table-head"><span>等级</span><span>DPR</span><span>HP</span><span>MP</span><span>速度</span><span>ExpRate</span></div>
              <div v-for="band in balanceBaselineRows()" :key="band.level" class="balance-table-row">
                <NumberInput :model-value="band.level" @update:model-value="(v: number) => (band.level = Math.max(1, Math.round(v)))" />
                <NumberInput :model-value="band.playerDpr" @update:model-value="(v: number) => (band.playerDpr = Math.max(1, v))" />
                <NumberInput :model-value="band.playerHp" @update:model-value="(v: number) => (band.playerHp = Math.max(1, v))" />
                <NumberInput :model-value="band.playerMp" @update:model-value="(v: number) => (band.playerMp = Math.max(0, v))" />
                <NumberInput :model-value="band.playerSpeed" @update:model-value="(v: number) => (band.playerSpeed = Math.max(0, v))" />
                <NumberInput :model-value="band.expRate" @update:model-value="(v: number) => (band.expRate = Math.max(0.0001, v))" />
              </div>
            </div>
            <p class="status-text">主属性点：{{ balanceModel.primaryPointCap }} 点，允许分配到 {{ balanceModel.primaryPointAttrs.map(optionLabel).join(' / ') }}；幸运不在手动加点池中。</p>
          </section>

          <!-- Section A: 敌人等级模板 -->
          <section class="advanced-section">
            <div class="section-title">
              <div>
                <h3>敌人等级模板</h3>
                <p class="status-text">tier 决定敌人的难度、奖励和预算规模。</p>
              </div>
              <button class="secondary small" @click="addTemplateEntry('tiers')">＋ 新增等级</button>
            </div>
            <div class="balance-table">
              <div class="balance-table-head tier-head">
                <span>标签 / key</span><span>计分权重</span><span>奖励倍率</span><span>预算倍率</span><span>HP倍率</span><span>压力%/回合</span><span>轮数 min</span><span>轮数 max</span><span></span><span></span>
              </div>
              <div v-for="tier in scoring.enemyAttributeTemplates.tiers" :key="tier.key" class="balance-table-row tier-row">
                <span class="tier-label">
                  <input v-model="tier.label" class="inline-label-input" />
                  <small>{{ tier.key }}</small>
                </span>
                <NumberInput :model-value="tier.scoreMult" @update:model-value="(v: number) => (tier.scoreMult = Math.max(0, v))" />
                <NumberInput :model-value="tier.rewardMult" @update:model-value="(v: number) => (tier.rewardMult = Math.max(0, v))" />
                <NumberInput :model-value="tier.budgetMultiplier" @update:model-value="(v: number) => (tier.budgetMultiplier = Math.max(0, v))" />
                <NumberInput :model-value="tier.hpMultiplier ?? 1" @update:model-value="(v: number) => (tier.hpMultiplier = Math.max(0, v))" />
                <NumberInput :model-value="tier.pressurePct" @update:model-value="(v: number) => (tier.pressurePct = Math.max(0, Math.min(1, v)))" />
                <NumberInput :model-value="tier.targetRounds.min" @update:model-value="(v: number) => (tier.targetRounds.min = Math.max(1, Math.round(v)))" />
                <NumberInput :model-value="tier.targetRounds.max" @update:model-value="(v: number) => (tier.targetRounds.max = Math.max(tier.targetRounds.min, Math.round(v)))" />
                <button class="icon-btn small" :title="`自动填充 ${tier.key} 参数`" @click="autoFillTierParams(tier)">⚡</button>
                <button class="icon-btn danger" :title="`删除 ${tier.key}`" @click="removeTemplateEntry('tiers', tier.key)">×</button>
              </div>
            </div>
          </section>

          <!-- Section B: 战斗职业模板 -->
          <section class="advanced-section">
            <div class="section-title">
              <div>
                <h3>战斗职业模板</h3>
                <p class="status-text">role 决定数值分配方向。点击行展开权重编辑。权重为相对分值，可正可负。</p>
              </div>
              <button class="secondary small" @click="addTemplateEntry('roles')">＋ 新增职业</button>
            </div>
            <div class="template-list">
              <div v-for="role in scoring.enemyAttributeTemplates.roles" :key="role.key" class="template-item">
                <button class="template-header" @click="toggleExpandRole(role.key)">
                  <strong>{{ role.label }}</strong>
                  <small>{{ role.key }}</small>
                  <div class="weight-bar-mini">
                    <div v-for="bar in templateWeightBar(role.weights)" :key="bar.key" class="weight-bar-seg" :style="{ width: bar.width + '%' }" :title="`${bar.label}: ${bar.width}%`" />
                  </div>
                  <span class="toggle-icon">{{ expandedRoles.has(role.key) ? '▲' : '▼' }}</span>
                </button>
                <div v-if="expandedRoles.has(role.key)" class="template-body">
                  <div class="template-meta-row">
                    <label class="template-label-field"><span>名称</span><input v-model="role.label" class="inline-label-input" /></label>
                    <button class="icon-btn danger small" :title="`删除职业 ${role.key}`" @click="removeTemplateEntry('roles', role.key)">× 删除</button>
                  </div>
                  <div class="radar-wrap">
                    <svg width="120" height="120" viewBox="0 0 120 120">
                      <polygon :points="radarRolePoints(role.weights)" fill="rgba(99,102,241,0.25)" stroke="rgba(99,102,241,0.8)" stroke-width="1.5" />
                      <polygon v-if="previewSpeciesData" :points="radarMergedPoints(role.weights, previewSpeciesData.weights)" fill="rgba(234,88,12,0.12)" stroke="rgba(234,88,12,0.6)" stroke-width="1" />
                    </svg>
                  </div>
                  <div class="weight-section">
                    <div class="weight-section-title">权重增量</div>
                    <div class="weight-kv">
                      <div v-for="(val, wkey) in role.weights" :key="wkey" class="weight-row">
                        <span class="weight-attr-label" :title="String(wkey)">{{ attrLabel(String(wkey)) }}</span>
                        <NumberInput :model-value="val" @update:model-value="(v: number) => (role.weights[wkey] = v)" />
                        <button class="icon-btn danger" @click="removeAttrFromMap(role.weights, String(wkey))">×</button>
                      </div>
                      <div class="weight-add-row">
                        <select class="attr-select" @change="(e) => { addAttrToMap(role.weights, (e.target as HTMLSelectElement).value); (e.target as HTMLSelectElement).value = ''; }">
                          <option value="">＋ 添加属性权重…</option>
                          <option v-for="attr in scoring.attributes" :key="attr.key" :value="attr.key" :disabled="attr.key in role.weights">
                            {{ attr.label }}（{{ attr.key }}）
                          </option>
                        </select>
                      </div>
                    </div>
                  </div>
                  <div class="skill-types-row">
                    <span class="status-text">技能偏好：</span>
                    <span v-for="tag in (role.preferredSkillTypes ?? [])" :key="tag" class="skill-tag">{{ tag }}</span>
                  </div>
                </div>
              </div>
            </div>
          </section>

          <!-- Section C: 种族模板 -->
          <section class="advanced-section">
            <div class="section-title">
              <div>
                <h3>种族模板</h3>
                <p class="status-text">species 叠加在 role 上。权重为增量，基础属性为固有平加值，倍数在最终属性上相乘。</p>
              </div>
              <button class="secondary small" @click="addTemplateEntry('species')">＋ 新增种族</button>
            </div>
            <div class="template-list">
              <div v-for="sp in scoring.enemyAttributeTemplates.species" :key="sp.key" class="template-item">
                <button class="template-header" @click="toggleExpandSpecies(sp.key)">
                  <strong>{{ sp.label }}</strong>
                  <small>{{ sp.key }}</small>
                  <div class="weight-bar-mini">
                    <div v-for="bar in templateWeightBar(sp.weights)" :key="bar.key" class="weight-bar-seg species-bar" :style="{ width: Math.abs(bar.width) + '%' }" />
                  </div>
                  <span class="toggle-icon">{{ expandedSpecies.has(sp.key) ? '▲' : '▼' }}</span>
                </button>
                <div v-if="expandedSpecies.has(sp.key)" class="template-body">
                  <div class="template-meta-row">
                    <label class="template-label-field"><span>名称</span><input v-model="sp.label" class="inline-label-input" /></label>
                    <button class="icon-btn danger small" :title="`删除种族 ${sp.key}`" @click="removeTemplateEntry('species', sp.key)">× 删除</button>
                  </div>

                  <!-- 权重增量 -->
                  <div class="weight-section">
                    <div class="weight-section-title">权重增量（叠加到 role 分配）</div>
                    <div class="weight-kv">
                      <div v-for="(val, wkey) in sp.weights" :key="wkey" class="weight-row">
                        <span class="weight-attr-label" :title="String(wkey)">{{ attrLabel(String(wkey)) }}</span>
                        <NumberInput :model-value="val" @update:model-value="(v: number) => (sp.weights[wkey] = v)" />
                        <button class="icon-btn danger" @click="removeAttrFromMap(sp.weights, String(wkey))">×</button>
                      </div>
                      <div class="weight-add-row">
                        <select class="attr-select" @change="(e) => { addAttrToMap(sp.weights, (e.target as HTMLSelectElement).value); (e.target as HTMLSelectElement).value = ''; }">
                          <option value="">＋ 添加权重属性…</option>
                          <option v-for="attr in scoring.attributes" :key="attr.key" :value="attr.key" :disabled="attr.key in sp.weights">
                            {{ attr.label }}（{{ attr.key }}）
                          </option>
                        </select>
                      </div>
                    </div>
                  </div>

                  <!-- 基础属性 -->
                  <div class="weight-section">
                    <div class="weight-section-title">基础属性（固有平加，独立于权重分配）</div>
                    <div class="weight-kv">
                      <div v-for="(val, wkey) in (sp.baseAttrs ?? {})" :key="wkey" class="weight-row">
                        <span class="weight-attr-label" :title="String(wkey)">{{ attrLabel(String(wkey)) }}</span>
                        <NumberInput :model-value="val" @update:model-value="(v: number) => { if (!sp.baseAttrs) sp.baseAttrs = {}; sp.baseAttrs[wkey] = v; }" />
                        <button class="icon-btn danger" @click="removeAttrFromMap(sp.baseAttrs ??= {}, String(wkey))">×</button>
                      </div>
                      <div class="weight-add-row">
                        <select class="attr-select" @change="(e) => { if (!sp.baseAttrs) sp.baseAttrs = {}; addAttrToMap(sp.baseAttrs, (e.target as HTMLSelectElement).value); (e.target as HTMLSelectElement).value = ''; }">
                          <option value="">＋ 添加基础属性…</option>
                          <option v-for="attr in scoring.attributes" :key="attr.key" :value="attr.key" :disabled="attr.key in (sp.baseAttrs ?? {})">
                            {{ attr.label }}（{{ attr.key }}）
                          </option>
                        </select>
                      </div>
                    </div>
                  </div>

                  <!-- 倍数 -->
                  <div class="weight-section">
                    <div class="weight-section-title">属性倍数（最终属性 × 此值，1 = 无效果）</div>
                    <div class="weight-kv">
                      <div v-for="(val, wkey) in (sp.multipliers ?? {})" :key="wkey" class="weight-row">
                        <span class="weight-attr-label" :title="String(wkey)">{{ attrLabel(String(wkey)) }}</span>
                        <NumberInput :model-value="val" @update:model-value="(v: number) => { if (!sp.multipliers) sp.multipliers = {}; sp.multipliers[wkey] = v; }" />
                        <button class="icon-btn danger" @click="removeAttrFromMap(sp.multipliers ??= {}, String(wkey))">×</button>
                      </div>
                      <div class="weight-add-row">
                        <select class="attr-select" @change="(e) => { if (!sp.multipliers) sp.multipliers = {}; addAttrToMap(sp.multipliers, (e.target as HTMLSelectElement).value, 1); (e.target as HTMLSelectElement).value = ''; }">
                          <option value="">＋ 添加倍数属性…</option>
                          <option v-for="attr in scoring.attributes" :key="attr.key" :value="attr.key" :disabled="attr.key in (sp.multipliers ?? {})">
                            {{ attr.label }}（{{ attr.key }}）
                          </option>
                        </select>
                      </div>
                    </div>
                  </div>

                  <div class="skill-types-row">
                    <span class="status-text">技能偏好：</span>
                    <span v-for="tag in (sp.preferredSkillTypes ?? [])" :key="tag" class="skill-tag">{{ tag }}</span>
                  </div>
                </div>
              </div>
            </div>
          </section>

          <!-- Section: 派生属性最低值 -->
          <section class="advanced-section">
            <div class="section-title">
              <div>
                <h3>派生属性最低值</h3>
                <p class="status-text">保证生成的敌人至少有这些基础数值（即使权重分配计算后低于此值也会被提升到最低值）。</p>
              </div>
            </div>
            <div class="minimums-grid">
              <div v-for="(min, key) in (scoring.enemyAttributeTemplates.derivedStatMinimums ?? {})" :key="key" class="minimum-item">
                <label>
                  <span class="minimum-label" :title="String(key)">{{ attrLabel(String(key)) }}</span>
                  <NumberInput :model-value="min" @update:model-value="(v: number) => { if (scoring.enemyAttributeTemplates.derivedStatMinimums) scoring.enemyAttributeTemplates.derivedStatMinimums[String(key)] = Math.max(0, v); }" />
                </label>
              </div>
              <div class="minimum-add">
                <select class="attr-select" @change="(e) => { const key = (e.target as HTMLSelectElement).value; if (key && scoring.enemyAttributeTemplates.derivedStatMinimums) { scoring.enemyAttributeTemplates.derivedStatMinimums[key] = 1; } (e.target as HTMLSelectElement).value = ''; }">
                  <option value="">＋ 添加最低值属性…</option>
                  <option v-for="attr in scoring.attributes" :key="attr.key" :value="attr.key" :disabled="attr.key in (scoring.enemyAttributeTemplates.derivedStatMinimums ?? {})">
                    {{ attr.label }}（{{ attr.key }}）
                  </option>
                </select>
              </div>
            </div>
          </section>

          <!-- Section D: 敌人数值预览 -->
          <section class="advanced-section">
            <div class="section-title">
              <div>
                <h3>敌人数值预览</h3>
                <p class="status-text">选择等级 + tier + role + species，查看模型推导出的建议数值。</p>
              </div>
            </div>
            <div class="preview-controls">
              <label>等级 <NumberInput :model-value="previewLevel" @update:model-value="(v: number) => (previewLevel = Math.max(1, Math.min(balanceModel.maxBattleLevel, Math.round(v))))" /></label>
              <label>等级类型
                <select :value="previewTier" @change="(e) => (previewTier = (e.target as HTMLSelectElement).value)">
                  <option v-for="t in scoring.enemyAttributeTemplates.tiers" :key="t.key" :value="t.key">{{ t.label }}</option>
                </select>
              </label>
              <label>职业
                <select :value="previewRole" @change="(e) => (previewRole = (e.target as HTMLSelectElement).value)">
                  <option v-for="r in scoring.enemyAttributeTemplates.roles" :key="r.key" :value="r.key">{{ r.label }}</option>
                </select>
              </label>
              <label>种族
                <select :value="previewSpecies" @change="(e) => (previewSpecies = (e.target as HTMLSelectElement).value)">
                  <option value="">无</option>
                  <option v-for="s in scoring.enemyAttributeTemplates.species" :key="s.key" :value="s.key">{{ s.label }}</option>
                </select>
              </label>
            </div>
            <div v-if="previewTierData" class="preview-result">
              <div class="preview-header">Lv.{{ previewLevel }} × {{ previewTierData.label }}<span v-if="previewRoleData"> × {{ previewRoleData.label }}</span><span v-if="previewSpeciesData"> × {{ previewSpeciesData.label }}</span></div>
              <div class="preview-metrics">
                <div class="preview-metric">
                  <span>目标 HP</span>
                  <strong>{{ formatScoreNumber(previewBaseline.playerDpr * previewTierData.targetRounds.min * (previewTierData.budgetMultiplier ?? 1) * (previewTierData.hpMultiplier ?? 1)) }} ~ {{ formatScoreNumber(previewBaseline.playerDpr * previewTierData.targetRounds.max * (previewTierData.budgetMultiplier ?? 1) * (previewTierData.hpMultiplier ?? 1)) }}</strong>
                  <small>玩家 DPR {{ formatScoreNumber(previewBaseline.playerDpr) }} × {{ previewTierData.targetRounds.min }}~{{ previewTierData.targetRounds.max }} 回合 × {{ previewTierData.budgetMultiplier ?? 1 }} 预算 × {{ previewTierData.hpMultiplier ?? 1 }} HP倍率</small>
                </div>
                <div class="preview-metric">
                  <span>目标 DPS</span>
                  <strong>{{ formatScoreNumber(previewBaseline.playerHp * previewTierData.pressurePct * (previewTierData.budgetMultiplier ?? 1)) }}</strong>
                  <small>玩家 HP {{ formatScoreNumber(previewBaseline.playerHp) }} × {{ formatPercent(previewTierData.pressurePct) }}/回合 × {{ previewTierData.budgetMultiplier ?? 1 }} 预算</small>
                </div>
                <div class="preview-metric">
                  <span>战斗中位</span>
                  <strong>{{ formatScoreNumber((previewTierData.targetRounds.min + previewTierData.targetRounds.max) / 2) }} 回合</strong>
                  <small>防御饱和K = {{ balanceModel.defenseK }}（{{ balanceModel.defenseK }}防御=50%减免）</small>
                </div>
              </div>
              <div v-if="Object.keys(previewStats).length > 0" class="preview-stats">
                <div class="preview-stats-title">模型建议数值：</div>
                <div class="preview-stats-grid">
                  <span v-for="(val, stat) in previewStats" :key="stat" class="preview-stat-item">
                    <small>{{ optionLabel(String(stat)) }}</small>
                    <strong>{{ val }}</strong>
                  </span>
                </div>
              </div>
              <div v-if="previewRoleData" class="radar-section">
                <svg width="140" height="140" viewBox="0 0 120 120" class="radar-svg">
                  <polygon v-if="previewRoleData" :points="radarRolePoints(previewRoleData.weights)" fill="rgba(99,102,241,0.2)" stroke="rgba(99,102,241,0.7)" stroke-width="1.5" />
                  <polygon v-if="previewSpeciesData" :points="radarMergedPoints(previewRoleData.weights, previewSpeciesData.weights)" fill="rgba(234,88,12,0.1)" stroke="rgba(234,88,12,0.5)" stroke-width="1.5" stroke-dasharray="4,2" />
                </svg>
                <div class="radar-legend">
                  <span class="legend-item role">■ 职业权重</span>
                  <span v-if="previewSpeciesData" class="legend-item species">▪ +种族叠加</span>
                </div>
              </div>
            </div>
          </section>

          <section class="advanced-section">
            <div class="section-title">
              <div>
                <h3>诊断列表</h3>
                <p class="status-text">按危险度聚合技能、敌人、装备、词条的超模/低模/经验/机制风险。</p>
              </div>
            </div>
            <div class="diagnostic-list">
              <button v-for="issue in balanceIssues" :key="`${issue.table}:${issue.id}`" class="diagnostic-row" @click="jumpToBalanceIssue(issue)">
                <span>{{ TABLE_LABELS[issue.table] }} / {{ issue.id }}</span>
                <strong>{{ Math.round(issue.score) }}分</strong>
                <small v-for="hint in issue.hints.slice(0, 2)" :key="hint.title" :class="balanceSeverityClass(hint.severity)">{{ hint.title }}：{{ hint.detail }}</small>
              </button>
            </div>
          </section>
        </section>

        <section v-else-if="activeView === 'specialization_trees'" class="specialization-workspace">
          <SpecializationTreeDesigner
            :model-value="catalog.specialization_trees"
            :attributes="catalog.attributes"
            :items="catalog.items"
            :skills="catalog.skills"
            :buffs="catalog.buff"
            :equipment-templates="catalog.equipment_templates"
            @update:model-value="updateSpecializationTrees"
            @status="(message: string) => (status = message)"
          />
        </section>

        <div v-else-if="!isSpecialView(activeView)" class="workspace">
          <section class="list-pane">
            <div class="pane-head">
              <div>
                <h2>{{ TABLE_LABELS[activeTable] }}</h2>
                <p>{{ tableRows.length }} 行</p>
              </div>
              <button @click="addRow">新增</button>
            </div>
            <label class="checkbox-field sort-toggle"><input v-model="sortByScore" type="checkbox" />按评分排序</label>
            <input v-model="filterInputText" class="search-input" placeholder="过滤 id / 名称 / nameKey / JSON 内容" />
            <div class="row-list">
              <div v-for="group in groupedRows" :key="group.key" class="row-group">
                <button class="group-head" @click="toggleGroupCollapse(group.key)">
                  <span>{{ group.label }}</span>
                  <small>{{ group.totalCount }}项 / {{ Math.round(group.totalScore) }}分</small>
                </button>
                <template v-if="!isGroupCollapsed(group.key)">
                  <template v-if="group.subGroups?.length">
                    <section v-for="subGroup in group.subGroups" :key="subGroup.key" class="row-subgroup">
                      <div class="subgroup-head">
                        <span>{{ subGroup.label }}</span>
                        <small>{{ subGroup.totalCount }}项 / {{ Math.round(subGroup.totalScore) }}分</small>
                      </div>
                      <div
                        v-for="{ row, index } in subGroup.rows"
                        :key="`${rowId(row, String(index))}-${index}`"
                        class="row-item"
                        :class="{ active: selectedTable === activeTable && selectedIndex === index }"
                      >
                        <button class="row-main" @click="selectRow(index)">
                          <strong>{{ displayName(row) }}</strong>
                          <span>{{ rowMetaLine(activeTable, row) }}</span>
                          <span>{{ rowId(row, '(no id)') }}</span>
                          <span v-if="rowDisabled(row)" class="disabled-pill">disabled</span>
                          <span v-if="rowExtraScoreLine(activeTable, row)" class="row-extra-score">{{ rowExtraScoreLine(activeTable, row) }}</span>
                        </button>
                        <div class="row-side">
                          <button class="ghost compare-button" @click="(event) => toggleComparePanel(index, event as MouseEvent)">对比</button>
                          <em v-if="activeTable === 'items'">{{ itemRowSideLine(row) }}</em>
                          <em v-else>{{ scoreFor(activeTable, row).score }}分<br />{{ scoreFor(activeTable, row).goldValue }}金</em>
                        </div>
                      </div>
                    </section>
                  </template>
                  <template v-else>
                    <div
                      v-for="{ row, index } in group.rows"
                      :key="`${rowId(row, String(index))}-${index}`"
                      class="row-item"
                      :class="{ active: selectedTable === activeTable && selectedIndex === index }"
                    >
                      <button class="row-main" @click="selectRow(index)">
                        <strong>{{ displayName(row) }}</strong>
                        <span>{{ rowMetaLine(activeTable, row) }}</span>
                        <span>{{ rowId(row, '(no id)') }}</span>
                        <span v-if="rowDisabled(row)" class="disabled-pill">disabled</span>
                        <span v-if="rowExtraScoreLine(activeTable, row)" class="row-extra-score">{{ rowExtraScoreLine(activeTable, row) }}</span>
                      </button>
                      <div class="row-side">
                        <button class="ghost compare-button" @click="(event) => toggleComparePanel(index, event as MouseEvent)">对比</button>
                        <em v-if="activeTable === 'items'">{{ itemRowSideLine(row) }}</em>
                        <em v-else>{{ scoreFor(activeTable, row).score }}分<br />{{ scoreFor(activeTable, row).goldValue }}金</em>
                      </div>
                    </div>
                  </template>
                </template>
              </div>
            </div>
          </section>

          <section class="editor-pane" v-if="selectedRow">
            <div class="pane-head sticky">
              <div>
                <h2>{{ rowId(selectedRow, '未命名') }}</h2>
                <p>{{ TABLE_LABELS[editorTable] }} / #{{ selectedIndex + 1 }}</p>
              </div>
              <div class="toolbar">
                <button class="secondary" @click="toggleSelectedRowDisabled">{{ rowDisabled(selectedRow) ? '启用' : '禁用' }}</button>
                <button class="secondary" @click="duplicateRow">复制</button>
                <button class="danger" @click="removeRow">删除</button>
              </div>
            </div>

            <div class="score-panel">
              <div v-if="editorTable !== 'items' && editorTable !== 'actions'"><span>评分</span><strong>{{ selectedScore?.score ?? 0 }}</strong></div>
              <div v-if="editorTable !== 'items' && editorTable !== 'actions'"><span>金币价值</span><strong>{{ selectedScore?.goldValue ?? 0 }}</strong></div>
              <template v-if="editorTable === 'items'">
                <div class="item-economy-card">
                  <span>主产出动作（最短耗时）</span>
                  <strong>{{ selectedItemMainOutputAction ? rowId(selectedItemMainOutputAction) : '-' }}</strong>
                  <small>{{ t('common.itemPredictedValue') }}：{{ selectedItemOutputEconomy ? `${formatGoldDisplay(selectedItemOutputEconomy.mainOutputUnitGold)} 金` : '-' }}</small>
                  <button v-if="selectedItemMainOutputAction" type="button" class="secondary" @click="jumpToSelectedItemMainOutputAction">{{ t('common.jumpToMainOutputAction') }}</button>
                </div>
                <div class="item-economy-card">
                  <span>怪物掉落参考价</span>
                  <strong>{{ selectedItemDropPricing ? `${formatGoldDisplay(selectedItemDropPricing.suggestedValue)} 金` : '-' }}</strong>
                  <small v-if="selectedItemDropPricing">
                    当前 {{ formatGoldDisplay(selectedItemDropPricing.currentValue) }} 金 / 差异 {{ dropPricingDeltaText(selectedItemDropPricing.deltaRatio) }}
                  </small>
                  <small v-if="selectedItemDropPricing">
                    定价稀有度 {{ selectedItemDropPricing.pricingRarity }}
                    <template v-if="selectedItemDropPricing.catalogRarity !== selectedItemDropPricing.pricingRarity"> / 原始 {{ selectedItemDropPricing.catalogRarity }}</template>
                    <template v-if="selectedItemDropPricing.skillbookAnchorEnemyType"> / 技能书锚定 {{ selectedItemDropPricing.skillbookAnchorEnemyType }}</template>
                  </small>
                  <small v-if="selectedItemDropPricing">
                    锚定来源 {{ selectedItemDropPricing.mainWorldId }} / {{ selectedItemDropPricing.mainEnemyId }} / 等级 {{ selectedItemDropPricing.mainEnemyLevel }} / 类型 x{{ formatScoreNumber(selectedItemDropPricing.mainEnemyTypeWeight) }} / 等级 x{{ formatScoreNumber(selectedItemDropPricing.mainEnemyLevelPriceMultiplier) }}
                  </small>
                  <small v-else>无非世界首领掉落来源</small>
                  <div v-if="selectedItemDropPricing" class="risk-tags">
                    <em v-for="flag in selectedItemDropPricing.riskFlags" :key="flag" :class="dropPricingRiskClass(flag)">{{ flag }}</em>
                    <small v-if="selectedItemDropPricing.riskFlags.length === 0">正常</small>
                  </div>
                  <button v-if="selectedItemDropPricing?.mainEnemyId" type="button" class="secondary" @click="jumpToSelectedItemDropEnemy">跳转主要来源敌人</button>
                </div>
              </template>
              <div v-if="editorTable === 'actions' && selectedActionEconomy"><span>主产出单价</span><strong>{{ formatGoldDisplay(selectedActionEconomy.mainOutputUnitGold) }} 金</strong></div>
              <div v-if="editorTable === 'actions' && selectedActionEconomy"><span>工时价值</span><strong>{{ formatGoldDisplay(selectedActionEconomy.baseLaborGold) }} 金</strong></div>
              <div v-if="editorTable === 'actions' && selectedActionEconomy"><span>消耗总价值</span><strong>{{ formatGoldDisplay(selectedActionEconomy.totalCostGold) }} 金</strong></div>
              <div v-if="editorTable === 'actions' && selectedActionEconomy"><span>主产出期望量</span><strong>{{ formatGoldDisplay(selectedActionEconomy.mainOutputExpectedCount) }}</strong></div>
              <div v-if="editorTable === 'equipment_templates'"><span>强化属性分</span><strong>{{ Math.round(selectedScore?.detail.enhanceMin ?? 0) }}~{{ Math.round(selectedScore?.detail.enhanceMax ?? 0) }}</strong></div>
              <div v-if="editorTable === 'affix_definitions'"><span>词条随机分</span><strong>{{ Math.round(selectedScore?.detail.affixMin ?? 0) }}~{{ Math.round(selectedScore?.detail.affixMax ?? 0) }}</strong></div>
              <div v-if="selectedScore?.sections?.length" class="score-breakdown">
                <strong>分项</strong>
                <span v-for="section in selectedScore.sections" :key="section.id">{{ section.label }} {{ section.score }}分</span>
              </div>
              <!-- <pre>{{ JSON.stringify(selectedScore?.detail ?? {}, null, 2) }}</pre> -->
            </div>

            <OverviewPanel
              v-if="selectedRowOverview"
              :table="editorTable"
              :overview="selectedRowOverview"
            />

            <section v-if="selectedBalanceDiagnostics" class="balance-hint-panel">
              <div class="section-title compact">
                <div>
                  <h3>数值模型提示</h3>
                  <p class="status-text">{{ selectedBaselineText() }}</p>
                </div>
              </div>
              <div v-if="selectedBalanceDiagnostics.enemy" class="balance-metrics">
                <span>建议经验 <strong>{{ selectedBalanceDiagnostics.enemy.recommendedExp }}</strong></span>
                <span>经验偏差 <strong>{{ formatPercent(selectedBalanceDiagnostics.enemy.expDeltaPct) }}</strong></span>
                <span>建议生命 <strong>{{ formatScoreNumber(selectedBalanceDiagnostics.enemy.recommendedHpMin) }}~{{ formatScoreNumber(selectedBalanceDiagnostics.enemy.recommendedHpMax) }}</strong></span>
                <span>预计回合 <strong>{{ formatScoreNumber(selectedBalanceDiagnostics.enemy.estimatedRounds) }}</strong></span>
                <span>单回合压力 <strong>{{ formatPercent(selectedBalanceDiagnostics.enemy.pressurePct) }}</strong></span>
                <span>防御饱和K <strong>{{ formatScoreNumber(selectedBalanceDiagnostics.enemy.defenseK) }}</strong></span>
              </div>
              <div class="diagnostic-grid">
                <article v-for="hint in selectedBalanceDiagnostics.hints" :key="hint.title" class="diagnostic-card" :class="balanceSeverityClass(hint.severity)">
                  <strong>{{ hint.title }}</strong>
                  <span>{{ hint.detail }}</span>
                </article>
              </div>
            </section>

            <section v-if="editorTable === 'enemies'" class="enemy-template-panel">
              <div class="section-title">
                <div>
                  <h3>属性分配模板</h3>
                  <p class="status-text">输入基础属性分，按地位倍率、职业定位和种族权重生成属性矩阵。</p>
                </div>
                <div class="section-actions">
                  <button class="secondary" @click="applyEnemyDifficultyGuardrailsToCurrent">约束当前防御</button>
                  <button @click="applyEnemyAttributeTemplate">应用到当前敌人</button>
                </div>
              </div>
              <div class="enemy-template-grid">
                <label>等级<NumberInput :model-value="enemyTemplateLevel" @update:model-value="(value: number) => { enemyTemplateLevel = value; enemyTemplateBudgetOverride = null; }" /></label>
                <label>基础属性分 (自动: {{ enemyTemplateBudget }})<div class="budget-input-row"><NumberInput :model-value="enemyTemplateBudgetOverride ?? enemyTemplateBudget" @update:model-value="(value: number) => (enemyTemplateBudgetOverride = value)" /><button class="reset-btn" @click="enemyTemplateBudgetOverride = null" title="重置为等级自动值">↻</button></div></label>
                <label>随机偏差<NumberInput :model-value="enemyTemplateVariance" @update:model-value="(value: number) => (enemyTemplateVariance = Math.max(0, Math.min(0.85, value)))" /></label>
                <label>地位<select v-model="enemyTemplateTier"><option v-for="option in enemyTemplateTierOptions" :key="option" :value="option">{{ templateOptionLabel('tiers', option) }}</option></select></label>
                <label>职业定位<select v-model="enemyTemplateRole"><option v-for="option in enemyTemplateRoleOptions" :key="option" :value="option">{{ templateOptionLabel('roles', option) }}</option></select></label>
                <label>种族<select v-model="enemyTemplateSpecies"><option v-for="option in enemyTemplateSpeciesOptions" :key="option" :value="option">{{ templateOptionLabel('species', option) }}</option></select></label>
                <div class="template-preview-expanded">
                  <div class="template-preview-head">
                    <span>实际属性分</span>
                    <strong>{{ Math.round(selectedEnemyTemplateBudget()) }}</strong>
                    <small>{{ formatEnemyTemplatePreview() || '没有有效权重' }}</small>
                  </div>
                  <div class="template-preview-primary">
                    <div class="stat-item primary"><span>力量</span><strong>{{ enemyTemplateDerivedStats.strength }}</strong></div>
                    <div class="stat-item primary"><span>敏捷</span><strong>{{ enemyTemplateDerivedStats.dexterity }}</strong></div>
                    <div class="stat-item primary"><span>智力</span><strong>{{ enemyTemplateDerivedStats.intelligence }}</strong></div>
                    <div class="stat-item primary"><span>感知</span><strong>{{ enemyTemplateDerivedStats.perception }}</strong></div>
                    <div class="stat-item primary"><span>耐力</span><strong>{{ enemyTemplateDerivedStats.stamina }}</strong></div>
                  </div>
                  <div class="template-preview-stats">
                    <div class="stat-item"><span>生命值</span><strong>{{ enemyTemplateDerivedStats.hp }}</strong></div>
                    <div class="stat-item"><span>魔法值</span><strong>{{ enemyTemplateDerivedStats.mp }}</strong></div>
                    <div class="stat-item"><span>速度</span><strong>{{ enemyTemplateDerivedStats.speed }}</strong></div>
                    <div class="stat-item"><span>物理攻击</span><strong>{{ enemyTemplateDerivedStats.physics_damage }}</strong></div>
                    <div class="stat-item"><span>魔法攻击</span><strong>{{ enemyTemplateDerivedStats.magical_damage }}</strong></div>
                    <div class="stat-item"><span>物理防御</span><strong>{{ enemyTemplateDerivedStats.physics_defence }}</strong></div>
                    <div class="stat-item"><span>魔法防御</span><strong>{{ enemyTemplateDerivedStats.magical_defence }}</strong></div>
                    <div class="stat-item"><span>暴击抵抗</span><strong>{{ (enemyTemplateDerivedStats.crit_damage_resist * 100).toFixed(1) }}%</strong></div>
                  </div>
                  <!-- 实际数据对比 -->
                  <div v-if="actualEnemyStats" class="template-preview-comparison">
                    <div class="comparison-title">当前 vs 应用后</div>
                    <div class="comparison-grid">
                      <div class="comparison-item">
                        <span>生命值</span>
                        <div class="comparison-values">
                          <div class="value-group">
                            <small>当前</small>
                            <strong class="actual">{{ actualEnemyStats.hp }}</strong>
                          </div>
                          <div class="value-group">
                            <small>应用后</small>
                            <strong class="calculated">{{ enemyTemplateDerivedStats.hp }}</strong>
                          </div>
                          <span class="diff" :class="getDiffClass(actualEnemyStats.hp, enemyTemplateDerivedStats.hp)">{{ formatDiff(actualEnemyStats.hp, enemyTemplateDerivedStats.hp) }}</span>
                        </div>
                      </div>
                      <div class="comparison-item">
                        <span>魔法值</span>
                        <div class="comparison-values">
                          <div class="value-group">
                            <small>当前</small>
                            <strong class="actual">{{ actualEnemyStats.mp }}</strong>
                          </div>
                          <div class="value-group">
                            <small>应用后</small>
                            <strong class="calculated">{{ enemyTemplateDerivedStats.mp }}</strong>
                          </div>
                          <span class="diff" :class="getDiffClass(actualEnemyStats.mp, enemyTemplateDerivedStats.mp)">{{ formatDiff(actualEnemyStats.mp, enemyTemplateDerivedStats.mp) }}</span>
                        </div>
                      </div>
                      <div class="comparison-item">
                        <span>物理攻击</span>
                        <div class="comparison-values">
                          <div class="value-group">
                            <small>当前</small>
                            <strong class="actual">{{ actualEnemyStats.physics_damage }}</strong>
                          </div>
                          <div class="value-group">
                            <small>应用后</small>
                            <strong class="calculated">{{ enemyTemplateDerivedStats.physics_damage }}</strong>
                          </div>
                          <span class="diff" :class="getDiffClass(actualEnemyStats.physics_damage, enemyTemplateDerivedStats.physics_damage)">{{ formatDiff(actualEnemyStats.physics_damage, enemyTemplateDerivedStats.physics_damage) }}</span>
                        </div>
                      </div>
                      <div class="comparison-item">
                        <span>魔法攻击</span>
                        <div class="comparison-values">
                          <div class="value-group">
                            <small>当前</small>
                            <strong class="actual">{{ actualEnemyStats.magical_damage }}</strong>
                          </div>
                          <div class="value-group">
                            <small>应用后</small>
                            <strong class="calculated">{{ enemyTemplateDerivedStats.magical_damage }}</strong>
                          </div>
                          <span class="diff" :class="getDiffClass(actualEnemyStats.magical_damage, enemyTemplateDerivedStats.magical_damage)">{{ formatDiff(actualEnemyStats.magical_damage, enemyTemplateDerivedStats.magical_damage) }}</span>
                        </div>
                      </div>
                    </div>
                  </div>
                </div>
              </div>
            </section>

            <div class="form-grid editor-grid">
              <label v-for="field in activeFields" :key="field.key" :class="{ wide: (field.span ?? 1) > 1 || field.kind === 'json' || field.kind === 'enemyEquipment' || field.kind === 'enemySkills' || field.kind === 'enemyDrops' || field.kind === 'difficultySettings' || field.kind === 'achievementCondition' || field.kind === 'achievementRewards', full: (field.span ?? 1) >= 4 || field.kind === 'json' || field.kind === 'enemyEquipment' || field.kind === 'enemySkills' || field.kind === 'enemyDrops' || field.kind === 'difficultySettings' || field.kind === 'achievementCondition' || field.kind === 'achievementRewards' || field.kind === 'effects' || field.kind === 'targeting' }">
                <span class="field-heading">
                  <span>{{ field.label }}</span>
                  <em v-if="fieldContribution(field)">{{ fieldContribution(field) }}</em>
                  <em v-if="fieldMetaText(field)" class="field-meta">{{ fieldMetaText(field) }}</em>
                </span>
                <FieldRenderer :field="field" />
              </label>
            </div>

            <section v-if="editorTable === 'buff' && currentBuffEffects.length > 0" class="buff-effects-preview">
              <div class="section-title">
                <h3>Buff 效果列表 ({{ currentBuffEffects.length }})</h3>
              </div>
              <div class="buff-effects-list">
                <div v-for="({ row, index }, i) in currentBuffEffects" :key="i" class="buff-effect-card">
                  <div class="buff-effect-head">
                    <strong>效果 #{{ i + 1 }}</strong>
                    <span class="buff-effect-id">{{ rowId(row, '(no id)') }}</span>
                    <span class="buff-effect-trigger">{{ OPTION_LABEL_OVERRIDES[asString(row.trigger)] || asString(row.trigger) }}</span>
                    <button class="secondary" @click="jumpToRow('buff_effects', rowId(row))">编辑</button>
                  </div>
                  <div class="buff-effect-body">
                    <div v-for="(effect, ei) in asArray(row.effects).filter(isRecord)" :key="ei" class="buff-effect-item">
                      <span class="buff-effect-kind">{{ OPTION_LABEL_OVERRIDES[asString(effect.kind || effect.type)] || asString(effect.kind || effect.type) }}</span>
                      <span v-if="effect.attr || effect.stat">{{ effect.attr || effect.stat }} {{ effect.value ?? effect.percent }}</span>
                      <span v-else-if="effect.kind === 'add_buff' || effect.type === 'add_buff'">{{ effect.buffId }} {{ effect.durationTurns }}回合</span>
                      <span v-else-if="effect.kind === 'remove_buff' || effect.type === 'remove_buff'">{{ effect.buffId }}</span>
                      <span v-else-if="effect.value !== undefined">{{ effect.value }}</span>
                    </div>
                    <div v-if="asArray(row.conditions).filter(isRecord).length > 0" class="buff-effect-conditions">
                      <small>条件: {{ asArray(row.conditions).filter(isRecord).length }}个</small>
                    </div>
                  </div>
                </div>
              </div>
            </section>

            <section class="advanced-section">
              <div class="section-title">
                <h3>高级字段</h3>
                <button class="secondary" @click="addAdvancedField">新增字段</button>
              </div>
              <div v-if="advancedEntries.length === 0" class="empty-text">没有额外字段。</div>
              <div v-for="entry in advancedEntries" :key="entry.key" class="advanced-row">
                <strong>{{ entry.key }}</strong>
                <SearchSelect
                  v-if="editorTable === 'actions' && entry.key === 'sub_type'"
                  :model-value="String(entry.value ?? '')"
                  :options="actionSubTypeOptions"
                  @update:model-value="(value) => updateAdvancedKey(entry.key, value)"
                />
                <JsonFieldEditor v-else :model-value="entry.value" :path="[entry.key]" @update:model-value="(value: unknown) => updateAdvancedKey(entry.key, value)" />
                <button class="danger" @click="removeAdvancedField(entry.key)">删除</button>
              </div>
            </section>
          </section>

          <div class="compare-panel-layer">
            <article
              v-for="panel in comparePanels"
              :key="panel.key"
              class="compare-panel"
              :style="{ top: `${panel.top}px`, left: `${panel.left}px`, zIndex: String(panel.z) }"
              @mousedown="bringComparePanelToFront(panel.key)"
            >
              <header class="compare-panel-head" @mousedown.prevent="(event) => startComparePanelDrag(panel.key, event as MouseEvent)">
                <strong>{{ TABLE_LABELS[panel.table] }} / {{ rowId(comparePanelRow(panel.table, panel.index) ?? {}, '(missing)') }}</strong>
                <button class="ghost" @click="closeComparePanel(panel.key)">关闭</button>
              </header>
              <p>{{ displayName(comparePanelRow(panel.table, panel.index) ?? {}) }}</p>
              <small v-if="comparePanelRow(panel.table, panel.index)">
                评分 {{ scoreFor(panel.table, comparePanelRow(panel.table, panel.index) ?? {}).score }} / 金币 {{ scoreFor(panel.table, comparePanelRow(panel.table, panel.index) ?? {}).goldValue }}
              </small>
              <div class="compare-panel-fields">
                <label v-for="entry in comparePanelRows(panel.table, panel.index)" :key="`${panel.key}-${entry.key}`">
                  <span>{{ entry.label }}</span>
                  <pre v-if="entry.multiline">{{ entry.value || '-' }}</pre>
                  <input v-else :value="entry.value || '-'" readonly />
                </label>
              </div>
            </article>
          </div>

          <section class="editor-pane empty-pane" v-if="!selectedRow">
            <h2>没有可编辑行</h2>
            <button @click="addRow">新增第一行</button>
          </section>
        </div>

        <section v-if="issues.length" class="issues-pane">
          <h3>校验问题</h3>
          <ul>
            <li v-for="issue in issues" :key="issue">{{ issue }}</li>
          </ul>
        </section>
      </section>
    </main>
  </div>
</template>
