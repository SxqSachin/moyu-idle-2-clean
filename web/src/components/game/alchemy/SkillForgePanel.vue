<template>
  <section class="skill-forge" data-testid="skill-forge-panel" data-tutorial="skill-forge-panel">
    <header class="skill-forge__toolbar" data-tutorial="skill-forge-source">
      <div class="skill-forge__draft-tools">
        <el-select
          :model-value="activeDraftId"
          class="draft-select"
          :placeholder="t('skillForge.draft.select')"
          :disabled="saving || forging"
          @change="selectDraft"
        >
          <el-option v-for="draft in availableDesigns" :key="draft.id" :label="draftLabel(draft)" :value="draft.id" />
        </el-select>
        <el-button type="primary" :icon="EditPen" @click="openSourceDialog">{{ t("skillForge.draft.chooseSource") }}</el-button>
        <el-button
          :icon="Delete"
          :loading="deletingDraft"
          :disabled="!activeDraft || saving || Boolean(activeDraft?.pendingRevisionId)"
          @click="deleteActiveDraft"
        >
          {{ t("skillForge.draft.delete") }}
        </el-button>
        <span class="draft-count">{{ t("skillForge.draft.count", { current: activeDraftCount, max: maxDrafts }) }}</span>
      </div>
      <div class="skill-forge__canvas-tools">
				<el-tooltip :content="t('skillForge.undo')"><el-button :icon="RefreshLeft" :aria-label="t('skillForge.undo')" :disabled="undoStack.length === 0" @click="undo" /></el-tooltip>
				<el-tooltip :content="t('skillForge.redo')"><el-button :icon="RefreshRight" :aria-label="t('skillForge.redo')" :disabled="redoStack.length === 0" @click="redo" /></el-tooltip>
				<el-tooltip :content="t('skillForge.deleteSelection')"><el-button :icon="Delete" :aria-label="t('skillForge.deleteSelection')" :disabled="!hasDeletableSelection" @click="removeSelection" /></el-tooltip>
				<el-tooltip :content="t('skillForge.autoLayout')"><el-button :icon="Grid" :aria-label="t('skillForge.autoLayout')" :disabled="!activeDraft" @click="autoLayout" /></el-tooltip>
        <span class="save-state" :class="{ 'is-saving': saving, 'is-dirty': draftDirty, 'is-error': Boolean(saveError) }">
          {{ saveStateLabel }}
        </span>
        <el-button v-if="saveError" link type="primary" :disabled="saving" @click="retrySaveDraft">{{ t("skillForge.retrySave") }}</el-button>
      </div>
    </header>

    <div v-if="loading" class="skill-forge__loading"><el-skeleton :rows="9" animated /></div>
    <el-result v-else-if="loadError" icon="error" :title="t('skillForge.stateLoadFailed')" :sub-title="loadError">
      <template #extra><el-button type="primary" @click="loadState()">{{ t("skillForge.retry") }}</el-button></template>
    </el-result>
    <el-empty v-else-if="!activeDraft" :description="t('skillForge.empty')" />
    <div v-else class="skill-forge__workspace">
      <nav class="workspace-switcher" :aria-label="t('skillForge.workspace.label')">
        <el-segmented v-model="mobileWorkspacePane" :options="workspacePaneOptions" />
      </nav>

      <aside class="module-panel" :class="{ 'is-mobile-hidden': mobileWorkspacePane !== 'materials' }" data-tutorial="skill-forge-materials">
        <div class="panel-heading">
          <strong>{{ selectedParameter ? nodeTitle(selectedParameter.data) : t("skillForge.materials") }}</strong>
          <span>{{ filteredModules.length }}</span>
        </div>
        <p v-if="selectedParameter" class="module-filter-hint">{{ t("skillForge.parameter.filteredMaterials") }}</p>
        <el-input v-model="moduleQuery" :prefix-icon="Search" clearable :placeholder="t('skillForge.searchMaterial')" />
        <div class="module-list">
          <el-popover
            v-for="module in filteredModules"
            :key="module.id"
            placement="right-start"
            :width="286"
            trigger="hover"
            :show-after="180"
          >
            <template #reference>
              <button
                class="module-row"
                draggable="true"
                :class="{ 'is-short': inventoryCount(module.itemId) < module.unitBasis }"
                :style="{ '--module-rarity-color': itemRarityColor(module.itemId) }"
                @click="attachModuleToSelected(module)"
                @dragstart="startModuleDrag($event, module.id)"
                @dragend="endModuleDrag"
              >
                <span class="module-row__icon"><ItemIcon :item-id="module.itemId" :size="28" /></span>
                <span class="module-row__body">
                  <span class="module-row__name">{{ itemName(module.itemId) }}</span>
                  <span class="module-row__meta">{{ moduleTargetLabel(module) }}</span>
                  <span class="module-row__batch">{{ t("skillForge.inspector.batchSummary", { quantity: module.unitBasis, range: moduleBaseRangeLabel(module, module.unitBasis), max: module.unitBasis * module.maxBasisMultiplier }) }}</span>
                  <span class="module-row__chance" :class="successModifierClass(module.successChanceModifier)">{{ t("skillForge.inspector.chanceImpact", { value: formatSignedPercent(module.successChanceModifier) }) }}</span>
                </span>
                <span class="module-row__stock">{{ t("skillForge.inspector.stock", { count: inventoryCount(module.itemId) }) }}</span>
              </button>
            </template>
            <div class="module-inspector">
              <header><ItemIcon :item-id="module.itemId" :size="28" /><strong>{{ itemName(module.itemId) }}</strong></header>
              <div><span>{{ t("skillForge.inspector.target") }}</span><strong>{{ moduleTargetLabel(module) }}</strong></div>
              <div><span>{{ t("skillForge.inspector.perUnit") }}</span><strong>{{ moduleBaseRangeLabel(module, 1) }}</strong></div>
              <div><span>{{ t("skillForge.inspector.batchQuantity") }}</span><strong>{{ module.unitBasis }}</strong></div>
              <div><span>{{ t("skillForge.inspector.batchRange") }}</span><strong>{{ moduleBaseRangeLabel(module, module.unitBasis) }}</strong></div>
              <div><span>{{ t("skillForge.inspector.maxQuantity") }}</span><strong>{{ module.unitBasis * module.maxBasisMultiplier }}</strong></div>
              <div><span>{{ t("skillForge.inspector.successChance") }}</span><strong :class="successModifierClass(module.successChanceModifier)">{{ formatSignedPercent(module.successChanceModifier) }}</strong></div>
              <div><span>{{ t("skillForge.inspector.budget") }}</span><strong>{{ formatNumber(module.budgetCreditPerUnit * module.unitBasis) }}</strong></div>
              <div><span>{{ t("skillForge.inspector.inventory") }}</span><strong>{{ inventoryCount(module.itemId) }}</strong></div>
            </div>
          </el-popover>
        </div>
      </aside>

      <main class="graph-panel" :class="{ 'is-mobile-hidden': mobileWorkspacePane !== 'graph' }" data-tutorial="skill-forge-graph" @dragover.prevent @drop="dropModule">
        <VueFlow
          v-model:nodes="canvasNodes"
          v-model:edges="canvasEdges"
          class="skill-forge-flow"
          :min-zoom="0.25"
          :max-zoom="2"
          :snap-to-grid="true"
          :snap-grid="[16, 16]"
          :delete-key-code="null"
          :edges-selectable="false"
          :default-edge-options="{ type: 'smoothstep', style: { stroke: '#7890ad', strokeWidth: 2 } }"
          fit-view-on-init
          @connect="connectNodes"
          @node-click="handleNodeClick"
          @pane-click="clearParameterSelection"
          @node-drag-start="captureBeforeChange"
          @node-drag-stop="handleNodeDragStop"
          @pane-ready="fitGraph"
        >
          <Background :gap="20" :size="1" pattern-color="var(--app-border)" />
          <MiniMap pannable zoomable node-color="var(--app-primary)" mask-color="color-mix(in srgb, var(--app-bg) 74%, transparent)" />
          <Controls position="bottom-right" />

          <template #node-forge="{ id, data, selected }">
            <article class="forge-node" :class="[`forge-node--${String(data.kind)}`, { 'is-selected': selected || selectedParameterId === id, 'is-compatible-drop': parameterAcceptsDraggedMaterial(data), 'is-incompatible-drop': draggingModuleId && data.kind === 'parameter' && !parameterAcceptsDraggedMaterial(data) }]" :style="data.kind === 'material' ? { '--module-rarity-color': itemRarityColor(String(data.moduleItemId || '')) } : undefined" :data-testid="`forge-node-${String(data.kind)}`">
              <template v-if="data.kind === 'source'">
                <Handle type="source" id="skill" :position="Position.Right" :style="{ top: '22%' }" />
                <Handle type="source" id="condition" :position="Position.Right" :style="{ top: '50%' }" />
                <Handle type="source" id="effect" :position="Position.Right" :style="{ top: '78%' }" />
              </template>
              <template v-else-if="data.kind === 'output'">
                <Handle type="target" id="skill" :position="Position.Left" :style="{ top: '22%' }" />
                <Handle type="target" id="condition" :position="Position.Left" :style="{ top: '50%' }" />
                <Handle type="target" id="effect" :position="Position.Left" :style="{ top: '78%' }" />
              </template>
              <template v-else-if="data.kind === 'effect'">
                <Handle type="target" id="in" :position="Position.Left" />
                <Handle type="source" id="out" :position="Position.Right" />
              </template>
              <template v-else-if="data.kind === 'parameter'">
                <Handle type="target" id="in" :position="Position.Left" />
                <Handle type="source" id="out" :position="Position.Right" />
                <Handle v-if="parameterCompatibleModuleCount(data) > 0" type="source" id="modifier" :position="Position.Bottom" class="forge-port--modifier" />
              </template>
              <template v-else-if="data.kind === 'material' && data.targetNodeId && !isBranchCondition(data)">
                <Handle type="target" id="in" :position="Position.Top" />
                <Handle v-if="hasMaterialFollower(id)" type="source" id="out" :position="Position.Bottom" />
              </template>
              <template v-else>
                <Handle v-for="(port, index) in moduleForData(data)?.typedPorts.inputs || []" :key="`in-${port.id}`" type="target" :id="port.id" :position="Position.Left" :style="materialPortStyle(data, port.id, index, moduleForData(data)?.typedPorts.inputs.length || 1, true)" />
                <Handle v-for="(port, index) in moduleForData(data)?.typedPorts.outputs || []" :key="`out-${port.id}`" type="source" :id="port.id" :position="Position.Right" :class="`forge-port--${port.id}`" :style="materialPortStyle(data, port.id, index, moduleForData(data)?.typedPorts.outputs.length || 1, false)" />
              </template>

              <header class="forge-node__header">
                <span v-if="data.kind === 'material'" class="forge-node__item"><ItemIcon :item-id="String(data.moduleItemId || '')" :size="24" /></span>
                <span class="forge-node__title">{{ nodeTitle(data) }}</span>
								<el-button v-if="canRemoveNode(id)" class="forge-node__remove nodrag" text :icon="Close" :aria-label="t('skillForge.deleteNode')" @click.stop="removeCanvasNode(id)" />
              </header>
              <div class="forge-node__body">
                <template v-if="data.kind === 'source'">
                  <span>{{ sourceLabel(activeDraft) }}</span>
                  <template v-if="activeDraft.sourceKind === 'official'">
                    <small>{{ itemName(activeDraft.sourceSkillbookItemId || '') }} · {{ activeDraft.sourceSkillbookCount }} {{ t("skillForge.draft.books") }}</small>
                    <small>{{ t("skillForge.preview.bookBonus") }} {{ formatPercent(skillbookBonusForCount(activeDraft.sourceSkillbookCount)) }}</small>
                    <el-input-number class="nodrag source-book-stepper" :model-value="activeDraft.sourceSkillbookCount" :min="sourceBookMinCount" :max="sourceBookMaxCount" :step="sourceBookCountStep" size="small" @change="updateSourceBookCount" />
                  </template>
                  <small>{{ t("skillForge.port.effect") }} · {{ t("skillForge.port.condition") }} · {{ t("skillForge.port.skill") }}</small>
                </template>
                <template v-else-if="data.kind === 'effect'">
										<el-select
											v-if="canChooseEffectKind(data)"
											class="effect-kind-select nodrag"
											:model-value="normalizeEffectKind(String(data.effectKind || 'empty'))"
											size="small"
											:aria-label="t('skillForge.effect.chooseType')"
											:placeholder="t('skillForge.effect.chooseType')"
											@mousedown.stop
											@change="(value: string) => updateEffectKind(id, value)"
										>
											<el-option v-for="option in effectKindOptions" :key="option.value" :label="option.label" :value="option.value" />
										</el-select>
										<el-tag v-else size="small" :type="effectTagType(String(data.effectKind || 'empty'))">{{ t(`skillForge.effect.${String(data.effectKind || "empty")}`) }}</el-tag>
										<small>{{ effectHeaderValue(data) }}</small>
                </template>
                <template v-else-if="data.kind === 'parameter'">
                  <strong class="forge-node__dsl-value">{{ parameterValueLabel(data) }}</strong>
                  <small v-if="parameterMaterialCount(id) > 0" class="forge-node__customized-cost">{{ customizedNodeCostLabel(id) }}</small>
                  <small v-if="parameterCompatibleModuleCount(data) > 0" class="forge-node__slot">{{ t("skillForge.parameter.accepts", { count: parameterCompatibleModuleCount(data) }) }}</small>
                  <small v-else>{{ t("skillForge.parameter.readOnly") }}</small>
                </template>
                <template v-else-if="data.kind === 'material'">
				  <span>{{ materialOperationLabel(data) }}</span>
                  <div class="node-quantity nodrag" @mousedown.stop>
                    <span>{{ t("skillForge.quantity") }}</span>
                    <el-input-number
                      :model-value="Number(data.quantity || 1)"
                      :min="Number(data.unitBasis || 1)"
                      :max="Number(data.maxQuantity || 1)"
                      :step="Number(data.unitBasis || 1)"
                      size="small"
                      controls-position="right"
                      @change="(value: number | undefined) => updateQuantity(id, Number(value || 1))"
                    />
                  </div>
										<small class="forge-node__contribution">{{ materialNodeContribution(data) }}</small>
					<small :class="successModifierClass(materialNodeSuccessModifier(data))">{{ t("skillForge.inspector.totalSuccessChance") }} {{ materialNodeSuccess(data) }}</small>
					<template v-if="!isBranchCondition(data) && !isGlobalModule(data)">
							<small>{{ materialAttachedParameterLabel(data) }}</small>
					</template>
					<div v-else class="condition-branches"><span>{{ t("skillForge.condition.true") }}</span><span>{{ t("skillForge.condition.false") }}</span></div>
                </template>
                <template v-else><span>{{ t("skillForge.outputHint") }}</span></template>
              </div>
            </article>
          </template>
        </VueFlow>
      </main>

      <aside class="preview-panel" :class="{ 'is-mobile-hidden': mobileWorkspacePane !== 'preview' }" data-tutorial="skill-forge-preview">
        <div class="panel-heading">
          <strong>{{ t("skillForge.preview.title") }}</strong>
          <el-tag :type="preview?.valid ? 'success' : 'danger'" size="small">{{ preview?.valid ? t("skillForge.preview.ready") : t("skillForge.preview.blocked") }}</el-tag>
        </div>

        <section class="preview-metrics">
					<div><span>{{ t("skillForge.preview.rarity") }}</span><strong>{{ skillRarityLabel }}</strong></div>
			<div><span>{{ t("skillForge.preview.baseSuccess") }}</span><strong>{{ formatPercent(preview?.baseSuccessChance ?? state?.limits.baseSuccessChance ?? 0.3) }}</strong></div>
          <div><span>{{ t("skillForge.preview.materialModifier") }}</span><strong :class="successModifierClass(preview?.materialSuccessModifier || 0)">{{ formatSignedPercent(preview?.materialSuccessModifier || 0) }}</strong></div>
          <div v-if="activeDraft.sourceKind === 'official'"><span>{{ t("skillForge.preview.bookBonus") }}</span><strong>+{{ formatPercent(preview?.skillbookBonus || 0) }}</strong></div>
          <div><span>{{ t("skillForge.preview.finalSuccess") }}</span><strong>{{ formatPercent(1 - (preview?.finalFailChance ?? preview?.failChance ?? 0)) }}</strong></div>
          <div><span>{{ t("skillForge.preview.budget") }}</span><strong>{{ formatNumber(preview?.budgetUsed || 0) }} / {{ formatNumber(preview?.budgetLimit || 0) }} · ×{{ formatNumber(preview?.budgetMultiplier || 1) }}</strong></div>
          <div><span>{{ t("skillForge.preview.refund") }}</span><strong>{{ formatNumber(preview?.budgetRefund || 0) }}</strong></div>
          <div v-if="preview?.goldCost"><span>{{ t("skillForge.preview.goldCost") }}</span><strong>{{ Number(preview.goldCost).toLocaleString() }}</strong></div>
        </section>

        <el-segmented v-model="previewPane" class="preview-switcher" :options="previewPaneOptions" />

        <section v-if="editableCustomSkills.length" class="preview-section">
          <h4>{{ t("skillForge.skills.title") }}</h4>
          <div class="custom-skill-list">
            <div v-for="skill in editableCustomSkills" :key="skill.id">
              <SkillQuickRef :skill-id="skill.id" :show-icon="true" :runtime-skill="skill.compiledSkill || null" />
              <el-tooltip :content="t('skillForge.skills.archive')">
                <el-button
                  text
                  :icon="FolderDelete"
									:aria-label="t('skillForge.skills.archive')"
                  :loading="archivingSkillId === skill.id"
                  @click="archiveSkill(skill.id)"
                />
              </el-tooltip>
            </div>
          </div>
        </section>

        <section v-if="preview?.blockingReasons?.length" class="blocking-list">
          <div v-for="reason in preview.blockingReasons" :key="reason"><WarningFilled /> <span>{{ friendlyReason(reason) }}</span></div>
			<el-button v-if="sourceIsStale" size="small" @click="rebaseDraft">{{ t("skillForge.rebase") }}</el-button>
        </section>

        <template v-if="previewPane === 'effects'">
          <section class="preview-section final-skill-preview">
            <h4>{{ t("skillForge.preview.finalSkill") }}</h4>
            <SkillQuickRefDetailContent
              v-if="finalPreviewSkill"
              :skill-id="finalPreviewSkillId"
              :runtime-skill="finalPreviewSkill"
              :baseline-skill="finalPreviewBaselineSkill"
              :unit-skill-level="1"
            />
            <el-empty v-else :image-size="54" :description="t('skillForge.preview.noEffects')" />
          </section>

          <section class="preview-section">
            <h4>{{ t("skillForge.preview.ranges") }}</h4>
            <div class="range-list">
              <div v-for="range in displayedRanges" :key="range.nodeId">
                <span class="range-label"><strong>{{ rangeMaterialName(range.nodeId) }}</strong><small>{{ operationLabel(range.kind, range.direction) }} · {{ rangeTargetLabel(range.nodeId, range.targetNodeId) }}</small></span>
                <span class="range-value">
                  <strong v-if="typeof range.rolled === 'number'">{{ formatRangeValue(range.kind, range.rolled) }}</strong>
                  <strong v-else>{{ formatRangeSpan(range.kind, range.min, range.max) }}</strong>
                  <small>{{ formatRangeSpan(range.kind, range.baseMin, range.baseMax) }} → {{ formatRangeSpan(range.kind, range.min, range.max) }}</small>
                </span>
              </div>
            </div>
          </section>
        </template>

        <section v-else class="preview-section recipe-section">
          <h4>{{ t("skillForge.preview.recipe") }}</h4>
          <div class="recipe-list">
            <div v-for="row in recipeRows" :key="row.itemId" :class="{ 'is-short': row.missing > 0 }">
              <span class="recipe-item">
                <ItemQuickRef :item-id="row.itemId" />
                <el-popover placement="left" :width="300" trigger="hover">
                  <template #reference>
                    <el-button class="recipe-usage-help" text circle :icon="QuestionFilled" :aria-label="t('skillForge.preview.whereSpent')" />
                  </template>
                  <div class="recipe-usage">
                    <strong>{{ t("skillForge.preview.whereSpent") }}</strong>
                    <div v-for="usage in recipeUsage(row.itemId)" :key="usage">{{ usage }}</div>
                  </div>
                </el-popover>
              </span>
              <strong>{{ row.owned }}/{{ row.required }}</strong>
            </div>
          </div>
          <p class="recipe-note">{{ mandatoryRecipeHint }}</p>
        </section>

        <el-alert
          v-if="!currentRulesetEnabled"
          type="error"
          :closable="false"
          show-icon
          :title="t('skillForge.rulesetDisabled')"
        />
        <div v-if="activeDraft.sourceKind === 'official'" class="forge-action" data-tutorial="skill-forge-submit">
          <el-button type="primary" size="large" :loading="forging" :disabled="saving || draftDirty || !!saveError || !preview?.valid || hasRecipeShortage || !!activeDraft.pendingRevisionId || !currentRulesetEnabled" @click="forge">
            {{ t("skillForge.forge") }}
          </el-button>
        </div>
        <el-alert v-else type="info" :closable="false" show-icon :title="t('skillForge.blankPreviewOnly')" />

        <section v-if="lastAttempt?.failed" class="pending-result is-failed">
          <h4>{{ t("skillForge.result.failed") }}</h4>
          <p>{{ t("skillForge.result.failedHint") }}</p>
          <div class="recipe-list">
            <div v-for="(qty, itemId) in lastAttempt.consumedRecipe" :key="itemId"><span>{{ itemName(String(itemId)) }}</span><strong>{{ t("skillForge.result.consumedRefunded", { consumed: qty, refunded: lastAttempt.refundedRecipe[String(itemId)] || 0 }) }}</strong></div>
          </div>
        </section>

        <section v-if="acceptedBook" class="pending-result">
          <h4>{{ t("skillForge.result.bookCreated") }}</h4>
          <SkillQuickRef :skill-id="acceptedBook.customSkillId" :runtime-skill="acceptedBook.compiledSkill || null" :show-icon="true" />
          <p>{{ t("skillForge.result.bookCreatedHint") }}</p>
        </section>

        <section v-if="pendingRevision" class="pending-result" :class="{ 'is-failed': pendingRevision.failed }">
          <h4>{{ t("skillForge.result.success") }}</h4>
          <p>{{ t("skillForge.result.successHint") }}</p>
          <el-alert v-if="!pendingRulesetEnabled" type="error" :closable="false" show-icon :title="t('skillForge.rulesetDisabled')" />
          <div class="pending-result__actions">
            <el-button type="primary" :loading="resolvingPending" :disabled="resolvingPending || !pendingRulesetEnabled" @click="resolvePending(true)">{{ t("skillForge.result.accept") }}</el-button>
            <el-button :disabled="resolvingPending" @click="resolvePending(false)">{{ t("skillForge.result.discard") }}</el-button>
          </div>
        </section>
      </aside>
    </div>

    <GameDialog v-model="createDialogVisible" :title="t('skillForge.draft.chooseSource')" width="min(1080px, 96vw)">
      <el-form label-position="top">
        <el-form-item :label="t('skillForge.draft.source')">
          <el-segmented v-model="createSourceKind" :options="sourceKindOptions" />
        </el-form-item>
        <el-form-item v-if="createSourceKind === 'official'" :label="t('skillForge.draft.officialSkill')">
          <ItemBrowserPanel
            class="source-book-picker"
            :items="sourceBookBrowserItems"
            :empty-text="t('skillForge.draft.noSkillbooks')"
            :search-placeholder="t('skillForge.draft.searchSkillbooks')"
            search-width-class="md:w-32!"
            :grid-min-width="76"
            grid-gap-class="gap-[10px]"
            max-height="min(56vh, 460px)"
            :group-all-by-type="true"
            :virtualized="false"
            :active="createDialogVisible"
            :reset-on-activate="true"
            @item-click="selectSourceBook"
          >
            <template #item="{ item, onClick }">
              <div
                class="source-book-tile"
                :class="{ 'is-selected': createSourceBookItemId === item.itemId, 'is-disabled': sourceBookQuantity(item.itemId) < sourceBookMinCount }"
                role="option"
                :aria-selected="createSourceBookItemId === item.itemId"
                :aria-disabled="sourceBookQuantity(item.itemId) < sourceBookMinCount"
                tabindex="0"
                @click="onClick"
                @keydown.enter.prevent="onClick"
                @keydown.space.prevent="onClick"
              >
                <ItemCard :item-id="item.itemId" :quantity="sourceBookQuantity(item.itemId)" />
                <Check v-if="createSourceBookItemId === item.itemId" class="source-book-tile__check" />
              </div>
            </template>
          </ItemBrowserPanel>
        </el-form-item>
        <el-form-item v-if="createSourceKind === 'official'" :label="t('skillForge.draft.bookCount')">
          <el-input-number v-model="createSourceBookCount" :min="sourceBookMinCount" :max="sourceBookMaxCount" :step="sourceBookCountStep" />
          <span class="book-count-hint">{{ t("skillForge.draft.bookCountHint", { bonus: formatPercent(skillbookBonusForCount(createSourceBookCount)) }) }}</span>
        </el-form-item>
        <el-alert
          v-if="draftLimitReached && !matchingCreateDraft"
          type="warning"
          :closable="false"
          show-icon
          :title="t('skillForge.draft.limitReached', { max: maxDrafts })"
        />
      </el-form>
      <template #footer>
        <el-button @click="createDialogVisible = false">{{ t("game.cancel") }}</el-button>
        <el-button type="primary" :loading="creating" :disabled="!canStartDesign" @click="createDraft">{{ t("skillForge.draft.start") }}</el-button>
      </template>
    </GameDialog>
  </section>
