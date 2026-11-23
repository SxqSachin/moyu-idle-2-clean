<template>
  <div class="battle-room-member-card">
    <div class="flex flex-col gap-1">
      <div class="text-[14px] font-medium flex flex-row items-center">
        <span>
          {{ memberName }}
        </span>
        <el-tag v-if="isKitty" size="small" type="warning" effect="plain" class="ml-2">{{ t("game.catteryKittyLabel") }}</el-tag>

        <template v-if="member.isReady">
          <el-tag size="small" type="success" class="ml-2">{{ t("game.battleReadyGetReady") }}</el-tag>
        </template>
        <template v-else>
          <el-tag size="small" type="info" class="ml-2">{{ t("game.battleReadyNotReady") }}</el-tag>
        </template>
      </div>
      <div class="app-muted">{{ t("game.battleRoomPosition") }}：{{ positionLabel(member.position) }}</div>
      <!-- <div class="app-muted">{{ t("game.battleReady") }}：{{ member.isReady ? t("game.switchOn") : t("game.switchOff") }}</div> -->
    </div>

    <div v-if="isSelf" class="mt-2">
      <el-select
        :model-value="member.position"
        :disabled="roomStatus !== 'idle' && roomStatus !== 'preparing'"
        class="w-full"
        @change="handlePositionChange"
      >
        <el-option v-for="option in positionOptions" :key="option.value" :label="positionLabel(option.value)" :value="option.value">
          <div class="flex items-center gap-1">
            <span>{{ positionLabel(option.value) }}</span>
            <el-popover placement="right" trigger="hover" :width="300">
              <template #reference>
                <el-icon class="cursor-help text-[var(--app-muted)]">
                  <QuestionFilled />
                </el-icon>
              </template>
              <div class="battle-position-popover">
                <div class="battle-position-popover__title">{{ positionLabel(option.value) }}</div>
                <div class="battle-position-popover__desc">{{ t(option.descKey) }}</div>
                <BuffQuickRef :buff-id="option.buffId" :remain-turns="-1" />
              </div>
            </el-popover>
          </div>
        </el-option>
      </el-select>
      <div v-if="positionLockHint" class="mt-1 text-[11px] text-[var(--el-color-warning)]">
        {{ positionLockHint }}
      </div>
    </div>

    <div v-if="isSelf && isKitty" class="mt-2">
      <el-button
        size="small"
        :disabled="roomStatus !== 'idle' && roomStatus !== 'preparing'"
        @click="handleRemoveKitty"
      >
        {{ t("game.battleRoomRemoveKittyAction") }}
      </el-button>
    </div>

    <div v-if="isOwner && !isSelf && !isKitty" class="mt-2">
      <el-button size="small" type="danger" :disabled="roomStatus !== 'idle' && roomStatus !== 'preparing'" @click="$emit('kick', member.userId)">
        {{ t("game.battleKickMember") }}
      </el-button>
    </div>

    <slot></slot>
  </div>
</template>

<script setup lang="ts">
import { computed } from "vue"
import { useI18n } from "vue-i18n"
import { QuestionFilled } from "@element-plus/icons-vue"
import BuffQuickRef from "@components/game/buff/BuffQuickRef.vue"

const props = defineProps<{
  member: { userId: number; username?: string; position: string; isReady: boolean; unitType?: "player" | "kitty"; kittyName?: string; kittyId?: number; ownerUserId?: number; positionLockedByPersonalityId?: string; positionLockedByPersonalityKey?: string }
  myUserId: number
  isOwner: boolean
  roomStatus: string
  positionLabel: (position: string) => string
}>()

const { t } = useI18n()
const isKitty = computed(() => props.member.unitType === "kitty")
const isSelf = computed(() => (props.member.ownerUserId || props.member.userId) === props.myUserId)
const positionLockHint = computed(() => {
  if (!isKitty.value || !props.member.positionLockedByPersonalityId) return ""
  const personality = personalityLabel(props.member.positionLockedByPersonalityId, props.member.positionLockedByPersonalityKey)
  return t("game.battleRoomKittyPositionLockedHint", {
    personality,
    position: props.positionLabel(props.member.position)
  })
})
const positionOptions = [
  { value: "vanguard", buffId: "formation_frontline_guard", descKey: "game.battlePositionVanguardDesc" },
  { value: "midguard", buffId: "formation_midline_speed", descKey: "game.battlePositionMidguardDesc" },
  { value: "rearguard", buffId: "formation_rearline_amplify", descKey: "game.battlePositionRearguardDesc" }
] as const

const emit = defineEmits<{
  (e: "update-position", payload: { position: string; unitType: "player" | "kitty"; kittyId?: number }): void
  (e: "kick", userId: number): void
  (e: "remove-kitty", kittyId: number): void
}>()

function handlePositionChange(value: unknown) {
  const unitType = isKitty.value ? "kitty" : "player"
  const kittyId = Number(props.member.kittyId || 0)
  emit("update-position", {
    position: String(value || "vanguard"),
    unitType,
    kittyId: unitType === "kitty" && kittyId > 0 ? kittyId : undefined
  })
}

function handleRemoveKitty() {
  const kittyId = Number(props.member.kittyId || 0)
  if (kittyId <= 0) return
  emit("remove-kitty", kittyId)
}

const memberName = computed(() => {
  if (isKitty.value) {
    const kittyName = String(props.member.kittyName || "").trim()
    const displayId = Number(props.member.kittyId || props.member.userId || 0)
    return kittyName || `${t("game.catteryKittyLabel")} #${displayId > 0 ? displayId : "-"}`
  }
  const username = String(props.member.username || "").trim()
  if (username) return username
  return `${t("game.playerLabel")} ${props.member.userId}`
})

function personalityLabel(personalityID: string, nameKey?: string) {
  const normalizedID = String(personalityID || "").trim().toLowerCase()
  const normalizedKey = String(nameKey || "").trim()
  const key = normalizedKey || `kitty.personality.${normalizedID}.name`
  const localized = t(key)
  if (localized !== key && localized.trim()) return localized
  return normalizedID || "-"
}
</script>

<style scoped>
.battle-room-member-card {
  position: relative;
  border: 1px solid var(--app-border);
  border-radius: 8px;
  background: color-mix(in srgb, var(--app-surface) 94%, var(--app-bg));
  color: var(--app-text);
  padding: 8px;
  font-size: 12px;
}

.battle-position-popover {
  display: grid;
  gap: 8px;
  color: var(--el-text-color-regular);
  font-size: 12px;
}

.battle-position-popover__title {
  color: var(--app-text);
  font-weight: 600;
}

.battle-position-popover__desc {
  color: var(--app-muted);
}
</style>
