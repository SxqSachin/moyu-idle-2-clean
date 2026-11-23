<template>
  <div class="alchemy-inherit-panel">
    <section class="inherit-workbench" data-tutorial="alchemy-inherit-setup">
      <div class="inherit-setup-grid">
        <div class="inherit-picker inherit-picker--source" data-tutorial="alchemy-inherit-parents">
          <div class="inherit-picker__title"><span class="inherit-step">1</span>{{ t("alchemy.inherit.leftSource") }}</div>
          <button v-if="selectedFather" type="button" class="inherit-instance inherit-instance--with-affix" @click="fatherSelectorVisible = true">
            <Card :item-id="selectedFather.templateItemId" :instance-id="selectedFather.instanceId" :quantity="selectedFather.quantity" />
          </button>
          <EmptyCard v-else :label="t('alchemy.inherit.pickLeftSource')" @click="fatherSelectorVisible = true" />
          <div v-if="fatherAffixOptions.length" class="inherit-affix-choice-list">
            <button
              v-for="option in fatherAffixOptions"
              :key="option.index"
              type="button"
              class="inherit-affix-choice"
              :class="{ 'is-active': option.index === fatherAffixIndex }"
              @click.stop="selectFatherAffix(option.index)"
            >
              <AffixQuickRef :affix-id="option.affix.affixId" :slot="option.affix" :show-instance-details="false" />
            </button>
          </div>
        </div>

        <div class="inherit-picker inherit-picker--source" data-tutorial="alchemy-inherit-parents">
          <div class="inherit-picker__title"><span class="inherit-step">2</span>{{ t("alchemy.inherit.rightSource") }}</div>
          <button v-if="selectedMother" type="button" class="inherit-instance" @click="motherSelectorVisible = true">
            <Card :item-id="selectedMother.templateItemId" :instance-id="selectedMother.instanceId" :quantity="selectedMother.quantity" />
          </button>
          <EmptyCard v-else :label="t('alchemy.inherit.pickRightSource')" @click="motherSelectorVisible = true" />
          <div v-if="motherAffixOptions.length" class="inherit-affix-choice-list">
            <button
              v-for="option in motherAffixOptions"
              :key="option.index"
              type="button"
              class="inherit-affix-choice"
              :class="{ 'is-active': option.index === motherAffixIndex }"
              @click.stop="selectMotherAffix(option.index)"
            >
              <AffixQuickRef :affix-id="option.affix.affixId" :slot="option.affix" :show-instance-details="false" />
            </button>
          </div>
        </div>

        <div class="inherit-picker inherit-picker--medium" data-tutorial="alchemy-inherit-medium">
          <div class="inherit-picker__title"><span class="inherit-step">3</span>{{ t("alchemy.inherit.medium") }}</div>
          <div class="inherit-medium-slot">
            <Card
              v-if="selectedMedium"
              :item-id="selectedMedium.itemId"
              :quantity="selectedMediumOwnedQty"
              @click="mediumSelectorVisible = true"
            />
            <EmptyCard v-else :label="t('alchemy.inherit.pickMedium')" @click="mediumSelectorVisible = true" />
          </div>
          <div v-if="!selectedMedium" class="inherit-picker__hint">
            {{ t("alchemy.inherit.noMediumHint") }}
          </div>
          <el-button v-if="!selectedMedium" size="small" plain class="inherit-inline-button" @click="mediumCodexVisible = true">
            {{ t("alchemy.inherit.viewMediums") }}
          </el-button>
          <div v-else class="inherit-medium-detail">
            <div class="inherit-medium-detail__head">
              <strong>{{ t(selectedMedium.nameKey) }}</strong>
              <el-button link size="small" type="danger" @click="clearMedium">{{ t("game.remove") }}</el-button>
            </div>
            <dl class="inherit-medium-kv">
              <div v-for="row in selectedMediumKvRows" :key="row.key" class="inherit-medium-kv__row">
                <dt>{{ row.label }}</dt>
                <dd>{{ row.value }}</dd>
              </div>
            </dl>
            <div class="inherit-medium-desc">{{ t(selectedMedium.descKey) }}</div>
          </div>
        </div>

        <div class="inherit-picker" data-tutorial="alchemy-inherit-carrier">
          <div class="inherit-picker__title"><span class="inherit-step">4</span>{{ t("alchemy.inherit.carrier") }}</div>
          <button v-if="selectedCarrier" type="button" class="inherit-instance" @click="carrierSelectorVisible = true">
            <Card :item-id="selectedCarrier.templateItemId" :instance-id="selectedCarrier.instanceId" :quantity="selectedCarrier.quantity" />
          </button>
          <EmptyCard v-else :label="t('alchemy.inherit.pickCarrier')" @click="carrierSelectorVisible = true" />
          <div class="inherit-picker__hint">{{ t("alchemy.inherit.orphanNotice") }}</div>
        </div>
      </div>
    </section>

    <AlchemyFusionStatusBar v-if="selectedMedium || allSources.length > 0" :cards="statusCards" />

    <AlchemyFusionSourceMatrix
      :source-groups="sourceGroups"
      :selected-kind="selectedKind"
      :kind-mismatch="kindMismatch"
      :decision-options="decisionOptions"
      :decision-for="decisionFor"
      :relation-label="relationLabel"
      :relation-hint="relationHint"
      :relation-tag-type="relationTagType"
      :decision-hint="decisionHint"
      @set-decision="setDecision"
    />

    <div v-if="issues.length > 0" class="alchemy-inherit-issues">
      <div class="alchemy-inherit-issues__title">{{ t("alchemy.bench.issuesTitle") }}</div>
      <ul>
        <li v-for="issue in issues" :key="issue">{{ issue }}</li>
      </ul>
    </div>

    <AlchemyFusionAffixPreview
      :inherit-used="inheritUsed"
      :suppress-used="suppressUsed"
      :expected-lineage-text="expectedLineageText"
      :hybrid-potential-count="hybridPotentialCount"
      :inbreed-risk-count="inbreedRiskCount"
      :expected-ignore-text="expectedIgnoreText"
      :affix-preview-lines="affixPreviewLines"
      :affix-preview-has-ordinary-source="affixPreviewHasOrdinarySource"
      :affix-preview-lineage-text="affixPreviewLineageText"
      :last-result="lastResult"
      :result-instance="lastResultInstance"
      :tag-type="affixPreviewTagType"
    />

    <div class="inherit-submit">
      <div class="inherit-submit__meta">
        <span>{{ t("alchemy.inherit.extraCost") }}</span>
        <strong>{{ t("alchemy.cost.gold", { value: extraGoldCost }) }}</strong>
      </div>
      <el-button type="primary" :loading="submitting || inheritCooldownActive" :disabled="issues.length > 0 || inheritCooldownActive" @click="submitInherit">
        {{ inheritCooldownActive ? t("alchemy.inherit.submitCooldown", { seconds: inheritCooldownSeconds }) : t("alchemy.inherit.submit") }}
      </el-button>
    </div>

    <InventoryInstanceSelectorDialog
      v-model="fatherSelectorVisible"
      :title="t('alchemy.inherit.pickLeftSource')"
      :instance-ids="parentCandidateIds"
      :ineligible-instance-ids="ineligibleParentIds"
      :selected-instance-id="fatherInstanceId"
      :empty-text="t('alchemy.inherit.noParents')"
      @confirm="handleFatherPick"
    />
    <InventoryInstanceSelectorDialog
      v-model="motherSelectorVisible"
      :title="t('alchemy.inherit.pickRightSource')"
      :instance-ids="motherCandidateIds"
      :ineligible-instance-ids="ineligibleParentIds"
      :selected-instance-id="motherInstanceId"
      :empty-text="t('alchemy.inherit.noParents')"
      @confirm="handleMotherPick"
    />
    <InventoryInstanceSelectorDialog
      v-model="mediumSelectorVisible"
      :title="t('alchemy.inherit.pickMedium')"
      :instance-ids="availableMediumInstanceIds"
      :selected-instance-id="null"
      :allow-clear="true"
      :template-item-ids="allMediumTemplateIds"
      :template-items-title="t('game.inventorySelectorAvailableItemsTitle', { title: t('alchemy.inherit.medium') })"
      :empty-text="t('alchemy.inherit.noMediums')"
      @confirm="handleSelectMedium"
    />

    <GameDialog v-model="mediumCodexVisible" :title="t('alchemy.inherit.mediumCodexTitle')" width="760px">
      <div class="inherit-medium-codex">
        <article v-for="medium in mediumOptions" :key="medium.itemId" class="inherit-medium-codex__item">
          <Card :item-id="medium.itemId" :quantity="medium.available" :hide-label="true" :width="54" :height="54" :icon-size="30" />
          <div>
            <strong>{{ t(medium.nameKey) }}</strong>
            <p>{{ t(medium.descKey) }}</p>
            <div class="inherit-medium-tags">
              <el-tag v-for="line in mediumLines(medium)" :key="line" size="small" effect="plain">{{ line }}</el-tag>
            </div>
          </div>
        </article>
      </div>
    </GameDialog>

    <InventoryInstanceSelectorDialog
      v-model="carrierSelectorVisible"
      :title="t('alchemy.inherit.pickCarrier')"
      :instance-ids="carrierCandidateIds"
      :ineligible-instance-ids="ineligibleCarrierIds"
      :selected-instance-id="carrierInstanceId"
      :template-item-ids="carrierTemplateItemIds"
      :template-items-title="t('game.inventorySelectorAvailableItemsTitle', { title: t('alchemy.inherit.carrier') })"
      :empty-text="t('alchemy.inherit.noCarriers')"
      @confirm="handleCarrierPick"
    />
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from "vue"
import { useI18n } from "vue-i18n"
import { ElMessage } from "element-plus"
import { useInventoryStore, type InventoryInstance } from "@stores/inventory"
import { useGameStore } from "@stores/game"
import { usePlayerStore } from "@stores/player"
import { apiPost } from "@services/api"
import { formatAttributeValueByKey, getAttributeDefinition } from "@services/attribute-display"
import { resolveErrorMessage } from "@services/error-messages"
import { getAffixDefinition, isCarrierItemId } from "@services/item-definition"
import { computeAlchemyInheritEffectiveInheritSlots, computeAlchemyInheritEffectiveSuppressSlots, computeAlchemyInheritIgnoreKeepChance, listAlchemyComboRecipes, listAlchemyInheritMediums, type AlchemyInheritMedium } from "@services/alchemy-data"
import { ALCHEMY_CONFIG } from "@/config/shared-global-config"
import Card from "@components/game/item/Card.vue"
import EmptyCard from "@components/game/item/EmptyCard.vue"
import AffixQuickRef from "@components/game/affix/AffixQuickRef.vue"
import InventoryInstanceSelectorDialog from "@components/game/inventory/InventoryInstanceSelectorDialog.vue"
import AlchemyFusionAffixPreview from "./AlchemyFusionAffixPreview.vue"
import AlchemyFusionSourceMatrix from "./AlchemyFusionSourceMatrix.vue"
import AlchemyFusionStatusBar from "./AlchemyFusionStatusBar.vue"
import type { EquipmentAffixSlot } from "@/types/equipment"
import GameDialog from "@components/game/common/GameDialog.vue"

