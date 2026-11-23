import { computed, readonly, ref } from "vue"

const MOBILE_MAX_WIDTH = 767

const viewportWidth = ref(typeof window === "undefined" ? 1280 : window.innerWidth)
const touchLike = ref(
  typeof window === "undefined"
    ? false
    : window.matchMedia("(pointer: coarse)").matches || navigator.maxTouchPoints > 0
)
const supportsHover = ref(
  typeof window === "undefined" ? true : window.matchMedia("(hover: hover) and (pointer: fine)").matches
)

let initialized = false
let hoverMediaQuery: MediaQueryList | null = null
let touchMediaQuery: MediaQueryList | null = null

function updateViewportWidth() {
  viewportWidth.value = window.innerWidth
}

function updateHoverCapability() {
  if (!hoverMediaQuery) return
  supportsHover.value = hoverMediaQuery.matches
}

function updateTouchCapability() {
  if (!touchMediaQuery) return
  touchLike.value = touchMediaQuery.matches || navigator.maxTouchPoints > 0
}

function initializeDeviceTracking() {
  if (initialized || typeof window === "undefined") return
  initialized = true
  hoverMediaQuery = window.matchMedia("(hover: hover) and (pointer: fine)")
  touchMediaQuery = window.matchMedia("(pointer: coarse)")
  updateViewportWidth()
  updateHoverCapability()
  updateTouchCapability()
  window.addEventListener("resize", updateViewportWidth)
  hoverMediaQuery.addEventListener("change", updateHoverCapability)
  touchMediaQuery.addEventListener("change", updateTouchCapability)
}

export function useDevice() {
  initializeDeviceTracking()
  const isMobile = computed(() => viewportWidth.value <= MOBILE_MAX_WIDTH)
  return {
    viewportWidth: readonly(viewportWidth),
    isMobile,
    supportsHover: readonly(supportsHover),
    isTouchLike: readonly(touchLike)
  }
}
