<template>
  <GameDialog
    v-model="visible"
    width="380px"
    destroy-on-close
    class="action-detail-dialog"
  >
    <div v-if="action" class="text-sm">
      <div class="space-y-4">
      <div class="flex items-center justify-center gap-2 text-center">
        <span>{{ actionLabel(action) }}</span>
      </div>

      <div v-if="actionDescription(action)" class="text-gray-500 text-xs text-center">
        {{ actionDescription(action) }}
      </div>

      </div>

      <el-divider></el-divider>
      <el-tabs v-model="activeDetailTab" class="action-detail-tabs" data-tutorial="action-detail-tabs">
        <el-tab-pane :label="t('game.actionExecutionSettings')" name="execution">
          <OperationEffectNotice v-if="isPrimaryWork(action.mainType)" :context="workMealContext" />
          <div class="grid grid-cols-1 gap-2" data-tutorial="action-detail-overview">

          <BaseCard shadow="never">
            <div class="text-xs font-semibold uppercase tracking-wide app-muted">
              <div>{{ t("game.actionDuration") }}: {{ formatDurationSecondsOneDecimal(perRunDurationSeconds, t) }}</div>
              <div v-if="furnitureEfficiency(action.mainType || '', action.subType || '', action.id) > 0">{{ t('game.furniture.efficiencyLabel') }} +{{ Number((furnitureEfficiency(action.mainType || '', action.subType || '', action.id) * 100).toFixed(2)) }}%</div>
              <FoodValueHint v-if="workMealModifiers.durationPenaltyRate > 0">{{ t('game.fishCuisine.workDuration', { percent: workMealModifiers.durationPenaltyRate * 100 }) }}</FoodValueHint>
              <FoodValueHint v-if="workMealModifiers.durationReductionRate > 0">{{ basePerRunDurationSeconds <= 2 ? t('game.fishCuisine.workSpeedFloor') : t('game.fishCuisine.workSpeed', { percent: Number((workMealModifiers.durationReductionRate * 100).toFixed(2)) }) }}</FoodValueHint>
            </div>
          </BaseCard>

        <BaseCard shadow="never" v-if="statusRequireEntries(action.statusRequirement).length">
          <div class="text-xs font-semibold uppercase tracking-wide app-muted">
            {{ t("game.statusRequirements") }}:
          </div>
          <div v-if="statusRequireEntries(action.statusRequirement).length" class="flex flex-wrap gap-2">
            <StatusRequireTag
              v-for="entry in statusRequireEntries(action.statusRequirement)"
              :key="`status-require-${action.id}-${entry.attrKey}`"
              :attr-key="entry.attrKey"
              :required-value="entry.required"
            />
          </div>
        </BaseCard>

        <BaseCard shadow="never" v-if="buildingRequirementEntries(action.buildingRequirement).length">
          <div class="text-xs font-semibold uppercase tracking-wide app-muted">
            {{ t("game.buildingRequirements") }}:
          </div>
          <div class="flex flex-wrap gap-2">
            <BuildingQuickRef
              v-for="entry in buildingRequirementEntries(action.buildingRequirement)"
              :key="`building-require-${action.id}-${entry.buildingId}`"
              :building-id="entry.buildingId"
              :required-level="entry.required"
              :current-level="entry.current"
            />
          </div>
        </BaseCard>

        <BaseCard class="action-detail-costs" data-tutorial="action-detail-costs" shadow="never" v-if="itemEntries(action.costs).length">
          <div class="text-xs font-semibold uppercase tracking-wide app-muted">
            {{ t("game.actionRequirements") }}:
          </div>
          <div v-if="itemEntries(action.costs).length" class="flex flex-row gap-2 flex-wrap">
            <div
              v-for="entry in itemEntries(action.costs)"
              :key="`cost-${action.id}-${entry.itemId}`"
              class="flex flex-col items-start gap-1"
            >
              <QuickRefWithRequire :item-id="entry.itemId" :required-count="entry.quantity * count">
              </QuickRefWithRequire >
              <div v-if="materialRequirementLabels(entry.itemId).length" class="flex flex-wrap gap-1">
                <el-tag
                  v-for="label in materialRequirementLabels(entry.itemId)"
                  :key="`material-requirement-${entry.itemId}-${label}`"
                  size="small"
                  type="warning"
                  effect="plain"
                >
                  {{ label }}
                </el-tag>
              </div>
            </div>
          </div>
        </BaseCard>

        <BaseCard v-if="fishFoodRecipe" shadow="never" class="fish-food-ingredients">
          <div class="flex flex-wrap items-center justify-between gap-3">
            <div class="text-xs font-semibold uppercase tracking-wide app-muted">
              {{ t("game.fishFood.ingredientsTitle") }}:
            </div>
            <label
              v-if="fishFoodRecipe.allowSpecificIngredients"
              class="inline-flex min-w-0 items-center gap-2 text-xs app-muted"
            >
              <span>{{ t("game.fishFood.useSpecific") }}</span>
              <el-switch
                v-model="useSpecificFishIngredients"
                :aria-label="t('game.fishFood.useSpecific')"
                @change="handleSpecificFishModeChange"
              />
            </label>
          </div>

          <div v-if="!useSpecificFishIngredients" class="mt-2 space-y-2">
            <div
              v-for="ingredient in fishFoodRecipe.fishIngredients"
              :key="`fish-requirement-${ingredient.speciesId}`"
              class="flex items-center gap-2"
            >
              <img
                v-if="fishSpeciesIcon(ingredient.speciesId)"
                :src="fishSpeciesIcon(ingredient.speciesId)"
                :alt="fishSpeciesLabel(ingredient.speciesId)"
                class="h-8 w-11 shrink-0 object-contain"
              />
              <div class="min-w-0 flex-1 text-xs font-medium">
                {{ fishSpeciesLabel(ingredient.speciesId) }} × {{ ingredient.quantity * count }}
              </div>
              <span
                class="text-xs tabular-nums"
                :class="fishOwnedCount(ingredient.speciesId) >= ingredient.quantity * count ? 'text-[var(--el-color-success)]' : 'text-[var(--el-color-danger)]'"
              >
                {{ t("game.fishFood.owned", { count: fishOwnedCount(ingredient.speciesId) }) }}
              </span>
            </div>
            <p class="text-xs app-muted leading-5">{{ t("game.fishFood.automaticHint") }}</p>
          </div>

          <div v-else class="mt-2 space-y-2">
            <div
              v-for="slot in fishFoodIngredientSlots"
              :key="slot.key"
              class="flex min-h-14 items-center gap-2 border-b border-[var(--app-border)] py-2 last:border-b-0"
            >
              <FishMutationArt
                v-if="selectedFishForSlot(slot.key)?.fishData && fishSpeciesIcon(slot.speciesId)"
                :src="fishSpeciesIcon(slot.speciesId)"
                :alt="fishSpeciesLabel(slot.speciesId)"
                :fish="selectedFishForSlot(slot.key)!.fishData"
                width="48px"
              />
              <img
                v-else-if="fishSpeciesIcon(slot.speciesId)"
                :src="fishSpeciesIcon(slot.speciesId)"
                :alt="fishSpeciesLabel(slot.speciesId)"
                class="h-9 w-12 shrink-0 object-contain opacity-70"
              />
              <div class="min-w-0 flex-1">
                <div class="truncate text-xs font-semibold">{{ fishSpeciesLabel(slot.speciesId) }}</div>
                <div v-if="selectedFishForSlot(slot.key)?.fishData" class="mt-0.5 text-xs app-muted tabular-nums">
                  {{ formatSelectedFish(selectedFishForSlot(slot.key)!) }}
                </div>
                <div v-else class="mt-0.5 text-xs app-muted">{{ t("game.fishFood.ingredientUnselected") }}</div>
              </div>
              <el-button size="small" plain @click="openFishSelector(slot.key)">
                {{ selectedFishForSlot(slot.key) ? t("game.fishFood.replaceIngredient") : t("game.fishFood.selectIngredient") }}
              </el-button>
            </div>
            <p class="text-xs app-muted leading-5">{{ t("game.fishFood.specificHint") }}</p>
          </div>

          <div class="mt-3 border-t border-[var(--app-border)] pt-3">
            <div class="flex flex-wrap items-center justify-between gap-2 text-xs">
              <span class="font-semibold">{{ t("game.fishFood.effectPreview") }}</span>
              <div v-if="fishFoodQualityPreview" class="ml-auto inline-flex items-center gap-1 text-[var(--el-color-primary)]">
                <span class="font-semibold tabular-nums">
                  {{ t("game.fishCuisine.specialMultiplier", { value: (fishFoodQualityPreview.specialMultiplier || 1).toFixed(4) }) }}
                </span>
                <el-popover
                  placement="bottom-end"
                  :trigger="!isMobile && supportsHover ? ['hover', 'focus'] : 'click'"
                  :width="320"
                  :popper-style="{ maxWidth: 'calc(100vw - 24px)' }"
                  :show-after="120"
                  popper-class="fish-food-quality-popper"
                >
                  <template #reference>
                    <button
                      type="button"
                      class="inline-flex h-6 w-6 shrink-0 items-center justify-center rounded-full hover:bg-[var(--el-color-primary-light-9)] focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-[var(--el-color-primary)]"
                      :aria-label="t('game.fishCuisine.qualityDetails')"
                    >
                      <Icon icon="lucide:circle-help" :width="15" :height="15" aria-hidden="true" />
                    </button>
                  </template>
                  <FishFoodQualityBreakdown :quality="fishFoodQualityPreview" :scales="fishFoodRecipe.parameterScaling" compact />
                </el-popover>
              </div>
              <span v-else-if="!useSpecificFishIngredients" class="app-muted">{{ t("game.fishFood.defaultQuality") }}</span>
            </div>
            <OperationEffectSummary v-if="fishFoodEffectLines.length" class="mt-2" :lines="fishFoodEffectLines" />
            <div v-else class="mt-2 text-xs text-[var(--el-color-warning)]">{{ t("game.fishFood.previewIncomplete") }}</div>
            <div v-if="fishFoodHintText" class="mt-2 text-xs text-[var(--el-color-warning)]">
              {{ fishFoodHintText }}
            </div>
          </div>
        </BaseCard>

        <BaseCard shadow="never" v-if="bindableCostEntries.length">
          <div class="text-xs font-semibold uppercase tracking-wide app-muted">
            {{ t("game.actionUseSpecificItems") }}:
          </div>
          <div class="text-xs app-muted mt-1 mb-2">
            {{ t(hasMaterialInstanceRequirements ? "game.actionUseSpecificItemsHint" : "game.actionUseSpecificItemsHintUnrestricted") }}
          </div>
          <div class="space-y-2">
            <div
              v-for="entry in bindableCostEntries"
              :key="`binding-${action.id}-${entry.itemId}`"
              class="flex items-center gap-2"
            >
              <span class="text-xs app-muted w-8 shrink-0">{{ bindableItemLabel(entry.itemId) }}</span>
              <Card
                v-if="selectedMaterialEntry(entry.itemId)"
                :item-id="selectedMaterialEntry(entry.itemId)!.templateItemId"
                :instance-id="selectedMaterialEntry(entry.itemId)!.instanceId"
                :quantity="selectedMaterialEntry(entry.itemId)!.quantity"
                @click="openMaterialSelector(entry.itemId)"
              />
              <EmptyCard v-else :label="t('game.inventorySelectorChoose')" @click="openMaterialSelector(entry.itemId)" />
            </div>
            <div v-if="inheritancePreviewText" class="text-xs app-muted">
              {{ inheritancePreviewText }}
            </div>
            <div v-if="materialBindingHintText" class="text-xs text-[var(--el-color-warning)]">
              {{ materialBindingHintText }}
            </div>
          </div>
        </BaseCard>

        <BaseCard v-if="attrExpEntries(action.statusExpReward).length" shadow="never">
          <div class="text-xs font-semibold uppercase tracking-wide app-muted">
            {{ t("game.actionOutputAttrExp") }}:
          </div>
          <div class="space-y-1 app-muted">
            <template v-for="entry in attrExpEntries(action.statusExpReward)">
              <StatusTag :attr-key="entry.attrKey" :value="entry.amount * count" />
              <FoodValueHint v-if="entry.attrKey === action.mainType && workMealModifiers.experienceBonusRate > 0">{{ t('game.fishCuisine.workExperience', { percent: Number((workMealModifiers.experienceBonusRate * 100).toFixed(2)) }) }}</FoodValueHint>
            </template>
          </div>
        </BaseCard>

        <BaseCard class="action-detail-rewards" data-tutorial="action-detail-rewards" v-if="rewardItemEntries(action.itemReward).length" shadow="never">
          <div class="text-xs font-semibold uppercase tracking-wide app-muted">
            {{ t("game.actionOutputItemRewards") }}:
          </div>
          <div class="space-y-1 app-muted">
              <div
                v-for="entry in rewardItemEntries(action.itemReward)"
                :key="`reward-${action.id}-${entry.itemId}`"
                class="flex flex-wrap items-center gap-2"
              >
                <QuickRef :item-id="entry.itemId">
                  <span>: {{ entry.quantity }} </span>
                  <span> ({{ entry.percent }}%) </span>
                </QuickRef>
                <FoodValueHint v-if="entry.itemId === action.mainOutput && workMealModifiers.chance > 0">{{ t('game.fishCuisine.workHarvest', { percent: Number((workMealModifiers.chance * 100).toFixed(2)) }) }}</FoodValueHint>
              </div>
          </div>
          </BaseCard>
          </div>

          <el-divider></el-divider>

          <div class="flex flex-col gap-2">
        <div class="w-full flex items-center space-x-2 mb-2" data-tutorial="action-detail-count">
          <span class="text-sm w-12 shrink-0 text-gray-700 dark:text-gray-300">{{t('game.actionExecution')}}: </span>
          <el-input-number class="w-full!" :model-value="count" :min="1" :max="useSpecificFishIngredients ? 1 : MAX_ACTION_COUNT" @update:model-value="onCountChange" />
          <span class="text-sm text-gray-700 dark:text-gray-300">{{ t('game.times') }}</span>
        </div>
        <div class="w-full flex items-center space-x-2 mb-2">
          <span class="text-sm w-12 shrink-0 text-gray-700 dark:text-gray-300">{{ t('game.actionExecutor') }}:</span>
          <el-select v-model="selectedActorKey" class="w-full">
            <el-option
              v-for="option in actorOptions"
              :key="option.key"
              :label="option.label"
              :value="option.key"
            >
              <div class="flex items-center gap-1">
                <span>{{ option.label }}</span>
                <el-tooltip v-if="option.actorType === 'kitty'" :content="kittyExecutorHint" placement="top">
                  <span class="cursor-help text-[var(--app-muted)]">ⓘ</span>
                </el-tooltip>
              </div>
            </el-option>
          </el-select>
        </div>

        <div class="text-sm text-gray-500 dark:text-gray-400">{{ t('game.actionExecutionQA') }}:</div>

        <div class="grid grid-cols-3">
          <el-button size="small" :disabled="useSpecificFishIngredients" @click="appendActionCount(10)">+10</el-button>
          <el-button size="small" :disabled="useSpecificFishIngredients" @click="appendActionCount(100)">+100</el-button>
          <el-button size="small" :disabled="useSpecificFishIngredients" @click="appendActionCount(1000)">+1000</el-button>
        </div>
        <div class="grid grid-cols-4">
          <el-button size="small" :disabled="useSpecificFishIngredients" @click="appendActionCountByDuration(10 * 60)">{{ t("game.actionQuickAdd10m") }}</el-button>
          <el-button size="small" :disabled="useSpecificFishIngredients" @click="appendActionCountByDuration(30 * 60)">{{ t("game.actionQuickAdd30m") }}</el-button>
          <el-button size="small" :disabled="useSpecificFishIngredients" @click="appendActionCountByDuration(60 * 60)">{{ t("game.actionQuickAdd1h") }}</el-button>
          <el-button size="small" :disabled="useSpecificFishIngredients" @click="appendActionCountByDuration(8 * 60 * 60)">{{ t("game.actionQuickAdd8h") }}</el-button>
        </div>

        <div v-if="count > 0"
          class="text-xs text-center text-gray-600 dark:text-gray-300 bg-gray-100 dark:bg-gray-700 rounded-lg p-3 border border-gray-200 dark:border-gray-600">
          <span class="font-medium">{{t('game.actionExecutionEstimatedTime')}}：</span>
          <span class="text-blue-600 dark:text-blue-400 font-bold">
            {{ estimatedDurationText }}
          </span>
        </div>

            <div class="flex items-center gap-2 mt-2" data-tutorial="action-submit-buttons">
              <el-button class="w-full action-detail-run-now" data-tutorial="action-detail-run-now" type="primary" :disabled="!canSubmitAction" @click="emitRunNow(action.id)">
                {{ t("game.actionRunNow") }}
              </el-button>
              <el-button class="w-full action-detail-enqueue" :disabled="!canAddToQueue" @click="emitEnqueue(action.id)">{{ t("game.actionEnqueue") }}</el-button>
            </div>
          </div>
        </el-tab-pane>
        <el-tab-pane class="action-detail-quick-arrange-pane" :label="t('game.actionQuickArrangeTab')" name="quick-arrange">
          <div class="space-y-2">
            <BaseCard shadow="never">
              <div class="text-xs app-muted">{{ t("game.actionQuickArrangeSummaryTitle") }}</div>
              <div class="mt-1 flex items-center gap-2 flex-wrap">
                <ActionQuickRef :action-id="action.id" />
                <span class="text-xs font-semibold">x{{ count }}</span>
              </div>
              <div class="text-xs app-muted mt-1">
                {{ t("game.actionQuickArrangeDuration", { duration: formatDurationI18n(quickPlan.totalDurationS || 0, t) }) }}
              </div>
              <div class="text-xs app-muted mt-1 flex items-center gap-2 flex-wrap">
                {{ t("game.actionQuickArrangeSteps", { count: quickPlan.actionSteps.length }) }}
                <el-button size="small" type="primary" @click="workPlanVisible = true" class="ml-0!">
                  {{ t("game.actionQuickArrangeShowPlan") }}
                </el-button>
                <el-button size="small" type="primary" @click="planActionsVisible = true" class="ml-0!">
                  {{ t("game.actionQuickArrangeShowActions") }}
                </el-button>
                <el-button
                  v-if="quickBlockedActions.length"
                  size="small"
                  type="warning"
                  @click="blockedActionsVisible = true"
                  class="ml-0!"
                >
                  {{ t("game.actionQuickArrangeShowBlocked") }}
                </el-button>
              </div>
            </BaseCard>
            <BaseCard shadow="never" v-if="quickPlan.hasCycle">
              <div class="text-xs text-[var(--el-color-danger)]">
                {{ t("game.actionQuickArrangeCycleDetected") }}
              </div>
            </BaseCard>
            <BaseCard shadow="never" v-if="quickPlan.unresolvedMaterials.length">
              <div class="text-xs font-semibold uppercase tracking-wide app-muted">{{ t("game.actionQuickArrangeUnresolved") }}:</div>
              <div class="flex flex-wrap gap-2 mt-1">
                <div
                  v-for="entry in quickPlan.unresolvedMaterials"
                  :key="`unresolved-${entry.itemId}`"
                  class="inline-flex items-center gap-2 rounded border border-[var(--el-color-danger-light-7)] px-2 py-1"
                >
                  <QuickRefWithRequire :item-id="entry.itemId" :required-count="entry.deficit" />
                </div>
              </div>
            </BaseCard>
            <BaseCard shadow="never">
              <div class="w-full flex items-center space-x-2 mb-2">
                <span class="text-sm w-16 shrink-0">{{ t("game.actionExecution") }}:</span>
                <el-input-number class="w-full!" :model-value="count" :min="1" @update:model-value="onCountChange" />
                <span class="text-sm">{{ t("game.times") }}</span>
              </div>
              <div class="grid grid-cols-3 gap-2">
                <el-button size="small" @click="appendActionCount(10)">+10</el-button>
                <el-button size="small" @click="appendActionCount(100)">+100</el-button>
                <el-button size="small" @click="appendActionCount(1000)">+1000</el-button>
              </div>
            </BaseCard>
            <BaseCard shadow="never">
              <div class="w-full flex items-center space-x-2">
                <span class="text-sm w-16 shrink-0">{{ t("game.actionExecutor") }}:</span>
                <el-select v-model="selectedActorKey" class="w-full">
                  <el-option
                    v-for="option in actorOptions"
                    :key="`quick-${option.key}`"
                    :label="option.label"
                    :value="option.key"
                  />
                </el-select>
              </div>
            </BaseCard>
            <div class="grid grid-cols-2 gap-2">
              <el-button
                class="w-full"
                type="primary"
                :disabled="!canEnqueueQuickPlan"
                @click="emitEnqueuePlan('run_now_like_prepend')"
              >
                {{ t("game.actionQuickArrangeStartNow") }}
              </el-button>
              <el-button
                class="w-full ml-0!"
                type="primary"
                :disabled="!canEnqueueQuickPlan"
                @click="emitEnqueuePlan('append')"
              >
                {{ t("game.actionQuickArrangeEnqueue") }}
              </el-button>
            </div>
          </div>
        </el-tab-pane>
      </el-tabs>
    </div>
    <InventoryInstanceSelectorDialog
      v-model="materialSelectorVisible"
      :title="materialSelectorTitle"
      :instance-ids="activeMaterialSelectorInstanceIds"
      :selected-instance-id="activeMaterialSelectedInstanceId"
      :allow-clear="true"
      :template-item-ids="activeMaterialSelectorTemplateItemIds"
      :template-items-title="t('game.inventorySelectorAvailableItemsTitle', { title: materialSelectorTitle })"
      :empty-text="t('game.inventorySelectorNoItems')"
      @confirm="handleMaterialSelectorConfirm"
    />
    <InventoryInstanceSelectorDialog
      v-model="fishSelectorVisible"
      :title="fishSelectorTitle"
      :instance-ids="activeFishSelectorInstanceIds"
      :selected-instance-id="activeFishSelectedInstanceId"
      :allow-clear="true"
      :empty-text="t('game.fishFood.noEligibleFish')"
      @confirm="handleFishSelectorConfirm"
    />
    <GameDialog
      v-model="workPlanVisible"
      width="820px"
      destroy-on-close
      class="action-work-plan-dialog"
    >
      <template #header>
        <div class="font-semibold">{{ t("game.actionQuickArrangeDialogTitle") }}</div>
      </template>
      <div class="space-y-3">
        <div v-if="quickPlan.root" ref="workPlanScrollRef" class="max-h-[70vh] overflow-auto pr-1">
          <ActionProductionTreeD3 :root="quickPlan.root" />
          <BaseCard v-if="quickSelectableMaterials.length" shadow="never" class="mt-3">
            <div class="text-xs font-semibold uppercase tracking-wide app-muted mb-2">{{ t("game.actionQuickArrangeRecipeSwitch") }}</div>
            <div class="space-y-2">
              <div
                v-for="material in quickSelectableMaterials"
                :key="`recipe-${material.nodeId}`"
                class="rounded border border-[var(--app-border)] p-2"
              >
                <div class="flex items-center gap-2 mb-2">
                  <QuickRefWithRequire :item-id="material.itemId" :required-count="material.required" />
                </div>
                <el-select
                  :model-value="selectedRecipeByNode[material.nodeId] || material.selectedActionId || ''"
                  size="small"
                  class="w-full"
                  @update:model-value="createRecipeChoiceUpdater(material.nodeId)"
                >
                  <el-option
                    v-for="option in material.options"
                    :key="`${material.nodeId}-${option.actionId}`"
                    :label="recipeActionLabel(option.actionId, option.nameKey)"
                    :value="option.actionId"
                  />
                </el-select>
              </div>
            </div>
          </BaseCard>
        </div>
        <div v-else class="text-xs app-muted">{{ t("game.actionQuickArrangeEmpty") }}</div>
      </div>
    </GameDialog>
    <GameDialog v-model="planActionsVisible" width="640px" destroy-on-close>
      <template #header>
        <div class="font-semibold">{{ t("game.actionQuickArrangeActionsDialogTitle") }}</div>
      </template>
      <div class="space-y-2 max-h-[70vh] overflow-auto pr-1">
        <div
          v-for="step in quickPlan.actionSteps"
          :key="`plan-action-${step.actionId}`"
          class="rounded border border-[var(--app-border)] p-2"
        >
          <div class="flex items-center gap-2 flex-wrap">
            <ActionQuickRef :action-id="step.actionId" />
            <span class="text-xs font-semibold">x{{ step.count }}</span>
            <span class="text-[11px] app-muted">
              {{ t("game.actionQuickArrangeNodeDuration", { single: formatDurationSecondsOneDecimal(step.durationS, t), total: formatDurationI18n(step.totalDurationS, t) }) }}
            </span>
          </div>
        </div>
      </div>
    </GameDialog>
    <GameDialog v-model="blockedActionsVisible" width="640px" destroy-on-close>
      <template #header>
        <div class="font-semibold">{{ t("game.actionQuickArrangeBlockedDialogTitle") }}</div>
      </template>
      <div class="space-y-2 max-h-[70vh] overflow-auto pr-1">
        <div
          v-for="entry in quickBlockedActions"
          :key="`blocked-dialog-${entry.actionId}`"
          class="rounded border border-[var(--el-color-danger-light-7)] p-2"
        >
          <div class="flex items-center gap-2">
            <ActionQuickRef :action-id="entry.actionId" />
            <span class="text-xs font-semibold">x{{ entry.count }}</span>
          </div>
          <div v-if="entry.unmetStatus.length" class="flex flex-wrap gap-1 mt-2">
            {{ t("game.actionReqCount") }}:
            <StatusRequireTag
              v-for="status in entry.unmetStatus"
              :key="`blocked-dialog-status-${entry.actionId}-${status.attrKey}`"
              :attr-key="status.attrKey"
              :required-value="status.required"
            />
          </div>
          <div v-if="entry.unmetBuildings.length" class="flex flex-wrap gap-1 mt-2">
            <BuildingQuickRef
              v-for="building in entry.unmetBuildings"
              :key="`blocked-dialog-building-${entry.actionId}-${building.buildingId}`"
              :building-id="building.buildingId"
              :required-level="building.required"
              :current-level="building.current"
            />
          </div>
        </div>
      </div>
    </GameDialog>
  </GameDialog>
