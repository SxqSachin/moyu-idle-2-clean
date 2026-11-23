package operationeffect

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"time"
)

// Passive rules have an external lifecycle (for example a maintained facility).
// They share operation selectors without pretending to be consumable food grants.
const (
	KindOwnerExperience         = "owner_experience"
	KindExtraLoot               = "kitty_extra_loot"
	KindActionEfficiency        = "action_efficiency_add"
	DomainActionStart    Domain = "action_start"
	DomainBattleReward   Domain = "battle_reward"
)

type PassiveRule struct {
	Kind     string   `json:"kind"`
	Domain   Domain   `json:"domain"`
	RateBPS  int64    `json:"rateBps"`
	Selector Selector `json:"selector,omitempty"`
}

func (r *PassiveRule) UnmarshalJSON(raw []byte) error {
	type value PassiveRule
	var v value
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err := d.Decode(&v); err != nil {
		return err
	}
	*r = PassiveRule(v)
	return r.Validate()
}

func (r PassiveRule) Validate() error {
	for _, values := range [][]string{r.Selector.ActionIDs, r.Selector.MainTypes, r.Selector.SubTypes} {
		if len(values) > 64 {
			return errors.New("passive selector too large")
		}
		seen := map[string]bool{}
		for _, value := range values {
			if value == "" || len(value) > 160 || seen[value] {
				return errors.New("invalid passive selector")
			}
			seen[value] = true
		}
	}
	maxRate := int64(10000)
	if r.Kind == KindActionEfficiency {
		maxRate = 20000
	}
	if r.RateBPS < 0 || r.RateBPS > maxRate {
		return fmt.Errorf("passive rate must be 0..%d basis points", maxRate)
	}
	switch r.Kind {
	case KindActionEfficiency:
		if r.Domain == DomainActionStart && len(r.Selector.MainTypes) > 0 {
			for _, main := range r.Selector.MainTypes {
				if !WorkEfficiencyMainType(main) {
					return errors.New("unsupported work efficiency scope")
				}
			}
			return nil
		}
	case KindOwnerExperience:
		if r.Domain == DomainLifeAction || r.Domain == DomainFishingCast {
			return nil
		}
	case KindExtraLoot:
		if r.Domain == DomainBattleReward && len(r.Selector.ActionIDs)+len(r.Selector.MainTypes)+len(r.Selector.SubTypes) == 0 {
			return nil
		}
	}
	return fmt.Errorf("unsupported passive rule %s/%s", r.Domain, r.Kind)
}

type PassiveSource struct {
	Level   int           `json:"level,omitempty"`
	ID      string        `json:"id"`
	Version string        `json:"version"`
	Rules   []PassiveRule `json:"rules"`
}

type PassiveWindow struct {
	From   time.Time     `json:"from"`
	Until  time.Time     `json:"until"`
	Source PassiveSource `json:"source"`
}

type PassiveSnapshot struct {
	Version int             `json:"version"`
	Sources []PassiveSource `json:"sources,omitempty"`
}

