package store

import (
	"context"
	"errors"
	"os"
	"reflect"
	"testing"
	"time"

	"mysecondwebgame/server/internal/catalog"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

func fishingTournamentPostgresStore(t *testing.T) (*Store, context.Context) {
	t.Helper()
	dsn := os.Getenv("FISHING_TOURNAMENT_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set FISHING_TOURNAMENT_TEST_DATABASE_URL for isolated PostgreSQL tests")
	}
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatal(err)
	}
	if host := cfg.ConnConfig.Host; host != "localhost" && host != "127.0.0.1" && host != "::1" {
		t.Fatal("loopback database required")
	}
	// Keep server-side parameter inference and isolate every write to temporary tables.
	cfg.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeCacheStatement
	cfg.ConnConfig.RuntimeParams["search_path"] = "pg_temp"
	cfg.MaxConns = 1
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	_, err = pool.Exec(ctx, `
		CREATE TEMP TABLE users(id bigint PRIMARY KEY, username text NOT NULL);
		CREATE TEMP TABLE fishing_tournaments(
			id text PRIMARY KEY, week_key text NOT NULL, target_species_id text NOT NULL,
			ruleset_version text NOT NULL, starts_at timestamptz NOT NULL, ends_at timestamptz NOT NULL,
			status text NOT NULL DEFAULT 'open' CHECK(status IN ('open', 'finalized')),
			finalized_at timestamptz, created_at timestamptz NOT NULL DEFAULT now(),
			UNIQUE(week_key, ruleset_version)
		);
		CREATE TEMP TABLE fishing_tournament_entries(
			tournament_id text NOT NULL REFERENCES fishing_tournaments(id), user_id bigint NOT NULL REFERENCES users(id),
			division text NOT NULL CHECK(division IN ('open', 'standard')), catch_id bigint, fish_instance_id bigint,
			source_kind text NOT NULL DEFAULT 'catch' CHECK(source_kind IN ('catch', 'cultured')),
			length_cm double precision NOT NULL CHECK(length_cm > 0), weight_kg double precision NOT NULL CHECK(weight_kg > 0),
			submitted_at timestamptz NOT NULL, PRIMARY KEY(tournament_id, user_id, division)
		);
		CREATE TEMP TABLE fishing_tournament_rewards(
			tournament_id text NOT NULL REFERENCES fishing_tournaments(id), user_id bigint NOT NULL REFERENCES users(id),
			division text NOT NULL CHECK(division IN ('open', 'standard')), rank int NOT NULL CHECK(rank > 0),
			token_amount bigint NOT NULL DEFAULT 0 CHECK(token_amount >= 0),
			item_id text NOT NULL DEFAULT '', cosmetic_key text NOT NULL DEFAULT '',
			claimed_at timestamptz, created_at timestamptz NOT NULL DEFAULT now(),
			PRIMARY KEY(tournament_id, user_id, division)
		);
		CREATE TEMP TABLE fishing_player_wallets(
			user_id bigint PRIMARY KEY REFERENCES users(id), tokens bigint NOT NULL DEFAULT 0 CHECK(tokens >= 0),
			updated_at timestamptz NOT NULL DEFAULT now()
		);
		CREATE TEMP TABLE fishing_token_redemptions(
			user_id bigint NOT NULL REFERENCES users(id), reward_key text NOT NULL,
			token_cost bigint NOT NULL CHECK(token_cost > 0), redeemed_at timestamptz NOT NULL DEFAULT now(),
			PRIMARY KEY(user_id, reward_key)
		);
		INSERT INTO users SELECT id, 'angler-' || id FROM generate_series(1, 11) AS id;
	`)
	if err != nil {
		t.Fatal(err)
	}
	data, err := catalog.Load("../../../packages/game-data/generated/game-data.json")
	if err != nil {
		t.Fatal(err)
	}
	return &Store{db: pool, gameData: data}, ctx
}

