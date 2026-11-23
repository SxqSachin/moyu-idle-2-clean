<template>
  <div class="item-popover-content" :class="fixHeight ? 'item-popover-content--fix-height' : ''" v-loading="loading">
    <div class="item-popover-rarity-mask" :style="rarityGradientStyle" />
    <div class="item-popover-header">
      <FishMutationArt v-if="fishIconPath" class="item-popover-fish-icon" :src="fishIconPath" :alt="itemName" :fish="fishData" width="56px" />
      <TypeIcon v-else :type="iconType" :id="iconID" :alt="itemName" :size="store.playerSimpleMode ? 48 : 52" :fallback="itemFallbackIcon" />
      <div class="item-popover-title-wrap">
        <div class="item-popover-title" :class="customItemLabelClass">{{ itemName }}</div>
        <div class="item-popover-subtitle">
          <RarityText :rarity-id="itemRarity" /> · {{ itemTypeLabel }}
          <template v-if="equipmentSlotLabel"> ·
            {{ equipmentSlotLabel }}
          </template>
          <template v-if="equipmentWeaponType && equipmentWeaponType.length > 0"> 
            · {{ equipmentWeaponType.map(type => t(`weapon_class.${type}.name`)).join(', ') }}
          </template>
        </div>
      </div>
    </div>

    <div class="item-popover-inventory">
      {{ t('game.surface.inventory') }}: {{ inventoryQuantity  }}
    </div>

    <div v-if="fishData" class="item-popover-fish-summary">
      <p v-if="fishSaleNotice" class="item-popover-fish-sale-status">
        <Icon icon="lucide:hourglass" />
        <span>{{ fishSaleNotice }}</span>
      </p>
      <div v-if="fishMutationNames.length" class="item-popover-fish-mutations">
        <span v-for="mutation in fishMutationNames" :key="mutation.id"><Icon icon="lucide:sparkles" />{{ mutation.name }}</span>
      </div>
      <div><span>{{ t("game.fishing.length") }}</span><strong>{{ formatFishLength(fishData.lengthCm) }}<FishSizeIndicator metric="length" :value="fishData.lengthCm" :species="fishSpecies" /></strong></div>
      <div><span>{{ t("game.fishing.weight") }}</span><strong>{{ formatFishWeight(fishData.weightKg) }}<FishSizeIndicator metric="weight" :value="fishData.weightKg" :species="fishSpecies" /></strong></div>
      <div><span>{{ t("game.fishing.age") }}</span><strong>{{ formatFishAge(fishData.ageDays) }}</strong></div>
      <div><span>{{ t("game.fishing.sex") }}</span><strong>{{ t(`game.fishing.sex_${fishData.sex}`) }}</strong></div>
      <div><span>{{ t("game.fishing.bodyCondition") }}</span><strong>{{ formatFishCondition(fishData.condition) }}</strong></div>
      <div><span>{{ t("game.fishing.health") }}</span><strong>{{ formatFishHealth(fishData.health) }}</strong></div>
      <p><Icon icon="lucide:map-pin" />{{ fishOrigin }}</p>
      <p><Icon icon="lucide:clock-3" />{{ formatFishCaughtAt(fishData.caughtAt) }}</p>
    </div>

    <FishingTackleStaticInfo
      v-if="isFishingTackle"
      :item-id="props.itemId"
      :effective-attrs="equippedEffectAttrs"
    />

    <div class="item-popover-row" v-if="customItemName">
      <div class="item-popover-label">{{ t("game.itemOriginalName") }}</div>
      <div class="item-popover-value">{{ originalItemName }}</div>
    </div>

    <!-- <div class="item-popover-row">
      <div class="item-popover-label">{{ t("game.itemDetailDescription") }}</div>
    </div> -->

    <div class="item-popover-row" v-if="itemEffect.length && !collectibleBonusEntries.length && !isFishCuisine">
      <div class="item-popover-label">{{ t("game.itemDetailEffect") }}</div>
      <div class="item-popover-value">{{ itemEffect }}</div>
    </div>
    <div class="item-popover-row" v-if="itemDescription.length">
      <div class="item-popover-label">{{ t("game.itemDetailDescription") }}</div>
      <div class="item-popover-value">{{ itemDescription }}</div>
    </div>
    <FishFoodInstanceInfo :item-id="itemId" :base-snapshot="instance?.baseSnapshot" :authoritative-effect="detail?.operationEffect" />
    <BattleUsableItemStaticInfo :battle-usable-item="battleUsableItemDefinition" />
    <div class="item-popover-row" v-if="skillbookGrantSkillID">
      <div class="item-popover-label">{{ t("game.itemDetailSkillbookGrantSkill") }}</div>
      <div class="item-popover-value">
        <SkillQuickRef :skill-id="skillbookGrantSkillID" :show-icon="true" />
      </div>
    </div>
    <div class="item-popover-row" v-if="collectibleBonusEntries.length">
      <div class="item-popover-label">{{ t("game.itemDetailEffect") }}</div>
      <div class="item-popover-value collectible-bonus-list">
        <div v-if="collectiblePlayerBonusEntries.length" class="collectible-bonus-group">
          <div class="item-popover-sublabel">{{ t("collectible.playerBonus") }}</div>
          <div class="status-require-list">
            <StatusTag
              v-for="entry in collectiblePlayerBonusEntries"
              :key="`collectible-player-${entry.attrKey}`"
              :attr-key="entry.attrKey"
              :value="entry.value"
              :signed="true"
            />
          </div>
        </div>
        <div v-if="collectibleKittyBonusEntries.length" class="collectible-bonus-group">
          <div class="item-popover-sublabel">{{ t("collectible.kittyBonus") }}</div>
          <div class="status-require-list">
            <StatusTag
              v-for="entry in collectibleKittyBonusEntries"
              :key="`collectible-kitty-${entry.attrKey}`"
              :attr-key="entry.attrKey"
              :value="entry.value"
              :signed="true"
            />
          </div>
        </div>
      </div>
    </div>

    <!-- <div class="item-popover-row" v-if="visibleEquipmentScore">
      <div class="item-popover-label">价值评分</div>
      <div class="item-popover-value">
        <div>{{ visibleEquipmentScore.score }} 分 / {{ visibleEquipmentScore.goldValue }} 金</div>
        <div v-if="equipmentInstanceScore" class="item-popover-score-breakdown">
          快照 {{ equipmentInstanceScore.detail.snapshotAttrs }} · 强化 {{ equipmentInstanceScore.detail.enhanceAttrs }} · 词条 {{ equipmentInstanceScore.detail.affixInstances }}
        </div>
        <div v-if="equipmentScoreBreakdown.length" class="item-popover-score-breakdown">
          {{ equipmentScoreBreakdown.join(" · ") }}
        </div>
      </div>
    </div> -->

    <EquipmentStaticInfo
      :equipment-definition="equipmentDefinition"
      :equipped-effect-attrs="equippedEffectAttrs"
      :excluded-attr-keys="isFishingTackle ? fishingTackleAttributeKeys : []"
      :requirement-attributes="requirementAttributes"
      :enhance-level="enhanceLevel"
    />
    <template v-if="hasEnhanceInfo">
      <div class="item-popover-row detail-surface grid grid-cols-2 gap-x-4 gap-y-2">
        <div>
          <div class="item-popover-label">{{ t("game.itemDetailEnhance") }}</div>
          <div class="item-popover-value">
            <div>{{ t("game.enhanceLevel") }}: +{{ enhanceLevel }}</div>
          </div>
        </div>

        <div>
          <div class="item-popover-label">
          {{ t("game.itemDetailEnhanceBonus") }}:
          </div>
          <div class="item-popover-value">
            <div v-if="enhanceBonusTags.length" class="status-require-list">
              <StatusTag
                v-for="entry in enhanceBonusTags"
                :key="`enhance-bonus-${entry.attrKey}`"
                :attr-key="entry.attrKey"
                :value="entry.value"
              />
            </div>
          </div>
        </div>
      </div>
    </template>
    <div v-if="affixSlotSummary.length || (isEquipment && affixSlotCount > 0)" class="item-popover-row detail-surface">
      <div class="item-popover-label detail-section-title">{{ t("game.itemDetailAffixSlots") }}</div>
      <div class="item-popover-value">
        <div v-if="showingTemplateAffixSlots" class="item-popover-tip">
          {{
            t("game.itemDetailTemplateDefaultAffixHint", {
              defaultCount: templateDefaultAffixCount,
              totalCount: affixSlotCount
            })
          }}
        </div>
        <template v-if="affixSlotSummary.length">
          <div class="item-popover-slot-list">
            <div v-for="slot in affixSlotSummary" :key="`affix-slot-${slot.slotIndex}`">
              <template v-if="slot.affixId">
                <div class="item-popover-affix-entry">
                  <AffixQuickRef
                    :affix-id="slot.affixId"
                    :slot="slot"
                    :show-instance-details="!isTemplateRandomDefaultAffixSlot(slot)"
                    :prefix-label="`#${slot.slotIndex + 1} `"
                    :template-rarity-weights="templateRandomDefaultAffixRarityWeights(slot)"
                  />
                  <div v-if="slotSourceBadges(slot).length" class="item-popover-source-badges">
                    <span
                      v-for="badge in slotSourceBadges(slot)"
                      :key="`${slot.slotIndex}-${badge.kind}`"
                      class="item-popover-source-badge"
                      :class="`is-${badge.kind}`"
                    >
                      <Icon :icon="badge.icon" />
                      <span>{{ badge.title }}</span>
                    </span>
                  </div>
                </div>
              </template>
              <template v-else>
                <AffixEmptyRef
                  :prefix-label="`#${slot.slotIndex + 1} ` + t('game.itemDetailAffixEmpty')"
                />
              </template>
            </div>
          </div>
        </template>
      </div>
    </div>
    <template v-else>
      <div v-if="isEquipment && (templateAffixSlotCount ?? 0) > 0" class="item-popover-row detail-surface">
        <div class="item-popover-label detail-section-title">{{ t("game.itemDetailAffixSlots") }}</div>
        <div class="item-popover-value">
          {{ t("game.itemDetailAffixSlotCount", { count: templateAffixSlotCount }) }}
        </div>
      </div>
    </template>
    <!-- <InstanceInfo v-if="instance" :instance="instance"></InstanceInfo> -->

    <div class="item-popover-flow-grid" v-if="insightUses.length || hasInsightProductionSources">
      <div class="item-popover-flow-block" v-if="insightUses.length">
        <div class="item-popover-label">{{ t("game.itemTooltip.usedIn") }}</div>
        <div class="item-popover-value item-popover-action-ref-list">
          <template v-if="insightUses.length">
            <ActionQuickRef
              v-for="flow in insightUses"
              :key="`used-in-${props.itemId}-${flow.actionId}`"
              :action-id="flow.actionId"
            />
          </template>
          <template v-else>{{ t("game.itemTooltip.none") }}</template>
        </div>
      </div>
      <div class="item-popover-flow-block" v-if="hasInsightProductionSources">
        <div class="item-popover-label">{{ t("game.itemTooltip.sourcedFrom") }}</div>
        <template v-if="insightSources.length">
          <div class="item-popover-sublabel">{{ t("game.itemTooltip.sourcedFromActions") }}</div>
          <div class="item-popover-value item-popover-action-ref-list">
            <ActionQuickRef
              v-for="flow in visibleInsightSources"
              :key="`sourced-from-${props.itemId}-${flow.actionId}`"
              :action-id="flow.actionId"
            />
          </div>
          <button v-if="insightSources.length > INSIGHT_COLLAPSE_LIMIT" class="item-popover-expand-btn" @click="sourcesExpanded = !sourcesExpanded">
            {{ sourcesExpanded ? t("game.itemTooltip.collapse") : t("game.itemTooltip.showMore", { count: insightSources.length }) }}
          </button>
        </template>
        <template v-if="insightEnemySources.length">
          <div class="item-popover-sublabel">{{ t("game.itemTooltip.sourcedFromEnemies") }}</div>
          <div class="item-popover-value item-popover-enemy-drop-list">
            <div
              v-for="drop in visibleInsightEnemySources"
              :key="`sourced-from-enemy-${props.itemId}-${drop.enemyId}-${drop.min}-${drop.max}-${drop.percent}`"
              class="item-popover-enemy-drop-row"
            >
              <EnemyUnitQuickRef :enemy-id="drop.enemyId" />
              <span class="item-popover-enemy-drop-detail">
                {{ t("game.battleEnemyDropRate", { percent: formatInsightPercent(drop.percent) }) }}
                ·
                {{ t("game.battleEnemyDropRange", { min: drop.min, max: drop.max }) }}
              </span>
            </div>
          </div>
          <button v-if="insightEnemySources.length > INSIGHT_COLLAPSE_LIMIT" class="item-popover-expand-btn" @click="enemySourcesExpanded = !enemySourcesExpanded">
            {{ enemySourcesExpanded ? t("game.itemTooltip.collapse") : t("game.itemTooltip.showMore", { count: insightEnemySources.length }) }}
          </button>
        </template>
        <template v-if="insightCatVillageCommissionSources.length">
          <div class="item-popover-sublabel">{{ t("game.itemTooltip.sourcedFromCatVillageCommissions") }}</div>
          <div class="item-popover-value item-popover-named-source-list">
            <span
              v-for="source in visibleInsightCatVillageCommissionSources"
              :key="`sourced-from-cat-village-${props.itemId}-${source.id}`"
              class="item-popover-named-source-chip"
            >
              {{ localizeNamedSource(source) }}
            </span>
          </div>
          <button v-if="insightCatVillageCommissionSources.length > INSIGHT_COLLAPSE_LIMIT" class="item-popover-expand-btn" @click="catVillageSourcesExpanded = !catVillageSourcesExpanded">
            {{ catVillageSourcesExpanded ? t("game.itemTooltip.collapse") : t("game.itemTooltip.showMore", { count: insightCatVillageCommissionSources.length }) }}
          </button>
        </template>
        <template v-if="insightCatteryCommissionSources.length">
          <div class="item-popover-sublabel">{{ t("game.itemTooltip.sourcedFromCatteryCommissions") }}</div>
          <div class="item-popover-value item-popover-named-source-list">
            <span
              v-for="source in visibleInsightCatteryCommissionSources"
              :key="`sourced-from-cattery-${props.itemId}-${source.id}`"
              class="item-popover-named-source-chip"
            >
              {{ localizeNamedSource(source) }}
            </span>
          </div>
          <button v-if="insightCatteryCommissionSources.length > INSIGHT_COLLAPSE_LIMIT" class="item-popover-expand-btn" @click="catterySourcesExpanded = !catterySourcesExpanded">
            {{ catterySourcesExpanded ? t("game.itemTooltip.collapse") : t("game.itemTooltip.showMore", { count: insightCatteryCommissionSources.length }) }}
          </button>
        </template>
        <template v-if="insightKittyJournalSources.length">
          <div class="item-popover-sublabel">{{ t("game.itemTooltip.sourcedFromKittyJournal") }}</div>
          <div class="item-popover-value item-popover-named-source-list">
            <span
              v-for="source in visibleInsightKittyJournalSources"
              :key="`sourced-from-kitty-journal-${props.itemId}-${source.id}`"
              class="item-popover-named-source-chip"
            >
              {{ localizeNamedSource(source) }}
            </span>
          </div>
          <button v-if="insightKittyJournalSources.length > INSIGHT_COLLAPSE_LIMIT" class="item-popover-expand-btn" @click="kittyJournalSourcesExpanded = !kittyJournalSourcesExpanded">
            {{ kittyJournalSourcesExpanded ? t("game.itemTooltip.collapse") : t("game.itemTooltip.showMore", { count: insightKittyJournalSources.length }) }}
          </button>
        </template>
        <div v-if="!hasInsightProductionSources" class="item-popover-value">
          {{ t("game.itemTooltip.none") }}
        </div>
      </div>
    </div>

    <div v-if="loadError" class="item-popover-error">{{ loadError }}</div>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, watch } from "vue"
