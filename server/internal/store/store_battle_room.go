package store

import (
	"context"
	"encoding/json"
	"errors"
	"hash/fnv"
	"log"
	"math"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

const (
	BattleRoomStatusIdle       = "idle"
	BattleRoomStatusPreparing  = "preparing"
	BattleRoomStatusEntering   = "entering"
	BattleRoomStatusPlaying    = "playing"
	BattleRoomStatusRecovering = "recovering"
)

const (
	BattleRunStatusCompiled    = "compiled"
	BattleRunStatusRunning     = "playing"
	BattleRunStatusSettling    = "settling"
	BattleRunStatusEnded       = "ended"
	BattleRunStatusFailed      = "failed"
	BattleRunStatusInterrupted = "interrupted"
)

const battleFinalizeStoreSlowLogThreshold = 250 * time.Millisecond
const battleRunCompiledSaveSlowLogThreshold = 100 * time.Millisecond
const battleRunCompiledSaveLargePayloadLogThreshold = 1024 * 1024
const battleRunHistoryCompactMaxRooms = 256
const battleRunHistoryCompactMaxDeleteBatch = 1000

type battleFinalizeStoreMetrics struct {
	roomLockElapsed       time.Duration
	runUpdateElapsed      time.Duration
	rewardUserLockElapsed time.Duration
	rewardItemElapsed     time.Duration
	rewardAttrElapsed     time.Duration
	memberUpdateElapsed   time.Duration
	roomUpdateElapsed     time.Duration
	txElapsed             time.Duration
	detailLoadElapsed     time.Duration
	rewardUsers           int
	rewardItemKinds       int
	rewardItemQuantity    int64
	rewardAttrKeys        int
}

func (m battleFinalizeStoreMetrics) totalElapsed() time.Duration {
	return m.txElapsed + m.detailLoadElapsed
}

func (m battleFinalizeStoreMetrics) shouldLog() bool {
	return m.totalElapsed() >= battleFinalizeStoreSlowLogThreshold ||
		m.roomLockElapsed >= battleFinalizeStoreSlowLogThreshold ||
		m.runUpdateElapsed >= battleFinalizeStoreSlowLogThreshold ||
		m.rewardUserLockElapsed >= battleFinalizeStoreSlowLogThreshold ||
		m.rewardItemElapsed >= battleFinalizeStoreSlowLogThreshold ||
		m.rewardAttrElapsed >= battleFinalizeStoreSlowLogThreshold ||
		m.memberUpdateElapsed >= battleFinalizeStoreSlowLogThreshold ||
		m.roomUpdateElapsed >= battleFinalizeStoreSlowLogThreshold ||
		m.detailLoadElapsed >= battleFinalizeStoreSlowLogThreshold
}

func logBattleFinalizeStoreStages(kind string, roomID int64, runID int64, metrics battleFinalizeStoreMetrics) {
	if !metrics.shouldLog() {
		return
	}
	log.Printf(
		"battle finalize db stages kind=%s room_id=%d run_id=%d tx_ms=%d detail_ms=%d total_ms=%d lock_room_ms=%d update_run_ms=%d reward_user_lock_ms=%d reward_item_ms=%d reward_attr_ms=%d update_members_ms=%d update_room_ms=%d reward_users=%d reward_item_kinds=%d reward_item_qty=%d reward_attr_keys=%d",
		kind,
		roomID,
		runID,
		metrics.txElapsed.Milliseconds(),
		metrics.detailLoadElapsed.Milliseconds(),
		metrics.totalElapsed().Milliseconds(),
		metrics.roomLockElapsed.Milliseconds(),
		metrics.runUpdateElapsed.Milliseconds(),
		metrics.rewardUserLockElapsed.Milliseconds(),
		metrics.rewardItemElapsed.Milliseconds(),
		metrics.rewardAttrElapsed.Milliseconds(),
		metrics.memberUpdateElapsed.Milliseconds(),
		metrics.roomUpdateElapsed.Milliseconds(),
		metrics.rewardUsers,
		metrics.rewardItemKinds,
		metrics.rewardItemQuantity,
		metrics.rewardAttrKeys,
	)
}

func logBattleRunCompiledSaveStages(
	runID int64,
	wave int,
	status string,
	compileInputMarshalElapsed time.Duration,
	stateMarshalElapsed time.Duration,
	timelineMarshalElapsed time.Duration,
	finalResultMarshalElapsed time.Duration,
	compileInputBytes int,
	stateBytes int,
	timelineBytes int,
	finalResultBytes int,
	dbExecElapsed time.Duration,
	totalElapsed time.Duration,
) {
	if !shouldLogBattleRunCompiledSaveStages(
		finalResultMarshalElapsed,
		dbExecElapsed,
		totalElapsed,
		stateBytes,
		timelineBytes,
		finalResultBytes,
	) {
		return
	}
	log.Printf(
		"battle run compiled save slow run_id=%d wave=%d status=%s marshal_compile_input_ms=%d marshal_state_ms=%d marshal_timeline_ms=%d marshal_final_result_ms=%d marshal_total_ms=%d db_exec_ms=%d total_ms=%d compile_input_bytes=%d state_bytes=%d timeline_bytes=%d final_result_bytes=%d",
		runID,
		wave,
		status,
		compileInputMarshalElapsed.Milliseconds(),
		stateMarshalElapsed.Milliseconds(),
		timelineMarshalElapsed.Milliseconds(),
		finalResultMarshalElapsed.Milliseconds(),
		(compileInputMarshalElapsed + stateMarshalElapsed + timelineMarshalElapsed + finalResultMarshalElapsed).Milliseconds(),
		dbExecElapsed.Milliseconds(),
		totalElapsed.Milliseconds(),
		compileInputBytes,
		stateBytes,
		timelineBytes,
		finalResultBytes,
	)
}

func shouldLogBattleRunCompiledSaveStages(
	finalResultMarshalElapsed time.Duration,
	dbExecElapsed time.Duration,
	totalElapsed time.Duration,
	stateBytes int,
	timelineBytes int,
	finalResultBytes int,
) bool {
	return totalElapsed >= battleRunCompiledSaveSlowLogThreshold ||
		dbExecElapsed >= battleRunCompiledSaveSlowLogThreshold ||
		finalResultMarshalElapsed >= battleRunCompiledSaveSlowLogThreshold ||
		stateBytes >= battleRunCompiledSaveLargePayloadLogThreshold ||
		timelineBytes >= battleRunCompiledSaveLargePayloadLogThreshold ||
		finalResultBytes >= battleRunCompiledSaveLargePayloadLogThreshold
}

func sortedBattleRewardUserIDs(rewards map[int64]BattleWaveReward) []int64 {
	userIDs := make([]int64, 0, len(rewards))
	for userID := range rewards {
		if userID > 0 {
			userIDs = append(userIDs, userID)
		}
	}
	sort.Slice(userIDs, func(i, j int) bool { return userIDs[i] < userIDs[j] })
	return userIDs
}

func sortedBattleRewardItemIDs(items map[string]int64) []string {
	itemIDs := make([]string, 0, len(items))
	for itemID, qty := range items {
		if strings.TrimSpace(itemID) != "" && qty > 0 {
			itemIDs = append(itemIDs, itemID)
		}
	}
	sort.Strings(itemIDs)
	return itemIDs
}

func countBattleRewardAttrKeys(attrs map[string]float64) int {
	count := 0
	for key, gain := range attrs {
		if strings.TrimSpace(key) != "" && gain != 0 {
			count++
		}
	}
	return count
}

func battleRewardNeedsUserLock(itemKinds, attrKeys int, applyAttrExp bool) bool {
	return itemKinds > 0 || (applyAttrExp && attrKeys > 0)
}

func (s *Store) applyBattleRoomRewardsTx(
	ctx context.Context,
	tx pgx.Tx,
	roomID int64,
	runID int64,
	rewards map[int64]BattleWaveReward,
	rewardType string,
	applyAttrExp bool,
	metrics *battleFinalizeStoreMetrics,
) error {
	if len(rewards) == 0 {
		return nil
	}
	for _, userID := range sortedBattleRewardUserIDs(rewards) {
		reward := rewards[userID]
		itemIDs := sortedBattleRewardItemIDs(reward.ItemRewards)
		attrKeys := countBattleRewardAttrKeys(reward.AttrExp)
		if !battleRewardNeedsUserLock(len(itemIDs), attrKeys, applyAttrExp) && len(reward.Contributions) == 0 {
			continue
		}
		metrics.rewardUsers++
		lockStartedAt := time.Now()
		if err := s.lockUserTx(ctx, tx, userID); err != nil {
			return err
		}
		metrics.rewardUserLockElapsed += time.Since(lockStartedAt)
		if err := validateBattleContributions(reward); err != nil {
			return err
		}
		fresh, receiptErr := s.recordBattleRewardReceiptTx(ctx, tx, runID, userID, reward.ItemRewards)
		if receiptErr != nil {
			return receiptErr
		}
		if !fresh {
			continue
		}
		if err := s.recordEffectContributionsTx(ctx, tx, userID, reward.Contributions); err != nil {
			return err
		}
		for _, itemID := range itemIDs {
			qty := reward.ItemRewards[itemID]
			itemStartedAt := time.Now()
			if err := s.GrantByTemplateTx(ctx, tx, userID, itemID, qty); err != nil {
				return err
			}
			if err := s.ledgerLogTx(ctx, tx, userID, itemID, qty, InventoryLedgerMeta{
				SourceSurface: "battle_room",
				TargetSurface: "inventory",
				EventType:     WarehouseChangeEventSystemToWarehouse,
				Details: map[string]any{
					"roomId": roomID,
					"runId":  runID,
					"type":   rewardType,
				},
			}); err != nil {
				return err
			}
			metrics.rewardItemElapsed += time.Since(itemStartedAt)
			metrics.rewardItemKinds++
			metrics.rewardItemQuantity += qty
		}
		if applyAttrExp && attrKeys > 0 {
			attrStartedAt := time.Now()
			if err := s.applyAttrExpLockedTx(ctx, tx, userID, reward.AttrExp); err != nil {
				return err
			}
			metrics.rewardAttrElapsed += time.Since(attrStartedAt)
			metrics.rewardAttrKeys += attrKeys
		}
	}
	return nil
}

type battleRoomJourneyResetState struct {
	Status              string
	EnteringStartedAt   *time.Time
	RecoveringStartedAt *time.Time
	RecoveryEndsAt      *time.Time
	NextTickAt          time.Time
}

type battleRoomUnitRepairMismatch struct {
	UserID    int64
	FromReady bool
	ToReady   bool
	FromPos   string
	ToPos     string
}

type battleRoomUnitRepairPlan struct {
	RoomID                int64
	MemberUserIDs         []int64
	UnitOwnerUserIDs      []int64
	MissingPlayerUnits    []int64
	PlayerUnitMismatches  []battleRoomUnitRepairMismatch
	OrphanUnits           []BattleRoomUnit
	UnexpectedPlayerUnits []BattleRoomUnit
}

func (p battleRoomUnitRepairPlan) hasChanges() bool {
	return len(p.MissingPlayerUnits) > 0 ||
		len(p.PlayerUnitMismatches) > 0 ||
		len(p.OrphanUnits) > 0 ||
		len(p.UnexpectedPlayerUnits) > 0
}

func (p battleRoomUnitRepairPlan) canonicalOrphans() []BattleRoomUnit {
	out := make([]BattleRoomUnit, 0, len(p.OrphanUnits)+len(p.UnexpectedPlayerUnits))
	out = append(out, p.OrphanUnits...)
	out = append(out, p.UnexpectedPlayerUnits...)
	return out
}

func sortedInt64Keys[T any](rows map[int64]T) []int64 {
	out := make([]int64, 0, len(rows))
	for key := range rows {
		if key > 0 {
			out = append(out, key)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

func planBattleRoomUnitRepair(roomID int64, members []BattleRoomMember, units []BattleRoomUnit) battleRoomUnitRepairPlan {
	memberByUser := make(map[int64]BattleRoomMember, len(members))
	memberUserIDs := make([]int64, 0, len(members))
	for _, member := range members {
		if member.UserID <= 0 {
			continue
		}
		memberByUser[member.UserID] = member
		memberUserIDs = append(memberUserIDs, member.UserID)
	}
	sort.Slice(memberUserIDs, func(i, j int) bool { return memberUserIDs[i] < memberUserIDs[j] })

	ownerSet := make(map[int64]struct{}, len(units))
	playerUnitsByOwner := map[int64][]BattleRoomUnit{}
	orphanUnits := make([]BattleRoomUnit, 0)
	unexpectedPlayerUnits := make([]BattleRoomUnit, 0)
	for _, unit := range units {
		if unit.OwnerUserID > 0 {
			ownerSet[unit.OwnerUserID] = struct{}{}
		}
		member, ok := memberByUser[unit.OwnerUserID]
		if !ok {
			orphanUnits = append(orphanUnits, unit)
			continue
		}
		if strings.TrimSpace(unit.UnitType) != "player" {
			continue
		}
		if unit.UnitID != 0 {
			unexpectedPlayerUnits = append(unexpectedPlayerUnits, unit)
			continue
		}
		if unit.OwnerUserID != member.UserID {
			unexpectedPlayerUnits = append(unexpectedPlayerUnits, unit)
			continue
		}
		playerUnitsByOwner[unit.OwnerUserID] = append(playerUnitsByOwner[unit.OwnerUserID], unit)
	}

	unitOwnerUserIDs := make([]int64, 0, len(ownerSet))
	for _, userID := range sortedInt64Keys(ownerSet) {
		unitOwnerUserIDs = append(unitOwnerUserIDs, userID)
	}

	plan := battleRoomUnitRepairPlan{
		RoomID:           roomID,
		MemberUserIDs:    memberUserIDs,
		UnitOwnerUserIDs: unitOwnerUserIDs,
		OrphanUnits:      orphanUnits,
	}
	if len(unexpectedPlayerUnits) > 0 {
		plan.UnexpectedPlayerUnits = append(plan.UnexpectedPlayerUnits, unexpectedPlayerUnits...)
	}

	for _, userID := range memberUserIDs {
		member := memberByUser[userID]
		playerUnits := playerUnitsByOwner[userID]
		if len(playerUnits) == 0 {
			plan.MissingPlayerUnits = append(plan.MissingPlayerUnits, userID)
			continue
		}
		canonical := playerUnits[0]
		if normalizedBattlePosition(canonical.Position) != normalizedBattlePosition(member.Position) || canonical.IsReady != member.IsReady {
			plan.PlayerUnitMismatches = append(plan.PlayerUnitMismatches, battleRoomUnitRepairMismatch{
				UserID:    userID,
				FromReady: canonical.IsReady,
				ToReady:   member.IsReady,
				FromPos:   normalizedBattlePosition(canonical.Position),
				ToPos:     normalizedBattlePosition(member.Position),
			})
		}
		if len(playerUnits) > 1 {
			plan.UnexpectedPlayerUnits = append(plan.UnexpectedPlayerUnits, playerUnits[1:]...)
		}
	}
	return plan
}

func filterBattleRoomUnitsByMembers(roomID int64, members []BattleRoomMember, units []BattleRoomUnit) ([]BattleRoomUnit, []BattleRoomUnit) {
	allowed := make(map[int64]struct{}, len(members))
	for _, member := range members {
		if member.UserID > 0 {
			allowed[member.UserID] = struct{}{}
		}
	}
	filtered := make([]BattleRoomUnit, 0, len(units))
	orphaned := make([]BattleRoomUnit, 0)
	for _, unit := range units {
		if _, ok := allowed[unit.OwnerUserID]; !ok {
			orphaned = append(orphaned, unit)
			continue
		}
		filtered = append(filtered, unit)
	}
	if len(orphaned) > 0 {
		log.Printf(
			"battle room detail filtered orphan units room_id=%d orphan_units=%s",
			roomID,
			battleRoomUnitsLogValue(orphaned),
		)
	}
	return filtered, orphaned
}

func battleRoomUnitsLogValue(units []BattleRoomUnit) string {
	if len(units) == 0 {
		return "[]"
	}
	parts := make([]string, 0, len(units))
	for _, unit := range units {
		parts = append(parts, strings.Join([]string{
			"owner=" + int64ToString(unit.OwnerUserID),
			"type=" + strings.TrimSpace(unit.UnitType),
			"unit=" + int64ToString(unit.UnitID),
			"kitty=" + int64PtrToString(unit.KittyID),
		}, ","))
	}
	sort.Strings(parts)
	return "[" + strings.Join(parts, ";") + "]"
}

func int64ToString(value int64) string {
	return strconv.FormatInt(value, 10)
}

func int64PtrToString(value *int64) string {
	if value == nil {
		return "nil"
	}
	return int64ToString(*value)
}

func battleFailureRecoveryDuration(base time.Duration, consecutiveFailures int) time.Duration {
	if base < 0 {
		base = 0
	}
	if consecutiveFailures < 0 {
		consecutiveFailures = 0
	}
	extraFailures := consecutiveFailures - 1
	if extraFailures < 0 {
		extraFailures = 0
	}
	recovery := base + time.Duration(extraFailures)*15*time.Second
	maxRecovery := 600 * time.Second
	if recovery > maxRecovery {
		return maxRecovery
	}
	return recovery
}

func battleNextConsecutiveFailures(current int, wave int) int {
	if wave > 3 {
		return 0
	}
	if current < 0 {
		current = 0
	}
	return current + 1
}

func (s *Store) resetBattleRoomJourneyTx(ctx context.Context, tx pgx.Tx, roomID int64, next battleRoomJourneyResetState, phase string, schemaVersion int) error {
	_, err := tx.Exec(ctx, `
		UPDATE battle_rooms
		SET status = $2,
		    current_run_id = NULL,
		    completed_runs = 0,
		    seed = 0,
		    entering_started_at = $3,
		    recovering_started_at = $4,
		    recovery_ends_at = $5,
		    owner_worker_id = '',
		    lease_until = NULL,
		    next_tick_at = $6,
		    phase = $7,
		    schema_version = $8,
		    updated_at = now()
		WHERE id = $1
	`, roomID, next.Status, next.EnteringStartedAt, next.RecoveringStartedAt, next.RecoveryEndsAt, next.NextTickAt, phase, schemaVersion)
	return err
}

func normalizedBattlePosition(position string) string {
	switch strings.TrimSpace(strings.ToLower(position)) {
	case "midguard":
		return "midguard"
	case "rearguard":
		return "rearguard"
	default:
		return "vanguard"
	}
}

func worldMaxTeamSizeFromAny(raw any) int {
	switch typed := raw.(type) {
	case int:
		if typed > 0 {
			return typed
		}
	case int32:
		if typed > 0 {
			return int(typed)
		}
	case int64:
		if typed > 0 {
			return int(typed)
		}
	case float32:
		value := int(typed)
		if value > 0 {
			return value
		}
	case float64:
		value := int(typed)
		if value > 0 {
			return value
		}
	case json.Number:
		if value, err := typed.Int64(); err == nil && value > 0 {
			return int(value)
		}
	}
	return 0
}

func (s *Store) battleWorldMaxTeamSizeByDungeonID(dungeonID string) (int, bool) {
	dungeonID = strings.TrimSpace(dungeonID)
	if dungeonID == "" {
		return 0, false
	}
	for _, raw := range s.gameData.World {
		id, _ := raw["id"].(string)
		if strings.TrimSpace(id) != dungeonID {
			continue
		}
		maxTeamSize := worldMaxTeamSizeFromAny(raw["maxTeamSize"])
		if maxTeamSize <= 0 {
			maxTeamSize = 3
		}
		return maxTeamSize, true
	}
	return 0, false
}

func (s *Store) ListPublicBattleRooms(ctx context.Context, dungeonID string, limit int) ([]BattleRoom, error) {
	if limit <= 0 {
		limit = 100
	}
	if limit > 500 {
		limit = 500
	}
	dungeonID = strings.TrimSpace(dungeonID)
	rows, err := s.db.Query(ctx, `
		SELECT id, owner_id, dungeon_id, room_name, auto_reenter_on_defeat, disable_entry_round, is_public, is_fog_mode, status, difficulty_level, completed_runs, consecutive_failures, current_run_id, seed, entering_started_at, recovering_started_at, recovery_ends_at, created_at, updated_at, phase, schema_version,
		       COALESCE((SELECT COUNT(*)::INT FROM battle_room_member_units u WHERE u.room_id = battle_rooms.id), (SELECT COUNT(*)::INT FROM battle_room_members m WHERE m.room_id = battle_rooms.id)) AS member_count
		FROM battle_rooms
		WHERE is_public = TRUE AND ($1 = '' OR dungeon_id = $1)
		ORDER BY updated_at DESC, id DESC
		LIMIT $2
	`, dungeonID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]BattleRoom, 0, limit)
	for rows.Next() {
		var room BattleRoom
		var currentRunID *int64
		if err := rows.Scan(
			&room.ID,
			&room.OwnerID,
			&room.DungeonID,
			&room.RoomName,
			&room.AutoReenterOnDefeat,
			&room.DisableEntryRound,
			&room.IsPublic,
			&room.IsFogMode,
			&room.Status,
			&room.DifficultyLevel,
			&room.CompletedRuns,
			&room.ConsecutiveFailures,
			&currentRunID,
			&room.Seed,
			&room.EnteringStartedAt,
			&room.RecoveringStartedAt,
			&room.RecoveryEndsAt,
			&room.CreatedAt,
			&room.UpdatedAt,
			&room.Phase,
			&room.SchemaVersion,
			&room.MemberCount,
		); err != nil {
			return nil, err
		}
		room.CurrentRunID = currentRunID
		out = append(out, room)
	}
	return out, rows.Err()
}

func (s *Store) ListBattleRoomsByStatus(ctx context.Context, statuses []string, limit int) ([]BattleRoom, error) {
	if len(statuses) == 0 {
		return []BattleRoom{}, nil
	}
	if limit <= 0 {
		limit = 200
	}
	if limit > 1000 {
		limit = 1000
	}
	normalized := make([]string, 0, len(statuses))
	seen := map[string]struct{}{}
	for _, status := range statuses {
		status = strings.TrimSpace(strings.ToLower(status))
		if status == "" {
			continue
		}
		if _, ok := seen[status]; ok {
			continue
		}
		seen[status] = struct{}{}
		normalized = append(normalized, status)
	}
	if len(normalized) == 0 {
		return []BattleRoom{}, nil
	}
	rows, err := s.db.Query(ctx, `
		SELECT id, owner_id, dungeon_id, room_name, auto_reenter_on_defeat, disable_entry_round, is_public, is_fog_mode, status, difficulty_level, completed_runs, consecutive_failures, current_run_id, seed, entering_started_at, recovering_started_at, recovery_ends_at, created_at, updated_at, phase, schema_version
		FROM battle_rooms
		WHERE status = ANY($1::text[])
		ORDER BY updated_at ASC, id ASC
		LIMIT $2
	`, normalized, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]BattleRoom, 0, limit)
	for rows.Next() {
		item, err := scanBattleRoomRow(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *Store) ClaimBattleRoomsForTick(
	ctx context.Context,
	workerID string,
	statuses []string,
	limit int,
	leaseDuration time.Duration,
	now time.Time,
) ([]BattleRoom, error) {
	workerID = strings.TrimSpace(workerID)
	if workerID == "" || len(statuses) == 0 {
		return []BattleRoom{}, nil
	}
	if limit <= 0 {
		limit = 200
	}
	if leaseDuration <= 0 {
		leaseDuration = 3 * time.Second
	}
	leaseUntil := now.Add(leaseDuration)
	rows, err := s.db.Query(ctx, `
		WITH candidates AS (
			SELECT r.id
			FROM battle_rooms r
			LEFT JOIN battle_runs cr ON cr.id = r.current_run_id
			WHERE r.status = ANY($1::text[])
			  AND r.schema_version = 1
			  AND r.next_tick_at <= $2
			  AND (r.status <> 'playing' OR COALESCE(cr.status, '') <> 'settling')
			  AND (
				r.lease_until IS NULL
				OR r.lease_until < $2
				OR r.owner_worker_id = $3
			  )
			ORDER BY
				CASE
					WHEN r.status = 'playing' AND r.current_run_id IS NULL THEN 0
					WHEN r.status = 'recovering' THEN 1
					ELSE 2
				END ASC,
				r.next_tick_at ASC,
				r.updated_at ASC,
				r.id ASC
			LIMIT $4
			FOR UPDATE OF r SKIP LOCKED
		)
		UPDATE battle_rooms AS r
		SET owner_worker_id = $3,
		    lease_until = $5
		FROM candidates c
		WHERE r.id = c.id
		RETURNING r.id, r.owner_id, r.dungeon_id, r.room_name, r.auto_reenter_on_defeat, r.disable_entry_round, r.is_public, r.is_fog_mode, r.status, r.difficulty_level, r.completed_runs, r.consecutive_failures, r.current_run_id, r.seed, r.entering_started_at, r.recovering_started_at, r.recovery_ends_at, r.created_at, r.updated_at, r.phase, r.schema_version
	`, statuses, now, workerID, limit, leaseUntil)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]BattleRoom, 0, limit)
	for rows.Next() {
		room, scanErr := scanBattleRoomRow(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		out = append(out, room)
	}
	return out, rows.Err()
}

func (s *Store) ClaimBattleRoomsByIDsForTick(
	ctx context.Context,
	workerID string,
	statuses []string,
	roomIDs []int64,
	leaseDuration time.Duration,
	now time.Time,
) ([]BattleRoom, error) {
	workerID = strings.TrimSpace(workerID)
	if workerID == "" || len(statuses) == 0 || len(roomIDs) == 0 {
		return []BattleRoom{}, nil
	}
	if leaseDuration <= 0 {
		leaseDuration = 3 * time.Second
	}
	leaseUntil := now.Add(leaseDuration)
	rows, err := s.db.Query(ctx, `
		WITH input_ids AS (
			SELECT UNNEST($4::bigint[]) AS id
		), candidates AS (
			SELECT r.id
			FROM battle_rooms r
			JOIN input_ids i ON i.id = r.id
			LEFT JOIN battle_runs cr ON cr.id = r.current_run_id
			WHERE r.status = ANY($1::text[])
			  AND r.schema_version = 1
			  AND r.next_tick_at <= $2
			  AND (r.status <> 'playing' OR COALESCE(cr.status, '') <> 'settling')
			  AND (
				r.lease_until IS NULL
				OR r.lease_until < $2
				OR r.owner_worker_id = $3
			)
			ORDER BY
				CASE
					WHEN r.status = 'playing' AND r.current_run_id IS NULL THEN 0
					WHEN r.status = 'recovering' THEN 1
					ELSE 2
				END ASC,
				r.next_tick_at ASC,
				r.updated_at ASC,
				r.id ASC
			FOR UPDATE OF r SKIP LOCKED
		)
		UPDATE battle_rooms AS r
		SET owner_worker_id = $3,
		    lease_until = $5
		FROM candidates c
		WHERE r.id = c.id
		RETURNING r.id, r.owner_id, r.dungeon_id, r.room_name, r.auto_reenter_on_defeat, r.disable_entry_round, r.is_public, r.is_fog_mode, r.status, r.difficulty_level, r.completed_runs, r.consecutive_failures, r.current_run_id, r.seed, r.entering_started_at, r.recovering_started_at, r.recovery_ends_at, r.created_at, r.updated_at, r.phase, r.schema_version
	`, statuses, now, workerID, roomIDs, leaseUntil)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]BattleRoom, 0, len(roomIDs))
	for rows.Next() {
		room, scanErr := scanBattleRoomRow(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		out = append(out, room)
	}
	return out, rows.Err()
}

func (s *Store) ClaimBattlePlaybackRooms(
	ctx context.Context,
	workerID string,
	limit int,
	leaseDuration time.Duration,
	now time.Time,
) ([]BattleRoom, error) {
	workerID = strings.TrimSpace(workerID)
	if workerID == "" {
		return []BattleRoom{}, nil
	}
	if limit <= 0 {
		limit = 32
	}
	if leaseDuration <= 0 {
		leaseDuration = 3 * time.Second
	}
	leaseUntil := now.Add(leaseDuration)
	rows, err := s.db.Query(ctx, `
		WITH candidates AS (
			SELECT r.id
			FROM battle_rooms r
			JOIN battle_runs cr ON cr.id = r.current_run_id
			WHERE r.status = $1
			  AND r.schema_version = 1
			  AND cr.status = $2
			  AND r.next_tick_at <= $3
			  AND (
				r.lease_until IS NULL
				OR r.lease_until < $3
				OR r.owner_worker_id = $4
			)
			ORDER BY r.next_tick_at ASC, r.updated_at ASC, r.id ASC
			LIMIT $5
			FOR UPDATE OF r SKIP LOCKED
		)
		UPDATE battle_rooms AS r
		SET owner_worker_id = $4,
		    lease_until = $6
		FROM candidates c
		WHERE r.id = c.id
		RETURNING r.id, r.owner_id, r.dungeon_id, r.room_name, r.auto_reenter_on_defeat, r.disable_entry_round, r.is_public, r.is_fog_mode, r.status, r.difficulty_level, r.completed_runs, r.consecutive_failures, r.current_run_id, r.seed, r.entering_started_at, r.recovering_started_at, r.recovery_ends_at, r.created_at, r.updated_at, r.phase, r.schema_version
	`, BattleRoomStatusPlaying, BattleRunStatusRunning, now, workerID, limit, leaseUntil)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]BattleRoom, 0, limit)
	for rows.Next() {
		room, scanErr := scanBattleRoomRow(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		out = append(out, room)
	}
	return out, rows.Err()
}

func (s *Store) ClaimBattleStateAdvanceRooms(
	ctx context.Context,
	workerID string,
	statuses []string,
	limit int,
	leaseDuration time.Duration,
	now time.Time,
) ([]BattleRoom, error) {
	workerID = strings.TrimSpace(workerID)
	if workerID == "" || len(statuses) == 0 {
		return []BattleRoom{}, nil
	}
	if limit <= 0 {
		limit = 200
	}
	if leaseDuration <= 0 {
		leaseDuration = 3 * time.Second
	}
	leaseUntil := now.Add(leaseDuration)
	rows, err := s.db.Query(ctx, `
		WITH candidates AS (
			SELECT r.id
			FROM battle_rooms r
			LEFT JOIN battle_runs cr ON cr.id = r.current_run_id
			WHERE r.status = ANY($1::text[])
			  AND r.schema_version = 1
			  AND r.next_tick_at <= $2
			  AND (
				r.status <> 'playing'
				OR r.current_run_id IS NULL
				OR COALESCE(cr.status, '') = 'compiled'
			  )
			  AND (
				r.lease_until IS NULL
				OR r.lease_until < $2
				OR r.owner_worker_id = $3
			  )
			ORDER BY r.next_tick_at ASC, r.updated_at ASC, r.id ASC
			LIMIT $4
			FOR UPDATE OF r SKIP LOCKED
		)
		UPDATE battle_rooms AS r
		SET owner_worker_id = $3,
		    lease_until = $5
		FROM candidates c
		WHERE r.id = c.id
		RETURNING r.id, r.owner_id, r.dungeon_id, r.room_name, r.auto_reenter_on_defeat, r.disable_entry_round, r.is_public, r.is_fog_mode, r.status, r.difficulty_level, r.completed_runs, r.consecutive_failures, r.current_run_id, r.seed, r.entering_started_at, r.recovering_started_at, r.recovery_ends_at, r.created_at, r.updated_at, r.phase, r.schema_version
	`, statuses, now, workerID, limit, leaseUntil)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]BattleRoom, 0, limit)
	for rows.Next() {
		room, scanErr := scanBattleRoomRow(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		out = append(out, room)
	}
	return out, rows.Err()
}

