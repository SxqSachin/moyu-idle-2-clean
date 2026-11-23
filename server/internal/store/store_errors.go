package store

import (
	"errors"
	"fmt"
	"strings"
)

var ErrDuplicateUser = errors.New("duplicate username")
var ErrDuplicateEmail = errors.New("duplicate email")
var ErrPlayerUsernameInvalid = errors.New("player username invalid")
var ErrPlayerUsernameUnchanged = errors.New("player username unchanged")
var ErrPlayerUsernameConflict = errors.New("player username changed since it was loaded")
var ErrPlayerUsernameRenameCostInvalid = errors.New("player username rename cost invalid")
var ErrInsufficientGold = errors.New("insufficient gold")
var ErrInsufficientItem = errors.New("insufficient item")
var ErrInventoryQuantityOverflow = errors.New("inventory quantity overflow")
var ErrListingNotEnough = errors.New("listing not enough quantity")
var ErrMarketListingNotFound = errors.New("market listing not found")
var ErrMarketListingNotActive = errors.New("market listing not active")
var ErrMarketListingForbidden = errors.New("market listing forbidden")
var ErrMarketItemOrderSideConflict = errors.New("market item opposite side order exists")
var ErrMarketItemOrderPriceConflict = errors.New("market item same side order has different price")
var ErrMarketItemOrderAlreadyExists = errors.New("market item same side order already exists")
var ErrMarketItemNotListable = errors.New("market item not listable")
var ErrBuyOrderNotEnough = errors.New("buy order not enough quantity")
var ErrBadQuantity = errors.New("bad quantity")
var ErrDuplicateIdempotency = errors.New("duplicate idempotency key")
var ErrInstanceNotOwned = errors.New("item instance not owned")
var ErrInvalidInstanceState = errors.New("invalid item instance state")

type InvalidInstanceStateError struct {
	State string
}

func (e *InvalidInstanceStateError) Error() string {
	if e == nil {
		return ErrInvalidInstanceState.Error()
	}
	if e.State == "" {
		return ErrInvalidInstanceState.Error()
	}
	return fmt.Sprintf("%s: %s", ErrInvalidInstanceState.Error(), e.State)
}

func (e *InvalidInstanceStateError) Unwrap() error {
	return ErrInvalidInstanceState
}

