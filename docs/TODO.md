# TODO

## Runtime delta/queue scaffolding (spec-conformance pass)
- [x] Align mechanical constants/fields for `internal/deltaqueue/{types,flags,stubs}.go`
- [x] Add compile-safe stubs without semantic behavior
- [x] Keep/annotate `TODO(spec)` where frozen docs are ambiguous
- [x] Resolve `DerivedDelta.StepsSince` spec mismatch between appendix and queue spec
  - **Resolution (2026-03-13):** Constants appendix is authoritative for classifier→delta interface surface. Queue spec omits `StepsSince` because it defines the queue-side contract only, not the classifier output shape. Field is confirmed required on `DerivedDelta`.
- [x] Define nil `ClassifierState` contract for `Classify` — returns minimal delta, no panic
- [x] Implement classifier core (`Classify` in `classifier.go`, 0 allocs, ~55ns/op)
- [x] Define queue state contracts (`MemFrontier`, `MemOpLog`, `FixedQuota` in `queue_state.go`)
- [x] Implement predicate semantics for `Revalidate*` (`revalidate.go`, 6 predicates)
- [x] Implement `ApplyDelta` + `ApplyDeltaTarget` + `ApplyDeltaMerge` (`apply.go`)
- [x] Implement all 8 invariant conformance tests (`conformance_test.go`)
- [x] Implement all 12 benchmark variants (classifier + queue)
- [ ] Wire CI targets for conformance + benchmarks

## Chore — Codex mechanical-only pass (deferred)
- [ ] Run Codex with the mechanical guardrails prompt (spec-to-code only; no semantic invention)
- [ ] Scope strictly to:
  - `internal/deltaqueue/types.go`
  - `internal/deltaqueue/flags.go`
  - `internal/deltaqueue/stubs.go`
  - `internal/deltaqueue/spec_conformance_test.go`
- [ ] Require output summary:
  1. files changed
  2. mechanical changes made
  3. TODO(spec) items left unresolved
  4. compile issues introduced/fixed
- [ ] Accept only if:
  - compile is improved/passing for target package
  - spec names align with frozen docs
  - no semantic drift introduced
- [ ] Ignore for this pass:
  - `fib_semantic.md`
  - bridge behavior
  - promotion logic
  - performance tuning
  - "making it smart"
- [ ] Smallest next action:
  - Run Codex on those 4 files, then inspect diff before accepting

## Marker / Rosetta layer
- Define marker payload (what is checkpointed)
- Define probe semantics under retrospective dilation `r++`:
  - log2 bounds: update equations in terms of (k_max, r)
  - modular fingerprints: what is N(r)? how to update residues cheaply when r changes?

## Proprioceptive feedback (done 2026-04-23)
- [x] EMA trackers for dilateRate, markerRate, sketchDrift
- [x] Calibration rules with hysteresis deadband
- [x] Convergence detection (drift < ε, dilate ≈ 0, stable)
- [x] Safety caps: width ∈ [1,5], threshold ≥ 4
- [x] Integration with StepWord64 (zero overhead when disabled)

## Sketch v2 (done 2026-04-23)
- [x] 8 precomputed HashFamilies with large odd multipliers
- [x] mixSketch avalanche mixer: RotateLeft64(sketch*A + B, R)
- [x] Rich folding: zeroRun, R, seeds, event salts
- [x] Rolling SketchDelta: per-step bit-change tracker
- [x] StepV2 / StepWord64V2 exact semantic preservation
- [x] NewWithFamily(id): independent families per transponder

## Rich features (done 2026-04-23)
- [x] Descriptor: 256-bit local feature vector (4×uint64)
- [x] Extractor: rolling 64-bit window, 8 sub-regions
- [x] Haar-like responses: density, transitions, Haar-X, Haar-Y
- [x] Distance metrics: L1, CosineSimilarity
- [x] FeatureBuffer: ring buffer + nearest-neighbour matching
- [x] Integration wrappers: StepWithExtractor, StepWord64WithExtractor, StepV2WithExtractor, StepWord64V2WithExtractor
- [x] Benchmarks: Extract ~820ns, Distance ~15ns, StepWithExtractor ~97ns

## Segmentation automaton
- Implement small NFA for allowed cuts at candidate markers (started: `internal/segauto`)
- Define deterministic exemplar extraction rules for UI

## Ingest model
- Decide whether input is raw bits, spike times, or both via adapter interfaces
- Implement adapter layer without changing core

## Signal decomposition / transforms
- Windowing over the rope (fixed-size and marker-aligned)
- Boolean→bipolar conversion (`0→-1, 1→+1`) and mean-centering
- FFT adapter (radix-2) OR external library hook
- Walsh–Hadamard transform (WHT) on power-of-two windows
- Autocorrelation at a small set of lags
- Multiscale / fractal summaries (box-counting on 2D embedding)

## Benchmarks
- Microbench: ingest cost per bit, per dilation event
- Stress: adversarial stream with frequent `11` and long zero runs
- Render budget tests (ensure renderer cannot stall ingest)
- Rich feature stress: high-event-rate stream descriptor churn

## Proof sketches
- Bounded work per input symbol (ingest)
- Termination/monotonicity properties for dilation counter and marker emission
- DoS resistance argument: all heavy work is budgeted + can degrade output
- Sketch-v2 avalanche property (formal proof)
- Proprioceptive convergence (periodic input → O(p) convergence)
- Descriptor distance metric validity (L1 satisfies triangle inequality)
