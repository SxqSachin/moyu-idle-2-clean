package operationeffect

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
)

const (
	SnapshotSchemaVersion = 1
	// MaxDurationSeconds keeps conversion to time.Duration overflow-free.
	MaxDurationSeconds int64 = 9_223_372_036
	// MaxChargeCount remains exactly representable by JSON clients.
	MaxChargeCount int64 = 9_007_199_254_740_991
)

type Domain string

const (
	DomainNone        Domain = "none"
	DomainLifeAction  Domain = "life_action"
	DomainEnhancement Domain = "enhancement"
	DomainAlchemy     Domain = "alchemy"
	DomainFishingCast Domain = "fishing_cast"
	DomainBuilding    Domain = "building"
)

type ActorKind string

const (
	ActorPlayer ActorKind = "player"
	ActorKitty  ActorKind = "kitty"
)

type TargetKind string

const (
	TargetPlayer           TargetKind = "player"
	TargetKitty            TargetKind = "kitty"
	TargetAllKitties       TargetKind = "all_kitties"
	TargetPlayerAndKitties TargetKind = "player_and_kitties"
)

type LifetimeKind string

const (
	LifetimeDuration LifetimeKind = "duration"
	LifetimeCharges  LifetimeKind = "charges"
)

type StackMode string

const (
	StackExtendDuration StackMode = "extend_duration"
	StackAddCharges     StackMode = "add_charges"
	StackIndependent    StackMode = "independent"
)

type LifetimeDefinition struct {
	Kind            LifetimeKind `json:"kind"`
	DurationSeconds int64        `json:"durationSeconds,omitempty"`
	Charges         int64        `json:"charges,omitempty"`
}

type StackingDefinition struct {
	Exclusive          bool      `json:"exclusive,omitempty"`
	Key                string    `json:"key"`
	Mode               StackMode `json:"mode"`
	MaxDurationSeconds int64     `json:"maxDurationSeconds,omitempty"`
	MaxCharges         int64     `json:"maxCharges,omitempty"`
}

type Selector struct {
	ActionIDs []string `json:"actionIds,omitempty"`
	MainTypes []string `json:"mainTypes,omitempty"`
	SubTypes  []string `json:"subTypes,omitempty"`
}

type Definition struct {
	ID             string             `json:"id"`
	RulesetVersion string             `json:"rulesetVersion"`
	Domain         Domain             `json:"domain"`
	Kind           string             `json:"kind"`
	NameKey        string             `json:"nameKey"`
	DescKey        string             `json:"descKey"`
	IconID         string             `json:"iconId,omitempty"`
	Priority       int                `json:"priority,omitempty"`
	AllowedTargets []TargetKind       `json:"allowedTargets"`
	Lifetime       LifetimeDefinition `json:"lifetime"`
	Stacking       StackingDefinition `json:"stacking"`
	Selector       Selector           `json:"selector,omitempty"`
	Params         json.RawMessage    `json:"params"`
}

type Target struct {
	Kind    TargetKind `json:"kind"`
	ActorID int64      `json:"actorId,omitempty"`
}

type Grant struct {
	ID               int64      `json:"id"`
	Definition       Definition `json:"definition"`
	Target           Target     `json:"target"`
	SourceItemID     string     `json:"sourceItemId,omitempty"`
	ValidFrom        time.Time  `json:"validFrom"`
	ValidUntil       *time.Time `json:"validUntil,omitempty"`
	InitialCharges   int64      `json:"initialCharges,omitempty"`
	RemainingCharges int64      `json:"remainingCharges,omitempty"`
	CreatedAt        time.Time  `json:"createdAt"`
}

type Operation struct {
	Key          string          `json:"key"`
	Domain       Domain          `json:"domain"`
	ActorKind    ActorKind       `json:"actorKind"`
	ActorID      int64           `json:"actorId,omitempty"`
	ActionID     string          `json:"actionId,omitempty"`
	MainType     string          `json:"mainType,omitempty"`
	SubType      string          `json:"subType,omitempty"`
	StartedAt    time.Time       `json:"startedAt"`
	Capabilities map[string]bool `json:"capabilities,omitempty"`
}

type RuleSnapshot struct {
	GrantID        int64           `json:"grantId"`
	DefinitionID   string          `json:"definitionId"`
	RulesetVersion string          `json:"rulesetVersion"`
	Domain         Domain          `json:"domain"`
	Kind           string          `json:"kind"`
	Target         Target          `json:"target"`
	Params         json.RawMessage `json:"params"`
}

type Snapshot struct {
	SchemaVersion int            `json:"schemaVersion"`
	OperationKey  string         `json:"operationKey"`
	StartedAt     time.Time      `json:"startedAt"`
	Rules         []RuleSnapshot `json:"rules"`
}

func EmptySnapshot(operation Operation) Snapshot {
	return Snapshot{
		SchemaVersion: SnapshotSchemaVersion,
		OperationKey:  strings.TrimSpace(operation.Key),
		StartedAt:     operation.StartedAt,
		Rules:         []RuleSnapshot{},
	}
}

