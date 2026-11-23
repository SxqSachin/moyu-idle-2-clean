<template>
  <div class="game-shell-view grid h-screen min-h-[100svh] grid-rows-[auto_minmax(0,1fr)] overflow-hidden [height:100dvh]">
    <header class="game-shell-view__topbar flex items-center justify-between gap-3 border-b border-[var(--app-border)] bg-[var(--app-surface)] px-4 py-3 text-[var(--app-text)]">
      <div class="flex flex-wrap items-center gap-2.5">
        <el-button v-if="!isCompactMenuMode" class="hidden! max-[1023px]:inline-flex!" @click="mobileMenuOpen = true">
          {{ t("game.menuOpen") }}
        </el-button>
        <span v-else class="inline-flex! max-[1023px]:hidden! text-lg font-semibold">{{ t("app.title") }}</span>
        <span v-if="isCompactMenuMode" class="hidden! max-[1023px]:inline-flex! text-lg font-semibold">{{ t("app.title") }}</span>
      </div>
      <div class="flex flex-wrap items-center gap-2.5">
        <!-- <el-tag size="small" :type="presenceStatus === 'online' ? 'success' : 'info'">
          {{ presenceStatus === "online" ? t("game.presenceOnline") : t("game.presenceOffline") }}
        </el-tag> -->
        <GlobalEffectToolbar />
        <el-dropdown trigger="click" :hide-on-click="true" @command="onSelectPlayerLanguage">
          <el-button circle :loading="updatingLanguage">
            <el-icon><Icon icon="material-symbols:translate" /></el-icon>
          </el-button>
          <template #dropdown>
            <el-dropdown-menu>
              <el-dropdown-item
                v-for="option in languageOptions"
                :key="option.value"
                :command="option.value"
                :disabled="option.value === currentPlayerLocale"
              >
                {{ option.label }}
              </el-dropdown-item>
            </el-dropdown-menu>
          </template>
        </el-dropdown>
        <el-button circle @click="onToggleDarkMode(!darkModeEnabled)">
          {{ darkModeEnabled ? "☀" : "🌙" }}
        </el-button>
        <el-dropdown trigger="click" @command="onUserCommand">
          <button class="inline-flex cursor-pointer items-center gap-2 rounded-full border border-[var(--app-border)] bg-[var(--app-surface)] px-2.5 py-1 text-[var(--app-text)]" type="button">
            <span class="inline-flex h-6 w-6 items-center justify-center rounded-full bg-[var(--app-primary)] text-xs font-semibold text-white">{{ playerInitial }}</span>
            <span class="text-[13px] leading-none">{{ playerDisplayName }}</span>
          </button>
          <template #dropdown>
            <el-dropdown-menu>
              <el-dropdown-item command="settings">{{ t("game.userMenu.settings") }}</el-dropdown-item>
              <el-dropdown-item command="switchAccount">{{ t("game.userMenu.switchAccount") }}</el-dropdown-item>
              <el-dropdown-item divided command="logout">{{ t("game.logout") }}</el-dropdown-item>
            </el-dropdown-menu>
          </template>
        </el-dropdown>
      </div>
    </header>

    <div
      class="grid h-full min-h-0 overflow-hidden"
      :class="isCompactMenuMode ? 'grid-cols-[120px_1fr] max-[1023px]:grid-cols-[42px_1fr]' : 'grid-cols-[120px_1fr] max-[1023px]:grid-cols-1'"
    >
      <aside class="game-shell-view__sidebar overflow-y-auto overflow-x-hidden bg-[var(--app-surface)] overscroll-contain max-[1023px]:hidden">
        <el-menu
          :default-active="currentTab"
          class="game-shell-view__menu h-full border-r-0 bg-transparent [--el-menu-base-level-padding:8px] [--el-menu-item-height:36px] [--el-menu-sub-item-height:36px] [&_.el-menu-item]:my-px [&_.el-menu-item]:rounded-lg [&_.el-menu-item]:text-[13px] [&_.el-sub-menu__title]:my-px [&_.el-sub-menu__title]:rounded-lg [&_.el-sub-menu__title]:text-[13px]"
          @select="onSelectMenu"
        >
          <template v-for="item in menuRegistry" :key="item.key">
            <el-divider v-if="item.type === 'divider'" class="my-[5px]!" />
            <el-sub-menu v-else-if="item.type === 'submenu'" :index="item.index">
              <template #header>
                <span class="inline-flex items-center gap-1">
                  <el-icon v-if="item.icon"><Icon :icon="item.icon" /></el-icon>
                  <span>{{ t(item.titleKey) }}</span>
                </span>
              </template>
              <el-menu-item
                v-for="child in item.children"
                :key="child.index"
                :index="child.index"
              >
                <span class="inline-flex items-center gap-1">
                  <el-icon v-if="child.icon"><Icon :icon="child.icon" /></el-icon>
                  <span>{{ t(child.titleKey) }}</span>
                </span>
              </el-menu-item>
            </el-sub-menu>
            <el-menu-item v-else :index="item.index">
              <span class="inline-flex items-center gap-1">
                <el-icon v-if="item.icon"><Icon :icon="item.icon" /></el-icon>
                <span>{{ t(item.titleKey) }}</span>
              </span>
            </el-menu-item>
          </template>
        </el-menu>
      </aside>

      <aside
        v-if="isCompactMenuMode"
        class="game-shell-view__sidebar game-shell-mobile-icon-rail hidden overflow-y-auto overflow-x-hidden bg-[var(--app-surface)] overscroll-contain max-[1023px]:block"
        @pointerdown="onMobileIconRailPointerDown"
        @pointermove="onMobileIconRailPointerMove"
        @pointerup="onMobileIconRailPointerEnd"
        @pointercancel="onMobileIconRailPointerEnd"
      >
       <el-menu
         :default-active="currentTab"
         :collapse="true"
         class="game-shell-view__menu h-full border-r-0 bg-transparent [--el-menu-base-level-padding:8px] [--el-menu-item-height:36px] [--el-menu-sub-item-height:36px] [&_.el-menu-item]:my-px [&_.el-menu-item]:rounded-lg [&_.el-menu-item]:text-[13px] [&_.el-sub-menu__title]:my-px [&_.el-sub-menu__title]:rounded-lg [&_.el-sub-menu__title]:text-[13px]"
         @select="onSelectMenu"
       >
         <template v-for="item in menuRegistry" :key="item.key">
           <el-divider v-if="item.type === 'divider'" class="my-[5px]!" />
           <el-sub-menu v-else-if="item.type === 'submenu'" :index="item.index">
             <template #header>
               <el-icon v-if="item.icon" :title="t(item.titleKey)"><Icon :icon="item.icon" /></el-icon>
             </template>
             <el-menu-item
               v-for="child in item.children"
               :key="child.index"
               :index="child.index"
               :title="t(child.titleKey)"
             >
               <el-icon v-if="child.icon"><Icon :icon="child.icon" /></el-icon>
             </el-menu-item>
           </el-sub-menu>
           <el-menu-item v-else :index="item.index" :title="t(item.titleKey)">
             <el-icon v-if="item.icon"><Icon :icon="item.icon" /></el-icon>
           </el-menu-item>
         </template>
       </el-menu>
      </aside>

      <main class="flex h-full min-h-0 min-w-0 flex-col overflow-hidden">
       <div v-if="offlineRecoveryRetryTarget" role="status" class="flex shrink-0 flex-wrap items-center gap-2 border-b border-[var(--app-border)] bg-[var(--app-surface)] px-3 py-2 text-sm text-[var(--app-text)]">
         <el-icon class="text-[var(--el-color-warning)]"><Icon icon="material-symbols:warning-outline" /></el-icon>
         <span class="min-w-0 flex-1 basis-48 break-words">{{ t("game.fastForwardStalled") }}</span>
         <el-button size="small" :loading="offlineRecoveryRetrying" :disabled="offlineRewardsInFlight" @click="retryOfflineRecovery">
           <el-icon v-if="!offlineRecoveryRetrying"><Icon icon="material-symbols:refresh" /></el-icon>
           <span>{{ t("game.fastForwardRetry") }}</span>
         </el-button>
       </div>
       <section class="game-content-scroll relative min-h-0 w-full min-w-0 flex-1 overflow-auto overscroll-contain p-0 md:p-0 border-0! [-webkit-overflow-scrolling:touch]">
         <router-view v-slot="{ Component, route }">
           <div class="game-route-stage">
             <Transition name="game-route-view">
               <div :key="route.path === '/game/market' ? route.path : route.fullPath" class="game-route-page">
                 <component :is="Component" v-if="gameEntryReady" />
               </div>
             </Transition>
           </div>
         </router-view>
       </section>
      </main>
    </div>

    <el-drawer
      v-if="!isCompactMenuMode"
      v-model="mobileMenuOpen"
      direction="ltr"
      size="62%"
      body-class="p-0! pl-2!"
      :show-close="false"
    >
      <template #header>{{ t("game.menuTitle") }}</template>
      <el-menu
       :default-active="currentTab"
       class="h-full border-r-0 bg-transparent [--el-menu-base-level-padding:8px] [--el-menu-item-height:36px] [--el-menu-sub-item-height:36px] [&_.el-menu-item]:my-px [&_.el-menu-item]:rounded-lg [&_.el-menu-item]:text-[13px] [&_.el-sub-menu__title]:my-px [&_.el-sub-menu__title]:rounded-lg [&_.el-sub-menu__title]:text-[13px]"
       @select="onSelectMenu"
      >
       <template v-for="item in menuRegistry" :key="item.key">
         <el-divider v-if="item.type === 'divider'" class="my-[5px]!" />
         <el-sub-menu v-else-if="item.type === 'submenu'" :index="item.index">
           <template #header>
             <span class="inline-flex items-center gap-1">
               <el-icon v-if="item.icon"><Icon :icon="item.icon" /></el-icon>
               <span>{{ t(item.titleKey) }}</span>
             </span>
           </template>
           <el-menu-item
             v-for="child in item.children"
             :key="child.index"
             :index="child.index"
           >
             <span class="inline-flex items-center gap-1">
               <el-icon v-if="child.icon"><Icon :icon="child.icon" /></el-icon>
               <span>{{ t(child.titleKey) }}</span>
             </span>
           </el-menu-item>
         </el-sub-menu>
         <el-menu-item v-else :index="item.index">
           <span class="inline-flex items-center gap-1">
             <el-icon v-if="item.icon"><Icon :icon="item.icon" /></el-icon>
             <span>{{ t(item.titleKey) }}</span>
           </span>
         </el-menu-item>
       </template>
      </el-menu>
    </el-drawer>

    <GameDialog
      v-model="fastForwardDialogVisible"
      class="dialog-mobile-windowed"
      :title="t('game.fastForwardRewardsSummary', { steps: fastForwardCompletedSteps })"
      width="560px"
      :close-on-click-modal="true"
    >
      <div class="flex flex-col gap-4">
        <div v-if="fastForwardOfflineDurationSeconds > 0" class="text-sm text-[var(--app-muted)]">
          {{ t("game.fastForwardOfflineDuration", { seconds: formatDurationI18n(fastForwardOfflineDurationSeconds, t)}) }}
        </div>
        <div
          v-if="!fastForwardHasDetails && fastForwardOfflineDurationSeconds > 0"
          class="rounded border border-[var(--app-border)] px-3 py-2 text-sm text-[var(--app-muted)]"
        >
          {{ t("game.fastForwardNoRewards") }}
        </div>
        <div v-if="fastForwardFishingCatchEntries.length">
          <div class="mb-2 text-sm font-semibold text-[var(--app-muted)]">{{ t("game.fishing.queueCatch") }}</div>
          <div class="grid grid-cols-1 gap-x-5 gap-y-2 sm:grid-cols-2">
            <div
              v-for="entry in fastForwardFishingCatchEntries"
              :key="`ff-fish-${entry.speciesId}`"
              class="flex min-h-12 items-center gap-3 border-b border-[var(--app-border)] py-2"
            >
              <img class="h-10 w-10 shrink-0 object-contain" :src="entry.iconPath" :alt="entry.name" />
              <div class="min-w-0 flex-1">
                <div class="truncate text-sm font-semibold">{{ entry.name }}</div>
                <div class="text-xs text-[var(--app-muted)]">
                  {{ t("game.fishing.kept") }} {{ entry.kept }} · {{ t("game.fishing.released") }} {{ entry.released }}
                </div>
              </div>
              <strong class="shrink-0 tabular-nums">×{{ entry.landed }}</strong>
            </div>
          </div>
        </div>
        <div v-if="fastForwardFishingMutationEntries.length" class="fast-forward-special-catches">
          <div class="fast-forward-special-catches__heading">
            <Icon icon="lucide:sparkles" />
            <span>{{ t("game.fishing.offlineSpecialCatches") }}</span>
          </div>
          <div class="fast-forward-special-catches__grid">
            <article
              v-for="entry in fastForwardFishingMutationEntries"
              :key="`ff-special-fish-${entry.key}`"
              class="fast-forward-special-catch"
              :aria-label="`${entry.name} · ${entry.mutations.map((mutation) => t(mutation.nameKey)).join('、')}`"
            >
              <div class="fast-forward-special-catch__art">
                <FishMutationArt
                  :src="entry.iconPath"
                  :alt="entry.name"
                  :fish="{ mutations: entry.mutationIds }"
                  :mutations="fastForwardRewards.fishingMutations"
                  width="68px"
                />
                <strong>×{{ entry.landed }}</strong>
              </div>
              <div class="fast-forward-special-catch__body">
                <strong>{{ entry.name }}</strong>
                <div class="fast-forward-special-catch__mutations" role="list">
                  <span
                    v-for="mutation in entry.mutations"
                    :key="mutation.id"
                    role="listitem"
                    :class="`is-${mutation.visualEffect}`"
                  >
                    <Icon :icon="mutation.badgeIcon" aria-hidden="true" />
                    {{ t(mutation.nameKey) }}
                  </span>
                </div>
                <small>{{ t("game.fishing.kept") }} {{ entry.kept }} · {{ t("game.fishing.released") }} {{ entry.released }}</small>
              </div>
            </article>
          </div>
          <p v-if="fastForwardFishingMutationHiddenCount > 0" class="fast-forward-special-catches__overflow">
            {{ t("game.fishing.offlineSpecialOverflow", { count: fastForwardFishingMutationHiddenCount }) }}
          </p>
        </div>
        <div v-if="fastForwardCombatItemEntries.length">
          <div class="mb-2 text-sm font-semibold text-[var(--app-muted)]">{{ t("game.fastForwardCombatItems") }}</div>
          <div class="flex flex-wrap gap-2">
            <Card
              v-for="entry in fastForwardCombatItemEntries"
              :key="`ff-combat-${entry.itemId}`"
              :item-id="entry.itemId"
              :quantity="entry.quantity"
            />
          </div>
        </div>
        <div v-if="fastForwardItemEntries.length">
          <div class="mb-2 text-sm font-semibold text-[var(--app-muted)]">{{ t("game.fastForwardRewardsItems") }}</div>
          <div class="flex flex-wrap gap-2">
            <Card
              v-for="entry in fastForwardItemEntries"
              :key="`ff-item-${entry.itemId}`"
              :item-id="entry.itemId"
              :quantity="entry.quantity"
            />
          </div>
        </div>
        <div v-if="fastForwardCostEntries.length">
          <div class="mb-2 text-sm font-semibold text-[var(--app-muted)]">{{ t("game.fastForwardCostsItems") }}</div>
          <div class="flex flex-wrap gap-2">
            <Card
              v-for="entry in fastForwardCostEntries"
              :key="`ff-cost-${entry.itemId}`"
              :item-id="entry.itemId"
              :quantity="entry.quantity"
            />
          </div>
        </div>
        <div v-if="fastForwardAttrEntries.length">
          <div class="mb-2 text-sm font-semibold text-[var(--app-muted)]">{{ t("game.fastForwardRewardsAttrExp") }}</div>
          <div class="flex flex-wrap gap-2">
            <StatusTag
              v-for="entry in fastForwardAttrEntries"
              :key="`ff-attr-${entry.attrKey}`"
              :attr-key="entry.attrKey"
              :value="entry.value"
            />
          </div>
        </div>
        <div v-if="fastForwardKittyActorEntries.length">
          <div class="mb-2 text-sm font-semibold text-[var(--app-muted)]">{{ t("game.executorKitty") }}</div>
          <div class="flex flex-col gap-3">
            <div
              v-for="entry in fastForwardKittyActorEntries"
              :key="`ff-kitty-${entry.actorID}`"
              class="rounded border border-[var(--app-border)] p-3"
            >
              <div class="mb-2 text-sm font-semibold">
                {{ t("game.queueLaneKitty", { name: entry.name }) }}
              </div>
              <div class="mb-2 text-xs text-[var(--app-muted)]">
                {{ t("game.actionExecution") }}: {{ entry.completedSteps }}
              </div>
              <div v-if="entry.itemRewards.length">
                <div class="mb-2 text-xs font-semibold text-[var(--app-muted)]">{{ t("game.fastForwardRewardsItems") }}</div>
                <div class="flex flex-wrap gap-2">
                  <Card
                    v-for="row in entry.itemRewards"
                    :key="`ff-kitty-reward-${entry.actorID}-${row.itemId}`"
                    :item-id="row.itemId"
                    :quantity="row.quantity"
                  />
                </div>
              </div>
              <div v-if="entry.itemCosts.length" class="mt-3">
                <div class="mb-2 text-xs font-semibold text-[var(--app-muted)]">{{ t("game.fastForwardCostsItems") }}</div>
                <div class="flex flex-wrap gap-2">
                  <Card
                    v-for="row in entry.itemCosts"
                    :key="`ff-kitty-cost-${entry.actorID}-${row.itemId}`"
                    :item-id="row.itemId"
                    :quantity="row.quantity"
                  />
                </div>
              </div>
              <div v-if="entry.ownerAttrExp.length" class="mt-3">
                <div class="mb-2 text-xs font-semibold text-[var(--app-muted)]">{{ t('game.furniture.sharedExperience') }}</div>
                <div class="flex flex-wrap gap-2"><StatusTag v-for="row in entry.ownerAttrExp" :key="`ff-owner-${entry.actorID}-${row.attrKey}`" :attr-key="row.attrKey" :value="row.value" /></div>
              </div>
              <div v-if="entry.attrExp.length" class="mt-3">
                <div class="mb-2 text-xs font-semibold text-[var(--app-muted)]">{{ t("game.fastForwardRewardsAttrExp") }}</div>
                <div class="flex flex-wrap gap-2">
                  <StatusTag
                    v-for="row in entry.attrExp"
                    :key="`ff-kitty-attr-${entry.actorID}-${row.attrKey}`"
                    :attr-key="row.attrKey"
                    :value="row.value"
                  />
                </div>
              </div>
            </div>
          </div>
        </div>
        <FurnitureReport :active="fastForwardDialogVisible" />
        <div v-if="fastForwardCanceledEntries.length">
          <div class="mb-2 text-sm font-semibold text-[var(--app-muted)]">{{ t("game.fastForwardCanceledQueues") }}</div>
          <div class="flex flex-col gap-1 text-sm">
            <div v-for="entry in fastForwardCanceledEntries" :key="`ff-canceled-${entry.queueId}`">
              #{{ entry.queueId }} · {{ entry.actionName }} · {{ entry.reasonLabel }}
            </div>
          </div>
        </div>
        <div v-if="fastForwardEnhanceEntries.length">
          <div class="mb-2 text-sm font-semibold text-[var(--app-muted)]">{{ t("game.fastForwardEnhanceResults") }}</div>
          <div class="flex flex-col gap-1 text-sm">
            <div v-for="entry in fastForwardEnhanceEntries" :key="`ff-enhance-${entry.queueId}-${entry.attemptNo}`">
              #{{ entry.queueId }} · +{{ entry.beforeLevel }} → +{{ entry.afterLevel }} · {{ entry.resultLabel }}
            </div>
          </div>
        </div>
      </div>
    </GameDialog>

    <GameDialog
      v-model="clientUpdateDialogVisible"
      class="dialog-mobile-windowed"
      :title="t('game.clientUpdateTitle')"
      width="460px"
      :close-on-click-modal="false"
      :show-close="false"
    >
      <div class="flex flex-col gap-3">
        <div class="text-sm text-[var(--app-text)]">
          {{ t(clientUpdateMessageKey) }}
        </div>
        <div class="text-xs text-[var(--app-muted)]">
          {{ t("game.clientUpdateAutoReloadIn", { seconds: clientUpdateCountdownSeconds }) }}
        </div>
      </div>
      <template #footer>
        <el-button type="primary" @click="reloadForClientUpdate">{{ t("game.clientUpdateRefreshNow") }}</el-button>
      </template>
    </GameDialog>

    <GameDialog
      v-model="onboardingPromptVisible"
      class="onboarding-prompt-dialog dialog-mobile-windowed"
      modal-class="onboarding-prompt-overlay"
      :title="t('game.onboardingPrompt.title')"
      width="430px"
      :close-on-click-modal="false"
      :close-on-press-escape="true"
      @close="handleOnboardingPromptClosed"
    >
      <div class="onboarding-prompt-body">
        <div class="onboarding-prompt-icon">
          <Icon icon="material-symbols:explore-outline-rounded" />
        </div>
        <p class="onboarding-prompt-message">
          {{ t("game.onboardingPrompt.message") }}
        </p>
      </div>
      <template #footer>
        <div class="onboarding-prompt-actions">
          <el-button class="onboarding-prompt-secondary" @click="skipOnboardingPrompt">
            <el-icon><Icon icon="material-symbols:directions-walk-rounded" /></el-icon>
            {{ t("game.onboardingPrompt.cancel") }}
          </el-button>
          <el-button class="onboarding-prompt-primary" type="warning" @click="startOnboardingFromPrompt">
            <el-icon><Icon icon="material-symbols:travel-explore-rounded" /></el-icon>
            {{ t("game.onboardingPrompt.confirm") }}
          </el-button>
        </div>
      </template>
    </GameDialog>

    <transition
      enter-active-class="transition-opacity duration-380 ease-out"
      enter-from-class="opacity-0"
      enter-to-class="opacity-100"
      leave-active-class="transition-opacity duration-380 ease-in"
      leave-from-class="opacity-100"
      leave-to-class="opacity-0"
    >
      <div
        v-if="!gameEntryReady"
        class="fixed inset-0 z-[3000] flex items-center justify-center bg-black/65 dark:bg-black/85 backdrop-blur-[1px]"
        aria-live="polite"
        :aria-busy="gameEntryInitializing"
      >
        <div v-if="gameEntryErrorMessage" class="flex max-w-sm flex-col items-center gap-3 px-6 text-center text-white">
          <Icon icon="lucide:circle-alert" class="text-3xl" />
          <p class="m-0 text-sm font-medium">{{ gameEntryErrorMessage }}</p>
          <el-button type="primary" :loading="gameEntryInitializing" @click="retryGameEntryInitialization">
            <Icon icon="lucide:refresh-cw" />
            {{ t("game.refresh") }}
          </el-button>
        </div>
        <div v-else class="flex w-full max-w-sm flex-col items-center gap-3 px-6 text-center text-white">
          <div class="h-10 w-10 animate-spin rounded-full border-4 border-white/30 border-t-white"></div>
        </div>
      </div>
    </transition>

    <ChatFloatingPanel v-if="gameEntryReady" />
    <ArenaReplayHost />
    <CombatTrialReplayHost />
    <ItemPopoverDialogHost />
    <SkillQuickRefDialogHost />
    <EnemyUnitQuickRefDialogHost />
    <ActionQuickRefDialogHost />
    <UserProfileDialogHost />
    <ProfileDetailDialogHost />
  </div>
