import { z } from "zod"
import { TABLES, schemaRegistry } from "./schemaRegistry"

const rowSchema = z.object({ id: z.string().min(1) }).passthrough()

function crossValidate(catalog) {
  const issues = []
  for (const table of TABLES) {
    const rows = catalog[table] || []
    const idSet = new Set()
    rows.forEach((row, index) => {
      if (!row || typeof row !== "object") {
        issues.push({ table, index, field: "row", message: "行数据必须是对象" })
        return
      }
      const id = typeof row.id === "string" ? row.id.trim() : ""
      if (!id) return
      if (idSet.has(id)) {
        issues.push({ table, index, field: "id", message: `重复 id: ${id}` })
      }
      idSet.add(id)
    })
  }
  return issues
}

export function validateCatalog(catalog) {
  const issues = []
  for (const table of TABLES) {
    const rows = catalog[table] || []
    if (!Array.isArray(rows)) {
      issues.push({ table, field: table, message: "表数据必须是数组" })
      continue
    }
    const def = schemaRegistry[table]
    rows.forEach((row, index) => {
      const parsed = rowSchema.safeParse(row)
      if (!parsed.success) {
        issues.push({ table, index, field: "row", message: parsed.error.issues.map((x) => x.message).join("; ") })
        return
      }
      for (const req of def.required) {
        const value = row[req]
        const missing = value === undefined || value === null || (typeof value === "string" && value.trim() === "")
        if (missing) {
          issues.push({ table, index, field: req, message: `${req} 不能为空` })
        }
      }
    })
  }
  issues.push(...crossValidate(catalog))
  return issues
}
