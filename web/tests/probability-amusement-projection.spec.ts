import { describe, expect, it } from "vitest"
import zhGameData from "../src/locales/game-data/zh-CN"
import zhGame from "../src/locales/game/zh-CN"
import { buildBattleHookDescriptions } from "../src/services/battle-hook-display"
import { buildEffectDetailSections } from "../src/services/effect-detail-renderer"
import { parseWorldRows, parseWorldTriggerEncounters, parseWorldWaveSchedules } from "../src/composables/game/combat/useCombatPage"
import { projectDerivedFinalStats } from "../src/services/attribute-derived-impact"
import enemies from "@game-data/chunks/enemies.json"
import rawGlobalConfig from "@shared-config/global-config.json"

function t(key: string, params: Record<string, unknown> = {}): string {
  let localized: unknown
  for (const root of [zhGame, zhGameData]) {
    let cursor: unknown = root
    for (const part of key.split(".")) {
      if (!cursor || typeof cursor !== "object" || Array.isArray(cursor) || !(part in cursor)) {
        cursor = undefined
        break
      }
      cursor = (cursor as Record<string, unknown>)[part]
    }
    if (typeof cursor === "string") {
      localized = cursor
      break
    }
  }
  if (typeof localized !== "string") return key
  return localized.replace(/\{([^}]+)\}/g, (_, name: string) => String(params[name] ?? `{${name}}`))
}

