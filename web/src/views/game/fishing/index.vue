<template>
  <div class="page-shell fishing-page">
    <GamePageFrame
      title-key="game.fishing.title"
      tutorial-id="fishing"
      :visual-src="fishingVisual.src"
      :visual-tone="fishingVisual.tone"
      visual-key="fishing"
      body-padding="10px 20px 24px"
      mobile-body-padding="6px 10px 92px"
      data-tutorial="fishing-page"
    >
      <template #actions>
        <el-dropdown
          class="fishing-angler-dropdown"
          trigger="click"
          :disabled="Boolean(activeTrip)"
          @command="selectAnglerCommand"
        >
          <button
            type="button"
            class="fishing-angler-control"
            :disabled="Boolean(activeTrip)"
            :aria-label="t('game.fishing.anglerControlLabel', { name: selectedAnglerName, level: selectedFishingLevel })"
          >
            <span class="fishing-angler-avatar fishing-angler-control__avatar">
              <Icon v-if="selectedAngler?.actorType !== 'kitty'" icon="lucide:user-round" />
              <template v-else>{{ kittyAvatar(selectedAnglerName) }}</template>
            </span>
            <span class="fishing-angler-control__identity">
              <small>{{ t("game.fishing.currentAngler") }}</small>
              <strong>{{ selectedAnglerName }}</strong>
            </span>
            <span class="fishing-angler-control__level"><small>{{ t("game.fishing.levelShort") }}</small><strong>{{ selectedFishingLevel }}</strong></span>
            <Icon v-if="!activeTrip" icon="lucide:chevron-down" class="fishing-angler-control__chevron" />
          </button>
          <template #dropdown>
            <el-dropdown-menu class="fishing-angler-menu">
              <el-dropdown-item
                v-for="angler in anglerOptions"
                :key="anglerKey(angler.actorType, angler.actorId)"
                :command="anglerKey(angler.actorType, angler.actorId)"
                :disabled="anglerUnavailable(angler)"
              >
                <span class="fishing-angler-menu__avatar">
                  <Icon v-if="angler.actorType === 'player'" icon="lucide:user-round" />
                  <template v-else>{{ kittyAvatar(anglerName(angler)) }}</template>
                </span>
                <span class="fishing-angler-menu__copy"><strong>{{ anglerName(angler) }}</strong><small>{{ anglerStatusCopy(angler) }}</small></span>
                <span class="fishing-angler-menu__level">{{ t("game.fishing.level", { level: angler.fishingLevel }) }}</span>
                <Icon v-if="anglerKey(angler.actorType, angler.actorId) === selectedAnglerKey" icon="lucide:check" />
              </el-dropdown-item>
            </el-dropdown-menu>
          </template>
        </el-dropdown>
        <el-button circle size="small" :aria-label="t('game.fishing.refresh')" :loading="refreshing || (activeTab === 'history' && historyLoading)" @click="refreshPage">
          <Icon icon="lucide:refresh-cw" />
        </el-button>
      </template>

      <el-tabs v-model="activeTab" class="fishing-tabs" data-tutorial="fishing-tabs">
        <el-tab-pane :label="t('game.fishing.navWaters')" name="waters">
          <OperationEffectNotice :context="{ domain: 'fishing_cast', actionId: 'fishing.trip', actorKind: tripActorType, actorId: tripActorId }" />
          <div v-if="loading && !overview" class="fishing-loading">
            <Icon icon="lucide:loader-circle" />
            <span>{{ t("game.fishing.loading") }}</span>
          </div>

          <section v-else-if="activeTrip" class="fishing-live" data-tutorial="fishing-waters">
            <div
              class="fishing-live-water"
              :class="[
                `phase-${activeTrip.phase}`,
                `event-${liveSceneEvent?.kind || 'none'}`,
                `weather-${liveEnvironment.weather}`,
                `period-${liveEnvironment.period}`
              ]"
            >
              <FishingWaterBackdrop :water-id="activeTrip.waterId" :environment="liveEnvironment" />
              <svg class="fishing-water-traces" viewBox="0 0 100 100" preserveAspectRatio="none" aria-hidden="true">
                <path d="M -4 18 C 12 11, 24 24, 40 17 S 70 11, 104 20" />
                <path d="M -8 38 C 9 31, 22 45, 39 37 S 73 30, 108 40" />
                <path d="M -5 60 C 17 52, 30 67, 49 59 S 79 53, 105 63" />
                <path d="M -10 82 C 11 76, 25 88, 44 81 S 76 75, 110 85" />
              </svg>
              <div class="fishing-live-water__head">
                <span><Icon :icon="liveEnvironment.weather === 'unknown' ? 'lucide:cloud' : liveEnvironment.celestial === 'moon' ? 'lucide:moon' : weatherIcon(currentForecast?.now.weather)" />{{ liveEnvironment.weather === 'unknown' ? '--' : weatherLabel(currentForecast?.now.weather) }}</span>
                <strong>{{ currentSpot ? spotName(currentSpot) : "" }}</strong>
                <span class="fishing-live-water__time"><Icon :icon="livePeriodIcon" /><span v-if="liveEnvironment.period !== 'unknown'">{{ t(`game.fishing.scenePeriod_${liveEnvironment.period}`) }}</span><time>{{ liveEnvironment.period === 'unknown' ? '--:--' : formatGameHour(currentForecast?.now.gameHour) }}</time></span>
              </div>
              <svg class="fishing-cast-line" viewBox="0 0 100 100" preserveAspectRatio="none" aria-hidden="true">
                <path d="M 96 -4 C 91 18, 77 34, 61 50" />
              </svg>
              <img src="/assets/icons/item/reed_float.png" alt="" class="fishing-live-bobber" aria-hidden="true" />
              <div class="fishing-ripples" aria-hidden="true"><i /><i /><i /><i /></div>
              <div class="fishing-live-water__phase" aria-live="polite">
                <span class="fishing-live-water__phase-copy">
                  <span class="fishing-live-water__phase-label"><Icon :icon="phaseIcon(activeTrip.phase)" />{{ phaseCopy(activeTrip.phase) }}</span>
                  <strong>{{ liveHeadline }}</strong>
                  <span>{{ liveStatusCopy }}</span>
                </span>
              </div>
              <div class="fishing-live-water__end"><Icon icon="lucide:clock-3" /><span>{{ tripEndCopy(activeTrip) }}</span></div>
            </div>

            <aside class="fishing-live-panel">
              <header>
                <div><span>{{ currentWater ? waterName(currentWater) : "" }} · {{ activeAnglerName }}</span><h2>{{ phaseCopy(activeTrip.phase) }}</h2></div>
                <el-tag effect="plain" type="success">{{ t("game.fishing.tripRunning") }}</el-tag>
              </header>
              <dl class="fishing-live-signals">
                <div><dt><Icon icon="lucide:circle-dot-dashed" />{{ t("game.fishing.currentStage") }}</dt><dd>{{ phaseCopy(activeTrip.phase) }}</dd></div>
                <div><dt><Icon icon="lucide:gauge" />{{ t("game.fishing.lineState") }}</dt><dd :class="`line-${activeTrip.currentFish?.lineState || 'steady'}`">{{ lineStateCopy(activeTrip.currentFish?.lineState) }}</dd></div>
                <div>
                  <dt><Icon icon="lucide:activity" />{{ t("game.fishing.fishStamina") }}</dt>
                  <dd class="fishing-stamina">
                    <template v-if="activeTrip.currentFish"><span>{{ liveFishStamina }}%</span><el-progress :percentage="liveFishStamina" :show-text="false" :stroke-width="7" /></template>
                    <span v-else class="fishing-stamina__empty">{{ t("game.fishing.noFishOnLine") }}</span>
                  </dd>
                </div>
              </dl>
              <section class="fishing-live-log">
                <header>
                  <h3>{{ t("game.fishing.liveLog") }}</h3>
                  <span><Icon icon="lucide:clock-3" />{{ t("game.fishing.liveLogContinuesOffline") }}</span>
                </header>
                <ol v-if="liveTimelineEvents.length" ref="liveLogEl" role="log" aria-live="polite">
                  <li v-for="event in liveTimelineEvents" :key="event.sequence" :class="`is-${eventTone(event)}`">
                    <span class="fishing-live-log__marker">
                      <FishMutationArt v-if="event.kind === 'landed' && eventSpecies(event)" :src="eventSpecies(event)?.iconPath || ''" :fish="{ mutations: event.mutations }" :mutations="overview?.mutations || []" width="27px" />
                      <Icon v-else :icon="eventIcon(event)" />
                    </span>
                    <span class="fishing-live-log__copy"><strong>{{ eventTitle(event) }}</strong><small>{{ eventDetail(event) }}</small></span>
                    <time :datetime="event.at">{{ eventTime(event.at) }}</time>
                  </li>
                </ol>
                <p v-else>{{ t("game.fishing.liveLogWaiting") }}</p>
              </section>
              <section class="fishing-live-supplies">
                <h3>{{ t("game.fishing.remainingSupplies") }}</h3>
                <div class="fishing-live-supply-grid">
                  <Card
                    v-for="row in liveSupplyRows"
                    :key="row.key"
                    :item-id="row.itemId"
                    :quantity="row.quantity"
                    show-zero-quantity
                    :width="58"
                    :height="58"
                    :icon-size="40"
                  />
                </div>
              </section>
              <details class="fishing-trip-details">
                <summary>{{ t("game.fishing.tripDetails") }}</summary>
                <div class="fishing-summary-grid"><span><b>{{ activeTrip.summary.landed || 0 }}</b>{{ t("game.fishing.landed") }}</span><span><b>{{ activeTrip.summary.released || 0 }}</b>{{ t("game.fishing.released") }}</span><span><b>{{ formatWeight(activeTrip.summary.totalWeightKg || 0) }}</b>{{ t("game.fishing.totalWeight") }}</span><span><b>{{ activeTrip.summary.baitUsed || 0 }}</b>{{ t("game.fishing.baitUsed") }}</span></div>
              </details>
              <el-button plain type="danger" class="fishing-stop" :loading="stoppingTrip" :disabled="stoppingTrip" @click="confirmStopTrip"><Icon icon="lucide:square" />{{ t("game.fishing.stop") }}</el-button>
            </aside>
          </section>

          <template v-else-if="overview">
            <section class="fishing-status-strip" data-tutorial="fishing-waters">
              <div><Icon icon="lucide:clock-3" /><span>{{ t("game.fishing.currentTime") }}</span><strong>{{ formatGameHour(selectedForecast?.now.gameHour) }}</strong></div>
              <div><Icon :icon="weatherIcon(selectedForecast?.now.weather)" /><span>{{ t("game.fishing.weather") }}</span><strong>{{ weatherLabel(selectedForecast?.now.weather) }}</strong></div>
              <div><Icon icon="lucide:thermometer" /><span>{{ t("game.fishing.waterTemperature") }}</span><strong>{{ Number(selectedForecast?.now.temperatureC || 0).toFixed(1) }}°C</strong></div>
              <div><Icon icon="lucide:fish" /><span>{{ t("game.fishing.currentFishActivity") }}</span><strong>{{ activityCopy(selectedForecast?.now.activityBand) }}</strong></div>
            </section>

            <section class="fishing-water-switch" :aria-label="t('game.fishing.waters')">
              <button v-for="water in overview.waters" :key="water.id" type="button" :disabled="!waterUnlocked(water)" :class="{ 'is-selected': selectedWaterId === water.id }" @click="selectWater(water.id)">
                <Icon :icon="waterIcon(water.waterKind)" />
                <span><strong>{{ waterName(water) }}</strong><small v-if="waterUnlocked(water)">{{ activityCopy(forecastForWater(water.id)?.now.activityBand) }}</small><small v-else>{{ t("game.fishing.anglerUnlockProgress", { name: selectedAnglerName, current: selectedFishingLevel, required: water.requiredFishingLevel }) }}</small></span>
                <Icon v-if="!waterUnlocked(water)" icon="lucide:lock" />
              </button>
            </section>

            <section class="fishing-water-layout">
              <div class="fishing-spot-column">
                <header class="fishing-section-head"><div><span>{{ t("game.fishing.chooseSpot") }}</span><h2>{{ selectedWater ? waterName(selectedWater) : t("game.fishing.chooseWater") }}</h2></div></header>
                <div class="fishing-spot-list">
                  <button v-for="spot in selectedWaterSpots" :key="spot.id" type="button" :class="{ 'is-selected': selectedSpotId === spot.id }" @click="selectedSpotId = spot.id">
                    <span class="fishing-spot-marker"><i /></span>
                    <span class="fishing-spot-main"><strong>{{ spotName(spot) }}</strong><small>{{ spotMethodCopy(spot) }} · {{ depthCopy(spot) }}</small><small class="fishing-spot-fish"><Icon icon="lucide:fish-symbol" />{{ spotSpeciesSummary(spot) }}</small><em><Icon icon="lucide:users" />{{ anglersCopy(spot.id) }}</em></span>
                    <span class="fishing-spot-forecast"><b :class="`is-${forecastBySpot(spot.id)?.now.activityBand || 'steady'}`">{{ activityCopy(forecastBySpot(spot.id)?.now.activityBand) }}</b><small><Icon :icon="trendIcon(forecastBySpot(spot.id)?.now.trend)" />{{ trendCopy(forecastBySpot(spot.id)?.now.trend) }}</small></span>
                  </button>
                </div>
              </div>

              <aside v-if="selectedSpot" class="fishing-spot-detail">
                <header><div><span>{{ selectedWater ? waterName(selectedWater) : "" }}</span><h2>{{ spotName(selectedSpot) }}</h2></div><el-button circle :aria-label="t('game.fishing.ecologyDetails')" @click="ecologyDialogOpen = true"><Icon icon="lucide:info" /></el-button></header>
                <p>{{ selectedWater ? waterDescription(selectedWater) : "" }}</p>
                <dl><div><dt>{{ t("game.fishing.currentFishActivity") }}</dt><dd>{{ activityCopy(selectedForecast?.now.activityBand) }}</dd></div><div><dt>{{ t("game.fishing.method") }}</dt><dd>{{ spotMethodCopy(selectedSpot) }}</dd></div><div><dt>{{ t("game.fishing.waterLayer") }}</dt><dd>{{ depthCopy(selectedSpot) }}</dd></div><div><dt>{{ t("game.fishing.weatherTrend") }}</dt><dd><Icon :icon="trendIcon(selectedForecast?.now.trend)" />{{ trendCopy(selectedForecast?.now.trend) }}</dd></div><div><dt>{{ t("game.fishing.anglersHere") }}</dt><dd>{{ anglersCopy(selectedSpot.id) }}</dd></div></dl>
                <section class="fishing-spot-species">
                  <header><h3>{{ t("game.fishing.spotFish") }}</h3><span>{{ t("game.fishing.spotFishCount", { count: selectedSpotSpecies.length }) }}</span></header>
                  <div class="fishing-spot-species__list">
                    <div v-for="row in selectedSpotSpecies" :key="row.species.id">
                      <img :src="row.species.iconPath" :alt="speciesName(row.species.id)" />
                      <span><strong>{{ speciesName(row.species.id) }}</strong><small>{{ rarityLabel(row.species.rarity) }} · {{ speciesLayerSummary(row.species) }}</small></span>
                      <b>{{ t("game.fishing.spotFishBaseShare", { value: formatSpotShare(row.share) }) }}</b>
                    </div>
                  </div>
                  <p>{{ t("game.fishing.spotFishHint") }}</p>
                </section>
                <el-button v-if="!isMobile" type="primary" class="fishing-main-action" data-tutorial="fishing-trip-start" @click="openTripDialog"><Icon icon="lucide:play" />{{ t("game.fishing.setOut") }}</el-button>
                <el-affix v-else class="fishing-departure-affix" position="bottom" :offset="8" :z-index="55" teleported>
                  <div class="fishing-departure-dock">
                    <el-button type="primary" class="fishing-main-action" data-tutorial="fishing-trip-start" @click="openTripDialog"><Icon icon="lucide:play" />{{ t("game.fishing.setOut") }}</el-button>
                  </div>
                </el-affix>
              </aside>
            </section>
          </template>
        </el-tab-pane>

        <el-tab-pane :label="t('game.fishing.navLoadouts')" name="loadouts">
          <section class="fishing-tab-toolbar fishing-tab-toolbar--end" data-tutorial="fishing-loadouts">
            <el-button type="primary" @click="openLoadoutEditor()"><Icon icon="lucide:plus" />{{ t("game.fishing.newLoadout") }}</el-button>
          </section>
          <div v-if="loadouts.length" class="fishing-loadout-list">
            <article v-for="loadout in loadouts" :key="loadout.loadoutId">
              <img :src="loadoutRodIcon(loadout)" alt="" />
              <div><span>{{ methodLabel(loadout.method) }}</span><h3>{{ loadoutName(loadout) }}</h3><p>{{ loadoutComponentSummary(loadout) }}</p></div>
              <dl><div><dt>{{ t("game.fishing.strategy") }}</dt><dd>{{ strategyLabel(loadout.strategy) }}</dd></div></dl>
              <div class="fishing-row-actions"><el-button circle :aria-label="t('game.fishing.edit')" @click="openLoadoutEditor(loadout)"><Icon icon="lucide:pencil" /></el-button><el-button circle :aria-label="t('game.fishing.delete')" @click="removeLoadout(loadout)"><Icon icon="lucide:trash-2" /></el-button></div>
            </article>
          </div>
          <section v-else class="fishing-empty"><img src="/assets/icons/item/basic_fishing_rod.png" alt="" /><h2>{{ t("game.fishing.firstLoadout") }}</h2><p>{{ t("game.fishing.firstLoadoutHint") }}</p><el-button type="primary" @click="openLoadoutEditor()">{{ t("game.fishing.buildLoadout") }}</el-button></section>
        </el-tab-pane>

        <el-tab-pane :label="t('game.fishing.navCatches')" name="catches">
          <section class="fishing-tab-toolbar fishing-catch-toolbar" data-tutorial="fishing-catches">
            <el-segmented v-model="catchSection" class="fishing-subnav" :options="catchSections" />
            <div class="fishing-heading-actions">
              <el-tooltip v-if="catchSection === 'collection'" :content="batchSellMode ? t('game.fishing.exitBatchSale') : t('game.fishing.batchSale')" placement="bottom">
                <el-button circle :type="batchSellMode ? 'primary' : 'default'" :aria-label="batchSellMode ? t('game.fishing.exitBatchSale') : t('game.fishing.batchSale')" :aria-pressed="batchSellMode" @click="toggleBatchSaleMode"><Icon icon="lucide:badge-dollar-sign" /></el-button>
              </el-tooltip>
              <el-tooltip v-if="catchSection === 'collection'" :content="batchPondMode ? t('game.fishing.exitBatchPond') : t('game.fishing.batchStockPond')" placement="bottom">
                <el-button circle :type="batchPondMode ? 'primary' : 'default'" :loading="loadingPondSelection" :aria-label="batchPondMode ? t('game.fishing.exitBatchPond') : t('game.fishing.batchStockPond')" :aria-pressed="batchPondMode" @click="toggleBatchPondMode"><Icon icon="game-icons:fish-eggs" /></el-button>
              </el-tooltip>
              <el-tooltip v-if="catchSection === 'collection'" :content="t('game.fishing.filter')" placement="bottom">
                <el-button circle :type="catchFiltersActive ? 'primary' : undefined" :aria-label="t('game.fishing.filter')" @click="filterDialogOpen = true"><Icon icon="lucide:list-filter" /></el-button>
              </el-tooltip>
              <el-tooltip :content="t('game.fishing.refresh')" placement="bottom">
                <el-button circle :aria-label="t('game.fishing.refresh')" :loading="catchesLoading" @click="loadCatchData()"><Icon icon="lucide:refresh-cw" /></el-button>
              </el-tooltip>
            </div>
          </section>
          <template v-if="catchSection === 'collection'">
            <ItemBrowserPanel
              v-if="displayCatches.length"
              v-loading="catchesLoading"
              :items="catchBrowserItems"
              :empty-text="t('game.fishing.noCatches')"
              :show-toolbar="false"
              :show-search="false"
              :group-all-by-type="false"
              :secondary-category-toggle-visible="false"
              :virtualized="true"
              :grid-min-width="isMobile ? 56 : 64"
              :grid-gap-class="isMobile ? 'gap-2' : 'gap-4'"
              tutorial-scope="fishing-catches"
            >
              <template #item="{ item }">
                <article
                  class="fishing-catch-item mt-2"
                  :class="{
                    'is-batch': catchSelectionMode,
                    'is-selected': selectedCatchIds.has(item.instanceId || 0),
                    'is-unavailable': catchSelectionMode && !canSelectCatch(item.payload)
                  }"
                  role="button"
                  :tabindex="catchSelectionMode && !canSelectCatch(item.payload) ? -1 : 0"
                  :aria-label="catchCardAriaLabel(item.payload)"
                  :aria-pressed="catchSelectionMode ? selectedCatchIds.has(item.instanceId || 0) : undefined"
                  :aria-disabled="catchSelectionMode && !canSelectCatch(item.payload) ? 'true' : undefined"
                  @click="handleCatchCardClick(item.payload)"
                  @keydown.enter.prevent="handleCatchCardClick(item.payload)"
                  @keydown.space.prevent="handleCatchCardClick(item.payload)"
                >
                  <span v-if="hasRecord(item.payload)" class="fishing-catch-record" :title="t('game.fishing.record')"><Icon icon="lucide:medal" /></span>
                  <span v-if="catchMutations(item.payload).length" class="fishing-catch-mutations" role="list">
                    <span
                      v-for="mutation in catchMutations(item.payload)"
                      :key="mutation.id"
                      role="listitem"
                      :class="`is-${mutation.visualEffect}`"
                      :title="t(mutation.nameKey)"
                      :aria-label="t(mutation.nameKey)"
                    ><Icon :icon="mutation.badgeIcon" aria-hidden="true" /></span>
                  </span>
                  <span v-if="catchSelectionMode && selectedCatchIds.has(item.instanceId || 0)" class="fishing-catch-check"><Icon icon="lucide:check" /></span>
                  <span v-else-if="isImmatureCulturedCatch(item.payload)" class="fishing-catch-immature" :title="saleIneligibleCopy(item.payload)" :aria-label="saleIneligibleCopy(item.payload)"><Icon icon="lucide:hourglass" /></span>
                  <Card
                    item-id="fish_catch"
                    :instance-id="item.instanceId || null"
                    :instance-preview="{
                      fishData: item.payload.fish,
                      saleEligible: item.payload.saleEligible,
                      saleIneligibleReason: item.payload.saleIneligibleReason,
                      saleCurrentAgeDays: item.payload.saleCurrentAgeDays,
                      saleRequiredAgeDays: item.payload.saleRequiredAgeDays
                    }"
                    :width="isMobile ? 48 : 56"
                    :height="isMobile ? 48 : 56"
                    :icon-size="isMobile ? 28 : 32"
                  />
                </article>
              </template>
            </ItemBrowserPanel>
            <section v-else class="fishing-empty compact" v-loading="catchesLoading">
              <Icon :icon="catchFiltersActive ? 'lucide:search-x' : 'lucide:fish-off'" />
              <h2>{{ t(catchFiltersActive ? "game.fishing.fishSelectorEmpty" : "game.fishing.noCatches") }}</h2>
              <el-button v-if="catchFiltersActive" @click="catchFilters = defaultFishFilters('recent'); applyCatchFilters()">{{ t("game.fishing.fishSelectorReset") }}</el-button>
              <p v-else>{{ t("game.fishing.noCatchesHint") }}</p>
            </section>
            <BatchActionBar
              :visible="catchSelectionMode && activeTab === 'catches' && catchSection === 'collection'"
              tutorial-id="fishing-catch-action-bar"
            >
              <template #hint>
                {{ t("game.fishing.selectedFish", { count: selectedCatchIds.size }) }}
                <template v-if="batchSellMode && saleImmatureCatchCount"> · {{ t("game.fishing.saleImmatureCount", { count: saleImmatureCatchCount }) }}</template>
                <template v-if="batchSellMode && saleOtherUnavailableCatchCount"> · {{ t("game.fishing.saleUnavailableCount", { count: saleOtherUnavailableCatchCount }) }}</template>
                <template v-if="batchPondMode"> · {{ t("game.fishing.pondSelectionHint", { count: remainingPondSlots }) }}</template>
              </template>
              <el-button
                v-if="batchSameSpeciesSelectVisible"
                size="small"
                type="primary"
                plain
                :disabled="catchActionSubmitting"
                @click="selectAllFocusedCatchSpecies"
              >
                {{ batchSameSpeciesSelectLabel }}
              </el-button>
              <el-button size="small" :disabled="catchActionSubmitting" @click="exitCatchActionMode">
                {{ batchPondMode ? t("game.fishing.exitBatchPond") : t("game.fishing.exitBatchSale") }}
              </el-button>
              <el-button
                size="small"
                type="primary"
                :disabled="selectedCatchIds.size === 0"
                :loading="catchActionSubmitting"
                @click="batchSellMode ? sellSelectedCatches() : stockSelectedCatches()"
              >
                {{ batchSellMode ? t("game.fishing.getEstimate") : t("game.fishing.pondConfirmStock") }}
                <template v-if="selectedCatchIds.size > 0"> ({{ selectedCatchIds.size }})</template>
              </el-button>
            </BatchActionBar>
          </template>
          <div v-else-if="catchSection === 'journal'" class="fishing-journal-grid">
            <article v-for="species in overview?.species || []" :key="species.id" :class="{ 'is-unknown': !knowledgeBySpecies(species.id)?.catches }">
              <img :src="species.iconPath" :alt="knowledgeBySpecies(species.id)?.catches ? speciesName(species.id) : ''" />
              <div>
                <span>{{ species.fantasy ? t("game.fishing.strangeFish") : rarityLabel(species.rarity) }}</span>
                <h3>{{ knowledgeBySpecies(species.id)?.catches ? speciesName(species.id) : t("game.fishing.unknownFish") }}</h3>
                <p>{{ knowledgeBySpecies(species.id)?.catches ? t("game.fishing.caughtCount", { count: knowledgeBySpecies(species.id)?.catches || 0 }) : t("game.fishing.notDiscovered") }}</p>
                <template v-if="knowledgeBySpecies(species.id)?.catches">
                  <div class="fishing-species-needs">
                    <span><Icon icon="lucide:layers-3" />{{ t("game.fishing.preferredLayer") }} · {{ speciesLayerSummary(species) }}</span>
                    <span><Icon icon="lucide:crosshair" />{{ t("game.fishing.castingNeed") }} · {{ challengeCopy(species.castingChallenge) }}</span>
                    <span><Icon icon="game-icons:fishing-hook" />{{ t("game.fishing.hookingNeed") }} · {{ challengeCopy(species.hookingChallenge) }}</span>
                  </div>
                  <div v-if="pondSpeciesProfile(species.id)" class="fishing-species-ecology">
                    <span><Icon icon="lucide:wheat" />{{ pondDietLabel(species.id) }}</span>
                    <span v-if="pondSpeciesProfile(species.id)?.predation" class="is-predator"><Icon icon="lucide:utensils" />{{ pondPredatorGuide(species.id) }}</span>
                  </div>
                </template>
              </div>
            </article>
          </div>
          <template v-else>
            <section class="fishing-record-controls"><el-select v-model="recordSpeciesId" :placeholder="t('game.fishing.chooseSpecies')" filterable @change="loadLeaderboard"><el-option v-for="species in overview?.species || []" :key="species.id" :label="speciesName(species.id)" :value="species.id" /></el-select><el-segmented v-model="recordMetric" :options="recordMetrics" @change="loadLeaderboard" /></section>
            <section v-if="knowledgeBySpecies(recordSpeciesId)?.catches" class="fishing-personal-record"><span>{{ t("game.fishing.personalBest") }}</span><div><small>{{ t("game.fishing.longest") }}</small><strong>{{ formatLength(knowledgeBySpecies(recordSpeciesId)?.bestLengthCm || 0) }}<FishSizeIndicator metric="length" :value="knowledgeBySpecies(recordSpeciesId)?.bestLengthCm || 0" :species="speciesById(recordSpeciesId)" /></strong></div><div><small>{{ t("game.fishing.heaviest") }}</small><strong>{{ formatWeight(knowledgeBySpecies(recordSpeciesId)?.bestWeightKg || 0) }}<FishSizeIndicator metric="weight" :value="knowledgeBySpecies(recordSpeciesId)?.bestWeightKg || 0" :species="speciesById(recordSpeciesId)" /></strong></div></section>
            <div class="fishing-record-board"><div v-for="row in leaderboard" :key="`${row.rank}-${row.userId}`"><span>{{ row.rank }}</span><b>{{ row.playerName }}</b><strong>{{ recordMetric === 'length' ? formatLength(row.value) : formatWeight(row.value) }}<FishSizeIndicator :metric="recordMetric" :value="row.value" :species="speciesById(recordSpeciesId)" /></strong></div><section v-if="!leaderboard.length" class="fishing-empty compact"><Icon icon="lucide:medal" /><h2>{{ t("game.fishing.noRecords") }}</h2></section></div>
          </template>
        </el-tab-pane>

        <el-tab-pane :label="t('game.fishing.navHistory')" name="history">
          <section class="fishing-history" data-tutorial="fishing-history">
            <header class="fishing-section-head fishing-history__head">
              <div><span>{{ t("game.fishing.historyEyebrow") }}</span><h2>{{ t("game.fishing.historyTitle", { count: tripHistoryLimit }) }}</h2></div>
              <span v-if="historyLoaded" class="fishing-history__count">{{ t("game.fishing.historyCount", { count: tripHistory.length }) }}</span>
            </header>

            <div v-if="historyLoading && !historyLoaded" class="fishing-loading">
              <Icon icon="lucide:loader-circle" />
              <span>{{ t("game.fishing.historyLoading") }}</span>
            </div>
            <section v-else-if="historyLoaded && !tripHistory.length" class="fishing-empty compact">
              <Icon icon="lucide:history" />
              <h2>{{ t("game.fishing.historyEmpty") }}</h2>
              <p>{{ t("game.fishing.historyEmptyHint") }}</p>
            </section>
            <ol v-else class="fishing-history-list">
              <li v-for="trip in tripHistory" :key="trip.tripId">
                <details class="fishing-history-trip" @toggle="handleHistoryTripToggle($event, trip)">
                  <summary>
                    <span class="fishing-history-trip__status" :class="`is-${trip.status}`">
                      <Icon v-if="trip.status === 'stopped'" icon="lucide:square" />
                      <Icon v-else icon="lucide:check" />
                    </span>
                    <span class="fishing-history-trip__identity">
                      <strong>{{ spotNameById(trip.spotId) }}</strong>
                      <small>{{ waterNameById(trip.waterId) }} · {{ tripAnglerName(trip) }} · {{ historyStatusCopy(trip) }}</small>
                      <time :datetime="trip.startedAt">{{ formatTripStartedAt(trip.startedAt) }}</time>
                    </span>
                    <span class="fishing-history-trip__result">
                      <strong>{{ t("game.fishing.historyCatchCount", { count: trip.summary.landed || 0 }) }}</strong>
                      <small>{{ formatWeight(trip.summary.totalWeightKg || 0) }}</small>
                    </span>
                    <Icon icon="lucide:chevron-down" class="fishing-history-trip__chevron" />
                  </summary>
                  <div class="fishing-history-trip__body">
                    <div class="fishing-history-trip__meta">
                      <span><Icon icon="lucide:clock-3" />{{ formatTripDuration(trip) }}</span>
                      <span><Icon icon="lucide:gauge" />{{ strategyLabel(trip.strategy) }}</span>
                      <span><Icon icon="lucide:package-open" />{{ t("game.fishing.baitUsed") }} {{ trip.summary.baitUsed || 0 }}</span>
                    </div>
                    <dl class="fishing-history-stats">
                      <div><dt>{{ t("game.fishing.historyCasts") }}</dt><dd>{{ trip.summary.casts || 0 }}</dd></div>
                      <div><dt>{{ t("game.fishing.historyBites") }}</dt><dd>{{ trip.summary.bites || 0 }}</dd></div>
                      <div><dt>{{ t("game.fishing.landed") }}</dt><dd>{{ trip.summary.landed || 0 }}</dd></div>
                      <div><dt>{{ t("game.fishing.kept") }}</dt><dd>{{ trip.summary.kept || 0 }}</dd></div>
                      <div><dt>{{ t("game.fishing.released") }}</dt><dd>{{ trip.summary.released || 0 }}</dd></div>
                      <div><dt>{{ t("game.fishing.historyEscaped") }}</dt><dd>{{ (trip.summary.escaped || 0) + (trip.summary.lineBreaks || 0) }}</dd></div>
                    </dl>
                    <section class="fishing-history-events">
                      <header>
                        <strong>{{ t("game.fishing.historyEvents") }}</strong>
                        <span
                          v-if="historyEventState(trip.tripId)?.loaded"
                          :class="{ 'is-partial': !historyEventState(trip.tripId)?.historyComplete }"
                        >{{ historyEventStateCopy(trip.tripId) }}</span>
                      </header>
                      <div v-if="historyEventsInitialLoading(trip.tripId)" class="fishing-history-events__loading">
                        <Icon icon="lucide:loader-circle" />
                        <span>{{ t("game.fishing.historyEventsLoading") }}</span>
                      </div>
                      <div v-else-if="historyEventState(trip.tripId)?.error && !historyEventState(trip.tripId)?.loaded" class="fishing-history-events__error">
                        <span>{{ t("game.fishing.historyEventsLoadFailed") }}</span>
                        <el-button text @click="loadHistoryTripEvents(trip)">{{ t("game.fishing.historyEventsRetry") }}</el-button>
                      </div>
                      <ol v-else-if="tripHistoryEvents(trip).length">
                        <li v-for="event in tripHistoryEvents(trip)" :key="event.sequence" :class="`is-${eventTone(event)}`">
                          <span class="fishing-history-events__marker">
                            <FishMutationArt v-if="event.kind === 'landed' && eventSpecies(event)" :src="eventSpecies(event)?.iconPath || ''" :fish="{ mutations: event.mutations }" :mutations="overview?.mutations || []" width="25px" />
                            <Icon v-else :icon="eventIcon(event)" />
                          </span>
                          <span><strong>{{ eventTitle(event) }}</strong><small>{{ eventDetail(event) }}</small></span>
                          <time :datetime="event.at">{{ eventTime(event.at) }}</time>
                        </li>
                      </ol>
                      <p v-else>{{ t("game.fishing.historyNoEvents") }}</p>
                      <footer v-if="historyEventState(trip.tripId)?.nextCursor">
                        <span v-if="historyEventState(trip.tripId)?.error">{{ t("game.fishing.historyEventsLoadFailed") }}</span>
                        <el-button text :loading="historyEventState(trip.tripId)?.loading" @click="loadHistoryTripEvents(trip, true)">
                          <Icon icon="lucide:history" />{{ historyEventState(trip.tripId)?.error ? t("game.fishing.historyEventsRetry") : t("game.fishing.historyEventsLoadOlder") }}
                        </el-button>
                      </footer>
                    </section>
                  </div>
                </details>
              </li>
            </ol>
          </section>
        </el-tab-pane>

        <el-tab-pane :label="t('game.fishing.navTournaments')" name="tournaments">
          <section v-if="tournament" class="fishing-tournament-hero"><div class="fishing-target-fish"><img :src="speciesById(tournament.current.targetSpeciesId)?.iconPath" :alt="speciesName(tournament.current.targetSpeciesId)" /></div><div><span>{{ t("game.fishing.weeklyTarget") }}</span><h2>{{ speciesName(tournament.current.targetSpeciesId) }}</h2><p>{{ t("game.fishing.tournamentEnds", { time: formatTournamentRemaining(tournament.current.endsAt) }) }}</p></div><div class="fishing-token"><Icon icon="lucide:ticket" /><span>{{ t("game.fishing.tokens") }}</span><strong>{{ tournament.tokens }}</strong></div></section>
          <section v-if="tournament?.myEntries?.length" class="fishing-my-entries"><span>{{ t("game.fishing.myEntries") }}</span><div v-for="row in tournament.myEntries" :key="row.division"><small>{{ row.division === 'standard' ? t("game.fishing.standardDivision") : t("game.fishing.openDivision") }} · {{ t("game.fishing.rankValue", { rank: row.rank }) }}</small><strong>{{ formatWeight(row.weightKg) }}<FishSizeIndicator metric="weight" :value="row.weightKg" :species="speciesById(tournament.current.targetSpeciesId)" /></strong></div></section>
          <section class="fishing-tournament-board" data-tutorial="fishing-tournaments">
            <header class="fishing-tournament-toolbar"><strong>{{ t("game.fishing.weeklyTournament") }}</strong><el-segmented v-model="tournamentDivision" :options="tournamentDivisions" /></header>
            <section v-if="tournamentDivision === 'standard'" class="fishing-standard-rules" :class="{ 'is-designated-loadout': tournament.standardLoadout }">
              <header>
                <span><Icon icon="lucide:scale" /><strong>{{ t(tournament.standardLoadout ? "game.fishing.standardRulesTitle" : "game.fishing.legacyStandardRulesTitle") }}</strong></span>
                <small><Icon icon="lucide:users-round" />{{ t(tournament.standardLoadout ? "game.fishing.standardRulesShared" : "game.fishing.legacyStandardRulesShared") }}</small>
              </header>
              <template v-if="tournament.standardLoadout">
                <p class="fishing-standard-method"><Icon icon="lucide:circle-dot-dashed" />{{ t("game.fishing.standardRequiredMethod", { method: methodLabel(tournament.standardLoadout.method) }) }}</p>
                <div class="fishing-standard-tackle">
                  <div v-for="item in tournamentStandardItems" :key="item.slot">
                    <span>{{ item.slotLabel }}</span>
                    <ItemQuickRef :item-id="item.itemId" />
                    <small :class="item.owned ? 'is-owned' : 'is-missing'"><Icon :icon="item.owned ? 'lucide:check' : 'lucide:circle-alert'" />{{ t(item.owned ? "game.fishing.standardTackleOwned" : "game.fishing.standardTackleMissing") }}</small>
                  </div>
                </div>
                <footer class="fishing-standard-status" :class="{ 'is-ready': matchingStandardLoadout }">
                  <span><Icon :icon="matchingStandardLoadout ? 'lucide:badge-check' : 'lucide:circle-alert'" /><strong>{{ matchingStandardLoadout ? t("game.fishing.standardLoadoutReady", { name: loadoutName(matchingStandardLoadout) }) : t("game.fishing.standardLoadoutNotReady") }}</strong></span>
                  <el-button v-if="!matchingStandardLoadout" text @click="activeTab = 'loadouts'"><Icon icon="lucide:wrench" />{{ t("game.fishing.standardLoadoutManage") }}</el-button>
                </footer>
                <p>{{ t("game.fishing.standardDivisionHint") }}</p>
              </template>
              <template v-else>
                <dl v-if="tournamentStandardStats.length">
                  <div v-for="entry in tournamentStandardStats" :key="entry.key"><dt>{{ entry.label }}</dt><dd>{{ entry.value }}</dd></div>
                </dl>
                <p>{{ t("game.fishing.legacyStandardDivisionHint") }}</p>
              </template>
            </section>
            <div class="fishing-record-board"><div v-for="row in tournamentBoard" :key="`${row.division}-${row.userId}`"><span>{{ row.rank }}</span><b>{{ row.playerName }}<small>{{ formatLength(row.lengthCm) }}<FishSizeIndicator metric="length" :value="row.lengthCm" :species="speciesById(tournament?.current.targetSpeciesId || '')" /></small></b><strong>{{ formatWeight(row.weightKg) }}<FishSizeIndicator metric="weight" :value="row.weightKg" :species="speciesById(tournament?.current.targetSpeciesId || '')" /></strong></div><section v-if="!tournamentBoard.length" class="fishing-empty compact"><Icon icon="lucide:trophy" /><h2>{{ t("game.fishing.noEntries") }}</h2><p>{{ t(tournament?.standardLoadout ? "game.fishing.autoEntryHint" : "game.fishing.legacyAutoEntryHint") }}</p></section></div>
          </section>
          <section v-if="unclaimedRewards.length" class="fishing-reward"><span><Icon icon="lucide:gift" /><strong>{{ t("game.fishing.rewardReady") }}</strong></span><el-button type="primary" :loading="claimingReward" @click="claimNextReward">{{ t("game.fishing.claim") }}</el-button></section>
          <section v-if="tournament?.catchUpAvailable" class="fishing-catch-up"><div><strong>{{ t("game.fishing.catchUpTackle") }}</strong><p>{{ t("game.fishing.catchUpHint", { cost: tournament.catchUpCost }) }}</p></div><el-button :loading="redeemingCatchUp" @click="redeemCatchUp">{{ t("game.fishing.redeem") }}</el-button></section>
        </el-tab-pane>
      </el-tabs>
    </GamePageFrame>

    <GameDialog v-model="tripDialogOpen" :title="t('game.fishing.planTrip')" width="720px">
      <div class="fishing-trip-planner">
        <OperationEffectNotice :context="{ domain: 'fishing_cast', actionId: 'fishing.trip', actorKind: tripActorType, actorId: tripActorId }" />
        <el-steps :active="tripStep - 1" finish-status="success" align-center><el-step :title="t('game.fishing.spot')" /><el-step :title="t('game.fishing.loadout')" /><el-step :title="t('game.fishing.duration')" /></el-steps>
        <section v-if="tripStep === 1" class="fishing-planner-step"><h2>{{ t("game.fishing.whereToday") }}</h2><button v-for="spot in selectedWaterSpots" :key="spot.id" type="button" :class="{ 'is-selected': selectedSpotId === spot.id }" @click="selectedSpotId = spot.id"><span><strong>{{ spotName(spot) }}</strong><small>{{ spotMethodCopy(spot) }} · {{ anglersCopy(spot.id) }}</small><small class="fishing-planner-fish"><Icon icon="lucide:fish-symbol" />{{ spotSpeciesSummary(spot) }}</small></span><b>{{ activityCopy(forecastBySpot(spot.id)?.now.activityBand) }}</b></button></section>
        <section v-else-if="tripStep === 2" class="fishing-planner-step">
          <h2>{{ t("game.fishing.whichLoadout", { name: selectedAnglerName }) }}</h2>
          <div v-if="selectedSpot" class="fishing-method-notice" :class="{ 'is-mismatch': tripMethodIssue || selectedTripEquipmentIssues.length }" role="status" aria-live="polite" aria-atomic="true">
            <Icon :icon="tripMethodIssue || selectedTripEquipmentIssues.length ? 'lucide:triangle-alert' : 'lucide:map-pin'" />
            <div>
              <strong>{{ t("game.fishing.spotSupportedMethods", { spot: spotName(selectedSpot), methods: spotMethodCopy(selectedSpot) }) }}</strong>
              <p v-if="selectedTripLoadout">{{ t("game.fishing.currentTripLoadout", { loadout: loadoutName(selectedTripLoadout), method: methodLabel(selectedTripLoadout.method) }) }}</p>
              <p v-if="tripMethodIssue">{{ t("game.fishing.tripMethodMismatchHint") }}</p>
              <p v-if="selectedTripEquipmentIssues.some((row) => row.current !== null)">{{ t("game.fishing.tripEquipmentMismatchHint", { name: selectedAnglerName }) }}</p>
            </div>
          </div>
          <div v-if="tripEquipmentNeedsAttributes && !tripEquipmentAttributesReady" class="fishing-angler-hint" role="status">
            <span>{{ t(tripEquipmentLoadFailed ? "game.fishing.tripEquipmentLoadFailed" : "game.fishing.tripEquipmentLoading", { name: selectedAnglerName }) }}</span>
            <el-button v-if="tripEquipmentLoadFailed" :aria-label="t('game.fishing.refresh')" circle @click="loadTripEquipmentAttributes"><Icon icon="lucide:refresh-cw" /></el-button>
          </div>
          <p class="fishing-angler-hint"><Icon icon="lucide:circle-help" />{{ t("game.fishing.anglerGearHint") }}</p>
          <button v-for="loadout in loadouts" :key="loadout.loadoutId" type="button" :class="{ 'is-selected': tripLoadoutId === loadout.loadoutId }" :aria-pressed="tripLoadoutId === loadout.loadoutId" @click="selectTripLoadout(loadout.loadoutId)">
            <Icon :icon="methodIcon(loadout.method)" />
            <span><strong>{{ loadoutName(loadout) }}</strong><small>{{ methodLabel(loadout.method) }} · {{ strategyLabel(loadout.strategy) }}</small><small v-if="selectedSpot && !loadoutSupportsSelectedSpot(loadout)" class="fishing-loadout-method-issue"><Icon icon="lucide:triangle-alert" />{{ t("game.fishing.loadoutMethodUnsupported") }}</small><small v-for="issue in tripEquipmentIssuesByLoadout.get(loadout.loadoutId)?.filter((row) => row.current !== null)" :key="`${issue.instanceId}:${issue.attribute}`" class="fishing-loadout-method-issue"><Icon icon="lucide:triangle-alert" />{{ equipmentRequirementCopy(issue) }}</small></span>
            <b v-if="loadoutMatchesTournamentStandard(loadout)" class="fishing-standard-eligible"><Icon icon="lucide:badge-check" />{{ t("game.fishing.standardEligible") }}</b>
          </button>
          <div v-if="!loadouts.length" class="fishing-planner-empty"><p>{{ t("game.fishing.needLoadout") }}</p><el-button @click="tripDialogOpen = false; activeTab = 'loadouts'">{{ t("game.fishing.buildLoadout") }}</el-button></div>
          <div v-else-if="selectedSpot && !hasCompatibleTripLoadout" class="fishing-loadout-recovery"><p>{{ t("game.fishing.noCompatibleTripLoadout") }}</p><el-button @click="tripDialogOpen = false; activeTab = 'loadouts'"><Icon icon="lucide:wrench" />{{ t("game.fishing.standardLoadoutManage") }}</el-button></div>
          <section v-if="tournament?.standardLoadout" class="fishing-tournament-choice">
            <header><strong>{{ t("game.fishing.chooseTournamentDivision") }}</strong><small>{{ t("game.fishing.tournamentDivisionHint") }}</small></header>
            <div role="radiogroup" :aria-label="t('game.fishing.chooseTournamentDivision')">
              <button type="button" :class="{ 'is-selected': tripTournamentDivision === 'open' }" :aria-pressed="tripTournamentDivision === 'open'" @click="selectTournamentDivision('open')"><Icon icon="lucide:sliders-horizontal" /><span><strong>{{ t("game.fishing.openDivision") }}</strong><small>{{ t("game.fishing.openDivisionChoiceHint") }}</small></span><Icon :icon="tripTournamentDivision === 'open' ? 'lucide:circle-check' : 'lucide:circle'" /></button>
              <button type="button" :class="{ 'is-selected': tripTournamentDivision === 'standard' }" :aria-pressed="tripTournamentDivision === 'standard'" :disabled="!selectedTripStandardEligible" @click="selectTournamentDivision('standard')"><Icon icon="lucide:badge-check" /><span><strong>{{ t("game.fishing.standardDivision") }}</strong><small>{{ t("game.fishing.standardDivisionChoiceHint") }}</small></span><Icon :icon="tripTournamentDivision === 'standard' ? 'lucide:circle-check' : 'lucide:circle'" /></button>
            </div>
            <p :class="selectedTripStandardEligible ? 'is-ready' : 'is-mismatch'"><Icon :icon="selectedTripStandardEligible ? 'lucide:check' : 'lucide:circle-alert'" />{{ t(selectedTripStandardEligible ? "game.fishing.standardLoadoutSelectedReady" : "game.fishing.standardLoadoutSelectedMismatch") }}</p>
          </section>
        </section>
        <section v-else class="fishing-planner-step fishing-duration-step">
          <h2>{{ t("game.fishing.howLong") }}</h2>
          <div class="fishing-duration-options"><button v-for="option in durationOptions" :key="option.value" type="button" :class="{ 'is-selected': tripDurationChoice === option.value }" @click="tripDurationChoice = option.value">{{ option.label }}</button></div>
          <label v-if="tripDurationChoice === -1" class="fishing-field"><span>{{ t("game.fishing.customMinutes") }}</span><el-input-number v-model="customDurationMinutes" :min="10" :max="1440" :step="10" /></label>
          <p>{{ durationHint }}</p>
          <section class="fishing-supply-pack">
            <header>
              <h3>{{ t("game.fishing.suppliesToBring") }}</h3>
              <p>{{ t("game.fishing.suppliesHint") }}</p>
            </header>
            <div class="fishing-supply-grid">
              <div v-for="row in supplyRows" :key="row.key" class="fishing-supply-row">
                <div class="fishing-supply-ref">
                  <ItemQuickRef :item-id="row.itemId" :count="row.owned" normal-number-format />
                  <small>{{ row.slotLabel }}</small>
                </div>
                <el-input-number
                  v-model="tripSupplies[row.key]"
                  :aria-label="row.label"
                  :min="0"
                  :max="row.owned"
                  :step="1"
                />
              </div>
            </div>
          </section>
          <details class="fishing-more-settings">
            <summary>
              <span><strong>{{ t("game.fishing.moreSettings") }}</strong><small>{{ advancedSettingsSummary }}</small></span>
              <Icon icon="lucide:chevron-down" />
            </summary>
            <div class="fishing-more-settings__body">
              <label class="fishing-setting fishing-setting--wide">
                <span class="fishing-setting__head"><span><Icon icon="lucide:layers-3" /><strong>{{ t("game.fishing.targetLayer") }}</strong></span><small>{{ targetLayerDescription(tripTargetLayer) }}</small></span>
                <el-segmented v-model="tripTargetLayer" :options="targetLayerOptions" />
              </label>
              <label class="fishing-setting fishing-setting--wide">
                <span class="fishing-setting__head"><span><Icon icon="lucide:timer" /><strong>{{ t("game.fishing.fishingPace") }}</strong></span><small>{{ paceDescription(tripPace) }}</small></span>
                <el-segmented v-model="tripPace" :options="paceOptions" />
              </label>
              <label class="fishing-setting fishing-setting--wide">
                <span class="fishing-setting__head"><span><Icon icon="lucide:gauge" /><strong>{{ t("game.fishing.strategy") }}</strong></span><small>{{ strategyDescription(tripStrategy) }}</small></span>
                <el-segmented v-model="tripStrategy" :options="strategyOptions" />
              </label>
              <label class="fishing-setting">
                <span class="fishing-setting__head"><strong>{{ t("game.fishing.keepRule") }}</strong></span>
                <el-select v-model="keepMode"><el-option v-for="option in keepOptions" :key="option.value" :label="option.label" :value="option.value" /></el-select>
              </label>
              <label v-if="keepMode === 'min_weight'" class="fishing-setting">
                <span class="fishing-setting__head"><strong>{{ t("game.fishing.minimumWeight") }}</strong></span>
                <el-input-number v-model="minimumKeepWeight" :min="0.1" :max="100" :step="0.1" :precision="1" />
              </label>
            </div>
          </details>
          <p v-if="supplyIssue" class="fishing-supply-issue"><Icon icon="lucide:circle-alert" />{{ supplyIssue }}</p>
          <p v-if="tripMethodIssue" class="fishing-supply-issue" role="alert"><Icon icon="lucide:triangle-alert" />{{ tripMethodIssue }}</p>
          <p v-for="issue in selectedTripEquipmentIssues.filter((row) => row.current !== null)" :key="`${issue.instanceId}:${issue.attribute}`" class="fishing-supply-issue" role="alert"><Icon icon="lucide:triangle-alert" />{{ equipmentRequirementCopy(issue) }}</p>
        </section>
      </div>
      <template #footer><div class="fishing-dialog-footer"><el-button v-if="tripStep > 1" @click="tripStep--"><Icon icon="lucide:arrow-left" />{{ t("game.fishing.back") }}</el-button><span /><el-button v-if="tripStep < 3" type="primary" :disabled="!canContinueTrip" @click="tripStep++">{{ t("game.fishing.continue") }}<Icon icon="lucide:arrow-right" /></el-button><el-button v-else type="primary" :loading="startingTrip" :disabled="!canStartTrip" @click="startTrip"><Icon icon="lucide:play" />{{ t("game.fishing.startFishing") }}</el-button></div></template>
    </GameDialog>

    <GameDialog v-model="loadoutDialogOpen" :title="editingLoadout.loadoutId ? t('game.fishing.editLoadout') : t('game.fishing.newLoadout')" width="680px">
      <div class="fishing-loadout-editor">
        <div class="fishing-loadout-basics">
          <label class="fishing-field"><span>{{ t("game.fishing.loadoutName") }}</span><el-input v-model="editingLoadout.name" maxlength="40" /></label>
          <label class="fishing-field"><span>{{ t("game.fishing.method") }}</span><el-segmented v-model="editingLoadout.method" :options="methodOptions" @change="resetMethodComponents" /></label>
        </div>
        <div class="fishing-component-fields">
          <div v-for="field in componentFields" :key="field.key" class="fishing-component-field">
            <span>{{ field.label }}</span>
            <button type="button" :aria-label="t('game.fishing.chooseTackleFor', { slot: field.label })" @click="openTacklePicker(field.key)">
              <Card
                v-if="selectedTackleForField(field)"
                :item-id="selectedTackleForField(field)?.itemId || ''"
                :instance-id="selectedTackleForField(field)?.instanceId || null"
                :instance-preview="tacklePreview(selectedTackleForField(field))"
                :quantity="field.instance ? 1 : (selectedTackleForField(field)?.quantity || 0)"
                :width="64"
                :height="64"
                :icon-size="44"
                @click.stop="openTacklePicker(field.key)"
              />
              <EmptyCard v-else :label="field.label" hide-label />
            </button>
          </div>
        </div>
        <div class="fishing-loadout-rules">
          <label class="fishing-field"><span>{{ t("game.fishing.strategy") }}</span><el-segmented v-model="editingLoadout.strategy" :options="strategyOptions" /><small>{{ strategyDescription(editingLoadout.strategy) }}</small></label>
        </div>
      </div>
      <template #footer><el-button class="w-full" type="primary" :loading="savingLoadout" :disabled="!loadoutComplete" @click="saveLoadout">{{ t("game.fishing.saveLoadout") }}</el-button></template>
    </GameDialog>

    <GameDialog v-model="tacklePickerOpen" :title="t('game.fishing.chooseTackleFor', { slot: activeTackleField?.label || '' })" width="720px">
      <ItemBrowserPanel
        :items="tacklePickerItems"
        :empty-text="t('game.fishing.noTackleForSlot', { slot: activeTackleField?.label || '' })"
        :search-placeholder="t('game.inventorySearchPlaceholder')"
        search-width-class="md:w-40!"
        :group-all-by-type="false"
        :grid-min-width="76"
        grid-gap-class="gap-[10px]"
        max-height="min(56vh, 460px)"
        :active="tacklePickerOpen"
        :reset-on-activate="true"
        :secondary-category-toggle-visible="false"
        @item-click="selectTackleFromPicker"
      >
        <template #item="{ item, onClick }">
          <Card
            :item-id="item.itemId"
            :instance-id="item.instanceId || null"
            :instance-preview="item.payload || null"
            :quantity="item.quantity || 0"
            :width="68"
            :height="68"
            :icon-size="46"
            @click="onClick"
          />
        </template>
      </ItemBrowserPanel>
      <section
        v-if="tacklePickerTemplateItemIds.length > 0"
        class="space-y-2"
        :aria-label="t('game.inventorySelectorAvailableItemsTitle', { title: activeTackleField?.label || '' })"
      >
        <el-divider />
        <div class="text-sm app-muted">{{ t("game.inventorySelectorAvailableItemsTitle", { title: activeTackleField?.label || "" }) }}</div>
        <div class="grid gap-[10px] grid-cols-[repeat(auto-fill,minmax(76px,1fr))]">
          <div v-for="itemId in tacklePickerTemplateItemIds" :key="`fishing-tackle-reference-${itemId}`">
            <Card
              :item-id="itemId"
              :quantity="ownedTackleQuantity(itemId)"
              :width="68"
              :height="68"
              :icon-size="46"
            />
          </div>
        </div>
      </section>
    </GameDialog>

    <GameDialog
      v-model="ecologyDialogOpen"
      :title="t('game.fishing.ecologyDetails')"
      class="fishing-ecology-dialog"
      width="min(1040px, calc(100vw - 64px))"
      align-center
    >
      <template #header="{ titleId, titleClass }">
        <div class="fishing-ecology-header">
          <h2 :id="titleId" :class="titleClass">{{ t("game.fishing.ecologyDetails") }}</h2>
          <p>{{ t("game.fishing.ecologyHint") }}</p>
        </div>
      </template>

      <div v-if="selectedForecast" class="fishing-ecology">
        <section class="fishing-ecology-summary" :aria-label="t('game.fishing.currentFishActivity')">
          <div class="fishing-ecology-verdict" :class="`is-${selectedForecast.now.activityBand || 'steady'}`">
            <span>{{ t("game.fishing.currentFishActivity") }}</span>
            <strong><Icon icon="lucide:fish-symbol" /><span>{{ activityCopy(selectedForecast.now.activityBand) }}</span></strong>
          </div>

          <dl class="fishing-ecology-conditions">
            <div>
              <dt><Icon :icon="weatherIcon(selectedForecast.now.weather)" />{{ t("game.fishing.weather") }}</dt>
              <dd>{{ weatherLabel(selectedForecast.now.weather) }}</dd>
            </div>
            <div>
              <dt><Icon icon="lucide:thermometer" />{{ t("game.fishing.waterTemperature") }}</dt>
              <dd>{{ selectedForecast.now.temperatureC.toFixed(1) }}°C</dd>
            </div>
            <div>
              <dt><Icon icon="lucide:wind" />{{ t("game.fishing.oxygen") }}</dt>
              <dd>{{ Math.round(selectedForecast.now.oxygen * 100) }}%</dd>
            </div>
            <div>
              <dt><Icon icon="lucide:footprints" />{{ t("game.fishing.spotPressure") }}</dt>
              <dd>{{ pressureCopy(selectedForecast.pressure) }}</dd>
            </div>
          </dl>
        </section>

        <section class="fishing-ecology-section fishing-ecology-section--forecast">
          <h3 id="fishing-ecology-forecast-title">{{ t("game.fishing.nextHour") }}</h3>
          <div
            class="fishing-tide-scroll"
            role="region"
            tabindex="0"
            aria-labelledby="fishing-ecology-forecast-title"
          >
            <ol class="fishing-tide-chart">
              <li
                v-for="forecast in selectedForecast.next.slice(0, 6)"
                :key="`${forecast.gameHour}-${forecast.weather}`"
                :class="`is-${forecast.activityBand || 'steady'}`"
              >
                <span class="fishing-tide-meta">
                  <time>{{ formatGameHour(forecast.gameHour) }}</time>
                  <Icon :icon="weatherIcon(forecast.weather)" />
                </span>
                <span class="fishing-tide-line" aria-hidden="true"><i /></span>
                <b>{{ activityCopy(forecast.activityBand) }}</b>
              </li>
            </ol>
          </div>
        </section>

        <section v-if="selectedStrangeFish.length" class="fishing-ecology-section fishing-ecology-section--strange">
          <h3>{{ t("game.fishing.strangeWindows") }}</h3>
          <div class="fishing-strange-list">
            <article v-for="species in selectedStrangeFish" :key="species.id">
              <img :src="species.iconPath" :alt="speciesName(species.id)" />
              <div class="fishing-strange-species">
                <strong>{{ speciesName(species.id) }}</strong>
                <span><Icon :icon="weatherIcon(species.windowWeather)" />{{ weatherLabel(species.windowWeather) }}</span>
              </div>
              <div class="fishing-strange-times" :aria-label="strangeWindowCopy(species)">
                <time v-for="hour in species.activeHours" :key="hour">{{ formatGameHour(hour) }}</time>
              </div>
            </article>
          </div>
        </section>
      </div>
    </GameDialog>

    <GameDialog v-model="catchDetailDialogOpen" class="fishing-catch-dialog" :title="t('game.fishing.fishDetails')" width="700px">
      <div v-if="selectedCatchDetail" class="fishing-catch-detail">
        <section class="fishing-catch-identity">
          <div class="fishing-catch-portrait">
            <FishMutationArt :src="selectedCatchSpecies?.iconPath || '/assets/icons/item/fish_catch.png'" :alt="speciesName(selectedCatchDetail.speciesId)" :fish="selectedCatchDetail.fish" :mutations="overview?.mutations || []" width="100%" />
            <span v-if="hasRecord(selectedCatchDetail)"><Icon icon="lucide:medal" />{{ t("game.fishing.record") }}</span>
          </div>
          <header>
            <span>{{ rarityLabel(selectedCatchSpecies?.rarity || 'common') }}</span>
            <h2>{{ speciesName(selectedCatchDetail.speciesId) }}</h2>
          </header>
          <div class="fishing-catch-measurements">
            <span><small>{{ t("game.fishing.length") }}</small><strong>{{ formatLength(selectedCatchDetail.fish.lengthCm) }}<FishSizeIndicator metric="length" :value="selectedCatchDetail.fish.lengthCm" :species="selectedCatchSpecies" /></strong></span>
            <span><small>{{ t("game.fishing.weight") }}</small><strong>{{ formatWeight(selectedCatchDetail.fish.weightKg) }}<FishSizeIndicator metric="weight" :value="selectedCatchDetail.fish.weightKg" :species="selectedCatchSpecies" /></strong></span>
          </div>
        </section>
        <section class="fishing-catch-profile">
          <div class="fishing-catch-layer">
            <header><span><Icon icon="lucide:layers-3" />{{ t("game.fishing.preferredLayer") }}</span><strong>{{ speciesLayerSummary(selectedCatchSpecies) }}</strong></header>
            <div class="fishing-layer-profile" role="img" :aria-label="speciesLayerSummary(selectedCatchSpecies)">
              <span v-for="layer in speciesWaterLayers" :key="layer" :class="{ 'is-active': speciesUsesLayer(selectedCatchSpecies, layer) }"><i /><b>{{ targetLayerLabel(layer) }}</b></span>
            </div>
          </div>
          <dl>
            <div><dt>{{ t("game.fishing.age") }}</dt><dd>{{ ageCopy(selectedCatchDetail.fish.ageDays) }}</dd></div>
            <div><dt>{{ t("game.fishing.sex") }}</dt><dd>{{ sexCopy(selectedCatchDetail.fish.sex) }}</dd></div>
            <div class="fishing-catch-vital"><dt>{{ t("game.fishing.bodyCondition") }}</dt><dd><span>{{ conditionCopy(selectedCatchDetail.fish.condition) }}</span><b>{{ Math.round(selectedCatchDetail.fish.condition * 100) }}%</b></dd><i><span :style="{ width: `${metricPercent(selectedCatchDetail.fish.condition)}%` }" /></i></div>
            <div class="fishing-catch-vital"><dt>{{ t("game.fishing.health") }}</dt><dd><span>{{ healthCopy(selectedCatchDetail.fish.health) }}</span><b>{{ Math.round(selectedCatchDetail.fish.health * 100) }}%</b></dd><i><span :style="{ width: `${metricPercent(selectedCatchDetail.fish.health)}%` }" /></i></div>
          </dl>
          <footer><Icon icon="lucide:map-pin" /><span><strong>{{ spotNameById(selectedCatchDetail.fish.spotId) }}</strong><small>{{ formatCaughtAt(selectedCatchDetail.caughtAt) }}</small></span></footer>
        </section>
      </div>
    </GameDialog>

    <GameDialog
      v-model="saleQuoteDialogOpen"
      class="fishing-sale-dialog"
      :title="t('game.fishing.saleEstimate')"
      width="720px"
      :close-on-click-modal="!confirmingSale"
      :close-on-press-escape="!confirmingSale"
      :show-close="!confirmingSale"
    >
      <div v-if="saleQuote" class="fishing-sale-quote" data-testid="fishing-sale-quote">
        <section class="fishing-sale-guide" aria-labelledby="fishing-sale-guide-title">
          <header>
            <span class="fishing-sale-guide__icon"><Icon icon="lucide:badge-dollar-sign" /></span>
            <div>
              <h3 id="fishing-sale-guide-title">{{ t("game.fishing.saleGuideTitle") }}</h3>
              <p>{{ t("game.fishing.saleBreakdownHint") }}</p>
            </div>
            <time :datetime="saleQuote.marketDate"><Icon icon="lucide:calendar-days" />{{ t("game.fishing.saleMarketDate", { date: formatMarketDate(saleQuote.marketDate) }) }}</time>
          </header>
          <dl>
            <div>
              <dt><Icon icon="lucide:tag" />{{ t("game.fishing.speciesBaseValue") }}</dt>
              <dd>{{ t("game.fishing.saleBaseSource") }}</dd>
            </div>
            <div>
              <dt><Icon icon="lucide:heart-handshake" />{{ t("game.fishing.todayDemand") }}</dt>
              <dd>{{ t("game.fishing.saleDemandSource") }}</dd>
            </div>
            <div>
              <dt><Icon icon="lucide:scale" />{{ t("game.fishing.currentSupply") }}</dt>
              <dd>{{ t("game.fishing.saleSupplySource") }}</dd>
            </div>
          </dl>
          <p class="fishing-sale-guide__range"><Icon icon="lucide:shield-check" />{{ t("game.fishing.saleMarketRangeHint") }}</p>
        </section>
        <div class="fishing-sale-lines">
          <article v-for="line in saleQuote.items" :key="line.instanceId" class="fishing-sale-line">
            <header class="fishing-sale-line__identity">
              <span class="fishing-sale-line__picture">
                <FishMutationArt v-if="speciesById(line.speciesId)?.iconPath" :src="speciesById(line.speciesId)?.iconPath || ''" :fish="{ mutations: line.mutations }" :mutations="overview?.mutations || []" width="50px" />
                <Icon v-else icon="lucide:fish-symbol" />
              </span>
              <span class="fishing-sale-line__name">
                <strong>{{ speciesName(line.speciesId) }}</strong>
                <small>{{ ageCopy(line.ageDays) }} · {{ formatLength(line.lengthCm) }}<FishSizeIndicator metric="length" :value="line.lengthCm" :species="speciesById(line.speciesId)" /> · {{ formatWeight(line.weightKg) }}<FishSizeIndicator metric="weight" :value="line.weightKg" :species="speciesById(line.speciesId)" /></small>
              </span>
              <span class="fishing-sale-line__price">
                <small>{{ t("game.fishing.saleLineEstimate") }}</small>
                <strong>{{ line.finalGold.toLocaleString(locale) }}</strong>
                <em>{{ t("game.fishing.goldUnit") }}</em>
              </span>
            </header>
            <div class="fishing-sale-equation" :class="{ 'has-culture': line.cultureFactor < 1, 'has-mutation': line.mutationMultiplier > 1 }">
              <span><small>{{ t("game.fishing.speciesBaseValue") }}</small><b>{{ line.baseValue.toLocaleString(locale) }}</b></span>
              <span><small>{{ t("game.fishing.specimenFactor") }}</small><b :class="factorTone(line.specimenMultiplier)">{{ formatFactor(line.specimenMultiplier) }}</b></span>
              <span v-if="line.mutationMultiplier > 1"><small>{{ t("game.fishing.mutationSaleFactor") }}</small><b class="is-positive">{{ formatFactor(line.mutationMultiplier) }}</b></span>
              <span><small>{{ t("game.fishing.marketFactor") }}</small><b :class="factorTone(line.marketMultiplier)">{{ formatFactor(line.marketMultiplier) }}</b></span>
              <span v-if="line.cultureFactor < 1"><small>{{ t("game.fishing.culturedFactor") }}</small><b class="is-negative">{{ formatFactor(line.cultureFactor) }}</b></span>
            </div>
            <div class="fishing-sale-breakdown">
              <section>
                <header><span><Icon icon="lucide:scan-search" />{{ t("game.fishing.specimenDetails") }}</span><b :class="factorTone(line.specimenMultiplier)">{{ formatFactor(line.specimenMultiplier) }}</b></header>
                <dl>
                  <div><dt>{{ t("game.fishing.ageFactor") }}</dt><dd :class="factorTone(line.ageFactor)">{{ formatFactor(line.ageFactor) }}</dd></div>
                  <div><dt>{{ t("game.fishing.lengthFactor") }}</dt><dd :class="factorTone(line.lengthFactor)">{{ formatFactor(line.lengthFactor) }}</dd></div>
                  <div><dt>{{ t("game.fishing.weightFactor") }}</dt><dd :class="factorTone(line.weightFactor)">{{ formatFactor(line.weightFactor) }}</dd></div>
                  <div><dt>{{ t("game.fishing.qualityFactor") }}</dt><dd :class="factorTone(line.qualityFactor)">{{ formatFactor(line.qualityFactor) }}</dd></div>
                </dl>
              </section>
              <section class="fishing-sale-breakdown__market">
                <header><span><Icon icon="lucide:store" />{{ t("game.fishing.marketDetails") }}</span><b :class="factorTone(line.marketMultiplier)">{{ formatFactor(line.marketMultiplier) }}</b></header>
                <dl>
                  <div><dt>{{ t("game.fishing.todayDemand") }}</dt><dd :class="factorTone(line.dailyDemand)">{{ formatFactor(line.dailyDemand) }}</dd></div>
                  <div><dt>{{ t("game.fishing.currentSupply") }}</dt><dd :class="factorTone(line.supplyFactor)">{{ formatFactor(line.supplyFactor) }}</dd></div>
                </dl>
              </section>
            </div>
          </article>
        </div>
      </div>
      <template #footer>
        <div class="fishing-sale-dialog__footer">
          <div v-if="saleQuote" class="fishing-sale-total">
            <span>{{ t("game.fishing.estimatedTotal") }}</span>
            <strong><Icon icon="lucide:coins" />{{ saleQuote.totalGold.toLocaleString(locale) }}<small>{{ t("game.fishing.goldUnit") }}</small></strong>
          </div>
          <div class="fishing-sale-dialog__actions">
            <el-button :disabled="confirmingSale" @click="saleQuoteDialogOpen = false">{{ t("game.fishing.cancel") }}</el-button>
            <el-button type="primary" :loading="confirmingSale" @click="confirmFishingSale"><Icon icon="lucide:coins" />{{ t("game.fishing.confirmSale") }}</el-button>
          </div>
        </div>
      </template>
    </GameDialog>

    <GameDialog v-model="filterDialogOpen" :title="t('game.fishing.filter')" width="min(760px, 96vw)">
      <FishFilterControls v-model="catchFilters" :species="overview?.species || []" :mutations="overview?.mutations || []" />
      <template #footer>
        <el-button @click="catchFilters = defaultFishFilters('recent')">{{ t("game.fishing.fishSelectorReset") }}</el-button>
        <el-button type="primary" @click="applyCatchFilters">{{ t("game.fishing.applyFilter") }}</el-button>
      </template>
    </GameDialog>

  </div>
