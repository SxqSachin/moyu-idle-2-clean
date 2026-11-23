import { nextTick } from "vue"
import { driver, type Config, type DriveStep, type Driver, type DriverHook, type PopoverDOM } from "driver.js"
import "driver.js/dist/driver.css"
import i18n from "@/i18n"
import router from "@/router"
import { listActionMainTypeDefinitions } from "@services/action-definition"
import { useDevice } from "@/composables/game/useDevice"
import type { TutorialConfig, TutorialStep, TargetDescriptor, TutorialStepMobileOverrides } from "./types"

const t = (key: string) => i18n.global.t(key)

function delay(ms: number): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, ms))
}

/** True when the element renders at least one box (i.e. not display:none). */
function isElementVisible(el: Element): boolean {
  return el.getClientRects().length > 0
}

/**
 * Resolve a selector to the first VISIBLE match, falling back to the first match.
 * Many pages keep both a mobile and a desktop variant of the same element in the DOM
 * (one hidden via `lg:hidden` / `hidden lg:block`) sharing one `data-tutorial` hook.
 * Plain `querySelector` returns whichever comes first in document order, which on the
 * wrong breakpoint is the `display:none` one — so always prefer the visible variant.
 */
function queryVisible(selector: string): Element | null {
  const matches = Array.from(document.querySelectorAll(selector))
  for (const el of matches) {
    if (isElementVisible(el)) return el
  }
  return matches[0] ?? null
}

/** Sync element lookup — returns visible element if already in DOM, undefined otherwise. */
function findElementSync(selector: string | undefined): Element | undefined {
  if (!selector) return undefined
  return queryVisible(selector) ?? undefined
}

/**
 * Async target preparation: navigate route + run setup + wait for element in DOM.
 * This must fully resolve BEFORE Driver.js tries to highlight the step.
 */
async function prepareTarget(desc: TargetDescriptor): Promise<void> {
  if (desc.route && router.currentRoute.value.path !== desc.route) {
    await router.push(desc.route)
    await nextTick()
    await delay(350)
  }
  if (desc.setup) {
    await desc.setup()
    await nextTick()
    await delay(100)
  }
  if (desc.selector) {
    const el = await waitForElementAsync(desc.selector, desc.waitForMs ?? 5000)
    if (!el && !desc.optional) {
      console.warn(`[tutorial] Target not found: ${desc.selector}`)
    }
  }
}

/**
 * Create a Driver.js instance from a TutorialConfig.
 */
export function createTutorialDriver(
  config: TutorialConfig,
  onComplete: () => void,
  onSkip: () => void,
): Driver {
  const { isMobile } = useDevice()
  const mobile = isMobile.value

  const filtered = filterStepsForDevice(config.steps)
  const steps: DriveStep[] = filtered.map((step, index) => buildDriveStep(step, index, filtered, mobile))

  const driverConfig: Config = {
    animate: true,
    overlayColor: "#000",
    overlayOpacity: 0.7,
    smoothScroll: true,
    allowClose: true,
    disableActiveInteraction: true,
    stagePadding: mobile ? 12 : 8,
    stageRadius: 8,
    allowKeyboardControl: true,
    showProgress: true,
    progressText: "{{current}} / {{total}}",
    nextBtnText: t("tutorial.next"),
    prevBtnText: t("tutorial.prev"),
    doneBtnText: t("tutorial.done"),
    // Ignore clicks on the dimmed overlay so a misclick doesn't abort the tour.
    // Players still exit via the popover close (X) button or the Esc key.
    overlayClickBehavior: (() => {}) as DriverHook,
    popoverClass: mobile ? "tutorial-popover tutorial-popover-mobile" : "tutorial-popover",
    onDestroyStarted: ((
      _el: Element | undefined,
      _step: DriveStep,
      opts: { driver: Driver },
    ) => {
      const d = opts.driver
      if (d.isLastStep()) {
        onComplete()
      } else {
        onSkip()
      }
      d.destroy()
    }) as DriverHook,
  }

  const d = driver(driverConfig)
  d.setSteps(steps)
  return d
}

/**
 * Start a tutorial: pre-prepares the first step's target (route + setup + DOM wait),
 * then calls drive(). The first step's element will be in DOM when Driver.js renders it.
 */
export async function startTutorialDriver(
  config: TutorialConfig,
  onComplete: () => void,
  onSkip: () => void,
): Promise<Driver> {
  const filtered = filterStepsForDevice(config.steps)
  if (filtered.length > 0) {
    await prepareTarget(filtered[0].target)
  }
  const d = createTutorialDriver(config, onComplete, onSkip)
  d.drive()
  return d
}

function mergeMobileStep(step: TutorialStep, overrides: TutorialStepMobileOverrides): TutorialStep {
  return {
    ...step,
    ...overrides,
    popover: { ...step.popover, ...overrides.popover },
    target: { ...step.target, ...overrides.target },
  }
}

