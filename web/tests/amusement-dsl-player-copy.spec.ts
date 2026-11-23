import { describe, expect, it } from "vitest"
import enGameData from "../src/locales/game-data/en-US"
import zhGameData from "../src/locales/game-data/zh-CN"
import enGame from "../src/locales/game/en-US"
import zhGame from "../src/locales/game/zh-CN"
import { buildBattleHookDescriptions } from "../src/services/battle-hook-display"
import { getBuffEffectDescriptions, getLocalizedBuffKindLabel, getLocalizedBuffName } from "../src/services/buff-definition"
import { getEquipmentTemplateDefinition } from "../src/services/item-definition"

type LocaleTree = Record<string, unknown>

function readLocale(tree: LocaleTree, key: string): unknown {
  let cursor: unknown = tree
  for (const part of key.split(".")) {
    if (!cursor || typeof cursor !== "object" || Array.isArray(cursor) || !(part in cursor)) return undefined
    cursor = (cursor as LocaleTree)[part]
  }
  return cursor
}

function createT(...trees: LocaleTree[]) {
  return (key: string, params: Record<string, unknown> = {}): string => {
    const value = trees.map((tree) => readLocale(tree, key)).find((entry) => typeof entry === "string")
    if (typeof value !== "string") return key
    return value.replace(/\{([^}]+)\}/g, (_, name: string) => String(params[name] ?? `{${name}}`))
  }
}

const zhT = createT(zhGame as LocaleTree, zhGameData as LocaleTree)
const enT = createT(enGame as LocaleTree, enGameData as LocaleTree)

function visibleHookText(hooks: ReturnType<typeof buildBattleHookDescriptions>): string {
  return hooks.flatMap((hook) => [
    hook.triggerText,
    ...hook.conditions.map((condition) => condition.text),
    ...hook.effects.flatMap((effect) => effect.kind === "text" ? [effect.text] : [])
  ]).join(" ")
}

