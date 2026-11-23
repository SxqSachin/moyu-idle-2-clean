import { createApp, defineComponent, h, nextTick, type App } from "vue"
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest"

const mocks = vi.hoisted(() => {
  const wsHandlers = new Map<string, (payload: unknown) => void>()
  return {
    apiGet: vi.fn(),
    apiPost: vi.fn(),
    wsHandlers,
    bind: vi.fn((event: string, handler: (payload: unknown) => void) => {
      wsHandlers.set(event, handler)
      return () => wsHandlers.delete(event)
    }),
    gameStore: {
      token: "test-token",
      userId: 7,
      restoreToken: vi.fn()
    },
    chatStore: {
      syncBattleRoomChannel: vi.fn(),
      clearBattleRoomChannels: vi.fn()
    },
    realtimeStore: {
      setBattleRoomStatus: vi.fn()
    }
  }
})

vi.mock("@services/api", () => ({ apiGet: mocks.apiGet, apiPost: mocks.apiPost }))
vi.mock("@services/game-data-runtime", () => ({ getGameDataChunk: () => [] }))
vi.mock("@services/skill-definition", () => ({ getSkillDefinition: () => null }))
vi.mock("@services/ws", () => ({ bind: mocks.bind }))
vi.mock("@services/error-messages", () => ({ resolveErrorMessage: () => "request failed" }))
vi.mock("@stores/game", () => ({ useGameStore: () => mocks.gameStore }))
vi.mock("@stores/chat", () => ({ useChatStore: () => mocks.chatStore }))
vi.mock("@stores/realtime", () => ({ useRealtimeStore: () => mocks.realtimeStore }))
vi.mock("vue-i18n", () => ({ useI18n: () => ({ t: (key: string) => key }) }))
vi.mock("element-plus", () => ({
  ElMessage: {
    error: vi.fn(),
    info: vi.fn(),
    success: vi.fn(),
    warning: vi.fn()
  },
  ElNotification: vi.fn()
}))

import { useCombatPage } from "@/composables/game/combat/useCombatPage"

const mountedApps: App<Element>[] = []

const myRoom = {
  room: {
    id: 101,
    ownerId: 7,
    dungeonId: "test-dungeon",
    roomName: "Test room",
    autoReenterOnDefeat: true,
    disableEntryRound: false,
    isPublic: false,
    isFogMode: false,
    status: "playing",
    difficultyLevel: 0,
    completedRuns: 1,
    consecutiveFailures: 0,
    currentRunId: 501,
    seed: 11
  },
  members: [{ roomId: 101, userId: 7, position: "vanguard", isReady: true }]
}

const activeRun = {
  id: 501,
  roomId: 101,
  wave: 2,
  status: "playing",
  state: {
    roomId: 101,
    wave: 2,
    seq: 8,
    players: [{ unitId: "player-7", side: "player", hp: 100, maxHp: 100 }],
    enemies: [{ unitId: "enemy-1", side: "enemy", hp: 80, maxHp: 80 }],
    events: []
  }
}

beforeEach(() => {
  mocks.apiGet.mockReset()
  mocks.apiPost.mockReset()
  mocks.bind.mockClear()
  mocks.wsHandlers.clear()
  mocks.gameStore.restoreToken.mockClear()
  mocks.chatStore.syncBattleRoomChannel.mockClear()
  mocks.chatStore.clearBattleRoomChannels.mockClear()
  mocks.realtimeStore.setBattleRoomStatus.mockClear()
})

afterEach(() => {
  for (const app of mountedApps.splice(0)) app.unmount()
})

describe("combat room run loading", () => {
  it("reuses the selected room and playable snapshot when reopening the same battle tab", async () => {
    installInitialRoomResponses()
    const page = await mountCombatPage()
    await page.loadRooms()

    const selectedRoomBefore = page.selectedRoom.value
    const selectedRunBefore = page.selectedRun.value
    mocks.apiGet.mockClear()

    page.selectMyRoom()

    expect(page.selectedRoom.value).toBe(selectedRoomBefore)
    expect(page.selectedRun.value).toBe(selectedRunBefore)
    expect(page.runState.value?.seq).toBe(8)
    expect(page.runLoading.value).toBe(false)
    expect(mocks.apiGet).not.toHaveBeenCalled()
  })

  it("coalesces repeated websocket run refreshes for the same room", async () => {
    installInitialRoomResponses()
    const page = await mountCombatPage()
    await page.loadRooms()
    await Promise.resolve()

    let resolveRun!: (value: unknown) => void
    mocks.apiGet.mockReset()
    mocks.apiGet.mockImplementation((url: string) => {
      if (url === "/api/battle/rooms/101/run") {
        return new Promise((resolve) => { resolveRun = resolve })
      }
      return Promise.reject(new Error(`unexpected request: ${url}`))
    })

    const compiledHandler = mocks.wsHandlers.get("battle.run.compiled")
    expect(compiledHandler).toBeTypeOf("function")
    compiledHandler?.({ roomId: 101, runId: 501 })
    compiledHandler?.({ roomId: 101, runId: 501 })
    await Promise.resolve()

    expect(mocks.apiGet).toHaveBeenCalledTimes(1)
    expect(page.runState.value?.seq).toBe(8)

    resolveRun({ run: activeRun })
    await vi.waitFor(() => expect(page.runLoading.value).toBe(false))
    expect(page.runState.value?.seq).toBe(8)
  })

  it("starts a new request when a newer run arrives during an older run flight", async () => {
    installInitialRoomResponses()
    const page = await mountCombatPage()
    await page.loadRooms()
    await Promise.resolve()

    const pendingResolvers: Array<(value: unknown) => void> = []
    mocks.apiGet.mockReset()
    mocks.apiGet.mockImplementation((url: string) => {
      if (url !== "/api/battle/rooms/101/run") {
        return Promise.reject(new Error(`unexpected request: ${url}`))
      }
      return new Promise((resolve) => pendingResolvers.push(resolve))
    })

    mocks.wsHandlers.get("battle.run.compiled")?.({ roomId: 101, runId: 501 })
    await Promise.resolve()
    mocks.wsHandlers.get("battle.run.started")?.({ roomId: 101, runId: 502 })
    await Promise.resolve()

    expect(mocks.apiGet).toHaveBeenCalledTimes(2)
    expect(pendingResolvers).toHaveLength(2)

    pendingResolvers[0]({ run: activeRun })
    await Promise.resolve()
    expect(page.selectedRun.value?.id).toBe(501)

    pendingResolvers[1]({
      run: {
        ...activeRun,
        id: 502,
        wave: 3,
        state: { ...activeRun.state, wave: 3, seq: 1 }
      }
    })
    await vi.waitFor(() => expect(page.selectedRun.value?.id).toBe(502))
  })
})

function installInitialRoomResponses() {
  mocks.apiGet.mockImplementation((url: string) => {
    if (url === "/api/battle/rooms") return Promise.resolve({ items: [], mine: myRoom })
    if (url === "/api/battle/rooms/101/run") return Promise.resolve({ run: activeRun })
    return Promise.reject(new Error(`unexpected request: ${url}`))
  })
}

async function mountCombatPage() {
  let page!: ReturnType<typeof useCombatPage>
  const app = createApp(defineComponent({
    setup() {
      page = useCombatPage()
      return () => h("div")
    }
  }))
  const root = document.createElement("div")
  document.body.appendChild(root)
  app.mount(root)
  mountedApps.push(app)
  await nextTick()
  return page
}
