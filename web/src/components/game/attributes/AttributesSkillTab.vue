<template>
  <div class="space-y-4 text-sm" data-tutorial="skills-panel">
    <div class="rounded-lg border border-[var(--app-border)] p-3" data-tutorial="skills-slot-rule">
      <div class="text-xs app-muted">
        {{ t("game.combatSlotRuleHint", { base: slotRule.baseSkillSlots, intPer: slotRule.intPerExtra, min: slotRule.minSkillSlots, max: slotRule.maxSkillSlots, current: slotRule.current }) }}
      </div>
    </div>

    <EquipmentSkillSettings v-if="!configGetPath.includes('/cattery/')" :token="token" :views="equipmentSkillViews" :catalog="skillCatalog" :carried="config.skillSlots" :owned="ownedSkills" :draft-mode="isDraftMode" :item-options="ruleItemOptions" @saved="onEquipmentSkillSaved" />

    <el-tabs v-model="activeSkillTab" type="card" data-tutorial="skills-tabs">
      <el-tab-pane :label="t('game.combatSkillsTabEnabled')" name="enabled">
        <div class="space-y-2" data-tutorial="skills-enabled-list">
          <draggable
            v-if="enabledSkillsModel.length > 0"
            v-model="enabledSkillsModel"
            item-key="id"
            handle=".skill-card__drag-handle"
            :animation="180"
            class="grid gap-2 skill-grid"
          >
            <template #item="{ element }">
              <el-card class="skill-card h-full w-full" shadow="hover" body-class="p-2! h-36! h-full" data-tutorial="skills-enabled-card">
                <div class="flex h-full flex-col justify-between gap-3">
                  <div class="skill-card__drag-handle min-w-0" data-tutorial="skills-drag-handle">
                    <div class="flex items-start justify-between gap-1">
                      <SkillQuickRef
                        :skill-id="element.id"
                        :show-icon="true"
                        :unit-attributes="normalizedStatusValue"
                        :unit-skill-level="resolveOwnedSkillState(element.id).level"
                      />
                      <el-tooltip v-if="comboOnlySkills.has(element.id)" :content="locale.startsWith('zh') ? '仅通过连武施放' : 'Cast only through Martial Chain'"><Icon icon="lucide:link" class="shrink-0" /></el-tooltip>
                    </div>
                    <div class="mt-2 text-xs app-muted flex flex-row justify-between items-center">
                      <div>
                        {{ t("game.attrLevelLabel") }}: {{ resolveOwnedSkillState(element.id).level }}
                      </div>
                      <div class="text-[10px] mt-1">
                        {{ resolveOwnedSkillState(element.id).exp }} / {{ getLevelExp(resolveOwnedSkillState(element.id).level) }}
                      </div>
                    </div>
                    <template v-if="!isBasicSkill(element.id)">
                      <el-progress
                        class="mt-2"
                        :percentage="getExpPercent(resolveOwnedSkillState(element.id).level, resolveOwnedSkillState(element.id).exp)"
                        :stroke-width="6"
                        color="#67c23a"
                        :show-text="false"
                      />
                    </template>
                  </div>

                  <div class="space-y-1">
                    <el-button class="w-full" size="small" type="primary" data-tutorial="skills-rule-button" @click="openRuleDialog(element.id)">
                      {{ t("game.ruleConfigTitle") }}
                      <template v-if="getSkillRules(element.id).length">
                        ({{ getSkillRules(element.id).length }})
                      </template>
                    </el-button>
                    <el-button
                      v-if="!isBasicSkill(element.id)"
                      class="w-full ml-0!"
                      size="small"
                      type="danger"
                      data-tutorial="skills-disable-button"
                      @click="disableSkill(element.id)"
                    >
                      {{ t("game.combatSkillDisable") }}
                    </el-button>
                  </div>
                </div>
              </el-card>
            </template>
          </draggable>
          <div v-if="enabledSkillsModel.length === 0" class="text-xs app-muted">
            {{ t("game.combatSlotEmpty") }}
          </div>
        </div>
      </el-tab-pane>

      <el-tab-pane :label="t('game.combatSkillsTabDisabled')" name="disabled">
        <div class="space-y-2" data-tutorial="skills-disabled-list">
          <template v-if="disabledSkillSections.length > 0">
            <section v-for="section in disabledSkillSections" :key="`disabled-skill-group-${section.id}`" class="space-y-2">
              <div class="text-sm font-semibold">{{ section.label }}</div>
              <div class="grid gap-2 skill-grid">
                <el-card
                  v-for="entry in section.items"
                  :key="`disabled-skill-${entry.id}`"
                  class="skill-card h-full w-full"
                  shadow="hover"
                  body-class="p-2! h-full"
                  data-tutorial="skills-disabled-card"
                >
                  <div class="flex h-full flex-col justify-between gap-3">
                    <div class="min-w-0">
                      <div class="skill-card__header">
                        <SkillQuickRef
                          class="skill-card__name"
                          :skill-id="entry.id"
                          :show-icon="true"
                          :unit-attributes="normalizedStatusValue"
                          :unit-skill-level="resolveOwnedSkillState(entry.id).level"
                        />
                        <el-button
                          class="skill-card__toggle"
                          size="small"
                          type="success"
                          :disabled="Boolean(entry.reason)"
                          data-tutorial="skills-enable-button"
                          @click="enableSkill(entry.id)"
                        >
                          {{ t("game.combatSkillEnable") }}
                        </el-button>
                      </div>
                      <div class="mt-2 text-xs app-muted">
                        {{ t("game.attrLevelLabel") }}：{{ resolveOwnedSkillState(entry.id).level }}
                      </div>
                      <el-tag v-if="hasSkillRule(entry.id)" class="mt-1" size="small" type="warning" effect="plain">
                        已设置规则
                      </el-tag>
                      <template v-if="!isBasicSkill(entry.id)">
                        <el-progress
                          class="mt-2"
                          :percentage="getExpPercent(resolveOwnedSkillState(entry.id).level, resolveOwnedSkillState(entry.id).exp)"
                          :stroke-width="10"
                          color="#67c23a"
                          :show-text="false"
                        />
                      </template>
                      <div
                        v-if="entry.reason"
                        class="skill-requirement-notice mt-2"
                        :class="`skill-requirement-notice--${entry.reasonKind || 'other'}`"
                        data-tutorial="skills-requirements"
                      >
                        <span class="skill-requirement-notice__dot" aria-hidden="true"></span>
                        {{ entry.reason }}
                      </div>
                    </div>

                    <el-button class="w-full" size="small" type="primary" data-tutorial="skills-rule-button" @click="openRuleDialog(entry.id)">
                      {{ t("game.ruleConfigTitle") }}
                    </el-button>
                  </div>
                </el-card>
              </div>
            </section>
          </template>
          <div v-else class="text-xs app-muted">
            {{ t("game.combatOwnedSkillsEmpty") }}
          </div>
        </div>
      </el-tab-pane>

    </el-tabs>

    <GameDialog v-model="ruleDialogVisible" :title="ruleDialogTitle" width="560px" data-tutorial="skills-rule-dialog">
      <div class="space-y-3">
        <div class="rounded-lg border border-[var(--app-border)] p-3" data-tutorial="skills-rule-target">
          <div class="flex flex-wrap items-center gap-3">
            <SkillQuickRef
              v-if="selectedRuleSlot"
              :skill-id="selectedRuleSlot"
              :show-icon="true"
              :unit-attributes="normalizedStatusValue"
              :unit-skill-level="resolveOwnedSkillState(selectedRuleSlot).level"
            />
          </div>
        </div>

        <RuleLogicEditor
          v-if="selectedRuleSlot"
          v-model="selectedRuleGroups"
          data-tutorial="skills-rule-editor"
          :max-groups="ruleLimits.maxGroups"
          :max-conditions="ruleLimits.maxConditionsPerGroup"
          :skill-options="ruleSkillOptions"
          :item-options="ruleItemOptions"
          @validation="ruleValidation = $event"
        />

        <div v-else class="text-xs app-muted">
          {{ t("game.ruleNeedSkillSlot") }}
        </div>
      </div>

      <template #footer>
        <el-button @click="closeRuleDialog">{{ t("game.cancel") }}</el-button>
        <el-button type="primary" :disabled="!selectedRuleSlot || !ruleValidation.valid" @click="saveRule">
          {{ t("game.saveConfig") }}
        </el-button>
      </template>
    </GameDialog>
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from "vue"
import draggable from "vuedraggable"
import { ElMessage } from "element-plus"
import { useI18n } from "vue-i18n"
import { Icon } from "@iconify/vue"
import { apiGet, apiPost } from "@services/api"
import { resolveErrorMessage } from "@services/error-messages"
import RuleLogicEditor from "@components/game/rules/RuleLogicEditor.vue"
import SkillQuickRef from "@components/game/skill/SkillQuickRef.vue"
import EquipmentSkillSettings from "@components/game/skill/EquipmentSkillSettings.vue"
import { emptyEquipmentSkillConfig, equipmentSelectionReason, equipmentCarriedSkillIDs, type EquipmentSkillConfig, type EquipmentSkillView } from "@services/equipment-skill-config"
import { getEquipmentTemplateDefinition } from "@services/item-definition"
import { getLocalizedSkillName, getSkillDefinition, getSkillTypeDefinition, installRuntimeSkillDefinitions, listSkillTypeDefinitions } from "@services/skill-definition"
import { getLocalizedWeaponClassLabel } from "@services/weapon-class"
import { listItemDefinitions } from "@services/item-definition"
import { getLocalizedItemName } from "@services/item-label"
import { createDefaultRuleCondition, normalizeRuleGroups, type RuleGroup } from "@services/rule-logic"
import { useEquipmentStore } from "@stores/equipment"
import { useAttributeStats } from "@/composables/game/attributes/useAttributeStats"
import GameDialog from "@components/game/common/GameDialog.vue"

