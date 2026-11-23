<template>
  <div v-loading="loading" class="space-y-3 item-detail-panel">
    <template v-if="detail">
      <ItemPopoverContent
        :item-id="itemId"
        :instance-id="instanceId"
        :requirement-attributes="displayRequirementAttributes"
        :active="active"
        :display-name-override="forgedSkillBookDisplayName"
      />

      <section v-if="forgedSkillBook" class="forged-book-detail">
        <el-alert v-if="forgedSkillBook.usable === false" :title="t('skillForge.rulesetDisabled')" type="error" :closable="false" show-icon />
        <SkillQuickRef :skill-id="forgedSkillBook.customSkillId" :runtime-skill="forgedSkillBook.compiledSkill || null" :show-icon="true" />
        <div class="forged-book-detail__meta">
          <span>{{ t("skillForge.public.source") }}</span><strong>{{ getLocalizedSkillName(t, forgedSkillBook.sourceSkillId, forgedSkillBook.sourceSkillId) }}</strong>
          <span>{{ t("skillForge.preview.budget") }}</span><strong>{{ Number(forgedSkillBook.budget?.used || 0).toFixed(1) }} / {{ Number(forgedSkillBook.budget?.limit || 0).toFixed(1) }}</strong>
        </div>
        <div class="forged-book-detail__actions">
          <el-button type="primary" :disabled="forgedSkillBook.usable === false" @click="learnForgedSkillbook('player')">{{ t("skillForge.book.learnPlayer") }}</el-button>
          <div v-if="kittyTargets.length" class="forged-book-detail__kitty">
            <el-select v-model="forgedKittyKey" :placeholder="t('skillForge.book.chooseKitty')">
              <el-option v-for="target in kittyTargets" :key="target.key" :label="target.label" :value="target.key" />
            </el-select>
            <el-button :disabled="!forgedKittyKey || forgedSkillBook.usable === false" @click="learnForgedSkillbook('kitty')">{{ t("skillForge.book.learnKitty") }}</el-button>
          </div>
          <el-button type="danger" plain @click="destroyForgedSkillbook">{{ t("skillForge.book.destroy") }}</el-button>
        </div>
      </section>

      <el-divider />

      <ItemDetailActionBar
        :can-use-skillbook="canShowUseSkillbook"
        :can-use-collectible="canShowUseCollectible"
        :can-use-operation-effect="canShowUseOperationEffect"
        :can-enhance="canShowEnhance"
        :can-equip="canShowEquip"
        :can-unequip="canShowUnequip"
        :can-repair="canShowRepair"
        :can-rename="canShowRename"
        :can-show-affix-operations="canShowAffixOperations"
        :can-detach-affix="canDetachAffixAction"
        :can-attach-affix="canAttachAffixAction"
        :can-enchant-affix="canEnchantAffixAction"
        :can-alchemy-convert="canShowAlchemyConvert"
        :can-alchemy-transmute="canShowAlchemyTransmute"
        :can-show-market="canShowMarket"
        :can-show-recycle="canShowRecycle"
        :can-show-destructive-recycle="canShowDestructiveRecycle"
        @use-skillbook="useSkillbook"
        @use-collectible="useCollectible"
        @use-operation-effect="openOperationEffectDialog"
        @goto-enhance="gotoEnhance"
        @equip="openEquipDialog"
        @unequip="unequipItem"
        @repair="openRepairDialog"
        @rename="openRenameDialog"
        @goto-extract="gotoExtract"
        @goto-embed="gotoEmbed"
        @goto-enchant="gotoEnchant"
        @goto-alchemy-convert="gotoAlchemyConvert"
        @goto-alchemy-transmute="gotoAlchemyTransmute"
        @goto-market="gotoMarket"
        @recycle="openRecycleDialog"
        @destructive-recycle="destructiveRecycleVisible = true"
      />
    </template>

    <GameDialog
      v-model="operationEffectDialogVisible"
      :title="t('game.operationEffects.useTitle')"
      width="400px"
      :close-on-click-modal="!operationEffectSubmitting"
    >
      <div v-if="detail?.operationEffect" class="operation-effect-use">
        <div class="operation-effect-use__summary">
          <div class="operation-effect-use__icon" aria-hidden="true">
            <GameIcon
              v-if="detail.operationEffect.iconId || currentItemId"
              :icon-id="detail.operationEffect.iconId || `item.${currentItemId}`"
              :size="36"
              fallback="✦"
            />
            <Icon v-else icon="lucide:utensils" />
          </div>
          <div class="min-w-0">
            <strong>{{ operationEffectName }}</strong>
            <p>{{ operationEffectDescription }}</p>
          </div>
        </div>

        <div class="operation-effect-use__lifetime">
          <Icon :icon="detail.operationEffect.lifetime.kind === 'duration' ? 'lucide:timer' : 'lucide:circle-dot-dashed'" />
          <span>{{ operationEffectLifetimeText }}</span>
        </div>
        <OperationEffectAudience :targets="operationEffectTargetOptions.filter(option => option.key === operationEffectTargetKey).map(option => option.targetKind)" :lifetime-kind="detail.operationEffect.lifetime.kind" />

        <div v-if="operationEffectModifierLines.length" class="operation-effect-use__modifiers">
          <div v-for="line in operationEffectModifierLines" :key="line">
            <Icon icon="lucide:flask-conical" />
            <span>{{ line }}</span>
          </div>
        </div>

        <label class="operation-effect-use__target">
          <span>{{ t("game.operationEffects.targetLabel") }}</span>
          <el-select v-model="operationEffectTargetKey" class="w-full" :disabled="operationEffectSubmitting">
            <el-option
              v-for="target in operationEffectTargetOptions"
              :key="target.key"
              :label="target.label"
              :value="target.key"
            />
          </el-select>
        </label>
        <OperationEffectServingControl
          v-model="operationEffectQuantity"
          :preview="operationEffectServingPreview"
          :stock="Math.max(0, Number(detail.instance?.quantity || 0))"
          :ready="operationEffectPreviewReady"
          :disabled="operationEffectSubmitting"
        />
      </div>
      <template #footer>
        <div class="flex justify-end gap-2">
          <el-button :disabled="operationEffectSubmitting" @click="operationEffectDialogVisible = false">
            {{ t("game.cancel") }}
          </el-button>
          <el-button
            type="success"
            :loading="operationEffectSubmitting"
            :disabled="!operationEffectTargetKey || !operationEffectPreviewReady || !operationEffectServingPreview.valid"
            @click="confirmUseOperationEffect"
          >
            <span class="inline-flex items-center gap-1.5">
              <Icon icon="lucide:utensils" />
              <span>{{ t("game.operationEffects.consume") }}</span>
            </span>
          </el-button>
        </div>
      </template>
    </GameDialog>

    <GameDialog
      v-model="operationTargetDialogVisible"
      :title="operationTargetDialogTitle"
      width="360px"
      :close-on-click-modal="false"
      @closed="cancelOperationTargetDialog"
    >
      <div class="space-y-2">
        <div class="text-xs app-muted">{{ t("game.itemOperationTargetLabel") }}</div>
        <el-select v-model="operationTargetDialogSelection" class="w-full">
          <el-option
            v-for="target in operationTargets"
            :key="target.key"
            :label="target.label"
            :value="target.key"
          />
        </el-select>
      </div>
      <template #footer>
        <div class="flex justify-end gap-2">
          <el-button @click="cancelOperationTargetDialog">{{ t("game.cancel") }}</el-button>
          <el-button type="primary" @click="confirmOperationTargetDialog">{{ t("game.confirm") }}</el-button>
        </div>
      </template>
    </GameDialog>

    <ItemEquipDialog
      v-model="equipDialogVisible"
      v-model:selected-slot="equipSlot"
      v-model:selected-target="equipTargetKey"
      :slots="equipSlots"
      :show-target-select="showEquipTargetSelectInDialog"
      :target-options="operationTargets"
      :requirement-rows="equipRequirementRows"
      :disabled="!canOperateCurrentInstance || equipSubmitting"
      @confirm="equipItem"
    />

    <ItemRepairDialog v-model="repairDialogVisible" @confirm="repairItem" />

    <ItemRenameDialog
      v-model="renameDialogVisible"
      :instance-id="currentInstanceId()"
      :custom-data="currentInstance?.customData"
      @renamed="handleItemRenamed"
    />

    <GameDialog
      v-model="skillbookUseDialogVisible"
      :title="t('game.useSkillbook')"
      class="dialog-mobile-windowed"
      width="360px"
      :close-on-click-modal="false"
    >
      <div class="space-y-3">
        <template v-if="showEquipTargetSelectInDialog">
          <div class="text-xs app-muted">{{ t("game.itemOperationTargetLabel") }}</div>
          <el-select v-model="skillbookTargetKey" class="w-full">
            <el-option
              v-for="target in operationTargets"
              :key="target.key"
              :label="target.label"
              :value="target.key"
            />
          </el-select>
        </template>
        <div class="text-xs app-muted">{{ t("game.useSkillbookCountLabel") }}</div>
        <el-input-number
          v-model="skillbookUseCount"
          :min="1"
          :max="skillbookUseMaxCount"
          :step="1"
          class="w-full!"
        />
        <div class="text-xs app-muted">{{ t("game.useSkillbookCountHint", { max: skillbookUseMaxCount }) }}</div>
      </div>
      <template #footer>
        <div class="flex justify-end gap-2">
          <el-button @click="skillbookUseDialogVisible = false">{{ t("game.cancel") }}</el-button>
          <el-button type="primary" :disabled="skillbookUseCount < 1" @click="confirmUseSkillbook">{{ t("game.confirm") }}</el-button>
        </div>
      </template>
    </GameDialog>

    <ItemDestructiveRecycleDialog
      v-model="destructiveRecycleVisible"
      :items="currentInstance ? [currentInstance] : []"
      @done="onDestructiveRecycleDone"
    />
    <ItemRecycleDialog
      v-model="recycleDialogVisible"
      v-model:quantity="recycleQuantity"
      :item-id="currentItemId"
      :max-quantity="recycleMaxQuantity"
      :base-value="recycleBaseValue"
      :recycle-coefficient="recycleCoefficient"
      @confirm="recycleItem"
    />
  </div>