type SourceDecision = "inherit" | "suppress" | "ignore"
type ParentSide = "father" | "mother"
type SourceComponentKind = "attr" | "effect"
type MediumKvRow = {
  key: string
  label: string
  value: string
}

type InheritSourceRow = {
  choiceKey: string
  parent: ParentSide
  componentIndex: number
  sourceKey: string
  attr: string
  attrLabel: string
  componentKind: SourceComponentKind
  sourceLabel: string
  sourceNote?: string
  effectLabel?: string
  compositionItemIds?: string[]
  itemId?: string
  affixId?: string
  affixSlot?: EquipmentAffixSlot | null
  comboRecipeId?: string
  currentValue: number
  rangeMin: number
  rangeMax: number
  valueText: string
  rangeText: string
}

type SourceMatrixRow = {
  key: string
  attr: string
  sourceKey: string
  father: InheritSourceRow | null
  mother: InheritSourceRow | null
  relation: "hybrid" | "inbreed" | "single"
}

type SourceGroup = {
  key: string
  attr: string
  label: string
  rows: SourceMatrixRow[]
}

type AffixPreviewStatus = "fixed" | "possible" | "hybrid" | "risk" | "removed"
type AffixPreviewTone = "positive" | "negative" | "neutral"

type PreviewSourceRef = {
  key: string
  label: string
  itemId: string
}

type AffixPreviewLine = {
  key: string
  attr: string
  label: string
  valueText: string
  projectionText: string
  sourceText: string
  sourceRefs: PreviewSourceRef[]
  ignoreText: string
  status: AffixPreviewStatus
  statusText: string
  probabilityText: string
  probabilityHint: string
  riskText: string
  tone: AffixPreviewTone
  hint: string
  hasOrdinarySource: boolean
}

type ParentAffixOption = {
  index: number
  affix: EquipmentAffixSlot
}

type InheritImmediateResult = {
  targetCarrierInstanceId?: number
  affix?: EquipmentAffixSlot | null
}

const emit = defineEmits<{
  (e: "submitted"): void
}>()

const { t } = useI18n()
const inventoryStore = useInventoryStore()
const gameStore = useGameStore()
const playerStore = usePlayerStore()
const comboById = new Map(listAlchemyComboRecipes().map((recipe) => [recipe.id, recipe]))

const fatherSelectorVisible = ref(false)
const motherSelectorVisible = ref(false)
const mediumSelectorVisible = ref(false)
const mediumCodexVisible = ref(false)
const carrierSelectorVisible = ref(false)
const fatherInstanceId = ref<number | null>(null)
const motherInstanceId = ref<number | null>(null)
const carrierInstanceId = ref<number | null>(null)
const fatherAffixIndex = ref(0)
const motherAffixIndex = ref(0)
const mediumItemId = ref("")
const decisions = ref<Record<string, SourceDecision>>({})
const submitting = ref(false)
const inheritCooldownUntil = ref(0)
const inheritCooldownNow = ref(Date.now())
const lastResult = ref<InheritImmediateResult | null>(null)
const lastResultInputSignature = ref("")
let inheritCooldownTimer: number | undefined

const decisionOptions = computed(() => [
  { label: t("alchemy.inherit.include"), value: "inherit" as const },
  { label: t("alchemy.inherit.suppress"), value: "suppress" as const },
  { label: t("alchemy.inherit.ignore"), value: "ignore" as const }
])
const playerAlchemyLevel = computed(() => Number(playerStore.attributes?.alchemy_level?.value ?? 1))
const playerAlchemyStability = computed(() => Number(playerStore.attributes?.alchemy_craft_success?.value ?? 0))
const playerAlchemyPrecision = computed(() => Number(playerStore.attributes?.alchemy_precision?.value ?? 0))
const mediumOptions = computed(() =>
  listAlchemyInheritMediums().map((medium) => ({ ...medium, available: itemQuantity(medium.itemId) }))
)
const selectedMedium = computed(() => mediumOptions.value.find((medium) => medium.itemId === mediumItemId.value) || null)
const selectedMediumOwnedQty = computed(() => selectedMedium.value ? itemQuantity(selectedMedium.value.itemId) : 0)
const allMediumTemplateIds = computed(() => mediumOptions.value.map((medium) => medium.itemId))
const availableMediumInstanceIds = computed(() => inventoryInstances.value.filter((item) => allMediumTemplateIds.value.includes(item.templateItemId)).map((item) => item.instanceId))
const selectedMediumKvRows = computed<MediumKvRow[]>(() => {
  const medium = selectedMedium.value
  if (!medium) return []
  return [
    { key: "inherit", label: t("alchemy.inherit.inheritSlots"), value: t("alchemy.inherit.mediumTimes", { count: effectiveInheritSlots.value }) },
    { key: "suppress", label: t("alchemy.inherit.suppressSlots"), value: t("alchemy.inherit.mediumTimes", { count: effectiveSuppressSlots.value }) },
    { key: "positive-ignore", label: t("alchemy.inherit.mediumPositiveIgnore"), value: formatPercent(positiveIgnoreKeepChance.value) },
    { key: "negative-ignore", label: t("alchemy.inherit.mediumNegativeIgnore"), value: formatPercent(negativeIgnoreKeepChance.value) },
    { key: "inbreed", label: t("alchemy.inherit.inbreedMutationChance"), value: formatPercent(medium.modifiers.inbreedMutationChance) },
    { key: "divisor", label: t("alchemy.inherit.hybridDivisor"), value: formatNumber(medium.modifiers.hybridMergeDivisor, 2) },
    { key: "consume", label: t("alchemy.inherit.mediumConsume"), value: t("alchemy.inherit.mediumConsumeValue", { count: medium.consumePerCraft, owned: medium.available ?? itemQuantity(medium.itemId) }) }
  ]
})

const inventoryInstances = computed(() => inventoryStore.instances.filter((item) => item.state === "inventory" && item.quantity > 0))
const selectedFather = computed(() => getInstance(fatherInstanceId.value))
const selectedMother = computed(() => getInstance(motherInstanceId.value))
const selectedCarrier = computed(() => getInstance(carrierInstanceId.value))
const lastResultInstance = computed(() => {
  const id = Number(lastResult.value?.targetCarrierInstanceId || 0)
  if (!id) return null
  return inventoryStore.instances.find((item) => item.instanceId === id) || null
})
const inheritCooldownRemainingMs = computed(() => Math.max(0, inheritCooldownUntil.value - inheritCooldownNow.value))
const inheritCooldownActive = computed(() => inheritCooldownRemainingMs.value > 0)
const inheritCooldownSeconds = computed(() => Math.max(1, Math.ceil(inheritCooldownRemainingMs.value / 1000)))
const fatherAffixOptions = computed(() => eligibleAffixOptions(selectedFather.value))
const motherAffixOptions = computed(() => eligibleAffixOptions(selectedMother.value))
const selectedFatherAffix = computed(() => fatherAffixOptions.value.find((option) => option.index === fatherAffixIndex.value)?.affix || null)
const selectedMotherAffix = computed(() => motherAffixOptions.value.find((option) => option.index === motherAffixIndex.value)?.affix || null)
const selectedKind = computed(() => affixInheritKind(selectedFatherAffix.value) || affixInheritKind(selectedMotherAffix.value))
const kindMismatch = computed(() => {
  const fatherKind = affixInheritKind(selectedFatherAffix.value)
  const motherKind = affixInheritKind(selectedMotherAffix.value)
  return !!fatherKind && !!motherKind && fatherKind !== motherKind
})

const parentCandidateIds = computed(() => inventoryInstances.value.filter((item) => eligibleAffixOptions(item).length > 0).map((item) => item.instanceId))
const motherCandidateIds = computed(() => parentCandidateIds.value.filter((id) => id !== fatherInstanceId.value))
const ineligibleParentIds = computed(() => inventoryInstances.value.filter((item) => isCarrierItemId(item.templateItemId) && (item.affixes || []).length > 0 && eligibleAffixOptions(item).length === 0).map((item) => item.instanceId))
const carrierCandidateIds = computed(() => inventoryInstances.value.filter(isBlankCarrier).map((item) => item.instanceId))
const ineligibleCarrierIds = computed(() => inventoryInstances.value.filter((item) => isCarrierItemId(item.templateItemId) && !isBlankCarrier(item)).map((item) => item.instanceId))
const carrierTemplateItemIds = computed(() => {
  const ids = new Set<string>()
  for (const item of inventoryStore.instances) {
    if (isCarrierItemId(item.templateItemId)) ids.add(item.templateItemId)
  }
  return Array.from(ids)
})

