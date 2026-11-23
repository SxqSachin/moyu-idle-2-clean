package httpapi

// battle_fog.go — Fog Mode dungeon logic.
//
// Fog Mode is an advanced room setting that:
//   - Consumes a per-dungeon "fog map" item on each dungeon entry (initial + auto-reenter).
//   - Draws from a server-wide merged enemy pool instead of the single dungeon pool.
//   - Overrides the special-wave interval to fogMode.specialWaveEvery.
//   - Has a configurable chance each wave to replace enemies with a "player phantom" team
//     drawn from another active fog room in the same dungeon (single-direction, B unaware).
//   - Applies dedicated loot (fogPlayerDrops) when a player phantom wave is defeated.

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"time"

	"github.com/jackc/pgx/v5"
	"mysecondwebgame/server/internal/config"
	"mysecondwebgame/server/internal/store"
)

// --------------------------------------------------------------------------
// Pool pre-construction (called once at server startup)
// --------------------------------------------------------------------------

// initFogModePools builds the merged enemy pools and computes fogBaseEnemyLevel
// from the last dungeon in world.json. All fog-mode battles draw from these pools.
func (s *Server) initFogModePools() {
	enabledIDs := s.enabledEnemyIDSet()
	world := s.deps.GameData.World

	normalSet := map[string]struct{}{}
	specialSet := map[string]struct{}{}

	for _, raw := range world {
		for _, id := range filterEnabledIDs(toStringSlice(raw["enemyIds"]), enabledIDs) {
			normalSet[id] = struct{}{}
		}
		for _, id := range filterEnabledIDs(toStringSlice(raw["specialEnemyIds"]), enabledIDs) {
			specialSet[id] = struct{}{}
		}
	}

	normalPool := make([]string, 0, len(normalSet))
	for id := range normalSet {
		normalPool = append(normalPool, id)
	}
	specialPool := make([]string, 0, len(specialSet))
	for id := range specialSet {
		specialPool = append(specialPool, id)
	}

	// Fog base enemy level = average level of normal+special enemies in the last dungeon.
	baseLevel := 1
	if len(world) > 0 {
		lastRaw := world[len(world)-1]
		lastNormal := filterEnabledIDs(toStringSlice(lastRaw["enemyIds"]), enabledIDs)
		lastSpecial := filterEnabledIDs(toStringSlice(lastRaw["specialEnemyIds"]), enabledIDs)
		allLast := append(lastNormal, lastSpecial...)
		catalog := s.battleEnemyCatalog()
		sum, count := 0, 0
		for _, id := range allLast {
			if cfg, ok := catalog[id]; ok {
				sum += cfg.Level
				count++
			}
		}
		if count > 0 {
			baseLevel = sum / count
			if baseLevel < 1 {
				baseLevel = 1
			}
		}
	}

	s.battleDataMu.Lock()
	s.fogNormalEnemyPool = normalPool
	s.fogSpecialEnemyPool = specialPool
	s.fogBaseEnemyLevel = baseLevel
	s.battleDataMu.Unlock()
}

// --------------------------------------------------------------------------
// Fog mode entry: item consumption / disable
// --------------------------------------------------------------------------

// handleFogModeEntry checks and consumes 1 fog map item for the given room.
// Must be called within a DB transaction. Returns fogDisabled=true if items
// were insufficient and fog mode was immediately turned off.
func (s *Server) handleFogModeEntry(ctx context.Context, tx pgx.Tx, room store.BattleRoom) (fogDisabled bool, err error) {
	if !room.IsFogMode {
		return false, nil
	}
	worldCfg, ok := s.battleWorldConfigByID(room.DungeonID)
	if !ok || worldCfg.FogMapItemID == "" {
		// No configured item — allow fog entry for free.
		return false, nil
	}
	qty, err := s.deps.Store.InventoryQuantityTx(ctx, tx, room.OwnerID, worldCfg.FogMapItemID)
	if err != nil {
		return false, fmt.Errorf("fog: check item qty: %w", err)
	}
	if qty < 1 {
		if err := s.deps.Store.DisableFogModeTx(ctx, tx, room.ID); err != nil {
			return false, fmt.Errorf("fog: disable fog mode: %w", err)
		}
		return true, nil
	}
	if err := s.deps.Store.RemoveInventoryItemTx(ctx, tx, room.OwnerID, worldCfg.FogMapItemID, 1); err != nil {
		return false, fmt.Errorf("fog: consume item: %w", err)
	}
	return false, nil
}

