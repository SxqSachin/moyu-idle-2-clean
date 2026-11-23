import { beforeEach, describe, expect, it } from "vitest"
import { createPinia, setActivePinia } from "pinia"
import { useSkillQuickRefDialogStore } from "../src/stores/skill-quick-ref-dialog"

describe("skill quick ref dialog store", () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

  it("stores runtime skill payload when opening dialog", () => {
    const store = useSkillQuickRefDialogStore()
    const runtimeSkill = {
      id: "fireball",
      targeting: { targetCount: { base: 2, formula: [] } },
      effects: [{ kind: "damage", damageType: "magical" }]
    }

    store.open({
      skillId: "fireball",
      unitSkillLevel: 3,
      runtimeSkill
    })

    expect(store.visible).toBe(true)
    expect(store.skillId).toBe("fireball")
    expect(store.unitSkillLevel).toBe(3)
    expect(store.runtimeSkill).toEqual(runtimeSkill)
  })

  it("clears runtime skill payload when closing dialog", () => {
    const store = useSkillQuickRefDialogStore()
    store.open({
      skillId: "fireball",
      runtimeSkill: { id: "fireball", effects: [{ kind: "damage" }] }
    })
    store.close()
    expect(store.visible).toBe(false)
    expect(store.runtimeSkill).toBeNull()
  })
})
