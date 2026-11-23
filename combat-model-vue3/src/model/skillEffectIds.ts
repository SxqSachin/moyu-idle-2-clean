type EffectRow = Record<string, unknown>;

function isRecord(value: unknown): value is EffectRow {
  return Boolean(value) && typeof value === 'object' && !Array.isArray(value);
}

function normalizedIdPart(value: unknown, fallback: string): string {
  const normalized = String(value ?? '')
    .trim()
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, '_')
    .replace(/^_+|_+$/g, '');
  return normalized || fallback;
}

export function stableSkillEffectId(effect: unknown): string {
  if (!isRecord(effect)) return '';
  return String(effect.id ?? effect.effectId ?? '').trim();
}

export function createStableSkillEffectId(ownerId: unknown, kind: unknown, effects: unknown[]): string {
  const base = `${normalizedIdPart(ownerId, 'skill')}_${normalizedIdPart(kind, 'effect')}`;
  const existing = new Set(effects.map(stableSkillEffectId).filter(Boolean).map((id) => id.toLowerCase()));
  if (!existing.has(base)) return base;
  let suffix = 2;
  while (existing.has(`${base}_${suffix}`)) suffix += 1;
  return `${base}_${suffix}`;
}

export function validateSkillEffectStableIds(skillId: unknown, effects: unknown[]): string[] {
  const owner = String(skillId ?? '').trim() || '(未命名技能)';
  const issues: string[] = [];
  const seen = new Set<string>();
  for (const [index, raw] of effects.entries()) {
    if (!isRecord(raw)) continue;
    const label = `技能 ${owner}: effects #${index + 1}`;
    const hasId = Object.prototype.hasOwnProperty.call(raw, 'id');
    const hasLegacyId = Object.prototype.hasOwnProperty.call(raw, 'effectId');
    const canonical = hasId ? String(raw.id ?? '').trim() : '';
    const legacy = hasLegacyId ? String(raw.effectId ?? '').trim() : '';
    if (!hasId && !hasLegacyId) issues.push(`${label}: id 必须是非空字符串`);
    if (hasId && (typeof raw.id !== 'string' || !canonical)) issues.push(`${label}: id 必须是非空字符串`);
    if (hasLegacyId && (typeof raw.effectId !== 'string' || !legacy)) issues.push(`${label}: effectId 必须是非空字符串`);
    if (canonical && legacy && canonical !== legacy) issues.push(`${label}: id 与旧 effectId 不一致`);
    const id = canonical || legacy;
    if (!id) continue;
    if (!/^[a-z0-9][a-z0-9._:-]*$/.test(id)) issues.push(`${label}: 稳定效果 ID 只能包含小写字母、数字、点、下划线、冒号或连字符`);
    const identity = id.toLowerCase();
    if (seen.has(identity)) issues.push(`技能 ${owner}: effects 稳定 ID 重复 ${id}`);
    seen.add(identity);
  }
  return issues;
}

export function validateGlobalSkillEffectStableIds(skills: unknown[]): string[] {
  const issues: string[] = [];
  const ownerById = new Map<string, string>();
  for (const raw of skills) {
    if (!isRecord(raw)) continue;
    const owner = String(raw.id ?? '').trim() || '(未命名技能)';
    const effects = Array.isArray(raw.effects) ? raw.effects : [];
    for (const effect of effects) {
      const id = stableSkillEffectId(effect);
      if (!id) continue;
      const identity = id.toLowerCase();
      const previousOwner = ownerById.get(identity);
      if (previousOwner && previousOwner !== owner) {
        issues.push(`技能效果稳定 ID 全局重复 ${id}: ${previousOwner} / ${owner}`);
        continue;
      }
      ownerById.set(identity, owner);
    }
  }
  return issues;
}