</template>

<script setup lang="ts">
import OperationEffectAudience from "@components/game/operation-effect/OperationEffectAudience.vue"
import OperationEffectServingControl from "@components/game/operation-effect/OperationEffectServingControl.vue"
import { previewOperationEffectServings } from "@services/operation-effect-servings"
import { operationEffectFoodTargets } from "@services/operation-effect-food-targets"
import { operationEffectLifetimeText as describeEffectLifetime, operationEffectModifierLines as describeEffectModifiers } from "@services/operation-effect-display"
import { computed, ref, watch } from "vue"
import { ElMessage, ElMessageBox } from "element-plus"
import { Icon } from "@iconify/vue"
import { useI18n } from "vue-i18n"
import { useRouter } from "vue-router"
import { APIClientError, apiGet, apiPost, createIdempotencyKey } from "@services/api"
import { getAlchemyGroupForItem, isAlchemyTransmutableItem } from "@services/alchemy-data"
import { resolveErrorMessage } from "@services/error-messages"
import { hasAffixCapacity, hasAnyAffix, isAffixFull } from "@services/equipment-affix-slots"
import { getEquipmentAllowedSlots, getEquipmentEquipType, getEquipmentItemDefinition, getEquipmentTemplateDefinition, getItemDefinition, isItemFixedMarketListable } from "@services/item-definition"
import { itemCustomName } from "@services/item-customization"
import { KITTY_BATTLE_EQUIP_SLOTS } from "@/config/shared-global-config"
import { useGameStore } from "@stores/game"
import { useEquipmentStore } from "@stores/equipment"
import { useInventoryStore } from "@stores/inventory"
import { useItemInstanceDetailStore } from "@stores/item-instance-detail"
import { usePlayerStore } from "@stores/player"
import { useRoleStateStore, type RoleKey } from "@stores/role-state"
import { useSpecializationStore } from "@stores/specialization"
import { filterPlayerBattleEquipSlots } from "@services/specialization"
import { getRecycleTemplateMatchKind, isTemplateCommodityInstance, useInventorySummary } from "@/composables/game/useInventorySummary"
import type { ItemDetailResponse } from "@/types/equipment"
import ItemPopoverContent from "./ItemPopoverContent.vue"
import ItemDetailActionBar from "./ItemDetailActionBar.vue"
import ItemEquipDialog from "./ItemEquipDialog.vue"
import ItemRenameDialog from "./ItemRenameDialog.vue"
import ItemRepairDialog from "./ItemRepairDialog.vue"
import ItemRecycleDialog from "./ItemRecycleDialog.vue"
import ItemDestructiveRecycleDialog from "./ItemDestructiveRecycleDialog.vue"
import { isDestructiveRecycleCandidate } from "@services/destructive-recycle"
import GameDialog from "@components/game/common/GameDialog.vue"
import SkillQuickRef from "@components/game/skill/SkillQuickRef.vue"
import { getLocalizedRuntimeSkillName, getLocalizedSkillName } from "@services/skill-definition"
import {
  type OperationEffectActivationTarget,
  type OperationEffectTargetKind
} from "@services/operation-effects"
import { useOperationEffects } from "@/composables/game/useOperationEffects"
import { formatDurationI18n } from "@/utils/duration-format"
import GameIcon from "@components/game/icon/Icon.vue"