// ClaimBattleReadyNextRooms prioritizes rooms that are already ready to start the next run
// (playing + current_run_id is null), so they don't get starved behind other room states.
func (s *Store) ClaimBattleReadyNextRooms(
	ctx context.Context,
	workerID string,
	limit int,
	leaseDuration time.Duration,
	now time.Time,
) ([]BattleRoom, error) {
	workerID = strings.TrimSpace(workerID)
	if workerID == "" {
		return []BattleRoom{}, nil
	}
	if limit <= 0 {
		limit = 32
	}
	if leaseDuration <= 0 {
		leaseDuration = 3 * time.Second
	}
	leaseUntil := now.Add(leaseDuration)
	rows, err := s.db.Query(ctx, `
		WITH candidates AS (
			SELECT r.id
			FROM battle_rooms r
			WHERE r.status = 'playing'
			  AND r.schema_version = 1
			  AND r.current_run_id IS NULL
			  AND r.next_tick_at <= $1
			  AND (
				r.lease_until IS NULL
				OR r.lease_until < $1
				OR r.owner_worker_id = $2
			  )
			ORDER BY r.next_tick_at ASC, r.updated_at ASC, r.id ASC
			LIMIT $3
			FOR UPDATE OF r SKIP LOCKED
		)
		UPDATE battle_rooms AS r
		SET owner_worker_id = $2,
		    lease_until = $4
		FROM candidates c
		WHERE r.id = c.id
		RETURNING r.id, r.owner_id, r.dungeon_id, r.room_name, r.auto_reenter_on_defeat, r.disable_entry_round, r.is_public, r.is_fog_mode, r.status, r.difficulty_level, r.completed_runs, r.consecutive_failures, r.current_run_id, r.seed, r.entering_started_at, r.recovering_started_at, r.recovery_ends_at, r.created_at, r.updated_at, r.phase, r.schema_version
	`, now, workerID, limit, leaseUntil)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]BattleRoom, 0, limit)
	for rows.Next() {
		room, scanErr := scanBattleRoomRow(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		out = append(out, room)
	}
	return out, rows.Err()
}

func (s *Store) ClaimBattleStateAdvanceRoomsByIDs(
	ctx context.Context,
	workerID string,
	statuses []string,
	roomIDs []int64,
	leaseDuration time.Duration,
	now time.Time,
) ([]BattleRoom, error) {
	workerID = strings.TrimSpace(workerID)
	if workerID == "" || len(statuses) == 0 || len(roomIDs) == 0 {
		return []BattleRoom{}, nil
	}
	if leaseDuration <= 0 {
		leaseDuration = 3 * time.Second
	}
	leaseUntil := now.Add(leaseDuration)
	rows, err := s.db.Query(ctx, `
		WITH input_ids AS (
			SELECT UNNEST($4::bigint[]) AS id
		), candidates AS (
			SELECT r.id
			FROM battle_rooms r
			JOIN input_ids i ON i.id = r.id
			LEFT JOIN battle_runs cr ON cr.id = r.current_run_id
			WHERE r.status = ANY($1::text[])
			  AND r.schema_version = 1
			  AND r.next_tick_at <= $2
			  AND (
				r.status <> 'playing'
				OR r.current_run_id IS NULL
				OR COALESCE(cr.status, '') = 'compiled'
			  )
			  AND (
				r.lease_until IS NULL
				OR r.lease_until < $2
				OR r.owner_worker_id = $3
			)
			ORDER BY r.next_tick_at ASC, r.updated_at ASC, r.id ASC
			FOR UPDATE OF r SKIP LOCKED
		)
		UPDATE battle_rooms AS r
		SET owner_worker_id = $3,
		    lease_until = $5
		FROM candidates c
		WHERE r.id = c.id
		RETURNING r.id, r.owner_id, r.dungeon_id, r.room_name, r.auto_reenter_on_defeat, r.disable_entry_round, r.is_public, r.is_fog_mode, r.status, r.difficulty_level, r.completed_runs, r.consecutive_failures, r.current_run_id, r.seed, r.entering_started_at, r.recovering_started_at, r.recovery_ends_at, r.created_at, r.updated_at, r.phase, r.schema_version
	`, statuses, now, workerID, roomIDs, leaseUntil)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]BattleRoom, 0, len(roomIDs))
	for rows.Next() {
		room, scanErr := scanBattleRoomRow(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		out = append(out, room)
	}
	return out, rows.Err()
}

