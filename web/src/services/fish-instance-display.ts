import { getGameDataChunk } from "@services/game-data-runtime"

type LocalizedText = Record<string, string>

export type FishSpeciesDisplay = {
  id: string
  rarity: string
  names: LocalizedText
  iconPath: string
  minLengthCm: number
  maxLengthCm: number
  shapeFactor: number
}

export type FishingPlaceDisplay = {
  id: string
  names: LocalizedText
}

let speciesByID: Map<string, FishSpeciesDisplay> | null = null
let spotByID: Map<string, FishingPlaceDisplay> | null = null
let waterByID: Map<string, FishingPlaceDisplay> | null = null

function ensureMaps() {
  if (speciesByID && spotByID && waterByID) return
  const species = (getGameDataChunk("fishing_species") as { fishing_species?: FishSpeciesDisplay[] }).fishing_species || []
  const spots = (getGameDataChunk("fishing_spots") as { fishing_spots?: FishingPlaceDisplay[] }).fishing_spots || []
  const waters = (getGameDataChunk("fishing_waters") as { fishing_waters?: FishingPlaceDisplay[] }).fishing_waters || []
  speciesByID = new Map(species.map((row) => [row.id, row]))
  spotByID = new Map(spots.map((row) => [row.id, row]))
  waterByID = new Map(waters.map((row) => [row.id, row]))
}

export function getFishSpeciesDisplay(speciesID: string): FishSpeciesDisplay | null {
  ensureMaps()
  return speciesByID!.get(String(speciesID || "").trim()) || null
}

export function getFishingSpotDisplay(spotID: string): FishingPlaceDisplay | null {
  ensureMaps()
  return spotByID!.get(String(spotID || "").trim()) || null
}

export function getFishingWaterDisplay(waterID: string): FishingPlaceDisplay | null {
  ensureMaps()
  return waterByID!.get(String(waterID || "").trim()) || null
}

export function localizedFishText(names: LocalizedText | undefined, locale: string, fallback = ""): string {
  if (!names) return fallback
  const normalized = String(locale || "").trim()
  return names[normalized] || names[normalized.split("-")[0]] || names["zh-CN"] || names["en-US"] || fallback
}
