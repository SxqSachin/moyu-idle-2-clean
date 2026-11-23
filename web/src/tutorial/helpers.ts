import { nextTick } from "vue"
import { usePlayerStore } from "@/stores/player"
import { listActionMainTypeDefinitions } from "@services/action-definition"
import { navigateToDynamicPanelTab } from "./driver-factory"

export function delay(ms: number): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, ms))
}

export async function waitForSelector(selector: string, timeoutMs = 5000): Promise<Element | null> {
  const existing = document.querySelector(selector)
  if (existing) return existing

  return new Promise((resolve) => {
    let done = false
    const timer = window.setTimeout(() => {
      if (done) return
      done = true
      observer.disconnect()
      resolve(null)
    }, timeoutMs)

    const observer = new MutationObserver(() => {
      if (done) return
      const el = document.querySelector(selector)
      if (!el) return
      done = true
      window.clearTimeout(timer)
      observer.disconnect()
      resolve(el)
    })
    observer.observe(document.body, { childList: true, subtree: true })
  })
}

export async function clickFirstActionCard(): Promise<void> {
  const selectors = [
    ".page-shell .action-panel-tabs [role='tabpanel']:not([aria-hidden='true']) .action-card",
    ".page-shell .action-panel-tabs [role='tabpanel']:not([aria-hidden='true']) .grid [role='button']",
    ".page-shell .action-panel-tabs [role='tabpanel']:not([aria-hidden='true']) .grid button",
    ".page-shell .action-panel-tabs [role='tabpanel']:not([aria-hidden='true']) .grid .cursor-pointer",
    ".page-shell .action-panel-tabs .action-card",
    ".page-shell .action-panel-tabs .grid [role='button']",
    ".page-shell .action-panel-tabs .grid button",
    ".page-shell .action-panel-tabs .grid .cursor-pointer",
  ]
  for (const selector of selectors) {
    const el = document.querySelector<HTMLElement>(selector)
    if (!el) continue
    el.dispatchEvent(new MouseEvent("click", { bubbles: true, cancelable: true, view: window }))
    await nextTick()
    await delay(350)
    return
  }
}

export async function ensureActionDetailDialogOpen(): Promise<void> {
  if (document.querySelector(".action-detail-tabs")) return
  await clickFirstActionCard()
  await waitForSelector(".action-detail-tabs", 5000)
}

export async function switchActionDetailTab(tabIndex: number): Promise<void> {
  await clickTab(".action-detail-tabs", tabIndex)
}

export async function clickTab(containerSelector: string, tabIndex: number): Promise<void> {
  const tabs = Array.from(document.querySelectorAll<HTMLElement>(`${containerSelector} .el-tabs__item`))
  const tab = tabs[tabIndex]
  if (!tab) return
  tab.dispatchEvent(new MouseEvent("click", { bubbles: true, cancelable: true, view: window }))
  await nextTick()
  await delay(150)
}

export async function ensureTabIndex(containerSelector: string, tabIndex: number, timeoutMs = 4000): Promise<void> {
  await waitForSelector(`${containerSelector} .el-tabs__item`, timeoutMs)
  await clickTab(containerSelector, tabIndex)
}

export async function clickLastTab(containerSelector: string): Promise<void> {
  const tabs = Array.from(document.querySelectorAll<HTMLElement>(`${containerSelector} .el-tabs__item`))
  const tab = tabs[tabs.length - 1]
  if (!tab) return
  tab.dispatchEvent(new MouseEvent("click", { bubbles: true, cancelable: true, view: window }))
  await nextTick()
  await delay(150)
}

/**
 * Switch the combat page to the tab at `index` (area=0, rooms=1, mine=2, battle=3, help=4)
 * by clicking its el-tabs header. Panes are lazy, so allow extra time for the
 * sub-view to mount before the tutorial highlights it.
 */
export async function switchCombatTab(index: number): Promise<void> {
  await clickTab("[data-tutorial='combat-tabs']", index)
  await delay(250)
}

export async function navigateToAvailableActionPanel(preferred = "collecting"): Promise<string> {
  const ids = listActionMainTypeDefinitions().map((entry) => String(entry.id || "").trim()).filter(Boolean)
  const tabId = ids.includes(preferred) ? preferred : ids[0] || preferred
  await navigateToDynamicPanelTab(tabId)
  return tabId
}

export function hasOwnedKitties(): boolean {
  const playerStore = usePlayerStore()
  return Array.isArray(playerStore.kitties) && playerStore.kitties.length > 0
}
