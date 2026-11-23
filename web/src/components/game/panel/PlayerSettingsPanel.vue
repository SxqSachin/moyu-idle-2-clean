<template>
  <div class="w-full" data-tutorial="settings-page">
    <el-tabs v-model="activeTab" data-tutorial="settings-tabs">
      <el-tab-pane :label="t('settings.usernameTitle')" name="username">
        <el-form label-position="top" class="mt-3 user-setting-form">
          <el-form-item :label="t('settings.usernameTitle')">
            <el-input v-model="username" :placeholder="t('settings.usernamePlaceholder')" @input="handleUsernameInput" />
          </el-form-item>
          <div class="-mt-2 mb-4 space-y-3 text-xs">
            <p class="app-muted leading-5">{{ t("settings.usernameRenamePublicHint") }}</p>
            <div
              v-if="usernameConflict"
              class="flex items-center justify-between gap-3 border-l-2 border-[var(--el-color-warning)] bg-[var(--el-color-warning-light-9)] px-3 py-2"
            >
              <span class="leading-5">{{ t("settings.usernameChangedElsewhere") }}</span>
              <el-button link type="primary" class="shrink-0" @click="useLatestUsername">
                {{ t("settings.usernameUseLatest") }}
              </el-button>
            </div>
            <div class="flex flex-wrap items-center justify-between gap-2 border-t border-[var(--app-border)] pt-3">
              <span class="app-muted">{{ t("settings.usernameRenameCost") }}</span>
              <QuickRefWithRequire
                :item-id="PLAYER_USERNAME_RENAME_COST_ITEM_ID"
                :required-count="PLAYER_USERNAME_RENAME_COST_ITEM_COUNT"
              />
            </div>
            <p v-if="inventoryStore.initialized && !hasUsernameRenameMaterial" class="text-[var(--el-color-danger)]">
              {{ t("settings.error.usernameInsufficientItem") }}
            </p>
          </div>
          <div class="flex justify-end">
            <el-button type="primary" :loading="updatingUsername" :disabled="!canSubmitUsername" @click="submitUsername">
              {{ t("settings.saveUsername") }}
            </el-button>
          </div>
        </el-form>
      </el-tab-pane>

      <el-tab-pane :label="t('settings.passwordTitle')" name="password">
        <el-form label-position="top" class="mt-3 user-setting-form">
          <el-form-item :label="t('settings.currentPasswordPlaceholder')">
            <el-input v-model="currentPassword" show-password type="password" :placeholder="t('settings.currentPasswordPlaceholder')" />
          </el-form-item>
          <el-form-item :label="t('settings.newPasswordPlaceholder')">
            <el-input v-model="newPassword" show-password type="password" :placeholder="t('settings.newPasswordPlaceholder')" />
          </el-form-item>
          <el-form-item :label="t('settings.confirmPasswordPlaceholder')">
            <el-input v-model="confirmPassword" show-password type="password" :placeholder="t('settings.confirmPasswordPlaceholder')" />
          </el-form-item>
          <div class="flex justify-end">
            <el-button type="primary" :loading="updatingPassword" @click="submitPassword">
              {{ t("settings.savePassword") }}
            </el-button>
          </div>
        </el-form>
      </el-tab-pane>

      <el-tab-pane :label="t('settings.emailTitle')" name="email">
        <el-form label-position="top" class="mt-3 user-setting-form">
          <el-form-item :label="t('settings.emailTitle')">
            <el-input v-model="email" :placeholder="t('settings.emailPlaceholder')" />
          </el-form-item>
          <div id="email-geetest-reg" class="mb-4"></div>
          <div class="flex items-center justify-between gap-3">
            <div class="text-xs app-muted">
              {{ emailVerified ? t("settings.emailStatusVerified") : t("settings.emailStatusPending") }}
            </div>
            <el-button type="primary" :loading="updatingEmail" :disabled="!emailCaptchaReady" @click="submitEmail">
              {{ t("settings.sendEmailVerifyCode") }}
            </el-button>
          </div>
          <template v-if="emailVerificationPending">
            <el-divider class="my-4!" />
            <el-form-item :label="t('settings.emailVerifyCode')">
              <el-input v-model="emailVerifyCode" :placeholder="t('settings.emailVerifyCodePlaceholder')" />
            </el-form-item>
            <div class="flex justify-end">
              <el-button type="primary" :loading="confirmingEmail" @click="confirmEmail">
                {{ t("settings.confirmEmail") }}
              </el-button>
            </div>
          </template>
        </el-form>
      </el-tab-pane>

      <el-tab-pane :label="t('settings.personalizationTitle')" name="personalization">
        <div class="mt-3 flex flex-col gap-6">
          <!-- <el-form label-position="top" class="user-setting-form w-72">
            <el-form-item :label="t('settings.languageTitle')">
              <el-select v-model="language" class="w-full">
                <el-option v-for="option in languageOptions" :key="option.value" :label="option.label" :value="option.value" />
              </el-select>
            </el-form-item>
            <div class="flex justify-end">
              <el-button type="primary" :loading="updatingLanguage" @click="submitLanguage">
                {{ t("settings.saveLanguage") }}
              </el-button>
            </div>
          </el-form> -->

          <el-divider class="my-0!" />

          <el-form label-position="top" class="user-setting-form w-full max-w-md">
            <el-form-item>
              <div class="flex items-center justify-between gap-4 w-full">
                <div>
                  <div class="text-sm">{{ t("settings.simpleMode") }}</div>
                  <div class="text-xs app-muted mt-1">{{ t("settings.simpleModeHint") }}</div>
                </div>
                <el-switch
                  v-model="simpleMode"
                  :loading="updatingSimpleMode"
                  @change="(v: boolean) => submitSimpleMode(v)"
                />
              </div>
            </el-form-item>
          </el-form>

          <el-divider class="my-0!" />

          <el-form label-position="top" class="user-setting-form w-72">
            <el-form-item :label="t('settings.menuLayoutTitle')">
              <el-radio-group v-model="menuLayout">
                <el-radio v-for="option in menuLayoutOptions" :key="option.value" :value="option.value">
                  {{ option.label }}
                </el-radio>
              </el-radio-group>
            </el-form-item>
            <div class="flex justify-end">
              <el-button type="primary" :loading="updatingMenuLayout" @click="submitMenuLayout">
                {{ t("settings.saveMenuLayout") }}
              </el-button>
            </div>
          </el-form>

          <!-- <el-divider class="my-0!" />

          <el-form label-position="top">
            <el-form-item :label="t('settings.skinTitle')">
              <SkinSelector @update:skin="(skin: ThemeSkin) => setSkin(skin)" />
            </el-form-item>
          </el-form> -->

          <el-divider class="my-0!" />

          <el-form label-position="top" class="user-setting-form w-full max-w-md" data-tutorial="settings-title">
            <div class="grid grid-cols-2 gap-3">
              <el-form-item :label="t('settings.titlePrefixLabel')">
                <el-select v-model="selectedPrefixId" class="w-full" :placeholder="t('settings.titleNone')" clearable filterable :loading="loadingTitleWardrobe">
                  <el-option
                    v-for="item in ownedPrefixes"
                    :key="item.itemId"
                    :label="item.label"
                    :value="item.itemId"
                  />
                </el-select>
              </el-form-item>
              <el-form-item :label="t('settings.titleSuffixLabel')">
                <el-select v-model="selectedSuffixId" class="w-full" :placeholder="t('settings.titleNone')" clearable filterable :loading="loadingTitleWardrobe">
                  <el-option
                    v-for="item in ownedSuffixes"
                    :key="item.itemId"
                    :label="item.label"
                    :value="item.itemId"
                  />
                </el-select>
              </el-form-item>
            </div>
            <div class="mb-3 rounded-md bg-[var(--app-surface-elevated)] p-2 text-center">
              <div class="text-xs text-[var(--app-muted)]">{{ t('settings.titlePreview') }}</div>
              <div class="mt-1 text-base font-semibold">{{ displayTitle || t('settings.titleNone') }}</div>
            </div>
            <div class="flex justify-end">
              <el-button type="primary" :loading="savingTitle" @click="submitTitle">
                {{ t("settings.saveTitle") }}
              </el-button>
            </div>
          </el-form>

          <template v-if="emailVerified">
            <el-divider class="my-0!" />

            <el-form label-position="top" class="user-setting-form w-72">
              <el-form-item>
                <div class="flex items-center justify-between gap-4 w-full">
                  <div>
                    <div class="text-sm">{{ t("settings.disableIPLoginVerify") }}</div>
                    <div class="text-xs app-muted mt-1">{{ t("settings.disableIPLoginVerifyHint") }}</div>
                  </div>
                  <el-switch
                    v-model="disableIPLoginVerify"
                    :loading="updatingIPLoginVerify"
                    @change="(v: boolean) => submitIPLoginVerify(v)"
                  />
                </div>
              </el-form-item>
            </el-form>
          </template>
        </div>
      </el-tab-pane>
    </el-tabs>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref, watch } from "vue"