</template>

<script setup lang="ts">
import OperationEffectNotice from "@components/game/operation-effect/OperationEffectNotice.vue"
import { computed, nextTick, onMounted, onUnmounted, reactive, ref, watch } from "vue"
import { ElMessage, ElMessageBox } from "element-plus"
import { Icon } from "@iconify/vue"
import { useI18n } from "vue-i18n"
import BatchActionBar from "@components/game/common/BatchActionBar.vue"
import GameDialog from "@components/game/common/GameDialog.vue"
import GamePageFrame from "@components/game/common/GamePageFrame.vue"
import Card from "@components/game/item/Card.vue"
import EmptyCard from "@components/game/item/EmptyCard.vue"
import ItemBrowserPanel, { type ItemBrowserItem } from "@components/game/item/ItemBrowserPanel.vue"
import ItemQuickRef from "@components/game/item/QuickRef.vue"
import FishMutationArt from "@components/game/fishing/FishMutationArt.vue"
import FishingWaterBackdrop from "@components/game/fishing/FishingWaterBackdrop.vue"
import { fishingSceneEnvironment } from "@services/fishing-scene"
import FishSizeIndicator from "@components/game/fishing/FishSizeIndicator.vue"
import { useDevice } from "@composables/game/useDevice"
import { useGameStore } from "@stores/game"
import { useFishingStore } from "@stores/fishing"
import { useInventoryStore, type InventoryInstance } from "@stores/inventory"
import { useRoleStateStore, type RoleKey } from "@stores/role-state"
import { fishingLoadoutEquipmentRequirements, type FishingEquipmentRequirement } from "@services/fishing-loadout-requirements"
import { useKittiesViewModel } from "@/composables/game/cattery/useKittiesViewModel"
import { gamePageVisuals } from "@services/game-page-visuals"
import { resolveErrorMessage } from "@services/error-messages"
import { apiGet, APIClientError } from "@services/api"
import { fishingPondPredationRisks, mergeFishingPondEcosystem } from "@services/fishing-pond-ecology"
import { fishMutationIds, resolveFishMutations } from "@services/fish-mutations"
import { defaultFishFilters, filterAndSortFish } from "@services/fish-filters"
import FishFilterControls from "@components/game/fishing/FishFilterControls.vue"
import { getItemDefinition } from "@services/item-definition"
import {
  claimFishingStarter,
  addFishingPondFishBatch,
  claimFishingTournamentReward,
  deleteFishingLoadout,
  enqueueFishingTrip,
  fetchFishingCatches,
  fetchFishingJournal,
  fetchFishingLeaderboard,
  fetchFishingLoadouts,
  fetchFishingOverview,
  fetchFishingTripHistory,
  fetchFishingTripEvents,
  fetchFishingPond,
  loadAllAvailablePondFish,
  fetchFishingTournaments,
  FISHING_SALE_BATCH_MAX,
  fishingPondZone,
  listCompatibleFishingTackleItemIds,
  localizedFishingText,
  redeemFishingCatchUpRod,
  quoteFishingSale,
  saveFishingLoadout,
  sellFishingCatches,
  stopFishingTrip,
  type FishingAngler,
  type FishingCatch,
  type FishIndividual,
  type FishingKeepMode,
  type FishingKnowledge,
  type FishingLeaderboardRow,
  type FishingLoadout,
  type FishingLoadoutComponents,
  type FishingMethod,
  type FishingOverview,
  type FishingPondSpeciesProfile,
  type FishingPondOverview,
  type FishingOwnedTackle,
  type FishingPace,
  type FishingSaleQuote,
  type FishingSpecies,
  type FishingSpot,
  type FishingStrategy,
  type FishingSupplyCounts,
  type FishingTargetLayer,
  type FishingTournamentOverview,
  type FishingTrip,
  type FishingTripEvent,
  type FishingWater
} from "@services/fishing"

