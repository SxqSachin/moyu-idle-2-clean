<template>
  <section v-if="views.length" class="equipment-skills" data-testid="equipment-skills">
    <div class="equipment-skills__title">{{ copy.title }}</div>
    <div v-for="view in views" :key="view.skillId" class="equipment-skills__row">
      <div class="equipment-skills__identity">
        <SkillQuickRef :skill-id="view.skillId" :show-icon="true" />
        <span class="text-xs app-muted">{{ itemName(view.itemId) }} +{{ view.enhanceLevel }}</span>
      </div>
      <div class="equipment-skills__summary">
        <span>{{ view.config.skillIds.map(skillName).join(' → ') || copy.notConfigured }}</span>
        <span class="text-xs app-muted">{{ view.config.skillIds.length }} / {{ view.capacity }}</span>
        <span v-if="reasonFor(view)" class="equipment-skills__warning">{{ reasonText(reasonFor(view)) }}</span>
        <span v-else class="text-xs app-muted">{{ view.config.enabled ? copy.active : copy.inactive }}</span>
      </div>
      <el-tooltip :content="copy.advanced">
        <el-button circle :aria-label="copy.advanced" data-testid="equipment-skill-settings" @click="open(view)"><Icon icon="lucide:settings-2" /></el-button>
      </el-tooltip>
    </div>
    <GameDialog v-model="visible" :title="`${skillName(selected?.skillId || '')} · ${copy.advanced}`" width="680px" :close-on-click-modal="!saving" :close-on-press-escape="!saving" :show-close="!saving">
      <template v-if="selected">
        <div class="equipment-settings__header">
          <el-switch v-model="draft.enabled" :active-text="copy.active" />
          <span>{{ draft.skillIds.length }} / {{ selected.capacity }}</span>
        </div>
        <p v-if="draftReason" class="equipment-skills__warning" role="status">{{ reasonText(draftReason) }}</p>
        <draggable v-model="draft.skillIds" :item-key="(id: string) => id" handle=".combo-drag" :disabled="!isCombo || saving" class="equipment-settings__selected">
          <template #item="{ element: id, index }">
            <div class="equipment-settings__chosen">
              <Icon v-if="isCombo" class="combo-drag" icon="lucide:grip-vertical" />
              <span v-if="isCombo" class="equipment-settings__ordinal">{{ index + 1 }}</span>
              <div class="equipment-settings__skill">
                <SkillQuickRef :skill-id="id" :show-icon="true" :unit-skill-level="level(id)" />
                <span v-if="isCombo" class="text-xs app-muted">Lv.{{ level(id) }} · HP {{ row(id)?.hpCost || 0 }} · MP {{ row(id)?.mpCost || 0 }} · {{ copy.cooldown }} {{ row(id)?.cooldown || 0 }}</span>
                <span v-if="!candidateIDs.has(id) || (isCombo && !carriedIDs.has(id))" class="equipment-skills__warning">{{ copy.unavailable }}</span>
              </div>
              <template v-if="isCombo">
                <el-button circle :disabled="index === 0 || saving" :aria-label="copy.up" :title="copy.up" @click="draft.skillIds = moveEquipmentComboSkill(draft.skillIds, index, -1)"><Icon icon="lucide:arrow-up" /></el-button>
                <el-button circle :disabled="index === draft.skillIds.length - 1 || saving" :aria-label="copy.down" :title="copy.down" @click="draft.skillIds = moveEquipmentComboSkill(draft.skillIds, index, 1)"><Icon icon="lucide:arrow-down" /></el-button>
              </template>
              <el-button circle :disabled="saving" :aria-label="copy.remove" :title="copy.remove" @click="draft.skillIds = draft.skillIds.filter(value => value !== id)"><Icon icon="lucide:x" /></el-button>
            </div>
          </template>
        </draggable>
        <div class="equipment-settings__filters">
          <el-input v-model="search" clearable :placeholder="copy.search" :aria-label="copy.search" />
          <el-select v-model="category" :empty-values="[null, undefined]" :aria-label="copy.category"><el-option :label="copy.all" value="" /><el-option v-for="id in categories" :key="id" :value="id" :label="categoryName(id)" /></el-select>
        </div>
        <div class="equipment-settings__candidates">
          <label v-for="skill in filtered" :key="skill.id" class="equipment-settings__candidate">
            <el-checkbox :model-value="draft.skillIds.includes(skill.id)" :disabled="saving || (!draft.skillIds.includes(skill.id) && (draft.skillIds.length >= selected.capacity || (isCombo && !carriedIDs.has(skill.id))))" @change="toggle(skill.id)" :aria-label="skillName(skill.id)" />
            <div class="equipment-settings__skill"><SkillQuickRef :skill-id="skill.id" :show-icon="true" /><span v-if="isCombo && !carriedIDs.has(skill.id)" class="text-xs app-muted">{{ copy.notCarried }}</span></div>
          </label>
          <div v-if="!filtered.length" class="text-sm app-muted">{{ copy.noMatches }}</div>
        </div>
        <template v-if="isCombo">
          <div class="equipment-settings__totals">HP {{ totalHP }} · MP {{ totalMP }} · {{ copy.cooldown }} 6</div>
          <el-checkbox v-model="draft.allowStandalone">{{ copy.standalone }}</el-checkbox>
          <div class="equipment-settings__priority">
            <span>{{ copy.priority }}</span>
            <el-select v-model="draft.beforeSkillId" :empty-values="[null, undefined]" :aria-label="copy.priority"><el-option value="" :label="copy.first" /><el-option v-for="id in carried" :key="id" :value="id" :label="`${copy.before} ${skillName(id)}`" /></el-select>
          </div>
          <el-collapse><el-collapse-item :title="copy.rules" name="rules"><RuleLogicEditor v-model="groups" :max-groups="6" :max-conditions="6" :skill-options="ruleOptions" :item-options="itemOptions" @validation="ruleValid = $event.valid" /></el-collapse-item></el-collapse>
        </template>
        <el-alert v-if="error" type="error" :closable="false" :title="error" class="mt-3" />
      </template>
      <template #footer><el-button :disabled="saving" @click="visible = false">{{ t('game.cancel') }}</el-button><el-button type="primary" :loading="saving" :disabled="Boolean(draftReason) || !ruleValid" @click="save">{{ t('game.saveConfig') }}</el-button></template>
    </GameDialog>
  </section>
