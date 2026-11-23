<template>
  <div class="chat-attachment-card" :style="accentStyle">
    <button
      class="chat-attachment-card__main"
      type="button"
      :aria-label="t('game.chatAttachmentOpen', { name: title })"
      @click="emit('activate')"
    >
      <span class="chat-attachment-card__icon" aria-hidden="true">
        <Icon v-if="isReplay" icon="lucide:film" />
        <FishMutationArt
          v-else-if="attachedFish"
          :src="attachedFish.iconPath"
          :fish="attachedFish.data"
          width="30px"
        />
        <TypeIcon
          v-else
          :type="resolvedIcon.type"
          :id="resolvedIcon.id"
          :alt="title"
          :size="28"
          :fallback="resolvedIcon.fallback"
        />
      </span>
      <span class="chat-attachment-card__copy">
        <strong :class="titleClass">{{ title }}</strong>
        <small>{{ subtitle }}</small>
      </span>
    </button>

    <el-tooltip v-if="removable" :content="t('game.chatAttachmentRemove')" placement="top">
      <el-button
        text
        circle
        class="chat-attachment-card__control"
        :aria-label="t('game.chatAttachmentRemove')"
        @click.stop="emit('remove')"
      >
        <Icon icon="lucide:x" />
      </el-button>
    </el-tooltip>
    <el-tooltip v-else :content="t('game.chatAttachmentOpen', { name: title })" placement="top">
      <el-button
        text
        circle
        class="chat-attachment-card__control"
        :aria-label="t('game.chatAttachmentOpen', { name: title })"
        @click.stop="emit('activate')"
      >
        <Icon :icon="isReplay ? 'lucide:play' : 'lucide:chevron-right'" />
      </el-button>
    </el-tooltip>
  </div>
</template>

<script setup lang="ts">
import { computed } from "vue"
import { useI18n } from "vue-i18n"
import { Icon } from "@iconify/vue"
import TypeIcon from "@components/game/icon/TypeIcon.vue"
import FishMutationArt from "@components/game/fishing/FishMutationArt.vue"
import { getActionDefinition } from "@services/action-definition"
import { getFishSpeciesDisplay, localizedFishText } from "@services/fish-instance-display"
import { itemCustomLabelClass, itemCustomName } from "@services/item-customization"
import { getLocalizedItemName } from "@services/item-label"
import {
  parseIconId,
  resolveFallbackIconByIconId,
  resolveIconIdByItemId,
  resolveItemFallbackIcon
} from "@services/item-icons"
import type { ChatAttachment } from "@services/chat-attachments"

const props = withDefaults(defineProps<{
  attachment: ChatAttachment
  removable?: boolean
  pending?: boolean
}>(), {
  removable: false,
  pending: false
})

const emit = defineEmits<{
  (e: "activate"): void
  (e: "remove"): void
}>()

const { t, locale } = useI18n()

const isReplay = computed(() => props.attachment.type === "arena_replay" || props.attachment.type === "combat_trial_replay")
const attachedFish = computed(() => {
  const attachment = props.attachment
  if (attachment.type !== "item_instance" || !attachment.preview.fishData) return null
  const species = getFishSpeciesDisplay(attachment.preview.fishData.speciesId || "")
  return {
    data: attachment.preview.fishData,
    iconPath: species?.iconPath || "/assets/icons/item/fish_catch.png"
  }
})
const title = computed(() => {
  const attachment = props.attachment
  if (attachment.type === "arena_replay") {
    return t("game.arena.chatReplayTitle", { id: attachment.challengeId })
  }
  if (attachment.type === "combat_trial_replay") {
    return t("game.combatTrial.chatReplayTitle", { id: attachment.runId })
  }
  if (attachment.type === "action") {
    const action = getActionDefinition(attachment.actionId)
    if (action?.nameKey) {
      const translated = t(action.nameKey)
      if (translated !== action.nameKey && translated.trim()) return translated
    }
    const fallbackKey = `action.${attachment.actionId}.name`
    const fallback = t(fallbackKey)
    return fallback !== fallbackKey && fallback.trim() ? fallback : attachment.actionId
  }
  const baseName = getLocalizedItemName(t, attachment.itemId, { fallbackText: attachment.itemId })
  if (attachment.type === "item") return baseName
  const customName = itemCustomName(attachment.preview.customData)
  if (customName) return customName
  const fish = getFishSpeciesDisplay(attachment.preview.fishData?.speciesId || "")
  return localizedFishText(fish?.names, locale.value, baseName)
})

const titleClass = computed(() => {
  if (props.attachment.type !== "item_instance") return ""
  return itemCustomLabelClass(props.attachment.preview.customData)
})