// pushFogModeDisabledEvent sends a real-time WS notification to all room members
// that fog mode was automatically disabled due to insufficient items.
func (s *Server) pushFogModeDisabledEvent(detail store.BattleRoomDetail, ctx ...context.Context) {
	userIDs := make([]int64, 0, len(detail.Members))
	for _, m := range detail.Members {
		if m.UserID > 0 {
			userIDs = append(userIDs, m.UserID)
		}
	}
	s.pushBattleRoomEventToUsers(userIDs, "fog_mode_disabled", map[string]any{
		"roomId":  detail.Room.ID,
		"ownerId": detail.Room.OwnerID,
		"reason":  "insufficient_items",
	}, ctx...)
}

// --------------------------------------------------------------------------
// Member snapshot: write & build phantoms
//
// Design rationale: the snapshot stores lightweight BattleRoomUnit references
// (room unit roster data), NOT compiled battleUnitState. Phantom units are
// reconstructed at encounter time using the exact same compilation pipeline as
// normal battle startup. This guarantees:
//   - Units are always in valid, fully-initialised state (Alive=true, HP=MaxHP)
//   - Internal fields (BaseStats, StatMode, etc.) are properly set by the pipeline
//   - Any future changes to unit compilation logic automatically apply to phantoms
// --------------------------------------------------------------------------

// writeFogMemberSnapshots stores each room member's BattleRoomUnit list in
// fog_room_member_snapshots so phantom encounters can reconstruct the team later.
func (s *Server) writeFogMemberSnapshots(ctx context.Context, detail store.BattleRoomDetail) error {
	unitsByOwner := map[int64][]store.BattleRoomUnit{}
	for _, u := range detail.Units {
		unitsByOwner[u.OwnerUserID] = append(unitsByOwner[u.OwnerUserID], u)
	}
	// Fallback: units not yet present in detail — synthesise player-only entries from member list.
	if len(unitsByOwner) == 0 {
		for _, m := range detail.Members {
			unitsByOwner[m.UserID] = append(unitsByOwner[m.UserID], store.BattleRoomUnit{
				RoomID:      m.RoomID,
				OwnerUserID: m.UserID,
				Username:    m.Username,
				UnitType:    "player",
				Position:    m.Position,
			})
		}
	}

	for ownerID, roomUnits := range unitsByOwner {
		raw, err := json.Marshal(roomUnits)
		if err != nil {
			return fmt.Errorf("fog: marshal unit refs user=%d: %w", ownerID, err)
		}
		if err := s.deps.Store.UpsertFogMemberSnapshot(ctx, detail.Room.ID, detail.Room.DungeonID, ownerID, raw); err != nil {
			return fmt.Errorf("fog: upsert snapshot user=%d: %w", ownerID, err)
		}
	}
	return nil
}

// --------------------------------------------------------------------------
// Player phantom encounter
// --------------------------------------------------------------------------

// resolvePlayerPhantomEncounter decides whether the current wave becomes a
// player-phantom encounter. Returns (units, true, nil) when triggered.
func (s *Server) resolvePlayerPhantomEncounter(ctx context.Context, state *battleRuntimeState) ([]battleUnitState, bool, error) {
	if !state.IsFogMode {
		return nil, false, nil
	}
	fogCfg := s.deps.Config.SharedGlobalSettings.FogMode
	if rand.Float64() >= fogCfg.PlayerEncounterChance {
		return nil, false, nil
	}

	candidates, err := s.deps.Store.GetFogRoomCandidates(ctx, state.DungeonID, state.RoomID)
	if err != nil {
		return nil, false, fmt.Errorf("fog: get candidates: %w", err)
	}
	if len(candidates) == 0 {
		return nil, false, nil
	}

	chosenRoomID := candidates[rand.Intn(len(candidates))]
	snapshots, err := s.deps.Store.GetFogRoomSnapshots(ctx, chosenRoomID)
	if err != nil || len(snapshots) == 0 {
		return nil, false, err
	}

	var units []battleUnitState
	for i, snap := range snapshots {
		memberUnits, err := s.buildPhantomUnits(ctx, snap, i)
		if err != nil {
			return nil, false, fmt.Errorf("fog: build phantom member=%d: %w", i, err)
		}
		units = append(units, memberUnits...)
	}
	if len(units) == 0 {
		return nil, false, nil
	}
	return units, true, nil
}

