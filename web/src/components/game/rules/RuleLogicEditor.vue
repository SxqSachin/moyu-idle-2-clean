<template>
  <div class="rule-builder">
    <div class="rule-builder__toolbar">
      <el-button size="small" @click="applyTemplate('heal')">{{ t("game.ruleTemplateHeal") }}</el-button>
      <el-button size="small" @click="applyTemplate('self')">{{ t("game.ruleTemplateSelfProtect") }}</el-button>
      <el-button size="small" @click="applyTemplate('execute')">{{ t("game.ruleTemplateExecute") }}</el-button>
      <el-button size="small" @click="applyTemplate('boss')">{{ t("game.ruleTemplateBoss") }}</el-button>
      <el-button size="small" @click="applyTemplate('cleanse')">{{ t("game.ruleTemplateCleanse") }}</el-button>
      <el-button size="small" type="primary" plain @click="addGroup" :disabled="groups.length >= maxGroups">
        {{ t("game.ruleAddGroup") }}
      </el-button>
    </div>

    <div v-if="groups.length === 0" class="rule-builder__empty">
      {{ t("game.ruleNoConditions") }}
    </div>

    <el-tabs v-else v-model="activeTab" type="card" class="rule-builder__tabs">
      <el-tab-pane
        v-for="(group, groupIndex) in groups"
        :key="groupIndex"
        :label="`${t('game.ruleGroupLabel')} ${groupIndex + 1}`"
        :name="String(groupIndex)"
      >
        <div class="rule-group">
          <div class="rule-group__hint">{{ t("game.ruleGroupHint") }}</div>
          <article
            v-for="(logic, logicIndex) in group.conditions"
            :key="`${groupIndex}-${logicIndex}`"
            class="rule-condition"
            :class="{ 'rule-condition--invalid': conditionErrors(groupIndex, logicIndex).length > 0 }"
          >
            <header class="rule-condition__header">
              <button type="button" class="rule-condition__summary" @click="toggleExpanded(groupIndex, logicIndex)">
                <span class="rule-condition__index">{{ logicIndex + 1 }}</span>
                <span class="rule-condition__text">{{ summarizeCondition(logic) }}</span>
              </button>
              <div class="rule-condition__actions">
                <el-tag v-if="conditionErrors(groupIndex, logicIndex).length > 0" type="danger" effect="plain">
                  {{ t("game.ruleInvalid") }}
                </el-tag>
                <el-button text size="small" @click="toggleExpanded(groupIndex, logicIndex)">
                  {{ isExpanded(groupIndex, logicIndex) ? t("game.ruleCollapse") : t("game.ruleExpand") }}
                </el-button>
                <el-button text type="danger" size="small" @click="removeCondition(groupIndex, logicIndex)">
                  {{ t("game.ruleDeleteCondition") }}
                </el-button>
              </div>
            </header>

            <div v-if="conditionErrors(groupIndex, logicIndex).length > 0" class="rule-condition__errors">
              <div v-for="error in conditionErrors(groupIndex, logicIndex)" :key="error">{{ error }}</div>
            </div>

            <div v-if="isExpanded(groupIndex, logicIndex)" class="rule-condition__editor">
              <label>
                <span>{{ t("game.ruleCompareType") }}</span>
                <el-select v-model="logic.compareType" class="w-full" @change="onCompareTypeChange(groupIndex, logicIndex)">
                  <el-option v-for="option in compareTypeOptions" :key="option.value" :value="option.value" :label="option.label" />
                </el-select>
              </label>
              <label>
                <span>{{ t("game.ruleTarget") }}</span>
                <el-select v-model="logic.target" class="w-full">
                  <el-option v-for="option in targetOptions" :key="option.value" :value="option.value" :label="option.label" />
                </el-select>
              </label>

              <template v-if="logic.compareType === 'value'">
                <label>
                  <span>{{ t("game.ruleDataType") }}</span>
                  <el-select v-model="logic.dataType" class="w-full" @change="onDataTypeChange(groupIndex, logicIndex)">
                    <el-option-group v-for="groupOption in groupedDataTypeOptions" :key="groupOption.label" :label="groupOption.label">
                      <el-option v-for="option in groupOption.options" :key="option.value" :value="option.value" :label="option.label" />
                    </el-option-group>
                  </el-select>
                </label>
                <label>
                  <span>{{ t("game.ruleCompareAction") }}</span>
                  <el-select v-model="logic.valueCompareAction" class="w-full">
                    <el-option v-for="option in compareActionOptions" :key="option.value" :value="option.value" :label="option.label" />
                  </el-select>
                </label>
                <label>
                  <span>{{ t("game.ruleConditionValue") }}</span>
                  <el-input
                    :model-value="stringValue(logic)"
                    :placeholder="t('game.ruleValuePlaceholder')"
                    @update:model-value="updateStringValue(groupIndex, logicIndex, $event)"
                  />
                </label>
                <label v-if="logic.dataType === 'enemy_count'">
                  <span>{{ t("game.ruleEnemyCountPosition") }}</span>
                  <el-select v-model="logic.enemyCountPosition" class="w-full">
                    <el-option v-for="option in enemyCountPositionOptions" :key="option.value" :value="option.value" :label="option.label" />
                  </el-select>
                </label>
                <div class="rule-condition__note">{{ t("game.rulePercentHint") }}</div>
              </template>

              <template v-else-if="logic.compareType === 'state'">
                <label>
                  <span>{{ t("game.ruleStateAction") }}</span>
                  <el-select v-model="logic.stateCompareAction" class="w-full">
                    <el-option v-for="option in stateActionOptions" :key="option.value" :value="option.value" :label="option.label" />
                  </el-select>
                </label>
                <label class="rule-condition__wide">
                  <span>{{ t("game.ruleStateId") }}</span>
                  <el-select v-model="logic.stateCompareStateId" class="w-full" filterable>
                    <el-option-group v-for="groupOption in groupedStateOptions" :key="groupOption.label" :label="groupOption.label">
                      <el-option v-for="option in groupOption.options" :key="option.value" :value="option.value" :label="option.label" />
                    </el-option-group>
                  </el-select>
                </label>
              </template>

              <template v-else-if="logic.compareType === 'stateStack'">
                <label class="rule-condition__wide">
                  <span>{{ t("game.ruleStateId") }}</span>
                  <el-select v-model="logic.stateCompareStateId" class="w-full" filterable>
                    <el-option-group v-for="groupOption in groupedStackStateOptions" :key="groupOption.label" :label="groupOption.label">
                      <el-option v-for="option in groupOption.options" :key="option.value" :value="option.value" :label="option.label" />
                    </el-option-group>
                  </el-select>
                </label>
                <label>
                  <span>{{ t("game.ruleCompareAction") }}</span>
                  <el-select v-model="logic.valueCompareAction" class="w-full">
                    <el-option v-for="option in compareActionOptions" :key="option.value" :value="option.value" :label="option.label" />
                  </el-select>
                </label>
                <label>
                  <span>{{ t("game.ruleConditionValue") }}</span>
                  <el-input-number
                    :model-value="numberValue(logic)"
                    class="w-full"
                    :controls="false"
                    :step="1"
                    step-strictly
                    @update:model-value="updateNumberValue(groupIndex, logicIndex, $event)"
                  />
                </label>
              </template>

              <template v-else-if="logic.compareType === 'unitExistState'">
                <label>
                  <span>{{ t("game.ruleUnitExistAction") }}</span>
                  <el-select v-model="logic.unitExistCompareAction" class="w-full">
                    <el-option v-for="option in unitExistActionOptions" :key="option.value" :value="option.value" :label="option.label" />
                  </el-select>
                </label>
                <label class="rule-condition__wide">
                  <span>{{ t("game.ruleUnitId") }}</span>
                  <el-select v-model="logic.unitId" class="w-full" filterable>
                    <el-option v-for="option in unitOptionsWithSelected(logic.unitId)" :key="option.value" :value="option.value" :label="option.label" />
                  </el-select>
                </label>
              </template>

              <template v-else-if="logic.compareType === 'cooldown'">
                <label>
                  <span>{{ t("game.ruleCooldownAction") }}</span>
                  <el-select v-model="logic.cooldownCompareAction" class="w-full">
                    <el-option v-for="option in cooldownActionOptions" :key="option.value" :value="option.value" :label="option.label" />
                  </el-select>
                </label>
                <label class="rule-condition__wide">
                  <span>{{ t("game.ruleCooldownSkills") }}</span>
                  <el-select v-model="logic.cooldownSkillIds" class="w-full" multiple filterable>
                    <el-option v-for="option in skillOptions" :key="option.value" :value="option.value" :label="option.label" />
                  </el-select>
                </label>
              </template>

              <template v-else-if="logic.compareType === 'itemCooldown'">
                <label>
                  <span>{{ t("game.ruleCooldownAction") }}</span>
                  <el-select v-model="logic.cooldownCompareAction" class="w-full">
                    <el-option v-for="option in cooldownActionOptions" :key="option.value" :value="option.value" :label="option.label" />
                  </el-select>
                </label>
                <label class="rule-condition__wide">
                  <span>{{ t("game.ruleCooldownItems") }}</span>
                  <el-select v-model="logic.cooldownItemIds" class="w-full" multiple filterable>
                    <el-option v-for="option in itemOptions" :key="option.value" :value="option.value" :label="option.label" />
                  </el-select>
                </label>
              </template>

              <template v-else-if="logic.compareType === 'special'">
                <label class="rule-condition__wide">
                  <span>{{ t("game.ruleSpecialAction") }}</span>
                  <el-select v-model="logic.specialAction" class="w-full">
                    <el-option v-for="option in specialActionOptions" :key="option.value" :value="option.value" :label="option.label" />
                  </el-select>
                </label>
              </template>
            </div>
          </article>

          <div class="rule-group__footer">
            <el-button size="small" @click="addCondition(groupIndex)" :disabled="group.conditions.length >= maxConditions">
              {{ t("game.ruleAddCondition") }}
            </el-button>
            <el-button size="small" type="danger" plain @click="removeGroup(groupIndex)">
              {{ t("game.ruleDeleteGroup") }}
            </el-button>
          </div>
        </div>
      </el-tab-pane>
    </el-tabs>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, watch } from "vue"
