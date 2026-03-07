# IMPLEMENTATION_GAPS

Scope: mechanical gap inventory against `delta-queue-spec (2).md` and `delta-classifier-constants (4).md`.

## Present (skeleton only)
- `CoreDelta`, `DerivedDelta`, `ClassifierState`, `AuxBuckets`, `Subscription` type skeletons.
- `MemOpKind`, `MemOp`, `Score` type skeletons.
- Flag constant sets (core + derived).
- API stubs: `Classify`, `ApplyDelta`, `Revalidate*`.
- Skipped conformance/benchmark test names from spec.

## Missing implementation (TODO, no guessing)
1. `Classify` logic (allocation-free behavior, suppress/reset behavior, sketch update order).
2. `DerivedDelta.Aux` packing/unpacking helpers for `AuxBuckets`.
3. `QueueState` concrete shape.
4. `FrontierIndex` contract.
5. `OpLog` contract.
6. `PromotionQuota` contract.
7. `ApplyDelta` idempotent translation + revalidation wiring.
8. All `Revalidate*` predicate logic with O(1)/O(log n) guarantees.
9. Frontier derivation/replay support from append-only log.
10. Materialize-eligible handoff set wiring.

## Spec consistency TODOs
- `delta-queue-spec (2).md` defines `DerivedDelta` without `StepsSince`.
- `delta-classifier-constants (4).md` conformance text references `DerivedDelta.StepsSince[0]`.
- TODO: reconcile this before implementing `TestClassifierStepsSinceOrdering` semantics.

## Out of scope (kept intentionally unimplemented)
- JIT materialization internals.
- Policy engine internals.
- Semantic identity logic beyond queue boundary.