// NextBattleRoomTickAt peeks only legacy V1 rooms; V2 phases have dedicated loops.
func (s *Store) NextBattleRoomTickAt(ctx context.Context, statuses []string) (time.Time, bool, error) {
	if len(statuses) == 0 {
		return time.Time{}, false, nil
	}
	normalized := make([]string, 0, len(statuses))
	seen := map[string]struct{}{}
	for _, status := range statuses {
		status = strings.TrimSpace(strings.ToLower(status))
		if status == "" {
			continue
		}
		if _, ok := seen[status]; ok {
			continue
		}
		seen[status] = struct{}{}
		normalized = append(normalized, status)
	}
	if len(normalized) == 0 {
		return time.Time{}, false, nil
	}
	var nextAt *time.Time
	if err := s.db.QueryRow(ctx, `
		SELECT MIN(r.next_tick_at)
		FROM battle_rooms r
		LEFT JOIN battle_runs cr ON cr.id = r.current_run_id
		WHERE r.status = ANY($1::text[])
		  AND r.schema_version = 1
		  AND (r.status <> 'playing' OR COALESCE(cr.status, '') <> 'settling')
	`, normalized).Scan(&nextAt); err != nil {
		return time.Time{}, false, err
	}
	if nextAt == nil || nextAt.IsZero() {
		return time.Time{}, false, nil
	}
	return *nextAt, true, nil
}

func (s *Store) ScheduleBattleRoomNextTick(ctx context.Context, roomID int64, workerID string, nextTickAt, leaseUntil time.Time) error {
	if roomID <= 0 {
		return ErrBattleRoomConfigInvalid
	}
	workerID = strings.TrimSpace(workerID)
	if workerID == "" {
		return ErrBattleRoomConfigInvalid
	}
	if nextTickAt.IsZero() {
		nextTickAt = time.Now()
	}
	if leaseUntil.IsZero() {
		leaseUntil = time.Now().Add(3 * time.Second)
	}
	_, err := s.db.Exec(ctx, `
		UPDATE battle_rooms
		SET next_tick_at = $3,
		    lease_until = $4
		WHERE id = $1
		  AND owner_worker_id = $2
	`, roomID, workerID, nextTickAt, leaseUntil)
	return err
}

const holdBattleRoomPlaybackLeaseSQL = `
	UPDATE battle_rooms
	SET next_tick_at = $4,
	    owner_worker_id = $3,
	    lease_until = $4,
	    updated_at = now()
	WHERE id = $1
	  AND owner_worker_id = $2
	  AND status = $5
`

func (s *Store) HoldBattleRoomPlaybackLease(ctx context.Context, roomID int64, workerID string, playbackOwnerToken string, leaseUntil time.Time) error {
	if roomID <= 0 {
		return ErrBattleRoomConfigInvalid
	}
	workerID = strings.TrimSpace(workerID)
	playbackOwnerToken = strings.TrimSpace(playbackOwnerToken)
	if workerID == "" || playbackOwnerToken == "" {
		return ErrBattleRoomConfigInvalid
	}
	if leaseUntil.IsZero() {
		leaseUntil = time.Now().Add(30 * time.Second)
	}
	tag, err := s.db.Exec(ctx, holdBattleRoomPlaybackLeaseSQL, roomID, workerID, playbackOwnerToken, leaseUntil, BattleRoomStatusPlaying)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrBattleRoomStateInvalid
	}
	return nil
}

const handoffBattleRoomPlaybackLeaseSQL = `
	UPDATE battle_rooms
	SET next_tick_at = $5,
	    owner_worker_id = $4,
	    lease_until = $5,
	    updated_at = now()
	WHERE id = $1
	  AND current_run_id = $2
	  AND status = $6
	  AND (
		owner_worker_id = $3
		OR lease_until IS NULL
		OR lease_until < $7
	  )
`

func (s *Store) HandoffBattleRoomPlaybackLease(ctx context.Context, roomID int64, runID int64, workerID string, playbackOwnerToken string, leaseUntil time.Time, now time.Time) (bool, error) {
	if roomID <= 0 || runID <= 0 {
		return false, ErrBattleRoomConfigInvalid
	}
	workerID = strings.TrimSpace(workerID)
	playbackOwnerToken = strings.TrimSpace(playbackOwnerToken)
	if workerID == "" || playbackOwnerToken == "" {
		return false, ErrBattleRoomConfigInvalid
	}
	if leaseUntil.IsZero() {
		leaseUntil = time.Now().Add(30 * time.Second)
	}
	if now.IsZero() {
		now = time.Now()
	}
	tag, err := s.db.Exec(ctx, handoffBattleRoomPlaybackLeaseSQL, roomID, runID, workerID, playbackOwnerToken, leaseUntil, BattleRoomStatusPlaying, now)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}

const extendBattleRoomPlaybackLeaseSQL = `
	UPDATE battle_rooms
	SET next_tick_at = $4,
	    lease_until = $4
	WHERE id = $1
	  AND status = $5
	  AND current_run_id = $2
	  AND owner_worker_id = $3
	  AND (lease_until IS NULL OR lease_until < $4)
`

func (s *Store) ExtendBattleRoomPlaybackLease(ctx context.Context, roomID int64, runID int64, playbackOwnerToken string, leaseUntil time.Time) (bool, error) {
	if roomID <= 0 || runID <= 0 {
		return false, ErrBattleRoomConfigInvalid
	}
	playbackOwnerToken = strings.TrimSpace(playbackOwnerToken)
	if playbackOwnerToken == "" {
		return false, ErrBattleRoomConfigInvalid
	}
	if leaseUntil.IsZero() {
		leaseUntil = time.Now().Add(15 * time.Second)
	}
	tag, err := s.db.Exec(ctx, extendBattleRoomPlaybackLeaseSQL, roomID, runID, playbackOwnerToken, leaseUntil, BattleRoomStatusPlaying)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}

func (s *Store) GetBattleRoomDetail(ctx context.Context, roomID int64) (BattleRoomDetail, error) {
	if roomID <= 0 {
		return BattleRoomDetail{}, ErrBattleRoomNotFound
	}
	room, err := s.getBattleRoom(ctx, roomID)
	if err != nil {
		return BattleRoomDetail{}, err
	}
	members, err := s.listBattleRoomMembers(ctx, roomID)
	if err != nil {
		return BattleRoomDetail{}, err
	}
	units, err := s.listBattleRoomUnits(ctx, roomID)
	if err != nil {
		return BattleRoomDetail{}, err
	}
	units, _ = filterBattleRoomUnitsByMembers(roomID, members, units)
	return BattleRoomDetail{Room: room, Members: members, Units: units}, nil
}

func (s *Store) GetBattleRoomMemberUserIDs(ctx context.Context, roomID int64) ([]int64, error) {
	if roomID <= 0 {
		return nil, ErrBattleRoomNotFound
	}
	rows, err := s.db.Query(ctx, `SELECT user_id FROM battle_room_members WHERE room_id = $1`, roomID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]int64, 0, 4)
	for rows.Next() {
		var uid int64
		if err := rows.Scan(&uid); err != nil {
			return nil, err
		}
		out = append(out, uid)
	}
	return out, rows.Err()
}

func (s *Store) GetBattleRoomDetailHot(ctx context.Context, roomID int64) (BattleRoomDetail, error) {
	if roomID <= 0 {
		return BattleRoomDetail{}, ErrBattleRoomNotFound
	}
	batch := &pgx.Batch{}
	batch.Queue(`
		SELECT id, owner_id, dungeon_id, room_name, auto_reenter_on_defeat, disable_entry_round, is_public, is_fog_mode, status, difficulty_level, completed_runs, consecutive_failures, current_run_id, seed, entering_started_at, recovering_started_at, recovery_ends_at, created_at, updated_at, phase, schema_version
		FROM battle_rooms
		WHERE id = $1
	`, roomID)
	batch.Queue(`
		SELECT m.room_id, m.user_id, COALESCE(u.username, ''), m.position, m.is_ready, m.ready_at, m.joined_at, m.updated_at
		FROM battle_room_members m
		LEFT JOIN users u ON u.id = m.user_id
		WHERE room_id = $1
		ORDER BY m.joined_at ASC, m.user_id ASC
	`, roomID)
	batch.Queue(`
		SELECT u.room_id, u.owner_user_id, COALESCE(owner.username, ''), u.unit_type, u.unit_id, u.kitty_id, COALESCE(k.name, ''), u.position, u.is_ready, u.joined_at, u.updated_at
		FROM battle_room_member_units u
		LEFT JOIN users owner ON owner.id = u.owner_user_id
		LEFT JOIN player_kitties k ON k.id = u.kitty_id
		WHERE u.room_id = $1
		ORDER BY u.joined_at ASC, u.owner_user_id ASC, CASE WHEN u.unit_type = 'player' THEN 0 ELSE 1 END, u.unit_id ASC
	`, roomID)
	results := s.db.SendBatch(ctx, batch)
	defer results.Close()

	roomRows, err := results.Query()
	if err != nil {
		return BattleRoomDetail{}, err
	}
	if !roomRows.Next() {
		roomRows.Close()
		return BattleRoomDetail{}, ErrBattleRoomNotFound
	}
	room, err := scanBattleRoomRow(roomRows)
	if err != nil {
		roomRows.Close()
		return BattleRoomDetail{}, err
	}
	if err := roomRows.Err(); err != nil {
		roomRows.Close()
		return BattleRoomDetail{}, err
	}
	roomRows.Close()

	memberRows, err := results.Query()
	if err != nil {
		return BattleRoomDetail{}, err
	}
	members := make([]BattleRoomMember, 0, 4)
	for memberRows.Next() {
		item, scanErr := scanBattleRoomMemberRow(memberRows)
		if scanErr != nil {
			memberRows.Close()
			return BattleRoomDetail{}, scanErr
		}
		members = append(members, item)
	}
	if err := memberRows.Err(); err != nil {
		memberRows.Close()
		return BattleRoomDetail{}, err
	}
	memberRows.Close()

	unitRows, err := results.Query()
	if err != nil {
		return BattleRoomDetail{}, err
	}
	units := make([]BattleRoomUnit, 0, 8)
	for unitRows.Next() {
		item, scanErr := scanBattleRoomUnitRow(unitRows)
		if scanErr != nil {
			unitRows.Close()
			return BattleRoomDetail{}, scanErr
		}
		units = append(units, item)
	}
	if err := unitRows.Err(); err != nil {
		unitRows.Close()
		return BattleRoomDetail{}, err
	}
	unitRows.Close()

	units, _ = filterBattleRoomUnitsByMembers(roomID, members, units)
	return BattleRoomDetail{Room: room, Members: members, Units: units}, nil
}

func (s *Store) getBattleRoomDetailBatched(ctx context.Context, roomID int64) (BattleRoomDetail, error) {
	detail, err := s.GetBattleRoomDetailHot(ctx, roomID)
	if err != nil {
		return BattleRoomDetail{}, err
	}
	detail.Units, err = s.applyBattleRoomUnitPositionLocks(ctx, detail.Units)
	if err != nil {
		return BattleRoomDetail{}, err
	}
	return detail, nil
}

func (s *Store) CreateBattleRoom(ctx context.Context, ownerID int64, dungeonID, roomName string, autoReenterOnDefeat bool, isPublic bool, isFogMode bool, ownerPosition string, difficultyLevel int, schemaVersion int) (BattleRoomDetail, error) {
	dungeonID = strings.TrimSpace(dungeonID)
	roomName = strings.TrimSpace(roomName)
	if ownerID <= 0 || dungeonID == "" || roomName == "" {
		return BattleRoomDetail{}, ErrBattleRoomConfigInvalid
	}
	if _, ok := s.battleWorldMaxTeamSizeByDungeonID(dungeonID); !ok {
		return BattleRoomDetail{}, ErrBattleRoomConfigInvalid
	}
	var roomID int64
	err := s.withTx(ctx, func(tx pgx.Tx) error {
		if exists, err := s.userInAnyBattleRoomTx(ctx, tx, ownerID); err != nil {
			return err
		} else if exists {
			return ErrBattleRoomAlreadyMember
		}
		if schemaVersion <= 0 {
			schemaVersion = 1
		}
		if err := tx.QueryRow(ctx, `
			INSERT INTO battle_rooms(owner_id, dungeon_id, room_name, auto_reenter_on_defeat, is_public, is_fog_mode, status, difficulty_level, completed_runs, seed, schema_version, created_at, updated_at)
			VALUES($1, $2, $3, $4, $5, $6, $7, $8, 0, 0, $9, now(), now())
			RETURNING id
		`, ownerID, dungeonID, roomName, autoReenterOnDefeat, isPublic, isFogMode, BattleRoomStatusIdle, difficultyLevel, schemaVersion).Scan(&roomID); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `
			INSERT INTO battle_room_members(room_id, user_id, position, is_ready, joined_at, updated_at)
			VALUES($1, $2, $3, FALSE, now(), now())
		`, roomID, ownerID, normalizedBattlePosition(ownerPosition))
		if err != nil {
			return err
		}
		if err := s.upsertBattleRoomPlayerUnitTx(ctx, tx, roomID, ownerID, normalizedBattlePosition(ownerPosition), false); err != nil {
			return err
		}
		return s.ensureBattleRoomUnitsConsistentTx(ctx, tx, roomID, "create_room")
	})
	if err != nil {
		return BattleRoomDetail{}, err
	}
	return s.GetBattleRoomDetail(ctx, roomID)
}

func (s *Store) JoinBattleRoom(ctx context.Context, roomID, userID int64, position string) (BattleRoomDetail, error) {
	if roomID <= 0 || userID <= 0 {
		return BattleRoomDetail{}, ErrBattleRoomConfigInvalid
	}
	if err := s.withTx(ctx, func(tx pgx.Tx) error {
		if exists, err := s.userInAnyBattleRoomTx(ctx, tx, userID); err != nil {
			return err
		} else if exists {
			return ErrBattleRoomAlreadyMember
		}
		var isPublic bool
		var status string
		var dungeonID string
		err := tx.QueryRow(ctx, `
			SELECT is_public, status, dungeon_id
			FROM battle_rooms
			WHERE id = $1
			FOR UPDATE
		`, roomID).Scan(&isPublic, &status, &dungeonID)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrBattleRoomNotFound
		}
		if err != nil {
			return err
		}
		if !isPublic {
			return ErrBattleRoomForbidden
		}
		if status == BattleRoomStatusPlaying || status == BattleRoomStatusRecovering {
			return ErrBattleRoomStateInvalid
		}
		maxTeamSize, ok := s.battleWorldMaxTeamSizeByDungeonID(dungeonID)
		if !ok || maxTeamSize <= 0 {
			return ErrBattleRoomConfigInvalid
		}
		var memberCount int
		if err := tx.QueryRow(ctx, `
			SELECT COUNT(1)
			FROM battle_room_member_units
			WHERE room_id = $1
		`, roomID).Scan(&memberCount); err != nil {
			return err
		}
		if memberCount >= maxTeamSize {
			return ErrBattleRoomFull
		}
		_, err = tx.Exec(ctx, `
			INSERT INTO battle_room_members(room_id, user_id, position, is_ready, joined_at, updated_at)
			VALUES($1, $2, $3, FALSE, now(), now())
		`, roomID, userID, normalizedBattlePosition(position))
		if err != nil {
			return err
		}
		if err := s.upsertBattleRoomPlayerUnitTx(ctx, tx, roomID, userID, normalizedBattlePosition(position), false); err != nil {
			return err
		}
		if err := s.ensureBattleRoomUnitsConsistentTx(ctx, tx, roomID, "join_room"); err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `UPDATE battle_rooms SET updated_at = now() WHERE id = $1`, roomID)
		return err
	}); err != nil {
		return BattleRoomDetail{}, err
	}
	return s.GetBattleRoomDetail(ctx, roomID)
}

func (s *Store) UpdateBattleRoomMemberPosition(ctx context.Context, roomID, userID int64, position string) (BattleRoomDetail, error) {
	if roomID <= 0 || userID <= 0 {
		return BattleRoomDetail{}, ErrBattleRoomConfigInvalid
	}
	nextPosition := normalizedBattlePosition(position)
	if err := s.withTx(ctx, func(tx pgx.Tx) error {
		var status string
		err := tx.QueryRow(ctx, `
			SELECT status
			FROM battle_rooms
			WHERE id = $1
			FOR UPDATE
		`, roomID).Scan(&status)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrBattleRoomNotFound
		}
		if err != nil {
			return err
		}
		if status != BattleRoomStatusIdle && status != BattleRoomStatusPreparing {
			return ErrBattleRoomStateInvalid
		}
		tag, err := tx.Exec(ctx, `
			UPDATE battle_room_members
			SET position = $3,
			    updated_at = now()
			WHERE room_id = $1
			  AND user_id = $2
		`, roomID, userID, nextPosition)
		if err != nil {
			return err
		}
		if tag.RowsAffected() <= 0 {
			return ErrBattleRoomMemberMissing
		}
		if _, err := tx.Exec(ctx, `
			UPDATE battle_room_member_units
			SET position = $3,
			    updated_at = now()
			WHERE room_id = $1
			  AND owner_user_id = $2
			  AND unit_type = 'player'
			  AND unit_id = 0
		`, roomID, userID, nextPosition); err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `UPDATE battle_rooms SET updated_at = now() WHERE id = $1`, roomID)
		return err
	}); err != nil {
		return BattleRoomDetail{}, err
	}
	return s.GetBattleRoomDetail(ctx, roomID)
}

