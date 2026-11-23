package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

const (
	CompetitiveMaxKittyCount     = 2
	ArenaMaxKittyCount           = CompetitiveMaxKittyCount
	ArenaBattleKind              = "arena"
	ArenaChallengeStatusQueued   = "queued"
	ArenaChallengeStatusRunning  = "running"
	ArenaChallengeAttackerWin    = "attacker_win"
	ArenaChallengeDefenderWin    = "defender_win"
	ArenaChallengeStatusFailed   = "failed"
	ArenaChallengeHistoryMaxRead = 50
	ArenaFormatFilterAll         = "all"
)

const listArenaTargetsQuery = `
	SELECT d.user_id,d.revision,d.active,d.battle_level,d.kitty_ids,d.selection_jsonb,d.lineup_jsonb,d.lineup_digest,d.bounty_tokens,d.successful_defenses,d.challenge_count,d.published_at,d.updated_at,u.username,COALESCE(ps.rating,$3::bigint)
	FROM arena_defenses d
	JOIN users u ON u.id=d.user_id
	LEFT JOIN arena_player_states ps ON ps.user_id=d.user_id
	WHERE d.active=TRUE AND d.user_id<>$1
		AND d.selection_jsonb->>'eligibilityPolicyVersion'=$8::text
		AND (
			$7::text='all'
			OR d.selection_jsonb->>'formatId'=$7::text
		)
	ORDER BY CASE WHEN $6::boolean THEN ABS(d.battle_level-$2::bigint) ELSE 0 END ASC,
		COALESCE(ps.rating,$3::bigint) DESC,d.user_id ASC
	LIMIT $4 OFFSET $5
`

const listArenaMoreTargetsQuery = `
	WITH forward AS (
		SELECT d.user_id,0::int AS sample_segment
		FROM arena_defenses d
		WHERE d.active=TRUE AND d.user_id<>$1::bigint AND d.user_id>=$2::bigint
			AND d.selection_jsonb->>'eligibilityPolicyVersion'=$7::text
		ORDER BY d.user_id ASC
		LIMIT $3::int
	), wrapped AS (
		SELECT d.user_id,1::int AS sample_segment
		FROM arena_defenses d
		WHERE d.active=TRUE AND d.user_id<>$1::bigint AND d.user_id<$2::bigint
			AND d.selection_jsonb->>'eligibilityPolicyVersion'=$7::text
		ORDER BY d.user_id ASC
		LIMIT $3::int
	), sampled_ids AS (
		SELECT candidate.user_id
		FROM (
			SELECT user_id,sample_segment FROM forward
			UNION ALL
			SELECT user_id,sample_segment FROM wrapped
		) candidate
		ORDER BY candidate.sample_segment ASC,candidate.user_id ASC
		LIMIT $3::int
	)
	SELECT d.user_id,d.revision,d.active,d.battle_level,d.kitty_ids,d.selection_jsonb,d.lineup_jsonb,d.lineup_digest,d.bounty_tokens,d.successful_defenses,d.challenge_count,d.published_at,d.updated_at,u.username,COALESCE(ps.rating,$5::bigint)
	FROM sampled_ids sample
	JOIN arena_defenses d ON d.user_id=sample.user_id
	JOIN users u ON u.id=d.user_id
	LEFT JOIN arena_player_states ps ON ps.user_id=d.user_id
	WHERE $6::text='all'
		OR d.selection_jsonb->>'formatId'=$6::text
	ORDER BY md5(d.user_id::text || ':' || ($1::bigint)::text || ':' || $4::text) ASC,d.user_id ASC
`

const maxArenaActiveDefenseUserIDQuery = `
	SELECT COALESCE(MAX(user_id),0)
	FROM arena_defenses
	WHERE active=TRUE
`

const arenaChallengeOpponentCooldownsQuery = `
	WITH clock AS (SELECT clock_timestamp() AS server_time)
	SELECT clock.server_time,c.defender_id,max(c.created_at)+($3::bigint*interval '1 second')
	FROM arena_challenges c
	CROSS JOIN clock
	WHERE $3::bigint>0
		AND c.attacker_id=$1::bigint
		AND c.defender_id=ANY($2::bigint[])
		AND c.status<>'failed'
		AND c.created_at>clock.server_time-($3::bigint*interval '1 second')
	GROUP BY clock.server_time,c.defender_id
`

const listArenaLeaderboardQuery = `
	WITH eligible AS (
		SELECT ps.user_id,u.username,ps.rating,
			ps.attack_wins,ps.attack_losses,ps.defense_wins,ps.defense_losses,
			COALESCE(d.active,FALSE) AS active_defense,
			CASE WHEN d.active=TRUE THEN d.battle_level ELSE 0 END AS battle_level
		FROM arena_player_states ps
		JOIN users u ON u.id=ps.user_id
		LEFT JOIN arena_defenses d ON d.user_id=ps.user_id
		WHERE COALESCE(d.active,FALSE)
			OR ps.attack_wins+ps.attack_losses+ps.defense_wins+ps.defense_losses>0
	), ranked AS (
		SELECT row_number() OVER (
				ORDER BY rating DESC,
					(attack_wins+defense_wins) DESC,
					(attack_losses+defense_losses) ASC,
					user_id ASC
			)::bigint AS rank,
			count(*) OVER ()::bigint AS total,
			user_id,username,rating,attack_wins,attack_losses,defense_wins,defense_losses,active_defense,battle_level
		FROM eligible
	), page AS (
		SELECT * FROM ranked ORDER BY rank LIMIT $1::int OFFSET $2::int
	), viewer AS (
		SELECT * FROM ranked
		WHERE user_id=$3::bigint
			AND NOT EXISTS(SELECT 1 FROM page WHERE user_id=$3::bigint)
	)
	SELECT rank,total,user_id,username,rating,attack_wins,attack_losses,defense_wins,defense_losses,active_defense,battle_level,FALSE AS self_only
	FROM page
	UNION ALL
	SELECT rank,total,user_id,username,rating,attack_wins,attack_losses,defense_wins,defense_losses,active_defense,battle_level,TRUE AS self_only
	FROM viewer
	ORDER BY self_only ASC,rank ASC
`

const adminResetArenaLeaderboardQuery = `
	WITH locked AS MATERIALIZED (
		SELECT user_id
		FROM arena_player_states
		ORDER BY user_id ASC
		FOR UPDATE
	), reset AS (
		UPDATE arena_player_states ps
		SET rating=$1::bigint,
			attack_wins=0,
			attack_losses=0,
			defense_wins=0,
			defense_losses=0,
			updated_at=now()
		FROM locked
		WHERE ps.user_id=locked.user_id
		RETURNING ps.user_id
	)
	SELECT count(*)::bigint,clock_timestamp()
	FROM reset
`

const listArenaDefenseInitializationCandidatesQuery = `
	SELECT p.user_id
	FROM player_presence p
	WHERE p.last_seen_at >= $1
		AND p.user_id > $2::bigint
		AND NOT EXISTS (
			SELECT 1
			FROM arena_defenses d
			WHERE d.user_id = p.user_id
		)
	ORDER BY p.user_id ASC
	LIMIT $3::int
`

const createArenaDefenseIfAbsentQuery = `
	INSERT INTO arena_defenses(user_id,revision,active,battle_level,kitty_ids,selection_jsonb,lineup_jsonb,lineup_digest,published_at,updated_at)
	VALUES($1,1,TRUE,$2,$3,$4,$5,$6,now(),now())
	ON CONFLICT(user_id) DO NOTHING
	RETURNING user_id,revision,active,battle_level,kitty_ids,selection_jsonb,lineup_jsonb,lineup_digest,bounty_tokens,successful_defenses,challenge_count,published_at,updated_at
`

type ArenaPlayerState struct {
	UserID                     int64     `json:"userId"`
	Rating                     int64     `json:"rating"`
	Tokens                     int64     `json:"tokens"`
	AttackWins                 int64     `json:"attackWins"`
	AttackLosses               int64     `json:"attackLosses"`
	DefenseWins                int64     `json:"defenseWins"`
	DefenseLosses              int64     `json:"defenseLosses"`
	ChallengeCharges           int       `json:"-"`
	ChallengeChargeRefreshedAt time.Time `json:"-"`
	UpdatedAt                  time.Time `json:"updatedAt"`
}

type ArenaDefense struct {
	UserID             int64           `json:"userId"`
	Revision           int64           `json:"revision"`
	Active             bool            `json:"active"`
	BattleLevel        int64           `json:"battleLevel"`
	KittyIDs           []int64         `json:"kittyIds"`
	Selection          json.RawMessage `json:"selection"`
	Lineup             json.RawMessage `json:"lineup"`
	LineupDigest       string          `json:"lineupDigest"`
	BountyTokens       int64           `json:"bountyTokens"`
	SuccessfulDefenses int64           `json:"successfulDefenses"`
	ChallengeCount     int64           `json:"challengeCount"`
	PublishedAt        time.Time       `json:"publishedAt"`
	UpdatedAt          time.Time       `json:"updatedAt"`
}

