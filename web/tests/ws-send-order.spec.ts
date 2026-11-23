import { beforeEach, describe, expect, it, vi } from "vitest"

class MockWebSocket {
  static CONNECTING = 0
  static OPEN = 1
  static CLOSED = 3
  static instances: MockWebSocket[] = []

  readyState = MockWebSocket.CONNECTING
  binaryType = ""
  sent: string[] = []
  onopen: (() => void) | null = null
  onclose: (() => void) | null = null
  onerror: (() => void) | null = null
  onmessage: ((event: { data: unknown }) => void) | null = null

  constructor(public url: string) {
    MockWebSocket.instances.push(this)
  }

  send(raw: string) {
    this.sent.push(raw)
  }

  close() {
    this.readyState = MockWebSocket.CLOSED
    this.onclose?.()
  }

  open() {
    this.readyState = MockWebSocket.OPEN
    this.onopen?.()
  }
}

describe("ws send ordering", () => {
  beforeEach(() => {
    vi.resetModules()
    MockWebSocket.instances = []
    vi.stubGlobal("WebSocket", MockWebSocket)
  })

  it("sends auth.bind before flushing queued gameplay messages", async () => {
    const ws = await import("../src/services/ws")

    ws.send("action.offline.rewards.get")
    ws.connectWS("ws://game.test/ws", {
      onOpen: () => ws.send("auth.bind", { token: "token-1" })
    })

    const socket = MockWebSocket.instances[0]
    socket.open()

    const sentTypes = socket.sent.map((raw) => JSON.parse(raw).type)
    expect(sentTypes).toEqual(["auth.bind", "action.offline.rewards.get"])
  })

  it("returns the request id written into the websocket envelope", async () => {
    const ws = await import("../src/services/ws")
    ws.connectWS("ws://game.test/ws")
    const socket = MockWebSocket.instances[0]
    socket.open()

    const requestId = ws.send("action.offline.rewards.get", { protocolVersion: 2 })
    const sent = JSON.parse(socket.sent[0])

    expect(requestId).toBe(sent.requestId)
    expect(sent.payload).toEqual({ protocolVersion: 2 })
  })
})
