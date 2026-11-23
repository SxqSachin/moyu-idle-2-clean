import { readFileSync } from "node:fs"
import { resolve } from "node:path"
import { describe, expect, it } from "vitest"
import enGameData from "../src/locales/game-data/en-US"
import zhGameData from "../src/locales/game-data/zh-CN"
import enGame from "../src/locales/game/en-US"
import zhGame from "../src/locales/game/zh-CN"
import { buildAffixMutationDisplayLines } from "../src/services/affix-mutation-display"
import { buildBattleHookDescriptions } from "../src/services/battle-hook-display"
import { buildSpecializationGrantDisplayLines } from "../src/services/specialization-grant-display"
import type { SpecializationGrant } from "../src/services/specialization"

const sourcePath = resolve(process.cwd(), "../packages/game-data/source/json/specialization_trees.json")
const tree = JSON.parse(readFileSync(sourcePath, "utf8")).specialization_trees[0]

function translate(locale: Record<string, unknown>, key: string, params: Record<string, unknown> = {}): string {
  const attributes: Record<string, string> = {
    "attr.strength.name": "力量",
    "attr.dexterity.name": "敏捷",
    "attr.intelligence.name": "智力",
    "attr.perception.name": "感知",
    "attr.stamina.name": "耐力"
  }
  if (attributes[key]) return attributes[key]
  let cursor: unknown = locale
  for (const part of key.split(".")) {
    if (!cursor || typeof cursor !== "object" || Array.isArray(cursor) || !(part in cursor)) return key
    cursor = (cursor as Record<string, unknown>)[part]
  }
  if (typeof cursor !== "string") return key
  return cursor.replace(/\{([^}]+)\}/g, (_, name: string) => String(params[name] ?? `{${name}}`))
}

const zhLocale = { ...zhGame, ...zhGameData } as Record<string, unknown>
const enLocale = { ...enGame, ...enGameData } as Record<string, unknown>
const t = (key: string, params: Record<string, unknown> = {}) => translate(zhLocale, key, params)
const enT = (key: string, params: Record<string, unknown> = {}) => translate(enLocale, key, params)

