package store

import (
	"context"
	"encoding/json"
	"strings"

	"mysecondwebgame/server/internal/catalog"
	"mysecondwebgame/server/internal/config"

	"github.com/jackc/pgx/v5"
)

const publicFishingPondFishLimit = 200

const (
	FishingPondCapabilityFishInspect     = "fish.inspect"
	FishingPondCapabilityFishTrack       = "fish.track"
	FishingPondCapabilityWaterThrowStone = "water.throw_stone"
	FishingPondCapabilityWaterSkipStone  = "water.skip_stone"
)

// Capabilities are presentation hints only. Every data mutation must continue
// to authorize ownership from the authenticated actor on its own endpoint.
var publicFishingPondCapabilities = []string{
	FishingPondCapabilityFishInspect,
	FishingPondCapabilityFishTrack,
	FishingPondCapabilityWaterThrowStone,
	FishingPondCapabilityWaterSkipStone,
}

type FishingPondPublicLimits struct {
	FishCapacity    int64   `json:"fishCapacity"`
	NurseryCapacity int64   `json:"nurseryCapacity"`
	BiomassKG       float64 `json:"biomassKg"`
}

// FishingPondPublicFishData deliberately mirrors only player-visible traits.
// Persistence-only provenance, lineage IDs, timers, and death details never
// cross the public profile boundary.
type FishingPondPublicFishData struct {
	SpeciesID  string   `json:"speciesId"`
	AgeDays    int      `json:"ageDays"`
	Sex        string   `json:"sex"`
	LengthCM   float64  `json:"lengthCm"`
	WeightKG   float64  `json:"weightKg"`
	PondZone   string   `json:"pondZone"`
	LifeState  string   `json:"lifeState,omitempty"`
	Generation int      `json:"generation,omitempty"`
	Mutations  []string `json:"mutations,omitempty"`
}

type FishingPondPublicFishView struct {
	InstanceID       int64                         `json:"instanceId"`
	CustomData       ItemCustomization             `json:"customData,omitempty"`
	Fish             FishingPondPublicFishData     `json:"fish"`
	MaturityAgeDays  int                           `json:"maturityAgeDays"`
	Mature           bool                          `json:"mature"`
	MaturityProgress float64                       `json:"maturityProgress,omitempty"`
	Phenotypes       []string                      `json:"phenotypes,omitempty"`
	PhenotypeEffects []FishingPondPhenotypeEffect  `json:"phenotypeEffects,omitempty"`
	Predation        *catalog.FishingPondPredation `json:"predation,omitempty"`
}

type FishingPondPublicOverview struct {
	OwnerUserID    int64                            `json:"ownerUserId"`
	Built          bool                             `json:"built"`
	Level          int64                            `json:"level"`
	RulesetVersion string                           `json:"rulesetVersion"`
	Limits         FishingPondPublicLimits          `json:"limits"`
	Fish           []FishingPondPublicFishView      `json:"fish"`
	BiomassKG      float64                          `json:"biomassKg"`
	Capabilities   []string                         `json:"capabilities"`
	Mutations      []config.FishingMutationSettings `json:"mutations"`
}

