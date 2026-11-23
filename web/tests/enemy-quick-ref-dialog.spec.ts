import { beforeEach, describe, expect, it } from "vitest"
import { createPinia, setActivePinia } from "pinia"
import { useEnemyUnitQuickRefDialogStore } from "../src/stores/enemy-unit-quick-ref-dialog"

describe("enemy unit quick ref dialog store", () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

  it("keeps the dungeon difficulty context with the selected enemy", () => {
    const store = useEnemyUnitQuickRefDialogStore()
    store.open({
      enemyId: "parade_king_boss_001",
      difficultyContext: {
        worldId: "amusement_park_001",
        worldLabel: "猫咪游乐园",
        worldDifficultySettings: {
          maxLevel: 10,
          statBonuses: [{ attrKey: "hp", base: 1402.5, perLevelMult: 1.4 }],
          specialDrops: []
        }
      }
    })

    expect(store.visible).toBe(true)
    expect(store.enemyId).toBe("parade_king_boss_001")
    expect(store.difficultyContext).toMatchObject({
      worldId: "amusement_park_001",
      worldLabel: "猫咪游乐园",
      worldDifficultySettings: { maxLevel: 10 }
    })
  })

  it("does not leak a previous dungeon context into unrelated quick refs", () => {
    const store = useEnemyUnitQuickRefDialogStore()
    store.open({
      enemyId: "parade_king_boss_001",
      difficultyContext: {
        worldId: "amusement_park_001",
        worldLabel: "猫咪游乐园",
        worldDifficultySettings: null
      }
    })
    store.open({ enemyId: "plain_mouse_001" })

    expect(store.enemyId).toBe("plain_mouse_001")
    expect(store.difficultyContext).toBeNull()
  })
})
