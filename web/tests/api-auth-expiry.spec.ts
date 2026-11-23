import { describe, expect, it } from "vitest"
import { APIClientError, isAuthExpiredError } from "../src/services/api"

describe("auth expiry classification", () => {
  it("preserves the session for transient entry failures", () => {
    expect(isAuthExpiredError(new APIClientError("busy", {
      code: "AUTH_BACKEND_BUSY",
      status: 503
    }))).toBe(false)
    expect(isAuthExpiredError(new APIClientError("rate limited", {
      code: "RATE_LIMITED",
      status: 429
    }))).toBe(false)
    expect(isAuthExpiredError(new TypeError("network error"))).toBe(false)
  })

  it("expires the session only for explicit auth failures", () => {
    expect(isAuthExpiredError(new APIClientError("invalid", {
      code: "AUTH_INVALID_TOKEN",
      status: 401
    }))).toBe(true)
    expect(isAuthExpiredError({ code: "WS_SESSION_REVOKED" })).toBe(true)
    expect(isAuthExpiredError({ status: 401 })).toBe(true)
  })
})