const fatherSources = computed(() => extractSources(selectedFatherAffix.value, selectedFather.value, "father"))
const motherSources = computed(() => extractSources(selectedMotherAffix.value, selectedMother.value, "mother"))
const allSources = computed(() => [...fatherSources.value, ...motherSources.value])
const sourceGroups = computed(() => buildSourceGroups(fatherSources.value, motherSources.value))
const inheritUsed = computed(() => allSources.value.filter((source) => decisionFor(source) === "inherit").length)
const suppressUsed = computed(() => allSources.value.filter((source) => decisionFor(source) === "suppress").length)
const effectiveInheritSlots = computed(() => computeAlchemyInheritEffectiveInheritSlots(ALCHEMY_CONFIG.inherit.baseInheritSlots, playerAlchemyPrecision.value, selectedMedium.value?.modifiers.inheritSlots ?? 0))
const effectiveSuppressSlots = computed(() => computeAlchemyInheritEffectiveSuppressSlots(ALCHEMY_CONFIG.inherit.baseSuppressSlots, playerAlchemyStability.value, selectedMedium.value?.modifiers.suppressSlots ?? 0))
const inheritOverLimit = computed(() => inheritUsed.value > effectiveInheritSlots.value)
const suppressOverLimit = computed(() => suppressUsed.value > effectiveSuppressSlots.value)
const retainedCandidateSources = computed(() => allSources.value.filter((source) => decisionFor(source) !== "suppress"))
const hybridPotentialCount = computed(() => countHybridPotential(retainedCandidateSources.value))
const inbreedRiskCount = computed(() => countInbreedRisk(retainedCandidateSources.value))
const ordinaryCandidateCount = computed(() => retainedCandidateSources.value.filter(isOrdinaryInheritSource).length)
const ordinaryGuaranteedCount = computed(() => allSources.value.filter((source) => decisionFor(source) === "inherit" && isOrdinaryInheritSource(source)).length)
const ignoreCandidateCount = computed(() => allSources.value.filter((source) => decisionFor(source) === "ignore").length)
const expectedLineageText = computed(() => {
  if (ordinaryGuaranteedCount.value > 0) return t("alchemy.inherit.previewLineageOrphanCertain", { count: ordinaryGuaranteedCount.value })
  if (ordinaryCandidateCount.value > 0) return t("alchemy.inherit.previewLineageOrphanPossible", { count: ordinaryCandidateCount.value })
  return t("alchemy.inherit.previewLineageNormal")
})
const expectedIgnoreText = computed(() => {
  if (ignoreCandidateCount.value <= 0) return t("alchemy.inherit.previewIgnoreNone")
  return t("alchemy.inherit.previewIgnoreValue", { count: ignoreCandidateCount.value })
})
const affixPreviewLines = computed(() => buildAffixPreviewLines(allSources.value))
const affixPreviewHasOrdinarySource = computed(() => affixPreviewLines.value.some((line) => line.hasOrdinarySource && line.status !== "removed"))
const affixPreviewLineageText = computed(() => affixPreviewHasOrdinarySource.value
  ? t("alchemy.inherit.affixPreviewLineageLocked")
  : t("alchemy.inherit.affixPreviewLineageClear")
)
const extraGoldCost = computed(() => {
  const mods = selectedMedium.value?.modifiers
  if (!mods) return 0
  return inheritUsed.value * Math.max(0, mods.inheritCost.gold) + suppressUsed.value * Math.max(0, mods.suppressCost.gold)
})
const positiveIgnoreKeepChance = computed(() => computeAlchemyInheritIgnoreKeepChance(selectedMedium.value, 0, 1, playerAlchemyStability.value))
const negativeIgnoreKeepChance = computed(() => computeAlchemyInheritIgnoreKeepChance(selectedMedium.value, 0, -1, playerAlchemyStability.value))
const keepRemaining = computed(() => Math.max(0, effectiveInheritSlots.value - inheritUsed.value))
const suppressRemaining = computed(() => Math.max(0, effectiveSuppressSlots.value - suppressUsed.value))
const keepStatusText = computed(() => inheritOverLimit.value
  ? t("alchemy.inherit.statusKeepOver", { used: inheritUsed.value, max: effectiveInheritSlots.value })
  : t("alchemy.inherit.statusKeepValue", { remaining: keepRemaining.value, used: inheritUsed.value, max: effectiveInheritSlots.value })
)
const suppressStatusText = computed(() => suppressOverLimit.value
  ? t("alchemy.inherit.statusSuppressOver", { used: suppressUsed.value, max: effectiveSuppressSlots.value })
  : t("alchemy.inherit.statusSuppressValue", { remaining: suppressRemaining.value, used: suppressUsed.value, max: effectiveSuppressSlots.value })
)
const ignoreStatusText = computed(() => t("alchemy.inherit.statusIgnoreValue", { count: ignoreCandidateCount.value }))
const alchemyFormulaRatio = computed(() => 1 / Math.max(0.01, 1 + Math.max(0, playerAlchemyStability.value)))
const slotFormulaHint = computed(() => t("alchemy.inherit.slotFormulaHint", {
  precision: formatPercent(playerAlchemyPrecision.value),
  inheritBase: ALCHEMY_CONFIG.inherit.baseInheritSlots,
  inherit: effectiveInheritSlots.value,
  suppressBase: ALCHEMY_CONFIG.inherit.baseSuppressSlots,
  suppress: effectiveSuppressSlots.value
}))
const ignoreStatusHint = computed(() => {
  const medium = selectedMedium.value
  if (!medium) return t("alchemy.inherit.statusIgnoreHintBase")
  return t("alchemy.inherit.statusIgnoreFormulaHint", {
    stability: formatPercent(playerAlchemyStability.value),
    ratio: formatPercent(alchemyFormulaRatio.value),
    positiveBase: formatPercent(ALCHEMY_CONFIG.inherit.positiveIgnoreKeepChanceBase),
    positive: formatPercent(positiveIgnoreKeepChance.value),
    negativeBase: formatPercent(ALCHEMY_CONFIG.inherit.negativeIgnoreKeepChanceBase),
    negative: formatPercent(negativeIgnoreKeepChance.value)
  })
})
const outcomeStatusText = computed(() => {
  if (inbreedRiskCount.value > 0) return t("alchemy.inherit.statusOutcomeRisk", { count: inbreedRiskCount.value, chance: formatPercent(adjustedInbreedMutationChance()) })
  if (hybridPotentialCount.value > 0) return t("alchemy.inherit.statusOutcomeHybrid", { count: hybridPotentialCount.value })
  return t("alchemy.inherit.statusOutcomeStable")
})
const outcomeStatusHint = computed(() => {
  if (inbreedRiskCount.value > 0) return t("alchemy.inherit.statusOutcomeRiskHint")
  if (hybridPotentialCount.value > 0) return t("alchemy.inherit.statusOutcomeHybridHint")
  return t("alchemy.inherit.statusOutcomeStableHint")
})
const statusCards = computed(() => [
  {
    key: "keep",
    title: t("alchemy.inherit.statusKeepTitle"),
    value: keepStatusText.value,
    hint: t("alchemy.inherit.statusKeepHint"),
    tooltip: slotFormulaHint.value,
    over: inheritOverLimit.value
  },
  {
    key: "suppress",
    title: t("alchemy.inherit.statusSuppressTitle"),
    value: suppressStatusText.value,
    hint: t("alchemy.inherit.statusSuppressHint"),
    tooltip: slotFormulaHint.value,
    over: suppressOverLimit.value
  },
  {
    key: "ignore",
    title: t("alchemy.inherit.statusIgnoreTitle"),
    value: ignoreStatusText.value,
    hint: t("alchemy.inherit.statusIgnoreHintShort"),
    tooltip: ignoreStatusHint.value
  },
  {
    key: "outcome",
    title: t("alchemy.inherit.statusOutcomeTitle"),
    value: outcomeStatusText.value,
    hint: outcomeStatusHint.value,
    risk: inbreedRiskCount.value > 0
  }
])
const submitArgs = computed<Record<string, unknown>>(() => ({
  fatherInstanceId: fatherInstanceId.value || 0,
  fatherAffixIndex: fatherAffixIndex.value,
  motherInstanceId: motherInstanceId.value || 0,
  motherAffixIndex: motherAffixIndex.value,
  carrierInstanceId: carrierInstanceId.value || 0,
  mediumItemId: selectedMedium.value?.itemId || "",
  geneChoices: allSources.value.map((source) => ({
    parent: source.parent,
    componentIndex: source.componentIndex,
    sourceKey: source.sourceKey,
    decision: decisionFor(source)
  }))
}))
const inputSignature = computed(() => JSON.stringify(submitArgs.value))
const issues = computed(() => {
  const rows: string[] = []
  if (!selectedFather.value) rows.push(t("alchemy.inherit.issue.noLeftSource"))
  if (!selectedMother.value) rows.push(t("alchemy.inherit.issue.noRightSource"))
  if (fatherInstanceId.value && fatherInstanceId.value === motherInstanceId.value) rows.push(t("alchemy.inherit.issue.sameParent"))
  if (!selectedFatherAffix.value) rows.push(t("alchemy.inherit.issue.noLeftAffix"))
  if (!selectedMotherAffix.value) rows.push(t("alchemy.inherit.issue.noRightAffix"))
  if (kindMismatch.value) rows.push(t("alchemy.inherit.issue.kindMismatch"))
  if (!selectedMedium.value) rows.push(t("alchemy.inherit.issue.noMedium"))
  if (selectedMedium.value && itemQuantity(selectedMedium.value.itemId) < selectedMedium.value.consumePerCraft) rows.push(t("alchemy.inherit.issue.mediumQty"))
  if (selectedMedium.value && playerAlchemyLevel.value < selectedMedium.value.minAlchemyLevel) rows.push(t("alchemy.inherit.issue.mediumLevel", { level: selectedMedium.value.minAlchemyLevel }))
  if (!selectedCarrier.value) rows.push(t("alchemy.inherit.issue.noCarrier"))
  if (allSources.value.length === 0) rows.push(t("alchemy.inherit.issue.noSources"))
  if (inheritOverLimit.value) rows.push(t("alchemy.inherit.issue.inheritSlots", { used: inheritUsed.value, max: effectiveInheritSlots.value }))
  if (suppressOverLimit.value) rows.push(t("alchemy.inherit.issue.suppressSlots", { used: suppressUsed.value, max: effectiveSuppressSlots.value }))
  return rows
})

watch(parentCandidateIds, (ids) => {
  if (!fatherInstanceId.value && ids[0]) fatherInstanceId.value = ids[0]
  if (!motherInstanceId.value) motherInstanceId.value = ids.find((id) => id !== fatherInstanceId.value) || null
}, { immediate: true })

watch(carrierCandidateIds, (ids) => {
  if (!carrierInstanceId.value && ids[0]) carrierInstanceId.value = ids[0]
}, { immediate: true })

watch(fatherAffixOptions, (options) => {
  if (!options.some((option) => option.index === fatherAffixIndex.value)) fatherAffixIndex.value = options[0]?.index ?? 0
}, { immediate: true })

watch(motherAffixOptions, (options) => {
  if (!options.some((option) => option.index === motherAffixIndex.value)) motherAffixIndex.value = options[0]?.index ?? 0
}, { immediate: true })

watch(allSources, (rows) => {
  const keys = new Set(rows.map((row) => row.choiceKey))
  const next: Record<string, SourceDecision> = {}
  for (const key of keys) next[key] = decisions.value[key] || "ignore"
  decisions.value = next
})

onBeforeUnmount(() => {
  if (inheritCooldownTimer !== undefined) {
    window.clearInterval(inheritCooldownTimer)
    inheritCooldownTimer = undefined
  }
})

function handleFatherPick(instanceId: number | null) {
  clearLastResultOnUserChange(instanceId !== fatherInstanceId.value)
  fatherInstanceId.value = instanceId
  if (motherInstanceId.value === instanceId) motherInstanceId.value = motherCandidateIds.value[0] || null
}