type ArenaTarget struct {
	ArenaDefense
	Username string `json:"username"`
	Rating   int64  `json:"rating"`
}

type ArenaOpponentCooldownState struct {
	ServerTime            time.Time
	OpponentUntilByUserID map[int64]time.Time
}

type ArenaChallengeChargePolicy struct {
	Capacity int
	Recovery time.Duration
}

type ArenaChallengeChargeState struct {
	ServerTime      time.Time
	Remaining       int
	Capacity        int
	RecoverySeconds int64
	NextRecoveryAt  *time.Time
}

type ArenaLeaderboardEntry struct {
	Rank          int64  `json:"rank"`
	UserID        int64  `json:"userId"`
	Username      string `json:"username"`
	Rating        int64  `json:"rating"`
	AttackWins    int64  `json:"attackWins"`
	AttackLosses  int64  `json:"attackLosses"`
	DefenseWins   int64  `json:"defenseWins"`
	DefenseLosses int64  `json:"defenseLosses"`
	ActiveDefense bool   `json:"activeDefense"`
	BattleLevel   int64  `json:"battleLevel"`
}

type ArenaLeaderboardPage struct {
	Entries []ArenaLeaderboardEntry
	Self    *ArenaLeaderboardEntry
	Total   int64
}

type ArenaLeaderboardResetResult struct {
	ResetPlayers  int64     `json:"resetPlayers"`
	DefaultRating int64     `json:"defaultRating"`
	ResetAt       time.Time `json:"resetAt"`
}

type ArenaChallenge struct {
	ID                   int64           `json:"id"`
	BattleKind           string          `json:"battleKind"`
	AttackerID           int64           `json:"attackerId"`
	DefenderID           int64           `json:"defenderId"`
	OpponentID           int64           `json:"opponentId,omitempty"`
	OpponentName         string          `json:"opponentName,omitempty"`
	Role                 string          `json:"role,omitempty"`
	DefenseRevision      int64           `json:"defenseRevision"`
	Seed                 int64           `json:"seed"`
	Status               string          `json:"status"`
	Outcome              string          `json:"outcome"`
	WorkerID             string          `json:"-"`
	Attempts             int             `json:"attempts"`
	AttackerLineup       json.RawMessage `json:"attackerLineup,omitempty"`
	DefenderLineup       json.RawMessage `json:"defenderLineup,omitempty"`
	FormatSnapshot       json.RawMessage `json:"-"`
	RatingEligible       bool            `json:"ratingEligible"`
	AttackerRewardTokens int64           `json:"attackerRewardTokens"`
	DefenderRewardTokens int64           `json:"defenderRewardTokens"`
	BountyRewardTokens   int64           `json:"bountyRewardTokens"`
	AttackerRatingDelta  int64           `json:"attackerRatingDelta"`
	DefenderRatingDelta  int64           `json:"defenderRatingDelta"`
	CreatedAt            time.Time       `json:"createdAt"`
	StartedAt            *time.Time      `json:"startedAt,omitempty"`
	CompletedAt          *time.Time      `json:"completedAt,omitempty"`
}

type ArenaChallengePayload struct {
	ChallengeID  int64           `json:"challengeId"`
	BattleKind   string          `json:"battleKind"`
	Seed         int64           `json:"seed"`
	Status       string          `json:"status"`
	Outcome      string          `json:"outcome"`
	InitialState json.RawMessage `json:"initialState"`
	Timeline     json.RawMessage `json:"timeline"`
	FinalResult  json.RawMessage `json:"finalResult"`
}

type ArenaReplayShare struct {
	Token         string    `json:"token"`
	ChallengeID   int64     `json:"challengeId"`
	CreatedBy     int64     `json:"createdBy"`
	CreatedByName string    `json:"createdByName"`
	Outcome       string    `json:"outcome"`
	CreatedAt     time.Time `json:"createdAt"`
}

type ArenaSharedReplay struct {
	Share   ArenaReplayShare      `json:"share"`
	Payload ArenaChallengePayload `json:"payload"`
}

type PublishArenaDefenseInput struct {
	UserID       int64
	BattleLevel  int64
	KittyIDs     []int64
	Selection    json.RawMessage
	Lineup       json.RawMessage
	LineupDigest string
}

type CreateArenaChallengeInput struct {
	AttackerID              int64
	DefenderID              int64
	ExpectedDefenseRevision int64
	Seed                    int64
	AttackerLineup          json.RawMessage
	FormatSnapshot          json.RawMessage
	RatingEligible          bool
	DefaultRating           int64
	ChargePolicy            ArenaChallengeChargePolicy
	OpponentCooldown        time.Duration
}

type ArenaSettlementPolicy struct {
	DefaultRating             int64
	AttackerWinTokens         int64
	AttackerLossTokens        int64
	DefenderWinTokens         int64
	BountyPerDefenseWin       int64
	BountyCap                 int64
	AttackerWinRating         int64
	AttackerLossRatingPenalty int64
	DefenderWinRating         int64
	DefenderLossRatingPenalty int64
}

type CompleteArenaChallengeInput struct {
	ChallengeID  int64
	WorkerID     string
	Outcome      string
	InitialState json.RawMessage
	Timeline     json.RawMessage
	FinalResult  json.RawMessage
	Policy       ArenaSettlementPolicy
}

type ArenaSettlementResult struct {
	ChallengeID          int64 `json:"challengeId"`
	AttackerID           int64 `json:"attackerId"`
	DefenderID           int64 `json:"defenderId"`
	RatingEligible       bool  `json:"ratingEligible"`
	AttackerRewardTokens int64 `json:"attackerRewardTokens"`
	DefenderRewardTokens int64 `json:"defenderRewardTokens"`
	BountyRewardTokens   int64 `json:"bountyRewardTokens"`
	AttackerRatingDelta  int64 `json:"attackerRatingDelta"`
	DefenderRatingDelta  int64 `json:"defenderRatingDelta"`
}

type ArenaShopPurchaseResult struct {
	OrderID   int64 `json:"orderId"`
	Tokens    int64 `json:"tokens"`
	Duplicate bool  `json:"duplicate"`
}

func arenaSettlementRatingDeltas(outcome string, ratingEligible bool, policy ArenaSettlementPolicy) (int64, int64) {
	if !ratingEligible {
		return 0, 0
	}
	if outcome == ArenaChallengeAttackerWin {
		return policy.AttackerWinRating, -policy.DefenderLossRatingPenalty
	}
	if outcome == ArenaChallengeDefenderWin {
		return -policy.AttackerLossRatingPenalty, policy.DefenderWinRating
	}
	return 0, 0
}

func validArenaKittyIDs(kittyIDs []int64) bool {
	if len(kittyIDs) > ArenaMaxKittyCount {
		return false
	}
	seen := make(map[int64]struct{}, len(kittyIDs))
	for _, kittyID := range kittyIDs {
		if kittyID <= 0 {
			return false
		}
		if _, exists := seen[kittyID]; exists {
			return false
		}
		seen[kittyID] = struct{}{}
	}
	return true
}

func normalizeArenaHistoryKeepCount(keepCount int) int {
	if keepCount <= 0 || keepCount > ArenaChallengeHistoryMaxRead {
		return ArenaChallengeHistoryMaxRead
	}
	return keepCount
}

func (s *Store) GetPlayerBaseBattleLevel(ctx context.Context, userID int64) (int64, error) {
	var level int64
	err := s.db.QueryRow(ctx, `SELECT base_level FROM player_attributes WHERE user_id=$1 AND attr_key='battle'`, userID).Scan(&level)
	if errors.Is(err, pgx.ErrNoRows) {
		return 1, nil
	}
	if err != nil {
		return 0, err
	}
	return maxInt64(1, level), nil
}

func (s *Store) GetOwnedKittyPositionLock(ctx context.Context, userID, kittyID int64) (string, string, string, error) {
	if userID <= 0 || kittyID <= 0 {
		return "", "", "", ErrKittyNotFound
	}
	if _, err := s.GetPlayerKitty(ctx, userID, kittyID); err != nil {
		return "", "", "", err
	}
	return s.kittyPositionLock(ctx, userID, kittyID)
}

func (s *Store) GetArenaKittyPositionLock(ctx context.Context, userID, kittyID int64) (string, string, string, error) {
	return s.GetOwnedKittyPositionLock(ctx, userID, kittyID)
}

func validArenaChallengeChargePolicy(policy ArenaChallengeChargePolicy) bool {
	return policy.Capacity > 0 && policy.Recovery > 0
}

func arenaChallengeChargeRecoverySeconds(recovery time.Duration) int64 {
	if recovery <= 0 {
		return 0
	}
	seconds := recovery / time.Second
	if recovery%time.Second != 0 {
		seconds++
	}
	return int64(seconds)
}

