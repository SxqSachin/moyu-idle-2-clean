import { apiDelete, apiGet, apiPost, createIdempotencyKey } from "@services/api"

export type FishingMethod = "float" | "bottom" | "lure"
export type FishingStrategy = "conservative" | "balanced" | "aggressive"
export type FishingTargetLayer = "auto" | "surface" | "midwater" | "near_bottom"
export type FishingPace = "patient" | "balanced" | "search"
export type FishingKeepMode = "keep_all" | "release_all" | "record_only" | "min_weight"

export const FISHING_SALE_BATCH_MAX = 100

export type LocalizedFishingText = Record<string, string>

export type FishingWater = {
  id: string
  order: number
  requiredFishingLevel: number
  names: LocalizedFishingText
  descriptions: LocalizedFishingText
  baseTemperatureC: number
  waterKind: string
}

export type FishingSpot = {
  id: string
  waterId: string
  order: number
  names: LocalizedFishingText
  habitat: string
  depthM: number
  oxygen: number
  baseBiteRate: number
  methods: FishingMethod[]
  speciesWeights: Record<string, number>
}

export type FishingSpecies = {
  id: string
  waterId: string
  rarity: string
  fantasy: boolean
  names: LocalizedFishingText
  minLengthCm: number
  maxLengthCm: number
  maturityLengthCm: number
  maturityAgeDays: number
  lifespanDays: number
  shapeFactor: number
  baseValue: number
  fightPower: number
  castingChallenge: number
  hookingChallenge: number
  mainLineWearMultiplier: number
  leaderWearMultiplier: number
  preferredMethods: FishingMethod[]
  preferredLayers: Exclude<FishingTargetLayer, "auto">[]
  depth: string
  activeHours: number[]
  windowWeather?: string
  iconPath: string
}

export type FishingPondDietClass = "herbal" | "insect" | "protein" | "arcane"

export type FishingPondPredation = {
  maxPreyLengthRatio: number
  huntBelowSatiety: number
  dailyHuntChance: number
}

export type FishingPondSpeciesProfile = {
  id: string
  speciesId: string
  dietClass: FishingPondDietClass
  secondaryFeedClasses: FishingPondDietClass[]
  maturityAgeDays: number
  lifespanDays: number
  predation?: FishingPondPredation
}

export type FishingMutationDefinition = {
  id: string
  nameKey: string
  descriptionKey: string
  visualEffect: string
  badgeIcon: string
  appearanceChance: number
  inheritanceChance: number
  lifespanMultiplier: number
  saleValueMultiplier: number
  alchemy: {
    primaryValueMultiplier: number
    bonusAttributes: Array<{ attrKey: string; min: number; max: number }>
  }
}

export type FishingTackleDefinition = {
  itemId: string
  slot: string
  methods: FishingMethod[]
  control?: number
  sensitivity?: number
  casting?: number
  hooking?: number
  efficiency?: number
  maxLoadKg?: number
  wearPerFight?: number
  biteMultiplier?: number
  mutationChanceMultiplier?: number
  waterBiteModifiers?: Record<string, number>
  speciesWeightModifiers?: Record<string, number>
  terminal?: boolean
  tournamentReward?: boolean
}

export type FishingOwnedTackle = {
  itemId: string
  slot: string
  quantity: number
  instanceId?: number
  enhanceLevel?: number
  durability?: number
  maxDurability?: number
}

export type FishingConditions = {
  gameHour: number
  season: string
  weather: string
  temperatureC: number
  oxygen: number
  activity: number
  activityBand: "busy" | "steady" | "quiet"
  trend: "rising" | "steady" | "falling"
}

export type FishingSpotForecast = {
  spotId: string
  pressure: number
  now: FishingConditions
  next: FishingConditions[]
}

export type FishingLoadoutComponents = {
  rodInstanceId: number
  reelInstanceId: number
  mainLineItemId: string
  leaderItemId: string
  hookItemId?: string
  floatOrSinkerItemId?: string
  lureItemId?: string
  baitItemId?: string
}

export type FishingLoadout = {
  loadoutId: string
  name: string
  method: FishingMethod
  strategy: FishingStrategy
  safetyLoadPercent: number
  components: FishingLoadoutComponents
  updatedAt?: string
}

