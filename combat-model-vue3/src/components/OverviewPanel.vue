<script setup lang="ts">
import type { SkillOverview, EquipmentOverview, EffectDisplay } from '../model/overview';

const props = defineProps<{
  table: string;
  overview: SkillOverview | EquipmentOverview | null;
  multiTargetFactor?: number;
}>();

const POSITION_LABELS: Record<string, string> = {
  vanguard: '前排', midguard: '中排', rearguard: '后排'
};
const SEEK_LABELS: Record<string, string> = {
  front_to_rear: '从前往后', rear_to_front: '从后往前', random_alive: '随机存活', none: '无'
};
const SCOPE_LABELS: Record<string, string> = {
  enemy: '敌方', ally: '友方', self: '自身', all: '全体'
};
const RULE_LABELS: Record<string, string> = {
  random: '随机', boss_first: '优先Boss', hp_min: '最低血量', hp_max: '最高血量',
  hp_percent_min: '最低血比', hp_percent_max: '最高血比',
  first: '第一个', attr_max: '最高属性', attr_min: '最低属性', self: '自身'
};
const KIND_LABELS: Record<string, string> = {
  damage: '伤害', heal: '治疗', status: '状态', resource: '资源',
  add_buff: '施加状态', remove_buff: '移除状态', consume_mp: '消耗MP',
  execute: '斩杀', runtime_damage_amp: '运行时增伤',
  add_runtime_damage_amp: '叠加运行时增伤', add_damage_amp: '叠加增伤',
  summon: '召唤', player_clone: '玩家影分身', mirror_cast_skill: '镜像施法'
};
const DAMAGE_TYPE_LABELS: Record<string, string> = {
  physical: '物理', magical: '魔法', true: '真实', mixed: '混合'
};
const TRIGGER_LABELS: Record<string, string> = {
  on_battle_start: '战斗开始', on_wave_start: '波次开始',
  on_turn_start: '回合开始', on_turn_end: '回合结束',
  on_before_deal_damage: '造成伤害前', on_after_deal_damage: '造成伤害后',
  on_after_deal_crit: '暴击后', on_after_deal_no_crit: '未暴击后',
  on_after_basic_attack_deal_damage: '普攻伤害后',
  on_before_take_damage: '受击前', on_after_take_damage: '受击后',
  on_after_deal_heal: '治疗后', on_kill: '击杀后', on_unit_death: '任意单位阵亡后',
  on_after_summon: '召唤后', on_other_unit_cast_skill: '其他单位施法',
  on_battle_end: '战斗结束'
};

function posLabel(p: string): string { return POSITION_LABELS[p] ?? p; }
function seekLabel(s: string): string { return SEEK_LABELS[s] ?? s; }
function scopeLabel(s: string): string { return SCOPE_LABELS[s] ?? s; }
function ruleLabel(r: string): string { return RULE_LABELS[r] ?? r; }
function kindLabel(k: string): string { return KIND_LABELS[k] ?? k; }
function dtLabel(dt: string): string { return DAMAGE_TYPE_LABELS[dt] ?? dt; }
function triggerLabel(t: string): string { return TRIGGER_LABELS[t] ?? t; }

function targetCountText(effect: EffectDisplay): string {
  const max = effect.targetCountMax;
  if (max !== null && max !== effect.targetCountBase && max > 0) {
    return `${effect.targetCountBase}~${max}`;
  }
  return `${effect.targetCountBase}`;
}

function multiTargetFactorDisplay(effect: EffectDisplay, discount: number): string {
  const base = effect.targetCountBase;
  const max = effect.targetCountMax ?? base;
  const count = max > 0 ? max : base;
  if (count <= 1) return '×1';
  const effective = 1 + (count - 1) * discount;
  return `${count}目标 × ${(discount * 100).toFixed(0)}%折 = ×${effective.toFixed(2)}`;
}

function perTargetWeight(effect: EffectDisplay): number {
  return Math.round(effect.rawFormulaWeight * effect.hitDamageEquivalent * 1000) / 1000;
}