import { ElMessage, ElMessageBox } from "element-plus"
import { useI18n } from "vue-i18n"
import { useRoute } from "vue-router"
import { APIClientError } from "@services/api"
import { parseAuthErrorResponse, resolveAuthError } from "@services/auth"
import { resolveErrorMessage } from "@services/error-messages"
import {
  fetchPlayerSettings,
  updateDisableIPLoginVerify,
  updatePlayerLanguage,
  updatePlayerMenuLayout,
  updatePlayerPassword,
  updatePlayerSimpleMode,
  updatePlayerUsername,
  reconcilePlayerUsernameDraft,
  type PlayerLanguage,
  type PlayerMenuLayout
} from "@services/player-settings"
import { confirmPlayerEmailVerification, requestPlayerEmailVerification } from "@services/auth-recovery"
import { useGameStore } from "@stores/game"
import { useInventoryStore } from "@stores/inventory"
import { useLocalUserStore } from "@stores/local-user"
import { GameFullLocaleNamespaces, getLocaleNamespacesForPath, getLocaleOptions, normalizeSupportedLocale, setAppLocale } from "@/i18n"
import { switchAppLocale } from "@services/locale-switch"
import { useSkin } from "@/composables/useSkin"
import type { ThemeSkin } from "@/themes/registry"
import SkinSelector from "@/components/theme/SkinSelector.vue"
import type { GeetestCaptchaInstance, GeetestValidatePayload } from "@/types/geetest"
import { useWardrobeStore } from "@stores/wardrobe"
import { getGameDataTable } from "@services/game-data-runtime"
import { getLocalizedItemName } from "@services/item-label"
import { useInventorySummary } from "@composables/game/useInventorySummary"
import QuickRefWithRequire from "@components/game/item/QuickRefWithRequire.vue"
import {
  PLAYER_USERNAME_RENAME_COST_ITEM_COUNT,
  PLAYER_USERNAME_RENAME_COST_ITEM_ID
} from "@/config/shared-global-config"

