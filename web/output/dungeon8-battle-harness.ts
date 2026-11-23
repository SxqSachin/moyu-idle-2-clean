import { createApp, defineComponent, h, ref } from "vue"
import { createPinia } from "pinia"
import i18n, { setAppLocale } from "@/i18n"
import "@/style.css"
import BattleView from "@/views/game/combat/views/BattleView.vue"
import { provideCombatViewContext } from "@/views/game/combat/useCombatViewContext"
import { loadAllGameData } from "@/services/game-data-runtime"

await setAppLocale("zh-CN")
await loadAllGameData()

const players = [
  {
    unitId: "player-1",
    side: "player",
    unitType: "player",
    name: "星弦",
    position: "vanguard",
    hp: 7240,
    maxHp: 8200,
    mp: 118,
    maxMp: 220,
    alive: true,
    skillIds: ["hushing_incantation_page", "cooldown_correction", "basic_attack"],
    skillCooldowns: { hushing_incantation_page: 3, cooldown_correction: 5, basic_attack: 0 },
    buffs: [{ buffId: "silence", remainTurns: 1, payload: {} }]
  },
  {
    unitId: "kitty-1",
    side: "player",
    unitType: "kitty",
    name: "铜铃",
    position: "vanguard",
    hp: 6020,
    maxHp: 6800,
    mp: 96,
    maxMp: 180,
    alive: true,
    skillIds: ["forbidden_trace", "basic_attack"],
    skillCooldowns: { forbidden_trace: 2, basic_attack: 0 },
    buffs: []
  },
  {
    unitId: "player-2",
    side: "player",
    unitType: "player",
    name: "墨砚",
    position: "midguard",
    hp: 5580,
    maxHp: 7100,
    mp: 202,
    maxMp: 260,
    alive: true,
    skillIds: ["arcane_missile", "adv_heal", "basic_attack"],
    skillCooldowns: { arcane_missile: 2, adv_heal: 4, basic_attack: 0 },
    buffs: []
  },
  {
    unitId: "kitty-2",
    side: "player",
    unitType: "kitty",
    name: "纸鸢",
    position: "rearguard",
    hp: 4910,
    maxHp: 5900,
    mp: 155,
    maxMp: 210,
    alive: true,
    skillIds: ["blessing", "basic_attack"],
    skillCooldowns: { blessing: 1, basic_attack: 0 },
    buffs: []
  }
]

const enemies = [
  { unitId: "enemy-1", side: "enemy", unitType: "enemy", nameKey: "enemy.brass_book_guard_001.name", position: "vanguard", hp: 5200, maxHp: 7200, mp: 0, maxMp: 0, alive: true, buffs: [{ buffId: "brass_book_guard_bulwark_buff", remainTurns: 2, payload: {} }] },
  { unitId: "enemy-2", side: "enemy", unitType: "enemy", nameKey: "enemy.living_bookmark_001.name", position: "midguard", hp: 6100, maxHp: 7000, mp: 0, maxMp: 0, alive: true, buffs: [] },
  { unitId: "enemy-3", side: "enemy", unitType: "enemy", nameKey: "enemy.watcher_owl_001.name", position: "rearguard", hp: 6800, maxHp: 7200, mp: 0, maxMp: 0, alive: true, buffs: [] }
]

const worlds = ref([{
  id: "forbidden_library_001",
  name: "禁魔图书馆",
  entryDurationSeconds: 15,
  battlefieldEffects: [{
    id: "forbidden_library_audit",
    nameKey: "battlefield_effect.forbidden_library_audit.name",
    descKey: "battlefield_effect.forbidden_library_audit.desc",
    icon: "game-icons:spell-book",
    initial: 1,
    displayMode: "progress"
  }]
}])

const selectedRoom = ref({
  room: {
    id: 8001,
    dungeonId: "forbidden_library_001",
    roomName: "禁魔图书馆",
    status: "playing",
    difficultyLevel: 1,
    currentRunId: 8002
  },
  members: []
})

const runState = ref({
  seq: 44,
  wave: 13,
  players,
  enemies,
  order: ["player-1", "enemy-1", "kitty-1", "enemy-2", "player-2", "enemy-3", "kitty-2"],
  battlefieldEffects: [{ id: "forbidden_library_audit", remaining: 1, progress: 3, goal: 4, windows: {} }]
})

const runEvents = ref([{
  seq: 44,
  type: "battle.field.trigger",
  ts: Date.now(),
  data: {
    effectId: "forbidden_library_audit",
    targets: [
      { unitId: "player-1", outcomeId: "silence", effectKind: "status", statusId: "silence", durationTurns: 1 },
      { unitId: "player-2", outcomeId: "sealed", effectKind: "adjust_skill_cooldown", cooldownChanges: [{ skillId: "arcane_missile", before: 1, after: 2 }] }
    ]
  }
}])

const Host = defineComponent({
  setup() {
    provideCombatViewContext({
      worlds,
      selectedRoom,
      selectedRun: ref({ id: 8002, wave: 13 }),
      runState,
      runEvents,
      roomsLoading: ref(false),
      runLoading: ref(false),
      roomStatusLabel: (status: string) => status,
      runStatusLabel: (status: string) => status,
      positionLabel: (position: string) => position,
      selectRoom: async () => {},
      interBattleWait: ref(null),
      activeTab: ref("battle"),
      setActiveTab: () => {}
    } as never)
    return () => h("main", { class: "mx-auto max-w-[1180px] p-3" }, h(BattleView))
  }
})

createApp(Host).use(createPinia()).use(i18n).mount("#app")