func (s *Store) LeaveBattleRoom(ctx context.Context, roomID, userID int64) (BattleRoomDetail, error) {
	if roomID <= 0 || userID <= 0 {
		return BattleRoomDetail{}, ErrBattleRoomConfigInvalid
	}
	retainedForRewardJobs := false
	if err := s.withTx(ctx, func(tx pgx.Tx) error {
		var ownerID int64
		var status string
		err := tx.QueryRow(ctx, `
			SELECT owner_id, status
			FROM battle_rooms
			WHERE id = $1
			FOR UPDATE
		`, roomID).Scan(&ownerID, &status)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrBattleRoomNotFound
		}
		if err != nil {
			return err
		}
		if status == BattleRoomStatusPlaying {
			return ErrBattleRoomStateInvalid
		}
		tag, err := tx.Exec(ctx, `
			DELETE FROM battle_room_members
			WHERE room_id = $1 AND user_id = $2
		`, roomID, userID)
		if err != nil {
			return err
		}
		if tag.RowsAffected() <= 0 {
			return ErrBattleRoomMemberMissing
		}
		if err := s.removeBattleRoomUnitsForUserTx(ctx, tx, roomID, userID); err != nil {
			return err
		}
		var memberCount int
		if err := tx.QueryRow(ctx, `
			SELECT COUNT(1)
			FROM battle_room_member_units
			WHERE room_id = $1
		`, roomID).Scan(&memberCount); err != nil {
			return err
		}
		if memberCount <= 0 {
			pendingRewards, err := s.battleRoomHasPendingRewardWorkTx(ctx, tx, roomID)
			if err != nil {
				return err
			}
			if pendingRewards {
				retainedForRewardJobs = true
				_, err = tx.Exec(ctx, `
					UPDATE battle_rooms
					SET is_public = FALSE,
					    status = 'idle',
					    entering_started_at = NULL,
					    recovering_started_at = NULL,
					    recovery_ends_at = NULL,
					    owner_worker_id = '',
					    lease_until = NULL,
					    phase = 'reward_cleanup',
					    updated_at = now()
					WHERE id = $1
				`, roomID)
				return err
			}
			_, err = tx.Exec(ctx, `DELETE FROM battle_rooms WHERE id = $1`, roomID)
			return err
		}
		if ownerID == userID {
			var nextOwner int64
			if err := tx.QueryRow(ctx, `
				SELECT user_id
				FROM battle_room_members
				WHERE room_id = $1
				ORDER BY joined_at ASC, user_id ASC
				LIMIT 1
			`, roomID).Scan(&nextOwner); err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, `
				UPDATE battle_rooms
				SET owner_id = $2,
				    updated_at = now()
				WHERE id = $1
			`, roomID, nextOwner); err != nil {
				return err
			}
			return s.ensureBattleRoomUnitsConsistentTx(ctx, tx, roomID, "leave_room")
		}
		if err := s.ensureBattleRoomUnitsConsistentTx(ctx, tx, roomID, "leave_room"); err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `UPDATE battle_rooms SET updated_at = now() WHERE id = $1`, roomID)
		return err
	}); err != nil {
		return BattleRoomDetail{}, err
	}
	if retainedForRewardJobs {
		// The caller has left successfully. Keep the now-hidden room only as a
		// durable parent for unfinished reward outboxes, then report the same
		// result as the existing last-member deletion path.
		return BattleRoomDetail{}, ErrBattleRoomNotFound
	}
	return s.GetBattleRoomDetail(ctx, roomID)
}

func (s *Store) KickBattleRoomMember(ctx context.Context, roomID, ownerUserID, targetUserID int64) (BattleRoomDetail, error) {
	if roomID <= 0 || ownerUserID <= 0 || targetUserID <= 0 || ownerUserID == targetUserID {
		return BattleRoomDetail{}, ErrBattleRoomConfigInvalid
	}
	if err := s.withTx(ctx, func(tx pgx.Tx) error {
		var ownerID int64
		var status string
		err := tx.QueryRow(ctx, `
			SELECT owner_id, status
			FROM battle_rooms
			WHERE id = $1
			FOR UPDATE
		`, roomID).Scan(&ownerID, &status)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrBattleRoomNotFound
		}
		if err != nil {
			return err
		}
		if ownerID != ownerUserID {
			return ErrBattleRoomNotOwner
		}
		if status != BattleRoomStatusIdle && status != BattleRoomStatusPreparing {
			return ErrBattleRoomStateInvalid
		}
		tag, err := tx.Exec(ctx, `
			DELETE FROM battle_room_members
			WHERE room_id = $1 AND user_id = $2
		`, roomID, targetUserID)
		if err != nil {
			return err
		}
		if tag.RowsAffected() <= 0 {
			return ErrBattleRoomMemberMissing
		}
		if err := s.removeBattleRoomUnitsForUserTx(ctx, tx, roomID, targetUserID); err != nil {
			return err
		}
		if err := s.ensureBattleRoomUnitsConsistentTx(ctx, tx, roomID, "kick_member"); err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `UPDATE battle_rooms SET updated_at = now() WHERE id = $1`, roomID)
		return err
	}); err != nil {
		return BattleRoomDetail{}, err
	}
	return s.GetBattleRoomDetail(ctx, roomID)
}

func (s *Store) UpdateBattleRoomConfig(ctx context.Context, roomID, userID int64, dungeonID, roomName string, autoReenterOnDefeat bool, disableEntryRound bool, isPublic bool, isFogMode bool, difficultyLevel int) (BattleRoomDetail, error) {
	dungeonID = strings.TrimSpace(dungeonID)
	roomName = strings.TrimSpace(roomName)
	if roomID <= 0 || userID <= 0 || dungeonID == "" || roomName == "" {
		return BattleRoomDetail{}, ErrBattleRoomConfigInvalid
	}
	maxTeamSize, hasWorld := s.battleWorldMaxTeamSizeByDungeonID(dungeonID)
	if !hasWorld || maxTeamSize <= 0 {
		return BattleRoomDetail{}, ErrBattleRoomConfigInvalid
	}
	if err := s.withTx(ctx, func(tx pgx.Tx) error {
		var ownerID int64
		var status string
		err := tx.QueryRow(ctx, `
			SELECT owner_id, status
			FROM battle_rooms
			WHERE id = $1
			FOR UPDATE
		`, roomID).Scan(&ownerID, &status)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrBattleRoomNotFound
		}
		if err != nil {
			return err
		}
		if ownerID != userID {
			return ErrBattleRoomNotOwner
		}
		if status != BattleRoomStatusIdle && status != BattleRoomStatusPreparing {
			return ErrBattleRoomStateInvalid
		}
		var memberCount int
		if err := tx.QueryRow(ctx, `
			SELECT COUNT(1)
			FROM battle_room_members
			WHERE room_id = $1
		`, roomID).Scan(&memberCount); err != nil {
			return err
		}
		if memberCount > maxTeamSize {
			return &BattleRoomTeamSizeExceededError{
				MemberCount: memberCount,
				MaxTeamSize: maxTeamSize,
			}
		}
		_, err = tx.Exec(ctx, `
				UPDATE battle_rooms
				SET dungeon_id = $2,
				    room_name = $3,
				    auto_reenter_on_defeat = $4,
				    disable_entry_round = $5,
				    is_public = $6,
				    is_fog_mode = $7,
				    difficulty_level = $8,
				    consecutive_failures = 0,
				    updated_at = now()
				WHERE id = $1
		`, roomID, dungeonID, roomName, autoReenterOnDefeat, disableEntryRound, isPublic, isFogMode, difficultyLevel)
		return err
	}); err != nil {
		return BattleRoomDetail{}, err
	}
	return s.GetBattleRoomDetail(ctx, roomID)
}

func (s *Store) TransferBattleRoomOwner(ctx context.Context, roomID, ownerUserID, targetUserID int64) (BattleRoomDetail, error) {
	if roomID <= 0 || ownerUserID <= 0 || targetUserID <= 0 || ownerUserID == targetUserID {
		return BattleRoomDetail{}, ErrBattleRoomConfigInvalid
	}
	if err := s.withTx(ctx, func(tx pgx.Tx) error {
		var ownerID int64
		var status string
		err := tx.QueryRow(ctx, `
			SELECT owner_id, status
			FROM battle_rooms
			WHERE id = $1
			FOR UPDATE
		`, roomID).Scan(&ownerID, &status)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrBattleRoomNotFound
		}
		if err != nil {
			return err
		}
		if ownerID != ownerUserID {
			return ErrBattleRoomNotOwner
		}
		if status == BattleRoomStatusPlaying {
			return ErrBattleRoomStateInvalid
		}
		var memberExists bool
		if err := tx.QueryRow(ctx, `
			SELECT EXISTS(
				SELECT 1
				FROM battle_room_members
				WHERE room_id = $1 AND user_id = $2
			)
		`, roomID, targetUserID).Scan(&memberExists); err != nil {
			return err
		}
		if !memberExists {
			return ErrBattleRoomTransferTargetInvalid
		}
		_, err = tx.Exec(ctx, `
			UPDATE battle_rooms
			SET owner_id = $2,
			    updated_at = now()
			WHERE id = $1
		`, roomID, targetUserID)
		return err
	}); err != nil {
		return BattleRoomDetail{}, err
	}
	return s.GetBattleRoomDetail(ctx, roomID)
}

func (s *Store) SetBattleRoomReady(ctx context.Context, roomID, userID int64, ready bool) (BattleRoomDetail, bool, error) {
	if roomID <= 0 || userID <= 0 {
		return BattleRoomDetail{}, false, ErrBattleRoomConfigInvalid
	}
	allReady := false
	err := s.withTx(ctx, func(tx pgx.Tx) error {
		var status string
		var schemaVersion int
		err := tx.QueryRow(ctx, `
			SELECT status, schema_version
			FROM battle_rooms
			WHERE id = $1
			FOR UPDATE
		`, roomID).Scan(&status, &schemaVersion)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrBattleRoomNotFound
		}
		if err != nil {
			return err
		}
		if status == BattleRoomStatusPlaying {
			return ErrBattleRoomStateInvalid
		}
		tag, err := tx.Exec(ctx, `
			UPDATE battle_room_members
			SET is_ready = $3,
			    ready_at = CASE WHEN $3 THEN now() ELSE NULL END,
			    updated_at = now()
			WHERE room_id = $1 AND user_id = $2
		`, roomID, userID, ready)
		if err != nil {
			return err
		}
		if tag.RowsAffected() <= 0 {
			return ErrBattleRoomMemberMissing
		}
		if _, err := tx.Exec(ctx, `
			UPDATE battle_room_member_units
			SET is_ready = $3,
			    updated_at = now()
			WHERE room_id = $1 AND owner_user_id = $2
		`, roomID, userID, ready); err != nil {
			return err
		}
		if err := s.ensureBattleRoomUnitsConsistentTx(ctx, tx, roomID, "set_ready"); err != nil {
			return err
		}
		var memberCount int
		var readyCount int
		if err := tx.QueryRow(ctx, `
			SELECT COUNT(1), SUM(CASE WHEN is_ready THEN 1 ELSE 0 END)
			FROM battle_room_members
			WHERE room_id = $1
		`, roomID).Scan(&memberCount, &readyCount); err != nil {
			return err
		}
		if memberCount > 0 && readyCount == memberCount {
			allReady = true
			enteringAt := time.Now().UTC()
			return s.resetBattleRoomJourneyTx(ctx, tx, roomID, battleRoomJourneyResetState{
				Status:            BattleRoomStatusEntering,
				EnteringStartedAt: &enteringAt,
				NextTickAt:        enteringAt,
			}, "", schemaVersion)
		}
		targetSchemaVersion := s.battleRoomTargetSchemaVersion(schemaVersion, BattleRoomStatusPreparing)
		_, err = tx.Exec(ctx, `
			UPDATE battle_rooms
			SET status = $2,
			    entering_started_at = NULL,
			    recovering_started_at = NULL,
			    recovery_ends_at = NULL,
			    owner_worker_id = '',
			    lease_until = NULL,
			    next_tick_at = now(),
			    phase = '',
			    schema_version = $3,
			    updated_at = now()
			WHERE id = $1
		`, roomID, BattleRoomStatusPreparing, targetSchemaVersion)
		return err
	})
	if err != nil {
		return BattleRoomDetail{}, false, err
	}
	detail, err := s.GetBattleRoomDetail(ctx, roomID)
	if err != nil {
		return BattleRoomDetail{}, false, err
	}
	return detail, allReady, nil
}

// StartBattleRoomFight transitions an entering room to playing and creates the first battle run.
// fogItemID: if non-empty, one unit is consumed from the owner's inventory before starting.
// If the fog item is insufficient, is_fog_mode is disabled and FogDisabled is returned true.
func (s *Store) StartBattleRoomFight(ctx context.Context, roomID int64, entryCosts map[string]int64, fogItemID string, now time.Time) (detail BattleRoomDetail, seed int64, fogDisabled bool, err error) {
	if roomID <= 0 {
		return BattleRoomDetail{}, 0, false, ErrBattleRoomConfigInvalid
	}
	err = s.withTx(ctx, func(tx pgx.Tx) error {
		var status string
		var schemaVersion int
		var ownerID int64
		txErr := tx.QueryRow(ctx, `
			SELECT status, schema_version, owner_id
			FROM battle_rooms
			WHERE id = $1
			FOR UPDATE
		`, roomID).Scan(&status, &schemaVersion, &ownerID)
		if errors.Is(txErr, pgx.ErrNoRows) {
			return ErrBattleRoomNotFound
		}
		if txErr != nil {
			return txErr
		}
		if status != BattleRoomStatusEntering {
			return ErrBattleRoomStateInvalid
		}
		// Fog mode item check (runs before any other logic so we can disable fog gracefully).
		if fogItemID != "" {
			qty, qErr := s.inventoryQuantityTx(ctx, tx, ownerID, fogItemID)
			if qErr != nil {
				return qErr
			}
			if qty < 1 {
				// Insufficient — disable fog mode and continue as normal.
				if dErr := s.DisableFogModeTx(ctx, tx, roomID); dErr != nil {
					return dErr
				}
				fogDisabled = true
			} else {
				if rErr := s.RemoveInventoryItemTx(ctx, tx, ownerID, fogItemID, 1); rErr != nil {
					return rErr
				}
			}
		}
		phase := ""
		if schemaVersion == 2 {
			phase = "compiling"
		}
		members, txErr := s.listBattleRoomMembersTx(ctx, tx, roomID, true)
		if txErr != nil {
			return txErr
		}
		if txErr = s.ensureBattleRoomUnitsConsistentTx(ctx, tx, roomID, "start_fight"); txErr != nil {
			return txErr
		}
		members, txErr = s.listBattleRoomMembersTx(ctx, tx, roomID, true)
		if txErr != nil {
			return txErr
		}
		if len(members) == 0 {
			return ErrBattleRoomStateInvalid
		}
		for _, member := range members {
			if !member.IsReady {
				return ErrBattleRoomStateInvalid
			}
		}
		for _, member := range members {
			for itemID, qty := range entryCosts {
				if qty <= 0 {
					continue
				}
				if txErr := s.LedgerConsumeTemplateTx(ctx, tx, member.UserID, itemID, qty, InventoryLedgerMeta{
					SourceSurface: "inventory",
					TargetSurface: "battle_room_entry",
					Details: map[string]any{
						"roomId": roomID,
					},
				}); txErr != nil {
					return txErr
				}
			}
		}
		seed = battleSeed(roomID, now)
		var runID int64
		memberIDs := make([]int64, 0, len(members))
		for _, member := range members {
			memberIDs = append(memberIDs, member.UserID)
		}
		passiveRaw, passiveAt, passiveErr := s.freezeBattlePassiveTx(ctx, tx, roomID, memberIDs...)
		if passiveErr != nil {
			return passiveErr
		}
		if txErr = tx.QueryRow(ctx, `
			INSERT INTO battle_runs(room_id, wave, status, compile_input_jsonb, timeline_jsonb, playback_state_jsonb, final_result_jsonb, result_summary_jsonb, started_at, updated_at)
			VALUES($1, 1, $2, '{}'::jsonb, '[]'::jsonb, $3::jsonb, '{}'::jsonb, '{}'::jsonb, $4, now())
			RETURNING id
		`, roomID, BattleRunStatusCompiled, passiveRaw, passiveAt).Scan(&runID); txErr != nil {
			return txErr
		}
		if _, txErr = tx.Exec(ctx, `
			UPDATE battle_rooms
			SET status = $2,
			    current_run_id = $3,
			    seed = $4,
			    entering_started_at = NULL,
			    recovering_started_at = NULL,
			    recovery_ends_at = NULL,
			    owner_worker_id = '',
			    lease_until = NULL,
			    next_tick_at = $5,
			    phase = $6,
			    updated_at = now()
			WHERE id = $1
		`, roomID, BattleRoomStatusPlaying, runID, seed, now, phase); txErr != nil {
			return txErr
		}
		_, txErr = tx.Exec(ctx, `
			UPDATE battle_room_members
			SET updated_at = now()
			WHERE room_id = $1
		`, roomID)
		return txErr
	})
	if err != nil {
		return BattleRoomDetail{}, 0, false, err
	}
	detail, err = s.GetBattleRoomDetail(ctx, roomID)
	if err != nil {
		return BattleRoomDetail{}, 0, false, err
	}
	return detail, seed, fogDisabled, nil
}

func (s *Store) StartNextBattleRoomRun(ctx context.Context, roomID int64, now time.Time) (BattleRoomDetail, int64, error) {
	if roomID <= 0 {
		return BattleRoomDetail{}, 0, ErrBattleRoomConfigInvalid
	}
	var seed int64
	err := s.withTx(ctx, func(tx pgx.Tx) error {
		var status string
		var currentRunID *int64
		var completedRuns int
		var schemaVersion int
		err := tx.QueryRow(ctx, `
			SELECT status, current_run_id, completed_runs, schema_version
			FROM battle_rooms
			WHERE id = $1
			FOR UPDATE
		`, roomID).Scan(&status, &currentRunID, &completedRuns, &schemaVersion)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrBattleRoomNotFound
		}
		if err != nil {
			return err
		}
		if status != BattleRoomStatusPlaying || currentRunID != nil {
			return ErrBattleRoomStateInvalid
		}
		phase := ""
		if schemaVersion == 2 {
			phase = "compiling"
		}
		members, err := s.listBattleRoomMembersTx(ctx, tx, roomID, true)
		if err != nil {
			return err
		}
		if len(members) == 0 {
			return ErrBattleRoomStateInvalid
		}
		seed = battleSeed(roomID, now)
		var runID int64
		wave := completedRuns + 1
		if wave <= 0 {
			wave = 1
		}
		memberIDs := make([]int64, 0, len(members))
		for _, member := range members {
			memberIDs = append(memberIDs, member.UserID)
		}
		passiveRaw, passiveAt, passiveErr := s.freezeBattlePassiveTx(ctx, tx, roomID, memberIDs...)
		if passiveErr != nil {
			return passiveErr
		}
		if err := tx.QueryRow(ctx, `
			INSERT INTO battle_runs(room_id, wave, status, compile_input_jsonb, timeline_jsonb, playback_state_jsonb, final_result_jsonb, result_summary_jsonb, started_at, updated_at)
			VALUES($1, $2, $3, '{}'::jsonb, '[]'::jsonb, $4::jsonb, '{}'::jsonb, '{}'::jsonb, $5, now())
			RETURNING id
		`, roomID, wave, BattleRunStatusCompiled, passiveRaw, passiveAt).Scan(&runID); err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `
			UPDATE battle_rooms
			SET current_run_id = $2,
			    seed = $3,
			    owner_worker_id = '',
			    lease_until = NULL,
			    next_tick_at = $4,
			    phase = $5,
			    updated_at = now()
			WHERE id = $1
		`, roomID, runID, seed, now, phase)
		return err
	})
	if err != nil {
		return BattleRoomDetail{}, 0, err
	}
	detail, err := s.getBattleRoomDetailBatched(ctx, roomID)
	if err != nil {
		return BattleRoomDetail{}, 0, err
	}
	return detail, seed, nil
}