import { useI18n } from "vue-i18n"
import { listBuffDefinitions, getLocalizedBuffName } from "@services/buff-definition"
import { listEnemyDefinitions, getLocalizedEnemyName } from "@services/enemy-definition"
import {
  createDefaultRuleCondition,
  normalizeRuleGroups,
  type RuleCompareType,
  type RuleCondition,
  type RuleGroup
} from "@services/rule-logic"

type SelectOption = { value: string; label: string }
type OptionGroup = { label: string; options: SelectOption[] }

const props = withDefaults(
  defineProps<{
    modelValue: RuleGroup[]
    maxGroups?: number
    maxConditions?: number
    skillOptions?: SelectOption[]
    itemOptions?: SelectOption[]
  }>(),
  {
    maxGroups: 6,
    maxConditions: 6,
    skillOptions: () => [],
    itemOptions: () => []
  }
)

const emit = defineEmits<{
  "update:modelValue": [RuleGroup[]]
  validation: [payload: { valid: boolean; errors: string[] }]
}>()

const { t } = useI18n()
const activeTab = ref("0")
const expandedKeys = ref<string[]>([])
const groups = ref<RuleGroup[]>(normalizeEditableRuleGroups(props.modelValue))
const syncingFromProps = ref(false)

const validationErrors = computed(() => {
  const errors: string[] = []
  groups.value.forEach((group, groupIndex) => {
    group.conditions.forEach((_condition, conditionIndex) => {
      errors.push(...conditionErrors(groupIndex, conditionIndex))
    })
  })
  return errors
})

