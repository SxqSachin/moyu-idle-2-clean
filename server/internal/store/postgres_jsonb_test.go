package store

import (
	"encoding/json"
	"errors"
	"testing"
)

func TestValidatePostgresJSONBRawRejectsDecodedNULOnly(t *testing.T) {
	t.Parallel()

	unsupported, err := json.Marshal(map[string]any{"counter\x00group": "value"})
	if err != nil {
		t.Fatal(err)
	}
	if err := validatePostgresJSONBRaw("carry_state", unsupported); !errors.Is(err, errPostgresJSONBContainsNUL) {
		t.Fatalf("decoded NUL validation error=%v", err)
	}

	literal, err := json.Marshal(map[string]any{"text": `\u0000`})
	if err != nil {
		t.Fatal(err)
	}
	if err := validatePostgresJSONBRaw("literal", literal); err != nil {
		t.Fatalf("literal Unicode escape text must remain valid: %v", err)
	}

	ordinary, err := json.Marshal(map[string]any{"text": "正常文本", "count": 1})
	if err != nil {
		t.Fatal(err)
	}
	if err := validatePostgresJSONBRaw("ordinary", ordinary); err != nil {
		t.Fatalf("ordinary JSON rejected: %v", err)
	}
}
