package store

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"mysecondwebgame/server/internal/catalog"
)

var (
	ErrAchievementNotFound      = errors.New("achievement not found")
	ErrAchievementNotCompleted  = errors.New("achievement not completed")
	ErrAchievementRewardClaimed = errors.New("achievement reward already claimed")
	ErrAchievementRewardInvalid = errors.New("achievement reward invalid")
	ErrAchievementUserBusy      = errors.New("achievement user busy")
)

const (
	AchievementEventKillEnemy                  = "kill_enemy"
	AchievementEventCompleteAction             = "complete_action"
	AchievementEventObtainItem                 = "obtain_item"
	AchievementEventEnchantCount               = "enchant_count"
	AchievementEventEnhanceCount               = "enhance_count"
	AchievementEventCompleteCommission         = "complete_commission"
	AchievementEventCompleteRumorCommission    = "complete_rumor_commission"
	AchievementEventCompleteSpecificCommission = "complete_specific_commission"
	AchievementEventStateReconcile             = "state_reconcile"

	AchievementEventStatusPending    = "pending"
	AchievementEventStatusProcessing = "processing"
	AchievementEventStatusProcessed  = "processed"
	AchievementEventStatusFailed     = "failed"

	achievementEventInsertBatchSize = 1000
)

type AchievementEvent struct {
	Kind         string
	Count        int64
	SourceKey    string
	SubjectID    string
	ItemID       string
	ActionID     string
	ActorType    string
	EnemyID      string
	CommissionID string
	Successful   bool
	EnhanceLevel int
	StateKind    string
	AttrKey      string
	ValueSource  string
}

type QueuedAchievementEvent struct {
	EventID    string
	UserID     int64
	Kind       string
	CounterKey string
	Delta      int64
	Attempts   int
	Payload    json.RawMessage
}

type AchievementEventBatch struct {
	UserID int64
	Events []AchievementEvent
}

type achievementEventInsertRow struct {
	EventID    string
	UserID     int64
	Kind       string
	CounterKey string
	Delta      int64
	Payload    string
}

type AchievementClaimResult struct {
	InventoryChanged bool `json:"inventoryChanged"`
	WardrobeChanged  bool `json:"wardrobeChanged"`
}

type PlayerAchievementList struct {
	Achievements []PlayerAchievementStateView `json:"achievements"`
}

type PlayerAchievementStateView struct {
	ID            string     `json:"id"`
	Completed     bool       `json:"completed"`
	Claimed       bool       `json:"claimed"`
	ProgressValue int64      `json:"progressValue"`
	UnlockedAt    *time.Time `json:"unlockedAt,omitempty"`
	CompletedAt   *time.Time `json:"completedAt,omitempty"`
	ClaimedAt     *time.Time `json:"claimedAt,omitempty"`
}

func (s *Store) SetAchievementEventNotifier(notify func(context.Context)) {
	s.achievementNotify = notify
}

func (s *Store) SetAchievementEventEnqueuer(fn func(ctx context.Context, userID int64, events []AchievementEvent) error) {
	s.achievementEnqueue = fn
}

type achievementDefinition struct {
	ID                   string
	BaseID               string
	StageIndex           int
	StageProgressMode    string
	CategoryID           string
	NameKey              string
	DescKey              string
	Rarity               string
	IconID               string
	Order                int64
	Hidden               bool
	Disabled             bool
	PrereqAchievementIDs []string
	Condition            map[string]any
	Rewards              []map[string]any
	Hash                 string
}

type achievementCategoryDefinition struct {
	ID       string
	NameKey  string
	IconID   string
	Order    int64
	Hidden   bool
	Disabled bool
}

type achievementUnlockState struct {
	UnlockedAt    *time.Time
	CompletedAt   *time.Time
	ClaimedAt     *time.Time
	ProgressValue int64
}

func (s *Store) ListPlayerAchievements(ctx context.Context, userID int64) (PlayerAchievementList, error) {
	if !s.achievementsEnabled() {
		return PlayerAchievementList{}, nil
	}
	return s.ListPlayerAchievementsCurrentStates(ctx, userID)
}

// ListPlayerAchievementsCurrentStates returns the player's achievement list from the
// current DB snapshot without reconciling or acquiring the user advisory lock.
// Suitable for push notifications where the caller has already ensured freshness
// (e.g. immediately after a ClaimAchievementReward transaction).
func (s *Store) ListPlayerAchievementsCurrentStates(ctx context.Context, userID int64) (PlayerAchievementList, error) {
	if !s.achievementsEnabled() {
		return PlayerAchievementList{}, nil
	}
	rows, err := s.db.Query(ctx, `
		SELECT achievement_id, unlocked_at, completed_at, claimed_at, progress_value
		FROM player_achievement_states
		WHERE user_id = $1
	`, userID)
	if err != nil {
		return PlayerAchievementList{}, err
	}
	defer rows.Close()
	states := map[string]achievementUnlockState{}
	for rows.Next() {
		var id string
		var st achievementUnlockState
		if err := rows.Scan(&id, &st.UnlockedAt, &st.CompletedAt, &st.ClaimedAt, &st.ProgressValue); err != nil {
			return PlayerAchievementList{}, err
		}
		states[id] = st
	}
	if err := rows.Err(); err != nil {
		return PlayerAchievementList{}, err
	}
	return s.buildPlayerAchievementList(states), nil
}

func (s *Store) ClaimAchievementReward(ctx context.Context, userID int64, achievementID string) (AchievementClaimResult, error) {
	achievementID = strings.TrimSpace(achievementID)
	if achievementID == "" {
		return AchievementClaimResult{}, ErrAchievementNotFound
	}
	if !s.achievementsEnabled() {
		return AchievementClaimResult{}, ErrAchievementNotFound
	}
	var result AchievementClaimResult
	err := s.withTx(ctx, func(tx pgx.Tx) error {
		if err := s.lockUserTx(ctx, tx, userID); err != nil {
			return err
		}
		if err := s.ensureAchievementUnlocksTx(ctx, tx, userID); err != nil {
			return err
		}
		if err := s.reconcileStateAchievementsTx(ctx, tx, userID); err != nil {
			return err
		}
		def, ok := s.achievementDefinitionByID(achievementID)
		if !ok || def.Disabled {
			return ErrAchievementNotFound
		}
		var completedAt *time.Time
		var claimedAt *time.Time
		if err := tx.QueryRow(ctx, `
			SELECT completed_at, claimed_at
			FROM player_achievement_states
			WHERE user_id = $1 AND achievement_id = $2
			FOR UPDATE
		`, userID, achievementID).Scan(&completedAt, &claimedAt); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrAchievementNotCompleted
			}
			return err
		}
		if completedAt == nil {
			return ErrAchievementNotCompleted
		}
		if claimedAt != nil {
			return ErrAchievementRewardClaimed
		}
		for _, reward := range def.Rewards {
			kind := strings.TrimSpace(stringFromAchievementAny(reward["kind"]))
			switch kind {
			case "item":
				itemID := strings.TrimSpace(stringFromAchievementAny(reward["itemId"]))
				qty := int64FromAchievementAny(reward["quantity"], 0)
				if itemID == "" || qty <= 0 {
					return ErrAchievementRewardInvalid
				}
				if err := s.LedgerGrantTemplateTx(ctx, tx, userID, itemID, qty, InventoryLedgerMeta{
					Category:      InventoryAuditCategorySystem,
					SourceSurface: "achievement",
					TargetSurface: "inventory",
					Details: map[string]any{
						"type":          "achievement_reward",
						"achievementId": achievementID,
					},
				}); err != nil {
					return err
				}
				result.InventoryChanged = true
			case "wardrobe":
				category := strings.TrimSpace(stringFromAchievementAny(reward["category"]))
				itemID := strings.TrimSpace(stringFromAchievementAny(reward["itemId"]))
				if category == "" || itemID == "" {
					return ErrAchievementRewardInvalid
				}
				if _, err := s.GrantCosmeticTx(ctx, tx, userID, CosmeticGrant{
					Category: category,
					ItemID:   itemID,
					Source:   "achievement:" + achievementID,
				}); err != nil {
					return err
				}
				result.WardrobeChanged = true
			default:
				return fmt.Errorf("%w: %s", ErrAchievementRewardInvalid, kind)
			}
		}
		_, err := tx.Exec(ctx, `
			UPDATE player_achievement_states
			SET claimed_at = now()
			WHERE user_id = $1 AND achievement_id = $2 AND claimed_at IS NULL
		`, userID, achievementID)
		return err
	})
	return result, err
}

