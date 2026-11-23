import { describe, expect, it } from "vitest"
import { APIClientError } from "../src/services/api"
import { resolveErrorI18nKey, resolveErrorMessage } from "../src/services/error-messages"

describe("error message mapping", () => {
  it("explains deferred charge replenishment without suggesting food was consumed", () => {
    expect(resolveErrorI18nKey({ code: "OPERATION_EFFECT_RECOVERY_PENDING" }, "game.error.generic")).toBe(
      "game.operationEffects.recoveryPending"
    )
  })

  it("maps authoritative skill forge inventory conflicts", () => {
    expect(resolveErrorI18nKey({ code: "SKILL_FORGE_INSUFFICIENT_MATERIAL" }, "skillForge.forgeFailed")).toBe(
      "skillForge.recipeInsufficient"
    )
  })

  it("maps affix attach and enchant failures to specific messages", () => {
    expect(resolveErrorI18nKey({ code: "AFFIX_ATTACH_INVALID_ITEM" }, "game.error.generic")).toBe(
      "game.error.affixAttachInvalidItem"
    )
    expect(resolveErrorI18nKey({ code: "AFFIX_ATTACH_AFFIX_NOT_ALLOWED" }, "game.error.generic")).toBe(
      "game.error.affixAttachAffixNotAllowed"
    )
    expect(resolveErrorI18nKey({ code: "AFFIX_ENCHANT_INVALID_ITEM" }, "game.error.generic")).toBe(
      "game.error.affixEnchantInvalidItem"
    )
    expect(resolveErrorI18nKey({ code: "AFFIX_ENCHANT_NO_RARITY_CANDIDATE" }, "game.error.generic")).toBe(
      "game.error.affixEnchantInvalidItem"
    )
    expect(resolveErrorI18nKey({ code: "AFFIX_ATTACH_EQUIPMENT_MAX_EFFECTIVE_EXCEEDED" }, "game.error.generic")).toBe(
      "game.error.affixAttachEquipmentMaxEffectiveExceeded"
    )
    expect(resolveErrorI18nKey({ code: "AFFIX_ENCHANT_EQUIPMENT_MAX_EFFECTIVE_EXCEEDED" }, "game.error.generic")).toBe(
      "game.error.affixEnchantEquipmentMaxEffectiveExceeded"
    )
    expect(resolveErrorI18nKey({ code: "EQUIP_AFFIX_BODY_MAX_EFFECTIVE_EXCEEDED" }, "game.error.generic")).toBe(
      "game.error.equipAffixBodyMaxEffectiveExceeded"
    )
  })

  it("infers i18n key for unmapped backend codes", () => {
    expect(resolveErrorI18nKey({ code: "EQUIP_INVALID_PAYLOAD" }, "game.error.generic")).toBe("game.error.generic")
    expect(resolveErrorI18nKey({ code: "ITEM_RECYCLE_INSTANCE_NOT_OWNED" }, "game.error.generic")).toBe("game.error.unauthorized")
  })

  it("always returns localized text without exposing backend code", () => {
    const err = { code: "EQUIP_INVALID_PAYLOAD", message: "invalid payload" }
    const t = (key: string) => `i18n:${key}`
    expect(resolveErrorMessage(t, err, "game.error.generic")).toBe("i18n:game.error.generic")
  })

  it("maps kitty mode conflict codes to specific messages", () => {
    expect(resolveErrorI18nKey({ code: "KITTY_MODE_CONFLICT_LIFE_QUEUE" }, "game.error.generic")).toBe(
      "game.error.kittyInCombat"
    )
    expect(resolveErrorI18nKey({ code: "KITTY_MODE_CONFLICT_COMBAT" }, "game.error.generic")).toBe(
      "game.error.kittyInLifeQueue"
    )
    expect(resolveErrorI18nKey({ code: "WS_ACTION_KITTY_IN_COMBAT" }, "game.error.generic")).toBe(
      "game.error.wsActionKittyInCombat"
    )
    expect(resolveErrorI18nKey({ code: "WS_ACTION_KITTY_IN_LIFE_QUEUE" }, "game.error.generic")).toBe(
      "game.error.wsActionKittyInLifeQueue"
    )
  })

  it("maps previously generic affix codes to specific messages", () => {
    const codes = [
      ["AFFIX_DETACH_INVALID_PAYLOAD", "game.error.affixDetachInvalidPayload"],
      ["AFFIX_DETACH_FAILED", "game.error.affixDetachFailed"],
      ["AFFIX_ATTACH_INVALID_PAYLOAD", "game.error.affixAttachInvalidPayload"],
      ["AFFIX_ATTACH_SLOT_OCCUPIED", "game.error.affixAttachSlotOccupied"],
      ["AFFIX_ATTACH_NO_EMPTY_SLOT", "game.error.affixAttachNoEmptySlot"],
      ["AFFIX_ATTACH_FAILED", "game.error.affixAttachFailed"],
      ["AFFIX_ENCHANT_INVALID_PAYLOAD", "game.error.affixEnchantInvalidPayload"],
      ["AFFIX_ENCHANT_INSTANCE_INVALID_STATE", "game.error.affixEnchantInstanceInvalidState"],
      ["AFFIX_ENCHANT_INSUFFICIENT_ITEM", "game.error.affixEnchantInsufficientItem"],
      ["AFFIX_ENCHANT_FAILED", "game.error.affixEnchantFailed"],
      ["AFFIX_SMELT_INVALID_CARRIER", "game.error.affixSmeltInvalidCarrier"],
      ["AFFIX_SMELT_FAILED", "game.error.affixSmeltFailed"],
    ] as const
    for (const [code, expected] of codes) {
      expect(resolveErrorI18nKey({ code }, "game.error.generic")).toBe(expected)
    }
  })

  it("maps cattery battle point and shop codes to specific messages", () => {
    expect(resolveErrorI18nKey({ code: "CATTERY_BATTLE_POINT_INSUFFICIENT" }, "game.error.generic")).toBe(
      "game.error.catteryBattlePointInsufficient"
    )
    expect(resolveErrorI18nKey({ code: "SHOP_BUY_INSUFFICIENT_GOLD" }, "game.error.generic")).toBe(
      "game.error.shopBuyInsufficientGold"
    )
    expect(resolveErrorI18nKey({ code: "SHOP_BUY_OFFER_NOT_FOUND" }, "game.error.generic")).toBe(
      "game.error.shopBuyOfferNotFound"
    )
  })

  it("maps battle room and equip codes to specific messages", () => {
    expect(resolveErrorI18nKey({ code: "BATTLE_ROOM_FORBIDDEN" }, "game.error.generic")).toBe(
      "game.error.battleRoomForbidden"
    )
    expect(resolveErrorI18nKey({ code: "BATTLE_ROOM_ENTRY_COST_INSUFFICIENT" }, "game.error.generic")).toBe(
      "game.error.battleRoomEntryCostInsufficient"
    )
    expect(resolveErrorI18nKey({ code: "EQUIP_FAILED" }, "game.error.generic")).toBe("game.error.equipFailed")
    expect(resolveErrorI18nKey({ code: "UNEQUIP_SLOT_EMPTY" }, "game.error.generic")).toBe(
      "game.error.unequipSlotEmpty"
    )
  })

  it("maps specialization conflicts without exposing backend codes", () => {
    const cases = [
      ["SPECIALIZATION_TREE_LOCKED", "game.specialization.error.treeLocked"],
      ["SPECIALIZATION_ACTIVE_ROOT_LIMIT", "game.specialization.error.activeRootLimit"],
      ["SPECIALIZATION_EXCLUSIVE_CONFLICT", "game.specialization.error.exclusiveConflict"],
      ["SPECIALIZATION_POINTS_INSUFFICIENT", "game.specialization.error.pointsInsufficient"],
      ["SPECIALIZATION_REVISION_CONFLICT", "game.specialization.error.revisionConflict"],
      ["SPECIALIZATION_RULESET_CONFLICT", "game.specialization.error.rulesetConflict"],
      ["SPECIALIZATION_IMPACT_CONFLICT", "game.specialization.error.impactConflict"],
      ["SPECIALIZATION_IMPACT_CONFIRMATION_REQUIRED", "game.specialization.error.impactConfirmationRequired"],
      ["SPECIALIZATION_RESET_COST_INSUFFICIENT", "game.specialization.error.resetCostInsufficient"],
      ["EQUIP_SLOT_DISABLED_BY_SPECIALIZATION", "game.specialization.error.slotDisabled"]
    ] as const
    for (const [code, expected] of cases) {
      expect(resolveErrorI18nKey({ code }, "game.specialization.operationFailed")).toBe(expected)
    }
  })

  it("maps WS enhance and chat codes to specific messages", () => {
    expect(resolveErrorI18nKey({ code: "WS_ENHANCE_OVERFLOW" }, "game.error.generic")).toBe(
      "game.error.wsEnhanceOverflow"
    )
    expect(resolveErrorI18nKey({ code: "WS_CHAT_TOO_LONG" }, "game.error.generic")).toBe("game.error.wsChatTooLong")
    expect(resolveErrorI18nKey({ code: "WS_CHAT_CONTENT_BLOCKED" }, "game.error.generic")).toBe(
      "game.error.chatContentBlocked"
    )
  })

  it("maps content moderation codes to specific messages", () => {
    expect(resolveErrorI18nKey({ code: "CATTERY_ADOPT_NAME_BLOCKED" }, "game.error.generic")).toBe(
      "game.error.adoptNameBlocked"
    )
    expect(resolveErrorI18nKey({ code: "BATTLE_ROOM_NAME_BLOCKED" }, "game.error.generic")).toBe(
      "game.error.battleRoomNameBlocked"
    )
    expect(resolveErrorI18nKey({ code: "CHAT_CONTENT_BLOCKED" }, "game.error.generic")).toBe(
      "game.error.chatContentBlocked"
    )
  })

  it("maps account restriction codes to specific messages", () => {
    expect(resolveErrorI18nKey({ code: "AUTH_LOGIN_LOCKED" }, "game.error.generic")).toBe(
      "game.error.authLoginLocked"
    )
    expect(resolveErrorI18nKey({ code: "ACCOUNT_RESTRICTED" }, "game.error.generic")).toBe(
      "game.error.accountRestricted"
    )
  })

  it("injects affix name for max-effective errors", () => {
    const err = new APIClientError("affix exceeds equipment max", {
      code: "AFFIX_ATTACH_EQUIPMENT_MAX_EFFECTIVE_EXCEEDED",
      instanceState: "affix.speed_minor"
    })
    const t = (key: string, params?: Record<string, unknown>) => {
      if (key === "affix.speed_minor.name") return "急速"
      if (key === "game.error.affixAttachEquipmentMaxEffectiveExceeded") {
        return `词条 ${String(params?.affix || "")} 超限`
      }
      return key
    }
    expect(resolveErrorMessage(t, err, "game.error.generic")).toBe("词条 急速 超限")
  })

  it("names the exact fishing supply that is missing", () => {
    const err = new APIClientError("fishing supply insufficient", {
      code: "WS_FISHING_SUPPLY_INSUFFICIENT",
      instanceState: "reed_float"
    })
    const t = (key: string, params?: Record<string, unknown>) => {
      if (key === "item.reed_float.name") return "芦苇浮漂"
      if (key === "game.fishing.supplyUnavailable") {
        return `仓库里的${String(params?.item || "")}不足`
      }
      return key
    }
    expect(resolveErrorMessage(t, err, "game.fishing.tripStartFailed")).toBe("仓库里的芦苇浮漂不足")
    expect(resolveErrorI18nKey({ code: "WS_FISHING_TACKLE_MISSING" }, "game.fishing.tripStartFailed")).toBe(
      "game.fishing.tackleUnavailable"
    )
  })

  it("names the tackle the chosen angler still needs to equip", () => {
    const err = new APIClientError("fishing tackle is not equipped", {
      code: "WS_FISHING_TACKLE_NOT_EQUIPPED",
      instanceState: "basic_fishing_rod"
    })
    const t = (key: string, params?: Record<string, unknown>) => {
      if (key === "item.basic_fishing_rod.name") return "基础鱼竿"
      if (key === "game.fishing.tackleNotEquipped") {
        return `出发者还没有装备这套钓组中的${String(params?.item || "")}`
      }
      return key
    }
    expect(resolveErrorMessage(t, err, "game.fishing.tripStartFailed")).toBe("出发者还没有装备这套钓组中的基础鱼竿")
  })

  it("explains that active fishing gear cannot be moved", () => {
    expect(resolveErrorI18nKey({ code: "WS_FISHING_TACKLE_IN_USE" }, "game.error.generic")).toBe(
      "game.fishing.tackleInUse"
    )
    expect(resolveErrorI18nKey({ code: "EQUIP_FISHING_TACKLE_IN_USE" }, "game.error.generic")).toBe(
      "game.fishing.tackleInUse"
    )
    expect(resolveErrorI18nKey({ code: "CATTERY_KITTY_EQUIP_FISHING_TACKLE_IN_USE" }, "game.error.generic")).toBe(
      "game.fishing.tackleInUse"
    )
  })
})