// buildPhantomUnits compiles a fog member's room units into battle enemy units.
// It deserialises the stored BattleRoomUnit references and runs them through the
// same compilation pipeline used at normal battle startup — ensuring every unit
// starts with correct Alive=true and full HP regardless of the caller's context.
func (s *Server) buildPhantomUnits(ctx context.Context, snap store.FogMemberSnapshot, memberIndex int) ([]battleUnitState, error) {
	var roomUnits []store.BattleRoomUnit
	if err := json.Unmarshal(snap.Snapshot, &roomUnits); err != nil {
		return nil, fmt.Errorf("unmarshal room units: %w", err)
	}

	now := time.Now()
	units := make([]battleUnitState, 0, len(roomUnits))

	for i, ru := range roomUnits {
		unit, err := s.buildBattleRoomUnitSnapshot(ctx, ru, now)
		if err != nil {
			// Skip units that cannot be compiled (e.g. kitty retired since snapshot).
			continue
		}
		// Apply equipment runtime hooks so stat interactions are identical to real battles.
		s.battleApplyUnitEquipmentRuntime(&unit)

		// Mark as fog-player phantom enemy.
		unit.Side = "enemy"
		unit.OriginSide = "enemy"
		unit.BaseSide = "enemy"
		unit.EnemyType = "fog_player"
		unit.RewardEligible = true
		unit.IsPlayerUnit = false
		unit.UnitID = fmt.Sprintf("fog_player:%d:%d", memberIndex, i)

		// Guarantee live state — the compilation pipeline sets Alive=true and
		// HP=MaxHP, but we assert it explicitly as a defensive invariant.
		unit.HP = math.Max(1, unit.MaxHP)
		unit.MP = math.Max(0, unit.MaxMP)
		unit.Alive = true

		units = append(units, unit)
	}
	return units, nil
}

// --------------------------------------------------------------------------
// Dynamic enemy stat generation (port of combat-model-vue3/balance.ts)
// --------------------------------------------------------------------------