import { Icon } from "@iconify/vue"
import { useI18n } from "vue-i18n"
import { getGameDataChunk } from "@services/game-data-runtime"
import { APIClientError } from "@services/api"
import {
  getBattleUsableItemDefinitionByItemID,
  getEquipmentItemDefinition,
  getItemDefinition,
  getRarityDefinition
} from "@services/item-definition"
import { getActionDefinition, isGenericActionVisible, listActionDefinitions } from "@services/action-definition"
import {
  aggregateEquipmentAttrChanges,
  buildAffixSlotSummary
} from "@services/equipment-affix-display"
import { getLocalizedItemName } from "@services/item-label"
import { getItemInsightMap, type ItemInsight, type ItemNamedSourceRef } from "@services/item-insights"
import { parseIconId, resolveIconIdByItemId, resolveItemFallbackIcon } from "@services/item-icons"
import { resolveErrorMessage } from "@services/error-messages"
import { fishMutationIds } from "@services/fish-mutations"
import { getAffixSlotSourceBadgeDefinitions } from "@services/affix-source-badges"
import { itemCustomLabelClass, itemCustomName } from "@services/item-customization"
import { getLocalizedRuntimeSkillName } from "@services/skill-definition"
import {
  getFishSpeciesDisplay,
  getFishingSpotDisplay,
  getFishingWaterDisplay,
  localizedFishText
} from "@services/fish-instance-display"
import { useGameStore } from "@stores/game"
import { useItemInstanceDetailStore } from "@stores/item-instance-detail"
import type { ItemPopoverDialogPublicInstancePreview } from "@stores/item-popover-dialog"
import TypeIcon from "@components/game/icon/TypeIcon.vue"
import StatusTag from "@components/game/attribute/StatusTag.vue"
import AffixQuickRef from "@components/game/affix/AffixQuickRef.vue"
import RarityText from "@components/game/common/RarityText.vue"
import ActionQuickRef from "@components/game/action/ActionQuickRef.vue"
import SkillQuickRef from "@components/game/skill/SkillQuickRef.vue"
import EnemyUnitQuickRef from "@components/game/combat/EnemyUnitQuickRef.vue"
import type { EquipmentAffixSlot, ItemDetailResponse } from "@/types/equipment"
import AffixEmptyRef from "../affix/AffixEmptyRef.vue"
import EquipmentStaticInfo from "./EquipmentStaticInfo.vue"
import BattleUsableItemStaticInfo from "./BattleUsableItemStaticInfo.vue"
import FishingTackleStaticInfo from "@components/game/fishing/FishingTackleStaticInfo.vue"
import FishMutationArt from "@components/game/fishing/FishMutationArt.vue"
import FishSizeIndicator from "@components/game/fishing/FishSizeIndicator.vue"
import FishFoodInstanceInfo from "./FishFoodInstanceInfo.vue"
import { listFishFoodRecipes } from "@services/fish-food"
import { useInventorySummary } from "@/composables/game/useInventorySummary"
import { scoreEquipmentDefinition, scoreEquipmentInstance } from "@services/combat-scoring"

