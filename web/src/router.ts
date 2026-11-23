import { createRouter, createWebHistory } from "vue-router"
import { gameRouteViews } from "@/router/game-route-loaders"
import { ensureLocaleNamespaces, getLocaleNamespacesForPath, getRouteLocaleNamespaces, normalizeSupportedLocale, syncDocumentTitle } from "@/i18n"

const gameBootView = () => import("@views/game/boot/index.vue")

const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: "/", component: () => import("@views/home/index.vue"), meta: { i18nNamespaces: getLocaleNamespacesForPath("/") } },
    { path: "/instructions", component: () => import("@views/instructions/index.vue"), meta: { i18nNamespaces: getLocaleNamespacesForPath("/instructions") } },
    { path: "/auth", component: () => import("@views/auth/index.vue"), meta: { requiresGuest: true, i18nNamespaces: getLocaleNamespacesForPath("/auth") } },
    { path: "/auth/forgot-password", component: () => import("@views/auth/forgot-password.vue"), meta: { requiresGuest: true, i18nNamespaces: getLocaleNamespacesForPath("/auth/forgot-password") } },
    {
      path: "/game",
      component: gameBootView,
      meta: { requiresAuth: true, i18nNamespaces: getLocaleNamespacesForPath("/game") },
      children: [
        { path: "", redirect: "/game/home" },
        { path: "home", component: gameRouteViews.home },
        { path: "market", component: gameRouteViews.market },
        { path: "shop", component: gameRouteViews.shop },
        { path: "auction", component: gameRouteViews.auction },
        { path: "arena", component: gameRouteViews.arena },
        { path: "combat-trial", component: gameRouteViews.combatTrial },
        { path: "fishing", component: gameRouteViews.fishing },
        { path: "pond", component: gameRouteViews.pond },
        { path: "combat", component: gameRouteViews.combat },
        { path: "community", component: gameRouteViews.community },
        { path: "attributes", component: gameRouteViews.attributes },
        { path: "training", component: gameRouteViews.training },
        { path: "work", component: gameRouteViews.work },
        { path: "exploration", component: gameRouteViews.exploration },
        { path: "building", component: gameRouteViews.building },
        { path: "cheater-list", component: gameRouteViews.cheaterList },
        { path: "crafting", component: gameRouteViews.crafting },
        { path: "warehouse", component: gameRouteViews.warehouse },
        { path: "enhance", component: gameRouteViews.enhance },
        { path: "affix", component: gameRouteViews.affix },
        { path: "alchemy", component: gameRouteViews.alchemy },
        { path: "alchemy/convert", redirect: "/game/alchemy?tab=convert" },
        { path: "alchemy/bench", redirect: "/game/alchemy?tab=bench" },
        { path: "alchemy/inherit", redirect: "/game/alchemy?tab=inherit" },
        { path: "alchemy/transmute", redirect: "/game/alchemy?tab=transmute" },
        { path: "alchemy/smelt", redirect: "/game/alchemy?tab=smelt" },
        { path: "smelt", redirect: "/game/alchemy?tab=smelt" },
        { path: "extract", redirect: "/game/affix?tab=detach" },
        { path: "embed", redirect: "/game/affix?tab=attach" },
        { path: "enchant", redirect: "/game/affix?tab=enchant" },
        { path: "tasks", component: gameRouteViews.tasks },
        { path: "achievements", component: gameRouteViews.achievements },
        { path: "wardrobe", component: gameRouteViews.wardrobe },
        { path: "panel/:tab", component: gameRouteViews.panel }
      ]
    },
    { path: "/:pathMatch(.*)*", redirect: "/" }
  ]
})

router.beforeEach(async (to) => {
  const token = localStorage.getItem("token") || ""
  if (to.meta.requiresAuth && !token) {
    return { path: "/auth", query: { redirect: to.fullPath } }
  }
  if (to.meta.requiresGuest && token) {
    return { path: "/game/home" }
  }
  if ((to.path === "/game/market" || to.path === "/game/auction") && token) {
    const result = await validateVerifiedMarketAccess(token)
    if (result !== true) return result
  }
  await ensureLocaleNamespaces(
    normalizeSupportedLocale(localStorage.getItem("locale")),
    getRouteLocaleNamespaces(to.path, Array.isArray(to.meta.i18nNamespaces) ? to.meta.i18nNamespaces as string[] : null)
  )
  syncDocumentTitle()
  return true
})

async function validateVerifiedMarketAccess(token: string) {
  const [{ useGameStore }, { fetchPlayerSettings }] = await Promise.all([
    import("@stores/game"),
    import("@services/player-settings")
  ])

  const gameStore = useGameStore()
  gameStore.restoreToken()
  if (!gameStore.emailVerified) {
    return fetchPlayerSettings(token)
      .then((payload) => {
        gameStore.applyRemoteSettings({
          username: payload.username,
          language: payload.language,
          theme: payload.theme,
          menuLayout: payload.menuLayout,
          simpleMode: payload.simpleMode,
          emailVerified: payload.emailVerified
        })
        if (payload.emailVerified) return true
        return { path: "/game/panel/settings", query: { subTab: "email" } }
      })
      .catch(() => ({ path: "/game/panel/settings", query: { subTab: "email" } }))
  }
  return true
}

export default router
