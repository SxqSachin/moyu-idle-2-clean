<script setup lang="ts">
import { computed, nextTick, provide, ref, watch, type CSSProperties } from 'vue';
import {
  Handle,
  Position,
  VueFlow,
  useVueFlow,
  type Connection,
  type Edge,
  type Node,
  type NodeDragEvent,
  type NodeMouseEvent
} from '@vue-flow/core';
import { MiniMap, type MiniMapNodeFunc } from '@vue-flow/minimap';
import '@vue-flow/core/dist/style.css';
import '@vue-flow/core/dist/theme-default.css';
import '@vue-flow/minimap/dist/style.css';
import { asArray, asString, clone, isRecord, rowId, type DataRow } from '../model/catalog';
import {
  SPECIALIZATION_GRANT_KINDS,
  SPECIALIZATION_NODE_KINDS,
  SPECIALIZATION_REQUIREMENT_KINDS,
  SPECIALIZATION_SCHOOLS,
  SPECIALIZATION_SCHOOL_META,
  SPECIALIZATION_SLOT_FAMILIES,
  collectSpecializationEffectIds,
  createDefaultSpecializationGrant,
  createDefaultSpecializationNode,
  createDefaultSpecializationRequirement,
  createDefaultSpecializationTree,
  createNextSpecializationLevel,
  normalizeSpecializationTrees,
  validateSpecializationTrees,
  type SpecializationGrantKind,
  type SpecializationInspectorTab,
  type SpecializationNode,
  type SpecializationRequirementKind,
  type SpecializationSchool,
  type SpecializationTree
} from '../model/specialization';
import SafeJsonEditor from './SafeJsonEditor.vue';
import SpecializationBattleHooksEditor from './SpecializationBattleHooksEditor.vue';
import SpecializationEffectMutationsEditor from './SpecializationEffectMutationsEditor.vue';

const props = withDefaults(defineProps<{
  modelValue: DataRow[];
  attributes?: DataRow[];
  items?: DataRow[];
  skills?: DataRow[];
  buffs?: DataRow[];
  equipmentTemplates?: DataRow[];
}>(), { attributes: () => [], items: () => [], skills: () => [], buffs: () => [], equipmentTemplates: () => [] });

const emit = defineEmits<{
  'update:modelValue': [value: DataRow[]];
  status: [value: string];
}>();

const FLOW_ID = 'specialization-tree-designer';
const { fitView, setCenter, zoomIn, zoomOut } = useVueFlow(FLOW_ID);
const draftRows = ref<SpecializationTree[]>(normalizeSpecializationTrees(props.modelValue));
const selectedTreeIndex = ref(0);
const selectedNodeId = ref('');
const selectedLevelIndex = ref(0);
const inspectorTab = ref<SpecializationInspectorTab>('basic');
const newNodeSchool = ref<SpecializationSchool>('strength');
const grantKindToAdd = ref<SpecializationGrantKind>('stat_modifier');
const requirementKindToAdd = ref<SpecializationRequirementKind>('node_rank');
const undoStack = ref<SpecializationTree[][]>([]);
const redoStack = ref<SpecializationTree[][]>([]);
const searchQuery = ref('');
const searchCursor = ref(0);
const rulesEditMode = ref(false);
const persistedTreeIds = ref(new Set<string>());
const persistedNodeKeys = ref(new Set<string>());
let lastEmitted = '';

const selectedTree = computed(() => draftRows.value[selectedTreeIndex.value] ?? null);
const selectedNode = computed(() => selectedTree.value?.nodes.find((node) => node.id === selectedNodeId.value) ?? null);
const selectedLevel = computed(() => selectedNode.value?.levels[selectedLevelIndex.value] ?? null);
const nodeLocaleIds = ['zh-CN', 'en-US'] as const;
const attributeIds = computed(() => Array.from(new Set([
  ...SPECIALIZATION_SCHOOLS,
  ...props.attributes.map((row) => rowId(row)).filter(Boolean)
])).sort());
const itemIds = computed(() => props.items.map((row) => rowId(row)).filter(Boolean).sort());
const skillIds = computed(() => props.skills.map((row) => rowId(row)).filter(Boolean).sort());
const skillCategories = computed(() => Array.from(new Set(props.skills.map((row) => asString(row.category).trim()).filter(Boolean))).sort());
const weaponClasses = computed(() => Array.from(new Set(props.equipmentTemplates.flatMap((row) => asArray(row.weaponClass).map((entry) => asString(entry).trim())).filter(Boolean))).sort());
provide('specializationWeaponClasses', weaponClasses);
const buffIds = computed(() => props.buffs.map((row) => rowId(row)).filter(Boolean).sort());
const buffKinds = computed(() => Array.from(new Set(props.buffs.map((row) => asString(row.kind).trim()).filter(Boolean))).sort());
const selectableEffects = computed(() => [
  ...props.skills.flatMap((row) => asArray(row.effects)),
  ...props.equipmentTemplates.flatMap((row) => asArray(row.basicAttackEffects))
].filter(isRecord));
const skillEffectKinds = computed(() => Array.from(new Set(selectableEffects.value.map((effect) => asString(effect.kind).trim()).filter(Boolean))).sort());
const sourceEffectIds = computed(() => selectableEffects.value.flatMap((effect) => [asString(effect.id).trim(), asString(effect.effectId).trim()]).filter(Boolean));
const effectKindsById = computed(() => Object.fromEntries(selectableEffects.value.flatMap((effect) => {
  const kind = asString(effect.kind).trim();
  return [asString(effect.id).trim(), asString(effect.effectId).trim()].filter(Boolean).map((id) => [id, kind]);
})));
const equipmentEffectIds = computed(() => props.equipmentTemplates.flatMap((row) => asArray(row.basicAttackEffects)).filter(isRecord).flatMap((effect) => [asString(effect.id).trim(), asString(effect.effectId).trim()]).filter(Boolean));
const effectIds = computed(() => collectSpecializationEffectIds(draftRows.value, sourceEffectIds.value));
const issues = computed(() => validateSpecializationTrees(draftRows.value, {
  attributeIds: props.attributes.map((row) => rowId(row)).filter(Boolean),
  itemIds: itemIds.value,
  skillIds: skillIds.value,
  skillCategories: skillCategories.value,
  weaponClasses: weaponClasses.value,
  buffIds: buffIds.value,
  buffKinds: buffKinds.value,
  skillEffectKinds: skillEffectKinds.value,
  // The validator registers this tree's appendEffects itself so it can detect
  // collisions. The combined list is only for authoring selector options.
  effectIds: sourceEffectIds.value,
  effectKindsById: effectKindsById.value,
  equipmentEffectIds: equipmentEffectIds.value
}));
const selectedTreeIssues = computed(() => issues.value.filter((issue) => !issue.treeId || issue.treeId === selectedTree.value?.id));
const nodeIssueCounts = computed(() => {
  const counts = new Map<string, number>();
  for (const issue of selectedTreeIssues.value) if (issue.nodeId) counts.set(issue.nodeId, (counts.get(issue.nodeId) ?? 0) + 1);
  return counts;
});

function capturePersistedRows(rows: DataRow[]) {
  const trees = new Set(persistedTreeIds.value);
  const nodes = new Set(persistedNodeKeys.value);
  for (const rawTree of rows) {
    if (!isRecord(rawTree)) continue;
    const treeId = asString(rawTree.id).trim();
    if (!treeId) continue;
    trees.add(treeId);
    for (const rawNode of asArray(rawTree.nodes).filter(isRecord)) {
      const nodeId = asString(rawNode.id).trim();
      if (nodeId) nodes.add(`${treeId}\u0000${nodeId}`);
    }
  }
  persistedTreeIds.value = trees;
  persistedNodeKeys.value = nodes;
}

capturePersistedRows(props.modelValue);

function isPersistedTree(tree = selectedTree.value): boolean {
  return Boolean(tree && persistedTreeIds.value.has(tree.id));
}

function isPersistedNode(node = selectedNode.value, tree = selectedTree.value): boolean {
  return Boolean(node && tree && persistedNodeKeys.value.has(`${tree.id}\u0000${node.id}`));
}

function blockTreeRuleEdit(): boolean {
  if (!isPersistedTree() || rulesEditMode.value) return false;
  emit('status', '已发布专精树默认锁定；请先进入规则编辑，并按改动类型更新 balanceRevision 或 rulesetVersion');
  return true;
}

function blockNodeRuleEdit(): boolean {
  if ((!isPersistedTree() && !isPersistedNode()) || rulesEditMode.value) return false;
  emit('status', '已保存节点默认锁定；兼容新增或数值/DSL 调整递增 balanceRevision，破坏既有投入契约时才更新 rulesetVersion');
  return true;
}

function schoolColor(school: SpecializationSchool): string {
  const configured = selectedTree.value?.schools.find((entry) => asString(entry.id) === school);
  const ui = configured && isRecord(configured.ui) ? configured.ui : {};
  return asString(ui.color).trim() || SPECIALIZATION_SCHOOL_META[school].color;
}

function snapshot(): SpecializationTree[] {
  return clone(draftRows.value);
}

function emitChange(message?: string) {
  const payload = clone(draftRows.value);
  lastEmitted = JSON.stringify(payload);
  emit('update:modelValue', payload);
  if (message) emit('status', message);
}

function remember() {
  undoStack.value.push(snapshot());
  if (undoStack.value.length > 100) undoStack.value.shift();
  redoStack.value = [];
}

function mutate(change: () => void, message?: string) {
  remember();
  change();
  emitChange(message);
}

function reconcileSelection() {
  if (selectedTreeIndex.value >= draftRows.value.length) selectedTreeIndex.value = Math.max(0, draftRows.value.length - 1);
  const tree = selectedTree.value;
  if (!tree) {
    selectedNodeId.value = '';
    selectedLevelIndex.value = 0;
    return;
  }
  if (!tree.nodes.some((node) => node.id === selectedNodeId.value)) selectedNodeId.value = tree.nodes[0]?.id ?? '';
  const node = selectedNode.value;
  selectedLevelIndex.value = node ? Math.min(selectedLevelIndex.value, Math.max(0, node.levels.length - 1)) : 0;
}

watch(
  () => props.modelValue,
  (rows) => {
    const encoded = JSON.stringify(rows);
    if (encoded === lastEmitted) return;
    capturePersistedRows(rows);
    draftRows.value = normalizeSpecializationTrees(rows);
    undoStack.value = [];
    redoStack.value = [];
    reconcileSelection();
  },
  { deep: true }
);

watch(selectedTree, () => reconcileSelection(), { immediate: true });

function undo() {
  const previous = undoStack.value.pop();
  if (!previous) return;
  redoStack.value.push(snapshot());
  draftRows.value = previous;
  reconcileSelection();
  emitChange('已撤销专精树修改');
}

function redo() {
  const next = redoStack.value.pop();
  if (!next) return;
  undoStack.value.push(snapshot());
  draftRows.value = next;
  reconcileSelection();
  emitChange('已重做专精树修改');
}

function setTreeIndex(index: number) {
  selectedTreeIndex.value = Math.max(0, Math.min(index, draftRows.value.length - 1));
  selectedNodeId.value = selectedTree.value?.nodes[0]?.id ?? '';
  selectedLevelIndex.value = 0;
  void nextTick(() => fitView({ padding: 0.18, duration: 250 }));
}

function addTree() {
  mutate(() => {
    draftRows.value.push(createDefaultSpecializationTree(draftRows.value.length));
    setTreeIndex(draftRows.value.length - 1);
  }, '已新增独立专精树；新 treeId 没有存量状态，不需要迁移既有专精');
}

function duplicateTree() {
  if (!selectedTree.value) return;
  mutate(() => {
    const copy = clone(selectedTree.value!);
    copy.id = `${copy.id}_copy`;
    copy.nameKey = `specialization.tree.${copy.id}.name`;
    copy.descKey = `specialization.tree.${copy.id}.desc`;
    copy.rulesetVersion = `${copy.rulesetVersion}-copy`;
    draftRows.value.splice(selectedTreeIndex.value + 1, 0, copy);
    setTreeIndex(selectedTreeIndex.value + 1);
  }, '已复制专精树');
}

function deleteTree() {
  if (isPersistedTree() && !rulesEditMode.value) {
    emit('status', '已阻止删除当前专精树；请先进入规则编辑，项目只保留修改后的唯一规则集');
    return;
  }
  if (!selectedTree.value || !confirm(`删除专精树 ${selectedTree.value.id}？`)) return;
  mutate(() => {
    draftRows.value.splice(selectedTreeIndex.value, 1);
    reconcileSelection();
  }, '已删除专精树');
}

function updateTreeField(key: keyof SpecializationTree, value: unknown) {
  if (!selectedTree.value) return;
  if (key !== 'ui' && key !== 'balanceRevision' && blockTreeRuleEdit()) return;
  mutate(() => {
    if (key === 'id') {
      const oldId = selectedTree.value!.id;
      const newId = asString(value).trim();
      if (persistedTreeIds.value.has(oldId)) {
        persistedTreeIds.value = new Set([...persistedTreeIds.value, newId]);
        persistedNodeKeys.value = new Set([
          ...persistedNodeKeys.value,
          ...selectedTree.value!.nodes.map((node) => `${newId}\u0000${node.id}`)
        ]);
      }
      selectedTree.value!.id = newId;
      return;
    }
    (selectedTree.value as DataRow)[key] = value;
  });
}

