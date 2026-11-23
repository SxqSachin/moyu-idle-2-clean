<template>
  <article
    class="relative flex h-full min-h-[98px] flex-col overflow-hidden rounded-lg border border-[var(--app-border)] bg-[var(--app-surface)] p-2.5 pl-4 shadow-sm transition hover:-translate-y-0.5 hover:shadow-md"
    :class="{ 'opacity-80': achievement.locked }"
    :style="cardStyle"
  >
    <span class="absolute bottom-0 left-0 top-0 w-1 bg-[var(--achievement-rarity)]" />

    <div class="flex min-w-0 flex-1 flex-col gap-2">
      <div class="flex items-start justify-between gap-2">
        <div class="min-w-0">
          <h3 class="truncate text-sm font-extrabold leading-5 tracking-tight text-[var(--app-text)]">
            {{ title }}
          </h3>
          <p v-if="description" class="mt-0.5 line-clamp-1 text-xs leading-5 text-[var(--app-muted)]">
            {{ description }}
          </p>
        </div>
        <el-tag v-if="statusLabel" :type="statusType" effect="light" round size="small">
          {{ statusLabel }}
        </el-tag>
      </div>

      <div class="mt-auto space-y-1">
        <div class="flex items-center justify-between gap-2 text-xs font-medium text-[var(--app-muted)]">
          <span class="min-w-0 font-normal text-[11px] truncate">{{ conditionText }}</span>
          <span class="shrink-0 tabular-nums font-normal text-[11px] text-[var(--app-text)]/80">{{ progressText }}</span>
        </div>
        <div class="h-1 overflow-hidden rounded-full bg-black/10">
          <div
            class="h-full rounded-full transition-[width]"
            :class="achievement.completed ? 'bg-green-500' : 'bg-blue-500'"
            :style="{ width: `${progressPercent}%` }"
          />
        </div>
      </div>

      <div class="flex items-center justify-between gap-2 pt-0.5">
        <div v-if="visibleRewards.length > 0" class="flex min-w-0 flex-wrap items-center gap-1 text-xs">
          <span class="font-semibold text-[var(--app-muted)]">{{ t("achievements.rewards") }}</span>
          <template v-for="(reward, index) in visibleRewards" :key="rewardKey(reward, index)">
            <ItemQuickRef
              v-if="rewardKind(reward) === 'item'"
              :item-id="rewardItemId(reward)"
              :count="rewardQuantity(reward)"
            />
            <TitleQuickRef
              v-else-if="rewardKind(reward) === 'wardrobe'"
              :category="rewardTitleCategory(reward)"
              :item-id="rewardItemId(reward)"
            />
            <span v-else class="rounded-full border border-[var(--app-border)] px-2 py-0.5 text-[var(--app-muted)]">
              {{ rewardKind(reward) || t("achievements.reward.unknown") }}
            </span>
          </template>
        </div>
        <span v-else class="min-w-0" />
        <el-button
          class="shrink-0"
          size="small"
          type="primary"
          plain
          :disabled="!achievement.completed || achievement.claimed"
          :loading="claiming"
          @click="$emit('claim', achievement.id)"
        >
          {{ achievement.claimed ? t("achievements.claimed") : t("achievements.claim") }}
        </el-button>
      </div>
    </div>
  </article>
</template>

<script setup lang="ts">
import { computed } from "vue"
import { useI18n } from "vue-i18n"
import type { PlayerAchievementView } from "@stores/achievements"
import ItemQuickRef from "@components/game/item/QuickRef.vue"
import TitleQuickRef from "@components/game/wardrobe/TitleQuickRef.vue"
import { getRarityDefinition } from "@services/item-definition"
import { resolveAchievementTitle, resolveAchievementDescription } from "@services/achievement-text"

const props = defineProps<{
  achievement: PlayerAchievementView
  claiming?: boolean
}>()

defineEmits<{
  claim: [achievementId: string]
}>()

const { t, locale } = useI18n()

const title = computed(() => resolveAchievementTitle(t, props.achievement.nameKey, props.achievement.condition || {}, props.achievement.id, locale.value))

const description = computed(() => {
  const custom = localizedOptional(props.achievement.descKey)
  if (custom) return custom
  return null; //resolveAchievementDescription(t, props.achievement.descKey, props.achievement.condition || {})
})
const conditionKind = computed(() => String(props.achievement.condition?.kind || "").trim())

const conditionText = computed(() => {
  const resolved = resolveAchievementDescription(t, "", props.achievement.condition || {}, locale.value)
  if (resolved) return resolved
  return conditionKind.value || t("achievements.condition.unknown")
})

const progressPercent = computed(() => {
  const target = Math.max(1, Number(props.achievement.targetValue || 1))
  return Math.max(0, Math.min(100, Math.floor((Number(props.achievement.progressValue || 0) / target) * 100)))
})

const progressText = computed(() => `${Math.min(props.achievement.progressValue, props.achievement.targetValue)} / ${props.achievement.targetValue}`)

const statusLabel = computed(() => {
  if (props.achievement.locked) return t("achievements.statusLocked")
  if (props.achievement.claimed) return t("achievements.statusClaimed")
  if (props.achievement.completed) return t("achievements.statusClaimable")
  if (props.achievement.progressValue >= 1) return t("achievements.statusInProgress")
  return ""
})

const statusType = computed(() => {
  if (props.achievement.completed && !props.achievement.claimed) return "success"
  if (props.achievement.progressValue >= 1 && !props.achievement.completed) return "warning"
  return "info"
})

const visibleRewards = computed(() => props.achievement.rewards)

const cardStyle = computed(() => ({
  "--achievement-rarity": getRarityDefinition(props.achievement.rarity || "common")?.color || "var(--app-primary)"
}))

function localizedOptional(key: unknown) {
  const normalized = String(key || "").trim()
  if (!normalized) return ""
  const localized = t(normalized)
  return localized === normalized ? "" : localized
}

function rewardKind(reward: Record<string, unknown>) {
  return String(reward.kind || "").trim()
}

function rewardItemId(reward: Record<string, unknown>) {
  return String(reward.itemId || "").trim()
}

function rewardQuantity(reward: Record<string, unknown>) {
  const parsed = Number(reward.quantity)
  return Number.isFinite(parsed) && parsed > 0 ? parsed : 1
}

function rewardTitleCategory(reward: Record<string, unknown>) {
  const category = String(reward.category || "").trim()
  return category === "title_prefix" || category === "title_suffix" ? category : "title_suffix"
}

function rewardKey(reward: Record<string, unknown>, index: number) {
  return `${rewardKind(reward)}:${rewardTitleCategory(reward)}:${rewardItemId(reward)}:${index}`
}
</script>
