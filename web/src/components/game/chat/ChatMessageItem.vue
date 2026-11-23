<template>
  <div class="mb-2 rounded-md bg-[var(--app-surface)] px-2.5 py-2">
    <div class="flex items-baseline justify-between gap-2">
      <div class="flex flex-row items-center gap-1">
        <UserName :user-id="message.userId" :username="message.username" plain class="text-[14px] font-semibold" />
        <template v-if="displayTitle">
          <span class="inline-block text-[10px] text-[var(--app-muted)]">[{{ displayTitle }}]</span>
        </template>
      </div>
      <span class="shrink-0 text-[10px] leading-tight text-[var(--app-muted)]">{{ formattedTime }}</span>
    </div>
    <div v-if="message.content" class="mt-1.5 break-words text-xs leading-5 text-[var(--app-text)]">{{ message.content }}</div>
    <ChatAttachmentCard
      v-if="attachment"
      class="mt-2"
      :attachment="attachment"
      @activate="openAttachment"
    />
  </div>
</template>

<script setup lang="ts">
import { computed } from "vue"
import { useI18n } from "vue-i18n"
import type { ChatMessage } from "@stores/chat"
import UserName from "@components/common/UserName.vue"
import { findPrefix, findSuffix } from "@composables/useTitleDisplay"
import { openArenaReplay } from "@services/arena-replay-viewer"
import { openCombatTrialReplay } from "@services/combat-trial-replay-viewer"
import { useActionQuickRefDialogStore } from "@stores/action-quick-ref-dialog"
import { useItemPopoverDialogStore } from "@stores/item-popover-dialog"
import ChatAttachmentCard from "./ChatAttachmentCard.vue"

const props = defineProps<{
  message: ChatMessage
}>()

const { t } = useI18n()
const actionDialogStore = useActionQuickRefDialogStore()
const itemDialogStore = useItemPopoverDialogStore()
const attachment = computed(() => props.message.attachment || null)

function openAttachment() {
  const value = attachment.value
  if (!value) return
  if (value.type === "combat_trial_replay") {
    openCombatTrialReplay({ shareToken: value.token })
    return
  }
  if (value.type === "arena_replay") {
    openArenaReplay({ shareToken: value.token })
    return
  }
  if (value.type === "action") {
    actionDialogStore.open({ actionId: value.actionId, count: 1 })
    return
  }
  if (value.type === "item_instance") {
    itemDialogStore.open({
      itemId: value.itemId,
      instanceId: value.instanceId ?? null,
      publicInstancePreview: value.preview
    })
    return
  }
  itemDialogStore.open({ itemId: value.itemId })
}

const formattedTime = computed(() => {
  const { ts } = props.message
  if (!Number.isFinite(ts)) return ""
  return new Date(ts).toLocaleTimeString([], { hour: "2-digit", minute: "2-digit", second: "2-digit" })
})

// Title: look up nameKey from game data so unit-category suffixes (enemy.*) resolve correctly.
const displayTitle = computed(() => {
  const parts: string[] = []
  const prefixId = props.message.titlePrefixId
  const suffixId = props.message.titleSuffixId
  if (prefixId) {
    const entry = findPrefix(prefixId)
    if (entry?.nameKey) {
      const translated = t(entry.nameKey)
      if (translated && translated !== entry.nameKey) parts.push(translated)
    }
  }
  if (suffixId) {
    const entry = findSuffix(suffixId)
    if (entry?.nameKey) {
      const translated = t(entry.nameKey)
      if (translated && translated !== entry.nameKey) parts.push(translated)
    }
  }
  return parts.join("")
})
</script>