</template>

<script setup lang="ts">
import { computed, nextTick, onMounted, onUnmounted, ref } from "vue"
import { useI18n } from "vue-i18n"
import { ElMessage, ElMessageBox } from "element-plus"
import { Check, Close, Delete, EditPen, FolderDelete, Grid, QuestionFilled, RefreshLeft, RefreshRight, Search, WarningFilled } from "@element-plus/icons-vue"
import { Background } from "@vue-flow/background"
import { Controls } from "@vue-flow/controls"
import { MiniMap } from "@vue-flow/minimap"
import { Handle, Position, VueFlow, useVueFlow, type Connection, type NodeMouseEvent } from "@vue-flow/core"
import "@vue-flow/core/dist/style.css"
import "@vue-flow/core/dist/theme-default.css"
import "@vue-flow/controls/dist/style.css"
import "@vue-flow/minimap/dist/style.css"
import { useGameStore } from "@stores/game"
import { useInventoryStore } from "@stores/inventory"
import ItemIcon from "@components/game/icon/ItemIcon.vue"
import ItemCard from "@components/game/item/Card.vue"
import ItemBrowserPanel, { type ItemBrowserItem } from "@components/game/item/ItemBrowserPanel.vue"
import ItemQuickRef from "@components/game/item/QuickRef.vue"
import GameDialog from "@components/game/common/GameDialog.vue"
import SkillQuickRef from "@components/game/skill/SkillQuickRef.vue"
import SkillQuickRefDetailContent from "@components/game/skill/SkillQuickRefDetailContent.vue"
import { getLocalizedSkillName, getSkillDefinition, mergeRuntimeSkillDefinitions } from "@services/skill-definition"
import { getItemDefinition, getRarityDefinition } from "@services/item-definition"
import { resolveErrorMessage } from "@services/error-messages"
import { APIClientError } from "@services/api"
import {
  acceptSkillForgeRevision,
  archiveCustomSkill,
  createBlankSkillForgeGraph,
  createOfficialSkillForgeGraph,
  createSkillForgeDraft,
  deleteSkillForgeDraft,
  discardSkillForgeRevision,
  fetchSkillForgeState,
  forgeSkillDraft,
  normalizeSkillForgeMaterialQuantity,
  previewSkillForgeDraft,
  rebaseSkillForgeDraft,
  updateSkillForgeDraft,
  type SkillForgeDraft,
  type SkillForgeGraph,
  type SkillForgeGraphNode,
  type SkillForgeModule,
  type SkillForgeParameterKind,
  type SkillForgePreview,
  type SkillForgeRevision,
  type SkillForgeState,
  type PlayerCustomSkill
} from "@services/skill-forge"

type ForgeCanvasData = {
  kind: SkillForgeGraphNode["kind"]
  moduleId?: string
  moduleItemId?: string
  compileOpKind?: string
  compileOpDirection?: string
  quantity?: number
  unitBasis?: number
  maxQuantity?: number
  effectKind?: string
  effectIndex?: number
  parameterKind?: SkillForgeParameterKind
  dslPath?: string
  targetNodeId?: string
}

type ForgeCanvasNode = { id: string; type: "forge"; position: { x: number; y: number }; data: ForgeCanvasData; selected?: boolean }
type ForgeCanvasEdge = { id: string; source: string; target: string; sourceHandle?: string | null; targetHandle?: string | null; selected?: boolean }
type GraphSnapshot = { nodes: ForgeCanvasNode[]; edges: ForgeCanvasEdge[] }
type DraftSaveSnapshot = {
  draftId: string
  expectedRevision: number
  sourceSkillbookCount: number
  graph: SkillForgeGraph
  changeVersion: number
}

const { t } = useI18n()
const gameStore = useGameStore()
const inventoryStore = useInventoryStore()
const { fitView, screenToFlowCoordinate, setCenter } = useVueFlow()

const loading = ref(true)
const loadError = ref("")
const saving = ref(false)
const draftDirty = ref(false)
const saveError = ref("")
const creating = ref(false)
const deletingDraft = ref(false)
const forging = ref(false)
const resolvingPending = ref(false)
const archivingSkillId = ref("")
const state = ref<SkillForgeState | null>(null)
const activeDraftId = ref("")
const canvasNodes = ref<ForgeCanvasNode[]>([])
const canvasEdges = ref<ForgeCanvasEdge[]>([])
const preview = ref<SkillForgePreview | null>(null)
const moduleQuery = ref("")
const draggingModuleId = ref("")
const selectedParameterId = ref("")
const mobileWorkspacePane = ref<"materials" | "graph" | "preview">("graph")
const previewPane = ref<"effects" | "costs">("effects")
const createDialogVisible = ref(false)
const createSourceKind = ref<"blank" | "official">("blank")
const createSourceBookItemId = ref("")
const createSourceBookCount = ref(200)
const lastAttempt = ref<SkillForgeRevision | null>(null)
const acceptedBook = ref<NonNullable<SkillForgeRevision["forgedSkillbook"]> | null>(null)
const undoStack = ref<GraphSnapshot[]>([])
const redoStack = ref<GraphSnapshot[]>([])
let saveTimer: ReturnType<typeof setTimeout> | null = null
let activeSavePromise: Promise<boolean> | null = null
let changeVersion = 0
let hydrating = false
let forgeIdempotencyKey = ""
let previewDraftId = ""
let previewRevisionNo = 0

