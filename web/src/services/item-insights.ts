import rawKittyConfig from "@shared-config/kitty-config.json"
import type { ActionDef, InsightMap, ItemEnemyDropRef, ItemFlowRef, ItemInsight, ItemNamedSourceKind, ItemNamedSourceRef } from "@/types/item-insights"
import { getGameDataChunk } from "@services/game-data-runtime"
import { listEnemyDefinitions } from "@services/enemy-definition"

export type { ItemEnemyDropRef, ItemFlowRef, ItemInsight, ItemNamedSourceRef } from "@/types/item-insights"

let cachedPromise: Promise<InsightMap> | null = null

type EnemyDropDefinition = {
  itemId?: unknown
  min?: unknown
  max?: unknown
  percent?: unknown
}

type EnemySourceDefinition = {
  id: string
  drops?: EnemyDropDefinition[]
}

type CatVillageCommissionDefinition = {
  id?: unknown
  disabled?: unknown
  nameKey?: unknown
  descKey?: unknown
  rewardPool?: unknown
}

type BuildInsightMapOptions = {
  enemyDefinitions?: EnemySourceDefinition[]
  catVillageCommissions?: CatVillageCommissionDefinition[]
  kittyConfig?: unknown
}

function emptyInsight(): ItemInsight {
  return {
    uses: [],
    sources: [],
    enemySources: [],
    catVillageCommissionSources: [],
    catteryCommissionSources: [],
    kittyJournalSources: []
  }
}

function ensureInsight(out: InsightMap, itemId: string): ItemInsight {
  out[itemId] = out[itemId] || emptyInsight()
  return out[itemId]
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return !!value && typeof value === "object" && !Array.isArray(value)
}

function pushUnique(list: ItemFlowRef[], next: ItemFlowRef) {
  if (list.some((item) => item.actionId === next.actionId)) return
  list.push(next)
}

function pushUniqueEnemyDrop(list: ItemEnemyDropRef[], next: ItemEnemyDropRef) {
  if (list.some((item) => item.enemyId === next.enemyId && item.min === next.min && item.max === next.max && item.percent === next.percent)) {
    return
  }
  list.push(next)
}

function pushUniqueNamedSource(list: ItemNamedSourceRef[], next: ItemNamedSourceRef) {
  if (list.some((item) => item.kind === next.kind && item.id === next.id)) return
  list.push(next)
}

function addNamedSource(out: InsightMap, itemId: unknown, source: ItemNamedSourceRef) {
  const normalizedItemId = String(itemId || "").trim()
  if (!normalizedItemId) return
  const insight = ensureInsight(out, normalizedItemId)
  switch (source.kind) {
    case "cat_village_commission":
      pushUniqueNamedSource(insight.catVillageCommissionSources, source)
      break
    case "cattery_commission":
      pushUniqueNamedSource(insight.catteryCommissionSources, source)
      break
    case "kitty_journal":
      pushUniqueNamedSource(insight.kittyJournalSources, source)
      break
  }
}

function itemIdsFromObjectKeys(raw: unknown): string[] {
  if (!isRecord(raw)) return []
  return Object.keys(raw).map((key) => key.trim()).filter(Boolean)
}

function itemIdsFromRewardPool(raw: unknown): string[] {
  if (!isRecord(raw)) return []
  const out: string[] = []
  const seen = new Set<string>()
  const rolls = Array.isArray(raw.rolls) ? raw.rolls : []
  for (const roll of rolls) {
    if (!isRecord(roll) || !Array.isArray(roll.entries)) continue
    for (const entry of roll.entries) {
      if (!isRecord(entry)) continue
      const itemId = String(entry.itemId || "").trim()
      if (!itemId || seen.has(itemId)) continue
      seen.add(itemId)
      out.push(itemId)
    }
  }
  return out
}

function sourceRef(kind: ItemNamedSourceKind, id: string, nameKey: unknown, detailKey?: unknown): ItemNamedSourceRef {
  return {
    kind,
    id,
    nameKey: String(nameKey || "").trim(),
    detailKey: String(detailKey || "").trim() || undefined
  }
}

function addCatVillageCommissionSources(out: InsightMap, commissions: CatVillageCommissionDefinition[]) {
  for (const commission of commissions) {
    if (!isRecord(commission) || commission.disabled === true) continue
    const id = String(commission.id || "").trim()
    if (!id) continue
    const ref = sourceRef("cat_village_commission", id, commission.nameKey || `catVillage.commission.${id}.name`, commission.descKey)
    for (const itemId of itemIdsFromObjectKeys(commission.rewardPool)) {
      addNamedSource(out, itemId, ref)
    }
  }
}

function addCatteryCommissionSources(out: InsightMap, kittyConfig: unknown) {
  const commissions = isRecord(kittyConfig) && isRecord(kittyConfig.commissions) ? kittyConfig.commissions : null
  if (!commissions || commissions.enabled === false || !Array.isArray(commissions.items) || !isRecord(commissions.rewardPools)) return
  for (const item of commissions.items) {
    if (!isRecord(item) || item.disabled === true) continue
    const id = String(item.id || "").trim()
    const rewardPoolId = String(item.rewardPoolId || "").trim()
    if (!id || !rewardPoolId) continue
    const pool = commissions.rewardPools[rewardPoolId]
    const ref = sourceRef("cattery_commission", id, item.nameKey || `game.catteryCommission.${id}.name`, item.descKey)
    for (const itemId of itemIdsFromRewardPool(pool)) {
      addNamedSource(out, itemId, ref)
    }
  }
}

