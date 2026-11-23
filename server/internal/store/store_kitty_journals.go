package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

const (
	KittyJournalEntryTypeDiary     = "diary"
	KittyJournalEntryTypeEncounter = "encounter"
)

type KittyJournalPersonalityGrantInput struct {
	CandidateIDs []string
	Chance       float64
	RNG          *rand.Rand
}

type KittyJournalWriteInput struct {
	UserID           int64
	KittyID          int64
	EncounterID      *int64
	EntryType        string
	TitleKey         string
	ContentKey       string
	ContentParams    map[string]any
	ActionID         string
	Mood             *float64
	Reward           map[string]any
	Extra            map[string]any
	KittyExp         int64
	BondExp          int64
	MoodDelta        float64
	PersonalityGrant *KittyJournalPersonalityGrantInput
}

type KittyJournalRewardClaimInput struct {
	RewardDate string
	RewardSlot string
	RewardKey  string
}

type KittyEncounterWriteInput struct {
	FromUserID    int64
	FromKittyID   int64
	ToUserID      int64
	ToKittyID     int64
	ContentKey    string
	ContentParams map[string]any
}

type KittyRewardGrantSpec struct {
	ItemID   string
	Quantity int64
}

type KittyCandidate struct {
	UserID int64
	Kitty  PlayerKitty
}

func (s *Store) GetPlayerKitty(ctx context.Context, userID int64, kittyID int64) (PlayerKitty, error) {
	var row PlayerKitty
	var extraRaw []byte
	err := s.db.QueryRow(ctx, `
		SELECT id, name, status, level, battle_level_cap, exp, total_exp, bond_exp, mood, extra_jsonb, created_at
		FROM player_kitties
		WHERE user_id = $1 AND id = $2
	`, userID, kittyID).Scan(&row.ID, &row.Name, &row.Status, &row.Level, &row.BattleLevelCap, &row.Exp, &row.TotalExp, &row.BondExp, &row.Mood, &extraRaw, &row.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return PlayerKitty{}, ErrKittyNotFound
	}
	if err != nil {
		return PlayerKitty{}, err
	}
	s.enrichPlayerKittyRow(&row, extraRaw)
	personalityByKitty, personalityErr := s.listKittyPersonalityIDs(ctx, userID, []int64{kittyID})
	if personalityErr != nil {
		return PlayerKitty{}, personalityErr
	}
	row.Extra = setKittyPersonalityIDsExtra(row.Extra, personalityByKitty[kittyID])
	return row, nil
}

func (s *Store) ListPlayerKittyJournals(ctx context.Context, userID int64, kittyID int64, page int, pageSize int) ([]PlayerKittyJournal, int64, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 200 {
		pageSize = 200
	}
	offset := int64((page - 1) * pageSize)

	var total int64
	if err := s.db.QueryRow(ctx, `
		SELECT COUNT(1)
		FROM player_kitty_journals
		WHERE user_id = $1 AND kitty_id = $2
	`, userID, kittyID).Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := s.db.Query(ctx, `
		SELECT id, user_id, kitty_id, encounter_id, entry_type, title_key, content_key, content_params_jsonb, action_id, mood, reward_jsonb, extra_jsonb, created_at
		FROM player_kitty_journals
		WHERE user_id = $1 AND kitty_id = $2
		ORDER BY created_at DESC, id DESC
		OFFSET $3 LIMIT $4
	`, userID, kittyID, offset, pageSize)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	out := make([]PlayerKittyJournal, 0, pageSize)
	for rows.Next() {
		var (
			row            PlayerKittyJournal
			contentRaw     []byte
			rewardRaw      []byte
			extraRaw       []byte
			encounterIDRaw *int64
			moodRaw        *float64
		)
		if err := rows.Scan(
			&row.ID,
			&row.UserID,
			&row.KittyID,
			&encounterIDRaw,
			&row.EntryType,
			&row.TitleKey,
			&row.ContentKey,
			&contentRaw,
			&row.ActionID,
			&moodRaw,
			&rewardRaw,
			&extraRaw,
			&row.CreatedAt,
		); err != nil {
			return nil, 0, err
		}
		row.EncounterID = encounterIDRaw
		row.Mood = moodRaw
		row.ContentParams = decodeJSONMap(contentRaw)
		row.Reward = decodeJSONMap(rewardRaw)
		row.Extra = decodeJSONMap(extraRaw)
		out = append(out, row)
	}
	return out, total, rows.Err()
}

