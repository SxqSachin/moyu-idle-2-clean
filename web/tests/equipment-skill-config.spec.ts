import { describe, expect, it } from "vitest"
import { reactive, toRaw } from "vue"
import { cloneEquipmentSkillConfig, emptyEquipmentSkillConfig, equipmentSelectionReason, equipmentCarriedSkillIDs, equipmentComboPriorityAnchor, moveEquipmentComboSkill, type EquipmentSkillView } from "../src/services/equipment-skill-config"

describe("equipment skill configuration", () => {
  const view: EquipmentSkillView = {skillId:"equipment_martial_combo",itemId:"hundred_battle_badge",enhanceLevel:0,capacity:2,config:{...emptyEquipmentSkillConfig(),enabled:true,skillIds:["a","b"]}}
  it("does not apply live weapon eligibility to another loadout draft", () => {
    const catalog = [{id:"a",eligible:false},{id:"b",eligible:true},{id:"c",eligible:true}]
    const live = equipmentCarriedSkillIDs(["a","b"], catalog)
    const draft = equipmentCarriedSkillIDs(["a","b"], catalog, true)
    expect([...live]).toEqual(["b"])
    expect([...draft]).toEqual(["a","b"])
    expect(draft.has("c")).toBe(false)
    expect(equipmentSelectionReason(view,view.config,new Set(["a","b"]),draft)).toBe("")
    expect(equipmentSelectionReason(view,view.config,new Set(["a","b"]),live)).toBe("SKILL_NOT_CARRIED_OR_PHYSICAL")
  })
  it("shows first priority when the saved anchor was removed", () => {
    expect(equipmentComboPriorityAnchor("removed", ["a","b"])).toBe("")
    expect(equipmentComboPriorityAnchor("b", ["a","b"])).toBe("b")
    expect(equipmentComboPriorityAnchor(undefined, ["a","b"])).toBe("")
  })
  it("clones reactive loadout drafts without mutating the saved selection", () => {
    const source = reactive(view.config)
    const draft = cloneEquipmentSkillConfig(toRaw(source))
    draft.skillIds.reverse()
    expect(source.skillIds).toEqual(["a","b"])
    expect(draft.skillIds).toEqual(["b","a"])
  })
  it("keeps stable ordering and bounds mobile move controls", () => {
    expect(moveEquipmentComboSkill(["a","b","c"],1,-1)).toEqual(["b","a","c"])
    expect(moveEquipmentComboSkill(["a","b"],0,-1)).toEqual(["a","b"])
    expect(moveEquipmentComboSkill(["a","b"],1,1)).toEqual(["a","b"])
  })
  it("reports invalidation without silently truncating a downgraded loadout", () => {
    const candidates = new Set(["a","b","c"])
    const cfg = {...view.config,skillIds:["a","b","c"]}
    expect(equipmentSelectionReason(view,cfg,candidates,candidates)).toBe("CAPACITY_EXCEEDED")
    expect(cfg.skillIds).toHaveLength(3)
    expect(equipmentSelectionReason(view,view.config,candidates,new Set(["a"]))).toBe("SKILL_NOT_CARRIED_OR_PHYSICAL")
    expect(equipmentSelectionReason(view,view.config,candidates,candidates)).toBe("")
    expect(equipmentSelectionReason(view,{...cfg,skillIds:["a","a"]},candidates,candidates)).toBe("INVALID_CONFIG")
  })
})