describe("amusement DSL player-facing copy", () => {
  it("merges the completed-action cycle and turn filters into one sentence in both locales", () => {
    const hooks = getEquipmentTemplateDefinition("last_light_of_the_parade")?.battleHooks ?? []
    const zh = buildBattleHookDescriptions(hooks, zhT)
    const en = buildBattleHookDescriptions(hooks, enT)

    expect(zh[0]?.conditions.map((condition) => condition.text)).toEqual([
      "每完成 4 次行动（含额外行动）"
    ])
    expect(en[0]?.conditions.map((condition) => condition.text)).toEqual([
      "Every 4 completed actions (including extra actions)"
    ])

    for (const text of [visibleHookText(zh), visibleHookText(en)]) {
      expect(text).not.toMatch(/self_completed_turn_count_mod_eq|turnKinds|includePreparation|includeSuppressedTurns/)
    }
  })

  it("shows the cat sword name instead of its probability source ID", () => {
    const hooks = getEquipmentTemplateDefinition("cat_sword")?.battleHooks ?? []
    const zhText = visibleHookText(buildBattleHookDescriptions(hooks, zhT))
    const enText = visibleHookText(buildBattleHookDescriptions(hooks, enT))

    expect(zhText).toContain(`仅响应「${zhT("item.cat_sword.name")}」产生的判定`)
    expect(enText).toContain(`Only responds to checks from ${enT("item.cat_sword.name")}`)
    expect(zhText).not.toContain("cat_sword")
    expect(enText).not.toContain("cat_sword")
  })

  it("renders lucky blessing as one missing-resource recovery rule without raw formulas or zero rows", () => {
    const zhLines = getBuffEffectDescriptions(zhT, "lucky_blessing").map((line) => line.label)
    const enLines = getBuffEffectDescriptions(enT, "lucky_blessing").map((line) => line.label)

    expect(zhLines).toContain("每次概率成功（每回合最多 3 次）: 回复 5% 已损生命和魔力")
    expect(enLines).toContain("Whenever a probability check succeeds (per-turn limit: 3): Restore 5% of missing HP and MP")
    expect(zhLines).toContain("每次概率判定前: 装备触发与状态施加概率 +3%")
    expect(enLines).toContain("Before each probability check: Equipment proc and status application chance +3%")

    const visibleText = [...zhLines, ...enLines].join(" ")
    expect(visibleText).not.toMatch(/on_probability_success|on_before_probability_check|max_hp|current_hp|max_mp|current_mp|\+0(?:\D|$)/)
    expect(getLocalizedBuffKindLabel(zhT, "buff")).toBe("增益")
    expect(getLocalizedBuffKindLabel(zhT, "debuff")).toBe("减益")
    expect(getLocalizedBuffKindLabel(enT, "buff")).toBe("Buff")
    expect(getLocalizedBuffKindLabel(enT, "debuff")).toBe("Debuff")
  })

  it("localizes lethal checks and summon-template filters without exposing DSL IDs", () => {
    const lethal = buildBattleHookDescriptions([{
      trigger: "on_before_take_damage",
      conditions: [{ type: "incoming_damage_would_be_lethal" }],
      effects: [{ kind: "extra_turn" }]
    }], zhT)
    expect(lethal[0]?.conditions[0]?.text).toBe("这次伤害足以致命")

    const hooks = getEquipmentTemplateDefinition("king_of_amusement_park")?.battleHooks ?? []
    const crownDescriptions = buildBattleHookDescriptions(hooks, zhT)
    const zhText = visibleHookText(crownDescriptions)
    expect(crownDescriptions.find((hook) => hook.key === "king_of_amusement_park_curtain_call")?.triggerText)
      .toBe(`召唤出的${zhT("enemy.candy_clown_001.name")}被击倒时`)
    expect(zhText).not.toMatch(/includeSummonTemplateIds|candy_clown_001|extra_turn|on_unit_death/)
  })

  it("turns buff conditions into lethal, stack, and first-hit player rules", () => {
    const lastLight = getBuffEffectDescriptions(zhT, "last_parade_light").map((line) => line.label).join(" ")
    const stamp = getBuffEffectDescriptions(zhT, "midnight_punched_stamp").map((line) => line.label).join(" ")
    const standInLines = getBuffEffectDescriptions(zhT, "plush_stunt_double_role")
    const standIn = standInLines.map((line) => line.label).join(" ")

    expect(lastLight).toContain("下一次受到致命伤害时: 抵消本次伤害")
    expect(stamp).toContain("达到 3 层时: 无法使用主动技能")
    expect(standIn).toContain("首次受到伤害后")
    expect(standInLines.some((line) => line.buffIds?.includes("amusement_stun"))).toBe(true)
    expect([lastLight, stamp, standIn].join(" ")).not.toMatch(/self_charges_remaining|self_buff_count_gte|consume_charge|消耗充能/)
  })

  it("summarizes the crown summon curve as enhancement milestones", () => {
    const hooks = getEquipmentTemplateDefinition("king_of_amusement_park")?.battleHooks ?? []
    const descriptions = buildBattleHookDescriptions(hooks, zhT)
    const summon = descriptions
      .find((hook) => hook.key === "king_of_amusement_park_opening_parade")
      ?.effects.find((effect) => effect.kind === "summon")

    expect(summon?.kind).toBe("summon")
    if (!summon || summon.kind !== "summon") return
    expect(summon.durationText).toBe("（持续 2 回合；强化 +20 后为 3 回合，强化 +40 后为 4 回合）")
    expect(summon.countText).toBe("数量 1；强化 +20 后为 2，强化 +40 后为 3")
    expect(JSON.stringify(summon)).not.toMatch(/持续公式|向下取整|0\.025|0\.05|装备强化等级/)
  })

  it("makes the empress unique-wearer, every-wave, and no-turn rules explicit", () => {
    const hooks = getEquipmentTemplateDefinition("empress_cloak")?.battleHooks ?? []
    const zh = buildBattleHookDescriptions(hooks, zhT)
    const en = buildBattleHookDescriptions(hooks, enT)
    const zhCommand = zh.find((hook) => hook.key === "empress_cloak_command")
    const enCommand = en.find((hook) => hook.key === "empress_cloak_command")

    expect(zhCommand?.triggerText).toBe("每波开始时")
    expect(enCommand?.triggerText).toBe("At the start of each wave")
    expect(zhCommand?.conditions.map((condition) => condition.text)).toContain(
      `全队恰好 1 个玩家单位装备「${zhT("item.empress_cloak.name")}」（召唤物与幻象不计）`
    )
    expect(enCommand?.conditions.map((condition) => condition.text)).toContain(
      `Exactly 1 player unit(s) in the party equip ${enT("item.empress_cloak.name")} (summons and illusions do not count)`
    )

    const zhEffect = zhT("item.empress_cloak.effect")
    const enEffect = enT("item.empress_cloak.effect")
    expect(zhEffect).toContain("无法获得准备、正常或额外回合")
    expect(zhEffect).toContain("每波次战斗开始前")
    expect(zhEffect).toContain("只刷新持续时间与数值，不叠加")
    expect(enEffect).toContain("cannot receive preparation, ordered, or extra turns")
    expect(enEffect).toContain("Before every wave")

    const zhPassive = getBuffEffectDescriptions(zhT, "empress_cloak_enthroned").map((line) => line.label).join(" ")
    const enPassive = getBuffEffectDescriptions(enT, "empress_cloak_enthroned").map((line) => line.label).join(" ")
    expect(zhPassive).toContain("不进入战斗行动队列，且无法获得准备、正常或额外回合")
    expect(enPassive).toContain("Does not enter the battle action queue and cannot receive preparation, ordered, or extra turns")
    expect([visibleHookText(zh), visibleHookText(en)].join(" ")).not.toMatch(/player_party_equipped_unit_count_eq|turn_ineligible/)
  })

  it("explains the exit ticket's per-wave limit and cooldown debuff", () => {
    const hooks = getEquipmentTemplateDefinition("haunted_house_exit_ticket")?.battleHooks ?? []
    const zhText = visibleHookText(buildBattleHookDescriptions(hooks, zhT))
    const enText = visibleHookText(buildBattleHookDescriptions(hooks, enT))

    expect(zhText).toContain("每个敌人刷新波次最多触发 1 次")
    expect(zhText).toContain(zhT("buff.haunted_exit_route_cooldown.name"))
    expect(enText).toContain("Maximum triggers per enemy-refresh wave: 1")
    expect(enText).toContain(enT("buff.haunted_exit_route_cooldown.name"))

    const zhEffect = zhT("item.haunted_house_exit_ticket.effect")
    const enEffect = enT("item.haunted_house_exit_ticket.effect")
    expect(zhEffect).toContain("每个敌人刷新波次最多触发1次")
    expect(zhEffect).toContain("3个自身回合")
    expect(zhEffect).toContain("本次额外回合计入")
    expect(enEffect).toContain("once per enemy-refresh wave")
    expect(enEffect).toContain("3 of the wearer's turns")
    expect(enEffect).toContain("including the granted extra turn")
    expect([zhText, enText].join(" ")).not.toMatch(/maxTriggersPerWave|self_buff_count_eq|haunted_exit_route_cooldown/)
  })

  it("shows the applause crown cooldown instead of a per-battle cap", () => {
    const hooks = getEquipmentTemplateDefinition("full_house_applause_crown")?.battleHooks ?? []
    const zhEncore = buildBattleHookDescriptions(hooks, zhT)
      .find((hook) => hook.key === "full_house_applause_crown_encore")
    const enEncore = buildBattleHookDescriptions(hooks, enT)
      .find((hook) => hook.key === "full_house_applause_crown_encore")
    const zhConditionText = zhEncore?.conditions.map((condition) => condition.text).join(" ") ?? ""
    const enConditionText = enEncore?.conditions.map((condition) => condition.text).join(" ") ?? ""

    expect(zhConditionText).toContain(zhT("buff.full_house_applause_cooldown.name"))
    expect(enConditionText).toContain(enT("buff.full_house_applause_cooldown.name"))
    expect(zhEncore?.conditions.map((condition) => condition.text)).toContain("每回合最多触发 1 次")
    expect(enEncore?.conditions.map((condition) => condition.text)).toContain("Per-turn trigger limit: 1")
    expect(zhConditionText).not.toContain("每场战斗最多触发")
    expect(enConditionText).not.toContain("Per-battle trigger limit")

    const cooldown = zhEncore?.effects.find((effect) => effect.kind === "add_buff" && effect.buffId === "full_house_applause_cooldown")
    expect(cooldown && cooldown.kind === "add_buff" ? cooldown.turns : null).toBe(3)
    const zhEffect = zhT("item.full_house_applause_crown.effect")
    const enEffect = enT("item.full_house_applause_crown.effect")
    expect(zhEffect).toContain("持续3回合")
    expect(zhEffect).toContain("本次额外回合计入")
    expect(enEffect).toContain("for 3 turns")
    expect(enEffect).toContain("granted extra turn counts")
    expect(zhEffect).not.toContain("每场最多3次")
    expect(enEffect).not.toContain("3 times per battle")
    expect(JSON.stringify([zhEncore, enEncore])).not.toMatch(/self_buff_count_eq|maxTriggersPerBattle/)
  })

  it("states that only a summoned clown being defeated grants the encore", () => {
    const zhEffect = zhT("item.king_of_amusement_park.effect")
    const enEffect = enT("item.king_of_amusement_park.effect")
    expect(zhEffect).toContain("召唤出的糖果小丑被击倒时")
    expect(zhEffect).not.toMatch(/任意糖果小丑|退场/)
    expect(enEffect).toContain("When a summoned Candy Clown is defeated")
    expect(enEffect).not.toMatch(/any Candy Clown|leaves the stage/)
  })

  it("explains the encore mask cost, enhancement power, and hard cooldown without DSL fields", () => {
    const hooks = getEquipmentTemplateDefinition("encore_echo_mask")?.battleHooks ?? []
    const zh = buildBattleHookDescriptions(hooks, zhT, { equipmentEnhanceLevel: 40 })[0]
    const en = buildBattleHookDescriptions(hooks, enT, { equipmentEnhanceLevel: 40 })[0]

    expect(zh?.conditions.map((condition) => condition.text)).toContain(
      "触发需支付最大生命的 10% 与最大魔法的 15%；任一不足则不触发，且生命代价不会致死"
    )
    expect(zh?.conditions.map((condition) => condition.text)).toContain(
      "触发后，需完成 2 个正常或额外行动才可再次触发；准备回合不计"
    )
    const zhMirror = zh?.effects.find((effect) => effect.kind === "mirror_cast_skill")
    const enMirror = en?.effects.find((effect) => effect.kind === "mirror_cast_skill")
    expect(zhMirror && zhMirror.kind === "mirror_cast_skill" ? zhMirror.descriptionText : "").toContain("50%")
    expect(enMirror && enMirror.kind === "mirror_cast_skill" ? enMirror.descriptionText : "").toContain("50%")
    expect(JSON.stringify([zh, en])).not.toMatch(/activationCost|maxHpRatio|maxMpRatio|cooldownCompletedTurns|amountRatioPerEnhanceLevel/)
  })

  it("keeps the dark cat sword target and no-chain copy concise", () => {
    const hooks = getEquipmentTemplateDefinition("dark_cat_sword")?.battleHooks ?? []
    const text = visibleHookText(buildBattleHookDescriptions(hooks, zhT))

    expect(text).toContain("前排 / 中排 / 后排所有单位")
    expect(text).toContain("该伤害不会再次触发概率效果")
    expect(text).not.toContain("所有全部")
    expect(text).not.toContain("概率钩子")
  })

  it("uses locale punctuation in English hook copy", () => {
    const hooks = getEquipmentTemplateDefinition("cat_sword")?.battleHooks ?? []
    const text = visibleHookText(buildBattleHookDescriptions(hooks, enT))
    expect(text).not.toMatch(/[，；]/)
    expect(text).not.toMatch(/success\(es\)|time\(s\)/)
  })

  it("uses a player-facing fallback until a newly added buff receives localized copy", () => {
    const localized = getLocalizedBuffName(zhT, "jackpot_payout_ticket", zhT("game.buffNameFallback"))
    expect(localized).not.toBe("jackpot_payout_ticket")
    expect(localized.trim()).not.toBe("")
  })
})
