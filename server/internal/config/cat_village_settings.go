package config

import (
	"encoding/json"
	"fmt"
	"os"
)

type CatVillageSettings struct {
	RepLevels []CatVillageRepLevel `json:"repLevels"`
}

type CatVillageRepLevel struct {
	Level      int   `json:"level"`
	XpRequired int64 `json:"xpRequired"`
}

func DefaultCatVillageSettings() CatVillageSettings {
	return CatVillageSettings{
		RepLevels: []CatVillageRepLevel{
			{Level: 1, XpRequired: 100},
			{Level: 2, XpRequired: 150},
			{Level: 3, XpRequired: 225},
			{Level: 4, XpRequired: 338},
			{Level: 5, XpRequired: 506},
			{Level: 6, XpRequired: 759},
			{Level: 7, XpRequired: 1139},
			{Level: 8, XpRequired: 1708},
			{Level: 9, XpRequired: 2563},
			{Level: 10, XpRequired: 3844},
		},
	}
}

func LoadCatVillageSettings(path string) (CatVillageSettings, error) {
	settings := DefaultCatVillageSettings()
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return settings, nil
		}
		return CatVillageSettings{}, fmt.Errorf("read cat village config %q: %w", path, err)
	}
	if err := json.Unmarshal(raw, &settings); err != nil {
		return CatVillageSettings{}, fmt.Errorf("parse cat village config %q: %w", path, err)
	}
	return settings, nil
}
