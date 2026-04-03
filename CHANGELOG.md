# Changelog

All notable changes to this project will be documented in this file.

## [0.1.1] - 2026-04-03

Polish/sync release to align the public repo surface with the released research baseline.

### Added

- MIT `LICENSE` at repo root.
- `RELEASE_NOTES_v0.1.1.md` for public-surface consistency release notes.

### Changed

- Synced `README.md` status/version language to `v0.1.1` polish/sync scope.
- Clarified roadmap/checklist wording for settled licensing and release hygiene.

### Claim Boundaries (Unchanged)

- FSVM remains the contribution under test.
- Seed-only calibration remains falsified.
- Structural calibration via adjacency width remains demonstrated.
- Threshold second-axis work remains future work.
- No tokenizer replacement, transformer replacement, or agent-superiority claims are introduced.

### Next Work

- `0.1.1` is polish-only.
- Science-facing expansion is deferred to `0.2.0`.

## [0.1.0] - 2026-04-03

Initial public research-baseline release.

### Added

- `VERSION` file for release identification.
- `RELEASE_NOTES_v0.1.0.md` for the release narrative and scope.
- `docs/RELEASE_CHECKLIST.md` for repeatable release hygiene.
- GitHub Actions CI workflow running the canonical `make ci` check.

### Changed

- Rewrote `README.md` to make the current thesis, evidence, non-claims, and reading order explicit.
- Re-scoped `BUILD_ORDER.md`, `CONFORMANCE_TARGETS.md`, and `IMPLEMENTATION_GAPS.md` as subsystem-specific / historical documents instead of top-level repo story.

### Proven In This Release

- The FSVM remains the contribution under test.
- Seed-only calibration is falsified.
- Structural calibration via adjacency width is demonstrated.
- Width is the demonstrated locality-sensitivity axis.

### Not Claimed In This Release

- Threshold-based second-axis calibration is not yet demonstrated.
- No tokenizer replacement, transformer replacement, or agent-superiority claims are made.
