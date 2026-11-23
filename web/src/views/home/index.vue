<template>
  <div class="home-landing">
    <header class="home-topbar">
      <RouterLink class="home-brand" to="/" aria-label="Moyu Idle">
        <span>{{ t("home.title") }}</span>
      </RouterLink>

      <div class="home-topbar__actions">
        <el-button class="home-topbar__link" text @click="goInstructions">
          {{ t("home.instructions") }}
        </el-button>
        <el-button class="home-topbar__link" text @click="goCommunity">
          {{ t("home.community.nav") }}
        </el-button>
        <el-button class="home-topbar__link" text @click="goGame">
          {{ t("home.enterGame") }}
        </el-button>
        <el-dropdown trigger="click" @command="changeLocale">
          <el-button class="home-language-button" circle aria-label="Change language">
            <el-icon><Icon icon="material-symbols:translate" /></el-icon>
          </el-button>
          <template #dropdown>
            <el-dropdown-menu>
              <el-dropdown-item
                v-for="option in languageOptions"
                :key="option.value"
                :command="option.value"
                :disabled="option.value === currentLocale"
              >
                {{ option.label }}
              </el-dropdown-item>
            </el-dropdown-menu>
          </template>
        </el-dropdown>
      </div>
    </header>

    <main>
      <section class="home-hero" aria-labelledby="home-hero-title">
        <div class="home-hero__content">
          <p class="home-eyebrow">{{ t("home.heroTag") }}</p>
          <h1 id="home-hero-title">
            <span>{{ t("home.title") }}</span>
            <span class="home-hero__title-status">{{ t("home.titleStatus") }}</span>
          </h1>
          <p class="home-hero__subtitle">{{ t("home.subtitle") }}</p>
          <p class="home-hero__intro">{{ t("home.intro") }}</p>

          <div class="home-hero__actions">
            <el-button class="home-cta-button home-action-button" type="primary" size="large" @click="goAuth">
              <el-icon><Icon icon="material-symbols:play-arrow-rounded" /></el-icon>
              {{ t("home.start") }}
            </el-button>
            <el-button class="home-secondary-button home-action-button" size="large" @click="goInstructions">
              <el-icon><Icon icon="material-symbols:menu-book-outline-rounded" /></el-icon>
              {{ t("home.instructions") }}
            </el-button>
            <el-button class="home-secondary-button home-action-button" size="large" @click="goCommunity">
              <el-icon><Icon icon="material-symbols:forum-rounded" /></el-icon>
              {{ t("home.community.nav") }}
            </el-button>
          </div>

        </div>
      </section>

      <section id="features" class="home-section home-section--features" aria-labelledby="home-features-title">
        <div class="home-section__heading">
          <p class="home-eyebrow">{{ t("home.featureEyebrow") }}</p>
          <h2 id="home-features-title">{{ t("home.featureSectionTitle") }}</h2>
          <p>{{ t("home.featureSectionDesc") }}</p>
        </div>

        <div class="home-feature-grid">
          <article
            v-for="feature in features"
            :key="feature.title"
            class="home-feature-card"
            :class="`home-feature-card--${feature.accent}`"
          >
            <div class="home-feature-card__copy">
              <h3>{{ feature.title }}</h3>
              <p>{{ feature.desc }}</p>
            </div>
            <figure class="home-feature-card__image">
              <img :src="feature.image" alt="" />
            </figure>
          </article>
        </div>
      </section>

      <section class="home-section home-section--loop" aria-labelledby="home-loop-title">
        <div class="home-section__heading">
          <p class="home-eyebrow">{{ t("home.loopEyebrow") }}</p>
          <h2 id="home-loop-title">{{ t("home.loopTitle") }}</h2>
        </div>

        <div class="home-loop-steps">
          <article v-for="(step, index) in loopSteps" :key="step.title">
            <span>{{ String(index + 1).padStart(2, "0") }}</span>
            <h3>{{ step.title }}</h3>
            <p>{{ step.desc }}</p>
          </article>
        </div>
      </section>

      <section id="community" class="home-section home-section--community" aria-labelledby="home-community-title">
        <div class="home-section__heading">
          <p class="home-eyebrow">{{ t("home.community.eyebrow") }}</p>
          <h2 id="home-community-title">{{ t("home.community.title") }}</h2>
          <p>{{ t("home.community.desc") }}</p>
        </div>

        <div class="home-community-grid">
          <article v-for="source in communitySources" :key="source.label" class="home-community-card">
            <span class="home-community-card__icon" :class="`home-community-card__icon--${source.kind}`">
              <Icon :icon="source.icon" />
            </span>
            <div>
              <h3>{{ source.label }}</h3>
              <p>{{ source.value }}</p>
            </div>
            <a
              v-if="source.href"
              class="home-community-card__action"
              :href="source.href"
              target="_blank"
              rel="noopener noreferrer"
            >
              {{ t("home.community.open") }}
            </a>
            <button v-else class="home-community-card__action" type="button" @click="copyQQGroup">
              {{ t("home.community.copy") }}
            </button>
          </article>
        </div>
      </section>

      <section class="home-notice">
        <div>
          <p class="home-eyebrow">{{ t("home.noticeEyebrow") }}</p>
          <h2>{{ t("home.noticeTitle") }}</h2>
          <p>{{ t("home.noticeDesc") }}</p>
        </div>
        <div class="home-notice__actions">
          <el-button type="primary" size="large" @click="goAuth">{{ t("home.start") }}</el-button>
          <el-button size="large" @click="goGame">{{ t("home.enterGame") }}</el-button>
        </div>
      </section>
    </main>
  </div>
