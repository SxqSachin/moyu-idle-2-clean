import type { AuthErrorPayload } from "@/types/auth"
export type { AuthMode } from "@/types/auth"

export const MIN_USERNAME_LENGTH = 2
export const MAX_USERNAME_LENGTH = 24
export const MIN_PASSWORD_LENGTH = 6
export const MAX_EMAIL_LENGTH = 128

const emailPattern = /^[^\s@]+@[^\s@]+\.[^\s@]+$/
const usernamePattern = /^[A-Za-z0-9_\u3400-\u9FFF\u3040-\u309F\u30A0-\u30FF\uAC00-\uD7AF]+$/

const errorCodeToI18nKey: Record<string, string> = {
  AUTH_INVALID_PAYLOAD: "auth.error.invalidPayload",
  AUTH_CREDENTIAL_TOO_SHORT: "auth.error.tooShort",
  AUTH_USERNAME_EXISTS: "auth.error.usernameExists",
  AUTH_INVALID_CREDENTIALS: "auth.error.invalidCredentials",
  AUTH_HASH_PASSWORD_FAILED: "auth.error.serverFailed",
  AUTH_CREATE_USER_FAILED: "auth.error.serverFailed",
  AUTH_SIGN_TOKEN_FAILED: "auth.error.serverFailed",
  AUTH_LOOKUP_USER_FAILED: "auth.error.serverFailed",
  AUTH_TERMS_REQUIRED: "auth.error.termsRequired",
  AUTH_INVALID_EMAIL: "auth.error.invalidEmail",
  AUTH_EMAIL_DOMAIN_BLOCKED: "auth.error.emailDomainBlocked",
  AUTH_EMAIL_EXISTS: "auth.error.emailExists",
  AUTH_EMAIL_SEND_FAILED: "auth.error.emailSendFailed",
  AUTH_VERIFY_CODE_INVALID: "auth.error.verifyCodeInvalid",
  AUTH_VERIFY_CODE_TOO_MANY_ATTEMPTS: "auth.error.verifyCodeTooMany",
  AUTH_VERIFY_CODE_EXPIRED: "auth.error.verifyCodeExpired",
  AUTH_RESET_TOKEN_INVALID: "auth.error.resetTokenInvalid",
  AUTH_RESET_TOKEN_EXPIRED: "auth.error.resetTokenExpired",
  AUTH_RESET_PASSWORD_FAILED: "auth.error.resetPasswordFailed",
  AUTH_USER_NOT_FOUND: "auth.error.userNotFound",
  AUTH_CAPTCHA_REQUIRED: "auth.error.captchaRequired",
  AUTH_CAPTCHA_INVALID: "auth.error.captchaInvalid",
  AUTH_CAPTCHA_UNAVAILABLE: "auth.error.captchaUnavailable",
  AUTH_ACCOUNT_BANNED: "auth.error.accountBanned",
  LOGIN_IP_VERIFY_REQUIRED: "auth.error.loginIPVerifyRequired",
  PROFILE_USERNAME_TOO_LONG: "auth.error.usernameTooLong",
  PROFILE_USERNAME_INVALID_FORMAT: "auth.error.usernameInvalidFormat"
}

const legacyErrorToI18nKey: Record<string, string> = {
  "invalid payload": "auth.error.invalidPayload",
  "username/password too short": "auth.error.tooShort",
  "username already exists": "auth.error.usernameExists",
  "invalid credentials": "auth.error.invalidCredentials",
  "hash password failed": "auth.error.serverFailed",
  "create user failed": "auth.error.serverFailed",
  "sign jwt failed": "auth.error.serverFailed",
  "login failed": "auth.error.serverFailed"
}

export function validateAuthInput(username: string, password: string): string | null {
  const trimmedUsername = username.trim()
  if (!trimmedUsername) return "auth.error.usernameRequired"
  if (trimmedUsername.length < MIN_USERNAME_LENGTH) return "auth.error.usernameTooShort"
  if (password.length < MIN_PASSWORD_LENGTH) return "auth.error.passwordTooShort"
  return null
}

export function validateRegisterInput(username: string, password: string, email: string): string | null {
  const baseError = validateAuthInput(username, password)
  if (baseError) return baseError
  const trimmedUsername = username.trim()
  if (trimmedUsername.length > MAX_USERNAME_LENGTH) return "auth.error.usernameTooLong"
  if (!usernamePattern.test(trimmedUsername)) return "auth.error.usernameInvalidFormat"
  const normalizedEmail = email.trim().toLowerCase()
  if (!normalizedEmail || normalizedEmail.length > MAX_EMAIL_LENGTH || !emailPattern.test(normalizedEmail)) {
    return "auth.error.invalidEmail"
  }
  return null
}

export function resolveAuthError(payload: AuthErrorPayload | null): { key: string; debug: string; detail: string } {
  if (!payload) return { key: "auth.error.requestFailed", debug: "", detail: "" }
  const payloadDetail = payload.detail || payload.instanceState || ""
  if (payload.error && typeof payload.error === "object") {
    const errorCode = payload.error.code
    const errorMsg = payload.error.message || ""
    const debug = payload.error.debug || errorCode || errorMsg
    const detail = payload.error.detail || payload.error.instanceState || payloadDetail
    if (errorCode && errorCodeToI18nKey[errorCode]) {
      return { key: errorCodeToI18nKey[errorCode], debug, detail }
    }
    return { key: "auth.error.requestFailed", debug, detail }
  }
  if (payload.code && errorCodeToI18nKey[payload.code]) {
    return {
      key: errorCodeToI18nKey[payload.code],
      debug: payload.debug || payload.code,
      detail: payloadDetail
    }
  }
  if (typeof payload.error === "string" && legacyErrorToI18nKey[payload.error]) {
    return { key: legacyErrorToI18nKey[payload.error], debug: payload.error, detail: payloadDetail }
  }
  if (payload.message) {
    return { key: "auth.error.requestFailed", debug: payload.message, detail: payloadDetail }
  }
  return { key: "auth.error.requestFailed", debug: "", detail: payloadDetail }
}

export function resolveAuthErrorKey(code?: string): string {
  return code && errorCodeToI18nKey[code] ? errorCodeToI18nKey[code] : "auth.error.requestFailed"
}

export async function parseAuthErrorResponse(res: Response): Promise<AuthErrorPayload | null> {
  const raw = await res.text()
  if (!raw) return null
  try {
    return JSON.parse(raw) as AuthErrorPayload
  } catch {
    return { message: raw }
  }
}
