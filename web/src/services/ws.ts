import { parseWSCompressedBinaryJSON } from "@services/compression"
import type { Handler, WSEvent } from "@/types/ws"
import { useRealtimeStore } from "@stores/realtime"

const handlers = new Map<string, Set<Handler>>()
let socket: WebSocket | null = null
type ConnectWSOptions = {
  onOpen?: () => void
  onClose?: () => void
  onError?: () => void
}

const outboundQueue: WSEvent[] = []
const outboundQueueMaxSize = 128
const outboundQueueTTL = 15_000


function createRequestId(prefix: string): string {
  const random = Math.random().toString(36).slice(2)
  return `${prefix}-${Date.now()}-${random}`
}

async function normalizeIncomingData(data: unknown): Promise<string | ArrayBuffer> {
  if (typeof data === "string" || data instanceof ArrayBuffer) {
    return data
  }
  if (data instanceof Blob) {
    return data.arrayBuffer()
  }
  throw new Error("unsupported ws payload type")
}

async function handleIncoming(data: unknown) {
  const normalized = await normalizeIncomingData(data)
  const msg = typeof normalized === "string"
    ? (JSON.parse(normalized) as WSEvent)
    : parseWSCompressedBinaryJSON<WSEvent>(normalized)
  if (typeof msg.ts === "number" && msg.ts > 0) {
    const skew = Date.now() - msg.ts
    if (Math.abs(skew) < 30_000) {
      useRealtimeStore().setServerClockSkewMs(skew)
    }
  }
  const typeHandlers = handlers.get(msg.type)
  if (!typeHandlers || typeHandlers.size === 0) return
  if (msg.ok === false && msg.error) {
    for (const handler of typeHandlers) {
      handler(msg.error)
    }
    return
  }
  if (msg.data !== undefined) {
    for (const handler of typeHandlers) {
      handler(msg.data)
    }
    return
  }
  for (const handler of typeHandlers) {
    handler(msg.payload)
  }
}

function queueOutbound(event: WSEvent) {
  const now = Date.now()
  while (outboundQueue.length > 0) {
    const first = outboundQueue[0]
    if (now - first.ts <= outboundQueueTTL) break
    outboundQueue.shift()
  }
  if (outboundQueue.length >= outboundQueueMaxSize) {
    outboundQueue.shift()
  }
  outboundQueue.push(event)
}

function flushOutboundQueue() {
  if (!socket || socket.readyState !== WebSocket.OPEN || outboundQueue.length <= 0) return
  while (outboundQueue.length > 0) {
    const event = outboundQueue.shift()
    if (!event) continue
    socket.send(JSON.stringify(event))
  }
}

export function connectWS(url: string, options: ConnectWSOptions = {}) {
  if (socket && socket.readyState === WebSocket.OPEN) {
    socket.close()
  }
  socket = new WebSocket(url)
  socket.binaryType = "arraybuffer"
  socket.onopen = () => {
    options.onOpen?.()
    flushOutboundQueue()
  }
  socket.onclose = () => {
    options.onClose?.()
  }
  socket.onerror = () => {
    options.onError?.()
  }
  socket.onmessage = (event) => {
    void handleIncoming(event.data)
  }
}

export function bind(type: string, handler: Handler) {
  if (!handlers.has(type)) {
    handlers.set(type, new Set())
  }
  handlers.get(type)?.add(handler)
  return () => {
    handlers.get(type)?.delete(handler)
  }
}

export function send(type: string, payload: any = {}, options: { queueIfUnavailable?: boolean } = {}) {
  const requestId = createRequestId("ws")
  const event: WSEvent = { type, requestId, ts: Date.now(), payload }
  if (!socket || socket.readyState !== WebSocket.OPEN) {
    if (options.queueIfUnavailable !== false) {
      queueOutbound(event)
    }
    return requestId
  }
  socket.send(JSON.stringify(event))
  return requestId
}