// generateFogEnemyStats dynamically generates a stat map for a fog-mode enemy
// using the combat balance model. This replicates generateEnemyStats() from
// combat-model-vue3/src/model/balance.ts exactly.
//
// tier  = "normal" | "elite" | "boss"
// class = enemy's class/role key (e.g. "scout", "warrior")
// race  = enemy's species/race key (e.g. "beast", "humanoid")
func (s *Server) generateFogEnemyStats(level int, tier, class, race string) map[string]float64 {
	scoring := s.deps.Config.SharedGlobalSettings.CombatScoring
	model := s.deps.Config.SharedGlobalSettings.CombatBalanceModel

	// Locate tier/role/species templates.
	var tierTpl *config.EnemyTierTemplate
	for i := range scoring.EnemyAttributeTemplates.Tiers {
		if scoring.EnemyAttributeTemplates.Tiers[i].Key == tier {
			tierTpl = &scoring.EnemyAttributeTemplates.Tiers[i]
			break
		}
	}
	// Fallback: use first tier.
	if tierTpl == nil && len(scoring.EnemyAttributeTemplates.Tiers) > 0 {
		tierTpl = &scoring.EnemyAttributeTemplates.Tiers[0]
	}
	if tierTpl == nil {
		return nil
	}

	var roleTpl *config.EnemyRoleTemplate
	for i := range scoring.EnemyAttributeTemplates.Roles {
		if scoring.EnemyAttributeTemplates.Roles[i].Key == class {
			roleTpl = &scoring.EnemyAttributeTemplates.Roles[i]
			break
		}
	}
	// Fallback: use first role.
	if roleTpl == nil && len(scoring.EnemyAttributeTemplates.Roles) > 0 {
		roleTpl = &scoring.EnemyAttributeTemplates.Roles[0]
	}
	if roleTpl == nil {
		return nil
	}

	var speciesTpl *config.EnemySpeciesTemplate
	for i := range scoring.EnemyAttributeTemplates.Species {
		if scoring.EnemyAttributeTemplates.Species[i].Key == race {
			speciesTpl = &scoring.EnemyAttributeTemplates.Species[i]
			break
		}
	}

	// Build merged weights: role + species (additive).
	merged := make(map[string]float64, len(roleTpl.Weights)+8)
	for k, v := range roleTpl.Weights {
		if v > 0 {
			merged[k] = v
		}
	}
	if speciesTpl != nil {
		for k, v := range speciesTpl.Weights {
			merged[k] = math.Max(0, merged[k]+v)
		}
	}

	// Compute budget.
	safeLevel := level
	if safeLevel < 1 {
		safeLevel = 1
	}
	budgetMult := tierTpl.BudgetMultiplier
	if budgetMult <= 0 {
		budgetMult = 1
	}
	baseBudget := math.Max(1, math.Floor(math.Pow(float64(safeLevel), 1.2)))
	totalBudget := baseBudget * budgetMult

	attrUnitValues := scoring.AttributeUnitValues

	// Allocate primary attributes from budget; skip hp/physics_damage/magical_damage — target-based below.
	result := make(map[string]float64)
	for stat, weight := range merged {
		if stat == "hp" || stat == "physics_damage" || stat == "magical_damage" {
			continue
		}
		unitValue := attrUnitValues[stat]
		if unitValue <= 0 {
			unitValue = 1
		}
		result[stat] = math.Round(totalBudget * weight / unitValue)
	}

	// Apply role baseAttrs and multipliers.
	for stat, base := range roleTpl.BaseAttrs {
		result[stat] = result[stat] + math.Round(base)
	}
	for stat, mult := range roleTpl.Multipliers {
		if _, ok := result[stat]; ok {
			result[stat] = math.Round(result[stat] * mult)
		}
	}

	// Apply species baseAttrs and multipliers.
	if speciesTpl != nil {
		for stat, base := range speciesTpl.BaseAttrs {
			result[stat] = result[stat] + math.Round(base)
		}
		for stat, mult := range speciesTpl.Multipliers {
			if _, ok := result[stat]; ok {
				result[stat] = math.Round(result[stat] * mult)
			}
		}
	}

	// Derived values from primary attributes.
	str := result["strength"]
	dex := result["dexterity"]
	intel := result["intelligence"]
	per := result["perception"]
	sta := result["stamina"]
	derivedHp := sta*9 + str*1.2
	derivedPhysDmg := str*0.55 + dex*0.25
	derivedMagDmg := intel*0.7 + per*0.3

	// Target HP and DPS from player baseline at this level.
	baseline := fogBaselineAtLevel(model, safeLevel)
	midRounds := (tierTpl.TargetRounds.Min + tierTpl.TargetRounds.Max) / 2
	hpMult := tierTpl.HpMultiplier
	if hpMult <= 0 {
		hpMult = 1
	}
	targetHp := baseline.PlayerDPR * midRounds * budgetMult * hpMult
	targetDps := baseline.PlayerHP * tierTpl.PressurePct * budgetMult

	physW := merged["physics_damage"]
	magW := merged["magical_damage"]
	totalDpsW := physW + magW
	if totalDpsW <= 0 {
		totalDpsW = 1
		physW = 0.5
		magW = 0.5
	}
	targetPhysDmg := targetDps * physW / totalDpsW
	targetMagDmg := targetDps * magW / totalDpsW

	result["hp"] = math.Max(1, math.Round(targetHp-derivedHp))
	result["physics_damage"] = math.Max(1, math.Round(targetPhysDmg-derivedPhysDmg))
	result["magical_damage"] = math.Max(1, math.Round(targetMagDmg-derivedMagDmg))

	// Apply derived stat minimums.
	for stat, minVal := range scoring.EnemyAttributeTemplates.DerivedStatMinimums {
		if v, ok := result[stat]; ok && v < minVal {
			result[stat] = math.Round(minVal)
		}
	}

	return result
}