function ruleGroupsSignature(value: RuleGroup[]) {
  return JSON.stringify(normalizeEditableRuleGroups(value))
}

function normalizeEditableRuleGroups(value: unknown) {
  return normalizeRuleGroups(value, { allowEmpty: true })
}

watch(
  () => props.modelValue,
  (value) => {
    const normalized = normalizeEditableRuleGroups(value)
    if (ruleGroupsSignature(normalized) === ruleGroupsSignature(groups.value)) return
    syncingFromProps.value = true
    groups.value = normalized
  },
  { deep: true }
)

watch(
  groups,
  (value) => {
    if (syncingFromProps.value) {
      syncingFromProps.value = false
      emitValidation()
      return
    }
    const normalized = normalizeEditableRuleGroups(value)
    if (ruleGroupsSignature(normalized) !== ruleGroupsSignature(props.modelValue)) {
      emit("update:modelValue", normalized)
    }
    emitValidation()
  },
  { deep: true, immediate: true }
)

const compareTypeOptions = computed<SelectOption[]>(() => [
  { value: "value", label: t("game.ruleCompareTypeValue") },
  { value: "state", label: t("game.ruleCompareTypeState") },
  { value: "unitExistState", label: t("game.ruleCompareTypeUnitExistState") },
  { value: "stateStack", label: t("game.ruleCompareTypeStateStack") },
  { value: "cooldown", label: t("game.ruleCompareTypeCooldown") },
  { value: "itemCooldown", label: t("game.ruleCompareTypeItemCooldown") },
  { value: "special", label: t("game.ruleCompareTypeSpecial") }
])

