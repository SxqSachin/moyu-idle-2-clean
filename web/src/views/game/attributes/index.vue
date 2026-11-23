<template>
  <div
    class="page-shell"
    :class="{ 'attributes-page-shell--specialization': activeRoleTab === 'specialization' && selectedRoleType === 'player' }"
  >
    <GamePageFrame
      class="attributes-page-card"
      title-key="game.menu.character"
      :tutorial-id="currentCharacterTutorialId"
      :visual-src="characterVisual.src"
      :visual-tone="characterVisual.tone"
      visual-key="character"
      data-tutorial="character-page"
    >
      <div
        class="attributes-mobile-shell space-y-3 lg:grid lg:grid-cols-[240px_minmax(0,1fr)] lg:gap-4 lg:space-y-0"
        :class="{ 'is-specialization-active': activeRoleTab === 'specialization' && selectedRoleType === 'player' }"
      >
        <div class="attributes-mobile-rolebar sticky top-0 z-20 min-h-12 rounded-lg border border-[var(--app-border)] bg-[var(--app-surface-elevated)] p-2 lg:hidden" data-tutorial="role-selector">
          <div class="flex items-center gap-2 overflow-x-auto">
            <button
              type="button"
              class="shrink-0 rounded-lg border px-2 py-1.5 text-left text-xs transition"
              :class="selectedRoleType === 'player'
                ? 'border-[var(--el-color-primary)] bg-[var(--el-color-primary-light-9)]'
                : 'border-[var(--app-border)] hover:border-[var(--el-color-primary)]'"
              @click="handleSelectPlayerRole"
            >
              <span class="inline-flex items-center gap-1">
                <span class="rounded border border-[var(--el-color-primary)] px-1 text-[10px] text-[var(--el-color-primary)]">
                  {{ t("game.playerLabel") }}
                </span>
                <span class="block max-w-20 truncate font-semibold">{{ playerRoleName }}</span>
              </span>
            </button>
            <button
              v-for="kitty in kitties"
              :key="`mobile-role-kitty-${kitty.id}`"
              type="button"
              class="flex shrink-0 items-center gap-1.5 rounded-lg border px-2 py-1.5 text-left text-xs transition"
              :class="selectedRoleType === 'kitty' && selectedKittyId === Number(kitty.id || 0)
                ? 'border-[var(--el-color-primary)] bg-[var(--el-color-primary-light-9)]'
                : 'border-[var(--app-border)] hover:border-[var(--el-color-primary)]'"
              @click="handleSelectKittyRole(Number(kitty.id || 0))"
            >
              <span class="rounded border border-[var(--el-color-warning)] px-1 text-[10px] text-[var(--el-color-warning)]">
                {{ t("game.catteryKittyLabel") }}
              </span>
              <span class="max-w-20 truncate">{{ kitty.name }}</span>
            </button>
          </div>
        </div>
        <div class="attributes-desktop-rolebar hidden space-y-3 rounded-xl border border-[var(--app-border)] bg-[var(--app-surface-elevated)] p-3 lg:block" data-tutorial="role-selector">
          <div class="text-xs app-muted">{{ t("game.attributesPlayerRoleTitle") }}</div>
          <button
            type="button"
            class="w-full rounded-lg border px-3 py-2 text-left transition"
            :class="selectedRoleType === 'player'
              ? 'border-[var(--el-color-primary)] bg-[var(--el-color-primary-light-9)]'
              : 'border-[var(--app-border)] hover:border-[var(--el-color-primary)]'"
            @click="handleSelectPlayerRole"
          >
            <div class="truncate text-sm font-semibold">{{ playerRoleName }}</div>
          </button>
          <div class="pt-1 text-xs app-muted">{{ t("game.catteryOwnedTitle") }}</div>
          <div v-if="kitties.length <= 0" class="rounded-lg border border-dashed border-[var(--app-border)] px-3 py-2 text-xs app-muted">
            {{ t("game.catteryNoKitty") }}
          </div>
          <button
            v-for="kitty in kitties"
            :key="`role-kitty-${kitty.id}`"
            type="button"
            class="flex w-full items-center gap-2 rounded-lg border px-3 py-2 text-left transition"
            :class="selectedRoleType === 'kitty' && selectedKittyId === Number(kitty.id || 0)
              ? 'border-[var(--el-color-primary)] bg-[var(--el-color-primary-light-9)]'
              : 'border-[var(--app-border)] hover:border-[var(--el-color-primary)]'"
            @click="handleSelectKittyRole(Number(kitty.id || 0))"
          >
            <span class="inline-flex h-7 w-7 shrink-0 items-center justify-center rounded-full border border-[var(--app-border)] bg-[var(--app-surface)] text-xs font-semibold">
              {{ kittyAvatar(kitty.name) }}
            </span>
            <span class="min-w-0 flex-1">
              <span class="block truncate text-sm">{{ kitty.name }}</span>
              <span class="mt-0.5 block text-xs app-muted">{{ kittyStatusLabel(String(kitty.status || "idle")) }}</span>
            </span>
          </button>
        </div>
        <div class="attributes-role-content min-w-0">
          <el-tabs
            v-model="activeRoleTab"
            class="attributes-mobile-sticky-tabs"
            :class="{ 'is-specialization-active': activeRoleTab === 'specialization' && selectedRoleType === 'player' }"
            :lazy="true"
            data-tutorial="character-tabs"
          >
            <el-tab-pane :label="t('game.attributesMainTabAttributes')" name="attributes" :lazy="true">
              <RoleStatsPanel
                v-if="selectedRoleType === 'player'"
                role-key="player:self"
                :formula="formula"
                :basic-attack="basicAttack"
                @refresh="loadStatsSafe"
                @open-allocate-dialog="openAllocateDialog"
              />
              <KittyCombatPanel
                v-else
                :token="store.token"
                :kitty-id="selectedKittyId"
                :active-tab="'attributes'"
                :embedded-tabs="false"
              />
            </el-tab-pane>
            <el-tab-pane :label="t('game.attributesMainTabEquipment')" name="equipment" :lazy="true">
              <EquipmentSlotTab
                v-if="selectedRoleType === 'player'"
                :slot-cards="slotCards"
                :equipment-type-active-tab="equipmentTypeActiveTab"
                :equipping-instance-id="equippingInstanceId"
                :equip-success-version="equipSuccessVersion"
                @equip="equipToSlot"
                @updated="refreshEquipmentAndStats"
                @update:equipment-type-active-tab="equipmentTypeActiveTab = $event"
              />
              <KittyCombatPanel
                v-else
                :token="store.token"
                :kitty-id="selectedKittyId"
                :active-tab="'equipment'"
                :embedded-tabs="false"
              />
            </el-tab-pane>
            <el-tab-pane :label="t('game.attributesMainTabSkills')" name="skills" :lazy="true">
              <AttributesSkillTab v-if="selectedRoleType === 'player'" :token="store.token" />
              <KittyCombatPanel
                v-else
                :token="store.token"
                :kitty-id="selectedKittyId"
                :active-tab="'skills'"
                :embedded-tabs="false"
              />
            </el-tab-pane>
            <el-tab-pane
              v-if="selectedRoleType === 'player'"
              class="attributes-specialization-pane"
              :label="t('game.attributesMainTabSpecialization')"
              name="specialization"
              :lazy="true"
            >
              <SpecializationTreeTab
                :active="activeRoleTab === 'specialization'"
                @changed="refreshEquipmentAndStats"
              />
            </el-tab-pane>
            <el-tab-pane v-if="selectedRoleType === 'player'" :label="t('game.ruleKindItem')" name="battle-items" :lazy="true">
              <AttributesBattleItemTab :token="store.token" />
            </el-tab-pane>
            <el-tab-pane v-if="selectedRoleType === 'player'" :label="t('game.loadoutTabTitle')" name="loadouts" :lazy="true">
              <AttributesLoadoutTab :token="store.token" />
            </el-tab-pane>
          </el-tabs>
        </div>
      </div>
    </GamePageFrame>
    <GameDialog v-model="allocateDialogVisible" :title="t('game.battlePointAllocateBatchTitle')" width="520px" data-tutorial="attribute-allocate-dialog">
      <div class="space-y-3">
        <div class="rounded-lg border border-[var(--app-border)] p-3" data-tutorial="attribute-allocate-summary">
          <div class="mb-1 text-sm font-medium">{{ t("game.battlePointTitle") }}</div>
          <div class="mb-2 text-xs app-muted">
            {{ t("game.battlePointSummary", { total: battlePointState.totalPoints, spent: battlePointState.spentPoints, available: battlePointState.available }) }}
          </div>
          <div class="flex flex-wrap items-center gap-2 text-xs">
            <QuickRefWithRequire
              v-if="battlePointState.resetRequirement.itemId"
              :item-id="battlePointState.resetRequirement.itemId"
              :required-count="battlePointState.resetRequirement.count"
            />
            <el-button :disabled="battlePointState.spentPoints <= 0 || resetSubmitting" :loading="resetSubmitting" @click="resetBattlePoints">
              {{ t("game.battlePointReset") }}
            </el-button>
          </div>
        </div>
        <div class="text-xs app-muted">
          {{ t("game.battlePointAllocateBatchSummary", { available: battlePointState.available, assigned: allocateBatchTotal, remaining: allocateBatchRemaining }) }}
        </div>
        <div class="space-y-2" data-tutorial="attribute-allocate-rows">
          <div
            v-for="row in allocatableRows"
            :key="`allocate-${row.key}`"
            class="flex items-center justify-between gap-3 rounded border border-[var(--app-border)] px-3 py-2"
          >
            <div class="flex items-center gap-2 text-sm">
              <span>{{ row.name }} · {{ row.value }}</span>
              <el-popover placement="top" trigger="hover" :width="220">
                <template #reference>
                  <button
                    type="button"
                    class="inline-flex h-4 w-4 items-center justify-center rounded-full border border-white/20 text-[10px] leading-none opacity-80 transition hover:opacity-100"
                    aria-label="Attribute per-point effect details"
                  >
                    <el-icon>
                      <Icon icon="ep:question-filled" />
                    </el-icon>
                  </button>
                </template>
                <div class="space-y-1 text-xs whitespace-pre-wrap">
                  <div class="font-semibold">{{ t("game.attrPerPointImpactTitle") }}</div>
                  <div>{{ row.impactLines.length > 0 ? row.impactLines.join("\n") : "-" }}</div>
                </div>
              </el-popover>
            </div>
            <el-input-number
              :model-value="allocateBatchValues[row.key] || 0"
              :min="0"
              :max="allocationMaxFor(row.key)"
              :step="1"
              @update:model-value="handleBatchAllocationInput(row.key, $event)"
            />
          </div>
        </div>
      </div>
      <template #footer>
        <el-button @click="closeAllocateDialog">{{ t("game.cancel") }}</el-button>
        <el-button
          type="primary"
          :disabled="allocateSubmitting || allocateBatchTotal <= 0 || allocateBatchTotal > battlePointState.available"
          :loading="allocateSubmitting"
          @click="submitAllocateBatch"
        >
          {{ t("game.battlePointAllocateSubmit") }}
        </el-button>
      </template>
    </GameDialog>

  </div>