func (s *Store) ApplyAchievementEvents(ctx context.Context, userID int64, events []AchievementEvent) error {
	return s.EnqueueAchievementEvents(ctx, userID, events)
}

func (s *Store) ApplyAchievementEventsNoReconcile(ctx context.Context, userID int64, events []AchievementEvent) error {
	return s.EnqueueAchievementEvents(ctx, userID, events)
}

func (s *Store) EnqueueAchievementEvents(ctx context.Context, userID int64, events []AchievementEvent) error {
	if !s.achievementsEnabled() || userID <= 0 || len(events) == 0 {
		return nil
	}
	if s.achievementEnqueue != nil {
		if err := s.achievementEnqueue(ctx, userID, events); err == nil {
			return nil
		}
	}
	return s.withTx(ctx, func(tx pgx.Tx) error {
		return s.EnqueueAchievementEventsTx(ctx, tx, userID, events)
	})
}

func (s *Store) BulkEnqueueAchievementEvents(ctx context.Context, batches []AchievementEventBatch) error {
	if !s.achievementsEnabled() || len(batches) == 0 {
		return nil
	}
	rows, err := prepareAchievementEventBatchRows(batches)
	if err != nil || len(rows) == 0 {
		return err
	}
	inserted := false
	err = s.withTx(ctx, func(tx pgx.Tx) error {
		var insertErr error
		inserted, insertErr = s.insertAchievementEventRowsTx(ctx, tx, rows)
		return insertErr
	})
	if err != nil {
		return err
	}
	if inserted && s.achievementNotify != nil {
		s.achievementNotify(ctx)
	}
	return nil
}

func (s *Store) ApplyAchievementEventsTx(ctx context.Context, tx pgx.Tx, userID int64, events []AchievementEvent) error {
	return s.EnqueueAchievementEventsTx(ctx, tx, userID, events)
}

func (s *Store) ApplyAchievementEventsNoReconcileTx(ctx context.Context, tx pgx.Tx, userID int64, events []AchievementEvent) error {
	return s.EnqueueAchievementEventsTx(ctx, tx, userID, events)
}

func (s *Store) EnqueueAchievementEventsTx(ctx context.Context, tx pgx.Tx, userID int64, events []AchievementEvent) error {
	if !s.achievementsEnabled() || userID <= 0 || len(events) == 0 {
		return nil
	}
	rows, err := prepareAchievementEventRows(userID, events)
	if err != nil || len(rows) == 0 {
		return err
	}
	if batch, ok := ctx.Value(offlineAchievementBatchKey{}).(*offlineAchievementBatch); ok && batch.store == s && batch.tx == tx && batch.userID == userID {
		return batch.append(ctx, rows)
	}
	inserted, err := s.insertAchievementEventRowsTx(ctx, tx, rows)
	if err != nil {
		return err
	}
	if inserted && s.achievementNotify != nil {
		s.achievementNotify(ctx)
	}
	return nil
}

func prepareAchievementEventBatchRows(batches []AchievementEventBatch) ([]achievementEventInsertRow, error) {
	rows := make([]achievementEventInsertRow, 0)
	for _, batch := range batches {
		if batch.UserID <= 0 || len(batch.Events) == 0 {
			continue
		}
		prepared, err := prepareAchievementEventRows(batch.UserID, batch.Events)
		if err != nil {
			return nil, err
		}
		rows = append(rows, prepared...)
	}
	return rows, nil
}

func prepareAchievementEventRows(userID int64, events []AchievementEvent) ([]achievementEventInsertRow, error) {
	if userID <= 0 || len(events) == 0 {
		return nil, nil
	}
	events = normalizeAchievementEvents(events)
	rows := make([]achievementEventInsertRow, 0, len(events))
	for _, event := range events {
		counterKey := achievementEventPrimaryCounterKey(event)
		if counterKey == "" {
			continue
		}
		payload, err := json.Marshal(event)
		if err != nil {
			return nil, err
		}
		rows = append(rows, achievementEventInsertRow{
			EventID:    achievementEventID(userID, event, counterKey),
			UserID:     userID,
			Kind:       event.Kind,
			CounterKey: counterKey,
			Delta:      event.Count,
			Payload:    string(payload),
		})
	}
	return rows, nil
}

func (s *Store) insertAchievementEventRowsTx(ctx context.Context, tx pgx.Tx, rows []achievementEventInsertRow) (bool, error) {
	inserted := false
	for start := 0; start < len(rows); start += achievementEventInsertBatchSize {
		end := min(start+achievementEventInsertBatchSize, len(rows))
		chunk := rows[start:end]
		if len(chunk) == 1 {
			row := chunk[0]
			tag, err := tx.Exec(ctx, `
				INSERT INTO achievement_events(event_id, user_id, kind, counter_key, delta, payload, status, available_at)
				VALUES($1, $2, $3, $4, $5, $6::jsonb, 'pending', now())
				ON CONFLICT (event_id) DO NOTHING
			`, row.EventID, row.UserID, row.Kind, row.CounterKey, row.Delta, row.Payload)
			if err != nil {
				return false, err
			}
			inserted = inserted || tag.RowsAffected() > 0
			continue
		}

		eventIDs := make([]string, 0, len(chunk))
		userIDs := make([]int64, 0, len(chunk))
		kinds := make([]string, 0, len(chunk))
		counterKeys := make([]string, 0, len(chunk))
		deltas := make([]int64, 0, len(chunk))
		payloads := make([]string, 0, len(chunk))
		for _, row := range chunk {
			eventIDs = append(eventIDs, row.EventID)
			userIDs = append(userIDs, row.UserID)
			kinds = append(kinds, row.Kind)
			counterKeys = append(counterKeys, row.CounterKey)
			deltas = append(deltas, row.Delta)
			payloads = append(payloads, row.Payload)
		}
		tag, err := tx.Exec(ctx, `
			INSERT INTO achievement_events(event_id, user_id, kind, counter_key, delta, payload, status, available_at)
			SELECT input.event_id,
			       input.user_id,
			       input.kind,
			       input.counter_key,
			       input.delta,
			       input.payload::jsonb,
			       'pending',
			       now()
			FROM unnest($1::text[], $2::bigint[], $3::text[], $4::text[], $5::bigint[], $6::text[])
			  AS input(event_id, user_id, kind, counter_key, delta, payload)
			ON CONFLICT (event_id) DO NOTHING
		`, eventIDs, userIDs, kinds, counterKeys, deltas, payloads)
		if err != nil {
			return false, err
		}
		inserted = inserted || tag.RowsAffected() > 0
	}
	return inserted, nil
}

