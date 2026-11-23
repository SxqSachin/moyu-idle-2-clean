package store

import (
	"context"
	"fmt"
	"hash/fnv"
	"math"
	"math/rand"
	"strings"

	"github.com/jackc/pgx/v5"
)

const (
	kittyPersonalityIDsExtraKey = "personalityIds"
)

func normalizeLower(raw string) string {
	return strings.ToLower(strings.TrimSpace(raw))
}

func containsLower(values []string, target string) bool {
	target = normalizeLower(target)
	for _, value := range values {
		if normalizeLower(value) == target {
			return true
		}
	}
	return false
}

func normalizeUniqueIDs(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := map[string]struct{}{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		normalized := normalizeLower(value)
		if normalized == "" {
			continue
		}
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}
		out = append(out, normalized)
	}
	return out
}

func extractKittyPersonalityIDs(extra map[string]any) []string {
	if extra == nil {
		return nil
	}
	out := make([]string, 0, 4)
	switch raw := extra[kittyPersonalityIDsExtraKey].(type) {
	case []any:
		for _, entry := range raw {
			out = append(out, normalizeLower(fmt.Sprintf("%v", entry)))
		}
	case []string:
		for _, entry := range raw {
			out = append(out, normalizeLower(entry))
		}
	}
	return normalizeUniqueIDs(out)
}

func (s *Store) ResolveKittyPersonalityProfiles(extra map[string]any) []KittyPersonalityProfile {
	ids := extractKittyPersonalityIDs(extra)
	if len(ids) == 0 {
		return nil
	}
	out := make([]KittyPersonalityProfile, 0, len(ids))
	for _, id := range ids {
		profile, ok := s.config.KittyPersonalityProfiles[id]
		if !ok {
			continue
		}
		out = append(out, profile)
	}
	return out
}

func resolveKittyPositionLockFromProfiles(profiles []KittyPersonalityProfile) (string, string, string) {
	for _, profile := range profiles {
		for _, effect := range profile.Effects {
			if effect.Kind != "battle_position_lock" {
				continue
			}
			locked := normalizedBattlePosition(effect.Position)
			if locked == "" {
				continue
			}
			return locked, normalizeLower(profile.ID), strings.TrimSpace(profile.NameKey)
		}
	}
	return "", "", ""
}

// ResolveKittyPositionLock resolves a kitty's authored position lock from the
// personality IDs already attached to a PlayerKitty snapshot.
func (s *Store) ResolveKittyPositionLock(extra map[string]any) (string, string, string) {
	return resolveKittyPositionLockFromProfiles(s.ResolveKittyPersonalityProfiles(extra))
}

func setKittyPersonalityIDsExtra(extra map[string]any, personalityIDs []string) map[string]any {
	if extra == nil {
		extra = map[string]any{}
	}
	extra[kittyPersonalityIDsExtraKey] = normalizeUniqueIDs(personalityIDs)
	return extra
}

func (s *Store) listOwnedKittyPersonalityIDsTx(ctx context.Context, tx pgx.Tx, userID, kittyID int64, lock bool) ([]string, error) {
	clause := ""
	if lock {
		clause = " FOR UPDATE"
	}
	primarySQL := personalityPrimarySQL + clause
	fallbackSQL := personalityFallbackSQL + clause
	return s.queryPersonalityIDs(ctx, tx, primarySQL, fallbackSQL,
		[]any{userID, kittyID}, []any{kittyID, userID})
}

