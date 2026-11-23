package main

import (
	"context"
	"flag"
	"log"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"mysecondwebgame/server/internal/catalog"
	"mysecondwebgame/server/internal/config"
	"mysecondwebgame/server/internal/store"
)

func main() {
	batchSize := flag.Int("batch-size", 200, "batch size per transaction")
	flag.Parse()

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}
	ctx := context.Background()
	db, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("connect postgres: %v", err)
	}
	defer db.Close()

	gameData, err := catalog.Load(cfg.GameDataPath)
	if err != nil {
		log.Fatalf("load game data: %v", err)
	}
	st := store.New(db, store.RuntimeConfig{
		OfflineProgressMaxDuration:   time.Duration(cfg.GlobalSettings.Activity.OfflineProgressMaxDurationSeconds) * time.Second,
		ActionQueueLaneLimit:         int(cfg.SharedGlobalSettings.Action.QueueLaneLimit),
		KittyActionQueueLaneLimit:    int(cfg.SharedGlobalSettings.Action.KittyQueueLaneLimit),
		KittyBattleLevelCapDefault:   cfg.SharedGlobalSettings.Kitty.BattleLevelCapDefault,
		KittyMaxLevel:                cfg.KittySettings.Leveling.MaxLevel,
		KittyLevelExp:                kittyLevelExpMap(cfg.KittySettings),
		KittyLevelExpBase:            cfg.KittySettings.Leveling.FallbackFormula.Base,
		KittyLevelExpPerLevel:        cfg.KittySettings.Leveling.FallbackFormula.PerLevel,
		KittyLevelExpPower:           cfg.KittySettings.Leveling.FallbackFormula.Power,
		ItemRecycleGoldCoefficient:   cfg.GlobalSettings.Economy.ItemRecycleGoldCoefficient,
		SkillLevelUpBaseExp:          cfg.GlobalSettings.Skill.LevelUpBaseExp,
		SkillLevelUpPerLevelExtraExp: cfg.GlobalSettings.Skill.LevelUpPerLevelExtraExp,
	}, gameData)

	updated, err := st.RebuildAllEquipmentSnapshots(ctx, *batchSize)
	if err != nil {
		log.Fatalf("rebuild snapshots failed: %v", err)
	}
	log.Printf("rebuild snapshots completed updated=%d batch_size=%d", updated, *batchSize)
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
