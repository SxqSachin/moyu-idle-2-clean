<template>
  <section ref="sceneEl" class="pond-water" :class="{ 'is-fallback': fallback, 'is-paused': paused, 'is-tracking': trackedId, 'is-interacting': activeTool }" :aria-label="t('game.fishing.pondWaterLabel')" @click="closeInteractionMenu">
    <div ref="worldEl" class="pond-water__world">
      <img class="pond-water__garden" src="/assets/scenes/pond-garden-v1.webp" alt="" draggable="false" />
      <canvas ref="canvasEl" />
      <div v-if="activeFeedBurst || stoneBursts.length || stoneProjectile || skipPreviewPoints.length" class="pond-water__effect-layer" aria-hidden="true">
        <span v-if="activeFeedBurst" class="pond-feed-burst" :style="feedBurstStyle">
          <b class="pond-feed-burst__ripple pond-feed-burst__ripple--one" />
          <b class="pond-feed-burst__ripple pond-feed-burst__ripple--two" />
          <i v-for="index in 18" :key="index" :style="feedParticleStyle(index)" />
        </span>
        <span
          v-for="burst in stoneBursts"
          :key="burst.id"
          class="pond-stone-impact"
          :class="`is-${burst.kind}`"
          :style="pointStyle(burst)"
        >
          <b />
          <i v-for="index in burst.kind === 'throw' ? 7 : 4" :key="index" :style="splashParticleStyle(burst, index)" />
        </span>
        <span v-if="stoneProjectile" class="pond-stone-projectile" :class="`is-${stoneProjectile.kind}`" :style="pointStyle(stoneProjectile)"><i /></span>
        <span v-for="point in skipPreviewPoints" :key="point.id" class="pond-skip-preview" :style="pointStyle(point)" />
      </div>
      <div class="pond-water__fish-layer">
        <button
          v-for="sprite in sprites"
          :key="sprite.instanceId"
          :ref="(element) => setSpriteElement(sprite.instanceId, element)"
          type="button"
          class="pond-fish-sprite"
          :class="[sprite.mutationClasses, { 'is-dead': sprite.dead, 'is-reverse': sprite.reverse, 'is-tracked': trackedId === sprite.instanceId }]"
          :data-fish-mutations="sprite.mutationIds.join(' ') || undefined"
          :style="{
            '--fish-width': `${sprite.width}px`,
            '--fish-body-duration': `${sprite.bodyDuration}s`,
            '--fish-body-delay': `${sprite.bodyDelay}s`,
            '--fish-tail-duration': `${sprite.tailDuration}s`,
            '--fish-tail-glide-duration': `${sprite.tailGlideDuration}s`,
            '--fish-tail-kick-duration': `${sprite.tailKickDuration}s`,
            '--fish-tail-delay': `${sprite.tailDelay}s`,
            '--fish-mutation-mask': `url(&quot;${sprite.iconPath}&quot;)`
          }"
          :disabled="!canInspectFish && !canTrackFish"
          :tabindex="interactionLocked || (!canInspectFish && !canTrackFish) ? -1 : 0"
          :aria-label="canTrackFish ? t('game.fishing.pondTrackFishLabel', { name: sprite.name }) : sprite.name"
          @mouseenter="showTooltip(sprite.instanceId)"
          @mouseleave="hideTooltip(sprite.instanceId)"
          @focus="showTooltip(sprite.instanceId)"
          @blur="hideTooltip(sprite.instanceId)"
          @click="trackFish(sprite.instanceId)"
        >
          <span class="pond-fish-sprite__art" aria-hidden="true">
            <img class="pond-fish-sprite__body" :src="sprite.iconPath" alt="" draggable="false" />
            <img class="pond-fish-sprite__tail" :src="sprite.iconPath" alt="" draggable="false" />
            <span v-if="sprite.mutationClasses.includes('fish-mutation--prismatic')" class="pond-fish-sprite__prism" />
            <span v-if="sprite.mutationClasses.includes('fish-mutation--rainbow')" class="pond-fish-sprite__rainbow" />
            <span v-if="sprite.mutationClasses.includes('fish-mutation--shimmering')" class="pond-fish-sprite__glints"><i /><i /><i /><i /><i /></span>
          </span>
          <span v-if="sprite.predator" class="pond-fish-sprite__ecology" aria-hidden="true">
            <Icon icon="lucide:utensils" />
          </span>
          <i v-if="sprite.dead" />
        </button>
      </div>
    </div>

    <button
      v-if="activeTool"
      ref="interactionSurfaceEl"
      type="button"
      class="pond-water__interaction-surface"
      :class="`is-${activeTool}`"
      :disabled="interactionSurfaceBusy"
      :aria-label="interactionSurfaceLabel"
      @click="useInteractionAtPointer"
      @pointerdown="startSkipGesture"
      @pointermove="moveSkipGesture"
      @pointerup="finishSkipGesture"
      @pointercancel="cancelSkipGesture"
    />

    <div v-if="hasInteractionControls" class="pond-water__interaction-controls" @click.stop>
      <template v-if="scatterMode && canFeed">
        <span class="pond-water__interaction-selection">
          <Icon :icon="scatterBusy ? 'lucide:loader-circle' : 'lucide:wheat'" />
          <strong>{{ selectedFeedLabel }}</strong>
          <small>×{{ selectedFeedQuantity }}</small>
        </span>
        <el-tooltip :content="t('game.fishing.pondCancelScatter')">
          <button type="button" :aria-label="t('game.fishing.pondCancelScatter')" :disabled="scatterBusy" @click="emit('cancel-scatter')"><Icon icon="lucide:x" /></button>
        </el-tooltip>
      </template>
      <template v-else-if="localInteractionMode">
        <span class="pond-water__interaction-selection">
          <Icon :icon="localInteractionMode === 'throw' ? 'lucide:circle-dot' : 'lucide:waves'" />
          <strong>{{ t(localInteractionMode === "throw" ? "game.fishing.pondThrowStone" : "game.fishing.pondSkipStone") }}</strong>
        </span>
        <el-tooltip :content="t('game.fishing.pondCancelInteraction')">
          <button type="button" :aria-label="t('game.fishing.pondCancelInteraction')" @click="cancelLocalInteraction"><Icon icon="lucide:x" /></button>
        </el-tooltip>
      </template>
      <template v-else>
        <el-tooltip :content="t('game.fishing.pondInteract')" placement="left" :disabled="interactionMenuOpen">
          <button
            type="button"
            class="pond-water__interaction-action"
            :aria-label="t('game.fishing.pondInteract')"
            :disabled="effectBusy"
            aria-haspopup="menu"
            :aria-expanded="interactionMenuOpen"
            @click="toggleInteractionMenu"
          >
            <Icon icon="lucide:hand" />
          </button>
        </el-tooltip>
        <div v-show="interactionMenuOpen" class="pond-water__interaction-menu" role="menu" :aria-label="t('game.fishing.pondInteractMenuLabel')">
          <button v-if="canFeed" type="button" role="menuitem" :disabled="feedDisabled" @click="selectInteraction('feed')"><Icon icon="lucide:hand-platter" /><span>{{ t("game.fishing.pondScatterFeed") }}</span></button>
          <button v-if="canThrowStone" type="button" role="menuitem" @click="selectInteraction('throw')"><Icon icon="lucide:circle-dot" /><span>{{ t("game.fishing.pondThrowStone") }}</span></button>
          <button v-if="canSkipStone" type="button" role="menuitem" @click="selectInteraction('skip')"><Icon icon="lucide:waves" /><span>{{ t("game.fishing.pondSkipStone") }}</span></button>
        </div>
      </template>
    </div>

    <div v-if="hoveredFish" ref="tooltipEl" class="pond-water__tooltip" role="tooltip">
      <strong :class="itemCustomLabelClass(hoveredFish.customData)">{{ fishName(hoveredFish) }}</strong>
      <small v-if="customName(hoveredFish)">{{ speciesName(hoveredFish.fish.speciesId) }}</small>
      <RarityText :rarity-id="fishRarity(hoveredFish)" />
      <dl>
        <div><dt>{{ t("game.fishing.length") }}</dt><dd>{{ formatLength(hoveredFish.fish.lengthCm) }}<FishSizeIndicator metric="length" :value="hoveredFish.fish.lengthCm" :species="speciesById.get(hoveredFish.fish.speciesId)" /></dd></div>
        <div><dt>{{ t("game.fishing.weight") }}</dt><dd>{{ formatWeight(hoveredFish.fish.weightKg) }}<FishSizeIndicator metric="weight" :value="hoveredFish.fish.weightKg" :species="speciesById.get(hoveredFish.fish.speciesId)" /></dd></div>
        <div v-if="canInspectVitals"><dt>{{ t("game.fishing.health") }}</dt><dd>{{ healthPercent(hoveredFish) }}%</dd></div>
      </dl>
      <div v-if="hoveredFish.predation" class="pond-water__tooltip-ecology">
        <span class="is-predator"><Icon icon="lucide:utensils" />{{ predatorRule(hoveredFish) }}</span>
      </div>
      <div v-if="resolvedMutations(hoveredFish).length" class="pond-water__tooltip-mutations">
        <span v-for="mutation in resolvedMutations(hoveredFish)" :key="mutation.id">{{ t(mutation.nameKey) }}</span>
      </div>
    </div>

    <div v-if="trackedFish" class="pond-water__tracking" aria-live="polite">
      <Icon icon="lucide:scan-eye" />
      <span><small>{{ t("game.fishing.pondTracking") }}</small><strong :class="itemCustomLabelClass(trackedFish.customData)">{{ fishName(trackedFish) }}</strong></span>
      <el-tooltip :content="t('game.fishing.pondExitTracking')">
        <button type="button" :aria-label="t('game.fishing.pondExitTracking')" @click="stopTracking"><Icon icon="lucide:x" /></button>
      </el-tooltip>
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from "vue"
import type { ComponentPublicInstance } from "vue"
import { Icon } from "@iconify/vue"
import { useI18n } from "vue-i18n"
import RarityText from "@components/game/common/RarityText.vue"
import FishSizeIndicator from "./FishSizeIndicator.vue"
import { localizedFishingText, type FishingMutationDefinition, type FishingPondVisibleFish, type FishingSpecies } from "@services/fishing"
import { fishMutationClassNames, fishMutationIds, resolveFishMutations } from "@services/fish-mutations"
import { FISHING_POND_CAPABILITY } from "@services/fishing-pond-access"
import { itemCustomLabelClass } from "@services/item-customization"