func normalizeArenaChallengeCharges(charges int, refreshedAt, serverTime time.Time, policy ArenaChallengeChargePolicy) (int, time.Time, error) {
	if charges < 0 || refreshedAt.IsZero() || serverTime.IsZero() || !validArenaChallengeChargePolicy(policy) {
		return 0, time.Time{}, ErrBattleRoomConfigInvalid
	}
	if charges >= policy.Capacity {
		return policy.Capacity, refreshedAt, nil
	}
	if refreshedAt.After(serverTime) {
		refreshedAt = serverTime
	}
	recovered := int(serverTime.Sub(refreshedAt) / policy.Recovery)
	if recovered <= 0 {
		return charges, refreshedAt, nil
	}
	if recovered >= policy.Capacity-charges {
		return policy.Capacity, serverTime, nil
	}
	return charges + recovered, refreshedAt.Add(time.Duration(recovered) * policy.Recovery), nil
}

func arenaChallengeChargeState(charges int, refreshedAt, serverTime time.Time, policy ArenaChallengeChargePolicy) ArenaChallengeChargeState {
	state := ArenaChallengeChargeState{
		ServerTime:      serverTime,
		Remaining:       charges,
		Capacity:        policy.Capacity,
		RecoverySeconds: arenaChallengeChargeRecoverySeconds(policy.Recovery),
	}
	if charges < policy.Capacity {
		next := refreshedAt.Add(policy.Recovery)
		state.NextRecoveryAt = &next
	}
	return state
}

func consumeArenaChallengeCharge(charges int, refreshedAt, serverTime time.Time, policy ArenaChallengeChargePolicy) (int, time.Time, ArenaChallengeChargeState, error) {
	charges, refreshedAt, err := normalizeArenaChallengeCharges(charges, refreshedAt, serverTime, policy)
	if err != nil {
		return 0, time.Time{}, ArenaChallengeChargeState{}, err
	}
	if charges <= 0 {
		return charges, refreshedAt, arenaChallengeChargeState(charges, refreshedAt, serverTime, policy), ErrArenaChallengeChargesExhausted
	}
	if charges >= policy.Capacity {
		refreshedAt = serverTime
	}
	charges--
	return charges, refreshedAt, arenaChallengeChargeState(charges, refreshedAt, serverTime, policy), nil
}

func refundArenaChallengeCharge(charges int, refreshedAt, serverTime time.Time, policy ArenaChallengeChargePolicy) (int, time.Time, error) {
	charges, refreshedAt, err := normalizeArenaChallengeCharges(charges, refreshedAt, serverTime, policy)
	if err != nil {
		return 0, time.Time{}, err
	}
	if charges >= policy.Capacity {
		return charges, refreshedAt, nil
	}
	charges++
	if charges >= policy.Capacity {
		charges = policy.Capacity
		refreshedAt = serverTime
	}
	return charges, refreshedAt, nil
}

func (s *Store) EnsureArenaPlayerState(ctx context.Context, userID, defaultRating int64, initialChallengeCharges int) (ArenaPlayerState, error) {
	if userID <= 0 || defaultRating < 0 || initialChallengeCharges <= 0 {
		return ArenaPlayerState{}, ErrBadQuantity
	}
	if _, err := s.db.Exec(ctx, `
		INSERT INTO arena_player_states(user_id,rating,challenge_charges)
		VALUES($1,$2,$3)
		ON CONFLICT(user_id) DO NOTHING
	`, userID, defaultRating, initialChallengeCharges); err != nil {
		return ArenaPlayerState{}, err
	}
	return s.getArenaPlayerState(ctx, s.db, userID)
}

