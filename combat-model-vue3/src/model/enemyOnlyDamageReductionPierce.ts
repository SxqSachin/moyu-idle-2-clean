import { asArray, asString, isRecord, type DataRow } from './catalog.ts';

export interface EnemyOnlyDamageReductionPierceSpec {
  enemyOnlyDamageReductionPierce?: number;
  enemyOnlyDamageReductionPierceTargetStatuses?: string[];
}

export const ENEMY_ONLY_DAMAGE_REDUCTION_PIERCE_FIELDS = [
  'enemyOnlyDamageReductionPierce',
  'enemyOnlyDamageReductionPierceTargetStatuses'
] as const;

export function configuredEnemyOnlyDamageReductionPierceFields(spec: unknown): string[] {
  if (!isRecord(spec)) return [];
  return ENEMY_ONLY_DAMAGE_REDUCTION_PIERCE_FIELDS.filter((field) => Object.prototype.hasOwnProperty.call(spec, field));
}

export function removeEnemyOnlyDamageReductionPierceFields(spec: unknown): DataRow {
  const next = isRecord(spec) ? { ...spec } : {};
  for (const field of ENEMY_ONLY_DAMAGE_REDUCTION_PIERCE_FIELDS) delete next[field];
  return next;
}

export function validateEnemyOnlyDamageReductionPierceSkill(
  skill: DataRow,
  knownBuffIds: Iterable<string>
): string[] {
  const issues: string[] = [];
  const skillId = asString(skill.id).trim() || '(未命名)';
  const knownBuffs = new Set(knownBuffIds);
  for (const [effectIndex, effect] of asArray(skill.effects).filter(isRecord).entries()) {
    if (asString(effect.kind).trim() !== 'damage' || !isRecord(effect.damageSpec)) continue;
    const spec = effect.damageSpec;
    const configured = configuredEnemyOnlyDamageReductionPierceFields(spec);
    if (configured.length === 0) continue;
    const label = `技能 ${skillId}: effects #${effectIndex + 1}.damageSpec`;
    const pierce = Number(spec.enemyOnlyDamageReductionPierce);
    if (!Number.isFinite(pierce) || pierce < 0 || pierce > 1) {
      issues.push(`${label}.enemyOnlyDamageReductionPierce 必须在 0~1 之间`);
    } else if (pierce > 0 && skill.enemyOnly !== true) {
      issues.push(`${label}: 配置减伤穿透时，技能必须勾选“仅供怪物使用”`);
    }
    if (Object.prototype.hasOwnProperty.call(spec, 'enemyOnlyDamageReductionPierceTargetStatuses')) {
      if (!Array.isArray(spec.enemyOnlyDamageReductionPierceTargetStatuses)) {
        issues.push(`${label}.enemyOnlyDamageReductionPierceTargetStatuses 必须是状态列表`);
        continue;
      }
      const seen = new Set<string>();
      for (const rawStatusId of spec.enemyOnlyDamageReductionPierceTargetStatuses) {
        const statusId = typeof rawStatusId === 'string' ? rawStatusId.trim() : '';
        if (!statusId) {
          issues.push(`${label}.enemyOnlyDamageReductionPierceTargetStatuses 含有空状态`);
          continue;
        }
        if (seen.has(statusId)) issues.push(`${label}.enemyOnlyDamageReductionPierceTargetStatuses 重复 ${statusId}`);
        seen.add(statusId);
        if (!knownBuffs.has(statusId)) issues.push(`${label}.enemyOnlyDamageReductionPierceTargetStatuses 未定义 ${statusId}`);
      }
    }
  }
  return issues;
}

export function validatePlayerOwnedDamageSpecs(effects: unknown, label: string): string[] {
  const issues: string[] = [];
  for (const [effectIndex, effect] of asArray(effects).filter(isRecord).entries()) {
    if (asString(effect.kind).trim() !== 'damage' || !isRecord(effect.damageSpec)) continue;
    const configured = configuredEnemyOnlyDamageReductionPierceFields(effect.damageSpec);
    if (configured.length > 0) {
      issues.push(`${label} #${effectIndex + 1}.damageSpec: ${configured.join('、')} 仅允许怪物专属技能使用`);
    }
  }
  return issues;
}

export function validateEnemyOnlyTargetPrioritySkill(skill: DataRow): string[] {
  const issues: string[] = [];
  const skillId = asString(skill.id).trim() || '(未命名)';
  const targetings: Array<{ targeting: unknown; label: string }> = [];
  if (skill.targeting !== undefined) targetings.push({ targeting: skill.targeting, label: 'targeting' });
  for (const [effectIndex, effect] of asArray(skill.effects).filter(isRecord).entries()) {
    targetings.push({ targeting: effect.targeting, label: `effects #${effectIndex + 1}.targeting` });
    if (effect.corpseTargeting !== undefined) targetings.push({ targeting: effect.corpseTargeting, label: `effects #${effectIndex + 1}.corpseTargeting` });
  }
  for (const entry of targetings) {
    if (!isRecord(entry.targeting) || !Object.prototype.hasOwnProperty.call(entry.targeting, 'ignoreTargetPriority')) continue;
    const label = `技能 ${skillId}: ${entry.label}.ignoreTargetPriority`;
    if (typeof entry.targeting.ignoreTargetPriority !== 'boolean') issues.push(`${label} 必须是布尔值`);
    if (skill.enemyOnly !== true) issues.push(`${label} 仅允许勾选“仅供怪物使用”的技能配置`);
  }
  return issues;
}

export function validatePlayerOwnedTargetPriority(effects: unknown, label: string): string[] {
  const issues: string[] = [];
  for (const [effectIndex, effect] of asArray(effects).filter(isRecord).entries()) {
    if (!isRecord(effect.targeting) || !Object.prototype.hasOwnProperty.call(effect.targeting, 'ignoreTargetPriority')) continue;
    issues.push(`${label} #${effectIndex + 1}.targeting.ignoreTargetPriority 仅允许怪物专属技能使用`);
  }
  return issues;
}
