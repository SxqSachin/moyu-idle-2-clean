<template>
  <span
    class="fish-mutation-art"
    :class="classes"
    :style="artStyle"
    :data-fish-mutations="mutationIds.join(' ') || undefined"
  >
    <img :src="src" :alt="alt" draggable="false" />
    <span v-if="visualClasses.includes('fish-mutation--prismatic')" class="fish-mutation-art__prism" aria-hidden="true" />
    <span v-if="visualClasses.includes('fish-mutation--rainbow')" class="fish-mutation-art__rainbow" aria-hidden="true" />
    <span v-if="visualClasses.includes('fish-mutation--shimmering')" class="fish-mutation-art__glints" aria-hidden="true"><i /><i /><i /><i /><i /></span>
  </span>
</template>

<script setup lang="ts">
import { computed } from "vue"
import { fishMutationClassNames, fishMutationIds, type FishMutationSource } from "@services/fish-mutations"
import type { FishingMutationDefinition } from "@services/fishing"

const props = withDefaults(defineProps<{
  src: string
  alt?: string
  fish?: FishMutationSource | null
  mutations?: readonly FishingMutationDefinition[]
  width?: number | string
  dead?: boolean
  animated?: boolean
}>(), {
  alt: "",
  fish: null,
  mutations: () => [],
  width: 92,
  dead: false,
  animated: true
})

const mutationIds = computed(() => fishMutationIds(props.fish))
const visualClasses = computed(() => fishMutationClassNames(props.fish, props.mutations))
const classes = computed(() => [
  ...visualClasses.value,
  { "is-dead": props.dead, "is-static": !props.animated }
])
const artStyle = computed(() => ({
  width: typeof props.width === "number" ? `${props.width}px` : props.width,
  "--fish-mutation-mask": `url("${String(props.src).replaceAll('"', '\\"')}")`
}))
</script>

