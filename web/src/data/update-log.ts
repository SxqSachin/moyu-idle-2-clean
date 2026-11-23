export type UpdateLogLocaleText = {
  "zh-CN": string[]
  "en-US": string[]
}

// Inline menu link format: [visible text](menu:key). Unknown keys remain plain text.
export const updateLogMenuRoutes = {
  arena: "/game/arena",
  combatTrial: "/game/combat-trial",
  fishing: "/game/fishing",
  pond: "/game/pond",
} as const

export type UpdateLogMenuKey = keyof typeof updateLogMenuRoutes

export type UpdateLogEntry = {
  id: string
  date: string
  version?: string
  items: UpdateLogLocaleText
}

const DATE_PATTERN = /^\d{4}-\d{2}-\d{2}$/

function normalizeTextList(raw: string[]): string[] {
  return raw.map((line) => String(line || "").trim()).filter((line) => line !== "")
}

function assertValidEntries(entries: UpdateLogEntry[]) {
  const idSet = new Set<string>()
  for (const entry of entries) {
    if (!entry.id || !entry.id.trim()) {
      throw new Error("update log entry id is required")
    }
    if (idSet.has(entry.id)) {
      throw new Error(`duplicate update log entry id: ${entry.id}`)
    }
    idSet.add(entry.id)
    if (!DATE_PATTERN.test(entry.date)) {
      throw new Error(`invalid update log date format: ${entry.id}`)
    }
    entry.items["zh-CN"] = normalizeTextList(entry.items["zh-CN"])
    entry.items["en-US"] = normalizeTextList(entry.items["en-US"])
    if (entry.items["zh-CN"].length <= 0 || entry.items["en-US"].length <= 0) {
      throw new Error(`update log entry items must be non-empty for both locales: ${entry.id}`)
    }
  }
}

function sortNewestFirst(entries: UpdateLogEntry[]): UpdateLogEntry[] {
  return [...entries].sort((a, b) => {
    if (a.date === b.date) return b.id.localeCompare(a.id)
    return b.date.localeCompare(a.date)
  })
}

