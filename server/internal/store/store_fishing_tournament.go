package store

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"time"

	"mysecondwebgame/server/internal/catalog"

	"github.com/jackc/pgx/v5"
)

const (
	fishingTournamentCatchUpCost       = int64(200)
	fishingTournamentParticipantTokens = int64(50)
	fishingTournamentTopTenTokens      = int64(65)
	fishingTournamentPodiumTokens      = int64(90)
	fishingTournamentChampionTokens    = int64(120)
	fishingTournamentCatchUpWait       = 28 * 24 * time.Hour
	fishingTournamentDivisionOpen      = "open"
	fishingTournamentDivisionStandard  = "standard"
	// Keep the already-running fishing-v2 event and its entries unchanged.
	// The designated-tackle format begins with the next UTC tournament week.
	fishingTournamentStandardLoadoutStartWeek = "2026-09-07"
	// The continuity migration intentionally kept this live event's original
	// target even though the fishing-v2 schedule would have selected another fish.
	fishingTournamentEntryRepairWeek   = "2026-08-31"
	fishingTournamentEntryRepairTarget = "emberfin"
)

type FishingTournament struct {
	ID              string     `json:"id"`
	WeekKey         string     `json:"weekKey"`
	TargetSpeciesID string     `json:"targetSpeciesId"`
	RulesetVersion  string     `json:"rulesetVersion"`
	StartsAt        time.Time  `json:"startsAt"`
	EndsAt          time.Time  `json:"endsAt"`
	Status          string     `json:"status"`
	FinalizedAt     *time.Time `json:"finalizedAt,omitempty"`
}

type FishingTournamentEntryView struct {
	Rank           int64   `json:"rank"`
	Division       string  `json:"division"`
	UserID         int64   `json:"userId"`
	PlayerName     string  `json:"playerName"`
	CatchID        int64   `json:"catchId,omitempty"`
	FishInstanceID int64   `json:"fishInstanceId,omitempty"`
	SourceKind     string  `json:"sourceKind"`
	LengthCM       float64 `json:"lengthCm"`
	WeightKG       float64 `json:"weightKg"`
}

type FishingTournamentRewardView struct {
	TournamentID string     `json:"tournamentId"`
	Division     string     `json:"division"`
	Rank         int        `json:"rank"`
	TokenAmount  int64      `json:"tokenAmount"`
	ItemID       string     `json:"itemId,omitempty"`
	CosmeticKey  string     `json:"cosmeticKey,omitempty"`
	ClaimedAt    *time.Time `json:"claimedAt,omitempty"`
}

type FishingTournamentStandardRules struct {
	FishingLevel int64   `json:"fishingLevel"`
	Strategy     string  `json:"strategy"`
	Control      float64 `json:"control"`
	Sensitivity  float64 `json:"sensitivity"`
	Hooking      float64 `json:"hooking"`
	MaxLoadKG    float64 `json:"maxLoadKg"`
}

type FishingTournamentStandardTackleItem struct {
	Slot   string `json:"slot"`
	ItemID string `json:"itemId"`
}

type FishingTournamentStandardLoadout struct {
	Method string                                `json:"method"`
	Items  []FishingTournamentStandardTackleItem `json:"items"`
}

type FishingTournamentOverview struct {
	Current             FishingTournament                 `json:"current"`
	StandardRules       *FishingTournamentStandardRules   `json:"standardRules,omitempty"`
	StandardLoadout     *FishingTournamentStandardLoadout `json:"standardLoadout,omitempty"`
	OpenLeaderboard     []FishingTournamentEntryView      `json:"openLeaderboard"`
	StandardLeaderboard []FishingTournamentEntryView      `json:"standardLeaderboard"`
	MyEntries           []FishingTournamentEntryView      `json:"myEntries"`
	Rewards             []FishingTournamentRewardView     `json:"rewards"`
	Tokens              int64                             `json:"tokens"`
	CatchUpAvailable    bool                              `json:"catchUpAvailable"`
	CatchUpCost         int64                             `json:"catchUpCost"`
}

type fishingTournamentRepairBest struct {
	Open     *fishingTournamentCatchCandidate
	Standard *fishingTournamentCatchCandidate
}

type fishingTournamentCatchCandidate struct {
	CatchID  int64
	Fish     FishInstanceData
	CaughtAt time.Time
}

func fishingStandardTournamentRules() FishingTournamentStandardRules {
	return FishingTournamentStandardRules{
		FishingLevel: 20,
		Strategy:     "balanced",
		Control:      2.4,
		Sensitivity:  1.8,
		Hooking:      0,
		MaxLoadKG:    9.5,
	}
}

