# Handoff Packet: deltaqueue Implementation Patterns

Scope: `internal/deltaqueue/` — classifier, queue state, revalidation, apply, conformance, benchmarks.

For: spec implementors, Codex passes, or future contributors who need to interop with the existing scaffold.

---

## Architecture Summary

The deltaqueue package implements the semantic sidecar ABI for a delta/queue engine. It sits between the FSVM (core bit-stream machine) and the heap runtime (frontier management). Four layers:

```
FSVM → CoreDelta → Classifier → DerivedDelta → ApplyDelta → MemOp log
```

The classifier is a pure function. The queue is a decision gate. The heap runtime is external.

---

## Non-Negotiable Invariants

These are conformance-tested. Breaking them breaks the contract.

### 1. Classifier is allocation-free
- `Classify()` makes **zero heap allocations**. Verified by `TestInvariantClassifierIsAllocationFree`.
- All output lives on the stack. No slices, no maps, no interface boxing.
- Budget: <5µs/op enforced by `TestInvariantClassifierCostBudget` (actual: ~55ns).

### 2. Classifier update order is frozen
```
1. Increment StepsSince lanes (saturating at 0xFFFF)
2. Derive flags from CoreDelta step-truths
3. Apply suppress/reset on stream discontinuity
4. Reset StepsSince lanes on matching events (only if not suppressed)
5. Compute AuxBuckets (recency/novelty/stability/urgency)
6. Update Sketch (Zobrist fold)
7. Emit DerivedDelta
```
Do not reorder. Do not add steps between 1-7 without spec review.

### 3. Core and derived flag namespaces are separate
- Core flags (`flags.go` first block): step-truths from FSVM only.
- Derived flags (`flags.go` second block): classifier-computed hints only.
- They share some bit positions **intentionally** (e.g., `FlagWindowChanged=0x10` core, `FlagViewCanonicalityBroken=0x10` derived). Different namespaces, same bit value.
- The classifier must **never** copy a core flag verbatim into derived. Tested by `TestInvariantClassifierHintOnly`.

### 4. Revalidation before mutation
Every `MemOp` emitted by `ApplyDelta` must pass the corresponding `Revalidate*` predicate. The queue is a decision gate, not a mutation engine. Tested by `TestInvariantQueueRevalidatesBeforeAct`.

### 5. MemOp log is append-only
`MemOp` is a value type. Once created, its fields never change. The op log is the source of truth for frontier derivation. Tested by `TestInvariantLogIsAppendOnly`.

### 6. Tombstoning removes from view, persists in log
`frontier.Remove(id)` + `log.AddTombstone(id)`. The tombstone outlives the frontier entry. Tested by `TestInvariantDeletionIsViewRemoval`.

### 7. Score snapshot at append time
`MemOp.Score` captures the frontier score at the moment the op is created. It is **not** a live reference. This is by design for append-only replay.

---

## Code Conventions

### Naming
- `Revalidate*` predicates: named after the operation they gate (Add, Promote, Demote, Tombstone, Merge, MaterializeEligible).
- `Apply*` functions: `ApplyDelta` (nil-safe entry), `ApplyDeltaTarget` (per-item gate), `ApplyDeltaMerge` (parent-gated merge).
- `pack*` / `classify*` helpers: internal to classifier, not exported.

### Value types over pointers
- `MemOp`, `Score`, `AuxBuckets`, `CoreDelta`, `DerivedDelta` are all value types.
- `ClassifierState` is pointer because the classifier mutates it in-place.
- `MemFrontier`, `MemOpLog` are pointer-backed (map internals) but satisfy interface contracts.

### TODO(spec) vs TODO
- `TODO(spec)`: spec ambiguity that needs authorial resolution. Block implementation.
- `TODO`: implementation gap with clear spec. Don't block, just incomplete.
- Never invent semantics when a `TODO(spec)` is present. Ask or leave it.

### Threshold design
- Revalidation predicates handle **structural** validity (item exists, not tombstoned, quota available).
- Score-threshold comparisons (promotion threshold, demotion threshold, materialization threshold) are **caller responsibility**. The predicate gates don't know your policy.
- This keeps the predicates simple and the policy layer flexible.

### AuxBuckets encoding
Four 8-bit ordinals packed MSB→LSB into uint32: `[Recency|Novelty|Stability|Urgency]`.
- `packAuxBuckets()` / unpack via shifts.
- Recency: log-scale, higher = fewer steps since last event.
- Novelty: inverted StepsSince[StateChange].
- Stability: high when PatternRepeat set with no state change.
- Urgency: 0x80 baseline for candidates, 0xFF on discontinuity.

---

## Test Patterns

### Conformance tests live in `conformance_test.go`
- Invariant tests: `TestInvariant*`
- Classifier variants: `BenchmarkClassifyVariants/*`
- Queue variants: `BenchmarkQueueVariants/*`

