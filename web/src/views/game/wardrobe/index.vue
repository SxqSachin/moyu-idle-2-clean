<template>
  <div class="page-shell space-y-4">
    <el-card class="app-card app-card--no-border game-page-frame" shadow="never">
      <template #header>
        <GamePageHeader
          title-key="wardrobe.title"
          :visual-src="wardrobeVisual.src"
          :visual-tone="wardrobeVisual.tone"
          visual-key="wardrobe"
        >
          <template #actions>
          <el-button size="small" @click="refresh">{{ t("game.refresh") }}</el-button>
          </template>
        </GamePageHeader>
      </template>

      <el-tabs v-model="activeTab">
        <el-tab-pane :label="t('wardrobe.tabTitles')" name="titles" :lazy="true">
          <div class="mb-3 rounded-lg bg-[var(--app-surface-elevated)] p-3 text-center">
            <div class="text-xs text-[var(--app-muted)]">{{ t("wardrobe.preview") }}</div>
            <div class="mt-1 text-lg font-bold">{{ playerTitleDisplay || t("wardrobe.noTitle") }}</div>
          </div>

          <el-alert v-if="!hasOwnedTitle" class="mb-3" type="info" :closable="false" show-icon>
            <template #title>
              <router-link class="font-medium text-[var(--app-primary)]" to="/game/achievements">
                {{ t("wardrobe.emptyAchievementLink") }}
              </router-link>
            </template>
          </el-alert>

          <div class="grid grid-cols-1 gap-4 md:grid-cols-2">
            <div>
              <div class="mb-2 text-sm font-medium">{{ t("wardrobe.prefixes") }}</div>
              <TitleItemList
                :items="prefixItems"
                :equipped-id="wardrobe.playerTitlePrefix"
                category="title_prefix"
                @select="(id) => handleEquip('title_prefix', id)"
                @unequip="handleUnequip('title_prefix')"
              />
            </div>
            <div>
              <div class="mb-2 text-sm font-medium">{{ t("wardrobe.suffixes") }}</div>
              <TitleItemList
                :items="suffixItems"
                :equipped-id="wardrobe.playerTitleSuffix"
                category="title_suffix"
                @select="(id) => handleEquip('title_suffix', id)"
                @unequip="handleUnequip('title_suffix')"
              />
            </div>
          </div>
        </el-tab-pane>

        <el-tab-pane :label="t('wardrobe.tabKittyTitles')" name="kitty-titles" :lazy="true">
          <div v-if="!kitties.length" class="py-4 text-center text-sm text-[var(--app-muted)]">
            {{ t("wardrobe.noKitties") }}
          </div>
          <template v-else>
            <div class="mb-3">
              <el-select v-model="selectedKittyId" class="w-48" size="small">
                <el-option
                  v-for="kitty in kitties"
                  :key="kitty.id"
                  :label="kitty.name || `#${kitty.id}`"
                  :value="Number(kitty.id)"
                />
              </el-select>
            </div>

            <div class="mb-3 rounded-lg bg-[var(--app-surface-elevated)] p-3 text-center">
              <div class="text-xs text-[var(--app-muted)]">{{ t("wardrobe.preview") }}</div>
              <div class="mt-1 text-lg font-bold">{{ kittyTitleDisplay || t("wardrobe.noTitle") }}</div>
            </div>

            <div class="grid grid-cols-1 gap-4 md:grid-cols-2">
              <div>
                <div class="mb-2 text-sm font-medium">{{ t("wardrobe.prefixes") }}</div>
                <TitleItemList
                  :items="prefixItems"
                  :equipped-id="kittyEquippedPrefix"
                  category="title_prefix"
                  @select="(id) => handleKittyEquip('kitty_title_prefix', id)"
                  @unequip="handleKittyUnequip('kitty_title_prefix')"
                />
              </div>
              <div>
                <div class="mb-2 text-sm font-medium">{{ t("wardrobe.suffixes") }}</div>
                <TitleItemList
                  :items="suffixItems"
                  :equipped-id="kittyEquippedSuffix"
                  category="title_suffix"
                  @select="(id) => handleKittyEquip('kitty_title_suffix', id)"
                  @unequip="handleKittyUnequip('kitty_title_suffix')"
                />
              </div>
            </div>
          </template>
        </el-tab-pane>
      </el-tabs>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from "vue"
import { useI18n } from "vue-i18n"
import { ElMessage } from "element-plus"
import { useWardrobeStore } from "@stores/wardrobe"
import { usePlayerStore } from "@stores/player"
import { useGameStore } from "@stores/game"
import { getGameDataTable } from "@services/game-data-runtime"
import TitleItemList from "./TitleItemList.vue"
import GamePageHeader from "@components/game/common/GamePageHeader.vue"
import { gamePageVisuals } from "@services/game-page-visuals"