func (rules FishingTournamentStandardRules) loadoutSnapshot() FishingLoadoutSnapshot {
	return FishingLoadoutSnapshot{
		Control:      rules.Control,
		Sensitivity:  rules.Sensitivity,
		Hooking:      rules.Hooking,
		MaxLoadKG:    rules.MaxLoadKG,
		FishingLevel: rules.FishingLevel,
	}
}

func fishingTournamentStandardLoadout() FishingTournamentStandardLoadout {
	return FishingTournamentStandardLoadout{
		Method: "float",
		Items: []FishingTournamentStandardTackleItem{
			{Slot: "rod", ItemID: "basic_fishing_rod"},
			{Slot: "reel", ItemID: "basic_spinning_reel"},
			{Slot: "main_line", ItemID: "braided_main_line"},
			{Slot: "leader", ItemID: "clear_leader"},
			{Slot: "hook", ItemID: "fine_fishing_hook"},
			{Slot: "float_or_sinker", ItemID: "reed_float"},
			{Slot: "bait", ItemID: "earthworm_bait"},
		},
	}
}

func fishingTournamentUsesStandardLoadout(tournament FishingTournament) bool {
	weekKey := strings.TrimSpace(tournament.WeekKey)
	if weekKey == "" && !tournament.StartsAt.IsZero() {
		weekKey = fishingWeekStart(tournament.StartsAt).Format("2006-01-02")
	}
	weekStart, err := time.Parse("2006-01-02", weekKey)
	if err != nil {
		return false
	}
	cutover, err := time.Parse("2006-01-02", fishingTournamentStandardLoadoutStartWeek)
	return err == nil && !weekStart.Before(cutover)
}

func (standard FishingTournamentStandardLoadout) itemIDsBySlot() (map[string]string, bool) {
	if normalizeFishingMethod(standard.Method) == "" || len(standard.Items) == 0 {
		return nil, false
	}
	items := make(map[string]string, len(standard.Items))
	for _, item := range standard.Items {
		slot := strings.TrimSpace(item.Slot)
		itemID := strings.TrimSpace(item.ItemID)
		if slot == "" || itemID == "" {
			return nil, false
		}
		if _, duplicate := items[slot]; duplicate {
			return nil, false
		}
		items[slot] = itemID
	}
	return items, true
}

func fishingLoadoutMatchesTournamentStandard(loadout FishingLoadoutSnapshot, standard FishingTournamentStandardLoadout) bool {
	if normalizeFishingMethod(loadout.Method) != normalizeFishingMethod(standard.Method) {
		return false
	}
	expected, ok := standard.itemIDsBySlot()
	if !ok || len(loadout.FrozenComponents) != len(expected) {
		return false
	}
	actual := make(map[string]string, len(loadout.FrozenComponents))
	for _, component := range loadout.FrozenComponents {
		slot := strings.TrimSpace(component.Slot)
		itemID := strings.TrimSpace(component.ItemID)
		if slot == "" || itemID == "" {
			return false
		}
		if _, duplicate := actual[slot]; duplicate {
			return false
		}
		actual[slot] = itemID
	}
	if len(actual) != len(expected) {
		return false
	}
	for slot, itemID := range expected {
		if actual[slot] != itemID {
			return false
		}
	}
	return true
}

func normalizeFishingTournamentDivision(division string) string {
	switch strings.ToLower(strings.TrimSpace(division)) {
	case "", fishingTournamentDivisionOpen:
		return fishingTournamentDivisionOpen
	case fishingTournamentDivisionStandard:
		return fishingTournamentDivisionStandard
	default:
		return ""
	}
}

func (s *Store) validatedFishingTournamentStandardLoadout() (FishingTournamentStandardLoadout, error) {
	standard := fishingTournamentStandardLoadout()
	items, ok := standard.itemIDsBySlot()
	if !ok {
		return FishingTournamentStandardLoadout{}, fishingLoadoutError("STANDARD_CONFIG_INVALID", "")
	}
	for slot, itemID := range items {
		definition := s.gameData.FishingTackleByItemID(itemID)
		if definition == nil || strings.TrimSpace(definition.Slot) != slot || !containsFishingMethod(definition.Methods, standard.Method) {
			return FishingTournamentStandardLoadout{}, fishingLoadoutError("STANDARD_CONFIG_INVALID", "")
		}
	}
	return standard, nil
}

