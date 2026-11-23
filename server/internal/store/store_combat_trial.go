package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

const (
	CombatTrialBattleKind       = "combat_trial"
	CombatTrialStatusQueued     = "queued"
	CombatTrialStatusRunning    = "running"
	CombatTrialStatusCompleted  = "completed"
	CombatTrialStatusFailed     = "failed"
	CombatTrialHistoryMaxRead   = 50
	CombatTrialLeaderboardLimit = 100
)

type CombatTrialRun struct {
	ID                     int64              `json:"id"`
	BattleKind             string             `json:"battleKind"`
	UserID                 int64              `json:"userId"`
	FormatID               string             `json:"formatId"`
	FormatDefinitionDigest string             `json:"formatDefinitionDigest"`
	Seed                   int64              `json:"seed"`
	Status                 string             `json:"status"`
	WorkerID               string             `json:"-"`
	Attempts               int                `json:"attempts"`
	Lineup                 json.RawMessage    `json:"lineup,omitempty"`
	LineupDigest           string             `json:"lineupDigest,omitempty"`
	FormatSnapshot         json.RawMessage    `json:"formatSnapshot,omitempty"`
	Metrics                map[string]float64 `json:"metrics"`
	ActionCount            int                `json:"actionCount"`
	ActionBudget           int                `json:"actionBudget"`
	EndReason              string             `json:"endReason"`
	KittyCount             int                `json:"kittyCount"`
	CreatedAt              time.Time          `json:"createdAt"`
	StartedAt              *time.Time         `json:"startedAt,omitempty"`
	CompletedAt            *time.Time         `json:"completedAt,omitempty"`
}

type CombatTrialPersonalBest struct {
	UserID                 int64              `json:"userId"`
	FormatID               string             `json:"formatId"`
	FormatDefinitionDigest string             `json:"formatDefinitionDigest"`
	MetricID               string             `json:"metricId"`
	Value                  float64            `json:"value"`
	Metrics                map[string]float64 `json:"metrics"`
	EndReason              string             `json:"endReason"`
	KittyCount             int                `json:"kittyCount"`
	AchievedAt             time.Time          `json:"achievedAt"`
	RunID                  *int64             `json:"runId,omitempty"`
}

type CombatTrialLeaderboardEntry struct {
	Rank       int64              `json:"rank"`
	UserID     int64              `json:"userId"`
	Username   string             `json:"username"`
	MetricID   string             `json:"metricId"`
	Value      float64            `json:"value"`
	Metrics    map[string]float64 `json:"metrics"`
	EndReason  string             `json:"-"`
	KittyCount int                `json:"kittyCount"`
	AchievedAt time.Time          `json:"achievedAt"`
}

type CombatTrialLeaderboardPage struct {
	Entries []CombatTrialLeaderboardEntry
	Self    *CombatTrialLeaderboardEntry
	Total   int64
}

type CombatTrialRunPayload struct {
	RunID        int64              `json:"runId"`
	BattleKind   string             `json:"battleKind"`
	Seed         int64              `json:"seed"`
	Status       string             `json:"status"`
	EndReason    string             `json:"endReason"`
	Metrics      map[string]float64 `json:"metrics"`
	ActionCount  int                `json:"actionCount"`
	ActionBudget int                `json:"actionBudget"`
	InitialState json.RawMessage    `json:"initialState"`
	Timeline     json.RawMessage    `json:"timeline"`
	FinalResult  json.RawMessage    `json:"finalResult"`
}

type CombatTrialReplayShare struct {
	Token          string             `json:"token"`
	RunID          int64              `json:"runId"`
	CreatedBy      int64              `json:"createdBy"`
	CreatedByName  string             `json:"createdByName"`
	EndReason      string             `json:"endReason"`
	Metrics        map[string]float64 `json:"metrics"`
	FormatSnapshot json.RawMessage    `json:"-"`
	CreatedAt      time.Time          `json:"createdAt"`
}

type CombatTrialSharedReplay struct {
	Share   CombatTrialReplayShare `json:"share"`
	Payload CombatTrialRunPayload  `json:"payload"`
}

