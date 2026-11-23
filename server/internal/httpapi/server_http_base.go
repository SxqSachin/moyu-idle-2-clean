package httpapi

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"mysecondwebgame/server/internal/reqmetrics"
	"mysecondwebgame/server/internal/risk"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/websocket/v2"
	"github.com/redis/go-redis/v9"
)

func (s *Server) routes() {
	s.Use(s.requestMetricsMiddleware())
	rateLimitCfg := s.deps.Config.GlobalSettings.RateLimit
	apiLimiter := newIPRateLimiter("api", rateLimitCfg.API.Max, time.Duration(rateLimitCfg.API.WindowSeconds)*time.Second, s.deps.Redis)
	registerLimiter := newIPRateLimiter("register", rateLimitCfg.Register.Max, time.Duration(rateLimitCfg.Register.WindowSeconds)*time.Second, s.deps.Redis)
	loginLimiter := newIPRateLimiter("login", rateLimitCfg.Login.Max, time.Duration(rateLimitCfg.Login.WindowSeconds)*time.Second, s.deps.Redis)
	wsConnectLimiter := newIPRateLimiter("ws_connect", rateLimitCfg.WSConnect.Max, time.Duration(rateLimitCfg.WSConnect.WindowSeconds)*time.Second, s.deps.Redis)
	profileUsernameLimiter := newIPRateLimiter("profile_username", 10, 10*time.Minute, s.deps.Redis)
	profilePasswordLimiter := newIPRateLimiter("profile_password", 8, 10*time.Minute, s.deps.Redis)
	emailVerifySendLimiter := newIPRateLimiter("email_verify_send", 8, 10*time.Minute, s.deps.Redis)
	emailVerifyConfirmLimiter := newIPRateLimiter("email_verify_confirm", 15, 10*time.Minute, s.deps.Redis)
	forgotPasswordSendLimiter := newIPRateLimiter("forgot_password_send", 10, 10*time.Minute, s.deps.Redis)
	forgotPasswordVerifyLimiter := newIPRateLimiter("forgot_password_verify", 15, 10*time.Minute, s.deps.Redis)
	forgotPasswordResetLimiter := newIPRateLimiter("forgot_password_reset", 15, 10*time.Minute, s.deps.Redis)
	settingsUpdateLimiter := newIPRateLimiter("player_settings", 30, time.Minute, s.deps.Redis)
	profileViewLimiter := newIPRateLimiter("player_profile_view", 10, time.Minute, s.deps.Redis)

	api := s.Group("/api", apiLimiter)
	api.Get("/system/version", s.handleSystemVersionGet)
	api.Post("/admin/release/announce", s.releaseAdminMiddleware(), s.handleReleaseAnnounce)
	api.Get("/admin/battle/metrics", s.releaseAdminMiddleware(), s.handleBattleMetricsGet)
	api.Get("/admin/battle/slow-runs", s.releaseAdminMiddleware(), s.handleAdminBattleSlowRunsList)
	api.Get("/admin/battle/slow-runs/:archiveID", s.releaseAdminMiddleware(), s.handleAdminBattleSlowRunGet)
	api.Post("/admin/battle/slow-runs/cleanup", s.releaseAdminMiddleware(), s.handleAdminBattleSlowRunsCleanup)
	api.Post("/admin/battle/runs/cleanup", s.releaseAdminMiddleware(), s.handleAdminBattleRunsCleanup)
	api.Post("/admin/battle/rooms/reinitialize", s.releaseAdminMiddleware(), s.handleAdminBattleRoomsReinitialize)
	api.Post("/admin/arena/defenses/initialize", s.releaseAdminMiddleware(), s.handleAdminArenaDefensesInitialize)
	api.Post("/admin/arena/leaderboard/reset", s.releaseAdminMiddleware(), s.handleAdminArenaLeaderboardReset)
	api.Post("/admin/inventory/grant", s.releaseAdminMiddleware(), s.handleAdminInventoryGrant)
	api.Post("/admin/fishing/pond/fish", s.releaseAdminMiddleware(), s.handleAdminFishingPondFishAdd)
	api.Post("/admin/wardrobe/grant", s.releaseAdminMiddleware(), s.handleAdminWardrobeGrant)
	api.Post("/admin/wardrobe/batch-grant", s.releaseAdminMiddleware(), s.handleAdminWardrobeBatchGrant)
	api.Post("/admin/wardrobe/revoke", s.releaseAdminMiddleware(), s.handleAdminWardrobeRevoke)
	api.Get("/admin/wardrobe/list", s.releaseAdminMiddleware(), s.handleAdminWardrobeList)
	api.Post("/admin/cattery/trigger", s.releaseAdminMiddleware(), s.handleAdminCatteryTrigger)
	api.Post("/admin/cattery/personality/add", s.releaseAdminMiddleware(), s.handleAdminKittyPersonalityAdd)
	api.Post("/admin/cattery/personality/remove", s.releaseAdminMiddleware(), s.handleAdminKittyPersonalityRemove)
	api.Post("/admin/cattery/personality/journal-trigger", s.releaseAdminMiddleware(), s.handleAdminKittyPersonalityJournalTrigger)
	api.Post("/admin/cat-village/commissions/generate", s.releaseAdminMiddleware(), s.handleAdminCatVillageGenerate)
	api.Get("/admin/users/ban-reasons", s.releaseAdminMiddleware(), s.handleAdminUserBanReasons)
	api.Get("/admin/users/banned", s.releaseAdminMiddleware(), s.handleAdminBannedUsers)
	api.Get("/admin/users/:userID", s.releaseAdminMiddleware(), s.handleAdminUserDetail)
	api.Post("/admin/users/ban", s.releaseAdminMiddleware(), s.handleAdminUserBan)
	api.Post("/admin/users/unban", s.releaseAdminMiddleware(), s.handleAdminUserUnban)
	api.Get("/admin/risk/users/:userID", s.releaseAdminMiddleware(), s.handleAdminRiskUserDetail)
	api.Get("/admin/risk/clusters/:clusterID", s.releaseAdminMiddleware(), s.handleAdminRiskClusterDetail)
	api.Get("/admin/risk/clusters", s.releaseAdminMiddleware(), s.handleAdminRiskClusters)
	api.Get("/admin/risk/high-risk-users", s.releaseAdminMiddleware(), s.handleAdminRiskHighRiskUsers)
	api.Get("/admin/risk/search", s.releaseAdminMiddleware(), s.handleAdminRiskSearch)
	api.Get("/admin/risk/review-cases", s.releaseAdminMiddleware(), s.handleAdminRiskReviewCases)
	api.Get("/admin/risk/password-groups", s.releaseAdminMiddleware(), s.handleAdminRiskPasswordGroups)
	api.Post("/admin/risk/clusters/:clusterID/recompute", s.releaseAdminMiddleware(), s.handleAdminRiskClusterRecompute)
	api.Post("/admin/risk/clusters/:clusterID/restrict", s.releaseAdminMiddleware(), s.handleAdminRiskClusterRestrict)
	api.Post("/admin/risk/clusters/:clusterID/release", s.releaseAdminMiddleware(), s.handleAdminRiskClusterRelease)
	api.Post("/admin/risk/clusters/:clusterID/ban", s.releaseAdminMiddleware(), s.handleAdminRiskClusterBan)
	api.Post("/admin/risk/review-cases/:caseID/resolve", s.releaseAdminMiddleware(), s.handleAdminRiskReviewCaseResolve)
	api.Get("/admin/risk/auction-transfers", s.releaseAdminMiddleware(), s.handleAdminRiskAuctionTransfers)
	api.Post("/admin/risk/reset-all", s.releaseAdminMiddleware(), s.handleAdminRiskResetAll)

	s.Get("/healthz", func(c *fiber.Ctx) error {
		if err := readinessStoreHealth(s, c.Context()); err != nil {
			return s.writeAPIError(c, fiber.StatusServiceUnavailable, "HEALTH_CHECK_FAILED", "service unavailable", err.Error())
		}
		return s.writeAPISuccess(c, fiber.StatusOK, fiber.Map{"status": "ok"})
	})
	s.Get("/readyz", func(c *fiber.Ctx) error {
		reasons := s.readinessDegradedReasons(time.Now())
		if len(reasons) > 0 {
			log.Printf("instance readiness degraded path=/readyz reasons=%s node=%s", strings.Join(reasons, ","), s.wsNodeAdvertise)
			return s.writeAPIError(c, fiber.StatusServiceUnavailable, "READINESS_CHECK_FAILED", "service unavailable", strings.Join(reasons, ","))
		}
		return s.writeAPISuccess(c, fiber.StatusOK, fiber.Map{"status": "ready"})
	})

	api.Post("/register", registerLimiter, s.handleRegister)
	api.Post("/login", loginLimiter, s.handleLogin)
	api.Post("/login/ip-verify", loginLimiter, s.handleLoginIPVerify)
	api.Post("/password/forgot/send", forgotPasswordSendLimiter, s.handleForgotPasswordSend)
	api.Post("/password/forgot/verify", forgotPasswordVerifyLimiter, s.handleForgotPasswordVerify)
	api.Post("/password/forgot/reset", forgotPasswordResetLimiter, s.handleForgotPasswordReset)
	api.Get("/geetest/register", s.handleGeetestRegister)
	api.Post("/geetest/validate", s.handleGeetestValidate)
	protected := api.Group("", s.authMiddleware(), s.replayGuardMiddleware())
	marketProtected := protected.Group("/market", s.emailVerifiedMiddleware())
	auctionProtected := protected.Group("/auction", s.emailVerifiedMiddleware())
	protected.Get("/inventory/instances", s.handleInventoryInstances)
	protected.Post("/inventory/instances/archive", s.handleInventoryInstancesArchive)
	protected.Get("/items/detail", s.handleItemDetail)
	protected.Post("/items/rename", s.handleRenameItem)
	protected.Post("/items/use-skillbook", s.handleUseSkillbook)
	protected.Post("/items/forged-skillbook/destroy", s.handleDestroyForgedSkillbook)
	protected.Post("/items/use-collectible", s.handleUseCollectible)
	protected.Post("/items/use-operation-effect", s.handleUseOperationEffect)
	protected.Get("/operation-effects", s.handleOperationEffectsGet)
	protected.Get("/cattery/furniture", s.handleFurnitureGet)
	protected.Post("/cattery/furniture", s.handleFurnitureCommand)
	protected.Get("/cattery/furniture/report", s.handleFurnitureReport)
	protected.Post("/cattery/furniture/report/ack", s.handleFurnitureReport)
	protected.Get("/operation-effects/transmute-eligibility/:itemId", s.handleTransmuteFoodEligibility)
	protected.Get("/collectibles", s.handleGetCollectibles)
	protected.Post("/items/recycle", s.handleRecycleItem)
	protected.Post("/items/recycle/batch", s.handleRecycleItemBatch)
	protected.Post("/items/recycle/destructive/quote", s.handleDestructiveRecycleQuote)
	protected.Post("/items/recycle/destructive", s.handleDestructiveRecycle)
	protected.Get("/player/presence", s.handlePlayerPresence)
	protected.Post("/player/presence", s.handlePlayerPresenceUpdate)
	protected.Get("/player/stats", s.handlePlayerStats)
	protected.Get("/player/specializations/:treeId", s.handlePlayerSpecializationGet)
	protected.Post("/player/specializations/:treeId/allocate", s.handlePlayerSpecializationAllocate)
	protected.Post("/player/specializations/:treeId/reset", s.handlePlayerSpecializationReset)
	protected.Get("/player/public/:userID", profileViewLimiter, s.handlePlayerPublicProfileGet)
	protected.Get("/player/profile/:userID", profileViewLimiter, s.handlePlayerProfileGet)
	protected.Get("/player/profile/:userID/pond", profileViewLimiter, s.handlePlayerProfilePondGet)
	protected.Get("/player/profile/:userID/collectibles", profileViewLimiter, s.handlePlayerProfileCollectiblesGet)
	protected.Get("/player/profile/:userID/achievements", profileViewLimiter, s.handlePlayerProfileAchievementsGet)
	protected.Get("/bootstrap/init", s.handleBootstrapInit)
	protected.Get("/player/settings", s.handlePlayerSettingsGet)
	protected.Post("/player/settings/language", settingsUpdateLimiter, s.handlePlayerSettingsLanguage)
	protected.Post("/player/settings/theme", settingsUpdateLimiter, s.handlePlayerSettingsTheme)
	protected.Post("/player/settings/menu-layout", settingsUpdateLimiter, s.handlePlayerSettingsMenuLayout)
	protected.Post("/player/settings/simple-mode", settingsUpdateLimiter, s.handlePlayerSettingsSimpleMode)
	protected.Post("/player/settings/disable-ip-login-verify", settingsUpdateLimiter, s.handlePlayerSettingsDisableIPLoginVerify)
	protected.Get("/player/tutorial-progress", s.handlePlayerTutorialProgressGet)
	protected.Post("/player/tutorial-progress", settingsUpdateLimiter, s.handlePlayerTutorialProgressUpsert)
	protected.Post("/player/tutorial-progress/reset", settingsUpdateLimiter, s.handlePlayerTutorialProgressReset)
	protected.Get("/public-risk/noticeboard", s.handlePublicRiskNoticeboard)
	protected.Get("/player/loadouts", s.handlePlayerLoadoutsGet)
	protected.Post("/player/loadouts/battle/upsert", s.handlePlayerBattleLoadoutUpsert)
	protected.Post("/player/loadouts/work/upsert", s.handlePlayerWorkLoadoutUpsert)
	protected.Post("/player/loadouts/battle/apply", s.handlePlayerBattleLoadoutApply)
	protected.Post("/player/loadouts/work/apply", s.handlePlayerWorkLoadoutApply)
	protected.Post("/player/loadouts/work/bind-actions", s.handlePlayerWorkLoadoutBindingsUpdate)
	protected.Post("/player/loadouts/:type/delete", s.handlePlayerLoadoutDelete)
	protected.Post("/player/profile/username", profileUsernameLimiter, s.handlePlayerProfileUsername)
	protected.Post("/player/profile/email", profileUsernameLimiter, s.handlePlayerProfileEmail)
	protected.Post("/player/profile/email/verify/send", emailVerifySendLimiter, s.handlePlayerProfileEmailVerifySend)
	protected.Post("/player/profile/email/verify/confirm", emailVerifyConfirmLimiter, s.handlePlayerProfileEmailVerifyConfirm)
	protected.Post("/player/profile/password", profilePasswordLimiter, s.handlePlayerProfilePassword)
	protected.Get("/chat/config", s.handleChatConfig)
	protected.Get("/chat/messages", s.handleChatMessages)
	protected.Get("/wardrobe", s.handleWardrobeGet)
	protected.Post("/wardrobe/equip", s.handleWardrobeEquip)
	protected.Post("/wardrobe/unequip", s.handleWardrobeUnequip)
	protected.Get("/achievements", s.handleAchievementsList)
	protected.Post("/achievements/:achievementID/claim", s.handleAchievementClaim)
	protected.Get("/equipment/instances", s.handleEquipmentInstances)
	protected.Post("/equipment/equip", s.handleEquip)
	protected.Post("/equipment/unequip", s.handleUnequip)
	protected.Post("/equipment/unequip-instance", s.handleUnequipInstance)
	marketProtected.Post("/list", s.handleMarketList)
	marketProtected.Post("/instances/quote", s.handleInstanceMarketQuote)
	marketProtected.Post("/instances/list", s.handleInstanceMarketList)
	marketProtected.Post("/instances/buy", s.handleInstanceMarketBuy)
	marketProtected.Post("/instances/cancel", s.handleInstanceMarketCancel)
	marketProtected.Get("/instances/:view", s.handleInstanceMarketRead)
	marketProtected.Post("/list-instance", s.handleMarketListInstance)
	marketProtected.Post("/buy", s.handleMarketBuy)
	marketProtected.Post("/listings/cancel", s.handleMarketCancelListing)
	marketProtected.Get("/snapshot", s.handleMarketSnapshot)
	marketProtected.Get("/listings", s.handleMarketListings)
	marketProtected.Get("/buy-orders", s.handleMarketBuyOrders)
	marketProtected.Post("/buy-orders", s.handleMarketCreateBuyOrder)
	marketProtected.Post("/buy-orders/fill", s.handleMarketFillBuyOrder)
	marketProtected.Post("/buy-orders/cancel", s.handleMarketCancelBuyOrder)
	auctionProtected.Get("/listings", s.handleAuctionListings)
	auctionProtected.Get("/detail", s.handleAuctionDetail)
	auctionProtected.Get("/bids", s.handleAuctionBids)
	auctionProtected.Post("/list-instance", s.handleAuctionListInstance)
	auctionProtected.Post("/bid", s.handleAuctionBid)
	auctionProtected.Post("/cancel", s.handleAuctionCancel)
	protected.Get("/shop/offers", s.handleShopOffers)
	protected.Post("/shop/buy", s.handleShopBuy)
	protected.Post("/shop/sell", s.handleShopSell)
	marketProtected.Get("/fees/summary", s.handleMarketFeeSummary)
	protected.Post("/rules/evaluate", s.handleRuleEvaluate)
	protected.Get("/rules/config", s.handleRuleConfigList)
	protected.Post("/rules/config", s.handleRuleConfigUpsert)
	protected.Get("/battle/skills/config", s.handleBattleSkillConfigGet)
	protected.Post("/battle/skills/config", s.handleBattleSkillConfigUpdate)
	protected.Post("/battle/skills/equipment-config", s.handleEquipmentSkillConfigUpdate)
	protected.Post("/battle/skills/owned/grant", s.handleBattleOwnedSkillGrant)
	protected.Post("/battle/skills/owned/revoke", s.handleBattleOwnedSkillRevoke)
	protected.Get("/battle/items/bag", s.handleBattleItemBagGet)
	protected.Post("/battle/items/bag/add", s.handleBattleItemBagAdd)
	protected.Post("/battle/items/bag/takeback", s.handleBattleItemBagTakeBack)
	protected.Get("/battle/rooms", s.handleBattleRoomPublicList)
	protected.Get("/battle/worlds", s.handleBattleWorlds)
	protected.Get("/battle/rooms/self", s.handleBattleRoomSelf)
	protected.Post("/battle/rooms", s.handleBattleRoomCreate)
	protected.Get("/battle/rooms/:roomID", s.handleBattleRoomDetail)
	protected.Post("/battle/rooms/:roomID/join", s.handleBattleRoomJoin)
	protected.Post("/battle/rooms/:roomID/leave", s.handleBattleRoomLeave)
	protected.Post("/battle/rooms/:roomID/kick", s.handleBattleRoomKick)
	protected.Post("/battle/rooms/:roomID/owner/transfer", s.handleBattleRoomTransferOwner)
	protected.Post("/battle/rooms/:roomID/config", s.handleBattleRoomUpdateConfig)
	protected.Post("/battle/rooms/:roomID/ready", s.handleBattleRoomReady)
	protected.Post("/battle/rooms/:roomID/position", s.handleBattleRoomPosition)
	protected.Post("/battle/rooms/:roomID/kitty/add", s.handleBattleRoomKittyAdd)
	protected.Post("/battle/rooms/:roomID/kitty/remove", s.handleBattleRoomKittyRemove)
	protected.Post("/battle/rooms/:roomID/end", s.handleBattleRoomInterrupt)
	protected.Get("/battle/rooms/:roomID/run", s.handleBattleRoomRun)
	protected.Get("/battle/points", s.handleBattlePointsGet)
	protected.Post("/battle/points/allocate", s.handleBattlePointsAllocate)
	protected.Post("/battle/points/allocate_batch", s.handleBattlePointsAllocateBatch)
	protected.Post("/battle/points/reset", s.handleBattlePointsReset)
	protected.Get("/arena/overview", s.handleArenaOverview)
	protected.Get("/arena/targets", s.handleArenaTargets)
	protected.Post("/arena/targets/refresh", s.handleArenaTargetsRefresh)
	protected.Get("/arena/leaderboard", s.handleArenaLeaderboard)
	protected.Post("/arena/defense/publish", s.handleArenaDefensePublish)
	protected.Post("/arena/defense/unpublish", s.handleArenaDefenseUnpublish)
	protected.Post("/arena/challenges", s.handleArenaChallengeCreate)
	protected.Get("/arena/challenges", s.handleArenaChallengeHistory)
	protected.Get("/arena/challenges/:challengeID/replay", s.handleArenaChallengeReplay)
	protected.Post("/arena/challenges/:challengeID/share", s.handleArenaChallengeShare)
	protected.Get("/arena/replays/shared/:token", s.handleArenaSharedReplay)
	protected.Get("/arena/shop", s.handleArenaShopGet)
	protected.Post("/arena/shop/buy", s.handleArenaShopBuy)
	protected.Get("/combat-trials/overview", s.handleCombatTrialOverview)
	protected.Post("/combat-trials/runs", s.handleCombatTrialRunCreate)
	protected.Get("/combat-trials/runs", s.handleCombatTrialRuns)
	protected.Get("/combat-trials/leaderboard", s.handleCombatTrialLeaderboard)
	protected.Get("/combat-trials/runs/:runID/replay", s.handleCombatTrialRunReplay)
	protected.Post("/combat-trials/runs/:runID/share", s.handleCombatTrialRunShare)
	protected.Get("/combat-trials/replays/shared/:token", s.handleCombatTrialSharedReplay)
	protected.Post("/equipment/affix/detach", s.handleDetachEquipmentAffix)
	protected.Post("/equipment/affix/attach", s.handleAttachEquipmentAffix)
	protected.Post("/equipment/affix/enchant", s.handleEnchantEquipmentAffix)
	protected.Post("/equipment/affix/smelt", s.handleSmeltCarrierAffixes)
	protected.Get("/alchemy/discoveries", s.handleAlchemyDiscoveries)
	protected.Post("/alchemy/inherit", s.handleAlchemyInheritImmediate)
	protected.Post("/alchemy/bench/fish-preview", s.handleFishingInfusionPreview)
	protected.Get("/alchemy/skill-forge/state", s.handleSkillForgeState)
	protected.Post("/alchemy/skill-forge/drafts", s.handleSkillForgeCreateDraft)
	protected.Put("/alchemy/skill-forge/drafts/:draftId", s.handleSkillForgeUpdateDraft)
	protected.Delete("/alchemy/skill-forge/drafts/:draftId", s.handleSkillForgeDeleteDraft)
	protected.Post("/alchemy/skill-forge/drafts/:draftId/rebase", s.handleSkillForgeRebaseDraft)
	protected.Post("/alchemy/skill-forge/drafts/:draftId/preview", s.handleSkillForgePreview)
	protected.Post("/alchemy/skill-forge/drafts/:draftId/forge", s.handleSkillForgeForge)
	protected.Post("/alchemy/skill-forge/results/:revisionId/accept", s.handleSkillForgeAccept)
	protected.Post("/alchemy/skill-forge/results/:revisionId/discard", s.handleSkillForgeDiscard)
	protected.Post("/alchemy/skill-forge/skills/:skillId/archive", s.handleSkillForgeArchive)
	protected.Post("/equipment/repair", s.handleRepairEquip)
	protected.Get("/equipment/operation-info/:instanceID", s.handleEquipmentOperationInfo)
	protected.Get("/cat-village/commissions", s.handleCatVillageCommissions)
	protected.Post("/cat-village/commissions/:taskID/submit", s.handleCatVillageCommissionSubmit)
	protected.Delete("/cat-village/commissions/:taskID", s.handleCatVillageCommissionDelete)
	protected.Post("/cat-village/commissions/:taskID/refresh", s.handleCatVillageCommissionRefresh)
	protected.Post("/cattery/adopt", s.handleCatteryAdopt)
	protected.Get("/cattery/commissions/catalog", s.handleCatteryCommissionCatalog)
	protected.Get("/cattery/commissions/runs", s.handleCatteryCommissionRuns)
	protected.Post("/cattery/commissions/start", s.handleCatteryCommissionStart)
	protected.Post("/cattery/commissions/claim", s.handleCatteryCommissionClaim)
	protected.Get("/cattery/kitties/:kittyID/combat-profile", s.handleCatteryCombatProfile)
	protected.Get("/cattery/kitties/:kittyID/combat-config", s.handleCatteryCombatConfigGet)
	protected.Post("/cattery/kitties/:kittyID/combat-config", s.handleCatteryCombatConfigUpdate)
	protected.Get("/cattery/kitties/:kittyID/battle-points", s.handleCatteryBattlePointsGet)
	protected.Post("/cattery/kitties/:kittyID/battle-points/allocate_batch", s.handleCatteryBattlePointsAllocateBatch)
	protected.Post("/cattery/kitties/:kittyID/battle-points/reset", s.handleCatteryBattlePointsReset)
	protected.Post("/cattery/kitties/:kittyID/rules/config", s.handleCatterySkillRuleConfigUpdate)
	protected.Post("/cattery/kitties/:kittyID/equip", s.handleCatteryKittyEquip)
	protected.Post("/cattery/kitties/:kittyID/unequip-instance", s.handleCatteryKittyUnequip)
	protected.Post("/cattery/kitties/:kittyID/feed", s.handleCatteryKittyFeed)
	protected.Post("/cattery/kitties/:kittyID/personality/remove", s.handleCatteryKittyPersonalityRemove)
	protected.Post("/cattery/kitties/:kittyID/wardrobe/equip", s.handleKittyWardrobeEquip)
	protected.Post("/cattery/kitties/:kittyID/wardrobe/unequip", s.handleKittyWardrobeUnequip)
	protected.Get("/cattery/journals", s.handleCatteryJournals)
	protected.Get("/cattery/encounters", s.handleCatteryEncounters)
	protected.Get("/cattery/kitty/public", s.handleCatteryPublicKittyGet)
	protected.Get("/actions/catalog", s.handleActionCatalog)
	protected.Post("/actions/queue/batch", s.handleActionQueueBatchEnqueue)
	protected.Get("/fishing/overview", s.handleFishingOverview)
	protected.Get("/fishing/trips/history", s.handleFishingTripHistory)
	protected.Get("/fishing/trips/:tripID/events", s.handleFishingTripEvents)
	protected.Post("/fishing/trips/stop", s.handleFishingTripStop)
	protected.Post("/fishing/starter/claim", s.handleFishingStarterClaim)
	protected.Get("/fishing/loadouts", s.handleFishingLoadoutsList)
	protected.Post("/fishing/loadouts", s.handleFishingLoadoutSave)
	protected.Delete("/fishing/loadouts/:loadoutID", s.handleFishingLoadoutDelete)
	protected.Get("/fishing/catches", s.handleFishingCatchesList)
	protected.Get("/fishing/journal", s.handleFishingJournal)
	protected.Get("/fishing/leaderboard", s.handleFishingLeaderboard)
	protected.Get("/fishing/pond", s.handleFishingPondGet)
	protected.Get("/fishing/pond/available-fish", s.handleFishingPondAvailableFish)
	protected.Post("/fishing/pond/fish", s.handleFishingPondFishAdd)
	protected.Post("/fishing/pond/fish/batch", s.handleFishingPondFishBatchAdd)
	protected.Post("/fishing/pond/fish/:instanceID/remove", s.handleFishingPondFishRemove)
	protected.Post("/fishing/pond/fish/:instanceID/move-to-main", s.handleFishingPondFishMoveToMain)
	protected.Post("/fishing/pond/feed", s.handleFishingPondFeedAdd)
	protected.Post("/fishing/pond/feed/manage", s.handleFishingPondFeedManage)
	protected.Post("/fishing/pond/fish/:instanceID/feed", s.handleFishingPondTargetedFeed)
	protected.Post("/fishing/pond/pairs", s.handleFishingPondPairSet)
	protected.Delete("/fishing/pond/pairs/:slotIndex", s.handleFishingPondPairDelete)
	protected.Post("/fishing/pond/pairs/:slotIndex/mash", s.handleFishingPondMashApply)
	protected.Post("/fishing/pond/fish/:instanceID/reveal", s.handleFishingPondGenomeReveal)
	protected.Post("/fishing/pond/fish/:instanceID/process", s.handleFishingPondFishProcess)
	protected.Post("/fishing/quote", s.handleFishingSaleQuote)
	protected.Post("/fishing/sell", s.handleFishingSale)
	protected.Get("/fishing/tournaments", s.handleFishingTournaments)
	protected.Post("/fishing/tournaments/open/pond", s.handleFishingPondTournamentSubmit)
	protected.Post("/fishing/tournaments/rewards/claim", s.handleFishingTournamentRewardClaim)
	protected.Post("/fishing/tournaments/catch-up/redeem", s.handleFishingTournamentCatchUpRedeem)
	protected.Get("/warehouses/logs", s.handleWarehouseChangeLogs)

	s.Use("/ws", wsConnectLimiter, func(c *fiber.Ctx) error {
		if websocket.IsWebSocketUpgrade(c) {
			token := c.Query("token")
			if token == "" {
				token = extractBearerToken(c.Get("Authorization"))
			}
			if token != "" {
				c.Locals("wsToken", token)
			}
			c.Locals("wsIP", c.IP())
			return c.Next()
		}
		return fiber.ErrUpgradeRequired
	})
	s.Get("/ws", websocket.New(s.handleWS))
}