</template>

<script setup lang="ts">
import { computed, defineAsyncComponent, nextTick, onBeforeUnmount, onMounted, ref, watch } from "vue"
import { ElMessage } from "element-plus"
import { useI18n } from "vue-i18n"
import { Icon } from "@iconify/vue"
import { apiGet, apiPost, createIdempotencyKey } from "@services/api"
import { resolveErrorMessage } from "@services/error-messages"
import { useGameStore } from "@stores/game"
import { useEquipmentStore } from "@stores/equipment"
import { useInventoryStore } from "@stores/inventory"
import { useRoleStateStore } from "@stores/role-state"
import { useSpecializationStore } from "@stores/specialization"
import { useAttributeStats } from "@/composables/game/attributes/useAttributeStats"
import { useKittiesViewModel } from "@/composables/game/cattery/useKittiesViewModel"
import { useEquipmentSlots } from "@/composables/game/attributes/useEquipmentSlots"
import { useRoleAttributeModel } from "@/composables/game/roles/useRoleAttributeModel"
import { consumeCharacterTarget } from "@/composables/game/roles/useCharacterTarget"
import { isRolePanelTab, normalizeRolePanelTabForRole } from "@/composables/game/roles/role-panel-tabs"
import { useRolePanelTabs } from "@/composables/game/roles/useRolePanelTabs"
import { useRoleSelection } from "@/composables/game/roles/useRoleSelection"
import type { RoleKey } from "@stores/role-state"
import RoleStatsPanel from "@components/game/roles/RoleStatsPanel.vue"
import AttributesSkillTab from "@components/game/attributes/AttributesSkillTab.vue"
import AttributesBattleItemTab from "@components/game/attributes/AttributesBattleItemTab.vue"
import AttributesLoadoutTab from "@components/game/attributes/AttributesLoadoutTab.vue"
import EquipmentSlotTab from "@components/game/equipmentslot/EquipmentSlotTab.vue"
import KittyCombatPanel from "@components/game/panel/cattery/KittyCombatPanel.vue"
import QuickRefWithRequire from "@components/game/item/QuickRefWithRequire.vue"
import GameDialog from "@components/game/common/GameDialog.vue"
import GamePageFrame from "@components/game/common/GamePageFrame.vue"
import { gamePageVisuals } from "@services/game-page-visuals"
import { PLAYER_FISHING_SLOT_IDS, PLAYER_WORK_SLOT_IDS } from "@services/specialization"
import type { EquipmentPanelTab, EquipmentSlotType } from "@/types/equipment"