type CreateCombatTrialRunInput struct {
	UserID                 int64
	FormatID               string
	FormatDefinitionDigest string
	Seed                   int64
	Lineup                 json.RawMessage
	LineupDigest           string
	FormatSnapshot         json.RawMessage
	ActionBudget           int
	KittyCount             int
}

type CompleteCombatTrialRunInput struct {
	RunID                int64
	WorkerID             string
	Metrics              map[string]float64
	MetricRankDirections map[string]string
	ActionCount          int
	EndReason            string
	InitialState         json.RawMessage
	Timeline             json.RawMessage
	FinalResult          json.RawMessage
}

func validCombatTrialResultID(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" || len(value) > 64 {
		return false
	}
	for _, char := range value {
		if (char < 'a' || char > 'z') && (char < '0' || char > '9') && char != '_' && char != ':' && char != '-' {
			return false
		}
	}
	return true
}

func decodeCombatTrialMetrics(raw []byte) (map[string]float64, error) {
	out := map[string]float64{}
	if len(raw) == 0 {
		return out, nil
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func scanCombatTrialRun(row pgx.Row) (CombatTrialRun, error) {
	var out CombatTrialRun
	var lineupRaw, formatRaw, metricsRaw []byte
	err := row.Scan(
		&out.ID, &out.BattleKind, &out.UserID, &out.FormatID, &out.FormatDefinitionDigest, &out.Seed,
		&out.Status, &out.WorkerID, &out.Attempts, &lineupRaw, &out.LineupDigest, &formatRaw, &metricsRaw,
		&out.ActionCount, &out.ActionBudget, &out.EndReason, &out.KittyCount,
		&out.CreatedAt, &out.StartedAt, &out.CompletedAt,
	)
	if err != nil {
		return CombatTrialRun{}, err
	}
	out.Lineup = append(json.RawMessage(nil), lineupRaw...)
	out.FormatSnapshot = append(json.RawMessage(nil), formatRaw...)
	out.Metrics, err = decodeCombatTrialMetrics(metricsRaw)
	return out, err
}

const combatTrialRunSelectColumns = `
	id,battle_kind,user_id,format_id,format_definition_digest,seed,status,worker_id,attempts,
	lineup_jsonb,lineup_digest,format_snapshot_jsonb,metrics_jsonb,action_count,action_budget,end_reason,
	kitty_count,created_at,started_at,completed_at`

func (s *Store) GetActiveCombatTrialRun(ctx context.Context, userID int64) (CombatTrialRun, error) {
	run, err := scanCombatTrialRun(s.db.QueryRow(ctx, `SELECT `+combatTrialRunSelectColumns+` FROM combat_trial_runs WHERE user_id=$1 AND status IN ('queued','running') ORDER BY id DESC LIMIT 1`, userID))
	if errors.Is(err, pgx.ErrNoRows) {
		return CombatTrialRun{}, ErrCombatTrialRunNotFound
	}
	return run, err
}

func (s *Store) CreateCombatTrialRun(ctx context.Context, input CreateCombatTrialRunInput) (CombatTrialRun, error) {
	input.FormatID = strings.TrimSpace(input.FormatID)
	input.FormatDefinitionDigest = strings.TrimSpace(input.FormatDefinitionDigest)
	input.LineupDigest = strings.TrimSpace(input.LineupDigest)
	if input.UserID <= 0 || input.FormatID == "" || input.FormatDefinitionDigest == "" || input.Seed <= 0 || input.ActionBudget <= 0 || input.KittyCount < 0 || input.KittyCount > 2 || input.LineupDigest == "" || !json.Valid(input.Lineup) || !json.Valid(input.FormatSnapshot) {
		return CombatTrialRun{}, ErrBattleRoomConfigInvalid
	}
	run, err := scanCombatTrialRun(s.db.QueryRow(ctx, `
		INSERT INTO combat_trial_runs(user_id,format_id,format_definition_digest,seed,lineup_jsonb,lineup_digest,format_snapshot_jsonb,action_budget,kitty_count)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9)
		RETURNING `+combatTrialRunSelectColumns,
		input.UserID, input.FormatID, input.FormatDefinitionDigest, input.Seed, input.Lineup, input.LineupDigest, input.FormatSnapshot, input.ActionBudget, input.KittyCount,
	))
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == "uq_combat_trial_runs_user_active" {
		return CombatTrialRun{}, ErrCombatTrialRunActive
	}
	return run, err
}

func (s *Store) ClaimCombatTrialRun(ctx context.Context, workerID string, lease time.Duration, now time.Time) (CombatTrialRun, error) {
	workerID = strings.TrimSpace(workerID)
	if workerID == "" || lease <= 0 || now.IsZero() {
		return CombatTrialRun{}, ErrBattleRoomConfigInvalid
	}
	return scanCombatTrialRun(s.db.QueryRow(ctx, `
		WITH candidate AS (
			SELECT id AS run_id FROM combat_trial_runs
			WHERE status='queued' OR (status='running' AND lease_until IS NOT NULL AND lease_until<=$2)
			ORDER BY created_at ASC,id ASC FOR UPDATE SKIP LOCKED LIMIT 1
		), claimed AS (
			UPDATE combat_trial_runs r
			SET status='running',worker_id=$1,lease_until=$3,attempts=r.attempts+1,started_at=COALESCE(r.started_at,$2),updated_at=$2
			FROM candidate WHERE r.id=candidate.run_id
			RETURNING r.*
		)
		SELECT `+combatTrialRunSelectColumns+` FROM claimed`,
		workerID, now, now.Add(lease),
	))
}

func (s *Store) CompleteCombatTrialRun(ctx context.Context, input CompleteCombatTrialRunInput) (CombatTrialRun, error) {
	input.WorkerID = strings.TrimSpace(input.WorkerID)
	input.EndReason = strings.TrimSpace(input.EndReason)
	if input.RunID <= 0 || input.WorkerID == "" || input.ActionCount < 0 ||
		!validCombatTrialResultID(input.EndReason) ||
		len(input.Metrics) == 0 || !json.Valid(input.InitialState) || !json.Valid(input.Timeline) || !json.Valid(input.FinalResult) {
		return CombatTrialRun{}, ErrBattleRoomConfigInvalid
	}
	metricsRaw, err := json.Marshal(input.Metrics)
	if err != nil {
		return CombatTrialRun{}, ErrBattleRoomConfigInvalid
	}
	var out CombatTrialRun
	err = s.withTx(ctx, func(tx pgx.Tx) error {
		var userID int64
		var status, workerID, formatID, formatDigest string
		var kittyCount, actionBudget int
		if err := tx.QueryRow(ctx, `SELECT user_id,status,worker_id,format_id,format_definition_digest,kitty_count,action_budget FROM combat_trial_runs WHERE id=$1 FOR UPDATE`, input.RunID).Scan(&userID, &status, &workerID, &formatID, &formatDigest, &kittyCount, &actionBudget); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrCombatTrialRunNotFound
			}
			return err
		}
		if status != CombatTrialStatusRunning || workerID != input.WorkerID {
			return ErrCombatTrialRunLeaseLost
		}
		if input.ActionCount > actionBudget {
			return ErrBattleRoomConfigInvalid
		}
		if _, err := tx.Exec(ctx, `
			UPDATE combat_trial_runs SET status='completed',worker_id='',lease_until=NULL,error_code='',metrics_jsonb=$2,
				action_count=$3,end_reason=$4,completed_at=now(),updated_at=now() WHERE id=$1
		`, input.RunID, metricsRaw, input.ActionCount, input.EndReason); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO combat_trial_run_payloads(run_id,initial_state_jsonb,timeline_jsonb,final_result_jsonb)
			VALUES($1,$2,$3,$4)
			ON CONFLICT(run_id) DO UPDATE SET initial_state_jsonb=EXCLUDED.initial_state_jsonb,timeline_jsonb=EXCLUDED.timeline_jsonb,final_result_jsonb=EXCLUDED.final_result_jsonb
		`, input.RunID, input.InitialState, input.Timeline, input.FinalResult); err != nil {
			return err
		}
		for metricID, value := range input.Metrics {
			if !validCombatTrialResultID(metricID) || value < 0 || math.IsNaN(value) || math.IsInf(value, 0) {
				return ErrBattleRoomConfigInvalid
			}
		}
		metricIDs := make([]string, 0, len(input.MetricRankDirections))
		for metricID, direction := range input.MetricRankDirections {
			if _, ok := input.Metrics[metricID]; !ok || (direction != "asc" && direction != "desc") {
				return ErrBattleRoomConfigInvalid
			}
			metricIDs = append(metricIDs, metricID)
		}
		sort.Strings(metricIDs)
		for _, metricID := range metricIDs {
			if _, err := tx.Exec(ctx, `
				INSERT INTO combat_trial_personal_bests(user_id,format_id,format_definition_digest,metric_id,value,metrics_jsonb,end_reason,kitty_count,achieved_at,run_id)
				VALUES($1,$2,$3,$4,$5,$6,$7,$8,now(),$9)
				ON CONFLICT(user_id,format_definition_digest,metric_id) DO UPDATE SET
					format_id=EXCLUDED.format_id,value=EXCLUDED.value,metrics_jsonb=EXCLUDED.metrics_jsonb,
					end_reason=EXCLUDED.end_reason,kitty_count=EXCLUDED.kitty_count,achieved_at=EXCLUDED.achieved_at,run_id=EXCLUDED.run_id
				WHERE ($10='desc' AND EXCLUDED.value>combat_trial_personal_bests.value)
					OR ($10='asc' AND EXCLUDED.value<combat_trial_personal_bests.value)
			`, userID, formatID, formatDigest, metricID, input.Metrics[metricID], metricsRaw, input.EndReason, kittyCount, input.RunID, input.MetricRankDirections[metricID]); err != nil {
				return err
			}
		}
		var scanErr error
		out, scanErr = scanCombatTrialRun(tx.QueryRow(ctx, `SELECT `+combatTrialRunSelectColumns+` FROM combat_trial_runs WHERE id=$1`, input.RunID))
		return scanErr
	})
	return out, err
}

func (s *Store) FailCombatTrialRun(ctx context.Context, runID int64, workerID, errorCode string, maxAttempts int) (bool, error) {
	workerID = strings.TrimSpace(workerID)
	if runID <= 0 || workerID == "" || maxAttempts <= 0 {
		return false, ErrBattleRoomConfigInvalid
	}
	var terminal bool
	err := s.withTx(ctx, func(tx pgx.Tx) error {
		var attempts int
		var status, lockedWorker string
		if err := tx.QueryRow(ctx, `SELECT attempts,status,worker_id FROM combat_trial_runs WHERE id=$1 FOR UPDATE`, runID).Scan(&attempts, &status, &lockedWorker); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrCombatTrialRunNotFound
			}
			return err
		}
		if status != CombatTrialStatusRunning || lockedWorker != workerID {
			return ErrCombatTrialRunLeaseLost
		}
		terminal = attempts >= maxAttempts
		nextStatus, completedExpr := CombatTrialStatusQueued, "NULL"
		if terminal {
			nextStatus, completedExpr = CombatTrialStatusFailed, "now()"
		}
		_, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE combat_trial_runs SET status=$2,worker_id='',lease_until=NULL,error_code=$3,completed_at=%s,updated_at=now() WHERE id=$1`, completedExpr), runID, nextStatus, strings.TrimSpace(errorCode))
		return err
	})
	return terminal, err
}

