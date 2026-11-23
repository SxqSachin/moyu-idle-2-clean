// Browser-only protocol fixture. No game request reaches a real backend.
export function installOfflineRecoveryFixture() {
  localStorage.setItem("locale", "zh-CN")
  localStorage.setItem("token", "isolated-offline-recovery-test")
  localStorage.setItem("userId", "42")
  const settings = { language: "zh-CN", theme: "light", menuLayout: "icon-rail", simpleMode: true, emailVerified: true }
  localStorage.setItem("player-settings:42", JSON.stringify(settings))
  const state = { stats: {}, attributes: {}, buildings: { cattery: 2 }, kitties: [] }
  const originalFetch = window.fetch.bind(window)
  window.__inventoryRequests = 0
  window.fetch = async (input, options = {}) => {
    const url = new URL(typeof input === "string" ? input : input.url, location.href)
    if (!url.pathname.startsWith("/api/")) return originalFetch(input, options)
    if (url.pathname === "/api/inventory/instances" && url.searchParams.get("fresh") === "1" && localStorage.getItem("offline-fixture-mode").includes("inventory")) {
      window.__inventoryRequests++
      await new Promise((resolve) => { window.__releaseInventory = resolve })
      const items = Array.from({ length: 50000 }, (_, index) => ({
        instanceId: index + 1, templateItemId: "iron_bracers", quantity: 1, state: "inventory",
        enhanceLevel: 0, durability: 100, maxDurability: 100, affixes: []
      }))
      return new Response(JSON.stringify({ ok: true, ts: Date.now(), data: { items } }), { status: 200, headers: { "Content-Type": "application/json" } })
    }
    let data = { items: [], hasMore: false }
    if (url.pathname === "/api/bootstrap/init") data = {
      playerStats: state, inventory: { items: [] }, chatConfig: { channels: [], defaultChannel: "world" },
      chatHistory: { items: [] }, playerSettings: { username: "Recovery QA", ...settings }, tutorialProgress: {},
      playerPresence: { status: "online" }, systemVersion: {}, recycleConfig: { goldCoefficient: .5, affixMismatchExtraCoefficient: .6 }
    }
    if (url.pathname === "/api/player/stats") data = state
    return new Response(JSON.stringify({ ok: true, ts: Date.now(), data }), { status: 200, headers: { "Content-Type": "application/json" } })
  }
  window.__offlineRequests = []
  let revision = 1
  const WS = window.WebSocket
  window.WebSocket = class extends EventTarget {
    static OPEN = 1
    static CLOSED = 3
    constructor(url, protocol) {
      super()
      if (!new URL(url, location.href).pathname.startsWith("/ws")) return new WS(url, protocol)
      this.readyState = 1
      window.__emitOfflineReady = () => this.reply("action.offline.recovery.ready", {})
      setTimeout(() => { const event = new Event("open"); this.dispatchEvent(event); this.onopen?.(event) }, 50)
    }
    send(raw) {
      const request = JSON.parse(raw)
      window.__offlineRequests.push(request)
      const replies = {
        "auth.bind": ["auth.bound", {}], "action.queue.get": ["action.queue.updated", { items: [] }],
        "game.state.get": ["game.state.patch", state]
      }
      if (request.type === "action.offline.rewards.get") {
        let mode = localStorage.getItem("offline-fixture-mode") || "progress"
        if (mode === "timeout") return
        if (mode.startsWith("combined")) {
          const acked = new Set(JSON.parse(localStorage.getItem("offline-fixture-acknowledged") || "[]"))
          const all = mode === "combined-new" ? ["9001", "9002", "9003"] : ["9001", "9002"]
          const ack = request.payload?.ackReportKey
            ? all.filter((id) => BigInt(id) <= BigInt(request.payload.ackTaskId)).map((taskId) => ({ taskId, revision: request.payload.ackRevision }))
            : request.payload?.ackTasks || (request.payload?.ackTaskId ? [{ taskId: request.payload.ackTaskId, revision: request.payload.ackRevision }] : [])
          if (ack.length && mode === "combined-lost-ack") return
          if (mode !== "combined-replay") for (const task of ack) if (task.revision === 3) acked.add(task.taskId)
          localStorage.setItem("offline-fixture-acknowledged", JSON.stringify([...acked]))
          const pending = all.filter((id) => !acked.has(id))
          const tasks = request.payload.reportPolicy === "combined" ? pending : pending.slice(0, 1)
          const complete = mode !== "combined-progress"
          this.reply("action.offline.rewards.result", {
            protocolVersion: 3, requestId: request.requestId, cumulative: true,
            taskId: tasks.at(-1) || "", revision: 3,
            ...(tasks.length && request.payload.reportPolicy === "combined" ? { reportKey: tasks.join("").padEnd(64, "0"), reportTasks: tasks.map((taskId) => ({ taskId, revision: 3 })) } : {}),
            status: complete ? "complete" : "progress", hasMoreDue: !complete,
            completedSteps: tasks.length * 1500, itemRewards: tasks.length ? { wood: tasks.length * 1500 } : {},
            itemCosts: {}, attrExp: {}, actorSummaries: [], offlineDurationSeconds: tasks.length * 3600,
            combatItemRewards: mode === "combined-pending" ? {} : { wood: 7 }, combatPendingRuns: mode === "combined-pending" ? 1 : 0,
            snapshotSynced: complete, retryAfterMS: 750, targetAtMS: 1788753600000,
            ...(mode === "combined-inventory" && tasks.length ? { inventorySync: { version: 8, epoch: "inventory-fixture" } } : {})
          })
          return
        }
        if (mode.startsWith("stalled") && request.payload?.retryTaskId === "9001" && request.payload?.retryRevision === revision) {
          revision++
          if (mode !== "stalled-repeat") {
            localStorage.setItem("offline-fixture-mode", "late")
            if (mode === "stalled-timeout") return
            mode = "late"
          }
        }
        const stalled = mode.startsWith("stalled")
        const complete = mode === "complete" || mode === "late" || mode === "inventory"
        const ack = request.payload?.ackTaskId
        const data = {
          protocolVersion: 3, requestId: request.requestId, cumulative: true, taskId: ack ? "" : "9001",
          revision: complete ? 3 : revision, status: complete || ack ? "complete" : stalled ? "stalled" : "progress",
          hasMoreDue: !complete && !ack && !stalled, completedSteps: ack ? 0 : complete ? 3000 : 1000,
          startedAtMS: 1788724800000, targetAtMS: 1788753600000, offlineDurationSeconds: 28800,
          itemRewards: complete && !ack ? { wood: 3000 } : {}, itemCosts: {}, attrExp: {}, actorSummaries: [],
          combatItemRewards: mode === "late" ? { wood: 7 } : {}, combatPendingRuns: mode === "complete" ? 1 : 0,
          snapshotSynced: !stalled, projectionDeferred: stalled, retryAfterMS: 750,
          ...(mode === "inventory" && !ack ? { inventorySync: { version: 8, epoch: "inventory-fixture" } } : {}),
          recoveryErrorCode: stalled ? "OFFLINE_RECOVERY_RETRY" : ""
        }
        replies[request.type] = ["action.offline.rewards.result", data]
      }
      const reply = replies[request.type]
      if (reply) this.reply(reply[0], reply[1])
    }
    reply(type, data) {
      setTimeout(() => {
        const event = new MessageEvent("message", { data: JSON.stringify({ type, data, ts: Date.now() }) })
        this.dispatchEvent(event); this.onmessage?.(event)
      }, 5)
    }
    close() { this.readyState = 3 }
  }
}
