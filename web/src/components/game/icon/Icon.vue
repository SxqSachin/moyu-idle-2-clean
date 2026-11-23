<template>
  <span
    v-if="shouldShowAtlas"
    class="game-icon-image game-icon-atlas"
    :style="atlasStyle"
    :aria-label="alt || iconId || `${type}.${id}`"
    role="img"
  />
  <img
    v-else-if="shouldShowImage"
    class="game-icon-image"
    :src="iconPath"
    :alt="alt || iconId || `${type}.${id}`"
    :style="imageStyle"
    @error="onIconError"
  />
  <span v-else class="game-icon-fallback" :style="textStyle">{{ fallback || "🖼️" }}</span>
</template>

<script setup lang="ts">
import { computed, ref, watch } from "vue"
import {
  findItemIconAtlasEntry,
  isItemIconAtlasCandidate,
  loadItemIconAtlas,
  loadItemIconAtlasImage,
  resolveItemIconAtlasSheet,
  useItemIconAtlas
} from "@services/icon-atlas"
import { resolveIconPathCandidatesByIconId, resolveIconPathCandidatesByTypeAndId } from "@services/item-icons"

const props = withDefaults(
  defineProps<{
    iconId?: string
    type?: string
    id?: string
    alt?: string
    size?: number
    fallback?: string
  }>(),
  {
    iconId: "",
    type: "",
    id: "",
    alt: "",
    size: 14,
    fallback: "🖼️"
  }
)

const candidateIndex = ref(0)
const itemAtlas = useItemIconAtlas()
const iconPathCandidates = computed(() => {
  if (props.type && props.id) return resolveIconPathCandidatesByTypeAndId(props.type, props.id)
  if (!props.iconId) return []
  return resolveIconPathCandidatesByIconId(props.iconId)
})
const iconPath = computed(() => iconPathCandidates.value[candidateIndex.value] || "")
const iconAtlasKeys = computed(() => {
  const keys = [...iconPathCandidates.value]
  if (props.iconId) keys.unshift(props.iconId)
  if (props.type && props.id) keys.unshift(`${props.type}.${props.id}`)
  return [...new Set(keys.filter((key) => typeof key === "string" && key.trim() !== ""))]
})
const hasAtlasCandidate = computed(() => iconAtlasKeys.value.some((key) => isItemIconAtlasCandidate(key)))
const atlasEntry = computed(() => findItemIconAtlasEntry(itemAtlas.manifest.value, iconAtlasKeys.value))
const atlasSheet = computed(() => resolveItemIconAtlasSheet(itemAtlas.manifest.value, atlasEntry.value))
const atlasImageStatus = computed(() => {
  const image = atlasSheet.value?.image || ""
  return image ? itemAtlas.imageStatus.value[image] || "idle" : "idle"
})
const shouldShowAtlas = computed(() => !!atlasEntry.value && !!atlasSheet.value && atlasImageStatus.value === "loaded")
const imageStyle = computed(() => ({ width: `${props.size}px`, height: `${props.size}px` }))
const atlasStyle = computed(() => {
  const sheet = atlasSheet.value
  const entry = atlasEntry.value
  if (!sheet || !entry || !sheet.image) return imageStyle.value
  const scale = props.size / Math.max(1, entry.w)
  return {
    ...imageStyle.value,
    backgroundImage: `url("${sheet.image}")`,
    backgroundPosition: `${-entry.x * scale}px ${-entry.y * scale}px`,
    backgroundSize: `${sheet.width * scale}px ${sheet.height * scale}px`
  }
})
const shouldShowImage = computed(() => {
  if (!iconPath.value) return false
  if (!hasAtlasCandidate.value) return true
  if (atlasEntry.value) return atlasImageStatus.value === "failed"
  return itemAtlas.failed.value || !!itemAtlas.manifest.value
})
const textStyle = computed(() => ({ fontSize: `${props.size - 8}px`, }))

watch(
  () => [props.iconId, props.type, props.id],
  () => {
    candidateIndex.value = 0
  }
)

watch(
  hasAtlasCandidate,
  (shouldLoad) => {
    if (shouldLoad) void loadItemIconAtlas()
  },
  { immediate: true }
)

watch(
  () => atlasSheet.value?.image || "",
  (image) => {
    if (image) void loadItemIconAtlasImage(image)
  },
  { immediate: true }
)

function onIconError() {
  if (candidateIndex.value < iconPathCandidates.value.length - 1) {
    candidateIndex.value += 1
    return
  }
  candidateIndex.value = iconPathCandidates.value.length
}
</script>

<style scoped>
.game-icon-image,
.game-icon-fallback {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  object-fit: contain;
  border-radius: 2px;
  vertical-align: middle;
}

.game-icon-atlas {
  flex: none;
  background-repeat: no-repeat;
}
</style>
