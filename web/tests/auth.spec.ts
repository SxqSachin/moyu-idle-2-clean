import { describe, expect, it } from "vitest"
import { resolveAuthError, validateAuthInput, validateRegisterInput } from "../src/services/auth"

describe("auth input validation", () => {
  it("rejects empty username", () => {
    expect(validateAuthInput("", "123456")).toBe("auth.error.usernameRequired")
  })

  it("rejects short username", () => {
    expect(validateAuthInput("a", "123456")).toBe("auth.error.usernameTooShort")
  })

  it("rejects short password", () => {
    expect(validateAuthInput("abc", "12345")).toBe("auth.error.passwordTooShort")
  })

  it("passes valid username and password", () => {
    expect(validateAuthInput("ab", "123456")).toBeNull()
  })
})

describe("register input validation", () => {
  it("accepts CJK usernames", () => {
    expect(validateRegisterInput("玩家テスト한글_01", "123456", "user@example.com")).toBeNull()
  })

  it("rejects invalid username symbols", () => {
    expect(validateRegisterInput("玩家-01", "123456", "user@example.com")).toBe("auth.error.usernameInvalidFormat")
  })

  it("rejects username over max length", () => {
    expect(validateRegisterInput("abcdefghijklmnopqrstuvwxy", "123456", "user@example.com")).toBe("auth.error.usernameTooLong")
  })
})

describe("auth error mapping", () => {
  it("maps structured backend code to i18n key", () => {
    const result = resolveAuthError({ code: "AUTH_INVALID_CREDENTIALS", message: "Invalid username or password" })
    expect(result.key).toBe("auth.error.invalidCredentials")
  })

  it("maps legacy backend errors for backward compatibility", () => {
    const result = resolveAuthError({ error: "username/password too short" })
    expect(result.key).toBe("auth.error.tooShort")
  })

  it("maps captcha backend errors", () => {
    expect(resolveAuthError({ code: "AUTH_CAPTCHA_REQUIRED" }).key).toBe("auth.error.captchaRequired")
    expect(resolveAuthError({ code: "AUTH_CAPTCHA_INVALID" }).key).toBe("auth.error.captchaInvalid")
    expect(resolveAuthError({ code: "AUTH_CAPTCHA_UNAVAILABLE" }).key).toBe("auth.error.captchaUnavailable")
  })

  it("maps recovery backend errors", () => {
    expect(resolveAuthError({ code: "AUTH_EMAIL_SEND_FAILED" }).key).toBe("auth.error.emailSendFailed")
    expect(resolveAuthError({ code: "AUTH_VERIFY_CODE_INVALID" }).key).toBe("auth.error.verifyCodeInvalid")
    expect(resolveAuthError({ code: "AUTH_RESET_TOKEN_INVALID" }).key).toBe("auth.error.resetTokenInvalid")
  })

  it("maps blocked email domains to a specific player message", () => {
    expect(resolveAuthError({ code: "AUTH_EMAIL_DOMAIN_BLOCKED" }).key).toBe("auth.error.emailDomainBlocked")
  })
})