export type FishIndividual = {
  speciesId: string
  ageDays: number
  sex: "male" | "female"
  lengthCm: number
  weightKg: number
  condition: number
  health: number
  waterId: string
  spotId: string
  caughtAt: string
  rulesetVersion: string
  recordFlags?: string[]
  origin?: "wild" | "cultured"
  pondZone?: FishingPondZone
  lifeState?: "alive" | "dead"
  generation?: number
  mutations?: string[]
  parentInstanceIds?: number[]
  genes?: FishGenes
  genomeRevealed?: boolean
  satiety?: number
  preferredFeedHours?: number
  starvationHours?: number
  pondEnteredAt?: string
  bornAt?: string
  lastBredAt?: string
  deathAt?: string
  deathCause?: string
}

export type FishingPondZone = "main" | "nursery"

export function fishingPondZone(fish: Pick<FishIndividual, "pondZone">): FishingPondZone {
  return fish.pondZone === "nursery" ? "nursery" : "main"
}

export type FishGenes = {
  sizePotential?: number
  growthRate?: number
  feedEfficiency?: number
  vitality?: number
  lifespan?: number
  fertility?: number
  mutations?: string[]
}

export type FishingPondLimits = {
  fishCapacity: number
  nurseryCapacity: number
  biomassKg: number
  breedingPairs: number
  feederNutrition: number
}

export type FishingPondVisibleIndividual = Pick<FishIndividual,
  | "speciesId"
  | "ageDays"
  | "sex"
  | "lengthCm"
  | "weightKg"
  | "pondZone"
  | "lifeState"
  | "generation"
  | "mutations"
> & Partial<Pick<FishIndividual, "health" | "satiety">>

export type FishingPondVisibleFish = {
  instanceId: number
  customData?: {
    name?: string
    labelColor?: string
  }
  fish: FishingPondVisibleIndividual
  dietClass?: FishingPondDietClass
  secondaryFeedClasses?: FishingPondDietClass[]
  maturityAgeDays?: number
  expectedLifespanDays?: number
  mature?: boolean
  maturityProgress?: number
  maturityRealHoursRemaining?: number
  phenotypes?: string[]
  phenotypeEffects?: FishingPondPhenotypeEffect[]
  predation?: FishingPondPredation
}

export type FishingPondFish = FishingPondVisibleFish & {
  careRevision?: number
  naturalProtectionHours?: number
  fish: FishIndividual
  dietClass: FishingPondDietClass
  maturityAgeDays: number
  expectedLifespanDays: number
  mature: boolean
}

export type FishingPondPhenotypeEffect = {
  key: string
  value: number
}

export type FishingPondFeed = {
  delivery?: "feeder" | "targeted"
  role?: "staple" | "functional"
  maxPortionsPerFishHour?: number
  storedPortions?: number
  paused?: boolean
  itemId: string
  feedClass: string
  nutrition: number
  rulesetVersion?: string
  consumePriority?: number
  conditions?: FishingPondFeedCondition[]
  effects?: FishingPondFeedEffect[]
  inventoryCount: number
  storedNutrition: number
}

export type FishingPondFeedCondition = {
  field: "dietClass" | "pondZone" | "health" | "satiety" | "condition" | string
  op: "eq" | "neq" | "lt" | "lte" | "gt" | "gte" | string
  value: string | number
}

export type FishingPondFeedEffect = {
  kind: "stat_delta" | string
  stat: "lengthCm" | "weightKg" | "health" | "satiety" | string
  mode: "bonus_band" | "flat" | string
  valuePerPortion: number
  maxTargetMultiplier?: number
  floor?: number
  ceiling?: number
}

export type FishingPondPair = {
  slotIndex: number
  femaleInstanceId: number
  maleInstanceId: number
  mashReady: boolean
}

export type FishingPondEvent = {
  id: number
  kind: string
  fishInstanceId?: number
  relatedInstanceId?: number
  details?: Record<string, unknown>
  createdAt: string
}

