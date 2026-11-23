import { adminPost } from "./client"

export function grantInventory(payload: Record<string, unknown>) {
  return adminPost<Record<string, unknown>>("inventory", "/grant", payload)
}
