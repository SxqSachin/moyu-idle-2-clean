<template>
  <div class="item-detail-action-bar space-y-2">
    <div v-if="canEnhance || canEquip || canUnequip || canRepair || canRename || canUseSkillbook || canUseCollectible || canUseOperationEffect"
      class="item-detail-action-bar__row flex flex-row items-center gap-2 justify-between">
      <el-button class="w-full ml-0!" v-if="canEquip" type="primary" @click="$emit('equip')">
        <span class="item-detail-action-bar__button-content">
          <Icon v-if="showRichIcons" icon="game-icons:chest-armor" />
          <span>{{ t("game.equipToSlot") }}</span>
        </span>
      </el-button>
      <el-button class="w-full ml-0!" v-if="canUnequip" @click="$emit('unequip')">
        <span class="item-detail-action-bar__button-content">
          <Icon v-if="showRichIcons" icon="material-symbols:logout-rounded" />
          <span>{{ t("game.unequipSlot") }}</span>
        </span>
      </el-button>
      <el-button class="w-full ml-0!" v-if="canEnhance" type="primary" @click="$emit('goto-enhance')">
        <span class="item-detail-action-bar__button-content">
          <Icon v-if="showRichIcons" icon="game-icons:upgrade" />
          <span>{{ t("game.menu.enhance") }}</span>
        </span>
      </el-button>
      <el-button class="w-full ml-0!" v-if="canRepair" @click="$emit('repair')">
        <span class="item-detail-action-bar__button-content">
          <Icon v-if="showRichIcons" icon="material-symbols:build-rounded" />
          <span>{{ t("game.repair") }}</span>
        </span>
      </el-button>
      <el-button class="w-full ml-0!" v-if="canRename" @click="$emit('rename')">
        <span class="item-detail-action-bar__button-content">
          <Icon v-if="showRichIcons" icon="material-symbols:edit-rounded" />
          <span>{{ t("game.renameItem") }}</span>
        </span>
      </el-button>
      <el-button class="w-full ml-0!" v-if="canUseSkillbook" @click="$emit('use-skillbook')">
        <span class="item-detail-action-bar__button-content">
          <Icon v-if="showRichIcons" icon="material-symbols:menu-book-outline-rounded" />
          <span>{{ t("game.useSkillbook") }}</span>
        </span>
      </el-button>
      <el-button class="w-full ml-0!" v-if="canUseCollectible" type="success" @click="$emit('use-collectible')">
        <span class="item-detail-action-bar__button-content">
          <Icon v-if="showRichIcons" icon="material-symbols:trophy-outline-rounded" />
          <span>{{ t("game.useCollectible") }}</span>
        </span>
      </el-button>
      <el-button class="w-full ml-0!" v-if="canUseOperationEffect" type="success" @click="$emit('use-operation-effect')">
        <span class="item-detail-action-bar__button-content">
          <Icon v-if="showRichIcons" icon="lucide:utensils" />
          <span>{{ t("game.operationEffects.consume") }}</span>
        </span>
      </el-button>
    </div>

    <div v-if="canShowAffixOperations" class="item-detail-action-bar__row flex flex-row items-center gap-2 justify-between">
      <el-button class="w-full ml-0!" v-if="canEnchantAffix" @click="$emit('goto-enchant')">
        <span class="item-detail-action-bar__button-content">
          <Icon v-if="showRichIcons" icon="game-icons:sparkling-sabre" />
          <span>{{ t("game.enchant") }}</span>
        </span>
      </el-button>
      <el-button class="w-full ml-0!" v-if="canAttachAffix" @click="$emit('goto-embed')">
        <span class="item-detail-action-bar__button-content">
          <Icon v-if="showRichIcons" icon="material-symbols:add-link-rounded" />
          <span>{{ t("game.itemDetailAttachAffix") }}</span>
        </span>
      </el-button>
      <el-button class="w-full ml-0!" v-if="canDetachAffix" @click="$emit('goto-extract')">
        <span class="item-detail-action-bar__button-content">
          <Icon v-if="showRichIcons" icon="material-symbols:link-off-rounded" />
          <span>{{ t("game.itemDetailDetachAffix") }}</span>
        </span>
      </el-button>
    </div>

    <div v-if="canAlchemyConvert || canAlchemyTransmute" class="item-detail-action-bar__row flex flex-row items-center gap-2 justify-between">
      <el-button class="w-full ml-0!" v-if="canAlchemyConvert" @click="$emit('goto-alchemy-convert')">
        <span class="item-detail-action-bar__button-content">
          <Icon v-if="showRichIcons" icon="game-icons:chemical-drop" />
          <span>{{ t("alchemy.convert.actionTitle") }}</span>
        </span>
      </el-button>
      <el-button class="w-full ml-0!" v-if="canAlchemyTransmute" @click="$emit('goto-alchemy-transmute')">
        <span class="item-detail-action-bar__button-content">
          <Icon v-if="showRichIcons" icon="game-icons:gold-bar" />
          <span>{{ t("alchemy.transmute.title") }}</span>
        </span>
      </el-button>
    </div>

    <div v-if="canShowMarket || canShowRecycle || canShowDestructiveRecycle" class="item-detail-action-bar__row item-detail-action-bar__row--secondary flex flex-row items-center gap-2 justify-between">
      <el-button class="w-full ml-0!" v-if="canShowMarket" @click="$emit('goto-market')">
        <span class="item-detail-action-bar__button-content">
          <Icon v-if="showRichIcons" icon="game-icons:shop" />
          <span>{{ t("game.menu.market") }}</span>
        </span>
      </el-button>
      <el-button class="w-full ml-0!" v-if="canShowRecycle" type="warning" @click="$emit('recycle')">
        <span class="item-detail-action-bar__button-content">
          <Icon v-if="showRichIcons" icon="game-icons:recycle" />
          <span>{{ t("game.recycle") }}</span>
        </span>
      </el-button>
      <el-button v-else-if="canShowDestructiveRecycle" class="w-full ml-0!" type="danger" plain @click="$emit('destructive-recycle')">
        <span class="item-detail-action-bar__button-content">
          <Icon icon="lucide:trash-2" />
          <span>{{ t("game.destructiveRecycle.title") }}</span>
        </span>
      </el-button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from "vue"