const SKILLS_TUTORIAL_OPEN_RULE_EVENT = "character-skills:tutorial-open-rule"

type OwnedSkill = {
  skillId: string
  level?: number
  exp?: number
}

type SkillCatalogItem = {
	comboEligible?: boolean
	dominationEligible?: boolean
  id: string
  name?: string
  nameKey?: string
  descKey?: string
  category?: string
  hpCost: number
  mpCost: number
  cooldown: number
  statusRequirement?: Record<string, number>
  weaponRequirement?: string[]
  eligible?: boolean
  unmetStatusRequirements?: Array<{
    attrId: string
    required: number
    current: number
  }>
  missingWeaponClasses?: string[]
}

type DisabledSkillEntry = {
  id: string
  reason: string | null
  reasonKind?: SkillDisableReasonKind
}

type SkillSection<T extends { id: string }> = {
  id: string
  label: string
  items: T[]
}

type RuleSet = {
  slotKey: string
  enabled: boolean
  groups: unknown
}

type CombatConfigPayload = {
	equipmentSkills?: EquipmentSkillView[]
  defaultBasicSkillId?: string
  config: {
    skillSlots: string[]
  }
  ownedSkills: OwnedSkill[]
  rules: RuleSet[]
  skillCatalog: SkillCatalogItem[]
  slotRule: {
    baseSkillSlots: number
    intPerExtra: number
    minSkillSlots: number
    maxSkillSlots: number
    current: number
  }
  ruleLimits?: {
    maxGroups: number
    maxConditionsPerGroup: number
  }
}

