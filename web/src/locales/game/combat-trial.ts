const en = {
  kind: "Kitty Trials",
  currentFormat: "Current project",
  actionBudget: "Action budget",
  dummyDefense: "Dummy defense",
  attempts: "Attempts",
  unlimited: "Unlimited",
  refresh: "Refresh",
  formatMode: "Trial projects", formatHint: "Choose a project. Each one defines its own rules, objectives, and leaderboard.", rules: "Rules", opponents: "Trial objectives", formationPreview: "Formation preview", formationEditor: "Formation editor",
  passive: "Does not attack", active: "Active enemy", scriptedAlly: "Protected ally", behavior: "Behavior", healthMode: "Health", normalHealth: "Can be defeated", defense: "Defense", defensePair: "P.DEF / M.DEF {value}", damageResist: "Damage reduction", criticalDamageResist: "Critical resistance",
  waveNumber: "Wave {count}", waveCount: "{count} waves", waveTargets: "{count} objectives", waveProgress: "Wave {current} of {count}", waveNavigator: "Wave navigator", previousWave: "Previous wave", nextWave: "Next wave",
  tabs: { trial: "Trial", leaderboard: "Leaderboard", history: "Records" },
  lineup: "Trial team",
  lineupHintShared: "Uses your current character and up to two owned kitties. Units may share a row; their current state is frozen when the trial starts.",
  lineupHintDistinct: "Uses your current character and up to two owned kitties. Each unit must use a different row; their current state is frozen when the trial starts.",
  formation: "Trial formation",
  emptyPosition: "Empty",
  immutable: "Immortal target",
  currentPlayer: "You",
  kitty: "Kitty",
  kittySlot: "Kitty {slot}",
  noKitty: "No kitty",
  positionLocked: "This kitty's position is locked by personality",
  lineupInvalid: "Check the selected units and their positions.",
  rankedNote: "Eligible results are entered into this project's leaderboard automatically.",
  start: "Start trial",
  runQueuedToast: "Trial queued",
  runQueued: "Run #{id} is queued",
  runRunning: "Run #{id} is in progress",
  personalBests: "Personal bests",
  currentVersionOnly: "Current rules version",
  ruleActionBudget: "The run ends after {count} complete player-side main actions.",
  rulePositionShared: "Units may share a row",
  rulePositionDistinct: "Each unit uses a different row",
  ruleThreeDummies: "Three immortal dummies occupy the front, middle, and rear rows.",
  ruleNoRewards: "No items, EXP, medals, rating, or other resources are granted or consumed.",
  leaderboard: "Current leaderboard",
  leaderboardPrivacy: "Only name, scores, kitty count, and achievement time are public.",
  yourRank: "Your rank",
  rank: "Rank",
  player: "Player",
  kittyCount: "Kitties",
  achievedAt: "Achieved",
  leaderboardEmpty: "No results yet",
  history: "Recent trials",
  historyLimit: "Your latest {count} runs are retained.",
  historyEmpty: "No trial records yet",
  actionProgress: "Main actions",
  challengers: "Trial team",
  watchReplay: "Watch replay",
  shareReplay: "Attach replay to chat",
  shareReplaySuccess: "Replay attached. Send it from chat when ready.",
  removeReplayAttachment: "Remove replay attachment",
  replayRetentionHint: "Available while the run remains in recent records",
  chatReplayTitle: "Trial replay #{id}",
  replayTitle: "Trial replay #{id}",
  replayUnavailable: "This replay is no longer available.",
  replayInProgress: "Trial in progress",
  failed: "Compilation failed",
  status: { queued: "Queued", running: "Running", completed: "Completed", failed: "Failed" },
  endReason: { actionBudgetReached: "Action budget reached", teamDefeated: "Team defeated", enemyTeamDefeated: "Objective defeated", allWavesCleared: "All waves cleared" },
  position: { vanguard: "Front", midguard: "Middle", rearguard: "Rear" },
  dummies: { vanguard: "Front-row dummy", midguard: "Middle-row dummy", rearguard: "Rear-row dummy" },
  formats: {
    standard: {
      name: "Damage Trial",
      shortDesc: "Three-target damage challenge",
      desc: "Three immortal dummies use the current combat defense baseline and 50% critical resistance. The front row gains 20% damage resistance, the middle row 10%, and the rear row none. Score damage across 20 player-side main actions."
    },
    gauntlet: {
      name: "Gauntlet Run",
      shortDesc: "Five escalating enemy waves",
      desc: "Clear five waves at N2, N5, N10, N15, and N20 strength within the project action budget. Health, mana, buffs, and summons carry between waves. Rankings first track waves cleared; completed runs rank fewer player-side main turns higher.",
      waves: { meadowPatrol: "Meadow Patrol", forestHunt: "Forest Hunt", snowfieldScouts: "Snowfield Scouts", blizzardLine: "Blizzard Line", snowfieldCrown: "Snowfield Crown" }
    },
    recovery: {
      name: "Miracle Recovery",
      shortDesc: "Sustain a wounded ally",
      desc: "Protect a patient with 100 million max HP who starts at half health, alongside three passive practice targets. An undispellable wound removes 5% max HP at the start of each player-side main turn but cannot reduce the patient below 1 HP. Rankings track effective healing and remaining HP."
    }
  },
  objectives: { woundedPatient: "Wounded patient", recoveryClock: "Practice target", recoveryClockVanguard: "Front practice target", recoveryClockMidguard: "Middle practice target", recoveryClockRearguard: "Rear practice target" },
  metrics: {
    damageTotal: { name: "Total Damage", shortName: "Total damage", desc: "Final damage dealt by all player-side sources to all three dummies." },
    damageMaxSegment: { name: "Highest Single Hit", shortName: "Highest hit", desc: "The largest final-damage value from one hit against one target." },
    actionsToVictory: { name: "Clear Turns", shortName: "Clear turns", desc: "Player-side main turns elapsed before every wave is cleared. Fewer turns rank higher." },
    wavesCleared: { name: "Waves Cleared", shortName: "Waves", desc: "Enemy waves fully defeated before the trial ends." },
    objectiveUnitsDefeated: { name: "Objectives Defeated", shortName: "Defeated", desc: "Total objective enemies defeated across all waves." },
    effectiveHealing: { name: "Effective Healing", shortName: "Healing", desc: "Health actually restored to the protected patient; overhealing is excluded." },
    objectiveRemainingHp: { name: "Patient HP Remaining", shortName: "Remaining HP", desc: "The protected patient's remaining health when the trial ends." }
  },
  error: {
    disabled: "Kitty Trials are currently closed",
    formatInvalid: "This trial format is unavailable",
    metricInvalid: "This leaderboard metric is unavailable",
    runActive: "You already have a trial queued or running",
    runNotFound: "Trial record not found",
    replayExpired: "This replay is no longer retained",
    lineupChanged: "Equipment changed while the lineup was being frozen. Try again.",
    kittyNotFound: "A selected kitty is unavailable",
    lineupInvalid: "The trial lineup is invalid",
    alchemyEquipmentRestricted: "The lineup contains alchemy equipment above the League limit",
    overviewFailed: "Failed to load Kitty Trials",
    runCreateFailed: "Failed to start the trial",
    historyFailed: "Failed to load trial records",
    leaderboardFailed: "Failed to load the leaderboard",
    replayFailed: "Failed to load the replay",
    replaySharedFailed: "This shared replay is unavailable",
    replayShareFailed: "Failed to create the replay attachment"
  }
} as const

