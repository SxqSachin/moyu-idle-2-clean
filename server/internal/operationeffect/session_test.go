package operationeffect_test

import (
	"encoding/json"
	"testing"
	"time"

	"mysecondwebgame/server/internal/operationeffect"
	"mysecondwebgame/server/internal/operationeffect/lifeaction"
)

func TestSessionStableStartCountPreservesBoundariesAndCharges(t *testing.T) {
	at := time.Date(2026, 9, 7, 0, 0, 0, 0, time.UTC)
	for _, offset := range []time.Duration{4 * time.Second, 4*time.Second + time.Nanosecond} {
		grant := testGrant(at, operationeffect.LifetimeDuration, operationeffect.TargetPlayer)
		grant.Definition.Lifetime = operationeffect.LifetimeDefinition{Kind: operationeffect.LifetimeDuration, DurationSeconds: 3600}
		grant.Definition.Stacking.Mode = operationeffect.StackExtendDuration
		until := at.Add(offset)
		grant.ValidUntil = &until
		s, err := operationeffect.NewSession(testRegistry(t), []operationeffect.Grant{grant})
		if err != nil {
			t.Fatal(err)
		}
		want := 2
		if offset > 4*time.Second {
			want = 3
		}
		if got := s.StableStartCount(testOperation("next", at, operationeffect.ActorPlayer, 0), at.Add(time.Hour), 2*time.Second, 100); got != want {
			t.Fatalf("half-open limit=%d want %d", got, want)
		}
		grant.ValidFrom = until
		end := until.Add(time.Hour)
		grant.ValidUntil = &end
		s, err = operationeffect.NewSession(testRegistry(t), []operationeffect.Grant{grant})
		if err != nil {
			t.Fatal(err)
		}
		if got := s.StableStartCount(testOperation("next", at, operationeffect.ActorPlayer, 0), end, 2*time.Second, 100); got != want {
			t.Fatalf("future start limit=%d want %d", got, want)
		}
	}
	grant := testGrant(at, operationeffect.LifetimeCharges, operationeffect.TargetPlayer)
	grant.Definition.Lifetime = operationeffect.LifetimeDefinition{Kind: operationeffect.LifetimeCharges, Charges: 7}
	grant.Definition.Stacking.Mode = operationeffect.StackAddCharges
	grant.InitialCharges = 7
	grant.RemainingCharges = 3
	s, err := operationeffect.NewSession(testRegistry(t), []operationeffect.Grant{grant})
	if err != nil {
		t.Fatal(err)
	}
	op := testOperation("next", at, operationeffect.ActorPlayer, 0)
	if got := s.StableStartCount(op, at.Add(time.Hour), 2*time.Second, 100); got != 3 {
		t.Fatalf("charge limit=%d", got)
	}
	if len(s.ChargeReceipts()) != 0 {
		t.Fatal("planning consumed charges")
	}
	if _, err := s.Commit(s.Prepare(op)); err != nil {
		t.Fatal(err)
	}
	if got := s.StableStartCount(op, at.Add(time.Hour), 2*time.Second, 100); got != 1 {
		t.Fatalf("replayed start batched: %d", got)
	}
	registry := operationeffect.NewRegistry()
	handler, _ := testRegistry(t).Handler(grant.Definition.Kind)
	if err := registry.Register(grant.Definition.Kind, unreviewedStartHandler{handler}); err != nil {
		t.Fatal(err)
	}
	s, err = operationeffect.NewSession(registry, []operationeffect.Grant{grant})
	if err != nil {
		t.Fatal(err)
	}
	if got := s.StableStartCount(op, at.Add(time.Hour), 2*time.Second, 100); got != 1 {
		t.Fatalf("handler without stable-start contract batched: %d", got)
	}
	grant.Target.Kind = operationeffect.TargetPlayerAndKitties
	grant.Definition.AllowedTargets = []operationeffect.TargetKind{operationeffect.TargetPlayerAndKitties}
	s, err = operationeffect.NewSession(testRegistry(t), []operationeffect.Grant{grant})
	if err != nil {
		t.Fatal(err)
	}
	if got := s.StableStartCount(op, at.Add(time.Hour), 2*time.Second, 100); got != 1 {
		t.Fatalf("shared pool batched: %d", got)
	}
	if got := s.StableOrderedStartCount(op, at.Add(time.Hour), 2*time.Second, 100); got != 3 {
		t.Fatalf("ordered shared pool limit=%d, want remaining 3", got)
	}
	if len(s.ChargeReceipts()) != 0 {
		t.Fatal("ordered planning consumed shared charges")
	}
}

