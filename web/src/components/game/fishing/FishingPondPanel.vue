<template>
  <div class="pond-panel">
    <div v-if="loading && !pond" class="pond-loading">
      <Icon icon="lucide:loader-circle" />
      <span>{{ t("game.fishing.pondLoading") }}</span>
    </div>

    <section v-else-if="pond && !pond.built" class="pond-unbuilt">
      <span><Icon icon="lucide:construction" /></span>
      <div><h2>{{ t("game.fishing.pondNotBuilt") }}</h2><p>{{ t("game.fishing.pondNotBuiltHint") }}</p></div>
      <el-button type="primary" @click="router.push('/game/building')"><Icon icon="lucide:hammer" />{{ t("game.fishing.pondGoBuild") }}</el-button>
    </section>

    <template v-else-if="pond">
      <section class="pond-status" aria-live="polite" data-tutorial="pond-status">
        <div><Icon icon="lucide:fish-symbol" /><span>{{ t("game.fishing.pondLevel") }}</span><strong>{{ pond.level }}</strong></div>
        <div><Icon icon="lucide:fish" /><span>{{ t("game.fishing.pondMainZone") }}</span><strong>{{ mainPondFish.length }} / {{ pond.limits.fishCapacity }}</strong></div>
        <div><Icon icon="game-icons:fish-eggs" /><span>{{ t("game.fishing.pondNurseryZone") }}</span><strong>{{ nurseryPondFish.length }} / {{ pond.limits.nurseryCapacity }}</strong></div>
        <div><Icon icon="lucide:weight" /><span>{{ t("game.fishing.pondBiomass") }}</span><strong>{{ formatWeight(pond.biomassKg) }} / {{ formatWeight(pond.limits.biomassKg) }}</strong></div>
        <div><Icon icon="lucide:wheat" /><span>{{ t("game.fishing.pondFeeder") }}</span><strong>{{ formatNumber(pond.feedTotalNutrition) }} / {{ formatNumber(pond.limits.feederNutrition) }}</strong></div>
      </section>

      <section class="pond-layout">
        <div class="pond-main">
          <div class="pond-zone-toolbar">
            <el-segmented v-model="activePondZone" :options="pondZoneOptions" data-testid="pond-zone-switch" />
            <span v-if="activePondZone === 'nursery'"><Icon icon="lucide:shield-check" />{{ t("game.fishing.pondNurseryProtected") }}</span>
          </div>
          <PondWaterScene
            data-tutorial="pond-water"
            :fish="visiblePondFish"
            :species="species"
            :mutations="pond.mutations || []"
            :capabilities="ownerCapabilities"
            :scatter-mode="Boolean(pendingScatter)"
            :scatter-busy="busyId === 'feed-scatter'"
            :feed-disabled="feederFull || Boolean(busyId)"
            :selected-feed-label="pendingScatter ? itemName(pendingScatter.itemId) : ''"
            :selected-feed-quantity="pendingScatter?.quantity || 1"
            :feed-burst="feedBurst"
            @request-feed="openFeedDialog('scatter')"
            @cancel-scatter="cancelScatter"
            @scatter="scatterFeedAt"
          />

          <header class="pond-section-head" data-tutorial="pond-residents">
            <div><span>{{ t(activePondZone === 'main' ? "game.fishing.pondMainZone" : "game.fishing.pondNurseryZone") }}</span><h2>{{ t("game.fishing.pondFishCount", { count: visiblePondFish.length }) }}</h2></div>
            <el-button v-if="activePondZone === 'main'" type="primary" :loading="loadingStockFish" :disabled="(!pond.availableFish.length && !pond.availableFishNextCursor) || remainingFishCapacity <= 0" @click="openStockDialog">
              <Icon icon="lucide:plus" />{{ t("game.fishing.pondStockFish") }}
            </el-button>
          </header>

          <div v-if="visiblePondFish.length" class="pond-fish-list">
            <article v-for="row in visiblePondFish" :key="row.instanceId" :class="[mutationClasses(row), { 'is-dead': row.fish.lifeState === 'dead' }]" :data-fish-mutations="mutationIds(row).join(' ') || undefined">
              <FishMutationArt :src="speciesIcon(row.fish.speciesId)" :fish="row.fish" :mutations="pond.mutations || []" :dead="row.fish.lifeState === 'dead'" :animated="false" width="48px" />
              <div class="pond-fish-copy">
                <div class="pond-fish-identity"><h3 :class="customNameClass(row)">{{ displayFishName(row) }}</h3><span>{{ sexLabel(row.fish.sex) }} · {{ t("game.fishing.pondGeneration", { value: row.fish.generation || 0 }) }}</span></div>
                <small v-if="customName(row)">{{ speciesName(row.fish.speciesId) }} · <RarityText :rarity-id="fishRarity(row)" /></small>
                <small v-else><RarityText :rarity-id="fishRarity(row)" /> · {{ formatAge(row.fish.ageDays) }} · {{ formatLength(row.fish.lengthCm) }}<FishSizeIndicator metric="length" :value="row.fish.lengthCm" :species="speciesById.get(row.fish.speciesId)" /> · {{ formatWeight(row.fish.weightKg) }}<FishSizeIndicator metric="weight" :value="row.fish.weightKg" :species="speciesById.get(row.fish.speciesId)" /></small>
                <small v-if="customName(row)">{{ formatAge(row.fish.ageDays) }} · {{ formatLength(row.fish.lengthCm) }}<FishSizeIndicator metric="length" :value="row.fish.lengthCm" :species="speciesById.get(row.fish.speciesId)" /> · {{ formatWeight(row.fish.weightKg) }}<FishSizeIndicator metric="weight" :value="row.fish.weightKg" :species="speciesById.get(row.fish.speciesId)" /></small>
                <div class="pond-fish-bars">
                  <label><span>{{ t("game.fishing.pondSatiety") }}</span><el-progress :percentage="clampPercent(row.fish.satiety ?? 100)" :show-text="false" :stroke-width="5" /><small>{{ clampPercent(row.fish.satiety ?? 100) }}%</small></label>
                  <label><span>{{ t("game.fishing.health") }}</span><el-progress :percentage="clampPercent((row.fish.health || 0) * 100)" :show-text="false" :stroke-width="5" color="#2f8f6b" /><small>{{ clampPercent((row.fish.health || 0) * 100) }}%</small></label>
                </div>
                <div class="pond-fish-tags">
                  <el-tooltip v-if="(row.naturalProtectionHours || 0) > 0 && row.fish.lifeState !== 'dead'" :content="t('game.fishing.pondCareWarning')"><el-tag size="small" type="success" effect="plain"><Icon icon="lucide:shield-check" />{{ t('game.fishing.pondCareRemaining', { value: formatRealDuration(row.naturalProtectionHours!) }) }}</el-tag></el-tooltip>
                  <el-tag size="small" effect="plain">{{ dietLabel(row.dietClass) }}</el-tag>
                  <el-tooltip v-if="activePondZone === 'main' && row.predation" :content="predatorRule(row)">
                    <el-tag size="small" type="danger" effect="plain"><Icon icon="lucide:utensils" />{{ t("game.fishing.pondPredator") }}</el-tag>
                  </el-tooltip>
                  <el-tooltip :content="maturityTooltip(row)" placement="top">
                    <el-tag tabindex="0" size="small" :type="row.mature ? 'success' : 'info'" effect="plain">{{ t(row.mature ? "game.fishing.pondMature" : "game.fishing.pondImmature") }}</el-tag>
                  </el-tooltip>
                  <el-tag v-if="row.fish.lifeState === 'dead'" size="small" type="danger" effect="dark">{{ t("game.fishing.pondDead") }}</el-tag>
                  <el-tooltip v-for="trait in row.phenotypes || []" :key="trait" :content="phenotypeTooltip(row, trait)" placement="top">
                    <el-tag tabindex="0" size="small" type="warning" effect="plain">{{ phenotypeLabel(trait) }}</el-tag>
                  </el-tooltip>
                  <el-tooltip v-for="mutation in mutationRows(row)" :key="mutation.id" :content="mutationTooltip(mutation)" placement="top">
                    <el-tag tabindex="0" size="small" effect="plain" class="pond-mutation-tag" :class="`is-${mutation.visualEffect}`"><Icon icon="lucide:sparkles" />{{ t(mutation.nameKey) }}</el-tag>
                  </el-tooltip>
                </div>
                <details v-if="row.fish.genomeRevealed" class="pond-genome-details">
                  <summary><Icon icon="lucide:dna" />{{ t("game.fishing.pondGenomeDetails") }}<Icon icon="lucide:chevron-down" /></summary>
                  <dl class="pond-genes">
                    <div v-for="gene in geneRows(row)" :key="gene.key"><dt>{{ gene.label }}</dt><dd :class="gene.value >= 0 ? 'is-positive' : 'is-negative'">{{ signedPercent(gene.value) }}</dd></div>
                    <div v-if="mutationRows(row).length" class="pond-genes__mutations"><dt>{{ t("game.fishing.pondMutationGenes") }}</dt><dd>{{ mutationRows(row).map((mutation) => t(mutation.nameKey)).join(" · ") }}</dd></div>
                  </dl>
                </details>
              </div>
              <div class="pond-fish-actions">
                <el-tooltip v-if="row.fish.lifeState !== 'dead'" :content="t('game.fishing.pondTargetedFeed')"><el-button circle :aria-label="t('game.fishing.pondTargetedFeed')" :disabled="!!busyId" @click="openCare(row)"><Icon icon="lucide:heart-plus" /></el-button></el-tooltip>
                <el-tooltip :content="t('game.renameItem')">
                  <el-button circle :aria-label="t('game.renameItem')" :disabled="!!busyId" @click="openFishRename(row)"><Icon icon="lucide:pencil" /></el-button>
                </el-tooltip>
                <div v-if="!row.fish.genomeRevealed && row.fish.lifeState !== 'dead'" class="pond-resource-action">
                  <el-tooltip :content="genomeTooltip">
                    <span><el-button circle :aria-label="t('game.fishing.pondRevealGenome')" :disabled="pond.lineageReagentCount <= 0" :loading="busyId === row.instanceId" @click="revealGenome(row.instanceId)"><Icon icon="lucide:dna" /></el-button></span>
                  </el-tooltip>
                  <small><Icon icon="lucide:flask-conical" />{{ pond.lineageReagentCount }}</small>
                </div>
                <el-tooltip v-if="activePondZone === 'main' && row.mature && row.fish.lifeState !== 'dead' && row.fish.speciesId === tournamentTargetSpeciesId" :content="t('game.fishing.pondSubmitTournament')">
                  <el-button circle type="warning" :aria-label="t('game.fishing.pondSubmitTournament')" :loading="busyId === `tournament-${row.instanceId}`" @click="submitTournamentFish(row.instanceId)"><Icon icon="lucide:trophy" /></el-button>
                </el-tooltip>
                <el-tooltip v-if="activePondZone === 'nursery' && row.fish.lifeState !== 'dead'" :content="moveToMainTooltip(row)">
                  <span><el-button circle type="primary" :disabled="!canMoveToMain(row)" :loading="busyId === `move-${row.instanceId}`" :aria-label="t('game.fishing.pondMoveToMain')" @click="moveToMain(row.instanceId)"><Icon icon="lucide:move-right" /></el-button></span>
                </el-tooltip>
                <el-tooltip v-if="row.fish.lifeState !== 'dead'" :content="t('game.fishing.pondRemoveFish')">
                  <el-button circle :aria-label="t('game.fishing.pondRemoveFish')" :loading="busyId === row.instanceId" @click="removeFish(row.instanceId)"><Icon icon="lucide:package-open" /></el-button>
                </el-tooltip>
                <el-tooltip v-else :content="t('game.fishing.pondProcessFish')">
                  <el-button circle type="danger" :aria-label="t('game.fishing.pondProcessFish')" :loading="busyId === row.instanceId" @click="processFish(row.instanceId)"><Icon icon="lucide:recycle" /></el-button>
                </el-tooltip>
              </div>
            </article>
          </div>
          <section v-else class="pond-empty"><Icon icon="lucide:fish-off" /><h3>{{ t(activePondZone === 'main' ? "game.fishing.pondEmpty" : "game.fishing.pondNurseryEmpty") }}</h3></section>
        </div>

        <aside class="pond-sidebar">
          <section class="pond-feed-section" data-tutorial="pond-feed">
            <header>
              <div><span>{{ t("game.fishing.pondFeeder") }}</span><h2>{{ t("game.fishing.pondFeedStock") }}</h2></div>
              <el-button type="primary" plain :disabled="feederFull" @click="openFeedDialog('stock')"><Icon icon="lucide:package-plus" />{{ t("game.fishing.pondAddFeed") }}</el-button>
            </header>
            <p class="pond-shared-feed"><Icon icon="lucide:refresh-cw" />{{ t("game.fishing.pondSharedFeed") }}</p>
            <div class="pond-feed-meter"><el-progress :percentage="feederPercent" :show-text="false" :stroke-width="10" color="#287d68" /><strong>{{ feederPercent }}%</strong></div>
            <div v-if="storedFeeds.length" class="pond-feed-list">
              <article v-for="feed in storedFeeds" :key="feed.itemId">
                <Card :item-id="feed.itemId" :quantity="feed.inventoryCount" show-zero-quantity :width="50" :height="50" :icon-size="39" />
                <div class="pond-feed-copy">
                  <strong>{{ itemName(feed.itemId) }}</strong>
                  <small>{{ t(feed.role === 'functional' || feed.effects?.length ? 'game.fishing.pondFeedFunctional' : 'game.fishing.pondFeedStaple') }} · {{ t('game.fishing.pondFeedPriority', { value: feed.consumePriority || 0 }) }}<template v-if="feed.paused"> · {{ t('game.fishing.pondFeedPaused') }}</template></small>
                  <small>{{ dietLabel(feed.feedClass) }} · {{ t("game.fishing.pondNutritionEach", { value: feed.nutrition }) }}</small>
                  <div v-if="feedDetailLabels(feed).length" class="pond-feed-traits">
                    <span v-for="label in feedDetailLabels(feed)" :key="label" :class="{ 'is-danger': feedHasHealthPenalty(feed) && label.startsWith(feedHealthPenaltyLabel) }">{{ label }}</span>
                  </div>
                  <span>{{ t("game.fishing.pondFeedStoredPortions", { value: formatFeedPortions(feedPortions(feed)) }) }}</span>
                  <div v-if="feedPortions(feed) > 0" class="pond-feed-controls">
                    <el-tooltip :content="t(feed.paused ? 'game.fishing.pondFeedResume' : 'game.fishing.pondFeedPause')"><el-button circle size="small" :disabled="Boolean(busyId)" :aria-label="t(feed.paused ? 'game.fishing.pondFeedResume' : 'game.fishing.pondFeedPause')" @click="manageFeed(feed, feed.paused ? 'resume' : 'pause')"><Icon :icon="feed.paused ? 'lucide:play' : 'lucide:pause'" /></el-button></el-tooltip>
                    <el-tooltip :content="t('game.fishing.pondFeedWithdraw')"><el-button circle size="small" :disabled="Boolean(busyId) || feedPortions(feed) < 1" :aria-label="t('game.fishing.pondFeedWithdraw')" @click="manageFeed(feed, 'withdraw')"><Icon icon="lucide:package-minus" /></el-button></el-tooltip>
                    <el-tooltip v-if="feedPortions(feed) < 1" :content="t('game.fishing.pondFeedDiscard')"><el-button circle size="small" type="danger" plain :disabled="Boolean(busyId)" :aria-label="t('game.fishing.pondFeedDiscard')" @click="manageFeed(feed, 'discard_remainder')"><Icon icon="lucide:trash-2" /></el-button></el-tooltip>
                  </div>
                </div>
              </article>
            </div>
            <p v-else class="pond-section-empty">{{ t("game.fishing.pondFeederEmpty") }}</p>
          </section>

          <section class="pond-breeding-section" data-tutorial="pond-breeding">
            <header>
              <div><span>{{ t("game.fishing.pondBreeding") }}</span><h2>{{ t("game.fishing.pondPairSlots", { used: pond.pairs.length, total: pond.limits.breedingPairs }) }}</h2></div>
              <el-button circle :disabled="pond.pairs.length >= pond.limits.breedingPairs || eligibleFemales.length === 0" :aria-label="t('game.fishing.pondAddPair')" @click="openPairDialog"><Icon icon="lucide:heart-plus" /></el-button>
            </header>
            <p v-if="nurseryFull" class="pond-nursery-full"><Icon icon="lucide:pause-circle" />{{ t("game.fishing.pondNurseryFull") }}</p>
            <div v-if="pond.pairs.length" class="pond-pairs">
              <article v-for="pair in pond.pairs" :key="pair.slotIndex">
                <div class="pond-pair-fish"><FishMutationArt :src="fishIcon(pair.femaleInstanceId)" :fish="pondFishById.get(pair.femaleInstanceId)?.fish" :mutations="pond.mutations || []" :animated="false" width="34px" /><span><small>{{ t("game.fishing.sex_female") }}</small><strong>{{ fishName(pair.femaleInstanceId) }}</strong></span></div>
                <Icon icon="lucide:heart" class="pond-pair-heart" />
                <div class="pond-pair-fish"><FishMutationArt :src="fishIcon(pair.maleInstanceId)" :fish="pondFishById.get(pair.maleInstanceId)?.fish" :mutations="pond.mutations || []" :animated="false" width="34px" /><span><small>{{ t("game.fishing.sex_male") }}</small><strong>{{ fishName(pair.maleInstanceId) }}</strong></span></div>
                <div class="pond-pair-actions">
                  <el-tooltip :content="mashTooltip">
                    <span><el-button circle :type="pair.mashReady ? 'success' : 'default'" :disabled="pair.mashReady || pond.broodstockMashCount <= 0" :loading="busyId === `pair-${pair.slotIndex}`" @click="applyMash(pair.slotIndex)"><Icon icon="lucide:soup" /></el-button></span>
                  </el-tooltip>
                  <small v-if="!pair.mashReady"><Icon icon="lucide:package" />{{ pond.broodstockMashCount }}</small>
                  <el-button circle :aria-label="t('game.fishing.pondRemovePair')" @click="removePair(pair.slotIndex)"><Icon icon="lucide:x" /></el-button>
                </div>
              </article>
            </div>
            <p v-else class="pond-section-empty">{{ t("game.fishing.pondNoPairs") }}</p>
          </section>

          <section class="pond-events" data-tutorial="pond-events">
            <header><div><span>{{ t("game.fishing.pondJournal") }}</span><h2>{{ t("game.fishing.pondRecentEvents") }}</h2></div></header>
            <ol v-if="pond.events.length" tabindex="0" :aria-label="t('game.fishing.pondRecentEvents')"><li v-for="event in pond.events.slice(0, 12)" :key="event.id"><Icon :icon="eventIcon(event.kind)" /><span><strong>{{ eventLabel(event) }}</strong><small>{{ formatEventTime(event.createdAt) }}</small></span></li></ol>
            <p v-else class="pond-section-empty">{{ t("game.fishing.pondNoEvents") }}</p>
          </section>
        </aside>
      </section>
    </template>

    <section v-else class="pond-error"><Icon icon="lucide:circle-alert" /><span>{{ t("game.fishing.pondLoadFailed") }}</span><el-button @click="loadPond">{{ t("game.fishing.refresh") }}</el-button></section>

    <ItemRenameDialog
      v-model="renameDialogOpen"
      :instance-id="renameFish?.instanceId"
      :custom-data="renameFish?.customData"
      @renamed="applyPondFishRename"
    />

    <FishSelectorDialog
      v-if="pond"
      v-model="stockDialogOpen"
      :fish="pond.availableFish"
      :species="species"
      :title="t('game.fishing.pondStockFish')"
      :confirm-label="t('game.fishing.pondConfirmStock')"
      :hint="t('game.fishing.pondStockSelectorHint', { count: remainingFishCapacity })"
      :ecology-fish="mainPondFish"
      :mutations="pond.mutations || []"
      multiple
      :max-selection="remainingFishCapacity"
      :loading="busyId === 'stock-batch'"
      @confirm="stockFishBatch"
    />

    <GameDialog v-if="pond" v-model="feedDialogOpen" :title="t(feedDialogMode === 'scatter' ? 'game.fishing.pondScatterFeed' : 'game.fishing.pondAddFeed')" width="min(760px, 94vw)">
      <p class="pond-dialog-note"><Icon icon="lucide:info" />{{ t(feedDialogMode === "scatter" ? "game.fishing.pondScatterFeedDialogHint" : "game.fishing.pondFeedDialogHint") }}</p>
      <div v-if="availableFeeds.length" ref="feedPickerRef" class="pond-feed-picker">
        <button v-for="feed in availableFeeds" :key="feed.itemId" type="button" :class="{ 'is-selected': selectedFeedId === feed.itemId }" @click="selectFeed(feed.itemId)">
          <Card :item-id="feed.itemId" :quantity="feed.inventoryCount" :width="58" :height="58" :icon-size="46" />
          <span>
            <strong>{{ itemName(feed.itemId) }}</strong>
            <small>{{ dietLabel(feed.feedClass) }} · {{ t("game.fishing.pondNutritionEach", { value: feed.nutrition }) }}</small>
            <span v-if="feed.conditions?.length" class="pond-feed-eligibility">{{ t("game.fishing.pondFeedEligibleCount", { count: eligibleFishCount(feed), total: livingPondFish.length }) }}</span>
            <span v-if="feedDetailLabels(feed).length" class="pond-feed-traits">
              <span v-for="label in feedDetailLabels(feed)" :key="label" :class="{ 'is-danger': feedHasHealthPenalty(feed) && label.startsWith(feedHealthPenaltyLabel) }">{{ label }}</span>
            </span>
            <em>{{ t("game.fishing.pondFeedOwned", { count: feed.inventoryCount }) }}</em>
          </span>
          <Icon :icon="selectedFeedId === feed.itemId ? 'lucide:circle-check' : 'lucide:circle'" />
        </button>
      </div>
      <div v-else class="pond-feed-missing"><Icon icon="lucide:wheat-off" /><span>{{ t("game.fishing.pondNoFeedInStorage") }}</span><el-button @click="router.push('/game/panel/cooking'); feedDialogOpen = false">{{ t("game.fishing.pondGoCookFeed") }}</el-button></div>
      <label v-if="selectedFeed" class="pond-feed-quantity"><span>{{ t(feedDialogMode === "scatter" ? "game.fishing.pondScatterFeedQuantity" : "game.fishing.pondFeedQuantity") }}</span><el-input-number v-model="feedQuantity" :min="1" :max="Math.max(1, selectedFeedMaximum)" controls-position="right" /></label>
      <template #footer><el-button @click="feedDialogOpen = false">{{ t("game.fishing.cancel") }}</el-button><el-button type="primary" :disabled="!selectedFeed || selectedFeedMaximum <= 0" :loading="busyId === 'feed-add'" @click="confirmFeedSelection">{{ t(feedDialogMode === "scatter" ? "game.fishing.pondPrepareScatter" : "game.fishing.pondConfirmFeed") }}</el-button></template>
    </GameDialog>

    <GameDialog v-if="pond && careFish" v-model="careDialogOpen" :title="t('game.fishing.pondTargetedFeed')" width="min(560px, 94vw)">
      <p class="pond-dialog-note"><Icon icon="lucide:fish" />{{ displayFishName(careFish) }}<span v-if="(careFish.naturalProtectionHours || 0) > 0">{{ t('game.fishing.pondCareRemaining', { value: formatRealDuration(careFish.naturalProtectionHours!) }) }}</span></p>
      <div v-if="careFeeds.length" class="pond-feed-picker pond-care-picker">
        <button v-for="feed in careFeeds" :key="feed.itemId" type="button" :class="{ 'is-selected': careFeedId === feed.itemId }" :disabled="feed.inventoryCount < 1 || !feedEligibleForFish(feed, careFish)" @click="careFeedId = feed.itemId">
          <Card :item-id="feed.itemId" :quantity="feed.inventoryCount" :width="58" :height="58" :icon-size="46" />
          <span><strong>{{ itemName(feed.itemId) }}</strong><span class="pond-feed-traits"><span v-for="label in feedDetailLabels(feed)" :key="label" :class="{ 'is-danger': feedHasHealthPenalty(feed) && label.startsWith(feedHealthPenaltyLabel) }">{{ label }}</span></span><em>{{ t('game.fishing.pondFeedOwned', { count: feed.inventoryCount }) }}</em></span>
          <Icon :icon="careFeedId === feed.itemId ? 'lucide:circle-check' : 'lucide:circle'" />
        </button>
      </div>
      <p class="pond-dialog-note">{{ t('game.fishing.pondCareWarning') }}</p>
      <template #footer><el-button @click="router.push('/game/panel/cooking'); careDialogOpen = false">{{ t('game.fishing.pondGoCookFeed') }}</el-button><el-button type="primary" :disabled="!selectedCareFeed || selectedCareFeed.inventoryCount < 1 || !feedEligibleForFish(selectedCareFeed, careFish)" :loading="busyId === 'care'" @click="confirmCare">{{ t('game.fishing.pondCareConfirm') }}</el-button></template>
    </GameDialog>

    <GameDialog v-if="pond" v-model="pairDialogOpen" :title="t('game.fishing.pondAddPair')" width="min(720px, 94vw)">
      <p class="pond-dialog-note"><Icon icon="lucide:info" />{{ t("game.fishing.pondPairHint") }}</p>
      <div class="pond-pair-form">
        <button type="button" :class="{ 'has-fish': selectedFemale }" @click="openPairSelector('female')">
          <template v-if="selectedFemale"><FishMutationArt :src="speciesIcon(selectedFemale.fish.speciesId)" :fish="selectedFemale.fish" :mutations="pond.mutations || []" width="76px" /><span><small>{{ t("game.fishing.sex_female") }}</small><strong>{{ displayFishName(selectedFemale) }}</strong><em>{{ formatLength(selectedFemale.fish.lengthCm) }}<FishSizeIndicator metric="length" :value="selectedFemale.fish.lengthCm" :species="speciesById.get(selectedFemale.fish.speciesId)" /> · {{ formatWeight(selectedFemale.fish.weightKg) }}<FishSizeIndicator metric="weight" :value="selectedFemale.fish.weightKg" :species="speciesById.get(selectedFemale.fish.speciesId)" /></em></span></template>
          <template v-else><Icon icon="lucide:circle-plus" /><span><strong>{{ t("game.fishing.pondChooseFemale") }}</strong><small>{{ t("game.fishing.pondChooseParentHint") }}</small></span></template>
        </button>
        <Icon icon="lucide:heart" />
        <button type="button" :class="{ 'has-fish': selectedMale }" :disabled="!selectedFemale" @click="openPairSelector('male')">
          <template v-if="selectedMale"><FishMutationArt :src="speciesIcon(selectedMale.fish.speciesId)" :fish="selectedMale.fish" :mutations="pond.mutations || []" width="76px" /><span><small>{{ t("game.fishing.sex_male") }}</small><strong>{{ displayFishName(selectedMale) }}</strong><em>{{ formatLength(selectedMale.fish.lengthCm) }}<FishSizeIndicator metric="length" :value="selectedMale.fish.lengthCm" :species="speciesById.get(selectedMale.fish.speciesId)" /> · {{ formatWeight(selectedMale.fish.weightKg) }}<FishSizeIndicator metric="weight" :value="selectedMale.fish.weightKg" :species="speciesById.get(selectedMale.fish.speciesId)" /></em></span></template>
          <template v-else><Icon icon="lucide:circle-plus" /><span><strong>{{ t("game.fishing.pondChooseMale") }}</strong><small>{{ selectedFemale ? t("game.fishing.pondChooseMatchingMale") : t("game.fishing.pondChooseFemaleFirst") }}</small></span></template>
        </button>
      </div>
      <template #footer><el-button @click="pairDialogOpen = false">{{ t("game.fishing.close") }}</el-button><el-button type="primary" :disabled="!pairFemaleId || !pairMaleId" :loading="busyId === 'pair-new'" @click="createPair">{{ t("game.fishing.pondConfirmPair") }}</el-button></template>
    </GameDialog>

    <FishSelectorDialog
      v-if="pond"
      v-model="pairSelectorOpen"
      :fish="pairSelectorCandidates"
      :species="species"
      :title="pairSelectorRole === 'female' ? t('game.fishing.pondChooseFemale') : t('game.fishing.pondChooseMale')"
      :confirm-label="t('game.fishing.pondConfirmParent')"
      :initial-selected-ids="pairSelectorRole === 'female' && pairFemaleId ? [pairFemaleId] : pairSelectorRole === 'male' && pairMaleId ? [pairMaleId] : []"
      :mutations="pond.mutations || []"
      @confirm="confirmPairSelector"
    />
  </div>