</template>

<script setup lang="ts">
import "@/style.css"
import "element-plus/theme-chalk/el-message.css"
import "element-plus/theme-chalk/el-notification.css"
import { computed, nextTick, onMounted, onUnmounted, ref, watch } from "vue"
import { ElMessage, ElNotification } from "element-plus"
import { Icon } from "@iconify/vue"
import { useI18n } from "vue-i18n"
import { useRoute, useRouter } from "vue-router"
import { useGameStore } from "@stores/game"
import { useTutorialStore } from "@/tutorial"
import { useInventoryStore } from "@stores/inventory"
import { usePlayerStore } from "@stores/player"
import { useRealtimeStore } from "@stores/realtime"
import { useWarehouseStore } from "@stores/warehouse"
import { useEquipmentStore } from "@stores/equipment"
import { useRoleStateStore } from "@stores/role-state"
import { useChatStore } from "@stores/chat"
import { useWardrobeStore } from "@stores/wardrobe"
import { useAchievementsStore } from "@stores/achievements"
import { useSpecializationStore } from "@stores/specialization"
import { useFishingStore } from "@stores/fishing"
import { useOperationEffects } from "@/composables/game/useOperationEffects"
import { useLocalUserStore } from "@stores/local-user"
import { useUserProfileDialogStore } from "@stores/user-profile-dialog"
import Card from "@components/game/item/Card.vue"
import StatusTag from "@components/game/attribute/StatusTag.vue"
import FishMutationArt from "@components/game/fishing/FishMutationArt.vue"
import ChatFloatingPanel from "@components/game/chat/ChatFloatingPanel.vue"
import ArenaReplayHost from "@views/game/arena/ArenaReplayHost.vue"
import CombatTrialReplayHost from "@views/game/combat-trial/CombatTrialReplayHost.vue"
import { bind, connectWS, send } from "@services/ws"
import { fetchBootstrapInit } from "@services/bootstrap"
import { resolveErrorMessage } from "@services/error-messages"
import { resolveFeedbackUrl } from "@services/feedback"
import { clearOnboardingPromptPending, shouldShowOnboardingPrompt } from "@services/onboarding-prompt"
import { AUTH_EXPIRED_EVENT, apiPost, isAuthExpiredError } from "@services/api"
import { fetchPlayerSettings, updatePlayerLanguage, updatePlayerTheme } from "@services/player-settings"
import { resolveVisibilityRecoveryAction } from "@services/visibility-recovery"
import {
  createOfflineFastForwardRequestPayload,
  createOfflineFastForwardRetryPayload,
  isOfflineFastForwardResultFailure,
  isOfflineFastForwardResultForRequest,
  isOfflineFastForwardResultStalled,
  mergeOfflineRewardsPayload,
  normalizedFastForwardMutationGroups,
  shouldReplaceOfflineFastForwardAccumulator
} from "@services/offline-fast-forward"
import type { FastForwardFishingCatchTally } from "@services/offline-fast-forward"
import {
  clearOfflineRecoveryAcknowledgement,
  isOfflineRecoveryReportReady,
  pendingOfflineRecoveryAcknowledgement,
  presentOfflineRecoveryReport
} from "@services/offline-recovery-report"
import { getFishSpeciesDisplay, localizedFishText } from "@services/fish-instance-display"
import { resolveFishMutations } from "@services/fish-mutations"
import type { FishingMutationDefinition } from "@services/fishing"
import { VISIBILITY_OFFLINE_DELAY_MS } from "@/config/shared-global-config"
import { listActionDefinitions, listActionMainTypeDefinitions } from "@services/action-definition"
import { formatDurationI18n } from "@/utils/duration-format"
import { GameFullLocaleNamespaces, getLocaleNamespacesForPath, getLocaleOptions, normalizeSupportedLocale, setAppLocale } from "@/i18n"
import { switchAppLocale } from "@services/locale-switch"
import ActionQuickRefDialogHost from "@components/game/action/ActionQuickRefDialogHost.vue"
import ItemPopoverDialogHost from "@components/game/item/ItemPopoverDialogHost.vue"
import SkillQuickRefDialogHost from "@components/game/skill/SkillQuickRefDialogHost.vue"
import EnemyUnitQuickRefDialogHost from "@components/game/combat/EnemyUnitQuickRefDialogHost.vue"
import UserProfileDialogHost from "@components/common/UserProfileDialogHost.vue"
import ProfileDetailDialogHost from "@components/common/ProfileDetailDialogHost.vue"
import GameDialog from "@components/game/common/GameDialog.vue"
import GlobalEffectToolbar from "@components/game/operation-effect/GlobalEffectToolbar.vue"
import type { OperationEffectProjection } from "@services/operation-effects"
import { useCatteryFurnitureStore } from '@stores/cattery-furniture'
import FurnitureReport from '@components/game/panel/cattery/FurnitureReport.vue'
import type { FurnitureProjection } from '@services/cattery-furniture'