const store = useGameStore()
const inventoryStore = useInventoryStore()
const localUserStore = useLocalUserStore()
const route = useRoute()
const { t } = useI18n()
const { set: setSkin } = useSkin()
const { getInventoryQuantity } = useInventorySummary()
const languageOptions = getLocaleOptions()
const menuLayoutOptions = computed(() => [
  { value: "drawer" as const, label: t("settings.menuLayoutDrawer") },
  { value: "icon-rail" as const, label: t("settings.menuLayoutIconRail") }
])

const username = ref("")
const usernameBaseline = ref("")
const usernameConflict = ref(false)
const normalizedUsername = computed(() => username.value.trim())
const renameCostItemName = computed(() =>
  getLocalizedItemName(t, PLAYER_USERNAME_RENAME_COST_ITEM_ID, { fallbackText: PLAYER_USERNAME_RENAME_COST_ITEM_ID })
)
const hasUsernameRenameMaterial = computed(
  () => getInventoryQuantity(PLAYER_USERNAME_RENAME_COST_ITEM_ID) >= PLAYER_USERNAME_RENAME_COST_ITEM_COUNT
)
const canSubmitUsername = computed(
  () =>
    Boolean(store.token) &&
    normalizedUsername.value.length > 0 &&
    normalizedUsername.value !== store.username.trim() &&
    hasUsernameRenameMaterial.value &&
    !usernameConflict.value &&
    !updatingUsername.value
)
const email = ref("")
const emailVerified = ref(false)
const emailVerificationPending = ref(false)
const emailVerifyCode = ref("")
const emailCaptchaReady = ref(false)
const emailCaptchaObj = ref<GeetestCaptchaInstance | null>(null)
const confirmingEmail = ref(false)
const language = ref<PlayerLanguage>("zh-CN")
const menuLayout = ref<PlayerMenuLayout>("drawer")
const activeTab = ref("username")
const currentPassword = ref("")
const newPassword = ref("")
const confirmPassword = ref("")
const updatingUsername = ref(false)
const updatingPassword = ref(false)
const updatingEmail = ref(false)
const updatingLanguage = ref(false)
const updatingMenuLayout = ref(false)
const simpleMode = ref(false)
const updatingSimpleMode = ref(false)
const disableIPLoginVerify = ref(false)
const updatingIPLoginVerify = ref(false)