var ErrEquipSlotNotFound = errors.New("equip slot not found")
var ErrEquipSlotNotAllowed = errors.New("equip slot not allowed")
var ErrEquipSlotDisabledBySpecialization = errors.New("equip slot disabled by specialization")
var ErrEquipTargetNotAllowed = errors.New("equip target not allowed")
var ErrEquipAccessoryConflict = errors.New("equip accessory conflict")
var ErrCatVillageTaskNotFound = errors.New("cat village task not found")
var ErrCatVillageTaskNotOpen = errors.New("cat village task is not open")
var ErrCatVillageTaskAlreadyRefreshed = errors.New("cat village task already refreshed")
var ErrCatVillageTaskNotDeletable = errors.New("cat village task cannot be deleted")
var ErrCatVillageTaskNotRefreshable = errors.New("cat village task cannot be refreshed")
var ErrCollectibleAlreadyOwned = errors.New("collectible already owned")
var ErrCollectibleNotOwned = errors.New("collectible not owned")
var ErrCollectibleNotUpgradeable = errors.New("collectible not upgradeable")
var ErrCollectibleMaxLevel = errors.New("collectible max level reached")
var ErrShopOfferNotFound = errors.New("shop offer not found")
var ErrInvalidRuleKind = errors.New("invalid rule kind")
var ErrAffixSlotNotFound = errors.New("affix slot not found")
var ErrAffixSlotOccupied = errors.New("affix slot occupied")
var ErrAffixNoEmptySlot = errors.New("affix no empty slot")
var ErrAffixCarrierNotFound = errors.New("affix carrier not found")
var ErrAffixInvalidCarrier = errors.New("invalid affix carrier")
var ErrAffixNotAllowedByEquipment = errors.New("affix not allowed by equipment")
var ErrAffixEnchantRuleNotFound = errors.New("affix enchant rule not found")
var ErrAffixRarityNotAllowed = errors.New("affix rarity not allowed by carrier")
var ErrAffixEnchantNoRarityCandidate = errors.New("affix enchant has no rarity candidate")
var ErrAffixEquipmentMaxEffectiveExceeded = errors.New("affix equipment max effective exceeded")
var ErrAffixBodyMaxEffectiveExceeded = errors.New("affix body max effective exceeded")
var ErrInvalidActionCount = errors.New("invalid action count")
var ErrStatusRequireNotMet = errors.New("status requirements not met")
var ErrBuildingRequireNotMet = errors.New("building requirements not met")
var ErrQueueRunNowNotStarted = errors.New("queue run_now did not enter running")
var ErrActionQueueLaneLimitReached = errors.New("action queue lane limit reached")
var ErrBuildingArgsInvalid = errors.New("building args invalid")
var ErrBuildingNotFound = errors.New("building not found")
var ErrBuildingMaxLevel = errors.New("building max level reached")
var ErrFishingArgsInvalid = errors.New("fishing trip settings invalid")
var ErrFishingSpotNotFound = errors.New("fishing spot not found")
var ErrFishingWaterLocked = errors.New("fishing water is locked")
var ErrFishingLoadoutNotFound = errors.New("fishing loadout not found")
var ErrFishingLoadoutInvalid = errors.New("fishing loadout invalid")
var ErrFishingTackleMissing = errors.New("fishing tackle missing")
var ErrFishingTackleInUse = errors.New("fishing tackle is in use")
var ErrFishingTripActive = errors.New("fishing trip already active")
var ErrFishingTripNotFound = errors.New("fishing trip not found")
var ErrFishingCatchNotFound = errors.New("fishing catch not found")
var ErrFishingSaleInvalid = errors.New("fishing sale invalid")
var ErrFishingSaleQuoteChanged = errors.New("fishing sale quote changed")
var ErrFishingSaleUnavailable = errors.New("fishing sale is not available")
var ErrFishingRewardUnavailable = errors.New("fishing reward unavailable")
var ErrFishingTournamentEntryUnavailable = errors.New("fish is not eligible for the current tournament")
var ErrFishingInfusionUnavailable = errors.New("fish infusion is not available")
var ErrFishingPondNotBuilt = errors.New("fishing pond is not built")
var ErrFishingPondFull = errors.New("fishing pond capacity is full")
var ErrFishingPondBiomassFull = errors.New("fishing pond biomass capacity is full")
var ErrFishingPondFishInvalid = errors.New("fish is not available for the pond")
var ErrFishingPondFeedInvalid = errors.New("fish feed is invalid")
var ErrFishingPondFeederFull = errors.New("fishing pond feeder is full")
var ErrFishingPondPairInvalid = errors.New("fishing pond breeding pair is invalid")
var ErrFishingPondPairFull = errors.New("fishing pond breeding pair slots are full")
var ErrFishingPondLineageReagentMissing = errors.New("fishing pond lineage reagent is missing")
var ErrFishingPondBroodstockMashMissing = errors.New("fishing pond broodstock mash is missing")
var ErrFishingPondRulesetUnsupported = errors.New("fishing pond ruleset is unsupported")
var ErrAdminFishingPondFishInvalid = errors.New("admin fishing pond fish input is invalid")

type FishingSupplyShortageError struct {
	ItemID    string
	Required  int64
	Available int64
}

func (e *FishingSupplyShortageError) Error() string {
	if e == nil {
		return ErrInsufficientItem.Error()
	}
	return fmt.Sprintf("%s: item=%s required=%d available=%d", ErrInsufficientItem.Error(), strings.TrimSpace(e.ItemID), e.Required, e.Available)
}

func (e *FishingSupplyShortageError) Unwrap() error {
	return ErrInsufficientItem
}

type FishingTackleNotEquippedError struct {
	ItemID string
}

func (e *FishingTackleNotEquippedError) Error() string {
	if e == nil || strings.TrimSpace(e.ItemID) == "" {
		return ErrFishingTackleMissing.Error()
	}
	return fmt.Sprintf("%s: item=%s", ErrFishingTackleMissing.Error(), strings.TrimSpace(e.ItemID))
}

func (e *FishingTackleNotEquippedError) Unwrap() error {
	return ErrFishingTackleMissing
}

