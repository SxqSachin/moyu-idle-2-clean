<template>
  <div class="combat-help" data-tutorial="combat-tab-help">
    <el-card class="app-card app-card--no-border combat-help__card" shadow="never">
      <template #header>
        <div class="combat-help__header">
          <div>
            <h2 class="combat-help__title">{{ t("game.battleHelpTitle") }}</h2>
            <p class="combat-help__subtitle">{{ t("game.battleHelpSubtitle") }}</p>
          </div>
        </div>
      </template>

      <div class="combat-help__grid">
        <section
          v-for="section in sections"
          :key="section.key"
          class="combat-help-section"
        >
          <span class="combat-help-section__accent" aria-hidden="true" />
          <div class="combat-help-section__header">
            <span class="combat-help-section__badge">{{ section.badge }}</span>
            <h3 class="combat-help-section__title">{{ section.title }}</h3>
          </div>
          <ul class="combat-help-section__list">
            <li v-for="line in section.lines" :key="`${section.key}-${line}`" class="combat-help-section__line">
              <span class="combat-help-section__dot" aria-hidden="true" />
              <span>{{ line }}</span>
            </li>
          </ul>
        </section>
      </div>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { computed } from "vue"
import { useI18n } from "vue-i18n"

const { t } = useI18n()

type HelpSection = {
  key: string
  badge: string
  title: string
  lines: string[]
}

const sections = computed<HelpSection[]>(() => [
  {
    key: "flow",
    badge: "01",
    title: t("game.battleHelpFlowTitle"),
    lines: [
      t("game.battleHelpFlowLine1"),
      t("game.battleHelpFlowLine2"),
      t("game.battleHelpFlowLine3")
    ]
  },
  {
    key: "fog",
    badge: "02",
    title: t("game.battleHelpFogTitle"),
    lines: [
      t("game.battleHelpFogLine1"),
      t("game.battleHelpFogLine2"),
      t("game.battleHelpFogLine3"),
      t("game.battleHelpFogLine4")
    ]
  },
  {
    key: "position",
    badge: "03",
    title: t("game.battleHelpPositionTitle"),
    lines: [
      t("game.battleHelpPositionLine1"),
      t("game.battleHelpPositionLine2"),
      t("game.battleHelpPositionLine3")
    ]
  },
  {
    key: "turn",
    badge: "04",
    title: t("game.battleHelpTurnTitle"),
    lines: [
      t("game.battleHelpTurnLine1"),
      t("game.battleHelpTurnLine2"),
      t("game.battleHelpTurnLine3")
    ]
  },
  {
    key: "damage",
    badge: "05",
    title: t("game.battleHelpDamageTitle"),
    lines: [
      t("game.battleHelpDamageLine1"),
      t("game.battleHelpDamageLine2"),
      t("game.battleHelpDamageLine3"),
      t("game.battleHelpDamageLine4"),
      t("game.battleHelpDamageLine5")
    ]
  },
  {
    key: "skill",
    badge: "06",
    title: t("game.battleHelpSkillTitle"),
    lines: [
      t("game.battleHelpSkillLine1"),
      t("game.battleHelpSkillLine2"),
      t("game.battleHelpSkillLine3")
    ]
  },
  {
    key: "enemy",
    badge: "07",
    title: t("game.battleHelpEnemyTitle"),
    lines: [
      t("game.battleHelpEnemyLine1"),
      t("game.battleHelpEnemyLine2"),
      t("game.battleHelpEnemyLine3")
    ]
  },
  {
    key: "reward",
    badge: "08",
    title: t("game.battleHelpRewardTitle"),
    lines: [
      t("game.battleHelpRewardLine1"),
      t("game.battleHelpRewardLine2"),
      t("game.battleHelpRewardLine3")
    ]
  },
  {
    key: "kitty",
    badge: "09",
    title: t("game.battleHelpKittyTitle"),
    lines: [
      t("game.battleHelpKittyLine1"),
      t("game.battleHelpKittyLine2"),
      t("game.battleHelpKittyLine3"),
      t("game.battleHelpKittyLine4"),
      t("game.battleHelpKittyLine5")
    ]
  }
])
</script>

<style scoped>
.combat-help {
  color: var(--app-text);
}

.combat-help__card :deep(.el-card__header) {
  padding: 12px 0;
}

.combat-help__card :deep(.el-card__body) {
  padding: 12px 0 0;
}

.combat-help__header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
}

.combat-help__title {
  margin: 0;
  font-size: 15px;
  font-weight: 700;
  line-height: 1.35;
}

.combat-help__subtitle {
  margin: 4px 0 0;
  color: var(--app-muted);
  font-size: 12px;
  line-height: 1.6;
}

.combat-help__grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(260px, 1fr));
  gap: 10px;
}

.combat-help-section {
  position: relative;
  min-width: 0;
  overflow: hidden;
  border: 1px solid var(--app-border);
  border-radius: 8px;
  background: color-mix(in srgb, var(--app-surface-elevated) 58%, var(--app-surface));
  padding: 12px 12px 12px 14px;
}

.combat-help-section--featured {
  border-color: color-mix(in srgb, var(--el-color-primary) 38%, var(--app-border));
  background: color-mix(in srgb, var(--el-color-primary-light-9) 44%, var(--app-surface));
}

.combat-help-section__accent {
  position: absolute;
  inset: 0 auto 0 0;
  width: 3px;
  background: color-mix(in srgb, var(--app-primary) 54%, var(--app-border));
}

.combat-help-section--featured .combat-help-section__accent {
  background: var(--app-primary);
}

.combat-help-section__header {
  display: flex;
  min-width: 0;
  align-items: center;
  gap: 8px;
}

.combat-help-section__badge {
  display: inline-flex;
  flex: 0 0 auto;
  align-items: center;
  justify-content: center;
  min-width: 30px;
  height: 22px;
  border: 1px solid color-mix(in srgb, var(--app-border) 74%, transparent);
  border-radius: 999px;
  background: var(--app-surface);
  color: var(--app-muted);
  font-size: 11px;
  font-weight: 700;
  line-height: 1;
}

.combat-help-section--featured .combat-help-section__badge {
  border-color: color-mix(in srgb, var(--app-primary) 34%, var(--app-border));
  color: var(--app-primary);
}

.combat-help-section__title {
  min-width: 0;
  margin: 0;
  overflow-wrap: anywhere;
  color: var(--app-text);
  font-size: 13px;
  font-weight: 700;
  line-height: 1.45;
}

.combat-help-section__list {
  display: flex;
  flex-direction: column;
  gap: 6px;
  margin: 10px 0 0;
  padding: 0;
  list-style: none;
}

.combat-help-section__line {
  display: grid;
  grid-template-columns: 6px minmax(0, 1fr);
  gap: 8px;
  align-items: start;
  color: var(--app-muted);
  font-size: 12px;
  line-height: 1.65;
}

.combat-help-section__dot {
  width: 5px;
  height: 5px;
  margin-top: 8px;
  border-radius: 999px;
  background: color-mix(in srgb, var(--app-primary) 52%, var(--app-border));
}

@media (max-width: 640px) {
  .combat-help__grid {
    grid-template-columns: 1fr;
  }

  .combat-help-section {
    padding: 11px 10px 11px 13px;
  }
}
</style>
