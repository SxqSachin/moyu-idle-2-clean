# Discounted Recycling

Player-facing name: 折价回收. Existing `destructive` API routes and internal identifiers are retained for compatibility.

This is an opt-in disposal path, not affix extraction or smelting. Existing ordinary recycling, market eligibility, enhancement, affix and smelting rules are unchanged.

## Eligibility and Pricing

- Only owned, inventory-state, nonstackable equipment instances with quantity 1 and valid affixes or a positive enhancement level can use this path. Enhanced equipment without affixes is supported. Already ordinarily recyclable equipment must use ordinary recycling.
- Durability, maximum durability, customization and base-attribute restrictions remain. Enhancement levels must be valid for the authoritative enhancement runtime. Archived equipment is supported. Player equipment, kitty equipment and active fishing references are rejected even if the instance state is inconsistent.
- Pricing accepts either the template base snapshot (including legacy empty snapshots) or the existing enhancement-plus-effective-affix snapshot-builder result. It does not discard arbitrary base modifications. Enhancement attributes and affix limits use the existing server resolvers.
- A local copy has enhancement reset to zero, its affixes removed and its base restored to the template. Ordinary recycling evaluates this copy, including the existing missing-native-affix discount. The persisted equipment is never cleared or rewritten.
- Disposal fee is 10% of the integer body price, rounded up **per equipment instance**. Net payout is body price minus fee. Payout below 1 gold is rejected, not rounded up. Splitting batches cannot lower the fee. Values and aggregate body prices must fit the browser's exact integer range.
- No hammers are consumed and no carriers, afterglow, glow cores or other affix rewards are granted. Enhancement costs are not refunded. Enhancement level, affix rarity, rolls, modifiers and extraction investment do not raise the body price. The same 10% equipment processing fee applies with or without affixes.

## API and Transactions

- `POST /api/items/recycle/destructive/quote`: `{ "items": [{ "instanceId": 123 }] }`.
- `POST /api/items/recycle/destructive`: the same list with a `quoteToken` for every item. Duplicates, unknown fields, missing tokens and batches outside 1-1000 items are rejected.
- Quotes contain the exact original enhancement level and affixes, body price, disposal fee and final gold. The v2 SHA-256 state fingerprint binds ownership, instance identity, quantity, state, archive state, customization, enhancement, durability, base snapshot, original affixes, milestone claims, roll seed and computed price. Old quotes require a new preview.
- The fingerprint is an optimistic-concurrency guard, not a client price or a bearer authorization. The service always rechecks ownership, eligibility and prices under locks.
- Both paths acquire the user advisory lock and instance row locks in ascending ID order. Confirmation validates **every item before any deletion**. Bulk deletion and the normal gold ledger grant share one PostgreSQL transaction. Any failure rolls back all equipment and gold changes. Repeated or concurrent submissions cannot pay twice because each accepted instance is completely consumed.
- Warehouse auditing uses the existing post-commit audit buffer. One aggregate event preserves original enhancement levels, every original affix and per-item price, plus a correlated gold event. As elsewhere in this ledger, audit persistence is best-effort after commit; it is not part of the equipment/gold atomicity guarantee.
- No schema migration, automatic repair, new worker, cache or background task is introduced.

## UI and Verification

Item details expose the action only for destructive-recycle candidates. Warehouse inventory and archived menus have a separate destructive selection mode; ordinary selection is not broadened. Server eligibility remains authoritative if a candidate fails quoting.

The shared confirmation dialog defaults to unchecked consent, lists original enhancement levels, affixes and server prices, and resets on reopen, selection changes and account changes. Consent includes the number of enhanced items and explicitly states that upgrades are not refunded. It disables repeat submission and never automatically retries an uncertain mutation. A stale or failed request requires a fresh quote and new consent.

Focused tests cover unchanged ordinary eligibility, body-only prices, integer rounding, limits, stale fingerprints, ownership, malformed requests, consent resets, account switches and double clicks. Opt-in PostgreSQL tests create a fresh isolated schema on a loopback database and test concurrent exactly-once payout, whole-batch rejection, gold-grant rollback, preserved affixes, complete batch audit details and equipment references. Set `DESTRUCTIVE_RECYCLE_TEST_DATABASE_URL` to run them; no live player tables are accessed.

`web/output/destructive-recycle-harness.html` mounts the actual dialog with an explicitly isolated quote fixture and disables mutations. This verifies desktop/mobile layout, not authenticated player-path integration. The running backend must be rebuilt/restarted to serve the new routes.
