export const INVENTORY_AUDIT_CATEGORIES = [
  "action_queue",
  "operation_effect",
  "battle",
  "market_trade",
  "auction",
  "shop",
  "admin",
  "daily_task",
  "alchemy",
  "affix",
  "recycle",
  "kitty",
  "cat_village_commission",
  "battle_item_bag",
  "system"
] as const

export type InventoryAuditCategory = (typeof INVENTORY_AUDIT_CATEGORIES)[number]
export type InventoryAuditCategoryFilter = "all" | InventoryAuditCategory

export function auditCategoryQuery(category: InventoryAuditCategoryFilter) {
  return category === "all" ? "" : category
}
