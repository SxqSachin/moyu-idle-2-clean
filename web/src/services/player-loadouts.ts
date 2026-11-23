import { apiGet, apiPost } from "@services/api"
import type { EquipmentSkillConfig } from "@services/equipment-skill-config"

export type LoadoutEquipmentSlot = {
  slot: string
  instanceId: number
}

export type LoadoutSkillRule = {
  skillId: string
  enabled: boolean
  groups: unknown
}

export type BattleLoadout = {
  equipmentSkills?: Record<string, EquipmentSkillConfig>
  id: string
  name: string
  equipment: LoadoutEquipmentSlot[]
  skillSlots: string[]
  skillRules: LoadoutSkillRule[]
}

export type WorkLoadout = {
  id: string
  name: string
  equipment: LoadoutEquipmentSlot[]
}

export type PlayerLoadoutsPayload = {
  battleLoadouts: BattleLoadout[]
  workLoadouts: WorkLoadout[]
  workActionBindings: Record<string, string>
}

export type BattleLoadoutApplyResult = {
  ok: boolean
  skippedSkills?: Array<{
    skillId: string
    reason: string
  }>
}

export async function fetchPlayerLoadouts(token: string) {
  return apiGet<PlayerLoadoutsPayload>("/api/player/loadouts", token)
}

export async function upsertBattleLoadout(token: string, loadout: BattleLoadout) {
  return apiPost<PlayerLoadoutsPayload>("/api/player/loadouts/battle/upsert", token, { loadout })
}

export async function upsertWorkLoadout(token: string, loadout: WorkLoadout) {
  return apiPost<PlayerLoadoutsPayload>("/api/player/loadouts/work/upsert", token, { loadout })
}

export async function applyBattleLoadout(token: string, loadoutId: string) {
  return apiPost<BattleLoadoutApplyResult>("/api/player/loadouts/battle/apply", token, { loadoutId })
}

export async function applyWorkLoadout(token: string, loadoutId: string) {
  return apiPost<{ ok: boolean }>("/api/player/loadouts/work/apply", token, { loadoutId })
}

export async function updateWorkLoadoutBindings(token: string, bindings: Record<string, string>) {
  return apiPost<PlayerLoadoutsPayload>("/api/player/loadouts/work/bind-actions", token, { bindings })
}

export async function deletePlayerLoadout(token: string, type: "battle" | "work", loadoutId: string) {
  return apiPost<PlayerLoadoutsPayload>(`/api/player/loadouts/${type}/delete`, token, { loadoutId })
}