type SwimMotion = "gliding" | "kicking" | "resting"
type Agent = {
  x: number
  y: number
  vx: number
  vy: number
  wander: number
  strokePeriod: number
  strokeOffset: number
  restPeriod: number
  facing: -1 | 1
  motion: SwimMotion
  dead: boolean
}
type FeedBurst = { id: number; x: number; y: number; feedClass: string; fishIds?: number[] }
type FeedingState = Omit<FeedBurst, "fishIds"> & { expiresAt: number; fishIds: Set<number> }
type PondPoint = { x: number; y: number }
type LocalInteractionMode = "throw" | "skip"
type StoneKind = "throw" | "skip"
type StoneBurst = PondPoint & { id: number; kind: StoneKind }
type StoneProjectile = PondPoint & { id: number; kind: StoneKind }
type SkipGesture = { pointerId: number; start: PondPoint; current: PondPoint }
type FearPulse = PondPoint & { radius: number; strength: number; maxSpeed: number; expiresAt: number }

const props = withDefaults(defineProps<{
  fish: readonly FishingPondVisibleFish[]
  species: readonly FishingSpecies[]
  mutations?: readonly FishingMutationDefinition[]
  capabilities: readonly string[]
  scatterMode?: boolean
  scatterBusy?: boolean
  feedDisabled?: boolean
  selectedFeedLabel?: string
  selectedFeedQuantity?: number
  feedBurst?: FeedBurst | null
}>(), {
  scatterMode: false,
  scatterBusy: false,
  feedDisabled: false,
  selectedFeedLabel: "",
  selectedFeedQuantity: 1,
  feedBurst: null
})
const emit = defineEmits<{
  "request-feed": []
  "cancel-scatter": []
  scatter: [position: { x: number; y: number }]
}>()
const { t, locale } = useI18n()
const sceneEl = ref<HTMLElement | null>(null)
const worldEl = ref<HTMLElement | null>(null)
const canvasEl = ref<HTMLCanvasElement | null>(null)
const interactionSurfaceEl = ref<HTMLButtonElement | null>(null)
const tooltipEl = ref<HTMLElement | null>(null)
const fallback = ref(false)
const paused = ref(true)
const hoveredId = ref(0)
const trackedId = ref(0)
const activeFeedBurst = ref<FeedBurst | null>(null)
const interactionMenuOpen = ref(false)
const localInteractionMode = ref<LocalInteractionMode | null>(null)
const effectBusy = ref(false)
const stoneBursts = ref<StoneBurst[]>([])
const stoneProjectile = ref<StoneProjectile | null>(null)
const skipGesture = ref<SkipGesture | null>(null)
const spriteElements = new Map<number, HTMLElement>()
const agents = new Map<number, Agent>()
let agentEntries: [number, Agent][] = []
const fearPulses: FearPulse[] = []
const effectTimers: number[] = []
let animationFrame = 0
let resizeObserver: ResizeObserver | null = null
let intersectionObserver: IntersectionObserver | null = null
let motionQuery: MediaQueryList | null = null
let inViewport = false
let disposed = false
let sceneWidth = 1
let sceneHeight = 1
let previousWaterFrame = 0
let glBuffer: WebGLBuffer | null = null
let gl: WebGLRenderingContext | null = null
let glProgram: WebGLProgram | null = null
let resolutionUniform: WebGLUniformLocation | null = null
let timeUniform: WebGLUniformLocation | null = null
let startedAt = 0
let previousFrame = 0
let reducedMotion = false
let cameraX = 0
let cameraY = 0
let cameraScale = 1
let feedingState: FeedingState | null = null
let feedBurstTimer = 0
let effectSequence = 0
let suppressNextSurfaceClick = false

const speciesById = computed(() => new Map(props.species.map((row) => [row.id, row])))
const fishById = computed(() => new Map(props.fish.map((row) => [row.instanceId, row])))
const effectiveCapabilities = computed(() => new Set(props.capabilities))
const canInspectFish = computed(() => effectiveCapabilities.value.has(FISHING_POND_CAPABILITY.inspectFish))
const canInspectVitals = computed(() => effectiveCapabilities.value.has(FISHING_POND_CAPABILITY.inspectVitals))
const canTrackFish = computed(() => effectiveCapabilities.value.has(FISHING_POND_CAPABILITY.trackFish))
const canFeed = computed(() => effectiveCapabilities.value.has(FISHING_POND_CAPABILITY.feed))
const canThrowStone = computed(() => effectiveCapabilities.value.has(FISHING_POND_CAPABILITY.throwStone))
const canSkipStone = computed(() => effectiveCapabilities.value.has(FISHING_POND_CAPABILITY.skipStone))
const hasInteractionControls = computed(() => canFeed.value || canThrowStone.value || canSkipStone.value)
const hoveredFish = computed(() => canInspectFish.value ? fishById.value.get(hoveredId.value) || null : null)
const trackedFish = computed(() => canTrackFish.value ? fishById.value.get(trackedId.value) || null : null)
const activeTool = computed<"feed" | LocalInteractionMode | null>(() => props.scatterMode && canFeed.value ? "feed" : localInteractionMode.value)
const interactionLocked = computed(() => Boolean(activeTool.value) || interactionMenuOpen.value)
const interactionSurfaceBusy = computed(() => props.scatterBusy || effectBusy.value)
const interactionSurfaceLabel = computed(() => {
  if (props.scatterBusy) return t("game.fishing.pondScatterBusy")
  if (activeTool.value === "feed") return t("game.fishing.pondScatterTargetLabel", { feed: props.selectedFeedLabel, count: props.selectedFeedQuantity })
  if (activeTool.value === "throw") return t("game.fishing.pondThrowStoneTargetLabel")
  return t("game.fishing.pondSkipStoneTargetLabel")
})
const skipPreviewPoints = computed(() => skipGesture.value
  ? buildSkipPath(skipGesture.value.start, skipGesture.value.current).map((point, index) => ({ ...point, id: index }))
  : [])
const feedBurstStyle = computed(() => ({
  left: `${(activeFeedBurst.value?.x || 0) * 100}%`,
  top: `${(activeFeedBurst.value?.y || 0) * 100}%`,
  "--feed-color": feedColor(activeFeedBurst.value?.feedClass || "")
}))
const sprites = computed(() => props.fish.map((row) => {
  const seed = stableSeed(row.instanceId)
  const agent = agents.get(row.instanceId)
  const tailDuration = .56 + ((seed >>> 23) % 28) / 100
  return {
    instanceId: row.instanceId,
    iconPath: speciesById.value.get(row.fish.speciesId)?.iconPath || "/assets/icons/item/fish_catch.png",
    dead: row.fish.lifeState === "dead",
    reverse: agent ? agent.facing < 0 : ((seed >>> 11) & 1) === 1,
    width: Math.max(34, Math.min(76, 31 + row.fish.lengthCm * .38)),
    bodyDuration: (2.45 + ((seed >>> 19) % 86) / 100).toFixed(2),
    bodyDelay: (-((seed >>> 7) % 280) / 100).toFixed(2),
    tailDuration: tailDuration.toFixed(2),
    tailGlideDuration: (tailDuration * 1.55).toFixed(2),
    tailKickDuration: (tailDuration * .68).toFixed(2),
    tailDelay: (-((seed >>> 13) % 80) / 100).toFixed(2),
    name: fishName(row),
    predator: Boolean(row.predation),
    mutationIds: fishMutationIds(row.fish),
    mutationClasses: fishMutationClassNames(row.fish, props.mutations)
  }
}))

watch(() => props.fish, syncAgents, { immediate: true })
watch(trackedFish, (row) => { if (!row) stopTracking() })
watch(() => props.scatterMode, (active) => {
  if (!active) return
  interactionMenuOpen.value = false
  localInteractionMode.value = null
})
watch(effectiveCapabilities, () => {
  if (!canInspectFish.value) hoveredId.value = 0
  if (!canTrackFish.value) stopTracking()
  if (localInteractionMode.value === "throw" && !canThrowStone.value) cancelLocalInteraction()
  if (localInteractionMode.value === "skip" && !canSkipStone.value) cancelLocalInteraction()
  if (!hasInteractionControls.value) interactionMenuOpen.value = false
})
watch(activeTool, (tool) => {
  if (!tool) { skipGesture.value = null; return }
  hoveredId.value = 0
  stopTracking()
  nextTick(() => interactionSurfaceEl.value?.focus())
})
watch(() => props.feedBurst, (burst) => {
  if (!burst || burst.id === activeFeedBurst.value?.id) return
  beginFeeding(burst)
})