// Title management
const wardrobeStore = useWardrobeStore()
const selectedPrefixId = ref("")
const selectedSuffixId = ref("")
const savingTitle = ref(false)
const loadingTitleWardrobe = ref(false)
let titleWardrobeLoadSeq = 0

type TitleEntry = { id: string; nameKey: string; rarity: string; category?: string; order: number; disabled: boolean }
let allPrefixesCache: TitleEntry[] | null = null
let allSuffixesCache: TitleEntry[] | null = null

function allPrefixes() {
  if (!allPrefixesCache) {
    allPrefixesCache = getGameDataTable<TitleEntry>("title_prefixes").filter((e) => !e.disabled)
  }
  return allPrefixesCache
}

function allSuffixes() {
  if (!allSuffixesCache) {
    allSuffixesCache = getGameDataTable<TitleEntry>("title_suffixes").filter((e) => !e.disabled)
  }
  return allSuffixesCache
}

const ownedPrefixes = computed(() =>
  wardrobeStore.owned
    .filter((o) => o.category === "title_prefix")
    .map((o) => ({
      itemId: o.itemId,
      label: allPrefixes().find((p) => p.id === o.itemId)?.nameKey
        ? t(allPrefixes().find((p) => p.id === o.itemId)!.nameKey)
        : o.itemId
    }))
)

const ownedSuffixes = computed(() =>
  wardrobeStore.owned
    .filter((o) => o.category === "title_suffix")
    .map((o) => ({
      itemId: o.itemId,
      label: allSuffixes().find((s) => s.id === o.itemId)?.nameKey
        ? t(allSuffixes().find((s) => s.id === o.itemId)!.nameKey)
        : o.itemId
    }))
)

const displayTitle = computed(() => {
  const parts: string[] = []
  if (selectedPrefixId.value) {
    const entry = allPrefixes().find((p) => p.id === selectedPrefixId.value)
    if (entry) {
      const translated = t(entry.nameKey)
      if (translated && translated !== entry.nameKey) parts.push(translated)
    }
  }
  if (selectedSuffixId.value) {
    const entry = allSuffixes().find((s) => s.id === selectedSuffixId.value)
    if (entry) {
      const translated = t(entry.nameKey)
      if (translated && translated !== entry.nameKey) parts.push(translated)
    }
  }
  return parts.join("")
})

async function submitTitle() {
  if (!store.token) return
  savingTitle.value = true
  try {
    const currentPrefix = wardrobeStore.playerTitlePrefix
    const currentSuffix = wardrobeStore.playerTitleSuffix
    // Handle prefix change
    if (selectedPrefixId.value !== currentPrefix) {
      if (selectedPrefixId.value) {
        await wardrobeStore.equip(store.token, "title_prefix", selectedPrefixId.value)
      } else {
        await wardrobeStore.unequip(store.token, "title_prefix")
      }
    }
    // Handle suffix change
    if (selectedSuffixId.value !== currentSuffix) {
      if (selectedSuffixId.value) {
        await wardrobeStore.equip(store.token, "title_suffix", selectedSuffixId.value)
      } else {
        await wardrobeStore.unequip(store.token, "title_suffix")
      }
    }
    selectedPrefixId.value = wardrobeStore.playerTitlePrefix
    selectedSuffixId.value = wardrobeStore.playerTitleSuffix
    ElMessage.success(t("settings.titleSaveSuccess"))
  } catch (err) {
    ElMessage.error(resolveErrorMessage(t, err, "settings.titleSaveFailed"))
  } finally {
    savingTitle.value = false
  }
}

