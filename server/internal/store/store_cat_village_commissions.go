package store

import (
	"context"
	"encoding/json"
	"errors"
	"math/rand"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"mysecondwebgame/server/internal/catalog"
	appconfig "mysecondwebgame/server/internal/config"
)

type openTaskRow struct {
	commissionID string
}

func (s *Store) ListCatVillageCommissions(ctx context.Context, userID int64, now time.Time, templates []catalog.CatVillageCommission, cfg appconfig.CatVillageCommissionSettings) ([]CatVillageTask, time.Time, CatVillageReputationState, error) {
	totalRep, err := s.GetCatVillageReputation(ctx, userID)
	if err != nil {
		return nil, time.Time{}, CatVillageReputationState{}, err
	}
	repState := ComputeRepLevel(totalRep, s.config.CatVillageRepLevels)
	if err := s.refreshCatVillageCommissions(ctx, userID, now, templates, cfg, repState.Level); err != nil {
		return nil, time.Time{}, CatVillageReputationState{}, err
	}

	var nextRefreshAt time.Time
	if err := s.db.QueryRow(ctx, `
		SELECT next_refresh_at
		FROM player_cat_village_task_state
		WHERE user_id = $1
	`, userID).Scan(&nextRefreshAt); err != nil {
		return nil, time.Time{}, CatVillageReputationState{}, err
	}

	rows, err := s.db.Query(ctx, `
		SELECT id, commission_id, costs_jsonb, reward_items, reputation_grant, refreshed, status, created_at, completed_at, not_refreshable, not_deletable
		FROM player_cat_village_tasks
		WHERE user_id = $1 AND status = 'open'
		ORDER BY id DESC
	`, userID)
	if err != nil {
		return nil, time.Time{}, CatVillageReputationState{}, err
	}

	templateByID := make(map[string]catalog.CatVillageCommission, len(templates))
	for _, t := range templates {
		templateByID[t.ID] = t
	}

	out := make([]CatVillageTask, 0, cfg.MaxActiveTasks)
	for rows.Next() {
		var task CatVillageTask
		var costsRaw []byte
		var rewardItemsRaw []byte
		if err := rows.Scan(
			&task.ID,
			&task.CommissionID,
			&costsRaw,
			&rewardItemsRaw,
			&task.ReputationGrant,
			&task.Refreshed,
			&task.Status,
			&task.CreatedAt,
			&task.CompletedAt,
			&task.NotRefreshable,
			&task.NotDeletable,
		); err != nil {
			rows.Close()
			return nil, time.Time{}, CatVillageReputationState{}, err
		}
		task.Costs = decodeCosts(costsRaw)
		task.RewardItems = decodeRewardItems(rewardItemsRaw)
		if tmpl, ok := templateByID[task.CommissionID]; ok {
			task.IsRumor = tmpl.IsRumor
			task.IsBattle = tmpl.IsBattle
			task.RumorBadge = tmpl.RumorBadge
			task.PityThreshold = tmpl.PityThreshold
		}
		out = append(out, task)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, time.Time{}, CatVillageReputationState{}, err
	}
	rows.Close()

	// Load pity counters after task rows are closed so this request never holds
	// two pool connections at once for small read-only result sets.
	pityCounters := map[string]int64{}
	pRows, pErr := s.db.Query(ctx, `
		SELECT commission_id, counter FROM player_rumor_pity WHERE user_id = $1
	`, userID)
	if pErr != nil {
		return nil, time.Time{}, CatVillageReputationState{}, pErr
	}
	for pRows.Next() {
		var cid string
		var cnt int64
		if scanErr := pRows.Scan(&cid, &cnt); scanErr != nil {
			pRows.Close()
			return nil, time.Time{}, CatVillageReputationState{}, scanErr
		}
		pityCounters[cid] = cnt
	}
	if err := pRows.Err(); err != nil {
		pRows.Close()
		return nil, time.Time{}, CatVillageReputationState{}, err
	}
	pRows.Close()

	// Also expose pity progress for rumor commissions NOT yet active but eligible
	// (so frontend can show pending pity info) — not needed for active tasks,
	// but include counter in active rumors for the "how close" display
	for i := range out {
		if out[i].IsRumor {
			out[i].PityCounter = pityCounters[out[i].CommissionID]
		}
	}

	return out, nextRefreshAt, repState, nil
}