type unreviewedStartHandler struct{ operationeffect.RuleHandler }

func TestSessionCountEffectConsumesEveryMatchingStart(t *testing.T) {
	registry := testRegistry(t)
	now := time.Date(2026, time.September, 4, 12, 0, 0, 0, time.UTC)
	grant := testGrant(now, operationeffect.LifetimeCharges, operationeffect.TargetPlayer)
	grant.InitialCharges = 2
	grant.RemainingCharges = 2
	grant.Definition.Lifetime = operationeffect.LifetimeDefinition{Kind: operationeffect.LifetimeCharges, Charges: 2}
	grant.Definition.Stacking = operationeffect.StackingDefinition{Key: "test-extra", Mode: operationeffect.StackAddCharges}

	session, err := operationeffect.NewSession(registry, []operationeffect.Grant{grant})
	if err != nil {
		t.Fatal(err)
	}
	first := testOperation("action:10:1", now, operationeffect.ActorPlayer, 0)
	firstSnapshot, err := session.Commit(session.Prepare(first))
	if err != nil {
		t.Fatal(err)
	}
	if len(firstSnapshot.Rules) != 1 {
		t.Fatalf("expected one matching rule, got %#v", firstSnapshot.Rules)
	}
	// A failed probability roll is still a matching start and therefore still a
	// charge. Committing the same operation key remains idempotent.
	if _, err := session.Commit(session.Prepare(first)); err != nil {
		t.Fatal(err)
	}
	second := testOperation("action:10:2", now.Add(time.Minute), operationeffect.ActorPlayer, 0)
	if _, err := session.Commit(session.Prepare(second)); err != nil {
		t.Fatal(err)
	}
	third := testOperation("action:10:3", now.Add(2*time.Minute), operationeffect.ActorPlayer, 0)
	thirdSnapshot, err := session.Commit(session.Prepare(third))
	if err != nil {
		t.Fatal(err)
	}
	if len(thirdSnapshot.Rules) != 0 {
		t.Fatalf("exhausted grant matched a third start: %#v", thirdSnapshot.Rules)
	}
	if got := len(session.ChargeReceipts()); got != 2 {
		t.Fatalf("expected two charge receipts, got %d", got)
	}
}

func TestSessionDurationUsesHalfOpenStartInterval(t *testing.T) {
	registry := testRegistry(t)
	from := time.Date(2026, time.September, 4, 12, 0, 0, 0, time.UTC)
	grant := testGrant(from, operationeffect.LifetimeDuration, operationeffect.TargetPlayer)
	until := from.Add(time.Hour)
	grant.ValidUntil = &until
	grant.Definition.Lifetime = operationeffect.LifetimeDefinition{Kind: operationeffect.LifetimeDuration, DurationSeconds: 3600}
	grant.Definition.Stacking = operationeffect.StackingDefinition{Key: "test-extra", Mode: operationeffect.StackExtendDuration}

	session, err := operationeffect.NewSession(registry, []operationeffect.Grant{grant})
	if err != nil {
		t.Fatal(err)
	}
	inside, err := session.Commit(session.Prepare(testOperation("inside", until.Add(-time.Nanosecond), operationeffect.ActorPlayer, 0)))
	if err != nil {
		t.Fatal(err)
	}
	atEnd, err := session.Commit(session.Prepare(testOperation("at-end", until, operationeffect.ActorPlayer, 0)))
	if err != nil {
		t.Fatal(err)
	}
	if len(inside.Rules) != 1 || len(atEnd.Rules) != 0 {
		t.Fatalf("expected [validFrom, validUntil) semantics, inside=%d atEnd=%d", len(inside.Rules), len(atEnd.Rules))
	}
}

