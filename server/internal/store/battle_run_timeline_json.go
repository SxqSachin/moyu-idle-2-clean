package store

import (
	"bytes"
	"encoding/json"
	"errors"
)

// BattleRunTimelineJSON is JSON produced by encoding/json for a timeline.
// Its bytes stay private so the compiled-save path cannot accept
// unvalidated external JSON.
type BattleRunTimelineJSON struct {
	raw []byte
}

func MarshalBattleRunTimelineJSON(timeline []map[string]any) (BattleRunTimelineJSON, error) {
	raw, err := json.Marshal(timeline)
	if err != nil {
		return BattleRunTimelineJSON{}, err
	}
	return BattleRunTimelineJSON{raw: raw}, nil
}

func (value BattleRunTimelineJSON) bytesForSave() ([]byte, error) {
	isArray := len(value.raw) >= 2 && value.raw[0] == '[' && value.raw[len(value.raw)-1] == ']'
	if !isArray && !bytes.Equal(value.raw, []byte("null")) {
		return nil, errors.New("battle run timeline JSON is not initialized")
	}
	return value.raw, nil
}

// CloneBytes exposes a defensive copy for equivalence tests and diagnostics.
func (value BattleRunTimelineJSON) CloneBytes() []byte {
	return append([]byte(nil), value.raw...)
}