type PrimaryTab = "waters" | "loadouts" | "catches" | "history" | "tournaments"
type CatchSection = "collection" | "journal" | "records"
type TournamentDivision = "open" | "standard"
type ComponentFieldKey = keyof FishingLoadoutComponents
type TackleField = { key: ComponentFieldKey; slot: string; label: string; instance?: boolean }
type SupplyKey = keyof FishingSupplyCounts
type SupplyRow = { key: SupplyKey; itemId: string; label: string; slotLabel: string; owned: number }
type SpotSpeciesRow = { species: FishingSpecies; weight: number; share: number }
type SpeciesWaterLayer = Exclude<FishingTargetLayer, "auto">
type FishingTripEventHistoryState = {
  items: FishingTripEvent[]
  total: number
  nextCursor: number
  historyComplete: boolean
  loading: boolean
  loaded: boolean
  error: boolean
}

const speciesWaterLayers: SpeciesWaterLayer[] = ["surface", "midwater", "near_bottom"]

const { t, locale } = useI18n()
const { isMobile } = useDevice()
const gameStore = useGameStore()
const fishingStore = useFishingStore()
const inventoryStore = useInventoryStore()
const roleStateStore = useRoleStateStore()
const { kitties, ensureInitialized: ensureKittiesInitialized, kittyAvatar } = useKittiesViewModel(t)
const fishingVisual = gamePageVisuals.fishing

const activeTab = ref<PrimaryTab>("waters")
const overview = ref<FishingOverview | null>(null)
const loadouts = ref<FishingLoadout[]>([])
const loading = ref(false)
const refreshing = ref(false)
const selectedWaterId = ref("")
const selectedSpotId = ref("")
const tripDialogOpen = ref(false)
const tripStep = ref(1)
const tripLoadoutId = ref("")
const tripTournamentDivision = ref<TournamentDivision>("open")
const tripActorType = ref<"player" | "kitty">("player")
const tripActorId = ref(0)
const tripDurationChoice = ref(7200)
const customDurationMinutes = ref(60)
const tripStrategy = ref<FishingStrategy>("balanced")
const tripTargetLayer = ref<FishingTargetLayer>("auto")
const tripPace = ref<FishingPace>("balanced")
const keepMode = ref<FishingKeepMode>("keep_all")
const minimumKeepWeight = ref(1)
const startingTrip = ref(false)
const stoppingTrip = ref(false)
const ecologyDialogOpen = ref(false)
const loadoutDialogOpen = ref(false)
const tacklePickerOpen = ref(false)
const tacklePickerFieldKey = ref<ComponentFieldKey | null>(null)
const savingLoadout = ref(false)
const filterDialogOpen = ref(false)
const catchSection = ref<CatchSection>("collection")
const catches = ref<FishingCatch[]>([])
const catchesLoading = ref(false)
const tripHistory = ref<FishingTrip[]>([])
const tripHistoryLimit = ref(20)
const historyLoading = ref(false)
const historyLoaded = ref(false)
const historyEventStates = reactive<Record<string, FishingTripEventHistoryState>>({})
const journal = ref<FishingKnowledge[]>([])
const catchFilters = ref(defaultFishFilters("recent"))
const appliedCatchFilters = ref(defaultFishFilters("recent"))
const catchFiltersActive = computed(() => {
  const filters = appliedCatchFilters.value
  return Boolean(filters.search.trim() || filters.species || filters.rarity || filters.mutation || filters.health !== "all")
})
const catchActionMode = ref<"" | "sell" | "pond">("")
const selectedCatchIds = ref(new Set<number>())
const focusedCatchSpeciesId = ref("")
const selectedCatchDetail = ref<FishingCatch | null>(null)
const catchDetailDialogOpen = ref(false)
const recordSpeciesId = ref("")
const recordMetric = ref<"length" | "weight">("weight")
const leaderboard = ref<FishingLeaderboardRow[]>([])
const tournament = ref<FishingTournamentOverview | null>(null)
const tournamentDivision = ref<"open" | "standard">("open")
const claimingReward = ref(false)
const redeemingCatchUp = ref(false)
const sellingCatches = ref(false)
const confirmingSale = ref(false)
const saleQuoteDialogOpen = ref(false)
const saleQuote = ref<FishingSaleQuote | null>(null)
const stockingCatches = ref(false)
const loadingPondSelection = ref(false)
const pondSelection = ref<FishingPondOverview | null>(null)
const liveClock = ref(Date.now())
const liveEventHistory = ref<FishingTripEvent[]>([])
const liveLogEl = ref<HTMLElement | null>(null)
const tripSupplies = reactive<Required<FishingSupplyCounts>>({ mainLine: 0, leader: 0, hook: 0, floatOrSinker: 0, lure: 0, bait: 0 })
let liveClockTimer: number | undefined
let overviewTimer: number | undefined
let liveEventTripId = ""
let tripActorInitialized = false
let catchLoadGeneration = 0

const emptyComponents = (): FishingLoadoutComponents => ({
  rodInstanceId: 0,
  reelInstanceId: 0,
  mainLineItemId: "",
  leaderItemId: "",
  hookItemId: "",
  floatOrSinkerItemId: "",
  lureItemId: "",
  baitItemId: ""
})
const editingLoadout = reactive<FishingLoadout>({ loadoutId: "", name: "", method: "float", strategy: "balanced", safetyLoadPercent: 82, components: emptyComponents() })

const catchSections = computed(() => [
  { value: "collection", label: t("game.fishing.collection") },
  { value: "journal", label: t("game.fishing.journal") },
  { value: "records", label: t("game.fishing.records") }
])
const recordMetrics = computed(() => [
  { value: "weight", label: t("game.fishing.heaviest") },
  { value: "length", label: t("game.fishing.longest") }
])
const tournamentDivisions = computed(() => [
  { value: "open", label: t("game.fishing.openDivision") },
  { value: "standard", label: t("game.fishing.standardDivision") }
])
const tournamentStandardStats = computed(() => {
  const rules = tournament.value?.standardRules
  if (!rules) return []
  return [
    { key: "level", label: t("game.fishing.standardFishingLevel"), value: String(rules.fishingLevel) },
    { key: "strategy", label: t("game.fishing.strategy"), value: strategyLabel(rules.strategy) },
    { key: "load", label: t("game.fishing.tackleMaxLoad"), value: t("game.fishing.weightValue", { value: formatTournamentStandardNumber(rules.maxLoadKg) }) },
    { key: "control", label: t("attr.fishing_control.name"), value: formatTournamentStandardNumber(rules.control) },
    { key: "sensitivity", label: t("attr.fishing_sensitivity.name"), value: formatTournamentStandardNumber(rules.sensitivity) },
    { key: "hooking", label: t("attr.fishing_hooking.name"), value: formatTournamentStandardNumber(rules.hooking) }
  ]
})
const durationOptions = computed(() => [
  { value: 1800, label: t("game.fishing.thirtyMinutes") },
  { value: 7200, label: t("game.fishing.twoHours") },
  { value: 28800, label: t("game.fishing.eightHours") },
  { value: -1, label: t("game.fishing.custom") },
  { value: 0, label: t("game.fishing.continuous") }
])
const strategyOptions = computed(() => [
  { value: "conservative", label: t("game.fishing.conservative") },
  { value: "balanced", label: t("game.fishing.balanced") },
  { value: "aggressive", label: t("game.fishing.aggressive") }
])
const targetLayerOptions = computed(() => {
  const options = [
    { value: "auto", label: t("game.fishing.layer_auto") },
    { value: "surface", label: t("game.fishing.layer_surface") },
    { value: "midwater", label: t("game.fishing.layer_midwater") },
    { value: "near_bottom", label: t("game.fishing.layer_near_bottom") }
  ]
  return selectedTripLoadout.value?.method === "bottom"
    ? options.filter((option) => option.value === "auto" || option.value === "near_bottom")
    : options
})
const paceOptions = computed(() => [
  { value: "patient", label: t("game.fishing.pace_patient") },
  { value: "balanced", label: t("game.fishing.pace_balanced") },
  { value: "search", label: t("game.fishing.pace_search") }
])
const methodOptions = computed(() => [
  { value: "float", label: t("game.fishing.floatMethod") },
  { value: "bottom", label: t("game.fishing.bottomMethod") },
  { value: "lure", label: t("game.fishing.lureMethod") }
])
const keepOptions = computed(() => [
  { value: "keep_all", label: t("game.fishing.keepAll") },
  { value: "release_all", label: t("game.fishing.releaseAll") },
  { value: "record_only", label: t("game.fishing.recordsOnly") },
  { value: "min_weight", label: t("game.fishing.keepAboveWeight") }
])

const activeTrip = computed<FishingTrip | null>(() => fishingStore.trip?.status === "running" ? fishingStore.trip : overview.value?.currentTrip || null)
const anglerOptions = computed<FishingAngler[]>(() => {
  const rows: FishingAngler[] = Array.isArray(overview.value?.anglers) && overview.value.anglers.length
    ? overview.value.anglers
    : [{ actorType: "player" as const, actorId: 0, status: "idle", fishingLevel: overview.value?.level || 1 }]
  const byKey = new Map(rows.map((row) => [anglerKey(row.actorType, row.actorId), { ...row }]))
  for (const kitty of kitties.value) {
    const key = anglerKey("kitty", kitty.id)
    const existing = byKey.get(key)
    byKey.set(key, {
      actorType: "kitty",
      actorId: kitty.id,
      name: existing?.name || kitty.name,
      status: existing?.status || kitty.status || "idle",
      fishingLevel: Math.max(1, Number(existing?.fishingLevel || 1))
    })
  }
  return [...byKey.values()]
})
const playerAnglerName = computed(() => {
  const username = String(gameStore.username || "").trim()
  return username ? t("game.fishing.playerAnglerSelf", { name: username }) : t("game.fishing.playerAngler")
})
const selectedAnglerKey = computed(() => anglerKey(tripActorType.value, tripActorId.value))
const selectedAngler = computed(() => anglerOptions.value.find((row) => anglerKey(row.actorType, row.actorId) === selectedAnglerKey.value) || anglerOptions.value[0])
const selectedAnglerName = computed(() => anglerName(selectedAngler.value))
const selectedFishingLevel = computed(() => Math.max(1, Number(selectedAngler.value?.fishingLevel || overview.value?.level || 1)))
const activeAnglerName = computed(() => {
  const trip = activeTrip.value
  if (!trip) return selectedAnglerName.value
  const angler = anglerOptions.value.find((row) => anglerKey(row.actorType, row.actorId) === anglerKey(trip.actorType, trip.actorId))
  if (angler) return anglerName(angler)
  return trip.actorType === "kitty" ? t("game.fishing.kittyAngler") : playerAnglerName.value
})
const selectedWater = computed(() => overview.value?.waters.find((water) => water.id === selectedWaterId.value) || null)
const selectedWaterSpots = computed(() => (overview.value?.spots || []).filter((spot) => spot.waterId === selectedWaterId.value).sort((a, b) => a.order - b.order))
const selectedSpot = computed(() => overview.value?.spots.find((spot) => spot.id === selectedSpotId.value) || null)
const speciesCatalogById = computed(() => new Map((overview.value?.species || []).map((species) => [species.id, species])))
const pondSpeciesById = computed(() => new Map((overview.value?.pondSpecies || []).map((profile) => [profile.speciesId, profile])))
const displayCatches = computed(() => {
  const rows = catches.value.map((row) => {
    const fish = currentCatchFish(row)
    return fish === row.fish ? row : { ...row, fish }
  })
  const seenInstances = new Set(rows.flatMap((row) => row.inventoryInstanceId ? [row.inventoryInstanceId] : []))
  const inventoryOnlyRows: FishingCatch[] = []
  // Pond-born and administratively added fish can enter inventory without a catch-history row.
  for (const instance of inventoryStore.instances) {
    if (!isCurrentInventoryFish(instance) || seenInstances.has(instance.instanceId)) continue
    inventoryOnlyRows.push(inventoryFishAsCatch(instance))
  }
  inventoryOnlyRows.sort((left, right) => Number(right.inventoryInstanceId || 0) - Number(left.inventoryInstanceId || 0))
  return filterAndSortFish([...inventoryOnlyRows, ...rows], appliedCatchFilters.value, {
    fish: (row) => row.fish,
    name: (row) => String(inventoryStore.instancesById.get(row.inventoryInstanceId || 0)?.customData?.name || speciesName(row.speciesId)),
    speciesName, speciesById: speciesCatalogById.value,
    instanceId: (row) => row.inventoryInstanceId || 0, locale: locale.value
  })
})
const selectedSpotSpecies = computed(() => selectedSpot.value ? spotSpeciesRows(selectedSpot.value) : [])
const selectedTripLoadout = computed(() => loadouts.value.find((loadout) => loadout.loadoutId === tripLoadoutId.value) || null)
const tripEquipmentLoadFailed = ref(false)
let tripEquipmentLoadGeneration = 0
const tripRoleKey = computed<RoleKey>(() => tripActorType.value === "kitty" ? `kitty:${tripActorId.value}` : "player:self")
const tripEquipmentAttributesReady = computed(() => roleStateStore.ensureRole(tripRoleKey.value).initialized.attributes)
const tripEquipmentRequirementsByLoadout = computed(() => {
  const attributes = tripEquipmentAttributesReady.value ? roleStateStore.roleUnitAttributes(tripRoleKey.value) : null
  return new Map(loadouts.value.map((loadout) => [loadout.loadoutId, fishingLoadoutEquipmentRequirements(loadout, overview.value?.ownedTackle || [], attributes)]))
})
const tripEquipmentNeedsAttributes = computed(() => [...tripEquipmentRequirementsByLoadout.value.values()].some((rows) => rows.length > 0))
const tripEquipmentIssuesByLoadout = computed(() => new Map([...tripEquipmentRequirementsByLoadout.value].map(([id, rows]) => [id, rows.filter((row) => !row.met)])))
const selectedTripEquipmentIssues = computed(() => tripEquipmentIssuesByLoadout.value.get(tripLoadoutId.value) || [])
function equipmentRequirementCopy(issue: FishingEquipmentRequirement) {
  return t("game.fishing.tripEquipmentRequirement", {
    item: t(`item.${issue.itemId}.name`, issue.itemId),
    attribute: t(`attr.${issue.attribute}.name`, issue.attribute),
    required: issue.required,
    current: issue.current,
    name: selectedAnglerName.value
  })
}
async function loadTripEquipmentAttributes() {
  const generation = ++tripEquipmentLoadGeneration
  const roleKey = tripRoleKey.value
  const token = gameStore.token
  const isCurrent = () => generation === tripEquipmentLoadGeneration && token === gameStore.token && roleKey === tripRoleKey.value
  tripEquipmentLoadFailed.value = false
  if (!tripDialogOpen.value || !tripEquipmentNeedsAttributes.value || tripEquipmentAttributesReady.value) return
  try {
    if (roleKey === "player:self") {
      const snapshot = await apiGet<Parameters<typeof roleStateStore.initializePlayerFromSnapshot>[0]>("/api/player/stats", token)
      if (isCurrent() && !tripEquipmentAttributesReady.value) {
        roleStateStore.initializePlayerFromSnapshot(snapshot)
      }
    } else {
      const kittyId = tripActorId.value
      const profile = await apiGet<Parameters<typeof roleStateStore.initializeKittyFromProfile>[1]>(`/api/cattery/kitties/${encodeURIComponent(String(kittyId))}/combat-profile`, token)
      if (isCurrent()) roleStateStore.initializeKittyFromProfile(kittyId, profile)
    }
    if (isCurrent()) tripEquipmentLoadFailed.value = !tripEquipmentAttributesReady.value
  } catch {
    if (isCurrent()) tripEquipmentLoadFailed.value = true
  }
}
watch([tripDialogOpen, tripRoleKey, tripEquipmentNeedsAttributes, () => gameStore.token], () => { void loadTripEquipmentAttributes() })
function loadoutSupportsSelectedSpot(loadout: FishingLoadout | null) {
  return Boolean(loadout && selectedSpot.value?.methods.includes(loadout.method))
}
const selectedTripMethodSupported = computed(() => loadoutSupportsSelectedSpot(selectedTripLoadout.value))
const hasCompatibleTripLoadout = computed(() => loadouts.value.some(loadoutSupportsSelectedSpot))
const tripMethodIssue = computed(() => {
  const spot = selectedSpot.value
  const loadout = selectedTripLoadout.value
  if (!spot || !loadout || selectedTripMethodSupported.value) return ""
  return [
    t("game.fishing.spotSupportedMethods", { spot: spotName(spot), methods: spotMethodCopy(spot) }),
    t("game.fishing.currentTripLoadout", { loadout: loadoutName(loadout), method: methodLabel(loadout.method) }),
    t("game.fishing.tripMethodMismatchHint")
  ].join(" ")
})
const tournamentStandardItems = computed(() => (tournament.value?.standardLoadout?.items || []).map((item) => ({
  ...item,
  slotLabel: tournamentTackleSlotLabel(item.slot),
  owned: ownedTackleQuantity(item.itemId) > 0
})))
const matchingStandardLoadout = computed(() => loadouts.value.find(loadoutMatchesTournamentStandard) || null)
const selectedTripStandardEligible = computed(() => loadoutMatchesTournamentStandard(selectedTripLoadout.value))
const selectedCatchSpecies = computed(() => selectedCatchDetail.value ? speciesById(selectedCatchDetail.value.speciesId) : undefined)
const batchSellMode = computed(() => catchActionMode.value === "sell")
const batchPondMode = computed(() => catchActionMode.value === "pond")
const catchSelectionMode = computed(() => Boolean(catchActionMode.value))
const catchActionSubmitting = computed(() => sellingCatches.value || stockingCatches.value)
const saleImmatureCatchCount = computed(() => displayCatches.value.filter((row) => row.saleIneligibleReason === "cultured_immature").length)
const saleOtherUnavailableCatchCount = computed(() => displayCatches.value.filter((row) => row.saleEligible === false && row.saleIneligibleReason !== "cultured_immature").length)
const remainingPondSlots = computed(() => Math.max(0, (pondSelection.value?.limits.fishCapacity || 0) - (pondSelection.value?.fish.filter((row) => fishingPondZone(row.fish) === "main").length || 0)))
const pondSelectableIds = computed(() => new Set((pondSelection.value?.availableFish || []).map((row) => row.instanceId)))
const catchBrowserItems = computed<ItemBrowserItem[]>(() => displayCatches.value.map((row) => ({
  key: row.catchId,
  itemId: "fish_catch",
  itemTypes: ["fish"],
  instanceId: row.inventoryInstanceId || undefined,
  quantity: 1,
  payload: row
})))
const catchByInstanceId = computed(() => new Map(
  displayCatches.value
    .filter((row) => Boolean(row.inventoryInstanceId))
    .map((row) => [row.inventoryInstanceId as number, row])
))
const selectedCatchRows = computed(() => [...selectedCatchIds.value]
  .map((instanceId) => catchByInstanceId.value.get(instanceId))
  .filter((row): row is FishingCatch => Boolean(row)))
