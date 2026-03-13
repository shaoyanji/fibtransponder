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

4. ~~Implement queue state contracts~~ (done 2026-03-13)
   - `MemFrontier`: map-backed FrontierIndex with Insert/Remove/SetMaterialized
   - `MemOpLog`: map-backed OpLog with tombstone tracking
   - `FixedQuota`: counter-based PromotionQuota with overflow guard
   - `NewQueueState()` constructor, exported fields
   - `revalidate.go`: all 6 predicates implemented
   - `apply.go`: ApplyDelta, ApplyDeltaTarget, ApplyDeltaMerge

5. ~~Implement revalidation predicates~~ (done 2026-03-13, bundled with #4)

6. ~~Implement `ApplyDelta`~~ (done 2026-03-13, bundled with #4)
   - Idempotent translate via revalidation gates
   - Score snapshot at append time
   - Merge via ApplyDeltaMerge (parent-gated)

7. Implement conformance test (TODO)
   - `TestClassifierStepsSinceOrdering` per appendix text.

8. Implement benchmark suites (TODO)
   - `BenchmarkClassify/*`
   - `BenchmarkQueue/*`

9. Wire CI targets (TODO)
   - Ensure conformance + benchmark targets can run reproducibly.
