export const adminModulePrefixes = {
  risk: "/api/admin/risk",
  users: "/api/admin/users",
  release: "/api/admin/release",
  battle: "/api/admin/battle",
  inventory: "/api/admin/inventory",
  cattery: "/api/admin/cattery",
  wardrobe: "/api/admin/wardrobe"
} as const

export type AdminModuleName = keyof typeof adminModulePrefixes