var ErrActionActorInvalid = errors.New("action actor invalid")
var ErrActionActorForbidden = errors.New("action actor forbidden")
var ErrEnhanceArgsInvalid = errors.New("enhance args invalid")
var ErrEnhanceTargetInvalid = errors.New("enhance target invalid")
var ErrEnhanceNotInInventory = errors.New("enhance target not in inventory")
var ErrEnhanceTemplateInvalid = errors.New("enhance template invalid")
var ErrEnhanceOverflow = errors.New("enhance overflow")
var ErrActionMaterialBindingInvalid = errors.New("action material binding invalid")
var ErrActionMaterialAffixForbidden = errors.New("action material affix forbidden")
var ErrAlchemyLevelInsufficient = errors.New("alchemy level insufficient")
var ErrAlchemyMixedKind = errors.New("alchemy mixed trait kind")
var ErrAlchemyKindMismatch = errors.New("alchemy carrier trait kind mismatch")
var ErrAlchemyCarrierFull = errors.New("alchemy carrier custom affix full")
var ErrAlchemyCarrierOccupied = errors.New("alchemy carrier already has affixes")
var ErrAlchemyMediumUnknown = errors.New("alchemy medium unknown")
var ErrAlchemyTraitSlotOverflow = errors.New("alchemy trait slot overflow")
var ErrAlchemyMaterialOverflow = errors.New("alchemy material overflow")
var ErrAlchemyGroupNotFound = errors.New("alchemy group not found")
var ErrAlchemyBasisInvalid = errors.New("alchemy material basis invalid")
var ErrAlchemyArgsInvalid = errors.New("alchemy args invalid")
var ErrCatteryUnavailable = errors.New("cattery unavailable")
var ErrKittyAdoptLimitReached = errors.New("kitty adopt limit reached")
var ErrKittyNameInvalid = errors.New("kitty name invalid")
var ErrKittyNotFound = errors.New("kitty not found")
var ErrKittyEncounterTargetNotFound = errors.New("kitty encounter target not found")
var ErrKittyCommissionNotFound = errors.New("kitty commission not found")
var ErrKittyCommissionNotReady = errors.New("kitty commission not ready")
var ErrKittyCommissionNotRunning = errors.New("kitty commission not running")
var ErrKittyCommissionLimitReached = errors.New("kitty commission limit reached")
var ErrKittyLevelTooLow = errors.New("kitty level too low")
var ErrKittyModeConflictCombat = errors.New("kitty mode conflict combat")
var ErrKittyModeConflictLifeQueue = errors.New("kitty mode conflict life queue")
var ErrKittyPersonalityInvalid = errors.New("kitty personality invalid")
var ErrKittyPersonalityExists = errors.New("kitty personality already exists")
var ErrKittyPersonalityNotFound = errors.New("kitty personality not found")
var ErrKittyPersonalityCostInvalid = errors.New("kitty personality cost invalid")
var ErrKittyFeedItemInvalid = errors.New("kitty feed item invalid")
var ErrUserNotFound = errors.New("user not found")
var ErrUserBanned = errors.New("user is banned")
var ErrInvalidSkillID = errors.New("invalid skill id")
var ErrSkillExpOverflow = errors.New("skill experience overflow")
var ErrBasicSkillLocked = errors.New("basic skill cannot be revoked")
var ErrMarketItemMustAuction = errors.New("item instance must be auctioned")
var ErrAuctionNotActive = errors.New("auction not active")
var ErrAuctionForbidden = errors.New("auction forbidden")
var ErrAuctionBidTooLow = errors.New("auction bid too low")
var ErrAuctionHasBid = errors.New("auction already has bidder")
var ErrAuctionStartPriceTooLow = errors.New("auction start price too low")
var ErrAuctionBuyoutTooLow = errors.New("auction buyout price too low")
var ErrAuctionMinBidIncrementInvalid = errors.New("auction min bid increment invalid")
var ErrAuctionStackItemNotAllowed = errors.New("auction stack item not allowed")
var ErrAuctionSellerHasActiveListing = errors.New("auction seller has active listing")
var ErrAuctionCancelFeeInsufficientGold = errors.New("auction cancel fee insufficient gold")
var ErrAuctionReputationTooLow = errors.New("auction reputation too low")
var ErrItemNotRecyclable = errors.New("item not recyclable")
var ErrRecyclePriceInvalid = errors.New("recycle price invalid")
var ErrInvalidItemID = errors.New("invalid item id")
var ErrItemRenameInvalidName = errors.New("item rename invalid name")
var ErrItemRenameStackableItem = errors.New("item rename stackable item")
var ErrItemRenameSameName = errors.New("item rename same name")
var ErrItemRenameCostInvalid = errors.New("item rename cost invalid")
var ErrAdminInventoryGrantModeInvalid = errors.New("admin inventory grant mode invalid")
var ErrAdminInventoryGrantEnhanceLevelInvalid = errors.New("admin inventory grant enhance level invalid")
var ErrBattleRoomNotFound = errors.New("battle room not found")
var ErrBattleRoomFull = errors.New("battle room is full")
var ErrBattleRoomForbidden = errors.New("battle room forbidden")
var ErrBattleRoomConfigInvalid = errors.New("battle room config invalid")
var ErrBattleRoomStateInvalid = errors.New("battle room state invalid")
var ErrBattleSettlementRunDetached = errors.New("battle settlement run is detached from room")
var ErrBattleSkillExpJobLeaseLost = errors.New("battle skill experience job lease lost")
var ErrBattleAttrExpJobLeaseLost = errors.New("battle attribute experience job lease lost")
var ErrBattleAttrExpUserBusy = errors.New("battle attribute experience user lock busy")
var ErrBattleRoomAlreadyMember = errors.New("already in battle room")
var ErrBattleRoomMemberMissing = errors.New("battle room member missing")
var ErrBattleRoomNotOwner = errors.New("battle room not owner")
var ErrBattleRoomRequiresSinglePrivate = errors.New("battle room requires single private room")
var ErrBattleRoomTransferTargetInvalid = errors.New("battle room transfer target invalid")
var ErrBattlePointInsufficient = errors.New("battle points insufficient")
var ErrBattlePointAttrNotAllowed = errors.New("battle point attr not allowed")
var ErrBattleItemBagLocked = errors.New("battle item bag is locked during battle")
var ErrBattleItemBagItemNotFound = errors.New("battle item bag item not found")
var ErrBattleItemBagSlotNotFound = errors.New("battle item bag slot not found")
var ErrBattleItemBagSlotInvalid = errors.New("battle item bag slot invalid")
var ErrBattleItemBagSlotOccupied = errors.New("battle item bag slot occupied")
var ErrBattleItemBagInsufficientQuantity = errors.New("battle item bag insufficient quantity")
var ErrBattleItemBagKindsLimitExceeded = errors.New("battle item bag kinds limit exceeded")
var ErrBattleItemBagTotalLimitExceeded = errors.New("battle item bag total quantity limit exceeded")
var ErrBattleSlowRunArchiveNotFound = errors.New("battle slow run archive not found")
var ErrArenaDefenseNotFound = errors.New("arena defense not found")
var ErrArenaDefenseBusy = errors.New("arena defense has active challenges")
var ErrArenaDefenseRevisionChanged = errors.New("arena defense revision changed")
var ErrArenaChallengeNotFound = errors.New("arena challenge not found")
var ErrArenaChallengeForbidden = errors.New("arena challenge forbidden")
var ErrArenaChallengeActive = errors.New("arena attacker already has an active challenge")
var ErrArenaChallengeChargesExhausted = errors.New("arena challenge charges exhausted")
var ErrArenaChallengeOpponentCooldown = errors.New("arena opponent is on challenge cooldown")
var ErrArenaChallengeLeaseLost = errors.New("arena challenge lease lost")
var ErrArenaTokensInsufficient = errors.New("arena tokens insufficient")
var ErrArenaShopOfferNotFound = errors.New("arena shop offer not found")
var ErrArenaReplayShareUnavailable = errors.New("arena replay share unavailable")
var ErrCombatTrialRunNotFound = errors.New("combat trial run not found")
var ErrCombatTrialRunActive = errors.New("combat trial run already active")
var ErrCombatTrialRunLeaseLost = errors.New("combat trial run lease lost")
var ErrCombatTrialReplayUnavailable = errors.New("combat trial replay unavailable")

