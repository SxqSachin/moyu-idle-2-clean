# Exact Offline Batch Engine

Status: approved target architecture, 2026-09-08; ordinary statistical settlement
implemented locally on 2026-09-10 after explicit sampler/migration approval.
The legacy-compatible conversion/transmutation slice remains separate. The
general cross-domain engine is not complete. No production deployment is included.

### Ordinary Statistical Slice

The approved ordinary queue version includes independent equipment and food
procs. It activates at the next unstarted operation, keeps current prepaid
legacy work intact, and preserves its new random stream across worker splits.
See [design, rollout and verification](../backend/ordinary-statistical-settlement.md).
The admission flag defaults off until every action-capable binary is upgraded.

### Implemented First Slice

`server/internal/store/offline_alchemy_segment.go` compiles reward parameters,
plans stable intervals, evaluates exact single-operation random draws and XP
rounding in memory, and persists inventory/XP once per touched template or
attribute. It does not loop around the original executor. Food starts retain
individual receipts; only the final successor uses the original start adapter.

Admission is limited to frozen-effect player conversion/template transmutation,
disjoint cost/output templates and ordinary stackable rewards. It stops before
level changes, changed food selection and other running lanes (including ties).
Unknown/shared-charge/instance/feedback contracts retain the existing path.
The existing no-effect aggregate sampler is deliberately NOT replaced.

This slice keeps legacy entropy policy and payloads, rather than introducing
V2 random state. Tests supply identical per-ordinal random seeds to old and new
paths; production still uses independently time-seeded operation streams. It
does not claim deterministic replay of uncommitted legacy randomness or the
same counterfactual wall-clock seeds after changing execution speed. A V2 RNG
or sampler change still requires the separate versioning gates below.

