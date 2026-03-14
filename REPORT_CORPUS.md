# Corpus Experiment Report

**Date:** 2026-03-14  
**Input:** 3 text corpora (~1KB each), converted to UTF-8 bitstreams  
**Transponders:** tight, medium, wide (distinct Zobrist seed tables)  
**Window size:** 4096 bits

---

## 1. Input Corpora

| Class | Bytes | Bits | Description |
|---|---|---|---|
| prose | 1007 | 8056 | Natural language (Fibonacci essay) |
| code | 1185 | 9480 | Go source code (tree operations) |
| synthetic | 1536 | 12288 | Repeating 8-byte Fibonacci pattern |

## 2. Event Rates (DILATE rate = fraction of bits triggering adjacency)

| Class | dil-rate | mark-rate | total-rate |
|---|---|---|---|
| prose | 0.1992 | 0.0000 | 0.1992 |
| code | 0.1763 | 0.0000 | 0.1763 |
| synthetic | 0.0313 | 0.0000 | 0.0313 |

**Within each class, all three transponders produced identical event counts.**

| Class | transponders | dilations | markers | r |
|---|---|---|---|---|
| prose | tight/medium/wide | 1605 | 0 | 1605 |
| code | tight/medium/wide | 1671 | 0 | 1671 |
| synthetic | tight/medium/wide | 384 | 0 | 384 |

## 3. Sketch Divergence (XOR distance between transponders)

### Prose
| pair | XOR distance |
|---|---|
| tight ⊕ medium | `0x0000000000000070` |
| tight ⊕ wide | `0x0000000000000000` ← COLLISION |
| medium ⊕ wide | `0x0000000000000070` |

### Code
| pair | XOR distance |
|---|---|
| tight ⊕ medium | `0xf86d58a8debb0ccd` |
| tight ⊕ wide | `0x636c7ab49db92bd1` |
| medium ⊕ wide | `0x9b01221c4302271c` |

### Synthetic
| pair | XOR distance |
|---|---|
| tight ⊕ medium | `0x000000000000000c` |
| tight ⊕ wide | `0x0000000000000008` |
| medium ⊕ wide | `0x0000000000000004` |

## 4. Sketch Trajectory (window snapshots)

### Prose (2 windows)
```
W0: tight=0xcf4bb80e586876dd  medium=0x3726e0a686d37a8c  wide=0xac27c2bac5d15d5c
W1: tight=0x0000000000000078  medium=0x0000000000000008  wide=0x0000000000000078
```

### Code (3 windows)
```
W0: tight=0x0000000000000000  medium=0x00000000000000d8  wide=0x00000000000003e0
W1: tight=0xcf4bb80e586876e7  medium=0x3726e0a686d37a72  wide=0xac27c2bac5d15d36
W2: tight=0xcf4bb80e5868768f  medium=0x3726e0a686d37a42  wide=0xac27c2bac5d15d5e
```

### Synthetic (3 windows)
```
W0: tight=0x000000000000002e  medium=0x0000000000000022  wide=0x0000000000000026
W1: tight=0x000000000000002e  medium=0x0000000000000022  wide=0x0000000000000026  ← identical
W2: tight=0x000000000000002e  medium=0x0000000000000022  wide=0x0000000000000026  ← identical
```

## 5. Proven

1. **Classes are separable by event rate.** Prose (0.199), code (0.176), and synthetic (0.031) produce measurably different DILATE rates. The FSVM detects structural differences between input types.

2. **Calibration does not affect event structure.** All three transponders produced identical DILATE and marker counts within each class. The adjacency detector is a physical property of the input stream, not a property of the Zobrist seeds.

3. **Sketch identity is calibration-dependent but input-sensitive.** For code, all three sketches diverged fully (64-bit XOR distances). For prose and synthetic, sketch divergence was minimal (lower-bits only) or zero.

4. **Repeating input produces stable sketches.** The synthetic pattern produced identical sketches across all windows — the sketch converges to a fixed point when the input is periodic.

## 6. Not Proven