function hitDamageScalingText(effect: EffectDisplay): string {
  const scaling = effect.hitDamageScaling;
  if (!scaling) return '';
  return `${scaling.secondHitMultiplier} ^ ((n-1) ^ ${scaling.curvePower}) · 等效 ${effect.hitDamageEquivalent.toFixed(4)} 次`;
}

function effectiveTotalWeight(effect: EffectDisplay, discount: number): number {
  const count = (effect.targetCountMax ?? effect.targetCountBase) || effect.targetCountBase;
  const effective = 1 + (Math.max(1, count) - 1) * discount;
  return Math.round(perTargetWeight(effect) * effective * 1000) / 1000;
}

function hasFormula(effect: EffectDisplay): boolean {
  return ['damage', 'heal', 'resource'].includes(effect.kind);
}

function isSkillOverview(ov: SkillOverview | EquipmentOverview): ov is SkillOverview {
  return 'effects' in ov && 'cooldown' in ov;
}

function verdictClass(status: string): string {
  if (status === 'ok') return 'verdict-ok';
  if (status === 'caution') return 'verdict-caution';
  return 'verdict-overtuned';
}
function verdictIcon(status: string): string {
  if (status === 'ok') return '✓';
  if (status === 'caution') return '⚠';
  return '✗';
}
function verdictText(status: string): string {
  if (status === 'ok') return '合理';
  if (status === 'caution') return '偏强';
  return '超模';
}

const MULTI_DISCOUNT = 0.35;
</script>

