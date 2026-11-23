# Offline Recovery

The target architecture and remaining phases are documented in
[Exact Offline Batch Engine](../planning/offline-batch-engine.md).

## Single Login Report (2026-09-10)

New clients opt into `reportPolicy: "combined"`. The gateway projects all sealed,
unacknowledged recovery rows into one cumulative report; it does not merge or
rewrite their settlement windows. Any unfinished member delays the report, and
a stalled response retains that member's own retry ID/revision. Offline duration
is the sum of capped windows, not the span including online gaps. Battle rewards
use the union of actual absence intervals plus captured late runs, counting each
receipt/log once and preserving earlier-window ownership.

`reportTasks` identifies every member and revision; `reportKey` also participates
in terminal snapshot synchronization. The inventory/player refresh happens once
for the combined report. A compact digest ACK reconstructs membership under the
user lock, with a maximum task ID so later absences cannot enter it. Even a large
backlog stays below the incoming WebSocket frame limit. The ACK is atomic, user-bound, revision-fenced and
blocked until every member and captured battle is complete. Replayed ACKs cannot
consume later absences. Legacy clients retain the single-task protocol.

The client waits for actions, late battle receipts and authoritative inventory
before displaying or acknowledging the report. A bounded, account-scoped local
ledger stores only shown task identities/high-water mark and a pending ACK, not reward data. It
survives reload/ACK loss; Web Locks serialize display claims across supported
tabs. Storage-disabled clients retain in-page deduplication only. A request/session
generation fences asynchronous inventory responses after newer requests,
disconnect, logout or unmount. Repeated unaccepted ACKs use slower polling.

Deploy matching gateway and web builds; no schema migration or reward replay is
needed. Isolated PostgreSQL and browser fixtures are not production recovery or
network-load measurements. Do not force-complete or delete recovery rows to fix
a presentation problem.

## Stalled Recovery Operations (2026-09-09)

Run [the read-only scan](../../server/schema/operations/offline_recovery_scan.sql)
on the affected database. It reports unfinished task counts, up to 100 stalled
or suspicious running tasks, and up to 100 unfinished battle dependencies of
complete action receipts. Sample twice and correlate the task/user IDs with
`offline recovery retry`, `offline recovery scan failed`,
`offline action queue executor made no progress`, and battle-worker logs.
`running` plus attempts is not proof of permanent failure; `available_at` can
move while the revision remains unchanged during repeated transaction failures.
Background windows parked as complete at the offline cap are not broken tasks.

After repairing the underlying error, use
[the bounded stalled-task retry](../../server/schema/operations/offline_recovery_retry_stalled.sql).
It defaults to a preview transaction ending in `ROLLBACK`. Review the returned
database identity, task IDs and previous error codes. To apply one batch, change
only the final `ROLLBACK` to `COMMIT` and run once. Each batch locks/skips busy
rows and retries at most 100 currently stalled tasks, preserving their exact
window, summary, pending battles, inventory and queue checkpoints. It increments
the revision and clears only retry metadata. Do not loop the script against a
persistent failure, reset running/complete tasks, delete receipts, or force
completion/ACK. More than 100 candidates require separate reviewed batches.

Matching gateway/action-worker processes must be running: sealed tasks resume
through the gateway dispatcher; open background tasks use the action worker's
existing scheduling/rebuild path. Re-run the scan and verify revisions/committed
steps advance and the task eventually completes. A task returning to `stalled`
still needs its executor/schema/data error repaired. Already-complete actions
waiting on a battle must be investigated in the battle worker, not restarted.
The scan cannot diagnose a client-only or final-projection error; inspect
`offline recovery read failed` / `offline recovery projection deferred` logs
when no unfinished task/battle explains the player symptom.

The client now displays a compact, nonblocking notice only for a confirmed
stalled task, with an explicit task/revision-bound retry. Polling, page refresh,
and transport timeouts do not reset tasks. A lost retry response reuses the same
version; retry increments the revision so delayed worker failure handlers and
duplicate old commands cannot alter the restarted generation. Stalled polls
return compact progress/status without fetching battle rewards or synchronizing
the full inventory; only completed tasks enter terminal projection and ACK.
Normal background progress still does not own a loading overlay or toolbar entry.

No schema migration is required for this fix. Deploy the matching server and
web build; the existing three-service compatibility requirements below remain
in force. Neither local tests nor these scripts establish that production
accounts have recovered. No production database was modified during this work.