const props = withDefaults(
  defineProps<{
    itemId: string
    instanceId?: number | null
    active?: boolean
    operationTargetType?: "auto" | "player" | "kitty"
    operationTargetKittyId?: number | null
    requirementAttributes?: Record<string, number> | null
  }>(),
  {
    instanceId: null,
    active: true,
    operationTargetType: "auto",
    operationTargetKittyId: null,
    requirementAttributes: null
  }
)

const emit = defineEmits<{
  (e: "updated"): void
  (e: "request-close"): void
}>()

const { t, te } = useI18n()
const router = useRouter()
const store = useGameStore()
const playerStore = usePlayerStore()
const operationEffects = useOperationEffects()
const roleStateStore = useRoleStateStore()
const specializationStore = useSpecializationStore()
const equipmentStore = useEquipmentStore()
const inventoryStore = useInventoryStore()
const instanceDetailStore = useItemInstanceDetailStore()
const { hasInventoryInstance, getInventoryQuantity } = useInventorySummary()

const loading = ref(false)
const detail = ref<ItemDetailResponse | null>(null)
const equipDialogVisible = ref(false)
const equipSubmitting = ref(false)
const repairDialogVisible = ref(false)
const renameDialogVisible = ref(false)
const recycleDialogVisible = ref(false)
const destructiveRecycleVisible = ref(false)
const recycleQuantity = ref(1)
const skillbookUseDialogVisible = ref(false)
const skillbookUseCount = ref(1)
const skillbookTargetKey = ref("player")
const forgedKittyKey = ref("")
const operationEffectDialogVisible = ref(false)
const operationEffectSubmitting = ref(false)
const operationEffectTargetKey = ref("")
const operationEffectQuantity = ref(1)
const operationEffectPreviewLoading = ref(false)
const operationEffectPreviewReady = computed(() => !operationEffectPreviewLoading.value && operationEffects.initialized.value && !operationEffects.stale.value)
const operationEffectServingPreview = computed(() => previewOperationEffectServings(
  detail.value?.operationEffect, selectedOperationEffectTarget(), operationEffects.activeEffects.value,
  operationEffects.serverNowMs.value, Number(detail.value?.instance?.quantity || 0), operationEffectQuantity.value
))
const operationEffectPendingRequest = ref<{ signature: string; requestId: string } | null>(null)
const equipSlot = ref("right_hand")
const equipTargetKey = ref("player")
const operationTargetDialogVisible = ref(false)
const operationTargetDialogSelection = ref("player")
const operationTargetDialogAction = ref<"equip" | "skillbook">("equip")
let operationTargetDialogResolver: ((target: OperationTarget | null) => void) | null = null

type OperationTarget = { type: "player" } | { type: "kitty"; kittyId: number }
type RoleMutationSyncPayload = {
  roleAttributePatch?: Record<string, unknown>
  roleEquipmentPatch?: Record<string, unknown>
}