const CHARACTER_TUTORIAL_RESET_EVENT = "character:tutorial-reset"
const CHARACTER_TUTORIAL_OPEN_ALLOCATE_EVENT = "character:tutorial-open-allocate"
const CHARACTER_TUTORIAL_OPEN_TAB_EVENT = "character:tutorial-open-tab"
const SpecializationTreeTab = defineAsyncComponent(() => import("@components/game/attributes/SpecializationTreeTab.vue"))

const { t } = useI18n()
const characterVisual = gamePageVisuals.character
const store = useGameStore()
const equipmentStore = useEquipmentStore()
const inventoryStore = useInventoryStore()
const roleStateStore = useRoleStateStore()
const specializationStore = useSpecializationStore()
const { kitties, ensureInitialized: ensureKittiesInitialized, kittyAvatar, kittyStatusLabel } = useKittiesViewModel(t)
const equipmentTypeActiveTab = ref<EquipmentPanelTab>("work")
const effectiveEquipmentType = computed<EquipmentSlotType>(() =>
  equipmentTypeActiveTab.value === "fishing" ? "work" : equipmentTypeActiveTab.value
)
const { selectedRoleType, selectedKittyId, selectPlayerRole, selectKittyRole } = useRoleSelection(kitties)
const { activeTab: activeRoleTab, normalizeForRole } = useRolePanelTabs(selectedRoleType)
const { formula, basicAttack, loadStats } = useAttributeStats()
const visibleEquipmentSlots = computed(() =>
  selectedRoleType.value !== "player"
    ? []
    : equipmentTypeActiveTab.value === "battle"
      ? specializationStore.enabledBattleSlotIds
      : equipmentTypeActiveTab.value === "fishing"
        ? [...PLAYER_FISHING_SLOT_IDS]
        : [...PLAYER_WORK_SLOT_IDS]
)
const { slotCards } = useEquipmentSlots(effectiveEquipmentType, { visibleSlots: visibleEquipmentSlots })
const equippingInstanceId = ref<number | null>(null)
const equipSuccessVersion = ref(0)
const playerRoleKey = computed(() => "player:self" as RoleKey)
const { attrRows } = useRoleAttributeModel(playerRoleKey, formula, t)
const playerRoleName = computed(() => {
  const username = String(store.username || "").trim()
  if (username) return username
  if (store.userId > 0) return `${t("game.playerLabel")} #${store.userId}`
  return t("game.playerLabel")
})
const battlePointState = ref({
  totalPoints: 0,
  spentPoints: 0,
  available: 0,
  resetRequirement: {
    itemId: "",
    count: 1
  }
})
const allocateDialogVisible = ref(false)
const allocateSubmitting = ref(false)
const resetSubmitting = ref(false)
const allocateBatchValues = ref<Record<string, number>>({})
const allocatableRows = computed(() =>
  attrRows.value.filter((row) => row.category === "battle" && row.allowBattlePointLevelUp)
)
const currentCharacterTutorialId = computed(() => {
  switch (activeRoleTab.value) {
    case "equipment":
      return "character-equipment"
    case "skills":
      return "character-skills"
    case "specialization":
      return "character-specialization"
    case "battle-items":
      return "character-battle-items"
    case "loadouts":
      return "character-loadouts"
    case "attributes":
    default:
      return "character-attributes"
  }
})
const allocateBatchTotal = computed(() =>
  Object.values(allocateBatchValues.value).reduce((sum, points) => sum + Math.max(0, Math.trunc(Number(points || 0))), 0)
)
const allocateBatchRemaining = computed(() => Math.max(0, battlePointState.value.available - allocateBatchTotal.value))