const { getInventoryQuantity } = useInventorySummary();

type CollectibleDefinition = {
  id: string
  itemId: string
  playerMods?: Record<string, number>
  kittyMods?: Record<string, number>
}

type ModEntry = {
  attrKey: string
  value: number
}

function emptyItemInsight(): ItemInsight {
  return {
    uses: [],
    sources: [],
    enemySources: [],
    catVillageCommissionSources: [],
    catteryCommissionSources: [],
    kittyJournalSources: []
  }
}

let collectibleById: Map<string, CollectibleDefinition> | null = null
let collectibleByItemId: Map<string, CollectibleDefinition> | null = null

function ensureCollectibleMaps() {
  if (collectibleById && collectibleByItemId) return
  const collectiblesChunk = getGameDataChunk("collectibles")
  const collectibleDefinitions = ((collectiblesChunk as { collectibles?: CollectibleDefinition[] }).collectibles || [])
  collectibleById = new Map(collectibleDefinitions.map((entry) => [entry.id, entry]))
  collectibleByItemId = new Map(collectibleDefinitions.map((entry) => [entry.itemId, entry]))
}

const inventoryQuantity = computed(() => getInventoryQuantity(props.itemId))
const isFishCuisine = computed(() => listFishFoodRecipes().some(row => row.outputItemId === props.itemId))

