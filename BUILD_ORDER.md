# BUILD_ORDER

Scope: mechanical order only, following existing spec text.

1. Freeze the interface surface (done)
   - Keep type and function signatures aligned with spec names.

2. ~~Resolve one spec ambiguity~~ (done 2026-03-13)
   - Constants appendix is authoritative for `DerivedDelta.StepsSince`.
   - Queue spec omits it by design (queue-side contract only).

3. ~~Implement classifier core~~ (done 2026-03-13)
   - `Classify(core, cls)` implemented in `classifier.go`
   - Allocation-free: 0 B/op, ~55ns/op
   - Update order: StepsSince increment → derive flags → suppress/reset → AuxBuckets → Sketch
   - Conformance test `TestClassifierStepsSinceOrdering` passing
   - Allocation-free test passing

4. Implement queue state contracts (TODO)
   - Define `QueueState`, `FrontierIndex`, `OpLog`, `PromotionQuota` concrete shapes.

5. Implement revalidation predicates (TODO)
   - `RevalidateAdd`, `RevalidatePromote`, `RevalidateDemote`, `RevalidateTombstone`, `RevalidateMerge`, `RevalidateMaterializeEligible`.

6. Implement `ApplyDelta` (TODO)
   - Translate hints to `MemOp` append operations.
   - Enforce idempotent stale-hint behavior via revalidation.

7. Implement conformance test (TODO)
   - `TestClassifierStepsSinceOrdering` per appendix text.

8. Implement benchmark suites (TODO)
   - `BenchmarkClassify/*`
   - `BenchmarkQueue/*`

9. Wire CI targets (TODO)
   - Ensure conformance + benchmark targets can run reproducibly.