function updateNestedField(container: 'unlock' | 'reset', key: string, value: unknown) {
  if (!selectedTree.value) return;
  if (blockTreeRuleEdit()) return;
  mutate(() => { (selectedTree.value![container] as DataRow)[key] = value; });
}

function addPointBand() {
  if (!selectedTree.value) return;
  if (blockTreeRuleEdit()) return;
  mutate(() => selectedTree.value!.pointBands.push({ id: `band_${selectedTree.value!.pointBands.length + 1}`, fromLevel: 1, toLevel: 1, everyLevels: 1, pointsPerGrant: 1 }));
}

function updatePointBand(index: number, key: string, value: unknown) {
  if (!selectedTree.value || !isRecord(selectedTree.value.pointBands[index])) return;
  if (blockTreeRuleEdit()) return;
  mutate(() => { selectedTree.value!.pointBands[index][key] = value; });
}

function removePointBand(index: number) {
  if (!selectedTree.value) return;
  if (blockTreeRuleEdit()) return;
  mutate(() => selectedTree.value!.pointBands.splice(index, 1));
}

function nextUniqueNodeId(base: string): string {
  const ids = new Set(selectedTree.value?.nodes.map((node) => node.id) ?? []);
  if (!ids.has(base)) return base;
  let index = 2;
  while (ids.has(`${base}_${index}`)) index += 1;
  return `${base}_${index}`;
}

function addNode() {
  if (!selectedTree.value) return;
  if (blockTreeRuleEdit()) return;
  mutate(() => {
    const node = createDefaultSpecializationNode([newNodeSchool.value], selectedTree.value!.nodes.length);
    node.id = nextUniqueNodeId(node.id);
    node.nameKey = `specialization.node.${node.id}.name`;
    node.descKey = `specialization.node.${node.id}.desc`;
    selectedTree.value!.nodes.push(node);
    for (const localeId of nodeLocaleIds) {
      const entry = ensureNodeLocaleEntry(localeId, node.id);
      entry.name = node.id;
      entry.desc = localeId === 'zh-CN' ? '待填写节点风味文案。' : 'Node flavor copy to be authored.';
      entry.effectSummary = [localeId === 'zh-CN' ? '待填写玩家效果说明。' : 'Player-facing effect summary to be authored.'];
      entry.levelEffectSummary = [clone(entry.effectSummary)];
    }
    selectedNodeId.value = node.id;
    inspectorTab.value = 'basic';
  }, '已新增专精节点；使用全新 ID 的兼容新增只需递增 balanceRevision');
}

function duplicateNode() {
  if (!selectedTree.value || !selectedNode.value) return;
  if (blockTreeRuleEdit()) return;
  mutate(() => {
    const sourceId = selectedNode.value!.id;
    const copy = clone(selectedNode.value!);
    copy.id = nextUniqueNodeId(`${copy.id}_copy`);
    copy.nameKey = `specialization.node.${copy.id}.name`;
    copy.descKey = `specialization.node.${copy.id}.desc`;
    const offset = asString(selectedTree.value!.ui.layout) === 'five_spoke' ? 0.04 : 36;
    copy.ui = {
      ...copy.ui,
      x: Math.min(1, Number(copy.ui.x ?? 0) + offset),
      y: Math.min(1, Number(copy.ui.y ?? 0) + offset)
    };
    copy.levels.forEach((level) => level.grants.forEach((grant, grantIndex) => {
      grant.id = `${copy.id}_grant_${grantIndex + 1}`;
      asArray(grant.battleHooks).filter(isRecord).forEach((hook, hookIndex) => { hook.id = `${copy.id}_hook_${hookIndex + 1}`; });
      asArray(grant.effectMutations).filter(isRecord).forEach((mutation, mutationIndex) => { mutation.id = `${copy.id}_mutation_${mutationIndex + 1}`; });
    }));
    const i18n = isRecord(selectedTree.value!.i18n) ? selectedTree.value!.i18n : {};
    for (const localeId of ['zh-CN', 'en-US']) {
      const locale = isRecord(i18n[localeId]) ? i18n[localeId] : {};
      const nodes = isRecord(locale.nodes) ? locale.nodes : {};
      if (isRecord(nodes[sourceId])) nodes[copy.id] = clone(nodes[sourceId]);
      locale.nodes = nodes;
      i18n[localeId] = locale;
    }
    selectedTree.value!.i18n = i18n;
    selectedTree.value!.nodes.push(copy);
    selectedNodeId.value = copy.id;
  }, '已复制专精节点');
}

function deleteNode() {
  if ((isPersistedTree() || isPersistedNode()) && !rulesEditMode.value) {
    emit('status', '已阻止删除当前节点；请先进入规则编辑并更新唯一 rulesetVersion');
    return;
  }
  if (!selectedTree.value || !selectedNode.value || !confirm(`删除节点 ${selectedNode.value.id}？`)) return;
  const id = selectedNode.value.id;
  mutate(() => {
    const index = selectedTree.value!.nodes.findIndex((node) => node.id === id);
    selectedTree.value!.nodes.splice(index, 1);
    if (isRecord(selectedTree.value!.i18n)) {
      for (const locale of Object.values(selectedTree.value!.i18n).filter(isRecord)) {
        if (isRecord(locale.nodes)) delete locale.nodes[id];
      }
    }
    selectedNodeId.value = selectedTree.value!.nodes[Math.max(0, index - 1)]?.id ?? '';
    selectedLevelIndex.value = 0;
  }, '已删除专精节点');
}

function ensureNodeLocaleEntry(localeId: typeof nodeLocaleIds[number], nodeId = selectedNode.value?.id ?? ''): DataRow {
  if (!selectedTree.value || !nodeId) return {};
  const tree = selectedTree.value as DataRow;
  if (!isRecord(tree.i18n)) tree.i18n = {};
  const i18n = tree.i18n as DataRow;
  if (!isRecord(i18n[localeId])) i18n[localeId] = {};
  const locale = i18n[localeId] as DataRow;
  if (!isRecord(locale.nodes)) locale.nodes = {};
  const nodes = locale.nodes as DataRow;
  if (!isRecord(nodes[nodeId])) nodes[nodeId] = { name: '', desc: '', effectSummary: [''], levelEffectSummary: [['']] };
  const entry = nodes[nodeId] as DataRow;
  if (!Array.isArray(entry.effectSummary)) entry.effectSummary = [''];
  const levelCount = selectedTree.value.nodes.find((node) => node.id === nodeId)?.levels.length ?? 1;
  if (!Array.isArray(entry.levelEffectSummary)) entry.levelEffectSummary = [];
  const summaries = entry.levelEffectSummary as unknown[];
  while (summaries.length < levelCount) summaries.push(clone(summaries[summaries.length - 1] || entry.effectSummary));
  if (summaries.length > levelCount) summaries.splice(levelCount);
  for (let index = 0; index < summaries.length; index += 1) if (!Array.isArray(summaries[index]) || (summaries[index] as unknown[]).length === 0) summaries[index] = clone(entry.effectSummary);
  return entry;
}

function nodeLocaleEntry(localeId: typeof nodeLocaleIds[number]): DataRow {
  if (!selectedTree.value || !selectedNode.value || !isRecord(selectedTree.value.i18n)) return {};
  const locale = selectedTree.value.i18n[localeId];
  if (!isRecord(locale) || !isRecord(locale.nodes) || !isRecord(locale.nodes[selectedNode.value.id])) return {};
  return locale.nodes[selectedNode.value.id] as DataRow;
}

function nodeEffectSummary(localeId: typeof nodeLocaleIds[number]): string[] {
  return asArray(nodeLocaleEntry(localeId).effectSummary).map((line) => asString(line));
}

function nodeLevelEffectSummary(localeId: typeof nodeLocaleIds[number], levelIndex = selectedLevelIndex.value): string[] {
  const summaries = asArray(nodeLocaleEntry(localeId).levelEffectSummary);
  return asArray(summaries[levelIndex]).map((line) => asString(line));
}

function updateNodeLocaleField(localeId: typeof nodeLocaleIds[number], key: 'name' | 'desc', value: string) {
  if (!selectedNode.value || blockNodeRuleEdit()) return;
  mutate(() => { ensureNodeLocaleEntry(localeId)[key] = value; });
}

function updateEffectSummaryLine(localeId: typeof nodeLocaleIds[number], index: number, value: string) {
  if (!selectedNode.value || blockNodeRuleEdit()) return;
  mutate(() => { (ensureNodeLocaleEntry(localeId).effectSummary as unknown[])[index] = value; });
}

function addEffectSummaryLine(localeId: typeof nodeLocaleIds[number]) {
  if (!selectedNode.value || blockNodeRuleEdit()) return;
  mutate(() => {
    const lines = ensureNodeLocaleEntry(localeId).effectSummary as unknown[];
    if (lines.length < 7) lines.push('');
  });
}

function removeEffectSummaryLine(localeId: typeof nodeLocaleIds[number], index: number) {
  if (!selectedNode.value || blockNodeRuleEdit()) return;
  mutate(() => {
    const lines = ensureNodeLocaleEntry(localeId).effectSummary as unknown[];
    if (lines.length > 1) lines.splice(index, 1);
  });
}

function moveEffectSummaryLine(localeId: typeof nodeLocaleIds[number], index: number, direction: -1 | 1) {
  if (!selectedNode.value || blockNodeRuleEdit()) return;
  mutate(() => {
    const lines = ensureNodeLocaleEntry(localeId).effectSummary as unknown[];
    const target = index + direction;
    if (target < 0 || target >= lines.length) return;
    [lines[index], lines[target]] = [lines[target], lines[index]];
  });
}

function updateLevelEffectSummaryLine(localeId: typeof nodeLocaleIds[number], levelIndex: number, lineIndex: number, value: string) {
  if (!selectedNode.value || blockNodeRuleEdit()) return;
  mutate(() => {
    const summaries = ensureNodeLocaleEntry(localeId).levelEffectSummary as unknown[][];
    summaries[levelIndex][lineIndex] = value;
  });
}

function addLevelEffectSummaryLine(localeId: typeof nodeLocaleIds[number], levelIndex: number) {
  if (!selectedNode.value || blockNodeRuleEdit()) return;
  mutate(() => {
    const lines = (ensureNodeLocaleEntry(localeId).levelEffectSummary as unknown[][])[levelIndex];
    if (lines.length < 4) lines.push('');
  });
}

function removeLevelEffectSummaryLine(localeId: typeof nodeLocaleIds[number], levelIndex: number, lineIndex: number) {
  if (!selectedNode.value || blockNodeRuleEdit()) return;
  mutate(() => {
    const lines = (ensureNodeLocaleEntry(localeId).levelEffectSummary as unknown[][])[levelIndex];
    if (lines.length > 1) lines.splice(lineIndex, 1);
  });
}

function updateNodeField(key: keyof SpecializationNode, value: unknown) {
  if (!selectedNode.value || !selectedTree.value) return;
  if (blockNodeRuleEdit()) return;
  mutate(() => {
    if (key === 'id') {
      const oldId = selectedNode.value!.id;
      const newId = asString(value).trim();
      const wasPersisted = persistedNodeKeys.value.has(`${selectedTree.value!.id}\u0000${oldId}`);
      selectedNode.value!.id = newId;
      for (const node of selectedTree.value!.nodes) {
        const groups = [node.requirements.all, ...node.requirements.anyOf];
        for (const requirement of groups.flat()) if (requirement.kind === 'node_rank' && requirement.nodeId === oldId) requirement.nodeId = newId;
      }
      if (isRecord(selectedTree.value!.i18n)) {
        for (const locale of Object.values(selectedTree.value!.i18n).filter(isRecord)) {
          if (!isRecord(locale.nodes) || !isRecord(locale.nodes[oldId])) continue;
          locale.nodes[newId] = locale.nodes[oldId];
          delete locale.nodes[oldId];
        }
      }
      if (wasPersisted) persistedNodeKeys.value = new Set([...persistedNodeKeys.value, `${selectedTree.value!.id}\u0000${newId}`]);
      selectedNodeId.value = newId;
      return;
    }
    (selectedNode.value as DataRow)[key] = value;
    if (key === 'kind' && selectedNode.value!.levels[0]) {
      const costs: Record<string, number> = { root: 2, technique: 2, confluence: 6, terminal: 4 };
      selectedNode.value!.levels[0].cost = costs[asString(value)] ?? selectedNode.value!.levels[0].cost;
    }
  });
}

function updateNodeUI(key: string, value: unknown) {
  if (!selectedNode.value) return;
  mutate(() => { selectedNode.value!.ui[key] = value; });
}