function addKittyJournalSources(out: InsightMap, kittyConfig: unknown) {
  const journalRewards = isRecord(kittyConfig) && isRecord(kittyConfig.journalRewards) ? kittyConfig.journalRewards : null
  if (!journalRewards || journalRewards.enabled === false) return

  if (isRecord(journalRewards.pools)) {
    for (const [poolId, pool] of Object.entries(journalRewards.pools)) {
      const id = String(poolId || "").trim()
      if (!id || !isRecord(pool)) continue
      const ref = sourceRef("kitty_journal", `pool:${id}`, pool.labelKey || `game.catteryRewardPool.${id}`)
      for (const itemId of itemIdsFromRewardPool(pool)) {
        addNamedSource(out, itemId, ref)
      }
    }
  }

  if (isRecord(journalRewards.scheduledAffection) && journalRewards.scheduledAffection.enabled !== false) {
    const rewardItemId = String(journalRewards.scheduledAffection.rewardItemId || "").trim()
    const rewardKey = String(journalRewards.scheduledAffection.rewardKey || "scheduled_affection").trim()
    addNamedSource(out, rewardItemId, sourceRef("kitty_journal", `scheduled:${rewardKey}`, "game.itemTooltip.kittyJournalScheduledAffection"))
  }

  if (isRecord(journalRewards.cuteEquipmentDrop) && journalRewards.cuteEquipmentDrop.enabled !== false && Array.isArray(journalRewards.cuteEquipmentDrop.itemIds)) {
    const ref = sourceRef("kitty_journal", "cute_equipment_drop", "game.itemTooltip.kittyJournalCuteEquipmentDrop")
    for (const itemId of journalRewards.cuteEquipmentDrop.itemIds) {
      addNamedSource(out, itemId, ref)
    }
  }

  if (Array.isArray(journalRewards.rareItemDrops)) {
    for (const [index, drop] of journalRewards.rareItemDrops.entries()) {
      if (!isRecord(drop) || drop.enabled === false) continue
      const itemId = String(drop.itemId || "").trim()
      const id = String(drop.id || drop.itemId || index).trim()
      addNamedSource(out, itemId, sourceRef("kitty_journal", `rare:${id}`, "game.itemTooltip.kittyJournalRareItemDrop"))
    }
  }

  if (Array.isArray(journalRewards.encounterRareItemDrops)) {
    for (const [index, drop] of journalRewards.encounterRareItemDrops.entries()) {
      if (!isRecord(drop) || drop.enabled === false) continue
      const itemId = String(drop.itemId || "").trim()
      const id = String(drop.id || drop.itemId || index).trim()
      addNamedSource(out, itemId, sourceRef("kitty_journal", `encounter-rare:${id}`, "game.itemTooltip.kittyEncounterRareItemDrop"))
    }
  }
}

function readCatVillageCommissions(): CatVillageCommissionDefinition[] {
  const chunk = getGameDataChunk("cat_village_commissions")
  const rows = (chunk as { cat_village_commissions?: unknown }).cat_village_commissions
  return Array.isArray(rows) ? rows as CatVillageCommissionDefinition[] : []
}

export function buildInsightMap(actions: ActionDef[], options: BuildInsightMapOptions = {}): InsightMap {
  const out: InsightMap = {}
  for (const action of actions) {
    if (action.disabled) continue
    const ref: ItemFlowRef = {
      actionId: action.id,
      menu: action.mainType || action.menu || "",
      nameKey: action.nameKey
    }
    for (const itemId of Object.keys(action.costs || {})) {
      pushUnique(ensureInsight(out, itemId).uses, ref)
    }
    for (const itemId of Object.keys(action.itemReward || {})) {
      pushUnique(ensureInsight(out, itemId).sources, ref)
    }
  }
  for (const enemy of options.enemyDefinitions ?? listEnemyDefinitions()) {
    for (const drop of enemy.drops || []) {
      const itemId = String(drop.itemId || "").trim()
      if (!itemId) continue
      pushUniqueEnemyDrop(ensureInsight(out, itemId).enemySources, {
        enemyId: enemy.id,
        min: Math.max(0, Math.trunc(Number(drop.min) || 0)),
        max: Math.max(0, Math.trunc(Number(drop.max) || 0)),
        percent: Math.max(0, Math.min(1, Number(drop.percent) || 0))
      })
    }
  }
  addCatVillageCommissionSources(out, options.catVillageCommissions ?? readCatVillageCommissions())
  addCatteryCommissionSources(out, options.kittyConfig ?? rawKittyConfig)
  addKittyJournalSources(out, options.kittyConfig ?? rawKittyConfig)
  return out
}

export async function getItemInsightMap(_token?: string): Promise<InsightMap> {
  if (!cachedPromise) {
    const actionsChunk = getGameDataChunk("actions")
    const actions = (actionsChunk as { actions?: ActionDef[] }).actions || []
    cachedPromise = Promise.resolve(buildInsightMap(actions))
  }
  return cachedPromise
}