const currentItemId = computed(() => detail.value?.item?.id || props.itemId)
const currentInstance = computed(() => detail.value?.instance || null)
const forgedSkillBook = computed(() => currentInstance.value?.forgedSkillBook || null)
const forgedSkillBookDisplayName = computed(() => {
  const book = forgedSkillBook.value
  if (!book) return ""
  const skillName = getLocalizedRuntimeSkillName(t, book.customSkillId, book.compiledSkill || null)
  return t("skillForge.book.displayName", { skill: skillName })
})
const ownedInstance = computed(() => {
  const instanceId = currentInstanceId()
  if (!instanceId) return null
  return inventoryStore.instances.find((item) => item.instanceId === instanceId) || null
})
const equipmentDefinition = computed(() => getEquipmentItemDefinition(currentItemId.value))
const equipSlots = computed(() => {
  if (!equipmentDefinition.value) return ["right_hand"]
  const allowed = getEquipmentAllowedSlots(equipmentDefinition.value.id)
  if (allowed.length > 0) {
    const target = resolvePreferredOperationTarget() || parseOperationTargetKey(String(equipTargetKey.value || "").trim())
    if (target?.type === "player" && getEquipmentEquipType(equipmentDefinition.value.id) === "battle") {
      return filterPlayerBattleEquipSlots(
        allowed,
        specializationStore.summary ? specializationStore.enabledBattleSlotIds : undefined
      )
    }
    if (target?.type === "kitty" && getEquipmentEquipType(equipmentDefinition.value.id) === "battle") {
      const roleKey = `kitty:${target.kittyId}` as RoleKey
      const visibleSlots = roleStateStore.ensureRole(roleKey).capabilities.visibleSlots
      const enabled = new Set(visibleSlots.length > 0 ? visibleSlots : KITTY_BATTLE_EQUIP_SLOTS)
      return allowed.filter((slot) => enabled.has(slot))
    }
    return allowed
  }
  return ["right_hand"]
})
const affixSlots = computed(() => detail.value?.operations?.affixSlots || detail.value?.instance?.affixes || [])
const affixSlotCount = computed(() => detail.value?.operations?.affixSlotCount || 0)
const hasAffixSlots = computed(() => hasAffixCapacity(affixSlotCount.value))
const hasAnyFilledAffix = computed(() => hasAnyAffix(affixSlots.value, affixSlotCount.value))
const affixSlotsFull = computed(() => isAffixFull(affixSlots.value, affixSlotCount.value))
const canDetachAffix = computed(() => hasAffixSlots.value && hasAnyFilledAffix.value)
const canAttachAffix = computed(() => hasAffixSlots.value && !affixSlotsFull.value)
const canEnchantAffix = computed(() => hasAffixSlots.value && !affixSlotsFull.value)
const canShowAffixOperations = computed(() => isEquipment.value && hasAffixSlots.value)
const isEquipment = computed(() => !!equipmentDefinition.value || detail.value?.item.isEquipment === true)
const canUseSkillbook = computed(() => detail.value?.item.type?.includes("skillbook") && !!detail.value?.instance)
const skillbookUseMaxCount = computed(() => Math.max(1, detail.value?.instance?.quantity ?? 1))
const canUseCollectible = computed(() => detail.value?.item.type?.includes("collectible") && !!detail.value?.instance && detail.value.instance.state === "inventory")
const canEnhance = computed(() => {
  if (!currentInstance.value || currentInstance.value.state !== "inventory" || !isEquipment.value) return false
  const maxLevel = Number(getEquipmentTemplateDefinition(currentItemId.value)?.enhanceMaxLevel ?? 0)
  return Number.isFinite(maxLevel) && maxLevel > 0
})
const canEquip = computed(() => !!currentInstance.value && currentInstance.value.state === "inventory" && isEquipment.value)
const canUnequip = computed(() => currentInstance.value?.state === "equipped" && isEquipment.value)
const canRepair = computed(() => false && !!currentInstance.value && isEquipment.value)
const canDetachAffixAction = computed(() => !!detail.value?.instance && isEquipment.value && canDetachAffix.value)
const canAttachAffixAction = computed(() => !!detail.value?.instance && isEquipment.value && canAttachAffix.value)
const canEnchantAffixAction = computed(() => !!detail.value?.instance && isEquipment.value && canEnchantAffix.value)
const canShowUseSkillbook = computed(() => canUseSkillbook.value && canOperateCurrentInstance.value && !forgedSkillBook.value)
const canShowUseCollectible = computed(() => canUseCollectible.value && canOperateCurrentInstance.value)
const operationEffectTargetOptions = computed(() =>
  operationEffectFoodTargets(detail.value?.operationEffect, playerStore.kitties || [], t)
)
const canUseOperationEffect = computed(() => {
  const item = detail.value?.item
  const instance = detail.value?.instance
  return Boolean(
    item?.type?.includes("food") &&
    String(item.useOperationEffectId || "").trim() &&
    detail.value?.operationEffect &&
    instance?.state === "inventory"
  )
})
const canShowUseOperationEffect = computed(() =>
  canUseOperationEffect.value && canOperateCurrentInstance.value && operationEffectTargetOptions.value.length > 0
)
const operationEffectName = computed(() => {
  const definition = detail.value?.operationEffect
  return localizedOperationEffectText(definition?.nameKey, definition?.id || currentItemId.value)
})
const operationEffectDescription = computed(() => {
  const definition = detail.value?.operationEffect
  return localizedOperationEffectText(definition?.descKey, definition?.kind || "")
})
const operationEffectLifetimeText = computed(() => {
  const definition = detail.value?.operationEffect
  if (!definition) return ""
  const specificText = describeEffectLifetime(definition, t)
  if (specificText) return specificText
  const lifetime = definition.lifetime
  if (lifetime.kind === "charges") {
    return t("game.operationEffects.grantsCharges", {
      count: Math.max(0, Math.trunc(Number(lifetime.charges || 0)))
    })
  }
  return t("game.operationEffects.grantsDuration", {
    time: formatDurationI18n(Math.max(0, Number(lifetime.durationSeconds || 0)), t)
  })
})
const operationEffectModifierLines = computed(() => {
  const definition = detail.value?.operationEffect
  if (!definition) return []
  return describeEffectModifiers(definition.kind, definition.params || {}, t)
})
const canShowEnhance = computed(() => canEnhance.value && canOperateCurrentInstance.value)
const canShowEquip = computed(() => canEquip.value && canOperateCurrentInstance.value)
const canShowUnequip = computed(() => canUnequip.value)
const canShowRepair = computed(() => canRepair.value)
const canShowAlchemyConvert = computed(() => {
  if (!currentInstance.value || currentInstance.value.state !== "inventory") return false
  if (!canOperateCurrentInstance.value) return false
  return !!getAlchemyGroupForItem(currentItemId.value)
})
const canShowAlchemyTransmute = computed(() => {
  if (!currentInstance.value || currentInstance.value.state !== "inventory") return false
  if (!canOperateCurrentInstance.value) return false
  const unitValue = Number(getItemDefinition(currentItemId.value)?.economyBaseValue || 0)
  return isAlchemyTransmutableItem(currentItemId.value) && Number.isFinite(unitValue) && unitValue > 0
})
const canRename = computed(() => {
  const instance = currentInstance.value
  if (!instance || !ownedInstance.value) return false
  if (!["inventory", "equipped", "auction"].includes(String(instance.state || ""))) return false
  return getItemDefinition(currentItemId.value)?.stackable === false
})
const canShowRename = computed(() => canRename.value && !forgedSkillBook.value)
const canShowMarket = computed(() => {
  if (!currentInstance.value) return false
  if (currentInstance.value.state !== "inventory") return false
  if (!canOperateCurrentInstance.value) return false
  if (getItemDefinition(currentItemId.value)?.marketPolicy?.fixedPrice.mode === "instance_variants") return currentInstance.value.quantity === 1 && !currentInstance.value.archivedAt
  if (!isItemFixedMarketListable(currentItemId.value)) return false
  if (itemCustomName(currentInstance.value.customData)) return false
  if (!isTemplateCommodityInstance(currentInstance.value)) return false
  return true
})
const recycleBaseValue = computed(() => {
  const value = Number(detail.value?.item?.economyBaseValue ?? 0)
  return Number.isFinite(value) ? value : 0
})
const recycleMatchKind = computed(() => {
  if (!currentInstance.value) return "not_recyclable" as const
  return getRecycleTemplateMatchKind(currentInstance.value)
})
const recycleCoefficient = computed(() => {
  const res = (() => {
    const base = Number(detail.value?.recycleGoldCoefficient ?? 1)
    const extraCoefficient = Number(detail.value?.recycleAffixMismatchExtraCoefficient ?? 0)
    if (!Number.isFinite(base) || base <= 0) return 0
    if (!currentInstance.value) return base
    if (recycleMatchKind.value !== "affix_mismatch_only") return base
    if (!Number.isFinite(extraCoefficient) || extraCoefficient <= 0) return base
    const effective = base + extraCoefficient - 1
    return Number.isFinite(effective) && effective > 0 ? effective : base
  })();

  return +res.toFixed(2);
})
const recycleMaxQuantity = computed(() => Math.max(0, detail.value?.instance?.quantity ?? 0))
const canShowRecycle = computed(() => {
  return (
    detail.value?.instance?.state === "inventory" &&
    recycleBaseValue.value > 0 &&
    recycleMaxQuantity.value > 0 &&
    recycleCoefficient.value > 0 &&
    recycleMatchKind.value !== "not_recyclable"
  )
})
const canConfirmRecycle = computed(() => {
  return (
    canShowRecycle.value &&
    recycleQuantity.value >= 1 &&
    recycleQuantity.value <= recycleMaxQuantity.value
  )
})
const canShowDestructiveRecycle = computed(() =>
  canOperateCurrentInstance.value && !!currentInstance.value && isDestructiveRecycleCandidate(currentInstance.value)
)
watch(
  () => [
    props.active,
    props.instanceId,
    detail.value?.instance?.instanceId ?? 0,
    detail.value?.instance?.state || "",
    hasInventoryInstance(detail.value?.instance?.instanceId || 0)
  ] as const,
  ([active, requestedInstanceId, loadedInstanceId, state, inInventory]) => {
    if (!active) return
    if (!requestedInstanceId || loadedInstanceId !== requestedInstanceId) return
    if (state === "inventory" && !inInventory) {
      detail.value = null
      closeActionDialogs()
      emit("request-close")
    }
  }
)
const canOperateCurrentInstance = computed(() => {
  const instanceId = currentInstanceId()
  if (!instanceId) return false
  return hasInventoryInstance(instanceId)
})
const showEquipTargetSelectInDialog = computed(() =>
  props.operationTargetType === "auto" && operationTargets.value.length > 1
)
const equipStatusRequirements = computed(() => {
  const requirements = getEquipmentTemplateDefinition(currentItemId.value)?.statusRequirement
  return requirements && typeof requirements === "object" ? requirements : {}
})
const operationTargets = computed(() => {
  const targets: Array<{ key: string; label: string }> = [
    { key: "player", label: t("game.itemOperationTargetPlayer") }
  ]
  for (const kitty of playerStore.kitties || []) {
    const kittyID = Number(kitty.id || 0)
    if (!Number.isFinite(kittyID) || kittyID <= 0) continue
    const kittyName = String(kitty.name || "").trim() || `#${kittyID}`
    targets.push({
      key: `kitty:${kittyID}`,
      label: t("game.itemOperationTargetKitty", { name: kittyName })
    })
  }
  return targets
})
const kittyTargets = computed(() => operationTargets.value.filter((target) => target.key.startsWith("kitty:")))
const operationTargetDialogTitle = computed(() => {
  const action = operationTargetDialogAction.value === "skillbook"
    ? t("game.itemOperationTargetActionSkillbook")
    : t("game.itemOperationTargetActionEquip")
  return t("game.itemOperationTargetDialogTitle", { action })
})
const currentEquipTarget = computed<OperationTarget | null>(() =>
  resolvePreferredOperationTarget() || parseOperationTargetKey(String(equipTargetKey.value || "").trim())
)
const equipTargetAttributes = computed<Record<string, number>>(() => {
  if (currentEquipTarget.value?.type === "kitty") {
    return roleStateStore.roleUnitAttributes(`kitty:${currentEquipTarget.value.kittyId}` as RoleKey)
  }
  return roleStateStore.roleUnitAttributes("player:self")
})
const displayRequirementAttributes = computed(() => props.requirementAttributes ?? equipTargetAttributes.value)
const equipRequirementRows = computed(() => {
  const rows: Array<{ attr: string; name: string; current: number; required: number; met: boolean }> = []
  for (const [attr, requiredRaw] of Object.entries(equipStatusRequirements.value)) {
    const required = Number(requiredRaw || 0)
    if (!Number.isFinite(required) || required <= 0) continue
    const current = Number(equipTargetAttributes.value[attr] || 0)
    const attrName = t(`attr.${attr}.name`)
    rows.push({
      attr,
      name: attrName === `attr.${attr}.name` ? attr : attrName,
      current,
      required,
      met: current >= required
    })
  }
  return rows
})

