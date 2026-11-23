import { createRouter, createWebHashHistory } from "vue-router"

const router = createRouter({
  history: createWebHashHistory(),
  routes: [
    { path: "/", redirect: "/risk" },
    { path: "/risk", component: () => import("@/views/risk/index.vue") },
    { path: "/battle", component: () => import("@/views/battle/index.vue") },
    { path: "/users", component: () => import("@/views/users/index.vue") },
    { path: "/wardrobe", component: () => import("@/views/wardrobe/index.vue") },
    { path: "/settings", component: () => import("@/views/settings/index.vue") },
    { path: "/:pathMatch(.*)*", redirect: "/risk" }
  ]
})

export default router