</template>

<script setup lang="ts">
import { computed, nextTick, ref, watch } from "vue"
import { ElMessage, ElMessageBox } from "element-plus"
import { Icon } from "@iconify/vue"
import { useI18n } from "vue-i18n"
import { useRouter } from "vue-router"
import Card from "@components/game/item/Card.vue"
import ItemRenameDialog from "@components/game/item/ItemRenameDialog.vue"
import GameDialog from "@components/game/common/GameDialog.vue"
import RarityText from "@components/game/common/RarityText.vue"
import FishSelectorDialog from "./FishSelectorDialog.vue"
import FishMutationArt from "./FishMutationArt.vue"
import FishSizeIndicator from "./FishSizeIndicator.vue"
import PondWaterScene from "./PondWaterScene.vue"
import { useGameStore } from "@stores/game"
import { useInventoryStore } from "@stores/inventory"
import { getItemDefinition } from "@services/item-definition"
import { itemCustomLabelClass } from "@services/item-customization"
import { resolveErrorMessage } from "@services/error-messages"
import { fishingPondPredationRisks, mergeFishingPondEcosystem } from "@services/fishing-pond-ecology"
import { fishMutationClassNames, fishMutationIds, resolveFishMutations, type ResolvedFishMutation } from "@services/fish-mutations"
import { OWNER_FISHING_POND_CAPABILITIES } from "@services/fishing-pond-access"
import type { ItemInstance } from "@/types/equipment"
import {
  addFishingPondFeed,
  manageFishingPondFeed,
  useFishingPondTargetedFeed,
  addFishingPondFishBatch,
  applyFishingPondMash,
  deleteFishingPondPair,
  fetchFishingPond,
  loadAllAvailablePondFish,
  fishingPondZone,
  localizedFishingText,
  moveFishingPondFishToMain,
  processFishingPondFish,
  removeFishingPondFish,
  revealFishingPondGenome,
  setFishingPondPair,
  submitFishingPondTournamentFish,
  type FishingPondEvent,
  type FishingPondFeed,
  type FishingPondFeedCondition,
  type FishingPondFish,
  type FishingPondOverview,
  type FishingPondZone,
  type FishingSpecies
} from "@services/fishing"