The recovery transaction, revision fence, step/time budgets, achievement
aggregation and atomic effect receipts remain authoritative. No new persisted
format is produced, so rollback to the previous executor needs no migration.
See [implementation evidence](../backend/offline-recovery.md#exact-alchemy-segments-2026-09-08)
for measured scope and remaining rollout requirements.

## 1. Decision

Replace operation-by-operation business execution with a dependency-aware batch
engine. Its normal unit is a stable logical interval, not one completed action.

The target pipeline is:

```text
durable recovery window / existing user lock
  -> bounded load of relevant authoritative state
  -> decoded, typed settlement working set
  -> plan stable semantic segments
  -> algebraic / random / stateful batch kernels
  -> typed mutation journal and resumable checkpoint
  -> bulk persistence + recovery receipt in one transaction
  -> committed projection/outbox notification
```

Do not implement this as a loop around `SettleFastForwardTx(..., 1)`, even if the
loop is moved to a worker or some SQL reads are cached. Kernels must not call
PostgreSQL, Redis, HTTP, wall-clock RNG, or the original full start/settle handler
per operation. JSON is decoded on admission and encoded on persistence, not on
each virtual start.

Not every result can be reduced to a constant-time formula. The architecture
eliminates repeated business orchestration and I/O; it preserves indispensable
random draws and state transitions in specialized in-memory batch algorithms.
Producing N distinct instances or N required receipts still has O(N) output cost.

## 2. Current Constraints Verified In Code

- `action_queue.go`: queue selection, executor calls, running-row updates and
  completion handling share the user transaction. Certain operation effects
  and shared charge pools force a one-iteration executor budget. Replacing
  that condition without a dependency proof is unsafe.
- `action_effect_batch.go`: cost-free primary work already has a restricted
  stable-segment optimization. Its eligibility, duration/effect checks and
  level boundary behavior are useful reference contracts, not a general engine.
- `store_alchemy.go`: individual settlement seeds RNG from wall-clock time.
  Conversion has an aggregate sampler; transmutation switches from exact draws
  to buckets at 128 runs and uses an average lucky multiplier on that path.
  Thus arbitrary regrouping is not currently guaranteed to preserve outcomes.
- `action_executor_enhance.go`: a stateful attempt planner already separates
  much calculation from persistence and keys attempt RNG by operation identity.
  Its success/failure/material/milestone behavior must survive extraction.
- `action_executor_fishing.go`: phases, RNG, supplies, catches, records and
  tournament writes are related but distinct; catches already have bounded
  bulk insertion. A catch is not equivalent to a completed queue action.
- `operationeffect/session.go`: start selection, grant ordering, remaining
  charges, committed operation identities and snapshots are domain authority.
- `offline_recovery.go`: the persisted absence window, revision fence, summary,
  background-to-return seal, cap and scheduling intent already exist.
- `docs/backend/battle-room-runtime.md`: combat is owned by room/run workers;
  the action recovery engine must not synthesize extra battles during downtime.

Use these implementations and `docs/backend/offline-recovery.md` as current
authority. Older numeric budgets in `action-queue-design.md` are historical.

## 3. Correctness Contract Before Performance

### Reference And Equality

For the same initial state, target time, rule versions, random inputs and
legacy semantic grouping/ordering, compare the existing authoritative path
against the new kernel and persistence adapter.

Compare all of the following, not just aggregate inventory:

- Queue status, counts, cancellation reasons, lane handoffs, partial current
  run, start/end timestamps and final deferred work-loadout application.
- Template and instance inventory, archived/unarchived consumption order,
  custom/affix exclusions, equipment ownership and reserved inputs.
- Per-operation rounding, XP acceptance/caps, levels, derived values, battle
  points, level-triggered side effects and relevant lock-version increments.
- Food selection/order, charge usage, grant validity, current frozen snapshots,
  start keys, immutable historical receipts and the prepaid next operation.
- Enhancement attempts/milestones, fishing phase/cast progression, retained and
  released catches, fish identity/metadata, records, knowledge and tournaments.
- Achievement event identities and meanings, reward provenance, cumulative
  summary, late battle receipt attribution and presentation/ACK lifecycle.

Already committed inventory and receipts are never recalculated or replaced.
Existing instance IDs remain stable. New instances are compared by stable source
operation/output lineage; ID remapping is allowed in tests only when proven not
to affect RNG, ordering, references or any other observable behavior.

### No Silent Semantic Repairs

The current offline implementation is the migration oracle. Do not replace its
ordering with an idealized online replay and call the difference a correction.
If two existing paths disagree, document the precise mismatch and isolate the
affected compatibility mode. Probability fixes and new gameplay rules require
their own approval/version, not an incidental performance change.

Batch partition invariance is a required property for newly admitted exact
kernels: splitting at an execution checkpoint must not change the final result.
It is not assumed for legacy aggregate samplers. Their semantic group boundaries
must remain separate from physical CPU/database chunk boundaries.

## 4. Ownership And Module Shape

Introduce a small pure package, proposed name `server/internal/actionsettlement`.
It may depend on immutable catalog/domain rule types and `operationeffect`, but
must not depend on `store`, database interfaces or gateway transport types.

| Owner | Responsibility | Forbidden responsibility |
| --- | --- | --- |
| Existing recovery coordinator | Absence identity, target/cap, seal, ownership, retry, schedule | Reward formulas |
| Store load adapter | User lock, relevant state reads, legacy decoding | Virtual per-operation queries |
| Batch planner | Dependencies, compatible ordering, segment horizon | Alchemy/fishing-specific branches |
| Domain kernels | Exact batch math and domain transitions | SQL, notifications, serialization |
| Working set/journal | Read-your-writes, reservations, dirtiness, local rollback | Global mutable caches |
| Store commit adapter | Bulk writes, receipts, outbox and checkpoint transaction | Re-running simulation |
| Gateway/projection | Committed snapshots, cumulative report and ACK | Authoritative settlement |

Conceptual kernel API, subject to type extraction rather than a new generic DSL:

```go
type BatchKernel interface {
    Dependencies(CompiledAction) Footprint
    Plan(ReadView, Horizon) (SegmentPlan, error)
    Evaluate(*WorkingSet, SegmentPlan, RandomInput, Budget) (BatchResult, error)
}
```

`SegmentPlan` includes the kernel/rule identity, lane, start ordinal, maximum
safe operation range, boundary reasons, ordering domain and expected outputs.
`BatchResult` reports completed operations separately from attempted starts,
internal transitions, outputs and the continuation checkpoint.

Extract existing formulas into the pure kernels; do not maintain independent
online/offline copies. An online call may evaluate one operation while retaining
its existing wall-clock scheduling policy. Sharing the math must not accidentally
give online work the offline catch-up policy.

## 5. Working Set And Transactional Journal

Load only data in the union of admitted kernels' dependency footprints:

- Active/pending lanes and directly referenced instances/loadouts.
- Necessary attributes, specialization contribution and building state.
- Commodity stack metadata for relevant templates, including eligible archived
  rows; lazily acquire newly needed templates at a segment boundary.
- Frozen current operation state and effect grants intersecting the relevant
  start window, including shared pools and passive/furniture timelines.
- Fishing/enhancement state needed by those kernels, not unrelated warehouse
  detail or all historical audit logs.

The working set is local to one bounded transaction. It provides typed lookup,
reservations, attribute progression and journal marks. A failed start restores
all changes since its mark: materials, effect charges, instance reservations,
generated outputs and event records. This is the in-memory counterpart of the
existing transactional start boundary, not just a map of final deltas.

Inventory conservation must be checked at every relevant prefix. A positive
final balance does not prove intermediate starts were affordable. Preserve stack
deletion/recreation, instance selection order and grant normalization; netting a
consume and later grant into zero must not preserve a row that used to disappear.
Overflow, capacity and protected/custom-instance rules still apply before any
accepted operation, even if final net quantities fit.

Attributes need a dedicated compatibility arithmetic adapter. Current repeated
`roundFloat(..., 4)` is the oracle. Integer XP units are only an implementation
option after equivalence is proven over valid magnitudes and boundary cases;
replacing repeated rounding by `gain * count` is not automatically equivalent.

Only dirty state is persisted. Preserve meaningful per-operation version deltas
and derived-state side effects when collapsing physical UPDATE statements.

### Compile Once, Evaluate Many

Compile action/effect definitions when their owning rule artifact is loaded:
resolve template/attribute indices, fixed material vectors, stable reward-table
ordering, formulas and effect selectors. Own compiled plans by the immutable
artifact/Store lifetime; no unbounded process-global cache keyed by player data.

For each stable segment, select its effect rules once, resolve modifiers and
derived inputs once, and hold grant references by ID. Do not rebuild a grant map,
copy every rule's JSON parameters or retain one full effect snapshot object per
virtual start. A compact start-range description can be expanded into the
existing per-operation receipts at persistence, without changing their authority.
The current in-flight successor retains its full frozen snapshot.

Use packed result columns and reducers instead of allocating a result map for
every random draw. Required detail is emitted into bounded output pages; optional
debug traces are sampled, not generated for every production operation. A
compatibility kernel may keep scalar arithmetic, but never the old heavyweight
per-operation handler/serialization path.

## 6. Segment Planning

### Stable Segment

A segment is the largest prefix whose required calculation contract stays
stable. Its limit is the earliest of:

1. Target/cap horizon or remaining requested operations.
2. A level/attribute change that affects a successor, dependency or observer.
3. Effect activation/expiry, exclusivity selection change or charge exhaustion.
4. Material affordability, storage/capacity, durability or reservation boundary.
5. Queue completion, required loadout switch or a domain milestone.
6. A conflicting other lane's next relevant event, under the existing order.
7. Bounded compute/output capacity, with a legal resumable continuation.

First complete the existing frozen run using its own duration/effects/prepayment;
do not reinterpret it as a fresh run of the segment. Subsequent starts use their
actual logical timestamps and half-open effect intervals `[validFrom,validUntil)`.

For a constant recurrence, time-derived completion count can be calculated once.
An upgrading completion is a boundary operation: settle it, update attributes,
then derive successor timing. Do not merely mark a level dirty until the end of
the whole absence. If a level cannot affect any observable result in the range,
coalescing it requires an explicit domain proof, not an assumption.

The final partial successor is real state. For N completed operations there may
be N+1 accepted starts, materials reserved and food charges consumed. Completion
counts cannot be reused as start counts.

### Batch Capability Instead Of A Boolean Gate

Replace broad per-start fallback with explicit capabilities on admitted handlers:

- Stable selection/parameters over a start range.
- Read/write dependencies and the state that invalidates those parameters.
- RNG consumption semantics and whether internal results feed later operations.
- Effect/grant range representability and required output detail.
- Stop conditions and continuation support.

Reuse/extend the current `StableStartCount` and domain registry rather than
putting food kinds into the queue scheduler. An unknown handler/effect declines
optimization with a diagnostic reason. CI must require an explicit batch policy
for new content so performance cannot silently collapse to one SQL loop per run.

## 7. Three Batch Kernels

### Algebraic Batch

Use for stable fixed costs/rewards/duration where exact aggregation is proven.
Calculate the safe count, total resources, compatible rounded XP and ending
timeline directly. Work should scale with boundaries and changed state, not N.

Example: 10,000 identical fixed-output starts without a level/effect/resource
boundary become one segment result, not 10,000 calls to the action executor.
The actual persisted partial successor/prepayment is still handled explicitly.

### Exact Random Batch

Use for stable probabilities/parameters with no unaccounted causal feedback.
Compile tables once, evaluate the original random contract in a tight batch
kernel, and reduce results into totals/prefix summaries/output columns.
Avoid per-operation maps, payloads, catalog scans and handler dispatch.

An aggregate random sampler is permitted only when it is already the applicable
legacy contract or when exact equivalence is established. Equal expected value
or a similar histogram is not enough. A vector of original draws may still cost
O(N); that is fundamentally different from N transactions/business executions.

### Stateful Batch

Use for feedback-driven enhancement, resource reuse and fishing transitions.
Keep dependencies in a compact local state machine. Precompute invariant data,
emit columnar results and stop/replan only at genuine contract changes.

Do not infer cost from final net inventory when random output can finance the
next start. Use an exact prefix affordability calculation where possible;
otherwise preserve that local recurrence. No unproved probability shortcut,
success-count substitution or matrix approximation replaces a causal history.

## 8. Multiple Lanes And Shared Resources

Represent dependencies at least by template balance, specific instance,
attribute/owner XP, effect grant/pool, equipment/loadout and domain progression.
Most lanes share inventory; a different actor ID alone does not establish
independence. A kitty can affect player XP through a passive effect.

The planner may extend/reorder a segment only if all affected operations commute
and have no relevant observer, ordering or random-stream dependency. Otherwise
respect the current lane selection and tie breakers, including current null-end
handling. A universal chronological heap would be a behavior change where the
existing implementation legitimately settles a whole lane prefix first.

Shared food pools are allocated in the existing logical start order. Do not give
an entire batch to one lane and compensate the other lane later. Periodic
interleaved starts may eventually use an exact merged-range algorithm, but this
requires a separate proof of charge allocation and key ordering.

Initial implementation remains serial within a user. Parallelize independent
users through a bounded worker pool before attempting intra-user parallelism.

## 9. Randomness, Rules And Compatibility

This is a release gate, not a detail to defer until after optimization.

1. Capture explicit random input in the reference harness: seeds/draw streams,
   legacy group sizes, rule versions and all exogenous time inputs. A wall-clock
   seed from an unexecuted past run cannot be reconstructed or promised identical.
2. Existing committed results/receipts are immutable. Frozen in-flight operation
   inputs and prepaid costs are imported without rerolling or retroactive effects.
3. Preserve existing legacy sampler/group semantics where required. Physical
   execution can yield within an admitted semantic group only if its exact RNG,
   reducer and domain continuation can be checkpointed. Never change group size
   just because a database batch filled up.
4. Persist an engine/checkpoint version, selected rule manifest and explicit RNG
   continuation for newly admitted V2 work. Keep algorithm and draw order when
   reproducibility allows it; do not substitute a new RNG algorithm for speed.
5. Prefer existing stable operation identities for deterministic domains such
   as enhancement. Any new counter-based random scheme is a separate future
   ruleset decision, not part of the compatibility claim.
6. Legacy workloads whose randomness/grouping cannot be proven compatible stay
   on a fenced legacy adapter until drained or explicitly version-migrated.
   Record coverage and fallback reasons; do not report them as optimized.

The initial reference deliverable must list any conflict between 'same current
offline outputs' and 'arbitrary partition-invariant replay'. Do not silently
resolve this in favor of nicer mathematics or a different distribution.

Catalog/rule retention must survive a process restart. A hash without the
corresponding retained artifact is insufficient. Follow the current frozen/live
resolution points for imported work; do not retroactively freeze the entire
historical absence under new rules. Reject unavailable compatibility versions
rather than recomputing a historical checkpoint with current defaults.

## 10. Bulk Persistence And Durability

### First Implementation: Existing Lock, Bounded Transaction

Keep the established user lock and recovery revision fence. Load a bounded
working set, evaluate bounded segments, then persist:

1. Inventory/instance lifecycle and exact attribute/equipment changes.
2. Queue/current-run state, reservations, effect charges and receipts.
3. Domain outputs: catches, milestones, records and tournament changes.
4. Achievement events and any required durable projection/outbox records.
5. Cumulative recovery summary, checkpoint, revision and scheduling intent.

Commit all five together. Only committed state may become spendable or visible
as final recovery rewards. An uncertain COMMIT is resolved by reading persisted
identity/revision, never by assuming failure and granting again.

Use parameterized multi-row/array writes or appropriate staging/COPY for large
outputs. The goal is bulk SQL by table/chunk, not merely a pipeline of thousands
of individual business statements. Preserve duplicate-key semantics: identical
targets in one UPSERT statement cannot be assumed to behave like sequential
UPSERTs. Normalize only proven-equivalent journals or split at such conflicts.

Keep existing per-operation effect receipt authority initially; generate/insert
rows in bulk. Contiguous range compression is a later storage optimization and
requires equivalent membership lookup, overlap prevention, partial retry and
old-reader behavior. Do not drop uniqueness or use an approximate summary as
the evidence that food was consumed.

Required instance output is inserted in bounded pages with stable source
lineage and exact returned-ID mapping. The source operation/index, not a retry's
arrival time, identifies a generated result. If IDs affect a legacy computation,
allocate/map them before that computation under the compatibility contract.

No per-operation notifications or full player snapshots. Project once for a
committed change boundary; maintain the existing cumulative final report, late
battle receipt refresh and owner/revision-bound ACK. The UI does not wait for
replaying operation animations or rendering an unbounded list of details.

### Optional Later Phase: Compute Outside The Transaction

Only consider this after the in-transaction engine is measured. It requires a
complete read-set version fence across inventory, equipment, attributes, effects,
domain state and configuration, honored by every writer. Recovery revision alone
does not detect all conflicting mutations. Shared battle/resource writes matter.

Use bounded ownership/leases to avoid duplicate expensive computation and a
short validated atomic commit. Version conflicts discard the plan and retry
from authoritative state. Do not add optimistic execution before these fences
exist; short in-memory computation under the existing lock is the safer first
high-performance architecture.

## 11. Budgets And Scheduling

Separate three quantities:

- Semantic operations completed: player-visible accounting.
- Compute work: boundary count, random/state transitions and elapsed CPU budget.
- Output work: dirty rows, receipts, instances and encoded bytes.

A fixed 10,000-completion limit should not permanently force an algebraic
100,000-operation range into ten transactions. Conversely, 10,000 generated
instances cannot be treated as equally cheap. Replace the old step ceiling only
for validated new kernels, using measured compute/output budgets; retain it for
legacy handlers and retain hard safety ceilings throughout migration.

Yields occur at resumable semantic boundaries. Persist partial semantic-group
continuation where supported; cancellation/error cannot publish a half-applied
working set. Limits and tuning must not change RNG grouping or gameplay results.

Reuse the current bounded worker infrastructure. Give returning tails priority
without starving background work, apply per-user fairness/backoff and cap global
DB/CPU/output concurrency. Background settlement uses the same engine; increasing
its frequency is not the optimization. Stop capped windows and keep Redis as a
wake-up index, not an accounting source.

## 12. Domain Admission Matrix

| Domain | Initial kernel | Mandatory boundary/proof | Not a shortcut |
| --- | --- | --- | --- |
| Fixed gathering/farming | Algebraic | XP rounding/levels, timing/food, owner XP | Cost-free assumption for all actions |
| Fixed recipe crafting/forging | Algebraic or exact random | Prefix affordability, instances/inheritance, refunds | Netting away prepaid starts or consumed identities |
| Conversion/transmutation | Exact random/stateful; preserve legacy aggregate modes | RNG/group compatibility, gold/input feedback, XP, food | Wider batches that silently change sampler |
| Enhancement | Stateful | Every attempt result, protection, reset, material shortage, milestones | Sampling final success counts instead of attempt history |
| Building upgrades | Stateful over upgrade levels | Changing cost/duration/unlocks and derived effects | Treating different levels as identical repetitions |
| Fishing | Stateful phases plus bulk outputs | Cast seed/checkpoint, gear/supplies, XP, retention, records/tournaments | Average catch yield or skipping retained instances |
| Alchemy bench/smelt/inherit | Stateful/output-oriented | Carrier slots, selected instances, inherited custom state | Combining distinct instance crafts into item totals |
| Combat | Existing battle compiler/room worker | Run-level replay, carry state and authoritative reward receipts | Creating extra runs, accelerating online teammates or double-granting loot |

Combat is a separate optimization track. Recovery consumes its receipts and
respects shared mutations; action batching must not change room pacing or
simulate battles the current system would not have run.

## 13. Migration And Implementation Sequence

### A. Freeze The Oracle And Performance Baseline

Build real data fixtures for every domain and record reference inputs/results,
randomness modes, semantic group shapes and SQL/CPU/output costs. Resolve the
legacy RNG compatibility inventory before enabling new production behavior.

### B. First Complete Vertical Slice

Implement working set, journal, bulk commit, checkpoint and comparison harness
with player conversion/transmutation. Include levels, expiring/charged food,
prepayment, material exhaustion and output that funds a later start. Deliver the
whole recovery path, not only a fast isolated reward function.

### C. Expand Stable Batch Coverage

Admit fixed gathering, crafting and forging, then dependency-aware multi-lane
work. Replace the blanket effectful-one-step gate only for proven capabilities.
Exercise player/kitty shared inventory and food before declaring this phase done.

### D. Stateful Domains And Output Pipeline

Extract the enhancement planner, fishing phases and instance alchemy. Preserve
their rich output contracts. Profile and optimize mandatory output persistence
after the engine has removed per-operation orchestration.

### E. Controlled Rollout And Optional Optimism

Add a versioned recovery/checkpoint decoder and per-kernel rollout gate. New
metadata migrations must be additive/idempotent; do not rewrite old receipts or
drop compatibility data. Roll out matching workers with the current recovery
deployment prerequisites. Unknown versions fail closed at admission/startup.

Run sampled pure shadow comparison without a second database writer or second
reward grant, first offline and then at a strictly bounded staging/canary rate.
Do not double all production settlement cost indefinitely.

Before the first V2 commit, failure can return to the legacy path after rollback.
After a V2 checkpoint exists, use its pinned compatible engine; do not switch a
half-finished RNG/history to the old algorithm. A rollback must keep a reader/
worker capable of draining V2 work, or explicitly pause it without losing state.

## 14. Acceptance Gates

### Correctness

- Golden A/B equality for the complete contract in section 3, with real catalog
  fixtures and fixed random inputs, not only synthetic constant wood rewards.
- New exact-kernel partition tests: uninterrupted, randomly split chunks, process
  restart, repeated delivery, sealed background tail and cap handoff yield the
  same result. Legacy group-sensitive modes retain their explicit restrictions.
- Failure injection after reservations, random evaluation, instance writes,
  effect receipts, achievement/outbox insertion and checkpoint write; test lost
  COMMIT response, stale workers, savepoint-equivalent start cancellation and
  callbacks after COMMIT.
- Multi-actor shared food/materials, tied timestamps, output-financed starts,
  level changes, future effects, archived/custom stacks, exhausted supplies,
  canceled queues, malformed snapshots and unavailable rule versions.
- Late battle receipts, concurrent mutations, multiple tabs and final report
  deduplication without extra grants or an early resource-unlock fence.

### Structural Performance

- Pure-kernel tests reject any database/transport dependency.
- Stable algebraic fixtures do not call a handler once per operation; increasing
  repetitions by 10x leaves work proportional to boundaries/changed entities.
- Statement/round-trip counts follow loaded tables and bounded output batches,
  not the number of simple virtual starts. Track rows/bytes too, not just SQL count.
- No per-operation payload JSON, full attribute construction or catalog scans.
- Peak working-set/output memory is bounded independently of total absence.
- Record optimized coverage and fallback reasons; a feature that silently falls
  back on every food-equipped player is not an accepted batch implementation.

### Workload And Latency

Benchmark 8-hour capped absences, short return tails and worker-downtime backlogs
for each admitted domain, with and without food, growing and capped XP, multiple
actors, plentiful and nearly exhausted materials, and small/large output volume.
Run cold/warm tests and concurrent reconnects under a recorded fixed CPU/RAM/DB
configuration; include a representative constrained-host run.

Measure load, plan, evaluate, persist, COMMIT, projection, network/bootstrap and
browser/report readiness separately. Report p50/p95/p99, CPU time, allocations,
peak memory, SQL/rows/bytes, lock waits, queue age and database pool saturation.

The aspirational target is an order-of-magnitude reduction for the currently
I/O-bound overnight alchemy path, with no regression to already-fast algebraic
work. This is a design target, not a measured result or an unconditional
millisecond guarantee. Establish numeric latency gates from the baseline on the
chosen host before implementation is called complete. Complex instance output
and combat must be reported separately, not hidden behind a single fast average.

## 15. Explicit Non-Goals

- No expected-value rewards, unapproved sampler changes or lost detailed output.
- No giant transaction for an entire absence or unbounded background goroutines.
- No GPU/WASM/new service/database replacement before profiling the pure engine.
- No second source of inventory truth or client-side authoritative simulation.
- No schema/data reset, automatic compensation or mixed-version reward writers.
- No blanket promise that all causal/random histories can be evaluated in O(1).

## 16. Design Review Record

September 8 follow-up: existing ordinary-work and fishing batches now reuse
operation metadata, decoded life-action parameters and original RNG storage.
Fishing knowledge/records use set-based persistence and plain scaleglow uses one
inventory merge with unchanged individual events. These are locally verified
working-set improvements, not completion of the general feedback/instance
engine below. See [Offline Recovery](../backend/offline-recovery.md) for exact
scope, full-state differential fixtures and measurement limits.

Feature review: checked full settlement state, partial/prepaid successors, exact
prefix affordability, rounding, domain outputs, report timing and failed starts.

Integration review: checked current ordering/effect authority, shared player/
kitty resources, battle ownership, worker/recovery revisions, rule retention and
nontrivial post-checkpoint rollback compatibility.

Operational review: separated semantic groups from physical batches, bounded
memory/I/O/concurrency, retained transactional authority, and made fallback
coverage and end-to-end measurements explicit rollout gates.

Outstanding general-engine gates: extend the sampler/group inventory and ordered
working set to the remaining special executors; measure staging reconnect
concurrency and production-representative latency. Ordinary work now has its own
versioned random state, bounded inventory adapter and retained rules, not a
replacement for every stateful domain. Its fixtures do not waive these broader gates.