</template>

<script setup lang="ts">
import OperationEffectSummary from "@components/game/operation-effect/OperationEffectSummary.vue"
import { computed, nextTick, reactive, ref, unref, watch } from "vue"
import { ElMessage } from "element-plus"
import { useI18n } from "vue-i18n"
import type { ActionDefinition } from "@game-data/game-data"
import type { ActionMaterialInstanceRequirement } from "@/types/generated/game-data"
import QuickRef from "@components/game/item/QuickRef.vue"
import ActionQuickRef from "@components/game/action/ActionQuickRef.vue"
import ActionProductionTreeD3 from "@components/game/action/ActionProductionTreeD3.vue"
import StatusRequireTag from "@components/game/attribute/StatusRequireTag.vue"
import { getLocalizedItemName } from "@services/item-label"
import { evaluateStatusRequire, toStatusRequireEntries } from "@services/status-require"
import { buildInsufficientActionCosts } from "@composables/game/action/action-cost-availability"
import { isTemplateCommodityInstance, useInventorySummary } from "@/composables/game/useInventorySummary"
import {
  ENHANCE_INHERITANCE_RATIO,
  KITTY_STATUS_EXP_MULTIPLIER
} from "@/config/shared-global-config"
import { usePlayerStore } from "@stores/player"
import { useGameStore } from "@stores/game"
import { useCatteryFurnitureStore } from '@stores/cattery-furniture'
import { furnitureWorkEfficiency } from '@services/cattery-furniture'
import StatusTag from "../attribute/StatusTag.vue"
import { useAttributeStats } from "@/composables/game/attributes/useAttributeStats"
import { formatDurationI18n, formatDurationSecondsOneDecimal } from "@/utils/duration-format"
import { resolveActionDurationSeconds } from "@/services/action-duration"
import { useWorkMeal } from "@/composables/game/useWorkMeal"
import { isPrimaryWork, workMealDuration } from "@services/work-meal"
import OperationEffectNotice from "@components/game/operation-effect/OperationEffectNotice.vue"
import FoodValueHint from "@components/game/operation-effect/FoodValueHint.vue"
import { listActionDefinitions } from "@services/action-definition"
import { buildProductionPlan, type ProductionMaterialNode } from "@services/action-production-plan"
import { apiGet } from "@services/api"
import BaseCard from "@components/game/common/BaseCard.vue"
import QuickRefWithRequire from "../item/QuickRefWithRequire.vue"
import BuildingQuickRef from "@components/game/building/BuildingQuickRef.vue"
import { getEquipmentTemplateDefinition } from "@services/item-definition"
import Card from "@components/game/item/Card.vue"
import EmptyCard from "@components/game/item/EmptyCard.vue"
import InventoryInstanceSelectorDialog from "@components/game/inventory/InventoryInstanceSelectorDialog.vue"
import GameDialog from "@components/game/common/GameDialog.vue"
import FishMutationArt from "@components/game/fishing/FishMutationArt.vue"
import FishFoodQualityBreakdown from "@components/game/item/FishFoodQualityBreakdown.vue"
import { Icon } from "@iconify/vue"
import { useDevice } from "@/composables/game/useDevice"
import { operationEffectLifetimeText, operationEffectModifierLines } from "@services/operation-effect-display"
import { getFishSpeciesDisplay, localizedFishText } from "@services/fish-instance-display"
import {
  buildFishFoodSpecificArgs,
  getFishFoodEffectDefinition,
  getFishFoodRecipeByActionId,
  resolveFishFoodEffectParams,
  resolveFishFoodLifetime,
  resolveFishFoodQuality,
  type FishFoodRecipeDefinition
} from "@services/fish-food"
import type { InventoryInstance } from "@/types/equipment"
import {
  actionMaterialAffixesMatchTemplate,
  actionMaterialCanProvideEnhancementInheritance,
  actionMaterialInstanceSatisfiesRequirement,
  buildActionMaterialBindingArgs
} from "@services/action-material-requirement"