<template>
  <section v-if="overview" class="overview-panel">

    <!-- SKILL OVERVIEW -->
    <template v-if="table === 'skills' && isSkillOverview(overview)">
      <div class="overview-header">
        <strong>技能概览</strong>
        <span class="meta-pill">等级 {{ (overview as SkillOverview).level }}</span>
        <span v-if="(overview as SkillOverview).cooldown > 0" class="meta-pill">冷却 {{ (overview as SkillOverview).cooldown }} 回</span>
        <span v-if="(overview as SkillOverview).mpCost > 0" class="meta-pill">MP {{ (overview as SkillOverview).mpCost }}</span>
        <span v-if="(overview as SkillOverview).effects.length === 0" class="dim">无效果</span>
      </div>

      <div v-for="(effect, i) in (overview as SkillOverview).effects" :key="i" class="effect-block">
        <!-- Effect header -->
        <div class="effect-title">
          <span class="eff-num">效果 #{{ i + 1 }}</span>
          <span class="kind-tag">{{ kindLabel(effect.kind) }}</span>
          <span v-if="effect.damageType" class="dtype-tag">{{ dtLabel(effect.damageType) }}</span>
        </div>

        <!-- Targeting row -->
        <div class="info-row">
          <span class="row-label">目标</span>
          <span class="row-value">
            <span v-for="pos in effect.targetPositions" :key="pos" class="pos-tag">{{ posLabel(pos) }}</span>
            <span v-if="!effect.targetPositions.length" class="dim">-</span>
            <span>{{ scopeLabel(effect.targetScope) }}</span>
            <span class="sep">·</span>
            <span>{{ seekLabel(effect.secondarySeek) }}</span>
            <span class="sep">·</span>
            <span>{{ ruleLabel(effect.selectRule) }}</span>
            <span v-if="effect.selectAttrKey" class="dim">({{ effect.selectAttrKey }})</span>
            <span class="sep">·</span>
            <strong class="hi">{{ targetCountText(effect) }} 目标</strong>
            <span v-if="effect.targetCountMax !== null && effect.targetCountMax !== effect.targetCountBase" class="dim">(上限 {{ effect.targetCountMax }})</span>
            <span v-if="effect.fallbackSelf" class="dim">· 无目标时自身</span>
          </span>
        </div>

        <!-- Formula + weight breakdown (damage/heal/resource) -->
        <template v-if="hasFormula(effect)">
          <div class="info-row">
            <span class="row-label">伤害次数</span>
            <span class="row-value"><strong class="hi">{{ effect.hitCountBase }} 次</strong><span v-if="effect.hitDamageScaling" class="dim"> · {{ hitDamageScalingText(effect) }}</span></span>
          </div>

          <div v-if="effect.formulaTerms.length > 0 || effect.baseAdd !== 0" class="formula-block">
            <div class="formula-title">公式权重拆解 <span class="formula-hint">（系数 ÷ 每主属性点换算率 = 主属性等价权重）</span></div>

            <div v-if="effect.baseAdd !== 0" class="formula-row">
              <span class="term-attr">基础值</span>
              <span class="term-coeff">{{ effect.baseAdd }}</span>
              <span class="term-op">÷</span>
              <span class="term-src dim">—</span>
              <span class="term-eq">=</span>
              <span class="term-w">{{ Math.abs(effect.baseAdd).toFixed(3) }}</span>
            </div>

            <div v-for="(term, ti) in effect.formulaTerms" :key="ti" class="formula-row">
              <span class="term-attr">{{ term.attrLabel }}<span class="dim term-key">({{ term.attrKey }})</span></span>
              <span class="term-coeff">{{ term.coefficient }}<span v-if="term.perSkillLevel !== 0" class="term-perlv"> +{{ term.perSkillLevel }}/级</span></span>
              <span class="term-op">÷</span>
              <!-- 换算信息：显示最优来源的换算率 -->
              <span class="term-src" :class="{ 'term-src-primary': term.primaryEquivSource === term.attrKey, 'term-src-unknown': !term.primaryEquivSource }"
                    :title="term.primaryEquivSource ? (term.primaryEquivSource === term.attrKey ? '本身为主属性，换算系数=1' : `最优来源: ${term.primaryEquivSource}，换算率 = 1/${term.primaryEquivFactor.toFixed(4)} = ${(1/term.primaryEquivFactor).toFixed(4)}/点`) : '无已知来源，使用 unitValue 估算'">
                <span v-if="term.primaryEquivFactor === 1 && term.primaryEquivSource === term.attrKey" class="src-label-primary">主属性</span>
                <span v-else-if="term.primaryEquivSource">÷{{ (1/term.primaryEquivFactor).toFixed(4) }}<span class="dim src-from"> from {{ term.primaryEquivSource }}</span></span>
                <span v-else class="dim">?</span>
              </span>
              <span class="term-eq">=</span>
              <span class="term-w">{{ term.termWeight.toFixed(3) }}</span>
            </div>

            <div class="formula-header-row">
              <span class="dim">属性</span>
              <span class="dim">系数</span>
              <span></span>
              <span class="dim">换算(主属性点/1该属性)</span>
              <span></span>
              <span class="dim">主属性权重</span>
            </div>

            <div class="formula-sum-row">
              <span class="sum-label">每次命中 · 主属性等价权重合计</span>
              <span class="sum-val">{{ effect.rawFormulaWeight.toFixed(3) }}</span>
            </div>
            <div class="formula-sum-row hi-row">
              <span class="sum-label">× {{ effect.hitDamageScaling ? '衰减后等效命中' : '命中次数' }} {{ effect.hitDamageEquivalent.toFixed(effect.hitDamageScaling ? 4 : 0) }}</span>
              <span class="sum-val hi">→ 每目标权重 {{ perTargetWeight(effect).toFixed(3) }} 主属性点</span>
            </div>
            <div class="formula-sum-row">
              <span class="sum-label">目标因子 {{ multiTargetFactorDisplay(effect, MULTI_DISCOUNT) }}</span>
              <span class="sum-val">有效总权重 ≈ {{ effectiveTotalWeight(effect, MULTI_DISCOUNT).toFixed(3) }} 主属性点</span>
            </div>
          </div>

          <div v-else class="dim" style="margin: 4px 0 4px 60px; font-size: 11px;">无伤害公式</div>
        </template>

        <!-- Score line -->
        <div class="info-row score-row">
          <span class="row-label">效果分</span>
          <span class="row-value score-num">{{ effect.effectScore }}</span>
        </div>
      </div>

      <!-- Balance verdict -->
      <div class="verdict-block">
        <div class="verdict-title">平衡结论</div>
        <div class="verdict-row">
          <span class="dim">等级预算</span>
          <strong>{{ (overview as SkillOverview).balance.baseBudget }}</strong>
          <span class="dim">×</span>
          <span>{{ (overview as SkillOverview).balance.rarityLabel }}({{ (overview as SkillOverview).balance.rarityMult }})</span>
          <span class="dim">=</span>
          <span>允许 <strong>{{ (overview as SkillOverview).balance.budgetAllowed }}</strong></span>
        </div>
        <div class="verdict-row">
          <span class="dim">技能总分</span>
          <strong>{{ (overview as SkillOverview).balance.totalEffectScore }}</strong>
          <span class="verdict-badge" :class="verdictClass((overview as SkillOverview).balance.status)">
            {{ (Math.round((overview as SkillOverview).balance.ratio * 1000) / 10).toFixed(1) }}%
            {{ verdictIcon((overview as SkillOverview).balance.status) }}
            {{ verdictText((overview as SkillOverview).balance.status) }}
          </span>
        </div>
        <div v-if="(overview as SkillOverview).balance.suggestedRarity" class="verdict-suggest">
          建议最低稀有度：<strong>{{ (overview as SkillOverview).balance.suggestedRarityLabel }} ({{ (overview as SkillOverview).balance.suggestedRarity }})</strong>
        </div>
        <div class="verdict-note dim">
          注：公式权重 = 主属性等价点数（由换算表推算，取最优来源）。如 physics_damage÷0.55 对应「1力量=0.55物理攻击」→需 1/0.55≈1.82力量获得1物理攻击。多目标评分按 +{{ (MULTI_DISCOUNT * 100).toFixed(0) }}%/额外目标 折算。换算表可在数值模型配置中调整。
        </div>
      </div>
    </template>

    <!-- EQUIPMENT OVERVIEW -->
    <template v-else-if="table === 'equipment_templates' && !isSkillOverview(overview)">
      <div class="overview-header">
        <strong>装备概览</strong>
        <span v-if="(overview as EquipmentOverview).slot" class="meta-pill">{{ (overview as EquipmentOverview).slot }}</span>
        <span class="meta-pill">强化上限 {{ (overview as EquipmentOverview).enhanceMaxLevel }}</span>
      </div>

      <div v-if="(overview as EquipmentOverview).basicAttackEffects.length === 0" class="dim" style="padding: 4px 0;">
        无普攻效果
      </div>

      <div v-for="(effect, i) in (overview as EquipmentOverview).basicAttackEffects" :key="i" class="effect-block">
        <div class="effect-title">
          <span class="eff-num">普攻效果 #{{ i + 1 }}</span>
          <span class="kind-tag">{{ kindLabel(effect.kind) }}</span>
          <span v-if="effect.damageType" class="dtype-tag">{{ dtLabel(effect.damageType) }}</span>
        </div>

        <div class="info-row">
          <span class="row-label">目标</span>
          <span class="row-value">
            <span v-for="pos in effect.targetPositions" :key="pos" class="pos-tag">{{ posLabel(pos) }}</span>
            <span v-if="!effect.targetPositions.length" class="dim">-</span>
            <span>{{ scopeLabel(effect.targetScope) }}</span>
            <span class="sep">·</span>
            <span>{{ seekLabel(effect.secondarySeek) }}</span>
            <span class="sep">·</span>
            <span>{{ ruleLabel(effect.selectRule) }}</span>
            <span class="sep">·</span>
            <strong class="hi">{{ targetCountText(effect) }} 目标</strong>
          </span>
        </div>

        <template v-if="hasFormula(effect)">
          <div class="info-row">
            <span class="row-label">伤害次数</span>
            <span class="row-value"><strong class="hi">{{ effect.hitCountBase }} 次</strong><span v-if="effect.hitDamageScaling" class="dim"> · {{ hitDamageScalingText(effect) }}</span></span>
          </div>

          <div v-if="effect.formulaTerms.length > 0 || effect.baseAdd !== 0" class="formula-block">
            <div class="formula-title">公式权重拆解 <span class="formula-hint">（系数 ÷ 每主属性点换算率 = 主属性等价权重）</span></div>

            <div v-if="effect.baseAdd !== 0" class="formula-row">
              <span class="term-attr">基础值</span>
              <span class="term-coeff">{{ effect.baseAdd }}</span>
              <span class="term-op">÷</span>
              <span class="term-src dim">—</span>
              <span class="term-eq">=</span>
              <span class="term-w">{{ Math.abs(effect.baseAdd).toFixed(3) }}</span>
            </div>

            <div v-for="(term, ti) in effect.formulaTerms" :key="ti" class="formula-row">
              <span class="term-attr">{{ term.attrLabel }}<span class="dim term-key">({{ term.attrKey }})</span></span>
              <span class="term-coeff">{{ term.coefficient }}<span v-if="term.perSkillLevel !== 0" class="term-perlv"> +{{ term.perSkillLevel }}/级</span></span>
              <span class="term-op">÷</span>
              <span class="term-src"
                    :class="{ 'term-src-primary': term.primaryEquivSource === term.attrKey, 'term-src-unknown': !term.primaryEquivSource }"
                    :title="term.primaryEquivSource === term.attrKey ? '本身为主属性，等价系数=1' : term.primaryEquivSource ? `最优来源: ${term.primaryEquivSource}，换算率=${(1/term.primaryEquivFactor).toFixed(4)}该属性/主属性点` : '无已知来源'">
                <template v-if="term.primaryEquivFactor === 1 && term.primaryEquivSource === term.attrKey">
                  <span class="src-label-primary">主属性</span>
                </template>
                <template v-else-if="term.primaryEquivSource">
                  {{ (1/term.primaryEquivFactor).toFixed(4) }}<span class="dim src-from"> ({{ term.primaryEquivSource }})</span>
                </template>
                <template v-else><span class="dim">?</span></template>
              </span>
              <span class="term-eq">=</span>
              <span class="term-w">{{ term.termWeight.toFixed(3) }}</span>
            </div>

            <div class="formula-sum-row">
              <span class="sum-label">每次命中 · 主属性等价权重合计</span>
              <span class="sum-val">{{ effect.rawFormulaWeight.toFixed(3) }}</span>
            </div>
            <div class="formula-sum-row hi-row">
              <span class="sum-label">× {{ effect.hitDamageScaling ? '衰减后等效命中' : '命中次数' }} {{ effect.hitDamageEquivalent.toFixed(effect.hitDamageScaling ? 4 : 0) }}</span>
              <span class="sum-val hi">→ 每目标权重 {{ perTargetWeight(effect).toFixed(3) }} 主属性点</span>
            </div>
            <div v-if="effect.targetCountBase > 1 || (effect.targetCountMax ?? 0) > 1" class="formula-sum-row">
              <span class="sum-label">目标因子 {{ multiTargetFactorDisplay(effect, MULTI_DISCOUNT) }}</span>
              <span class="sum-val">有效总权重 ≈ {{ effectiveTotalWeight(effect, MULTI_DISCOUNT).toFixed(3) }}</span>
            </div>
          </div>
        </template>

        <div class="info-row score-row">
          <span class="row-label">效果分</span>
          <span class="row-value score-num">{{ effect.effectScore }}</span>
        </div>
      </div>

      <div v-if="(overview as EquipmentOverview).battleHookCount > 0" class="mechanism-block">
        <div class="mechanism-title">
          <span>装备机制</span>
          <strong class="hi">机制分 {{ (overview as EquipmentOverview).mechanismScore }}</strong>
        </div>
        <div v-for="hook in (overview as EquipmentOverview).battleHooks" :key="hook.id" class="hook-row">
          <span class="trigger-tag">{{ triggerLabel(hook.trigger) }}</span>
          <span class="row-value">
            <span v-for="kind in hook.effectKinds" :key="kind" class="kind-tag">{{ kindLabel(kind) }}</span>
            <span v-if="hook.conditionCount > 0" class="dim">条件 {{ hook.conditionCount }}</span>
            <span v-if="hook.hidden" class="dim">隐藏文案</span>
          </span>
        </div>
      </div>

      <div v-if="(overview as EquipmentOverview).affixMilestones.length > 0" class="mechanism-block">
        <div class="mechanism-title">
          <span>强化里程碑词条</span>
        </div>
        <div v-for="milestone in (overview as EquipmentOverview).affixMilestones" :key="milestone.level" class="hook-row">
          <span class="trigger-tag">+{{ milestone.level }}</span>
          <span class="row-value">
            <span v-for="entry in milestone.entries" :key="entry.affixId + entry.rarityId" class="kind-tag">
              {{ entry.affixId }} · {{ entry.rarityId }}<template v-if="entry.rollValue !== null"> · {{ entry.rollValue }}</template>
            </span>
          </span>
        </div>
      </div>

      <div v-if="(overview as EquipmentOverview).basicAttackEffects.length > 0" class="verdict-block">
        <div class="verdict-title">普攻平衡</div>
        <div class="verdict-row">
          <span class="dim">普攻总分</span>
          <strong>{{ (overview as EquipmentOverview).balance.totalEffectScore }}</strong>
          <span class="verdict-badge" :class="verdictClass((overview as EquipmentOverview).balance.status)">
            {{ verdictIcon((overview as EquipmentOverview).balance.status) }}
            {{ verdictText((overview as EquipmentOverview).balance.status) }}
          </span>
        </div>
        <div v-if="(overview as EquipmentOverview).balance.suggestedRarity" class="verdict-suggest">
          建议最低稀有度：<strong>{{ (overview as EquipmentOverview).balance.suggestedRarityLabel }}</strong>
        </div>
      </div>
    </template>
  </section>