</template>

<script setup lang="ts">
import "@/style.css"
import { computed } from "vue"
import { RouterLink, useRouter } from "vue-router"
import { useI18n } from "vue-i18n"
import { Icon } from "@iconify/vue"
import { getLocaleNamespacesForPath, getLocaleOptions, normalizeSupportedLocale } from "@/i18n"
import { switchAppLocale } from "@services/locale-switch"

const router = useRouter()
const { t, locale } = useI18n()

const languageOptions = getLocaleOptions()

const currentLocale = computed(() => normalizeSupportedLocale(locale.value))

const features = computed(() => [
  {
    image: "/images/game/menu-actions/collecting.png",
    accent: "green",
    title: t("home.features.queue.title"),
    desc: t("home.features.queue.desc")
  },
  {
    image: "/images/game/menu-actions/character.png",
    accent: "blue",
    title: t("home.features.character.title"),
    desc: t("home.features.character.desc")
  },
  {
    image: "/images/game/menu-actions/cattery.png",
    accent: "rose",
    title: t("home.features.kitty.title"),
    desc: t("home.features.kitty.desc")
  },
  {
    image: "/images/game/menu-actions/quest.png",
    accent: "cyan",
    title: t("home.features.commission.title"),
    desc: t("home.features.commission.desc")
  },
  {
    image: "/images/game/menu-actions/battle.png",
    accent: "orange",
    title: t("home.features.combat.title"),
    desc: t("home.features.combat.desc")
  },
  {
    image: "/images/game/menu-actions/enhance.png",
    accent: "amber",
    title: t("home.features.equipment.title"),
    desc: t("home.features.equipment.desc")
  },
  {
    image: "/images/game/menu-actions/affix.png",
    accent: "violet",
    title: t("home.features.affix.title"),
    desc: t("home.features.affix.desc")
  },
  {
    image: "/images/game/menu-actions/alchemy.png",
    accent: "cyan",
    title: t("home.features.alchemy.title"),
    desc: t("home.features.alchemy.desc")
  },
  {
    image: "/images/game/menu-actions/market.png",
    accent: "green",
    title: t("home.features.market.title"),
    desc: t("home.features.market.desc")
  }
])

const loopSteps = computed(() => [
  { title: t("home.loop.plan.title"), desc: t("home.loop.plan.desc") },
  { title: t("home.loop.return.title"), desc: t("home.loop.return.desc") },
  { title: t("home.loop.refine.title"), desc: t("home.loop.refine.desc") },
  { title: t("home.loop.share.title"), desc: t("home.loop.share.desc") }
])

