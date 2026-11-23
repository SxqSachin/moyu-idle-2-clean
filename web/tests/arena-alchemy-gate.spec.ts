import { describe, expect, it } from "vitest"
import { APIClientError } from "../src/services/api"
import { arenaAlchemyGatePayloadFromError } from "../src/services/arena"

describe("arena alchemy equipment gate", () => {
  it("keeps unit, equipment, and attribute details from the rejected request", () => {
    const error = new APIClientError("restricted", {
      code: "ARENA_ALCHEMY_EQUIPMENT_RESTRICTED",
      status: 409,
      data: {
        policyVersion: "arena-alchemy-v1",
        requiredAlchemyRulesetVersion: "alchemy-v2",
        violations: [{
          unitType: "kitty",
          unitId: 12,
          unitName: "Mimi",
          equipmentInstanceId: 91,
          templateItemId: "moon_armor",
          affixSlotIndex: 1,
          attribute: "speed",
          actualValue: 2.5,
          allowedValue: 0.75,
          reason: "legacy_unversioned_value_exceeds_current_ruleset"
        }]
      }
    })

    expect(arenaAlchemyGatePayloadFromError(error)).toEqual({
      policyVersion: "arena-alchemy-v1",
      requiredAlchemyRulesetVersion: "alchemy-v2",
      violations: [{
        unitType: "kitty",
        unitId: 12,
        unitName: "Mimi",
        equipmentInstanceId: 91,
        templateItemId: "moon_armor",
        equipmentName: undefined,
        equippedSlot: undefined,
        affixSlotIndex: 1,
        alchemyRulesetVersion: undefined,
        attribute: "speed",
        actualValue: 2.5,
        allowedValue: 0.75,
        reason: "legacy_unversioned_value_exceeds_current_ruleset"
      }]
    })
  })

  it("does not reinterpret unrelated API failures as equipment results", () => {
    const error = new APIClientError("failed", { code: "ARENA_LINEUP_INVALID", data: { violations: [] } })
    expect(arenaAlchemyGatePayloadFromError(error)).toBeNull()
  })
})
