package store

import "testing"

func TestIsValidItemCustomNameAllowsEmoji(t *testing.T) {
	tests := []string{
		"月影🌙",
		"hello✨",
		"剑🧙‍♂️",
		"护符👍🏽",
		"abc_123",
	}
	for _, name := range tests {
		if !IsValidItemCustomName(name) {
			t.Fatalf("IsValidItemCustomName(%q)=false, want true", name)
		}
	}
}

func TestIsValidItemCustomNameRejectsUnsafeCharacters(t *testing.T) {
	tests := []string{
		"",
		"hello world",
		"hello!",
		"<script>",
		"\u200d",
		"abc\u200d",
		"👨‍A",
	}
	for _, name := range tests {
		if IsValidItemCustomName(name) {
			t.Fatalf("IsValidItemCustomName(%q)=true, want false", name)
		}
	}
}

func TestItemRenameLabelColorsRejectUnsupportedValues(t *testing.T) {
	colors := normalizeItemRenameLabelColors([]string{"gold", "url(javascript:alert(1))", "sky", "gold", "bad class"})
	if len(colors) != 2 || colors[0] != "gold" || colors[1] != "sky" {
		t.Fatalf("normalizeItemRenameLabelColors returned %#v, want [gold sky]", colors)
	}
	if got := normalizeItemRenameLabelColors([]string{"bad"}); len(got) == 0 || got[0] != "gold" {
		t.Fatalf("normalizeItemRenameLabelColors fallback returned %#v, want gold first", got)
	}
}

func TestItemInstanceSupportsRenameKeepsGeneratedBooksButAllowsFish(t *testing.T) {
	if itemInstanceSupportsRename("forged_skillbook") {
		t.Fatal("itemInstanceSupportsRename(forged_skillbook)=true, want false")
	}
	for _, templateItem := range []string{"fish_catch", "wood_sword"} {
		if !itemInstanceSupportsRename(templateItem) {
			t.Fatalf("itemInstanceSupportsRename(%q)=false, want true", templateItem)
		}
	}
}

func TestItemRenameAllowedStatesIncludeFishingPond(t *testing.T) {
	for _, state := range []string{"inventory", "equipped", "auction", fishingPondState} {
		if _, ok := itemRenameAllowedStates[state]; !ok {
			t.Fatalf("itemRenameAllowedStates[%q] missing", state)
		}
	}
	if _, ok := itemRenameAllowedStates["processing"]; ok {
		t.Fatal("itemRenameAllowedStates unexpectedly allows processing")
	}
}
