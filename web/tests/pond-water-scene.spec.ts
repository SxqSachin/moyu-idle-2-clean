import { afterEach, beforeEach, describe, expect, it, vi } from "vitest"
import { createApp, nextTick, type App } from "vue"
import { createI18n } from "vue-i18n"
import PondWaterScene from "@components/game/fishing/PondWaterScene.vue"
import type { FishingPondVisibleFish } from "@services/fishing"

describe("pond rendering budget and lifecycle", () => {
  let app: App | undefined
  let host: HTMLElement
  let now: number
  let width: number
  let hidden: boolean
  let reduced: boolean
  let sequence: number
  let frames: Map<number, FrameRequestCallback>
  let intersect: (entries: Partial<IntersectionObserverEntry>[]) => void
  let resize: () => void
  let motionChange: () => void
  let gl: Record<string, any>
  let disconnect: ReturnType<typeof vi.fn>

  beforeEach(() => {
    now = 1000
    width = 1000
    hidden = false
    reduced = false
    sequence = 0
    frames = new Map()
    disconnect = vi.fn()
    vi.spyOn(performance, "now").mockImplementation(() => now)
    vi.spyOn(HTMLElement.prototype, "clientWidth", "get").mockImplementation(() => width)
    vi.spyOn(HTMLElement.prototype, "clientHeight", "get").mockReturnValue(400)
    vi.spyOn(document, "hidden", "get").mockImplementation(() => hidden)
    vi.stubGlobal("devicePixelRatio", 3)
    vi.stubGlobal("requestAnimationFrame", (callback: FrameRequestCallback) => {
      frames.set(++sequence, callback)
      return sequence
    })
    vi.stubGlobal("cancelAnimationFrame", (id: number) => frames.delete(id))
    vi.stubGlobal("IntersectionObserver", class {
      constructor(callback: typeof intersect) { intersect = callback }
      observe() {}
      disconnect = disconnect
    })
    vi.stubGlobal("ResizeObserver", class {
      constructor(callback: typeof resize) { resize = callback }
      observe() {}
      disconnect = disconnect
    })
    vi.stubGlobal("matchMedia", () => ({
      get matches() { return reduced },
      addEventListener: (_: string, callback: () => void) => { motionChange = callback },
      removeEventListener: vi.fn()
    }))
    gl = Object.fromEntries([
      "shaderSource", "compileShader", "attachShader", "linkProgram", "bindBuffer", "bufferData",
      "useProgram", "enableVertexAttribArray", "vertexAttribPointer", "uniform2f", "uniform1f",
      "drawArrays", "viewport", "deleteShader", "deleteProgram", "deleteBuffer"
    ].map((name) => [name, vi.fn()]))
    for (const name of ["createShader", "createProgram", "createBuffer", "getUniformLocation"]) gl[name] = vi.fn(() => ({}))
    gl.getShaderParameter = gl.getProgramParameter = vi.fn(() => true)
    gl.getAttribLocation = vi.fn(() => 0)
    gl.getExtension = vi.fn(() => ({ loseContext: vi.fn() }))
    vi.spyOn(HTMLCanvasElement.prototype, "getContext").mockImplementation(() => gl as any)
    host = document.createElement("div")
    document.body.append(host)
  })

  afterEach(() => {
    app?.unmount()
    app = undefined
    host.remove()
    vi.restoreAllMocks()
    vi.unstubAllGlobals()
  })

  async function mount(fish: FishingPondVisibleFish[] = []) {
    app = createApp(PondWaterScene, { fish, species: [], capabilities: [] })
    app.component("el-tooltip", { template: "<slot />" })
    app.use(createI18n({ legacy: false, locale: "en", missingWarn: false, fallbackWarn: false }))
    app.mount(host)
    await nextTick()
  }

  function advance(milliseconds: number, hz = 120) {
    const end = now + milliseconds
    while (now < end) {
      now += 1000 / hz
      const pending = [...frames.values()]
      frames.clear()
      pending.forEach((callback) => callback(now))
    }
  }

  it("bounds water pixels and draws independently of a 120Hz display", async () => {
    await mount()
    expect(gl.drawArrays).not.toHaveBeenCalled()
    intersect([{ isIntersecting: true }])
    advance(2000)
    expect(gl.drawArrays.mock.calls.length).toBeGreaterThan(15)
    expect(gl.drawArrays.mock.calls.length).toBeLessThanOrEqual(25)
    const canvas = host.querySelector("canvas")!
    expect(canvas.width * canvas.height).toBeLessThanOrEqual(960 * 540)
    expect(canvas.width).toBeLessThanOrEqual(width)
    width = 390
    resize()
    expect(canvas.width).toBe(292)
    expect(canvas.height).toBe(300)
  })

  it("pauses offscreen and in background, resumes once, and cleans up", async () => {
    await mount()
    intersect([{ isIntersecting: true }])
    advance(200)
    intersect([{ isIntersecting: false }])
    const count = gl.drawArrays.mock.calls.length
    advance(1000)
    expect(gl.drawArrays).toHaveBeenCalledTimes(count)
    expect(frames.size).toBe(0)
    await nextTick()
    expect(host.querySelector(".is-paused")).not.toBeNull()
    hidden = true
    intersect([{ isIntersecting: true }])
    expect(frames.size).toBe(0)
    hidden = false
    document.dispatchEvent(new Event("visibilitychange"))
    document.dispatchEvent(new Event("visibilitychange"))
    expect(frames.size).toBe(1)
    advance(200)
    expect(gl.drawArrays.mock.calls.length).toBeGreaterThan(count)
    app!.unmount()
    app = undefined
    expect(frames.size).toBe(0)
    expect(disconnect).toHaveBeenCalledTimes(2)
    expect(gl.deleteBuffer).toHaveBeenCalledTimes(1)
    expect(gl.deleteProgram).toHaveBeenCalledTimes(1)
    document.dispatchEvent(new Event("visibilitychange"))
    expect(frames.size).toBe(0)
  })

  it("draws once with reduced motion and responds to preference changes", async () => {
    reduced = true
    await mount()
    intersect([{ isIntersecting: true }])
    advance(1000)
    expect(gl.drawArrays).toHaveBeenCalledTimes(1)
    expect(frames.size).toBe(0)
    reduced = false
    motionChange()
    advance(1000)
    expect(gl.drawArrays.mock.calls.length).toBeGreaterThan(5)
    reduced = true
    motionChange()
    advance(100)
    expect(frames.size).toBe(0)
  })

  it("retains resize and visibility handling when WebGL is unavailable", async () => {
    vi.mocked(HTMLCanvasElement.prototype.getContext).mockReturnValue(null)
    await mount()
    expect(host.querySelector(".is-fallback")).not.toBeNull()
    intersect([{ isIntersecting: true }])
    resize()
    advance(200)
    expect(frames.size).toBe(1)
    intersect([{ isIntersecting: false }])
    expect(frames.size).toBe(0)
  })

  it("moves fish with transforms rather than layout and omits unused mutation layers", async () => {
    await mount([{ instanceId: 1, fish: { speciesId: "carp", lengthCm: 30, lifeState: "alive" } } as FishingPondVisibleFish])
    const fish = host.querySelector<HTMLElement>(".pond-fish-sprite")!
    expect(fish.style.left).toBe("")
    expect(fish.style.top).toBe("")
    expect(fish.querySelector(".pond-fish-sprite__prism, .pond-fish-sprite__rainbow, .pond-fish-sprite__glints")).toBeNull()
    const initial = fish.style.transform
    intersect([{ isIntersecting: true }])
    advance(1000)
    expect(fish.style.transform).not.toBe(initial)
    expect(fish.style.transform).toContain("translate3d(")
    expect(host.querySelector<HTMLElement>(".pond-water__world")!.style.transform).toBe("")
    intersect([{ isIntersecting: false }])
    const stopped = fish.style.transform
    advance(1000)
    expect(fish.style.transform).toBe(stopped)
  })
})