const selectedCatchSpeciesIds = computed(() => new Set(selectedCatchRows.value.map((row) => row.speciesId)))
const batchFocusedCatchSpeciesId = computed(() => {
  if (focusedCatchSpeciesId.value && selectedCatchSpeciesIds.value.has(focusedCatchSpeciesId.value)) {
    return focusedCatchSpeciesId.value
  }
  return selectedCatchRows.value[0]?.speciesId || ""
})
const batchFocusedSameSpeciesCatches = computed(() => {
  const speciesId = batchFocusedCatchSpeciesId.value
  if (!speciesId) return []
  return displayCatches.value.filter((row) => row.speciesId === speciesId && canSelectCatch(row))
})
const batchFocusedSameSpeciesBulkCatches = computed(() => batchFocusedSameSpeciesCatches.value.filter((row) => !hasRecord(row) && !hasFishMutation(row)))
const batchFocusedSameSpeciesUnselectedCount = computed(() => batchFocusedSameSpeciesBulkCatches.value
  .filter((row) => !selectedCatchIds.value.has(row.inventoryInstanceId || 0)).length)
const batchSameSpeciesSelectVisible = computed(() => batchFocusedSameSpeciesCatches.value.length > 5 && batchFocusedSameSpeciesUnselectedCount.value > 0)
const batchSameSpeciesSelectLabel = computed(() => t("game.batchSelectAllSameItem", {
  name: speciesName(batchFocusedCatchSpeciesId.value),
  n: batchFocusedSameSpeciesBulkCatches.value.length
}))
const advancedSettingsSummary = computed(() => [targetLayerLabel(tripTargetLayer.value), paceLabel(tripPace.value), strategyLabel(tripStrategy.value)].join(" · "))
const currentWater = computed(() => overview.value?.waters.find((water) => water.id === activeTrip.value?.waterId) || selectedWater.value)
const currentSpot = computed(() => overview.value?.spots.find((spot) => spot.id === activeTrip.value?.spotId) || selectedSpot.value)
const currentForecast = computed(() => activeTrip.value ? forecastBySpot(activeTrip.value.spotId) : undefined)
const liveEnvironment = computed(() => fishingSceneEnvironment(currentForecast.value?.now))
const livePeriodIcon = computed(() => ({ dawn: "lucide:sunrise", day: "lucide:sun", dusk: "lucide:sunset", night: "lucide:moon", unknown: "lucide:clock-3" })[liveEnvironment.value.period])
const selectedForecast = computed(() => selectedSpot.value ? forecastBySpot(selectedSpot.value.id) : selectedWaterSpots.value[0] ? forecastBySpot(selectedWaterSpots.value[0].id) : undefined)
const selectedStrangeFish = computed(() => (overview.value?.species || []).filter((species) => species.waterId === selectedWaterId.value && species.fantasy))
const durationSeconds = computed(() => tripDurationChoice.value === -1 ? customDurationMinutes.value * 60 : tripDurationChoice.value)
const durationHint = computed(() => durationSeconds.value === 0 ? t("game.fishing.continuousHint") : t("game.fishing.durationHint", { time: formatDuration(durationSeconds.value) }))
const tripActorReady = computed(() => Boolean(selectedAngler.value && !anglerUnavailable(selectedAngler.value) && (!selectedWater.value || waterUnlocked(selectedWater.value))))
const selectedTournamentDivisionValid = computed(() => tripTournamentDivision.value !== "standard" || selectedTripStandardEligible.value)
const canContinueTrip = computed(() => tripStep.value === 1 ? Boolean(selectedSpot.value) : tripStep.value === 2 ? Boolean(selectedTripMethodSupported.value && !selectedTripEquipmentIssues.value.length && tripActorReady.value && selectedTournamentDivisionValid.value) : true)
const supplyRows = computed<SupplyRow[]>(() => buildSupplyRows(selectedTripLoadout.value))
const supplyIssue = computed(() => {
  const missing = supplyRows.value.find((row) => tripSupplies[row.key] < 1)
  if (missing) return t("game.fishing.supplyMissing", { item: missing.label })
  const overflow = supplyRows.value.find((row) => tripSupplies[row.key] > row.owned)
  return overflow ? t("game.fishing.supplyTooMany", { item: overflow.label }) : ""
})
const canStartTrip = computed(() => Boolean(selectedTripMethodSupported.value && !selectedTripEquipmentIssues.value.length && tripActorReady.value && selectedTournamentDivisionValid.value && supplyRows.value.length > 0 && !supplyIssue.value))
const liveFishStamina = computed(() => {
  const trip = activeTrip.value
  if (!trip?.currentFish) return 0
  const reported = Number(trip.currentFish.stamina)
  if (Number.isFinite(reported) && reported >= 0 && reported < 99) return Math.round(reported)
  if (trip.phase !== "fighting") return 100
  const start = Date.parse(trip.phaseStartedAt)
  const end = Date.parse(trip.phaseEndsAt)
  if (!Number.isFinite(start) || !Number.isFinite(end) || end <= start) return 100
  return Math.round(Math.max(0, Math.min(1, (end - liveClock.value) / (end - start))) * 100)
})
const liveTimelineEvents = computed(() => liveEventHistory.value.slice(-120).reverse())
const liveSceneEvent = computed(() => {
  const events = liveTimelineEvents.value
  const phase = activeTrip.value?.phase
  const wantedKinds = phase === "landed"
    ? ["landed"]
    : phase === "resting"
      ? ["escaped", "line_break"]
      : phase === "fighting"
        ? ["hooked"]
        : phase === "bite"
          ? ["bite"]
          : []
  return events.find((event) => wantedKinds.includes(event.kind)) || events[0]
})
const liveHeadline = computed(() => {
  const trip = activeTrip.value
  const event = liveSceneEvent.value
  if (trip?.phase === "landed" && event?.kind === "landed") return eventTitle(event)
  if (trip?.phase === "resting" && (event?.kind === "escaped" || event?.kind === "line_break")) return eventTitle(event)
  if (trip?.phase === "bite") return t("game.fishing.sceneBiteHeadline")
  if (trip?.phase === "fighting") return t("game.fishing.sceneHookedHeadline")
  return t("game.fishing.sceneWaitingHeadline")
})
const liveStatusCopy = computed(() => {
  const trip = activeTrip.value
  const event = liveSceneEvent.value
  if ((trip?.phase === "landed" || trip?.phase === "resting") && event) return eventDetail(event)
  if (trip?.phase === "bite") return t("game.fishing.eventBiteDetail", { size: sizeClassCopy(event?.sizeClass) })
  if (trip?.phase === "fighting") {
    return trip.currentFish?.sizeClass === "large" ? t("game.fishing.largeFishOn") : t("game.fishing.sceneFightDetail")
  }
  return t("game.fishing.waterSettling")
})
const liveSupplyRows = computed(() => buildLiveSupplyRows(activeTrip.value))
const tournamentBoard = computed(() => tournamentDivision.value === "open" ? tournament.value?.openLeaderboard || [] : tournament.value?.standardLeaderboard || [])
const unclaimedRewards = computed(() => (tournament.value?.rewards || []).filter((reward) => !reward.claimedAt))
const componentFields = computed<TackleField[]>(() => {
  const common: TackleField[] = [
    { key: "rodInstanceId", slot: "rod", label: t("game.fishing.rod"), instance: true },
    { key: "reelInstanceId", slot: "reel", label: t("game.fishing.reel"), instance: true },
    { key: "mainLineItemId", slot: "main_line", label: t("game.fishing.mainLine") },
    { key: "leaderItemId", slot: "leader", label: t("game.fishing.leader") }
  ]
  if (editingLoadout.method === "lure") return [...common, { key: "lureItemId", slot: "lure", label: t("game.fishing.lure") }]
  return [
    ...common,
    { key: "hookItemId", slot: "hook", label: t("game.fishing.hook") },
    { key: "floatOrSinkerItemId", slot: "float_or_sinker", label: editingLoadout.method === "float" ? t("game.fishing.float") : t("game.fishing.sinker") },
    { key: "baitItemId", slot: "bait", label: t("game.fishing.bait") }
  ]
})
const activeTackleField = computed(() => componentFields.value.find((field) => field.key === tacklePickerFieldKey.value) || null)
const tacklePickerTemplateItemIds = computed(() => {
  const field = activeTackleField.value
  if (!field) return []
  return listCompatibleFishingTackleItemIds(overview.value?.tackle, field.slot, editingLoadout.method)
})
const tacklePickerItems = computed<ItemBrowserItem[]>(() => {
  const field = activeTackleField.value
  if (!field) return []
  const compatibleItemIds = new Set(tacklePickerTemplateItemIds.value)
  return tackleForField(field.slot).filter((item) => compatibleItemIds.has(item.itemId)).map((item, index) => ({
    key: item.instanceId || `${item.itemId}-${index}`,
    itemId: item.itemId,
    itemTypes: [...(getItemDefinition(item.itemId)?.type || [])],
    instanceId: item.instanceId,
    quantity: field.instance ? 1 : item.quantity,
    payload: tacklePreview(item)
  }))
})
const loadoutComplete = computed(() => componentFields.value.every((field) => Boolean(editingLoadout.components[field.key])))

watch(activeTab, (tab) => {
  if (tab !== "catches") exitCatchActionMode()
  if (tab === "catches" && !catches.value.length) void loadCatchData()
  if (tab === "history" && !historyLoaded.value) void loadTripHistory()
  if (tab === "tournaments" && !tournament.value) void loadTournament()
})
watch(catchSection, (section) => {
  if (section !== "collection") exitCatchActionMode()
  if (section === "journal" && !journal.value.length) void loadJournal()
  if (section === "records") {
    if (!journal.value.length) void loadJournal()
    if (!leaderboard.value.length) void loadLeaderboard()
  }
})
watch(loadoutDialogOpen, (open) => {
  if (open) return
  tacklePickerOpen.value = false
  tacklePickerFieldKey.value = null
})
watch(
  () => `${fishingStore.trip?.tripId || ""}:${fishingStore.trip?.status || ""}`,
  (next, previous) => {
    if (next === previous) return
    if (previous.endsWith(":running") && !next.endsWith(":running")) {
      historyLoaded.value = false
      if (activeTab.value === "history") void loadTripHistory(true)
    }
    if (overview.value) {
      overview.value.currentTrip = fishingStore.trip?.status === "running" ? fishingStore.trip : null
      if (fishingStore.trip) overview.value.latestTrip = fishingStore.trip
    }
    void loadOverview()
  }
)
watch(activeTrip, (trip) => {
  if (!trip) return
  const changedTrip = liveEventTripId !== trip.tripId
  if (changedTrip) {
    liveEventTripId = trip.tripId
    liveEventHistory.value = []
  }
  const incoming = trip.summary?.recentEvents || []
  if (!incoming.length) return
  const log = liveLogEl.value
  const previousHeight = log?.scrollHeight || 0
  const previousTop = log?.scrollTop || 0
  const shouldStick = changedTrip || !log || previousTop < 72
  const previousLast = liveEventHistory.value[liveEventHistory.value.length - 1]?.sequence || 0
  const merged = new Map(liveEventHistory.value.map((event) => [event.sequence, event]))
  for (const event of incoming) merged.set(event.sequence, event)
  liveEventHistory.value = [...merged.values()].sort((a, b) => a.sequence - b.sequence).slice(-120)
  const nextLast = liveEventHistory.value[liveEventHistory.value.length - 1]?.sequence || 0
  if (nextLast > previousLast) {
    void nextTick(() => {
      const nextLog = liveLogEl.value
      if (!nextLog) return
      nextLog.scrollTop = shouldStick ? 0 : previousTop + nextLog.scrollHeight - previousHeight
    })
  }
}, { immediate: true })

onMounted(async () => {
  liveClockTimer = window.setInterval(() => { liveClock.value = Date.now() }, 500)
  overviewTimer = window.setInterval(() => { if (document.visibilityState === "visible") void loadOverview() }, 30_000)
  document.addEventListener("visibilitychange", handleVisibilityChange)
  await Promise.all([claimStarterOnce(), ensureKittiesInitialized(gameStore.token)])
  await Promise.all([loadOverview(), loadLoadouts(), loadTournament(false)])
})

onUnmounted(() => {
  tripEquipmentLoadGeneration++
  catchLoadGeneration++
  if (liveClockTimer !== undefined) window.clearInterval(liveClockTimer)
  if (overviewTimer !== undefined) window.clearInterval(overviewTimer)
  document.removeEventListener("visibilitychange", handleVisibilityChange)
})

function handleVisibilityChange() {
  if (document.visibilityState === "visible") void loadOverview()
}

async function refreshPage() {
  await loadOverview()
  if (activeTab.value === "history") await loadTripHistory(true)
}

async function claimStarterOnce() {
  try {
    const result = await claimFishingStarter(gameStore.token)
    if (result.granted) ElMessage.success(t("game.fishing.starterReady"))
    await inventoryStore.refreshAll(gameStore.token)
  } catch {
    // A temporary gift-service issue must not block the page.
  }
}

async function loadOverview() {
  const first = !overview.value
  if (first) loading.value = true
  else refreshing.value = true
  try {
    const payload = await fetchFishingOverview(gameStore.token)
    payload.spotCounts ||= {}
    overview.value = payload
    fishingStore.setTrip(payload.currentTrip || payload.latestTrip || null)
    syncTripActor(payload)
    const firstUnlocked = payload.waters.find(waterUnlocked)
    if (!selectedWaterId.value) selectedWaterId.value = payload.currentTrip?.waterId || firstUnlocked?.id || payload.waters[0]?.id || ""
    if (!selectedSpotId.value || !payload.spots.some((spot) => spot.id === selectedSpotId.value && spot.waterId === selectedWaterId.value)) {
      selectedSpotId.value = payload.currentTrip?.spotId || payload.spots.find((spot) => spot.waterId === selectedWaterId.value)?.id || ""
    }
    if (!recordSpeciesId.value) recordSpeciesId.value = payload.species[0]?.id || ""
  } catch {
    if (first) ElMessage.error(t("game.fishing.loadFailed"))
  } finally {
    loading.value = false
    refreshing.value = false
  }
}

async function loadLoadouts() {
  try {
    loadouts.value = await fetchFishingLoadouts(gameStore.token)
    if (!tripLoadoutId.value) tripLoadoutId.value = loadouts.value[0]?.loadoutId || ""
  } catch {
    ElMessage.error(t("game.fishing.loadoutsFailed"))
  }
}

async function loadTripHistory(force = false) {
  if (historyLoading.value || (historyLoaded.value && !force)) return
  historyLoading.value = true
  try {
    const payload = await fetchFishingTripHistory(gameStore.token)
    tripHistory.value = Array.isArray(payload.items) ? payload.items : []
    tripHistoryLimit.value = Math.max(1, Math.trunc(Number(payload.limit || 20)))
    const retainedTripIds = new Set(tripHistory.value.map((trip) => trip.tripId))
    for (const tripId of Object.keys(historyEventStates)) {
      if (!retainedTripIds.has(tripId)) delete historyEventStates[tripId]
    }
    historyLoaded.value = true
  } catch {
    ElMessage.error(t("game.fishing.historyLoadFailed"))
  } finally {
    historyLoading.value = false
  }
}

function ensureHistoryEventState(tripId: string): FishingTripEventHistoryState {
  historyEventStates[tripId] ||= {
    items: [], total: 0, nextCursor: 0, historyComplete: false,
    loading: false, loaded: false, error: false
  }
  return historyEventStates[tripId]
}

function historyEventState(tripId: string) { return historyEventStates[tripId] }

async function loadHistoryTripEvents(trip: FishingTrip, older = false) {
  const state = ensureHistoryEventState(trip.tripId)
  if (state.loading || (!older && state.loaded) || (older && !state.nextCursor)) return
  state.loading = true
  state.error = false
  try {
    const payload = await fetchFishingTripEvents(gameStore.token, trip.tripId, older ? state.nextCursor : 0)
    const incoming = Array.isArray(payload.items) ? payload.items : []
    const merged = new Map<number, FishingTripEvent>()
    for (const event of older ? state.items : []) merged.set(event.sequence, event)
    for (const event of incoming) merged.set(event.sequence, event)
    state.items = [...merged.values()].sort((left, right) => right.sequence - left.sequence)
    state.total = Math.max(0, Math.trunc(Number(payload.total || 0)))
    state.nextCursor = Math.max(0, Math.trunc(Number(payload.nextCursor || 0)))
    state.historyComplete = Boolean(payload.historyComplete)
    state.loaded = true
  } catch {
    state.error = true
  } finally {
    state.loading = false
  }
}

function handleHistoryTripToggle(event: Event, trip: FishingTrip) {
  const details = event.currentTarget as HTMLDetailsElement | null
  if (details?.open) void loadHistoryTripEvents(trip)
}

function historyEventsInitialLoading(tripId: string) {
  const state = historyEventState(tripId)
  return !state || (state.loading && !state.loaded)
}

function historyEventStateCopy(tripId: string) {
  const state = historyEventState(tripId)
  if (!state?.loaded) return ""
  if (state.nextCursor) return t("game.fishing.historyEventsLoaded", { loaded: state.items.length, total: state.total })
  if (state.historyComplete) return t("game.fishing.historyEventsComplete", { count: state.total })
  return t("game.fishing.historyEventsPartial", { count: state.total })
}

function applyCatchFilters() {
  appliedCatchFilters.value = { ...catchFilters.value }
  filterDialogOpen.value = false
  exitCatchActionMode()
}

async function loadCatchData() {
  const generation = ++catchLoadGeneration
  catchesLoading.value = true
  try {
    const loaded: FishingCatch[] = []
    const seenInstances = new Set<number>()
    const seenCursors = new Set<number>()
    let cursor = 0
    while (true) {
      if (cursor > 0) {
        if (seenCursors.has(cursor)) break
        seenCursors.add(cursor)
      }
      const page = await fetchFishingCatches(gameStore.token, {
        cursor: cursor || undefined,
        limit: 100,
        kept: true
      })
      if (generation !== catchLoadGeneration) return
      for (const row of page.items || []) {
        const instanceId = Number(row.inventoryInstanceId || 0)
        if (instanceId <= 0 || seenInstances.has(instanceId)) continue
        seenInstances.add(instanceId)
        loaded.push(row)
      }
      const nextCursor = Math.max(0, Math.trunc(Number(page.nextCursor || 0)))
      if (nextCursor <= 0) break
      cursor = nextCursor
    }
    catches.value = loaded
    selectedCatchIds.value = new Set()
    focusedCatchSpeciesId.value = ""
    catchActionMode.value = ""
    pondSelection.value = null
  } catch {
    if (generation === catchLoadGeneration) ElMessage.error(t("game.fishing.catchesFailed"))
  } finally {
    if (generation === catchLoadGeneration) catchesLoading.value = false
  }
}

async function loadJournal() {
  try { journal.value = await fetchFishingJournal(gameStore.token) } catch { ElMessage.error(t("game.fishing.journalFailed")) }
}
async function loadLeaderboard() {
  if (!recordSpeciesId.value) return
  try { leaderboard.value = await fetchFishingLeaderboard(gameStore.token, recordSpeciesId.value, recordMetric.value) } catch { ElMessage.error(t("game.fishing.recordsFailed")) }
}
async function loadTournament(showError = true) {
  try { tournament.value = await fetchFishingTournaments(gameStore.token) } catch { if (showError) ElMessage.error(t("game.fishing.tournamentFailed")) }
}

function selectWater(id: string) {
  const water = overview.value?.waters.find((row) => row.id === id)
  if (!water || !waterUnlocked(water)) return
  selectedWaterId.value = id
  selectedSpotId.value = overview.value?.spots.find((spot) => spot.waterId === id)?.id || ""
}

function openTripDialog() {
  tripStep.value = 1
  tripTournamentDivision.value = "open"
  const choice = loadouts.value.find((row) => row.loadoutId === tripLoadoutId.value) || loadouts.value[0]
  selectTripLoadout(choice?.loadoutId || "")
  void loadTournament(false)
  tripDialogOpen.value = true
}

function selectTripLoadout(id: string) {
  tripLoadoutId.value = id
  const choice = loadouts.value.find((row) => row.loadoutId === id)
  tripStrategy.value = choice?.strategy || "balanced"
  tripTargetLayer.value = "auto"
  tripPace.value = "balanced"
  resetTripSupplies(choice || null)
  if (tripTournamentDivision.value === "standard" && !loadoutMatchesTournamentStandard(choice || null)) tripTournamentDivision.value = "open"
}

function selectTournamentDivision(division: TournamentDivision) {
  if (division === "standard" && !selectedTripStandardEligible.value) return
  tripTournamentDivision.value = division
}

function selectTripActor(actorType: "player" | "kitty", actorId: number) {
  const angler = anglerFor(actorType, actorId)
  if (!angler || anglerUnavailable(angler)) return
  tripActorType.value = actorType
  tripActorId.value = actorType === "kitty" ? actorId : 0
  reconcileWaterForSelectedAngler(true)
}

function syncTripActor(payload: FishingOverview) {
  const current = payload.currentTrip
  if (current) {
    tripActorType.value = current.actorType === "kitty" ? "kitty" : "player"
    tripActorId.value = current.actorType === "kitty" ? current.actorId : 0
    tripActorInitialized = true
    return
  }
  if (tripActorInitialized && anglerFor(tripActorType.value, tripActorId.value)) return
  const previous = payload.latestTrip
  if (previous && anglerFor(previous.actorType, previous.actorId)) {
    tripActorType.value = previous.actorType === "kitty" ? "kitty" : "player"
    tripActorId.value = previous.actorType === "kitty" ? previous.actorId : 0
  } else {
    tripActorType.value = "player"
    tripActorId.value = 0
  }
  tripActorInitialized = true
}

function selectAnglerCommand(command: unknown) {
  const [actorType, rawActorId] = String(command || "").split(":")
  if (actorType !== "player" && actorType !== "kitty") return
  const actorId = actorType === "kitty" ? Math.max(0, Math.trunc(Number(rawActorId || 0))) : 0
  if (actorType === "kitty" && actorId <= 0) return
  selectTripActor(actorType, actorId)
}

function anglerKey(actorType: "player" | "kitty", actorId: number) {
  return actorType === "kitty" ? `kitty:${Math.max(0, Math.trunc(Number(actorId || 0)))}` : "player:0"
}

function anglerFor(actorType: "player" | "kitty", actorId: number) {
  const key = anglerKey(actorType, actorId)
  return anglerOptions.value.find((row) => anglerKey(row.actorType, row.actorId) === key)
}

function anglerName(angler?: FishingAngler) {
  if (!angler || angler.actorType === "player") return playerAnglerName.value
  return String(angler.name || kitties.value.find((kitty) => kitty.id === angler.actorId)?.name || t("game.fishing.kittyAngler"))
}

function anglerUnavailable(angler: FishingAngler) {
  return angler.actorType === "kitty" && angler.status === "in_combat"
}

function anglerStatusCopy(angler: FishingAngler) {
  return angler.actorType === "kitty" ? fishingKittyStatus(angler.status || "idle") : t("game.fishing.readyToFish")
}

function reconcileWaterForSelectedAngler(announce: boolean) {
  const water = selectedWater.value
  if (!water || waterUnlocked(water)) return
  const fallback = [...(overview.value?.waters || [])]
    .filter(waterUnlocked)
    .sort((left, right) => right.order - left.order)[0]
  if (!fallback) return
  selectWater(fallback.id)
  if (announce) {
    ElMessage.info(t("game.fishing.anglerWaterAdjusted", { name: selectedAnglerName.value, water: waterName(fallback) }))
  }
}

function fishingKittyStatus(status: string) {
  if (status === "in_combat") return t("game.fishing.catBusy")
  if (status === "in_life_queue") return t("game.fishing.catQueued")
  return t("game.fishing.readyToFish")
}

function resetTripSupplies(loadout: FishingLoadout | null) {
  for (const key of Object.keys(tripSupplies) as SupplyKey[]) tripSupplies[key] = 0
  const defaults: Record<SupplyKey, number> = { mainLine: 8, leader: 8, hook: 12, floatOrSinker: 4, lure: 8, bait: 120 }
  for (const row of buildSupplyRows(loadout)) tripSupplies[row.key] = Math.min(defaults[row.key], row.owned)
}

async function startTrip() {
  if (!canStartTrip.value) return
  startingTrip.value = true
  try {
    const supplies: FishingSupplyCounts = {
      mainLine: tripSupplies.mainLine,
      leader: tripSupplies.leader,
      hook: tripSupplies.hook,
      floatOrSinker: tripSupplies.floatOrSinker,
      lure: tripSupplies.lure,
      bait: tripSupplies.bait
    }
    await enqueueFishingTrip(gameStore.token, {
      spotId: selectedSpotId.value,
      loadoutId: tripLoadoutId.value,
      tournamentDivision: tripTournamentDivision.value,
      durationSeconds: durationSeconds.value,
      strategy: tripStrategy.value,
      targetLayer: tripTargetLayer.value,
      pace: tripPace.value,
      keepRule: { mode: keepMode.value, minWeightKg: keepMode.value === "min_weight" ? minimumKeepWeight.value : undefined },
      supplies
    }, { actorType: tripActorType.value, actorId: tripActorId.value })
    tripDialogOpen.value = false
    ElMessage.success(t("game.fishing.tripStarted"))
    await loadOverview()
  } catch (error) {
    ElMessage.error({
      message: resolveErrorMessage(t, error, "game.fishing.tripStartFailed"),
      duration: 10000,
      showClose: true,
      customClass: "fishing-start-error"
    })
  } finally {
    startingTrip.value = false
  }
}

async function confirmStopTrip() {
  if (!activeTrip.value) return
  let confirmed = false
  try {
    const fighting = activeTrip.value.phase === "bite" || activeTrip.value.phase === "fighting"
    await ElMessageBox.confirm(
      fighting ? t("game.fishing.stopDuringFightHint") : t("game.fishing.stopHint"),
      t("game.fishing.stopTitle"),
      { confirmButtonText: t("game.fishing.stopNow"), cancelButtonText: t("game.fishing.keepFishing"), type: "warning" }
    )
    confirmed = true
    stoppingTrip.value = true
    await stopFishingTrip(gameStore.token)
    historyLoaded.value = false
    ElMessage.success(t("game.fishing.stopSuccess"))
    await loadOverview()
  } catch {
    if (confirmed) ElMessage.error(t("game.fishing.stopFailed"))
  } finally {
    stoppingTrip.value = false
  }
}

function openLoadoutEditor(loadout?: FishingLoadout) {
  Object.assign(editingLoadout, loadout ? JSON.parse(JSON.stringify(loadout)) : {
    loadoutId: "",
    name: t("game.fishing.defaultLoadoutName"),
    method: "float",
    strategy: "balanced",
    safetyLoadPercent: 82,
    components: emptyComponents()
  })
  tacklePickerFieldKey.value = null
  tacklePickerOpen.value = false
  loadoutDialogOpen.value = true
}
function resetMethodComponents() {
  editingLoadout.components.hookItemId = ""
  editingLoadout.components.floatOrSinkerItemId = ""
  editingLoadout.components.lureItemId = ""
  editingLoadout.components.baitItemId = ""
}
function openTacklePicker(fieldKey: ComponentFieldKey) {
  tacklePickerFieldKey.value = fieldKey
  tacklePickerOpen.value = true
}
function selectTackleFromPicker(item: ItemBrowserItem | null) {
  const field = activeTackleField.value
  if (!field || !item) return
  if (field.key === "rodInstanceId" || field.key === "reelInstanceId") {
    editingLoadout.components[field.key] = Number(item.instanceId || 0)
  } else {
    editingLoadout.components[field.key] = item.itemId
  }
  tacklePickerOpen.value = false
  tacklePickerFieldKey.value = null
}
async function saveLoadout() {
  if (!loadoutComplete.value) return
  savingLoadout.value = true
  try {
    const payload = JSON.parse(JSON.stringify(editingLoadout)) as FishingLoadout
    payload.safetyLoadPercent = safetyLoadForStrategy(payload.strategy)
    await saveFishingLoadout(gameStore.token, payload)
    await loadLoadouts()
    loadoutDialogOpen.value = false
    ElMessage.success(t("game.fishing.loadoutSaved"))
  } catch {
    ElMessage.error(t("game.fishing.loadoutSaveFailed"))
  } finally {
    savingLoadout.value = false
  }
}
async function removeLoadout(loadout: FishingLoadout) {
  try {
    await ElMessageBox.confirm(t("game.fishing.deleteLoadoutHint", { name: loadoutName(loadout) }), t("game.fishing.deleteLoadoutTitle"), { confirmButtonText: t("game.fishing.delete"), cancelButtonText: t("game.fishing.cancel"), type: "warning" })
    await deleteFishingLoadout(gameStore.token, loadout.loadoutId)
    await loadLoadouts()
  } catch {
    // Keep the preset.
  }
}