const activeDraft = computed(() => state.value?.drafts.find((row) => row.id === activeDraftId.value) || null)
const saveStateLabel = computed(() => {
  if (saving.value) return t("skillForge.saving")
  if (saveError.value) return t("skillForge.saveFailed")
  if (draftDirty.value) return t("skillForge.unsaved")
  return t("skillForge.saved")
})
const availableDesigns = computed(() => (state.value?.drafts || []).filter((draft) => draft.status === "active" || draft.id === activeDraftId.value))
const disabledRulesetVersions = computed(() => new Set((state.value?.limits.disabledRulesetVersions || []).map(Number)))
const currentRulesetEnabled = computed(() => !disabledRulesetVersions.value.has(Number(state.value?.limits.rulesetVersion || 0)))
const sourceBookMinCount = computed(() => Math.max(1, Number(state.value?.limits.sourceSkillbookMinCount ?? 200)))
const sourceBookMaxCount = computed(() => Math.max(sourceBookMinCount.value, Number(state.value?.limits.sourceSkillbookMaxCount ?? 1000)))
const sourceBookCountStep = computed(() => Math.max(1, Number(state.value?.limits.sourceSkillbookCountStep ?? 100)))
const sourceBookBonusPerStep = computed(() => Math.max(0, Number(state.value?.limits.sourceSkillbookBonusPerStep ?? 0.10)))
const sourceBookMaxBonus = computed(() => Math.max(0, Number(state.value?.limits.sourceSkillbookMaxBonus ?? 0.80)))
const selectedSourceBook = computed(() => state.value?.sourceSkillbooks.find((book) => book.itemId === createSourceBookItemId.value))
const activeDraftCount = computed(() => (state.value?.drafts || []).filter((draft) => draft.status === "active").length)
const maxDrafts = computed(() => Math.max(1, Number(state.value?.limits.maxDrafts ?? 4)))
const draftLimitReached = computed(() => activeDraftCount.value >= maxDrafts.value)
const matchingCreateDraft = computed(() => (state.value?.drafts || []).find((draft) => draft.status === "active" && draft.sourceKind === createSourceKind.value && (
  createSourceKind.value === "blank" || draft.sourceSkillbookItemId === createSourceBookItemId.value
)) || null)
const sourceBookBrowserItems = computed<ItemBrowserItem[]>(() => (state.value?.sourceSkillbooks || []).map((book) => ({
  key: book.itemId,
  itemId: book.itemId,
  itemTypes: getItemDefinition(book.itemId)?.type || ["skillbook"],
  quantity: book.quantity,
  payload: book
})))
const canStartDesign = computed(() => {
  const sourceIsValid = createSourceKind.value === "blank" || Boolean(
    selectedSourceBook.value
    && selectedSourceBook.value.quantity >= createSourceBookCount.value
    && validSourceBookCount(createSourceBookCount.value)
  )
  return sourceIsValid && (!draftLimitReached.value || Boolean(matchingCreateDraft.value))
})
const inventoryMap = computed(() => new Map([
  ...(state.value?.inventory || []).map((row) => [row.itemId, row.quantity] as const),
  ...(state.value?.sourceSkillbooks || []).map((row) => [row.itemId, row.quantity] as const)
]))
const moduleMap = computed(() => new Map((state.value?.modules || []).map((row) => [row.id, row])))
const selectedParameter = computed(() => canvasNodes.value.find((node) => node.id === selectedParameterId.value && node.data.kind === "parameter") || null)
const filteredModules = computed(() => {
  const query = moduleQuery.value.trim().toLowerCase()
  return (state.value?.modules || []).filter((module) => {
    if (selectedParameter.value && !moduleCanModifyParameter(module, selectedParameter.value.data)) return false
    return !query
      || itemName(module.itemId).toLowerCase().includes(query)
      || opLabel(module.compileOp.kind).toLowerCase().includes(query)
      || moduleTargetLabel(module).toLowerCase().includes(query)
  })
})
const editableCustomSkills = computed(() => [] as PlayerCustomSkill[])
const hasDeletableSelection = computed(() => canvasNodes.value.some((node) => node.selected && canRemoveNode(node.id)))
const pendingRevision = computed(() => {
  const pendingId = activeDraft.value?.pendingRevisionId
  return (state.value?.pendingRevisions || []).find((row) => row.id === pendingId) || null
})
const pendingRulesetEnabled = computed(() => !pendingRevision.value || !disabledRulesetVersions.value.has(Number(pendingRevision.value.rulesetVersion || 0)))
const recipeRows = computed(() => {
  const recipe = { ...(preview.value?.recipe || {}) }
  if (activeDraft.value?.sourceKind === "official" && activeDraft.value.sourceSkillbookItemId) recipe[activeDraft.value.sourceSkillbookItemId] = (recipe[activeDraft.value.sourceSkillbookItemId] || 0) + activeDraft.value.sourceSkillbookCount
  return Object.entries(recipe).map(([itemId, required]) => {
    const owned = inventoryCount(itemId)
    return { itemId, required, owned, missing: Math.max(0, required - owned) }
  })
})
const hasRecipeShortage = computed(() => recipeRows.value.some((row) => row.missing > 0))
const finalPreviewSkill = computed(() => pendingRevision.value?.compiledSkill || preview.value?.compiledSkill || null)
const finalPreviewSkillId = computed(() => String(activeDraft.value?.sourceSkillId || finalPreviewSkill.value?.id || "custom:pending"))
const finalPreviewBaselineSkill = computed(() => {
  const draft = activeDraft.value
  if (!draft || draft.sourceKind !== "official" || !draft.sourceSkillId) return null
  return getSkillDefinition(draft.sourceSkillId) as unknown as Record<string, unknown> | null
})
const mandatoryRecipeHint = computed(() => t("skillForge.preview.mandatoryCostHint", {
	sealCount: Number(state.value?.limits.customizedNodeSealCount || 10),
	sealItem: itemName(String(state.value?.limits.customizedNodeSealItemId || "spell_seal_lock_plate")),
	catalystCount: Number(state.value?.limits.forgeCatalystCount || 5),
	catalystItem: itemName(String(state.value?.limits.forgeCatalystItemId || "final_volume_key_sigil")),
	minCooldown: Number(state.value?.limits.minCooldown || 1)
}))
function customizedNodeCostLabel(parameterID: string) {
	const modifications = parameterMaterialCount(parameterID)
	return t("skillForge.parameter.customizedCost", {
		modifications,
		count: modifications * Number(state.value?.limits.customizedNodeSealCount || 10),
		item: itemName(String(state.value?.limits.customizedNodeSealItemId || "spell_seal_lock_plate"))
	})
}
const displayedRanges = computed(() => pendingRevision.value?.rolls?.length ? pendingRevision.value.rolls : preview.value?.ranges || [])
const sourceIsStale = computed(() => (preview.value?.blockingReasons || []).some((reason) => reason.includes("source definition changed")))
const skillRarityLabel = computed(() => {
  const compiled = pendingRevision.value?.compiledSkill || preview.value?.compiledSkill
  const rarity = String(compiled?.rarity || "rare")
  return localizedName(`rarity.${rarity}.name`, rarity)
})
const sourceKindOptions = computed(() => [
  { label: t("skillForge.draft.blank"), value: "blank" },
  { label: t("skillForge.draft.official"), value: "official" }
])
const workspacePaneOptions = computed(() => [
  { label: t("skillForge.workspace.materials"), value: "materials" },
  { label: t("skillForge.workspace.graph"), value: "graph" },
  { label: t("skillForge.workspace.preview"), value: "preview" }
])
const previewPaneOptions = computed(() => [
  { label: t("skillForge.preview.finalSkill"), value: "effects" },
  { label: t("skillForge.preview.recipe"), value: "costs" }
])
const effectKindOptions = computed(() => (["damage", "heal", "status", "resource"] as const).map((value) => ({
  value,
  label: t(`skillForge.effect.${value}`)
})))

onMounted(() => {
  window.addEventListener("keydown", handleDeleteKey)
  void loadState()
})
onUnmounted(() => {
  if (saveTimer) clearTimeout(saveTimer)
  window.removeEventListener("keydown", handleDeleteKey)
})

async function loadState(preferredDraftId = "") {
  gameStore.restoreToken()
  if (!gameStore.token) return
  loading.value = true
  loadError.value = ""
  try {
    state.value = await fetchSkillForgeState(gameStore.token)
    createSourceBookCount.value = sourceBookMinCount.value
    mergeRuntimeSkillDefinitions(state.value.skills.map((skill) => skill.compiledSkill).filter(Boolean))
    const requestedID = preferredDraftId || activeDraftId.value
    const nextId = state.value.drafts.some((draft) => draft.id === requestedID) ? requestedID : state.value.drafts[0]?.id || ""
    activeDraftId.value = nextId
    draftDirty.value = false
    saveError.value = ""
    changeVersion = 0
    hydrateActiveDraft()
  } catch (error) {
    state.value = null
    activeDraftId.value = ""
    loadError.value = error instanceof Error ? error.message : t("skillForge.stateLoadFailed")
  } finally {
    loading.value = false
  }
}

function hydrateActiveDraft() {
  hydrating = true
  selectedParameterId.value = ""
  undoStack.value = []
  redoStack.value = []
  preview.value = null
  previewDraftId = ""
  previewRevisionNo = 0
  const draft = activeDraft.value
  if (!draft) {
    canvasNodes.value = []
    canvasEdges.value = []
    hydrating = false
    return
  }
  let needsDslExpansion = false
  if (draft.sourceKind === "official") {
    const definition = getSkillDefinition(draft.sourceSkillId || "") as unknown as Record<string, unknown> | undefined
    if (definition) {
      const expanded = createOfficialSkillForgeGraph(draft.sourceSkillId || "", definition)
      const existingParameterIDs = new Set(draft.graph.nodes.filter((node) => node.kind === "parameter").map((node) => node.id))
      needsDslExpansion = expanded.nodes.some((node) => node.kind === "parameter" && !existingParameterIDs.has(node.id))
      if (needsDslExpansion) draft.graph = expandDraftGraph(draft.graph, expanded)
    }
  } else {
    const effectNodes = draft.graph.nodes
      .filter((node) => node.kind === "effect")
      .sort((left, right) => Number(left.effectIndex || 0) - Number(right.effectIndex || 0))
    const expanded = createBlankSkillForgeGraph("blank", "", effectNodes.map((node) => String(node.effectKind || "empty")))
    const existingParameterIDs = new Set(draft.graph.nodes.filter((node) => node.kind === "parameter").map((node) => node.id))
    needsDslExpansion = expanded.nodes.some((node) => node.kind === "parameter" && !existingParameterIDs.has(node.id))
    if (needsDslExpansion) draft.graph = expandDraftGraph(draft.graph, expanded)
  }
  canvasNodes.value = draft.graph.nodes.map(toCanvasNode)
  canvasEdges.value = draft.graph.edges.map((edge) => ({
    id: edge.id,
    source: edge.source,
    target: edge.target,
    sourceHandle: edge.sourceHandle,
    targetHandle: edge.targetHandle
  }))
	const normalizedAddedEffectHooks = ensureAdditionalEffectHooks()
	const normalizedAttachments = normalizeMaterialAttachments()
	const normalizedTargets = normalizeOfficialMaterialTargets()
	const normalizedStructure = normalizeOfficialStructure()
	const normalizedCoreLayout = normalizeCoreLayout()
	normalizeBlankEffectKinds()
  nextTick(() => {
    hydrating = false
    fitGraph()
    if (needsDslExpansion || normalizedAddedEffectHooks || normalizedAttachments || normalizedTargets || normalizedStructure || normalizedCoreLayout) markDirty()
    else void refreshPreview()
  })
}

function expandDraftGraph(previous: SkillForgeGraph, expanded: SkillForgeGraph): SkillForgeGraph {
  const next: SkillForgeGraph = {
    ...expanded,
    source: { ...expanded.source, definitionHash: previous.source.definitionHash },
    nodes: expanded.nodes.map((node) => ({ ...node, position: { ...node.position } })),
    edges: expanded.edges.map((edge) => ({ ...edge }))
  }
  const materials = previous.nodes.filter((node) => node.kind === "material")
  for (const material of materials) {
    const module = material.moduleId ? moduleMap.value.get(material.moduleId) : undefined
    if (!module) continue
    if (module.compileOp.kind === "effect_slot") {
      next.nodes.push({ ...material, targetNodeId: undefined, position: { x: 620, y: 168 } })
      next.edges.push(
        { id: `edge-upgrade-${material.id}-in`, source: "source", sourceHandle: "skill", target: material.id, targetHandle: "in" },
        { id: `edge-upgrade-${material.id}-out`, source: material.id, sourceHandle: "out", target: "output", targetHandle: "skill" }
      )
      continue
    }
    const exactTarget = material.targetNodeId ? next.nodes.find((node) => node.id === material.targetNodeId && node.kind === "parameter" && moduleCanModifyParameter(module, node)) : undefined
    const target = exactTarget || compatibleParameterNodes(module, next.nodes, Number(material.effectIndex || 0))[0]
    if (!target) continue
    next.nodes.push({ ...material, targetNodeId: target.id, effectIndex: target.effectIndex, position: { x: target.position.x, y: target.position.y + 176 } })
    next.edges.push(...materialAttachmentEdges(material.id, module, target))
  }
  return next
}

async function selectDraft(nextDraftId: string) {
  if (!nextDraftId || nextDraftId === activeDraftId.value) return
  if (!(await flushActiveDraft())) return
  forgeIdempotencyKey = ""
  activeDraftId.value = nextDraftId
  draftDirty.value = false
  saveError.value = ""
  hydrateActiveDraft()
}

function toCanvasNode(node: SkillForgeGraphNode): ForgeCanvasNode {
  const module = node.moduleId ? moduleMap.value.get(node.moduleId) : undefined
  return {
    id: node.id,
    type: "forge",
    position: { ...node.position },
    data: {
      kind: node.kind,
      moduleId: node.moduleId,
      moduleItemId: module?.itemId,
      compileOpKind: module?.compileOp.kind,
      compileOpDirection: String(module?.compileOp.direction || ""),
      quantity: node.quantity,
      unitBasis: module?.unitBasis,
      maxQuantity: module ? module.unitBasis * module.maxBasisMultiplier : undefined,
      effectKind: node.effectKind,
      effectIndex: node.effectIndex,
      parameterKind: node.parameterKind,
      dslPath: node.dslPath,
      targetNodeId: node.targetNodeId
    } satisfies ForgeCanvasData
  }
}

function serializeGraph(): SkillForgeGraph {
  const source = activeDraft.value?.graph.source || { kind: "blank" as const }
  return {
    version: 1,
    source: { ...source },
    nodes: canvasNodes.value.map((node) => {
      const data = node.data as ForgeCanvasData
      return {
        id: node.id,
        kind: data.kind,
        ...(data.moduleId ? { moduleId: data.moduleId } : {}),
        ...(data.quantity ? { quantity: Math.floor(data.quantity) } : {}),
        ...(data.effectKind ? { effectKind: data.effectKind as "empty" | "damage" | "heal" | "status" | "resource" } : {}),
        ...(typeof data.effectIndex === "number" ? { effectIndex: data.effectIndex } : {}),
        ...(data.parameterKind ? { parameterKind: data.parameterKind } : {}),
        ...(data.dslPath ? { dslPath: data.dslPath } : {}),
        ...(data.targetNodeId ? { targetNodeId: data.targetNodeId } : {}),
        position: { x: node.position.x, y: node.position.y }
      }
    }),
    edges: canvasEdges.value.map((edge) => ({
      id: edge.id,
      source: edge.source,
      sourceHandle: edge.sourceHandle || "",
      target: edge.target,
      targetHandle: edge.targetHandle || ""
    }))
  }
}

function moduleForData(raw: unknown): SkillForgeModule | undefined {
  const data = raw as ForgeCanvasData
  return data.moduleId ? moduleMap.value.get(data.moduleId) : undefined
}

function captureBeforeChange() {
  if (hydrating) return
  undoStack.value.push(snapshot())
  if (undoStack.value.length > 40) undoStack.value.shift()
  redoStack.value = []
}

function snapshot(): GraphSnapshot {
  return {
    nodes: canvasNodes.value.map(cloneCanvasNode),
    edges: canvasEdges.value.map(cloneCanvasEdge)
  }
}

function restore(next: GraphSnapshot) {
  hydrating = true
  canvasNodes.value = next.nodes.map(cloneCanvasNode)
  canvasEdges.value = next.edges.map(cloneCanvasEdge)
  nextTick(() => { hydrating = false; markDirty() })
}

function cloneCanvasNode(node: ForgeCanvasNode): ForgeCanvasNode {
  return {
    id: node.id,
    type: "forge",
    position: { x: node.position.x, y: node.position.y },
    data: { ...(node.data as ForgeCanvasData) },
    ...(node.selected ? { selected: true } : {})
  }
}

function cloneCanvasEdge(edge: ForgeCanvasEdge): ForgeCanvasEdge {
  return {
    id: edge.id,
    source: edge.source,
    target: edge.target,
    sourceHandle: edge.sourceHandle || null,
    targetHandle: edge.targetHandle || null,
    ...(edge.selected ? { selected: true } : {})
  }
}

function undo() {
  const previous = undoStack.value.pop()
  if (!previous) return
  redoStack.value.push(snapshot())
  restore(previous)
}

function redo() {
  const next = redoStack.value.pop()
  if (!next) return
  undoStack.value.push(snapshot())
  restore(next)
}

function startModuleDrag(event: DragEvent, moduleId: string) {
  draggingModuleId.value = moduleId
  event.dataTransfer?.setData("application/x-skill-forge-module", moduleId)
  if (event.dataTransfer) event.dataTransfer.effectAllowed = "copy"
}

function endModuleDrag() {
  draggingModuleId.value = ""
}

function handleNodeClick({ node }: NodeMouseEvent) {
  const data = node.data as ForgeCanvasData
  for (const candidate of canvasNodes.value) candidate.selected = candidate.id === node.id
  selectedParameterId.value = data.kind === "parameter" && parameterCompatibleModuleCount(data) > 0 ? node.id : ""
}

function handleNodeDragStop({ node }: NodeMouseEvent) {
  const data = node.data as ForgeCanvasData
  if (data.kind === "material" && data.targetNodeId) rebuildParameterMaterialEdges(data.targetNodeId)
  markDirty()
}

function clearParameterSelection() {
  selectedParameterId.value = ""
}

function attachModuleToSelected(module: SkillForgeModule) {
  const target = selectedParameter.value
  if (!target || !moduleCanModifyParameter(module, target.data)) return
  const requiredKind = effectKindForModule(module)
  const effectIndex = Number(target.data.effectIndex || 0)
  if (requiredKind && !canAssignEffectKind(effectIndex, requiredKind)) {
    ElMessage.warning(t("skillForge.effect.incompatible", { effect: t(`skillForge.effect.${requiredKind}`) }))
    return
  }
  captureBeforeChange()
  addMaterialToParameter(module, target)
  markDirty()
}

function addMaterialToParameter(module: SkillForgeModule, target: ForgeCanvasNode) {
	const effectIndex = Number(target.data.effectIndex || 0)
	const requiredKind = effectKindForModule(module)
	const effect = effectNodes().find((node) => Number(node.data.effectIndex || 0) === effectIndex)
	if (requiredKind && effect && canChooseEffectKind(effect.data)) assignEffectKind(effectIndex, requiredKind)
  const id = `material-${Date.now()}-${Math.random().toString(36).slice(2, 7)}`
  const siblings = canvasNodes.value.filter((node) => node.data.kind === "material" && node.data.targetNodeId === target.id).length
  const node: SkillForgeGraphNode = {
    id,
    kind: "material",
    moduleId: module.id,
    quantity: module.unitBasis,
    effectIndex,
    targetNodeId: target.id,
    position: { x: target.position.x, y: target.position.y + 176 + siblings * 240 }
  }
  canvasNodes.value.push(toCanvasNode(node))
  rebuildParameterMaterialEdges(target.id)
  nextTick(() => {
    const materialIDs = parameterMaterialNodes(target.id).map((row) => row.id)
    void fitView({ nodes: [target.id, ...materialIDs], padding: 0.24, minZoom: 0.55, maxZoom: 0.86, duration: 220 })
  })
}