func (s *Store) listKittyPersonalityIDs(ctx context.Context, userID int64, kittyIDs []int64) (map[int64][]string, error) {
	result := make(map[int64][]string, len(kittyIDs))
	if len(kittyIDs) == 0 {
		return result, nil
	}
	rows, err := s.db.Query(ctx, `
		SELECT kitty_id, personality_id
		FROM player_kitty_personalities
		WHERE user_id = $1
		  AND kitty_id = ANY($2::bigint[])
		ORDER BY kitty_id ASC, created_at ASC, personality_id ASC
	`, userID, kittyIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var kittyID int64
		var personalityID string
		if err := rows.Scan(&kittyID, &personalityID); err != nil {
			return nil, err
		}
		result[kittyID] = append(result[kittyID], normalizeLower(personalityID))
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for kittyID, ids := range result {
		result[kittyID] = normalizeUniqueIDs(ids)
	}
	return result, nil
}

func (s *Store) getOwnedKittyPersonalityProfilesTx(ctx context.Context, tx pgx.Tx, userID, kittyID int64) ([]KittyPersonalityProfile, error) {
	ids, err := s.listOwnedKittyPersonalityIDsTx(ctx, tx, userID, kittyID, false)
	if err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return nil, nil
	}
	out := make([]KittyPersonalityProfile, 0, len(ids))
	for _, id := range ids {
		profile, ok := s.config.KittyPersonalityProfiles[id]
		if !ok {
			continue
		}
		out = append(out, profile)
	}
	return out, nil
}

func (s *Store) ListOwnedKittyPersonalityProfiles(ctx context.Context, userID, kittyID int64) ([]KittyPersonalityProfile, error) {
	idsByKitty, err := s.listKittyPersonalityIDs(ctx, userID, []int64{kittyID})

	if err != nil {
		return nil, err
	}
	ids := idsByKitty[kittyID]
	if len(ids) == 0 {
		return nil, nil
	}
	out := make([]KittyPersonalityProfile, 0, len(ids))
	for _, id := range ids {
		profile, ok := s.config.KittyPersonalityProfiles[id]
		if !ok {
			continue
		}
		out = append(out, profile)
	}
	return out, nil
}

func (s *Store) resolveNewKittyPersonalityIDs(kittySeed int64) []string {
	defaultID := normalizeLower(s.config.KittyDefaultPersonalityID)
	if defaultID != "" {
		if _, ok := s.config.KittyPersonalityProfiles[defaultID]; ok {
			return []string{defaultID}
		}
	}
	if len(s.config.KittyPersonalityOrder) == 0 {
		return nil
	}
	weighted := make([]KittyPersonalityProfile, 0, len(s.config.KittyPersonalityOrder))
	totalWeight := 0.0
	for _, id := range s.config.KittyPersonalityOrder {
		profile, ok := s.config.KittyPersonalityProfiles[normalizeLower(id)]
		if !ok || profile.Weight <= 0 {
			continue
		}
		totalWeight += profile.Weight
		weighted = append(weighted, profile)
	}
	if len(weighted) == 0 || totalWeight <= 0 {
		for _, id := range s.config.KittyPersonalityOrder {
			normalized := normalizeLower(id)
			if _, ok := s.config.KittyPersonalityProfiles[normalized]; ok {
				return []string{normalized}
			}
		}
		return nil
	}
	hasher := fnv.New64a()
	_, _ = hasher.Write([]byte(fmt.Sprintf("%d", kittySeed)))
	pick := math.Mod(float64(hasher.Sum64()), totalWeight)
	acc := 0.0
	for _, profile := range weighted {
		acc += profile.Weight
		if pick <= acc {
			return []string{normalizeLower(profile.ID)}
		}
	}
	return []string{normalizeLower(weighted[len(weighted)-1].ID)}
}