const props = withDefaults(
  defineProps<{
    itemId: string
    instanceId?: number | null
    publicInstancePreview?: ItemPopoverDialogPublicInstancePreview | null
    requirementAttributes?: Record<string, number> | null
    active?: boolean
    fixHeight?: boolean
    displayNameOverride?: string
  }>(),
  {
    instanceId: null,
    publicInstancePreview: null,
    requirementAttributes: null,
    active: false,
    fixHeight: false,
    displayNameOverride: "",
  }
)

const { t, locale } = useI18n()
const store = useGameStore()
const instanceDetailStore = useItemInstanceDetailStore()
const loading = ref(false)
const loadError = ref("")
const insightMap = ref<Record<string, ItemInsight>>({})
const detail = ref<ItemDetailResponse | null>(null)
const loadedDetailKey = ref("")
const loadedInsight = ref(false)

const definition = computed(() => getItemDefinition(props.itemId))
const battleUsableItemDefinition = computed(() => getBattleUsableItemDefinitionByItemID(props.itemId))
const fishingTackleAttributeKeys = [
  "fishing_control",
  "fishing_sensitivity",
  "fishing_efficiency",
  "fishing_power",
  "fishing_casting",
  "fishing_hooking"
]

const equipmentDefinition = computed(() => getEquipmentItemDefinition(props.itemId))
const isEquipment = computed(() => detail.value?.item?.isEquipment ?? definition.value?.isEquipment ?? false)
const equipmentScore = computed(() => equipmentDefinition.value ? scoreEquipmentDefinition(equipmentDefinition.value as unknown as Record<string, unknown>) : null)
const equipmentInstanceScore = computed(() => {
  if (!equipmentDefinition.value || !instance.value) return null
  return scoreEquipmentInstance(equipmentDefinition.value as unknown as Record<string, unknown>, {
    snapshotAttrs: resolveInstanceSnapshotAttrs(),
    enhanceAttrs: detail.value?.enhance?.cumulativeAttrs || {},
    affixSlots: affixSlots.value as unknown as Array<Record<string, unknown>>
  })
})
const visibleEquipmentScore = computed(() => (equipmentInstanceScore.value || equipmentScore.value))
const equipmentScoreBreakdown = computed(() => {
  return (equipmentScore.value?.sections || [])
    .slice(0, 2)
    .map((section) => `${section.label} ${section.score}分`)
})

