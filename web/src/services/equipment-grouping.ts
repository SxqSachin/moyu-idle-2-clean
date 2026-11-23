import { getEquipmentTemplateDefinition } from "@/services/item-definition";
import { getLocalizedWeaponClassLabel, listWeaponClassDefinitions } from "@/services/weapon-class";

export type GroupSection<T> = {
  id: string;
  label: string;
  items: T[];
  subSections?: GroupSection<T>[];
};

const EQUIPMENT_SLOT_ORDER = ["head", "body", "cloak", "arm", "legs", "feet", "right_hand", "left_hand", "accessory"] as const;

type Translate = (key: string) => string;

function localizeFallback(t: Translate, key: string, fallback: string): string {
  const localized = t(key);
  return localized === key ? fallback : localized;
}

function localizeEquipSlot(t: Translate, slotId: string): string {
  return localizeFallback(t, `game.equipSlot.${slotId}`, slotId);
}

function localizeEquipmentSecondaryClass(t: Translate, classId: string): string {
  return localizeFallback(t, `equipment_secondary_class.${classId}.name`, classId);
}

function resolveSecondarySection<T>(item: T, getItemId: (item: T) => string, t: Translate): { id: string; label: string; order: number } {
  const template = getEquipmentTemplateDefinition(getItemId(item));
  if (!template) {
    return {
      id: "other",
      label: t("game.itemSectionOther"),
      order: 9999
    };
  }
  const weaponClassId = Array.isArray(template.weaponClass) ? String(template.weaponClass[0] || "").trim() : "";
  if (weaponClassId) {
    const weaponOrder = listWeaponClassDefinitions()
      .map((row) => String(row.id || "").trim())
      .filter(Boolean)
      .indexOf(weaponClassId);
    return {
      id: `weapon:${weaponClassId}`,
      label: getLocalizedWeaponClassLabel(t, weaponClassId, weaponClassId),
      order: weaponOrder >= 0 ? weaponOrder : 500
    };
  }
  const equipmentSecondaryClassId = String(template.equipmentSecondaryClass || "").trim();
  if (equipmentSecondaryClassId) {
    return {
      id: `secondary:${equipmentSecondaryClassId}`,
      label: localizeEquipmentSecondaryClass(t, equipmentSecondaryClassId),
      order: 1000
    };
  }
  return {
    id: "other",
    label: t("game.itemSectionOther"),
    order: 9999
  };
}

function compareSectionByOrder<T>(a: GroupSection<T>, b: GroupSection<T>, orderMap: Map<string, number>) {
  const orderA = orderMap.get(a.id) ?? 9999;
  const orderB = orderMap.get(b.id) ?? 9999;
  if (orderA !== orderB) return orderA - orderB;
  return a.label.localeCompare(b.label);
}

export function buildEquipmentSlotSecondarySections<T>(params: {
  items: T[];
  getItemId: (item: T) => string;
  t: Translate;
}): GroupSection<T>[] {
  const { items, getItemId, t } = params;
  if (!items.length) return [];
  const slotMap = new Map<string, T[]>();
  const otherSlotItems: T[] = [];
  for (const item of items) {
    const template = getEquipmentTemplateDefinition(getItemId(item));
    const slotId = String(template?.slot || "").trim();
    if (!slotId) {
      otherSlotItems.push(item);
      continue;
    }
    if (!slotMap.has(slotId)) slotMap.set(slotId, []);
    slotMap.get(slotId)?.push(item);
  }

  const orderedSlots = [...EQUIPMENT_SLOT_ORDER.filter((slot) => slotMap.has(slot)), ...[...slotMap.keys()].filter((slot) => !EQUIPMENT_SLOT_ORDER.includes(slot as (typeof EQUIPMENT_SLOT_ORDER)[number]))];
  const slotSections: GroupSection<T>[] = [];
  for (const slotId of orderedSlots) {
    const slotItems = slotMap.get(slotId) || [];
    const grouped = new Map<string, T[]>();
    const sectionOrder = new Map<string, number>();
    const sectionLabel = new Map<string, string>();
    for (const item of slotItems) {
      const secondary = resolveSecondarySection(item, getItemId, t);
      if (!grouped.has(secondary.id)) grouped.set(secondary.id, []);
      grouped.get(secondary.id)?.push(item);
      sectionOrder.set(secondary.id, secondary.order);
      sectionLabel.set(secondary.id, secondary.label);
    }
    const subSections = [...grouped.entries()].map(([id, sectionItems]) => ({
      id,
      label: sectionLabel.get(id) || id,
      items: sectionItems
    }));
    subSections.sort((a, b) => compareSectionByOrder(a, b, sectionOrder));
    slotSections.push({
      id: slotId,
      label: localizeEquipSlot(t, slotId),
      items: [],
      subSections
    });
  }

  if (otherSlotItems.length > 0) {
    slotSections.push({
      id: "other",
      label: t("game.itemSectionOther"),
      items: otherSlotItems
    });
  }
  return slotSections;
}
