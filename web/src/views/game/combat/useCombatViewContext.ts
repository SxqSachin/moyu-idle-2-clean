import { inject, provide, type InjectionKey, type Ref } from "vue"
import type { useCombatPage } from "@/composables/game/combat/useCombatPage"

export type BattleTabName = "area" | "rooms" | "mine" | "battle" | "help"

type CombatPageState = ReturnType<typeof useCombatPage>

export type BattleViewSource = Pick<
  CombatPageState,
  | "worlds"
  | "selectedRoom"
  | "selectedRun"
  | "runState"
  | "runEvents"
  | "roomsLoading"
  | "runLoading"
  | "roomStatusLabel"
  | "runStatusLabel"
  | "positionLabel"
  | "selectRoom"
  | "interBattleWait"
>

export type CombatViewContext = CombatPageState & {
  activeTab: Ref<BattleTabName>
  setActiveTab: (tab: BattleTabName) => void
}

const CombatViewContextKey: InjectionKey<CombatViewContext> = Symbol("CombatViewContext")

export function provideCombatViewContext(context: CombatViewContext) {
  provide(CombatViewContextKey, context)
}

export function useCombatViewContext() {
  const context = inject(CombatViewContextKey)
  if (!context) {
    throw new Error("Combat view context is missing")
  }
  return context
}
