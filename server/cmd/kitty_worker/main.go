package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"mysecondwebgame/server/internal/catalog"
	"mysecondwebgame/server/internal/config"
	"mysecondwebgame/server/internal/kittyworker"
	"mysecondwebgame/server/internal/reqmetrics"
	"mysecondwebgame/server/internal/store"
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

	gameData, err := catalog.Load(cfg.GameDataPath)
	if err != nil {
		log.Fatalf("load game data: %v", err)
	}
	if err := config.ValidateKittySettingsItemIDs(cfg.KittySettings, catalogItemIDSet(gameData)); err != nil {
		log.Fatalf("validate kitty config items: %v", err)
	}
	if err := config.ValidateKittySettingsRarityIDs(cfg.KittySettings, catalogRarityIDSet(gameData)); err != nil {
		log.Fatalf("validate kitty config rarities: %v", err)
	}

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
		AchievementsEnabled:              cfg.AchievementsEnabled,
		WarehouseDefaultCols:             cfg.GlobalSettings.Warehouse.DefaultCols,
		WarehouseDefaultRows:             cfg.GlobalSettings.Warehouse.DefaultRows,
		WarehouseLogDefaultLookback:      time.Duration(cfg.GlobalSettings.Warehouse.LogDefaultLookbackDays) * 24 * time.Hour,
		OfflineProgressMaxDuration:       time.Duration(cfg.GlobalSettings.Activity.OfflineProgressMaxDurationSeconds) * time.Second,
		OfflineSettleExecBudget:          time.Duration(cfg.GlobalSettings.ActionQueue.FastForward.MaxExecMs) * time.Millisecond,
		ActionQueueFastForwardMaxSteps:   cfg.GlobalSettings.ActionQueue.FastForward.MaxSteps,
		OrdinarySettlementEnabled:        cfg.OrdinarySettlementEnabled,
		ActionQueueFastForwardMaxExec:    time.Duration(cfg.GlobalSettings.ActionQueue.FastForward.MaxExecMs) * time.Millisecond,
		ActionQueueLaneLimit:             int(cfg.SharedGlobalSettings.Action.QueueLaneLimit),
		KittyActionQueueLaneLimit:        int(cfg.SharedGlobalSettings.Action.KittyQueueLaneLimit),
		KittyMaxPersonalities:            cfg.SharedGlobalSettings.Kitty.MaxPersonalities,
		KittyBattleLevelCapDefault:       cfg.SharedGlobalSettings.Kitty.BattleLevelCapDefault,
		KittyBattleLevelCapPerKittyLevel: cfg.SharedGlobalSettings.Kitty.BattleLevelCapPerKittyLevel,
		KittyStatusExpMultiplier:         cfg.SharedGlobalSettings.Kitty.StatusExpMultiplier,
		KittyMaxLevel:                    cfg.KittySettings.Leveling.MaxLevel,
		KittyLevelExp:                    kittyLevelExpMap(cfg.KittySettings),
		KittyLevelExpBase:                cfg.KittySettings.Leveling.FallbackFormula.Base,
		KittyLevelExpPerLevel:            cfg.KittySettings.Leveling.FallbackFormula.PerLevel,
		KittyLevelExpPower:               cfg.KittySettings.Leveling.FallbackFormula.Power,
		AuctionFeeRateBps:                cfg.SharedGlobalSettings.Auction.FeeRateBps,
		AuctionMinBidIncrement:           cfg.SharedGlobalSettings.Auction.MinBidIncrement,
		AuctionAntiSnipeTriggerSeconds:   cfg.SharedGlobalSettings.Auction.AntiSnipeTriggerSeconds,
		AuctionAntiSnipeExtendSeconds:    cfg.SharedGlobalSettings.Auction.AntiSnipeExtendSeconds,
		AuctionMinRepLevel:               cfg.SharedGlobalSettings.Auction.MinRepLevel,
		ActionEnhanceInheritanceRatio:    cfg.SharedGlobalSettings.Action.EnhanceInheritanceRatio,
		ItemRecycleGoldCoefficient:       cfg.GlobalSettings.Economy.ItemRecycleGoldCoefficient,
		SkillLevelUpBaseExp:              cfg.GlobalSettings.Skill.LevelUpBaseExp,
		SkillLevelUpPerLevelExtraExp:     cfg.GlobalSettings.Skill.LevelUpPerLevelExtraExp,
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
	}, gameData)

	worker := kittyworker.New(st, cfg.GlobalSettings.KittyJournal, cfg.KittySettings, gameData.KittyJournalPersonalityGrants)
	workerCtx, cancel := context.WithCancel(context.Background())
	go worker.Run(workerCtx)
	log.Printf("kitty worker started")

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh
	cancel()
	log.Printf("kitty worker stopped")
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
