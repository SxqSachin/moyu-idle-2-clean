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
	"mysecondwebgame/server/internal/app"
	"mysecondwebgame/server/internal/battleruntime"
	"mysecondwebgame/server/internal/catalog"
	"mysecondwebgame/server/internal/config"
	"mysecondwebgame/server/internal/diagnostics"
	"mysecondwebgame/server/internal/gamedata"
	"mysecondwebgame/server/internal/mq"
	"mysecondwebgame/server/internal/reqmetrics"
	"mysecondwebgame/server/internal/store"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}
	diagnostics.StartPprofFromEnv("BATTLE_WORKER_PPROF_ADDR", "battle worker")
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

	publisher, err := mq.NewNatsPublisher(cfg.NATSURL, cfg.MQTopicPrefix)
	if err != nil {
		log.Fatalf("connect nats: %v", err)
	}
	defer publisher.Close()

	gameData, err := catalog.Load(cfg.GameDataPath)
	if err != nil {
		log.Fatalf("load game data: %v", err)
	}
	kittyPersonalityRows := gamedata.KittyPersonalityConfigsFromCatalog(gameData.KittyPersonalities)
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
		KittyBattleLevelCapDefault:       cfg.SharedGlobalSettings.Kitty.BattleLevelCapDefault,
		KittyBattleLevelCapPerKittyLevel: cfg.SharedGlobalSettings.Kitty.BattleLevelCapPerKittyLevel,
		KittyMaxPersonalities:            cfg.SharedGlobalSettings.Kitty.MaxPersonalities,
		KittyDefaultPersonalityID:        kittyDefaultPersonalityID,
		KittyPersonalityProfiles:         gamedata.KittyPersonalityProfileMap(kittyPersonalityRows),
		KittyPersonalityOrder:            gamedata.KittyPersonalityIDOrder(kittyPersonalityRows),
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

	if err := st.BindFurnitureInvalidations(publisher); err != nil {
		log.Fatalf("subscribe furniture cache: %v", err)
	}
	deps := app.Dependencies{
		Config:   cfg,
		GameData: gameData,
		Store:    st,
		Redis:    rd,
		MQ:       publisher,
	}

	worker := battleruntime.NewWorker(deps)
	log.Printf("battle worker started nats=%s", cfg.NATSURL)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 45*time.Second)
	shutdownErr := worker.Close(shutdownCtx)
	shutdownCancel()
	if shutdownErr != nil {
		// Never close the shared DB/Redis/NATS clients underneath an in-flight
		// compile. The service manager may still enforce its outer stop timeout,
		// but an orderly process exit must wait for worker-owned tasks first.
		log.Printf("battle worker graceful shutdown deadline reached err=%v; waiting for in-flight tasks", shutdownErr)
		_ = worker.Close(context.Background())
	}
	log.Printf("battle worker stopped")
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
