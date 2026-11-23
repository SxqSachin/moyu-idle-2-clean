package store

import (
	"errors"
	"reflect"
	"strconv"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

func TestOfflineRecoveryReportProjection(t *testing.T) {
	start := time.Unix(10000, 0)
	rows := []OfflineRecovery{
		{ID: 1, Revision: 3, Status: "complete", StartedAt: start, TargetAt: start.Add(time.Minute), Summary: ActionFastForwardSummary{
			CompletedSteps: 10, ItemRewards: map[string]int64{"wood": 10}, ItemCosts: map[string]int64{"coal": 2}, AttrExp: map[string]float64{"strength": 1},
			ActorSummaries:        []ActionActorSummary{{ActorType: ActionActorTypeKitty, ActorID: 2, CompletedSteps: 10, ItemRewards: map[string]int64{"wood": 10}, OwnerAttrExp: map[string]float64{"fishing": 1}, EquipmentChanged: true}},
			ActivatedWorkLoadouts: []ActivatedWorkLoadout{{}}, FishingChanged: true,
		}},
		{ID: 2, Revision: 0, Status: "stalled", StartedAt: start.Add(time.Hour), TargetAt: start.Add(2 * time.Hour), Summary: ActionFastForwardSummary{
			CompletedSteps: 20, ItemRewards: map[string]int64{"wood": 20}, StalledQueueID: 99, SettledUntilMS: 123,
			ActorSummaries: []ActionActorSummary{{ActorType: ActionActorTypeKitty, ActorID: 2, CompletedSteps: 20, ItemRewards: map[string]int64{"wood": 20}, OwnerAttrExp: map[string]float64{"fishing": 2}}},
		}},
		{ID: 3, Revision: 5, Status: "running", StartedAt: start.Add(3 * time.Hour), TargetAt: start.Add(4 * time.Hour)},
	}
	report := buildOfflineRecoveryReport(rows)
	if report.Recovery.ID != 2 || report.Recovery.Status != "stalled" || report.Recovery.Revision != 0 || report.Recovery.Summary.StalledQueueID != 99 {
		t.Fatal("retry did not address first unfinished member", report)
	}
	summary := report.Recovery.Summary
	if summary.CompletedSteps != 30 || summary.ItemRewards["wood"] != 30 || summary.ItemCosts["coal"] != 2 || summary.AttrExp["strength"] != 1 ||
		!summary.FishingChanged || len(summary.ActivatedWorkLoadouts) != 1 || len(summary.ActorSummaries) != 1 || !summary.ActorSummaries[0].EquipmentChanged || summary.ActorSummaries[0].CompletedSteps != 30 {
		t.Fatal("report lost cumulative domains", summary)
	}
	if summary.ActorSummaries[0].OwnerAttrExp["fishing"] != 3 {
		t.Fatal("report lost kitty owner experience", summary.ActorSummaries)
	}
	if report.DurationSeconds != 7260 || len(report.Tasks) != 3 {
		t.Fatal("online gaps must not count as offline time", report)
	}
	if again := buildOfflineRecoveryReport(rows); again.Key != report.Key || !reflect.DeepEqual(again.Recovery.Summary, report.Recovery.Summary) {
		t.Fatal("report read mutated input")
	}
	rows[1].Status, rows[2].Status = "complete", "complete"
	rows[1].Revision++
	complete := buildOfflineRecoveryReport(rows)
	if complete.Recovery.ID != 3 || complete.Recovery.Status != "complete" || complete.Key == report.Key {
		t.Fatal("member revision did not invalidate report snapshot")
	}
}

func TestOfflineRecoveryReportPostgresAtomicAcknowledgement(t *testing.T) {
	s, ctx, _ := recoveryPostgresStore(t)
	start := time.Now().Truncate(time.Second).Add(-8 * time.Hour)
	insert := func(userID int64, offset time.Duration, status string, summary string, background bool) OfflineRecoveryReceipt {
		t.Helper()
		var id int64
		if err := s.db.QueryRow(ctx, `INSERT INTO player_offline_recoveries(user_id,started_at,target_at,resumed_at,status,revision,summary,background)
 VALUES($1,$2,$3,$3,$4,3,$5,$6) RETURNING id`, userID, start.Add(offset), start.Add(offset+time.Hour), status, summary, background).Scan(&id); err != nil {
			t.Fatal(err)
		}
		return OfflineRecoveryReceipt{TaskID: strconv.FormatInt(id, 10), Revision: 3}
	}
	first := insert(970071, 0, "complete", `{"completedSteps":10,"itemRewards":{"wood":10}}`, false)
	second := insert(970071, 2*time.Hour, "running", `{"completedSteps":20,"itemRewards":{"wood":20}}`, false)
	foreign := insert(970072, 0, "complete", `{}`, false)
	background := insert(970071, 4*time.Hour, "complete", `{}`, true)
	assertUnacknowledged := func() {
		t.Helper()
		var count int
		if err := s.db.QueryRow(ctx, `SELECT count(*) FROM player_offline_recoveries WHERE acknowledged_at IS NOT NULL`).Scan(&count); err != nil || count != 0 {
			t.Fatal("partial or premature acknowledgement", count, err)
		}
	}
	report, err := s.GetOfflineRecoveryReport(ctx, 970071)
	if err != nil || len(report.Tasks) != 2 || report.Recovery.Status != "running" || report.Recovery.Summary.ItemRewards["wood"] != 30 {
		t.Fatal("backlog report not combined or background leaked", report, err)
	}
	stale := second
	stale.Revision--
	for _, tasks := range [][]OfflineRecoveryReceipt{{first, second}, {first, stale}, {first, foreign}, {first, background}, {first, {TaskID: "999999", Revision: 3}}} {
		if err := s.AcknowledgeOfflineRecoveryReport(ctx, 970071, tasks); err != nil {
			t.Fatal(err)
		}
		assertUnacknowledged()
	}
	for _, tasks := range [][]OfflineRecoveryReceipt{{first, first}, {{TaskID: "invalid", Revision: 0}}, {{TaskID: first.TaskID, Revision: -1}}} {
		if err := s.AcknowledgeOfflineRecoveryReport(ctx, 970071, tasks); err == nil {
			t.Fatal("invalid receipt accepted", tasks)
		}
		assertUnacknowledged()
	}
	if _, err := s.db.Exec(ctx, `UPDATE player_offline_recoveries SET status='complete' WHERE id=$1`, second.TaskID); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if err := s.AcknowledgeOfflineRecoveryReport(ctx, 970071, report.Tasks); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := s.GetOfflineRecoveryReport(ctx, 970071); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatal("acknowledged report returned again", err)
	}
	third := insert(970071, 6*time.Hour, "complete", `{"completedSteps":7,"itemRewards":{"ore":7}}`, false)
	if err := s.AcknowledgeOfflineRecoveryReport(ctx, 970071, report.Tasks); err != nil {
		t.Fatal(err)
	}
	next, err := s.GetOfflineRecoveryReport(ctx, 970071)
	if err != nil || len(next.Tasks) != 1 || next.Tasks[0] != third || next.Recovery.Summary.ItemRewards["ore"] != 7 {
		t.Fatal("old ACK consumed a new absence", next, err)
	}
}

