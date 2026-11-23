type TranslateFn = (key: string, values?: Record<string, unknown>) => string

export function formatDurationI18n(totalSeconds: number, t: TranslateFn): string {
  const safeSeconds = Math.max(0, Math.floor(Number(totalSeconds) || 0))
  const days = Math.floor(safeSeconds / 86400)
  const hours = Math.floor((safeSeconds % 86400) / 3600)
  const minutes = Math.floor((safeSeconds % 3600) / 60)
  const seconds = safeSeconds % 60

  const parts: string[] = []
  if (days > 0) parts.push(t("game.durationDay", { value: days }))
  if (hours > 0) parts.push(t("game.durationHour", { value: hours }))
  if (minutes > 0) parts.push(t("game.durationMinute", { value: minutes }))
  if (seconds > 0 || parts.length === 0) {
    parts.push(t("game.durationSecond", { value: seconds }))
  }

  return parts.join(" ")
}

export function formatDurationSecondsOneDecimal(totalSeconds: number, t: TranslateFn): string {
  const safeSeconds = Number.isFinite(totalSeconds) ? Math.max(0, totalSeconds) : 0
  return t("game.durationSecond", { value: safeSeconds.toFixed(1) })
}

export function formatCountdownMillisecondsOneDecimal(totalMilliseconds: number, t: TranslateFn): string {
  const safeMilliseconds = Number.isFinite(totalMilliseconds) ? Math.max(0, totalMilliseconds) : 0
  const remainingTenths = Math.ceil(safeMilliseconds / 100)
  return t("game.durationSecond", { value: (remainingTenths / 10).toFixed(1) })
}