watch(
  () => [props.active, props.itemId, props.instanceId, store.token] as const,
  async ([active]) => {
    if (!active || !store.token) return
    await loadDetail()
  },
  { immediate: true }
)

watch(
  operationTargets,
  (targets) => {
    if (targets.some((target) => target.key === operationTargetDialogSelection.value)) return
    operationTargetDialogSelection.value = "player"
  },
  { immediate: true }
)

watch(
  operationEffectTargetOptions,
  (targets) => {
    if (targets.some((target) => target.key === operationEffectTargetKey.value)) return
    operationEffectTargetKey.value = targets[0]?.key || ""
  },
  { immediate: true }
)

watch(
  equipSlots,
  (slots) => {
    if (slots.includes(equipSlot.value)) return
    equipSlot.value = slots[0] || ""
  },
  { immediate: true }
)

watch(
  operationTargets,
  (targets) => {
    if (targets.some((target) => target.key === equipTargetKey.value)) return
    equipTargetKey.value = "player"
  },
  { immediate: true }
)

watch(
  operationTargets,
  (targets) => {
    if (targets.some((target) => target.key === skillbookTargetKey.value)) return
    skillbookTargetKey.value = "player"
  },
  { immediate: true }
)

watch(
  () => props.active,
  (active) => {
    if (!active) {
      closeActionDialogs()
    }
  }
)

watch(
  () => [equipDialogVisible.value, equipTargetKey.value, props.operationTargetType, props.operationTargetKittyId] as const,
  async ([visible]) => {
    if (!visible) return
    if (props.operationTargetType === "kitty" && Number(props.operationTargetKittyId || 0) > 0) {
      const kittyID = Number(props.operationTargetKittyId)
      equipTargetKey.value = `kitty:${kittyID}`
      await ensureRoleAttributes({ type: "kitty", kittyId: kittyID })
    } else if (props.operationTargetType === "player") {
      equipTargetKey.value = "player"
    }
    const target = parseOperationTargetKey(String(equipTargetKey.value || "").trim())
    if (target?.type === "kitty") {
      await ensureRoleAttributes(target)
    } else if (target?.type === "player") {
      await ensurePlayerBattleSlotTopology()
    }
  }
)

watch(
  () => recycleMaxQuantity.value,
  (maxQty) => {
    if (maxQty <= 0) {
      recycleQuantity.value = 1
      return
    }
    recycleQuantity.value = Math.min(Math.max(1, recycleQuantity.value), maxQty)
  }
)

async function loadDetail() {
  if (!store.token) return
  if (!props.itemId && !props.instanceId) return
  loading.value = true
  try {
    if (props.instanceId) {
      detail.value = await instanceDetailStore.fetchByInstance(store.token, props.instanceId, {
        itemId: props.itemId || undefined,
        force: true
      })
    } else {
      const query = new URLSearchParams()
      if (props.itemId) query.set("itemId", props.itemId)
      detail.value = await apiGet<ItemDetailResponse>(`/api/items/detail?${query.toString()}`, store.token)
    }
    const firstSlot = equipSlots.value[0]
    if (firstSlot) equipSlot.value = firstSlot
  } catch (err) {
    if (isMissingInstanceDetailError(err, props.instanceId)) {
      detail.value = null
      closeActionDialogs()
      emit("request-close")
      return
    }
    ElMessage.error(resolveErrorMessage(t, err, "game.itemDetailLoadFailed"))
  } finally {
    loading.value = false
  }
}

function isMissingInstanceDetailError(err: unknown, instanceId?: number | null): boolean {
  if (!instanceId || instanceId <= 0) return false
  if (!(err instanceof APIClientError)) return false
  return err.code === "ITEM_DETAIL_INSTANCE_NOT_FOUND"
}

function closeActionDialogs() {
  equipDialogVisible.value = false
  repairDialogVisible.value = false
  recycleDialogVisible.value = false
  destructiveRecycleVisible.value = false
  operationEffectDialogVisible.value = false
  cancelOperationTargetDialog()
}