const iconInfo = computed(() => parseIconId(resolveIconIdByItemId(props.itemId)))
const iconType = computed(() => iconInfo.value?.type || "item")
const iconID = computed(() => iconInfo.value?.id || props.itemId)
const itemFallbackIcon = computed(() => resolveItemFallbackIcon(props.itemId))
const fishData = computed(() => detail.value?.instance?.fishData || props.publicInstancePreview?.fishData || null)
const fishMutationNames = computed(() => fishMutationIds(fishData.value).map((id) => {
  const key = `game.fishing.mutation_${id}`
  const localized = t(key)
  return { id, name: localized === key ? id : localized }
}))
const fishSaleNotice = computed(() => {
  const preview = props.publicInstancePreview
  if (!fishData.value || preview?.saleEligible !== false) return ""
  if (preview.saleIneligibleReason === "cultured_immature") {
    return t("game.fishing.saleIneligibleCulturedImmature", {
      current: Math.max(0, Number(preview.saleCurrentAgeDays ?? fishData.value.ageDays ?? 0)),
      required: Math.max(0, Number(preview.saleRequiredAgeDays || 0))
    })
  }
  if (preview.saleIneligibleReason === "dead") return t("game.fishing.saleIneligibleDead")
  return t("game.fishing.saleIneligibleInvalid")
})
const fishSpecies = computed(() => getFishSpeciesDisplay(fishData.value?.speciesId || ""))
const fishIconPath = computed(() => fishSpecies.value?.iconPath || "")
const fishSpeciesName = computed(() => localizedFishText(fishSpecies.value?.names, locale.value, ""))
const fishOrigin = computed(() => {
  const spot = getFishingSpotDisplay(fishData.value?.spotId || "")
  const water = getFishingWaterDisplay(fishData.value?.waterId || "")
  const spotName = localizedFishText(spot?.names, locale.value, "")
  const waterName = localizedFishText(water?.names, locale.value, "")
  return [waterName, spotName].filter(Boolean).join(" · ") || t("game.fishing.unknownSpot")
})
const originalItemName = computed(() => {
  return getLocalizedItemName(t, props.itemId, {
    nameKey: detail.value?.item?.nameKey || definition.value?.nameKey,
    fallbackText: t("game.unnamedItem")
  })
})
const customItemName = computed(() => itemCustomName(instance.value?.customData || detail.value?.instance?.customData))
const customItemLabelClass = computed(() => itemCustomLabelClass(instance.value?.customData || detail.value?.instance?.customData))
const forgedSkillBookName = computed(() => {
  const book = detail.value?.instance?.forgedSkillBook
  if (!book) return ""
  const skillName = getLocalizedRuntimeSkillName(t, book.customSkillId, book.compiledSkill || null)
  return t("skillForge.book.displayName", { skill: skillName })
})
const itemName = computed(
  () => String(props.displayNameOverride || "").trim() || (fishData.value ? customItemName.value || fishSpeciesName.value : forgedSkillBookName.value || customItemName.value || originalItemName.value)
)
const itemDescription = computed(() => {
  const primaryKey = detail.value?.item?.descKey || definition.value?.descKey
  const keys = primaryKey ? [primaryKey] : [`item.${props.itemId}.desc`]
  for (const key of keys) {
    const localized = t(key)
    if (localized !== key && localized.trim() !== "") return localized
  }
  return t("game.itemDetailUnknown")
})
const itemEffect = computed(() => {
  const primaryKey = detail.value?.item?.effectDescKey || definition.value?.effectDescKey
  const keys = primaryKey ? [primaryKey] : [`item.${props.itemId}.effect`]
  for (const key of keys) {
    const localized = t(key)
    if (localized !== key && localized.trim() !== "") return localized
  }
  return t("game.itemDetailUnknown")
})
const itemTypes = computed(() => {
  if (fishData.value) return ["fish"]
  const fromDetail = detail.value?.item?.type
  if (Array.isArray(fromDetail) && fromDetail.length > 0) return fromDetail
  const fromDefinition = definition.value?.type
  if (Array.isArray(fromDefinition) && fromDefinition.length > 0) return fromDefinition
  return ["unknown"]
})
const isFishingTackle = computed(() => itemTypes.value.includes("fishing_gear"))
const itemRarity = computed(() => fishSpecies.value?.rarity || detail.value?.item?.rarity || definition.value?.rarity || "common")
const skillbookGrantSkillID = computed(() => {
  const fromDetail = String(detail.value?.item?.useGrantSkillId || "").trim()
  if (fromDetail) return fromDetail
  const fromDefinition = String(definition.value?.useGrantSkillId || "").trim()
  if (fromDefinition) return fromDefinition
  return ""
})
const collectibleGrantID = computed(() => {
  const fromDetail = String(detail.value?.item?.useGrantCollectibleId || "").trim()
  if (fromDetail) return fromDetail
  const fromDefinition = String(definition.value?.useGrantCollectibleId || "").trim()
  if (fromDefinition) return fromDefinition
  return ""
})
const collectibleDefinition = computed(() => {
  ensureCollectibleMaps()
  if (collectibleGrantID.value) return collectibleById!.get(collectibleGrantID.value) || null
  return collectibleByItemId!.get(props.itemId) || null
})
const collectiblePlayerBonusEntries = computed(() => modEntries(collectibleDefinition.value?.playerMods))
const collectibleKittyBonusEntries = computed(() => modEntries(collectibleDefinition.value?.kittyMods))
const collectibleBonusEntries = computed(() => [...collectiblePlayerBonusEntries.value, ...collectibleKittyBonusEntries.value])
const equipmentSlotLabel = computed(() => (equipmentDefinition.value ? formatEquippedSlot(equipmentDefinition.value.slot) : ""))

const equipmentWeaponType = computed(() => (equipmentDefinition.value ? equipmentDefinition.value.weaponClass : ""))

const itemTypeLabel = computed(() => {
  return itemTypes.value.map((itemType) => localizeItemType(itemType)).join(" / ")
})
const rarityColor = computed(() => getRarityDefinition(itemRarity.value)?.color || "#9CA3AF")
const rarityGradientStyle = computed(() => ({
  // "--item-rarity-color": rarityColor.value
}))
const instance = computed(() => {
  if (detail.value?.instance) return detail.value.instance
  if (!props.publicInstancePreview) return null
  return {
    instanceId: Number(props.instanceId || 0),
    templateItemId: props.itemId,
    customData: props.publicInstancePreview.customData || {},
    fishData: props.publicInstancePreview.fishData,
    quantity: 1,
    state: "auction",
    enhanceLevel: Number(props.publicInstancePreview.enhanceLevel || 0),
    durability: Number(props.publicInstancePreview.durability || 0),
    maxDurability: Number(props.publicInstancePreview.maxDurability || 0),
    baseSnapshot: props.publicInstancePreview.baseSnapshot || {},
    affixes: props.publicInstancePreview.affixes || []
  }
})
const templateAffixSlotCount = computed(() => {
  return equipmentDefinition.value?.affixSlotCount
})
const templateDefaultAffixSlots = computed(() => {
  return equipmentDefinition.value?.defaultAffixSlots || []
})
const templateDefaultAffixCount = computed(() => {
  return templateDefaultAffixSlots.value.filter((slot) => String(slot.affixId || "").trim() !== "").length
})
const affixSlotCount = computed(() => detail.value?.operations?.affixSlotCount || templateAffixSlotCount.value || 0)
const showingTemplateAffixSlots = computed(() => !props.instanceId && !props.publicInstancePreview && affixSlotCount.value > 0)

const randomDefaultAffixPolicies = computed(() => {
  const out: Array<{ actionId: string; rarityWeights: Record<string, number> }> = []
  if (props.instanceId) return out
  for (const action of listActionDefinitions()) {
    const reward = action.itemReward?.[props.itemId]
    if (!reward || typeof reward !== "object" || Array.isArray(reward)) continue
    if (reward.grantPolicy !== "rollDefaultAffixes") continue
    const weights = normalizePositiveNumberMap(reward.defaultAffixRarityWeights)
    if (Object.keys(weights).length === 0) continue
    out.push({ actionId: action.id, rarityWeights: weights })
  }
  return out
})
const preferredRandomDefaultAffixPolicy = computed(() => randomDefaultAffixPolicies.value[0] || null)

