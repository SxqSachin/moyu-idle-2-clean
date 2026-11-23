<template>
  <div class="pointer-events-none fixed bottom-8 right-6 z-[60] flex flex-col items-end gap-2 max-[767px]:bottom-2 max-[767px]:right-2">
    <el-badge :is-dot="showGlobalUnreadDot" class="pointer-events-auto" :offset="[-5, 5]">
      <el-button
        class="w-12! h-12! shadow-md"
        type="primary"
        circle
        :title="t('game.chatTitle')"
        :aria-label="t('game.chatTitle')"
        @click="togglePanel"
      >
        <el-icon :size="24"><Icon icon="ep:chat-dot-square" /></el-icon>
      </el-button>
    </el-badge>

    <transition name="chat-panel-fade">
      <section
        v-if="chatStore.panelOpen"
        class="pointer-events-auto flex h-[440px] w-[640px] flex-col overflow-hidden rounded-xl border border-[var(--app-border)] bg-[var(--app-surface)] shadow-xl max-[767px]:h-[68dvh] max-[767px]:w-[calc(100vw-16px)] max-[767px]:rounded-2xl"
      >
        <header class="flex items-center justify-between border-b border-[var(--app-border)] px-3 py-2">
          <div class="text-sm font-semibold text-[var(--app-text)]">{{ t("game.chatTitle") }}</div>
          <button
            class="rounded-md px-2 py-1 text-xs text-[var(--app-muted)] hover:bg-[var(--app-bg)]"
            type="button"
            @click="closePanel"
          >
            {{ t("game.chatClose") }}
          </button>
        </header>

        <div class="flex gap-1 border-b border-[var(--app-border)] px-2 py-2">
          <button
            v-for="channel in channels"
            :key="channel.id"
            class="relative rounded-md px-2 py-1 text-xs"
            :class="channel.id === chatStore.activeChannel ? 'bg-[var(--app-primary)] text-white' : 'bg-[var(--app-bg)] text-[var(--app-text)]'"
            type="button"
            @click="selectChannel(channel.id)"
          >
            {{ localizeChannel(channel.label, channel.nameKey, channel.id) }}
            <span
              v-if="unreadCount(channel.id) > 0"
              class="absolute -right-1 -top-1 h-2 w-2 rounded-full bg-red-500"
            />
          </button>
        </div>

        <div ref="messageBoxRef" class="min-h-0 flex-1 overflow-y-auto bg-[var(--app-bg)] px-3 py-2">
          <div v-if="hasMore" class="mb-2 flex justify-center">
            <button
              class="rounded-md border border-[var(--app-border)] bg-[var(--app-surface)] px-2 py-1 text-xs text-[var(--app-text)]"
              type="button"
              @click="loadMore"
            >
              {{ t("game.chatLoadMore") }}
            </button>
          </div>

          <div v-if="messages.length === 0" class="py-10 text-center text-xs text-[var(--app-muted)]">
            {{ t("game.chatEmpty") }}
          </div>

          <ChatMessageItem
            v-for="message in messages"
            :key="`${message.id}-${message.ts}-${message.username}`"
            :message="message"
          />
        </div>

        <footer class="border-t border-[var(--app-border)] bg-[var(--app-surface)] p-2">
          <div v-if="!canSend" class="rounded-md bg-[var(--app-bg)] px-2 py-2 text-xs text-[var(--app-muted)]">
            {{ t("game.chatReadonlyHint") }}
          </div>
          <div v-else class="chat-composer">
            <ChatAttachmentCard
              v-if="pendingAttachment"
              :attachment="pendingAttachment"
              pending
              removable
              @activate="openAttachmentPicker"
              @remove="chatStore.clearPendingAttachment()"
            />
            <div class="chat-composer__row">
              <el-popover
                v-if="!isMobile"
                v-model:visible="attachmentPickerOpen"
                placement="top-start"
                :width="520"
                trigger="click"
                popper-class="chat-attachment-popover"
              >
                <template #reference>
                  <el-button
                    circle
                    class="chat-composer__insert"
                    data-testid="chat-insert-trigger"
                    :title="t('game.chatInsertContent')"
                    :aria-label="t('game.chatInsertContent')"
                  >
                    <Icon icon="lucide:plus" />
                  </el-button>
                </template>
                <div class="chat-attachment-popover__header">
                  <strong>{{ t("game.chatInsertTitle") }}</strong>
                  <el-button
                    text
                    circle
                    size="small"
                    :aria-label="t('game.chatClose')"
                    @click="attachmentPickerOpen = false"
                  >
                    <Icon icon="lucide:x" />
                  </el-button>
                </div>
                <ChatAttachmentPicker
                  :active="attachmentPickerOpen"
                  @select="selectAttachment"
                />
              </el-popover>

              <el-button
                v-else
                circle
                class="chat-composer__insert"
                data-testid="chat-insert-trigger"
                :title="t('game.chatInsertContent')"
                :aria-label="t('game.chatInsertContent')"
                @click="openAttachmentPicker"
              >
                <Icon icon="lucide:plus" />
              </el-button>
              <el-input
                ref="draftInputRef"
                v-model="draft"
                class="chat-composer__input"
                :maxlength="chatStore.maxContentLength"
                :placeholder="t('game.chatPlaceholder')"
                size="small"
                @keyup.enter="sendMessage"
              />
              <el-button class="chat-composer__send" size="small" type="primary" :disabled="!canSubmit" @click="sendMessage">
                <Icon icon="lucide:send" />{{ t("game.chatSend") }}
              </el-button>
            </div>
          </div>
        </footer>
      </section>
    </transition>

    <el-drawer
      v-if="isMobile"
      v-model="attachmentPickerOpen"
      class="chat-attachment-drawer"
      direction="btt"
      size="70%"
      append-to-body
      :title="t('game.chatInsertTitle')"
      :close-on-click-modal="true"
      :close-on-press-escape="true"
      modal-class="chat-attachment-drawer-overlay"
    >
      <ChatAttachmentPicker
        :active="attachmentPickerOpen"
        @select="selectAttachment"
      />
    </el-drawer>
  </div>
