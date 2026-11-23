package store

import (
	"encoding/json"
	"errors"
	"github.com/jackc/pgx/v5"
	"mysecondwebgame/server/internal/catteryfurniture"
	"mysecondwebgame/server/internal/operationeffect"
	"os"
	"testing"
	"time"
)

func TestFurniturePostgresUpgradeQuoteAndAtomicity(t *testing.T) {
	s, ctx := furniturePostgresStore(t)
	root := &s.gameData.CatteryFurniture[0]
	root.Levels = []catteryfurniture.Level{{Level: 2, UpgradeCosts: map[string]int64{"wood": 20, "fiber": 10}, RefillCostAdd: map[string]int64{"wood": 3, "fiber": 1}, Rules: []operationeffect.PassiveRule{{Kind: operationeffect.KindOwnerExperience, Domain: operationeffect.DomainLifeAction, RateBPS: 1600}}}}
	const uid int64 = 970041
	p, err := s.CommandFurniture(ctx, uid, FurnitureCommand{RequestID: "install-0001", FurnitureID: "test_desk", Version: "v1", Action: "install"})
	if err != nil {
		t.Fatal(err)
	}
	q := p.Items[0].Upgrade
	if q == nil || q.TotalCosts["wood"] != 30 || q.TotalCosts["fiber"] != 14 {
		t.Fatalf("wrong quote: %+v", q)
	}
	cmd := FurnitureCommand{RequestID: "upgrade-0001", FurnitureID: "test_desk", Version: "v1", Revision: p.Revision, Action: "upgrade", UpgradeQuote: &q.FurnitureQuoteIdentity}
	bad := cmd
	id := *cmd.UpgradeQuote
	id.RemainingMS--
	bad.UpgradeQuote = &id
	if _, err = s.CommandFurniture(ctx, uid, bad); !errors.Is(err, ErrFurnitureConflict) {
		t.Fatalf("tampered quote: %v", err)
	}
	if _, err = s.db.Exec(ctx, `UPDATE player_item_instances SET quantity=0 WHERE template_item_id='wood'`); err != nil {
		t.Fatal(err)
	}
	if _, err = s.CommandFurniture(ctx, uid, cmd); !errors.Is(err, ErrInsufficientItem) {
		t.Fatalf("expected failure: %v", err)
	}
	after, err := s.GetFurniture(ctx, uid)
	if err != nil {
		t.Fatal(err)
	}
	if after.Revision != p.Revision || after.Inventory["fiber"] != 990 || after.Items[0].Definition.CurrentLevel() != 1 {
		t.Fatal("partial upgrade committed")
	}
	if _, err = s.db.Exec(ctx, `UPDATE player_item_instances SET quantity=990 WHERE template_item_id='wood'`); err != nil {
		t.Fatal(err)
	}
	p, err = s.CommandFurniture(ctx, uid, cmd)
	if err != nil {
		t.Fatal(err)
	}
	if p.Items[0].Definition.CurrentLevel() != 2 || p.Items[0].RemainingMS != 120000000 || p.Items[0].EnabledUntil != nil || p.Inventory["wood"] != 960 || p.Inventory["fiber"] != 976 || p.Items[0].Upgrade != nil {
		t.Fatalf("upgrade changed service or costs: %+v", p)
	}
	again, err := s.CommandFurniture(ctx, uid, cmd)
	if err != nil || again.Revision != p.Revision || again.Inventory["wood"] != 960 {
		t.Fatalf("replay: %v %+v", err, again)
	}
	// Exhaustion must resolve the current catalog at the stored level, not reset it.
	if _, err = s.db.Exec(ctx, `UPDATE player_cattery_furniture SET remaining_ms=0`); err != nil {
		t.Fatal(err)
	}
	p, err = s.GetFurniture(ctx, uid)
	if err != nil {
		t.Fatal(err)
	}
	if p.Items[0].Definition.CurrentLevel() != 2 || p.Items[0].Definition.RefillCosts["wood"] != 5 {
		t.Fatal("exhaustion reset level")
	}
}