function dropModule(event: DragEvent) {
  const moduleId = event.dataTransfer?.getData("application/x-skill-forge-module") || ""
  const module = moduleMap.value.get(moduleId)
  draggingModuleId.value = ""
  if (!module || !activeDraft.value) return
  captureBeforeChange()
  const position = screenToFlowCoordinate({ x: event.clientX, y: event.clientY })
	if (module.compileOp.kind === "effect_slot") {
		if (!addEffectSlotMaterial(module, position)) {
			undoStack.value.pop()
			return
		}
		markDirty()
		return
	}
	const target = selectedParameter.value && moduleCanModifyParameter(module, selectedParameter.value.data)
		? selectedParameter.value
		: nearestCompatibleParameter(module, position)
	if (target) {
		const requiredKind = effectKindForModule(module)
		const effectIndex = Number(target.data.effectIndex || 0)
		if (requiredKind && !canAssignEffectKind(effectIndex, requiredKind)) {
			undoStack.value.pop()
			ElMessage.warning(t("skillForge.effect.incompatible", { effect: t(`skillForge.effect.${requiredKind}`) }))
			return
		}
		addMaterialToParameter(module, target)
		markDirty()
		return
	}
	if (activeDraft.value.sourceKind === "official") {
		if (!target) {
			undoStack.value.pop()
			ElMessage.warning(t("skillForge.parameter.noCompatibleSlot"))
			return
		}
	}
	const effectIndex = nearestEffectIndex(position.y)
	const requiredKind = effectKindForModule(module)
	if (requiredKind && !canAssignEffectKind(effectIndex, requiredKind)) {
		undoStack.value.pop()
		ElMessage.warning(t("skillForge.effect.incompatible", { effect: t(`skillForge.effect.${requiredKind}`) }))
		return
	}
  const id = `material-${Date.now()}-${Math.random().toString(36).slice(2, 7)}`
  const node: SkillForgeGraphNode = { id, kind: "material", moduleId, quantity: module.unitBasis, effectIndex, position }
  canvasNodes.value.push(toCanvasNode(node))
	if (module.typedPorts.inputs[0]?.type === "effect" && module.typedPorts.outputs[0]?.type === "effect") {
		assignEffectKind(effectIndex, requiredKind)
		appendMaterialToEffectBranch(id, effectIndex)
	} else if (module.typedPorts.inputs[0]?.type === "skill" && module.typedPorts.outputs[0]?.type === "skill") {
		appendMaterialToSkillBranch(id)
	}
  markDirty()
}

function serializeCanvasParameter(node: ForgeCanvasNode): SkillForgeGraphNode {
  return {
    id: node.id,
    kind: "parameter",
    ...(node.data.effectKind ? { effectKind: normalizeEffectKind(String(node.data.effectKind)) } : {}),
    effectIndex: Number(node.data.effectIndex || 0),
    parameterKind: node.data.parameterKind,
    dslPath: node.data.dslPath,
    position: { ...node.position }
  }
}

function materialAttachmentEdges(materialId: string, module: SkillForgeModule, target: SkillForgeGraphNode): SkillForgeGraph["edges"] {
  const effectIndex = Number(target.effectIndex || 0)
  if (module.compileOp.kind === "target_status_condition" || module.compileOp.kind === "caster_status_condition") {
    return [
      { id: `edge-${materialId}-in`, source: target.id, sourceHandle: "modifier", target: materialId, targetHandle: "in" },
      { id: `edge-${materialId}-true`, source: materialId, sourceHandle: "true", target: `effect-${effectIndex + 1}`, targetHandle: "in" },
      { id: `edge-${materialId}-false`, source: materialId, sourceHandle: "false", target: "output", targetHandle: "effect" }
    ]
  }
  return [{ id: `edge-${materialId}-in`, source: target.id, sourceHandle: "modifier", target: materialId, targetHandle: "in" }]
}

function materialCanChain(data: ForgeCanvasData): boolean {
  if (isBranchCondition(data)) return false
  const module = moduleForData(data)
  return Boolean(module && module.typedPorts.inputs[0]?.type === module.typedPorts.outputs[0]?.type)
}

function parameterMaterialNodes(targetNodeId: string): ForgeCanvasNode[] {
  return canvasNodes.value
    .filter((node) => node.data.kind === "material" && node.data.targetNodeId === targetNodeId)
    .sort((a, b) => a.position.y - b.position.y || a.position.x - b.position.x || a.id.localeCompare(b.id))
}

function rebuildParameterMaterialEdges(targetNodeId: string): boolean {
  const target = canvasNodes.value.find((node) => node.id === targetNodeId && node.data.kind === "parameter")
  if (!target) return false
  const materials = parameterMaterialNodes(targetNodeId)
  if (materials.length === 0) return false
  const materialIDs = new Set(materials.map((node) => node.id))
  const retained = canvasEdges.value.filter((edge) => !materialIDs.has(edge.source) && !materialIDs.has(edge.target))
  const rebuilt: ForgeCanvasEdge[] = []
  let previousId = target.id
  let previousHandle = "modifier"
  for (const material of materials.filter((node) => materialCanChain(node.data))) {
    rebuilt.push({
      id: `edge-${previousId}-${material.id}`,
      source: previousId,
      sourceHandle: previousHandle,
      target: material.id,
      targetHandle: "in"
    })
    previousId = material.id
    previousHandle = "out"
  }
  for (const material of materials.filter((node) => !materialCanChain(node.data))) {
    const module = moduleForData(material.data)
    if (module) rebuilt.push(...materialAttachmentEdges(material.id, module, serializeCanvasParameter(target)))
  }
  const before = edgeSetSignature(canvasEdges.value)
  canvasEdges.value = [...retained, ...rebuilt]
  return before !== edgeSetSignature(canvasEdges.value)
}

function edgeSetSignature(edges: ForgeCanvasEdge[]): string {
  return edges
    .map((edge) => `${edge.source}:${edge.sourceHandle || ""}>${edge.target}:${edge.targetHandle || ""}`)
    .sort()
    .join("|")
}

function normalizeOfficialStructure(): boolean {
  const draft = activeDraft.value
  if (!draft || draft.sourceKind !== "official") return false
  const definition = getSkillDefinition(draft.sourceSkillId || "") as unknown as Record<string, unknown> | undefined
  if (!definition) return false
  const template = createOfficialSkillForgeGraph(draft.sourceSkillId || "", definition)
  const coreIDs = new Set(template.nodes.map((node) => node.id))
  const before = edgeSetSignature(canvasEdges.value)
  canvasEdges.value = [
    ...canvasEdges.value.filter((edge) => !(coreIDs.has(edge.source) && coreIDs.has(edge.target))),
    ...template.edges
  ]
  const targetIDs = new Set(
    canvasNodes.value
      .filter((node) => node.data.kind === "material" && node.data.targetNodeId)
      .map((node) => String(node.data.targetNodeId))
  )
  for (const targetID of targetIDs) rebuildParameterMaterialEdges(targetID)
  return before !== edgeSetSignature(canvasEdges.value)
}

function normalizeCoreLayout(): boolean {
	const draft = activeDraft.value
	if (!draft) return false
	const effectKinds = canvasNodes.value
		.filter((node) => node.data.kind === "effect")
		.sort((left, right) => Number(left.data.effectIndex || 0) - Number(right.data.effectIndex || 0))
		.map((node) => normalizeEffectKind(String(node.data.effectKind || "empty")))
	const definition = draft.sourceKind === "official"
		? getSkillDefinition(draft.sourceSkillId || "") as unknown as Record<string, unknown> | undefined
		: undefined
	const template = definition
		? createOfficialSkillForgeGraph(draft.sourceSkillId || "", definition)
		: createBlankSkillForgeGraph("blank", "", effectKinds)
	const templateCore = template.nodes.filter((node) => node.kind !== "material")
	const currentByID = new Map(canvasNodes.value.map((node) => [node.id, node]))
	const expectedRows = new Map<number, typeof templateCore>()
	for (const node of templateCore) {
		const row = expectedRows.get(node.position.y) || []
		row.push(node)
		expectedRows.set(node.position.y, row)
	}
	let previousRowY = Number.NEGATIVE_INFINITY
	let ordered = true
	for (const [, expectedNodes] of [...expectedRows.entries()].sort(([left], [right]) => left - right)) {
		const currentNodes = expectedNodes.map((node) => currentByID.get(node.id)).filter((node): node is ForgeCanvasNode => Boolean(node))
		if (currentNodes.length !== expectedNodes.length) continue
		const rowY = Math.min(...currentNodes.map((node) => node.position.y))
		const rowYMax = Math.max(...currentNodes.map((node) => node.position.y))
		const expectedOrder = expectedNodes.slice().sort((left, right) => left.position.x - right.position.x).map((node) => node.id).join("|")
		const currentOrder = currentNodes.slice().sort((left, right) => left.position.x - right.position.x).map((node) => node.id).join("|")
		if (rowYMax - rowY > 8 || rowY <= previousRowY || currentOrder !== expectedOrder) ordered = false
		previousRowY = rowY
	}
	if (ordered) return false
	for (const expected of templateCore) {
		const current = currentByID.get(expected.id)
		if (current) current.position = { ...expected.position }
	}
	const materialsByTarget = new Map<string, ForgeCanvasNode[]>()
	for (const material of canvasNodes.value.filter((node) => node.data.kind === "material" && node.data.targetNodeId)) {
		const targetID = String(material.data.targetNodeId)
		const siblings = materialsByTarget.get(targetID) || []
		siblings.push(material)
		materialsByTarget.set(targetID, siblings)
	}
	for (const [targetID, materials] of materialsByTarget) {
		const target = currentByID.get(targetID)
		if (!target) continue
		materials.sort((left, right) => left.position.y - right.position.y || left.position.x - right.position.x || left.id.localeCompare(right.id))
		materials.forEach((material, index) => {
			material.position = { x: target.position.x, y: target.position.y + 176 + index * 240 }
		})
	}
	return true
}

function normalizeMaterialAttachments(): boolean {
  let changed = false
  for (const node of canvasNodes.value) {
    if (node.data.kind !== "material") continue
    const module = moduleForData(node.data)
    if (!module) continue
    const current = Math.max(0, Math.floor(Number(node.data.quantity || 0)))
    const rarity = String(getItemDefinition(module.itemId)?.rarity || "")
    const normalized = normalizeSkillForgeMaterialQuantity(current, module, rarity)
    if (current !== normalized) {
      node.data.quantity = normalized
      changed = true
    }
    if (!node.data.targetNodeId) continue
  }
  const targetIDs = new Set(
    canvasNodes.value
      .filter((node) => node.data.kind === "material" && node.data.targetNodeId)
      .map((node) => String(node.data.targetNodeId))
  )
  for (const targetID of targetIDs) changed = rebuildParameterMaterialEdges(targetID) || changed
  return changed
}

function normalizeOfficialMaterialTargets(): boolean {
	if (activeDraft.value?.sourceKind !== "official") return false
	let changed = false
	const siblingByTarget = new Map<string, number>()
	const materials = canvasNodes.value.filter((node) => node.data.kind === "material" && node.data.targetNodeId)
	for (const material of materials) {
		const module = moduleForData(material.data)
		const current = canvasNodes.value.find((node) => node.id === material.data.targetNodeId && node.data.kind === "parameter")
		if (module && current && moduleCanModifyParameter(module, current.data)) {
			siblingByTarget.set(current.id, (siblingByTarget.get(current.id) || 0) + 1)
		}
	}
	for (const material of materials) {
		const module = moduleForData(material.data)
		if (!module) continue
		const current = canvasNodes.value.find((node) => node.id === material.data.targetNodeId && node.data.kind === "parameter")
		if (current && moduleCanModifyParameter(module, current.data)) continue
		const preferredEffect = Number(material.data.effectIndex || 0)
		const candidates = canvasNodes.value.filter((node) => node.data.kind === "parameter" && moduleCanModifyParameter(module, node.data))
		const target = candidates.find((node) => Number(node.data.effectIndex || 0) === preferredEffect) || candidates[0]
		if (!target) continue
		const sibling = siblingByTarget.get(target.id) || 0
		siblingByTarget.set(target.id, sibling + 1)
		material.data.targetNodeId = target.id
		material.data.effectIndex = Number(target.data.effectIndex || 0)
		material.position = { x: target.position.x, y: target.position.y + 176 + sibling * 240 }
		changed = true
	}
	return changed
}

function connectNodes(connection: Connection) {
  if (!connection.source || !connection.target || !connection.sourceHandle || !connection.targetHandle) return
  if (!portsMatch(connection)) {
    ElMessage.warning(t("skillForge.portMismatch"))
    return
  }
  captureBeforeChange()
  const sourceNode = canvasNodes.value.find((node) => node.id === connection.source)
  const targetNode = canvasNodes.value.find((node) => node.id === connection.target)
	const sourceEffectIndex = Number(sourceNode?.data.effectIndex || 0)
	const targetModule = targetNode?.data.kind === "material" ? moduleForData(targetNode.data) : undefined
	const requiredKind = targetModule ? effectKindForModule(targetModule) : ""
	if (sourceNode?.data.kind === "effect" && requiredKind && !canAssignEffectKind(sourceEffectIndex, requiredKind)) {
		undoStack.value.pop()
		ElMessage.warning(t("skillForge.effect.incompatible", { effect: t(`skillForge.effect.${requiredKind}`) }))
		return
	}
  if (sourceNode && targetNode?.data.kind === "material" && (sourceNode.data.kind === "effect" || sourceNode.data.kind === "material")) {
    targetNode.data.effectIndex = Number(sourceNode.data.effectIndex || 0)
			assignEffectKind(Number(targetNode.data.effectIndex || 0), requiredKind)
  }
  if (sourceNode?.data.kind === "parameter" && targetNode?.data.kind === "material") {
    const targetModule = moduleForData(targetNode.data)
    if (!targetModule || !moduleCanModifyParameter(targetModule, sourceNode.data)) {
      undoStack.value.pop()
      ElMessage.warning(t("skillForge.parameter.noCompatibleSlot"))
      return
    }
    targetNode.data.targetNodeId = sourceNode.id
    targetNode.data.effectIndex = Number(sourceNode.data.effectIndex || 0)
  }
  canvasEdges.value.push({
    id: `edge-${Date.now()}-${Math.random().toString(36).slice(2, 6)}`,
    source: connection.source,
    target: connection.target,
    sourceHandle: connection.sourceHandle,
    targetHandle: connection.targetHandle
  })
  markDirty()
}

function portsMatch(connection: Connection): boolean {
  const source = canvasNodes.value.find((node) => node.id === connection.source)
  const target = canvasNodes.value.find((node) => node.id === connection.target)
  if (!source || !target) return false
  return portType(source, connection.sourceHandle || "", false) === portType(target, connection.targetHandle || "", true)
}

function portType(node: ForgeCanvasNode, handle: string, input: boolean): string {
  const data = node.data as ForgeCanvasData
  if (data.kind === "source" && !input) return handle
  if (data.kind === "output" && input) return handle
  if (data.kind === "effect") return handle === "in" && input || handle === "out" && !input ? "effect" : ""
  if (data.kind === "parameter") {
    const baseType = data.parameterKind === "requirements" || data.parameterKind === "hp_cost" || data.parameterKind === "mp_cost" || data.parameterKind === "cooldown" ? "skill" : "effect"
    if (input && handle === "in" || !input && handle === "out") return baseType
    if (!input && handle === "modifier") return data.parameterKind === "conditions" ? "condition" : baseType
    return ""
  }
  const module = moduleForData(data)
  if (!input && handle === "out" && isBranchCondition(data)) return "effect"
  const ports = input ? module?.typedPorts.inputs : module?.typedPorts.outputs
  return ports?.find((port) => port.id === handle)?.type || ""
}

function removeCanvasNode(id: string) {
	if (!canRemoveNode(id)) return
  const removed = canvasNodes.value.find((node) => node.id === id)
  const targetNodeId = removed?.data.targetNodeId
  captureBeforeChange()
	if (removed?.data.compileOpKind === "effect_slot") trimEffectNodeCount(sourceBaseEffectCount())
  canvasNodes.value = canvasNodes.value.filter((node) => node.id !== id)
  canvasEdges.value = canvasEdges.value.filter((edge) => edge.source !== id && edge.target !== id)
	if (targetNodeId) rebuildParameterMaterialEdges(targetNodeId)
	normalizeBlankEffectKinds()
  markDirty()
}

function canRemoveNode(id: string): boolean {
  const node = canvasNodes.value.find((row) => row.id === id)
  if (!node) return false
  const data = node.data as ForgeCanvasData
  return data.kind === "material"
}

function removeSelection() {
  const nodeIDs = new Set(canvasNodes.value.filter((node) => node.selected && canRemoveNode(node.id)).map((node) => node.id))
	if (nodeIDs.size === 0) return
  const targetIDs = new Set(
    canvasNodes.value
      .filter((node) => nodeIDs.has(node.id) && node.data.targetNodeId)
      .map((node) => String(node.data.targetNodeId))
  )
  captureBeforeChange()
	if (canvasNodes.value.some((node) => nodeIDs.has(node.id) && node.data.compileOpKind === "effect_slot")) trimEffectNodeCount(sourceBaseEffectCount())
  canvasNodes.value = canvasNodes.value.filter((node) => !nodeIDs.has(node.id))
	canvasEdges.value = canvasEdges.value.filter((edge) => !nodeIDs.has(edge.source) && !nodeIDs.has(edge.target))
  for (const targetID of targetIDs) rebuildParameterMaterialEdges(targetID)
  normalizeBlankEffectKinds()
  markDirty()
}

function handleDeleteKey(event: KeyboardEvent) {
  if (event.key !== "Delete" && event.key !== "Backspace") return
  const target = event.target as HTMLElement | null
  if (target?.matches("input, textarea, [contenteditable='true']")) return
  if (!hasDeletableSelection.value) return
  event.preventDefault()
  removeSelection()
}

function updateQuantity(id: string, quantity: number) {
  captureBeforeChange()
  const node = canvasNodes.value.find((row) => row.id === id)
  if (!node) return
  const data = node.data as ForgeCanvasData
	const unitBasis = Math.max(1, Number(data.unitBasis || 1))
	let nextQuantity = Math.max(unitBasis, Math.min(data.maxQuantity || unitBasis, Math.round(quantity / unitBasis) * unitBasis))
  data.quantity = nextQuantity
	if (data.compileOpKind === "effect_slot") {
		const desired = Math.min(state.value?.limits.maxEffects || 3, sourceBaseEffectCount() + Math.floor(nextQuantity / unitBasis))
		if (effectNodes().length > desired) trimEffectNodeCount(desired)
		else ensureEffectNodeCount(desired)
	}
  markDirty()
}