onMounted(async () => {
  syncUsernameDraft(store.username)
  language.value = normalizeSupportedLocale(store.playerLanguage)
  menuLayout.value = store.playerMenuLayout
  simpleMode.value = store.playerSimpleMode
  activeTab.value = resolveSettingsTab(route.query.subTab)
  void initEmailCaptcha()
})

watch(
  () => route.query.subTab,
  (value) => {
    activeTab.value = resolveSettingsTab(value)
  }
)

watch(
  () => store.username,
  (remoteUsername) => {
    const reconciled = reconcilePlayerUsernameDraft(username.value, usernameBaseline.value, remoteUsername)
    username.value = reconciled.draft
    usernameBaseline.value = reconciled.baseline
    usernameConflict.value = reconciled.conflict
  },
  { immediate: true }
)

watch(
  () => store.token,
  (token, previousToken) => {
    if (!token || token === previousToken) return
    void refreshSettings()
    void refreshTitleWardrobe(token)
  },
  { immediate: true }
)

async function refreshTitleWardrobe(token = store.token) {
  if (!token) return
  const seq = ++titleWardrobeLoadSeq
  loadingTitleWardrobe.value = true
  try {
    await wardrobeStore.load(token)
    if (seq !== titleWardrobeLoadSeq || token !== store.token) return
    selectedPrefixId.value = wardrobeStore.playerTitlePrefix
    selectedSuffixId.value = wardrobeStore.playerTitleSuffix
  } catch (error) {
    if (seq === titleWardrobeLoadSeq) {
      console.warn("[settings] failed to load title wardrobe", error)
      ElMessage.error(resolveErrorMessage(t, error, "settings.error.requestFailed"))
    }
  } finally {
    if (seq === titleWardrobeLoadSeq) loadingTitleWardrobe.value = false
  }
}

async function refreshSettings() {
  const token = store.token
  const userId = store.userId
  if (!token || userId <= 0) return
  try {
    const payload = await fetchPlayerSettings(token)
    if (token !== store.token || userId !== store.userId) return
    const usernameSnapshotApplied = localUserStore.applyProfileUsernameSnapshot(
      userId,
      payload.username,
      payload.usernameChangeId
    )
    store.applyRemoteSettings({
      username: usernameSnapshotApplied ? payload.username : undefined,
      language: payload.language,
      theme: payload.theme,
      menuLayout: payload.menuLayout,
      simpleMode: payload.simpleMode,
      emailVerified: payload.emailVerified
    })
    email.value = payload.email
    emailVerified.value = Boolean(payload.emailVerified)
    emailVerificationPending.value = !payload.emailVerified && !!payload.email
    language.value = normalizeSupportedLocale(payload.language)
    menuLayout.value = payload.menuLayout
    simpleMode.value = Boolean(payload.simpleMode)
    disableIPLoginVerify.value = Boolean(payload.disableIPLoginVerify)
    await setAppLocale(payload.language, getLocaleNamespacesForPath(route.path))
  } catch (error) {
    if (token !== store.token || userId !== store.userId) return
    showAPIError(error)
  }
}