func TestSessionTargetsPlayerAndKittiesIndependently(t *testing.T) {
	registry := testRegistry(t)
	now := time.Date(2026, time.September, 4, 12, 0, 0, 0, time.UTC)
	kittyGrant := testGrant(now, operationeffect.LifetimeCharges, operationeffect.TargetKitty)
	kittyGrant.Target.ActorID = 7
	kittyGrant.InitialCharges, kittyGrant.RemainingCharges = 1, 1
	kittyGrant.Definition.AllowedTargets = []operationeffect.TargetKind{operationeffect.TargetKitty}
	kittyGrant.Definition.Lifetime = operationeffect.LifetimeDefinition{Kind: operationeffect.LifetimeCharges, Charges: 1}
	kittyGrant.Definition.Stacking = operationeffect.StackingDefinition{Key: "kitty-only", Mode: operationeffect.StackAddCharges}

	session, err := operationeffect.NewSession(registry, []operationeffect.Grant{kittyGrant})
	if err != nil {
		t.Fatal(err)
	}
	player := session.Prepare(testOperation("player", now, operationeffect.ActorPlayer, 0)).Snapshot()
	otherKitty := session.Prepare(testOperation("kitty-8", now, operationeffect.ActorKitty, 8)).Snapshot()
	targetKitty := session.Prepare(testOperation("kitty-7", now, operationeffect.ActorKitty, 7)).Snapshot()
	if len(player.Rules) != 0 || len(otherKitty.Rules) != 0 || len(targetKitty.Rules) != 1 {
		t.Fatalf("target isolation failed: player=%d other=%d target=%d", len(player.Rules), len(otherKitty.Rules), len(targetKitty.Rules))
	}
}

func TestSessionIndependentGrantsEachApplyAndConsume(t *testing.T) {
	registry := testRegistry(t)
	now := time.Date(2026, time.September, 4, 12, 0, 0, 0, time.UTC)
	first := testGrant(now, operationeffect.LifetimeCharges, operationeffect.TargetPlayer)
	first.ID = 1
	first.InitialCharges, first.RemainingCharges = 1, 1
	first.Definition.Lifetime = operationeffect.LifetimeDefinition{Kind: operationeffect.LifetimeCharges, Charges: 1}
	first.Definition.Stacking = operationeffect.StackingDefinition{Key: "independent-extra", Mode: operationeffect.StackIndependent}
	second := first
	second.ID = 2

	session, err := operationeffect.NewSession(registry, []operationeffect.Grant{first, second})
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := session.Commit(session.Prepare(testOperation("independent", now, operationeffect.ActorPlayer, 0)))
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Rules) != 2 {
		t.Fatalf("expected both independent grants, got %#v", snapshot.Rules)
	}
	if got := len(session.ChargeReceipts()); got != 2 {
		t.Fatalf("expected one receipt per independent grant, got %d", got)
	}
}

func TestSessionDistinctDefinitionsDoNotSuppressEachOtherByStackingKey(t *testing.T) {
	registry := testRegistry(t)
	now := time.Date(2026, time.September, 4, 12, 0, 0, 0, time.UTC)
	first := testGrant(now, operationeffect.LifetimeCharges, operationeffect.TargetPlayer)
	first.ID = 1
	first.InitialCharges, first.RemainingCharges = 1, 1
	first.Definition.Lifetime = operationeffect.LifetimeDefinition{Kind: operationeffect.LifetimeCharges, Charges: 1}
	first.Definition.Stacking = operationeffect.StackingDefinition{Key: "shared-authoring-label", Mode: operationeffect.StackAddCharges}
	second := first
	second.ID = 2
	second.Definition.ID = "another-extra-output"

	session, err := operationeffect.NewSession(registry, []operationeffect.Grant{first, second})
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := session.Commit(session.Prepare(testOperation("separate-definitions", now, operationeffect.ActorPlayer, 0)))
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Rules) != 2 {
		t.Fatalf("distinct definitions unexpectedly suppressed each other: %#v", snapshot.Rules)
	}
}