export const combatTrialEnUS = en

export const combatTrialZhCN = {
  kind: "猫猫试炼", currentFormat: "当前项目", actionBudget: "行动预算", dummyDefense: "木桩防御", attempts: "挑战次数", unlimited: "不限次数", refresh: "刷新",
  formatMode: "试炼项目", formatHint: "选择一项试炼项目，每个项目拥有独立规则、目标与排行榜。", rules: "规则说明", opponents: "试炼目标", formationPreview: "阵容预览", formationEditor: "阵容编辑", passive: "不攻击", active: "主动敌人", scriptedAlly: "需要守护的友方", behavior: "行动方式", healthMode: "生命规则", normalHealth: "可被击败", defense: "防御", defensePair: "物防 / 魔防 {value}", damageResist: "伤害减免", criticalDamageResist: "暴击抵抗",
  waveNumber: "第 {count} 波", waveCount: "{count} 波", waveTargets: "{count} 个目标", waveProgress: "第 {current} / {count} 波", waveNavigator: "波次导航", previousWave: "上一波", nextWave: "下一波",
  tabs: { trial: "试炼", leaderboard: "排行榜", history: "记录" },
  lineup: "出战小队", lineupHintShared: "使用当前角色，最多选择两只已有猫咪；单位可以位于同一排，开始时会冻结当前状态。", lineupHintDistinct: "使用当前角色，最多选择两只已有猫咪；每个单位需要位于不同排，开始时会冻结当前状态。", formation: "试炼阵形", emptyPosition: "空位", immutable: "生命保持满值",
  currentPlayer: "当前角色", kitty: "猫咪", kittySlot: "猫咪 {slot}", noKitty: "不携带猫咪", positionLocked: "该猫咪的站位已由性格锁定",
  lineupInvalid: "请检查所选单位及其站位。", rankedNote: "符合项目计分条件的结果会自动计入排行榜。", start: "开始试炼",
  runQueuedToast: "试炼已进入队列", runQueued: "第 #{id} 场试炼正在排队", runRunning: "第 #{id} 场试炼进行中",
  personalBests: "个人最佳", currentVersionOnly: "仅统计当前规则版本", ruleActionBudget: "全队完成 {count} 次玩家侧主行动后结束。", rulePositionShared: "单位可以位于同一排", rulePositionDistinct: "每个单位需要位于不同排", ruleThreeDummies: "前、中、后排各有一个不死木桩。", ruleNoRewards: "不产出也不消耗物品、经验、奖章、积分或其他资源。",
  leaderboard: "当前排行榜", leaderboardPrivacy: "仅公开玩家名、成绩、猫咪数量与达成时间。", yourRank: "我的排名", rank: "排名", player: "玩家", kittyCount: "猫咪数", achievedAt: "达成时间", leaderboardEmpty: "当前还没有成绩",
  history: "最近试炼", historyLimit: "保留最近 {count} 场试炼记录。", historyEmpty: "还没有试炼记录", actionProgress: "主行动", challengers: "试炼小队",
  watchReplay: "观看录像", shareReplay: "加入聊天附件", shareReplaySuccess: "录像已加入聊天，发送后才会公开。", removeReplayAttachment: "移除录像附件", replayRetentionHint: "仅在该场记录仍被保留时可观看",
  chatReplayTitle: "试炼录像 #{id}", replayTitle: "试炼录像 #{id}", replayUnavailable: "该录像已随历史记录清理，无法继续观看。", replayInProgress: "试炼进行中", failed: "编译失败",
  status: { queued: "排队中", running: "进行中", completed: "已完成", failed: "失败" }, endReason: { actionBudgetReached: "行动预算已用尽", teamDefeated: "队伍提前倒下", enemyTeamDefeated: "试炼目标已击败", allWavesCleared: "全部波次已通过" },
  position: { vanguard: "前排", midguard: "中排", rearguard: "后排" }, dummies: { vanguard: "前排木桩", midguard: "中排木桩", rearguard: "后排木桩" },
  formats: {
    standard: { name: "伤害试炼", shortDesc: "三目标伤害挑战", desc: "三个不死木桩采用当前战斗防御基准并拥有 50% 暴击抵抗；前排额外拥有 20% 伤害减免，中排 10%，后排不变。在 20 次玩家侧主行动内统计伤害。" },
    gauntlet: {
      name: "连战冲关", shortDesc: "五波渐强敌阵", desc: "在项目行动预算内连续挑战 N2、N5、N10、N15、N20 五波敌人，生命、魔法、状态与召唤物会延续至下一波。排名先比较通过波数；全部通关时，经过的玩家侧主行动回合越少排名越高。",
      waves: { meadowPatrol: "原野巡队", forestHunt: "密林围猎", snowfieldScouts: "雪原斥候", blizzardLine: "暴雪战线", snowfieldCrown: "雪原双冠" }
    },
    recovery: { name: "妙手回春", shortDesc: "守护负伤队员", desc: "守护一名生命上限一亿、初始半血的负伤队员，并面对三个不会攻击的演练目标。伤员拥有永久且不可驱散的伤势效果，每次玩家侧主行动回合开始时损失 5% 最大生命，最低保留 1 点；统计有效治疗与结束时剩余生命。" }
  },
  objectives: { woundedPatient: "负伤队员", recoveryClock: "演练目标", recoveryClockVanguard: "前排演练目标", recoveryClockMidguard: "中排演练目标", recoveryClockRearguard: "后排演练目标" },
  metrics: {
    damageTotal: { name: "总伤害", shortName: "总伤害", desc: "玩家侧所有来源对三个木桩造成的最终伤害总和。" },
    damageMaxSegment: { name: "最高单段伤害", shortName: "最高单段", desc: "对单个目标的一次命中所造成的最高最终伤害。" },
    actionsToVictory: { name: "通关回合数", shortName: "通关回合", desc: "全部波次通关前经过的玩家侧主行动回合数，越少排名越高。" },
    wavesCleared: { name: "通过波数", shortName: "通过波数", desc: "试炼结束前完整击败的敌人波数。" },
    objectiveUnitsDefeated: { name: "累计击败目标", shortName: "击败目标", desc: "所有波次中累计击败的计分目标数量。" },
    effectiveHealing: { name: "有效治疗", shortName: "有效治疗", desc: "实际为负伤队员恢复的生命值，溢出治疗不计入。" },
    objectiveRemainingHp: { name: "伤员剩余生命", shortName: "剩余生命", desc: "试炼结束时负伤队员剩余的生命值。" }
  },
  error: { disabled: "猫猫试炼暂未开放", formatInvalid: "该试炼项目当前不可用", metricInvalid: "该排行榜指标当前不可用", runActive: "已有一场试炼正在排队或运行", runNotFound: "试炼记录不存在", replayExpired: "该录像已不在保留范围内", lineupChanged: "冻结阵容时装备发生变化，请重试", kittyNotFound: "选择的猫咪当前不可用", lineupInvalid: "出战小队无效", alchemyEquipmentRestricted: "小队中的炼金装备超过联赛限制", overviewFailed: "猫猫试炼加载失败", runCreateFailed: "无法开始试炼", historyFailed: "试炼记录加载失败", leaderboardFailed: "排行榜加载失败", replayFailed: "录像加载失败", replaySharedFailed: "该共享录像已失效", replayShareFailed: "录像分享创建失败" }
} as const

