import { beforeEach, describe, expect, it } from "vitest"
import {
  clearOnboardingPromptPending,
  markOnboardingPromptPending,
  shouldShowOnboardingPrompt,
} from "../src/services/onboarding-prompt"

describe("onboarding prompt marker", () => {
  beforeEach(() => {
    localStorage.clear()
  })

  it("marks only the newly registered user as pending", () => {
    markOnboardingPromptPending(12)

    expect(shouldShowOnboardingPrompt(12)).toBe(true)
    expect(shouldShowOnboardingPrompt(13)).toBe(false)
    expect(shouldShowOnboardingPrompt(0)).toBe(false)
  })

  it("clears the marker after the player makes a choice", () => {
    markOnboardingPromptPending(12)
    clearOnboardingPromptPending(12)

    expect(shouldShowOnboardingPrompt(12)).toBe(false)
  })
})