func (s *Server) requestMetricsMiddleware() fiber.Handler {
	return func(c *fiber.Ctx) error {
		collector := reqmetrics.New(time.Now())
		c.Context().SetUserValue(reqmetrics.ContextKey, collector)
		c.Context().SetUserValue("audit_source", "http:"+c.Path())
		collector.SetBusinessStart(time.Now())
		err := c.Next()
		collector.SetBusinessEnd(time.Now())
		logAPIRequestSlow(c, collector.Snapshot(time.Now()))
		return err
	}
}

func (s *Server) authMiddleware() fiber.Handler {
	return func(c *fiber.Ctx) error {
		userID, err := s.authUserID(c)
		if err != nil {
			if err.Error() == "missing bearer token" {
				return s.writeAPIError(c, fiber.StatusUnauthorized, codeAuthMissingToken, "Missing bearer token", "")
			}
			if err.Error() == "invalid token" {
				return s.writeAPIError(c, fiber.StatusUnauthorized, codeAuthInvalidToken, "Invalid bearer token", "")
			}
			if errors.Is(err, errAuthSessionRevoked) || err.Error() == "session revoked" {
				return s.writeAPIError(c, fiber.StatusUnauthorized, codeAuthSessionRevoked, "Session has been revoked", "")
			}
			if errors.Is(err, errAuthAccountBanned) || err.Error() == "account banned" {
				return s.writeAPIError(c, fiber.StatusForbidden, codeAuthAccountBanned, "Account has been banned", "")
			}
			if errors.Is(err, errAuthBackendBusy) {
				return s.writeAPIError(c, fiber.StatusServiceUnavailable, codeAuthBackendBusy, "auth backend busy", "")
			}
			return s.writeAPIError(c, fiber.StatusUnauthorized, codeAuthInvalidToken, "Invalid bearer token", err.Error())
		}
		c.Locals("authUserID", userID)
		if recoveryBlocksHTTPMutation(c.Method(), c.Path()) {
			ctx, cancel := context.WithTimeout(c.Context(), time.Second)
			pending, recoveryErr := s.deps.Store.HasOfflineRecovery(ctx, userID)
			cancel()
			if recoveryErr != nil || pending {
				return s.writeAPIError(c, fiber.StatusConflict, "OFFLINE_RECOVERY_PENDING", "Offline progress is still settling. Check recovery progress before changing items or actions.", "")
			}
		}
		return c.Next()
	}
}