async function submitUsername() {
  if (!store.token) return
  if (!normalizedUsername.value) {
    ElMessage.error(t("settings.error.usernameRequired"))
    return
  }
  if (usernameConflict.value) {
    ElMessage.warning(t("settings.usernameChangedElsewhere"))
    return
  }
  if (!hasUsernameRenameMaterial.value) {
    ElMessage.error(
      t("settings.error.usernameInsufficientItem", {
        item: renameCostItemName.value,
        count: PLAYER_USERNAME_RENAME_COST_ITEM_COUNT
      })
    )
    return
  }
  const nextUsername = normalizedUsername.value
  const expectedCurrentUsername = usernameBaseline.value.trim()
  if (!expectedCurrentUsername || expectedCurrentUsername !== store.username.trim()) {
    ElMessage.warning(t("settings.usernameChangedElsewhere"))
    return
  }
  try {
    await ElMessageBox.confirm(
      t("settings.usernameRenameConfirm", {
        username: nextUsername,
        item: renameCostItemName.value,
        count: PLAYER_USERNAME_RENAME_COST_ITEM_COUNT
      }),
      t("settings.usernameRenameConfirmTitle"),
      {
        type: "warning",
        confirmButtonText: t("settings.usernameRenameConfirmAction"),
        cancelButtonText: t("game.cancel")
      }
    )
  } catch {
    return
  }

  if (
    usernameConflict.value ||
    normalizedUsername.value !== nextUsername ||
    usernameBaseline.value.trim() !== expectedCurrentUsername ||
    expectedCurrentUsername !== store.username.trim() ||
    nextUsername === store.username.trim()
  ) {
    ElMessage.warning(t("settings.usernameChangedElsewhere"))
    return
  }

  const token = store.token
  const userId = store.userId
  if (!token || userId <= 0) return
  updatingUsername.value = true
  let shouldRefreshInventory = false
  try {
    const payload = await updatePlayerUsername(token, nextUsername, expectedCurrentUsername)
    const applied = localUserStore.applyProfileChanged(
      userId,
      payload.username,
      payload.changeId,
      payload.previousUsername
    )
    if (token === store.token && userId === store.userId) {
      if (applied) {
        store.applyRemoteSettings({ username: payload.username })
        syncUsernameDraft(payload.username)
      } else {
        syncUsernameDraft(store.username)
      }
      shouldRefreshInventory = true
      ElMessage.success(t("settings.success.usernameUpdated"))
    }
  } catch (error) {
    if (error instanceof APIClientError && error.code === "PROFILE_USERNAME_CURRENT_CONFLICT") {
      ElMessage.error(t("settings.error.usernameCurrentConflict"))
      await refreshSettings()
    } else {
      showAPIError(error)
    }
  } finally {
    updatingUsername.value = false
  }

  if (!shouldRefreshInventory) return
  try {
    await inventoryStore.refreshAll(token)
  } catch (error) {
    console.warn("[settings] username changed but inventory refresh failed", error)
    ElMessage.warning(t("settings.inventoryRefreshPending"))
  }
}

function syncUsernameDraft(value: string) {
  const normalized = String(value || "").trim()
  username.value = normalized
  usernameBaseline.value = normalized
  usernameConflict.value = false
}

function handleUsernameInput() {
  if (!usernameConflict.value) return
  usernameBaseline.value = store.username.trim()
  usernameConflict.value = false
}

function useLatestUsername() {
  syncUsernameDraft(store.username)
}

async function submitPassword() {
  if (!store.token) return
  if (!currentPassword.value || !newPassword.value) {
    ElMessage.error(t("settings.error.passwordRequired"))
    return
  }
  if (newPassword.value !== confirmPassword.value) {
    ElMessage.error(t("settings.error.passwordConfirmMismatch"))
    return
  }
  updatingPassword.value = true
  try {
    const payload = await updatePlayerPassword(store.token, currentPassword.value, newPassword.value)
    store.setAuth(payload.userId, payload.token, store.username)
    currentPassword.value = ""
    newPassword.value = ""
    confirmPassword.value = ""
    ElMessage.success(t("settings.success.passwordUpdated"))
  } catch (error) {
    showAPIError(error)
  } finally {
    updatingPassword.value = false
  }
}

