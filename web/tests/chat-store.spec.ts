import { beforeEach, describe, expect, it } from "vitest"
import { createPinia, setActivePinia } from "pinia"
import { useChatStore } from "../src/stores/chat"

describe("chat store", () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

  it("tracks unread by panel visibility and active tab", () => {
    const store = useChatStore()
    store.applyConfig({
      channels: [
        { id: "world", nameKey: "chat.channel.world", order: 10, playerSendable: true },
        { id: "life", nameKey: "chat.channel.life", order: 20, playerSendable: true },
        { id: "system", nameKey: "chat.channel.system", order: 30, playerSendable: false }
      ],
      defaultChannel: "world"
    })

    store.setPanelOpen(false)
    store.setActiveChannel("world")

    store.appendIncomingMessage({ id: 1, channel: "world", userId: 1, username: "A", content: "hello", ts: 1 })
    expect(store.unreadByChannel.world).toBe(1)
    expect(store.totalUnread).toBe(1)

    store.setPanelOpen(true)
    expect(store.unreadByChannel.world).toBe(0)

    store.appendIncomingMessage({ id: 2, channel: "life", userId: 2, username: "B", content: "hi", ts: 2 })
    expect(store.unreadByChannel.life).toBe(1)

    store.setActiveChannel("life")
    expect(store.unreadByChannel.life).toBe(0)
    expect(store.totalUnread).toBe(0)
  })

  it("deduplicates same message id across events", () => {
    const store = useChatStore()
    store.applyConfig({
      channels: [{ id: "world", nameKey: "chat.channel.world", order: 10, playerSendable: true }],
      defaultChannel: "world"
    })

    store.appendIncomingMessage({ id: 99, channel: "world", userId: 1, username: "A", content: "same", ts: 1000 })
    store.appendIncomingMessage({ id: 99, channel: "world", userId: 1, username: "A", content: "same", ts: 1001 })

    expect(store.messagesByChannel.world.length).toBe(1)
  })

  it("syncs and clears dynamic battle room channels", () => {
    const store = useChatStore()
    store.applyConfig({
      channels: [{ id: "world", nameKey: "chat.channel.world", order: 10, playerSendable: true }],
      defaultChannel: "world"
    })

    store.syncBattleRoomChannel(12, "Alpha")
    expect(store.channels.some((channel) => channel.id === "battle_room:12" && channel.label === "Alpha")).toBe(true)

    store.setActiveChannel("battle_room:12")
    store.appendIncomingMessage({ id: 1, channel: "battle_room:12", userId: 1, username: "A", content: "room", ts: 1 })
    expect(store.activeChannel).toBe("battle_room:12")

    store.clearBattleRoomChannels()
    expect(store.channels.some((channel) => channel.id.startsWith("battle_room:"))).toBe(false)
    expect(store.messagesByChannel["battle_room:12"]).toBeUndefined()
    expect(store.activeChannel).toBe("world")
  })

  it("keeps a replay attachment even when the text body is empty", () => {
    const store = useChatStore()
    store.applyConfig({
      channels: [{ id: "world", nameKey: "chat.channel.world", order: 10, playerSendable: true }],
      defaultChannel: "world"
    })

    store.appendIncomingMessage({
      id: 101,
      channel: "world",
      userId: 2,
      username: "B",
      content: "",
      ts: 1000,
      attachment: {
        type: "arena_replay",
        battleKind: "arena",
        token: "a".repeat(64),
        challengeId: 77,
        createdBy: 2,
        createdByName: "B",
        outcome: "attacker_win"
      }
    })

    expect(store.messagesByChannel.world).toHaveLength(1)
    const attachment = store.messagesByChannel.world[0].attachment
    expect(attachment?.type).toBe("arena_replay")
    expect(attachment?.type === "arena_replay" ? attachment.challengeId : 0).toBe(77)
  })

  it("keeps replay shares as a removable pending attachment until chat send", () => {
    const store = useChatStore()
    const attachment = {
      type: "arena_replay" as const,
      battleKind: "arena" as const,
      token: "b".repeat(64),
      challengeId: 88,
      createdBy: 2,
      createdByName: "B",
      outcome: "defender_win"
    }

    store.setPendingAttachment(attachment)
    expect(store.pendingAttachment).toEqual(attachment)
    store.clearPendingAttachment()
    expect(store.pendingAttachment).toBeNull()
  })

  it("keeps structured item and action attachments without a text body", () => {
    const store = useChatStore()
    store.applyConfig({
      channels: [{ id: "world", nameKey: "chat.channel.world", order: 10, playerSendable: true }],
      defaultChannel: "world"
    })

    store.appendIncomingMessage({
      id: 102,
      channel: "world",
      userId: 2,
      username: "B",
      content: "",
      ts: 1001,
      attachment: { type: "item", itemId: "iron_ore" }
    })
    store.appendIncomingMessage({
      id: 103,
      channel: "world",
      userId: 2,
      username: "B",
      content: "",
      ts: 1002,
      attachment: { type: "action", actionId: "mining.iron" }
    })

    expect(store.messagesByChannel.world).toHaveLength(2)
    expect(store.messagesByChannel.world[0].attachment).toEqual({ type: "item", itemId: "iron_ore" })
    expect(store.messagesByChannel.world[1].attachment).toEqual({ type: "action", actionId: "mining.iron" })
  })
})