### Classifier tests live in `classifier_test.go`
- Functional: `TestClassifierStepsSinceOrdering`, `TestClassifyPatternRepeat`
- Allocation: `TestClassifyAllocationFree`
- Benchmark: `BenchmarkClassify` (generic)

### Revalidation/Apply tests live in `revalidate_test.go`
- Per-predicate: `TestRevalidate*`
- Per-flow: `TestApplyDelta*`
- Quota: `TestFixedQuota`

### Test discipline
- Fresh `ClassifierState` per test case. State bleed causes false PatternRepeat.
- Use `t.Fatalf` for invariant violations (stop immediately).
- Use table-driven patterns when testing multiple flag combinations.

---

## Benchmark Interpretation

| Benchmark | What it measures | Expected |
|-----------|-----------------|----------|
| `zero_stream` | Classifier with no flags set | ~100ns, 0 allocs |
| `dense_transition_stream` | StateChange every tick | ~50ns, 0 allocs (StepsSince reset is cheap) |
| `checkpoint_stream` | Checkpoint every 8 steps | ~83ns, 0 allocs |
| `periodic_checkpoint_stream` | Mixed flags on cycle | ~72ns, 0 allocs |
| `*reprioritization*` | ApplyDeltaTarget with promote/demote | ~200-270ns, 1 alloc (map lookup) |
| `stale_hint_flood` | Target mostly missing items | ~48ns, 0 allocs (fast rejection) |
| `tombstone_density` | 50% tombstone rate | ~200ns, 0-1 allocs |

If classifier benchmarks show allocs >0, something boxed. Find and fix.

---

## Known Gaps (do not silently fill)

1. **No dedicated core flag for marker emission.** Classifier proxies via `FlagCheckpointCrossed`. See `classifier.go` TODO(spec).
2. **Nil `ClassifierState` contract.** Currently returns minimal delta with no panic. Behavior beyond that is unspecified.
3. **Promote/Demote/Tombstone/Materialize candidate flags** are not set by the classifier. They require queue state context (policy layer). The classifier reserves the flag bits but never emits them.
4. **Frontier replay from op log.** `TestInvariantFrontierIsDerivable` tests the concept with a toy replay. Real replay needs the full op sequence and score history.
5. **Sketch is informational.** The Zobrist sketch detects divergence but has no formal collision bound. Don't treat it as cryptographic.

---

## Files Quick Reference

| File | Purpose |
|------|---------|
| `types.go` | CoreDelta, DerivedDelta, ClassifierState, Score, interfaces |
| `flags.go` | All flag constants (core + derived), ZobristSeed, suppress values |
| `classifier.go` | `Classify()`, `classifyAux()`, AuxBuckets packing, StepsSince helpers |
| `revalidate.go` | 6 revalidation predicates |
| `apply.go` | `ApplyDelta`, `ApplyDeltaTarget`, `ApplyDeltaMerge` |
| `queue_state.go` | `MemFrontier`, `MemOpLog`, `FixedQuota`, `NewQueueState` |
| `stubs.go` | Reference pointer to real implementations |
| `classifier_test.go` | Classifier functional + allocation tests |
| `revalidate_test.go` | Predicate + ApplyDelta flow tests |
| `conformance_test.go` | 8 invariant tests + 12 benchmark variants |
| `spec_conformance_test.go` | Stub (content moved to conformance_test.go) |
| `heap_abi.go` | Heap runtime ABI (separate concern, not wired to classifier) |

---

## How to Add a New Revalidation Predicate

1. Add function to `revalidate.go` following the pattern: `(id, frontier, log, ...quota) → bool`.
2. Check structural validity only (exists, not tombstoned, quota). Leave thresholds to caller.
3. Add test in `revalidate_test.go`: fresh state → valid, missing → invalid, tombstoned → invalid.
4. If it emits a new `MemOpKind`, add the constant to `types.go` and wire it in `apply.go`.
5. Add conformance invariant test if the predicate guards a documented invariant.

---

## How to Add a New Derived Flag

1. Add constant to `flags.go` second block (derived flags). Pick an unused bit position.
2. Add derivation logic to `Classify()` in `classifier.go` (step 2).
3. Document what core flags or state transitions trigger it.
4. If it resets a StepsSince lane, add the lane constant and reset logic (steps 1/4).
5. Add test verifying: correct flag set on trigger, zero when trigger absent.
6. Update `TestInvariantClassifierHintOnly` if the new flag's bit collides with a core flag (add to comment).

---

## Philosophy (carried from session)

- Spec-first, code follows. Don't invent semantics.
- Allocation-free is a hard constraint for the classifier, not a suggestion.
- Revalidation gates are cheap — prefer rejecting stale/invalid ops over handling them.
- Value types preserve append-only log immutability.
- Tests are conformance proofs, not coverage theater.
- Benchmark regressions are real regressions. Track them.

---

Generated: 2026-03-13 19:51 CET
Session: full BUILD_ORDER completion (9/9 items)
Commits: 1fcf2b2, 1661b6b, 381dd16, 65a6294, 19e347b
