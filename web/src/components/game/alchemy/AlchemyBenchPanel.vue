<template>
  <div class="flex flex-col gap-4">
    <el-segmented v-model="benchMode" :options="benchModeOptions" class="w-full max-w-[420px]" />
    <OperationEffectNotice :context="{ domain: 'alchemy', actionId: 'alchemy.bench', actorKind: 'player', infusionMode: benchMode, materialRarities: infusionMaterialRarities }" />

    <!-- Carrier picker -->
    <section class="space-y-2" data-tutorial="alchemy-bench-carrier">
      <div class="text-sm font-semibold app-muted">{{ t("alchemy.bench.carrier") }}</div>
      <div class="flex flex-wrap items-center gap-3">
        <Card
          v-if="selectedCarrier"
          :item-id="selectedCarrier.templateItemId"
          :instance-id="selectedCarrier.instanceId"
          :quantity="selectedCarrier.quantity"
          @click="carrierSelectorVisible = true"
        />
        <EmptyCard v-else :label="t('alchemy.bench.pickCarrier')" @click="carrierSelectorVisible = true" />
        <div class="flex flex-col gap-1 text-xs app-muted">
          <div v-if="carrierInstanceIds.length > 0">{{ t("game.itemDetailOwnedCount", { count: carrierInstanceIds.length }) }}</div>
          <div>{{ t("alchemy.bench.playerLevelDisplay", { level: playerAlchemyLevel }) }}</div>
        </div>
      </div>
    </section>

    <template v-if="benchMode === 'fish'">
      <section class="alchemy-fish-setup">
        <div class="alchemy-fish-choice-list">
          <button
            v-for="slotIndex in 4"
            :key="`fish-slot-${slotIndex}`"
            class="alchemy-fish-choice"
            type="button"
            @click="fishSelectorVisible = true"
          >
            <FishMutationArt v-if="selectedFish[slotIndex - 1]" :src="fishSpeciesIcon(selectedFish[slotIndex - 1])" :fish="selectedFish[slotIndex - 1].fishData" width="54px" />
            <el-icon v-else><Plus /></el-icon>
            <span>
              <small>{{ t("alchemy.bench.fish.sourceSlot", { index: slotIndex }) }}</small>
              <strong>{{ selectedFish[slotIndex - 1] ? fishSpeciesName(selectedFish[slotIndex - 1]) : t("alchemy.bench.fish.pick") }}</strong>
              <em v-if="selectedFish[slotIndex - 1]">{{ formatFishLength(selectedFish[slotIndex - 1]) }}<FishSizeIndicator metric="length" :value="selectedFish[slotIndex - 1].fishData?.lengthCm || 0" :species="fishSpeciesDefinition(selectedFish[slotIndex - 1])" /> · {{ formatFishWeight(selectedFish[slotIndex - 1]) }}<FishSizeIndicator metric="weight" :value="selectedFish[slotIndex - 1].fishData?.weightKg || 0" :species="fishSpeciesDefinition(selectedFish[slotIndex - 1])" /></em>
            </span>
          </button>
        </div>

        <div class="alchemy-fish-medium">
          <Card item-id="scaleglow_medium" :quantity="scaleglowQuantity" hide-label :width="52" :height="52" />
          <span>
            <small>{{ t("alchemy.bench.fish.medium") }}</small>
            <strong>{{ t("item.scaleglow_medium.name") }}</strong>
            <em>{{ t("alchemy.bench.fish.mediumCount", { count: requiredScaleglow }) }}</em>
          </span>
        </div>
      </section>

      <section class="alchemy-fish-preview">
        <header>
          <div>
            <small>{{ t("alchemy.bench.preview") }}</small>
            <strong>{{ selectedFish.length ? t("alchemy.bench.fish.previewCount", { count: selectedFish.length }) : t("alchemy.bench.fish.awaiting") }}</strong>
          </div>
          <el-tag v-if="selectedFish.length" effect="plain">{{ selectedFish.length }} / 4</el-tag>
        </header>
        <el-skeleton v-if="fishPreviewLoading" :rows="2" animated />
        <div v-else-if="fishPreviewEntries.length" class="alchemy-fish-preview-list">
          <article v-for="entry in fishPreviewEntries" :key="entry.fish.instanceId" class="alchemy-fish-preview__values">
            <header>
              <span>{{ fishSpeciesName(entry.fish) }}</span>
              <el-tag size="small" effect="plain" :type="entry.preview?.traitKind === 'combat' ? 'danger' : 'success'">
                {{ entry.preview ? t(`alchemy.kind.${entry.preview.traitKind}`) : t("alchemy.bench.fish.awaiting") }}
              </el-tag>
            </header>
            <template v-if="entry.preview">
              <div v-if="entry.preview.mutations?.length" class="alchemy-fish-preview__mutations">
                <el-tag v-for="mutation in entry.preview.mutations" :key="mutation" size="small" effect="plain"><Icon icon="lucide:sparkles" />{{ mutationName(mutation) }}</el-tag>
              </div>
              <div>
                <span>{{ localizeAttr(entry.preview.attribute) }}</span>
                <strong>{{ formatAttrRange(entry.preview.attribute, entry.preview.effectiveRange.min, entry.preview.effectiveRange.max) }}</strong>
              </div>
              <div v-if="entry.preview.mutationPowerMultiplier > 1">
                <span>{{ t("game.fishing.mutationAlchemyPower") }}</span>
                <strong>+{{ Math.round((entry.preview.mutationPowerMultiplier - 1) * 100) }}%</strong>
              </div>
              <div v-for="bonus in entry.preview.mutationBonusAttributes || []" :key="bonus.attribute" class="alchemy-fish-preview__bonus">
                <span>{{ localizeAttr(bonus.attribute) }}</span>
                <strong>{{ formatAttrRange(bonus.attribute, bonus.effectiveRange.min, bonus.effectiveRange.max) }}</strong>
              </div>
              <div>
                <span>{{ t("alchemy.bench.fish.floor") }}</span>
                <strong>{{ formatAttributeValueByKey(entry.preview.attribute, entry.preview.failureFloor) }}</strong>
              </div>
            </template>
            <p v-else>{{ fishPreviewError || t("alchemy.bench.fish.issuePreview") }}</p>
          </article>
          <p class="alchemy-fish-preview__rule">{{ t("alchemy.bench.fish.limitHint") }}</p>
        </div>
        <el-empty v-else :description="fishPreviewError || t('alchemy.bench.fish.previewHint')" :image-size="64" />
      </section>

      <section v-if="fishValidationIssues.length" class="rounded border border-[color:var(--el-color-warning,#e6a23c)]/40 bg-[color:var(--el-color-warning,#e6a23c)]/10 p-3">
        <ul class="m-0 list-inside list-disc space-y-1 text-xs">
          <li v-for="issue in fishValidationIssues" :key="issue">{{ issue }}</li>
        </ul>
      </section>
    </template>

    <!-- Medium picker (slot-style) -->
    <section v-if="benchMode === 'materials'" class="space-y-2" data-tutorial="alchemy-bench-medium">
      <div class="text-sm font-semibold app-muted">{{ t("alchemy.bench.medium") }}</div>
      <div class="grid gap-[10px] grid-cols-[repeat(auto-fill,minmax(56px,1fr))]">
        <Card
          v-if="selectedMedium"
          :item-id="selectedMedium.itemId"
          :quantity="selectedMediumOwnedQty"
          @click="mediumSelectorVisible = true"
        />
        <EmptyCard v-else :label="t('alchemy.bench.pickMedium')" @click="mediumSelectorVisible = true" />
      </div>
      <div v-if="!selectedMedium" class="text-xs app-muted">
        {{ t("alchemy.bench.noMediumHint", { slots: DEFAULT_BENCH_MATERIAL_SLOTS }) }}
      </div>
      <el-button v-if="!selectedMedium" size="small" plain class="w-fit" @click="mediumCodexVisible = true">
        {{ t("alchemy.bench.viewMediums") }}
      </el-button>
      <div v-else class="rounded-lg border border-[color:var(--app-border)] bg-[color:var(--app-bg-mute,rgba(0,0,0,0.03))] p-2.5 grid gap-1">
        <div class="flex items-center justify-between gap-2">
          <div class="font-semibold text-sm">{{ t(selectedMedium.nameKey) }}</div>
          <el-button link size="small" type="danger" @click="clearMedium">{{ t("game.remove") }}</el-button>
        </div>
        <div class="text-xs app-muted">{{ t(selectedMedium.descKey) }}</div>
        <div class="flex flex-wrap gap-1.5">
          <el-tag size="small" type="primary" effect="plain">
            {{ t("alchemy.bench.mediumSlots", { slots: effectiveSlots }) }}
          </el-tag>
          <el-tag v-if="selectedMedium.consumePerCraft > 1" size="small" effect="plain">
            {{ t("alchemy.bench.mediumConsume", { qty: selectedMedium.consumePerCraft }) }}
          </el-tag>
          <el-tag v-if="selectedMedium.modifiers.valueMultiplier !== 1" size="small" type="success" effect="plain">
            {{ t("alchemy.bench.mediumValueMult", { pct: formatSignedPercent(selectedMedium.modifiers.valueMultiplier - 1) }) }}
          </el-tag>
          <el-tag v-if="selectedMedium.modifiers.negativeMultiplier !== 1" size="small" :type="selectedMedium.modifiers.negativeMultiplier < 1 ? 'success' : 'warning'" effect="plain">
            {{ t("alchemy.bench.mediumNegativeMult", { pct: formatSignedPercent(selectedMedium.modifiers.negativeMultiplier - 1) }) }}
          </el-tag>
          <el-tag v-if="selectedMedium.modifiers.failChanceDelta !== 0" size="small" :type="selectedMedium.modifiers.failChanceDelta < 0 ? 'success' : 'warning'" effect="plain">
            {{ t("alchemy.bench.mediumFailDelta", { pct: formatSignedPercent(selectedMedium.modifiers.failChanceDelta) }) }}
          </el-tag>
          <el-tag v-if="selectedMedium.modifiers.successFloor > 0" size="small" type="success" effect="plain">
            {{ t("alchemy.bench.mediumSuccessFloor", { pct: formatPercent(selectedMedium.modifiers.successFloor) }) }}
          </el-tag>
          <el-popover
            v-for="(bonus, family) in selectedMedium.modifiers.familyAffinity"
            :key="`affinity-${family}`"
            placement="bottom-start"
            :width="340"
            trigger="hover"
            :show-arrow="true"
          >
              <template #reference>
                <el-tag size="small" effect="plain" :type="bonus >= 0 ? 'primary' : 'warning'" class="cursor-help">
                  {{ t("alchemy.bench.mediumFamilyAffinity", { family: localizeFamily(family), pct: formatSignedPercent(bonus) }) }}
                </el-tag>
              </template>
              <AlchemyFamilyPopoverContent :family="String(family)" />
          </el-popover>
        </div>
        <StatusRequireTag :attr-key="'alchemy_level'" :required-value="selectedMedium.minAlchemyLevel" :current-value="playerAlchemyLevel" placement="bottom-start" :popover-width="320" />
      </div>
    </section>

    <!-- Material slot grid (Card-style to match InventoryInstanceSelectorDialog) -->
    <section v-if="benchMode === 'materials'" class="space-y-2" data-tutorial="alchemy-bench-materials">
      <div class="flex flex-wrap items-center justify-between gap-2">
        <div class="text-sm font-semibold app-muted">
          {{ t("alchemy.bench.materialsWithSlots", { used: filledRows.length, max: effectiveSlots }) }}
        </div>
        <div v-if="selectedCombo" class="alchemy-combo-stepper" data-tutorial="alchemy-bench-combo-stepper">
          <span>{{ t("alchemy.bench.comboStepLabel") }}</span>
          <el-button size="small" :disabled="selectedComboMultiplier <= 1" @click="setComboMultiplier(selectedCombo, selectedComboMultiplier - 1)">-</el-button>
          <strong>{{ t("alchemy.bench.comboMultiplier", { multiplier: selectedComboMultiplier }) }}</strong>
          <el-button size="small" :disabled="selectedComboMultiplier >= selectedComboMaxMultiplier" @click="setComboMultiplier(selectedCombo, selectedComboMultiplier + 1)">+</el-button>
        </div>
      </div>

      <div class="grid gap-[10px] grid-cols-[repeat(auto-fill,minmax(56px,1fr))]">
        <template v-for="(slot, idx) in slotEntries" :key="`slot-${idx}`">
          <Card
            v-if="slot.itemId"
            :item-id="slot.itemId"
            :quantity="materials[slot.itemId] || 0"
            @click="handleSlotClick(idx)"
          />
          <EmptyCard v-else :label="t('alchemy.bench.emptySlot', { n: idx + 1 })" @click="handleSlotClick(idx)" />
        </template>
      </div>

      <!-- Detail panels: one per filled slot, so all material effects are visible at once. -->
      <div class="grid md:grid-cols-2 gap-2">
      <div
        v-for="row in filledRows"
        :key="`detail-${row.itemId}`"
        class="grid gap-2 rounded-lg border border-[color:var(--app-border)] p-3"
      >
        <div class="flex items-start gap-3">
          <Card :item-id="row.itemId" :quantity="row.availableQuantity" hide-label :width="48" :height="48" />
          <div class="flex-1 min-w-0 grid gap-1">
            <div class="font-semibold text-sm">{{ row.itemName }}</div>
            <div class="text-xs app-muted">{{ t(row.trait.nameKey, row.itemName) }} · {{ t(`alchemy.kind.${row.trait.kind}`) }}</div>
          </div>
          <el-button link size="small" type="danger" @click="removeMaterial(row.itemId)">{{ t("game.remove") }}</el-button>
        </div>

        <div class="flex flex-wrap items-center gap-1.5">
          <StatusRequireTag :attr-key="'alchemy_level'" :required-value="row.trait.minAlchemyLevel" :current-value="playerAlchemyLevel" placement="bottom-start" :popover-width="320" />
          <el-tag size="small" effect="plain" :type="row.qtyValid ? 'primary' : 'danger'">
            {{ t("alchemy.bench.qtyRange", { min: row.trait.unitBasis, max: materialLimits(row).capacity }) }}
          </el-tag>
          <el-tag size="small" effect="plain" type="info">
            {{ t("alchemy.bench.materialAffixLimit", { equipment: row.trait.maxEffectivePerEquipment, body: row.trait.maxEffectivePerBody }) }}
          </el-tag>
          <el-popover placement="top" :width="300" trigger="hover">
            <template #reference>
              <el-tag size="small" effect="plain" :type="successTagType(row.successPct)" class="cursor-help">
                {{ t("alchemy.bench.successRate", { rate: formatPercent(row.successPct) }) }}
              </el-tag>
            </template>
            <FailBreakdown :breakdown="row.failBreakdown" :format-percent="formatPercent" :format-signed="formatSignedPercent" :tag-class="successTagColorClass" :t="t" />
          </el-popover>
          <InfusionFoodSuccessHint :failure-delta="row.failBreakdown.foodDelta" />
        </div>

        <div class="alchemy-material-quantity">
          <el-input-number
            v-model="materials[row.itemId]"
            :min="row.maxQty >= row.trait.unitBasis ? row.trait.unitBasis : 0"
            :max="row.maxQty"
            :disabled="row.maxQty < row.trait.unitBasis"
            :step="row.trait.unitBasis"
            step-strictly
            size="small"
            controls-position="right"
            class="alchemy-material-quantity__input"
          />
          <div class="alchemy-material-quick-steps">
            <el-tooltip
              v-for="stepCount in materialQuickStepCounts"
              :key="`quick-${row.itemId}-${stepCount}`"
              placement="top"
              :content="quickStepHint(row, stepCount)"
            >
              <span>
                <el-button
                  size="small"
                  plain
                  class="alchemy-material-quick-step"
                  :disabled="!canIncreaseMaterialBySteps(row)"
                  @click="increaseMaterialBySteps(row, stepCount)"
                >
                  +{{ stepCount }}
                </el-button>
              </span>
            </el-tooltip>
          </div>
          <div class="text-xs app-muted">{{ t("alchemy.bench.using", { qty: materials[row.itemId] || 0 }) }}</div>
        </div>
        <div class="flex flex-wrap items-center gap-x-3 gap-y-1" data-testid="infusion-quantity-feedback">
          <OperationEffectAdjustment
            v-if="materialLimits(row).bonus > 0"
            :label="t('game.fishCuisine.controls.capacity', { count: materialLimits(row).effectiveBonus })"
            :detail="materialCapacityDetail(row)"
          />
          <span v-if="materialLimits(row).stockLimited" class="text-xs app-muted">
            {{ t('game.fishCuisine.controls.stock', { owned: row.availableQuantity, basis: row.trait.unitBasis, usable: row.maxQty }) }}
          </span>
          <span v-if="materialLimits(row).bonus > 0" class="text-xs app-muted">{{ materialBreakthroughText(row) }}</span>
        </div>

        <div class="alchemy-effect-list">
          <div
            v-for="effect in row.trait.effects"
            :key="`${row.itemId}-${effect.attr}`"
            class="alchemy-effect-row"
            :class="effectColorClass(effect)"
          >
            <div class="alchemy-effect-row__main">
              <span class="font-semibold">{{ localizeAttr(effect.attr) }}</span>
              <span class="alchemy-effect-row__value">
                <span class="font-mono tabular-nums">{{ formatEffectTotalPlain(effect, materials[row.itemId] || 0, row.trait.family) }}</span>
                <el-popover placement="top" :width="320" trigger="hover">
                  <template #reference>
                    <el-icon class="alchemy-range-help"><QuestionFilled /></el-icon>
                  </template>
                  <div class="alchemy-range-popover">
                    <div class="alchemy-range-popover__title">{{ t("alchemy.bench.rangeHelpTitle") }}</div>
                    <div v-for="line in effectRangeTooltipRows(effect, materials[row.itemId] || 0, row.trait.family)" :key="`${line.label}-${line.value}`" class="alchemy-range-popover__row">
                      <span>{{ line.label }}</span>
                      <strong>{{ line.value }}</strong>
                    </div>
                  </div>
                </el-popover>
              </span>
            </div>
            <div class="alchemy-effect-row__meta">{{ formatEffectPerUnit(effect, row.trait.family) }}</div>
            <div class="flex flex-wrap gap-x-3 gap-y-1">
              <OperationEffectAdjustment
                v-for="change in effectFoodAdjustments(effect, materials[row.itemId] || 0, row.trait.family)"
                :key="change.label"
                v-bind="change"
              />
            </div>
          </div>
        </div>
      </div>
       </div>
    </section>

    <!-- Preview -->
    <section v-if="benchMode === 'materials'" class="space-y-2" data-tutorial="alchemy-bench-preview">
      <div class="text-sm font-semibold app-muted">{{ t("alchemy.bench.preview") }}</div>
      <div class="grid gap-2 rounded-lg border border-[color:var(--app-border)] p-3">
        <div class="flex flex-wrap items-center gap-2">
          <el-tag v-if="selectedCombo" type="success" effect="plain">{{ t(selectedCombo.nameKey) }}</el-tag>
          <el-tag v-else-if="selectedMaterialCount > 0" type="primary" effect="plain">{{ previewName }}</el-tag>
          <el-tag v-else type="info" effect="plain">{{ t("alchemy.bench.noPreview") }}</el-tag>
          <el-tag v-if="selectedMaterialCount > 0" effect="plain">{{ t("alchemy.bench.materialKinds", { count: selectedMaterialCount }) }}</el-tag>
          <el-tag v-if="selectedCombo && selectedComboMultiplier > 1" type="warning" effect="plain">
            {{ t("alchemy.bench.comboMultiplier", { multiplier: selectedComboMultiplier }) }}
          </el-tag>
          <el-popover v-if="selectedMaterialCount > 0" placement="top" :width="340" trigger="hover">
            <template #reference>
              <el-tag effect="plain" :type="successTagType(aggregatedSuccessPct)" class="cursor-help">
                {{ t("alchemy.bench.overallSuccess", { rate: formatPercent(aggregatedSuccessPct) }) }}
              </el-tag>
            </template>
            <div class="grid gap-2 text-xs">
              <div class="font-semibold">{{ selectedCombo ? t("alchemy.bench.comboSuccessBreakdownTitle") : t("alchemy.bench.overallSuccessBreakdownTitle") }}</div>
              <FailBreakdown :breakdown="overallFailBreakdown" :format-percent="formatPercent" :format-signed="formatSignedPercent" :tag-class="successTagColorClass" :t="t" />
              <div v-if="!selectedCombo" class="grid gap-1 border-t border-[color:var(--app-border)] pt-2">
                <div v-for="row in selectedSuccessRows" :key="`overall-${row.itemId}`" class="flex items-center justify-between gap-3">
                  <span class="truncate">{{ row.itemName }}</span>
                  <span class="font-mono tabular-nums">{{ formatPercent(row.successPct) }}</span>
                </div>
                <div class="app-muted">{{ t("alchemy.bench.overallSuccessRule") }}</div>
              </div>
              <div v-else class="app-muted">{{ t("alchemy.bench.comboSuccessRule") }}</div>
            </div>
          </el-popover>
          <InfusionFoodSuccessHint v-if="selectedMaterialCount > 0" :failure-delta="overallFailBreakdown.foodDelta" />
        </div>

        <div v-if="selectedCombo" class="alchemy-combo-summary">
          <div class="alchemy-combo-summary__head">
            <div class="font-semibold">{{ t("alchemy.bench.comboSpecialEffect") }}</div>
            <div class="app-muted">{{ selectedComboSummary }}</div>
          </div>
          <div class="alchemy-combo-effect-grid">
            <div
              v-for="row in selectedComboEffectRows"
              :key="row.key"
              class="alchemy-combo-effect-row"
              :class="`alchemy-combo-effect-row--${row.tone}`"
            >
              <span>
                <strong>{{ row.label }}</strong>
                <small v-if="row.meta">{{ row.meta }}</small>
              </span>
              <strong class="font-mono tabular-nums">{{ row.valueText }}</strong>
            </div>
          </div>
        </div>

        <div v-if="aggregatedAttrEntries.length > 0" class="grid gap-1">
          <div class="text-[11px] app-muted">{{ t("alchemy.bench.previewAttrs") }}</div>
          <div class="alchemy-preview-groups">
            <div v-if="positiveAttrEntries.length > 0" class="alchemy-effect-list">
              <div
                v-for="entry in positiveAttrEntries"
                :key="`agg-pos-${entry.attr}`"
                class="alchemy-effect-row text-emerald-700 dark:text-emerald-300"
              >
                <div class="alchemy-effect-row__main">
                  <span class="font-semibold truncate">{{ localizeAttr(entry.attr) }}</span>
                  <span class="alchemy-effect-row__value">
                    <span class="font-mono tabular-nums whitespace-nowrap">{{ formatAttrRange(entry.attr, entry.min, entry.max) }}</span>
                    <el-popover placement="top" :width="320" trigger="hover">
                      <template #reference>
                        <el-icon class="alchemy-range-help"><QuestionFilled /></el-icon>
                      </template>
                      <div class="alchemy-range-popover">
                        <div class="alchemy-range-popover__title">{{ t("alchemy.bench.rangeHelpTitle") }}</div>
                        <div v-for="line in aggregateRangeTooltipRows(entry)" :key="`${line.label}-${line.value}`" class="alchemy-range-popover__row">
                          <span>{{ line.label }}</span>
                          <strong>{{ line.value }}</strong>
                        </div>
                      </div>
                    </el-popover>
                  </span>
                </div>
              </div>
            </div>
            <div v-if="negativeAttrEntries.length > 0" class="alchemy-effect-list">
              <div
                v-for="entry in negativeAttrEntries"
                :key="`agg-neg-${entry.attr}`"
                class="alchemy-effect-row text-rose-600 dark:text-rose-400"
              >
                <div class="alchemy-effect-row__main">
                  <span class="font-semibold truncate">{{ localizeAttr(entry.attr) }}</span>
                  <span class="alchemy-effect-row__value">
                    <span class="font-mono tabular-nums whitespace-nowrap">{{ formatAttrRange(entry.attr, entry.min, entry.max) }}</span>
                    <el-popover placement="top" :width="320" trigger="hover">
                      <template #reference>
                        <el-icon class="alchemy-range-help"><QuestionFilled /></el-icon>
                      </template>
                      <div class="alchemy-range-popover">
                        <div class="alchemy-range-popover__title">{{ t("alchemy.bench.rangeHelpTitle") }}</div>
                        <div v-for="line in aggregateRangeTooltipRows(entry)" :key="`${line.label}-${line.value}`" class="alchemy-range-popover__row">
                          <span>{{ line.label }}</span>
                          <strong>{{ line.value }}</strong>
                        </div>
                      </div>
                    </el-popover>
                  </span>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>
    </section>

    <!-- Validation issues (always rendered to make blockers obvious) -->
    <section v-if="benchMode === 'materials' && validationIssues.length > 0" class="rounded-lg border border-[color:var(--el-color-warning,#e6a23c)]/40 bg-[color:var(--el-color-warning,#e6a23c)]/10 p-3">
      <div class="text-xs font-semibold mb-1 text-[color:var(--el-color-warning,#e6a23c)]">{{ t("alchemy.bench.issuesTitle") }}</div>
      <ul class="list-disc list-inside text-xs space-y-0.5">
        <li v-for="(issue, idx) in validationIssues" :key="`issue-${idx}`">{{ issue }}</li>
      </ul>
    </section>

    <!-- Submit -->
    <section data-tutorial="alchemy-bench-submit">
      <AlchemyActionSubmitBar
        action-id="alchemy.bench"
        :args="activeSubmitArgs"
        :count="activeSubmitCount"
        :max-count="activeMaxSubmitCount"
        :per-run-duration-sec="activeDurationSec"
        :gold-cost="activeGoldCost"
        :exp-reward="activeExpReward"
        :success-rate="activeSuccessRate"
        :disabled="!activeCanSubmit"
        :gold-breakdown="activeGoldBreakdown"
        :duration-breakdown="activeDurationBreakdown"
        :exp-breakdown="activeExpBreakdown"
        :success-breakdown="activeSuccessBreakdown"
        :lock-count="benchMode === 'fish'"
        @update:count="updateActiveSubmitCount"
        @submitted="emit('submitted')"
      />
    </section>

    <InventoryInstanceSelectorDialog
      v-model="carrierSelectorVisible"
      :title="t('alchemy.bench.pickCarrier')"
      :instance-ids="carrierInstanceIds"
      :selected-instance-id="selectedCarrierInstanceId"
      :allow-clear="false"
      :template-item-ids="carrierTemplateItemIds"
      :template-items-title="t('game.inventorySelectorAvailableItemsTitle', { title: t('alchemy.bench.carrier') })"
      :empty-text="t('alchemy.bench.noCarrier')"
      @confirm="handleSelectCarrier"
    />

    <InventoryInstanceSelectorDialog
      v-model="materialSelectorVisible"
      :title="t('alchemy.bench.pickMaterial')"
      :instance-ids="availableMaterialInstanceIds"
      :selected-instance-id="null"
      :allow-clear="false"
      :template-item-ids="availableMaterialTemplateIds"
      :template-items-title="t('game.inventorySelectorAvailableItemsTitle', { title: t('alchemy.bench.materials') })"
      :empty-text="t('alchemy.bench.noMaterials')"
      @confirm="handleAddMaterial"
    />

    <InventoryInstanceSelectorDialog
      v-model="mediumSelectorVisible"
      :title="t('alchemy.bench.pickMedium')"
      :instance-ids="availableMediumInstanceIds"
      :selected-instance-id="null"
      :allow-clear="true"
      :template-item-ids="allMediumTemplateIds"
      :template-items-title="t('game.inventorySelectorAvailableItemsTitle', { title: t('alchemy.bench.medium') })"
      :empty-text="t('alchemy.bench.noMediums')"
      @confirm="handleSelectMedium"
    />

    <GameDialog v-model="mediumCodexVisible" :title="t('alchemy.bench.mediumCodexTitle')" width="860px">
      <AlchemyCodexPanel :show-families="false" />
    </GameDialog>

    <GameDialog v-model="fishSelectorVisible" :title="t('alchemy.bench.fish.pick')" width="760px">
      <p class="alchemy-fish-picker-count">{{ t("alchemy.bench.fish.selectedCount", { count: selectedFish.length, max: 4 }) }}</p>
      <div v-if="fishInventory.length" class="alchemy-fish-picker">
        <button
          v-for="fish in fishInventory"
          :key="fish.instanceId"
          type="button"
          :class="{ 'is-selected': selectedFishInstanceIds.includes(fish.instanceId) }"
          @click="toggleFish(fish.instanceId)"
        >
          <FishMutationArt :src="fishSpeciesIcon(fish)" :fish="fish.fishData" width="72px" />
          <span>
            <strong>{{ fishSpeciesName(fish) }}</strong>
            <small>{{ formatFishLength(fish) }}<FishSizeIndicator metric="length" :value="fish.fishData?.lengthCm || 0" :species="fishSpeciesDefinition(fish)" /> · {{ formatFishWeight(fish) }}<FishSizeIndicator metric="weight" :value="fish.fishData?.weightKg || 0" :species="fishSpeciesDefinition(fish)" /></small>
          </span>
          <el-icon v-if="selectedFishInstanceIds.includes(fish.instanceId)"><Check /></el-icon>
        </button>
      </div>
      <el-empty v-else :description="t('alchemy.bench.fish.empty')" />
      <template #footer>
        <el-button type="primary" @click="fishSelectorVisible = false">{{ t("game.confirm") }}</el-button>
      </template>
    </GameDialog>
  </div>
