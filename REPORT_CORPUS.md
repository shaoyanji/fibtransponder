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

All three transponders report the same event structure (identical DILATE counts, identical marker counts). The sketch diverges between transponders, but this divergence is:
- Unstable (collisions occur on some inputs)
- Redundant (correlated with DILATE rate, which is already captured without calibration)
- Not class-separating (the same transponder produces overlapping sketch ranges across classes)

What *does* sense structure is the FSVM itself — DILATE rate cleanly separates the three input classes (0.199 vs 0.176 vs 0.031). But this separation requires only one transponder. Adding two more with different seed tables produces different sketch values but does not add separability.

The transponder array hypothesis — that different calibrations detect different structural features — is **falsified by this experiment**. At least with the current Zobrist-seed-only calibration approach, all transponders see the same structure. They only differ in the hash of what they see.

## 8. What Would Need to Change

For the array to actually sense different structure, calibration would need to affect the *detection logic*, not just the sketch hashing. Possibilities:

- **Locality spacing:** Different transponders look at different bit windows (1-bit adjacency vs 3-bit patterns vs 8-bit patterns)
- **Window size:** Different W widths (currently fixed at 6 bits)
- **Marker thresholds:** Different zero-run thresholds for marker emission
- **Dilation rule:** Different adjacency sensitivity (currently fixed at "11")

As long as all transponders run the same Step() logic with different Zobrist seeds, they will always produce identical event structures with different sketch fingerprints.

---

*Generated: 2026-03-14 20:22 CET*
