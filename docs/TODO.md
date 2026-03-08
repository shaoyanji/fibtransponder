# TODO

## Runtime delta/queue scaffolding (spec-conformance pass)
- [x] Align mechanical constants/fields for `internal/deltaqueue/{types,flags,stubs}.go`
- [x] Add compile-safe stubs without semantic behavior
- [x] Keep/annotate `TODO(spec)` where frozen docs are ambiguous
- [ ] Resolve `DerivedDelta.StepsSince` spec mismatch between appendix and queue spec
- [ ] Define nil `ClassifierState` contract for `Classify`
- [ ] Implement predicate semantics for `Revalidate*` (spec-only, no guessing)

## Marker / Rosetta layer
- Define marker payload (what is checkpointed)
- Define probe semantics under retrospective dilation `r++`:
  - log2 bounds: update equations in terms of (k_max, r)
  - modular fingerprints: what is N(r)? how to update residues cheaply when r changes?

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

## Proof sketches
- Bounded work per input symbol (ingest)
- Termination/monotonicity properties for dilation counter and marker emission
- DoS resistance argument: all heavy work is budgeted + can degrade output