async function openEquipDialog() {
  if (equipSubmitting.value) return
  const preferred = resolvePreferredOperationTarget()
  equipTargetKey.value = preferred?.type === "kitty" ? `kitty:${preferred.kittyId}` : "player"
  if ((preferred?.type || "player") === "player" && !await ensurePlayerBattleSlotTopology()) return
  if (equipSlots.value.length === 0 && !showEquipTargetSelectInDialog.value) {
    ElMessage.warning(t("game.specialization.error.slotDisabled"))
    return
  }
  const firstSlot = equipSlots.value[0]
  if (firstSlot) {
    equipSlot.value = firstSlot
  }
  if (preferred) {
    await ensureRoleAttributes(preferred)
  }
  if (equipSlots.value.length === 1 && firstSlot && !showEquipTargetSelectInDialog.value) {
    await equipItem(firstSlot)
    return
  }
  if (showEquipTargetSelectInDialog.value) {
    const target = parseOperationTargetKey(String(equipTargetKey.value || "").trim())
    if (target?.type === "kitty") {
      await ensureRoleAttributes(target)
    }
  }
  equipDialogVisible.value = true
}

async function ensurePlayerBattleSlotTopology(): Promise<boolean> {
  if (!store.token || getEquipmentEquipType(currentItemId.value) !== "battle" || specializationStore.summary) return true
  try {
    await specializationStore.load()
    return true
  } catch (err: unknown) {
    ElMessage.error(resolveErrorMessage(t, err, "game.specialization.loadFailed"))
    return false
  }
}

function openRepairDialog() {
  repairDialogVisible.value = true
}

function openRenameDialog() {
  if (!canRename.value) return
  renameDialogVisible.value = true
}

async function handleItemRenamed() {
  emit("updated")
  await loadDetail()
}

function currentInstanceId(): number | null {
  return detail.value?.instance?.instanceId ?? null
}

async function equipItem(slot: string) {
  const instanceId = currentInstanceId()
  if (!store.token || !instanceId) return
  if (equipSubmitting.value) return
  if (!slot) {
    ElMessage.warning(t("game.equipSlotRequired"))
    return
  }
  if (!hasInventoryInstance(instanceId)) {
    ElMessage.warning(t("game.itemOperationInstanceNotInInventory"))
    return
  }
  equipSubmitting.value = true
  try {
    const equipType = getEquipmentEquipType(detail.value?.item.id || "")
    const target = (equipDialogVisible.value
      ? parseOperationTargetKey(String(equipTargetKey.value || "").trim())
      : await resolveOperationTarget("equip")) || resolvePreferredOperationTarget()
    if (!target) {
      ElMessage.warning(t("game.itemOperationTargetInvalid"))
      return
    }
    if (target.type === "kitty") {
      const result = await apiPost<RoleMutationSyncPayload>(`/api/cattery/kitties/${encodeURIComponent(String(target.kittyId))}/equip`, store.token, {
        instanceId,
        slot,
        equipType
      })
      applyRoleMutationSyncPayload(result)
    } else {
      await apiPost("/api/equipment/equip", store.token, {
        instanceId,
        slot,
        equipType
      })
    }
    ElMessage.success(t("game.equipSuccess"))
    equipDialogVisible.value = false
    emit("updated")
    emit("request-close")
  } catch (err) {
    ElMessage.error(resolveErrorMessage(t, err, "game.equipFailed"))
  } finally {
    equipSubmitting.value = false
  }
}

async function unequipItem() {
  const instanceId = currentInstanceId()
  if (!store.token || !instanceId) return
  try {
    if (props.operationTargetType === "kitty" && Number(props.operationTargetKittyId || 0) > 0) {
      const result = await apiPost<RoleMutationSyncPayload>(
        `/api/cattery/kitties/${encodeURIComponent(String(props.operationTargetKittyId))}/unequip-instance`,
        store.token,
        { instanceId }
      )
      applyRoleMutationSyncPayload(result)
    } else {
      await apiPost("/api/equipment/unequip-instance", store.token, { instanceId })
    }
    ElMessage.success(t("game.unequipSuccess"))
    emit("updated")
    emit("request-close")
    if (props.operationTargetType !== "kitty") {
      await loadDetail()
    }
  } catch (err) {
    ElMessage.error(resolveErrorMessage(t, err, "game.unequipFailed"))
  }
}

async function useSkillbook() {
  const instanceId = currentInstanceId()
  if (!store.token || !instanceId || !canUseSkillbook.value) return
  if (!hasInventoryInstance(instanceId)) {
    ElMessage.warning(t("game.itemOperationInstanceNotInInventory"))
    return
  }
  skillbookUseCount.value = 1
  const preferred = resolvePreferredOperationTarget()
  if (preferred) {
    skillbookTargetKey.value = preferred.type === "kitty" ? `kitty:${preferred.kittyId}` : "player"
  } else if (!operationTargets.value.some((t) => t.key === skillbookTargetKey.value)) {
    skillbookTargetKey.value = operationTargets.value[0]?.key || "player"
  }
  skillbookUseDialogVisible.value = true
}

async function learnForgedSkillbook(targetType: "player" | "kitty") {
  const instanceId = currentInstanceId()
  if (!store.token || !instanceId || !forgedSkillBook.value) return
  const kittyID = targetType === "kitty" ? Number(forgedKittyKey.value.split(":")[1] || 0) : 0
  if (targetType === "kitty" && kittyID <= 0) return
  try {
    await apiPost("/api/items/use-skillbook", store.token, { instanceId, targetType, targetKittyId: kittyID, count: 1 })
    ElMessage.success(t("skillForge.book.learned"))
    await inventoryStore.refreshAll(store.token)
    emit("updated")
    emit("request-close")
  } catch (err) {
    ElMessage.error(resolveErrorMessage(t, err, "game.useSkillbookFailed"))
  }
}

async function destroyForgedSkillbook() {
  const instanceId = currentInstanceId()
  if (!store.token || !instanceId || !forgedSkillBook.value) return
  await ElMessageBox.confirm(t("skillForge.book.destroyConfirm"), t("skillForge.book.destroy"), { type: "warning" })
  try {
    await apiPost("/api/items/forged-skillbook/destroy", store.token, { instanceId })
    ElMessage.success(t("skillForge.book.destroyed"))
    await inventoryStore.refreshAll(store.token)
    emit("updated")
    emit("request-close")
  } catch (err) {
    ElMessage.error(resolveErrorMessage(t, err, "game.itemOperationFailed"))
  }
}

async function confirmUseSkillbook() {
  const instanceId = currentInstanceId()
  if (!store.token || !instanceId || !canUseSkillbook.value) return
  const count = Math.max(1, Math.min(skillbookUseCount.value, skillbookUseMaxCount.value))
  skillbookUseDialogVisible.value = false
  const preferred = resolvePreferredOperationTarget()
  const target = preferred ?? parseOperationTargetKey(String(skillbookTargetKey.value || "").trim())
  if (!target) {
    ElMessage.warning(t("game.itemOperationTargetInvalid"))
    return
  }
  try {
    const result = await apiPost<{
      skillId: string
      gainedSkill: boolean
      gainedExp: number
      level: number
      exp: number
    }>("/api/items/use-skillbook", store.token, {
      instanceId,
      targetType: target.type,
      targetKittyId: target.type === "kitty" ? target.kittyId : 0,
      count
    })
    if (result.gainedSkill && result.gainedExp > 0) {
      ElMessage.success(t("game.useSkillbookLearnedAndExp", { exp: result.gainedExp }))
    } else if (result.gainedSkill) {
      ElMessage.success(t("game.useSkillbookLearned"))
    } else {
      ElMessage.success(t("game.useSkillbookGainedExp", { exp: result.gainedExp }))
    }
    emit("updated")
    if (getInventoryQuantity(currentItemId.value) <= 0) {
      emit("request-close")
      return
    }
    await loadDetail()
  } catch (err) {
    ElMessage.error(resolveErrorMessage(t, err, "game.useSkillbookFailed"))
  }
}