func seedFishingTournamentPostgres(t *testing.T, s *Store, ctx context.Context, participants int) FishingTournament {
	t.Helper()
	var tournament FishingTournament
	err := s.withTx(ctx, func(tx pgx.Tx) error {
		var err error
		tournament, err = s.ensureFishingTournamentTx(ctx, tx, time.Date(2026, 9, 7, 0, 0, 0, 0, time.UTC))
		if err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `
			INSERT INTO fishing_tournament_entries(tournament_id, user_id, division, length_cm, weight_kg, submitted_at)
			SELECT $1, id, division, 20, CASE WHEN division = 'open' THEN 12 - id ELSE id END, $2
			FROM generate_series(1, $3::int) AS id
			CROSS JOIN (VALUES('open'), ('standard')) AS divisions(division)
		`, tournament.ID, tournament.StartsAt.Add(time.Hour), participants)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return tournament
}

func TestFishingTournamentPostgresOverviewFinalization(t *testing.T) {
	for _, tc := range []struct {
		name         string
		participants int
	}{
		{name: "all reward tiers in both divisions", participants: 11},
		{name: "empty leaderboards", participants: 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, ctx := fishingTournamentPostgresStore(t)
			tournament := seedFishingTournamentPostgres(t, s, ctx, tc.participants)
			now := tournament.EndsAt
			var firstOverview FishingTournamentOverview
			var firstRewardsJSON string
			for attempt := range 2 {
				overview, err := s.GetFishingTournamentOverview(ctx, 1, now.Add(time.Duration(attempt)*time.Minute))
				if err != nil {
					t.Fatalf("overview attempt %d: %v", attempt, err)
				}
				if overview.Current.ID == tournament.ID || !overview.Current.StartsAt.Equal(now) || overview.Current.Status != "open" || overview.Current.FinalizedAt != nil || overview.StandardLoadout == nil {
					t.Fatalf("invalid next-week overview: %+v", overview)
				}
				if len(overview.OpenLeaderboard) != 0 || len(overview.StandardLeaderboard) != 0 || len(overview.MyEntries) != 0 || overview.Tokens != 0 {
					t.Fatalf("prior entries or unclaimed tokens leaked into current overview: %+v", overview)
				}
				wantRewards := 0
				if tc.participants > 0 {
					wantRewards = 2
				}
				if len(overview.Rewards) != wantRewards {
					t.Fatalf("user rewards=%+v, want %d", overview.Rewards, wantRewards)
				}
				for _, reward := range overview.Rewards {
					wantRank := 1
					if reward.Division == "standard" {
						wantRank = 11
					}
					tokens, item, cosmetic := fishingTournamentRewardForRank(wantRank)
					if reward.TournamentID != tournament.ID || reward.Rank != wantRank || reward.TokenAmount != tokens || reward.ItemID != item || reward.CosmeticKey != cosmetic || reward.ClaimedAt != nil {
						t.Fatalf("invalid user reward: %+v", reward)
					}
				}
				var status string
				var finalizedAt *time.Time
				if err := s.db.QueryRow(ctx, `SELECT status, finalized_at FROM fishing_tournaments WHERE id = $1`, tournament.ID).Scan(&status, &finalizedAt); err != nil {
					t.Fatal(err)
				}
				if status != "finalized" || finalizedAt == nil || !finalizedAt.Equal(now) {
					t.Fatalf("finalization changed on retry: status=%s finalizedAt=%v", status, finalizedAt)
				}
				var rewardCount, invalidRewards int
				if err := s.db.QueryRow(ctx, `
					SELECT count(*), count(*) FILTER (WHERE
						tournament_id <> $1 OR
						rank <> CASE WHEN division = 'open' THEN user_id ELSE 12 - user_id END OR
						token_amount <> CASE WHEN rank = 1 THEN 120 WHEN rank <= 3 THEN 90 WHEN rank <= 10 THEN 65 ELSE 50 END OR
						item_id <> CASE WHEN rank = 1 THEN 'champion_current_rod' WHEN rank <= 3 THEN 'tournament_current_rod' ELSE '' END OR
						cosmetic_key <> CASE WHEN rank = 1 THEN 'fishing_champion' ELSE '' END OR claimed_at IS NOT NULL)
					FROM fishing_tournament_rewards
				`, tournament.ID).Scan(&rewardCount, &invalidRewards); err != nil {
					t.Fatal(err)
				}
				if rewardCount != tc.participants*2 || invalidRewards != 0 {
					t.Fatalf("reward count=%d invalid=%d", rewardCount, invalidRewards)
				}
				var rewardsJSON string
				if err := s.db.QueryRow(ctx, `SELECT COALESCE(jsonb_agg(to_jsonb(r) ORDER BY division, user_id), '[]'::jsonb)::text FROM fishing_tournament_rewards r`).Scan(&rewardsJSON); err != nil {
					t.Fatal(err)
				}
				if attempt == 0 {
					firstOverview, firstRewardsJSON = overview, rewardsJSON
				} else if firstRewardsJSON != rewardsJSON || !reflect.DeepEqual(firstOverview.Current, overview.Current) {
					t.Fatal("repeated overview changed persisted rewards or current tournament")
				}
			}
		})
	}
}