func (s *Store) ListCombatTrialRuns(ctx context.Context, userID int64, limit, offset int) ([]CombatTrialRun, error) {
	if limit <= 0 || limit > CombatTrialHistoryMaxRead {
		limit = CombatTrialHistoryMaxRead
	}
	if offset < 0 {
		offset = 0
	}
	rows, err := s.db.Query(ctx, `SELECT `+combatTrialRunSelectColumns+` FROM combat_trial_runs WHERE user_id=$1 ORDER BY created_at DESC,id DESC LIMIT $2 OFFSET $3`, userID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]CombatTrialRun, 0, limit)
	for rows.Next() {
		run, scanErr := scanCombatTrialRun(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		run.Lineup = nil
		run.LineupDigest = ""
		out = append(out, run)
	}
	return out, rows.Err()
}

func (s *Store) ListCombatTrialPersonalBests(ctx context.Context, userID int64, formatDigest string) ([]CombatTrialPersonalBest, error) {
	rows, err := s.db.Query(ctx, `
		SELECT user_id,format_id,format_definition_digest,metric_id,value,metrics_jsonb,end_reason,kitty_count,achieved_at,run_id
		FROM combat_trial_personal_bests WHERE user_id=$1 AND format_definition_digest=$2 ORDER BY metric_id ASC
	`, userID, strings.TrimSpace(formatDigest))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []CombatTrialPersonalBest{}
	for rows.Next() {
		var row CombatTrialPersonalBest
		var raw []byte
		if err := rows.Scan(&row.UserID, &row.FormatID, &row.FormatDefinitionDigest, &row.MetricID, &row.Value, &raw, &row.EndReason, &row.KittyCount, &row.AchievedAt, &row.RunID); err != nil {
			return nil, err
		}
		row.Metrics, err = decodeCombatTrialMetrics(raw)
		if err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (s *Store) ListCombatTrialLeaderboard(ctx context.Context, userID int64, formatDigest, metricID, rankDirection string, limit int) (CombatTrialLeaderboardPage, error) {
	if limit <= 0 || limit > CombatTrialLeaderboardLimit {
		limit = CombatTrialLeaderboardLimit
	}
	valueOrder := "DESC"
	if rankDirection == "asc" {
		valueOrder = "ASC"
	} else if rankDirection != "desc" {
		return CombatTrialLeaderboardPage{}, ErrBattleRoomConfigInvalid
	}
	rows, err := s.db.Query(ctx, fmt.Sprintf(`
		WITH ranked AS (
			SELECT row_number() OVER(ORDER BY pb.value %s,pb.achieved_at ASC,pb.user_id ASC)::bigint AS rank,
				count(*) OVER()::bigint AS total,pb.user_id,u.username,pb.metric_id,pb.value,pb.metrics_jsonb,pb.end_reason,pb.kitty_count,pb.achieved_at
			FROM combat_trial_personal_bests pb JOIN users u ON u.id=pb.user_id
			WHERE pb.format_definition_digest=$1 AND pb.metric_id=$2
		), page AS (
			SELECT * FROM ranked ORDER BY rank LIMIT $3
		), viewer AS (
			SELECT * FROM ranked WHERE user_id=$4 AND NOT EXISTS(SELECT 1 FROM page WHERE user_id=$4)
		)
		SELECT rank,total,user_id,username,metric_id,value,metrics_jsonb,end_reason,kitty_count,achieved_at,FALSE FROM page
		UNION ALL
		SELECT rank,total,user_id,username,metric_id,value,metrics_jsonb,end_reason,kitty_count,achieved_at,TRUE FROM viewer
		ORDER BY 11 ASC,rank ASC
	`, valueOrder), strings.TrimSpace(formatDigest), strings.TrimSpace(metricID), limit, userID)
	if err != nil {
		return CombatTrialLeaderboardPage{}, err
	}
	defer rows.Close()
	out := CombatTrialLeaderboardPage{Entries: []CombatTrialLeaderboardEntry{}}
	for rows.Next() {
		var row CombatTrialLeaderboardEntry
		var total int64
		var raw []byte
		var selfOnly bool
		if err := rows.Scan(&row.Rank, &total, &row.UserID, &row.Username, &row.MetricID, &row.Value, &raw, &row.EndReason, &row.KittyCount, &row.AchievedAt, &selfOnly); err != nil {
			return CombatTrialLeaderboardPage{}, err
		}
		row.Metrics, err = decodeCombatTrialMetrics(raw)
		if err != nil {
			return CombatTrialLeaderboardPage{}, err
		}
		out.Total = total
		if selfOnly {
			copy := row
			out.Self = &copy
		} else {
			out.Entries = append(out.Entries, row)
			if row.UserID == userID {
				copy := row
				out.Self = &copy
			}
		}
	}
	return out, rows.Err()
}

func (s *Store) GetCombatTrialRunReplay(ctx context.Context, userID, runID int64) (CombatTrialRunPayload, error) {
	var out CombatTrialRunPayload
	var metricsRaw, initialRaw, timelineRaw, finalRaw []byte
	err := s.db.QueryRow(ctx, `
		SELECT r.id,r.battle_kind,r.seed,r.status,r.end_reason,r.metrics_jsonb,r.action_count,r.action_budget,
			p.initial_state_jsonb,p.timeline_jsonb,p.final_result_jsonb
		FROM combat_trial_runs r JOIN combat_trial_run_payloads p ON p.run_id=r.id
		WHERE r.id=$1 AND r.user_id=$2
	`, runID, userID).Scan(&out.RunID, &out.BattleKind, &out.Seed, &out.Status, &out.EndReason, &metricsRaw, &out.ActionCount, &out.ActionBudget, &initialRaw, &timelineRaw, &finalRaw)
	if errors.Is(err, pgx.ErrNoRows) {
		return CombatTrialRunPayload{}, ErrCombatTrialRunNotFound
	}
	if err != nil {
		return CombatTrialRunPayload{}, err
	}
	out.Metrics, err = decodeCombatTrialMetrics(metricsRaw)
	out.InitialState = append(json.RawMessage(nil), initialRaw...)
	out.Timeline = append(json.RawMessage(nil), timelineRaw...)
	out.FinalResult = append(json.RawMessage(nil), finalRaw...)
	return out, err
}

func (s *Store) PruneCombatTrialRuns(ctx context.Context, userID int64, keepCount int) (int64, error) {
	if userID <= 0 || keepCount <= 0 || keepCount > CombatTrialHistoryMaxRead {
		return 0, ErrBattleRoomConfigInvalid
	}
	command, err := s.db.Exec(ctx, `
		WITH removable AS (
			SELECT id FROM combat_trial_runs WHERE user_id=$1 AND status IN ('completed','failed')
			ORDER BY created_at DESC,id DESC OFFSET $2
		)
		DELETE FROM combat_trial_runs r USING removable x WHERE r.id=x.id
	`, userID, keepCount)
	if err != nil {
		return 0, err
	}
	return command.RowsAffected(), nil
}

func (s *Store) CreateCombatTrialReplayShare(ctx context.Context, createdBy, runID int64, token string) (CombatTrialReplayShare, error) {
	token = strings.TrimSpace(token)
	if createdBy <= 0 || runID <= 0 || len(token) < 32 || len(token) > 128 {
		return CombatTrialReplayShare{}, ErrBattleRoomConfigInvalid
	}
	var out CombatTrialReplayShare
	var metricsRaw []byte
	err := s.db.QueryRow(ctx, `
		WITH eligible AS (
			SELECT r.id,r.end_reason,r.metrics_jsonb FROM combat_trial_runs r
			JOIN combat_trial_run_payloads p ON p.run_id=r.id
			WHERE r.id=$2 AND r.user_id=$1 AND r.status='completed'
		), inserted AS (
			INSERT INTO combat_trial_replay_shares(token,run_id,created_by)
			SELECT $3,e.id,$1 FROM eligible e
			ON CONFLICT(run_id,created_by) DO UPDATE SET
				token=CASE WHEN combat_trial_replay_shares.revoked_at IS NULL THEN combat_trial_replay_shares.token ELSE EXCLUDED.token END,
				created_at=CASE WHEN combat_trial_replay_shares.revoked_at IS NULL THEN combat_trial_replay_shares.created_at ELSE now() END,
				revoked_at=CASE WHEN combat_trial_replay_shares.revoked_at IS NULL THEN combat_trial_replay_shares.revoked_at ELSE NULL END
			RETURNING token,run_id,created_by,created_at
		)
		SELECT i.token,i.run_id,i.created_by,u.username,e.end_reason,e.metrics_jsonb,i.created_at
		FROM inserted i JOIN eligible e ON e.id=i.run_id JOIN users u ON u.id=i.created_by
	`, createdBy, runID, token).Scan(&out.Token, &out.RunID, &out.CreatedBy, &out.CreatedByName, &out.EndReason, &metricsRaw, &out.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return CombatTrialReplayShare{}, ErrCombatTrialRunNotFound
	}
	if err != nil {
		return CombatTrialReplayShare{}, err
	}
	out.Metrics, err = decodeCombatTrialMetrics(metricsRaw)
	return out, err
}

func (s *Store) GetCombatTrialReplayShare(ctx context.Context, token string) (CombatTrialReplayShare, error) {
	token = strings.TrimSpace(token)
	if len(token) < 32 || len(token) > 128 {
		return CombatTrialReplayShare{}, ErrCombatTrialReplayUnavailable
	}
	var out CombatTrialReplayShare
	var metricsRaw, formatRaw []byte
	err := s.db.QueryRow(ctx, `
		SELECT s.token,s.run_id,s.created_by,u.username,r.end_reason,r.metrics_jsonb,r.format_snapshot_jsonb,s.created_at
		FROM combat_trial_replay_shares s
		JOIN combat_trial_runs r ON r.id=s.run_id JOIN combat_trial_run_payloads p ON p.run_id=r.id JOIN users u ON u.id=s.created_by
		WHERE s.token=$1 AND s.revoked_at IS NULL
	`, token).Scan(&out.Token, &out.RunID, &out.CreatedBy, &out.CreatedByName, &out.EndReason, &metricsRaw, &formatRaw, &out.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return CombatTrialReplayShare{}, ErrCombatTrialReplayUnavailable
	}
	if err != nil {
		return CombatTrialReplayShare{}, err
	}
	out.Metrics, err = decodeCombatTrialMetrics(metricsRaw)
	out.FormatSnapshot = append(json.RawMessage(nil), formatRaw...)
	return out, err
}

func (s *Store) GetCombatTrialSharedReplay(ctx context.Context, token string) (CombatTrialSharedReplay, error) {
	token = strings.TrimSpace(token)
	if len(token) < 32 || len(token) > 128 {
		return CombatTrialSharedReplay{}, ErrCombatTrialReplayUnavailable
	}
	var out CombatTrialSharedReplay
	var shareMetricsRaw, payloadMetricsRaw, initialRaw, timelineRaw, finalRaw []byte
	err := s.db.QueryRow(ctx, `
		SELECT s.token,s.run_id,s.created_by,u.username,r.end_reason,r.metrics_jsonb,s.created_at,
			r.id,r.battle_kind,r.seed,r.status,r.end_reason,r.metrics_jsonb,r.action_count,r.action_budget,
			p.initial_state_jsonb,p.timeline_jsonb,p.final_result_jsonb
		FROM combat_trial_replay_shares s
		JOIN combat_trial_runs r ON r.id=s.run_id JOIN combat_trial_run_payloads p ON p.run_id=r.id JOIN users u ON u.id=s.created_by
		WHERE s.token=$1 AND s.revoked_at IS NULL
	`, token).Scan(
		&out.Share.Token, &out.Share.RunID, &out.Share.CreatedBy, &out.Share.CreatedByName, &out.Share.EndReason, &shareMetricsRaw, &out.Share.CreatedAt,
		&out.Payload.RunID, &out.Payload.BattleKind, &out.Payload.Seed, &out.Payload.Status, &out.Payload.EndReason, &payloadMetricsRaw, &out.Payload.ActionCount, &out.Payload.ActionBudget,
		&initialRaw, &timelineRaw, &finalRaw,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return CombatTrialSharedReplay{}, ErrCombatTrialReplayUnavailable
	}
	if err != nil {
		return CombatTrialSharedReplay{}, err
	}
	out.Share.Metrics, err = decodeCombatTrialMetrics(shareMetricsRaw)
	if err != nil {
		return CombatTrialSharedReplay{}, err
	}
	out.Payload.Metrics, err = decodeCombatTrialMetrics(payloadMetricsRaw)
	out.Payload.InitialState = append(json.RawMessage(nil), initialRaw...)
	out.Payload.Timeline = append(json.RawMessage(nil), timelineRaw...)
	out.Payload.FinalResult = append(json.RawMessage(nil), finalRaw...)
	return out, err
}
