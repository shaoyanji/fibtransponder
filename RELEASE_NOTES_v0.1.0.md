# Release Notes — v0.1.0

## Title

`v0.1.0` — Research Baseline

## Release intent

This release makes the repo presentable as a public research baseline. It is a packaging, alignment, and reproducibility release, not a feature-expansion release.

## Canonical thesis for this release

- The FSVM is the contribution.
- Seed-only calibration is falsified.
- Structural calibration via adjacency width is demonstrated.
- Width selects locality sensitivity.
- Threshold remains planned second-axis work, not a result claimed here.

## What a new reader should take away

`fibtransponder` is a testable substrate for lower-level stream sensing relevant to tokenization and semantic harness research. The repo currently supports a narrow but defensible claim set: the FSVM is the core object of study; changing seeds alone does not produce detector diversity; changing adjacency width does produce materially different sensitivity profiles.

## Evidence base

- `REPORT_CORPUS.md`: seed-only calibration falsified.
- `REPORT_STRUCTURAL.md`: structural calibration via width demonstrated.
- `docs/SPEC.md`: current FSVM semantics and explicit open questions.
- `HANDOFF_VISION.md`: canonical research-direction document.
- `docs/BENCHMARKS.md`: baseline performance context.

## Deliberate non-goals for v0.1.0

- No new second-axis threshold experiments.
- No expansion of scientific claims beyond the existing evidence.
- No tokenizer replacement or transformer replacement claims.
- No broad refactor of the FSVM hot path.

## Release hygiene included

- README rewritten around proven results and non-claims.
- Initial changelog entry added.
- Visible CI workflow added.
- Release checklist added.
- Top-level doc hierarchy clarified.

## Known limits and follow-up

- Threshold as a second calibration axis is next work, not done work.
- The sketch should not be read as a stand-alone semantic identity proof.
- Public licensing was unresolved at the time of this `v0.1.0` note. This was resolved in `v0.1.1` by adding MIT `LICENSE`.
