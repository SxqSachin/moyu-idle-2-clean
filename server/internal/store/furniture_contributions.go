package store

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"github.com/jackc/pgx/v5"
	"mysecondwebgame/server/internal/operationeffect"
	"time"
)

type FurnitureContributionReport struct {
	Token   string                        `json:"token"`
	Through time.Time                     `json:"through"`
	Rows    operationeffect.Contributions `json:"rows"`
	Totals  operationeffect.Contributions `json:"totals,omitempty"`
	Names   map[string]string             `json:"names,omitempty"`
}

func validateBattleContributions(reward BattleWaveReward) error {
	if err := reward.Contributions.Validate(); err != nil {
		return err
	}
	totals := map[string]int64{}
	for _, r := range reward.Contributions {
		if len(r.Experience) > 0 {
			return errors.New("battle furniture cannot attribute experience")
		}
		for id, n := range r.Items {
			totals[id] += n
			if totals[id] > reward.ItemRewards[id] {
				return errors.New("attribution exceeds granted reward")
			}
		}
	}
	return nil
}

// Committed atomically with the existing reward plan. The caller already holds
// the user lock and run idempotency gate.
func (s *Store) recordEffectContributionsTx(ctx context.Context, tx pgx.Tx, userID int64, delta operationeffect.Contributions) error {
	if len(delta) == 0 {
		return nil
	}
	return s.flushFurnitureAccountingTx(ctx, tx, userID, delta, nil)
}

// One atomic increment, shared by battle and life actions. The returned value
// is validated inside the caller's transaction; it is never a cache writeback.
func (s *Store) flushFurnitureAccountingTx(ctx context.Context, tx pgx.Tx, userID int64, delta operationeffect.Contributions, remainders map[string]int64) error {
	if err := delta.Validate(); err != nil {
		return err
	}
	if delta == nil {
		delta = operationeffect.Contributions{}
	}
	raw, err := json.Marshal(delta)
	if err != nil {
		return err
	}
	var remainderRaw any
	if remainders != nil {
		for _, n := range remainders {
			if n < 0 || n >= 10000 {
				return errors.New("invalid furniture remainder")
			}
		}
		b, err := json.Marshal(remainders)
		if err != nil {
			return err
		}
		remainderRaw = string(b)
	}
	var totalRaw []byte
	if err = tx.QueryRow(ctx, `UPDATE player_cattery_furniture_state SET
	 contribution_totals=cattery_merge_contributions(contribution_totals,$2::jsonb),
	 exp_remainders=COALESCE($3::jsonb,exp_remainders) WHERE user_id=$1 RETURNING contribution_totals`, userID, string(raw), remainderRaw).Scan(&totalRaw); err != nil {
		return err
	}
	var total operationeffect.Contributions
	if err = json.Unmarshal(totalRaw, &total); err != nil {
		return err
	}
	return total.Validate()
}

// A persisted report is stable across reconnects. New commits remain in totals
// and enter the next report, never the already displayed/acknowledged snapshot.
func (s *Store) FurnitureReport(ctx context.Context, userID int64, ack string) (FurnitureContributionReport, error) {
	out := FurnitureContributionReport{Rows: operationeffect.Contributions{}}
	err := s.withTx(ctx, func(tx pgx.Tx) error {
		if err := s.lockUserTx(ctx, tx, userID); err != nil {
			return err
		}
		var totalRaw, seenRaw, reportRaw []byte
		err := tx.QueryRow(ctx, `SELECT contribution_totals,contribution_seen,contribution_report FROM player_cattery_furniture_state WHERE user_id=$1 FOR UPDATE`, userID).Scan(&totalRaw, &seenRaw, &reportRaw)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
		if err = json.Unmarshal(reportRaw, &out); err != nil {
			return err
		}
		if ack != "" {
			if out.Token == "" {
				return nil
			}
			if ack != out.Token {
				return ErrFurnitureConflict
			}
			seen, err := json.Marshal(out.Totals)
			if err != nil {
				return err
			}
			_, err = tx.Exec(ctx, `UPDATE player_cattery_furniture_state SET contribution_seen=$2,contribution_report='{}'::jsonb WHERE user_id=$1`, userID, string(seen))
			return err
		}
		if out.Token != "" {
			return nil
		}
		var total, seen operationeffect.Contributions
		if err = json.Unmarshal(totalRaw, &total); err != nil {
			return err
		}
		if err = json.Unmarshal(seenRaw, &seen); err != nil {
			return err
		}
		if err = total.Validate(); err != nil {
			return err
		}
		if err = seen.Validate(); err != nil {
			return err
		}
		out.Rows = operationeffect.ContributionDifference(total, seen)
		if len(out.Rows) == 0 {
			return nil
		}
		out.Totals = total
		out.Through = time.Now().UTC()
		out.Names = map[string]string{}
		for _, d := range s.gameData.CatteryFurniture {
			out.Names[d.ID] = d.NameKey
		}
		raw, err := json.Marshal([]any{userID, out})
		if err != nil {
			return err
		}
		hash := sha256.Sum256(raw)
		out.Token = hex.EncodeToString(hash[:])
		raw, err = json.Marshal(out)
		if err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `UPDATE player_cattery_furniture_state SET contribution_report=$2 WHERE user_id=$1`, userID, string(raw))
		return err
	})
	out.Totals = nil
	return out, err
}