describe("probability and amusement frontend projection", () => {
  it("renders probability filters, shared limits, and modifier outcomes", () => {
    const descriptions = buildBattleHookDescriptions([
      {
        id: "probability_result_hook",
        trigger: "on_probability_success",
        eventFilter: {
          probabilityKinds: ["equipment_proc"],
          sourceIds: ["cat_sword_echo"],
          excludeSourceIds: ["hollow_heart_lock"]
        },
        maxTriggersPerTurn: 1,
        maxTriggersPerWave: 2,
        maxTriggersPerBattle: 3,
        sharedTriggerGroup: "cat_sword_result",
        effects: [
          {
            kind: "add_probability",
            value: 0.12,
            valueFormula: {
              baseAdd: 0,
              terms: [{ attrKey: "equipment_enhance_level", coefficient: 0.001 }]
            },
            probabilityKinds: ["status_apply"]
          },
          { kind: "force_probability_failure", sourceIds: ["dark_cat_sword_trial"] }
        ]
      }
    ], t)

    expect(descriptions).toHaveLength(1)
    expect(descriptions[0]?.triggerText).toBe("概率判定成功后")
    const conditions = descriptions[0]?.conditions.map((entry) => entry.text).join(" / ") || ""
    expect(conditions).toContain("装备触发判定")
    expect(conditions).toContain(t("item.cat_sword.name"))
    expect(conditions).toContain(t("item.hollow_heart.name"))
    expect(conditions).not.toContain("cat_sword_echo")
    expect(conditions).not.toContain("hollow_heart_lock")
    expect(conditions).toContain("每回合最多触发 1 次")
    expect(conditions).toContain("每个敌人刷新波次最多触发 2 次")
    expect(conditions).toContain("每场战斗（跨所有敌人刷新波次）最多触发 3 次")
    expect(conditions).not.toContain("共享触发次数")
    expect(conditions).not.toContain("钩子")

    const effects = descriptions[0]?.effects.filter((effect) => effect.kind === "text") || []
    expect(effects).toHaveLength(2)
    expect(effects[0]?.kind === "text" ? effects[0].text : "").toContain("+12%")
    expect(effects[0]?.kind === "text" ? effects[0].text : "").toContain("状态施加")
    expect(effects[1]?.kind === "text" ? effects[1].text : "").toContain("必定失败")
    expect(effects[1]?.kind === "text" ? effects[1].text : "").toContain(t("item.dark_cat_sword.name"))
    expect(effects[1]?.kind === "text" ? effects[1].text : "").not.toContain("dark_cat_sword_trial")
  })

  it("keeps battle-item status chance, summon, weighted status, and cooldown details structured", () => {
    const sections = buildEffectDetailSections({
      variant: "battle_item",
      t,
      effects: [
        {
          kind: "status",
          targeting: { targetScope: "enemy" },
          statusPayload: {
            statusIds: ["buff_amusement_burn"],
            durationBaseTurns: 2,
            applyChanceBase: 0.4,
            applyChancePerSkillLevel: 0.01,
            applyChanceMin: 0.25,
            applyChanceMax: 0.8
          }
        },
        {
          kind: "summon",
          targeting: { targetScope: "self", selectRule: "self" },
          summonPayload: {
            unitId: "enemy_parade_decoy_001",
            side: "player",
            countFormula: { baseAdd: 1, terms: [] },
            durationFormula: { baseAdd: 3, terms: [] },
            durationTurns: 3,
            maxCountPerUnit: 1,
            onCapOverflow: "replace_oldest",
            statsFormula: {
              hp: { baseAdd: 0, terms: [{ attrKey: "max_hp", coefficient: 0.4 }] }
            }
          }
        },
        {
          kind: "weighted_random_per_target",
          targeting: { targetScope: "enemy" },
          statusPayload: {
            applyChanceBase: 0.45,
            applyChanceMin: 0.45,
            applyChanceMax: 0.45
          },
          choices: [
            { id: "burn", weight: 3, statusId: "buff_amusement_burn", durationTurns: 2 },
            { id: "stun", weight: 1, statusId: "buff_amusement_stun", durationTurns: 1 }
          ]
        },
        {
          kind: "adjust_skill_cooldown",
          targeting: { targetScope: "self", selectRule: "self" },
          amount: -1,
          includeBasic: true,
          skillIds: ["basic_attack"],
          excludeSkillIds: ["skill_locked"]
        }
      ]
    })

    expect(sections).toHaveLength(4)
    expect(sections[0]?.rows.find((row) => row.label === "施加概率")?.value).toContain("40%")
    expect(sections[0]?.rows.find((row) => row.label === "施加概率")?.value).toContain("25%~80%")
    expect(sections[1]?.rows.find((row) => row.label === "召唤单位")?.value).toBe("指定召唤物")
    expect(sections[1]?.rows.find((row) => row.label === "存在时间")?.value).toBe("3 回合")
    expect(sections[1]?.rows.find((row) => row.label === "同来源上限")?.value).toBe("1")
    expect(sections[1]?.rows.find((row) => row.label === "属性加成")?.value).toContain("最大生命值")
    expect(sections[2]?.rows.find((row) => row.label === "随机结果")?.value).toContain("75%")
    expect(sections[2]?.rows.find((row) => row.label === "随机结果")?.value).toContain("25%")
    expect(sections[2]?.rows.find((row) => row.label === "随机结果")?.value).not.toContain("buff_amusement_burn")
    expect(sections[2]?.rows.find((row) => row.label === "随机结果")?.value).not.toContain("权重")
    expect(sections[2]?.rows.find((row) => row.label === "施加概率")?.value).toContain("45%")
    expect(sections[3]?.rows.find((row) => row.label === "冷却变化")?.value).toBe("-1")
    expect(sections[3]?.rows.find((row) => row.label === "包含普通攻击")?.value).toBe("是")
  })

  it("labels drains, healing, cleansing, and summon HP in player terms", () => {
    const sections = buildEffectDetailSections({
      variant: "skill",
      t,
      effects: [
        {
          kind: "resource",
          resource: "mp",
          targeting: { targetScope: "enemy" },
          amountFormula: { baseAdd: -60, terms: [] }
        },
        {
          kind: "heal",
          damageType: "physical",
          targeting: { targetScope: "ally" },
          amountFormula: { baseAdd: 750, terms: [] }
        },
        {
          kind: "remove_buff",
          targeting: { targetScope: "ally" },
          buffIds: ["panic", "amusement_stun", "candy_sticky"],
          maxRemoved: 3
        },
        {
          kind: "summon",
          targeting: { targetScope: "ally", selectRule: "self" },
          summonPayload: {
            unitId: "plush_stunt_double_001",
            side: "player",
            countFormula: { baseAdd: 1, terms: [] },
            durationFormula: { baseAdd: 3, terms: [] },
            statsFormula: {
              hp: { baseAdd: -1, terms: [{ attrKey: "max_hp", coefficient: 0.4 }] }
            }
          }
        }
      ]
    })

    expect(sections[0]?.rows.find((row) => row.label === "效果")?.value).toBe("抽取资源")
    expect(sections[0]?.rows.find((row) => row.label === "数值")?.value).toBe("60")
    expect(sections[1]?.rows.some((row) => row.label === "效果类型")).toBe(false)
    expect(sections[2]?.rows.find((row) => row.label === "效果")?.value).toBe("净化状态")
    expect(sections[2]?.rows.find((row) => row.label === "移除状态")?.buffIds).toEqual(["panic", "amusement_stun", "candy_sticky"])
    expect(sections[2]?.rows.find((row) => row.label === "最多移除")?.value).toBe("3")
    const summonStats = sections[3]?.rows.find((row) => row.label === "属性加成")?.value || ""
    expect(summonStats).toContain("最大生命值的 40%")
    expect(summonStats).not.toContain("-1")
  })

  it("normalizes and prioritizes both amusement-park wave schedules", () => {
    const schedules = parseWorldWaveSchedules([
      {
        id: "amusement_elite_troupe",
        nameKey: "world.amusement_park_001.waveSchedule.amusement_elite_troupe.name",
        descKey: "world.amusement_park_001.waveSchedule.amusement_elite_troupe.desc",
        every: 4,
        priority: 10,
        enemyChoices: [
          { enemyId: "candy_clown_001", weight: 40 },
          { enemyId: "carousel_cat_001", weight: 30 },
          { enemyId: "ghostly_rider_001", weight: 30 }
        ],
        minCount: 1,
        maxCount: 3
      },
      {
        id: "amusement_parade_king",
        nameKey: "world.amusement_park_001.waveSchedule.amusement_parade_king.name",
        every: 8,
        priority: 20,
        enemyChoices: [{ enemyId: "parade_king_boss_001", weight: 1 }],
        minCount: 1,
        maxCount: 1
      },
      { id: "invalid", every: 0, enemyChoices: [] }
    ], (key) => ({
      "world.amusement_park_001.waveSchedule.amusement_elite_troupe.name": "游乐园精英剧团",
      "world.amusement_park_001.waveSchedule.amusement_elite_troupe.desc": "每四波加入一组剧团来客。",
      "world.amusement_park_001.waveSchedule.amusement_parade_king.name": "游行之王",
    }[key] || key))

    expect(schedules.map((schedule) => schedule.id)).toEqual(["amusement_parade_king", "amusement_elite_troupe"])
    expect(schedules[0]).toMatchObject({ label: "游行之王", every: 8, minCount: 1, maxCount: 1 })
    expect(schedules[0]?.enemyChoices).toHaveLength(1)
    expect(schedules[1]).toMatchObject({ label: "游乐园精英剧团", every: 4, description: "每四波加入一组剧团来客。" })
    expect(schedules[1]?.enemyChoices).toHaveLength(3)
  })

  it("keeps the localized world description for hidden encounter conditions", () => {
    const description = "携带鬼屋夜场券时，第17波起每8波还会追加午夜检票员。"
    const worlds = parseWorldRows((key) => ({
      "world.amusement_park_001.name": "猫咪游乐园",
      "world.amusement_park_001.desc": description
    }[key] || key))

    const amusementWorld = worlds.find((world) => world.id === "amusement_park_001")
    expect(amusementWorld).toMatchObject({
      label: "猫咪游乐园",
      description,
      triggerEncounters: [
        {
          id: "trigger_faceless_understudy_invitation",
          itemId: "faceless_understudy_invitation",
          enemyIds: ["faceless_understudy_001"],
          minWave: 17,
          intervalWaves: 8,
          perWaveChance: 1,
          partyTriggerGroup: "amusement_faceless_understudy"
        },
        {
          id: "trigger_haunted_after_hours_ticket",
          itemId: "haunted_after_hours_ticket",
          enemyIds: ["midnight_ticket_collector_001"],
          minWave: 17,
          intervalWaves: 8,
          perWaveChance: 1,
          partyTriggerGroup: "amusement_after_hours"
        }
      ]
    })
    expect(amusementWorld?.difficultySettings?.maxLevel).toBe(10)
    expect(amusementWorld?.difficultySettings?.statBonuses[0]).toEqual({
      attrKey: "hp",
      base: 1402.5,
      perLevelMult: 1.4
    })
  })

  it("projects only enabled trigger-item enemies into their referenced world", () => {
    const encounters = parseWorldTriggerEncounters({
      battle_usable_items: [
        {
          id: "night_ticket",
          itemId: "haunted_after_hours_ticket",
          isTrigger: true,
          dungeonIds: ["amusement_park_001", "amusement_park_001"],
          spawnEnemyIds: ["midnight_ticket_collector_001", "midnight_ticket_collector_001"],
          minWave: 17,
          intervalWaves: 8,
          perWaveChance: 1.2,
          partyTriggerGroup: "amusement_after_hours"
        },
        {
          id: "disabled_ticket",
          itemId: "disabled_ticket",
          isTrigger: true,
          dungeonIds: ["amusement_park_001"],
          spawnEnemyIds: ["should_not_render"],
          disabled: true
        },
        {
          id: "ordinary_battle_item",
          itemId: "ordinary_battle_item",
          dungeonIds: ["amusement_park_001"],
          spawnEnemyIds: ["should_not_render"]
        }
      ]
    }, "amusement_park_001")

    expect(encounters).toEqual([{
      id: "night_ticket",
      itemId: "haunted_after_hours_ticket",
      enemyIds: ["midnight_ticket_collector_001"],
      minWave: 17,
      intervalWaves: 8,
      perWaveChance: 1,
      partyTriggerGroup: "amusement_after_hours"
    }])
  })

  it("projects the parade king's authored base stats to the rebalanced combat HP", () => {
    const paradeKing = enemies.enemies.find((enemy) => enemy.id === "parade_king_boss_001")
    const enemyRow = (paradeKing || {}) as Record<string, unknown>
    const baseStats = Object.fromEntries([
      "strength", "dexterity", "intelligence", "stamina", "luck", "perception",
      "hp", "mp", "speed", "physics_damage", "magical_damage", "physics_defence", "magical_defence"
    ].map((key) => [key, enemyRow[key] ?? 0]))
    const derived = Object.fromEntries(
      rawGlobalConfig.derivedStats.formulas.map((formula) => [
        formula.targetAttr,
        formula.terms.map((term) => `${term.attrKey}*${term.coefficient}`).join("+")
      ])
    )

    expect(projectDerivedFinalStats(baseStats, derived).hp).toBe(42500)
  })
})