</template>

<script setup lang="ts">
import { computed, reactive, ref, watch } from "vue"
import { useI18n } from "vue-i18n"
import { Icon } from "@iconify/vue"
import { Check, Plus, QuestionFilled } from "@element-plus/icons-vue"
import { ElMessage } from "element-plus"
import { useInventoryStore } from "@stores/inventory"
import { usePlayerStore } from "@stores/player"
import { useGameStore } from "@stores/game"
import {
  DEFAULT_BENCH_MATERIAL_SLOTS,
  computeAlchemyLevelDeficitFailDelta,
  computeComboFailChanceBreakdown,
  computeFailChanceBreakdown,
  computeTraitMaxQuantity,
  computeTraitQuantityCeiling,
  effectiveBenchSlots,
  fetchFishingInfusionPreview,
  getAlchemyMediumByItemId,
  getAlchemyTraitForItem,
  getItemAlchemyNameAffix,
  listAlchemyComboRecipes,
  listAlchemyMediums,
  listAlchemyTraits,
  type AlchemyMedium,
  type AlchemyComboRecipe,
  type AlchemyTrait,
  type AlchemyTraitEffect,
  type FailChanceBreakdown,
  type FishingInfusionPreview
} from "@services/alchemy-data"
import { formatAttributeValueByKey, getAttributeUnit } from "@services/attribute-display"
import { getItemDefinition, isCarrierItemId } from "@services/item-definition"
import { infusionQuantityLimits, applyInfusionOperationRange } from "@services/alchemy-operation-effects"
import { ALCHEMY_CONFIG } from "@/config/shared-global-config"
import { formatDurationI18n } from "@/utils/duration-format"
import Card from "@components/game/item/Card.vue"
import EmptyCard from "@components/game/item/EmptyCard.vue"
import InventoryInstanceSelectorDialog from "@components/game/inventory/InventoryInstanceSelectorDialog.vue"
import StatusRequireTag from "@components/game/attribute/StatusRequireTag.vue"
import FailBreakdown from "@components/game/alchemy/FailBreakdown.vue"
import InfusionFoodSuccessHint from "@components/game/alchemy/InfusionFoodSuccessHint.vue"
import AlchemyFamilyPopoverContent from "@components/game/alchemy/AlchemyFamilyPopoverContent.vue"
import AlchemyCodexPanel from "@components/game/alchemy/AlchemyCodexPanel.vue"
import AlchemyActionSubmitBar from "./AlchemyActionSubmitBar.vue"
import GameDialog from "@components/game/common/GameDialog.vue"
import FishMutationArt from "@components/game/fishing/FishMutationArt.vue"
import FishSizeIndicator from "@components/game/fishing/FishSizeIndicator.vue"
import { getGameDataChunk } from "@services/game-data-runtime"
import { useOperationEffects } from "@/composables/game/useOperationEffects"
import OperationEffectNotice from "@components/game/operation-effect/OperationEffectNotice.vue"
import OperationEffectAdjustment from "@components/game/operation-effect/OperationEffectAdjustment.vue"
import { applyInfusionFoodFailureChance } from "@services/alchemy-operation-effects"
import type { InventoryInstance } from "@/types/equipment"

