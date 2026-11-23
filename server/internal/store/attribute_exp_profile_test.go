package store

import (
	"testing"

	"mysecondwebgame/server/internal/catalog"
)

func TestAttributeExpProfileProgress(t *testing.T) {
	resolver, err := buildAttributeExpProfileResolver(catalog.Data{
		Attributes: []catalog.AttributeDefinition{
			{ID: "battle", GrowthType: "levelable", ExpProfileID: "base"},
		},
		AttributeExpRules: []catalog.AttributeExpProfile{
			{ID: "base", Level: 1, ExpRequired: 3},
			{ID: "base", Level: 2, ExpRequired: 7},
		},
	})
	if err != nil {
		t.Fatalf("buildAttributeExpProfileResolver() err=%v", err)
	}

	cases := []struct {
		name    string
		exp     float64
		level   int64
		current float64
		next    float64
	}{
		{name: "initial", exp: 0, level: 1, current: 0, next: 3},
		{name: "within level 1", exp: 2, level: 1, current: 2, next: 3},
		{name: "just reached level 2", exp: 3, level: 2, current: 0, next: 7},
		{name: "within level 2", exp: 9, level: 2, current: 6, next: 7},
		{name: "capped", exp: 10, level: 3, current: 0, next: 0},
		{name: "overflow still capped", exp: 999, level: 3, current: 0, next: 0},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, progressErr := resolver.progress("battle", tc.exp)
			if progressErr != nil {
				t.Fatalf("progress err=%v", progressErr)
			}
			if got.Level != tc.level || got.Exp != tc.current || got.NextExp != tc.next {
				t.Fatalf("progress(level=%d exp=%v next=%v), want (%d %v %v)", got.Level, got.Exp, got.NextExp, tc.level, tc.current, tc.next)
			}
		})
	}
}

func TestAttributeExpProfileClampTotalExp(t *testing.T) {
	resolver, err := buildAttributeExpProfileResolver(catalog.Data{
		Attributes: []catalog.AttributeDefinition{
			{ID: "battle", GrowthType: "levelable", ExpProfileID: "base"},
		},
		AttributeExpRules: []catalog.AttributeExpProfile{
			{ID: "base", Level: 1, ExpRequired: 3},
			{ID: "base", Level: 2, ExpRequired: 7},
		},
	})
	if err != nil {
		t.Fatalf("buildAttributeExpProfileResolver() err=%v", err)
	}

	got, err := resolver.clampTotalExp("battle", 999)
	if err != nil {
		t.Fatalf("clampTotalExp err=%v", err)
	}
	if got != 10 {
		t.Fatalf("clampTotalExp=%.2f, want 10", got)
	}
}

func TestAttributeExpProfileTerminalZeroCapsAtDeclaredLevel(t *testing.T) {
	resolver, err := buildAttributeExpProfileResolver(catalog.Data{
		Attributes: []catalog.AttributeDefinition{
			{ID: "battle", GrowthType: "levelable", ExpProfileID: "base"},
		},
		AttributeExpRules: []catalog.AttributeExpProfile{
			{ID: "base", Level: 1, ExpRequired: 3},
			{ID: "base", Level: 2, ExpRequired: 7},
			{ID: "base", Level: 3, ExpRequired: 0},
		},
	})
	if err != nil {
		t.Fatalf("buildAttributeExpProfileResolver() err=%v", err)
	}

	got, err := resolver.progress("battle", 10)
	if err != nil {
		t.Fatalf("progress err=%v", err)
	}
	if got.Level != 3 || got.Exp != 0 || got.NextExp != 0 {
		t.Fatalf("progress(level=%d exp=%v next=%v), want (3 0 0)", got.Level, got.Exp, got.NextExp)
	}
	clamped, err := resolver.clampTotalExp("battle", 999)
	if err != nil {
		t.Fatalf("clampTotalExp err=%v", err)
	}
	if clamped != 10 {
		t.Fatalf("clampTotalExp=%.2f, want 10", clamped)
	}
}

func TestBuildAttributeExpProfileResolverRequiresLevelOne(t *testing.T) {
	_, err := buildAttributeExpProfileResolver(catalog.Data{
		Attributes: []catalog.AttributeDefinition{
			{ID: "battle", GrowthType: "levelable", ExpProfileID: "base"},
		},
		AttributeExpRules: []catalog.AttributeExpProfile{
			{ID: "base", Level: 2, ExpRequired: 7},
		},
	})
	if err == nil {
		t.Fatalf("expected resolver build error when profile does not start at level 1")
	}
}
