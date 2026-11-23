<template>
  <div class="space-y-4">
    <div class="rounded-xl border border-[var(--app-border)] bg-[var(--app-surface-elevated)] p-3">
      <div class="font-semibold">{{ t("game.catteryCapacity", { current: kitties.length, capacity: capacityDisplay }) }}</div>
      <div class="mt-1 text-xs app-muted">{{ t("game.catteryBuiltRequired") }}</div>
    </div>

    <div v-if="capacity <= 0" class="rounded-xl border border-dashed border-[var(--app-border)] p-4 app-muted">
      {{ t("game.catteryBuiltRequired") }}
    </div>

    <div v-else-if="canAdoptInUI" class="rounded-xl border border-[var(--app-border)] bg-[var(--app-surface-elevated)] p-4 space-y-3">
      <div class="font-semibold">{{ t("game.catteryAdoptTitle") }}</div>
      <div class="flex flex-wrap gap-2">
        <el-input
          v-model="adoptName"
          :maxlength="24"
          :placeholder="t('game.catteryNamePlaceholder')"
          class="min-w-[220px] flex-1"
          @keydown.enter.prevent="adoptKitty"
        />
        <el-button type="primary" :loading="adopting" @click="adoptKitty">
          {{ t("game.catteryAdoptAction") }}
        </el-button>
      </div>
    </div>

    <div v-else-if="!kitties.length" class="rounded-xl border border-dashed border-[var(--app-border)] p-4 app-muted">
      {{ t("game.catteryNoKitty") }}
    </div>

    <div v-if="kitties.length" class="grid grid-cols-1 gap-3 md:grid-cols-2">
      <div
        v-for="kitty in kitties"
        :key="kitty.id"
        class="rounded-xl border border-[var(--app-border)] bg-[var(--app-surface)] p-4 shadow-sm"
      >
        <div class="flex items-start gap-3">
          <div class="flex h-12 w-12 shrink-0 items-center justify-center rounded-xl border border-[var(--app-border)] bg-[var(--app-surface-elevated)] text-xl">
            {{ kittyAvatar(kitty.name) }}
          </div>
          <div class="min-w-0 flex-1">
            <div class="flex flex-wrap items-center gap-2">
              <div class="truncate text-base font-semibold text-[var(--app-text)]">{{ kitty.name }}</div>
              <!-- <span class="rounded-full bg-[var(--el-color-primary-light-9)] px-2 py-0.5 text-xs font-medium text-[var(--el-color-primary)]">
                {{ t("game.catteryKittyBattleLevel", { level: kitty.level || 1, cap: kitty.battleLevelCap || KITTY_BATTLE_LEVEL_CAP_DEFAULT }) }}
              </span> -->
            </div>
            <el-tag size="small">{{ kittyStatusLabel(String(kitty.status || "idle")) }}</el-tag>
            <!-- <div class="mt-1 truncate text-xs app-muted">
              #{{ kitty.id }} · {{ kitty.createdAt ? formatDate(kitty.createdAt) : "-" }}
            </div> -->
          </div>
        </div>

        <div class="mt-3">
          <div class="mb-1 flex items-center justify-between text-xs">
            <span class="app-muted">{{ t("game.catteryKittyExp") }}</span>
            <span class="font-medium">{{ kitty.exp || 0 }} / {{ kitty.nextExp || 0 }}</span>
          </div>
          <el-progress
            :percentage="expPercent(kitty.exp || 0, kitty.nextExp || 0)"
            :show-text="false"
            :stroke-width="8"
          />
        </div>

        <div class="mt-3 grid grid-cols-3 gap-2 text-xs">
          <div class="rounded-lg border border-[var(--app-border)] bg-[var(--app-surface-elevated)] px-2 py-2">
            <div class="app-muted">{{ t("game.catteryKittyBond") }}</div>
            <div class="mt-1 font-semibold">{{ kitty.bondExp || 0 }}</div>
          </div>
          <div class="rounded-lg border border-[var(--app-border)] bg-[var(--app-surface-elevated)] px-2 py-2">
            <div class="app-muted">{{ t("game.catteryKittyMood") }}</div>
            <div class="mt-1 font-semibold">{{ moodLabel(kitty.mood || 0) }}</div>
          </div>
          <div class="rounded-lg border border-[var(--app-border)] bg-[var(--app-surface-elevated)] px-2 py-2">
            <div class="app-muted">{{ t("game.catteryKittyTotalExp") }}</div>
            <div class="mt-1 font-semibold">{{ kitty.totalExp || 0 }}</div>
          </div>
        </div>

        <div class="mt-3 flex flex-wrap gap-2">
          <!-- <el-button size="small" plain @click="openKittyAttributes(Number(kitty.id || 0))">
            {{ t("game.attributesMainTabAttributes") }}
          </el-button> -->
          <!-- <el-button
            size="small"
            type="primary"
            plain
            :disabled="kittyFeedInstanceIds.length <= 0 || feedSubmitting"
            :loading="feedSubmitting && feedingKittyID === Number(kitty.id || 0)"
            @click="openFeedDialog(Number(kitty.id || 0))"
          >
            {{ t("game.catteryKittyFeedAction") }}
          </el-button> -->
        </div>
        <div class="mt-2 space-y-2">
          <div class="flex flex-row gap-2 items-center">
          <div class="text-xs app-muted">
            {{ t("game.kittyPersonalityTitle") }}
            <span v-if="kittyPersonalityCountText(kitty as Record<string, unknown>)" class="ml-1">
              {{ kittyPersonalityCountText(kitty as Record<string, unknown>) }}
            </span>
          </div>
            <el-button
              v-if="kittyPersonalityIDs(kitty as Record<string, unknown>).length > 0"
              size="small"
              @click="openPersonalityManager(Number(kitty.id || 0))"
            >
              {{ t("game.kittyPersonalityManage") }}
            </el-button>
          </div>
          <div class="flex flex-wrap gap-2">
            <template v-if="kittyPersonalityIDs(kitty as Record<string, unknown>).length > 0">
            <KittyPersonalityQuickRef
              v-for="personalityID in kittyPersonalityIDs(kitty as Record<string, unknown>)"
              :key="`${kitty.id}-${personalityID}`"
              :personality-id="personalityID"
            />
            </template>
            <template v-else>
              <div class="rounded-lg border-[var(--app-border)] py-2 text-xs app-muted">
                {{ t("game.kittyPersonalityEmpty") }}
              </div>
            </template>
          </div>
        </div>
      </div>
    </div>

    <GameDialog
      v-model="personalityDialogVisible"
      :title="t('game.kittyPersonalityManageDialogTitle', { name: managingKittyName })"
      width="560px"
      class="dialog-mobile-windowed"
      destroy-on-close
    >
      <div class="space-y-3">
        <div
          v-for="personalityID in managingKittyPersonalityIDs"
          :key="`personality-manage-${managingKittyID}-${personalityID}`"
          class="rounded-lg border border-[var(--app-border)] bg-[var(--app-surface-elevated)] p-3"
        >
          <div class="flex items-start justify-between gap-3">
            <div class="min-w-0">
              <div class="font-medium">{{ personalityLabel(personalityID) }}</div>
              <div class="mt-1 text-xs app-muted">{{ personalityDescription(personalityID) }}</div>
            </div>
            <el-popover
              placement="left"
              trigger="click"
              :width="340"
              :visible="removeConfirmKey === removeConfirmEntryKey(personalityID)"
              @update:visible="(visible: any) => handleRemoveConfirmVisible(personalityID, visible)"
            >
              <template #reference>
                <el-button
                  size="small"
                  type="danger"
                  plain
                  :loading="removingPersonalityKey === `${managingKittyID}:${personalityID}`"
                  :disabled="removingPersonalityKey === `${managingKittyID}:${personalityID}`"
                >
                  {{ t("game.kittyPersonalityRemoveAction") }}
                </el-button>
              </template>
              <div class="space-y-2 text-xs">
                <div>{{ t("game.kittyPersonalityRemoveConfirm") }}</div>
                <QuickRefWithRequire :item-id="personalityCost.itemId" :required-count="personalityCost.count" />
                <div class="flex justify-end gap-2 pt-1">
                  <el-button size="small" @click="closeRemoveConfirm()">{{ t("game.cancel") }}</el-button>
                  <el-button
                    size="small"
                    type="danger"
                    :loading="removingPersonalityKey === `${managingKittyID}:${personalityID}`"
                    :disabled="removingPersonalityKey === `${managingKittyID}:${personalityID}`"
                    @click="confirmRemovePersonality(personalityID)"
                  >
                    {{ t("game.confirm") }}
                  </el-button>
                </div>
              </div>
            </el-popover>
          </div>
        </div>
        <div v-if="managingKittyPersonalityIDs.length === 0" class="rounded border border-dashed border-[var(--app-border)] p-3 text-xs app-muted">
          {{ t("game.kittyPersonalityEmpty") }}
        </div>
      </div>
    </GameDialog>

    <InventoryInstanceSelectorDialog
      v-model="feedDialogVisible"
      :title="t('game.catteryKittyFeedDialogTitle')"
      :instance-ids="kittyFeedInstanceIds"
      :selected-instance-id="null"
      :empty-text="t('game.catteryKittyFeedEmpty')"
      @confirm="confirmFeedKitty"
    />
  </div>
