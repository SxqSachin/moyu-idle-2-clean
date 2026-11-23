import assert from "node:assert/strict"
import { readFileSync } from "node:fs"
import { dirname, resolve } from "node:path"
import test from "node:test"
import { fileURLToPath } from "node:url"

const repoRoot = resolve(dirname(fileURLToPath(import.meta.url)), "../../..")
const sourceRoot = resolve(repoRoot, "packages/game-data/source/json")

function readTable(fileName, key) {
  return JSON.parse(readFileSync(resolve(sourceRoot, fileName), "utf8"))[key]
}

const journalRows = [
  ["astral_guard_001", "astral_guard", "meteor_corridor", [50, 250, 1000, 2500, 5000]],
  ["astral_mage_001", "astral_mage", "meteor_corridor", [50, 250, 1000, 2500, 5000]],
  ["starlight_healer_001", "starlight_healer", "meteor_corridor", [50, 250, 1000, 2500, 5000]],
  ["refraction_mirror_doll_001", "refraction_mirror_doll", "meteor_corridor", [50, 250, 1000, 2500, 5000]],
  ["meteor_core_porter_001", "meteor_core_porter", "meteor_corridor", [20, 100, 300, 800, 1500]],
  ["deranged_calibrator_001", "deranged_calibrator", "meteor_corridor", [20, 100, 300, 800, 1500]],
  ["corridor_timekeeper_cat_boss_001", "corridor_timekeeper_cat", "meteor_corridor", [3, 10, 25, 50, 100]],
  ["silence_tome_001", "silence_tome", "forbidden_library", [50, 250, 1000, 2500, 5000]],
  ["brass_book_guard_001", "brass_book_guard", "forbidden_library", [50, 250, 1000, 2500, 5000]],
  ["living_bookmark_001", "living_bookmark", "forbidden_library", [50, 250, 1000, 2500, 5000]],
  ["watcher_owl_001", "watcher_owl", "forbidden_library", [50, 250, 1000, 2500, 5000]],
  ["binding_librarian_001", "binding_librarian", "forbidden_library", [20, 100, 300, 800, 1500]],
  ["banished_spellcat_001", "banished_spellcat", "forbidden_library", [20, 100, 300, 800, 1500]],
  ["forbidden_library_keeper_boss_001", "forbidden_library_keeper", "forbidden_library", [3, 10, 25, 50, 100]],
  ["stardust_mouse_001", "stardust_mouse", "amusement_park", [50, 250, 1000, 2500, 5000]],
  ["carousel_cat_001", "carousel_cat", "amusement_park", [50, 250, 1000, 2500, 5000]],
  ["candy_clown_001", "candy_clown", "amusement_park", [50, 250, 1000, 2500, 5000]],
  ["plush_guard_001", "plush_guard", "amusement_park", [50, 250, 1000, 2500, 5000]],
  ["starlight_healer_002", "backstage_starlight_healer", "amusement_park", [50, 250, 1000, 2500, 5000]],
  ["ghostly_rider_001", "ghostly_rider", "amusement_park", [20, 100, 300, 800, 1500]],
  ["parade_king_boss_001", "parade_king", "amusement_park", [5, 15, 40, 75, 150]],
  ["midnight_ticket_collector_001", "midnight_ticket_collector", "amusement_park", [1, 3, 10, 25, 50]],
  ["faceless_understudy_001", "faceless_understudy", "amusement_park", [1, 3, 10, 25, 50]],
]

const materialWeights = new Map([
  ["star_rail_copper_plate", 29],
  ["refraction_lens_shard", 22],
  ["meteor_core_fragment", 21],
  ["condensed_stardust", 18],
  ["timekeeper_core", 14],
  ["hushed_ink_crystal", 28],
  ["wandering_page_silk", 21],
  ["blue_night_audit_feather", 20],
  ["spell_seal_lock_plate", 17],
  ["final_volume_key_sigil", 13],
  ["wayward_carousel_spring", 17],
  ["candy_bomb", 18],
  ["plush_fur", 19],
  ["bottled_starlight", 16],
  ["ghost_essence", 13],
  ["parade_king_medal", 10],
  ["skillbook_refraction_mark", 17],
  ["skillbook_meteor_core_crush", 17],
  ["skillbook_timekeeper_stasis", 12],
  ["skillbook_hushing_incantation_page", 17],
  ["skillbook_forbidden_trace", 14],
  ["skillbook_cooldown_correction", 10],
])