func (s *Store) ListPlayerKittyEncounters(ctx context.Context, userID int64, kittyID int64, page int, pageSize int) ([]PlayerKittyEncounter, int64, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 200 {
		pageSize = 200
	}
	offset := int64((page - 1) * pageSize)

	var total int64
	if err := s.db.QueryRow(ctx, `
		SELECT COUNT(1)
		FROM player_kitty_encounters
		WHERE (from_user_id = $1 AND from_kitty_id = $2)
		   OR (to_user_id = $1 AND to_kitty_id = $2)
	`, userID, kittyID).Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := s.db.Query(ctx, `
		SELECT id, from_user_id, from_kitty_id, to_user_id, to_kitty_id, content_key, content_params_jsonb, created_at
		FROM player_kitty_encounters
		WHERE (from_user_id = $1 AND from_kitty_id = $2)
		   OR (to_user_id = $1 AND to_kitty_id = $2)
		ORDER BY created_at DESC, id DESC
		OFFSET $3 LIMIT $4
	`, userID, kittyID, offset, pageSize)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	out := make([]PlayerKittyEncounter, 0, pageSize)
	for rows.Next() {
		var (
			row        PlayerKittyEncounter
			contentRaw []byte
		)
		if err := rows.Scan(
			&row.ID,
			&row.FromUserID,
			&row.FromKittyID,
			&row.ToUserID,
			&row.ToKittyID,
			&row.ContentKey,
			&contentRaw,
			&row.CreatedAt,
		); err != nil {
			return nil, 0, err
		}
		row.ContentParams = decodeJSONMap(contentRaw)
		out = append(out, row)
	}
	return out, total, rows.Err()
}