function handleMotherPick(instanceId: number | null) {
  clearLastResultOnUserChange(instanceId !== motherInstanceId.value)
  motherInstanceId.value = instanceId
}

function handleCarrierPick(instanceId: number | null) {
  clearLastResultOnUserChange(instanceId !== carrierInstanceId.value)
  carrierInstanceId.value = instanceId
}

function handleSelectMedium(instanceId: number | null) {
  if (!instanceId) {
    clearMedium()
    return
  }
  const inst = inventoryStore.instances.find((row) => row.instanceId === instanceId)
  const nextMediumItemId = inst?.templateItemId || ""
  clearLastResultOnUserChange(nextMediumItemId !== mediumItemId.value)
  mediumItemId.value = nextMediumItemId
}

function clearMedium() {
  clearLastResultOnUserChange(mediumItemId.value !== "")
  mediumItemId.value = ""
}

function clearLastResult() {
  lastResult.value = null
  lastResultInputSignature.value = ""
}

function clearLastResultOnUserChange(changed = true) {
  if (changed) clearLastResult()
}

function selectFatherAffix(index: number) {
  clearLastResultOnUserChange(index !== fatherAffixIndex.value)
  fatherAffixIndex.value = index
}

function selectMotherAffix(index: number) {
  clearLastResultOnUserChange(index !== motherAffixIndex.value)
  motherAffixIndex.value = index
}

async function submitInherit() {
  if (issues.value.length > 0 || submitting.value || inheritCooldownActive.value || !gameStore.token) return
  submitting.value = true
  clearLastResult()
  startInheritCooldown()
  try {
    const result = await apiPost<InheritImmediateResult>("/api/alchemy/inherit", gameStore.token, submitArgs.value)
    lastResult.value = result
    lastResultInputSignature.value = inputSignature.value
    await inventoryStore.refreshAll(gameStore.token).catch(() => undefined)
    ElMessage.success(t("alchemy.inherit.success"))
    emit("submitted")
  } catch (err) {
    ElMessage.error(resolveErrorMessage(t, err, "alchemy.inherit.failed"))
  } finally {
    submitting.value = false
  }
}

function startInheritCooldown(ms = 10_000) {
  inheritCooldownNow.value = Date.now()
  inheritCooldownUntil.value = Math.max(inheritCooldownUntil.value, inheritCooldownNow.value + ms)
  if (inheritCooldownTimer !== undefined) return
  inheritCooldownTimer = window.setInterval(() => {
    inheritCooldownNow.value = Date.now()
    if (!inheritCooldownActive.value && inheritCooldownTimer !== undefined) {
      window.clearInterval(inheritCooldownTimer)
      inheritCooldownTimer = undefined
    }
  }, 250)
}

function decisionFor(source: InheritSourceRow): SourceDecision {
  return decisions.value[source.choiceKey] || "ignore"
}

function setDecision(source: InheritSourceRow, decision: SourceDecision) {
  clearLastResultOnUserChange(decision !== decisionFor(source))
  decisions.value = { ...decisions.value, [source.choiceKey]: decision }
}

function decisionHint(source: { attr: string; currentValue: number }, decision: SourceDecision): string {
  if (decision === "inherit") {
    const gold = selectedMedium.value?.modifiers.inheritCost.gold ?? 0
    return t("alchemy.inherit.decisionHintInherit", { cost: t("alchemy.cost.gold", { value: gold }) })
  }
  if (decision === "suppress") {
    const gold = selectedMedium.value?.modifiers.suppressCost.gold ?? 0
    return t("alchemy.inherit.decisionHintSuppress", { cost: t("alchemy.cost.gold", { value: gold }) })
  }
  const keep = ignoreKeepChanceForSource(source)
  return t("alchemy.inherit.decisionHintIgnore", {
    keep: formatPercent(keep),
    discard: formatPercent(1 - keep)
  })
}

function getInstance(instanceId: number | null): InventoryInstance | null {
  if (!instanceId) return null
  return inventoryStore.instances.find((item) => item.instanceId === instanceId && item.state === "inventory" && item.quantity > 0) || null
}

function itemQuantity(itemId: string): number {
  return inventoryStore.instances.reduce((sum, item) => item.state === "inventory" && item.templateItemId === itemId ? sum + item.quantity : sum, 0)
}

function isBlankCarrier(item: InventoryInstance): boolean {
  return item.state === "inventory" && item.quantity > 0 && isCarrierItemId(item.templateItemId) && (item.affixes || []).length === 0
}

function eligibleAffixOptions(item: InventoryInstance | null): ParentAffixOption[] {
  if (!item) return []
  return (item.affixes || [])
    .map((affix, index) => ({ affix, index }))
    .filter((option) => isEligibleAffix(item, option.affix))
}

function isEligibleAffix(item: InventoryInstance, affix: EquipmentAffixSlot): boolean {
  if (!isCarrierItemId(item.templateItemId)) return false
  if (isCustomAffix(affix)) {
    const kind = String(affix.sourceKind || "").trim()
    if (kind === "inherit_orphan" || Boolean(affix.effectParams?.inheritOrphan)) return false
    if (kind !== "craft" && kind !== "inherit") return false
    return !!affixInheritKind(affix) && extractCustomSources(affix, "father").length > 0
  }
  const def = getAffixDefinition(String(affix.affixId || "").trim())
  return !!def?.active && !!affixInheritKind(affix) && extractOrdinarySources(affix, "father").length > 0
}

function extractSources(affix: EquipmentAffixSlot | null, item: InventoryInstance | null, parent: ParentSide): InheritSourceRow[] {
  if (!affix) return []
  const rows = isCustomAffix(affix) ? extractCustomSources(affix, parent) : extractOrdinarySources(affix, parent)
  return rows.map((row) => ({ ...row, affixSlot: row.affixId ? affix : row.affixSlot, sourceNote: row.sourceNote || sourceParentLabel(parent, item) }))
}

function extractCustomSources(affix: EquipmentAffixSlot, parent: ParentSide): InheritSourceRow[] {
  const components = Array.isArray(affix.effectParams?.alchemyComponents) ? affix.effectParams?.alchemyComponents || [] : []
  const comboMaterialItemIds = comboMaterialItemsFromComponents(components)
  const rows = components
    .map((entry, index) => sourceFromComponent(entry, index, parent, affix, comboMaterialItemIds))
    .filter((row): row is InheritSourceRow => !!row)
  const bundledItems = new Set<string>()
  for (const row of rows) {
    if (!row.comboRecipeId || row.compositionItemIds?.length === 0) continue
    for (const itemId of row.compositionItemIds || []) bundledItems.add(itemId)
  }
  if (bundledItems.size === 0) return rows
  return rows.filter((row) => {
    if (row.comboRecipeId) return true
    return !row.itemId || !bundledItems.has(row.itemId)
  })
}

function comboMaterialItemsFromComponents(components: unknown[]): string[] {
  const itemIds: string[] = []
  for (const entry of components) {
    if (!entry || typeof entry !== "object" || Array.isArray(entry)) continue
    const row = entry as Record<string, unknown>
    const sourceKind = String(row.sourceKind || row.sourceType || "").trim()
    if (sourceKind !== "combo_material") continue
    const itemId = String(row.itemId || "").trim()
    if (itemId) itemIds.push(itemId)
  }
  return uniqueStrings(itemIds)
}

function extractOrdinarySources(affix: EquipmentAffixSlot, parent: ParentSide): InheritSourceRow[] {
  const affixId = String(affix.affixId || "").trim()
  const def = getAffixDefinition(affixId)
  if (!def?.active) return []
  const params = resolveAffixParamsForSlot(def.effectParams, affix.rarityId)
  const attrMods = affix.attrMods || {}
  const attrKeys = Object.keys(attrMods).filter(Boolean).sort()
  const rows: InheritSourceRow[] = attrKeys.map((attr, index) => {
    const currentValue = numberValue(attrMods[attr], 0)
    const range = ordinaryAttrRange(params, attr, currentValue)
    return buildSourceRow({
      parent,
      componentIndex: index,
      sourceKey: `affix:${affixId}:${attr}`,
      attr,
      componentKind: "attr",
      currentValue,
      rangeMin: range.min,
      rangeMax: range.max,
      sourceLabel: t(def.nameKey, affixId),
      sourceNote: t("alchemy.inherit.ordinaryAffixSource"),
      affixId,
      affixSlot: affix
    })
  })
  if (hasComplexEffectParams(params) || rows.length === 0) {
    rows.push(buildSourceRow({
      parent,
      componentIndex: rows.length,
      sourceKey: `affix:${affixId}:effect`,
      attr: "",
      componentKind: "effect",
      currentValue: 0,
      rangeMin: 0,
      rangeMax: 0,
      sourceLabel: t(def.nameKey, affixId),
      sourceNote: t("alchemy.inherit.orphanSourceShort"),
      affixId,
      affixSlot: affix
    }))
  }
  return rows
}

function sourceFromComponent(entry: unknown, index: number, parent: ParentSide, affix: EquipmentAffixSlot, comboMaterialItemIds: string[] = []): InheritSourceRow | null {
  if (!entry || typeof entry !== "object" || Array.isArray(entry)) return null
  const row = entry as Record<string, unknown>
  const attr = cleanSourceAttr(String(row.attr || "").trim())
  const sourceKind = String(row.sourceKind || row.sourceType || "").trim()
  if (sourceKind === "combo_material") return null
  const comboRecipeId = String(row.comboRecipeId || "").trim()
  const componentKind = (!attr && (sourceKind === "combo_output" || comboRecipeId))
    ? "effect"
    : String(row.componentKind || (attr ? "attr" : "effect")).trim() === "effect" ? "effect" : "attr"
  if (!attr && componentKind !== "effect") return null
  const sourceKey = cleanSourceKey(String(row.sourceKey || "").trim()) || deriveSourceKey(row, attr, index)
  if (!sourceKey) return null
  const currentValue = numberValue(row.currentValue ?? row.value, 0)
  const range = row.valueRange && typeof row.valueRange === "object" && !Array.isArray(row.valueRange)
    ? row.valueRange as Record<string, unknown>
    : {}
  const rangeMin = numberValue(range.min, currentValue)
  const rangeMax = numberValue(range.max, currentValue)
  const sourceMeta = componentSourceMeta(row)
  const compositionItemIds = sourceMeta.comboRecipeId
    ? uniqueStrings([...stringArray(row.sourceItems), ...comboMaterialItemIds])
    : []
  return buildSourceRow({
    parent,
    componentIndex: index,
    sourceKey,
    attr,
    componentKind,
    currentValue,
    rangeMin,
    rangeMax,
    sourceLabel: sourceMeta.label,
    sourceNote: sourceMeta.note,
    effectLabel: sourceMeta.effectLabel,
    compositionItemIds,
    itemId: sourceMeta.itemId,
    affixId: sourceMeta.affixId,
    affixSlot: sourceMeta.affixId ? affix : null,
    comboRecipeId: sourceMeta.comboRecipeId
  })
}