const sourceEntries: UpdateLogEntry[] = [
  {
    id: "2026-09-07-fish-filters",
    date: "2026-09-07",
    items: {
      "zh-CN": [
        "[池塘](menu:pond)放鱼列表不再遗漏库存较早的鱼只，支持分页查看并保留跨页选择。",
        "[渔获](menu:fishing)与池塘选鱼统一支持鱼名、鱼种、完整稀有度、健康状态和变异筛选，可查找闪光、棱彩、彩虹及有无变异的鱼只。"
      ],
      "en-US": [
        "The [pond](menu:pond) stocking list now includes older inventory fish, with paged browsing that preserves selections across pages.",
        "[Catches](menu:fishing) and pond fish selection now share name, species, full rarity, health and mutation filters, including Shimmering, Prismatic, Rainbow, and fish with or without mutations."
      ]
    }
  },
  {
    id: "2026-09-07-equipment-skills",
    date: "2026-09-07",
    items: {
      "zh-CN": [
        "新增饰品「统御印记」与「百战徽记」，分别获得可指定敌方技能的「统御」和可组合物理技能的「连武」；强化至 +10、+20 时增加可选技能数量。",
        "装备技能可在技能页的高级设置中选择技能、调整连武顺序与施放规则，并随战斗套装保存。",
        "统御每次成功拦截均消耗最大生命与魔法的15%，资源不足不触发；战斗记录会显示消耗与被打断的技能。",
        "两件饰品拥有专属图标：百战徽记以裂兆印记、狼王獠牙等战利品制作，统御印记以凝愿香核、深渊核心等黑暗材料制作。"
      ],
      "en-US": [
        "Added Domination Seal and Hundred-Battle Badge, granting configurable Domination and Martial Chain. Enhance to +10 and +20 to select more skills.",
        "Configure equipment skills, Martial Chain order and cast rules in advanced skill settings, and save them with battle loadouts.",
        "Each successful Domination interruption costs 15% of maximum HP and MP and cannot trigger without sufficient resources. Battle logs show both costs and the interrupted skill.",
        "Both accessories now have dedicated icons. Craft the badge with an assassination core, a wolf king fang and other trophies; craft the seal with a congealed vow ember, abyssal cores and other dark materials."
      ]
    }
  },
  {
    id: "2026-09-06",
    date: "2026-09-06",
    items: {
      "zh-CN": [
        "修复：猫咪钓鱼等级达标后，为其保存鱼竿、鱼线轮的钓具方案不再被玩家自身的钓鱼等级阻挡；实际使用时仍需当前钓手满足装备要求。",
        "调整：可使用物品的最大叠加时间/次数调整",
      ],
      "en-US": [
        "修复：猫咪钓鱼等级达标后，为其保存鱼竿、鱼线轮的钓具方案不再被玩家自身的钓鱼等级阻挡；实际使用时仍需当前钓手满足装备要求。",
        "调整：可使用物品的最大叠加时间/次数调整",
      ]
    }
  },
  {
    id: "2026-09-05",
    date: "2026-09-05",
    items: {
      "zh-CN": [
        "修复：部分情况下专精无法升级问题。",
        "优化：仓库物品中的附魔物品以及包含强化等级的物品现在可以进行折价回收。",
      ],
      "en-US": [
        "Fix: Overlapping refreshes could show incorrect specialization state when allocating or resetting points. Successful changes are no longer reported as failed if the following refresh fails.",
      ]
    }
  },
  {
    id: "2026-09-04",
    date: "2026-09-04",
    items: {
      "zh-CN": [
        "修复：长时间打开游戏或专精状态发生变化后，可能一直无法分配专精的问题。",
      ],
      "en-US": [
        "Fix: Specializations could remain unallocatable after a long-running game session or a specialization state change.",
      ]
    }
  },
  {
    id: "2026-09-03.b",
    date: "2026-09-03",
    items: {
      "zh-CN": [
        "修复：渔获目前可以被拍卖的问题",
        "修复：部分灌注获得的词条显示为英文问题",
      ],
      "en-US": [
        "Fix: Fish caught in [Fishing](menu:fishing) can now be sold at the Auction House.",
      ]
    }
  },
  {
    id: "2026-09-03",
    date: "2026-09-03",
    items: {
      "zh-CN": [
        "新增：现在聊天中可以发送库存物品",
        "新增：现在可以查看别人的池塘",
        "修复：进入钓鱼或池塘页面时可能会退出登录的问题",
        "修复：专精“魔武转换”无法自动识别物理攻击与魔法攻击最高值并替换的问题",
      ],
      "en-US": [
        "Fix: Entering the Fishing or Pond page could sometimes log you out.",
        "Fix: The specialization 'Magic-Weapon Conversion' could not automatically identify the highest physical and magical attack values and replace them.",
      ]
    }
  },
  {
    id: "2026-09-02.d",
    date: "2026-09-02",
    items: {
      "zh-CN": [
        "调整：[钓鱼](menu:fishing) 顶部现在可以切换当前钓手，水域会按玩家或猫咪各自的钓鱼等级解锁。玩家会显示为“角色名 - 我”，规划钓行时不再重复选择角色。",
      ],
      "en-US": [
        "Adjustment: The [Fishing](menu:fishing) header now switches the current angler, and waters unlock according to that player or kitty's own Fishing level. The player appears as “character name - Me”, and trip planning reuses the header selection instead of asking for the angler again.",
      ]
    }
  },
  {
    id: "2026-09-02.c",
    date: "2026-09-02",
    items: {
      "zh-CN": [
        "调整：[池塘](menu:pond) 的建造与升级降低耗时。",
        "调整：调整了养殖鱼出售时的价格折算。",
        "调整：item:full_house_applause_crown 移除了每场战斗最多返场3次的限制。返场后会获得不可驱散、持续3回合的“余音未歇”；冷却期间仍可积累喝彩，但不能再次返场，本次额外回合计入冷却。",
      ],
      "en-US": [
        "Adjustment: [Pond](menu:pond) construction and upgrades now take less time.",
        "Adjustment: Adjusted the sale-price scaling for cultured fish.",
        "Adjustment: item:full_house_applause_crown no longer has a three-encores-per-battle limit. Triggering an encore now grants the undispellable 3-turn Lingering Ovation cooldown; Cheers can still accumulate while it is active, but another encore cannot trigger, and the granted extra turn counts toward the cooldown.",
      ]
    }
  },
  {
    id: "2026-09-02.b",
    date: "2026-09-02",
    items: {
      "zh-CN": [
        "修复：修复了 [钓鱼](menu:fishing) 的一些已知问题",
      ],
      "en-US": [
        "Fix: Fixed some known issues with [Fishing](menu:fishing)",
      ]
    }
  },
  {
    id: "2026-09-02.a",
    date: "2026-09-02",
    items: {
      "zh-CN": [
        "新增：[池塘](menu:pond) ",
      ],
      "en-US": [
        "New: [Pond](menu:pond) ",
      ]
    }
  },
  {
    id: "2026-09-01.b",
    date: "2026-09-01",
    items: {
      "zh-CN": [
        "修复：[钓鱼](menu:fishing) 各钓具数据未体现问题",
        "修复：[钓鱼](menu:fishing) 猫咪助手无法进行钓鱼问题",
      ],
      "en-US": [
        "Fix: Fishing tackle data not reflected in [Fishing](menu:fishing)",
        "Fix: Cat assistant unable to fish in [Fishing](menu:fishing)",
      ]
    }
  },
  {
    id: "2026-09-01",
    date: "2026-09-01",
    items: {
      "zh-CN": [
        "新增：[猫猫试炼](menu:combatTrial)",
        "新增：[钓鱼](menu:fishing)",
        "优化：优化了战斗性能",
      ],
      "en-US": [
        "New: [Cat Trial](menu:combatTrial)",
        "New: [Fishing](menu:fishing)",
        "Optimization: Optimized combat performance",
      ]
    }
  },
  {
    id: "2026-08-30",
    date: "2026-08-30",
    items: {
      "zh-CN": [
        "新增：[猫村联赛](menu:arena)",
        "调整：降低了部分副本中单位的基础属性",
        "修复：装备界面中，更新装备后装备槽位不会及时更新问题",
      ],
      "en-US": [
        "New: [Cat Village League](menu:arena)",
        "调整：降低了部分副本中单位的基础属性",
        "修复：装备界面中，更新装备后装备槽位不会及时更新问题",
      ]
    }
  },
  {
    id: "2026-08-29",
    date: "2026-08-29",
    items: {
      "zh-CN": [
        "新增：文字战斗日志",
        "修复：装备的“战斗开始时”效果会在每个敌人刷新波次开始时判定；移除了 item:rock_golem_silkwood_staff、item:berry_birds_beloved_staff 与 item:king_of_amusement_park 错误的整场一次上限。",
      ],
      "en-US": [
        "新增：文字战斗日志",
        "Fix: Equipment effects triggered at battle start are checked at the start of every enemy-refresh wave. Removed the incorrect once-per-battle limit from item:rock_golem_silkwood_staff, item:berry_birds_beloved_staff, and item:king_of_amusement_park.",
      ]
    }
  },
  {
    id: "2026-08-27",
    date: "2026-08-27",
    items: {
      "zh-CN": [
        "新增：「游乐园」",
        "新增：“角色”->“专精”",
        "调整：炼金稳定提供的可投入材料上限调整",
      ],
      "en-US": [
        "新增：「游乐园」",
        "新增：“角色”->“专精”",
        "调整：炼金稳定提供的可投入材料上限调整",
      ]
    }
  },
  {
    id: "2026-08-15",
    date: "2026-08-15",
    items: {
      "zh-CN": [
        "新增：「流星回廊」「禁魔图书馆」",
        "新增：item:timekeeper_eight_tick_wheel item:forbidden_binding_bracer item:hushed_lunar_veil_pointed_hat item:five_tick_starfall_lunar_veil_robe item:stellar_testament_pendant item:five_vow_echo_censer item:holy_cat_revenge_star_staff item:prayer_ash_venom_censer item:cat_ancestor_forbidden_seal_crown item:volatile_spore_fuse_ring",
        "新增：“炼金”->“技能炼成”",
      ],
      "en-US": [
        "New: Meteor Corridor, a main-story dungeon placed after Shadowpaw Hideout. Corridor Timekeeper Cat appears every 12 waves.",
        "New: Ordered player and kitty turns advance Meteorfall in Meteor Corridor. The starburst damages the whole party and briefly removes the Timekeeper's normal damage reduction.",
        "New: Meteor Corridor enemies enemy:astral_guard_001 enemy:astral_mage_001 enemy:starlight_healer_001 enemy:refraction_mirror_doll_001 enemy:meteor_core_porter_001 enemy:deranged_calibrator_001 enemy:corridor_timekeeper_cat_boss_001",
        "New: Meteor Corridor materials and maps item:star_rail_copper_plate item:refraction_lens_shard item:meteor_core_fragment item:condensed_stardust item:timekeeper_core item:map_meteor_corridor_001, plus the Fogbound Meteor Corridor Map.",
        "New: Obtainable Meteor Corridor skills skill:refraction_mark skill:meteor_core_crush skill:timekeeper_stasis",
        "New: Forbidden Library, a main-story dungeon placed after Meteor Corridor. Forbidden Library Keeper Cat appears every 12 waves.",
        "New: After each full ordered party cycle, Forbidden Audit randomly silences one or two players or kitties, or increases all of their non-basic skill cooldowns by 1 turn.",
        "New: Forbidden Library enemies enemy:silence_tome_001 enemy:brass_book_guard_001 enemy:living_bookmark_001 enemy:watcher_owl_001 enemy:binding_librarian_001 enemy:banished_spellcat_001 enemy:forbidden_library_keeper_boss_001",
        "New: Forbidden Library materials and maps item:hushed_ink_crystal item:wandering_page_silk item:blue_night_audit_feather item:spell_seal_lock_plate item:final_volume_key_sigil item:map_forbidden_library_001, plus the Fogbound Forbidden Library Map.",
        "New: Obtainable Forbidden Library skills skill:hushing_incantation_page skill:forbidden_trace skill:cooldown_correction",
        "New: Craftable item:timekeeper_eight_tick_wheel spends MP to reduce the highest remaining cooldown among Arcane, Perception, and Snowpact skills. At +40, it gains an extra check at each wave start.",
        "New: Craftable item:forbidden_binding_bracer transfers 1 turn of cooldown between eligible skills after casting. At +40, the first successful binding each wave no longer increases the cast skill's cooldown.",
        "New: Craftable item:hushed_lunar_veil_pointed_hat deals bonus magical damage based on Perception and enhancement level after applying Silence to another unit.",
        "New: Craftable item:five_tick_starfall_lunar_veil_robe calls down a battlefield-wide starfall after every 5 ordered wearer actions. Enhancement raises enemy damage while reducing allied damage.",
        "Improvement: The battlefield header now shows authoritative Meteorfall, Timekeeper Exposure, and Forbidden Audit progress, including after wave transitions and reconnects.",
        "Balance: Meteor Corridor and Forbidden Library materials use restrained base values. Every enhancement level of the four new items consumes all four regular materials from its dungeon.",
        "New: Skill Forge is now available from Alchemy. Invest 200 to 1,000 matching official skillbooks and use loot or high-grade life outputs to modify targeting, seeking, formulas, statuses, costs, and cooldowns. Blank designs can be built and previewed but cannot produce an item.",
        "New: Skill Forge starts at a 30% success rate and gains 10% per additional 100 source books. Every material batch changes success chance according to its power, while higher costs and cooldowns can trade power for reliability.",
        "New: A successful forge creates an account-bound, non-tradable modified skillbook for the player or one owned kitty. A unit cannot enable both the source skill and one of its forged variants.",
        "Improvement: Before submission, Skill Forge shows the final skill, original and modified values, budget, gold, material usage, and final success rate. A failed forge returns half of each input under the forge rules.",
        "Balance: Stackable mythic-material parameters now require 15 items per batch, while legendary materials keep their original batch sizes. One-time mechanic changes still cost five batches, and per-batch effect, budget, and success impact remain unchanged.",
        "Fix: Some skill cooldown adjustment effects displayed an untranslated internal identifier in their details.",
      ]
    }
  },
  {
    id: "2026-06-30",
    date: "2026-06-30",
    items: {
      "zh-CN": [
        "修复：离开游戏或切到后台后再回来时，动作队列可能卡在 100% 不继续推进的问题",
        "修复：部分词条匹配的物品无法被回收问题",
        "修复：部分物品因基础价值为1导致无法被回收问题",
        "修复：item:cute_shoes 装备部位错误问题",
        "优化：优化了库存物品选择与库存界面",
        "优化：优化了库存批量回收与收纳操作",
        "优化：提高了批量回收、批量收纳功能，单次可操作的物品数量上限",
      ],
      "en-US": [
        "Fix: Action queues could get stuck at 100% after leaving the game or returning from the background.",
        "Fix: Some items with a base value of 1 could not be recycled.",
      ]
    }
  },
  {
    id: "2026-06-29-b",
    date: "2026-06-29",
    items: {
      "zh-CN": [
        "优化：为物品补充了更多的“可产出于”的来源信息",
        "优化：猫猫村委托在进行“换一个”后，按钮消失导致误触问题",
        "优化：优化了部分功能的教程",
        "修复：猫猫村委托只会刷新战斗委托的问题",
      ],
      "en-US": [
        "Fix: Cat Village battle commissions no longer move into the life commission list after being refreshed.",
        "Fix: Cats now receive full battle attribute EXP when fighting, without reducing the player's battle EXP.",
        "Improvement: Bringing a cat into early dungeons now reliably increases enemy count, making cat battle rewards clearer.",
        "Note: Battle loot is still settled per player; cats do not duplicate loot rolls.",
        "Improvement: Action queue work loadout auto-switching now skips reapplying a loadout that already matches the current equipment.",
        "Improvement: Affix detaching with a hammer now shows an in-page warning when the original equipment is damaged, without a misleading failure message.",
        "Improvement: Affix detail sections are cleaner and easier to distinguish when multiple affixes are shown together.",
        "New: Warehouse Stash lets you batch-stash items you do not want to see every day, keeping the main inventory cleaner.",
      ]
    }
  },
  {
    id: "2026-06-29",
    date: "2026-06-29",
    items: {
      "zh-CN": [
        "修复：猫猫村战斗委托刷新后可能错误进入生活委托列表的问题",
        "修复：猫咪参战现在会获得完整战斗属性经验，不再分摊并扣减玩家战斗经验",
        "修复：现在携带猫咪参战可以正确增加敌人数量",
        "修复：离线队列处理逻辑优化",
        "修复：存量的历史传闻无法被放弃问题",
        "优化：行动队列自动切换生活套装时，若目标套装已与当前装备一致，将不再重复应用或弹出切换提示",
        "优化：使用item:craftsmans_hammer拆卸附魔时，如果损坏了装备，现在会给出明确提示",
        "优化：词条详情中多词条的正负效果展示更清晰简洁，减少分割线造成的误读",
        "其他：现在猫咪参战时，可以获得完整的战斗经验与技能经验",
        "其他：仓库新增“收纳箱”，可将暂时不想看的物品批量收纳，让日常库存列表更简洁。",
      ],
      "en-US": [
        "Fix: Cat Village battle commissions no longer move into the life commission list after being refreshed.",
        "Fix: Cats now receive full battle attribute EXP when fighting, without reducing the player's battle EXP.",
        "Improvement: Bringing a cat into early dungeons now reliably increases enemy count, making cat battle rewards clearer.",
        "Note: Battle loot is still settled per player; cats do not duplicate loot rolls.",
        "Improvement: Action queue work loadout auto-switching now skips reapplying a loadout that already matches the current equipment.",
        "Improvement: Affix detaching with a hammer now shows an in-page warning when the original equipment is damaged, without a misleading failure message.",
        "Improvement: Affix detail sections are cleaner and easier to distinguish when multiple affixes are shown together.",
        "New: Warehouse Stash lets you batch-stash items you do not want to see every day, keeping the main inventory cleaner.",
      ]
    }
  },
  {
    id: "2026-06-28",
    date: "2026-06-28",
    items: {
      "zh-CN": [
        "新增：item:markseeker_quiver item:frostbreaker_charm item:fusion_heart item:formation_commander_baton",
        "调整：现在传闻类的委托的第一阶段可以放弃",
      ],
      "en-US": [
        "New: item:markseeker_quiver item:frostbreaker_charm",
        "Adjust: Rumor commission tasks can now be abandoned during their first stage.",
      ]
    }
  },
  {
    id: "2026-06-27",
    date: "2026-06-27",
    items: {
      "zh-CN": [
        "新增：item:mirror_paw_guard_shield item:shadow_paw_void_codex item:mirror_paw_pack_bell item:shadow_paw_threadwheel item:mirror_paw_echo_charm item:shadow_paw_veilstring_charm item:shadow_paw_pattern_gloves item:mirror_paw_calibration_lens",
        "新增：item:battle_morning_egg item:battle_sweet_milk item:battle_silverleaf_custard item:battle_silkweed_custard item:battle_sunlit_omelette item:battle_moon_pudding item:battle_mending_broth item:battle_mana_tea item:battle_moondew_stew item:battle_training_ration item:battle_lucky_feast",
        "新增：item:battle_furfire_broth item:battle_mosslight_breach_soup item:battle_snowseal_swift_soup item:battle_shadowmoon_pudding item:battle_holy_incense_jelly item:battle_silent_whisker_plate",
        "新增：action:cook_battle_furfire_broth action:cook_battle_mosslight_breach_soup action:cook_battle_snowseal_swift_soup action:cook_battle_shadowmoon_pudding action:cook_battle_holy_incense_jelly action:cook_battle_silent_whisker_plate",
        "新增：item:battle_mirror_paw_shield_vial item:battle_claw_venom_flask",
        // "新增：“炼金”->“融合”",
        "优化：优化了词条显示效果",
        "优化：敌人词典改为按副本分类展示",
        "优化：优化了技能/战斗物品释放规则编辑器",
        "修复：item:cute_cape 穿戴位置错误问题",
        "修复：查看副本中敌人的信息时，敌方单位详情中的初始Buff现在可以正确显示",
        "修复：装备物品时，可装备物品界面物品错位问题",
        "修复：炼金相关物品选择弹窗在移动端会被左侧图标菜单挤压或裁切的问题。",
        "修复：移动端教程中的“上一步/下一步/完成”按钮可能无法点击的问题。",
        "优化：游戏引导列表加载教程脚本时不再误显示“暂无可用引导”。",
        "修复：补全了一些图标",
      ],
      "en-US": [
        "New: item:mirror_paw_guard_shield item:shadow_paw_void_codex item:mirror_paw_pack_bell item:shadow_paw_threadwheel item:mirror_paw_echo_charm item:shadow_paw_veilstring_charm item:shadow_paw_pattern_gloves item:mirror_paw_calibration_lens",
        "New: item:battle_morning_egg item:battle_sweet_milk item:battle_silverleaf_custard item:battle_silkweed_custard item:battle_sunlit_omelette item:battle_moon_pudding item:battle_mending_broth item:battle_mana_tea item:battle_moondew_stew item:battle_training_ration item:battle_lucky_feast",
        "New: item:battle_furfire_broth item:battle_mosslight_breach_soup item:battle_snowseal_swift_soup item:battle_shadowmoon_pudding item:battle_holy_incense_jelly item:battle_silent_whisker_plate",
        "New: action:cook_battle_furfire_broth action:cook_battle_mosslight_breach_soup action:cook_battle_snowseal_swift_soup action:cook_battle_shadowmoon_pudding action:cook_battle_holy_incense_jelly action:cook_battle_silent_whisker_plate",
        "New: item:battle_mirror_paw_shield_vial item:battle_claw_venom_flask",
        "Improvement: Affix display is clearer.",
        "Improvement: Enemy Dictionary is now grouped by dungeon.",
        "Improvement: Skill and battle item release rules now have a clearer builder with ally/enemy HP, state, and cooldown conditions.",
        "Fix: Alchemy item selection dialogs are no longer squeezed or clipped by the mobile icon rail.",
        "Fix: Tutorial Previous, Next, and Done buttons are now touchable on mobile.",
        "Improvement: The guide list now shows a loading state while tutorial scripts are loading instead of an empty state.",
      ]
    }
  },
  {
    id: "2026-06-25",
    date: "2026-06-25",
    items: {
      "zh-CN": [
        "修复：“转换”功能中，部分素材加入队列后会立刻失败问题",
      ],
      "en-US": [
        "New: Alchemy conversion, infusion, and transmutation.",
        "Fix: Equipping the Coarse Linen Hood no longer causes it to disappear from both the equipment view and the inventory.",
        "Fix: Battle HP/MP bars appeared blank on some older browsers (older WeChat/Android WebView, Safari < 16.4).",
        "Improvement: Queue items now have Move to Top and Move to Bottom buttons.",
      ]
    }
  },
  {
    id: "2026-06-24-b",
    date: "2026-06-24",
    items: {
      "zh-CN": [
        "修复：灌注所获得的自定义附魔没有显示可装备数量限制的问题",
        "修复：部分罕见素材单次可灌注的数量上限为50的问题",
        "修复：炼金等级不足时依旧可以进行灌注的问题",
        "修复：部灌注功能会允许填入的素材会不满足所需数量整数倍数数量的素材，导致灌注操作失败的问题",
        "调整：调整了灌注功能中部分素材的收益",
        "调整：调整了灌注功能中部分介质的属性",
      ],
      "en-US": [
        "New: Alchemy conversion, infusion, and transmutation.",
        "Fix: Equipping the Coarse Linen Hood no longer causes it to disappear from both the equipment view and the inventory.",
        "Fix: Battle HP/MP bars appeared blank on some older browsers (older WeChat/Android WebView, Safari < 16.4).",
        "Improvement: Queue items now have Move to Top and Move to Bottom buttons.",
      ]
    }
  },
  {
    id: "2026-06-24",
    date: "2026-06-24",
    items: {
      "zh-CN": [
        "新增：attr:alchemy_level",
        "新增：炼金转换、灌注、点金",
        "优化：队列新增「置顶」和「置底」按钮，可在待执行队列中快速进行行动管理。",
        "修复：穿上「粗麻兜帽」后，道具从装备栏和背包中同时消失的问题。",
        "修复：战斗血条/魔法条可能显示为空白的问题，部分文字无法显示问题。",
      ],
      "en-US": [
        "New: Alchemy conversion, infusion, and transmutation.",
        "Fix: Equipping the Coarse Linen Hood no longer causes it to disappear from both the equipment view and the inventory.",
        "Fix: Battle HP/MP bars appeared blank on some older browsers (older WeChat/Android WebView, Safari < 16.4).",
        "Improvement: Queue items now have Move to Top and Move to Bottom buttons.",
      ]
    }
  },
  {

    id: "2026-06-22",
    date: "2026-06-22",
    items: {
      "zh-CN": [
        "优化：套装系统支持“强化”类动作自动切换。",
        "优化：若自动切换套装时因套装不存在、装备未持有或不在背包中等原因无法完成切换，现在会弹出提示告知具体原因。",
        "修复：仓库“变动记录”中，布告栏的物品消耗与奖励产出无法按分类筛选",
        "调整：下调战斗等级升级所需经验，已经获得的战斗经验会继续保留，并按新曲线重新计算当前等级。",
      ],
      "en-US": [
        "New: Equipment Enhancement now supports auto-switching work loadouts.",
        "New: If a loadout auto-switch fails, you will now see a notification explaining the exact reason.",
        "Fix: Cat Village Commission costs and rewards were not categorized in warehouse change logs.",
        "Balance: Battle level experience requirements have been reduced. Previously earned battle experience is kept and recalculates your current level under the new curve.",
      ]
    }
  },
  {
    id: "2026-06-21",
    date: "2026-06-21",
    items: {
      "zh-CN": [
        "调整：调整了大多数战斗掉落物的价格",
        "调整：“布告牌”中添加了提交战利品的战斗类委托",
        "修复：传闻Boss现在可以被「优先攻击首领」类装备/技能正确识别并优先索敌",
        "修复：对Boss生效的伤害加成（如装备词缀「猎首者」）现在也会对传闻Boss生效",
        "修复：仓库变动记录中所有物品的数量显示为实际值两倍的问题",
        "修复：装备属性详情中词条加成来源显示「来自affix」未翻译的问题，现已更正为「来自词条」",
        "修复：猫咪装备详情中的属性需求现在会按当前猫咪属性显示，不再误用玩家属性",
        "修复：鼠标悬停物品图标查看详情时，将鼠标移至详情面板后面板会立即消失的问题",
        "修复：手机端战斗房间底部按钮有时看不到或点不到的问题，现在准备、离开房间、结束房间等操作会固定在屏幕底部附近，方便在准备阶段操作",
      ],
      "en-US": [
        "Fix: World Boss enemies are now correctly recognized as boss-class by boss-first targeting on equipment and skills.",
        "Fix: Damage bonuses that apply to bosses (e.g. the Boss Stalker equipment affix) now also apply to World Boss enemies.",
        "Fix: Warehouse change log was showing double the actual item quantities for all entries.",
        "Fix: Equipment attribute breakdown was displaying affix contribution source as 'from affix' (untranslated) — now correctly shown in Chinese as '来自词条'.",
        "Fix: Cat equipment requirement details now use the currently selected cat's attributes instead of the player's attributes.",
        "Fix: Item detail popover now stays open when moving the mouse from the item icon to the detail panel, allowing you to scroll through the full content.",
        "Fix: On mobile, battle room action buttons can no longer get trapped below the visible screen. Ready, leave room, and end room now stay near the bottom so the preparation phase remains playable.",
        "New: Equipment Enhancement now supports auto-switching work loadouts. Bind a loadout to the Enhancement category in the Work Action Bindings section, and it will automatically activate whenever you run an enhancement action — same as any other life-skill loadout.",
        "New: If a loadout auto-switch fails (e.g. the loadout no longer exists, an item is not owned, or is not in the bag), you will now see a notification explaining the exact reason instead of a silent fallback.",
        "Fix: Cat Village Commission costs and rewards were not categorized in warehouse change logs — they are now filterable under the \"Cat Village Commission\" tab with proper display names.",
      ],
    },
  },
  {
    id: "2026-06-18",
    date: "2026-06-18",
    items: {
      "zh-CN": [
        "新增：副本迷雾模式",
        "优化：布告牌界面，已满足材料条件的委托现在会优先显示在上方，传闻仍保持最顶部",
        "优化：移动端图标窄栏菜单现在更容易上下滑动，避免滑动时误点到菜单图标",
        "优化：优化了在同一个设备上登陆时会出现邮件验证的情况",
        "优化：优化了部分组件在夜间模式无法看清的问题",
        "优化：市场「我的记录」标签页现将上架记录与求购记录界面优化",
        "修复：上线后行动队列当前动作的进度条有时显示不准确的问题（进度过快卡在100%、或进度过慢/跳位）",
        "修复：仓库中快速划过多个物品时，物品详情浮窗偶尔不会自动消失的问题",
        "修复：动作类见闻在队列与离线结算中，偶尔不会正确累计的问题",
        " - 大批量完成动作时，跨阶段见闻现在会正确继承已累计次数，不会只停在上一阶段目标值",
        " - 队列结算后的见闻进度现在会更及时刷新，减少需要手动刷新页面才能看到进度变化的情况",
        "修复：异常情况下，已经没有正确穿戴的装备仍可能继续提供属性的问题；现在装备加成会严格按照当前实际穿戴状态生效",
        "修复：item:frost_hex_tome 描述与效果不一致问题",
        "调整：调整了副本中单位的各项属性值",
        " - 去除了敌人的基础生命值，敌人的生命值现在直接由耐力属性决定，因此敌人现在的生命值将大幅下降",
        " - 战斗中非玩家的单位，从耐力中获得的自然生命值恢复量降低66%",
        " - 降低了所有敌人提供的经验值",
        "调整：防御计算公式调整:",
        " - 有效防御 = (防御 - 固定穿透) * (1 - 百分比穿透)",
        " - 最终伤害 = 伤害 * 100 / (100 + 有效防御)",
        " - 有效防御 > 0 时为减伤；有效防御 < 0 时为增伤（溢出穿透）",
        " - 防御属性对应的具体减伤百分比会在防御数值后进行显示",
        " - 对物理防御与魔法防御均生效",
      ],
      "en-US": [
        "Fix: Action-based achievements now count more reliably from queued and offline progress.",
        "Fix: Item detail popovers in the warehouse now close reliably after moving the mouse away.",
        " - Large batches now carry accumulated progress across achievement stages instead of stopping at the previous stage target.",
        " - Achievement progress refreshes more promptly after queue settlement, reducing cases where a page refresh was needed.",
        "Fix: Equipment bonuses now strictly follow what is actually equipped, preventing stale headgear bonuses from lingering after abnormal equipment changes.",
        "Improvement: The mobile icon-rail menu is now easier to scroll, with fewer accidental menu taps while swiping.",
        "Fix: Action queue progress bar occasionally showing incorrect progress after logging in (running too fast and sticking at 100%, or running too slow with a jump).",
        "Balance: defense formula updated to flat-pen-first then percent-pen:",
        " - Effective defense = (defense - flatPen) * (1 - percentPen)",
        " - Final damage = damage * K / (K + effectiveDefense), K defaults to 100",
        " - Effective defense > 0 mitigates damage; < 0 amplifies damage (overflow penetration)",
        " - Effective defense floored at -K/2, giving a max 2x amplification (symmetric: K defense = 0.5x)",
        "Improvement: On the bulletin board, commissions you can already fulfill (all required items in inventory) are now sorted to the top. Rumors remain pinned above all.",
        "Improvement: Logging in from a familiar device no longer triggers email verification every time. After verifying once on a new device, that device is remembered for 30 days.",
        "Improvement: Market history now splits listings and buy orders into separate sub-tabs for easier browsing.",
        "Improvement: Listing and buy order history now supports pagination (15 entries per page), replacing the single long scroll.",
        "New: Each history entry now has a \"View in Market\" button that jumps directly to that item's trading page.",
      ],
    }
  },
  {
    id: "2026-06-15",
    date: "2026-06-15",
    items: {
      "zh-CN": [
        "调整：统一了各副本的人数限制",
        "修复：见闻中“猫影深渊”Boss类敌人的击杀数量异常问题",
      ],
      "en-US": [
        "todo"
      ],
    }
  },
  {
    id: "2026-06-12",
    date: "2026-06-12",
    items: {
      "zh-CN": [
        "新增：“见闻”",
        "新增：两个传闻以及对应的技能书及道具",
        "修复：item:crystal_fang 的击杀叠层效果现在会正确累计",
        "调整：item:summon_sync_trinket_001",
        "修复：市场页面进入交易面板后，点击「商品目录」选项卡无法返回的问题",
        "修复：技能设置中\"普通攻击\"的释放顺序在进入副本后总是会被重置到末尾的问题，如果仍存在异常请重新设置一次优先级",
        "修复：离线收益弹窗中，猫咪助手获得的物品被重复计算导致数量显示不一致的问题",
        "其他：现在登录状态可以在多个设备中保存",
        "其他：明确了大多数报错信息的具体内容，方便玩家反馈问题时提供更准确的信息",
      ],
      "en-US": [
        "Fix: item:crystal_fang kill stacks now correctly accumulate `stack_count` buffs, allowing its awakening and item:crystal_fang_blood_rage bonus to trigger after 5 stacks.",
        "Fix: Basic attack priority order in skill settings is now preserved when entering dungeons.",
        "Fix: Offline rewards popup no longer double-counts kitty actor rewards in the top-level item rewards section, preventing mismatched quantities.",
      ],
    }
  },
  {
    id: "2026-06-11",
    date: "2026-06-11",
    items: {
      "zh-CN": [
        "修复：仓库物品错位问题",
      ],
      "en-US": [
        "Fix: Resolved an issue where the healing effect from item:moss_sleep_ring was not properly triggering the item:snow_oath_codex effect.",
      ],
    }
  },
  {
    id: "2026-06-09",
    date: "2026-06-09",
    items: {
      "zh-CN": [
        "新增：库存批量回收，可以通过点击库存中的设置按钮进入批量回收模式",
        "新增：item:summon_sync_trinket_001 item:folding_boundary_orb",
        "调整：调整了传闻类物品对应敌人的触发的基础波次与概率",
        "调整：现在大多数召唤物都可以跨波次存在，在战斗开始时持续时间会额外-1回合。",
        "调整：召唤单位现在不会获得备战回合。",
        "修复：item:moss_sleep_ring 治疗不触发 item:snow_oath_codex",
        "修复：item:crystal_fang 无法交易问题",
        "修复：套装功能中的装备或饰品无法卸下的问题",
        "修复：猫咪助手的装备不会收到附魔数量限制的问题",
      ],
      "en-US": [
        "Fix: Resolved an issue where the healing effect from item:moss_sleep_ring was not properly triggering the item:snow_oath_codex effect.",
      ],
    }
  },
  {
    id: "2026-06-07",
    date: "2026-06-07",
    items: {
      "zh-CN": [
        "新增：item:snow_oath_codex item:condensed_light_bow",
        "优化：异地登陆邮件发送逻辑优化",
        "修复：套装中包含饰品时，手动应用与自动应用会失败的问题",
        "修复：库存变动时（获得物品、卸下装备）有时不会立即更新仓库库存，需要刷新后才会在库存中显示的问题",
        "调整：item:crystal_repeating_crossbow item:holy_cat_sage_charm",
        // "调整：调整了强化成功率曲线，整体降低了达到最终强化等级的概率，提高了高稀有度物品的强化概率",
        "调整：猫咪自身等级现在会提升战斗等级上限（每级+50）",
        "修复：战斗中单位生命值为小数时，生命值显示为0的问题（现在向上取整显示）",
        "修复：skill:hearthside_thaw 现在能正确对血量最低的队友施法，不再跳过需要治疗的目标",
      ],
      "en-US": [
        "Fix: skill:hearthside_thaw now correctly targets the ally with the lowest HP instead of skipping units that need healing",
        "TODO: translate zh-CN update log generated from game-data diff",
      ],
    }
  },
  {
    id: "2026-06-06-b",
    date: "2026-06-06",
    items: {
      "zh-CN": [
        "新增：item:moss_sleep_ring item:floating_silver_charm item:healing_staff",
        "其他：现在战斗房间可以选择是否启用“备战回合”",
      ],
      "en-US": [
        "New：item:moss_sleep_ring item:floating_silver_charm item:healing_staff item:silver_corrode_ring item:moss_sleep_vambrace",
        "Fix: Resolved an issue where the `taunt` flow granted by the kitty trait `taunting_tail` could fail to properly prioritize the taunting kitty for enemy single-target attacks. With this fix, enemies will correctly target the taunting kitty when the trait is active.",
      ],
    }
  },
  {
    id: "2026-06-06",
    date: "2026-06-06",
    items: {
      "zh-CN": [
        "新增：特性词典。",
        "优化：现在技能书使用时可以选择使用次数。",
        "优化：skill:hearthside_thaw 技能现在只会在友方存在非满血单位时才会释放（优先于“技能释放规则”生效）。",
        "优化：已验证邮箱的账号现在默认启用异地登陆验证",
        "修复：“套装”->“战斗套装”配置后无法应用问题",
        "修复：咪助手装备物品后，即使没有上场，也会对玩家角色生效的问题。",
        "修复：猫咪随记中的获取物品的相关事件中物品显示为id问题",
        "修复：现在打开“技能释放条件”弹窗时，不会向其中添加一个默认条件",
      ],
      "en-US": [
        "Fix: Added regression coverage for the `taunt` flow granted by the kitty trait `taunting_tail`, ensuring enemy single-target attacks continue to prioritize the taunting kitty.",
      ],
    }
  },
  {
    id: "2026-06-04",
    date: "2026-06-04",
    items: {
      "zh-CN": [
        "新增：角色页新增“套装”页签，支持保存、编辑、应用战斗套装与生活套装。",
        " - 战斗套装可同时记录战斗装备、技能配置与技能触发规则、战斗物品袋。",
        " - 生活套装支持按动作大类绑定，执行对应动作时会在动作启动前自动切换到对应套装。",
        "修复：战斗房间成员与实际参战单位偶发不同步，导致进入房间后开战看不到自己、显示成别人或缺人、且不结算战斗经验的问题。",
        "调整：猫咪助手现在的基础行动效率为-100%。",
        "调整：现在猫舍可以被升级到2级。",
        // "优化：在创建房间、加入房间、准备、踢人、离队、开战前会自动校验并修复参战单位数据，清理失效单位并补齐缺失的玩家单位。",
      ],
      "en-US": [
        "Fix: Resolved an occasional desync between battle-room members and actual combat units that could cause players to disappear from battle, show the wrong unit list, or miss battle EXP rewards.",
        "Improvement: Room create/join/ready/leave/kick/start now auto-validate and repair combat-unit records by removing stale units and restoring missing player units.",
      ],
    }
  },
  {
    id: "2026-06-01-a",
    date: "2026-06-01",
    items: {
      "zh-CN": [
        "优化：仓库、市场的“默认排序”菜单中新增“二级分类”开关，默认关闭，按分类本地记忆（localStorage）。",
        "作用位置：仓库、市场、词典共用同一套物品浏览分类逻辑。",
        "规则：词典内装备始终使用二级分类展示，不受开关控制。",
      ],
      "en-US": [
        "Improvement: Added a \"Secondary categories\" toggle in the sort menu for Warehouse and Market, with per-category local persistence (localStorage).",
        "Scope: Warehouse, Market, and Dictionary share the same item-browser classification pipeline.",
        "Rule: Dictionary always uses secondary equipment grouping and is not controlled by the toggle.",
      ],
    }
  },
  {
    id: "2026-05-29-b",
    date: "2026-05-29",
    items: {
      "zh-CN": [
        '新增：item:crystal_repeating_crossbow item:base2_dragon_scale_lance item:base4_nightveil_lance item:rock_golem_silkwood_staff item:frostspore_scale_shield',
        '优化：优化了装备的分类展示效果',
        '修复：动作详情面板中，部分情况下“使用具体物品”选择物品后，无法立即执行此动作问题',
        '修复：主页的工作队列中的预计完成时间没有受到效率影响的问题',
        '修复：affix:base4_affix_boss_stalker 品质与数值异常的问题',
        '修复：长矛类武器伤害系数问题',
        '修复：词典中技能等级加成不显示问题',
        '调整：工作队列最大长度调整为25',
        '调整：item:base4_night_fan_blades 降低了伤害系数',
        '调整：item:base3_rime_ledger_scythe',
        '调整：调整了装备类物品的回收判定：',
        ' - 若装备与原物品完全一致，按基础回收系数结算',
        ' - 原物品有词条，回收的物品无词条，按更低的回收系数结算',
        '以下情况不能回收：',
        ' - 有强化的物品',
        ' - 原物品无词条但回收的物品有词条',
        ' - 原物品有词条，回收的物品也有词条，但词条类型与原物品词条不一致。',
      ],
      "en-US": [
        'todo'
      ],
    }
  },
  {
    id: "2026-05-29",
    date: "2026-05-29",
    items: {
      "zh-CN": [
        "新增：attr:perception attr:magic_crit_chance attr:magic_crit_damage attr:crit_damage_resist",
        "新增：item:forbidden_magic_bow item:mystic_mind_orb item:battlemage_perception_blade",
        "新增：item:skillbook_perception_mind_control item:skillbook_perception_disrupt item:skillbook_perception_shockburst",
        '调整：attr:intelligence attr:stamina attr:strength',
        '调整：item:coarse_linen_vambraces item:soft_cotton_vambraces item:silver_vein_silk_vambraces item:wisp_weave_vambraces item:gilded_brocade_vambraces item:lunar_veil_vambraces '
      ],
      "en-US": [
        "TODO: translate zh-CN update log generated from game-data diff",
      ],
    }
  },
  {
    id: "2026-05-27",
    date: "2026-05-27",
    items: {
      "zh-CN": [
        "新增：词典",
        "新增：备战回合",
        "新增：item:basic_hook_claw_001 item:base5_breaker_pendant item:base5_bloodforge_armor item:base1_sprout_guard_charm item:base1_barkguard_ring item:base1_mosswarden_pendant",
        "优化：现在会在物品详情中显示战利品中包含此物品的敌人信息",
        "优化：现在一些单位身上的部分状态会在战斗卡片上显示出来",
        "调整：调整了 affix:sewing_attacking_ops affix:sewing_attacking_targets_opt affix:sewing_skill_ops 伤害构成",
        "调整：现在战斗中释放技能后会提供2点对应的技能经验，在战斗结束时结算",
      ],
      "en-US": [
        "TODO: translate zh-CN update log generated from game-data diff",
      ],
    }
  },
  {
    id: "2026-05-26-c",
    date: "2026-05-26",
    items: {
      "zh-CN": [
        "修复：物品价值异常问题",
      ],
      "en-US": [
        "todo"
      ],
    },
  },
  {
    id: "2026-05-26-b",
    date: "2026-05-26",
    items: {
      "zh-CN": [
        "调整：skill:base4_abyssal_pressure 第 1 个效果的寻敌方式从“随机寻找存活目标”调整为“从前排向后排继续寻找目标”；第 1 个效果的目标范围移除中排、后排目标",
        "调整：skill:base4_nightfall_curse_skill 第 1 个效果的寻敌方式从“随机寻找存活目标”调整为“从前排向后排继续寻找目标”；第 1 个效果的目标范围移除中排、后排目标；默认寻敌方式从“随机寻找存活目标”调整为“从前排向后排继续寻找目标”，目标数量调整至2",
        "调整：skill:base4_shadow_bind_skill 第 1 个效果的寻敌方式从“随机寻找存活目标”调整为“从前排向后排继续寻找目标”；第 1 个效果的目标范围移除中排、后排目标；默认寻敌方式从“随机寻找存活目标”调整为“从前排向后排继续寻找目标”",
        "调整：skill:base4_void_lance 第 1 个效果的寻敌方式从“从后排向前排继续寻找目标”调整为“从前排向后排继续寻找目标”；第 1 个效果的目标范围新增前排目标，移除后排目标；默认寻敌方式从“从后排向前排继续寻找目标”调整为“从前排向后排继续寻找目标”",
      ],
      "en-US": [
        "todo"
      ],
    },
  },
  {
    id: "2026-05-26",
    date: "2026-05-26",
    items: {
      "zh-CN": [
        "优化：优化了库存变动记录分类",
        "优化：现在从物品详情中装备非饰品类物品时，不再需要选择装备槽位",
        "优化：库存中物品数量显示优化",
        "优化：拍卖行金额显示统一为千分位完整数字，不再使用缩写",
        "优化：市场金额显示统一为千分位完整数字，不再使用缩写",
        "优化：公开拍卖新增状态筛选（进行中可参与/已成交/流拍/全部），默认显示进行中且可参与的拍卖",
        "优化：房间内聊天",
        "修复：拍卖报错问题",
        "修复：移动端长按物品、词条时，不显示详情信息或者显示的详情信息不完整问题",
        "调整：为大多数装备添加了强化路径上的里程碑奖励，包含额外的属性提升以及附魔",
        "调整：为 affix:sewing_attacking_targets_opt affix:sewing_attacking_ops affix:sewing_skill_ops 附魔添加了单件装备附魔上限与可佩戴的数量上限",
        "调整：item:base2_spectral_focus_staff 普通攻击的数值公式中魔法伤害系数从 0.82 调整为 1.3；普通攻击的伤害公式中魔法伤害系数从 0.82 调整为 1.3；普通攻击的目标数量上限新增为 3",
        "调整：item:base3_ice_account_abacus 强化加成的魔法增伤从 0.002 调整为 0.001；强化加成的智力从 0.45 调整为 0.6；强化加成的速度从 0.45 调整为 0.5",
        "调整：item:base4_enhancer_shadow_lens 10级强化加成的强化成功率从 0.01 调整为 0.02；20级强化加成的强化成功率从 0.01 调整为 0.02",
        "调整：item:base4_night_fan_blades 普通攻击的目标数量从 4 调整为 2；普通攻击的目标数量上限新增为 4",
        "调整：item:coarse_linen_hood 强化加成的物理增伤从 0.01 调整为 0.005",
        "调整：item:copper_bow 强化加成的物理伤害从 1.15 调整为 1.5",
        "调整：item:copper_crossbow 强化加成的物理伤害从 1.15 调整为 1.5",
        "调整：item:copper_great_shield 普通攻击的目标范围新增中排目标",
        "调整：item:copper_scythe 普通攻击的寻敌方式改为前排没有目标时，会继续向后排寻找",
        "调整：item:copper_spear 普通攻击的寻敌方式改为前排没有目标时，会继续向后排寻找",
        "调整：item:crystal_bow 强化加成的物理伤害从 2 调整为 5",
        "调整：item:crystal_bracers 强化加成的物理伤害从 0.75 调整为 1；强化加成的物理防御从 0.75 调整为 1",
        "调整：item:crystal_crossbow 强化加成的物理伤害从 2 调整为 5",
        "调整：item:crystal_great_shield 普通攻击的目标范围新增中排目标",
        "调整：item:crystal_scythe 普通攻击的寻敌方式改为前排没有目标时，会继续向后排寻找",
        "调整：item:crystal_spear 普通攻击的寻敌方式改为前排没有目标时，会继续向后排寻找",
        "调整：item:gilded_brocade_hood 强化加成的物理增伤从 0.014 调整为 0.009",
        "调整：item:gilded_brocade_pointed_hat 强化加成的智力从 1.8 调整为 2.2",
        "调整：item:gold_bow 强化加成的物理伤害从 1.8 调整为 3",
        "调整：item:gold_bracers 强化加成的物理伤害从 0.65 调整为 0.85；强化加成的物理防御从 0.65 调整为 0.85",
        "调整：item:gold_crossbow 强化加成的物理伤害从 1.8 调整为 3.5",
        "调整：item:gold_great_shield 普通攻击的目标范围新增中排目标",
        "调整：item:gold_scythe 普通攻击的寻敌方式改为前排没有目标时，会继续向后排寻找",
        "调整：item:gold_spear 普通攻击的寻敌方式改为前排没有目标时，会继续向后排寻找",
        "调整：item:iron_bow 强化加成的物理伤害从 1.3 调整为 2",
        "调整：item:iron_bracers 强化加成的物理伤害从 0.55 调整为 0.6；强化加成的物理防御从 0.55 调整为 0.6",
        "调整：item:iron_crossbow 强化加成的物理伤害从 1.3 调整为 2",
        "调整：item:iron_great_shield 普通攻击的目标范围新增中排目标",
        "调整：item:iron_scythe 普通攻击的寻敌方式改为前排没有目标时，会继续向后排寻找",
        "调整：item:iron_spear 普通攻击的寻敌方式改为前排没有目标时，会继续向后排寻找",
        "调整：item:lunar_veil_hood 强化加成的物理增伤从 0.015 调整为 0.01",
        "调整：item:lunar_veil_pointed_hat 强化加成的智力从 2 调整为 2.5",
        "调整：item:sewing_weapon_1 普通攻击的目标数量从 3 调整为 2",
        "调整：item:sewing_weapon_2 普通攻击的目标数量从 3 调整为 2",
        "调整：item:sewing_weapon_3 普通攻击的目标数量从 3 调整为 2",
        "调整：item:silver_bow 强化加成的物理伤害从 1.6 调整为 2.5",
        "调整：item:silver_bracers 强化加成的物理伤害从 0.6 调整为 0.7；强化加成的物理防御从 0.6 调整为 0.7",
        "调整：item:silver_crossbow 强化加成的物理伤害从 1.6 调整为 2.5",
        "调整：item:silver_great_shield 普通攻击的目标范围新增中排目标",
        "调整：item:silver_scythe 普通攻击的寻敌方式改为前排没有目标时，会继续向后排寻找",
        "调整：item:silver_spear 普通攻击的寻敌方式改为前排没有目标时，会继续向后排寻找",
        "调整：item:silver_vein_silk_hood 强化加成的物理增伤从 0.012 调整为 0.01；强化加成的物理增伤从 0.012 调整为 0.007",
        "调整：item:silver_vein_silk_pointed_hat 强化加成的智力从 1.3 调整为 1.6",
        "调整：item:soft_cotton_hood 强化加成的物理增伤从 0.011 调整为 0.01；强化加成的物理增伤从 0.011 调整为 0.006",
        "调整：item:soft_cotton_pointed_hat 强化加成的智力从 1.15 调整为 1.3",
        "调整：item:wisp_weave_hood 强化加成的物理增伤从 0.013 调整为 0.008",
        "调整：item:wisp_weave_pointed_hat 强化加成的智力从 1.6 调整为 1.9",
        "调整：item:wisp_weave_vambraces 40级强化加成的魔法增伤从 0.12 调整为 0.1；50级强化加成的魔法增伤从 0.14 调整为 0.15",
        "调整：item:wood_spear 普通攻击的寻敌方式改为前排没有目标时，会继续向后排寻找",
        "调整：skill:arcane_meteor 第 1 个效果的目标数量从 全部目标 调整为 8；默认目标数量从 全部目标 调整为 8",
        "调整：skill:arrow_rain 第 1 个效果的目标数量从 5 调整为 4；第 1 个效果的目标数量公式中新增敏捷系数 0.05；第 1 个效果的目标数量上限新增为 7",
        "调整：skill:base4_knife_fan_skill 默认目标数量公式中新增敏捷系数 0.02",
        "调整：skill:blade_whirlwind 第 1 个效果的目标数量从 全部目标 调整为 4；第 1 个效果的目标数量公式中新增力量系数 0.02；第 1 个效果的目标数量上限新增为 1；默认目标数量从 全部目标 调整为 6",
        "调整：skill:base4_ancestor_decree 第 2 个效果调整",
        "调整：skill:roar 添加了可成长目标数； 第 1 个效果的数值公式中耐力系数从 0.5 调整为 1；第 1 个效果的伤害结算细节调整；第 1 个效果的寻敌方式改为只检查当前目标范围",
        "调整：affix:base4_affix_shadow_hunger 最多同时生效数量从 8 调整为 未限制",
        "调整：affix:base4_affix_shadow_vigor 最多同时生效数量从 8 调整为 未限制",
        "调整：affix:base4_affix_enhance_shadow_lens 最多同时生效数量从 未限制 调整为 8",
        "调整：affix:base4_affix_shadow_pierce 最多同时生效数量从 未限制 调整为 8",
        "调整：affix:base4_affix_shadow_rush 最多同时生效数量从 未限制 调整为 8",
        "调整：affix:enhancement_success_rate_minor 最多同时生效数量从 未限制 调整为 8",
        "调整：affix:magical_damage_resist 最多同时生效数量从 未限制 调整为 4",
        "调整：affix:physics_damage_resist 最多同时生效数量从 未限制 调整为 4",
        "调整：enemy:abyssal_lord_001 新增掉落 item:skillbook_base4_abyssal_pressure，掉落率 0.1%，数量 1~1；在战斗中装备了物品 item:wood_staff",
        "调整：enemy:cat_ancestor_king_001 新增掉落 item:skillbook_base4_ancestor_decree，掉落率 0.1%，数量 1~1；在战斗中装备了物品 item:sturdy_wood_staff",
        "调整：enemy:frost_spirit_001 在战斗中装备了物品 item:wood_staff",
        "调整：enemy:phantom_mouse_001 在战斗中装备了物品 item:iron_dagger",
        "调整：enemy:ruins_boss_001 新增掉落 item:skillbook_roar，掉落率 0.1%，数量 1~1",
        "调整：enemy:shadow_mage_001 在战斗中装备了物品 item:wood_staff",
        "调整：enemy:spirit_001 在战斗中装备了物品 item:wood_staff",
        "调整：enemy:witch_cat_001 在战斗中装备了物品 item:wood_staff",
        "调整：调整了极寒雪原、猫影深渊中部分敌人的攻击方式与技能",
        "其他：绑定邮箱，可在登陆后右上角个人玩家设置中进行绑定邮箱操作",
        "其他：忘记密码",
      ],
      "en-US": [
        "todo",
      ],
    }
  },
  {
    id: "2026-05-23",
    date: "2026-05-23",
    items: {
      "zh-CN": [
        "新增：item:sewing_weapon_1 item:sewing_weapon_2 item:sewing_weapon_3",
        "优化：词条新增装备类型约束，工作属性的词条仅可用于工作装备，战斗属性的词条仅可用于战斗装备",
        "修复： affix:base4_affix_shadow_hunger 无属性加成的问题",
        "调整：降低了通过 item:base4_shadow_ink 获得的附魔的效果",
        "调整：下调了 item:base4_shadow_ink 的掉落率",
        "优化：优化了动作中装备类行动的分类",
      ],
      "en-US": [
        "New: item:sewing_weapon_1 item:sewing_weapon_2 item:sewing_weapon_3",
        "Optimize: Added equip type constraints to affixes, work attribute affixes can only be applied to work equipment, battle attribute affixes can only be applied to battle equipment",
        "Fix: affix:base4_affix_shadow_hunger not providing attribute bonuses issue",
        "Adjust: Reduced the power of enchantments obtained from item:base4_shadow_ink",
        "Optimize: Improved the classification of equipment actions in animations",
      ],
    }
  },
  {
    id: "2026-05-22",
    date: "2026-05-22",
    items: {
      "zh-CN": [
        "优化：优化了战斗效率",
        "优化：部分词条无法装配问题。补充附魔与词条装配说明，并优化相关失败提示",
        "优化：拍卖详情中现在展示当前最高出价人的名称，出价区域增加“一口价填入”按钮",
        "优化：“技能释放规则”按钮上增加显示了当前技能的释放规则数量",
        "修复：战斗在波次切换时，技能冷却可能显示异常跳变的问题",
        "修复：部分技能释放规则不生效的问题",
        "修复：进入正在进行的战斗时，战斗画面可能停留在初始状态且没有任何动画的问题",
        "修复：技能释放规则现在可以完全清空，不会再强制保留一条默认规则",
        "修复：可能导致在取消队列时不返还装备的问题",
        "修复：未改动的固定默认词条装备现在可正常上架、回收，并可作为普通制造和强化材料使用",
        "修复：法杖类基础装备强化所需素材不正确的问题",
        "修复：一些本地化问题",
        "调整：市场现在可以交易与物品本身一致的所有可交易的通用物品",
        "调整：伤害结算中的穿透顺序改为先百分比穿透再固定穿透",
        "调整：当结算固定穿透超出剩余防御时，超出量会转为固定加伤",
        "调整：超穿转化的固定加伤会继续参与暴击、伤害增幅与减伤计算",
        "调整：创建拍卖时长输入改为“小时”，限制在 4~72 小时",
      ],
      "en-US": [
        "Optimize: Improved battle efficiency",
        "Optimize: Added clearer guidance for affix enchanting and attaching, with improved related failure messages",
        "Fix: Resolved an issue where skill cooldown display could jump unexpectedly when switching waves during battle replay",
        "Fix: Resolved an issue where later waves in continuous battles might not inherit the full player runtime state, causing skill rules, cooldowns, and status effects to behave incorrectly",
        "Fix: Resolved an issue where refreshing into an active battle could leave the battle view stuck at the initial state without replay animations",
        "Fix: Skill release rules can now be fully cleared instead of forcing one default rule to remain",
        "Fix: Issue where equipment might not be returned when canceling a queue",
        "Fix: Unmodified equipment with fixed default affixes, such as great shields and Moonlit Staffs, can now be listed, recycled, and used as normal crafting or enhancement materials",
        "Fix: Incorrect materials required for enhancing basic staff items issue",
        "Adjust: The fixed-price market now only trades template-matching commodity items; random-affix, enhanced, damaged, or customized equipment should use auctions",
        "Adjust: Changed the order of penetration in damage calculation to percentage penetration first, then flat penetration",
        "Adjust: When flat penetration exceeds remaining defense, the excess amount will now be converted to flat damage increase",
        "Adjust: Flat damage increase from over-penetration will now continue to be included in critical hit, damage amplification, and damage reduction calculations",
      ],
    }
  },
  {
    id: "2026-05-21-b",
    date: "2026-05-21",
    items: {
      "zh-CN": [
        "修复：同一个饰品可以装备在不同饰品槽的问题",
      ],
      "en-US": [
        "Fix: The same accessory can be equipped in different accessory slots issue",
      ],
    }
  },
  {
    id: "2026-05-21",
    date: "2026-05-21",
    items: {
      "zh-CN": [
        "优化：优化了网络环境，新增一个服务器节点，可通过 https://test-hk.moyu-idle.com 访问",
        "修复：修复了副本中敌人的数值与副本介绍中展示不一致的问题",
        "调整：降低了敌人的基础主属性以及属性桶分配逻辑，以补正敌人自带的次要属性的数值（攻击、防御等）"
      ],
      "en-US": [
        "Optimize: Optimized network environment with a new server node, accessible via https://test-hk.moyu-idle.com",
        "Fix: Fixed the issue where enemy stats in dungeons did not match the displayed information",
        "Adjust: Reduced the base primary attributes of enemies to balance the stats of their secondary attributes (such as attack, defense, etc.)"
      ],
    }
  },
  {
    id: "2026-05-20",
    date: "2026-05-20",
    items: {
      "zh-CN": [
        "优化：现在“工作计划”功能支持“立即执行”",
        "优化：自己拥有的可强化物品的详细信息弹窗中添加了“强化”快捷入口",
        "优化：优化了技能效果中，数值部分中包含技能等级的显示",
        "调整：降低了基础锻造动作的锻造等级要求",
        "调整：在 item:craftsmans_hammer item:hammer_of_divine_work 的描述中补充了此物品为一次性使用的说明",
        "调整：调整了 item:copper_helmet item:copper_armor item:copper_bracers item:copper_leggings 系列装备的强化加成",
        "调整：为大多数技能添加了技能等级加成",
        "调整：调整了盾类装备可获得的附魔类型",
        "修复：铜盾无法继续作为素材进行其他物品锻造的问题",
        ' - 当执行动作时，如果选择“使用具体物品”，那么只有选择的物品与对应物品默认的强化槽位一致时，才可以选择该物品',
        "修复：在物品中信息中点击“可产出于”中的动作后，在选择“交给谁”执行时，弹窗会消失问题",
        "修复：通过强化装备获得的属性，在详情界面中显示的属性提升为双倍的问题",
        "修复：技能书使用后会报错问题",
        "修复：强化概率显示有误问题",
      ],
      "en-US": [
        "Adjust: Reduced the forging level requirement for basic smithing actions",
        "Adjust: Added clarification in item:craftsmans_hammer and item:hammer_of_divine_work descriptions that these items are one-time use",
        "Fix: Copper shield can no longer be used as a material for other item forging issue",
        ' - When performing an action, if "Use specific items" is selected, only items that match the default affix slots of the corresponding item can be selected',
        "Fix: After clicking on the action in 'Produced by' in item details, the popup disappears when selecting 'Give to who' to perform the action issue",
        "Fix: Attribute bonuses obtained from enhancing equipment being displayed as double in the details page issue",
        "Fix: Error after using skill books issue",
        "Fix: Incorrect enhancement success rate display issue",
      ],
    }
  },
  {
    id: "2026-05-18-b",
    date: "2026-05-19",
    items: {
      "zh-CN": [
        // "新增：更新日志支持快捷引用 skill:arcane_missile 与 item:gold_coin",
        "新增：盾类武器",
        "新增：一些工作装",
        "新增： item:base2_sporeveil_robe",
        "优化：物品上附带的附魔数量及品质现在会在物品图标上体现",
        "修复：法杖装备强化后提供的是物理伤害的问题",
        "修复：拍卖行中的装备无法看到装备具体信息的问题",
        "修复：工作计划的预计时间没有考虑角色效率属性的问题",
        "修复：可能导致战斗过程卡住的一些问题",
        "修复：战斗內单位卡片中普通攻击信息错误问题",
        "调整：降低了“悠闲平原”每一波次的敌人数量",
        "调整：调整了敌人的属性分布，降低了速度属性的分布权重",
        "调整：调整了锻造产出的防具的装备属性需求",
        // "调整：现在每一场战斗，会随着战斗时长的增加，额外提供最终结算时的，有上限的百分比经验值加成",
      ],
      "en-US": [
        // "New: Update log now supports quick refs like skill:arcane_missile and item:gold_coin",
        "New: Shield type weapons",
        "New: Some work outfits",
        "New: item:base2_sporeveil_robe",
        "Optimize: The number and quality of enchantments on items can now be more clearly displayed on the item icon",
        "Fix: Staff equipment providing physical damage after enhancement issue",
        "Fix: Unable to view detailed information of equipment in auction issue",
        "Fix: Estimated time of work plans not taking character efficiency attribute into account issue",
        "Fix: Fixed some issues that could cause combat timeline to get stuck",
        "Fix: Incorrect normal attack information in battle unit cards issue",
        "Adjust: Reduced the number of enemies in each wave of 'Leisure Plains'",
        "Adjust: Adjusted attributes of some enemies, reducing the weight of Speed attribute",
        "Adjust: Adjusted the attribute requirements of armor produced by Smithing",
        // "Optimize: The number and quality of enchantments on items can now be more clearly displayed on the item icon",
      ],
    }
  },
  {
    id: "2026-05-18",
    date: "2026-05-18",
    items: {
      "zh-CN": [
        "新增：现在用户可以在个人设置->个性化中设置移动端菜单显示方式",
        "新增：动作详情新增“工作计划”能力，可一键生成制造链路并按计划加入队列",
        "新增：拍卖行",
        " - 拍卖行允许玩家交易定制化的物品，如包含强化或附魔的装备、拥有附魔的铭刻宝石等",
        "修复：百分比单位的属性，构成部分显示为小数的问题",
        "修复：物品强化提供的属性加成，每一级的加成会累加而不是按等级正确提供的问题",
        "修复：强化动作不提供经验的问题",
        "修复：初级疗愈与高级疗愈只对敌人生效的问题",
        "修复：物理攻击、魔法攻击的附魔没有数值问题",
        "调整：为“奥数魔法闪电”的每一部分伤害都添加了次要寻敌",
        "调整：调整了物品各等级的强化成功率",
        "调整：降低了每点智力提供的魔法防御",
      ],
      "en-US": [
        "New: Users can now set the mobile menu display mode in Personal Settings -> Personalization",
        "New: Action details now have a 'Work Plan' feature, allowing users to generate production chains and add them to the queue with one click",
        "Fix: Percentage-based attributes displaying as decimals in breakdown issue",
        "Fix: Attribute bonuses provided by item enhancement were being incorrectly applied cumulatively instead of per level",
        "Fix: Enhancement actions not providing experience issue",
        "Fix: Basic and Advanced Healing only affecting enemies issue",
        "Fix: Physical and magical attack enchantments not having values issue",
        "Adjust: Added secondary targeting to each part of the 'Arcane Magic Lightning' damage",
        "Adjust: Adjusted the enhancement success rates for items at various levels",
        "Adjust: Reduced the magic defense provided per point of Intelligence",
      ],
    }
  },
  {
    id: "2026-05-17-g",
    date: "2026-05-17",
    items: {
      "zh-CN": [
        "新增：现在对于可装备物品的生成动作（比如缝纫、制造、锻造），可以选择使用具体的装备作为消耗来完成这次动作",
        " - 如果选择了具体的装备，那么产出的装备将会继承该装备50%的强化等级",
        " - 拥有强化词条的装备无法被选择",
      ],
      "en-US": [
        "New: Now for crafting actions that can produce equipment (such as Tailoring, Smithing, etc.), you can choose to use a specific inventory item as the cost to perform the action",
        " - If a specific piece of equipment is chosen, the produced equipment will inherit 50% of the enhancement level of that piece of equipment",
        " - Equipment with enhancement attributes cannot be selected",
      ],
    }
  },
  {
    id: "2026-05-17-f",
    date: "2026-05-17",
    items: {
      "zh-CN": [
        "调整：悠闲平原、神秘洞窟的敌人的技能配置调整",
        "调整：神秘洞窟的敌人的数值调整",
      ],
      "en-US": [
        "Adjust: Adjusted the skill configuration of enemies in Leisure Plains and Mysterious Cave",
        "Adjust: Adjusted the stats of enemies in Mysterious Cave",
      ],
    }
  },
  {
    id: "2026-05-17-e",
    date: "2026-05-17",
    items: {
      "zh-CN": [
        "新增：矛类基础武器",
        "优化：技能逻辑中添加了对不同位置敌人数量的判断",
        "调整：调整了各属性提供的次要属性加成",
        "调整：现在大多数可以使用长剑类武器释放的技能，同样也支持大刀类、镰刀类武器了",
        "调整：重置战斗属性加点现在消耗金币x5000",
        "调整：镰刀类武器的攻击索敌模式",
        "修复：奥术飞弹技能目标数量错误问题",
        "修复：箭雨技能目标数量错误问题",
        "修复：技能释放规则中的“敌人数量”会把阵亡单位也计算在内的问题",
        "修复：移动端长按物品图标或物品卡片时，不会显示物品详细信息的问题",
        "修复：进入战斗详情页面时，有时会出现没有战斗过程的情况",
        "修复：点击菜单项时会经过一段延迟后才能进入页面的问题",
        "修复：强化后的装备，装备后属性变动部分数值显示问题",
        "修复：动作的预计耗时，始终显示为秒数的问题",
      ],
      "en-US": [
        "New: Basic weapon type - Spear",
        "Optimize: Added enemy count conditions based on enemy position in skill logic",
        "Adjust: Adjusted the secondary attribute bonuses provided by each attribute",
        "Adjust: Now most skills that can be released with longsword type weapons can also be released with greatsword type and scythe type weapons",
        "Adjust: Resetting battle attribute points now costs 5000 gold",
        "Adjust: Scythe weapon attack targeting mode",
        "Fix: Arcane Missiles skill target count issue",
        "Fix: Arrows Rain skill target count issue",
        "Fix: Enemy count in skill release rules counting dead units as well issue",
        "Fix: On mobile, long pressing item icons or item cards does not show item details issue",
        "Fix: Sometimes combat details page would show no combat timeline",
        "Fix: Delay before navigating to page after clicking menu item",
        "Fix: Display issue of changed attribute values after equipping enhanced equipment",
        "Fix: Estimated duration of actions always showing in seconds",
      ],
    }
  },
  {
    id: "2026-05-16-d",
    date: "2026-05-16",
    items: {
      "zh-CN": [
        "修复：属性变化后，动作队列中的动作耗时不会更新问题",
        "修复：装备提供的属性效果不会被应用到战斗中的问题",
        "修复：因双方都无法造成有效伤害导致战斗无限循环导致战斗服务卡顿的问题，现在战斗的回合数超过5000时，玩家阵营将会进入重整状态",
      ],
      "en-US": [
        "Fix: Action queue action duration not updating after attribute changes issue",
        "Fix: Attribute effects provided by equipment not being applied in combat issue",
        "Fix: Combat infinite loop and freezing issue when both sides cannot deal effective damage, now if the combat round exceeds 5000, the player's side will enter regrouping state",
      ],
    }
  },
  {
    id: "2026-05-16-c",
    date: "2026-05-16",
    items: {
      "zh-CN": [
        "优化：武器类装备上显示此武器的类型",
        "修复：采矿装备属性需求为“采集”的问题",
        "修复：玩家角色进入战斗时，属性加成会被计算两次问题",
        "修复：部分物品穿戴槽位异常问题",
        "修复：房间最大成员数量与副本所描述的不符问题",
        "修复：创建、加入房间时，不会自动跳转到房间界面问题",
        "调整：技能武器需求中的文本描述为类型id问题",
        "调整：降低了敏捷属性提供的暴击率与暴击伤害",
        "调整：提高了耐力属性提供的生命值与生命恢复",
        "调整：提高了智力属性提供的魔法值与魔法恢复",
        "调整：提高了力量与敏捷属性提供的物理攻击",
        "调整：箭雨现在索敌改为全场索敌",
        "调整：现在生活装备的附魔默认情况不会出现战斗类属性",
        "调整：战斗內单位卡片现在可以点击后查看详情",
      ],
      "en-US": [
        "Optimize: Display weapon type on weapon equipment",
        "Fix: Mining equipment attribute requirement issue",
        "Fix: Player character attribute bonuses being calculated twice when entering combat",
        "Fix: Some items having incorrect equip slot issue",
        "Fix: Room max member count not matching the description in the dungeon",
        "Fix: Not automatically navigating to room screen after creating or joining a room",
        "Adjust: Text description in skill weapon requirements showing type id issue",
        "Adjust: Reduced the crit chance and crit damage bonuses provided by Dexterity",
        "Adjust: Increased the HP and HP regen bonuses provided by Stamina",
        "Adjust: Increased the MP and MP regen bonuses provided by Intelligence",
        "Adjust: Increased the physical damage bonuses provided by Strength and Dexterity",
        "Adjust: Arrows Rain now targets the entire field instead of a small area around the target",
        "Adjust: Battle unit cards are now clickable to view details",
      ],
    }
  },
  {
    id: "2026-05-16-b",
    date: "2026-05-16",
    items: {
      "zh-CN": [
        "新增：一些工作装备",
        "优化：战斗效率问题处理",
        "优化：技能卡片交互",
        "优化：造成的伤害根据物理和魔法伤害类型显示不同的颜色",
        "调整：调整了敌人的属性分布",
        "修复：武器装备的攻击方式及数值修正(如攻击伤害构成、伤害类型等)没有生效问题",
        "修复：一些战斗属性没有生效问题",
        "修复：市场上架订单中，仅剩最后一件物品时，无法购买问题",
      ],
      "en-US": [
        "New: Some work equipment",
        "Optimize: Combat efficiency",
        "Optimize: Skill card interactions",
        "Optimize: Damage numbers now display different colors based on physical and magical damage types",
        "Adjust: Adjusted attributes of some enemies",
        "Fix: Weapon equipment attack method and stat corrections (such as attack damage composition, damage type, etc.) not taking effect",
        "Fix: Some battle attributes not taking effect",
        "Fix: Unable to purchase the last item in a market listing issue",
      ],
    }
  },
  {
    id: "2026-05-16-a",
    date: "2026-05-16",
    items: {
      "zh-CN": [
        "新增：猫舍",
        "修复：市场中单个物品无法购买的问题",
        "修复：离线弹窗显示过于频繁问题",
        "其他：更新日志菜单",
        "其他：问题反馈菜单",
        "其他：因为此项目是完全重写的，原有的功能会逐步的进行迁移，欢迎大家在游戏内的反馈菜单中提出任何意见和建议！",
      ],
      "en-US": [
        "New: Cattery",
        "Fix: Unable to purchase single items in the market",
        "Fix: Offline popup appears too frequently",
        "Other: Update log menu",
        "Other: Feedback menu",
        "Other: Since this project is a complete rewrite, many of the original features will be migrated gradually. Please feel free to provide any feedback and suggestions in the in-game feedback menu!",
      ],
    }
  }
]

assertValidEntries(sourceEntries)

export const updateLogTimeline: UpdateLogEntry[] = sortNewestFirst(sourceEntries)