</template>

<script setup lang="ts">
import { computed, ref } from "vue"
import { ElMessage } from "element-plus"
import { useI18n } from "vue-i18n"
import { useRouter } from "vue-router"
import { apiPost } from "@services/api"
import { useGameStore } from "@stores/game"
import { usePlayerStore } from "@stores/player"
import { useInventoryStore } from "@stores/inventory"
import { useKittiesViewModel } from "@/composables/game/cattery/useKittiesViewModel"
import { resolveErrorMessage } from "@services/error-messages"
import {
  KITTY_BATTLE_LEVEL_CAP_DEFAULT,
  KITTY_MAX_PERSONALITIES,
  KITTY_PERSONALITIES,
  KITTY_REMOVE_PERSONALITY_COUNT,
  KITTY_REMOVE_PERSONALITY_ITEM_ID
} from "@/config/shared-global-config"
import { getItemDefinition } from "@services/item-definition"
import KittyPersonalityQuickRef from "@/components/game/cattery/KittyPersonalityQuickRef.vue"
import { openCharacterTarget } from "@/composables/game/roles/useCharacterTarget"
import QuickRefWithRequire from "@/components/game/item/QuickRefWithRequire.vue"
import InventoryInstanceSelectorDialog from "@components/game/inventory/InventoryInstanceSelectorDialog.vue"
import GameDialog from "@components/game/common/GameDialog.vue"