func (s *Store) enqueueBattleAchievementEventBatchesTx(ctx context.Context, tx pgx.Tx, batches []AchievementEventBatch) error {
	for _, batch := range batches {
		if err := s.EnqueueAchievementEventsTx(ctx, tx, batch.UserID, batch.Events); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) CompleteBattleRunAndMaybeContinue(
	ctx context.Context,
	roomID, expectedRunID int64,
	rewards map[int64]BattleWaveReward,
	itemBagUpdates []BattleItemBagQuantityUpdate,
	attrExpJobPayload []byte,
	skillExpJobPayload []byte,
	killAchievementBatches []AchievementEventBatch,
	now time.Time,
	interBattleDelay time.Duration,
) (BattleRoomDetail, bool, error) {
	if roomID <= 0 || expectedRunID <= 0 {
		return BattleRoomDetail{}, false, ErrBattleRoomConfigInvalid
	}
	normalizedItemBagUpdates, err := normalizeBattleItemBagQuantityUpdates(itemBagUpdates)
	if err != nil {
		return BattleRoomDetail{}, false, err
	}
	if interBattleDelay < 0 {
		interBattleDelay = 0
	}
	shouldContinue := false
	metrics := battleFinalizeStoreMetrics{}
	var finalizedRunID int64
	txStartedAt := time.Now()
	if err := s.withTx(ctx, func(tx pgx.Tx) error {
		var currentRunID *int64
		var completedRuns int
		var status string
		var schemaVersion int
		stageStartedAt := time.Now()
		err := tx.QueryRow(ctx, `
			SELECT current_run_id, completed_runs, status, schema_version
			FROM battle_rooms
			WHERE id = $1
			FOR UPDATE
		`, roomID).Scan(&currentRunID, &completedRuns, &status, &schemaVersion)
		metrics.roomLockElapsed += time.Since(stageStartedAt)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrBattleRoomNotFound
		}
		if err != nil {
			return err
		}
		if status != BattleRoomStatusPlaying {
			return ErrBattleRoomStateInvalid
		}
		if currentRunID == nil || *currentRunID != expectedRunID {
			return ErrBattleSettlementRunDetached
		}
		finalizedRunID = expectedRunID
		stageStartedAt = time.Now()
		tag, err := tx.Exec(ctx, `
				UPDATE battle_runs
				SET status = $2,
				    ended_at = $3,
				    updated_at = now()
				WHERE id = $1
				  AND room_id = $4
				  AND status IN ($5, $6)
				`, expectedRunID, BattleRunStatusEnded, now, roomID, BattleRunStatusSettling, BattleRunStatusRunning)
		if err != nil {
			return err
		}
		if tag.RowsAffected() != 1 {
			return ErrBattleSettlementRunDetached
		}
		metrics.runUpdateElapsed += time.Since(stageStartedAt)
		if err := s.setBattleItemBagQuantitiesTx(ctx, tx, normalizedItemBagUpdates); err != nil {
			return err
		}
		if err := s.applyBattleRoomRewardsTx(ctx, tx, roomID, finalizedRunID, rewards, "finish_reward", false, &metrics); err != nil {
			return err
		}
		if err := s.enqueueBattleAttrExpJobTx(ctx, tx, finalizedRunID, attrExpJobPayload, now); err != nil {
			return err
		}
		if err := s.enqueueBattleSkillExpJobTx(ctx, tx, finalizedRunID, skillExpJobPayload, now); err != nil {
			return err
		}
		if err := s.enqueueBattleAchievementEventBatchesTx(ctx, tx, killAchievementBatches); err != nil {
			return err
		}
		nextCompleted := completedRuns + 1
		shouldContinue = true
		if shouldContinue {
			nextTick := now.Add(interBattleDelay)
			phase := ""
			if schemaVersion == 2 {
				phase = "awaiting_run"
			}
			stageStartedAt = time.Now()
			_, err = tx.Exec(ctx, `
				UPDATE battle_rooms
					SET status = $2,
					    current_run_id = NULL,
					    completed_runs = $3,
					    consecutive_failures = 0,
					    entering_started_at = NULL,
				    recovering_started_at = NULL,
				    recovery_ends_at = NULL,
				    owner_worker_id = '',
				    lease_until = NULL,
				    next_tick_at = $4,
				    phase = $5,
				    updated_at = now()
				WHERE id = $1
			`, roomID, BattleRoomStatusPlaying, nextCompleted, nextTick, phase)
			metrics.roomUpdateElapsed += time.Since(stageStartedAt)
			return err
		}
		stageStartedAt = time.Now()
		if _, err := tx.Exec(ctx, `
			UPDATE battle_room_members
			SET is_ready = FALSE,
			    ready_at = NULL,
			    updated_at = now()
			WHERE room_id = $1
		`, roomID); err != nil {
			return err
		}
		metrics.memberUpdateElapsed += time.Since(stageStartedAt)
		stageStartedAt = time.Now()
		targetSchemaVersion := s.battleRoomTargetSchemaVersion(schemaVersion, BattleRoomStatusIdle)
		_, err = tx.Exec(ctx, `
			UPDATE battle_rooms
				SET status = $2,
				    current_run_id = NULL,
				    completed_runs = $3,
				    consecutive_failures = 0,
				    entering_started_at = NULL,
			    recovering_started_at = NULL,
			    recovery_ends_at = NULL,
			    owner_worker_id = '',
			    lease_until = NULL,
			    next_tick_at = now(),
			    phase = '',
			    schema_version = $4,
			    updated_at = now()
			WHERE id = $1
		`, roomID, BattleRoomStatusIdle, nextCompleted, targetSchemaVersion)
		if err != nil {
			return err
		}
		metrics.roomUpdateElapsed += time.Since(stageStartedAt)
		// Clear fog member snapshots when room returns to idle.
		return s.DeleteFogMemberSnapshotsByRoomTx(ctx, tx, roomID)
	}); err != nil {
		return BattleRoomDetail{}, false, err
	}
	metrics.txElapsed = time.Since(txStartedAt)
	detailStartedAt := time.Now()
	detail, err := s.GetBattleRoomDetail(ctx, roomID)
	if err != nil {
		return BattleRoomDetail{}, false, err
	}
	metrics.detailLoadElapsed = time.Since(detailStartedAt)
	logBattleFinalizeStoreStages("continue", roomID, finalizedRunID, metrics)
	return detail, shouldContinue, nil
}

func (s *Store) HandleBattleRunFailure(
	ctx context.Context,
	roomID, expectedRunID int64,
	rewards map[int64]BattleWaveReward,
	itemBagUpdates []BattleItemBagQuantityUpdate,
	attrExpJobPayload []byte,
	skillExpJobPayload []byte,
	killAchievementBatches []AchievementEventBatch,
	now time.Time,
	recoveryDuration time.Duration,
) (BattleRoomDetail, error) {
	if roomID <= 0 || expectedRunID <= 0 {
		return BattleRoomDetail{}, ErrBattleRoomConfigInvalid
	}
	normalizedItemBagUpdates, err := normalizeBattleItemBagQuantityUpdates(itemBagUpdates)
	if err != nil {
		return BattleRoomDetail{}, err
	}
	if recoveryDuration < 0 {
		recoveryDuration = 0
	}
	metrics := battleFinalizeStoreMetrics{}
	var finalizedRunID int64
	txStartedAt := time.Now()
	if err := s.withTx(ctx, func(tx pgx.Tx) error {
		var currentRunID *int64
		var status string
		var schemaVersion int
		var consecutiveFailures int
		stageStartedAt := time.Now()
		err := tx.QueryRow(ctx, `
			SELECT current_run_id, status, schema_version, consecutive_failures
			FROM battle_rooms
			WHERE id = $1
			FOR UPDATE
		`, roomID).Scan(&currentRunID, &status, &schemaVersion, &consecutiveFailures)
		metrics.roomLockElapsed += time.Since(stageStartedAt)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrBattleRoomNotFound
		}
		if err != nil {
			return err
		}
		if status != BattleRoomStatusPlaying {
			return ErrBattleRoomStateInvalid
		}
		if currentRunID == nil || *currentRunID != expectedRunID {
			return ErrBattleSettlementRunDetached
		}
		finalizedRunID = expectedRunID
		var failedRunWave int
		stageStartedAt = time.Now()
		if err := tx.QueryRow(ctx, `
				UPDATE battle_runs
				SET status = $2,
				    ended_at = $3,
					updated_at = now()
				WHERE id = $1
				  AND room_id = $4
				  AND status IN ($5, $6)
				RETURNING wave
			`, expectedRunID, BattleRunStatusFailed, now, roomID, BattleRunStatusSettling, BattleRunStatusRunning).Scan(&failedRunWave); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrBattleSettlementRunDetached
			}
			return err
		}
		metrics.runUpdateElapsed += time.Since(stageStartedAt)
		if err := s.setBattleItemBagQuantitiesTx(ctx, tx, normalizedItemBagUpdates); err != nil {
			return err
		}
		if err := s.applyBattleRoomRewardsTx(ctx, tx, roomID, finalizedRunID, rewards, "fail_reward", false, &metrics); err != nil {
			return err
		}
		if err := s.enqueueBattleAttrExpJobTx(ctx, tx, finalizedRunID, attrExpJobPayload, now); err != nil {
			return err
		}
		if err := s.enqueueBattleSkillExpJobTx(ctx, tx, finalizedRunID, skillExpJobPayload, now); err != nil {
			return err
		}
		if err := s.enqueueBattleAchievementEventBatchesTx(ctx, tx, killAchievementBatches); err != nil {
			return err
		}
		nextConsecutiveFailures := battleNextConsecutiveFailures(consecutiveFailures, failedRunWave)
		recoveryEndsAt := now.Add(battleFailureRecoveryDuration(recoveryDuration, nextConsecutiveFailures))
		recoveringAt := now
		stageStartedAt = time.Now()
		defer func() {
			metrics.roomUpdateElapsed += time.Since(stageStartedAt)
		}()
		if _, err := tx.Exec(ctx, `
			UPDATE battle_rooms
			SET consecutive_failures = $2
			WHERE id = $1
		`, roomID, nextConsecutiveFailures); err != nil {
			return err
		}
		targetSchemaVersion := s.battleRoomTargetSchemaVersion(schemaVersion, BattleRoomStatusRecovering)
		return s.resetBattleRoomJourneyTx(ctx, tx, roomID, battleRoomJourneyResetState{
			Status:              BattleRoomStatusRecovering,
			RecoveringStartedAt: &recoveringAt,
			RecoveryEndsAt:      &recoveryEndsAt,
			NextTickAt:          recoveryEndsAt,
		}, "", targetSchemaVersion)
	}); err != nil {
		return BattleRoomDetail{}, err
	}
	metrics.txElapsed = time.Since(txStartedAt)
	detailStartedAt := time.Now()
	detail, err := s.GetBattleRoomDetail(ctx, roomID)
	if err != nil {
		return BattleRoomDetail{}, err
	}
	metrics.detailLoadElapsed = time.Since(detailStartedAt)
	logBattleFinalizeStoreStages("failure", roomID, finalizedRunID, metrics)
	return detail, nil
}

func (s *Store) AdvanceBattleRoomRecovery(ctx context.Context, roomID int64, now time.Time) (BattleRoomDetail, bool, error) {
	if roomID <= 0 {
		return BattleRoomDetail{}, false, ErrBattleRoomConfigInvalid
	}
	transitioned := false
	retryAt := now
	if err := s.withTx(ctx, func(tx pgx.Tx) error {
		var status string
		var currentRunID *int64
		var autoReenterOnDefeat bool
		var recoveryEndsAt *time.Time
		var schemaVersion int
		err := tx.QueryRow(ctx, `
			SELECT status, current_run_id, auto_reenter_on_defeat, recovery_ends_at, schema_version
			FROM battle_rooms
			WHERE id = $1
			FOR UPDATE
		`, roomID).Scan(&status, &currentRunID, &autoReenterOnDefeat, &recoveryEndsAt, &schemaVersion)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrBattleRoomNotFound
		}
		if err != nil {
			return err
		}
		if status != BattleRoomStatusRecovering {
			return ErrBattleRoomStateInvalid
		}
		if recoveryEndsAt != nil && !recoveryEndsAt.IsZero() && now.Before(*recoveryEndsAt) {
			retryAt = *recoveryEndsAt
			if _, err := tx.Exec(ctx, `
				UPDATE battle_rooms
				SET next_tick_at = $2,
				    updated_at = now()
				WHERE id = $1
			`, roomID, retryAt); err != nil {
				return err
			}
			return nil
		}
		transitioned = true
		if autoReenterOnDefeat {
			if _, err := tx.Exec(ctx, `
				UPDATE battle_room_members
				SET is_ready = TRUE,
				    ready_at = now(),
				    updated_at = now()
				WHERE room_id = $1
			`, roomID); err != nil {
				return err
			}
			enteringAt := now
			return s.resetBattleRoomJourneyTx(ctx, tx, roomID, battleRoomJourneyResetState{
				Status:            BattleRoomStatusEntering,
				EnteringStartedAt: &enteringAt,
				NextTickAt:        enteringAt,
			}, "", schemaVersion)
		}
		if _, err := tx.Exec(ctx, `
			UPDATE battle_room_members
			SET is_ready = FALSE,
			    ready_at = NULL,
			    updated_at = now()
			WHERE room_id = $1
		`, roomID); err != nil {
			return err
		}
		targetSchemaVersion := s.battleRoomTargetSchemaVersion(schemaVersion, BattleRoomStatusIdle)
		return s.resetBattleRoomJourneyTx(ctx, tx, roomID, battleRoomJourneyResetState{
			Status:     BattleRoomStatusIdle,
			NextTickAt: now,
		}, "", targetSchemaVersion)
	}); err != nil {
		return BattleRoomDetail{}, false, err
	}
	detail, err := s.GetBattleRoomDetail(ctx, roomID)
	if err != nil {
		return BattleRoomDetail{}, false, err
	}
	if !transitioned && !retryAt.IsZero() {
		detail.Room.RecoveryEndsAt = &retryAt
	}
	return detail, transitioned, nil
}

func (s *Store) PauseBattleRoom(ctx context.Context, roomID int64) error {
	_ = ctx
	_ = roomID
	return nil
}

func (s *Store) ResumeBattleRoom(ctx context.Context, roomID int64) error {
	_ = ctx
	_ = roomID
	return nil
}

func (s *Store) FinishBattleRoom(
	ctx context.Context,
	roomID, expectedRunID int64,
	runStatus string,
	rewards map[int64]BattleWaveReward,
	itemBagUpdates []BattleItemBagQuantityUpdate,
	attrExpJobPayload []byte,
	skillExpJobPayload []byte,
	killAchievementBatches []AchievementEventBatch,
	now time.Time,
) (BattleRoomDetail, error) {
	if roomID <= 0 || expectedRunID <= 0 {
		return BattleRoomDetail{}, ErrBattleRoomConfigInvalid
	}
	normalizedItemBagUpdates, err := normalizeBattleItemBagQuantityUpdates(itemBagUpdates)
	if err != nil {
		return BattleRoomDetail{}, err
	}
	metrics := battleFinalizeStoreMetrics{}
	var finalizedRunID int64
	txStartedAt := time.Now()
	if err := s.withTx(ctx, func(tx pgx.Tx) error {
		var currentRunID *int64
		var schemaVersion int
		var consecutiveFailures int
		currentRunWave := 0
		stageStartedAt := time.Now()
		err := tx.QueryRow(ctx, `
			SELECT current_run_id, schema_version, consecutive_failures
			FROM battle_rooms
			WHERE id = $1
			FOR UPDATE
		`, roomID).Scan(&currentRunID, &schemaVersion, &consecutiveFailures)
		metrics.roomLockElapsed += time.Since(stageStartedAt)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrBattleRoomNotFound
		}
		if err != nil {
			return err
		}
		if currentRunID == nil || *currentRunID != expectedRunID {
			return ErrBattleSettlementRunDetached
		}
		finalizedRunID = expectedRunID
		stageStartedAt = time.Now()
		if err := tx.QueryRow(ctx, `
					UPDATE battle_runs
				SET status = $2,
				    ended_at = $3,
				    updated_at = now()
				WHERE id = $1
				  AND room_id = $4
				  AND status IN ($5, $6)
				RETURNING wave
			`, expectedRunID, runStatus, now, roomID, BattleRunStatusSettling, BattleRunStatusRunning).Scan(&currentRunWave); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrBattleSettlementRunDetached
			}
			return err
		}
		metrics.runUpdateElapsed += time.Since(stageStartedAt)
		if err := s.setBattleItemBagQuantitiesTx(ctx, tx, normalizedItemBagUpdates); err != nil {
			return err
		}
		if err := s.applyBattleRoomRewardsTx(ctx, tx, roomID, finalizedRunID, rewards, "interrupt_reward", false, &metrics); err != nil {
			return err
		}
		if err := s.enqueueBattleAttrExpJobTx(ctx, tx, finalizedRunID, attrExpJobPayload, now); err != nil {
			return err
		}
		if err := s.enqueueBattleSkillExpJobTx(ctx, tx, finalizedRunID, skillExpJobPayload, now); err != nil {
			return err
		}
		if err := s.enqueueBattleAchievementEventBatchesTx(ctx, tx, killAchievementBatches); err != nil {
			return err
		}
		stageStartedAt = time.Now()
		if _, err := tx.Exec(ctx, `
			UPDATE battle_room_members
			SET is_ready = FALSE,
			    ready_at = NULL,
			    updated_at = now()
			WHERE room_id = $1
		`, roomID); err != nil {
			return err
		}
		metrics.memberUpdateElapsed += time.Since(stageStartedAt)
		stageStartedAt = time.Now()
		targetSchemaVersion := s.battleRoomTargetSchemaVersion(schemaVersion, BattleRoomStatusIdle)
		nextConsecutiveFailures := 0
		if runStatus == BattleRunStatusFailed {
			nextConsecutiveFailures = battleNextConsecutiveFailures(consecutiveFailures, currentRunWave)
		}
		if _, err := tx.Exec(ctx, `
			UPDATE battle_rooms
				SET status = $2,
				    current_run_id = NULL,
				    completed_runs = completed_runs + CASE WHEN $3 = 'ended' THEN 1 ELSE 0 END,
				    consecutive_failures = $5,
				    entering_started_at = NULL,
				    recovering_started_at = NULL,
				    recovery_ends_at = NULL,
				    owner_worker_id = '',
				    lease_until = NULL,
				    next_tick_at = now(),
				    phase = '',
				    schema_version = $4,
				    updated_at = now()
			WHERE id = $1
		`, roomID, BattleRoomStatusIdle, runStatus, targetSchemaVersion, nextConsecutiveFailures); err != nil {
			return err
		}
		metrics.roomUpdateElapsed += time.Since(stageStartedAt)
		return nil
	}); err != nil {
		return BattleRoomDetail{}, err
	}
	metrics.txElapsed = time.Since(txStartedAt)
	detailStartedAt := time.Now()
	detail, err := s.GetBattleRoomDetail(ctx, roomID)
	if err != nil {
		return BattleRoomDetail{}, err
	}
	metrics.detailLoadElapsed = time.Since(detailStartedAt)
	logBattleFinalizeStoreStages("finish", roomID, finalizedRunID, metrics)
	return detail, nil
}