function toggleCatch(instanceId?: number) {
  if (!instanceId) return
  const next = new Set(selectedCatchIds.value)
  if (next.has(instanceId)) {
    next.delete(instanceId)
  } else {
    if (batchSellMode.value && next.size >= FISHING_SALE_BATCH_MAX) {
      ElMessage.warning(t("game.fishing.saleBatchLimit", { count: FISHING_SALE_BATCH_MAX }))
      return
    }
    if (batchPondMode.value && next.size >= remainingPondSlots.value) {
      ElMessage.warning(t("game.fishing.pondSelectionFull", { count: remainingPondSlots.value }))
      return
    }
    next.add(instanceId)
  }
  selectedCatchIds.value = next
}
function saleIneligibleCopy(row: FishingCatch) {
  if (row.saleIneligibleReason === "cultured_immature") {
    return t("game.fishing.saleIneligibleCulturedImmature", {
      current: Math.max(0, Number(row.saleCurrentAgeDays ?? row.fish.ageDays ?? 0)),
      required: Math.max(0, Number(row.saleRequiredAgeDays || 0))
    })
  }
  if (row.saleIneligibleReason === "dead") return t("game.fishing.saleIneligibleDead")
  return t("game.fishing.saleIneligibleInvalid")
}
function isImmatureCulturedCatch(row: FishingCatch) {
  return row.saleEligible === false && row.saleIneligibleReason === "cultured_immature"
}
function catchCardAriaLabel(row: FishingCatch) {
  if (!catchSelectionMode.value) return t("game.fishing.inspectFish")
  if (batchSellMode.value && !canSelectCatch(row)) return saleIneligibleCopy(row)
  return batchPondMode.value ? t("game.fishing.batchStockPond") : t("game.fishing.batchSale")
}
function canSelectCatch(row: FishingCatch) {
  if (!row.kept || !row.inventoryInstanceId) return false
  if (batchSellMode.value && row.saleEligible === false) return false
  return !batchPondMode.value || pondSelectableIds.value.has(row.inventoryInstanceId)
}
function handleCatchCardClick(row: FishingCatch) {
  if (catchSelectionMode.value) {
    if (canSelectCatch(row)) {
      focusedCatchSpeciesId.value = row.speciesId
      toggleCatch(row.inventoryInstanceId)
    } else if (batchSellMode.value && row.saleEligible === false) {
      ElMessage.info(saleIneligibleCopy(row))
    }
    return
  }
  openCatchDetail(row)
}
function toggleBatchSaleMode() {
  if (batchSellMode.value) {
    exitCatchActionMode()
    return
  }
  catchActionMode.value = "sell"
  selectedCatchIds.value = new Set()
  focusedCatchSpeciesId.value = ""
}
async function toggleBatchPondMode() {
  if (batchPondMode.value) {
    exitCatchActionMode()
    return
  }
  if (loadingPondSelection.value) return
  loadingPondSelection.value = true
  try {
    const current = await loadAllAvailablePondFish(gameStore.token, await fetchFishingPond(gameStore.token))
    if (!current.built) {
      ElMessage.warning(t("game.fishing.pondNotBuilt"))
      return
    }
    pondSelection.value = current
    if (current.fish.filter((row) => fishingPondZone(row.fish) === "main").length >= current.limits.fishCapacity) {
      ElMessage.warning(t("game.fishing.pondSelectionFull", { count: 0 }))
      return
    }
    catchActionMode.value = "pond"
    selectedCatchIds.value = new Set()
    focusedCatchSpeciesId.value = ""
  } catch (error) {
    ElMessage.error(resolveErrorMessage(t, error, "game.fishing.pondLoadFailed"))
  } finally {
    loadingPondSelection.value = false
  }
}
function exitCatchActionMode() {
  catchActionMode.value = ""
  selectedCatchIds.value = new Set()
  focusedCatchSpeciesId.value = ""
  pondSelection.value = null
}
function selectAllFocusedCatchSpecies() {
  const speciesId = batchFocusedCatchSpeciesId.value
  if (!speciesId) return
  const selectionLimit = batchSellMode.value ? FISHING_SALE_BATCH_MAX : remainingPondSlots.value
  const next = new Set(selectedCatchIds.value)
  let skipped = 0
  for (const row of batchFocusedSameSpeciesBulkCatches.value) {
    const instanceId = Number(row.inventoryInstanceId || 0)
    if (instanceId <= 0 || next.has(instanceId)) continue
    if (next.size >= selectionLimit) {
      skipped++
      continue
    }
    next.add(instanceId)
  }
  focusedCatchSpeciesId.value = speciesId
  selectedCatchIds.value = next
  if (skipped <= 0) return
  if (batchSellMode.value) {
    ElMessage.warning(t("game.fishing.saleBatchLimit", { count: FISHING_SALE_BATCH_MAX }))
    return
  }
  ElMessage.warning(t("game.fishing.pondSelectionFull", { count: remainingPondSlots.value }))
}
async function sellSelectedCatches() {
  const instanceIds = [...selectedCatchIds.value]
  if (!instanceIds.length || sellingCatches.value) return
  sellingCatches.value = true
  try {
    saleQuote.value = await quoteFishingSale(gameStore.token, instanceIds)
    saleQuoteDialogOpen.value = true
  } catch (error) {
    ElMessage.error(resolveErrorMessage(t, error, "game.fishing.estimateFailed"))
  } finally {
    sellingCatches.value = false
  }
}
async function confirmFishingSale() {
  const quote = saleQuote.value
  if (!quote || confirmingSale.value) return
  const instanceIds = quote.items.map((line) => line.instanceId)
  confirmingSale.value = true
  try {
    const result = await sellFishingCatches(gameStore.token, instanceIds, quote.totalGold)
    ElMessage.success(t("game.fishing.soldFor", { gold: result.totalGold.toLocaleString(locale.value) }))
    saleQuoteDialogOpen.value = false
    saleQuote.value = null
    exitCatchActionMode()
    await Promise.all([loadCatchData(), inventoryStore.refreshAll(gameStore.token)])
  } catch (error) {
    if (error instanceof APIClientError && error.code === "FISHING_SALE_QUOTE_CHANGED") {
      try {
        saleQuote.value = await quoteFishingSale(gameStore.token, instanceIds)
        ElMessage.warning(t("game.fishing.saleQuoteChanged"))
      } catch (refreshError) {
        saleQuoteDialogOpen.value = false
        ElMessage.error(resolveErrorMessage(t, refreshError, "game.fishing.saleFailed"))
      }
      return
    }
    ElMessage.error(resolveErrorMessage(t, error, "game.fishing.saleFailed"))
  } finally {
    confirmingSale.value = false
  }
}
async function stockSelectedCatches() {
  const instanceIds = [...selectedCatchIds.value]
  if (!instanceIds.length || stockingCatches.value || instanceIds.length > remainingPondSlots.value) return
  stockingCatches.value = true
  try {
    const selectedIds = new Set(instanceIds)
    const selected = (pondSelection.value?.availableFish || []).filter((row) => selectedIds.has(row.instanceId))
    const mainFish = (pondSelection.value?.fish || []).filter((row) => fishingPondZone(row.fish) === "main")
    const introducedRisks = fishingPondPredationRisks(mergeFishingPondEcosystem(mainFish, selected))
      .filter((risk) => selectedIds.has(risk.predator.instanceId) || selectedIds.has(risk.prey.instanceId))
    if (introducedRisks.length) {
      const preyCount = new Set(introducedRisks.map((risk) => risk.prey.instanceId)).size
      await ElMessageBox.confirm(
        t("game.fishing.pondPredationStockConfirm", { count: preyCount }),
        t("game.fishing.pondPredationRiskTitle"),
        { type: "warning", confirmButtonText: t("game.fishing.pondConfirmStock"), cancelButtonText: t("game.fishing.cancel") }
      )
    }
    pondSelection.value = await addFishingPondFishBatch(gameStore.token, instanceIds)
    ElMessage.success(t("game.fishing.pondStockedBatch"))
    exitCatchActionMode()
    await Promise.all([loadCatchData(), inventoryStore.refreshAll(gameStore.token)])
  } catch (error) {
    if (String(error).includes("cancel")) return
    ElMessage.error(resolveErrorMessage(t, error, "game.fishing.pondActionFailed"))
  } finally {
    stockingCatches.value = false
  }
}
function openCatchDetail(row: FishingCatch) {
  selectedCatchDetail.value = row
  catchDetailDialogOpen.value = true
}
async function claimNextReward() {
  const reward = unclaimedRewards.value[0]
  if (!reward) return
  claimingReward.value = true
  try {
    await claimFishingTournamentReward(gameStore.token, reward.tournamentId, reward.division)
    ElMessage.success(t("game.fishing.rewardClaimed"))
    await loadTournament()
  } catch {
    ElMessage.error(t("game.fishing.rewardFailed"))
  } finally {
    claimingReward.value = false
  }
}
async function redeemCatchUp() {
  redeemingCatchUp.value = true
  try {
    await redeemFishingCatchUpRod(gameStore.token)
    ElMessage.success(t("game.fishing.redeemSuccess"))
    await loadTournament()
  } catch {
    ElMessage.error(t("game.fishing.redeemFailed"))
  } finally {
    redeemingCatchUp.value = false
  }
}

function buildSupplyRows(loadout: FishingLoadout | null): SupplyRow[] {
  if (!loadout) return []
  const rows: Array<[SupplyKey, string, string]> = [
    ["mainLine", loadout.components.mainLineItemId, t("game.fishing.mainLine")],
    ["leader", loadout.components.leaderItemId, t("game.fishing.leader")]
  ]
  if (loadout.method === "lure") {
    rows.push(["lure", loadout.components.lureItemId || "", t("game.fishing.lure")])
  } else {
    rows.push(["hook", loadout.components.hookItemId || "", t("game.fishing.hook")])
    rows.push(["floatOrSinker", loadout.components.floatOrSinkerItemId || "", loadout.method === "float" ? t("game.fishing.float") : t("game.fishing.sinker")])
    rows.push(["bait", loadout.components.baitItemId || "", t("game.fishing.bait")])
  }
  return rows
    .filter(([, itemId]) => Boolean(itemId))
    .map(([key, itemId, slotLabel]) => ({
      key,
      itemId,
      label: t(`item.${itemId}.name`, slotLabel),
      slotLabel,
      owned: ownedTackleQuantity(itemId)
    }))
}

function buildLiveSupplyRows(trip: FishingTrip | null) {
  const loadout = loadouts.value.find((row) => row.loadoutId === trip?.loadoutId) || null
  const remaining = trip?.supplies?.remaining
  return buildSupplyRows(loadout).map((row) => ({ ...row, quantity: Math.max(0, Number(remaining?.[row.key] || 0)) }))
}
function ownedTackleQuantity(itemId: string) {
  return (overview.value?.ownedTackle || []).filter((row) => row.itemId === itemId).reduce((sum, row) => sum + Math.max(0, Number(row.quantity || 0)), 0)
}
function waterName(water?: FishingWater | null) { return localizedFishingText(water?.names, locale.value) }
function waterDescription(water: FishingWater) { return localizedFishingText(water.descriptions, locale.value) }
function waterNameById(id: string) { const water = overview.value?.waters.find((row) => row.id === id); return water ? waterName(water) : id }
function spotName(spot: FishingSpot) { return localizedFishingText(spot.names, locale.value) }
function spotNameById(id: string) { const spot = overview.value?.spots.find((row) => row.id === id); return spot ? spotName(spot) : t("game.fishing.unknownSpot") }
function speciesById(id: string) { return speciesCatalogById.value.get(id) }
function speciesName(id: string) { return localizedFishingText(speciesById(id)?.names, locale.value) || t("game.fishing.unknownFish") }
function loadoutName(loadout: FishingLoadout) { return loadout.loadoutId === "starter-float-v1" ? t("game.fishing.starterLoadoutName") : loadout.name }
function tournamentTackleSlotLabel(slot: string) {
  if (slot === "rod") return t("game.fishing.rod")
  if (slot === "reel") return t("game.fishing.reel")
  if (slot === "main_line") return t("game.fishing.mainLine")
  if (slot === "leader") return t("game.fishing.leader")
  if (slot === "hook") return t("game.fishing.hook")
  if (slot === "float_or_sinker") return tournament.value?.standardLoadout?.method === "bottom" ? t("game.fishing.sinker") : t("game.fishing.float")
  if (slot === "lure") return t("game.fishing.lure")
  if (slot === "bait") return t("game.fishing.bait")
  return slot
}
function loadoutTournamentItems(loadout?: FishingLoadout | null) {
  if (!loadout) return new Map<string, string>()
  const equipmentItemId = (instanceId: number) => (overview.value?.ownedTackle || []).find((item) => item.instanceId === instanceId)?.itemId || ""
  const items = new Map<string, string>([
    ["rod", equipmentItemId(loadout.components.rodInstanceId)],
    ["reel", equipmentItemId(loadout.components.reelInstanceId)],
    ["main_line", loadout.components.mainLineItemId],
    ["leader", loadout.components.leaderItemId]
  ])
  if (loadout.method === "lure") {
    items.set("lure", loadout.components.lureItemId || "")
  } else {
    items.set("hook", loadout.components.hookItemId || "")
    items.set("float_or_sinker", loadout.components.floatOrSinkerItemId || "")
    items.set("bait", loadout.components.baitItemId || "")
  }
  return items
}
function loadoutMatchesTournamentStandard(loadout?: FishingLoadout | null) {
  const standard = tournament.value?.standardLoadout
  if (!loadout || !standard || loadout.method !== standard.method || !standard.items.length) return false
  const actual = loadoutTournamentItems(loadout)
  return actual.size === standard.items.length && standard.items.every((item) => actual.get(item.slot) === item.itemId)
}
function forecastBySpot(id: string) { return overview.value?.forecast.find((row) => row.spotId === id) }
function forecastForWater(id: string) { const spot = overview.value?.spots.find((row) => row.waterId === id); return spot ? forecastBySpot(spot.id) : undefined }
function waterUnlocked(water: FishingWater) { return selectedFishingLevel.value >= water.requiredFishingLevel }
function knowledgeBySpecies(id: string) { return journal.value.find((row) => row.speciesId === id) }
function anglersCopy(spotId: string) { return t("game.fishing.anglersCount", { count: overview.value?.spotCounts?.[spotId] || 0 }) }
function activityCopy(band?: string) { return band === "busy" ? t("game.fishing.activityBusy") : band === "quiet" ? t("game.fishing.activityQuiet") : t("game.fishing.activitySteady") }
function phaseCopy(phase?: string) { return phase === "bite" ? t("game.fishing.phaseBite") : phase === "fighting" ? t("game.fishing.phaseFighting") : phase === "landed" ? t("game.fishing.phaseLanded") : phase === "resting" ? t("game.fishing.phaseResting") : phase === "finished" ? t("game.fishing.phaseFinished") : t("game.fishing.phaseWaiting") }
function phaseIcon(phase?: string) { return phase === "fighting" ? "lucide:activity" : phase === "landed" ? "lucide:check-circle-2" : phase === "bite" ? "lucide:circle-dot" : "lucide:waves" }
function lineStateCopy(state?: string) { return state === "critical" ? t("game.fishing.lineCritical") : state === "tight" ? t("game.fishing.lineTight") : t("game.fishing.lineSteady") }
function weatherLabel(weather?: string) { return t(`game.fishing.weather_${weather || "clear"}`) }
function weatherIcon(weather?: string) {
  const icons: Record<string, string> = { clear: "lucide:sun", sunny: "lucide:sun", cloud: "lucide:cloud", rain: "lucide:cloud-rain", storm: "lucide:cloud-lightning", mist: "lucide:cloud-fog", snow: "lucide:snowflake" }
  return icons[weather || "clear"] || icons.clear
}
function eventSpecies(event: FishingTripEvent) { return event.speciesId ? speciesById(event.speciesId) : undefined }
function sizeClassCopy(size?: FishingTripEvent["sizeClass"]) {
  return size === "large" ? t("game.fishing.fishSizeLarge") : size === "small" ? t("game.fishing.fishSizeSmall") : size === "medium" ? t("game.fishing.fishSizeMedium") : t("game.fishing.fishSizeUnknown")
}
function eventTitle(event: FishingTripEvent) {
  if (event.kind === "cast") return t("game.fishing.eventCastTitle", { count: event.castNumber || 1 })
  if (event.kind === "quiet") return t("game.fishing.eventQuietTitle")
  if (event.kind === "bite") return t("game.fishing.eventBiteTitle")
  if (event.kind === "hooked") return t("game.fishing.eventHookedTitle")
  if (event.kind === "landed") return t("game.fishing.eventLandedTitle", { fish: event.speciesId ? speciesName(event.speciesId) : t("game.fishing.unknownFish") })
  if (event.kind === "line_break") return t("game.fishing.eventLineBreakTitle")
  if (event.kind === "escaped") return event.reason === "lost_control" ? t("game.fishing.eventLostControlTitle") : t("game.fishing.eventEscapedTitle")
  return t("game.fishing.eventFinishedTitle")
}
function eventDetail(event: FishingTripEvent) {
  if (event.kind === "cast") return t("game.fishing.eventCastDetail")
  if (event.kind === "quiet") return t("game.fishing.eventQuietDetail")
  if (event.kind === "bite") return t("game.fishing.eventBiteDetail", { size: sizeClassCopy(event.sizeClass) })
  if (event.kind === "hooked") return t("game.fishing.eventHookedDetail", { size: sizeClassCopy(event.sizeClass) })
  if (event.kind === "landed") {
    const disposition = event.disposition === "released" ? t("game.fishing.eventReleased") : t("game.fishing.eventKept")
    return t("game.fishing.eventLandedDetail", { weight: formatWeight(event.weightKg || 0), length: formatLength(event.lengthCm || 0), disposition })
  }
  if (event.kind === "line_break") return event.reason === "line_overload" ? t("game.fishing.eventLineOverloadDetail") : t("game.fishing.eventLineBreakDetail")
  if (event.kind === "escaped") return event.reason === "lost_control" ? t("game.fishing.eventLostControlDetail") : t("game.fishing.eventEscapedDetail")
  if (event.reason === "time_reached") return t("game.fishing.eventFinishedTime")
  if (event.reason === "bait_empty") return t("game.fishing.eventFinishedBait")
  if (event.reason === "tackle_empty") return t("game.fishing.eventFinishedTackle")
  return t("game.fishing.eventFinishedDetail")
}
function eventIcon(event: FishingTripEvent) {
  if (event.kind === "cast") return "lucide:send"
  if (event.kind === "quiet") return "lucide:waves"
  if (event.kind === "bite") return "lucide:circle-alert"
  if (event.kind === "hooked") return "game-icons:fishing-hook"
  if (event.kind === "landed") return "lucide:fish-symbol"
  if (event.kind === "line_break") return "lucide:unplug"
  if (event.kind === "escaped") return "lucide:fish-off"
  return "lucide:flag"
}
function eventTone(event: FishingTripEvent) {
  if (event.kind === "landed") return "success"
  if (event.kind === "escaped" || event.kind === "line_break") return "danger"
  if (event.kind === "bite" || event.kind === "hooked") return "active"
  return "calm"
}
function eventTime(value: string) {
  const date = new Date(value)
  if (!Number.isFinite(date.getTime())) return ""
  return new Intl.DateTimeFormat(locale.value, { hour: "2-digit", minute: "2-digit", second: "2-digit", hour12: false }).format(date)
}
function tripAnglerName(trip: FishingTrip) {
  const angler = anglerOptions.value.find((row) => anglerKey(row.actorType, row.actorId) === anglerKey(trip.actorType, trip.actorId))
  if (angler) return anglerName(angler)
  return trip.actorType === "kitty" ? t("game.fishing.kittyAngler") : playerAnglerName.value
}
function historyStatusCopy(trip: FishingTrip) { return trip.status === "stopped" ? t("game.fishing.historyStopped") : t("game.fishing.historyCompleted") }
function tripHistoryEvents(trip: FishingTrip) { return historyEventState(trip.tripId)?.items || [] }
function formatTripStartedAt(value: string) {
  const date = new Date(value)
  if (!Number.isFinite(date.getTime())) return ""
  return new Intl.DateTimeFormat(locale.value, { year: "numeric", month: "short", day: "numeric", hour: "2-digit", minute: "2-digit" }).format(date)
}
function formatTripDuration(trip: FishingTrip) {
  const startedAt = Date.parse(trip.startedAt)
  const endedAt = Date.parse(trip.endedAt || trip.phaseEndsAt)
  if (!Number.isFinite(startedAt) || !Number.isFinite(endedAt) || endedAt < startedAt) return t("game.fishing.historyDurationUnknown")
  return formatDuration(Math.round((endedAt - startedAt) / 1000))
}
function waterIcon(kind: string) { return kind.includes("river") ? "lucide:route" : kind.includes("deep") ? "lucide:mountain-snow" : "lucide:waves" }
function trendIcon(trend?: string) { return trend === "rising" ? "lucide:trending-up" : trend === "falling" ? "lucide:trending-down" : "lucide:minus" }
function trendCopy(trend?: string) { return trend === "rising" ? t("game.fishing.trendRising") : trend === "falling" ? t("game.fishing.trendFalling") : t("game.fishing.trendSteady") }
function strangeWindowCopy(species: FishingSpecies) { return t("game.fishing.strangeWindow", { weather: weatherLabel(species.windowWeather), hours: species.activeHours.map(formatGameHour).join(" · ") }) }
function challengeCopy(value: number) {
  if (value <= 0.75) return t("game.fishing.challengeEasy")
  if (value <= 1.5) return t("game.fishing.challengeSteady")
  if (value <= 2.25) return t("game.fishing.challengeDemanding")
  return t("game.fishing.challengeExtreme")
}
function methodIcon(method: FishingMethod) { return method === "lure" ? "lucide:fish-symbol" : method === "bottom" ? "lucide:anchor" : "lucide:circle-dot" }
function methodLabel(method: FishingMethod) { return t(`game.fishing.method_${method}`) }
function strategyLabel(strategy: FishingStrategy) { return t(`game.fishing.strategy_${strategy}`) }
function strategyDescription(strategy: FishingStrategy) { return t(`game.fishing.strategy_${strategy}_hint`) }
function targetLayerLabel(layer: FishingTargetLayer) { return t(`game.fishing.layer_${layer}`) }
function targetLayerDescription(layer: FishingTargetLayer) {
  if (layer === "auto" && selectedTripLoadout.value?.method === "bottom") return t("game.fishing.layer_auto_bottom_hint")
  return t(`game.fishing.layer_${layer}_hint`)
}
function paceLabel(pace: FishingPace) { return t(`game.fishing.pace_${pace}`) }
function paceDescription(pace: FishingPace) { return t(`game.fishing.pace_${pace}_hint`) }
function safetyLoadForStrategy(strategy: FishingStrategy) { return strategy === "conservative" ? 94 : strategy === "aggressive" ? 70 : 82 }
function preferredSpeciesLayers(species?: FishingSpecies): SpeciesWaterLayer[] {
  if (species?.preferredLayers?.length) return species.preferredLayers
  if (species?.depth === "shallow") return ["surface"]
  if (species?.depth === "mid") return ["midwater"]
  if (species?.depth === "bottom" || species?.depth === "deep") return ["near_bottom"]
  return []
}
function speciesLayerSummary(species?: FishingSpecies) { return preferredSpeciesLayers(species).map((layer) => targetLayerLabel(layer)).join(" / ") }
function speciesUsesLayer(species: FishingSpecies | undefined, layer: SpeciesWaterLayer) { return preferredSpeciesLayers(species).includes(layer) }
function metricPercent(value: number) { return Math.max(0, Math.min(100, Math.round(Number(value || 0) * 100))) }
function spotMethodCopy(spot: FishingSpot) { return spot.methods.map(methodLabel).join(" / ") }
function spotSpeciesRows(spot: FishingSpot): SpotSpeciesRow[] {
  const rows = Object.entries(spot.speciesWeights || {}).flatMap(([speciesId, rawWeight]) => {
    const species = speciesById(speciesId)
    const weight = Number(rawWeight)
    return species && Number.isFinite(weight) && weight > 0 ? [{ species, weight, share: 0 }] : []
  })
  const total = rows.reduce((sum, row) => sum + row.weight, 0)
  return rows
    .map((row) => ({ ...row, share: total > 0 ? row.weight / total : 0 }))
    .sort((a, b) => b.weight - a.weight || speciesName(a.species.id).localeCompare(speciesName(b.species.id), locale.value))
}
function spotSpeciesSummary(spot: FishingSpot) {
  const rows = spotSpeciesRows(spot)
  const names = rows.slice(0, 3).map((row) => speciesName(row.species.id)).join(" / ")
  if (rows.length > 3) return t("game.fishing.spotFishSummaryMore", { names, count: rows.length })
  return t("game.fishing.spotFishSummary", { names })
}
function formatSpotShare(value: number) { return Math.max(0, Math.round(Number(value || 0) * 1000) / 10).toFixed(1).replace(/\.0$/, "") }
function depthCopy(spot: FishingSpot) { return spot.depthM < 2 ? t("game.fishing.shallowWater") : spot.depthM > 6 ? t("game.fishing.deepWater") : t("game.fishing.midWater") }
function pressureCopy(value: number) { return value > 0.62 ? t("game.fishing.pressureTired") : value > 0.28 ? t("game.fishing.pressureSettling") : t("game.fishing.pressureFresh") }
function rarityLabel(rarity: string) { return t(`game.fishing.rarity_${rarity}`) }
type CurrentInventoryFish = InventoryInstance & { fishData: NonNullable<InventoryInstance["fishData"]> }
type CatchSaleState = Pick<FishingCatch, "saleEligible" | "saleIneligibleReason" | "saleCurrentAgeDays" | "saleRequiredAgeDays">

function isCurrentInventoryFish(instance: InventoryInstance): instance is CurrentInventoryFish {
  return instance.templateItemId === "fish_catch"
    && instance.state === "inventory"
    && !instance.archivedAt
    && Number.isFinite(instance.instanceId)
    && instance.instanceId > 0
    && Boolean(instance.fishData?.speciesId)
}

function inventoryFishAsCatch(instance: CurrentInventoryFish): FishingCatch {
  const mutations = fishMutationIds(instance.fishData)
  const fish: FishIndividual = {
    ...instance.fishData,
    mutations,
    genes: {
      ...instance.fishData.genes,
      mutations
    }
  }
  const recordFlags = new Set(fish.recordFlags || [])
  return {
    catchId: -instance.instanceId,
    tripId: "",
    speciesId: fish.speciesId,
    fish,
    kept: true,
    inventoryInstanceId: instance.instanceId,
    caughtAt: fish.caughtAt,
    record: {
      speciesId: fish.speciesId,
      length: recordFlags.has("length"),
      weight: recordFlags.has("weight")
    },
    ...inventoryFishSaleState(fish)
  }
}

function inventoryFishSaleState(fish: FishIndividual): CatchSaleState {
  const species = speciesCatalogById.value.get(fish.speciesId)
  if (!species
    || species.baseValue <= 0
    || species.minLengthCm <= 0
    || species.maxLengthCm < species.minLengthCm
    || species.shapeFactor <= 0
    || fish.ageDays < 0
    || fish.lengthCm <= 0
    || fish.weightKg <= 0
  ) {
    return { saleEligible: false, saleIneligibleReason: "invalid_data", saleCurrentAgeDays: fish.ageDays }
  }
  if ((fish.lifeState || "alive") !== "alive") {
    return { saleEligible: false, saleIneligibleReason: "dead", saleCurrentAgeDays: fish.ageDays }
  }
  if (fish.origin === "cultured") {
    const requiredAgeDays = Number(pondSpeciesById.value.get(fish.speciesId)?.maturityAgeDays || 0)
    if (requiredAgeDays <= 0) {
      return { saleEligible: false, saleIneligibleReason: "invalid_data", saleCurrentAgeDays: fish.ageDays }
    }
    if (fish.ageDays < requiredAgeDays) {
      return {
        saleEligible: false,
        saleIneligibleReason: "cultured_immature",
        saleCurrentAgeDays: fish.ageDays,
        saleRequiredAgeDays: requiredAgeDays
      }
    }
  }
  return { saleEligible: true, saleCurrentAgeDays: fish.ageDays }
}