func (s PassiveSnapshot) Validate() error {
	if s.Version != 1 || len(s.Sources) > 32 {
		return errors.New("invalid passive snapshot")
	}
	seen := map[string]bool{}
	for _, source := range s.Sources {
		if source.ID == "" || source.Version == "" || source.Level < 0 || source.Level > 10 || seen[source.ID] || len(source.Rules) == 0 || len(source.Rules) > 8 {
			return errors.New("invalid passive source")
		}
		seen[source.ID] = true
		for _, rule := range source.Rules {
			if err := rule.Validate(); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s PassiveSnapshot) Best(kind string) (int64, string) {
	var rate int64
	var sourceID string
	for _, source := range s.Sources {
		for _, rule := range source.Rules {
			if rule.Kind == kind && (rule.RateBPS > rate || (rule.RateBPS == rate && source.ID < sourceID)) {
				rate, sourceID = rule.RateBPS, source.ID
			}
		}
	}
	return rate, sourceID
}

type PassiveSession struct {
	Contributions          Contributions
	PendingExperience      map[string]map[string]float64
	Windows                []PassiveWindow
	Remainders             map[string]int64
	Dirty                  bool
	PendingOwnerExperience bool
	ProgressOrdering       bool
	windowIndex            map[string][]PassiveWindow
	indexedWindowCount     int
	efficiencyBoundaries   map[string][]time.Time
}

// A shared progress writer introduces a causal boundary for player starts.
func (s *PassiveSession) AffectsOwnerProgress() bool {
	if s == nil {
		return false
	}
	if s.PendingOwnerExperience {
		return true
	}
	for _, w := range s.Windows {
		for _, r := range w.Source.Rules {
			if r.Kind == KindOwnerExperience && r.RateBPS > 0 {
				return true
			}
		}
	}
	return false
}

func (s *PassiveSession) Snapshot(op Operation) PassiveSnapshot {
	out := PassiveSnapshot{Version: 1}
	if s == nil || op.ActorKind != ActorKitty {
		return out
	}
	s.ensureWindowIndex()
	for _, windows := range s.windowIndex {
		index := sort.Search(len(windows), func(i int) bool { return windows[i].From.After(op.StartedAt) }) - 1
		if index < 0 {
			continue
		}
		window := windows[index]
		if op.StartedAt.Before(window.From) || (!window.Until.IsZero() && !op.StartedAt.Before(window.Until)) {
			continue
		}
		source := PassiveSource{ID: window.Source.ID, Version: window.Source.Version, Level: window.Source.Level}
		for _, rule := range window.Source.Rules {
			if rule.Domain == op.Domain && rule.RateBPS > 0 && selectorMatches(rule.Selector, op) {
				source.Rules = append(source.Rules, rule)
			}
		}
		if len(source.Rules) > 0 {
			out.Sources = append(out.Sources, source)
		}
	}
	sort.Slice(out.Sources, func(i, j int) bool { return out.Sources[i].ID < out.Sources[j].ID })
	return out
}

func (s *PassiveSession) ensureWindowIndex() {
	if s.windowIndex == nil || s.indexedWindowCount != len(s.Windows) {
		s.windowIndex = map[string][]PassiveWindow{}
		s.efficiencyBoundaries = map[string][]time.Time{}
		for _, w := range s.Windows {
			s.windowIndex[w.Source.ID] = append(s.windowIndex[w.Source.ID], w)
			for _, rule := range w.Source.Rules {
				if rule.Kind != KindActionEfficiency {
					continue
				}
				for _, main := range rule.Selector.MainTypes {
					s.efficiencyBoundaries[main] = append(s.efficiencyBoundaries[main], w.From)
					if !w.Until.IsZero() {
						s.efficiencyBoundaries[main] = append(s.efficiencyBoundaries[main], w.Until)
					}
				}
			}
		}
		for _, windows := range s.windowIndex {
			sort.Slice(windows, func(i, j int) bool { return windows[i].From.Before(windows[j].From) })
		}
		for _, times := range s.efficiencyBoundaries {
			sort.Slice(times, func(i, j int) bool { return times[i].Before(times[j]) })
		}
		s.indexedWindowCount = len(s.Windows)
	}
}

// Share uses fixed point and a per-owner, per-attribute carry. Partitioning a
// recovery transaction never destroys fractional experience.
func (s *PassiveSession) Share(snapshot PassiveSnapshot, exp map[string]float64) (map[string]float64, error) {
	return s.ShareRepeated(snapshot, exp, 1)
}

// Repeated identical operations are evaluated algebraically, with exactly the
// same per-operation input quantization and carry as individual settlement.
func (s *PassiveSession) ShareRepeated(snapshot PassiveSnapshot, exp map[string]float64, count int64) (map[string]float64, error) {
	out := map[string]float64{}
	if count < 0 {
		return nil, errors.New("invalid experience operation count")
	}
	if err := snapshot.Validate(); err != nil {
		return nil, err
	}
	rate, source := snapshot.Best(KindOwnerExperience)
	if rate == 0 || count == 0 {
		return out, nil
	}
	if s == nil {
		return nil, errors.New("passive experience session missing")
	}
	if s.Remainders == nil {
		s.Remainders = map[string]int64{}
	}
	for key, value := range exp {
		if math.IsNaN(value) || math.IsInf(value, 0) || value < 0 || value > float64(math.MaxInt64/10000)/10000 {
			return nil, errors.New("invalid source experience")
		}
		units := int64(math.Round(value * 10000))
		if units == 0 {
			continue
		}
		carry := s.Remainders[key]
		if carry < 0 || carry >= 10000 {
			return nil, errors.New("invalid experience remainder")
		}
		if units > (math.MaxInt64-carry)/rate/count {
			return nil, errors.New("experience overflow")
		}
		total := units*rate*count + carry
		out[key] = float64(total/10000) / 10000
		s.Remainders[key] = total % 10000
		s.Dirty = true
	}
	if s.PendingExperience == nil {
		s.PendingExperience = map[string]map[string]float64{}
	}
	row := s.PendingExperience[source]
	if row == nil {
		row = map[string]float64{}
	}
	for key, value := range out {
		row[key] = math.Round((row[key]+value)*10000) / 10000
		if row[key] > 1e12 {
			return nil, errors.New("experience contribution overflow")
		}
	}
	s.PendingExperience[source] = row
	return out, nil
}

// Attribute only accepted XP. Ordinary rewards get the remaining cap first;
// source order makes attribution deterministic when several effects share it.
func (s *PassiveSession) CommitExperience(requested, applied, owner map[string]float64) error {
	if s == nil || len(s.PendingExperience) == 0 {
		return nil
	}
	remaining := map[string]float64{}
	for key, n := range owner {
		remaining[key] = math.Round(math.Max(0, math.Min(n, applied[key]-(requested[key]-n)))*10000) / 10000
		owner[key] = remaining[key]
	}
	if s.Contributions == nil {
		s.Contributions = Contributions{}
	}
	ids := make([]string, 0, len(s.PendingExperience))
	for id := range s.PendingExperience {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		row := s.Contributions[id]
		for key, n := range s.PendingExperience[id] {
			gain := math.Min(n, remaining[key])
			if gain <= 0 {
				continue
			}
			if row.Experience == nil {
				row.Experience = map[string]float64{}
			}
			row.Experience[key] = math.Round((row.Experience[key]+gain)*10000) / 10000
			if row.Experience[key] > 1e12 {
				return errors.New("experience contribution overflow")
			}
			remaining[key] = math.Round((remaining[key]-gain)*10000) / 10000
		}
		if len(row.Experience) > 0 {
			s.Contributions[id] = row
		}
	}
	s.PendingExperience = nil
	return nil
}

func (s *Session) Passive() *PassiveSession {
	if s == nil {
		return nil
	}
	return s.passive
}
func (s *Session) SetPassive(p *PassiveSession) { s.passive = p }
