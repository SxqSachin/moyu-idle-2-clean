package httpapi

import "testing"

func TestIsValidUsernameSupportsConfiguredScripts(t *testing.T) {
	tests := []struct {
		name     string
		username string
		valid    bool
	}{
		{name: "ascii", username: "Player_01", valid: true},
		{name: "chinese", username: "玩家01", valid: true},
		{name: "japanese", username: "テスト123", valid: true},
		{name: "korean", username: "한글_7", valid: true},
		{name: "mixed", username: "玩家テスト한글_01", valid: true},
		{name: "space", username: "玩家 01", valid: false},
		{name: "hyphen", username: "玩家-01", valid: false},
		{name: "emoji", username: "玩家😀", valid: false},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			if got := isValidUsername(tc.username); got != tc.valid {
				t.Fatalf("isValidUsername(%q)=%v, want %v", tc.username, got, tc.valid)
			}
		})
	}
}

func TestUsernameLengthUsesRunes(t *testing.T) {
	twoRune := "玩家"
	if got := usernameLength(twoRune); got != 2 {
		t.Fatalf("usernameLength(%q)=%d, want 2", twoRune, got)
	}

	maxRune := "玩玩玩玩玩玩玩玩玩玩玩玩玩玩玩玩玩玩玩玩玩玩玩玩"
	if got := usernameLength(maxRune); got != maxUsernameLength {
		t.Fatalf("usernameLength(maxRune)=%d, want %d", got, maxUsernameLength)
	}

	overMax := maxRune + "玩"
	if got := usernameLength(overMax); got != maxUsernameLength+1 {
		t.Fatalf("usernameLength(overMax)=%d, want %d", got, maxUsernameLength+1)
	}
}