type AdoptResponse = {
  kitties?: Array<{ id?: number; name?: string; createdAt?: string }>
}
type RemovePersonalityResponse = {
  kitty?: Record<string, unknown>
  kitties?: Array<Record<string, unknown>>
}
type FeedKittyResponse = {
  kitty?: Record<string, unknown>
  kitties?: Array<Record<string, unknown>>
  gain?: {
    battleLevelCapGain?: number
    battleLevelCap?: number
  }
}

const { t } = useI18n()
const router = useRouter()
const gameStore = useGameStore()
const playerStore = usePlayerStore()
const inventoryStore = useInventoryStore()
const adopting = ref(false)
const removingPersonalityKey = ref("")
const removeConfirmKey = ref("")
const feedDialogVisible = ref(false)
const feedingKittyID = ref(0)
const feedSubmitting = ref(false)
const personalityDialogVisible = ref(false)
const managingKittyID = ref(0)
const adoptName = ref("")
const personalityCost = {
  itemId: KITTY_REMOVE_PERSONALITY_ITEM_ID,
  count: KITTY_REMOVE_PERSONALITY_COUNT
}
const { kitties, capacity, capacityDisplay, canAdoptInUI, kittyAvatar, kittyStatusLabel, moodLabel, expPercent, kittyPersonalityIDs } = useKittiesViewModel(t)
const managingKitty = computed(() =>
  kitties.value.find((kitty) => Number(kitty?.id || 0) === managingKittyID.value) || null
)
const managingKittyName = computed(() => {
  const name = String(managingKitty.value?.name || "").trim()
  return name || `#${managingKittyID.value}`
})
const managingKittyPersonalityIDs = computed(() => {
  if (!managingKitty.value) return []
  return kittyPersonalityIDs(managingKitty.value as Record<string, unknown>)
})
const kittyFeedInstanceIds = computed(() =>
  inventoryStore.instances
    .filter((row) => {
      if (row.instanceId <= 0 || row.quantity <= 0 || String(row.state || "") !== "inventory") return false
      const item = getItemDefinition(String(row.templateItemId || ""))
      return Number(item?.kittyFeed?.battleLevelCapGain || 0) > 0
    })
    .map((row) => row.instanceId)
)

function openKittyAttributes(kittyId: number) {
  if (!Number.isFinite(kittyId) || kittyId <= 0) return
  openCharacterTarget({ roleType: "kitty", kittyId, tab: "attributes" })
  void router.push("/game/attributes")
}

async function adoptKitty() {
  const name = adoptName.value.trim()
  if (!name || !isValidKittyName(name)) {
    ElMessage.warning(t("game.catteryNameInvalid"))
    return
  }
  if (!gameStore.token) return
  adopting.value = true
  try {
    const payload = await apiPost<AdoptResponse>("/api/cattery/adopt", gameStore.token, { name })
    if (Array.isArray(payload?.kitties)) {
      playerStore.patchFromRequest({ kitties: payload.kitties as Record<string, unknown>[] })
    }
    adoptName.value = ""
    ElMessage.success(t("game.catteryAdoptSuccess"))
  } catch (err) {
    ElMessage.error(resolveErrorMessage(t, err, "game.catteryAdoptFailed"))
  } finally {
    adopting.value = false
  }
}

