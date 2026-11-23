<template>
  <div class="fishing-backdrop" :class="[`period-${environment.period}`, `weather-${environment.weather}`, { 'is-alpine': waterId === 'frostglass_lake' }]" :data-period="environment.period" :data-weather="environment.weather" aria-hidden="true">
    <img class="fishing-backdrop__landscape" :src="fishingSceneAsset(waterId)" alt="" draggable="false" />
    <div class="fishing-backdrop__tone" />
    <div class="fishing-backdrop__cloud-cover" />
    <template v-if="environment.celestial">
      <div class="fishing-backdrop__celestial" :class="`is-${environment.celestial}`" />
      <div v-if="environment.celestial === 'moon'" class="fishing-backdrop__stars">
        <i v-for="star in stars" :key="star.id" :style="star.style" />
      </div>
    </template>
    <div v-if="environment.weather === 'mist'" class="fishing-backdrop__mist"><i /><i /><i /></div>
    <div v-if="environment.precipitation" class="fishing-backdrop__precipitation" :class="`is-${environment.precipitation}`">
      <i v-for="particle in particles.slice(0, environment.weather === 'storm' ? 40 : 26)" :key="particle.id" :style="particle.style" />
    </div>
    <div v-if="environment.precipitation === 'rain'" class="fishing-backdrop__rain-rings">
      <i v-for="ring in rings" :key="ring.id" :style="ring.style" />
    </div>
  </div>
</template>

<script setup lang="ts">
import { fishingSceneAsset, type FishingSceneEnvironment } from "@services/fishing-scene"

defineProps<{ waterId?: string; environment: FishingSceneEnvironment }>()

const particles = Array.from({ length: 40 }, (_, id) => ({ id, style: {
  left: `${(id * 37 + 7) % 100}%`,
  top: `${(id * 23 + 11) % 100}%`,
  "--fall-delay": `${-(id * .37)}s`,
  "--snow-duration": `${6 + id % 5}s`
} }))
const rings = Array.from({ length: 12 }, (_, id) => ({ id, style: {
  left: `${8 + (id * 23) % 84}%`, top: `${40 + (id * 17) % 42}%`,
  animationDelay: `${-(id * .43)}s`
} }))
const stars = Array.from({ length: 18 }, (_, id) => ({ id, style: {
  left: `${5 + (id * 31) % 90}%`, top: `${4 + (id * 7) % 14}%`,
  opacity: .35 + (id % 4) * .15
} }))
</script>

