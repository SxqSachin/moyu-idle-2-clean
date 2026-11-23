package store

import (
	"context"
	"crypto/sha256"
	"fmt"
	"github.com/jackc/pgx/v5"
	"log"
	fishingeffect "mysecondwebgame/server/internal/operationeffect/fishing"
	"mysecondwebgame/server/internal/operationeffect/ratebonus"
	"strings"
	"time"
)

// Frozen offline executor before the September 8 working-set optimization.
func (batch *fishingFastForwardCatchBatch) persistReference(ctx context.Context, tx pgx.Tx, executor *fishingExecutor, userID int64, tripID string) (*FishingCatchView, error) {
	if batch == nil || !batch.Loaded {
		return nil, nil
	}
	if err := insertFishingCatchBatchTx(ctx, tx, userID, tripID, batch.Pending); err != nil {
		return nil, err
	}

	var lastCatch *FishingCatchView
	for _, pending := range batch.Pending {
		recordState := batch.Records[pending.Fish.SpeciesID]
		if recordState != nil {
			if pending.Record.Length {
				recordState.BestLengthCatchID = &pending.CatchID
			}
			if pending.Record.Weight {
				recordState.BestWeightCatchID = &pending.CatchID
			}
		}
		view := FishingCatchView{
			CatchID: pending.CatchID, TripID: tripID, SpeciesID: pending.Fish.SpeciesID,
			Fish: pending.Fish, Kept: pending.Kept, InventoryInstanceID: pending.InventoryInstanceID,
			CaughtAt: pending.CaughtAt, Record: pending.Record,
		}
		lastCatch = &view
		if pending.Scaleglow {
			if err := executor.store.LedgerGrantTemplateTx(ctx, tx, userID, "scaleglow_medium", 1, InventoryLedgerMeta{
				SourceSurface: "fishing", TargetSurface: "inventory",
				Details: map[string]any{"type": "fishing_scaleglow", "tripId": tripID, "catchId": pending.CatchID},
			}); err != nil {
				return nil, err
			}
		}
	}
	if err := batch.persistTournamentEntries(ctx, tx, executor.store, userID); err != nil {
		return nil, err
	}
	if err := batch.flushStateReference(ctx, tx, userID); err != nil {
		return nil, err
	}
	if len(batch.Pending) > 0 {
		if err := executor.store.pruneFishingCatchHistoryTx(ctx, tx, userID); err != nil {
			return nil, err
		}
	}
	return lastCatch, nil
}