func (s *Store) ReconcileStateAchievements(ctx context.Context, userID int64) error {
	if !s.achievementsEnabled() {
		return nil
	}
	return s.withTx(ctx, func(tx pgx.Tx) error {
		if err := s.lockUserTx(ctx, tx, userID); err != nil {
			return err
		}
		if err := s.ensureAchievementUnlocksTx(ctx, tx, userID); err != nil {
			return err
		}
		return s.reconcileStateAchievementsTx(ctx, tx, userID)
	})
}

func (s *Store) achievementsEnabled() bool {
	return s.config.AchievementsEnabled
}

func (s *Store) reconcileStateAchievementsTx(ctx context.Context, tx pgx.Tx, userID int64) error {
	states, err := s.loadAchievementUnlockStatesTx(ctx, tx, userID)
	if err != nil {
		return err
	}
	// Cache per-query-type results to avoid N identical DB calls for N defs of the same kind.
	var cachedKittyCount *int64
	cachedItemCount := map[string]int64{}
	var cachedFishingCatchCounts map[string]int64
	for _, def := range s.achievementDefinitions() {
		state, unlocked := states[def.ID]
		if !unlocked || state.CompletedAt != nil {
			continue
		}
		kind := achievementConditionKind(def)
		var value int64
		var valueErr error
		switch kind {
		case "own_kitty":
			if cachedKittyCount == nil {
				v, e := s.currentAchievementKittyCountTx(ctx, tx, userID)
				if e != nil {
					return e
				}
				cachedKittyCount = &v
			}
			value = *cachedKittyCount
		case "own_item":
			itemID := strings.TrimSpace(stringFromAchievementAny(def.Condition["itemId"]))
			if cached, ok := cachedItemCount[itemID]; ok {
				value = cached
			} else {
				value, valueErr = s.currentAchievementItemCountTx(ctx, tx, userID, itemID)
				if valueErr != nil {
					return valueErr
				}
				cachedItemCount[itemID] = value
			}
		case "player_attr_reached":
			value, valueErr = s.currentAchievementPlayerAttrValueTx(ctx, tx, userID, def)
			if valueErr != nil {
				return valueErr
			}
		case "catch_fish":
			if cachedFishingCatchCounts == nil {
				cachedFishingCatchCounts, valueErr = s.currentAchievementFishingCatchCountsTx(ctx, tx, userID)
				if valueErr != nil {
					return valueErr
				}
			}
			value = cachedFishingCatchCounts[strings.TrimSpace(stringFromAchievementAny(def.Condition["speciesId"]))]
		default:
			continue
		}
		if value >= achievementTargetCount(def) {
			if err := s.completeAchievementTx(ctx, tx, userID, def, value); err != nil {
				return err
			}
			// Sync in-memory so ensureAchievementUnlocksTxWithStates sees completed prereqs
			// without a second DB round-trip.
			existing := states[def.ID]
			now := time.Now()
			existing.CompletedAt = &now
			states[def.ID] = existing
		} else if err := s.updateAchievementProgressTx(ctx, tx, userID, def.ID, value); err != nil {
			return err
		}
	}
	// Reuse the already-loaded (and in-memory-updated) states map instead of re-loading from DB.
	return s.ensureAchievementUnlocksTxWithStates(ctx, tx, userID, states)
}

func (s *Store) ensureAchievementUnlocksTx(ctx context.Context, tx pgx.Tx, userID int64) error {
	states, err := s.loadAchievementUnlockStatesTx(ctx, tx, userID)
	if err != nil {
		return err
	}
	return s.ensureAchievementUnlocksTxWithStates(ctx, tx, userID, states)
}

func (s *Store) ensureAchievementUnlocksTxWithStates(ctx context.Context, tx pgx.Tx, userID int64, states map[string]achievementUnlockState) error {
	defs := s.achievementDefinitions()
	if len(defs) == 0 {
		return nil
	}
	for pass := 0; pass < len(defs)+1; pass++ {
		inserted := false
		for _, def := range defs {
			if _, exists := states[def.ID]; exists || def.Disabled {
				continue
			}
			if !achievementPrereqsCompleted(def, states) {
				continue
			}
			initialProgress, initialCompleted := s.initialAchievementProgressTx(def, states)
			if _, err := tx.Exec(ctx, `
				INSERT INTO player_achievement_states(user_id, achievement_id, unlocked_at, progress_value, definition_hash)
				VALUES($1, $2, now(), $3, $4)
				ON CONFLICT (user_id, achievement_id) DO NOTHING
			`, userID, def.ID, initialProgress, def.Hash); err != nil {
				return err
			}
			if initialCompleted {
				if err := s.completeAchievementTx(ctx, tx, userID, def, initialProgress); err != nil {
					return err
				}
			}
			newState := achievementUnlockState{
				UnlockedAt:    timePtr(time.Now()),
				ProgressValue: initialProgress,
			}
			if initialCompleted {
				now := time.Now()
				newState.CompletedAt = &now
			}
			states[def.ID] = newState
			inserted = true
		}
		if !inserted {
			return nil
		}
	}
	return nil
}

func timePtr(t time.Time) *time.Time {
	return &t
}

func (s *Store) initialAchievementProgressTx(def achievementDefinition, states map[string]achievementUnlockState) (int64, bool) {
	if def.StageProgressMode != "cumulative" || def.StageIndex <= 1 || def.BaseID == "" {
		return 0, false
	}
	previousDef, ok := s.achievementDefinitionByID(achievementStageID(def.BaseID, def.StageIndex-2))
	if !ok {
		return 0, false
	}
	progress := achievementTargetCount(previousDef)
	if previousState, ok := states[previousDef.ID]; ok && previousState.ProgressValue > progress {
		progress = previousState.ProgressValue
	}
	target := achievementTargetCount(def)
	return progress, progress >= target
}