const props = defineProps<{ species: FishingSpecies[]; tournamentTargetSpeciesId?: string }>()
type FeedDialogMode = "stock" | "scatter"
type PendingScatter = { itemId: string; feedClass: string; quantity: number }
type FeedBurst = { id: number; x: number; y: number; feedClass: string; fishIds: number[] }

const { t, locale } = useI18n()
const router = useRouter()
const gameStore = useGameStore()
const inventoryStore = useInventoryStore()
const ownerCapabilities = OWNER_FISHING_POND_CAPABILITIES
const pond = ref<FishingPondOverview | null>(null)
const activePondZone = ref<FishingPondZone>("main")
const loading = ref(false)
const busyId = ref<string | number>("")
const stockDialogOpen = ref(false)
const feedDialogOpen = ref(false)
const feedDialogMode = ref<FeedDialogMode>("stock")
const feedPickerRef = ref<HTMLElement | null>(null)
const selectedFeedId = ref("")
const feedQuantity = ref(1)
const careDialogOpen = ref(false)
const careFish = ref<FishingPondFish | null>(null)
const careFeedId = ref('')
const careFeeds = computed(() => (pond.value?.feeds || []).filter(feed => feed.delivery === 'targeted'))
const selectedCareFeed = computed(() => careFeeds.value.find(feed => feed.itemId === careFeedId.value))
const pendingScatter = ref<PendingScatter | null>(null)
const feedBurst = ref<FeedBurst | null>(null)
const pairDialogOpen = ref(false)
const pairSelectorOpen = ref(false)
const pairSelectorRole = ref<"female" | "male">("female")
const pairFemaleId = ref(0)
const pairMaleId = ref(0)
const renameDialogOpen = ref(false)
const renameFish = ref<FishingPondFish | null>(null)

