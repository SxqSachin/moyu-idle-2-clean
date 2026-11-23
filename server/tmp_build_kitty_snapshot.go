package main

import (
	"encoding/json"
	"fmt"
	"log"

	"mysecondwebgame/server/internal/catalog"
	"mysecondwebgame/server/internal/config"
	"mysecondwebgame/server/internal/gamedata"
	"mysecondwebgame/server/internal/store"

	"github.com/jackc/pgx/v5/pgxpool"
	"context"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}
	ctx := context.Background()
	dbConfig, err := pgxpool.ParseConfig(cfg.DatabaseURL)
	if err != nil {
		log.Fatal(err)
	}
	db, err := pgxpool.NewWithConfig(ctx, dbConfig)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	gameData, err := catalog.Load(cfg.GameDataPath)
	if err != nil {
		log.Fatal(err)
	}
	kittyRows := gamedata.KittyPersonalityConfigsFromCatalog(gameData.KittyPersonalities)
	_ = store.New(db, store.RuntimeConfig{
		KittyBattleLevelCapDefault:          cfg.SharedGlobalSettings.Kitty.BattleLevelCapDefault,
		KittyBattleLevelCapPerKittyLevel:    cfg.SharedGlobalSettings.Kitty.BattleLevelCapPerKittyLevel,
		KittyBattleEquipSlots:            append([]string(nil), cfg.SharedGlobalSettings.Kitty.BattleEquipSlots...),
		KittyBattleAttrExpMultiplier:     cfg.SharedGlobalSettings.Kitty.BattleAttrExpMultiplier,
		KittyBattleSkillExpMultiplier:    cfg.SharedGlobalSettings.Kitty.BattleSkillExpMultiplier,
		KittyMaxPersonalities:            cfg.SharedGlobalSettings.Kitty.MaxPersonalities,
		KittyRemovePersonalityItemID:     cfg.SharedGlobalSettings.Kitty.RemovePersonalityItemID,
		KittyRemovePersonalityCount:      cfg.SharedGlobalSettings.Kitty.RemovePersonalityCount,
		KittyDefaultPersonalityID:        gamedata.ResolveDefaultKittyPersonalityID(cfg.SharedGlobalSettings.Kitty.DefaultPersonalityID, kittyRows),
		KittyPersonalityProfiles:         gamedata.KittyPersonalityProfileMap(kittyRows),
		KittyPersonalityOrder:            gamedata.KittyPersonalityIDOrder(kittyRows),
		Battle:                           store.BattleRuntimeConfig{},
		DerivedStatsFormula:              store.DerivedStatsFormulaConfig{Version: cfg.GlobalSettings.DerivedStats.Version, Operators: append([]string(nil), cfg.GlobalSettings.DerivedStats.Operators...)},
	}, gameData)

	payload := map[string]any{
		"kittyPersonalityCount": len(kittyRows),
		"defaultPersonalityID":  gamedata.ResolveDefaultKittyPersonalityID(cfg.SharedGlobalSettings.Kitty.DefaultPersonalityID, kittyRows),
		"personalityOrder":      gamedata.KittyPersonalityIDOrder(kittyRows),
	}
	b, _ := json.MarshalIndent(payload, "", "  ")
	fmt.Println(string(b))
}