function syncAgents() {
  const active = new Set(props.fish.map((row) => row.instanceId))
  for (const id of agents.keys()) if (!active.has(id)) agents.delete(id)
  for (const row of props.fish) {
    const dead = row.fish.lifeState === "dead"
    const current = agents.get(row.instanceId)
    if (current) { current.dead = dead; continue }
    const seed = stableSeed(row.instanceId)
    const angle = (seed % 6283) / 1000
    const vx = Math.cos(angle) * .012
    const vy = Math.sin(angle) * .008
    agents.set(row.instanceId, {
      x: .12 + ((seed >>> 3) % 7600) / 10000,
      y: dead ? .12 : .16 + ((seed >>> 11) % 6800) / 10000,
      vx,
      vy,
      wander: ((seed >>> 17) % 1000) / 100,
      strokePeriod: 2.2 + ((seed >>> 20) % 140) / 100,
      strokeOffset: ((seed >>> 8) % 1000) / 100,
      restPeriod: 18 + ((seed >>> 12) % 120) / 10,
      facing: vx < 0 ? -1 : 1,
      motion: "gliding",
      dead
    })
  }
  agentEntries = Array.from(agents.entries())
  nextTick(renderAgents)
}

function updateBoids(dt: number, now: number) {
  const entries = agentEntries
  for (const [id, agent] of entries) {
    if (agent.dead || id === hoveredId.value) continue
    let neighbors = 0
    let centerX = 0
    let centerY = 0
    let alignX = 0
    let alignY = 0
    let separateX = 0
    let separateY = 0
    for (const [otherId, other] of entries) {
      if (otherId === id || other.dead) continue
      const dx = other.x - agent.x
      const dy = other.y - agent.y
      const distanceSquared = dx * dx + dy * dy
      if (distanceSquared > .22 * .22) continue
      const distance = Math.sqrt(distanceSquared)
      neighbors++
      centerX += other.x
      centerY += other.y
      alignX += other.vx
      alignY += other.vy
      if (distance < .085 && distance > .0001) {
        const strength = (.085 - distance) / .085
        separateX -= dx / distance * strength
        separateY -= dy / distance * strength
      }
    }
    let ax = Math.sin(now * .00032 + agent.wander) * .0026
    let ay = Math.cos(now * .00027 + agent.wander * 1.7) * .0019
    // Independent roaming keeps a small shoal from settling into one corner.
    const roamX = .5 + Math.sin(now * .00012 + agent.wander * 2.3) * .3
    const roamY = .46 + Math.cos(now * .00016 + agent.wander) * .22
    ax += (roamX - agent.x) * .009
    ay += (roamY - agent.y) * .006
    if (neighbors > 0) {
      ax += ((centerX / neighbors - agent.x) * .010) + ((alignX / neighbors - agent.vx) * .16) + separateX * .020
      ay += ((centerY / neighbors - agent.y) * .010) + ((alignY / neighbors - agent.vy) * .16) + separateY * .020
    }
    if (agent.x < .08) ax += (.08 - agent.x) * .22
    if (agent.x > .92) ax -= (agent.x - .92) * .22
    if (agent.y < .12) ay += (.12 - agent.y) * .22
    if (agent.y > .88) ay -= (agent.y - .88) * .22
    let maxSpeed = .025
    let forcedMotion = false
    if (feedingState?.fishIds.has(id)) {
      forcedMotion = true
      const seed = stableSeed(id)
      const angle = (seed % 6283) / 1000
      const radius = .025 + ((seed >>> 12) % 28) / 1000
      const targetX = Math.max(.04, Math.min(.96, feedingState.x + Math.cos(angle) * radius))
      const targetY = Math.max(.08, Math.min(.92, feedingState.y + Math.sin(angle) * radius * .72))
      const dx = targetX - agent.x
      const dy = targetY - agent.y
      const distance = Math.max(.0001, Math.hypot(dx, dy))
      if (distance > .032) {
        ax += dx / distance * .19
        ay += dy / distance * .19
        maxSpeed = .105
      } else {
        const nibble = Math.sin(now * .018 + angle) * .012
        ax += dx * 1.8 - dy / distance * nibble
        ay += dy * 1.8 + dx / distance * nibble
        maxSpeed = .035
      }
    }
    for (const pulse of fearPulses) {
      if (pulse.expiresAt <= now) continue
      const dx = agent.x - pulse.x
      const dy = agent.y - pulse.y
      const distance = Math.max(.0001, Math.hypot(dx, dy))
      if (distance >= pulse.radius) continue
      forcedMotion = true
      const force = (1 - distance / pulse.radius) * pulse.strength
      ax += dx / distance * force
      ay += dy / distance * force
      maxSpeed = Math.max(maxSpeed, pulse.maxSpeed)
    }
    agent.vx += ax * dt
    agent.vy += ay * dt
    let speed = Math.hypot(agent.vx, agent.vy)
    const resting = !forcedMotion && ((now / 1000 + agent.strokeOffset) % agent.restPeriod) < 2.4
    if (!forcedMotion) {
      maxSpeed = .031
      const strokePhase = ((now / 1000 + agent.strokeOffset) % agent.strokePeriod) / agent.strokePeriod
      const kicking = !resting && strokePhase < .28
      agent.motion = resting ? "resting" : kicking ? "kicking" : "gliding"
      if (kicking && speed > .0001) {
        const thrust = Math.sin(strokePhase / .28 * Math.PI) * .01 * dt
        agent.vx += agent.vx / speed * thrust
        agent.vy += agent.vy / speed * thrust
      } else {
        const drag = Math.exp(-(resting ? 1.8 : .18) * dt)
        agent.vx *= drag
        agent.vy *= drag
      }
      speed = Math.hypot(agent.vx, agent.vy)
    }
    if (speed > maxSpeed) { agent.vx = agent.vx / speed * maxSpeed; agent.vy = agent.vy / speed * maxSpeed }
    if (!resting && speed < .006) agent.vx += Math.cos(agent.wander) * .045 * dt
    if (agent.vx < -.0035) agent.facing = -1
    if (agent.vx > .0035) agent.facing = 1
    agent.x = Math.max(.035, Math.min(.965, agent.x + agent.vx * dt))
    agent.y = Math.max(.08, Math.min(.92, agent.y + agent.vy * dt))
  }
}

function renderAgents() {
  if (disposed) return
  const now = performance.now()
  const width = sceneWidth
  const height = sceneHeight
  for (const [id, agent] of agents) {
    const element = spriteElements.get(id)
    if (!element) continue
    element.style.transform = `translate3d(${agent.x * width}px, ${agent.y * height}px, 0) translate(-50%, -50%)`
    const tilt = agent.dead || reducedMotion ? 0 : Math.max(-14, Math.min(14, Math.atan2(agent.vy * height, Math.abs(agent.vx) * width + .5) * 180 / Math.PI)) * agent.facing
    element.style.setProperty("--fish-tilt", `${tilt.toFixed(1)}deg`)
    element.style.setProperty("--fish-depth", (agent.dead ? 1 : .88 + agent.y * .16).toFixed(3))
    element.classList.toggle("is-reverse", agent.facing < 0)
    const isFeeding = !agent.dead && Boolean(feedingState?.fishIds.has(id))
    const feedingDistance = feedingState ? Math.hypot(agent.x - feedingState.x, agent.y - feedingState.y) : Infinity
    const isStartled = !agent.dead && fearPulses.some((pulse) => pulse.expiresAt > now && Math.hypot(agent.x - pulse.x, agent.y - pulse.y) < pulse.radius)
    const isObserved = !agent.dead && id === hoveredId.value
    const isCruising = !agent.dead && !isFeeding && !isStartled && !isObserved
    element.classList.toggle("is-gliding", isCruising && agent.motion === "gliding")
    element.classList.toggle("is-kicking", isCruising && agent.motion === "kicking")
    element.classList.toggle("is-resting", isCruising && agent.motion === "resting")
    element.classList.toggle("is-observed", isObserved)
    element.classList.toggle("is-heading-to-feed", isFeeding)
    element.classList.toggle("is-nibbling", isFeeding && feedingDistance < .095)
    element.classList.toggle("is-startled", isStartled)
  }
  updateCamera()
  updateTooltipPosition()
}

function updateCamera() {
  const scene = sceneEl.value
  const world = worldEl.value
  const target = agents.get(trackedId.value)
  if (!scene || !world) return
  if (!target && Math.abs(cameraScale - 1) < .001 && Math.abs(cameraX) < .1 && Math.abs(cameraY) < .1) {
    if (world.style.transform) stopTracking()
    return
  }
  const width = sceneWidth
  const height = sceneHeight
  const desiredScale = target ? 1.28 : 1
  const desiredX = target ? Math.max(width - width * desiredScale, Math.min(0, width * .5 - target.x * width * desiredScale)) : 0
  const desiredY = target ? Math.max(height - height * desiredScale, Math.min(0, height * .5 - target.y * height * desiredScale)) : 0
  const smoothing = reducedMotion ? 1 : .18
  cameraScale += (desiredScale - cameraScale) * smoothing
  cameraX += (desiredX - cameraX) * smoothing
  cameraY += (desiredY - cameraY) * smoothing
  world.style.transform = `translate3d(${cameraX}px, ${cameraY}px, 0) scale(${cameraScale})`
}

