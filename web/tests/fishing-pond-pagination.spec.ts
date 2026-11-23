import { beforeEach, describe, expect, it, vi } from "vitest"
import { apiGet } from "@services/api"
import { loadAllAvailablePondFish, type FishingPondFish, type FishingPondOverview } from "@services/fishing"

vi.mock("@services/api", () => ({ apiGet: vi.fn(), apiPost: vi.fn() }))
const fish = (id: number) => ({ instanceId: id } as FishingPondFish)
const pond = () => ({ built: true, availableFish: [fish(601)], availableFishNextCursor: 600 } as FishingPondOverview)

describe("pond candidate pagination", () => {
  beforeEach(() => vi.clearAllMocks())
  it("continues through empty pages and deduplicates without mutating the overview", async () => {
    vi.mocked(apiGet).mockResolvedValueOnce({ items: [], nextCursor: 400 })
      .mockResolvedValueOnce({ items: [fish(601), ...Array.from({ length: 350 }, (_, index) => fish(350 - index))] })
    const original = pond()
    const complete = await loadAllAvailablePondFish("token", original)
    expect(complete.availableFish).toHaveLength(351)
    expect(complete.availableFish.at(-1)?.instanceId).toBe(1)
    expect(complete.availableFishNextCursor).toBe(0)
    expect(original.availableFish).toHaveLength(1)
    expect(apiGet).toHaveBeenNthCalledWith(2, "/api/fishing/pond/available-fish?cursor=400", "token")
  })
  it("does not accept partial results on request failure or a repeating cursor", async () => {
    vi.mocked(apiGet).mockRejectedValueOnce(new Error("network"))
    await expect(loadAllAvailablePondFish("token", pond())).rejects.toThrow("network")
    vi.mocked(apiGet).mockResolvedValueOnce({ items: [], nextCursor: 600 })
    await expect(loadAllAvailablePondFish("token", pond())).rejects.toThrow("cursor")
  })
  it("makes no extra request when the initial page is complete", async () => {
    await loadAllAvailablePondFish("token", { ...pond(), availableFishNextCursor: 0 })
    expect(apiGet).not.toHaveBeenCalled()
  })
  it("preserves the not-built response even when its fish arrays are null", async () => {
    const unbuilt = { built: false, availableFish: null } as unknown as FishingPondOverview
    expect(await loadAllAvailablePondFish("token", unbuilt)).toBe(unbuilt)
    expect(apiGet).not.toHaveBeenCalled()
  })
})