type ActionDef = ActionDefinition & {
  materialInstanceRequirements?: Record<string, ActionMaterialInstanceRequirement>
}

type ActionItemRewardRule = {
  count?: number
  range?: { min: number; max: number }
  percent?: number
}

type ActionActorType = "player" | "kitty"
type ActionActorOption = {
  key: string
  label: string
  actorType: ActionActorType
  actorId: number
}
type FishFoodIngredientSlot = {
  key: string
  speciesId: string
  index: number
}

const props = defineProps<{
  modelValue: boolean
  action: ActionDef | null
  count: number
  currentAttributes?: Record<string, number>
}>()

const { normalizedStatusValue } = useAttributeStats()
const playerStore = usePlayerStore()
const gameStore = useGameStore()
const furnitureStore = useCatteryFurnitureStore()
const kittyDurationAttributesById = ref<Record<number, Record<string, number>>>({})

const emit = defineEmits<{
  (e: "update:modelValue", value: boolean): void
  (e: "update:count", value: number): void
  (e: "run-now", payload: { actionId: string; actorType: ActionActorType; actorId: number; args?: Record<string, unknown> }): void
  (e: "enqueue", payload: { actionId: string; actorType: ActionActorType; actorId: number; args?: Record<string, unknown> }): void
  (e: "enqueue-plan", payload: {
    steps: Array<{ actionId: string; count: number }>
    actorType: ActionActorType
    actorId: number
    insertMode: "append" | "run_now_like_prepend"
  }): void
}>()

