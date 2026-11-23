import type { FishingConditions } from "./fishing"

export type FishingDayPeriod = "dawn" | "day" | "dusk" | "night" | "unknown"
export type FishingSceneWeather = "clear" | "cloud" | "rain" | "storm" | "mist" | "snow" | "unknown"
export type FishingSceneEnvironment = {
  period: FishingDayPeriod
  weather: FishingSceneWeather
  celestial: "sun" | "moon" | null
  precipitation: "rain" | "snow" | null
}

const sceneAssets: Readonly<Record<string, string>> = {
  sunmere_shallows: "/assets/scenes/fishing-sunmere-v1.webp",
  willowrun_valley: "/assets/scenes/fishing-willowrun-v1.webp",
  frostglass_lake: "/assets/scenes/fishing-frostglass-v1.webp"
}

export function fishingSceneAsset(waterId?: string): string {
  return waterId && Object.prototype.hasOwnProperty.call(sceneAssets, waterId) ? sceneAssets[waterId] : sceneAssets.sunmere_shallows
}

// Presentation only: use the server's game hour, never the device clock.
export function fishingSceneEnvironment(conditions?: Pick<FishingConditions, "gameHour" | "weather">): FishingSceneEnvironment {
  const hour = conditions?.gameHour
  let period: FishingDayPeriod = "unknown"
  if (typeof hour === "number" && Number.isFinite(hour) && hour >= 0 && hour < 24) {
    if (hour < 5 || hour >= 21) period = "night"
    else if (hour < 9) period = "dawn"
    else if (hour < 17) period = "day"
    else period = "dusk"
  }
  let weather: FishingSceneWeather = "unknown"
  switch (conditions?.weather) {
    case "clear": case "sunny": weather = "clear"; break
    case "cloud": case "rain": case "storm": case "mist": case "snow": weather = conditions.weather; break
  }
  return {
    period,
    weather,
    celestial: weather !== "clear" || period === "unknown" ? null : period === "night" ? "moon" : "sun",
    precipitation: weather === "snow" ? "snow" : weather === "rain" || weather === "storm" ? "rain" : null
  }
}