func (s *Store) loadAchievementUnlockStatesTx(ctx context.Context, tx pgx.Tx, userID int64) (map[string]achievementUnlockState, error) {
	rows, err := tx.Query(ctx, `
		SELECT achievement_id, unlocked_at, completed_at, claimed_at, progress_value
		FROM player_achievement_states
		WHERE user_id = $1
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]achievementUnlockState{}
	for rows.Next() {
		var id string
		var state achievementUnlockState
		if err := rows.Scan(&id, &state.UnlockedAt, &state.CompletedAt, &state.ClaimedAt, &state.ProgressValue); err != nil {
			return nil, err
		}
		out[id] = state
	}
	return out, rows.Err()
}

func (s *Store) updateAchievementProgressTx(ctx context.Context, tx pgx.Tx, userID int64, achievementID string, value int64) error {
	_, err := tx.Exec(ctx, `
		UPDATE player_achievement_states
		SET progress_value = $3,
		    updated_at = now()
		WHERE user_id = $1 AND achievement_id = $2 AND completed_at IS NULL
	`, userID, achievementID, value)
	return err
}

func (s *Store) completeAchievementTx(ctx context.Context, tx pgx.Tx, userID int64, def achievementDefinition, value int64) error {
	_, err := tx.Exec(ctx, `
		UPDATE player_achievement_states
		SET completed_at = COALESCE(completed_at, now()),
		    progress_value = $3,
		    definition_hash = $4,
		    updated_at = now()
		WHERE user_id = $1 AND achievement_id = $2
	`, userID, def.ID, value, def.Hash)
	return err
}

func (s *Store) ClaimQueuedAchievementEvents(ctx context.Context, workerID string, limit int, lease time.Duration) ([]QueuedAchievementEvent, error) {
	workerID = strings.TrimSpace(workerID)
	if workerID == "" {
		workerID = "achievement-worker"
	}
	if limit <= 0 {
		limit = 200
	}
	if lease <= 0 {
		lease = 30 * time.Second
	}
	leaseSeconds := int64(lease.Seconds())
	if leaseSeconds <= 0 {
		leaseSeconds = 30
	}
	rows, err := s.db.Query(ctx, `
		WITH pending AS (
			SELECT event_id, available_at AS sort_at, created_at
			FROM achievement_events
			WHERE status = 'pending'
			  AND available_at <= now()
			ORDER BY available_at ASC, created_at ASC
			LIMIT $1
			FOR UPDATE SKIP LOCKED
		), reclaim AS (
			SELECT event_id, locked_until AS sort_at, created_at
			FROM achievement_events
			WHERE status = 'processing'
			  AND locked_until IS NOT NULL
			  AND locked_until <= now()
			ORDER BY locked_until ASC, created_at ASC
			LIMIT $1
			FOR UPDATE SKIP LOCKED
		), picked AS (
			SELECT event_id
			FROM (
				SELECT event_id, sort_at, created_at FROM pending
				UNION ALL
				SELECT event_id, sort_at, created_at FROM reclaim
			) candidates
			ORDER BY sort_at ASC, created_at ASC
			LIMIT $1
		)
		UPDATE achievement_events e
		SET status = 'processing',
		    locked_by = $2,
		    locked_until = now() + ($3 * interval '1 second'),
		    updated_at = now()
		FROM picked
		WHERE e.event_id = picked.event_id
		RETURNING e.event_id, e.user_id, e.kind, e.counter_key, e.delta, e.attempts, e.payload
	`, limit, workerID, leaseSeconds)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	events := make([]QueuedAchievementEvent, 0, limit)
	for rows.Next() {
		var event QueuedAchievementEvent
		if err := rows.Scan(&event.EventID, &event.UserID, &event.Kind, &event.CounterKey, &event.Delta, &event.Attempts, &event.Payload); err != nil {
			return nil, err
		}
		events = append(events, event)
	}
	return events, rows.Err()
}

func (s *Store) ProcessQueuedAchievementEvent(ctx context.Context, queued QueuedAchievementEvent, workerID string) error {
	err := s.withTx(ctx, func(tx pgx.Tx) error {
		var status string
		var lockedBy string
		if err := tx.QueryRow(ctx, `
			SELECT status, locked_by
			FROM achievement_events
			WHERE event_id = $1
			FOR UPDATE
		`, queued.EventID).Scan(&status, &lockedBy); err != nil {
			return err
		}
		if status != AchievementEventStatusProcessing || lockedBy != strings.TrimSpace(workerID) {
			return nil
		}
		locked, err := s.tryLockUserTx(ctx, tx, queued.UserID)
		if err != nil {
			return err
		}
		if !locked {
			return ErrAchievementUserBusy
		}
		if err := s.ensureAchievementUnlocksTx(ctx, tx, queued.UserID); err != nil {
			return err
		}
		var event AchievementEvent
		if len(queued.Payload) > 0 {
			if err := json.Unmarshal(queued.Payload, &event); err != nil {
				return err
			}
		}
		event.Kind = strings.TrimSpace(firstNonEmptyAchievementString(event.Kind, queued.Kind))
		if event.Count <= 0 {
			event.Count = queued.Delta
		}
		if event.Count <= 0 {
			event.Count = 1
		}
		if event.Kind == AchievementEventStateReconcile {
			if err := s.reconcileStateAchievementsTx(ctx, tx, queued.UserID); err != nil {
				return err
			}
			return s.markQueuedAchievementEventProcessedTx(ctx, tx, queued.EventID, workerID)
		}
		if err := s.processAchievementCounterEventTx(ctx, tx, queued.UserID, event); err != nil {
			return err
		}
		return s.markQueuedAchievementEventProcessedTx(ctx, tx, queued.EventID, workerID)
	})
	if err != nil {
		if errors.Is(err, ErrAchievementUserBusy) {
			return s.DeferQueuedAchievementEvents(ctx, []string{queued.EventID}, workerID, 2*time.Second, "user busy")
		}
		_ = s.MarkQueuedAchievementEventFailed(ctx, queued.EventID, workerID, err)
		return err
	}
	return nil
}

// ProcessQueuedAchievementEventsBatch processes multiple events grouped by user in a single tx per user.
// It returns only users whose processing transaction committed, so deferred or failed work does not
// trigger a stale full-state push.
func (s *Store) ProcessQueuedAchievementEventsBatch(ctx context.Context, events []QueuedAchievementEvent, workerID string) ([]int64, error) {
	if len(events) == 0 {
		return nil, nil
	}
	// Group by userID
	byUser := make(map[int64][]QueuedAchievementEvent)
	for _, e := range events {
		byUser[e.UserID] = append(byUser[e.UserID], e)
	}
	// Sort userIDs for deterministic processing order
	userIDs := make([]int64, 0, len(byUser))
	for uid := range byUser {
		userIDs = append(userIDs, uid)
	}
	sort.Slice(userIDs, func(i, j int) bool { return userIDs[i] < userIDs[j] })

	processedUserIDs := make([]int64, 0, len(userIDs))
	for _, userID := range userIDs {
		userEvents := byUser[userID]
		if err := s.processUserAchievementEventsBatchTx(ctx, userID, userEvents, workerID); err != nil {
			if errors.Is(err, ErrAchievementUserBusy) {
				ids := make([]string, 0, len(userEvents))
				for _, e := range userEvents {
					ids = append(ids, e.EventID)
				}
				_ = s.DeferQueuedAchievementEvents(ctx, ids, workerID, 2*time.Second, "user busy")
				continue
			}
			// Mark all events as failed
			for _, e := range userEvents {
				_ = s.MarkQueuedAchievementEventFailed(ctx, e.EventID, workerID, err)
			}
			continue
		}
		processedUserIDs = append(processedUserIDs, userID)
	}
	return processedUserIDs, nil
}

func (s *Store) processUserAchievementEventsBatchTx(ctx context.Context, userID int64, events []QueuedAchievementEvent, workerID string) error {
	return s.withTx(ctx, func(tx pgx.Tx) error {
		locked, err := s.tryLockUserTx(ctx, tx, userID)
		if err != nil {
			return err
		}
		if !locked {
			return ErrAchievementUserBusy
		}
		// Load states ONCE for all events
		states, err := s.loadAchievementUnlockStatesTx(ctx, tx, userID)
		if err != nil {
			return err
		}
		// Ensure unlocks ONCE
		if err := s.ensureAchievementUnlocksTxWithStates(ctx, tx, userID, states); err != nil {
			return err
		}
		processedIDs := make([]string, 0, len(events))
		// Process each event using pre-loaded states
		for _, queued := range events {
			var event AchievementEvent
			if len(queued.Payload) > 0 {
				if err := json.Unmarshal(queued.Payload, &event); err != nil {
					return err
				}
			}
			event.Kind = strings.TrimSpace(firstNonEmptyAchievementString(event.Kind, queued.Kind))
			if event.Count <= 0 {
				event.Count = queued.Delta
			}
			if event.Count <= 0 {
				event.Count = 1
			}
			if event.Kind == AchievementEventStateReconcile {
				if err := s.reconcileStateAchievementsTx(ctx, tx, userID); err != nil {
					return err
				}
			} else {
				if err := s.processAchievementCounterEventWithStatesTx(ctx, tx, userID, event, states); err != nil {
					return err
				}
			}
			processedIDs = append(processedIDs, queued.EventID)
		}
		// Single UPDATE for all processed events instead of one per event.
		return s.markQueuedAchievementEventsProcessedBatchTx(ctx, tx, processedIDs, workerID)
	})
}

func (s *Store) processAchievementCounterEventTx(ctx context.Context, tx pgx.Tx, userID int64, event AchievementEvent) error {
	states, err := s.loadAchievementUnlockStatesTx(ctx, tx, userID)
	if err != nil {
		return err
	}
	return s.processAchievementCounterEventWithStatesTx(ctx, tx, userID, event, states)
}

func (s *Store) processAchievementCounterEventWithStatesTx(ctx context.Context, tx pgx.Tx, userID int64, event AchievementEvent, states map[string]achievementUnlockState) error {
	defsByCounterKey := s.achievementDefinitionsByCounterKey()
	seenDefs := map[string]struct{}{}
	anyCompleted := false
	for _, counterKey := range achievementEventCandidateCounterKeys(event) {
		for _, def := range defsByCounterKey[counterKey] {
			if _, seen := seenDefs[def.ID]; seen {
				continue
			}
			seenDefs[def.ID] = struct{}{}
			state, unlocked := states[def.ID]
			if !unlocked || state.CompletedAt != nil {
				continue
			}
			matchedCounterKey, ok := achievementEventMatches(def, event)
			if !ok || matchedCounterKey != counterKey {
				continue
			}
			value, completed, err := s.incrementAchievementProgressTx(ctx, tx, userID, def, event.Count)
			if err != nil {
				return err
			}
			state.ProgressValue = value
			if completed {
				now := time.Now()
				state.CompletedAt = &now
				anyCompleted = true
			}
			states[def.ID] = state
		}
	}
	// Only run ensureUnlocks when an achievement was completed — completing one achievement
	// may satisfy the prereq for the next stage, which ensureUnlocks will then insert.
	if anyCompleted {
		return s.ensureAchievementUnlocksTxWithStates(ctx, tx, userID, states)
	}
	return nil
}

func (s *Store) incrementAchievementProgressTx(ctx context.Context, tx pgx.Tx, userID int64, def achievementDefinition, delta int64) (int64, bool, error) {
	if delta <= 0 {
		delta = 1
	}
	target := achievementTargetCount(def)
	var value int64
	var completedAt *time.Time
	err := tx.QueryRow(ctx, `
		UPDATE player_achievement_states
		SET progress_value = progress_value + $3,
		    completed_at = CASE
		      WHEN completed_at IS NULL AND progress_value + $3 >= $4 THEN now()
		      ELSE completed_at
		    END,
		    definition_hash = $5,
		    updated_at = now()
		WHERE user_id = $1 AND achievement_id = $2 AND completed_at IS NULL
		RETURNING progress_value, completed_at
	`, userID, def.ID, delta, target, def.Hash).Scan(&value, &completedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, false, nil
	}
	return value, completedAt != nil, err
}

func (s *Store) markQueuedAchievementEventProcessedTx(ctx context.Context, tx pgx.Tx, eventID string, workerID string) error {
	_, err := tx.Exec(ctx, `
		UPDATE achievement_events
		SET status = 'processed',
		    processed_at = now(),
		    locked_by = '',
		    locked_until = NULL,
		    updated_at = now(),
		    last_error = ''
		WHERE event_id = $1 AND status = 'processing' AND locked_by = $2
	`, eventID, strings.TrimSpace(workerID))
	return err
}

func (s *Store) markQueuedAchievementEventsProcessedBatchTx(ctx context.Context, tx pgx.Tx, eventIDs []string, workerID string) error {
	if len(eventIDs) == 0 {
		return nil
	}
	_, err := tx.Exec(ctx, `
		UPDATE achievement_events
		SET status = 'processed',
		    processed_at = now(),
		    locked_by = '',
		    locked_until = NULL,
		    updated_at = now(),
		    last_error = ''
		WHERE event_id = ANY($1) AND status = 'processing' AND locked_by = $2
	`, eventIDs, strings.TrimSpace(workerID))
	return err
}

func (s *Store) MarkQueuedAchievementEventFailed(ctx context.Context, eventID string, workerID string, cause error) error {
	message := ""
	if cause != nil {
		message = cause.Error()
	}
	if len(message) > 1000 {
		message = message[:1000]
	}
	_, err := s.db.Exec(ctx, `
		UPDATE achievement_events
		SET attempts = attempts + 1,
		    status = CASE WHEN attempts + 1 >= 10 THEN 'failed' ELSE 'pending' END,
		    available_at = CASE
		      WHEN attempts + 1 >= 10 THEN available_at
		      ELSE now() + (LEAST(300, POWER(2, attempts + 1))::text || ' seconds')::interval
		    END,
		    locked_by = '',
		    locked_until = NULL,
		    updated_at = now(),
		    last_error = $2
		WHERE event_id = $1 AND status = 'processing' AND locked_by = $3
	`, eventID, message, strings.TrimSpace(workerID))
	return err
}

func (s *Store) DeferQueuedAchievementEvents(ctx context.Context, eventIDs []string, workerID string, delay time.Duration, reason string) error {
	if len(eventIDs) == 0 {
		return nil
	}
	if delay <= 0 {
		delay = 2 * time.Second
	}
	if len(reason) > 1000 {
		reason = reason[:1000]
	}
	_, err := s.db.Exec(ctx, `
		UPDATE achievement_events
		SET status = 'pending',
		    available_at = now() + ($2 * interval '1 second'),
		    locked_by = '',
		    locked_until = NULL,
		    updated_at = now(),
		    last_error = $4
		WHERE event_id = ANY($1) AND status = 'processing' AND locked_by = $3
	`, eventIDs, int64(delay/time.Second), strings.TrimSpace(workerID), reason)
	return err
}

func (s *Store) CleanupAchievementEvents(ctx context.Context) error {
	// Delete in batches to avoid generating a single huge WAL write.
	for {
		tag, err := s.db.Exec(ctx, `
			DELETE FROM achievement_events
			WHERE event_id IN (
				SELECT event_id FROM achievement_events
				WHERE (status = 'processed' AND processed_at < now() - interval '10 minutes')
				   OR (status = 'failed'    AND updated_at   < now() - interval '30 days')
				LIMIT 5000
			)
		`)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return nil
		}
	}
}

func (s *Store) currentAchievementKittyCountTx(ctx context.Context, tx pgx.Tx, userID int64) (int64, error) {
	var count int64
	err := tx.QueryRow(ctx, `SELECT COUNT(1) FROM player_kitties WHERE user_id = $1`, userID).Scan(&count)
	return count, err
}

func (s *Store) currentAchievementFishingCatchCountsTx(ctx context.Context, tx pgx.Tx, userID int64) (map[string]int64, error) {
	rows, err := tx.Query(ctx, `
		SELECT species_id, catches
		FROM fishing_species_knowledge
		WHERE user_id = $1 AND catches > 0
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	counts := map[string]int64{}
	for rows.Next() {
		var speciesID string
		var catches int64
		if err := rows.Scan(&speciesID, &catches); err != nil {
			return nil, err
		}
		if speciesID = strings.TrimSpace(speciesID); speciesID != "" && catches > 0 {
			counts[speciesID] = catches
		}
	}
	return counts, rows.Err()
}

func normalizeAchievementEvents(events []AchievementEvent) []AchievementEvent {
	out := make([]AchievementEvent, 0, len(events))
	for _, event := range events {
		event.Kind = strings.TrimSpace(event.Kind)
		if event.Kind == "" {
			continue
		}
		if event.Count <= 0 {
			event.Count = 1
		}
		out = append(out, event)
	}
	return out
}

func (s *Store) currentAchievementItemCountTx(ctx context.Context, tx pgx.Tx, userID int64, itemID string) (int64, error) {
	itemID = strings.TrimSpace(itemID)
	if itemID == "" {
		return 0, nil
	}
	var count int64
	err := tx.QueryRow(ctx, `
		SELECT COALESCE(SUM(quantity), 0)
		FROM player_item_instances
		WHERE user_id = $1 AND template_item_id = $2
	`, userID, itemID).Scan(&count)
	return count, err
}

func (s *Store) currentAchievementPlayerAttrValueTx(ctx context.Context, tx pgx.Tx, userID int64, def achievementDefinition) (int64, error) {
	attrKey := strings.TrimSpace(stringFromAchievementAny(def.Condition["attrKey"]))
	if attrKey == "" {
		return 0, nil
	}
	if err := s.ensureAllPlayerAttributesTx(ctx, tx, userID); err != nil {
		return 0, err
	}
	var baseLevel int64
	var skillPointLevel int64
	var finalValue float64
	err := tx.QueryRow(ctx, `
		SELECT base_level, skill_point_level, final_value
		FROM player_attributes
		WHERE user_id = $1 AND attr_key = $2
	`, userID, attrKey).Scan(&baseLevel, &skillPointLevel, &finalValue)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	switch strings.TrimSpace(stringFromAchievementAny(def.Condition["valueSource"])) {
	case "final_value":
		return int64(finalValue), nil
	case "skill_point_level":
		return skillPointLevel, nil
	default:
		return baseLevel, nil
	}
}

func (s *Store) buildPlayerAchievementList(states map[string]achievementUnlockState) PlayerAchievementList {
	defs := s.achievementDefinitions()
	out := PlayerAchievementList{
		Achievements: make([]PlayerAchievementStateView, 0, len(states)),
	}
	for _, def := range defs {
		if def.Disabled {
			continue
		}
		state, unlocked := states[def.ID]
		if !unlocked {
			continue
		}
		progress := state.ProgressValue
		if progress < 0 {
			progress = 0
		}
		target := achievementTargetCount(def)
		completed := state.CompletedAt != nil
		if completed && progress < target {
			progress = target
		}
		out.Achievements = append(out.Achievements, PlayerAchievementStateView{
			ID:            def.ID,
			Completed:     completed,
			Claimed:       state.ClaimedAt != nil,
			ProgressValue: progress,
			UnlockedAt:    state.UnlockedAt,
			CompletedAt:   state.CompletedAt,
			ClaimedAt:     state.ClaimedAt,
		})
	}
	sort.Slice(out.Achievements, func(i, j int) bool {
		return out.Achievements[i].ID < out.Achievements[j].ID
	})
	return out
}

func (s *Store) achievementCategories() []achievementCategoryDefinition {
	out := make([]achievementCategoryDefinition, 0, len(s.gameData.AchievementCategories))
	for _, row := range s.gameData.AchievementCategories {
		id := strings.TrimSpace(stringFromAchievementAny(row["id"]))
		if id == "" {
			continue
		}
		out = append(out, achievementCategoryDefinition{
			ID:       id,
			NameKey:  strings.TrimSpace(stringFromAchievementAny(row["nameKey"])),
			IconID:   strings.TrimSpace(stringFromAchievementAny(row["iconId"])),
			Order:    int64FromAchievementAny(row["order"], 0),
			Hidden:   boolFromAchievementAny(row["hidden"]),
			Disabled: boolFromAchievementAny(row["disabled"]),
		})
	}
	return out
}

func buildAchievementDefinitions(gameData catalog.Data) []achievementDefinition {
	out := make([]achievementDefinition, 0, len(gameData.Achievements))
	for _, row := range gameData.Achievements {
		out = append(out, achievementDefinitionsFromRow(row)...)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Order == out[j].Order {
			return out[i].ID < out[j].ID
		}
		return out[i].Order < out[j].Order
	})
	return out
}

