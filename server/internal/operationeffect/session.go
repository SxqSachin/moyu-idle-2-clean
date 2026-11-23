package operationeffect

import (
	"errors"
	"sort"
	"strings"
	"time"
)

var (
	ErrPreparedOperationStale   = errors.New("operation effect prepared operation is stale")
	ErrPreparedOperationForeign = errors.New("operation effect prepared operation belongs to another session")
)

type preparedRule struct {
	grantID       int64
	consumeCharge bool
	rule          RuleSnapshot
}

type stackIdentity struct {
	definitionID   string
	rulesetVersion string
	stackingKey    string
	targetKind     TargetKind
	targetActorID  int64
	grantID        int64
}

type PreparedOperation struct {
	operation Operation
	rules     []preparedRule
	session   *Session
}

func (p PreparedOperation) Snapshot() Snapshot {
	snapshot := EmptySnapshot(p.operation)
	for _, prepared := range p.rules {
		snapshot.Rules = append(snapshot.Rules, prepared.rule)
	}
	return snapshot
}

type Session struct {
	passive             *PassiveSession
	registry            *Registry
	grants              []*Grant
	receipts            []ChargeReceipt
	committedOperations map[string]Snapshot
}

func (s *Session) HasGrants() bool { return s != nil && len(s.grants) > 0 }

func NewSession(registry *Registry, grants []Grant) (*Session, error) {
	if registry == nil {
		registry = NewRegistry()
	}
	owned := make([]*Grant, 0, len(grants))
	for idx := range grants {
		grant := grants[idx]
		grant.Definition = normalizeDefinition(grant.Definition)
		if err := registry.ValidateDefinition(grant.Definition); err != nil {
			return nil, err
		}
		if err := ValidateTarget(grant.Definition, grant.Target); err != nil {
			return nil, err
		}
		if grant.ID <= 0 || grant.ValidFrom.IsZero() {
			return nil, errors.New("operation effect grant identity and validFrom are required")
		}
		if grant.Definition.Lifetime.Kind == LifetimeDuration {
			if grant.ValidUntil == nil || !grant.ValidUntil.After(grant.ValidFrom) {
				return nil, errors.New("operation effect duration grant has an invalid interval")
			}
		} else if grant.InitialCharges <= 0 || grant.InitialCharges > MaxChargeCount ||
			grant.RemainingCharges < 0 || grant.RemainingCharges > MaxChargeCount || grant.InitialCharges < grant.RemainingCharges {
			return nil, errors.New("operation effect charge grant has invalid charges")
		}
		owned = append(owned, &grant)
	}
	sort.SliceStable(owned, func(i, j int) bool {
		left, right := owned[i], owned[j]
		if left.Definition.Priority != right.Definition.Priority {
			return left.Definition.Priority > right.Definition.Priority
		}
		if left.ValidUntil != nil && right.ValidUntil != nil && !left.ValidUntil.Equal(*right.ValidUntil) {
			return left.ValidUntil.Before(*right.ValidUntil)
		}
		if left.ValidUntil != nil && right.ValidUntil == nil {
			return true
		}
		if left.ValidUntil == nil && right.ValidUntil != nil {
			return false
		}
		return left.ID < right.ID
	})
	return &Session{
		registry:            registry,
		grants:              owned,
		committedOperations: make(map[string]Snapshot),
	}, nil
}

func NewEmptySession() *Session {
	session, _ := NewSession(NewRegistry(), nil)
	return session
}