func (s *Store) validateFishingTournamentSelection(tournament FishingTournament, division string, loadout FishingLoadoutSnapshot) error {
	division = normalizeFishingTournamentDivision(division)
	if division == "" {
		return fishingArgsError("DIVISION_INVALID", "")
	}
	if division == fishingTournamentDivisionOpen {
		return nil
	}
	if !fishingTournamentUsesStandardLoadout(tournament) {
		return fishingArgsError("STANDARD_UNAVAILABLE", "")
	}
	standard, err := s.validatedFishingTournamentStandardLoadout()
	if err != nil {
		return err
	}
	if !fishingLoadoutMatchesTournamentStandard(loadout, standard) {
		return fishingLoadoutError("STANDARD_MISMATCH", "")
	}
	return nil
}

func fishingWeekStart(at time.Time) time.Time {
	at = at.UTC()
	dayStart := time.Date(at.Year(), at.Month(), at.Day(), 0, 0, 0, 0, time.UTC)
	daysSinceMonday := (int(dayStart.Weekday()) + 6) % 7
	return dayStart.AddDate(0, 0, -daysSinceMonday)
}

func (s *Store) ensureFishingTournamentTx(ctx context.Context, tx pgx.Tx, at time.Time) (FishingTournament, error) {
	if at.IsZero() {
		at = time.Now()
	}
	startsAt := fishingWeekStart(at)
	endsAt := startsAt.Add(7 * 24 * time.Hour)
	weekKey := startsAt.Format("2006-01-02")
	if len(s.gameData.FishingSpecies) == 0 {
		return FishingTournament{}, ErrFishingCatchNotFound
	}
	index := int(stableFishingHash(weekKey+":"+FishingRulesetVersion) % uint64(len(s.gameData.FishingSpecies)))
	targetID := s.gameData.FishingSpecies[index].ID
	id := "weekly_" + strings.ReplaceAll(weekKey, "-", "") + "_" + FishingRulesetVersion
	if _, err := tx.Exec(ctx, `
		INSERT INTO fishing_tournaments(id, week_key, target_species_id, ruleset_version, starts_at, ends_at, status, created_at)
		VALUES($1, $2, $3, $4, $5, $6, 'open', now())
		ON CONFLICT (week_key, ruleset_version) DO NOTHING
	`, id, weekKey, targetID, FishingRulesetVersion, startsAt, endsAt); err != nil {
		return FishingTournament{}, err
	}
	var row FishingTournament
	err := tx.QueryRow(ctx, `
		SELECT id, week_key, target_species_id, ruleset_version, starts_at, ends_at, status, finalized_at
		FROM fishing_tournaments
		WHERE week_key = $1 AND ruleset_version = $2
	`, weekKey, FishingRulesetVersion).Scan(&row.ID, &row.WeekKey, &row.TargetSpeciesID, &row.RulesetVersion, &row.StartsAt, &row.EndsAt, &row.Status, &row.FinalizedAt)
	return row, err
}