function updateTooltipPosition() {
  const tooltip = tooltipEl.value
  const scene = sceneEl.value
  const agent = agents.get(hoveredId.value)
  if (!tooltip || !scene || !agent) return
  const x = cameraX + agent.x * sceneWidth * cameraScale
  const y = cameraY + agent.y * sceneHeight * cameraScale
  const halfWidth = Math.max(105, tooltip.offsetWidth / 2)
  const height = tooltip.offsetHeight
  const left = Math.max(halfWidth + 8, Math.min(sceneWidth - halfWidth - 8, x))
  const above = y - height - 30
  const below = y + 30
  const top = above >= 8 ? above : Math.min(sceneHeight - height - 8, below)
  tooltip.style.left = `${left}px`
  tooltip.style.top = `${top}px`
}

function frame(now: number) {
  animationFrame = 0
  if (paused.value || disposed) return
  const interval = sceneWidth <= 720 ? 50 : 1000 / 30
  if (previousFrame && now - previousFrame < interval - .5) {
    animationFrame = requestAnimationFrame(frame)
    return
  }
  const dt = previousFrame ? Math.min(.1, (now - previousFrame) / 1000) : 0
  previousFrame = now
  for (let index = fearPulses.length - 1; index >= 0; index--) if (fearPulses[index].expiresAt <= now) fearPulses.splice(index, 1)
  if (!reducedMotion) updateBoids(dt, now)
  if (feedingState && now >= feedingState.expiresAt) finishFeeding()
  renderAgents()
  if (!previousWaterFrame || now - previousWaterFrame >= 1000 / 12) {
    drawWater(now)
    previousWaterFrame = now
  }
  if (!reducedMotion) animationFrame = requestAnimationFrame(frame)
}

function refreshActivity() {
  cancelAnimationFrame(animationFrame)
  animationFrame = 0
  paused.value = document.hidden || !inViewport
  previousFrame = 0
  previousWaterFrame = 0
  if (!paused.value && !disposed) animationFrame = requestAnimationFrame(frame)
}

function refreshMotion() {
  reducedMotion = Boolean(motionQuery?.matches)
  refreshActivity()
}

function resizeScene() {
  sceneWidth = Math.max(1, sceneEl.value?.clientWidth || 1)
  sceneHeight = Math.max(1, sceneEl.value?.clientHeight || 1)
  if (gl && canvasEl.value) resizeCanvas(canvasEl.value, gl)
  if (!paused.value) { renderAgents(); drawWater(performance.now()) }
}

function initWater() {
  const canvas = canvasEl.value
  if (!canvas) return
  gl = canvas.getContext("webgl", { antialias: false, alpha: true, premultipliedAlpha: false, powerPreference: "low-power" })
  if (!gl) { fallback.value = true; return }
  const vertex = compileShader(gl, gl.VERTEX_SHADER, `attribute vec2 a_position; void main(){ gl_Position=vec4(a_position,0.0,1.0); }`)
  const fragment = compileShader(gl, gl.FRAGMENT_SHADER, `
    precision mediump float;
    uniform vec2 u_resolution;
    uniform float u_time;
    void main(){
      vec2 uv=gl_FragCoord.xy/u_resolution.xy;
      vec2 p=vec2(uv.x*u_resolution.x/max(1.0,u_resolution.y),uv.y);
      float a=sin(p.x*19.0+sin(p.y*11.0+u_time*0.18)*1.8-u_time*0.28);
      float b=sin(p.y*23.0+sin(p.x*13.0-u_time*0.14)*1.6+u_time*0.22);
      float line=1.0-smoothstep(0.02,0.10,abs(a+b));
      float fade=smoothstep(0.02,0.2,uv.y)*(1.0-smoothstep(0.82,1.0,uv.y));
      gl_FragColor=vec4(0.88,1.0,0.96,line*fade*0.055);
    }
  `)
  if (!vertex || !fragment) { gl.deleteShader(vertex); gl.deleteShader(fragment); fallback.value = true; return }
  glProgram = gl.createProgram()
  if (!glProgram) { gl.deleteShader(vertex); gl.deleteShader(fragment); fallback.value = true; return }
  gl.attachShader(glProgram, vertex)
  gl.attachShader(glProgram, fragment)
  gl.linkProgram(glProgram)
  gl.deleteShader(vertex)
  gl.deleteShader(fragment)
  if (!gl.getProgramParameter(glProgram, gl.LINK_STATUS)) { gl.deleteProgram(glProgram); glProgram = null; fallback.value = true; return }
  glBuffer = gl.createBuffer()
  gl.bindBuffer(gl.ARRAY_BUFFER, glBuffer)
  gl.bufferData(gl.ARRAY_BUFFER, new Float32Array([-1,-1,1,-1,-1,1,-1,1,1,-1,1,1]), gl.STATIC_DRAW)
  gl.useProgram(glProgram)
  const position = gl.getAttribLocation(glProgram, "a_position")
  gl.enableVertexAttribArray(position)
  gl.vertexAttribPointer(position, 2, gl.FLOAT, false, 0, 0)
  resolutionUniform = gl.getUniformLocation(glProgram, "u_resolution")
  timeUniform = gl.getUniformLocation(glProgram, "u_time")
  startedAt = performance.now()
}