const { t, locale } = useI18n()
const { isMobile, supportsHover } = useDevice()
const { getTemplateCommodityQuantity, inventoryInstances } = useInventorySummary()
const MAX_ACTION_COUNT = 9999999999
const allActionDefinitions = listActionDefinitions()
const actionDefinitionById = new Map<string, ActionDefinition>(allActionDefinitions.map((row) => [row.id, row]))
const selectedMaterialInstanceByItem = reactive<Record<string, number>>({})
const selectedInheritanceSourceInstanceId = ref(0)
const materialSelectorVisible = ref(false)
const materialSelectorItemId = ref("")
const useSpecificFishIngredients = ref(false)
const selectedFishInstanceBySlot = reactive<Record<string, number>>({})
const fishSelectorVisible = ref(false)
const fishSelectorSlotKey = ref("")
const activeDetailTab = ref("execution")
const workPlanVisible = ref(false)
const workPlanScrollRef = ref<HTMLElement | null>(null)
const planActionsVisible = ref(false)
const blockedActionsVisible = ref(false)
const selectedRecipeByNode = reactive<Record<string, string>>({})

const visible = computed({
  get: () => props.modelValue,
  set: (value: boolean) => emit("update:modelValue", value)
})
const selectedActorKey = ref("player:0")
const actorOptions = computed<ActionActorOption[]>(() => {
  const options: ActionActorOption[] = [
    {
      key: "player:0",
      label: t("game.executorPlayer"),
      actorType: "player",
      actorId: 0
    }
  ]
  for (const kitty of playerStore.kitties || []) {
    options.push({
      key: `kitty:${kitty.id}`,
      label: `${t("game.executorKitty")}: ${kitty.name}`,
      actorType: "kitty",
      actorId: kitty.id
    })
  }
  return options
})
const selectedActor = computed<ActionActorOption>(() => {
  const selected = actorOptions.value.find((row) => row.key === selectedActorKey.value)
  return selected || actorOptions.value[0]
})
const playerDurationAttributes = computed<Record<string, number>>(() => {
  const raw = props.currentAttributes || unref(normalizedStatusValue) || {}
  const out: Record<string, number> = {}
  for (const [key, value] of Object.entries(raw as Record<string, unknown>)) {
    if (typeof value === "number" && Number.isFinite(value)) {
      out[key] = value
    }
  }
  return out
})
const durationAttributes = computed<Record<string, number>>(() => {
  if (selectedActor.value.actorType !== "kitty" || selectedActor.value.actorId <= 0) {
    return playerDurationAttributes.value
  }
  return kittyDurationAttributesById.value[selectedActor.value.actorId] || {}
})
const workMealContext = computed(() => ({
  domain: "life_action" as const,
  actionId: props.action?.id,
  mainType: props.action?.mainType,
  subType: props.action?.subType,
  mainOutput: props.action?.mainOutput,
  actorKind: selectedActor.value.actorType,
  actorId: selectedActor.value.actorId
}))
const { modifiers: workMealModifiers, planSeconds: workMealPlanSeconds } = useWorkMeal(() => workMealContext.value)
const furnitureEfficiency = (main: string, sub: string, id: string) => selectedActor.value.actorType === 'kitty'
  ? furnitureWorkEfficiency(furnitureStore.projection?.items || [], main, sub, id, furnitureStore.serverNow()) : 0