function toggleSchool(school: SpecializationSchool, checked: boolean) {
  if (!selectedNode.value) return;
  if (blockNodeRuleEdit()) return;
  const current = selectedNode.value.schoolIds;
  if (checked && (current.includes(school) || current.length >= 2)) return;
  if (!checked && (current.length <= 1 || !current.includes(school))) return;
  mutate(() => {
    selectedNode.value!.schoolIds = checked ? [...current, school] : current.filter((id) => id !== school);
    if (selectedNode.value!.schoolIds.length > 1) selectedNode.value!.kind = 'confluence';
    else if (selectedNode.value!.kind === 'confluence') selectedNode.value!.kind = 'technique';
    const costs: Record<string, number> = { root: 2, technique: 2, confluence: 6, terminal: 4 };
    if (selectedNode.value!.levels[0]) selectedNode.value!.levels[0].cost = costs[selectedNode.value!.kind] ?? selectedNode.value!.levels[0].cost;
  });
}

function addRequirement(target: 'all' | 'anyOf', groupIndex = 0) {
  if (!selectedNode.value) return;
  if (blockNodeRuleEdit()) return;
  mutate(() => {
    const requirement = createDefaultSpecializationRequirement(requirementKindToAdd.value);
    if (target === 'all') selectedNode.value!.requirements.all.push(requirement);
    else selectedNode.value!.requirements.anyOf[groupIndex].push(requirement);
  });
}

function addAnyOfGroup() {
  if (!selectedNode.value) return;
  if (blockNodeRuleEdit()) return;
  mutate(() => selectedNode.value!.requirements.anyOf.push([createDefaultSpecializationRequirement(requirementKindToAdd.value)]));
}

function updateRequirement(target: 'all' | 'anyOf', index: number, key: string, value: unknown, groupIndex = 0) {
  if (!selectedNode.value) return;
  if (blockNodeRuleEdit()) return;
  mutate(() => {
    const list = target === 'all' ? selectedNode.value!.requirements.all : selectedNode.value!.requirements.anyOf[groupIndex];
    const requirement = list[index];
    if (key === 'kind') {
      list[index] = createDefaultSpecializationRequirement(value as SpecializationRequirementKind);
    } else requirement[key] = value;
  });
}

function removeRequirement(target: 'all' | 'anyOf', index: number, groupIndex = 0) {
  if (!selectedNode.value) return;
  if (blockNodeRuleEdit()) return;
  mutate(() => {
    const list = target === 'all' ? selectedNode.value!.requirements.all : selectedNode.value!.requirements.anyOf[groupIndex];
    list.splice(index, 1);
  });
}

function removeAnyOfGroup(index: number) {
  if (!selectedNode.value) return;
  if (blockNodeRuleEdit()) return;
  mutate(() => selectedNode.value!.requirements.anyOf.splice(index, 1));
}

function addLevel() {
  if (!selectedNode.value) return;
  if (blockNodeRuleEdit()) return;
  if (selectedNode.value.levels.length >= maximumRankCount(selectedNode.value)) return emit('status', `该层级专精最多 ${maximumRankCount(selectedNode.value)} 级`);
  if (selectedNode.value.levels.some((level) => level.grants.some((grant) => grant.kind === 'equipment_slot_capacity'))) return emit('status', '槽位交换必须保持单级，不能追加等级');
  mutate(() => {
    const node = selectedNode.value!;
    const previous = node.levels[node.levels.length - 1]!;
    node.levels.push(createNextSpecializationLevel(previous, node.kind));
    selectedLevelIndex.value = node.levels.length - 1;
    for (const localeId of nodeLocaleIds) {
      const entry = ensureNodeLocaleEntry(localeId);
      const summaries = entry.levelEffectSummary as unknown[][];
      summaries[selectedLevelIndex.value] = clone(summaries[selectedLevelIndex.value - 1] || entry.effectSummary) as unknown[];
    }
  }, '已复制上一等级的完整效果快照；请继续增强机制并填写本级文案');
}

function maximumRankCount(node: SpecializationNode): number {
  if (node.levels.some((level) => level.grants.some((grant) => grant.kind === 'equipment_slot_capacity'))) return 1;
  if (node.kind === 'root') return 6;
  if (node.kind === 'technique') return 4;
  return 3;
}

function updateLevel(index: number, key: 'cost', value: number) {
  if (!selectedNode.value?.levels[index]) return;
  if (isPersistedNode() && !rulesEditMode.value) {
    emit('status', '已保存节点改价会改变投入成本；请先进入规则编辑并更新唯一 rulesetVersion');
    return;
  }
  mutate(() => { selectedNode.value!.levels[index][key] = value; });
}

function removeLevel(index: number) {
  if (!selectedNode.value || selectedNode.value.levels.length <= 1) return;
  if (blockNodeRuleEdit()) return;
  mutate(() => {
    selectedNode.value!.levels.splice(index, 1);
    selectedNode.value!.levels.forEach((level, levelIndex) => { level.level = levelIndex + 1; });
    for (const localeId of nodeLocaleIds) {
      const entry = ensureNodeLocaleEntry(localeId);
      (entry.levelEffectSummary as unknown[]).splice(index, 1);
    }
    selectedLevelIndex.value = Math.min(selectedLevelIndex.value, selectedNode.value!.levels.length - 1);
  });
}

function addGrant(kind = grantKindToAdd.value) {
  if (!selectedLevel.value) return;
  if (blockNodeRuleEdit()) return;
  mutate(() => selectedLevel.value!.grants.push(createDefaultSpecializationGrant(kind)));
}

function removeGrant(index: number) {
  if (!selectedLevel.value) return;
  if (blockNodeRuleEdit()) return;
  mutate(() => selectedLevel.value!.grants.splice(index, 1));
}

function updateGrant(index: number, key: string, value: unknown) {
  if (!selectedLevel.value || !isRecord(selectedLevel.value.grants[index])) return;
  if (blockNodeRuleEdit()) return;
  mutate(() => { selectedLevel.value!.grants[index][key] = value; });
}

function beginJsonEdit() {
  remember();
}

function updateTreeJson(key: string, value: unknown) {
  if (!selectedTree.value) return;
  (selectedTree.value as DataRow)[key] = value;
  emitChange();
}

function addModifier(grantIndex: number) {
  if (!selectedLevel.value) return;
  if (blockNodeRuleEdit()) return;
  mutate(() => {
    const grant = selectedLevel.value!.grants[grantIndex];
    if (!Array.isArray(grant.modifiers)) grant.modifiers = [];
    (grant.modifiers as DataRow[]).push({ attributeKey: 'strength', operation: 'add', value: 1 });
  });
}

function updateModifier(grantIndex: number, modifierIndex: number, key: string, value: unknown) {
  if (!selectedLevel.value) return;
  if (blockNodeRuleEdit()) return;
  mutate(() => {
    const grant = selectedLevel.value!.grants[grantIndex];
    const modifiers = asArray(grant.modifiers).filter(isRecord);
    modifiers[modifierIndex][key] = value;
    grant.modifiers = modifiers;
  });
}

function removeModifier(grantIndex: number, modifierIndex: number) {
  if (!selectedLevel.value) return;
  if (blockNodeRuleEdit()) return;
  mutate(() => {
    const grant = selectedLevel.value!.grants[grantIndex];
    const modifiers = asArray(grant.modifiers).filter(isRecord);
    modifiers.splice(modifierIndex, 1);
    grant.modifiers = modifiers;
  });
}

function updateSlotDelta(grantIndex: number, family: string, value: number) {
  if (!selectedLevel.value) return;
  if (blockNodeRuleEdit()) return;
  mutate(() => {
    const grant = selectedLevel.value!.grants[grantIndex];
    if (!isRecord(grant.deltas)) grant.deltas = {};
    if (value === 0) delete (grant.deltas as DataRow)[family];
    else (grant.deltas as DataRow)[family] = Math.trunc(value);
  });
}

const SCHOOL_ANGLES: Record<SpecializationSchool, number> = {
  strength: -Math.PI / 2,
  dexterity: -Math.PI / 10,
  intelligence: Math.PI * 3 / 10,
  perception: Math.PI * 7 / 10,
  stamina: Math.PI * 11 / 10
};
const NORMALIZED_CENTER = { x: 1050, y: 1000 };
const FLOW_NODE_HALF = { x: 77, y: 40 };

function normalizedCanvasPosition(node: SpecializationNode, x: number, y: number): { x: number; y: number } {
  const radius = Math.hypot(x, y);
  const angle = Math.atan2(y, x);
  const confluence = node.kind === 'confluence' || node.schoolIds.length > 1;
  const canvasRadius = confluence ? 120 + radius * 1300 : 420 + Math.max(0, radius - 0.3) * 1000;
  return {
    x: NORMALIZED_CENTER.x + Math.cos(angle) * canvasRadius - FLOW_NODE_HALF.x,
    y: NORMALIZED_CENTER.y + Math.sin(angle) * canvasRadius - FLOW_NODE_HALF.y
  };
}

function automaticRadialPosition(node: SpecializationNode): { x: number; y: number } {
  const nodes = selectedTree.value?.nodes ?? [];
  const center = { x: 500, y: 390 };
  if (node.schoolIds.length > 1 || node.kind === 'confluence') {
    const firstIndex = SPECIALIZATION_SCHOOLS.indexOf(node.schoolIds[0] ?? 'strength');
    const secondIndex = SPECIALIZATION_SCHOOLS.indexOf(node.schoolIds[1] ?? 'dexterity');
    const firstAngle = SCHOOL_ANGLES[node.schoolIds[0] ?? 'strength'];
    const secondAngle = SCHOOL_ANGLES[node.schoolIds[1] ?? 'dexterity'];
    const x = Math.cos(firstAngle) + Math.cos(secondAngle);
    const y = Math.sin(firstAngle) + Math.sin(secondAngle);
    const angle = Math.atan2(y, x);
    const distance = Math.min(Math.abs(firstIndex - secondIndex), SPECIALIZATION_SCHOOLS.length - Math.abs(firstIndex - secondIndex));
    const radius = distance === 1 ? 330 : 180;
    return { x: center.x + Math.cos(angle) * radius - 92, y: center.y + Math.sin(angle) * radius - 46 };
  }
  const school = node.schoolIds[0] ?? 'strength';
  const branch = nodes
    .filter((entry) => entry.schoolIds.length === 1 && entry.schoolIds[0] === school && entry.kind !== 'confluence')
    .sort((a, b) => Number(a.ui.column) - Number(b.ui.column) || Number(a.ui.row) - Number(b.ui.row) || a.id.localeCompare(b.id));
  const row = Number(node.ui.row);
  const rows = Array.from(new Set(branch.map((entry) => Number(entry.ui.row)))).sort((a, b) => a - b);
  const depth = Math.max(1, rows.indexOf(row) + 1);
  const peers = branch.filter((entry) => Number(entry.ui.row) === row);
  const peerIndex = Math.max(0, peers
    .sort((a, b) => Number(a.ui.column) - Number(b.ui.column) || a.id.localeCompare(b.id))
    .findIndex((entry) => entry.id === node.id));
  const lane = peerIndex - (peers.length - 1) / 2;
  const angle = SCHOOL_ANGLES[school] + lane * 0.16;
  const radius = 390 + depth * 170;
  return { x: center.x + Math.cos(angle) * radius - 92, y: center.y + Math.sin(angle) * radius - 46 };
}

function automaticNormalizedPosition(node: SpecializationNode, confluenceIndex: number): { x: number; y: number; column: number; row: number } {
  const schoolIds: SpecializationSchool[] = node.schoolIds.length ? node.schoolIds : ['strength'];
  if (node.kind === 'confluence' || schoolIds.length > 1) {
    const first = schoolIds[0] ?? 'strength';
    const second = schoolIds[1] ?? 'dexterity';
    const firstIndex = SPECIALIZATION_SCHOOLS.indexOf(first);
    const secondIndex = SPECIALIZATION_SCHOOLS.indexOf(second);
    const distance = Math.min(Math.abs(firstIndex - secondIndex), SPECIALIZATION_SCHOOLS.length - Math.abs(firstIndex - secondIndex));
    const vectorX = Math.cos(SCHOOL_ANGLES[first]) + Math.cos(SCHOOL_ANGLES[second]);
    const vectorY = Math.sin(SCHOOL_ANGLES[first]) + Math.sin(SCHOOL_ANGLES[second]);
    const angle = Math.atan2(vectorY, vectorX);
    const radius = distance === 1 ? 0.24 : 0.12;
    return { x: Math.cos(angle) * radius, y: Math.sin(angle) * radius, column: 30 + confluenceIndex, row: 0 };
  }
  const school = schoolIds[0];
  const schoolIndex = Math.max(0, SPECIALIZATION_SCHOOLS.indexOf(school));
  const peers = (selectedTree.value?.nodes ?? [])
    .filter((entry) => entry.schoolIds.length === 1 && entry.schoolIds[0] === school && entry.kind === node.kind)
    .sort((a, b) => a.id.localeCompare(b.id));
  const peerIndex = Math.max(0, peers.findIndex((entry) => entry.id === node.id));
  const lane = peerIndex - (peers.length - 1) / 2;
  const radius = node.kind === 'root' ? 0.38 : node.kind === 'terminal' ? 0.88 : 0.63;
  const angle = SCHOOL_ANGLES[school] + lane * 0.12;
  const row = node.kind === 'root' ? 0 : node.kind === 'terminal' ? 2 : 1;
  return { x: Math.cos(angle) * radius, y: Math.sin(angle) * radius, column: schoolIndex * 3 + peerIndex, row };
}

