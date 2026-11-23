import { adminPost } from "./client"

export function triggerCattery(payload: Record<string, unknown>) {
  return adminPost<Record<string, unknown>>("cattery", "/trigger", payload)
}

export function addCatteryPersonality(payload: Record<string, unknown>) {
  return adminPost<Record<string, unknown>>("cattery", "/personality/add", payload)
}

export function removeCatteryPersonality(payload: Record<string, unknown>) {
  return adminPost<Record<string, unknown>>("cattery", "/personality/remove", payload)
}
