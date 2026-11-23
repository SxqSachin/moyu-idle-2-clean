export type ActionDef = {
  id: string
  disabled?: boolean
  menu?: string
  mainType?: string
  subType?: string
  nameKey: string
  costs: Record<string, number>
  itemReward: Record<string, number>
}

export type ItemFlowRef = {
  actionId: string
  menu: string
  nameKey: string
}

export type ItemEnemyDropRef = {
  enemyId: string
  min: number
  max: number
  percent: number
}

export type ItemNamedSourceKind = "cat_village_commission" | "cattery_commission" | "kitty_journal"

export type ItemNamedSourceRef = {
  id: string
  kind: ItemNamedSourceKind
  nameKey: string
  detailKey?: string
}

export type ItemInsight = {
  uses: ItemFlowRef[]
  sources: ItemFlowRef[]
  enemySources: ItemEnemyDropRef[]
  catVillageCommissionSources: ItemNamedSourceRef[]
  catteryCommissionSources: ItemNamedSourceRef[]
  kittyJournalSources: ItemNamedSourceRef[]
}

export type InsightMap = Record<string, ItemInsight>