const speciesById = computed(() => new Map(props.species.map((row) => [row.id, row])))
const pondFishById = computed(() => new Map((pond.value?.fish || []).map((row) => [row.instanceId, row])))
const mainPondFish = computed(() => (pond.value?.fish || []).filter((row) => fishingPondZone(row.fish) === "main"))
const nurseryPondFish = computed(() => (pond.value?.fish || []).filter((row) => fishingPondZone(row.fish) === "nursery"))
const livingPondFish = computed(() => (pond.value?.fish || []).filter((row) => row.fish.lifeState !== "dead"))
const visiblePondFish = computed(() => activePondZone.value === "main" ? mainPondFish.value : nurseryPondFish.value)
const pondZoneOptions = computed(() => [
  { value: "main", label: `${t("game.fishing.pondMainZone")} ${mainPondFish.value.length}/${pond.value?.limits.fishCapacity || 0}` },
  { value: "nursery", label: `${t("game.fishing.pondNurseryZone")} ${nurseryPondFish.value.length}/${pond.value?.limits.nurseryCapacity || 0}` }
])
const pairedFishIds = computed(() => new Set((pond.value?.pairs || []).flatMap((pair) => [pair.femaleInstanceId, pair.maleInstanceId])))
const remainingFishCapacity = computed(() => Math.max(0, (pond.value?.limits.fishCapacity || 0) - mainPondFish.value.length))
const nurseryFull = computed(() => Boolean(pond.value) && nurseryPondFish.value.length >= (pond.value?.limits.nurseryCapacity || 0))
const feederPercent = computed(() => pond.value ? clampPercent(pond.value.feedTotalNutrition / Math.max(1, pond.value.limits.feederNutrition) * 100) : 0)
const feederFull = computed(() => !pond.value || pond.value.feedTotalNutrition >= pond.value.limits.feederNutrition - .0001)
const storedFeeds = computed(() => (pond.value?.feeds || []).filter((feed) => feed.delivery !== 'targeted' && (feed.storedNutrition > 0 || feed.inventoryCount > 0)))
const availableFeeds = computed(() => (pond.value?.feeds || []).filter((feed) => feed.delivery !== 'targeted' && feed.inventoryCount > 0 && maxFeedAmount(feed) > 0))
const selectedFeed = computed(() => (pond.value?.feeds || []).find((feed) => feed.itemId === selectedFeedId.value) || null)
const selectedFeedMaximum = computed(() => selectedFeed.value ? maxFeedAmount(selectedFeed.value) : 0)
const eligibleFemales = computed(() => mainPondFish.value.filter((row) => row.fish.sex === "female" && row.mature && row.fish.lifeState !== "dead" && !pairedFishIds.value.has(row.instanceId)))
const eligibleMales = computed(() => mainPondFish.value.filter((row) => row.fish.sex === "male" && row.mature && row.fish.lifeState !== "dead" && !pairedFishIds.value.has(row.instanceId) && (!pairFemaleId.value || row.fish.speciesId === pondFishById.value.get(pairFemaleId.value)?.fish.speciesId)))
const selectedFemale = computed(() => pondFishById.value.get(pairFemaleId.value) || null)
const selectedMale = computed(() => pondFishById.value.get(pairMaleId.value) || null)
const pairSelectorCandidates = computed(() => pairSelectorRole.value === "female" ? eligibleFemales.value : eligibleMales.value)
const genomeTooltip = computed(() => pond.value?.lineageReagentCount ? t("game.fishing.pondRevealGenomeCost", { count: pond.value.lineageReagentCount }) : t("game.fishing.pondReagentMissing"))
const mashTooltip = computed(() => pond.value?.broodstockMashCount ? t("game.fishing.pondUseMashCost", { count: pond.value.broodstockMashCount }) : t("game.fishing.pondMashMissing"))
const feedHealthPenaltyLabel = computed(() => t("game.fishing.pondFeedEffectHealthLoss"))
let feedBurstSequence = 0

watch(() => gameStore.token, (token) => {
  if (token && !pond.value && !loading.value) void loadPond()
}, { immediate: true })

watch(activePondZone, () => {
  pendingScatter.value = null
  feedBurst.value = null
})

const loadingStockFish = ref(false)
async function openStockDialog() {
  if (!pond.value || loadingStockFish.value) return
  loadingStockFish.value = true
  const current = pond.value
  try {
    const complete = await loadAllAvailablePondFish(gameStore.token, current)
    if (pond.value !== current) return
    pond.value = complete
    stockDialogOpen.value = true
  } catch (error) {
    ElMessage.error(resolveErrorMessage(t, error, "game.fishing.pondLoadFailed"))
  } finally { loadingStockFish.value = false }
}

async function loadPond() {
  if (!gameStore.token) return
  loading.value = true
  try { pond.value = await fetchFishingPond(gameStore.token) } catch { pond.value = null } finally { loading.value = false }
}

async function mutate(id: string | number, action: () => Promise<FishingPondOverview>, successKey: string) {
  if (!gameStore.token || busyId.value) return false
  busyId.value = id
  try {
    pond.value = await action()
    await inventoryStore.refreshAll(gameStore.token)
    ElMessage.success(t(successKey))
    return true
  } catch (error) {
    ElMessage.error(resolveErrorMessage(t, error, "game.fishing.pondActionFailed"))
    return false
  } finally { busyId.value = "" }
}