func (s *Store) ListAllKittyCandidates(ctx context.Context) ([]KittyCandidate, error) {
	rows, err := s.db.Query(ctx, `
		SELECT k.user_id, k.id, k.name, k.status, k.level, k.battle_level_cap, k.exp, k.total_exp, k.bond_exp, k.mood, k.extra_jsonb, k.created_at
		FROM player_kitties k
		JOIN users u ON u.id = k.user_id
		WHERE NOT (
			COALESCE(TRIM(u.ban_reason), '') <> ''
			AND (
				u.ban_is_permanent = TRUE
				OR (u.ban_until IS NOT NULL AND u.ban_until > now())
			)
		)
		ORDER BY k.id ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]KittyCandidate, 0, 64)
	for rows.Next() {
		var row KittyCandidate
		var extraRaw []byte
		if err := rows.Scan(&row.UserID, &row.Kitty.ID, &row.Kitty.Name, &row.Kitty.Status, &row.Kitty.Level, &row.Kitty.BattleLevelCap, &row.Kitty.Exp, &row.Kitty.TotalExp, &row.Kitty.BondExp, &row.Kitty.Mood, &extraRaw, &row.Kitty.CreatedAt); err != nil {
			return nil, err
		}
		row.Kitty.Name = strings.TrimSpace(row.Kitty.Name)
		row.Kitty.Status = normalizeKittyStatus(row.Kitty.Status)
		row.Kitty.BattleLevelCap = s.effectiveBattleLevelCap(row.Kitty.BattleLevelCap, row.Kitty.Level)
		row.Kitty.Extra = decodeJSONMap(extraRaw)
		row.Kitty.NextExp = s.KittyNextExp(row.Kitty.Level)
		out = append(out, row)
	}
	return out, rows.Err()
}

func (s *Store) PickRandomEncounterTarget(ctx context.Context, fromUserID int64, fromKittyID int64, rng *rand.Rand) (KittyCandidate, error) {
	rows, err := s.db.Query(ctx, `
		SELECT k.user_id, k.id, k.name, k.status, k.level, k.battle_level_cap, k.exp, k.total_exp, k.bond_exp, k.mood, k.extra_jsonb, k.created_at
		FROM player_kitties k
		JOIN users u ON u.id = k.user_id
		WHERE k.user_id <> $1
		  AND k.id <> $2
		  AND NOT (
			  COALESCE(TRIM(u.ban_reason), '') <> ''
			  AND (
				  u.ban_is_permanent = TRUE
				  OR (u.ban_until IS NOT NULL AND u.ban_until > now())
			  )
		  )
		ORDER BY k.id ASC
	`, fromUserID, fromKittyID)
	if err != nil {
		return KittyCandidate{}, err
	}
	defer rows.Close()

	candidates := make([]KittyCandidate, 0, 16)
	for rows.Next() {
		var row KittyCandidate
		var extraRaw []byte
		if err := rows.Scan(&row.UserID, &row.Kitty.ID, &row.Kitty.Name, &row.Kitty.Status, &row.Kitty.Level, &row.Kitty.BattleLevelCap, &row.Kitty.Exp, &row.Kitty.TotalExp, &row.Kitty.BondExp, &row.Kitty.Mood, &extraRaw, &row.Kitty.CreatedAt); err != nil {
			return KittyCandidate{}, err
		}
		row.Kitty.Name = strings.TrimSpace(row.Kitty.Name)
		row.Kitty.Status = normalizeKittyStatus(row.Kitty.Status)
		row.Kitty.BattleLevelCap = s.effectiveBattleLevelCap(row.Kitty.BattleLevelCap, row.Kitty.Level)
		row.Kitty.Extra = decodeJSONMap(extraRaw)
		row.Kitty.NextExp = s.KittyNextExp(row.Kitty.Level)
		candidates = append(candidates, row)
	}
	if err := rows.Err(); err != nil {
		return KittyCandidate{}, err
	}
	if len(candidates) == 0 {
		return KittyCandidate{}, ErrKittyEncounterTargetNotFound
	}
	if rng == nil {
		rng = rand.New(rand.NewSource(time.Now().UnixNano()))
	}
	return candidates[rng.Intn(len(candidates))], nil
}

func (s *Store) GetKittyRunningActionID(ctx context.Context, userID int64, kittyID int64) (string, error) {
	var actionID string
	err := s.db.QueryRow(ctx, `
		SELECT action_id
		FROM player_action_queue
		WHERE user_id = $1
		  AND actor_type = 'kitty'
		  AND actor_id = $2
		  AND status = 'running'
		ORDER BY queue_order ASC, id ASC
		LIMIT 1
	`, userID, kittyID).Scan(&actionID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	return strings.TrimSpace(actionID), err
}

func (s *Store) CreateKittyDiaryWithRewards(ctx context.Context, input KittyJournalWriteInput, rewards []KittyRewardGrantSpec, maxEntriesPerKitty int) error {
	_, err := s.createKittyDiaryWithRewards(ctx, input, rewards, maxEntriesPerKitty, nil)
	return err
}

func (s *Store) CreateKittyDiaryWithRewardsOnce(ctx context.Context, input KittyJournalWriteInput, rewards []KittyRewardGrantSpec, maxEntriesPerKitty int, claim KittyJournalRewardClaimInput) (bool, error) {
	return s.createKittyDiaryWithRewards(ctx, input, rewards, maxEntriesPerKitty, &claim)
}

func (s *Store) createKittyDiaryWithRewards(ctx context.Context, input KittyJournalWriteInput, rewards []KittyRewardGrantSpec, maxEntriesPerKitty int, claim *KittyJournalRewardClaimInput) (bool, error) {
	created := false
	err := s.withTx(ctx, func(tx pgx.Tx) error {
		if err := s.lockUserTx(ctx, tx, input.UserID); err != nil {
			return err
		}
		if claim != nil {
			claimed, err := s.claimKittyJournalRewardTx(ctx, tx, input.UserID, input.KittyID, *claim)
			if err != nil {
				return err
			}
			if !claimed {
				return nil
			}
		}
		if g := input.PersonalityGrant; g != nil && g.RNG != nil && len(g.CandidateIDs) > 0 && g.RNG.Float64() < g.Chance {
			if acquiredID, err := s.tryGrantJournalPersonalityTx(ctx, tx, input.UserID, input.KittyID, g.CandidateIDs, g.RNG); err == nil && acquiredID != "" {
				if input.Extra == nil {
					input.Extra = make(map[string]any)
				}
				input.Extra["acquiredPersonality"] = acquiredID
			}
		}
		if _, err := s.insertKittyJournalTx(ctx, tx, input); err != nil {
			return err
		}
		for _, reward := range rewards {
			itemID := strings.TrimSpace(reward.ItemID)
			if itemID == "" || reward.Quantity <= 0 {
				continue
			}
			if err := s.LedgerGrantTemplateTx(ctx, tx, input.UserID, itemID, reward.Quantity, InventoryLedgerMeta{
				SourceSurface: "kitty_journal",
				TargetSurface: "inventory",
				Details: map[string]any{
					"type":    "kitty_journal_reward",
					"kittyId": input.KittyID,
				},
			}); err != nil {
				return err
			}
		}
		if err := s.applyKittyGrowthTx(ctx, tx, input.UserID, input.KittyID, input.KittyExp, input.BondExp, input.MoodDelta); err != nil {
			return err
		}
		if err := s.trimKittyJournalHistoryTx(ctx, tx, input.UserID, input.KittyID, maxEntriesPerKitty); err != nil {
			return err
		}
		created = true
		return nil
	})
	return created, err
}

func (s *Store) claimKittyJournalRewardTx(ctx context.Context, tx pgx.Tx, userID int64, kittyID int64, claim KittyJournalRewardClaimInput) (bool, error) {
	rewardDate := strings.TrimSpace(claim.RewardDate)
	rewardSlot := strings.TrimSpace(claim.RewardSlot)
	rewardKey := strings.TrimSpace(claim.RewardKey)
	if userID <= 0 || kittyID <= 0 || rewardDate == "" || rewardSlot == "" || rewardKey == "" {
		return false, ErrBadQuantity
	}
	tag, err := tx.Exec(ctx, `
		INSERT INTO player_kitty_journal_reward_claims(user_id, kitty_id, reward_date, reward_slot, reward_key, created_at)
		VALUES($1, $2, $3, $4, $5, now())
		ON CONFLICT (user_id, kitty_id, reward_date, reward_slot, reward_key) DO NOTHING
	`, userID, kittyID, rewardDate, rewardSlot, rewardKey)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}

func (s *Store) applyKittyGrowthTx(ctx context.Context, tx pgx.Tx, userID int64, kittyID int64, expGain int64, bondGain int64, moodDelta float64) error {
	if expGain < 0 || bondGain < 0 {
		return ErrBadQuantity
	}
	if expGain == 0 && bondGain == 0 && moodDelta == 0 {
		return nil
	}
	var level int
	var exp int64
	var totalExp int64
	var bondExp int64
	var mood float64
	if err := tx.QueryRow(ctx, `
		SELECT level, exp, total_exp, bond_exp, mood
		FROM player_kitties
		WHERE user_id = $1 AND id = $2
		FOR UPDATE
	`, userID, kittyID).Scan(&level, &exp, &totalExp, &bondExp, &mood); err != nil {
		return err
	}
	totalExp += expGain
	exp += expGain
	maxLevel := s.config.KittyMaxLevel
	if maxLevel <= 0 {
		maxLevel = 30
	}
	for level < maxLevel {
		next := s.KittyNextExp(level)
		if next <= 0 || exp < next {
			break
		}
		exp -= next
		level++
	}
	if level >= maxLevel {
		exp = 0
	}
	bondExp += bondGain
	mood += moodDelta
	if mood > 1 {
		mood = 1
	}
	if mood < -1 {
		mood = -1
	}
	_, err := tx.Exec(ctx, `
		UPDATE player_kitties
		SET level = $3, exp = $4, total_exp = $5, bond_exp = $6, mood = $7, updated_at = now()
		WHERE user_id = $1 AND id = $2
	`, userID, kittyID, level, exp, totalExp, bondExp, mood)
	return err
}

func (s *Store) CreateKittyEncounterWithDualJournals(
	ctx context.Context,
	encounter KittyEncounterWriteInput,
	fromJournal KittyJournalWriteInput,
	toJournal KittyJournalWriteInput,
	maxEntriesPerKitty int,
) error {
	return s.CreateKittyEncounterWithDualJournalsAndRewards(ctx, encounter, fromJournal, toJournal, nil, maxEntriesPerKitty)
}

func (s *Store) CreateKittyEncounterWithDualJournalsAndRewards(
	ctx context.Context,
	encounter KittyEncounterWriteInput,
	fromJournal KittyJournalWriteInput,
	toJournal KittyJournalWriteInput,
	fromRewards []KittyRewardGrantSpec,
	maxEntriesPerKitty int,
) error {
	return s.withTx(ctx, func(tx pgx.Tx) error {
		firstUserID, secondUserID := encounter.FromUserID, encounter.ToUserID
		if secondUserID < firstUserID {
			firstUserID, secondUserID = secondUserID, firstUserID
		}
		if err := s.lockUserTx(ctx, tx, firstUserID); err != nil {
			return err
		}
		if secondUserID != firstUserID {
			if err := s.lockUserTx(ctx, tx, secondUserID); err != nil {
				return err
			}
		}
		encounterID, err := s.insertKittyEncounterTx(ctx, tx, encounter)
		if err != nil {
			return err
		}
		fromJournal.EncounterID = &encounterID
		fromJournal.EntryType = KittyJournalEntryTypeEncounter
		if g := fromJournal.PersonalityGrant; g != nil && g.RNG != nil && len(g.CandidateIDs) > 0 && g.RNG.Float64() < g.Chance {
			if acquiredID, err := s.tryGrantJournalPersonalityTx(ctx, tx, fromJournal.UserID, fromJournal.KittyID, g.CandidateIDs, g.RNG); err == nil && acquiredID != "" {
				if fromJournal.Extra == nil {
					fromJournal.Extra = make(map[string]any)
				}
				fromJournal.Extra["acquiredPersonality"] = acquiredID
			}
		}
		if _, err := s.insertKittyJournalTx(ctx, tx, fromJournal); err != nil {
			return err
		}
		toJournal.EncounterID = &encounterID
		toJournal.EntryType = KittyJournalEntryTypeEncounter
		if g := toJournal.PersonalityGrant; g != nil && g.RNG != nil && len(g.CandidateIDs) > 0 && g.RNG.Float64() < g.Chance {
			if acquiredID, err := s.tryGrantJournalPersonalityTx(ctx, tx, toJournal.UserID, toJournal.KittyID, g.CandidateIDs, g.RNG); err == nil && acquiredID != "" {
				if toJournal.Extra == nil {
					toJournal.Extra = make(map[string]any)
				}
				toJournal.Extra["acquiredPersonality"] = acquiredID
			}
		}
		if _, err := s.insertKittyJournalTx(ctx, tx, toJournal); err != nil {
			return err
		}
		for _, reward := range fromRewards {
			itemID := strings.TrimSpace(reward.ItemID)
			if itemID == "" || reward.Quantity <= 0 {
				continue
			}
			if err := s.LedgerGrantTemplateTx(ctx, tx, encounter.FromUserID, itemID, reward.Quantity, InventoryLedgerMeta{
				SourceSurface: "kitty_encounter",
				TargetSurface: "inventory",
				Details: map[string]any{
					"type":        "kitty_encounter_reward",
					"kittyId":     encounter.FromKittyID,
					"encounterId": encounterID,
				},
			}); err != nil {
				return err
			}
		}
		if err := s.trimKittyJournalHistoryTx(ctx, tx, fromJournal.UserID, fromJournal.KittyID, maxEntriesPerKitty); err != nil {
			return err
		}
		return s.trimKittyJournalHistoryTx(ctx, tx, toJournal.UserID, toJournal.KittyID, maxEntriesPerKitty)
	})
}

func (s *Store) insertKittyEncounterTx(ctx context.Context, tx pgx.Tx, input KittyEncounterWriteInput) (int64, error) {
	contentParamsRaw, err := encodeJSONMap(input.ContentParams)
	if err != nil {
		return 0, err
	}
	var encounterID int64
	err = tx.QueryRow(ctx, `
		INSERT INTO player_kitty_encounters(
			from_user_id, from_kitty_id, to_user_id, to_kitty_id, content_key, content_params_jsonb, created_at
		)
		VALUES($1, $2, $3, $4, $5, $6::jsonb, now())
		RETURNING id
	`, input.FromUserID, input.FromKittyID, input.ToUserID, input.ToKittyID, strings.TrimSpace(input.ContentKey), string(contentParamsRaw)).Scan(&encounterID)
	return encounterID, err
}

func (s *Store) insertKittyJournalTx(ctx context.Context, tx pgx.Tx, input KittyJournalWriteInput) (int64, error) {
	contentParamsRaw, err := encodeJSONMap(input.ContentParams)
	if err != nil {
		return 0, err
	}
	rewardRaw, err := encodeJSONMap(input.Reward)
	if err != nil {
		return 0, err
	}
	extraRaw, err := encodeJSONMap(input.Extra)
	if err != nil {
		return 0, err
	}
	entryType := strings.TrimSpace(input.EntryType)
	if entryType == "" {
		entryType = KittyJournalEntryTypeDiary
	}
	var journalID int64
	err = tx.QueryRow(ctx, `
		INSERT INTO player_kitty_journals(
			user_id, kitty_id, encounter_id, entry_type, title_key, content_key, content_params_jsonb, action_id, mood, reward_jsonb, extra_jsonb, created_at
		)
		VALUES($1, $2, $3, $4, $5, $6, $7::jsonb, $8, $9, $10::jsonb, $11::jsonb, now())
		RETURNING id
	`, input.UserID, input.KittyID, input.EncounterID, entryType, strings.TrimSpace(input.TitleKey), strings.TrimSpace(input.ContentKey), string(contentParamsRaw), strings.TrimSpace(input.ActionID), input.Mood, string(rewardRaw), string(extraRaw)).Scan(&journalID)
	return journalID, err
}

func (s *Store) trimKittyJournalHistoryTx(ctx context.Context, tx pgx.Tx, userID int64, kittyID int64, maxEntriesPerKitty int) error {
	if maxEntriesPerKitty <= 0 {
		return nil
	}
	_, err := tx.Exec(ctx, `
		WITH stale AS (
			SELECT id
			FROM player_kitty_journals
			WHERE user_id = $1 AND kitty_id = $2
			ORDER BY created_at DESC, id DESC
			OFFSET $3
		)
		DELETE FROM player_kitty_journals
		WHERE id IN (SELECT id FROM stale)
	`, userID, kittyID, maxEntriesPerKitty)
	return err
}

func encodeJSONMap(input map[string]any) ([]byte, error) {
	if input == nil {
		input = map[string]any{}
	}
	return json.Marshal(input)
}

func decodeJSONMap(raw []byte) map[string]any {
	if len(raw) == 0 {
		return map[string]any{}
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil || out == nil {
		return map[string]any{}
	}
	return out
}

// CountPlayerKittyJournalsTotal returns the total number of journal entries for a user across all kitties.
func (s *Store) CountPlayerKittyJournalsTotal(ctx context.Context, userID int64) (int64, error) {
	var count int64
	err := s.db.QueryRow(ctx, `SELECT COUNT(1) FROM player_kitty_journals WHERE user_id = $1`, userID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count journals: %w", err)
	}
	return count, nil
}