function buildDriveStep(step: TutorialStep, index: number, allFiltered: TutorialStep[], mobile: boolean): DriveStep {
  let cleanupTargetClick: (() => void) | null = null
  let moving = false
  let active = false

  const moveTo = async (opts: { driver: Driver }, direction: "next" | "previous") => {
    if (moving) return
    moving = true
    cleanupTargetClick?.()
    cleanupTargetClick = null
    try {
      const targetIdx = direction === "next" ? index + 1 : index - 1
      if (targetIdx >= 0 && targetIdx < allFiltered.length) {
        await prepareTarget(allFiltered[targetIdx].target)
        if (direction === "next") opts.driver.moveNext()
        else opts.driver.movePrevious()
        return
      }
      if (direction === "next") opts.driver.moveNext()
      else opts.driver.movePrevious()
    } finally {
      moving = false
    }
  }

  return {
    element: () => findElementSync(step.target.selector),

    // Per-step override of the global disableActiveInteraction; interactive
    // steps let the player click/type on the highlighted element themselves.
    disableActiveInteraction: !step.interactive,

    onDeselected: (() => {
      active = false
      cleanupTargetClick?.()
      cleanupTargetClick = null
    }) as DriverHook,

    onHighlighted: ((_el, _step, opts) => {
      active = true
      if (!_el && step.target.selector) {
        void waitForElementAsync(step.target.selector, step.target.waitForMs ?? 3000).then(() => {
          opts.driver.refresh()
        })
      }
      if (_el && step.advanceOnTargetClick) {
        const onClick = () => {
          cleanupTargetClick?.()
          cleanupTargetClick = null
          window.setTimeout(() => {
            void moveTo(opts, "next")
          }, 150)
        }
        _el.addEventListener("click", onClick, { capture: true, once: true })
        cleanupTargetClick = () => _el.removeEventListener("click", onClick, { capture: true })
      }
      if (step.advanceWhen?.selector) {
        void waitForElementAsync(step.advanceWhen.selector, step.advanceWhen.waitForMs ?? 30000).then((el) => {
          if (!el || !active) return
          void moveTo(opts, "next")
        })
      }
    }) as DriverHook,

    popover: {
      title: t(step.popover.titleKey),
      description: t(step.popover.descriptionKey),
      side: step.popover.side ?? "bottom",
      align: step.popover.align ?? "center",
      popoverClass: step.popover.popoverClass,
      onPopoverRender: mobile
        ? (popover, opts) => {
            bindMobilePopoverTouchFallback(popover, opts, moveTo)
          }
        : undefined,

      onNextClick: ((_el, _step, opts) => {
        void moveTo(opts, "next")
      }) as DriverHook,

      onPrevClick: ((_el, _step, opts) => {
        void moveTo(opts, "previous")
      }) as DriverHook,
    },
  }
}

function bindMobilePopoverTouchFallback(
  popover: PopoverDOM,
  opts: { driver: Driver },
  moveTo: (opts: { driver: Driver }, direction: "next" | "previous") => Promise<void>,
): void {
  const bind = (button: HTMLButtonElement, direction: "next" | "previous") => {
    button.addEventListener(
      "touchend",
      (event) => {
        if (button.disabled || button.classList.contains("driver-popover-btn-disabled")) return
        event.preventDefault()
        event.stopPropagation()
        void moveTo(opts, direction)
      },
      { passive: false },
    )
  }

  bind(popover.nextButton, "next")
  bind(popover.previousButton, "previous")
}

function filterStepsForDevice(steps: TutorialStep[]): TutorialStep[] {
  const { isMobile } = useDevice()
  const mobile = isMobile.value
  return steps
    .map((step) => {
      const effective = mobile && step.mobile ? mergeMobileStep(step, step.mobile) : step
      if (!mobile) return effective
      return {
        ...effective,
        popover: {
          ...effective.popover,
          align: effective.popover.align ?? "center",
        },
      }
    })
    .filter((step) => !step.when || step.when())
}

/**
 * Wait for the shell's dynamic action menu to load, then navigate to a panel tab.
 * Use this in tutorial step `setup` when targeting dynamic action tabs (e.g., collecting, crafting).
 * The shell loads actionMainTypes asynchronously after bootstrap — this ensures the tab
 * is in allowedTabSet before navigation, preventing premature redirect to homepage.
 */
export async function navigateToDynamicPanelTab(tabId: string): Promise<void> {
  const targetPath = `/game/panel/${tabId}`

  // Wait until the action catalog includes this tab before navigating. The shell derives
  // its menu and allowedTabSet from the same listActionMainTypeDefinitions() source, so once
  // the id is present the route guard won't redirect to home. (Do NOT gate on a DOM selector:
  // Element Plus el-menu-item does not render the `index` prop as an attribute, so a
  // `.el-menu-item[index=...]` query never matches and just burns the full timeout.)
  for (let i = 0; i < 50; i++) {
    const ids = listActionMainTypeDefinitions().map((entry) => String(entry?.id || "").trim())
    if (ids.includes(tabId)) break
    await delay(100)
  }

  for (let attempt = 0; attempt < 3; attempt++) {
    await router.push(targetPath)
    await nextTick()
    for (let i = 0; i < 16; i++) {
      if (router.currentRoute.value.path === targetPath) {
        await delay(250)
        return
      }
      await delay(100)
    }
  }
  console.warn(`[tutorial] Dynamic panel route did not stabilize: ${targetPath}`)
  await delay(250)
}

function waitForElementAsync(selector: string, timeoutMs: number): Promise<Element | undefined> {
  const existing = queryVisible(selector)
  if (existing) return Promise.resolve(existing)

  return new Promise((resolve) => {
    let done = false
    const timer = setTimeout(() => {
      if (done) return
      done = true
      observer.disconnect()
      resolve(undefined)
    }, timeoutMs)

    const observer = new MutationObserver(() => {
      if (done) return
      const el = queryVisible(selector)
      if (el) {
        done = true
        clearTimeout(timer)
        observer.disconnect()
        resolve(el)
      }
    })
    observer.observe(document.body, { childList: true, subtree: true })
  })
}
