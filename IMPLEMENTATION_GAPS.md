# IMPLEMENTATION_GAPS

Status: historical gap inventory for the `internal/deltaqueue` sidecar work.
Scope: mechanical gap inventory against `delta-queue-spec (2).md` and `delta-classifier-constants (4).md`. This should not be read as the top-level roadmap for the repo's research baseline release.

## Present (skeleton only)
- `CoreDelta`, `DerivedDelta`, `ClassifierState`, `AuxBuckets`, `Subscription` type skeletons.
- `MemOpKind`, `MemOp`, `Score` type skeletons.
- Flag constant sets (core + derived).
- API stubs: `Classify`, `ApplyDelta`, `Revalidate*`.
- Skipped conformance/benchmark test names from spec.

## Missing implementation (TODO, no guessing)
1. ~~`Classify` logic~~ (done 2026-03-13 — see `classifier.go`)
2. ~~`DerivedDelta.Aux` packing/unpacking helpers~~ (done 2026-03-13 — `packAuxBuckets`/`classifyAux` in `classifier.go`)
3. ~~`QueueState` concrete shape~~ (done 2026-03-13 — `MemFrontier`, `MemOpLog`, `FixedQuota` in `queue_state.go`)
4. ~~`FrontierIndex` contract~~ (done 2026-03-13 — interface + `MemFrontier`)
5. ~~`OpLog` contract~~ (done 2026-03-13 — interface + `MemOpLog`)
6. ~~`PromotionQuota` contract~~ (done 2026-03-13 — interface + `FixedQuota`)
7. ~~`ApplyDelta` idempotent translation + revalidation wiring~~ (done 2026-03-13 — `apply.go`)
8. ~~All `Revalidate*` predicate logic~~ (done 2026-03-13 — `revalidate.go`)
9. Frontier derivation/replay support from append-only log.
10. Materialize-eligible handoff set wiring.

## Spec consistency TODOs
- ~~`delta-queue-spec (2).md` defines `DerivedDelta` without `StepsSince`.~~
- ~~`delta-classifier-constants (4).md` conformance text references `DerivedDelta.StepsSince[0]`.~~
- **Resolved 2026-03-13:** Constants appendix is authoritative for classifier→delta interface. `StepsSince` field is required on `DerivedDelta`. Queue spec omits it because it defines the queue-side contract only.

## Out of scope (kept intentionally unimplemented)
- JIT materialization internals.
- Policy engine internals.
- Semantic identity logic beyond queue boundary.