async function removeKittyPersonality(kittyID: number, personalityID: string) {
  if (!gameStore.token || kittyID <= 0 || !personalityID) return
  removingPersonalityKey.value = `${kittyID}:${personalityID}`
  try {
    const payload = await apiPost<RemovePersonalityResponse>(
      `/api/cattery/kitties/${encodeURIComponent(String(kittyID))}/personality/remove`,
      gameStore.token,
      { personalityId: personalityID }
    )
    if (Array.isArray(payload?.kitties)) {
      playerStore.patchFromRequest({ kitties: payload.kitties })
    } else {
      await playerStore.initialize(gameStore.token, true)
    }
    ElMessage.success(t("game.saveSuccess"))
  } catch (err) {
    ElMessage.error(resolveErrorMessage(t, err, "game.saveFailed"))
  } finally {
    removingPersonalityKey.value = ""
  }
}

async function confirmRemovePersonality(personalityID: string) {
  await removeKittyPersonality(managingKittyID.value, personalityID)
  closeRemoveConfirm()
}

function removeConfirmEntryKey(personalityID: string): string {
  return `${managingKittyID.value}:${personalityID}`
}

function openRemoveConfirm(personalityID: string) {
  removeConfirmKey.value = removeConfirmEntryKey(personalityID)
}

function closeRemoveConfirm() {
  removeConfirmKey.value = ""
}

function handleRemoveConfirmVisible(personalityID: string, visible: boolean) {
  if (visible) {
    openRemoveConfirm(personalityID)
    return
  }
  if (removeConfirmKey.value === removeConfirmEntryKey(personalityID)) {
    closeRemoveConfirm()
  }
}

function openPersonalityManager(kittyID: number) {
  if (kittyID <= 0) return
  managingKittyID.value = kittyID
  removeConfirmKey.value = ""
  personalityDialogVisible.value = true
}

function openFeedDialog(kittyID: number) {
  if (kittyID <= 0) return
  feedingKittyID.value = kittyID
  feedDialogVisible.value = true
}

async function confirmFeedKitty(instanceId: number | null) {
  if (!gameStore.token || feedingKittyID.value <= 0 || !instanceId) return
  feedSubmitting.value = true
  try {
    const payload = await apiPost<FeedKittyResponse>(
      `/api/cattery/kitties/${encodeURIComponent(String(feedingKittyID.value))}/feed`,
      gameStore.token,
      { instanceId: Number(instanceId) }
    )
    if (Array.isArray(payload?.kitties)) {
      playerStore.patchFromRequest({ kitties: payload.kitties })
    } else {
      await playerStore.initialize(gameStore.token, true)
    }
    await inventoryStore.initialize(gameStore.token, true)
    ElMessage.success(t("game.catteryKittyFeedSuccess"))
  } catch (err) {
    ElMessage.error(resolveErrorMessage(t, err, "game.catteryKittyFeedFailed"))
  } finally {
    feedDialogVisible.value = false
    feedSubmitting.value = false
    feedingKittyID.value = 0
  }
}

function personalityLabel(personalityID: string): string {
  const id = String(personalityID || "").trim().toLowerCase()
  const profile = KITTY_PERSONALITIES.find((row) => row.id === id)
  const key = profile?.nameKey || `kitty.personality.${id}.name`
  const localized = t(key)
  return localized === key ? id : localized
}

function personalityDescription(personalityID: string): string {
  const id = String(personalityID || "").trim().toLowerCase()
  const profile = KITTY_PERSONALITIES.find((row) => row.id === id)
  const key = profile?.descKey || `kitty.personality.${id}.desc`
  const localized = t(key)
  return localized === key ? "-" : localized
}

function kittyPersonalityCountText(kitty: Record<string, unknown>): string {
  const count = kittyPersonalityIDs(kitty).length
  if (count <= 0) return ""
  return `${count}/${KITTY_MAX_PERSONALITIES}`
}

function isValidKittyName(name: string) {
  if (name.length < 1 || name.length > 24) return false
  return /^[A-Za-z0-9_\u4E00-\u9FFF\u3040-\u30FF\uAC00-\uD7AF]+$/.test(name)
}

function formatDate(value: string) {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return date.toLocaleString()
}

</script>
