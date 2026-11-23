import type { ModelState } from '../model/types';
import {
  effectsToJson,
  generatedEnemyStats,
  milestonesToJson,
  scoreAffix,
  scoreBuff,
  scoreEnemy,
  scoreEquipment,
  scoreSkill,
  round
} from '../model/scoring';

export type ExportKind = 'Export_equipment' | 'Export_affix' | 'Export_enemy' | 'Export_combat';

export function toTsv(rows: Array<Record<string, unknown>>): string {
  if (rows.length === 0) return '';
  const headers = Object.keys(rows[0]);
  const escape = (value: unknown) => {
    const text = value == null ? '' : String(value);
    return text.replace(/\t/g, ' ').replace(/\r?\n/g, ' ');
  };
  return [headers.join('\t'), ...rows.map((row) => headers.map((h) => escape(row[h])).join('\t'))].join('\n');
}

export function buildEquipmentRows(state: ModelState): Array<Record<string, unknown>> {
  return state.equipments.map((item) => {
    const score = scoreEquipment(item, state);
    return {
      id: item.id,
      name: item.name,
      level: item.level,
      slot: item.slot,
      rarity: item.rarity,
      archetype: item.archetype,
      score: score.score,
      goldValue: score.goldValue,
      affixSlots: item.affixSlots,
      maxEnhanceLevel: item.maxEnhanceLevel,
      enhanceCostGold: score.detail.enhanceCostGold,
      baseAttributes: effectsToJson(item.baseEffectsText),
      enhancePerLevel: effectsToJson(item.enhancePerLevelText),
      enhanceMilestones: milestonesToJson(item.milestonesText),
      tags: item.tags
    };
  });
}

export function buildAffixRows(state: ModelState): Array<Record<string, unknown>> {
  return state.affixes.map((affix) => {
    const score = scoreAffix(affix, state);
    return {
      id: affix.id,
      name: affix.name,
      rarity: affix.rarity,
      scope: affix.scope,
      score: score.score,
      goldValue: score.goldValue,
      attributes: effectsToJson(affix.effectsText),
      weight: affix.weight,
      conflictGroup: affix.conflictGroup,
      tags: affix.tags
    };
  });
}

export function buildEnemyRows(state: ModelState): Array<Record<string, unknown>> {
  return state.enemies.map((enemy) => {
    const score = scoreEnemy(enemy, state);
    const stats = generatedEnemyStats(enemy, state);
    return {
      id: enemy.id,
      name: enemy.name,
      level: enemy.level,
      tier: enemy.tier,
      role: enemy.role,
      countInWave: enemy.countInWave,
      skillPressure: enemy.skillPressure,
      score: score.score,
      goldValue: score.goldValue,
      rewardGold: score.detail.rewardGold,
      rewardExp: score.detail.rewardExp,
      hp: stats.hp ?? '',
      mp: stats.mp ?? '',
      physics_damage: stats.physics_damage ?? '',
      magical_damage: stats.magical_damage ?? '',
      physics_defence: stats.physics_defence ?? '',
      magical_defence: stats.magical_defence ?? '',
      speed: stats.speed ?? '',
      crit_chance: stats.crit_chance ?? '',
      statsJson: JSON.stringify(stats),
      tags: enemy.tags
    };
  });
}

export function buildCombatRows(state: ModelState): Array<Record<string, unknown>> {
  const skillRows = state.skills.map((skill) => {
    const score = scoreSkill(skill, state);
    return {
      id: skill.id,
      type: 'skill',
      name: skill.name,
      level: skill.level,
      rarity: skill.rarity,
      role: skill.role,
      targetSide: skill.targetSide,
      targetCount: skill.targetCount,
      targetPosition: skill.targetPosition,
      damageType: skill.damageType,
      damageMultiplier: skill.damageMultiplier,
      hitRate: skill.hitRate,
      mpCost: skill.mpCost,
      cooldownTurns: skill.cooldownTurns,
      durationTurns: skill.effectDurationTurns,
      stackLimit: '',
      uptime: '',
      score: score.score,
      goldValue: score.goldValue,
      effects: effectsToJson(skill.effectsText),
      tags: skill.tags
    };
  });

  const buffRows = state.buffs.map((buff) => {
    const score = scoreBuff(buff, state);
    return {
      id: buff.id,
      type: 'buff',
      name: buff.name,
      icon: buff.icon,
      level: '',
      rarity: buff.rarity,
      role: '',
      targetSide: buff.targetSide,
      targetCount: buff.targetCount,
      targetPosition: '',
      damageType: 'none',
      damageMultiplier: '',
      hitRate: '',
      mpCost: '',
      cooldownTurns: '',
      durationTurns: buff.durationTurns,
      stackLimit: buff.stackLimit,
      uptime: round(buff.uptime),
      showInBattleCard: buff.showInBattleCard,
      score: score.score,
      goldValue: score.goldValue,
      effects: effectsToJson(buff.effectsText),
      tags: buff.tags
    };
  });

  return [...skillRows, ...buffRows];
}

export function buildRows(kind: ExportKind, state: ModelState): Array<Record<string, unknown>> {
  switch (kind) {
    case 'Export_equipment':
      return buildEquipmentRows(state);
    case 'Export_affix':
      return buildAffixRows(state);
    case 'Export_enemy':
      return buildEnemyRows(state);
    case 'Export_combat':
      return buildCombatRows(state);
  }
}

export function downloadText(filename: string, text: string, mime = 'text/plain;charset=utf-8') {
  const blob = new Blob([text], { type: mime });
  const url = URL.createObjectURL(blob);
  const a = document.createElement('a');
  a.href = url;
  a.download = filename;
  a.click();
  URL.revokeObjectURL(url);
}
