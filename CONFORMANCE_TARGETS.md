# CONFORMANCE_TARGETS

Status: subsystem-specific reference for `internal/deltaqueue/`.
This file tracks queue/classifier conformance and benchmark targets. It is not a summary of the repo's primary research claims; use `README.md`, `HANDOFF_VISION.md`, `REPORT_CORPUS.md`, and `REPORT_STRUCTURAL.md` for the release-level story.

Source documents:
- `delta-classifier-constants (4).md`
- `delta-queue-spec (2).md`

## Tests

### `TestClassifierStepsSinceOrdering`
- Source: constants appendix section "Required conformance test".
- Status: **implemented** (2026-03-13) — per-lane increment/reset/saturation verified.

## Classifier Benchmarks (implemented 2026-03-13)

- `BenchmarkClassifyVariants/zero_stream` — ~99 ns/op, 0 allocs
- `BenchmarkClassifyVariants/dense_transition_stream` — ~50 ns/op, 0 allocs
- `BenchmarkClassifyVariants/checkpoint_stream` — ~83 ns/op, 0 allocs
- `BenchmarkClassifyVariants/periodic_checkpoint_stream` — ~72 ns/op, 0 allocs

## Queue Benchmarks (implemented 2026-03-13)

- `BenchmarkQueueVariants/low_reprioritization_small_frontier` — ~222 ns/op, 1 alloc
- `BenchmarkQueueVariants/high_reprioritization_small_frontier` — ~273 ns/op, 1 alloc
- `BenchmarkQueueVariants/low_reprioritization_medium_frontier` — ~238 ns/op, 1 alloc
- `BenchmarkQueueVariants/high_reprioritization_medium_frontier` — ~254 ns/op, 1 alloc
- `BenchmarkQueueVariants/frequent_meld` — ~248 ns/op, 1 alloc
- `BenchmarkQueueVariants/rare_meld` — ~245 ns/op, 1 alloc
- `BenchmarkQueueVariants/stale_hint_flood` — ~48 ns/op, 0 allocs (fast rejection)
- `BenchmarkQueueVariants/tombstone_density` — ~203 ns/op, 0-1 allocs

## Invariant Targets (implemented 2026-03-13)

All 8 invariant conformance tests implemented in `conformance_test.go`:

| Invariant | Test | Status |
|-----------|------|--------|
| `ClassifierHintOnly` | `TestInvariantClassifierHintOnly` | ✅ |
| `CoreFlagsAreStepTruths` | `TestInvariantCoreFlagsAreStepTruths` | ✅ |
| `ClassifierIsAllocationFree` | `TestInvariantClassifierIsAllocationFree` | ✅ |
| `ClassifierCostBudget` | `TestInvariantClassifierCostBudget` | ✅ (54 ns/op, budget 5µs) |
| `QueueRevalidatesBeforeAct` | `TestInvariantQueueRevalidatesBeforeAct` | ✅ |
| `LogIsAppendOnly` | `TestInvariantLogIsAppendOnly` | ✅ |
| `DeletionIsViewRemoval` | `TestInvariantDeletionIsViewRemoval` | ✅ |
| `FrontierIsDerivable` | `TestInvariantFrontierIsDerivable` | ✅ |
