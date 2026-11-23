import { APIClientError } from "@services/api"
import { getLocalizedItemName } from "@services/item-label"
import { fishingValidationMessages } from "@services/fishing-validation-errors"

const codeToI18nKey: Record<string, string> = {
  ...Object.fromEntries(Object.entries(fishingValidationMessages).map(([code, message]) => [code, message.key])),
  WS_FISHING_INVALID: "game.fishing.validation_unknown",
  FISHING_INVALID: "game.fishing.validation_unknown",
  FISHING_POND_REAGENT_MISSING: "game.fishing.pondReagentMissing",
  FISHING_POND_MASH_MISSING: "game.fishing.pondMashMissing",
  ARENA_DISABLED: "game.arena.error.disabled",
  ARENA_LEADERBOARD_FAILED: "game.arena.error.leaderboardFailed",
  ARENA_DEFENSE_NOT_FOUND: "game.arena.error.defenseNotFound",
  ARENA_DEFENSE_BUSY: "game.arena.error.defenseBusy",
  ARENA_DEFENSE_POLICY_OUTDATED: "game.arena.error.defensePolicyOutdated",
  ARENA_ALCHEMY_EQUIPMENT_RESTRICTED: "game.arena.error.alchemyEquipmentRestricted",
  ARENA_LINEUP_CHANGED: "game.arena.error.lineupChanged",
  ARENA_CHALLENGE_NOT_FOUND: "game.arena.error.challengeNotFound",
  ARENA_CHALLENGE_ACTIVE: "game.arena.error.challengeActive",
  ARENA_CHALLENGE_CHARGES_EXHAUSTED: "game.arena.error.challengeEntriesExhausted",
  ARENA_CHALLENGE_OPPONENT_COOLDOWN: "game.arena.error.opponentCooldown",
  ARENA_RECOMMENDATION_REFRESH_COOLDOWN: "game.arena.error.recommendationRefreshCooldown",
  ARENA_RECOMMENDATION_EXPIRED: "game.arena.error.recommendationExpired",
  ARENA_TOKENS_INSUFFICIENT: "game.arena.error.tokensInsufficient",
  ARENA_KITTY_NOT_FOUND: "game.arena.error.kittyNotFound",
  ARENA_LINEUP_INVALID: "game.arena.error.lineupInvalid",
  ARENA_REPLAY_SHARE_EXPIRED: "game.arena.error.replaySharedFailed",
  WS_ARENA_REPLAY_EXPIRED: "game.arena.error.replaySharedFailed",
  COMBAT_TRIAL_DISABLED: "game.combatTrial.error.disabled",
  COMBAT_TRIAL_FORMAT_INVALID: "game.combatTrial.error.formatInvalid",
  COMBAT_TRIAL_METRIC_INVALID: "game.combatTrial.error.metricInvalid",
  COMBAT_TRIAL_RUN_ACTIVE: "game.combatTrial.error.runActive",
  COMBAT_TRIAL_RUN_NOT_FOUND: "game.combatTrial.error.runNotFound",
  COMBAT_TRIAL_REPLAY_EXPIRED: "game.combatTrial.error.replayExpired",
  COMBAT_TRIAL_LINEUP_CHANGED: "game.combatTrial.error.lineupChanged",
  COMBAT_TRIAL_KITTY_NOT_FOUND: "game.combatTrial.error.kittyNotFound",
  COMBAT_TRIAL_LINEUP_INVALID: "game.combatTrial.error.lineupInvalid",
  COMBAT_TRIAL_ALCHEMY_EQUIPMENT_RESTRICTED: "game.combatTrial.error.alchemyEquipmentRestricted",
  COMBAT_TRIAL_OVERVIEW_FAILED: "game.combatTrial.error.overviewFailed",
  COMBAT_TRIAL_RUN_CREATE_FAILED: "game.combatTrial.error.runCreateFailed",
  COMBAT_TRIAL_HISTORY_FAILED: "game.combatTrial.error.historyFailed",
  COMBAT_TRIAL_LEADERBOARD_FAILED: "game.combatTrial.error.leaderboardFailed",
  COMBAT_TRIAL_REPLAY_FAILED: "game.combatTrial.error.replayFailed",
  COMBAT_TRIAL_REPLAY_SHARED_FAILED: "game.combatTrial.error.replaySharedFailed",
  COMBAT_TRIAL_REPLAY_SHARE_FAILED: "game.combatTrial.error.replayShareFailed",
  WS_COMBAT_TRIAL_REPLAY_EXPIRED: "game.combatTrial.error.replayExpired",
  WS_CHAT_ATTACHMENT_INVALID: "game.error.wsChatAttachmentInvalid",
  WS_CHAT_ITEM_INSTANCE_UNAVAILABLE: "game.error.wsChatItemInstanceUnavailable",
  SKILL_FORGE_LIMIT: "skillForge.draft.limitReachedShort",
  SKILL_FORGE_RULESET_DISABLED: "skillForge.rulesetDisabled",
  SKILL_FORGE_INSUFFICIENT_MATERIAL: "skillForge.recipeInsufficient",
  DAILY_TASK_NOT_FOUND: "game.error.taskNotFound",
  DAILY_TASK_ALREADY_COMPLETED: "game.error.taskAlreadyCompleted",
  DAILY_TASK_INSUFFICIENT_ITEMS: "game.error.taskInsufficientItems",
  EQUIP_SLOT_NOT_ALLOWED: "game.error.equipSlotNotAllowed",
  EQUIP_TARGET_NOT_ALLOWED: "game.error.equipTargetNotAllowed",
  EQUIP_ACCESSORY_CONFLICT: "game.error.equipAccessoryConflict",
  EQUIP_STATUS_REQUIREMENT_NOT_MET: "game.error.statusRequirementNotMet",
  EQUIP_SLOT_DISABLED_BY_SPECIALIZATION: "game.specialization.error.slotDisabled",
  COMBAT_CONFIG_SKILL_STATUS_REQUIREMENT_NOT_MET: "game.error.combatSkillStatusRequirementNotMet",
  COMBAT_CONFIG_SKILL_WEAPON_REQUIREMENT_NOT_MET: "game.error.combatSkillWeaponRequirementNotMet",
  EQUIPMENT_ITEM_NOT_EQUIPMENT: "game.error.itemNotEquipment",
  EQUIPMENT_SKILL_CONFIG_INVALID: "game.error.equipmentSkillConfigInvalid",
  EQUIPMENT_SKILL_CONFIG_CHANGED: "game.error.equipmentSkillConfigChanged",
  EQUIPMENT_SKILL_CONFIG_SAVE_FAILED: "game.saveFailed",
  ITEM_USE_SKILLBOOK_INVALID_TYPE: "game.useSkillbookFailed",
  ITEM_USE_SKILLBOOK_INVALID_STATE: "game.useSkillbookFailed",
  ITEM_USE_SKILLBOOK_INVALID_SKILL: "game.useSkillbookFailed",
  ITEM_USE_SKILLBOOK_FAILED: "game.useSkillbookFailed",
  OPERATION_EFFECT_USE_INVALID_REQUEST: "game.operationEffects.targetInvalid",
  OPERATION_EFFECT_TARGET_NOT_OWNED: "game.operationEffects.targetInvalid",
  OPERATION_EFFECT_DEFINITION_NOT_FOUND: "game.operationEffects.unavailable",
  OPERATION_EFFECT_AT_CAP: "game.operationEffects.atCap",
  OPERATION_EFFECT_RECOVERY_PENDING: "game.operationEffects.recoveryPending",
  OFFLINE_RECOVERY_PENDING: "game.fastForwardMutationPending",
  OFFLINE_RECOVERY_UNAVAILABLE: "game.fastForwardWaiting",
  OPERATION_EFFECT_STRENGTH_CONFLICT: "game.operationEffects.strengthConflict",
  OPERATION_EFFECT_REQUEST_CONFLICT: "game.operationEffects.useFailed",
  OPERATION_EFFECT_ITEM_UNAVAILABLE: "game.operationEffects.itemUnavailable",
  OPERATION_EFFECT_ITEM_NOT_OWNED: "game.error.unauthorized",
  OPERATION_EFFECT_USE_FAILED: "game.operationEffects.useFailed",
  ITEM_RECYCLE_INVALID_PAYLOAD: "game.recycleFailed",
  ITEM_RECYCLE_INVALID_REQUEST: "game.recycleFailed",
  ITEM_RECYCLE_INVALID_OPERATION: "game.recycleFailed",
  ITEM_DESTRUCTIVE_RECYCLE_CHANGED: "game.destructiveRecycle.changed",
  ITEM_DESTRUCTIVE_RECYCLE_INELIGIBLE: "game.destructiveRecycle.ineligible",
  ITEM_RECYCLE_INSTANCE_NOT_FOUND: "game.recycleFailed",
  ITEM_RECYCLE_INSTANCE_NOT_OWNED: "game.error.unauthorized",
  ITEM_RECYCLE_STATE_OR_QUANTITY_CONFLICT: "game.recycleFailed",
  ITEM_RECYCLE_FAILED: "game.recycleFailed",
  ITEM_RENAME_INVALID_PAYLOAD: "game.itemRenameFailed",
  ITEM_RENAME_INVALID_NAME: "game.itemRenameNameInvalid",
  ITEM_RENAME_NAME_BLOCKED: "error.contentModeration.blocked",
  ITEM_RENAME_INSTANCE_NOT_FOUND: "game.itemRenameFailed",
  ITEM_RENAME_INSTANCE_NOT_OWNED: "game.error.unauthorized",
  ITEM_RENAME_INSTANCE_INVALID_STATE: "game.itemRenameInvalidState",
  ITEM_RENAME_STACKABLE_ITEM: "game.itemRenameStackableItem",
  ITEM_RENAME_SAME_NAME: "game.itemRenameSameName",
  ITEM_RENAME_INSUFFICIENT_ITEM: "game.itemRenameInsufficientItem",
  ITEM_RENAME_FAILED: "game.itemRenameFailed",
  INVENTORY_ARCHIVE_INVALID_PAYLOAD: "game.inventoryArchiveFailed",
  INVENTORY_ARCHIVE_INVALID_REQUEST: "game.inventoryArchiveFailed",
  INVENTORY_ARCHIVE_INSTANCE_NOT_FOUND: "game.inventoryArchiveFailed",
  INVENTORY_ARCHIVE_INSTANCE_INVALID_STATE: "game.inventoryArchiveFailed",
  INVENTORY_ARCHIVE_FAILED: "game.inventoryArchiveFailed",
  MARKET_BUY_ORDER_INSUFFICIENT_GOLD: "game.marketCreateBuyOrderFailed",
  MARKET_BUY_ORDER_CREATE_FAILED: "game.marketCreateBuyOrderFailed",
  MARKET_BUY_ORDER_FILL_INSUFFICIENT_ITEM: "game.marketFillBuyOrderFailedInsufficientItem",
  MARKET_BUY_ORDER_FILL_NOT_ENOUGH: "game.marketFillBuyOrderFailedInsufficientOrderQuantity",
  MARKET_BUY_ORDER_FILL_FAILED: "game.marketFillBuyOrderFailed",
  MARKET_BUY_ORDER_CANCEL_FAILED: "game.marketCancelBuyOrderFailed",
  MARKET_INSUFFICIENT_GOLD: "game.marketBuyFailedInsufficientGold",
  MARKET_INSUFFICIENT_LISTING_QUANTITY: "game.marketBuyFailedInsufficientListingQuantity",
  MARKET_DUPLICATE_IDEMPOTENCY_KEY: "game.marketBuyFailedDuplicateRequest",
  MARKET_INVALID_QUANTITY: "game.marketBuyFailedInvalidQuantity",
  MARKET_INSTANCE_STATE_INVALID: "game.marketBuyFailedInvalidState",
  MARKET_MISSING_IDEMPOTENCY_KEY: "game.marketBuyFailedInvalidRequest",
  MARKET_BUY_FAILED: "game.marketBuyFailedUnexpected",
  MARKET_BUY_ORDER_FILL_INVALID_REQUEST: "game.marketFillBuyOrderFailedInvalidQuantity",
  MARKET_BUY_ORDER_CANCEL_INVALID_STATE: "game.marketCancelBuyOrderFailedInvalidState",
  MARKET_LISTING_CANCEL_INVALID_STATE: "game.marketCancelListingFailedInvalidState",
  MARKET_ITEM_NOT_LISTABLE: "game.marketItemNotListable",
  MARKET_INSTANCE_REQUIRE_AUCTION: "game.marketInstanceRequireAuction",
  MARKET_BUY_ORDER_INVALID_ITEM: "game.marketCreateBuyOrderFailed",
  AUCTION_ITEM_NOT_LISTABLE: "game.auctionItemNotListable",
  AUCTION_BID_TOO_LOW: "game.auctionBidTooLow",
  AUCTION_NOT_ACTIVE: "game.auctionNotActive",
  AUCTION_CANCEL_HAS_BID: "game.auctionCancelHasBid",
  AUCTION_CANCEL_FEE_INSUFFICIENT_GOLD: "game.auctionCancelFeeInsufficientGold",
  AUCTION_BID_INSUFFICIENT_GOLD: "game.auctionBidInsufficientGold",
  AUCTION_BIDS_QUERY_FAILED: "game.auctionBidFailed",
  AUCTION_BID_FAILED: "game.auctionBidFailed",
  AUCTION_CREATE_FAILED: "game.auctionCreateFailed",
  AUCTION_SELLER_HAS_ACTIVE_AUCTION: "game.auctionSellerHasActiveAuction",
  AUCTION_CANCEL_FAILED: "game.auctionCancelFailed",
  AUCTION_NOT_FOUND: "game.auctionNotFound",
  AUCTION_DETAIL_QUERY_FAILED: "game.auctionDetailLoadFailed",
  AUCTION_START_PRICE_TOO_LOW: "game.auctionStartPriceTooLow",
  AUCTION_BUYOUT_PRICE_TOO_LOW: "game.auctionBuyoutPriceTooLow",
  AUCTION_MIN_BID_INCREMENT_INVALID: "game.auctionMinBidIncrementInvalid",
  AUCTION_STACK_ITEM_NOT_ALLOWED: "game.auctionStackItemNotAllowed",
  AUCTION_REPUTATION_TOO_LOW: "game.auctionReputationTooLow",
  SPECIALIZATION_TREE_NOT_FOUND: "game.specialization.error.treeNotFound",
  SPECIALIZATION_TREE_LOCKED: "game.specialization.error.treeLocked",
  SPECIALIZATION_NODE_NOT_FOUND: "game.specialization.error.nodeNotFound",
  SPECIALIZATION_NODE_MAX_RANK: "game.specialization.error.nodeMaxRank",
  SPECIALIZATION_REQUIREMENTS_UNMET: "game.specialization.error.requirementsUnmet",
  SPECIALIZATION_ACTIVE_ROOT_LIMIT: "game.specialization.error.activeRootLimit",
  SPECIALIZATION_EXCLUSIVE_CONFLICT: "game.specialization.error.exclusiveConflict",
  SPECIALIZATION_POINTS_INSUFFICIENT: "game.specialization.error.pointsInsufficient",
  SPECIALIZATION_REVISION_CONFLICT: "game.specialization.error.revisionConflict",
  SPECIALIZATION_RULESET_CONFLICT: "game.specialization.error.rulesetConflict",
  SPECIALIZATION_IMPACT_CONFLICT: "game.specialization.error.impactConflict",
  SPECIALIZATION_IMPACT_CONFIRMATION_REQUIRED: "game.specialization.error.impactConfirmationRequired",
  SPECIALIZATION_RESET_COST_INSUFFICIENT: "game.specialization.error.resetCostInsufficient",
  SPECIALIZATION_IDEMPOTENCY_KEY_REQUIRED: "game.specialization.error.idempotencyRequired",
  SPECIALIZATION_IDEMPOTENCY_CONFLICT: "game.specialization.error.idempotencyConflict",
  SPECIALIZATION_DEFINITION_INVALID: "game.specialization.error.definitionInvalid",
  SPECIALIZATION_MUTATION_FAILED: "game.specialization.operationFailed",
  // Affix Detach
  AFFIX_DETACH_INVALID_PAYLOAD: "game.error.affixDetachInvalidPayload",
  AFFIX_DETACH_INVALID_REQUEST: "game.error.affixDetachInvalidRequest",
  AFFIX_DETACH_INSTANCE_NOT_OWNED: "game.error.unauthorized",
  AFFIX_DETACH_SLOT_NOT_FOUND: "game.error.affixDetachSlotNotFound",
  AFFIX_DETACH_INVALID_ITEM: "game.error.affixDetachInvalidItem",
  AFFIX_DETACH_FAILED: "game.error.affixDetachFailed",
  AFFIX_DETACH_INSUFFICIENT_ITEM: "game.error.actionInsufficientItems",
  AFFIX_DETACH_CARRIER_OCCUPIED: "game.error.affixDetachFailed",
  AFFIX_DETACH_CARRIER_RARITY_NOT_ALLOWED: "game.error.affixDetachFailed",
  AFFIX_DETACH_INSTANCE_INVALID_STATE: "game.error.affixDetachFailed",
  // Affix Attach
  AFFIX_ATTACH_INVALID_PAYLOAD: "game.error.affixAttachInvalidPayload",
  AFFIX_ATTACH_INVALID_REQUEST: "game.error.affixAttachInvalidRequest",
  AFFIX_ATTACH_INSTANCE_NOT_OWNED: "game.error.unauthorized",
  AFFIX_ATTACH_SLOT_NOT_FOUND: "game.error.affixAttachSlotNotFound",
  AFFIX_ATTACH_SLOT_OCCUPIED: "game.error.affixAttachSlotOccupied",
  AFFIX_ATTACH_NO_EMPTY_SLOT: "game.error.affixAttachNoEmptySlot",
  AFFIX_ATTACH_INVALID_ITEM: "game.error.affixAttachInvalidItem",
  AFFIX_ATTACH_AFFIX_NOT_ALLOWED: "game.error.affixAttachAffixNotAllowed",
  AFFIX_ATTACH_EQUIPMENT_MAX_EFFECTIVE_EXCEEDED: "game.error.affixAttachEquipmentMaxEffectiveExceeded",
  AFFIX_ATTACH_CARRIER_RARITY_NOT_ALLOWED: "game.error.affixAttachFailed",
  AFFIX_ATTACH_INSTANCE_INVALID_STATE: "game.error.affixAttachFailed",
  AFFIX_ATTACH_FAILED: "game.error.affixAttachFailed",
  // Affix Enchant
  AFFIX_ENCHANT_INVALID_PAYLOAD: "game.error.affixEnchantInvalidPayload",
  AFFIX_ENCHANT_INVALID_REQUEST: "game.error.affixEnchantInvalidRequest",
  AFFIX_ENCHANT_INSTANCE_NOT_OWNED: "game.error.unauthorized",
  AFFIX_ENCHANT_INSTANCE_INVALID_STATE: "game.error.affixEnchantInstanceInvalidState",
  AFFIX_ENCHANT_SLOT_NOT_FOUND: "game.error.affixEnchantSlotNotFound",
  AFFIX_ENCHANT_SLOT_OCCUPIED: "game.error.affixEnchantSlotOccupied",
  AFFIX_ENCHANT_NO_EMPTY_SLOT: "game.error.affixEnchantNoEmptySlot",
  AFFIX_ENCHANT_INSUFFICIENT_ITEM: "game.error.affixEnchantInsufficientItem",
  AFFIX_ENCHANT_INVALID_ITEM: "game.error.affixEnchantInvalidItem",
  AFFIX_ENCHANT_NO_RARITY_CANDIDATE: "game.error.affixEnchantInvalidItem",
  AFFIX_ENCHANT_EQUIPMENT_MAX_EFFECTIVE_EXCEEDED: "game.error.affixEnchantEquipmentMaxEffectiveExceeded",
  AFFIX_ENCHANT_FAILED: "game.error.affixEnchantFailed",
  // Affix Smelt
  AFFIX_SMELT_INVALID_PAYLOAD: "game.error.affixSmeltFailed",
  AFFIX_SMELT_INVALID_REQUEST: "game.error.affixSmeltFailed",
  AFFIX_SMELT_INVALID_CARRIER: "game.error.affixSmeltInvalidCarrier",
  AFFIX_SMELT_INSTANCE_NOT_OWNED: "game.error.unauthorized",
  AFFIX_SMELT_INSTANCE_INVALID_STATE: "game.error.affixSmeltFailed",
  AFFIX_SMELT_FAILED: "game.error.affixSmeltFailed",
  EQUIP_AFFIX_BODY_MAX_EFFECTIVE_EXCEEDED: "game.error.equipAffixBodyMaxEffectiveExceeded",
  PROFILE_EMAIL_VERIFICATION_REQUIRED: "game.error.emailVerificationRequired",
  // WS Action
  WS_ACTION_NOT_FOUND: "game.error.actionNotFound",
  WS_ACTION_INSUFFICIENT_ITEM: "game.error.actionInsufficientItems",
  WS_FISH_FOOD_INGREDIENT_INVALID: "game.fishFood.ingredientUnavailable",
  WS_ACTION_STATUS_REQUIREMENT_NOT_MET: "game.error.statusRequirementNotMet",
  WS_ACTION_BUILDING_REQUIREMENT_NOT_MET: "game.error.buildingRequirementNotMet",
  WS_ACTION_QUEUE_LIMIT_REACHED: "game.error.actionQueueLimitReached",
  WS_ACTION_INVALID_PAYLOAD: "game.error.wsActionInvalidPayload",
  WS_ACTION_ID_REQUIRED: "game.error.wsActionIdRequired",
  WS_ACTION_INVALID_COUNT: "game.error.wsActionInvalidCount",
  WS_ACTION_QUEUE_ID_REQUIRED: "game.error.wsActionQueueIdRequired",
  WS_ACTION_BATCH_TOO_MANY_STEPS: "game.error.wsActionBatchTooManySteps",
  WS_ACTION_ACTOR_INVALID: "game.error.wsActionActorInvalid",
  WS_ACTION_ACTOR_FORBIDDEN: "game.error.wsActionActorForbidden",
  WS_ACTION_ACCOUNT_BANNED: "game.error.unauthorized",
  WS_ACTION_KITTY_IN_COMBAT: "game.error.wsActionKittyInCombat",
  WS_ACTION_KITTY_IN_LIFE_QUEUE: "game.error.wsActionKittyInLifeQueue",
  WS_FISHING_TRIP_ACTIVE: "game.fishing.tripAlreadyPlanned",
  WS_FISHING_SUPPLY_INSUFFICIENT: "game.fishing.supplyUnavailable",
  WS_FISHING_TACKLE_NOT_EQUIPPED: "game.fishing.tackleNotEquipped",
  WS_FISHING_TACKLE_IN_USE: "game.fishing.tackleInUse",
  WS_FISHING_TACKLE_MISSING: "game.fishing.tackleUnavailable",
  // WS Enhance
  WS_ENHANCE_ARGS_INVALID: "game.error.wsEnhanceArgsInvalid",
  WS_ENHANCE_TARGET_INVALID: "game.error.wsEnhanceTargetInvalid",
  WS_ENHANCE_NOT_IN_INVENTORY: "game.error.wsEnhanceNotInInventory",
  WS_ENHANCE_TEMPLATE_INVALID: "game.error.wsEnhanceTemplateInvalid",
  WS_ENHANCE_OVERFLOW: "game.error.wsEnhanceOverflow",
  // WS Building
  WS_BUILDING_ARGS_INVALID: "game.error.wsBuildingArgsInvalid",
  WS_BUILDING_NOT_FOUND: "game.error.wsBuildingNotFound",
  WS_BUILDING_MAX_LEVEL: "game.buildingMaxed",
  WS_ACTION_CANCEL_FAILED: "game.error.queueCancelFailed",
  WS_ACTION_QUEUE_MOVE_FAILED: "game.error.queueMoveFailed",
  WS_ACTION_QUEUE_RUN_NOW_FAILED: "game.error.queueRunNowFailed",
  // WS Protocol
  WS_REQUEST_ID_REQUIRED: "game.error.wsRequestIdRequired",
  WS_DUPLICATE_REQUEST: "game.error.wsDuplicateRequest",
  WS_REPLAY_GUARD_FAILED: "game.error.wsReplayGuardFailed",
  REQUEST_IDEMPOTENCY_KEY_REQUIRED: "game.error.requestIdempotencyKeyRequired",
  REQUEST_DUPLICATE: "game.error.requestDuplicate",
  REQUEST_REPLAY_GUARD_FAILED: "game.error.requestReplayGuardFailed",
  WS_UNAUTHORIZED: "game.error.unauthorized",
  WS_UNKNOWN_MESSAGE_TYPE: "game.error.wsUnknownMessageType",
  WS_INVALID_MESSAGE_TYPE: "game.error.wsInvalidMessageType",
  WS_INVALID_TOKEN: "game.error.unauthorized",
  WS_SESSION_REVOKED: "game.error.unauthorized",
  AUTH_SESSION_REVOKED: "game.error.unauthorized",
  // WS Chat
  WS_CHAT_TOO_LONG: "game.error.wsChatTooLong",
  WS_CHAT_SEND_FAILED: "game.error.wsChatSendFailed",
  WS_CHAT_CHANNEL_READONLY: "game.error.wsChatChannelReadonly",
  WS_CHAT_CHANNEL_FORBIDDEN: "game.error.wsChatChannelForbidden",
  WS_CHAT_CHANNEL_INVALID: "game.error.wsChatChannelInvalid",
  WS_CHAT_CONTENT_BLOCKED: "game.error.chatContentBlocked",
  // WS Battle Item Bag
  WS_BATTLE_ITEM_BAG_ITEM_NOT_FOUND: "game.error.wsBattleItemBagItemNotFound",
  WS_BATTLE_ITEM_BAG_ITEM_NOT_CARRYABLE: "game.error.wsBattleItemBagItemNotCarryable",
  WS_BATTLE_ITEM_BAG_ITEM_NOT_USABLE: "game.error.wsBattleItemBagItemNotUsable",
  WS_BATTLE_ITEM_BAG_SLOT_INVALID: "game.error.wsBattleItemBagSlotInvalid",
  WS_BATTLE_ITEM_BAG_SLOT_OCCUPIED: "game.error.wsBattleItemBagSlotOccupied",
  WS_BATTLE_ITEM_BAG_KINDS_LIMIT_EXCEEDED: "game.error.wsBattleItemBagKindsLimitExceeded",
  WS_BATTLE_ITEM_BAG_TOTAL_LIMIT_EXCEEDED: "game.error.wsBattleItemBagTotalLimitExceeded",
  WS_BATTLE_ITEM_BAG_INSUFFICIENT_INVENTORY: "game.error.wsBattleItemBagInsufficientInventory",
  WS_BATTLE_ITEM_BAG_LOCKED_IN_BATTLE: "game.error.wsBattleItemBagLockedInBattle",
  // Battle Room
  BATTLE_ROOM_DUNGEON_TEAM_SIZE_EXCEEDED: "game.battleRoomMaxTeamSizeExceeded",
  BATTLE_ROOM_FULL: "game.battleRoomMaxTeamSizeExceeded",
  BATTLE_ROOM_FORBIDDEN: "game.error.battleRoomForbidden",
  BATTLE_ROOM_NOT_FOUND: "game.error.battleRoomNotFound",
  BATTLE_ROOM_NOT_OWNER: "game.error.battleRoomNotOwner",
  BATTLE_ROOM_ALREADY_MEMBER: "game.error.battleRoomAlreadyMember",
  BATTLE_ROOM_ENTRY_COST_INSUFFICIENT: "game.error.battleRoomEntryCostInsufficient",
  BATTLE_ROOM_NAME_INVALID: "game.error.battleRoomNameInvalid",
  BATTLE_ROOM_STATE_INVALID: "game.error.battleRoomStateInvalid",
  BATTLE_ROOM_KICK_SELF_FORBIDDEN: "game.error.battleRoomKickSelfForbidden",
  BATTLE_ROOM_KICK_TARGET_INVALID: "game.error.battleRoomKickTargetInvalid",
  BATTLE_ROOM_DUNGEON_NOT_FOUND: "game.error.battleRoomDungeonNotFound",
  BATTLE_ROOM_INVALID_ID: "game.error.battleRoomInvalidId",
  BATTLE_ROOM_MEMBER_MISSING: "game.error.battleRoomMemberMissing",
  BATTLE_ROOM_NAME_BLOCKED: "game.error.battleRoomNameBlocked",
  // Kitty mode conflict
  KITTY_MODE_CONFLICT_LIFE_QUEUE: "game.error.kittyInCombat",
  KITTY_MODE_CONFLICT_COMBAT: "game.error.kittyInLifeQueue",
  // Cattery Battle Point
  CATTERY_BATTLE_POINT_INSUFFICIENT: "game.error.catteryBattlePointInsufficient",
  CATTERY_BATTLE_POINT_ATTR_NOT_ALLOWED: "game.error.catteryBattlePointAttrNotAllowed",
  CATTERY_BATTLE_POINT_RESET_ITEM_INSUFFICIENT: "game.error.catteryBattlePointResetItemInsufficient",
  CATTERY_BATTLE_POINT_RESET_UNAVAILABLE: "game.error.catteryBattlePointResetUnavailable",
  CATTERY_BATTLE_POINT_INVALID_REQUEST: "game.error.catteryBattlePointInvalidRequest",
  CATTERY_BATTLE_POINT_ALLOCATE_FAILED: "game.error.catteryBattlePointAllocateFailed",
  CATTERY_BATTLE_POINT_GET_FAILED: "game.error.catteryBattlePointGetFailed",
  CATTERY_BATTLE_POINT_RESET_FAILED: "game.error.catteryBattlePointResetFailed",
  // Battle Point (player)
  BATTLE_POINT_INSUFFICIENT: "game.error.battlePointInsufficient",
  BATTLE_POINT_ATTR_NOT_ALLOWED: "game.error.battlePointAttrNotAllowed",
  BATTLE_POINT_RESET_ITEM_INSUFFICIENT: "game.error.battlePointResetItemInsufficient",
  // Cattery
  CATTERY_KITTY_NOT_FOUND: "game.error.catteryKittyNotFound",
  CATTERY_CAPACITY_REACHED: "game.error.catteryCapacityReached",
  CATTERY_NOT_BUILT: "game.error.catteryNotBuilt",
  CATTERY_ADOPT_NAME_INVALID: "game.error.catteryAdoptNameInvalid",
  CATTERY_ADOPT_NAME_BLOCKED: "game.error.adoptNameBlocked",
  CATTERY_KITTY_LEVEL_TOO_LOW: "game.error.catteryKittyLevelTooLow",
  CATTERY_COMMISSION_LIMIT_REACHED: "game.error.catteryCommissionLimitReached",
  CATTERY_COMMISSION_NOT_FOUND: "game.error.catteryCommissionNotFound",
  CATTERY_COMMISSION_NOT_READY: "game.error.catteryCommissionNotReady",
  CATTERY_COMMISSION_NOT_RUNNING: "game.error.catteryCommissionNotRunning",
  CATTERY_COMMISSION_COST_NOT_ENOUGH: "game.error.catteryCommissionCostNotEnough",
  CATTERY_COMMISSION_NOT_VISIBLE: "game.error.catteryCommissionNotVisible",
  CATTERY_KITTY_PERSONALITY_REMOVE_COST_INSUFFICIENT: "game.error.catteryPersonalityRemoveCostInsufficient",
  CATTERY_KITTY_EQUIP_TARGET_NOT_ALLOWED: "game.error.equipTargetNotAllowed",
  CATTERY_KITTY_FEED_INVALID_ITEM: "game.error.kittyFeedInvalidItem",
  // Shop
  SHOP_BUY_INSUFFICIENT_GOLD: "game.error.shopBuyInsufficientGold",
  SHOP_BUY_OFFER_NOT_FOUND: "game.error.shopBuyOfferNotFound",
  SHOP_BUY_INVALID_QUANTITY: "game.error.shopBuyInvalidQuantity",
  SHOP_SELL_INSUFFICIENT_ITEMS: "game.error.shopSellInsufficientItems",
  SHOP_SELL_OFFER_NOT_FOUND: "game.error.shopSellOfferNotFound",
  SHOP_BUY_FAILED: "game.error.shopBuyFailed",
  SHOP_SELL_FAILED: "game.error.shopSellFailed",
  // Equip / Unequip
  EQUIP_FAILED: "game.error.equipFailed",
  EQUIP_TYPE_MISMATCH: "game.error.equipTypeMismatch",
  EQUIP_TYPE_REQUIRED: "game.error.equipTypeRequired",
  EQUIP_ITEM_NOT_FOUND: "game.error.equipItemNotFound",
  EQUIP_FISHING_TACKLE_IN_USE: "game.fishing.tackleInUse",
  CATTERY_KITTY_EQUIP_FISHING_TACKLE_IN_USE: "game.fishing.tackleInUse",
  UNEQUIP_SLOT_EMPTY: "game.error.unequipSlotEmpty",
  UNEQUIP_INSTANCE_NOT_EQUIPPED: "game.error.unequipInstanceNotEquipped",
  UNEQUIP_FISHING_TACKLE_IN_USE: "game.fishing.tackleInUse",
  UNEQUIP_FAILED: "game.error.unequipFailed",
  // Combat Config
  COMBAT_CONFIG_DUPLICATE_SKILL: "game.error.combatConfigDuplicateSkill",
  COMBAT_CONFIG_SLOT_LIMIT: "game.error.combatConfigSlotLimit",
  COMBAT_CONFIG_INVALID_SKILL: "game.error.combatConfigInvalidSkill",
  COMBAT_CONFIG_SKILL_LINEAGE_CONFLICT: "game.error.combatConfigLineageConflictGeneric",
  // Cat Village
  CAT_VILLAGE_TASK_NOT_OPEN: "game.error.catVillageTaskNotOpen",
  CAT_VILLAGE_TASK_ALREADY_REFRESHED: "game.error.catVillageTaskAlreadyRefreshed",
  CAT_VILLAGE_TASK_NOT_DELETABLE: "game.error.catVillageTaskNotDeletable",
  CAT_VILLAGE_TASK_NOT_REFRESHABLE: "game.error.catVillageTaskNotRefreshable",
  CAT_VILLAGE_TASK_SUBMIT_FAILED: "game.error.catVillageTaskSubmitFailed",
  // Collectible
  COLLECTIBLE_USE_INVALID_REQUEST: "game.error.collectibleUseInvalidRequest",
  COLLECTIBLE_LOAD_FAILED: "game.error.collectibleUseFailed",
  COLLECTIBLE_INVALID_TYPE: "game.error.collectibleInvalidType",
  COLLECTIBLE_DEF_NOT_FOUND: "game.error.collectibleDefNotFound",
  COLLECTIBLE_ITEM_MISMATCH: "game.error.collectibleItemMismatch",
  COLLECTIBLE_ADD_FAILED: "game.error.collectibleUseFailed",
  COLLECTIBLE_NOT_FOUND: "game.error.collectibleNotFound",
  COLLECTIBLE_NOT_OWNED: "game.error.collectibleNotOwned",
  COLLECTIBLE_ALREADY_OWNED: "game.error.collectibleAlreadyOwned",
  // Skillbook
  ITEM_USE_SKILLBOOK_INSUFFICIENT: "game.error.itemUseSkillbookInsufficient",
  ITEM_USE_SKILLBOOK_NOT_OWNED: "game.error.itemUseSkillbookNotOwned",
  // Repair
  REPAIR_FAILED: "game.error.repairFailed",
  REPAIR_ITEM_NOT_FOUND: "game.error.repairItemNotFound",
  REPAIR_INSTANCE_NOT_FOUND: "game.error.repairInstanceNotFound",
  REPAIR_INVALID_INSTANCE: "game.error.repairInvalidInstance",
  // Content moderation
  PLAYER_LOADOUT_NAME_BLOCKED: "game.error.loadoutNameBlocked",
  CHAT_CONTENT_BLOCKED: "game.error.chatContentBlocked",
  // Account
  AUTH_LOGIN_LOCKED: "game.error.authLoginLocked",
  ACCOUNT_RESTRICTED: "game.error.accountRestricted",
  ACCOUNT_REVIEW_REQUIRED: "game.error.accountReviewRequired",
  // Existing
  RATE_LIMITED: "game.error.rateLimited",
  RATE_LIMIT_BACKEND_UNAVAILABLE: "game.error.rateLimitBackendUnavailable",
  LOADOUT_EQUIP_ITEM_MISSING: "game.loadoutEquipItemMissing",
  LOADOUT_EQUIP_ITEM_NOT_OWNED: "game.loadoutEquipItemNotOwned",
  LOADOUT_EQUIP_ITEM_STATE_INVALID: "game.loadoutEquipItemStateInvalid",
  LOADOUT_EQUIP_EQUIP_TYPE_MISMATCH: "game.loadoutEquipTypeMismatch",
  LOADOUT_EQUIP_SLOT_NOT_ALLOWED: "game.loadoutEquipSlotNotAllowed",
  LOADOUT_EQUIP_STATUS_REQUIREMENT_NOT_MET: "game.loadoutEquipStatusRequirementNotMet",
  LOADOUT_EQUIP_ACCESSORY_CONFLICT: "game.loadoutEquipAccessoryConflict",
  LOADOUT_EQUIP_AFFIX_BODY_LIMIT: "game.loadoutEquipAffixBodyLimit"
}

