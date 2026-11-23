package main

import (
	"context"
	"log"
	"time"

	"mysecondwebgame/server/internal/achievement"
	"mysecondwebgame/server/internal/app"
	"mysecondwebgame/server/internal/auth"
	"mysecondwebgame/server/internal/catalog"
	"mysecondwebgame/server/internal/config"
	"mysecondwebgame/server/internal/discovery"
	"mysecondwebgame/server/internal/emailpolicy"
	"mysecondwebgame/server/internal/game"
	"mysecondwebgame/server/internal/gamedata"
	"mysecondwebgame/server/internal/httpapi"
	"mysecondwebgame/server/internal/market"
	"mysecondwebgame/server/internal/moderation"
	"mysecondwebgame/server/internal/mq"
	"mysecondwebgame/server/internal/reqmetrics"
	"mysecondwebgame/server/internal/risk"
	"mysecondwebgame/server/internal/riskinsight"
	"mysecondwebgame/server/internal/scheduler"
	"mysecondwebgame/server/internal/store"
	"mysecondwebgame/server/internal/ws"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}
	ctx := context.Background()

	dbConfig, err := pgxpool.ParseConfig(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("parse postgres config: %v", err)
	}
	dbConfig.ConnConfig.Tracer = reqmetrics.NewPGXQueryTracer()
	db, err := pgxpool.NewWithConfig(ctx, dbConfig)
	if err != nil {
		log.Fatalf("connect postgres: %v", err)
	}
	defer db.Close()
	if err := store.CheckOfflineRecoverySchema(ctx, db); err != nil {
		log.Fatal(err)
	}

	authDBConfig, err := pgxpool.ParseConfig(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("parse auth postgres config: %v", err)
	}
	authDBConfig.MaxConns = cfg.AuthDBPoolMaxConns
	authDBConfig.ConnConfig.Tracer = reqmetrics.NewPGXQueryTracer()
	authDB, err := pgxpool.NewWithConfig(ctx, authDBConfig)
	if err != nil {
		log.Fatalf("connect auth postgres: %v", err)
	}
	defer authDB.Close()

	rd := redis.NewClient(&redis.Options{
		Addr:     cfg.RedisAddr,
		Password: cfg.RedisPassword,
		DB:       cfg.RedisDB,
	})
	rd.AddHook(reqmetrics.NewRedisHook())
	if err := rd.Ping(ctx).Err(); err != nil {
		log.Fatalf("connect redis: %v", err)
	}
	defer rd.Close()
	var nodeRegistry *discovery.RedisNodeRegistry
	if cfg.NodeAdvertiseAddr != "" {
		nodeRegistry, err = discovery.NewRedisNodeRegistry(
			rd,
			cfg.NodeRegistryPrefix,
			cfg.NodeAdvertiseAddr,
			time.Duration(cfg.NodeHeartbeatSec)*time.Second,
			time.Duration(cfg.NodeHeartbeatIntSec)*time.Second,
		)
		if err != nil {
			log.Fatalf("init redis node registry: %v", err)
		}
		if err := nodeRegistry.Start(ctx); err != nil {
			log.Fatalf("start redis node registry: %v", err)
		}
		defer func() {
			cleanupCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			if err := nodeRegistry.Stop(cleanupCtx); err != nil {
				log.Printf("stop redis node registry: %v", err)
			}
		}()
		log.Printf("node registry enabled addr=%s ttl=%ds interval=%ds", cfg.NodeAdvertiseAddr, cfg.NodeHeartbeatSec, cfg.NodeHeartbeatIntSec)
	}
	publisher, err := mq.NewNatsPublisher(cfg.NATSURL, cfg.MQTopicPrefix)
	if err != nil {
		log.Fatalf("connect nats: %v", err)
	}
	defer publisher.Close()

	gameData, err := catalog.Load(cfg.GameDataPath)
	if err != nil {
		log.Fatalf("load game data: %v", err)
	}
	catalogItemIDs := catalogItemIDSet(gameData)
	if err := config.ValidatePlayerUsernameRenameItemID(cfg.SharedGlobalSettings.PlayerUsernameRename, catalogItemIDs); err != nil {
		log.Fatalf("validate player username rename config item: %v", err)
	}
	if err := config.ValidateKittySettingsItemIDs(cfg.KittySettings, catalogItemIDs); err != nil {
		log.Fatalf("validate kitty config items: %v", err)
	}
	if err := config.ValidateKittySettingsRarityIDs(cfg.KittySettings, catalogRarityIDSet(gameData)); err != nil {
		log.Fatalf("validate kitty config rarities: %v", err)
	}
	kittyPersonalityRows := gamedata.KittyPersonalityConfigsFromCatalog(gameData.KittyPersonalities)
	if len(kittyPersonalityRows) == 0 {
		log.Fatalf("invalid game data: kitty_personalities is empty")
	}
	kittyDefaultPersonalityID := gamedata.ResolveDefaultKittyPersonalityID(cfg.SharedGlobalSettings.Kitty.DefaultPersonalityID, kittyPersonalityRows)
	derivedFormulas := make([]store.DerivedStatFormula, 0, len(cfg.GlobalSettings.DerivedStats.Formulas))
	for _, formula := range cfg.GlobalSettings.DerivedStats.Formulas {
		terms := make([]store.DerivedStatFormulaTerm, 0, len(formula.Terms))
		for _, term := range formula.Terms {
			terms = append(terms, store.DerivedStatFormulaTerm{
				AttrKey:     term.AttrKey,
				Coefficient: term.Coefficient,
			})
		}
		derivedFormulas = append(derivedFormulas, store.DerivedStatFormula{
			TargetAttr:  formula.TargetAttr,
			IncludeBase: formula.IncludeBase,
			Terms:       terms,
		})
	}

	st := store.New(db, store.RuntimeConfig{
		WarehouseDefaultCols:                     cfg.GlobalSettings.Warehouse.DefaultCols,
		WarehouseDefaultRows:                     cfg.GlobalSettings.Warehouse.DefaultRows,
		WarehouseLogDefaultLookback:              time.Duration(cfg.GlobalSettings.Warehouse.LogDefaultLookbackDays) * 24 * time.Hour,
		OfflineProgressMaxDuration:               time.Duration(cfg.GlobalSettings.Activity.OfflineProgressMaxDurationSeconds) * time.Second,
		OfflineSettleExecBudget:                  time.Duration(cfg.GlobalSettings.ActionQueue.FastForward.MaxExecMs) * time.Millisecond,
		ActionQueueFastForwardMaxSteps:           cfg.GlobalSettings.ActionQueue.FastForward.MaxSteps,
		OrdinarySettlementEnabled:                cfg.OrdinarySettlementEnabled,
		ActionQueueFastForwardMaxExec:            time.Duration(cfg.GlobalSettings.ActionQueue.FastForward.MaxExecMs) * time.Millisecond,
		ActionQueueLaneLimit:                     int(cfg.SharedGlobalSettings.Action.QueueLaneLimit),
		KittyActionQueueLaneLimit:                int(cfg.SharedGlobalSettings.Action.KittyQueueLaneLimit),
		KittyStatusExpMultiplier:                 cfg.SharedGlobalSettings.Kitty.StatusExpMultiplier,
		KittyBattleEquipSlots:                    append([]string(nil), cfg.SharedGlobalSettings.Kitty.BattleEquipSlots...),
		KittyBattleAttrExpMultiplier:             cfg.SharedGlobalSettings.Kitty.BattleAttrExpMultiplier,
		KittyBattleSkillExpMultiplier:            cfg.SharedGlobalSettings.Kitty.BattleSkillExpMultiplier,
		KittyBattleLevelCapDefault:               cfg.SharedGlobalSettings.Kitty.BattleLevelCapDefault,
		KittyBattleLevelCapPerKittyLevel:         cfg.SharedGlobalSettings.Kitty.BattleLevelCapPerKittyLevel,
		KittyMaxPersonalities:                    cfg.SharedGlobalSettings.Kitty.MaxPersonalities,
		KittyRemovePersonalityItemID:             cfg.SharedGlobalSettings.Kitty.RemovePersonalityItemID,
		KittyRemovePersonalityCount:              cfg.SharedGlobalSettings.Kitty.RemovePersonalityCount,
		KittyDefaultPersonalityID:                kittyDefaultPersonalityID,
		KittyPersonalityProfiles:                 gamedata.KittyPersonalityProfileMap(kittyPersonalityRows),
		KittyPersonalityOrder:                    gamedata.KittyPersonalityIDOrder(kittyPersonalityRows),
		KittyMaxLevel:                            cfg.KittySettings.Leveling.MaxLevel,
		KittyLevelExp:                            kittyLevelExpMap(cfg.KittySettings),
		KittyLevelExpBase:                        cfg.KittySettings.Leveling.FallbackFormula.Base,
		KittyLevelExpPerLevel:                    cfg.KittySettings.Leveling.FallbackFormula.PerLevel,
		KittyLevelExpPower:                       cfg.KittySettings.Leveling.FallbackFormula.Power,
		AuctionFeeRateBps:                        cfg.SharedGlobalSettings.Auction.FeeRateBps,
		AuctionMinBidIncrement:                   cfg.SharedGlobalSettings.Auction.MinBidIncrement,
		AuctionAntiSnipeTriggerSeconds:           cfg.SharedGlobalSettings.Auction.AntiSnipeTriggerSeconds,
		AuctionAntiSnipeExtendSeconds:            cfg.SharedGlobalSettings.Auction.AntiSnipeExtendSeconds,
		AuctionMinRepLevel:                       cfg.SharedGlobalSettings.Auction.MinRepLevel,
		ItemRenameCost:                           cfg.SharedGlobalSettings.ItemRename.Cost,
		ItemRenameMaxNameRunes:                   cfg.SharedGlobalSettings.ItemRename.MaxNameRunes,
		ItemRenameLabelColors:                    cfg.SharedGlobalSettings.ItemRename.LabelColors,
		PlayerUsernameRenameCostItemID:           cfg.SharedGlobalSettings.PlayerUsernameRename.CostItemID,
		PlayerUsernameRenameCostItemCount:        cfg.SharedGlobalSettings.PlayerUsernameRename.CostItemCount,
		ActionEnhanceInheritanceRatio:            cfg.SharedGlobalSettings.Action.EnhanceInheritanceRatio,
		ItemRecycleGoldCoefficient:               cfg.GlobalSettings.Economy.ItemRecycleGoldCoefficient,
		ItemRecycleAffixMismatchExtraCoefficient: cfg.GlobalSettings.Economy.ItemRecycleAffixMismatchExtraCoefficient,
		SkillLevelUpBaseExp:                      cfg.GlobalSettings.Skill.LevelUpBaseExp,
		SkillLevelUpPerLevelExtraExp:             cfg.GlobalSettings.Skill.LevelUpPerLevelExtraExp,
		SkillForge:                               cfg.GlobalSettings.SkillForge,
		AchievementsEnabled:                      cfg.AchievementsEnabled,
		CatVillageRepLevels:                      cfg.CatVillageSettings.RepLevels,
		Battle: store.BattleRuntimeConfig{
			DropLuckFactor:        cfg.GlobalSettings.Battle.DropLuckFactor,
			ExpBonusStart:         time.Duration(cfg.GlobalSettings.Battle.ExpBonusStartSeconds) * time.Second,
			ExpBonusPerSecond:     cfg.GlobalSettings.Battle.ExpBonusPerSecond,
			ExpBonusMaxMultiplier: cfg.GlobalSettings.Battle.ExpBonusMaxMultiplier,
			ResetItemID:           cfg.GlobalSettings.Battle.ResetItemID,
			ResetItemCount:        cfg.GlobalSettings.Battle.ResetItemCount,
		},
		DerivedStatsFormula: store.DerivedStatsFormulaConfig{
			Version:   cfg.GlobalSettings.DerivedStats.Version,
			Operators: append([]string(nil), cfg.GlobalSettings.DerivedStats.Operators...),
			Formulas:  derivedFormulas,
		},
		Alchemy: store.AlchemyRuntimeConfig{
			InfusionBaseGoldCost:           cfg.SharedGlobalSettings.Alchemy.InfusionBaseGoldCost,
			InfusionBaseDurationSec:        cfg.SharedGlobalSettings.Alchemy.InfusionBaseDurationSec,
			InfusionBaseExp:                cfg.SharedGlobalSettings.Alchemy.InfusionBaseExp,
			DurationDefaultSec:             cfg.SharedGlobalSettings.Alchemy.DurationsSec.Default,
			DurationConvertSec:             cfg.SharedGlobalSettings.Alchemy.DurationsSec.Convert,
			DurationBenchSec:               cfg.SharedGlobalSettings.Alchemy.DurationsSec.Bench,
			DurationTransmuteSec:           cfg.SharedGlobalSettings.Alchemy.DurationsSec.Transmute,
			DurationSmeltSec:               cfg.SharedGlobalSettings.Alchemy.DurationsSec.Smelt,
			TransmuteDurationByRarityIndex: append([]int(nil), cfg.SharedGlobalSettings.Alchemy.TransmuteDurationByRarityIndex...),
			LevelDeficitFailChancePerLevel: cfg.SharedGlobalSettings.Alchemy.LevelDeficitFailChancePerLevel,
			LevelDeficitFailChanceCap:      cfg.SharedGlobalSettings.Alchemy.LevelDeficitFailChanceCap,
			Inherit: store.AlchemyInheritRuntimeConfig{
				BaseInheritSlots:             cfg.SharedGlobalSettings.Alchemy.Inherit.BaseInheritSlots,
				BaseSuppressSlots:            cfg.SharedGlobalSettings.Alchemy.Inherit.BaseSuppressSlots,
				InheritSlotPrecisionScale:    cfg.SharedGlobalSettings.Alchemy.Inherit.InheritSlotPrecisionScale,
				SuppressSlotStabilityBonus:   cfg.SharedGlobalSettings.Alchemy.Inherit.SuppressSlotStabilityBonus,
				PositiveIgnoreKeepChanceBase: cfg.SharedGlobalSettings.Alchemy.Inherit.PositiveIgnoreKeepChanceBase,
				NegativeIgnoreKeepChanceBase: cfg.SharedGlobalSettings.Alchemy.Inherit.NegativeIgnoreKeepChanceBase,
				HybridMergeDivisor:           cfg.SharedGlobalSettings.Alchemy.Inherit.HybridMergeDivisor,
				InbreedMutationChance:        cfg.SharedGlobalSettings.Alchemy.Inherit.InbreedMutationChance,
				InbreedGoodChance:            cfg.SharedGlobalSettings.Alchemy.Inherit.InbreedGoodChance,
				InbreedGoodMultiplierRange:   catalog.AlchemyInheritMultiplierRange{Min: cfg.SharedGlobalSettings.Alchemy.Inherit.InbreedGoodMultiplierRange.Min, Max: cfg.SharedGlobalSettings.Alchemy.Inherit.InbreedGoodMultiplierRange.Max},
				InbreedBadMultiplierRange:    catalog.AlchemyInheritMultiplierRange{Min: cfg.SharedGlobalSettings.Alchemy.Inherit.InbreedBadMultiplierRange.Min, Max: cfg.SharedGlobalSettings.Alchemy.Inherit.InbreedBadMultiplierRange.Max},
				MutationChance:               cfg.SharedGlobalSettings.Alchemy.Inherit.MutationChance,
				NumericValueCapMultiplier:    cfg.SharedGlobalSettings.Alchemy.Inherit.NumericValueCapMultiplier,
			},
		},
		Fishing: cfg.GlobalSettings.Fishing,
	}, gameData)
	authGate := store.NewAuthGateStore(authDB)
	if err := st.BindFurnitureInvalidations(publisher); err != nil {
		log.Fatalf("subscribe furniture cache: %v", err)
	}
	hub := ws.NewHub()
	engine := scheduler.EngineType(cfg.SchedulerEngine)
	sch := scheduler.New(scheduler.Options{
		Engine:     engine,
		ShardCount: cfg.SchedulerShards,
		Tick:       time.Duration(cfg.SchedulerTickMS) * time.Millisecond,
		WheelSize:  cfg.SchedulerWheelSize,
	})
	gm := game.NewService(st, sch)
	au := auth.New(cfg.JWTSecret, 24*time.Hour)
	priceSteps := make([]market.PriceStep, 0, len(cfg.SharedGlobalSettings.Market.PriceSteps))
	for _, step := range cfg.SharedGlobalSettings.Market.PriceSteps {
		priceSteps = append(priceSteps, market.PriceStep{
			Min:  step.Min,
			Max:  step.Max,
			Step: step.Step,
		})
	}
	mk := market.NewService(st, publisher, gameData, market.ServiceOptions{
		MarketPriceSteps: priceSteps,
	})
	riskSvc := risk.NewService(st, cfg.RiskPasswordPepper)
	riskInsightSvc := riskinsight.NewService(st, rd)
	moderationSvc, err := moderation.New(moderation.Config{
		Enabled:            cfg.GlobalSettings.ContentModeration.Enabled,
		VocabularyJSONPath: cfg.GlobalSettings.ContentModeration.VocabularyJSONPath,
		BlockMessageKey:    cfg.GlobalSettings.ContentModeration.BlockMessageKey,
		Targets: moderation.TargetConfig{
			Chat:           cfg.GlobalSettings.ContentModeration.Targets.Chat,
			Username:       cfg.GlobalSettings.ContentModeration.Targets.Username,
			KittyName:      cfg.GlobalSettings.ContentModeration.Targets.KittyName,
			BattleRoomName: cfg.GlobalSettings.ContentModeration.Targets.BattleRoomName,
			LoadoutName:    cfg.GlobalSettings.ContentModeration.Targets.LoadoutName,
			ItemCustomName: cfg.GlobalSettings.ContentModeration.Targets.ItemCustomName,
		},
	})
	if err != nil {
		log.Fatalf("init content moderation: %v", err)
	}
	emailPolicySvc, err := emailpolicy.New(emailpolicy.Config{
		Enabled:        cfg.GlobalSettings.EmailDomainPolicy.Enabled,
		BlocklistPath:  cfg.GlobalSettings.EmailDomainPolicy.BlocklistPath,
		BlockedDomains: cfg.GlobalSettings.EmailDomainPolicy.BlockedDomains,
		AllowedDomains: cfg.GlobalSettings.EmailDomainPolicy.AllowedDomains,
	})
	if err != nil {
		log.Fatalf("init email domain policy: %v", err)
	}
	if cfg.GlobalSettings.EmailDomainPolicy.Enabled {
		log.Printf("email domain policy enabled with %d blocked domains", emailPolicySvc.BlockedDomainCount())
	}

	sch.Start(ctx)
	defer sch.Stop()

	achievementSvc := achievement.NewService(st, achievement.Options{
		Enabled:         cfg.AchievementsEnabled && cfg.AchievementWorkerEnabled,
		WorkerID:        "server-achievement-worker",
		Concurrency:     cfg.AchievementWorkerConcurrency,
		BatchSize:       cfg.AchievementBatchSize,
		PollInterval:    time.Duration(cfg.AchievementPollIntervalMS) * time.Millisecond,
		CleanupInterval: time.Duration(cfg.AchievementCleanupIntervalMS) * time.Millisecond,
		Redis:           rd,
	})
	achievementSvc.SetBulkEnqueuer(st)
	st.SetAchievementEventEnqueuer(achievementSvc.RedisEnqueue)
	st.SetAchievementEventNotifier(func(context.Context) {
		achievementSvc.Wake()
		go func() {
			wakeCtx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
			defer cancel()
			_ = publisher.PublishJSON(wakeCtx, "achievement.events.available", map[string]any{
				"available": true,
			})
		}()
	})
	deps := app.Dependencies{
		Config:      cfg,
		GameData:    gameData,
		Store:       st,
		AuthGate:    authGate,
		Redis:       rd,
		Auth:        au,
		EmailPolicy: emailPolicySvc,
		Moderation:  moderationSvc,
		MQ:          publisher,
		Game:        gm,
		Market:      mk,
		Risk:        riskSvc,
		RiskInsight: riskInsightSvc,
		Hub:         hub,
		Scheduler:   sch,
	}

	srv := httpapi.NewServer(deps)
	achievementSvc.SetProcessedUsersCallback(srv.NotifyAchievementUsersProcessed)
	achievementSvc.Start(ctx)
	if err := srv.Listen(":" + cfg.Port); err != nil {
		log.Printf("server stopped: %v", err)
	}
}

func kittyLevelExpMap(settings config.KittySettings) map[int]int64 {
	out := make(map[int]int64, len(settings.Leveling.Levels))
	for _, row := range settings.Leveling.Levels {
		if row.Level > 0 && row.NextExp > 0 {
			out[row.Level] = row.NextExp
		}
	}
	return out
}

func catalogItemIDSet(data catalog.Data) map[string]bool {
	out := make(map[string]bool, len(data.Items))
	for _, item := range data.Items {
		if item.ID != "" {
			out[item.ID] = true
		}
	}
	return out
}

func catalogRarityIDSet(data catalog.Data) map[string]bool {
	out := make(map[string]bool, len(data.RarityDefinitions))
	for _, row := range data.RarityDefinitions {
		if row.ID != "" {
			out[row.ID] = true
		}
	}
	return out
}
