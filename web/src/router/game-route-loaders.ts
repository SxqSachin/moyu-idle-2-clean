import { defineComponent, h, onUnmounted, shallowRef, watch, type Component, type WatchStopHandle } from "vue"

type GameRouteModule = { default: Component }
type GameRouteLoader = () => Promise<GameRouteModule>

const gameRouteViewLoaders = {
  achievements: () => import("@views/game/achievements/index.vue"),
  affix: () => import("@views/game/affix/index.vue"),
  alchemy: () => import("@views/game/alchemy/index.vue"),
  arena: () => import("@views/game/arena/index.vue"),
  combatTrial: () => import("@views/game/combat-trial/index.vue"),
  attributes: () => import("@views/game/attributes/index.vue"),
  auction: () => import("@views/game/auction/index.vue"),
  building: () => import("@views/game/building/index.vue"),
  cheaterList: () => import("@views/game/cheater-list/index.vue"),
  combat: () => import("@views/game/combat/index.vue"),
  community: () => import("@views/game/community/index.vue"),
  crafting: () => import("@views/game/crafting/index.vue"),
  enhance: () => import("@views/game/enhance/index.vue"),
  exploration: () => import("@views/game/exploration/index.vue"),
  fishing: () => import("@views/game/fishing/index.vue"),
  pond: () => import("@views/game/fishing-pond/index.vue"),
  home: () => import("@views/game/home/index.vue"),
  market: () => import("@views/game/market/index.vue"),
  panel: () => import("@views/game/panel/index.vue"),
  shop: () => import("@views/game/shop/index.vue"),
  tasks: () => import("@views/game/tasks/index.vue"),
  training: () => import("@views/game/training/index.vue"),
  wardrobe: () => import("@views/game/wardrobe/index.vue"),
  warehouse: () => import("@views/game/warehouse/index.vue"),
  work: () => import("@views/game/work/index.vue")
} satisfies Record<string, GameRouteLoader>

const gameRoutePathLoaders: Array<[RegExp, GameRouteLoader]> = [
  [/^\/game\/?$/, gameRouteViewLoaders.home],
  [/^\/game\/home\/?$/, gameRouteViewLoaders.home],
  [/^\/game\/market\/?$/, gameRouteViewLoaders.market],
  [/^\/game\/shop\/?$/, gameRouteViewLoaders.shop],
  [/^\/game\/auction\/?$/, gameRouteViewLoaders.auction],
  [/^\/game\/arena\/?$/, gameRouteViewLoaders.arena],
  [/^\/game\/combat-trial\/?$/, gameRouteViewLoaders.combatTrial],
  [/^\/game\/combat\/?$/, gameRouteViewLoaders.combat],
  [/^\/game\/community\/?$/, gameRouteViewLoaders.community],
  [/^\/game\/attributes\/?$/, gameRouteViewLoaders.attributes],
  [/^\/game\/training\/?$/, gameRouteViewLoaders.training],
  [/^\/game\/work\/?$/, gameRouteViewLoaders.work],
  [/^\/game\/exploration\/?$/, gameRouteViewLoaders.exploration],
  [/^\/game\/fishing\/?$/, gameRouteViewLoaders.fishing],
  [/^\/game\/pond\/?$/, gameRouteViewLoaders.pond],
  [/^\/game\/building\/?$/, gameRouteViewLoaders.building],
  [/^\/game\/cheater-list\/?$/, gameRouteViewLoaders.cheaterList],
  [/^\/game\/crafting\/?$/, gameRouteViewLoaders.crafting],
  [/^\/game\/warehouse\/?$/, gameRouteViewLoaders.warehouse],
  [/^\/game\/enhance\/?$/, gameRouteViewLoaders.enhance],
  [/^\/game\/affix\/?$/, gameRouteViewLoaders.affix],
  [/^\/game\/alchemy\/?$/, gameRouteViewLoaders.alchemy],
  [/^\/game\/tasks\/?$/, gameRouteViewLoaders.tasks],
  [/^\/game\/achievements\/?$/, gameRouteViewLoaders.achievements],
  [/^\/game\/wardrobe\/?$/, gameRouteViewLoaders.wardrobe],
  [/^\/game\/panel(?:\/|$)/, gameRouteViewLoaders.panel]
]

const loadedRouteViews = new Map<GameRouteLoader, Promise<GameRouteModule>>()
const resolvedRouteViews = new Map<GameRouteLoader, GameRouteModule>()