function drawWater(now: number) {
  if (!gl || !glProgram || !canvasEl.value) return
  gl.useProgram(glProgram)
  gl.uniform2f(resolutionUniform, canvasEl.value.width, canvasEl.value.height)
  gl.uniform1f(timeUniform, reducedMotion ? .35 : (now - startedAt) / 1000)
  gl.drawArrays(gl.TRIANGLES, 0, 6)
}
function compileShader(context: WebGLRenderingContext, type: number, source: string) { const shader = context.createShader(type); if (!shader) return null; context.shaderSource(shader, source); context.compileShader(shader); if (context.getShaderParameter(shader, context.COMPILE_STATUS)) return shader; context.deleteShader(shader); return null }
function resizeCanvas(canvas: HTMLCanvasElement, context: WebGLRenderingContext) {
  // Water is a faint overlay, not detail that needs a Retina-sized render target.
  const ratio = Math.min(window.devicePixelRatio || 1, sceneWidth <= 720 ? .75 : 1, Math.sqrt(960 * 540 / (sceneWidth * sceneHeight)))
  const width = Math.max(1, Math.floor(sceneWidth * ratio))
  const height = Math.max(1, Math.floor(sceneHeight * ratio))
  if (canvas.width !== width || canvas.height !== height) { canvas.width = width; canvas.height = height; context.viewport(0, 0, width, height) }
}
function stableSeed(value: number) { let seed = (value | 0) ^ 0x45d9f3b; seed = Math.imul(seed ^ (seed >>> 16), 0x45d9f3b); seed = Math.imul(seed ^ (seed >>> 16), 0x45d9f3b); return (seed ^ (seed >>> 16)) >>> 0 }
function pointStyle(point: PondPoint) { return { left: `${point.x * 100}%`, top: `${point.y * 100}%` } }
function feedParticleStyle(index: number) {
  const seed = stableSeed(activeFeedBurst.value?.id ? activeFeedBurst.value.id * 31 + index : index)
  const angle = (seed % 6283) / 1000
  const radius = 18 + ((seed >>> 9) % 42)
  return {
    "--feed-x": `${Math.cos(angle) * radius}px`,
    "--feed-y": `${Math.sin(angle) * radius * .58}px`,
    "--feed-delay": `${((seed >>> 15) % 180) / 1000}s`,
    "--feed-size": `${3 + ((seed >>> 21) % 4)}px`
  }
}
function splashParticleStyle(burst: StoneBurst, index: number) {
  const seed = stableSeed(burst.id * 41 + index)
  const angle = (seed % 6283) / 1000
  const radius = (burst.kind === "throw" ? 27 : 16) + ((seed >>> 11) % (burst.kind === "throw" ? 28 : 16))
  return {
    "--splash-x": `${Math.cos(angle) * radius}px`,
    "--splash-y": `${Math.sin(angle) * radius * .58 - (burst.kind === "throw" ? 14 : 7)}px`,
    "--splash-delay": `${((seed >>> 17) % 90) / 1000}s`,
    "--splash-size": `${2 + ((seed >>> 22) % (burst.kind === "throw" ? 5 : 3))}px`
  }
}
function beginFeeding(burst: FeedBurst) {
  window.clearTimeout(feedBurstTimer)
  const now = performance.now()
  const requestedIds = burst.fishIds ? new Set(burst.fishIds) : null
  const fishIds = new Set(props.fish
    .filter((row) => row.fish.lifeState !== "dead" && (requestedIds ? requestedIds.has(row.instanceId) : Number(row.fish.satiety ?? 100) < 100))
    .map((row) => row.instanceId))
  effectBusy.value = true
  activeFeedBurst.value = burst
  feedingState = { id: burst.id, x: burst.x, y: burst.y, feedClass: burst.feedClass, expiresAt: now + 7200, fishIds }
  feedBurstTimer = window.setTimeout(finishFeeding, 7200)
  if (reducedMotion) requestAnimationFrame(renderAgents)
}
function finishFeeding() {
  window.clearTimeout(feedBurstTimer)
  feedBurstTimer = 0
  feedingState = null
  activeFeedBurst.value = null
  effectBusy.value = false
  renderAgents()
}
function selectInteraction(mode: "feed" | LocalInteractionMode) {
  if (mode === "feed" && !canFeed.value) return
  if (mode === "throw" && !canThrowStone.value) return
  if (mode === "skip" && !canSkipStone.value) return
  if (effectBusy.value || (mode === "feed" && props.feedDisabled)) return
  interactionMenuOpen.value = false
  if (mode === "feed") { emit("request-feed"); return }
  localInteractionMode.value = mode
}
function toggleInteractionMenu() {
  if (!hasInteractionControls.value) return
  const opening = !interactionMenuOpen.value
  interactionMenuOpen.value = opening
  if (!opening) return
  hoveredId.value = 0
  stopTracking()
}
function closeInteractionMenu() { interactionMenuOpen.value = false }
function cancelLocalInteraction() {
  localInteractionMode.value = null
  skipGesture.value = null
}
function normalizedPointer(event: MouseEvent | PointerEvent): PondPoint {
  if (!sceneEl.value) return { x: .5, y: .5 }
  const bounds = sceneEl.value.getBoundingClientRect()
  return {
    x: Math.max(.04, Math.min(.96, (event.clientX - bounds.left) / Math.max(1, bounds.width))),
    y: Math.max(.08, Math.min(.9, (event.clientY - bounds.top) / Math.max(1, bounds.height)))
  }
}
function useInteractionAtPointer(event: MouseEvent) {
  if (!activeTool.value || interactionSurfaceBusy.value) return
  if (suppressNextSurfaceClick) { suppressNextSurfaceClick = false; return }
  const keyboardPlacement = event.detail === 0
  const target = keyboardPlacement ? { x: .5, y: .5 } : normalizedPointer(event)
  if (activeTool.value === "feed") { emit("scatter", target); return }
  if (activeTool.value === "throw") { launchThrow(target); return }
  const start = keyboardPlacement ? { x: .18, y: .68 } : target
  const end = keyboardPlacement
    ? { x: .8, y: .42 }
    : { x: target.x < .58 ? Math.min(.92, target.x + .48) : Math.max(.08, target.x - .48), y: Math.max(.12, target.y - .16) }
  launchSkip(start, end)
}
function startSkipGesture(event: PointerEvent) {
  if (activeTool.value !== "skip" || interactionSurfaceBusy.value || (event.button !== 0 && event.pointerType === "mouse")) return
  const point = normalizedPointer(event)
  skipGesture.value = { pointerId: event.pointerId, start: point, current: point }
  interactionSurfaceEl.value?.setPointerCapture(event.pointerId)
  event.preventDefault()
}
function moveSkipGesture(event: PointerEvent) {
  if (!skipGesture.value || skipGesture.value.pointerId !== event.pointerId) return
  skipGesture.value = { ...skipGesture.value, current: normalizedPointer(event) }
  event.preventDefault()
}
function finishSkipGesture(event: PointerEvent) {
  const gesture = skipGesture.value
  if (!gesture || gesture.pointerId !== event.pointerId) return
  const end = normalizedPointer(event)
  interactionSurfaceEl.value?.releasePointerCapture(event.pointerId)
  skipGesture.value = null
  suppressNextSurfaceClick = true
  scheduleEffect(() => { suppressNextSurfaceClick = false }, 0)
  if (Math.hypot(end.x - gesture.start.x, end.y - gesture.start.y) < .06) {
    launchSkip(gesture.start, {
      x: gesture.start.x < .58 ? Math.min(.92, gesture.start.x + .48) : Math.max(.08, gesture.start.x - .48),
      y: Math.max(.12, gesture.start.y - .16)
    })
  } else {
    launchSkip(gesture.start, end)
  }
  event.preventDefault()
}
function cancelSkipGesture(event: PointerEvent) {
  if (skipGesture.value?.pointerId !== event.pointerId) return
  skipGesture.value = null
}
function buildSkipPath(start: PondPoint, end: PondPoint): PondPoint[] {
  const dx = end.x - start.x
  const dy = end.y - start.y
  const distance = Math.hypot(dx, dy)
  const count = Math.max(3, Math.min(6, Math.round(2 + distance * 6)))
  const curve = Math.min(.055, distance * .09)
  const normalX = distance > .0001 ? -dy / distance : 0
  const normalY = distance > .0001 ? dx / distance : -1
  return Array.from({ length: count }, (_, index) => {
    const progress = (index + 1) / count
    const bend = Math.sin(progress * Math.PI) * curve
    return {
      x: Math.max(.05, Math.min(.95, start.x + dx * progress + normalX * bend)),
      y: Math.max(.09, Math.min(.9, start.y + dy * progress + normalY * bend))
    }
  })
}
function launchThrow(target: PondPoint) {
  const id = ++effectSequence
  const start = { x: Math.max(.08, Math.min(.92, target.x + (target.x < .5 ? -.13 : .13))), y: .96 }
  localInteractionMode.value = null
  effectBusy.value = true
  stoneProjectile.value = { id, kind: "throw", ...start }
  nextTick(() => requestAnimationFrame(() => {
    if (stoneProjectile.value?.id === id) stoneProjectile.value = { id, kind: "throw", ...target }
  }))
  scheduleEffect(() => {
    if (stoneProjectile.value?.id === id) stoneProjectile.value = null
    addStoneImpact(target, "throw")
  }, 360)
  scheduleEffect(() => { effectBusy.value = false }, 1900)
}
function launchSkip(start: PondPoint, end: PondPoint) {
  const points = buildSkipPath(start, end)
  const id = ++effectSequence
  const interval = 190
  localInteractionMode.value = null
  skipGesture.value = null
  effectBusy.value = true
  stoneProjectile.value = { id, kind: "skip", ...start }
  points.forEach((point, index) => {
    scheduleEffect(() => {
      if (stoneProjectile.value?.id === id) stoneProjectile.value = { id, kind: "skip", ...point }
    }, 35 + index * interval)
    scheduleEffect(() => addStoneImpact(point, "skip"), 150 + index * interval)
  })
  const travelTime = 150 + points.length * interval
  scheduleEffect(() => { if (stoneProjectile.value?.id === id) stoneProjectile.value = null }, travelTime)
  scheduleEffect(() => { effectBusy.value = false }, travelTime + 900)
}
function addStoneImpact(point: PondPoint, kind: StoneKind) {
  const burst: StoneBurst = { id: ++effectSequence, kind, ...point }
  stoneBursts.value = [...stoneBursts.value, burst]
  addFearPulse(point, kind)
  scheduleEffect(() => { stoneBursts.value = stoneBursts.value.filter((row) => row.id !== burst.id) }, kind === "throw" ? 1450 : 980)
}
function addFearPulse(point: PondPoint, kind: StoneKind) {
  const duration = kind === "throw" ? 1450 : 820
  const pulse: FearPulse = {
    ...point,
    radius: kind === "throw" ? .34 : .22,
    strength: kind === "throw" ? .48 : .34,
    maxSpeed: kind === "throw" ? .14 : .115,
    expiresAt: performance.now() + duration
  }
  fearPulses.push(pulse)
  renderAgents()
  scheduleEffect(() => {
    const index = fearPulses.indexOf(pulse)
    if (index >= 0) fearPulses.splice(index, 1)
    renderAgents()
  }, duration)
}
function scheduleEffect(callback: () => void, delay: number) {
  const timer = window.setTimeout(() => {
    const index = effectTimers.indexOf(timer)
    if (index >= 0) effectTimers.splice(index, 1)
    callback()
  }, delay)
  effectTimers.push(timer)
}
function feedColor(feedClass: string) {
  if (feedClass === "herbal") return "#b9d46f"
  if (feedClass === "insect") return "#d7a04a"
  if (feedClass === "protein") return "#ec9870"
  if (feedClass === "arcane") return "#aebaf0"
  return "#d5bd86"
}
function setSpriteElement(instanceId: number, element: Element | ComponentPublicInstance | null) { if (element instanceof HTMLElement) spriteElements.set(instanceId, element); else spriteElements.delete(instanceId) }
function showTooltip(instanceId: number) { if (!canInspectFish.value || interactionLocked.value) return; hoveredId.value = instanceId; nextTick(updateTooltipPosition) }
function hideTooltip(instanceId: number) { if (hoveredId.value === instanceId) hoveredId.value = 0 }
function trackFish(instanceId: number) { if (!canTrackFish.value || interactionLocked.value || fishById.value.get(instanceId)?.fish.lifeState === "dead") return; trackedId.value = trackedId.value === instanceId ? 0 : instanceId; if (reducedMotion) renderAgents() }
function stopTracking() { trackedId.value = 0; cameraX = 0; cameraY = 0; cameraScale = 1; if (worldEl.value) worldEl.value.style.transform = "" }
function onKeydown(event: KeyboardEvent) {
  if (event.key !== "Escape") return
  if (interactionMenuOpen.value) { interactionMenuOpen.value = false; return }
  if (props.scatterMode && canFeed.value && !props.scatterBusy) { emit("cancel-scatter"); return }
  if (localInteractionMode.value) { cancelLocalInteraction(); return }
  if (trackedId.value) stopTracking()
}
function customName(row: FishingPondVisibleFish) { return String(row.customData?.name || "").trim() }
function fishName(row: FishingPondVisibleFish) { return customName(row) || speciesName(row.fish.speciesId) }
function speciesName(speciesId: string) { const row = speciesById.value.get(speciesId); return localizedFishingText(row?.names, locale.value) || speciesId }
function fishRarity(row: FishingPondVisibleFish) { return speciesById.value.get(row.fish.speciesId)?.rarity || "common" }
function healthPercent(row: FishingPondVisibleFish) { return Math.max(0, Math.min(100, Math.round((row.fish.health || 0) * 100))) }
function predatorRule(row: FishingPondVisibleFish) { return row.predation ? t("game.fishing.pondPredatorRule", { ratio: Math.round(row.predation.maxPreyLengthRatio * 100), satiety: Math.round(row.predation.huntBelowSatiety), chance: Math.round(row.predation.dailyHuntChance * 100) }) : "" }
function resolvedMutations(row: FishingPondVisibleFish) { return resolveFishMutations(row.fish, props.mutations) }
function formatLength(value: number) { return `${Number(value || 0).toFixed(1)} cm` }
function formatWeight(value: number) { return `${Number(value || 0).toFixed(value < 1 ? 3 : 1)} kg` }