Validation: isolated PostgreSQL recovery/background/projection tests pass, as
do focused race tests for concurrent retry and the operations scripts (preview
rollback, immutable receipt preservation, replay, 100-row cap and locked-row
skipping). The 24 focused web tests, typecheck, Vite build and all three service
builds pass. Isolated browser fixtures cover desktop/mobile layout, one explicit
retry, repeated failure without automatic restart/ACK, and a lost response that
replays the same retry revision before showing one complete report. Review
passes checked player behavior, receipt/worker integration and operational
bounds. The full repository suite and production recovery were not run.

## Ordinary Actions And Fishing (2026-09-08)

These paths already had real batching before the alchemy segment adapter:
eligible primary work uses bounded stable segments, and fishing advances its
state machine in memory with at most 128 pending catches. This follow-up removes
redundant work inside those batches rather than changing their semantic grouping.

- Generic starts parse immutable operation metadata once per batch. Every start
  still receives its original ordinal/time key, frozen selection and charge
  receipt. Ineligible instance-bound operations remain ineligible.
- A batch-local life-action resolver decodes repeated parameters once and reuses
  RNG storage. Each roll is reseeded with the original key/grant/kind/slot seed;
  no roll, outcome, material cost or grant selection is cached. Experience keeps
  the original per-operation rounding and accumulation order.
- Fishing reuses one phase RNG allocation, reseeding on every original RNG call.
  Only the existing seed and event sequence enter the checkpoint. Fish identity,
  individual attributes, mutations, fight outcomes and supply wear are unchanged.
- Knowledge and records now use at most two set-based UPSERT statements, not one
  statement per species. Sorted input, nullable dates/IDs, record ties and
  unchanged-record timestamps preserve the former SQL semantics.
- Plain stackable scaleglow grants merge inventory once per catch batch, while
  keeping each catch's ledger and achievement events. Fish insertion still comes
  first, preserving catch/instance IDs. Non-stackable/default-affix variants keep
  individual grants. No schema or save-format migration is required.

Local Apple M4 Pro measurements (not production guarantees):

| Workload | Previous path | Current path |
| --- | --- | --- |
| 512 generic start snapshots, CPU only | 2.00 ms / 1.25 MB allocated | 0.048 ms / 0.084 MB |
| 512 three-rule life-action resolutions, CPU only | 12.47 ms / 10.43 MB | 10.84 ms / 0.163 MB |
| 2000 fishing RNG calls, CPU only | 14.51 ms / 10.85 MB | 13.71 ms / 0.0054 MB |
| 128 pending catches with 85 scaleglow grants, synthetic persistence stress | 42.4 ms / 173 direct query calls | 5.3 ms / 5 direct query calls |

The scaleglow stress fixture intentionally has many more drops than a normal
trip. The 24-hour dense fishing fixture lands 1870 fish in about 0.3 seconds of
executor/persistence work; the previous batched path is already about as fast.
Do not describe this as an order-of-magnitude end-to-end fishing speedup. The
primary-work reference still verifies 14400 completions: 40.3 seconds sequential
versus 0.192 seconds batched. That speedup includes the pre-existing batching,
not just this follow-up. Timers exclude player bootstrap/network/UI; direct query
counters do not count individual statements inside pgx pipelines.

Proof includes frozen pre-change fishing executor/state writers and generic
snapshot construction. Tests compare complete database rows, IDs, each physical
checkpoint/result, XP, food receipts, tournament entries across a UTC week,
catch disposition and audit/achievement contents. Scenarios cover 128-catch and
37-phase boundaries, all-kept/all-released/record-only, food charge exhaustion,
bait/tackle exhaustion, nullable/tied records, archived/custom/duplicate stacks,
and non-stackable fallback. Only the unchanged refund map's unspecified audit
representative is normalized; its full item quantities remain exact.

The real durable fishing recovery test injects failures in records, trip state
and the recovery checkpoint, then checks complete rollback, concurrent/stale
retry and process reconstruction. Partial phases do not count as completed
trips: its 1870 fish span eight checkpoints and exactly one completed action.

Three review passes checked output/rounding/RNG parity, transaction/effect/worker
integration, and bounded allocation/SQL/error behavior. No new sampler, larger
budget, removed lock or cross-actor ordering relaxation is included. Material
refund feedback, inherited instances and dependent crafting retain their prior
sequential safety paths; a general ordered inventory engine remains planned.
Production reconnect/load and constrained-host measurements remain required.

Validation: the new differential/rollback/recovery tests and their `-race` run
pass; `server`, `action_worker` and `battle_worker` build successfully. The full
backend run with isolated PostgreSQL coverage still has only the two known
unrelated failures, `TestLoadSpecializationCatalogSource` and
`TestArenaAcceptsProportionalCapacityStampAfterSerialization`. No deployment was
performed and no production player database was modified.

