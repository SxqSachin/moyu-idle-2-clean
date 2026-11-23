package marketpolicy

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"sort"
	"strconv"
	"strings"
)

const InstanceVariants = "instance_variants"

type Policy struct {
	FixedPrice FixedPrice `json:"fixedPrice"`
	BuyOrders  bool       `json:"buyOrders"`
	Auction    bool       `json:"auction"`
}
type FixedPrice struct {
	Mode      string `json:"mode"`
	ProfileID string `json:"profileId"`
}
type Key struct {
	Source  string `json:"source"`
	Path    string `json:"path"`
	Compare string `json:"compare"`
}
type Field struct {
	Key
	LabelKey string `json:"labelKey"`
	Format   string `json:"format"`
}
type Profile struct {
	ID               string         `json:"id"`
	Disabled         bool           `json:"disabled,omitempty"`
	SchemaVersion    int            `json:"schemaVersion"`
	GroupingVersion  int            `json:"groupingVersion"`
	CatalogKeys      []Key          `json:"catalogKeys"`
	CatalogSource    *CatalogSource `json:"catalogSource,omitempty"`
	GroupKeys        []Key          `json:"groupKeys"`
	UniqueByInstance bool           `json:"uniqueByInstance,omitempty"`
	Presentation     []Field        `json:"presentation"`
}

// CatalogSource describes display entries, not inventory templates or offer identity.
// Values are stable string identifiers, one per catalog key.
type CatalogSource struct {
	Table      string   `json:"table"`
	NamesPath  string   `json:"namesPath"`
	ValuePaths []string `json:"valuePaths"`
}

func validCatalogPath(path string) bool {
	return strings.HasPrefix(path, "/") && len(path) > 1 && validPointer(path)
}

func strictDecode(raw []byte, out any) error {
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err := d.Decode(out); err != nil {
		return err
	}
	if err := d.Decode(new(any)); err != io.EOF {
		return errors.New("trailing market configuration")
	}
	return nil
}
func (p *Policy) UnmarshalJSON(raw []byte) error {
	type alias Policy
	var v alias
	if err := strictDecode(raw, &v); err != nil {
		return err
	}
	var required map[string]json.RawMessage
	if err := json.Unmarshal(raw, &required); err != nil {
		return err
	}
	for _, key := range []string{"fixedPrice", "buyOrders", "auction"} {
		if len(required[key]) == 0 || bytes.Equal(required[key], []byte("null")) {
			return errors.New("missing market capability")
		}
	}
	*p = Policy(v)
	return nil
}
func (p *Profile) UnmarshalJSON(raw []byte) error {
	type alias Profile
	var v alias
	if err := strictDecode(raw, &v); err != nil {
		return err
	}
	*p = Profile(v)
	return nil
}
func (p Policy) Validate() error {
	if p.FixedPrice.Mode != InstanceVariants || p.FixedPrice.ProfileID == "" || p.BuyOrders {
		return errors.New("unsupported market policy")
	}
	return nil
}
func validPointer(path string) bool {
	if len(path) > 256 || (path != "" && !strings.HasPrefix(path, "/")) {
		return false
	}
	for _, s := range strings.Split(path, "/") {
		if strings.ContainsAny(s, "*[]") || s == "__proto__" || s == "constructor" || s == "prototype" {
			return false
		}
		for i := 0; i < len(s); i++ {
			if s[i] == '~' && (i+1 == len(s) || (s[i+1] != '0' && s[i+1] != '1')) {
				return false
			}
		}
	}
	return true
}
func validKey(k Key) bool {
	if k.Source != "instance" && k.Source != "resolved.operationEffect" {
		return false
	}
	if k.Compare != "exact" && k.Compare != "set" {
		return false
	}
	if !validPointer(k.Path) {
		return false
	}
	if k.Path != "" {
		root := strings.Split(k.Path[1:], "/")[0]
		allowed := map[string]bool{"baseSnapshot": true, "customData": true, "enhanceLevel": true, "affixes": true, "durability": true, "maxDurability": true, "fishData": true, "specimenNames": true}
		if k.Source == "resolved.operationEffect" {
			allowed = map[string]bool{"id": true, "rulesetVersion": true, "domain": true, "kind": true, "priority": true, "params": true, "lifetime": true, "allowedTargets": true, "selector": true, "stacking": true}
		}
		if !allowed[root] {
			return false
		}
	}
	return true
}
func (p Profile) Validate() error {
	if p.ID == "" || p.SchemaVersion != 1 || p.GroupingVersion < 1 || len(p.GroupKeys) == 0 || len(p.GroupKeys) > 32 || len(p.CatalogKeys) > 4 || len(p.Presentation) > 12 {
		return errors.New("invalid market profile")
	}
	for _, k := range append(append([]Key{}, p.CatalogKeys...), p.GroupKeys...) {
		if !validKey(k) {
			return fmt.Errorf("invalid market key: %s", k.Path)
		}
	}
	if s := p.CatalogSource; s != nil {
		if s.Table == "" || !validCatalogPath(s.NamesPath) || len(s.ValuePaths) == 0 || len(s.ValuePaths) != len(p.CatalogKeys) {
			return errors.New("invalid market catalog source")
		}
		for i, ch := range s.Table {
			if !(ch >= 'a' && ch <= 'z') && !(i > 0 && (ch == '_' || ch >= '0' && ch <= '9')) {
				return errors.New("invalid market catalog table")
			}
		}
		for i, path := range s.ValuePaths {
			k := p.CatalogKeys[i]
			if !validCatalogPath(path) || k.Source != "instance" || k.Compare != "exact" || k.Path == "" {
				return errors.New("invalid market catalog selector")
			}
		}
	}
	for _, f := range p.Presentation {
		if !validKey(f.Key) || f.LabelKey == "" {
			return errors.New("invalid market presentation field")
		}
		switch f.Format {
		case "text", "number", "percent", "seconds", "operation_effect":
		default:
			return errors.New("invalid market field format")
		}
	}
	return nil
}