type DraftSkillRule = {
  skillId: string
  enabled: boolean
  groups: unknown
}

type SkillDisableReasonKind = "status" | "weapon" | "other"

type SkillDisableNotice = {
  text: string
  kind: SkillDisableReasonKind
}

const props = withDefaults(defineProps<{
  token: string
  mode?: "live" | "draft"
  configGetPath?: string
  configPostPath?: string
  ruleGetPath?: string
  rulePostPath?: string
  unitAttributes?: Record<string, number> | null
  draftSkillSlots?: string[]
  draftSkillRules?: DraftSkillRule[]
  draftEquipmentSkills?: Record<string, EquipmentSkillConfig>
  draftEquipment?: { templateItemId: string; enhanceLevel: number }[]
}>(), {
  mode: "live",
  draftSkillSlots: () => [],
  draftSkillRules: () => []
})

const emit = defineEmits<{
  (e: "update:draftSkillSlots", value: string[]): void
  (e: "update:draftSkillRules", value: DraftSkillRule[]): void
  (e: "update:draftEquipmentSkills", value: Record<string, EquipmentSkillConfig>): void
}>()

const { t, locale } = useI18n()
const equipmentStore = useEquipmentStore()
const { normalizedStatusValue: playerStatusValue } = useAttributeStats()
const activeSkillTab = ref("enabled")
const defaultBasicSkillID = ref("")
const config = ref({
  skillSlots: [] as string[]
})
const slotRule = ref({
  baseSkillSlots: 1,
  intPerExtra: 10,
  minSkillSlots: 1,
  maxSkillSlots: 8,
  current: 1
})
const ruleLimits = ref({
  maxGroups: 6,
  maxConditionsPerGroup: 6
})
const ownedSkills = ref<OwnedSkill[]>([])
const skillCatalog = ref<SkillCatalogItem[]>([])
const liveEquipmentSkillViews = ref<EquipmentSkillView[]>([])
const equipmentSkillViews = computed<EquipmentSkillView[]>(() => {
  if (!isDraftMode.value) return liveEquipmentSkillViews.value
  const views = new Map<string, EquipmentSkillView>()
  for (const item of props.draftEquipment || []) {
    const grant = getEquipmentTemplateDefinition(item.templateItemId)?.grantedSkill
    if (!grant) continue
    const capacity = grant.baseCapacity + grant.capacityMilestones.filter(level => item.enhanceLevel >= level).length
    if ((views.get(grant.skillId)?.capacity || 0) >= capacity) continue
    views.set(grant.skillId, { skillId: grant.skillId, itemId: item.templateItemId, enhanceLevel: item.enhanceLevel, capacity, config: props.draftEquipmentSkills?.[grant.skillId] || emptyEquipmentSkillConfig() })
  }
  return [...views.values()]
})
const comboOnlySkills = computed(() => {
  const view = equipmentSkillViews.value.find(row => row.skillId === "equipment_martial_combo")
  if (!view || !view.config.enabled || view.config.allowStandalone || view.reason) return new Set<string>()
  const candidates = new Set(skillCatalog.value.filter(row => row.comboEligible).map(row => row.id))
  const carried = equipmentCarriedSkillIDs(config.value.skillSlots, skillCatalog.value, isDraftMode.value)
  return equipmentSelectionReason(view, view.config, candidates, carried) ? new Set<string>() : new Set(view.config.skillIds)
})
function onEquipmentSkillSaved(skillId: string, saved: EquipmentSkillConfig) {
  if (isDraftMode.value) emit("update:draftEquipmentSkills", { ...props.draftEquipmentSkills, [skillId]: saved })
  else liveEquipmentSkillViews.value = liveEquipmentSkillViews.value.map(view => view.skillId === skillId ? { ...view, config: saved, reason: undefined } : view)
}
const skillRulesBySlot = ref<Record<string, { enabled: boolean; groups: RuleGroup[] }>>({})
const itemRulesBySlot = ref<Record<string, { enabled: boolean; groups: RuleGroup[] }>>({})
const selectedRuleKind = ref<"skill" | "item">("skill")
const selectedRuleSlot = ref("")
const ruleDialogVisible = ref(false)
const ruleValidation = ref<{ valid: boolean; errors: string[] }>({ valid: true, errors: [] })
const loadingConfig = ref(false)
const savingConfig = ref(false)
const pendingSaveConfig = ref(false)
const autoSaveTimer = ref<ReturnType<typeof setTimeout> | null>(null)
const lastSavedSkillSlots = ref<string[]>([])
const configGetPath = computed(() => props.configGetPath || "/api/battle/skills/config")
const configPostPath = computed(() => props.configPostPath || "/api/battle/skills/config")
const ruleGetPath = computed(() => props.ruleGetPath || "/api/rules/config?kind=item")
const rulePostPath = computed(() => props.rulePostPath || "/api/rules/config")
const normalizedStatusValue = computed(() => props.unitAttributes || playerStatusValue.value)
const isDraftMode = computed(() => props.mode === "draft")