async function stockFishBatch(instanceIds: number[]) {
  if (instanceIds.length === 0 || instanceIds.length > remainingFishCapacity.value) return
  const selectedIds = new Set(instanceIds)
  const selected = (pond.value?.availableFish || []).filter((row) => selectedIds.has(row.instanceId))
  const introducedRisks = fishingPondPredationRisks(mergeFishingPondEcosystem(mainPondFish.value, selected))
    .filter((risk) => selectedIds.has(risk.predator.instanceId) || selectedIds.has(risk.prey.instanceId))
  if (introducedRisks.length) {
    const preyCount = new Set(introducedRisks.map((risk) => risk.prey.instanceId)).size
    try {
      await ElMessageBox.confirm(
        t("game.fishing.pondPredationStockConfirm", { count: preyCount }),
        t("game.fishing.pondPredationRiskTitle"),
        { type: "warning", confirmButtonText: t("game.fishing.pondConfirmStock"), cancelButtonText: t("game.fishing.cancel") }
      )
    } catch { return }
  }
  if (await mutate("stock-batch", () => addFishingPondFishBatch(gameStore.token, instanceIds), "game.fishing.pondStockedBatch")) stockDialogOpen.value = false
}
async function removeFish(instanceId: number) { await mutate(instanceId, () => removeFishingPondFish(gameStore.token, instanceId), "game.fishing.pondRemoved") }
function openCare(row: FishingPondFish) {
  careFish.value = row
  careFeedId.value = careFeeds.value.find(feed => feed.inventoryCount > 0 && feedEligibleForFish(feed, row))?.itemId || ''
  careDialogOpen.value = true
}
async function confirmCare() {
  const fish = careFish.value, feed = selectedCareFeed.value
  if (!fish || !feed || feed.inventoryCount < 1 || busyId.value) return
  const success = await mutate('care', () => useFishingPondTargetedFeed(gameStore.token, fish.instanceId, feed.itemId, fish.careRevision ?? 0), 'game.fishing.pondCareDone')
  careDialogOpen.value = false
  if (!success) await loadPond()
}
async function moveToMain(instanceId: number) { await mutate(`move-${instanceId}`, () => moveFishingPondFishToMain(gameStore.token, instanceId), "game.fishing.pondMovedToMain") }
async function revealGenome(instanceId: number) { if ((pond.value?.lineageReagentCount || 0) <= 0) return; await mutate(instanceId, () => revealFishingPondGenome(gameStore.token, instanceId), "game.fishing.pondGenomeRevealed") }
async function submitTournamentFish(instanceId: number) { if (!gameStore.token || busyId.value) return; busyId.value = `tournament-${instanceId}`; try { await submitFishingPondTournamentFish(gameStore.token, instanceId); ElMessage.success(t("game.fishing.pondTournamentSubmitted")) } catch { ElMessage.error(t("game.fishing.pondTournamentUnavailable")) } finally { busyId.value = "" } }
async function processFish(instanceId: number) { try { await ElMessageBox.confirm(t("game.fishing.pondProcessConfirm"), t("game.fishing.pondProcessFish"), { type: "warning" }) } catch { return }; await mutate(instanceId, () => processFishingPondFish(gameStore.token, instanceId), "game.fishing.pondProcessed") }

function openFishRename(row: FishingPondFish) {
  if (busyId.value) return
  renameFish.value = row
  renameDialogOpen.value = true
}

function applyPondFishRename(instance: ItemInstance) {
  const row = pond.value?.fish.find((candidate) => candidate.instanceId === instance.instanceId)
  if (!row) return
  row.customData = { ...(instance.customData || {}) }
  renameFish.value = row
}

function openFeedDialog(mode: FeedDialogMode) {
  if (mode === "stock") pendingScatter.value = null
  feedDialogMode.value = mode
  selectedFeedId.value = availableFeeds.value[0]?.itemId || ""
  feedQuantity.value = 1
  feedDialogOpen.value = true
  void nextTick(() => { if (feedPickerRef.value) feedPickerRef.value.scrollTop = 0 })
}
function selectFeed(itemId: string) { selectedFeedId.value = itemId; feedQuantity.value = 1 }
async function confirmFeedSelection() {
  if (!selectedFeed.value || selectedFeedMaximum.value <= 0) return
  if (feedHasHealthPenalty(selectedFeed.value)) {
    try {
      await ElMessageBox.confirm(
        t("game.fishing.pondFeedDangerConfirm", { name: itemName(selectedFeed.value.itemId) }),
        t("game.fishing.pondFeedDangerTitle"),
        { type: "warning", confirmButtonText: t("game.fishing.pondFeedDangerContinue"), cancelButtonText: t("game.fishing.cancel") }
      )
    } catch { return }
  }
  const quantity = Math.max(1, Math.min(selectedFeedMaximum.value, feedQuantity.value))
  if (feedDialogMode.value === "scatter") {
    pendingScatter.value = { itemId: selectedFeed.value.itemId, feedClass: selectedFeed.value.feedClass, quantity }
    feedDialogOpen.value = false
    return
  }
  if (await mutate("feed-add", () => addFishingPondFeed(gameStore.token, selectedFeed.value!.itemId, quantity), "game.fishing.pondFeedAdded")) feedDialogOpen.value = false
}
function cancelScatter() { if (busyId.value !== "feed-scatter") pendingScatter.value = null }
async function scatterFeedAt(position: { x: number; y: number }) {
  const selection = pendingScatter.value
  const feed = pond.value?.feeds.find((row) => row.itemId === selection?.itemId)
  if (!selection || !feed || maxFeedAmount(feed) <= 0) {
    pendingScatter.value = null
    ElMessage.error(t("game.fishing.pondScatterUnavailable"))
    return
  }
  const quantity = Math.max(1, Math.min(selection.quantity, maxFeedAmount(feed)))
  const fishIds = visiblePondFish.value
    .filter((row) => row.fish.lifeState !== "dead" && Number(row.fish.satiety ?? 100) < 100 && feedEligibleForFish(feed, row))
    .map((row) => row.instanceId)
  if (!(await mutate("feed-scatter", () => addFishingPondFeed(gameStore.token, selection.itemId, quantity), "game.fishing.pondFeedScattered"))) return
  feedBurst.value = { id: ++feedBurstSequence, x: position.x, y: position.y, feedClass: selection.feedClass, fishIds }
  pendingScatter.value = null
}

function openPairDialog() { pairFemaleId.value = 0; pairMaleId.value = 0; pairDialogOpen.value = true }
function openPairSelector(role: "female" | "male") { pairSelectorRole.value = role; pairSelectorOpen.value = true }
function confirmPairSelector(instanceIds: number[]) { const instanceId = instanceIds[0] || 0; if (pairSelectorRole.value === "female") { if (pairFemaleId.value !== instanceId) pairMaleId.value = 0; pairFemaleId.value = instanceId } else pairMaleId.value = instanceId; pairSelectorOpen.value = false }
async function createPair() { if (!(await mutate("pair-new", () => setFishingPondPair(gameStore.token, pairFemaleId.value, pairMaleId.value), "game.fishing.pondPairCreated"))) return; pairDialogOpen.value = false; pairFemaleId.value = 0; pairMaleId.value = 0 }
async function removePair(slotIndex: number) { await mutate(`pair-${slotIndex}`, () => deleteFishingPondPair(gameStore.token, slotIndex), "game.fishing.pondPairRemoved") }
async function applyMash(slotIndex: number) { if ((pond.value?.broodstockMashCount || 0) <= 0) return; await mutate(`pair-${slotIndex}`, () => applyFishingPondMash(gameStore.token, slotIndex), "game.fishing.pondMashApplied") }

