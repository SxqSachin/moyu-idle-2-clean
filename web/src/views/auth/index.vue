<template>
  <div class="page-shell flex items-center justify-center p-6">
    <el-card class="app-card w-full max-w-[460px]">
      <template #header>
        <div class="flex items-start justify-between gap-4">
          <div class="space-y-1">
            <div class="text-xl font-semibold">{{ t("login.title") }}</div>
            <div class="app-muted text-sm">{{ t("login.subtitle") }}</div>
          </div>
          <el-dropdown trigger="click" @command="changeLocale">
            <el-button circle>
              <el-icon><Icon icon="material-symbols:translate" /></el-icon>
            </el-button>
            <template #dropdown>
              <el-dropdown-menu>
                <el-dropdown-item
                  v-for="option in languageOptions"
                  :key="option.value"
                  :command="option.value"
                  :disabled="option.value === currentLocale"
                >
                  {{ option.label }}
                </el-dropdown-item>
              </el-dropdown-menu>
            </template>
          </el-dropdown>
        </div>
      </template>

      <template v-if="ipVerifyMode">
        <div class="space-y-4">
          <div class="text-sm app-muted">{{ t("auth.ipVerify.hint") }}</div>
          <el-form label-position="top" @submit.prevent>
            <el-form-item :label="t('auth.ipVerify.codeLabel')">
              <el-input
                v-model="ipVerifyCode"
                :placeholder="t('auth.ipVerify.codePlaceholder')"
                maxlength="6"
                @keyup.enter="submitIPVerify"
              />
            </el-form-item>
          </el-form>
          <div class="flex gap-2">
            <el-button class="flex-1" :loading="submittingIPVerify" type="primary" @click="submitIPVerify">
              {{ t("auth.ipVerify.submit") }}
            </el-button>
            <el-button @click="cancelIPVerify">{{ t("auth.ipVerify.back") }}</el-button>
          </div>
        </div>
      </template>

      <template v-else>
        <el-tabs v-model="activeMode">
          <el-tab-pane :label="t('login.login')" name="login">
            <el-form label-position="top" @submit.prevent>
              <el-form-item :label="t('login.username')">
                <el-input v-model="loginUsername" :placeholder="t('login.usernamePlaceholder')" />
              </el-form-item>
              <el-form-item :label="t('login.password')">
                <el-input v-model="loginPassword" type="password" show-password :placeholder="t('login.passwordPlaceholder')" />
              </el-form-item>
              <div class="flex justify-end">
                <el-link type="primary" @click.prevent="goToForgotPassword">{{ t("login.forgotPassword") }}</el-link>
              </div>
            </el-form>
          </el-tab-pane>
          <el-tab-pane :label="t('login.register')" name="register">
            <el-form label-position="top" @submit.prevent>
              <el-form-item :label="t('login.username')">
                <el-input v-model="registerUsername" :placeholder="t('login.usernamePlaceholder')" />
              </el-form-item>
              <el-form-item :label="t('login.email')">
                <el-input v-model="registerEmail" :placeholder="t('login.emailPlaceholder')" />
              </el-form-item>
              <el-form-item :label="t('login.password')">
                <el-input v-model="registerPassword" type="password" show-password :placeholder="t('login.passwordPlaceholder')" />
              </el-form-item>
              <el-form-item>
                <el-checkbox v-model="acceptedTerms">
                  <span>{{ t("login.acceptTermsPrefix") }}</span>
                  <el-link class="mx-1" type="primary" @click.prevent="openInstructions">{{ t("login.acceptTermsLink") }}</el-link>
                  <span>{{ t("login.acceptTermsSuffix") }}</span>
                </el-checkbox>
              </el-form-item>
            </el-form>
          </el-tab-pane>
        </el-tabs>
        <div id="geetest-reg" class="mb-4"></div>
        <el-button
          v-if="activeMode === 'login'"
          type="primary"
          class="w-full"
          :loading="submitting"
          :disabled="!captchaReady"
          @click="submitLogin"
        >
          {{ t("login.login") }}
        </el-button>
        <el-button
          v-else
          type="primary"
          class="w-full"
          :loading="submitting"
          :disabled="!captchaReady"
          @click="submitRegister"
        >
          {{ t("login.register") }}
        </el-button>
        <div class="mt-4 text-xs app-muted">
          {{ t("login.rulesHint") }}
        </div>
      </template>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import "@/style.css"
import "element-plus/theme-chalk/el-message.css"
import { computed, nextTick, onMounted, ref } from "vue"
import { useRoute, useRouter } from "vue-router"
import { useI18n } from "vue-i18n"
import { ElMessage } from "element-plus"
import { Icon } from "@iconify/vue"
import { useGameStore } from "@stores/game"
import { parseAuthErrorResponse, resolveAuthError, validateAuthInput, validateRegisterInput } from "@services/auth"
import { markOnboardingPromptPending } from "@services/onboarding-prompt"
import { collectDeviceProfile } from "@services/risk-device"
import { getLocaleNamespacesForPath, getLocaleOptions, normalizeSupportedLocale } from "@/i18n"
import { switchAppLocale } from "@services/locale-switch"
import type { GeetestCaptchaInstance, GeetestValidatePayload } from "@/types/geetest"

