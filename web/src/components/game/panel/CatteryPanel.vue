<template>
  <div class="space-y-4 text-sm" data-tutorial="cattery-page">
    <div class="rounded-2xl border border-[var(--app-border)] bg-[var(--app-surface-elevated)] p-4 shadow-sm" data-tutorial="cattery-summary">
      <div class="text-lg font-bold">{{ t("game.catteryTitle") }}</div>
      <div class="mt-2 leading-6 app-muted">
        {{ t("game.catteryIntro") }}
      </div>
    </div>

    <div class="rounded-2xl border border-[var(--app-border)] bg-[var(--app-surface)] p-3 shadow-sm" data-tutorial="cattery-tabs">
      <el-tabs v-model="activeTab" class="cattery-tabs">
        <el-tab-pane :label="t('game.catteryTabKitties')" name="kitties">
          <div data-tutorial="cattery-kitties">
            <CatteryKittiesSection />
          </div>
        </el-tab-pane>
        <el-tab-pane v-if="showCommissionTab" :label="t('game.catteryTabCommissions')" name="commissions" lazy>
          <CatteryCommissionSection />
        </el-tab-pane>
        <el-tab-pane :label="t('game.catteryTabFurniture')" name="furniture" lazy>
          <CatteryFurnitureSection :active="activeTab === 'furniture'" />
        </el-tab-pane>
        <el-tab-pane :label="t('game.catteryTabJournal')" name="journal" lazy>
          <div data-tutorial="cattery-journal">
            <CatteryJournalSection />
          </div>
        </el-tab-pane>
        <el-tab-pane :label="t('game.catteryTabFriends')" name="friends" lazy>
          <CatteryFriendsSection />
        </el-tab-pane>
      </el-tabs>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref } from "vue"
import { useI18n } from "vue-i18n"
import CatteryKittiesSection from "./cattery/CatteryKittiesSection.vue"
import CatteryCommissionSection from "./cattery/CatteryCommissionSection.vue"
import CatteryFurnitureSection from "./cattery/CatteryFurnitureSection.vue"
import CatteryJournalSection from "./cattery/CatteryJournalSection.vue"
import CatteryFriendsSection from "./cattery/CatteryFriendsSection.vue"

const { t } = useI18n()
const activeTab = ref("kitties")
const showCommissionTab = false
</script>