const { t } = useI18n()
const wardrobeVisual = gamePageVisuals.wardrobe
const wardrobe = useWardrobeStore()
const playerStore = usePlayerStore()
const gameStore = useGameStore()

const activeTab = ref("titles")
const selectedKittyId = ref(0)
const busy = ref(false)

type TitleEntry = { id: string; nameKey: string; rarity: string; category?: string; order: number; disabled: boolean }
let allPrefixesCache: TitleEntry[] | null = null
let allSuffixesCache: TitleEntry[] | null = null

function allPrefixes() {
  if (!allPrefixesCache) {
    allPrefixesCache = getGameDataTable<TitleEntry>("title_prefixes").filter((e) => !e.disabled)
  }
  return allPrefixesCache
}

function allSuffixes() {
  if (!allSuffixesCache) {
    allSuffixesCache = getGameDataTable<TitleEntry>("title_suffixes").filter((e) => !e.disabled)
  }
  return allSuffixesCache
}

const kitties = computed(() => playerStore.kitties ?? [])
const prefixItems = computed(() =>
  allPrefixes().map((e) => ({
    ...e,
    owned: wardrobe.owned.some((o) => o.category === "title_prefix" && o.itemId === e.id),
    label: t(e.nameKey)
  }))
)
const suffixItems = computed(() =>
  allSuffixes().map((e) => ({
    ...e,
    owned: wardrobe.owned.some((o) => o.category === "title_suffix" && o.itemId === e.id),
    label: t(e.nameKey)
  }))
)
const hasOwnedTitle = computed(() => wardrobe.owned.some((item) => item.category === "title_prefix" || item.category === "title_suffix"))

const playerTitleDisplay = computed(() => {
  const prefix = allPrefixes().find((e) => e.id === wardrobe.playerTitlePrefix)
  const suffix = allSuffixes().find((e) => e.id === wardrobe.playerTitleSuffix)
  const parts: string[] = []
  if (prefix) parts.push(t(prefix.nameKey))
  if (suffix) parts.push(t(suffix.nameKey))
  return parts.join("")
})

const kittyEquippedPrefix = computed(() => wardrobe.equippedMap["kitty_title_prefix"]?.itemId ?? "")
const kittyEquippedSuffix = computed(() => wardrobe.equippedMap["kitty_title_suffix"]?.itemId ?? "")

const kittyTitleDisplay = computed(() => {
  const prefix = allPrefixes().find((e) => e.id === kittyEquippedPrefix.value)
  const suffix = allSuffixes().find((e) => e.id === kittyEquippedSuffix.value)
  const parts: string[] = []
  if (prefix) parts.push(t(prefix.nameKey))
  if (suffix) parts.push(t(suffix.nameKey))
  return parts.join("")
})

async function refresh() {
  if (!gameStore.token) return
  await wardrobe.load(gameStore.token)
}

async function handleEquip(slotId: string, itemId: string) {
  if (!gameStore.token || busy.value) return
  busy.value = true
  try {
    await wardrobe.equip(gameStore.token, slotId, itemId)
    ElMessage.success(t("wardrobe.equipSuccess"))
  } catch {
    ElMessage.error(t("wardrobe.equipFailed"))
  } finally {
    busy.value = false
  }
}

async function handleUnequip(slotId: string) {
  if (!gameStore.token || busy.value) return
  busy.value = true
  try {
    await wardrobe.unequip(gameStore.token, slotId)
    ElMessage.success(t("wardrobe.unequipSuccess"))
  } catch {
    ElMessage.error(t("wardrobe.unequipFailed"))
  } finally {
    busy.value = false
  }
}

async function handleKittyEquip(slotId: string, itemId: string) {
  if (!gameStore.token || !selectedKittyId.value || busy.value) return
  busy.value = true
  try {
    await wardrobe.equipKitty(gameStore.token, selectedKittyId.value, slotId, itemId)
    ElMessage.success(t("wardrobe.equipSuccess"))
  } catch {
    ElMessage.error(t("wardrobe.equipFailed"))
  } finally {
    busy.value = false
  }
}

async function handleKittyUnequip(slotId: string) {
  if (!gameStore.token || !selectedKittyId.value || busy.value) return
  busy.value = true
  try {
    await wardrobe.unequipKitty(gameStore.token, selectedKittyId.value, slotId)
    ElMessage.success(t("wardrobe.unequipSuccess"))
  } catch {
    ElMessage.error(t("wardrobe.unequipFailed"))
  } finally {
    busy.value = false
  }
}

onMounted(() => {
  if (!wardrobe.loaded && gameStore.token) {
    wardrobe.load(gameStore.token)
  }
  if (kitties.value.length && !selectedKittyId.value) {
    selectedKittyId.value = Number(kitties.value[0]?.id ?? 0)
  }
})
</script>