const route = useRoute()
const activeMode = ref<"login" | "register">(route.query.mode === "register" ? "register" : "login")
const loginUsername = ref(typeof route.query.username === "string" ? route.query.username : "")
const loginPassword = ref("")
const registerUsername = ref("")
const registerEmail = ref("")
const registerPassword = ref("")
const acceptedTerms = ref(false)
const submitting = ref(false)
const captchaReady = ref(false)
const captchaObj = ref<GeetestCaptchaInstance | null>(null)
const router = useRouter()
const store = useGameStore()
const { t, locale } = useI18n()
const languageOptions = getLocaleOptions()

const ipVerifyMode = ref(false)
const ipVerifyPendingToken = ref("")
const ipVerifyCode = ref("")
const submittingIPVerify = ref(false)

const currentLocale = computed(() => normalizeSupportedLocale(locale.value))

function showError(i18nKey: string, debug = "") {
  const inDev = import.meta.env.DEV || import.meta.env.VITE_APP_ENV === "development"
  const text = inDev && debug ? `${t(i18nKey)} (${debug})` : t(i18nKey)
  ElMessage.error(text)
}

async function submitLogin() {
  const validationError = validateAuthInput(loginUsername.value, loginPassword.value)
  if (validationError) {
    showError(validationError)
    return
  }
  submitting.value = true
  try {
    const captchaToken = await verifyCaptchaAndIssueToken()
    const deviceProfile = await collectDeviceProfile()
    const data = await submitAuthRequest("/api/login", {
      username: loginUsername.value.trim(),
      password: loginPassword.value,
      captchaToken,
      deviceProfile
    })
    store.setAuth(data.userId, data.token, loginUsername.value.trim())
    ElMessage.success(t("auth.success.login"))
    router.push("/game")
  } catch (error) {
    if (error instanceof Error && error.message.startsWith("auth.ipVerify|")) {
      const pendingToken = error.message.slice("auth.ipVerify|".length)
      ipVerifyPendingToken.value = pendingToken
      ipVerifyCode.value = ""
      ipVerifyMode.value = true
      ElMessage.info(t("auth.ipVerify.hint"))
    } else if (error instanceof Error && error.message.startsWith("auth.error.accountBanned|")) {
      showAccountBannedError(error.message.slice("auth.error.accountBanned|".length))
    } else if (error instanceof Error && error.message.startsWith("auth.")) {
      showError(error.message)
    } else {
      const detail = error instanceof Error ? error.message : ""
      showError("auth.error.networkFailed", detail)
    }
  } finally {
    resetCaptcha()
    submitting.value = false
  }
}

async function submitIPVerify() {
  const code = ipVerifyCode.value.trim()
  if (!code) {
    showError("auth.ipVerify.codeRequired")
    return
  }
  submittingIPVerify.value = true
  try {
    const res = await fetch("/api/login/ip-verify", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ pendingToken: ipVerifyPendingToken.value, code })
    })
    if (!res.ok) {
      const payload = await parseAuthErrorResponse(res)
      const resolved = resolveAuthError(payload)
      showError(resolved.key)
      return
    }
    const raw = await res.json()
    const data = raw?.data ?? raw
    store.setAuth(data.userId, data.token, loginUsername.value.trim())
    ElMessage.success(t("auth.ipVerify.success"))
    router.push("/game")
  } catch (error) {
    const detail = error instanceof Error ? error.message : ""
    showError("auth.error.networkFailed", detail)
  } finally {
    submittingIPVerify.value = false
  }
}

function cancelIPVerify() {
  ipVerifyMode.value = false
  ipVerifyPendingToken.value = ""
  ipVerifyCode.value = ""
  captchaReady.value = false
  captchaObj.value = null
  nextTick(() => {
    initCaptcha().catch(() => {})
  })
}

async function submitRegister() {
  const validationError = validateRegisterInput(registerUsername.value, registerPassword.value, registerEmail.value)
  if (validationError) {
    showError(validationError)
    return
  }
  if (!acceptedTerms.value) {
    showError("auth.error.termsRequired")
    return
  }
  submitting.value = true
  try {
    const captchaToken = await verifyCaptchaAndIssueToken()
    const deviceProfile = await collectDeviceProfile()
    const data = await submitAuthRequest("/api/register", {
      username: registerUsername.value.trim(),
      email: registerEmail.value.trim().toLowerCase(),
      password: registerPassword.value,
      acceptedTerms: acceptedTerms.value,
      acceptedTermsVersion: "v1",
      captchaToken,
      deviceProfile
    })
    store.setAuth(data.userId, data.token, registerUsername.value.trim())
    markOnboardingPromptPending(data.userId)
    ElMessage.success(t("auth.success.register"))
    router.push("/game")
  } catch (error) {
    if (error instanceof Error && error.message.startsWith("auth.")) {
      showError(error.message)
    } else {
      const detail = error instanceof Error ? error.message : ""
      showError("auth.error.networkFailed", detail)
    }
  } finally {
    resetCaptcha()
    submitting.value = false
  }
}

