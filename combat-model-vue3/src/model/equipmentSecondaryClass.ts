import definitions from './equipment-secondary-classes.json';

export type EquipmentSecondaryClassDefinition = {
  id: string;
  module: 'work' | 'battle' | 'all';
  order: number;
  nameKey: string;
  descKey: string;
  zhCN?: { name?: string; desc?: string };
  enUS?: { name?: string; desc?: string };
};

function normalizeDefinition(raw: unknown): EquipmentSecondaryClassDefinition | null {
  if (!raw || typeof raw !== 'object') return null;
  const row = raw as Record<string, unknown>;
  const id = typeof row.id === 'string' ? row.id.trim() : '';
  if (!id) return null;
  const module = row.module === 'work' || row.module === 'battle' || row.module === 'all' ? row.module : 'all';
  const order = Number.isFinite(row.order) ? Number(row.order) : 9999;
  const nameKey = typeof row.nameKey === 'string' && row.nameKey.trim()
    ? row.nameKey.trim()
    : `equipment_secondary_class.${id}.name`;
  const descKey = typeof row.descKey === 'string' && row.descKey.trim()
    ? row.descKey.trim()
    : `equipment_secondary_class.${id}.desc`;
  const zhCN = row.zhCN && typeof row.zhCN === 'object' ? (row.zhCN as { name?: string; desc?: string }) : undefined;
  const enUS = row.enUS && typeof row.enUS === 'object' ? (row.enUS as { name?: string; desc?: string }) : undefined;
  return { id, module, order, nameKey, descKey, zhCN, enUS };
}

export const equipmentSecondaryClassDefinitions: EquipmentSecondaryClassDefinition[] = (Array.isArray(definitions) ? definitions : [])
  .map(normalizeDefinition)
  .filter((row): row is EquipmentSecondaryClassDefinition => Boolean(row))
  .sort((a, b) => a.order - b.order || a.id.localeCompare(b.id));

export const equipmentSecondaryClassIds = equipmentSecondaryClassDefinitions.map((row) => row.id);