function autoLayout() {
  if (!selectedTree.value) return;
  mutate(() => {
    let confluenceIndex = 0;
    for (const node of selectedTree.value!.nodes) {
      const position = automaticNormalizedPosition(node, confluenceIndex);
      if (node.kind === 'confluence' || node.schoolIds.length > 1) confluenceIndex += 1;
      node.ui.x = Math.round(position.x * 1000) / 1000;
      node.ui.y = Math.round(position.y * 1000) / 1000;
      node.ui.column = position.column;
      node.ui.row = position.row;
      node.ui.accent = node.ui.accent || schoolColor(node.schoolIds[0] ?? 'strength');
    }
    selectedTree.value!.ui = { ...selectedTree.value!.ui, layout: 'five_spoke' };
  }, '已按五辐结构自动布局');
  void nextTick(() => fitView({ padding: 0.16, duration: 300 }));
}

function localizedNodeSearchText(tree: SpecializationTree, node: SpecializationNode): string {
  const chunks = [node.id, node.nameKey, node.descKey, node.kind, ...node.schoolIds];
  const i18n = isRecord(tree.i18n) ? tree.i18n : {};
  for (const localeId of ['zh-CN', 'en-US']) {
    const locale = isRecord(i18n[localeId]) ? i18n[localeId] : {};
    const nodes: DataRow = isRecord(locale.nodes) ? locale.nodes : {};
    const entry: DataRow = isRecord(nodes[node.id]) ? nodes[node.id] as DataRow : {};
    chunks.push(asString(entry.name), asString(entry.desc), ...asArray(entry.effectSummary).map((line) => asString(line)));
  }
  return chunks.join(' ').toLocaleLowerCase();
}

function localizedNodeName(tree: SpecializationTree, node: SpecializationNode): string {
  const i18n = isRecord(tree.i18n) ? tree.i18n : {};
  for (const localeId of ['zh-CN', 'en-US']) {
    const locale = isRecord(i18n[localeId]) ? i18n[localeId] : {};
    const nodes: DataRow = isRecord(locale.nodes) ? locale.nodes : {};
    const entry: DataRow = isRecord(nodes[node.id]) ? nodes[node.id] as DataRow : {};
    const name = asString(entry.name).trim();
    if (name) return name;
  }
  return node.id;
}

const searchMatches = computed(() => {
  const query = searchQuery.value.trim().toLocaleLowerCase();
  if (!query || !selectedTree.value) return [];
  return selectedTree.value.nodes.filter((node) => localizedNodeSearchText(selectedTree.value!, node).includes(query));
});

watch(searchQuery, () => { searchCursor.value = 0; });

function focusSearchMatch(direction = 0) {
  if (searchMatches.value.length === 0) return;
  if (direction !== 0) searchCursor.value = (searchCursor.value + direction + searchMatches.value.length) % searchMatches.value.length;
  else searchCursor.value = Math.min(searchCursor.value, searchMatches.value.length - 1);
  const match = searchMatches.value[searchCursor.value];
  selectedNodeId.value = match.id;
  selectedLevelIndex.value = 0;
  void nextTick(() => {
    const node = flowNodes.value.find((entry) => entry.id === match.id);
    if (node) void setCenter(node.position.x + FLOW_NODE_HALF.x, node.position.y + FLOW_NODE_HALF.y, { zoom: 1.15, duration: 260 });
  });
}

const flowNodes = computed<Node[]>(() => (selectedTree.value?.nodes ?? []).map((node) => {
  const first = node.schoolIds[0] ?? 'strength';
  const fallback = automaticRadialPosition(node);
  const normalized = Number.isFinite(Number(node.ui.x)) && Number.isFinite(Number(node.ui.y));
  const position = asString(selectedTree.value?.ui.layout) === 'five_spoke' && normalized
    ? normalizedCanvasPosition(node, Number(node.ui.x), Number(node.ui.y))
    : fallback;
  return {
    id: node.id,
    type: 'specialization',
    position,
    selected: node.id === selectedNodeId.value,
    data: {
      id: node.id,
      displayName: localizedNodeName(selectedTree.value!, node),
      kind: node.kind,
      schools: node.schoolIds,
      schoolText: node.schoolIds.map((id) => SPECIALIZATION_SCHOOL_META[id]?.label ?? id).join(' × '),
      maxLevel: node.levels.length,
      totalCost: node.levels.reduce((sum, level) => sum + Number(level.cost || 0), 0),
      issueCount: nodeIssueCounts.value.get(node.id) ?? 0,
      color: schoolColor(first)
    }
  };
}));

const flowEdges = computed<Edge[]>(() => {
  const edges: Edge[] = [];
  const seen = new Set<string>();
  for (const node of selectedTree.value?.nodes ?? []) {
    const alternativeIds = new Set(node.requirements.anyOf.flat()
      .filter((requirement) => requirement.kind === 'node_rank')
      .map((requirement) => asString(requirement.nodeId))
      .filter(Boolean));
    const groups = [node.requirements.all, ...node.requirements.anyOf];
    for (const requirement of groups.flat()) {
      if (requirement.kind !== 'node_rank') continue;
      const source = asString(requirement.nodeId);
      if (!source) continue;
      const key = `${source}->${node.id}`;
      if (seen.has(key)) continue;
      seen.add(key);
      edges.push({
        id: key,
        source,
        target: node.id,
        animated: node.kind === 'confluence',
        style: { stroke: '#7d8aa3', strokeWidth: 2, strokeDasharray: alternativeIds.has(source) ? '7 5' : undefined }
      });
    }
  }
  return edges;
});

function nodeStyle(schools: SpecializationSchool[]): CSSProperties {
  const colors = schools.map((id) => schoolColor(id));
  return {
    '--spec-node-accent': colors[0],
    '--spec-node-fill': colors.length > 1
      ? `linear-gradient(135deg, ${colors[0]} 0 47%, ${colors[1]} 53% 100%)`
      : colors[0]
  } as CSSProperties;
}

const miniMapNodeColor: MiniMapNodeFunc = (node) => isRecord(node.data) && asString(node.data.color).trim()
  ? asString(node.data.color).trim()
  : '#7d8aa3';

function onNodeClick(event: NodeMouseEvent) {
  selectedNodeId.value = event.node.id;
  selectedLevelIndex.value = 0;
}

function onNodeDragStop(event: NodeDragEvent) {
  const node = selectedTree.value?.nodes.find((entry) => entry.id === event.node.id);
  if (!node) return;
  mutate(() => {
    if (asString(selectedTree.value?.ui.layout) === 'five_spoke') {
      const dx = event.node.position.x + FLOW_NODE_HALF.x - NORMALIZED_CENTER.x;
      const dy = event.node.position.y + FLOW_NODE_HALF.y - NORMALIZED_CENTER.y;
      const canvasRadius = Math.hypot(dx, dy);
      const confluence = node.kind === 'confluence' || node.schoolIds.length > 1;
      const normalizedRadius = confluence
        ? Math.max(0, (canvasRadius - 120) / 1300)
        : Math.max(0.3, 0.3 + (canvasRadius - 420) / 1000);
      const angle = Math.atan2(dy, dx);
      const x = Math.cos(angle) * normalizedRadius;
      const y = Math.sin(angle) * normalizedRadius;
      node.ui.x = Math.max(-1, Math.min(1, Math.round(x * 1000) / 1000));
      node.ui.y = Math.max(-1, Math.min(1, Math.round(y * 1000) / 1000));
    } else {
      node.ui.x = Math.round(event.node.position.x);
      node.ui.y = Math.round(event.node.position.y);
      node.ui.column = Math.round((event.node.position.x - 500) / 220);
      node.ui.row = Math.round((event.node.position.y - 400) / 170);
    }
  });
}

function onConnect(connection: Connection) {
  if (!connection.source || !connection.target || connection.source === connection.target || !selectedTree.value) return;
  const target = selectedTree.value.nodes.find((node) => node.id === connection.target);
  if (!target || target.requirements.all.some((req) => req.kind === 'node_rank' && req.nodeId === connection.source)) return;
  if (isPersistedNode(target) && !rulesEditMode.value) {
    emit('status', '已阻止修改当前节点前置；请先进入规则编辑并更新唯一 rulesetVersion');
    return;
  }
  mutate(() => target.requirements.all.push({ kind: 'node_rank', nodeId: connection.source, minRank: 1 }));
}

function focusIssue(index: number) {
  const issue = selectedTreeIssues.value[index];
  if (!issue) return;
  const treeIndex = issue.treeId ? draftRows.value.findIndex((tree) => tree.id === issue.treeId) : selectedTreeIndex.value;
  if (treeIndex >= 0) selectedTreeIndex.value = treeIndex;
  if (issue.nodeId) selectedNodeId.value = issue.nodeId;
  inspectorTab.value = issue.tab;
  void nextTick(() => {
    const node = flowNodes.value.find((entry) => entry.id === issue.nodeId);
    if (node) void setCenter(node.position.x + FLOW_NODE_HALF.x, node.position.y + FLOW_NODE_HALF.y, { zoom: 1.15, duration: 300 });
  });
}

function topologyGrantEntries() {
  return (selectedLevel.value?.grants ?? [])
    .map((grant, index) => ({ grant, index }))
    .filter(({ grant }) => grant.kind === 'equipment_slot_capacity');
}

function regularGrantEntries() {
  return (selectedLevel.value?.grants ?? [])
    .map((grant, index) => ({ grant, index }))
    .filter(({ grant }) => grant.kind !== 'equipment_slot_capacity');
}

function numberText(value: unknown): string {
  const number = Number(value);
  return Number.isFinite(number) ? String(Math.round(number * 1000) / 1000) : '数值错误';
}

function ratioText(value: unknown): string {
  const number = Number(value);
  return Number.isFinite(number) ? `${Math.round(number * 10000) / 100}%` : '数值错误';
}

function formulaText(raw: unknown): string {
  if (!isRecord(raw)) return '公式错误';
  const terms = asArray(raw.terms).filter(isRecord).map((term) => `${asString(term.attributeKey)} × ${numberText(term.coefficient)}`);
  const base = Number(raw.baseAdd);
  if (Number.isFinite(base) && base !== 0) terms.unshift(numberText(base));
  return terms.length ? terms.join(' + ') : '公式错误';
}

function valueFormulaSuffix(raw: unknown): string {
  return isRecord(raw) && isRecord(raw.valueFormula) ? `；公式 ${formulaText(raw.valueFormula)}` : '';
}

function conditionText(raw: unknown): string {
  if (!isRecord(raw)) return '条件配置错误';
  const kind = asString(raw.kind);
  const fixed: Record<string, string> = {
    adjacent_ally_exists: '存在相邻友军', context_mp_insufficient: '当前 MP 不足', context_skill_non_basic: '当前技能不是普攻', incoming_damage_would_be_lethal: '本次伤害将致命', source_is_enemy: '来源为敌人', target_is_lowest_hp_ally: '目标是最低生命友军'
  };
  if (fixed[kind]) return fixed[kind];
  if (kind === 'context_damage_type') return `伤害类型为 ${asString(raw.value)}`;
  if (kind === 'context_hit_count_gte') return `当前技能至少 ${numberText(raw.value)} 段`;
  if (kind === 'counter_gte') return `${asString(raw.counterId)} 至少 ${numberText(raw.value)} 层`;
  if (kind === 'self_hp_ratio_lte') return `自身生命不高于 ${ratioText(raw.value)}`;
  if (kind === 'self_mp_ratio_gte') return `自身 MP 不低于 ${ratioText(raw.value)}`;
  if (kind === 'self_weapon_class') return `装备武器类别：${asArray(raw.weaponClasses).map((value) => asString(value)).join(' / ')}`;
  if (kind === 'target_has_buff') return `目标带状态 ${asString(raw.buffId)}`;
  if (kind === 'target_has_any_buff') return `目标带任一状态：${asArray(raw.buffIds).map((value) => asString(value)).join(' / ')}`;
  if (kind === 'trigger_limit') return ['perTurn', 'perWave', 'perBattle'].filter((key) => raw[key] !== undefined).map((key) => `${key} ${numberText(raw[key])} 次`).join('，');
  return `条件配置错误：${kind || '(空)'}`;
}

