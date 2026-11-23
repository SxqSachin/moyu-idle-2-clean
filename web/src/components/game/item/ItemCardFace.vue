<template>
  <LabeledFrameCard
    :class="{ 'item-card--skillbook': isSkillbook }"
    :hide-border="hideBorder"
    :label="hideLabel ? undefined : itemLabel"
    :label-class="labelClass"
    :label-lines="2"
    :rarity-color="itemRarityColor"
    :width="width"
    :height="height"
  >
    <FishMutationArt
      v-if="customIconSrc && fishData"
      class="item-card-custom-fish-icon"
      :src="customIconSrc"
      :alt="itemLabel"
      :fish="fishData"
      :width="pIconSize + 8"
    />
    <img
      v-else-if="customIconSrc"
      class="item-card-custom-icon"
      :src="customIconSrc"
      :alt="itemLabel"
      :style="{ width: `${pIconSize + 8}px`, height: `${pIconSize + 8}px` }"
    />
    <template v-else-if="skillbookOverlayIcon">
      <TypeIcon :type="iconType" :id="iconID" :alt="itemLabel" :size="pIconSize + 8" :fallback="fallbackIcon" />
      <div class="absolute bottom-1 left-1 inline-flex items-center justify-center rounded p-[1px] shadow-sm" :style="skillbookOverlayStyle">
        <Icon :icon="skillbookOverlayIcon" class="text-[12px]" />
      </div>
    </template>
    <template v-else>
      <template v-if="isSkillbook">
        <TypeIcon :type="iconType" :id="iconID" :alt="itemLabel" :size="pIconSize + 8" :fallback="fallbackIcon" />
      </template>
      <template v-else>
        <TypeIcon :type="iconType" :id="iconID" :alt="itemLabel" :size="pIconSize" :fallback="fallbackIcon" />
      </template>
    </template>
    <template v-if="(quantity > 0 || showZeroQuantity) && (enhanceLevelBadge === null || quantity > 1)">
      <div class="absolute bottom-[1px] right-[3px] text-[10px] drop-shadow-sm text-gray-700 dark:text-gray-300">
        {{ formatQuantityDisplay(quantity) }}
      </div>
    </template>
    <template v-if="enhanceLevelBadge !== null">
      <div class="absolute top-[4px] right-[4px] rounded px-1 text-[10px] font-semibold text-amber-700 drop-shadow-sm">
        +{{ enhanceLevelBadge }}
      </div>
    </template>
    <template v-if="previewAffixes.length">
      <div class="absolute top-1.5 left-1.5 flex flex-col flex-wrap gap-1 h-8 w-4">
        <template v-for="(affix, index) in previewAffixes" :key="`preview-affix-${index}`">
          <span class="w-[4px] h-[4px] rounded-full inline-block mr-[1px] drop-shadow-sm"
          :style="{
            backgroundColor: getRarityDefinition(affix.rarityId)?.color
          }"
          >
          </span>
        </template>
      </div>
    </template>
    <template v-if="sourceBadges.length">
      <div class="item-card-source-badges absolute bottom-[3px] left-[3px] flex flex-row gap-[1px]">
        <span
          v-for="badge in sourceBadges"
          :key="badge.kind"
          class="item-card-source-badge"
          :class="`is-${badge.kind}`"
          :title="badge.title"
          :aria-label="badge.title"
        >
          <Icon :icon="badge.icon" />
        </span>
      </div>
    </template>
    <template v-if="!previewAffixes.length && !instanceId && templateAffixes.length">
      <div class="absolute top-1.5 left-1.5 flex flex-col flex-wrap gap-1 h-8 w-4">
        <template v-for="(affix, index) in templateAffixes" :key="`template-affix-${index}`">
          <span class="w-[3px] h-[3px] rounded-full inline-block mr-[1px] drop-shadow-sm"
          :style="{
            backgroundColor: getRarityDefinition(affix.rarityId)?.color
          }"
          >
          </span>
        </template>
      </div>
    </template>
  </LabeledFrameCard>
</template>

<script setup lang="ts">
import { Icon } from "@iconify/vue"
import TypeIcon from "@components/game/icon/TypeIcon.vue"
import LabeledFrameCard from "@components/game/common/LabeledFrameCard.vue"
import FishMutationArt from "@components/game/fishing/FishMutationArt.vue"
import { getRarityDefinition } from "@services/item-definition"
import { formatQuantityDisplay } from "@/utils/number-format"
import type { EquipmentAffixSlot, FishInstanceData } from "@/types/equipment"

withDefaults(
  defineProps<{
    itemLabel: string
    itemRarityColor: string
    iconType: string
    iconID: string
    fallbackIcon: string
    pIconSize: number
    isSkillbook: boolean
    skillbookOverlayIcon: string
    skillbookOverlayStyle: Record<string, string>
    quantity: number
    enhanceLevelBadge: number | null
    sourceBadges: Array<{ kind: "craft" | "inherit" | "orphan" | "ordinary"; icon: string; title: string }>
    previewAffixes: EquipmentAffixSlot[]
    templateAffixes: EquipmentAffixSlot[]
    customIconSrc?: string
    fishData?: FishInstanceData | null
    labelClass?: string
    instanceId?: number | null
    width?: number
    height?: number
    hideBorder?: boolean
    hideLabel?: boolean
    showZeroQuantity?: boolean
  }>(),
  {
    labelClass: "",
    instanceId: null,
    hideBorder: false,
    hideLabel: false,
    showZeroQuantity: false,
    customIconSrc: "",
    fishData: null
  }
)
</script>

<style scoped>
.item-card-source-badge {
  width: 13px;
  height: 13px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border-radius: 3px;
  border: 1px solid rgba(255, 255, 255, 0.85);
  color: #fff;
  font-size: 9px;
  line-height: 1;
}

.item-card-custom-icon {
  display: block;
  object-fit: contain;
}

.item-card-custom-fish-icon {
  max-height: calc(100% - 16px);
}

.item-card-source-badge :deep(svg) {
  width: 9px;
  height: 9px;
  display: block;
}

.item-card-source-badge.is-craft {
  background: #278a5a;
}

.item-card-source-badge.is-inherit {
  background: #356fc7;
}

.item-card-source-badge.is-orphan {
  background: #bd4650;
}

.item-card-source-badge.is-ordinary {
  background: #6b7280;
}
</style>