export type FishingPondOverview = {
  built: boolean
  level: number
  rulesetVersion: string
  bioDaysPerRealDay: number
  maxOfflineHours: number
  limits: FishingPondLimits
  fish: FishingPondFish[]
  availableFish: FishingPondFish[]
  availableFishNextCursor?: number
  pairs: FishingPondPair[]
  feeds: FishingPondFeed[]
  feedTotalNutrition: number
  biomassKg: number
  events: FishingPondEvent[]
  lineageReagentCount: number
  broodstockMashCount: number
  lastSettledAt: string
  mutations: FishingMutationDefinition[]
}

export type FishingPondPublicOverview = {
  ownerUserId: number
  built: boolean
  level: number
  rulesetVersion: string
  limits: Pick<FishingPondLimits, "fishCapacity" | "nurseryCapacity" | "biomassKg">
  fish: FishingPondVisibleFish[]
  biomassKg: number
  capabilities: string[]
  mutations: FishingMutationDefinition[]
}

export type FishingRecordFlag = {
  speciesId: string
  length: boolean
  weight: boolean
}

export type FishingCatch = {
  catchId: number
  tripId: string
  speciesId: string
  fish: FishIndividual
  kept: boolean
  inventoryInstanceId?: number
  saleEligible?: boolean
  saleIneligibleReason?: "cultured_immature" | "dead" | "invalid_data"
  saleCurrentAgeDays?: number
  saleRequiredAgeDays?: number
  caughtAt: string
  record: FishingRecordFlag
}

export type FishingTripSummary = {
  casts: number
  bites: number
  landed: number
  kept: number
  released: number
  escaped: number
  lineBreaks: number
  baitUsed: number
  scaleglowFound: number
  totalWeightKg: number
  fishingExp: number
  newSpecies?: string[]
  records?: FishingRecordFlag[]
  lastCatch?: FishingCatch
  recentEvents?: FishingTripEvent[]
  stopReason?: string
}

export type FishingTripEvent = {
  sequence: number
  kind: "cast" | "quiet" | "bite" | "hooked" | "landed" | "escaped" | "line_break" | "finished"
  reason?: "hook_slipped" | "lost_control" | "line_overload" | "line_break" | "time_reached" | "bait_empty" | "tackle_empty" | string
  at: string
  castNumber?: number
  sizeClass?: "small" | "medium" | "large"
  speciesId?: string
  lengthCm?: number
  weightKg?: number
  mutations?: string[]
  disposition?: "kept" | "released"
}

export type FishingSupplyCounts = {
  mainLine: number
  leader: number
  hook?: number
  floatOrSinker?: number
  lure?: number
  bait?: number
}

export type FishingTripSupplies = {
  initial: FishingSupplyCounts
  remaining: FishingSupplyCounts
  consumed: FishingSupplyCounts
  wear?: {
    mainLine?: number
    leader?: number
  }
  returned?: boolean
}

export type FishingTrip = {
  tripId: string
  queueId: number
  actorType: "player" | "kitty"
  actorId: number
  waterId: string
  spotId: string
  loadoutId: string
  strategy: FishingStrategy
  safetyLoadPercent: number
  status: "running" | "completed" | "stopped"
  phase: "waiting" | "bite" | "fighting" | "landed" | "resting" | "finished"
  phaseStartedAt: string
  phaseEndsAt: string
  plannedEndAt?: string
  currentFish?: {
    sizeClass?: "small" | "medium" | "large"
    stamina?: number
    lineLoadKg?: number
    lineState?: "steady" | "tight" | "critical"
  }
  summary: FishingTripSummary
  supplies: FishingTripSupplies
  stopReason?: string
  startedAt: string
  endedAt?: string
}

export type FishingAngler = {
  actorType: "player" | "kitty"
  actorId: number
  name?: string
  status: string
  fishingLevel: number
}

export type FishingOverview = {
  level: number
  anglers: FishingAngler[]
  waters: FishingWater[]
  spots: FishingSpot[]
  species: FishingSpecies[]
  pondSpecies: FishingPondSpeciesProfile[]
  mutations: FishingMutationDefinition[]
  tackle: FishingTackleDefinition[]
  ownedTackle: FishingOwnedTackle[]
  forecast: FishingSpotForecast[]
  currentTrip: FishingTrip | null
  latestTrip: FishingTrip | null
  spotCounts: Record<string, number>
  rulesetVersion: string
}