const targetOptions = computed<SelectOption[]>(() => [
  { value: "self", label: t("game.ruleTargetSelf") },
  { value: "allies", label: t("game.ruleTargetAnyAlly") },
  { value: "enemies", label: t("game.ruleTargetAnyEnemy") },
  { value: "eachAllies", label: t("game.ruleTargetEachAlly") },
  { value: "eachEnemies", label: t("game.ruleTargetEachEnemy") },
  { value: "all", label: t("game.ruleTargetAnyUnit") },
  { value: "selfSummonedUnit", label: t("game.ruleTargetSelfSummonedUnit") },
  { value: "none", label: t("game.ruleTargetNone") }
])

const groupedDataTypeOptions = computed<OptionGroup[]>(() => [
  {
    label: t("game.ruleDataTypeCommon"),
    options: [
      { value: "hp", label: t("game.ruleFieldHp") },
      { value: "mp", label: t("game.ruleFieldMp") },
      { value: "lostHp", label: t("game.ruleFieldLostHp") },
      { value: "lostMp", label: t("game.ruleFieldLostMp") },
      { value: "totalHp", label: t("game.ruleFieldTotalHp") },
      { value: "count", label: t("game.ruleFieldCount") },
      { value: "buffCount", label: t("game.ruleFieldBuffCount") },
      { value: "debuffCount", label: t("game.ruleFieldDebuffCount") },
      { value: "passedTurn", label: t("game.ruleFieldPassedTurn") }
    ]
  },
  {
    label: t("game.ruleDataTypeLegacy"),
    options: [
      { value: "self_hp_percent", label: t("game.ruleFieldSelfHpPercent") },
      { value: "enemy_count", label: t("game.ruleFieldEnemyCount") },
      { value: "cooldown_ready", label: t("game.ruleFieldCooldownReady") },
      { value: "has_buff", label: t("game.ruleFieldHasBuff") },
      { value: "inventory_count", label: t("game.ruleFieldInventoryCount") },
      { value: "distance", label: t("game.ruleFieldDistance") },
      { value: "time_window", label: t("game.ruleFieldTimeWindow") },
      { value: "hour_of_day", label: t("game.ruleFieldHourOfDay") }
    ]
  }
])

const compareActionOptions = computed<SelectOption[]>(() => [
  { value: ">", label: ">" },
  { value: ">=", label: ">=" },
  { value: "<", label: "<" },
  { value: "<=", label: "<=" },
  { value: "=", label: "=" },
  { value: "!=", label: "!=" }
])

