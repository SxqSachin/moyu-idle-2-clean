import { createApp, defineComponent, h, ref } from "vue"
import { createPinia } from "pinia"
import i18n, { setAppLocale } from "@/i18n"
import "@/style.css"
import EnemyUnitQuickRefDialogHost from "@/components/game/combat/EnemyUnitQuickRefDialogHost.vue"
import AreaView from "@/views/game/combat/views/AreaView.vue"
import { provideCombatViewContext } from "@/views/game/combat/useCombatViewContext"
import { loadAllGameData } from "@/services/game-data-runtime"
import { parseWorldRows } from "@/composables/game/combat/useCombatPage"
import { usePlayerStore } from "@/stores/player"
import rawGlobalConfig from "@shared-config/global-config.json"

await setAppLocale("zh-CN")
await loadAllGameData()

const worlds = ref(parseWorldRows((key) => i18n.global.t(key)).filter((world) => world.id === "amusement_park_001"))

const Host = defineComponent({
  setup() {
    const playerStore = usePlayerStore()
    playerStore.formula.derived = Object.fromEntries(
      rawGlobalConfig.derivedStats.formulas.map((formula) => [
        formula.targetAttr,
        formula.terms.map((term) => `${term.attrKey}*${term.coefficient}`).join("+")
      ])
    )
    provideCombatViewContext({
      selectedWorldID: ref("amusement_park_001"),
      worlds,
      createRoomName: ref("猫咪游乐园"),
      createAutoReenterOnDefeat: ref(false),
      createIsPublic: ref(true),
      createPosition: ref("vanguard"),
      createDifficultyLevel: ref(1),
      loadRooms: async () => {},
      createRoom: async () => null,
      activeTab: ref("area"),
      setActiveTab: () => {}
    } as never)
    return () => h("main", { class: "mx-auto max-w-[1180px] p-3" }, [h(AreaView), h(EnemyUnitQuickRefDialogHost)])
  }
})

createApp(Host).use(createPinia()).use(i18n).mount("#app")