const { t, te, locale } = useI18n()
const route = useRoute()
const router = useRouter()
const store = useGameStore()
store.restoreToken()
const inventoryStore = useInventoryStore()
const playerStore = usePlayerStore()
const tutorialStore = useTutorialStore()
const realtimeStore = useRealtimeStore()
const warehouseStore = useWarehouseStore()
const equipmentStore = useEquipmentStore()
const roleStateStore = useRoleStateStore()
const chatStore = useChatStore()
const wardrobeStore = useWardrobeStore()
const achievementsStore = useAchievementsStore()
const specializationStore = useSpecializationStore()
const fishingStore = useFishingStore()
const operationEffects = useOperationEffects({ token: () => store.token, autoRefresh: true })
const localUserStore = useLocalUserStore()
const userProfileDialogStore = useUserProfileDialogStore()
const presenceStatus = ref<"online" | "offline">("offline")
const darkModeEnabled = ref(false)
const updatingLanguage = ref(false)
const mobileMenuOpen = ref(false)
const mobileIconRailScrolling = ref(false)
const fastForwardDialogVisible = ref(false)
const offlineRewardsLoading = ref(false)
const offlineRecoveryPaused = ref(false)
const offlineRecoveryRetryTarget = ref<ReturnType<typeof createOfflineFastForwardRetryPayload>>(null)
const offlineRecoveryRetrying = ref(false)
let offlineRecoveryPollTimer: number | null = null
let offlineRecoveryLastProgressAt = 0
let offlineRecoveryLastVersion = ""
let offlineRecoveryCommand: Record<string, unknown> = {}
let offlineRecoverySynced: Record<string, unknown> = {}
let offlineRecoveryGeneration = 0
let offlineRecoveryVisibleReport = ""
let offlineRecoverySnapshotTask = ""
const clientUpdateDialogVisible = ref(false)
const onboardingPromptVisible = ref(false)
const gameEntryReady = ref(false)
const gameEntryInitializing = ref(false)
const gameEntryErrorMessage = ref("")
const clientUpdateMessageKey = ref("game.clientUpdateAvailable")
const clientUpdateCountdownSeconds = ref(0)
let onboardingPromptHandled = false
let presenceTimer: number | null = null
let wsConnected = false
let presenceTransitionVersion = 0
let visibilityOfflineTimer: number | null = null
let visibilityHiddenSinceMs = 0
let visibilityOfflinePromise: Promise<boolean> | null = null
let themeTransitionFrame: number | null = null
let visibilityRecoveryPending = false
let presenceOfflineFromDisconnect = false
let gameWSUrl = ""
let releaseReloadTimer: number | null = null
let releaseCountdownTimer: number | null = null
let offlineRewardsLoadingTimer: number | null = null
let ownProfileSettingsRefreshSeq = 0
let mobileIconRailPointer: { id: number; startX: number; startY: number } | null = null
let mobileIconRailScrollTimer: number | null = null
let offlineRewardsLoadingStartedAt = 0
const offlineRewardsInFlight = ref(false)
let offlineRewardsRequestTimer: number | null = null
let offlineRewardsAccumulatedPayload: any | null = null
let offlineRewardsActiveRequestId = ""
let wsAuthBound = false
let initialRealtimeSyncDone = false
const visibilityChangeListener = () => {
  void handleVisibilityChange()
}
const wsUnbinders: Array<() => void> = []
let handlingAuthExpired = false
const offlineRewardsRequestTimeoutMS = 12_000

type ClientUpdatePayload = {
  releaseId?: string
  kind?: "soft" | "force" | string
  messageI18nKey?: string
  recommendedDelaySec?: number
  forceAtMs?: number
  minAcceptedBuild?: string
  announcedAtMs?: number
}

type MenuLeaf = {
  index: string
  titleKey: string
  icon?: string
}

type MenuNode =
  | { type: "divider"; key: string }
  | { type: "item"; key: string; index: string; titleKey: string; icon?: string }
  | { type: "submenu"; key: string; index: string; titleKey: string; icon?: string; children: MenuLeaf[] }

type DynamicActionSeed = {
  id: string
  nameKey?: string
  icon?: string
  order?: number
}

const defaultTab = "homepage"
const updateLogReadReleasePrefix = "update-log:last-read-release-id"
const updateLogPanelPath = "/game/panel/updateLog"
const actionMainTypes = ref<DynamicActionSeed[]>([])
const currentSystemReleaseId = ref("")
let hasAutoRedirectedToUpdateLog = false
const standaloneActionMainTypes = new Set(["alchemy", "affix", "equipment", "enhance", "fishing"])

const dynamicActionMenu = computed<MenuNode[]>(() =>
  actionMainTypes.value
    .filter((seed) => !standaloneActionMainTypes.has(String(seed.id || "").trim()))
    .map((seed) => ({
      type: "item" as const,
      key: `dynamic-${seed.id}`,
      index: seed.id,
      titleKey: seed.nameKey || `action_main_type.${seed.id}.name`,
      icon: normalizeMenuIcon(seed.icon)
    }))
)
const isCompactMenuMode = computed(() => store.playerMenuLayout === "icon-rail")

const routeBackedTabEntries = [
  ["homepage", "/game/home"],
  ["shop", "/game/shop"],
  ["market", "/game/market"],
  ["auction", "/game/auction"],
  ["battle", "/game/combat"],
  ["arena", "/game/arena"],
  ["combatTrial", "/game/combat-trial"],
  ["fishing", "/game/fishing"],
  ["pond", "/game/pond"],
  ["character", "/game/attributes"],
  ["exercise", "/game/training"],
  ["explore", "/game/exploration"],
  ["building", "/game/building"],
  ["inventory", "/game/warehouse"],
  ["enhance", "/game/enhance"],
  ["affix", "/game/affix"],
  ["alchemy", "/game/alchemy"],
  ["quest", "/game/tasks"],
  ["achievements", "/game/achievements"],
  ["community", "/game/community"]
  // ["cheaterList", "/game/cheater-list"]
] as const

const tabRouteMap: Record<string, string> = Object.fromEntries(routeBackedTabEntries)
const routeTabMap: Record<string, string> = Object.fromEntries(
  routeBackedTabEntries.map(([tab, path]) => [path, tab])
)