type ChargeReceipt struct {
	GrantID      int64     `json:"grantId"`
	OperationKey string    `json:"operationKey"`
	StartedAt    time.Time `json:"startedAt"`
}

type RuleHandler interface {
	Domain() Domain
	ValidateParams(json.RawMessage) error
	Applies(Operation, json.RawMessage) bool
}

// StableStartHandler opts into lifecycle-bounded batching: Applies must not
// depend on the operation key or start time when its other fields are unchanged.
type StableStartHandler interface {
	StableAcrossStarts() bool
}

// DefinitionConstraintHandler lets a business domain reject configurations
// that are structurally valid but could never apply to that handler.
type DefinitionConstraintHandler interface {
	ValidateDefinitionConstraints(Definition) error
}

type Registry struct {
	handlers map[string]RuleHandler
}

func NewRegistry() *Registry {
	return &Registry{handlers: make(map[string]RuleHandler)}
}

func (r *Registry) Register(kind string, handler RuleHandler) error {
	kind = strings.TrimSpace(kind)
	if kind == "" || handler == nil {
		return errors.New("operation effect handler kind is required")
	}
	if _, exists := r.handlers[kind]; exists {
		return fmt.Errorf("operation effect handler already registered: %s", kind)
	}
	r.handlers[kind] = handler
	return nil
}

func (r *Registry) Handler(kind string) (RuleHandler, bool) {
	if r == nil {
		return nil, false
	}
	handler, ok := r.handlers[strings.TrimSpace(kind)]
	return handler, ok
}

func (r *Registry) ValidateDefinition(def Definition) error {
	if strings.TrimSpace(def.ID) == "" {
		return errors.New("operation effect definition id is required")
	}
	if strings.TrimSpace(def.RulesetVersion) == "" {
		return fmt.Errorf("operation effect rulesetVersion is required: %s", def.ID)
	}
	if def.Domain == DomainNone || !validDomain(def.Domain) {
		return fmt.Errorf("operation effect domain is required: %s", def.ID)
	}
	handler, ok := r.Handler(def.Kind)
	if !ok {
		return fmt.Errorf("operation effect kind is not registered: %s.%s", def.ID, def.Kind)
	}
	if handler.Domain() != def.Domain {
		return fmt.Errorf("operation effect domain mismatch: %s.%s", def.ID, def.Kind)
	}
	if strings.TrimSpace(def.NameKey) == "" || strings.TrimSpace(def.DescKey) == "" {
		return fmt.Errorf("operation effect display keys are required: %s", def.ID)
	}
	if def.Priority < -1<<31 || def.Priority > 1<<31-1 {
		return fmt.Errorf("operation effect priority is outside int32 range: %s", def.ID)
	}
	if len(def.AllowedTargets) == 0 {
		return fmt.Errorf("operation effect allowedTargets is required: %s", def.ID)
	}
	seenTargets := make(map[TargetKind]struct{}, len(def.AllowedTargets))
	for _, target := range def.AllowedTargets {
		if !validTargetKind(target) {
			return fmt.Errorf("operation effect target is invalid: %s.%s", def.ID, target)
		}
		if _, exists := seenTargets[target]; exists {
			return fmt.Errorf("operation effect target is duplicated: %s.%s", def.ID, target)
		}
		seenTargets[target] = struct{}{}
	}
	switch def.Lifetime.Kind {
	case LifetimeDuration:
		if def.Lifetime.DurationSeconds <= 0 || def.Lifetime.DurationSeconds > MaxDurationSeconds {
			return fmt.Errorf("operation effect duration must be positive: %s", def.ID)
		}
		if def.Lifetime.Charges != 0 {
			return fmt.Errorf("operation effect duration cannot define charges: %s", def.ID)
		}
		if def.Stacking.Mode != StackExtendDuration && def.Stacking.Mode != StackIndependent {
			return fmt.Errorf("operation effect duration stacking mode is invalid: %s", def.ID)
		}
		if def.Stacking.MaxDurationSeconds < 0 || def.Stacking.MaxDurationSeconds > MaxDurationSeconds ||
			(def.Stacking.MaxDurationSeconds > 0 && def.Stacking.MaxDurationSeconds < def.Lifetime.DurationSeconds) {
			return fmt.Errorf("operation effect max duration is below one serving: %s", def.ID)
		}
		if def.Stacking.MaxCharges != 0 {
			return fmt.Errorf("operation effect duration cannot define max charges: %s", def.ID)
		}
		if def.Stacking.Mode == StackIndependent && def.Stacking.MaxDurationSeconds != 0 {
			return fmt.Errorf("operation effect independent duration cannot define a shared cap: %s", def.ID)
		}
	case LifetimeCharges:
		if def.Lifetime.Charges <= 0 || def.Lifetime.Charges > MaxChargeCount {
			return fmt.Errorf("operation effect charges must be positive: %s", def.ID)
		}
		if def.Lifetime.DurationSeconds != 0 {
			return fmt.Errorf("operation effect charges cannot define duration: %s", def.ID)
		}
		if def.Stacking.Mode != StackAddCharges && def.Stacking.Mode != StackIndependent {
			return fmt.Errorf("operation effect charge stacking mode is invalid: %s", def.ID)
		}
		if def.Stacking.MaxCharges < 0 || def.Stacking.MaxCharges > MaxChargeCount ||
			(def.Stacking.MaxCharges > 0 && def.Stacking.MaxCharges < def.Lifetime.Charges) {
			return fmt.Errorf("operation effect max charges is below one serving: %s", def.ID)
		}
		if def.Stacking.MaxDurationSeconds != 0 {
			return fmt.Errorf("operation effect charges cannot define max duration: %s", def.ID)
		}
		if def.Stacking.Mode == StackIndependent && def.Stacking.MaxCharges != 0 {
			return fmt.Errorf("operation effect independent charges cannot define a shared cap: %s", def.ID)
		}
	default:
		return fmt.Errorf("operation effect lifetime kind is invalid: %s", def.ID)
	}
	if strings.TrimSpace(def.Stacking.Key) == "" {
		return fmt.Errorf("operation effect stacking key is required: %s", def.ID)
	}
	if len(def.Params) == 0 || string(def.Params) == "null" {
		def.Params = json.RawMessage(`{}`)
	}
	if err := handler.ValidateParams(def.Params); err != nil {
		return fmt.Errorf("operation effect params are invalid: %s: %w", def.ID, err)
	}
	if constraints, ok := handler.(DefinitionConstraintHandler); ok {
		if err := constraints.ValidateDefinitionConstraints(def); err != nil {
			return fmt.Errorf("operation effect definition is incompatible: %s: %w", def.ID, err)
		}
	}
	return validateSelector(def.ID, def.Selector)
}