const enemyCountPositionOptions = computed<SelectOption[]>(() => [
  { value: "all", label: t("game.ruleEnemyCountPositionAll") },
  { value: "vanguard", label: t("game.ruleEnemyCountPositionVanguard") },
  { value: "midguard", label: t("game.ruleEnemyCountPositionMidguard") },
  { value: "rearguard", label: t("game.ruleEnemyCountPositionRearguard") }
])

const stateActionOptions = computed<SelectOption[]>(() => [
  { value: "someoneHasState", label: t("game.ruleStateActionSomeoneHasState") },
  { value: "AllHasState", label: t("game.ruleStateActionAllHasState") },
  { value: "someoneNotHasState", label: t("game.ruleStateActionSomeoneNotHasState") },
  { value: "allNotHasState", label: t("game.ruleStateActionAllNotHasState") }
])

const cooldownActionOptions = computed<SelectOption[]>(() => [
  { value: "allInCooldown", label: t("game.ruleCooldownActionAllIn") },
  { value: "allNotInCooldown", label: t("game.ruleCooldownActionAllNotIn") },
  { value: "someoneInCooldown", label: t("game.ruleCooldownActionSomeoneIn") },
  { value: "someoneNotInCooldown", label: t("game.ruleCooldownActionSomeoneNotIn") },
  { value: "inCooldown", label: t("game.ruleCooldownActionIn") },
  { value: "notInCooldown", label: t("game.ruleCooldownActionNotIn") }
])

const unitExistActionOptions = computed<SelectOption[]>(() => [
  { value: "exist", label: t("game.ruleUnitExistActionExist") },
  { value: "notExist", label: t("game.ruleUnitExistActionNotExist") }
])

const specialActionOptions = computed<SelectOption[]>(() => [
  { value: "enemyHasBoss", label: t("game.ruleSpecialActionEnemyHasBoss") },
  { value: "enemyNotHasBoss", label: t("game.ruleSpecialActionEnemyNotHasBoss") }
])

const allStateOptions = computed(() =>
  listBuffDefinitions()
    .map((buff) => ({
      value: buff.id,
      label: getLocalizedBuffName(t, buff.id, buff.id),
      kind: buff.kind,
      stackable: buff.stackPolicy === "stack"
    }))
    .sort((a, b) => a.label.localeCompare(b.label))
)

const groupedStateOptions = computed<OptionGroup[]>(() => buildStateGroups(allStateOptions.value))
const groupedStackStateOptions = computed<OptionGroup[]>(() => {
  const ordered = [...allStateOptions.value].sort((a, b) => Number(b.stackable) - Number(a.stackable) || a.label.localeCompare(b.label))
  return buildStateGroups(ordered)
})

const unitOptions = computed<SelectOption[]>(() =>
  listEnemyDefinitions()
    .map((enemy) => ({ value: enemy.id, label: getLocalizedEnemyName(t, enemy.id, enemy.id) }))
    .sort((a, b) => a.label.localeCompare(b.label))
)

function buildStateGroups(options: Array<SelectOption & { kind: string }>): OptionGroup[] {
  const buff = options.filter((option) => option.kind === "buff")
  const debuff = options.filter((option) => option.kind === "debuff")
  const other = options.filter((option) => option.kind !== "buff" && option.kind !== "debuff")
  return [
    { label: t("game.ruleStateGroupBuff"), options: buff },
    { label: t("game.ruleStateGroupDebuff"), options: debuff },
    { label: t("game.ruleStateGroupOther"), options: other }
  ].filter((group) => group.options.length > 0)
}

function emitValidation() {
  const errors = validationErrors.value
  emit("validation", { valid: errors.length === 0, errors })
}

function expandedKey(groupIndex: number, conditionIndex: number) {
  return `${groupIndex}:${conditionIndex}`
}

function isExpanded(groupIndex: number, conditionIndex: number) {
  return expandedKeys.value.includes(expandedKey(groupIndex, conditionIndex))
}

