import type { AdminBanReason, AdminUserDetail } from "@shared/types"
import { adminGet, adminPost } from "./client"

export function fetchBanReasons() {
  return adminGet<{ ok: boolean; reasons: AdminBanReason[] }>("users", "/ban-reasons")
}

export function fetchBannedUsers(query = "") {
  return adminGet<{ items: AdminUserDetail[] }>("users", "/banned", { q: query })
}

export function fetchAdminUserDetail(userId: number) {
  return adminGet<AdminUserDetail>("users", `/${userId}`)
}

export function banUser(payload: {
  userId?: number
  username?: string
  reason: string
  isPermanent: boolean
  banUntil?: string
}) {
  return adminPost<{ ok: boolean; userId: number }>("users", "/ban", payload)
}

export function unbanUser(payload: { userId?: number; username?: string }) {
  return adminPost<{ ok: boolean; userId: number }>("users", "/unban", payload)
}