function speciesName(speciesId: string) { const row = speciesById.value.get(speciesId); return localizedFishingText(row?.names, locale.value) || speciesId }
function speciesIcon(speciesId: string) { return speciesById.value.get(speciesId)?.iconPath || "/assets/icons/item/fish_catch.png" }
function fishRarity(row: FishingPondFish) { return speciesById.value.get(row.fish.speciesId)?.rarity || "common" }
function customName(row: FishingPondFish) { return String(row.customData?.name || "").trim() }
function displayFishName(row: FishingPondFish) { return customName(row) || speciesName(row.fish.speciesId) }
function customNameClass(row: FishingPondFish) { return customName(row) ? itemCustomLabelClass(row.customData) : "" }
function fishName(instanceId: number) { const row = pondFishById.value.get(instanceId); return row ? displayFishName(row) : t("game.fishing.unknownFish") }
function fishIcon(instanceId: number) { const row = pondFishById.value.get(instanceId); return row ? speciesIcon(row.fish.speciesId) : "/assets/icons/item/fish_catch.png" }
function itemName(itemId: string) { const definition = getItemDefinition(itemId); return definition?.nameKey ? t(definition.nameKey) : itemId }
function feedHasHealthPenalty(feed: FishingPondFeed) { return (feed.effects || []).some((effect) => effect.stat === "health" && Number(effect.valuePerPortion) < 0) }
function feedPortions(feed: FishingPondFeed) { return feed.storedPortions ?? feed.storedNutrition / feed.nutrition }
function formatFeedPortions(value: number) {
  if (value > 0 && value < 0.000001) return '<0.000001'
  return (Math.floor(value * 1e6) / 1e6).toLocaleString(locale.value, { maximumFractionDigits: 6 })
}
async function manageFeed(feed: FishingPondFeed, operation: 'pause' | 'resume' | 'withdraw' | 'discard_remainder') {
  if (busyId.value) return
  let quantity = 0
  try {
    if (operation === 'withdraw') {
      const maximum = Math.min(10000, Math.floor(feedPortions(feed)))
      const result = await ElMessageBox.prompt(t('game.fishing.pondFeedWithdrawCount', { value: maximum }), itemName(feed.itemId), {
        inputValue: String(maximum), inputType: 'number',
        inputValidator: value => { const n = Number(value); return Number.isInteger(n) && n >= 1 && n <= maximum },
        confirmButtonText: t('game.fishing.pondFeedWithdraw'), cancelButtonText: t('game.fishing.cancel')
      })
      quantity = Number(result.value)
    } else if (operation === 'discard_remainder') {
      await ElMessageBox.confirm(t('game.fishing.pondFeedDiscardConfirm', { value: formatFeedPortions(feedPortions(feed)), name: itemName(feed.itemId) }), t('game.fishing.pondFeedDiscard'), { type: 'warning', confirmButtonText: t('game.fishing.pondFeedDiscard'), cancelButtonText: t('game.fishing.cancel') })
    }
  } catch { return }
  await mutate('feed-manage', () => manageFishingPondFeed(gameStore.token, feed.itemId, operation, quantity), 'game.fishing.pondFeedManaged')
}
function eligibleFishCount(feed: FishingPondFeed) { return livingPondFish.value.filter((row) => feedEligibleForFish(feed, row)).length }
function feedEligibleForFish(feed: FishingPondFeed, row: FishingPondFish) {
  return (feed.conditions || []).every((condition) => {
    let actual: string | number
    if (condition.field === "dietClass") actual = row.dietClass
    else if (condition.field === "pondZone") actual = fishingPondZone(row.fish)
    else if (condition.field === "health") actual = Number(row.fish.health || 0)
    else if (condition.field === "satiety") actual = Number(row.fish.satiety ?? 100)
    else if (condition.field === "condition") actual = Number(row.fish.condition || 0)
    else return false
    if (typeof actual === "string" || typeof condition.value === "string") {
      if (typeof actual !== "string" || typeof condition.value !== "string") return false
      return condition.op === "eq" ? actual === condition.value : condition.op === "neq" ? actual !== condition.value : false
    }
    const expected = Number(condition.value)
    if (!Number.isFinite(actual) || !Number.isFinite(expected)) return false
    if (condition.op === "eq") return Math.abs(actual - expected) <= .000001
    if (condition.op === "neq") return Math.abs(actual - expected) > .000001
    if (condition.op === "lt") return actual < expected
    if (condition.op === "lte") return actual <= expected
    if (condition.op === "gt") return actual > expected
    if (condition.op === "gte") return actual >= expected
    return false
  })
}
function feedDetailLabels(feed: FishingPondFeed) {
  const labels = (feed.conditions || []).map(feedConditionLabel).filter((label): label is string => Boolean(label))
  if (feed.effects?.length && feed.delivery !== 'targeted') labels.push(t('game.fishing.pondFeedHourlyDose', { value: feed.maxPortionsPerFishHour || 0.5 }))
  for (const effect of feed.effects || []) {
    if (effect.stat === 'longevityDays' || effect.stat === 'naturalProtectionDays') { labels.push(t(effect.stat === 'longevityDays' ? 'game.fishing.pondCareLifeGain' : 'game.fishing.pondCareProtectionGain', { value: formatRealDuration(effect.valuePerPortion * 24 / (pond.value?.bioDaysPerRealDay || 365)) })); continue }
    if (effect.mode === "bonus_band" && effect.stat === "lengthCm") labels.push(t("game.fishing.pondFeedEffectLength", { value: feedBonusPercent(effect.maxTargetMultiplier) }))
    else if (effect.mode === "bonus_band" && effect.stat === "weightKg") labels.push(t("game.fishing.pondFeedEffectWeight", { value: feedBonusPercent(effect.maxTargetMultiplier) }))
    else if (effect.stat === "health") labels.push(`${t(effect.valuePerPortion > 0 ? 'game.fishing.pondFeedEffectHealthGain' : 'game.fishing.pondFeedEffectHealthLoss')} ${t('game.fishing.pondFeedPerPortion', { value: `${effect.valuePerPortion > 0 ? '+' : ''}${feedUnitPercent(effect.valuePerPortion)}%` })}`)
    else if (effect.stat === "satiety") labels.push(`${t(effect.valuePerPortion > 0 ? 'game.fishing.pondFeedEffectSatietyGain' : 'game.fishing.pondFeedEffectSatietyLoss')} ${t('game.fishing.pondFeedPerPortion', { value: `${effect.valuePerPortion > 0 ? '+' : ''}${effect.valuePerPortion}` })}`)
  }
  return [...new Set(labels)]
}
function feedConditionLabel(condition: FishingPondFeedCondition) {
  const op = ({ eq: "=", neq: "!=", lt: "<", lte: "<=", gt: ">", gte: ">=" } as Record<string, string>)[condition.op]
  if (!op) return ""
  if (condition.field === "dietClass") {
    if (condition.op === "eq" && condition.value === "herbal") return t("game.fishing.pondFeedConditionHerbalOnly")
    return typeof condition.value === "string" ? t("game.fishing.pondFeedConditionDiet", { op, value: dietLabel(condition.value) }) : ""
  }
  if (condition.field === "pondZone") {
    const zone = condition.value === "main" ? t("game.fishing.pondMainZone") : condition.value === "nursery" ? t("game.fishing.pondNurseryZone") : ""
    return zone ? t("game.fishing.pondFeedConditionZone", { op, value: zone }) : ""
  }
  const value = Number(condition.value)
  if (!Number.isFinite(value)) return ""
  if (condition.field === "health") return t("game.fishing.pondFeedConditionHealth", { op, value: feedUnitPercent(value) })
  if (condition.field === "satiety") return t("game.fishing.pondFeedConditionSatiety", { op, value: formatNumber(value) })
  if (condition.field === "condition") return t("game.fishing.pondFeedConditionBody", { op, value: feedUnitPercent(value) })
  return ""
}
function feedBonusPercent(multiplier?: number) { return Math.round(Math.max(0, Number(multiplier || 1) - 1) * 1000) / 10 }
function feedUnitPercent(value: number) { return Number((value * 100).toFixed(6)) }
function maxFeedAmount(feed: FishingPondOverview["feeds"][number]) { if (!pond.value || feed.nutrition <= 0) return 0; const remaining = Math.max(0, pond.value.limits.feederNutrition - pond.value.feedTotalNutrition); return Math.max(0, Math.min(feed.inventoryCount, 10_000, Math.floor((remaining + .0001) / feed.nutrition))) }
function canMoveToMain(row: FishingPondFish) { return Boolean(pond.value) && remainingFishCapacity.value > 0 && pond.value!.biomassKg + row.fish.weightKg <= pond.value!.limits.biomassKg + .0001 }
function moveToMainTooltip(row: FishingPondFish) { return canMoveToMain(row) ? t("game.fishing.pondMoveToMain") : t(remainingFishCapacity.value <= 0 ? "game.fishing.pondMainFull" : "game.fishing.pondMainBiomassFull") }
function sexLabel(sex: string) { return t(`game.fishing.sex_${sex === "female" ? "female" : "male"}`) }
function dietLabel(feedClass: string) { return t(`game.fishing.pondDiet_${feedClass}`) }
function phenotypeLabel(value: string) { return t(`game.fishing.pondPhenotype_${value}`) }
function phenotypeTooltip(row: FishingPondFish, trait: string) {
  const effect = (row.phenotypeEffects || []).find((entry) => entry.key === trait)
  if (!effect) return phenotypeLabel(trait)
  const value = Math.round(Number(effect.value || 0) * 1000) / 10
  return t(`game.fishing.pondPhenotypeEffect_${trait}`, { value })
}
function mutationRows(row: FishingPondFish) { return resolveFishMutations(row.fish, pond.value?.mutations) }
function mutationIds(row: FishingPondFish) { return fishMutationIds(row.fish) }
function mutationClasses(row: FishingPondFish) { return fishMutationClassNames(row.fish, pond.value?.mutations) }
function mutationTooltip(mutation: ResolvedFishMutation) {
  const details = mutation.definition
  if (!details) return t(mutation.descriptionKey)
  return `${t(mutation.descriptionKey)} · ${t("game.fishing.pondMutationEffects", {
    lifespan: Math.round((details.lifespanMultiplier - 1) * 100),
    alchemy: Math.round((details.alchemy.primaryValueMultiplier - 1) * 100)
  })}`
}
function maturityHoursRemaining(row: FishingPondFish) {
  const provided = Number(row.maturityRealHoursRemaining)
  if (Number.isFinite(provided) && provided >= 0) return provided
  const remainingBioDays = Math.max(0, Number(row.maturityAgeDays || 0) - Number(row.fish.ageDays || 0))
  return remainingBioDays / Math.max(1, Number(pond.value?.bioDaysPerRealDay || 365)) * 24
}
function maturityTooltip(row: FishingPondFish) {
  if (row.mature) return t("game.fishing.pondMaturityReadyTooltip", { age: formatAge(row.fish.ageDays) })
  const fallbackProgress = Number(row.fish.ageDays || 0) / Math.max(1, Number(row.maturityAgeDays || 1))
  const progress = clampPercent(Number.isFinite(Number(row.maturityProgress)) ? Number(row.maturityProgress) * 100 : fallbackProgress * 100)
  return t("game.fishing.pondMaturityTooltip", { progress, time: formatRealDuration(maturityHoursRemaining(row)) })
}
function predatorRule(row: FishingPondFish) { return row.predation ? t("game.fishing.pondPredatorRule", { ratio: Math.round(row.predation.maxPreyLengthRatio * 100), satiety: Math.round(row.predation.huntBelowSatiety), chance: Math.round(row.predation.dailyHuntChance * 100) }) : "" }
function formatNumber(value: number) { return Math.max(0, value || 0).toLocaleString(locale.value, { maximumFractionDigits: 0 }) }
function formatLength(value: number) { return `${Number(value || 0).toFixed(1)} cm` }
function formatWeight(value: number) { return `${Number(value || 0).toFixed(value < 1 ? 3 : 1)} kg` }
function formatAge(days: number) { if (days >= 365) return t("game.fishing.ageYearsMonths", { years: Math.floor(days / 365), months: Math.floor(days % 365 / 30) }); if (days >= 30) return t("game.fishing.ageMonths", { months: Math.floor(days / 30) }); return t("game.fishing.ageDays", { days }) }
function formatRealDuration(hours: number) { if (hours < 1) return t("game.fishing.pondRealUnderHour"); const totalHours = Math.ceil(hours); if (totalHours < 24) return t("game.fishing.pondRealHours", { hours: totalHours }); const days = Math.floor(totalHours / 24); const remainingHours = totalHours % 24; return remainingHours ? t("game.fishing.pondRealDaysHours", { days, hours: remainingHours }) : t("game.fishing.pondRealDays", { days }) }
function clampPercent(value: number) { return Math.max(0, Math.min(100, Math.round(value || 0))) }
function signedPercent(value: number) { const normalized = Math.round((value || 0) * 100); return `${normalized >= 0 ? "+" : ""}${normalized}%` }
function geneRows(row: FishingPondFish) { const genes = row.fish.genes || {}; return [["sizePotential", genes.sizePotential], ["growthRate", genes.growthRate], ["feedEfficiency", genes.feedEfficiency], ["vitality", genes.vitality], ["lifespan", genes.lifespan], ["fertility", genes.fertility]].map(([key, value]) => ({ key: String(key), value: Number(value || 0), label: t(`game.fishing.pondGene_${key}`) })) }
function formatEventTime(value: string) { return new Intl.DateTimeFormat(locale.value, { month: "short", day: "numeric", hour: "2-digit", minute: "2-digit" }).format(new Date(value)) }
function eventIcon(kind: string) { if (kind === "predated") return "lucide:utensils"; if (kind === "born") return "lucide:sparkles"; if (kind === "fed") return "lucide:wheat"; if (kind === "processed") return "lucide:recycle"; if (kind === "pair_set") return "lucide:heart"; if (kind === "genome_revealed") return "lucide:dna"; if (kind === "tournament_submitted") return "lucide:trophy"; if (kind === "moved_to_main") return "lucide:move-right"; if (kind === "removed") return "lucide:package-open"; return "lucide:fish" }
function eventLabel(event: FishingPondEvent) {
  if (event.kind === 'feed_managed') return `${t('game.fishing.pondFeedManaged')} · ${itemName(String(event.details?.itemId || ''))}`
  if (event.kind === "predated") {
    const predatorSpeciesId = typeof event.details?.predatorSpeciesId === "string" ? event.details.predatorSpeciesId : ""
    const preySpeciesId = typeof event.details?.preySpeciesId === "string" ? event.details.preySpeciesId : ""
    const predator = typeof event.details?.predatorName === "string" && event.details.predatorName.trim() ? event.details.predatorName.trim() : speciesName(predatorSpeciesId)
    const prey = typeof event.details?.preyName === "string" && event.details.preyName.trim() ? event.details.preyName.trim() : speciesName(preySpeciesId)
    return t("game.fishing.pondPredationEventLine", { predator, prey })
  }
  const namedFish = event.fishInstanceId ? pondFishById.value.get(event.fishInstanceId) : null
  const speciesId = typeof event.details?.speciesId === "string" ? event.details.speciesId : ""
  const base = t(`game.fishing.pondEvent_${event.kind}`)
  return namedFish ? `${base} · ${displayFishName(namedFish)}` : speciesId ? `${base} · ${speciesName(speciesId)}` : base
}

