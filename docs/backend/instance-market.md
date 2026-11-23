# Instance Variant Market

## Contract

The ordinary commodity market is unchanged for items without `marketPolicy`.
Opted-in items use a separate fixed-price order book, with no new buy orders.
Existing commodity listings and buy orders remain readable, purchasable,
fillable with standard instances, and cancellable. They are never converted
to custom-instance listings or refunded automatically.

Source authority:

- `packages/game-data/source/json/items.json`: optional item capability.
- `packages/game-data/source/json/market_profiles.json`: grouping and display.
- `server/internal/marketpolicy`: strict selectors and deterministic identity.
- `server/internal/store/market_instance_projection.go`: public domain adapters.

Example item capability:

```json
"marketPolicy": {
  "fixedPrice": { "mode": "instance_variants", "profileId": "effect_consumable_v1" },
  "buyOrders": false,
  "auction": false
}
```

The first rollout opts in `fish_catch` and the currently configured recipe outputs.
Runtime routing never checks their names, item IDs, categories, or food labels.
Stackable items and currency cannot opt into this first instance protocol.

## Profile DSL

Profiles have `id`, `schemaVersion: 1`, `groupingVersion`, `catalogKeys`,
`groupKeys`, `presentation`, optional `uniqueByInstance`, and optional `disabled`.
An item references one profile. Missing references and unknown fields fail
validation during game-data build and catalog load.

Each key specifies `source`, `path`, and `compare`:

- Sources: `instance` (public instance snapshot), `resolved.operationEffect`
  (the authoritative consumable effect after existing compatibility resolution).
- Paths: object-only JSON pointers; empty path selects the whole source.
  Known source roots are validated; missing nested values fail closed at quote.
  There is no arbitrary SQL, scripting, wildcard, implicit coercion, or expression evaluation.
- Comparison: `exact` preserves array order; explicit `set` sorts and deduplicates arrays.
- `catalogKeys` participate in identity and can filter the groups API via
  `catalogKey`. The initial UI is a flat, paginated variant grid, not another
  catalog-navigation layer.
- `groupKeys` define exact equality. Numbers are canonical exact rationals,
  not rounded labels: `0.104` equals `0.1040`; `0.10401` differs from `0.10402`.
  Null, missing fields, strings, and numbers are not conflated.
- `presentation` adds `labelKey` and one of `text`, `number`, `percent`,
  `seconds`, or `operation_effect`. It does not determine identity.

Food grouping selects the full resolved effect, including all parameters,
lifetime, targets, selectors, stacking rules, and effect version. Different
charges or duration do not merge just because the displayed percentage matches.

The initial specimen profile uses public specimen data and `uniqueByInstance`.
This deliberately keeps each fish separate, since undisclosed genetic data
cannot safely establish publicly interchangeable stock. Public snapshots and
hashes exclude unrevealed genes, parent instance IDs, and internal care fields.
The original database row retains them unchanged through sale or withdrawal.
Future profiles can select different public fields without changing market code.

Disable a profile to block new quotes and purchases while retaining withdrawal.
Grouping configuration is versioned and hashed; stale listings cannot settle
under a different grouping contract. Presentation-only edits do not invalidate
identity; existing offers retain their original display snapshots.

## Storage And Settlement

Four additive tables keep the old order book and its settlement code separate:

| Table | Purpose |
| --- | --- |
| `market_instance_listings` | Seller, price, exact group, quantity, immutable quote identity |
| `market_instance_listing_items` | Escrow reference and immutable public snapshot per original instance |
| `market_instance_orders` | Purchase receipts, exact price, fees, idempotency identity |
| `market_instance_ledger` | Settlement accounting for the new book |

Instances stay in `player_item_instances` with `state='market'` while listed.
Buying changes ownership/state of the original rows; it never calls a template
grant for the purchased item. Partial buys select the oldest instance IDs.
Terminal escrow references are detached, so later consumption cannot violate
foreign keys. History retains the original ID and public snapshot.

