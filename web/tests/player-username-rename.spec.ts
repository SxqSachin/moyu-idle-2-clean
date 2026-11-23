import { afterEach, beforeEach, describe, expect, it, vi } from "vitest"
import { createPinia, setActivePinia } from "pinia"
import {
  PLAYER_USERNAME_RENAME_COST_ITEM_COUNT,
  PLAYER_USERNAME_RENAME_COST_ITEM_ID
} from "../src/config/shared-global-config"
import { reconcilePlayerUsernameDraft, updatePlayerUsername } from "../src/services/player-settings"
import { useLocalUserStore } from "../src/stores/local-user"
import rawSharedGlobalConfig from "../../packages/game-config/source/shared-global-config.json"

describe("player username rename contract", () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it("reads the rename material and quantity from shared config", () => {
    expect(PLAYER_USERNAME_RENAME_COST_ITEM_ID).toBe(rawSharedGlobalConfig.playerUsernameRename.costItemId)
    expect(PLAYER_USERNAME_RENAME_COST_ITEM_COUNT).toBe(rawSharedGlobalConfig.playerUsernameRename.costItemCount)
    expect(PLAYER_USERNAME_RENAME_COST_ITEM_ID).not.toBe("")
    expect(PLAYER_USERNAME_RENAME_COST_ITEM_COUNT).toBeGreaterThan(0)
  })

  it("sends the loaded username baseline in the official rename request", async () => {
    const fetchMock = vi.fn().mockResolvedValue(new Response(JSON.stringify({
      ok: true,
      ts: Date.now(),
      data: { username: "After", previousUsername: "Before", changeId: 7 }
    }), {
      status: 200,
      headers: { "Content-Type": "application/json" }
    }))
    vi.stubGlobal("fetch", fetchMock)

    await updatePlayerUsername("token", "  After  ", "  Before  ")

    expect(fetchMock).toHaveBeenCalledTimes(1)
    const [, init] = fetchMock.mock.calls[0] as [string, RequestInit]
    expect(JSON.parse(String(init.body))).toEqual({
      username: "After",
      expectedCurrentUsername: "Before"
    })
  })

  it("updates public previews and expires detailed profile cache without hiding known history", () => {
    const store = useLocalUserStore()
    store.byUserID[42] = {
      profile: { userId: 42, username: "OldName", derived: { hp: 100 } },
      cachedAt: 123
    }
    store.profileByUserID[42] = {
      profile: {
        userId: 42,
        username: "OldName",
        nameHistory: [{ oldName: "FirstName" }],
        collectiblesCount: 7
      },
      cachedAt: 456
    }

    store.applyProfileChanged(42, "NewName")

    expect(store.byUserID[42]).toEqual({
      profile: { userId: 42, username: "NewName", usernameChangeId: 0, derived: { hp: 100 } },
      cachedAt: 123
    })
    expect(store.profileByUserID[42]?.profile).toMatchObject({
      userId: 42,
      username: "NewName",
      collectiblesCount: 7
    })
    expect(store.profileByUserID[42]?.profile.nameHistory).toEqual([
      { oldName: "FirstName" }
    ])
    expect(store.profileByUserID[42]?.cachedAt).toBe(0)
    expect(store.authoritativeProfileUsernameByUserID[42]).toBe("NewName")
  })

  it("creates an expired preview for an uncached user", () => {
    const store = useLocalUserStore()

    store.applyProfileChanged(9, "RenamedPlayer")

    expect(store.byUserID[9]).toEqual({
      profile: { userId: 9, username: "RenamedPlayer", usernameChangeId: 0 },
      cachedAt: 0
    })
    expect(store.profileByUserID[9]).toBeUndefined()
  })

  it("uses the authoritative previous username to update cached history immediately", () => {
    const store = useLocalUserStore()
    store.profileByUserID[7] = {
      profile: {
        userId: 7,
        username: "Before",
        nameHistory: [{ oldName: "First" }]
      },
      cachedAt: 100
    }

    store.applyProfileChanged(7, "After", 12, "Before")

    expect(store.profileByUserID[7]?.profile.nameHistory).toEqual([
      { oldName: "Before" },
      { oldName: "First" }
    ])
    expect(store.profileByUserID[7]?.cachedAt).toBe(100)
  })

  it("fills the missing history row when the websocket event arrives before the rename response", () => {
    const store = useLocalUserStore()
    store.profileByUserID[7] = {
      profile: {
        userId: 7,
        username: "Before",
        nameHistory: [{ oldName: "First" }]
      },
      cachedAt: 100
    }

    store.applyProfileChanged(7, "After", 12)
    store.applyProfileChanged(7, "After", 12, "Before")

    expect(store.profileByUserID[7]?.profile.nameHistory).toEqual([
      { oldName: "Before" },
      { oldName: "First" }
    ])
  })

  it("applies an authoritative reconnect snapshot and expires stale detailed history", () => {
    const store = useLocalUserStore()
    store.byUserID[3] = { profile: { userId: 3, username: "Before" }, cachedAt: 100 }
    store.profileByUserID[3] = {
      profile: {
        userId: 3,
        username: "Before",
        nameHistory: [{ oldName: "First" }]
      },
      cachedAt: 200
    }

    expect(store.applyProfileUsernameSnapshot(3, "After")).toBe(true)
    store.setPreview(3, "StaleChatPreview")
    expect(store.byUserID[3]?.profile.username).toBe("After")
    expect(store.authoritativeProfileUsernameByUserID[3]).toBe("After")
    expect(store.profileByUserID[3]?.profile).toMatchObject({
      username: "After",
      nameHistory: [{ oldName: "First" }]
    })
    expect(store.profileByUserID[3]?.cachedAt).toBe(0)
  })

  it("expires detailed history when the revision advances back to the same visible username", () => {
    const store = useLocalUserStore()
    store.applyProfileUsernameSnapshot(4, "SameName", 20)
    store.profileByUserID[4] = {
      profile: { userId: 4, username: "SameName", usernameChangeId: 20, nameHistory: [] },
      cachedAt: 200
    }

    expect(store.applyProfileUsernameSnapshot(4, "SameName", 22)).toBe(true)

    expect(store.profileByUserID[4]?.profile.usernameChangeId).toBe(22)
    expect(store.profileByUserID[4]?.cachedAt).toBe(0)
  })

  it("ignores out-of-order change events without rolling a username back", () => {
    const store = useLocalUserStore()

    expect(store.applyProfileChanged(5, "Newest", 20)).toBe(true)
    expect(store.applyProfileChanged(5, "Older", 19)).toBe(false)
    expect(store.applyProfileChanged(5, "ConflictingReplay", 20)).toBe(false)
    expect(store.applyProfileChanged(5, "Newest", 20)).toBe(true)

    expect(store.get(5)?.username).toBe("Newest")
    expect(store.latestProfileChangeIdByUserID[5]).toBe(20)
  })

  it("does not let a forced stale profile response roll back a known newer username", async () => {
    const store = useLocalUserStore()
    store.applyProfileChanged(5, "Newest", 20)
    const fetchMock = vi.fn().mockResolvedValue(new Response(JSON.stringify({
      ok: true,
      ts: Date.now(),
      data: {
        userId: 5,
        username: "Older",
        usernameChangeId: 19,
        nameHistory: [{ oldName: "Oldest" }],
        nameHistoryHasMore: false
      }
    }), {
      status: 200,
      headers: { "Content-Type": "application/json" }
    }))
    vi.stubGlobal("fetch", fetchMock)

    const profile = await store.fetchProfileByUserId("token", 5, { force: true })

    expect(profile.username).toBe("Newest")
    expect(profile.nameHistory).toEqual([])
    expect(store.profileByUserID[5]?.cachedAt).toBe(0)
    expect(fetchMock).toHaveBeenCalledTimes(1)
  })

  it("protects a versioned settings snapshot from older public and profile responses", async () => {
    const store = useLocalUserStore()
    store.applyProfileUsernameSnapshot(6, "Newest", 20)
    const fetchMock = vi.fn()
      .mockResolvedValueOnce(new Response(JSON.stringify({
        ok: true,
        ts: Date.now(),
        data: { userId: 6, username: "Older", usernameChangeId: 19 }
      }), {
        status: 200,
        headers: { "Content-Type": "application/json" }
      }))
      .mockResolvedValueOnce(new Response(JSON.stringify({
        ok: true,
        ts: Date.now(),
        data: {
          userId: 6,
          username: "Older",
          usernameChangeId: 19,
          nameHistory: [{ oldName: "Oldest" }],
          nameHistoryHasMore: false
        }
      }), {
        status: 200,
        headers: { "Content-Type": "application/json" }
      }))
    vi.stubGlobal("fetch", fetchMock)

    const publicProfile = await store.fetchByUserId("token", 6, { force: true })
    const detailedProfile = await store.fetchProfileByUserId("token", 6, { force: true })

    expect(store.latestProfileChangeIdByUserID[6]).toBe(20)
    expect(publicProfile.usernameChangeId).toBe(20)
    expect(publicProfile.username).toBe("Newest")
    expect(detailedProfile.username).toBe("Newest")
    expect(detailedProfile.nameHistory).toEqual([])
    expect(store.byUserID[6]?.cachedAt).toBe(0)
    expect(store.profileByUserID[6]?.cachedAt).toBe(0)
    expect(fetchMock).toHaveBeenCalledTimes(2)
  })

  it("does not treat chat preview names as authoritative", async () => {
    const store = useLocalUserStore()
    store.setPreview(8, "ChatPreview")
    expect(store.authoritativeProfileUsernameByUserID[8]).toBeUndefined()
    const fetchMock = vi.fn().mockResolvedValue(new Response(JSON.stringify({
      ok: true,
      ts: Date.now(),
      data: { userId: 8, username: "ServerName", usernameChangeId: 4 }
    }), {
      status: 200,
      headers: { "Content-Type": "application/json" }
    }))
    vi.stubGlobal("fetch", fetchMock)

    const profile = await store.fetchByUserId("token", 8, { force: true })

    expect(store.authoritativeProfileUsernameByUserID[8]).toBe("ServerName")
    expect(store.latestProfileChangeIdByUserID[8]).toBe(4)
    expect(profile.username).toBe("ServerName")
    expect(store.byUserID[8]?.cachedAt).toBeGreaterThan(0)
  })

  it("rejects older and equal-conflicting revisions, then accepts a newer public username revision", async () => {
    const store = useLocalUserStore()
    store.applyProfileUsernameSnapshot(10, "KnownName", 20)
    store.profileByUserID[10] = {
      profile: { userId: 10, username: "KnownName", usernameChangeId: 20, nameHistory: [] },
      cachedAt: 200
    }
    const fetchMock = vi.fn()
      .mockResolvedValueOnce(new Response(JSON.stringify({
        ok: true,
        ts: Date.now(),
        data: { userId: 10, username: "OlderName", usernameChangeId: 19 }
      }), {
        status: 200,
        headers: { "Content-Type": "application/json" }
      }))
      .mockResolvedValueOnce(new Response(JSON.stringify({
        ok: true,
        ts: Date.now(),
        data: { userId: 10, username: "ConflictingName", usernameChangeId: 20 }
      }), {
        status: 200,
        headers: { "Content-Type": "application/json" }
      }))
      .mockResolvedValueOnce(new Response(JSON.stringify({
        ok: true,
        ts: Date.now(),
        data: { userId: 10, username: "NewerName", usernameChangeId: 21 }
      }), {
        status: 200,
        headers: { "Content-Type": "application/json" }
      }))
    vi.stubGlobal("fetch", fetchMock)

    const older = await store.fetchByUserId("token", 10, { force: true })
    const equalConflict = await store.fetchByUserId("token", 10, { force: true })
    const newer = await store.fetchByUserId("token", 10, { force: true })

    expect(older).toMatchObject({ username: "KnownName", usernameChangeId: 20 })
    expect(equalConflict).toMatchObject({ username: "KnownName", usernameChangeId: 20 })
    expect(newer).toMatchObject({ username: "NewerName", usernameChangeId: 21 })
    expect(store.authoritativeProfileUsernameByUserID[10]).toBe("NewerName")
    expect(store.latestProfileChangeIdByUserID[10]).toBe(21)
    expect(store.byUserID[10]?.cachedAt).toBeGreaterThan(0)
    expect(store.profileByUserID[10]?.profile).toMatchObject({ username: "NewerName", usernameChangeId: 21 })
    expect(store.profileByUserID[10]?.cachedAt).toBe(0)
  })

  it("self-heals a missed websocket rename from a newer detailed profile revision", async () => {
    const store = useLocalUserStore()
    store.applyProfileUsernameSnapshot(11, "KnownName", 20)
    store.byUserID[11] = {
      profile: { userId: 11, username: "KnownName", usernameChangeId: 20, derived: { hp: 10 } },
      cachedAt: 200
    }
    const fetchMock = vi.fn().mockResolvedValue(new Response(JSON.stringify({
      ok: true,
      ts: Date.now(),
      data: {
        userId: 11,
        username: "NewerName",
        usernameChangeId: 21,
        nameHistory: [{ oldName: "KnownName" }],
        nameHistoryHasMore: false
      }
    }), {
      status: 200,
      headers: { "Content-Type": "application/json" }
    }))
    vi.stubGlobal("fetch", fetchMock)

    const profile = await store.fetchProfileByUserId("token", 11, { force: true })

    expect(profile).toMatchObject({
      username: "NewerName",
      usernameChangeId: 21,
      nameHistory: [{ oldName: "KnownName" }]
    })
    expect(store.authoritativeProfileUsernameByUserID[11]).toBe("NewerName")
    expect(store.latestProfileChangeIdByUserID[11]).toBe(21)
    expect(store.profileByUserID[11]?.cachedAt).toBeGreaterThan(0)
    expect(store.byUserID[11]?.profile).toMatchObject({
      username: "NewerName",
      usernameChangeId: 21,
      derived: { hp: 10 }
    })
    expect(store.byUserID[11]?.cachedAt).toBe(0)
  })

  it("blocks a stale draft after a remote rename until the player edits or accepts the latest name", () => {
    expect(reconcilePlayerUsernameDraft("Before", "Before", "After")).toEqual({
      draft: "After",
      baseline: "After",
      conflict: false
    })
    expect(reconcilePlayerUsernameDraft("My Draft", "Before", "After")).toEqual({
      draft: "My Draft",
      baseline: "After",
      conflict: true
    })
    expect(reconcilePlayerUsernameDraft("After", "Before", "After")).toEqual({
      draft: "After",
      baseline: "After",
      conflict: false
    })
  })
})
