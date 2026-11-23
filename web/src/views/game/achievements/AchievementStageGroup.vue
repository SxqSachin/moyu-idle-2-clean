<template>
  <div class="relative h-full">
    <AchievementCardContent
      :achievement="primaryItem"
      :claiming="!!claimingIds[primaryItem.id]"
      @claim="$emit('claim', $event)"
    />

    <el-popover
      trigger="hover"
      placement="top-end"
      :width="340"
      popper-class="achievement-stage-popover"
    >
      <template #reference>
        <button
          type="button"
          class="absolute right-2 top-2 z-10 rounded-full border border-[var(--app-border)] bg-[var(--app-surface)]/95 px-2 py-0.5 font-bold text-xs text-[var(--app-text)] shadow-sm transition hover:border-blue-400 hover:text-blue-600"
        >
          {{ t("achievements.stageCompact", { current: primaryItem.stageIndex || 1, total: allItems.length }) }}
        </button>
      </template>

      <div class="space-y-2">
        <div class="flex items-center justify-between gap-3 border-b border-[var(--app-border)] pb-2">
          <div class="min-w-0">
            <div class="truncate text-sm font-extrabold text-[var(--app-text)]">{{ groupTitle }}</div>
            <div class="text-xs text-[var(--app-muted)]">
              {{ t("achievements.stageGroupSummary", { completed: completedCount, total: allItems.length }) }}
            </div>
          </div>
        </div>

        <div class="max-h-[320px] space-y-2 overflow-auto p-1">
          <div
            v-for="stage in allItems"
            :key="stage.id"
            class="rounded-md border border-[var(--app-border)] bg-[var(--app-bg)] p-2"
            :class="{ 'ring-1 ring-blue-400': stage.id === primaryItem.id }"
          >
            <div class="flex items-center justify-between gap-2">
              <div class="min-w-0 text-xs font-bold text-[var(--app-text)]">
                {{ t("achievements.stageLabel", { current: stage.stageIndex || 1, total: allItems.length }) }}
              </div>
              <el-tag v-if="stage.claimed" size="small" round effect="plain">{{ t("achievements.statusClaimed") }}</el-tag>
              <el-tag v-else-if="stage.completed" type="success" size="small" round effect="light">{{ t("achievements.statusClaimable") }}</el-tag>
              <el-tag v-else-if="stage.locked" size="small" round effect="plain">{{ t("achievements.statusLocked") }}</el-tag>
            </div>
            <div class="mt-1 flex items-center justify-between gap-2 text-xs text-[var(--app-muted)]">
              <span class="truncate">{{ stageConditionText(stage) }}</span>
              <span class="shrink-0 tabular-nums">{{ progressText(stage) }}</span>
            </div>
            <div class="mt-1.5 h-1.5 overflow-hidden rounded-full bg-black/10">
              <div
                class="h-full rounded-full"
                :class="stage.completed ? 'bg-green-500' : 'bg-blue-500'"
                :style="{ width: `${progressPercent(stage)}%` }"
              />
            </div>
            <div v-if="stage.rewards.length > 0 || (stage.completed && !stage.claimed)" class="mt-2 flex items-center justify-between gap-2">
              <div class="flex min-w-0 flex-wrap items-center gap-1">
                <template v-for="(reward, index) in stage.rewards" :key="rewardKey(reward, index)">
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
                </template>
              </div>
              <el-button
                v-if="stage.completed && !stage.claimed"
                size="small"
                type="success"
                plain
                :loading="!!claimingIds[stage.id]"
                @click="$emit('claim', stage.id)"
              >
                {{ t("achievements.claim") }}
              </el-button>
            </div>
          </div>
        </div>
      </div>
    </el-popover>
  </div>
</template>

<script setup lang="ts">
import { computed } from "vue"
import { useI18n } from "vue-i18n"
import type { PlayerAchievementView } from "@stores/achievements"
import ItemQuickRef from "@components/game/item/QuickRef.vue"
import TitleQuickRef from "@components/game/wardrobe/TitleQuickRef.vue"
import { resolveAchievementTitle, resolveAchievementDescription } from "@services/achievement-text"
import AchievementCardContent from "./AchievementCardContent.vue"

const props = defineProps<{
  visibleItems: PlayerAchievementView[]
  allItems: PlayerAchievementView[]
  claimingIds: Record<string, boolean>
}>()

defineEmits<{
  claim: [achievementId: string]
}>()

const { t, locale } = useI18n()

const sortedAllItems = computed(() => [...props.allItems].sort(sortByStage))
const primaryItem = computed(() => {
  return sortedAllItems.value.find((item) => !item.completed && !item.locked)
    ?? sortedAllItems.value.find((item) => item.completed && !item.claimed)
    ?? sortedAllItems.value[0]
    ?? props.allItems[0]
})
const allItems = computed(() => sortedAllItems.value)
const completedCount = computed(() => allItems.value.filter((item) => item.completed).length)
const groupTitle = computed(() => primaryItem.value ? resolveAchievementTitle(t, primaryItem.value.nameKey, primaryItem.value.condition || {}, primaryItem.value.baseAchievementId || primaryItem.value.id, locale.value) : "")

function sortByStage(a: PlayerAchievementView, b: PlayerAchievementView) {
  return Number(a.stageIndex || 0) - Number(b.stageIndex || 0) || a.id.localeCompare(b.id)
}

function stageConditionText(stage: PlayerAchievementView) {
  const resolved = resolveAchievementDescription(t, "", stage.condition || {}, locale.value)
  if (resolved) return resolved
  return t(`achievements.condition.${String(stage.condition?.kind || "unknown")}`)
}

function progressPercent(stage: PlayerAchievementView) {
  const target = Math.max(1, Number(stage.targetValue || 1))
  return Math.max(0, Math.min(100, Math.floor((Number(stage.progressValue || 0) / target) * 100)))
}

function progressText(stage: PlayerAchievementView) {
  return `${Math.min(Number(stage.progressValue || 0), Number(stage.targetValue || 0))} / ${stage.targetValue}`
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
