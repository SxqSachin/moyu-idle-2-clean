import { getGameDataChunk } from "@services/game-data-runtime"
import type { ActionDefinition, ActionMainTypeDefinition } from "@game-data/game-data"

function isActionDefinition(value: unknown): value is ActionDefinition {
  if (!value || typeof value !== "object") return false
  const row = value as Record<string, unknown>
  return (
    typeof row.id === "string" &&
    typeof row.mainType === "string" &&
    typeof row.subType === "string" &&
    typeof row.menu === "string" &&
    typeof row.nameKey === "string" &&
    typeof row.queueable === "boolean" &&
    typeof row.durationS === "number" &&
    !!row.costs &&
    typeof row.costs === "object" &&
    !!row.itemReward &&
    typeof row.itemReward === "object" &&
    !!row.statusExpReward &&
    typeof row.statusExpReward === "object"
  )
}

function getRawActions(catalog: unknown): unknown[] {
  if (!catalog || typeof catalog !== "object") return []
  const actions = (catalog as { actions?: unknown }).actions
  return Array.isArray(actions) ? actions : []
}

function parseBoolean(raw: unknown, fallback = false): boolean {
  if (typeof raw === "boolean") return raw
  if (typeof raw === "number") return raw !== 0
  if (typeof raw === "string") {
    const normalized = raw.trim().toLowerCase()
    if (normalized === "true" || normalized === "1") return true
    if (normalized === "false" || normalized === "0") return false
  }
  return fallback
}

function isActionDisabled(action: ActionDefinition): boolean {
  return parseBoolean((action as Partial<ActionDefinition> & { disabled?: unknown }).disabled, false)
}

const SPECIAL_QUEUE_ACTION_IDS = new Set([
  "fishing.trip",
  "equipment_enhance",
  "building_upgrade",
  "alchemy.convert",
  "alchemy.bench",
  "alchemy.inherit",
  "alchemy.transmute",
  "alchemy.smelt"
])

export function isSpecialQueueAction(actionId: string): boolean {
  return SPECIAL_QUEUE_ACTION_IDS.has(String(actionId || "").trim())
}

export function isGenericActionVisible(action: ActionDefinition | undefined): action is ActionDefinition {
  return !!action && !isSpecialQueueAction(action.id)
}

function getRawActionMainTypes(catalog: unknown): unknown[] {
  if (!catalog || typeof catalog !== "object") return []
  const rows = (catalog as { action_main_types?: unknown }).action_main_types
  return Array.isArray(rows) ? rows : []
}

function isActionMainTypeDefinition(value: unknown): value is ActionMainTypeDefinition {
  if (!value || typeof value !== "object") return false
  const row = value as Record<string, unknown>
  return (
    typeof row.id === "string" &&
    typeof row.nameKey === "string" &&
    typeof row.icon === "string" &&
    typeof row.color === "string" &&
    typeof row.order === "number" &&
    Array.isArray(row.actions)
  )
}

let _actionDefinitions: ActionDefinition[] | null = null
let _actionDefinitionMap: Map<string, ActionDefinition> | null = null
let _visibleActionIDSet: Set<string> | null = null
let _actionMainTypeDefinitions: ActionMainTypeDefinition[] | null = null

function ensureActionDefinitions() {
  if (_actionDefinitions) return
  const allActions = getRawActions(getGameDataChunk("actions"))
    .filter((action): action is ActionDefinition => isActionDefinition(action))
  _actionDefinitions = allActions.filter((action) => !isActionDisabled(action))
  _actionDefinitionMap = new Map<string, ActionDefinition>(_actionDefinitions.map((action) => [action.id, action]))
  _visibleActionIDSet = new Set<string>(allActions.filter(isGenericActionVisible).map((action) => action.id))
}

function ensureActionMainTypeDefinitions() {
  if (_actionMainTypeDefinitions) return
  ensureActionDefinitions()
  _actionMainTypeDefinitions = getRawActionMainTypes(getGameDataChunk("action_main_types"))
    .filter((row): row is ActionMainTypeDefinition => isActionMainTypeDefinition(row))
    .map((row) => {
      const actions = row.actions.filter((actionID) => _visibleActionIDSet!.has(actionID))
      return {
        ...row,
        actions
      }
    })
    .filter((row) => row.actions.length > 0)
    .sort((a, b) => (a.order - b.order) || a.id.localeCompare(b.id))
}

export function listActionDefinitions(): ActionDefinition[] {
  ensureActionDefinitions()
  return _actionDefinitions!.filter(isGenericActionVisible)
}

export function getActionDefinition(actionId: string): ActionDefinition | undefined {
  ensureActionDefinitions()
  return _actionDefinitionMap!.get(actionId)
}

export function listActionMainTypeDefinitions(): ActionMainTypeDefinition[] {
  ensureActionMainTypeDefinitions()
  if (_actionMainTypeDefinitions!.length > 0) return _actionMainTypeDefinitions!
  const actions = listActionDefinitions()
  const byType = new Set<string>()
  for (const action of actions) {
    const mainType = String(action.mainType || action.menu || "").trim()
    if (mainType) byType.add(mainType)
  }
  return Array.from(byType)
    .sort((a, b) => a.localeCompare(b))
    .map((id, index) => ({
      id,
      nameKey: `action_main_type.${id}.name`,
      descKey: `action_main_type.${id}.desc`,
      icon: "",
      color: "#333",
      order: index,
      actions: actions
        .filter((action) => String(action.mainType || action.menu || "").trim() === id)
        .map((action) => action.id)
    }))
}