const skillCatalogMap = computed(() => {
  const map = new Map<string, SkillCatalogItem>()
  for (const row of skillCatalog.value) {
    const id = String(row.id || "").trim()
    if (!id) continue
    map.set(id, row)
  }
  return map
})

const basicSkillID = computed(() => {
  const preferred = String(defaultBasicSkillID.value || "").trim()
  if (preferred) return preferred
  return String(skillCatalog.value[0]?.id || "").trim()
})

const ownedSkillIDs = computed(() => {
  const out: string[] = []
  const seen = new Set<string>()
  for (const row of ownedSkills.value) {
    const id = String(row.skillId || "").trim()
    if (!id) continue
    const key = id.toLowerCase()
    if (seen.has(key)) continue
    seen.add(key)
    out.push(id)
  }
  if (basicSkillID.value && !seen.has(basicSkillID.value.toLowerCase())) {
    out.push(basicSkillID.value)
  }
  out.sort((a, b) => a.localeCompare(b))
  return out
})

const enabledSkillsModel = computed<SkillCatalogItem[]>({
  get: () =>
    config.value.skillSlots
      .map((id) => skillCatalogMap.value.get(id))
      .filter((row): row is SkillCatalogItem => Boolean(row)),
  set: (rows) => {
    config.value.skillSlots = rows.map((row) => row.id)
  }
})

const disabledSkillIDs = computed(() => {
  const enabled = new Set(config.value.skillSlots.map((row) => row.toLowerCase()))
  const basic = basicSkillID.value.toLowerCase()
  return ownedSkillIDs.value.filter((skillID) => {
    const key = skillID.toLowerCase()
    if (key === basic) return false
    return !enabled.has(key)
  })
})

const disabledSkills = computed(() =>
  disabledSkillIDs.value.map((skillID) => {
    const notice = resolveSkillDisableNotice(skillID)
    return {
      id: skillID,
      reason: notice?.text || null,
      reasonKind: notice?.kind || "other"
    }
  })
)

const skillCategoryOrder = computed(() =>
  listSkillTypeDefinitions()
    .map((type) => String(type.id || "").trim())
    .filter((id) => id !== "")
)

const disabledSkillSections = computed<SkillSection<DisabledSkillEntry>[]>(() =>
  buildSkillSections(disabledSkills.value, (item) => item.id)
)

const selectedRuleGroups = computed({
  get: () => {
    const slotKey = selectedRuleSlot.value
    if (!slotKey) return []
    const rule = getRuleMap(selectedRuleKind.value)[slotKey]
    if (!rule) return []
    return rule.groups
  },
  set: (value: RuleGroup[]) => {
    const slotKey = selectedRuleSlot.value
    if (!slotKey) return
    ensureRule(selectedRuleKind.value, slotKey)
    const rule = getRuleMap(selectedRuleKind.value)[slotKey]
    rule.groups = normalizeRuleGroups(value, { allowEmpty: true })
  }
})

const ruleSkillOptions = computed(() =>
  skillCatalog.value
    .map((row) => ({ value: row.id, label: resolveSkillName(row.id) }))
    .sort((a, b) => a.label.localeCompare(b.label))
)

