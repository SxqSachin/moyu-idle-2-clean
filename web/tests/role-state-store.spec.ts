import { beforeEach, describe, expect, it } from "vitest"
import { createPinia, setActivePinia } from "pinia"
import { useRoleStateStore } from "../src/stores/role-state"

describe("role state store", () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

  it("patches player attributes with version guards", () => {
    const store = useRoleStateStore()

    store.patchRoleAttributesPayload({
      roleType: "player",
      roleId: "self",
      upsert: {
        intelligence: { value: 10, category: "battle", unit: "number", growthType: "static", order: 1 }
      },
      attributeProgressUpsert: {
        intelligence: { value: 10, level: 1, exp: 0, nextExp: 100 }
      },
      meta: { version: 2 }
    })

    expect(store.roleUnitAttributes("player:self").intelligence).toBe(10)
    expect(store.roles["player:self"].attributeProgress.intelligence.level).toBe(1)

    store.patchRoleAttributesPayload({
      roleType: "player",
      roleId: "self",
      upsert: {
        intelligence: { value: 1, category: "battle", unit: "number", growthType: "static", order: 1 }
      },
      meta: { version: 1 }
    })

    expect(store.roleUnitAttributes("player:self").intelligence).toBe(10)
  })

  it("ignores same-version duplicate role patches after the first applied packet", () => {
    const store = useRoleStateStore()

    store.patchRoleAttributesPayload({
      roleType: "kitty",
      roleId: 3,
      upsert: {
        intelligence: { value: 10, category: "battle", unit: "number", growthType: "static", order: 1 }
      },
      meta: { version: 4 }
    })
    store.patchRoleAttributesPayload({
      roleType: "kitty",
      roleId: 3,
      upsert: {
        intelligence: { value: 20, category: "battle", unit: "number", growthType: "static", order: 1 }
      },
      meta: { version: 4 }
    })

    expect(store.roleUnitAttributes("kitty:3").intelligence).toBe(10)
  })

  it("initializes kitty static attributes without stale progress entries", () => {
    const store = useRoleStateStore()

    store.initializeKittyFromProfile(9, {
      attributes: [
        {
          attrKey: "intelligence",
          finalValue: 88,
          unit: "number",
          category: "battle",
          growthType: "static",
          order: 1
        },
        {
          attrKey: "battle",
          finalValue: 6,
          unit: "number",
          category: "battle",
          growthType: "levelable",
          progressLevel: 6,
          progressExp: 12,
          progressNextExp: 50,
          order: 2
        }
      ],
      equipment: []
    })

    expect(store.roleUnitAttributes("kitty:9").intelligence).toBe(88)
    expect(store.roles["kitty:9"].attributeProgress.intelligence).toBeUndefined()
    expect(store.roles["kitty:9"].attributeProgress.battle.level).toBe(6)
  })

  it("does not let versionless kitty profile snapshots overwrite newer role patches", () => {
    const store = useRoleStateStore()

    store.patchRoleMutationPayload({
      roleAttributePatch: {
        roleType: "kitty",
        roleId: 5,
        upsert: {
          intelligence: { value: 100, category: "battle", unit: "number", growthType: "static", order: 1 }
        },
        meta: { version: 2 }
      },
      roleEquipmentPatch: {
        roleType: "kitty",
        roleId: 5,
        upsert: [
          {
            instanceId: 55,
            templateItemId: "wood_staff",
            equipType: "battle",
            enhanceLevel: 0,
            durability: 10,
            maxDurability: 10,
            state: "equipped",
            equippedSlot: "right_hand",
            updatedAt: "2026-06-12T00:00:00Z"
          }
        ],
        meta: { version: 2 }
      }
    })
    store.initializeKittyFromProfile(5, {
      attributes: [
        {
          attrKey: "intelligence",
          finalValue: 1,
          unit: "number",
          category: "battle",
          growthType: "static",
          order: 1
        }
      ],
      equipment: []
    })

    expect(store.roleUnitAttributes("kitty:5").intelligence).toBe(100)
    expect(store.roles["kitty:5"].equipment.map((item) => item.instanceId)).toEqual([55])
    expect(store.roles["kitty:5"].initialized.profile).toBe(true)
  })

  it("keeps kitty attribute and equipment patches isolated by role key", () => {
    const store = useRoleStateStore()

    store.patchRoleAttributesPayload({
      roleType: "kitty",
      roleId: 1,
      upsert: {
        intelligence: { value: 100, category: "battle", unit: "number", growthType: "static", order: 1 }
      },
      meta: { version: 1 }
    })
    store.patchRoleAttributesPayload({
      roleType: "kitty",
      roleId: 2,
      upsert: {
        intelligence: { value: 5, category: "battle", unit: "number", growthType: "static", order: 1 }
      },
      meta: { version: 1 }
    })
    store.patchRoleEquipmentPayload({
      roleType: "kitty",
      roleId: 1,
      upsert: [
        {
          instanceId: 11,
          templateItemId: "wood_staff",
          equipType: "battle",
          enhanceLevel: 0,
          durability: 10,
          maxDurability: 10,
          state: "equipped",
          equippedSlot: "right_hand",
          updatedAt: "2026-06-12T00:00:00Z"
        }
      ],
      remove: [],
      meta: { version: 1 }
    })

    expect(store.roleUnitAttributes("kitty:1").intelligence).toBe(100)
    expect(store.roleUnitAttributes("kitty:2").intelligence).toBe(5)
    expect(store.roles["kitty:1"].equipment.map((item) => item.instanceId)).toEqual([11])
    expect(store.roles["kitty:2"].equipment).toEqual([])
  })

  it("applies role mutation payloads through one store entrypoint", () => {
    const store = useRoleStateStore()

    store.patchRoleMutationPayload({
      roleAttributePatch: {
        roleType: "kitty",
        roleId: 4,
        upsert: {
          intelligence: { value: 77, category: "battle", unit: "number", growthType: "static", order: 1 }
        },
        meta: { version: 1 }
      },
      roleEquipmentPatch: {
        roleType: "kitty",
        roleId: 4,
        upsert: [
          {
            instanceId: 44,
            templateItemId: "wood_staff",
            equipType: "battle",
            enhanceLevel: 0,
            durability: 10,
            maxDurability: 10,
            state: "equipped",
            equippedSlot: "right_hand",
            updatedAt: "2026-06-12T00:00:00Z"
          }
        ],
        meta: { version: 1 }
      }
    })

    expect(store.roleUnitAttributes("kitty:4").intelligence).toBe(77)
    expect(store.roles["kitty:4"].equipment.map((item) => item.instanceId)).toEqual([44])
  })

  it("removes kitty equipment and exposes current capability defaults", () => {
    const store = useRoleStateStore()

    store.patchRoleEquipmentPayload({
      roleType: "kitty",
      roleId: 7,
      upsert: [
        {
          instanceId: 21,
          templateItemId: "wood_staff",
          equipType: "battle",
          enhanceLevel: 0,
          durability: 10,
          maxDurability: 10,
          state: "equipped",
          equippedSlot: "right_hand",
          updatedAt: "2026-06-12T00:00:00Z"
        }
      ],
      meta: { version: 1 }
    })
    store.patchRoleEquipmentPayload({
      roleType: "kitty",
      roleId: 7,
      remove: [21],
      meta: { version: 2 }
    })

    const kittyCaps = store.roleCapabilities("kitty:7")
    const playerCaps = store.roleCapabilities("player:self")
    expect(store.roles["kitty:7"].equipment).toEqual([])
    expect(kittyCaps.battleItemBag).toBe(false)
    expect(kittyCaps.loadouts).toBe(false)
    expect(kittyCaps.sets).toBe(false)
    expect(kittyCaps.attributeScenes).toEqual(["battle", "work"])
    expect(kittyCaps.equipmentTypes).toEqual(["battle", "work"])
    expect(kittyCaps.visibleSlots).toContain("fishing_rod")
    expect(kittyCaps.visibleSlots).toContain("fishing_reel")
    expect(playerCaps.battleItemBag).toBe(true)
    expect(playerCaps.loadouts).toBe(true)
    expect(playerCaps.sets).toBe(true)
  })

  it("replaces stale kitty slots from an authoritative mutation snapshot", () => {
    const store = useRoleStateStore()
    const oldItem = {
      instanceId: 71,
      templateItemId: "old_fan",
      equipType: "battle" as const,
      enhanceLevel: 0,
      durability: 10,
      maxDurability: 10,
      state: "equipped",
      equippedSlot: "right_hand",
      updatedAt: "2026-09-03T00:00:00Z"
    }
    const newItem = {
      ...oldItem,
      instanceId: 72,
      templateItemId: "new_fan"
    }

    store.initializeKittyFromProfile(11, { equipment: [oldItem] })
    store.patchRoleMutationPayload({
      roleEquipmentPatch: {
        roleType: "kitty",
        roleId: 11,
        items: [newItem],
        meta: { version: 1 }
      }
    })

    expect(store.roles["kitty:11"].equipment.map((item) => item.instanceId)).toEqual([72])

    store.patchRoleMutationPayload({
      roleEquipmentPatch: {
        roleType: "kitty",
        roleId: 11,
        items: [],
        meta: { version: 2 }
      }
    })

    expect(store.roles["kitty:11"].equipment).toEqual([])
  })

  it("rebases player equipment versions from an authoritative reconnect snapshot", () => {
    const store = useRoleStateStore()
    const item = {
      instanceId: 31,
      templateItemId: "wood_sword",
      equipType: "battle" as const,
      enhanceLevel: 0,
      durability: 10,
      maxDurability: 10,
      state: "equipped",
      equippedSlot: "right_hand",
      updatedAt: "2026-08-30T00:00:00Z"
    }

    store.patchRoleEquipmentPayload({
      roleType: "player",
      roleId: "self",
      upsert: [item],
      meta: { version: 9 }
    })
    store.patchRoleEquipmentPayload({
      roleType: "player",
      roleId: "self",
      items: [{ ...item, equippedSlot: "head" }],
      meta: { version: 1 }
    })
    store.patchRoleEquipmentPayload({
      roleType: "player",
      roleId: "self",
      remove: [31],
      meta: { version: 2 }
    })

    expect(store.roles["player:self"].versions.equipment).toBe(2)
    expect(store.roles["player:self"].equipment).toEqual([])
  })
})