function payoffText(raw: unknown): string {
  if (!isRecord(raw)) return '满层收益配置错误';
  const consume = `消耗 ${numberText(raw.consumeStacks)} 层`;
  if (raw.kind === 'empower_next_hit') return `${consume}，下一击增伤 ${ratioText(raw.damageAmp)}${valueFormulaSuffix(raw)}`;
  if (raw.kind === 'restore_mp_and_tick_cooldown') return `${consume}，恢复 ${ratioText(raw.mpRatio)} 最大 MP，冷却 ${numberText(raw.cooldownAmount)}${valueFormulaSuffix(raw)}`;
  if (raw.kind === 'restore_mp') return `${consume}，恢复 ${ratioText(raw.mpRatio)} 最大 MP${valueFormulaSuffix(raw)}`;
  if (raw.kind === 'guard_lowest_hp_ally') return `${consume}，最低生命友军减伤 ${ratioText(raw.damageReduction)}，持续 ${numberText(raw.durationTurns)} 回合${valueFormulaSuffix(raw)}`;
  return '满层收益配置错误';
}

function hookEffectText(raw: unknown): string {
  if (!isRecord(raw)) return '效果配置错误';
  const kind = asString(raw.kind);
  const target = asString(raw.targetMode);
  const dispatch = raw.dispatchHooks === true ? '，继续派发 Hook' : '，不再派发 Hook';
  if (kind === 'adjust_skill_cooldown') return `${target ? `${target} ` : ''}技能冷却 ${numberText(raw.amount)}（${asString(raw.selectionMode) || '指定技能'}）`;
  if (kind === 'consume_counter_for_damage_amp') return `消耗 ${numberText(raw.consume)} 层 ${asString(raw.counterId)}，增伤 ${ratioText(raw.value)}${raw.percentDefensePenetration !== undefined ? `，百分比穿甲 ${ratioText(raw.percentDefensePenetration)}` : ''}${valueFormulaSuffix(raw)}`;
  if (kind === 'cycle_counter') return `${asString(raw.counterId)} +${numberText(raw.gain)}（上限 ${numberText(raw.maxStacks)}）${raw.atMax !== undefined ? `；${payoffText(raw.atMax)}` : ''}`;
  if (kind === 'damage') return `对 ${target} 造成 ${asString(raw.damageType)} 伤害（${formulaText(raw.amountFormula)}）${dispatch}`;
  if (kind === 'extra_turn') return `以 ${ratioText(raw.procChanceBase)} 概率获得一次完整额外行动（上限 ${ratioText(raw.procChanceMax)}）${dispatch}`;
  if (kind === 'flat_defense_penetration') return `获得固定穿甲（${formulaText(raw.valueFormula)}）`;
  if (kind === 'heal') return `治疗 ${target} ${raw.maxHpRatio !== undefined ? `${ratioText(raw.maxHpRatio)} 最大生命` : `${ratioText(raw.missingHpRatio)} 已损生命`}${valueFormulaSuffix(raw)}`;
  if (kind === 'buff_cycle') return `首击施加 ${asString(raw.buffId)}，再次命中增伤 ${isRecord(raw.buffedHit) ? ratioText(raw.buffedHit.value) : '配置错误'}${isRecord(raw.buffedHit) ? valueFormulaSuffix(raw.buffedHit) : ''}${isRecord(raw.buffedHit) && raw.buffedHit.consumeBuff ? '并移除状态' : ''}`;
  if (kind === 'mirror_cast_skill') return `复诵技能，倍率 ${ratioText(raw.amountRatio)}，${raw.applyCooldown ? '计入' : '不计'}冷却${dispatch}${valueFormulaSuffix(raw)}`;
  if (kind === 'prevent_lethal') return `阻止一次致命伤并保留 ${ratioText(raw.remainingHpRatio)} 生命${valueFormulaSuffix(raw)}`;
  if (kind === 'redirect_damage') return `向 ${target} 转移 ${ratioText(raw.ratio)} 伤害${valueFormulaSuffix(raw)}`;
  if (kind === 'remove_buff_by_kind') return `移除至多 ${numberText(raw.maxBuffs)} 个 ${asString(raw.buffKind)} 增益`;
  if (kind === 'repeat_context_effect') return `对 ${target} 复现当前效果，倍率 ${ratioText(raw.amountRatio)}${dispatch}${valueFormulaSuffix(raw)}`;
  if (kind === 'resource_debt_cast') return `消耗 ${ratioText(raw.hpCostRatio)} 最大生命透支施法，冷却 ${numberText(raw.cooldownDelta)}${valueFormulaSuffix(raw)}`;
  if (kind === 'resource_shield') return `以 ${asString(raw.resource)} 吸收 ${ratioText(raw.absorbRatio)} 伤害，每点伤害消耗 ${numberText(raw.resourcePerDamage)}${valueFormulaSuffix(raw)}`;
  if (kind === 'restore_mp') return `恢复 ${ratioText(raw.maxMpRatio)} 最大 MP${valueFormulaSuffix(raw)}`;
  if (kind === 'retaliate') return `反击 ${target}（力量 × ${numberText(raw.strengthRatio)} + 耐力 × ${numberText(raw.staminaRatio)}）${dispatch}`;
  if (kind === 'runtime_damage_amp') return `本次伤害提高 ${ratioText(raw.value)}${valueFormulaSuffix(raw)}`;
  if (kind === 'shared_damage_reduction') return `共享减伤 ${ratioText(raw.value)}，持续 ${numberText(raw.durationTurns)} 回合${valueFormulaSuffix(raw)}`;
  if (kind === 'shift_position') return `按 ${asString(raw.policy)} 移动至多 ${numberText(raw.maxSteps)} 格`;
  if (kind === 'add_buff') return `给 ${target || asString(raw.selectionMode) || '目标'} 施加状态 ${asString(raw.buffId)}${raw.durationTurns !== undefined ? `，持续 ${numberText(raw.durationTurns)} 回合` : ''}${dispatch}`;
  if (kind === 'remove_buff') return `从 ${target || '目标'} 移除状态 ${asString(raw.buffId)}${dispatch}`;
  if (kind === 'cast_skill') return `施放技能 ${asString(raw.skillId)}${raw.amountRatio !== undefined ? `，倍率 ${ratioText(raw.amountRatio)}` : ''}${dispatch}`;
  if (kind === 'consume_hp') return `消耗 ${ratioText(raw.maxHpRatio)} 最大生命，至少保留 ${numberText(raw.minimumHp)} 点`;
  if (kind === 'splash_damage') return `向 ${numberText(raw.targetCount)} 个额外目标扩散，倍率 ${ratioText(raw.amountRatio)}`;
  return `效果配置错误：${kind || '(空)'}`;
}

function mutationSelectorText(mutation: DataRow): string {
  const selector = isRecord(mutation.effectSelector) ? mutation.effectSelector : {};
  const base = selector.mode === 'kind'
    ? `效果类别 ${asArray(selector.kinds).join(' / ')}`
    : selector.mode === 'effect_id'
      ? `效果 ID ${asArray(selector.effectIds).join(' / ')}`
      : '全部稳定效果';
  const parts = [base];
  if (selector.includeGenerated === true) parts.push('含其他变异生成效果');
  const filters: Array<[string, unknown]> = [
    ['技能', mutation.skillIds],
    ['技能类别', mutation.skillCategories],
    ['武器类别', mutation.weaponClasses],
    ['伤害类型', mutation.damageTypes]
  ];
  for (const [label, raw] of filters) {
    const values = asArray(raw).map((entry) => asString(entry)).filter(Boolean);
    if (values.length) parts.push(`${label} ${values.join(' / ')}`);
  }
  return parts.join(' · ');
}

function mutationOpsText(raw: unknown): string[] {
  if (!isRecord(raw)) return ['变异操作配置错误'];
  const labels: Record<string, string> = { targetCountDelta: '目标数', amountBaseAddDelta: '基础量', cooldownDelta: '冷却', mpCostDelta: 'MP 消耗', hitCountDelta: '段数' };
  const lines: string[] = [];
  for (const key of Object.keys(labels)) if (raw[key] !== undefined) lines.push(`${labels[key]} ${Number(raw[key]) > 0 ? '+' : ''}${numberText(raw[key])}`);
  if (raw.amountMultiplier !== undefined) lines.push(`效果量 × ${numberText(raw.amountMultiplier)}`);
  if (Array.isArray(raw.amountTermsAppend)) lines.push(`追加公式项：${raw.amountTermsAppend.filter(isRecord).map((term) => `${asString(term.attributeKey)} × ${numberText(term.coefficient)}`).join('，')}`);
  if (Array.isArray(raw.statusIdsAppend)) lines.push(`追加状态：${raw.statusIdsAppend.join(' / ')}`);
  if (Array.isArray(raw.statusIdsRemove)) lines.push(`移除状态：${raw.statusIdsRemove.join(' / ')}`);
  if (Array.isArray(raw.appendEffects)) lines.push(...raw.appendEffects.map((effect) => isRecord(effect)
    ? `追加稳定效果 ${asString(effect.id) || '(缺少 id)'}：${hookEffectText(effect)}`
    : '追加效果配置错误'));
  if (raw.attackStatPolicy !== undefined) lines.push(`攻击属性策略：${asString(raw.attackStatPolicy)}`);
  if (raw.damageTypePolicy !== undefined) lines.push(`伤害类型策略：${asString(raw.damageTypePolicy)}`);
  if (isRecord(raw.targetingPatch)) {
    const patch = raw.targetingPatch;
    lines.push(`选敌改写：${Object.entries(patch).map(([key, value]) => `${key}=${Array.isArray(value) ? value.join('/') : String(value)}`).join('，')}`);
  }
  return lines.length ? lines : ['变异操作配置错误'];
}

const mechanicalPreviewLines = computed(() => {
  const lines: string[] = [];
  for (const grant of selectedLevel.value?.grants ?? []) {
    if (grant.kind === 'equipment_slot_capacity') continue;
    if (grant.kind === 'stat_modifier') {
      const scope = grant.scope === 'character' ? '角色投影' : '战斗快照';
      for (const modifier of asArray(grant.modifiers).filter(isRecord)) lines.push(`${scope}：${asString(modifier.attributeKey)} ${modifier.operation === 'multiply' ? '×' : Number(modifier.value) > 0 ? '+' : ''}${numberText(modifier.value)}`);
    } else if (grant.kind === 'battle_hook') {
      for (const hook of asArray(grant.battleHooks).filter(isRecord)) {
        const conditions = asArray(hook.conditions).map(conditionText).filter(Boolean);
        const prefix = `${asString(hook.trigger)} · 优先级 ${numberText(hook.priority)} · ${hook.dispatchHooks === true ? '默认继续派发 Hook' : '默认不再派发 Hook'}${conditions.length ? ` · 当 ${conditions.join(' 且 ')}` : ''}`;
        for (const effect of asArray(hook.effects)) lines.push(`${prefix}：${hookEffectText(effect)}`);
      }
    } else if (grant.kind === 'effect_mutation') {
      for (const mutation of asArray(grant.effectMutations).filter(isRecord)) {
        const prefix = `${asString(mutation.scope)} · ${mutationSelectorText(mutation)}`;
        for (const operation of mutationOpsText(mutation.ops)) lines.push(`${prefix}：${operation}`);
      }
    } else lines.push(`Grant 配置错误：${asString(grant.kind) || '(空)'}`);
  }
  return lines;
});

const SLOT_CAPACITY: Record<string, { label: string; base: number; min: number; max: number }> = {
  head: { label: '头部', base: 1, min: 0, max: 1 },
  accessory: { label: '饰品', base: 4, min: 0, max: 6 },
  right_hand: { label: '右手', base: 1, min: 0, max: 2 }
};

const topologyPreview = computed(() => {
  const deltas: Record<string, number> = {};
  for (const grant of selectedLevel.value?.grants ?? []) {
    if (grant.kind !== 'equipment_slot_capacity' || !isRecord(grant.deltas)) continue;
    for (const [family, value] of Object.entries(grant.deltas)) deltas[family] = (deltas[family] ?? 0) + Number(value);
  }
  return Object.entries(SLOT_CAPACITY).map(([family, rule]) => {
    const delta = deltas[family] ?? 0;
    const after = rule.base + delta;
    return { family, ...rule, delta, after, valid: Number.isInteger(after) && after >= rule.min && after <= rule.max };
  }).filter((entry) => entry.delta !== 0);
});

function concreteSlots(family: string, count: number): string {
  if (family === 'head') return count > 0 ? 'head' : '无';
  if (family === 'right_hand') return count === 2 ? 'right_hand, right_hand_2' : count === 1 ? 'right_hand' : '无';
  if (family === 'accessory') return count > 0 ? Array.from({ length: count }, (_, index) => `accessory_${index + 1}`).join(', ') : '无';
  return '无';
}
</script>