func TestFurniturePostgresContributionReportLateCommitsAndRollback(t *testing.T) {
	s, ctx := furniturePostgresStore(t)
	const uid int64 = 970041
	if _, err := s.db.Exec(ctx, `INSERT INTO player_cattery_furniture_state(user_id) VALUES($1)`, uid); err != nil {
		t.Fatal(err)
	}
	delta := operationeffect.Contributions{"test_desk": {Experience: map[string]float64{"gather": 1.2345}}, "cabinet": {Items: map[string]int64{"wood": 2}, Opportunities: 3, Triggered: 1}}
	commit := func(rollback bool) error {
		return s.withTx(ctx, func(tx pgx.Tx) error {
			if err := s.lockUserTx(ctx, tx, uid); err != nil {
				return err
			}
			if err := s.recordEffectContributionsTx(ctx, tx, uid, delta); err != nil {
				return err
			}
			if rollback {
				return errors.New("simulated reward failure")
			}
			return nil
		})
	}
	if err := commit(true); err == nil {
		t.Fatal("expected rollback")
	}
	report, err := s.FurnitureReport(ctx, uid, "")
	if err != nil || len(report.Rows) != 0 {
		t.Fatalf("rolled back contribution visible: %+v %v", report, err)
	}
	if err = commit(false); err != nil {
		t.Fatal(err)
	}
	report, err = s.FurnitureReport(ctx, uid, "")
	if err != nil || report.Token == "" || report.Totals != nil {
		t.Fatalf("bad report %+v %v", report, err)
	}
	if err = commit(false); err != nil {
		t.Fatal(err)
	}
	retry, err := s.FurnitureReport(ctx, uid, "")
	if err != nil || retry.Token != report.Token || retry.Rows["cabinet"].Items["wood"] != 2 {
		t.Fatal("open report changed")
	}
	if _, err = s.FurnitureReport(ctx, uid, "wrong"); !errors.Is(err, ErrFurnitureConflict) {
		t.Fatal("wrong ack accepted")
	}
	if _, err = s.FurnitureReport(ctx, uid, report.Token); err != nil {
		t.Fatal(err)
	}
	if _, err = s.FurnitureReport(ctx, uid, report.Token); err != nil {
		t.Fatal("ack retry failed", err)
	}
	next, err := s.FurnitureReport(ctx, uid, "")
	if err != nil || next.Token == report.Token || next.Rows["cabinet"].Items["wood"] != 2 || next.Rows["test_desk"].Experience["gather"] != 1.2345 {
		t.Fatalf("late contribution lost: %+v %v", next, err)
	}
	if _, err = s.FurnitureReport(ctx, uid, report.Token); !errors.Is(err, ErrFurnitureConflict) {
		t.Fatal("old ack consumed next report")
	}
	other, err := s.FurnitureReport(ctx, 970042, "")
	if err != nil || len(other.Rows) != 0 {
		t.Fatal("cross-account report")
	}
	if validateBattleContributions(BattleWaveReward{ItemRewards: map[string]int64{"wood": 1}, Contributions: operationeffect.Contributions{"x": {Items: map[string]int64{"wood": 2}}}}) == nil {
		t.Fatal("overattribution accepted")
	}
}

func TestFurniturePostgresV2MigrationPreservesHistory(t *testing.T) {
	s, ctx := furniturePostgresStore(t)
	const uid int64 = 970041
	d := s.gameData.CatteryFurniture[0]
	d.ID = "loot_cabinet"
	d.Version = "loot-cabinet-v1"
	d.Rules = []operationeffect.PassiveRule{{Kind: operationeffect.KindExtraLoot, Domain: operationeffect.DomainBattleReward, RateBPS: 2000}}
	raw, _ := json.Marshal(d)
	source, _ := json.Marshal(d.Source())
	from, until := time.Now().Add(-time.Hour), time.Now().Add(time.Hour)
	if _, err := s.db.Exec(ctx, `INSERT INTO player_cattery_furniture_state(user_id) VALUES($1)`, uid); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.Exec(ctx, `INSERT INTO player_cattery_furniture VALUES($1,'loot_cabinet',$2,3600000,$3,$4)`, uid, string(raw), until, from); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.Exec(ctx, `INSERT INTO player_cattery_furniture_periods(user_id,furniture_id,valid_from,valid_until,source) VALUES($1,'loot_cabinet',$2,$3,$4)`, uid, from, until, string(source)); err != nil {
		t.Fatal(err)
	}
	migration, err := os.ReadFile("../../schema/migrations/20260906_cattery_furniture_v2.sql")
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if _, err = s.db.Exec(ctx, string(migration)); err != nil {
			t.Fatal(err)
		}
	}
	var count, revision int64
	if err = s.db.QueryRow(ctx, `SELECT COUNT(*) FROM player_cattery_furniture_periods`).Scan(&count); err != nil || count != 2 {
		t.Fatalf("migration not idempotent: %d %v", count, err)
	}
	if err = s.db.QueryRow(ctx, `SELECT revision FROM player_cattery_furniture_state`).Scan(&revision); err != nil || revision != 1 {
		t.Fatal("revision not idempotent")
	}
	var oldEnd, newStart, newEnd time.Time
	var oldRate, newRate int64
	if err = s.db.QueryRow(ctx, `SELECT valid_until,(source->'rules'->0->>'rateBps')::bigint FROM player_cattery_furniture_periods ORDER BY id LIMIT 1`).Scan(&oldEnd, &oldRate); err != nil {
		t.Fatal(err)
	}
	if err = s.db.QueryRow(ctx, `SELECT valid_from,valid_until,(source->'rules'->0->>'rateBps')::bigint FROM player_cattery_furniture_periods ORDER BY id DESC LIMIT 1`).Scan(&newStart, &newEnd, &newRate); err != nil {
		t.Fatal(err)
	}
	if oldRate != 2000 || newRate != 500 || !oldEnd.Equal(newStart) || newEnd.Sub(until).Abs() > time.Microsecond {
		t.Fatal("history or paid window was rewritten")
	}
	var storedRaw []byte
	var remaining int64
	if err = s.db.QueryRow(ctx, `SELECT definition,remaining_ms FROM player_cattery_furniture`).Scan(&storedRaw, &remaining); err != nil {
		t.Fatal(err)
	}
	var stored catteryfurniture.Definition
	if err = json.Unmarshal(storedRaw, &stored); err != nil {
		t.Fatal(err)
	}
	if stored.Rules[0].RateBPS != 500 || stored.Version != "loot-cabinet-v1-balance-v2" || remaining != 3600000 || stored.RefillCosts["wood"] != d.RefillCosts["wood"] {
		t.Fatal("migration changed paid costs/time")
	}
}