export const combatTrialZhTW = {
  kind: "貓貓試煉", currentFormat: "目前項目", actionBudget: "行動預算", dummyDefense: "木樁防禦", attempts: "挑戰次數", unlimited: "不限次數", refresh: "重新整理",
  formatMode: "試煉項目", formatHint: "選擇一項試煉項目，每個項目都有獨立規則、目標與排行榜。", rules: "規則說明", opponents: "試煉目標", formationPreview: "陣容預覽", formationEditor: "陣容編輯", passive: "不攻擊", active: "主動敵人", scriptedAlly: "需要守護的友方", behavior: "行動方式", healthMode: "生命規則", normalHealth: "可被擊敗", defense: "防禦", defensePair: "物防 / 魔防 {value}", damageResist: "傷害減免", criticalDamageResist: "暴擊抵抗",
  waveNumber: "第 {count} 波", waveCount: "{count} 波", waveTargets: "{count} 個目標", waveProgress: "第 {current} / {count} 波", waveNavigator: "波次導覽", previousWave: "上一波", nextWave: "下一波",
  tabs: { trial: "試煉", leaderboard: "排行榜", history: "紀錄" },
  lineup: "出戰小隊", lineupHintShared: "使用目前角色，最多選擇兩隻已有貓咪；單位可以位於同一排，開始時會凍結目前狀態。", lineupHintDistinct: "使用目前角色，最多選擇兩隻已有貓咪；每個單位需要位於不同排，開始時會凍結目前狀態。", formation: "試煉陣形", emptyPosition: "空位", immutable: "生命保持滿值",
  currentPlayer: "目前角色", kitty: "貓咪", kittySlot: "貓咪 {slot}", noKitty: "不攜帶貓咪", positionLocked: "該貓咪的站位已由性格鎖定",
  lineupInvalid: "請檢查所選單位及其站位。", rankedNote: "符合項目計分條件的結果會自動計入排行榜。", start: "開始試煉",
  runQueuedToast: "試煉已進入佇列", runQueued: "第 #{id} 場試煉正在排隊", runRunning: "第 #{id} 場試煉進行中",
  personalBests: "個人最佳", currentVersionOnly: "僅統計目前規則版本", ruleActionBudget: "全隊完成 {count} 次玩家側主行動後結束。", rulePositionShared: "單位可以位於同一排", rulePositionDistinct: "每個單位需要位於不同排", ruleThreeDummies: "前、中、後排各有一個不死木樁。", ruleNoRewards: "不產出也不消耗物品、經驗、獎章、積分或其他資源。",
  leaderboard: "目前排行榜", leaderboardPrivacy: "僅公開玩家名稱、成績、貓咪數量與達成時間。", yourRank: "我的排名", rank: "排名", player: "玩家", kittyCount: "貓咪數", achievedAt: "達成時間", leaderboardEmpty: "目前還沒有成績",
  history: "最近試煉", historyLimit: "保留最近 {count} 場試煉紀錄。", historyEmpty: "還沒有試煉紀錄", actionProgress: "主行動", challengers: "試煉小隊",
  watchReplay: "觀看錄影", shareReplay: "加入聊天附件", shareReplaySuccess: "錄影已加入聊天，傳送後才會公開。", removeReplayAttachment: "移除錄影附件", replayRetentionHint: "僅在該場紀錄仍被保留時可觀看",
  chatReplayTitle: "試煉錄影 #{id}", replayTitle: "試煉錄影 #{id}", replayUnavailable: "該錄影已隨歷史紀錄清理，無法繼續觀看。", replayInProgress: "試煉進行中", failed: "編譯失敗",
  status: { queued: "排隊中", running: "進行中", completed: "已完成", failed: "失敗" }, endReason: { actionBudgetReached: "行動預算已用盡", teamDefeated: "隊伍提前倒下", enemyTeamDefeated: "試煉目標已擊敗", allWavesCleared: "全部波次已通過" },
  position: { vanguard: "前排", midguard: "中排", rearguard: "後排" }, dummies: { vanguard: "前排木樁", midguard: "中排木樁", rearguard: "後排木樁" },
  formats: {
    standard: { name: "傷害試煉", shortDesc: "三目標傷害挑戰", desc: "三個不死木樁採用目前戰鬥防禦基準並擁有 50% 暴擊抵抗；前排額外擁有 20% 傷害減免，中排 10%，後排不變。在 20 次玩家側主行動內統計傷害。" },
    gauntlet: { name: "連戰衝關", shortDesc: "五波漸強敵陣", desc: "在項目行動預算內連續挑戰 N2、N5、N10、N15、N20 五波敵人，生命、魔法、狀態與召喚物會延續至下一波。排名先比較通過波數；全部通關時，經過的玩家側主行動回合越少排名越高。", waves: { meadowPatrol: "原野巡隊", forestHunt: "密林圍獵", snowfieldScouts: "雪原斥候", blizzardLine: "暴雪戰線", snowfieldCrown: "雪原雙冠" } },
    recovery: { name: "妙手回春", shortDesc: "守護負傷隊員", desc: "守護一名生命上限一億、初始半血的負傷隊員，並面對三個不會攻擊的演練目標。傷員擁有永久且不可驅散的傷勢效果，每次玩家側主行動回合開始時損失 5% 最大生命，最低保留 1 點；統計有效治療與結束時剩餘生命。" }
  },
  objectives: { woundedPatient: "負傷隊員", recoveryClock: "演練目標", recoveryClockVanguard: "前排演練目標", recoveryClockMidguard: "中排演練目標", recoveryClockRearguard: "後排演練目標" },
  metrics: {
    damageTotal: { name: "總傷害", shortName: "總傷害", desc: "玩家側所有來源對三個木樁造成的最終傷害總和。" }, damageMaxSegment: { name: "最高單段傷害", shortName: "最高單段", desc: "對單一目標的一次命中所造成的最高最終傷害。" }, actionsToVictory: { name: "通關回合數", shortName: "通關回合", desc: "全部波次通關前經過的玩家側主行動回合數，越少排名越高。" },
    wavesCleared: { name: "通過波數", shortName: "通過波數", desc: "試煉結束前完整擊敗的敵人波數。" }, objectiveUnitsDefeated: { name: "累計擊敗目標", shortName: "擊敗目標", desc: "所有波次中累計擊敗的計分目標數量。" }, effectiveHealing: { name: "有效治療", shortName: "有效治療", desc: "實際為負傷隊員恢復的生命值，溢出治療不計入。" }, objectiveRemainingHp: { name: "傷員剩餘生命", shortName: "剩餘生命", desc: "試煉結束時負傷隊員剩餘的生命值。" }
  },
  error: { disabled: "貓貓試煉暫未開放", formatInvalid: "該試煉項目目前不可用", metricInvalid: "該排行榜指標目前不可用", runActive: "已有一場試煉正在排隊或執行", runNotFound: "試煉紀錄不存在", replayExpired: "該錄影已不在保留範圍內", lineupChanged: "凍結陣容時裝備發生變化，請重試", kittyNotFound: "選擇的貓咪目前不可用", lineupInvalid: "出戰小隊無效", alchemyEquipmentRestricted: "小隊中的煉金裝備超過聯賽限制", overviewFailed: "貓貓試煉載入失敗", runCreateFailed: "無法開始試煉", historyFailed: "試煉紀錄載入失敗", leaderboardFailed: "排行榜載入失敗", replayFailed: "錄影載入失敗", replaySharedFailed: "該共享錄影已失效", replayShareFailed: "錄影分享建立失敗" }
} as const