func ensureArenaPlayerStateTx(ctx context.Context, tx pgx.Tx, userID, defaultRating int64, initialChallengeCharges int) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO arena_player_states(user_id,rating,challenge_charges)
		VALUES($1,$2,$3)
		ON CONFLICT(user_id) DO NOTHING
	`, userID, defaultRating, initialChallengeCharges)
	return err
}

type arenaStateQuerier interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}

func (s *Store) getArenaPlayerState(ctx context.Context, q arenaStateQuerier, userID int64) (ArenaPlayerState, error) {
	var out ArenaPlayerState
	err := q.QueryRow(ctx, `
		SELECT user_id,rating,tokens,attack_wins,attack_losses,defense_wins,defense_losses,challenge_charges,challenge_charge_refreshed_at,updated_at
		FROM arena_player_states
		WHERE user_id=$1
	`, userID).Scan(
		&out.UserID, &out.Rating, &out.Tokens, &out.AttackWins, &out.AttackLosses, &out.DefenseWins, &out.DefenseLosses,
		&out.ChallengeCharges, &out.ChallengeChargeRefreshedAt, &out.UpdatedAt,
	)
	return out, err
}

func refreshArenaPlayerStateTx(ctx context.Context, tx pgx.Tx, userID int64, policy ArenaChallengeChargePolicy) (ArenaPlayerState, ArenaChallengeChargeState, error) {
	var state ArenaPlayerState
	var serverTime time.Time
	err := tx.QueryRow(ctx, `
		SELECT user_id,rating,tokens,attack_wins,attack_losses,defense_wins,defense_losses,challenge_charges,challenge_charge_refreshed_at,updated_at,clock_timestamp()
		FROM arena_player_states
		WHERE user_id=$1
		FOR UPDATE
	`, userID).Scan(
		&state.UserID, &state.Rating, &state.Tokens, &state.AttackWins, &state.AttackLosses, &state.DefenseWins, &state.DefenseLosses,
		&state.ChallengeCharges, &state.ChallengeChargeRefreshedAt, &state.UpdatedAt, &serverTime,
	)
	if err != nil {
		return ArenaPlayerState{}, ArenaChallengeChargeState{}, err
	}
	charges, refreshedAt, err := normalizeArenaChallengeCharges(state.ChallengeCharges, state.ChallengeChargeRefreshedAt, serverTime, policy)
	if err != nil {
		return ArenaPlayerState{}, ArenaChallengeChargeState{}, err
	}
	if charges != state.ChallengeCharges || !refreshedAt.Equal(state.ChallengeChargeRefreshedAt) {
		if _, err := tx.Exec(ctx, `
			UPDATE arena_player_states
			SET challenge_charges=$2,challenge_charge_refreshed_at=$3,updated_at=now()
			WHERE user_id=$1
		`, userID, charges, refreshedAt); err != nil {
			return ArenaPlayerState{}, ArenaChallengeChargeState{}, err
		}
		state.ChallengeCharges = charges
		state.ChallengeChargeRefreshedAt = refreshedAt
		state.UpdatedAt = serverTime
	}
	return state, arenaChallengeChargeState(charges, refreshedAt, serverTime, policy), nil
}

func (s *Store) RefreshArenaPlayerState(ctx context.Context, userID, defaultRating int64, policy ArenaChallengeChargePolicy) (ArenaPlayerState, ArenaChallengeChargeState, error) {
	if userID <= 0 || defaultRating < 0 || !validArenaChallengeChargePolicy(policy) {
		return ArenaPlayerState{}, ArenaChallengeChargeState{}, ErrBattleRoomConfigInvalid
	}
	var state ArenaPlayerState
	var chargeState ArenaChallengeChargeState
	err := s.withTx(ctx, func(tx pgx.Tx) error {
		if err := s.lockUserTx(ctx, tx, userID); err != nil {
			return err
		}
		if err := ensureArenaPlayerStateTx(ctx, tx, userID, defaultRating, policy.Capacity); err != nil {
			return err
		}
		var err error
		state, chargeState, err = refreshArenaPlayerStateTx(ctx, tx, userID, policy)
		return err
	})
	return state, chargeState, err
}

func consumeArenaChallengeChargeTx(ctx context.Context, tx pgx.Tx, userID int64, policy ArenaChallengeChargePolicy) (ArenaChallengeChargeState, error) {
	state, chargeState, err := refreshArenaPlayerStateTx(ctx, tx, userID, policy)
	if err != nil {
		return ArenaChallengeChargeState{}, err
	}
	remaining, refreshedAt, nextState, err := consumeArenaChallengeCharge(state.ChallengeCharges, state.ChallengeChargeRefreshedAt, chargeState.ServerTime, policy)
	if err != nil {
		return nextState, err
	}
	if _, err := tx.Exec(ctx, `
		UPDATE arena_player_states
		SET challenge_charges=$2,challenge_charge_refreshed_at=$3,updated_at=now()
		WHERE user_id=$1
	`, userID, remaining, refreshedAt); err != nil {
		return ArenaChallengeChargeState{}, err
	}
	return nextState, nil
}

func refundArenaChallengeChargeTx(ctx context.Context, tx pgx.Tx, userID int64, policy ArenaChallengeChargePolicy) error {
	state, chargeState, err := refreshArenaPlayerStateTx(ctx, tx, userID, policy)
	if err != nil {
		return err
	}
	charges, refreshedAt, err := refundArenaChallengeCharge(state.ChallengeCharges, state.ChallengeChargeRefreshedAt, chargeState.ServerTime, policy)
	if err != nil {
		return err
	}
	if charges == state.ChallengeCharges && refreshedAt.Equal(state.ChallengeChargeRefreshedAt) {
		return nil
	}
	_, err = tx.Exec(ctx, `
		UPDATE arena_player_states
		SET challenge_charges=$2,challenge_charge_refreshed_at=$3,updated_at=now()
		WHERE user_id=$1
	`, userID, charges, refreshedAt)
	return err
}

func scanArenaDefense(row pgx.Row) (ArenaDefense, error) {
	var out ArenaDefense
	var selectionRaw, lineupRaw []byte
	err := row.Scan(&out.UserID, &out.Revision, &out.Active, &out.BattleLevel, &out.KittyIDs, &selectionRaw, &lineupRaw, &out.LineupDigest, &out.BountyTokens, &out.SuccessfulDefenses, &out.ChallengeCount, &out.PublishedAt, &out.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ArenaDefense{}, ErrArenaDefenseNotFound
		}
		return ArenaDefense{}, err
	}
	out.Selection = append(json.RawMessage(nil), selectionRaw...)
	out.Lineup = append(json.RawMessage(nil), lineupRaw...)
	return out, nil
}

func (s *Store) GetArenaDefense(ctx context.Context, userID int64) (ArenaDefense, error) {
	return scanArenaDefense(s.db.QueryRow(ctx, `
		SELECT user_id,revision,active,battle_level,kitty_ids,selection_jsonb,lineup_jsonb,lineup_digest,bounty_tokens,successful_defenses,challenge_count,published_at,updated_at
		FROM arena_defenses WHERE user_id=$1
	`, userID))
}

// ListArenaDefenseInitializationCandidates returns recently active users that
// have never published or initialized an arena defense. Any existing defense
// row, including an inactive one, excludes the user from initialization.
func (s *Store) ListArenaDefenseInitializationCandidates(ctx context.Context, activeSince time.Time, afterUserID int64, limit int) ([]int64, bool, error) {
	if activeSince.IsZero() || afterUserID < 0 || limit <= 0 || limit > 100 {
		return nil, false, ErrBadQuantity
	}
	rows, err := s.db.Query(ctx, listArenaDefenseInitializationCandidatesQuery, activeSince, afterUserID, limit+1)
	if err != nil {
		return nil, false, err
	}
	defer rows.Close()
	userIDs := make([]int64, 0, limit+1)
	for rows.Next() {
		var userID int64
		if err := rows.Scan(&userID); err != nil {
			return nil, false, err
		}
		userIDs = append(userIDs, userID)
	}
	if err := rows.Err(); err != nil {
		return nil, false, err
	}
	hasMore := len(userIDs) > limit
	if hasMore {
		userIDs = userIDs[:limit]
	}
	return userIDs, hasMore, nil
}

// CreateArenaDefenseIfAbsent inserts an initialized defense without ever
// updating a player-owned defense that appeared before or during the request.
func (s *Store) CreateArenaDefenseIfAbsent(ctx context.Context, input PublishArenaDefenseInput) (ArenaDefense, bool, error) {
	if input.UserID <= 0 || input.BattleLevel <= 0 || !validArenaKittyIDs(input.KittyIDs) || !json.Valid(input.Selection) || !json.Valid(input.Lineup) {
		return ArenaDefense{}, false, ErrBattleRoomConfigInvalid
	}
	kittyIDs := append([]int64{}, input.KittyIDs...)
	row := s.db.QueryRow(ctx, createArenaDefenseIfAbsentQuery,
		input.UserID,
		input.BattleLevel,
		kittyIDs,
		input.Selection,
		input.Lineup,
		strings.TrimSpace(input.LineupDigest),
	)
	defense, err := scanArenaDefense(row)
	if errors.Is(err, ErrArenaDefenseNotFound) {
		return ArenaDefense{}, false, nil
	}
	if err != nil {
		return ArenaDefense{}, false, err
	}
	return defense, true, nil
}

func (s *Store) PublishArenaDefense(ctx context.Context, input PublishArenaDefenseInput) (ArenaDefense, error) {
	if input.UserID <= 0 || input.BattleLevel <= 0 || !validArenaKittyIDs(input.KittyIDs) || !json.Valid(input.Selection) || !json.Valid(input.Lineup) {
		return ArenaDefense{}, ErrBattleRoomConfigInvalid
	}
	var out ArenaDefense
	err := s.withTx(ctx, func(tx pgx.Tx) error {
		var lockedUserID int64
		if err := tx.QueryRow(ctx, `SELECT user_id FROM arena_defenses WHERE user_id=$1 FOR UPDATE`, input.UserID).Scan(&lockedUserID); err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		var busy bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM arena_challenges WHERE (attacker_id=$1 OR defender_id=$1) AND status IN ('queued','running'))`, input.UserID).Scan(&busy); err != nil {
			return err
		}
		if busy {
			return ErrArenaDefenseBusy
		}
		row := tx.QueryRow(ctx, `
			INSERT INTO arena_defenses(user_id,revision,active,battle_level,kitty_ids,selection_jsonb,lineup_jsonb,lineup_digest,published_at,updated_at)
			VALUES($1,1,TRUE,$2,$3,$4,$5,$6,now(),now())
			ON CONFLICT(user_id) DO UPDATE SET
				revision=arena_defenses.revision+1,
				active=TRUE,
				battle_level=EXCLUDED.battle_level,
				kitty_ids=EXCLUDED.kitty_ids,
				selection_jsonb=EXCLUDED.selection_jsonb,
				lineup_jsonb=EXCLUDED.lineup_jsonb,
				lineup_digest=EXCLUDED.lineup_digest,
				published_at=now(),
				updated_at=now()
			RETURNING user_id,revision,active,battle_level,kitty_ids,selection_jsonb,lineup_jsonb,lineup_digest,bounty_tokens,successful_defenses,challenge_count,published_at,updated_at
		`, input.UserID, input.BattleLevel, input.KittyIDs, input.Selection, input.Lineup, strings.TrimSpace(input.LineupDigest))
		var scanErr error
		out, scanErr = scanArenaDefense(row)
		return scanErr
	})
	return out, err
}

