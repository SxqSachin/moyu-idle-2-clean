package store

import (
	"context"
	"math"
	"sort"

	"github.com/jackc/pgx/v5"
)

type ordinaryExperienceRow struct {
	key      string
	after    []float64
	versions []int64
}

type ordinaryExperiencePlan struct {
	limit int
	rows  []ordinaryExperienceRow
}

// Compute the exact r4 additions and version increments in memory. A level
// changing completion uses the existing single-operation owner, including its
// derived-stat and other-running-queue refresh hooks.
func (s *Store) planOrdinaryExperienceTx(ctx context.Context, tx pgx.Tx, userID int64, gain map[string]float64, limit int) (ordinaryExperiencePlan, error) {
	plan := ordinaryExperiencePlan{limit: limit}
	keys := make([]string, 0, len(gain))
	for key, value := range gain {
		if math.IsNaN(value) || math.IsInf(value, 0) || value < 0 {
			return plan, ErrBadQuantity
		}
		if roundFloat(value, 4) != 0 {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	if len(keys) == 0 || limit <= 1 {
		return plan, nil
	}
	rows, err := tx.Query(ctx, `SELECT attr_key,attr_exp FROM player_attributes WHERE user_id=$1 AND attr_key=ANY($2::text[]) ORDER BY attr_key FOR UPDATE`, userID, keys)
	if err != nil {
		return plan, err
	}
	before := map[string]float64{}
	for rows.Next() {
		var key string
		var exp float64
		if err := rows.Scan(&key, &exp); err != nil {
			rows.Close()
			return plan, err
		}
		before[key] = exp
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return plan, err
	}
	if len(before) != len(keys) {
		plan.limit = 1
		return plan, nil
	}
	for _, key := range keys {
		exp := before[key]
		progress, err := s.resolveAttributeProgress(key, exp)
		if err != nil {
			return plan, err
		}
		row := ordinaryExperienceRow{key: key, after: make([]float64, 0, limit), versions: make([]int64, 0, limit)}
		var version int64
		for i := 0; i < limit; i++ {
			after, err := s.attributeExpProfile.clampTotalExp(key, roundFloat(exp+roundFloat(gain[key], 4), 4))
			if err != nil {
				return plan, err
			}
			after = roundFloat(after, 4)
			next, err := s.resolveAttributeProgress(key, after)
			if err != nil {
				return plan, err
			}
			if next.Level != progress.Level {
				plan.limit = min(plan.limit, max(1, i))
				break
			}
			if math.Abs(after-exp) >= 1e-9 {
				version++
			}
			row.after = append(row.after, after)
			row.versions = append(row.versions, version)
			exp = after
		}
		plan.rows = append(plan.rows, row)
	}
	return plan, nil
}

func (plan ordinaryExperiencePlan) flushTx(ctx context.Context, tx pgx.Tx, userID int64, count int) error {
	for _, row := range plan.rows {
		if count <= 0 || count > len(row.after) {
			return ErrBadQuantity
		}
		if row.versions[count-1] == 0 {
			continue
		}
		if _, err := tx.Exec(ctx, `UPDATE player_attributes SET attr_exp=$3,lock_version=lock_version+$4,updated_at=now() WHERE user_id=$1 AND attr_key=$2`, userID, row.key, row.after[count-1], row.versions[count-1]); err != nil {
			return err
		}
	}
	return nil
}