export const combatTrialJa = {
  kind: "猫猫試練", currentFormat: "現在の種目", actionBudget: "行動上限", dummyDefense: "木人の防御", attempts: "挑戦回数", unlimited: "回数無制限", refresh: "更新",
  formatMode: "試練種目", formatHint: "種目ごとに異なるルール、目標、ランキングが用意されています。", rules: "ルール", opponents: "試練目標", formationPreview: "編成プレビュー", formationEditor: "編成編集", passive: "攻撃しない", active: "行動する敵", scriptedAlly: "守護対象の味方", behavior: "行動", healthMode: "生命ルール", normalHealth: "撃破可能", defense: "防御", defensePair: "物防 / 魔防 {value}", damageResist: "ダメージ軽減", criticalDamageResist: "会心耐性",
  waveNumber: "第 {count} 波", waveCount: "{count} 波", waveTargets: "目標 {count} 体", waveProgress: "第 {current} / {count} 波", waveNavigator: "波ナビゲーション", previousWave: "前の波", nextWave: "次の波",
  tabs: { trial: "試練", leaderboard: "ランキング", history: "記録" }, lineup: "出場チーム", lineupHintShared: "現在のプレイヤーと所持猫2匹までを使用します。同じ列にも配置でき、開始時の状態を固定します。", lineupHintDistinct: "現在のプレイヤーと所持猫2匹までを使用します。各ユニットを別の列に配置し、開始時の状態を固定します。", formation: "試練陣形", emptyPosition: "空き", immutable: "HPは常に満タン",
  currentPlayer: "プレイヤー", kitty: "猫", kittySlot: "猫 {slot}", noKitty: "猫なし", positionLocked: "性格によりこの猫の配置は固定されています", lineupInvalid: "選択したユニットと配置を確認してください。", rankedNote: "種目の集計条件を満たす結果はランキングに自動登録されます。",
  start: "試練開始", runQueuedToast: "試練を待機列に追加しました", runQueued: "試練 #{id} は待機中", runRunning: "試練 #{id} は進行中",
  personalBests: "自己ベスト", currentVersionOnly: "現在のルール版のみ", ruleActionBudget: "味方側の主行動を {count} 回完了すると終了します。", rulePositionShared: "同じ列に複数配置可能", rulePositionDistinct: "各ユニットは別の列に配置", ruleThreeDummies: "前・中・後列に不死の木人が1体ずついます。", ruleNoRewards: "アイテム、経験値、勲章、レートなどの獲得・消費はありません。",
  leaderboard: "現在のランキング", leaderboardPrivacy: "名前、スコア、猫の数、達成時刻だけが公開されます。", yourRank: "自分の順位", rank: "順位", player: "プレイヤー", kittyCount: "猫数", achievedAt: "達成時刻", leaderboardEmpty: "まだ記録がありません",
  history: "最近の試練", historyLimit: "直近 {count} 件を保存します。", historyEmpty: "試練記録はありません", actionProgress: "主行動", challengers: "試練チーム", watchReplay: "リプレイを見る", shareReplay: "チャットに添付", shareReplaySuccess: "リプレイをチャットに添付しました。送信後に公開されます。", removeReplayAttachment: "添付を外す", replayRetentionHint: "最近の記録に残っている間のみ視聴可能",
  chatReplayTitle: "試練リプレイ #{id}", replayTitle: "試練リプレイ #{id}", replayUnavailable: "このリプレイは保存期間を過ぎたため利用できません。", replayInProgress: "試練進行中", failed: "コンパイル失敗",
  status: { queued: "待機中", running: "進行中", completed: "完了", failed: "失敗" }, endReason: { actionBudgetReached: "行動上限に到達", teamDefeated: "チーム全滅", enemyTeamDefeated: "目標撃破", allWavesCleared: "全波を突破" }, position: { vanguard: "前列", midguard: "中列", rearguard: "後列" }, dummies: { vanguard: "前列の木人", midguard: "中列の木人", rearguard: "後列の木人" },
  formats: {
    standard: { name: "ダメージ試練", shortDesc: "3目標ダメージ挑戦", desc: "現在の戦闘防御基準と50%の会心耐性を持つ3体の不死木人に、前列20%、中列10%のダメージ軽減を追加し、後列は据え置きです。味方側20回の主行動で与えたダメージを競います。" },
    gauntlet: { name: "連戦突破", shortDesc: "強さを増す5波", desc: "種目の行動上限内にN2、N5、N10、N15、N20の5波へ連続挑戦します。HP、MP、状態、召喚は次の波へ引き継がれます。まず突破波数を比較し、全波突破時は味方側の主行動ターンが少ないほど上位です。", waves: { meadowPatrol: "草原巡回隊", forestHunt: "森林包囲", snowfieldScouts: "雪原斥候", blizzardLine: "吹雪戦線", snowfieldCrown: "雪原双冠" } },
    recovery: { name: "妙手回春", shortDesc: "負傷した味方を守る", desc: "最大HP1億、HP半分から始まる負傷者を、攻撃しない3体の演習目標とともに守ります。負傷者の永久かつ解除不能な傷は、味方側の各主行動ターン開始時に最大HPの5%を失わせますが、1未満にはなりません。有効回復量と終了時HPを記録します。" }
  },
  objectives: { woundedPatient: "負傷者", recoveryClock: "演習用標的", recoveryClockVanguard: "前列の演習標的", recoveryClockMidguard: "中列の演習標的", recoveryClockRearguard: "後列の演習標的" },
  metrics: {
    damageTotal: { name: "総ダメージ", shortName: "総ダメージ", desc: "味方側の全ソースが3体の木人に与えた最終ダメージの合計。" }, damageMaxSegment: { name: "最大単発ダメージ", shortName: "最大単発", desc: "1体への1ヒットで記録した最大の最終ダメージ。" }, actionsToVictory: { name: "突破ターン数", shortName: "突破ターン", desc: "全波を突破するまでに経過した味方側の主行動ターン数。少ないほど上位です。" },
    wavesCleared: { name: "突破波数", shortName: "突破波数", desc: "試練終了までに完全撃破した敵の波数。" }, objectiveUnitsDefeated: { name: "目標撃破数", shortName: "撃破数", desc: "全波で撃破した得点対象の合計。" }, effectiveHealing: { name: "有効回復量", shortName: "有効回復", desc: "負傷者に実際に回復したHP。過剰回復は含みません。" }, objectiveRemainingHp: { name: "負傷者の残りHP", shortName: "残りHP", desc: "試練終了時に負傷者へ残ったHP。" }
  },
  error: { disabled: "猫猫試練は現在利用できません", formatInvalid: "この試練種目は利用できません", metricInvalid: "このランキング指標は利用できません", runActive: "待機中または進行中の試練があります", runNotFound: "試練記録が見つかりません", replayExpired: "このリプレイは保存期間を過ぎました", lineupChanged: "編成の固定中に装備が変わりました。再試行してください", kittyNotFound: "選択した猫は利用できません", lineupInvalid: "出場チームが無効です", alchemyEquipmentRestricted: "錬金装備がリーグ上限を超えています", overviewFailed: "猫猫試練を読み込めませんでした", runCreateFailed: "試練を開始できませんでした", historyFailed: "試練記録を読み込めませんでした", leaderboardFailed: "ランキングを読み込めませんでした", replayFailed: "リプレイを読み込めませんでした", replaySharedFailed: "共有リプレイは利用できません", replayShareFailed: "リプレイを共有できませんでした" }
} as const