export type FishingTripHistory = {
  items: FishingTrip[]
  limit: number
}

export type FishingTripEventPage = {
  items: FishingTripEvent[]
  total: number
  nextCursor?: number
  historyComplete: boolean
}

export function listCompatibleFishingTackleItemIds(
  tackle: readonly FishingTackleDefinition[] | null | undefined,
  slot: string,
  method: FishingMethod
): string[] {
  const normalizedSlot = String(slot || "").trim()
  if (!normalizedSlot) return []

  const itemIds: string[] = []
  const seen = new Set<string>()
  for (const definition of tackle || []) {
    if (String(definition.slot || "").trim() !== normalizedSlot) continue
    if (!(definition.methods || []).some((candidate) => String(candidate || "").trim() === method)) continue
    const itemId = String(definition.itemId || "").trim()
    if (!itemId || seen.has(itemId)) continue
    seen.add(itemId)
    itemIds.push(itemId)
  }
  return itemIds
}

export type FishingStarterGrant = {
  granted: boolean
  grantKey: string
  loadoutId: string
  items: Record<string, number>
}

export type FishingKnowledge = {
  speciesId: string
  encounters: number
  catches: number
  releases: number
  firstCaughtAt?: string
  lastCaughtAt?: string
  bestLengthCm: number
  bestWeightKg: number
}

export type FishingLeaderboardRow = {
  rank: number
  userId: number
  playerName: string
  speciesId: string
  value: number
}

export type FishingSaleLine = {
  instanceId: number
  speciesId: string
  ageDays: number
  lengthCm: number
  weightKg: number
  mutations?: string[]
  baseValue: number
  ageFactor: number
  lengthFactor: number
  weightFactor: number
  qualityFactor: number
  specimenMultiplier: number
  mutationMultiplier: number
  dailyDemand: number
  supplyFactor: number
  cultureFactor: number
  marketMultiplier: number
  finalGold: number
}

export type FishingSaleQuote = {
  quoteAt: string
  marketDate: string
  items: FishingSaleLine[]
  totalGold: number
}

export type FishingTournamentEntry = {
  rank: number
  division: "open" | "standard"
  userId: number
  playerName: string
  catchId?: number
  fishInstanceId?: number
  sourceKind: "catch" | "cultured"
  lengthCm: number
  weightKg: number
}

export type FishingTournamentReward = {
  tournamentId: string
  division: "open" | "standard"
  rank: number
  tokenAmount: number
  itemId?: string
  cosmeticKey?: string
  claimedAt?: string
}

export type FishingTournamentStandardRules = {
  fishingLevel: number
  strategy: FishingStrategy
  control: number
  sensitivity: number
  hooking: number
  maxLoadKg: number
}

export type FishingTournamentStandardTackleItem = {
  slot: string
  itemId: string
}

export type FishingTournamentStandardLoadout = {
  method: FishingMethod
  items: FishingTournamentStandardTackleItem[]
}

export type FishingTournamentOverview = {
  current: {
    id: string
    weekKey: string
    targetSpeciesId: string
    rulesetVersion: string
    startsAt: string
    endsAt: string
    status: string
  }
  standardRules?: FishingTournamentStandardRules
  standardLoadout?: FishingTournamentStandardLoadout
  openLeaderboard: FishingTournamentEntry[]
  standardLeaderboard: FishingTournamentEntry[]
  myEntries: FishingTournamentEntry[]
  rewards: FishingTournamentReward[]
  tokens: number
  catchUpAvailable: boolean
  catchUpCost: number
}

export function localizedFishingText(value: LocalizedFishingText | undefined, locale: string): string {
  if (!value) return ""
  return value[locale] || value[locale === "zh-TW" ? "zh-CN" : locale] || value["en-US"] || Object.values(value)[0] || ""
}

export async function fetchFishingOverview(token: string): Promise<FishingOverview> {
  return apiGet<FishingOverview>("/api/fishing/overview", token)
}

export async function fetchFishingTripHistory(token: string): Promise<FishingTripHistory> {
  return apiGet<FishingTripHistory>("/api/fishing/trips/history", token)
}