function inferErrorI18nKeyByCode(code: string): string | null {
  const normalized = String(code || "").trim().toUpperCase()
  if (!normalized) return null
  if (
    normalized.includes("UNAUTHORIZED") ||
    normalized.includes("INVALID_TOKEN") ||
    normalized.includes("SESSION_REVOKED") ||
    normalized.endsWith("_INSTANCE_NOT_OWNED") ||
    normalized.endsWith("_NOT_OWNED") ||
    normalized.endsWith("_FORBIDDEN")
  ) {
    return "game.error.unauthorized"
  }
  if (normalized.includes("STATUS_REQUIREMENT_NOT_MET")) {
    return "game.error.statusRequirementNotMet"
  }
  if (normalized.includes("BUILDING_REQUIREMENT_NOT_MET")) {
    return "game.error.buildingRequirementNotMet"
  }
  if (
    normalized.includes("INVALID_PAYLOAD") ||
    normalized.includes("INVALID_REQUEST") ||
    normalized.includes("INVALID_") ||
    normalized.endsWith("_FAILED") ||
    normalized.endsWith("_NOT_FOUND") ||
    normalized.endsWith("_NOT_ACTIVE") ||
    normalized.includes("_STATE_INVALID") ||
    normalized.includes("_QUERY_FAILED") ||
    normalized.includes("_LOAD_FAILED") ||
    normalized.includes("_LIST_FAILED") ||
    normalized.includes("_GET_FAILED") ||
    normalized.includes("_SAVE_FAILED") ||
    normalized.includes("_UPDATE_FAILED")
  ) {
    return "game.error.generic"
  }
  return null
}

