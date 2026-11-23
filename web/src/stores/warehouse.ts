import { defineStore } from "pinia"
import { apiGet } from "@services/api"
import type { InventoryAuditCategoryFilter } from "@services/inventory-audit"
import { auditCategoryQuery } from "@services/inventory-audit"

type WarehouseLogItem = {
  id: number
  eventType: "system_to_warehouse" | "warehouse_operation"
  category?: string
  direction?: string
  subjectType?: string
  subjectId?: string
  warehouseId: number
  instanceId: number
  instanceIds?: number[]
  templateItemId: string
  quantity: number
  items?: Record<string, number>
  sourceSurface: string
  targetSurface: string
  details?: Record<string, unknown>
  createdAt: string
}

type WarehouseLogsPayload = {
  items?: WarehouseLogItem[]
}

export type { WarehouseLogItem }

export const useWarehouseStore = defineStore("warehouse", {
  state: () => ({
    initialized: false,
    activeCategory: "all" as InventoryAuditCategoryFilter,
    logItems: [] as WarehouseLogItem[],
    logItemsByCategory: {} as Record<string, WarehouseLogItem[]>,
    initializedByCategory: {} as Record<string, boolean>
  }),
  actions: {
    applySnapshot(items: WarehouseLogItem[], category: InventoryAuditCategoryFilter = "all") {
      this.activeCategory = category
      this.logItems = items
      this.logItemsByCategory[category] = items
      this.initialized = true
      this.initializedByCategory[category] = true
    },
    async refreshAll(token: string, category: InventoryAuditCategoryFilter = "all") {
      const query = auditCategoryQuery(category)
      const path = query ? `/api/warehouses/logs?category=${encodeURIComponent(query)}` : "/api/warehouses/logs"
      const logsData = await apiGet<WarehouseLogsPayload>(path, token)
      this.applySnapshot(logsData.items || [], category)
    },
    async initialize(token: string, force = false, category: InventoryAuditCategoryFilter = "all") {
      if (!token) {
        this.reset()
        return
      }
      if (this.initializedByCategory[category] && !force) {
        this.activeCategory = category
        this.logItems = this.logItemsByCategory[category] || []
        return
      }
      await this.refreshAll(token, category)
    },
    reset() {
      this.initialized = false
      this.activeCategory = "all"
      this.logItems = []
      this.logItemsByCategory = {}
      this.initializedByCategory = {}
    }
  }
})
