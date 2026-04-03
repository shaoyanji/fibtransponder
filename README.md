# fibtransponder

[![Go](https://img.shields.io/badge/Go-1.25+-00ADD8?logo=go)](https://go.dev/)
[![Status](https://img.shields.io/badge/status-v0.1.0%20research%20baseline-blue)](#status)
[![Spec](https://img.shields.io/badge/spec-docs%2FSPEC.md-blue)](docs/SPEC.md)

`fibtransponder` `v0.1.0` is a public research-baseline release for a lower-level tokenization / semantic substrate experiment built around a Fibonacci-radix streaming state machine. The repo is intended as a testable substrate for lower-level stream sensing relevant to tokenization and semantic harness research, not as a finished model stack or product layer.

## Canonical thesis

- The FSVM is the contribution.
- Seed-only calibration is falsified.
- Structural calibration via adjacency width is demonstrated.
- Width selects locality sensitivity.
- Threshold is a planned second axis, not yet a proven result.

## What this repo currently proves

- A deterministic FSVM can ingest a bitstream with bounded per-step work while tracking dilation, zero-run markers, and a cheap state sketch. See `docs/SPEC.md` and `docs/BENCHMARKS.md`.
- Seed-only calibration does not create detector diversity. Different Zobrist seed tables change sketch identity, but not event structure. See `REPORT_CORPUS.md`.
- Structural calibration is real when geometry changes. Varying adjacency width changes class sensitivity ranking, including a prose-first to code-first shift across widths. See `REPORT_STRUCTURAL.md`.
- Adjacency width already acts as a locality-sensitivity selector. Width is the demonstrated control axis in this release.

## What is not claimed

- This release does not claim threshold-based structural calibration. Threshold is next work, not current evidence.
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

`v0.1.0` is a research baseline release. It packages the current FSVM-centered thesis, the falsification of seed-only calibration, and the demonstrated structural result from adjacency width. Second-axis threshold work is explicitly next and is not included in the claims of this release.

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

This release is for packaging, alignment, and reproducibility. It is not a feature-expansion release. The FSVM hot path is kept intact aside from release hygiene, and unproven second-axis experiments are intentionally left out of `v0.1.0`.