## Exact Alchemy Segments (2026-09-08)

Durable recovery now admits eligible player conversion/transmutation to an
independent stable-segment adapter. It compiles parameters once, performs only
necessary random/rounding/receipt work in memory, updates inventory per touched
template and XP once per segment, and writes the queue/checkpoint through the
existing recovery transaction. It never invokes the full old executor per
virtual completion. RNG storage is reused and reseeded without changing the
single-operation generator or draw order; weighted keys are sorted once.

Segments are bounded by the existing shared budget and 1024 completions, the
first level-changing completion, food charges/validity/selection changes, exact
prefix affordability, and the next other-lane completion (ties are not crossed).
The first prepaid input and the extra pending successor reservation are retained.
The final successor still uses the authoritative start adapter. Frozen snapshots,
per-start receipt keys, per-reward achievement events, recovery-token completion
aggregation, per-operation XP rounding/version increments, inventory identities
and summary floating-point addition order are preserved.

Output/cost feedback, instance-targeted transmutation, non-stackable rewards,
unknown XP contracts, shared cross-actor charge pools and other alchemy executors
remain on the old path. Other runnable lanes are boundaries, not ignored work;
combat-paused kitty lanes use the same exclusion as the existing scheduler.
No-effect legacy aggregate samplers remain unchanged: their grouping/random
semantics are different from repeated single operations. This is not a new RNG
protocol; identical explicit random inputs are the differential-test contract,
not counterfactual wall-clock outcomes or deterministic uncommitted retries.

Local isolated PostgreSQL evidence, 2026-09-08:

| Workload | Reference | Segment |
| --- | --- | --- |
| 512 effectful transmutations | 1115 ms / 5460 core SQL calls | 42 ms / 25 calls |
| 512 effectful conversions | 1330 ms / 5729 core SQL calls | 18 ms / 34 calls |
| 8192 fixed-reward transmutations, real recovery | No old end-to-end baseline in this fixture | 1421 ms, one checkpoint, 156 core SQL calls |

The first two timers include executor/queue/achievement and food-receipt writes,
but exclude COMMIT and post-commit audit delivery. The 8192 timer includes COMMIT
and audit writes (approximately 834 ms settlement plus 584 ms audit), not worker
dispatch, network delivery, frontend rendering or concurrent production load.
Core SQL counters exclude individual statements in audit pipelines. These are
local synthetic measurements, not production latency guarantees.

Differential tests compare full inventory rows (including archived stacks and
IDs), attributes/versions, discoveries, rewards/costs, continuation/cancellation,
both summary levels, audit contents and persisted food receipts/remaining charges.
They cover level/charge/duration/future-effect boundaries, shortages, terminal
and pending successors and different physical partitions. The real recovery
test injects inventory, effect, achievement and checkpoint failures, verifies
full rollback/no premature notification, and retries concurrent/stale revisions.
Audit comparison permits only the legacy map-order-dependent representative of
multi-item reserve events; all item quantities and run identities remain exact.

Committed recovery logs include `alchemy_segments` and `alchemy_segment_steps`
alongside `settle_ms`/`audit_ms`. Use these to distinguish admission/fallback from
settlement and projection costs before changing budgets or worker concurrency.
Three review passes covered feature correctness, upstream/downstream contracts,
and operational safety. Focused PostgreSQL/store/HTTP/effect tests and selected
race tests pass; server, action-worker and battle-worker binaries build. The
full backend suite still fails the two existing specialization-version and
Arena proportional-capacity expectations documented below, not this segment's
tests. No claim of a clean full-suite run or zero residual risk is made.

This change adds no schema migration or production deployment. Staging workload,
reconnect/concurrency and player-device validation remain release requirements.

## Settlement I/O Follow-up (2026-09-08)

### Design And Invariants

Background recovery already removes most normal absence work from login. The
remaining expensive paths must do less work per logical operation, not merely
increase timeouts, run competing settlements, or move the same backlog again.
This follow-up changes I/O and repeated reads only; it does not change executor
selection, batch cardinality, reward algorithms, RNG creation/draw order, food
selection, prepaid costs, rounding, actor ordering, or recovery budgets.

1. Player conversion/transmutation reuse derived inputs and base timing inside
   one uninterrupted queue lane in a single user-locked transaction. Any level
   change invalidates these reads before preparing the successor. Another
   executor/lane, a new queued start/loadout, or a new chunk also discards them.
   Frozen work-loadout inputs retain precedence. Kitty work, bench/inheritance
   and instance-targeted transmutation do not opt in: they may change equipment
   or have other attribute dependencies. Food snapshots remain per-start and
   are never cached as base timing. No cross-request/global cache is added.