func recoveryBlocksHTTPMutation(method, path string) bool {
	if method == fiber.MethodGet || method == fiber.MethodHead || method == fiber.MethodOptions {
		return false
	}
	return path != "/api/player/presence" && !strings.HasPrefix(path, "/api/player/settings/") &&
		path != "/api/player/profile/password" && !strings.HasPrefix(path, "/api/player/profile/email") && !strings.HasPrefix(path, "/api/chat/")
}

func (s *Server) replayGuardMiddleware() fiber.Handler {
	return func(c *fiber.Ctx) error {
		if c.Method() != fiber.MethodPost {
			return c.Next()
		}
		// Durable mutation domains persist idempotency keys with their result.
		// Let those records answer retries instead of rejecting them in the
		// short-lived Redis replay guard before the store can return the result.
		if hasDurableHTTPIdempotency(c.Path()) {
			return c.Next()
		}
		userID, err := s.authUserID(c)
		if err != nil {
			return c.Next()
		}
		key := strings.TrimSpace(c.Get("X-Idempotency-Key"))
		if key == "" {
			return s.writeAPIError(c, fiber.StatusBadRequest, "REQUEST_IDEMPOTENCY_KEY_REQUIRED", "missing X-Idempotency-Key", "")
		}
		duplicate, replayErr := s.markReplay(c.Context(), "http:"+c.Path(), userID, key, 2*time.Minute)
		if replayErr != nil {
			return s.writeAPIError(c, fiber.StatusInternalServerError, "REQUEST_REPLAY_GUARD_FAILED", "request replay guard failed", replayErr.Error())
		}
		if duplicate {
			return s.writeAPIError(c, fiber.StatusConflict, "REQUEST_DUPLICATE", "duplicate request", "")
		}
		return c.Next()
	}
}