func (s *Store) InterruptBattleRoom(ctx context.Context, roomID, userID int64, now time.Time) (BattleRoomDetail, error) {
	if roomID <= 0 || userID <= 0 {
		return BattleRoomDetail{}, ErrBattleRoomConfigInvalid
	}
	if err := s.withTx(ctx, func(tx pgx.Tx) error {
		var currentRunID *int64
		var status string
		var schemaVersion int
		err := tx.QueryRow(ctx, `
			SELECT current_run_id, status, schema_version
			FROM battle_rooms
			WHERE id = $1
			FOR UPDATE
		`, roomID).Scan(&currentRunID, &status, &schemaVersion)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrBattleRoomNotFound
		}
		if err != nil {
			return err
		}
		if !s.isBattleRoomMemberTx(ctx, tx, roomID, userID) {
			return ErrBattleRoomMemberMissing
		}
		if currentRunID != nil {
			if _, err := tx.Exec(ctx, `
				UPDATE battle_runs
				SET status = $2,
				    ended_at = $3,
				    updated_at = now()
				WHERE id = $1
			`, *currentRunID, BattleRunStatusInterrupted, now); err != nil {
				return err
			}
		}
		if _, err := tx.Exec(ctx, `
			UPDATE battle_room_members
			SET is_ready = CASE WHEN user_id = $2 THEN FALSE ELSE is_ready END,
			    ready_at = CASE WHEN user_id = $2 THEN NULL ELSE ready_at END,
			    updated_at = now()
			WHERE room_id = $1
		`, roomID, userID); err != nil {
			return err
		}
		nextStatus := BattleRoomStatusPreparing
		if status == BattleRoomStatusIdle {
			nextStatus = BattleRoomStatusIdle
		}
		targetSchemaVersion := s.battleRoomTargetSchemaVersion(schemaVersion, nextStatus)
		if _, err := tx.Exec(ctx, `
			UPDATE battle_rooms
			SET status = $2,
			    current_run_id = NULL,
			    entering_started_at = NULL,
			    recovering_started_at = NULL,
			    recovery_ends_at = NULL,
			    owner_worker_id = '',
			    lease_until = NULL,
			    next_tick_at = now(),
			    phase = '',
			    schema_version = $3,
			    updated_at = now()
			WHERE id = $1
		`, roomID, nextStatus, targetSchemaVersion); err != nil {
			return err
		}
		return nil
	}); err != nil {
		return BattleRoomDetail{}, err
	}
	return s.GetBattleRoomDetail(ctx, roomID)
}

