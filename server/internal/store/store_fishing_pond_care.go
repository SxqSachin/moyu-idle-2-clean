package store

import (
	"context"
	"math"
	"time"

	"github.com/jackc/pgx/v5"
	"mysecondwebgame/server/internal/catalog"
)

// One whole portion, independent of appetite. The revision prevents a stale
// confirmation or duplicate request from consuming another precious item.
func (s *Store) UseFishingPondTargetedFeed(ctx context.Context, userID, instanceID int64, itemID string, expectedRevision int64, now time.Time) (FishingPondOverview, error) {
	return s.mutateFishingPond(ctx, userID, now, func(tx pgx.Tx, _ int64, _ FishingPondLimits) error {
		return s.useFishingPondTargetedFeedTx(ctx, tx, userID, instanceID, itemID, expectedRevision, now)
	})
}

func (s *Store) useFishingPondTargetedFeedTx(ctx context.Context, tx pgx.Tx, userID, instanceID int64, itemID string, expectedRevision int64, now time.Time) error {
	if err := s.lockUserTx(ctx, tx, userID); err != nil {
		return err
	}
	fish, err := s.lockFishingPondFishTx(ctx, tx, userID, instanceID)
	if err != nil {
		return err
	}
	item := s.gameData.ItemByID(itemID)
	if item == nil || item.Disabled {
		return ErrFishingPondFeedInvalid
	}
	if profile := s.gameData.FishingPondSpeciesByID(fish.SpeciesID); profile != nil {
		s.initializeFishingPondFish(&fish, userID, instanceID, now, profile)
	}
	if err := s.applyFishingPondTargetedFeed(&fish, item.PondFeed, expectedRevision); err != nil {
		return err
	}
	if err := s.LedgerConsumeTemplateTx(ctx, tx, userID, itemID, 1, pondLedgerMeta("inventory", "fishing_pond", InventoryAuditDirectionConsume, "pond_targeted_feed")); err != nil {
		return err
	}
	if err := s.updateFishingPondFishTx(ctx, tx, userID, instanceID, fish, now); err != nil {
		return err
	}
	return s.addFishingPondEventTx(ctx, tx, userID, "targeted_fed", &instanceID, nil, map[string]any{"itemId": itemID, "quantity": 1})
}

func (s *Store) applyFishingPondTargetedFeed(fish *FishInstanceData, feed *catalog.PondFeedConfig, expectedRevision int64) error {
	if fish == nil || normalizePondLifeState(fish.LifeState) != "alive" || fish.Health <= 0 || expectedRevision < 0 || expectedRevision != fish.PondCareRevision || expectedRevision == math.MaxInt64 || feed == nil || feed.Delivery != "targeted" || feed.RulesetVersion != pondFeedRulesetVersion || len(feed.Effects) == 0 {
		return ErrFishingPondFeedInvalid
	}
	species, profile := s.gameData.FishingSpeciesByID(fish.SpeciesID), s.gameData.FishingPondSpeciesByID(fish.SpeciesID)
	if species == nil || profile == nil || !pondFeedConditionsMatch(*fish, profile, feed) || pondFeedPayableDose(*fish, feed, 1) < 1 {
		return ErrFishingPondFeedInvalid
	}
	after := *fish
	s.applyFishingPondFeedCompound(&after, species, profile, feed, 1)
	s.clampFishingPondFishSize(&after, species)
	for _, value := range []float64{after.LongevityLimitDays, after.NaturalProtectionUntilAgeDays, after.Health, after.Satiety, after.LengthCM, after.WeightKG} {
		if math.IsNaN(value) || math.IsInf(value, 0) {
			return ErrFishingPondFeedInvalid
		}
	}
	if after.LongevityLimitDays == fish.LongevityLimitDays && after.NaturalProtectionUntilAgeDays == fish.NaturalProtectionUntilAgeDays && after.Health <= fish.Health && after.Satiety <= fish.Satiety && after.LengthCM <= fish.LengthCM && after.WeightKG <= fish.WeightKG {
		return ErrFishingPondFeedInvalid
	}
	after.PondCareRevision++
	*fish = after
	return nil
}