</template>

<script setup lang="ts">
import { computed, ref, toRaw } from "vue"
import { useI18n } from "vue-i18n"
import { Icon } from "@iconify/vue"
import draggable from "vuedraggable"
import GameDialog from "@components/game/common/GameDialog.vue"
import SkillQuickRef from "./SkillQuickRef.vue"
import RuleLogicEditor from "@components/game/rules/RuleLogicEditor.vue"
import { apiPost } from "@services/api"
import { resolveErrorMessage } from "@services/error-messages"
import { getLocalizedSkillName, getSkillTypeDefinition } from "@services/skill-definition"
import { getLocalizedItemName } from "@services/item-label"
import { normalizeRuleGroups, type RuleGroup } from "@services/rule-logic"
import { emptyEquipmentSkillConfig, cloneEquipmentSkillConfig, moveEquipmentComboSkill, equipmentSelectionReason, equipmentCarriedSkillIDs, equipmentComboPriorityAnchor, type EquipmentSkillConfig, type EquipmentSkillView } from "@services/equipment-skill-config"

type Candidate = { id: string; category?: string; hpCost: number; mpCost: number; cooldown: number; eligible?: boolean; comboEligible?: boolean; dominationEligible?: boolean }
const props = defineProps<{ token: string; views: EquipmentSkillView[]; catalog: Candidate[]; carried: string[]; owned: { skillId: string; level?: number }[]; draftMode?: boolean; itemOptions: { value: string; label: string }[] }>()
const emit = defineEmits<{ saved: [skillId: string, config: EquipmentSkillConfig] }>()
const { t, locale } = useI18n()
const copy = computed(() => locale.value.startsWith("zh") ? {
  title: "装备技能", advanced: "高级设置", active: "启用", inactive: "未启用", notConfigured: "未配置", cooldown: "冷却", unavailable: "该技能当前不可用，请重新选择", up: "上移", down: "下移", remove: "移除", search: "搜索技能", category: "技能分类", all: "全部分类", notCarried: "未携带或条件不符", noMatches: "没有匹配的技能", standalone: "组合成员也可单独施放", priority: "施放优先级", first: "优先施放", before: "早于", rules: "施放规则", invalid: "配置无效，请重新选择", capacity: "超过当前强化容量，请移除多余技能", minimum: "请选择足够数量的技能", changed: "配置已在其他页面更新，请重新打开设置", failed: "保存失败，请检查装备与已携带技能"
} : {
  title: "Equipment skills", advanced: "Advanced settings", active: "Enabled", inactive: "Disabled", notConfigured: "Not configured", cooldown: "Cooldown", unavailable: "Skill unavailable. Choose another skill.", up: "Move up", down: "Move down", remove: "Remove", search: "Search skills", category: "Skill category", all: "All categories", notCarried: "Not carried or requirements unmet", noMatches: "No matching skills", standalone: "Also cast components individually", priority: "Cast priority", first: "Cast first", before: "Before", rules: "Cast rules", invalid: "Invalid configuration. Select skills again.", capacity: "Over enhancement capacity. Remove excess skills.", minimum: "Select enough skills", changed: "Configuration changed elsewhere. Reopen settings.", failed: "Save failed. Check equipment and carried skills."
})
const visible = ref(false), saving = ref(false), error = ref(""), search = ref(""), category = ref(""), ruleValid = ref(true)
const selected = ref<EquipmentSkillView | null>(null)
const draft = ref<EquipmentSkillConfig>(emptyEquipmentSkillConfig())
const isCombo = computed(() => selected.value?.skillId === "equipment_martial_combo")
const catalogByID = computed(() => new Map(props.catalog.map(row => [row.id, row])))
const ownedByID = computed(() => new Map(props.owned.map(row => [row.skillId, row])))
const carriedIDs = computed(() => equipmentCarriedSkillIDs(props.carried, props.catalog, props.draftMode))
const candidateIDs = computed(() => new Set(props.catalog.filter(row => isCombo.value ? row.comboEligible : row.dominationEligible).map(row => row.id)))
const categories = computed(() => [...new Set(props.catalog.filter(row => candidateIDs.value.has(row.id)).map(row => row.category || ""))].filter(Boolean))
const filtered = computed(() => props.catalog.filter(row => candidateIDs.value.has(row.id) && (!isCombo.value || ownedByID.value.has(row.id)) && (!category.value || row.category === category.value) && (!search.value.trim() || skillName(row.id).toLowerCase().includes(search.value.trim().toLowerCase()))))
const row = (id: string) => catalogByID.value.get(id)
const level = (id: string) => ownedByID.value.get(id)?.level || 1
const skillName = (id: string) => getLocalizedSkillName(t, id)
const itemName = (id: string) => getLocalizedItemName(t, id)
const categoryName = (id: string) => t(getSkillTypeDefinition(id)?.nameKey || id)
const totalHP = computed(() => draft.value.skillIds.reduce((n, id) => n + (row(id)?.hpCost || 0), 0))
const totalMP = computed(() => draft.value.skillIds.reduce((n, id) => n + (row(id)?.mpCost || 0), 0))
const groups = computed({ get: () => draft.value.groups || [], set: (value: RuleGroup[]) => { draft.value.groups = value } })
const ruleOptions = computed(() => props.catalog.filter(row => carriedIDs.value.has(row.id)).map(row => ({ value: row.id, label: skillName(row.id) })))
const draftReason = computed(() => !selected.value || (!draft.value.enabled && draft.value.skillIds.length === 0) ? "" : equipmentSelectionReason(selected.value, draft.value, candidateIDs.value, carriedIDs.value))
function reasonFor(view: EquipmentSkillView) {
  const candidates = new Set(props.catalog.filter(row => view.skillId === "equipment_martial_combo" ? row.comboEligible : row.dominationEligible).map(row => row.id))
  return equipmentSelectionReason(view, view.config, candidates, carriedIDs.value) || view.reason || ""
}
function reasonText(reason: string) { return reason === "CAPACITY_EXCEEDED" ? copy.value.capacity : reason === "NOT_CONFIGURED" ? copy.value.minimum : reason === "SKILL_NOT_CARRIED_OR_PHYSICAL" ? copy.value.notCarried : copy.value.invalid }
function open(view: EquipmentSkillView) {
  if (saving.value) return
  selected.value = view
  draft.value = cloneEquipmentSkillConfig(toRaw(view.config))
  draft.value.beforeSkillId = equipmentComboPriorityAnchor(draft.value.beforeSkillId, props.carried)
  draft.value.groups = normalizeRuleGroups(draft.value.groups, { allowEmpty: true })
  search.value = ""; category.value = ""; error.value = ""; ruleValid.value = true; visible.value = true
}
function toggle(id: string) { draft.value.skillIds = draft.value.skillIds.includes(id) ? draft.value.skillIds.filter(value => value !== id) : [...draft.value.skillIds, id] }
async function save() {
  if (!selected.value || draftReason.value || !ruleValid.value || saving.value) return
  const skillId = selected.value.skillId
  saving.value = true; error.value = ""
  try {
    const next = cloneEquipmentSkillConfig(toRaw(draft.value))
    const saved = props.draftMode ? next : (await apiPost<{ config: EquipmentSkillConfig }>("/api/battle/skills/equipment-config", props.token, { skillId, config: next })).config
    emit("saved", skillId, saved)
    visible.value = false
  } catch (err) { error.value = resolveErrorMessage(t, err, "game.saveFailed") } finally { saving.value = false }
}
</script>

