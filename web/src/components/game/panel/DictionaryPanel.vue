<template>
  <div class="space-y-4">
    <el-input
      v-if="activeTab !== 'items'"
      v-model="keyword"
      clearable
      size="small"
      :placeholder="t('game.dictionarySearchPlaceholder')"
    />
    <el-tabs v-model="activeTab" class="[&_.el-tabs__content]:overflow-visible">
      <el-tab-pane :label="t('game.dictionaryItemsTitle')" name="items" lazy>
        <ItemBrowserPanel
          :items="itemBrowserItems"
          :empty-text="t('game.dictionaryEmptyState')"
          :search-placeholder="t('game.dictionarySearchPlaceholder')"
          search-width-class="md:w-32!"
          :all-label="t('game.itemCategoryAll')"
          :grid-min-width="64"
          grid-gap-class="gap-4"
          :secondary-category-toggle-visible="false"
          :secondary-category-override-enabled="true"
        >
          <template #item="{ item, onClick }">
            <Card :item-id="item.itemId" @click="onClick" />
          </template>
        </ItemBrowserPanel>
      </el-tab-pane>
      <el-tab-pane :label="t('game.dictionarySkillsTitle')" name="skills" lazy>
        <div v-if="skillSections.length" class="space-y-4">
          <section v-for="section in skillSections" :key="`dict-skill-section-${section.id}`" class="space-y-2">
            <div class="text-sm font-semibold">{{ section.label }}</div>
            <div class="flex flex-wrap gap-2">
              <SkillQuickRef v-for="skill in section.items" :key="`dict-skill-${skill.id}`" :skill-id="skill.id" show-icon />
            </div>
          </section>
        </div>
        <div v-else class="text-xs app-muted">{{ t('game.dictionaryEmptyState') }}</div>
      </el-tab-pane>
      <el-tab-pane :label="t('game.dictionaryBuffsTitle')" name="buffs" lazy>
        <div v-if="buffGroups.length" class="space-y-5">
          <section v-for="group in buffGroups" :key="`dict-buff-group-${group.id}`" class="space-y-3">
            <div class="inline-flex items-center text-lg font-semibold">
              {{ group.label }}
            </div>
            <section v-for="section in group.sections" :key="`dict-buff-section-${group.id}-${section.id}`" class="space-y-2">
              <div class="text-sm">{{ section.label }}</div>
              <div class="flex flex-wrap gap-2">
                <BuffQuickRef v-for="buff in section.items" :key="`dict-buff-${buff.id}`" :buff-id="buff.id" :remain-turns="-1" />
              </div>
            </section>
          </section>
        </div>
        <div v-else class="text-xs app-muted">{{ t('game.dictionaryEmptyState') }}</div>
      </el-tab-pane>
      <el-tab-pane :label="t('game.dictionaryAffixesTitle')" name="affixes" lazy>
        <div v-if="affixSections.length" class="space-y-4">
          <section v-for="section in affixSections" :key="`dict-affix-section-${section.id}`" class="space-y-2">
            <div class="text-sm font-semibold">{{ section.label }}</div>
            <div class="flex flex-wrap gap-2">
              <AffixQuickRef v-for="affix in section.items" :key="`dict-affix-${affix.id}`" :affix-id="affix.id" />
            </div>
          </section>
        </div>
        <div v-else class="text-xs app-muted">{{ t('game.dictionaryEmptyState') }}</div>
      </el-tab-pane>
      <el-tab-pane :label="t('game.dictionaryAlchemyTitle')" name="alchemy" lazy>
        <AlchemyCodexPanel />
      </el-tab-pane>
      <el-tab-pane :label="t('game.dictionaryEnemiesTitle')" name="enemies" lazy>
        <div v-if="enemySections.length" class="space-y-4">
          <section v-for="section in enemySections" :key="`dict-enemy-section-${section.id}`" class="space-y-2">
            <div class="text-sm font-semibold">{{ section.label }}</div>
            <div class="flex flex-wrap gap-2">
              <EnemyUnitQuickRef v-for="enemy in section.items" :key="`dict-enemy-${enemy.id}`" :enemy-id="enemy.id" />
            </div>
          </section>
        </div>
        <div v-else class="text-xs app-muted">{{ t('game.dictionaryEmptyState') }}</div>
      </el-tab-pane>
      <el-tab-pane :label="t('game.dictionaryPersonalitiesTitle')" name="personalities" lazy>
        <div v-if="personalitySections.length" class="space-y-4">
          <section v-for="section in personalitySections" :key="`dict-personality-section-${section.id}`" class="space-y-2">
            <div class="text-sm font-semibold">{{ section.label }}</div>
            <div class="flex flex-wrap gap-2">
              <KittyPersonalityQuickRef v-for="personality in section.items" :key="`dict-personality-${personality.id}`" :personality-id="personality.id" />
            </div>
          </section>
        </div>
        <div v-else class="text-xs app-muted">{{ t('game.dictionaryEmptyState') }}</div>
      </el-tab-pane>
    </el-tabs>
  </div>
