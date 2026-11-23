import { computed } from "vue"
import { usePlayerStore } from "@stores/player"

type TranslateLike = (key: string) => string

export function useKittiesViewModel(t: TranslateLike, options?: { uiMaxKitties?: number }) {
  const playerStore = usePlayerStore()
  const configuredMax = Number(options?.uiMaxKitties)
  const uiMaxKitties = Number.isFinite(configuredMax) && configuredMax > 0
    ? Math.floor(configuredMax)
    : 0

  const kitties = computed(() => playerStore.kitties || [])
  const capacity = computed(() => Math.max(0, Math.floor(Number(playerStore.buildings.cattery || 0))))
  const capacityDisplay = computed(() =>
    uiMaxKitties > 0
      ? Math.max(0, Math.min(uiMaxKitties, capacity.value))
      : capacity.value
  )
  const canAdoptInUI = computed(() => capacity.value > 0 && kitties.value.length < capacityDisplay.value)

  async function ensureInitialized(token: string) {
    if (!token) return
    await playerStore.initialize(token)
  }

  function kittyAvatar(name: string) {
    const trimmed = String(name || "").trim()
    return trimmed ? trimmed.slice(0, 1) : "猫"
  }

  function kittyStatusLabel(status: string) {
    if (status === "in_combat") return "战斗中"
    if (status === "in_life_queue") return "生活队列中"
    return "空闲"
  }

  function moodLabel(value: number) {
    const percent = Math.round(Math.max(-1, Math.min(1, value)) * 100)
    if (percent >= 60) return `${t("game.catteryKittyMoodHappy")} ${percent}%`
    if (percent <= -30) return `${t("game.catteryKittyMoodLow")} ${percent}%`
    return `${t("game.catteryKittyMoodCalm")} ${percent}%`
  }

  function expPercent(exp: number, nextExp: number) {
    if (!Number.isFinite(nextExp) || nextExp <= 0) return 100
    return Math.max(0, Math.min(100, Math.round((Math.max(0, exp) / nextExp) * 100)))
  }

  function kittyPersonalityIDs(kitty: Record<string, unknown>): string[] {
    const extra = (kitty.extra && typeof kitty.extra === "object") ? kitty.extra as Record<string, unknown> : {}
    const listRaw = extra.personalityIds
    const out: string[] = []
    if (Array.isArray(listRaw)) {
      for (const row of listRaw) {
        const id = String(row || "").trim().toLowerCase()
        if (id && !out.includes(id)) out.push(id)
      }
    }
    return out
  }

  return {
    kitties,
    capacity,
    capacityDisplay,
    canAdoptInUI,
    ensureInitialized,
    kittyAvatar,
    kittyStatusLabel,
    moodLabel,
    expPercent,
    kittyPersonalityIDs
  }
}
