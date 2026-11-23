import { APIClientError, apiPost, apiPostPublic, createIdempotencyKey } from "@services/api"
import { resolveAuthErrorKey } from "@services/auth"

async function postRecovery<T>(endpoint: string, body: Record<string, unknown>, token?: string): Promise<T> {
  try {
    const headers = { "X-Idempotency-Key": createIdempotencyKey("auth-recovery") }
    return token
      ? await apiPost<T>(endpoint, token, body, headers)
      : await apiPostPublic<T>(endpoint, body, headers)
  } catch (error) {
    if (error instanceof APIClientError) {
      throw new Error(resolveAuthErrorKey(error.code))
    }
    throw error
  }
}

export async function requestForgotPasswordCode(email: string, captchaToken: string) {
  return postRecovery<{ sent: boolean }>("/api/password/forgot/send", {
    email,
    captchaToken
  })
}

export async function verifyForgotPasswordCode(email: string, code: string) {
  return postRecovery<{ resetToken: string; expiresIn: number }>("/api/password/forgot/verify", {
    email,
    code
  })
}

export async function resetForgotPassword(token: string, newPassword: string) {
  return postRecovery<{ userId: number }>("/api/password/forgot/reset", {
    token,
    newPassword
  })
}

export async function requestPlayerEmailVerification(token: string, email: string, captchaToken: string) {
  return postRecovery<{ email: string; emailVerified: boolean }>(
    "/api/player/profile/email/verify/send",
    {
      email,
      captchaToken
    },
    token
  )
}

export async function confirmPlayerEmailVerification(token: string, code: string) {
  return postRecovery<{ email: string; emailVerified: boolean; username: string }>(
    "/api/player/profile/email/verify/confirm",
    {
      code
    },
    token
  )
}