const ruleItemOptions = computed(() =>
  listItemDefinitions()
    .filter((item) => item.carryableInCombat)
    .map((item) => ({
      value: item.id,
      label: getLocalizedItemName(t, item.id, { fallbackText: item.id })
    }))
    .sort((a, b) => a.label.localeCompare(b.label))
)

const ruleDialogTitle = computed(() => {
  if (!selectedRuleSlot.value) {
    return t("game.ruleConfigTitle")
  }
  return `${t("game.ruleConfigTitle")} · ${resolveSkillName(selectedRuleSlot.value)}`
})

watch(
  () => props.token,
  (token) => {
    if (!token) return
    void loadConfig()
  },
  { immediate: true }
)

onMounted(() => {
  window.addEventListener(SKILLS_TUTORIAL_OPEN_RULE_EVENT, handleSkillsTutorialOpenRule)
})

watch(
  () => props.draftSkillSlots,
  (rows) => {
    if (!isDraftMode.value) return
    config.value.skillSlots = normalizeSkillSlots(Array.isArray(rows) ? rows : [])
    lastSavedSkillSlots.value = [...config.value.skillSlots]
  },
  { immediate: true, deep: true }
)

watch(
  () => props.draftSkillRules,
  (rows) => {
    if (!isDraftMode.value) return
    skillRulesBySlot.value = {}
    for (const row of Array.isArray(rows) ? rows : []) {
      const key = String(row.skillId || "").trim()
      if (!key) continue
      skillRulesBySlot.value[key] = {
        enabled: Boolean(row.enabled),
        groups: normalizeRuleGroups(row.groups, { allowEmpty: true })
      }
    }
  },
  { immediate: true, deep: true }
)

function resolveSkillName(skillID: string) {
  return getLocalizedSkillName(t, skillID)
}

function resolveSkillCategory(skillID: string): string {
  const category = String(getSkillDefinition(skillID)?.category || "").trim()
  return category || "other"
}

function resolveSkillCategoryLabel(categoryID: string): string {
  if (categoryID === "other") return t("game.dictionaryOtherGroup")
  const nameKey = getSkillTypeDefinition(categoryID)?.nameKey
  if (!nameKey) return categoryID
  const localized = t(nameKey)
  return localized !== nameKey && localized.trim() !== "" ? localized : categoryID
}

function buildSkillSections<T extends { id: string }>(items: T[], getSkillID: (item: T) => string): SkillSection<T>[] {
  const groups = new Map<string, T[]>()
  for (const item of items) {
    const skillID = String(getSkillID(item) || "").trim()
    if (!skillID) continue
    const categoryID = resolveSkillCategory(skillID)
    if (!groups.has(categoryID)) groups.set(categoryID, [])
    groups.get(categoryID)?.push(item)
  }
  const orderedCategories = skillCategoryOrder.value.filter((categoryID) => groups.has(categoryID))
  for (const categoryID of groups.keys()) {
    if (!orderedCategories.includes(categoryID)) orderedCategories.push(categoryID)
  }
  return orderedCategories
    .map((categoryID) => ({
      id: categoryID,
      label: resolveSkillCategoryLabel(categoryID),
      items: groups.get(categoryID) || []
    }))
    .filter((section) => section.items.length > 0)
}

function getRuleMap(kind: "skill" | "item") {
  return kind === "item" ? itemRulesBySlot.value : skillRulesBySlot.value
}

function ensureRule(kind: "skill" | "item", slotKey: string) {
  const map = getRuleMap(kind)
  if (!map[slotKey]) {
    map[slotKey] = {
      enabled: false,
      groups: []
    }
  }
}

function findSkillRule(skillID: string) {
  const normalized = String(skillID || "").trim()
  if (!normalized) return null
  const exact = skillRulesBySlot.value[normalized]
  if (exact) return exact
  const lowered = normalized.toLowerCase()
  const matchedKey = Object.keys(skillRulesBySlot.value).find((key) => key.toLowerCase() === lowered)
  return matchedKey ? skillRulesBySlot.value[matchedKey] : null
}

function getSkillRules(skillID: string) {
  const rule = findSkillRule(skillID)
  if (!rule) return []
  return normalizeRuleGroups(rule.groups, { allowEmpty: true })
}

function hasSkillRule(skillID: string) {
  return getSkillRules(skillID).some((group) => group.conditions.length > 0)
}

function resolveOwnedSkillState(skillID: string) {
  const key = String(skillID || "").trim().toLowerCase()
  if (!key) return { level: 1, exp: 0 }
  const row = ownedSkills.value.find((item) => String(item.skillId || "").trim().toLowerCase() === key)
  if (!row) return { level: 1, exp: 0 }
  return {
    level: Math.max(1, Math.trunc(Number(row.level ?? 1))),
    exp: Math.max(0, Math.trunc(Number(row.exp ?? 0)))
  }
}