watch(() => [visible.value, selectedActor.value.actorType, gameStore.token] as const, ([open, actor, token]) => {
  if (open && actor === 'kitty' && token) void furnitureStore.refresh(token)
}, { immediate: true })
const basePerRunDurationSeconds = computed(() =>
  resolveActionDurationSeconds(
    Number(props.action?.durationS || 1),
    props.action?.mainType,
    props.action?.subType,
    props.action?.id,
    selectedActor.value.actorType,
    durationAttributes.value,
    furnitureEfficiency(props.action?.mainType || '', props.action?.subType || '', props.action?.id || '')
  )
)
const perRunDurationSeconds = computed(() => workMealDuration(basePerRunDurationSeconds.value, workMealModifiers.value.durationPenaltyRate, workMealModifiers.value.durationReductionRate))
const kittyExecutorHint = computed(() => {
  if (KITTY_STATUS_EXP_MULTIPLIER <= 0) {
    return t("game.executorKittyTooltipNoExp")
  }
  return t("game.executorKittyTooltip", {
    expMultiplier: KITTY_STATUS_EXP_MULTIPLIER.toFixed(2)
  })
})
const fishFoodRecipe = computed<FishFoodRecipeDefinition | null>(() =>
  getFishFoodRecipeByActionId(String(props.action?.id || ""))
)
const fishFoodEffectDefinition = computed(() => getFishFoodEffectDefinition(fishFoodRecipe.value))
const fishFoodIngredientSlots = computed<FishFoodIngredientSlot[]>(() => {
  const slots: FishFoodIngredientSlot[] = []
  for (const ingredient of fishFoodRecipe.value?.fishIngredients || []) {
    const quantity = Math.max(0, Math.trunc(Number(ingredient.quantity || 0)))
    for (let index = 0; index < quantity; index += 1) {
      slots.push({
        key: `${ingredient.speciesId}:${index}`,
        speciesId: ingredient.speciesId,
        index
      })
    }
  }
  return slots
})
const fishFoodSpecificBindings = computed<Record<string, number[]>>(() => {
  const bindings: Record<string, number[]> = {}
  for (const slot of fishFoodIngredientSlots.value) {
    const selected = selectedFishForSlot(slot.key)
    if (!selected) continue
    if (!bindings[slot.speciesId]) bindings[slot.speciesId] = []
    bindings[slot.speciesId].push(selected.instanceId)
  }
  return bindings
})
const fishFoodSpecificArgs = computed(() => {
  const recipe = fishFoodRecipe.value
  if (!recipe || !useSpecificFishIngredients.value) return undefined
  return buildFishFoodSpecificArgs(recipe, fishFoodSpecificBindings.value)
})
const selectedFishFoodInstances = computed<InventoryInstance[]>(() =>
  fishFoodIngredientSlots.value
    .map((slot) => selectedFishForSlot(slot.key))
    .filter((entry): entry is InventoryInstance => !!entry)
)
const fishFoodQualityPreview = computed(() => {
  const recipe = fishFoodRecipe.value
  if (!recipe || !useSpecificFishIngredients.value || !fishFoodSpecificArgs.value) return null
  return resolveFishFoodQuality(recipe, selectedFishFoodInstances.value)
})
const fishFoodEffectiveParams = computed<Record<string, number> | null>(() => {
  const definition = fishFoodEffectDefinition.value
  if (!definition) return null
  if (!useSpecificFishIngredients.value) return resolveFishFoodEffectParams(definition, 1)
  if (!fishFoodQualityPreview.value) return null
  return resolveFishFoodEffectParams(definition, fishFoodQualityPreview.value, fishFoodRecipe.value || undefined)
})
const fishFoodEffectLines = computed<string[]>(() => {
  const definition = fishFoodEffectDefinition.value
  const params = fishFoodEffectiveParams.value
  if (!definition || !params) return []
  const lines: string[] = []
  const lifetime = resolveFishFoodLifetime(definition, fishFoodQualityPreview.value)
  const lifetimeText = operationEffectLifetimeText({ ...definition, params, lifetime }, t)
  if (lifetimeText) lines.push(lifetimeText)
  lines.push(...operationEffectModifierLines(definition.kind, params, t))
  return lines
})
const fishFoodAutomaticAvailable = computed(() => {
  const recipe = fishFoodRecipe.value
  if (!recipe) return true
  const repeatCount = clampActionCount(props.count || 1)
  return recipe.fishIngredients.every((ingredient) => (
    fishOwnedCount(ingredient.speciesId) >= Math.max(0, Math.trunc(Number(ingredient.quantity || 0))) * repeatCount
  ))
})
const fishFoodSubmitAllowed = computed(() => {
  if (!fishFoodRecipe.value || !useSpecificFishIngredients.value) return true
  return clampActionCount(props.count || 1) === 1 && !!fishFoodSpecificArgs.value && !!fishFoodQualityPreview.value
})
const fishFoodRunNowAllowed = computed(() => {
  if (!fishFoodRecipe.value) return true
  if (useSpecificFishIngredients.value) return fishFoodSubmitAllowed.value
  return fishFoodAutomaticAvailable.value
})
const fishFoodHintText = computed(() => {
  if (!fishFoodRecipe.value) return ""
  if (useSpecificFishIngredients.value) {
    if (clampActionCount(props.count || 1) !== 1) return t("game.fishFood.specificCountLimit")
    if (!fishFoodSpecificArgs.value || !fishFoodQualityPreview.value) return t("game.fishFood.selectAllIngredients")
    return ""
  }
  if (!fishFoodAutomaticAvailable.value) return t("game.fishFood.insufficientFish")
  return ""
})
const activeFishSelectorSlot = computed(() =>
  fishFoodIngredientSlots.value.find((slot) => slot.key === fishSelectorSlotKey.value) || null
)
const activeFishSelectorInstanceIds = computed(() => {
  const slot = activeFishSelectorSlot.value
  if (!slot) return []
  const selectedElsewhere = new Set(
    fishFoodIngredientSlots.value
      .filter((entry) => entry.key !== slot.key)
      .map((entry) => Number(selectedFishInstanceBySlot[entry.key] || 0))
      .filter((instanceID) => instanceID > 0)
  )
  return inventoryInstances.value
    .filter((entry) => isSelectableFishIngredient(entry, slot.speciesId) && !selectedElsewhere.has(entry.instanceId))
    .map((entry) => entry.instanceId)
})
const activeFishSelectedInstanceId = computed<number | null>(() => {
  const slot = activeFishSelectorSlot.value
  if (!slot) return null
  const instanceID = Number(selectedFishInstanceBySlot[slot.key] || 0)
  return activeFishSelectorInstanceIds.value.includes(instanceID) ? instanceID : null
})
const fishSelectorTitle = computed(() => {
  const slot = activeFishSelectorSlot.value
  return slot
    ? t("game.fishFood.selectorTitle", { fish: fishSpeciesLabel(slot.speciesId) })
    : t("game.fishFood.ingredientsTitle")
})

watch(
  () => visible.value,
  (open) => {
    if (!open) {
      resetMaterialBindings()
      resetFishFoodBindings()
      resetRecipeSelections()
      return
    }
    activeDetailTab.value = "execution"
    selectedActorKey.value = "player:0"
    resetMaterialBindings()
    resetFishFoodBindings()
    resetRecipeSelections()
    planActionsVisible.value = false
    blockedActionsVisible.value = false
    void ensureSelectedKittyDurationAttributesLoaded()
  }
)
watch(
  () => props.action?.id,
  () => resetFishFoodBindings()
)
watch(activeDetailTab, (tab) => {
  if (tab === "execution" || !useSpecificFishIngredients.value) return
  useSpecificFishIngredients.value = false
  resetFishSelector()
})
watch(actorOptions, (options) => {
  if (!options.some((row) => row.key === selectedActorKey.value)) {
    selectedActorKey.value = "player:0"
  }
})
watch(
  () => [visible.value, selectedActor.value.actorType, selectedActor.value.actorId] as const,
  () => {
    void ensureSelectedKittyDurationAttributesLoaded()
  }
)
watch(workPlanVisible, async (open) => {
  if (!open) return
  await nextTick()
  const container = workPlanScrollRef.value
  if (!container) return
  const rootNode = container.querySelector("[data-root-node='true']") as HTMLElement | null
  if (!rootNode) return
  rootNode.scrollIntoView({
    behavior: "auto",
    block: "start",
    inline: "center"
  })
})
const insufficientCosts = computed(() => {
  return buildInsufficientActionCosts(props.action?.costs, props.count || 1, (itemId) => {
    return getTemplateCommodityQuantity(itemId) + (selectedBindingExtraQuantityByItem.value[itemId] || 0)
  })
})
const insufficientStatus = computed(() => {
  return evaluateStatusRequire(props.action?.statusRequirement, unref(normalizedStatusValue))
})
const insufficientBuilding = computed(() =>
  buildingRequirementEntries(props.action?.buildingRequirement).filter((entry) => entry.current < entry.required)
)
const canSubmitAction = computed(
  () =>
    canAddToQueue.value &&
    fishFoodRunNowAllowed.value &&
    insufficientCosts.value.length === 0 &&
    insufficientStatus.value.length === 0 &&
    insufficientBuilding.value.length === 0 &&
    materialBindingSubmitAllowed.value
)

