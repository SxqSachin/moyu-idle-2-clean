package operationeffect

import (
	"errors"
	"math"
)

// Attribution describes already granted rewards; it is never a grant command.
type Contribution struct {
	Experience    map[string]float64 `json:"experience,omitempty"`
	Items         map[string]int64   `json:"items,omitempty"`
	Opportunities int64              `json:"opportunities,omitempty"`
	Triggered     int64              `json:"triggered,omitempty"`
	Empty         int64              `json:"empty,omitempty"`
}
type Contributions map[string]Contribution

func (c Contributions) Validate() error {
	if len(c) > 32 {
		return errors.New("contribution source bound exceeded")
	}
	for id, r := range c {
		if id == "" || len(id) > 80 || len(r.Experience) > 128 || len(r.Items) > 2048 || r.Opportunities < 0 || r.Opportunities > 1e12 || r.Triggered < 0 || r.Empty < 0 || r.Empty > r.Triggered || r.Triggered > r.Opportunities {
			return errors.New("invalid contribution")
		}
		for key, n := range r.Experience {
			if key == "" || math.IsNaN(n) || math.IsInf(n, 0) || n < 0 || n > 1e12 {
				return errors.New("invalid experience contribution")
			}
		}
		for key, n := range r.Items {
			if key == "" || n < 0 || n > 1e12 {
				return errors.New("invalid item contribution")
			}
		}
	}
	return nil
}

func MergeContributions(a, b Contributions) (Contributions, error) {
	if err := a.Validate(); err != nil {
		return nil, err
	}
	if err := b.Validate(); err != nil {
		return nil, err
	}
	out := Contributions{}
	for _, src := range []Contributions{a, b} {
		for id, r := range src {
			target := out[id]
			if target.Experience == nil {
				target.Experience = map[string]float64{}
			}
			if target.Items == nil {
				target.Items = map[string]int64{}
			}
			for k, v := range r.Experience {
				target.Experience[k] = math.Round((target.Experience[k]+v)*10000) / 10000
			}
			for k, v := range r.Items {
				target.Items[k] += v
			}
			target.Opportunities += r.Opportunities
			target.Triggered += r.Triggered
			target.Empty += r.Empty
			out[id] = target
		}
	}
	return out, out.Validate()
}

func ContributionDifference(total, seen Contributions) Contributions {
	out := Contributions{}
	for id, r := range total {
		before := seen[id]
		d := Contribution{Experience: map[string]float64{}, Items: map[string]int64{}, Opportunities: r.Opportunities - before.Opportunities, Triggered: r.Triggered - before.Triggered, Empty: r.Empty - before.Empty}
		for k, n := range r.Experience {
			if v := math.Round((n-before.Experience[k])*10000) / 10000; v > 0 {
				d.Experience[k] = v
			}
		}
		for k, n := range r.Items {
			if v := n - before.Items[k]; v > 0 {
				d.Items[k] = v
			}
		}
		if len(d.Experience)+len(d.Items) > 0 || d.Opportunities > 0 {
			out[id] = d
		}
	}
	return out
}