func TestFishingTournamentPostgresOverviewDoesNotFinalizeEarly(t *testing.T) {
	s, ctx := fishingTournamentPostgresStore(t)
	tournament := seedFishingTournamentPostgres(t, s, ctx, 11)
	overview, err := s.GetFishingTournamentOverview(ctx, 1, tournament.EndsAt.Add(-time.Microsecond))
	if err != nil {
		t.Fatal(err)
	}
	if overview.Current.ID != tournament.ID || overview.Current.Status != "open" || overview.Current.FinalizedAt != nil || len(overview.OpenLeaderboard) != 11 || len(overview.StandardLeaderboard) != 11 || len(overview.MyEntries) != 2 || len(overview.Rewards) != 0 {
		t.Fatalf("unfinished tournament changed: %+v", overview)
	}
	var count int
	if err := s.db.QueryRow(ctx, `SELECT count(*) FROM fishing_tournament_rewards`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("premature rewards=%d err=%v", count, err)
	}
}

func TestFishingTournamentPostgresOverviewRollbackAndRetry(t *testing.T) {
	s, ctx := fishingTournamentPostgresStore(t)
	tournament := seedFishingTournamentPostgres(t, s, ctx, 11)
	// Fail the second division after the first division's rewards have been inserted.
	if _, err := s.db.Exec(ctx, `ALTER TABLE fishing_tournament_rewards ADD CONSTRAINT reject_standard CHECK(division <> 'standard')`); err != nil {
		t.Fatal(err)
	}
	_, err := s.GetFishingTournamentOverview(ctx, 1, tournament.EndsAt)
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "23514" || pgErr.ConstraintName != "reject_standard" {
		t.Fatalf("expected injected reward failure, got %v", err)
	}
	var status string
	var finalizedAt *time.Time
	var rewardCount, tournamentCount int
	if err := s.db.QueryRow(ctx, `
		SELECT status, finalized_at, (SELECT count(*) FROM fishing_tournament_rewards), (SELECT count(*) FROM fishing_tournaments)
		FROM fishing_tournaments WHERE id = $1
	`, tournament.ID).Scan(&status, &finalizedAt, &rewardCount, &tournamentCount); err != nil {
		t.Fatal(err)
	}
	if status != "open" || finalizedAt != nil || rewardCount != 0 || tournamentCount != 1 {
		t.Fatalf("partial settlement committed: status=%s finalizedAt=%v rewards=%d tournaments=%d", status, finalizedAt, rewardCount, tournamentCount)
	}
	if _, err := s.db.Exec(ctx, `ALTER TABLE fishing_tournament_rewards DROP CONSTRAINT reject_standard`); err != nil {
		t.Fatal(err)
	}
	overview, err := s.GetFishingTournamentOverview(ctx, 1, tournament.EndsAt)
	if err != nil || overview.Current.ID == tournament.ID || len(overview.Rewards) != 2 {
		t.Fatalf("retry overview=%+v err=%v", overview, err)
	}
	if err := s.db.QueryRow(ctx, `SELECT count(*) FROM fishing_tournament_rewards`).Scan(&rewardCount); err != nil || rewardCount != 22 {
		t.Fatalf("retry rewards=%d err=%v", rewardCount, err)
	}
}