async function submitEmail() {
  if (!store.token) return
  if (!email.value.trim()) {
    ElMessage.error(t("settings.error.emailRequired"))
    return
  }
  if (!emailCaptchaReady.value) {
    ElMessage.error(t("auth.error.captchaUnavailable"))
    return
  }
  updatingEmail.value = true
  try {
    const captchaToken = await verifyEmailCaptchaAndIssueToken()
    const payload = await requestPlayerEmailVerification(store.token, email.value.trim(), captchaToken)
    email.value = payload.email
    emailVerified.value = payload.emailVerified
    store.setEmailVerified(payload.emailVerified)
    emailVerificationPending.value = true
    emailVerifyCode.value = ""
    ElMessage.success(t("settings.success.emailVerifyCodeSent"))
  } catch (error) {
    showAuthError(error)
  } finally {
    resetEmailCaptcha()
    updatingEmail.value = false
  }
}

async function confirmEmail() {
  if (!store.token) return
  if (!emailVerifyCode.value.trim()) {
    ElMessage.error(t("settings.error.emailVerifyCodeRequired"))
    return
  }
  confirmingEmail.value = true
  try {
    const payload = await confirmPlayerEmailVerification(store.token, emailVerifyCode.value.trim())
    email.value = payload.email
    emailVerified.value = payload.emailVerified
    store.setEmailVerified(payload.emailVerified)
    emailVerificationPending.value = false
    emailVerifyCode.value = ""
    ElMessage.success(t("settings.success.emailVerified"))
  } catch (error) {
    showAuthError(error)
  } finally {
    confirmingEmail.value = false
  }
}

async function submitLanguage() {
  if (!store.token) return
  updatingLanguage.value = true
  try {
    const nextLocale = normalizeSupportedLocale(language.value)
    await switchAppLocale(nextLocale, {
      namespaces: GameFullLocaleNamespaces,
      persistLocal: true,
      syncRemote: async (locale) => {
        const payload = await updatePlayerLanguage(store.token, locale)
        store.setPlayerLanguage(payload.language)
        language.value = normalizeSupportedLocale(payload.language)
        return payload.language
      }
    })
  } catch {
    // switchAppLocale already reports the failure and rolls back local state.
  } finally {
    updatingLanguage.value = false
  }
}

async function submitMenuLayout() {
  if (!store.token) return
  updatingMenuLayout.value = true
  try {
    const payload = await updatePlayerMenuLayout(store.token, menuLayout.value)
    store.setPlayerMenuLayout(payload.menuLayout)
    menuLayout.value = payload.menuLayout
    ElMessage.success(t("settings.success.menuLayoutUpdated"))
  } catch (error) {
    showAPIError(error)
  } finally {
    updatingMenuLayout.value = false
  }
}

async function submitSimpleMode(value: boolean) {
  if (!store.token) return
  updatingSimpleMode.value = true
  try {
    const payload = await updatePlayerSimpleMode(store.token, value)
    simpleMode.value = payload.simpleMode
    store.setPlayerSimpleMode(payload.simpleMode)
    ElMessage.success(t("settings.success.simpleModeUpdated"))
  } catch (error) {
    showAPIError(error)
    simpleMode.value = !value
  } finally {
    updatingSimpleMode.value = false
  }
}

async function submitIPLoginVerify(value: boolean) {
  if (!store.token) return
  updatingIPLoginVerify.value = true
  try {
    const payload = await updateDisableIPLoginVerify(store.token, value)
    disableIPLoginVerify.value = payload.disableIPLoginVerify
    ElMessage.success(t("settings.success.disableIPLoginVerifyUpdated"))
  } catch (error) {
    showAPIError(error)
    disableIPLoginVerify.value = !value
  } finally {
    updatingIPLoginVerify.value = false
  }
}

function showAPIError(error: unknown) {
  if (error instanceof APIClientError && error.code) {
    const key = errorCodeToI18nKey[error.code]
    if (key) {
      ElMessage.error(t(key))
      return
    }
  }
  ElMessage.error(resolveErrorMessage(t, error, "settings.error.requestFailed"))
}

function showAuthError(error: unknown) {
  if (error instanceof Error && error.message.startsWith("auth.")) {
    ElMessage.error(t(error.message))
    return
  }
  showAPIError(error)
}

