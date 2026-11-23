import itemMetrics from "../config/metrics/item.json"
import equipmentMetrics from "../config/metrics/equipment.json"
import actionMetrics from "../config/metrics/action.json"
import enemyMetrics from "../config/metrics/enemy.json"

export const metricRegistry = {
  items: itemMetrics,
  equipment_templates: equipmentMetrics,
  actions: actionMetrics,
  enemies: enemyMetrics
}