type MaterialRow = {
  itemId: string
  itemName: string
  availableQuantity: number
  maxQty: number
  qtyValid: boolean
  levelMet: boolean
  successPct: number
  failBreakdown: FailChanceBreakdown
  trait: AlchemyTrait
}

type AggregatedAttrEntry = {
  attr: string
  baseMin: number
  baseMax: number
  derivedMin: number
  derivedMax: number
  min: number
  max: number
}

type RangeTooltipLine = {
  label: string
  value: string
}

type ComboMatch = {
  combo: AlchemyComboRecipe
  multiplier: number
}

type ComboEffectRow = {
  key: string
  label: string
  valueText: string
  meta?: string
  tone: "positive" | "negative" | "neutral"
}

type ComboPowerRange = {
  baseMin: number
  baseMax: number
  min: number
  max: number
}

type BenchMode = "materials" | "fish"

type FishingSpeciesDisplay = {
  id: string
  names: Record<string, string>
  iconPath: string
  minLengthCm: number
  maxLengthCm: number
  shapeFactor: number
}

const emit = defineEmits<{
  (e: "submitted"): void
}>()
const props = withDefaults(defineProps<{
  comboFillRequest?: { id: string; nonce: number } | null
}>(), {
  comboFillRequest: null
})

const { t, locale } = useI18n()
const inventoryStore = useInventoryStore()
const playerStore = usePlayerStore()
const gameStore = useGameStore()
const { alchemyInfusionModifiers: availableInfusionModifiers, alchemyInfusionModifiersForMaterials } = useOperationEffects()
const infusionMaterialRarities = computed(() =>
  Object.entries(materials).filter(([, qty]) => qty > 0).map(([id]) => getItemDefinition(id)?.rarity || "")
)
const alchemyInfusionModifiers = computed(() => alchemyInfusionModifiersForMaterials(infusionMaterialRarities.value))
const materialQuickStepCounts = [5, 10, 20] as const

const benchMode = ref<BenchMode>("materials")
const benchModeOptions = computed(() => [
  { label: t("alchemy.bench.modeMaterials"), value: "materials" },
  { label: t("alchemy.bench.modeFish"), value: "fish" }
])
const selectedFishInstanceIds = ref<number[]>([])
const fishSelectorVisible = ref(false)
const fishPreviews = ref<Record<number, FishingInfusionPreview>>({})
const fishPreviewLoading = ref(false)
const fishPreviewError = ref("")
let fishPreviewRequest = 0

const selectedCarrierInstanceId = ref<number | null>(null)
const selectedMediumItemId = ref<string | null>(null)
const carrierSelectorVisible = ref(false)
const materialSelectorVisible = ref(false)
const mediumSelectorVisible = ref(false)
const mediumCodexVisible = ref(false)
const materials = reactive<Record<string, number>>({})
const selectedSlots = ref<(string | null)[]>(Array(DEFAULT_BENCH_MATERIAL_SLOTS).fill(null))
const activeSlotIndex = ref<number | null>(null)
const benchSubmitCount = ref(1)

const playerAlchemyLevel = computed(() => Number(playerStore.attributes?.alchemy_level?.value ?? 1))
const playerAlchemyStability = computed(() => Number(playerStore.attributes?.alchemy_craft_success?.value ?? 0))
const playerAlchemyPrecision = computed(() => Number(playerStore.attributes?.alchemy_precision?.value ?? 0))

const fishingSpecies = readFishingSpecies()
const fishingSpeciesById = new Map(fishingSpecies.map((row) => [row.id, row]))
const fishInventory = computed(() => inventoryStore.instances.filter((item) =>
  item.state === "inventory" &&
  item.quantity === 1 &&
  item.templateItemId === "fish_catch" &&
  Boolean(item.fishData?.speciesId)
))
const selectedFish = computed(() => selectedFishInstanceIds.value
  .map((instanceId) => fishInventory.value.find((row) => row.instanceId === instanceId))
  .filter((row): row is (typeof fishInventory.value)[number] => Boolean(row)))
const requiredScaleglow = computed(() => Math.max(1, selectedFish.value.length))
const fishPreviewEntries = computed(() => selectedFish.value.map((fish) => ({
  fish,
  preview: fishPreviews.value[fish.instanceId] || null
})))
const primaryFishPreview = computed(() => fishPreviewEntries.value.find((entry) => entry.preview)?.preview || null)
const scaleglowQuantity = computed(() => inventoryStore.instances
  .filter((row) => row.state === "inventory" && row.templateItemId === "scaleglow_medium")
  .reduce((total, row) => total + Math.max(0, Number(row.quantity || 0)), 0))

// --- Carrier ----------------------------------------------------------------
const carriers = computed(() =>
  inventoryStore.instances.filter(
    (item) =>
      item.state === "inventory" &&
      item.quantity > 0 &&
      isCarrierItemId(item.templateItemId) &&
      (item.affixes?.length ?? 0) === 0
  )
)
const carrierInstanceIds = computed(() => carriers.value.map((row) => row.instanceId))
const carrierTemplateItemIds = computed(() => uniqueTemplateIds(carriers.value))
const selectedCarrier = computed(() => carriers.value.find((row) => row.instanceId === selectedCarrierInstanceId.value) || null)

// --- Medium -----------------------------------------------------------------
const allMediumDefs = computed(() => listAlchemyMediums())
const availableMediums = computed(() =>
  inventoryStore.instances.filter((item) => {
    if (item.state !== "inventory" || item.quantity <= 0) return false
    const def = allMediumDefs.value.find((m) => m.itemId === item.templateItemId)
    return !!def
  })
)
const availableMediumInstanceIds = computed(() => availableMediums.value.map((row) => row.instanceId))
const allMediumTemplateIds = computed(() => allMediumDefs.value.map((medium) => medium.itemId))
const selectedMedium = computed<AlchemyMedium | null>(() =>
  selectedMediumItemId.value ? (getAlchemyMediumByItemId(selectedMediumItemId.value) || null) : null
)
const selectedMediumOwnedQty = computed(() => {
  const itemId = selectedMedium.value?.itemId
  if (!itemId) return 0
  return inventoryStore.instances
    .filter((row) => row.state === "inventory" && row.templateItemId === itemId)
    .reduce((acc, row) => acc + row.quantity, 0)
})
const effectiveSlots = computed(() => effectiveBenchSlots(selectedMedium.value))

// Resize slot array when slot count changes (preserve existing selections in order).
watch(effectiveSlots, (next, prev) => {
  if (next === prev) return
  const existing = selectedSlots.value.filter(Boolean) as string[]
  const truncated = existing.slice(0, next)
  selectedSlots.value = [...truncated, ...Array(Math.max(0, next - truncated.length)).fill(null)]
  if (activeSlotIndex.value !== null && activeSlotIndex.value >= next) activeSlotIndex.value = null
}, { immediate: false })

// --- Materials --------------------------------------------------------------
type TraitEntry = { itemId: string; itemName: string; availableQuantity: number; trait: AlchemyTrait }

