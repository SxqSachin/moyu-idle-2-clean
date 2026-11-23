export type AuthMode = "register" | "login"

export type AuthErrorPayload = {
  ok?: boolean
  error?:
    | {
        code?: string
        message?: string
        debug?: string
        detail?: string
        instanceState?: string
      }
    | string
  code?: string
  message?: string
  debug?: string
  detail?: string
  instanceState?: string
}

export type DeviceProfile = {
  fingerprintId: string
  fingerprintVersion: string
  componentsHash: string
  deviceFamilyHash: string
  browserFamilyHash: string
  runtimeFamilyHash: string
  cookieId: string
  localDeviceId: string
  browserInstallId: string
  sessionChainId: string
  timezone: string
  language: string
  screenBucket: string
  canvasHash: string
  webglHash: string
  audioHash: string
  platformHint: string
  cpuHint: string
  memoryHint: string
  captchaScore: number
}