export function resolveErrorI18nKey(err: unknown, fallbackKey: string): string {
  if (err && typeof err === "object" && "code" in err) {
    const maybeCode = String((err as { code?: string }).code || "")
    if (maybeCode && codeToI18nKey[maybeCode]) {
      return codeToI18nKey[maybeCode]
    }
    const inferred = inferErrorI18nKeyByCode(maybeCode)
    if (inferred) return inferred
  }
  if (err instanceof APIClientError && err.code && codeToI18nKey[err.code]) {
    return codeToI18nKey[err.code]
  }
  if (err instanceof APIClientError && err.code) {
    const inferred = inferErrorI18nKeyByCode(err.code)
    if (inferred) return inferred
  }
  if (err instanceof Error) {
    const maybeCode = err.message.split(":")[0]?.trim()
    if (maybeCode && codeToI18nKey[maybeCode]) {
      return codeToI18nKey[maybeCode]
    }
    const inferred = inferErrorI18nKeyByCode(maybeCode)
    if (inferred) return inferred
  }
  return fallbackKey
}

const loadoutEquipItemCodes = new Set([
  "LOADOUT_EQUIP_STATUS_REQUIREMENT_NOT_MET",
  "LOADOUT_EQUIP_ACCESSORY_CONFLICT",
  "LOADOUT_EQUIP_AFFIX_BODY_LIMIT",
  "LOADOUT_EQUIP_ITEM_NOT_OWNED",
  "LOADOUT_EQUIP_ITEM_STATE_INVALID",
  "LOADOUT_EQUIP_EQUIP_TYPE_MISMATCH",
  "LOADOUT_EQUIP_SLOT_NOT_ALLOWED",
])