function getLevelExp(level: number) {
  const normalized = Math.max(1, Math.trunc(Number(level || 1)))
  return 100 + (normalized - 1) * 100
}

function getExpPercent(level: number, exp: number) {
  const need = getLevelExp(level)
  if (!need) return 100
  return Math.min(100, Math.round((Math.max(0, Number(exp || 0)) / need) * 100))
}

function openRuleDialog(skillID: string) {
  const normalized = String(skillID || "").trim()
  if (!normalized) return
  selectedRuleKind.value = "skill"
  selectedRuleSlot.value = normalized
  ensureRule("skill", normalized)
  ruleDialogVisible.value = true
}

function pickTutorialSkillID() {
  return String(enabledSkillsModel.value[0]?.id || ownedSkillIDs.value[0] || skillCatalog.value[0]?.id || basicSkillID.value || "").trim()
}

function handleSkillsTutorialOpenRule() {
  activeSkillTab.value = "enabled"
  const skillID = pickTutorialSkillID()
  if (skillID) {
    openRuleDialog(skillID)
  }
}

function closeRuleDialog() {
  ruleDialogVisible.value = false
}

function skillLineageRoot(skillID: string) {
  const normalized = String(skillID || "").trim()
  const sourceID = String(getSkillDefinition(normalized)?.customForge?.sourceSkillId || "").trim()
  return (sourceID || normalized).toLowerCase()
}

function findSkillLineageConflict(skillIDs: string[]) {
  const byRoot = new Map<string, string>()
  for (const value of skillIDs) {
    const skillID = String(value || "").trim()
    if (!skillID) continue
    const root = skillLineageRoot(skillID)
    const existing = byRoot.get(root)
    if (existing && existing.toLowerCase() !== skillID.toLowerCase()) {
      return { first: existing, second: skillID }
    }
    byRoot.set(root, skillID)
  }
  return null
}

function enabledRuleSkillIDs(excludeSkillID = "") {
  const excluded = excludeSkillID.trim().toLowerCase()
  return Object.entries(skillRulesBySlot.value)
    .filter(([skillID, state]) => state.enabled && skillID.trim().toLowerCase() !== excluded)
    .map(([skillID]) => skillID)
}

function showSkillLineageConflict(conflict: { first: string; second: string }) {
  ElMessage.warning(t("game.error.combatConfigLineageConflict", {
    first: resolveSkillName(conflict.first),
    second: resolveSkillName(conflict.second)
  }))
}

function enableSkill(skillID: string) {
  const notice = resolveSkillDisableNotice(skillID)
  if (notice) {
    ElMessage.warning(notice.text)
    return
  }
  const lineageConflict = findSkillLineageConflict([
    ...config.value.skillSlots,
    ...enabledRuleSkillIDs(),
    skillID
  ])
  if (lineageConflict) {
    showSkillLineageConflict(lineageConflict)
    return
  }
  if (config.value.skillSlots.length >= slotRule.value.current) {
    ElMessage.warning(t("game.combatSlotLimitReached", { count: slotRule.value.current }))
    return
  }
  const key = skillID.toLowerCase()
  if (config.value.skillSlots.some((row) => row.toLowerCase() === key)) return
  config.value.skillSlots = [...config.value.skillSlots, skillID]
}

function disableSkill(skillID: string) {
  if (isBasicSkill(skillID)) return
  config.value.skillSlots = config.value.skillSlots.filter((row) => row !== skillID)
}

async function loadConfig() {
  if (!props.token) return
  loadingConfig.value = true
  try {
    const payload = await apiGet<CombatConfigPayload>(configGetPath.value, props.token)
    defaultBasicSkillID.value = String(payload.defaultBasicSkillId || "").trim()
    const liveSkillSlots = Array.isArray(payload.config?.skillSlots)
      ? payload.config.skillSlots
          .map((row) => String(row || "").trim())
          .filter((row) => Boolean(row))
      : []
    config.value = {
      skillSlots: isDraftMode.value
        ? normalizeSkillSlots(Array.isArray(props.draftSkillSlots) ? props.draftSkillSlots : [])
        : liveSkillSlots
    }
    lastSavedSkillSlots.value = [...config.value.skillSlots]
    ownedSkills.value = Array.isArray(payload.ownedSkills) ? payload.ownedSkills : []
	 liveEquipmentSkillViews.value = payload.equipmentSkills || []
    skillCatalog.value = Array.isArray(payload.skillCatalog) ? payload.skillCatalog : []
	installRuntimeSkillDefinitions(skillCatalog.value)
    slotRule.value = payload.slotRule || slotRule.value
    if (payload.ruleLimits) {
      ruleLimits.value = {
        maxGroups: Math.max(1, Number(payload.ruleLimits.maxGroups || 6)),
        maxConditionsPerGroup: Math.max(1, Number(payload.ruleLimits.maxConditionsPerGroup || 6))
      }
    }
    skillRulesBySlot.value = {}
    const rawRules = isDraftMode.value
      ? (Array.isArray(props.draftSkillRules) ? props.draftSkillRules.map((row) => ({
          slotKey: row.skillId,
          enabled: row.enabled,
          groups: row.groups
        })) : [])
      : (Array.isArray(payload.rules) ? payload.rules : [])
    for (const rule of rawRules) {
      const slotKey = String(rule.slotKey || "").trim()
      if (!slotKey) continue
      skillRulesBySlot.value[slotKey] = {
        enabled: Boolean(rule.enabled),
        groups: normalizeRuleGroups(rule.groups, { allowEmpty: true })
      }
    }
    itemRulesBySlot.value = {}
    if (!isDraftMode.value) {
      const itemRulesPayload = await apiGet<{ items: RuleSet[] }>(ruleGetPath.value, props.token)
      const itemRules = Array.isArray(itemRulesPayload.items) ? itemRulesPayload.items : []
      for (const rule of itemRules) {
        const slotKey = String(rule.slotKey || "").trim()
        if (!slotKey) continue
        itemRulesBySlot.value[slotKey] = {
          enabled: Boolean(rule.enabled),
          groups: normalizeRuleGroups(rule.groups, { allowEmpty: true })
        }
      }
    }
  } catch (err) {
    ElMessage.error(resolveErrorMessage(t, err, "game.combatConfigLoadFailed"))
  } finally {
    loadingConfig.value = false
  }
}

