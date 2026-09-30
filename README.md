# fibtransponder

[![Go](https://img.shields.io/badge/Go-1.25+-00ADD8?logo=go)](https://go.dev/)
[![Status](https://img.shields.io/badge/status-v0.1.1%20public%20surface%20sync-blue)](#status)
[![Spec](https://img.shields.io/badge/spec-docs%2FSPEC.md-blue)](docs/SPEC.md)
[![License](https://img.shields.io/badge/license-MIT-green.svg)](LICENSE)

`fibtransponder` `v0.1.1` is a public-surface consistency release on top of the `v0.1.0` research baseline for a lower-level tokenization / semantic substrate experiment built around a Fibonacci-radix streaming state machine. The repo is intended as a testable substrate for lower-level stream sensing relevant to tokenization and semantic harness research, not as a finished model stack or product layer.

## Canonical thesis

- The FSVM is the contribution.
- Seed-only calibration is falsified.
- Adjacency width is an **ordered threshold**, not an independent detector axis.
- Threshold is a planned second axis, **untested** — no markers fire on current corpora.

## What this repo currently proves

- A deterministic FSVM can ingest a bitstream with bounded per-step work while tracking dilation, zero-run markers, and a state sketch. See `docs/SPEC.md` and `docs/BENCHMARKS.md`.
- Seed-only calibration does not create detector diversity. Different Zobrist seed tables change sketch identity, but not event structure. See `REPORT_CORPUS.md`.
- Varying adjacency width changes the class sensitivity ranking (a prose-first to code-first shift). See `REPORT_STRUCTURAL.md`.
- The v1 sketch now uses its full 64-bit range. An earlier revision added the 6-bit window directly to the seed, which left the upper 56 bits a function of bit counts alone: 20000 random streams produced only 256 distinct sketches, colliding after 18 streams. `fsvm.SketchTerm` now spreads the window with an odd multiplier, giving ~19713 distinct.

## What is not claimed

- **Width is not an independent axis.** `Width1/2/3` test 1-runs of length >= 2/3/4, so the event sets are nested and dilation counts are non-increasing in width for *every* input. This is asserted on 2000 randomized streams by `TestStructuralCalibration`. No "orthogonal axes", "multi-detector sensor", or "2-dimensional parameter space" claim is made. The earlier version of this README and of `REPORT_STRUCTURAL.md` did make one; both are withdrawn.
- **Threshold calibration is untested, not merely unproven.** Marker counts are zero across all 9 (width, threshold) configurations on all three corpora, so no independence conclusion can be drawn. `TestSecondAxisCalibration` asserts this precondition.
- **The sketch has no collision guarantee.** It is a parity function over a 128-symbol alphabet, so ~1.5% of random streams of length 1..200 collide and an all-zero stream yields only 2 distinct sketches at any length. The former "2^-64 collision probability" figure was wrong and is withdrawn. See `docs/SPEC.md` §7 "Not claimed".
- `classify_results.json` is generated output, is not tracked, and must not be cited: it has no train/test split, its `n_transponders` field is never assigned, and `cmd/classify` imports no fibtransponder package. See the header of `cmd/classify/main.go`.
- This release does not claim tokenizer replacement, transformer replacement, or agent superiority.
- This release does not claim that the current sketch is a sufficient semantic identity mechanism on its own.
- This release does not claim broad convergence or proprioceptive control results beyond what is directly documented in this repo.

## Reading order

1. `README.md`
2. `docs/SPEC.md`
3. `HANDOFF_VISION.md`
4. `REPORT_STRUCTURAL.md`

Then read:

- `REPORT_CORPUS.md` for the seed-only falsification result
- `docs/BENCHMARKS.md` for baseline performance context

## Status

`v0.1.1` is a polish/sync release that keeps the `v0.1.0` claim boundaries intact while aligning public-facing docs and licensing. It packages no new scientific result. Second-axis threshold work remains future work and is not included in the claims of this release.

## Canonical checks

The boring release check is:

```bash
make ci
```

That runs:

- `go vet ./...`
- `go test ./... -count=1 -timeout=120s`
- `go test ./internal/deltaqueue/ -v -run "TestInvariant|TestClassifier" -count=1 -timeout=60s`

For direct local testing:

```bash
go test ./... -count=1
```

## Repo guide

- `docs/SPEC.md` — source-of-truth FSVM semantics and explicit open questions
- `HANDOFF_VISION.md` — canonical research-direction document
- `REPORT_CORPUS.md` — evidence that seed-only calibration is falsified
- `REPORT_STRUCTURAL.md` — evidence that structural calibration via width is demonstrated
- `docs/BENCHMARKS.md` — baseline performance notes
- `docs/RELEASE_CHECKLIST.md` — release hygiene checklist for this repo

Historical or subsystem-specific documents:

- `BUILD_ORDER.md` — historical implementation order for the `internal/deltaqueue` sidecar work
- `CONFORMANCE_TARGETS.md` — `internal/deltaqueue` conformance and benchmark targets
- `IMPLEMENTATION_GAPS.md` — historical gap log for the `internal/deltaqueue` sidecar
- `HANDOFF.md` — implementation handoff packet for the `internal/deltaqueue` subsystem

## Release scope

This release is for public-surface sync and release hygiene. It is not a feature-expansion release. The FSVM hot path is kept intact, and unproven second-axis experiments are intentionally left out of `v0.1.1`. The next science-facing line is `0.2.0`.