const menuRegistry = computed<MenuNode[]>(() => {
  const nodes: MenuNode[] = [
    { type: "item", key: "homepage", index: "homepage", icon: 'material-symbols:family-home-outline-rounded', titleKey: "game.menu.homepage" },
    { type: "divider", key: "divider-1" },
    { type: "item", key: "character", index: "character", icon: 'material-symbols:emoji-people-rounded', titleKey: "game.menu.character" },
    // { type: "item", key: "friend", index: "friend", titleKey: "game.menu.friend" },
    { type: "item", key: "quest", index: "quest", icon: 'material-symbols:format-list-bulleted', titleKey: "game.menu.quest" },
    { type: "item", key: "achievements", index: "achievements", icon: 'material-symbols:trophy-outline-rounded', titleKey: "game.menu.achievements" },
    { type: "item", key: "building", icon: 'material-symbols:home-work-outline-rounded', index: "building", titleKey: "game.menu.building" },
    ...(Number(playerStore.buildings.cattery || 0) > 0
      ? ([{ type: "item", key: "cattery", icon: 'game-icons:cat', index: "cattery", titleKey: "game.menu.cattery" }] as MenuNode[])
      : []),
    { type: "item", key: "inventory", icon: 'material-symbols:warehouse-outline-rounded', index: "inventory", titleKey: "game.menu.inventory" },
    { type: "divider", key: "divider-2" },
    // {
    //   type: "submenu",
    //   key: "village",
    //   index: "village",
    //   titleKey: "game.menuGroup.village",
    //   children: [
    //     // { index: "village", titleKey: "game.menu.village" },
    //     // { index: "playerShop", titleKey: "game.menu.playerShop" }
    //   ]
    // },

    { type:"item", key: "shop", icon: 'material-symbols:shopping-basket-outline', index: "shop", titleKey: "game.menu.shop" },
    { type:"item", key: "market", icon: 'game-icons:shop', index: "market", titleKey: "game.menu.market" },
    { type:"item", key: "auction", icon: 'mingcute:auction-line', index: "auction", titleKey: "game.menu.auction" },

    // { type: "item", key: "guild", index: "guild", titleKey: "game.menu.guild" },
    { type: "divider", key: "divider-3" },
    { type: "item", key: "battle", icon: 'game-icons:crossed-swords', index: "battle", titleKey: "game.menu.battle" },
    { type: "item", key: "arena", icon: 'game-icons:cat', index: "arena", titleKey: "game.menu.arena" },
    { type: "item", key: "combatTrial", icon: "lucide:target", index: "combatTrial", titleKey: "game.menu.combatTrial" },
    // { type: "item", key: "expedition", index: "expedition", titleKey: "game.menu.expedition" },
    { type: "divider", key: "divider-4" },
    { type: "item", key: "enhance", icon: 'game-icons:upgrade', index: "enhance", titleKey: "game.menu.enhance" },
    { type: "item", key: "affix", icon: 'game-icons:sparkling-sabre', index: "affix", titleKey: "game.menu.affix" },
    { type: "item", key: "alchemy", icon: "game-icons:chemical-drop", index: "alchemy", titleKey: "game.menuGroup.alchemy" },
    // { type: "item", key: "dismantle", index: "dismantle", titleKey: "game.menu.dismantle" },
    // { type: "item", key: "mysterious", index: "mysterious", titleKey: "game.menu.mysterious" },
    { type: "divider", key: "divider-5" },
    { type: "item", key: "fishing", icon: "game-icons:fishing-hook", index: "fishing", titleKey: "action_main_type.fishing.name" },
    { type: "item", key: "pond", icon: "game-icons:fish-eggs", index: "pond", titleKey: "building.fish_pond.name" },
    { type: "divider", key: "divider-5-actions" },
    ...dynamicActionMenu.value,
    // { type: "item", key: "rankingList", index: "rankingList", titleKey: "game.menu.rankingList" },
    // ...(Number(playerStore.catVillageReputation.level || 0) >= 20
    //   ? ([{ type: "item", key: "cheaterList", icon: "material-symbols:gavel-rounded", index: "cheaterList", titleKey: "game.menu.cheaterList" }] as MenuNode[])
    //   : []),
    { type: "divider", key: "divider-6" },
    { type: "item", key: "gameguide", icon: "material-symbols:school-outline-rounded", index: "gameguide", titleKey: "game.menu.gameguide" },
    { type: "item", key: "instructions", icon: 'material-symbols:bookmark-star-outline-rounded', index: "instructions", titleKey: "game.menu.instructions" },
    { type: "item", key: "dictionary", icon: "material-symbols:menu-book-outline-rounded", index: "dictionary", titleKey: "game.menu.dictionary" },
    { type: "item", key: "updateLog", icon: "material-symbols:update-rounded", index: "updateLog", titleKey: "game.menu.updateLog" },
    { type: "item", key: "community", icon: "material-symbols:forum-rounded", index: "community", titleKey: "game.menu.community" },
    { type: "item", key: "feedback", icon: 'material-symbols:support-agent-rounded', index: "feedback", titleKey: "game.menu.feedback" },
    // { type: "item", key: "report", index: "report", titleKey: "game.menu.report" },
    // { type: "item", key: "gm", index: "gm", titleKey: "game.menu.gm" },
    // { type: "item", key: "sponsor", index: "sponsor", titleKey: "game.menu.sponsor" }
  ]
  return nodes
})

const allowedTabSet = computed(() => {
  const tabs = new Set<string>([defaultTab, "selfProfile", "bindEmail", "resetPassword", "switchAccount", "settings"])
  for (const node of menuRegistry.value) {
    if (node.type === "item") tabs.add(node.index)
    if (node.type === "submenu") {
      for (const child of node.children) tabs.add(child.index)
    }
  }
  return tabs
})

const currentTab = ref(defaultTab)
const languageOptions = getLocaleOptions()
const currentPlayerLocale = computed(() => normalizeSupportedLocale(store.playerLanguage))
const fastForwardRewards = ref<{
  completedSteps: number
  itemRewards: Record<string, number>
  itemCosts: Record<string, number>
  attrExp: Record<string, number>
  actorSummaries: Array<{
    actorType: "player" | "kitty"
    actorId: number
    completedSteps: number
    itemRewards: Record<string, number>
    itemCosts: Record<string, number>
    attrExp: Record<string, number>
    ownerAttrExp?: Record<string, number>
  }>
  queueCanceled: Array<{ queueId: number; actionId: string; reason: string }>
  enhanceResults: Array<{ queueId: number; attemptNo: number; beforeLevel: number; afterLevel: number; success: boolean }>
  offlineDurationSeconds: number
  combatItemRewards: Record<string, number>
  fishingCatches: Record<string, FastForwardFishingCatchTally>
  fishingMutations: FishingMutationDefinition[]
}>({
  completedSteps: 0,
  itemRewards: {},
  itemCosts: {},
  attrExp: {},
  actorSummaries: [],
  queueCanceled: [],
  enhanceResults: [],
  offlineDurationSeconds: 0,
  combatItemRewards: {},
  fishingCatches: {},
  fishingMutations: [],
})
const fastForwardCompletedSteps = computed(() => fastForwardRewards.value.completedSteps)
const fastForwardOfflineDurationSeconds = computed(() => fastForwardRewards.value.offlineDurationSeconds)
const fastForwardItemEntries = computed(() =>
  Object.entries(fastForwardRewards.value.itemRewards)
    .filter(([, qty]) => Number(qty) > 0)
    .map(([itemId, quantity]) => ({ itemId, quantity }))
)
const fastForwardCostEntries = computed(() =>
  Object.entries(fastForwardRewards.value.itemCosts)
    .filter(([, qty]) => Number(qty) > 0)
    .map(([itemId, quantity]) => ({ itemId, quantity }))
)
const fastForwardAttrEntries = computed(() =>
  Object.entries(fastForwardRewards.value.attrExp)
    .filter(([, value]) => Number(value) > 0)
    .map(([attrKey, value]) => ({ attrKey, value }))
)
const fastForwardKittyActorEntries = computed(() => {
  const kittyNameByID = new Map<number, string>()
  for (const kitty of playerStore.kitties || []) {
    const kittyID = Math.max(1, Math.floor(Number(kitty.id || 0)))
    if (!kittyID || !Number.isFinite(kittyID)) continue
    kittyNameByID.set(kittyID, kitty.name)
  }
  return (fastForwardRewards.value.actorSummaries || [])
    .filter((row) => normalizeActorType(row.actorType) === "kitty")
    .map((row) => {
      const actorID = Math.max(1, Math.floor(Number(row.actorId || 0)))
      const name = kittyNameByID.get(actorID) || `#${actorID}`
      const itemRewards = Object.entries(row.itemRewards || {})
        .filter(([, qty]) => Number(qty) > 0)
        .map(([itemId, quantity]) => ({ itemId, quantity }))
      const itemCosts = Object.entries(row.itemCosts || {})
        .filter(([, qty]) => Number(qty) > 0)
        .map(([itemId, quantity]) => ({ itemId, quantity }))
      const attrExp = Object.entries(row.attrExp || {})
        .filter(([, value]) => Number(value) > 0)
        .map(([attrKey, value]) => ({ attrKey, value }))
      const ownerAttrExp = Object.entries(row.ownerAttrExp || {}).filter(([, value]) => Number(value) > 0).map(([attrKey, value]) => ({ attrKey, value }))
      return {
        actorID,
        name,
        completedSteps: Math.max(0, Math.floor(Number(row.completedSteps || 0))),
        itemRewards,
        ownerAttrExp,
        itemCosts,
        attrExp
      }
    })
    .filter((row) => row.itemRewards.length > 0 || row.itemCosts.length > 0 || row.attrExp.length > 0 || row.ownerAttrExp.length > 0 || row.completedSteps > 0)
    .sort((a, b) => a.actorID - b.actorID)
})
const fastForwardCanceledEntries = computed(() =>
  fastForwardRewards.value.queueCanceled.map((entry) => ({
    queueId: Number(entry.queueId || 0),
    actionName: resolveActionName(String(entry.actionId || "")),
    reasonLabel: resolveCancelReason(String(entry.reason || ""))
  }))
)
const fastForwardEnhanceEntries = computed(() =>
  fastForwardRewards.value.enhanceResults.map((entry) => ({
    queueId: Number(entry.queueId || 0),
    attemptNo: Number(entry.attemptNo || 0),
    beforeLevel: Number(entry.beforeLevel || 0),
    afterLevel: Number(entry.afterLevel || 0),
    resultLabel: entry.success ? t("game.enhanceSuccess") : t("game.queueStatus.failed")
  }))
)
const fastForwardCombatItemEntries = computed(() =>
  Object.entries(fastForwardRewards.value.combatItemRewards)
    .filter(([, qty]) => Number(qty) > 0)
    .map(([itemId, quantity]) => ({ itemId, quantity }))
)
const fastForwardFishingCatchEntries = computed(() =>
  Object.entries(fastForwardRewards.value.fishingCatches || {})
    .flatMap(([speciesId, raw]) => {
      const landed = Math.max(0, Math.floor(Number(raw?.landed || 0)))
      if (landed <= 0) return []
      const species = getFishSpeciesDisplay(speciesId)
      return [{
        speciesId,
        name: localizedFishText(species?.names, locale.value, speciesId),
        iconPath: species?.iconPath || "/assets/icons/item/fish_catch.png",
        landed,
        kept: Math.max(0, Math.floor(Number(raw?.kept || 0))),
        released: Math.max(0, Math.floor(Number(raw?.released || 0)))
      }]
    })
    .sort((a, b) => b.landed - a.landed || a.name.localeCompare(b.name, locale.value))
)
const fastForwardFishingMutationEntries = computed(() =>
  Object.entries(fastForwardRewards.value.fishingCatches || {})
    .flatMap(([speciesId, raw]) => {
      const species = getFishSpeciesDisplay(speciesId)
      const name = localizedFishText(species?.names, locale.value, speciesId)
      const iconPath = species?.iconPath || "/assets/icons/item/fish_catch.png"
      return normalizedFastForwardMutationGroups(raw?.mutationGroups).map((group) => ({
        key: `${speciesId}:${group.mutations.join("+")}`,
        speciesId,
        name,
        iconPath,
        mutationIds: group.mutations,
        mutations: resolveFishMutations({ mutations: group.mutations }, fastForwardRewards.value.fishingMutations),
        landed: group.landed,
        kept: group.kept,
        released: group.released
      }))
    })
    .sort((a, b) => b.landed - a.landed || a.name.localeCompare(b.name, locale.value) || a.key.localeCompare(b.key))
)
const fastForwardFishingMutationHiddenCount = computed(() =>
  Object.values(fastForwardRewards.value.fishingCatches || {}).reduce((total, raw) => {
    const listed = normalizedFastForwardMutationGroups(raw?.mutationGroups)
      .reduce((sum, group) => sum + group.landed, 0)
    return total + Math.max(0, Math.floor(Number(raw?.mutatedLanded || listed)) - listed)
  }, 0)
)
const fastForwardHasDetails = computed(() =>
  fastForwardCompletedSteps.value > 0
  || fastForwardItemEntries.value.length > 0
  || fastForwardCostEntries.value.length > 0
  || fastForwardAttrEntries.value.length > 0
  || fastForwardKittyActorEntries.value.length > 0
  || fastForwardCanceledEntries.value.length > 0
  || fastForwardEnhanceEntries.value.length > 0
  || fastForwardCombatItemEntries.value.length > 0
  || fastForwardFishingCatchEntries.value.length > 0
  || fastForwardFishingMutationEntries.value.length > 0
)

function clearOfflineRewardsLoading(force = false) {
  if (offlineRewardsLoadingTimer !== null) {
    window.clearTimeout(offlineRewardsLoadingTimer)
    offlineRewardsLoadingTimer = null
  }
  if (!offlineRewardsLoading.value) return
  if (force) {
    offlineRewardsLoading.value = false
    return
  }
  const elapsed = Date.now() - offlineRewardsLoadingStartedAt
  const remain = Math.max(0, 1_000 - elapsed)
  if (remain <= 0) {
    offlineRewardsLoading.value = false
    return
  }
  offlineRewardsLoadingTimer = window.setTimeout(() => {
    offlineRewardsLoadingTimer = null
    offlineRewardsLoading.value = false
  }, remain)
}

