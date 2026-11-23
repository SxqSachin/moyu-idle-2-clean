package store

import (
	"errors"
	"math"
	"testing"
)

func TestCheckedSkillExpTotalRejectsOverflowAndCorruptCurrentValue(t *testing.T) {
	tests := []struct {
		name    string
		current int64
		gained  int64
	}{
		{name: "addition overflow", current: 1, gained: math.MaxInt64},
		{name: "negative current", current: -1, gained: 1},
		{name: "negative gain", current: 1, gained: -1},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := checkedSkillExpTotal(tc.current, tc.gained); !errors.Is(err, ErrSkillExpOverflow) {
				t.Fatalf("error=%v, want ErrSkillExpOverflow", err)
			}
		})
	}

	if got, err := checkedSkillExpTotal(math.MaxInt64-2, 2); err != nil || got != math.MaxInt64 {
		t.Fatalf("boundary result=(%d, %v), want (%d, nil)", got, err, int64(math.MaxInt64))
	}
}
