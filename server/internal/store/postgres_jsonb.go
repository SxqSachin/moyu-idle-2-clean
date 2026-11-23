package store

import (
	"bytes"
	"errors"
	"fmt"
)

var errPostgresJSONBContainsNUL = errors.New("PostgreSQL JSONB does not support U+0000")
var postgresJSONBNULEscape = []byte(`\u0000`)

// validatePostgresJSONBRaw distinguishes a decoded NUL escape from a literal
// "\\u0000" string. Callers pass bytes produced by encoding/json, so a fast
// substring check handles ordinary snapshots and the string scanner only runs
// when the escape is actually present.
func validatePostgresJSONBRaw(field string, raw []byte) error {
	if !bytes.Contains(raw, postgresJSONBNULEscape) {
		return nil
	}
	inString := false
	for index := 0; index < len(raw); index++ {
		switch raw[index] {
		case '"':
			inString = !inString
		case '\\':
			if !inString || index+1 >= len(raw) {
				continue
			}
			escape := raw[index+1]
			if escape == 'u' && index+5 < len(raw) &&
				raw[index+2] == '0' && raw[index+3] == '0' && raw[index+4] == '0' && raw[index+5] == '0' {
				return fmt.Errorf("%s: %w", field, errPostgresJSONBContainsNUL)
			}
			// Skip the escaped byte. In particular, this prevents the second
			// slash in "\\\\u0000" from being mistaken for a Unicode escape.
			index++
		}
	}
	return nil
}