func (s *Store) ListUserActiveBattleRooms(ctx context.Context, userID int64) ([]BattleRoom, error) {
	rows, err := s.db.Query(ctx, `
		SELECT r.id, r.owner_id, r.dungeon_id, r.room_name, r.auto_reenter_on_defeat, r.disable_entry_round, r.is_public, r.is_fog_mode, r.status, r.difficulty_level, r.completed_runs, r.consecutive_failures, r.current_run_id, r.seed, r.entering_started_at, r.recovering_started_at, r.recovery_ends_at, r.created_at, r.updated_at, r.phase, r.schema_version
		FROM battle_rooms r
		JOIN battle_room_members m ON m.room_id = r.id
		WHERE m.user_id = $1 AND r.status IN ('playing')
		ORDER BY r.updated_at DESC, r.id DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]BattleRoom, 0, 4)
	for rows.Next() {
		room, err := scanBattleRoomRow(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, room)
	}
	return out, rows.Err()
}

func (s *Store) GetBattleRoomByUser(ctx context.Context, userID int64) (BattleRoomDetail, error) {
	if userID <= 0 {
		return BattleRoomDetail{}, ErrBattleRoomConfigInvalid
	}
	var roomID int64
	err := s.db.QueryRow(ctx, `
		SELECT room_id
		FROM battle_room_members
		WHERE user_id = $1
		LIMIT 1
	`, userID).Scan(&roomID)
	if errors.Is(err, pgx.ErrNoRows) {
		return BattleRoomDetail{}, ErrBattleRoomNotFound
	}
	if err != nil {
		return BattleRoomDetail{}, err
	}
	return s.GetBattleRoomDetail(ctx, roomID)
}

func (s *Store) SaveBattleRunState(ctx context.Context, runID int64, wave int, status string, state map[string]any, endedAt *time.Time) error {
	if runID <= 0 {
		return ErrBattleRoomConfigInvalid
	}
	raw, err := json.Marshal(state)
	if err != nil {
		return err
	}
	if err := validatePostgresJSONBRaw("battle run playback state", raw); err != nil {
		return err
	}
	if endedAt != nil {
		_, err = s.db.Exec(ctx, `
		UPDATE battle_runs
		SET wave = $2,
		    status = $3,
		    playback_state_jsonb = $4::jsonb,
		    ended_at = $5,
		    updated_at = now()
		WHERE id = $1
	`, runID, wave, status, string(raw), *endedAt)
		return err
	}
	_, err = s.db.Exec(ctx, `
		UPDATE battle_runs
		SET wave = $2,
		    status = $3,
		    playback_state_jsonb = $4::jsonb,
		    updated_at = now()
		WHERE id = $1
	`, runID, wave, status, string(raw))
	return err
}

func (s *Store) SaveBattleRunCompiled(
	ctx context.Context,
	runID int64,
	wave int,
	status string,
	compileInput map[string]any,
	state map[string]any,
	timeline []map[string]any,
	resultSummary map[string]any,
	carryState map[string]any,
	finalResult map[string]any,
) error {
	_, err := s.SaveBattleRunCompiledIfStatus(ctx, runID, wave, status, compileInput, state, timeline, resultSummary, carryState, finalResult, BattleRunStatusCompiled)
	return err
}

func marshalBattleRunFinalResult(finalResult map[string]any, carryState map[string]any, carryStateRaw []byte) ([]byte, error) {
	finalCarry, ok := finalResult["carryState"]
	if !ok || len(carryStateRaw) == 0 || !sameBattleCarryStateMap(finalCarry, carryState) {
		return json.Marshal(finalResult)
	}
	withRawCarry := make(map[string]any, len(finalResult))
	for key, value := range finalResult {
		withRawCarry[key] = value
	}
	// The carry state is persisted separately immediately before the cold
	// payload. Reuse its validated JSON here instead of recursively encoding
	// the same large unit graph a second time.
	withRawCarry["carryState"] = json.RawMessage(carryStateRaw)
	return json.Marshal(withRawCarry)
}

func sameBattleCarryStateMap(finalCarry any, carryState map[string]any) bool {
	typed, ok := finalCarry.(map[string]any)
	if !ok {
		return false
	}
	if typed == nil || carryState == nil {
		return typed == nil && carryState == nil
	}
	return reflect.ValueOf(typed).Pointer() == reflect.ValueOf(carryState).Pointer()
}

func (s *Store) SaveBattleRunCompiledIfStatus(
	ctx context.Context,
	runID int64,
	wave int,
	status string,
	compileInput map[string]any,
	state map[string]any,
	timeline []map[string]any,
	resultSummary map[string]any,
	carryState map[string]any,
	finalResult map[string]any,
	fromStatus string,
) (bool, error) {
	return s.saveBattleRunCompiledIfStatus(
		ctx,
		runID,
		wave,
		status,
		compileInput,
		state,
		timeline,
		nil,
		resultSummary,
		carryState,
		finalResult,
		fromStatus,
	)
}

func (s *Store) SaveBattleRunCompiledWithTimelineJSONIfStatus(
	ctx context.Context,
	runID int64,
	wave int,
	status string,
	compileInput map[string]any,
	state map[string]any,
	timelineJSON BattleRunTimelineJSON,
	resultSummary map[string]any,
	carryState map[string]any,
	finalResult map[string]any,
	fromStatus string,
) (bool, error) {
	timelineRaw, err := timelineJSON.bytesForSave()
	if err != nil {
		return false, err
	}
	return s.saveBattleRunCompiledIfStatus(
		ctx,
		runID,
		wave,
		status,
		compileInput,
		state,
		nil,
		timelineRaw,
		resultSummary,
		carryState,
		finalResult,
		fromStatus,
	)
}

func (s *Store) saveBattleRunCompiledIfStatus(
	ctx context.Context,
	runID int64,
	wave int,
	status string,
	compileInput map[string]any,
	state map[string]any,
	timeline []map[string]any,
	timelineRaw []byte,
	resultSummary map[string]any,
	carryState map[string]any,
	finalResult map[string]any,
	fromStatus string,
) (bool, error) {
	if runID <= 0 {
		return false, ErrBattleRoomConfigInvalid
	}
	totalStartedAt := time.Now()

	// Marshal all JSON BEFORE the transaction to minimize lock-hold time.
	marshalStartedAt := time.Now()
	stateRaw, err := json.Marshal(state)
	if err != nil {
		return false, err
	}
	if err := validatePostgresJSONBRaw("battle run playback state", stateRaw); err != nil {
		return false, err
	}
	stateMarshalElapsed := time.Since(marshalStartedAt)

	marshalStartedAt = time.Now()
	summaryRaw, err := json.Marshal(resultSummary)
	if err != nil {
		return false, err
	}
	if err := validatePostgresJSONBRaw("battle run result summary", summaryRaw); err != nil {
		return false, err
	}
	summaryMarshalElapsed := time.Since(marshalStartedAt)

	marshalStartedAt = time.Now()
	carryStateRaw, err := json.Marshal(carryState)
	if err != nil {
		return false, err
	}
	if err := validatePostgresJSONBRaw("battle run carry state", carryStateRaw); err != nil {
		return false, err
	}
	carryStateMarshalElapsed := time.Since(marshalStartedAt)

	marshalStartedAt = time.Now()
	compileInputRaw, err := json.Marshal(compileInput)
	if err != nil {
		return false, err
	}
	if timelineRaw == nil {
		timelineRaw, err = json.Marshal(timeline)
		if err != nil {
			return false, err
		}
	}
	finalResultRaw, err := marshalBattleRunFinalResult(finalResult, carryState, carryStateRaw)
	if err != nil {
		return false, err
	}
	coldPayloadMarshalElapsed := time.Since(marshalStartedAt)

	if strings.TrimSpace(fromStatus) == "" {
		fromStatus = BattleRunStatusCompiled
	}

	// Transaction only does DB writes with pre-marshaled bytes. Lock the room
	// before the run to match every finalize/interrupt path and avoid a
	// room<->run deadlock. The room guard also fences stale compilers before any
	// carry or cold payload can be overwritten.
	var tag pgconn.CommandTag
	execStartedAt := time.Now()
	err = s.withTx(ctx, func(tx pgx.Tx) error {
		var (
			roomID        int64
			schemaVersion int
		)
		lockErr := tx.QueryRow(ctx, `
			SELECT room.id, room.schema_version
			FROM battle_rooms AS room
			JOIN battle_runs AS br ON br.room_id = room.id
			WHERE br.id = $1
			  AND room.current_run_id = br.id
			  AND (
			    room.schema_version <> 2
			    OR (room.status = 'playing' AND room.phase = 'compiling')
			  )
			FOR UPDATE OF room
		`, runID).Scan(&roomID, &schemaVersion)
		if errors.Is(lockErr, pgx.ErrNoRows) {
			return nil
		}
		if lockErr != nil {
			return lockErr
		}

		var execErr error
		tag, execErr = tx.Exec(ctx, `
			UPDATE battle_runs
			SET wave = $2,
			    status = $3,
			    playback_state_jsonb = $4::jsonb,
			    result_summary_jsonb = $5::jsonb,
			    updated_at = now()
			WHERE id = $1
			  AND status = $6
		`, runID, wave, status, string(stateRaw), string(summaryRaw), fromStatus)
		if execErr != nil || tag.RowsAffected() == 0 {
			return execErr
		}
		if err := s.upsertBattleRunCarryStateRaw(ctx, tx, runID, carryStateRaw); err != nil {
			return err
		}
		if err := s.upsertBattleRunColdPayloadRaw(ctx, tx, runID, compileInputRaw, timelineRaw, finalResultRaw); err != nil {
			return err
		}
		if schemaVersion != 2 {
			return nil
		}
		phaseTag, execErr := tx.Exec(ctx, `
			UPDATE battle_rooms
			SET phase = 'playback',
			    updated_at = now()
			WHERE id = $2
			  AND current_run_id = $1
			  AND schema_version = 2
			  AND status = 'playing'
			  AND phase = 'compiling'
		`, runID, roomID)
		if execErr != nil {
			return execErr
		}
		if phaseTag.RowsAffected() != 1 {
			return ErrBattleRoomStateInvalid
		}
		return nil
	})
	dbExecElapsed := time.Since(execStartedAt)
	totalElapsed := time.Since(totalStartedAt)
	logBattleRunCompiledSaveStages(
		runID,
		wave,
		status,
		coldPayloadMarshalElapsed, // compileInputMarshalElapsed (compileInput + timeline + finalResult combined)
		stateMarshalElapsed+carryStateMarshalElapsed, // stateMarshalElapsed (state + carryState combined)
		0,                     // timelineMarshalElapsed (already in coldPayloadMarshalElapsed)
		summaryMarshalElapsed, // finalResultMarshalElapsed (actually resultSummary)
		len(compileInputRaw),
		len(stateRaw),
		len(timelineRaw),
		len(finalResultRaw),
		dbExecElapsed,
		totalElapsed,
	)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}

func (s *Store) CompactBattleRunHistory(ctx context.Context, roomID int64, keepCount int) error {
	if roomID <= 0 {
		return ErrBattleRoomConfigInvalid
	}
	for {
		deleted, err := s.CompactBattleRunHistories(ctx, []int64{roomID}, keepCount, battleRunHistoryCompactMaxDeleteBatch)
		if err != nil {
			return err
		}
		if deleted < battleRunHistoryCompactMaxDeleteBatch {
			return nil
		}
	}
}

func normalizeBattleRunHistoryRoomIDs(roomIDs []int64) ([]int64, error) {
	if len(roomIDs) == 0 {
		return nil, nil
	}
	if len(roomIDs) > battleRunHistoryCompactMaxRooms {
		return nil, ErrBattleRoomConfigInvalid
	}
	seen := make(map[int64]struct{}, len(roomIDs))
	normalized := make([]int64, 0, len(roomIDs))
	for _, roomID := range roomIDs {
		if roomID <= 0 {
			return nil, ErrBattleRoomConfigInvalid
		}
		if _, ok := seen[roomID]; ok {
			continue
		}
		seen[roomID] = struct{}{}
		normalized = append(normalized, roomID)
	}
	sort.Slice(normalized, func(i, j int) bool { return normalized[i] < normalized[j] })
	return normalized, nil
}

// CompactBattleRunHistories applies the dungeon-room retention policy to a
// bounded set of rooms in one statement. It never touches arena or combat
// trial history, which use separate tables and retention contracts.
func (s *Store) CompactBattleRunHistories(ctx context.Context, roomIDs []int64, keepCount, batchSize int) (int64, error) {
	roomIDs, err := normalizeBattleRunHistoryRoomIDs(roomIDs)
	if err != nil {
		return 0, err
	}
	if len(roomIDs) == 0 {
		return 0, nil
	}
	if keepCount <= 0 {
		keepCount = 10
	}
	if batchSize <= 0 {
		batchSize = 64
	}
	if batchSize > battleRunHistoryCompactMaxDeleteBatch {
		return 0, ErrBattleRoomConfigInvalid
	}
	tag, err := s.db.Exec(ctx, `
		WITH requested_rooms AS (
			SELECT DISTINCT room_id
			FROM unnest($1::bigint[]) AS requested(room_id)
		),
		ranked AS (
			SELECT br.id,
			       br.room_id,
			       ROW_NUMBER() OVER (PARTITION BY br.room_id ORDER BY br.id DESC) AS row_no
			FROM battle_runs br
			JOIN requested_rooms requested ON requested.room_id = br.room_id
		),
		doomed AS (
			SELECT ranked.id
			FROM ranked
			LEFT JOIN battle_rooms room ON room.current_run_id = ranked.id
			WHERE ranked.row_no > $2
			  AND room.id IS NULL
			  AND NOT EXISTS (
				SELECT 1
				FROM battle_attr_exp_jobs attr_job
				WHERE attr_job.run_id = ranked.id AND attr_job.applied_at IS NULL
			  )
			  AND NOT EXISTS (
				SELECT 1
				FROM battle_settlement_jobs settlement_job
				WHERE settlement_job.run_id = ranked.id AND settlement_job.status <> 'done'
			  )
			  AND NOT EXISTS (
				SELECT 1
				FROM battle_skill_exp_jobs skill_job
				WHERE skill_job.run_id = ranked.id AND skill_job.status <> 'done'
			  )
			ORDER BY ranked.id ASC
			LIMIT $3
		),
		deleted_archives AS (
			DELETE FROM battle_slow_run_archives archive
			USING doomed d
			WHERE archive.run_id = d.id
		)
		DELETE FROM battle_runs br
		USING doomed d
		WHERE br.id = d.id
	`, roomIDs, keepCount, batchSize)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

func (s *Store) PurgeBattleRunHistory(ctx context.Context, keepCount, batchSize int) (int64, error) {
	if keepCount <= 0 {
		keepCount = 10
	}
	if batchSize <= 0 {
		batchSize = 1000
	}
	tag, err := s.db.Exec(ctx, `
		WITH ranked AS (
			SELECT br.id,
			       ROW_NUMBER() OVER (PARTITION BY br.room_id ORDER BY br.id DESC) AS rn
			FROM battle_runs br
			LEFT JOIN battle_rooms room ON room.current_run_id = br.id
			WHERE room.id IS NULL
			  AND NOT EXISTS (
				SELECT 1
				FROM battle_attr_exp_jobs attr_job
				WHERE attr_job.run_id = br.id AND attr_job.applied_at IS NULL
			  )
			  AND NOT EXISTS (
				SELECT 1
				FROM battle_settlement_jobs settlement_job
				WHERE settlement_job.run_id = br.id AND settlement_job.status <> 'done'
			  )
			  AND NOT EXISTS (
				SELECT 1
				FROM battle_skill_exp_jobs skill_job
				WHERE skill_job.run_id = br.id AND skill_job.status <> 'done'
			  )
		),
		doomed AS (
			SELECT id
			FROM ranked
			WHERE rn > $1
			ORDER BY id ASC
			LIMIT $2
		),
		deleted_archives AS (
			DELETE FROM battle_slow_run_archives archive
			USING doomed d
			WHERE archive.run_id = d.id
		)
		DELETE FROM battle_runs br
		USING doomed d
		WHERE br.id = d.id
	`, keepCount, batchSize)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

func (s *Store) AdminReinitializeBattleRooms(ctx context.Context, activeStatuses []string, now time.Time, recoveryDuration time.Duration) (int64, int64, error) {
	if len(activeStatuses) == 0 {
		activeStatuses = []string{
			BattleRoomStatusEntering,
			BattleRoomStatusPlaying,
			BattleRoomStatusRecovering,
		}
	}
	if recoveryDuration < 0 {
		recoveryDuration = 0
	}
	recoveryEndsAt := now.Add(recoveryDuration)
	normalized := make([]string, 0, len(activeStatuses))
	seen := map[string]struct{}{}
	for _, status := range activeStatuses {
		status = strings.TrimSpace(strings.ToLower(status))
		if status == "" {
			continue
		}
		if _, ok := seen[status]; ok {
			continue
		}
		seen[status] = struct{}{}
		normalized = append(normalized, status)
	}
	if len(normalized) == 0 {
		return 0, 0, nil
	}
	var affectedRooms int64
	var affectedRuns int64
	err := s.withTx(ctx, func(tx pgx.Tx) error {
		var err error
		affectedRooms, affectedRuns, err = adminReinitializeBattleRoomsTx(ctx, tx, normalized, now, recoveryEndsAt, s.battleStateMachineV2)
		return err
	})
	if err != nil {
		return 0, 0, err
	}
	return affectedRooms, affectedRuns, nil
}

func adminReinitializeBattleRoomsTx(ctx context.Context, tx pgx.Tx, activeStatuses []string, now, recoveryEndsAt time.Time, battleStateMachineV2 bool) (int64, int64, error) {
	var affectedRooms int64
	var affectedRuns int64
	err := tx.QueryRow(ctx, `
			WITH target_rooms AS (
				SELECT id, current_run_id
				FROM battle_rooms
				WHERE status = ANY($1::text[])
				FOR UPDATE
			),
			updated_runs AS (
				UPDATE battle_runs br
				SET status = $2,
				    ended_at = COALESCE(br.ended_at, $3),
				    updated_at = now()
				FROM target_rooms tr
				WHERE tr.current_run_id IS NOT NULL
				  AND br.id = tr.current_run_id
				  AND br.status IN ($4, $5, $6)
				RETURNING br.id
			),
			updated_rooms AS (
				UPDATE battle_rooms r
				SET status = $7,
				    current_run_id = NULL,
				    completed_runs = 0,
				    consecutive_failures = 0,
				    seed = 0,
				    entering_started_at = NULL,
				    recovering_started_at = $3,
				    recovery_ends_at = $8,
				    owner_worker_id = '',
				    lease_until = NULL,
				    next_tick_at = $8,
				    phase = '',
				    schema_version = CASE WHEN $9 = true AND r.schema_version < 2 THEN 2 ELSE r.schema_version END,
				    updated_at = now()
				FROM target_rooms tr
				WHERE r.id = tr.id
				RETURNING r.id
			)
			SELECT
				COALESCE((SELECT COUNT(*)::bigint FROM updated_rooms), 0),
				COALESCE((SELECT COUNT(*)::bigint FROM updated_runs), 0)
		`, activeStatuses, BattleRunStatusInterrupted, now, BattleRunStatusCompiled, BattleRunStatusRunning, BattleRunStatusSettling, BattleRoomStatusRecovering, recoveryEndsAt, battleStateMachineV2).Scan(&affectedRooms, &affectedRuns)
	if err != nil {
		return 0, 0, err
	}
	return affectedRooms, affectedRuns, nil
}

func mergeBattleRunResult(summary map[string]any, detail map[string]any, carryState map[string]any) map[string]any {
	out := map[string]any{}
	for _, source := range []map[string]any{summary, detail} {
		for key, value := range source {
			out[key] = value
		}
	}
	if len(carryState) > 0 {
		out["carryState"] = carryState
	}
	return out
}

func (s *Store) loadBattleRunColdArtifacts(ctx context.Context, runID int64, legacyTimelineRaw []byte, legacyFinalResultRaw []byte) ([]map[string]any, map[string]any, map[string]any, error) {
	timeline := []map[string]any{}
	finalResult := map[string]any{}
	carryState := map[string]any{}
	if payload, err := s.GetBattleRunColdPayloadByID(ctx, runID); err == nil {
		timeline = payload.Timeline
		finalResult = payload.FinalResult
	} else if errors.Is(err, ErrBattleRoomNotFound) {
		if len(legacyTimelineRaw) > 0 {
			_ = json.Unmarshal(legacyTimelineRaw, &timeline)
		}
		if len(legacyFinalResultRaw) > 0 {
			_ = json.Unmarshal(legacyFinalResultRaw, &finalResult)
		}
	} else {
		return nil, nil, nil, err
	}
	if state, err := s.GetBattleRunCarryStateByID(ctx, runID); err == nil {
		carryState = state
	} else if len(finalResult) > 0 {
		if raw, ok := finalResult["carryState"].(map[string]any); ok {
			carryState = raw
		}
	}
	return timeline, finalResult, carryState, nil
}

func (s *Store) GetBattleRunByRoom(ctx context.Context, roomID int64) (BattleRun, error) {
	if roomID <= 0 {
		return BattleRun{}, ErrBattleRoomConfigInvalid
	}
	var out BattleRun
	var raw []byte
	var rawSummary []byte
	var rawTimeline []byte
	var rawFinalResult []byte
	var endedAt *time.Time
	err := s.db.QueryRow(ctx, `
		SELECT br.id, br.room_id, br.wave, br.status, br.playback_state_jsonb, br.result_summary_jsonb, br.timeline_jsonb, br.final_result_jsonb, br.started_at, br.ended_at, br.updated_at
		FROM battle_runs br
		JOIN battle_rooms r ON r.current_run_id = br.id
		WHERE r.id = $1
	`, roomID).Scan(&out.ID, &out.RoomID, &out.Wave, &out.Status, &raw, &rawSummary, &rawTimeline, &rawFinalResult, &out.StartedAt, &endedAt, &out.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return BattleRun{}, ErrBattleRoomNotFound
	}
	if err != nil {
		return BattleRun{}, err
	}
	out.State = map[string]any{}
	_ = json.Unmarshal(raw, &out.State)
	summary := map[string]any{}
	if len(rawSummary) > 0 {
		_ = json.Unmarshal(rawSummary, &summary)
	}
	detail, carryState := map[string]any{}, map[string]any{}
	out.Timeline, detail, carryState, err = s.loadBattleRunColdArtifacts(ctx, out.ID, rawTimeline, rawFinalResult)
	if err != nil {
		return BattleRun{}, err
	}
	out.FinalResult = mergeBattleRunResult(summary, detail, carryState)
	out.EndedAt = endedAt
	return out, nil
}

func (s *Store) GetBattleRunByRoomLite(ctx context.Context, roomID int64) (BattleRun, error) {
	if roomID <= 0 {
		return BattleRun{}, ErrBattleRoomConfigInvalid
	}
	var out BattleRun
	var raw []byte
	var endedAt *time.Time
	err := s.db.QueryRow(ctx, `
		SELECT br.id, br.room_id, br.wave, br.status, br.playback_state_jsonb, br.started_at, br.ended_at, br.updated_at
		FROM battle_runs br
		JOIN battle_rooms r ON r.current_run_id = br.id
		WHERE r.id = $1
	`, roomID).Scan(&out.ID, &out.RoomID, &out.Wave, &out.Status, &raw, &out.StartedAt, &endedAt, &out.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return BattleRun{}, ErrBattleRoomNotFound
	}
	if err != nil {
		return BattleRun{}, err
	}
	out.State = map[string]any{}
	_ = json.Unmarshal(raw, &out.State)
	out.EndedAt = endedAt
	return out, nil
}

func (s *Store) GetBattleRunByID(ctx context.Context, runID int64) (BattleRun, error) {
	if runID <= 0 {
		return BattleRun{}, ErrBattleRoomConfigInvalid
	}
	var out BattleRun
	var raw []byte
	var rawSummary []byte
	var rawTimeline []byte
	var rawFinalResult []byte
	var endedAt *time.Time
	err := s.db.QueryRow(ctx, `
		SELECT id, room_id, wave, status, playback_state_jsonb, result_summary_jsonb, timeline_jsonb, final_result_jsonb, started_at, ended_at, updated_at
		FROM battle_runs
		WHERE id = $1
	`, runID).Scan(&out.ID, &out.RoomID, &out.Wave, &out.Status, &raw, &rawSummary, &rawTimeline, &rawFinalResult, &out.StartedAt, &endedAt, &out.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return BattleRun{}, ErrBattleRoomNotFound
	}
	if err != nil {
		return BattleRun{}, err
	}
	out.State = map[string]any{}
	_ = json.Unmarshal(raw, &out.State)
	summary := map[string]any{}
	if len(rawSummary) > 0 {
		_ = json.Unmarshal(rawSummary, &summary)
	}
	detail, carryState := map[string]any{}, map[string]any{}
	out.Timeline, detail, carryState, err = s.loadBattleRunColdArtifacts(ctx, out.ID, rawTimeline, rawFinalResult)
	if err != nil {
		return BattleRun{}, err
	}
	out.FinalResult = mergeBattleRunResult(summary, detail, carryState)
	out.EndedAt = endedAt
	return out, nil
}

const battleRunPlaybackTimelineColdSQL = `
	SELECT COALESCE(timeline_raw, ''::bytea), timeline_jsonb
	FROM battle_run_payloads
	WHERE run_id = $1
`

const battleRunPlaybackTimelineLegacySQL = `
	SELECT timeline_jsonb
	FROM battle_runs
	WHERE id = $1
`

func (s *Store) GetBattleRunPlaybackTimelineByID(ctx context.Context, runID int64) ([]map[string]any, error) {
	if runID <= 0 {
		return nil, ErrBattleRoomConfigInvalid
	}
	var rawTimelineBytes []byte
	var rawTimeline []byte
	err := s.db.QueryRow(ctx, battleRunPlaybackTimelineColdSQL, runID).Scan(&rawTimelineBytes, &rawTimeline)
	if err == nil {
		return decodeBattleRunTimelinePayload(rawTimelineBytes, rawTimeline), nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	err = s.db.QueryRow(ctx, battleRunPlaybackTimelineLegacySQL, runID).Scan(&rawTimeline)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrBattleRoomNotFound
	}
	if err != nil {
		return nil, err
	}
	return decodeBattleRunTimelinePayload(nil, rawTimeline), nil
}

func decodeBattleRunTimelinePayload(rawBytes, rawJSONB []byte) []map[string]any {
	raw := rawJSONB
	if len(rawBytes) > 0 {
		raw = rawBytes
	}
	timeline := []map[string]any{}
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &timeline)
	}
	return timeline
}

func (s *Store) userInAnyBattleRoomTx(ctx context.Context, tx pgx.Tx, userID int64) (bool, error) {
	var exists bool
	err := tx.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1
			FROM battle_room_members
			WHERE user_id = $1
		)
	`, userID).Scan(&exists)
	return exists, err
}