2. Durable recovery buffers prepared achievement events in batches of at most
   1,000 rows, using the existing bulk INSERT and conflict handling. Event IDs
   and payloads are prepared at their original call sites, not regenerated or
   aggregated later. The buffer is Store/transaction/user-bound. All remaining
   events are inserted before the recovery checkpoint; a failed insert or
   checkpoint rolls back rewards, queue progress and events together. One
   wake-up follows a successful COMMIT rather than each uncommitted grant.
   Ordinary non-recovery transactions retain their previous enqueue path.
3. Post-commit inventory audit writes use pipelines of at most 128 unchanged
   INSERT/UPSERT statements. Individual rows, identities, order and conflict
   behavior are preserved, including repeated keys in one pipeline. This
   removes round trips rather than merging away operation detail. Each pipeline
   is atomic; a failed pipeline returns an error and later pipelines do not run.
   As before, these logs are best-effort projection data, not reward authority.
   Log creation timestamps share the pipeline transaction time; settlement and
   battle receipt timestamps are unchanged.

Recovery commit logs now separate `settle_ms` (through COMMIT and achievement
wake-up) from `audit_ms` (post-commit audit projection), alongside total elapsed
time and completed steps. This avoids interpreting slow logging as slow reward
calculation. Lock acquisition before the settlement loop is not included in
these fields; correlate gateway/request and database lock-wait metrics too.

### Local Evidence

Measured in newly created disposable loopback PostgreSQL databases, not the
player database or production:

| Workload | Original | Optimized | Coverage |
| --- | --- | --- | --- |
| 512 sequential transmutations | 1.658 s / 8,820 SQL | 1.038 s / 5,820 SQL | Original executor with fixed reward fixture, XP upgrades, food exhaustion and material shortage; excludes audit flush/COMMIT |
| 512 sequential conversions | 2.465 s / 10,475 SQL | 1.617 s / 7,540 SQL | Same scope, gold costs and conversion food |
| 4,096 achievement events plus duplicate replays | 378 ms / 2,235 SQL | 78 ms / 5 SQL | Exact event identities, payloads and deduplication; excludes COMMIT |
| 2,049 audit writes | 404 ms | 47 ms | Same stored rows and ordered UPSERT, including pipeline boundaries |
| Existing real one-second-food transmute return tail, 30 operations | 116 ms | 55 ms | Store recovery call plus receipt read; excludes network/bootstrap/browser and uses fixture-disabled achievements |

The sequential A/B tests compare every result payload, logical timestamp,
inventory total, attribute XP/level/value/lock version and food charge receipt.
Separate fixed-seed tests compare every random reward for unmodified conversion
and transmutation rules. Recovery tests also inject both event-write failure
and checkpoint failure, verify rollback and notification-after-COMMIT, and
exercise stale revisions, concurrent recovery, restart/background handoff and
late battle receipts. Audit tests verify bounded failure and connection reuse.

Focused PostgreSQL/store/HTTP recovery and effect regressions passed. The
ownership, rollback/replay and audit failure tests also passed with `-race`;
server/action-worker/battle-worker builds passed. Three review passes checked
settlement results and failure paths, upstream/downstream invalidation and
transaction boundaries, then query/memory bounds and operational safety. Full
backend tests still report `TestLoadSpecializationCatalogSource` and
`TestArenaAcceptsProportionalCapacityStampAfterSerialization`, matching the two
previously documented failures below; the full suite is not green.

```sh
cd server
OFFLINE_RECOVERY_TEST_DATABASE_URL='postgres://game:game@127.0.0.1:5432/game?sslmode=disable' go test ./internal/store -run 'TestOfflineAlchemyReads|TestOfflineAchievementBatch|TestOfflineAuditPostgres|TestOfflineRecoveryPostgres|TestOfflineBackgroundPostgres' -count=1 -v
```

There is no new migration, save reset, reward compensation or client protocol
change. Deploy matching server/action-worker builds to use the recovery changes;
the shared audit pipeline is also used by other rebuilt workers. The existing
recovery schema/version rollout prerequisites below still apply. No deployment
was performed here. These improvements do not prove that an eight-hour complex
multi-actor backlog after worker downtime completes instantly. Representative
concurrent reconnect p50/p95 and real-device UI profiling remain rollout gates;
retain bounded chunks and historical resource fences until work actually commits.

## Background Settlement (2026-09-07)

### Return Correctness Follow-up (2026-09-08)

#### Review Fixes (2026-09-08)