func TestSessionOverlappingTargetsDoNotSuppressEachOther(t *testing.T) {
	registry := testRegistry(t)
	now := time.Date(2026, time.September, 4, 12, 0, 0, 0, time.UTC)
	allKitties := testGrant(now, operationeffect.LifetimeCharges, operationeffect.TargetAllKitties)
	allKitties.ID = 1
	allKitties.InitialCharges, allKitties.RemainingCharges = 1, 1
	allKitties.Definition.AllowedTargets = []operationeffect.TargetKind{
		operationeffect.TargetKitty,
		operationeffect.TargetAllKitties,
	}
	allKitties.Definition.Lifetime = operationeffect.LifetimeDefinition{Kind: operationeffect.LifetimeCharges, Charges: 1}
	allKitties.Definition.Stacking = operationeffect.StackingDefinition{Key: "targeted-extra", Mode: operationeffect.StackAddCharges}

	oneKitty := allKitties
	oneKitty.ID = 2
	oneKitty.Target = operationeffect.Target{Kind: operationeffect.TargetKitty, ActorID: 7}

	session, err := operationeffect.NewSession(registry, []operationeffect.Grant{allKitties, oneKitty})
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := session.Commit(session.Prepare(testOperation("kitty-overlap", now, operationeffect.ActorKitty, 7)))
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Rules) != 2 {
		t.Fatalf("overlapping target grants unexpectedly suppressed each other: %#v", snapshot.Rules)
	}
	if got := len(session.ChargeReceipts()); got != 2 {
		t.Fatalf("expected one receipt per target grant, got %d", got)
	}
}

func TestSessionOnlyCrossActorChargesRequireLaneInterleaving(t *testing.T) {
	registry := testRegistry(t)
	now := time.Date(2026, time.September, 4, 12, 0, 0, 0, time.UTC)
	player := testGrant(now, operationeffect.LifetimeCharges, operationeffect.TargetPlayer)
	player.ID = 1
	player.InitialCharges, player.RemainingCharges = 2, 2
	player.Definition.Lifetime = operationeffect.LifetimeDefinition{Kind: operationeffect.LifetimeCharges, Charges: 2}
	player.Definition.Stacking = operationeffect.StackingDefinition{Key: "player-extra", Mode: operationeffect.StackAddCharges}

	playerSession, err := operationeffect.NewSession(registry, []operationeffect.Grant{player})
	if err != nil {
		t.Fatal(err)
	}
	playerOperation := testOperation("player-batch", now, operationeffect.ActorPlayer, 0)
	if playerSession.HasMatchingCrossActorCharge(playerOperation, now.Add(time.Hour)) {
		t.Fatal("player-only charge unnecessarily disabled single-lane batching")
	}

	shared := player
	shared.ID = 2
	shared.Target = operationeffect.Target{Kind: operationeffect.TargetPlayerAndKitties}
	shared.Definition.AllowedTargets = []operationeffect.TargetKind{operationeffect.TargetPlayerAndKitties}
	shared.Definition.Stacking.Key = "shared-extra"
	sharedSession, err := operationeffect.NewSession(registry, []operationeffect.Grant{shared})
	if err != nil {
		t.Fatal(err)
	}
	if !sharedSession.HasMatchingCrossActorCharge(playerOperation, now.Add(time.Hour)) {
		t.Fatal("shared charge did not require cross-lane ordering")
	}
}

func TestSessionHasMatchingFindsAGrantInsideOfflineWindow(t *testing.T) {
	registry := testRegistry(t)
	startedAt := time.Date(2026, time.September, 4, 12, 0, 0, 0, time.UTC)
	validFrom := startedAt.Add(5 * time.Minute)
	validUntil := validFrom.Add(10 * time.Minute)
	grant := testGrant(validFrom, operationeffect.LifetimeDuration, operationeffect.TargetPlayer)
	grant.ValidUntil = &validUntil
	grant.Definition.Lifetime = operationeffect.LifetimeDefinition{Kind: operationeffect.LifetimeDuration, DurationSeconds: 600}
	grant.Definition.Stacking = operationeffect.StackingDefinition{Key: "offline-window", Mode: operationeffect.StackExtendDuration}
	session, err := operationeffect.NewSession(registry, []operationeffect.Grant{grant})
	if err != nil {
		t.Fatal(err)
	}
	operation := testOperation("future-window", startedAt, operationeffect.ActorPlayer, 0)
	if session.HasMatching(operation, startedAt.Add(4*time.Minute)) {
		t.Fatal("grant matched before validFrom")
	}
	if !session.HasMatching(operation, startedAt.Add(6*time.Minute)) {
		t.Fatal("grant inside offline window was not detected")
	}
}