function toggleExpanded(groupIndex: number, conditionIndex: number) {
  const key = expandedKey(groupIndex, conditionIndex)
  expandedKeys.value = expandedKeys.value.includes(key)
    ? expandedKeys.value.filter((item) => item !== key)
    : [...expandedKeys.value, key]
}

function addGroup() {
  if (groups.value.length >= props.maxGroups) return
  groups.value = [...groups.value, { conditions: [createDefaultRuleCondition()] }]
  activeTab.value = String(groups.value.length - 1)
  expandedKeys.value = [expandedKey(groups.value.length - 1, 0)]
}

function removeGroup(groupIndex: number) {
  const next = groups.value.filter((_, idx) => idx !== groupIndex)
  groups.value = next
  activeTab.value = String(Math.max(0, Math.min(groupIndex, next.length - 1)))
}

function addCondition(groupIndex: number) {
  const next = [...groups.value]
  if (!next[groupIndex] || next[groupIndex].conditions.length >= props.maxConditions) return
  next[groupIndex] = { conditions: [...next[groupIndex].conditions, createDefaultRuleCondition()] }
  groups.value = next
  expandedKeys.value = [expandedKey(groupIndex, next[groupIndex].conditions.length - 1)]
}

function removeCondition(groupIndex: number, conditionIndex: number) {
  const next = [...groups.value]
  if (!next[groupIndex]) return
  const conditions = next[groupIndex].conditions.filter((_, idx) => idx !== conditionIndex)
  if (conditions.length > 0) next[groupIndex] = { conditions }
  else next.splice(groupIndex, 1)
  groups.value = next
  activeTab.value = String(Math.max(0, Math.min(groupIndex, next.length - 1)))
}

function onCompareTypeChange(groupIndex: number, conditionIndex: number) {
  const next = [...groups.value]
  if (!next[groupIndex]?.conditions[conditionIndex]) return
  const current = next[groupIndex].conditions[conditionIndex]
  const compareType = current.compareType as RuleCompareType
  const replaced = {
    ...createDefaultRuleCondition(compareType),
    target: current.target || createDefaultRuleCondition(compareType).target
  }
  if (compareType === "state" && !replaced.stateCompareStateId) replaced.stateCompareStateId = firstStateID("debuff")
  if (compareType === "stateStack" && !replaced.stateCompareStateId) replaced.stateCompareStateId = firstStackStateID()
  next[groupIndex].conditions[conditionIndex] = replaced
  groups.value = next
}

function onDataTypeChange(groupIndex: number, conditionIndex: number) {
  const next = [...groups.value]
  const condition = next[groupIndex]?.conditions[conditionIndex]
  if (!condition) return
  if (condition.dataType === "count" && !condition.valueCompareAction) condition.valueCompareAction = ">="
  groups.value = next
}

function applyTemplate(kind: "heal" | "self" | "execute" | "boss" | "cleanse") {
  let condition: RuleCondition
  if (kind === "boss") {
    condition = { compareType: "special", target: "enemies", specialAction: "enemyHasBoss" }
  } else if (kind === "cleanse") {
    condition = {
      compareType: "state",
      target: "allies",
      stateCompareAction: "someoneHasState",
      stateCompareStateId: firstStateID("debuff")
    }
  } else {
    condition = createDefaultRuleCondition()
    condition.dataType = "hp"
    condition.valueCompareAction = "<="
    condition.target = kind === "self" ? "self" : kind === "execute" ? "enemies" : "allies"
    condition.valueConditionValue = kind === "self" ? "50%" : kind === "execute" ? "30%" : "70%"
  }
  groups.value = [{ conditions: [condition] }]
  activeTab.value = "0"
  expandedKeys.value = [expandedKey(0, 0)]
}

function firstStateID(kind: "buff" | "debuff") {
  return allStateOptions.value.find((option) => option.kind === kind)?.value || allStateOptions.value[0]?.value || ""
}

function firstStackStateID() {
  return allStateOptions.value.find((option) => option.stackable)?.value || allStateOptions.value[0]?.value || ""
}

