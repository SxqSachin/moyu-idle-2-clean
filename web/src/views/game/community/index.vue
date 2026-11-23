<template>
  <div class="page-shell">
    <GamePageFrame
      title-key="game.menu.community"
      icon="material-symbols:forum-rounded"
      :visual-src="communityVisual.src"
      :visual-tone="communityVisual.tone"
      visual-key="community"
    >
      <div class="community-page">
        <header class="community-page__intro">
          <p>{{ t("game.communityIntro") }}</p>
        </header>

        <div class="community-source-grid">
          <article v-for="source in sources" :key="source.label" class="community-source-card">
            <span class="community-source-card__mark" :class="`community-source-card__mark--${source.kind}`">
              <Icon :icon="source.icon" />
            </span>
            <div class="community-source-card__body">
              <h2>{{ source.label }}</h2>
              <p>{{ source.value }}</p>
            </div>
            <a
              v-if="source.href"
              class="community-source-card__action"
              :href="source.href"
              target="_blank"
              rel="noopener noreferrer"
            >
              {{ t("game.communityOpen") }}
            </a>
            <el-button v-else class="community-source-card__button" @click="copyQQGroup">
              {{ t("game.communityCopy") }}
            </el-button>
          </article>
        </div>
      </div>
    </GamePageFrame>
  </div>
</template>

<script setup lang="ts">
import { computed } from "vue"
import { ElMessage } from "element-plus"
import { useI18n } from "vue-i18n"
import { Icon } from "@iconify/vue"
import GamePageFrame from "@components/game/common/GamePageFrame.vue"
import { gamePageVisuals } from "@services/game-page-visuals"

const { t } = useI18n()
const communityVisual = gamePageVisuals.community

const qqGroup = "932589202"
const discordUrl = "https://discord.gg/qXEugp3vqX"

const sources = computed(() => [
  {
    kind: "qq",
    icon: "icon-park-outline:tencent-qq",
    label: t("game.communityQQ"),
    value: qqGroup,
    href: ""
  },
  {
    kind: "discord",
    icon: "ic:baseline-discord",
    label: t("game.communityDiscord"),
    value: discordUrl,
    href: discordUrl
  }
])

async function copyQQGroup() {
  await navigator.clipboard?.writeText(qqGroup)
  ElMessage.success(t("game.communityCopied"))
}
</script>

<style scoped>
.community-page {
  display: grid;
  gap: 16px;
}

.community-page__intro {
  max-width: 720px;
  color: var(--app-muted);
  font-size: 0.95rem;
  line-height: 1.7;
}

.community-source-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 12px;
}

.community-source-card {
  display: grid;
  grid-template-columns: auto minmax(0, 1fr) auto;
  align-items: center;
  gap: 14px;
  border: 1px solid var(--app-border);
  border-radius: 8px;
  background: var(--app-surface);
  padding: 16px;
}

.community-source-card__mark {
  display: grid;
  width: 44px;
  height: 44px;
  place-items: center;
  border-radius: 8px;
  color: #fff;
  font-weight: 900;
}

.community-source-card__mark--qq {
  background: #168447;
}

.community-source-card__mark--discord {
  background: #5865f2;
}

.community-source-card__body {
  min-width: 0;
}

.community-source-card__body h2 {
  margin: 0;
  color: var(--app-text);
  font-size: 1rem;
  font-weight: 900;
}

.community-source-card__body p {
  margin: 6px 0 0;
  color: var(--app-muted);
  overflow-wrap: anywhere;
}

.community-source-card__action,
.community-source-card__button {
  border-radius: 8px;
  font-weight: 800;
}

.community-source-card__action {
  border: 1px solid var(--app-border);
  padding: 8px 12px;
  color: var(--app-text);
  text-decoration: none;
}

@media (max-width: 767px) {
  .community-source-grid {
    grid-template-columns: 1fr;
  }

  .community-source-card {
    grid-template-columns: auto minmax(0, 1fr);
  }

  .community-source-card__action,
  .community-source-card__button {
    grid-column: 1 / -1;
    justify-self: stretch;
    text-align: center;
  }
}
</style>
