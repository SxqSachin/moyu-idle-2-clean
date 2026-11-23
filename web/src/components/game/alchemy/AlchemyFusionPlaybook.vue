<template>
  <section class="inherit-playbook" data-tutorial="alchemy-inherit-playbook">
    <div class="inherit-playbook__title">{{ t("alchemy.inherit.playbookTitle") }}</div>
    <div class="inherit-playbook__steps">
      <span>{{ t("alchemy.inherit.playbookKeep") }}</span>
      <span>{{ t("alchemy.inherit.playbookSuppress") }}</span>
      <span>{{ t("alchemy.inherit.playbookIgnore") }}</span>
      <span>{{ t("alchemy.inherit.playbookRisk") }}</span>
    </div>
  </section>
</template>

<script setup lang="ts">
import { useI18n } from "vue-i18n"

const { t } = useI18n()
</script>

<style scoped>
.inherit-playbook {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 8px 12px;
  padding: 9px 10px;
  border: 1px solid color-mix(in srgb, var(--el-color-primary, #409eff) 24%, var(--app-border, #dde4ea));
  border-radius: 8px;
  background: color-mix(in srgb, var(--el-color-primary, #409eff) 5%, var(--app-surface, #fff));
}

.inherit-playbook__title {
  flex: 0 0 auto;
  color: var(--app-text, #1f2328);
  font-size: 12px;
  font-weight: 900;
}

.inherit-playbook__steps {
  min-width: 0;
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  color: var(--app-muted, #667085);
  font-size: 11px;
  line-height: 1.35;
}

.inherit-playbook__steps span {
  padding: 3px 7px;
  border: 1px solid color-mix(in srgb, var(--app-border, #dde4ea) 78%, transparent);
  border-radius: 999px;
  background: var(--app-surface, #fff);
}
</style>
