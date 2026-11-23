<template>
  <div
    class="game-page-header flex flex-wrap items-center justify-between gap-2"
    :class="{ 'game-page-header--visual': visualSrc && !store.playerSimpleMode }"
    :data-visual-key="visualKey || undefined"
  >
    <RichModeOnly>
      <div v-if="visualSrc" class="game-page-header__visual" :class="`game-page-header__visual--${visualTone}`" aria-hidden="true">
        <Transition name="game-page-header-visual-image" appear>
          <img
            :key="`${visualTone}:${visualSrc}`"
            class="game-page-header__visual-image"
            :src="visualSrc"
            :alt="visualAlt"
            loading="lazy"
            decoding="async"
          />
        </Transition>
      </div>
    </RichModeOnly>
    <div class="game-page-header__title flex min-w-0 items-center gap-2">
      <Icon v-if="icon" :icon="icon" class="shrink-0 text-xl text-[var(--app-muted)]" />
      <div class="min-w-0 truncate font-bold">{{ resolvedTitle }}</div>
    </div>
    <div class="game-page-header__actions flex flex-wrap items-center justify-end gap-2">
      <slot name="actions" />
      <el-button v-if="tutorialId" class="pl-2!" size="small" type="success" @click="startTutorial">
        <Icon icon="material-symbols:support-agent" class="text-xl mr-1"/>
        {{ t("tutorial.open") }}
      </el-button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from "vue"
import { useI18n } from "vue-i18n"
import { Icon } from "@iconify/vue"
import { useTutorialStore } from "@/tutorial"
import { useGameStore } from "@stores/game"
import RichModeOnly from "@components/game/common/RichModeOnly.vue"

const props = withDefaults(defineProps<{
  title?: string
  titleKey?: string
  icon?: string
  tutorialId?: string
  visualSrc?: string
  visualAlt?: string
  visualTone?: "green" | "blue" | "amber" | "violet" | "rose" | "cyan" | "flame"
  visualKey?: string
}>(), {
  title: "",
  titleKey: "",
  icon: "",
  tutorialId: "",
  visualSrc: "",
  visualAlt: "",
  visualTone: "green",
  visualKey: ""
})

const { t } = useI18n()
const store = useGameStore()
const tutorialStore = useTutorialStore()

const resolvedTitle = computed(() => {
  if (props.title) return props.title
  if (props.titleKey) return t(props.titleKey)
  return ""
})

function startTutorial() {
  if (!props.tutorialId) return
  void tutorialStore.start(props.tutorialId, { ignorePrerequisites: true })
}
</script>

<style scoped>
.game-page-header {
  position: relative;
  overflow: visible;
}

.game-page-header__title,
.game-page-header__actions {
  position: relative;
  z-index: 2;
}

.game-page-header__title {
  max-width: 100%;
  isolation: isolate;
}

.game-page-header__visual {
  position: absolute;
  inset: calc(-1 * var(--el-card-padding));
  z-index: 1;
  pointer-events: none;
}

.game-page-header__visual::before {
  position: absolute;
  inset: 0;
  content: "";
  animation: game-page-header-visual-wash-in 0.36s cubic-bezier(0.16, 1, 0.3, 1) both;
}

.game-page-header__visual--green::before {
  background: linear-gradient(90deg, rgba(22, 132, 71, 0.22) 0%, rgba(22, 132, 71, 0.12) 52%, rgba(22, 132, 71, 0.04) 100%);
}

.game-page-header__visual--blue::before {
  background: linear-gradient(90deg, rgba(37, 99, 235, 0.20) 0%, rgba(37, 99, 235, 0.10) 52%, rgba(37, 99, 235, 0.035) 100%);
}

.game-page-header__visual--amber::before {
  background: linear-gradient(90deg, rgba(217, 119, 6, 0.22) 0%, rgba(217, 119, 6, 0.11) 52%, rgba(217, 119, 6, 0.035) 100%);
}

.game-page-header__visual--violet::before {
  background: linear-gradient(90deg, rgba(109, 40, 217, 0.19) 0%, rgba(109, 40, 217, 0.10) 52%, rgba(109, 40, 217, 0.035) 100%);
}

.game-page-header__visual--rose::before {
  background: linear-gradient(90deg, rgba(225, 29, 72, 0.19) 0%, rgba(225, 29, 72, 0.10) 52%, rgba(225, 29, 72, 0.035) 100%);
}

.game-page-header__visual--cyan::before {
  background: linear-gradient(90deg, rgba(8, 145, 178, 0.20) 0%, rgba(8, 145, 178, 0.10) 52%, rgba(8, 145, 178, 0.035) 100%);
}

.game-page-header__visual--flame::before {
  background: linear-gradient(90deg, rgba(234, 88, 12, 0.22) 0%, rgba(234, 88, 12, 0.11) 52%, rgba(234, 88, 12, 0.04) 100%);
}

.game-page-header__visual-image {
  position: absolute;
  left: clamp(122px, 8vw, 300px);
  bottom: -26px;
  width: 96px;
  /* width: clamp(128px, 12vw, 178px); */
  max-height: 132px;
  object-fit: contain;
  object-position: left bottom;
  opacity: 0.68;
  transform: scaleX(-1);
  transform-origin: center;
  filter: drop-shadow(0 12px 14px rgba(15, 23, 42, 0.12));
}

.game-page-header-visual-image-enter-active,
.game-page-header-visual-image-leave-active {
  transition:
    opacity 0.32s cubic-bezier(0.16, 1, 0.3, 1),
    transform 0.32s cubic-bezier(0.16, 1, 0.3, 1),
    filter 0.32s cubic-bezier(0.16, 1, 0.3, 1);
}

.game-page-header-visual-image-enter-active {
  z-index: 1;
}

.game-page-header-visual-image-leave-active {
  z-index: 2;
}

.game-page-header-visual-image-enter-from,
.game-page-header-visual-image-leave-to {
  opacity: 0;
  transform: translate3d(10px, 6px, 0) scaleX(-1) scale(0.96);
  filter: drop-shadow(0 8px 10px rgba(15, 23, 42, 0));
}

.game-page-header-visual-image-leave-active {
  position: absolute;
}

@keyframes game-page-header-visual-wash-in {
  from {
    opacity: 0;
  }

  to {
    opacity: 1;
  }
}

@media (prefers-reduced-motion: reduce) {
  .game-page-header__visual::before {
    animation: none;
  }

  .game-page-header-visual-image-enter-active,
  .game-page-header-visual-image-leave-active {
    transition: none;
  }
}

.game-page-header[data-visual-key="homepage"] .game-page-header__visual-image {
  bottom: -10px;
}

.game-page-header[data-visual-key="market"] .game-page-header__visual-image {
  bottom: -10px;
}

@media (max-width: 767px) {
  .game-page-header__visual-image {
    left: 80px;
    bottom: -22px;
    width: 90px;
    max-height: 108px;
    opacity: 0.72;
    mix-blend-mode: multiply;
  }

  .game-page-header[data-visual-key="homepage"] .game-page-header__visual-image {
    bottom: -26px;
  }

  .game-page-header[data-visual-key="market"] .game-page-header__visual-image {
    bottom: -22px;
  }
}
</style>
