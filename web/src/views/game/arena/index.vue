<template>
  <div class="page-shell arena-page-shell">
    <GamePageFrame
      title-key="game.menu.arena"
      icon="game-icons:cat"
      tutorial-id="arena"
      :visual-src="arenaVisual.src"
      :visual-tone="arenaVisual.tone"
      visual-key="arena"
      data-tutorial="arena-page"
    >
      <div class="arena-page" v-loading="overviewLoading">
        <header
          v-if="overview"
          class="arena-command-bar"
          :class="{ 'arena-command-bar--without-refresh': !showHeaderRefresh }"
          data-tutorial="arena-overview"
        >
          <div class="arena-rating-block">
            <span class="arena-command-icon"><Icon icon="lucide:trophy" /></span>
            <div>
              <span>{{ t("game.arena.rating") }}</span>
              <strong>{{ formatNumber(overview.state.rating) }}</strong>
            </div>
          </div>
          <div class="arena-command-stat">
            <span>{{ t("game.arena.attackRecord") }}</span>
            <strong>{{ overview.state.attackWins }} <small>/</small> {{ overview.state.attackLosses }}</strong>
          </div>
          <div class="arena-command-stat">
            <span>{{ t("game.arena.defenseRecord") }}</span>
            <strong>{{ overview.state.defenseWins }} <small>/</small> {{ overview.state.defenseLosses }}</strong>
          </div>
          <div class="arena-command-stat arena-command-stat--bounty">
            <span>{{ t("game.arena.currentBounty") }}</span>
            <strong>{{ formatNumber(overview.defense?.bountyTokens || 0) }} <small>{{ t("game.arena.tokenUnit") }}</small></strong>
          </div>
          <span v-if="showHeaderRefresh" class="arena-command-refresh">
            <el-tooltip :content="headerRefreshLabel" placement="top">
              <span class="arena-command-refresh__trigger">
                <el-button
                  circle
                  :loading="refreshing"
                  :aria-label="headerRefreshLabel"
                  @click="refreshCurrentView"
                >
                  <Icon icon="lucide:refresh-cw" />
                </el-button>
              </span>
            </el-tooltip>
          </span>
        </header>

        <el-tabs v-model="activeTab" class="arena-tabs" data-tutorial="arena-tabs">
          <el-tab-pane name="targets">
            <template #label><span class="arena-tab-label"><Icon icon="lucide:swords" />{{ t("game.arena.targetsTab") }}</span></template>
            <section class="arena-pane" data-tutorial="arena-targets">
              <div class="arena-pane-toolbar">
                <div class="arena-target-filters">
                  <el-segmented v-model="targetScope" :options="targetScopeOptions" />
                </div>
                <span v-if="!overview?.defense?.active" class="arena-inline-notice">
                  <Icon icon="lucide:shield-alert" />{{ t("game.arena.publishRequired") }}
                </span>
                <span v-else-if="!ownDefenseEligibilityCurrent" class="arena-inline-notice">
                  <Icon icon="lucide:shield-alert" />{{ t("game.arena.defensePolicyOutdatedNotice") }}
                </span>
                <el-tooltip v-else :content="challengeEntryPolicyText" placement="top">
                  <span class="arena-entry-status" :class="{ 'is-empty': challengeEntriesRemaining <= 0 }">
                    <Icon :icon="challengeEntriesRemaining > 0 ? 'lucide:gauge' : 'lucide:timer'" />
                    {{ challengeEntryStatusText }}
                    <span class="arena-entry-status__meter" aria-hidden="true">
                      <i
                        v-for="entryIndex in challengeEntryMeterSize"
                        :key="entryIndex"
                        :class="{ 'is-filled': entryIndex <= challengeEntryMeterFilled }"
                      />
                    </span>
                    <Icon class="arena-entry-status__help" icon="lucide:circle-help" />
                  </span>
                </el-tooltip>
              </div>

              <nav class="arena-format-tabs" :aria-label="t('game.arena.formatFilter')" data-tutorial="arena-format-tabs">
                <button
                  type="button"
                  role="tab"
                  :aria-selected="targetFormatFilter === 'all'"
                  :class="{ 'is-active': targetFormatFilter === 'all' }"
                  @click="targetFormatFilter = 'all'"
                >
                  <Icon icon="lucide:layout-list" />
                  <span>{{ t("game.arena.formatAll") }}</span>
                </button>
                <ArenaFormatRulesPopover
                  v-for="format in availableArenaFormats"
                  :key="format.id"
                  :format="format"
                >
                  <button
                    type="button"
                    role="tab"
                    :aria-selected="targetFormatFilter === format.id"
                    :class="{ 'is-active': targetFormatFilter === format.id }"
                    @click="targetFormatFilter = format.id"
                  >
                    <Icon :icon="format.icon" />
                    <span>{{ formatName(format) }}</span>
                  </button>
                </ArenaFormatRulesPopover>
              </nav>

              <div v-if="targetScope === 'suggested'" class="arena-target-list-toolbar">
                <span class="arena-target-list-title">
                  <Icon icon="lucide:sparkles" />
                  {{ t("game.arena.suggested") }}
                </span>
                <el-button
                  type="primary"
                  plain
                  :loading="refreshing"
                  :disabled="recommendationRefreshDisabled"
                  :aria-label="recommendationRefreshLabel"
                  @click="refreshRecommendedTargets"
                >
                  <Icon v-if="!refreshing" icon="lucide:refresh-cw" />
                  {{ recommendationRefreshLabel }}
                </el-button>
              </div>

              <div v-loading="targetsLoading" class="arena-target-grid">
                <el-empty v-if="!targetsLoading && targets.length === 0" :description="t('game.arena.targetsEmpty')" />
                <article v-for="target in targets" :key="target.userId" class="arena-target">
                  <header class="arena-target__header">
                    <div class="arena-target__identity">
                      <span class="arena-target__avatar"><Icon icon="game-icons:cat" /></span>
                      <div>
                        <h2>{{ target.username }}</h2>
                        <span>{{ t("game.arena.ratingValue", { value: formatNumber(target.rating) }) }}</span>
                        <span class="arena-target__rating-rule" :class="{ 'is-rated': target.ratingEligible }">
                          <Icon :icon="target.ratingEligible ? 'lucide:trophy' : 'lucide:swords'" />
                          {{ targetRatingRuleLabel(target) }}
                        </span>
                        <ArenaFormatRulesPopover :format="target.format">
                          <span class="arena-format-marker" :class="{ 'arena-format-marker--standard': target.format.standard }">
                            <Icon :icon="target.format.icon" />
                            {{ formatName(target.format) }}
                          </span>
                        </ArenaFormatRulesPopover>
                        <span class="arena-target__signals">
                          <el-tooltip :content="t('game.arena.successfulDefenses', { count: target.successfulDefenses })" placement="top">
                            <span><Icon icon="lucide:shield-check" />{{ target.successfulDefenses }}</span>
                          </el-tooltip>
                          <el-tooltip :content="t('game.arena.challengeCount', { count: target.challengeCount })" placement="top">
                            <span><Icon icon="lucide:history" />{{ target.challengeCount }}</span>
                          </el-tooltip>
                        </span>
                      </div>
                    </div>
                    <div class="arena-target__bounty">
                      <span>{{ t("game.arena.bounty") }}</span>
                      <strong>+{{ formatNumber(target.bountyTokens) }}</strong>
                    </div>
                  </header>

                  <div class="arena-lineup-board" :aria-label="t('game.arena.defenseLineup')">
                    <div
                      v-for="lane in lineupLanes(target.lineup)"
                      :key="`${target.userId}:${lane.position}`"
                      class="arena-lineup-lane"
                      :class="{ 'arena-lineup-lane--empty': lane.units.length === 0 }"
                    >
                      <span class="arena-lineup-lane__position">{{ positionLabel(lane.position) }}</span>
                      <div v-if="lane.units.length > 0" class="arena-lineup-lane__members">
                        <div v-for="unit in lane.units" :key="`${unit.unitType}:${unit.kittyId || unit.name}`" class="arena-lineup-member">
                          <span class="arena-lineup-lane__token">
                            <Icon :icon="unit.unitType === 'player' ? 'lucide:user' : 'game-icons:cat'" />
                          </span>
                          <span class="arena-lineup-member__copy">
                            <strong>{{ unit.name }}</strong>
                            <small>{{ t("game.arena.battleLevelShort", { level: unit.battleLevel }) }}</small>
                          </span>
                          <span class="arena-lineup-lane__hp">
                            <Icon icon="lucide:heart" />{{ formatNumber(unit.maxHp) }}
                          </span>
                          <span class="arena-lineup-lane__skills">
                            <SkillQuickRef
                              v-for="skillID in visibleArenaSkills(unit)"
                              :key="skillID"
                              :skill-id="skillID"
                            />
                            <small v-if="hiddenArenaSkillCount(unit) > 0">+{{ hiddenArenaSkillCount(unit) }}</small>
                          </span>
                        </div>
                      </div>
                      <div v-else class="arena-lineup-lane__empty">
                        <span class="arena-lineup-lane__token"><Icon icon="lucide:minus" /></span>
                        <strong>{{ t("game.arena.emptyPosition") }}</strong>
                      </div>
                    </div>
                  </div>

                  <footer class="arena-target__footer">
                    <div>
                      <el-tooltip :content="t('game.arena.tokensHelp')" placement="top">
                        <span class="arena-help-label">{{ t("game.arena.winReward") }}<Icon icon="lucide:circle-help" /></span>
                      </el-tooltip>
                      <strong>{{ formatNumber(target.estimatedRewardTokens) }} <small>{{ t("game.arena.tokenUnit") }}</small></strong>
                    </div>
                    <el-button
                      type="primary"
                      :loading="challengingID === target.userId"
                      :disabled="!ownDefenseEligible || challengingID > 0 || isTargetUnavailable(target)"
                      @click="challengeTarget(target)"
                    >
                      <Icon :icon="isTargetUnavailable(target) ? 'lucide:timer' : 'lucide:swords'" />
                      {{ challengeButtonLabel(target) }}
                    </el-button>
                  </footer>
                </article>
              </div>

              <div v-if="targetScope === 'more' && targetsHasMore" class="arena-target-load-more">
                <el-button :loading="targetsLoadingMore" :disabled="targetsLoadingMore" @click="loadMoreTargets">
                  <Icon icon="lucide:chevrons-down" />
                  {{ t("game.arena.loadMoreTeams") }}
                </el-button>
              </div>

            </section>
          </el-tab-pane>

          <el-tab-pane name="defense">
            <template #label><span class="arena-tab-label"><Icon icon="lucide:shield" />{{ t("game.arena.defenseTab") }}</span></template>
            <section class="arena-pane arena-defense-pane" data-tutorial="arena-defense">
              <header class="arena-defense-status">
                <div>
                  <span class="arena-kind-mark"><Icon icon="game-icons:cat" />{{ t("game.arena.kind") }}</span>
                  <div>
                    <h2>{{ overview?.defense?.active ? t("game.arena.defensePublished") : t("game.arena.defenseUnpublished") }}</h2>
                    <span>{{ t("game.arena.formationCount", { count: formationUnitCount }) }}</span>
                  </div>
                </div>
                <div v-if="overview?.defense" class="arena-defense-counters">
                  <ArenaFormatRulesPopover :format="overview.defense.format">
                    <span class="arena-format-marker" :class="{ 'arena-format-marker--standard': overview.defense.format.standard }">
                      <Icon :icon="overview.defense.format.icon" />
                      {{ formatName(overview.defense.format) }}
                    </span>
                  </ArenaFormatRulesPopover>
                  <span><Icon icon="lucide:shield-check" />{{ t("game.arena.successfulDefenses", { count: overview.defense.successfulDefenses }) }}</span>
                  <span><Icon icon="lucide:history" />{{ t("game.arena.challengeCount", { count: overview.defense.challengeCount }) }}</span>
                </div>
              </header>

              <div v-if="overview?.defense?.active && !ownDefenseEligibilityCurrent" class="arena-inline-notice arena-defense-policy-notice">
                <Icon icon="lucide:shield-alert" />{{ t("game.arena.defensePolicyOutdatedNotice") }}
              </div>

              <div class="arena-defense-workspace">
                <section class="arena-formation-preview" data-tutorial="arena-formation-preview">
                  <header>
                    <span>{{ t("game.arena.formationPreview") }}</span>
                    <strong>{{ formationUnitCount }} / 3</strong>
                  </header>
                  <div class="arena-preview-lanes">
                    <div
                      v-for="lane in formationLanes"
                      :key="lane.position"
                      class="arena-preview-lane"
                      :class="{ 'arena-preview-lane--empty': lane.units.length === 0 }"
                    >
                      <span>{{ positionLabel(lane.position) }}</span>
                      <div v-if="lane.units.length > 0" class="arena-preview-lane__members">
                        <div v-for="unit in lane.units" :key="`${unit.unitType}:${unit.name}`" class="arena-preview-member">
                          <span class="arena-preview-lane__token">
                            <Icon :icon="unit.unitType === 'player' ? 'lucide:user' : 'game-icons:cat'" />
                          </span>
                          <span>
                            <strong>{{ unit.name }}</strong>
                            <small>{{ unit.unitType === "player" ? t("game.arena.player") : t("game.arena.kitty") }}</small>
                          </span>
                        </div>
                      </div>
                      <div v-else class="arena-preview-lane__empty">
                        <span class="arena-preview-lane__token"><Icon icon="lucide:plus" /></span>
                        <strong>{{ t("game.arena.emptyPosition") }}</strong>
                      </div>
                    </div>
                  </div>
                </section>

                <div class="arena-formation-editor" data-tutorial="arena-formation-editor">
                  <div class="arena-formation-row">
                    <div class="arena-formation-row__identity">
                      <span><Icon icon="lucide:user" /></span>
                      <div>
                        <strong>{{ playerDisplayName }}</strong>
                        <small>{{ t("game.arena.player") }}</small>
                      </div>
                    </div>
                    <el-select
                      v-model="selection.playerPosition"
                      class="arena-formation-row__player-position"
                      :aria-label="t('game.arena.playerPosition')"
                    >
                      <el-option
                        v-for="option in positionOptions"
                        :key="option.value"
                        :label="option.label"
                        :value="option.value"
                      />
                    </el-select>
                  </div>

                  <div v-for="index in [0, 1]" :key="index" class="arena-formation-row">
                    <div class="arena-formation-row__identity">
                      <span><Icon icon="game-icons:cat" /></span>
                      <div>
                        <strong>{{ selectedKitty(index)?.name || t("game.arena.kittySlot", { slot: index + 1 }) }}</strong>
                        <small>{{ selectedKitty(index) ? t("game.arena.kitty") : t("game.arena.optionalSlot") }}</small>
                      </div>
                    </div>
                    <el-select
                      v-model="selection.kitties[index].kittyId"
                      class="arena-formation-row__unit-select"
                      filterable
                      :placeholder="t('game.arena.selectKitty')"
                      @change="handleKittyChange(index)"
                    >
                      <el-option :label="t('game.arena.notDeployed')" :value="0" />
                      <el-option
                        v-for="kitty in overview?.kitties || []"
                        :key="kitty.id"
                        :label="kitty.name"
                        :value="kitty.id"
                        :disabled="kittyUsedByOther(kitty.id, index)"
                      />
                    </el-select>
                    <el-select
                      v-model="selection.kitties[index].position"
                      class="arena-formation-row__position-select"
                      :disabled="selection.kitties[index].kittyId <= 0 || Boolean(selectedKitty(index)?.positionLocked)"
                      :aria-label="t('game.arena.kittyPosition', { slot: index + 1 })"
                    >
                      <el-option
                        v-for="option in positionOptions"
                        :key="option.value"
                        :label="option.label"
                        :value="option.value"
                      />
                    </el-select>
                    <el-tooltip v-if="selectedKitty(index)?.positionLocked" :content="t('game.arena.positionLocked')" placement="top">
                      <span class="arena-position-lock" :aria-label="t('game.arena.positionLocked')"><Icon icon="lucide:lock-keyhole" /></span>
                    </el-tooltip>
                  </div>
                </div>
              </div>

              <section class="arena-format-policy" data-tutorial="arena-format-policy">
                <header>
                  <span class="arena-format-policy__icon"><Icon icon="lucide:party-popper" /></span>
                  <div>
                    <strong>{{ t("game.arena.formatPolicy") }}</strong>
                    <small>{{ t("game.arena.formatPolicyHint") }}</small>
                  </div>
                </header>
                <div class="arena-format-options" role="radiogroup" :aria-label="t('game.arena.formatPolicy')">
                  <ArenaFormatRulesPopover
                    v-for="format in availableArenaFormats"
                    :key="format.id"
                    :format="format"
                  >
                    <button
                      type="button"
                      role="radio"
                      :aria-checked="selectedFormatId === format.id"
                      :aria-label="`${formatName(format)}。${formatShortDescription(format)}`"
                      :class="{ 'is-selected': selectedFormatId === format.id }"
                      @click="selection.formatId = format.id"
                    >
                      <span class="arena-format-option__icon"><Icon :icon="format.icon" /></span>
                      <span class="arena-format-option__copy">
                        <strong>{{ formatName(format) }}</strong>
                        <small>{{ formatShortDescription(format) }}</small>
                      </span>
                      <Icon
                        class="arena-format-option__state"
                        :icon="selectedFormatId === format.id ? 'lucide:check' : 'lucide:info'"
                      />
                    </button>
                  </ArenaFormatRulesPopover>
                </div>
                <div v-if="selectedArenaFormat" class="arena-format-touch-rules" aria-live="polite">
                  <ArenaFormatRulesContent :format="selectedArenaFormat" />
                </div>
              </section>

              <footer class="arena-defense-actions">
                <span>{{ t("game.arena.formationCount", { count: formationUnitCount }) }}</span>
                <div>
                  <el-button
                    v-if="overview?.defense?.active"
                    :loading="unpublishing"
                    :disabled="publishing"
                    @click="unpublishDefense"
                  >
                    <Icon icon="lucide:shield-off" />{{ t("game.arena.unpublish") }}
                  </el-button>
                  <el-button
                    type="primary"
                    data-tutorial="arena-defense-submit"
                    :loading="publishing"
                    :disabled="!selectionValid || unpublishing"
                    @click="publishDefense"
                  >
                    <Icon icon="lucide:shield-check" />{{ overview?.defense?.active ? t("game.arena.updateDefense") : t("game.arena.publishDefense") }}
                  </el-button>
                </div>
              </footer>
            </section>
          </el-tab-pane>

          <el-tab-pane name="records">
            <template #label><span class="arena-tab-label"><Icon icon="lucide:scroll-text" />{{ t("game.arena.recordsTab") }}</span></template>
            <section class="arena-pane" data-tutorial="arena-records">
              <div class="arena-pane-toolbar">
                <el-segmented v-model="historyRole" :options="historyRoleOptions" />
                <span class="arena-history-limit">
                  {{ t("game.arena.historyCount", { count: history.length, limit: overview?.rules.historyKeepCount || 50 }) }}
                </span>
              </div>

              <div v-loading="historyLoading" class="arena-history-list">
                <el-empty v-if="!historyLoading && history.length === 0" :description="t('game.arena.historyEmpty')" />
                <article v-for="challenge in history" :key="challenge.id" class="arena-history-row">
                  <div class="arena-history-row__marker"><Icon icon="game-icons:cat" /></div>
                  <div class="arena-history-row__main">
                    <div class="arena-history-row__title">
                      <strong>{{ challenge.opponentName || t("game.arena.unknownOpponent") }}</strong>
                      <el-tag size="small" effect="plain">{{ roleLabel(challenge.role) }}</el-tag>
                      <el-tag size="small" :type="historyStatusType(challenge)" effect="dark">{{ historyStatusLabel(challenge) }}</el-tag>
                      <ArenaFormatRulesPopover :format="challenge.format">
                        <span class="arena-format-marker" :class="{ 'arena-format-marker--standard': challenge.format.standard }">
                          <Icon :icon="challenge.format.icon" />
                          {{ formatName(challenge.format) }}
                        </span>
                      </ArenaFormatRulesPopover>
                    </div>
                    <div class="arena-history-row__meta">
                      <span>#{{ challenge.id }}</span>
                      <span>{{ formatDate(challenge.createdAt) }}</span>
                    </div>
                  </div>
                  <div class="arena-history-row__settlement">
                    <span v-if="ownReward(challenge) !== 0">{{ t("game.arena.rewardValue", { value: signedValue(ownReward(challenge)) }) }}</span>
                    <span v-if="challenge.ratingEligible">{{ t("game.arena.ratingDelta", { value: signedValue(ownRatingDelta(challenge)) }) }}</span>
                    <span v-else>{{ t("game.arena.unratedSettlement") }}</span>
                  </div>
                  <div class="arena-history-row__actions">
                    <el-tooltip :content="t('game.arena.watchReplay')" placement="top">
                      <el-button
                        circle
                        :disabled="!replayAvailable(challenge)"
                        :aria-label="t('game.arena.watchReplay')"
                        @click="openReplay(challenge.id)"
                      >
                        <Icon icon="lucide:film" />
                      </el-button>
                    </el-tooltip>
                    <el-tooltip :content="t('game.arena.shareReplay')" placement="top">
                      <el-button
                        circle
                        :loading="sharingChallengeID === challenge.id"
                        :disabled="!replayAvailable(challenge) || sharingChallengeID > 0"
                        :aria-label="t('game.arena.shareReplay')"
                        @click="shareReplay(challenge)"
                      >
                        <Icon icon="lucide:message-square-share" />
                      </el-button>
                    </el-tooltip>
                  </div>
                </article>
              </div>
            </section>
          </el-tab-pane>

          <el-tab-pane name="leaderboard">
            <template #label><span class="arena-tab-label"><Icon icon="lucide:chart-no-axes-column-increasing" />{{ t("game.arena.leaderboardTab") }}</span></template>
            <ArenaLeaderboard
              :active="activeTab === 'leaderboard'"
              :refresh-version="leaderboardRefreshVersion"
            />
          </el-tab-pane>

          <el-tab-pane name="shop">
            <template #label><span class="arena-tab-label"><Icon icon="lucide:store" />{{ t("game.arena.shopTab") }}</span></template>
            <section class="arena-pane arena-shop-pane" data-tutorial="arena-shop">
              <header class="arena-shop-header">
                <div class="arena-shop-balance">
                  <span class="arena-shop-balance__icon"><Icon icon="lucide:ticket" /></span>
                  <div>
                    <el-tooltip :content="t('game.arena.tokensHelp')" placement="top">
                      <span class="arena-help-label">{{ t("game.arena.tokens") }}<Icon icon="lucide:circle-help" /></span>
                    </el-tooltip>
                    <strong>{{ formatNumber(shop?.tokens ?? overview?.state.tokens ?? 0) }}</strong>
                  </div>
                </div>
                <span><Icon icon="lucide:shopping-bag" />{{ t("game.arena.shop") }}</span>
              </header>

              <div v-loading="shopLoading" class="arena-shop-list">
                <div v-if="!shopLoading && (shop?.offers || []).length === 0" class="arena-shop-empty">
                  <span><Icon icon="lucide:store" /></span>
                  <strong>{{ t("game.arena.shopPreparing") }}</strong>
                  <small>{{ t("game.arena.shopPreparingHint") }}</small>
                </div>
                <article v-for="offer in shop?.offers || []" :key="offer.id" class="arena-shop-offer">
                  <ItemQuickRef :item-id="offer.itemId" :count="offer.quantity" />
                  <div class="arena-shop-offer__cost">
                    <span>{{ t("game.arena.cost") }}</span>
                    <strong>{{ formatNumber(offer.cost) }}</strong>
                  </div>
                  <el-button
                    type="primary"
                    :loading="buyingOfferID === offer.id"
                    :disabled="buyingOfferID !== '' || Number(shop?.tokens || 0) < offer.cost"
                    @click="buyOffer(offer)"
                  >
                    <Icon icon="lucide:shopping-cart" />{{ t("game.arena.buy") }}
                  </el-button>
                </article>
              </div>
            </section>
          </el-tab-pane>
        </el-tabs>
      </div>
    </GamePageFrame>
  </div>