function showOfflineRewardsLoading() {
  if (offlineRewardsLoadingTimer !== null) {
    window.clearTimeout(offlineRewardsLoadingTimer)
    offlineRewardsLoadingTimer = null
  }
  if (!offlineRewardsLoading.value) {
    offlineRewardsLoadingStartedAt = Date.now()
  }
  offlineRewardsLoading.value = true
}

function clearOfflineRewardsRequestTimer() {
  if (offlineRewardsRequestTimer !== null) {
    window.clearTimeout(offlineRewardsRequestTimer)
    offlineRewardsRequestTimer = null
  }
}

function completeOfflineRewardsRequest(options: { forceClearLoading?: boolean; preserveLoading?: boolean } = {}) {
  offlineRewardsInFlight.value = false
  offlineRewardsActiveRequestId = ""
  clearOfflineRewardsRequestTimer()
  if (options.preserveLoading) return
  clearOfflineRewardsLoading(Boolean(options.forceClearLoading))
}

function clearOfflineRecoveryPoll() {
  if (offlineRecoveryPollTimer !== null) window.clearTimeout(offlineRecoveryPollTimer)
  offlineRecoveryPollTimer = null
}

function pollOfflineRecovery(delay: number, background = false) {
  clearOfflineRecoveryPoll()
  offlineRecoveryPollTimer = window.setTimeout(() => {
    offlineRecoveryPollTimer = null
    if (wsConnected && wsAuthBound) requestOfflineRewards(background)
    else if (store.token) pollOfflineRecovery(5000, background)
  }, Math.max(250, Math.min(5000, delay)))
}

function wakeOfflineRecovery() {
  if (!offlineRewardsAccumulatedPayload?.taskId || offlineRecoveryCommand.ackTaskId || offlineRecoveryCommand.ackTasks || offlineRewardsInFlight.value) return
  pollOfflineRecovery(250, true)
}

function retryOfflineRecovery() {
  if (!offlineRecoveryRetryTarget.value || offlineRecoveryRetrying.value || offlineRewardsInFlight.value) return
  offlineRecoveryCommand = { ...offlineRecoveryRetryTarget.value }
  offlineRecoveryRetrying.value = true
  // Queue one version-bound command, including across a transport reconnect.
  pollOfflineRecovery(250, true)
}

function requestOfflineRewards(background = false) {
  if (!store.token) return false
  if (offlineRewardsInFlight.value) return false
  clearOfflineRecoveryPoll()
  offlineRewardsInFlight.value = true
  offlineRecoveryGeneration++
  if (!Object.keys(offlineRecoveryCommand).length) {
    offlineRecoveryCommand = { ...pendingOfflineRecoveryAcknowledgement(store.userId) }
  }
  if (!background) showOfflineRewardsLoading()
  offlineRewardsActiveRequestId = send("action.offline.rewards.get", { ...createOfflineFastForwardRequestPayload(), ...offlineRecoverySynced, ...offlineRecoveryCommand })
  clearOfflineRewardsRequestTimer()
  offlineRewardsRequestTimer = window.setTimeout(() => {
    offlineRewardsRequestTimer = null
    offlineRewardsInFlight.value = false
    offlineRewardsActiveRequestId = ""
    offlineRecoveryPaused.value = true
    // A transport timeout never stops observing the durable task.
    pollOfflineRecovery(1500, true)
  }, offlineRewardsRequestTimeoutMS)
  return true
}

function requestEntrySyncBundle() {
  if (!wsAuthBound) return
  if (initialRealtimeSyncDone) return
  initialRealtimeSyncDone = true
  send("game.state.get")
  send("action.queue.get")
  requestOfflineRewards()
  send("presence.ping", {}, { queueIfUnavailable: false })
}

function requestOfflineRecoverySnapshot() {
  if (!initialRealtimeSyncDone) return
  send("game.state.get")
  send("action.queue.get")
}

function finishOfflineRecoverySync() {
  if (visibilityRecoveryPending) {
    void finishVisibilityRecovery()
    return
  }
  requestOfflineRecoverySnapshot()
}

function requestForegroundSyncBundle() {
  if (!wsAuthBound) return
  send("game.state.get")
  send("action.queue.get")
  send("presence.ping", {}, { queueIfUnavailable: false })
}

function stopPresenceHeartbeat() {
  if (presenceTimer !== null) {
    window.clearInterval(presenceTimer)
    presenceTimer = null
  }
}

function clearVisibilityOfflineTimer() {
  if (visibilityOfflineTimer !== null) {
    window.clearTimeout(visibilityOfflineTimer)
    visibilityOfflineTimer = null
  }
}

function clearVisibilityOfflinePromise() {
  visibilityOfflinePromise = null
}

function resetVisibilityHiddenState() {
  visibilityHiddenSinceMs = 0
}

function armVisibilityOfflineTimer(hiddenAtMs?: number) {
  clearVisibilityOfflineTimer()
  const hiddenSince = Number.isFinite(hiddenAtMs || 0) && (hiddenAtMs || 0) > 0
    ? Math.trunc(hiddenAtMs || 0)
    : (visibilityHiddenSinceMs > 0 ? visibilityHiddenSinceMs : Date.now())
  visibilityHiddenSinceMs = hiddenSince
  visibilityOfflineTimer = window.setTimeout(() => {
    visibilityOfflineTimer = null
    if (document.visibilityState !== "hidden" || !store.token) return
    if (presenceStatus.value === "offline") return
    const promise = transitionPresenceStatus("offline")
    visibilityOfflinePromise = promise
    void promise.finally(() => {
      if (visibilityOfflinePromise === promise) {
        clearVisibilityOfflinePromise()
      }
    })
  }, VISIBILITY_OFFLINE_DELAY_MS)
}

function startPresenceHeartbeat() {
  stopPresenceHeartbeat()
  presenceTimer = window.setInterval(() => {
    send("presence.ping", {}, { queueIfUnavailable: false })
  }, 20_000)
}

async function updatePresenceStatus(status: "online" | "offline"): Promise<boolean> {
  if (!store.token) return false
  try {
    await apiPost("/api/player/presence", store.token, { status })
    return true
  } catch (err) {
    if (isAuthExpiredError(err)) {
      handleAuthExpired("game.error.unauthorized")
      return false
    }
    ElMessage.error(resolveErrorMessage(t, err, "game.error.generic"))
    return false
  }
}

async function transitionPresenceStatus(status: "online" | "offline"): Promise<boolean> {
  const transitionID = ++presenceTransitionVersion
  const ok = await updatePresenceStatus(status)
  if (transitionID !== presenceTransitionVersion) return false
  if (!ok) return false
  presenceStatus.value = status
  presenceOfflineFromDisconnect = false
  return true
}

async function beginVisibilityRecovery() {
  if (visibilityRecoveryPending) return true
  visibilityRecoveryPending = true
  const ok = await transitionPresenceStatus("online")
  if (!ok) {
    visibilityRecoveryPending = false
    return false
  }
  if (offlineRewardsInFlight.value) {
    return true
  }
  if (!requestOfflineRewards()) {
    void finishVisibilityRecovery()
  }
  return true
}

async function finishVisibilityRecovery() {
  if (!visibilityRecoveryPending) return
  visibilityRecoveryPending = false
  requestForegroundSyncBundle()
  resetVisibilityHiddenState()
}

async function recoverVisibilityAndSync(): Promise<boolean> {
  if (visibilityRecoveryPending) {
    return true
  }
  const action = resolveVisibilityRecoveryAction({
    presenceStatus: presenceStatus.value,
    presenceStatusSource: presenceOfflineFromDisconnect ? "disconnect" : "server",
    hiddenSinceMs: visibilityHiddenSinceMs,
    nowMs: Date.now(),
    offlineDelayMs: VISIBILITY_OFFLINE_DELAY_MS
  })
  if (action === "online_then_rewards") {
    return beginVisibilityRecovery()
  }
  resetVisibilityHiddenState()
  return false
}

function connectGameWS() {
  if (!gameWSUrl) return
  connectWS(gameWSUrl, {
    onOpen: handleGameWSOpen,
    onClose: handleGameWSClose,
    onError: handleGameWSError
  })
}

function handleGameWSOpen() {
  offlineRecoveryGeneration++
  offlineRecoverySynced = {}
  wsConnected = true
  wsAuthBound = false
  initialRealtimeSyncDone = false
  inventoryStore.$patch({ version: 0 })
  send("auth.bind", { token: store.token })
  if (document.visibilityState === "hidden") {
    stopPresenceHeartbeat()
    if (presenceStatus.value !== "offline") {
      armVisibilityOfflineTimer(visibilityHiddenSinceMs || Date.now())
    }
    return
  }
  startPresenceHeartbeat()
}

function handleGameWSClose() {
  offlineRecoveryGeneration++
  wsConnected = false
  wsAuthBound = false
  initialRealtimeSyncDone = false
  visibilityRecoveryPending = false
  stopPresenceHeartbeat()
  presenceStatus.value = "offline"
  presenceOfflineFromDisconnect = true
  completeOfflineRewardsRequest()
  clearOfflineRecoveryPoll()
}

async function refreshOwnUsernameAfterWSBind() {
  const token = store.token
  const userId = store.userId
  if (!token || userId <= 0) return
  const seq = ++ownProfileSettingsRefreshSeq
  try {
    const settings = await fetchPlayerSettings(token)
    if (seq !== ownProfileSettingsRefreshSeq || token !== store.token || userId !== store.userId) return
    const username = String(settings.username || "").trim()
    if (!username) return
    const applied = localUserStore.applyProfileUsernameSnapshot(userId, username, settings.usernameChangeId)
    if (applied) {
      store.applyRemoteSettings({ username })
    }
    if (userProfileDialogStore.visible && userProfileDialogStore.userId === userId) {
      void localUserStore.fetchProfileByUserId(token, userId, { force: true }).catch((error) => {
        console.warn("[profile] failed to refresh profile after websocket bind", error)
      })
    }
  } catch (error) {
    if (seq !== ownProfileSettingsRefreshSeq || token !== store.token || userId !== store.userId) return
    console.warn("[profile] failed to refresh username after websocket bind", error)
  }
}

function handleGameWSError() {
  // Connection status is surfaced via close + auth/error handlers.
}

async function handleVisibilityChange() {
  if (!store.token) return
  if (document.visibilityState === "hidden") {
    stopPresenceHeartbeat()
    armVisibilityOfflineTimer()
    return
  }
  clearVisibilityOfflineTimer()
  if (!wsConnected) {
    connectGameWS()
    return
  }
  startPresenceHeartbeat()
  const recovering = await recoverVisibilityAndSync()
  if (!recovering) {
    requestForegroundSyncBundle()
  }
}

function handleAuthExpired(messageKey: string) {
  if (handlingAuthExpired) return
  if (!store.token) return
  handlingAuthExpired = true
  completeOfflineRewardsRequest({ forceClearLoading: true })
  ElMessage.error(t(messageKey))
  logout()
}

function onAuthExpiredEvent() {
  handleAuthExpired("game.error.unauthorized")
}

const playerDisplayName = computed(() => {
  if (store.username) return store.username
  if (store.userId > 0) return `${t("game.playerLabel")} #${store.userId}`
  return t("game.playerGuest")
})
const playerInitial = computed(() => {
  const source = store.username || playerDisplayName.value
  return source.slice(0, 1).toUpperCase()
})

onMounted(() => {
  window.addEventListener(AUTH_EXPIRED_EVENT, onAuthExpiredEvent)
  initDarkMode()
  if (!store.token) {
    router.push("/auth")
    return
  }
  showOfflineRewardsLoading()
  ElNotification({
    title: t("game.testBuildNoticeTitle"),
    message: t("game.testBuildNoticeDesc"),
    type: "warning",
    duration: 3000,
    showClose: true
  })
  void initializeGameEntry()
})

