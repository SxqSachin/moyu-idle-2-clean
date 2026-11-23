import { afterEach, beforeAll, describe, expect, it } from "vitest"
import { createApp, defineComponent, h, nextTick, ref, type App, type Component, type Ref } from "vue"
import i18n, { setAppLocale } from "@/i18n"
import { provideCombatViewContext, type BattleTabName, type CombatViewContext } from "@/views/game/combat/useCombatViewContext"
import { buildBattleSpecializationSourceLabel } from "@/services/battle-specialization-source-display"

type MountResult = {
  root: HTMLDivElement
  app: App<Element>
}

const EmptyStub = defineComponent({
  name: "ElEmpty",
  props: {
    description: {
      type: String,
      default: ""
    }
  },
  setup(props) {
    return () => h("div", { class: "el-empty" }, props.description)
  }
})

const TagStub = defineComponent({
  name: "ElTag",
  setup(_, { slots }) {
    return () => h("span", { class: "el-tag" }, slots.default?.())
  }
})

const ProgressStub = defineComponent({
  name: "ElProgress",
  setup() {
    return () => h("div", { class: "el-progress" })
  }
})

const DividerStub = defineComponent({
  name: "ElDivider",
  setup() {
    return () => h("hr", { class: "el-divider" })
  }
})

const ButtonStub = defineComponent({
  name: "ElButton",
  setup(_, { slots }) {
    return () => h("button", { type: "button" }, slots.default?.())
  }
})

const TooltipStub = defineComponent({
  name: "ElTooltip",
  props: {
    content: { type: String, default: "" },
    placement: { type: String, default: "top" }
  },
  setup(_, { slots }) {
    return () => h("span", { class: "el-tooltip" }, slots.default?.())
  }
})

const TabsStub = defineComponent({
  name: "ElTabs",
  props: {
    modelValue: { type: String, default: "" }
  },
  setup(props, { slots }) {
    return () => h("div", { class: "el-tabs", "data-active-tab": props.modelValue }, slots.default?.())
  }
})

const TabPaneStub = defineComponent({
  name: "ElTabPane",
  props: {
    label: { type: String, default: "" },
    name: { type: String, default: "" }
  },
  setup(props, { slots }) {
    return () => h("section", { class: "el-tab-pane", "data-tab-name": props.name }, [
      h("span", { class: "el-tab-pane__label" }, props.label),
      slots.default?.()
    ])
  }
})

const mountedApps: MountResult[] = []
let BattleView: Component

beforeAll(async () => {
  if (!window.matchMedia) {
    Object.defineProperty(window, "matchMedia", {
      writable: true,
      value: () => ({
        matches: false,
        media: "",
        onchange: null,
        addListener: () => {},
        removeListener: () => {},
        addEventListener: () => {},
        removeEventListener: () => {},
        dispatchEvent: () => false
      })
    })
  }
  await setAppLocale("zh-CN")
  BattleView = (await import("@/views/game/combat/views/BattleView.vue")).default
})

afterEach(() => {
  for (const item of mountedApps.splice(0)) {
    item.app.unmount()
    item.root.remove()
  }
})

