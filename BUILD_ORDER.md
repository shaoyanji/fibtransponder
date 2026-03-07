# BUILD_ORDER

Scope: mechanical order only, following existing spec text.

1. Freeze the interface surface (done)
   - Keep type and function signatures aligned with spec names.

2. Resolve one spec ambiguity (TODO)
   - Reconcile `DerivedDelta.StepsSince` mention in constants appendix vs struct in interface spec.

3. Implement classifier core (TODO)
   - `Classify(core, cls)` with update order from spec.
   - Keep allocation-free constraint.

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