func hasDurableHTTPIdempotency(path string) bool {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) == 5 && parts[0] == "api" && parts[1] == "player" && parts[2] == "specializations" && strings.TrimSpace(parts[3]) != "" {
		return parts[4] == "allocate" || parts[4] == "reset"
	}
	if len(parts) == 6 && parts[0] == "api" && parts[1] == "alchemy" && parts[2] == "skill-forge" && strings.TrimSpace(parts[4]) != "" {
		switch parts[3] {
		case "drafts":
			return parts[5] == "forge"
		case "results":
			return parts[5] == "accept" || parts[5] == "discard"
		}
	}
	return false
}

func (s *Server) emailVerifiedMiddleware() fiber.Handler {
	return func(c *fiber.Ctx) error {
		rawUserID := c.Locals("authUserID")
		userID, _ := rawUserID.(int64)
		if userID <= 0 {
			var err error
			userID, err = s.authUserID(c)
			if err != nil {
				return s.writeAPIError(c, fiber.StatusUnauthorized, codeAuthInvalidToken, "Invalid bearer token", err.Error())
			}
		}
		settings, err := s.deps.Store.GetUserSettings(c.Context(), userID)
		if err != nil {
			return s.writeAPIError(c, fiber.StatusInternalServerError, codeSettingsReadFailed, "Failed to load player settings", err.Error())
		}
		if !settings.EmailVerified {
			return s.writeAPIError(c, fiber.StatusForbidden, codeProfileEmailVerificationRequired, "Email verification required", "")
		}
		return c.Next()
	}
}