// fogBaselineAtLevel interpolates the player baseline stats at the given level,
// replicating baselineAtLevel() from combat-model-vue3/src/model/balance.ts.
func fogBaselineAtLevel(model config.CombatBalanceModelSettings, level int) config.CombatBalanceLevelBand {
	bands := model.LevelBands
	if len(bands) == 0 {
		return config.CombatBalanceLevelBand{Level: level, PlayerDPR: 10, PlayerHP: 100}
	}
	safeLevel := level
	if safeLevel < 1 {
		safeLevel = 1
	}
	if safeLevel <= bands[0].Level {
		b := bands[0]
		b.Level = safeLevel
		return b
	}
	last := bands[len(bands)-1]
	if safeLevel >= last.Level {
		if len(bands) < 2 {
			return last
		}
		prev := bands[len(bands)-2]
		span := math.Max(1, float64(last.Level-prev.Level))
		dprGrowth := math.Pow(math.Max(1.01, last.PlayerDPR/math.Max(1, prev.PlayerDPR)), 1/span)
		hpGrowth := math.Pow(math.Max(1.01, last.PlayerHP/math.Max(1, prev.PlayerHP)), 1/span)
		delta := float64(safeLevel - last.Level)
		return config.CombatBalanceLevelBand{
			Level:     safeLevel,
			PlayerDPR: math.Round(last.PlayerDPR*math.Pow(dprGrowth, delta)*10000) / 10000,
			PlayerHP:  math.Round(last.PlayerHP*math.Pow(hpGrowth, delta)*10000) / 10000,
		}
	}
	// Linear interpolation between two bands.
	var upperIdx int
	for i := range bands {
		if bands[i].Level >= safeLevel {
			upperIdx = i
			break
		}
	}
	lowerIdx := upperIdx - 1
	if lowerIdx < 0 {
		lowerIdx = 0
	}
	lower := bands[lowerIdx]
	upper := bands[upperIdx]
	t := float64(safeLevel-lower.Level) / math.Max(1, float64(upper.Level-lower.Level))
	lerp := func(a, b float64) float64 {
		return math.Round((a+(b-a)*t)*10000) / 10000
	}
	return config.CombatBalanceLevelBand{
		Level:     safeLevel,
		PlayerDPR: lerp(lower.PlayerDPR, upper.PlayerDPR),
		PlayerHP:  lerp(lower.PlayerHP, upper.PlayerHP),
	}
}

// --------------------------------------------------------------------------
// Fog wave enemy spawning
// --------------------------------------------------------------------------