func (s *Store) achievementDefinitions() []achievementDefinition {
	return s.achievementDefs
}

func achievementDefinitionsFromRow(row map[string]any) []achievementDefinition {
	id := strings.TrimSpace(stringFromAchievementAny(row["id"]))
	if id == "" {
		return nil
	}
	stages := mapSliceFromAchievementAny(row["stages"])
	if len(stages) == 0 {
		return []achievementDefinition{achievementDefinitionFromRow(row, id, stringSliceFromAchievementAny(row["prereqAchievementIds"]), nil)}
	}
	out := make([]achievementDefinition, 0, len(stages))
	basePrereqs := stringSliceFromAchievementAny(row["prereqAchievementIds"])
	for index, stage := range stages {
		stageID := achievementStageID(id, index)
		prereqs := append([]string(nil), basePrereqs...)
		if index > 0 {
			prereqs = append(prereqs, achievementStageID(id, index-1))
		}
		def := achievementDefinitionFromRow(row, stageID, prereqs, stage)
		def.BaseID = id
		def.StageIndex = index + 1
		def.StageProgressMode = normalizeAchievementStageProgressMode(stringFromAchievementAny(row["stageProgressMode"]))
		def.Order = int64FromAchievementAny(stage["order"], def.Order+int64(index))
		out = append(out, def)
	}
	return out
}

