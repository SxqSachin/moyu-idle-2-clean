import type { RuleGroup } from "@services/rule-logic"

export type EquipmentSkillConfig = {
  version: number
  revision: number
  enabled: boolean
  skillIds: string[]
  allowStandalone: boolean
  beforeSkillId?: string
  groups?: RuleGroup[]
}
export type EquipmentSkillView = {
  skillId: string
  itemId: string
  enhanceLevel: number
  capacity: number
  config: EquipmentSkillConfig
  reason?: string
}
export const emptyEquipmentSkillConfig = (): EquipmentSkillConfig => ({ version: 1, revision: 0, enabled: false, skillIds: [], allowStandalone: false })
export function cloneEquipmentSkillConfig(value: EquipmentSkillConfig): EquipmentSkillConfig {
  return structuredClone(value)
}
export function equipmentCarriedSkillIDs(carried: string[], catalog: { id: string; eligible?: boolean }[], draftMode = false): Set<string> {
  // Draft loadouts may use a different weapon/status setup than the live character.
  if (draftMode) return new Set(carried)
  const unavailable = new Set(catalog.filter(row => row.eligible === false).map(row => row.id))
  return new Set(carried.filter(id => !unavailable.has(id)))
}
export function equipmentComboPriorityAnchor(anchor: string | undefined, carried: string[]): string {
  return anchor && carried.includes(anchor) ? anchor : ""
}
export function moveEquipmentComboSkill(ids: string[], index: number, offset: number): string[] {
  const result = [...ids]
  const target = index + offset
  if (index < 0 || index >= ids.length || target < 0 || target >= ids.length) return result
  ;[result[index], result[target]] = [result[target], result[index]]
  return result
}
export function equipmentSelectionReason(view: EquipmentSkillView, cfg: EquipmentSkillConfig, candidates: Set<string>, carried: Set<string>): string {
  if (cfg.skillIds.length > view.capacity) return "CAPACITY_EXCEEDED"
  const combo = view.skillId === "equipment_martial_combo"
  if (cfg.skillIds.length < (combo ? 2 : 1)) return "NOT_CONFIGURED"
  if (new Set(cfg.skillIds).size !== cfg.skillIds.length) return "INVALID_CONFIG"
  if (cfg.skillIds.some(id => !candidates.has(id) || (combo && !carried.has(id)))) return combo ? "SKILL_NOT_CARRIED_OR_PHYSICAL" : "SKILL_NOT_INTERRUPTIBLE"
  return ""
}