func (s *Store) finalizeFishingTournamentsTx(ctx context.Context, tx pgx.Tx, now time.Time) error {
	rows, err := tx.Query(ctx, `
		SELECT id, week_key, target_species_id, ruleset_version, starts_at, ends_at, status, finalized_at
		FROM fishing_tournaments
		WHERE status = 'open' AND ends_at <= $1
		ORDER BY ends_at ASC
		FOR UPDATE
	`, now)
	if err != nil {
		return err
	}
	tournaments := make([]FishingTournament, 0, 8)
	for rows.Next() {
		var tournament FishingTournament
		if err := rows.Scan(
			&tournament.ID,
			&tournament.WeekKey,
			&tournament.TargetSpeciesID,
			&tournament.RulesetVersion,
			&tournament.StartsAt,
			&tournament.EndsAt,
			&tournament.Status,
			&tournament.FinalizedAt,
		); err != nil {
			rows.Close()
			return err
		}
		tournaments = append(tournaments, tournament)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	for _, tournament := range tournaments {
		if err := s.repairFishingTournamentEntryIntegrityTx(ctx, tx, tournament); err != nil {
			return err
		}
		for _, division := range []string{fishingTournamentDivisionOpen, fishingTournamentDivisionStandard} {
			if _, err := tx.Exec(ctx, `
				WITH ranked AS (
					SELECT user_id,
					       ROW_NUMBER() OVER (ORDER BY weight_kg DESC, length_cm DESC, submitted_at ASC, user_id ASC)::int AS rank
					FROM fishing_tournament_entries
					WHERE tournament_id = $1 AND division = $2
				)
				INSERT INTO fishing_tournament_rewards(
					tournament_id, user_id, division, rank, token_amount, item_id, cosmetic_key, created_at
				)
				SELECT $1, user_id, $2, rank,
				       CASE WHEN rank = 1 THEN $3::bigint
				            WHEN rank <= 3 THEN $4::bigint
				            WHEN rank <= 10 THEN $5::bigint
				            ELSE $6::bigint END,
				       CASE WHEN rank = 1 THEN 'champion_current_rod' WHEN rank <= 3 THEN 'tournament_current_rod' ELSE '' END,
				       CASE WHEN rank = 1 THEN 'fishing_champion' ELSE '' END,
				       now()
				FROM ranked
				ON CONFLICT (tournament_id, user_id, division) DO NOTHING
			`, tournament.ID, division, fishingTournamentChampionTokens, fishingTournamentPodiumTokens, fishingTournamentTopTenTokens, fishingTournamentParticipantTokens); err != nil {
				return err
			}
		}
		if _, err := tx.Exec(ctx, `
			UPDATE fishing_tournaments SET status = 'finalized', finalized_at = $2 WHERE id = $1
		`, tournament.ID, now); err != nil {
			return err
		}
	}
	return nil
}

func fishingTournamentRewardForRank(rank int) (int64, string, string) {
	switch {
	case rank == 1:
		return fishingTournamentChampionTokens, "champion_current_rod", "fishing_champion"
	case rank > 1 && rank <= 3:
		return fishingTournamentPodiumTokens, "tournament_current_rod", ""
	case rank > 3 && rank <= 10:
		return fishingTournamentTopTenTokens, "", ""
	default:
		// Four weekly participations earn the 200 tokens needed for the
		// catch-up rod, while a podium finish grants the same function now.
		return fishingTournamentParticipantTokens, "", ""
	}
}

func (s *Store) GetFishingTournamentOverview(ctx context.Context, userID int64, now time.Time) (FishingTournamentOverview, error) {
	if now.IsZero() {
		now = time.Now()
	}
	out := FishingTournamentOverview{}
	err := s.withTx(ctx, func(tx pgx.Tx) error {
		if err := s.finalizeFishingTournamentsTx(ctx, tx, now); err != nil {
			return err
		}
		current, err := s.ensureFishingTournamentTx(ctx, tx, now)
		if err != nil {
			return err
		}
		out.Current = current
		if err := s.repairFishingTournamentEntryIntegrityTx(ctx, tx, current); err != nil {
			return err
		}
		if fishingTournamentUsesStandardLoadout(current) {
			standard, err := s.validatedFishingTournamentStandardLoadout()
			if err != nil {
				return err
			}
			out.StandardLoadout = &standard
		} else {
			legacyRules := fishingStandardTournamentRules()
			out.StandardRules = &legacyRules
		}
		out.CatchUpCost = fishingTournamentCatchUpCost
		out.OpenLeaderboard, err = s.listFishingTournamentBoardTx(ctx, tx, current.ID, fishingTournamentDivisionOpen, 30)
		if err != nil {
			return err
		}
		out.StandardLeaderboard, err = s.listFishingTournamentBoardTx(ctx, tx, current.ID, fishingTournamentDivisionStandard, 30)
		if err != nil {
			return err
		}
		out.MyEntries, err = s.listFishingTournamentUserEntriesTx(ctx, tx, current.ID, userID)
		if err != nil {
			return err
		}
		rewardRows, err := tx.Query(ctx, `
			SELECT tournament_id, division, rank, token_amount, item_id, cosmetic_key, claimed_at
			FROM fishing_tournament_rewards
			WHERE user_id = $1
			ORDER BY created_at DESC
			LIMIT 20
		`, userID)
		if err != nil {
			return err
		}
		for rewardRows.Next() {
			var row FishingTournamentRewardView
			if err := rewardRows.Scan(&row.TournamentID, &row.Division, &row.Rank, &row.TokenAmount, &row.ItemID, &row.CosmeticKey, &row.ClaimedAt); err != nil {
				rewardRows.Close()
				return err
			}
			out.Rewards = append(out.Rewards, row)
		}
		rewardRows.Close()
		if err := rewardRows.Err(); err != nil {
			return err
		}
		if err := tx.QueryRow(ctx, `SELECT COALESCE(tokens, 0) FROM fishing_player_wallets WHERE user_id = $1`, userID).Scan(&out.Tokens); err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		var eligible bool
		if err := tx.QueryRow(ctx, `
				SELECT EXISTS (
					SELECT 1
					FROM fishing_tournament_rewards r
					JOIN fishing_tournaments t ON t.id = r.tournament_id
					WHERE r.user_id = $1 AND r.claimed_at IS NOT NULL AND t.ends_at <= $2
				) AND NOT EXISTS (
					SELECT 1 FROM fishing_token_redemptions
					WHERE user_id = $1 AND reward_key = 'tournament_current_rod'
				)
			`, userID, now.Add(-fishingTournamentCatchUpWait)).Scan(&eligible); err != nil {
			return err
		}
		out.CatchUpAvailable = eligible && out.Tokens >= fishingTournamentCatchUpCost
		return nil
	})
	return out, err
}

func (s *Store) repairFishingTournamentEntryIntegrityTx(ctx context.Context, tx pgx.Tx, tournament FishingTournament) error {
	if !fishingTournamentNeedsEntryIntegrityRepair(tournament) {
		return nil
	}

	var hasInvalidCatchEntry bool
	if err := tx.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM fishing_tournament_entries e
			LEFT JOIN fishing_catches c ON c.id = e.catch_id
			WHERE e.tournament_id = $1
			  AND e.source_kind = 'catch'
			  AND (
				e.catch_id IS NULL
				OR c.id IS NULL
				OR c.user_id IS DISTINCT FROM e.user_id
				OR c.species_id IS DISTINCT FROM $2
				OR c.fish_data_jsonb->>'speciesId' IS DISTINCT FROM $2
				OR c.caught_at < $3
				OR c.caught_at >= $4
			  )
		)
	`, tournament.ID, tournament.TargetSpeciesID, tournament.StartsAt, tournament.EndsAt).Scan(&hasInvalidCatchEntry); err != nil {
		return err
	}
	if !hasInvalidCatchEntry {
		return nil
	}

	species := s.gameData.FishingSpeciesByID(tournament.TargetSpeciesID)
	if species == nil {
		return ErrFishingCatchNotFound
	}
	rows, err := tx.Query(ctx, `
		SELECT id, user_id, fish_data_jsonb, caught_at
		FROM fishing_catches
		WHERE species_id = $1
		  AND caught_at >= $2
		  AND caught_at < $3
		ORDER BY user_id ASC, caught_at ASC, id ASC
	`, tournament.TargetSpeciesID, tournament.StartsAt, tournament.EndsAt)
	if err != nil {
		return err
	}
	bestByUser := make(map[int64]*fishingTournamentRepairBest)
	for rows.Next() {
		var userID int64
		var fishRaw []byte
		candidate := &fishingTournamentCatchCandidate{}
		if err := rows.Scan(&candidate.CatchID, &userID, &fishRaw, &candidate.CaughtAt); err != nil {
			rows.Close()
			return err
		}
		if err := json.Unmarshal(fishRaw, &candidate.Fish); err != nil {
			rows.Close()
			return err
		}
		if candidate.Fish.SpeciesID != tournament.TargetSpeciesID || candidate.Fish.LengthCM <= 0 || candidate.Fish.WeightKG <= 0 {
			rows.Close()
			return ErrFishingArgsInvalid
		}
		best := bestByUser[userID]
		if best == nil {
			best = &fishingTournamentRepairBest{}
			bestByUser[userID] = best
		}
		if betterFishingTournamentRepairCatch(candidate, best.Open) {
			best.Open = candidate
		}
		rulesetVersion := strings.TrimSpace(candidate.Fish.RulesetVersion)
		if rulesetVersion == "" {
			rulesetVersion = FishingRulesetVersion
		}
		if fishingStandardTournamentQualifiesForRuleset(tournament.ID, candidate.CatchID, *species, candidate.Fish, rulesetVersion) &&
			betterFishingTournamentRepairCatch(candidate, best.Standard) {
			best.Standard = candidate
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}

	if _, err := tx.Exec(ctx, `
		DELETE FROM fishing_tournament_entries
		WHERE tournament_id = $1 AND source_kind = 'catch'
	`, tournament.ID); err != nil {
		return err
	}
	userIDs := make([]int64, 0, len(bestByUser))
	for userID := range bestByUser {
		userIDs = append(userIDs, userID)
	}
	sort.Slice(userIDs, func(i, j int) bool { return userIDs[i] < userIDs[j] })
	for _, userID := range userIDs {
		best := bestByUser[userID]
		for _, entry := range []struct {
			Division string
			Catch    *fishingTournamentCatchCandidate
		}{
			{Division: fishingTournamentDivisionOpen, Catch: best.Open},
			{Division: fishingTournamentDivisionStandard, Catch: best.Standard},
		} {
			if entry.Catch == nil {
				continue
			}
			if err := s.upsertFishingTournamentCatchEntryTx(
				ctx,
				tx,
				userID,
				tournament.ID,
				entry.Division,
				entry.Catch.CatchID,
			); err != nil {
				return err
			}
		}
	}
	return nil
}

func fishingTournamentNeedsEntryIntegrityRepair(tournament FishingTournament) bool {
	return tournament.WeekKey == fishingTournamentEntryRepairWeek &&
		tournament.TargetSpeciesID == fishingTournamentEntryRepairTarget &&
		tournament.RulesetVersion == FishingRulesetVersion &&
		!fishingTournamentUsesStandardLoadout(tournament)
}

func betterFishingTournamentRepairCatch(candidate, current *fishingTournamentCatchCandidate) bool {
	if candidate == nil {
		return false
	}
	if current == nil {
		return true
	}
	return candidate.Fish.WeightKG > current.Fish.WeightKG ||
		(candidate.Fish.WeightKG == current.Fish.WeightKG && candidate.Fish.LengthCM > current.Fish.LengthCM)
}

func (s *Store) listFishingTournamentUserEntriesTx(ctx context.Context, tx pgx.Tx, tournamentID string, userID int64) ([]FishingTournamentEntryView, error) {
	rows, err := tx.Query(ctx, `
		SELECT ranked.rank, e.division, e.user_id, COALESCE(u.username, ''),
		       COALESCE(e.catch_id, 0), COALESCE(e.fish_instance_id, 0), e.source_kind,
		       e.length_cm, e.weight_kg
		FROM fishing_tournament_entries e
		JOIN users u ON u.id = e.user_id
		CROSS JOIN LATERAL (
			SELECT 1 + COUNT(*) AS rank
			FROM fishing_tournament_entries ahead
			WHERE ahead.tournament_id = e.tournament_id
			  AND ahead.division = e.division
			  AND (
				ahead.weight_kg > e.weight_kg
				OR (ahead.weight_kg = e.weight_kg AND ahead.length_cm > e.length_cm)
				OR (ahead.weight_kg = e.weight_kg AND ahead.length_cm = e.length_cm AND ahead.submitted_at < e.submitted_at)
				OR (ahead.weight_kg = e.weight_kg AND ahead.length_cm = e.length_cm AND ahead.submitted_at = e.submitted_at AND ahead.user_id < e.user_id)
			  )
		) ranked
		WHERE e.tournament_id = $1 AND e.user_id = $2
		ORDER BY e.division
	`, tournamentID, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]FishingTournamentEntryView, 0, 2)
	for rows.Next() {
		var row FishingTournamentEntryView
		if err := rows.Scan(&row.Rank, &row.Division, &row.UserID, &row.PlayerName, &row.CatchID, &row.FishInstanceID, &row.SourceKind, &row.LengthCM, &row.WeightKG); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (s *Store) listFishingTournamentBoardTx(ctx context.Context, tx pgx.Tx, tournamentID, division string, limit int) ([]FishingTournamentEntryView, error) {
	rows, err := tx.Query(ctx, `
		SELECT ROW_NUMBER() OVER (ORDER BY e.weight_kg DESC, e.length_cm DESC, e.submitted_at ASC, e.user_id ASC),
		       e.division, e.user_id, COALESCE(u.username, ''),
		       COALESCE(e.catch_id, 0), COALESCE(e.fish_instance_id, 0), e.source_kind,
		       e.length_cm, e.weight_kg
		FROM fishing_tournament_entries e
		JOIN users u ON u.id = e.user_id
		WHERE e.tournament_id = $1 AND e.division = $2
		ORDER BY e.weight_kg DESC, e.length_cm DESC, e.submitted_at ASC, e.user_id ASC
		LIMIT $3
	`, tournamentID, division, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]FishingTournamentEntryView, 0, limit)
	for rows.Next() {
		var row FishingTournamentEntryView
		if err := rows.Scan(&row.Rank, &row.Division, &row.UserID, &row.PlayerName, &row.CatchID, &row.FishInstanceID, &row.SourceKind, &row.LengthCM, &row.WeightKG); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (s *Store) SubmitFishingPondTournamentFish(ctx context.Context, userID, instanceID int64, now time.Time) (FishingTournamentEntryView, error) {
	if instanceID <= 0 {
		return FishingTournamentEntryView{}, ErrFishingTournamentEntryUnavailable
	}
	if now.IsZero() {
		now = time.Now()
	}
	var out FishingTournamentEntryView
	err := s.withTx(ctx, func(tx pgx.Tx) error {
		if err := s.lockUserTx(ctx, tx, userID); err != nil {
			return err
		}
		level, err := s.getPlayerBuildingLevelTx(ctx, tx, userID, FishingPondBuildingID)
		if err != nil {
			return err
		}
		if level <= 0 {
			return ErrFishingPondNotBuilt
		}
		if _, err := s.settleFishingPondTx(ctx, tx, userID, level, now); err != nil {
			return err
		}
		var raw []byte
		var state string
		if err := tx.QueryRow(ctx, `
			SELECT state, COALESCE(fish_data, '{}'::jsonb)
			FROM player_item_instances
			WHERE user_id=$1 AND instance_id=$2 AND template_item_id='fish_catch'
			  AND state IN ('inventory', 'fishing_pond') AND quantity=1
			FOR UPDATE
		`, userID, instanceID).Scan(&state, &raw); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrFishingTournamentEntryUnavailable
			}
			return err
		}
		var fish FishInstanceData
		if json.Unmarshal(raw, &fish) != nil || fish.Origin != "cultured" || normalizePondLifeState(fish.LifeState) != "alive" {
			return ErrFishingTournamentEntryUnavailable
		}
		if state == fishingPondState && normalizeFishingPondZone(fish.PondZone) == fishingPondZoneNursery {
			return ErrFishingTournamentEntryUnavailable
		}
		profile := s.gameData.FishingPondSpeciesByID(fish.SpeciesID)
		if !fishingPondTournamentEligible(fish, profile) {
			return ErrFishingTournamentEntryUnavailable
		}
		tournament, err := s.ensureFishingTournamentTx(ctx, tx, now)
		if err != nil {
			return err
		}
		if tournament.Status != "open" || !now.Before(tournament.EndsAt) || tournament.TargetSpeciesID != fish.SpeciesID {
			return ErrFishingTournamentEntryUnavailable
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO fishing_tournament_entries(
				tournament_id, user_id, division, catch_id, fish_instance_id, source_kind,
				length_cm, weight_kg, submitted_at
			)
			VALUES($1, $2, 'open', NULL, $3, 'cultured', $4, $5, $6)
			ON CONFLICT (tournament_id, user_id, division) DO UPDATE
			SET catch_id=NULL, fish_instance_id=EXCLUDED.fish_instance_id, source_kind='cultured',
			    length_cm=EXCLUDED.length_cm, weight_kg=EXCLUDED.weight_kg, submitted_at=EXCLUDED.submitted_at
			WHERE EXCLUDED.weight_kg > fishing_tournament_entries.weight_kg
			   OR (EXCLUDED.weight_kg = fishing_tournament_entries.weight_kg AND EXCLUDED.length_cm > fishing_tournament_entries.length_cm)
		`, tournament.ID, userID, instanceID, fish.LengthCM, roundFloat(fish.WeightKG, 3), now); err != nil {
			return err
		}
		if state == fishingPondState {
			if err := s.addFishingPondEventTx(ctx, tx, userID, "tournament_submitted", &instanceID, nil, map[string]any{"speciesId": fish.SpeciesID, "tournamentId": tournament.ID}); err != nil {
				return err
			}
		}
		rows, err := s.listFishingTournamentUserEntriesTx(ctx, tx, tournament.ID, userID)
		if err != nil {
			return err
		}
		for _, row := range rows {
			if row.Division == "open" {
				out = row
				return nil
			}
		}
		return ErrFishingTournamentEntryUnavailable
	})
	return out, err
}