function autoLayout() {
  captureBeforeChange()
  if (activeDraft.value && canvasNodes.value.some((node) => node.data.kind === "parameter")) {
    const definition = activeDraft.value.sourceKind === "official"
      ? getSkillDefinition(activeDraft.value.sourceSkillId || "") as unknown as Record<string, unknown> | undefined
      : undefined
    const layout = definition
      ? createOfficialSkillForgeGraph(activeDraft.value.sourceSkillId || "", definition)
      : createBlankSkillForgeGraph("blank", "", effectNodes().map((node) => String(node.data.effectKind || "damage")))
    if (layout) {
      const positions = new Map(layout.nodes.map((node) => [node.id, node.position]))
      for (const node of canvasNodes.value) {
        const base = positions.get(node.id)
        if (base) node.position = { ...base }
      }
      ensureAdditionalEffectHooks()
      const siblingByTarget = new Map<string, number>()
      for (const node of canvasNodes.value.filter((row) => row.data.kind === "material" && row.data.targetNodeId)) {
        const target = canvasNodes.value.find((row) => row.id === node.data.targetNodeId)
        if (!target) continue
        const sibling = siblingByTarget.get(target.id) || 0
        siblingByTarget.set(target.id, sibling + 1)
        node.position = { x: target.position.x, y: target.position.y + 176 + sibling * 240 }
      }
      if (activeDraft.value.sourceKind === "official") normalizeOfficialStructure()
      markDirty()
      nextTick(fitGraph)
      return
    }
  }
  const source = canvasNodes.value.find((node) => (node.data as ForgeCanvasData).kind === "source")
  const output = canvasNodes.value.find((node) => (node.data as ForgeCanvasData).kind === "output")
  if (source) source.position = { x: 40, y: 280 }
  if (output) output.position = { x: 1040, y: 280 }
  const middle = canvasNodes.value.filter((node) => node !== source && node !== output)
  middle.forEach((node, index) => {
    const column = Math.floor(index / 5)
    const row = index % 5
    node.position = { x: 360 + column * 320, y: 60 + row * 210 }
  })
  markDirty()
  nextTick(fitGraph)
}

function markDirty() {
  if (hydrating || !activeDraft.value) return
  changeVersion += 1
  draftDirty.value = true
  saveError.value = ""
  preview.value = null
  previewDraftId = ""
  previewRevisionNo = 0
  if (saveTimer) clearTimeout(saveTimer)
  const scheduledDraftId = activeDraft.value.id
  const scheduledVersion = changeVersion
  saveTimer = setTimeout(() => {
    saveTimer = null
    if (activeDraftId.value !== scheduledDraftId || changeVersion !== scheduledVersion) return
    void flushActiveDraft()
  }, 700)
}

function captureDraftSaveSnapshot(): DraftSaveSnapshot | null {
  const draft = activeDraft.value
  if (!draft) return null
  return {
    draftId: draft.id,
    expectedRevision: draft.revisionNo,
    sourceSkillbookCount: draft.sourceSkillbookCount || 0,
    graph: serializeGraph(),
    changeVersion
  }
}

async function persistDraftSnapshot(snapshot: DraftSaveSnapshot): Promise<boolean> {
  if (!gameStore.token) return false
  saving.value = true
  try {
    const updated = await updateSkillForgeDraft(
      gameStore.token,
      snapshot.draftId,
      snapshot.expectedRevision,
      snapshot.sourceSkillbookCount,
      snapshot.graph
    )
    const index = state.value?.drafts.findIndex((row) => row.id === snapshot.draftId) ?? -1
    if (state.value && index >= 0) state.value.drafts[index] = updated
    if (activeDraftId.value === snapshot.draftId) {
      saveError.value = ""
      if (snapshot.changeVersion === changeVersion) {
        draftDirty.value = false
        void refreshPreview(snapshot.draftId, updated.revisionNo)
      }
    }
    return true
  } catch (error) {
    if (activeDraftId.value === snapshot.draftId) {
      saveError.value = error instanceof Error ? error.message : t("skillForge.saveFailed")
      draftDirty.value = true
      ElMessage.error(saveError.value)
    }
    return false
  } finally {
    saving.value = false
  }
}

async function flushActiveDraft(): Promise<boolean> {
  if (saveTimer) {
    clearTimeout(saveTimer)
    saveTimer = null
  }
  while (true) {
    if (activeSavePromise) {
      if (!(await activeSavePromise)) return false
      continue
    }
    if (!draftDirty.value) return !saveError.value
    const snapshot = captureDraftSaveSnapshot()
    if (!snapshot || !gameStore.token) return false
    const promise = persistDraftSnapshot(snapshot)
    activeSavePromise = promise
    const saved = await promise
    if (activeSavePromise === promise) activeSavePromise = null
    if (!saved) return false
    if (activeDraftId.value !== snapshot.draftId || snapshot.changeVersion === changeVersion) return true
  }
}

async function retrySaveDraft() {
  await flushActiveDraft()
}

async function refreshPreview(draftId = activeDraft.value?.id || "", expectedRevision = activeDraft.value?.revisionNo || 0): Promise<boolean> {
  if (!draftId || !expectedRevision || !gameStore.token) return false
  try {
    const result = await previewSkillForgeDraft(gameStore.token, draftId)
    if (activeDraft.value?.id !== draftId || activeDraft.value.revisionNo !== expectedRevision || draftDirty.value) return false
    preview.value = result
    previewDraftId = draftId
    previewRevisionNo = expectedRevision
    return true
  } catch (error) {
    if (activeDraft.value?.id === draftId && activeDraft.value.revisionNo === expectedRevision) {
      preview.value = null
      previewDraftId = ""
      previewRevisionNo = 0
    }
    return false
  }
}

async function rebaseDraft() {
  if (!activeDraft.value || !gameStore.token) return
  if (!(await flushActiveDraft())) return
  await ElMessageBox.confirm(t("skillForge.rebaseConfirm"), t("skillForge.rebase"), { type: "warning" })
  const updated = await rebaseSkillForgeDraft(gameStore.token, activeDraft.value.id)
  const index = state.value?.drafts.findIndex((row) => row.id === updated.id) ?? -1
  if (state.value && index >= 0) state.value.drafts[index] = updated
  hydrateActiveDraft()
}

function openSourceDialog() {
	createSourceBookCount.value = sourceBookMinCount.value
	createSourceBookItemId.value = ""
	createDialogVisible.value = true
}

function selectSourceBook(item: ItemBrowserItem | null) {
  if (!item || sourceBookQuantity(item.itemId) < sourceBookMinCount.value) return
  createSourceBookItemId.value = item.itemId
  const affordable = Math.min(sourceBookMaxCount.value, sourceBookQuantity(item.itemId))
  const affordableSteps = Math.floor((affordable - sourceBookMinCount.value) / sourceBookCountStep.value)
  createSourceBookCount.value = sourceBookMinCount.value + affordableSteps * sourceBookCountStep.value
}

function sourceBookQuantity(itemId: string): number {
  return Number(state.value?.sourceSkillbooks.find((book) => book.itemId === itemId)?.quantity || 0)
}

async function deleteActiveDraft() {
  const draft = activeDraft.value
  if (!draft || !gameStore.token || deletingDraft.value || saving.value || draft.pendingRevisionId) return
  try {
    await ElMessageBox.confirm(t("skillForge.draft.deleteConfirm"), t("skillForge.draft.delete"), {
      type: "warning",
      confirmButtonText: t("game.confirm"),
      cancelButtonText: t("game.cancel")
    })
  } catch {
    return
  }
  deletingDraft.value = true
  if (saveTimer) {
    clearTimeout(saveTimer)
    saveTimer = null
  }
  try {
    await deleteSkillForgeDraft(gameStore.token, draft.id)
    activeDraftId.value = ""
    ElMessage.success(t("skillForge.draft.deleted"))
    await loadState()
  } catch (error) {
    ElMessage.error(resolveErrorMessage(t, error, "skillForge.draft.deleteFailed"))
  } finally {
    deletingDraft.value = false
  }
}

async function createDraft() {
  if (!gameStore.token) return
  if (activeDraft.value && !(await flushActiveDraft())) return
  creating.value = true
  try {
    const sourceBook = state.value?.sourceSkillbooks.find((book) => book.itemId === createSourceBookItemId.value)
		const existing = state.value?.drafts.find((draft) => draft.status === "active" && draft.sourceKind === createSourceKind.value && (
			createSourceKind.value === "blank" || draft.sourceSkillbookItemId === createSourceBookItemId.value
		))
		if (existing) {
			if (createSourceKind.value === "official" && existing.sourceSkillbookCount !== createSourceBookCount.value) {
				const updated = await updateSkillForgeDraft(gameStore.token, existing.id, existing.revisionNo, createSourceBookCount.value, existing.graph)
				const index = state.value?.drafts.findIndex((draft) => draft.id === existing.id) ?? -1
				if (state.value && index >= 0) state.value.drafts[index] = updated
			}
			createDialogVisible.value = false
			await loadState(existing.id)
			return
		}
    const sourceSkillId = createSourceKind.value === "official" ? sourceBook?.skillId || "" : ""
    const sourceDefinition = getSkillDefinition(sourceSkillId)
    const graph = createSourceKind.value === "official" && sourceDefinition
      ? createOfficialSkillForgeGraph(sourceSkillId, sourceDefinition as unknown as Record<string, unknown>)
      : createBlankSkillForgeGraph("blank")
    const draft = await createSkillForgeDraft(gameStore.token, createSourceKind.value, createSourceKind.value === "official" ? createSourceBookItemId.value : "", createSourceKind.value === "official" ? createSourceBookCount.value : 0, graph)
    createDialogVisible.value = false
    createSourceBookItemId.value = ""
    createSourceBookCount.value = sourceBookMinCount.value
    await loadState(draft.id)
  } catch (error) {
    ElMessage.error(resolveErrorMessage(t, error, "skillForge.createFailed"))
  } finally {
    creating.value = false
  }
}

async function archiveSkill(skillId: string) {
  if (!gameStore.token) return
  await ElMessageBox.confirm(t("skillForge.skills.archiveConfirm"), t("skillForge.skills.archive"), { type: "warning" })
  archivingSkillId.value = skillId
  try {
    await archiveCustomSkill(gameStore.token, skillId)
    ElMessage.success(t("skillForge.skills.archived"))
    await loadState()
  } finally {
    archivingSkillId.value = ""
  }
}

async function forge() {
  if (forging.value || !activeDraft.value || !gameStore.token) return
  if (!(await flushActiveDraft())) return
  const draft = activeDraft.value
  if (!draft) return
  const hasCurrentPreview = previewDraftId === draft.id && previewRevisionNo === draft.revisionNo && Boolean(preview.value?.graphHash)
  if (!hasCurrentPreview && !(await refreshPreview(draft.id, draft.revisionNo))) {
    ElMessage.error(t("skillForge.previewFailed"))
    return
  }
  if (!preview.value?.valid || !preview.value.graphHash) return
  if (hasRecipeShortage.value) {
    ElMessage.error(t("skillForge.recipeInsufficient"))
    return
  }
  const draftId = draft.id
  const expectedRevision = draft.revisionNo
  const expectedGraphHash = preview.value.graphHash
  forging.value = true
  try {
    forgeIdempotencyKey ||= `skill-forge-${draftId}-${Date.now()}-${Math.random().toString(36).slice(2)}`
    const revision = await forgeSkillDraft(gameStore.token, draftId, expectedRevision, expectedGraphHash, forgeIdempotencyKey)
    forgeIdempotencyKey = ""
    lastAttempt.value = revision.failed ? revision : null
    ElMessage.success(revision.failed ? t("skillForge.result.failed") : t("skillForge.result.success"))
    await loadState(draftId)
  } catch (error) {
    if (error instanceof APIClientError && error.code === "REQUEST_DUPLICATE") {
      // Compatibility with older servers that rejected a retry before the
      // durable forge idempotency record could return the original result.
      forgeIdempotencyKey = ""
      await loadState(draftId)
			ElMessage.info(t("skillForge.result.retryResolved"))
			return
    }
    ElMessage.error(resolveErrorMessage(t, error, "skillForge.forgeFailed"))
  } finally {
    forging.value = false
  }
}

async function resolvePending(accept: boolean) {
  if (resolvingPending.value || !pendingRevision.value || !gameStore.token || !activeDraft.value) return
  const token = gameStore.token
  const revisionId = pendingRevision.value.id
  const draftId = activeDraft.value.id
  if (!accept) {
    try {
      await ElMessageBox.confirm(t("skillForge.result.discardConfirm"), t("skillForge.result.discard"), { type: "warning" })
    } catch {
      return
    }
  }
  resolvingPending.value = true
  try {
    if (accept) {
      const accepted = await acceptSkillForgeRevision(token, revisionId)
      acceptedBook.value = accepted.forgedSkillbook || null
      const [inventoryRefresh] = await Promise.allSettled([inventoryStore.refreshAll(token), loadState(draftId)])
      if (inventoryRefresh.status === "rejected") {
        ElMessage.warning(t("skillForge.result.inventoryRefreshFailed"))
      }
    } else {
      await discardSkillForgeRevision(token, revisionId)
      await loadState(draftId)
    }
    ElMessage.success(accept ? t("skillForge.result.accepted") : t("skillForge.result.discarded"))
  } catch (error) {
    ElMessage.error(resolveErrorMessage(t, error, "skillForge.result.resolveFailed"))
  } finally {
    resolvingPending.value = false
  }
}

function fitGraph() {
  if (activeDraft.value?.sourceKind === "official" && canvasNodes.value.some((node) => node.data.kind === "parameter")) {
		const compact = window.innerWidth <= 768
		void setCenter(compact ? 300 : 940, compact ? 260 : 300, { zoom: compact ? 0.72 : 0.82, duration: 220 })
    return
  }
  void fitView({ padding: 0.2, duration: 220, minZoom: 0.62, maxZoom: 0.96 })
}
function normalizeEffectKind(kind: string): "empty" | "damage" | "heal" | "status" | "resource" {
  return kind === "damage" || kind === "heal" || kind === "status" || kind === "resource" ? kind : "empty"
}

function moduleParameterKinds(module: SkillForgeModule): SkillForgeParameterKind[] {
  const kinds: Partial<Record<string, SkillForgeParameterKind[]>> = {
    requirement_reduction: ["requirements"],
    formula_term: ["formula"],
    skill_level_term: ["formula"],
    flat_amount: ["formula"],
    target_count: ["target_count"],
    target_count_max: ["target_count_max"],
    target_direction: ["target_direction"],
    hit_count: ["hit_count"],
    target_scope: ["target_scope"],
    target_group: ["target_scope"],
    target_position: ["target_positions"],
    select_rule: ["select_rule"],
    status_apply: ["status_apply"],
    status_duration: ["status_duration"],
    effect_chain: ["effect_chain"],
    caster_status_condition: ["conditions"],
    target_status_condition: ["conditions"],
    consume_caster_status: ["conditions"],
    consume_target_status: ["conditions"],
    self_current_hp_loss: ["formula"],
    hp_cost: ["hp_cost"],
    mp_cost: ["mp_cost"],
    cooldown: ["cooldown"]
  }
  return kinds[module.compileOp.kind] || []
}

function moduleCanModifyParameter(module: SkillForgeModule, parameter: Pick<ForgeCanvasData, "parameterKind" | "effectKind">): boolean {
  if (!parameter.parameterKind || !moduleParameterKinds(module).includes(parameter.parameterKind)) return false
  const expected = String(module.compileOp.effectKind || "")
  const effectKind = String(parameter.effectKind || "")
  if (expected && effectKind && expected !== effectKind) return false
  if (module.compileOp.kind === "status_apply" || module.compileOp.kind === "status_duration") return !effectKind || effectKind === "status"
  if (module.compileOp.kind === "hit_count") return !effectKind || effectKind === "damage"
  if (module.compileOp.kind === "self_current_hp_loss") return !effectKind || effectKind === "resource"
  return true
}

function compatibleParameterNodes(module: SkillForgeModule, nodes: SkillForgeGraphNode[], preferredEffectIndex?: number): SkillForgeGraphNode[] {
  const candidates = nodes.filter((node) => node.kind === "parameter" && moduleCanModifyParameter(module, node))
  if (typeof preferredEffectIndex !== "number") return candidates
  const sameEffect = candidates.filter((node) => node.effectIndex === preferredEffectIndex)
  return sameEffect.length ? sameEffect : candidates
}

function nearestCompatibleParameter(module: SkillForgeModule, position: { x: number; y: number }): ForgeCanvasNode | undefined {
  const candidates = canvasNodes.value.filter((node) => node.data.kind === "parameter" && moduleCanModifyParameter(module, node.data))
  return candidates.sort((a, b) => {
    const distanceA = Math.hypot(a.position.x + 95 - position.x, a.position.y + 52 - position.y)
    const distanceB = Math.hypot(b.position.x + 95 - position.x, b.position.y + 52 - position.y)
    return distanceA - distanceB
  })[0]
}

function parameterAcceptsDraggedMaterial(raw: unknown): boolean {
  const data = raw as ForgeCanvasData
  const module = draggingModuleId.value ? moduleMap.value.get(draggingModuleId.value) : undefined
  return data.kind === "parameter" && !!module && moduleCanModifyParameter(module, data)
}