func TestOfflineRecoveryReportPostgresCombatUnion(t *testing.T) {
	s, ctx, _ := recoveryPostgresStore(t)
	start := time.Now().Truncate(time.Second).Add(-6 * time.Hour)
	var roomID int64
	if err := s.db.QueryRow(ctx, `INSERT INTO battle_rooms(owner_id,dungeon_id) VALUES(970071,'test') RETURNING id`).Scan(&roomID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.Exec(ctx, `INSERT INTO battle_runs(id,room_id,status) VALUES(123,$1,$2),(124,$1,$2),(125,$1,$2)`, roomID, BattleRunStatusRunning); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.Exec(ctx, `INSERT INTO player_offline_recoveries(user_id,started_at,target_at,resumed_at,status,pending_run_ids)
 VALUES(970071,$1::timestamptz,$1::timestamptz+interval '1 hour',$1::timestamptz+interval '1 hour','complete','{123}'),
 (970071,$1::timestamptz+interval '2 hour',$1::timestamptz+interval '3 hour',$1::timestamptz+interval '3 hour','complete','{123,124}')`, start); err != nil {
		t.Fatal(err)
	}
	report, err := s.GetOfflineRecoveryReport(ctx, 970071)
	if err != nil {
		t.Fatal(err)
	}
	combat, err := s.GetOfflineRecoveryReportCombat(ctx, 970071, report.Tasks)
	if err != nil || combat.PendingRuns != 2 {
		t.Fatal("shared captured run counted twice", combat, err)
	}
	if err := s.AcknowledgeOfflineRecoveryReport(ctx, 970071, report.Tasks); err != nil {
		t.Fatal(err)
	}
	if err := s.AcknowledgeOfflineRecoveryReportKey(ctx, 970071, report.Recovery.ID, report.Key); err != nil {
		t.Fatal(err)
	}
	still, err := s.GetOfflineRecoveryReport(ctx, 970071)
	if err != nil || len(still.Tasks) != 2 {
		t.Fatal("early ACK hid pending battles", err)
	}
	if _, err := s.db.Exec(ctx, `INSERT INTO battle_reward_receipts(run_id,user_id,item_rewards,settled_at)
 VALUES(123,970071,'{"wood":7}',$1::timestamptz+interval '150 minute'),(124,970071,'{"wood":3}',$1::timestamptz+interval '4 hour'),
 (125,970071,'{"wood":100}',$1::timestamptz+interval '90 minute');
`, start); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.Exec(ctx, `INSERT INTO player_warehouse_change_logs(user_id,event_type,template_item_id,item_totals,category,direction,details,created_at)
 VALUES(970071,'system_to_warehouse','wood','{"wood":7}','battle','gain','{"runId":123}',$1::timestamptz+interval '150 minute'),
 (970071,'system_to_warehouse','wood','{"wood":2}','battle','gain','{"runId":99}',$1::timestamptz+interval '30 minute'),
 (970071,'system_to_warehouse','wood','{"wood":200}','battle','gain','{}',$1::timestamptz+interval '90 minute')`, start); err != nil {
		t.Fatal(err)
	}
	combat, err = s.GetOfflineRecoveryReportCombat(ctx, 970071, report.Tasks)
	if err != nil || combat.PendingRuns != 0 || combat.Items["wood"] != 12 || !combat.LegacyLogs {
		t.Fatal("late receipts/legacy logs duplicated or online gap counted", combat, err)
	}
	if err := s.AcknowledgeOfflineRecoveryReportKey(ctx, 970071, report.Recovery.ID, report.Key); err != nil {
		t.Fatal(err)
	}
	if _, err := s.GetOfflineRecoveryReport(ctx, 970071); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatal("settled report could not be acknowledged", err)
	}
}

