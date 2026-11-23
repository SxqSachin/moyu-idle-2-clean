import { getGameDataChunk } from "@services/game-data-runtime"
import type { BuildingDefinition } from "@game-data/game-data"

function isBuildingDefinition(value: unknown): value is BuildingDefinition {
  if (!value || typeof value !== "object") return false
  const row = value as Record<string, unknown>
  return (
    typeof row.id === "string" &&
    typeof row.iconId === "string" &&
    typeof row.nameKey === "string" &&
    typeof row.descKey === "string" &&
    typeof row.order === "number" &&
    typeof row.maxLevel === "number" &&
    typeof row.baseDurationS === "number" &&
    typeof row.stepDurationS === "number" &&
    !!row.baseCosts &&
    typeof row.baseCosts === "object" &&
    !!row.stepCosts &&
    typeof row.stepCosts === "object" &&
    !!row.baseStatusRequirement &&
    typeof row.baseStatusRequirement === "object" &&
    !!row.stepStatusRequirement &&
    typeof row.stepStatusRequirement === "object"
  )
}

function getRawBuildings(catalog: unknown): unknown[] {
  if (!catalog || typeof catalog !== "object") return []
  const buildings = (catalog as { buildings?: unknown }).buildings
  return Array.isArray(buildings) ? buildings : []
}

let buildingDefinitions: BuildingDefinition[] | null = null
let buildingDefinitionMap: Map<string, BuildingDefinition> | null = null

function ensureBuildingDefinitions() {
  if (buildingDefinitions && buildingDefinitionMap) return
  buildingDefinitions = getRawBuildings(getGameDataChunk("buildings"))
    .filter((row): row is BuildingDefinition => isBuildingDefinition(row))
  buildingDefinitionMap = new Map<string, BuildingDefinition>(buildingDefinitions.map((row) => [row.id, row]))
}

export function listBuildingDefinitions(): BuildingDefinition[] {
  ensureBuildingDefinitions()
  return buildingDefinitions!
}

export function getBuildingDefinition(buildingId: string): BuildingDefinition | undefined {
  ensureBuildingDefinitions()
  return buildingDefinitionMap!.get(buildingId)
}