</template>

<style scoped>
.overview-panel {
  border: 1px solid color-mix(in srgb, currentColor 18%, transparent);
  border-radius: 6px;
  padding: 12px 14px;
  margin-bottom: 12px;
  font-size: 12px;
}

.overview-header {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 10px;
  padding-bottom: 7px;
  border-bottom: 1px solid color-mix(in srgb, currentColor 12%, transparent);
  font-size: 13px;
}

.meta-pill {
  font-size: 11px;
  padding: 1px 6px;
  border-radius: 10px;
  border: 1px solid color-mix(in srgb, currentColor 20%, transparent);
  opacity: 0.7;
}

.effect-block {
  margin-bottom: 10px;
  padding: 8px 10px;
  border-radius: 4px;
  border: 1px solid color-mix(in srgb, currentColor 12%, transparent);
}

.effect-title {
  display: flex;
  align-items: center;
  gap: 5px;
  margin-bottom: 6px;
}

.mechanism-block {
  margin-top: 10px;
  padding: 8px 10px;
  border-radius: 4px;
  border: 1px solid color-mix(in srgb, currentColor 12%, transparent);
}

.mechanism-title {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  margin-bottom: 6px;
  font-size: 11px;
  font-weight: 700;
}

.hook-row {
  display: flex;
  align-items: baseline;
  gap: 6px;
  margin-top: 4px;
  flex-wrap: wrap;
}