func fishingPondTournamentEligible(fish FishInstanceData, profile *catalog.FishingPondSpecies) bool {
	return profile != nil && fish.Origin == "cultured" && normalizeFishingPondZone(fish.PondZone) == fishingPondZoneMain && normalizePondLifeState(fish.LifeState) == "alive" &&
		fish.AgeDays >= profile.MaturityAgeDays && fish.Health >= 0.85 && fish.Condition >= 0.9 && fish.Satiety >= 80
}

func (s *Store) ClaimFishingTournamentReward(ctx context.Context, userID int64, tournamentID, division string) (FishingTournamentRewardView, error) {
	tournamentID = strings.TrimSpace(tournamentID)
	division = strings.ToLower(strings.TrimSpace(division))
	if tournamentID == "" || (division != "open" && division != "standard") {
		return FishingTournamentRewardView{}, ErrFishingRewardUnavailable
	}
	var out FishingTournamentRewardView
	err := s.withTx(ctx, func(tx pgx.Tx) error {
		if err := s.lockUserTx(ctx, tx, userID); err != nil {
			return err
		}
		if err := tx.QueryRow(ctx, `
			SELECT tournament_id, division, rank, token_amount, item_id, cosmetic_key, claimed_at
			FROM fishing_tournament_rewards
			WHERE tournament_id = $1 AND user_id = $2 AND division = $3
			FOR UPDATE
		`, tournamentID, userID, division).Scan(&out.TournamentID, &out.Division, &out.Rank, &out.TokenAmount, &out.ItemID, &out.CosmeticKey, &out.ClaimedAt); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrFishingRewardUnavailable
			}
			return err
		}
		if out.ClaimedAt != nil {
			return nil
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO fishing_player_wallets(user_id, tokens, updated_at)
			VALUES($1, $2, now())
			ON CONFLICT (user_id) DO UPDATE
			SET tokens = fishing_player_wallets.tokens + EXCLUDED.tokens, updated_at = now()
		`, userID, out.TokenAmount); err != nil {
			return err
		}
		if out.ItemID != "" {
			if err := s.AddInventoryItemTx(ctx, tx, userID, out.ItemID, 1); err != nil {
				return err
			}
		}
		if out.CosmeticKey != "" {
			if _, err := s.GrantCosmeticTx(ctx, tx, userID, CosmeticGrant{
				Category: CosmeticCategoryTitlePrefix,
				ItemID:   out.CosmeticKey,
				Source:   "fishing_tournament:" + tournamentID,
			}); err != nil {
				return err
			}
		}
		claimedAt := time.Now()
		if _, err := tx.Exec(ctx, `
			UPDATE fishing_tournament_rewards SET claimed_at = $4
			WHERE tournament_id = $1 AND user_id = $2 AND division = $3 AND claimed_at IS NULL
		`, tournamentID, userID, division, claimedAt); err != nil {
			return err
		}
		out.ClaimedAt = &claimedAt
		return nil
	})
	return out, err
}

func (s *Store) RedeemFishingCatchUpRod(ctx context.Context, userID int64, now time.Time) error {
	if now.IsZero() {
		now = time.Now()
	}
	return s.withTx(ctx, func(tx pgx.Tx) error {
		if err := s.lockUserTx(ctx, tx, userID); err != nil {
			return err
		}
		var eligible bool
		if err := tx.QueryRow(ctx, `
			SELECT EXISTS (
				SELECT 1 FROM fishing_tournament_rewards r
				JOIN fishing_tournaments t ON t.id = r.tournament_id
				WHERE r.user_id = $1 AND r.claimed_at IS NOT NULL AND t.ends_at <= $2
			)
		`, userID, now.Add(-fishingTournamentCatchUpWait)).Scan(&eligible); err != nil {
			return err
		}
		if !eligible {
			return ErrFishingRewardUnavailable
		}
		var tokens int64
		if err := tx.QueryRow(ctx, `SELECT tokens FROM fishing_player_wallets WHERE user_id = $1 FOR UPDATE`, userID).Scan(&tokens); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrFishingRewardUnavailable
			}
			return err
		}
		if tokens < fishingTournamentCatchUpCost {
			return ErrFishingRewardUnavailable
		}
		var already bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM fishing_token_redemptions WHERE user_id = $1 AND reward_key = 'tournament_current_rod')`, userID).Scan(&already); err != nil {
			return err
		}
		if already {
			return ErrFishingRewardUnavailable
		}
		if _, err := tx.Exec(ctx, `UPDATE fishing_player_wallets SET tokens = tokens - $2, updated_at = now() WHERE user_id = $1`, userID, fishingTournamentCatchUpCost); err != nil {
			return err
		}
		if err := s.AddInventoryItemTx(ctx, tx, userID, "tournament_current_rod", 1); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `
			INSERT INTO fishing_token_redemptions(user_id, reward_key, token_cost, redeemed_at)
			VALUES($1, 'tournament_current_rod', $2, now())
		`, userID, fishingTournamentCatchUpCost)
		return err
	})
}