</template>

<script setup lang="ts">
import { computed, h, onBeforeUnmount, onMounted, reactive, ref, watch } from "vue"
import { Icon } from "@iconify/vue"
import { ElMessage, ElMessageBox } from "element-plus"
import { useI18n } from "vue-i18n"
import GamePageFrame from "@components/game/common/GamePageFrame.vue"
import ItemQuickRef from "@components/game/item/QuickRef.vue"
import SkillQuickRef from "@components/game/skill/SkillQuickRef.vue"
import ArenaLeaderboard from "./ArenaLeaderboard.vue"
import ArenaFormatRulesContent from "./ArenaFormatRulesContent.vue"
import ArenaFormatRulesPopover from "./ArenaFormatRulesPopover.vue"
import { APIClientError, apiGet, apiPost, createIdempotencyKey } from "@services/api"
import {
  activeArenaKittySelections,
  arenaAlchemyGatePayloadFromError,
  arenaCooldownSecondsRemaining,
  formatArenaCooldown,
  isArenaChallengeTerminal,
  isArenaLineupSelectionValid,
  projectArenaChallengeEntries,
  type ArenaChallenge,
  type ArenaChallengeCreatePayload,
  type ArenaChallengeEntries,
  type ArenaChallengeRole,
  type ArenaFormat,
  type ArenaHistoryPayload,
  type ArenaKitty,
  type ArenaLineupSelection,
  type ArenaLineupUnit,
  type ArenaOverview,
  type ArenaPosition,
  type ArenaRecommendationRefresh,
  type ArenaRecommendationRefreshPayload,
  type ArenaReplaySharePayload,
  type ArenaShopOffer,
  type ArenaShopPayload,
  type ArenaTarget,
  type ArenaTargetsPayload
} from "@services/arena"
import { formatAttributeValueByKey } from "@services/attribute-display"
import { resolveErrorMessage } from "@services/error-messages"
import { gamePageVisuals } from "@services/game-page-visuals"
import { getLocalizedItemName } from "@services/item-label"
import {
  cancelPreparedArenaReplay,
  openArenaReplay,
  prepareArenaReplayPlayback
} from "@services/arena-replay-viewer"
import { useChatStore } from "@stores/chat"
import { useGameStore } from "@stores/game"
import { formatNumberWithThousands } from "@utils/number-format"