export const combatTrialKo = {
  kind: "냥냥 시련", currentFormat: "현재 종목", actionBudget: "행동 제한", dummyDefense: "허수아비 방어", attempts: "도전 횟수", unlimited: "횟수 무제한", refresh: "새로고침",
  formatMode: "시련 종목", formatHint: "종목마다 별도의 규칙, 목표, 순위표가 적용됩니다.", rules: "규칙", opponents: "시련 목표", formationPreview: "편성 미리보기", formationEditor: "편성 편집", passive: "공격하지 않음", active: "행동하는 적", scriptedAlly: "보호 대상 아군", behavior: "행동 방식", healthMode: "생명 규칙", normalHealth: "처치 가능", defense: "방어", defensePair: "물방 / 마방 {value}", damageResist: "피해 감소", criticalDamageResist: "치명타 저항",
  waveNumber: "제 {count} 웨이브", waveCount: "{count} 웨이브", waveTargets: "목표 {count}개", waveProgress: "{current} / {count} 웨이브", waveNavigator: "웨이브 탐색", previousWave: "이전 웨이브", nextWave: "다음 웨이브",
  tabs: { trial: "시련", leaderboard: "순위표", history: "기록" }, lineup: "출전 팀", lineupHintShared: "현재 캐릭터와 보유 고양이 최대 두 마리를 사용합니다. 같은 열에 배치할 수 있으며 시작 시 상태를 고정합니다.", lineupHintDistinct: "현재 캐릭터와 보유 고양이 최대 두 마리를 사용합니다. 각 유닛을 다른 열에 배치하고 시작 시 상태를 고정합니다.", formation: "시련 진형", emptyPosition: "빈 자리", immutable: "생명력 항상 최대",
  currentPlayer: "플레이어", kitty: "고양이", kittySlot: "고양이 {slot}", noKitty: "고양이 없음", positionLocked: "성격 때문에 이 고양이의 위치가 고정되어 있습니다", lineupInvalid: "선택한 유닛과 위치를 확인하세요.", rankedNote: "종목의 집계 조건을 충족한 결과는 순위표에 자동 등록됩니다.", start: "수련 시작", runQueuedToast: "수련이 대기열에 등록되었습니다", runQueued: "수련 #{id} 대기 중", runRunning: "수련 #{id} 진행 중",
  personalBests: "개인 최고", currentVersionOnly: "현재 규칙 버전", ruleActionBudget: "아군 측 주 행동을 {count}회 완료하면 종료됩니다.", rulePositionShared: "같은 열에 여러 유닛 배치 가능", rulePositionDistinct: "각 유닛을 다른 열에 배치", ruleThreeDummies: "전열, 중열, 후열에 불사 허수아비가 하나씩 있습니다.", ruleNoRewards: "아이템, 경험치, 메달, 점수 등 어떤 자원도 획득하거나 소모하지 않습니다.",
  leaderboard: "현재 순위표", leaderboardPrivacy: "이름, 점수, 고양이 수, 달성 시각만 공개됩니다.", yourRank: "내 순위", rank: "순위", player: "플레이어", kittyCount: "고양이 수", achievedAt: "달성 시각", leaderboardEmpty: "아직 기록이 없습니다", history: "최근 수련", historyLimit: "최근 {count}개의 수련 기록을 보관합니다.", historyEmpty: "수련 기록이 없습니다", actionProgress: "주 행동", challengers: "수련 팀",
  watchReplay: "리플레이 보기", shareReplay: "채팅에 첨부", shareReplaySuccess: "리플레이를 채팅에 첨부했습니다. 전송 후 공개됩니다.", removeReplayAttachment: "첨부 제거", replayRetentionHint: "최근 기록에 남아 있는 동안만 시청 가능", chatReplayTitle: "수련 리플레이 #{id}", replayTitle: "수련 리플레이 #{id}", replayUnavailable: "이 리플레이는 보관 기간이 지나 사용할 수 없습니다.", replayInProgress: "수련 진행 중", failed: "컴파일 실패",
  status: { queued: "대기 중", running: "진행 중", completed: "완료", failed: "실패" }, endReason: { actionBudgetReached: "행동 제한 도달", teamDefeated: "팀 전멸", enemyTeamDefeated: "목표 처치", allWavesCleared: "모든 웨이브 돌파" }, position: { vanguard: "전열", midguard: "중열", rearguard: "후열" }, dummies: { vanguard: "전열 허수아비", midguard: "중열 허수아비", rearguard: "후열 허수아비" },
  formats: {
    standard: { name: "피해 시련", shortDesc: "피해 도전", desc: "현재 전투 방어 기준과 치명타 저항 50%를 가진 불사 허수아비 3개 중 전열에는 피해 감소 20%, 중열에는 10%를 추가하고 후열은 그대로 유지합니다. 아군 측 주 행동 20회 동안 피해를 측정합니다." },
    gauntlet: { name: "연전 돌파", shortDesc: "점점 강해지는 5개 웨이브", desc: "종목의 행동 제한 안에서 N2, N5, N10, N15, N20의 5개 웨이브를 연속으로 상대합니다. HP, MP, 상태와 소환수는 다음 웨이브로 이어집니다. 먼저 돌파 웨이브를 비교하고, 모두 돌파한 경우 아군 측 주 행동 턴이 적을수록 높은 순위가 됩니다.", waves: { meadowPatrol: "초원 순찰대", forestHunt: "숲 포위전", snowfieldScouts: "설원 정찰대", blizzardLine: "눈보라 전선", snowfieldCrown: "설원 쌍관" } },
    recovery: { name: "묘수회춘", shortDesc: "부상당한 아군 보호", desc: "최대 HP 1억, 절반의 HP로 시작하는 부상자를 공격하지 않는 연습 목표 3개와 함께 보호합니다. 영구적이고 해제할 수 없는 상처는 아군 측 주 행동 턴이 시작될 때마다 최대 HP의 5%를 잃게 하지만 1 미만으로 내려가지 않습니다. 유효 치유량과 종료 시 남은 HP를 기록합니다." }
  },
  objectives: { woundedPatient: "부상자", recoveryClock: "훈련용 표적", recoveryClockVanguard: "전열 훈련 표적", recoveryClockMidguard: "중열 훈련 표적", recoveryClockRearguard: "후열 훈련 표적" },
  metrics: {
    damageTotal: { name: "총 피해", shortName: "총 피해", desc: "아군 측 모든 소스가 세 허수아비에게 준 최종 피해의 합계입니다." }, damageMaxSegment: { name: "최고 단일 피해", shortName: "최고 단일", desc: "한 대상에게 한 번 적중하여 준 가장 높은 최종 피해입니다." }, actionsToVictory: { name: "돌파 턴 수", shortName: "돌파 턴", desc: "모든 웨이브를 돌파하기까지 지난 아군 측 주 행동 턴 수입니다. 적을수록 높은 순위가 됩니다." },
    wavesCleared: { name: "돌파 웨이브", shortName: "돌파", desc: "시련 종료 전 완전히 처치한 적 웨이브 수입니다." }, objectiveUnitsDefeated: { name: "목표 처치 수", shortName: "처치 수", desc: "모든 웨이브에서 처치한 점수 목표의 합계입니다." }, effectiveHealing: { name: "유효 치유량", shortName: "유효 치유", desc: "부상자에게 실제로 회복된 HP이며 초과 치유는 제외됩니다." }, objectiveRemainingHp: { name: "부상자 남은 HP", shortName: "남은 HP", desc: "시련 종료 시 부상자에게 남은 HP입니다." }
  },
  error: { disabled: "수련장은 현재 이용할 수 없습니다", formatInvalid: "이 수련 규칙은 이용할 수 없습니다", metricInvalid: "이 순위 지표는 이용할 수 없습니다", runActive: "대기 중이거나 진행 중인 수련이 있습니다", runNotFound: "수련 기록을 찾을 수 없습니다", replayExpired: "이 리플레이는 더 이상 보관되지 않습니다", lineupChanged: "편성을 고정하는 동안 장비가 변경되었습니다. 다시 시도하세요", kittyNotFound: "선택한 고양이를 이용할 수 없습니다", lineupInvalid: "수련 편성이 올바르지 않습니다", alchemyEquipmentRestricted: "연금 장비가 리그 제한을 초과합니다", overviewFailed: "수련장을 불러오지 못했습니다", runCreateFailed: "수련을 시작하지 못했습니다", historyFailed: "수련 기록을 불러오지 못했습니다", leaderboardFailed: "순위표를 불러오지 못했습니다", replayFailed: "리플레이를 불러오지 못했습니다", replaySharedFailed: "공유 리플레이를 사용할 수 없습니다", replayShareFailed: "리플레이를 공유하지 못했습니다" }
} as const