async function useCollectible() {
  const instanceId = currentInstanceId()
  if (!store.token || !instanceId || !canUseCollectible.value) return
  try {
    const result = await apiPost<{ collectibleId: string; newLevel: number; wasUpgraded: boolean }>(
      "/api/items/use-collectible",
      store.token,
      { instanceId }
    )
    if (result.wasUpgraded) {
      ElMessage.success(t("game.upgradeCollectibleSuccess", { level: result.newLevel }))
    } else {
      ElMessage.success(t("game.useCollectibleSuccess"))
    }
    emit("updated")
    emit("request-close")
  } catch (err) {
    ElMessage.error(resolveErrorMessage(t, err, "game.useCollectibleFailed"))
  }
}

function localizedOperationEffectText(key: string | undefined, fallback: string): string {
  const normalized = String(key || "").trim()
  return normalized && te(normalized) ? t(normalized) : fallback
}

function formatOperationEffectPercent(raw: unknown): string {
  const value = Math.max(0, Number(raw || 0) * 100)
  return value.toFixed(2).replace(/\.00$/, "").replace(/(\.\d)0$/, "$1")
}

async function openOperationEffectDialog() {
  if (!canShowUseOperationEffect.value || operationEffectSubmitting.value) return
  if (!operationEffectTargetOptions.value.some((target) => target.key === operationEffectTargetKey.value)) {
    operationEffectTargetKey.value = operationEffectTargetOptions.value[0]?.key || ""
  }
  operationEffectDialogVisible.value = true
  operationEffectQuantity.value = 1
  operationEffectPreviewLoading.value = true
  try {
    await operationEffects.refresh(store.token)
  } catch (err) {
    ElMessage.error(resolveErrorMessage(t, err, "game.operationEffects.useFailed"))
  } finally {
    operationEffectPreviewLoading.value = false
  }
}

function selectedOperationEffectTarget(): OperationEffectActivationTarget | null {
  const selected = operationEffectTargetOptions.value.find((target) => target.key === operationEffectTargetKey.value)
  if (!selected) return null
  const targetKind: OperationEffectTargetKind = selected.targetKind
  return {
    targetKind,
    targetActorId: targetKind === "kitty" ? selected.targetActorId : undefined
  }
}

async function confirmUseOperationEffect() {
  const instanceId = currentInstanceId()
  const target = selectedOperationEffectTarget()
  if (!store.token || !instanceId || !target || !canShowUseOperationEffect.value) {
    ElMessage.warning(t("game.operationEffects.targetInvalid"))
    return
  }
  if (operationEffectSubmitting.value) return
  if (!operationEffectPreviewReady.value || !operationEffectServingPreview.value.valid) return
  const quantity = operationEffectQuantity.value
  const requestSignature = `${instanceId}:${target.targetKind}:${target.targetActorId || 0}:${quantity}`
  if (operationEffectPendingRequest.value?.signature !== requestSignature) {
    operationEffectPendingRequest.value = {
      signature: requestSignature,
      requestId: createIdempotencyKey("operation-effect")
    }
  }
  const requestId = operationEffectPendingRequest.value.requestId
  operationEffectSubmitting.value = true
  try {
    await operationEffects.activate(store.token, instanceId, target, requestId, quantity)
    operationEffectPendingRequest.value = null
    operationEffectDialogVisible.value = false
    ElMessage.success(t("game.operationEffects.useSuccess"))
    emit("updated")
    emit("request-close")
    void inventoryStore.refreshAll(store.token).catch((error) => {
      console.warn("[operation-effects] inventory refresh after consuming food failed", error)
    })
  } catch (err) {
    ElMessage.error(resolveErrorMessage(t, err, "game.operationEffects.useFailed"))
    void operationEffects.refresh(store.token).catch(() => undefined)
  } finally {
    operationEffectSubmitting.value = false
  }
}

async function repairItem() {
  const instanceId = currentInstanceId()
  if (!store.token || !instanceId) return
  try {
    await apiPost("/api/equipment/repair", store.token, { instanceId })
    ElMessage.success(t("game.repairSuccess"))
    repairDialogVisible.value = false
    emit("updated")
    await loadDetail()
  } catch (err) {
    ElMessage.error(resolveErrorMessage(t, err, "game.repairFailed"))
  }
}

function openRecycleDialog() {
  if (!canShowRecycle.value) return
  recycleQuantity.value = Math.min(Math.max(1, recycleQuantity.value), Math.max(1, recycleMaxQuantity.value))
  recycleDialogVisible.value = true
}

function onDestructiveRecycleDone() {
  emit("updated")
  emit("request-close")
}

async function recycleItem(quantity: number) {
  const instanceId = currentInstanceId()
  recycleQuantity.value = Math.min(Math.max(1, Math.floor(quantity || 1)), Math.max(1, recycleMaxQuantity.value))
  if (!store.token || !instanceId || !canConfirmRecycle.value) return
  try {
    const result = await apiPost<{
      ok: true
      recycledQuantity: number
      unitPrice: number
      totalGold: number
      instanceRemoved: boolean
    }>("/api/items/recycle", store.token, { instanceId, quantity: recycleQuantity.value })
    ElMessage.success(t("game.recycleSuccess", { quantity: result.recycledQuantity, gold: result.totalGold }))
    recycleDialogVisible.value = false
    emit("updated")
    emit("request-close")
  } catch (err) {
    ElMessage.error(resolveErrorMessage(t, err, "game.recycleFailed"))
  }
}

function setOperationTarget(): { instanceId: number; templateItemId: string } | null {
  const instanceId = currentInstanceId()
  if (!instanceId || !detail.value) return null
  const target = {
    instanceId,
    templateItemId: currentItemId.value
  }
  equipmentStore.setOperationTarget(target)
  emit("request-close")
  return target
}

function gotoExtract() {
  if (!setOperationTarget()) return
  void router.push({ path: "/game/affix", query: { tab: "detach" } })
}

function gotoEnhance() {
  const target = setOperationTarget()
  if (!target) return
  void router.push({ path: "/game/enhance", query: { instanceId: String(target.instanceId) } })
}

function gotoEmbed() {
  if (!setOperationTarget()) return
  void router.push({ path: "/game/affix", query: { tab: "attach" } })
}