onMounted(() => {
  window.addEventListener(CHARACTER_TUTORIAL_RESET_EVENT, handleCharacterTutorialReset)
  window.addEventListener(CHARACTER_TUTORIAL_OPEN_ALLOCATE_EVENT, handleCharacterTutorialOpenAllocate)
  window.addEventListener(CHARACTER_TUTORIAL_OPEN_TAB_EVENT, handleCharacterTutorialOpenTab)
  void initializePage()
})

onBeforeUnmount(() => {
  window.removeEventListener(CHARACTER_TUTORIAL_RESET_EVENT, handleCharacterTutorialReset)
  window.removeEventListener(CHARACTER_TUTORIAL_OPEN_ALLOCATE_EVENT, handleCharacterTutorialOpenAllocate)
  window.removeEventListener(CHARACTER_TUTORIAL_OPEN_TAB_EVENT, handleCharacterTutorialOpenTab)
})

function handleSelectKittyRole(kittyId: number) {
  const switched = selectKittyRole(kittyId)
  if (!switched) return
  normalizeForRole()
}

function handleSelectPlayerRole() {
  selectPlayerRole()
  normalizeForRole()
}

function resetCharacterTutorialState() {
  closeAllocateDialog()
  selectPlayerRole()
  activeRoleTab.value = "attributes"
}