const traitMaterials = computed<TraitEntry[]>(() => {
  const map = new Map<string, TraitEntry>()
  for (const item of inventoryStore.instances) {
    if (item.state !== "inventory" || item.quantity <= 0) continue
    const trait = getAlchemyTraitForItem(item.templateItemId)
    if (!trait) continue
    const existing = map.get(item.templateItemId)
    if (existing) {
      existing.availableQuantity += item.quantity
    } else {
      map.set(item.templateItemId, {
        itemId: item.templateItemId,
        itemName: t(`item.${item.templateItemId}.name`, item.templateItemId),
        availableQuantity: item.quantity,
        trait
      })
    }
  }
  return Array.from(map.values())
})

function buildMaterialRow(entry: TraitEntry): MaterialRow {
  const trait = entry.trait
  const maxQty = materialMaxQuantity(entry)
  const qty = Math.max(0, Math.trunc(Number(materials[entry.itemId] || 0)))
  const basis = materialUnitBasis(trait)
  const qtyValid = qty >= basis && qty <= maxQty && qty % basis === 0
  const levelMet = playerAlchemyLevel.value >= trait.minAlchemyLevel
  const failBreakdown = withFoodFailureModifiers(computeFailChanceBreakdown(trait, playerAlchemyLevel.value, playerAlchemyStability.value, selectedMedium.value, qty))
  return {
    itemId: entry.itemId,
    itemName: entry.itemName,
    availableQuantity: entry.availableQuantity,
    maxQty,
    qtyValid,
    levelMet,
    successPct: 1 - failBreakdown.final,
    failBreakdown,
    trait
  }
}

function materialUnitBasis(trait: AlchemyTrait): number {
  return Math.max(1, Math.trunc(Number(trait.unitBasis) || 1))
}

function alignMaterialQuantity(value: number, basis: number): number {
  const normalizedBasis = Math.max(1, Math.trunc(Number(basis) || 1))
  const qty = Math.max(0, Math.trunc(Number(value) || 0))
  return Math.floor(qty / normalizedBasis) * normalizedBasis
}

function materialLimits(entry: TraitEntry) {
  const baseLimit = computeTraitMaxQuantity(entry.trait, playerAlchemyStability.value)
  return infusionQuantityLimits(getItemDefinition(entry.itemId)?.rarity || "", baseLimit, entry.availableQuantity, materialUnitBasis(entry.trait), availableInfusionModifiers.value)
}

function materialMaxQuantity(entry: TraitEntry): number {
  return materialLimits(entry).selectable
}

function materialCapacityDetail(row: MaterialRow): string {
  const limits = materialLimits(row)
  return t('game.fishCuisine.controls.capacityDetail', { base: limits.baseCapacity, capacity: limits.capacity, bonus: limits.bonus, basis: limits.basis })
}

function materialBreakthroughText(row: MaterialRow): string {
  const base = computeTraitQuantityCeiling(row.trait)
  const boosted = infusionQuantityLimits(getItemDefinition(row.itemId)?.rarity || "", base, 0, materialUnitBasis(row.trait), availableInfusionModifiers.value).capacity
  return t('game.fishCuisine.controls.breakthrough', { base, boosted })
}

function normalizeMaterialInput(row: MaterialRow): number {
  const basis = materialUnitBasis(row.trait)
  const maxQty = alignMaterialQuantity(row.maxQty, basis)
  if (maxQty < basis) return 0
  const current = Math.max(basis, Math.trunc(Number(materials[row.itemId] || 0)))
  return Math.min(maxQty, alignMaterialQuantity(current, basis))
}

const slotEntries = computed<MaterialRow[]>(() => {
  const emptyTrait = (itemId = ""): AlchemyTrait => ({
    id: "",
    itemId,
    nameKey: "",
    kind: "",
    family: "",
    effectType: "",
    effects: [],
    unitBasis: 1,
    maxBasisMultiplier: 1,
    minAlchemyLevel: 1,
    failBaseChance: 0,
    failChancePerLevelAbove: 0,
    failChancePerExtraBasis: 0,
    failChanceQuantityCap: 0,
    maxEffectivePerEquipment: 2,
    maxEffectivePerBody: 5,
    goldCostPerUnit: 0,
    durationSecPerUnit: 0,
    alchemyExpPerUnit: 0,
    active: false
  })
  return selectedSlots.value.map((id) => {
    if (!id) return { itemId: "", itemName: "", availableQuantity: 0, maxQty: 0, qtyValid: false, levelMet: true, successPct: 1, failBreakdown: { base: 0, levelDelta: 0, quantityDelta: 0, craftSuccessDelta: 0, mediumDelta: 0, successFloorClamp: 0, final: 0 }, trait: emptyTrait() }
    const entry = traitMaterials.value.find((m) => m.itemId === id)
    if (!entry) return { itemId: id, itemName: id, availableQuantity: 0, maxQty: 0, qtyValid: false, levelMet: true, successPct: 1, failBreakdown: { base: 0, levelDelta: 0, quantityDelta: 0, craftSuccessDelta: 0, mediumDelta: 0, successFloorClamp: 0, final: 0 }, trait: emptyTrait(id) }
    return buildMaterialRow(entry)
  })
})

// Available pool for "pick material" dialog: not already used in another slot,
// and stackable count is enough for at least one basis unit.
const availableMaterials = computed<MaterialRow[]>(() => {
  const usedSet = new Set(selectedSlots.value.filter(Boolean))
  return traitMaterials.value
    .filter((row) => !usedSet.has(row.itemId))
    .map(buildMaterialRow)
    .filter((row) => row.maxQty >= row.trait.unitBasis)
})
const availableMaterialInstanceIds = computed(() => {
  const allowed = new Set(availableMaterials.value.map((row) => row.itemId))
  const ids: number[] = []
  for (const item of inventoryStore.instances) {
    if (item.state !== "inventory" || item.quantity <= 0) continue
    if (allowed.has(item.templateItemId)) ids.push(item.instanceId)
  }
  return ids
})
const availableMaterialTemplateIds = computed(() => listAlchemyTraits().map((trait) => trait.itemId))

// --- Composed state ---------------------------------------------------------
const filledRows = computed(() => slotEntries.value.filter((row) => !!row.itemId))
watch(
  () => slotEntries.value.map((row) => `${row.itemId}:${row.maxQty}:${materials[row.itemId] || 0}`).join("|"),
  () => {
    for (const row of slotEntries.value) {
      if (!row.itemId) continue
      const normalized = normalizeMaterialInput(row)
      if (normalized > 0 && normalized !== materials[row.itemId]) {
        materials[row.itemId] = normalized
      }
    }
  },
  { flush: "post" }
)
const selectedMaterialMap = computed(() => {
  const out: Record<string, number> = {}
  for (const row of filledRows.value) {
    if (!row.qtyValid) continue
    const qty = Math.trunc(Number(materials[row.itemId] || 0))
    if (qty > 0) out[row.itemId] = qty
  }
  return out
})
const selectedMaterialCount = computed(() => Object.keys(selectedMaterialMap.value).length)
const selectedComboMatch = computed<ComboMatch | null>(() => {
  const matches = listAlchemyComboRecipes()
    .map((combo) => {
      const multiplier = materialMapRatio(selectedMaterialMap.value, combo.inputs)
      return multiplier > 0 ? { combo, multiplier } : null
    })
    .filter((row): row is ComboMatch => !!row)
    .sort((a, b) => b.combo.priority - a.combo.priority || a.combo.id.localeCompare(b.combo.id))
  return matches[0] || null
})
const selectedCombo = computed(() => selectedComboMatch.value?.combo || null)
const selectedComboMultiplier = computed(() => selectedComboMatch.value?.multiplier || 0)
const selectedComboMaxMultiplier = computed(() => selectedCombo.value ? maxComboMultiplier(selectedCombo.value) : 1)
const selectedComboSummary = computed(() => {
  const combo = selectedCombo.value
  if (!combo) return ""
  const key = combo.summaryKey || combo.descKey
  const localized = t(key, { multiplier: selectedComboMultiplier.value })
  return localized === key ? t(combo.descKey) : localized
})
const selectedComboPowerRange = computed<ComboPowerRange | null>(() => {
  const combo = selectedCombo.value
  if (!combo) return null
  return comboPowerRange(combo, selectedComboMultiplier.value)
})
const selectedComboEffectRows = computed<ComboEffectRow[]>(() => {
  const combo = selectedCombo.value
  const power = selectedComboPowerRange.value
  if (!combo || !power) return []
  const rows: ComboEffectRow[] = [{
    key: "combo-power",
    label: t("alchemy.bench.comboPower"),
    valueText: formatNumberRange(power.min, power.max, 2),
    meta: t("alchemy.bench.comboPowerMeta", { base: formatNumberRange(power.baseMin, power.baseMax, 2) }),
    tone: "neutral"
  }]
  rows.push(...comboTemplateEffectRows(combo.outputAffix?.params, power, selectedComboMultiplier.value))
  return rows
})
const previewName = computed(() => {
  const selected = filledRows.value.filter((row) => (selectedMaterialMap.value[row.itemId] || 0) > 0)
  if (selected.length === 0) return t("alchemy.bench.noPreview")
  const affixes = selected.map((row) => getItemAlchemyNameAffix(row.itemId))
  const prefixes = affixes.map((affix) => localizeKey(affix?.prefixKey || "")).filter(Boolean)
  const suffix = localizeKey(affixes[0]?.suffixKey || "")
  return t("alchemy.affix.generatedName", { prefix: prefixes.join(t("alchemy.affix.prefixSeparator")), suffix })
})
const aggregatedSuccessPct = computed(() => {
  if (selectedCombo.value) {
    return 1 - overallFailBreakdown.value.final
  }
  const selected = filledRows.value.filter((row) => (selectedMaterialMap.value[row.itemId] || 0) > 0)
  if (selected.length === 0) return 1
  return Math.min(...selected.map((row) => row.successPct))
})
const selectedSuccessRows = computed(() => filledRows.value.filter((row) => (selectedMaterialMap.value[row.itemId] || 0) > 0))
const overallFailBreakdown = computed<FailChanceBreakdown>(() => {
  if (selectedCombo.value) {
    return withFoodFailureModifiers(computeComboFailChanceBreakdown(selectedCombo.value, playerAlchemyLevel.value, playerAlchemyStability.value, selectedMedium.value, comboMaterialLevelDeficitDelta.value))
  }
  const selected = selectedSuccessRows.value
  if (selected.length === 0) return { base: 0, levelDelta: 0, quantityDelta: 0, craftSuccessDelta: 0, mediumDelta: 0, successFloorClamp: 0, final: 0 }
  return selected.reduce((worst, row) => row.failBreakdown.final >= worst.final ? row.failBreakdown : worst, selected[0].failBreakdown)
})
const comboMaterialLevelDeficitDelta = computed(() => {
  if (!selectedCombo.value) return 0
  let delta = 0
  for (const row of filledRows.value) {
    const next = computeAlchemyLevelDeficitFailDelta(playerAlchemyLevel.value, row.trait.minAlchemyLevel)
    if (next > delta) delta = next
  }
  return delta
})

// Aggregate attribute preview across all selected materials (with medium multipliers applied).
const aggregatedAttrEntries = computed<AggregatedAttrEntry[]>(() => {
  if (selectedCombo.value) return comboAttrPreviewEntries(selectedCombo.value, selectedComboMultiplier.value)
  const acc = new Map<string, Omit<AggregatedAttrEntry, "attr">>()
  for (const row of filledRows.value) {
    const qty = selectedMaterialMap.value[row.itemId] || 0
    if (qty <= 0) continue
    for (const effect of row.trait.effects) {
      const { posMult, negMult } = effectMultipliers(row.trait.family)
      const baseMinRaw = effect.valuePerUnit.min * qty
      const baseMaxRaw = effect.valuePerUnit.max * qty
      const baseLo = Math.min(baseMinRaw, baseMaxRaw)
      const baseHi = Math.max(baseMinRaw, baseMaxRaw)
      const [derivedMin, derivedMax] = applyAlchemyDerivedRange(effect.valuePerUnit.min * qty, effect.valuePerUnit.max * qty)
      const derivedLo = Math.min(derivedMin, derivedMax)
      const derivedHi = Math.max(derivedMin, derivedMax)
      const [rawMin, rawMax] = applyAlchemyInfusionOperationRange(derivedMin, derivedMax)
      const min = roundAlchemyAttributeValue(effect.attr, rawMin >= 0 ? rawMin * posMult : rawMin * negMult)
      const max = roundAlchemyAttributeValue(effect.attr, rawMax >= 0 ? rawMax * posMult : rawMax * negMult)
      const lo = Math.min(min, max)
      const hi = Math.max(min, max)
      const existing = acc.get(effect.attr)
      if (existing) {
        existing.baseMin += baseLo
        existing.baseMax += baseHi
        existing.derivedMin += derivedLo
        existing.derivedMax += derivedHi
        existing.min += lo
        existing.max += hi
      } else {
        acc.set(effect.attr, { baseMin: baseLo, baseMax: baseHi, derivedMin: derivedLo, derivedMax: derivedHi, min: lo, max: hi })
      }
    }
  }
  return Array.from(acc.entries())
    .map(([attr, range]) => ({
      attr,
      ...range,
      min: roundAlchemyAttributeValue(attr, range.min),
      max: roundAlchemyAttributeValue(attr, range.max)
    }))
    .sort((a, b) => Math.abs(b.max) - Math.abs(a.max))
})