const affixSlots = computed(() => {
  const operationSlots = detail.value?.operations?.affixSlots
  if (Array.isArray(operationSlots) && operationSlots.length > 0) return operationSlots
  const instanceSlots = detail.value?.instance?.affixes || instance.value?.affixes
  if (Array.isArray(instanceSlots) && instanceSlots.length > 0) return instanceSlots
  if (props.instanceId) return []
  return templateDefaultAffixSlots.value
})
const equippedEffectAttrs = computed(() => {
  const snapshotAttrs = resolveInstanceSnapshotAttrs()
  if (Object.keys(snapshotAttrs).length > 0) return snapshotAttrs
  return aggregateEquipmentAttrChanges(resolveEquipmentBaseAttrs(), affixSlots.value)
})
const hasEnhanceInfo = computed(() => Number.isFinite(Number(instance.value?.enhanceLevel)) && Number(instance.value?.enhanceLevel) > 0)
const enhanceLevel = computed(() => Number(instance.value?.enhanceLevel || 0))
const enhanceBonusTags = computed(() => {
  return Object.entries(detail.value?.enhance?.cumulativeAttrs || {})
    .filter(([, value]) => Number.isFinite(value) && Number(value) !== 0)
    .map(([attrKey, value]) => ({ attrKey, value: Number(value) }))
})
const enhanceNextPreview = computed(() => detail.value?.enhance?.nextPreview || null)
const enhanceCostLines = computed(() =>
  Object.entries(enhanceNextPreview.value?.costs || {})
    .filter(([, qty]) => Number(qty) > 0)
    .map(([itemId, qty]) => {
      const item = getItemDefinition(itemId)
      const key = item?.nameKey || `item.${itemId}.name`
      const localized = t(key)
      const name = localized === key ? itemId : localized
      return `${name} x${qty}`
    })
)
const affixSlotSummary = computed(() => {
  if (affixSlotCount.value <= 0) {
    return (affixSlots.value || [])
      .filter((slot) => Number.isInteger(slot.slotIndex) && String(slot.affixId || "").trim() !== "")
      .sort((a, b) => a.slotIndex - b.slotIndex)
  }
  return buildAffixSlotSummary(affixSlotCount.value, affixSlots.value)
})

function isTemplateRandomDefaultAffixSlot(slot: EquipmentAffixSlot): boolean {
  if (!showingTemplateAffixSlots.value || !preferredRandomDefaultAffixPolicy.value) return false
  const affixID = String(slot.affixId || "").trim()
  if (!affixID) return false
  return templateDefaultAffixSlots.value.some((entry) => Number(entry.slotIndex) === Number(slot.slotIndex) && String(entry.affixId || "").trim() === affixID)
}

function templateRandomDefaultAffixRarityWeights(slot: EquipmentAffixSlot): Record<string, number> | null {
  if (!isTemplateRandomDefaultAffixSlot(slot)) return null
  return preferredRandomDefaultAffixPolicy.value?.rarityWeights || null
}

function slotSourceBadges(slot: EquipmentAffixSlot): Array<{ kind: string; icon: string; title: string }> {
  return getAffixSlotSourceBadgeDefinitions(slot).map((badge) => ({ ...badge, title: t(badge.titleKey) }))
}

function normalizePositiveNumberMap(raw: unknown): Record<string, number> {
  if (!raw || typeof raw !== "object" || Array.isArray(raw)) return {}
  const out: Record<string, number> = {}
  for (const [key, value] of Object.entries(raw as Record<string, unknown>)) {
    const normalizedKey = String(key || "").trim()
    const normalizedValue = Number(value)
    if (!normalizedKey || !Number.isFinite(normalizedValue) || normalizedValue <= 0) continue
    out[normalizedKey] = normalizedValue
  }
  return out
}

function modEntries(mods: Record<string, number> | undefined): ModEntry[] {
  if (!mods) return []
  return Object.entries(mods)
    .filter(([, value]) => Number.isFinite(Number(value)) && Number(value) !== 0)
    .map(([attrKey, value]) => ({ attrKey, value: Number(value) }))
    .sort((a, b) => a.attrKey.localeCompare(b.attrKey))
}
const insight = computed<ItemInsight>(() => insightMap.value[props.itemId] || emptyItemInsight())
const insightUses = computed(() => insight.value.uses.filter((flow) => isGenericActionVisible(getActionDefinition(flow.actionId))))
const insightSources = computed(() => insight.value.sources.filter((flow) => isGenericActionVisible(getActionDefinition(flow.actionId))))
const insightEnemySources = computed(() => insight.value.enemySources)
const insightCatVillageCommissionSources = computed(() => insight.value.catVillageCommissionSources)
const insightCatteryCommissionSources = computed(() => insight.value.catteryCommissionSources)
const insightKittyJournalSources = computed(() => insight.value.kittyJournalSources)
const hasInsightProductionSources = computed(() =>
  insightSources.value.length > 0 ||
  insightEnemySources.value.length > 0 ||
  insightCatVillageCommissionSources.value.length > 0 ||
  insightCatteryCommissionSources.value.length > 0 ||
  insightKittyJournalSources.value.length > 0
)

const INSIGHT_COLLAPSE_LIMIT = 3
const sourcesExpanded = ref(false)
const enemySourcesExpanded = ref(false)
const catVillageSourcesExpanded = ref(false)
const catterySourcesExpanded = ref(false)
const kittyJournalSourcesExpanded = ref(false)
const visibleInsightSources = computed(() => sourcesExpanded.value ? insightSources.value : insightSources.value.slice(0, INSIGHT_COLLAPSE_LIMIT))
const visibleInsightEnemySources = computed(() => enemySourcesExpanded.value ? insightEnemySources.value : insightEnemySources.value.slice(0, INSIGHT_COLLAPSE_LIMIT))
const visibleInsightCatVillageCommissionSources = computed(() =>
  catVillageSourcesExpanded.value ? insightCatVillageCommissionSources.value : insightCatVillageCommissionSources.value.slice(0, INSIGHT_COLLAPSE_LIMIT)
)
const visibleInsightCatteryCommissionSources = computed(() =>
  catterySourcesExpanded.value ? insightCatteryCommissionSources.value : insightCatteryCommissionSources.value.slice(0, INSIGHT_COLLAPSE_LIMIT)
)
const visibleInsightKittyJournalSources = computed(() =>
  kittyJournalSourcesExpanded.value ? insightKittyJournalSources.value : insightKittyJournalSources.value.slice(0, INSIGHT_COLLAPSE_LIMIT)
)

