import type { ActionMaterialInstanceRequirement } from "@/types/generated/game-data"
import type { InventoryInstance, ItemCustomization } from "@/types/equipment"

type AffixIdentity = {
  slotIndex?: number
  affixId?: string | null
}

type RequirementCandidate = Pick<
  InventoryInstance,
  "enhanceLevel" | "durability" | "maxDurability" | "quantity" | "state"
> & {
  affixes?: AffixIdentity[]
  customData?: ItemCustomization
}

export type ActionMaterialBindingArgs = {
  materialInstanceBindings: Record<string, number[]>
  inheritFromInstanceId?: number
}

function normalizeAffixIdentities(rows: AffixIdentity[] | null | undefined) {
  return (rows || [])
    .map((row) => ({
      slotIndex: Math.floor(Number(row?.slotIndex)),
      affixId: String(row?.affixId || "").trim()
    }))
    .filter((row) => Number.isFinite(row.slotIndex) && row.slotIndex >= 0 && row.affixId !== "")
    .sort((a, b) => (a.slotIndex - b.slotIndex) || a.affixId.localeCompare(b.affixId))
}

export function actionMaterialAffixesMatchTemplate(
  affixes: AffixIdentity[] | null | undefined,
  templateAffixes: AffixIdentity[] | null | undefined
): boolean {
  const actual = normalizeAffixIdentities(affixes)
  const expected = normalizeAffixIdentities(templateAffixes)
  if (actual.length !== expected.length) return false
  return actual.every((row, index) => (
    row.slotIndex === expected[index]?.slotIndex && row.affixId === expected[index]?.affixId
  ))
}

export function actionMaterialInstanceSatisfiesRequirement(
  instance: RequirementCandidate,
  requirement: ActionMaterialInstanceRequirement,
  templateAffixes: AffixIdentity[] | null | undefined
): boolean {
  if (instance.state !== "inventory" || Number(instance.quantity || 0) <= 0) return false
  if (Number(instance.enhanceLevel || 0) !== Number(requirement.enhanceLevel)) return false
  if (requirement.requireTemplateAffixes && !actionMaterialAffixesMatchTemplate(instance.affixes, templateAffixes)) {
    return false
  }
  if (requirement.durability === "full") {
    const durability = Number(instance.durability || 0)
    const maxDurability = Number(instance.maxDurability || 0)
    if (durability <= 0 || maxDurability <= 0 || durability !== maxDurability) return false
  }
  const customName = String(instance.customData?.name || "").trim()
  const customLabelColor = String(instance.customData?.labelColor || "").trim()
  return customName === "" && customLabelColor === ""
}

export function actionMaterialCanProvideEnhancementInheritance(
  requirements: Record<string, ActionMaterialInstanceRequirement> | null | undefined,
  itemId: string
): boolean {
  return !requirements || !Object.prototype.hasOwnProperty.call(requirements, itemId)
}

export function buildActionMaterialBindingArgs(
  rawBindings: Record<string, number[]>,
  inheritanceSourceInstanceId = 0
): ActionMaterialBindingArgs | undefined {
  const bindings: Record<string, number[]> = {}
  const boundIDs = new Set<number>()
  for (const [itemId, rawIDs] of Object.entries(rawBindings)) {
    const normalizedItemId = String(itemId || "").trim()
    if (!normalizedItemId) continue
    const ids = Array.from(new Set(
      (rawIDs || [])
        .map((instanceId) => Math.floor(Number(instanceId)))
        .filter((instanceId) => Number.isFinite(instanceId) && instanceId > 0)
    ))
    if (!ids.length) continue
    bindings[normalizedItemId] = ids
    ids.forEach((instanceId) => boundIDs.add(instanceId))
  }
  if (!Object.keys(bindings).length) return undefined
  const sourceID = Math.floor(Number(inheritanceSourceInstanceId || 0))
  if (sourceID > 0 && boundIDs.has(sourceID)) {
    return { materialInstanceBindings: bindings, inheritFromInstanceId: sourceID }
  }
  return { materialInstanceBindings: bindings }
}