function buildSourceRow(input: {
  parent: ParentSide
  componentIndex: number
  sourceKey: string
  attr: string
  componentKind: SourceComponentKind
  currentValue: number
  rangeMin: number
  rangeMax: number
  sourceLabel: string
  sourceNote?: string
  effectLabel?: string
  compositionItemIds?: string[]
  itemId?: string
  affixId?: string
  affixSlot?: EquipmentAffixSlot | null
  comboRecipeId?: string
}): InheritSourceRow {
  const attr = input.attr
  const valueText = attr ? formatAttributeValueByKey(attr, input.currentValue, true) : ""
  const rangeText = attr
    ? `${formatAttributeValueByKey(attr, Math.min(input.rangeMin, input.rangeMax), true)} ~ ${formatAttributeValueByKey(attr, Math.max(input.rangeMin, input.rangeMax), true)}`
    : t("alchemy.inherit.effectSourceRange")
  return {
    choiceKey: `${input.parent}:${input.componentIndex}:${input.sourceKey}`,
    parent: input.parent,
    componentIndex: input.componentIndex,
    sourceKey: input.sourceKey,
    attr,
    attrLabel: attrLabel(attr),
    componentKind: input.componentKind,
    sourceLabel: input.sourceLabel,
    sourceNote: input.sourceNote,
    effectLabel: input.effectLabel,
    compositionItemIds: input.compositionItemIds,
    itemId: input.itemId,
    affixId: input.affixId,
    affixSlot: input.affixSlot,
    comboRecipeId: input.comboRecipeId,
    currentValue: input.currentValue,
    rangeMin: input.rangeMin,
    rangeMax: input.rangeMax,
    valueText,
    rangeText
  }
}

function buildSourceGroups(father: InheritSourceRow[], mother: InheritSourceRow[]): SourceGroup[] {
  const groupKeys = new Set([...father, ...mother].map((source) => source.attr || "effect"))
  return Array.from(groupKeys).sort().map((attr) => {
    const fatherRows = father.filter((source) => (source.attr || "effect") === attr)
    const motherRows = mother.filter((source) => (source.attr || "effect") === attr)
    const rows: SourceMatrixRow[] = []
    const usedFather = new Set<string>()
    const usedMother = new Set<string>()
    const sameKeys = Array.from(new Set(fatherRows.map((source) => source.sourceKey).filter((key) => motherRows.some((source) => source.sourceKey === key)))).sort()
    for (const sourceKey of sameKeys) {
      const left = fatherRows.find((source) => source.sourceKey === sourceKey) || null
      const right = motherRows.find((source) => source.sourceKey === sourceKey) || null
      if (!left || !right) continue
      usedFather.add(left.choiceKey)
      usedMother.add(right.choiceKey)
      rows.push({ key: `${attr}:inbreed:${sourceKey}`, attr, sourceKey, father: left, mother: right, relation: "inbreed" })
    }
    const fatherOnly = fatherRows.filter((source) => !usedFather.has(source.choiceKey)).sort((a, b) => a.sourceKey.localeCompare(b.sourceKey))
    const motherOnly = motherRows.filter((source) => !usedMother.has(source.choiceKey)).sort((a, b) => a.sourceKey.localeCompare(b.sourceKey))
    if (attr !== "effect" && fatherOnly.length > 0 && motherOnly.length > 0) {
      for (const left of fatherOnly) {
        for (const right of motherOnly) {
          rows.push({ key: `${attr}:hybrid:${left.sourceKey}:${right.sourceKey}`, attr, sourceKey: `${left.sourceKey}|${right.sourceKey}`, father: left, mother: right, relation: "hybrid" })
        }
      }
    } else {
      rows.push(...fatherOnly.map((source) => ({ key: `${attr}:father:${source.sourceKey}`, attr, sourceKey: source.sourceKey, father: source, mother: null, relation: "single" as const })))
      rows.push(...motherOnly.map((source) => ({ key: `${attr}:mother:${source.sourceKey}`, attr, sourceKey: source.sourceKey, father: null, mother: source, relation: "single" as const })))
    }
    return { key: attr, attr: attr === "effect" ? "" : attr, label: attr === "effect" ? t("alchemy.inherit.effectSourceGroup") : attrLabel(attr), rows }
  })
}

function countHybridPotential(sources: InheritSourceRow[]): number {
  const byAttr = new Map<string, { father: Set<string>; mother: Set<string> }>()
  for (const source of sources) {
    if (!source.attr) continue
    if (!byAttr.has(source.attr)) byAttr.set(source.attr, { father: new Set(), mother: new Set() })
    byAttr.get(source.attr)?.[source.parent].add(source.sourceKey)
  }
  let count = 0
  for (const entry of byAttr.values()) {
    for (const fatherKey of entry.father) {
      for (const motherKey of entry.mother) {
        if (fatherKey !== motherKey) count++
      }
    }
  }
  return count
}

function countInbreedRisk(sources: InheritSourceRow[]): number {
  const counts = new Map<string, Set<string>>()
  for (const source of sources) {
    if (!source.attr) continue
    const key = `${source.attr}:${source.sourceKey}`
    if (!counts.has(key)) counts.set(key, new Set())
    counts.get(key)?.add(source.parent)
  }
  return Array.from(counts.values()).filter((parents) => parents.size > 1).length
}

function isOrdinaryInheritSource(source: InheritSourceRow): boolean {
  const affixId = String(source.affixId || "").trim()
  return affixId !== "" && !affixId.startsWith("custom:")
}

function buildAffixPreviewLines(sources: InheritSourceRow[]): AffixPreviewLine[] {
  const lines: AffixPreviewLine[] = []
  const attrGroups = new Map<string, InheritSourceRow[]>()
  const effectGroups = new Map<string, InheritSourceRow[]>()
  for (const source of sources) {
    if (source.attr) {
      if (!attrGroups.has(source.attr)) attrGroups.set(source.attr, [])
      attrGroups.get(source.attr)?.push(source)
      continue
    }
    if (!effectGroups.has(source.sourceKey)) effectGroups.set(source.sourceKey, [])
    effectGroups.get(source.sourceKey)?.push(source)
  }

  for (const [attr, group] of Array.from(attrGroups.entries()).sort((a, b) => attrLabel(a[0]).localeCompare(attrLabel(b[0])))) {
    const kept = group.filter((source) => decisionFor(source) === "inherit")
    const ignored = group.filter((source) => decisionFor(source) === "ignore")
    const suppressed = group.filter((source) => decisionFor(source) === "suppress")
    const candidates = [...kept, ...ignored]
    if (candidates.length > 0) lines.push(buildAttrPreviewLine(attr, candidates, kept, ignored))
    if (suppressed.length > 0) lines.push(buildAttrPreviewLine(attr, suppressed, [], "removed"))
  }

  for (const [, group] of Array.from(effectGroups.entries()).sort((a, b) => sourceListLabel(a[1]).localeCompare(sourceListLabel(b[1])))) {
    const kept = group.filter((source) => decisionFor(source) === "inherit")
    const ignored = group.filter((source) => decisionFor(source) === "ignore")
    const suppressed = group.filter((source) => decisionFor(source) === "suppress")
    const candidates = [...kept, ...ignored]
    if (candidates.length > 0) lines.push(buildEffectPreviewLine(candidates, kept, ignored))
    if (suppressed.length > 0) lines.push(buildEffectPreviewLine(suppressed, [], "removed"))
  }

  return lines
}

function buildAttrPreviewLine(attr: string, sources: InheritSourceRow[], inheritedOrIgnored: InheritSourceRow[], ignoredOrStatus: InheritSourceRow[] | AffixPreviewStatus, forcedStatus?: AffixPreviewStatus): AffixPreviewLine {
  const inherited = Array.isArray(inheritedOrIgnored) ? inheritedOrIgnored.filter((source) => decisionFor(source) === "inherit") : []
  const ignored = Array.isArray(ignoredOrStatus) ? ignoredOrStatus : []
  const explicitStatus = typeof ignoredOrStatus === "string" ? ignoredOrStatus : forcedStatus
  const value = mergePreviewValue(sources)
  const bestValue = bestPreviewValue(sources)
  const riskValue = riskPreviewValue(sources)
  const status = explicitStatus || previewStatusForSources(sources, inherited)
  const baseValue = baselinePreviewValue(sources, inherited, status, value)
  const probability = previewRetentionProbability(sources)
  const riskProbability = status === "risk" ? previewInbreedMutationProbability(sources) : 0
  const sourceText = t("alchemy.inherit.affixPreviewFrom", { sources: sourceListLabel(sources) })
  const sourceRefs = sourcePreviewRefs(sources)
  const projectionText = attrPreviewProjectionText(attr, baseValue, bestValue, riskValue, status)
  const calculationText = attrPreviewCalculationText(attr, sources, value, status)
  return {
    key: `attr:${attr}:${status}:${sources.map((source) => source.choiceKey).join("|")}`,
    attr,
    label: attrLabel(attr),
    valueText: formatAttributeValueByKey(attr, value, true),
    projectionText,
    sourceText,
    sourceRefs,
    ignoreText: "",
    status,
    statusText: affixPreviewStatusText(status),
    probabilityText: previewProbabilityText(status, probability),
    probabilityHint: previewProbabilityHint(status, probability),
    riskText: previewRiskText(status, riskProbability),
    tone: valueTone(value),
    hint: calculationText,
    hasOrdinarySource: sources.some(isOrdinaryInheritSource) || ignored.some(isOrdinaryInheritSource)
  }
}