</template>

<script setup lang="ts">
import { computed, ref } from "vue"
import { useI18n } from "vue-i18n"
import { listAffixDefinitions, listItemDefinitions } from "@services/item-definition"
import { getSkillTypeDefinition, listSkillDefinitions, listSkillTypeDefinitions } from "@services/skill-definition"
import { getBuffTypeDefinition, listBuffDefinitions, listBuffTypeDefinitions } from "@services/buff-definition"
import { listEnemyDefinitions } from "@services/enemy-definition"
import { getGameDataChunk } from "@services/game-data-runtime"
import { KITTY_PERSONALITIES } from "@/config/shared-global-config"
import ItemBrowserPanel, { type ItemBrowserItem } from "@components/game/item/ItemBrowserPanel.vue"
import Card from "@components/game/item/Card.vue"
import SkillQuickRef from "@components/game/skill/SkillQuickRef.vue"
import BuffQuickRef from "@components/game/buff/BuffQuickRef.vue"
import AffixQuickRef from "@components/game/affix/AffixQuickRef.vue"
import AlchemyCodexPanel from "@components/game/alchemy/AlchemyCodexPanel.vue"
import EnemyUnitQuickRef from "@components/game/combat/EnemyUnitQuickRef.vue"
import KittyPersonalityQuickRef from "@components/game/cattery/KittyPersonalityQuickRef.vue"

type NamedEntry = { id: string; nameKey?: string; groupId: string; kind?: "buff" | "debuff"; stackPolicy?: string }
type Section = { id: string; label: string; items: NamedEntry[] }
type BuffGroupSection = { id: string; label: string; sections: Section[] }
type DictionaryWorld = { id: string; label: string; enemyIds: string[] }

const { t } = useI18n()
const activeTab = ref("items")
const keyword = ref("")

const itemBrowserItems = computed<ItemBrowserItem[]>(() =>
  listItemDefinitions()
    .map((item) => ({
      key: String(item.id || "").trim(),
      itemId: String(item.id || "").trim(),
      itemTypes: Array.isArray(item.type)
        ? item.type.map((entry) => String(entry || "").trim()).filter(Boolean)
        : []
    }))
    .filter((item) => item.itemId !== "")
)

const skills = computed<NamedEntry[]>(() =>
  listSkillDefinitions()
    .map((skill) => ({
      id: String(skill.id || "").trim(),
      nameKey: String(skill.nameKey || "").trim(),
      groupId: String(skill.category || "").trim()
    }))
    .filter((skill) => skill.id !== "")
)

const buffs = computed<NamedEntry[]>(() =>
  listBuffDefinitions()
    .map((buff) => ({
      id: String(buff.id || "").trim(),
      nameKey: String(buff.nameKey || "").trim(),
      groupId: String(buff.type || "").trim(),
      kind: buff.kind,
      stackPolicy: String(buff.stackPolicy || "").trim()
    }))
    .filter((buff) => buff.id !== "")
)

const affixes = computed<NamedEntry[]>(() =>
  listAffixDefinitions()
    .map((affix) => ({
      id: String(affix.id || "").trim(),
      nameKey: String(affix.nameKey || "").trim(),
      groupId: String(affix.type || "").trim()
    }))
    .filter((affix) => affix.id !== "")
)

const enemies = computed<NamedEntry[]>(() =>
  listEnemyDefinitions()
    .map((enemy) => ({
      id: String(enemy.id || "").trim(),
      nameKey: String(enemy.nameKey || "").trim(),
      groupId: String(enemy.position || "").trim()
    }))
    .filter((enemy) => enemy.id !== "")
)

const personalities = computed<NamedEntry[]>(() =>
  KITTY_PERSONALITIES
    .map((personality) => ({
      id: String(personality.id || "").trim(),
      nameKey: String(personality.nameKey || "").trim(),
      groupId: personalityGroupId(String(personality.id || "").trim())
    }))
    .filter((personality) => personality.id !== "")
)

