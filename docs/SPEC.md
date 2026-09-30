# SPEC — Fibonacci-radix streaming transponder (draft)

## 0. Definitions

- **Observed stream**: an unbounded boolean sequence `x_0, x_1, ...` produced by a sensor/transponder.
- **Canonical Zeckendorf word**: a bitstring with **no adjacent 1s**.
- **Indexing**: `bit[i]` corresponds to Fibonacci index **F_{i+2}**, i.e. `bit[0] ↔ F2 = 1`.

## 1. Core state (measurement-first)

The core is deterministic and bounded-work per input symbol.

### State variables
- `r ∈ ℕ`: global **dilation exponent** (# of retrospective dilation events).
- `w ∈ {0..63}`: 6-bit **hexagram window** over the most recent logical bits (orientation defined by implementation).
- `lastBit ∈ {0,1}`: last observed bit (for adjacency detection).
- `zeroRun ∈ ℕ`: current consecutive-zero run length.
- `sketch ∈ uint64`: Zobrist state fingerprint (v1: XOR-add fold; v2: avalanche mixer + rich folding).
- `sketchDelta ∈ uint8`: rolling count of bits changed in sketch (proprioceptive drift signal).
- `bitsProcessed ∈ ℕ`: total bits consumed (monotonic counter, for event timestamps).
- `t`: optional time/clock; not used for correctness.

Concrete footprint: `fsvm.State` is **96 bytes** on 64-bit (`unsafe.Sizeof`,
verified in `TestStateSize`), for both v1 and v2 — they share one state type.
Counters `ZeroRun`, `Dilations`, `Markers`, `BitsProcessed` are `uint64`;
`R` is `uint32` and wraps after 2³² dilations (~512 MB of all-ones input).

### Events
- `DILATE`: emitted when adjacency `11` is observed.
- `ZERO_RUN(k)`: implicitly tracked as `zeroRun`.
- `MARKER(m)`: emitted when `zeroRun` crosses a sparse threshold family (default: powers of two >= 8 → 8,16,32,...).

### Optional rich-feature state
- `Descriptor`: 256-bit local feature vector extracted at events (1-D SIFT/SURF analogue).
- `Extractor`: rolling 64-bit history window that produces Descriptors.
- `FeatureBuffer`: ring buffer of recent FeatureEvents for downstream matching.

### Optional proprioceptive state
- `width ∈ {1..5}`: adjacency detection width (sensitivity calibration).
- `threshold ∈ ℕ`: zero-run marker threshold (sparsity calibration).
- EMA trackers: `emaDilate`, `emaMarker`, `emaDrift` (scaled integer arithmetic).

## 2. Dilation rule (retrospective virtual stuffing)

If adjacency `11` is observed in the stream, increment `r`:

- `r := r + 1`

Interpretation: the *semantic mapping* of indices is retroactively rescaled as if the entire stream were transformed by the dilation operator:

`D(s) = s0 0 s1 0 s2 0 ... 0 s_{n-1}`

but **no zeros are materialized**. Instead, effective indices become `i << r` (conceptually).

## 3. Segmentation (allowed, not forced)

There is no explicit EOF. Segmentation is an interpretation layer:

- long runs of zeros suggest possible message boundaries (cuts)
- cuts are **allowed** at candidate points, never required

To remain unDoSable, candidate cut points are sparse and deterministic, e.g. when `zeroRun` hits `2^k` for some k.

Segmentation ambiguity is represented as a **regular language** (NFA/DFA) over cut/no-cut choices at candidate points.

## 4. Output / probes (lazy)

The system should avoid materializing gigantic integers.

Recommended always-on probes:
- `r(t)` and rate of dilation events
- `zeroRun` and marker frequency
- sparse `1` positions (base indices) or counts

Optional probes (Rosetta layer):
- log2 magnitude bounds using Binet-style asymptotics (no BigFloat in core)
- modular fingerprints for sync (`N mod p_i`), with careful definition under retrospective dilation

## 5. Safety / DoS constraints

- Ingestion must be O(1) (or O(1) amortized) per input bit.
- Memory growth should be linear in observed bits, with immutable block allocation.
- Rendering must be budgeted; if over budget, return summaries (ranges) and a small set of exemplars.

## 6. Open questions (explicit)

- Exact definition of semantic value under dilation (what does `N(r)` mean?)
- Which probes must track the *dilated* interpretation vs raw observation?
- Marker payload + update equations (rosetta layer)

## 7. Sketch versioning

Two sketch algorithms are defined:

**v1 (legacy):** `sketch ^= Seeds[b] + uint64(W) * 0x9E3779B97F4A7C15`
(`fsvm.SketchTerm`) — XOR fold of a per-bit seed plus a window term.

The window multiplier is load-bearing. `W` is only 0..63, so adding it
directly to a seed perturbs just the low bits and leaves the upper 56 bits a
function of the bit counts alone — a nominal 64-bit sketch then takes at most
4×256 = 1024 distinct values. Multiplying by an odd constant spreads each of
the 64 window values across the full word, so every bit position carries
window-dependent entropy. Measured (`TestSketchEntropy`): 20000 random streams
produce ~19713 distinct sketches, up from 256 before the multiplier was added.

**v2 (recommended):** Independent `HashFamily` per transponder with avalanche
mixer `mixSketch(sk, a, b, r) = RotateLeft64(sk*a + b, r)`. Rich folding
includes `zeroRun`, `R`, seeds, and per-event salts. SketchDelta tracks
bit-level drift. Every family's multiplier `A` must be **odd** — an even `A`
makes `x → x*A + B` two-to-one and destroys bit 63 on every step. Asserted by
`TestHashFamiliesWellFormed` and `TestMixSketchIsBijective`.

Implementations must support both. v2 is preferred for new deployments.

### Not claimed

- The sketch is **not** a universal hash and has **no** cryptographic collision
  bound. Do not cite a 2⁻⁶⁴ collision probability; that figure does not hold
  for this construction. See `TestSketchCollisionRate`: about 1.5% of random
  streams of length 1..200 collide, first collision observed near stream 350.
- The sketch is a **state fingerprint** for coherence and divergence tracking,
  not a stable identifier. Distinct inputs can and do collide.
- Pure-XOR folding is **parity-limited on degenerate input**. For an all-zero
  stream `W` stays 0, so the fold reduces to a parity count and yields only 2
  distinct sketches across all lengths (`TestSketchDegenerateStreamLimitation`).
  A working all-zero stream is the pathological case, not the typical one.
- Collision behaviour on the corpora in this repo is **observed, not proven**;
  no cross-transponder divergence guarantee is claimed.

## 8. Proprioceptive feedback (optional)

Transponders may run an adaptive calibration loop:

1. **Sense:** EMA trackers monitor dilateRate, markerRate, sketchDrift.
2. **Calibrate:** Adjust `width` and `threshold` via deterministic rules
   with hysteresis deadband.
3. **Converge:** Declare stable when drift < ε and rates settle.

Calibration is local to each transponder. No global coordination required.
Safety caps prevent runaway: `width ∈ [1,5]`, `threshold ≥ 4`.

**Width and threshold are not independent axes.** `width = 1,2,3` test for a
1-run of length ≥ 2/3/4 respectively, so the detected event sets are *nested*:
{run≥4} ⊆ {run≥3} ⊆ {run≥2}. Dilation counts are therefore non-increasing in
width for every possible input, by construction — asserted on 2000 randomized
streams by `TestStructuralCalibration`. Width is an ordered threshold on one
latent variable (1-run length), not a second free parameter, and no
"orthogonal axes" or "2-dimensional parameter space" claim is made.

Likewise, the threshold axis does not affect adjacency detection at all:
`StepFull`'s dilation branch never reads `thresh`. That invariance is a
property of the code, not an experimental finding. The marker threshold is
**untested** on the corpora in this repo — none produce zero runs long enough to
trigger any marker family, so no independence conclusion can be drawn from it
(`TestSecondAxisCalibration` asserts this precondition).

## 9. Rich features (optional)

At each event (Marker or Dilate), an implementation may extract a
`Descriptor` from a rolling 64-bit local window:

- 8 sub-regions × 8 bits
- Per-region: density, transitions, Haar-X, Haar-Y
- Packed into 4 × uint64 = 32 bytes

Descriptors support L1 distance and cosine similarity for downstream
clustering, motif detection, or structural fingerprinting.

Rich features are opt-in via `Extractor`. When disabled, core ingest
semantics and complexity are unchanged.