const specialTraitItemIds = [
  ...Array.from(materialWeights.keys()).filter((id) => !id.startsWith("skillbook_")),
  "faceless_understudy_mask",
]

test("new dungeon journals resolve through enemy, title, trophy, collectible, and icon data", () => {
  const achievements = readTable("achievements.json", "achievements")
  const enemies = new Map(readTable("enemies.json", "enemies").map((row) => [row.id, row]))
  const titles = new Map(readTable("title_suffixes.json", "title_suffixes").map((row) => [row.id, row]))
  const items = new Map(readTable("items.json", "items").map((row) => [row.id, row]))
  const collectibles = new Map(readTable("collectibles.json", "collectibles").map((row) => [row.id, row]))
  const categoryIds = new Set(readTable("collectible_categories.json", "collectible_categories").map((row) => row.id))
  const stageRarities = ["common", "uncommon", "rare", "epic", "legendary"]

  assert.equal(journalRows.length, 23)
  for (const [enemyId, slug, categoryId, counts] of journalRows) {
    assert.ok(enemies.has(enemyId), `enemy missing: ${enemyId}`)
    assert.ok(titles.has(enemyId), `title suffix missing: ${enemyId}`)
    assert.ok(categoryIds.has(categoryId), `collectible category missing: ${categoryId}`)

    const achievementId = `kill_${enemyId}`
    const matches = achievements.filter((row) => row.id === achievementId)
    assert.equal(matches.length, 1, `achievement mismatch: ${achievementId}`)
    const achievement = matches[0]
    assert.equal(achievement.categoryId, "combat")
    assert.equal(achievement.condition.enemyId, enemyId)
    assert.equal(achievement.condition.count, counts[0])
    assert.deepEqual(achievement.stages.map((stage) => stage.condition.count), counts)
    assert.deepEqual(achievement.stages.map((stage) => stage.rarity), stageRarities)
    assert.ok(achievement.stages[2].rewards.some((reward) => reward.kind === "wardrobe" && reward.itemId === enemyId))

    const trophyId = `${slug}_trophy`
    const collectibleId = `collectible_${slug}`
    assert.ok(achievement.stages[4].rewards.some((reward) => reward.kind === "item" && reward.itemId === trophyId && reward.quantity === 1))
    const trophy = items.get(trophyId)
    assert.ok(trophy, `trophy item missing: ${trophyId}`)
    assert.deepEqual(trophy.type, ["collectible"])
    assert.equal(trophy.useGrantCollectibleId, collectibleId)
    assert.equal(trophy.iconId, `item.${trophyId}`)
    const collectible = collectibles.get(collectibleId)
    assert.ok(collectible, `collectible missing: ${collectibleId}`)
    assert.equal(collectible.itemId, trophyId)
    assert.equal(collectible.categoryId, categoryId)

    const png = readFileSync(resolve(repoRoot, `web/public/assets/icons/item/${trophyId}.png`))
    assert.equal(png.subarray(0, 8).toString("hex"), "89504e470d0a1a0a")
    assert.equal(png.readUInt32BE(16), 128, `icon width mismatch: ${trophyId}`)
    assert.equal(png.readUInt32BE(20), 128, `icon height mismatch: ${trophyId}`)
    assert.ok([4, 6].includes(png[25]), `icon lacks alpha channel: ${trophyId}`)
  }
})