const positiveAttrEntries = computed(() => aggregatedAttrEntries.value.filter((entry) => entry.max > 0))
const negativeAttrEntries = computed(() => aggregatedAttrEntries.value.filter((entry) => entry.max <= 0))
const infusionGoldCost = computed(() => {
  let total = ALCHEMY_CONFIG.infusionBaseGoldCost
  for (const row of filledRows.value) {
    const qty = selectedMaterialMap.value[row.itemId] || 0
    total += qty * row.trait.goldCostPerUnit
  }
  if (selectedMedium.value) {
    const value = Number(getItemDefinition(selectedMedium.value.itemId)?.economyBaseValue || 0)
    total += Math.ceil(Math.sqrt(Math.max(0, value))) * selectedMedium.value.consumePerCraft
  }
  if (selectedCombo.value) {
    total += Math.max(0, Math.round(selectedCombo.value.extraGoldCost || 0)) * Math.max(1, selectedComboMultiplier.value)
  }
  return Math.max(0, Math.round(total))
})
const infusionMediumGoldCost = computed(() => {
  if (!selectedMedium.value) return 0
  const value = Number(getItemDefinition(selectedMedium.value.itemId)?.economyBaseValue || 0)
  return Math.ceil(Math.sqrt(Math.max(0, value))) * selectedMedium.value.consumePerCraft
})
const infusionDurationSec = computed(() => {
  let total = ALCHEMY_CONFIG.infusionBaseDurationSec
  for (const row of filledRows.value) {
    const qty = selectedMaterialMap.value[row.itemId] || 0
    total += qty * row.trait.durationSecPerUnit
  }
  if (selectedCombo.value) {
    total += Math.max(0, selectedCombo.value.extraDurationSec || 0) * Math.max(1, selectedComboMultiplier.value)
  }
  return Math.max(0, total)
})
const infusionExp = computed(() => {
  let total = ALCHEMY_CONFIG.infusionBaseExp
  for (const row of filledRows.value) {
    const qty = selectedMaterialMap.value[row.itemId] || 0
    total += qty * row.trait.alchemyExpPerUnit
  }
  return Math.max(0, total)
})
const benchMaxSubmitCount = computed(() => {
  if (!selectedCarrier.value) return 1
  const limits: number[] = [Math.max(1, selectedCarrier.value.quantity || 1)]
  for (const row of filledRows.value) {
    const qty = selectedMaterialMap.value[row.itemId] || 0
    if (qty > 0) limits.push(Math.floor(row.availableQuantity / qty))
  }
  if (selectedMedium.value) {
    limits.push(Math.floor(selectedMediumOwnedQty.value / Math.max(1, selectedMedium.value.consumePerCraft)))
  }
  return Math.max(1, Math.min(...limits.filter((value) => Number.isFinite(value) && value > 0)))
})
const benchSubmitArgs = computed(() => ({
  carrierInstanceId: selectedCarrierInstanceId.value || 0,
  carrierItemId: selectedCarrier.value?.templateItemId || "",
  materials: selectedMaterialMap.value,
  ...(selectedCombo.value ? { comboRecipeId: selectedCombo.value.id } : {}),
  ...(selectedMedium.value ? { mediumItemId: selectedMedium.value.itemId } : {})
}))
const benchGoldBreakdown = computed(() => {
  const rows = [
    { label: t("alchemy.bench.baseGold"), value: t("alchemy.cost.gold", { value: ALCHEMY_CONFIG.infusionBaseGoldCost }) },
  ]
  for (const row of filledRows.value) {
    const qty = selectedMaterialMap.value[row.itemId] || 0
    if (qty > 0) rows.push({ label: row.itemName, value: t("alchemy.cost.gold", { value: qty * row.trait.goldCostPerUnit }) })
  }
  if (selectedMedium.value) {
    rows.push({ label: t(selectedMedium.value.nameKey), value: t("alchemy.cost.gold", { value: infusionMediumGoldCost.value }) })
  }
  if (selectedCombo.value && selectedCombo.value.extraGoldCost > 0) {
    rows.push({ label: t("alchemy.bench.comboExtraGold"), value: t("alchemy.cost.gold", { value: selectedCombo.value.extraGoldCost * Math.max(1, selectedComboMultiplier.value) }) })
  }
  rows.push({ label: t("alchemy.bench.submitCount"), value: `x${benchSubmitCount.value}` })
  return rows
})
const benchDurationBreakdown = computed(() => {
  const rows = [{ label: t("alchemy.bench.baseDuration"), value: formatSeconds(ALCHEMY_CONFIG.infusionBaseDurationSec) }]
  for (const row of filledRows.value) {
    const qty = selectedMaterialMap.value[row.itemId] || 0
    if (qty > 0) rows.push({ label: row.itemName, value: formatSeconds(qty * row.trait.durationSecPerUnit) })
  }
  if (selectedCombo.value && selectedCombo.value.extraDurationSec > 0) {
    rows.push({ label: t("alchemy.bench.comboExtraDuration"), value: formatSeconds(selectedCombo.value.extraDurationSec * Math.max(1, selectedComboMultiplier.value)) })
  }
  rows.push({ label: t("alchemy.bench.submitCount"), value: `x${benchSubmitCount.value}` })
  return rows
})
const benchExpBreakdown = computed(() => {
  const rows = [{ label: t("alchemy.bench.baseExp"), value: formatNumber(ALCHEMY_CONFIG.infusionBaseExp, 2) }]
  for (const row of filledRows.value) {
    const qty = selectedMaterialMap.value[row.itemId] || 0
    if (qty > 0) rows.push({ label: row.itemName, value: formatNumber(qty * row.trait.alchemyExpPerUnit, 2) })
  }
  rows.push({ label: t("alchemy.bench.submitCount"), value: `x${benchSubmitCount.value}` })
  return rows
})

const fishValidationIssues = computed<string[]>(() => {
  const issues: string[] = []
  if (!selectedCarrier.value) issues.push(t("alchemy.bench.fish.issueCarrier"))
  if (!selectedFish.value.length) issues.push(t("alchemy.bench.fish.issueFish"))
  if (scaleglowQuantity.value < requiredScaleglow.value) {
    issues.push(t("alchemy.bench.fish.issueMediumCount", { count: requiredScaleglow.value }))
  }
  if (selectedFish.value.length && !fishPreviewLoading.value && fishPreviewEntries.value.some((entry) => !entry.preview)) {
    issues.push(fishPreviewError.value || t("alchemy.bench.fish.issuePreview"))
  }
  issues.push(t("alchemy.bench.fish.featurePending"))
  return issues
})

const fishSubmitArgs = computed(() => ({
  carrierInstanceId: selectedCarrierInstanceId.value || 0,
  fishInstanceIds: [...selectedFishInstanceIds.value]
}))
const activeSubmitArgs = computed(() => benchMode.value === "fish" ? fishSubmitArgs.value : benchSubmitArgs.value)
const activeSubmitCount = computed(() => benchMode.value === "fish" ? 1 : benchSubmitCount.value)
const activeMaxSubmitCount = computed(() => benchMode.value === "fish" ? 1 : benchMaxSubmitCount.value)
const activeDurationSec = computed(() => benchMode.value === "fish" ? ALCHEMY_CONFIG.infusionBaseDurationSec : infusionDurationSec.value)
const activeGoldCost = computed(() => benchMode.value === "fish"
  ? fishPreviewEntries.value.reduce((total, entry) => total + Number(entry.preview?.goldCost || ALCHEMY_CONFIG.infusionBaseGoldCost), 0)
  : infusionGoldCost.value)
const activeExpReward = computed(() => benchMode.value === "fish" ? ALCHEMY_CONFIG.infusionBaseExp : infusionExp.value)
const activeSuccessRate = computed(() => benchMode.value === "fish"
  ? (primaryFishPreview.value ? Math.min(...fishPreviewEntries.value.filter((entry) => entry.preview).map((entry) => 1 - Number(entry.preview!.failChance || 0))) : null)
  : aggregatedSuccessPct.value)
const activeCanSubmit = computed(() => benchMode.value === "fish" ? false : canSubmit.value)
const activeGoldBreakdown = computed(() => benchMode.value === "fish"
  ? [{ label: t("alchemy.bench.fish.singleInfusion"), value: t("alchemy.cost.gold", { value: activeGoldCost.value }) }]
  : benchGoldBreakdown.value)
const activeDurationBreakdown = computed(() => benchMode.value === "fish"
  ? [{ label: t("alchemy.bench.fish.singleInfusion"), value: formatSeconds(activeDurationSec.value) }]
  : benchDurationBreakdown.value)
const activeExpBreakdown = computed(() => benchMode.value === "fish"
  ? [{ label: t("alchemy.bench.fish.singleInfusion"), value: formatNumber(activeExpReward.value, 2) }]
  : benchExpBreakdown.value)
const activeSuccessBreakdown = computed(() => benchMode.value === "fish" && primaryFishPreview.value
  ? [
      { label: t("alchemy.bench.fish.success"), value: formatPercent(activeSuccessRate.value || 0) },
      { label: t("alchemy.bench.fish.selectedCount", { count: selectedFish.value.length, max: 4 }), value: t("alchemy.bench.fish.featurePendingShort") }
    ]
  : [])

function updateActiveSubmitCount(value: number) {
  if (benchMode.value === "materials") benchSubmitCount.value = value
}

// --- Validation issue list (single source of truth for canSubmit) -----------
const validationIssues = computed<string[]>(() => {
  const issues: string[] = []
  if (!selectedCarrier.value) issues.push(t("alchemy.bench.issue.noCarrier"))
  const filled = filledRows.value
  if (filled.length === 0) {
    issues.push(t("alchemy.bench.issue.noMaterial"))
  }
  if (filled.length > 0 && filled.length < effectiveSlots.value) {
    issues.push(t("alchemy.bench.issue.materialSlotsNotFull", { used: filled.length, max: effectiveSlots.value }))
  }
  for (const row of filled) {
    if (!row.levelMet) {
      issues.push(t("alchemy.bench.issue.materialLevel", { name: row.itemName, level: row.trait.minAlchemyLevel }))
    }
    if (!row.qtyValid) {
      issues.push(t("alchemy.bench.issue.materialQty", { name: row.itemName, step: row.trait.unitBasis, max: row.maxQty }))
    }
  }
  if (selectedMedium.value && playerAlchemyLevel.value < selectedMedium.value.minAlchemyLevel) {
    issues.push(t("alchemy.bench.issue.mediumLevel", { level: selectedMedium.value.minAlchemyLevel }))
  }
  // Kind consistency
  const kinds = new Set(filled.map((row) => row.trait.kind))
  if (!selectedCombo.value && kinds.size > 1) issues.push(t("alchemy.bench.issue.mixedKind"))
  return issues
})

const canSubmit = computed(() => validationIssues.value.length === 0)

// --- Watches ----------------------------------------------------------------
watch(carrierInstanceIds, (ids) => {
  if (selectedCarrierInstanceId.value === null && ids.length > 0) {
    selectedCarrierInstanceId.value = ids[0]
  } else if (selectedCarrierInstanceId.value !== null && !ids.includes(selectedCarrierInstanceId.value)) {
    selectedCarrierInstanceId.value = ids[0] || null
  }
}, { immediate: true })

watch(benchMaxSubmitCount, (maxCount) => {
  if (benchSubmitCount.value > maxCount) benchSubmitCount.value = maxCount
})

watch(fishInventory, (items) => {
  const available = new Set(items.map((row) => row.instanceId))
  selectedFishInstanceIds.value = selectedFishInstanceIds.value.filter((instanceId) => available.has(instanceId))
})

watch(
  [selectedFishInstanceIds, () => gameStore.token],
  () => { void refreshFishPreviews() },
  { immediate: true, deep: true }
)

watch(
  () => props.comboFillRequest,
  (request) => {
    if (!request?.id) return
    const combo = listAlchemyComboRecipes().find((row) => row.id === request.id)
    if (!combo) {
      ElMessage.warning(t("alchemy.bench.comboFillUnknown"))
      return
    }
    applyComboToMaterials(combo, 1)
  },
  { deep: true, immediate: true }
)

// --- Handlers ---------------------------------------------------------------
function handleSelectCarrier(instanceId: number | null) {
  selectedCarrierInstanceId.value = instanceId
}

function handleSelectMedium(instanceId: number | null) {
  if (instanceId === null) {
    selectedMediumItemId.value = null
    return
  }
  const inst = inventoryStore.instances.find((row) => row.instanceId === instanceId)
  if (inst) selectedMediumItemId.value = inst.templateItemId
}

function clearMedium() {
  selectedMediumItemId.value = null
}

function toggleFish(instanceId: number) {
  if (selectedFishInstanceIds.value.includes(instanceId)) {
    selectedFishInstanceIds.value = selectedFishInstanceIds.value.filter((id) => id !== instanceId)
    return
  }
  if (selectedFishInstanceIds.value.length >= 4) {
    ElMessage.warning(t("alchemy.bench.fish.maxSelected", { count: 4 }))
    return
  }
  selectedFishInstanceIds.value = [...selectedFishInstanceIds.value, instanceId]
}

