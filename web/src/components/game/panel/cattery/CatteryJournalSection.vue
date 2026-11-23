<template>
  <div class="space-y-3">
    <div class="rounded-xl border border-[var(--app-border)] bg-[var(--app-surface-elevated)] p-3">
      <div class="flex flex-wrap items-center justify-between gap-2">
        <div class="font-semibold">{{ t("game.catteryJournalTitle") }}</div>
        <el-select v-model="selectedKittyId" class="w-44" size="small" :disabled="!kitties.length">
          <el-option
            v-for="kitty in kitties"
            :key="kitty.id"
            :label="kitty.name || `#${kitty.id}`"
            :value="Number(kitty.id || 0)"
          />
        </el-select>
      </div>
    </div>

    <div v-if="!kitties.length" class="rounded-xl border border-dashed border-[var(--app-border)] p-4 app-muted">
      {{ t("game.catteryNoKitty") }}
    </div>

    <div v-else class="rounded-xl border border-[var(--app-border)] bg-[var(--app-surface-elevated)] p-3">
      <el-tabs v-model="activeJournalTab">
        <el-tab-pane :label="t('game.catteryDiaryTab')" name="journals" lazy>
          <CatteryDiaryList :selected-kitty-id="selectedKittyId" />
        </el-tab-pane>
        <el-tab-pane :label="t('game.catteryEncounterTab')" name="encounters" lazy>
          <CatteryEncounterList :selected-kitty-id="selectedKittyId" />
        </el-tab-pane>
      </el-tabs>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, watch } from "vue"
import { useI18n } from "vue-i18n"
import { usePlayerStore } from "@stores/player"
import CatteryDiaryList from "./CatteryDiaryList.vue"
import CatteryEncounterList from "./CatteryEncounterList.vue"

const { t } = useI18n()
const playerStore = usePlayerStore()
const activeJournalTab = ref<"journals" | "encounters">("journals")
const selectedKittyId = ref(0)
const kitties = computed(() => playerStore.kitties || [])

watch(
  () => kitties.value.map((kitty) => Number(kitty?.id || 0)).filter((id) => id > 0),
  (ids) => {
    if (!ids.length) {
      selectedKittyId.value = 0
      return
    }
    if (!ids.includes(selectedKittyId.value)) {
      selectedKittyId.value = ids[0]
    }
  },
  { immediate: true }
)
</script>