watch(
  () => props.itemId,
  () => {
    sourcesExpanded.value = false
    enemySourcesExpanded.value = false
    catVillageSourcesExpanded.value = false
    catterySourcesExpanded.value = false
    kittyJournalSourcesExpanded.value = false
  }
)

watch(
  () => [props.active, props.itemId, props.instanceId] as const,
  async ([active]) => {
    if (!active) return
    loadError.value = ""
    store.restoreToken()
    await ensureInsight()
    await ensureDetail()
  },
  { immediate: true }
)

async function ensureInsight() {
  if (loadedInsight.value) return
  if (!store.token) return
  try {
    insightMap.value = await getItemInsightMap(store.token)
    loadedInsight.value = true
  } catch (err) {
    loadError.value = resolveErrorMessage(t, err, "game.itemTooltip.none")
  }
}

async function ensureDetail() {
  if (props.publicInstancePreview) {
    detail.value = null
    loadedDetailKey.value = `public:${props.itemId}:${props.instanceId}`
    return
  }
  if (!props.instanceId) {
    detail.value = null
    loadedDetailKey.value = ""
    return
  }
  if (!store.token) return
  const detailKey = `${props.itemId}:${props.instanceId}`
  if (loadedDetailKey.value === detailKey) {
    const cached = instanceDetailStore.get(props.instanceId)
    if (cached) {
      detail.value = cached
    }
    return
  }
  loading.value = true
  try {
    detail.value = await instanceDetailStore.fetchByInstance(store.token, props.instanceId, {
      itemId: props.itemId
    })
    loadedDetailKey.value = detailKey
  } catch (err) {
    if (isMissingInstanceDetailError(err, props.instanceId)) {
      detail.value = null
      loadedDetailKey.value = ""
      loadError.value = ""
      return
    }
    loadError.value = resolveErrorMessage(t, err, "game.itemDetailLoadFailed")
  } finally {
    loading.value = false
  }
}

function isMissingInstanceDetailError(err: unknown, instanceId?: number | null): boolean {
  if (!instanceId || instanceId <= 0) return false
  if (!(err instanceof APIClientError)) return false
  return err.code === "ITEM_DETAIL_INSTANCE_NOT_FOUND"
}

function localizeItemType(itemType: string): string {
  const key = `item_type.${itemType}.name`
  const localized = t(key)
  return localized === key ? itemType : localized
}

function localizeEquipSlot(slot: string): string {
  const key = `game.equipSlot.${slot}`
  const localized = t(key)
  return localized === key ? slot : localized
}

function formatEquippedSlot(raw: string): string {
  return raw
    .split(",")
    .map((slot) => slot.trim())
    .filter(Boolean)
    .map(localizeEquipSlot)
    .join(" / ")
}

function resolveEquipmentBaseAttrs(): Record<string, number> {
  const snapshot = instance.value?.baseSnapshot
  return equipmentDefinition.value?.baseAttrs || {}
}

function resolveInstanceSnapshotAttrs(): Record<string, number> {
  const snapshot = instance.value?.baseSnapshot
  if (!snapshot || typeof snapshot !== "object" || Array.isArray(snapshot)) return {}
  const out: Record<string, number> = {}
  for (const [attrKey, value] of Object.entries(snapshot as Record<string, unknown>)) {
    const numeric = Number(value)
    if (!Number.isFinite(numeric)) continue
    out[attrKey] = numeric
  }
  return out
}

function formatInsightPercent(value: number) {
  const numeric = Number(value)
  if (!Number.isFinite(numeric)) return "0"
  return (numeric * 100).toFixed(3).replace(/\\.0$/, "")
}

function formatFishLength(value: number): string {
  return `${Number(value || 0).toFixed(1)} cm`
}

function formatFishWeight(value: number): string {
  const numeric = Number(value || 0)
  return `${numeric.toFixed(numeric >= 10 ? 1 : 2)} kg`
}

function formatFishAge(days: number): string {
  const safe = Math.max(1, Math.round(Number(days) || 1))
  if (safe < 60) return t("game.fishing.ageDays", { days: safe })
  const months = Math.floor(safe / 30)
  if (months < 24) return t("game.fishing.ageMonths", { months })
  return t("game.fishing.ageYearsMonths", { years: Math.floor(months / 12), months: months % 12 })
}

function formatFishCondition(value: number): string {
  const label = value >= 1.04 ? "conditionPrime" : value < 0.9 ? "conditionLean" : "conditionGood"
  return `${t(`game.fishing.${label}`)} · ${Math.round(Number(value || 0) * 100)}%`
}

function formatFishHealth(value: number): string {
  const label = value >= 0.92 ? "healthGood" : "healthFair"
  return `${t(`game.fishing.${label}`)} · ${Math.round(Number(value || 0) * 100)}%`
}

function formatFishCaughtAt(value: string): string {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return t("game.fishing.unknownCaughtAt")
  return new Intl.DateTimeFormat(locale.value, {
    year: "numeric",
    month: "short",
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit"
  }).format(date)
}

function localizeNamedSource(source: ItemNamedSourceRef): string {
  const key = String(source.nameKey || "").trim()
  if (key) {
    const localized = t(key)
    if (localized !== key && localized.trim() !== "") return localized
  }
  return source.id
}

</script>

<style scoped>
@reference "#app-style";

.item-popover-content {
  width: 100%;
  display: grid;
  gap: 16px;
  position: relative;
  overflow: hidden;

}
.item-popover-content > * {
  position: relative;
  /* z-index: 1; */
}

.item-popover-score-breakdown {
  margin-top: 2px;
  font-size: 12px;
  color: var(--el-text-color-secondary);
}

.item-popover-rarity-mask {
  position: absolute;
  left: 0;
  right: 0;
  top: 0;
  height: 72px;
  z-index: 0;
  pointer-events: none;
  background: linear-gradient(
    to bottom,
    color-mix(in srgb, var(--item-rarity-color, transparent) 35%, transparent) 0%,
    color-mix(in srgb, var(--item-rarity-color, transparent) 12%, transparent) 45%,
    transparent 100%
  );
}
.item-popover-content.item-popover-content--fix-height {
  max-height: 400px;
  overflow-y: auto;
}