func (s *Server) markReplay(ctx context.Context, scope string, userID int64, key string, ttl time.Duration) (bool, error) {
	key = strings.TrimSpace(key)
	if key == "" || userID <= 0 {
		return false, nil
	}
	if s.deps.Redis == nil {
		return false, errors.New("replay backend unavailable: redis is nil")
	}
	replayKey := fmt.Sprintf("replay:%s:%d:%s", scope, userID, key)
	ok, err := s.deps.Redis.SetNX(ctx, replayKey, "1", ttl).Result()
	if err != nil {
		return false, err
	}
	return !ok, nil
}

func isMutatingWSMessage(typ string) bool {
	switch typ {
	case "chat.send", "chat.global.send", "action.run", "action.enqueue", "action.cancel", "action.queue.move", "action.queue.run_now", "action.offline.rewards.get":
		return true
	default:
		return false
	}
}

var redisFixedWindowScript = redis.NewScript(`
local current = redis.call("INCR", KEYS[1])
if current == 1 then
	redis.call("PEXPIRE", KEYS[1], ARGV[1])
end
local ttl = redis.call("PTTL", KEYS[1])
return { current, ttl }
`)

func newIPRateLimiter(prefix string, max int, window time.Duration, redisClient *redis.Client) fiber.Handler {
	lim := &ipRateLimiter{
		max:    max,
		window: window,
		prefix: prefix,
		redis:  redisClient,
	}
	return func(c *fiber.Ctx) error {
		ip := c.IP()
		now := time.Now()
		limited, retryAfter, err := lim.hit(c.Context(), ip, now)
		if err != nil {
			return writeAPIErrorNoDebug(c, fiber.StatusServiceUnavailable, "RATE_LIMIT_BACKEND_UNAVAILABLE", "rate limit backend unavailable")
		}
		if limited {
			c.Set("Retry-After", strconv.Itoa(int(retryAfter.Seconds())))
			return writeAPIErrorNoDebug(c, fiber.StatusTooManyRequests, "RATE_LIMITED", "rate limit exceeded")
		}
		return c.Next()
	}
}