type ArenaTab = "targets" | "defense" | "records" | "leaderboard" | "shop"
type TargetScope = "suggested" | "more"
type TargetFormatFilter = "all" | string
type FormationPreviewUnit = { unitType: "player" | "kitty"; name: string }

const arenaPositions: ArenaPosition[] = ["vanguard", "midguard", "rearguard"]

const { t, locale } = useI18n()
const store = useGameStore()
const chatStore = useChatStore()
const arenaVisual = gamePageVisuals.arena
const activeTab = ref<ArenaTab>("targets")
const overview = ref<ArenaOverview | null>(null)
const overviewLoading = ref(true)
const refreshing = ref(false)
const targets = ref<ArenaTarget[]>([])
const targetsLoading = ref(false)
const targetsLoadingMore = ref(false)
const targetsHasMore = ref(false)
const targetsNextOffset = ref(0)
const recommendationRefresh = ref<ArenaRecommendationRefresh | null>(null)
const targetScope = ref<TargetScope>("suggested")
const targetFormatFilter = ref<TargetFormatFilter>("all")
const challengingID = ref(0)
const history = ref<ArenaChallenge[]>([])
const historyLoading = ref(false)
const historyRole = ref<ArenaChallengeRole>("all")
const shop = ref<ArenaShopPayload | null>(null)
const shopLoading = ref(false)
const buyingOfferID = ref("")
const publishing = ref(false)
const unpublishing = ref(false)
const sharingChallengeID = ref(0)
const leaderboardRefreshVersion = ref(0)
const editorHydrated = ref(false)
let historyPollTimer: ReturnType<typeof setTimeout> | null = null
let arenaClockTimer: ReturnType<typeof setInterval> | null = null
let targetsReloadQueued = false
const arenaClockOffsetMs = ref(0)
const arenaClockTickMs = ref(Date.now())

const selection = reactive<ArenaLineupSelection>({
  playerPosition: "vanguard",
  formatId: "standard",
  kitties: [
    { kittyId: 0, position: "midguard" },
    { kittyId: 0, position: "rearguard" }
  ]
})

const targetScopeOptions = computed(() => [
  { label: t("game.arena.suggested"), value: "suggested" },
  { label: t("game.arena.moreTeams"), value: "more" }
])
const availableArenaFormats = computed<ArenaFormat[]>(() => {
  return Array.isArray(overview.value?.formats) ? overview.value?.formats || [] : []
})
const selectedFormatId = computed(() => {
  const value = String(selection.formatId || "").trim().toLowerCase()
  if (availableArenaFormats.value.some((format) => format.id === value)) return value
  return availableArenaFormats.value.find((format) => format.id === "standard")?.id || availableArenaFormats.value[0]?.id || ""
})
const selectedArenaFormat = computed(() => availableArenaFormats.value.find((format) => format.id === selectedFormatId.value))
const historyRoleOptions = computed(() => [
  { label: t("game.arena.historyAll"), value: "all" },
  { label: t("game.arena.historyAttack"), value: "attack" },
  { label: t("game.arena.historyDefense"), value: "defense" }
])
const positionOptions = computed(() => (["vanguard", "midguard", "rearguard"] as ArenaPosition[]).map((value) => ({
  value,
  label: positionLabel(value)
})))
const selectionValid = computed(() => isArenaLineupSelectionValid(selection))
const ownDefenseEligibilityCurrent = computed(() => overview.value?.defense?.eligibilityPolicyCurrent !== false)
const ownDefenseEligible = computed(() => Boolean(overview.value?.defense?.active) && ownDefenseEligibilityCurrent.value)
const formationUnitCount = computed(() => 1 + activeArenaKittySelections(selection).length)
const playerDisplayName = computed(() => store.username || t("game.arena.currentPlayer"))
const formationLanes = computed(() => {
  const units: Array<{ position: ArenaPosition; unit: FormationPreviewUnit }> = [{
    position: selection.playerPosition,
    unit: { unitType: "player", name: playerDisplayName.value }
  }]
  for (let index = 0; index < selection.kitties.length; index += 1) {
    const kitty = selectedKitty(index)
    if (!kitty) continue
    units.push({
      position: selection.kitties[index].position,
      unit: { unitType: "kitty", name: kitty.name }
    })
  }
  return arenaPositions.map((position) => ({
    position,
    units: units.filter((entry) => entry.position === position).map((entry) => entry.unit)
  }))
})
const arenaNowMs = computed(() => arenaClockTickMs.value + arenaClockOffsetMs.value)
const recommendationRefreshSecondsRemaining = computed(() => arenaCooldownSecondsRemaining(
  recommendationRefresh.value?.availableAt,
  arenaNowMs.value
))
const showHeaderRefresh = computed(() => activeTab.value !== "targets")
const headerRefreshLabel = computed(() => t("game.arena.refresh"))
const recommendationRefreshDisabled = computed(() => (
  refreshing.value ||
  targetsLoading.value ||
  targetScope.value !== "suggested" ||
  recommendationRefreshSecondsRemaining.value > 0
))
const recommendationRefreshLabel = computed(() => {
  if (recommendationRefreshSecondsRemaining.value > 0) {
    return t("game.arena.refreshRecommendationsCooldown", {
      time: formatArenaCooldown(recommendationRefreshSecondsRemaining.value)
    })
  }
  return t("game.arena.refreshRecommendations")
})
const challengeEntryProjection = computed(() => projectArenaChallengeEntries(overview.value?.challengeEntries, arenaNowMs.value))
const challengeEntriesRemaining = computed(() => challengeEntryProjection.value.remaining)
const challengeEntryCapacity = computed(() => challengeEntryProjection.value.capacity)
const challengeEntrySecondsToNext = computed(() => challengeEntryProjection.value.secondsToNext)
const challengeEntryMeterSize = computed(() => Math.min(10, challengeEntryCapacity.value))
const challengeEntryMeterFilled = computed(() => challengeEntryCapacity.value <= 0
  ? 0
  : Math.ceil(challengeEntriesRemaining.value / challengeEntryCapacity.value * challengeEntryMeterSize.value))
const challengeEntryPolicyText = computed(() => t("game.arena.challengeEntryPolicy", {
  recovery: formatArenaCooldown(overview.value?.challengeEntries.recoverySeconds || 0),
  capacity: challengeEntryCapacity.value,
  opponent: formatArenaCooldown(overview.value?.challengeEntries.opponentCooldownSeconds || 0)
}))
const challengeEntryStatusText = computed(() => {
  if (challengeEntriesRemaining.value <= 0) {
    return t("game.arena.challengeEntriesEmpty", { time: formatArenaCooldown(challengeEntrySecondsToNext.value) })
  }
  if (challengeEntriesRemaining.value < challengeEntryCapacity.value) {
    return t("game.arena.challengeEntriesRecovering", {
      remaining: challengeEntriesRemaining.value,
      capacity: challengeEntryCapacity.value,
      time: formatArenaCooldown(challengeEntrySecondsToNext.value)
    })
  }
  return t("game.arena.challengeEntriesReady", {
    remaining: challengeEntriesRemaining.value,
    capacity: challengeEntryCapacity.value
  })
})