function parameterCompatibleModuleCount(raw: unknown): number {
  const data = raw as ForgeCanvasData
  if (data.kind !== "parameter") return 0
  return (state.value?.modules || []).filter((module) => moduleCanModifyParameter(module, data)).length
}
function parameterMaterialCount(parameterID: string): number {
	return canvasNodes.value.filter((node) => node.data.kind === "material" && node.data.targetNodeId === parameterID).length
}
function hasMaterialFollower(materialId: string): boolean {
  const node = canvasNodes.value.find((row) => row.id === materialId)
  if (!node?.data.targetNodeId || !materialCanChain(node.data)) return false
  const chain = parameterMaterialNodes(node.data.targetNodeId).filter((row) => materialCanChain(row.data))
  const index = chain.findIndex((row) => row.id === materialId)
  return index >= 0 && index < chain.length - 1
}
function effectNodes() {
	return canvasNodes.value.filter((node) => node.data.kind === "effect").sort((a, b) => Number(a.data.effectIndex || 0) - Number(b.data.effectIndex || 0))
}
function sourceBaseEffectCount(): number {
	const draft = activeDraft.value
	if (!draft || draft.sourceKind === "blank") return 1
	const definition = getSkillDefinition(draft.sourceSkillId || "")
	const count = Array.isArray(definition?.effects) ? definition.effects.length : 1
	return Math.max(1, Math.min(state.value?.limits.maxEffects || 3, count))
}
function effectKindForModule(module: SkillForgeModule): "damage" | "heal" | "status" | "resource" | "" {
	const explicit = String(module.compileOp.effectKind || "")
	if (explicit === "damage" || explicit === "heal" || explicit === "status" || explicit === "resource") return explicit
	if (module.compileOp.kind === "status_apply" || module.compileOp.kind === "status_duration") return "status"
	if (module.compileOp.kind === "hit_count") return "damage"
	if (module.compileOp.kind === "self_current_hp_loss") return "resource"
	return ""
}
function effectHasKindDefiningMaterial(effectIndex: number): boolean {
	return canvasNodes.value.some((node) => {
		if (node.data.kind !== "material" || Number(node.data.effectIndex || 0) !== effectIndex) return false
		const module = moduleForData(node.data)
		return Boolean(module && effectKindForModule(module))
	})
}
function canAssignEffectKind(effectIndex: number, kind: string): boolean {
	if (!kind) return true
	const node = effectNodes().find((row) => Number(row.data.effectIndex || 0) === effectIndex)
	const current = normalizeEffectKind(String(node?.data.effectKind || "empty"))
	return current === "empty" || current === kind
}
function assignEffectKind(effectIndex: number, kind: string) {
	if (!kind) return
	const node = effectNodes().find((row) => Number(row.data.effectIndex || 0) === effectIndex)
	if (node && canAssignEffectKind(effectIndex, kind)) {
		const normalized = normalizeEffectKind(kind)
		node.data.effectKind = normalized
		for (const parameter of canvasNodes.value.filter((row) => row.data.kind === "parameter" && Number(row.data.effectIndex || 0) === effectIndex)) {
			parameter.data.effectKind = normalized
		}
	}
}
function canChooseEffectKind(raw: unknown): boolean {
	const data = raw as ForgeCanvasData
	if (data.kind !== "effect") return false
	return activeDraft.value?.sourceKind === "blank" || Number(data.effectIndex || 0) >= sourceBaseEffectCount()
}
function updateEffectKind(nodeID: string, kind: string) {
	const effect = canvasNodes.value.find((node) => node.id === nodeID && node.data.kind === "effect")
	if (!effect || !canChooseEffectKind(effect.data)) return
	const normalized = normalizeEffectKind(kind)
	if (normalized === "empty" || normalized === effect.data.effectKind) return
	captureBeforeChange()
	effect.data.effectKind = normalized
	const effectIndex = Number(effect.data.effectIndex || 0)
	for (const parameter of canvasNodes.value.filter((node) => node.data.kind === "parameter" && Number(node.data.effectIndex || 0) === effectIndex)) {
		parameter.data.effectKind = normalized
	}
	markDirty()
}
function normalizeBlankEffectKinds() {
	if (activeDraft.value?.sourceKind !== "blank") return
	for (const effect of effectNodes()) {
		const index = Number(effect.data.effectIndex || 0)
		const definingKinds = canvasNodes.value
			.filter((node) => node.data.kind === "material" && Number(node.data.effectIndex || 0) === index)
			.map((node) => moduleForData(node.data))
			.filter((module): module is SkillForgeModule => !!module)
			.map(effectKindForModule)
			.filter(Boolean)
		const kind = definingKinds[0] || normalizeEffectKind(String(effect.data.effectKind || "empty"))
		effect.data.effectKind = kind
		for (const parameter of canvasNodes.value.filter((row) => row.data.kind === "parameter" && Number(row.data.effectIndex || 0) === index)) {
			parameter.data.effectKind = kind === "empty" ? undefined : kind
		}
	}
}
function nearestEffectIndex(y: number): number {
	const nodes = effectNodes()
	if (nodes.length === 0) return 0
	return Number(nodes.reduce((best, node) => Math.abs(node.position.y - y) < Math.abs(best.position.y - y) ? node : best).data.effectIndex || 0)
}

function effectBranchScaffold(effectIndex: number, effectKind: string): SkillForgeGraph {
	const kinds = Array.from({ length: effectIndex + 1 }, (_, index) => index === effectIndex ? normalizeEffectKind(effectKind) : "damage")
	return createBlankSkillForgeGraph("blank", "", kinds)
}

function ensureEffectHooks(effectIndex: number): boolean {
	const effect = effectNodes().find((node) => Number(node.data.effectIndex || 0) === effectIndex)
	if (!effect) return false
	let changed = false
	const kind = normalizeEffectKind(String(effect.data.effectKind || "empty"))
	const scaffold = effectBranchScaffold(effectIndex, kind)
	const scaffoldNodes = scaffold.nodes.filter((node) => Number(node.effectIndex ?? -1) === effectIndex)
	const scaffoldEffect = scaffoldNodes.find((node) => node.kind === "effect")
	if (!scaffoldEffect) return changed
	if (effect.id !== scaffoldEffect.id && !canvasNodes.value.some((node) => node.id === scaffoldEffect.id)) {
		const previousID = effect.id
		effect.id = scaffoldEffect.id
		for (const edge of canvasEdges.value) {
			if (edge.source === previousID) edge.source = effect.id
			if (edge.target === previousID) edge.target = effect.id
		}
		changed = true
	}
	const idMap = new Map<string, string>([[scaffoldEffect.id, effect.id]])
	if (effect.position.x !== scaffoldEffect.position.x || effect.position.y !== scaffoldEffect.position.y) {
		effect.position = { ...scaffoldEffect.position }
		changed = true
	}
	for (const parameter of scaffoldNodes.filter((node) => node.kind === "parameter")) {
		let current = canvasNodes.value.find((node) => (
			node.data.kind === "parameter"
			&& Number(node.data.effectIndex ?? -1) === effectIndex
			&& node.data.parameterKind === parameter.parameterKind
		))
		if (!current) {
			current = toCanvasNode({ ...parameter, ...(kind === "empty" ? {} : { effectKind: kind }) })
			canvasNodes.value.push(current)
			changed = true
		} else {
			current.data.effectKind = kind === "empty" ? undefined : kind
			if (current.position.x !== parameter.position.x || current.position.y !== parameter.position.y) {
				current.position = { ...parameter.position }
				changed = true
			}
		}
		idMap.set(parameter.id, current.id)
	}

	const branchIDs = new Set(canvasNodes.value
		.filter((node) => (node.data.kind === "effect" || node.data.kind === "parameter") && Number(node.data.effectIndex ?? -1) === effectIndex)
		.map((node) => node.id))
	const beforeEdges = edgeSetSignature(canvasEdges.value)
	canvasEdges.value = canvasEdges.value.filter((edge) => {
		if (!branchIDs.has(edge.source) && !branchIDs.has(edge.target)) return true
		const sourceKind = canvasNodes.value.find((node) => node.id === edge.source)?.data.kind
		const targetKind = canvasNodes.value.find((node) => node.id === edge.target)?.data.kind
		return sourceKind === "material" || targetKind === "material"
	})
	for (const edge of scaffold.edges) {
		const source = edge.source === "source" ? "source" : idMap.get(edge.source)
		const target = edge.target === "output" ? "output" : idMap.get(edge.target)
		if (!source || !target) continue
		canvasEdges.value.push({ ...edge, id: `effect-${effectIndex + 1}-${edge.id}`, source, target })
	}
	if (beforeEdges !== edgeSetSignature(canvasEdges.value)) changed = true
	const scaffoldOutput = scaffold.nodes.find((node) => node.kind === "output")
	const output = canvasNodes.value.find((node) => node.data.kind === "output")
	if (scaffoldOutput && output && output.position.x < scaffoldOutput.position.x) {
		output.position = { ...output.position, x: scaffoldOutput.position.x }
		changed = true
	}
	return changed
}

function ensureAdditionalEffectHooks(): boolean {
	let changed = false
	const minimumIndex = sourceBaseEffectCount()
	for (const effect of effectNodes()) {
		const effectIndex = Number(effect.data.effectIndex || 0)
		if (effectIndex < minimumIndex) continue
		// Older drafts stored the former default as damage even without a defining material.
		// Return that placeholder branch to the generic, unassigned state.
		if (normalizeEffectKind(String(effect.data.effectKind || "empty")) === "damage" && !effectHasKindDefiningMaterial(effectIndex)) {
			effect.data.effectKind = "empty"
			changed = true
		}
		changed = ensureEffectHooks(effectIndex) || changed
	}
	return changed
}

function addEffectSlotMaterial(module: SkillForgeModule, position: { x: number; y: number }): boolean {
	const maxEffects = state.value?.limits.maxEffects || 3
	const existing = canvasNodes.value.find((node) => node.data.compileOpKind === "effect_slot")
	if (effectNodes().length >= maxEffects) {
		ElMessage.warning(t("skillForge.effectLimit"))
		return false
	}
	let quantity = module.unitBasis
	if (existing) {
		quantity = Math.min(Number(existing.data.maxQuantity || 2), Number(existing.data.quantity || 0) + module.unitBasis)
		existing.data.quantity = quantity
	} else {
		const id = `material-${Date.now()}-${Math.random().toString(36).slice(2, 7)}`
		canvasNodes.value.push(toCanvasNode({ id, kind: "material", moduleId: module.id, quantity, position }))
		appendMaterialToSkillBranch(id)
	}
	ensureEffectNodeCount(Math.min(maxEffects, sourceBaseEffectCount() + Math.floor(quantity / module.unitBasis)))
	return true
}
function ensureEffectNodeCount(desired: number) {
	const current = effectNodes()
	for (let index = current.length; index < desired; index += 1) {
		const id = `effect-${index + 1}`
		canvasNodes.value.push(toCanvasNode({ id, kind: "effect", effectKind: "empty", effectIndex: index, position: { x: 604, y: 220 + index * 480 } }))
		ensureEffectHooks(index)
	}
}
function trimEffectNodeCount(desired: number) {
	const minimum = sourceBaseEffectCount()
	const keepCount = Math.max(minimum, desired)
	const removedIDs = new Set(
		canvasNodes.value
			.filter((node) => {
				const data = node.data as ForgeCanvasData
				return data.kind !== "source" && data.kind !== "output" && Number(data.effectIndex ?? -1) >= keepCount
			})
			.map((node) => node.id)
	)
	for (const node of canvasNodes.value) {
		if (node.data.kind === "material" && node.data.targetNodeId && removedIDs.has(node.data.targetNodeId)) removedIDs.add(node.id)
	}
	if (removedIDs.size === 0) return
	canvasNodes.value = canvasNodes.value.filter((node) => !removedIDs.has(node.id))
	canvasEdges.value = canvasEdges.value.filter((edge) => !removedIDs.has(edge.source) && !removedIDs.has(edge.target))
}
function appendMaterialToEffectBranch(materialId: string, effectIndex: number) {
	const terminal = canvasEdges.value.find((edge) => {
		if (edge.target !== "output" || edge.targetHandle !== "effect") return false
		const source = canvasNodes.value.find((node) => node.id === edge.source)
		return Number(source?.data.effectIndex || 0) === effectIndex
	})
	const effect = effectNodes().find((node) => Number(node.data.effectIndex || 0) === effectIndex)
	const sourceId = terminal?.source || effect?.id
	if (!sourceId) return
	if (terminal) canvasEdges.value = canvasEdges.value.filter((edge) => edge.id !== terminal.id)
	canvasEdges.value.push(
		{ id: `edge-${sourceId}-${materialId}`, source: sourceId, target: materialId, sourceHandle: "out", targetHandle: "in" },
		{ id: `edge-${materialId}-output`, source: materialId, target: "output", sourceHandle: "out", targetHandle: "effect" }
	)
}
function appendMaterialToSkillBranch(materialId: string) {
	const terminal = canvasEdges.value.find((edge) => edge.target === "output" && edge.targetHandle === "skill")
	const sourceId = terminal?.source || "source"
	if (terminal) canvasEdges.value = canvasEdges.value.filter((edge) => edge.id !== terminal.id)
	canvasEdges.value.push(
		{ id: `edge-${sourceId}-${materialId}`, source: sourceId, target: materialId, sourceHandle: terminal ? "out" : "skill", targetHandle: "in" },
		{ id: `edge-${materialId}-output`, source: materialId, target: "output", sourceHandle: "out", targetHandle: "skill" }
	)
}
function effectTagType(kind: string): "info" | "danger" | "success" | "warning" {
	if (kind === "damage" || kind === "resource") return "danger"
	if (kind === "heal") return "success"
	if (kind === "status") return "warning"
	return "info"
}
function inventoryCount(itemId: string) { return inventoryMap.value.get(itemId) || 0 }
function itemName(itemId: string) { return t(`item.${itemId}.name`, itemId) }
function itemRarityColor(itemId: string) {
	const rarity = getItemDefinition(itemId)?.rarity || "common"
	return getRarityDefinition(rarity)?.color || "var(--app-border)"
}
function recipeUsage(itemId: string): string[] {
	const usages = canvasNodes.value
		.filter((node) => node.data.kind === "material" && node.data.moduleItemId === itemId)
		.map((node) => {
			const target = node.data.targetNodeId ? canvasNodes.value.find((candidate) => candidate.id === node.data.targetNodeId) : undefined
			const targetLabel = target ? nodeTitle(target.data) : t("skillForge.inspector.skillStructure")
			return t("skillForge.preview.materialUsage", { target: targetLabel, count: Number(node.data.quantity || 0) })
		})
	const sealItemID = String(state.value?.limits.customizedNodeSealItemId || "spell_seal_lock_plate")
	if (itemId === sealItemID) {
		const modifications = canvasNodes.value.filter((node) => node.data.kind === "material").length
		if (modifications > 0) usages.push(t("skillForge.preview.sealUsage", {
			modifications,
			perModification: Number(state.value?.limits.customizedNodeSealCount || 10)
		}))
	}
	if (itemId === String(state.value?.limits.forgeCatalystItemId || "final_volume_key_sigil") && activeDraft.value?.sourceKind === "official") {
		usages.push(t("skillForge.preview.catalystUsage", { count: Number(state.value?.limits.forgeCatalystCount || 5) }))
	}
	if (itemId === String(state.value?.limits.goldCostItemId || "gold_coin") && preview.value) {
		const cost = preview.value.goldCostBreakdown || { base: 0, materials: 0, budget: 0 }
		usages.push(t("skillForge.preview.goldUsage", {
			base: Number(cost.base || 0).toLocaleString(),
			materials: Number(cost.materials || 0).toLocaleString(),
			budget: Number(cost.budget || 0).toLocaleString()
		}))
	}
	if (itemId === activeDraft.value?.sourceSkillbookItemId) {
		usages.push(t("skillForge.preview.sourceBookUsage", { count: activeDraft.value.sourceSkillbookCount }))
	}
	return usages.length ? usages : [t("skillForge.preview.materialUsageFallback")]
}
function skillName(skillId: string) { return getLocalizedSkillName(t, skillId, skillId) }
function opLabel(kind: string) { return t(`skillForge.op.${kind}`, kind) }
function nodeTitle(data: unknown) {
  const row = data as ForgeCanvasData
  if (row.kind === "source") return t("skillForge.node.source")
  if (row.kind === "output") return t("skillForge.node.output")
  if (row.kind === "effect") return `${t("skillForge.node.effect")} ${Number(row.effectIndex || 0) + 1} · ${t(`skillForge.effect.${String(row.effectKind || "empty")}`)}`
  if (row.kind === "parameter") return t(`skillForge.parameter.${String(row.parameterKind || "unknown")}`)
  return itemName(String(row.moduleItemId || ""))
}

