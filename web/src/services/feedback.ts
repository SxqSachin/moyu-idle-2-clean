import { normalizeSupportedLocale } from "@/i18n"


export const feedbackChineseUrl = "https://wj.qq.com/s2/27753457/o0zf/"
export const feedbackDefaultUrl = "https://wj.qq.com/s2/27175717/c466/"

export function resolveFeedbackUrl(rawLocale: string | null | undefined): string {
  const locale = normalizeSupportedLocale(rawLocale)
  return locale === "zh-CN" || locale === "zh-TW" ? feedbackChineseUrl : feedbackDefaultUrl
}