defineExpose({ loadPond })
</script>

<style scoped>
.pond-feed-picker.pond-care-picker { grid-template-columns: minmax(0, 1fr); margin-bottom: 12px; }
.pond-feed-controls { display: flex; gap: 6px; margin-top: 4px; }
.pond-feed-controls .el-button { margin-left: 0; }
.pond-panel { min-height: 360px; color: var(--app-text); }
.pond-loading, .pond-error, .pond-unbuilt { min-height: 340px; display: flex; align-items: center; justify-content: center; gap: 16px; }
.pond-loading svg { animation: pond-spin 1s linear infinite; }
.pond-unbuilt { justify-content: flex-start; padding: clamp(28px, 8vw, 90px); border: 1px dashed var(--el-border-color); border-radius: 8px; }
.pond-unbuilt > span { display: grid; place-items: center; width: 68px; aspect-ratio: 1; border-radius: 50%; background: #e8f4ef; color: #287d68; font-size: 30px; }
.pond-unbuilt div { flex: 1; }.pond-unbuilt h2, .pond-unbuilt p { margin: 0; }.pond-unbuilt p { margin-top: 7px; color: var(--app-text-muted); }
.pond-status { display: grid; grid-template-columns: .7fr 1fr 1fr 1.35fr 1.2fr; border-bottom: 1px solid var(--el-border-color); }
.pond-status > div { min-width: 0; display: grid; grid-template-columns: auto 1fr; grid-template-rows: auto auto; gap: 2px 8px; padding: 10px 12px; border-right: 1px solid var(--el-border-color); }
.pond-status > div:last-child { border-right: 0; }.pond-status svg { grid-row: 1 / 3; align-self: center; color: #287d68; font-size: 22px; }.pond-status span { color: var(--app-text-muted); font-size: 12px; }.pond-status strong { font-size: 15px; }
.pond-layout { display: grid; grid-template-columns: minmax(0, 1fr) minmax(280px, 310px); gap: 18px; padding-top: 14px; }.pond-main, .pond-sidebar { min-width: 0; }
.pond-zone-toolbar { display: flex; align-items: center; justify-content: space-between; gap: 12px; min-height: 36px; margin-bottom: 10px; }.pond-zone-toolbar :deep(.el-segmented) { width: min(100%, 440px); }.pond-zone-toolbar :deep(.el-segmented__group) { width: 100%; }.pond-zone-toolbar :deep(.el-segmented__item) { min-width: 0; flex: 1; }.pond-zone-toolbar > span, .pond-shared-feed, .pond-nursery-full { display: flex; align-items: center; gap: 5px; color: var(--app-text-muted); font-size: 11px; }.pond-zone-toolbar > span svg, .pond-shared-feed svg { flex: none; color: #287d68; }
.pond-section-head, .pond-sidebar section > header { display: flex; align-items: center; justify-content: space-between; gap: 8px; margin: 14px 0 8px; }.pond-section-head span, .pond-sidebar header span { color: #287d68; font-size: 12px; font-weight: 700; }.pond-section-head h2, .pond-sidebar header h2 { margin: 0; font-size: 14px; }.pond-section-head > div { display: flex; align-items: baseline; gap: 8px; }
.pond-fish-list { display: grid; grid-template-columns: repeat(auto-fit, minmax(min(100%, 310px), 1fr)); gap: 8px; align-items: start; }
.pond-fish-list > article { position: relative; min-width: 0; display: grid; grid-template-columns: 48px minmax(0, 1fr); gap: 6px 9px; align-items: start; overflow: hidden; padding: 10px; border: 1px solid var(--el-border-color); border-radius: 6px; background: var(--el-fill-color-blank); isolation: isolate; }
.pond-fish-list > article.is-dead { background: var(--el-color-danger-light-9); }
.pond-fish-list > article > .fish-mutation-art { height: 42px; }
.pond-fish-list > article.has-fish-mutation { border-color: color-mix(in srgb, #68bed2 52%, var(--el-border-color)); }
.pond-fish-list > article.fish-mutation--prismatic { box-shadow: inset 3px 0 0 rgb(94 209 226 / 72%); }
.pond-fish-list > article.fish-mutation--rainbow::before { content: ""; position: absolute; z-index: -1; inset: 0 0 auto; height: 3px; background: linear-gradient(90deg, #f16b7d, #f1c453, #55bd8a, #559bd8, #ad75d4); opacity: .76; }
.pond-fish-list > article.fish-mutation--shimmering { box-shadow: inset 0 0 0 1px rgb(246 203 94 / 44%); }
.pond-fish-list > article.has-stacked-fish-mutations { background: color-mix(in srgb, var(--el-fill-color-blank) 95%, #d8f5f4); }
.pond-fish-copy { min-width: 0; }
.pond-fish-copy > small { display: block; color: var(--app-text-muted); font-size: 11px; line-height: 1.6; overflow-wrap: anywhere; }
.pond-fish-identity { display: flex; flex-wrap: wrap; align-items: baseline; gap: 2px 8px; }
.pond-fish-identity > span { color: var(--app-text-muted); font-size: 10px; }
.pond-fish-copy h3 { min-width: 0; margin: 0; font-size: 14px; line-height: 1.5; overflow-wrap: anywhere; }
.pond-fish-copy :deep(.rarity-text) { font-size: inherit; }
.pond-fish-bars { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 10px; margin-top: 6px; }
.pond-fish-bars label { display: grid; grid-template-columns: auto minmax(20px, 1fr) auto; align-items: center; gap: 4px; }
.pond-fish-bars label > span, .pond-fish-bars label > small { color: var(--app-text-muted); font-size: 10px; font-variant-numeric: tabular-nums; }
.pond-fish-tags { display: flex; flex-wrap: wrap; gap: 4px; margin-top: 6px; }
.pond-fish-tags :deep(.el-tag) { height: auto; min-height: 20px; padding: 0 5px; font-size: 10px; white-space: normal; }
.pond-fish-tags :deep(.el-tag__content) { display: inline-flex; align-items: center; gap: 3px; min-width: 0; overflow-wrap: anywhere; }
.pond-fish-tags svg { flex: none; font-size: 12px; }
.pond-genome-details { margin-top: 7px; }
.pond-genome-details summary { display: flex; align-items: center; gap: 5px; width: fit-content; min-height: 28px; color: var(--app-text-muted); font-size: 11px; cursor: pointer; list-style: none; }
.pond-genome-details summary::-webkit-details-marker { display: none; }
.pond-genome-details summary:focus-visible { outline: 2px solid var(--app-primary); outline-offset: 2px; }
.pond-genome-details[open] summary > svg:last-child { transform: rotate(180deg); }
.pond-genes { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 5px 12px; margin: 10px 0 0; padding-top: 9px; border-top: 1px solid var(--el-border-color-lighter); }.pond-genes div { display: flex; justify-content: space-between; gap: 6px; font-size: 11px; }.pond-genes dt { color: var(--app-text-muted); white-space: nowrap; }.pond-genes dd { margin: 0; font-weight: 700; }.pond-genes .is-positive { color: #287d68; }.pond-genes .is-negative { color: var(--el-color-danger); }.pond-genes .pond-genes__mutations { grid-column: 1 / -1; padding-top: 4px; border-top: 1px dashed var(--el-border-color-lighter); }.pond-genes__mutations dd { color: #277d87; text-align: right; }
.pond-mutation-tag { border-color: color-mix(in srgb, #4daab6 45%, var(--el-border-color)) !important; color: #257985 !important; }.pond-mutation-tag :deep(svg) { margin-right: 3px; }.pond-mutation-tag.is-rainbow { border-color: color-mix(in srgb, #d26d9a 48%, var(--el-border-color)) !important; color: #a44576 !important; }.pond-mutation-tag.is-shimmering { border-color: color-mix(in srgb, #d3a52a 52%, var(--el-border-color)) !important; color: #9a7111 !important; }
.pond-fish-actions { grid-column: 1 / -1; display: flex; flex-wrap: wrap; align-items: center; justify-content: flex-end; gap: 6px; padding-top: 6px; border-top: 1px solid var(--el-border-color-lighter); }
.pond-fish-actions :deep(.el-button) { width: 28px; height: 28px; padding: 0; margin: 0; }
.pond-resource-action { display: flex; align-items: center; gap: 4px; }
.pond-resource-action small, .pond-pair-actions small { display: flex; align-items: center; gap: 2px; color: var(--app-text-muted); font-size: 9px; }
.pond-empty { display: grid; place-items: center; min-height: 150px; color: var(--app-text-muted); }.pond-empty svg { font-size: 28px; }
.pond-feed-section, .pond-breeding-section, .pond-events { padding-bottom: 16px; border-bottom: 1px solid var(--el-border-color); }.pond-sidebar section:first-child > header { margin-top: 0; }.pond-feed-meter { display: grid; grid-template-columns: 1fr auto; align-items: center; gap: 8px; }.pond-feed-meter strong { font-size: 12px; }
.pond-shared-feed, .pond-nursery-full { margin: -4px 0 9px; }.pond-nursery-full { align-items: flex-start; padding: 7px 8px; border-left: 3px solid var(--el-color-warning); background: var(--el-color-warning-light-9); color: var(--el-color-warning-dark-2); }.pond-nursery-full svg { flex: none; margin-top: 1px; }
.pond-feed-list, .pond-pairs { display: grid; gap: 8px; margin-top: 12px; }.pond-feed-list > article { display: grid; grid-template-columns: 50px minmax(0, 1fr); gap: 8px; align-items: center; }.pond-feed-list .pond-feed-copy { min-width: 0; display: grid; gap: 2px; }.pond-feed-list strong { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; font-size: 13px; }.pond-feed-list small, .pond-feed-list .pond-feed-copy > span { color: var(--app-text-muted); font-size: 10px; }
.pond-feed-traits { display: flex; flex-wrap: wrap; gap: 3px; margin: 2px 0; }.pond-feed-traits > span { width: fit-content; padding: 1px 5px; border: 1px solid color-mix(in srgb, #287d68 36%, var(--el-border-color)); border-radius: 4px; color: #287d68; font-size: 9px; line-height: 1.45; white-space: normal; }.pond-feed-traits > span.is-danger { border-color: color-mix(in srgb, var(--el-color-danger) 42%, var(--el-border-color)); color: var(--el-color-danger); }.pond-feed-eligibility { color: var(--el-color-warning-dark-2); font-size: 10px; }
.pond-pairs > article { display: grid; grid-template-columns: 1fr auto 1fr; gap: 7px; align-items: center; padding: 9px; border: 1px solid var(--el-border-color); border-radius: 6px; }.pond-pair-fish { min-width: 0; display: flex; align-items: center; gap: 5px; }.pond-pair-fish .fish-mutation-art { width: 34px; height: 28px; }.pond-pair-fish span { min-width: 0; display: grid; }.pond-pair-fish strong { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; font-size: 11px; }.pond-pair-heart { color: #d55d77; }.pond-pair-actions { grid-column: 1 / -1; display: flex; align-items: center; justify-content: flex-end; gap: 6px; padding-top: 6px; border-top: 1px solid var(--el-border-color-lighter); }
.pond-section-empty { color: var(--app-text-muted); font-size: 12px; }.pond-events ol { list-style: none; display: grid; gap: 8px; max-height: 230px; overflow-y: auto; scrollbar-width: thin; margin: 10px 0 0; padding: 0 6px 0 0; }.pond-events li { display: flex; gap: 8px; align-items: center; }.pond-events li > svg { flex: none; color: #287d68; }.pond-events li span { min-width: 0; display: grid; overflow-wrap: anywhere; }.pond-events li strong { font-size: 11px; }.pond-events li small { color: var(--app-text-muted); font-size: 10px; }
.pond-dialog-note { display: flex; gap: 7px; align-items: flex-start; margin: 0 0 12px; color: var(--app-text-muted); font-size: 12px; line-height: 1.5; }.pond-dialog-note svg { flex: 0 0 auto; margin-top: 2px; color: #287d68; }
.pond-feed-picker { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 8px; max-height: 48vh; overflow: auto; }.pond-feed-picker button { display: grid; grid-template-columns: 58px minmax(0, 1fr) auto; align-items: center; gap: 10px; min-width: 0; padding: 9px; border: 1px solid var(--el-border-color); border-radius: 6px; background: var(--el-fill-color-blank); color: inherit; text-align: left; cursor: pointer; }.pond-feed-picker button.is-selected { border-color: #287d68; box-shadow: 0 0 0 1px #287d68; }.pond-feed-picker button > span { min-width: 0; display: grid; gap: 2px; }.pond-feed-picker strong, .pond-feed-picker small, .pond-feed-picker em { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }.pond-feed-picker small { color: var(--app-text-muted); }.pond-feed-picker em { color: #287d68; font-size: 11px; font-style: normal; }.pond-feed-picker button > svg { color: #287d68; font-size: 20px; }.pond-feed-quantity { display: flex; align-items: center; justify-content: flex-end; gap: 12px; margin-top: 14px; }.pond-feed-missing { min-height: 180px; display: grid; place-content: center; justify-items: center; gap: 9px; color: var(--app-text-muted); }.pond-feed-missing svg { font-size: 28px; }
.pond-pair-form { display: grid; grid-template-columns: minmax(0, 1fr) auto minmax(0, 1fr); align-items: center; gap: 12px; }.pond-pair-form > svg { color: #d55d77; font-size: 24px; }.pond-pair-form > button { display: grid; grid-template-columns: auto minmax(0, 1fr); align-items: center; gap: 10px; min-height: 108px; padding: 12px; border: 1px dashed var(--el-border-color); border-radius: 6px; background: var(--el-fill-color-lighter); color: inherit; text-align: left; cursor: pointer; }.pond-pair-form > button:disabled { opacity: .52; cursor: not-allowed; }.pond-pair-form > button.has-fish { border-style: solid; background: var(--el-fill-color-blank); }.pond-pair-form button > .fish-mutation-art { width: 76px; height: 64px; }.pond-pair-form button > svg { font-size: 30px; color: #287d68; }.pond-pair-form button > span { min-width: 0; display: grid; }.pond-pair-form button strong, .pond-pair-form button small, .pond-pair-form button em { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }.pond-pair-form button small, .pond-pair-form button em { color: var(--app-text-muted); font-size: 11px; font-style: normal; }
@keyframes pond-spin { to { transform: rotate(360deg); } }
@media (max-width: 980px) { .pond-layout { grid-template-columns: 1fr; }.pond-sidebar { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 18px; }.pond-events { grid-column: 1 / -1; } }
@media (max-width: 720px) { .pond-status { grid-template-columns: repeat(2, minmax(0, 1fr)); }.pond-status > div:nth-child(2n) { border-right: 0; }.pond-status > div:nth-child(-n + 4) { border-bottom: 1px solid var(--el-border-color-lighter); }.pond-status > div:last-child { grid-column: 1 / -1; }.pond-layout { padding-top: 10px; }.pond-zone-toolbar { align-items: stretch; flex-direction: column; }.pond-zone-toolbar :deep(.el-segmented) { width: 100%; }.pond-sidebar { grid-template-columns: 1fr; }.pond-events { grid-column: auto; }.pond-fish-list { grid-template-columns: 1fr; }.pond-fish-list > article { grid-template-columns: 52px minmax(0, 1fr); }.pond-fish-list > article > .fish-mutation-art { width: 52px !important; }.pond-fish-actions { grid-column: 1 / -1; flex-direction: row; justify-content: flex-end; }.pond-genes { grid-template-columns: repeat(2, minmax(0, 1fr)); }.pond-unbuilt { flex-wrap: wrap; padding: 28px 18px; }.pond-feed-picker { grid-template-columns: 1fr; max-height: calc(100dvh - 250px); }.pond-pair-form { grid-template-columns: 1fr; }.pond-pair-form > svg { justify-self: center; }.pond-pair-form > button { min-height: 94px; } }
@media (max-width: 720px) {
  .pond-fish-actions :deep(.el-button) { width: 36px; height: 36px; }
  .pond-genome-details summary { min-height: 36px; }
  .pond-fish-bars label { grid-template-columns: 1fr auto; }
  .pond-fish-bars .el-progress { grid-column: 1 / -1; grid-row: 2; }
  .pond-status strong { overflow-wrap: anywhere; }
}
</style>