const canAddToQueue = computed(
  () => !!props.action?.id && materialBindingSubmitAllowed.value && fishFoodSubmitAllowed.value
)
const costEntries = computed(() => itemEntries(props.action?.costs))
const bindableCostEntries = computed(() =>
  costEntries.value.filter((entry) => (
    (getEquipmentTemplateDefinition(entry.itemId)?.enhanceMaxLevel || 0) > 0 ||
    !!materialInstanceRequirement(entry.itemId)
  ))
)
const hasMaterialInstanceRequirements = computed(() =>
  bindableCostEntries.value.some((entry) => !!materialInstanceRequirement(entry.itemId))
)
const materialInstanceOptionsByItem = computed(() => {
  const out: Record<string, Array<{ instanceId: number; enhanceLevel: number; quantity: number }>> = {}
  for (const entry of bindableCostEntries.value) {
    const itemID = entry.itemId
    const template = getEquipmentTemplateDefinition(itemID)
    const requirement = materialInstanceRequirement(itemID)
    out[itemID] = inventoryInstances.value
      .filter((row) => {
        return row.templateItemId === itemID &&
          row.state === "inventory" &&
          row.quantity > 0 &&
          actionMaterialAffixesMatchTemplate(row.affixes, template?.defaultAffixSlots) &&
          (!requirement || actionMaterialInstanceSatisfiesRequirement(row, requirement, template?.defaultAffixSlots))
      })
      .map((row) => ({
        instanceId: row.instanceId,
        enhanceLevel: Number(row.enhanceLevel || 0),
        quantity: Number(row.quantity || 0)
      }))
      .sort((a, b) => b.enhanceLevel - a.enhanceLevel || a.instanceId - b.instanceId)
  }
  return out
})
const selectedMaterialBindingInstanceIds = computed(() =>
  bindableCostEntries.value
    .map((entry) => Number(selectedMaterialInstanceByItem[entry.itemId] || 0))
    .filter((instanceId) => instanceId > 0)
)
const hasInvalidSelectedMaterialBinding = computed(() =>
  bindableCostEntries.value.some((entry) => {
    const instanceId = Number(selectedMaterialInstanceByItem[entry.itemId] || 0)
    return instanceId > 0 && !isSelectableMaterialInstance(entry.itemId, instanceId)
  })
)
const inheritanceBindableCostEntries = computed(() =>
  bindableCostEntries.value.filter((entry) => (
    actionMaterialCanProvideEnhancementInheritance(props.action?.materialInstanceRequirements, entry.itemId)
  ))
)
const selectedBindingOptions = computed(() =>
  inheritanceBindableCostEntries.value
    .map((entry) => selectedMaterialInstanceByItem[entry.itemId] || 0)
    .filter((instanceId) => instanceId > 0)
    .map((instanceId) => {
      const row = inventoryInstances.value.find((item) => item.instanceId === instanceId)
      return row
        ? {
            instanceId,
            enhanceLevel: Number(row.enhanceLevel || 0),
            quantity: Number(row.quantity || 0)
          }
        : null
    })
    .filter((row): row is { instanceId: number; enhanceLevel: number; quantity: number } => !!row)
)
const selectedBindingExtraQuantityByItem = computed<Record<string, number>>(() => {
  const out: Record<string, number> = {}
  for (const entry of bindableCostEntries.value) {
    const instanceId = Number(selectedMaterialInstanceByItem[entry.itemId] || 0)
    if (instanceId <= 0 || !isSelectableMaterialInstance(entry.itemId, instanceId)) continue
    const row = inventoryInstances.value.find((item) => item.instanceId === instanceId)
    if (!row || String(row.templateItemId || "").trim() !== entry.itemId) continue
    if (isTemplateCommodityInstance(row)) continue
    const quantity = Math.max(0, Number(row.quantity || 0))
    if (quantity <= 0) continue
    out[entry.itemId] = (out[entry.itemId] || 0) + quantity
  }
  return out
})
const activeMaterialSelectorInstanceIds = computed(() => {
  const itemID = String(materialSelectorItemId.value || "").trim()
  if (!itemID) return []
  return materialInstanceOptionsByItem.value[itemID]?.map((row) => row.instanceId) || []
})
const activeMaterialSelectorTemplateItemIds = computed(() => {
  const itemID = String(materialSelectorItemId.value || "").trim()
  return itemID ? [itemID] : []
})
const activeMaterialSelectedInstanceId = computed<number | null>(() => {
  const itemID = String(materialSelectorItemId.value || "").trim()
  if (!itemID) return null
  const selected = Number(selectedMaterialInstanceByItem[itemID] || 0)
  return selected > 0 ? selected : null
})
const materialSelectorTitle = computed(() => {
  const itemID = String(materialSelectorItemId.value || "").trim()
  if (!itemID) return t("game.actionRequirements")
  return bindableItemLabel(itemID)
})
const selectedInheritanceSourceLevel = computed(() => {
  const targetID = Number(selectedInheritanceSourceInstanceId.value || 0)
  if (targetID <= 0) return 0
  const row = inventoryInstances.value.find((item) => item.instanceId === targetID)
  return Math.max(0, Number(row?.enhanceLevel || 0))
})
const firstEnhanceableRewardMaxLevel = computed(() => {
  const rewards = props.action?.itemReward || {}
  for (const itemID of Object.keys(rewards)) {
    const template = getEquipmentTemplateDefinition(itemID)
    if (template && template.enhanceMaxLevel > 0) {
      return template.enhanceMaxLevel
    }
  }
  return 0
})
const inheritedTargetLevel = computed(() => {
  const sourceLevel = selectedInheritanceSourceLevel.value
  if (sourceLevel <= 0) return 0
  const maxLevel = firstEnhanceableRewardMaxLevel.value
  if (maxLevel <= 0) return 0
  return Math.max(0, Math.min(maxLevel, Math.floor(sourceLevel * ENHANCE_INHERITANCE_RATIO)))
})
const inheritancePreviewText = computed(() => {
  if (!selectedBindingOptions.value.length) return ""
  const sourceLevel = selectedInheritanceSourceLevel.value
  const targetLevel = inheritedTargetLevel.value
  return t("game.actionEnhanceInheritanceEffect", {
    source: sourceLevel,
    target: targetLevel
  })
})
const materialBindingSubmitAllowed = computed(() => {
  if (!selectedMaterialBindingInstanceIds.value.length) return true
  if (hasInvalidSelectedMaterialBinding.value) return false
  if (clampActionCount(props.count || 1) !== 1) return false
  if (!selectedBindingOptions.value.length) return true
  const sourceID = Number(selectedInheritanceSourceInstanceId.value || 0)
  return selectedBindingOptions.value.some((entry) => entry.instanceId === sourceID)
})