func (s *Store) UnpublishArenaDefense(ctx context.Context, userID int64) error {
	return s.withTx(ctx, func(tx pgx.Tx) error {
		var lockedUserID int64
		if err := tx.QueryRow(ctx, `SELECT user_id FROM arena_defenses WHERE user_id=$1 FOR UPDATE`, userID).Scan(&lockedUserID); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrArenaDefenseNotFound
			}
			return err
		}
		var busy bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM arena_challenges WHERE (attacker_id=$1 OR defender_id=$1) AND status IN ('queued','running'))`, userID).Scan(&busy); err != nil {
			return err
		}
		if busy {
			return ErrArenaDefenseBusy
		}
		command, err := tx.Exec(ctx, `UPDATE arena_defenses SET active=FALSE,updated_at=now() WHERE user_id=$1 AND active=TRUE`, userID)
		if err != nil {
			return err
		}
		if command.RowsAffected() == 0 {
			return ErrArenaDefenseNotFound
		}
		return nil
	})
}

func normalizeArenaTargetFormatFilter(formatFilter string) (string, error) {
	formatFilter = strings.ToLower(strings.TrimSpace(formatFilter))
	if formatFilter == "" {
		formatFilter = ArenaFormatFilterAll
	}
	if formatFilter != ArenaFormatFilterAll {
		if len(formatFilter) > 64 {
			return "", ErrBattleRoomConfigInvalid
		}
		for index, char := range formatFilter {
			if (char >= 'a' && char <= 'z') || (char >= '0' && char <= '9') || (index > 0 && (char == '_' || char == ':' || char == '-')) {
				continue
			}
			return "", ErrBattleRoomConfigInvalid
		}
	}
	return formatFilter, nil
}

func scanArenaTargetRows(rows pgx.Rows, capacity int) ([]ArenaTarget, error) {
	out := make([]ArenaTarget, 0, capacity)
	for rows.Next() {
		var target ArenaTarget
		var selectionRaw, lineupRaw []byte
		if err := rows.Scan(&target.UserID, &target.Revision, &target.Active, &target.BattleLevel, &target.KittyIDs, &selectionRaw, &lineupRaw, &target.LineupDigest, &target.BountyTokens, &target.SuccessfulDefenses, &target.ChallengeCount, &target.PublishedAt, &target.UpdatedAt, &target.Username, &target.Rating); err != nil {
			return nil, err
		}
		target.Selection = append(json.RawMessage(nil), selectionRaw...)
		target.Lineup = append(json.RawMessage(nil), lineupRaw...)
		out = append(out, target)
	}
	return out, rows.Err()
}

func arenaMoreTargetPivot(userID int64, sampleSeed string, maxUserID int64) int64 {
	if userID <= 0 || strings.TrimSpace(sampleSeed) == "" || maxUserID <= 0 {
		return 0
	}
	h := fnv.New64a()
	_, _ = fmt.Fprintf(h, "%d:%s", userID, sampleSeed)
	return int64(h.Sum64()%uint64(maxUserID)) + 1
}

func (s *Store) ListArenaTargets(ctx context.Context, userID, battleLevel, defaultRating int64, limit, offset int, nearby bool, formatFilter, eligibilityPolicyVersion string) ([]ArenaTarget, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}
	formatFilter, err := normalizeArenaTargetFormatFilter(formatFilter)
	if err != nil {
		return nil, err
	}
	eligibilityPolicyVersion = strings.TrimSpace(eligibilityPolicyVersion)
	if eligibilityPolicyVersion == "" || len(eligibilityPolicyVersion) > 64 {
		return nil, ErrBattleRoomConfigInvalid
	}
	rows, err := s.db.Query(ctx, listArenaTargetsQuery, userID, maxInt64(1, battleLevel), defaultRating, limit, offset, nearby, formatFilter, eligibilityPolicyVersion)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanArenaTargetRows(rows, limit)
}

// ListArenaMoreTargets returns one stable daily sample before applying the
// optional format filter, so switching format tabs cannot expose new teams.
func (s *Store) ListArenaMoreTargets(ctx context.Context, userID, defaultRating int64, sampleSeed string, limit int, formatFilter, eligibilityPolicyVersion string) ([]ArenaTarget, error) {
	sampleSeed = strings.TrimSpace(sampleSeed)
	if userID <= 0 || defaultRating < 0 || sampleSeed == "" || len(sampleSeed) > 64 || limit <= 0 || limit > 100 {
		return nil, ErrBadQuantity
	}
	formatFilter, err := normalizeArenaTargetFormatFilter(formatFilter)
	if err != nil {
		return nil, err
	}
	eligibilityPolicyVersion = strings.TrimSpace(eligibilityPolicyVersion)
	if eligibilityPolicyVersion == "" || len(eligibilityPolicyVersion) > 64 {
		return nil, ErrBattleRoomConfigInvalid
	}
	var maxUserID int64
	if err := s.db.QueryRow(ctx, maxArenaActiveDefenseUserIDQuery).Scan(&maxUserID); err != nil {
		return nil, err
	}
	if maxUserID <= 0 {
		return []ArenaTarget{}, nil
	}
	pivot := arenaMoreTargetPivot(userID, sampleSeed, maxUserID)
	rows, err := s.db.Query(ctx, listArenaMoreTargetsQuery, userID, pivot, limit, sampleSeed, defaultRating, formatFilter, eligibilityPolicyVersion)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanArenaTargetRows(rows, limit)
}

func (s *Store) ListArenaLeaderboard(ctx context.Context, userID int64, limit, offset int) (ArenaLeaderboardPage, error) {
	if userID <= 0 {
		return ArenaLeaderboardPage{}, ErrBadQuantity
	}
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	rows, err := s.db.Query(ctx, listArenaLeaderboardQuery, limit, offset, userID)
	if err != nil {
		return ArenaLeaderboardPage{}, err
	}
	defer rows.Close()
	out := ArenaLeaderboardPage{Entries: make([]ArenaLeaderboardEntry, 0, limit)}
	for rows.Next() {
		var row ArenaLeaderboardEntry
		var total int64
		var selfOnly bool
		if err := rows.Scan(
			&row.Rank, &total, &row.UserID, &row.Username, &row.Rating,
			&row.AttackWins, &row.AttackLosses, &row.DefenseWins, &row.DefenseLosses,
			&row.ActiveDefense, &row.BattleLevel, &selfOnly,
		); err != nil {
			return ArenaLeaderboardPage{}, err
		}
		out.Total = total
		if row.UserID == userID {
			self := row
			out.Self = &self
		}
		if !selfOnly {
			out.Entries = append(out.Entries, row)
		}
	}
	if err := rows.Err(); err != nil {
		return ArenaLeaderboardPage{}, err
	}
	return out, nil
}

// AdminResetArenaLeaderboard resets only the fields that feed the leaderboard.
// Arena tokens, challenge charges, defenses, and challenge history are player
// assets or operational state and intentionally remain unchanged.
func (s *Store) AdminResetArenaLeaderboard(ctx context.Context, defaultRating int64) (ArenaLeaderboardResetResult, error) {
	if defaultRating < 0 {
		return ArenaLeaderboardResetResult{}, ErrBadQuantity
	}
	result := ArenaLeaderboardResetResult{DefaultRating: defaultRating}
	if err := s.db.QueryRow(ctx, adminResetArenaLeaderboardQuery, defaultRating).Scan(&result.ResetPlayers, &result.ResetAt); err != nil {
		return ArenaLeaderboardResetResult{}, fmt.Errorf("reset arena leaderboard: %w", err)
	}
	return result, nil
}

type arenaChallengeCooldownQuerier interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
}

func arenaChallengeCooldownSeconds(value time.Duration) int64 {
	if value <= 0 {
		return 0
	}
	seconds := value / time.Second
	if value%time.Second != 0 {
		seconds++
	}
	return int64(seconds)
}

func normalizeArenaCooldownOpponentIDs(opponentIDs []int64) ([]int64, error) {
	if len(opponentIDs) == 0 {
		return nil, nil
	}
	seen := make(map[int64]struct{}, len(opponentIDs))
	normalized := make([]int64, 0, len(opponentIDs))
	for _, opponentID := range opponentIDs {
		if opponentID <= 0 {
			return nil, ErrBattleRoomConfigInvalid
		}
		if _, exists := seen[opponentID]; exists {
			continue
		}
		seen[opponentID] = struct{}{}
		normalized = append(normalized, opponentID)
	}
	return normalized, nil
}

func loadArenaOpponentCooldownState(ctx context.Context, querier arenaChallengeCooldownQuerier, attackerID int64, opponentIDs []int64, opponentCooldown time.Duration) (ArenaOpponentCooldownState, error) {
	if attackerID <= 0 || opponentCooldown < 0 {
		return ArenaOpponentCooldownState{}, ErrBattleRoomConfigInvalid
	}
	normalizedOpponentIDs, err := normalizeArenaCooldownOpponentIDs(opponentIDs)
	if err != nil {
		return ArenaOpponentCooldownState{}, err
	}
	state := ArenaOpponentCooldownState{
		ServerTime:            time.Now().UTC(),
		OpponentUntilByUserID: make(map[int64]time.Time, len(normalizedOpponentIDs)),
	}
	if len(normalizedOpponentIDs) == 0 || opponentCooldown <= 0 {
		return state, nil
	}
	rows, err := querier.Query(ctx, arenaChallengeOpponentCooldownsQuery, attackerID, normalizedOpponentIDs, arenaChallengeCooldownSeconds(opponentCooldown))
	if err != nil {
		return ArenaOpponentCooldownState{}, err
	}
	defer rows.Close()
	for rows.Next() {
		var serverTime time.Time
		var opponentID int64
		var until time.Time
		if err := rows.Scan(&serverTime, &opponentID, &until); err != nil {
			return ArenaOpponentCooldownState{}, err
		}
		state.ServerTime = serverTime
		state.OpponentUntilByUserID[opponentID] = until
	}
	if err := rows.Err(); err != nil {
		return ArenaOpponentCooldownState{}, err
	}
	return state, nil
}

func (s *Store) GetArenaOpponentCooldownState(ctx context.Context, attackerID int64, opponentIDs []int64, opponentCooldown time.Duration) (ArenaOpponentCooldownState, error) {
	return loadArenaOpponentCooldownState(ctx, s.db, attackerID, opponentIDs, opponentCooldown)
}

func (s *Store) CreateArenaChallenge(ctx context.Context, input CreateArenaChallengeInput) (ArenaChallenge, ArenaChallengeChargeState, error) {
	formatSnapshot := strings.TrimSpace(string(input.FormatSnapshot))
	if input.AttackerID <= 0 || input.DefenderID <= 0 || input.AttackerID == input.DefenderID || input.ExpectedDefenseRevision <= 0 ||
		!json.Valid(input.AttackerLineup) || !json.Valid(input.FormatSnapshot) || formatSnapshot == "" || formatSnapshot == "{}" || formatSnapshot == "null" ||
		input.DefaultRating < 0 || !validArenaChallengeChargePolicy(input.ChargePolicy) || input.OpponentCooldown < 0 {
		return ArenaChallenge{}, ArenaChallengeChargeState{}, ErrBattleRoomConfigInvalid
	}
	var out ArenaChallenge
	var chargeState ArenaChallengeChargeState
	err := s.withTx(ctx, func(tx pgx.Tx) error {
		if err := s.lockUserTx(ctx, tx, input.AttackerID); err != nil {
			return err
		}
		var active bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM arena_challenges WHERE attacker_id=$1 AND status IN ('queued','running'))`, input.AttackerID).Scan(&active); err != nil {
			return err
		}
		if active {
			return ErrArenaChallengeActive
		}
		cooldownState, err := loadArenaOpponentCooldownState(ctx, tx, input.AttackerID, []int64{input.DefenderID}, input.OpponentCooldown)
		if err != nil {
			return err
		}
		if _, cooling := cooldownState.OpponentUntilByUserID[input.DefenderID]; cooling {
			return ErrArenaChallengeOpponentCooldown
		}
		var defenderLineup []byte
		if err := tx.QueryRow(ctx, `
			SELECT revision,lineup_jsonb
			FROM arena_defenses
			WHERE user_id=$1 AND revision=$2 AND active=TRUE
			FOR UPDATE
		`, input.DefenderID, input.ExpectedDefenseRevision).Scan(&out.DefenseRevision, &defenderLineup); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrArenaDefenseNotFound
			}
			return err
		}
		out.AttackerLineup = append(json.RawMessage(nil), input.AttackerLineup...)
		out.DefenderLineup = append(json.RawMessage(nil), defenderLineup...)
		out.FormatSnapshot = append(json.RawMessage(nil), input.FormatSnapshot...)
		if err := ensureArenaPlayerStateTx(ctx, tx, input.AttackerID, input.DefaultRating, input.ChargePolicy.Capacity); err != nil {
			return err
		}
		chargeState, err = consumeArenaChallengeChargeTx(ctx, tx, input.AttackerID, input.ChargePolicy)
		if err != nil {
			return err
		}
		err = tx.QueryRow(ctx, `
			INSERT INTO arena_challenges(attacker_id,defender_id,defense_revision,seed,attacker_lineup_jsonb,defender_lineup_jsonb,format_snapshot_jsonb,rating_eligible)
			VALUES($1,$2,$3,$4,$5,$6,$7,$8)
			RETURNING id,battle_kind,status,outcome,created_at
		`, input.AttackerID, input.DefenderID, out.DefenseRevision, input.Seed, input.AttackerLineup, defenderLineup, input.FormatSnapshot, input.RatingEligible).Scan(&out.ID, &out.BattleKind, &out.Status, &out.Outcome, &out.CreatedAt)
		if err != nil {
			return err
		}
		out.AttackerID = input.AttackerID
		out.DefenderID = input.DefenderID
		out.Seed = input.Seed
		out.RatingEligible = input.RatingEligible
		_, err = tx.Exec(ctx, `UPDATE arena_defenses SET challenge_count=challenge_count+1,updated_at=now() WHERE user_id=$1`, input.DefenderID)
		return err
	})
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == "uq_arena_challenges_attacker_active" {
			return ArenaChallenge{}, ArenaChallengeChargeState{}, ErrArenaChallengeActive
		}
	}
	return out, chargeState, err
}