<style scoped>
.equipment-skills { border-top: 1px solid var(--app-border); padding-top: 12px; }
.equipment-skills__title { font-size: 14px; font-weight: 600; margin-bottom: 8px; }
.equipment-skills__row { display: grid; grid-template-columns: minmax(130px, 1fr) minmax(0, 2fr) 32px; gap: 12px; align-items: center; padding: 10px 0; border-bottom: 1px solid var(--app-border); }
.equipment-skills__identity, .equipment-skills__summary, .equipment-settings__skill { display: flex; flex-direction: column; gap: 4px; min-width: 0; overflow-wrap: anywhere; }
.equipment-skills__identity, .equipment-settings__skill { align-items: flex-start; }
.equipment-skills__warning { font-size: 12px; color: var(--el-color-danger); }
.equipment-settings__header, .equipment-settings__priority { display: flex; align-items: center; justify-content: space-between; gap: 12px; margin-bottom: 12px; }
.equipment-settings__selected { margin: 12px 0; }
.equipment-settings__chosen { display: flex; gap: 6px; align-items: center; padding: 8px 0; border-bottom: 1px solid var(--app-border); }
.equipment-settings__skill { flex: 1; }
.equipment-settings__ordinal { width: 18px; text-align: center; color: var(--el-color-primary); font-variant-numeric: tabular-nums; }
.combo-drag { cursor: grab; flex-shrink: 0; }
.equipment-settings__chosen .el-button, .equipment-skills__row .el-button { width: 28px; height: 28px; flex-shrink: 0; margin-left: 0; padding: 5px; }
.equipment-settings__filters { display: grid; grid-template-columns: minmax(0, 1fr) 140px; gap: 8px; margin: 12px 0; }
.equipment-settings__candidates { max-height: 260px; overflow-y: auto; border-block: 1px solid var(--app-border); }
.equipment-settings__candidate { display: flex; align-items: center; gap: 8px; padding: 7px 2px; }
.equipment-settings__totals { padding: 12px 0; font-variant-numeric: tabular-nums; }
.equipment-settings__priority { margin-top: 12px; }
.equipment-settings__priority .el-select { width: 200px; }
@media (max-width: 480px) { .equipment-skills__row { grid-template-columns: minmax(0, 1fr) 28px; gap: 8px; } .equipment-skills__summary { grid-column: 1; } .equipment-skills__row > :last-child { grid-column: 2; grid-row: 1 / 3; } .equipment-settings__filters { grid-template-columns: minmax(0, 1fr) 110px; } .equipment-settings__chosen { gap: 3px; } .combo-drag { display: none; } }
</style>
