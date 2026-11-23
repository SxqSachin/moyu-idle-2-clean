export function getAdminBridge() {
  if (typeof window === "undefined" || !window.admin) {
    throw new Error("Admin bridge unavailable. Start this UI through Electron, not a standalone browser tab.")
  }
  return window.admin
}