<style scoped>
.fishing-backdrop { position: absolute; z-index: 0; inset: 0; overflow: hidden; pointer-events: none; background: #57949b; }
.fishing-backdrop__landscape { width: 100%; height: 100%; object-fit: cover; object-position: center 30%; user-select: none; transition: filter 1.4s ease; }
.fishing-backdrop__tone, .fishing-backdrop__cloud-cover, .fishing-backdrop__precipitation, .fishing-backdrop__stars, .fishing-backdrop__rain-rings { position: absolute; inset: 0; }
.fishing-backdrop__tone { transition: background 1.4s ease; }
.period-dawn .fishing-backdrop__landscape { filter: sepia(.2) saturate(.85) brightness(.94); }
.period-dawn .fishing-backdrop__tone { background: linear-gradient(180deg, rgb(243 182 148 / 38%), transparent 46%, rgb(234 183 143 / 9%)); mix-blend-mode: soft-light; }
.period-dusk .fishing-backdrop__landscape { filter: sepia(.35) saturate(.78) brightness(.76); }
.period-dusk .fishing-backdrop__tone { background: linear-gradient(180deg, rgb(225 134 126 / 66%), rgb(236 172 123 / 18%) 40%, rgb(73 80 107 / 16%)); mix-blend-mode: hard-light; }
.period-night .fishing-backdrop__landscape { filter: saturate(.55) brightness(.36); }
.period-night .fishing-backdrop__tone { background: rgb(31 61 103 / 34%); mix-blend-mode: color; }
.fishing-backdrop__cloud-cover { opacity: 0; background: linear-gradient(180deg, rgb(104 121 130 / 65%), rgb(80 110 115 / 16%) 45%, rgb(72 98 109 / 12%)); transition: opacity 1.4s ease; }
.weather-cloud .fishing-backdrop__cloud-cover, .weather-rain .fishing-backdrop__cloud-cover, .weather-snow .fishing-backdrop__cloud-cover { opacity: .8; }
.weather-storm .fishing-backdrop__cloud-cover { opacity: 1; background: linear-gradient(180deg, rgb(27 43 55 / 88%), rgb(39 66 73 / 50%) 50%, rgb(41 66 74 / 36%)); }
.fishing-backdrop__celestial { position: absolute; top: 13%; left: 52%; width: 28px; height: 28px; border-radius: 50%; }
.fishing-backdrop__celestial.is-sun { background: #fff1c4; box-shadow: 0 0 15px rgb(255 231 171 / 32%); }
.period-dawn .fishing-backdrop__celestial, .period-dusk .fishing-backdrop__celestial { top: 13%; background: #ffddb0; }
.is-alpine .fishing-backdrop__celestial { top: 10%; }
.fishing-backdrop__celestial.is-moon { background: #e5efe9; box-shadow: inset -7px -3px 0 #96b2ba, 0 0 12px rgb(218 238 243 / 12%); }
.fishing-backdrop__stars i { position: absolute; width: 2px; height: 2px; border-radius: 50%; background: #e7efee; }
.fishing-backdrop__mist { position: absolute; inset: 20% -15% 20%; }
.fishing-backdrop__mist i { position: absolute; width: 100%; height: 38%; background: linear-gradient(180deg, transparent, rgb(220 235 229 / 44%), transparent); animation: fishing-mist-drift 18s ease-in-out infinite alternate; }
.fishing-backdrop__mist i:nth-child(2) { top: 24%; opacity: .6; animation-delay: -8s; }
.fishing-backdrop__mist i:nth-child(3) { top: 58%; opacity: .3; animation-delay: -13s; }
.fishing-backdrop__precipitation i { position: absolute; animation-delay: var(--fall-delay); animation-iteration-count: infinite; animation-timing-function: linear; }
.is-rain i { width: 1px; height: 17px; background: rgb(219 236 241 / 48%); animation-name: fishing-rain-fall; animation-duration: .9s; }
.weather-storm .is-rain i { height: 24px; animation-duration: .55s; }
.is-snow i { width: 3px; height: 3px; border-radius: 50%; background: rgb(248 253 255 / 82%); animation-name: fishing-snow-fall; animation-duration: var(--snow-duration); }
.fishing-backdrop__rain-rings i { position: absolute; width: 14px; height: 4px; border: 1px solid rgb(226 240 240 / 35%); border-radius: 50%; animation: fishing-rain-ring 2.1s ease-out infinite; }
@keyframes fishing-rain-fall { from { transform: translate(22px, -100px) rotate(14deg); opacity: 0; } 12% { opacity: .65; } to { transform: translate(-28px, 220px) rotate(14deg); opacity: 0; } }
@keyframes fishing-snow-fall { from { transform: translate(0, -100px); opacity: 0; } 15%, 70% { opacity: .85; } to { transform: translate(34px, 280px); opacity: 0; } }
@keyframes fishing-rain-ring { from { transform: scale(.3); opacity: 0; } 25% { opacity: .65; } to { transform: scale(1.9); opacity: 0; } }
@keyframes fishing-mist-drift { from { transform: translateX(-5%); } to { transform: translateX(5%); } }
@media (prefers-reduced-motion: reduce) {
  .fishing-backdrop *, .fishing-backdrop *::before, .fishing-backdrop *::after { animation: none !important; transition: none !important; }
  .fishing-backdrop__precipitation i { opacity: .45; }
  .fishing-backdrop__rain-rings { display: none; }
}
</style>