func TestSessionPrepareDoesNotConsumeUntilCommit(t *testing.T) {
	registry := testRegistry(t)
	now := time.Date(2026, time.September, 4, 12, 0, 0, 0, time.UTC)
	grant := testGrant(now, operationeffect.LifetimeCharges, operationeffect.TargetPlayer)
	grant.InitialCharges, grant.RemainingCharges = 1, 1
	grant.Definition.Lifetime = operationeffect.LifetimeDefinition{Kind: operationeffect.LifetimeCharges, Charges: 1}
	grant.Definition.Stacking = operationeffect.StackingDefinition{Key: "single-extra", Mode: operationeffect.StackAddCharges}

	session, err := operationeffect.NewSession(registry, []operationeffect.Grant{grant})
	if err != nil {
		t.Fatal(err)
	}
	first := session.Prepare(testOperation("first", now, operationeffect.ActorPlayer, 0))
	second := session.Prepare(testOperation("second", now.Add(time.Second), operationeffect.ActorPlayer, 0))
	if len(first.Snapshot().Rules) != 1 || len(second.Snapshot().Rules) != 1 || len(session.ChargeReceipts()) != 0 {
		t.Fatal("preparing an operation changed charge state")
	}
	if _, err := session.Commit(first); err != nil {
		t.Fatal(err)
	}
	if _, err := session.Commit(second); err != operationeffect.ErrPreparedOperationStale {
		t.Fatalf("second prepared operation error = %v, want ErrPreparedOperationStale", err)
	}
}

func TestSessionRejectsPreparedOperationFromAnotherSession(t *testing.T) {
	registry := testRegistry(t)
	now := time.Date(2026, time.September, 4, 12, 0, 0, 0, time.UTC)
	grant := testGrant(now, operationeffect.LifetimeCharges, operationeffect.TargetPlayer)
	grant.InitialCharges, grant.RemainingCharges = 1, 1
	grant.Definition.Lifetime = operationeffect.LifetimeDefinition{Kind: operationeffect.LifetimeCharges, Charges: 1}
	grant.Definition.Stacking = operationeffect.StackingDefinition{Key: "session-bound", Mode: operationeffect.StackAddCharges}

	first, err := operationeffect.NewSession(registry, []operationeffect.Grant{grant})
	if err != nil {
		t.Fatal(err)
	}
	second, err := operationeffect.NewSession(registry, []operationeffect.Grant{grant})
	if err != nil {
		t.Fatal(err)
	}
	prepared := first.Prepare(testOperation("foreign", now, operationeffect.ActorPlayer, 0))
	if _, err := second.Commit(prepared); err != operationeffect.ErrPreparedOperationForeign {
		t.Fatalf("foreign prepared operation error = %v, want ErrPreparedOperationForeign", err)
	}
	if got := len(second.ChargeReceipts()); got != 0 {
		t.Fatalf("foreign prepared operation consumed %d charges", got)
	}
}

func TestSessionRejectsSelectorThatNormalizesToEmpty(t *testing.T) {
	registry := testRegistry(t)
	now := time.Date(2026, time.September, 4, 12, 0, 0, 0, time.UTC)
	grant := testGrant(now, operationeffect.LifetimeCharges, operationeffect.TargetPlayer)
	grant.InitialCharges, grant.RemainingCharges = 1, 1
	grant.Definition.Lifetime = operationeffect.LifetimeDefinition{Kind: operationeffect.LifetimeCharges, Charges: 1}
	grant.Definition.Stacking = operationeffect.StackingDefinition{Key: "invalid-selector", Mode: operationeffect.StackAddCharges}
	grant.Definition.Selector.ActionIDs = []string{"   "}

	if _, err := operationeffect.NewSession(registry, []operationeffect.Grant{grant}); err == nil {
		t.Fatal("expected an empty normalized selector to fail closed")
	}
}