import { Icon } from "@iconify/vue"
import { useI18n } from "vue-i18n"
import { useGameStore } from "@stores/game"

withDefaults(
  defineProps<{
    canUseSkillbook?: boolean
    canUseCollectible?: boolean
    canUseOperationEffect?: boolean
    canEnhance: boolean
    canEquip: boolean
    canUnequip: boolean
    canRepair: boolean
    canRename: boolean
    canShowAffixOperations: boolean
    canDetachAffix: boolean
    canAttachAffix: boolean
    canEnchantAffix: boolean
    canAlchemyConvert: boolean
    canAlchemyTransmute: boolean
    canShowMarket: boolean
    canShowRecycle: boolean
    canShowDestructiveRecycle?: boolean
  }>(),
  {
    canUseSkillbook: false,
    canUseCollectible: false,
    canUseOperationEffect: false,
    canEnhance: false,
    canEquip: false,
    canUnequip: false,
    canRepair: false,
    canRename: false,
    canShowAffixOperations: false,
    canDetachAffix: false,
    canAttachAffix: false,
    canEnchantAffix: false,
    canAlchemyConvert: false,
    canAlchemyTransmute: false,
    canShowMarket: false,
    canShowRecycle: false,
    canShowDestructiveRecycle: false
  }
)

defineEmits<{
  (e: "use-skillbook"): void
  (e: "use-collectible"): void
  (e: "use-operation-effect"): void
  (e: "goto-enhance"): void
  (e: "equip"): void
  (e: "unequip"): void
  (e: "repair"): void
  (e: "rename"): void
  (e: "goto-extract"): void
  (e: "goto-embed"): void
  (e: "goto-enchant"): void
  (e: "goto-alchemy-convert"): void
  (e: "goto-alchemy-transmute"): void
  (e: "goto-market"): void
  (e: "recycle"): void
  (e: "destructive-recycle"): void
}>()

const { t } = useI18n()
const gameStore = useGameStore()
const showRichIcons = computed(() => !gameStore.playerSimpleMode)
</script>

<style scoped>
.item-detail-action-bar__button-content {
  display: inline-flex;
  min-width: 0;
  align-items: center;
  justify-content: center;
  gap: 6px;
}

.item-detail-action-bar__button-content svg {
  flex: 0 0 auto;
}
</style>