async function refreshFishPreviews() {
  const request = ++fishPreviewRequest
  const fishInstanceIds = [...selectedFishInstanceIds.value]
  fishPreviews.value = {}
  fishPreviewError.value = ""
  if (!fishInstanceIds.length || !gameStore.token) {
    fishPreviewLoading.value = false
    return
  }
  fishPreviewLoading.value = true
  try {
    const previews = await Promise.all(fishInstanceIds.map(async (fishInstanceId) => ({
      fishInstanceId,
      preview: await fetchFishingInfusionPreview(gameStore.token, fishInstanceId)
    })))
    if (request !== fishPreviewRequest) return
    fishPreviews.value = Object.fromEntries(previews.map((entry) => [entry.fishInstanceId, entry.preview]))
  } catch {
    if (request !== fishPreviewRequest) return
    fishPreviewError.value = t("alchemy.bench.fish.previewFailed")
  } finally {
    if (request === fishPreviewRequest) fishPreviewLoading.value = false
  }
}

function handleSlotClick(idx: number) {
  activeSlotIndex.value = idx
  materialSelectorVisible.value = true
}

function handleAddMaterial(instanceId: number | null) {
  if (instanceId === null) return
  const inst = inventoryStore.instances.find((row) => row.instanceId === instanceId)
  if (!inst) return
  const slotIdx = activeSlotIndex.value !== null
    ? activeSlotIndex.value
    : selectedSlots.value.findIndex((s) => !s)
  if (slotIdx < 0) return
  // Replacing a slot: discard the previous item's qty unless it's the same item.
  const prev = selectedSlots.value[slotIdx]
  if (prev && prev !== inst.templateItemId) {
    delete materials[prev]
  }
  const next = [...selectedSlots.value]
  next[slotIdx] = inst.templateItemId
  selectedSlots.value = next
  activeSlotIndex.value = slotIdx
  const entry = traitMaterials.value.find((m) => m.itemId === inst.templateItemId)
  if (entry) {
    const basis = materialUnitBasis(entry.trait)
    const maxQty = materialMaxQuantity(entry)
    materials[inst.templateItemId] = maxQty >= basis ? basis : 0
  }
}

function removeMaterial(itemId: string) {
  const idx = selectedSlots.value.indexOf(itemId)
  if (idx < 0) return
  const next = [...selectedSlots.value]
  next[idx] = null
  selectedSlots.value = next
  delete materials[itemId]
  if (activeSlotIndex.value === idx) activeSlotIndex.value = null
}

function normalizedMaterialQuantity(row: MaterialRow): number {
  return normalizeMaterialInput(row)
}

function canIncreaseMaterialBySteps(row: MaterialRow): boolean {
  const basis = materialUnitBasis(row.trait)
  return row.maxQty >= basis && normalizedMaterialQuantity(row) < row.maxQty
}

function quickStepAmount(row: MaterialRow, stepCount: number): number {
  const basis = materialUnitBasis(row.trait)
  return basis * Math.max(1, Math.trunc(Number(stepCount) || 1))
}

function quickStepHint(row: MaterialRow, stepCount: number): string {
  return String(t("alchemy.bench.quickStepHint", {
    steps: stepCount,
    qty: quickStepAmount(row, stepCount)
  }))
}

function increaseMaterialBySteps(row: MaterialRow, stepCount: number) {
  const basis = materialUnitBasis(row.trait)
  const maxQty = alignMaterialQuantity(row.maxQty, basis)
  if (maxQty < basis) return
  const current = normalizedMaterialQuantity(row)
  const next = Math.min(maxQty, Math.max(basis, current + quickStepAmount(row, stepCount)))
  materials[row.itemId] = alignMaterialQuantity(next, basis)
}

function applyComboToMaterials(combo: AlchemyComboRecipe, requestedMultiplier: number, options: { quiet?: boolean } = {}) {
  const inputEntries = comboInputEntries(combo)
  if (inputEntries.length === 0) {
    if (!options.quiet) ElMessage.warning(t("alchemy.bench.comboFillInvalid"))
    return
  }
  if (inputEntries.length !== effectiveSlots.value) {
    if (!options.quiet) ElMessage.warning(t("alchemy.bench.comboFillSlotMismatch", { need: inputEntries.length, current: effectiveSlots.value }))
    return
  }
  const maxMultiplier = maxComboMultiplier(combo)
  if (maxMultiplier <= 0) {
    if (!options.quiet) ElMessage.warning(t("alchemy.bench.comboFillInsufficient"))
    return
  }
  const multiplier = Math.max(1, Math.min(maxMultiplier, Math.floor(requestedMultiplier || 1)))
  const inputIds = inputEntries.map(([itemId]) => itemId)
  for (const itemId of Object.keys(materials)) {
    if (!inputIds.includes(itemId)) delete materials[itemId]
  }
  selectedSlots.value = inputIds.slice(0, effectiveSlots.value)
  for (const [itemId, qty] of inputEntries) {
    const entry = traitMaterials.value.find((row) => row.itemId === itemId)
    const basis = entry ? materialUnitBasis(entry.trait) : 1
    materials[itemId] = alignMaterialQuantity(qty * multiplier, basis)
  }
  activeSlotIndex.value = null
  if (options.quiet) {
    return
  }
  if (requestedMultiplier > maxMultiplier) {
    ElMessage.warning(t("alchemy.bench.comboFillClamped", { multiplier: maxMultiplier }))
  } else {
    ElMessage.success(t("alchemy.bench.comboFillSuccess", { name: t(combo.nameKey), multiplier }))
  }
}

function setComboMultiplier(combo: AlchemyComboRecipe, multiplier: number) {
  applyComboToMaterials(combo, multiplier, { quiet: true })
}

// --- Helpers ---------------------------------------------------------------
function comboInputEntries(combo: AlchemyComboRecipe): Array<[string, number]> {
  return Object.entries(combo.inputs || {})
    .map(([itemId, qty]) => [String(itemId || "").trim(), Math.trunc(Number(qty) || 0)] as [string, number])
    .filter(([itemId, qty]) => itemId !== "" && qty > 0)
}

function maxComboMultiplier(combo: AlchemyComboRecipe): number {
  const entries = comboInputEntries(combo)
  if (entries.length === 0 || entries.length !== effectiveSlots.value) return 0
  const limits = entries.map(([itemId, recipeQty]) => {
    const entry = traitMaterials.value.find((row) => row.itemId === itemId)
    if (!entry || recipeQty <= 0) return 0
    const maxQty = materialMaxQuantity(entry)
    return Math.floor(maxQty / recipeQty)
  })
  return Math.max(0, Math.min(...limits))
}

function materialMapRatio(actual: Record<string, number>, recipe: Record<string, number>): number {
  const actualKeys = Object.keys(actual).filter((key) => Number(actual[key]) > 0)
  const recipeKeys = Object.keys(recipe).filter((key) => Number(recipe[key]) > 0)
  if (actualKeys.length !== recipeKeys.length || recipeKeys.length === 0) return 0
  let ratio = 0
  for (const key of recipeKeys) {
    const actualQty = Math.trunc(Number(actual[key]) || 0)
    const recipeQty = Math.trunc(Number(recipe[key]) || 0)
    if (actualQty <= 0 || recipeQty <= 0 || actualQty % recipeQty !== 0) return 0
    const nextRatio = actualQty / recipeQty
    if (nextRatio <= 0) return 0
    if (ratio === 0) ratio = nextRatio
    else if (ratio !== nextRatio) return 0
  }
  return ratio
}

function uniqueTemplateIds(items: Array<{ templateItemId: string }>): string[] {
  const seen = new Set<string>()
  const out: string[] = []
  for (const row of items) {
    if (!seen.has(row.templateItemId)) {
      seen.add(row.templateItemId)
      out.push(row.templateItemId)
    }
  }
  return out
}

function readFishingSpecies(): FishingSpeciesDisplay[] {
  const chunk = getGameDataChunk("fishing_species") as Record<string, unknown> | null
  const rows = Array.isArray(chunk?.fishing_species) ? chunk.fishing_species : []
  return rows.flatMap((value) => {
    if (!value || typeof value !== "object" || Array.isArray(value)) return []
    const row = value as Record<string, unknown>
    const id = String(row.id || "").trim()
    if (!id) return []
    const rawNames = row.names && typeof row.names === "object" && !Array.isArray(row.names)
      ? row.names as Record<string, unknown>
      : {}
    const names = Object.fromEntries(Object.entries(rawNames).map(([key, name]) => [key, String(name || "")]))
    return [{
      id,
      names,
      iconPath: String(row.iconPath || ""),
      minLengthCm: Number(row.minLengthCm || 0),
      maxLengthCm: Number(row.maxLengthCm || 0),
      shapeFactor: Number(row.shapeFactor || 0)
    }]
  })
}

function fishSpeciesDefinition(fish: InventoryInstance): FishingSpeciesDisplay | null {
  return fishingSpeciesById.get(String(fish.fishData?.speciesId || "")) || null
}

function fishSpeciesName(fish: InventoryInstance): string {
  const species = fishSpeciesDefinition(fish)
  const language = String(locale.value || "zh-CN")
  return species?.names[language] || species?.names["en-US"] || species?.id || t("alchemy.bench.fish.unknown")
}

function fishSpeciesIcon(fish: InventoryInstance): string {
  return fishSpeciesDefinition(fish)?.iconPath || "/assets/icons/item/fish_catch.png"
}

function formatFishLength(fish: InventoryInstance): string {
  return t("game.fishing.lengthValue", { value: formatNumber(Number(fish.fishData?.lengthCm || 0), 1) })
}

function formatFishWeight(fish: InventoryInstance): string {
  return t("game.fishing.weightValue", { value: formatNumber(Number(fish.fishData?.weightKg || 0), 2) })
}

function localizeKey(key: string): string {
  const normalized = String(key || "").trim()
  if (!normalized) return ""
  const localized = t(normalized)
  return localized === normalized ? normalized : localized
}

function localizeAttr(attrKey: string): string {
  const key = `attr.${attrKey}.name`
  const localized = t(key)
  return localized === key ? attrKey : localized
}

function mutationName(id: string): string {
  const key = `game.fishing.mutation_${String(id || "").trim().toLowerCase()}`
  const localized = t(key)
  return localized === key ? id : localized
}

function localizeFamily(family: string): string {
  return t(`alchemy.family.${family}`, family)
}

function familyDescription(family: string): string {
  const key = `alchemy.familyDesc.${family}`
  const localized = t(key)
  return localized === key ? t("alchemy.familyDesc.default") : localized
}

function effectMultipliers(family: string): { posMult: number; negMult: number } {
  const m = selectedMedium.value
  let posMult = 1
  let negMult = 1
  if (m) {
    posMult = m.modifiers.valueMultiplier || 1
    negMult = m.modifiers.negativeMultiplier || 1
    if (family && m.modifiers.familyAffinity[family]) {
      posMult *= 1 + m.modifiers.familyAffinity[family]
    }
  }
  return { posMult, negMult }
}

function comboAttrPreviewEntries(combo: AlchemyComboRecipe, multiplier: number): AggregatedAttrEntry[] {
  const output = combo.outputAffix
  const attr = String(output?.params?.attr || "").trim()
  const range = output?.valueRange
  if (!attr || !range) return []
  const baseMin = Number(range.min || 0) * Math.max(1, multiplier)
  const baseMax = Number(range.max || 0) * Math.max(1, multiplier)
  const [derivedMin, derivedMax] = applyAlchemyDerivedRange(baseMin, baseMax)
  const derivedLo = Math.min(derivedMin, derivedMax)
  const derivedHi = Math.max(derivedMin, derivedMax)
  const [effectiveMin, effectiveMax] = applyAlchemyInfusionOperationRange(derivedMin, derivedMax)
  const { posMult, negMult } = effectMultipliers("")
  const min = roundAlchemyAttributeValue(attr, effectiveMin * (effectiveMin >= 0 ? posMult : negMult))
  const max = roundAlchemyAttributeValue(attr, effectiveMax * (effectiveMax >= 0 ? posMult : negMult))
  return [{
    attr,
    baseMin: Math.min(baseMin, baseMax),
    baseMax: Math.max(baseMin, baseMax),
    derivedMin: derivedLo,
    derivedMax: derivedHi,
    min,
    max
  }]
}

function comboPowerRange(combo: AlchemyComboRecipe, multiplier: number): ComboPowerRange {
  const scale = Math.max(1, multiplier || 1)
  const baseMin = Number(combo.outputAffix?.valueRange?.min || 0) * scale
  const baseMax = Number(combo.outputAffix?.valueRange?.max || 0) * scale
  const [derivedMin, derivedMax] = applyAlchemyDerivedRange(baseMin, baseMax)
  const [effectiveMin, effectiveMax] = applyAlchemyInfusionOperationRange(derivedMin, derivedMax)
  const { posMult, negMult } = effectMultipliers("")
  const min = effectiveMin * (effectiveMin >= 0 ? posMult : negMult)
  const max = effectiveMax * (effectiveMax >= 0 ? posMult : negMult)
  return {
    baseMin: Math.min(baseMin, baseMax),
    baseMax: Math.max(baseMin, baseMax),
    min,
    max
  }
}

function comboTemplateEffectRows(value: unknown, power: ComboPowerRange, multiplier: number, path: Array<string | number> = [], effectContext: Record<string, unknown> | null = null): ComboEffectRow[] {
  if (Array.isArray(value)) {
    return value.flatMap((child, index) => comboTemplateEffectRows(child, power, multiplier, [...path, index], effectContext))
  }
  if (!isPlainRecord(value)) return []
  if (isComboTemplateNode(value)) {
    return [comboTemplateEffectRow(value, path, power, multiplier, effectContext)]
  }
  const nextEffectContext = value.kind || value.type ? value : effectContext
  const rows: ComboEffectRow[] = []
  for (const [key, child] of Object.entries(value)) {
    rows.push(...comboTemplateEffectRows(child, power, multiplier, [...path, key], nextEffectContext))
  }
  return rows
}

