# Release Checklist

Use this checklist before tagging a public release.

## Docs

- [ ] `README.md` matches the current proven results and current non-claims.
- [ ] `docs/SPEC.md` still reflects the canonical FSVM semantics.
- [ ] `HANDOFF_VISION.md` remains the canonical research-direction document.
- [ ] Evidence docs (`REPORT_CORPUS.md`, `REPORT_STRUCTURAL.md`) are referenced, not overstated.
- [ ] Historical or subsystem-specific docs are clearly scoped as such.

## Verification

- [ ] `make ci` passes.
- [ ] Benchmarks are referenced accurately from `docs/BENCHMARKS.md` and are not reinterpreted beyond what they show.
- [ ] Release notes are written for the exact tag being cut.

## Packaging

- [ ] `VERSION` matches the intended release tag.
- [ ] `CHANGELOG.md` has an entry for the release.
- [ ] `RELEASE_NOTES_<version>.md` exists and matches the release scope.
- [ ] CI workflow is present and visible in `.github/workflows/`.
- [ ] `LICENSE` is present and matches the project licensing policy.

## Release action

- [ ] Review `git diff` for accidental scientific broadening.
- [ ] Create and push the release tag.
- [ ] Publish release notes with the tag.

## Open TODOs

- [ ] Confirm claim boundaries are unchanged from the prior science-facing release.