func (l *ipRateLimiter) hit(ctx context.Context, ip string, _ time.Time) (bool, time.Duration, error) {
	if l.redis == nil {
		return false, 0, errors.New("rate limiter backend unavailable: redis is nil")
	}
	limited, retryAfter, err := l.hitRedis(ctx, ip)
	if err != nil {
		return false, 0, err
	}
	return limited, retryAfter, nil
}

func (l *ipRateLimiter) hitRedis(ctx context.Context, ip string) (bool, time.Duration, error) {
	key := "ratelimit:" + l.prefix + ":" + ip
	result, err := redisFixedWindowScript.Run(ctx, l.redis, []string{key}, l.window.Milliseconds()).Result()
	if err != nil {
		return false, 0, err
	}
	values, ok := result.([]any)
	if !ok || len(values) != 2 {
		return false, 0, fmt.Errorf("unexpected redis fixed-window result")
	}
	count, err := toInt64(values[0])
	if err != nil {
		return false, 0, err
	}
	ttlMS, err := toInt64(values[1])
	if err != nil {
		return false, 0, err
	}
	retryAfter := time.Duration(ttlMS) * time.Millisecond
	if retryAfter < 0 {
		retryAfter = 0
	}
	return int(count) > l.max, retryAfter, nil
}

var redisLoginFailureScript = redis.NewScript(`
local failures = redis.call("INCR", KEYS[1])
if failures == 1 then
	redis.call("PEXPIRE", KEYS[1], ARGV[1])
end
if failures >= tonumber(ARGV[3]) then
	redis.call("SET", KEYS[2], "1", "PX", ARGV[2])
	redis.call("DEL", KEYS[1])
	return 1
end
return 0
`)

func newLoginAttemptGuard(maxFailures int, lockDuration, failureWindow time.Duration, redisClient *redis.Client, prefix string) *loginAttemptGuard {
	return &loginAttemptGuard{
		maxFailures:   maxFailures,
		lockDuration:  lockDuration,
		failureWindow: failureWindow,
		redis:         redisClient,
		prefix:        prefix,
	}
}

func (g *loginAttemptGuard) key(ip, username string) string {
	return ip + ":" + username
}

func (g *loginAttemptGuard) IsLocked(ip, username string, _ time.Time) (bool, time.Duration, error) {
	if g.redis == nil {
		return false, 0, errors.New("login guard backend unavailable: redis is nil")
	}
	locked, retryAfter, err := g.isLockedRedis(ip, username)
	if err != nil {
		return false, 0, err
	}
	return locked, retryAfter, nil
}

func (g *loginAttemptGuard) RecordFailure(ip, username string, _ time.Time) error {
	if g.redis == nil {
		return errors.New("login guard backend unavailable: redis is nil")
	}
	if err := g.recordFailureRedis(ip, username); err != nil {
		return err
	}
	return nil
}

func (g *loginAttemptGuard) Reset(ip, username string) error {
	if g.redis == nil {
		return errors.New("login guard backend unavailable: redis is nil")
	}
	return g.resetRedis(ip, username)
}