describe("specialization DSL display projection", () => {
  it("projects every configured hook without raw camel-case triggers or unknown DSL kinds", () => {
    const hooks = tree.nodes.flatMap((node: { levels: Array<{ grants: Array<{ battleHooks?: unknown[] }> }> }) =>
      node.levels.flatMap((level) => level.grants.flatMap((grant) => grant.battleHooks || []))
    )
    const descriptions = buildBattleHookDescriptions(hooks, t, {
      sourceKind: "specialization",
      unitAttributes: { strength: 20, dexterity: 20, intelligence: 20, perception: 20, stamina: 20 }
    })
    const text = JSON.stringify(descriptions)

    expect(descriptions).toHaveLength(hooks.length)
    expect(text).not.toContain("未知条件")
    expect(text).not.toContain("未知效果")
    expect(text).not.toContain("特殊时机")
    expect(text).not.toMatch(/on(?:Before|After|Battle|Turn|Wave|Kill|Ally)/)
    expect(text).toContain("破势")
    expect(text).toContain("最多积累 2 层")
    expect(text).toContain("破势至少有 2 层")
    expect(text).toContain("消耗 2 层破势")
    expect(text).toContain("每个敌人刷新波次最多 1 次")
    expect(text).toContain("威胁最高的敌方")
    expect(text).toContain("当前生命最低的敌方")
    expect(text).not.toContain("lowest_hp")

    const perBattleText = JSON.stringify(buildBattleHookDescriptions([{
      id: "per_battle_copy_probe",
      trigger: "onAfterDealDamage",
      priority: 0,
      conditions: [],
      effects: [{ kind: "runtime_damage_amp", value: 0.01 }],
      limits: { perBattle: 1 },
      dispatchHooks: false
    }], t, { sourceKind: "specialization", unitAttributes: {} }))
    expect(perBattleText).toContain("每场战斗（跨所有敌人刷新波次）最多触发 1 次")
  })

  it("keeps every level's hook effect visible and stat grants free of fake triggers", () => {
    for (const node of tree.nodes as Array<{ id: string; levels: Array<{ level: number; grants: SpecializationGrant[] }> }>) {
      for (const level of node.levels) {
        for (const grant of level.grants) {
          const lines = buildSpecializationGrantDisplayLines(grant, {
            t,
            localized: (key, fallback) => {
              const value = t(key)
              return value === key ? fallback : value
            },
            skillName: (skillId) => skillId,
            slotFamilyLabel: (family) => family,
            characterAttributes: { strength: 20, dexterity: 20, intelligence: 20, perception: 20, stamina: 20 },
            includeHookMetadata: true
          })
          const label = `${node.id} Lv.${level.level} ${grant.id}`
          if (grant.kind === "battle_hook") {
            expect(lines.length, `${label} has no player-visible technical projection`).toBeGreaterThan(0)
            expect(lines[0], `${label} does not start with its trigger`).toMatch(/^触发：/)
            let hasTrigger = false
            let hasEffect = false
            for (const line of lines) {
              if (line.startsWith("触发：")) {
                hasTrigger = true
                hasEffect = false
                continue
              }
              if (line.startsWith("条件：")) {
                expect(hasTrigger, `${label} puts a condition before its trigger`).toBe(true)
                expect(hasEffect, `${label} puts a condition after an effect in the same hook`).toBe(false)
                continue
              }
              if (line.startsWith("效果：")) {
                expect(hasTrigger, `${label} puts an effect before its trigger`).toBe(true)
                hasEffect = true
              }
            }
            expect(lines.some((line) => line.startsWith("效果：")), `${label} has no effect line`).toBe(true)
            expect(lines.join("\n"), `${label} fell through the unknown-effect renderer`).not.toContain("触发一项特殊效果")
          }
          if (grant.kind === "stat_modifier") {
            expect(lines.join("\n"), `${label} invented hook metadata for a passive stat grant`).not.toMatch(/(?:触发|条件)：/)
          }
        }
      }
    }
  })

  it("evaluates specialization value formulas from current attributes and treats missing attributes as zero", () => {
    const hook = [{
      id: "formula_preview",
      trigger: "onBeforeDealDamage",
      priority: 0,
      conditions: [],
      effects: [{
        kind: "runtime_damage_amp",
        value: 0.15,
        valueFormula: {
          baseAdd: 0.09,
          terms: [{ attributeKey: "strength", coefficient: 0.003 }]
        }
      }],
      dispatchHooks: false
    }]
    const atTwenty = JSON.stringify(buildBattleHookDescriptions(hook, t, {
      sourceKind: "specialization",
      unitAttributes: { strength: 20 }
    }))
    const atForty = JSON.stringify(buildBattleHookDescriptions(hook, t, {
      sourceKind: "specialization",
      unitAttributes: { strength: 40 }
    }))
    const missing = JSON.stringify(buildBattleHookDescriptions([{ ...hook[0], effects: [{
      kind: "runtime_damage_amp",
      value: 0.5,
      valueFormula: {
        baseAdd: 0.02,
        terms: [{ attributeKey: "unknown_attr", coefficient: 0.01 }]
      }
    }] }], t, {
      sourceKind: "specialization",
      unitAttributes: {}
    }))

    expect(atTwenty).toContain("力量")
    expect(atTwenty).toContain("0.003")
    expect(atTwenty).toContain("当前属性投影：+15%")
    expect(atForty).toContain("当前属性投影：+21%")
    expect(atForty).not.toEqual(atTwenty)
    expect(missing).toContain("unknown_attr")
    expect(missing).toContain("当前属性投影：+2%")
    expect(missing).not.toContain("+50%")
  })

  it("keeps the retired action-meter effect explicit about its threshold instead of implying turn-order progress", () => {
    const lines = buildBattleHookDescriptions([{
      id: "legacy_action_charge",
      trigger: "onAfterTurn",
      priority: 0,
      conditions: [],
      effects: [{ kind: "action_meter_gain", value: 0.25 }],
      dispatchHooks: false
    }], t, { sourceKind: "specialization", unitAttributes: {} })
    const text = JSON.stringify(lines)

    expect(text).toContain("额外行动蓄力 +25%")
    expect(text).toContain("累计至 100% 时立即完整行动 1 次")
    expect(text).not.toMatch(/出手提前|行动进度/)
  })

  it("projects every configured mutation selector and operation", () => {
    const mutations = tree.nodes.flatMap((node: { levels: Array<{ grants: Array<{ effectMutations?: unknown[] }> }> }) =>
      node.levels[0].grants.flatMap((grant) => grant.effectMutations || [])
    )
    const lines = buildAffixMutationDisplayLines(mutations, t)
    const text = lines.map((line) => `${line.label}:${line.value}`).join("\n")

    expect(lines.length).toBeGreaterThan(mutations.length)
    expect(text).toContain("主动技能")
    expect(text).toContain("命中次数")
    expect(text).toContain("冷却最低保留 1 回合")
    expect(text).toContain("魔法消耗变为原来的 450%")
    expect(text).toContain("力量")
    expect(text).toContain("扩散")
    expect(text).toContain("伤害公式采用物理攻击与魔法攻击中的较高值，伤害类型不变")
    expect(text).not.toContain("attributeKey")
    expect(text).not.toContain("attackStatPolicy")
  })

  it("lists recognized debuffs only when the hook actually requires them", () => {
    const renderNode = (nodeId: string) => {
      const node = tree.nodes.find((entry: { id: string }) => entry.id === nodeId)
      return buildSpecializationGrantDisplayLines(node.levels[0].grants[0], {
        t,
        localized: (_key, fallback) => fallback,
        skillName: (skillId) => skillId,
        slotFamilyLabel: (family) => family,
        characterAttributes: {},
        includeHookMetadata: true
      }).join("\n")
    }

    const plainFollowUp = renderNode("dexterity_root_deadly_strike")
    expect(plainFollowUp).not.toContain("指定减益")
    expect(plainFollowUp).not.toContain("perception_observed_gap")

    const debuffFollowUp = renderNode("confluence_dexterity_perception_hunt_sequence")
    expect(debuffFollowUp).toContain("观察")
    expect(debuffFollowUp).toContain("哨卫锁定")
    expect(debuffFollowUp).not.toContain("perception_observed_gap")
    expect(debuffFollowUp).not.toContain("specialization_sentinel_target")
    expect(debuffFollowUp.indexOf("触发：")).toBeLessThan(debuffFollowUp.indexOf("条件："))
    expect(debuffFollowUp.indexOf("条件：")).toBeLessThan(debuffFollowUp.indexOf("效果："))
  })

  it("shows One More Move as a capped rank-based probability", () => {
    const node = tree.nodes.find((entry: { id: string }) => entry.id === "dexterity_terminal_stolen_beat")
    const renderLevel = (levelIndex: number) => buildSpecializationGrantDisplayLines(node.levels[levelIndex].grants[0], {
      t,
      localized: (_key, fallback) => fallback,
      skillName: (skillId) => skillId,
      slotFamilyLabel: (family) => family,
      characterAttributes: {},
      includeHookMetadata: true
    }).join("\n")

    expect(renderLevel(0)).toContain("20%")
    expect(renderLevel(0)).toContain("最高 40%")
    expect(renderLevel(1)).toContain("30%")
    expect(renderLevel(2)).toContain("40%")
  })

  it("localizes weapon gates in both supported data namespaces", () => {
    const node = tree.nodes.find((entry: { id: string }) => entry.id === "dexterity_terminal_stolen_beat")
    const grant = node.levels[0].grants[0]
    const render = (translateFn: typeof t) => buildSpecializationGrantDisplayLines(grant, {
      t: translateFn,
      localized: (_key, fallback) => fallback,
      skillName: (skillId) => skillId,
      slotFamilyLabel: (family) => family,
      characterAttributes: {},
      includeHookMetadata: true
    }).join("\n")
    const zhText = render(t)
    const enText = render(enT)

    expect(zhText).toContain("战镰")
    expect(zhText).not.toContain("scythe")
    expect(enText).toContain("Scythe")
  })

  it("shows Critical Exposure through both the mind-orb and scythe specialization paths", () => {
    const renderGrant = (nodeId: string, grantId: string, translateFn: typeof t) => {
      const node = tree.nodes.find((entry: { id: string }) => entry.id === nodeId)
      const grant = node.levels[0].grants.find((entry: { id: string }) => entry.id === grantId)
      return buildSpecializationGrantDisplayLines(grant, {
        t: translateFn,
        localized: (_key, fallback) => fallback,
        skillName: (skillId) => skillId,
        slotFamilyLabel: (family) => family,
        characterAttributes: {},
        includeHookMetadata: true
      }).join("\n")
    }

    const mindOrbZh = renderGrant("perception_tech_reveal_weakness", "grant_perception_mind_orb_critical_exposure", t)
    const mindOrbEn = renderGrant("perception_tech_reveal_weakness", "grant_perception_mind_orb_critical_exposure", enT)
    const scytheZh = renderGrant("confluence_dexterity_perception_hunt_sequence", "grant_rework_confluence_dexterity_perception_hunt_sequence", t)

    expect(mindOrbZh).toContain("念珠")
    expect(mindOrbZh).toContain("暴击破绽")
    expect(mindOrbEn).toContain("Mind Orb")
    expect(mindOrbEn).toContain("Critical Exposure")
    expect(scytheZh).toContain("战镰")
    expect(scytheZh).toContain("暴击破绽")
    expect(tree.i18n["zh-CN"].nodes.perception_tech_reveal_weakness.levelEffectSummary[0].join(" ")).toContain("30 个百分点")
    expect(tree.i18n["zh-CN"].nodes.confluence_dexterity_perception_hunt_sequence.levelEffectSummary[0].join(" ")).toContain("30 个百分点")
  })

  it("localizes targeting patch fields and values without leaking DSL property names", () => {
    const node = tree.nodes.find((entry: { id: string }) => entry.id === "confluence_intelligence_perception_mind_lock")
    const mutation = node.levels[0].grants[0].effectMutations
    const zhText = buildAffixMutationDisplayLines(mutation, t).map((line) => `${line.label}:${line.value}`).join("\n")
    const enText = buildAffixMutationDisplayLines(mutation, enT).map((line) => `${line.label}:${line.value}`).join("\n")

    for (const text of [zhText, enText]) {
      expect(text).not.toContain("fallbackSelfIfEmpty")
      expect(text).not.toContain("secondarySeek")
      expect(text).not.toContain("selectRule")
      expect(text).not.toContain("targetScope")
    }
    expect(zhText).toContain("目标阵营：敌方")
    expect(zhText).toContain("选择规则：生命比例最低")
    expect(zhText).toContain("次要寻敌：前排到后排")
    expect(zhText).toContain("无可用目标时改选自身：否")
    expect(enText).toContain("target side: enemy")
    expect(enText).toContain("selection rule: Lowest HP ratio")
  })

  it("renders bounded weapon secondary targets with localized row policies", () => {
    const renderMutation = (nodeId: string, translateFn: typeof t) => {
      const node = tree.nodes.find((entry: { id: string }) => entry.id === nodeId)
      const mutation = node.levels.at(-1).grants[0].effectMutations
      return buildAffixMutationDisplayLines(mutation, translateFn).map((line) => `${line.label}:${line.value}`).join("\n")
    }
    const scytheZh = renderMutation("dexterity_terminal_reaping_sweep", t)
    const spearZh = renderMutation("perception_terminal_foresight", t)
    const longswordZh = renderMutation("confluence_strength_dexterity_swift_force", t)
    const spearEn = renderMutation("perception_terminal_foresight", enT)

    expect(scytheZh).toContain("原目标同排的 1 个其他敌人")
    expect(scytheZh).toContain("40%")
    expect(spearZh).toContain("后排")
    expect(spearZh).toContain("45%")
    expect(longswordZh).toContain("任意排位的 1 个其他敌人")
    expect(longswordZh).toContain("36%")
    expect(spearEn).toMatch(/rear/i)
    for (const text of [scytheZh, spearZh, longswordZh, spearEn]) {
      expect(text).toContain("1")
      expect(text).not.toMatch(/secondary_damage|same_row|fixed_row|all_rows|rearguard/)
    }
  })

  it("shows the current attribute result beside an appended mutation formula", () => {
    const lines = buildAffixMutationDisplayLines([{
      scope: "both",
      effectSelector: { mode: "all" },
      ops: {
        amountTermsAppend: [{ attributeKey: "dexterity", coefficient: 0.06 }]
      }
    }], t, { unitAttributes: { dexterity: 24 } })
    const text = lines.map((line) => `${line.label}:${line.value}`).join("\n")

    expect(text).toContain("+0.06x敏捷")
    expect(text).toContain("按当前属性：+1.44")
  })

  it("renders enemy buff selection instead of falsely saying the buff is applied to self", () => {
    const node = tree.nodes.find((entry: { id: string }) => entry.id === "perception_terminal_preemptive_strike")
    const lines = buildSpecializationGrantDisplayLines(node.levels[0].grants[0], {
      t,
      localized: (_key, fallback) => fallback,
      skillName: (skillId) => skillId,
      slotFamilyLabel: (family) => family,
      characterAttributes: {},
      includeHookMetadata: true
    })
    const text = lines.join("\n")

    expect(text).toContain("给威胁最高的敌方附加 抢先锁定")
    expect(text).toContain("每波开始时")
    expect(text).not.toContain("给自己附加 抢先锁定")
    expect(text).not.toContain("specialization_preemptive_target")
  })

  it("keeps specialization ally and event-target modes distinct from generic enemy targeting", () => {
    const renderNode = (nodeId: string) => {
      const node = tree.nodes.find((entry: { id: string }) => entry.id === nodeId)
      return node.levels[0].grants.flatMap((grant: SpecializationGrant) => buildSpecializationGrantDisplayLines(grant, {
        t,
        localized: (_key, fallback) => fallback,
        skillName: (skillId) => skillId,
        slotFamilyLabel: (family) => family,
        characterAttributes: {},
        includeHookMetadata: true
      })).join("\n")
    }

    const allyHeal = renderNode("confluence_perception_stamina_watchful_aid")
    expect(allyHeal).toContain("生命比例最低的友方")
    expect(allyHeal).not.toContain("敌军中的前排目标")

    const judgement = renderNode("perception_terminal_judgement_window")
    expect(judgement).toContain("从当前目标移除")
    expect(judgement).toContain("对当前目标的任意类型技能")
  })
})
