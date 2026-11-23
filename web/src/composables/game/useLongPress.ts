import { onBeforeUnmount, ref, unref, type MaybeRef } from "vue"

interface UseLongPressOptions {
  enabled?: MaybeRef<boolean>
  delayMs?: number
  moveThresholdPx?: number
}

export function useLongPress(onLongPress: () => void, options: UseLongPressOptions = {}) {
  const triggered = ref(false)
  let timerId: number | null = null
  let startX = 0
  let startY = 0
  let active = false

  const delayMs = options.delayMs ?? 400
  const moveThresholdPx = options.moveThresholdPx ?? 10

  function isEnabled() {
    return unref(options.enabled ?? true)
  }

  function clearTimer() {
    if (timerId === null) return
    window.clearTimeout(timerId)
    timerId = null
  }

  function cancelPress() {
    clearTimer()
    active = false
  }

  function onPointerDown(event: PointerEvent) {
    if (!isEnabled()) return
    if (event.pointerType === "mouse" || event.button !== 0) return
    cancelPress()
    triggered.value = false
    active = true
    startX = event.clientX
    startY = event.clientY
    timerId = window.setTimeout(() => {
      timerId = null
      active = false
      triggered.value = true
      onLongPress()
    }, delayMs)
  }

  function onPointerMove(event: PointerEvent) {
    if (!active || timerId === null) return
    const deltaX = Math.abs(event.clientX - startX)
    const deltaY = Math.abs(event.clientY - startY)
    if (deltaX > moveThresholdPx || deltaY > moveThresholdPx) {
      cancelPress()
    }
  }

  function onPointerUp() {
    cancelPress()
  }

  function onContextMenu(event: MouseEvent) {
    if (!isEnabled()) return
    event.preventDefault()
  }

  function consumeTriggeredClick(event: MouseEvent) {
    if (!triggered.value) return false
    event.preventDefault()
    event.stopPropagation()
    triggered.value = false
    return true
  }

  onBeforeUnmount(() => {
    clearTimer()
  })

  return {
    onPointerDown,
    onPointerMove,
    onPointerUp,
    onContextMenu,
    consumeTriggeredClick,
    triggered
  }
}