@media (max-width: 767px)  {
  .item-popover-content.item-popover-content--fix-height {
    max-height: fit-content;
  }
}

.item-popover-header {
  display: flex;
  align-items: center;
  gap: 8px;
}

.item-popover-fish-icon {
  width: 56px;
  height: 56px;
  flex: 0 0 56px;
  object-fit: contain;
}

.item-popover-fish-summary {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 8px 12px;
  padding: 10px 12px;
  border: 1px solid var(--app-border);
  border-radius: 6px;
  background: color-mix(in srgb, var(--app-surface-elevated) 72%, transparent);
}

.item-popover-fish-summary > div {
  min-width: 0;
  display: flex;
  justify-content: space-between;
  gap: 8px;
  font-size: 12px;
}
.item-popover-fish-summary > .item-popover-fish-mutations { grid-column: 1 / -1; justify-content: flex-start; flex-wrap: wrap; gap: 5px; }
.item-popover-fish-mutations span { display: inline-flex; align-items: center; gap: 3px; padding: 2px 6px; border: 1px solid color-mix(in srgb, #4daab6 44%, var(--app-border)); border-radius: 4px; color: #267884; font-size: 11px; font-weight: 700; }
.item-popover-fish-mutations svg { font-size: 12px; }

.item-popover-fish-summary span,
.item-popover-fish-summary p {
  color: var(--app-muted);
}

.item-popover-fish-summary strong {
  color: var(--app-text);
  text-align: right;
}

.item-popover-fish-summary p {
  grid-column: 1 / -1;
  display: flex;
  align-items: center;
  gap: 5px;
  margin: 0;
  font-size: 12px;
}

.item-popover-fish-summary p + p {
  margin-top: -4px;
}

.item-popover-fish-summary .item-popover-fish-sale-status {
  align-items: flex-start;
  gap: 6px;
  margin: 0 0 2px;
  padding: 7px 8px;
  border: 1px solid color-mix(in srgb, #9a6b22 34%, var(--app-border));
  border-radius: 5px;
  background: color-mix(in srgb, #c18a2f 10%, var(--app-surface-elevated));
  color: color-mix(in srgb, #8a5d18 86%, var(--app-text));
  line-height: 1.45;
}

.item-popover-fish-summary .item-popover-fish-sale-status svg {
  flex: none;
  margin-top: 1px;
  font-size: 14px;
}

.item-popover-fish-summary .item-popover-fish-sale-status span {
  color: inherit;
}

.item-popover-title-wrap {
  min-width: 0;
}

.item-popover-inventory {
  position: absolute !important;
  top: 8px;
  right: 0;
  font-size: 12px;
  color: var(--app-muted);
}

.item-popover-title {
  font-size: 14px;
  font-weight: 600;
  color: var(--app-text);
}

.item-popover-subtitle {
  font-size: 12px;
  color: var(--el-text-color-secondary);
}

.item-popover-row {
  display: grid;
  gap: 2px;
}

.item-popover-flow-grid {
  display: grid;
  gap: 8px;
}

.item-popover-flow-block {
  display: grid;
  gap: 2px;
}

.item-popover-label {
  font-size: 12px;
  color: var(--app-muted);
}

.item-popover-value {
  font-size: 12px;
  color: var(--el-text-color-regular);
  white-space: pre-wrap;
  word-break: break-word;
}

.item-popover-tip {
  margin-bottom: 4px;
  font-size: 11px;
  color: var(--app-muted);
}

.item-popover-slot-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
  border-radius: 8px;
  background: color-mix(in srgb, var(--app-bg) 30%, transparent);
}

.item-popover-slot-block {
  border: 1px solid color-mix(in srgb, var(--el-text-color-primary) 10%, transparent);
  border-radius: 8px;
  padding: 6px 8px;
  display: grid;
  gap: 2px;
}

.item-popover-affix-entry {
  min-width: 0;
  display: grid;
  gap: 4px;
  padding: 7px 8px;
  border: 1px solid color-mix(in srgb, var(--app-border) 72%, transparent);
  border-radius: 6px;
  background: color-mix(in srgb, var(--app-surface-elevated) 55%, transparent);
}

.item-popover-source-badges {
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
  padding-left: 0;
}

.item-popover-source-badge {
  min-width: 0;
  display: inline-flex;
  align-items: center;
  gap: 4px;
  padding: 2px 6px 2px 4px;
  border-radius: 999px;
  color: #fff;
  font-size: 11px;
  line-height: 1.3;
}

.item-popover-source-badge :deep(svg) {
  width: 13px;
  height: 13px;
  flex: 0 0 auto;
}

.item-popover-source-badge.is-craft {
  background: #1f9d61;
}

.item-popover-source-badge.is-inherit {
  background: #2f6fdb;
}

.item-popover-source-badge.is-orphan {
  background: #d64550;
}

.item-popover-source-badge.is-ordinary {
  background: #667085;
}

.item-popover-action-ref-list {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}

.item-popover-enemy-drop-list {
  display: grid;
  gap: 6px;
}

.item-popover-named-source-list {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}

.item-popover-named-source-chip {
  display: inline-flex;
  align-items: center;
  min-height: 24px;
  max-width: 100%;
  padding: 2px 8px;
  border: 1px solid transparent;
  border-radius: 4px;
  background: var(--el-color-primary-light-9);
  color: var(--el-color-primary);
  font-size: 12px;
  line-height: 1.35;
  overflow-wrap: anywhere;
}

.item-popover-enemy-drop-row {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 6px;
}

.item-popover-enemy-drop-detail {
  color: var(--el-text-color-secondary);
}

.item-popover-slot-desc {
  opacity: 0.8;
}

.status-require-list {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}

.item-popover-error {
  font-size: 12px;
  color: var(--el-color-danger);
}

.item-popover-sublabel {
  margin-top: 4px;
  font-size: 11px;
  color: var(--app-muted);
}

.item-popover-expand-btn {
  @apply text-[11px] text-blue-500 dark:text-blue-400 cursor-pointer mt-1 hover:underline;
  background: none;
  border: none;
  padding: 0;
  text-align: left;
}
</style>
