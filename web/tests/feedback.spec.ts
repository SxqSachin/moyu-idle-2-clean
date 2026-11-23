import { describe, expect, it } from "vitest"
import { feedbackChineseUrl, feedbackDefaultUrl, resolveFeedbackUrl } from "../src/services/feedback"

describe("feedback url", () => {
  it("keeps simplified and traditional Chinese on the existing survey", () => {
    expect(resolveFeedbackUrl("zh-CN")).toBe(feedbackChineseUrl)
    expect(resolveFeedbackUrl("zh-TW")).toBe(feedbackChineseUrl)
  })

  it("keeps Chinese browser aliases on the existing survey", () => {
    expect(resolveFeedbackUrl("zh")).toBe(feedbackChineseUrl)
    expect(resolveFeedbackUrl("zh-HK")).toBe(feedbackChineseUrl)
  })

  it("uses the default survey for non-Chinese locales", () => {
    for (const locale of ["en-US", "ja", "ko", "ru", undefined]) {
      expect(resolveFeedbackUrl(locale)).toBe(feedbackDefaultUrl)
    }
  })
})