func normalizeAchievementStageProgressMode(raw string) string {
	switch strings.TrimSpace(raw) {
	case "cumulative":
		return "cumulative"
	default:
		return "sequential"
	}
}

func achievementDefinitionFromRow(row map[string]any, id string, prereqs []string, stage map[string]any) achievementDefinition {
	source := row
	if stage == nil {
		stage = map[string]any{}
	}
	condition, _ := firstNonNilAchievementAny(stage["condition"], source["condition"]).(map[string]any)
	rewards := mapSliceFromAchievementAny(firstNonNilAchievementAny(stage["rewards"], source["rewards"]))
	def := achievementDefinition{
		ID:                   id,
		CategoryID:           strings.TrimSpace(stringFromAchievementAny(source["categoryId"])),
		NameKey:              strings.TrimSpace(stringFromAchievementAny(firstNonNilAchievementAny(stage["nameKey"], source["nameKey"]))),
		DescKey:              strings.TrimSpace(stringFromAchievementAny(firstNonNilAchievementAny(stage["descKey"], source["descKey"]))),
		Rarity:               firstNonEmptyAchievementString(stringFromAchievementAny(firstNonNilAchievementAny(stage["rarity"], source["rarity"])), "common"),
		IconID:               strings.TrimSpace(stringFromAchievementAny(firstNonNilAchievementAny(stage["iconId"], source["iconId"]))),
		Order:                int64FromAchievementAny(source["order"], 0),
		Hidden:               boolFromAchievementAny(firstNonNilAchievementAny(stage["hidden"], source["hidden"])),
		Disabled:             boolFromAchievementAny(firstNonNilAchievementAny(stage["disabled"], source["disabled"])),
		PrereqAchievementIDs: prereqs,
		Condition:            cloneAchievementMap(condition),
		Rewards:              cloneAchievementMapSlice(rewards),
	}
	def.Hash = achievementDefinitionHash(map[string]any{"base": source, "stage": stage, "id": id})
	return def
}