func TestRegistryRejectsUnsupportedIndependentCap(t *testing.T) {
	registry := testRegistry(t)
	now := time.Date(2026, time.September, 4, 12, 0, 0, 0, time.UTC)
	grant := testGrant(now, operationeffect.LifetimeCharges, operationeffect.TargetPlayer)
	grant.Definition.Lifetime = operationeffect.LifetimeDefinition{Kind: operationeffect.LifetimeCharges, Charges: 1}
	grant.Definition.Stacking = operationeffect.StackingDefinition{
		Key:        "independent-extra",
		Mode:       operationeffect.StackIndependent,
		MaxCharges: 10,
	}

	if err := registry.ValidateDefinition(grant.Definition); err == nil {
		t.Fatal("expected independent grant with a shared maxCharges cap to be rejected")
	}
}

func TestRegistryRejectsUnsafeLifetimeBounds(t *testing.T) {
	registry := testRegistry(t)
	now := time.Date(2026, time.September, 4, 12, 0, 0, 0, time.UTC)

	duration := testGrant(now, operationeffect.LifetimeDuration, operationeffect.TargetPlayer).Definition
	duration.Lifetime = operationeffect.LifetimeDefinition{
		Kind:            operationeffect.LifetimeDuration,
		DurationSeconds: operationeffect.MaxDurationSeconds + 1,
	}
	duration.Stacking = operationeffect.StackingDefinition{Key: "unsafe-duration", Mode: operationeffect.StackExtendDuration}
	if err := registry.ValidateDefinition(duration); err == nil {
		t.Fatal("expected a duration that overflows time.Duration to be rejected")
	}

	charges := testGrant(now, operationeffect.LifetimeCharges, operationeffect.TargetPlayer).Definition
	charges.Lifetime = operationeffect.LifetimeDefinition{
		Kind:    operationeffect.LifetimeCharges,
		Charges: operationeffect.MaxChargeCount + 1,
	}
	charges.Stacking = operationeffect.StackingDefinition{Key: "unsafe-charges", Mode: operationeffect.StackAddCharges}
	if err := registry.ValidateDefinition(charges); err == nil {
		t.Fatal("expected charges that cannot round-trip through JSON clients to be rejected")
	}
}

func testRegistry(t *testing.T) *operationeffect.Registry {
	t.Helper()
	registry := operationeffect.NewRegistry()
	if err := lifeaction.Register(registry); err != nil {
		t.Fatal(err)
	}
	return registry
}

func testGrant(now time.Time, lifetime operationeffect.LifetimeKind, target operationeffect.TargetKind) operationeffect.Grant {
	params, _ := json.Marshal(map[string]any{"chance": 0.0, "quantity": 1})
	return operationeffect.Grant{
		ID: 1,
		Definition: operationeffect.Definition{
			ID:             "test-extra-output",
			RulesetVersion: "test-v1",
			Domain:         operationeffect.DomainLifeAction,
			Kind:           lifeaction.KindExtraMainOutputChance,
			NameKey:        "effect.test.name",
			DescKey:        "effect.test.desc",
			AllowedTargets: []operationeffect.TargetKind{target},
			Lifetime:       operationeffect.LifetimeDefinition{Kind: lifetime},
			Stacking:       operationeffect.StackingDefinition{Key: "test-extra"},
			Params:         params,
		},
		Target:    operationeffect.Target{Kind: target},
		ValidFrom: now,
		CreatedAt: now,
	}
}

func testOperation(key string, at time.Time, actor operationeffect.ActorKind, actorID int64) operationeffect.Operation {
	return operationeffect.Operation{
		Key:       key,
		Domain:    operationeffect.DomainLifeAction,
		ActorKind: actor,
		ActorID:   actorID,
		ActionID:  "work_logging",
		MainType:  "logging",
		SubType:   "common",
		StartedAt: at,
		Capabilities: map[string]bool{
			lifeaction.CapabilityMainOutput: true,
		},
	}
}
