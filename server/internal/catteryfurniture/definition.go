package catteryfurniture

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"math/big"
	"mysecondwebgame/server/internal/operationeffect"
	"time"
)

type Definition struct {
	Service         string                        `json:"service,omitempty"`
	Level           int                           `json:"level,omitempty"`
	Levels          []Level                       `json:"levels,omitempty"`
	ID              string                        `json:"id"`
	Version         string                        `json:"version"`
	NameKey         string                        `json:"nameKey"`
	IconID          string                        `json:"iconId"`
	InstallCosts    map[string]int64              `json:"installCosts"`
	RefillCosts     map[string]int64              `json:"refillCosts"`
	CapacitySeconds int64                         `json:"capacitySeconds"`
	RefillSeconds   int64                         `json:"refillSeconds"`
	Rules           []operationeffect.PassiveRule `json:"rules"`
}

func (d Definition) Permanent() bool { return d.Service == "permanent" }

type Level struct {
	Level         int                           `json:"level"`
	UpgradeCosts  map[string]int64              `json:"upgradeCosts"`
	RefillCostAdd map[string]int64              `json:"refillCostAdd"`
	Rules         []operationeffect.PassiveRule `json:"rules"`
}

func (d Definition) CurrentLevel() int {
	if d.Level == 0 {
		return 1
	}
	return d.Level
}

// Catalog tiers are expanded once at the furniture boundary. Executors only
// receive the selected flat, immutable definition.
func (d Definition) AtLevel(level int) (Definition, error) {
	if level < 1 || level > len(d.Levels)+1 {
		return Definition{}, errors.New("invalid furniture level")
	}
	out := d
	out.Level, out.Levels = level, nil
	out.RefillCosts = maps.Clone(d.RefillCosts)
	for _, stage := range d.Levels {
		if stage.Level > level {
			break
		}
		for id, n := range stage.RefillCostAdd {
			out.RefillCosts[id] += n
		}
		out.Rules = stage.Rules
	}
	return out, out.Validate()
}

func ValidateCosts(costs map[string]int64) error {
	if len(costs) == 0 || len(costs) > 24 {
		return errors.New("invalid furniture costs")
	}
	for id, n := range costs {
		if id == "" || n <= 0 || n > 1000000 {
			return errors.New("invalid furniture material")
		}
	}
	return nil
}

// Quote the positive difference between paid service rates. Integer rational
// arithmetic handles old definitions with a different pack duration exactly.
func UpgradeSupplement(old, next Definition, remainingMS int64) (map[string]int64, error) {
	if old.Permanent() || next.Permanent() {
		if !old.Permanent() || !next.Permanent() || remainingMS != 0 {
			return nil, errors.New("incompatible furniture service")
		}
		return map[string]int64{}, nil
	}
	if remainingMS < 0 || remainingMS > 7*86400000 || old.RefillSeconds <= 0 || next.RefillSeconds <= 0 {
		return nil, errors.New("invalid paid service")
	}
	out := map[string]int64{}
	for id, n := range next.RefillCosts {
		delta := new(big.Rat).Sub(big.NewRat(n, next.RefillSeconds*1000), big.NewRat(old.RefillCosts[id], old.RefillSeconds*1000))
		if delta.Sign() <= 0 {
			continue
		}
		delta.Mul(delta, new(big.Rat).SetInt64(remainingMS))
		q, rem := new(big.Int), new(big.Int)
		q.QuoRem(delta.Num(), delta.Denom(), rem)
		if rem.Sign() > 0 {
			q.Add(q, big.NewInt(1))
		}
		if !q.IsInt64() || q.Int64() > 100000000 {
			return nil, errors.New("upgrade supplement overflow")
		}
		if q.Sign() > 0 {
			out[id] = q.Int64()
		}
	}
	return out, nil
}

func (d *Definition) UnmarshalJSON(raw []byte) error {
	type value Definition
	var v value
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&v); err != nil {
		return err
	}
	*d = Definition(v)
	return d.Validate()
}

func (d Definition) Validate() error {
	if d.CurrentLevel() < 1 || d.CurrentLevel() > 10 || len(d.Levels) > 9 || (len(d.Levels) > 0 && d.CurrentLevel() != 1) {
		return errors.New("invalid furniture progression")
	}
	if d.ID == "" || len(d.ID) > 80 || d.Version == "" || len(d.Version) > 80 || d.NameKey == "" || d.IconID == "" || len(d.Rules) == 0 || len(d.Rules) > 8 {
		return errors.New("invalid furniture definition")
	}
	if d.Service != "" && d.Service != "prepaid" && !d.Permanent() {
		return errors.New("invalid furniture service")
	}
	costSets := []map[string]int64{d.InstallCosts}
	if d.Permanent() {
		if d.CapacitySeconds != 0 || d.RefillSeconds != 0 || len(d.RefillCosts) != 0 {
			return errors.New("permanent furniture cannot require maintenance")
		}
	} else {
		if d.CapacitySeconds <= 0 || d.CapacitySeconds > 7*86400 || d.RefillSeconds <= 0 || d.RefillSeconds > d.CapacitySeconds {
			return errors.New("invalid furniture service duration")
		}
		costSets = append(costSets, d.RefillCosts)
	}
	for _, costs := range costSets {
		if err := ValidateCosts(costs); err != nil {
			return err
		}
	}
	for i, stage := range d.Levels {
		if stage.Level != i+2 || len(stage.Rules) == 0 || len(stage.Rules) > 8 {
			return fmt.Errorf("invalid furniture tier %d", stage.Level)
		}
		if err := ValidateCosts(stage.UpgradeCosts); err != nil {
			return err
		}
		if d.Permanent() {
			if len(stage.RefillCostAdd) != 0 {
				return errors.New("permanent tier cannot require maintenance")
			}
		} else if err := ValidateCosts(stage.RefillCostAdd); err != nil {
			return err
		}
		if _, err := d.AtLevel(stage.Level); err != nil {
			return err
		}
	}
	for _, r := range d.Rules {
		if err := r.Validate(); err != nil {
			return err
		}
	}
	return nil
}

func (d Definition) Source() operationeffect.PassiveSource {
	return operationeffect.PassiveSource{ID: d.ID, Version: d.Version, Level: d.CurrentLevel(), Rules: d.Rules}
}

// Paid time is the single maintenance unit: no floating point wear or cron.
func Remaining(savedMS int64, until *time.Time, now time.Time) int64 {
	if until == nil {
		return savedMS
	}
	ms := until.Sub(now).Milliseconds()
	if ms < 0 {
		return 0
	}
	return ms
}
