import { expect, test } from "@playwright/test"

test("renders login page in zh-CN", async ({ page }) => {
  await page.addInitScript(() => {
    localStorage.setItem("locale", "zh-CN")
  })
  await page.goto("/")
  const loginEntry = page.getByRole("button", { name: "登录 / 注册" }).first()
  await expect(loginEntry).toBeVisible()
  await loginEntry.click()
  await expect(page.getByText("登录后继续")).toBeVisible()
  await expect(page.getByRole("tab", { name: "注册" })).toBeVisible()
  await expect(page.getByRole("button", { name: "登录" })).toBeVisible()
})