const insufficientCostText = computed(() => {
  const first = insufficientCosts.value[0]
  if (!first) return ""
  const itemName = getLocalizedItemName(t, first.itemId, { fallbackText: first.itemId })
  return t("game.actionInsufficientMaterials", { item: itemName, owned: first.owned, required: first.required })
})
const insufficientStatusText = computed(() => {
  const first = insufficientStatus.value[0]
  if (!first) return ""
  const attrNameKey = `attr.${first.attrKey}.name`
  const localized = t(attrNameKey)
  const attrLabel = localized === attrNameKey ? first.attrKey : localized
  return t("game.actionStatusRequireNotMet", { attr: attrLabel, current: first.current, required: first.required })
})
const insufficientBuildingText = computed(() => {
  const first = insufficientBuilding.value[0]
  if (!first) return ""
  return t("game.actionBuildingRequireNotMet", {
    building: resolveBuildingName(first.buildingId),
    current: first.current,
    required: first.required
  })
})
const materialBindingHintText = computed(() => {
  if (materialBindingSubmitAllowed.value) return ""
  if (hasInvalidSelectedMaterialBinding.value) {
    return t("game.actionMaterialBindingInvalid")
  }
  if (clampActionCount(props.count || 1) !== 1) {
    return t("game.actionInstanceBindingCountLimit")
  }
  return t("game.actionSelectInheritanceSource")
})
const estimatedDurationText = computed(() => {
  const perRun = basePerRunDurationSeconds.value
  const repeatCount = clampActionCount(props.count || 1)
  return formatDurationI18n(workMealPlanSeconds(repeatCount, perRun), t)
})
const inventoryByItemId = computed<Record<string, number>>(() => {
  const out: Record<string, number> = {}
  for (const row of inventoryInstances.value) {
    const itemID = String(row.templateItemId || "").trim()
    if (!itemID) continue
    out[itemID] = (out[itemID] || 0) + Math.max(0, Number(row.quantity || 0))
  }
  return out
})
const quickPlan = computed(() => {
  const action = props.action
  if (!action) {
    return {
      root: null,
      actionSteps: [] as Array<{ actionId: string; nameKey: string; count: number; durationS: number; totalDurationS: number }>,
      unresolvedMaterials: [] as Array<{ itemId: string; deficit: number }>,
      totalDurationS: 0,
      hasCycle: false
    }
  }
  return buildProductionPlan({
    rootAction: action,
    rootCount: clampActionCount(props.count || 1),
    actions: allActionDefinitions,
    inventoryByItemId: inventoryByItemId.value,
    selectedRecipeByNode,
    durationContext: {
      actorType: selectedActor.value.actorType,
      attributes: durationAttributes.value,
      additionalEfficiency: furnitureEfficiency
    }
  })
})
const quickUnmetStatusByActionId = computed<Record<string, Array<{ attrKey: string; required: number; current: number }>>>(() => {
  const out: Record<string, Array<{ attrKey: string; required: number; current: number }>> = {}
  for (const step of quickPlan.value.actionSteps) {
    const def = actionDefinitionById.get(step.actionId)
    out[step.actionId] = evaluateStatusRequire(def?.statusRequirement, unref(normalizedStatusValue))
  }
  return out
})
const quickUnmetBuildingsByActionId = computed<Record<string, Array<{ buildingId: string; required: number; current: number }>>>(() => {
  const out: Record<string, Array<{ buildingId: string; required: number; current: number }>> = {}
  for (const step of quickPlan.value.actionSteps) {
    const def = actionDefinitionById.get(step.actionId)
    out[step.actionId] = buildingRequirementEntries(def?.buildingRequirement).filter((entry) => entry.current < entry.required)
  }
  return out
})
const quickBlockedActions = computed(() =>
  quickPlan.value.actionSteps
    .map((step) => ({
      actionId: step.actionId,
      count: step.count,
      unmetStatus: quickUnmetStatusByActionId.value[step.actionId] || [],
      unmetBuildings: quickUnmetBuildingsByActionId.value[step.actionId] || []
    }))
    .filter((row) => row.unmetStatus.length > 0 || row.unmetBuildings.length > 0)
)
const quickSelectableMaterials = computed<ProductionMaterialNode[]>(() => {
  const root = quickPlan.value.root
  if (!root) return []
  const out: ProductionMaterialNode[] = []
  const seen = new Set<string>()
  const walk = (actionNode: { costs: ProductionMaterialNode[] }) => {
    for (const material of actionNode.costs || []) {
      if (material.options.length > 1 && material.deficit > 0 && !seen.has(material.nodeId)) {
        seen.add(material.nodeId)
        out.push(material)
      }
      if (material.childAction) {
        walk(material.childAction)
      }
    }
  }
  walk(root)
  return out
})
const canEnqueueQuickPlan = computed(
  () =>
  (
    quickPlan.value.actionSteps.length > 0 &&
    // quickPlan.value.unresolvedMaterials.length === 0 &&
    !quickPlan.value.hasCycle
    // quickBlockedActions.value.length === 0
  )
)

function actionLabel(item: ActionDef): string {
  if (item.nameKey) {
    const localized = t(item.nameKey)
    if (localized !== item.nameKey && localized.trim() !== "") return localized
  }
  return item.id
}

function actionDescription(item: ActionDef): string {
  if (!item.descKey) return ""
  const localized = t(item.descKey)
  return localized === item.descKey || localized.trim() === "" ? "" : localized
}

function onCountChange(value: string | number | undefined) {
  if (useSpecificFishIngredients.value) {
    emit("update:count", 1)
    return
  }
  const next = clampActionCount(Number(value || 1))
  emit("update:count", next)
}

function clampActionCount(value: number) {
  return Math.min(MAX_ACTION_COUNT, Math.max(1, Math.floor(value || 1)))
}

function appendActionCount(delta: number) {
  if (useSpecificFishIngredients.value) return
  const base = clampActionCount(props.count || 1)
  const next = clampActionCount(base + Math.max(1, Math.floor(delta || 1)))
  emit("update:count", next)
}

function appendActionCountByDuration(durationSeconds: number) {
  if (useSpecificFishIngredients.value) return
  const perRun = perRunDurationSeconds.value
  const targetDuration = Math.max(1, Math.floor(durationSeconds || 1))
  const deltaCount = Math.max(1, Math.ceil(targetDuration / perRun))
  appendActionCount(deltaCount)
}

function emitRunNow(actionId: string) {
  if (!canSubmitAction.value) {
    ElMessage.warning(
      insufficientCostText.value ||
        insufficientStatusText.value ||
        insufficientBuildingText.value ||
        materialBindingHintText.value ||
        fishFoodHintText.value ||
      t("game.marketInsufficientInventory")
    )
    return
  }
  emit("run-now", {
    actionId,
    actorType: selectedActor.value.actorType,
    actorId: selectedActor.value.actorId,
    args: buildActionArgs()
  })
}

function emitEnqueue(actionId: string) {
  if (!canAddToQueue.value) {
    ElMessage.warning(fishFoodHintText.value || materialBindingHintText.value || insufficientStatusText.value || t("game.marketInsufficientInventory"))
    return
  }
  emit("enqueue", {
    actionId,
    actorType: selectedActor.value.actorType,
    actorId: selectedActor.value.actorId,
    args: buildActionArgs()
  })
}

function bindableItemLabel(itemId: string): string {
  return getLocalizedItemName(t, itemId, { fallbackText: itemId })
}

function recipeActionLabel(actionId: string, nameKey?: string): string {
  if (nameKey) {
    const localized = t(nameKey)
    if (localized !== nameKey && localized.trim() !== "") return localized
  }
  const fromCatalog = actionDefinitionById.get(actionId)
  if (fromCatalog?.nameKey) {
    const localized = t(fromCatalog.nameKey)
    if (localized !== fromCatalog.nameKey && localized.trim() !== "") return localized
  }
  return actionId
}

function updateRecipeChoice(nodeId: string, actionId: string) {
  const normalizedNodeId = String(nodeId || "").trim()
  const normalizedActionId = String(actionId || "").trim()
  if (!normalizedNodeId) return
  if (!normalizedActionId) {
    delete selectedRecipeByNode[normalizedNodeId]
    return
  }
  selectedRecipeByNode[normalizedNodeId] = normalizedActionId
}

function createRecipeChoiceUpdater(nodeId: string) {
  return (value: unknown) => {
    updateRecipeChoice(nodeId, String(value || ""))
  }
}

function resetRecipeSelections() {
  for (const key of Object.keys(selectedRecipeByNode)) {
    delete selectedRecipeByNode[key]
  }
  workPlanVisible.value = false
  planActionsVisible.value = false
  blockedActionsVisible.value = false
}

function emitEnqueuePlan(insertMode: "append" | "run_now_like_prepend") {
  if (!canEnqueueQuickPlan.value) {
    const blocked = quickBlockedActions.value[0]
    if (blocked) {
      ElMessage.warning(t("game.actionQuickArrangeBlockedActions"))
      return
    }
    const unresolved = quickPlan.value.unresolvedMaterials[0]
    if (unresolved) {
      ElMessage.warning(
        t("game.actionInsufficientMaterials", {
          item: bindableItemLabel(unresolved.itemId),
          owned: 0,
          required: unresolved.deficit
        })
      )
      return
    }
    ElMessage.warning(t("game.actionQuickArrangeCycleDetected"))
    return
  }
  emit("enqueue-plan", {
    steps: quickPlan.value.actionSteps.map((step) => ({
      actionId: step.actionId,
      count: step.count
    })),
    actorType: selectedActor.value.actorType,
    actorId: selectedActor.value.actorId,
    insertMode
  })
}

function selectedMaterialEntry(itemId: string) {
  const instanceId = Number(selectedMaterialInstanceByItem[itemId] || 0)
  if (instanceId <= 0 || !isSelectableMaterialInstance(itemId, instanceId)) return null
  return inventoryInstances.value.find((item) => item.instanceId === instanceId) || null
}

function isSelectableMaterialInstance(itemId: string, instanceId: number): boolean {
  return (materialInstanceOptionsByItem.value[itemId] || []).some((row) => row.instanceId === instanceId)
}

function openMaterialSelector(itemId: string) {
  materialSelectorItemId.value = String(itemId || "").trim()
  if (!materialSelectorItemId.value) return
  materialSelectorVisible.value = true
}

function handleMaterialSelectorConfirm(instanceId: number | null) {
  const itemID = String(materialSelectorItemId.value || "").trim()
  if (!itemID) return
  updateMaterialBinding(itemID, Number(instanceId || 0))
}

function updateMaterialBinding(itemId: string, instanceId: number) {
  if (instanceId > 0 && isSelectableMaterialInstance(itemId, instanceId)) {
    selectedMaterialInstanceByItem[itemId] = instanceId
  } else {
    delete selectedMaterialInstanceByItem[itemId]
  }
  if (!Object.values(selectedMaterialInstanceByItem).includes(selectedInheritanceSourceInstanceId.value)) {
    selectedInheritanceSourceInstanceId.value = selectedBindingOptions.value[0]?.instanceId || 0
  }
}