function buildEffectPreviewLine(sources: InheritSourceRow[], inheritedOrIgnored: InheritSourceRow[], ignoredOrStatus: InheritSourceRow[] | AffixPreviewStatus, forcedStatus?: AffixPreviewStatus): AffixPreviewLine {
  const ignored = Array.isArray(ignoredOrStatus) ? ignoredOrStatus : []
  const explicitStatus = typeof ignoredOrStatus === "string" ? ignoredOrStatus : forcedStatus
  const status = explicitStatus || (sources.some((source) => decisionFor(source) === "inherit") ? "fixed" : "possible")
  const probability = previewRetentionProbability(sources)
  const sourceText = t("alchemy.inherit.affixPreviewFrom", { sources: sourceListLabel(sources) })
  const sourceRefs = sourcePreviewRefs(sources)
  return {
    key: `effect:${status}:${sources.map((source) => source.choiceKey).join("|")}`,
    attr: "",
    label: sourceListLabel(sources),
    valueText: sourceEffectLabel(sources),
    projectionText: "",
    sourceText,
    sourceRefs,
    ignoreText: "",
    status,
    statusText: affixPreviewStatusText(status),
    probabilityText: previewProbabilityText(status, probability),
    probabilityHint: previewProbabilityHint(status, probability),
    riskText: "",
    tone: "neutral",
    hint: "",
    hasOrdinarySource: sources.some(isOrdinaryInheritSource) || ignored.some(isOrdinaryInheritSource)
  }
}

function previewStatusForSources(sources: InheritSourceRow[], inherited: InheritSourceRow[] = []): AffixPreviewStatus {
  if (hasCrossParentRepeatedSource(sources)) return "risk"
  if (hasCrossParentHybridSource(sources)) return "hybrid"
  if (inherited.length === 0 && sources.some((source) => decisionFor(source) === "ignore")) return "possible"
  return "fixed"
}

function hasCrossParentRepeatedSource(sources: InheritSourceRow[]): boolean {
  const father = new Set(sources.filter((source) => source.parent === "father").map((source) => source.sourceKey))
  return sources.some((source) => source.parent === "mother" && father.has(source.sourceKey))
}

function hasCrossParentHybridSource(sources: InheritSourceRow[]): boolean {
  const father = new Set(sources.filter((source) => source.parent === "father").map((source) => source.sourceKey))
  const mother = new Set(sources.filter((source) => source.parent === "mother").map((source) => source.sourceKey))
  for (const left of father) {
    for (const right of mother) {
      if (left !== right) return true
    }
  }
  return false
}

function shouldPreviewFuseValues(sources: InheritSourceRow[]): boolean {
  return hasCrossParentRepeatedSource(sources) || hasCrossParentHybridSource(sources)
}

function effectiveHybridMergeDivisor(): number {
  return Math.max(1, selectedMedium.value?.modifiers.hybridMergeDivisor ?? ALCHEMY_CONFIG.inherit.hybridMergeDivisor)
}

function previewMergeDenominator(sources: InheritSourceRow[]): number {
  if (sources.length <= 1 || !shouldPreviewFuseValues(sources)) return 1
  const divisor = effectiveHybridMergeDivisor()
  return 1 + (sources.length - 1) * (divisor - 1)
}

function mergePreviewValue(sources: InheritSourceRow[]): number {
  if (sources.length <= 0) return 0
  const sum = sources.reduce((total, source) => total + source.currentValue, 0)
  const denominator = previewMergeDenominator(sources)
  return Math.round((sum / denominator) * 1000) / 1000
}

function bestPreviewValue(sources: InheritSourceRow[]): number {
  const stable = mergePreviewValue(sources)
  if (!hasCrossParentRepeatedSource(sources)) return stable
  const range = selectedMedium.value?.modifiers.inbreedGoodMultiplierRange ?? ALCHEMY_CONFIG.inherit.inbreedGoodMultiplierRange
  const mult = Math.max(1, range.max || 1)
  const best = stable < 0 ? stable / mult : stable * mult
  return clampPreviewValue(best, sources)
}

function lessBeneficialValue(candidate: number, current: number): number {
  if (!Number.isFinite(candidate)) return current
  if (!Number.isFinite(current)) return candidate
  if (candidate >= 0 && current >= 0) return candidate < current ? candidate : current
  if (candidate <= 0 && current <= 0) return candidate < current ? candidate : current
  return candidate < current ? candidate : current
}

function baselinePreviewValue(sources: InheritSourceRow[], inherited: InheritSourceRow[], status: AffixPreviewStatus, mergedValue: number): number {
  if (status === "hybrid") return bestIndividualPreviewValue(sources)
  if (inherited.length > 0 && inherited.length < sources.length) return mergePreviewValue(inherited)
  return mergedValue
}

function bestIndividualPreviewValue(sources: InheritSourceRow[]): number {
  if (sources.length <= 0) return 0
  return sources.map((source) => source.currentValue).reduce((best, value) => moreBeneficialValue(value, best), sources[0].currentValue)
}

function moreBeneficialValue(candidate: number, current: number): number {
  if (!Number.isFinite(candidate)) return current
  if (!Number.isFinite(current)) return candidate
  if (candidate >= 0 && current >= 0) return candidate > current ? candidate : current
  if (candidate <= 0 && current <= 0) return candidate > current ? candidate : current
  return candidate > current ? candidate : current
}

function riskPreviewValue(sources: InheritSourceRow[]): number {
  const stable = mergePreviewValue(sources)
  if (!hasCrossParentRepeatedSource(sources)) return stable
  const range = selectedMedium.value?.modifiers.inbreedBadMultiplierRange ?? ALCHEMY_CONFIG.inherit.inbreedBadMultiplierRange
  const mult = Math.max(0.01, range.min || 1)
  const risk = stable < 0 ? stable / mult : stable * mult
  return lessBeneficialValue(clampPreviewValue(risk, sources), stable)
}

function clampPreviewValue(value: number, sources: InheritSourceRow[]): number {
  const primary = sources[0]
  if (!primary || !Number.isFinite(value)) return value
  const min = Math.min(primary.rangeMin, primary.rangeMax)
  const max = Math.max(primary.rangeMin, primary.rangeMax)
  const cap = Math.max(1, ALCHEMY_CONFIG.inherit.numericValueCapMultiplier)
  if (min === 0 && max === 0) return value
  if (min < 0 && max > 0) return Math.max(min * cap, Math.min(max * cap, value))
  if (max <= 0) return Math.max(min * cap, Math.min(0, value))
  return Math.max(0, Math.min(max * cap, value))
}

function previewRetentionProbability(sources: InheritSourceRow[]): number {
  if (sources.some((source) => decisionFor(source) === "inherit")) return 1
  const discardAll = sources.reduce((acc, source) => acc * (1 - sourceRetentionProbability(source)), 1)
  return clampProbability(1 - discardAll)
}

function sourceRetentionProbability(source: InheritSourceRow): number {
  const decision = decisionFor(source)
  if (decision === "inherit") return 1
  if (decision === "suppress") return 0
  return ignoreKeepChanceForSource(source)
}

function previewInbreedMutationProbability(sources: InheritSourceRow[]): number {
  const pairKeepChance = crossParentPairProbability(sources, true)
  return clampProbability(pairKeepChance * adjustedInbreedMutationChance())
}

function crossParentPairProbability(sources: InheritSourceRow[], sameSource: boolean): number {
  const fathers = sources.filter((source) => source.parent === "father")
  const mothers = sources.filter((source) => source.parent === "mother")
  let noPair = 1
  for (const father of fathers) {
    for (const mother of mothers) {
      if ((father.sourceKey === mother.sourceKey) !== sameSource) continue
      noPair *= 1 - sourceRetentionProbability(father) * sourceRetentionProbability(mother)
    }
  }
  return clampProbability(1 - noPair)
}

function adjustedInbreedMutationChance(): number {
  const mods = selectedMedium.value?.modifiers
  const base = mods?.inbreedMutationChance ?? ALCHEMY_CONFIG.inherit.inbreedMutationChance
  const scaling = mods?.stabilityScaling
  const reduction = scaling
    ? Math.min(Math.max(0, scaling.inbreedMutationChanceReductionCap), Math.max(0, playerAlchemyStability.value) * Math.max(0, scaling.inbreedMutationChanceReductionPerPoint))
    : 0
  return clampProbability(base - reduction)
}

function attrPreviewProjectionText(attr: string, currentValue: number, bestValue: number, riskValue: number, status: AffixPreviewStatus): string {
  if (!attr || status === "removed") return ""
  if (status === "risk") {
    const bestChanged = Math.abs(bestValue - currentValue) >= 0.0001
    const riskChanged = Math.abs(riskValue - currentValue) >= 0.0001
    if (!bestChanged && !riskChanged) return ""
    if (bestChanged && riskChanged) return t("alchemy.inherit.affixPreviewProjectionRisk", {
      current: formatAttributeValueByKey(attr, currentValue, true),
      best: formatAttributeValueByKey(attr, bestValue, true),
      risk: formatAttributeValueByKey(attr, riskValue, true)
    })
    if (bestChanged) return t("alchemy.inherit.affixPreviewProjectionRiskBestOnly", {
      current: formatAttributeValueByKey(attr, currentValue, true),
      best: formatAttributeValueByKey(attr, bestValue, true)
    })
    return t("alchemy.inherit.affixPreviewProjectionRiskDropOnly", {
      current: formatAttributeValueByKey(attr, currentValue, true),
      risk: formatAttributeValueByKey(attr, riskValue, true)
    })
  }
  if (status === "hybrid") {
    if (Math.abs(bestValue - currentValue) < 0.0001) return ""
    if (moreBeneficialValue(bestValue, currentValue) !== bestValue) return ""
  }
  if (status !== "hybrid" && Math.abs(bestValue - currentValue) < 0.0001) return ""
  return t("alchemy.inherit.affixPreviewProjection", {
    current: formatAttributeValueByKey(attr, currentValue, true),
    best: formatAttributeValueByKey(attr, bestValue, true)
  })
}

function attrPreviewCalculationText(attr: string, sources: InheritSourceRow[], value: number, status: AffixPreviewStatus): string {
  if (!attr || status === "removed" || !shouldPreviewFuseValues(sources)) return ""
  const denominator = previewMergeDenominator(sources)
  if (denominator <= 1) return ""
  const values = sources.map((source) => formatAttributeValueByKey(attr, source.currentValue, true)).join(" + ")
  return t("alchemy.inherit.affixPreviewFusionFormula", {
    values,
    divisor: formatNumber(denominator, 2),
    result: formatAttributeValueByKey(attr, value, true)
  })
}

function previewProbabilityText(status: AffixPreviewStatus, probability: number): string {
  if (status === "removed") return t("alchemy.inherit.affixPreviewChanceRemoved", { chance: formatPercent(0) })
  if (status === "fixed") return t("alchemy.inherit.affixPreviewChanceFixed", { chance: formatPercent(1) })
  return formatPercent(probability)
}