// spawnFogWaveEnemies implements the full fog-mode wave spawn:
//  1. Check for player phantom encounter (takes priority over the random fog pool).
//  2. If not phantom: determine special vs normal wave using fogMode.specialWaveEvery.
//  3. Pick enemies from the merged fog pool, replacing their stats with dynamically
//     generated values while keeping skills/equipment/buffs from the config.
//  4. Apply current room's difficultySettings scaling (same as normal mode).
//
// customLineupConf and waveDrops are intentionally skipped in fog mode.
// Trigger-item spawns continue to work but are NOT level-boosted.
func (s *Server) spawnFogWaveEnemies(
	ctx context.Context,
	state *battleRuntimeState,
	enemyCatalog map[string]battleEnemyConfig,
	worldCfg battleWorldConfig,
	defaultBasicSkillID string,
) {
	// ── 1. Player phantom encounter (replaces the random fog spawn) ──
	if phantoms, encountered, err := s.resolvePlayerPhantomEncounter(ctx, state); err == nil && encountered {
		state.Enemies = phantoms
		s.appendFogPhantomAuthoredEnemies(state, enemyCatalog, worldCfg, defaultBasicSkillID)
		// Difficulty scaling skips player phantoms but still applies to authored
		// encounters appended to the same wave.
		s.applyFogRoomDifficultyScaling(state, worldCfg)
		return
	}

	fogCfg := s.deps.Config.SharedGlobalSettings.FogMode

	// ── 2. Special wave determination (fog override: every N waves) ──
	specialWaveEvery := fogCfg.SpecialWaveEvery
	if specialWaveEvery <= 0 {
		specialWaveEvery = 10
	}
	isSpecialWave := state.Wave > 0 && state.Wave%specialWaveEvery == 0

	// ── 3. Pick the enemy pool ──
	s.battleDataMu.RLock()
	normalPool := s.fogNormalEnemyPool
	specialPool := s.fogSpecialEnemyPool
	fogLevel := s.fogBaseEnemyLevel
	s.battleDataMu.RUnlock()

	// Count scale (same formula as normal mode).
	count := s.rollBattleCountRange(state, worldCfg.NormalEnemyCountRange)
	extraPerPlayer := s.rollBattleCountRange(state, worldCfg.ExtraEnemyPerPlayer)
	nonSummonCount := battleEnemyScalingPlayerCount(state.Players, worldCfg.MaxTeamSize)
	count += maxInt(0, nonSummonCount-1) * extraPerPlayer
	if count <= 0 {
		count = 1
	}

	// World schedules and trigger-item spawns keep their authored base stats.
	scheduleSpawnIDs := s.resolveWaveScheduleEnemyIDs(state, worldCfg)

	state.Enemies = make([]battleUnitState, 0, count+len(scheduleSpawnIDs))

	// Spawn normal fog enemies.
	pool := normalPool
	if isSpecialWave {
		pool = specialPool
	}
	prefix := "enemy"
	if isSpecialWave {
		prefix = "enemy:s"
	}
	for i := 0; i < count && len(pool) > 0; i++ {
		idx := state.randInt(len(pool))
		enemyID := pool[idx]
		cfg, ok := enemyCatalog[enemyID]
		if !ok {
			continue
		}
		fogCfg := s.generateFogEnemyStats(fogLevel, cfg.Type, cfg.Class, cfg.Race)
		if fogCfg != nil {
			cfg.Stats = fogCfg
		}
		unitID := fmt.Sprintf("%s:%d:%d:%s", prefix, state.Wave, i+1, cfg.ID)
		enemyUnit := s.buildBattleEnemyUnit(cfg, unitID, false, defaultBasicSkillID)
		s.applyEnemyHpRegenRatio(&enemyUnit)
		state.Enemies = append(state.Enemies, enemyUnit)
	}

	for index, enemyID := range scheduleSpawnIDs {
		cfg, exists := enemyCatalog[enemyID]
		if !exists {
			continue
		}
		unitID := fmt.Sprintf("enemy:%d:schedule%d:%s", state.Wave, index+1, cfg.ID)
		enemyUnit := s.buildBattleEnemyUnit(cfg, unitID, false, defaultBasicSkillID)
		s.applyEnemyHpRegenRatio(&enemyUnit)
		state.Enemies = append(state.Enemies, enemyUnit)
	}

	// Trigger-item enemies keep their authored level and base stats.
	state.Enemies = append(state.Enemies, s.resolveAndBuildFogTriggerItemEnemies(state, enemyCatalog, worldCfg.ID, defaultBasicSkillID)...)

	// ── 4. Apply current room's difficulty scaling (fog enemies only, not phantoms) ──
	s.applyFogRoomDifficultyScaling(state, worldCfg)
}