function handleCharacterTutorialReset() {
  resetCharacterTutorialState()
}

function handleCharacterTutorialOpenAllocate() {
  resetCharacterTutorialState()
  void nextTick().then(() => {
    openAllocateDialog()
  })
}

function handleCharacterTutorialOpenTab(event: Event) {
  const tab = String((event as CustomEvent<{ tab?: string }>).detail?.tab || "")
  if (!isRolePanelTab(tab)) return
  closeAllocateDialog()
  selectPlayerRole()
  activeRoleTab.value = normalizeRolePanelTabForRole(tab, "player")
}

async function initializePage() {
  store.restoreToken()
  if (store.token) {
    void inventoryStore.initialize(store.token)
    try {
      await equipmentStore.initialize(store.token)
      roleStateStore.applyEquipmentSnapshot("player:self", equipmentStore.equippedItems, equipmentStore.equippedVersion)
    } catch (err: unknown) {
      ElMessage.error(resolveErrorMessage(t, err, "game.attributesLoadFailed"))
    }
    await ensureKittiesInitialized(store.token)
    try {
      await specializationStore.load()
    } catch (err: unknown) {
      ElMessage.error(resolveErrorMessage(t, err, "game.specialization.loadFailed"))
    }
    applyCharacterTarget()
  }
  await loadStatsSafe()
}

function applyCharacterTarget() {
  const target = consumeCharacterTarget()
  if (target.targetRoleType !== "kitty" && target.targetRoleType !== "player") return

  if (isRolePanelTab(target.targetTab)) {
    activeRoleTab.value = normalizeRolePanelTabForRole(target.targetTab, target.targetRoleType)
  }

  if (target.targetRoleType === "kitty") {
    const kittyId = Math.max(0, Math.trunc(Number(target.targetKittyId || 0)))
    const exists = kitties.value.some((kitty) => Number(kitty.id || 0) === kittyId)
    if (exists) {
      selectKittyRole(kittyId)
      normalizeForRole()
      return
    }
  }

  selectPlayerRole()
  normalizeForRole()
}

