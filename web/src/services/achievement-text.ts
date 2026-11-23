import { getGameDataChunk } from "@services/game-data-runtime"

type NamedEntry = { id: string; nameKey?: string; name?: string }
type LocalizedNamedEntry = NamedEntry & { names?: Record<string, string> }
type AchievementCondition = Record<string, unknown>
type TranslateFn = (key: string, ...args: unknown[]) => string

function mapNames(rows: NamedEntry[]): Record<string, string> {
  const out: Record<string, string> = {}
  for (const row of rows) {
    if (!row.id) continue
    out[row.id] = row.nameKey || row.name || row.id
  }
  return out
}

let itemNames: Record<string, string> | null = null
let enemyNames: Record<string, string> | null = null
let actionNames: Record<string, string> | null = null
let commissionNames: Record<string, string> | null = null
let attributeNames: Record<string, string> | null = null
let fishingSpeciesByID: Record<string, LocalizedNamedEntry> | null = null

function ensureNameMaps() {
  if (itemNames && enemyNames && actionNames && commissionNames && attributeNames && fishingSpeciesByID) return
  itemNames = mapNames((getGameDataChunk("core") as unknown as { items?: NamedEntry[] }).items ?? [])
  enemyNames = mapNames((getGameDataChunk("enemies") as unknown as { enemies?: NamedEntry[] }).enemies ?? [])
  actionNames = mapNames((getGameDataChunk("actions") as unknown as { actions?: NamedEntry[] }).actions ?? [])
  commissionNames = mapNames((getGameDataChunk("cat_village_commissions") as unknown as { cat_village_commissions?: NamedEntry[] }).cat_village_commissions ?? [])
  attributeNames = mapNames((getGameDataChunk("attributes") as unknown as { attributes?: NamedEntry[] }).attributes ?? [])
  fishingSpeciesByID = Object.fromEntries(
    ((getGameDataChunk("fishing_species") as unknown as { fishing_species?: LocalizedNamedEntry[] }).fishing_species ?? [])
      .filter((row) => row.id)
      .map((row) => [row.id, row])
  )
}

function buildDisplayName(t: TranslateFn, map: Record<string, string>, id: unknown): string {
  const key = String(id || "").trim()
  if (!key) return ""
  const nameKey = map[key]
  return nameKey ? t(nameKey) : key
}

function localizedOptional(t: TranslateFn, key: unknown): string {
  const normalized = String(key || "").trim()
  if (!normalized) return ""
  const localized = t(normalized)
  return localized === normalized ? "" : localized
}

function buildFishingSpeciesName(id: unknown, localeTag: string): string {
  const key = String(id || "").trim()
  if (!key) return ""
  const row = fishingSpeciesByID?.[key]
  return row?.names?.[localeTag]
    || row?.names?.[localeTag.split("-")[0]]
    || row?.names?.["en-US"]
    || row?.name
    || key
}

export function resolveAchievementTitle(t: TranslateFn, nameKey: string | undefined, condition: AchievementCondition, fallbackId: string, localeTag = "en-US"): string {
  ensureNameMaps()
  const custom = localizedOptional(t, nameKey)
  if (custom) return custom
  const kind = String(condition.kind || "").trim()
  switch (kind) {
    case "kill_enemy":
      return t("achievements.generatedTitle.kill_enemy", { target: buildDisplayName(t, enemyNames!, condition.enemyId) })
    case "complete_action":
      return t("achievements.generatedTitle.complete_action", { target: buildDisplayName(t, actionNames!, condition.actionId) })
    case "own_kitty":
      return t("achievements.generatedTitle.own_kitty")
    case "own_item":
      return t("achievements.generatedTitle.own_item", { target: buildDisplayName(t, itemNames!, condition.itemId) })
    case "obtain_item":
      return t("achievements.generatedTitle.obtain_item", { target: buildDisplayName(t, itemNames!, condition.itemId) })
    case "enchant_count":
      return t("achievements.generatedTitle.enchant_count")
    case "enhance_count":
      return t("achievements.generatedTitle.enhance_count")
    case "complete_commission":
      return t("achievements.generatedTitle.complete_commission")
    case "complete_rumor_commission":
      return t("achievements.generatedTitle.complete_rumor_commission")
    case "complete_specific_commission":
      return t("achievements.generatedTitle.complete_specific_commission", { target: buildDisplayName(t, commissionNames!, condition.commissionId) })
    case "player_attr_reached":
      return t("achievements.generatedTitle.player_attr_reached", { target: buildDisplayName(t, attributeNames!, condition.attrKey) })
    case "catch_fish":
      return t("achievements.generatedTitle.catch_fish", { target: buildFishingSpeciesName(condition.speciesId, localeTag) })
    default:
      return fallbackId
  }
}

export function resolveAchievementDescription(t: TranslateFn, descKey: string | undefined, condition: AchievementCondition, localeTag = "en-US"): string {
  ensureNameMaps()
  const custom = localizedOptional(t, descKey)
  if (custom) return custom
  const kind = String(condition.kind || "").trim()
  if (!kind) return ""
  switch (kind) {
    case "kill_enemy":
      return t("achievements.condition.kill_enemy", { target: buildDisplayName(t, enemyNames!, condition.enemyId) })
    case "complete_action":
      return t("achievements.condition.complete_action", {
        actor: t(`achievements.actor.${String(condition.actorType || "any").trim() || "any"}`),
        target: buildDisplayName(t, actionNames!, condition.actionId)
      })
    case "own_kitty":
      return t("achievements.condition.own_kitty")
    case "own_item":
      return t("achievements.condition.own_item", { target: buildDisplayName(t, itemNames!, condition.itemId) })
    case "obtain_item":
      return t("achievements.condition.obtain_item", { target: buildDisplayName(t, itemNames!, condition.itemId) })
    case "enchant_count":
      return t("achievements.condition.enchant_count")
    case "enhance_count":
      return t("achievements.condition.enhance_count")
    case "complete_commission":
      return t("achievements.condition.complete_commission")
    case "complete_rumor_commission":
      return t("achievements.condition.complete_rumor_commission")
    case "complete_specific_commission":
      return t("achievements.condition.complete_specific_commission", { target: buildDisplayName(t, commissionNames!, condition.commissionId) })
    case "player_attr_reached":
      return t("achievements.condition.player_attr_reached", { target: buildDisplayName(t, attributeNames!, condition.attrKey) })
    case "catch_fish":
      return t("achievements.condition.catch_fish", { target: buildFishingSpeciesName(condition.speciesId, localeTag) })
    default:
      return ""
  }
}
