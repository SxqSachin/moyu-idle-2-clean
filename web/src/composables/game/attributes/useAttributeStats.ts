import { usePlayerStore } from "@stores/player"
import { useRoleStateStore } from "@stores/role-state"
import { computed } from "vue"

export function useAttributeStats() {
  const playerStore = usePlayerStore()
  const roleStateStore = useRoleStateStore()
  const playerRole = computed(() => roleStateStore.ensureRole("player:self"))
  const stats = computed(() => playerStore.stats)
  const attributes = computed(() => playerRole.value.attributes)
  const attributeProgress = computed(() => playerRole.value.attributeProgress)
  const formula = computed(() => playerStore.formula)
  const basicAttack = computed(() => playerStore.basicAttack)

  async function loadStats(token: string) {
    await playerStore.initialize(token, true)
    roleStateStore.initializePlayerFromSnapshot(playerStore.snapshot)
  }

  const normalizedStatusValue = computed(() => {
    const normalized: Record<string, number> = {}

    for(const key in attributes.value) {
      normalized[key] = attributes.value[key].value || 0
    }

    return normalized
  })

  return { stats, attributes, attributeProgress, formula, basicAttack, loadStats, normalizedStatusValue }
}