func (s *Store) ClaimArenaChallenge(ctx context.Context, workerID string, lease time.Duration, now time.Time) (ArenaChallenge, error) {
	workerID = strings.TrimSpace(workerID)
	if workerID == "" || lease <= 0 {
		return ArenaChallenge{}, ErrBattleRoomConfigInvalid
	}
	var out ArenaChallenge
	var attackerRaw, defenderRaw, formatRaw []byte
	err := s.db.QueryRow(ctx, `
		WITH candidate AS (
			SELECT id
			FROM arena_challenges
			WHERE status='queued' OR (status='running' AND lease_until IS NOT NULL AND lease_until<=$2)
			ORDER BY created_at ASC,id ASC
			FOR UPDATE SKIP LOCKED
			LIMIT 1
		)
		UPDATE arena_challenges c
		SET status='running',worker_id=$1,lease_until=$3,attempts=c.attempts+1,started_at=COALESCE(c.started_at,$2),updated_at=$2
		FROM candidate
		WHERE c.id=candidate.id
		RETURNING c.id,c.battle_kind,c.attacker_id,c.defender_id,c.defense_revision,c.seed,c.status,c.outcome,c.worker_id,c.attempts,c.attacker_lineup_jsonb,c.defender_lineup_jsonb,c.format_snapshot_jsonb,c.rating_eligible,c.created_at,c.started_at,c.completed_at
	`, workerID, now, now.Add(lease)).Scan(&out.ID, &out.BattleKind, &out.AttackerID, &out.DefenderID, &out.DefenseRevision, &out.Seed, &out.Status, &out.Outcome, &out.WorkerID, &out.Attempts, &attackerRaw, &defenderRaw, &formatRaw, &out.RatingEligible, &out.CreatedAt, &out.StartedAt, &out.CompletedAt)
	if err != nil {
		return ArenaChallenge{}, err
	}
	out.AttackerLineup = append(json.RawMessage(nil), attackerRaw...)
	out.DefenderLineup = append(json.RawMessage(nil), defenderRaw...)
	out.FormatSnapshot = append(json.RawMessage(nil), formatRaw...)
	return out, nil
}

func (s *Store) CompleteArenaChallenge(ctx context.Context, input CompleteArenaChallengeInput) (ArenaSettlementResult, error) {
	if input.ChallengeID <= 0 || strings.TrimSpace(input.WorkerID) == "" || (input.Outcome != ArenaChallengeAttackerWin && input.Outcome != ArenaChallengeDefenderWin) || !json.Valid(input.InitialState) || !json.Valid(input.Timeline) || !json.Valid(input.FinalResult) {
		return ArenaSettlementResult{}, ErrBattleRoomConfigInvalid
	}
	var out ArenaSettlementResult
	err := s.withTx(ctx, func(tx pgx.Tx) error {
		var status, workerID string
		var ratingEligible bool
		var defenseRevision int64
		if err := tx.QueryRow(ctx, `SELECT attacker_id,defender_id,defense_revision,status,worker_id,rating_eligible FROM arena_challenges WHERE id=$1 FOR UPDATE`, input.ChallengeID).Scan(&out.AttackerID, &out.DefenderID, &defenseRevision, &status, &workerID, &ratingEligible); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrArenaChallengeNotFound
			}
			return err
		}
		if status != ArenaChallengeStatusRunning || workerID != strings.TrimSpace(input.WorkerID) {
			return ErrArenaChallengeLeaseLost
		}
		var currentRevision, currentBounty int64
		if err := tx.QueryRow(ctx, `SELECT revision,bounty_tokens FROM arena_defenses WHERE user_id=$1 FOR UPDATE`, out.DefenderID).Scan(&currentRevision, &currentBounty); err != nil {
			return err
		}
		if currentRevision != defenseRevision {
			return ErrArenaDefenseRevisionChanged
		}
		for _, userID := range []int64{out.AttackerID, out.DefenderID} {
			if _, err := tx.Exec(ctx, `INSERT INTO arena_player_states(user_id,rating) VALUES($1,$2) ON CONFLICT(user_id) DO NOTHING`, userID, input.Policy.DefaultRating); err != nil {
				return err
			}
		}
		rows, err := tx.Query(ctx, `SELECT user_id FROM arena_player_states WHERE user_id=ANY($1) ORDER BY user_id FOR UPDATE`, []int64{out.AttackerID, out.DefenderID})
		if err != nil {
			return err
		}
		for rows.Next() {
			var ignored int64
			if err := rows.Scan(&ignored); err != nil {
				rows.Close()
				return err
			}
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return err
		}
		out.ChallengeID = input.ChallengeID
		out.RatingEligible = ratingEligible
		out.AttackerRatingDelta, out.DefenderRatingDelta = arenaSettlementRatingDeltas(input.Outcome, ratingEligible, input.Policy)
		if input.Outcome == ArenaChallengeAttackerWin {
			out.BountyRewardTokens = currentBounty
			out.AttackerRewardTokens = input.Policy.AttackerWinTokens + currentBounty
			if _, err := tx.Exec(ctx, `UPDATE arena_defenses SET bounty_tokens=0,successful_defenses=0,updated_at=now() WHERE user_id=$1`, out.DefenderID); err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, `UPDATE arena_player_states SET rating=GREATEST(0,rating+$2),tokens=tokens+$3,attack_wins=attack_wins+1,updated_at=now() WHERE user_id=$1`, out.AttackerID, out.AttackerRatingDelta, out.AttackerRewardTokens); err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, `UPDATE arena_player_states SET rating=GREATEST(0,rating+$2),defense_losses=defense_losses+1,updated_at=now() WHERE user_id=$1`, out.DefenderID, out.DefenderRatingDelta); err != nil {
				return err
			}
		} else {
			out.AttackerRewardTokens = input.Policy.AttackerLossTokens
			out.DefenderRewardTokens = input.Policy.DefenderWinTokens
			if _, err := tx.Exec(ctx, `UPDATE arena_defenses SET bounty_tokens=LEAST($2,bounty_tokens+$3),successful_defenses=successful_defenses+1,updated_at=now() WHERE user_id=$1`, out.DefenderID, input.Policy.BountyCap, input.Policy.BountyPerDefenseWin); err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, `UPDATE arena_player_states SET rating=GREATEST(0,rating+$2),tokens=tokens+$3,attack_losses=attack_losses+1,updated_at=now() WHERE user_id=$1`, out.AttackerID, out.AttackerRatingDelta, out.AttackerRewardTokens); err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, `UPDATE arena_player_states SET rating=GREATEST(0,rating+$2),tokens=tokens+$3,defense_wins=defense_wins+1,updated_at=now() WHERE user_id=$1`, out.DefenderID, out.DefenderRatingDelta, out.DefenderRewardTokens); err != nil {
				return err
			}
		}
		if _, err := tx.Exec(ctx, `
			UPDATE arena_challenges SET status=$2,outcome=$2,worker_id='',lease_until=NULL,error_code='',attacker_reward_tokens=$3,defender_reward_tokens=$4,bounty_reward_tokens=$5,attacker_rating_delta=$6,defender_rating_delta=$7,completed_at=now(),updated_at=now()
			WHERE id=$1
		`, input.ChallengeID, input.Outcome, out.AttackerRewardTokens, out.DefenderRewardTokens, out.BountyRewardTokens, out.AttackerRatingDelta, out.DefenderRatingDelta); err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `
			INSERT INTO arena_challenge_payloads(challenge_id,initial_state_jsonb,timeline_jsonb,final_result_jsonb)
			VALUES($1,$2,$3,$4)
			ON CONFLICT(challenge_id) DO UPDATE SET initial_state_jsonb=EXCLUDED.initial_state_jsonb,timeline_jsonb=EXCLUDED.timeline_jsonb,final_result_jsonb=EXCLUDED.final_result_jsonb
		`, input.ChallengeID, input.InitialState, input.Timeline, input.FinalResult)
		return err
	})
	return out, err
}

