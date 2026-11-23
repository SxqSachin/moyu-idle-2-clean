import type { EquipmentAffixSlot } from "@/types/equipment"

export type AffixSourceBadgeKind = "craft" | "inherit" | "orphan" | "ordinary"

export type AffixSourceBadgeDefinition = {
  kind: AffixSourceBadgeKind
  icon: string
  titleKey: string
}

export function getAffixSourceBadgeDefinitions(affixes: EquipmentAffixSlot[]): AffixSourceBadgeDefinition[] {
  const badges: AffixSourceBadgeDefinition[] = []
  if (affixes.some(isCraftAffix)) badges.push(badgeByKind.craft)
  if (affixes.some(isInheritAffix)) badges.push(badgeByKind.inherit)
  if (affixes.some(isOrphanInheritAffix)) badges.push(badgeByKind.orphan)
  if (affixes.some(hasOrdinaryAffixSource)) badges.push(badgeByKind.ordinary)
  return badges
}

export function getAffixSlotSourceBadgeDefinitions(affix: EquipmentAffixSlot | null | undefined): AffixSourceBadgeDefinition[] {
  if (!affix) return []
  const badges: AffixSourceBadgeDefinition[] = []
  if (isCraftAffix(affix)) badges.push(badgeByKind.craft)
  if (isInheritAffix(affix)) badges.push(badgeByKind.inherit)
  if (isOrphanInheritAffix(affix)) badges.push(badgeByKind.orphan)
  if (hasOrdinaryAffixSource(affix)) badges.push(badgeByKind.ordinary)
  return badges
}

const badgeByKind: Record<AffixSourceBadgeKind, AffixSourceBadgeDefinition> = {
  craft: {
    kind: "craft",
    icon: "game-icons:round-bottom-flask",
    titleKey: "game.itemCardCraftedBadgeTitle"
  },
  inherit: {
    kind: "inherit",
    icon: "game-icons:family-tree",
    titleKey: "game.itemCardInheritedBadgeTitle"
  },
  orphan: {
    kind: "orphan",
    icon: "game-icons:broken-chain",
    titleKey: "game.itemCardOrphanBadgeTitle"
  },
  ordinary: {
    kind: "ordinary",
    icon: "game-icons:scroll-unfurled",
    titleKey: "game.itemCardOrdinarySourceBadgeTitle"
  }
}

function isCraftAffix(affix: EquipmentAffixSlot): boolean {
  const sourceKind = String(affix.sourceKind || "").trim()
  if (sourceKind === "craft") return true
  const params = affix.effectParams || {}
  return String(params.alchemyKind || "").trim() !== "" && String(params.inheritSourceKind || "").trim() === ""
}

function isInheritAffix(affix: EquipmentAffixSlot): boolean {
  const sourceKind = String(affix.sourceKind || "").trim()
  if (sourceKind === "inherit" || sourceKind === "inherit_orphan") return true
  const params = affix.effectParams || {}
  const inheritSourceKind = String(params.inheritSourceKind || "").trim()
  return inheritSourceKind === "inherit" || inheritSourceKind === "inherit_orphan"
}

function isOrphanInheritAffix(affix: EquipmentAffixSlot): boolean {
  const sourceKind = String(affix.sourceKind || "").trim()
  if (sourceKind === "inherit_orphan") return true
  const params = affix.effectParams || {}
  return Boolean(params.inheritOrphan) || String(params.inheritSourceKind || "").trim() === "inherit_orphan"
}

function hasOrdinaryAffixSource(affix: EquipmentAffixSlot): boolean {
  const params = affix.effectParams || {}
  if (String(params.inheritOrphanReason || "").trim() === "ordinary_affix_source") return true
  const components = Array.isArray(params.alchemyComponents) ? params.alchemyComponents : []
  return components.some((entry) => componentHasOrdinaryAffixSource(entry))
}

function componentHasOrdinaryAffixSource(entry: unknown): boolean {
  if (!entry || typeof entry !== "object" || Array.isArray(entry)) return false
  const row = entry as Record<string, unknown>
  const affixId = String(row.affixId || "").trim()
  if (affixId && !affixId.startsWith("custom:")) return true
  const ancestry = Array.isArray(row.ancestry) ? row.ancestry : []
  return ancestry.some((node) => {
    if (!node || typeof node !== "object" || Array.isArray(node)) return false
    const inheritedAffixId = String((node as Record<string, unknown>).affixId || "").trim()
    return inheritedAffixId !== "" && !inheritedAffixId.startsWith("custom:")
  })
}
