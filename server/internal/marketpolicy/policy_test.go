package marketpolicy

import (
	"encoding/json"
	"testing"
)

func testProfile() Profile {
	return Profile{ID: "arbitrary_v1", SchemaVersion: 1, GroupingVersion: 1, CatalogKeys: []Key{}, GroupKeys: []Key{{Source: "instance", Path: "/baseSnapshot/values", Compare: "exact"}}, Presentation: []Field{}}
}

func TestCatalogSourcePreservesIdentityAndFiltersExactCatalog(t *testing.T) {
	p := testProfile()
	p.CatalogKeys = []Key{{Source: "instance", Path: "/customData/family", Compare: "exact"}}
	sources := map[string]any{"instance": map[string]any{"customData": map[string]any{"family": "crystal"}, "baseSnapshot": map[string]any{"values": 1}}}
	before, err := Project(p, "arbitrary", 1, sources)
	if err != nil {
		t.Fatal(err)
	}
	p.CatalogSource = &CatalogSource{Table: "families", NamesPath: "/names", ValuePaths: []string{"/id"}}
	after, err := Project(p, "arbitrary", 1, sources)
	if err != nil || after.GroupKey != before.GroupKey || after.Revision != before.Revision {
		t.Fatalf("display source invalidated existing offers: %v", err)
	}
	key, err := p.CatalogFilter("arbitrary", `["crystal"]`)
	if err != nil || key != before.CatalogKey {
		t.Fatalf("catalog filter mismatch: %v", err)
	}
	other, _ := p.CatalogFilter("arbitrary", `["other"]`)
	if other == key {
		t.Fatal("different catalog values merged")
	}
	for _, raw := range []string{"", `null`, `[]`, `[null]`, `[1]`, `[""]`, `["crystal","other"]`, `["crystal"] true`, `{}`} {
		if _, err = p.CatalogFilter("arbitrary", raw); err == nil {
			t.Fatalf("accepted invalid values: %s", raw)
		}
	}
	p.CatalogSource.ValuePaths[0] = "/__proto__/id"
	if p.Validate() == nil {
		t.Fatal("unsafe catalog path accepted")
	}
}
func TestCanonicalExactNumbersAndTypes(t *testing.T) {
	equal := [][]string{{`1`, `1.0`, `1e0`}, {`{"x":1,"y":2}`, `{"y":2.0,"x":1.0}`}}
	for _, group := range equal {
		a, _ := Digest(json.RawMessage(group[0]))
		for _, v := range group {
			b, e := Digest(json.RawMessage(v))
			if e != nil || a != b {
				t.Fatalf("canonical mismatch %s: %v", v, e)
			}
		}
	}
	different := [][2]string{{`0.10401`, `0.10402`}, {`null`, `0`}, {`1`, `"1"`}, {`[1,2]`, `[2,1]`}, {`{}`, `{"x":null}`}, {`["number","1"]`, `1`}}
	for _, pair := range different {
		a, _ := Digest(json.RawMessage(pair[0]))
		b, _ := Digest(json.RawMessage(pair[1]))
		if a == b {
			t.Fatalf("merged %v", pair)
		}
	}
	if _, err := Canonical(json.RawMessage(`1e999999999`)); err == nil {
		t.Fatal("unbounded exponent")
	}
}
func TestProfileGroupingDoesNotDependOnItemLabelsOrSeller(t *testing.T) {
	p := testProfile()
	values := map[string]any{"rate": .104, "charges": 20}
	sources := map[string]any{"instance": map[string]any{"baseSnapshot": map[string]any{"values": values}}}
	a, e := Project(p, "unrelated_crystal", 1, sources)
	if e != nil {
		t.Fatal(e)
	}
	b, e := Project(p, "unrelated_crystal", 2, sources)
	if e != nil || a.GroupKey != b.GroupKey {
		t.Fatal("equal variants did not merge")
	}
	p.Presentation = []Field{{Key: Key{Source: "instance", Path: "/baseSnapshot/values/rate", Compare: "exact"}, LabelKey: "renamed", Format: "percent"}}
	b, e = Project(p, "unrelated_crystal", 2, sources)
	if e != nil || a.GroupKey != b.GroupKey {
		t.Fatal("presentation changed identity")
	}
	values["charges"] = 25
	b, _ = Project(p, "unrelated_crystal", 2, sources)
	if a.GroupKey == b.GroupKey {
		t.Fatal("different lifetime merged")
	}
	p.UniqueByInstance = true
	a, _ = Project(p, "unrelated_crystal", 1, sources)
	b, _ = Project(p, "unrelated_crystal", 2, sources)
	if a.GroupKey == b.GroupKey {
		t.Fatal("unique instances merged")
	}
}
func TestProfileRejectsUnknownOrMissingConfiguration(t *testing.T) {
	var p Profile
	if json.Unmarshal([]byte(`{"id":"x","script":"evil"}`), &p) == nil {
		t.Fatal("unknown field accepted")
	}
	p = testProfile()
	p.GroupKeys[0].Path = "/values/*"
	if p.Validate() == nil {
		t.Fatal("wildcard accepted")
	}
	p = testProfile()
	if _, err := Project(p, "x", 1, map[string]any{"instance": map[string]any{}}); err == nil {
		t.Fatal("missing group value accepted")
	}
	var policy Policy
	if json.Unmarshal([]byte(`{"fixedPrice":{"mode":"instance_variants","profileId":"x"}}`), &policy) == nil {
		t.Fatal("missing capabilities accepted")
	}
	if json.Unmarshal([]byte(`{"fixedPrice":{"mode":"instance_variants","profileId":"x","sql":"x"},"buyOrders":false,"auction":false}`), &policy) == nil {
		t.Fatal("unknown nested field accepted")
	}
}
func TestExplicitSetComparison(t *testing.T) {
	p := testProfile()
	p.GroupKeys[0].Compare = "set"
	a, err := Project(p, "x", 1, map[string]any{"instance": map[string]any{"baseSnapshot": map[string]any{"values": []any{1, 2}}}})
	if err != nil {
		t.Fatal(err)
	}
	b, err := Project(p, "x", 2, map[string]any{"instance": map[string]any{"baseSnapshot": map[string]any{"values": []any{2, 1, 1}}}})
	if err != nil || a.GroupKey != b.GroupKey {
		t.Fatal("set comparison unstable")
	}
}
