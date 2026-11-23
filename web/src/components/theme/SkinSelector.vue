<template>
  <div class="skin-selector">
    <div class="skin-selector__grid">
      <button
        v-for="skin in SKINS"
        :key="skin"
        type="button"
        class="skin-card"
        :class="{ 'skin-card--active': currentSkin === skin }"
        @click="$emit('update:skin', skin)"
      >
        <div class="skin-card__swatch">
          <div
            class="skin-card__swatch-primary"
            :style="{ background: SKIN_SWATCHES[skin].primary }"
          />
          <div
            class="skin-card__swatch-accent"
            :style="{ background: SKIN_SWATCHES[skin].accent }"
          />
        </div>
        <div class="skin-card__name">{{ t(SKIN_I18N_KEY[skin]) }}</div>
        <div v-if="currentSkin === skin" class="skin-card__check" aria-hidden="true">
          ✓
        </div>
      </button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from "vue-i18n"
import { SKINS, SKIN_I18N_KEY, SKIN_SWATCHES, type ThemeSkin } from "@/themes/registry"
import { useSkin } from "@/composables/useSkin"

defineEmits<{
  "update:skin": [skin: ThemeSkin]
}>()

const { t } = useI18n()
const { useCurrentSkin } = useSkin()
const currentSkin = useCurrentSkin()
</script>

<style scoped>
.skin-selector {
  width: 100%;
}

.skin-selector__grid {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 12px;
}

@media (max-width: 480px) {
  .skin-selector__grid {
    grid-template-columns: 1fr;
  }
}

.skin-card {
  position: relative;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 8px;
  padding: 12px;
  border: 1px solid var(--app-border);
  border-radius: 8px;
  background: var(--app-surface);
  cursor: pointer;
  transition: all 0.2s ease;
  font-family: inherit;
  color: var(--app-text);
}

.skin-card:hover {
  border-color: var(--app-primary);
  background: var(--app-surface-elevated);
}

.skin-card--active {
  border-color: var(--app-primary);
  box-shadow: var(--game-accent-glow);
}

.skin-card__swatch {
  display: flex;
  gap: 4px;
  width: 100%;
  height: 48px;
  border-radius: 4px;
  overflow: hidden;
}

.skin-card__swatch-primary {
  flex: 2;
}

.skin-card__swatch-accent {
  flex: 1;
}

.skin-card__name {
  font-size: 14px;
  font-weight: 500;
}

.skin-card__check {
  position: absolute;
  top: 6px;
  right: 6px;
  width: 20px;
  height: 20px;
  border-radius: 50%;
  background: var(--app-primary);
  color: white;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 12px;
  font-weight: bold;
}
</style>