func (e *fishingExecutor) settleFishingReferenceTx(ctx context.Context, tx pgx.Tx, userID int64, item QueueRuntimeItem, now time.Time, maxPhases int, budget FastForwardBudget, batchCatches bool) (SettleResult, error) {
	runtime, ok := parseFishingExecutorRuntime(item.PayloadRaw)
	runtime.effectSession = item.EffectSession
	if !ok || !fishingRuntimeCanSettle(runtime, item) {
		// CancelAfterSettle authorizes queue deletion, so close the persisted trip
		// and return supplies in this transaction before granting that permission.
		tripID, err := e.store.stopFishingTripForQueueTx(ctx, tx, userID, item.QueueID, fishingTripRuntimeInvalidStopReason, time.Now())
		if err != nil {
			return SettleResult{}, err
		}
		log.Printf("fishing runtime rejected user_id=%d queue_id=%d trip_id=%s parsed=%t payload_sha256=%x cleanup=pending_commit", userID, item.QueueID, tripID, ok, sha256.Sum256(item.PayloadRaw))
		return SettleResult{CancelAfterSettle: true, CancelReason: ActionQueueCancelReasonTargetInvalid}, nil
	}
	if maxPhases <= 0 {
		maxPhases = 1
	}
	if now.IsZero() {
		now = time.Now()
	}
	if err := e.ensureRulesSnapshot(&runtime); err != nil {
		return SettleResult{}, err
	}
	lastEventAt := time.Time{}
	attrExp := 0.0
	ownerExp := map[string]float64{}
	itemCosts := map[string]int64{}
	itemRewards := map[string]int64{}
	fishingCatches := map[string]FishingCatchTally{}
	var catchBatch *fishingFastForwardCatchBatch
	if batchCatches {
		catchBatch = newFishingFastForwardCatchBatch(runtime.Rules)
		catchBatch.TournamentDivision = runtime.TournamentDivision
		catchBatch.Loadout = runtime.Loadout
	}
	completed := false
	budgetStartedAt := budget.StartedAt
	if budgetStartedAt.IsZero() {
		budgetStartedAt = time.Now()
	}
	transitions := 0
	for transitions < maxPhases {
		if shouldYieldFishingFastForwardBudget(budgetStartedAt, budget, transitions) {
			break
		}
		phaseEnd := time.UnixMilli(runtime.PhaseEndsAtMS)
		if phaseEnd.After(now) {
			break
		}
		transitions++
		lastEventAt = phaseEnd
		beforeExp := attrExp
		beforePassive := runtime.CastPassive
		switch runtime.Phase {
		case fishingPhaseWaiting:
			if runtime.PlannedEndAtMS > 0 && phaseEnd.UnixMilli() >= runtime.PlannedEndAtMS {
				attrExp += 0.18
				runtime.Summary.StopReason = "time_reached"
				completed = true
				break
			}
			spot := runtime.Rules.Spot
			water := runtime.Rules.Water
			pressureElapsed := phaseEnd.Sub(time.UnixMilli(runtime.PhaseStartedAtMS))
			if pressureElapsed > 0 {
				runtime.Pressure = decayFishingPressure(runtime.Pressure, pressureElapsed)
			}
			conditions := fishingConditionsAt(water, spot, phaseEnd)
			rng := fishingRuntimeRNG(&runtime)
			targetLayer := effectiveFishingTargetLayer(runtime.TargetLayer)
			layerBiteMultiplier := fishingLayerBiteMultiplier(runtime.Rules.Species, spot, runtime.Loadout, conditions, targetLayer)
			biteChance := fishingBiteChance(spot, runtime.Loadout, effectiveFishingPace(runtime.Pace, runtime.Strategy), runtime.Pressure, conditions, layerBiteMultiplier)
			if runtime.CastEffects != nil {
				bonus, err := ratebonus.Resolve(*runtime.CastEffects, ratebonus.FishingBite)
				if err != nil {
					return SettleResult{}, err
				}
				biteChance = clampFloat(biteChance+bonus, 0, 0.97)
			}
			runtime.Pressure = clampFloat(runtime.Pressure+0.006, 0, 1)
			if rng.Float64() >= biteChance {
				attrExp += 0.18
				appendFishingTripEvent(&runtime, FishingTripEvent{
					Kind: "quiet", At: phaseEnd, CastNumber: runtime.Summary.Casts,
				})
				if ok, err := e.beginFishingCastTx(ctx, tx, userID, &runtime, phaseEnd); err != nil {
					return SettleResult{}, err
				} else if !ok {
					runtime.Summary.StopReason = "bait_empty"
					completed = true
				}
				break
			}
			species := selectFishingSpecies(runtime.Rules.Species, spot, runtime.Loadout, conditions, targetLayer, rng)
			if species == nil {
				attrExp += 0.18
				appendFishingTripEvent(&runtime, FishingTripEvent{
					Kind: "quiet", At: phaseEnd, CastNumber: runtime.Summary.Casts,
				})
				if ok, err := e.beginFishingCastTx(ctx, tx, userID, &runtime, phaseEnd); err != nil {
					return SettleResult{}, err
				} else if !ok {
					runtime.Summary.StopReason = "bait_empty"
					completed = true
				}
				break
			}
			fish := generateFishingIndividual(*species, runtime.WaterID, runtime.SpotID, runtime.Rules.RulesetVersion, phaseEnd, runtime.Pressure, rng)
			mutationSeed := fmt.Sprintf("%d:%d:%d:%d:%s", runtime.Seed, runtime.Summary.Casts, runtime.EventSeq, phaseEnd.UnixMilli(), fish.SpeciesID)
			fish.Genes.Mutations = rollFishingMutations(runtime.Rules.Mutations, mutationSeed, runtime.Loadout.MutationChanceMultiplier)
			runtime.CurrentFish = &fish
			runtime.CurrentSpeciesRarity = species.Rarity
			runtime.Summary.Bites++
			runtime.Pressure = clampFloat(runtime.Pressure+0.012, 0, 1)
			if catchBatch != nil {
				if err := catchBatch.ensureLoaded(ctx, tx, userID); err != nil {
					return SettleResult{}, err
				}
				catchBatch.recordEncounter(fish.SpeciesID)
			} else if err := e.recordFishingEncounterTx(ctx, tx, userID, fish.SpeciesID); err != nil {
				return SettleResult{}, err
			}
			runtime.Phase = fishingPhaseBite
			runtime.PhaseStartedAtMS = phaseEnd.UnixMilli()
			runtime.DurationMS = 3_000
			runtime.PhaseEndsAtMS = phaseEnd.Add(3 * time.Second).UnixMilli()
			appendFishingTripEvent(&runtime, FishingTripEvent{
				Kind: "bite", At: phaseEnd, CastNumber: runtime.Summary.Casts,
				SizeClass: fishingFishSizeClass(runtime.Rules, fish),
			})

		case fishingPhaseBite:
			if runtime.CurrentFish == nil {
				return SettleResult{}, ErrFishingArgsInvalid
			}
			species := runtime.Rules.speciesByID(runtime.CurrentFish.SpeciesID)
			if species == nil {
				return SettleResult{}, ErrFishingSpotNotFound
			}
			duration := fishingFightDuration(*species, *runtime.CurrentFish, runtime.Loadout, runtime.Strategy)
			runtime.Phase = fishingPhaseFighting
			runtime.PhaseStartedAtMS = phaseEnd.UnixMilli()
			runtime.DurationMS = duration.Milliseconds()
			runtime.PhaseEndsAtMS = phaseEnd.Add(duration).UnixMilli()
			runtime.CurrentLineLoad = roundFloat(fishingLineLoadForRuleset(*species, *runtime.CurrentFish, runtime.Strategy, runtime.Rules.RulesetVersion), 3)
			appendFishingTripEvent(&runtime, FishingTripEvent{
				Kind: "hooked", At: phaseEnd, CastNumber: runtime.Summary.Casts,
				SizeClass: fishingFishSizeClass(runtime.Rules, *runtime.CurrentFish),
			})

		case fishingPhaseFighting:
			if runtime.CurrentFish == nil {
				return SettleResult{}, ErrFishingArgsInvalid
			}
			species := runtime.Rules.speciesByID(runtime.CurrentFish.SpeciesID)
			if species == nil {
				return SettleResult{}, ErrFishingSpotNotFound
			}
			rng := fishingRuntimeRNG(&runtime)
			var modifiers fishingeffect.Modifiers
			if runtime.CastEffects != nil {
				var err error
				modifiers, err = fishingeffect.Resolve(*runtime.CastEffects)
				if err != nil {
					return SettleResult{}, err
				}
			}
			fight := resolveFishingFightWithEffects(*species, *runtime.CurrentFish, runtime.Loadout, runtime.Strategy, runtime.SafetyLoadPercent, runtime.Rules.RulesetVersion, modifiers, rng)
			runtime.CurrentLineLoad = fight.LineLoad
			if fight.LineBreak {
				runtime.Summary.LineBreaks++
				runtime.Summary.Escaped++
				appendFishingTripEvent(&runtime, FishingTripEvent{
					Kind: "line_break", Reason: fight.FailureReason, At: phaseEnd,
					CastNumber: runtime.Summary.Casts, SizeClass: fishingFishSizeClass(runtime.Rules, *runtime.CurrentFish),
				})
				for _, itemID := range fishingLineBreakCostIDs(runtime.Loadout) {
					itemCosts[itemID]++
				}
				consumeFishingTerminalSupplies(&runtime.Supplies, runtime.Loadout, true)
			} else {
				for itemID, quantity := range applyFishingTackleWear(&runtime.Supplies, runtime.Loadout, *species) {
					itemCosts[itemID] += quantity
				}
				if fight.Landed {
					var catchView FishingCatchView
					var record FishingRecordFlag
					var isNew, kept bool
					var gain float64
					var pendingCatch *fishingPendingCatch
					if catchBatch != nil {
						if err := catchBatch.ensureLoaded(ctx, tx, userID); err != nil {
							return SettleResult{}, err
						}
						pendingCatch = catchBatch.prepareCatch(*species, *runtime.CurrentFish, runtime.KeepRule, phaseEnd)
						if pendingCatch == nil {
							return SettleResult{}, ErrFishingArgsInvalid
						}
						record, isNew, kept, gain = pendingCatch.Record, pendingCatch.IsNew, pendingCatch.Kept, pendingCatch.Gain
					} else {
						var err error
						catchView, record, isNew, kept, gain, err = e.landFishingCatchTx(ctx, tx, userID, runtime.TripID, runtime.TournamentDivision, runtime.Loadout, *species, *runtime.CurrentFish, runtime.KeepRule, phaseEnd)
						if err != nil {
							return SettleResult{}, err
						}
					}
					runtime.Summary.Landed++
					if kept {
						runtime.Summary.Kept++
					} else {
						runtime.Summary.Released++
					}
					disposition := "released"
					if kept {
						disposition = "kept"
					}
					runtime.Summary.TotalWeight = roundFloat(runtime.Summary.TotalWeight+runtime.CurrentFish.WeightKG, 3)
					if catchBatch == nil {
						runtime.Summary.LastCatch = &catchView
					}
					addFishingCatchTally(fishingCatches, runtime.CurrentFish.SpeciesID, kept, runtime.CurrentFish.WeightKG, runtime.CurrentFish.Genes.Mutations)
					appendFishingTripEvent(&runtime, FishingTripEvent{
						Kind: "landed", At: phaseEnd, CastNumber: runtime.Summary.Casts,
						SizeClass: fishingFishSizeClass(runtime.Rules, *runtime.CurrentFish), SpeciesID: runtime.CurrentFish.SpeciesID,
						LengthCM: runtime.CurrentFish.LengthCM, WeightKG: runtime.CurrentFish.WeightKG,
						Mutations: append([]string(nil), runtime.CurrentFish.Genes.Mutations...), Disposition: disposition,
					})
					if isNew {
						runtime.Summary.NewSpecies = appendUniqueFishingString(runtime.Summary.NewSpecies, runtime.CurrentFish.SpeciesID)
					}
					if record.Length || record.Weight {
						runtime.Summary.Records = append(runtime.Summary.Records, record)
					}
					attrExp += gain
					if fishingScaleglowDrop(runtime.Seed, runtime.Summary.Landed) {
						if pendingCatch != nil {
							pendingCatch.Scaleglow = true
						} else {
							if err := e.store.LedgerGrantTemplateTx(ctx, tx, userID, "scaleglow_medium", 1, InventoryLedgerMeta{
								SourceSurface: "fishing", TargetSurface: "inventory",
								Details: map[string]any{"type": "fishing_scaleglow", "tripId": runtime.TripID, "catchId": catchView.CatchID},
							}); err != nil {
								return SettleResult{}, err
							}
						}
						runtime.Summary.ScaleglowFound++
						itemRewards["scaleglow_medium"]++
					}
				} else {
					runtime.Summary.Escaped++
					attrExp += 0.7
					appendFishingTripEvent(&runtime, FishingTripEvent{
						Kind: "escaped", Reason: fight.FailureReason, At: phaseEnd,
						CastNumber: runtime.Summary.Casts, SizeClass: fishingFishSizeClass(runtime.Rules, *runtime.CurrentFish),
					})
				}
			}
			runtime.CurrentFish = nil
			runtime.CurrentSpeciesRarity = ""
			runtime.Phase = fishingPhaseResting
			if fight.Landed {
				runtime.Phase = fishingPhaseLanded
			}
			runtime.PhaseStartedAtMS = phaseEnd.UnixMilli()
			runtime.DurationMS = 4_000
			runtime.PhaseEndsAtMS = phaseEnd.Add(4 * time.Second).UnixMilli()

		case fishingPhaseLanded, fishingPhaseResting:
			if runtime.PlannedEndAtMS > 0 && phaseEnd.UnixMilli() >= runtime.PlannedEndAtMS {
				runtime.Summary.StopReason = "time_reached"
				completed = true
				break
			}
			if ok, err := e.beginFishingCastTx(ctx, tx, userID, &runtime, phaseEnd); err != nil {
				return SettleResult{}, err
			} else if !ok {
				runtime.Summary.StopReason = "tackle_empty"
				completed = true
			}

		case fishingPhaseFinished:
			completed = true
		default:
			return SettleResult{}, ErrFishingArgsInvalid
		}
		if delta := attrExp - beforeExp; delta > 0 && beforePassive != nil && runtime.ActorType == ActionActorTypeKitty {
			gain, err := runtime.effectSession.Passive().Share(*beforePassive, map[string]float64{"fishing": roundFloat(delta, 4)})
			if err != nil {
				return SettleResult{}, err
			}
			ownerExp = mergeAttrExp(ownerExp, gain)
		}
		if completed || catchBatch.full() {
			break
		}
	}
	if catchBatch != nil {
		lastCatch, err := catchBatch.persistReference(ctx, tx, e, userID, runtime.TripID)
		if err != nil {
			return SettleResult{}, err
		}
		if lastCatch != nil {
			runtime.Summary.LastCatch = lastCatch
		}
	}
	if baitUsed := runtime.Summary.BaitUsed - runtime.ReportedBaitUsed; baitUsed > 0 {
		if baitID := strings.TrimSpace(runtime.Loadout.Components.BaitItemID); baitID != "" {
			itemCosts[baitID] += int64(baitUsed)
		}
	}
	runtime.ReportedBaitUsed = runtime.Summary.BaitUsed
	if len(ownerExp) > 0 {
		if err := e.store.applyAttributedExperienceTx(ctx, tx, userID, ownerExp, ownerExp, runtime.effectSession); err != nil {
			return SettleResult{}, err
		}
	}
	if attrExp > 0 {
		attrExp = roundFloat(attrExp, 4)
		if err := e.applyFishingAttrExpTx(ctx, tx, userID, runtime.ActorType, runtime.ActorID, attrExp); err != nil {
			return SettleResult{}, err
		}
		runtime.Summary.FishingExp = roundFloat(runtime.Summary.FishingExp+attrExp, 4)
	}
	if !lastEventAt.IsZero() {
		if err := e.store.saveFishingPressureTx(ctx, tx, userID, runtime.SpotID, runtime.Pressure, lastEventAt); err != nil {
			return SettleResult{}, err
		}
	}
	if completed {
		runtime.Phase = fishingPhaseFinished
		if lastEventAt.IsZero() {
			lastEventAt = now
		}
		appendFishingTripEvent(&runtime, FishingTripEvent{
			Kind: "finished", Reason: runtime.Summary.StopReason, At: lastEventAt,
		})
		runtime.PhaseStartedAtMS = lastEventAt.UnixMilli()
		runtime.PhaseEndsAtMS = lastEventAt.UnixMilli()
		runtime.DurationMS = 1_000
		if err := e.returnFishingSuppliesTx(ctx, tx, userID, runtime.Loadout, &runtime.Supplies); err != nil {
			return SettleResult{}, err
		}
		if err := e.saveFishingTripRuntimeTx(ctx, tx, userID, item.QueueID, runtime, true, lastEventAt); err != nil {
			return SettleResult{}, err
		}
		nextPayload, err := setFishingExecutorRuntime(item.PayloadRaw, runtime)
		if err != nil {
			return SettleResult{}, err
		}
		return SettleResult{
			Completed: 1, BudgetIterations: transitions, DoneAfter: item.TotalCount, CompletedAt: lastEventAt,
			NextRunStartAt: lastEventAt, NextRunEndsAt: lastEventAt.Add(time.Second),
			NextPayload: nextPayload, ItemCosts: itemCosts, ItemRewards: itemRewards,
			AttrExp: map[string]float64{"fishing": attrExp}, OwnerAttrExp: ownerExp, FishingCatches: fishingCatches,
		}, nil
	}
	if lastEventAt.IsZero() {
		return SettleResult{}, nil
	}
	if err := e.saveFishingTripRuntimeTx(ctx, tx, userID, item.QueueID, runtime, false, lastEventAt); err != nil {
		return SettleResult{}, err
	}
	nextPayload, err := setFishingExecutorRuntime(item.PayloadRaw, runtime)
	if err != nil {
		return SettleResult{}, err
	}
	return SettleResult{
		Progressed: true, BudgetIterations: transitions, CompletedAt: lastEventAt,
		NextRunStartAt: time.UnixMilli(runtime.PhaseStartedAtMS), NextRunEndsAt: time.UnixMilli(runtime.PhaseEndsAtMS),
		NextPayload: nextPayload, ItemCosts: itemCosts, ItemRewards: itemRewards,
		AttrExp: map[string]float64{"fishing": attrExp}, OwnerAttrExp: ownerExp, FishingCatches: fishingCatches,
	}, nil
}