onMounted(() => {
  window.addEventListener("keydown", onKeydown)
  document.addEventListener("visibilitychange", refreshActivity)
  motionQuery = window.matchMedia("(prefers-reduced-motion: reduce)")
  reducedMotion = motionQuery.matches
  motionQuery.addEventListener("change", refreshMotion)
  initWater()
  resizeScene()
  resizeObserver = new ResizeObserver(resizeScene)
  resizeObserver.observe(sceneEl.value!)
  intersectionObserver = new IntersectionObserver(([entry]) => { inViewport = entry.isIntersecting; refreshActivity() })
  intersectionObserver.observe(sceneEl.value!)
})
onBeforeUnmount(() => {
  disposed = true
  cancelAnimationFrame(animationFrame)
  window.clearTimeout(feedBurstTimer)
  effectTimers.forEach((timer) => window.clearTimeout(timer))
  resizeObserver?.disconnect()
  intersectionObserver?.disconnect()
  motionQuery?.removeEventListener("change", refreshMotion)
  document.removeEventListener("visibilitychange", refreshActivity)
  window.removeEventListener("keydown", onKeydown)
  spriteElements.clear()
  fearPulses.splice(0)
  gl?.deleteBuffer(glBuffer)
  gl?.deleteProgram(glProgram)
  gl?.getExtension("WEBGL_lose_context")?.loseContext()
  gl = null
  glProgram = null
})
</script>