const normalizedKeyword = computed(() => keyword.value.trim().toLowerCase())

function matchEntry(entry: NamedEntry) {
  if (!normalizedKeyword.value) return true
  const idText = entry.id.toLowerCase()
  const nameText = entry.nameKey ? t(entry.nameKey).toLowerCase() : ""
  return idText.includes(normalizedKeyword.value) || nameText.includes(normalizedKeyword.value)
}

const skillSections = computed(() => buildSkillSections(skills.value.filter((item) => matchEntry(item))))
const buffGroups = computed(() => buildBuffGroups(buffs.value.filter((item) => matchEntry(item))))
const affixSections = computed(() => buildAffixSections(affixes.value.filter((item) => matchEntry(item))))
const enemySections = computed(() => buildEnemySections(enemies.value.filter((item) => matchEntry(item))))
const personalitySections = computed(() => buildPersonalitySections(personalities.value.filter((item) => matchEntry(item))))

function buildSkillSections(entries: NamedEntry[]): Section[] {
  const groupOrder = listSkillTypeDefinitions().map((type) => type.id)
  return buildSections(entries, groupOrder, (groupId) => localizeDefinitionLabel(getSkillTypeDefinition(groupId)?.nameKey, groupId))
}

function buildBuffGroups(entries: NamedEntry[]): BuffGroupSection[] {
  const groupOrder = listBuffTypeDefinitions().map((type) => type.id)
  return ["buff", "debuff", "stack_marker"]
    .map((groupId) => {
      const sections = buildSections(
        entries.filter((entry) => buffDisplayGroup(entry) === groupId),
        groupOrder,
        (typeId) => localizeDefinitionLabel(getBuffTypeDefinition(typeId)?.nameKey, typeId)
      )
      return {
        id: groupId,
        label: buffDisplayGroupLabel(groupId),
        sections
      }
    })
    .filter((group) => group.sections.length > 0)
}

function buffDisplayGroup(entry: NamedEntry) {
  if (entry.stackPolicy === "stack") return "stack_marker"
  return entry.kind === "debuff" ? "debuff" : "buff"
}

function buffDisplayGroupLabel(groupId: string) {
  if (groupId === "buff") return t("game.dictionaryBuffPositiveGroup")
  if (groupId === "debuff") return t("game.dictionaryBuffNegativeGroup")
  if (groupId === "stack_marker") return t("game.dictionaryBuffStackMarkerGroup")
  return groupId
}

function buildAffixSections(entries: NamedEntry[]): Section[] {
  const groupOrder = ["battle", "work"]
  return buildSections(entries, groupOrder, (groupId) => {
    if (groupId === "battle") return t("game.dictionaryAffixBattleGroup")
    if (groupId === "work") return t("game.dictionaryAffixWorkGroup")
    return groupId
  })
}

function buildEnemySections(entries: NamedEntry[]): Section[] {
  const entryById = new Map(entries.map((entry) => [entry.id, entry]))
  const groupedEnemyIds = new Set<string>()
  const sections: Section[] = []

  for (const world of listDictionaryWorlds()) {
    const items = world.enemyIds
      .map((enemyId) => entryById.get(enemyId))
      .filter((entry): entry is NamedEntry => Boolean(entry))
    if (items.length === 0) continue
    for (const item of items) groupedEnemyIds.add(item.id)
    sections.push({
      id: world.id,
      label: world.label,
      items
    })
  }

  const ungrouped = entries.filter((entry) => !groupedEnemyIds.has(entry.id))
  if (ungrouped.length > 0) {
    sections.push({
      id: "other",
      label: t("game.dictionaryOtherGroup"),
      items: ungrouped
    })
  }

  return sections
}

function buildPersonalitySections(entries: NamedEntry[]): Section[] {
  const groupOrder = ["life", "battle_attr", "battle_mechanic"]
  return buildSections(entries, groupOrder, (groupId) => {
    if (groupId === "life") return t("game.dictionaryPersonalityLifeGroup")
    if (groupId === "battle_attr") return t("game.dictionaryPersonalityBattleAttrGroup")
    if (groupId === "battle_mechanic") return t("game.dictionaryPersonalityBattleMechanicGroup")
    return groupId
  })
}