async function loadStatsSafe() {
  if (!store.token) return
  try {
    await loadStats(store.token)
    await loadBattlePoints()
  } catch {
    ElMessage.error(t("game.attributesLoadFailed"))
  }
}

async function equipToSlot(payload: { slot: string; instanceId: number; equipType: "work" | "battle" }) {
  if (!store.token) return
  equippingInstanceId.value = payload.instanceId
  try {
    await apiPost("/api/equipment/equip", store.token, {
      instanceId: payload.instanceId,
      slot: payload.slot,
      equipType: payload.equipType
    })
    ElMessage.success(t("game.equipSuccess"))
    equipSuccessVersion.value += 1
    try {
      await refreshEquipmentState()
    } catch (err: unknown) {
      ElMessage.error(resolveErrorMessage(t, err, "game.attributesLoadFailed"))
    }
    await loadStatsSafe()
  } catch (err) {
    ElMessage.error(resolveErrorMessage(t, err, "game.equipFailed"))
  } finally {
    equippingInstanceId.value = null
  }
}

async function refreshEquipmentState() {
  if (!store.token) return
  await Promise.all([
    inventoryStore.initialize(store.token, true),
    equipmentStore.initialize(store.token, true)
  ])
  roleStateStore.applyEquipmentSnapshot("player:self", equipmentStore.equippedItems, equipmentStore.equippedVersion)
}

async function refreshEquipmentAndStats() {
  await refreshEquipmentState()
  await loadStatsSafe()
}

async function loadBattlePoints() {
  if (!store.token) return
  const payload = await apiGet<{
    totalPoints: number
    spentPoints: number
    available: number
    resetRequirement?: { itemId?: string; count?: number }
  }>("/api/battle/points", store.token)
  battlePointState.value = {
    totalPoints: Number(payload.totalPoints || 0),
    spentPoints: Number(payload.spentPoints || 0),
    available: Number(payload.available || 0),
    resetRequirement: {
      itemId: String(payload.resetRequirement?.itemId || "").trim(),
      count: Math.max(1, Number(payload.resetRequirement?.count || 1))
    }
  }
}

function openAllocateDialog() {
  const next: Record<string, number> = {}
  for (const row of allocatableRows.value) {
    next[row.key] = 0
  }
  allocateBatchValues.value = next
  allocateDialogVisible.value = true
}

function closeAllocateDialog() {
  allocateDialogVisible.value = false
}

function setBatchAllocation(attrKey: string, rawValue: number) {
  const normalized = Math.max(0, Math.trunc(Number(rawValue || 0)))
  const maxAllowed = allocationMaxFor(attrKey)
  allocateBatchValues.value = {
    ...allocateBatchValues.value,
    [attrKey]: Math.min(maxAllowed, normalized)
  }
}

function handleBatchAllocationInput(attrKey: string, value: number | null | undefined) {
  setBatchAllocation(attrKey, Number(value ?? 0))
}

function allocationMaxFor(attrKey: string) {
  const current = Math.max(0, Math.trunc(Number(allocateBatchValues.value[attrKey] || 0)))
  const assignedOthers = Object.entries(allocateBatchValues.value)
    .filter(([key]) => key !== attrKey)
    .reduce((sum, [, points]) => sum + Math.max(0, Math.trunc(Number(points || 0))), 0)
  const remaining = Math.max(0, battlePointState.value.available - assignedOthers)
  return current + remaining
}

async function submitAllocateBatch() {
  if (!store.token || allocateSubmitting.value) return
  const allocations = Object.entries(allocateBatchValues.value).reduce<Record<string, number>>((acc, [attrKey, points]) => {
    const normalized = Math.max(0, Math.trunc(Number(points || 0)))
    if (normalized > 0) {
      acc[attrKey] = normalized
    }
    return acc
  }, {})
  if (Object.keys(allocations).length <= 0) return
  allocateSubmitting.value = true
  try {
    await apiPost(
      "/api/battle/points/allocate_batch",
      store.token,
      { allocations },
      { "X-Idempotency-Key": createIdempotencyKey("battle-points-allocate-batch") }
    )
    await refreshEquipmentAndStats()
    closeAllocateDialog()
    ElMessage.success(t("game.saveSuccess"))
  } catch (err) {
    ElMessage.error(resolveErrorMessage(t, err, "game.saveFailed"))
  } finally {
    allocateSubmitting.value = false
  }
}