Locks are ordered: users in ascending ID order, listing, then instances.
Gold, original item transfers, stock, orders, fees, inventory audit, and risk
records commit together. Deferred constraints check escrow quantity at commit.
Quotes and new listings are bounded to 100 instances, sellers to 200 active
listings, read pages to 30 (store ceiling 50). Grouping and offer reads use SQL
aggregation and indexed, paginated queries, not client-side full-book downloads.

Public offers are anonymous: responses expose only `isOwn`, never the seller's
account ID or name. Internal listing/instance identifiers remain protocol keys
and are not rendered as player-facing labels. Offers sort by price, then listing
ID (the ordinary market's insertion order). `canBuy` identifies the first active
offer not owned by the viewer across the whole group, independent of pagination.
Settlement rechecks this priority before transferring items or gold; stale requests
cannot skip cheaper or earlier offers. A rejected quote requires fresh confirmation,
not an automatic purchase at another price. This uses the existing offers index
and requires no additional schema migration.

Variant listing and purchase history share the ordinary history tab level.
Effect bullets reuse the crafting preview component; money uses item quick refs.
Market query-string navigation retains its page instance and catalog state.

Equipped/active-loadout items, archived instances, non-inventory instances, and
instances backing an active competition entry cannot be listed. Competition
history remains with the original entrant, not the purchaser.

POST list/buy require an idempotency key and reject reused keys with different
payloads. The client retains the original payload/key after an uncertain result,
does not silently replay another operation, and discards responses from prior
item/account contexts. Confirmed writes remain successful if a follow-up read fails.

## Deployment

The migration is additive and included in `server/schema/init.sql` for fresh
installations. Existing installations must apply the dedicated migration before
serving the new API/frontend. Use the normal reviewed deployment connection:

```sh
# From server/, with DATABASE_URL already supplied by the deployment environment.
go run ./cmd/schema_apply schema/migrations/20260905_market_instance_variants.sql
```

Then deploy matching server, game-data, frontend, and workers together using the
normal project procedure. Do not run the entire init schema as a substitute.
Do not drop these tables or revert to code lacking withdrawal while escrow is
active. To stop new trades safely, disable the configured profiles and keep the
new cancellation endpoint available until all escrow has been returned.

The new ledger is separate from legacy `market_orders` statistics; reporting
consumers that need a combined total must explicitly sum both books rather than
assuming every market trade lives in the old table.

## Verification

- DSL canonicalization/strict validation, HTTP auth/body bounds, UI capability,
  locale namespaces, uncertain retries, stale account/item results, and refresh failures.
- Real PostgreSQL tests use a random schema, high fixture account IDs, and
  automatic schema cleanup. They cover equal multi-seller groups, partial sale,
  withdrawal, concurrent buyers, buy/cancel races, concurrent identical requests,
  late transaction rollback, all current custom recipe outputs, private fish data,
  competition claims, and gold conservation. Migration replay is included.
- Full-page browser fixture: `web/tests/fixtures/instance-market-browser.mjs`.
  It intercepts every game API request and game socket. Browser screenshots
  and click-throughs using this fixture are UI proof, not authenticated live E2E.

```sh
node --test packages/game-data/scripts/market-policy.test.mjs
cd server
INSTANCE_MARKET_TEST_DATABASE_URL="$LOCAL_TEST_DATABASE_URL" \
  go test ./internal/store -run TestInstanceMarketPostgres -count=1
```

Never point the PostgreSQL test variable at a production database. The tests
reject non-loopback hosts and never include `public` in their schema search path.

Full-server-suite verification in this dirty checkout also reported three
failures outside the market changes: `TestLoadSpecializationCatalogSource`
(balance revision 4 versus expected 3),
`TestFishCuisineReplacedItemsUseFrozenIngredients` (grant-retirement expectation),
and `TestWorkMealQualityAndFrozenLifetimes` (chance 0.015 versus expected 0.035).
They were not changed as part of this market rollout. Do not describe that run
as a clean full-suite pass or these focused tests as production verification.