</template>

<script setup lang="ts">
import { computed, nextTick, onMounted, ref, watch } from "vue"
import { ElMessage } from "element-plus"
import { useI18n } from "vue-i18n"
import { send } from "@services/ws"
import { resolveErrorMessage } from "@services/error-messages"
import { useGameStore } from "@stores/game"
import { useChatStore } from "@stores/chat"
import { Icon } from "@iconify/vue"
import { useDevice } from "@composables/game/useDevice"
import { toChatAttachmentRequest, type ChatAttachment } from "@services/chat-attachments"
import ChatAttachmentCard from "./ChatAttachmentCard.vue"
import ChatAttachmentPicker from "./ChatAttachmentPicker.vue"
import ChatMessageItem from "./ChatMessageItem.vue"

const { t } = useI18n()
const gameStore = useGameStore()
const chatStore = useChatStore()
const { isMobile } = useDevice()

const draft = ref("")
const messageBoxRef = ref<HTMLElement | null>(null)
const draftInputRef = ref<{ focus?: () => void } | null>(null)
const attachmentPickerOpen = ref(false)

const channels = computed(() => chatStore.channels)
const messages = computed(() => chatStore.activeMessages)
const hasMore = computed(() => chatStore.hasMoreByChannel[chatStore.activeChannel] || false)
const canSend = computed(() => chatStore.canSendToChannel(chatStore.activeChannel))
const showGlobalUnreadDot = computed(() => !chatStore.panelOpen && chatStore.totalUnread > 0)
const pendingAttachment = computed(() => chatStore.pendingAttachment)
const canSubmit = computed(() => Boolean(draft.value.trim() || pendingAttachment.value))

onMounted(() => {
  gameStore.restoreToken()
  if (gameStore.token && !chatStore.initialized) {
    void chatStore.initialize(gameStore.token)
  }
})

watch(
  () => chatStore.panelOpen,
  (open) => {
    if (!open) {
      attachmentPickerOpen.value = false
      return
    }
    if (gameStore.token) {
      void chatStore.loadChannelHistory(gameStore.token, chatStore.activeChannel)
    }
    void scrollToBottom()
  }
)

watch(
  () => chatStore.activeChannel,
  () => {
    if (gameStore.token) {
      void chatStore.loadChannelHistory(gameStore.token, chatStore.activeChannel)
    }
    void scrollToBottom()
  }
)