watch(targetScope, () => void loadTargets())
watch(targetFormatFilter, () => void loadTargets())
watch(historyRole, () => void loadHistory())
watch(activeTab, (tab) => {
  if (tab === "targets" && targets.value.length === 0) void loadTargets()
  if (tab === "records") void loadHistory()
  if (tab === "shop") void loadShop()
})

onMounted(async () => {
  store.restoreToken()
  arenaClockTimer = setInterval(() => {
    arenaClockTickMs.value = Date.now()
  }, 1000)
  await loadOverview()
  await loadTargets()
})

onBeforeUnmount(() => {
  clearHistoryPoll()
  if (arenaClockTimer !== null) clearInterval(arenaClockTimer)
})

function applyArenaServerTime(serverTime: unknown) {
  const serverTimeMs = Date.parse(String(serverTime || ""))
  if (Number.isFinite(serverTimeMs)) {
    arenaClockOffsetMs.value = serverTimeMs - Date.now()
    arenaClockTickMs.value = Date.now()
  }
}

function applyChallengeEntries(entries: ArenaChallengeEntries) {
  applyArenaServerTime(entries.serverTime)
  if (overview.value) overview.value.challengeEntries = entries
}

function applyRecommendationRefresh(state: ArenaRecommendationRefresh) {
  const currentServerTime = Date.parse(String(recommendationRefresh.value?.serverTime || ""))
  const nextServerTime = Date.parse(String(state.serverTime || ""))
  if (Number.isFinite(currentServerTime) && Number.isFinite(nextServerTime) && nextServerTime < currentServerTime) return
  applyArenaServerTime(state.serverTime)
  recommendationRefresh.value = state
}

function recommendationRefreshFromError(error: unknown): ArenaRecommendationRefresh | null {
  if (!(error instanceof APIClientError) || error.code !== "ARENA_RECOMMENDATION_REFRESH_COOLDOWN") return null
  const data = error.data as { recommendationRefresh?: ArenaRecommendationRefresh } | undefined
  const state = data?.recommendationRefresh
  if (!state || typeof state.serverTime !== "string" || !Number.isFinite(Number(state.cooldownSeconds))) return null
  return state
}

async function loadOverview(forceEditor = false) {
  if (!store.token) return
  try {
    const payload = await apiGet<ArenaOverview>("/api/arena/overview", store.token)
    overview.value = payload
    applyChallengeEntries(payload.challengeEntries)
    if (
      targetFormatFilter.value !== "all" &&
      !availableArenaFormats.value.some((format) => format.id === targetFormatFilter.value)
    ) {
      targetFormatFilter.value = "all"
    }
    if (!editorHydrated.value || forceEditor) hydrateEditor(payload)
  } catch (error) {
    ElMessage.error(resolveErrorMessage(t, error, "game.arena.error.loadFailed"))
  } finally {
    overviewLoading.value = false
  }
}

async function loadTargets(append = false) {
  if (!store.token) return
  if (append && (targetScope.value !== "more" || !targetsHasMore.value)) return
  if (append && (targetsLoading.value || targetsLoadingMore.value)) return
  if (targetsLoading.value || targetsLoadingMore.value) {
    targetsReloadQueued = true
    return
  }
  const requestedScope = targetScope.value
  const requestedFormat = targetFormatFilter.value
  const requestedOffset = append ? targetsNextOffset.value : 0
  if (append) targetsLoadingMore.value = true
  else {
    targetsLoading.value = true
    targetsHasMore.value = false
    targetsNextOffset.value = 0
  }
  try {
    const params = new URLSearchParams({
      scope: requestedScope,
      format: requestedFormat
    })
    if (requestedScope === "more") params.set("offset", String(requestedOffset))
    const payload = await apiGet<ArenaTargetsPayload>(`/api/arena/targets?${params.toString()}`, store.token)
    applyArenaServerTime(payload.opponentCooldown.serverTime)
    if (requestedScope !== targetScope.value || requestedFormat !== targetFormatFilter.value) {
      targetsReloadQueued = true
      return
    }
    if (payload.recommendationRefresh) applyRecommendationRefresh(payload.recommendationRefresh)
    if (append) {
      const existing = new Set(targets.value.map((target) => target.userId))
      targets.value = [...targets.value, ...payload.targets.filter((target) => !existing.has(target.userId))]
    } else {
      targets.value = payload.targets
    }
    if (requestedScope === "more") {
      const nextOffset = Math.max(requestedOffset, Math.trunc(Number(payload.nextOffset) || requestedOffset + payload.targets.length))
      targetsNextOffset.value = nextOffset
      targetsHasMore.value = Boolean(payload.hasMore) && nextOffset > requestedOffset
    } else {
      targetsNextOffset.value = 0
      targetsHasMore.value = false
    }
  } catch (error) {
    ElMessage.error(resolveErrorMessage(t, error, "game.arena.error.targetsFailed"))
  } finally {
    if (append) targetsLoadingMore.value = false
    else targetsLoading.value = false
    if (targetsReloadQueued) {
      targetsReloadQueued = false
      void loadTargets(false)
    }
  }
}

function loadMoreTargets() {
  void loadTargets(true)
}

async function loadHistory(silent = false) {
  if (!store.token || historyLoading.value) return
  if (!silent) historyLoading.value = true
  try {
    const payload = await apiGet<ArenaHistoryPayload>(`/api/arena/challenges?role=${historyRole.value}&offset=0`, store.token)
    const hadActive = history.value.some((row) => !isArenaChallengeTerminal(row.status))
    history.value = payload.challenges.slice(0, payload.historyKeepCount || 50)
    const hasActive = history.value.some((row) => !isArenaChallengeTerminal(row.status))
    if (hasActive) scheduleHistoryPoll()
    else {
      clearHistoryPoll()
      if (hadActive) await Promise.all([loadOverview(), loadTargets()])
    }
  } catch (error) {
    if (!silent) ElMessage.error(resolveErrorMessage(t, error, "game.arena.error.historyFailed"))
  } finally {
    historyLoading.value = false
  }
}

async function loadShop() {
  if (!store.token || shopLoading.value) return
  shopLoading.value = true
  try {
    shop.value = await apiGet<ArenaShopPayload>("/api/arena/shop", store.token)
  } catch (error) {
    ElMessage.error(resolveErrorMessage(t, error, "game.arena.error.shopFailed"))
  } finally {
    shopLoading.value = false
  }
}

async function refreshCurrentView() {
  if (refreshing.value || !showHeaderRefresh.value) return
  refreshing.value = true
  try {
    await loadOverview()
    if (activeTab.value === "records") await loadHistory()
    if (activeTab.value === "leaderboard") leaderboardRefreshVersion.value += 1
    if (activeTab.value === "shop") await loadShop()
  } finally {
    refreshing.value = false
  }
}

async function refreshRecommendedTargets() {
  if (!store.token || recommendationRefreshDisabled.value) return
  refreshing.value = true
  try {
    const payload = await apiPost<ArenaRecommendationRefreshPayload>("/api/arena/targets/refresh", store.token)
    applyRecommendationRefresh(payload.recommendationRefresh)
    await loadTargets(false)
  } catch (error) {
    const cooldownState = recommendationRefreshFromError(error)
    if (cooldownState) {
      applyRecommendationRefresh(cooldownState)
      await loadTargets(false)
    }
    ElMessage.error(resolveErrorMessage(t, error, "game.arena.error.targetsFailed"))
  } finally {
    refreshing.value = false
  }
}

function hydrateEditor(payload: ArenaOverview) {
  if (payload.defense?.selection) {
    const saved = payload.defense.selection
    const savedKitties = Array.isArray(saved.kitties) ? saved.kitties : []
    selection.playerPosition = normalizePosition(saved.playerPosition) || "vanguard"
    selection.formatId = String(saved.formatId).trim().toLowerCase()
    selection.kitties.splice(0, 2,
      savedKitties[0] ? { ...savedKitties[0] } : { kittyId: 0, position: "midguard" },
      savedKitties[1] ? { ...savedKitties[1] } : { kittyId: 0, position: "rearguard" }
    )
    editorHydrated.value = true
    return
  }
  const pair = pickDefaultKittyPair(payload.kitties)
  selection.formatId = "standard"
  selection.kitties[0].kittyId = pair[0]?.id || 0
  selection.kitties[1].kittyId = pair[1]?.id || 0
  normalizeEditorPositions()
  editorHydrated.value = true
}

function pickDefaultKittyPair(kitties: ArenaKitty[]): ArenaKitty[] {
  return kitties.slice(0, 2)
}

function normalizeEditorPositions() {
  for (let index = 0; index < 2; index += 1) {
    if (selection.kitties[index].kittyId <= 0) continue
    const locked = normalizePosition(selectedKitty(index)?.positionLocked)
    if (locked) selection.kitties[index].position = locked
  }
}

function handleKittyChange(index: number) {
  const locked = normalizePosition(selectedKitty(index)?.positionLocked)
  if (locked) selection.kitties[index].position = locked
  normalizeEditorPositions()
}

async function publishDefense() {
  if (!store.token || !selectionValid.value || publishing.value) return
  publishing.value = true
  try {
    await apiPost("/api/arena/defense/publish", store.token, {
      selection: {
        playerPosition: selection.playerPosition,
        formatId: selectedFormatId.value,
        kitties: activeArenaKittySelections(selection)
      }
    })
    ElMessage.success(t("game.arena.publishSuccess"))
    await Promise.all([loadOverview(true), loadTargets()])
  } catch (error) {
    if (!(await showArenaAlchemyEquipmentGate(error))) {
      ElMessage.error(resolveErrorMessage(t, error, "game.arena.error.publishFailed"))
    }
  } finally {
    publishing.value = false
  }
}

async function unpublishDefense() {
  if (!store.token || unpublishing.value) return
  try {
    await ElMessageBox.confirm(t("game.arena.unpublishConfirm"), t("game.arena.unpublish"), {
      confirmButtonText: t("game.arena.unpublish"),
      cancelButtonText: t("game.arena.cancel"),
      type: "warning"
    })
  } catch {
    return
  }
  unpublishing.value = true
  try {
    await apiPost("/api/arena/defense/unpublish", store.token)
    ElMessage.success(t("game.arena.unpublishSuccess"))
    await Promise.all([loadOverview(), loadTargets()])
  } catch (error) {
    ElMessage.error(resolveErrorMessage(t, error, "game.arena.error.unpublishFailed"))
  } finally {
    unpublishing.value = false
  }
}

function opponentCooldownRemaining(target: ArenaTarget): number {
  return arenaCooldownSecondsRemaining(target.opponentCooldownUntil, arenaNowMs.value)
}

function isTargetUnavailable(target: ArenaTarget): boolean {
  return challengeEntriesRemaining.value <= 0 || opponentCooldownRemaining(target) > 0
}

function challengeButtonLabel(target: ArenaTarget): string {
  if (challengeEntriesRemaining.value <= 0) {
    return t("game.arena.challengeEntryRecovering", { time: formatArenaCooldown(challengeEntrySecondsToNext.value) })
  }
  const remaining = opponentCooldownRemaining(target)
  if (remaining > 0) {
    return t("game.arena.challengeOpponentCooldown", { time: formatArenaCooldown(remaining) })
  }
  return t("game.arena.challenge")
}

