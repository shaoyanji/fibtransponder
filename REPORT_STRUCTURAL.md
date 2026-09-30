# Structural Calibration Report

**Date:** 2026-03-14 (verdicts revised after the v1 sketch fold fix and after
`internal/transponder/structural_test.go` gained real assertions)
**Experiment:** Fixed seeds (DefaultSeeds), varying adjacency width: w=1, w=2, w=3
**Corpus:** Same 3 classes as seed experiment (prose 8056b, code 9480b, synthetic 12288b)
**Window:** 2048 bits

> **Regenerate with:** `go test ./internal/transponder/ -run TestStructuralCalibration -v`
>
> The event-rate tables below are unchanged — width does not touch the sketch.
> What changed is the *interpretation*: the widths are nested thresholds, not
> independent detectors. See §4.


---

## 1. Sensitivity Matrix

DILATE rate per (width × class):

| width | prose | code | synthetic | most-sensitive |
|---|---|---|---|---|
| w=1 | **0.1992** | 0.1763 | 0.0313 | prose |
| w=2 | 0.0582 | **0.0609** | 0.0000 | code |
| w=3 | 0.0079 | **0.0150** | 0.0000 | code |

## 2. Falsification Results

**OBS A:** Dil-rate ordering is not *strictly* monotonic across widths.
w=1 ranks prose > code. w=2 and w=3 rank code > prose.
→ Note: the non-monotonicity is a **tie artefact** — synthetic yields 0.0000 at
both w=2 and w=3, so strict `>` fails on equal values. It is not evidence of
distinct detectors. See §4.

**OBS B:** Different widths are most sensitive to different classes.
w=1 → most sensitive to prose. w=2, w=3 → most sensitive to code.
→ The ranking does shift, but this follows from the widths being ordered
thresholds on a single latent variable. See §4.

**Additional:** Synthetic input vanishes entirely at w≥2.
The repeating Fibonacci byte pattern never produces 3+ consecutive 1-bits.
Wider adjacency windows completely suppress regular patterns.

## 3. Temporal Distribution

Windowed DILATE rate variance (σ²):

| class | w=1 σ² | w=2 σ² | w=3 σ² |
|---|---|---|---|
| prose | 0.000016 | 0.000007 | 0.000001 |
| code | 0.000222 | 0.000068 | 0.000020 |
| synthetic | 0.000000 | 0.000000 | 0.000000 |

Code has 14× higher temporal variance than prose at w=1.
Code's windowed profile is bursty; prose is uniform.
Wider windows compress variance for both.

## 4. What This Actually Proves

**Width is an ordered threshold on 1-run length, not an independent axis.**

The three widths are defined (`internal/transponder/threshold.go:96-110`) as
1-runs of length ≥ 2, ≥ 3 and ≥ 4. Those event sets are **nested**:

```
{run ≥ 4}  ⊆  {run ≥ 3}  ⊆  {run ≥ 2}
```

so dilation counts must be non-increasing in width for **every possible
input**. `TestStructuralCalibration` asserts this over 2000 randomized streams
and it holds with zero violations. There is therefore no gain-vs-selectivity
trade-off to calibrate over: width is one scalar read at three thresholds.

What survives from the original reading:

1. The **ranking** of classes by dilation rate does shift (prose-first at w=1,
   code-first at w=2/w=3). That is a real observation, but it is a consequence
   of the nesting, not evidence of multi-detector geometry.
2. Regular/periodic input is fully suppressed at w≥2, because the synthetic
   pattern never produces 3+ consecutive 1-bits.
3. Code is burstier than prose in time, by ~14× at w=1.

What does **not** survive:

- "The transponder array is now a multi-detector sensor, not a multi-fingerprint
  wrapper" — the three transponders here see identical event *structure*; only
  their sketch fingerprints differ.
- Any claim that width and threshold are orthogonal axes, or that the array
  spans a 2-dimensional parameter space. Width is nested by construction and
  the threshold axis is untested on these corpora (no markers fire at all).
  See `docs/SPEC.md` §8.

The prose/code shift is also partly an ASCII-encoding artifact: the w=2/w=3
sensitivity is driven by which ASCII bytes contain long 1-runs (`|`, `}`, `~`,
`>` all have runs of 4-6), and Go source is dense in those characters.

## 5. Connection to Biology

The cochlear analogy is suggestive but does not survive the nesting result.
Different hair cells differ in *physical spacing*, and that spacing genuinely
selects independent frequency bands — but the analogue here is a set of
ordered thresholds on one variable, not independent resonances. Treat the
analogy as motivation, not evidence.

---

**One-line:** Seeds label trajectories; width thresholds 1-run length.

*Generated: 2026-03-14 20:41 CET · verdicts revised after width-nesting
assertion and v1 sketch fold fix*