// GetPublicFishingPond is intentionally side-effect free. In particular, it
// must not call the owner pond reader, because that path settles pond state.
func (s *Store) GetPublicFishingPond(ctx context.Context, userID int64) (FishingPondPublicOverview, error) {
	out := FishingPondPublicOverview{
		OwnerUserID:    userID,
		RulesetVersion: FishingPondRulesetVersion,
		Fish:           []FishingPondPublicFishView{},
		Capabilities:   append([]string(nil), publicFishingPondCapabilities...),
		Mutations:      s.FishingMutationCatalog(),
	}
	if userID <= 0 {
		return FishingPondPublicOverview{}, ErrUserNotFound
	}

	err := s.withReadOnlyTx(ctx, func(tx pgx.Tx) error {
		var exists bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM users WHERE id=$1)`, userID).Scan(&exists); err != nil {
			return err
		}
		if !exists {
			return ErrUserNotFound
		}

		level, err := s.getPlayerBuildingLevelTx(ctx, tx, userID, FishingPondBuildingID)
		if err != nil {
			return err
		}
		out.Level = level
		out.Built = level > 0
		if level <= 0 {
			return nil
		}

		limits := s.fishingPondLimits(level)
		out.Limits = FishingPondPublicLimits{
			FishCapacity:    limits.FishCapacity,
			NurseryCapacity: limits.NurseryCapacity,
			BiomassKG:       limits.BiomassKG,
		}

		rows, err := tx.Query(ctx, `
			SELECT instance_id, COALESCE(custom_data, '{}'::jsonb), COALESCE(fish_data, '{}'::jsonb)
			FROM player_item_instances
			WHERE user_id=$1
			  AND state=$2
			  AND template_item_id='fish_catch'
			  AND quantity=1
			  AND archived_at IS NULL
			  AND fish_data<>'{}'::jsonb
			ORDER BY instance_id
			LIMIT $3
		`, userID, fishingPondState, publicFishingPondFishLimit)
		if err != nil {
			return err
		}
		defer rows.Close()

		for rows.Next() {
			var instanceID int64
			var customRaw, fishRaw []byte
			if err := rows.Scan(&instanceID, &customRaw, &fishRaw); err != nil {
				return err
			}
			var customData ItemCustomization
			var fish FishInstanceData
			if json.Unmarshal(customRaw, &customData) != nil || json.Unmarshal(fishRaw, &fish) != nil || strings.TrimSpace(fish.SpeciesID) == "" {
				continue
			}
			view := s.publicFishingPondFishView(instanceID, customData, fish)
			out.Fish = append(out.Fish, view)
			if view.Fish.PondZone == fishingPondZoneMain && view.Fish.LifeState == "alive" {
				out.BiomassKG += view.Fish.WeightKG
			}
		}
		if err := rows.Err(); err != nil {
			return err
		}
		out.BiomassKG = roundFloat(out.BiomassKG, 3)
		return nil
	})
	return out, err
}

func (s *Store) publicFishingPondFishView(instanceID int64, customData ItemCustomization, fish FishInstanceData) FishingPondPublicFishView {
	s.clampFishingPondFishSize(&fish, s.gameData.FishingSpeciesByID(fish.SpeciesID))
	publicFish := FishingPondPublicFishData{
		SpeciesID:  strings.TrimSpace(fish.SpeciesID),
		AgeDays:    fish.AgeDays,
		Sex:        normalizedPondSex(fish.Sex, instanceID, fish.SpeciesID),
		LengthCM:   fish.LengthCM,
		WeightKG:   fish.WeightKG,
		PondZone:   normalizeFishingPondZone(fish.PondZone),
		LifeState:  normalizePondLifeState(fish.LifeState),
		Generation: fish.Generation,
		Mutations:  s.configuredFishMutationIDs(fish.Genes.Mutations),
	}

	view := FishingPondPublicFishView{
		InstanceID:       instanceID,
		CustomData:       customData,
		Fish:             publicFish,
		Phenotypes:       pondPhenotypes(fish.Genes),
		PhenotypeEffects: pondPhenotypeEffects(fish.Genes),
	}
	if species := s.gameData.FishingSpeciesByID(fish.SpeciesID); species != nil {
		view.MaturityAgeDays = species.MaturityAgeDays
		view.Mature = fish.AgeDays >= species.MaturityAgeDays
		view.MaturityProgress = pondMaturityProgress(fish, species.MaturityAgeDays)
	}
	if profile := s.gameData.FishingPondSpeciesByID(fish.SpeciesID); publicFish.PondZone == fishingPondZoneMain && profile != nil && profile.Predation != nil {
		predation := *profile.Predation
		view.Predation = &predation
	}
	return view
}

func pondMaturityProgress(fish FishInstanceData, maturityAgeDays int) float64 {
	if maturityAgeDays <= 0 {
		return 0
	}
	progress := (float64(fish.AgeDays) + fish.BioAgeFraction) / float64(maturityAgeDays)
	return roundFloat(clampFloat(progress, 0, 1), 4)
}