function targetRatingRuleLabel(target: ArenaTarget): string {
  if (!target.ratingEligible) return t("game.arena.openChallengeUnrated")
  switch (target.recommendationTier) {
    case "nearby":
      return t("game.arena.recommendationNearby")
    case "relaxed":
      return t("game.arena.recommendationRelaxed")
    case "random":
      return t("game.arena.recommendationRandom")
    default:
      return t("game.arena.recommendationRated")
  }
}

async function challengeTarget(target: ArenaTarget) {
  if (!store.token || challengingID.value > 0 || !ownDefenseEligible.value || isTargetUnavailable(target)) return
  challengingID.value = target.userId
  try {
    await prepareArenaReplayPlayback()
    const payload = await apiPost<ArenaChallengeCreatePayload>("/api/arena/challenges", store.token, {
      defenderId: target.userId,
      recommendationToken: target.recommendationToken || ""
    })
    applyChallengeEntries(payload.challengeEntries)
    target.opponentCooldownUntil = payload.opponentCooldownUntil
    openArenaReplay({ challengeId: payload.challenge.id, live: true })
    ElMessage.success(t("game.arena.challengeQueued"))
    void loadHistory(true)
    scheduleHistoryPoll()
  } catch (error) {
    cancelPreparedArenaReplay()
    if (!(await showArenaAlchemyEquipmentGate(error))) {
      ElMessage.error(resolveErrorMessage(t, error, "game.arena.error.challengeFailed"))
    }
    void Promise.all([loadOverview(), loadTargets()])
  } finally {
    challengingID.value = 0
  }
}

async function showArenaAlchemyEquipmentGate(error: unknown): Promise<boolean> {
  const payload = arenaAlchemyGatePayloadFromError(error)
  if (!payload) return false
  const visible = payload.violations.slice(0, 12)
  const content = h("div", { class: "arena-alchemy-gate" }, [
    h("p", { class: "arena-alchemy-gate__hint" }, t("game.arena.alchemyGateHint")),
    ...visible.map((violation) => {
      const unitName = violation.unitName || (violation.unitType === "kitty" ? t("game.arena.kitty") : t("game.arena.currentPlayer"))
      const baseItemName = getLocalizedItemName(t, violation.templateItemId)
      const itemName = violation.equipmentName
        ? t("game.arena.alchemyGateEquipmentName", { custom: violation.equipmentName, base: baseItemName })
        : baseItemName
      const attrKey = `attr.${violation.attribute}.name`
      const translatedAttr = t(attrKey)
      const attrName = violation.attribute === "alchemy_combo_power"
        ? t("game.arena.alchemyGateComboPower")
        : translatedAttr === attrKey ? violation.attribute : translatedAttr
      const actual = formatAttributeValueByKey(violation.attribute, violation.actualValue, true, 3)
      const allowed = formatAttributeValueByKey(violation.attribute, violation.allowedValue, true, 3)
      const version = violation.alchemyRulesetVersion || t("game.arena.alchemyGateLegacyVersion")
      return h("div", { class: "arena-alchemy-gate__row" }, [
        h("strong", `${unitName} · ${itemName}`),
        h("span", `${attrName}: ${t("game.arena.alchemyGateValue", { actual, allowed })}`),
        h("small", t("game.arena.alchemyGateAffixVersion", { slot: violation.affixSlotIndex + 1, version }))
      ])
    }),
    ...(payload.violations.length > visible.length
      ? [h("small", { class: "arena-alchemy-gate__more" }, t("game.arena.alchemyGateMore", { count: payload.violations.length - visible.length }))]
      : [])
  ])
  try {
    await ElMessageBox.alert(content, t("game.arena.alchemyGateTitle"), {
      confirmButtonText: t("game.confirm"),
      type: "warning",
      customClass: "arena-alchemy-gate-dialog"
    })
  } catch {
    // Alert dismissal does not change the rejected league request.
  }
  return true
}

async function buyOffer(offer: ArenaShopOffer) {
  if (!store.token || buyingOfferID.value) return
  buyingOfferID.value = offer.id
  try {
    const payload = await apiPost<{ purchase: { tokens: number } }>(
      "/api/arena/shop/buy",
      store.token,
      { offerId: offer.id },
      { "X-Idempotency-Key": createIdempotencyKey(`arena-${offer.id}`) }
    )
    if (shop.value) shop.value.tokens = payload.purchase.tokens
    if (overview.value) overview.value.state.tokens = payload.purchase.tokens
    ElMessage.success(t("game.arena.buySuccess"))
  } catch (error) {
    ElMessage.error(resolveErrorMessage(t, error, "game.arena.error.buyFailed"))
  } finally {
    buyingOfferID.value = ""
  }
}

function scheduleHistoryPoll() {
  clearHistoryPoll()
  historyPollTimer = setTimeout(() => void loadHistory(true), 2000)
}

function clearHistoryPoll() {
  if (historyPollTimer === null) return
  clearTimeout(historyPollTimer)
  historyPollTimer = null
}

function selectedKitty(index: number): ArenaKitty | undefined {
  const kittyID = selection.kitties[index]?.kittyId
  return overview.value?.kitties.find((kitty) => kitty.id === kittyID)
}

function kittyUsedByOther(kittyID: number, index: number): boolean {
  if (kittyID <= 0) return false
  return selection.kitties.some((kitty, kittyIndex) => kittyIndex !== index && kitty.kittyId === kittyID)
}

function normalizePosition(raw: unknown): ArenaPosition | "" {
  const value = String(raw || "").trim()
  return value === "vanguard" || value === "midguard" || value === "rearguard" ? value : ""
}

function positionLabel(position: ArenaPosition): string {
  return t(`game.arena.position.${position}`)
}

function formatName(format: ArenaFormat): string {
	return t(format.nameKey)
}

function formatShortDescription(format: ArenaFormat): string {
	return t(format.shortDescKey)
}

function lineupLanes(lineup: ArenaLineupUnit[]) {
  return arenaPositions.map((position) => ({
    position,
    units: lineup.filter((unit) => unit.position === position)
  }))
}

function roleLabel(role: ArenaChallenge["role"]): string {
  return role === "defense" ? t("game.arena.defenseRole") : t("game.arena.attackRole")
}

function didViewerWin(challenge: ArenaChallenge): boolean {
  return (challenge.role === "attack" && challenge.status === "attacker_win") || (challenge.role === "defense" && challenge.status === "defender_win")
}

function historyStatusLabel(challenge: ArenaChallenge): string {
  if (challenge.status === "queued") return t("game.arena.queued")
  if (challenge.status === "running") return t("game.arena.running")
  if (challenge.status === "failed") return t("game.arena.failed")
  return didViewerWin(challenge) ? t("game.arena.victory") : t("game.arena.defeat")
}

function historyStatusType(challenge: ArenaChallenge): "success" | "warning" | "info" | "danger" {
  if (challenge.status === "queued" || challenge.status === "running") return "info"
  if (challenge.status === "failed") return "danger"
  return didViewerWin(challenge) ? "success" : "warning"
}

function ownReward(challenge: ArenaChallenge): number {
  return challenge.role === "defense" ? challenge.defenderRewardTokens : challenge.attackerRewardTokens
}

function ownRatingDelta(challenge: ArenaChallenge): number {
  return challenge.role === "defense" ? challenge.defenderRatingDelta : challenge.attackerRatingDelta
}

function signedValue(value: number): string {
  const rounded = Math.trunc(Number(value) || 0)
  return `${rounded > 0 ? "+" : ""}${formatNumber(rounded)}`
}

function replayAvailable(challenge: ArenaChallenge): boolean {
  return challenge.status === "attacker_win" || challenge.status === "defender_win"
}

function openReplay(challengeID: number) {
  openArenaReplay({ challengeId: challengeID })
}

async function shareReplay(challenge: ArenaChallenge) {
  if (!store.token || !replayAvailable(challenge) || sharingChallengeID.value > 0) return
  sharingChallengeID.value = challenge.id
  try {
    const payload = await apiPost<ArenaReplaySharePayload>(`/api/arena/challenges/${challenge.id}/share`, store.token)
    const channel = chatStore.canSendToChannel(chatStore.activeChannel)
      ? chatStore.activeChannel
      : chatStore.channels.find((item) => item.playerSendable)?.id || chatStore.defaultChannel
    if (!chatStore.canSendToChannel(channel)) {
      throw new Error("no sendable chat channel")
    }
    chatStore.setActiveChannel(channel)
    chatStore.setPanelOpen(true)
    chatStore.setPendingAttachment({
      type: "arena_replay",
      battleKind: "arena",
      token: payload.share.token,
      challengeId: payload.share.challengeId,
      createdBy: payload.share.createdBy,
      createdByName: payload.share.createdByName,
      outcome: payload.share.outcome
    })
    ElMessage.success(t("game.arena.shareReplaySuccess"))
  } catch (error) {
    ElMessage.error(resolveErrorMessage(t, error, "game.arena.error.replayShareFailed"))
  } finally {
    sharingChallengeID.value = 0
  }
}

function formatDate(raw: string): string {
  const date = new Date(raw)
  if (Number.isNaN(date.getTime())) return ""
  return new Intl.DateTimeFormat(locale.value, { month: "2-digit", day: "2-digit", hour: "2-digit", minute: "2-digit" }).format(date)
}

function formatNumber(value: number): string {
  return formatNumberWithThousands(Math.trunc(Number(value) || 0))
}

function arenaSkillIDs(unit: ArenaLineupUnit): string[] {
  return [...new Set((unit.skillIds || []).map((skillID) => String(skillID || "").trim()).filter(Boolean))]
}

function visibleArenaSkills(unit: ArenaLineupUnit): string[] {
  return arenaSkillIDs(unit).slice(0, 2)
}

function hiddenArenaSkillCount(unit: ArenaLineupUnit): number {
  return Math.max(0, arenaSkillIDs(unit).length - 2)
}
</script>

<style scoped>
.arena-page {
  --arena-blue: #2563eb;
  --arena-amber: #b96b08;
  --arena-green: #168447;
  --arena-red: #c2413b;
  --arena-line: var(--el-border-color-lighter);
  --arena-soft: color-mix(in srgb, var(--el-fill-color-light) 72%, var(--el-bg-color));
  display: grid;
  gap: 14px;
  min-height: 500px;
}

.arena-command-bar {
  position: relative;
  display: grid;
  grid-template-columns: minmax(190px, 1.25fr) repeat(3, minmax(130px, 0.8fr)) 48px;
  align-items: stretch;
  overflow: hidden;
  border: 1px solid var(--arena-line);
  border-radius: 6px;
  background: var(--arena-line);
}

.arena-command-bar--without-refresh {
  grid-template-columns: minmax(190px, 1.25fr) repeat(3, minmax(130px, 0.8fr));
}

.arena-rating-block,
.arena-command-stat {
  min-width: 0;
  min-height: 64px;
  background: var(--el-bg-color);
}

.arena-rating-block {
  display: flex;
  align-items: center;
  gap: 11px;
  padding: 10px 14px;
  border-left: 3px solid var(--arena-blue);
}

