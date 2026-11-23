export const TABLES = ["items", "actions", "equipment_templates", "enemies"]

export const schemaRegistry = {
  items: {
    idField: "id",
    required: ["id", "type"]
  },
  actions: {
    idField: "id",
    required: ["id", "nameKey"]
  },
  equipment_templates: {
    idField: "id",
    required: ["id", "slot"]
  },
  enemies: {
    idField: "id",
    required: ["id", "nameKey"]
  }
}

export const formulaTemplates = [
  {
    id: "economy_value",
    label: "经济价值",
    expression: "(Number(baseValue) || 0) * (Number(rarityMultiplier) || 1)"
  },
  {
    id: "equipment_power",
    label: "装备战力评分",
    expression: "(Number(atk) || 0) * 1.5 + (Number(def) || 0) + (Number(hp) || 0) * 0.05"
  },
  {
    id: "enemy_growth",
    label: "敌人数值成长",
    expression: "(Number(baseHp) || 0) * Math.pow(1 + (Number(growthRate) || 0), Number(level) || 1)"
  }
]