type AffixLimitExceededError struct {
	AffixID string
	Limit   int
	Scope   string
	cause   error
}

func (e *AffixLimitExceededError) Error() string {
	if e == nil {
		return "affix max effective exceeded"
	}
	if strings.TrimSpace(e.AffixID) == "" || e.Limit <= 0 {
		return e.cause.Error()
	}
	return fmt.Sprintf("%s: affix=%s limit=%d scope=%s", e.cause.Error(), strings.TrimSpace(e.AffixID), e.Limit, strings.TrimSpace(e.Scope))
}

func (e *AffixLimitExceededError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.cause
}

func NewAffixEquipmentMaxEffectiveExceededError(affixID string, limit int) error {
	return &AffixLimitExceededError{
		AffixID: strings.TrimSpace(affixID),
		Limit:   limit,
		Scope:   "equipment",
		cause:   ErrAffixEquipmentMaxEffectiveExceeded,
	}
}

func NewAffixBodyMaxEffectiveExceededError(affixID string, limit int) error {
	return &AffixLimitExceededError{
		AffixID: strings.TrimSpace(affixID),
		Limit:   limit,
		Scope:   "body",
		cause:   ErrAffixBodyMaxEffectiveExceeded,
	}
}

type BattleRoomTeamSizeExceededError struct {
	MemberCount int
	MaxTeamSize int
}

func (e *BattleRoomTeamSizeExceededError) Error() string {
	return fmt.Sprintf("battle room member count %d exceeds max team size %d", e.MemberCount, e.MaxTeamSize)
}

var ErrCosmeticNotOwned = errors.New("cosmetic not owned")
var ErrWardrobeSlotInvalid = errors.New("invalid wardrobe slot")
var ErrCosmeticItemNotFound = errors.New("cosmetic item not found in catalog")
var ErrCosmeticCategoryInvalid = errors.New("invalid cosmetic category")