function previewProbabilityHint(status: AffixPreviewStatus, probability: number): string {
  if (status === "removed") return t("alchemy.inherit.affixPreviewChanceRemovedHint")
  if (status === "fixed") return t("alchemy.inherit.affixPreviewChanceFixedHint")
  if (status === "risk") return t("alchemy.inherit.affixPreviewChanceRiskHint", { chance: formatPercent(probability) })
  return t("alchemy.inherit.affixPreviewChanceValueHint", { chance: formatPercent(probability) })
}

function previewRiskText(status: AffixPreviewStatus, probability: number): string {
  if (status !== "risk") return ""
  return t("alchemy.inherit.affixPreviewMutationChance", { chance: formatPercent(probability) })
}

function clampProbability(value: number): number {
  if (!Number.isFinite(value)) return 0
  return Math.max(0, Math.min(1, value))
}

function sourceListLabel(sources: InheritSourceRow[]): string {
  const labels = Array.from(new Set(sources.map((source) => source.sourceLabel).filter(Boolean)))
  if (labels.length <= 2) return labels.join(" / ") || t("alchemy.inherit.unknownSource")
  return t("alchemy.inherit.affixPreviewSourceCount", { first: labels[0], second: labels[1], count: labels.length - 2 })
}

function sourcePreviewRefs(sources: InheritSourceRow[]): PreviewSourceRef[] {
  const refs = new Map<string, PreviewSourceRef>()
  for (const source of sources) {
    const itemId = String(source.itemId || "").trim()
    if (!itemId) continue
    const key = `${itemId}:${source.sourceLabel || itemId}`
    if (!refs.has(key)) refs.set(key, { key, itemId, label: source.sourceLabel || itemId })
  }
  return Array.from(refs.values())
}

function sourceEffectLabel(sources: InheritSourceRow[]): string {
  const labels = Array.from(new Set(sources.map((source) => source.effectLabel).filter(Boolean)))
  if (labels.length === 1) return labels[0] || ""
  if (labels.length > 1) return t("alchemy.inherit.affixPreviewEffectValue")
  return t("alchemy.inherit.affixPreviewEffectValue")
}

function affixPreviewStatusText(status: AffixPreviewStatus): string {
  if (status === "possible") return t("alchemy.inherit.affixPreviewPossible")
  if (status === "hybrid") return t("alchemy.inherit.affixPreviewHybrid")
  if (status === "risk") return t("alchemy.inherit.affixPreviewRisk")
  if (status === "removed") return t("alchemy.inherit.affixPreviewRemoved")
  return t("alchemy.inherit.affixPreviewFixed")
}

function affixPreviewTagType(status: AffixPreviewStatus): "success" | "warning" | "danger" | "info" {
  if (status === "fixed") return "success"
  if (status === "hybrid") return "success"
  if (status === "risk" || status === "removed") return "danger"
  if (status === "possible") return "warning"
  return "info"
}

function valueTone(value: number): AffixPreviewTone {
  if (value > 0) return "positive"
  if (value < 0) return "negative"
  return "neutral"
}

function mediumLines(medium: AlchemyInheritMedium & { available?: number }): string[] {
  const mods = medium.modifiers
  const rows = [
    `${t("alchemy.inherit.inheritSlots")} ${computeAlchemyInheritEffectiveInheritSlots(ALCHEMY_CONFIG.inherit.baseInheritSlots, playerAlchemyPrecision.value, mods.inheritSlots)}`,
    `${t("alchemy.inherit.suppressSlots")} ${computeAlchemyInheritEffectiveSuppressSlots(ALCHEMY_CONFIG.inherit.baseSuppressSlots, playerAlchemyStability.value, mods.suppressSlots)}`,
    t("alchemy.inherit.positiveIgnoreBase", { chance: formatPercent(computeAlchemyInheritIgnoreKeepChance(medium, 0, 1, playerAlchemyStability.value)) }),
    t("alchemy.inherit.negativeIgnoreBase", { chance: formatPercent(computeAlchemyInheritIgnoreKeepChance(medium, 0, -1, playerAlchemyStability.value)) }),
    t("alchemy.inherit.fusionRuleShort", { divisor: formatNumber(mods.hybridMergeDivisor, 2), risk: formatPercent(mods.inbreedMutationChance) })
  ]
  if (mods.positiveIgnoreKeepChanceBonus > 0) rows.push(t("alchemy.inherit.positiveIgnoreBonus", { chance: formatPercent(mods.positiveIgnoreKeepChanceBonus) }))
  if (mods.negativeIgnoreSuppressChanceBonus > 0) rows.push(t("alchemy.inherit.negativeIgnoreBonus", { chance: formatPercent(mods.negativeIgnoreSuppressChanceBonus) }))
  if (medium.consumePerCraft > 1 || medium.available !== undefined) rows.push(t("alchemy.inherit.consumeMedium", { count: medium.consumePerCraft, owned: medium.available ?? itemQuantity(medium.itemId) }))
  return rows
}

function deriveSourceKey(row: Record<string, unknown>, attr: string, index: number): string {
  const itemId = String(row.itemId || "").trim()
  const affixId = String(row.affixId || "").trim()
  const traitId = String(row.traitId || row.sourceId || "").trim()
  const comboRecipeId = String(row.comboRecipeId || row.sourceId || "").trim()
  const mutationId = String(row.mutationId || row.sourceId || "").trim()
  const sourceType = String(row.sourceType || row.sourceKind || "").trim()
  const suffix = cleanSourceAttr(attr) || "effect"
  if (affixId) return `affix:${affixId}:${suffix}`
  if (sourceType === "trait" || sourceType === "material" || traitId || itemId) return `trait:${itemId}:${traitId || itemId}:${suffix}`
  if (sourceType === "combo" || comboRecipeId) return `combo:${comboRecipeId}:${suffix}`
  if (sourceType === "mutation" || mutationId) return `mutation:${mutationId || sourceType}:${suffix}`
  return `${sourceType || "component"}:${index}:${suffix}`
}

function componentSourceMeta(row: Record<string, unknown>): { label: string; note?: string; effectLabel?: string; itemId?: string; affixId?: string; comboRecipeId?: string } {
  const itemId = String(row.itemId || "").trim()
  const affixId = String(row.affixId || "").trim()
  const comboRecipeId = String(row.comboRecipeId || "").trim()
  const sourceType = String(row.sourceType || row.sourceKind || "component").trim()
  const sourceId = String(row.sourceId || row.traitId || row.comboRecipeId || row.mutationId || row.affixId || "").trim()
  if (itemId) return { label: t(`item.${itemId}.name`, itemId), itemId }
  if (affixId) {
    const def = getAffixDefinition(affixId)
    return { label: def?.nameKey ? t(def.nameKey, affixId) : affixId, affixId, note: t("alchemy.inherit.orphanSourceShort") }
  }
  if (comboRecipeId) {
    const combo = comboById.get(comboRecipeId)
    return { label: combo ? t(combo.nameKey, comboRecipeId) : comboRecipeId, note: t("alchemy.inherit.comboOutputSource"), effectLabel: t("alchemy.inherit.comboEffectSource"), comboRecipeId }
  }
  if (sourceType === "combo" && sourceId) {
    const combo = comboById.get(sourceId)
    return { label: combo ? t(combo.nameKey, sourceId) : sourceId, note: t("alchemy.inherit.comboOutputSource"), effectLabel: t("alchemy.inherit.comboEffectSource"), comboRecipeId: sourceId }
  }
  if (sourceType === "mutation") return { label: t("alchemy.inherit.mutationSource"), note: sourceId }
  return { label: sourceId ? `${sourceType}:${sourceId}` : sourceType || t("alchemy.inherit.unknownSource") }
}

function relationTagType(relation: SourceMatrixRow["relation"]): "danger" | "success" | "info" {
  if (relation === "inbreed") return "danger"
  if (relation === "hybrid") return "success"
  return "info"
}

function relationLabel(relation: SourceMatrixRow["relation"]): string {
  if (relation === "inbreed") return t("alchemy.inherit.badgeInbreed")
  if (relation === "hybrid") return t("alchemy.inherit.badgeHybrid")
  return t("alchemy.inherit.badgeSingle")
}

function relationHint(relation: SourceMatrixRow["relation"]): string {
  if (relation === "inbreed") return t("alchemy.inherit.inbreedHint")
  if (relation === "hybrid") return t("alchemy.inherit.hybridHint")
  return t("alchemy.inherit.ignoreHint")
}

function instanceLabel(item: InventoryInstance): string {
  return t(`item.${item.templateItemId}.name`, item.templateItemId)
}

function affixLabel(affix: EquipmentAffixSlot, index: number): string {
  const key = String(affix.displayNameKey || "").trim()
  const def = getAffixDefinition(String(affix.affixId || "").trim())
  const name = key ? t(key, buildAffixDisplayNameArgs(affix.displayNameArgs)) : def?.nameKey ? t(def.nameKey, affix.affixId) : t("alchemy.inherit.affixIndex", { index: index + 1 })
  const kind = affixInheritKind(affix)
  const orphan = isCustomAffix(affix) && (affix.sourceKind === "inherit_orphan" || Boolean(affix.effectParams?.inheritOrphan))
  return `${name}${kind ? ` · ${t(`alchemy.kind.${kind}`, kind)}` : ""}${orphan ? ` · ${t("alchemy.inherit.orphanAffix")}` : ""}`
}

function buildAffixDisplayNameArgs(raw: Record<string, unknown> | undefined): Record<string, string> {
  const out: Record<string, string> = {}
  if (!raw || typeof raw !== "object" || Array.isArray(raw)) return out
  for (const [key, value] of Object.entries(raw)) {
    if (key.endsWith("Key") && typeof value === "string") {
      out[key.slice(0, -"Key".length)] = localizeLooseKey(value)
      continue
    }
    if (key.endsWith("Keys") && Array.isArray(value)) {
      out[key.slice(0, -"Keys".length)] = value.map((entry) => localizeLooseKey(String(entry || ""))).filter(Boolean).join(t("alchemy.affix.prefixSeparator"))
      continue
    }
    out[key] = String(value ?? "")
  }
  return out
}

function localizeLooseKey(key: string): string {
  const normalized = String(key || "").trim()
  if (!normalized) return ""
  const localized = t(normalized)
  return localized === normalized ? normalized : localized
}

function attrLabel(attr: string): string {
  attr = cleanSourceAttr(attr)
  if (!attr) return t("alchemy.inherit.effectSource")
  const def = getAttributeDefinition(attr)
  return def?.nameKey ? t(def.nameKey, attr) : attr
}

function cleanSourceAttr(attr: string): string {
  const normalized = String(attr || "").trim()
  if (!normalized || normalized === "<nil>" || normalized === "<null>" || normalized.toLowerCase() === "nil" || normalized.toLowerCase() === "null") return ""
  return normalized
}