func (s *Server) buildFogAuthoredEnemies(
	state *battleRuntimeState,
	enemyCatalog map[string]battleEnemyConfig,
	defaultBasicSkillID string,
	enemyIDs []string,
	prefix string,
) []battleUnitState {
	if state == nil || len(enemyIDs) == 0 {
		return nil
	}
	units := make([]battleUnitState, 0, len(enemyIDs))
	for index, enemyID := range enemyIDs {
		cfg, exists := enemyCatalog[enemyID]
		if !exists {
			continue
		}
		unitID := fmt.Sprintf("enemy:%d:%s%d:%s", state.Wave, prefix, index+1, cfg.ID)
		enemyUnit := s.buildBattleEnemyUnit(cfg, unitID, false, defaultBasicSkillID)
		s.applyEnemyHpRegenRatio(&enemyUnit)
		units = append(units, enemyUnit)
	}
	return units
}

func (s *Server) appendFogPhantomAuthoredEnemies(
	state *battleRuntimeState,
	enemyCatalog map[string]battleEnemyConfig,
	worldCfg battleWorldConfig,
	defaultBasicSkillID string,
) {
	scheduleSpawnIDs := s.resolveWaveScheduleEnemyIDs(state, worldCfg)
	state.Enemies = append(state.Enemies, s.buildFogAuthoredEnemies(state, enemyCatalog, defaultBasicSkillID, scheduleSpawnIDs, "schedule")...)
	state.Enemies = append(state.Enemies, s.resolveAndBuildFogTriggerItemEnemies(state, enemyCatalog, worldCfg.ID, defaultBasicSkillID)...)
}

func (s *Server) resolveAndBuildFogTriggerItemEnemies(
	state *battleRuntimeState,
	enemyCatalog map[string]battleEnemyConfig,
	dungeonID string,
	defaultBasicSkillID string,
) []battleUnitState {
	triggerSpawnIDs := s.resolveTriggerItemSpawns(state, dungeonID)
	return s.buildFogAuthoredEnemies(state, enemyCatalog, defaultBasicSkillID, triggerSpawnIDs, "t")
}

func (s *Server) applyFogRoomDifficultyScaling(state *battleRuntimeState, worldCfg battleWorldConfig) {
	if state == nil {
		return
	}
	if mult := state.DifficultyMult; mult != 1.0 && mult > 0 {
		var statBonuses []battleWorldStatBonus
		if worldCfg.DifficultySettings != nil {
			statBonuses = worldCfg.DifficultySettings.StatBonuses
		}
		statBonusMap := make(map[string]float64, len(statBonuses))
		lvl := state.DifficultyLevel
		for _, sb := range statBonuses {
			if lvl >= 1 {
				statBonusMap[sb.AttrKey] = sb.Base * (1 + sb.PerLevelMult*float64(lvl-1))
			}
		}
		for i := range state.Enemies {
			e := &state.Enemies[i]
			if e.EnemyType == "fog_player" {
				continue // phantoms skip difficulty scaling
			}
			for key, val := range e.Stats {
				if isCombatStat(key) {
					e.Stats[key] = (val * mult) + statBonusMap[key]
				}
			}
			for key, val := range e.BaseStats {
				if isCombatStat(key) {
					e.BaseStats[key] = (val * mult) + statBonusMap[key]
				}
			}
			e.MaxHP = maxFloat(1, e.Stats["hp"])
			e.MaxMP = maxFloat(0, e.Stats["mp"])
			e.HP = e.MaxHP
			e.MP = e.MaxMP
		}
	}
}

// --------------------------------------------------------------------------
// Helpers
// --------------------------------------------------------------------------

// isWaveFogPlayerPhantom checks whether any unit in the enemy list is a player phantom.
func isWaveFogPlayerPhantom(enemies []battleUnitState) bool {
	for _, u := range enemies {
		if u.EnemyType == "fog_player" {
			return true
		}
	}
	return false
}

// fogMemberUserIDs extracts unique UserIDs from a player unit list.
func fogMemberUserIDs(players []battleUnitState) []int64 {
	seen := map[int64]struct{}{}
	var out []int64
	for _, u := range players {
		if u.UserID <= 0 {
			continue
		}
		if _, ok := seen[u.UserID]; ok {
			continue
		}
		seen[u.UserID] = struct{}{}
		out = append(out, u.UserID)
	}
	return out
}