async function verifyEmailCaptchaAndIssueToken(): Promise<string> {
  if (!emailCaptchaObj.value) {
    throw new Error("auth.error.captchaUnavailable")
  }
  const validate = emailCaptchaObj.value.getValidate()
  if (!validate) {
    throw new Error("auth.error.captchaRequired")
  }
  const data = await submitEmailCaptchaValidateRequest(validate)
  const token = String(data?.captchaToken || "")
  if (!token) {
    throw new Error("auth.error.captchaUnavailable")
  }
  return token
}

async function submitEmailCaptchaValidateRequest(payload: GeetestValidatePayload) {
  const res = await fetch("/api/geetest/validate", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(payload)
  })
  if (!res.ok) {
    const parsed = await parseAuthErrorResponse(res)
    const resolved = resolveAuthError(parsed)
    throw new Error(resolved.key)
  }
  const raw = await res.json()
  return raw?.data ?? raw
}

function resetEmailCaptcha() {
  emailCaptchaObj.value?.reset()
}

async function initEmailCaptcha() {
  if (typeof window.initGeetest !== "function") {
    emailCaptchaReady.value = false
    ElMessage.error(t("auth.error.captchaUnavailable"))
    return
  }
  const registerRes = await fetch(`/api/geetest/register?t=${Date.now()}`, { method: "GET" })
  if (!registerRes.ok) {
    emailCaptchaReady.value = false
    ElMessage.error(t("auth.error.captchaUnavailable"))
    return
  }
  const payload = await registerRes.json()
  const container = document.getElementById("email-geetest-reg")
  if (container) {
    container.innerHTML = ""
  }
  window.initGeetest(
    {
      gt: payload.gt,
      challenge: payload.challenge,
      new_captcha: Boolean(payload.new_captcha),
      offline: !payload.success,
      product: "float",
      width: "100%"
    },
    (instance) => {
      emailCaptchaObj.value = instance
      instance.appendTo("#email-geetest-reg")
      emailCaptchaReady.value = true
    }
  )
}

const errorCodeToI18nKey: Record<string, string> = {
  PROFILE_USERNAME_TOO_SHORT: "settings.error.usernameTooShort",
  PROFILE_USERNAME_TOO_LONG: "settings.error.usernameTooLong",
  PROFILE_USERNAME_INVALID_FORMAT: "settings.error.usernameInvalidFormat",
  PROFILE_USERNAME_BLOCKED: "settings.error.usernameBlocked",
  PROFILE_USERNAME_EXISTS: "settings.error.usernameExists",
  PROFILE_USERNAME_UNCHANGED: "settings.error.usernameUnchanged",
  PROFILE_USERNAME_CURRENT_CONFLICT: "settings.error.usernameCurrentConflict",
  PROFILE_USERNAME_INSUFFICIENT_ITEM: "settings.error.usernameInsufficientItem",
  PROFILE_PASSWORD_TOO_SHORT: "settings.error.passwordTooShort",
  PROFILE_PASSWORD_SAME_AS_OLD: "settings.error.passwordSameAsOld",
  PROFILE_PASSWORD_INVALID_CURRENT: "settings.error.passwordInvalidCurrent",
  PROFILE_EMAIL_INVALID: "settings.error.emailInvalid",
  PROFILE_EMAIL_EXISTS: "settings.error.emailExists",
  PROFILE_EMAIL_UNCHANGED: "settings.error.emailUnchanged",
  AUTH_EMAIL_DOMAIN_BLOCKED: "auth.error.emailDomainBlocked",
  SETTINGS_INVALID_LANGUAGE: "settings.error.invalidLanguage",
  SETTINGS_INVALID_MENU_LAYOUT: "settings.error.invalidMenuLayout",
  SETTINGS_READ_FAILED: "settings.error.requestFailed",
  SETTINGS_UPDATE_FAILED: "settings.error.requestFailed",
  PROFILE_UPDATE_FAILED: "settings.error.requestFailed",
}

function resolveSettingsTab(raw: unknown): string {
  const value = typeof raw === "string" ? raw : ""
  if (value === "username" || value === "password" || value === "email" || value === "personalization") {
    return value
  }
  return "username"
}
</script>

<style scoped>

.user-setting-form {
  padding: 1em;
  max-width: 26em;
  margin: auto;
}
</style>
