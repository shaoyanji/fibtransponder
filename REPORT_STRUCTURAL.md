# Structural Calibration Report

**Date:** 2026-03-14  
**Experiment:** Fixed seeds (DefaultSeeds), varying adjacency width: w=1, w=2, w=3  
**Corpus:** Same 3 classes as seed experiment (prose 8056b, code 9480b, synthetic 12288b)  
**Window:** 2048 bits

---

## 1. Sensitivity Matrix

DILATE rate per (width × class):

| width | prose | code | synthetic | most-sensitive |
|---|---|---|---|---|
| w=1 | **0.1992** | 0.1763 | 0.0313 | prose |
| w=2 | 0.0582 | **0.0609** | 0.0000 | code |
| w=3 | 0.0079 | **0.0150** | 0.0000 | code |

## 2. Falsification Results

**PASS A:** Dil-rate ordering is NOT monotonic across widths.  
w=1 ranks prose > code. w=2 and w=3 rank code > prose.  
→ Widths change sensitivity profile, not just gain.

**PASS B:** Different widths are most sensitive to different classes.  
w=1 → most sensitive to prose. w=2, w=3 → most sensitive to code.  
→ Calibration shifts which structure is most detected.

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

## 4. What This Proves

**Structural calibration is real.** Varying adjacency width produces:
1. Different class sensitivity rankings (prose-first → code-first)
2. Complete suppression of regular patterns at wider widths
3. Different temporal distribution profiles

The transponder array is now a multi-detector sensor, not a multi-fingerprint wrapper.

## 5. Connection to Biology

The cochlear analogy holds: different hair cells have different *physical spacings* along the basilar membrane. They don't differ in how they hash their output — they differ in *what they resonate to*. Wide-spaced cells detect low frequencies; tight-spaced cells detect high frequencies.

Similarly: wide-adjacency transponders detect sustained bit patterns (code structure); tight-adjacency transponders detect rapid alternation (prose structure). The calibration is the geometry, not the seed.

---

**One-line:** Seeds label trajectories; geometry senses structure.

*Generated: 2026-03-14 20:41 CET*