function activeSourceDefinition(): Record<string, unknown> {
  const raw = activeDraft.value?.sourceSkillId ? getSkillDefinition(activeDraft.value.sourceSkillId) : undefined
  if (raw && typeof raw === "object") return raw as unknown as Record<string, unknown>
  if (activeDraft.value?.sourceKind !== "blank") return {}
  return {
    hpCost: 0,
    mpCost: 0,
    cooldown: Math.max(1, Number(state.value?.limits.minCooldown || 1)),
    weaponRequirement: [],
    statusRequirement: {},
    effects: effectNodes().map((node) => {
      const kind = String(node.data.effectKind || "empty")
      return {
        kind,
        targeting: {
          targetScope: kind === "heal" || kind === "resource" ? "ally" : "enemy",
          targetPositions: ["vanguard"],
          secondarySeek: "front_to_rear",
          selectRule: "random",
          targetCount: { base: 1, max: 1, formula: [] }
        },
        amountFormula: { baseAdd: 0, terms: [] },
        damageSpec: { hitCount: { base: 1, formula: [] } },
        statusPayload: { statusIds: [], applyChanceBase: 0, durationBaseTurns: 1 }
      }
    })
  }
}

function sourceEffectDefinition(effectIndex: number): Record<string, unknown> {
  const effects = activeSourceDefinition().effects
  const raw = Array.isArray(effects) ? effects[effectIndex] : undefined
  return raw && typeof raw === "object" && !Array.isArray(raw) ? raw as Record<string, unknown> : {}
}

function recordValue(value: unknown): Record<string, unknown> {
  return value && typeof value === "object" && !Array.isArray(value) ? value as Record<string, unknown> : {}
}

function arrayStrings(value: unknown): string[] {
  return Array.isArray(value) ? value.map((entry) => String(entry || "")).filter(Boolean) : []
}

function formatFormulaNode(value: unknown): string {
  const formula = recordValue(value)
  const parts: string[] = []
  const base = Number(formula.baseAdd || 0)
  if (base) parts.push(formatNumber(base))
  for (const raw of Array.isArray(formula.terms) ? formula.terms : []) {
    const term = recordValue(raw)
    const attrKey = String(term.attrKey || "")
    if (!attrKey) continue
    const attrName = attrKey === "skill_level" ? t("skillForge.inspector.skillLevel") : attrKey === "current_hp" ? t("skillForge.inspector.currentHp") : localizedName(`attr.${attrKey}.name`, attrKey)
    const perLevel = Number(term.perSkillLevel || 0)
    parts.push(`${formatNumber(Number(term.coefficient || 0))}${perLevel ? ` + ${formatNumber(perLevel)}/Lv` : ""} × ${attrName}`)
  }
  return parts.length ? parts.join(" + ") : "0"
}

function effectConditionsLabel(effect: Record<string, unknown>): string {
  const groups: Array<[string, unknown]> = [
    [t("skillForge.parameter.requiredTarget"), effect.requiredTargetStatuses],
    [t("skillForge.parameter.forbiddenTarget"), effect.forbiddenTargetStatuses],
    [t("skillForge.parameter.requiredCaster"), effect.requiredCasterStatuses],
    [t("skillForge.parameter.forbiddenCaster"), effect.forbiddenCasterStatuses],
    [t("skillForge.parameter.consumeTarget"), effect.consumeStatusesOnHit],
    [t("skillForge.parameter.consumeCaster"), effect.consumeCasterStatusesOnHit]
  ]
  const labels = groups.flatMap(([label, raw]) => {
    const values = arrayStrings(raw).map((id) => localizedName(`buff.${id}.name`, id))
    return values.length ? [`${label}: ${values.join("、")}`] : []
  })
  return labels.length ? labels.join(" · ") : t("skillForge.parameter.none")
}

function parameterValueLabel(raw: unknown): string {
  const data = raw as ForgeCanvasData
  const definition = activeSourceDefinition()
  if (data.parameterKind === "requirements") {
    const weapons = arrayStrings(definition.weaponRequirement)
    const statuses = Object.keys(recordValue(definition.statusRequirement))
    const parts = [weapons.length ? `${t("skillForge.parameter.weapon")}: ${weapons.join("、")}` : "", statuses.length ? `${t("skillForge.parameter.status")}: ${statuses.join("、")}` : ""].filter(Boolean)
    return parts.length ? parts.join(" · ") : t("skillForge.parameter.none")
  }
  if (data.parameterKind === "hp_cost") return String(Number(definition.hpCost || 0))
  if (data.parameterKind === "mp_cost") return String(Number(definition.mpCost || 0))
  if (data.parameterKind === "cooldown") return `${Number(definition.cooldown || 0)} ${t("skillForge.inspector.turns")}`
  const effect = sourceEffectDefinition(Number(data.effectIndex || 0))
  const targeting = recordValue(effect.targeting)
  if (data.parameterKind === "conditions") return effectConditionsLabel(effect)
  if (data.parameterKind === "target_scope") return targetGroupLabel(String(targeting.targetScope || "enemy"), targeting.excludeSelf === true)
  if (data.parameterKind === "target_count") {
    const count = recordValue(targeting.targetCount)
    return formatFormulaNode({ baseAdd: count.base || 1, terms: count.formula || [] })
  }
  if (data.parameterKind === "target_count_max") {
    const count = recordValue(targeting.targetCount)
    return String(Number(count.max || count.base || 1))
  }
  if (data.parameterKind === "target_direction") {
    const seek = String(targeting.secondarySeek || "front_to_rear")
    return t(`skillForge.secondarySeek.${seek}`, seek)
  }
  if (data.parameterKind === "select_rule") {
    const rule = String(targeting.selectRule || "random")
    const attrKey = String(targeting.selectAttrKey || "")
    const attrLabel = attrKey ? ` · ${localizedName(`attr.${attrKey}.name`, attrKey)}` : ""
    return `${t(`skillForge.selectRule.${rule}`, rule)}${attrLabel}${targeting.fallbackSelfIfEmpty === true ? ` · ${t("skillForge.parameter.fallbackSelf")}` : ""}`
  }
  if (data.parameterKind === "target_positions") return arrayStrings(targeting.targetPositions).map((position) => t(`skillForge.position.${position}`, position)).join("、") || t("skillForge.parameter.none")
  if (data.parameterKind === "formula") return formatFormulaNode(effect.amountFormula)
  if (data.parameterKind === "hit_count") {
    const hit = recordValue(recordValue(effect.damageSpec).hitCount || effect.hitCount)
    return formatFormulaNode({ baseAdd: hit.base || 1, terms: hit.formula || [] })
  }
  if (data.parameterKind === "status_apply") {
    const payload = recordValue(effect.statusPayload)
    const statuses = arrayStrings(payload.statusIds).map((id) => localizedName(`buff.${id}.name`, id)).join("、") || t("skillForge.parameter.none")
    return `${statuses} · ${formatPercent(Number(payload.applyChanceBase ?? 1))}${Number(payload.applyChancePerSkillLevel || 0) ? ` + ${formatPercent(Number(payload.applyChancePerSkillLevel))}/Lv` : ""}`
  }
  if (data.parameterKind === "status_duration") {
    const payload = recordValue(effect.statusPayload)
    return `${Number(payload.durationBaseTurns || 0)} ${t("skillForge.inspector.turns")}${Number(payload.durationPerSkillLevelTurns || 0) ? ` + ${formatNumber(Number(payload.durationPerSkillLevelTurns))}/Lv` : ""}`
  }
  if (data.parameterKind === "effect_chain") {
    const source = Number(effect.reuseTargetsFromEffect || 0)
    return source > 0 ? t("skillForge.parameter.reuseEffect", { value: source }) : t("skillForge.parameter.independentTarget")
  }
  return t("skillForge.parameter.none")
}

function effectHeaderValue(raw: unknown): string {
  const data = raw as ForgeCanvasData
  const effect = sourceEffectDefinition(Number(data.effectIndex || 0))
  const kind = String(effect.kind || data.effectKind || "")
  if (kind === "damage") return `${t("skillForge.parameter.damageType")}: ${localizedName(`game.damageType.${String(effect.damageType || "physical")}`, String(effect.damageType || "physical"))}`
  if (kind === "resource") return `${t("skillForge.parameter.resource")}: ${String(effect.resource || "hp").toUpperCase()}`
  return t("skillForge.parameter.dslEffectRoot")
}

function materialAttachedParameterLabel(raw: unknown): string {
  const data = raw as ForgeCanvasData
  const target = data.targetNodeId ? canvasNodes.value.find((node) => node.id === data.targetNodeId) : undefined
  return target ? `${t("skillForge.parameter.modifies")}: ${nodeTitle(target.data)}` : t("skillForge.effectIndex", { value: Number(data.effectIndex || 0) + 1 })
}
function sourceLabel(draft: SkillForgeDraft) {
  if (draft.sourceKind === "blank") return t("skillForge.draft.blank")
  return skillName(draft.sourceSkillId || "")
}

function validSourceBookCount(count: number) {
	return count >= sourceBookMinCount.value && count <= sourceBookMaxCount.value && (count - sourceBookMinCount.value) % sourceBookCountStep.value === 0
}
function normalizeSourceBookCount(value: number | undefined) {
	const raw = Number(value || sourceBookMinCount.value)
	const steps = Math.round((raw - sourceBookMinCount.value) / sourceBookCountStep.value)
	return Math.max(sourceBookMinCount.value, Math.min(sourceBookMaxCount.value, sourceBookMinCount.value + steps * sourceBookCountStep.value))
}
function skillbookBonusForCount(count: number) {
	if (!validSourceBookCount(count)) return 0
	const steps = (count - sourceBookMinCount.value) / sourceBookCountStep.value
	return Math.min(sourceBookMaxBonus.value, steps * sourceBookBonusPerStep.value)
}
function updateSourceBookCount(value: number | undefined) {
  if (!activeDraft.value || activeDraft.value.sourceKind !== "official") return
  activeDraft.value.sourceSkillbookCount = normalizeSourceBookCount(value)
  markDirty()
}
function draftLabel(draft: SkillForgeDraft) { return sourceLabel(draft) }
function formatPercent(value: number) { return `${(Math.max(0, Math.min(1, value)) * 100).toFixed(1)}%` }
function formatSignedPercent(value: number) {
	const normalized = Number(value || 0)
	return `${normalized > 0 ? "+" : ""}${(normalized * 100).toFixed(1)}%`
}
function successModifierClass(value: number) {
	return Number(value || 0) > 0 ? "is-positive" : Number(value || 0) < 0 ? "is-negative" : ""
}
function formatNumber(value: number) { return Number(value || 0).toFixed(Math.abs(value) < 10 ? 2 : 1) }
function formatRangeValue(kind: string, value: number): string {
  if (kind === "status_apply") return formatPercent(value)
		if (kind === "self_current_hp_loss") return formatPercent(Math.abs(value))
	if (kind === "requirement_reduction") return formatPercent(value)
	if (["status_duration", "target_count", "target_count_max", "hit_count", "hp_cost", "mp_cost", "cooldown"].includes(kind)) {
		const rounded = Math.round(value)
		return kind === "status_duration" || kind === "cooldown" ? `${rounded} ${t("skillForge.inspector.turns")}` : String(rounded)
	}
  return formatNumber(value)
}
function formatRangeSpan(kind: string, min: number, max: number): string {
  if (Math.abs(min) < 0.000001 && Math.abs(max) < 0.000001) return t("skillForge.preview.fixedEffect")
  const minLabel = formatRangeValue(kind, min)
  const maxLabel = formatRangeValue(kind, max)
  return minLabel === maxLabel ? minLabel : `${minLabel}–${maxLabel}`
}
function moduleBaseRangeLabel(module: SkillForgeModule, quantity: number): string {
  if (module.compileOp.kind === "status_apply") {
    const steps = Math.max(1, Math.floor(quantity / Math.max(1, module.unitBasis)))
    const base = Number(state.value?.limits.statusBaseApplyChance ?? 0.25)
    const perStep = Number(state.value?.limits.statusApplyChancePerStep ?? 0.1)
    return formatPercent(Math.min(1, base + Math.max(0, steps - 1) * perStep))
  }
  return formatRangeSpan(module.compileOp.kind, module.valuePerUnit.min * quantity, module.valuePerUnit.max * quantity)
}
function moduleTargetLabel(module: SkillForgeModule): string {
  const op = module.compileOp
  const kind = String(op.kind || "")
  if (kind === "status_apply") {
    const buffId = String(op.buffId || "")
    return `${t("skillForge.effect.status")} · ${localizedName(`buff.${buffId}.name`, buffId)} · ${t("skillForge.inspector.applyChance")}`
  }
  if (kind === "status_duration") return `${t("skillForge.effect.status")} · ${t("skillForge.inspector.duration")}`
  if (kind === "target_group") return targetGroupLabel(String(op.scope || "enemy"), op.excludeSelf === true)
  if (kind === "target_scope") return targetGroupLabel(String(op.scope || "enemy"), op.excludeSelf === true)
  if (kind === "target_count") return t("skillForge.inspector.randomCount")
	if (kind === "target_count_max") return t("skillForge.inspector.maximumTargetCount")
	if (kind === "target_direction") return `${t("skillForge.parameter.target_direction")} · ${t(`skillForge.secondarySeek.${String(op.seek || "front_to_rear")}`, String(op.seek || "front_to_rear"))}`
	if (kind === "target_position") return `${t("skillForge.parameter.target_positions")} · ${arrayStrings(op.positions).map((position) => t(`skillForge.position.${position}`, position)).join("、")}`
	if (kind === "select_rule") {
		const rule = String(op.rule || "random")
		const attrKey = String(op.attrKey || "")
		const attrLabel = attrKey ? ` · ${localizedName(`attr.${attrKey}.name`, attrKey)}` : ""
		return `${t("skillForge.parameter.select_rule")} · ${t(`skillForge.selectRule.${rule}`, rule)}${attrLabel}`
	}
	if (kind === "requirement_reduction") return t("skillForge.inspector.requirementReduction")
		if (kind === "effect_slot") return t("skillForge.inspector.effectSlot")
	if (kind === "skill_level_term") return `${t("skillForge.effect.heal")} · ${t("skillForge.inspector.skillLevel")}`
	if (kind === "self_current_hp_loss") return `${t("skillForge.effect.resource")} · ${t("skillForge.inspector.currentHp")}`
  if (kind === "target_status_condition" || kind === "caster_status_condition") {
    const buffId = String(op.buffId || "")
    const subject = kind === "target_status_condition" ? t("skillForge.condition.target") : t("skillForge.condition.caster")
    return `${subject} · ${localizedName(`buff.${buffId}.name`, buffId)} · ${t("skillForge.condition.ifElse")}`
  }
  if (kind === "consume_target_status" || kind === "consume_caster_status") {
    const buffId = String(op.buffId || "")
    const subject = kind === "consume_target_status" ? t("skillForge.parameter.consumeTarget") : t("skillForge.parameter.consumeCaster")
    return `${subject} · ${localizedName(`buff.${buffId}.name`, buffId)}`
  }
  if (kind === "formula_term") {
    const attrKey = String(op.attrKey || "")
    return `${t(`skillForge.effect.${String(op.effectKind || "damage")}`)} · ${localizedName(`attr.${attrKey}.name`, attrKey)}`
  }
	  return operationLabel(kind, String(op.direction || ""))
}
function operationLabel(kind: string, direction?: string): string {
	const base = opLabel(kind)
	if (!["hp_cost", "mp_cost", "cooldown"].includes(kind)) return base
	if (direction === "increase") return `${base} · ${t("skillForge.direction.increase")}`
	if (direction === "reduce") return `${base} · ${t("skillForge.direction.reduce")}`
	return base
}
function materialOperationLabel(data: ForgeCanvasData): string {
	return operationLabel(String(data.compileOpKind || ""), data.compileOpDirection)
}
function materialNodeContribution(data: ForgeCanvasData): string {
  const module = moduleForData(data)
  if (!module) return ""
  return `${moduleTargetLabel(module)} · ${moduleBaseRangeLabel(module, Number(data.quantity || 1))}`
}
function materialNodeSuccess(data: ForgeCanvasData): string {
  return formatSignedPercent(materialNodeSuccessModifier(data))
}
function materialNodeSuccessModifier(data: ForgeCanvasData): number {
	const module = moduleForData(data)
	if (!module) return 0
	const basisCount = Number(data.quantity || module.unitBasis) / Math.max(1, module.unitBasis)
	return module.successChanceModifier * basisCount
}
function isBranchCondition(data: ForgeCanvasData): boolean {
  return data.compileOpKind === "target_status_condition" || data.compileOpKind === "caster_status_condition"
}
function isGlobalModule(data: ForgeCanvasData): boolean {
	return data.compileOpKind === "effect_slot" || moduleForData(data)?.typedPorts.inputs[0]?.type === "skill"
}
function materialPortStyle(data: ForgeCanvasData, portId: string, index: number, total: number, input: boolean): Record<string, string> {
  if (input || !isBranchCondition(data)) return total > 1 ? { top: `${((index + 1) / (total + 1)) * 100}%` } : {}
  return { top: portId === "true" ? "38%" : "72%" }
}
function rangeCanvasData(nodeId: string): ForgeCanvasData | undefined {
  return canvasNodes.value.find((node) => node.id === nodeId)?.data as ForgeCanvasData | undefined
}
function rangeMaterialName(nodeId: string): string {
  const data = rangeCanvasData(nodeId)
  return data?.moduleItemId ? itemName(data.moduleItemId) : nodeId
}
function rangeTargetLabel(nodeId: string, targetNodeId?: string): string {
  if (targetNodeId) {
    const target = canvasNodes.value.find((node) => node.id === targetNodeId)
    if (target) return nodeTitle(target.data)
  }
  const data = rangeCanvasData(nodeId)
	if (data && isGlobalModule(data)) return t("skillForge.inspector.skillStructure")
  return t("skillForge.effectIndex", { value: Number(data?.effectIndex || 0) + 1 })
}
function friendlyReason(reason: string) {
  if (reason.includes("requires at least one material")) return t("skillForge.reason.materialRequired")
  if (reason.includes("requires at least one effect")) return t("skillForge.reason.effectRequired")
  if (reason.includes("disconnected node")) return t("skillForge.reason.disconnected")
  if (reason.includes("port type mismatch")) return t("skillForge.reason.portMismatch")
  if (reason.includes("budget")) return t("skillForge.reason.budget")
  if (reason.includes("alchemy level")) return t("skillForge.reason.level")
  if (reason.includes("cycle")) return t("skillForge.reason.cycle")
  if (reason.includes("source definition changed")) return t("skillForge.reason.sourceStale")
	if (reason.includes("effect-slot material")) return t("skillForge.reason.effectSlot")
	if (reason.includes("material cannot modify")) return t("skillForge.reason.effectKind")
  return reason
}