async function verifyCaptchaAndIssueToken(): Promise<string> {
  if (!captchaObj.value) {
    throw new Error("auth.error.captchaUnavailable")
  }
  const validate = captchaObj.value.getValidate()
  if (!validate) {
    throw new Error("auth.error.captchaRequired")
  }
  const data = await submitCaptchaValidateRequest(validate)
  const token = String(data?.captchaToken || "")
  if (!token) {
    throw new Error("auth.error.captchaUnavailable")
  }
  return token
}

async function submitCaptchaValidateRequest(payload: GeetestValidatePayload) {
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

async function submitAuthRequest(endpoint: string, body: Record<string, unknown>) {
  const res = await fetch(endpoint, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body)
  })
  if (!res.ok) {
    const payload = await parseAuthErrorResponse(res)
    const resolved = resolveAuthError(payload)
    if (resolved.key === "auth.error.accountBanned") {
      throw new Error(`auth.error.accountBanned|${resolved.detail || ""}`)
    }
    if (resolved.key === "auth.error.loginIPVerifyRequired") {
      throw new Error(`auth.ipVerify|${resolved.detail || ""}`)
    }
    throw new Error(resolved.key)
  }
  const raw = await res.json()
  return raw?.data ?? raw
}

function showAccountBannedError(detailRaw: string) {
  const detail = String(detailRaw || "").trim()
  if (!detail) {
    showError("auth.error.accountBannedPermanent")
    return
  }
  const [reasonRaw = "", banUntilRaw = ""] = detail.split("|", 2).map((part) => part.trim())
  const reason = resolveBanReasonLabel(reasonRaw)
  if (!banUntilRaw) {
    if (reason) {
      ElMessage.error(t("auth.error.accountBannedPermanentWithReason", { reason }))
      return
    }
    showError("auth.error.accountBannedPermanent")
    return
  }
  const date = new Date(banUntilRaw)
  if (Number.isNaN(date.getTime())) {
    if (reason) {
      ElMessage.error(t("auth.error.accountBannedPermanentWithReason", { reason }))
      return
    }
    showError("auth.error.accountBannedPermanent")
    return
  }
  const normalizedLocale = normalizeSupportedLocale(locale.value)
  const localizedUntil = date.toLocaleString(normalizedLocale === "zh-CN" ? "zh-CN" : "en-US", {
    hour12: false
  })
  if (reason) {
    ElMessage.error(t("auth.error.accountBannedUntilWithReason", { reason, until: localizedUntil }))
    return
  }
  ElMessage.error(t("auth.error.accountBannedUntil", { until: localizedUntil }))
}

function resolveBanReasonLabel(raw: string): string {
  const reasonKey = String(raw || "").trim().toLowerCase()
  if (!reasonKey) {
    return ""
  }
  const i18nKey = `auth.banReason.${reasonKey}`
  const translated = t(i18nKey)
  if (translated === i18nKey) {
    return reasonKey
  }
  return translated
}

function openInstructions() {
  window.open("/instructions", "_blank")
}

function goToForgotPassword() {
  router.push({ path: "/auth/forgot-password", query: { username: loginUsername.value.trim() || undefined } })
}

async function changeLocale(value: string) {
  await switchAppLocale(value, {
    namespaces: getLocaleNamespacesForPath(router.currentRoute.value.path),
    persistLocal: true
  })
}

function resetCaptcha() {
  captchaObj.value?.reset()
}

async function initCaptcha() {
  if (typeof window.initGeetest !== "function") {
    captchaReady.value = false
    showError("auth.error.captchaUnavailable")
    return
  }
  const registerRes = await fetch(`/api/geetest/register?t=${Date.now()}`, { method: "GET" })
  if (!registerRes.ok) {
    throw new Error("auth.error.captchaUnavailable")
  }
  const payload = await registerRes.json()
  const container = document.getElementById("geetest-reg")
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
      captchaObj.value = instance
      instance.appendTo("#geetest-reg")
      captchaReady.value = true
    }
  )
}

onMounted(async () => {
  try {
    await initCaptcha()
  } catch (error) {
    const detail = error instanceof Error ? error.message : ""
    showError("auth.error.captchaUnavailable", detail)
  }
})
</script>