function gotoEnchant() {
  if (!setOperationTarget()) return
  void router.push({ path: "/game/affix", query: { tab: "enchant" } })
}

function gotoAlchemyConvert() {
  if (!canShowAlchemyConvert.value) return
  const target = setOperationTarget()
  if (!target) return
  void router.push({ path: "/game/alchemy", query: { tab: "convert", instanceId: String(target.instanceId) } })
}

function gotoAlchemyTransmute() {
  if (!canShowAlchemyTransmute.value) return
  const target = setOperationTarget()
  if (!target) return
  void router.push({ path: "/game/alchemy", query: { tab: "transmute", instanceId: String(target.instanceId) } })
}

function gotoMarket() {
  if (!canShowMarket.value) return
  if (!currentItemId.value) return
  emit("request-close")
  void router.push({ path: "/game/market", query: { tab: "trade", itemId: currentItemId.value } })
}

function parseOperationTargetKey(key: string): OperationTarget | null {
  if (!key || key === "player") {
    return { type: "player" }
  }
  if (!key.startsWith("kitty:")) {
    return null
  }
  const kittyID = Number(key.slice("kitty:".length))
  if (!Number.isFinite(kittyID) || kittyID <= 0) {
    return null
  }
  const exists = (playerStore.kitties || []).some((kitty) => Number(kitty.id || 0) === kittyID)
  if (!exists) {
    return null
  }
  return { type: "kitty", kittyId: kittyID }
}

function resolvePreferredOperationTarget(): OperationTarget | null {
  const type = String(props.operationTargetType || "auto").trim()
  if (type === "player") {
    return { type: "player" }
  }
  if (type === "kitty") {
    const kittyID = Number(props.operationTargetKittyId || 0)
    if (Number.isFinite(kittyID) && kittyID > 0) {
      return { type: "kitty", kittyId: kittyID }
    }
    return null
  }
  return null
}

function confirmOperationTargetDialog() {
  if (!operationTargetDialogResolver) return
  const target = parseOperationTargetKey(String(operationTargetDialogSelection.value || "").trim())
  const resolve = operationTargetDialogResolver
  operationTargetDialogResolver = null
  operationTargetDialogVisible.value = false
  resolve(target)
}

function cancelOperationTargetDialog() {
  if (!operationTargetDialogResolver) {
    operationTargetDialogVisible.value = false
    return
  }
  const resolve = operationTargetDialogResolver
  operationTargetDialogResolver = null
  operationTargetDialogVisible.value = false
  resolve(null)
}

async function chooseOperationTarget(action: "equip" | "skillbook"): Promise<OperationTarget | null> {
  if (operationTargets.value.length <= 1) {
    return { type: "player" }
  }
  operationTargetDialogAction.value = action
  if (!operationTargets.value.some((target) => target.key === operationTargetDialogSelection.value)) {
    operationTargetDialogSelection.value = operationTargets.value[0]?.key || "player"
  }
  return await new Promise<OperationTarget | null>((resolve) => {
    operationTargetDialogResolver = resolve
    operationTargetDialogVisible.value = true
  })
}

async function ensureRoleAttributes(target: OperationTarget): Promise<void> {
  if (!store.token) return
  if (target.type === "player") {
    roleStateStore.ensureRole("player:self")
    return
  }
  const roleKey = `kitty:${target.kittyId}` as RoleKey
  if (roleStateStore.ensureRole(roleKey).initialized.attributes) return
  try {
    await roleStateStore.ensureRoleInitialized(roleKey, store.token)
  } catch (err) {
    ElMessage.error(resolveErrorMessage(t, err, "game.itemOperationTargetLoadFailed"))
  }
}

function applyRoleMutationSyncPayload(payload: RoleMutationSyncPayload | null | undefined) {
  roleStateStore.patchRoleMutationPayload(payload)
}

async function resolveOperationTarget(action: "equip" | "skillbook"): Promise<OperationTarget | null> {
  const preferred = resolvePreferredOperationTarget()
  if (preferred) return preferred
  const type = String(props.operationTargetType || "auto").trim()
  if (type === "player" || type === "kitty") return null
  return await chooseOperationTarget(action)
}
</script>

<style scoped>
.item-detail-panel {
  min-width: 0;
}

.operation-effect-use {
  display: grid;
  gap: 14px;
}

.operation-effect-use__summary {
  display: grid;
  grid-template-columns: 46px minmax(0, 1fr);
  gap: 11px;
  align-items: start;
}

.operation-effect-use__icon {
  display: inline-flex;
  width: 46px;
  height: 46px;
  align-items: center;
  justify-content: center;
  border: 1px solid color-mix(in srgb, var(--app-success) 36%, var(--app-border));
  border-radius: 7px;
  background: color-mix(in srgb, var(--app-success) 10%, var(--app-surface));
  color: var(--app-success);
  font-size: 22px;
}

.operation-effect-use__summary strong {
  color: var(--app-text);
  font-size: 15px;
  line-height: 1.4;
  overflow-wrap: anywhere;
}

.operation-effect-use__summary p {
  margin: 4px 0 0;
  color: var(--app-text-secondary);
  font-size: 12px;
  line-height: 1.55;
  overflow-wrap: anywhere;
}

.operation-effect-use__lifetime {
  display: flex;
  align-items: center;
  gap: 7px;
  padding: 9px 10px;
  border-block: 1px solid var(--app-border);
  color: var(--app-primary);
  font-size: 13px;
}

.operation-effect-use__lifetime svg {
  flex: 0 0 auto;
}

.operation-effect-use__modifiers {
  display: grid;
  gap: 7px;
  padding: 0 2px;
}

.operation-effect-use__modifiers > div {
  display: flex;
  align-items: flex-start;
  gap: 7px;
  color: var(--app-text-secondary);
  font-size: 12px;
  line-height: 1.5;
}

.operation-effect-use__modifiers svg {
  flex: 0 0 auto;
  margin-top: 2px;
  color: var(--app-success);
}

.operation-effect-use__target {
  display: grid;
  gap: 7px;
}

.operation-effect-use__target > span {
  color: var(--app-muted);
  font-size: 12px;
}

.forged-book-detail { display: grid; gap: 12px; padding: 12px; border: 1px solid var(--app-border); background: var(--app-surface-elevated); }
.forged-book-detail__meta { display: grid; grid-template-columns: minmax(0, 1fr) auto; gap: 6px 12px; font-size: 12px; }
.forged-book-detail__meta span { color: var(--app-text-muted); }
.forged-book-detail__actions { display: grid; gap: 8px; }
.forged-book-detail__kitty { display: grid; grid-template-columns: minmax(0, 1fr) auto; gap: 8px; }

@media (max-width: 560px) {
  .forged-book-detail__kitty { grid-template-columns: 1fr; }
}
</style>