func (g *loginAttemptGuard) failureKey(ip, username string) string {
	return "ratelimit:" + g.prefix + ":fail:" + g.key(ip, username)
}

func (g *loginAttemptGuard) lockKey(ip, username string) string {
	return "ratelimit:" + g.prefix + ":lock:" + g.key(ip, username)
}

func (g *loginAttemptGuard) isLockedRedis(ip, username string) (bool, time.Duration, error) {
	ttl, err := g.redis.PTTL(context.Background(), g.lockKey(ip, username)).Result()
	if errors.Is(err, redis.Nil) {
		return false, 0, nil
	}
	if err != nil {
		return false, 0, err
	}
	if ttl <= 0 {
		return false, 0, nil
	}
	return true, ttl, nil
}

func (g *loginAttemptGuard) recordFailureRedis(ip, username string) error {
	_, err := redisLoginFailureScript.Run(
		context.Background(),
		g.redis,
		[]string{g.failureKey(ip, username), g.lockKey(ip, username)},
		g.failureWindow.Milliseconds(),
		g.lockDuration.Milliseconds(),
		g.maxFailures,
	).Result()
	return err
}

func (g *loginAttemptGuard) resetRedis(ip, username string) error {
	_, err := g.redis.Del(context.Background(), g.failureKey(ip, username), g.lockKey(ip, username)).Result()
	return err
}

func toInt64(v any) (int64, error) {
	switch n := v.(type) {
	case int64:
		return n, nil
	case int:
		return int64(n), nil
	case float64:
		return int64(n), nil
	case string:
		out, err := strconv.ParseInt(n, 10, 64)
		if err != nil {
			return 0, err
		}
		return out, nil
	default:
		return 0, fmt.Errorf("unsupported numeric type: %T", v)
	}
}

type registerRequest struct {
	Username             string             `json:"username"`
	Email                string             `json:"email"`
	Password             string             `json:"password"`
	AcceptedTerms        bool               `json:"acceptedTerms"`
	AcceptedTermsVersion string             `json:"acceptedTermsVersion"`
	CaptchaToken         string             `json:"captchaToken"`
	DeviceProfile        risk.DeviceProfile `json:"deviceProfile"`
}

type apiErrorResponse struct {
	OK            bool         `json:"ok"`
	Error         apiErrorBody `json:"error"`
	Data          any          `json:"data,omitempty"`
	TS            int64        `json:"ts"`
	InstanceState string       `json:"instanceState,omitempty"`
	reqmetrics.Cost
	Code    string `json:"code,omitempty"`
	Message string `json:"message,omitempty"`
	Debug   string `json:"debug,omitempty"`
}

type apiErrorBody struct {
	Code          string `json:"code"`
	Message       string `json:"message"`
	MessageKey    string `json:"messageKey,omitempty"`
	Debug         string `json:"debug,omitempty"`
	InstanceState string `json:"instanceState,omitempty"`
}

type apiSuccessResponse struct {
	OK         bool                  `json:"ok"`
	TS         int64                 `json:"ts"`
	Compressed *apiCompressedPayload `json:"compressed,omitempty"`
	reqmetrics.Cost
	Data any `json:"data,omitempty"`
}

type apiCompressedPayload struct {
	Format          string `json:"format"`
	Data            string `json:"data"`
	OriginalBytes   int    `json:"originalBytes,omitempty"`
	CompressedBytes int    `json:"compressedBytes,omitempty"`
}

const compressionFormatZlibBase64 = "zlib-base64"

const (
	codeAuthInvalidPayload               = "AUTH_INVALID_PAYLOAD"
	codeAuthCredentialTooShort           = "AUTH_CREDENTIAL_TOO_SHORT"
	codeAuthUsernameExists               = "AUTH_USERNAME_EXISTS"
	codeAuthHashPasswordFailed           = "AUTH_HASH_PASSWORD_FAILED"
	codeAuthCreateUserFailed             = "AUTH_CREATE_USER_FAILED"
	codeAuthSignTokenFailed              = "AUTH_SIGN_TOKEN_FAILED"
	codeAuthInvalidCredentials           = "AUTH_INVALID_CREDENTIALS"
	codeAuthLookupUserFailed             = "AUTH_LOOKUP_USER_FAILED"
	codeAuthInvalidEmail                 = "AUTH_INVALID_EMAIL"
	codeAuthEmailDomainBlocked           = "AUTH_EMAIL_DOMAIN_BLOCKED"
	codeAuthEmailExists                  = "AUTH_EMAIL_EXISTS"
	codeAuthUsernameBlocked              = "AUTH_USERNAME_BLOCKED"
	codeAuthTermsRequired                = "AUTH_TERMS_REQUIRED"
	codeAuthLoginLocked                  = "AUTH_LOGIN_LOCKED"
	codeAuthLoginGuardUnavailable        = "AUTH_LOGIN_GUARD_UNAVAILABLE"
	codeAuthCaptchaRequired              = "AUTH_CAPTCHA_REQUIRED"
	codeAuthCaptchaInvalid               = "AUTH_CAPTCHA_INVALID"
	codeAuthCaptchaUnavailable           = "AUTH_CAPTCHA_UNAVAILABLE"
	codeAccountRestricted                = "ACCOUNT_RESTRICTED"
	codeAccountActionUnavailable         = "ACCOUNT_ACTION_UNAVAILABLE"
	codeAccountReviewRequired            = "ACCOUNT_REVIEW_REQUIRED"
	codeAuthEmailSendFailed              = "AUTH_EMAIL_SEND_FAILED"
	codeAuthVerifyCodeInvalid            = "AUTH_VERIFY_CODE_INVALID"
	codeAuthVerifyCodeTooMany            = "AUTH_VERIFY_CODE_TOO_MANY_ATTEMPTS"
	codeAuthVerifyCodeExpired            = "AUTH_VERIFY_CODE_EXPIRED"
	codeAuthResetTokenInvalid            = "AUTH_RESET_TOKEN_INVALID"
	codeAuthResetTokenExpired            = "AUTH_RESET_TOKEN_EXPIRED"
	codeAuthResetPasswordFailed          = "AUTH_RESET_PASSWORD_FAILED"
	codeAuthUserNotFound                 = "AUTH_USER_NOT_FOUND"
	codeAuthMissingToken                 = "AUTH_MISSING_TOKEN"
	codeAuthInvalidToken                 = "AUTH_INVALID_TOKEN"
	codeAuthSessionRevoked               = "AUTH_SESSION_REVOKED"
	codeAuthAccountBanned                = "AUTH_ACCOUNT_BANNED"
	codeAuthBackendBusy                  = "AUTH_BACKEND_BUSY"
	codeProfileInvalidPayload            = "PROFILE_INVALID_PAYLOAD"
	codeProfileUsernameTooShort          = "PROFILE_USERNAME_TOO_SHORT"
	codeProfileUsernameTooLong           = "PROFILE_USERNAME_TOO_LONG"
	codeProfileUsernameFormat            = "PROFILE_USERNAME_INVALID_FORMAT"
	codeProfileUsernameExists            = "PROFILE_USERNAME_EXISTS"
	codeProfileUsernameBlocked           = "PROFILE_USERNAME_BLOCKED"
	codeProfileUsernameSame              = "PROFILE_USERNAME_UNCHANGED"
	codeProfileEmailInvalid              = "PROFILE_EMAIL_INVALID"
	codeProfileEmailExists               = "PROFILE_EMAIL_EXISTS"
	codeProfileEmailUnchanged            = "PROFILE_EMAIL_UNCHANGED"
	codeProfileEmailVerifyPending        = "PROFILE_EMAIL_VERIFY_PENDING"
	codeProfileEmailNotPending           = "PROFILE_EMAIL_NOT_PENDING"
	codeProfileEmailVerificationRequired = "PROFILE_EMAIL_VERIFICATION_REQUIRED"
	codeProfilePasswordTooShort          = "PROFILE_PASSWORD_TOO_SHORT"
	codeProfilePasswordSame              = "PROFILE_PASSWORD_SAME_AS_OLD"
	codeProfilePasswordInvalidOld        = "PROFILE_PASSWORD_INVALID_CURRENT"
	codeProfileUpdateFailed              = "PROFILE_UPDATE_FAILED"
	codeSettingsInvalidLanguage          = "SETTINGS_INVALID_LANGUAGE"
	codeSettingsInvalidTheme             = "SETTINGS_INVALID_THEME"
	codeSettingsInvalidMenuLayout        = "SETTINGS_INVALID_MENU_LAYOUT"
	codeSettingsReadFailed               = "SETTINGS_READ_FAILED"
	codeSettingsUpdateFailed             = "SETTINGS_UPDATE_FAILED"
	codeLoginIPVerifyRequired            = "LOGIN_IP_VERIFY_REQUIRED"
	codeLoginIPPendingExpired            = "LOGIN_IP_PENDING_EXPIRED"
	codeChatContentBlocked               = "CHAT_CONTENT_BLOCKED"
	codeCatteryAdoptNameBlocked          = "CATTERY_ADOPT_NAME_BLOCKED"
	codeBattleRoomNameBlocked            = "BATTLE_ROOM_NAME_BLOCKED"
	codePlayerLoadoutNameBlocked         = "PLAYER_LOADOUT_NAME_BLOCKED"
	codeItemRenameNameBlocked            = "ITEM_RENAME_NAME_BLOCKED"
)