let warmPromise: Promise<unknown[]> | null = null

export const gameRouteViews = {
  achievements: createGameRouteView("GameAchievementsRoute", gameRouteViewLoaders.achievements),
  affix: createGameRouteView("GameAffixRoute", gameRouteViewLoaders.affix),
  alchemy: createGameRouteView("GameAlchemyRoute", gameRouteViewLoaders.alchemy),
  arena: createGameRouteView("GameArenaRoute", gameRouteViewLoaders.arena),
  combatTrial: createGameRouteView("GameCombatTrialRoute", gameRouteViewLoaders.combatTrial),
  attributes: createGameRouteView("GameAttributesRoute", gameRouteViewLoaders.attributes),
  auction: createGameRouteView("GameAuctionRoute", gameRouteViewLoaders.auction),
  building: createGameRouteView("GameBuildingRoute", gameRouteViewLoaders.building),
  cheaterList: createGameRouteView("GameCheaterListRoute", gameRouteViewLoaders.cheaterList),
  combat: createGameRouteView("GameCombatRoute", gameRouteViewLoaders.combat),
  community: createGameRouteView("GameCommunityRoute", gameRouteViewLoaders.community),
  crafting: createGameRouteView("GameCraftingRoute", gameRouteViewLoaders.crafting),
  enhance: createGameRouteView("GameEnhanceRoute", gameRouteViewLoaders.enhance),
  exploration: createGameRouteView("GameExplorationRoute", gameRouteViewLoaders.exploration),
  fishing: createGameRouteView("GameFishingRoute", gameRouteViewLoaders.fishing),
  pond: createGameRouteView("GamePondRoute", gameRouteViewLoaders.pond),
  home: createGameRouteView("GameHomeRoute", gameRouteViewLoaders.home),
  market: createGameRouteView("GameMarketRoute", gameRouteViewLoaders.market),
  panel: createGameRouteView("GamePanelRoute", gameRouteViewLoaders.panel),
  shop: createGameRouteView("GameShopRoute", gameRouteViewLoaders.shop),
  tasks: createGameRouteView("GameTasksRoute", gameRouteViewLoaders.tasks),
  training: createGameRouteView("GameTrainingRoute", gameRouteViewLoaders.training),
  wardrobe: createGameRouteView("GameWardrobeRoute", gameRouteViewLoaders.wardrobe),
  warehouse: createGameRouteView("GameWarehouseRoute", gameRouteViewLoaders.warehouse),
  work: createGameRouteView("GameWorkRoute", gameRouteViewLoaders.work)
}

export function warmGameRouteViews(): Promise<unknown[]> {
  if (!warmPromise) {
    warmPromise = Promise.allSettled(Object.values(gameRouteViewLoaders).map((load) => loadGameRouteModule(load)))
  }
  return warmPromise
}

export function preloadGameRouteViewForPath(path: string): Promise<GameRouteModule | undefined> {
  const normalizedPath = path.split("?")[0].replace(/\/+$/, "") || "/game"
  const entry = gameRoutePathLoaders.find(([pattern]) => pattern.test(normalizedPath))
  return entry ? loadGameRouteModule(entry[1]) : Promise.resolve(undefined)
}

function createGameRouteView(name: string, loader: GameRouteLoader): Component {
  return defineComponent({
    name,
    setup() {
      const routeView = shallowRef<Component | null>(resolvedRouteViews.get(loader)?.default || null)
      let stopWatchingGameData: WatchStopHandle | null = null
      let disposed = false

      void import("@services/game-data-runtime").then(({ isGameDataLoaded }) => {
        if (disposed) return
        stopWatchingGameData = watch(
          isGameDataLoaded,
          (loaded) => {
            if (!loaded) return
            void loadGameRouteModule(loader).then((module) => {
              routeView.value = module.default
            })
          },
          { immediate: true }
        )
      })

      onUnmounted(() => {
        disposed = true
        stopWatchingGameData?.()
      })

      return () => routeView.value ? h(routeView.value) : null
    }
  })
}

function loadGameRouteModule(loader: GameRouteLoader): Promise<GameRouteModule> {
  let promise = loadedRouteViews.get(loader)
  if (!promise) {
    promise = loader().then((module) => {
      resolvedRouteViews.set(loader, module)
      return module
    })
    loadedRouteViews.set(loader, promise)
  }
  return promise
}