function cleanSourceKey(sourceKey: string): string {
  const normalized = String(sourceKey || "").trim()
  if (normalized.endsWith(":<nil>") || normalized.endsWith(":nil") || normalized.endsWith(":<null>") || normalized.endsWith(":null")) {
    const parts = normalized.split(":")
    parts[parts.length - 1] = "effect"
    return parts.join(":")
  }
  return normalized
}

function sourceParentLabel(parent: ParentSide, _item: InventoryInstance | null): string {
  return parent === "father" ? t("alchemy.inherit.fromLeftSource") : t("alchemy.inherit.fromRightSource")
}

function isCustomAffix(affix: EquipmentAffixSlot | null): boolean {
  return !!affix && String(affix.affixId || "").startsWith("custom:")
}

function affixInheritKind(affix: EquipmentAffixSlot | null): string {
  if (!affix) return ""
  if (isCustomAffix(affix)) return String(affix.effectParams?.alchemyKind || "").trim()
  const def = getAffixDefinition(String(affix.affixId || "").trim())
  const type = String(def?.type || "").trim()
  if (type === "battle" || type === "combat") return "combat"
  if (type === "work" || type === "life") return "life"
  return ""
}

function resolveAffixParamsForSlot(effectParams: Record<string, unknown> | undefined, rarityId: string): Record<string, unknown> {
  const out: Record<string, unknown> = {}
  if (!effectParams) return out
  for (const [key, value] of Object.entries(effectParams)) {
    out[key] = selectRarityValue(value, rarityId)
  }
  return out
}

function selectRarityValue(value: unknown, rarityId: string): unknown {
  if (!value || typeof value !== "object" || Array.isArray(value)) return value
  const node = value as Record<string, unknown>
  if (rarityId && node[rarityId] !== undefined) return node[rarityId]
  if (node.default !== undefined) return node.default
  if (node["*"] !== undefined) return node["*"]
  return value
}

function ordinaryAttrRange(params: Record<string, unknown>, attr: string, fallback: number): { min: number; max: number } {
  const attrMods = params.attrMods
  if (!attrMods || typeof attrMods !== "object" || Array.isArray(attrMods)) return { min: fallback, max: fallback }
  const raw = (attrMods as Record<string, unknown>)[attr]
  if (raw && typeof raw === "object" && !Array.isArray(raw)) {
    const node = raw as Record<string, unknown>
    return { min: numberValue(node.min, fallback), max: numberValue(node.max, fallback) }
  }
  return { min: numberValue(raw, fallback), max: numberValue(raw, fallback) }
}

function hasComplexEffectParams(params: Record<string, unknown>): boolean {
  return Object.keys(params).some((key) => !["attr", "attrMods"].includes(key))
}

function formatNumber(value: number, digits = 0): string {
  if (!Number.isFinite(value)) return "0"
  return value.toFixed(digits).replace(/\.0+$/, "").replace(/(\.\d*?)0+$/, "$1")
}

function formatPercent(value: number): string {
  if (!Number.isFinite(value)) return "0%"
  return `${(value * 100).toFixed(1)}%`
}

function ignoreKeepChanceForSource(source: { attr: string; currentValue: number }): number {
  return computeAlchemyInheritIgnoreKeepChance(selectedMedium.value, 0, source.attr ? source.currentValue : null, playerAlchemyStability.value)
}

function numberValue(raw: unknown, fallback = 0): number {
  const value = Number(raw)
  return Number.isFinite(value) ? value : fallback
}

function stringArray(raw: unknown): string[] {
  if (!Array.isArray(raw)) return []
  return raw.map((value) => String(value || "").trim()).filter(Boolean)
}

function uniqueStrings(values: string[]): string[] {
  const seen = new Set<string>()
  const out: string[] = []
  for (const value of values) {
    const normalized = String(value || "").trim()
    if (!normalized || seen.has(normalized)) continue
    seen.add(normalized)
    out.push(normalized)
  }
  return out
}
</script>

<style scoped>
.alchemy-inherit-panel {
  display: grid;
  gap: 14px;
  font-size: 13px;
  overflow: visible;
}

.inherit-workbench {
  display: grid;
  grid-template-columns: minmax(0, 1fr);
  gap: 12px;
  align-items: start;
}

.inherit-setup-grid {
  min-width: 0;
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 8px;
}

.inherit-picker {
  min-width: 0;
  display: grid;
  grid-template-columns: auto minmax(0, 1fr);
  grid-template-areas:
    "title title"
    "card affixes"
    "hint hint";
  align-content: start;
  align-items: start;
  gap: 6px 10px;
  padding: 8px 10px;
  border: 1px solid var(--app-border, #dde4ea);
  border-radius: 6px;
  background: var(--app-surface, #fff);
}

.inherit-picker--source {
  grid-template-columns: max-content minmax(0, 1fr);
}

.inherit-picker--medium {
  grid-template-columns: max-content minmax(0, 1fr);
}

.inherit-picker__title {
  grid-area: title;
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 12px;
  font-weight: 800;
  color: var(--app-muted, #667085);
}

.inherit-step {
  width: 18px;
  height: 18px;
  display: inline-grid;
  place-items: center;
  border-radius: 50%;
  background: color-mix(in srgb, var(--el-color-primary, #409eff) 12%, transparent);
  color: var(--el-color-primary, #409eff);
  font-size: 11px;
  font-weight: 900;
}

.inherit-picker__hint {
  grid-area: hint;
  color: var(--app-muted, #667085);
  font-size: 12px;
  line-height: 1.35;
}

.inherit-instance {
  grid-area: card;
  min-width: 0;
  display: flex;
  align-items: center;
  gap: 8px;
  border: 0;
  padding: 0;
  background: transparent;
  color: inherit;
  text-align: left;
  cursor: pointer;
}

.inherit-instance > span {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.inherit-affix-choice-list {
  grid-area: affixes;
  min-width: 0;
  display: flex;
  flex-wrap: wrap;
  align-content: start;
  align-items: flex-start;
  justify-content: flex-start;
  gap: 5px;
  padding-top: 2px;
}

.inherit-affix-choice {
  min-width: 0;
  display: inline-flex;
  justify-content: start;
  max-width: 100%;
  padding: 0;
  border: 0;
  border-radius: 4px;
  background: transparent;
  color: inherit;
  cursor: pointer;
  text-align: left;
}

.inherit-affix-choice.is-active {
  outline: 2px solid color-mix(in srgb, var(--el-color-primary, #409eff) 35%, transparent);
  outline-offset: 1px;
  background: transparent;
}

.inherit-affix-choice :deep(.affix-quick-ref) {
  min-width: 0;
}

.inherit-affix-choice :deep(.affix-quick-ref-tag) {
  max-width: 100%;
  background: var(--app-surface, #fff);
}

.inherit-affix-choice :deep(.affix-quick-ref-content) {
  max-width: 100%;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.inherit-medium-slot {
  grid-area: card;
  display: flex;
  align-items: flex-start;
}

.inherit-inline-button {
  grid-area: affixes;
  width: fit-content;
}

.inherit-medium-detail {
  grid-area: affixes;
  min-width: 0;
  display: grid;
  gap: 7px;
  padding: 2px 0;
  border: 0;
  background: var(--app-surface, #fff);
}

.inherit-medium-detail__head {
  min-width: 0;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  padding: 0;
}

.inherit-medium-detail__head strong {
  min-width: 0;
  font-size: 13px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.inherit-medium-kv {
  display: flex;
  flex-wrap: wrap;
  gap: 5px;
  margin: 0;
}

.inherit-medium-kv__row {
  min-width: 0;
  display: inline-flex;
  align-items: baseline;
  gap: 5px;
  padding: 4px 7px;
  border-radius: 999px;
  background: color-mix(in srgb, var(--app-bg, #f6f8fb) 72%, var(--app-surface, #fff));
}

.inherit-medium-kv dt {
  min-width: 0;
  color: var(--app-muted, #667085);
  font-size: 10px;
  font-weight: 700;
  white-space: nowrap;
}

.inherit-medium-kv dd {
  margin: 0;
  color: var(--app-text, #1f2328);
  font-size: 12px;
  font-weight: 800;
  white-space: nowrap;
}

.inherit-medium-desc {
  color: var(--app-muted, #667085);
  font-size: 11px;
  line-height: 1.35;
  overflow: hidden;
  display: -webkit-box;
  -webkit-box-orient: vertical;
  -webkit-line-clamp: 2;
}

.inherit-medium-tags {
  display: flex;
  flex-wrap: wrap;
  gap: 5px;
}

.inherit-medium-codex {
  display: grid;
  gap: 10px;
}

.inherit-medium-codex__item {
  min-width: 0;
  display: grid;
  grid-template-columns: auto minmax(0, 1fr);
  gap: 10px;
  align-items: start;
  padding: 10px;
  border: 1px solid var(--app-border, #dde4ea);
  border-radius: 6px;
  background: var(--app-surface, #fff);
}

.inherit-medium-codex__item strong {
  display: block;
  margin-bottom: 3px;
  font-size: 13px;
}

.inherit-medium-codex__item p {
  margin: 0 0 6px;
  color: var(--app-muted, #667085);
  font-size: 12px;
  line-height: 1.35;
}

.inherit-submit {
  display: flex;
  align-items: center;
  justify-content: flex-start;
  gap: 12px;
  padding-top: 2px;
}

.inherit-submit__meta {
  display: grid;
  justify-items: start;
  gap: 1px;
  font-size: 12px;
}

.inherit-submit__meta span {
  color: var(--app-muted, #667085);
  font-weight: 700;
}

.inherit-submit__meta strong {
  font-size: 14px;
}

.alchemy-inherit-issues {
  padding: 10px 12px;
  border: 1px solid color-mix(in srgb, var(--el-color-warning, #e6a23c) 40%, transparent);
  border-radius: 6px;
  background: color-mix(in srgb, var(--el-color-warning, #e6a23c) 8%, transparent);
}

.alchemy-inherit-issues__title {
  color: var(--el-color-warning, #e6a23c);
  font-size: 12px;
  font-weight: 800;
}

.alchemy-inherit-issues ul {
  margin: 6px 0 0;
  padding-left: 18px;
  color: var(--app-muted, #667085);
  font-size: 12px;
}

@media (max-width: 1100px) {
  .inherit-workbench {
    grid-template-columns: 1fr;
  }
}

@media (max-width: 700px) {
  .inherit-setup-grid {
    grid-template-columns: 1fr;
  }

  .inherit-picker {
    grid-template-columns: auto minmax(0, 1fr);
  }
}
</style>