async function resetBattlePoints() {
  if (!store.token || resetSubmitting.value) return
  resetSubmitting.value = true
  try {
    await apiPost(
      "/api/battle/points/reset",
      store.token,
      {},
      { "X-Idempotency-Key": createIdempotencyKey("battle-points-reset") }
    )
    await refreshEquipmentAndStats()
    ElMessage.success(t("game.saveSuccess"))
  } catch (err) {
    ElMessage.error(resolveErrorMessage(t, err, "game.saveFailed"))
  } finally {
    resetSubmitting.value = false
  }
}

</script>

<style scoped>
:global(.game-content-scroll:has(.attributes-page-shell--specialization)) {
  overflow: hidden !important;
  overscroll-behavior: none;
}

:global(.game-route-stage:has(.attributes-page-shell--specialization)),
:global(.game-route-page:has(.attributes-page-shell--specialization)) {
  height: 100%;
  min-height: 0;
  overflow: hidden;
}

:deep(.attributes-page-card.el-card),
:deep(.attributes-page-card .el-card__body) {
  overflow: visible;
}

.attributes-page-shell--specialization {
  height: 100%;
  min-height: 0;
  overflow: hidden;
}

.attributes-page-shell--specialization :deep(.attributes-page-card.el-card),
.attributes-page-shell--specialization :deep(.attributes-page-card .el-card__body) {
  min-height: 0;
  overflow: hidden;
}

.attributes-page-shell--specialization :deep(.attributes-page-card .el-card__body) {
  display: flex;
  flex-direction: column;
}

.attributes-mobile-shell.is-specialization-active {
  display: grid;
  flex: 1 1 auto;
  grid-template-rows: auto minmax(0, 1fr);
  min-height: 0;
  overflow: hidden;
}

.attributes-mobile-shell.is-specialization-active > :not([hidden]) ~ :not([hidden]) {
  margin-top: 0;
}

.attributes-role-content,
:deep(.attributes-mobile-sticky-tabs.is-specialization-active),
:deep(.attributes-mobile-sticky-tabs.is-specialization-active > .el-tabs__content),
:deep(.attributes-specialization-pane) {
  min-height: 0;
}

.attributes-mobile-shell.is-specialization-active .attributes-role-content,
:deep(.attributes-mobile-sticky-tabs.is-specialization-active),
:deep(.attributes-specialization-pane) {
  height: 100%;
  overflow: hidden;
}

:deep(.attributes-mobile-sticky-tabs.is-specialization-active) {
  display: flex;
  flex-direction: column;
}

:deep(.attributes-mobile-sticky-tabs.is-specialization-active > .el-tabs__header) {
  flex: 0 0 auto;
}

:deep(.attributes-mobile-sticky-tabs.is-specialization-active > .el-tabs__content) {
  flex: 1 1 auto;
  overflow: hidden;
}

@media (min-width: 1024px) {
  .attributes-mobile-shell.is-specialization-active {
    grid-template-columns: 240px minmax(0, 1fr);
    grid-template-rows: minmax(0, 1fr);
  }

  .attributes-mobile-shell.is-specialization-active > .attributes-desktop-rolebar {
    min-height: 0;
    overflow-y: auto;
  }
}

@media (max-width: 1023px) {
  .attributes-mobile-shell {
    --attributes-mobile-rolebar-offset: 48px;
  }

  .attributes-mobile-rolebar {
    backdrop-filter: blur(6px);
  }

  :deep(.attributes-mobile-sticky-tabs > .el-tabs__header) {
    position: sticky;
    top: var(--attributes-mobile-rolebar-offset);
    z-index: 18;
    margin: 0 0 8px;
    background: var(--app-surface);
    border-bottom: 1px solid var(--app-border);
  }

  :deep(.attributes-mobile-sticky-tabs.is-specialization-active > .el-tabs__header) {
    position: relative;
    top: auto;
  }
}
</style>