function targetGroupLabel(scope: string, excludeSelf: boolean): string {
  if (scope === "all") return t("skillForge.targetGroup.all")
  if (scope === "self") return t("skillForge.targetGroup.self")
  if (scope === "ally") return t(excludeSelf ? "skillForge.targetGroup.alliesWithoutSelf" : "skillForge.targetGroup.alliesWithSelf")
  return t("skillForge.targetGroup.enemies")
}

function localizedName(key: string, fallback: string): string {
  const value = t(key)
  return value === key ? fallback : value
}
</script>

<style scoped>
.skill-forge { display: flex; height: 100%; min-height: 0; flex-direction: column; overflow: hidden; }
.skill-forge__toolbar { display: flex; flex: 0 0 auto; align-items: center; justify-content: space-between; gap: 16px; margin-bottom: 16px; }
.skill-forge__draft-tools, .skill-forge__canvas-tools { display: flex; align-items: center; gap: 8px; }
.skill-forge__draft-tools { flex-wrap: wrap; }
.draft-select { width: min(360px, 36vw); }
.draft-count { color: var(--app-text-muted); font-size: 11px; white-space: nowrap; }
.save-state { min-width: 56px; color: var(--app-text-muted); font-size: 12px; text-align: right; }
.save-state.is-saving { color: var(--app-primary); }
.save-state.is-dirty { color: var(--el-color-warning); }
.save-state.is-error { color: var(--el-color-danger); }
.skill-forge__loading { flex: 1 1 auto; min-height: 0; overflow-y: auto; }
.skill-forge__workspace { display: grid; flex: 1 1 auto; grid-template-columns: minmax(250px, 280px) minmax(600px, 1fr) minmax(330px, 360px); grid-template-areas: "modules graph preview"; height: auto; min-height: 0; max-height: none; overflow: hidden; border: 1px solid var(--app-border); background: var(--app-surface); }
.workspace-switcher { display: none; }
.module-panel, .preview-panel { min-width: 0; padding: 16px; background: var(--app-surface); overflow: hidden; }
.module-panel { grid-area: modules; display: flex; flex-direction: column; min-height: 0; border-right: 1px solid var(--app-border); }
.preview-panel { grid-area: preview; border-left: 1px solid var(--app-border); overflow-y: auto; overscroll-behavior: contain; scrollbar-gutter: stable; }
.panel-heading { display: flex; align-items: center; justify-content: space-between; min-height: 36px; margin-bottom: 12px; }
.panel-heading strong { font-size: 15px; }
.panel-heading > span { color: var(--app-text-muted); font-size: 12px; }
.module-filter-hint { margin: -4px 0 12px; color: var(--app-primary); font-size: 12px; line-height: 1.5; }
.module-list { display: grid; flex: 1; align-content: start; grid-auto-rows: max-content; gap: 8px; min-height: 0; max-height: none; margin-top: 12px; padding-right: 6px; overflow-y: auto; overscroll-behavior: contain; scrollbar-gutter: stable; }
.module-row { width: 100%; display: grid; grid-template-columns: 36px minmax(0, 1fr) auto; align-items: center; gap: 10px; padding: 10px; border: 1px solid var(--app-border); border-left: 4px solid var(--module-rarity-color, var(--app-border)); border-radius: 6px; color: var(--app-text); background: var(--app-bg); text-align: left; cursor: grab; }
.module-row:hover { border-color: var(--app-primary); border-left-color: var(--module-rarity-color, var(--app-primary)); background: color-mix(in srgb, var(--app-primary) 7%, var(--app-bg)); }
.module-row.is-short { background: color-mix(in srgb, var(--game-danger) 5%, var(--app-bg)); }
.module-row__body { min-width: 0; display: grid; }
.module-row__name { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; font-size: 14px; font-weight: 600; }
.module-row__meta, .module-row__batch, .module-row__chance, .module-row__stock { color: var(--app-text-muted); font-size: 11px; }
.module-row__batch { margin-top: 2px; color: var(--app-text); line-height: 1.35; }
.module-row__chance { margin-top: 2px; font-weight: 600; }
.is-positive { color: var(--game-success) !important; }
.is-negative { color: var(--game-danger) !important; }
.module-row__stock { align-self: start; white-space: nowrap; }
.module-inspector { display: grid; gap: 7px; }
.module-inspector header { display: flex; align-items: center; gap: 8px; padding-bottom: 7px; border-bottom: 1px solid var(--app-border); }
.module-inspector > div { display: grid; grid-template-columns: 88px minmax(0, 1fr); gap: 8px; font-size: 12px; }
.module-inspector > div span { color: var(--app-text-muted); }
.module-inspector > div strong { overflow-wrap: anywhere; text-align: right; }
.graph-panel { grid-area: graph; min-width: 0; min-height: 0; position: relative; overflow: hidden; background: color-mix(in srgb, var(--app-bg) 94%, var(--game-gold)); }
.skill-forge-flow { width: 100%; height: 100%; min-height: 0; }
.forge-node { width: 210px; border: 1px solid var(--app-border); border-radius: 6px; color: var(--app-text); background: var(--app-surface); box-shadow: 0 5px 14px color-mix(in srgb, black 14%, transparent); overflow: visible; }
.forge-node.is-selected { border-color: var(--app-primary); box-shadow: 0 0 0 2px color-mix(in srgb, var(--app-primary) 24%, transparent); }
.forge-node--source { border-top: 3px solid var(--game-gold); }
.forge-node--effect { border-top: 3px solid var(--game-danger); }
.forge-node--parameter { width: 230px; border-top: 3px solid var(--game-gold); }
.forge-node--output { border-top: 3px solid var(--game-success); }
.forge-node--material { border-top: 3px solid var(--module-rarity-color, var(--app-primary)); }
.forge-node.is-compatible-drop { border-color: var(--game-success); box-shadow: 0 0 0 3px color-mix(in srgb, var(--game-success) 28%, transparent); }
.forge-node.is-incompatible-drop { opacity: .42; }
.forge-node__header { min-height: 42px; display: flex; align-items: center; gap: 8px; padding: 8px 10px; border-bottom: 1px solid var(--app-border); }
.forge-node__title { min-width: 0; flex: 1; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; font-size: 13px; font-weight: 700; }
.forge-node__remove { width: 24px; height: 24px; padding: 0; }
.forge-node__body { min-height: 62px; display: grid; align-content: center; gap: 7px; padding: 10px; font-size: 12px; }
.forge-node__body small { color: var(--app-text-muted); }
.forge-node__dsl-value { min-height: 30px; font-size: 12px; font-weight: 600; line-height: 1.45; overflow-wrap: anywhere; }
.forge-node__slot { width: max-content; max-width: 100%; padding: 2px 6px; border: 1px solid color-mix(in srgb, var(--app-primary) 36%, var(--app-border)); border-radius: 4px; color: var(--app-primary) !important; background: color-mix(in srgb, var(--app-primary) 7%, var(--app-surface)); }
.forge-node__customized-cost { color: var(--game-danger) !important; font-weight: 600; }
.effect-kind-select { width: 100%; }
.condition-branches { display: grid; grid-template-columns: 1fr 1fr; gap: 8px; padding-top: 2px; font-size: 11px; font-weight: 700; }
.condition-branches span:first-child { color: var(--game-success); text-align: right; }
.condition-branches span:last-child { color: var(--game-danger); text-align: right; }
:deep(.forge-port--true) { background: var(--game-success); }
:deep(.forge-port--false) { background: var(--game-danger); }
:deep(.forge-port--modifier) { width: 12px !important; height: 12px !important; background: var(--game-gold) !important; }
.forge-node__contribution { padding: 5px 6px; border-left: 2px solid var(--app-primary); background: color-mix(in srgb, var(--app-primary) 7%, var(--app-surface)); overflow-wrap: anywhere; }
.node-quantity { display: grid; grid-template-columns: auto 1fr; align-items: center; gap: 6px; }
.node-quantity :deep(.el-input-number) { width: 112px; }
.effect-index-input { width: 112px; }
.forge-node :deep(.vue-flow__handle) { width: 10px; height: 10px; border: 2px solid var(--app-surface); background: var(--app-primary); }
:deep(.vue-flow__edge-path) { stroke: #7890ad; stroke-width: 2; }
:deep(.vue-flow__edge:hover .vue-flow__edge-path) { stroke: var(--app-primary); stroke-width: 2.5; }
.preview-metrics { display: grid; grid-template-columns: 1fr 1fr; border: 1px solid var(--app-border); }
.preview-metrics div { display: grid; gap: 3px; padding: 9px; border-right: 1px solid var(--app-border); border-bottom: 1px solid var(--app-border); }
.preview-metrics div:nth-child(2n) { border-right: 0; }
.preview-metrics span { color: var(--app-text-muted); font-size: 11px; }
.preview-metrics strong { font-size: 13px; }
.preview-switcher { width: 100%; margin-top: 14px; }
.preview-switcher :deep(.el-segmented__group) { width: 100%; }
.preview-switcher :deep(.el-segmented__item) { flex: 1; }
.blocking-list { display: grid; gap: 6px; margin-top: 10px; padding: 9px; color: var(--game-danger); background: color-mix(in srgb, var(--game-danger) 8%, var(--app-surface)); border-left: 3px solid var(--game-danger); font-size: 12px; }
.blocking-list div { display: grid; grid-template-columns: 16px 1fr; gap: 6px; }
.preview-section { margin-top: 15px; }
.preview-section h4 { margin: 0 0 7px; font-size: 12px; }
.final-skill-preview { padding-top: 2px; }
.final-skill-preview :deep(.skill-quick-ref-content) { gap: 10px; }
.final-skill-preview :deep(.skill-quick-ref-header) { padding: 10px; border: 1px solid var(--app-border); background: var(--app-bg); }
.final-skill-preview :deep(.detail-surface) { padding: 10px; border: 1px solid var(--app-border); background: var(--app-bg); }
.final-skill-preview :deep(.detail-section) { grid-template-columns: 1fr; gap: 7px; }
.final-skill-preview :deep(.detail-section-title) { margin-bottom: 7px; font-size: 12px; font-weight: 700; }
.effect-list, .range-list, .recipe-list { display: grid; border-top: 1px solid var(--app-border); }
.custom-skill-list { display: grid; border-top: 1px solid var(--app-border); }
.custom-skill-list > div { min-width: 0; display: flex; align-items: center; justify-content: space-between; gap: 8px; padding: 6px 0; border-bottom: 1px solid var(--app-border); }
.effect-row, .range-list > div, .recipe-list > div { display: grid; grid-template-columns: minmax(0, 1fr) auto; gap: 8px; padding: 7px 2px; border-bottom: 1px solid var(--app-border); font-size: 12px; }
.effect-row { grid-template-columns: 72px minmax(0, 1fr); }
.effect-row span { color: var(--app-text-muted); overflow-wrap: anywhere; }
.range-value { display: grid; justify-items: end; gap: 2px; }
.range-value small { color: var(--app-text-muted); font-size: 10px; }
.range-label { display: grid; gap: 2px; min-width: 0; }
.range-label strong { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.range-label small { color: var(--app-text-muted); font-size: 10px; }
.recipe-list .is-short { color: var(--game-danger); }
.recipe-item { min-width: 0; display: flex; align-items: center; gap: 2px; }
.recipe-item :deep(.el-tag) { max-width: 100%; height: auto; min-height: 28px; white-space: normal; }
.recipe-item :deep(.item-quick-ref-name) { overflow-wrap: anywhere; }
.recipe-usage-help { flex: 0 0 24px; width: 24px; height: 24px; color: var(--app-text-muted); }
.recipe-usage { display: grid; gap: 7px; font-size: 12px; line-height: 1.45; }
.recipe-usage > strong { padding-bottom: 6px; border-bottom: 1px solid var(--app-border); }
.recipe-note { margin: 8px 0 0; color: var(--app-text-muted); font-size: 11px; line-height: 1.5; }
.forge-action { display: flex; margin-top: 18px; }
.forge-action .el-button { width: 100%; }
.pending-result { margin-top: 12px; padding: 10px; border: 1px solid var(--game-success); background: color-mix(in srgb, var(--game-success) 8%, var(--app-surface)); }
.pending-result.is-failed { border-color: var(--game-danger); background: color-mix(in srgb, var(--game-danger) 8%, var(--app-surface)); }
.pending-result h4 { margin: 0 0 5px; font-size: 13px; }
.pending-result p { margin: 0; color: var(--app-text-muted); font-size: 12px; }
.pending-result__actions { display: flex; justify-content: flex-end; gap: 8px; margin-top: 10px; }
.source-book-stepper { width: 128px; }
.book-count-hint { display: block; margin-top: 6px; color: var(--app-text-muted); font-size: 12px; }
.source-book-picker { width: 100%; min-height: 210px; }
.source-book-tile {
  position: relative;
  width: fit-content;
  padding: 2px;
  border: 2px solid transparent;
  border-radius: 6px;
  cursor: pointer;
  outline: none;
}
.source-book-tile:hover,
.source-book-tile:focus-visible { border-color: var(--app-primary); }
.source-book-tile.is-selected {
  border-color: var(--app-primary);
  background: color-mix(in srgb, var(--app-primary) 7%, transparent);
  box-shadow: 0 0 0 2px color-mix(in srgb, var(--app-primary) 18%, transparent);
}
.source-book-tile.is-disabled { opacity: .5; cursor: not-allowed; }
.source-book-tile__check {
  position: absolute;
  top: 2px;
  right: 2px;
  width: 18px;
  padding: 2px;
  border-radius: 50%;
  color: white;
  background: var(--app-primary);
}
@media (min-width: 761px) {
  .forge-action { padding-right: 58px; }
}
@media (max-width: 1280px) {
  .skill-forge__workspace { grid-template-columns: minmax(270px, 300px) minmax(600px, 1fr); grid-template-rows: minmax(0, 3fr) minmax(220px, 2fr); grid-template-areas: "modules graph" "preview preview"; }
  .module-panel { height: auto; min-height: 0; }
  .preview-panel { border-top: 1px solid var(--app-border); border-left: 0; }
  .preview-panel .final-skill-preview :deep(.skill-quick-ref-content) { grid-template-columns: minmax(0, 1fr) minmax(0, 1fr); }
}
@media (max-width: 900px) {
  .skill-forge__toolbar { align-items: stretch; flex-direction: column; }
  .skill-forge__draft-tools, .skill-forge__canvas-tools { width: 100%; }
  .skill-forge__canvas-tools { flex-wrap: wrap; }
  .skill-forge__canvas-tools :deep(.el-button + .el-button) { margin-left: 0; }
  .skill-forge__canvas-tools .save-state { margin-left: auto; }
  .draft-select { flex: 1; width: auto; }
  .skill-forge__workspace { display: flex; min-height: 0; flex-direction: column; }
  .workspace-switcher { display: block; flex: 0 0 auto; z-index: 8; padding: 10px; border-bottom: 1px solid var(--app-border); background: var(--app-surface); }
  .workspace-switcher :deep(.el-segmented),
  .workspace-switcher :deep(.el-segmented__group) { width: 100%; }
  .workspace-switcher :deep(.el-segmented__item) { flex: 1; }
  .is-mobile-hidden { display: none !important; }
  .module-panel, .graph-panel, .preview-panel { flex: 1 1 auto; min-height: 0; height: auto; border: 0; }
  .module-list { grid-template-columns: repeat(2, minmax(0, 1fr)); }
  .graph-panel, .skill-forge-flow { min-height: 0; height: 100%; }
  .preview-panel { padding-bottom: 84px; }
  .preview-panel .final-skill-preview :deep(.skill-quick-ref-content) { grid-template-columns: 1fr; }
}
@media (max-width: 560px) {
  .skill-forge__toolbar { gap: 10px; margin-bottom: 10px; }
  .skill-forge__draft-tools { display: grid; grid-template-columns: minmax(0, 1fr) auto; }
  .skill-forge__draft-tools .draft-select { grid-column: 1 / -1; width: 100%; }
  .skill-forge__draft-tools .draft-count { align-self: center; }
  .skill-forge__canvas-tools { gap: 6px; }
  .module-panel, .preview-panel { padding: 12px; min-height: 0; }
  .module-list { grid-template-columns: 1fr; }
  .graph-panel, .skill-forge-flow { min-height: 0; height: 100%; }
  .preview-metrics { grid-template-columns: 1fr 1fr; }
  .preview-metrics div { min-width: 0; }
  .preview-metrics strong { overflow-wrap: anywhere; }
  .pending-result__actions { display: grid; grid-template-columns: 1fr 1fr; }
  .pending-result__actions .el-button { margin-left: 0; }
}
</style>