const qqGroup = "932589202"
const discordUrl = "https://discord.gg/qXEugp3vqX"

const communitySources = computed(() => [
  {
    kind: "qq",
    icon: "icon-park-outline:tencent-qq",
    label: t("home.community.qq"),
    value: qqGroup,
    href: ""
  },
  {
    kind: "discord",
    icon: "ic:baseline-discord",
    label: t("home.community.discord"),
    value: discordUrl,
    href: discordUrl
  }
])

async function changeLocale(value: string) {
  await switchAppLocale(value, {
    namespaces: getLocaleNamespacesForPath(router.currentRoute.value.path),
    persistLocal: true
  })
}

function goAuth() {
  router.push("/auth")
}

function goGame() {
  router.push("/game")
}

function goInstructions() {
  document.getElementById("features")?.scrollIntoView({ behavior: "smooth", block: "start" })
}

function goCommunity() {
  document.getElementById("community")?.scrollIntoView({ behavior: "smooth", block: "start" })
}

async function copyQQGroup() {
  await navigator.clipboard?.writeText(qqGroup)
}
</script>

<style scoped>
.home-landing {
  min-height: 100vh;
  background:
    linear-gradient(180deg, #f5f0e5 0%, #f7f7ef 42%, #eef5f3 100%),
    #f7f7ef;
  color: #152019;
}

.home-topbar {
  position: sticky;
  top: 0;
  z-index: 20;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  padding: 14px clamp(16px, 4vw, 48px);
  border-bottom: 1px solid rgba(21, 32, 25, 0.1);
  background: rgba(255, 255, 255, 0.84);
  backdrop-filter: blur(18px);
}

.home-brand {
  display: inline-flex;
  align-items: center;
  gap: 10px;
  color: #152019;
  font-weight: 900;
  text-decoration: none;
}

.home-topbar__actions {
  display: flex;
  align-items: center;
  gap: 8px;
}

.home-topbar__link {
  color: #334155;
  font-weight: 700;
}

.home-language-button {
  border-radius: 8px;
}

.home-hero {
  position: relative;
  display: grid;
  min-height: min(720px, calc(100vh - 64px));
  overflow: hidden;
  padding: clamp(44px, 8vw, 96px) clamp(18px, 6vw, 72px);
  isolation: isolate;
}

.home-hero::before {
  position: absolute;
  inset: 0;
  z-index: -2;
  content: "";
  background:
    linear-gradient(90deg, rgba(20, 34, 27, 0.8), rgba(20, 34, 27, 0.52) 40%, rgba(20, 34, 27, 0.08) 72%),
    url("/images/landing/hero-cats-workshop.png");
  background-position: center;
  background-size: cover;
}

.home-hero::after {
  position: absolute;
  inset: 0;
  z-index: -1;
  content: "";
  background:
    linear-gradient(180deg, rgba(10, 15, 12, 0.04), rgba(10, 15, 12, 0.22)),
    linear-gradient(90deg, rgba(10, 15, 12, 0.36), transparent 56%);
}

.home-hero__content {
  position: relative;
  z-index: 2;
  align-self: center;
  max-width: 760px;
  color: #fff;
  text-shadow: 0 2px 20px rgba(0, 0, 0, 0.24);
}

.home-eyebrow {
  margin: 0 0 12px;
  color: #f8c466;
  font-size: 0.78rem;
  font-weight: 900;
  letter-spacing: 0;
  text-transform: uppercase;
}

.home-hero h1,
.home-section__heading h2,
.home-notice h2 {
  margin: 0;
  font-weight: 950;
  line-height: 1.04;
  letter-spacing: 0;
}

.home-hero h1 {
  display: flex;
  flex-wrap: wrap;
  align-items: baseline;
  gap: 0.18em;
  max-width: 700px;
  font-size: clamp(3.6rem, 9vw, 7.2rem);
}

.home-hero__title-status {
  font-size: clamp(1.05rem, 2.2vw, 2rem);
  font-weight: 850;
  line-height: 1.2;
  color: rgba(255, 255, 255, 0.84);
}

.home-hero__subtitle {
  margin: 18px 0 0;
  color: rgba(255, 255, 255, 0.92);
  font-size: clamp(1.25rem, 2.2vw, 1.75rem);
  font-weight: 850;
  line-height: 1.38;
}

.home-hero__intro {
  max-width: 620px;
  margin: 14px 0 0;
  color: rgba(255, 255, 255, 0.88);
  font-size: clamp(0.98rem, 1.35vw, 1.14rem);
  line-height: 1.7;
}

.home-hero__actions {
  display: flex;
  flex-wrap: wrap;
  gap: 12px;
  margin-top: 28px;
}

.home-cta-button,
.home-secondary-button {
  min-width: 150px;
  border-radius: 8px;
  font-weight: 800;
}

.home-action-button :deep(> span) {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
}

.home-action-button :deep(.el-icon) {
  margin: 0;
}

.home-secondary-button {
  border-color: rgba(255, 255, 255, 0.42);
  background: rgba(255, 255, 255, 0.14);
  color: #fff;
}

.home-section,
.home-notice {
  width: min(1180px, calc(100% - 36px));
  margin-inline: auto;
}

.home-section {
  padding: clamp(54px, 8vw, 92px) 0 0;
}

.home-section__heading {
  max-width: 720px;
}

.home-section__heading h2,
.home-notice h2 {
  color: #142019;
  font-size: clamp(2rem, 3.8vw, 3.4rem);
}

.home-section__heading p:not(.home-eyebrow),
.home-notice p:not(.home-eyebrow) {
  margin: 14px 0 0;
  color: #526070;
  font-size: 1.02rem;
  line-height: 1.75;
}

.home-feature-grid {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 14px;
  margin-top: 28px;
}

.home-feature-card,
.home-loop-steps article,
.home-community-card {
  border: 1px solid rgba(21, 32, 25, 0.1);
  border-radius: 8px;
  background: #fff;
  box-shadow: 0 16px 40px rgba(15, 23, 42, 0.06);
}

.home-feature-card {
  position: relative;
  display: grid;
  grid-template-columns: minmax(0, 1fr);
  align-items: center;
  gap: 16px;
  min-height: 168px;
  overflow: hidden;
  padding: 18px;
}

.home-feature-card::before {
  position: absolute;
  inset: 0;
  content: "";
  opacity: 0.92;
}

.home-feature-card--green::before {
  background: linear-gradient(135deg, #ffffff 0%, #edf7ef 58%, #cfe6d7 100%);
}

.home-feature-card--rose::before {
  background: linear-gradient(135deg, #ffffff 0%, #f8eef1 58%, #ead7dd 100%);
}

.home-feature-card--blue::before {
  background: linear-gradient(135deg, #ffffff 0%, #eef5fb 58%, #d6e5f3 100%);
}

.home-feature-card--amber::before {
  background: linear-gradient(135deg, #ffffff 0%, #fbf2e3 58%, #edd7b6 100%);
}

.home-feature-card--orange::before {
  background: linear-gradient(135deg, #ffffff 0%, #fff0e9 58%, #f1c7b5 100%);
}

.home-feature-card--violet::before {
  background: linear-gradient(135deg, #ffffff 0%, #f2eff9 58%, #ddd8ee 100%);
}

.home-feature-card--cyan::before {
  background: linear-gradient(135deg, #ffffff 0%, #edf6f4 58%, #cfe3de 100%);
}

.home-feature-card__copy {
  position: relative;
  z-index: 2;
  max-width: 56%;
}

.home-feature-card h3,
.home-loop-steps h3 {
  margin: 0;
  color: #172019;
  font-size: 1.12rem;
  font-weight: 900;
}

.home-feature-card p,
.home-loop-steps p {
  margin: 10px 0 0;
  color: #526070;
  line-height: 1.68;
}

.home-feature-card__image {
  position: absolute;
  inset: 12px 20px 12px auto;
  z-index: 1;
  width: 34%;
  margin: 0;
  overflow: visible;
  border-radius: 8px;
  background: transparent;
  pointer-events: none;
}

.home-feature-card__image img {
  display: block;
  width: 100%;
  height: 100%;
  object-fit: contain;
  opacity: 0.96;
  filter: drop-shadow(0 10px 16px rgba(15, 23, 42, 0.10));
}

.home-loop-steps {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 14px;
  margin-top: 28px;
}

.home-loop-steps article {
  padding: 22px;
}

.home-loop-steps span {
  color: #bd6a16;
  font-size: 0.88rem;
  font-weight: 950;
}

.home-loop-steps h3 {
  margin-top: 12px;
}

.home-community-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 14px;
  margin-top: 28px;
}

.home-community-card {
  display: grid;
  grid-template-columns: auto minmax(0, 1fr) auto;
  align-items: center;
  gap: 16px;
  padding: 22px;
}

.home-community-card__icon {
  display: grid;
  width: 48px;
  height: 48px;
  place-items: center;
  border-radius: 8px;
  color: #fff;
  font-size: 1.5rem;
}

.home-community-card__icon--qq {
  background: #168447;
}

.home-community-card__icon--discord {
  background: #5865f2;
}

.home-community-card h3 {
  margin: 0;
  color: #172019;
  font-size: 1.08rem;
  font-weight: 900;
}

.home-community-card p {
  margin: 6px 0 0;
  color: #526070;
  overflow-wrap: anywhere;
}

.home-community-card__action {
  border: 1px solid rgba(21, 32, 25, 0.14);
  border-radius: 8px;
  padding: 9px 12px;
  background: #f8fafc;
  color: #172019;
  cursor: pointer;
  font: inherit;
  font-weight: 850;
  text-decoration: none;
}

.home-notice {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 24px;
  margin-top: clamp(54px, 8vw, 92px);
  margin-bottom: 56px;
  border: 1px solid rgba(21, 32, 25, 0.1);
  border-radius: 8px;
  padding: clamp(22px, 4vw, 36px);
  background: #fff;
  box-shadow: 0 18px 46px rgba(15, 23, 42, 0.08);
}

.home-notice > div:first-child {
  max-width: 720px;
}

.home-notice__actions {
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
  justify-content: flex-end;
}

.home-notice__actions .el-button {
  border-radius: 8px;
  font-weight: 800;
}

@media (max-width: 960px) {
  .home-hero {
    min-height: 720px;
    align-items: end;
  }

  .home-hero__content {
    align-self: end;
  }

  .home-feature-grid,
  .home-loop-steps,
  .home-community-grid {
    grid-template-columns: repeat(2, 1fr);
  }

  .home-notice {
    align-items: flex-start;
    flex-direction: column;
  }

  .home-notice__actions {
    justify-content: flex-start;
  }
}

@media (max-width: 640px) {
  .home-topbar {
    position: static;
  }

  .home-topbar__link {
    display: none;
  }

  .home-hero {
    min-height: 760px;
    padding-inline: 18px;
  }

  .home-hero h1 {
    font-size: 4rem;
  }

  .home-hero__actions,
  .home-notice__actions {
    flex-direction: column;
    align-items: stretch;
  }

  .home-hero__actions .el-button,
  .home-notice__actions .el-button {
    width: 100%;
    margin-left: 0;
  }

  .home-feature-grid,
  .home-loop-steps,
  .home-community-grid {
    grid-template-columns: 1fr;
  }

  .home-feature-card {
    grid-template-columns: 1fr;
  }

  .home-feature-card__copy {
    max-width: 100%;
  }

  .home-feature-card__image {
    opacity: 0.18;
    width: 100%;
    inset: 0;
  }

  .home-community-card {
    grid-template-columns: auto minmax(0, 1fr);
  }

  .home-community-card__action {
    grid-column: 1 / -1;
    text-align: center;
  }
}
</style>
