package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"mysecondwebgame/server/internal/catteryfurniture"
	"mysecondwebgame/server/internal/operationeffect"
)

func waitFurnitureActivation(t *testing.T, ctx context.Context, at time.Time) {
	t.Helper()
	timer := time.NewTimer(time.Until(at.Add(time.Millisecond)))
	defer timer.Stop()
	select {
	case <-timer.C:
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
}

func TestFurniturePostgresCachedDelayedPermanentAndLostInvalidation(t *testing.T) {
	s, ctx := furniturePostgresStore(t)
	s.furniturePolicy = &furnitureRuntimePolicy{lease: 200 * time.Millisecond, delay: time.Second}
	d := &s.gameData.CatteryFurniture[0]
	d.Service, d.CapacitySeconds, d.RefillSeconds, d.RefillCosts = "permanent", 0, 0, nil
	d.Rules = trainingSource(1).Rules
	d.Levels = []catteryfurniture.Level{{Level: 2, UpgradeCosts: map[string]int64{"wood": 20}, Rules: trainingSource(2).Rules}}
	const uid int64 = 970041
	remote := &Store{db: s.db, gameData: s.gameData, furniturePolicy: s.furniturePolicy}
	before := time.Now().UTC()
	load := func(store *Store, at time.Time) *operationeffect.PassiveSession {
		t.Helper()
		var p *operationeffect.PassiveSession
		err := store.withTx(ctx, func(tx pgx.Tx) error {
			var err error
			p, err = store.loadPassiveSessionTx(ctx, tx, uid, before, at)
			return err
		})
		if err != nil {
			t.Fatal(err)
		}
		return p
	}
	if p := load(remote, before); p != nil {
		t.Fatal("uninstalled negative cache has effects")
	}
	install := FurnitureCommand{RequestID: "delayed-install", FurnitureID: d.ID, Version: d.Version, Action: "install"}
	p, err := s.CommandFurniture(ctx, uid, install)
	if err != nil {
		t.Fatal(err)
	}
	v := p.Items[0]
	if v.Pending == nil || v.Installed || !v.Pending.Installed || p.Inventory["wood"] != 990 {
		t.Fatalf("pending install: %+v", p)
	}
	deadline := v.Pending.EffectiveAt
	if !deadline.After(p.ServerNow) {
		t.Fatal("activation was immediate")
	}
	retry, err := s.CommandFurniture(ctx, uid, install)
	if err != nil || retry.Revision != p.Revision || retry.Inventory["wood"] != 990 {
		t.Fatal("retry charged twice", err)
	}
	if _, err = s.CommandFurniture(ctx, uid, FurnitureCommand{RequestID: "blocked-upgrade", FurnitureID: d.ID, Version: d.Version, Revision: p.Revision, Action: "upgrade"}); !errors.Is(err, ErrFurniturePending) {
		t.Fatal("pending mutation accepted", err)
	}
	if remote.furnitureEffects.entries[uid] == nil {
		t.Fatal("test did not retain the remote negative entry")
	}
	waitFurnitureActivation(t, ctx, deadline)
	// No invalidation is delivered to the second Store. Its fixed lease must
	// have expired before the new source can affect a started operation.
	session := load(remote, time.Now())
	op := operationeffect.Operation{ActorKind: operationeffect.ActorKitty, Domain: operationeffect.DomainActionStart, MainType: "collecting", StartedAt: deadline.Add(-time.Microsecond)}
	if session == nil || session.Snapshot(op).EfficiencyBPS() != 0 {
		t.Fatal("future training leaked into old work")
	}
	op.StartedAt = deadline
	frozen := session.Snapshot(op)
	if frozen.EfficiencyBPS() != 2000 {
		t.Fatal("lost invalidation hid activated training")
	}
	p, err = s.GetFurniture(ctx, uid)
	if err != nil || !p.Items[0].Installed || p.Items[0].Pending != nil || p.Items[0].Upgrade == nil {
		t.Fatal("projection did not promote without a job", err)
	}
	q := p.Items[0].Upgrade
	p, err = s.CommandFurniture(ctx, uid, FurnitureCommand{RequestID: "delayed-upgrade", FurnitureID: d.ID, Version: d.Version, Revision: p.Revision, Action: "upgrade", UpgradeQuote: &q.FurnitureQuoteIdentity})
	if err != nil {
		t.Fatal(err)
	}
	if p.Items[0].Definition.CurrentLevel() != 1 || p.Items[0].Pending.Definition.CurrentLevel() != 2 {
		t.Fatal("upgrade displayed target before activation")
	}
	deadline2 := p.Items[0].Pending.EffectiveAt
	waitFurnitureActivation(t, ctx, deadline2)
	session = load(remote, time.Now())
	op.StartedAt = deadline2.Add(-time.Microsecond)
	if session.Snapshot(op).EfficiencyBPS() != 2000 {
		t.Fatal("upgrade rewrote old interval")
	}
	op.StartedAt = deadline2
	if session.Snapshot(op).EfficiencyBPS() != 4000 || frozen.EfficiencyBPS() != 2000 {
		t.Fatal("upgrade changed frozen work or missed exact boundary")
	}
}

func TestFurniturePostgresDelayedPauseKeepsPaidBoundary(t *testing.T) {
	s, ctx := furniturePostgresStore(t)
	const uid int64 = 970041
	p, err := s.CommandFurniture(ctx, uid, FurnitureCommand{RequestID: "install-paid", FurnitureID: "test_desk", Version: "v1", Action: "install"})
	if err != nil {
		t.Fatal(err)
	}
	p, err = s.CommandFurniture(ctx, uid, FurnitureCommand{RequestID: "enable-paid", FurnitureID: "test_desk", Version: "v1", Revision: p.Revision, Action: "enable"})
	if err != nil {
		t.Fatal(err)
	}
	oldEnd := *p.Items[0].EnabledUntil
	s.furniturePolicy = &furnitureRuntimePolicy{lease: 200 * time.Millisecond, delay: time.Second}
	p, err = s.CommandFurniture(ctx, uid, FurnitureCommand{RequestID: "pause-delayed", FurnitureID: "test_desk", Version: "v1", Revision: p.Revision, Action: "pause"})
	if err != nil {
		t.Fatal(err)
	}
	v := p.Items[0]
	if v.Pending == nil || v.EnabledUntil == nil || !v.EnabledUntil.Equal(oldEnd) || v.Pending.EnabledUntil != nil {
		t.Fatal("pause stopped maintenance immediately")
	}
	if v.Pending.RemainingMS != oldEnd.Sub(v.Pending.EffectiveAt).Milliseconds() {
		t.Fatal("pause refunded delay or lost prepaid time")
	}
	var end time.Time
	if err = s.db.QueryRow(ctx, `SELECT valid_until FROM player_cattery_furniture_periods WHERE user_id=$1 ORDER BY id DESC LIMIT 1`, uid).Scan(&end); err != nil || !end.Equal(v.Pending.EffectiveAt) {
		t.Fatal("pause source deadline differs from UI", err)
	}
	waitFurnitureActivation(t, ctx, v.Pending.EffectiveAt)
	after, err := s.GetFurniture(ctx, uid)
	if err != nil || after.Items[0].Pending != nil || after.Items[0].EnabledUntil != nil || after.Items[0].RemainingMS != v.Pending.RemainingMS {
		t.Fatal("paused time continued to drain", err)
	}
}

func TestFurniturePostgresSummaryDoesNotReadInventoryOrAccounting(t *testing.T) {
	s, ctx := furniturePostgresStore(t)
	if _, err := s.db.Exec(ctx, `INSERT INTO player_cattery_furniture_state(user_id,contribution_totals) VALUES(970041,'"not an object"'); DROP TABLE player_item_instances`); err != nil {
		t.Fatal(err)
	}
	p, err := s.GetFurnitureSummary(ctx, 970041)
	if err != nil || !p.Summary || len(p.Items) != 1 || len(p.Inventory) != 0 || len(p.Contributions) != 0 || p.Items[0].Upgrade != nil {
		t.Fatal("summary read detailed data", err)
	}
}

func TestFurniturePostgresSessionFlushDrainsOnlyCommittedDelta(t *testing.T) {
	s, ctx := furniturePostgresStore(t)
	if _, err := s.db.Exec(ctx, `INSERT INTO player_cattery_furniture_state(user_id) VALUES(970041)`); err != nil {
		t.Fatal(err)
	}
	if err := s.withTx(ctx, func(tx pgx.Tx) error {
		p := &operationeffect.PassiveSession{Contributions: operationeffect.Contributions{"desk": {Experience: map[string]float64{"gather": 1.2345}}}, Remainders: map[string]int64{"gather": 123}, Dirty: true}
		session, _ := operationeffect.NewSession(operationeffect.NewRegistry(), nil)
		session.SetPassive(p)
		for i := 0; i < 2; i++ {
			if _, err := s.flushOperationEffectSessionTx(ctx, tx, 970041, session, time.Now()); err != nil {
				return err
			}
		}
		if p.Dirty || len(p.Contributions) != 0 || p.Remainders["gather"] != 123 {
			return errors.New("flush lost carry or retained a paid delta")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	report, err := s.FurnitureReport(ctx, 970041, "")
	if err != nil || report.Rows["desk"].Experience["gather"] != 1.2345 {
		t.Fatal("second flush duplicated attribution", err)
	}
}

func TestFurniturePostgresDelayedActiveUpgradePricesFutureRemainder(t *testing.T) {
	s, ctx := furniturePostgresStore(t)
	const uid int64 = 970041
	d := &s.gameData.CatteryFurniture[0]
	d.Levels = []catteryfurniture.Level{{Level: 2, UpgradeCosts: map[string]int64{"wood": 20}, RefillCostAdd: map[string]int64{"wood": 3}, Rules: []operationeffect.PassiveRule{{Kind: operationeffect.KindOwnerExperience, Domain: operationeffect.DomainLifeAction, RateBPS: 1600}}}}
	p, err := s.CommandFurniture(ctx, uid, FurnitureCommand{RequestID: "upgrade-install", FurnitureID: d.ID, Version: d.Version, Action: "install"})
	if err != nil {
		t.Fatal(err)
	}
	p, err = s.CommandFurniture(ctx, uid, FurnitureCommand{RequestID: "upgrade-enable", FurnitureID: d.ID, Version: d.Version, Revision: p.Revision, Action: "enable"})
	if err != nil {
		t.Fatal(err)
	}
	oldEnd := *p.Items[0].EnabledUntil
	s.furniturePolicy = nil
	p, err = s.GetFurniture(ctx, uid)
	if err != nil {
		t.Fatal(err)
	}
	q := p.Items[0].Upgrade
	if q.RemainingMS != oldEnd.Sub(p.ServerNow.Add(time.Minute)).Milliseconds() {
		t.Fatal("quote charged maintenance before activation")
	}
	stock := p.Inventory["wood"]
	p, err = s.CommandFurniture(ctx, uid, FurnitureCommand{RequestID: "active-upgrade", FurnitureID: d.ID, Version: d.Version, Revision: p.Revision, Action: "upgrade", UpgradeQuote: &q.FurnitureQuoteIdentity})
	if err != nil {
		t.Fatal(err)
	}
	v := p.Items[0]
	if v.Pending == nil || v.Definition.CurrentLevel() != 1 || v.Pending.Definition.CurrentLevel() != 2 || stock-p.Inventory["wood"] > q.TotalCosts["wood"] {
		t.Fatal("upgrade exceeded consent or applied early")
	}
	if v.Pending.EnabledUntil == nil || oldEnd.Sub(*v.Pending.EnabledUntil) < 0 || oldEnd.Sub(*v.Pending.EnabledUntil) >= time.Millisecond {
		t.Fatal("upgrade changed prepaid expiry")
	}
	if err = s.withTx(ctx, func(tx pgx.Tx) error {
		p, err := s.loadPassiveSessionTx(ctx, tx, uid, v.Pending.EffectiveAt.Add(-time.Second), v.Pending.EffectiveAt)
		if err != nil {
			return err
		}
		op := operationeffect.Operation{ActorKind: operationeffect.ActorKitty, Domain: operationeffect.DomainLifeAction, StartedAt: v.Pending.EffectiveAt.Add(-time.Microsecond)}
		old, _ := p.Snapshot(op).Best(operationeffect.KindOwnerExperience)
		op.StartedAt = v.Pending.EffectiveAt
		next, _ := p.Snapshot(op).Best(operationeffect.KindOwnerExperience)
		if old != 1500 || next != 1600 {
			return errors.New("active upgrade corrupted the rate boundary")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestFurniturePostgresColdReaderWaitsForCommittedSources(t *testing.T) {
	dsn := os.Getenv("OPERATION_EFFECT_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set OPERATION_EFFECT_TEST_DATABASE_URL for isolated-schema concurrency proof")
	}
	cfg, err := pgx.ParseConfig(dsn)
	if err != nil {
		t.Fatal(err)
	}
	if h := cfg.Host; h != "localhost" && h != "127.0.0.1" && h != "::1" {
		t.Fatal("loopback database required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	setup, err := pgx.ConnectConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer setup.Close(context.Background())
	schema := pgx.Identifier{fmt.Sprintf("furniture_source_test_%d", time.Now().UnixNano())}.Sanitize()
	if _, err = setup.Exec(ctx, `CREATE SCHEMA `+schema+`; SET search_path=`+schema+`; CREATE TABLE player_cattery_furniture_periods(id bigserial,user_id bigint,valid_from timestamptz,valid_until timestamptz,source jsonb)`); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if _, err := setup.Exec(context.Background(), `DROP SCHEMA `+schema+` CASCADE`); err != nil {
			t.Error(err)
		}
	}()
	cfg.RuntimeParams["search_path"] = schema
	reader, err := pgx.ConnectConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close(context.Background())
	writer, err := setup.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer writer.Rollback(context.Background())
	if err = lockFurnitureSourcesTx(ctx, writer, []int64{41}, false); err != nil {
		t.Fatal(err)
	}
	source := trainingSource(1)
	raw, _ := json.Marshal(source)
	at := time.Now()
	if _, err = writer.Exec(ctx, `INSERT INTO player_cattery_furniture_periods(user_id,valid_from,source) VALUES(41,$1,$2)`, at, string(raw)); err != nil {
		t.Fatal(err)
	}
	readTx, err := reader.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer readTx.Rollback(context.Background())
	started, finished := make(chan struct{}), make(chan error, 1)
	go func() {
		close(started)
		rows, err := loadFurnitureWindowsTx(ctx, readTx, []int64{41}, at)
		if err == nil && len(rows[41].windows) != 1 {
			err = errors.New("reader cached pre-commit source absence")
		}
		finished <- err
	}()
	<-started
	select {
	case err := <-finished:
		t.Fatal("cold read passed an uncommitted writer", err)
	case <-time.After(50 * time.Millisecond):
	}
	if err = writer.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if err = <-finished; err != nil {
		t.Fatal(err)
	}
}