func (s *Session) Prepare(operation Operation) PreparedOperation {
	operation.Key = strings.TrimSpace(operation.Key)
	operation.ActionID = strings.TrimSpace(operation.ActionID)
	operation.MainType = strings.TrimSpace(operation.MainType)
	operation.SubType = strings.TrimSpace(operation.SubType)
	prepared := PreparedOperation{operation: operation, rules: []preparedRule{}, session: s}
	if s == nil || operation.Key == "" || operation.StartedAt.IsZero() || operation.Domain == DomainNone {
		return prepared
	}
	if committed, exists := s.committedOperations[operation.Key]; exists {
		for _, rule := range committed.Rules {
			prepared.rules = append(prepared.rules, preparedRule{grantID: rule.GrantID, rule: rule})
		}
		return prepared
	}
	seenStacks := make(map[stackIdentity]struct{})
	for _, grant := range s.grants {
		if grant == nil || !grantMatchesOperation(grant, operation, s.registry) {
			continue
		}
		// Stacking merges duplicate grants of one version; it must not make two
		// separately-authored effect definitions silently suppress each other.
		stackKey := stackIdentity{
			definitionID:   grant.Definition.ID,
			rulesetVersion: grant.Definition.RulesetVersion,
			stackingKey:    strings.TrimSpace(grant.Definition.Stacking.Key),
			targetKind:     grant.Target.Kind,
			targetActorID:  grant.Target.ActorID,
		}
		if grant.Definition.Stacking.Mode == StackIndependent {
			stackKey.grantID = grant.ID
		}
		if _, exists := seenStacks[stackKey]; exists {
			continue
		}
		seenStacks[stackKey] = struct{}{}
		prepared.rules = append(prepared.rules, preparedRule{
			grantID:       grant.ID,
			consumeCharge: grant.Definition.Lifetime.Kind == LifetimeCharges,
			rule: RuleSnapshot{
				GrantID:        grant.ID,
				DefinitionID:   grant.Definition.ID,
				RulesetVersion: grant.Definition.RulesetVersion,
				Domain:         grant.Definition.Domain,
				Kind:           grant.Definition.Kind,
				Target:         grant.Target,
				Params:         append([]byte(nil), grant.Definition.Params...),
			},
		})
	}
	return prepared
}

func (s *Session) Commit(prepared PreparedOperation) (Snapshot, error) {
	snapshot := prepared.Snapshot()
	if s == nil || snapshot.OperationKey == "" {
		return snapshot, nil
	}
	if prepared.session != s {
		return EmptySnapshot(prepared.operation), ErrPreparedOperationForeign
	}
	if len(prepared.rules) == 0 {
		return snapshot, nil
	}
	if committed, exists := s.committedOperations[snapshot.OperationKey]; exists {
		return committed, nil
	}
	byID := make(map[int64]*Grant, len(s.grants))
	for _, grant := range s.grants {
		if grant != nil {
			byID[grant.ID] = grant
		}
	}
	for _, rule := range prepared.rules {
		if !rule.consumeCharge {
			continue
		}
		grant := byID[rule.grantID]
		if grant == nil || grant.RemainingCharges <= 0 {
			return EmptySnapshot(prepared.operation), ErrPreparedOperationStale
		}
	}
	for _, rule := range prepared.rules {
		if !rule.consumeCharge {
			continue
		}
		grant := byID[rule.grantID]
		grant.RemainingCharges--
		s.receipts = append(s.receipts, ChargeReceipt{
			GrantID:      grant.ID,
			OperationKey: snapshot.OperationKey,
			StartedAt:    snapshot.StartedAt,
		})
	}
	s.committedOperations[snapshot.OperationKey] = snapshot
	return snapshot, nil
}

// HasMatchingCrossActorCharge reports whether batching this lane could skip the
// global ordering required by a charge pool shared across multiple actors.
// Player-only and one-kitty grants can still batch safely inside their one lane.
func (s *Session) HasMatchingCrossActorCharge(operation Operation, through time.Time) bool {
	if s == nil || operation.Domain == DomainNone {
		return false
	}
	if through.IsZero() {
		through = operation.StartedAt
	}
	for _, grant := range s.grants {
		if grant == nil || grant.Definition.Lifetime.Kind != LifetimeCharges || grant.RemainingCharges <= 0 ||
			(grant.Target.Kind != TargetAllKitties && grant.Target.Kind != TargetPlayerAndKitties) {
			continue
		}
		if grantCouldMatchThrough(grant, operation, through, s.registry) {
			return true
		}
	}
	return false
}

// HasMatching reports whether at least one grant can match this operation at a
// start instant no later than through. It lets an executor retain batching when
// no effect is relevant, while switching to exact per-start processing across
// a duration boundary or a finite charge pool.
func (s *Session) HasMatching(operation Operation, through time.Time) bool {
	if s == nil || operation.Domain == DomainNone {
		return false
	}
	if through.IsZero() {
		through = operation.StartedAt
	}
	for _, grant := range s.grants {
		if grantCouldMatchThrough(grant, operation, through, s.registry) {
			return true
		}
	}
	return false
}

// StableStartCount bounds a series of equally spaced starts before selection
// can change. It is read-only: real starts still Commit their own charge receipt.
func (s *Session) StableStartCount(operation Operation, through time.Time, interval time.Duration, limit int) int {
	return s.stableStartCount(operation, through, interval, limit, false)
}

