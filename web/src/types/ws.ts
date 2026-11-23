export type Handler = (payload: any) => void

export type WSEvent = {
  type: string
  requestId?: string
  ts: number
  ok?: boolean
  data?: any
  error?: { code?: string; message?: string; debug?: string }
  payload?: any
}

export type BattleTimelineEvent = {
  seq: number
  offsetMs: number
  lane: "turn" | "hook" | "system" | string
  phase: "pre" | "main" | "post" | string
  parentSeq?: number
  groupId?: string
  orderKey: string
  actorId?: string
  actorUserId?: number
  hookId?: string
  actionDurationMs: number
  payload: Record<string, unknown>
}

export type BattleEventBatchPayload = {
  m: number
  r: number
  f: number
  t: number
  s: number
  e: number
  v: Array<{
    q: number
    o: number
    e?: string
    d?: Record<string, unknown>
  }>
  n?: {
    w?: number
    c?: number
    q?: number
    p?: Record<string, unknown>[]
    e?: Record<string, unknown>[]
  }
}

export type BattlePlayCheckpoint = {
  runId: number
  lastSeq: number
  lastOffsetMs: number
  updatedAtMs: number
}