describe("combat battle loading view", () => {
  it("separates the animation and text battle views into dedicated tabs", async () => {
    const mounted = await mountBattleView({
      selectedRoom: createRoomDetail("idle")
    })

    expect(mounted.root.querySelector(".el-tabs")?.getAttribute("data-active-tab")).toBe("animation")
    expect(mounted.root.querySelector('[data-tab-name="animation"]')?.textContent).toContain("动画")
    expect(mounted.root.querySelector('[data-tab-name="text"]')?.textContent).toContain("文字")
    expect(mounted.root.querySelector('[data-tab-name="text"] .battle-text-log')).not.toBeNull()
  })

  it("shows battle loading while the run request is pending", async () => {
    const mounted = await mountBattleView({
      selectedRoom: createRoomDetail("idle"),
      runLoading: true
    })

    expect(mounted.root.textContent).toContain("战斗加载中，请稍后")
    expect(mounted.root.textContent).not.toContain("当前房间暂无进行中的战斗")
    expect(mounted.root.querySelector(".el-empty")).toBeNull()
  })

  it("shows battle loading when a playing room has no run state yet", async () => {
    const mounted = await mountBattleView({
      selectedRoom: createRoomDetail("playing")
    })

    expect(mounted.root.textContent).toContain("战斗加载中，请稍后")
    expect(mounted.root.textContent).not.toContain("当前房间暂无进行中的战斗")
    expect(mounted.root.querySelector(".el-empty")).toBeNull()
  })

  it("shows no-run placeholder after loading finishes without an active battle", async () => {
    const mounted = await mountBattleView({
      selectedRoom: createRoomDetail("idle")
    })

    expect(mounted.root.textContent).toContain("当前房间暂无进行中的战斗")
    expect(mounted.root.textContent).not.toContain("战斗加载中，请稍后")
    expect(mounted.root.querySelector(".el-empty")).not.toBeNull()
  })

  it("keeps a visible preparation state until a compiled run becomes playable", async () => {
    const mounted = await mountBattleView({
      selectedRoom: createRoomDetail("playing"),
      selectedRun: {
        id: 2002,
        roomId: 1001,
        wave: 2,
        status: "compiled",
        state: {}
      },
      runState: {
        wave: 2,
        seq: 0,
        players: [{ unitId: "player-1", side: "player", hp: 100, maxHp: 100 }],
        enemies: [{ unitId: "enemy-1", side: "enemy", hp: 50, maxHp: 50 }]
      }
    })

    expect(mounted.root.querySelector(".battle-intermission-loading")).not.toBeNull()
    expect(mounted.root.textContent).toContain("战斗加载中，请稍后")
    expect(mounted.root.querySelector(".battle-intermission-progress")).toBeNull()
    expect(mounted.root.textContent).not.toContain("请等待…")
  })

  it("shows the real intermission countdown before preparation begins", async () => {
    const mounted = await mountBattleView({
      selectedRoom: createRoomDetail("playing"),
      runState: createRenderableRunState(),
      interBattleWait: {
        finishedAtMs: Date.now(),
        intervalMs: 3_000,
        finishedRunId: 2001,
        pendingRunId: 0
      }
    })

    expect(mounted.root.textContent).toContain("等待下一场战斗")
    expect(mounted.root.querySelector(".battle-intermission-progress")).not.toBeNull()
    expect(mounted.root.querySelector(".battle-intermission-loading")).toBeNull()
  })

  it("replaces an expired intermission with preparation feedback", async () => {
    const mounted = await mountBattleView({
      selectedRoom: createRoomDetail("playing"),
      runState: createRenderableRunState(),
      interBattleWait: {
        finishedAtMs: Date.now() - 4_000,
        intervalMs: 3_000,
        finishedRunId: 2001,
        pendingRunId: 0
      }
    })

    expect(mounted.root.textContent).toContain("战斗加载中，请稍后")
    expect(mounted.root.querySelector(".battle-intermission-loading")).not.toBeNull()
    expect(mounted.root.querySelector(".battle-intermission-progress")).toBeNull()
  })

  it("renders the authoritative battlefield countdown and exposure window", async () => {
    const mounted = await mountBattleView({
      selectedRoom: createRoomDetail("playing"),
      worlds: [{
        id: "test_dungeon",
        battlefieldEffects: [{
          id: "meteor_corridor_starburst",
          nameKey: "battlefield_effect.meteor_corridor_starburst.name",
          descKey: "battlefield_effect.meteor_corridor_starburst.desc",
          icon: "game-icons:falling-star",
          initial: 8,
          displayMode: "countdown"
        }]
      }],
      runState: {
        wave: 12,
        seq: 8,
        players: [],
        enemies: [],
        battlefieldEffects: [{
          id: "meteor_corridor_starburst",
          remaining: 3,
          progress: 1,
          goal: 2,
          windows: { timekeeper_exposed: 2 }
        }]
      }
    })

    const strip = mounted.root.querySelector(".battlefield-effect-strip")
    expect(strip).not.toBeNull()
    expect(strip?.textContent).toContain("流星降临")
    expect(strip?.textContent).toContain("3 → 0")
    expect(strip?.textContent).toContain("本轮 1/2")
    expect(strip?.textContent).toContain("司辰破绽 2 → 0")
  })

  it("renders audit effects as progress without a fake countdown warning", async () => {
    const mounted = await mountBattleView({
      selectedRoom: createRoomDetail("playing"),
      worlds: [{
        id: "test_dungeon",
        battlefieldEffects: [{
          id: "forbidden_library_audit",
          nameKey: "battlefield_effect.forbidden_library_audit.name",
          descKey: "battlefield_effect.forbidden_library_audit.desc",
          icon: "game-icons:spell-book",
          initial: 1,
          displayMode: "progress"
        }]
      }],
      runState: {
        wave: 2,
        seq: 10,
        players: [],
        enemies: [],
        battlefieldEffects: [{
          id: "forbidden_library_audit",
          remaining: 1,
          progress: 3,
          goal: 4,
          windows: {}
        }]
      }
    })

    const row = mounted.root.querySelector(".battlefield-effect-row")
    expect(row?.textContent).toContain("禁魔审查")
    expect(row?.textContent).toContain("当前 3/4")
    expect(row?.textContent).not.toContain("1 → 0")
    expect(row?.classList.contains("battlefield-effect-row--warning")).toBe(false)
  })

  it("labels specialization battle contributions with the localized node name", () => {
    const label = buildBattleSpecializationSourceLabel({
      runtimeDamageAmpSources: [{
        kind: "specialization",
        nodeId: "strength_root_break_stance",
        sourceNameKey: "specialization.node.strength_root_break_stance.name"
      }]
    }, i18n.global.t)

    expect(label).toBe("专精 · 蓄势重击")
    expect(label).not.toContain("装备")
  })

  it("labels every specialization that mutated the same battle effect", () => {
    const label = buildBattleSpecializationSourceLabel({
      sourceKind: "specialization",
      sourceId: "confluence_strength_intelligence_mana_edge",
      sourceNameKey: "specialization.node.confluence_strength_intelligence_mana_edge.name",
      mutationSources: [
        {
          sourceKind: "specialization",
          sourceId: "strength_root_break_stance",
          sourceNameKey: "specialization.node.strength_root_break_stance.name"
        },
        {
          sourceKind: "specialization",
          sourceId: "confluence_strength_intelligence_mana_edge",
          sourceNameKey: "specialization.node.confluence_strength_intelligence_mana_edge.name"
        }
      ]
    }, i18n.global.t)

    expect(label).toBe("专精 · 魔武转换 / 蓄势重击")
  })
})