<template>
  <section class="specialization-designer">
    <aside class="tree-panel" :class="{ 'rules-locked': isPersistedTree() && !rulesEditMode }">
      <header class="panel-head">
        <div>
          <h2>专精树</h2>
          <small>{{ draftRows.length }} 套 / {{ selectedTree?.nodes.length ?? 0 }} 节点</small>
        </div>
        <button class="icon-button" title="新增专精树" aria-label="新增专精树" @click="addTree">+</button>
      </header>

      <template v-if="selectedTree">
        <div class="tree-switcher">
          <select :value="selectedTreeIndex" @change="setTreeIndex(Number(($event.target as HTMLSelectElement).value))">
            <option v-for="(tree, index) in draftRows" :key="`${tree.id}:${index}`" :value="index">{{ tree.id }}</option>
          </select>
          <button class="icon-button subtle" title="复制专精树" aria-label="复制专精树" @click="duplicateTree">⎘</button>
          <button class="icon-button danger" :disabled="isPersistedTree() && !rulesEditMode" title="删除专精树" aria-label="删除专精树" @click="deleteTree">×</button>
        </div>

        <div class="tree-fields">
          <label class="rules-edit-toggle"><input v-model="rulesEditMode" type="checkbox" />规则编辑</label>
          <small v-if="rulesEditMode" class="rules-edit-warning">全新 ID 的树/节点新增及数值、Hook、Effect Mutation 调整递增 balanceRevision；删除、改名或改变既有投入/槽位契约时才更新 rulesetVersion。</small>
          <label>id<input :readonly="isPersistedTree() && !rulesEditMode" :value="selectedTree.id" @change="updateTreeField('id', ($event.target as HTMLInputElement).value)" /></label>
          <div class="field-pair">
            <label>schemaVersion<input type="number" min="1" :readonly="isPersistedTree() && !rulesEditMode" :value="selectedTree.schemaVersion" @change="updateTreeField('schemaVersion', Number(($event.target as HTMLInputElement).value))" /></label>
            <label>ownerType<input :value="selectedTree.ownerType" @change="updateTreeField('ownerType', ($event.target as HTMLInputElement).value)" /></label>
          </div>
          <label>rulesetVersion<input :readonly="isPersistedTree() && !rulesEditMode" :value="selectedTree.rulesetVersion" @change="updateTreeField('rulesetVersion', ($event.target as HTMLInputElement).value)" /></label>
          <label class="balance-revision-field">balanceRevision<input type="number" min="1" step="1" :value="selectedTree.balanceRevision" @change="updateTreeField('balanceRevision', Math.max(1, Math.trunc(Number(($event.target as HTMLInputElement).value))))" /></label>
          <label>nameKey<input :value="selectedTree.nameKey" @change="updateTreeField('nameKey', ($event.target as HTMLInputElement).value)" /></label>
          <label>descKey<input :value="selectedTree.descKey" @change="updateTreeField('descKey', ($event.target as HTMLInputElement).value)" /></label>
          <label>layout<select :value="selectedTree.ui.layout" @change="updateTreeField('ui', { ...selectedTree.ui, layout: ($event.target as HTMLSelectElement).value })"><option value="five_spoke">five_spoke</option></select></label>
          <div class="field-pair">
            <label>解锁等级<input type="number" min="1" :value="selectedTree.unlock.battleLevel" @change="updateNestedField('unlock', 'battleLevel', Number(($event.target as HTMLInputElement).value))" /></label>
            <label>初始点<input type="number" min="0" :value="selectedTree.initialPoints" @change="updateTreeField('initialPoints', Number(($event.target as HTMLInputElement).value))" /></label>
          </div>
          <div class="field-pair">
            <label>点数上限<input type="number" min="0" :value="selectedTree.maxPoints" @change="updateTreeField('maxPoints', Number(($event.target as HTMLInputElement).value))" /></label>
            <label>根路线上限<input type="number" min="1" :value="selectedTree.maxActiveRootNodes" @change="updateTreeField('maxActiveRootNodes', Number(($event.target as HTMLInputElement).value))" /></label>
          </div>
          <div class="field-pair">
            <label>洗点物品<select :value="selectedTree.reset.itemId" @change="updateNestedField('reset', 'itemId', ($event.target as HTMLSelectElement).value)"><option v-for="id in itemIds" :key="id" :value="id">{{ id }}</option></select></label>
            <label>数量<input type="number" min="0" :value="selectedTree.reset.count" @change="updateNestedField('reset', 'count', Number(($event.target as HTMLInputElement).value))" /></label>
          </div>
        </div>

        <details class="tree-json">
          <summary>i18n</summary>
          <SafeJsonEditor label="zh-CN / en-US" expected="object" :model-value="selectedTree.i18n" @focus="beginJsonEdit" @update:model-value="updateTreeJson('i18n', $event)" />
        </details>

        <section class="band-editor">
          <header><strong>点数里程碑</strong><button class="mini-button" @click="addPointBand">+</button></header>
          <div v-for="(band, index) in selectedTree.pointBands" :key="`${band.id}:${index}`" class="band-row">
            <input :value="band.id" aria-label="里程碑 id" @change="updatePointBand(index, 'id', ($event.target as HTMLInputElement).value)" />
            <input type="number" :value="band.fromLevel" aria-label="起始等级" @change="updatePointBand(index, 'fromLevel', Number(($event.target as HTMLInputElement).value))" />
            <span>-</span>
            <input type="number" :value="band.toLevel" aria-label="结束等级" @change="updatePointBand(index, 'toLevel', Number(($event.target as HTMLInputElement).value))" />
            <input type="number" :value="band.everyLevels" aria-label="间隔等级" @change="updatePointBand(index, 'everyLevels', Number(($event.target as HTMLInputElement).value))" />
            <input type="number" :value="band.pointsPerGrant" aria-label="每次点数" @change="updatePointBand(index, 'pointsPerGrant', Number(($event.target as HTMLInputElement).value))" />
            <button class="mini-button danger" title="删除里程碑" @click="removePointBand(index)">×</button>
          </div>
        </section>

        <div class="school-legend">
          <span v-for="school in SPECIALIZATION_SCHOOLS" :key="school"><i :style="{ background: schoolColor(school) }" />{{ SPECIALIZATION_SCHOOL_META[school].label }}</span>
        </div>

        <section class="issue-list">
          <header><strong>校验</strong><span>{{ selectedTreeIssues.length }}</span></header>
          <button v-for="(issue, index) in selectedTreeIssues" :key="`${issue.message}:${index}`" @click="focusIssue(index)">{{ issue.message }}</button>
          <p v-if="selectedTreeIssues.length === 0">通过</p>
        </section>
      </template>
      <div v-else class="empty-state"><button @click="addTree">新增第一套专精树</button></div>
    </aside>

    <main class="graph-panel">
      <header class="graph-toolbar">
        <select v-model="newNodeSchool" aria-label="新节点流派"><option v-for="school in SPECIALIZATION_SCHOOLS" :key="school" :value="school">{{ SPECIALIZATION_SCHOOL_META[school].label }}</option></select>
        <button :disabled="isPersistedTree() && !rulesEditMode" title="新增节点" @click="addNode">+节点</button>
        <span class="toolbar-separator" />
        <button class="icon-button subtle" :disabled="!selectedNode || (isPersistedTree() && !rulesEditMode)" title="复制节点" aria-label="复制节点" @click="duplicateNode">⎘</button>
        <button class="icon-button danger" :disabled="!selectedNode || (isPersistedTree() && !rulesEditMode)" title="删除节点" aria-label="删除节点" @click="deleteNode">×</button>
        <span class="toolbar-separator" />
        <button class="icon-button subtle" :disabled="undoStack.length === 0" title="撤销" aria-label="撤销" @click="undo">↶</button>
        <button class="icon-button subtle" :disabled="redoStack.length === 0" title="重做" aria-label="重做" @click="redo">↷</button>
        <button class="fit-button subtle" title="按五辐结构自动布局" @click="autoLayout">自动布局</button>
        <label class="graph-search"><input v-model="searchQuery" placeholder="搜索节点" @keydown.enter.prevent="focusSearchMatch(1)" /><span>{{ searchMatches.length ? `${searchCursor + 1}/${searchMatches.length}` : '0' }}</span></label>
        <button class="icon-button subtle" :disabled="searchMatches.length === 0" title="上一个搜索结果" aria-label="上一个搜索结果" @click="focusSearchMatch(-1)">‹</button>
        <button class="icon-button subtle" :disabled="searchMatches.length === 0" title="下一个搜索结果" aria-label="下一个搜索结果" @click="focusSearchMatch(1)">›</button>
        <span class="toolbar-spacer" />
        <button class="icon-button subtle" title="缩小" aria-label="缩小" @click="zoomOut()">−</button>
        <button class="icon-button subtle" title="放大" aria-label="放大" @click="zoomIn()">+</button>
        <button class="fit-button subtle" title="适应画布" @click="fitView({ padding: 0.18, duration: 250 })">Fit</button>
      </header>

      <div class="flow-wrap">
        <VueFlow
          :id="FLOW_ID"
          :nodes="flowNodes"
          :edges="flowEdges"
          :min-zoom="0.25"
          :max-zoom="2"
          :default-viewport="{ x: 0, y: 0, zoom: 0.72 }"
          :snap-to-grid="true"
          :snap-grid="[12, 12]"
          :nodes-connectable="true"
          fit-view-on-init
          @node-click="onNodeClick"
          @node-drag-stop="onNodeDragStop"
          @connect="onConnect"
        >
          <template #node-specialization="{ data }">
            <article class="spec-node" :class="{ hybrid: data.schools.length > 1, invalid: data.issueCount > 0 }" :style="nodeStyle(data.schools)">
              <Handle type="target" :position="Position.Top" />
              <div class="spec-node-accent" />
              <strong :title="data.id">{{ data.displayName }}</strong>
              <span>{{ data.schoolText }}</span>
              <footer><small>Lv.{{ data.maxLevel }} / {{ data.totalCost }}pt</small><b v-if="data.issueCount">{{ data.issueCount }}</b></footer>
              <Handle type="source" :position="Position.Bottom" />
            </article>
          </template>
          <MiniMap
            class="specialization-minimap"
            :node-color="miniMapNodeColor"
            node-stroke-color="#26334a"
            :node-stroke-width="1"
            :node-border-radius="5"
            mask-color="rgba(49, 87, 212, 0.12)"
            mask-stroke-color="#3157d4"
            :pannable="true"
            :zoomable="true"
            :width="176"
            :height="124"
            aria-label="专精盘小地图"
          />
        </VueFlow>
      </div>
    </main>

    <aside class="inspector-panel" :class="{ 'rules-locked': isPersistedTree() && !rulesEditMode }">
      <template v-if="selectedNode">
        <header class="inspector-head"><div><h2>{{ selectedNode.id }}</h2><small>{{ selectedNode.kind }}</small></div><span>{{ selectedNode.levels.length }} 级</span></header>
        <nav class="inspector-tabs">
          <button v-for="tab in (['basic', 'requirements', 'levels', 'grants', 'topology'] as SpecializationInspectorTab[])" :key="tab" :class="{ active: inspectorTab === tab }" @click="inspectorTab = tab">{{ { basic: '基础', requirements: '前置', levels: 'Levels', grants: 'Grants', topology: '槽位变换' }[tab] }}</button>
        </nav>

        <section v-if="inspectorTab === 'basic'" class="inspector-body">
          <label>id<input :readonly="isPersistedNode() && !rulesEditMode" :value="selectedNode.id" @change="updateNodeField('id', ($event.target as HTMLInputElement).value)" /></label>
          <label>kind<select :value="selectedNode.kind" @change="updateNodeField('kind', ($event.target as HTMLSelectElement).value)"><option v-for="kind in SPECIALIZATION_NODE_KINDS" :key="kind">{{ kind }}</option></select></label>
          <label>nameKey<input :value="selectedNode.nameKey" @change="updateNodeField('nameKey', ($event.target as HTMLInputElement).value)" /></label>
          <label>descKey<input :value="selectedNode.descKey" @change="updateNodeField('descKey', ($event.target as HTMLInputElement).value)" /></label>
          <fieldset v-for="localeId in nodeLocaleIds" :key="localeId" class="node-locale-editor">
            <legend>{{ localeId }} 玩家文案</legend>
            <label>名称<input :value="asString(nodeLocaleEntry(localeId).name)" @change="updateNodeLocaleField(localeId, 'name', ($event.target as HTMLInputElement).value)" /></label>
            <label>风味<textarea rows="2" :value="asString(nodeLocaleEntry(localeId).desc)" @change="updateNodeLocaleField(localeId, 'desc', ($event.target as HTMLTextAreaElement).value)" /></label>
            <div class="summary-editor">
              <header><strong>效果摘要</strong><button class="mini-button" :disabled="nodeEffectSummary(localeId).length >= 7" :title="`添加 ${localeId} 效果行`" :aria-label="`添加 ${localeId} 效果行`" @click="addEffectSummaryLine(localeId)">+</button></header>
              <div v-for="(line, lineIndex) in nodeEffectSummary(localeId)" :key="lineIndex" class="summary-line">
                <textarea rows="2" maxlength="180" :value="line" @change="updateEffectSummaryLine(localeId, lineIndex, ($event.target as HTMLTextAreaElement).value)" />
                <div class="summary-line-actions">
                  <button class="mini-button" :disabled="lineIndex === 0" title="上移效果行" aria-label="上移效果行" @click="moveEffectSummaryLine(localeId, lineIndex, -1)">↑</button>
                  <button class="mini-button" :disabled="lineIndex === nodeEffectSummary(localeId).length - 1" title="下移效果行" aria-label="下移效果行" @click="moveEffectSummaryLine(localeId, lineIndex, 1)">↓</button>
                  <button class="mini-button danger" :disabled="nodeEffectSummary(localeId).length <= 1" title="删除效果行" aria-label="删除效果行" @click="removeEffectSummaryLine(localeId, lineIndex)">×</button>
                </div>
              </div>
            </div>
          </fieldset>
          <label>exclusiveGroupId<input :value="selectedNode.exclusiveGroupId" @change="updateNodeField('exclusiveGroupId', ($event.target as HTMLInputElement).value)" /></label>
          <fieldset><legend>流派（1~2）</legend><label v-for="school in SPECIALIZATION_SCHOOLS" :key="school" class="check-row"><input type="checkbox" :checked="selectedNode.schoolIds.includes(school)" @change="toggleSchool(school, ($event.target as HTMLInputElement).checked)" /><i :style="{ background: schoolColor(school) }" />{{ SPECIALIZATION_SCHOOL_META[school].label }}</label></fieldset>
          <div class="field-pair"><label>column<input type="number" :value="selectedNode.ui.column" @change="updateNodeUI('column', Number(($event.target as HTMLInputElement).value))" /></label><label>row<input type="number" :value="selectedNode.ui.row" @change="updateNodeUI('row', Number(($event.target as HTMLInputElement).value))" /></label></div>
          <div class="field-pair"><label>x<input type="number" :value="selectedNode.ui.x" @change="updateNodeUI('x', Number(($event.target as HTMLInputElement).value))" /></label><label>y<input type="number" :value="selectedNode.ui.y" @change="updateNodeUI('y', Number(($event.target as HTMLInputElement).value))" /></label></div>
          <div class="field-pair"><label>branch<input :value="selectedNode.ui.branch" @change="updateNodeUI('branch', ($event.target as HTMLInputElement).value)" /></label><label>accent<input type="color" :value="selectedNode.ui.accent || '#50607d'" @change="updateNodeUI('accent', ($event.target as HTMLInputElement).value)" /></label></div>
        </section>

        <section v-else-if="inspectorTab === 'requirements'" class="inspector-body requirements-editor">
          <div class="inline-add"><select v-model="requirementKindToAdd"><option v-for="kind in SPECIALIZATION_REQUIREMENT_KINDS" :key="kind">{{ kind }}</option></select><button @click="addRequirement('all')">+ All</button><button class="secondary" @click="addAnyOfGroup">+ AnyOf</button></div>
          <section class="rule-group"><header><strong>All</strong><span>{{ selectedNode.requirements.all.length }}</span></header><div v-for="(requirement, index) in selectedNode.requirements.all" :key="index" class="requirement-row"><select :value="requirement.kind" @change="updateRequirement('all', index, 'kind', ($event.target as HTMLSelectElement).value)"><option v-for="kind in SPECIALIZATION_REQUIREMENT_KINDS" :key="kind">{{ kind }}</option></select><select v-if="requirement.kind === 'node_rank'" :value="requirement.nodeId" @change="updateRequirement('all', index, 'nodeId', ($event.target as HTMLSelectElement).value)"><option value="">选择节点</option><option v-for="node in selectedTree?.nodes.filter((node) => node.id !== selectedNode?.id)" :key="node.id">{{ node.id }}</option></select><select v-else-if="requirement.kind === 'attribute_gte'" :value="requirement.attributeKey" @change="updateRequirement('all', index, 'attributeKey', ($event.target as HTMLSelectElement).value)"><option v-for="id in SPECIALIZATION_SCHOOLS" :key="id">{{ id }}</option></select><select v-else :value="requirement.schoolId" @change="updateRequirement('all', index, 'schoolId', ($event.target as HTMLSelectElement).value)"><option v-for="id in SPECIALIZATION_SCHOOLS" :key="id">{{ id }}</option></select><input type="number" :value="requirement.kind === 'node_rank' ? requirement.minRank : requirement.kind === 'attribute_gte' ? requirement.value : requirement.points" @change="updateRequirement('all', index, requirement.kind === 'node_rank' ? 'minRank' : requirement.kind === 'attribute_gte' ? 'value' : 'points', Number(($event.target as HTMLInputElement).value))" /><button class="mini-button danger" @click="removeRequirement('all', index)">×</button></div></section>
          <section v-for="(group, groupIndex) in selectedNode.requirements.anyOf" :key="groupIndex" class="rule-group"><header><strong>AnyOf {{ groupIndex + 1 }}</strong><div><button class="mini-button" @click="addRequirement('anyOf', groupIndex)">+</button><button class="mini-button danger" @click="removeAnyOfGroup(groupIndex)">×</button></div></header><div v-for="(requirement, index) in group" :key="index" class="requirement-row"><select :value="requirement.kind" @change="updateRequirement('anyOf', index, 'kind', ($event.target as HTMLSelectElement).value, groupIndex)"><option v-for="kind in SPECIALIZATION_REQUIREMENT_KINDS" :key="kind">{{ kind }}</option></select><select v-if="requirement.kind === 'node_rank'" :value="requirement.nodeId" @change="updateRequirement('anyOf', index, 'nodeId', ($event.target as HTMLSelectElement).value, groupIndex)"><option value="">选择节点</option><option v-for="node in selectedTree?.nodes.filter((node) => node.id !== selectedNode?.id)" :key="node.id">{{ node.id }}</option></select><select v-else-if="requirement.kind === 'attribute_gte'" :value="requirement.attributeKey" @change="updateRequirement('anyOf', index, 'attributeKey', ($event.target as HTMLSelectElement).value, groupIndex)"><option v-for="id in SPECIALIZATION_SCHOOLS" :key="id">{{ id }}</option></select><select v-else :value="requirement.schoolId" @change="updateRequirement('anyOf', index, 'schoolId', ($event.target as HTMLSelectElement).value, groupIndex)"><option v-for="id in SPECIALIZATION_SCHOOLS" :key="id">{{ id }}</option></select><input type="number" :value="requirement.kind === 'node_rank' ? requirement.minRank : requirement.kind === 'attribute_gte' ? requirement.value : requirement.points" @change="updateRequirement('anyOf', index, requirement.kind === 'node_rank' ? 'minRank' : requirement.kind === 'attribute_gte' ? 'value' : 'points', Number(($event.target as HTMLInputElement).value), groupIndex)" /><button class="mini-button danger" @click="removeRequirement('anyOf', index, groupIndex)">×</button></div></section>
        </section>

        <section v-else-if="inspectorTab === 'levels'" class="inspector-body">
          <div class="section-bar"><strong>Levels</strong><button class="mini-button" :disabled="selectedNode.levels.length >= maximumRankCount(selectedNode) || (isPersistedNode() && !rulesEditMode)" title="新增等级" aria-label="新增等级" @click="addLevel">+</button></div>
          <div v-for="(level, index) in selectedNode.levels" :key="level.level" class="level-row" :class="{ selected: selectedLevelIndex === index }" @click="selectedLevelIndex = index">
            <b>Lv.{{ level.level }}</b>
            <label>点数<input type="number" min="1" max="10" :readonly="isPersistedNode() && !rulesEditMode" :value="level.cost" @click.stop @change="updateLevel(index, 'cost', Number(($event.target as HTMLInputElement).value))" /></label>
            <span>{{ level.grants.length }} grants</span>
            <button class="mini-button danger" :disabled="index === 0 || (isPersistedNode() && !rulesEditMode)" title="删除等级" aria-label="删除等级" @click.stop="removeLevel(index)">×</button>
          </div>
          <fieldset v-for="localeId in nodeLocaleIds" :key="localeId" class="node-locale-editor">
            <legend>{{ localeId }} · Lv.{{ selectedLevelIndex + 1 }} 效果文案</legend>
            <div class="summary-editor">
              <header><strong>本级实际收益</strong><button class="mini-button" :disabled="nodeLevelEffectSummary(localeId).length >= 4" :title="`添加 ${localeId} 本级效果行`" :aria-label="`添加 ${localeId} 本级效果行`" @click="addLevelEffectSummaryLine(localeId, selectedLevelIndex)">+</button></header>
              <div v-for="(line, lineIndex) in nodeLevelEffectSummary(localeId)" :key="lineIndex" class="summary-line">
                <textarea rows="2" maxlength="180" :value="line" @change="updateLevelEffectSummaryLine(localeId, selectedLevelIndex, lineIndex, ($event.target as HTMLTextAreaElement).value)" />
                <button class="mini-button danger" :disabled="nodeLevelEffectSummary(localeId).length <= 1" title="删除效果行" aria-label="删除效果行" @click="removeLevelEffectSummaryLine(localeId, selectedLevelIndex, lineIndex)">×</button>
              </div>
            </div>
          </fieldset>
        </section>

        <section v-else-if="inspectorTab === 'grants'" class="inspector-body grants-editor">
          <div class="level-select"><label>等级<select v-model.number="selectedLevelIndex"><option v-for="(level, index) in selectedNode.levels" :key="level.level" :value="index">Lv.{{ level.level }}</option></select></label><div class="inline-add"><select v-model="grantKindToAdd"><option v-for="kind in SPECIALIZATION_GRANT_KINDS.filter((kind) => kind !== 'equipment_slot_capacity')" :key="kind">{{ kind }}</option></select><button @click="addGrant()">+</button></div></div>
          <section v-for="entry in regularGrantEntries()" :key="entry.index" class="grant-block"><header><strong>{{ entry.grant.kind }}</strong><button class="mini-button danger" @click="removeGrant(entry.index)">×</button></header><label>id<input :readonly="isPersistedNode() && !rulesEditMode" :value="entry.grant.id" @change="updateGrant(entry.index, 'id', ($event.target as HTMLInputElement).value)" /></label><template v-if="entry.grant.kind === 'stat_modifier'"><label>scope<select :value="entry.grant.scope" @change="updateGrant(entry.index, 'scope', ($event.target as HTMLSelectElement).value)"><option value="character">character</option><option value="battle">battle</option></select></label><div class="section-bar"><strong>modifiers</strong><button class="mini-button" @click="addModifier(entry.index)">+</button></div><div v-for="(modifier, modifierIndex) in asArray(entry.grant.modifiers).filter(isRecord)" :key="modifierIndex" class="modifier-row"><select :value="modifier.attributeKey" @change="updateModifier(entry.index, modifierIndex, 'attributeKey', ($event.target as HTMLSelectElement).value)"><option v-for="id in attributeIds" :key="id">{{ id }}</option></select><select :value="modifier.operation" @change="updateModifier(entry.index, modifierIndex, 'operation', ($event.target as HTMLSelectElement).value)"><option>add</option><option>multiply</option></select><input type="number" step="any" :value="modifier.value" @change="updateModifier(entry.index, modifierIndex, 'value', Number(($event.target as HTMLInputElement).value))" /><button class="mini-button danger" @click="removeModifier(entry.index, modifierIndex)">×</button></div></template><SpecializationBattleHooksEditor v-else-if="entry.grant.kind === 'battle_hook'" :model-value="asArray(entry.grant.battleHooks)" :attribute-ids="attributeIds" :skill-ids="skillIds" :skill-categories="skillCategories" :buff-ids="buffIds" :buff-kinds="buffKinds" :lock-ids="isPersistedNode() && !rulesEditMode" @update:model-value="updateGrant(entry.index, 'battleHooks', $event)" /><SpecializationEffectMutationsEditor v-else-if="entry.grant.kind === 'effect_mutation'" :model-value="asArray(entry.grant.effectMutations)" :attribute-ids="attributeIds" :skill-ids="skillIds" :skill-categories="skillCategories" :weapon-classes="weaponClasses" :buff-ids="buffIds" :effect-ids="effectIds" :skill-effect-kinds="skillEffectKinds" :lock-ids="isPersistedNode() && !rulesEditMode" @update:model-value="updateGrant(entry.index, 'effectMutations', $event)" /></section>
          <section class="effect-preview"><header><strong>机械效果预览</strong><span>Lv.{{ selectedLevelIndex }} → Lv.{{ selectedLevelIndex + 1 }}</span></header><p v-if="mechanicalPreviewLines.length === 0">该等级尚未配置可执行效果。</p><ul v-else><li v-for="(line, index) in mechanicalPreviewLines" :key="`${line}:${index}`">{{ line }}</li></ul></section>
        </section>

        <section v-else class="inspector-body topology-editor">
          <div class="level-select"><label>等级<select v-model.number="selectedLevelIndex"><option v-for="(level, index) in selectedNode.levels" :key="level.level" :value="index">Lv.{{ level.level }}</option></select></label><button @click="addGrant('equipment_slot_capacity')">+槽位容量</button></div>
          <section v-for="entry in topologyGrantEntries()" :key="entry.index" class="grant-block"><header><strong>equipment_slot_capacity</strong><button class="mini-button danger" @click="removeGrant(entry.index)">×</button></header><label>id<input :readonly="isPersistedNode() && !rulesEditMode" :value="entry.grant.id" @change="updateGrant(entry.index, 'id', ($event.target as HTMLInputElement).value)" /></label><label>equipType<select :value="entry.grant.equipType" @change="updateGrant(entry.index, 'equipType', ($event.target as HTMLSelectElement).value)"><option value="battle">battle</option></select></label><div class="slot-deltas"><label v-for="family in SPECIALIZATION_SLOT_FAMILIES" :key="family"><span>{{ family }}</span><input type="number" step="1" :value="isRecord(entry.grant.deltas) ? Number(entry.grant.deltas[family] ?? 0) : 0" @change="updateSlotDelta(entry.index, family, Number(($event.target as HTMLInputElement).value))" /></label></div></section>
          <section class="topology-preview"><header><strong>槽位前后拓扑</strong><span>基础容量 → 学习后</span></header><p v-if="topologyPreview.length === 0">该等级没有槽位交换。</p><div v-for="entry in topologyPreview" :key="entry.family" class="topology-row" :class="{ invalid: !entry.valid }"><b>{{ entry.label }}</b><span>{{ entry.base }} {{ entry.delta > 0 ? '+' : '' }}{{ entry.delta }} = {{ entry.after }}</span><small>{{ concreteSlots(entry.family, entry.base) }} → {{ concreteSlots(entry.family, entry.after) }}</small></div></section>
        </section>
      </template>
      <div v-else class="empty-state"><p>选择或新增节点</p></div>
    </aside>
  </section>