func (s *Store) loadRumorPityCounters(ctx context.Context, tx pgx.Tx, userID int64) (map[string]int64, error) {
	rows, err := tx.Query(ctx, `
		SELECT commission_id, counter FROM player_rumor_pity WHERE user_id = $1
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make(map[string]int64)
	for rows.Next() {
		var id string
		var counter int64
		if err := rows.Scan(&id, &counter); err != nil {
			return nil, err
		}
		out[id] = counter
	}
	return out, rows.Err()
}

func (s *Store) upsertRumorPityCounter(ctx context.Context, tx pgx.Tx, userID int64, commissionID string, counter int64) error {
	if counter < 0 {
		counter = 0
	}
	_, err := tx.Exec(ctx, `
		INSERT INTO player_rumor_pity(user_id, commission_id, counter, updated_at)
		VALUES($1, $2, $3, now())
		ON CONFLICT (user_id, commission_id) DO UPDATE
		SET counter = $3, updated_at = now()
	`, userID, commissionID, counter)
	return err
}

func (s *Store) loadCompletedRumorIDs(ctx context.Context, tx pgx.Tx, userID int64) (map[string]struct{}, error) {
	rows, err := tx.Query(ctx, `
		SELECT commission_id FROM player_rumor_completion WHERE user_id = $1
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make(map[string]struct{})
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out[id] = struct{}{}
	}
	return out, rows.Err()
}