function buildSections(entries: NamedEntry[], groupOrder: string[], labelFor: (groupId: string) => string): Section[] {
  const groups = new Map<string, NamedEntry[]>()
  const ungrouped: NamedEntry[] = []
  for (const entry of entries) {
    const groupId = normalizeGroupId(entry.groupId)
    if (!groupId || groupId === "unknown") {
      ungrouped.push(entry)
      continue
    }
    if (!groups.has(groupId)) groups.set(groupId, [])
    groups.get(groupId)?.push(entry)
  }
  const orderedIds = [...groupOrder.filter((groupId) => groups.has(groupId))]
  for (const groupId of groups.keys()) {
    if (!orderedIds.includes(groupId)) orderedIds.push(groupId)
  }
  const sections = orderedIds.map((groupId) => ({
    id: groupId,
    label: labelFor(groupId),
    items: groups.get(groupId) || []
  }))
  if (ungrouped.length > 0) {
    sections.push({
      id: "other",
      label: t("game.dictionaryOtherGroup"),
      items: ungrouped
    })
  }
  return sections.filter((section) => section.items.length > 0)
}

function localizeDefinitionLabel(nameKey: string | undefined, fallback: string) {
  if (!nameKey) return fallback
  const localized = t(nameKey)
  return localized !== nameKey && localized.trim() !== "" ? localized : fallback
}

function normalizeGroupId(groupId: string) {
  return String(groupId || "").trim()
}

function personalityGroupId(id: string) {
  const normalized = String(id || "").trim().toLowerCase()
  if ([
    "ore_lover",
    "forager",
    "deep_delver",
    "dew_chaser",
    "barn_keeper",
    "meticulous_crafter",
    "furnace_friend",
    "stitch_fixer",
    "night_shift",
    "lazy_gourmet"
  ].includes(normalized)) return "life"
  if ([
    "balanced",
    "cowardly",
    "vanguard_heart",
    "fleetfoot",
    "muscle_brain",
    "overthinker",
    "keen_eyes",
    "iron_hide",
    "glass_paw",
    "battle_dancer"
  ].includes(normalized)) return "battle_attr"
  return "battle_mechanic"
}

function listDictionaryWorlds(): DictionaryWorld[] {
  const chunk = getGameDataChunk("world") as { world?: unknown[] }
  const rows = Array.isArray(chunk.world) ? chunk.world : []
  const worlds: DictionaryWorld[] = []
  for (const row of rows) {
    if (!row || typeof row !== "object" || Array.isArray(row)) continue
    const source = row as Record<string, unknown>
    if (parseDisabledFlag(source.disabled)) continue
    const id = String(source.id || "").trim()
    if (!id) continue
    worlds.push({
      id,
      label: resolveWorldLabel(source, id),
      enemyIds: uniqueStringList([
        ...toStringArray(source.enemyIds),
        ...toStringArray(source.specialEnemyIds),
        ...parseCustomLineupEnemyIds(source.customLineupConf)
      ])
    })
  }
  return worlds
}

function parseDisabledFlag(raw: unknown): boolean {
  if (typeof raw === "boolean") return raw
  if (typeof raw === "number") return raw !== 0
  if (typeof raw === "string") {
    const normalized = raw.trim().toLowerCase()
    if (normalized === "true" || normalized === "1") return true
    if (normalized === "false" || normalized === "0") return false
  }
  return false
}

function toStringArray(value: unknown): string[] {
  if (Array.isArray(value)) {
    return value.map((item) => String(item || "").trim()).filter(Boolean)
  }
  if (typeof value !== "string" || !value.trim()) return []
  const trimmed = value.trim()
  try {
    const parsed = JSON.parse(trimmed)
    if (Array.isArray(parsed)) return parsed.map((item) => String(item || "").trim()).filter(Boolean)
  } catch {
    // Fall through to comma-separated workbook cells.
  }
  return trimmed.split(",").map((item) => item.trim()).filter(Boolean)
}

function parseCustomLineupEnemyIds(raw: unknown): string[] {
  if (!Array.isArray(raw)) return []
  return uniqueStringList(raw.flatMap((entry) => {
    if (!entry || typeof entry !== "object" || Array.isArray(entry)) return []
    return toStringArray((entry as Record<string, unknown>).lineup)
  }))
}

function uniqueStringList(values: string[]): string[] {
  return Array.from(new Set(values.map((value) => String(value || "").trim()).filter(Boolean)))
}

function resolveWorldLabel(row: Record<string, unknown>, id: string): string {
  const nameKey = String(row.nameKey || "").trim()
  if (nameKey) {
    const localized = t(nameKey)
    if (localized && localized !== nameKey) return localized
  }
  const name = String(row.name || "").trim()
  return name || id
}
</script>