<style scoped>
.fish-mutation-art {
  position: relative;
  display: inline-grid;
  place-items: center;
  aspect-ratio: 1.35;
  isolation: isolate;
  flex: none;
}
.fish-mutation-art > img {
  position: relative;
  z-index: 1;
  display: block;
  width: 100%;
  height: 100%;
  object-fit: contain;
  user-select: none;
}
.fish-mutation-art__prism,
.fish-mutation-art__rainbow {
  position: absolute;
  z-index: 2;
  inset: 0;
  opacity: 0;
  pointer-events: none;
  background-repeat: repeat;
  -webkit-mask: var(--fish-mutation-mask) center / contain no-repeat;
  mask: var(--fish-mutation-mask) center / contain no-repeat;
}
.fish-mutation-art__prism {
  background: linear-gradient(112deg, rgb(103 221 255 / 34%) 0%, rgb(170 250 255 / 62%) 13%, rgb(255 255 238 / 94%) 25%, rgb(255 145 224 / 66%) 38%, rgb(255 230 128 / 72%) 51%, rgb(123 255 216 / 58%) 64%, rgb(151 169 255 / 66%) 77%, rgb(255 255 238 / 90%) 89%, rgb(103 221 255 / 34%) 100%);
  background-size: 200% 100%;
  mix-blend-mode: screen;
}
.fish-mutation-art__rainbow {
  background: linear-gradient(102deg, rgb(255 72 102 / 78%) 0%, rgb(255 196 64 / 82%) 16%, rgb(86 229 137 / 76%) 33%, rgb(66 198 236 / 80%) 50%, rgb(93 125 255 / 80%) 67%, rgb(215 92 235 / 78%) 84%, rgb(255 72 102 / 78%) 100%);
  background-size: 200% 100%;
  mix-blend-mode: color;
}
.fish-mutation-art__glints {
  position: absolute;
  z-index: 3;
  inset: 4% 1%;
  opacity: 0;
  pointer-events: none;
}
.fish-mutation-art__glints i {
  position: absolute;
  width: clamp(5px, 8%, 11px);
  aspect-ratio: 1;
  opacity: .78;
  border-radius: 50%;
  background: radial-gradient(circle, #fff 0 18%, #fffbd0 24%, rgb(151 236 255 / 82%) 42%, transparent 68%);
  filter: drop-shadow(0 0 3px #fff) drop-shadow(0 0 5px rgb(111 225 255 / 90%));
  will-change: transform;
}
.fish-mutation-art__glints i::before,
.fish-mutation-art__glints i::after {
  content: "";
  position: absolute;
  top: 50%;
  left: 50%;
  border-radius: 999px;
  background: rgb(255 255 241 / 96%);
  transform: translate(-50%, -50%);
}
.fish-mutation-art__glints i::before {
  width: 24%;
  height: 118%;
}
.fish-mutation-art__glints i::after {
  width: 118%;
  height: 24%;
}
.fish-mutation-art__glints i:nth-child(1) { top: 18%; left: 24%; }
.fish-mutation-art__glints i:nth-child(2) { top: 39%; right: 15%; animation-delay: -.42s; }
.fish-mutation-art__glints i:nth-child(3) { bottom: 15%; left: 47%; animation-delay: -.84s; }
.fish-mutation-art__glints i:nth-child(4) { top: 30%; left: 61%; width: clamp(4px, 6%, 8px); animation-delay: -1.26s; }
.fish-mutation-art__glints i:nth-child(5) { bottom: 27%; left: 32%; width: clamp(4px, 5%, 7px); animation-delay: -1.68s; }
.fish-mutation-art.fish-mutation--prismatic > img { filter: saturate(1.2) contrast(1.07) drop-shadow(0 0 5px rgb(112 230 255 / 68%)) drop-shadow(0 0 3px rgb(255 139 224 / 46%)); }
.fish-mutation-art.fish-mutation--prismatic .fish-mutation-art__prism { opacity: .82; animation: fish-prismatic-flow 3.6s linear infinite; }
.fish-mutation-art.fish-mutation--rainbow > img { filter: saturate(1.34) contrast(1.04) drop-shadow(0 0 5px rgb(255 151 193 / 58%)) drop-shadow(0 0 3px rgb(101 213 255 / 48%)); }
.fish-mutation-art.fish-mutation--rainbow .fish-mutation-art__rainbow { opacity: .76; animation: fish-rainbow-flow 4.2s linear infinite; }
.fish-mutation-art.fish-mutation--shimmering > img { filter: brightness(1.12) contrast(1.08) drop-shadow(0 0 7px rgb(255 246 169 / 72%)); }
.fish-mutation-art.fish-mutation--shimmering .fish-mutation-art__glints { opacity: 1; }
.fish-mutation-art.fish-mutation--shimmering .fish-mutation-art__glints i { animation: fish-shimmer-breathe 1.9s ease-in-out infinite; }
.fish-mutation-art.has-stacked-fish-mutations .fish-mutation-art__prism,
.fish-mutation-art.has-stacked-fish-mutations .fish-mutation-art__rainbow { opacity: .58; }
.fish-mutation-art.has-stacked-fish-mutations > img { filter: saturate(1.28) contrast(1.07) brightness(1.06) drop-shadow(0 0 7px rgb(181 239 255 / 68%)) drop-shadow(0 0 4px rgb(255 157 218 / 50%)); }
.fish-mutation-art.is-dead { opacity: .5; }
.fish-mutation-art.is-dead > img { filter: grayscale(.78); }
.fish-mutation-art.is-dead .fish-mutation-art__prism,
.fish-mutation-art.is-dead .fish-mutation-art__rainbow,
.fish-mutation-art.is-dead .fish-mutation-art__glints { display: none; }
.fish-mutation-art.is-static .fish-mutation-art__prism,
.fish-mutation-art.is-static .fish-mutation-art__rainbow,
.fish-mutation-art.is-static .fish-mutation-art__glints i { animation: none; will-change: auto; }
.fish-mutation-art.is-static .fish-mutation-art__prism { background-position: 42% 50%; }
.fish-mutation-art.is-static .fish-mutation-art__rainbow { background-position: 58% 50%; }
@keyframes fish-prismatic-flow { from { background-position: 0% 50%; } to { background-position: 200% 50%; } }
@keyframes fish-rainbow-flow { from { background-position: 200% 50%; } to { background-position: 0% 50%; } }
@keyframes fish-shimmer-breathe { 0%, 100% { transform: scale(.72) rotate(0deg); filter: brightness(.9) drop-shadow(0 0 3px #fff) drop-shadow(0 0 5px rgb(111 225 255 / 86%)); } 50% { transform: scale(1.24) rotate(45deg); filter: brightness(1.35) drop-shadow(0 0 4px #fff) drop-shadow(0 0 8px rgb(255 235 141 / 96%)); } }
@media (prefers-reduced-motion: reduce) {
  .fish-mutation-art__prism,
  .fish-mutation-art__rainbow,
  .fish-mutation-art__glints i { animation: none !important; }
  .fish-mutation-art.fish-mutation--prismatic .fish-mutation-art__prism { background-position: 42% 50%; }
  .fish-mutation-art.fish-mutation--rainbow .fish-mutation-art__rainbow { background-position: 58% 50%; }
}
</style>