function comboTemplateEffectRow(node: Record<string, unknown>, path: Array<string | number>, power: ComboPowerRange, multiplier: number, effectContext: Record<string, unknown> | null): ComboEffectRow {
  const coeffKey = node.alchemyMultiplierCoeff !== undefined ? "alchemyMultiplierCoeff" : "alchemyValueCoeff"
  const coeff = Number(node[coeffKey] || 0)
  const base = Number(node.base || 0)
  let min = base
  let max = base
  if (coeffKey === "alchemyMultiplierCoeff") {
    min = base + Math.max(1, multiplier || 1) * coeff
    max = min
  } else {
    min = base + power.min * coeff
    max = base + power.max * coeff
  }
  const labelInfo = comboTemplateLabel(path, effectContext)
  if (labelInfo.format === "chance") {
    const chanceBase = Number(effectContext?.procChanceBase ?? effectContext?.procChance ?? 0)
    const chanceMin = Number(effectContext?.procChanceMin ?? 0)
    const chanceMax = Number(effectContext?.procChanceMax ?? 1)
    min = clampNumber(chanceBase + min, chanceMin, chanceMax)
    max = clampNumber(chanceBase + max, chanceMin, chanceMax)
    return {
      key: comboPathText(path),
      label: labelInfo.label,
      valueText: formatPercentRange(min, max),
      meta: t("alchemy.bench.comboChanceMeta", { base: formatPercent(chanceBase), max: formatPercent(chanceMax) }),
      tone: "positive"
    }
  }
  return {
    key: comboPathText(path),
    label: labelInfo.label,
    valueText: labelInfo.attr ? formatAttrRange(labelInfo.attr, Math.min(min, max), Math.max(min, max)) : formatNumberRange(min, max, 2),
    meta: labelInfo.meta,
    tone: Math.max(min, max) < 0 ? "negative" : labelInfo.format === "neutral" ? "neutral" : "positive"
  }
}

function comboTemplateLabel(path: Array<string | number>, effectContext: Record<string, unknown> | null): { label: string; meta?: string; attr?: string; format?: "attr" | "chance" | "neutral" } {
  const attrModsIndex = path.findIndex((node) => node === "attrMods")
  if (attrModsIndex >= 0) {
    const attr = String(path[attrModsIndex + 1] || "")
    return { label: t("alchemy.bench.comboAttrMod", { attr: localizeAttr(attr) }), attr, format: "attr" }
  }
  const statsIndex = path.findIndex((node) => node === "statsFormula")
  if (statsIndex >= 0) {
    const attr = String(path[statsIndex + 1] || "")
    const summon = comboEffectSummonName(effectContext)
    return { label: t("alchemy.bench.comboSummonStat", { unit: summon, attr: localizeAttr(attr) }), attr, format: "attr" }
  }
  if (path.some((node) => String(node).toLowerCase().includes("procchance"))) {
    return { label: t("alchemy.bench.comboProcChance", { target: comboEffectBuffName(effectContext) }), format: "chance" }
  }
  if (path.includes("payload")) {
    return {
      label: t("alchemy.bench.comboBuffPower", { buff: comboEffectBuffName(effectContext) }),
      meta: t("alchemy.bench.comboBuffPowerMeta"),
      format: "neutral"
    }
  }
  return { label: comboPathText(path), format: "neutral" }
}

function comboEffectBuffName(effectContext: Record<string, unknown> | null): string {
  const buffId = String(effectContext?.buffId || "").trim()
  if (!buffId) return t("alchemy.bench.comboEffectTarget")
  return localizeKey(`buff.${buffId}.name`)
}

function comboEffectSummonName(effectContext: Record<string, unknown> | null): string {
  const payload = isPlainRecord(effectContext?.summonPayload) ? effectContext?.summonPayload : isPlainRecord(effectContext?.summon) ? effectContext?.summon : null
  const unitId = String(payload?.unitId || "").trim()
  if (!unitId) return t("alchemy.bench.comboSummon")
  return localizeKey(`enemy.${unitId}.name`)
}

function isComboTemplateNode(value: Record<string, unknown>): boolean {
  return value.alchemyValueCoeff !== undefined || value.alchemyMultiplierCoeff !== undefined
}

function isPlainRecord(value: unknown): value is Record<string, unknown> {
  return !!value && typeof value === "object" && !Array.isArray(value)
}

