import { describe, expect, it } from "vitest"
import {
  chatAttachmentIdentity,
  createItemInstanceChatAttachment,
  normalizeChatAttachment,
  toChatAttachmentRequest
} from "../src/services/chat-attachments"

describe("chat attachments", () => {
  it("normalizes item, inventory item, and action references", () => {
    expect(normalizeChatAttachment({ type: "item", itemId: " iron_ore " })).toEqual({
      type: "item",
      itemId: "iron_ore"
    })
    expect(normalizeChatAttachment({
      type: "item_instance",
      itemId: "iron_sword",
      instanceId: 42,
      quantity: 7,
      preview: {
        customData: { name: "Northwind", labelColor: "sky" },
        enhanceLevel: 6,
        durability: 81,
        maxDurability: 100,
        baseSnapshot: { attack: 18 },
        affixes: [{ slotIndex: 0, affixId: "sharp", rarityId: "rare" }]
      }
    })).toMatchObject({
      type: "item_instance",
      itemId: "iron_sword",
      instanceId: 42,
      preview: {
        customData: { name: "Northwind", labelColor: "sky" },
        enhanceLevel: 6
      }
    })
    expect(normalizeChatAttachment({
      type: "item_instance",
      itemId: "iron_sword",
      preview: { enhanceLevel: 2 }
    })).toMatchObject({ type: "item_instance", itemId: "iron_sword", preview: { enhanceLevel: 2 } })
    expect(normalizeChatAttachment({ type: "action", actionId: " mining.iron " })).toEqual({
      type: "action",
      actionId: "mining.iron"
    })
  })

  it("rejects malformed structured references", () => {
    expect(normalizeChatAttachment({ type: "item", itemId: "" })).toBeUndefined()
    expect(normalizeChatAttachment({ type: "item_instance", itemId: "", instanceId: 42 })).toBeUndefined()
    expect(normalizeChatAttachment({ type: "action", actionId: "" })).toBeUndefined()
  })

  it("sends only authoritative lookup fields", () => {
    const attachment = normalizeChatAttachment({
      type: "item_instance",
      itemId: "iron_sword",
      instanceId: 42,
      quantity: 1,
      preview: { enhanceLevel: 99, durability: 999, maxDurability: 999 }
    })
    expect(attachment).toBeDefined()
    expect(attachment).not.toHaveProperty("quantity")
    expect(attachment?.type === "item_instance" ? attachment.preview : {}).not.toHaveProperty("durability")
    expect(attachment?.type === "item_instance" ? attachment.preview : {}).not.toHaveProperty("maxDurability")
    expect(toChatAttachmentRequest(attachment!)).toEqual({
      type: "item_instance",
      instanceId: 42
    })
  })

  it("builds and identifies a pending inventory snapshot", () => {
    const attachment = createItemInstanceChatAttachment({
      instanceId: 77,
      templateItemId: "iron_sword",
      customData: { name: "Northwind" },
      quantity: 1,
      state: "inventory",
      enhanceLevel: 3,
      durability: 45,
      maxDurability: 60,
      baseSnapshot: { attack: 12 },
      affixes: []
    })
    expect(attachment.preview).toMatchObject({
      customData: { name: "Northwind" },
      enhanceLevel: 3
    })
    expect(attachment.preview).not.toHaveProperty("durability")
    expect(attachment.preview).not.toHaveProperty("maxDurability")
    expect(chatAttachmentIdentity(attachment)).toBe("item_instance:77")
  })
})
