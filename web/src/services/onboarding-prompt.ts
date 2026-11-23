const STORAGE_KEY_PREFIX = "onboarding:prompt:v1:"
const PENDING_VALUE = "pending"

function storageKey(userId: number) {
  return `${STORAGE_KEY_PREFIX}${userId}`
}

function normalizeUserId(userId: number): number {
  return Number.isFinite(userId) ? Math.floor(userId) : 0
}

export function markOnboardingPromptPending(userId: number): void {
  const normalized = normalizeUserId(userId)
  if (normalized <= 0) return
  try {
    localStorage.setItem(storageKey(normalized), PENDING_VALUE)
  } catch {
    // Storage can be unavailable in private mode; the prompt is optional.
  }
}

export function shouldShowOnboardingPrompt(userId: number): boolean {
  const normalized = normalizeUserId(userId)
  if (normalized <= 0) return false
  try {
    return localStorage.getItem(storageKey(normalized)) === PENDING_VALUE
  } catch {
    return false
  }
}

export function clearOnboardingPromptPending(userId: number): void {
  const normalized = normalizeUserId(userId)
  if (normalized <= 0) return
  try {
    localStorage.removeItem(storageKey(normalized))
  } catch {
    // Ignore storage failures; prompting should never block game entry.
  }
}