const itemContextCodes = new Set([
  ...loadoutEquipItemCodes,
  "WS_FISHING_SUPPLY_INSUFFICIENT",
  "WS_FISHING_TACKLE_NOT_EQUIPPED",
])

export function resolveErrorMessage(
  t: (key: string, params?: Record<string, unknown>) => string,
  err: unknown,
  fallbackKey: string
): string {
  const key = resolveErrorI18nKey(err, fallbackKey)
  const context = extractErrorContext(err)
  const fishingCode = context.code || (err instanceof Error ? err.message.split(":")[0]?.trim() : "") || ""
  const fishingValidation = fishingValidationMessages[fishingCode]
  if (fishingValidation || fishingCode === "WS_FISHING_INVALID" || fishingCode === "FISHING_INVALID") {
    const field = fishingValidation?.fieldKey ? t(fishingValidation.fieldKey) : ""
    return `${t(key, { field })} [${fishingCode}]`
  }
  if (context.affixId && (
    context.code === "AFFIX_ATTACH_EQUIPMENT_MAX_EFFECTIVE_EXCEEDED" ||
    context.code === "AFFIX_ENCHANT_EQUIPMENT_MAX_EFFECTIVE_EXCEEDED" ||
    context.code === "EQUIP_AFFIX_BODY_MAX_EFFECTIVE_EXCEEDED"
  )) {
    return t(key, { affix: localizeAffixName(t, context.affixId) })
  }
  if (context.itemId && itemContextCodes.has(context.code)) {
    return t(key, { item: getLocalizedItemName(t, context.itemId) })
  }
  return t(key)
}

