# Ordinary Statistical Settlement

Implemented locally, 2026-09-10. Production admission defaults off. This is the
approved ordinary-work slice, not a replacement for every offline executor.

## Scope And Migration

- Player and kitty gathering, mining, animal farming, crafting, forging, sewing
  and cooking use the same versioned kernel for online single operations and
  offline segments. Independent equipment is included, not collapsed into stacks.
- Automatic fish cooking uses a bounded input adapter. Specific fish ingredients
  and explicitly bound/inherited equipment retain their single-operation input
  and quality owners; their extra-output procs use the new random version.
- Enhancement, alchemy, fishing trips, buildings and combat retain their existing
  executors, rules and receipts.
- Existing prepaid operations finish with their old resolver/frozen snapshot.
  Admission happens only when their next start succeeds. No credited reward is
  recalculated. New pending starts may enter immediately when admission is on.

The internal queue envelope stores `ordinary-statistical-v1`, random version
`binomial-subset-v1`, a private 256-bit seed, first ordinal, action specification,
output/affix rules and fish recipe where applicable. Existing JSONB columns are
sufficient; this change requires no schema migration. Player projections never
include this seed. Unknown versions fail closed instead of switching samplers.

## Randomness And Causality

For every fixed 256-operation block and proc stream, sample K from a binomial
distribution, then uniformly sample K hit positions (Floyd subset selection).
This has the independent Bernoulli distribution, including variance, rather
than paying the fixed expectation `n * p`. Gonum v0.17.0 owns the binomial sampler.
The block size, stream derivation and sampler dependency belong to the version.

Positions matter: a refund may finance the next start, but a future refund must
not finance an earlier unaffordable start. Seed, proc identity, probability and
absolute ordinal determine the same hits for one-step, random-sized, restarted
and retried settlement. Base drops, extra output and affix rolls have separate
streams. A material-refund hit remains one correlated event for all its inputs;
per-material floors and per-operation XP rounding are retained.

Fixed disjoint stackable cost/output work can aggregate affordable starts and
hit counts algebraically. Feedback, refunds and equipment use a bounded in-memory
sequence, not repeated full business handlers or per-operation database queries.
Actual independent outputs and required receipts necessarily cost O(output).

## Boundaries And Persistence

- A segment completes at most 2048 operations, also bounded by output instances,
  fish input volume, remaining queue count, target time and existing worker budgets.
- Stop before another active lane's next completion; ties and unknown timestamps
  use one operation. A finished ordinary queue materializes its next head before
  another actor advances. Combat-paused kitty lanes are excluded as before.
- Food activation, expiry, charge exhaustion, selection changes, work duration
  and XP level transitions split segments. Shared food pools are permitted only
  within that ordered lane horizon under the existing user lock. Owner/furniture
  progress dependencies conservatively retain one-operation boundaries.
- Load only relevant eligible commodity rows, preserving unarchived-first and
  instance-ID consumption order. Depletion deletes a row; later refunds do not
  resurrect it. Custom/enhanced/rolled instances are not ordinary template inputs.
- Frozen grants retain their rules. If current inventory-eligibility defaults
  differ from those rules, use single-operation segments so catalog changes do
  not make outputs spendable only inside a batch.
- Bound explicit material inputs to their single craft. Inherited enhancement
  applies only to the base output, never to food-generated bonus equipment.
- Bulk-update dirty stacks/XP and insert new instances in pages of 512. Existing
  rows retain identity. Equipment rolls are per item; bonuses are not clones.
- Use the original start adapter and a savepoint for the final successor. The
  last incomplete operation can already have prepaid materials and food charges.
  Failed starts do not consume a new charge receipt or count as completed work.
- Queue checkpoint, grants, receipts, achievements and inventory remain inside
  the existing transaction/user-lock/recovery-revision fence. Audits retain the
  existing post-commit best-effort sink, with aggregate quantities, proc identity
  and operation ranges; they are not promoted to authoritative reward receipts.

## Terminal Inventory

Inventory deltas exceeding 256 KiB publish the existing `refreshRequired` message
directly, including local delivery without NATS. Item JSON is encoded outside
the global patch mutex. Per-user striped locks preserve publication order.
Cache reset/eviction creates a fresh inventory epoch as well as a new version.

A terminal recovery response includes `inventorySync` metadata. The browser
records the synchronized receipt, shows the final report and sends ACK only
after that version is applied or an uncached `/api/inventory/instances?fresh=1`
request succeeds. Concurrent requests for the same target share one promise;
newer versions trigger a bounded retry. Failed fetches and account resets do not
claim success. Older servers without this optional metadata remain compatible.

This removes oversized WS frames, not the real size of the warehouse. Full HTTP
projection, transfer and client memory still scale with item count. General
inventory pagination/virtualization and production reconnect load are separate
measurement tracks.

## Rollout And Rollback

1. Deploy matching `server`, `action_worker`, `battle_worker`, `kitty_worker` and
   the frontend with `ORDINARY_SETTLEMENT_ENABLED=false` (the default).
2. Verify that no old action-capable binaries remain, then set
   `ORDINARY_SETTLEMENT_ENABLED=true` on all four processes and restart them.
3. Observe settlement duration, progress age, DB/instance writes, projection
   size and errors with representative staging users before a broad rollout.

Turning the flag off stops new admissions; already versioned queues keep their
compatible engine. Do not roll those queues back to an old binary or delete
their checkpoint metadata. Keep a compatible worker until they drain. Existing
recovery scan/retry SQL is unchanged; retry must retain its task/revision fence.
Nothing in this implementation scanned, repaired or deployed production data.

## Verification And Self-Review

Local Go tests cover distribution mean/variance/position independence, arbitrary
partitions, ruleset retention, archived rows, prefix affordability, correlated
refunds, overflow, independent equipment and inherited-versus-bonus enhancement.
Isolated PostgreSQL comparisons cover fixed inputs, refunds, partial prepayment,
XP changes, automatic fish ingredients, shared player/kitty food and materials,
tied timestamps and queue handoff. The complete recovery test injects checkpoint
failure after equipment writes, retries with unchanged seed, disables admission,
replays old revisions and checks 50000 unique authoritative inventory instances.

Representative local runs (loopback PostgreSQL 16, Go 1.26.2; not production):

| Fixture | Result |
| --- | --- |
| 50000 stack-output actions, executor only | about 50 ms, 25 calls, 221 SQL |
| 14400 capped-XP food actions, executor comparison | 50.5 s / 100800 SQL single; 331 ms / 64 SQL batch |
| 50000 independent equipment, recovery commits plus injected retry | about 4.0 s, 1088 SQL |
| Those equipment instances, authoritative read plus JSON | 0.44-0.62 s; about 35 MB before transport |

The isolated browser fixture holds the 50000-item inventory response: zero ACKs
before release, one download, 50000 applied instances and one version-bound ACK
after release. The final report renders normally. This is a protocol/player-path
proof, not a production network benchmark or a full large-warehouse render test.
Focused frontend tests and typecheck, HTTP projection/transport tests, four
command builds and scoped race tests passed.

Separate self-review corrected operation-range timestamps, incomplete legacy
identity, unknown lane timestamps, queue carry ordering, input-volume bounds,
catalog eligibility changes and cache-reset epochs. A wider optional PostgreSQL
test, `TestFishCuisinePostgresExclusiveAndServingCaps`, fails on the soup serving
cap; the same failure was reproduced on the pre-change HEAD in a separate
temporary checkout. That pre-existing unrelated food-limit issue was not changed.

No production multi-node/concurrency, constrained-host p95/p99 or real network
latency claim is made. Source game data was not modified.