function comboPathText(path: Array<string | number>): string {
  return path.map((node) => typeof node === "number" ? `[${node}]` : String(node)).join(".").replace(/\.\[/g, "[")
}

function effectRangeTooltipRows(effect: AlchemyTraitEffect, qty: number, family: string): RangeTooltipLine[] {
  const usedQty = Math.max(0, qty || 0)
  const baseMin = effect.valuePerUnit.min * usedQty
  const baseMax = effect.valuePerUnit.max * usedQty
  const baseLo = Math.min(baseMin, baseMax)
  const baseHi = Math.max(baseMin, baseMax)
  const [derivedMin, derivedMax] = applyAlchemyDerivedRange(baseMin, baseMax)
  const derivedLo = Math.min(derivedMin, derivedMax)
  const derivedHi = Math.max(derivedMin, derivedMax)
  const [effectiveMin, effectiveMax] = applyAlchemyInfusionOperationRange(derivedMin, derivedMax)
  const { posMult, negMult } = effectMultipliers(family)
  const finalMin = roundAlchemyAttributeValue(effect.attr, effectiveMin >= 0 ? effectiveMin * posMult : effectiveMin * negMult)
  const finalMax = roundAlchemyAttributeValue(effect.attr, effectiveMax >= 0 ? effectiveMax * posMult : effectiveMax * negMult)
  const rows: RangeTooltipLine[] = [
    { label: t("alchemy.bench.rangeBase"), value: formatAttrRange(effect.attr, baseLo, baseHi) }
  ]
  if (baseLo >= 0 && baseHi > 0) {
    const precisionRatio = alchemySaturatingRatio(playerAlchemyPrecision.value)
    const stabilityRatio = alchemySaturatingRatio(playerAlchemyStability.value)
    const stabilityMin = stabilityRatio >= 0 ? Math.max(baseLo, baseHi * stabilityRatio) : baseHi * stabilityRatio
    rows.push({
      label: t("alchemy.bench.rangePrecision"),
      value: `${formatSignedPercent(precisionRatio)} · ${formatAttrValue(effect.attr, baseHi)} → ${formatAttrValue(effect.attr, baseHi * (1 + precisionRatio))}`
    })
    rows.push({
      label: t("alchemy.bench.rangeStability"),
      value: `${formatSignedPercent(stabilityRatio)} · ${formatAttrValue(effect.attr, baseLo)} → ${formatAttrValue(effect.attr, stabilityMin)} / ${formatSignedPercent(stabilityRatio * 0.25)}`
    })
    rows.push({ label: t("alchemy.bench.rangeDerived"), value: formatAttrRange(effect.attr, derivedLo, derivedHi) })
  } else {
    rows.push({ label: t("alchemy.bench.rangeDerived"), value: t("alchemy.bench.rangeUnaffected") })
  }
  for (const change of effectFoodAdjustments(effect, qty, family)) {
    rows.push({ label: change.label, value: change.detail })
  }
  rows.push({ label: t("alchemy.bench.rangeMedium"), value: formatSignedPercent((derivedHi >= 0 ? posMult : negMult) - 1) })
  rows.push({ label: t("alchemy.bench.rangeFinal"), value: formatAttrRange(effect.attr, Math.min(finalMin, finalMax), Math.max(finalMin, finalMax)) })
  return rows
}

function aggregateRangeTooltipRows(entry: AggregatedAttrEntry): RangeTooltipLine[] {
  const rows: RangeTooltipLine[] = [
    { label: t("alchemy.bench.rangeBase"), value: formatAttrRange(entry.attr, entry.baseMin, entry.baseMax) }
  ]
  if (entry.baseMin >= 0 && entry.baseMax > 0) {
    rows.push({
      label: t("alchemy.bench.rangePrecision"),
      value: formatSignedPercent(alchemySaturatingRatio(playerAlchemyPrecision.value))
    })
    rows.push({
      label: t("alchemy.bench.rangeStability"),
      value: `${formatSignedPercent(alchemySaturatingRatio(playerAlchemyStability.value))} / ${formatSignedPercent(alchemySaturatingRatio(playerAlchemyStability.value) * 0.25)}`
    })
  } else {
    rows.push({ label: t("alchemy.bench.rangeDerived"), value: t("alchemy.bench.rangeUnaffected") })
  }
  rows.push({ label: t("alchemy.bench.rangeDerived"), value: formatAttrRange(entry.attr, entry.derivedMin, entry.derivedMax) })
  rows.push({ label: t("alchemy.bench.rangeFinal"), value: formatAttrRange(entry.attr, entry.min, entry.max) })
  return rows
}

function isNegativeEffect(effect: AlchemyTraitEffect): boolean {
  return effect.valuePerUnit.max < 0 || (effect.valuePerUnit.min < 0 && effect.valuePerUnit.max <= 0)
}

function effectColorClass(effect: AlchemyTraitEffect): string {
  return isNegativeEffect(effect) ? "text-rose-600 dark:text-rose-400" : "text-emerald-700 dark:text-emerald-300"
}

function formatEffectTotal(effect: AlchemyTraitEffect, qty: number, family: string): string {
  if (qty <= 0) return t("alchemy.bench.totalAt", { min: formatAttrValue(effect.attr, 0), max: formatAttrValue(effect.attr, 0), qty: 0 })
  const { posMult, negMult } = effectMultipliers(family)
  const [derivedMin, derivedMax] = applyAlchemyDerivedRange(effect.valuePerUnit.min * qty, effect.valuePerUnit.max * qty)
  const [rawMin, rawMax] = applyAlchemyInfusionOperationRange(derivedMin, derivedMax)
  const min = roundAlchemyAttributeValue(effect.attr, rawMin >= 0 ? rawMin * posMult : rawMin * negMult)
  const max = roundAlchemyAttributeValue(effect.attr, rawMax >= 0 ? rawMax * posMult : rawMax * negMult)
  const lo = Math.min(min, max)
  const hi = Math.max(min, max)
  return t("alchemy.bench.totalAt", { min: formatAttrValue(effect.attr, lo), max: formatAttrValue(effect.attr, hi), qty })
}

function formatEffectTotalPlain(effect: AlchemyTraitEffect, qty: number, family: string): string {
  if (qty <= 0) return formatAttrRange(effect.attr, 0, 0)
  const { posMult, negMult } = effectMultipliers(family)
  const [derivedMin, derivedMax] = applyAlchemyDerivedRange(effect.valuePerUnit.min * qty, effect.valuePerUnit.max * qty)
  const [rawMin, rawMax] = applyAlchemyInfusionOperationRange(derivedMin, derivedMax)
  const min = roundAlchemyAttributeValue(effect.attr, rawMin >= 0 ? rawMin * posMult : rawMin * negMult)
  const max = roundAlchemyAttributeValue(effect.attr, rawMax >= 0 ? rawMax * posMult : rawMax * negMult)
  return formatAttrRange(effect.attr, Math.min(min, max), Math.max(min, max))
}

function formatEffectPerUnit(effect: AlchemyTraitEffect, family: string): string {
  const { posMult, negMult } = effectMultipliers(family)
  const [derivedMin, derivedMax] = applyAlchemyDerivedRange(effect.valuePerUnit.min, effect.valuePerUnit.max)
  const [rawMin, rawMax] = applyAlchemyInfusionOperationRange(derivedMin, derivedMax)
  const min = rawMin >= 0 ? rawMin * posMult : rawMin * negMult
  const max = rawMax >= 0 ? rawMax * posMult : rawMax * negMult
  const lo = Math.min(min, max)
  const hi = Math.max(min, max)
  return `${t("alchemy.bench.perUnit")} ${formatAttrRange(effect.attr, lo, hi)}`
}

function formatAttrRange(attr: string, min: number, max: number): string {
  return `${formatAttrValue(attr, min)} ~ ${formatAttrValue(attr, max)}`
}

function formatAttrValue(attr: string, value: number): string {
  return formatAttributeValueByKey(attr, value, false, attrPrecision(value))
}

function roundAlchemyAttributeValue(attr: string, value: number): number {
  const precision = getAttributeUnit(attr) === "percent" ? 6 : 3
  const factor = 10 ** precision
  const scaled = value * factor
  return (scaled < 0 ? -Math.round(-scaled) : Math.round(scaled)) / factor
}

function applyAlchemyDerivedRange(minRaw: number, maxRaw: number): [number, number] {
  const lo = Math.min(minRaw, maxRaw)
  const hi = Math.max(minRaw, maxRaw)
  if (lo < 0 || hi <= 0) return [lo, hi]
  const precisionRatio = alchemySaturatingRatio(playerAlchemyPrecision.value)
  const stabilityRatio = alchemySaturatingRatio(playerAlchemyStability.value)
  const min = stabilityRatio >= 0 ? Math.max(lo, hi * stabilityRatio) : hi * stabilityRatio
  const max = hi * Math.max(0.000001, 1 + precisionRatio + stabilityRatio * 0.25)
  return [Math.min(min, max), max]
}

function applyAlchemyInfusionOperationRange(minRaw: number, maxRaw: number): [number, number] {
  return applyInfusionOperationRange(minRaw, maxRaw, alchemyInfusionModifiers.value)
}

function effectFoodAdjustments(effect: AlchemyTraitEffect, qty: number, family: string) {
  const [lo, hi] = applyAlchemyDerivedRange(effect.valuePerUnit.min * Math.max(0, qty), effect.valuePerUnit.max * Math.max(0, qty))
  const before = applyInfusionOperationRange(lo, hi, { positiveFloorReductionRate: 0, positiveCeilingBonusRate: 0 })
  const after = applyAlchemyInfusionOperationRange(lo, hi)
  const { posMult, negMult } = effectMultipliers(family)
  const rounded = (value: number) => roundAlchemyAttributeValue(effect.attr, value * (value >= 0 ? posMult : negMult))
  return (['floor', 'ceiling'] as const).flatMap((key, index) => {
    const original = rounded(before[index])
    const current = rounded(after[index])
    const delta = roundAlchemyAttributeValue(effect.attr, current - original)
    if (delta === 0) return []
    return [{
      label: t(`game.fishCuisine.controls.${key}`, { delta: `${delta > 0 ? '+' : ''}${formatAttrValue(effect.attr, delta)}` }),
      detail: t('game.fishCuisine.controls.rangeDetail', { before: formatAttrValue(effect.attr, original), after: formatAttrValue(effect.attr, current) }),
      cost: delta < 0
    }]
  })
}

function withFoodFailureModifiers(breakdown: FailChanceBreakdown): FailChanceBreakdown {
  const final = applyInfusionFoodFailureChance(breakdown.final, alchemyInfusionModifiers.value.successRatePenalty, selectedMedium.value?.modifiers.successFloor || 0, alchemyInfusionModifiers.value.successRateBonus)
  return { ...breakdown, foodDelta: final - breakdown.final, final }
}

function alchemySaturatingRatio(value: number): number {
  const numeric = Number(value || 0)
  if (!Number.isFinite(numeric)) return 0
  return numeric / (1 + Math.abs(numeric))
}

function formatSeconds(value: number): string {
  return formatDurationI18n(Math.max(0, Number(value) || 0), t)
}

function formatNumber(value: number, digits = 0): string {
  if (!Number.isFinite(value)) return "0"
  return value.toFixed(digits).replace(/\.0+$/, "").replace(/(\.\d*?)0+$/, "$1")
}

function formatNumberRange(min: number, max: number, digits = 2): string {
  const lo = Math.min(min, max)
  const hi = Math.max(min, max)
  if (Math.abs(lo - hi) < 0.000001) return formatNumber(lo, digits)
  return `${formatNumber(lo, digits)} ~ ${formatNumber(hi, digits)}`
}

function formatPercentRange(min: number, max: number): string {
  const lo = Math.min(min, max)
  const hi = Math.max(min, max)
  if (Math.abs(lo - hi) < 0.000001) return formatPercent(lo)
  return `${formatPercent(lo)} ~ ${formatPercent(hi)}`
}

function clampNumber(value: number, min: number, max: number): number {
  if (!Number.isFinite(value)) return min
  return Math.max(min, Math.min(max, value))
}

function attrPrecision(value: number): number {
  if (!Number.isFinite(value)) return 2
  const abs = Math.abs(value)
  if (abs >= 100) return 0
  if (abs >= 10) return 1
  if (abs >= 1) return 2
  if (abs >= 0.01) return 3
  return 4
}

function formatPercent(value: number): string {
  if (!Number.isFinite(value)) return "0%"
  return `${(value * 100).toFixed(2)}%`
}

function formatSignedPercent(value: number): string {
  if (!Number.isFinite(value)) return "0%"
  const pct = value * 100
  return `${pct >= 0 ? "+" : ""}${pct.toFixed(2)}%`
}

function successTagType(pct: number): "success" | "warning" | "danger" {
  if (pct >= 0.85) return "success"
  if (pct >= 0.55) return "warning"
  return "danger"
}

function successTagColorClass(pct: number): string {
  if (pct >= 0.85) return "text-emerald-600 dark:text-emerald-400"
  if (pct >= 0.55) return "text-amber-600 dark:text-amber-400"
  return "text-rose-600 dark:text-rose-400"
}
</script>

<style scoped>
.alchemy-combo-stepper {
  display: inline-flex;
  align-items: center;
  gap: 7px;
  border: 1px solid color-mix(in srgb, var(--app-success, #16a34a) 30%, var(--app-border));
  border-radius: 8px;
  background: color-mix(in srgb, var(--app-success, #16a34a) 7%, transparent);
  padding: 5px 7px;
  color: var(--app-text);
  font-size: 12px;
}

.alchemy-combo-stepper > span {
  color: var(--app-muted);
}

.alchemy-combo-stepper strong {
  color: var(--app-success, #16a34a);
  white-space: nowrap;
}

.alchemy-material-quantity {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 8px 10px;
}

.alchemy-material-quantity__input {
  width: 120px;
}

.alchemy-material-quick-steps {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  border-radius: 8px;
  background: color-mix(in srgb, var(--el-color-primary) 7%, transparent);
  padding: 3px;
}

.alchemy-material-quick-step {
  min-width: 38px;
  padding-inline: 8px;
}

.alchemy-combo-summary {
  display: grid;
  gap: 8px;
  border-radius: 8px;
  border: 1px solid color-mix(in srgb, var(--app-success, #16a34a) 28%, var(--app-border));
  background: color-mix(in srgb, var(--app-success, #16a34a) 7%, transparent);
  padding: 10px;
  color: var(--app-text);
  font-size: 12px;
}

.alchemy-combo-summary__head {
  display: grid;
  gap: 2px;
}

.alchemy-combo-effect-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(230px, 1fr));
  gap: 7px;
}

.alchemy-combo-effect-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  border-radius: 7px;
  border: 1px solid color-mix(in srgb, currentColor 18%, transparent);
  background: color-mix(in srgb, currentColor 6%, transparent);
  padding: 7px 9px;
}

.alchemy-combo-effect-row > span {
  display: grid;
  gap: 1px;
  min-width: 0;
}

.alchemy-combo-effect-row strong {
  line-height: 1.2;
}

.alchemy-combo-effect-row small {
  color: var(--app-muted);
  font-size: 11px;
}

.alchemy-combo-effect-row > strong:last-child {
  flex: 0 0 auto;
  text-align: right;
  white-space: nowrap;
}

.alchemy-combo-effect-row--positive {
  color: #047857;
}

.alchemy-combo-effect-row--negative {
  color: #e11d48;
}

.alchemy-combo-effect-row--neutral {
  color: #1d4ed8;
}

.alchemy-effect-list {
  display: grid;
  gap: 6px;
  border-radius: 8px;
  background: var(--app-bg-mute, rgba(0, 0, 0, 0.03));
  padding: 8px;
}

.alchemy-effect-row {
  display: grid;
  gap: 2px;
  border-radius: 7px;
  background: color-mix(in srgb, currentColor 7%, transparent);
  padding: 7px 8px;
}

.alchemy-effect-row__main {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: 12px;
  font-size: 12px;
}

.alchemy-effect-row__value {
  display: inline-flex;
  align-items: center;
  justify-content: flex-end;
  gap: 5px;
  min-width: 0;
  white-space: nowrap;
}

.alchemy-range-help {
  cursor: help;
  color: var(--app-muted);
  font-size: 13px;
}

.alchemy-range-popover {
  display: grid;
  gap: 7px;
  font-size: 12px;
}

.alchemy-range-popover__title {
  font-weight: 700;
}

.alchemy-range-popover__row {
  display: grid;
  grid-template-columns: minmax(72px, max-content) 1fr;
  gap: 10px;
  align-items: baseline;
}

.alchemy-range-popover__row span {
  color: var(--app-muted);
}

.alchemy-range-popover__row strong {
  text-align: right;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, "Liberation Mono", monospace;
  font-weight: 700;
}

.alchemy-effect-row__meta {
  color: var(--app-muted);
  font-size: 11px;
}

.alchemy-preview-groups {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(240px, 1fr));
  gap: 8px;
}

.alchemy-fish-setup {
  display: grid;
  grid-template-columns: minmax(0, 1fr) minmax(210px, 0.42fr);
  gap: 10px;
  align-items: start;
}

.alchemy-fish-choice-list {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 8px;
}

.alchemy-fish-choice,
.alchemy-fish-medium {
  display: flex;
  min-height: 76px;
  align-items: center;
  gap: 12px;
  border: 1px solid var(--app-border);
  border-radius: 8px;
  background: var(--app-surface, #fff);
  padding: 10px 12px;
  color: var(--app-text);
  text-align: left;
}

.alchemy-fish-choice {
  min-height: 68px;
  padding: 8px 10px;
}

.alchemy-fish-choice {
  cursor: pointer;
}

.alchemy-fish-choice:hover {
  border-color: var(--el-color-primary);
}

.alchemy-fish-choice > .fish-mutation-art,
.alchemy-fish-choice > .el-icon {
  width: 54px;
  height: 46px;
  flex: 0 0 54px;
  object-fit: contain;
  font-size: 28px;
  color: var(--el-color-primary);
}
.alchemy-fish-choice > .fish-mutation-art { aspect-ratio: 1.18; }

.alchemy-fish-choice > span,
.alchemy-fish-medium > span {
  display: grid;
  min-width: 0;
  gap: 2px;
}

.alchemy-fish-choice small,
.alchemy-fish-medium small,
.alchemy-fish-choice em,
.alchemy-fish-medium em {
  color: var(--app-muted);
  font-size: 12px;
  font-style: normal;
}

.alchemy-fish-choice strong,
.alchemy-fish-medium strong {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.alchemy-fish-preview {
  display: grid;
  gap: 10px;
  border: 1px solid var(--app-border);
  border-radius: 8px;
  padding: 12px;
}

.alchemy-fish-preview > header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.alchemy-fish-preview > header > div {
  display: grid;
  gap: 2px;
}

.alchemy-fish-preview > header small,
.alchemy-fish-preview__values span,
.alchemy-fish-preview__values p {
  color: var(--app-muted);
  font-size: 12px;
}

.alchemy-fish-preview__values {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 8px;
}

.alchemy-fish-preview-list {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 8px;
}

.alchemy-fish-preview__values {
  border: 1px solid color-mix(in srgb, var(--app-border) 76%, transparent);
  border-radius: 7px;
  padding: 9px;
}

.alchemy-fish-preview__values > header {
  grid-column: 1 / -1;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  min-width: 0;
  font-size: 13px;
  font-weight: 600;
}

.alchemy-fish-preview__rule {
  grid-column: 1 / -1;
  margin: 2px 0 0;
  color: var(--app-muted);
  font-size: 12px;
}

.alchemy-fish-preview__values > div {
  display: grid;
  gap: 3px;
  border-radius: 7px;
  background: var(--app-bg-mute, rgba(0, 0, 0, 0.03));
  padding: 9px 10px;
}
.alchemy-fish-preview__values > .alchemy-fish-preview__mutations { grid-column: 1 / -1; display: flex; flex-wrap: wrap; gap: 5px; padding: 0; background: transparent; }
.alchemy-fish-preview__mutations :deep(.el-tag) { border-color: color-mix(in srgb, #4daab6 46%, var(--app-border)); color: #267884; }
.alchemy-fish-preview__mutations :deep(svg) { margin-right: 3px; }
.alchemy-fish-preview__values > .alchemy-fish-preview__bonus { border: 1px solid color-mix(in srgb, #d0a63b 34%, var(--app-border)); background: color-mix(in srgb, #f6d978 8%, var(--app-surface)); }

.alchemy-fish-preview__values p {
  grid-column: 1 / -1;
  margin: 0;
}

.alchemy-fish-picker {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 8px;
}

.alchemy-fish-picker-count {
  margin: 0 0 10px;
  color: var(--app-muted);
  font-size: 13px;
}

.alchemy-fish-picker > button {
  display: grid;
  grid-template-columns: 72px minmax(0, 1fr) 20px;
  min-height: 76px;
  align-items: center;
  gap: 10px;
  border: 1px solid var(--app-border);
  border-radius: 8px;
  background: var(--app-surface, #fff);
  padding: 8px 10px;
  color: var(--app-text);
  text-align: left;
}

.alchemy-fish-picker > button.is-selected {
  border-color: var(--el-color-primary);
  background: color-mix(in srgb, var(--el-color-primary) 7%, var(--app-surface, #fff));
}

.alchemy-fish-picker .fish-mutation-art {
  width: 72px;
  height: 56px;
}

.alchemy-fish-picker span {
  display: grid;
  min-width: 0;
  gap: 3px;
}

.alchemy-fish-picker strong,
.alchemy-fish-picker small {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.alchemy-fish-picker small {
  color: var(--app-muted);
}

@media (max-width: 720px) {
  .alchemy-fish-setup,
  .alchemy-fish-choice-list,
  .alchemy-fish-preview-list,
  .alchemy-fish-preview__values,
  .alchemy-fish-picker {
    grid-template-columns: 1fr;
  }

  .alchemy-fish-picker > button {
    grid-template-columns: 64px minmax(0, 1fr) 20px;
  }
}
</style>