func TestOfflineRecoveryReportPostgresKeyAcknowledgement(t *testing.T) {
	s, ctx, _ := recoveryPostgresStore(t)
	if _, err := s.db.Exec(ctx, `INSERT INTO player_offline_recoveries(user_id,started_at,target_at,resumed_at,status)
 SELECT 970071,'2026-01-01'::timestamptz+n*interval '2 hour','2026-01-01'::timestamptz+n*interval '2 hour'+interval '1 hour',
 '2026-01-01'::timestamptz+n*interval '2 hour'+interval '1 hour','complete' FROM generate_series(1,2000) n`); err != nil {
		t.Fatal(err)
	}
	report, err := s.GetOfflineRecoveryReport(ctx, 970071)
	if err != nil || len(report.Tasks) != 2000 || len(report.Key) != 64 {
		t.Fatal("large report membership missing", len(report.Tasks), err)
	}
	if _, err := s.db.Exec(ctx, `UPDATE player_offline_recoveries SET revision=revision+1 WHERE id=$1`, report.Tasks[0].TaskID); err != nil {
		t.Fatal(err)
	}
	for _, userID := range []int64{970071, 970072} {
		if err := s.AcknowledgeOfflineRecoveryReportKey(ctx, userID, report.Recovery.ID, report.Key); err != nil {
			t.Fatal(err)
		}
	}
	current, err := s.GetOfflineRecoveryReport(ctx, 970071)
	if err != nil || len(current.Tasks) != 2000 || current.Key == report.Key {
		t.Fatal("stale/mixed-owner digest acknowledged a changed report", len(current.Tasks), err)
	}
	if _, err := s.db.Exec(ctx, `INSERT INTO player_offline_recoveries(user_id,started_at,target_at,resumed_at,status)
 VALUES(970071,'2027-01-01','2027-01-02','2027-01-02','complete')`); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if err := s.AcknowledgeOfflineRecoveryReportKey(ctx, 970071, current.Recovery.ID, current.Key); err != nil {
			t.Fatal(err)
		}
	}
	next, err := s.GetOfflineRecoveryReport(ctx, 970071)
	if err != nil || len(next.Tasks) != 1 || next.Recovery.ID <= current.Recovery.ID {
		t.Fatal("compact ACK consumed a later absence", len(next.Tasks), err)
	}
}