async function saveConfigAuto() {
  if (!props.token) return
  const normalized = normalizeSkillSlots(config.value.skillSlots)
  config.value.skillSlots = normalized
  if (sameSkillSlots(normalized, lastSavedSkillSlots.value)) return
  const lineageConflict = findSkillLineageConflict([...normalized, ...enabledRuleSkillIDs()])
  if (lineageConflict) {
    showSkillLineageConflict(lineageConflict)
    return
  }
  if (isDraftMode.value) {
    lastSavedSkillSlots.value = [...normalized]
    emit("update:draftSkillSlots", [...normalized])
    return
  }
  if (savingConfig.value) {
    pendingSaveConfig.value = true
    return
  }
  savingConfig.value = true
  try {
    await apiPost(configPostPath.value, props.token, {
      skillSlots: normalized
    })
    lastSavedSkillSlots.value = [...normalized]
  } catch (err) {
    ElMessage.error(resolveErrorMessage(t, err, "game.saveFailed"))
  } finally {
    savingConfig.value = false
    if (pendingSaveConfig.value) {
      pendingSaveConfig.value = false
      void saveConfigAuto()
    }
  }
}

async function saveRule() {
  if (!props.token || !selectedRuleSlot.value) return
  if (!ruleValidation.value.valid) return
  try {
    ensureRule(selectedRuleKind.value, selectedRuleSlot.value)
    const ruleState = getRuleMap(selectedRuleKind.value)[selectedRuleSlot.value]
    if (selectedRuleKind.value === "skill") {
      const lineageConflict = findSkillLineageConflict([
        ...config.value.skillSlots,
        ...enabledRuleSkillIDs(selectedRuleSlot.value),
        selectedRuleSlot.value
      ])
      if (lineageConflict) {
        showSkillLineageConflict(lineageConflict)
        return
      }
      ruleState.enabled = true
    }
    if (isDraftMode.value) {
      const nextRules = Object.entries(skillRulesBySlot.value)
        .map(([skillId, state]) => ({
          skillId,
          enabled: Boolean(state.enabled),
          groups: normalizeRuleGroups(state.groups, { allowEmpty: true })
        }))
        .sort((a, b) => a.skillId.localeCompare(b.skillId))
      emit("update:draftSkillRules", nextRules)
      ruleDialogVisible.value = false
      return
    }
    await apiPost(rulePostPath.value, props.token, {
      ruleKind: selectedRuleKind.value,
      slotKey: selectedRuleSlot.value,
      enabled: true,
      groups: ruleState.groups
    })
    ElMessage.success(t("game.saveSuccess"))
    ruleDialogVisible.value = false
  } catch (err) {
    ElMessage.error(resolveErrorMessage(t, err, "game.saveFailed"))
  }
}

function isBasicSkill(skillID: string) {
  const basic = String(basicSkillID.value || "").trim().toLowerCase()
  return basic !== "" && String(skillID || "").trim().toLowerCase() === basic
}

function resolveSkillIcon(skillID: string): string {
  return String(getSkillDefinition(skillID)?.icon || "").trim()
}

function resolveWeaponClassLabel(weaponClass: string) {
  return getLocalizedWeaponClassLabel(t, weaponClass, String(weaponClass || "").trim())
}