// StableOrderedStartCount additionally permits shared charge pools. The
// caller MUST bound this lane before the next other-actor completion and keep
// the user lock through reservation and commit; this is not an unordered batch.
func (s *Session) StableOrderedStartCount(operation Operation, through time.Time, interval time.Duration, limit int) int {
	return s.stableStartCount(operation, through, interval, limit, true)
}

func (s *Session) stableStartCount(operation Operation, through time.Time, interval time.Duration, limit int, ordered bool) int {
	if s == nil || limit <= 1 {
		return limit
	}
	if interval <= 0 || operation.StartedAt.IsZero() {
		return 1
	}
	if _, replay := s.committedOperations[operation.Key]; replay {
		return 1
	}
	if !ordered && s.HasMatchingCrossActorCharge(operation, through) {
		return 1
	}
	before := func(boundary time.Time) {
		if boundary.After(operation.StartedAt) {
			count := int((boundary.Sub(operation.StartedAt)-1)/interval) + 1
			limit = min(limit, count)
		}
	}
	for _, grant := range s.grants {
		if grant != nil && grant.Definition.Domain == operation.Domain &&
			targetMatches(grant.Target, operation) && selectorMatches(grant.Definition.Selector, operation) {
			handler, _ := s.registry.Handler(grant.Definition.Kind)
			stable, ok := handler.(StableStartHandler)
			if !ok || !stable.StableAcrossStarts() {
				return 1
			}
		}
		if !grantCouldMatchThrough(grant, operation, through, s.registry) {
			continue
		}
		before(grant.ValidFrom)
		if grant.ValidUntil != nil {
			before(*grant.ValidUntil)
		}
	}
	prepared := s.Prepare(operation)
	for _, rule := range prepared.rules {
		if !rule.consumeCharge {
			continue
		}
		for _, grant := range s.grants {
			if grant.ID == rule.grantID {
				limit = min(limit, int(grant.RemainingCharges))
				break
			}
		}
	}
	return limit
}

func grantCouldMatchThrough(grant *Grant, operation Operation, through time.Time, registry *Registry) bool {
	if grant == nil {
		return false
	}
	probe := operation
	if probe.StartedAt.IsZero() || probe.StartedAt.Before(grant.ValidFrom) {
		probe.StartedAt = grant.ValidFrom
	}
	if probe.StartedAt.After(through) {
		return false
	}
	return grantMatchesOperation(grant, probe, registry)
}

func (s *Session) ChargeReceipts() []ChargeReceipt {
	if s == nil || len(s.receipts) == 0 {
		return nil
	}
	out := make([]ChargeReceipt, len(s.receipts))
	copy(out, s.receipts)
	return out
}

func grantMatchesOperation(grant *Grant, operation Operation, registry *Registry) bool {
	if grant == nil || operation.Domain != grant.Definition.Domain {
		return false
	}
	if operation.StartedAt.Before(grant.ValidFrom) {
		return false
	}
	if grant.Definition.Lifetime.Kind == LifetimeDuration {
		if grant.ValidUntil == nil || !operation.StartedAt.Before(*grant.ValidUntil) {
			return false
		}
	} else if grant.RemainingCharges <= 0 {
		return false
	}
	if !targetMatches(grant.Target, operation) || !selectorMatches(grant.Definition.Selector, operation) {
		return false
	}
	handler, ok := registry.Handler(grant.Definition.Kind)
	return ok && handler.Applies(operation, grant.Definition.Params)
}

func targetMatches(target Target, operation Operation) bool {
	switch target.Kind {
	case TargetPlayer:
		return operation.ActorKind == ActorPlayer
	case TargetKitty:
		return operation.ActorKind == ActorKitty && operation.ActorID > 0 && operation.ActorID == target.ActorID
	case TargetAllKitties:
		return operation.ActorKind == ActorKitty && operation.ActorID > 0
	case TargetPlayerAndKitties:
		return operation.ActorKind == ActorPlayer || (operation.ActorKind == ActorKitty && operation.ActorID > 0)
	default:
		return false
	}
}

func selectorMatches(selector Selector, operation Operation) bool {
	return valueAllowed(selector.ActionIDs, operation.ActionID) &&
		valueAllowed(selector.MainTypes, operation.MainType) &&
		valueAllowed(selector.SubTypes, operation.SubType)
}

func valueAllowed(allowed []string, actual string) bool {
	if len(allowed) == 0 {
		return true
	}
	actual = strings.TrimSpace(actual)
	for _, candidate := range allowed {
		if strings.TrimSpace(candidate) == actual {
			return true
		}
	}
	return false
}