func (s *Store) upsertKittyPersonalityIDsTx(ctx context.Context, tx pgx.Tx, userID, kittyID int64, personalityIDs []string) error {
	if err := s.replaceKittyPersonalityRowsTx(ctx, tx, userID, kittyID, normalizeUniqueIDs(personalityIDs)); err != nil {
		return err
	}
	tag, err := tx.Exec(ctx, `
		UPDATE player_kitties
		SET updated_at = now()
		WHERE user_id = $1 AND id = $2
	`, userID, kittyID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() <= 0 {
		return ErrKittyNotFound
	}
	return nil
}

func (s *Store) replaceKittyPersonalityRowsTx(ctx context.Context, tx pgx.Tx, userID, kittyID int64, personalityIDs []string) error {
	if _, err := tx.Exec(ctx, `
		DELETE FROM player_kitty_personalities
		WHERE user_id = $1 AND kitty_id = $2
	`, userID, kittyID); err != nil {
		return err
	}
	for _, personalityID := range personalityIDs {
		if personalityID == "" {
			continue
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO player_kitty_personalities(user_id, kitty_id, personality_id, created_at)
			VALUES($1, $2, $3, now())
			ON CONFLICT (kitty_id, personality_id) DO NOTHING
		`, userID, kittyID, personalityID); err != nil {
			return err
		}
	}
	return nil
}

// tryGrantJournalPersonalityTx attempts to grant one personality from candidates to a kitty
// within an existing transaction. Returns the acquired personality ID, or "" if nothing was granted.
// The caller must hold a user lock before calling this.
func (s *Store) tryGrantJournalPersonalityTx(ctx context.Context, tx pgx.Tx, userID, kittyID int64, candidates []string, rng *rand.Rand) (string, error) {
	if len(candidates) == 0 || rng == nil {
		return "", nil
	}
	owned, err := s.listOwnedKittyPersonalityIDsTx(ctx, tx, userID, kittyID, true)
	if err != nil {
		return "", err
	}
	ownedSet := make(map[string]struct{}, len(owned))
	for _, id := range owned {
		ownedSet[id] = struct{}{}
	}
	maxPersonalities := s.config.KittyMaxPersonalities
	if maxPersonalities > 0 && len(owned) >= maxPersonalities {
		return "", nil
	}
	eligible := make([]string, 0, len(candidates))
	for _, raw := range candidates {
		id := normalizeLower(raw)
		if id == "" {
			continue
		}
		if _, exists := ownedSet[id]; exists {
			continue
		}
		if _, ok := s.config.KittyPersonalityProfiles[id]; !ok {
			continue
		}
		eligible = append(eligible, id)
	}
	if len(eligible) == 0 {
		return "", nil
	}
	picked := eligible[rng.Intn(len(eligible))]
	newIDs := append(owned, picked)
	if err := s.upsertKittyPersonalityIDsTx(ctx, tx, userID, kittyID, newIDs); err != nil {
		return "", err
	}
	if err := s.recomputeKittyDerivedStatsTx(ctx, tx, userID, kittyID, s.attributeSeedsFromGameData()); err != nil {
		return "", err
	}
	return picked, nil
}

func (s *Store) AddKittyPersonality(ctx context.Context, userID, kittyID int64, personalityID string) (PlayerKitty, error) {
	normalized := normalizeLower(personalityID)
	if normalized == "" {
		return PlayerKitty{}, ErrKittyPersonalityInvalid
	}
	if _, ok := s.config.KittyPersonalityProfiles[normalized]; !ok {
		return PlayerKitty{}, ErrKittyPersonalityInvalid
	}
	if err := s.withTx(ctx, func(tx pgx.Tx) error {
		if err := s.lockUserTx(ctx, tx, userID); err != nil {
			return err
		}
		owned, err := s.isOwnedKittyTx(ctx, tx, userID, kittyID)
		if err != nil {
			return err
		}
		if !owned {
			return ErrKittyNotFound
		}
		ids, err := s.listOwnedKittyPersonalityIDsTx(ctx, tx, userID, kittyID, true)
		if err != nil {
			return err
		}
		exists := false
		for _, id := range ids {
			if id == normalized {
				exists = true
				break
			}
		}
		if exists {
			return ErrKittyPersonalityExists
		}
		ids = append(ids, normalized)
		if err := s.upsertKittyPersonalityIDsTx(ctx, tx, userID, kittyID, ids); err != nil {
			return err
		}
		return s.recomputeKittyDerivedStatsTx(ctx, tx, userID, kittyID, s.attributeSeedsFromGameData())
	}); err != nil {
		return PlayerKitty{}, err
	}
	return s.GetPlayerKitty(ctx, userID, kittyID)
}

func (s *Store) RemoveKittyPersonality(ctx context.Context, userID, kittyID int64, personalityID string) (PlayerKitty, error) {
	normalized := normalizeLower(personalityID)
	if normalized == "" {
		return PlayerKitty{}, ErrKittyPersonalityInvalid
	}
	if err := s.withTx(ctx, func(tx pgx.Tx) error {
		if err := s.lockUserTx(ctx, tx, userID); err != nil {
			return err
		}
		owned, err := s.isOwnedKittyTx(ctx, tx, userID, kittyID)
		if err != nil {
			return err
		}
		if !owned {
			return ErrKittyNotFound
		}
		ids, err := s.listOwnedKittyPersonalityIDsTx(ctx, tx, userID, kittyID, true)
		if err != nil {
			return err
		}
		next := make([]string, 0, len(ids))
		removed := false
		for _, id := range ids {
			if id == normalized {
				removed = true
				continue
			}
			next = append(next, id)
		}
		if !removed {
			return ErrKittyPersonalityNotFound
		}
		if err := s.upsertKittyPersonalityIDsTx(ctx, tx, userID, kittyID, next); err != nil {
			return err
		}
		return s.recomputeKittyDerivedStatsTx(ctx, tx, userID, kittyID, s.attributeSeedsFromGameData())
	}); err != nil {
		return PlayerKitty{}, err
	}
	return s.GetPlayerKitty(ctx, userID, kittyID)
}

func (s *Store) RemoveKittyPersonalityWithCost(
	ctx context.Context,
	userID, kittyID int64,
	personalityID string,
	costItemID string,
	costCount int64,
) (PlayerKitty, error) {
	normalizedCostItemID := strings.TrimSpace(costItemID)
	if normalizedCostItemID == "" || costCount <= 0 {
		return PlayerKitty{}, ErrKittyPersonalityCostInvalid
	}
	normalized := normalizeLower(personalityID)
	if normalized == "" {
		return PlayerKitty{}, ErrKittyPersonalityInvalid
	}
	if err := s.withTx(ctx, func(tx pgx.Tx) error {
		if err := s.lockUserTx(ctx, tx, userID); err != nil {
			return err
		}
		owned, err := s.isOwnedKittyTx(ctx, tx, userID, kittyID)
		if err != nil {
			return err
		}
		if !owned {
			return ErrKittyNotFound
		}
		ids, err := s.listOwnedKittyPersonalityIDsTx(ctx, tx, userID, kittyID, true)
		if err != nil {
			return err
		}
		next := make([]string, 0, len(ids))
		removed := false
		for _, id := range ids {
			if id == normalized {
				removed = true
				continue
			}
			next = append(next, id)
		}
		if !removed {
			return ErrKittyPersonalityNotFound
		}
		if err := s.LedgerConsumeTemplateTx(ctx, tx, userID, normalizedCostItemID, costCount, InventoryLedgerMeta{
			SourceSurface: "cattery_kitty_personality",
			TargetSurface: "inventory",
			Details: map[string]any{
				"type":          "kitty_personality_remove",
				"kittyId":       kittyID,
				"personalityId": normalized,
			},
		}); err != nil {
			return err
		}
		if err := s.upsertKittyPersonalityIDsTx(ctx, tx, userID, kittyID, next); err != nil {
			return err
		}
		return s.recomputeKittyDerivedStatsTx(ctx, tx, userID, kittyID, s.attributeSeedsFromGameData())
	}); err != nil {
		return PlayerKitty{}, err
	}
	return s.GetPlayerKitty(ctx, userID, kittyID)
}