- ACK is now guarded in PostgreSQL against open background windows and captured battle runs that have neither settled a reward receipt nor reached a terminal status. Previous-window attribution follows the same exclusion as the report query. Early/stale clients cannot discard late battle totals; normal terminal zero-drop battles can still be acknowledged.
- Terminal projection includes the player's changed equipment, attribute progress and derived equipment stats. Enhancement receipts do not necessarily contain an actor equipment-change flag, so the explicit enhancement results are checked too. A final-loadout ID alone does not trigger an equipment refresh when nothing changed. Projection uses deadline-bound reads and does not run the ordinary skill-configuration reconciliation path.
- Background preparation now reads and obeys `available_at`, returning the exact next retry deadline without executing an early chunk. The existing transaction/revision fence, retry limits and settlement budgets are unchanged.
- Expanded isolated PostgreSQL tests cover early ACK before late battle grants, open-window ACK rejection, background retry deadlines, and final player projection under database-enforced read-only transactions with an injected read failure. Focused offline executor, protocol, role projection and action-worker regression suites and server/action-worker builds passed. These are local component proofs, not production reconnect/load measurements.

- Local user 1's action receipt completed in 68 ms, but battle 373912 had been retrying compilation since the previous night because `player_skills.equipment_config_jsonb` was missing. After a local database backup, applying the existing `20260907_equipment_skill_config.sql` migration allowed the worker to finish normally. Startup recovery checks now include this battle dependency. No inventory, run outcome or receipt was manually rewritten. This does not compensate battles that never ran during that outage.
- Terminal recovery now reloads inventory without the warehouse cache and publishes its versioned patch before reporting snapshot synchronization. State/queue reads alone do not refresh inventory, and the initial bootstrap can precede a deferred final chunk. The synchronization key includes pending battle count as well as task/revision, so late battle settlement refreshes inventory without reloading it on unchanged polls. Projection errors keep the durable receipt unacknowledged.
- Committed recovery scheduling and battle completion wake the report reader after 250 ms rather than waiting for the five-second fallback poll. Hints never grant items or replace the durable receipt; concurrent reads and acknowledged tasks are guarded. The fallback remains for lost events. No unfinished-settlement banner or dialog is shown.
- Focused PostgreSQL recovery/background, HTTP protocol and action-worker regressions passed, including missing battle dependency rejection and late-battle snapshot invalidation. Fifteen frontend unit tests, typecheck, server build and isolated browser wake-up/nonblocking checks passed. The local gateway was updated and its health check passed; user 1's recovery gate is false and receipt 15 is acknowledged. Three review passes covered behavior, integration and operational safety. This is not a production concurrent-login latency guarantee.

Normal absence handling now runs in the existing action worker, not on the returning player's critical path. Disconnected users with active queues are processed roughly every 30-35 seconds with the existing worker concurrency, user lock, executor budgets and frozen operation snapshots. No offline inventory copy, average-reward approximation or per-player goroutine is introduced.

- A `background` recovery row is the cumulative receipt for an open absence. Its target advances in bounded batches, never beyond the original absence's configured cap. Interval completion does not resume online scheduling, shift action windows, or apply the final work loadout. Capped windows are parked.
- Return seals the same row under the user lock, captures in-flight battles, increments its revision and freezes the final target/resume time. Old background work cannot reopen or commit over that handoff. One bounded foreground tail is attempted before websocket binding finishes. Only final completion shifts capped action windows and applies the final loadout, exactly once.
- Open absences remain resource-fenced between background intervals as well: an HTTP mutation racing websocket authentication cannot change equipment or spend materials ahead of the remaining historical tail. Successful sealing/settlement removes that fence before normal websocket gameplay begins.
- Redis scheduling is rebuilt with keyset pages of at most 256 candidate users. Capped users still advance the scan cursor; an all-capped page cannot hide later users. `ZADD NX` never postpones an already scheduled online action. Empty queues are not discovered.
- The game shell requests state and queue immediately, independently of the offline report. Settlement never owns a fullscreen overlay. On return, one automatic report shows the complete absence's cumulative receipt, including prior background batches and the login tail. There is no toolbar entry. Pending battle receipts/final synchronization delay only the report; the game remains available. ACK follows report presentation, and repeated responses update the same report without reopening it after dismissal. Short absences with actual gains are also reported; empty brief reconnects do not produce a popup.
- Transport failures and the no-progress watchdog continue bounded polling automatically. Database/network/deadline failures retry with backoff and smaller step budgets; deterministic repeated errors or an executor that cannot advance still require repair. Report errors never assert that progress was saved without a committed receipt.
- This is not an unconditional promise of zero wait during server downtime or corrupted historical state. An unfinished historical resource-dependent transaction still protects inventory until it can finish safely. Do not remove those integrity checks, show uncommitted inventory, or report a queued mutation as successful.

