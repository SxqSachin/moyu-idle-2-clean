import type { ReleaseAnnouncement } from "@shared/types"
import { adminPost } from "./client"

export function announceRelease(payload: ReleaseAnnouncement) {
  return adminPost<ReleaseAnnouncement>("release", "/announce", payload)
}
