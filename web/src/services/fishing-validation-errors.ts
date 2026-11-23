const reasons = [
  "PAYLOAD_INVALID", "SPOT_REQUIRED", "LOADOUT_REQUIRED", "DIVISION_INVALID",
  "DURATION_INVALID", "STRATEGY_INVALID", "SAFETY_INVALID", "LAYER_INVALID", "PACE_INVALID",
  "METHOD_INVALID", "LOADOUT_CORRUPT", "LOAD_CAPACITY_INVALID", "SPOT_METHOD_MISMATCH",
  "BOTTOM_LAYER_MISMATCH", "STANDARD_UNAVAILABLE", "STANDARD_MISMATCH", "STANDARD_CONFIG_INVALID",
  "RUNTIME_INVALID", "LOADOUT_NAME_INVALID", "LOADOUT_LIMIT"
] as const

const slotKeys: Record<string, string> = {
  rod: "rod", reel: "reel", main_line: "mainLine", leader: "leader",
  hook: "hook", float_or_sinker: "floatOrSinker", lure: "lure", bait: "bait"
}

type FishingValidationMessage = { key: string; fieldKey?: string }

// The slot is part of the code so both HTTP and WebSocket errors retain it.
export const fishingValidationMessages: Record<string, FishingValidationMessage> = {}
function add(reason: string, field?: string) {
  const suffix = field ? `_${field.toUpperCase()}` : ""
  const message = {
    key: `game.fishing.validation_${reason}`,
    fieldKey: field ? `game.fishing.${slotKeys[field]}` : undefined
  }
  for (const prefix of ["FISHING_SETTINGS_", "WS_FISHING_SETTINGS_"]) {
    fishingValidationMessages[`${prefix}${reason}${suffix}`] = message
  }
}
for (const reason of reasons) add(reason)
for (const field of Object.keys(slotKeys)) {
  for (const reason of ["COMPONENT_REQUIRED", "COMPONENT_INCOMPATIBLE", "EQUIPMENT_INVALID"]) add(reason, field)
  if (field !== "rod" && field !== "reel") {
    for (const reason of ["SUPPLY_RANGE", "SUPPLY_REQUIRED", "SUPPLY_UNEXPECTED"]) add(reason, field)
  }
}
