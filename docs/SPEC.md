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
- `t`: optional time/clock; not used for correctness.

### Events
- `DILATE`: emitted when adjacency `11` is observed.
- `ZERO_RUN(k)`: implicitly tracked as `zeroRun`.
- `MARKER(m)`: emitted when `zeroRun` crosses a sparse threshold family (default: powers of two >= 8 → 8,16,32,...).

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