func achievementStageID(baseID string, index int) string {
	return fmt.Sprintf("%s__stage_%d", strings.TrimSpace(baseID), index+1)
}

func buildAchievementDefinitionsByCounterKey(defs []achievementDefinition) map[string][]achievementDefinition {
	out := make(map[string][]achievementDefinition, len(defs))
	for _, def := range defs {
		key := achievementConditionCounterKey(def)
		if key == "" {
			continue
		}
		out[key] = append(out[key], def)
	}
	return out
}

func (s *Store) achievementDefinitionsByCounterKey() map[string][]achievementDefinition {
	return s.achievementDefsByCounterKey
}

func (s *Store) achievementDefinitionByID(id string) (achievementDefinition, bool) {
	id = strings.TrimSpace(id)
	for _, def := range s.achievementDefinitions() {
		if def.ID == id {
			return def, true
		}
	}
	return achievementDefinition{}, false
}

func achievementPrereqsCompleted(def achievementDefinition, states map[string]achievementUnlockState) bool {
	for _, prereqID := range def.PrereqAchievementIDs {
		state, ok := states[prereqID]
		if !ok || state.CompletedAt == nil {
			return false
		}
	}
	return true
}

func achievementConditionKind(def achievementDefinition) string {
	return strings.TrimSpace(stringFromAchievementAny(def.Condition["kind"]))
}

func achievementTargetCount(def achievementDefinition) int64 {
	if achievementConditionKind(def) == "player_attr_reached" {
		target := int64FromAchievementAny(def.Condition["targetValue"], 0)
		if target <= 0 {
			target = int64FromAchievementAny(def.Condition["value"], 0)
		}
		if target <= 0 {
			target = int64FromAchievementAny(def.Condition["count"], 1)
		}
		if target <= 0 {
			return 1
		}
		return target
	}
	if kind := achievementConditionKind(def); kind == "own_item" || kind == "obtain_item" {
		quantity := int64FromAchievementAny(def.Condition["quantity"], 1)
		if quantity <= 0 {
			return 1
		}
		return quantity
	}
	count := int64FromAchievementAny(def.Condition["count"], 1)
	if count <= 0 {
		return 1
	}
	return count
}

func achievementConditionCounterKey(def achievementDefinition) string {
	kind := achievementConditionKind(def)
	switch kind {
	case "kill_enemy":
		return kind + ":" + strings.TrimSpace(stringFromAchievementAny(def.Condition["enemyId"]))
	case "complete_action":
		return kind + ":" + achievementActionActorType(def) + ":" + strings.TrimSpace(stringFromAchievementAny(def.Condition["actionId"]))
	case "own_item", "obtain_item":
		return kind + ":" + strings.TrimSpace(stringFromAchievementAny(def.Condition["itemId"]))
	case "complete_specific_commission":
		return kind + ":" + strings.TrimSpace(stringFromAchievementAny(def.Condition["commissionId"]))
	case "player_attr_reached":
		return kind + ":" + strings.TrimSpace(stringFromAchievementAny(def.Condition["attrKey"])) + ":" + achievementAttrValueSource(def)
	case "catch_fish":
		return kind + ":" + strings.TrimSpace(stringFromAchievementAny(def.Condition["speciesId"]))
	default:
		return kind
	}
}

func achievementEventPrimaryCounterKey(event AchievementEvent) string {
	if event.Kind == AchievementEventStateReconcile {
		stateKind := strings.TrimSpace(event.StateKind)
		switch stateKind {
		case "own_item":
			itemID := firstNonEmptyAchievementString(event.ItemID, event.SubjectID)
			if itemID == "" {
				return ""
			}
			return "state_reconcile:own_item:" + itemID
		case "player_attr_reached":
			attrKey := firstNonEmptyAchievementString(event.AttrKey, event.SubjectID)
			if attrKey == "" {
				return ""
			}
			return "state_reconcile:player_attr_reached:" + attrKey
		case "own_kitty":
			return "state_reconcile:own_kitty"
		case "catch_fish":
			return "state_reconcile:catch_fish"
		default:
			return "state_reconcile"
		}
	}
	keys := achievementEventCandidateCounterKeys(event)
	if len(keys) == 0 {
		return ""
	}
	return keys[0]
}

func achievementEventID(userID int64, event AchievementEvent, counterKey string) string {
	sourceKey := strings.TrimSpace(event.SourceKey)
	if sourceKey == "" {
		sourceKey = fmt.Sprintf("%s:%s:%d:%d", event.Kind, counterKey, userID, time.Now().UnixNano())
	}
	return fmt.Sprintf("%d:%s:%s", userID, counterKey, sourceKey)
}

func achievementEventCandidateCounterKeys(event AchievementEvent) []string {
	kind := strings.TrimSpace(event.Kind)
	switch kind {
	case "kill_enemy":
		enemyID := firstNonEmptyAchievementString(event.EnemyID, event.SubjectID)
		if enemyID == "" {
			return nil
		}
		return []string{kind + ":" + enemyID}
	case "complete_action":
		actionID := firstNonEmptyAchievementString(event.ActionID, event.SubjectID)
		if actionID == "" {
			return nil
		}
		actorType := normalizeActorType(event.ActorType)
		return []string{
			kind + ":" + actorType + ":" + actionID,
			kind + ":any:" + actionID,
		}
	case "own_item", "obtain_item":
		itemID := firstNonEmptyAchievementString(event.ItemID, event.SubjectID)
		if itemID == "" {
			return nil
		}
		return []string{kind + ":" + itemID}
	case "complete_specific_commission":
		commissionID := firstNonEmptyAchievementString(event.CommissionID, event.SubjectID)
		if commissionID == "" {
			return nil
		}
		return []string{kind + ":" + commissionID}
	case "enchant_count", "enhance_count", "complete_commission", "complete_rumor_commission":
		return []string{kind}
	default:
		return []string{kind}
	}
}

func achievementEventMatches(def achievementDefinition, event AchievementEvent) (string, bool) {
	kind := achievementConditionKind(def)
	if kind != strings.TrimSpace(event.Kind) {
		return "", false
	}
	switch kind {
	case "kill_enemy":
		enemyID := firstNonEmptyAchievementString(event.EnemyID, event.SubjectID)
		if enemyID != strings.TrimSpace(stringFromAchievementAny(def.Condition["enemyId"])) {
			return "", false
		}
	case "complete_action":
		actionID := firstNonEmptyAchievementString(event.ActionID, event.SubjectID)
		if actionID != strings.TrimSpace(stringFromAchievementAny(def.Condition["actionId"])) {
			return "", false
		}
		actorType := achievementActionActorType(def)
		if actorType != "any" && actorType != normalizeActorType(event.ActorType) {
			return "", false
		}
	case "obtain_item", "own_item":
		itemID := firstNonEmptyAchievementString(event.ItemID, event.SubjectID)
		if itemID != strings.TrimSpace(stringFromAchievementAny(def.Condition["itemId"])) {
			return "", false
		}
		if minLevel := int64FromAchievementAny(def.Condition["minEnhanceLevel"], 0); minLevel > 0 && int64(event.EnhanceLevel) < minLevel {
			return "", false
		}
	case "enchant_count":
		if boolFromAchievementAny(def.Condition["successfulOnly"]) && !event.Successful {
			return "", false
		}
	case "enhance_count":
		if boolFromAchievementAny(def.Condition["successfulOnly"]) && !event.Successful {
			return "", false
		}
		itemID := strings.TrimSpace(stringFromAchievementAny(def.Condition["itemId"]))
		if itemID != "" && itemID != strings.TrimSpace(event.ItemID) {
			return "", false
		}
		if minLevel := int64FromAchievementAny(def.Condition["minAfterLevel"], 0); minLevel > 0 && int64(event.EnhanceLevel) < minLevel {
			return "", false
		}
	case "complete_specific_commission":
		commissionID := firstNonEmptyAchievementString(event.CommissionID, event.SubjectID)
		if commissionID != strings.TrimSpace(stringFromAchievementAny(def.Condition["commissionId"])) {
			return "", false
		}
	case "complete_commission", "complete_rumor_commission":
	default:
		return "", false
	}
	return achievementConditionCounterKey(def), true
}