function stringValue(condition: RuleCondition): string {
  const raw = condition.valueConditionValue
  if (typeof raw === "number") return String(raw)
  return String(raw || "")
}

function updateStringValue(groupIndex: number, conditionIndex: number, value: string) {
  const next = [...groups.value]
  if (!next[groupIndex]?.conditions[conditionIndex]) return
  next[groupIndex].conditions[conditionIndex].valueConditionValue = value
  groups.value = next
}

function numberValue(condition: RuleCondition): number {
  const raw = Number(condition.valueConditionValue || 0)
  return Number.isFinite(raw) ? raw : 0
}

function updateNumberValue(groupIndex: number, conditionIndex: number, value: number | undefined) {
  const next = [...groups.value]
  if (!next[groupIndex]?.conditions[conditionIndex]) return
  next[groupIndex].conditions[conditionIndex].valueConditionValue = Number.isFinite(Number(value)) ? Number(value) : 0
  groups.value = next
}

function summarizeCondition(condition: RuleCondition): string {
  const target = optionLabel(targetOptions.value, condition.target)
  if (condition.compareType === "state") {
    return [target, optionLabel(stateActionOptions.value, condition.stateCompareAction), stateLabel(condition.stateCompareStateId)].join(" · ")
  }
  if (condition.compareType === "stateStack") {
    return [target, stateLabel(condition.stateCompareStateId), optionLabel(compareActionOptions.value, condition.valueCompareAction), stringValue(condition)].join(" · ")
  }
  if (condition.compareType === "unitExistState") {
    return [target, optionLabel(unitExistActionOptions.value, condition.unitExistCompareAction), unitLabel(condition.unitId)].join(" · ")
  }
  if (condition.compareType === "cooldown") {
    return [target, optionLabel(cooldownActionOptions.value, condition.cooldownCompareAction), listLabels(props.skillOptions, condition.cooldownSkillIds)].join(" · ")
  }
  if (condition.compareType === "itemCooldown") {
    return [target, optionLabel(cooldownActionOptions.value, condition.cooldownCompareAction), listLabels(props.itemOptions, condition.cooldownItemIds)].join(" · ")
  }
  if (condition.compareType === "special") {
    return [target, optionLabel(specialActionOptions.value, condition.specialAction)].join(" · ")
  }
  if (condition.dataType === "enemy_count") {
    return [
      target,
      dataTypeLabel(condition.dataType),
      optionLabel(enemyCountPositionOptions.value, condition.enemyCountPosition),
      optionLabel(compareActionOptions.value, condition.valueCompareAction),
      stringValue(condition)
    ].join(" · ")
  }
  return [target, dataTypeLabel(condition.dataType), optionLabel(compareActionOptions.value, condition.valueCompareAction), stringValue(condition)].join(" · ")
}

function dataTypeLabel(value: string | undefined) {
  for (const groupOption of groupedDataTypeOptions.value) {
    const found = groupOption.options.find((option) => option.value === value)
    if (found) return found.label
  }
  return value || t("game.ruleUnknownValue")
}

function stateLabel(value: string | undefined) {
  const found = allStateOptions.value.find((option) => option.value === value)
  return found?.label || value || t("game.ruleUnselected")
}

function unitLabel(value: string | undefined) {
  const found = unitOptions.value.find((option) => option.value === value)
  return found?.label || value || t("game.ruleUnselected")
}

function optionLabel(options: SelectOption[], value: string | undefined) {
  return options.find((option) => option.value === value)?.label || value || t("game.ruleUnselected")
}

function listLabels(options: SelectOption[], values: string[] | undefined) {
  const selected = Array.isArray(values) ? values : []
  if (selected.length === 0) return t("game.ruleUnselected")
  return selected.map((value) => optionLabel(options, value)).join(" / ")
}

function unitOptionsWithSelected(selected: string | undefined) {
  if (!selected || unitOptions.value.some((option) => option.value === selected)) return unitOptions.value
  return [{ value: selected, label: selected }, ...unitOptions.value]
}