Apply `20260907_offline_recovery.sql`, `20260908_background_offline_recovery.sql` and the current battle dependency `20260907_equipment_skill_config.sql` before starting matching server/action-worker/battle-worker versions. All three entry points check the required recovery dependencies at startup. The two recovery migrations were applied locally on 2026-09-07, and the missing battle dependency on 2026-09-08, after `pg_dump` backups; no production migration was performed.

Focused isolated PostgreSQL proof: `TestOfflineBackgroundPostgres*` covers committed interval inventory, checkpoint rollback, stale replay/presence, sealing, cap parking, capped-page pagination, final window parity against return-time settlement, and startup rejection of missing schema. An eight-hour synthetic 2-second generic queue completed 14,400 operations: 959 prior background chunks used about 3.39 seconds total store time, spread across the simulated absence; the returning tail contained 15 operations and took about 3.06 ms. A separate real transmute/food test completed 120 one-second operations across background and return, with a 30-operation tail around 106 ms, and verified exactly 121 prepaid starts/charge receipts. Ordinary actions keep their 2-second minimum; this is not a synthetic 1-second generic benchmark.

These are local store measurements, not authenticated end-to-end latency, eight-hour alchemy measurements, mobile device performance or concurrent reconnect percentiles. Representative multi-user production load and fault-recovery latency still need measurement.

## Contract

- Presence creates a PostgreSQL recovery window before advancing the last-seen state. Repeated binds for the same absence reuse that window. Later absences queue behind earlier unfinished work.
- The action worker advances open background absences; a bounded gateway dispatcher handles sealed recovery backlog. Both reuse existing action executors, preserving frozen effects, RNG/checkpoints, actor ordering and the configured step/time budgets.
- The user advisory lock and persisted revision fence each chunk. Inventory, queue state, cumulative summary and achievement enqueue commit together. An uncertain COMMIT is retried against the persisted revision, not assumed to have failed.
- Battle inventory and its `(run_id,user_id)` receipt commit together. Duplicate plans do not re-grant; conflicting plans fail. In-flight runs captured at reconnect remain in the summary after late settlement, and an earlier captured run is not reported again in a later absence.
- The offline duration cap limits action simulation, not battle loot already awarded. Combat receipts cover the actual absence through `resumed_at`, including battles that started after the action cap.
- Client protocol 3 polls compact progress at a bounded interval. Refreshing only reads the durable task. Terminal totals replace previous totals, never add them again. ACK is owner/revision-bound and occurs after receipt delivery; sending a websocket event is not ACK.
- Projection failures retain the terminal receipt for retry. Cumulative kitty, fishing and operation-effect changes refresh only at terminal reads. Ordinary inventory/state/queue snapshots refresh on terminal receipt rather than every chunk.
- The no-progress watchdog is informational and never stops polling. Normal background-completed returns are not gated on receipt display. Exceptional unfinished settlement still fences resource-dependent mutation; action queue commands recheck inside the user transaction. Other HTTP domain mutations retain their existing user locks and API precheck, not a new universal transaction fence.
- Completing a task persists `schedule_pending`; restarting after COMMIT but before Redis scheduling replays the scheduling intent. Redis is a wake-up hint, not the accounting authority.

## Deployment

1. Back up and apply `server/schema/migrations/20260907_offline_recovery.sql`, then `20260908_background_offline_recovery.sql` before new binaries. Both are additive and repeatable. No production migration is performed by this change.
2. Drain old gateway/action-worker/battle-worker processes and deploy all three from the same revision. Old workers do not honor the durable gate or write the new receipts. Do not perform a mixed-version rollout with active settlement.
3. Publish the new web build and require old open pages to reload. Protocol 1/2 clients do not implement durable ACK; their receipt lifecycle is not supported for continuing operation.
4. Start the new gateway dispatcher and workers. Exercise reconnect, browser refresh, late battle settlement, stalled retry and normal actions after completion in staging before admitting production traffic.
5. Rollback must first drain/resolve running and stalled tasks and preserve both new tables. Never drop receipts or roll back only the gateway while recovery workers are active.

## Verification

Run the isolated database proof against a local PostgreSQL account allowed to create temporary databases:

```sh
cd server
OFFLINE_RECOVERY_TEST_DATABASE_URL='postgres://game:game@127.0.0.1:5432/game?sslmode=disable' go test ./internal/store -run TestOfflineRecoveryPostgres -count=1 -v
```