func achievementActionActorType(def achievementDefinition) string {
	switch normalizeActorType(strings.TrimSpace(stringFromAchievementAny(def.Condition["actorType"]))) {
	case ActionActorTypeKitty:
		return ActionActorTypeKitty
	case ActionActorTypePlayer:
		return ActionActorTypePlayer
	default:
		return "any"
	}
}

func achievementAttrValueSource(def achievementDefinition) string {
	switch strings.TrimSpace(stringFromAchievementAny(def.Condition["valueSource"])) {
	case "final_value":
		return "final_value"
	case "skill_point_level":
		return "skill_point_level"
	default:
		return "base_level"
	}
}

func achievementDefinitionHash(row map[string]any) string {
	raw, _ := json.Marshal(row)
	sum := sha1.Sum(raw)
	return hex.EncodeToString(sum[:])
}

func firstNonEmptyAchievementString(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func firstNonNilAchievementAny(values ...any) any {
	for _, value := range values {
		if value != nil {
			return value
		}
	}
	return nil
}

func stringFromAchievementAny(raw any) string {
	switch v := raw.(type) {
	case string:
		return v
	case fmt.Stringer:
		return v.String()
	default:
		return ""
	}
}

func int64FromAchievementAny(raw any, fallback int64) int64 {
	switch v := raw.(type) {
	case int:
		return int64(v)
	case int64:
		return v
	case int32:
		return int64(v)
	case float64:
		return int64(v)
	case float32:
		return int64(v)
	case json.Number:
		if parsed, err := v.Int64(); err == nil {
			return parsed
		}
		return fallback
	default:
		return fallback
	}
}

func boolFromAchievementAny(raw any) bool {
	v, _ := raw.(bool)
	return v
}

func stringSliceFromAchievementAny(raw any) []string {
	rows, ok := raw.([]any)
	if !ok {
		if typed, typedOK := raw.([]string); typedOK {
			out := make([]string, 0, len(typed))
			for _, row := range typed {
				if trimmed := strings.TrimSpace(row); trimmed != "" {
					out = append(out, trimmed)
				}
			}
			return out
		}
		return []string{}
	}
	out := make([]string, 0, len(rows))
	for _, row := range rows {
		if trimmed := strings.TrimSpace(stringFromAchievementAny(row)); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

func mapSliceFromAchievementAny(raw any) []map[string]any {
	rows, ok := raw.([]any)
	if !ok {
		if typed, typedOK := raw.([]map[string]any); typedOK {
			return cloneAchievementMapSlice(typed)
		}
		return []map[string]any{}
	}
	out := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		if node, ok := row.(map[string]any); ok && node != nil {
			out = append(out, cloneAchievementMap(node))
		}
	}
	return out
}

func cloneAchievementMap(raw map[string]any) map[string]any {
	if raw == nil {
		return map[string]any{}
	}
	out := make(map[string]any, len(raw))
	for key, value := range raw {
		out[key] = cloneAchievementAny(value)
	}
	return out
}

func cloneAchievementMapSlice(raw []map[string]any) []map[string]any {
	out := make([]map[string]any, 0, len(raw))
	for _, row := range raw {
		out = append(out, cloneAchievementMap(row))
	}
	return out
}

// CountPlayerAchievementsReadOnly returns the count of completed achievements and total non-disabled definitions.
// Read-only path — no row lock, no reconciliation. Suitable for profile views.
func (s *Store) CountPlayerAchievementsReadOnly(ctx context.Context, userID int64) (completed int64, total int64, err error) {
	defs := s.achievementDefinitions()
	for _, def := range defs {
		if !def.Disabled {
			total++
		}
	}
	if !s.achievementsEnabled() {
		return 0, total, nil
	}
	if err := s.db.QueryRow(ctx, `
		SELECT COUNT(1) FROM player_achievement_states
		WHERE user_id = $1 AND completed_at IS NOT NULL
	`, userID).Scan(&completed); err != nil {
		return 0, total, fmt.Errorf("count achievements: %w", err)
	}
	return completed, total, nil
}

// PlayerAchievementProfileView is a read-only achievement view for profile display.
type PlayerAchievementProfileView struct {
	ID          string         `json:"id"`
	NameKey     string         `json:"nameKey"`
	DescKey     string         `json:"descKey"`
	Rarity      string         `json:"rarity"`
	IconID      string         `json:"iconId"`
	CategoryID  string         `json:"categoryId"`
	Condition   map[string]any `json:"condition"`
	CompletedAt *time.Time     `json:"completedAt,omitempty"`
}

// ListPlayerAchievementsReadOnly returns completed achievements with definition metadata.
// Read-only path — no row lock, no reconciliation.
func (s *Store) ListPlayerAchievementsReadOnly(ctx context.Context, userID int64) ([]PlayerAchievementProfileView, error) {
	defs := s.achievementDefinitions()
	if !s.achievementsEnabled() {
		return nil, nil
	}
	completedStates, err := s.loadCompletedAchievementStates(ctx, userID)
	if err != nil {
		return nil, err
	}
	out := make([]PlayerAchievementProfileView, 0, len(completedStates))
	for _, def := range defs {
		if def.Disabled {
			continue
		}
		state, ok := completedStates[def.ID]
		if !ok {
			continue
		}
		out = append(out, PlayerAchievementProfileView{
			ID:          def.ID,
			NameKey:     def.NameKey,
			DescKey:     def.DescKey,
			Rarity:      def.Rarity,
			IconID:      def.IconID,
			CategoryID:  def.CategoryID,
			Condition:   def.Condition,
			CompletedAt: state.CompletedAt,
		})
	}
	return out, nil
}

func (s *Store) loadCompletedAchievementStates(ctx context.Context, userID int64) (map[string]achievementUnlockState, error) {
	rows, err := s.db.Query(ctx, `
		SELECT achievement_id, completed_at
		FROM player_achievement_states
		WHERE user_id = $1 AND completed_at IS NOT NULL
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("load completed achievements: %w", err)
	}
	defer rows.Close()
	out := map[string]achievementUnlockState{}
	for rows.Next() {
		var id string
		var state achievementUnlockState
		if err := rows.Scan(&id, &state.CompletedAt); err != nil {
			return nil, err
		}
		out[id] = state
	}
	return out, rows.Err()
}

func cloneAchievementAny(raw any) any {
	switch v := raw.(type) {
	case map[string]any:
		return cloneAchievementMap(v)
	case []any:
		out := make([]any, 0, len(v))
		for _, row := range v {
			out = append(out, cloneAchievementAny(row))
		}
		return out
	case []map[string]any:
		out := make([]any, 0, len(v))
		for _, row := range v {
			out = append(out, cloneAchievementMap(row))
		}
		return out
	default:
		return raw
	}
}