async function initializeGameEntry() {
  if (!store.token || gameEntryInitializing.value) return
  const entryToken = store.token
  gameEntryInitializing.value = true
  gameEntryErrorMessage.value = ""
  try {
    const payload = await fetchBootstrapInit(entryToken)
    if (entryToken !== store.token) return
    inventoryStore.applySnapshot(payload.inventory?.items || [])
    operationEffects.hydrate(payload.operationEffects)
    playerStore.applyFullSnapshot((payload.playerStats || {}) as Record<string, unknown>)
    roleStateStore.initializePlayerFromSnapshot((payload.playerStats || {}) as Record<string, unknown>)
    if (Array.isArray(payload.equipment?.items)) {
      equipmentStore.applySnapshot(payload.equipment.items, 0)
      roleStateStore.applyEquipmentSnapshot("player:self", payload.equipment.items, 0)
    }
    chatStore.applyBootstrapSnapshot(payload.chatConfig || {}, payload.chatHistory || {})
    const bootstrapUsernameApplied = localUserStore.applyProfileUsernameSnapshot(
      Number(payload.playerSettings?.userId || 0),
      String(payload.playerSettings?.username || ""),
      Number(payload.playerSettings?.usernameChangeId || 0)
    )
    store.applyRemoteSettings({
      username: bootstrapUsernameApplied ? payload.playerSettings?.username : undefined,
      language: payload.playerSettings?.language,
      theme: payload.playerSettings?.theme,
      menuLayout: payload.playerSettings?.menuLayout,
      simpleMode: payload.playerSettings?.simpleMode,
      emailVerified: payload.playerSettings?.emailVerified
    })
    tutorialStore.hydrate(payload.tutorialProgress)
    await setAppLocale(payload.playerSettings?.language, getLocaleNamespacesForPath(route.path))
    darkModeEnabled.value = payload.playerSettings?.theme === "dark"
    applyDarkModeClass(darkModeEnabled.value)
    presenceStatus.value = payload.playerPresence?.status || "offline"
    presenceOfflineFromDisconnect = false
    handleClientUpdate(payload.systemVersion || {})
    if (payload.recycleConfig) {
      store.applyRecycleConfig(payload.recycleConfig)
    }
    void specializationStore.load().catch(() => undefined)
  } catch (err) {
    if (entryToken !== store.token) return
    if (isAuthExpiredError(err)) {
      handleAuthExpired("game.error.unauthorized")
      return
    }
    completeOfflineRewardsRequest({ forceClearLoading: true })
    gameEntryErrorMessage.value = resolveErrorMessage(t, err, "game.error.generic")
    ElMessage.error(gameEntryErrorMessage.value)
    return
  } finally {
    gameEntryInitializing.value = false
  }
  if (entryToken !== store.token) return
  const wsProtocol = location.protocol === "https:" ? "wss" : "ws"
  gameWSUrl = `${wsProtocol}://${location.host}/ws?token=${encodeURIComponent(entryToken)}`
  registerBinding("game.state.patch", (payload) => playerStore.patchFromWS(payload))
  registerBinding("idle.tick.result", (payload) => playerStore.patchFromWS(payload))
  registerBinding("inventory.instances.updated", (payload) => {
    if (inventoryStore.needsWSAuthoritativeRefresh(payload)) {
      void inventoryStore.refreshFromWSInvalidation(store.token, payload).catch((error) => {
        console.warn("[inventory] authoritative refresh after compact ws event failed", error)
      })
      return
    }
    inventoryStore.patchFromWS(payload)
  })
  registerBinding("player.profile.changed", (payload) => {
    const userId = Math.floor(Number(payload?.userId || 0))
    const username = String(payload?.username || "").trim()
    const changeId = Math.floor(Number(payload?.changeId || 0))
    if (userId <= 0 || !username || changeId <= 0) return
    if (!localUserStore.applyProfileChanged(userId, username, changeId)) return
    if (userId === store.userId) {
      store.applyRemoteSettings({ username })
    }
    if (userProfileDialogStore.visible && userProfileDialogStore.userId === userId && store.token) {
      void localUserStore.fetchProfileByUserId(store.token, userId, { force: true }).catch((error) => {
        console.warn("[profile] failed to refresh changed player profile", error)
      })
    }
  })
  registerBinding("game.wardrobe.patch", (payload) => wardrobeStore.applyPatch(payload as Parameters<typeof wardrobeStore.applyPatch>[0]))
  registerBinding("game.achievements.patch", (payload) => achievementsStore.applyPatch(payload as Parameters<typeof achievementsStore.applyPatch>[0]))
  registerBinding("action.queue.updated", (payload) => realtimeStore.patchActionQueue(payload))
  registerBinding("operation.effects.updated", (payload) => {
    operationEffects.hydrate(payload as OperationEffectProjection)
  })
  registerBinding('game.cattery.furniture', payload => {
    if (entryToken === store.token) useCatteryFurnitureStore().hydrate(entryToken, payload as FurnitureProjection)
  })
  registerBinding("game.fishing.trip", (payload) => fishingStore.applyTripPayload(payload))
  registerBinding("battle.room.self", (payload) => {
    realtimeStore.patchBattleRoomSelf(payload)
    const room = (payload as { room?: { id?: number; roomName?: string } } | null | undefined)?.room
    const roomID = Number(room?.id || 0)
    if (roomID > 0) {
      chatStore.syncBattleRoomChannel(roomID, String(room?.roomName || ""))
    } else {
      chatStore.clearBattleRoomChannels()
    }
  })
  registerBinding("player.attributes.updated", (payload) => {
    playerStore.patchAttributesEvent(payload)
    roleStateStore.patchRoleAttributesPayload(payload)
    realtimeStore.patchAttributes(payload)
  })
  registerBinding("kitty.attributes.updated", (payload) => {
    roleStateStore.patchRoleAttributesPayload(payload)
  })
  registerBinding("kitty.equipment.updated", (payload) => {
    roleStateStore.patchRoleEquipmentPayload(payload)
  })
  registerBinding("action.queue.auto_cancelled", () => {
    ElMessage.warning(t("game.queueAutoCancelledHint"))
  })
  registerBinding("action.offline.recovery.ready", wakeOfflineRecovery)
  registerBinding("battle.run.finished", () => {
    if (Number(offlineRewardsAccumulatedPayload?.combatPendingRuns || 0) > 0) wakeOfflineRecovery()
  })
  registerBinding("action.offline.rewards.result", async (payload) => {
    if (!isOfflineFastForwardResultForRequest(payload, offlineRewardsActiveRequestId)) return
    const generation = offlineRecoveryGeneration
    const reportUserID = store.userId
    const isCurrent = () => generation === offlineRecoveryGeneration && entryToken === store.token && reportUserID === store.userId
    if (isOfflineFastForwardResultFailure(payload)) {
      completeOfflineRewardsRequest({ preserveLoading: true })
      offlineRecoveryPaused.value = true
      pollOfflineRecovery(Number(payload?.retryAfterMS || 1500), true)
      return
    }
    const wasAcknowledging = Boolean(offlineRecoveryCommand.ackTaskId || offlineRecoveryCommand.ackTasks)
    if (wasAcknowledging) {
      await clearOfflineRecoveryAcknowledgement(reportUserID, offlineRecoveryCommand)
      if (!isCurrent()) return
    }
    offlineRecoveryCommand = {}
    const hasMoreDue = Boolean(payload?.hasMoreDue)
    const stalled = isOfflineFastForwardResultStalled(payload)
    offlineRecoveryRetryTarget.value = createOfflineFastForwardRetryPayload(payload)
    offlineRecoveryRetrying.value = false
    const continuing = hasMoreDue && !stalled
    completeOfflineRewardsRequest({ forceClearLoading: !continuing, preserveLoading: continuing })
    const accumulationBase = shouldReplaceOfflineFastForwardAccumulator(offlineRewardsAccumulatedPayload, payload)
      ? null
      : offlineRewardsAccumulatedPayload
    const mergedPayload = mergeOfflineRewardsPayload(accumulationBase, payload)
    offlineRewardsAccumulatedPayload = mergedPayload
    const version = payload?.reportKey || `${payload?.taskId || ""}:${payload?.revision || 0}`
    if (version !== offlineRecoveryLastVersion) {
      offlineRecoveryLastVersion = version
      offlineRecoveryLastProgressAt = Date.now()
      offlineRecoveryPaused.value = false
    }
    if (continuing) {
      if (Date.now() - offlineRecoveryLastProgressAt >= 30_000) {
        offlineRecoveryPaused.value = true
      }
      pollOfflineRecovery(offlineRecoveryPaused.value ? 5000 : Number(payload?.retryAfterMS || 750), true)
      return
    }
    if (stalled) {
      offlineRecoveryPaused.value = true
      showOfflineRewardsLoading()
      pollOfflineRecovery(5000, true)
      return
    }
    offlineRecoveryPaused.value = false
    if (!payload?.taskId && payload?.protocolVersion >= 3) {
      offlineRewardsAccumulatedPayload = null
      if (!wasAcknowledging) finishOfflineRecoverySync()
      return
    }
    realtimeStore.patchFastForwardRewards(mergedPayload)
    const taskID = String(payload?.taskId || "")
    if (taskID && payload?.snapshotSynced && payload?.inventorySync) {
      try {
        await inventoryStore.ensureRecoverySnapshot(entryToken, payload.inventorySync)
      } catch (error) {
        if (!isCurrent()) return
        console.warn("[inventory] recovery snapshot synchronization failed", error)
        offlineRecoveryPaused.value = true
        showOfflineRewardsLoading()
        pollOfflineRecovery(5000, true)
        return
      }
      if (!isCurrent()) return
    }
    if (taskID && payload?.snapshotSynced) offlineRecoverySynced = {
      syncedTaskId: taskID,
      syncedReportKey: payload.reportKey || "",
      syncedRevision: payload.revision,
      syncedCombatPendingRuns: Number(payload?.combatPendingRuns || 0)
    }
    const reportPending = Boolean(taskID && !isOfflineRecoveryReportReady(payload))
    const snapshotKey = `${payload?.reportKey || taskID}:${reportPending ? "pending" : "complete"}`
    if (!taskID || offlineRecoverySnapshotTask !== snapshotKey) {
      finishOfflineRecoverySync()
      offlineRecoverySnapshotTask = snapshotKey
    }
    // Report the complete absence once, not one popup per settlement batch.
    // Late battle receipts may delay the report, never entry into the game.
    if (reportPending) {
      pollOfflineRecovery(5000, true)
      return
    }
    if (taskID) {
      const acknowledgement = await presentOfflineRecoveryReport(reportUserID, mergedPayload, isCurrent, (open) => {
        const reportKey = String(mergedPayload.reportKey || taskID)
        if (open || offlineRecoveryVisibleReport === reportKey) {
          showFastForwardRewards(mergedPayload, open)
          offlineRecoveryVisibleReport = reportKey
        }
        if (open && payload?.combatLegacyLogs) ElMessage.warning(t("game.fastForwardLegacySummary"))
      })
      if (!isCurrent() || !acknowledgement) return
      offlineRecoveryCommand = acknowledgement
      pollOfflineRecovery(wasAcknowledging ? 5000 : 250, true)
    } else {
      showFastForwardRewards(mergedPayload)
      offlineRewardsAccumulatedPayload = null
    }
  })
  registerBinding("auth.bound", () => {
    wsAuthBound = true
    equipmentStore.beginRealtimeSession()
    roleStateStore.beginEquipmentRealtimeSession("player:self")
    void refreshOwnUsernameAfterWSBind()
    if (document.visibilityState === "hidden") return
    void (async () => {
      const recovering = await recoverVisibilityAndSync()
      if (!recovering) {
        requestEntrySyncBundle()
      }
    })()
  })
  registerBinding("equipment.instances.updated", (payload) => {
    if (equipmentStore.needsWSAuthoritativeRefresh(payload)) {
      void equipmentStore.refreshFromWSInvalidation(store.token, payload).then(() => {
        roleStateStore.applyEquipmentSnapshot("player:self", equipmentStore.equippedItems, equipmentStore.equippedVersion)
      }).catch((error) => {
        console.warn("[equipment] authoritative refresh after gateway change failed", error)
      })
      return
    }
    equipmentStore.patchEquippedItems(payload)
    roleStateStore.patchRoleEquipmentPayload(payload)
  })
  registerBinding("equipment.stats.updated", (payload) => equipmentStore.patchStats(payload))
  registerBinding("equipment.enhance.result", (payload) => realtimeStore.appendEnhanceResult(payload))
  registerBinding("player.work_loadout.activated", (payload) => {
    const data = payload as { loadoutName?: string; loadoutId?: string } | null | undefined
    const loadoutName = String(data?.loadoutName || data?.loadoutId || "").trim()
    if (!loadoutName) return
    ElNotification({
      title: t("game.loadoutAutoSwitchNoticeTitle"),
      message: t("game.loadoutAutoSwitchNoticeMessage", { name: loadoutName }),
      type: "success",
      duration: 2600,
      position: "top-right",
      showClose: true
    })
  })
  registerBinding("player.work_loadout.failed", (payload) => {
    const data = payload as { loadoutName?: string; loadoutId?: string; reason?: string } | null | undefined
    const loadoutName = String(data?.loadoutName || data?.loadoutId || "").trim()
    if (!loadoutName) return
    const reason = String(data?.reason || "").trim()
    const reasonKey = `game.workLoadoutFallback.${reason}`
    const reasonText = reason ? (te(reasonKey) ? t(reasonKey) : reason) : ""
    ElNotification({
      title: t("game.loadoutAutoSwitchFailedTitle"),
      message: reasonText
        ? `${t("game.loadoutAutoSwitchFailedMessage", { name: loadoutName })}: ${reasonText}`
        : t("game.loadoutAutoSwitchFailedMessage", { name: loadoutName }),
      type: "warning",
      duration: 4000,
      position: "top-right",
      showClose: true
    })
  })
  registerBinding("chat.message", (payload) => chatStore.appendIncomingMessage(payload))
  registerBinding("chat.global.message", (payload) => chatStore.appendIncomingMessage(payload))
  registerBinding("presence.pong", () => {
    if (document.visibilityState !== "hidden") {
      presenceStatus.value = "online"
      presenceOfflineFromDisconnect = false
    }
  })
  registerBinding("auth.kicked", (payload) => {
    const reason = String((payload as Record<string, unknown>)?.reason || "").trim()
    if (reason === "CONNECTION_LIMIT") {
      handleAuthExpired("game.connectionLimitKicked")
      return
    }
    handleAuthExpired("game.sessionKicked")
  })
  registerBinding("system.client.update", (payload) => {
    handleClientUpdate(payload as ClientUpdatePayload)
  })
  registerBinding("error", (payload) => {
    if (isAuthExpiredError(payload)) {
      handleAuthExpired("game.error.unauthorized")
      return
    }
    // The correlated result event owns request completion. Generic WS errors do
    // not carry a request ID; the timeout remains the fallback if result delivery fails.
    ElMessage.error(resolveErrorMessage(t, payload, "game.error.generic"))
  })
  document.addEventListener("visibilitychange", visibilityChangeListener)
  connectGameWS()
  loadActionCatalog()
  gameEntryReady.value = true
  void maybePromptNewUserOnboarding()
}