function currentCatchFish(row: FishingCatch): FishIndividual {
  const instanceId = Number(row.inventoryInstanceId || 0)
  if (instanceId <= 0) return row.fish
  const current = inventoryStore.instancesById.get(instanceId)?.fishData
  if (!current || current.speciesId !== row.speciesId) return row.fish
  const mutations = fishMutationIds(current)
  return {
    ...row.fish,
    ...current,
    mutations,
    genes: {
      ...row.fish.genes,
      ...current.genes,
      mutations
    }
  }
}
function hasRecord(row: FishingCatch) { return Boolean(row.record?.length || row.record?.weight || row.fish.recordFlags?.length) }
function hasFishMutation(row: FishingCatch) { return fishMutationIds(row.fish).length > 0 }
function catchMutations(row: FishingCatch) { return resolveFishMutations(row.fish, overview.value?.mutations) }
function formatTournamentStandardNumber(value: number) { return Number(value || 0).toFixed(2).replace(/\.00$/, "").replace(/(\.\d)0$/, "$1") }
function formatLength(value: number) { return `${Number(value || 0).toFixed(1)} cm` }
function formatWeight(value: number) { return `${Number(value || 0).toFixed(value >= 10 ? 1 : 2)} kg` }
function formatFactor(value: number) { return `×${Number(value || 0).toFixed(3)}` }
function factorTone(value: number) { const delta = Number(value || 0) - 1; return delta > 0.0005 ? "is-positive" : delta < -0.0005 ? "is-negative" : "is-neutral" }
function formatMarketDate(value: string) { const date = new Date(`${value}T00:00:00Z`); return Number.isNaN(date.getTime()) ? value : new Intl.DateTimeFormat(locale.value, { month: "short", day: "numeric", timeZone: "UTC" }).format(date) }
function formatGameHour(hour?: number) { const h = Math.floor(Number(hour || 0)); const minute = Math.round((Number(hour || 0) - h) * 60); return `${String(h).padStart(2, "0")}:${String(minute).padStart(2, "0")}` }
function formatDuration(seconds: number) { return seconds >= 3600 ? t("game.fishing.hours", { value: Math.round(seconds / 360) / 10 }) : t("game.fishing.minutes", { value: Math.round(seconds / 60) }) }
function formatCaughtAt(value: string) { const date = new Date(value); return Number.isNaN(date.getTime()) ? "" : new Intl.DateTimeFormat(locale.value, { month: "short", day: "numeric", hour: "2-digit", minute: "2-digit" }).format(date) }
function ageCopy(days: number) { const safe = Math.max(1, Math.round(days || 1)); if (safe < 60) return t("game.fishing.ageDays", { days: safe }); const months = Math.floor(safe / 30); return months < 24 ? t("game.fishing.ageMonths", { months }) : t("game.fishing.ageYearsMonths", { years: Math.floor(months / 12), months: months % 12 }) }
function sexCopy(sex: "male" | "female") { return t(`game.fishing.sex_${sex}`) }
function conditionCopy(value: number) { return value >= 1.04 ? t("game.fishing.conditionPrime") : value < 0.9 ? t("game.fishing.conditionLean") : t("game.fishing.conditionGood") }
function healthCopy(value: number) { return value >= 0.92 ? t("game.fishing.healthGood") : t("game.fishing.healthFair") }
function pondSpeciesProfile(speciesId: string): FishingPondSpeciesProfile | undefined { return pondSpeciesById.value.get(speciesId) }
function pondDietLabel(speciesId: string) { const profile = pondSpeciesProfile(speciesId); return profile ? t(`game.fishing.pondDiet_${profile.dietClass}`) : "" }
function pondPredatorGuide(speciesId: string) { const config = pondSpeciesProfile(speciesId)?.predation; return config ? t("game.fishing.pondPredatorGuide", { ratio: Math.round(config.maxPreyLengthRatio * 100), satiety: Math.round(config.huntBelowSatiety), chance: Math.round(config.dailyHuntChance * 100) }) : "" }
function formatTournamentRemaining(value: string) { const seconds = Math.max(0, Math.floor((Date.parse(value) - Date.now()) / 1000)); const days = Math.floor(seconds / 86400); const hours = Math.floor((seconds % 86400) / 3600); return days ? t("game.fishing.daysHours", { days, hours }) : t("game.fishing.hours", { value: hours }) }
function tripEndCopy(trip: FishingTrip) { if (!trip.plannedEndAt) return t("game.fishing.untilSuppliesEnd"); const date = new Date(trip.plannedEndAt); return Number.isNaN(date.getTime()) ? t("game.fishing.untilSuppliesEnd") : t("game.fishing.endsAt", { time: new Intl.DateTimeFormat(locale.value, { hour: "2-digit", minute: "2-digit" }).format(date) }) }
function loadoutComponentSummary(loadout: FishingLoadout) { return t("game.fishing.tacklePieces", { count: Object.values(loadout.components).filter(Boolean).length }) }
function tackleForField(slot: string) { return (overview.value?.ownedTackle || []).filter((item) => item.slot === slot) }
function selectedTackleForField(field: TackleField) {
  const selected = editingLoadout.components[field.key]
  if (!selected) return undefined
  return tackleForField(field.slot).find((item) => field.instance ? item.instanceId === Number(selected) : item.itemId === String(selected))
}
function tacklePreview(item?: FishingOwnedTackle) {
  if (!item?.instanceId) return null
  return {
    enhanceLevel: item.enhanceLevel || 0,
    durability: item.durability || 0,
    maxDurability: item.maxDurability || 0
  }
}
function tackleLabel(item: FishingOwnedTackle) { const label = t(`item.${item.itemId}.name`, item.itemId); return item.instanceId ? `${label} · +${item.enhanceLevel || 0}` : `${label} · ×${item.quantity}` }
function tackleImage(itemId: string) { return `/assets/icons/item/${itemId}.png` }
function loadoutRodIcon(loadout: FishingLoadout) { const rod = (overview.value?.ownedTackle || []).find((item) => item.instanceId === loadout.components.rodInstanceId); return tackleImage(rod?.itemId || "basic_fishing_rod") }
</script>

<style scoped>
:global(.el-message.fishing-start-error) {
  max-width: min(560px, calc(100vw - 24px));
}
:global(.fishing-start-error .el-message__content) {
  min-width: 0;
  overflow-wrap: anywhere;
}
.fishing-page {
  --fish: #17747b;
  --fish-dark: #124f55;
  --fish-soft: color-mix(in srgb, var(--fish) 10%, var(--app-surface));
  --fish-gold: #bd8424;
  --fish-ink: var(--app-text);
  --fish-muted: var(--app-muted);
  min-width: 0;
}

