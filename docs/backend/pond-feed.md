# Pond feed DSL and feeder persistence

## Contract

`pond-feed-v2` keeps the flat `conditions` (AND) and `effects` arrays.
`role` is `staple` or `functional`. Missing role derives from the presence of
effects; old nutrition-only feeds remain staples. Functional feeds support
`maxPortionsPerFishHour` (0 < value <= 0.5, default 0.5). Every stat can occur
only once and at most four effects are supported. Negative effects require a
floor. Conditions and effect variants are validated in the builder and editor.

Functional feeds sort before staples, then by descending `consumePriority`,
diet absorption, and item ID. Each fish consumes at most one functional type
per settlement step. It consumes no more than its nutrition need, dose budget,
available inventory, and payable health/satiety cost. Remaining nutrition comes
from staples. Without staples a functional dose does not expand to replace
the entire meal. All fish request before stock is deducted; each candidate
round distributes scarce stock proportionally to requests. No ID-based first
consumer advantage remains. Main pond and nursery continue to share stock.

`valuePerPortion` means one physical portion, not absorbed nutrition. Diet and
genes affect nutrition only. Compound effects read a common pre-effect fish
snapshot; changing effect array order cannot multiply growth targets. An
unpayable negative effect reduces the dose of the complete compound, not only
the cost. Floors cannot heal an already-unhealthy fish and ceilings cannot
damage a fish above that ceiling. Species size clamps still run before save.
Ordinary growth and predation cannot round away existing fractional feed gains.
The length maximum is the species `MaxLengthCM`; weight is capped at the species
shape-derived maximum (maximum length cubed, times shape factor and 1.12).

## Extension boundary

`delivery` defaults to `feeder`. `targeted` feeds use `role: functional`, zero
nutrition and no feeder priority/hourly dose. They are excluded from shared
stock, scatter feeding and automatic allocation. A player chooses one living
pond resident and consumes one whole portion, regardless of appetite or diet
absorption. Explicit diet/other conditions still apply. A compound with an
unpayable cost or no remaining benefit is rejected without consumption.

Two targeted-only `stat_delta` / `flat` primitives use biological days:
`longevityDays` adds to the existing individual lifespan limit;
`naturalProtectionDays` extends protection from the later of current biological
age or the previous protection deadline. Each value must be positive and at
most 36500 days per portion, with no floor/ceiling. Repeated applications have
no gameplay lifetime count cap. Actual age and genes are unchanged. Offspring
start without these acquired effects.

The sacred feed `pond_feed_sacred_longevity` grants 1095 days of each effect:
72 hours of pond aging at the current 365:1 scale. Its one-portion recipe uses
2 royal jewels, 2 sage scrolls, 20 milk and 1 health-recovery feed. It is excluded
from alchemy conversion to preserve the precious-material acquisition path.

Protection suppresses natural-death rolls, including their health modifier,
but does not prevent health-zero starvation deaths or predation. Expiration
inside a fractional settlement step charges risk only for the unprotected
fraction. It never revives dead fish. Protection follows biological aging, not
wall-clock expiry: existing offline settlement limits and inventory pauses
remain unchanged. Stopping renewal restores ordinary probabilistic mortality,
not instant death at expiration.

`POST /api/fishing/pond/fish/:instanceID/feed` requires `itemId` and
`expectedRevision`. Ownership, living state and revision are checked after
settlement in the existing owner transaction; inventory consumption, fish
changes and pond event insertion commit or roll back together. Warehouse audit
delivery retains the existing after-commit mechanism. Success increments the
persisted care revision so repeated/stale requests cannot charge again.
Clients refresh state after an uncertain failure and never replay automatically.
Owner views expose remaining protection hours and care revision, not the raw
lifespan/deadline. Public pond views omit all care fields.

New combinations of supported conditions and effects require only item/recipe
data and localized artwork/copy; do not add item-ID branches to settlement.
New primitive fields, operators, or effect modes require coordinated catalog
types, builder/editor validation, runtime evaluation, display and regression
tests. Unknown versions fail closed. Keep effects flat, with one effect per stat,
rather than introducing recursively nested programs or implicit effect order.

## Persistence and rollout

Runtime calculations use nutrition units; the database stores physical portions:

```json
{"version":2,"feeds":{"pond_feed_herbal":{"portions":1.25,"paused":true}}}
```

The reader accepts the old flat nutrition map. The next transaction writes the
versioned document, retaining fractional portions. Do not change legacy feed
nutrition values during the initial upgrade: the first conversion uses the
catalog's nutrition value. Subsequent nutrition balance changes preserve the
stored portion count. Unknown versions and invalid catalog references fail
closed rather than silently deleting player stock.

This format is NOT readable by old API/worker binaries. Stop all old writers,
deploy server and regenerated game data together, then restart. Do not perform
a mixed-version rolling deployment. No SQL migration or automatic production
repair is executed. Back up `fishing_ponds` before rollout. A rollback requires
restoring compatible stock JSON as well as the old server/data; do not merely
restart an old binary against version-2 documents.

Targeted care adds optional fields to the existing `fish_data` JSON, requiring
no SQL migration. Old saves start with zero protection/revision. All fish-data
writers must be upgraded together: old binaries cannot preserve the new fields
or honor protection. Do not roll back to old writers after care has been used.

## Management

`POST /api/fishing/pond/feed/manage` accepts `itemId`, `operation`, `quantity`.
Operations: `pause`, `resume`, `withdraw` (positive integer, <= 10000), and
`discard_remainder` (only when less than one portion remains). Every operation
settles elapsed time first, locks the owner, and updates stock plus inventory
balances in the same transaction. Warehouse audit delivery retains the existing
after-commit mechanism. Withdrawals never round fractional feed up.
A stale excessive withdrawal fails without granting inventory. Pause settings
persist across further feed additions. Discard requires explicit UI confirmation.

## Authoring and tests

All feed recipe outputs must use the feed as their canonical main output.
Each ingredient count must be >= output count; at most one finished feed is
allowed; input base value must be >= 120% of output value. V2 recipes require
loot plus a separate player-produced ingredient. Broodstock mash and lineage
reagent are not feeder items and retain their building requirements.

Focused tests cover capped physical consumption, staple fallback, proportional
allocation, paused stock, effect-order independence, payable compound costs,
legacy fractional stock conversion, and immutable physical portions after a
nutrition rebalance. Seven-day settlement tests exercise every authored shared-feeder v2 feed
against all catalog pond species for dose and size/health/satiety invariants;
they are not a claim that long-term economy balance has been playtested.
Browser tests cover management, targeted renewal and structured editor saves
with isolated mocks. Targeted-care PostgreSQL tests use loopback session-local
temporary tables and cover owner isolation, empty stock, stale retries and
rollback after inventory/fish/event writes; run with `POND_CARE_TEST_DATABASE_URL`.
They do not replace a full authenticated production-shaped API test.
Shared-feeder withdrawal still needs its own database rollback integration
proof; the targeted-care tests cover only their respective mutation path.
