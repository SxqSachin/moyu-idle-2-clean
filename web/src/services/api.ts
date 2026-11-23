import { parseCompressedJSON, type CompressedEnvelope } from "@services/compression"

export const AUTH_EXPIRED_EVENT = "app:auth-expired"

export function authHeaders(token: string): Record<string, string> {
  return {
    Authorization: `Bearer ${token}`
  }
}

type APIEnvelope<T> = {
  ok: boolean
  ts: number
  data?: T
  compressed?: CompressedEnvelope
  error?: { code?: string; message?: string; debug?: string; instanceState?: string }
}

export class APIClientError extends Error {
  code?: string
  status?: number
  debug?: string
  instanceState?: string
  data?: unknown

  constructor(message: string, options?: { code?: string; status?: number; debug?: string; instanceState?: string; data?: unknown }) {
    super(message)
    this.name = "APIClientError"
    this.code = options?.code
    this.status = options?.status
    this.debug = options?.debug
    this.instanceState = options?.instanceState
    this.data = options?.data
  }
}

export function isAuthExpiredError(payload: unknown): boolean {
  if (payload instanceof APIClientError) {
    return isAuthExpired(payload.code, payload.status)
  }
  if (!payload || typeof payload !== "object") return false
  const value = payload as { code?: unknown; status?: unknown }
  const code = typeof value.code === "string" ? value.code : undefined
  const status = Number(value.status || 0) || undefined
  return isAuthExpired(code, status)
}

const authExpiredCodes = new Set([
  "AUTH_INVALID_TOKEN",
  "AUTH_SESSION_REVOKED",
  "WS_UNAUTHORIZED",
  "WS_INVALID_TOKEN",
  "WS_SESSION_REVOKED"
])

let lastAuthExpiredEventAt = 0

function isAuthExpired(code: string | undefined, status: number | undefined): boolean {
  return (code ? authExpiredCodes.has(code) : false) || status === 401
}

function emitAuthExpired(code: string | undefined, status: number | undefined) {
  if (!isAuthExpired(code, status)) return
  if (typeof window === "undefined") return
  const now = Date.now()
  if (now - lastAuthExpiredEventAt < 1_000) return
  lastAuthExpiredEventAt = now
  window.dispatchEvent(new CustomEvent(AUTH_EXPIRED_EVENT, { detail: { code, status } }))
}

async function parseAPIResponse<T>(res: Response): Promise<T> {
  const raw = (await res.json()) as APIEnvelope<T> | T
  if (typeof raw === "object" && raw !== null && "ok" in raw) {
    const envelope = raw as APIEnvelope<T>
    if (envelope.ok && envelope.data !== undefined) {
      return envelope.data
    }
    if (envelope.ok && envelope.compressed) {
      return parseCompressedJSON<T>(envelope.compressed)
    }
    const message = envelope.error?.message || "request failed"
    const code = envelope.error?.code
    emitAuthExpired(code, res.status)
    throw new APIClientError(message, {
      code,
      status: res.status,
      debug: envelope.error?.debug,
      instanceState: envelope.error?.instanceState,
      data: envelope.data
    })
  }
  return raw as T
}

async function throwAPIErrorFromResponse(res: Response): Promise<never> {
  const raw = await res.text()
  if (!raw) {
    throw new APIClientError("request failed", { status: res.status })
  }
  try {
    const parsed = JSON.parse(raw) as APIEnvelope<unknown> | { data?: unknown; error?: { code?: string; message?: string; debug?: string; instanceState?: string } }
    if (parsed && typeof parsed === "object" && "error" in parsed && parsed.error) {
      const err = new APIClientError(parsed.error.message || "request failed", {
        code: parsed.error.code,
        status: res.status,
        debug: parsed.error.debug,
        instanceState: parsed.error.instanceState,
        data: parsed.data
      })
      emitAuthExpired(err.code, err.status)
      throw err
    }
  } catch (err) {
    if (err instanceof APIClientError) {
      throw err
    }
    // fall through and keep raw text
  }
  emitAuthExpired(undefined, res.status)
  throw new APIClientError(raw, { status: res.status })
}

async function apiRequest<T>(url: string, init: RequestInit & { token?: string } = {}): Promise<T> {
  const { token, headers, ...rest } = init
  const res = await fetch(url, {
    ...rest,
    headers: {
      ...(token ? authHeaders(token) : {}),
      ...(headers || {})
    }
  })
  if (!res.ok) {
    return throwAPIErrorFromResponse(res)
  }
  return parseAPIResponse<T>(res)
}

export async function apiGet<T>(url: string, token: string): Promise<T> {
  return apiRequest<T>(url, {
    token
  })
}

export async function apiPost<T>(url: string, token: string, body?: unknown, extraHeaders?: Record<string, string>): Promise<T> {
  const idempotencyKey = extraHeaders?.["X-Idempotency-Key"] || createIdempotencyKey("api")
  return apiRequest<T>(url, {
    method: "POST",
    token,
    headers: {
      "Content-Type": "application/json",
      "X-Idempotency-Key": idempotencyKey,
      ...(extraHeaders || {})
    },
    body: body ? JSON.stringify(body) : undefined
  })
}

export async function apiPut<T>(url: string, token: string, body?: unknown): Promise<T> {
  return apiRequest<T>(url, {
    method: "PUT",
    token,
    headers: { "Content-Type": "application/json" },
    body: body === undefined ? undefined : JSON.stringify(body)
  })
}

export async function apiDelete<T = void>(url: string, token: string): Promise<T> {
  return apiRequest<T>(url, { method: "DELETE", token })
}

export async function apiPostPublic<T>(url: string, body?: unknown, extraHeaders?: Record<string, string>): Promise<T> {
  const idempotencyKey = extraHeaders?.["X-Idempotency-Key"] || createIdempotencyKey("api")
  return apiRequest<T>(url, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
      "X-Idempotency-Key": idempotencyKey,
      ...(extraHeaders || {})
    },
    body: body ? JSON.stringify(body) : undefined
  })
}

export function createIdempotencyKey(prefix: string): string {
  const random = Math.random().toString(36).slice(2)
  return `${prefix}-${Date.now()}-${random}`
}
