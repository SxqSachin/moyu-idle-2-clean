<template>
  <div class="rounded-xl border border-dashed border-[var(--app-border)] bg-[var(--app-surface-elevated)] p-6 text-center space-y-2">
    <div class="text-base font-semibold">{{ t("game.catteryFriendsTitle") }}</div>
    <div class="app-muted">{{ t("game.catteryFeatureComingSoon") }}</div>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from "vue-i18n"

const { t } = useI18n()
</script>