function conditionErrors(groupIndex: number, conditionIndex: number): string[] {
  const condition = groups.value[groupIndex]?.conditions[conditionIndex]
  if (!condition) return []
  const prefix = t("game.ruleErrorLocation", { group: groupIndex + 1, index: conditionIndex + 1 })
  const errors: string[] = []
  if (!condition.compareType) errors.push(`${prefix}${t("game.ruleErrorCompareTypeRequired")}`)
  if (condition.compareType === "value") {
    if (!condition.dataType) errors.push(`${prefix}${t("game.ruleErrorDataTypeRequired")}`)
    const value = stringValue(condition).trim()
    if (!value) errors.push(`${prefix}${t("game.ruleErrorValueRequired")}`)
    else if (!/^-?\d+(\.\d+)?%?$/.test(value)) errors.push(`${prefix}${t("game.ruleErrorValueFormat")}`)
  }
  if ((condition.compareType === "state" || condition.compareType === "stateStack") && !String(condition.stateCompareStateId || "").trim()) {
    errors.push(`${prefix}${t("game.ruleErrorStateRequired")}`)
  }
  if (condition.compareType === "unitExistState" && !String(condition.unitId || "").trim()) {
    errors.push(`${prefix}${t("game.ruleErrorUnitRequired")}`)
  }
  if (condition.compareType === "cooldown" && (!Array.isArray(condition.cooldownSkillIds) || condition.cooldownSkillIds.length === 0)) {
    errors.push(`${prefix}${t("game.ruleErrorSkillRequired")}`)
  }
  if (condition.compareType === "itemCooldown" && (!Array.isArray(condition.cooldownItemIds) || condition.cooldownItemIds.length === 0)) {
    errors.push(`${prefix}${t("game.ruleErrorItemRequired")}`)
  }
  return errors
}
</script>

<style scoped>
.rule-builder {
  display: grid;
  gap: 12px;
}

.rule-builder__toolbar {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.rule-builder__empty {
  border: 1px dashed var(--app-border);
  border-radius: 8px;
  color: var(--app-muted);
  font-size: 12px;
  padding: 12px;
}

.rule-group {
  display: grid;
  gap: 10px;
}

.rule-group__hint {
  color: var(--app-muted);
  font-size: 12px;
}

.rule-condition {
  border: 1px solid var(--app-border);
  border-radius: 8px;
  padding: 10px;
}

.rule-condition--invalid {
  border-color: var(--el-color-danger);
}

.rule-condition__header,
.rule-condition__actions,
.rule-group__footer {
  align-items: center;
  display: flex;
  gap: 8px;
}

.rule-condition__header {
  justify-content: space-between;
}

.rule-condition__summary {
  align-items: center;
  background: transparent;
  border: 0;
  color: var(--app-text);
  cursor: pointer;
  display: flex;
  flex: 1;
  gap: 8px;
  min-width: 0;
  padding: 0;
  text-align: left;
}

.rule-condition__index {
  align-items: center;
  background: var(--app-surface-soft);
  border-radius: 999px;
  display: inline-flex;
  flex: 0 0 24px;
  font-size: 12px;
  height: 24px;
  justify-content: center;
}

.rule-condition__text {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.rule-condition__errors {
  color: var(--el-color-danger);
  font-size: 12px;
  margin-top: 8px;
}

.rule-condition__editor {
  display: grid;
  gap: 10px;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  margin-top: 10px;
}

.rule-condition__editor label {
  display: grid;
  gap: 4px;
}

.rule-condition__editor label span,
.rule-condition__note {
  color: var(--app-muted);
  font-size: 12px;
}

.rule-condition__wide,
.rule-condition__note {
  grid-column: 1 / -1;
}

@media (max-width: 720px) {
  .rule-condition__header,
  .rule-condition__actions {
    align-items: stretch;
    flex-direction: column;
  }

  .rule-condition__editor {
    grid-template-columns: 1fr;
  }
}
</style>
