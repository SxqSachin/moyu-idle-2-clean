import { apiGet, apiPost } from "@services/api"
import type { AppLocale } from "@/i18n"

export type PlayerLanguage = AppLocale
export type PlayerTheme = "light" | "dark"
export type PlayerMenuLayout = "drawer" | "icon-rail"

export type PlayerSettingsPayload = {
  userId: number
  username: string
  usernameChangeId: number
  email: string
  emailVerified: boolean
  language: PlayerLanguage
  theme: PlayerTheme
  menuLayout: PlayerMenuLayout
  simpleMode: boolean
  disableIPLoginVerify: boolean
}

export type PlayerUsernameUpdatePayload = {
  username: string
  previousUsername: string
  changeId: number
}

export type PlayerUsernameUpdateRequest = {
  username: string
  expectedCurrentUsername: string
}

export type PlayerUsernameDraftReconcileResult = {
  draft: string
  baseline: string
  conflict: boolean
}

export function reconcilePlayerUsernameDraft(
  draftUsername: string,
  baselineUsername: string,
  remoteUsername: string
): PlayerUsernameDraftReconcileResult {
  const draft = String(draftUsername || "")
  const normalizedDraft = draft.trim()
  const baseline = String(baselineUsername || "").trim()
  const remote = String(remoteUsername || "").trim()
  if (!remote) {
    return { draft, baseline, conflict: false }
  }
  if (normalizedDraft === baseline || normalizedDraft === remote) {
    return { draft: remote, baseline: remote, conflict: false }
  }
  return { draft, baseline: remote, conflict: true }
}

export async function fetchPlayerSettings(token: string) {
  return apiGet<PlayerSettingsPayload>("/api/player/settings", token)
}

export async function updatePlayerLanguage(token: string, language: PlayerLanguage) {
  return apiPost<{ language: PlayerLanguage }>("/api/player/settings/language", token, { language })
}

export async function updatePlayerTheme(token: string, theme: PlayerTheme) {
  return apiPost<{ theme: PlayerTheme }>("/api/player/settings/theme", token, { theme })
}

export async function updatePlayerMenuLayout(token: string, menuLayout: PlayerMenuLayout) {
  return apiPost<{ menuLayout: PlayerMenuLayout }>("/api/player/settings/menu-layout", token, { menuLayout })
}

export async function updatePlayerSimpleMode(token: string, simpleMode: boolean) {
  return apiPost<{ simpleMode: boolean }>("/api/player/settings/simple-mode", token, { simpleMode })
}

export async function updatePlayerUsername(token: string, username: string, expectedCurrentUsername: string) {
  const request: PlayerUsernameUpdateRequest = {
    username: username.trim(),
    expectedCurrentUsername: expectedCurrentUsername.trim()
  }
  return apiPost<PlayerUsernameUpdatePayload>("/api/player/profile/username", token, request)
}

export async function updatePlayerEmail(token: string, email: string) {
  return apiPost<{ email: string; emailVerified: boolean }>("/api/player/profile/email", token, { email })
}

export async function updatePlayerPassword(token: string, currentPassword: string, newPassword: string) {
  return apiPost<{ userId: number; token: string }>("/api/player/profile/password", token, {
    currentPassword,
    newPassword
  })
}

export async function updateDisableIPLoginVerify(token: string, disableIPLoginVerify: boolean) {
  return apiPost<{ disableIPLoginVerify: boolean }>("/api/player/settings/disable-ip-login-verify", token, {
    disableIPLoginVerify
  })
}
