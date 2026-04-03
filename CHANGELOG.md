# Changelog

All notable changes to this project will be documented in this file.

## [0.1.1-dev]

- Post-release polish and public-surface fixes only.
- Public surface sync, badge/link cleanup, and feedback-driven clarity fixes.
- No new scientific claims are introduced in this development window.

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