const subtitle = computed(() => {
  const attachment = props.attachment
  if (attachment.type === "item") return t("game.chatAttachmentItemReference")
  if (attachment.type === "item_instance") {
    const parts = [t("game.chatAttachmentInventoryItem")]
    const enhanceLevel = Math.max(0, Number(attachment.preview.enhanceLevel || 0))
    if (enhanceLevel > 0) parts.push(`+${enhanceLevel}`)
    return parts.join(" · ")
  }
  if (attachment.type === "action") {
    const action = getActionDefinition(attachment.actionId)
    const mainType = String(action?.mainType || action?.menu || "").trim()
    if (!mainType) return t("game.chatAttachmentAction")
    const key = `action_main_type.${mainType}.name`
    const localized = t(key)
    return localized !== key && localized.trim()
      ? `${t("game.chatAttachmentAction")} · ${localized}`
      : t("game.chatAttachmentAction")
  }
  if (!props.pending) return t("game.chatAttachmentReplay")
  return `${replayOutcome.value} · ${replayRetention.value}`
})

const replayOutcome = computed(() => {
  const attachment = props.attachment
  if (attachment.type === "combat_trial_replay") {
    const labelKey = String(attachment.outcomeLabelKey || "").trim()
    if (labelKey) {
      const translated = t(labelKey)
      if (translated && translated !== labelKey) return translated
    }
    switch (attachment.outcome) {
      case "team_defeated": return t("game.combatTrial.endReason.teamDefeated")
      case "enemy_team_defeated": return t("game.combatTrial.endReason.enemyTeamDefeated")
      case "action_budget_reached": return t("game.combatTrial.endReason.actionBudgetReached")
      default: return t("game.combatTrial.status.completed")
    }
  }
  if (attachment.type === "arena_replay") {
    if (attachment.outcome === "attacker_win") return t("game.arena.attackerWin")
    if (attachment.outcome === "defender_win") return t("game.arena.defenderWin")
  }
  return t("game.arena.failed")
})

const replayRetention = computed(() => t(props.attachment.type === "combat_trial_replay"
  ? "game.combatTrial.replayRetentionHint"
  : "game.arena.replayRetentionHint"))

const resolvedIcon = computed(() => {
  const attachment = props.attachment
  if (attachment.type === "item" || attachment.type === "item_instance") {
    const iconKey = resolveIconIdByItemId(attachment.itemId)
    const parsed = parseIconId(iconKey)
    return {
      type: parsed?.type || "item",
      id: parsed?.id || attachment.itemId,
      fallback: resolveItemFallbackIcon(attachment.itemId)
    }
  }
  if (attachment.type === "action") {
    const iconKey = getActionDefinition(attachment.actionId)?.iconId || `action.${attachment.actionId}`
    const parsed = parseIconId(iconKey)
    return {
      type: parsed?.type || "action",
      id: parsed?.id || attachment.actionId,
      fallback: resolveFallbackIconByIconId(iconKey, "?")
    }
  }
  return { type: "item", id: "", fallback: "?" }
})

const accentStyle = computed(() => {
  const type = props.attachment.type
  const accent = type === "item"
    ? "var(--el-color-info)"
    : type === "item_instance"
      ? "var(--el-color-success)"
      : type === "action"
        ? "var(--app-primary)"
        : "var(--el-color-warning)"
  return { "--chat-attachment-accent": accent }
})
</script>

<style scoped>
.chat-attachment-card {
  min-width: 0;
  display: grid;
  grid-template-columns: minmax(0, 1fr) 34px;
  align-items: center;
  gap: 6px;
  padding: 6px 7px;
  border-left: 3px solid var(--chat-attachment-accent);
  border-radius: 4px;
  background: color-mix(in srgb, var(--chat-attachment-accent) 7%, var(--app-bg));
}

.chat-attachment-card__main {
  min-width: 0;
  display: grid;
  grid-template-columns: 34px minmax(0, 1fr);
  align-items: center;
  gap: 8px;
  padding: 0;
  border: 0;
  color: inherit;
  background: transparent;
  text-align: left;
  cursor: pointer;
}

.chat-attachment-card__main:focus-visible {
  outline: 2px solid var(--app-primary);
  outline-offset: 2px;
}

.chat-attachment-card__icon {
  width: 32px;
  height: 32px;
  display: grid;
  place-items: center;
  overflow: hidden;
  color: var(--chat-attachment-accent);
  background: color-mix(in srgb, var(--chat-attachment-accent) 12%, transparent);
}

.chat-attachment-card__icon > :deep(svg) {
  width: 19px;
  height: 19px;
}

.chat-attachment-card__copy {
  min-width: 0;
  display: grid;
  gap: 2px;
}

.chat-attachment-card__copy strong,
.chat-attachment-card__copy small {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.chat-attachment-card__copy strong {
  color: var(--app-text);
  font-size: 0.75rem;
  line-height: 1.25rem;
}

.chat-attachment-card__copy strong.item-custom-label {
  color: var(--item-custom-label-color);
}

.chat-attachment-card__copy small {
  color: var(--app-muted);
  font-size: 0.64rem;
  line-height: 1rem;
}

.chat-attachment-card__control {
  width: 32px;
  height: 32px;
  color: var(--app-muted);
}

@media (max-width: 767px) {
  .chat-attachment-card__control {
    width: 38px;
    height: 38px;
  }
}
</style>