.trigger-tag {
  min-width: 76px;
  font-size: 10px;
  padding: 1px 5px;
  border-radius: 3px;
  border: 1px solid color-mix(in srgb, #7cb8e8 45%, transparent);
  color: #7cb8e8;
}

.eff-num {
  font-weight: 600;
  font-size: 12px;
}

.kind-tag, .dtype-tag {
  font-size: 10px;
  padding: 1px 5px;
  border-radius: 3px;
  border: 1px solid color-mix(in srgb, currentColor 20%, transparent);
  opacity: 0.75;
}

.dtype-tag {
  border-color: color-mix(in srgb, orange 40%, transparent);
  color: orange;
}

.info-row {
  display: flex;
  align-items: baseline;
  gap: 5px;
  margin-bottom: 3px;
  flex-wrap: wrap;
}

.row-label {
  font-size: 10px;
  opacity: 0.55;
  min-width: 42px;
  flex-shrink: 0;
}

.row-value {
  flex: 1;
  font-size: 11px;
  display: flex;
  align-items: baseline;
  gap: 4px;
  flex-wrap: wrap;
}

.pos-tag {
  font-size: 10px;
  padding: 1px 4px;
  border-radius: 3px;
  border: 1px solid color-mix(in srgb, cornflowerblue 50%, transparent);
  color: cornflowerblue;
  flex-shrink: 0;
}

.sep { opacity: 0.4; }
.dim { opacity: 0.5; }
.hi { color: #7cb8e8; }

.score-row {
  border-top: 1px solid color-mix(in srgb, currentColor 10%, transparent);
  padding-top: 3px;
  margin-top: 4px;
}

.score-num {
  font-weight: 700;
  color: #7ec898;
}

/* Formula breakdown */
.formula-block {
  margin: 5px 0 5px 44px;
  padding: 7px 8px;
  border-radius: 3px;
  border: 1px solid color-mix(in srgb, currentColor 10%, transparent);
  border-left: 2px solid color-mix(in srgb, currentColor 20%, transparent);
}

.formula-title {
  font-size: 10px;
  opacity: 0.55;
  margin-bottom: 5px;
  font-weight: 600;
  letter-spacing: 0.03em;
}

.formula-hint {
  font-weight: 400;
  opacity: 0.8;
}

.formula-row {
  display: grid;
  grid-template-columns: minmax(120px, 1fr) minmax(60px, auto) 16px 56px 16px 50px;
  align-items: baseline;
  gap: 4px;
  margin-bottom: 3px;
  font-size: 11px;
}

.term-attr { font-family: inherit; }
.term-key { font-size: 9px; margin-left: 2px; }
.term-coeff { font-weight: 600; color: #e8c878; text-align: right; }
.term-perlv { font-size: 9px; opacity: 0.6; }
.term-op { opacity: 0.4; text-align: center; }
.term-src { text-align: right; font-family: monospace; font-size: 10px; }
.term-src-primary .src-label-primary { color: #80c8a0; font-style: italic; }
.term-src-unknown { opacity: 0.4; }
.src-from { font-size: 9px; }
.term-eq { opacity: 0.4; text-align: center; }
.term-w { font-weight: 700; color: #d4a860; text-align: right; }

.formula-sum-row {
  display: flex;
  justify-content: space-between;
  align-items: baseline;
  margin-top: 4px;
  padding-top: 3px;
  border-top: 1px dashed color-mix(in srgb, currentColor 15%, transparent);
  font-size: 11px;
}

.hi-row {
  border-top-color: color-mix(in srgb, currentColor 20%, transparent);
}

.sum-label { opacity: 0.7; }
.sum-val { font-weight: 600; color: #d4a860; }
.sum-val.hi { color: #e88040; font-size: 12px; }

/* Verdict */
.verdict-block {
  margin-top: 10px;
  padding: 7px 10px;
  border-radius: 4px;
  border: 1px solid color-mix(in srgb, currentColor 12%, transparent);
  border-left: 3px solid color-mix(in srgb, currentColor 30%, transparent);
}

.verdict-title {
  font-size: 10px;
  font-weight: 700;
  opacity: 0.55;
  letter-spacing: 0.05em;
  text-transform: uppercase;
  margin-bottom: 5px;
}

.verdict-row {
  display: flex;
  align-items: baseline;
  gap: 6px;
  margin-bottom: 2px;
  font-size: 11px;
  flex-wrap: wrap;
}

.verdict-badge {
  font-weight: 700;
  font-size: 12px;
  margin-left: auto;
  padding: 1px 6px;
  border-radius: 3px;
}

.verdict-ok    { color: #3dd68c; border: 1px solid #3dd68c40; }
.verdict-caution { color: #f0b020; border: 1px solid #f0b02040; }
.verdict-overtuned { color: #f06060; border: 1px solid #f0606040; }

.verdict-suggest {
  font-size: 10px;
  opacity: 0.75;
  margin-top: 3px;
}

.verdict-note {
  font-size: 9px;
  margin-top: 5px;
  line-height: 1.5;
  border-top: 1px solid color-mix(in srgb, currentColor 10%, transparent);
  padding-top: 4px;
}
</style>