The test creates a uniquely named database, loads the baseline, applies the migration twice, and drops only that temporary database. It covers checkpoint rollback after inventory writes, concurrent/stale chunks, refresh identity, owner-scoped ACK, delayed failure after a successful commit, bounded retries, queued absences, durable scheduling intent, real playing-run capture, receipt/log overlap, late settlement and cross-absence attribution.

Local measurement on 2026-09-07: an 8-hour synthetic 14,400-action wood queue, with a 1,000-step test budget, completed 15 committed chunks in approximately 56 ms of store calls including injected rollback/replay, using 275 SQL calls. This is NOT end-to-end latency, production throughput, battle simulation performance, or a percentile guarantee. Fishing, effects, enhancement, many actors and concurrent users require representative staging load tests. Production retains its existing 10,000-step/1-second budget.

Browser fixture: `web/tests/fixtures/offline-recovery-browser.mjs`. It never forwards game API/socket traffic. UI fixture proof and real database proof are separate; neither is authenticated production end-to-end proof.

## Effectful Gathering Optimization

Cost-free player collecting, mining and animal farming with fixed stackable rewards can settle stable segments in batches. Each segment has at most 512 completions inside the existing outer step/time budget. This removes per-operation SQL, not per-operation randomness: every logical start retains its original key, frozen snapshot and charge receipt, and every reward roll is still resolved individually. Experience is summed after rounding each original operation's gain.

Segments stop before effect activation/expiry, charge exhaustion, changed action duration or a level-changing completion. The existing sequential path handles those boundaries and refreshes derived stats before starting the successor. Legacy runs without a frozen snapshot remain effect-free for that original run. Handlers must explicitly declare start-selection stability; unknown kinds do not opt in. Kitty lanes, shared player/kitty charge pools, progress-ordering effects, material-funded actions, random/instance rewards and other executors retain their existing paths. Alchemy's server-frozen one-second duration remains supported; this optimization does not replace it with a two-second floor.

Run the real executor A/B and transaction/replay proof:

```sh
cd server
OFFLINE_RECOVERY_TEST_DATABASE_URL='postgres://game:game@127.0.0.1:5432/game?sslmode=disable' go test ./internal/store -run 'TestPrimaryWorkEffectBatchPostgres|TestOfflineRecoveryPostgres' -count=1 -v
```

The A/B test runs the same queue and deterministic effect identities through the original one-operation path (rolled back) and the optimized path (committed). It compares inventory, experience, charge receipts, final timestamps and frozen snapshots. One case is eight hours at 5000% collecting efficiency with food, two-second starts and capped collecting XP: 14,400 completions. A second case exercises upgrades, duration changes, future food activation, expiry, charge exhaustion and a legacy empty snapshot. A separate recovery test injects failure after rewards and effect receipts but before checkpoint commit, rebuilds the Store, retries and replays stale revisions, and verifies the final inventory against the cumulative receipt.

Local isolated PostgreSQL measurement on 2026-09-07 for the food/capped-XP case: approximately 41.96 seconds and 101,548 SQL calls sequentially versus 0.358 seconds and 237 SQL calls batched (about 117x faster). The boundary-heavy case improved from 131 ms / 742 SQL calls to 31 ms / 153 calls. These timers cover executor calls, queue writes and effect-receipt flush, not fixture setup, final COMMIT, post-commit audit processing, dispatcher scheduling or client delivery. They are not production end-to-end or concurrent-load guarantees, and do not measure battle or alchemy speedups.

Three review passes covered settlement correctness, executor/session/recovery integration, and query/operational safety. Focused effect, gathering, recovery rollback/replay and transmute one-second tests pass. The full backend suite still has two pre-existing failures: `TestLoadSpecializationCatalogSource` (catalog version expectation) and `TestArenaAcceptsProportionalCapacityStampAfterSerialization` (proportional capacity expectation). They were also reproduced against the unchanged baseline during the recovery work and are not changed here. Staging reconnect/load validation remains required before rollout.

## Other Executors And Combat Summary

The follow-up optimization preserves the existing executor algorithms and transaction fences:

- Enhancement memoizes immutable base attempt parameters by level inside one planner call only. Attempt RNG, prepaid costs, protection use, effect selection/receipts, failure resets, milestone stops and detailed results remain sequential and unchanged. There is no cross-request cache.
- Alchemy reads the four derived inputs directly from an available frozen work-loadout map instead of copying every attribute. For effectful conversion/transmutation, the next start reuses the base timing just resolved after experience settlement. It still selects its own food snapshot and reserves its own costs. The normal start path and other alchemy executors keep their existing resolution. No timing is reused across completions or attribute changes, and the one-second transmute effect is applied after the reused base timing.
- Fishing persists an existing batch of at most 128 catches in one SQL statement rather than 128 pipelined statements. The statement assigns IDs in input order, joins catches to their exact inventory instances, and returns results in input order. Released fish do not create inventory. Event/RNG progression, species records, tournament entries, scaleglow grants, catch retention and checkpoints are unchanged. Catch and inventory writes remain atomic with the enclosing recovery transaction.
- Combat summary SQL is unchanged apart from extraction into a testable constant. A proposed split-range query measured slower and was discarded. Existing user/time and run/user indexes already support the tested workload; no additional migration or cache is introduced.

Local isolated measurements on 2026-09-07, not end-to-end overnight latency:

| Component | Before | After | Scope |
| --- | --- | --- | --- |
| Enhancement planner | 21.1 ms | 12.2 ms | 14,400 attempts, includes planner reads but not final persistence/projection |
| Transmute next-start preparation | 151.5 ms / 899 SQL | 127.7 ms / 771 SQL | 128 starts plus insufficient-material boundary, including food exhaustion and changed speed |
| Conversion next-start preparation | 200.4 ms / 1,155 SQL | 174.9 ms / 1,027 SQL | Same 128-start proof with conversion costs |
| Frozen alchemy derived-input read | 13,347 ns / 26,856 B | 28 ns / 0 B | Microbenchmark with 256 unrelated attributes; not total alchemy settlement |
| Fishing catch/instance insert | 6.67 ms | 3.19 ms | 128 mixed kept/released catches; excludes later records/tournament writes and COMMIT |
| Combat reward query | Unchanged | 2.0-2.4 ms | 100,000 synthetic receipts across two users, about 5,000 selected receipts plus late IDs |

The enhancement test compares complete plans and charge receipts against a frozen pre-change implementation, including prepaid effects, shortage/protection and milestone stops. Fishing compares stored catch/instance data against the old pipelined INSERT, validates ordered identities, and injects foreign-key failure and transaction rollback. Alchemy compares every next-start payload, material totals and effect receipts, and asserts exactly one saved query per successful start. Combat expected totals are computed independently and cover duplicate captured IDs, previous-window attribution, other-user isolation and receipt/log overlap; `EXPLAIN ANALYZE` verifies the time index. Existing recovery rollback/replay, late battle settlement, conversion-food audience, fishing lifecycle, and one-second timing regressions pass. The full backend suite still has the two existing failures noted above.

```sh
cd server
OFFLINE_RECOVERY_TEST_DATABASE_URL='postgres://game:game@127.0.0.1:5432/game?sslmode=disable' go test ./internal/store -run 'TestOffline(Alchemy|Enhance|Fishing|Combat)|TestOfflineRecoveryPostgres' -count=1 -v
go test ./internal/store -run '^$' -bench BenchmarkOfflineAlchemyDerived -benchmem
```

These are bounded component optimizations, not a claim that every enhancement, alchemy, or fishing workload now completes in under a second. Complex alchemy still follows its existing sequential/effect-dependent path. Full overnight multi-actor and concurrent-reconnect latency still needs staging measurement. No production deployment, automatic historical compensation, frontend behavior change, or new migration is included in this follow-up. The original recovery deployment requirements still apply.

## Monitoring And Retention

Track chunk elapsed time/steps, oldest running age, stalled count, retries, pending scheduling, database lock waits and pool saturation. Compare p50/p95 recovery completion for overnight workloads under expected concurrent reconnect traffic. The dispatcher has bounded concurrency; do not increase concurrency without measuring database contention.

```sql
SELECT status, count(*), min(created_at) AS oldest
FROM player_offline_recoveries GROUP BY status;
SELECT id, user_id, revision, attempts, error_code, available_at
FROM player_offline_recoveries WHERE status='stalled' ORDER BY id LIMIT 100;
SELECT count(*) FROM player_offline_recoveries WHERE schedule_pending;
```

No automatic deletion is introduced. Archiving must preserve the latest per-user deduplication window, all unfinished/unacknowledged receipts, captured-run attribution and battle receipt unique keys. Do not reuse the warehouse-log or replay retention cleanup for these tables.

Historical warehouse logs are best-effort and cannot prove whether missing historical loot was actually credited. This change does not automatically compensate old losses. Investigate the affected player's inventory, run settlement records and retained logs before any state-bound, idempotent compensation. Existing achievement events remain UNLOGGED and existing audit logs remain best-effort; this change does not turn those systems into permanent ledgers.
