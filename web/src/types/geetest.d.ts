export type GeetestValidatePayload = {
  geetest_challenge: string
  geetest_validate: string
  geetest_seccode: string
}

export type GeetestCaptchaInstance = {
  appendTo(container: string): void
  getValidate(): GeetestValidatePayload | null
  reset(): void
}

export type GeetestInitConfig = {
  gt: string
  challenge: string
  new_captcha: boolean
  offline: boolean
  product: "float" | "popup" | "bind"
  width?: string
}

declare global {
  interface Window {
    initGeetest: (
      config: GeetestInitConfig,
      onReady: (captcha: GeetestCaptchaInstance) => void
    ) => void
  }
}

export {}