.arena-command-icon {
  width: 34px;
  height: 34px;
  display: grid;
  place-items: center;
  flex: none;
  border-radius: 5px;
  color: var(--arena-blue);
  background: color-mix(in srgb, var(--arena-blue) 11%, transparent);
  font-size: 1rem;
}

.arena-rating-block > div,
.arena-command-stat {
  display: grid;
  align-content: center;
  gap: 3px;
}

.arena-command-stat {
  padding: 10px 13px;
}

.arena-rating-block span:not(.arena-command-icon),
.arena-command-stat > span {
  overflow: hidden;
  color: var(--app-muted);
  font-size: 0.7rem;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.arena-rating-block strong {
  font-size: 1.22rem;
}

.arena-command-stat strong {
  font-size: 0.96rem;
}

.arena-command-stat strong small {
  color: var(--app-muted);
  font-size: 0.68rem;
  font-weight: 500;
}

.arena-command-stat--bounty strong {
  color: var(--arena-amber);
}

.arena-command-refresh {
  display: grid;
  place-items: center;
}

.arena-command-refresh__trigger {
  display: inline-flex;
}

.arena-tab-label,
.arena-help-label {
  display: inline-flex;
  align-items: center;
  gap: 6px;
}

.arena-tab-label svg {
  font-size: 0.9rem;
}

.arena-tabs :deep(.el-tabs__header) {
  margin-bottom: 14px;
}

.arena-tabs,
.arena-tabs :deep(.el-tabs__content),
.arena-tabs :deep(.el-tab-pane) {
  min-width: 0;
  max-width: 100%;
}

.arena-tabs :deep(.el-tabs__item) {
  height: 42px;
  padding-inline: 18px;
  font-weight: 700;
}

.arena-pane {
  box-sizing: border-box;
  width: 100%;
  display: grid;
  align-content: start;
  gap: 14px;
  min-height: 380px;
}

.arena-pane-toolbar {
  min-height: 36px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.arena-target-filters {
  min-width: 0;
  display: flex;
  align-items: center;
  gap: 8px;
}

.arena-format-tabs {
  min-width: 0;
  display: flex;
  align-items: center;
  gap: 18px;
  overflow-x: auto;
  border-bottom: 1px solid var(--arena-line);
  scrollbar-width: thin;
}

.arena-format-tabs button {
  position: relative;
  min-width: max-content;
  height: 36px;
  display: inline-flex;
  align-items: center;
  gap: 6px;
  flex: none;
  padding: 0 1px 8px;
  border: 0;
  color: var(--app-muted);
  background: transparent;
  font: inherit;
  font-size: 0.72rem;
  font-weight: 700;
  cursor: pointer;
}

.arena-format-tabs button::after {
  position: absolute;
  right: 0;
  bottom: -1px;
  left: 0;
  height: 2px;
  background: transparent;
  content: "";
}

.arena-format-tabs button:hover,
.arena-format-tabs button.is-active {
  color: var(--arena-blue);
}

.arena-format-tabs button.is-active::after {
  background: var(--arena-blue);
}

.arena-target-list-toolbar {
  min-width: 0;
  min-height: 34px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.arena-target-list-title {
  min-width: 0;
  display: inline-flex;
  align-items: center;
  gap: 6px;
  color: var(--el-text-color-primary);
  font-size: 0.78rem;
  font-weight: 700;
}

.arena-target-list-title svg {
  color: var(--arena-blue);
}

.arena-target-list-toolbar .el-button {
  min-width: 136px;
  max-width: 100%;
}

.arena-inline-notice,
.arena-history-limit {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  color: var(--app-muted);
  font-size: 0.76rem;
}

.arena-inline-notice {
  color: var(--arena-amber);
}

.arena-defense-policy-notice {
  width: fit-content;
  max-width: 100%;
  padding: 7px 10px;
  border-left: 3px solid var(--arena-amber);
  background: color-mix(in srgb, var(--arena-amber) 8%, transparent);
}

.arena-entry-status {
  min-height: 28px;
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 3px 8px;
  border: 1px solid color-mix(in srgb, var(--arena-green) 28%, var(--arena-line));
  border-radius: 4px;
  color: var(--arena-green);
  background: color-mix(in srgb, var(--arena-green) 6%, transparent);
  font-size: 0.7rem;
  font-weight: 700;
  line-height: 1.25;
  cursor: help;
}

.arena-entry-status.is-empty {
  border-color: color-mix(in srgb, var(--arena-amber) 32%, var(--arena-line));
  color: var(--arena-amber);
  background: color-mix(in srgb, var(--arena-amber) 7%, transparent);
}

.arena-entry-status__meter {
  display: inline-grid;
  grid-auto-flow: column;
  grid-auto-columns: 5px;
  gap: 2px;
  height: 12px;
  align-items: end;
}

.arena-entry-status__meter i {
  width: 5px;
  height: 7px;
  border: 1px solid color-mix(in srgb, currentColor 32%, transparent);
  background: transparent;
}

.arena-entry-status__meter i.is-filled {
  height: 12px;
  border-color: currentColor;
  background: currentColor;
}

.arena-entry-status__help {
  color: var(--app-muted);
  font-size: 0.68rem;
}

.arena-target-grid {
  box-sizing: border-box;
  width: 100%;
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  align-items: start;
  gap: 12px;
  min-height: 280px;
}

.arena-target-grid > .el-empty {
  grid-column: 1 / -1;
}

.arena-target-load-more {
  min-height: 34px;
  display: flex;
  justify-content: center;
}

.arena-target-load-more .el-button {
  min-width: 132px;
}

.arena-target {
  box-sizing: border-box;
  width: 100%;
  max-width: 100%;
  min-width: 0;
  display: grid;
  align-content: start;
  overflow: hidden;
  border: 1px solid var(--arena-line);
  border-radius: 6px;
  background: var(--el-bg-color);
  transition: border-color 0.16s ease, box-shadow 0.16s ease;
}

.arena-target:hover {
  border-color: color-mix(in srgb, var(--arena-blue) 38%, var(--arena-line));
  box-shadow: 0 8px 20px color-mix(in srgb, var(--el-text-color-primary) 7%, transparent);
}

.arena-target__header,
.arena-target__footer {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 10px 12px;
}

.arena-target__header {
  min-height: 58px;
}

.arena-target__identity {
  min-width: 0;
  display: flex;
  align-items: center;
  gap: 9px;
}

.arena-target__avatar {
  width: 32px;
  height: 32px;
  display: grid;
  place-items: center;
  flex: none;
  border: 1px solid color-mix(in srgb, var(--arena-blue) 18%, transparent);
  border-radius: 5px;
  color: var(--arena-blue);
  background: color-mix(in srgb, var(--arena-blue) 9%, transparent);
}

.arena-target__identity > div {
  min-width: 0;
  display: grid;
  gap: 2px;
}

.arena-target__signals {
  display: flex;
  align-items: center;
  gap: 8px;
}

.arena-target__identity .arena-target__rating-rule {
  width: fit-content;
  display: inline-flex;
  align-items: center;
  gap: 3px;
  padding: 1px 5px;
  border: 1px solid var(--arena-line);
  border-radius: 3px;
  color: var(--app-muted);
  background: var(--arena-soft);
  font-size: 0.6rem;
  font-weight: 700;
  line-height: 1.3;
}

.arena-target__identity .arena-target__rating-rule.is-rated {
  border-color: color-mix(in srgb, var(--arena-blue) 28%, var(--arena-line));
  color: var(--arena-blue);
  background: color-mix(in srgb, var(--arena-blue) 7%, transparent);
}

.arena-format-marker {
  width: fit-content;
  max-width: 180px;
  display: inline-flex !important;
  align-items: center;
  gap: 4px;
  padding: 2px 5px;
  border: 1px solid color-mix(in srgb, var(--arena-amber) 30%, var(--arena-line));
  border-radius: 3px;
  color: var(--arena-amber) !important;
  background: color-mix(in srgb, var(--arena-amber) 7%, transparent);
  font-size: 0.6rem !important;
  font-weight: 700;
  line-height: 1.2;
}

.arena-format-marker--standard {
  border-color: var(--arena-line);
  color: var(--app-muted) !important;
  background: var(--arena-soft);
}

.arena-target__signals > span {
  display: inline-flex;
  align-items: center;
  gap: 3px;
  color: var(--app-muted);
  font-size: 0.62rem;
}

.arena-target h2 {
  overflow: hidden;
  margin: 0;
  font-size: 0.88rem;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.arena-target__identity span,
.arena-target__bounty span,
.arena-target__footer span {
  color: var(--app-muted);
  font-size: 0.68rem;
}

.arena-target__bounty,
.arena-target__footer > div {
  display: grid;
  gap: 2px;
  flex: none;
  text-align: right;
}

.arena-target__bounty strong {
  color: var(--arena-amber);
  font-size: 0.94rem;
}

.arena-lineup-board {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  min-height: 168px;
  border-block: 1px solid var(--arena-line);
  background: var(--arena-soft);
}

.arena-lineup-lane {
  min-width: 0;
  display: flex;
  flex-direction: column;
  align-items: stretch;
  gap: 7px;
  padding: 9px 7px;
  border-right: 1px solid var(--arena-line);
  text-align: center;
}

.arena-lineup-lane:last-child {
  border-right: 0;
}

.arena-lineup-lane__position {
  flex: none;
  color: var(--arena-blue);
  font-size: 0.68rem;
  font-weight: 700;
}

.arena-lineup-lane__members {
  min-width: 0;
  display: grid;
  align-content: center;
  gap: 5px;
  flex: 1;
}

.arena-lineup-member {
  min-width: 0;
  display: grid;
  grid-template-columns: 30px minmax(0, 1fr) auto;
  grid-template-rows: minmax(18px, auto) minmax(18px, auto) minmax(20px, auto);
  align-items: center;
  gap: 2px 6px;
  padding-block: 4px;
}

.arena-lineup-member + .arena-lineup-member {
  border-top: 1px solid var(--arena-line);
}

.arena-lineup-lane__token {
  width: 30px;
  height: 30px;
  display: grid;
  place-items: center;
  border: 1px solid var(--el-border-color);
  border-radius: 50%;
  color: var(--el-text-color-regular);
  background: var(--el-bg-color);
}

.arena-lineup-member > .arena-lineup-lane__token {
  grid-column: 1;
  grid-row: 1 / 3;
}

.arena-lineup-member__copy {
  min-width: 0;
  display: grid;
  grid-column: 2;
  grid-row: 1 / 3;
  align-content: center;
  gap: 1px;
  text-align: left;
}

.arena-lineup-member strong,
.arena-lineup-lane__empty strong {
  overflow: hidden;
  width: 100%;
  font-size: 0.76rem;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.arena-lineup-member small {
  color: var(--app-muted);
  font-size: 0.64rem;
}

.arena-lineup-lane__hp {
  display: inline-flex;
  align-items: center;
  gap: 3px;
  color: var(--arena-red);
  font-size: 0.66rem;
  font-variant-numeric: tabular-nums;
}

.arena-lineup-member > .arena-lineup-lane__hp {
  grid-column: 3;
  grid-row: 1;
  justify-self: end;
}

.arena-lineup-lane__skills {
  min-width: 0;
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 3px;
  overflow: hidden;
}

.arena-lineup-member > .arena-lineup-lane__skills {
  grid-column: 2 / 4;
  grid-row: 3;
  width: 100%;
  justify-content: flex-start;
  max-width: 100%;
}

.arena-lineup-lane__skills :deep(.el-tag) {
  min-width: 0;
  max-width: 46%;
  height: 19px;
  padding-inline: 4px;
  font-size: 0.58rem;
}

.arena-lineup-lane__skills :deep(.el-tag > span) {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.arena-lineup-lane__skills > small {
  flex: none;
  color: var(--arena-blue);
  font-size: 0.58rem;
}

.arena-lineup-lane--empty {
  color: var(--app-muted);
}

.arena-lineup-lane__empty {
  display: grid;
  place-items: center;
  align-content: center;
  gap: 5px;
  flex: 1;
}

.arena-lineup-lane--empty .arena-lineup-lane__token {
  border-style: dashed;
  background: transparent;
}

.arena-target__footer {
  min-height: 56px;
}

.arena-target__footer .arena-help-label {
  justify-self: end;
}

.arena-target__footer strong {
  color: var(--arena-green);
  font-size: 0.9rem;
}

.arena-target__footer strong small {
  font-size: 0.64rem;
}

.arena-defense-pane {
  max-width: 1160px;
}

.arena-defense-status,
.arena-shop-header {
  min-height: 62px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 14px;
  padding-bottom: 12px;
  border-bottom: 1px solid var(--arena-line);
}

.arena-defense-status > div:first-child {
  min-width: 0;
  display: flex;
  align-items: center;
  gap: 11px;
}

.arena-defense-status > div:first-child > div {
  min-width: 0;
  display: grid;
  gap: 3px;
}

.arena-defense-status h2 {
  overflow: hidden;
  margin: 0;
  font-size: 0.9rem;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.arena-defense-status h2 + span {
  color: var(--app-muted);
  font-size: 0.7rem;
}

.arena-kind-mark {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  flex: none;
  padding: 6px 8px;
  border: 1px solid color-mix(in srgb, var(--arena-amber) 45%, transparent);
  border-radius: 4px;
  color: var(--arena-amber);
  background: color-mix(in srgb, var(--arena-amber) 9%, transparent);
  font-size: 0.7rem;
  font-weight: 700;
}

.arena-defense-counters {
  display: flex;
  flex-wrap: wrap;
  justify-content: flex-end;
  gap: 12px;
  color: var(--app-muted);
  font-size: 0.72rem;
}

.arena-defense-counters span {
  display: inline-flex;
  align-items: center;
  gap: 5px;
}

.arena-defense-workspace {
  display: grid;
  grid-template-columns: minmax(310px, 0.8fr) minmax(520px, 1.35fr);
  align-items: stretch;
  gap: 16px;
}

.arena-formation-preview {
  min-width: 0;
  display: grid;
  grid-template-rows: auto 1fr;
  overflow: hidden;
  border: 1px solid var(--arena-line);
  border-radius: 6px;
  background: var(--arena-soft);
}

.arena-formation-preview > header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  min-height: 42px;
  padding: 8px 11px;
  border-bottom: 1px solid var(--arena-line);
  background: var(--el-bg-color);
  font-size: 0.72rem;
}

.arena-formation-preview > header span {
  color: var(--app-muted);
}

.arena-formation-preview > header strong {
  color: var(--arena-blue);
}

.arena-preview-lanes {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  min-height: 180px;
}

.arena-preview-lane {
  min-width: 0;
  display: flex;
  flex-direction: column;
  align-items: stretch;
  gap: 8px;
  padding: 12px 8px;
  border-right: 1px solid var(--arena-line);
  text-align: center;
}

.arena-preview-lane:last-child {
  border-right: 0;
}

.arena-preview-lane > span:first-child {
  flex: none;
  color: var(--arena-blue);
  font-size: 0.68rem;
  font-weight: 700;
}

.arena-preview-lane__members {
  min-width: 0;
  display: grid;
  align-content: center;
  gap: 5px;
  flex: 1;
}

.arena-preview-member {
  min-width: 0;
  display: grid;
  grid-template-columns: 40px minmax(0, 1fr);
  align-items: center;
  gap: 7px;
  padding-block: 3px;
  text-align: left;
}

.arena-preview-member + .arena-preview-member {
  border-top: 1px solid var(--arena-line);
}

.arena-preview-member > span:last-child {
  min-width: 0;
  display: grid;
  gap: 1px;
}

.arena-preview-lane__token {
  width: 40px;
  height: 40px;
  display: grid;
  place-items: center;
  border: 1px solid color-mix(in srgb, var(--arena-blue) 28%, var(--el-border-color));
  border-radius: 50%;
  color: var(--arena-blue);
  background: var(--el-bg-color);
  font-size: 1rem;
}

.arena-preview-member strong,
.arena-preview-lane__empty strong {
  overflow: hidden;
  width: 100%;
  font-size: 0.76rem;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.arena-preview-member small {
  color: var(--app-muted);
  font-size: 0.66rem;
}

.arena-preview-lane--empty {
  color: var(--app-muted);
}

.arena-preview-lane__empty {
  display: grid;
  place-items: center;
  align-content: center;
  gap: 5px;
  flex: 1;
}

.arena-preview-lane--empty .arena-preview-lane__token {
  border-style: dashed;
  color: var(--app-muted);
  background: transparent;
}

.arena-formation-editor {
  min-width: 0;
  display: grid;
  align-content: start;
  border-block: 1px solid var(--arena-line);
}

.arena-formation-row {
  position: relative;
  min-width: 0;
  display: grid;
  grid-template-columns: minmax(145px, 0.9fr) minmax(170px, 1.2fr) minmax(120px, 0.75fr) 28px;
  align-items: center;
  gap: 9px;
  min-height: 72px;
  padding: 9px 2px;
  border-bottom: 1px solid var(--el-border-color-extra-light);
}

.arena-formation-row:last-child {
  border-bottom: 0;
}

.arena-formation-row__player-position {
  grid-column: 2 / 4;
}

.arena-formation-row__identity {
  min-width: 0;
  display: flex;
  align-items: center;
  gap: 9px;
}

.arena-formation-row__identity > span {
  width: 30px;
  height: 30px;
  display: grid;
  place-items: center;
  flex: none;
  border: 1px solid var(--el-border-color);
  border-radius: 4px;
  color: var(--el-text-color-regular);
  background: var(--arena-soft);
}

.arena-formation-row__identity > div {
  min-width: 0;
  display: grid;
  gap: 2px;
}

.arena-formation-row__identity strong {
  overflow: hidden;
  font-size: 0.78rem;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.arena-formation-row__identity small {
  color: var(--app-muted);
  font-size: 0.66rem;
}

.arena-position-lock {
  width: 28px;
  height: 28px;
  display: grid;
  place-items: center;
  border: 1px solid color-mix(in srgb, var(--arena-amber) 40%, transparent);
  border-radius: 4px;
  color: var(--arena-amber);
  background: color-mix(in srgb, var(--arena-amber) 8%, transparent);
}

.arena-format-policy {
  min-width: 0;
  display: grid;
  gap: 10px;
  padding-block: 12px;
  border-block: 1px solid var(--arena-line);
}

.arena-format-policy > header {
  min-width: 0;
  display: flex;
  align-items: center;
  gap: 10px;
}

.arena-format-policy__icon {
  width: 34px;
  height: 34px;
  display: grid;
  place-items: center;
  flex: none;
  border: 1px solid color-mix(in srgb, var(--arena-amber) 24%, var(--arena-line));
  border-radius: 4px;
  color: var(--arena-amber);
  background: color-mix(in srgb, var(--arena-amber) 8%, transparent);
}

.arena-format-policy > header > div {
  min-width: 0;
  display: grid;
  gap: 3px;
}

.arena-format-policy > header strong {
  font-size: 0.78rem;
}

.arena-format-policy > header small {
  color: var(--app-muted);
  font-size: 0.66rem;
  line-height: 1.45;
}

.arena-format-options {
  min-width: 0;
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 8px;
}

.arena-format-options button {
  width: 100%;
  min-width: 0;
  min-height: 62px;
  display: grid;
  grid-template-columns: 32px minmax(0, 1fr) 18px;
  align-items: center;
  gap: 9px;
  padding: 9px 10px;
  border: 1px solid var(--arena-line);
  border-radius: 5px;
  color: var(--el-text-color-primary);
  background: var(--el-bg-color);
  font: inherit;
  text-align: left;
  cursor: pointer;
  transition: border-color 0.16s ease, background-color 0.16s ease;
}

.arena-format-options button:hover {
  border-color: color-mix(in srgb, var(--arena-blue) 38%, var(--arena-line));
}

.arena-format-options button.is-selected {
  border-color: color-mix(in srgb, var(--arena-blue) 58%, var(--arena-line));
  background: color-mix(in srgb, var(--arena-blue) 5%, var(--el-bg-color));
}

.arena-format-option__icon {
  width: 30px;
  height: 30px;
  display: grid;
  place-items: center;
  border-radius: 4px;
  color: var(--arena-amber);
  background: color-mix(in srgb, var(--arena-amber) 9%, transparent);
}

.arena-format-option__copy {
  min-width: 0;
  display: grid;
  gap: 3px;
}

.arena-format-option__copy strong {
  overflow: hidden;
  font-size: 0.74rem;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.arena-format-option__copy small {
  display: -webkit-box;
  overflow: hidden;
  color: var(--app-muted);
  font-size: 0.62rem;
  line-height: 1.35;
  -webkit-box-orient: vertical;
  -webkit-line-clamp: 2;
}

.arena-format-option__state {
  color: var(--arena-blue);
}

.arena-format-touch-rules {
  display: none;
  padding: 10px;
  border: 1px solid var(--arena-line);
  border-radius: 5px;
  background: color-mix(in srgb, var(--arena-amber) 3%, var(--el-bg-color));
}

@media (hover: none), (pointer: coarse) {
  .arena-format-touch-rules {
    display: block;
  }
}

.arena-defense-actions {
  min-height: 50px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding-top: 4px;
  border-top: 1px solid var(--arena-line);
}

.arena-defense-actions > span {
  color: var(--app-muted);
  font-size: 0.72rem;
}

.arena-defense-actions > div {
  display: flex;
  gap: 8px;
}

.arena-history-list {
  max-height: 580px;
  min-height: 300px;
  overflow-y: auto;
  border-block: 1px solid var(--arena-line);
}

.arena-history-row {
  min-width: 0;
  display: grid;
  grid-template-columns: 34px minmax(0, 1fr) minmax(170px, auto) 76px;
  align-items: center;
  gap: 10px;
  min-height: 70px;
  padding: 9px 4px;
  border-bottom: 1px solid var(--el-border-color-extra-light);
}

.arena-history-row__actions {
  display: flex;
  justify-content: flex-end;
  gap: 6px;
}

.arena-history-row:last-child {
  border-bottom: 0;
}

.arena-history-row__marker {
  width: 30px;
  height: 30px;
  display: grid;
  place-items: center;
  border: 1px solid var(--el-border-color);
  border-radius: 4px;
  color: var(--arena-amber);
  background: var(--arena-soft);
}

.arena-history-row__main {
  min-width: 0;
  display: grid;
  gap: 5px;
}

.arena-history-row__title,
.arena-history-row__meta {
  min-width: 0;
  display: flex;
  align-items: center;
  gap: 7px;
}

.arena-history-row__title strong {
  overflow: hidden;
  max-width: 260px;
  font-size: 0.8rem;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.arena-history-row__meta,
.arena-history-row__settlement {
  color: var(--app-muted);
  font-size: 0.68rem;
}

.arena-history-row__settlement {
  display: grid;
  gap: 4px;
  text-align: right;
}

.arena-shop-pane {
  max-width: 960px;
}

.arena-shop-header > span {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  color: var(--app-muted);
  font-size: 0.74rem;
}

.arena-shop-balance {
  display: flex;
  align-items: center;
  gap: 10px;
}

.arena-shop-balance__icon {
  width: 34px;
  height: 34px;
  display: grid;
  place-items: center;
  border-radius: 5px;
  color: var(--arena-amber);
  background: color-mix(in srgb, var(--arena-amber) 10%, transparent);
}

.arena-shop-balance > div {
  display: grid;
  gap: 2px;
}

.arena-shop-balance .arena-help-label {
  color: var(--app-muted);
  font-size: 0.7rem;
}

.arena-shop-balance strong {
  font-size: 1.06rem;
}

.arena-shop-list {
  display: grid;
  min-height: 290px;
  border-top: 1px solid var(--arena-line);
}

.arena-shop-empty {
  display: grid;
  justify-items: center;
  align-content: center;
  gap: 8px;
  min-height: 270px;
  color: var(--app-muted);
  text-align: center;
}

.arena-shop-empty > span {
  width: 48px;
  height: 48px;
  display: grid;
  place-items: center;
  border: 1px dashed var(--el-border-color);
  border-radius: 6px;
  color: var(--arena-amber);
  background: var(--arena-soft);
  font-size: 1.25rem;
}

.arena-shop-empty strong {
  color: var(--el-text-color-primary);
  font-size: 0.88rem;
}

.arena-shop-empty small {
  max-width: 420px;
  font-size: 0.72rem;
  line-height: 1.6;
}

.arena-shop-offer {
  min-width: 0;
  display: grid;
  grid-template-columns: minmax(0, 1fr) 110px auto;
  align-items: center;
  gap: 12px;
  min-height: 66px;
  padding: 8px 4px;
  border-bottom: 1px solid var(--el-border-color-extra-light);
}

.arena-shop-offer__cost {
  display: grid;
  gap: 2px;
  text-align: right;
}

.arena-shop-offer__cost span {
  color: var(--app-muted);
  font-size: 0.68rem;
}

@media (max-width: 1560px) {
  .arena-target-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}

@media (max-width: 1080px) {
  .arena-command-bar {
    grid-template-columns: minmax(180px, 1.2fr) repeat(3, minmax(115px, 0.8fr)) 46px;
  }

  .arena-target-grid {
    grid-template-columns: minmax(0, 1fr);
  }

  .arena-defense-workspace {
    grid-template-columns: minmax(0, 1fr);
  }

  .arena-format-options {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}

@media (max-width: 720px) {
  .arena-page {
    gap: 10px;
  }

  .arena-command-bar {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .arena-rating-block {
    grid-column: 1 / -1;
    padding-right: 58px;
  }

  .arena-command-bar--without-refresh .arena-rating-block {
    padding-right: 14px;
  }

  .arena-command-stat--bounty {
    grid-column: 1 / -1;
  }

  .arena-command-refresh {
    position: absolute;
    top: 15px;
    right: 12px;
  }

  .arena-tabs :deep(.el-tabs__nav-wrap) {
    overflow-x: auto;
  }

  .arena-tabs :deep(.el-tabs__item) {
    padding-inline: 11px;
    font-size: 0.76rem;
  }

  .arena-pane-toolbar,
  .arena-defense-status,
  .arena-shop-header {
    width: 100%;
    align-items: flex-start;
    flex-direction: column;
  }

  .arena-target-filters {
    width: 100%;
    flex-wrap: wrap;
  }

  .arena-entry-status {
    max-width: 100%;
  }

  .arena-entry-status__meter {
    display: none;
  }

  .arena-target-filters :deep(.el-segmented) {
    flex: 1 1 230px;
  }

  .arena-target-list-toolbar {
    align-items: stretch;
    flex-direction: column;
  }

  .arena-target-list-toolbar .el-button {
    width: 100%;
    min-height: 34px;
    height: auto;
    white-space: normal;
  }

  .arena-defense-counters {
    justify-content: flex-start;
  }

  .arena-lineup-board,
  .arena-preview-lanes {
    grid-template-columns: repeat(3, minmax(0, 1fr));
  }

  .arena-lineup-lane {
    padding-inline: 5px;
  }

  .arena-target__footer {
    align-items: flex-end;
  }

  .arena-formation-row {
    grid-template-columns: minmax(0, 1fr) minmax(104px, 0.55fr) 28px;
    gap: 8px;
    padding-block: 11px;
  }

  .arena-formation-row__identity {
    grid-column: 1 / -1;
  }

  .arena-formation-row__player-position {
    grid-column: 1 / -1;
  }

  .arena-formation-row__unit-select {
    grid-column: 1;
  }

  .arena-formation-row__position-select {
    grid-column: 2;
  }

  .arena-defense-actions {
    align-items: flex-start;
    flex-direction: column;
  }

  .arena-defense-actions > div {
    width: 100%;
    justify-content: flex-end;
  }

  .arena-history-row {
    grid-template-columns: 32px minmax(0, 1fr) 76px;
  }

  .arena-history-row__settlement {
    grid-column: 2 / 3;
    grid-row: 2;
    display: flex;
    flex-wrap: wrap;
    gap: 10px;
    text-align: left;
  }

  .arena-shop-offer {
    grid-template-columns: minmax(0, 1fr) auto;
  }

  .arena-shop-offer__cost {
    grid-column: 1;
    grid-row: 2;
    text-align: left;
  }
}

@media (max-width: 560px) {
  .arena-command-bar {
    border-radius: 4px;
  }

  .arena-rating-block,
  .arena-command-stat {
    min-height: 54px;
    padding-block: 8px;
  }

  .arena-target__header {
    align-items: flex-start;
  }

  .arena-target__signals {
    position: absolute;
    right: 10px;
    bottom: 8px;
  }

  .arena-target__identity {
    position: relative;
    width: 100%;
    padding-bottom: 18px;
  }

  .arena-lineup-board,
  .arena-preview-lanes {
    grid-template-columns: minmax(0, 1fr);
    min-height: 0;
  }

  .arena-lineup-lane {
    min-height: 62px;
    display: grid;
    grid-template-columns: 46px minmax(0, 1fr);
    align-items: start;
    gap: 8px;
    padding: 8px 10px;
    border-right: 0;
    border-bottom: 1px solid var(--arena-line);
    text-align: left;
  }

  .arena-lineup-lane:last-child {
    border-bottom: 0;
  }

  .arena-lineup-lane__position {
    grid-column: 1;
    padding-top: 7px;
  }

  .arena-lineup-lane__members,
  .arena-lineup-lane__empty {
    grid-column: 2;
  }

  .arena-lineup-lane__empty {
    min-height: 46px;
    grid-template-columns: 30px minmax(0, 1fr);
    justify-items: start;
    text-align: left;
  }

  .arena-lineup-lane__skills :deep(.el-tag) {
    max-width: 46%;
  }

  .arena-preview-lane {
    min-height: 58px;
    display: grid;
    grid-template-columns: 48px minmax(0, 1fr);
    align-items: start;
    gap: 8px;
    padding: 8px 10px;
    border-right: 0;
    border-bottom: 1px solid var(--arena-line);
    text-align: left;
  }

  .arena-preview-lane:last-child {
    border-bottom: 0;
  }

  .arena-preview-lane > span:first-child {
    grid-column: 1;
    padding-top: 10px;
  }

  .arena-preview-lane__members,
  .arena-preview-lane__empty {
    grid-column: 2;
  }

  .arena-preview-lane__empty {
    min-height: 44px;
    grid-template-columns: 36px minmax(0, 1fr);
    justify-items: start;
    text-align: left;
  }

  .arena-history-row {
    grid-template-columns: 30px minmax(0, 1fr) auto;
    gap: 8px;
    padding-block: 11px;
  }

  .arena-history-row__title {
    align-items: flex-start;
    flex-wrap: wrap;
  }

  .arena-history-row__meta {
    flex-wrap: wrap;
  }

  .arena-history-row__actions {
    align-self: start;
  }

  .arena-format-tabs {
    gap: 14px;
  }

  .arena-format-options {
    grid-template-columns: minmax(0, 1fr);
  }

  .arena-format-options button {
    min-height: 58px;
  }
}

@media (max-width: 420px) {
  .arena-target__header,
  .arena-target__footer {
    display: grid;
    grid-template-columns: minmax(0, 1fr) auto;
  }

  .arena-target__identity {
    overflow: hidden;
  }

  .arena-target-filters {
    display: grid;
    grid-template-columns: minmax(0, 1fr);
  }

  .arena-target__bounty {
    max-width: 72px;
  }

  .arena-target__footer > div {
    min-width: 0;
    text-align: left;
  }

  .arena-target__footer > .el-button {
    max-width: 88px;
    padding-inline: 12px;
  }

  .arena-lineup-lane strong,
  .arena-preview-lane strong {
    font-size: 0.7rem;
  }

  .arena-lineup-lane {
    grid-template-columns: 42px 30px minmax(0, 1fr) auto;
    padding-inline: 8px;
  }

  .arena-lineup-lane__skills {
    max-width: 110px;
  }

  .arena-lineup-lane__hp {
    font-size: 0.61rem;
  }

  .arena-target__header,
  .arena-target__footer {
    padding-inline: 9px;
  }

  .arena-target__footer {
    padding-right: 58px;
  }

  .arena-defense-actions > div {
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
    box-sizing: border-box;
    padding-right: 58px;
  }

  .arena-defense-actions > div > :only-child {
    grid-column: 1 / -1;
  }
}

@media (prefers-reduced-motion: reduce) {
  .arena-target {
    transition: none;
  }
}
</style>

<style>
.arena-alchemy-gate-dialog {
  width: min(560px, calc(100vw - 28px));
}

.arena-alchemy-gate {
  display: grid;
  gap: 8px;
  max-height: min(58vh, 520px);
  overflow-y: auto;
  padding-right: 4px;
}

.arena-alchemy-gate__hint {
  margin: 0 0 2px;
  color: var(--app-muted);
  line-height: 1.5;
}

.arena-alchemy-gate__row {
  display: grid;
  gap: 3px;
  padding: 8px 0;
  border-bottom: 1px solid var(--app-border);
}

.arena-alchemy-gate__row strong,
.arena-alchemy-gate__row span,
.arena-alchemy-gate__row small {
  overflow-wrap: anywhere;
}

.arena-alchemy-gate__row small,
.arena-alchemy-gate__more {
  color: var(--app-muted);
}
</style>