func (p Profile) CatalogFilter(itemID, raw string) (string, error) {
	if p.Disabled || p.CatalogSource == nil || p.Validate() != nil || len(raw) > 2048 {
		return "", errors.New("invalid market catalog filter")
	}
	var values []string
	if err := strictDecode([]byte(raw), &values); err != nil || len(values) != len(p.CatalogKeys) {
		return "", errors.New("invalid market catalog values")
	}
	for _, value := range values {
		if value == "" || len(value) > 256 {
			return "", errors.New("invalid market catalog value")
		}
	}
	return Digest([]any{itemID, values})
}

// Canonical numbers are exact rationals, not display-rounded floats. This also
// gives JSONB's 1, 1.0 and 1e0 the same identity without losing decimal precision.
func Canonical(v any) ([]byte, error) {
	raw, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	var node any
	if err = d.Decode(&node); err != nil {
		return nil, err
	}
	var normalize func(any) (any, error)
	normalize = func(v any) (any, error) {
		switch x := v.(type) {
		case json.Number:
			if len(x.String()) > 128 {
				return nil, errors.New("market number too large")
			}
			if parts := strings.FieldsFunc(x.String(), func(r rune) bool { return r == 'e' || r == 'E' }); len(parts) == 2 {
				exp, e := strconv.Atoi(parts[1])
				if e != nil || exp < -308 || exp > 308 {
					return nil, errors.New("market exponent out of range")
				}
			}
			r, ok := new(big.Rat).SetString(x.String())
			if !ok {
				return nil, errors.New("invalid market number")
			}
			return []any{"number", r.RatString()}, nil
		case map[string]any:
			keys := make([]string, 0, len(x))
			for k := range x {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			out := []any{"object"}
			for _, k := range keys {
				n, e := normalize(x[k])
				if e != nil {
					return nil, e
				}
				out = append(out, []any{k, n})
			}
			return out, nil
		case []any:
			out := []any{"array"}
			for _, v := range x {
				n, e := normalize(v)
				if e != nil {
					return nil, e
				}
				out = append(out, n)
			}
			return out, nil
		case string:
			return []any{"string", x}, nil
		case bool:
			return []any{"bool", x}, nil
		default:
			return []any{"null"}, nil
		}
	}
	n, err := normalize(node)
	if err != nil {
		return nil, err
	}
	return json.Marshal(n)
}
func Digest(v any) (string, error) {
	b, e := Canonical(v)
	if e != nil {
		return "", e
	}
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:]), nil
}