func (s *Store) insertCommissionTask(ctx context.Context, tx pgx.Tx, userID int64, tmpl catalog.CatVillageCommission, rng *rand.Rand) error {
	costs := rollItemRangeMap(pickGroup(tmpl.CostOptions, tmpl.Costs, rng), rng)
	costsJSON, err := encodeCosts(costs)
	if err != nil {
		return err
	}
	rewardItems := rollItemRangeMap(pickGroup(tmpl.RewardOptions, tmpl.RewardPool, rng), rng)
	rewardJSON, err := encodeRewardItems(rewardItems)
	if err != nil {
		return err
	}
	repGrant := tmpl.ReputationMin
	if tmpl.ReputationMax > tmpl.ReputationMin {
		repGrant += rng.Int63n(tmpl.ReputationMax - tmpl.ReputationMin + 1)
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO player_cat_village_tasks(
			user_id, commission_id, costs_jsonb,
			reward_items, reputation_grant, status, not_refreshable, not_deletable, created_at
		)
		VALUES($1, $2, $3::jsonb, $4::jsonb, $5, 'open', $6, $7, now())
	`, userID, tmpl.ID, string(costsJSON), string(rewardJSON), repGrant, tmpl.NotRefreshable, tmpl.NotDeletable)
	return err
}

func (s *Store) refreshCatVillageCommissions(ctx context.Context, userID int64, now time.Time, templates []catalog.CatVillageCommission, cfg appconfig.CatVillageCommissionSettings, playerRepLevel int) error {
	maxLife := cfg.MaxActiveTasks
	if maxLife <= 0 {
		maxLife = 24
	}
	maxBattle := cfg.MaxBattleTasks
	if maxBattle <= 0 {
		maxBattle = 10
	}
	batchSize := cfg.RefreshBatchSize
	if batchSize <= 0 {
		batchSize = 2
	}
	intervalHours := cfg.RefreshIntervalHours
	if intervalHours <= 0 {
		intervalHours = 4
	}
	rumorChance := cfg.RumorChancePerBatch
	if rumorChance <= 0 {
		rumorChance = 0.15
	}
	maxRumors := cfg.MaxRumorsActive
	if maxRumors <= 0 {
		maxRumors = 1
	}
	refreshCadence := time.Duration(intervalHours) * time.Hour

	return s.withTx(ctx, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `
			INSERT INTO player_cat_village_task_state(user_id, next_refresh_at, updated_at)
			VALUES($1, now(), now())
			ON CONFLICT (user_id) DO NOTHING
		`, userID); err != nil {
			return err
		}

		var nextRefreshAt time.Time
		if err := tx.QueryRow(ctx, `
			SELECT next_refresh_at
			FROM player_cat_village_task_state
			WHERE user_id = $1
			FOR UPDATE
		`, userID).Scan(&nextRefreshAt); err != nil {
			return err
		}

		if now.Before(nextRefreshAt) {
			return nil
		}

		// Build templateByID for classification
		templateByID := make(map[string]catalog.CatVillageCommission, len(templates))
		for _, t := range templates {
			templateByID[t.ID] = t
		}

		// Query open tasks to classify into rumor/battle/life counts
		openRows, err := tx.Query(ctx, `
			SELECT commission_id
			FROM player_cat_village_tasks
			WHERE user_id = $1 AND status = 'open'
		`, userID)
		if err != nil {
			return err
		}
		var openTasks []openTaskRow
		for openRows.Next() {
			var row openTaskRow
			if err := openRows.Scan(&row.commissionID); err != nil {
				openRows.Close()
				return err
			}
			openTasks = append(openTasks, row)
		}
		openRows.Close()
		if err := openRows.Err(); err != nil {
			return err
		}

		rumorCountFromOpen, activeBattleCount, activeLifeCount := classifyOpenCatVillageTasks(openTasks, templateByID)

		remainBattle := maxBattle - activeBattleCount
		if remainBattle < 0 {
			remainBattle = 0
		}
		remainLife := maxLife - activeLifeCount
		if remainLife < 0 {
			remainLife = 0
		}
		remaining := remainBattle + remainLife

		if remaining > 0 {
			elapsed := now.Sub(nextRefreshAt)
			cycles := int64(1)
			if elapsed > 0 {
				cycles += int64(elapsed / refreshCadence)
			}
			toAdd := int(cycles * int64(batchSize))
			if toAdd > remaining {
				toAdd = remaining
			}
			activeRumorCount := rumorCountFromOpen

			completedRumors, err := s.loadCompletedRumorIDs(ctx, tx, userID)
			if err != nil {
				return err
			}

			rng := rand.New(rand.NewSource(now.UnixNano()))

			// Load pity counters for all rumor commissions
			pityCounters, err := s.loadRumorPityCounters(ctx, tx, userID)
			if err != nil {
				return err
			}

			// Build eligible rumor list (not completed, prereqs met, correct rep level)
			eligibleRumors := filterRumorCandidates(templates, playerRepLevel, completedRumors, openTasks)

			// Compute updated pity counters and find force-spawn candidates.
			// pityIncrement = total auto-generated slots this batch (before any rumor takes one).
			// Once a rumor hits its threshold the counter is capped — it won't overflow.
			type pityEntry struct {
				tmpl       catalog.CatVillageCommission
				newCounter int64
			}
			var forceCandidates []pityEntry
			updatedPity := make(map[string]int64, len(eligibleRumors))

			for _, tmpl := range eligibleRumors {
				if tmpl.PityThreshold <= 0 {
					continue
				}
				threshold := int64(tmpl.PityThreshold)
				current := pityCounters[tmpl.ID]
				increment := int64(toAdd)
				var newCounter int64
				if current >= threshold {
					newCounter = threshold // already at cap, waiting for a slot
				} else {
					newCounter = current + increment
					if newCounter > threshold {
						newCounter = threshold
					}
				}
				updatedPity[tmpl.ID] = newCounter
				if newCounter >= threshold {
					forceCandidates = append(forceCandidates, pityEntry{tmpl, newCounter})
				}
			}

			// Sort force candidates: most overdue first (currently all at threshold, so order is stable;
			// sort by ID for determinism when multiple hit simultaneously)
			sort.Slice(forceCandidates, func(i, j int) bool {
				return forceCandidates[i].tmpl.ID < forceCandidates[j].tmpl.ID
			})

			// Phase 1: force-spawn pity candidates (consume slots from toAdd)
			rumorSpawnedThisBatch := false
			spawnedMutexGroups := map[string]struct{}{}
			for _, candidate := range forceCandidates {
				if toAdd <= 0 {
					break // no more slots; counter stays at threshold, triggers next cycle
				}
				if activeRumorCount >= maxRumors {
					break // active rumor slot full; stays at threshold
				}
				mutexGroup := strings.TrimSpace(candidate.tmpl.RumorMutexGroup)
				if mutexGroup != "" {
					if _, spawned := spawnedMutexGroups[mutexGroup]; spawned {
						continue
					}
				}
				if err := s.insertCommissionTask(ctx, tx, userID, candidate.tmpl, rng); err != nil {
					return err
				}
				toAdd--
				activeRumorCount++
				if mutexGroup != "" {
					spawnedMutexGroups[mutexGroup] = struct{}{}
				}
				updatedPity[candidate.tmpl.ID] = 0 // reset after spawn
				rumorSpawnedThisBatch = true
			}

			// Phase 2: random rumor attempt (only if no force-spawn this batch and slots remain)
			if !rumorSpawnedThisBatch && toAdd > 0 && activeRumorCount < maxRumors && rng.Float64() < rumorChance {
				noPityRumors := filterNoPityOrUnreachedPity(eligibleRumors, updatedPity)
				if rumorTmpl, ok := pickWeightedCommission(noPityRumors, rng, playerRepLevel); ok {
					if err := s.insertCommissionTask(ctx, tx, userID, rumorTmpl, rng); err != nil {
						return err
					}
					toAdd--
					activeRumorCount++
					updatedPity[rumorTmpl.ID] = 0 // reset pity on random spawn too
					rumorSpawnedThisBatch = true
				}
			}

			// Phase 3: split into battle and life commissions
			battleTemplates := filterBattleCandidates(templates)
			lifeTemplates := filterLifeCandidates(templates)

			plans := planCatVillageRefreshCategories(
				toAdd,
				remainLife,
				remainBattle,
				hasEligibleCatVillageCommission(lifeTemplates, playerRepLevel),
				hasEligibleCatVillageCommission(battleTemplates, playerRepLevel),
			)
			for _, plan := range plans {
				candidates := lifeTemplates
				if plan == catVillageRefreshBattle {
					candidates = battleTemplates
				}
				tmpl, ok := pickWeightedCommission(candidates, rng, playerRepLevel)
				if !ok {
					continue
				}
				if err := s.insertCommissionTask(ctx, tx, userID, tmpl, rng); err != nil {
					return err
				}
			}

			// Persist updated pity counters
			for commID, counter := range updatedPity {
				if err := s.upsertRumorPityCounter(ctx, tx, userID, commID, counter); err != nil {
					return err
				}
			}

			nextRefreshAt = nextRefreshAt.Add(time.Duration(cycles) * refreshCadence)
			if _, err := tx.Exec(ctx, `
				UPDATE player_cat_village_task_state
				SET next_refresh_at = $1, updated_at = now()
				WHERE user_id = $2
			`, nextRefreshAt, userID); err != nil {
				return err
			}
		}
		return nil
	})
}

func (s *Store) SubmitCatVillageCommission(ctx context.Context, userID, taskID int64, commissionByID map[string]catalog.CatVillageCommission) error {
	return s.withTx(ctx, func(tx pgx.Tx) error {
		var status string
		var commissionID string
		var costsRaw []byte
		var rewardItemsRaw []byte
		var reputationGrant int64
		err := tx.QueryRow(ctx, `
			SELECT status, commission_id, costs_jsonb, reward_items, reputation_grant
			FROM player_cat_village_tasks
			WHERE id = $1 AND user_id = $2
			FOR UPDATE
		`, taskID, userID).Scan(&status, &commissionID, &costsRaw, &rewardItemsRaw, &reputationGrant)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrCatVillageTaskNotFound
		}
		if err != nil {
			return err
		}
		if status != "open" {
			return ErrCatVillageTaskNotOpen
		}

		costs := decodeCosts(costsRaw)
		for _, cost := range costs {
			if cost.Quantity <= 0 {
				continue
			}
			owned, err := s.ConsumableQuantityByTemplateTx(ctx, tx, userID, cost.ItemID)
			if err != nil {
				return err
			}
			if owned < cost.Quantity {
				return ErrInsufficientItem
			}
		}

		for _, cost := range costs {
			if cost.Quantity <= 0 || cost.ItemID == "" {
				continue
			}
			if err = s.LedgerConsumeTemplateTx(ctx, tx, userID, cost.ItemID, cost.Quantity, InventoryLedgerMeta{
				Category:      InventoryAuditCategoryCatVillage,
				SourceSurface: "inventory",
				TargetSurface: "cat_village_commission",
				Details: map[string]any{
					"taskId": taskID,
					"type":   "submit_cost",
				},
			}); err != nil {
				return err
			}
		}

		for _, reward := range decodeRewardItems(rewardItemsRaw) {
			if reward.Count <= 0 || reward.ItemID == "" {
				continue
			}
			if err = s.LedgerGrantTemplateTx(ctx, tx, userID, reward.ItemID, reward.Count, InventoryLedgerMeta{
				Category:      InventoryAuditCategoryCatVillage,
				SourceSurface: "cat_village_commission",
				TargetSurface: "inventory",
				Details: map[string]any{
					"taskId": taskID,
					"type":   "submit_reward",
				},
			}); err != nil {
				return err
			}
		}

		if reputationGrant > 0 {
			if _, err = tx.Exec(ctx, `
				INSERT INTO player_cat_village_reputation(user_id, reputation, updated_at)
				VALUES($1, $2, now())
				ON CONFLICT (user_id) DO UPDATE
				SET reputation = player_cat_village_reputation.reputation + EXCLUDED.reputation,
				    updated_at = now()
			`, userID, reputationGrant); err != nil {
				return err
			}
		}

		if _, err = tx.Exec(ctx, `
			UPDATE player_cat_village_tasks
			SET status = 'completed', completed_at = now()
			WHERE id = $1 AND user_id = $2
		`, taskID, userID); err != nil {
			return err
		}

		events := []AchievementEvent{
			{
				Kind:         AchievementEventCompleteCommission,
				Count:        1,
				CommissionID: commissionID,
				SubjectID:    commissionID,
				SourceKey:    "cat_village_commission:" + strconv.FormatInt(taskID, 10) + ":any",
			},
			{
				Kind:         AchievementEventCompleteSpecificCommission,
				Count:        1,
				CommissionID: commissionID,
				SubjectID:    commissionID,
				SourceKey:    "cat_village_commission:" + strconv.FormatInt(taskID, 10) + ":specific",
			},
		}
		if tmpl, ok := commissionByID[commissionID]; ok && tmpl.IsRumor {
			events = append(events, AchievementEvent{
				Kind:         AchievementEventCompleteRumorCommission,
				Count:        1,
				CommissionID: commissionID,
				SubjectID:    commissionID,
				SourceKey:    "cat_village_commission:" + strconv.FormatInt(taskID, 10) + ":rumor",
			})
			if _, err = tx.Exec(ctx, `
				INSERT INTO player_rumor_completion(user_id, commission_id, completed_at)
				VALUES($1, $2, now())
				ON CONFLICT DO NOTHING
			`, userID, commissionID); err != nil {
				return err
			}
			// Clear specified completion records and reset their pity counters
			for _, clearID := range tmpl.ClearCompletionIds {
				if clearID == "" {
					continue
				}
				if _, err = tx.Exec(ctx, `
					DELETE FROM player_rumor_completion
					WHERE user_id = $1 AND commission_id = $2
				`, userID, clearID); err != nil {
					return err
				}
				if _, err = tx.Exec(ctx, `
					UPDATE player_rumor_pity SET counter = 0, updated_at = now()
					WHERE user_id = $1 AND commission_id = $2
				`, userID, clearID); err != nil {
					return err
				}
			}
		}
		if err := s.ApplyAchievementEventsTx(ctx, tx, userID, events); err != nil {
			return err
		}
		return nil
	})
}

func (s *Store) ForceGenerateCatVillageCommissions(ctx context.Context, userID int64, count int, templates []catalog.CatVillageCommission) (int, error) {
	totalRep, err := s.GetCatVillageReputation(ctx, userID)
	if err != nil {
		return 0, err
	}
	repState := ComputeRepLevel(totalRep, s.config.CatVillageRepLevels)
	if count <= 0 {
		count = 3
	}
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	generated := 0
	err = s.withTx(ctx, func(tx pgx.Tx) error {
		for i := 0; i < count; i++ {
			tmpl, ok := pickWeightedCommission(templates, rng, repState.Level)
			if !ok {
				break
			}
			if err := s.insertCommissionTask(ctx, tx, userID, tmpl, rng); err != nil {
				return err
			}
			generated++
		}
		return nil
	})
	return generated, err
}

func (s *Store) ForceInsertSpecificCommission(ctx context.Context, userID int64, commissionID string, templates []catalog.CatVillageCommission) error {
	var tmpl catalog.CatVillageCommission
	found := false
	for _, t := range templates {
		if t.ID == commissionID {
			tmpl = t
			found = true
			break
		}
	}
	if !found {
		return errors.New("commission not found: " + commissionID)
	}
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	return s.withTx(ctx, func(tx pgx.Tx) error {
		return s.insertCommissionTask(ctx, tx, userID, tmpl, rng)
	})
}

// GetCatVillageReputationState returns the computed reputation level state for a user.
func (s *Store) GetCatVillageReputationState(ctx context.Context, userID int64) (CatVillageReputationState, error) {
	totalRep, err := s.GetCatVillageReputation(ctx, userID)
	if err != nil {
		return CatVillageReputationState{}, err
	}
	return ComputeRepLevel(totalRep, s.config.CatVillageRepLevels), nil
}

func (s *Store) GetCatVillageReputation(ctx context.Context, userID int64) (int64, error) {
	var rep int64
	err := s.db.QueryRow(ctx, `
		SELECT reputation FROM player_cat_village_reputation WHERE user_id = $1
	`, userID).Scan(&rep)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, nil
	}
	return rep, err
}

func (s *Store) DeleteCatVillageCommission(ctx context.Context, userID, taskID int64) error {
	var notDeletable bool
	var status string
	err := s.db.QueryRow(ctx, `
		SELECT status, not_deletable FROM player_cat_village_tasks WHERE id = $1 AND user_id = $2
	`, taskID, userID).Scan(&status, &notDeletable)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrCatVillageTaskNotFound
	}
	if err != nil {
		return err
	}
	if status != "open" {
		return ErrCatVillageTaskNotFound
	}
	if notDeletable {
		return ErrCatVillageTaskNotDeletable
	}
	tag, err := s.db.Exec(ctx, `
		DELETE FROM player_cat_village_tasks
		WHERE id = $1 AND user_id = $2 AND status = 'open'
	`, taskID, userID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrCatVillageTaskNotFound
	}
	return nil
}

func (s *Store) RefreshCatVillageCommission(ctx context.Context, userID, taskID int64, templates []catalog.CatVillageCommission, refreshCost map[string]int64) (*CatVillageTask, error) {
	totalRep, err := s.GetCatVillageReputation(ctx, userID)
	if err != nil {
		return nil, err
	}
	repState := ComputeRepLevel(totalRep, s.config.CatVillageRepLevels)

	var newTask *CatVillageTask
	err = s.withTx(ctx, func(tx pgx.Tx) error {
		var commissionID string
		var refreshed bool
		var notRefreshable bool
		var status string
		scanErr := tx.QueryRow(ctx, `
			SELECT commission_id, status, refreshed, not_refreshable FROM player_cat_village_tasks
			WHERE id = $1 AND user_id = $2 FOR UPDATE
		`, taskID, userID).Scan(&commissionID, &status, &refreshed, &notRefreshable)
		if errors.Is(scanErr, pgx.ErrNoRows) {
			return ErrCatVillageTaskNotFound
		}
		if scanErr != nil {
			return scanErr
		}
		if status != "open" {
			return ErrCatVillageTaskNotOpen
		}
		if notRefreshable {
			return ErrCatVillageTaskNotRefreshable
		}
		if refreshed {
			return ErrCatVillageTaskAlreadyRefreshed
		}

		for itemID, qty := range refreshCost {
			if qty <= 0 || itemID == "" {
				continue
			}
			owned, err := s.ConsumableQuantityByTemplateTx(ctx, tx, userID, itemID)
			if err != nil {
				return err
			}
			if owned < qty {
				return ErrInsufficientItem
			}
		}

		for itemID, qty := range refreshCost {
			if qty <= 0 || itemID == "" {
				continue
			}
			if err := s.LedgerConsumeTemplateTx(ctx, tx, userID, itemID, qty, InventoryLedgerMeta{
				Category:      InventoryAuditCategoryCatVillage,
				SourceSurface: "inventory",
				TargetSurface: "cat_village_commission",
				Details:       map[string]any{"taskId": taskID, "type": "refresh_cost"},
			}); err != nil {
				return err
			}
		}

		if _, err := tx.Exec(ctx, `
			DELETE FROM player_cat_village_tasks WHERE id = $1 AND user_id = $2
		`, taskID, userID); err != nil {
			return err
		}

		rng := rand.New(rand.NewSource(time.Now().UnixNano()))
		candidates, ok := refreshCandidatesForExistingCommission(templates, commissionID)
		if !ok {
			return ErrCatVillageTaskNotFound
		}
		tmpl, ok := pickWeightedCommission(candidates, rng, repState.Level)
		if !ok {
			return ErrCatVillageTaskNotFound
		}
		costs := rollItemRangeMap(pickGroup(tmpl.CostOptions, tmpl.Costs, rng), rng)
		costsJSON, err := encodeCosts(costs)
		if err != nil {
			return err
		}
		rewardItems := rollItemRangeMap(pickGroup(tmpl.RewardOptions, tmpl.RewardPool, rng), rng)
		rewardJSON, err := encodeRewardItems(rewardItems)
		if err != nil {
			return err
		}
		repGrant := tmpl.ReputationMin
		if tmpl.ReputationMax > tmpl.ReputationMin {
			repGrant += rng.Int63n(tmpl.ReputationMax - tmpl.ReputationMin + 1)
		}

		var newID int64
		if err := tx.QueryRow(ctx, `
			INSERT INTO player_cat_village_tasks(
				user_id, commission_id, costs_jsonb,
				reward_items, reputation_grant, refreshed, status, not_refreshable, not_deletable, created_at
			)
			VALUES($1, $2, $3::jsonb, $4::jsonb, $5, TRUE, 'open', $6, $7, now())
			RETURNING id
		`, userID, tmpl.ID, string(costsJSON), string(rewardJSON), repGrant, tmpl.NotRefreshable, tmpl.NotDeletable).Scan(&newID); err != nil {
			return err
		}

		newTask = &CatVillageTask{
			ID:              newID,
			CommissionID:    tmpl.ID,
			Costs:           decodeCosts(costsJSON),
			RewardItems:     decodeRewardItems(rewardJSON),
			ReputationGrant: repGrant,
			Refreshed:       true,
			Status:          "open",
		}
		applyCatVillageTaskTemplateFields(newTask, tmpl)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return newTask, nil
}

func refreshCandidatesForExistingCommission(templates []catalog.CatVillageCommission, commissionID string) ([]catalog.CatVillageCommission, bool) {
	commissionID = strings.TrimSpace(commissionID)
	if commissionID == "" {
		return nil, false
	}
	for _, tmpl := range templates {
		if strings.TrimSpace(tmpl.ID) != commissionID {
			continue
		}
		if tmpl.IsRumor {
			return nil, false
		}
		if tmpl.IsBattle {
			return filterBattleCandidates(templates), true
		}
		return filterLifeCandidates(templates), true
	}
	return nil, false
}

func applyCatVillageTaskTemplateFields(task *CatVillageTask, tmpl catalog.CatVillageCommission) {
	if task == nil {
		return
	}
	task.IsRumor = tmpl.IsRumor
	task.IsBattle = tmpl.IsBattle
	task.RumorBadge = tmpl.RumorBadge
	task.NotRefreshable = tmpl.NotRefreshable
	task.NotDeletable = tmpl.NotDeletable
	task.PityThreshold = tmpl.PityThreshold
}

func ComputeRepLevel(totalRep int64, levels []appconfig.CatVillageRepLevel) CatVillageReputationState {
	sorted := make([]appconfig.CatVillageRepLevel, len(levels))
	copy(sorted, levels)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Level < sorted[j].Level })

	remaining := totalRep
	currentLevel := 0
	for _, lvl := range sorted {
		if lvl.XpRequired <= 0 || remaining < lvl.XpRequired {
			break
		}
		remaining -= lvl.XpRequired
		currentLevel = lvl.Level
	}

	nextLevelXp := int64(0)
	if currentLevel < len(sorted) {
		for _, lvl := range sorted {
			if lvl.Level == currentLevel+1 {
				nextLevelXp = lvl.XpRequired
				break
			}
		}
	}

	return CatVillageReputationState{
		TotalReputation: totalRep,
		Level:           currentLevel,
		LevelReputation: remaining,
		NextLevelXp:     nextLevelXp,
	}
}

func pickGroup(options []map[string]catalog.CatVillageItemRange, fallback map[string]catalog.CatVillageItemRange, rng *rand.Rand) map[string]catalog.CatVillageItemRange {
	if len(options) > 0 {
		return options[rng.Intn(len(options))]
	}
	return fallback
}

// filterNoPityOrUnreachedPity returns rumors that either have no pity threshold
// or have not yet reached it (so they're fair game for the random roll).
func filterNoPityOrUnreachedPity(rumors []catalog.CatVillageCommission, updatedPity map[string]int64) []catalog.CatVillageCommission {
	out := make([]catalog.CatVillageCommission, 0, len(rumors))
	for _, t := range rumors {
		if t.PityThreshold <= 0 {
			out = append(out, t)
			continue
		}
		if updatedPity[t.ID] < int64(t.PityThreshold) {
			out = append(out, t)
		}
	}
	return out
}

func filterBattleCandidates(templates []catalog.CatVillageCommission) []catalog.CatVillageCommission {
	out := make([]catalog.CatVillageCommission, 0, len(templates))
	for _, t := range templates {
		if !t.IsRumor && t.IsBattle {
			out = append(out, t)
		}
	}
	return out
}

func filterLifeCandidates(templates []catalog.CatVillageCommission) []catalog.CatVillageCommission {
	out := make([]catalog.CatVillageCommission, 0, len(templates))
	for _, t := range templates {
		if !t.IsRumor && !t.IsBattle {
			out = append(out, t)
		}
	}
	return out
}

type catVillageRefreshCategory string

const (
	catVillageRefreshLife   catVillageRefreshCategory = "life"
	catVillageRefreshBattle catVillageRefreshCategory = "battle"
)

func planCatVillageRefreshCategories(toAdd, lifeSlots, battleSlots int, lifeAvailable, battleAvailable bool) []catVillageRefreshCategory {
	if toAdd <= 0 {
		return nil
	}
	out := make([]catVillageRefreshCategory, 0, toAdd)
	for i := 0; i < toAdd; i++ {
		preferLife := i%6 < 4
		if preferLife {
			if lifeAvailable && lifeSlots > 0 {
				lifeSlots--
				out = append(out, catVillageRefreshLife)
				continue
			}
			if battleAvailable && battleSlots > 0 {
				battleSlots--
				out = append(out, catVillageRefreshBattle)
				continue
			}
			break
		}
		if battleAvailable && battleSlots > 0 {
			battleSlots--
			out = append(out, catVillageRefreshBattle)
			continue
		}
		if lifeAvailable && lifeSlots > 0 {
			lifeSlots--
			out = append(out, catVillageRefreshLife)
			continue
		}
		break
	}
	return out
}

func hasEligibleCatVillageCommission(templates []catalog.CatVillageCommission, playerRepLevel int) bool {
	for _, t := range templates {
		if t.Disabled || t.Weight <= 0 || t.MinRepLevel > playerRepLevel {
			continue
		}
		return true
	}
	return false
}

func classifyOpenCatVillageTasks(openTasks []openTaskRow, templateByID map[string]catalog.CatVillageCommission) (rumorCount, battleCount, lifeCount int) {
	for _, row := range openTasks {
		if tmpl, ok := templateByID[row.commissionID]; ok {
			if tmpl.IsRumor {
				rumorCount++
				continue
			}
			if tmpl.IsBattle {
				battleCount++
				continue
			}
		}
		lifeCount++
	}
	return rumorCount, battleCount, lifeCount
}

func filterRumorCandidates(templates []catalog.CatVillageCommission, playerRepLevel int, completedIDs map[string]struct{}, openTasks []openTaskRow) []catalog.CatVillageCommission {
	out := make([]catalog.CatVillageCommission, 0, len(templates))
	mutexLocks := rumorMutexLocks(templates, completedIDs, openTasks)
	for _, t := range templates {
		if !t.IsRumor || t.Disabled {
			continue
		}
		if _, done := completedIDs[t.ID]; done {
			continue
		}
		allPrereqsDone := true
		for _, prereqID := range t.PrereqRumorIDs {
			if _, done := completedIDs[prereqID]; !done {
				allPrereqsDone = false
				break
			}
		}
		if !allPrereqsDone {
			continue
		}
		if playerRepLevel < t.MinRepLevel {
			continue
		}
		if !rumorMutexAllows(t, mutexLocks, completedIDs) {
			continue
		}
		out = append(out, t)
	}
	return out
}

func rumorMutexLocks(templates []catalog.CatVillageCommission, completedIDs map[string]struct{}, openTasks []openTaskRow) map[string]struct{} {
	templateByID := make(map[string]catalog.CatVillageCommission, len(templates))
	for _, t := range templates {
		templateByID[t.ID] = t
	}
	locks := map[string]struct{}{}
	for _, row := range openTasks {
		tmpl, ok := templateByID[row.commissionID]
		if !ok {
			continue
		}
		if !tmpl.IsRumor {
			continue
		}
		group := strings.TrimSpace(tmpl.RumorMutexGroup)
		if group == "" {
			continue
		}
		locks[group] = struct{}{}
	}
	for completedID := range completedIDs {
		tmpl, ok := templateByID[completedID]
		if !ok {
			continue
		}
		group := strings.TrimSpace(tmpl.RumorMutexGroup)
		if group == "" {
			continue
		}
		locks[group] = struct{}{}
	}
	return locks
}

func rumorMutexAllows(t catalog.CatVillageCommission, locks map[string]struct{}, completedIDs map[string]struct{}) bool {
	group := strings.TrimSpace(t.RumorMutexGroup)
	if group == "" {
		return true
	}
	if _, locked := locks[group]; !locked {
		return true
	}
	for _, prereqID := range t.PrereqRumorIDs {
		if _, done := completedIDs[strings.TrimSpace(prereqID)]; done {
			return true
		}
	}
	return false
}

func pickWeightedCommission(templates []catalog.CatVillageCommission, rng *rand.Rand, playerRepLevel int) (catalog.CatVillageCommission, bool) {
	active := make([]catalog.CatVillageCommission, 0, len(templates))
	totalWeight := 0
	for _, t := range templates {
		hasCosts := len(t.Costs) > 0 || len(t.CostOptions) > 0
		if !t.Disabled && strings.TrimSpace(t.ID) != "" && hasCosts && playerRepLevel >= t.MinRepLevel {
			active = append(active, t)
			totalWeight += t.Weight
		}
	}
	if len(active) == 0 || totalWeight <= 0 {
		return catalog.CatVillageCommission{}, false
	}
	r := rng.Intn(totalWeight)
	for _, t := range active {
		r -= t.Weight
		if r < 0 {
			return t, true
		}
	}
	return active[len(active)-1], true
}

func rollItemRangeMap(pool map[string]catalog.CatVillageItemRange, rng *rand.Rand) []RewardItem {
	out := make([]RewardItem, 0, len(pool))
	for itemID, entry := range pool {
		itemID = strings.TrimSpace(itemID)
		if itemID == "" || entry.Min <= 0 {
			continue
		}
		rangeSize := entry.Max - entry.Min
		count := entry.Min
		if rangeSize > 0 {
			count += rng.Int63n(rangeSize + 1)
		}
		if count <= 0 {
			continue
		}
		out = append(out, RewardItem{ItemID: itemID, Count: count})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ItemID < out[j].ItemID })
	return out
}

func decodeCosts(raw []byte) []CostItem {
	if len(raw) == 0 {
		return []CostItem{}
	}
	m := make(map[string]int64)
	if err := json.Unmarshal(raw, &m); err != nil {
		return []CostItem{}
	}
	out := make([]CostItem, 0, len(m))
	for itemID, qty := range m {
		if itemID != "" && qty > 0 {
			out = append(out, CostItem{ItemID: itemID, Quantity: qty})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ItemID < out[j].ItemID })
	return out
}

func encodeCosts(costs []RewardItem) ([]byte, error) {
	m := make(map[string]int64, len(costs))
	for _, c := range costs {
		if c.ItemID != "" && c.Count > 0 {
			m[c.ItemID] = c.Count
		}
	}
	return json.Marshal(m)
}

func decodeRewardItems(raw []byte) []RewardItem {
	if len(raw) == 0 {
		return []RewardItem{}
	}
	items := make(map[string]int64)
	if err := json.Unmarshal(raw, &items); err != nil {
		return []RewardItem{}
	}
	out := make([]RewardItem, 0, len(items))
	for itemID, count := range items {
		if itemID == "" || count <= 0 {
			continue
		}
		out = append(out, RewardItem{ItemID: itemID, Count: count})
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].ItemID < out[j].ItemID
	})
	return out
}

func encodeRewardItems(items []RewardItem) ([]byte, error) {
	m := make(map[string]int64, len(items))
	for _, item := range items {
		if item.ItemID != "" && item.Count > 0 {
			m[item.ItemID] = item.Count
		}
	}
	return json.Marshal(m)
}
