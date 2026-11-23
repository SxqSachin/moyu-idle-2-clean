import { getAttributeDefinition } from "@services/attribute-display"

export const ALCHEMY_FAMILY_ATTRS: Record<string, string[]> = {
  gathering: ["collecting_speed"],
  mining: ["mining_speed", "forging_speed"],
  livestock: ["farming_animal_speed"],
  crafting: ["crafting_speed"],
  sewing: ["sewing_speed"],
  alchemy: ["alchemy_precision", "alchemy_craft_success"],
  physical: ["physics_damage", "strength", "physics_flat_defense_penetration", "physics_percent_defense_penetration", "physics_damage_amplification"],
  crit: ["crit_chance", "crit_damage", "physics_damage", "dexterity"],
  magic_crit: ["magic_crit_chance", "magic_crit_damage", "magicial_percent_defense_penetration", "perception", "battle_exp_bonus"],
  arcane: ["magical_damage", "intelligence", "magicial_flat_defense_penetration", "magicial_percent_defense_penetration", "magical_damage_amplification"],
  shadow: ["speed", "dexterity", "crit_chance", "physics_percent_defense_penetration", "battle_drop_rate_bonus"],
  brute: ["hp", "stamina", "strength", "physics_damage_resist", "battle_exp_bonus"],
  defense: ["physics_defence", "magical_defence", "physics_damage_resist", "magical_damage_resist", "crit_damage_resist"],
  apex: ["luck"],
}

export const ALCHEMY_FAMILY_MATERIAL_HINTS: Record<string, string> = {
  gathering: "alchemy.familyMaterialHint.gathering",
  mining: "alchemy.familyMaterialHint.mining",
  livestock: "alchemy.familyMaterialHint.livestock",
  crafting: "alchemy.familyMaterialHint.crafting",
  sewing: "alchemy.familyMaterialHint.sewing",
  alchemy: "alchemy.familyMaterialHint.alchemy",
  physical: "alchemy.familyMaterialHint.physical",
  crit: "alchemy.familyMaterialHint.crit",
  magic_crit: "alchemy.familyMaterialHint.magic_crit",
  arcane: "alchemy.familyMaterialHint.arcane",
  shadow: "alchemy.familyMaterialHint.shadow",
  brute: "alchemy.familyMaterialHint.brute",
  defense: "alchemy.familyMaterialHint.defense",
  apex: "alchemy.familyMaterialHint.apex",
}

export function listAlchemyFamilies(): string[] {
  return Object.keys(ALCHEMY_FAMILY_ATTRS)
}

export function getAlchemyFamilyAttrs(family: string): string[] {
  return ALCHEMY_FAMILY_ATTRS[String(family || "").trim()] || []
}

export function attrNameKey(attr: string): string {
  return getAttributeDefinition(attr)?.nameKey || `attr.${attr}.name`
}