function extractErrorContext(err: unknown): { code: string; affixId: string; itemId: string } {
  if (!err || typeof err !== "object") return { code: "", affixId: "", itemId: "" }
  if (err instanceof APIClientError) {
    const code = String(err.code || "").trim()
    const instanceState = String(err.instanceState || "").trim()
    return {
      code,
      affixId: itemContextCodes.has(code) ? "" : normalizeAffixID(instanceState),
      itemId: itemContextCodes.has(code) ? instanceState : ""
    }
  }
  const payload = err as { code?: string; instanceState?: string }
  const code = String(payload.code || "").trim()
  const instanceState = String(payload.instanceState || "").trim()
  return {
    code,
    affixId: itemContextCodes.has(code) ? "" : normalizeAffixID(instanceState),
    itemId: itemContextCodes.has(code) ? instanceState : ""
  }
}

function normalizeAffixID(raw: string): string {
  const normalized = String(raw || "").trim()
  if (!normalized) return ""
  return normalized.startsWith("affix.") ? normalized.slice("affix.".length) : normalized
}

function localizeAffixName(
  t: (key: string, params?: Record<string, unknown>) => string,
  affixId: string
): string {
  const normalized = normalizeAffixID(affixId)
  if (!normalized) return affixId
  const names = [`affix.${normalized}.name`, `${normalized}.name`]
  for (const key of names) {
    const localized = t(key)
    if (localized !== key && localized.trim() !== "") return localized
  }
  return normalized
}