test("new dungeon conversion weights stay explicit and mythic outputs stay excluded", () => {
  const items = new Map(readTable("items.json", "items").map((row) => [row.id, row]))
  const groups = readTable("alchemy_groups.json", "alchemy_groups")
  const conversionEntries = groups.flatMap((group) => [...group.entries, ...group.upgradeEntries])

  for (const [itemId, weight] of materialWeights) {
    assert.equal(items.get(itemId)?.alchemy?.convertWeight, weight, `explicit weight mismatch: ${itemId}`)
    assert.ok(conversionEntries.some((entry) => entry.itemId === itemId && entry.weight === weight), `conversion entry missing: ${itemId}`)
  }

  const mask = items.get("faceless_understudy_mask")
  assert.equal(mask?.alchemy?.convertWeight, undefined)
  assert.ok(!conversionEntries.some((entry) => entry.itemId === "faceless_understudy_mask"))
  assert.ok(groups.every((group) => !("ultraRareChanceBase" in group) && !("ultraRareEntries" in group)))
  for (const group of groups) {
    for (const entry of group.upgradeEntries) {
      assert.notEqual(items.get(entry.itemId)?.rarity, "mythic", `mythic conversion upgrade found: ${group.id}.${entry.itemId}`)
    }
  }
  const epicGroup = groups.find((group) => group.id === "group.battle_loot_epic")
  assert.ok(epicGroup)
  assert.equal(epicGroup.upgradeChanceBase, 0.05)
})

test("new dungeon transmute tiers stay lower than the default rule", () => {
  const rules = new Map(readTable("alchemy_transmute_rules.json", "alchemy_transmute_rules").map((row) => [row.id, row]))
  const expected = [
    ["transmute.hidden", 300, 0.30, 0.18, 0.005, ["faceless_understudy_mask"]],
    ["transmute.boss", 200, 0.26, 0.15, 0.015, ["timekeeper_core", "final_volume_key_sigil", "parade_king_medal"]],
    ["transmute.advanced", 100, 0.22, 0.12, 0.025, ["star_rail_copper_plate", "refraction_lens_shard", "meteor_core_fragment", "condensed_stardust", "hushed_ink_crystal", "wandering_page_silk", "blue_night_audit_feather", "spell_seal_lock_plate", "wayward_carousel_spring", "candy_bomb", "plush_fur", "bottled_starlight", "ghost_essence"]],
  ]
  const defaultRule = rules.get("transmute.default")
  assert.ok(defaultRule?.default)
  for (const [id, priority, failChance, minFailChance, luckyChance, itemIds] of expected) {
    const rule = rules.get(id)
    assert.ok(rule, `transmute rule missing: ${id}`)
    assert.equal(rule.priority, priority)
    assert.equal(rule.failChanceBase, failChance)
    assert.equal(rule.minFailChance, minFailChance)
    assert.equal(rule.luckyChanceBase, luckyChance)
    assert.deepEqual(rule.itemIds, itemIds)
    assert.ok(rule.failChanceBase > defaultRule.failChanceBase)
    assert.ok(rule.luckyChanceBase < defaultRule.luckyChanceBase)
  }
})

test("new dungeon infusion traits use distinct tradeoff combinations", () => {
  const traitSets = new Map(readTable("item_traits.json", "item_traits").map((row) => [row.itemId, row]))
  const signatures = new Set()

  assert.equal(specialTraitItemIds.length, 17)
  for (const itemId of specialTraitItemIds) {
    const traitSet = traitSets.get(itemId)
    assert.ok(traitSet, `trait set missing: ${itemId}`)
    assert.equal(traitSet.traits.length, 1)
    const trait = traitSet.traits[0]
    assert.equal(trait.kind, "combat")
    assert.equal(trait.effectType, "add_stat")
    assert.ok(trait.effects.some((effect) => effect.valuePerUnit.min > 0), `positive effect missing: ${itemId}`)
    assert.ok(trait.effects.some((effect) => effect.valuePerUnit.max < 0), `tradeoff missing: ${itemId}`)
    const signature = trait.effects.map((effect) => `${effect.attr}:${Math.sign(effect.valuePerUnit.min)}`).join("|")
    assert.ok(!signatures.has(signature), `duplicate trait combination: ${itemId}`)
    signatures.add(signature)
  }
})