func (s *Store) FailArenaChallenge(ctx context.Context, challengeID int64, workerID, errorCode string, maxAttempts int, chargePolicy ArenaChallengeChargePolicy) (bool, error) {
	if challengeID <= 0 || strings.TrimSpace(workerID) == "" || maxAttempts <= 0 || !validArenaChallengeChargePolicy(chargePolicy) {
		return false, ErrBattleRoomConfigInvalid
	}
	var terminal bool
	err := s.withTx(ctx, func(tx pgx.Tx) error {
		var attempts int
		var attackerID int64
		var status, lockedWorker string
		if err := tx.QueryRow(ctx, `SELECT attacker_id,attempts,status,worker_id FROM arena_challenges WHERE id=$1 FOR UPDATE`, challengeID).Scan(&attackerID, &attempts, &status, &lockedWorker); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrArenaChallengeNotFound
			}
			return err
		}
		if status != ArenaChallengeStatusRunning || lockedWorker != strings.TrimSpace(workerID) {
			return ErrArenaChallengeLeaseLost
		}
		terminal = attempts >= maxAttempts
		nextStatus := ArenaChallengeStatusQueued
		completedExpr := "NULL"
		if terminal {
			nextStatus = ArenaChallengeStatusFailed
			completedExpr = "now()"
		}
		if _, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE arena_challenges SET status=$2,worker_id='',lease_until=NULL,error_code=$3,completed_at=%s,updated_at=now() WHERE id=$1`, completedExpr), challengeID, nextStatus, strings.TrimSpace(errorCode)); err != nil {
			return err
		}
		if terminal {
			return refundArenaChallengeChargeTx(ctx, tx, attackerID, chargePolicy)
		}
		return nil
	})
	return terminal, err
}

func (s *Store) ListArenaChallengeHistory(ctx context.Context, userID int64, role string, limit, offset int) ([]ArenaChallenge, error) {
	if limit <= 0 || limit > ArenaChallengeHistoryMaxRead {
		limit = ArenaChallengeHistoryMaxRead
	}
	if offset < 0 {
		offset = 0
	}
	role = strings.ToLower(strings.TrimSpace(role))
	roleClause := ""
	switch role {
	case "attack":
		roleClause = "AND c.attacker_id=$1"
	case "defense":
		roleClause = "AND c.defender_id=$1"
	}
	query := fmt.Sprintf(`
		WITH retained AS (
			SELECT c.*
			FROM arena_challenges c
			WHERE c.attacker_id=$1 OR c.defender_id=$1
			ORDER BY c.created_at DESC,c.id DESC
			LIMIT $2
		)
		SELECT c.id,c.battle_kind,c.attacker_id,c.defender_id,
			CASE WHEN c.attacker_id=$1 THEN c.defender_id ELSE c.attacker_id END,
			u.username,
			CASE WHEN c.attacker_id=$1 THEN 'attack' ELSE 'defense' END,
			c.defense_revision,c.seed,c.status,c.outcome,c.attempts,c.format_snapshot_jsonb,c.rating_eligible,
			c.attacker_reward_tokens,c.defender_reward_tokens,c.bounty_reward_tokens,c.attacker_rating_delta,c.defender_rating_delta,
			c.created_at,c.started_at,c.completed_at
		FROM retained c
		JOIN users u ON u.id=CASE WHEN c.attacker_id=$1 THEN c.defender_id ELSE c.attacker_id END
		WHERE TRUE %s
		ORDER BY c.created_at DESC,c.id DESC
		LIMIT $2 OFFSET $3
	`, roleClause)
	rows, err := s.db.Query(ctx, query, userID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]ArenaChallenge, 0, limit)
	for rows.Next() {
		var row ArenaChallenge
		var formatRaw []byte
		if err := rows.Scan(&row.ID, &row.BattleKind, &row.AttackerID, &row.DefenderID, &row.OpponentID, &row.OpponentName, &row.Role, &row.DefenseRevision, &row.Seed, &row.Status, &row.Outcome, &row.Attempts, &formatRaw, &row.RatingEligible, &row.AttackerRewardTokens, &row.DefenderRewardTokens, &row.BountyRewardTokens, &row.AttackerRatingDelta, &row.DefenderRatingDelta, &row.CreatedAt, &row.StartedAt, &row.CompletedAt); err != nil {
			return nil, err
		}
		row.FormatSnapshot = append(json.RawMessage(nil), formatRaw...)
		out = append(out, row)
	}
	return out, rows.Err()
}

func (s *Store) GetArenaChallengeReplay(ctx context.Context, userID, challengeID int64, keepCount int) (ArenaChallengePayload, error) {
	keepCount = normalizeArenaHistoryKeepCount(keepCount)
	var out ArenaChallengePayload
	var initialRaw, timelineRaw, finalRaw []byte
	err := s.db.QueryRow(ctx, `
		WITH retained AS (
			SELECT id
			FROM arena_challenges
			WHERE attacker_id=$2 OR defender_id=$2
			ORDER BY created_at DESC,id DESC
			LIMIT $3
		)
		SELECT c.id,c.battle_kind,c.seed,c.status,c.outcome,p.initial_state_jsonb,p.timeline_jsonb,p.final_result_jsonb
		FROM arena_challenges c
		JOIN retained r ON r.id=c.id
		JOIN arena_challenge_payloads p ON p.challenge_id=c.id
		WHERE c.id=$1 AND (c.attacker_id=$2 OR c.defender_id=$2)
	`, challengeID, userID, keepCount).Scan(&out.ChallengeID, &out.BattleKind, &out.Seed, &out.Status, &out.Outcome, &initialRaw, &timelineRaw, &finalRaw)
	if errors.Is(err, pgx.ErrNoRows) {
		return ArenaChallengePayload{}, ErrArenaChallengeNotFound
	}
	if err != nil {
		return ArenaChallengePayload{}, err
	}
	out.InitialState = append(json.RawMessage(nil), initialRaw...)
	out.Timeline = append(json.RawMessage(nil), timelineRaw...)
	out.FinalResult = append(json.RawMessage(nil), finalRaw...)
	return out, nil
}

func (s *Store) PruneArenaChallengeHistory(ctx context.Context, keepCount int) (int64, error) {
	if keepCount <= 0 {
		return 0, ErrBattleRoomConfigInvalid
	}
	command, err := s.db.Exec(ctx, `
		WITH participant_rows AS (
			SELECT id AS challenge_id,attacker_id AS participant_id,created_at FROM arena_challenges
			UNION ALL
			SELECT id AS challenge_id,defender_id AS participant_id,created_at FROM arena_challenges
		), ranked AS (
			SELECT challenge_id,participant_id,row_number() OVER(PARTITION BY participant_id ORDER BY created_at DESC,challenge_id DESC) AS row_no
			FROM participant_rows
		), removable AS (
			SELECT challenge_id
			FROM ranked
			GROUP BY challenge_id
			HAVING COUNT(*)=2 AND MIN(row_no)>$1
		)
		DELETE FROM arena_challenges c
		USING removable r
		WHERE c.id=r.challenge_id AND c.status IN ('attacker_win','defender_win','failed')
	`, keepCount)
	if err != nil {
		return 0, err
	}
	return command.RowsAffected(), nil
}

func (s *Store) CreateArenaReplayShare(ctx context.Context, createdBy, challengeID int64, token string, keepCount int) (ArenaReplayShare, error) {
	token = strings.TrimSpace(token)
	if createdBy <= 0 || challengeID <= 0 || len(token) < 32 || len(token) > 128 {
		return ArenaReplayShare{}, ErrBattleRoomConfigInvalid
	}
	keepCount = normalizeArenaHistoryKeepCount(keepCount)
	var share ArenaReplayShare
	err := s.db.QueryRow(ctx, `
		WITH eligible AS (
			SELECT c.id,c.outcome
			FROM arena_challenges c
			JOIN arena_challenge_payloads p ON p.challenge_id=c.id
			WHERE c.id=$2
			  AND (c.attacker_id=$1 OR c.defender_id=$1)
			  AND c.status IN ('attacker_win','defender_win')
			  AND c.id IN (
				SELECT recent.id
				FROM arena_challenges recent
				WHERE recent.attacker_id=$1 OR recent.defender_id=$1
				ORDER BY recent.created_at DESC,recent.id DESC
				LIMIT $4::int
			  )
		), inserted AS (
			INSERT INTO arena_replay_shares(token,challenge_id,created_by,expires_at)
			SELECT $3,e.id,$1,NULL::timestamptz FROM eligible e
			ON CONFLICT(challenge_id,created_by) DO UPDATE SET
				token=CASE
					WHEN arena_replay_shares.revoked_at IS NULL THEN arena_replay_shares.token
					ELSE EXCLUDED.token
				END,
				created_at=CASE
					WHEN arena_replay_shares.revoked_at IS NULL THEN arena_replay_shares.created_at
					ELSE now()
				END,
				expires_at=NULL,
				revoked_at=CASE
					WHEN arena_replay_shares.revoked_at IS NULL THEN arena_replay_shares.revoked_at
					ELSE NULL
				END
			RETURNING token,challenge_id,created_by,created_at
		)
		SELECT i.token,i.challenge_id,i.created_by,u.username,e.outcome,i.created_at
		FROM inserted i
		JOIN eligible e ON e.id=i.challenge_id
		JOIN users u ON u.id=i.created_by
	`, createdBy, challengeID, token, keepCount).Scan(
		&share.Token, &share.ChallengeID, &share.CreatedBy, &share.CreatedByName,
		&share.Outcome, &share.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return ArenaReplayShare{}, ErrArenaChallengeNotFound
	}
	return share, err
}

func (s *Store) GetArenaReplayShare(ctx context.Context, token string, keepCount int) (ArenaReplayShare, error) {
	token = strings.TrimSpace(token)
	if len(token) < 32 || len(token) > 128 {
		return ArenaReplayShare{}, ErrArenaReplayShareUnavailable
	}
	keepCount = normalizeArenaHistoryKeepCount(keepCount)
	var share ArenaReplayShare
	err := s.db.QueryRow(ctx, `
		SELECT s.token,s.challenge_id,s.created_by,u.username,c.outcome,s.created_at
		FROM arena_replay_shares s
		JOIN arena_challenges c ON c.id=s.challenge_id
		JOIN arena_challenge_payloads p ON p.challenge_id=c.id
		JOIN users u ON u.id=s.created_by
		WHERE s.token=$1 AND s.revoked_at IS NULL
		  AND c.id IN (
			SELECT recent.id
			FROM arena_challenges recent
			WHERE recent.attacker_id=s.created_by OR recent.defender_id=s.created_by
			ORDER BY recent.created_at DESC,recent.id DESC
			LIMIT $2
		  )
	`, token, keepCount).Scan(
		&share.Token, &share.ChallengeID, &share.CreatedBy, &share.CreatedByName,
		&share.Outcome, &share.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return ArenaReplayShare{}, ErrArenaReplayShareUnavailable
	}
	return share, err
}

func (s *Store) GetArenaSharedReplay(ctx context.Context, token string, keepCount int) (ArenaSharedReplay, error) {
	token = strings.TrimSpace(token)
	if len(token) < 32 || len(token) > 128 {
		return ArenaSharedReplay{}, ErrArenaReplayShareUnavailable
	}
	keepCount = normalizeArenaHistoryKeepCount(keepCount)
	var out ArenaSharedReplay
	var initialRaw, timelineRaw, finalRaw []byte
	err := s.db.QueryRow(ctx, `
		SELECT s.token,s.challenge_id,s.created_by,u.username,c.outcome,s.created_at,
			c.id,c.battle_kind,c.seed,c.status,c.outcome,p.initial_state_jsonb,p.timeline_jsonb,p.final_result_jsonb
		FROM arena_replay_shares s
		JOIN arena_challenges c ON c.id=s.challenge_id
		JOIN arena_challenge_payloads p ON p.challenge_id=c.id
		JOIN users u ON u.id=s.created_by
		WHERE s.token=$1 AND s.revoked_at IS NULL
		  AND c.id IN (
			SELECT recent.id
			FROM arena_challenges recent
			WHERE recent.attacker_id=s.created_by OR recent.defender_id=s.created_by
			ORDER BY recent.created_at DESC,recent.id DESC
			LIMIT $2
		  )
	`, token, keepCount).Scan(
		&out.Share.Token, &out.Share.ChallengeID, &out.Share.CreatedBy, &out.Share.CreatedByName,
		&out.Share.Outcome, &out.Share.CreatedAt,
		&out.Payload.ChallengeID, &out.Payload.BattleKind, &out.Payload.Seed, &out.Payload.Status, &out.Payload.Outcome,
		&initialRaw, &timelineRaw, &finalRaw,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return ArenaSharedReplay{}, ErrArenaReplayShareUnavailable
	}
	if err != nil {
		return ArenaSharedReplay{}, err
	}
	out.Payload.InitialState = append(json.RawMessage(nil), initialRaw...)
	out.Payload.Timeline = append(json.RawMessage(nil), timelineRaw...)
	out.Payload.FinalResult = append(json.RawMessage(nil), finalRaw...)
	return out, nil
}

func (s *Store) BuyArenaShopOffer(ctx context.Context, userID, defaultRating int64, offerID, itemID string, quantity, cost int64, idempotencyKey string) (ArenaShopPurchaseResult, error) {
	idempotencyKey = strings.TrimSpace(idempotencyKey)
	if userID <= 0 || strings.TrimSpace(offerID) == "" || strings.TrimSpace(itemID) == "" || quantity <= 0 || cost <= 0 || idempotencyKey == "" || len(idempotencyKey) > 160 {
		return ArenaShopPurchaseResult{}, ErrBattleRoomConfigInvalid
	}
	var out ArenaShopPurchaseResult
	err := s.withTx(ctx, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `INSERT INTO arena_player_states(user_id,rating) VALUES($1,$2) ON CONFLICT(user_id) DO NOTHING`, userID, defaultRating); err != nil {
			return err
		}
		var tokens int64
		if err := tx.QueryRow(ctx, `SELECT tokens FROM arena_player_states WHERE user_id=$1 FOR UPDATE`, userID).Scan(&tokens); err != nil {
			return err
		}
		var existingID int64
		err := tx.QueryRow(ctx, `SELECT id FROM arena_shop_orders WHERE user_id=$1 AND idempotency_key=$2`, userID, idempotencyKey).Scan(&existingID)
		if err == nil {
			out = ArenaShopPurchaseResult{OrderID: existingID, Tokens: tokens, Duplicate: true}
			return nil
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		if tokens < cost {
			return ErrArenaTokensInsufficient
		}
		if err := s.LedgerGrantTemplateTx(ctx, tx, userID, strings.TrimSpace(itemID), quantity, InventoryLedgerMeta{
			SourceSurface: "arena_shop",
			TargetSurface: "inventory",
			Details:       map[string]any{"offerId": strings.TrimSpace(offerID), "tokenCost": cost},
		}); err != nil {
			return err
		}
		if err := tx.QueryRow(ctx, `
			INSERT INTO arena_shop_orders(user_id,offer_id,item_id,quantity,token_cost,idempotency_key)
			VALUES($1,$2,$3,$4,$5,$6)
			RETURNING id
		`, userID, strings.TrimSpace(offerID), strings.TrimSpace(itemID), quantity, cost, idempotencyKey).Scan(&out.OrderID); err != nil {
			return err
		}
		out.Tokens = tokens - cost
		_, err = tx.Exec(ctx, `UPDATE arena_player_states SET tokens=$2,updated_at=now() WHERE user_id=$1`, userID, out.Tokens)
		return err
	})
	return out, err
}