func validDomain(domain Domain) bool {
	value := string(domain)
	if len(value) == 0 || len(value) > 64 || value[0] < 'a' || value[0] > 'z' {
		return false
	}
	for index := 1; index < len(value); index++ {
		char := value[index]
		if (char < 'a' || char > 'z') && (char < '0' || char > '9') && char != '_' {
			return false
		}
	}
	return true
}

func validateSelector(id string, selector Selector) error {
	for label, values := range map[string][]string{
		"actionIds": selector.ActionIDs,
		"mainTypes": selector.MainTypes,
		"subTypes":  selector.SubTypes,
	} {
		seen := make(map[string]struct{}, len(values))
		for _, raw := range values {
			value := strings.TrimSpace(raw)
			if value == "" {
				return fmt.Errorf("operation effect selector contains an empty %s value: %s", label, id)
			}
			if _, exists := seen[value]; exists {
				return fmt.Errorf("operation effect selector contains duplicate %s value: %s.%s", label, id, value)
			}
			seen[value] = struct{}{}
		}
	}
	return nil
}

func validTargetKind(kind TargetKind) bool {
	switch kind {
	case TargetPlayer, TargetKitty, TargetAllKitties, TargetPlayerAndKitties:
		return true
	default:
		return false
	}
}

func ValidateTarget(def Definition, target Target) error {
	if !validTargetKind(target.Kind) {
		return errors.New("invalid operation effect target")
	}
	allowed := false
	for _, candidate := range def.AllowedTargets {
		if candidate == target.Kind {
			allowed = true
			break
		}
	}
	if !allowed {
		return errors.New("operation effect target is not allowed")
	}
	if target.Kind == TargetKitty {
		if target.ActorID <= 0 {
			return errors.New("operation effect kitty target is required")
		}
	} else if target.ActorID != 0 {
		return errors.New("operation effect actor id is only valid for a kitty target")
	}
	return nil
}

func normalizeDefinition(def Definition) Definition {
	def.ID = strings.TrimSpace(def.ID)
	def.RulesetVersion = strings.TrimSpace(def.RulesetVersion)
	def.Kind = strings.TrimSpace(def.Kind)
	def.NameKey = strings.TrimSpace(def.NameKey)
	def.DescKey = strings.TrimSpace(def.DescKey)
	def.IconID = strings.TrimSpace(def.IconID)
	def.Stacking.Key = strings.TrimSpace(def.Stacking.Key)
	def.Selector.ActionIDs = normalizeStrings(def.Selector.ActionIDs)
	def.Selector.MainTypes = normalizeStrings(def.Selector.MainTypes)
	def.Selector.SubTypes = normalizeStrings(def.Selector.SubTypes)
	if len(def.Params) == 0 || string(def.Params) == "null" {
		def.Params = json.RawMessage(`{}`)
	}
	return def
}

func normalizeStrings(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		out = append(out, strings.TrimSpace(value))
	}
	sort.Strings(out)
	return out
}