watch(
  () => messages.value.length,
  () => {
    if (!chatStore.panelOpen) return
    void scrollToBottom()
  }
)

async function scrollToBottom() {
  await nextTick()
  if (!messageBoxRef.value) return
  messageBoxRef.value.scrollTop = messageBoxRef.value.scrollHeight
}

function localizeChannel(label: string | undefined, nameKey: string, fallbackId: string): string {
  const fixedLabel = String(label || "").trim()
  if (fixedLabel) return fixedLabel
  const localized = t(nameKey)
  if (localized !== nameKey && localized.trim() !== "") return localized
  return fallbackId
}

function unreadCount(channelID: string): number {
  return chatStore.unreadByChannel[channelID] || 0
}

function togglePanel() {
  chatStore.togglePanel()
}

function closePanel() {
  chatStore.setPanelOpen(false)
}

function selectChannel(channelID: string) {
  chatStore.setActiveChannel(channelID)
}

function openAttachmentPicker() {
  if (isMobile.value && typeof document !== "undefined") {
    const activeElement = document.activeElement
    if (activeElement instanceof HTMLElement) activeElement.blur()
  }
  attachmentPickerOpen.value = true
}

async function selectAttachment(attachment: ChatAttachment) {
  chatStore.setPendingAttachment(attachment)
  attachmentPickerOpen.value = false
  if (isMobile.value) return
  await nextTick()
  draftInputRef.value?.focus?.()
}

async function loadMore() {
  if (!gameStore.token) return
  try {
    await chatStore.loadMoreHistory(gameStore.token, chatStore.activeChannel)
  } catch (err) {
    ElMessage.error(resolveErrorMessage(t, err, "game.chatLoadFailed"))
  }
}

function sendMessage() {
  if (!canSend.value || !gameStore.token) return
  const content = draft.value.trim()
  const attachment = pendingAttachment.value
  if (!content && !attachment) return
  if (content.length > chatStore.maxContentLength) {
    ElMessage.error(t("game.chatTooLong", { max: chatStore.maxContentLength }))
    return
  }
  send("chat.send", {
    channel: chatStore.activeChannel,
    content,
    ...(attachment ? { attachment: toChatAttachmentRequest(attachment) } : {})
  })
  draft.value = ""
  attachmentPickerOpen.value = false
  chatStore.clearPendingAttachment()
}
</script>

<style scoped>
.chat-composer {
  display: grid;
  gap: 8px;
}

.chat-composer__row {
  min-width: 0;
  display: flex;
  align-items: center;
  gap: 6px;
}

.chat-composer__insert {
  flex: 0 0 auto;
  width: 32px;
  height: 32px;
  color: var(--app-text);
}

.chat-composer__input {
  min-width: 0;
}

.chat-composer__send {
  flex: 0 0 auto;
}

.chat-panel-fade-enter-active,
.chat-panel-fade-leave-active {
  transition: opacity 0.15s ease, transform 0.15s ease;
}

.chat-panel-fade-enter-from,
.chat-panel-fade-leave-to {
  opacity: 0;
  transform: translateY(8px);
}

:global(.chat-attachment-popover.el-popper) {
  max-width: calc(100vw - 24px);
  padding: 10px 12px 12px;
  border-color: var(--app-border);
  background: var(--app-surface);
}

.chat-attachment-popover__header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  min-height: 32px;
  margin-bottom: 4px;
  color: var(--app-text);
  font-size: 0.82rem;
}

:global(.chat-attachment-drawer.el-drawer) {
  max-height: 70dvh;
  border-radius: 8px 8px 0 0;
  background: var(--app-surface);
}

:global(.chat-attachment-drawer .el-drawer__header) {
  min-height: 48px;
  margin: 0;
  padding: 10px 14px;
  border-bottom: 1px solid var(--app-border);
  color: var(--app-text);
  font-size: 0.9rem;
  font-weight: 600;
}

:global(.chat-attachment-drawer .el-drawer__body) {
  min-height: 0;
  padding: 10px 12px max(12px, env(safe-area-inset-bottom));
  overflow: hidden;
}

@media (max-width: 767px) {
  .chat-composer__insert {
    width: 44px;
    height: 44px;
  }

  .chat-composer__input :deep(.el-input__wrapper),
  .chat-composer__send {
    min-height: 44px;
  }
}
</style>
