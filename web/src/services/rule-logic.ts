export type RuleCompareType = "value" | "state" | "stateStack" | "unitExistState" | "cooldown" | "itemCooldown" | "special"

export type RuleCondition = {
  compareType: RuleCompareType
  target?: string
  enemyCountPosition?: string
  dataType?: string
  valueCompareAction?: string
  valueConditionValue?: string | number
  stateCompareAction?: string
  stateCompareStateId?: string
  cooldownCompareAction?: string
  cooldownSkillIds?: string[]
  cooldownItemIds?: string[]
  specialAction?: string
  unitExistCompareAction?: string
  unitId?: string
  field?: string
  op?: string
  value?: number
}

export type RuleGroup = {
  conditions: RuleCondition[]
}

export function createDefaultRuleCondition(compareType: RuleCompareType = "value"): RuleCondition {
  if (compareType === "state") {
    return {
      compareType: "state",
      target: "self",
      stateCompareAction: "someoneHasState",
      stateCompareStateId: ""
    }
  }
  if (compareType === "stateStack") {
    return {
      compareType: "stateStack",
      target: "self",
      stateCompareStateId: "",
      valueCompareAction: ">=",
      valueConditionValue: 1
    }
  }
  if (compareType === "unitExistState") {
    return {
      compareType: "unitExistState",
      target: "all",
      unitExistCompareAction: "exist",
      unitId: ""
    }
  }
  if (compareType === "cooldown") {
    return {
      compareType: "cooldown",
      target: "self",
      cooldownCompareAction: "allNotInCooldown",
      cooldownSkillIds: []
    }
  }
  if (compareType === "itemCooldown") {
    return {
      compareType: "itemCooldown",
      target: "self",
      cooldownCompareAction: "allNotInCooldown",
      cooldownItemIds: []
    }
  }
  if (compareType === "special") {
    return {
      compareType: "special",
      target: "enemies",
      specialAction: "enemyHasBoss"
    }
  }
  return {
    compareType: "value",
    target: "self",
    enemyCountPosition: "all",
    dataType: "hp",
    valueCompareAction: "<=",
    valueConditionValue: "50%"
  }
}

function normalizeLegacyCondition(raw: Record<string, unknown>): RuleCondition | null {
  const field = String(raw.field || "").trim()
  const op = String(raw.op || "").trim()
  const value = Number(raw.value ?? 0)
  if (!field || !op || !Number.isFinite(value)) return null
  return {
    compareType: "value",
    target: "self",
    dataType: field,
    valueCompareAction: op,
    valueConditionValue: value,
    field,
    op,
    value
  }
}

function normalizeRuleCondition(raw: unknown): RuleCondition | null {
  if (!raw || typeof raw !== "object") return null
  const row = raw as Record<string, unknown>
  const compareTypeRaw = String(row.compareType || "").trim()
  if (!compareTypeRaw) {
    return normalizeLegacyCondition(row)
  }
  const compareType = compareTypeRaw as RuleCompareType
  const base = createDefaultRuleCondition(compareType)
  const merged: RuleCondition = {
    ...base,
    ...row,
    compareType,
    target: String(row.target || base.target || "").trim() || base.target,
    enemyCountPosition: String(row.enemyCountPosition || base.enemyCountPosition || "").trim() || base.enemyCountPosition,
    dataType: String(row.dataType || base.dataType || "").trim() || base.dataType,
    valueCompareAction: String(row.valueCompareAction || row.op || base.valueCompareAction || "").trim() || base.valueCompareAction,
    stateCompareAction: String(row.stateCompareAction || base.stateCompareAction || "").trim() || base.stateCompareAction,
    stateCompareStateId: String(row.stateCompareStateId || base.stateCompareStateId || "").trim(),
    cooldownCompareAction: String(row.cooldownCompareAction || base.cooldownCompareAction || "").trim() || base.cooldownCompareAction,
    specialAction: String(row.specialAction || base.specialAction || "").trim() || base.specialAction,
    unitExistCompareAction: String(row.unitExistCompareAction || base.unitExistCompareAction || "").trim() || base.unitExistCompareAction,
    unitId: String(row.unitId || base.unitId || "").trim(),
    cooldownSkillIds: Array.isArray(row.cooldownSkillIds)
      ? row.cooldownSkillIds.map((item) => String(item || "").trim()).filter(Boolean)
      : (base.cooldownSkillIds || []),
    cooldownItemIds: Array.isArray(row.cooldownItemIds)
      ? row.cooldownItemIds.map((item) => String(item || "").trim()).filter(Boolean)
      : (base.cooldownItemIds || [])
  }
  if (compareType === "value") {
    const rawValue = row.valueConditionValue
    if (typeof rawValue === "string") {
      merged.valueConditionValue = rawValue.trim() || base.valueConditionValue
    } else if (typeof rawValue === "number" && Number.isFinite(rawValue)) {
      merged.valueConditionValue = rawValue
    } else if (typeof row.value === "number" && Number.isFinite(row.value)) {
      merged.valueConditionValue = row.value
    } else {
      merged.valueConditionValue = base.valueConditionValue
    }
    const numericLegacy = Number(merged.valueConditionValue)
    if (Number.isFinite(numericLegacy)) {
      merged.value = numericLegacy
    }
  }
  return merged
}

export function normalizeRuleGroups(raw: unknown, options: { allowEmpty?: boolean } = {}): RuleGroup[] {
  if (!Array.isArray(raw)) {
    if (options.allowEmpty) return []
    return [{ conditions: [createDefaultRuleCondition()] }]
  }
  const groups = raw
    .map((group) => {
      if (!group || typeof group !== "object") return null
      const conditionsRaw = Array.isArray((group as { conditions?: unknown }).conditions)
        ? (group as { conditions: unknown[] }).conditions
        : []
      const conditions = conditionsRaw
        .map((condition) => normalizeRuleCondition(condition))
        .filter((condition): condition is RuleCondition => Boolean(condition))
      if (conditions.length === 0) return null
      return { conditions }
    })
    .filter((group): group is RuleGroup => Boolean(group))

  if (groups.length === 0) {
    if (options.allowEmpty) return []
    return [{ conditions: [createDefaultRuleCondition()] }]
  }
  return groups
}