func (s *Server) isDevEnv() bool {
	return strings.EqualFold(s.deps.Config.AppEnv, "development")
}

func (s *Server) writeAPIError(c *fiber.Ctx, status int, code string, message string, debug string, instanceState ...string) error {
	return s.writeAPIErrorWithMessageKey(c, status, code, message, "", debug, instanceState...)
}

func (s *Server) writeAPIErrorWithMessageKey(c *fiber.Ctx, status int, code string, message string, messageKey string, debug string, instanceState ...string) error {
	return s.writeAPIErrorResponse(c, status, code, message, messageKey, debug, nil, instanceState...)
}

func (s *Server) writeAPIErrorWithData(c *fiber.Ctx, status int, code string, message string, debug string, data any) error {
	return s.writeAPIErrorResponse(c, status, code, message, "", debug, data)
}

func (s *Server) writeAPIErrorResponse(c *fiber.Ctx, status int, code string, message string, messageKey string, debug string, data any, instanceState ...string) error {
	now := time.Now()
	state := ""
	if len(instanceState) > 0 {
		state = instanceState[0]
	}
	body := apiErrorBody{
		Code:          code,
		Message:       message,
		MessageKey:    strings.TrimSpace(messageKey),
		InstanceState: state,
	}
	if s.isDevEnv() && debug != "" {
		body.Debug = debug
	}
	resp := apiErrorResponse{
		OK:            false,
		Error:         body,
		Data:          data,
		TS:            now.UnixMilli(),
		InstanceState: body.InstanceState,
		Cost:          requestCostSnapshot(c, now),
		Code:          body.Code,
		Message:       body.Message,
		Debug:         body.Debug,
	}
	return c.Status(status).JSON(resp)
}

func (s *Server) writeAPISuccess(c *fiber.Ctx, status int, data any) error {
	now := time.Now()
	resp := apiSuccessResponse{
		OK:         true,
		TS:         now.UnixMilli(),
		Cost:       requestCostSnapshot(c, now),
		Data:       data,
		Compressed: nil,
	}
	return c.Status(status).JSON(resp)
}

func (s *Server) writeAPISuccessMaybeCompressed(c *fiber.Ctx, status int, data any, minimumBytes int) error {
	compressionCfg := s.deps.Config.GlobalSettings.Transport.Compression
	if !compressionCfg.Enabled {
		return s.writeAPISuccess(c, status, data)
	}
	raw, err := json.Marshal(data)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusInternalServerError, "API_RESPONSE_MARSHAL_FAILED", "marshal response failed", err.Error())
	}
	threshold := compressionCfg.MinBytes
	if minimumBytes > 0 {
		threshold = minimumBytes
	}
	if threshold <= 0 || len(raw) < threshold {
		return s.writeAPISuccess(c, status, data)
	}
	compressed, err := compressRawZlib(raw, compressionCfg.Level)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusInternalServerError, "API_RESPONSE_COMPRESS_FAILED", "compress response failed", err.Error())
	}
	now := time.Now()
	resp := apiSuccessResponse{
		OK:   true,
		TS:   now.UnixMilli(),
		Cost: requestCostSnapshot(c, now),
		Compressed: &apiCompressedPayload{
			Format:          compressionFormatZlibBase64,
			Data:            base64.StdEncoding.EncodeToString(compressed),
			OriginalBytes:   len(raw),
			CompressedBytes: len(compressed),
		},
	}
	return c.Status(status).JSON(resp)
}

func writeAPIErrorNoDebug(c *fiber.Ctx, status int, code string, message string) error {
	now := time.Now()
	resp := apiErrorResponse{
		OK: false,
		Error: apiErrorBody{
			Code:    code,
			Message: message,
		},
		TS:      now.UnixMilli(),
		Cost:    requestCostSnapshot(c, now),
		Code:    code,
		Message: message,
	}
	return c.Status(status).JSON(resp)
}

func requestCostSnapshot(c *fiber.Ctx, now time.Time) reqmetrics.Cost {
	if c == nil {
		return reqmetrics.Cost{}
	}
	collector := reqmetrics.FromContext(c.Context())
	if collector == nil {
		return reqmetrics.Cost{}
	}
	return collector.Snapshot(now)
}
