package store

import (
	"context"
	"github.com/jackc/pgx/v5"
	"time"
)

// Frozen pre-optimization planner for exact outcome and performance comparison.
func (e *enhanceExecutor) planEnhanceFastForwardReferenceTx(ctx context.Context, tx pgx.Tx, userID int64, item QueueRuntimeItem, runtime enhanceExecutorRuntime, locked LockedItemInstance, completed int, firstRunCtx enhanceRunContext, firstRunCtxOK bool, budgetStartedAt time.Time, budget FastForwardBudget) (enhanceFastForwardPlan, error) {
	plan := enhanceFastForwardPlan{
		FinalLevel:     locked.EnhanceLevel,
		ItemCosts:      map[string]int64{},
		ChargeCosts:    map[string]int64{},
		AttrExpGain:    map[string]float64{},
		EnhanceResults: make([]EnhanceAttemptResult, 0, completed),
	}
	if completed <= 0 {
		return plan, nil
	}
	successRateBonus, err := e.enhancementSuccessRateBonusTx(ctx, tx, userID)
	if err != nil {
		return enhanceFastForwardPlan{}, err
	}
	ownedByItem, err := e.lockEnhanceCostInventoryTx(ctx, tx, userID, runtime, firstRunCtx, firstRunCtxOK)
	if err != nil {
		return enhanceFastForwardPlan{}, err
	}
	currentLevel := locked.EnhanceLevel
	for plan.AttemptsDone < completed {
		prepaid := plan.AttemptsDone == 0 && firstRunCtxOK
		var runCtx enhanceRunContext
		if prepaid {
			runCtx = normalizeEnhanceRunContext(firstRunCtx)
		} else {
			prepared, reason, preparedOK := e.buildEnhanceRunContext(runtime, currentLevel, successRateBonus)
			if !preparedOK {
				plan.CancelAfterSettle = true
				plan.StopReason = reason
				break
			}
			if ok, reason, err := reserveEnhanceLocalCosts(ownedByItem, prepared.Costs, runtime.ProtectItemID, prepared.UsedProtect); err != nil {
				return enhanceFastForwardPlan{}, err
			} else if !ok {
				plan.CancelAfterSettle = true
				plan.StopReason = reason
				break
			}
			if err := mergeEnhanceCosts(plan.ChargeCosts, prepared.Costs); err != nil {
				return enhanceFastForwardPlan{}, err
			}
			runCtx = prepared
			// The first legacy run has no start snapshot. Only subsequent real
			// starts may acquire effects during fast-forward.
			if plan.AttemptsDone > 0 {
				timing := resolveQueueRunTiming(item, time.Time{}, time.Duration(runtime.DurationMs)*time.Millisecond)
				snapshot, err := nextEnhanceOperationEffects(item, timing, time.Time{}, plan.AttemptsDone)
				if err != nil {
					return enhanceFastForwardPlan{}, err
				}
				if err := applyEnhanceOperationEffects(&runCtx, snapshot); err != nil {
					return enhanceFastForwardPlan{}, err
				}
			}
		}
		if err := mergeEnhanceCosts(plan.ItemCosts, runCtx.Costs); err != nil {
			return enhanceFastForwardPlan{}, err
		}

		beforeLevel := currentLevel
		success := enhanceAttemptRoll(userID, item.QueueID, item.DoneCount+plan.AttemptsDone, runCtx.SuccessRate)
		if success {
			currentLevel = clampEnhanceLevel(beforeLevel+1, runtime.MaxLevel)
		} else if runCtx.UsedProtect {
			currentLevel = clampEnhanceLevel(beforeLevel-1, runtime.MaxLevel)
		} else {
			currentLevel = 0
		}
		attemptNo := item.DoneCount + plan.AttemptsDone + 1
		plan.EnhanceResults = append(plan.EnhanceResults, EnhanceAttemptResult{
			QueueID:             item.QueueID,
			ActionID:            item.ActionID,
			AttemptNo:           attemptNo,
			EquipmentInstanceID: runtime.EquipmentInstanceID,
			TemplateItemID:      locked.TemplateItem,
			BeforeLevel:         beforeLevel,
			AfterLevel:          currentLevel,
			Success:             success,
			UsedProtect:         runCtx.UsedProtect,
			BaseSuccessRate:     runCtx.BaseSuccessRate,
			SuccessRateBonus:    runCtx.SuccessRateBonus,
			SuccessRate:         runCtx.SuccessRate,
			Costs:               cloneCostMap(runCtx.Costs),
			DeltaAttrs:          cloneAttrMap(runCtx.DeltaAttrs),
		})
		if runtime.EnhanceExp > 0 {
			plan.AttrExpGain["enhance"] = roundFloat(plan.AttrExpGain["enhance"]+runtime.EnhanceExp, 4)
		}
		plan.AttemptsDone++
		plan.FinalLevel = currentLevel
		if success {
			if _, exists := e.store.enhanceCache.AffixMilestonePool(runtime.TemplateID, currentLevel); exists {
				plan.MilestoneAttemptNo = attemptNo
				break
			}
		}
		if currentLevel >= runtime.TargetEnhanceLevel || currentLevel >= runtime.MaxLevel {
			plan.CancelAfterSettle = true
			if currentLevel >= runtime.MaxLevel {
				plan.StopReason = EnhanceStopReasonMaxLevelReached
			} else {
				plan.StopReason = EnhanceStopReasonTargetReached
			}
			break
		}
		if budget.MaxExec > 0 && time.Since(budgetStartedAt) >= budget.MaxExec {
			break
		}
	}
	return plan, nil
}