export async function fetchFishingTripEvents(token: string, tripId: string, cursor = 0): Promise<FishingTripEventPage> {
  const query = cursor > 0 ? `?cursor=${encodeURIComponent(String(cursor))}` : ""
  return apiGet<FishingTripEventPage>(`/api/fishing/trips/${encodeURIComponent(tripId)}/events${query}`, token)
}

export async function claimFishingStarter(token: string): Promise<FishingStarterGrant> {
  return apiPost<FishingStarterGrant>("/api/fishing/starter/claim", token, {})
}

export async function fetchFishingLoadouts(token: string): Promise<FishingLoadout[]> {
  const payload = await apiGet<{ items: FishingLoadout[] }>("/api/fishing/loadouts", token)
  return payload.items || []
}

export async function saveFishingLoadout(token: string, loadout: FishingLoadout): Promise<FishingLoadout> {
  return apiPost<FishingLoadout>("/api/fishing/loadouts", token, loadout)
}

export async function deleteFishingLoadout(token: string, loadoutId: string): Promise<void> {
  return apiDelete(`/api/fishing/loadouts/${encodeURIComponent(loadoutId)}`, token)
}

export async function enqueueFishingTrip(token: string, input: {
  spotId: string
  loadoutId: string
  tournamentDivision?: "open" | "standard"
  durationSeconds: number
  strategy: FishingStrategy
  targetLayer: FishingTargetLayer
  pace: FishingPace
  safetyLoadPercent?: number
  keepRule: { mode: FishingKeepMode; minWeightKg?: number }
  supplies: FishingSupplyCounts
}, actor: { actorType: "player" | "kitty"; actorId: number } = { actorType: "player", actorId: 0 }): Promise<void> {
  await apiPost("/api/actions/queue/batch", token, {
    steps: [{ actionId: "fishing.trip", count: 1, args: input }],
    actorType: actor.actorType,
    actorId: actor.actorType === "kitty" ? actor.actorId : 0,
    insertMode: "append"
  })
}

export async function stopFishingTrip(token: string): Promise<void> {
  await apiPost("/api/fishing/trips/stop", token, {})
}

export async function fetchFishingCatches(token: string, options: { cursor?: number; limit?: number; speciesId?: string; kept?: boolean } = {}): Promise<{ items: FishingCatch[]; nextCursor?: number }> {
  const query = new URLSearchParams()
  if (options.cursor) query.set("cursor", String(options.cursor))
  if (options.limit) query.set("limit", String(options.limit))
  if (options.speciesId) query.set("speciesId", options.speciesId)
  if (options.kept) query.set("kept", "true")
  return apiGet(`/api/fishing/catches?${query.toString()}`, token)
}

export async function fetchFishingJournal(token: string): Promise<FishingKnowledge[]> {
  const payload = await apiGet<{ items: FishingKnowledge[] }>("/api/fishing/journal", token)
  return payload.items || []
}

export async function fetchFishingLeaderboard(token: string, speciesId: string, metric: "length" | "weight"): Promise<FishingLeaderboardRow[]> {
  const query = new URLSearchParams({ speciesId, metric, limit: "50" })
  const payload = await apiGet<{ items: FishingLeaderboardRow[] }>(`/api/fishing/leaderboard?${query.toString()}`, token)
  return payload.items || []
}

export async function quoteFishingSale(token: string, instanceIds: number[]): Promise<FishingSaleQuote> {
  return apiPost("/api/fishing/quote", token, { instanceIds })
}

export async function sellFishingCatches(token: string, instanceIds: number[], expectedTotalGold: number): Promise<FishingSaleQuote> {
  const idempotencyKey = createIdempotencyKey("fishing-sale")
  return apiPost("/api/fishing/sell", token, { instanceIds, expectedTotalGold, idempotencyKey }, { "Idempotency-Key": idempotencyKey })
}

export async function fetchFishingTournaments(token: string): Promise<FishingTournamentOverview> {
  return apiGet("/api/fishing/tournaments", token)
}

export async function submitFishingPondTournamentFish(token: string, instanceId: number): Promise<FishingTournamentEntry> {
  return apiPost("/api/fishing/tournaments/open/pond", token, { instanceId })
}