.fishing-page :deep(.el-button) { min-height: 44px; }
.fishing-page :deep(.el-button.is-circle) { width: 44px; height: 44px; min-width: 44px; }
.fishing-page :deep(.game-page-header__actions .el-button) { min-height: 24px; }
.fishing-page :deep(.game-page-header__actions .el-button.is-circle) { width: 24px; height: 24px; min-width: 24px; }
.fishing-page :deep(.game-page-header__actions > .el-button + .el-button) { margin-left: 0; }
.fishing-page h2 { font-size: 18px; font-weight: 700; }
.fishing-page h3 { font-size: 14px; font-weight: 700; }
.fishing-angler-control { display: grid; grid-template-columns: 26px minmax(62px, auto) auto 14px; min-width: 208px; max-width: 260px; height: 32px; min-height: 32px; align-items: center; gap: 5px; margin-block: -4px; padding: 2px 7px; border: 1px solid var(--app-border); border-radius: 7px; background: var(--app-surface); color: var(--fish-ink); text-align: left; cursor: pointer; }
.fishing-angler-control:hover,
.fishing-angler-control:focus-visible { border-color: var(--fish); outline: none; }
.fishing-angler-control:disabled { cursor: default; }
.fishing-angler-control .fishing-angler-control__avatar { width: 26px; height: 26px; font-size: 11px; }
.fishing-angler-control__identity,
.fishing-angler-control__level { display: grid; min-width: 0; gap: 1px; }
.fishing-angler-control__identity small,
.fishing-angler-control__level small { overflow: hidden; color: var(--fish-muted); font-size: 10px; line-height: 1; text-overflow: ellipsis; white-space: nowrap; }
.fishing-angler-control__identity small { display: none; }
.fishing-angler-control__identity strong { overflow: hidden; font-size: 13px; line-height: 1; text-overflow: ellipsis; white-space: nowrap; }
.fishing-angler-control__level { grid-template-columns: auto auto; align-items: baseline; gap: 3px; padding-left: 5px; border-left: 1px solid var(--app-border); }
.fishing-angler-control__level strong { color: var(--fish); font-size: 16px; line-height: 1; }
.fishing-angler-control__chevron { color: var(--fish-muted); font-size: 14px; }
:global(.fishing-angler-menu .el-dropdown-menu__item) { display: grid; grid-template-columns: 32px minmax(104px, 1fr) auto 16px; min-width: 286px; gap: 8px; align-items: center; padding-block: 7px; }
:global(.fishing-angler-menu__avatar) { display: grid; place-items: center; width: 30px; height: 30px; border-radius: 50%; background: color-mix(in srgb, #17747b 10%, var(--app-surface)); color: #17747b; font-size: 12px; font-weight: 800; }
:global(.fishing-angler-menu__copy) { display: grid; min-width: 0; gap: 1px; }
:global(.fishing-angler-menu__copy strong),
:global(.fishing-angler-menu__copy small) { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
:global(.fishing-angler-menu__copy small),
:global(.fishing-angler-menu__level) { color: var(--app-muted); font-size: 11px; }
:global(.fishing-angler-menu .el-dropdown-menu__item > svg) { color: #17747b; }
.fishing-tabs :deep(.el-tabs__header) { margin-bottom: 10px; }
.fishing-tabs :deep(.el-tabs__item) { min-height: 42px; font-size: 14px; font-weight: 650; }
.fishing-loading { display: flex; min-height: 360px; align-items: center; justify-content: center; gap: 9px; color: var(--fish-muted); }
.fishing-loading svg { animation: fish-spin 1s linear infinite; }

.fishing-tab-toolbar { display: flex; min-width: 0; min-height: 44px; align-items: center; justify-content: space-between; gap: 8px; margin-bottom: 10px; }
.fishing-tab-toolbar--end { justify-content: flex-end; }

.fishing-status-strip { display: grid; grid-template-columns: repeat(4, minmax(0, 1fr)); border: 1px solid var(--app-border); background: var(--app-surface); }
.fishing-status-strip > div { display: grid; grid-template-columns: 24px 1fr; gap: 2px 8px; align-items: center; min-height: 58px; padding: 8px 12px; border-right: 1px solid var(--app-border); }
.fishing-status-strip > div:last-child { border-right: 0; }
.fishing-status-strip svg { grid-row: 1 / 3; color: var(--fish); font-size: 20px; }
.fishing-status-strip span { color: var(--fish-muted); font-size: 12px; }
.fishing-status-strip strong { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }

.fishing-water-switch { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 7px; margin: 10px 0; }
.fishing-water-switch button { display: grid; grid-template-columns: 32px minmax(0, 1fr) 20px; align-items: center; gap: 8px; min-height: 60px; padding: 8px 10px; border: 1px solid var(--app-border); border-radius: 7px; background: var(--app-surface); color: var(--fish-ink); text-align: left; cursor: pointer; }
.fishing-water-switch button > svg:first-child { color: var(--fish); font-size: 25px; }
.fishing-water-switch button > span { display: grid; min-width: 0; gap: 2px; }
.fishing-water-switch small { color: var(--fish-muted); font-size: 12px; }
.fishing-water-switch button.is-selected { border-color: var(--fish); background: var(--fish-soft); }
.fishing-water-switch button:disabled { cursor: not-allowed; opacity: .55; }

.fishing-water-layout { display: grid; grid-template-columns: minmax(0, 1.45fr) minmax(300px, .7fr); gap: 14px; align-items: start; }
.fishing-section-head { display: flex; align-items: flex-end; justify-content: space-between; gap: 12px; margin-bottom: 12px; }
.fishing-section-head span,
.fishing-spot-detail header span,
.fishing-loadout-list article > div > span,
.fishing-tournament-hero > div > span { color: var(--fish); font-size: 12px; font-weight: 700; }
.fishing-section-head h2,
.fishing-spot-detail h2,
.fishing-loadout-list h3,
.fishing-tournament-hero h2 { margin: 2px 0 0; color: var(--fish-ink); letter-spacing: 0; }
.fishing-spot-list { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 7px; }
.fishing-spot-list > button { display: grid; grid-template-columns: 22px minmax(0, 1fr) auto; gap: 9px; align-items: center; min-height: 96px; padding: 9px 10px; border: 1px solid var(--app-border); border-radius: 7px; background: var(--app-surface); color: var(--fish-ink); text-align: left; cursor: pointer; }
.fishing-spot-list > button.is-selected { border-color: var(--fish); background: var(--fish-soft); }
.fishing-spot-marker { display: grid; place-items: center; width: 20px; height: 20px; border: 2px solid #91aaab; border-radius: 50%; }
.fishing-spot-marker i { width: 8px; height: 8px; border-radius: 50%; }
.is-selected > .fishing-spot-marker { border-color: var(--fish); }
.is-selected > .fishing-spot-marker i { background: var(--fish); }
.fishing-spot-main { display: grid; min-width: 0; gap: 3px; }
.fishing-spot-main small,
.fishing-spot-main em { overflow: hidden; color: var(--fish-muted); font-size: 12px; font-style: normal; text-overflow: ellipsis; white-space: nowrap; }
.fishing-spot-main .fishing-spot-fish { display: flex; align-items: center; gap: 4px; color: var(--fish); }
.fishing-spot-main em { display: flex; align-items: center; gap: 4px; }
.fishing-spot-forecast { display: grid; justify-items: end; gap: 7px; }
.fishing-spot-forecast b { padding: 4px 6px; border-radius: 4px; font-size: 11px; }
.fishing-spot-forecast b.is-busy { color: #23653a; background: #e7f4e9; }
.fishing-spot-forecast b.is-steady { color: #755a18; background: #f8f0d8; }
.fishing-spot-forecast b.is-quiet { color: #56696d; background: #edf1f2; }
.fishing-spot-forecast small { display: flex; align-items: center; gap: 3px; color: var(--fish-muted); font-size: 11px; }
.fishing-spot-detail { position: sticky; top: 12px; display: grid; gap: 10px; border: 1px solid var(--app-border); border-radius: 7px; padding: 12px; background: var(--app-surface); }
.fishing-spot-detail header { display: flex; align-items: center; justify-content: space-between; gap: 10px; }
.fishing-spot-detail > p { margin: 0; color: var(--fish-muted); font-size: 13px; line-height: 1.6; }
.fishing-spot-detail dl { display: grid; margin: 0; border-top: 1px solid var(--app-border); }
.fishing-spot-detail dl > div { display: flex; align-items: center; justify-content: space-between; gap: 14px; min-height: 42px; border-bottom: 1px solid var(--app-border); }
.fishing-spot-detail dt { color: var(--fish-muted); font-size: 12px; }
.fishing-spot-detail dd { display: flex; align-items: center; gap: 5px; margin: 0; text-align: right; font-weight: 700; }
.fishing-spot-species { display: grid; gap: 7px; padding-top: 2px; border-top: 1px solid var(--app-border); }
.fishing-spot-species > header { min-height: 34px; }
.fishing-spot-species > header h3 { margin: 0; }
.fishing-spot-species__list { display: grid; gap: 1px; background: var(--app-border); }
.fishing-spot-species__list > div { display: grid; grid-template-columns: 30px minmax(0, 1fr) auto; min-width: 0; align-items: center; gap: 8px; min-height: 40px; padding: 5px 7px; background: var(--app-surface); }
.fishing-spot-species__list img { width: 28px; height: 28px; object-fit: contain; }
.fishing-spot-species__list span { display: grid; min-width: 0; gap: 1px; }
.fishing-spot-species__list strong { overflow: hidden; font-size: 12px; text-overflow: ellipsis; white-space: nowrap; }
.fishing-spot-species__list small { color: var(--fish-muted); font-size: 11px; }
.fishing-spot-species__list b { color: var(--fish); font-size: 11px; white-space: nowrap; }
.fishing-spot-species > p { margin: 0; color: var(--fish-muted); font-size: 11px; line-height: 1.45; }
.fishing-main-action { width: 100%; }

.fishing-live { display: grid; grid-template-columns: minmax(0, 1fr) minmax(340px, 380px); align-items: start; gap: 16px; }
.fishing-live-water { position: relative; isolation: isolate; display: flex; height: 560px; min-height: 560px; align-self: start; overflow: hidden; border: 1px solid var(--app-border); border-radius: 8px; flex-direction: column; justify-content: space-between; background: #609895; color: #f7fbf7; }
.fishing-water-traces { position: absolute; z-index: 1; left: -2%; top: 31%; width: 104%; height: 69%; overflow: visible; pointer-events: none; animation: fish-water-trace 9s ease-in-out infinite alternate; }
.fishing-water-traces path { fill: none; stroke: rgba(226, 241, 235, .16); stroke-width: .7; vector-effect: non-scaling-stroke; }
.fishing-water-traces path:nth-child(2) { stroke-opacity: .68; }
.fishing-water-traces path:nth-child(3) { stroke-opacity: .5; }
.fishing-water-traces path:nth-child(4) { stroke-opacity: .38; }
.fishing-live-water::before { position: absolute; z-index: 1; inset: 0; background: linear-gradient(180deg, rgb(15 38 43 / 28%), transparent 24%, transparent 62%, rgb(12 35 39 / 78%)); content: ""; pointer-events: none; }
.fishing-live-water.weather-storm .fishing-water-traces { animation-duration: 3s; }
.fishing-live-water.weather-rain .fishing-water-traces { animation-duration: 5s; }
.fishing-live-water__head { position: relative; z-index: 5; display: grid; grid-template-columns: 1fr auto 1fr; align-items: center; gap: 10px; margin: 16px; color: #fff; font-size: 12px; text-shadow: 0 1px 3px rgb(12 35 39 / 90%); }
.fishing-live-water__head > span { min-width: 0; }
.fishing-live-water__time { justify-content: flex-end; flex-wrap: wrap; font-variant-numeric: tabular-nums; }
.fishing-live-water__head svg { flex: none; }
.fishing-live-water__head span,
.fishing-live-water__end { display: flex; align-items: center; gap: 6px; }
.fishing-live-water__head strong { min-width: 0; text-align: center; overflow-wrap: anywhere; font-size: 13px; }
.fishing-live-water__end { position: relative; z-index: 5; align-self: flex-start; max-width: calc(100% - 28px); min-height: 30px; margin: auto 14px 14px; padding: 4px 0; color: #edf6f1; font-size: 11px; text-shadow: 0 1px 3px rgb(12 35 39 / 70%); }
.fishing-live-water__end svg { flex: none; }
.fishing-live-water.period-night .fishing-live-bobber { filter: drop-shadow(0 0 3px #fff4cc) drop-shadow(0 2px 3px #17272c); }
.fishing-cast-line { position: absolute; z-index: 2; inset: 0; width: 100%; height: 100%; overflow: visible; pointer-events: none; }
.fishing-cast-line path { fill: none; stroke: rgba(238, 244, 239, .36); stroke-width: .6; vector-effect: non-scaling-stroke; }
.fishing-live-bobber { position: absolute; z-index: 3; left: calc(61% - 16px); top: 47%; width: 32px; height: 32px; object-fit: contain; filter: drop-shadow(0 2px 2px rgba(25, 64, 67, .34)); transform: rotate(8deg); animation: fish-bobber 3.8s ease-in-out infinite; }
.fishing-live-water.phase-bite .fishing-live-bobber { animation: fish-bobber-bite .6s ease-in-out infinite alternate; }
.fishing-live-water.phase-fighting .fishing-live-bobber { transform: rotate(23deg); animation: fish-bobber-fight 1s ease-in-out infinite alternate; }
.fishing-live-water__phase { position: absolute; z-index: 4; left: 22px; bottom: 67px; display: flex; width: min(74%, 520px); min-width: 0; align-items: center; gap: 11px; color: #f6faf7; text-align: left; text-shadow: 0 2px 8px rgba(9, 40, 43, .78); }
.fishing-live-water__phase-copy { display: grid; min-width: 0; justify-items: start; gap: 3px; }
.fishing-live-water__phase-label { display: flex; align-items: center; gap: 5px; color: rgba(235, 244, 239, .9); font-size: 10px; font-weight: 700; }
.fishing-live-water__phase-label svg { color: #e7c46d; font-size: 14px; }
.fishing-live-water__phase strong { max-width: 100%; font-size: 21px; line-height: 1.35; overflow-wrap: anywhere; }
.fishing-live-water__phase-copy > span:last-child { max-width: 100%; color: rgba(239, 246, 242, .92); font-size: 12px; line-height: 1.45; overflow-wrap: anywhere; }
.fishing-live-water.event-landed .fishing-live-water__phase-label svg { color: #9bd4ad; }
.fishing-live-water.event-escaped .fishing-live-water__phase-label svg,
.fishing-live-water.event-line_break .fishing-live-water__phase-label svg { color: #eeaaa1; }
.fishing-ripples { position: absolute; z-index: 2; inset: 0; pointer-events: none; }
.fishing-ripples i { position: absolute; left: 61%; top: 51%; width: 88px; height: 19px; border: 1px solid rgba(224, 239, 233, .62); border-radius: 50%; transform: translate(-50%, -50%); animation: fish-ripple 3.6s ease-out infinite; }
.fishing-ripples i:nth-child(2) { animation-delay: .7s; }
.fishing-ripples i:nth-child(3) { animation-delay: 1.4s; }
.fishing-ripples i:nth-child(4) { animation-delay: 2.1s; }
.fishing-live-panel { display: flex; min-width: 0; flex-direction: column; gap: 12px; padding: 2px 0; color: var(--fish-ink); }
.fishing-live-panel > header { display: flex; align-items: center; justify-content: space-between; gap: 10px; }
.fishing-live-panel > header span { color: var(--fish); font-size: 12px; }
.fishing-live-panel h2 { margin: 3px 0 0; font-size: 18px; }
.fishing-live-signals { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); margin: 0; border-block: 1px solid var(--app-border); }
.fishing-live-signals > div { display: grid; min-width: 0; align-content: center; gap: 6px; min-height: 66px; padding: 8px; border-right: 1px solid var(--app-border); }
.fishing-live-signals > div:last-child { border-right: 0; }
.fishing-live-signals dt { display: flex; align-items: center; gap: 7px; color: var(--fish-muted); font-size: 12px; }
.fishing-live-signals dt svg { color: var(--fish); }
.fishing-live-signals dd { min-width: 0; overflow: hidden; margin: 0; font-size: 13px; font-weight: 700; text-overflow: ellipsis; white-space: nowrap; }
.fishing-live-signals dd.line-critical { color: var(--el-color-danger); }
.fishing-live-signals dd.line-tight { color: var(--fish-gold); }
.fishing-stamina { display: grid; grid-template-columns: auto minmax(32px, 1fr); align-items: center; gap: 6px; font-variant-numeric: tabular-nums; }
.fishing-stamina :deep(.el-progress) { min-width: 0; }
.fishing-stamina__empty { grid-column: 1 / -1; color: var(--fish-muted); font-weight: 500; }
.fishing-live-log { display: grid; min-height: 0; gap: 7px; }
.fishing-live-log > header { display: flex; align-items: center; justify-content: space-between; gap: 8px; }
.fishing-live-log h3 { margin: 0; font-size: 13px; }
.fishing-live-log > header span { display: flex; align-items: center; gap: 6px; color: var(--fish-muted); font-size: 11px; }
.fishing-live-log > header svg { flex: 0 0 auto; font-size: 13px; }
.fishing-live-log ol { max-height: 228px; overflow-y: auto; margin: 0; padding: 0; border-block: 1px solid var(--app-border); list-style: none; overscroll-behavior: contain; scrollbar-width: thin; }
.fishing-live-log li { --event-color: #718b89; display: grid; grid-template-columns: 32px minmax(0, 1fr) auto; min-width: 0; align-items: center; gap: 8px; min-height: 56px; padding: 7px 2px; }
.fishing-live-log li + li { border-top: 1px solid color-mix(in srgb, var(--app-border) 72%, transparent); }
.fishing-live-log li.is-active { --event-color: #b37b1f; }
.fishing-live-log li.is-success { --event-color: #2f7d59; }
.fishing-live-log li.is-danger { --event-color: #b7574e; }
.fishing-live-log__marker { display: grid; place-items: center; width: 30px; height: 30px; border: 1px solid color-mix(in srgb, var(--event-color) 35%, var(--app-border)); border-radius: 50%; background: color-mix(in srgb, var(--event-color) 9%, var(--app-surface)); color: var(--event-color); }
.fishing-live-log__marker svg { font-size: 15px; }
.fishing-live-log__marker .fish-mutation-art { width: 27px; height: 27px; }
.fishing-live-log__copy { display: grid; min-width: 0; gap: 2px; }
.fishing-live-log__copy strong { overflow: hidden; color: var(--fish-ink); font-size: 12px; text-overflow: ellipsis; white-space: nowrap; }
.fishing-live-log__copy small { overflow: hidden; color: var(--fish-muted); font-size: 10px; line-height: 1.35; text-overflow: ellipsis; white-space: nowrap; }
.fishing-live-log time { align-self: start; padding-top: 2px; color: var(--fish-muted); font-size: 10px; font-variant-numeric: tabular-nums; }
.fishing-live-log > p { margin: 0; padding: 12px 0; border-block: 1px solid var(--app-border); color: var(--fish-muted); font-size: 11px; line-height: 1.5; }
.fishing-live-supplies h3 { margin: 0 0 8px; font-size: 13px; }
.fishing-live-supply-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(58px, 1fr)); justify-items: center; gap: 10px 6px; padding: 3px 0 2px; }
.fishing-live-supply-grid :deep(.item-card-name) { width: 100%; overflow: hidden; text-overflow: ellipsis; }
.fishing-trip-details { border-top: 1px solid var(--app-border); padding-top: 10px; }
.fishing-trip-details summary { min-height: 44px; cursor: pointer; color: var(--fish); font-weight: 700; }
.fishing-summary-grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 6px; }
.fishing-summary-grid span { display: grid; gap: 2px; padding: 8px; background: var(--fish-soft); color: var(--fish-muted); font-size: 11px; }
.fishing-summary-grid b { color: var(--fish-ink); font-size: 15px; }
.fishing-stop { margin-top: auto; }

.fishing-loadout-list { display: grid; gap: 7px; }
.fishing-loadout-list article { display: grid; grid-template-columns: 52px minmax(0, 1fr) auto auto; align-items: center; gap: 11px; min-height: 76px; padding: 9px 11px; border: 1px solid var(--app-border); border-radius: 7px; }
.fishing-loadout-list article > img { width: 46px; height: 46px; object-fit: contain; }
.fishing-loadout-list p { margin: 4px 0 0; color: var(--fish-muted); font-size: 12px; }
.fishing-loadout-list dl { display: flex; gap: 20px; margin: 0; }
.fishing-loadout-list dl > div { display: grid; gap: 2px; }
.fishing-loadout-list dt { color: var(--fish-muted); font-size: 11px; }
.fishing-loadout-list dd { margin: 0; font-weight: 700; }
.fishing-row-actions,
.fishing-heading-actions { display: flex; gap: 6px; }

.fishing-catch-toolbar { border-bottom: 1px solid var(--app-border); padding-bottom: 6px; }
.fishing-subnav { min-width: 0; width: min(100%, 360px); }
.fishing-catch-item { position: relative; display: grid; width: fit-content; min-width: 0; justify-items: center; gap: 3px; margin-inline: auto; padding: 3px; border-radius: 8px; cursor: pointer; outline: none; }
.fishing-catch-item:focus-visible :deep(.item-card),
.fishing-catch-item.is-selected :deep(.item-card) { border-color: var(--fish); box-shadow: 0 0 0 2px color-mix(in srgb, var(--fish) 22%, transparent); }
.fishing-catch-item.is-batch:not(.is-unavailable) :deep(.item-card) { border-style: dashed; }
.fishing-catch-item.is-unavailable { cursor: not-allowed; }
.fishing-catch-item.is-unavailable :deep(.item-card) { opacity: .42; filter: grayscale(.35); }
.fishing-catch-record,
.fishing-catch-check,
.fishing-catch-immature { position: absolute; z-index: 3; top: -4px; display: grid; place-items: center; width: 19px; height: 19px; border: 2px solid var(--app-surface); border-radius: 5px; color: #fff; font-size: 11px; pointer-events: none; }
.fishing-catch-record { left: -3px; background: #c18a2f; }
.fishing-catch-check { right: -3px; background: var(--el-color-primary); }
.fishing-catch-immature { right: -3px; background: #8a6a32; }
.fishing-catch-mutations { position: absolute; z-index: 4; top: -4px; right: -3px; display: flex; width: 34px; flex-wrap: wrap; justify-content: flex-end; gap: 2px; pointer-events: none; }
.fishing-catch-mutations > span { display: grid; place-items: center; width: 16px; height: 16px; border: 2px solid var(--app-surface); border-radius: 5px; background: #52616c; color: #fff; font-size: 9px; box-shadow: 0 1px 3px rgb(20 33 41 / 24%); }
.fishing-catch-mutations > span.is-prismatic { background: #16879a; }
.fishing-catch-mutations > span.is-rainbow { background: linear-gradient(135deg, #db4e72 0 25%, #d89b27 25% 50%, #279b79 50% 75%, #536fd0 75%); }
.fishing-catch-mutations > span.is-shimmering { background: #b7821f; }
.fishing-catch-item.is-batch .fishing-catch-mutations { top: 17px; }
.fishing-journal-grid { display: grid; grid-template-columns: repeat(4, minmax(0, 1fr)); gap: 7px; }
.fishing-journal-grid article { display: grid; grid-template-columns: 76px minmax(0, 1fr); align-items: center; gap: 9px; min-height: 104px; padding: 8px; border: 1px solid var(--app-border); border-radius: 7px; }
.fishing-journal-grid img { width: 76px; height: 68px; object-fit: contain; }
.fishing-journal-grid span { color: var(--fish); font-size: 11px; font-weight: 700; }
.fishing-journal-grid h3 { margin: 2px 0; font-size: 14px; }
.fishing-journal-grid p { margin: 0; color: var(--fish-muted); font-size: 11px; }
.fishing-species-needs { display: flex; flex-wrap: wrap; gap: 3px 7px; margin-top: 6px; }
.fishing-species-needs span { display: inline-flex; align-items: center; gap: 3px; color: var(--fish-ink); font-size: 10px; font-weight: 600; }
.fishing-species-needs svg { color: var(--fish); font-size: 12px; }
.fishing-species-ecology { display: grid; gap: 3px; margin-top: 7px; padding-top: 6px; border-top: 1px solid var(--app-border); }
.fishing-species-ecology span { display: flex; min-width: 0; align-items: flex-start; gap: 4px; color: var(--fish-muted); font-size: 10px; font-weight: 650; line-height: 1.35; }
.fishing-species-ecology span.is-predator { color: var(--el-color-danger); }
.fishing-species-ecology svg { flex: none; margin-top: 1px; color: var(--fish); font-size: 12px; }
.fishing-species-ecology span.is-predator svg { color: inherit; }
.fishing-journal-grid article.is-unknown img { filter: grayscale(1) brightness(.25); opacity: .3; }
.fishing-record-controls { display: flex; gap: 9px; margin-bottom: 12px; }
.fishing-record-controls .el-select { width: min(360px, 100%); }
.fishing-personal-record,
.fishing-my-entries { display: flex; align-items: center; gap: 24px; margin-bottom: 12px; padding: 10px 0; border-block: 1px solid var(--app-border); }
.fishing-personal-record > span,
.fishing-my-entries > span { color: var(--fish); font-weight: 800; }
.fishing-personal-record > div,
.fishing-my-entries > div { display: grid; gap: 2px; }
.fishing-personal-record small,
.fishing-my-entries small { color: var(--fish-muted); }
.fishing-record-board { display: grid; border-top: 1px solid var(--app-border); }
.fishing-record-board > div { display: grid; grid-template-columns: 48px minmax(0, 1fr) auto; align-items: center; gap: 10px; min-height: 58px; border-bottom: 1px solid var(--app-border); }
.fishing-record-board > div > span { display: grid; place-items: center; width: 32px; height: 32px; border-radius: 50%; background: var(--fish-soft); color: var(--fish); font-weight: 800; }
.fishing-record-board b { display: grid; }
.fishing-record-board small { color: var(--fish-muted); font-weight: 400; }

.fishing-history { max-width: 1080px; margin-inline: auto; }
.fishing-history__head { align-items: center; }
.fishing-history__head > div > span { color: var(--fish); font-size: 12px; font-weight: 700; }
.fishing-history__head h2 { margin: 2px 0 0; color: var(--fish-ink); font-size: 20px; letter-spacing: 0; }
.fishing-history__count { padding: 5px 8px; border: 1px solid var(--app-border); border-radius: 5px; color: var(--fish-muted); font-size: 11px; font-variant-numeric: tabular-nums; }
.fishing-history .fishing-loading { min-height: 240px; }
.fishing-history-list { display: grid; gap: 8px; margin: 0; padding: 0; list-style: none; }
.fishing-history-trip { overflow: hidden; border: 1px solid var(--app-border); border-radius: 7px; background: var(--app-surface); }
.fishing-history-trip > summary { display: grid; grid-template-columns: 38px minmax(0, 1fr) auto 18px; min-width: 0; align-items: center; gap: 11px; min-height: 82px; padding: 10px 12px; cursor: pointer; list-style: none; }
.fishing-history-trip > summary::-webkit-details-marker { display: none; }
.fishing-history-trip > summary:focus-visible { outline: 2px solid var(--el-color-primary-light-5); outline-offset: -3px; }
.fishing-history-trip__status { display: grid; place-items: center; width: 36px; height: 36px; border: 1px solid color-mix(in srgb, var(--fish) 32%, var(--app-border)); border-radius: 50%; background: var(--fish-soft); color: var(--fish); font-size: 18px; }
.fishing-history-trip__status.is-stopped { border-color: color-mix(in srgb, var(--fish-gold) 38%, var(--app-border)); background: color-mix(in srgb, var(--fish-gold) 9%, var(--app-surface)); color: var(--fish-gold); }
.fishing-history-trip__identity { display: grid; min-width: 0; gap: 2px; }
.fishing-history-trip__identity strong { overflow: hidden; color: var(--fish-ink); font-size: 15px; text-overflow: ellipsis; white-space: nowrap; }
.fishing-history-trip__identity small { overflow: hidden; color: var(--fish-muted); font-size: 11px; text-overflow: ellipsis; white-space: nowrap; }
.fishing-history-trip__identity time { color: var(--fish-muted); font-size: 10px; font-variant-numeric: tabular-nums; }
.fishing-history-trip__result { display: grid; min-width: 88px; justify-items: end; gap: 2px; text-align: right; }
.fishing-history-trip__result strong { color: var(--fish-ink); font-size: 13px; font-variant-numeric: tabular-nums; }
.fishing-history-trip__result small { color: var(--fish); font-size: 11px; font-variant-numeric: tabular-nums; }
.fishing-history-trip__chevron { color: var(--fish-muted); transition: transform .2s ease; }
.fishing-history-trip[open] .fishing-history-trip__chevron { transform: rotate(180deg); }
.fishing-history-trip__body { display: grid; gap: 12px; padding: 12px; border-top: 1px solid var(--app-border); background: color-mix(in srgb, var(--fish) 2%, var(--app-surface)); }
.fishing-history-trip__meta { display: flex; min-width: 0; flex-wrap: wrap; gap: 8px 18px; color: var(--fish-muted); font-size: 11px; }
.fishing-history-trip__meta span { display: inline-flex; min-width: 0; align-items: center; gap: 5px; }
.fishing-history-trip__meta svg { flex: none; color: var(--fish); font-size: 14px; }
.fishing-history-stats { display: grid; grid-template-columns: repeat(6, minmax(0, 1fr)); margin: 0; border-block: 1px solid var(--app-border); }
.fishing-history-stats > div { display: grid; min-width: 0; justify-items: center; gap: 3px; padding: 9px 5px; }
.fishing-history-stats > div + div { border-left: 1px solid var(--app-border); }
.fishing-history-stats dt { color: var(--fish-muted); font-size: 10px; }
.fishing-history-stats dd { margin: 0; color: var(--fish-ink); font-size: 15px; font-variant-numeric: tabular-nums; font-weight: 750; }
.fishing-history-events { display: grid; gap: 6px; }
.fishing-history-events > header { display: flex; align-items: center; justify-content: space-between; gap: 8px; }
.fishing-history-events > header strong { color: var(--fish-ink); font-size: 12px; }
.fishing-history-events > header span { color: var(--fish-muted); font-size: 10px; }
.fishing-history-events > header span.is-partial { color: var(--fish-gold); }
.fishing-history-events ol { max-height: 320px; overflow-y: auto; margin: 0; padding: 0; border-top: 1px solid var(--app-border); list-style: none; scrollbar-width: thin; }
.fishing-history-events li { --event-color: #718b89; display: grid; grid-template-columns: 30px minmax(0, 1fr) auto; min-width: 0; align-items: center; gap: 8px; min-height: 48px; padding: 6px 2px; }
.fishing-history-events li + li { border-top: 1px solid color-mix(in srgb, var(--app-border) 72%, transparent); }
.fishing-history-events li.is-active { --event-color: #b37b1f; }
.fishing-history-events li.is-success { --event-color: #2f7d59; }
.fishing-history-events li.is-danger { --event-color: #b7574e; }
.fishing-history-events__marker { display: grid; place-items: center; width: 28px; height: 28px; border: 1px solid color-mix(in srgb, var(--event-color) 35%, var(--app-border)); border-radius: 50%; background: color-mix(in srgb, var(--event-color) 8%, var(--app-surface)); color: var(--event-color); }
.fishing-history-events__marker svg { font-size: 14px; }
.fishing-history-events__marker .fish-mutation-art { width: 25px; height: 25px; }
.fishing-history-events li > span:nth-child(2) { display: grid; min-width: 0; gap: 2px; }
.fishing-history-events li > span:nth-child(2) strong,
.fishing-history-events li > span:nth-child(2) small { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.fishing-history-events li > span:nth-child(2) strong { color: var(--fish-ink); font-size: 11px; }
.fishing-history-events li > span:nth-child(2) small { color: var(--fish-muted); font-size: 10px; }
.fishing-history-events li > time { align-self: start; padding-top: 2px; color: var(--fish-muted); font-size: 10px; font-variant-numeric: tabular-nums; }
.fishing-history-events > p { margin: 0; padding: 14px 0; border-block: 1px solid var(--app-border); color: var(--fish-muted); font-size: 11px; }
.fishing-history-events__loading,
.fishing-history-events__error { display: flex; min-height: 72px; align-items: center; justify-content: center; gap: 7px; border-block: 1px solid var(--app-border); color: var(--fish-muted); font-size: 11px; }
.fishing-history-events__loading svg { color: var(--fish); font-size: 16px; animation: fish-spin 1s linear infinite; }
.fishing-history-events__error { color: var(--fish-negative); }
.fishing-history-events__error :deep(.el-button) { min-height: 32px; padding-inline: 5px; }
.fishing-history-events > footer { display: flex; min-height: 42px; flex-wrap: wrap; align-items: center; justify-content: center; gap: 0 8px; border-top: 1px solid var(--app-border); }
.fishing-history-events > footer > span { color: var(--fish-negative); font-size: 10px; }
.fishing-history-events > footer :deep(.el-button) { min-height: 40px; color: var(--fish); }
.fishing-history-events > footer svg { margin-right: 5px; }

.fishing-tournament-hero { display: grid; grid-template-columns: 86px minmax(0, 1fr) auto; align-items: center; gap: 12px; min-height: 104px; padding: 10px 12px; border: 1px solid var(--app-border); }
.fishing-target-fish { display: grid; place-items: center; width: 80px; height: 76px; background: var(--fish-soft); }
.fishing-target-fish img { width: 90%; height: 90%; object-fit: contain; }
.fishing-tournament-hero h2 { font-size: 20px; }
.fishing-tournament-hero p { margin: 3px 0 0; color: var(--fish-muted); font-size: 12px; }
.fishing-token { display: grid; grid-template-columns: 24px 1fr; gap: 1px 6px; align-items: center; padding: 7px 9px; border-left: 1px solid var(--app-border); }
.fishing-token svg { grid-row: 1 / 3; color: var(--fish-gold); font-size: 21px; }
.fishing-token span { color: var(--fish-muted); font-size: 11px; }
.fishing-token strong { font-size: 17px; }
.fishing-tournament-board { margin-top: 8px; }
.fishing-tournament-toolbar { display: flex; min-height: 46px; align-items: center; justify-content: space-between; gap: 10px; border-bottom: 1px solid var(--app-border); }
.fishing-tournament-toolbar > strong { font-size: 14px; }
.fishing-standard-rules { padding: 10px 11px 9px; border-left: 3px solid var(--fish); background: var(--fish-soft); }
.fishing-standard-rules > header { display: flex; align-items: center; justify-content: space-between; gap: 10px; }
.fishing-standard-rules > header > span,
.fishing-standard-rules > header > small { display: inline-flex; align-items: center; gap: 6px; }
.fishing-standard-rules > header svg { flex: none; color: var(--fish); font-size: 15px; }
.fishing-standard-rules > header strong { color: var(--fish-ink); font-size: 13px; }
.fishing-standard-rules > header small { color: var(--fish); font-size: 11px; }
.fishing-standard-rules dl { display: grid; grid-template-columns: repeat(6, minmax(0, 1fr)); margin: 9px 0 0; border-block: 1px solid var(--app-border); }
.fishing-standard-rules dl > div { min-width: 0; padding: 7px 9px; }
.fishing-standard-rules dl > div + div { border-left: 1px solid var(--app-border); }
.fishing-standard-rules dt { color: var(--fish-muted); font-size: 10px; line-height: 1.3; }
.fishing-standard-rules dd { margin: 2px 0 0; color: var(--fish-ink); font-size: 13px; font-variant-numeric: tabular-nums; font-weight: 700; line-height: 1.3; overflow-wrap: anywhere; }
.fishing-standard-rules > p { margin: 7px 0 0; color: var(--fish-muted); font-size: 11px; line-height: 1.45; }
.fishing-standard-rules.is-designated-loadout { background: color-mix(in srgb, var(--fish) 5%, var(--app-surface)); }
.fishing-standard-method { display: flex; align-items: center; gap: 6px; }
.fishing-standard-method svg { flex: none; color: var(--fish); font-size: 14px; }
.fishing-standard-tackle { display: grid; grid-template-columns: repeat(4, minmax(0, 1fr)); margin-top: 9px; border: 1px solid var(--app-border); }
.fishing-standard-tackle > div { display: grid; min-width: 0; align-content: start; gap: 5px; padding: 8px 9px; border-right: 1px solid var(--app-border); border-bottom: 1px solid var(--app-border); background: var(--app-surface); }
.fishing-standard-tackle > div:nth-child(4n) { border-right: 0; }
.fishing-standard-tackle > div:nth-last-child(-n + 3) { border-bottom: 0; }
.fishing-standard-tackle > div > span { color: var(--fish-muted); font-size: 10px; }
.fishing-standard-tackle :deep(.el-tag) { max-width: 100%; min-height: 30px; justify-content: flex-start; }
.fishing-standard-tackle :deep(.item-quick-ref) { min-width: 0; }
.fishing-standard-tackle :deep(.item-quick-ref-name) { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.fishing-standard-tackle > div > small { display: inline-flex; align-items: center; gap: 4px; font-size: 10px; }
.fishing-standard-tackle > div > small.is-owned { color: #2f7d59; }
.fishing-standard-tackle > div > small.is-missing { color: #a96c19; }
.fishing-standard-status { display: flex; min-height: 38px; align-items: center; justify-content: space-between; gap: 10px; margin-top: 8px; padding: 4px 0 4px 8px; border-left: 2px solid #a96c19; color: #8b5d19; }
.fishing-standard-status.is-ready { border-left-color: #2f7d59; color: #2f7d59; }
.fishing-standard-status > span { display: inline-flex; min-width: 0; align-items: center; gap: 6px; }
.fishing-standard-status > span > svg { flex: none; }
.fishing-standard-status strong { overflow-wrap: anywhere; font-size: 11px; }
.fishing-standard-status :deep(.el-button) { min-height: 32px; padding-inline: 6px; color: var(--fish); }
.fishing-reward,
.fishing-catch-up { display: flex; align-items: center; justify-content: space-between; gap: 12px; margin-top: 12px; padding: 12px 14px; border: 1px solid var(--app-border); }
.fishing-reward > span { display: flex; align-items: center; gap: 8px; }
.fishing-catch-up p { margin: 3px 0; color: var(--fish-muted); font-size: 12px; }

.fishing-trip-planner { display: grid; gap: 20px; }
.fishing-planner-step { display: grid; gap: 7px; }
.fishing-planner-step h2 { margin: 0 0 7px; font-size: 20px; }
.fishing-planner-step > button { display: flex; align-items: center; justify-content: space-between; gap: 10px; min-height: 64px; padding: 10px 12px; border: 1px solid var(--app-border); border-radius: 7px; background: var(--app-surface); color: var(--fish-ink); text-align: left; }
.fishing-planner-step > button.is-selected,
.fishing-duration-options button.is-selected { border-color: var(--fish); background: var(--fish-soft); }
.fishing-planner-step > button > span { display: grid; flex: 1; gap: 2px; min-width: 0; overflow-wrap: anywhere; }
.fishing-method-notice { display: flex; align-items: flex-start; gap: 8px; padding: 10px 12px; border-left: 3px solid var(--el-color-info); background: var(--el-fill-color-light); color: var(--el-text-color-primary); font-size: 12px; line-height: 1.5; }
.fishing-method-notice > svg { flex: none; margin-top: 2px; font-size: 16px; }
.fishing-method-notice > div { min-width: 0; overflow-wrap: anywhere; }
.fishing-method-notice p { margin: 3px 0 0; }
.fishing-method-notice.is-mismatch { border-color: var(--el-color-warning); background: var(--el-color-warning-light-9); color: var(--el-text-color-primary); }
.fishing-method-notice.is-mismatch > svg { color: var(--el-color-warning); }
.fishing-planner-step .fishing-loadout-method-issue { display: flex; align-items: flex-start; gap: 4px; color: var(--el-color-danger); }
.fishing-loadout-method-issue svg { flex: none; margin-top: 2px; }
.fishing-loadout-recovery { display: flex; flex-wrap: wrap; align-items: center; justify-content: space-between; gap: 8px; padding-top: 8px; }
.fishing-loadout-recovery p { flex: 1 1 180px; margin: 0; font-size: 12px; color: var(--el-text-color-secondary); }
.fishing-loadout-recovery .el-button { min-height: 44px; }
.fishing-planner-step small { color: var(--fish-muted); }
.fishing-planner-step .fishing-planner-fish { display: flex; align-items: center; gap: 4px; color: var(--fish); line-height: 1.4; }
.fishing-planner-step > button > b { color: var(--fish); }
.fishing-planner-step > button > svg { color: var(--fish); font-size: 23px; }
.fishing-standard-eligible { display: inline-flex; flex: none; align-items: center; gap: 4px; font-size: 11px; }
.fishing-standard-eligible svg { font-size: 15px; }
.fishing-tournament-choice { display: grid; gap: 8px; margin-top: 9px; padding-top: 12px; border-top: 1px solid var(--app-border); }
.fishing-tournament-choice > header { display: flex; align-items: baseline; justify-content: space-between; gap: 10px; }
.fishing-tournament-choice > header strong { font-size: 14px; }
.fishing-tournament-choice > header small { text-align: right; }
.fishing-tournament-choice > div { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 7px; }
.fishing-tournament-choice > div > button { display: grid; grid-template-columns: 24px minmax(0, 1fr) 18px; min-width: 0; min-height: 68px; align-items: center; gap: 8px; padding: 9px 10px; border: 1px solid var(--app-border); border-radius: 7px; background: var(--app-surface); color: var(--fish-ink); text-align: left; cursor: pointer; }
.fishing-tournament-choice > div > button.is-selected { border-color: var(--fish); background: var(--fish-soft); }
.fishing-tournament-choice > div > button:disabled { cursor: not-allowed; opacity: .52; }
.fishing-tournament-choice > div > button > svg { color: var(--fish); font-size: 20px; }
.fishing-tournament-choice > div > button > svg:last-child { font-size: 16px; }
.fishing-tournament-choice > div > button > span { display: grid; min-width: 0; gap: 2px; }
.fishing-tournament-choice > div > button strong { font-size: 13px; }
.fishing-tournament-choice > div > button small { overflow-wrap: anywhere; line-height: 1.35; }
.fishing-tournament-choice > p { display: flex; align-items: flex-start; gap: 5px; margin: 0; font-size: 11px; line-height: 1.45; }
.fishing-tournament-choice > p svg { flex: none; margin-top: 2px; }
.fishing-tournament-choice > p.is-ready { color: #2f7d59; }
.fishing-tournament-choice > p.is-mismatch { color: #a96c19; }
.fishing-angler-avatar { display: grid; place-items: center; width: 40px; height: 40px; border-radius: 50%; background: var(--fish-soft); color: var(--fish); font-weight: 800; }
.fishing-angler-hint { display: flex; align-items: flex-start; gap: 6px; margin: 2px 0 8px; color: var(--fish-muted); font-size: 12px; line-height: 1.5; }
.fishing-angler-hint svg { flex: 0 0 auto; margin-top: 2px; color: var(--fish); }
.fishing-planner-empty { padding: 16px; border: 1px dashed var(--app-border); text-align: center; }
.fishing-duration-options { display: grid; grid-template-columns: repeat(5, minmax(0, 1fr)); gap: 5px; }
.fishing-duration-options button { min-height: 44px; border: 1px solid var(--app-border); border-radius: 6px; background: var(--app-surface); color: var(--fish-ink); }
.fishing-duration-step > p { margin: 3px 0; color: var(--fish-muted); font-size: 12px; }
.fishing-supply-pack { margin-top: 6px; border-top: 1px solid var(--app-border); padding-top: 12px; }
.fishing-supply-pack h3 { margin: 0; font-size: 14px; font-weight: 600; }
.fishing-supply-pack header p { margin: 3px 0 10px; color: var(--fish-muted); font-size: 12px; line-height: 1.45; }
.fishing-supply-grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 6px; }
.fishing-supply-row { display: grid; grid-template-columns: minmax(0, 1fr) 108px; align-items: center; gap: 10px; min-height: 62px; padding: 8px 9px; border: 1px solid var(--app-border); border-radius: 6px; }
.fishing-supply-ref { display: grid; min-width: 0; justify-items: start; gap: 3px; }
.fishing-supply-ref :deep(.el-tag) { max-width: 100%; min-height: 32px; }
.fishing-supply-ref :deep(.item-quick-ref) { min-width: 0; max-width: 100%; }
.fishing-supply-ref :deep(.item-quick-ref-name) { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.fishing-supply-ref small { color: var(--fish-muted); font-size: 10px; line-height: 1; }
.fishing-supply-pack .el-input-number { width: 108px; }
.fishing-more-settings { margin-top: 10px; border-block: 1px solid var(--app-border); }
.fishing-more-settings summary { display: flex; min-height: 52px; align-items: center; justify-content: space-between; gap: 12px; cursor: pointer; color: var(--fish-ink); list-style: none; }
.fishing-more-settings summary > span { display: grid; min-width: 0; gap: 2px; }
.fishing-more-settings summary strong { font-size: 13px; font-weight: 700; }
.fishing-more-settings summary small { overflow: hidden; color: var(--fish-muted); font-size: 11px; font-weight: 500; text-overflow: ellipsis; white-space: nowrap; }
.fishing-more-settings summary::-webkit-details-marker { display: none; }
.fishing-more-settings summary > svg { flex: none; color: var(--fish-muted); font-size: 17px; transition: transform .16s ease; }
.fishing-more-settings[open] summary svg { transform: rotate(180deg); }
.fishing-more-settings__body { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 0 16px; padding: 0 0 14px; }
.fishing-setting { display: grid; min-width: 0; gap: 8px; }
.fishing-setting--wide { grid-column: 1 / -1; padding: 12px 0; border-bottom: 1px solid var(--app-border); }
.fishing-setting__head { display: flex; min-height: 20px; align-items: center; justify-content: space-between; gap: 12px; color: var(--fish-ink); }
.fishing-setting__head > span { display: inline-flex; flex: none; align-items: center; gap: 6px; }
.fishing-setting__head svg { color: var(--fish); font-size: 16px; }
.fishing-setting__head strong { font-size: 13px; font-weight: 600; }
.fishing-setting__head small { color: var(--fish-muted); font-size: 11px; font-weight: 500; line-height: 1.35; text-align: right; }
.fishing-setting :deep(.el-segmented) { width: 100%; }
.fishing-setting :deep(.el-segmented__item) { min-width: 0; min-height: 46px; }
.fishing-setting :deep(.el-segmented__item-label) { overflow: visible; padding: 4px 6px; line-height: 1.2; white-space: normal; }
.fishing-setting > .el-select,
.fishing-setting > .el-input-number { width: 100%; }
.fishing-field { display: grid; gap: 7px; margin-top: 10px; color: var(--fish-muted); font-size: 12px; }
.fishing-field > small { min-height: 17px; color: var(--fish-muted); font-size: 11px; line-height: 1.45; }
.fishing-field :deep(.el-segmented__item) { min-height: 44px; }
.fishing-supply-issue { display: flex; align-items: center; gap: 6px; color: var(--el-color-warning) !important; }
.fishing-dialog-footer { display: flex; align-items: center; gap: 8px; }
.fishing-dialog-footer span { flex: 1; }
.fishing-dialog-footer .el-button { min-height: 44px; }
.fishing-loadout-editor { display: grid; gap: 16px; }
.fishing-loadout-basics { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 12px; }
.fishing-loadout-rules { display: grid; grid-template-columns: 1fr; gap: 12px; }
.fishing-loadout-basics .fishing-field,
.fishing-loadout-rules .fishing-field { margin-top: 0; }
.fishing-component-fields { display: grid; grid-template-columns: repeat(4, minmax(0, 1fr)); gap: 12px 8px; padding-block: 14px; border-block: 1px solid var(--app-border); }
.fishing-component-field { display: grid; min-width: 0; justify-items: center; gap: 6px; }
.fishing-component-field > span { overflow: hidden; max-width: 100%; color: var(--fish-muted); font-size: 11px; text-overflow: ellipsis; white-space: nowrap; }
.fishing-component-field > button { display: grid; width: 76px; min-height: 82px; place-items: center; padding: 0; border: 0; background: transparent; cursor: pointer; }
.fishing-component-field > button:focus-visible { outline: 2px solid var(--el-color-primary-light-5); outline-offset: 3px; border-radius: 7px; }

:global(.fishing-ecology-dialog.el-dialog) {
  --ecology-accent: #17747b;
  --ecology-accent-strong: #124f55;
  --ecology-soft: color-mix(in srgb, var(--ecology-accent) 8%, var(--app-surface));
  --ecology-special: #bd8424;
  --ecology-special-soft: color-mix(in srgb, var(--ecology-special) 9%, var(--app-surface));
  --ecology-tide-line: color-mix(in srgb, var(--ecology-accent) 72%, var(--app-border));
  max-width: calc(100vw - 64px);
}
:global(.fishing-ecology-dialog .el-dialog__header) { margin: 0; padding: 24px 28px 0; }
:global(.fishing-ecology-dialog .el-dialog__body) { padding: 0 28px 26px; }
:global(.fishing-ecology-dialog .el-dialog__headerbtn) { top: 14px; right: 16px; width: 44px; height: 44px; }
:global(.fishing-ecology-dialog .el-dialog__close) { font-size: 18px; }

.fishing-ecology-header { min-width: 0; padding-right: 48px; }
.fishing-ecology-header h2 { margin: 0; color: var(--app-text); font-size: 23px; font-weight: 700; line-height: 1.3; letter-spacing: 0; }
.fishing-ecology-header p { max-width: 760px; margin: 6px 0 0; color: var(--app-muted); font-size: 14px; line-height: 1.55; }
.fishing-ecology { display: grid; gap: 20px; padding-top: 18px; }

.fishing-ecology-summary { display: grid; grid-template-columns: minmax(210px, .82fr) minmax(0, 2.2fr); border-block: 1px solid var(--app-border); }
.fishing-ecology-verdict { --tide-state: var(--ecology-accent); display: grid; align-content: center; gap: 7px; min-height: 108px; padding: 16px 22px 16px 4px; }
.fishing-ecology-verdict.is-busy { --tide-state: var(--ecology-accent); }
.fishing-ecology-verdict.is-quiet { --tide-state: color-mix(in srgb, var(--app-muted) 86%, var(--app-text)); }
.fishing-ecology-verdict > span { color: var(--app-muted); font-size: 12px; }
.fishing-ecology-verdict strong { display: flex; min-width: 0; align-items: flex-start; gap: 8px; color: var(--tide-state); font-size: 25px; line-height: 1.2; }
.fishing-ecology-verdict strong svg { flex: none; font-size: 25px; }
.fishing-ecology-verdict strong span { min-width: 0; overflow-wrap: anywhere; }

.fishing-ecology-conditions { display: grid; grid-template-columns: repeat(4, minmax(0, 1fr)); margin: 0; }
.fishing-ecology-conditions > div { position: relative; display: grid; min-width: 0; align-content: center; gap: 7px; min-height: 108px; padding: 14px 14px; }
.fishing-ecology-conditions > div::before { position: absolute; left: 0; top: 28%; height: 44%; border-left: 1px solid var(--app-border); content: ""; }
.fishing-ecology-conditions dt { display: flex; min-width: 0; align-items: center; gap: 5px; color: var(--app-muted); font-size: 11px; line-height: 1.3; overflow-wrap: anywhere; }
.fishing-ecology-conditions dt svg { flex: none; color: var(--ecology-accent); font-size: 16px; }
.fishing-ecology-conditions dd { overflow: hidden; margin: 0; color: var(--app-text); font-size: 15px; font-variant-numeric: tabular-nums; font-weight: 700; text-overflow: ellipsis; white-space: nowrap; }

.fishing-ecology-section { display: grid; gap: 12px; }
.fishing-ecology-section h3 { margin: 0; color: var(--app-text); font-size: 17px; line-height: 1.4; letter-spacing: 0; }
.fishing-tide-scroll { max-width: 100%; overflow-x: auto; border-radius: 4px; overscroll-behavior-inline: contain; scrollbar-width: thin; }
.fishing-tide-scroll:focus-visible { outline: 2px solid var(--ecology-accent); outline-offset: 2px; }
.fishing-tide-chart { display: grid; grid-template-columns: repeat(6, minmax(0, 1fr)); min-width: 720px; margin: 0; padding: 0; border-block: 1px solid var(--app-border); background: var(--app-surface); list-style: none; }
.fishing-tide-chart li { --tide-level: 50px; --tide-state: var(--ecology-accent); display: grid; min-width: 0; grid-template-rows: 48px 82px minmax(34px, auto); align-items: center; justify-items: center; padding: 8px 0 6px; }
.fishing-tide-chart li.is-busy { --tide-level: 18px; --tide-state: var(--ecology-accent); }
.fishing-tide-chart li.is-quiet { --tide-level: 70px; --tide-state: color-mix(in srgb, var(--app-muted) 86%, var(--app-text)); }
.fishing-tide-meta { display: grid; justify-items: center; gap: 7px; }
.fishing-tide-meta time { color: var(--app-text); font-size: 13px; font-variant-numeric: tabular-nums; }
.fishing-tide-meta svg { color: var(--app-text); font-size: 24px; }
.fishing-tide-line { position: relative; display: block; width: 100%; height: 82px; }
.fishing-tide-line::before { position: absolute; inset-inline: 0; top: var(--tide-level); bottom: 0; background: color-mix(in srgb, var(--ecology-accent) 7%, transparent); content: ""; }
.fishing-tide-line::after { position: absolute; left: -1px; display: none; border-left: 2px solid var(--ecology-tide-line); content: ""; }
.fishing-tide-line > i { position: absolute; z-index: 1; inset-inline: 0; top: var(--tide-level); border-top: 2px solid var(--ecology-tide-line); transform: translateY(-1px); }
.fishing-tide-line > i::after { position: absolute; left: 50%; top: -5px; width: 10px; height: 10px; border-radius: 50%; background: var(--tide-state); box-shadow: 0 0 0 3px var(--app-surface); content: ""; transform: translateX(-50%); }
.fishing-tide-chart .is-busy + .is-steady .fishing-tide-line::after,
.fishing-tide-chart .is-steady + .is-busy .fishing-tide-line::after { top: 18px; display: block; height: 32px; }
.fishing-tide-chart .is-steady + .is-quiet .fishing-tide-line::after,
.fishing-tide-chart .is-quiet + .is-steady .fishing-tide-line::after { top: 50px; display: block; height: 20px; }
.fishing-tide-chart .is-busy + .is-quiet .fishing-tide-line::after,
.fishing-tide-chart .is-quiet + .is-busy .fishing-tide-line::after { top: 18px; display: block; height: 52px; }
.fishing-tide-chart li > b { min-width: 0; align-self: end; padding-inline: 6px; color: var(--app-text); font-size: 12px; line-height: 1.35; overflow-wrap: anywhere; text-align: center; }
.fishing-tide-chart li.is-busy > b { color: color-mix(in srgb, var(--tide-state) 88%, var(--app-text)); }
.fishing-tide-chart li.is-quiet > b { color: var(--app-muted); }

.fishing-ecology-section--strange { padding-top: 2px; border-top: 1px solid var(--app-border); }
.fishing-strange-list { display: grid; }
.fishing-strange-list article { display: grid; grid-template-columns: 98px minmax(150px, .8fr) minmax(0, 1.5fr); min-width: 0; align-items: center; gap: 12px; min-height: 92px; padding: 10px 0; }
.fishing-strange-list article + article { border-top: 1px solid var(--app-border); }
.fishing-strange-list img { width: 96px; height: 72px; object-fit: contain; }
.fishing-strange-species { display: grid; min-width: 0; gap: 5px; }
.fishing-strange-species strong { overflow: hidden; color: var(--app-text); font-size: 14px; text-overflow: ellipsis; white-space: nowrap; }
.fishing-strange-species span { display: flex; min-width: 0; align-items: center; gap: 5px; color: var(--app-muted); font-size: 12px; }
.fishing-strange-species svg { flex: none; color: var(--ecology-special); font-size: 17px; }
.fishing-strange-times { display: flex; min-width: 0; flex-wrap: wrap; gap: 6px; padding-left: 14px; border-left: 3px solid color-mix(in srgb, var(--ecology-special) 82%, var(--app-border)); }
.fishing-strange-times time { min-width: 54px; padding: 6px 8px; border: 1px solid color-mix(in srgb, var(--ecology-special) 28%, var(--app-border)); border-radius: 6px; background: var(--ecology-special-soft); color: var(--app-text); font-size: 12px; font-variant-numeric: tabular-nums; text-align: center; }

:global(.dark .fishing-ecology-dialog.el-dialog) {
  --ecology-accent: #63bec0;
  --ecology-accent-strong: #8bd0d1;
  --ecology-special: #e1ad54;
  --ecology-soft: color-mix(in srgb, var(--ecology-accent) 12%, var(--app-surface));
  --ecology-special-soft: color-mix(in srgb, var(--ecology-special) 11%, var(--app-surface));
  --ecology-tide-line: color-mix(in srgb, var(--ecology-accent) 68%, var(--app-border));
}

:global(.fishing-catch-dialog.el-dialog) { max-width: calc(100vw - 28px); }
:global(.fishing-catch-dialog .el-dialog__body) { padding-top: 12px; }
.fishing-catch-detail { display: grid; grid-template-columns: 250px minmax(0, 1fr); overflow: hidden; border: 1px solid var(--app-border); border-radius: 7px; }
.fishing-catch-identity { display: grid; align-content: start; background: var(--fish-soft); }
.fishing-catch-portrait { position: relative; display: grid; place-items: center; min-height: 210px; padding: 18px; }
.fishing-catch-portrait .fish-mutation-art { width: 100%; height: 176px; }
.fishing-catch-portrait span { position: absolute; left: 12px; bottom: 10px; display: inline-flex; align-items: center; gap: 4px; padding: 5px 7px; border: 1px solid #d8b767; border-radius: 4px; background: #f3d58d; color: #65490d; font-size: 11px; font-weight: 700; }
.fishing-catch-identity > header { padding: 0 18px 14px; text-align: center; }
.fishing-catch-identity > header > span { color: var(--fish); font-size: 11px; font-weight: 700; }
.fishing-catch-identity h2 { margin: 3px 0 0; color: var(--fish-ink); font-size: 22px; }
.fishing-catch-measurements { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); border-top: 1px solid color-mix(in srgb, var(--fish) 18%, var(--app-border)); }
.fishing-catch-measurements > span { display: grid; min-width: 0; gap: 3px; padding: 12px 8px; text-align: center; }
.fishing-catch-measurements > span + span { border-left: 1px solid color-mix(in srgb, var(--fish) 18%, var(--app-border)); }
.fishing-catch-measurements small { color: var(--fish-muted); font-size: 10px; }
.fishing-catch-measurements strong { overflow: hidden; color: var(--fish-ink); font-size: 15px; text-overflow: ellipsis; white-space: nowrap; }
.fishing-catch-profile { display: grid; align-content: start; gap: 17px; padding: 20px 22px; background: var(--app-surface); }
.fishing-catch-layer { display: grid; gap: 9px; }
.fishing-catch-layer > header { display: flex; align-items: center; justify-content: space-between; gap: 10px; }
.fishing-catch-layer > header span { display: inline-flex; align-items: center; gap: 6px; color: var(--fish-muted); font-size: 11px; }
.fishing-catch-layer > header svg { color: var(--fish); font-size: 16px; }
.fishing-catch-layer > header strong { color: var(--fish-ink); font-size: 12px; text-align: right; }
.fishing-layer-profile { display: grid; overflow: hidden; border: 1px solid var(--app-border); border-radius: 6px; }
.fishing-layer-profile > span { display: grid; grid-template-columns: 12px minmax(0, 1fr); align-items: center; gap: 8px; min-height: 38px; padding: 6px 10px; background: color-mix(in srgb, var(--app-muted) 4%, var(--app-surface)); color: var(--fish-muted); }
.fishing-layer-profile > span + span { border-top: 1px solid var(--app-border); }
.fishing-layer-profile i { width: 8px; height: 8px; border: 1px solid color-mix(in srgb, var(--fish-muted) 55%, var(--app-border)); border-radius: 50%; }
.fishing-layer-profile b { font-size: 11px; font-weight: 600; }
.fishing-layer-profile > span.is-active { color: var(--fish-ink); }
.fishing-layer-profile > span.is-active:nth-child(1) { background: color-mix(in srgb, #4bb9c5 12%, var(--app-surface)); }
.fishing-layer-profile > span.is-active:nth-child(2) { background: color-mix(in srgb, #3b8f88 14%, var(--app-surface)); }
.fishing-layer-profile > span.is-active:nth-child(3) { background: color-mix(in srgb, #8a8062 14%, var(--app-surface)); }
.fishing-layer-profile > span.is-active i { border-color: var(--fish); background: var(--fish); box-shadow: 0 0 0 3px color-mix(in srgb, var(--fish) 14%, transparent); }
.fishing-catch-profile dl { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 0 18px; margin: 0; border-top: 1px solid var(--app-border); }
.fishing-catch-profile dl > div { min-width: 0; padding: 11px 0; border-bottom: 1px solid var(--app-border); }
.fishing-catch-profile dt { color: var(--fish-muted); font-size: 10px; }
.fishing-catch-profile dd { margin: 4px 0 0; color: var(--fish-ink); font-size: 13px; font-weight: 700; }
.fishing-catch-vital dd { display: flex; justify-content: space-between; gap: 8px; }
.fishing-catch-vital dd b { color: var(--fish); font-size: 11px; }
.fishing-catch-vital > i { display: block; height: 4px; margin-top: 7px; overflow: hidden; border-radius: 2px; background: var(--app-border); }
.fishing-catch-vital > i span { display: block; height: 100%; border-radius: inherit; background: var(--fish); }
.fishing-catch-profile > footer { display: flex; align-items: center; gap: 9px; min-height: 44px; }
.fishing-catch-profile > footer > svg { flex: none; color: var(--fish); font-size: 20px; }
.fishing-catch-profile > footer > span { display: grid; min-width: 0; gap: 2px; }
.fishing-catch-profile > footer strong { overflow: hidden; color: var(--fish-ink); font-size: 13px; text-overflow: ellipsis; white-space: nowrap; }
.fishing-catch-profile > footer small { color: var(--fish-muted); font-size: 11px; }
:global(.fishing-sale-dialog.el-dialog) {
  --fish: #17747b;
  --fish-dark: #124f55;
  --fish-soft: color-mix(in srgb, var(--fish) 9%, var(--app-surface));
  --fish-gold: #a76d12;
  --fish-positive: #277755;
  --fish-negative: #ad5149;
  --fish-ink: var(--app-text);
  --fish-muted: var(--app-muted);
  max-width: calc(100vw - 28px);
}
:global(.dark .fishing-sale-dialog.el-dialog) { --fish-gold: #dfad54; --fish-positive: #69bd8d; --fish-negative: #e58b82; }
:global(.fishing-sale-dialog .el-dialog__body) { padding-top: 10px; }
.fishing-sale-quote { display: grid; min-width: 0; align-content: start; gap: 12px; }
.fishing-sale-guide { overflow: hidden; border: 1px solid color-mix(in srgb, var(--fish) 28%, var(--app-border)); border-radius: 7px; background: color-mix(in srgb, var(--fish) 4%, var(--app-surface)); }
.fishing-sale-guide > header { display: grid; grid-template-columns: 38px minmax(0, 1fr) auto; align-items: center; gap: 10px; padding: 11px 13px; }
.fishing-sale-guide__icon { display: grid; place-items: center; width: 36px; height: 36px; border-radius: 6px; background: var(--fish); color: #fff; font-size: 19px; }
.fishing-sale-guide h3 { margin: 0; color: var(--fish-ink); font-size: 14px; }
.fishing-sale-guide header p { margin: 3px 0 0; color: var(--fish-muted); font-size: 11px; line-height: 1.45; }
.fishing-sale-guide time { display: inline-flex; align-items: center; gap: 5px; padding: 5px 7px; border: 1px solid var(--app-border); border-radius: 5px; background: var(--app-surface); color: var(--fish-muted); font-size: 11px; font-variant-numeric: tabular-nums; white-space: nowrap; }
.fishing-sale-guide time svg { color: var(--fish); font-size: 13px; }
.fishing-sale-guide dl { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); margin: 0; border-top: 1px solid var(--app-border); }
.fishing-sale-guide dl > div { min-width: 0; padding: 9px 11px; }
.fishing-sale-guide dl > div + div { border-left: 1px solid var(--app-border); }
.fishing-sale-guide dt { display: flex; align-items: center; gap: 5px; color: var(--fish-ink); font-size: 11px; font-weight: 700; }
.fishing-sale-guide dt svg { flex: none; color: var(--fish); font-size: 14px; }
.fishing-sale-guide dd { margin: 4px 0 0; color: var(--fish-muted); font-size: 10px; line-height: 1.45; }
.fishing-sale-guide__range { display: flex; align-items: flex-start; gap: 6px; margin: 0; padding: 7px 11px; border-top: 1px solid var(--app-border); background: color-mix(in srgb, var(--fish-gold) 8%, var(--app-surface)); color: var(--fish-muted); font-size: 10px; line-height: 1.45; }
.fishing-sale-guide__range svg { flex: none; margin-top: 1px; color: var(--fish-gold); font-size: 14px; }
.fishing-sale-lines { display: grid; grid-auto-rows: max-content; align-content: start; gap: 10px; max-height: min(52vh, 540px); overflow: auto; padding-right: 4px; scrollbar-width: thin; }
.fishing-sale-line { overflow: hidden; border: 1px solid var(--app-border); border-radius: 7px; background: var(--app-surface); }
.fishing-sale-line__identity { display: grid; grid-template-columns: 54px minmax(0, 1fr) auto; min-width: 0; align-items: center; gap: 10px; padding: 9px 12px; }
.fishing-sale-line__picture { display: grid; place-items: center; width: 52px; height: 44px; overflow: hidden; border-radius: 5px; background: var(--fish-soft); color: var(--fish); font-size: 22px; }
.fishing-sale-line__picture .fish-mutation-art { width: 50px; height: 40px; }
.fishing-sale-line__name { display: grid; min-width: 0; gap: 3px; }
.fishing-sale-line__name strong { overflow: hidden; color: var(--fish-ink); font-size: 15px; text-overflow: ellipsis; white-space: nowrap; }
.fishing-sale-line__name small { overflow: hidden; color: var(--fish-muted); font-size: 11px; text-overflow: ellipsis; white-space: nowrap; }
.fishing-sale-line__price { display: grid; grid-template-columns: auto auto; align-items: baseline; justify-items: end; column-gap: 4px; text-align: right; }
.fishing-sale-line__price small { grid-column: 1 / -1; color: var(--fish-muted); font-size: 9px; }
.fishing-sale-line__price strong { color: var(--fish-gold); font-size: 20px; font-variant-numeric: tabular-nums; line-height: 1.1; }
.fishing-sale-line__price em { color: var(--fish-muted); font-size: 10px; font-style: normal; }
.fishing-sale-equation { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); border-block: 1px solid var(--app-border); background: color-mix(in srgb, var(--app-muted) 4%, var(--app-surface)); }
.fishing-sale-equation.has-culture,
.fishing-sale-equation.has-mutation { grid-template-columns: repeat(4, minmax(0, 1fr)); }
.fishing-sale-equation.has-culture.has-mutation { grid-template-columns: repeat(5, minmax(0, 1fr)); }
.fishing-sale-equation > span { position: relative; display: grid; min-width: 0; gap: 2px; padding: 7px 24px 7px 11px; }
.fishing-sale-equation > span:not(:last-child)::after { position: absolute; top: 50%; right: 7px; color: var(--fish-muted); content: "×"; font-size: 12px; transform: translateY(-50%); }
.fishing-sale-equation small { overflow: hidden; color: var(--fish-muted); font-size: 9px; text-overflow: ellipsis; white-space: nowrap; }
.fishing-sale-equation b { overflow: hidden; color: var(--fish-ink); font-size: 13px; font-variant-numeric: tabular-nums; text-overflow: ellipsis; white-space: nowrap; }
.fishing-sale-breakdown { display: grid; grid-template-columns: minmax(0, 1.35fr) minmax(0, 1fr); }
.fishing-sale-breakdown > section { min-width: 0; padding: 9px 11px 10px; }
.fishing-sale-breakdown > section + section { border-left: 1px solid var(--app-border); }
.fishing-sale-breakdown header { display: flex; align-items: center; justify-content: space-between; gap: 8px; }
.fishing-sale-breakdown header span { display: inline-flex; min-width: 0; align-items: center; gap: 5px; color: var(--fish-ink); font-size: 10px; font-weight: 700; }
.fishing-sale-breakdown header svg { flex: none; color: var(--fish); font-size: 13px; }
.fishing-sale-breakdown header b { font-size: 11px; font-variant-numeric: tabular-nums; }
.fishing-sale-breakdown dl { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 5px 14px; margin: 7px 0 0; }
.fishing-sale-breakdown__market dl { grid-template-columns: 1fr; }
.fishing-sale-breakdown dl > div { display: flex; min-width: 0; align-items: center; justify-content: space-between; gap: 8px; }
.fishing-sale-breakdown dt { overflow: hidden; color: var(--fish-muted); font-size: 10px; text-overflow: ellipsis; white-space: nowrap; }
.fishing-sale-breakdown dd { margin: 0; color: var(--fish-ink); font-size: 11px; font-variant-numeric: tabular-nums; font-weight: 700; white-space: nowrap; }
.fishing-sale-line .is-positive { color: var(--fish-positive); }
.fishing-sale-line .is-negative { color: var(--fish-negative); }
.fishing-sale-line .is-neutral { color: var(--fish-ink); }
.fishing-sale-dialog__footer { display: flex; width: 100%; align-items: center; justify-content: space-between; gap: 14px; }
.fishing-sale-total { display: grid; min-width: 0; gap: 1px; text-align: left; }
.fishing-sale-total > span { color: var(--fish-muted); font-size: 10px; }
.fishing-sale-total strong { display: flex; align-items: baseline; gap: 5px; color: var(--fish-gold); font-size: 23px; font-variant-numeric: tabular-nums; }
.fishing-sale-total strong svg { align-self: center; font-size: 18px; }
.fishing-sale-total small { color: var(--fish-muted); font-size: 10px; font-weight: 500; }
.fishing-sale-dialog__actions { display: flex; justify-content: flex-end; gap: 8px; }
.fishing-empty { display: flex; min-height: 300px; flex-direction: column; align-items: center; justify-content: center; padding: 24px; color: var(--fish-muted); text-align: center; }
.fishing-empty > img { width: 96px; height: 96px; object-fit: contain; }
.fishing-empty > svg { color: var(--fish); font-size: 34px; }
.fishing-empty h2 { margin: 8px 0 0; color: var(--fish-ink); }
.fishing-empty p { max-width: 440px; margin: 8px 0 16px; line-height: 1.5; }
.fishing-empty.compact { min-height: 150px; }

@keyframes fish-spin { to { transform: rotate(360deg); } }
@keyframes fish-ripple { 0% { opacity: .8; transform: translate(-50%, -50%) scale(.35); } 100% { opacity: 0; transform: translate(-50%, -50%) scale(2.8); } }
@keyframes fish-water-trace { from { transform: translateX(-1.4%); } to { transform: translateX(1.4%); } }
@keyframes fish-bobber { 0%, 100% { transform: translateY(0) rotate(8deg); } 50% { transform: translateY(4px) rotate(5deg); } }
@keyframes fish-bobber-bite { from { transform: translateY(0) rotate(8deg); } to { transform: translateY(13px) rotate(20deg); } }
@keyframes fish-bobber-fight { from { transform: translate(-2px, 0) rotate(19deg); } to { transform: translate(5px, 7px) rotate(31deg); } }

@media (max-width: 980px) {
  .fishing-water-layout,
  .fishing-live { grid-template-columns: 1fr; }
  .fishing-spot-detail { position: static; }
  .fishing-live-water { height: 460px; min-height: 460px; }
  .fishing-live-panel { border-top: 1px solid var(--app-border); }
  .fishing-journal-grid { grid-template-columns: repeat(2, minmax(0, 1fr)); }
  .fishing-history-stats { grid-template-columns: repeat(3, minmax(0, 1fr)); }
  .fishing-history-stats > div:nth-child(4) { border-left: 0; }
  .fishing-history-stats > div:nth-child(n + 4) { border-top: 1px solid var(--app-border); }
}

@media (max-width: 640px) {
  .fishing-angler-control { grid-template-columns: 26px minmax(36px, 1fr) auto 10px; width: 150px; min-width: 150px; gap: 3px; padding-inline: 5px; }
  .fishing-angler-control__level { padding-left: 4px; }
  .fishing-tabs :deep(.el-tabs__nav) { width: max-content; min-width: 100%; }
  .fishing-tabs :deep(.el-tabs__item) { flex: 1 0 auto; min-width: 60px; padding: 0 10px; }
  .fishing-status-strip { grid-template-columns: repeat(2, minmax(0, 1fr)); }
  .fishing-status-strip > div:nth-child(2) { border-right: 0; }
  .fishing-status-strip > div:nth-child(-n + 2) { border-bottom: 1px solid var(--app-border); }
  .fishing-water-switch,
  .fishing-spot-list { grid-template-columns: 1fr; }
  .fishing-spot-list > button { min-height: 96px; }
  .fishing-live-water { height: 380px; min-height: 380px; }
  .fishing-live-water__head { margin: 11px 12px; font-size: 10px; }
  .fishing-live-water__head strong { max-width: 120px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .fishing-live-bobber { left: calc(61% - 14px); width: 28px; height: 28px; }
  .fishing-ripples i { left: 61%; width: 72px; height: 16px; }
  .fishing-live-water__phase { left: 12px; bottom: 56px; width: calc(100% - 28px); gap: 8px; }
  .fishing-live-water__phase strong { font-size: 19px; }
  .fishing-live-water__phase-copy > span:last-child { font-size: 11px; }
  .fishing-live-water__end { margin: auto 9px 9px; padding: 4px 7px; font-size: 10px; }
  .fishing-live-panel { padding: 12px; }
  .fishing-live-signals > div { min-height: 74px; padding-inline: 7px; }
  .fishing-live-signals dt { gap: 4px; font-size: 11px; }
  .fishing-live-signals dd { font-size: 12px; }
  .fishing-stamina { grid-template-columns: 1fr; gap: 3px; }
  .fishing-live-log ol { max-height: 248px; }
  .fishing-live-log li { grid-template-columns: 32px minmax(0, 1fr); }
  .fishing-live-log time { display: none; }
  .fishing-section-head { align-items: flex-start; }
  .fishing-loadout-list article { grid-template-columns: 52px minmax(0, 1fr) auto; gap: 9px; }
  .fishing-loadout-list article > img { width: 48px; height: 48px; }
  .fishing-loadout-list dl { grid-column: 2 / 4; gap: 14px; }
  .fishing-row-actions { grid-column: 3; grid-row: 1; }
  .fishing-catch-toolbar { display: grid; grid-template-columns: minmax(0, 1fr); align-items: stretch; }
  .fishing-subnav { width: 100%; }
  .fishing-catch-toolbar .fishing-heading-actions { justify-content: flex-end; }
  .fishing-catch-item { justify-self: center; }
  .fishing-journal-grid { grid-template-columns: 1fr; }
  .fishing-record-controls { flex-direction: column; }
  .fishing-record-controls .el-select { width: 100%; }
  .fishing-history-trip > summary { grid-template-columns: 34px minmax(0, 1fr) 18px; gap: 8px; min-height: 96px; padding-inline: 9px; }
  .fishing-history-trip__status { grid-row: 1 / 3; width: 32px; height: 32px; font-size: 16px; }
  .fishing-history-trip__identity { grid-column: 2; grid-row: 1; }
  .fishing-history-trip__result { grid-column: 2; grid-row: 2; min-width: 0; grid-auto-flow: column; justify-content: start; justify-items: start; gap: 8px; text-align: left; }
  .fishing-history-trip__chevron { grid-column: 3; grid-row: 1 / 3; }
  .fishing-history-trip__body { padding: 10px 9px; }
  .fishing-history-trip__meta { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 7px 10px; }
  .fishing-history-trip__meta span { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .fishing-history-events li { grid-template-columns: 28px minmax(0, 1fr); }
  .fishing-history-events li > time { display: none; }
  .fishing-personal-record,
  .fishing-my-entries { align-items: flex-start; gap: 8px; flex-wrap: wrap; }
  .fishing-personal-record > span,
  .fishing-my-entries > span { width: 100%; }
  .fishing-tournament-hero { grid-template-columns: 70px minmax(0, 1fr) auto; gap: 8px; padding: 8px; }
  .fishing-target-fish { width: 68px; height: 64px; }
  .fishing-tournament-hero h2 { font-size: 17px; }
  .fishing-token { padding-inline: 7px; }
  .fishing-token span { display: none; }
  .fishing-token svg { grid-row: auto; }
  .fishing-tournament-toolbar { align-items: stretch; flex-direction: column; padding-block: 8px; }
  .fishing-tournament-toolbar :deep(.el-segmented) { width: 100%; }
  .fishing-standard-rules { padding-inline: 9px; }
  .fishing-standard-rules dl { grid-template-columns: repeat(3, minmax(0, 1fr)); }
  .fishing-standard-rules dl > div:nth-child(4) { border-left: 0; }
  .fishing-standard-rules dl > div:nth-child(n + 4) { border-top: 1px solid var(--app-border); }
  .fishing-standard-rules > header { align-items: flex-start; flex-direction: column; gap: 4px; }
  .fishing-standard-tackle { grid-template-columns: repeat(2, minmax(0, 1fr)); }
  .fishing-standard-tackle > div:nth-child(4n) { border-right: 1px solid var(--app-border); }
  .fishing-standard-tackle > div:nth-child(2n) { border-right: 0; }
  .fishing-standard-tackle > div:nth-last-child(-n + 3) { border-bottom: 1px solid var(--app-border); }
  .fishing-standard-tackle > div:last-child { border-bottom: 0; }
  .fishing-standard-status { align-items: flex-start; flex-direction: column; padding-block: 7px; }
  .fishing-reward,
  .fishing-catch-up { align-items: stretch; flex-direction: column; }
  .fishing-duration-options { grid-template-columns: repeat(2, minmax(0, 1fr)); }
  .fishing-tournament-choice > header { align-items: flex-start; flex-direction: column; gap: 2px; }
  .fishing-tournament-choice > header small { text-align: left; }
  .fishing-tournament-choice > div { grid-template-columns: 1fr; }
  .fishing-supply-grid { grid-template-columns: 1fr; }
  .fishing-loadout-basics,
  .fishing-component-fields { grid-template-columns: repeat(2, minmax(0, 1fr)); }
  .fishing-loadout-rules { grid-template-columns: 1fr; }
  .fishing-supply-row { grid-template-columns: minmax(0, 1fr) 104px; }
  .fishing-supply-pack .el-input-number { width: 104px; }
  .fishing-more-settings__body { grid-template-columns: 1fr; gap: 0; }
  .fishing-setting--wide { grid-column: auto; }
  .fishing-setting__head { align-items: flex-start; flex-direction: column; gap: 4px; }
  .fishing-setting__head small { text-align: left; }
  .fishing-dialog-footer .el-button { min-height: 44px; padding-inline: 10px; }
  .fishing-catch-detail { grid-template-columns: 1fr; }
  .fishing-catch-portrait { min-height: 184px; }
  .fishing-catch-portrait .fish-mutation-art { height: 150px; }
  .fishing-catch-profile { gap: 14px; padding: 16px; }
  .fishing-catch-detail dl { grid-template-columns: 1fr; }
  .fishing-sale-guide > header { grid-template-columns: 34px minmax(0, 1fr); padding: 10px; }
  .fishing-sale-guide__icon { width: 32px; height: 32px; }
  .fishing-sale-guide time { grid-column: 2; justify-self: start; }
  .fishing-sale-guide dl { grid-template-columns: 1fr; }
  .fishing-sale-guide dl > div { padding: 8px 10px; }
  .fishing-sale-guide dl > div + div { border-top: 1px solid var(--app-border); border-left: 0; }
  .fishing-sale-lines { padding-right: 0; }
  .fishing-sale-line__identity { grid-template-columns: 46px minmax(0, 1fr) auto; gap: 8px; padding-inline: 9px; }
  .fishing-sale-line__picture { width: 44px; height: 40px; }
  .fishing-sale-line__picture .fish-mutation-art { width: 42px; height: 36px; }
  .fishing-sale-line__name strong { font-size: 14px; }
  .fishing-sale-line__name small { font-size: 10px; }
  .fishing-sale-line__price strong { font-size: 18px; }
  .fishing-sale-equation > span { padding-inline: 9px 20px; }
  .fishing-sale-equation > span:not(:last-child)::after { right: 5px; }
  .fishing-sale-equation.has-culture,
  .fishing-sale-equation.has-mutation,
  .fishing-sale-equation.has-culture.has-mutation { grid-template-columns: repeat(2, minmax(0, 1fr)); }
  .fishing-sale-breakdown { grid-template-columns: 1fr; }
  .fishing-sale-breakdown > section + section { border-top: 1px solid var(--app-border); border-left: 0; }
  .fishing-sale-breakdown__market dl { grid-template-columns: repeat(2, minmax(0, 1fr)); }
  .fishing-sale-dialog__footer { gap: 8px; }
  .fishing-sale-total strong { font-size: 20px; }
  .fishing-sale-dialog__actions { gap: 5px; }
  .fishing-sale-dialog__actions :deep(.el-button) { min-height: 40px; padding-inline: 9px; }
}

.fishing-departure-dock {
  box-sizing: border-box;
  width: 100%;
  padding: 8px 60px max(8px, env(safe-area-inset-bottom, 0px)) 8px;
  border-top: 1px solid color-mix(in srgb, var(--app-border) 78%, transparent);
  background: color-mix(in srgb, var(--app-surface) 94%, transparent);
  box-shadow: 0 -6px 18px color-mix(in srgb, var(--app-text) 10%, transparent);
  backdrop-filter: blur(8px);
}

.fishing-departure-dock .fishing-main-action {
  min-height: 48px;
}

@media (min-width: 768px) and (max-width: 879px) {
  .fishing-tide-scroll { margin-inline: -28px; padding-inline: 28px; scroll-padding-inline: 28px; scroll-snap-type: inline proximity; }
  .fishing-tide-chart { grid-template-columns: none; grid-auto-flow: column; grid-auto-columns: 112px; min-width: max-content; }
  .fishing-tide-chart li { scroll-snap-align: start; }
}

@media (max-width: 767px) {
  :global(.fishing-sale-dialog .el-dialog__body) { padding-top: 8px; }
  .fishing-sale-lines { max-height: none; overflow: visible; }
  :global(.fishing-ecology-dialog.el-dialog) { max-width: 100%; }
  :global(.fishing-ecology-dialog .el-dialog__header) { padding: 18px 18px 0; }
  :global(.fishing-ecology-dialog .el-dialog__body) { padding: 0 18px max(24px, env(safe-area-inset-bottom)); }
  :global(.fishing-ecology-dialog .el-dialog__headerbtn) { top: 8px; right: 8px; }
  .fishing-ecology-header { padding-right: 42px; }
  .fishing-ecology-header h2 { font-size: 20px; }
  .fishing-ecology-header p { margin-top: 4px; font-size: 12px; line-height: 1.45; }
  .fishing-ecology { gap: 20px; padding-top: 16px; }
  .fishing-ecology-summary { grid-template-columns: 1fr; }
  .fishing-ecology-verdict { min-height: 88px; padding: 14px 4px; }
  .fishing-ecology-verdict strong { font-size: 23px; }
  .fishing-ecology-conditions { grid-template-columns: repeat(2, minmax(0, 1fr)); border-top: 1px solid var(--app-border); }
  .fishing-ecology-conditions > div { min-height: 66px; padding: 10px 8px; border-left: 0; }
  .fishing-ecology-conditions > div::before { display: none; }
  .fishing-ecology-conditions > div:nth-child(even) { border-left: 1px solid var(--app-border); }
  .fishing-ecology-conditions > div:nth-child(n + 3) { border-top: 1px solid var(--app-border); }
  .fishing-ecology-conditions dd { font-size: 14px; }
  .fishing-tide-scroll { margin-inline: -18px; padding-inline: 18px; scroll-padding-inline: 18px; scroll-snap-type: inline proximity; -webkit-overflow-scrolling: touch; }
  .fishing-tide-chart { grid-template-columns: none; grid-auto-flow: column; grid-auto-columns: 96px; min-width: max-content; }
  .fishing-tide-chart li { scroll-snap-align: start; }
  .fishing-strange-list article { grid-template-columns: 74px minmax(0, 1fr); gap: 8px 10px; min-height: 94px; padding-block: 10px; }
  .fishing-strange-list img { width: 72px; height: 60px; }
  .fishing-strange-times { grid-column: 2; padding-left: 0; border-left: 0; }
  .fishing-strange-times time { min-width: 50px; padding: 5px 7px; }
}

@media (prefers-reduced-motion: reduce) {
  .fishing-history-trip__chevron { transition: none; }
  .fishing-history-events__loading svg,
  .fishing-loading svg,
  .fishing-water-traces,
  .fishing-ripples i,
  .fishing-live-bobber { animation: none !important; }
}

</style>
