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

**v1 (legacy):** `sketch ^= Seeds[b] + uint64(W)` — simple XOR-add fold.
Suitable for coherence tracking, but collisions possible on structured input.

**v2 (recommended):** Independent `HashFamily` per transponder with avalanche
mixer `mixSketch(sk, a, b, r) = RotateLeft64(sk*a + b, r)`. Rich folding
includes `zeroRun`, `R`, seeds, and per-event salts. SketchDelta tracks
bit-level drift. Collision-resistant across all tested corpora.

Implementations must support both. v2 is preferred for new deployments.

## 8. Proprioceptive feedback (optional)

Transponders may run an adaptive calibration loop:

1. **Sense:** EMA trackers monitor dilateRate, markerRate, sketchDrift.
2. **Calibrate:** Adjust `width` and `threshold` via deterministic rules
   with hysteresis deadband.
3. **Converge:** Declare stable when drift < ε and rates settle.

Calibration is local to each transponder. No global coordination required.
Safety caps prevent runaway: `width ∈ [1,5]`, `threshold ≥ 4`.

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