function retryGameEntryInitialization() {
  if (!store.token || gameEntryInitializing.value) return
  void initializeGameEntry()
}

async function maybePromptNewUserOnboarding() {
  if (!shouldShowOnboardingPrompt(store.userId) || tutorialStore.isActive) return
  await nextTick()
  if (!shouldShowOnboardingPrompt(store.userId) || tutorialStore.isActive || clientUpdateDialogVisible.value) return
  onboardingPromptHandled = false
  onboardingPromptVisible.value = true
}

async function startOnboardingFromPrompt() {
  if (onboardingPromptHandled) return
  onboardingPromptHandled = true
  clearOnboardingPromptPending(store.userId)
  onboardingPromptVisible.value = false
  await nextTick()
  await tutorialStore.start("onboarding", { ignorePrerequisites: true })
}

function skipOnboardingPrompt(showHint = true) {
  if (onboardingPromptHandled) return
  onboardingPromptHandled = true
  clearOnboardingPromptPending(store.userId)
  onboardingPromptVisible.value = false
  if (showHint) {
    ElMessage.info(t("game.onboardingPrompt.skipHint"))
  }
}

function handleOnboardingPromptClosed() {
  if (onboardingPromptHandled) return
  skipOnboardingPrompt()
}

onUnmounted(() => {
  offlineRecoveryGeneration++
  clearOfflineRecoveryPoll()
  clearThemeSwitchingState()
  document.documentElement.classList.remove("game-simple-mode")
  if (offlineRewardsLoadingTimer !== null) {
    window.clearTimeout(offlineRewardsLoadingTimer)
    offlineRewardsLoadingTimer = null
  }
  clearOfflineRewardsRequestTimer()
  clearMobileIconRailScrollTimer()
  window.removeEventListener(AUTH_EXPIRED_EVENT, onAuthExpiredEvent)
  document.removeEventListener("visibilitychange", visibilityChangeListener)
  clearVisibilityOfflineTimer()
  resetVisibilityHiddenState()
  stopPresenceHeartbeat()
  clearReleaseReloadTimer()
  for (const unbind of wsUnbinders.splice(0)) {
    unbind()
  }
})

watch(
  [() => route.path, allowedTabSet],
  () => {
    const panelPrefix = "/game/panel/"
    const candidate = route.path.startsWith(panelPrefix)
      ? decodeURIComponent(route.path.slice(panelPrefix.length))
      : routeTabMap[route.path] || defaultTab
    const validated = allowedTabSet.value.has(candidate) ? candidate : defaultTab
    currentTab.value = validated
    syncUpdateLogReadStateWithRoute()
  },
  { immediate: true }
)

watch(
  () => store.playerSimpleMode,
  (enabled) => {
    document.documentElement.classList.toggle("game-simple-mode", enabled)
  },
  { immediate: true }
)

function onSelectMenu(tabIndex: string) {
  if (shouldSuppressMobileIconRailSelect()) return
  mobileMenuOpen.value = false
  if (tabIndex === "feedback") {
    window.open(resolveFeedbackUrl(locale.value), "_blank", "noopener,noreferrer")
    return
  }
  if ((tabIndex === "market" || tabIndex === "auction") && !store.emailVerified) {
    ElMessage.warning(t("game.error.emailVerificationRequired"))
    void router.push({ path: "/game/panel/settings", query: { subTab: "email" } })
    return
  }
  updateRouteTab(tabIndex)
}

function updateRouteTab(tabIndex: string) {
  const validated = allowedTabSet.value.has(tabIndex) ? tabIndex : defaultTab
  const targetPath = tabRouteMap[validated] || `/game/panel/${encodeURIComponent(validated)}`
  if (route.path === targetPath) return
  void router.push({ path: targetPath })
}

function logout() {
  offlineRecoveryGeneration++
  completeOfflineRewardsRequest({ forceClearLoading: true })
  clearOfflineRecoveryPoll()
  offlineRecoveryCommand = {}
  offlineRecoverySynced = {}
  offlineRecoveryVisibleReport = ""
  offlineRecoverySnapshotTask = ""
  offlineRecoveryLastVersion = ""
  offlineRecoveryPaused.value = false
  offlineRecoveryRetryTarget.value = null
  offlineRecoveryRetrying.value = false
  offlineRewardsAccumulatedPayload = null
  gameEntryReady.value = false
  gameEntryInitializing.value = false
  gameEntryErrorMessage.value = ""
  wsAuthBound = false
  initialRealtimeSyncDone = false
  visibilityRecoveryPending = false
  clearReleaseReloadTimer()
  inventoryStore.reset()
  playerStore.reset()
  warehouseStore.reset()
  realtimeStore.reset()
  equipmentStore.reset()
  roleStateStore.reset()
  chatStore.reset()
  wardrobeStore.reset()
  achievementsStore.reset()
  specializationStore.clear()
  operationEffects.reset()
  ownProfileSettingsRefreshSeq += 1
  fastForwardDialogVisible.value = false
  clientUpdateDialogVisible.value = false
  currentSystemReleaseId.value = ""
  hasAutoRedirectedToUpdateLog = false
  store.clearAuth()
  router.push("/auth")
}

function updateLogReadStorageKey() {
  if (store.userId <= 0) return ""
  return `${updateLogReadReleasePrefix}:${store.userId}`
}

function getLastReadUpdateLogReleaseId() {
  const key = updateLogReadStorageKey()
  if (!key) return ""
  return String(localStorage.getItem(key) || "").trim()
}

function setLastReadUpdateLogReleaseId(releaseId: string) {
  const key = updateLogReadStorageKey()
  if (!key) return
  const normalized = String(releaseId || "").trim()
  if (!normalized) return
  localStorage.setItem(key, normalized)
}

function tryAutoRedirectToUpdateLog() {
  if (hasAutoRedirectedToUpdateLog || !store.token) return
  const releaseId = currentSystemReleaseId.value.trim()
  if (!releaseId) return
  if (getLastReadUpdateLogReleaseId() === releaseId) return
  hasAutoRedirectedToUpdateLog = true
  if (route.path !== updateLogPanelPath) {
    void router.push({ path: updateLogPanelPath })
  }
}

function syncUpdateLogReadStateWithRoute() {
  if (route.path !== updateLogPanelPath) return
  const releaseId = currentSystemReleaseId.value.trim()
  if (!releaseId) return
  setLastReadUpdateLogReleaseId(releaseId)
}

function registerBinding(type: string, handler: (payload: any) => void) {
  const unbind = bind(type, handler)
  wsUnbinders.push(unbind)
}

function clearReleaseReloadTimer() {
  if (releaseReloadTimer !== null) {
    window.clearTimeout(releaseReloadTimer)
    releaseReloadTimer = null
  }
  if (releaseCountdownTimer !== null) {
    window.clearInterval(releaseCountdownTimer)
    releaseCountdownTimer = null
  }
}

function clearMobileIconRailScrollTimer() {
  if (mobileIconRailScrollTimer !== null) {
    window.clearTimeout(mobileIconRailScrollTimer)
    mobileIconRailScrollTimer = null
  }
}

function isMobileIconRailActive() {
  return isCompactMenuMode.value && window.matchMedia("(max-width: 1023px)").matches
}

function onMobileIconRailPointerDown(event: PointerEvent) {
  if (!isMobileIconRailActive()) return
  clearMobileIconRailScrollTimer()
  mobileIconRailScrolling.value = false
  mobileIconRailPointer = {
    id: event.pointerId,
    startX: event.clientX,
    startY: event.clientY
  }
}

function onMobileIconRailPointerMove(event: PointerEvent) {
  if (!isMobileIconRailActive() || !mobileIconRailPointer || event.pointerId !== mobileIconRailPointer.id) return
  const deltaX = Math.abs(event.clientX - mobileIconRailPointer.startX)
  const deltaY = Math.abs(event.clientY - mobileIconRailPointer.startY)
  if (deltaY >= 8 && deltaY > deltaX) {
    mobileIconRailScrolling.value = true
  }
}

function onMobileIconRailPointerEnd(event: PointerEvent) {
  if (mobileIconRailPointer && event.pointerId !== mobileIconRailPointer.id) return
  mobileIconRailPointer = null
  if (!mobileIconRailScrolling.value) return
  clearMobileIconRailScrollTimer()
  mobileIconRailScrollTimer = window.setTimeout(() => {
    mobileIconRailScrollTimer = null
    mobileIconRailScrolling.value = false
  }, 180)
}

function shouldSuppressMobileIconRailSelect() {
  return isMobileIconRailActive() && mobileIconRailScrolling.value
}

function normalizeUpdateKind(raw: unknown): "soft" | "force" {
  return String(raw || "").toLowerCase() === "force" ? "force" : "soft"
}

function normalizeRecommendedDelay(raw: unknown, fallback = 120): number {
  const parsed = Number(raw)
  if (!Number.isFinite(parsed) || parsed <= 0) return fallback
  if (parsed > 900) return 900
  return Math.floor(parsed)
}

function reloadForClientUpdate() {
  clearReleaseReloadTimer()
  ElMessage.warning(t("game.clientUpdateReloadingNow"))
  window.location.reload()
}

function handleClientUpdate(payload: ClientUpdatePayload | null | undefined) {
  if (!payload || typeof payload !== "object") return
  const releaseId = String(payload.releaseId || "").trim()
  if (!releaseId) return
  currentSystemReleaseId.value = releaseId
  tryAutoRedirectToUpdateLog()
  syncUpdateLogReadStateWithRoute()
  const processedKey = "system:last-release-id"
  const lastReleaseId = localStorage.getItem(processedKey) || ""
  if (lastReleaseId === releaseId) return
  localStorage.setItem(processedKey, releaseId)

  const kind = normalizeUpdateKind(payload.kind)
  const messageKey = String(payload.messageI18nKey || "game.clientUpdateAvailable").trim() || "game.clientUpdateAvailable"
  const recommendedDelay = normalizeRecommendedDelay(payload.recommendedDelaySec, 120)
  let autoReloadDelayMS = recommendedDelay * 1000
  const forceAtMS = Number(payload.forceAtMs || 0)
  if (kind === "force" && forceAtMS > 0) {
    autoReloadDelayMS = Math.max(1_000, forceAtMS - Date.now())
  }
  const autoReloadDelaySec = Math.max(1, Math.ceil(autoReloadDelayMS / 1000))

  clearReleaseReloadTimer()
  clientUpdateDialogVisible.value = true
  clientUpdateMessageKey.value = messageKey
  clientUpdateCountdownSeconds.value = autoReloadDelaySec
  releaseCountdownTimer = window.setInterval(() => {
    if (clientUpdateCountdownSeconds.value <= 1) {
      clientUpdateCountdownSeconds.value = 0
      if (releaseCountdownTimer !== null) {
        window.clearInterval(releaseCountdownTimer)
        releaseCountdownTimer = null
      }
      return
    }
    clientUpdateCountdownSeconds.value -= 1
  }, 1_000)
  releaseReloadTimer = window.setTimeout(() => {
    reloadForClientUpdate()
  }, autoReloadDelayMS)
}