<style scoped>
.pond-water { position: relative; height: clamp(320px, 29vw, 440px); overflow: hidden; border: 1px solid #639d99; border-radius: 8px; background: #55aaa9; isolation: isolate; touch-action: manipulation; }
.pond-water__world { position: absolute; inset: 0; transform-origin: 0 0; }
.pond-water.is-tracking .pond-water__world { will-change: transform; }
.pond-water canvas, .pond-water__fish-layer, .pond-water__effect-layer, .pond-water__garden { position: absolute; inset: 0; width: 100%; height: 100%; }
.pond-water__garden { object-fit: cover; object-position: center; pointer-events: none; user-select: none; }
.pond-water canvas { display: block; }
.pond-water.is-fallback { background: #55aaa9; }
.pond-water.is-fallback canvas { display: none; }
.pond-water__effect-layer { z-index: 5; overflow: hidden; pointer-events: none; }
.pond-water__fish-layer { z-index: 3; overflow: hidden; }
.pond-fish-sprite { position: absolute; left: 0; top: 0; display: grid; place-items: center; width: max(46px, var(--fish-width)); height: max(46px, calc(var(--fish-width) * .66)); padding: 0; border: 0; background: transparent; transform: translate(-50%, -50%); cursor: pointer; will-change: transform; }
.pond-fish-sprite.is-resting .pond-fish-sprite__tail { animation-duration: calc(var(--fish-tail-duration) * 2.4); }
.pond-fish-sprite.is-resting .pond-fish-sprite__art { animation-name: pond-hover; }
.pond-fish-sprite::after { content: ""; position: absolute; inset: 2px; border: 1px solid transparent; border-radius: 50%; }
.pond-fish-sprite:hover::after, .pond-fish-sprite:focus-visible::after { border-color: rgb(255 255 255 / 72%); background: rgb(255 255 255 / 8%); }
.pond-fish-sprite.is-tracked::after { border-color: #f6d078; box-shadow: 0 0 0 3px rgb(246 208 120 / 18%); }
.pond-fish-sprite__art { --fish-saturation: .92; --fish-brightness: 1; position: relative; display: block; width: var(--fish-width); height: calc(var(--fish-width) * .72); rotate: var(--fish-tilt, 0deg); transform: scaleX(-1) scale(var(--fish-depth, 1)); filter: drop-shadow(0 5px 5px rgb(0 28 33 / 28%)) saturate(var(--fish-saturation)) brightness(var(--fish-brightness)); animation: pond-breathe var(--fish-body-duration) var(--fish-body-delay) ease-in-out infinite; transition: transform .3s cubic-bezier(.45, .04, .55, .96), rotate .3s ease-out; pointer-events: none; }
.pond-fish-sprite__art img { position: absolute; inset: 0; width: 100%; height: 100%; object-fit: contain; user-select: none; pointer-events: none; }
.pond-fish-sprite__body { clip-path: inset(0 27% 0 0); }
.pond-fish-sprite__tail { --fish-tail-angle-start: -5deg; --fish-tail-angle-end: 5deg; clip-path: inset(0 0 0 66%); transform-origin: 69% 52%; animation: pond-tail-swim var(--fish-tail-duration) var(--fish-tail-delay) ease-in-out infinite; will-change: rotate, scale; }
.pond-fish-sprite__prism, .pond-fish-sprite__rainbow { position: absolute; z-index: 2; inset: 0; opacity: 0; background-repeat: repeat; -webkit-mask: var(--fish-mutation-mask) center / contain no-repeat; mask: var(--fish-mutation-mask) center / contain no-repeat; }
.pond-fish-sprite__prism { background: linear-gradient(112deg, rgb(103 221 255 / 34%) 0%, rgb(170 250 255 / 62%) 13%, rgb(255 255 238 / 94%) 25%, rgb(255 145 224 / 66%) 38%, rgb(255 230 128 / 72%) 51%, rgb(123 255 216 / 58%) 64%, rgb(151 169 255 / 66%) 77%, rgb(255 255 238 / 90%) 89%, rgb(103 221 255 / 34%) 100%); background-size: 200% 100%; mix-blend-mode: screen; }
.pond-fish-sprite__rainbow { background: linear-gradient(102deg, rgb(255 72 102 / 78%) 0%, rgb(255 196 64 / 82%) 16%, rgb(86 229 137 / 76%) 33%, rgb(66 198 236 / 80%) 50%, rgb(93 125 255 / 80%) 67%, rgb(215 92 235 / 78%) 84%, rgb(255 72 102 / 78%) 100%); background-size: 200% 100%; mix-blend-mode: color; }
.pond-fish-sprite__glints { position: absolute; z-index: 3; inset: 3% 0; opacity: 0; }
.pond-fish-sprite__glints i { position: absolute; width: clamp(4px, 9%, 7px); aspect-ratio: 1; opacity: .8; border-radius: 50%; background: radial-gradient(circle, #fff 0 18%, #fffbd0 24%, rgb(151 236 255 / 84%) 42%, transparent 68%); filter: drop-shadow(0 0 2px #fff) drop-shadow(0 0 4px rgb(111 225 255 / 92%)); will-change: transform; }
.pond-fish-sprite__glints i::before, .pond-fish-sprite__glints i::after { content: ""; position: absolute; top: 50%; left: 50%; border-radius: 999px; background: rgb(255 255 241 / 96%); transform: translate(-50%, -50%); }
.pond-fish-sprite__glints i::before { width: 24%; height: 118%; }
.pond-fish-sprite__glints i::after { width: 118%; height: 24%; }
.pond-fish-sprite__glints i:nth-child(1) { top: 18%; left: 25%; }
.pond-fish-sprite__glints i:nth-child(2) { top: 40%; right: 14%; animation-delay: -.42s; }
.pond-fish-sprite__glints i:nth-child(3) { bottom: 14%; left: 48%; animation-delay: -.84s; }
.pond-fish-sprite__glints i:nth-child(4) { top: 29%; left: 61%; width: clamp(3px, 7%, 6px); animation-delay: -1.26s; }
.pond-fish-sprite__glints i:nth-child(5) { bottom: 26%; left: 33%; width: clamp(3px, 6%, 5px); animation-delay: -1.68s; }
.pond-fish-sprite.fish-mutation--prismatic .pond-fish-sprite__art { --fish-saturation: 1.2; }
.pond-fish-sprite.fish-mutation--prismatic .pond-fish-sprite__prism { opacity: .84; animation: fish-prismatic-flow 3.6s linear infinite; }
.pond-fish-sprite.fish-mutation--rainbow .pond-fish-sprite__art { --fish-saturation: 1.34; }
.pond-fish-sprite.fish-mutation--rainbow .pond-fish-sprite__rainbow { opacity: .78; animation: fish-rainbow-flow 4.2s linear infinite; }
.pond-fish-sprite.fish-mutation--shimmering .pond-fish-sprite__art { --fish-brightness: 1.15; }
.pond-fish-sprite.fish-mutation--shimmering .pond-fish-sprite__glints { opacity: 1; }
.pond-fish-sprite.fish-mutation--shimmering .pond-fish-sprite__glints i { animation: fish-shimmer-breathe 1.9s ease-in-out infinite; }
.pond-fish-sprite.has-stacked-fish-mutations .pond-fish-sprite__prism, .pond-fish-sprite.has-stacked-fish-mutations .pond-fish-sprite__rainbow { opacity: .6; }
.pond-fish-sprite.is-reverse .pond-fish-sprite__art { transform: scaleX(1) scale(var(--fish-depth, 1)); }
.pond-fish-sprite.is-gliding .pond-fish-sprite__tail { --fish-tail-angle-start: -2.5deg; --fish-tail-angle-end: 2.5deg; animation-duration: var(--fish-tail-glide-duration); }
.pond-fish-sprite.is-kicking .pond-fish-sprite__tail { --fish-tail-angle-start: -6.5deg; --fish-tail-angle-end: 6.5deg; animation-duration: var(--fish-tail-kick-duration); }
.pond-fish-sprite.is-observed .pond-fish-sprite__tail { --fish-tail-angle-start: -1.75deg; --fish-tail-angle-end: 1.75deg; animation-duration: 1.35s; }
.pond-fish-sprite.is-dead { opacity: .48; cursor: default; }
.pond-fish-sprite.is-dead .pond-fish-sprite__art { transform: rotate(180deg) scaleX(-1); filter: grayscale(.72) drop-shadow(0 3px 3px rgb(0 0 0 / 20%)); animation: none; transition: none; }
.pond-fish-sprite.is-dead.is-reverse .pond-fish-sprite__art { transform: rotate(180deg) scaleX(1); }
.pond-fish-sprite.is-dead .pond-fish-sprite__tail { animation: none; }
.pond-fish-sprite.is-dead .pond-fish-sprite__prism, .pond-fish-sprite.is-dead .pond-fish-sprite__rainbow, .pond-fish-sprite.is-dead .pond-fish-sprite__glints { display: none; }
.pond-fish-sprite.is-heading-to-feed .pond-fish-sprite__art { filter: drop-shadow(0 6px 7px rgb(0 28 33 / 34%)) saturate(1.08); }
.pond-fish-sprite.is-heading-to-feed .pond-fish-sprite__tail { --fish-tail-angle-start: -7deg; --fish-tail-angle-end: 7deg; animation-duration: .34s; }
.pond-fish-sprite.is-nibbling .pond-fish-sprite__art { animation: pond-nibble .34s ease-in-out infinite alternate; }
.pond-fish-sprite.is-nibbling .pond-fish-sprite__tail { --fish-tail-angle-start: -4deg; --fish-tail-angle-end: 4deg; animation-duration: .22s; }
.pond-fish-sprite.is-startled .pond-fish-sprite__art { filter: drop-shadow(0 7px 8px rgb(0 28 33 / 38%)) saturate(1.16); animation: pond-startled .18s ease-in-out infinite alternate; }
.pond-fish-sprite.is-startled .pond-fish-sprite__tail { --fish-tail-angle-start: -8deg; --fish-tail-angle-end: 8deg; animation-duration: .16s; }
.pond-fish-sprite__ecology { position: absolute; z-index: 2; top: -2px; right: -2px; display: grid; pointer-events: none; }
.pond-fish-sprite__ecology svg { box-sizing: content-box; width: 11px; height: 11px; padding: 3px; border: 1px solid rgb(255 255 255 / 54%); border-radius: 50%; background: rgb(17 40 43 / 88%); color: #ff8d82; filter: drop-shadow(0 2px 3px rgb(0 0 0 / 28%)); }
.pond-fish-sprite > i { position: absolute; inset: 35% 15%; border: 1px solid rgb(255 255 255 / 35%); border-radius: 50%; }
.pond-water__interaction-surface { position: absolute; z-index: 6; inset: 0; width: 100%; height: 100%; padding: 0; border: 0; background: rgb(246 208 120 / 2%); cursor: crosshair; touch-action: none; }
.pond-water__interaction-surface:hover { background: rgb(246 208 120 / 5%); }
.pond-water__interaction-surface.is-skip { cursor: grab; }
.pond-water__interaction-surface.is-skip:active { cursor: grabbing; }
.pond-water__interaction-surface:focus-visible { outline: 2px solid #f6d078; outline-offset: -4px; }
.pond-water__interaction-surface:disabled { cursor: wait; }
.pond-water__interaction-controls { position: absolute; z-index: 9; top: 12px; right: 12px; display: flex; align-items: stretch; gap: 6px; max-width: calc(100% - 24px); }
.pond-water__interaction-controls button, .pond-water__interaction-selection { min-height: 38px; border: 1px solid rgb(255 255 255 / 24%); background: rgb(13 32 36 / 88%); color: #fff; box-shadow: 0 5px 16px rgb(0 29 33 / 24%); backdrop-filter: blur(8px); }
.pond-water__interaction-controls button { display: inline-flex; align-items: center; justify-content: center; gap: 7px; padding: 0 12px; border-radius: 6px; font: inherit; font-size: 12px; font-weight: 700; cursor: pointer; }
.pond-water__interaction-controls button:hover:not(:disabled) { border-color: rgb(246 208 120 / 78%); background: rgb(24 56 56 / 94%); }
.pond-water__interaction-controls button:focus-visible { outline: 2px solid #f6d078; outline-offset: 2px; }
.pond-water__interaction-controls button:disabled { opacity: .48; cursor: not-allowed; }
.pond-water__interaction-controls button > svg { flex: none; color: #f6d078; font-size: 17px; }
.pond-water__interaction-controls .pond-water__interaction-action { flex: 0 0 40px; width: 40px; height: 40px; min-width: 40px; min-height: 40px; padding: 0; border-color: var(--app-border); background: color-mix(in srgb, var(--app-surface) 90%, transparent); color: var(--app-text); box-shadow: 0 5px 16px rgb(0 29 33 / 20%); }
.pond-water__interaction-controls .pond-water__interaction-action:hover:not(:disabled) { border-color: color-mix(in srgb, var(--app-primary) 58%, var(--app-border)); background: color-mix(in srgb, var(--app-primary) 10%, var(--app-surface)); }
.pond-water__interaction-controls .pond-water__interaction-action:focus-visible { outline-color: var(--app-primary); }
.pond-water__interaction-controls .pond-water__interaction-action > svg { color: color-mix(in srgb, var(--app-primary) 78%, var(--app-text)); font-size: 20px; }
.pond-water__interaction-menu { position: absolute; top: calc(100% + 6px); right: 0; display: grid; gap: 3px; width: 154px; padding: 4px; border: 1px solid var(--app-border); border-radius: 6px; background: color-mix(in srgb, var(--app-surface) 96%, transparent); color: var(--app-text); box-shadow: 0 9px 24px color-mix(in srgb, var(--app-text) 18%, transparent); backdrop-filter: blur(10px); }
.pond-water__interaction-menu button { width: 100%; justify-content: flex-start; border-color: transparent; background: transparent; color: var(--app-text); box-shadow: none; backdrop-filter: none; }
.pond-water__interaction-menu button > svg { color: color-mix(in srgb, var(--app-primary) 82%, var(--app-text)); }
.pond-water__interaction-menu button:hover:not(:disabled) { border-color: color-mix(in srgb, var(--app-primary) 32%, var(--app-border)); background: color-mix(in srgb, var(--app-primary) 10%, var(--app-surface)); }
.pond-water__interaction-menu button:focus-visible { outline-color: var(--app-primary); }
.pond-water__interaction-selection { min-width: 0; display: grid; grid-template-columns: auto minmax(0, 1fr) auto; align-items: center; gap: 7px; max-width: min(310px, calc(100vw - 88px)); padding: 0 11px; border-radius: 6px; }
.pond-water__interaction-selection svg { color: #f6d078; font-size: 17px; }
.pond-water__interaction-selection strong { overflow: hidden; font-size: 12px; text-overflow: ellipsis; white-space: nowrap; }
.pond-water__interaction-selection small { color: #f6d078; font-size: 11px; font-weight: 800; }
.pond-water__interaction-selection svg[data-icon="lucide:loader-circle"] { animation: pond-spin 1s linear infinite; }
.pond-feed-burst { position: absolute; width: 1px; height: 1px; }
.pond-feed-burst > i { position: absolute; top: 0; left: 0; width: var(--feed-size); height: var(--feed-size); border: 1px solid rgb(78 50 14 / 72%); border-radius: 45% 55% 52% 48%; background: var(--feed-color); box-shadow: 0 1px 2px rgb(15 42 38 / 38%); opacity: 0; transform: translate(-50%, -34px) scale(.65); animation: pond-feed-fall 4.8s var(--feed-delay) ease-out both; }
.pond-feed-burst__ripple { position: absolute; top: 0; left: 0; width: 24px; height: 12px; border: 1px solid rgb(231 255 240 / 58%); border-radius: 50%; opacity: 0; transform: translate(-50%, -50%) scale(.45); animation: pond-feed-ripple 1.8s ease-out both; }
.pond-feed-burst__ripple--two { animation-delay: .32s; }
.pond-stone-impact, .pond-stone-projectile, .pond-skip-preview { position: absolute; width: 1px; height: 1px; }
.pond-stone-impact::before, .pond-stone-impact::after { content: ""; position: absolute; top: 0; left: 0; border: 2px solid rgb(225 249 247 / 80%); border-radius: 50%; opacity: 0; transform: translate(-50%, -50%) scale(.25); animation: pond-stone-ripple 1.15s ease-out both; }
.pond-stone-impact::after { animation-delay: .18s; }
.pond-stone-impact.is-throw::before, .pond-stone-impact.is-throw::after { width: 54px; height: 22px; }
.pond-stone-impact.is-skip::before, .pond-stone-impact.is-skip::after { width: 30px; height: 12px; border-width: 1px; animation-duration: .78s; }
.pond-stone-impact > b { position: absolute; top: 0; left: 0; width: 13px; height: 7px; border-radius: 50%; background: rgb(220 244 241 / 72%); opacity: 0; transform: translate(-50%, -50%) scale(.4); animation: pond-splash-crown .58s ease-out both; }
.pond-stone-impact.is-skip > b { width: 8px; height: 4px; animation-duration: .42s; }
.pond-stone-impact > i { position: absolute; top: 0; left: 0; width: var(--splash-size); height: calc(var(--splash-size) * 1.45); border-radius: 50% 50% 45% 45%; background: #e1f8f5; box-shadow: 0 0 5px rgb(207 250 245 / 46%); opacity: 0; transform: translate(-50%, -50%) scale(.55); animation: pond-splash-drop .62s var(--splash-delay) ease-out both; }
.pond-stone-projectile { z-index: 1; transition-property: left, top; transition-duration: .32s; transition-timing-function: cubic-bezier(.2, .72, .28, 1); }
.pond-stone-projectile.is-skip { transition-duration: .17s; transition-timing-function: linear; }
.pond-stone-projectile > i { position: absolute; top: 0; left: 0; width: 15px; height: 10px; border: 1px solid rgb(255 255 255 / 55%); border-radius: 50%; background: #818b87; box-shadow: 0 4px 7px rgb(0 24 29 / 42%); transform: translate(-50%, -50%) rotate(-18deg); }
.pond-stone-projectile.is-throw > i { animation: pond-thrown-stone .32s ease-out both; }
.pond-stone-projectile.is-skip > i { width: 11px; height: 6px; animation: pond-skipping-stone .19s linear infinite; }
.pond-skip-preview { width: 7px; height: 4px; border: 1px solid rgb(246 208 120 / 78%); border-radius: 50%; background: rgb(246 208 120 / 20%); transform: translate(-50%, -50%); box-shadow: 0 0 0 3px rgb(246 208 120 / 7%); }
.pond-water__tooltip { position: absolute; z-index: 8; display: grid; min-width: 210px; padding: 10px 12px; border: 1px solid color-mix(in srgb, var(--el-border-color) 88%, transparent); border-radius: 6px; background: color-mix(in srgb, var(--el-bg-color-overlay) 94%, transparent); color: var(--el-text-color-primary); box-shadow: 0 10px 28px color-mix(in srgb, var(--el-text-color-primary) 18%, transparent); transform: translateX(-50%); backdrop-filter: blur(10px); pointer-events: none; }
.pond-water__tooltip > small { color: var(--el-text-color-secondary); }
.pond-water__tooltip :deep(.rarity-text) { margin-top: 2px; font-size: 11px; }
.pond-water__tooltip dl { display: grid; grid-template-columns: repeat(3, 1fr); gap: 8px; margin: 8px 0 0; padding-top: 7px; border-top: 1px solid var(--el-border-color-lighter); }
.pond-water__tooltip dl div { display: grid; }
.pond-water__tooltip dt { color: var(--el-text-color-secondary); font-size: 9px; }
.pond-water__tooltip dd { margin: 0; font-size: 11px; font-weight: 700; }
.pond-water__tooltip-ecology { display: grid; gap: 4px; margin-top: 8px; padding-top: 7px; border-top: 1px solid var(--el-border-color-lighter); }
.pond-water__tooltip-ecology span { display: flex; align-items: flex-start; gap: 5px; color: var(--el-color-danger); font-size: 10px; line-height: 1.35; }
.pond-water__tooltip-ecology svg { flex: none; margin-top: 1px; font-size: 12px; }
.pond-water__tooltip-mutations { display: flex; flex-wrap: wrap; gap: 4px; margin-top: 8px; padding-top: 7px; border-top: 1px solid var(--el-border-color-lighter); }
.pond-water__tooltip-mutations span { padding: 2px 6px; border: 1px solid color-mix(in srgb, var(--el-color-warning) 45%, var(--el-border-color)); border-radius: 4px; color: var(--el-color-warning-dark-2); font-size: 10px; font-weight: 700; }
.pond-water__tracking { position: absolute; z-index: 7; top: 12px; left: 12px; display: grid; grid-template-columns: auto minmax(0, 1fr) auto; align-items: center; gap: 8px; min-width: 220px; max-width: calc(100% - 24px); padding: 7px 8px 7px 11px; border: 1px solid rgb(255 255 255 / 20%); border-radius: 6px; background: rgb(13 32 36 / 86%); color: #fff; backdrop-filter: blur(8px); }
.pond-water__tracking > svg { color: #f6d078; font-size: 18px; }
.pond-water__tracking span { min-width: 0; display: grid; }
.pond-water__tracking small { color: #aebdbd; font-size: 9px; }
.pond-water__tracking strong { overflow: hidden; font-size: 12px; text-overflow: ellipsis; white-space: nowrap; }
.pond-water__tracking button { display: grid; place-items: center; width: 32px; height: 32px; padding: 0; border: 0; border-radius: 50%; background: rgb(255 255 255 / 8%); color: #fff; cursor: pointer; }
@keyframes pond-breathe { 0%, 100% { scale: 1 .96; } 50% { scale: 1.025 1.02; } }
@keyframes pond-hover { 0%, 100% { translate: 0 -2px; } 50% { translate: 0 2px; } }
@keyframes pond-tail-swim { 0%, 100% { rotate: var(--fish-tail-angle-start); scale: 1 .96; } 50% { rotate: var(--fish-tail-angle-end); scale: .985 1.035; } }
@keyframes pond-nibble { from { translate: 0 -1px; scale: 1 .95; } to { translate: 2px 1px; scale: .97 1.03; } }
@keyframes pond-startled { from { translate: -1px 0; scale: 1 .92; } to { translate: 2px 0; scale: 1.04 1; } }
@keyframes pond-feed-fall { 0% { opacity: 0; transform: translate(-50%, -34px) scale(.65); } 14% { opacity: 1; } 30%, 78% { opacity: 1; transform: translate(calc(-50% + var(--feed-x)), calc(-50% + var(--feed-y))) scale(1); } 100% { opacity: 0; transform: translate(calc(-50% + var(--feed-x)), calc(-50% + var(--feed-y) + 5px)) scale(.58); } }
@keyframes pond-feed-ripple { 0% { opacity: .7; transform: translate(-50%, -50%) scale(.45); } 100% { opacity: 0; transform: translate(-50%, -50%) scale(3.4); } }
@keyframes pond-stone-ripple { 0% { opacity: .9; transform: translate(-50%, -50%) scale(.25); } 100% { opacity: 0; transform: translate(-50%, -50%) scale(2.1); } }
@keyframes pond-splash-crown { 0% { opacity: .9; transform: translate(-50%, -50%) scale(.4); } 55% { opacity: .75; transform: translate(-50%, -90%) scale(2.3, 1.4); } 100% { opacity: 0; transform: translate(-50%, -30%) scale(3.2, .45); } }
@keyframes pond-splash-drop { 0% { opacity: 0; transform: translate(-50%, -50%) scale(.55); } 20% { opacity: .95; } 100% { opacity: 0; transform: translate(calc(-50% + var(--splash-x)), calc(-50% + var(--splash-y))) scale(.8); } }
@keyframes pond-thrown-stone { from { rotate: 0deg; translate: 0 12px; scale: .8; } to { rotate: 210deg; translate: 0 -5px; scale: 1; } }
@keyframes pond-skipping-stone { to { rotate: 360deg; } }
@keyframes pond-spin { to { transform: rotate(360deg); } }
@keyframes fish-prismatic-flow { from { background-position: 0% 50%; } to { background-position: 200% 50%; } }
@keyframes fish-rainbow-flow { from { background-position: 200% 50%; } to { background-position: 0% 50%; } }
@keyframes fish-shimmer-breathe { 0%, 100% { transform: scale(.72) rotate(0deg); opacity: .8; } 50% { transform: scale(1.24) rotate(45deg); opacity: 1; } }
@media (max-width: 720px) { .pond-water { min-height: 270px; }.pond-water__tooltip { display: none; }.pond-water__interaction-controls { top: 9px; right: 9px; max-width: calc(100% - 18px); }.pond-water__interaction-controls button { min-height: 42px; }.pond-water__interaction-controls .pond-water__interaction-action { flex-basis: 42px; width: 42px; height: 42px; min-width: 42px; }.pond-water__interaction-selection { min-height: 42px; }.pond-water__interaction-menu { width: 166px; }.pond-water__interaction-menu button { min-height: 44px; }.pond-water__tracking { top: 60px; left: 9px; max-width: calc(100% - 18px); } }
@media (prefers-reduced-motion: reduce) { .pond-fish-sprite__art, .pond-fish-sprite__tail, .pond-fish-sprite__prism, .pond-fish-sprite__rainbow, .pond-fish-sprite__glints i, .pond-feed-burst > i, .pond-feed-burst__ripple, .pond-stone-impact::before, .pond-stone-impact::after, .pond-stone-impact > b, .pond-stone-impact > i, .pond-stone-projectile > i { animation: none; }.pond-fish-sprite__art { transition: none; }.pond-fish-sprite.fish-mutation--prismatic .pond-fish-sprite__prism { background-position: 42% 50%; }.pond-fish-sprite.fish-mutation--rainbow .pond-fish-sprite__rainbow { background-position: 58% 50%; }.pond-feed-burst > i { opacity: .82; transform: translate(calc(-50% + var(--feed-x)), calc(-50% + var(--feed-y))); }.pond-stone-impact::before { opacity: .62; transform: translate(-50%, -50%) scale(1); }.pond-stone-impact > b { opacity: .72; transform: translate(-50%, -50%) scale(1.4); }.pond-stone-projectile { transition: none; }.pond-water__world { transition: transform .18s ease-out; } }
.pond-water.is-paused *, .pond-water.is-paused *::before, .pond-water.is-paused *::after { animation-play-state: paused !important; }
@media (max-width: 720px) {
  .pond-water { height: 280px; }
  .pond-fish-sprite__prism, .pond-fish-sprite__rainbow, .pond-fish-sprite__glints i { animation: none !important; will-change: auto; }
  .pond-water__interaction-controls, .pond-water__tracking, .pond-water__interaction-selection { min-width: 0; }
  .pond-water__interaction-controls { left: 9px; justify-content: flex-end; }
}
</style>