1. **Calibration does not produce different structural readings.** The transponder array's three calibrations see identical event structures. They produce different sketch fingerprints, but those fingerprints are not more informative than the raw event counts.

2. **Sketch collision is possible.** Tight and wide produced identical final sketches (`0x78`) on prose input. The sketch alone cannot serve as a reliable class identifier.

3. **The sketch adds information beyond event rate.** For synthetic input, all sketches converged to constant values. For prose, sketch XOR distances were near-zero. The sketch appears to be a noisy encoding of the same information that DILATE rate already captures.

## 7. Does the Array Sense Structure?

**No. The array currently produces different fingerprints, not structural readings.**

The experiment confirms three facts:

1. **Seed variation changes fingerprint identity.** Different ZobristSeed tables produce different sketch values.
2. **Seed variation does not change event structure.** All transponders produce identical DILATE and marker counts on the same input.
3. **Class separation comes from FSVM dynamics, not calibration diversity.** A single transponder already separates prose (0.199) from code (0.176) from synthetic (0.031).

**Corrected conclusion:** The current array is a multi-fingerprint wrapper around one detector, not a multi-detector sensor array. ZobristSeed tables are suitable for sketch identity and coherence tracking, but they do not by themselves create distinct structural detectors.

**One-line summary:** Seed-only calibration falsified; structural calibration remains open.

## 8. Where Structural Diversity Must Come From

The vision doc's biological analogy (cochlear hair cells with different spacings) points to the correct calibration surface: **detection geometry**, not hash seeds. To produce different event profiles, transponders must vary structural parameters:

| Parameter | Current | Variation axis |
|---|---|---|
| Locality spacing | Fixed 1-bit adjacency | 1-bit vs 2-bit vs 4-bit adjacency windows |
| Window width | Fixed 6-bit (W) | 4-bit vs 6-bit vs 8-bit hexagram |
| Adjacency rule | Fixed "11" detection | Configurable pattern (e.g., "101", "110") |
| Marker thresholds | Fixed powers-of-2 ≥ 8 | Different threshold families |
| Dilation rule | Fixed r++ on "11" | Variable dilation exponent per pattern |

The next experiment should hold seeds fixed, vary exactly one structural parameter per transponder, and re-run the same corpus. First candidate: **effective locality spacing / window rule**, as it is the closest to the biological analogy and the most likely source of event-profile divergence.

## 9. Proven vs Hypothesis (Final)

**Proven:**
- FSVM DILATE rate separates input classes (structural detection works)
- Zobrist sketch provides identity fingerprinting (coherence tracking possible)
- Seed-only calibration does not alter event structure

**Hypothesis (falsified):**
- Different ZobristSeed tables produce different structural readings ← FALSE

**Hypothesis (open):**
- Varying detection geometry (window width, adjacency rule, marker thresholds) produces different structural readings ← UNTESTED
- Multi-detector array improves class separability over single detector ← UNTESTED
- Sketch divergence indicates convergence state (proprioceptive signal) ← UNTESTED

---

## 10. Next Experiment: Structural Calibration (Proposed)

**Goal:** Hold seeds fixed, vary exactly one structural parameter per transponder, re-run corpus.

**Design:** Create 3 FSVM variants with different adjacency window widths:
- T₁: 1-bit adjacency (current: `LastBit == 1 && b == 1`)
- T₂: 2-bit adjacency (`W & 0x03 == 0x03 && b == 1`)
- T₃: 3-bit adjacency (`W & 0x07 == 0x07 && b == 1`)

**Prediction:** If structural calibration works, the three transponders should produce different DILATE counts on the same input. T₁ (current) should fire most often; T₃ should fire least often.

**Implementation:** Add `AdjacencyMask uint8` and `AdjacencyValue uint8` to FSVM State. Step() checks `(s.W & s.AdjacencyMask) == s.AdjacencyValue && b == 1` instead of hardcoded `s.LastBit == 1 && b == 1`.

**Success criteria:** At least one input class shows different DILATE counts across the three transponders.

**Failure mode:** If all transponders still produce identical event rates, the hypothesis that detection geometry creates structural diversity is also falsified.
