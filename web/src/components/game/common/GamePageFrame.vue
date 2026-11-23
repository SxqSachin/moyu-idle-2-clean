<template>
  <el-card
    v-bind="$attrs"
    class="app-card app-card--no-border game-page-frame"
    :style="frameStyle"
    shadow="never"
  >
    <template #header>
      <slot name="header">
        <GamePageHeader
          :title="title"
          :title-key="titleKey"
          :icon="icon"
          :tutorial-id="tutorialId"
          :visual-src="visualSrc"
          :visual-alt="visualAlt"
          :visual-tone="visualTone"
          :visual-key="visualKey"
        >
          <template #actions>
            <slot name="actions" />
          </template>
        </GamePageHeader>
      </slot>
    </template>
    <slot />
  </el-card>
</template>

<script setup lang="ts">
import { computed } from "vue"
import GamePageHeader from "@components/game/common/GamePageHeader.vue"
import type { GamePageVisualTone } from "@/services/game-page-visuals"

defineOptions({
  inheritAttrs: false
})

const props = withDefaults(defineProps<{
  title?: string
  titleKey?: string
  icon?: string
  tutorialId?: string
  bodyPadding?: string
  mobileBodyPadding?: string
  visualSrc?: string
  visualAlt?: string
  visualTone?: GamePageVisualTone
  visualKey?: string
}>(), {
  title: "",
  titleKey: "",
  icon: "",
  tutorialId: "",
  bodyPadding: "var(--el-card-padding)",
  mobileBodyPadding: "0.5rem 1rem",
  visualSrc: "",
  visualAlt: "",
  visualTone: "green",
  visualKey: ""
})

const frameStyle = computed(() => ({
  "--game-page-body-padding": props.bodyPadding,
  "--game-page-mobile-body-padding": props.mobileBodyPadding
}))
</script>