function resetMaterialBindings() {
  for (const key of Object.keys(selectedMaterialInstanceByItem)) {
    delete selectedMaterialInstanceByItem[key]
  }
  selectedInheritanceSourceInstanceId.value = 0
  materialSelectorVisible.value = false
  materialSelectorItemId.value = ""
}

function buildActionArgs(): Record<string, unknown> | undefined {
  if (fishFoodRecipe.value) {
    if (useSpecificFishIngredients.value) return fishFoodSpecificArgs.value
    return { ingredientMode: "automatic" }
  }
  const bindings: Record<string, number[]> = {}
  for (const entry of bindableCostEntries.value) {
    const instanceId = Number(selectedMaterialInstanceByItem[entry.itemId] || 0)
    if (instanceId > 0 && isSelectableMaterialInstance(entry.itemId, instanceId)) {
      bindings[entry.itemId] = [instanceId]
    }
  }
  const sourceID = Number(selectedInheritanceSourceInstanceId.value || 0)
  const eligibleSourceID = selectedBindingOptions.value.some((entry) => entry.instanceId === sourceID) ? sourceID : 0
  return buildActionMaterialBindingArgs(bindings, eligibleSourceID)
}

function handleSpecificFishModeChange(value: string | number | boolean) {
  const enabled = Boolean(value)
  useSpecificFishIngredients.value = enabled
  if (enabled && clampActionCount(props.count || 1) !== 1) {
    emit("update:count", 1)
  }
  resetFishSelector()
}

function openFishSelector(slotKey: string) {
  if (!fishFoodIngredientSlots.value.some((slot) => slot.key === slotKey)) return
  fishSelectorSlotKey.value = slotKey
  fishSelectorVisible.value = true
}

function handleFishSelectorConfirm(instanceID: number | null) {
  const slot = activeFishSelectorSlot.value
  if (!slot) return
  const normalized = Number(instanceID || 0)
  if (normalized > 0 && activeFishSelectorInstanceIds.value.includes(normalized)) {
    selectedFishInstanceBySlot[slot.key] = normalized
  } else {
    delete selectedFishInstanceBySlot[slot.key]
  }
}

function resetFishSelector() {
  fishSelectorVisible.value = false
  fishSelectorSlotKey.value = ""
}

function resetFishFoodBindings() {
  for (const key of Object.keys(selectedFishInstanceBySlot)) {
    delete selectedFishInstanceBySlot[key]
  }
  useSpecificFishIngredients.value = false
  resetFishSelector()
}

function selectedFishForSlot(slotKey: string): InventoryInstance | null {
  const slot = fishFoodIngredientSlots.value.find((entry) => entry.key === slotKey)
  if (!slot) return null
  const instanceID = Number(selectedFishInstanceBySlot[slotKey] || 0)
  if (instanceID <= 0) return null
  return inventoryInstances.value.find((entry) => (
    entry.instanceId === instanceID && isSelectableFishIngredient(entry, slot.speciesId)
  )) || null
}

function isSelectableFishIngredient(entry: InventoryInstance, speciesID: string): boolean {
  return entry.templateItemId === "fish_catch" &&
    entry.state === "inventory" &&
    entry.quantity === 1 &&
    !entry.archivedAt &&
    String(entry.fishData?.speciesId || "").trim() === String(speciesID || "").trim()
}

function fishOwnedCount(speciesID: string): number {
  return inventoryInstances.value.filter((entry) => isSelectableFishIngredient(entry, speciesID)).length
}

function fishSpeciesLabel(speciesID: string): string {
  const species = getFishSpeciesDisplay(speciesID)
  return localizedFishText(species?.names, locale.value, speciesID)
}

function fishSpeciesIcon(speciesID: string): string {
  return getFishSpeciesDisplay(speciesID)?.iconPath || ""
}

function formatSelectedFish(entry: InventoryInstance): string {
  const length = Number(entry.fishData?.lengthCm || 0).toFixed(1)
  const rawWeight = Number(entry.fishData?.weightKg || 0)
  const weight = rawWeight.toFixed(rawWeight >= 10 ? 1 : 2)
  return t("game.fishFood.selectedMetrics", { length, weight })
}

function formatFishFoodPercent(raw: number): string {
  const value = Math.max(0, Number(raw || 0) * 100)
  return value.toFixed(2).replace(/\.00$/, "").replace(/(\.\d)0$/, "$1")
}

function materialInstanceRequirement(itemId: string): ActionMaterialInstanceRequirement | null {
  return props.action?.materialInstanceRequirements?.[itemId] || null
}

function materialRequirementLabels(itemId: string): string[] {
  const requirement = materialInstanceRequirement(itemId)
  if (!requirement) return []
  const labels: string[] = []
  if (requirement.enhanceLevel === 0 && requirement.requireTemplateAffixes) {
    labels.push(t("game.actionMaterialOriginalOnly"))
  } else {
    labels.push(t("game.actionMaterialEnhanceLevel", { level: requirement.enhanceLevel }))
    if (requirement.requireTemplateAffixes) labels.push(t("game.actionMaterialTemplateAffixes"))
  }
  if (requirement.durability === "full") labels.push(t("game.actionMaterialFullDurability"))
  return labels
}

function itemEntries(map?: Record<string, number | ActionItemRewardRule>) {
  return Object.entries(map || {}).map(([itemId, raw]) => {
    if (typeof raw === "number") {
      return {
        itemId,
        quantity: raw,
        percent: 1,
      }
    }
    const count = typeof raw?.count === "number" ? raw.count : undefined
    const range = raw?.range
    const percent = typeof raw?.percent === "number" ? raw.percent : undefined
    const quantity = count !== undefined ? count : range && range.min === range.max ? range.min : 1 
    const parts: string[] = []
    if (range && typeof range.min === "number" && typeof range.max === "number") {
      parts.push(`x${range.min}~${range.max}`)
    } else if (count !== undefined) {
      parts.push(`x${count}`)
    }
    if (percent !== undefined) {
      parts.push(`${Math.max(0, Math.min(100, Math.round(percent * 100)))}%`)
    }
    return {
      itemId,
      quantity,
      percent,
    }
  })
}

function rewardItemEntries(map?: Record<string, number | ActionItemRewardRule>) {
  return Object.entries(map || {}).map(([itemId, raw]) => {
    if (typeof raw === "number") {
      return {
        itemId,
        quantity: raw * props.count,
        percent: 100,
      }
    }

    let countStr = '';
    let rawCount = raw.count;

    if (typeof rawCount === 'number') {
      countStr = `${rawCount * props.count}`;
    } else if (raw.range && typeof raw.range.min === 'number' && typeof raw.range.max === 'number') {
      countStr = `${raw.range.min * props.count}~${raw.range.max * props.count}`;
    }

    const percent = typeof raw?.percent === "number" ? (raw.percent * 100).toFixed(1) : '100'

    return {
      itemId,
      quantity: countStr,
      percent,
    }
  })
}

async function ensureSelectedKittyDurationAttributesLoaded() {
  if (!visible.value || selectedActor.value.actorType !== "kitty" || selectedActor.value.actorId <= 0 || !gameStore.token) return
  const kittyID = selectedActor.value.actorId
  if (kittyDurationAttributesById.value[kittyID]) return
  const payload = await apiGet<{ attributes?: Array<Record<string, unknown>> }>(
    `/api/cattery/kitties/${encodeURIComponent(String(kittyID))}/combat-profile`,
    gameStore.token
  )
  const attrs: Record<string, number> = {}
  for (const row of payload.attributes || []) {
    const key = String(row.attrKey || "").trim()
    const value = Number(row.finalValue || 0)
    if (!key || !Number.isFinite(value)) continue
    attrs[key] = value
  }
  kittyDurationAttributesById.value = {
    ...kittyDurationAttributesById.value,
    [kittyID]: attrs
  }
}

function statusRequireEntries(map?: Record<string, number>) {
  return toStatusRequireEntries(map)
}

function buildingRequirementEntries(map?: Record<string, number>) {
  return Object.entries(map || {})
    .map(([buildingId, rawRequired]) => {
      const required = Math.max(0, Math.floor(Number(rawRequired || 0)))
      const current = Math.max(0, Math.floor(Number(playerStore.buildings?.[buildingId] || 0)))
      return {
        buildingId,
        required,
        current
      }
    })
    .filter((entry) => entry.required > 0)
}

function resolveBuildingName(buildingId: string) {
  const key = `building.${buildingId}.name`
  const localized = t(key)
  return localized === key ? buildingId : localized
}

function attrExpEntries(map?: Record<string, number>) {
  return Object.entries(map || {}).map(([attrKey, amount]) => {
    const key = `game.attr.${attrKey}`
    return {
      attrKey,
      amount,
    }
  })
}

</script>