function showFastForwardRewards(payload: any, open = true) {
  if (!payload || typeof payload !== "object") return false
  const steps = Number(payload.completedSteps || 0)
  const offlineDurationSeconds = Number(payload.offlineDurationSeconds || 0)
  const itemRewards = (payload.itemRewards || {}) as Record<string, number>
  const itemCosts = (payload.itemCosts || {}) as Record<string, number>
  const attrExp = (payload.attrExp || {}) as Record<string, number>
  const actorSummaries = (Array.isArray(payload.actorSummaries) ? payload.actorSummaries : []).map((row: any) => ({
    actorType: normalizeActorType(row?.actorType),
    actorId: normalizeActorType(row?.actorType) === "kitty" ? Math.max(1, Math.floor(Number(row?.actorId || 0))) : 0,
    completedSteps: Math.max(0, Math.floor(Number(row?.completedSteps || 0))),
    itemRewards: (row?.itemRewards && typeof row.itemRewards === "object") ? row.itemRewards as Record<string, number> : {},
    itemCosts: (row?.itemCosts && typeof row.itemCosts === "object") ? row.itemCosts as Record<string, number> : {},
    attrExp: (row?.attrExp && typeof row.attrExp === "object") ? row.attrExp as Record<string, number> : {},
    ownerAttrExp: (row?.ownerAttrExp && typeof row.ownerAttrExp === "object") ? row.ownerAttrExp as Record<string, number> : {}
  }))
  const queueCanceled = (Array.isArray(payload.queueCanceled) ? payload.queueCanceled : []) as Array<{ queueId: number; actionId: string; reason: string }>
  const enhanceResults = (Array.isArray(payload.enhanceResults) ? payload.enhanceResults : []) as Array<{
    queueId: number
    attemptNo: number
    beforeLevel: number
    afterLevel: number
    success: boolean
  }>
  const combatItemRewards = (payload.combatItemRewards && typeof payload.combatItemRewards === "object") ? payload.combatItemRewards as Record<string, number> : {}
  const fishingCatches = (payload.fishingCatches && typeof payload.fishingCatches === "object") ? payload.fishingCatches as Record<string, FastForwardFishingCatchTally> : {}
  const fishingMutations = (Array.isArray(payload.fishingMutations) ? payload.fishingMutations : [])
    .filter((row: unknown): row is FishingMutationDefinition => Boolean(row && typeof row === "object" && String((row as { id?: unknown }).id || "").trim()))
  const hasItems = Object.values(itemRewards).some((v) => Number(v) > 0)
  const hasCosts = Object.values(itemCosts).some((v) => Number(v) > 0)
  const hasAttr = Object.values(attrExp).some((v) => Number(v) > 0)
  const hasActorDetails = actorSummaries.some((row: any) => {
    return row.completedSteps > 0
      || Object.values(row.itemRewards || {}).some((v) => Number(v) > 0)
      || Object.values(row.itemCosts || {}).some((v) => Number(v) > 0)
      || Object.values(row.attrExp || {}).some((v) => Number(v) > 0)
  })
  const hasCanceled = queueCanceled.length > 0
  const hasEnhance = enhanceResults.length > 0
  const hasCombatItems = Object.values(combatItemRewards).some((v) => Number(v) > 0)
  const hasFishingCatches = Object.values(fishingCatches).some((row) => Number(row?.landed || 0) > 0)
  if (offlineDurationSeconds < 60 && steps <= 0 && !hasItems && !hasCosts && !hasAttr && !hasActorDetails && !hasCanceled && !hasEnhance && !hasCombatItems && !hasFishingCatches) return false
  fastForwardRewards.value = {
    completedSteps: steps,
    itemRewards,
    itemCosts,
    attrExp,
    actorSummaries,
    queueCanceled,
    enhanceResults,
    offlineDurationSeconds,
    combatItemRewards,
    fishingCatches,
    fishingMutations
  }
  if (open) fastForwardDialogVisible.value = true
  return true
}

function normalizeActorType(raw: unknown): "player" | "kitty" {
  return String(raw || "").trim().toLowerCase() === "kitty" ? "kitty" : "player"
}

function resolveActionName(actionId: string) {
  if (!actionId) return "-"
  const key = `action.${actionId}.name`
  const localized = t(key)
  return localized === key ? actionId : localized
}

function resolveCancelReason(reason: string) {
  if (reason === "COST_INSUFFICIENT") return t("game.fastForwardCancelReasonCost")
  if (reason === "PROTECT_ITEM_INSUFFICIENT") return t("game.enhanceStopReason_PROTECT_ITEM_INSUFFICIENT")
  if (reason === "TARGET_INVALID") return t("game.enhanceStopReason_TARGET_INVALID")
  if (reason === "STATUS_REQUIREMENT_NOT_MET") return t("game.fastForwardCancelReasonStatus")
  if (reason === "BUILDING_REQUIREMENT_NOT_MET") return t("game.fastForwardCancelReasonBuilding")
  return reason || t("game.error.generic")
}

function onUserCommand(command: string | number | object) {
  if (typeof command !== "string") return
  if (command === "logout") {
    logout()
    return
  }
  if (command === "profile") {
    updateRouteTab("settings")
    return
  }
  if (command === "settings") {
    updateRouteTab("settings")
    return
  }
  if (command === "bindEmail") {
    updateRouteTab("settings")
    return
  }
  if (command === "resetPassword") {
    updateRouteTab("settings")
    return
  }
  if (command === "switchAccount") {
    updateRouteTab("switchAccount")
    return
  }
}

function loadActionCatalog() {
  const mainTypes = listActionMainTypeDefinitions() as DynamicActionSeed[]
  const fromCatalog = mainTypes.filter((item) => typeof item.id === "string" && item.id.length > 0)
  if (fromCatalog.length > 0) {
    actionMainTypes.value = [...fromCatalog].sort((a, b) => (a.order || 0) - (b.order || 0))
    return
  }
  const seen = new Set<string>()
  const derived: DynamicActionSeed[] = []
  const actions = listActionDefinitions()
  for (const action of actions) {
    const mainType = action.mainType || action.menu
    if (!mainType || seen.has(mainType)) continue
    seen.add(mainType)
    derived.push({ id: mainType })
  }
  actionMainTypes.value = derived
}

function normalizeMenuIcon(raw: unknown): string | undefined {
  const icon = typeof raw === "string" ? raw.trim() : ""
  return icon || undefined
}

function initDarkMode() {
  darkModeEnabled.value = store.playerTheme === "dark"
  applyDarkModeClass(darkModeEnabled.value)
}

function clearThemeSwitchingState() {
  if (themeTransitionFrame !== null) {
    window.cancelAnimationFrame(themeTransitionFrame)
    themeTransitionFrame = null
  }
  document.documentElement.classList.remove("theme-switching")
}

function applyDarkModeClass(enabled: boolean, suppressTransitions = false) {
  const root = document.documentElement
  if (!suppressTransitions) {
    root.classList.toggle("dark", enabled)
    return
  }
  clearThemeSwitchingState()
  root.classList.add("theme-switching")
  root.classList.toggle("dark", enabled)
  themeTransitionFrame = window.requestAnimationFrame(() => {
    themeTransitionFrame = window.requestAnimationFrame(() => {
      root.classList.remove("theme-switching")
      themeTransitionFrame = null
    })
  })
}

async function onToggleDarkMode(enabled: boolean) {
  if (!store.token) return
  const previous = darkModeEnabled.value
  darkModeEnabled.value = enabled
  applyDarkModeClass(enabled, true)
  store.setPlayerTheme(enabled ? "dark" : "light")
  try {
    await updatePlayerTheme(store.token, enabled ? "dark" : "light")
  } catch (err) {
    darkModeEnabled.value = previous
    applyDarkModeClass(previous, true)
    store.setPlayerTheme(previous ? "dark" : "light")
    ElMessage.error(resolveErrorMessage(t, err, "settings.error.requestFailed"))
  }
}

async function onSelectPlayerLanguage(raw: string) {
  if (!store.token) return
  const nextLocale = normalizeSupportedLocale(raw)
  if (nextLocale === currentPlayerLocale.value) return
  updatingLanguage.value = true
  try {
    await switchAppLocale(nextLocale, {
      namespaces: GameFullLocaleNamespaces,
      persistLocal: true,
      syncRemote: async (locale) => {
        const payload = await updatePlayerLanguage(store.token, locale)
        store.setPlayerLanguage(payload.language)
        return payload.language
      }
    })
  } catch {
    // switchAppLocale already reports the failure and rolls back local state.
  } finally {
    updatingLanguage.value = false
  }
}
</script>

<style scoped>
.fast-forward-special-catches {
  border-top: 1px solid var(--app-border);
  padding-top: 14px;
}

.fast-forward-special-catches__heading {
  display: flex;
  align-items: center;
  gap: 7px;
  margin-bottom: 10px;
  color: #9a6700;
  font-size: 14px;
  font-weight: 700;
}

.fast-forward-special-catches__heading svg {
  font-size: 17px;
}

.fast-forward-special-catches__grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 10px;
}

.fast-forward-special-catch {
  display: grid;
  grid-template-columns: 78px minmax(0, 1fr);
  align-items: center;
  min-width: 0;
  min-height: 94px;
  border: 1px solid color-mix(in srgb, var(--app-border) 78%, #d6a538 22%);
  border-radius: 8px;
  background: color-mix(in srgb, var(--app-bg) 94%, #fff5c7 6%);
  padding: 9px;
}

.fast-forward-special-catch__art {
  position: relative;
  display: grid;
  place-items: center;
  width: 72px;
  min-height: 68px;
}

.fast-forward-special-catch__art > strong {
  position: absolute;
  right: 0;
  bottom: -2px;
  z-index: 5;
  min-width: 25px;
  border: 1px solid rgb(255 255 255 / 75%);
  border-radius: 5px;
  background: #7a5914;
  color: #fff;
  padding: 1px 5px;
  font-size: 11px;
  line-height: 18px;
  text-align: center;
}

.fast-forward-special-catch__body {
  display: flex;
  min-width: 0;
  flex-direction: column;
  gap: 6px;
}

.fast-forward-special-catch__body > strong {
  overflow: hidden;
  font-size: 14px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.fast-forward-special-catch__body > small {
  color: var(--app-muted);
  font-size: 11px;
}

.fast-forward-special-catch__mutations {
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
}

.fast-forward-special-catch__mutations > span {
  display: inline-flex;
  align-items: center;
  gap: 3px;
  border: 1px solid var(--app-border);
  border-radius: 4px;
  padding: 2px 5px;
  font-size: 11px;
  font-weight: 650;
  line-height: 16px;
}

.fast-forward-special-catch__mutations > span.is-prismatic {
  border-color: #61b8c9;
  background: rgb(222 249 250 / 76%);
  color: #176476;
}

.fast-forward-special-catch__mutations > span.is-rainbow {
  border-color: #d276a8;
  background: rgb(255 232 243 / 78%);
  color: #963b70;
}

.fast-forward-special-catch__mutations > span.is-shimmering {
  border-color: #d2a43e;
  background: rgb(255 248 208 / 82%);
  color: #835d08;
}

.fast-forward-special-catches__overflow {
  margin: 8px 0 0;
  color: var(--app-muted);
  font-size: 12px;
}

@media (max-width: 600px) {
  .fast-forward-special-catches__grid {
    grid-template-columns: minmax(0, 1fr);
  }
}
</style>