func Read(sources map[string]any, k Key) (any, error) {
	v, ok := sources[k.Source]
	if !ok {
		return nil, fmt.Errorf("missing market source %s", k.Source)
	}
	if k.Path != "" {
		for _, part := range strings.Split(k.Path[1:], "/") {
			part = strings.ReplaceAll(strings.ReplaceAll(part, "~1", "/"), "~0", "~")
			obj, yes := v.(map[string]any)
			if !yes {
				return nil, errors.New("invalid market field type")
			}
			v, ok = obj[part]
			if !ok {
				return nil, fmt.Errorf("missing market field %s", k.Path)
			}
		}
	}
	if v == nil {
		return nil, errors.New("null market field")
	}
	if k.Compare == "set" {
		arr, ok := v.([]any)
		if !ok {
			return nil, errors.New("market set requires array")
		}
		values := map[string]any{}
		for _, v := range arr {
			c, e := Canonical(v)
			if e != nil {
				return nil, e
			}
			values[string(c)] = v
		}
		keys := make([]string, 0, len(values))
		for k := range values {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		out := make([]any, 0, len(keys))
		for _, k := range keys {
			out = append(out, values[k])
		}
		v = out
	}
	return v, nil
}

type DisplayField struct {
	LabelKey string `json:"labelKey"`
	Format   string `json:"format"`
	Value    any    `json:"value"`
}
type Projection struct {
	CatalogKey string          `json:"catalogKey"`
	GroupKey   string          `json:"groupKey"`
	Revision   string          `json:"revision"`
	Descriptor json.RawMessage `json:"-"`
	Fields     []DisplayField  `json:"fields"`
}

func Project(p Profile, itemID string, instanceID int64, sources map[string]any) (Projection, error) {
	if p.Disabled {
		return Projection{}, errors.New("market profile disabled")
	}
	if err := p.Validate(); err != nil {
		return Projection{}, err
	}
	var out Projection
	readKeys := func(keys []Key) ([]any, error) {
		out := make([]any, 0, len(keys))
		for _, k := range keys {
			v, e := Read(sources, k)
			if e != nil {
				return nil, e
			}
			out = append(out, v)
		}
		return out, nil
	}
	c, e := readKeys(p.CatalogKeys)
	if e != nil {
		return out, e
	}
	g, e := readKeys(p.GroupKeys)
	if e != nil {
		return out, e
	}
	// Presentation-only changes must not invalidate existing offers.
	out.Revision, e = Digest([]any{p.ID, p.SchemaVersion, p.GroupingVersion, p.CatalogKeys, p.GroupKeys, p.UniqueByInstance})
	if e != nil {
		return out, e
	}
	out.CatalogKey, e = Digest([]any{itemID, c})
	if e != nil {
		return out, e
	}
	identity := []any{itemID, out.Revision, c, g}
	if p.UniqueByInstance {
		identity = append(identity, instanceID)
	}
	out.GroupKey, e = Digest(identity)
	if e != nil {
		return out, e
	}
	out.Descriptor, e = Canonical(identity)
	if e != nil {
		return out, e
	}
	out.Fields = make([]DisplayField, 0, len(p.Presentation))
	for _, f := range p.Presentation {
		v, e := Read(sources, f.Key)
		if e != nil {
			return out, e
		}
		if f.Format == "number" || f.Format == "percent" || f.Format == "seconds" {
			switch v.(type) {
			case json.Number, float64, int, int64:
			default:
				return out, errors.New("market numeric presentation requires number")
			}
		}
		out.Fields = append(out.Fields, DisplayField{f.LabelKey, f.Format, v})
	}
	return out, nil
}