func TestFurniturePostgresUpgradePreservesRunningBoundary(t *testing.T) {
	s, ctx := furniturePostgresStore(t)
	const uid int64 = 970041
	root := &s.gameData.CatteryFurniture[0]
	root.Levels = []catteryfurniture.Level{{Level: 2, UpgradeCosts: map[string]int64{"wood": 20}, RefillCostAdd: map[string]int64{"wood": 3}, Rules: []operationeffect.PassiveRule{{Kind: operationeffect.KindOwnerExperience, Domain: operationeffect.DomainLifeAction, RateBPS: 1600}}}}
	p, err := s.CommandFurniture(ctx, uid, FurnitureCommand{RequestID: "install-0001", FurnitureID: root.ID, Version: root.Version, Action: "install"})
	if err != nil {
		t.Fatal(err)
	}
	p, err = s.CommandFurniture(ctx, uid, FurnitureCommand{RequestID: "enable-00001", FurnitureID: root.ID, Version: root.Version, Revision: p.Revision, Action: "enable"})
	if err != nil {
		t.Fatal(err)
	}
	before := *p.Items[0].EnabledUntil
	cmd := FurnitureCommand{RequestID: "upgrade-live", FurnitureID: root.ID, Version: root.Version, Revision: p.Revision, Action: "upgrade", UpgradeQuote: &p.Items[0].Upgrade.FurnitureQuoteIdentity}
	if _, err = s.db.Exec(ctx, `INSERT INTO player_action_queue(id,user_id,status,actor_type,current_run_started_at,current_run_ends_at) VALUES(1,$1,'running','kitty',now()-interval '1 hour',now()-interval '1 minute')`, uid); err != nil {
		t.Fatal(err)
	}
	if _, err = s.CommandFurniture(ctx, uid, cmd); !errors.Is(err, ErrOperationEffectRecoveryPending) {
		t.Fatalf("upgrade borrowed historical supplies: %v", err)
	}
	if _, err = s.db.Exec(ctx, `UPDATE player_action_queue SET current_run_ends_at=now()+interval '1 hour'`); err != nil {
		t.Fatal(err)
	}
	p, err = s.CommandFurniture(ctx, uid, cmd)
	if err != nil {
		t.Fatal(err)
	}
	if p.Items[0].EnabledUntil == nil || p.Items[0].EnabledUntil.Sub(before).Abs() > time.Millisecond {
		t.Fatal("upgrade refilled time or paused running furniture")
	}
	var raw []byte
	if err = s.db.QueryRow(ctx, `SELECT jsonb_agg(jsonb_build_object('from',valid_from,'until',valid_until,'source',source) ORDER BY id) FROM player_cattery_furniture_periods`).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	var windows []operationeffect.PassiveWindow
	if err = json.Unmarshal(raw, &windows); err != nil {
		t.Fatal(err)
	}
	if len(windows) != 2 || !windows[0].Until.Equal(windows[1].From) || windows[0].Source.Rules[0].RateBPS != 1500 || windows[1].Source.Rules[0].RateBPS != 1600 {
		t.Fatalf("wrong upgrade boundary %s", raw)
	}
}

func TestFurniturePostgresExperienceCapAttribution(t *testing.T) {
	s, ctx := furniturePostgresStore(t)
	const uid int64 = 970041
	s.attributeExpProfile = &attributeExpProfileResolver{profileByAttr: map[string]string{"gather": "test"}, curves: map[string]map[int64]float64{"test": {1: 100, 2: 0}}, capTotalExp: map[string]float64{"gather": 100}}
	if _, err := s.db.Exec(ctx, `INSERT INTO player_attributes(user_id,attr_key,attr_exp) VALUES($1,'gather',100)`, uid); err != nil {
		t.Fatal(err)
	}
	err := s.withTx(ctx, func(tx pgx.Tx) error {
		p := &operationeffect.PassiveSession{PendingExperience: map[string]map[string]float64{"desk": {"gather": 15}}}
		effects, _ := operationeffect.NewSession(operationeffect.NewRegistry(), nil)
		effects.SetPassive(p)
		owner := map[string]float64{"gather": 15}
		if err := s.applyAttributedExperienceTx(ctx, tx, uid, map[string]float64{"gather": 115}, owner, effects); err != nil {
			return err
		}
		if owner["gather"] != 0 || len(p.Contributions) > 0 {
			t.Fatal("capped experience reported as credited")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
