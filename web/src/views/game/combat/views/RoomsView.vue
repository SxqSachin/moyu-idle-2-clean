<template>
  <div class="space-y-3" data-tutorial="combat-tab-rooms">
    <div class="mb-3 flex flex-wrap items-center justify-between gap-2">
      <el-select
        v-model="selectedWorldFilter"
        clearable
        filterable
        class="w-56"
        :placeholder="t('game.battleAreaSelectPlaceholder')"
      >
        <el-option :label="t('game.battleRoomDungeonAll')" value="" />
        <el-option v-for="world in worlds" :key="world.id" :label="world.label" :value="world.id" />
      </el-select>
    </div>

    <div v-if="publicRooms.length === 0" class="py-8">
      <el-empty :description="t('game.battleRoomListEmpty')" />
    </div>
    <div v-else class="room-grid">
      <div v-for="room in publicRooms" :key="room.id" class="room-card">
        <div class="room-card__header">
          <span class="room-card__name">{{ room.roomName }}</span>
          <el-tag size="small" :type="statusType(room.status)">{{ roomStatusLabel(room.status) }}</el-tag>
        </div>
        <div class="room-card__body">
          <div class="room-card__field">
            <span class="room-card__label">{{ t('game.battleRoomDungeon') }}</span>
            <span>{{ resolveDungeonLabel(room.dungeonId) }}</span>
          </div>
          <div class="room-card__field">
            <span class="room-card__label">{{ t('game.battleRoomMembersTitle') }}</span>
            <span>{{ Number(room.memberCount || 0) }}/{{ resolveWorldMaxTeamSize(room.dungeonId) }}</span>
          </div>
        </div>
        <div class="room-card__footer">
          <el-button size="small" type="primary" :disabled="!canJoinRoom(room)" @click="joinPublicRoom(room.id)">
            {{ t("game.battleJoinRoom") }}
          </el-button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from "vue"
import { useI18n } from "vue-i18n"
import { useCombatViewContext } from "../useCombatViewContext"

const { t } = useI18n()
const {
  selectedWorldID,
  worlds,
  publicRooms,
  myRoomDetail,
  roomStatusLabel,
  loadRooms,
  joinRoom,
  resolveWorldMaxTeamSize,
  setActiveTab
} = useCombatViewContext()

const selectedWorldFilter = computed({
  get: () => selectedWorldID.value,
  set: (value: string | undefined) => {
    selectedWorldID.value = typeof value === "string" ? value : ""
  }
})

function resolveDungeonLabel(dungeonId: string): string {
  const normalized = String(dungeonId || "").trim()
  if (!normalized) return "-"
  const world = worlds.value.find((item) => item.id === normalized)
  if (world?.label) return world.label
  const key = `world.${normalized}.name`
  const localized = t(key)
  if (localized !== key && localized.trim() !== "") return localized
  return normalized
}

function canJoinRoom(room: { id: number; isPublic: boolean; status: string; memberCount?: number; dungeonId?: string }) {
  if (!room.isPublic) return false
  if (room.status === "playing") return false
  if (room.status === "recovering") return false
  if (Number(room.memberCount || 0) >= resolveWorldMaxTeamSize(room.dungeonId || "")) return false
  if (myRoomDetail.value?.room.id) return false
  return true
}

function statusType(status: string): "" | "success" | "warning" | "danger" | "info" {
  switch (status) {
    case "idle":
      return "success"
    case "preparing":
      return "info"
    case "playing":
      return "warning"
    case "recovering":
      return "danger"
    default:
      return "info"
  }
}

async function joinPublicRoom(roomID: number) {
  if (!roomID) return
  const detail = await joinRoom(roomID)
  if (!detail) return
  setActiveTab("mine")
}
</script>

<style scoped>
.room-grid {
  display: grid;
  grid-template-columns: repeat(1, 1fr);
  gap: 12px;
}

@media (min-width: 640px) {
  .room-grid {
    grid-template-columns: repeat(2, 1fr);
  }
}

@media (min-width: 1024px) {
  .room-grid {
    grid-template-columns: repeat(3, 1fr);
  }
}

.room-card {
  display: flex;
  flex-direction: column;
  padding: 12px;
  border: 1px solid var(--app-border);
  border-radius: 8px;
  background: var(--app-surface);
  transition: all 0.2s ease;
}

.room-card:hover {
  border-color: var(--app-primary);
  box-shadow: var(--game-accent-glow);
}

.room-card__header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  margin-bottom: 8px;
}

.room-card__name {
  font-weight: 600;
  font-size: 14px;
  color: var(--app-text);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  flex: 1;
}

.room-card__body {
  display: flex;
  flex-direction: column;
  gap: 4px;
  flex: 1;
  font-size: 13px;
}

.room-card__field {
  display: flex;
  justify-content: space-between;
  gap: 8px;
}

.room-card__label {
  color: var(--app-muted);
}

.room-card__footer {
  margin-top: 12px;
  display: flex;
  justify-content: flex-end;
}
</style>
