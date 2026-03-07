# CONFORMANCE_TARGETS

Source documents:
- `delta-classifier-constants (4).md`
- `delta-queue-spec (2).md`

## Tests

### `TestClassifierStepsSinceOrdering`
- Source: constants appendix section "Required conformance test".
- Status: scaffolded and skipped.
- TODO: implement after `DerivedDelta.StepsSince` ambiguity is resolved.

## Classifier Benchmarks (required)

- `BenchmarkClassify/zero_stream` — scaffolded/skipped
- `BenchmarkClassify/dense_transition_stream` — scaffolded/skipped
- `BenchmarkClassify/checkpoint_stream` — scaffolded/skipped
- `BenchmarkClassify/periodic_checkpoint_stream` — scaffolded/skipped

## Queue Benchmarks (required)

- `BenchmarkQueue/low_reprioritization_small_frontier` — scaffolded/skipped
- `BenchmarkQueue/high_reprioritization_small_frontier` — scaffolded/skipped
- `BenchmarkQueue/low_reprioritization_medium_frontier` — scaffolded/skipped
- `BenchmarkQueue/high_reprioritization_medium_frontier` — scaffolded/skipped
- `BenchmarkQueue/frequent_meld` — scaffolded/skipped
- `BenchmarkQueue/rare_meld` — scaffolded/skipped
- `BenchmarkQueue/stale_hint_flood` — scaffolded/skipped
- `BenchmarkQueue/tombstone_density` — scaffolded/skipped

## Invariant Targets (implementation TODOs)
- `ClassifierHintOnly`
- `CoreFlagsAreStepTruths`
- `ClassifierIsAllocationFree`
- `ClassifierCostBudget`
- `QueueRevalidatesBeforeAct`
- `LogIsAppendOnly`
- `DeletionIsViewRemoval`
- `FrontierIsDerivable`