function resolveSkillDisableNotice(skillID: string): SkillDisableNotice | null {
  if (isDraftMode.value) return null
  const row = skillCatalogMap.value.get(skillID)
  if (!row || row.eligible !== false) return null
  const unmetStatus = Array.isArray(row.unmetStatusRequirements) ? row.unmetStatusRequirements : []
  if (unmetStatus.length > 0) {
    const requirements = unmetStatus
      .map((entry) => {
        const attrID = String(entry.attrId || "").trim()
        if (!attrID) return ""
        const attrName = t(`attr.${attrID}.name`)
        const displayName = attrName === `attr.${attrID}.name` ? attrID : attrName
        const required = Number(entry.required || 0)
        return `${displayName}≥${required}`
      })
      .filter(Boolean)
      .join(" / ")
    if (requirements) {
      return {
        text: t("game.combatSkillNeedStatus", { requirements }),
        kind: "status"
      }
    }
  }
  const missingWeaponClasses = Array.isArray(row.missingWeaponClasses) ? row.missingWeaponClasses : []
  if (missingWeaponClasses.length > 0) {
    const labels = missingWeaponClasses.map((weaponClass) => resolveWeaponClassLabel(weaponClass)).join(" / ")
    return {
      text: t("game.combatSkillNeedWeapon", { weapons: labels }),
      kind: "weapon"
    }
  }
  return null
}

function normalizeSkillSlots(raw: string[]) {
  const out: string[] = []
  const seen = new Set<string>()
  for (const row of raw) {
    const id = String(row || "").trim()
    if (!id) continue
    const key = id.toLowerCase()
    if (seen.has(key)) continue
    seen.add(key)
    out.push(id)
  }
  return out
}

function sameSkillSlots(a: string[], b: string[]) {
  if (a.length !== b.length) return false
  for (let idx = 0; idx < a.length; idx += 1) {
    if (a[idx].toLowerCase() !== b[idx].toLowerCase()) {
      return false
    }
  }
  return true
}

function scheduleAutoSaveConfig() {
  if (loadingConfig.value || !props.token) return
  if (autoSaveTimer.value) {
    clearTimeout(autoSaveTimer.value)
  }
  autoSaveTimer.value = setTimeout(() => {
    autoSaveTimer.value = null
    void saveConfigAuto()
  }, 300)
}

watch(
  () => config.value.skillSlots.map((row) => String(row || "").trim().toLowerCase()).join("|"),
  () => {
    scheduleAutoSaveConfig()
  }
)

watch(
  () => `${equipmentStore.equippedVersion}|${equipmentStore.stats.updatedAt}`,
  () => {
    if (isDraftMode.value) return
    if (!props.token) return
    void loadConfig()
  }
)

onBeforeUnmount(() => {
  window.removeEventListener(SKILLS_TUTORIAL_OPEN_RULE_EVENT, handleSkillsTutorialOpenRule)
  if (autoSaveTimer.value) {
    clearTimeout(autoSaveTimer.value)
    autoSaveTimer.value = null
  }
})
</script>

<style scoped>
.skill-card {
  position: relative;
  transition: box-shadow 0.18s ease, transform 0.18s ease;
}

.skill-card__icon {
  position: absolute;
  top: 8px;
  left: 8px;
  z-index: 1;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  font-size: 16px;
  color: var(--el-color-primary);
}

.skill-card__drag-handle {
  cursor: grab;
}

.skill-card__drag-handle:active {
  cursor: grabbing;
}

.skill-card__header {
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto;
  align-items: start;
  gap: 8px;
  min-width: 0;
}

.skill-card__name {
  width: 100%;
  min-width: 0;
  max-width: 100%;
  overflow: hidden;
}

.skill-card__toggle {
  flex: 0 0 auto;
  margin-left: 0;
}

:deep(.skill-card__name .el-tag__content) {
  display: block;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.skill-grid {
  grid-template-columns: repeat(auto-fill, minmax(160px, 1fr));
}

.skill-requirement-notice {
  --skill-requirement-dot-ring: rgba(107, 114, 128, 0.16);
  display: flex;
  align-items: flex-start;
  gap: 6px;
  width: 100%;
  border-radius: 6px;
  border: 1px solid transparent;
  padding: 5px 7px;
  font-size: 12px;
  line-height: 1.35;
}

.skill-requirement-notice__dot {
  flex: 0 0 auto;
  width: 7px;
  height: 7px;
  margin-top: 4px;
  border-radius: 999px;
  background: currentColor;
  box-shadow: 0 0 0 3px var(--skill-requirement-dot-ring);
}

.skill-requirement-notice--weapon {
  --skill-requirement-dot-ring: rgba(245, 158, 11, 0.18);
  border-color: #f59e0b;
  background: #fff7ed;
  color: #c2410c;
}

.skill-requirement-notice--status {
  --skill-requirement-dot-ring: rgba(244, 114, 182, 0.2);
  border-color: #f472b6;
  background: #fff1f7;
  color: #be185d;
}

.skill-requirement-notice--other {
  border-color: var(--app-border);
  background: var(--app-surface-soft, var(--app-surface));
  color: var(--el-text-color-regular);
}

@media (max-width: 640px) {
  .skill-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}
</style>
