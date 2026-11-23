import { describe, expect, it } from "vitest"
import { buildInsightMap } from "@/services/item-insights"
import type { ActionDef } from "@/types/item-insights"

function action(def: Partial<ActionDef> & Pick<ActionDef, "id">): ActionDef {
  return {
    id: def.id,
    menu: def.menu || "crafting",
    mainType: def.mainType || "crafting",
    subType: def.subType || "",
    nameKey: def.nameKey || `action.${def.id}.name`,
    costs: def.costs || {},
    itemReward: def.itemReward || {}
  }
}

describe("item-insights", () => {
  it("omits disabled production entries from both sources and material uses", () => {
    const retired = { ...action({ id: "retired_soup", costs: { milk: 3 }, itemReward: { soup: 1 } }), disabled: true }
    const map = buildInsightMap([retired, action({ id: "active_grill", costs: { coal: 2 }, itemReward: { grill: 1 } })], {
      enemyDefinitions: [], catVillageCommissions: [], kittyConfig: {}
    })
    expect(map.soup).toBeUndefined()
    expect(map.milk).toBeUndefined()
    expect(map.grill.sources.map(row => row.actionId)).toEqual(["active_grill"])
  })
  it("keeps action uses, action sources, and enemy drop sources", () => {
    const map = buildInsightMap(
      [
        action({ id: "mine_coal", itemReward: { coal: 1 } }),
        action({ id: "smelt_bar", costs: { coal: 2 }, itemReward: { iron_bar: 1 } })
      ],
      {
        enemyDefinitions: [
          {
            id: "slime",
            drops: [{ itemId: "slime_gel", min: 1, max: 2, percent: 0.25 }]
          }
        ],
        catVillageCommissions: [],
        kittyConfig: {}
      }
    )

    expect(map.coal.sources.map((row) => row.actionId)).toEqual(["mine_coal"])
    expect(map.coal.uses.map((row) => row.actionId)).toEqual(["smelt_bar"])
    expect(map.iron_bar.sources.map((row) => row.actionId)).toEqual(["smelt_bar"])
    expect(map.slime_gel.enemySources).toEqual([{ enemyId: "slime", min: 1, max: 2, percent: 0.25 }])
  })

  it("indexes cat village commission rewardPool without indexing commission costs", () => {
    const map = buildInsightMap([], {
      enemyDefinitions: [],
      catVillageCommissions: [
        {
          id: "cvc_trade_coal",
          nameKey: "catVillage.commission.cvc_trade_coal.name",
          costs: { coal: { min: 1, max: 2 } },
          costOptions: [{ wood: { min: 1, max: 2 } }],
          rewardPool: {
            gold_coin: { min: 10, max: 20 }
          }
        } as never
      ],
      kittyConfig: {}
    })

    expect(map.gold_coin.catVillageCommissionSources).toEqual([
      {
        id: "cvc_trade_coal",
        kind: "cat_village_commission",
        nameKey: "catVillage.commission.cvc_trade_coal.name",
        detailKey: undefined
      }
    ])
    expect(map.coal).toBeUndefined()
    expect(map.wood).toBeUndefined()
  })

  it("indexes cattery commission rewards through reward pools and deduplicates per commission", () => {
    const map = buildInsightMap([], {
      enemyDefinitions: [],
      catVillageCommissions: [],
      kittyConfig: {
        commissions: {
          enabled: true,
          items: [
            {
              id: "bring_back_wood",
              nameKey: "game.catteryCommission.bring_back_wood.name",
              rewardPoolId: "wood_pool"
            }
          ],
          rewardPools: {
            wood_pool: {
              rolls: [
                { chance: 1, mode: "weightedOne", entries: [{ itemId: "wood", weight: 1, min: 1, max: 2 }] },
                { chance: 1, mode: "independent", entries: [{ itemId: "wood", chance: 1, min: 1, max: 1 }] }
              ]
            }
          }
        }
      }
    })

    expect(map.wood.catteryCommissionSources).toEqual([
      {
        id: "bring_back_wood",
        kind: "cattery_commission",
        nameKey: "game.catteryCommission.bring_back_wood.name",
        detailKey: undefined
      }
    ])
  })

  it("indexes kitty journal pool, scheduled, cute equipment, and rare item rewards", () => {
    const map = buildInsightMap([], {
      enemyDefinitions: [],
      catVillageCommissions: [],
      kittyConfig: {
        journalRewards: {
          enabled: true,
          scheduledAffection: {
            enabled: true,
            rewardKey: "scheduled_affection",
            rewardItemId: "kitty_affection",
            contentKey: "game.catteryJournal.diary.affectionGrant"
          },
          cuteEquipmentDrop: {
            enabled: true,
            itemIds: ["cute_hat"]
          },
          rareItemDrops: [
            {
              enabled: true,
              itemId: "harmonic_embryo",
              contentKeys: ["game.catteryJournal.diary.harmonicEmbryoVariants.1"]
            }
          ],
          encounterRareItemDrops: [
            {
              enabled: true,
              itemId: "cat_sword",
              contentKeys: ["game.catteryJournal.encounter.catSwordVariants.1"]
            }
          ],
          pools: {
            idle_default_pool: {
              labelKey: "game.catteryRewardPool.idle_default_pool",
              rolls: [
                { chance: 1, mode: "weightedOne", entries: [{ itemId: "wood", weight: 1, min: 1, max: 2 }] }
              ]
            }
          }
        }
      }
    })

    expect(map.wood.kittyJournalSources.map((source) => source.id)).toEqual(["pool:idle_default_pool"])
    expect(map.kitty_affection.kittyJournalSources.map((source) => source.id)).toEqual(["scheduled:scheduled_affection"])
    expect(map.cute_hat.kittyJournalSources.map((source) => source.id)).toEqual(["cute_equipment_drop"])
    expect(map.harmonic_embryo.kittyJournalSources.map((source) => source.id)).toEqual(["rare:harmonic_embryo"])
    expect(map.harmonic_embryo.kittyJournalSources[0].nameKey).toBe("game.itemTooltip.kittyJournalRareItemDrop")
    expect(map.cat_sword.kittyJournalSources.map((source) => source.id)).toEqual(["encounter-rare:cat_sword"])
    expect(map.cat_sword.kittyJournalSources[0].nameKey).toBe("game.itemTooltip.kittyEncounterRareItemDrop")
  })
})