</template>

<style scoped>
.specialization-designer { height: calc(100vh - 36px); min-height: 720px; display: grid; grid-template-columns: 248px minmax(520px, 1fr) 390px; border: 1px solid #dfe6f1; border-radius: 8px; overflow: hidden; background: #fff; color: #172033; }
.tree-panel, .inspector-panel { min-width: 0; overflow: auto; background: #fff; }
.tree-panel { padding: 12px; border-right: 1px solid #dfe6f1; }
.inspector-panel { border-left: 1px solid #dfe6f1; }
.panel-head, .inspector-head, .section-bar, .rule-group > header, .grant-block > header, .band-editor > header, .issue-list > header { display: flex; align-items: center; justify-content: space-between; gap: 8px; }
h2 { margin: 0; font-size: 16px; letter-spacing: 0; }
small { color: #758198; font-size: 11px; }
.icon-button, .mini-button { display: inline-grid; place-items: center; flex: 0 0 auto; width: 32px; height: 32px; padding: 0; border-radius: 6px; }
.mini-button { width: 26px; height: 26px; font-size: 13px; }
button.subtle, .mini-button { background: #eef2f7; color: #3f4d66; }
button.danger { background: #fff0f1; color: #b42332; }
button:disabled { cursor: not-allowed; opacity: .42; }
.tree-switcher { display: grid; grid-template-columns: minmax(0, 1fr) auto auto; gap: 6px; margin-top: 12px; }
.tree-fields, .inspector-body { display: grid; gap: 10px; }
.node-locale-editor { display: grid; gap: 8px; padding: 10px; border: 1px solid #dfe6f1; border-radius: 6px; }
.node-locale-editor legend { padding: 0 5px; color: #3e4b61; font-size: 11px; font-weight: 700; }
.summary-editor { display: grid; gap: 7px; }
.summary-editor > header { display: flex; align-items: center; justify-content: space-between; gap: 8px; }
.summary-line { display: grid; grid-template-columns: minmax(0, 1fr) auto; gap: 6px; align-items: start; }
.summary-line textarea { min-width: 0; resize: vertical; }
.summary-line-actions { display: grid; gap: 4px; }
.tree-fields { margin-top: 12px; }
label { display: grid; gap: 5px; min-width: 0; color: #526079; font-size: 11px; font-weight: 750; }
input, select, textarea { border-radius: 6px; padding: 7px 8px; }
.field-pair { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 8px; }
.band-editor, .issue-list { display: grid; gap: 7px; margin-top: 14px; padding-top: 12px; border-top: 1px solid #e7ebf2; }
.tree-json { margin-top: 12px; padding-top: 10px; border-top: 1px solid #e7ebf2; }
.tree-json summary { margin-bottom: 8px; color: #526079; font-size: 11px; font-weight: 800; cursor: pointer; }
.band-row { display: grid; grid-template-columns: minmax(0, 1fr) 34px 8px 34px 34px 34px 26px; gap: 3px; align-items: center; }
.band-row input { padding: 5px 4px; font-size: 10px; }
.band-row span { color: #8792a6; font-size: 10px; }
.school-legend { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 6px; margin-top: 14px; }
.school-legend span, .check-row { display: flex; align-items: center; gap: 5px; color: #536078; font-size: 11px; }
.school-legend i, .check-row i { width: 9px; height: 9px; border-radius: 2px; flex: 0 0 auto; }
.issue-list button { padding: 7px; border: 1px solid #f0c8cd; background: #fff8f8; color: #982536; font-size: 10px; line-height: 1.35; text-align: left; font-weight: 700; }
.issue-list p { margin: 0; padding: 8px; color: #277052; background: #f0faf5; font-size: 11px; }
.empty-state { min-height: 180px; display: grid; place-items: center; color: #758198; }
.graph-panel { min-width: 0; display: grid; grid-template-rows: auto minmax(0, 1fr); background: #f5f7fb; }
.graph-toolbar { min-height: 48px; display: flex; flex-wrap: wrap; align-items: center; gap: 6px; padding: 7px 10px; border-bottom: 1px solid #dfe6f1; background: #fff; }
.graph-toolbar select { width: 112px; }
.toolbar-separator { width: 1px; height: 24px; margin: 0 2px; background: #dfe6f1; }
.toolbar-spacer { flex: 1; }
.fit-button { height: 32px; padding: 0 10px; border-radius: 6px; }
.graph-search { width: 172px; height: 32px; display: grid; grid-template-columns: minmax(0, 1fr) 36px; gap: 0; align-items: stretch; }
.graph-search input { min-width: 0; padding: 6px 7px; border-radius: 6px 0 0 6px; }
.graph-search span { display: grid; place-items: center; border: 1px solid #ccd5e2; border-left: 0; border-radius: 0 6px 6px 0; color: #637087; background: #f7f9fc; font-size: 9px; font-weight: 700; }
.flow-wrap { min-height: 0; background-color: #f5f7fb; background-image: radial-gradient(#cdd5e2 1px, transparent 1px); background-size: 20px 20px; }
.vue-flow { width: 100%; height: 100%; }
:deep(.specialization-minimap) { right: 12px; bottom: 12px; overflow: hidden; border: 1px solid #aebbd0; border-radius: 6px; background: rgba(255, 255, 255, .94); box-shadow: 0 4px 12px rgba(28, 40, 64, .14); }
.spec-node { position: relative; width: 154px; min-height: 80px; padding: 11px 10px 8px; display: grid; gap: 4px; overflow: hidden; border: 2px solid color-mix(in srgb, var(--spec-node-accent) 76%, #172033); border-radius: 7px; background: #fff; box-shadow: 0 5px 14px rgba(29, 39, 57, .12); }
.spec-node-accent { position: absolute; inset: 0 auto 0 0; width: 7px; background: var(--spec-node-fill); }
.spec-node.hybrid { border-style: double; border-width: 4px; }
.spec-node.invalid { box-shadow: 0 0 0 3px rgba(180, 35, 48, .18), 0 5px 14px rgba(29, 39, 57, .12); }
.spec-node strong { padding-left: 4px; overflow-wrap: anywhere; font-size: 12px; line-height: 1.25; letter-spacing: 0; }
.spec-node > span { padding-left: 4px; color: #5e6b81; font-size: 10px; }
.spec-node footer { padding-left: 4px; display: flex; justify-content: space-between; align-items: center; }
.spec-node footer b { display: grid; place-items: center; min-width: 20px; height: 20px; border-radius: 50%; background: #b42332; color: #fff; font-size: 10px; }
.inspector-head { min-height: 62px; padding: 12px; border-bottom: 1px solid #e4e9f1; }
.inspector-head h2 { overflow-wrap: anywhere; }
.inspector-head > span { color: #526079; font-size: 11px; white-space: nowrap; }
.inspector-tabs { position: sticky; top: 0; z-index: 4; display: grid; grid-template-columns: repeat(5, minmax(0, 1fr)); background: #fff; border-bottom: 1px solid #e4e9f1; }
.inspector-tabs button { min-width: 0; padding: 9px 3px; border-radius: 0; border-bottom: 2px solid transparent; background: transparent; color: #68758b; font-size: 10px; }
.inspector-tabs button.active { border-bottom-color: #3157d4; color: #3157d4; background: #f5f7ff; }
.inspector-body { padding: 12px; }
fieldset { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 7px; margin: 0; padding: 9px; border: 1px solid #dfe6f1; border-radius: 6px; }
legend { color: #526079; font-size: 11px; font-weight: 750; }
.check-row input { width: auto; }
.inline-add, .level-select { display: flex; align-items: end; gap: 6px; }
.inline-add select { min-width: 0; }
.rule-group, .grant-block { display: grid; gap: 8px; padding: 9px 0; border-top: 1px solid #e5eaf2; }
.rule-group > header div { display: flex; gap: 4px; }
.requirement-row { display: grid; grid-template-columns: 105px minmax(0, 1fr) 58px 26px; gap: 5px; align-items: center; }
.requirement-row input, .requirement-row select { min-width: 0; font-size: 10px; padding: 6px 5px; }
.level-row { display: grid; grid-template-columns: 46px 92px 1fr 26px; gap: 7px; align-items: center; padding: 8px; border: 1px solid #dfe6f1; border-radius: 6px; cursor: pointer; }
.level-row.selected { border-color: #6580dc; background: #f3f5ff; }
.level-row label { grid-template-columns: 28px 1fr; align-items: center; }
.level-row input { padding: 5px; }
.level-row span { color: #758198; font-size: 10px; }
.level-select { justify-content: space-between; padding-bottom: 4px; }
.level-select > label { width: 120px; }
.grant-block > header strong { overflow-wrap: anywhere; font-size: 12px; }
.modifier-row { display: grid; grid-template-columns: minmax(0, 1fr) 82px 70px 26px; gap: 5px; }
.modifier-row > * { min-width: 0; font-size: 10px; }
.slot-deltas { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 7px; }
.slot-deltas label { grid-template-columns: minmax(0, 1fr) 64px; align-items: center; }
.slot-deltas span { overflow: hidden; text-overflow: ellipsis; }
.rules-edit-toggle { display: flex; align-items: center; gap: 7px; padding: 7px; border: 1px solid #dfe6f1; border-radius: 6px; background: #f7f9fc; }
.rules-edit-toggle input { width: auto; }
.rules-edit-warning { padding: 7px; border-left: 3px solid #b87724; color: #815316; background: #fff8e8; line-height: 1.4; }
.tree-panel.rules-locked .tree-fields > :not(.rules-edit-toggle):not(.rules-edit-warning):not(.balance-revision-field), .tree-panel.rules-locked .band-editor { pointer-events: none; opacity: .58; }
.inspector-panel.rules-locked .inspector-body :is(input, select, button, textarea) { pointer-events: none; opacity: .58; }
input[readonly] { color: #68758b; background: #f0f3f7; cursor: not-allowed; }
.effect-preview, .topology-preview { display: grid; gap: 8px; margin-top: 4px; padding: 10px; border: 1px solid #cbd7e8; border-radius: 6px; background: #f5f8fc; }
.effect-preview > header, .topology-preview > header { display: flex; align-items: center; justify-content: space-between; gap: 8px; }
.effect-preview > header span, .topology-preview > header span { color: #6e7b91; font-size: 9px; }
.effect-preview p, .topology-preview p { margin: 0; color: #6e7b91; font-size: 10px; }
.effect-preview ul { display: grid; gap: 6px; margin: 0; padding-left: 18px; color: #344158; font-size: 10px; line-height: 1.45; }
.topology-row { display: grid; grid-template-columns: 44px 92px minmax(0, 1fr); gap: 7px; align-items: center; padding: 7px; border: 1px solid #dce4ef; border-radius: 5px; background: #fff; }
.topology-row span { color: #344158; font-size: 10px; font-variant-numeric: tabular-nums; }
.topology-row small { overflow-wrap: anywhere; font-size: 9px; }
.topology-row.invalid { border-color: #e0a6ad; background: #fff5f6; }
@media (max-width: 1320px) { .specialization-designer { grid-template-columns: 220px minmax(480px, 1fr) 350px; } .graph-search { width: 150px; } }
@media (max-width: 1080px) {
  .specialization-designer { height: auto; min-height: 0; grid-template-columns: minmax(0, 1fr); grid-template-rows: auto minmax(620px, 72vh) auto; overflow: visible; }
  .tree-panel, .inspector-panel { max-height: none; border: 0; }
  .tree-panel { border-bottom: 1px solid #dfe6f1; }
  .inspector-panel { border-top: 1px solid #dfe6f1; }
  .graph-toolbar { position: sticky; top: 0; z-index: 8; }
  :deep(.specialization-minimap) { width: 148px !important; height: 104px !important; left: 8px; right: auto; top: 8px; bottom: auto; }
}
</style>