func (s *Store) isBattleRoomMemberTx(ctx context.Context, tx pgx.Tx, roomID, userID int64) bool {
	var exists bool
	_ = tx.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1
			FROM battle_room_members
			WHERE room_id = $1 AND user_id = $2
		)
	`, roomID, userID).Scan(&exists)
	return exists
}

func (s *Store) getBattleRoom(ctx context.Context, roomID int64) (BattleRoom, error) {
	rows, err := s.db.Query(ctx, `
		SELECT id, owner_id, dungeon_id, room_name, auto_reenter_on_defeat, disable_entry_round, is_public, is_fog_mode, status, difficulty_level, completed_runs, consecutive_failures, current_run_id, seed, entering_started_at, recovering_started_at, recovery_ends_at, created_at, updated_at, phase, schema_version
		FROM battle_rooms
		WHERE id = $1
	`, roomID)
	if err != nil {
		return BattleRoom{}, err
	}
	defer rows.Close()
	if !rows.Next() {
		return BattleRoom{}, ErrBattleRoomNotFound
	}
	room, err := scanBattleRoomRow(rows)
	if err != nil {
		return BattleRoom{}, err
	}
	return room, rows.Err()
}

func (s *Store) listBattleRoomMembers(ctx context.Context, roomID int64) ([]BattleRoomMember, error) {
	rows, err := s.db.Query(ctx, `
		SELECT m.room_id, m.user_id, COALESCE(u.username, ''), m.position, m.is_ready, m.ready_at, m.joined_at, m.updated_at
		FROM battle_room_members m
		LEFT JOIN users u ON u.id = m.user_id
		WHERE room_id = $1
		ORDER BY m.joined_at ASC, m.user_id ASC
	`, roomID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]BattleRoomMember, 0, 4)
	for rows.Next() {
		item, err := scanBattleRoomMemberRow(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *Store) listBattleRoomMembersTx(ctx context.Context, tx pgx.Tx, roomID int64, forUpdate bool) ([]BattleRoomMember, error) {
	query := `
		SELECT m.room_id, m.user_id, COALESCE(u.username, ''), m.position, m.is_ready, m.ready_at, m.joined_at, m.updated_at
		FROM battle_room_members m
		LEFT JOIN users u ON u.id = m.user_id
		WHERE m.room_id = $1
		ORDER BY m.joined_at ASC, m.user_id ASC
	`
	if forUpdate {
		query += " FOR UPDATE OF m"
	}
	rows, err := tx.Query(ctx, query, roomID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]BattleRoomMember, 0, 4)
	for rows.Next() {
		item, err := scanBattleRoomMemberRow(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *Store) listBattleRoomUnitsTx(ctx context.Context, tx pgx.Tx, roomID int64, forUpdate bool) ([]BattleRoomUnit, error) {
	query := `
		SELECT u.room_id, u.owner_user_id, COALESCE(owner.username, ''), u.unit_type, u.unit_id, u.kitty_id, COALESCE(k.name, ''), u.position, u.is_ready, u.joined_at, u.updated_at
		FROM battle_room_member_units u
		LEFT JOIN users owner ON owner.id = u.owner_user_id
		LEFT JOIN player_kitties k ON k.id = u.kitty_id
		WHERE u.room_id = $1
		ORDER BY u.joined_at ASC, u.owner_user_id ASC, CASE WHEN u.unit_type = 'player' THEN 0 ELSE 1 END, u.unit_id ASC
	`
	if forUpdate {
		query += " FOR UPDATE OF u"
	}
	rows, err := tx.Query(ctx, query, roomID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]BattleRoomUnit, 0, 8)
	for rows.Next() {
		item, err := scanBattleRoomUnitRow(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *Store) listBattleRoomUnits(ctx context.Context, roomID int64) ([]BattleRoomUnit, error) {
	rows, err := s.db.Query(ctx, `
		SELECT u.room_id, u.owner_user_id, COALESCE(owner.username, ''), u.unit_type, u.unit_id, u.kitty_id, COALESCE(k.name, ''), u.position, u.is_ready, u.joined_at, u.updated_at
		FROM battle_room_member_units u
		LEFT JOIN users owner ON owner.id = u.owner_user_id
		LEFT JOIN player_kitties k ON k.id = u.kitty_id
		WHERE u.room_id = $1
		ORDER BY u.joined_at ASC, u.owner_user_id ASC, CASE WHEN u.unit_type = 'player' THEN 0 ELSE 1 END, u.unit_id ASC
	`, roomID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]BattleRoomUnit, 0, 8)
	for rows.Next() {
		item, err := scanBattleRoomUnitRow(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return s.applyBattleRoomUnitPositionLocks(ctx, out)
}

func (s *Store) applyBattleRoomUnitPositionLocks(ctx context.Context, units []BattleRoomUnit) ([]BattleRoomUnit, error) {
	for idx := range units {
		if units[idx].UnitType != "kitty" || units[idx].KittyID == nil || *units[idx].KittyID <= 0 {
			continue
		}
		lockedPosition, personalityID, personalityKey, err := s.kittyPositionLock(ctx, units[idx].OwnerUserID, *units[idx].KittyID)
		if err != nil {
			return nil, err
		}
		if lockedPosition == "" {
			continue
		}
		units[idx].Position = lockedPosition
		units[idx].PositionLockedByPersonalityID = personalityID
		units[idx].PositionLockedByPersonalityKey = personalityKey
	}
	return units, nil
}

func logBattleRoomUnitRepairPlan(stage string, plan battleRoomUnitRepairPlan) {
	if !plan.hasChanges() {
		return
	}
	mismatches := make([]string, 0, len(plan.PlayerUnitMismatches))
	for _, row := range plan.PlayerUnitMismatches {
		mismatches = append(mismatches, strings.Join([]string{
			"user=" + int64ToString(row.UserID),
			"from_pos=" + row.FromPos,
			"to_pos=" + row.ToPos,
			"from_ready=" + strconv.FormatBool(row.FromReady),
			"to_ready=" + strconv.FormatBool(row.ToReady),
		}, ","))
	}
	sort.Strings(mismatches)
	log.Printf(
		"battle room units repaired stage=%s room_id=%d member_user_ids=%v unit_owner_user_ids=%v missing_player_units=%v player_unit_mismatch=%v orphan_units=%s unexpected_player_units=%s",
		strings.TrimSpace(stage),
		plan.RoomID,
		plan.MemberUserIDs,
		plan.UnitOwnerUserIDs,
		plan.MissingPlayerUnits,
		mismatches,
		battleRoomUnitsLogValue(plan.OrphanUnits),
		battleRoomUnitsLogValue(plan.UnexpectedPlayerUnits),
	)
}

func (s *Store) ensureBattleRoomUnitsConsistentTx(ctx context.Context, tx pgx.Tx, roomID int64, stage string) error {
	members, err := s.listBattleRoomMembersTx(ctx, tx, roomID, true)
	if err != nil {
		return err
	}
	units, err := s.listBattleRoomUnitsTx(ctx, tx, roomID, true)
	if err != nil {
		return err
	}
	plan := planBattleRoomUnitRepair(roomID, members, units)
	if !plan.hasChanges() {
		return nil
	}
	logBattleRoomUnitRepairPlan(stage, plan)
	for _, unit := range plan.canonicalOrphans() {
		if _, err := tx.Exec(ctx, `
			DELETE FROM battle_room_member_units
			WHERE room_id = $1
			  AND owner_user_id = $2
			  AND unit_type = $3
			  AND unit_id = $4
		`, roomID, unit.OwnerUserID, strings.TrimSpace(unit.UnitType), unit.UnitID); err != nil {
			return err
		}
		if strings.TrimSpace(unit.UnitType) == "kitty" && unit.KittyID != nil && *unit.KittyID > 0 {
			if err := s.setOwnedKittyStatusTx(ctx, tx, unit.OwnerUserID, *unit.KittyID, KittyStatusIdle); err != nil {
				return err
			}
		}
	}
	memberByUser := make(map[int64]BattleRoomMember, len(members))
	for _, member := range members {
		if member.UserID > 0 {
			memberByUser[member.UserID] = member
		}
	}
	if len(plan.MissingPlayerUnits) > 0 || len(plan.PlayerUnitMismatches) > 0 {
		touched := map[int64]struct{}{}
		for _, userID := range plan.MissingPlayerUnits {
			touched[userID] = struct{}{}
		}
		for _, mismatch := range plan.PlayerUnitMismatches {
			touched[mismatch.UserID] = struct{}{}
		}
		for _, userID := range sortedInt64Keys(touched) {
			member, ok := memberByUser[userID]
			if !ok {
				continue
			}
			if err := s.upsertBattleRoomPlayerUnitTx(ctx, tx, roomID, userID, member.Position, member.IsReady); err != nil {
				return err
			}
		}
	}
	_, err = tx.Exec(ctx, `UPDATE battle_rooms SET updated_at = now() WHERE id = $1`, roomID)
	return err
}

func scanBattleRoomUnitRow(rows pgx.Rows) (BattleRoomUnit, error) {
	var item BattleRoomUnit
	if err := rows.Scan(
		&item.RoomID,
		&item.OwnerUserID,
		&item.Username,
		&item.UnitType,
		&item.UnitID,
		&item.KittyID,
		&item.KittyName,
		&item.Position,
		&item.IsReady,
		&item.JoinedAt,
		&item.UpdatedAt,
	); err != nil {
		return BattleRoomUnit{}, err
	}
	item.UnitType = strings.TrimSpace(item.UnitType)
	item.KittyName = strings.TrimSpace(item.KittyName)
	item.Username = strings.TrimSpace(item.Username)
	return item, nil
}

func (s *Store) kittyPositionLock(ctx context.Context, userID, kittyID int64) (string, string, string, error) {
	if userID <= 0 || kittyID <= 0 {
		return "", "", "", nil
	}
	rows, err := s.db.Query(ctx, `
		SELECT personality_id
		FROM player_kitty_personalities
		WHERE user_id = $1 AND kitty_id = $2
		ORDER BY created_at ASC, personality_id ASC
	`, userID, kittyID)
	if err != nil {
		return "", "", "", err
	}
	defer rows.Close()
	profiles := make([]KittyPersonalityProfile, 0, 2)
	for rows.Next() {
		var personalityID string
		if err := rows.Scan(&personalityID); err != nil {
			return "", "", "", err
		}
		normalized := normalizeLower(personalityID)
		profile, ok := s.config.KittyPersonalityProfiles[normalized]
		if !ok {
			continue
		}
		profiles = append(profiles, profile)
	}
	if err := rows.Err(); err != nil {
		return "", "", "", err
	}
	locked, personalityID, personalityKey := resolveKittyPositionLockFromProfiles(profiles)
	return locked, personalityID, personalityKey, nil
}

func (s *Store) upsertBattleRoomPlayerUnitTx(ctx context.Context, tx pgx.Tx, roomID, userID int64, position string, ready bool) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO battle_room_member_units(room_id, owner_user_id, unit_type, unit_id, kitty_id, position, is_ready, joined_at, updated_at)
		VALUES($1, $2, 'player', 0, NULL, $3, $4, now(), now())
		ON CONFLICT (room_id, owner_user_id, unit_type, unit_id)
		DO UPDATE SET position = EXCLUDED.position, is_ready = EXCLUDED.is_ready, updated_at = now()
	`, roomID, userID, normalizedBattlePosition(position), ready)
	return err
}

func (s *Store) removeBattleRoomUnitsForUserTx(ctx context.Context, tx pgx.Tx, roomID, userID int64) error {
	rows, err := tx.Query(ctx, `
		SELECT kitty_id
		FROM battle_room_member_units
		WHERE room_id = $1
		  AND owner_user_id = $2
		  AND unit_type = 'kitty'
		  AND kitty_id IS NOT NULL
		FOR UPDATE
	`, roomID, userID)
	if err != nil {
		return err
	}
	kittyIDs := make([]int64, 0, 4)
	for rows.Next() {
		var kittyID int64
		if err := rows.Scan(&kittyID); err != nil {
			rows.Close()
			return err
		}
		if kittyID > 0 {
			kittyIDs = append(kittyIDs, kittyID)
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()
	if _, err := tx.Exec(ctx, `
		DELETE FROM battle_room_member_units
		WHERE room_id = $1 AND owner_user_id = $2
	`, roomID, userID); err != nil {
		return err
	}
	for _, kittyID := range kittyIDs {
		if err := s.setOwnedKittyStatusTx(ctx, tx, userID, kittyID, KittyStatusIdle); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) AddBattleRoomKittyUnit(ctx context.Context, roomID, userID, kittyID int64, position string) (BattleRoomDetail, error) {
	if roomID <= 0 || userID <= 0 || kittyID <= 0 {
		return BattleRoomDetail{}, ErrBattleRoomConfigInvalid
	}
	if err := s.withTx(ctx, func(tx pgx.Tx) error {
		resolvedPosition := normalizedBattlePosition(position)
		var status, dungeonID string
		if err := tx.QueryRow(ctx, `
			SELECT status, dungeon_id
			FROM battle_rooms
			WHERE id = $1
			FOR UPDATE
		`, roomID).Scan(&status, &dungeonID); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrBattleRoomNotFound
			}
			return err
		}
		if status != BattleRoomStatusIdle && status != BattleRoomStatusPreparing {
			return ErrBattleRoomStateInvalid
		}
		var memberReady bool
		if err := tx.QueryRow(ctx, `
			SELECT is_ready
			FROM battle_room_members
			WHERE room_id = $1 AND user_id = $2
			FOR UPDATE
		`, roomID, userID).Scan(&memberReady); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrBattleRoomMemberMissing
			}
			return err
		}
		if memberReady {
			return ErrBattleRoomStateInvalid
		}
		if _, err := s.getOwnedKittyStatusTx(ctx, tx, userID, kittyID, true); err != nil {
			return err
		}
		if personalities, profileErr := s.getOwnedKittyPersonalityProfilesTx(ctx, tx, userID, kittyID); profileErr != nil {
			return profileErr
		} else {
			for _, personality := range personalities {
				for _, effect := range personality.Effects {
					if effect.Kind != "battle_position_lock" {
						continue
					}
					locked := normalizedBattlePosition(effect.Position)
					if locked == "" {
						continue
					}
					resolvedPosition = locked
					break
				}
			}
		}
		maxTeamSize, ok := s.battleWorldMaxTeamSizeByDungeonID(dungeonID)
		if !ok || maxTeamSize <= 0 {
			return ErrBattleRoomConfigInvalid
		}
		var unitCount int
		if err := tx.QueryRow(ctx, `SELECT COUNT(1) FROM battle_room_member_units WHERE room_id = $1`, roomID).Scan(&unitCount); err != nil {
			return err
		}
		var alreadyExists bool
		if err := tx.QueryRow(ctx, `
				SELECT EXISTS(
					SELECT 1
					FROM battle_room_member_units
					WHERE room_id = $1 AND owner_user_id = $2 AND unit_type = 'kitty' AND unit_id = $3::bigint
				)
			`, roomID, userID, kittyID).Scan(&alreadyExists); err != nil {
			return err
		}
		if !alreadyExists && unitCount >= maxTeamSize {
			return ErrBattleRoomFull
		}
		if err := s.setOwnedKittyStatusTx(ctx, tx, userID, kittyID, KittyStatusInCombat); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
				INSERT INTO battle_room_member_units(room_id, owner_user_id, unit_type, unit_id, kitty_id, position, is_ready, joined_at, updated_at)
				VALUES($1, $2, 'kitty', $3::bigint, $3::bigint, $4::text, FALSE, now(), now())
				ON CONFLICT (room_id, owner_user_id, unit_type, unit_id)
				DO UPDATE SET position = EXCLUDED.position, is_ready = FALSE, updated_at = now()
			`, roomID, userID, kittyID, resolvedPosition); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `UPDATE battle_rooms SET updated_at = now() WHERE id = $1`, roomID)
		return err
	}); err != nil {
		return BattleRoomDetail{}, err
	}
	return s.GetBattleRoomDetail(ctx, roomID)
}

func (s *Store) RemoveBattleRoomKittyUnit(ctx context.Context, roomID, userID, kittyID int64) (BattleRoomDetail, error) {
	if roomID <= 0 || userID <= 0 || kittyID <= 0 {
		return BattleRoomDetail{}, ErrBattleRoomConfigInvalid
	}
	if err := s.withTx(ctx, func(tx pgx.Tx) error {
		var status string
		if err := tx.QueryRow(ctx, `SELECT status FROM battle_rooms WHERE id = $1 FOR UPDATE`, roomID).Scan(&status); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrBattleRoomNotFound
			}
			return err
		}
		if status != BattleRoomStatusIdle && status != BattleRoomStatusPreparing {
			return ErrBattleRoomStateInvalid
		}
		tag, err := tx.Exec(ctx, `
			DELETE FROM battle_room_member_units
			WHERE room_id = $1
			  AND owner_user_id = $2
			  AND unit_type = 'kitty'
			  AND unit_id = $3
		`, roomID, userID, kittyID)
		if err != nil {
			return err
		}
		if tag.RowsAffected() <= 0 {
			return ErrBattleRoomMemberMissing
		}
		if err := s.setOwnedKittyStatusTx(ctx, tx, userID, kittyID, KittyStatusIdle); err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `UPDATE battle_rooms SET updated_at = now() WHERE id = $1`, roomID)
		return err
	}); err != nil {
		return BattleRoomDetail{}, err
	}
	return s.GetBattleRoomDetail(ctx, roomID)
}

// FogMemberSnapshot holds a single room-member's compiled battle units for phantom encounters.
type FogMemberSnapshot struct {
	UserID   int64           `json:"userId"`
	Snapshot json.RawMessage `json:"snapshot"` // raw []battleUnitState JSON
}

// DisableFogModeTx sets is_fog_mode=false for the given room within an existing transaction.
func (s *Store) DisableFogModeTx(ctx context.Context, tx pgx.Tx, roomID int64) error {
	_, err := tx.Exec(ctx, `
		UPDATE battle_rooms SET is_fog_mode = FALSE, updated_at = now() WHERE id = $1
	`, roomID)
	return err
}

// UpsertFogMemberSnapshotTx upserts a compiled battle-unit snapshot for one room member.
func (s *Store) UpsertFogMemberSnapshotTx(ctx context.Context, tx pgx.Tx, roomID int64, dungeonID string, userID int64, snapshotJSON []byte) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO fog_room_member_snapshots(room_id, user_id, dungeon_id, snapshot, updated_at)
		VALUES($1, $2, $3, $4, now())
		ON CONFLICT (room_id, user_id) DO UPDATE
		  SET dungeon_id = EXCLUDED.dungeon_id,
		      snapshot   = EXCLUDED.snapshot,
		      updated_at = EXCLUDED.updated_at
	`, roomID, userID, dungeonID, snapshotJSON)
	return err
}

// UpsertFogMemberSnapshot upserts a fog member snapshot in its own transaction.
// Convenience wrapper for callers outside the store package.
func (s *Store) UpsertFogMemberSnapshot(ctx context.Context, roomID int64, dungeonID string, userID int64, snapshotJSON []byte) error {
	return s.withTx(ctx, func(tx pgx.Tx) error {
		return s.UpsertFogMemberSnapshotTx(ctx, tx, roomID, dungeonID, userID, snapshotJSON)
	})
}

// DeleteFogMemberSnapshotsByRoomTx removes all snapshots for a room (called when room goes idle).
func (s *Store) DeleteFogMemberSnapshotsByRoomTx(ctx context.Context, tx pgx.Tx, roomID int64) error {
	_, err := tx.Exec(ctx, `DELETE FROM fog_room_member_snapshots WHERE room_id = $1`, roomID)
	return err
}

// GetFogRoomCandidates returns room IDs of other active fog rooms in the same dungeon.
// Uses a two-step approach: get candidates here, caller picks randomly in Go.
func (s *Store) GetFogRoomCandidates(ctx context.Context, dungeonID string, excludeRoomID int64) ([]int64, error) {
	rows, err := s.db.Query(ctx, `
		SELECT DISTINCT s.room_id
		FROM fog_room_member_snapshots s
		JOIN battle_rooms r ON r.id = s.room_id
		WHERE s.dungeon_id = $1
		  AND r.is_fog_mode = TRUE
		  AND r.status = 'playing'
		  AND s.room_id != $2
	`, dungeonID, excludeRoomID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// GetFogRoomSnapshots returns all member snapshots for a given room.
func (s *Store) GetFogRoomSnapshots(ctx context.Context, roomID int64) ([]FogMemberSnapshot, error) {
	rows, err := s.db.Query(ctx, `
		SELECT user_id, snapshot FROM fog_room_member_snapshots WHERE room_id = $1
	`, roomID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []FogMemberSnapshot
	for rows.Next() {
		var snap FogMemberSnapshot
		if err := rows.Scan(&snap.UserID, &snap.Snapshot); err != nil {
			return nil, err
		}
		out = append(out, snap)
	}
	return out, rows.Err()
}

func scanBattleRoomRow(rows pgx.Rows) (BattleRoom, error) {
	var room BattleRoom
	var currentRunID *int64
	if err := rows.Scan(
		&room.ID,
		&room.OwnerID,
		&room.DungeonID,
		&room.RoomName,
		&room.AutoReenterOnDefeat,
		&room.DisableEntryRound,
		&room.IsPublic,
		&room.IsFogMode,
		&room.Status,
		&room.DifficultyLevel,
		&room.CompletedRuns,
		&room.ConsecutiveFailures,
		&currentRunID,
		&room.Seed,
		&room.EnteringStartedAt,
		&room.RecoveringStartedAt,
		&room.RecoveryEndsAt,
		&room.CreatedAt,
		&room.UpdatedAt,
		&room.Phase,
		&room.SchemaVersion,
	); err != nil {
		return BattleRoom{}, err
	}
	room.CurrentRunID = currentRunID
	return room, nil
}

func scanBattleRoomMemberRow(rows pgx.Rows) (BattleRoomMember, error) {
	var item BattleRoomMember
	if err := rows.Scan(
		&item.RoomID,
		&item.UserID,
		&item.Username,
		&item.Position,
		&item.IsReady,
		&item.ReadyAt,
		&item.JoinedAt,
		&item.UpdatedAt,
	); err != nil {
		return BattleRoomMember{}, err
	}
	return item, nil
}

func battleSeed(roomID int64, now time.Time) int64 {
	h := fnv.New64a()
	writeI64(h, roomID)
	writeI64(h, now.UnixNano())
	seed := int64(h.Sum64())
	if seed == 0 {
		seed = 1
	}
	if seed == math.MinInt64 {
		seed = 42
	}
	return seed
}
