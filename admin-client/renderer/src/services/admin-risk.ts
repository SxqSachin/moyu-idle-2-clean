import type { AuctionTransferItem, ClusterSummary, HighRiskUserItem, PaginatedResult, PasswordGroup, RiskClusterDetail, RiskSearchItem, RiskUserDetail, ReviewCase } from "@shared/types"
import { adminGet, adminPost } from "./client"

function normalizePaginatedResult<T>(payload: unknown): PaginatedResult<T> {
  if (Array.isArray(payload)) {
    return {
      items: payload as T[],
      total: payload.length,
      limit: payload.length,
      offset: 0
    }
  }
  const value = (payload ?? {}) as Partial<PaginatedResult<T>> & { items?: T[] }
  const items = Array.isArray(value.items) ? value.items : []
  return {
    items,
    total: typeof value.total === "number" ? value.total : items.length,
    limit: typeof value.limit === "number" ? value.limit : items.length,
    offset: typeof value.offset === "number" ? value.offset : 0
  }
}

export function searchRiskAccounts(query: string) {
  return adminGet<{ items: RiskSearchItem[] }>("risk", "/search", { q: query })
}

export async function fetchHighRiskUsers(limit = 30, offset = 0) {
  const payload = await adminGet<unknown>("risk", "/high-risk-users", { limit, offset })
  return normalizePaginatedResult<HighRiskUserItem>(payload)
}

export async function fetchRiskClusters(limit = 30, offset = 0) {
  const payload = await adminGet<unknown>("risk", "/clusters", { limit, offset })
  return normalizePaginatedResult<ClusterSummary>(payload)
}

export function fetchRiskUserDetail(userId: number) {
  return adminGet<RiskUserDetail>("risk", `/users/${userId}`)
}

export function fetchRiskClusterDetail(clusterId: number) {
  return adminGet<RiskClusterDetail>("risk", `/clusters/${clusterId}`)
}

export function fetchRiskReviewCases() {
  return adminGet<{ items: ReviewCase[] }>("risk", "/review-cases")
}

export function applyRiskClusterAction(clusterId: number, action: "recompute" | "restrict" | "release" | "ban", operatorName: string, operatorNote: string) {
  return adminPost<{ ok: boolean }>("risk", `/clusters/${clusterId}/${action}`, { operatorName, operatorNote })
}

export function resolveRiskReviewCase(caseId: number, operatorName: string, operatorNote: string, decision: string) {
  return adminPost<{ ok: boolean }>("risk", `/review-cases/${caseId}/resolve`, { operatorName, operatorNote, decision })
}

export async function fetchPasswordGroups(limit = 30, offset = 0) {
  const payload = await adminGet<unknown>("risk", "/password-groups", { limit, offset })
  return normalizePaginatedResult<PasswordGroup>(payload)
}

export async function fetchAuctionTransfers(limit = 50, offset = 0) {
  const payload = await adminGet<unknown>("risk", "/auction-transfers", { limit, offset })
  return normalizePaginatedResult<AuctionTransferItem>(payload)
}

export function resetAllRiskData() {
  return adminPost<{ ok: boolean }>("risk", "/reset-all", {})
}