export const combatTrialRu = {
  kind: "Кошачьи испытания", currentFormat: "Текущее испытание", actionBudget: "Лимит действий", dummyDefense: "Защита манекена", attempts: "Попытки", unlimited: "Без лимита", refresh: "Обновить",
  formatMode: "Виды испытаний", formatHint: "У каждого вида свои правила, цели и рейтинг.", rules: "Правила", opponents: "Цели испытания", formationPreview: "Предпросмотр состава", formationEditor: "Редактор состава", passive: "Не атакует", active: "Активный враг", scriptedAlly: "Защищаемый союзник", behavior: "Поведение", healthMode: "Здоровье", normalHealth: "Можно победить", defense: "Защита", defensePair: "Физ. / маг. защита {value}", damageResist: "Снижение урона", criticalDamageResist: "Сопротивление критам",
  waveNumber: "Волна {count}", waveCount: "Волн: {count}", waveTargets: "Целей: {count}", waveProgress: "Волна {current} из {count}", waveNavigator: "Навигация по волнам", previousWave: "Предыдущая волна", nextWave: "Следующая волна",
  tabs: { trial: "Испытание", leaderboard: "Рейтинг", history: "История" }, lineup: "Команда", lineupHintShared: "Используется текущий герой и до двух ваших котов. Участники могут занимать один ряд; состояние фиксируется при старте.", lineupHintDistinct: "Используется текущий герой и до двух ваших котов. Каждый участник занимает отдельный ряд; состояние фиксируется при старте.", formation: "Расстановка", emptyPosition: "Пусто", immutable: "Здоровье всегда полное",
  currentPlayer: "Игрок", kitty: "Кот", kittySlot: "Кот {slot}", noKitty: "Без кота", positionLocked: "Позиция кота закреплена его характером", lineupInvalid: "Проверьте выбранных участников и их позиции.", rankedNote: "Результаты, отвечающие условиям проекта, автоматически попадают в рейтинг.", start: "Начать", runQueuedToast: "Испытание поставлено в очередь", runQueued: "Испытание #{id} в очереди", runRunning: "Испытание #{id} идет",
  personalBests: "Личные рекорды", currentVersionOnly: "Текущая версия правил", ruleActionBudget: "Испытание завершится после {count} основных действий вашей стороны.", rulePositionShared: "Участники могут занимать один ряд", rulePositionDistinct: "Каждый участник занимает отдельный ряд", ruleThreeDummies: "По одному бессмертному манекену в переднем, среднем и заднем рядах.", ruleNoRewards: "Предметы, опыт, медали, рейтинг и другие ресурсы не выдаются и не расходуются.",
  leaderboard: "Текущий рейтинг", leaderboardPrivacy: "Публичны только имя, результаты, число котов и время достижения.", yourRank: "Ваше место", rank: "Место", player: "Игрок", kittyCount: "Коты", achievedAt: "Получено", leaderboardEmpty: "Результатов пока нет", history: "Недавние испытания", historyLimit: "Хранятся последние {count} испытаний.", historyEmpty: "Истории пока нет", actionProgress: "Основные действия", challengers: "Команда",
  watchReplay: "Смотреть повтор", shareReplay: "Прикрепить к чату", shareReplaySuccess: "Повтор прикреплен к чату и станет публичным после отправки.", removeReplayAttachment: "Удалить вложение", replayRetentionHint: "Доступен, пока запись хранится в истории", chatReplayTitle: "Повтор испытания #{id}", replayTitle: "Повтор испытания #{id}", replayUnavailable: "Этот повтор больше недоступен: запись удалена из истории.", replayInProgress: "Испытание идет", failed: "Ошибка компиляции",
  status: { queued: "В очереди", running: "Выполняется", completed: "Завершено", failed: "Ошибка" }, endReason: { actionBudgetReached: "Лимит действий исчерпан", teamDefeated: "Команда побеждена", enemyTeamDefeated: "Цель побеждена", allWavesCleared: "Все волны пройдены" }, position: { vanguard: "Передний ряд", midguard: "Средний ряд", rearguard: "Задний ряд" }, dummies: { vanguard: "Передний манекен", midguard: "Средний манекен", rearguard: "Задний манекен" },
  formats: {
    standard: { name: "Испытание урона", shortDesc: "Урон", desc: "Три бессмертных манекена используют текущий боевой уровень защиты и имеют 50% сопротивления критам. Передний ряд получает 20% снижения урона, средний 10%, задний остается без дополнительного снижения. Урон считается за 20 основных действий вашей стороны." },
    gauntlet: { name: "Серия боёв", shortDesc: "Пять усиливающихся волн", desc: "Победите пять волн силы N2, N5, N10, N15 и N20 в пределах лимита действий этого испытания. Здоровье, мана, эффекты и призывы переходят между волнами. Сначала сравниваются пройденные волны; при полном прохождении выше тот, кто потратил меньше основных ходов своей стороны.", waves: { meadowPatrol: "Луговой патруль", forestHunt: "Лесная охота", snowfieldScouts: "Снежные разведчики", blizzardLine: "Линия метели", snowfieldCrown: "Снежная корона" } },
    recovery: { name: "Чудесное исцеление", shortDesc: "Защита раненого союзника", desc: "Защитите раненого с максимумом 100 миллионов HP и половиной здоровья рядом с тремя неатакующими учебными целями. Постоянная неснимаемая рана отнимает 5% максимального HP в начале каждого основного хода вашей стороны, но не снижает здоровье ниже 1. Учитываются эффективное лечение и оставшееся здоровье." }
  },
  objectives: { woundedPatient: "Раненый союзник", recoveryClock: "Учебная цель", recoveryClockVanguard: "Передняя учебная цель", recoveryClockMidguard: "Средняя учебная цель", recoveryClockRearguard: "Задняя учебная цель" },
  metrics: {
    damageTotal: { name: "Общий урон", shortName: "Общий урон", desc: "Сумма итогового урона от всех источников вашей стороны по трем манекенам." }, damageMaxSegment: { name: "Максимальный удар", shortName: "Макс. удар", desc: "Наибольший итоговый урон одного попадания по одной цели." }, actionsToVictory: { name: "Ходы до прохождения", shortName: "Ходы", desc: "Число основных ходов вашей стороны до прохождения всех волн. Меньшее значение занимает более высокое место." },
    wavesCleared: { name: "Пройдено волн", shortName: "Волны", desc: "Число полностью побежденных волн до завершения испытания." }, objectiveUnitsDefeated: { name: "Побеждено целей", shortName: "Цели", desc: "Общее число зачетных целей, побежденных во всех волнах." }, effectiveHealing: { name: "Эффективное лечение", shortName: "Лечение", desc: "Фактически восстановленное раненому здоровье без избыточного лечения." }, objectiveRemainingHp: { name: "Оставшееся здоровье", shortName: "Здоровье", desc: "Здоровье раненого союзника в конце испытания." }
  },
  error: { disabled: "Кошачьи испытания сейчас закрыты", formatInvalid: "Этот формат недоступен", metricInvalid: "Этот показатель рейтинга недоступен", runActive: "Испытание уже в очереди или выполняется", runNotFound: "Запись испытания не найдена", replayExpired: "Этот повтор больше не хранится", lineupChanged: "Снаряжение изменилось при фиксации состава. Повторите попытку", kittyNotFound: "Выбранный кот недоступен", lineupInvalid: "Недопустимый состав", alchemyEquipmentRestricted: "Алхимическое снаряжение превышает лимит Лиги", overviewFailed: "Не удалось загрузить испытания", runCreateFailed: "Не удалось начать испытание", historyFailed: "Не удалось загрузить историю", leaderboardFailed: "Не удалось загрузить рейтинг", replayFailed: "Не удалось загрузить повтор", replaySharedFailed: "Общий повтор недоступен", replayShareFailed: "Не удалось поделиться повтором" }
} as const
