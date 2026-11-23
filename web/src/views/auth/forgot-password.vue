<template>
  <div class="page-shell flex items-center justify-center p-6">
    <el-card class="app-card w-full max-w-[460px]">
      <template #header>
        <div class="flex items-center justify-between gap-3">
          <div>
            <div class="text-xl font-semibold">{{ t("login.forgotPasswordTitle") }}</div>
            <div class="app-muted text-sm">{{ t("login.forgotPasswordEmailHint") }}</div>
          </div>
          <el-link type="primary" @click="goBackToAuth">{{ t("login.back") }}</el-link>
        </div>
      </template>
      <div class="space-y-4">
        <template v-if="step === 'email'">
          <el-input v-model="email" :placeholder="t('login.emailPlaceholder')" />
          <div id="geetest-forgot" class="mb-2"></div>
          <div class="flex justify-end">
            <el-button type="primary" :loading="submitting" :disabled="!captchaReady" @click="sendCode">
              {{ t("login.forgotPasswordSendCode") }}
            </el-button>
          </div>
        </template>
        <template v-else-if="step === 'verify'">
          <p class="text-sm app-muted">{{ t("login.forgotPasswordVerifyHint") }}</p>
          <el-input v-model="code" :placeholder="t('login.forgotPasswordCodePlaceholder')" />
          <div class="flex justify-end gap-2">
            <el-button @click="step = 'email'">{{ t("login.back") }}</el-button>
            <el-button type="primary" :loading="submitting" @click="verifyCode">
              {{ t("login.forgotPasswordVerifyCode") }}
            </el-button>
          </div>
        </template>
        <template v-else>
          <p class="text-sm app-muted">{{ t("login.forgotPasswordResetHint") }}</p>
          <el-input v-model="newPassword" type="password" show-password :placeholder="t('login.newPasswordPlaceholder')" />
          <el-input v-model="confirmPassword" type="password" show-password :placeholder="t('login.confirmPasswordPlaceholder')" />
          <div class="flex justify-end gap-2">
            <el-button @click="step = 'email'">{{ t("login.back") }}</el-button>
            <el-button type="primary" :loading="submitting" @click="resetPassword">
              {{ t("login.forgotPasswordResetSubmit") }}
            </el-button>
          </div>
        </template>
      </div>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import "@/style.css"
import "element-plus/theme-chalk/el-message.css"
import { onMounted, ref } from "vue"
import { useRoute, useRouter } from "vue-router"
import { useI18n } from "vue-i18n"
import { ElMessage } from "element-plus"
import { parseAuthErrorResponse, resolveAuthError } from "@services/auth"
import { requestForgotPasswordCode, resetForgotPassword, verifyForgotPasswordCode } from "@services/auth-recovery"
import type { GeetestCaptchaInstance, GeetestValidatePayload } from "@/types/geetest"

const route = useRoute()
const router = useRouter()
const { t } = useI18n()

const step = ref<"email" | "verify" | "reset">("email")
const email = ref(typeof route.query.username === "string" ? route.query.username : "")
const code = ref("")
const newPassword = ref("")
const confirmPassword = ref("")
const resetToken = ref("")
const submitting = ref(false)
const captchaReady = ref(false)
const captchaObj = ref<GeetestCaptchaInstance | null>(null)

function showError(i18nKey: string, debug = "") {
  const inDev = import.meta.env.DEV || import.meta.env.VITE_APP_ENV === "development"
  ElMessage.error(inDev && debug ? `${t(i18nKey)} (${debug})` : t(i18nKey))
}

function goBackToAuth() {
  router.push({ path: "/auth", query: { mode: "login", username: email.value.trim() || undefined } })
}

async function sendCode() {
  const normalizedEmail = email.value.trim().toLowerCase()
  if (!normalizedEmail) {
    showError("auth.error.invalidEmail")
    return
  }
  submitting.value = true
  try {
    const captchaToken = await verifyCaptchaAndIssueToken()
    await requestForgotPasswordCode(normalizedEmail, captchaToken)
    email.value = normalizedEmail
    code.value = ""
    step.value = "verify"
    ElMessage.success(t("login.forgotPasswordSent"))
  } catch (error) {
    if (error instanceof Error && error.message.startsWith("auth.")) {
      showError(error.message)
    } else {
      showError("auth.error.networkFailed", error instanceof Error ? error.message : "")
    }
  } finally {
    resetCaptcha()
    submitting.value = false
  }
}

async function verifyCode() {
  const normalizedEmail = email.value.trim().toLowerCase()
  const verifyCodeValue = code.value.trim()
  if (!normalizedEmail || !verifyCodeValue) {
    showError("auth.error.invalidPayload")
    return
  }
  submitting.value = true
  try {
    const payload = await verifyForgotPasswordCode(normalizedEmail, verifyCodeValue)
    resetToken.value = payload.resetToken
    step.value = "reset"
    ElMessage.success(t("login.forgotPasswordVerified"))
  } catch (error) {
    if (error instanceof Error && error.message.startsWith("auth.")) {
      showError(error.message)
    } else {
      showError("auth.error.networkFailed", error instanceof Error ? error.message : "")
    }
  } finally {
    submitting.value = false
  }
}

async function resetPassword() {
  if (newPassword.value.length < 6) {
    showError("auth.error.passwordTooShort")
    return
  }
  if (newPassword.value !== confirmPassword.value) {
    showError("auth.error.passwordConfirmMismatch")
    return
  }
  if (!resetToken.value) {
    showError("auth.error.resetTokenInvalid")
    return
  }
  submitting.value = true
  try {
    await resetForgotPassword(resetToken.value, newPassword.value)
    ElMessage.success(t("login.forgotPasswordResetSuccess"))
    goBackToAuth()
  } catch (error) {
    if (error instanceof Error && error.message.startsWith("auth.")) {
      showError(error.message)
    } else {
      showError("auth.error.networkFailed", error instanceof Error ? error.message : "")
    }
  } finally {
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

function resetCaptcha() {
  captchaObj.value?.reset()
}

async function submitCaptchaValidateRequest(payload: GeetestValidatePayload) {
  const res = await fetch("/api/geetest/validate", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(payload)
  })
  if (!res.ok) {
    const parsed = await parseAuthErrorResponse(res)
    throw new Error(resolveAuthError(parsed).key)
  }
  const raw = await res.json()
  return raw?.data ?? raw
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
  const container = document.getElementById("geetest-forgot")
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
      instance.appendTo("#geetest-forgot")
      captchaReady.value = true
    }
  )
}

onMounted(async () => {
  try {
    await initCaptcha()
  } catch (error) {
    showError("auth.error.captchaUnavailable", error instanceof Error ? error.message : "")
  }
})
</script>