export async function fetchFishingPond(token: string): Promise<FishingPondOverview> {
  return apiGet("/api/fishing/pond", token)
}

export async function loadAllAvailablePondFish(token: string, pond: FishingPondOverview): Promise<FishingPondOverview> {
  if (!pond.built) return pond
  const fish = new Map(pond.availableFish.map((row) => [row.instanceId, row]))
  let cursor = pond.availableFishNextCursor || 0
  while (cursor > 0) {
    const page = await apiGet<{ items: FishingPondFish[]; nextCursor?: number }>(`/api/fishing/pond/available-fish?cursor=${cursor}`, token)
    for (const row of page.items) fish.set(row.instanceId, row)
    const next = page.nextCursor || 0
    if (next && (!Number.isSafeInteger(next) || next < 0 || next >= cursor)) throw new Error("Invalid pond fish cursor")
    cursor = next
  }
  return { ...pond, availableFish: [...fish.values()], availableFishNextCursor: 0 }
}

export async function fetchPlayerFishingPond(token: string, userId: number): Promise<FishingPondPublicOverview> {
  if (!Number.isSafeInteger(userId) || userId <= 0) throw new Error("invalid player id")
  return apiGet(`/api/player/profile/${encodeURIComponent(String(userId))}/pond`, token)
}

export async function addFishingPondFish(token: string, instanceId: number): Promise<FishingPondOverview> {
  return apiPost("/api/fishing/pond/fish", token, { instanceId })
}

export async function addFishingPondFishBatch(token: string, instanceIds: number[]): Promise<FishingPondOverview> {
  return apiPost("/api/fishing/pond/fish/batch", token, { instanceIds })
}

export async function removeFishingPondFish(token: string, instanceId: number): Promise<FishingPondOverview> {
  return apiPost(`/api/fishing/pond/fish/${instanceId}/remove`, token, {})
}

export async function moveFishingPondFishToMain(token: string, instanceId: number): Promise<FishingPondOverview> {
  return apiPost(`/api/fishing/pond/fish/${instanceId}/move-to-main`, token, {})
}

export async function addFishingPondFeed(token: string, itemId: string, quantity: number): Promise<FishingPondOverview> {
  return apiPost("/api/fishing/pond/feed", token, { itemId, quantity })
}

export async function manageFishingPondFeed(token: string, itemId: string, operation: "pause" | "resume" | "withdraw" | "discard_remainder", quantity = 0): Promise<FishingPondOverview> {
  return apiPost("/api/fishing/pond/feed/manage", token, { itemId, operation, quantity })
}

export async function useFishingPondTargetedFeed(token: string, instanceId: number, itemId: string, expectedRevision: number): Promise<FishingPondOverview> {
  return apiPost(`/api/fishing/pond/fish/${instanceId}/feed`, token, { itemId, expectedRevision })
}

export async function setFishingPondPair(token: string, femaleInstanceId: number, maleInstanceId: number): Promise<FishingPondOverview> {
  return apiPost("/api/fishing/pond/pairs", token, { femaleInstanceId, maleInstanceId })
}

export async function deleteFishingPondPair(token: string, slotIndex: number): Promise<FishingPondOverview> {
  return apiDelete(`/api/fishing/pond/pairs/${slotIndex}`, token)
}

export async function applyFishingPondMash(token: string, slotIndex: number): Promise<FishingPondOverview> {
  return apiPost(`/api/fishing/pond/pairs/${slotIndex}/mash`, token, {})
}

export async function revealFishingPondGenome(token: string, instanceId: number): Promise<FishingPondOverview> {
  return apiPost(`/api/fishing/pond/fish/${instanceId}/reveal`, token, {})
}

export async function processFishingPondFish(token: string, instanceId: number): Promise<FishingPondOverview> {
  return apiPost(`/api/fishing/pond/fish/${instanceId}/process`, token, {})
}

export async function claimFishingTournamentReward(token: string, tournamentId: string, division: string): Promise<FishingTournamentReward> {
  return apiPost("/api/fishing/tournaments/rewards/claim", token, { tournamentId, division })
}

export async function redeemFishingCatchUpRod(token: string): Promise<void> {
  await apiPost("/api/fishing/tournaments/catch-up/redeem", token, {})
}