async function mountBattleView(options: {
  selectedRoom?: unknown
  selectedRun?: unknown
  runState?: unknown
  interBattleWait?: unknown
  roomsLoading?: boolean
  runLoading?: boolean
  worlds?: unknown[]
}): Promise<MountResult> {
  const context = createCombatContext(options)
  const Host = defineComponent({
    setup() {
      provideCombatViewContext(context)
      return () => h(BattleView)
    }
  })

  const root = document.createElement("div")
  document.body.appendChild(root)
  const app = createApp(Host)
  app.use(i18n)
  app.component("el-empty", EmptyStub)
  app.component("ElEmpty", EmptyStub)
  app.component("el-tag", TagStub)
  app.component("ElTag", TagStub)
  app.component("el-progress", ProgressStub)
  app.component("ElProgress", ProgressStub)
  app.component("el-divider", DividerStub)
  app.component("ElDivider", DividerStub)
  app.component("el-button", ButtonStub)
  app.component("ElButton", ButtonStub)
  app.component("el-tooltip", TooltipStub)
  app.component("ElTooltip", TooltipStub)
  app.component("el-tabs", TabsStub)
  app.component("ElTabs", TabsStub)
  app.component("el-tab-pane", TabPaneStub)
  app.component("ElTabPane", TabPaneStub)
  app.mount(root)
  await nextTick()
  await nextTick()
  const mounted = { root, app }
  mountedApps.push(mounted)
  return mounted
}

function createCombatContext(options: {
  selectedRoom?: unknown
  selectedRun?: unknown
  runState?: unknown
  interBattleWait?: unknown
  roomsLoading?: boolean
  runLoading?: boolean
  worlds?: unknown[]
}): CombatViewContext {
  return {
    worlds: ref(options.worlds ?? []),
    selectedRoom: ref(options.selectedRoom ?? null),
    selectedRun: ref(options.selectedRun ?? null),
    runState: ref(options.runState ?? null),
    runEvents: ref([]),
    roomsLoading: ref(Boolean(options.roomsLoading)),
    runLoading: ref(Boolean(options.runLoading)),
    roomStatusLabel: (status: string) => status,
    runStatusLabel: (status: string) => status,
    positionLabel: (position: string) => position,
    selectRoom: () => {},
    interBattleWait: ref(options.interBattleWait ?? null),
    activeTab: ref("battle") as Ref<BattleTabName>,
    setActiveTab: () => {}
  } as unknown as CombatViewContext
}

function createRenderableRunState() {
  return {
    wave: 2,
    seq: 8,
    players: [{ unitId: "player-1", side: "player", hp: 100, maxHp: 100 }],
    enemies: [{ unitId: "enemy-1", side: "enemy", hp: 50, maxHp: 50 }]
  }
}

function createRoomDetail(status: "idle" | "playing") {
  return {
    room: {
      id: 1001,
      ownerId: 1,
      dungeonId: "test_dungeon",
      roomName: "测试房间",
      autoReenterOnDefeat: true,
      disableEntryRound: false,
      isPublic: false,
      isFogMode: false,
      status,
      difficultyLevel: 0,
      completedRuns: 0,
      consecutiveFailures: 0,
      currentRunId: status === "playing" ? 2001 : null,
      seed: 1
    },
    members: []
  }
}
