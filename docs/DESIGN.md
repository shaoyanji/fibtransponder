# DESIGN — implementation notes aligned to SPEC

This document translates `docs/SPEC.md` into implementation constraints and module boundaries.

## 1) Core model (measurement-first)

The core ingest path tracks exactly the spec state:

- `r ∈ ℕ` — dilation exponent (count of `DILATE` events)
- `w ∈ {0..63}` — 6-bit rolling window
- `lastBit ∈ {0,1}` — adjacency detector input
- `zeroRun ∈ ℕ` — consecutive-zero run length

Per-symbol ingest must remain O(1) (or amortized O(1)).

## 2) Event semantics

From observed bit updates:

- emit `DILATE` when adjacency `11` is observed, then `r := r + 1`
- update `zeroRun` continuously (implicit `ZERO_RUN(k)` state)
- emit sparse `MARKER(m)` on threshold crossings (`8,16,32,...` default)

No event may trigger unbounded retroactive rewriting work.

## 3) Retrospective dilation (virtual, not materialized)

The dilation operator is semantic:

- interpret indices as if globally transformed by `D(s)=s0 0 s1 0 ...`
- do **not** write stuffed zeros into storage
- prefer base-index storage so effective index lookup can be computed from `r`

Design consequence: ingestion mutates small local state only; heavy interpretation is deferred to probes/renderers.

## 4) Storage boundary

Use append-only immutable block strategy (`bitrope`-style substrate):

- linear growth in observed input size
- no full-stream rewrites on dilation events
- stable references for downstream probes

## 5) Segmentation layer (optional interpretation)

Segmentation is not required for correctness and is not part of the ingest state machine.

Implementation policy:

- only consider sparse deterministic candidate cuts
- represent ambiguity symbolically (NFA/DFA state sets)
- never enumerate unbounded hypothesis trees

## 6) Probe layer (lazy by default)

Always-on low-cost probes:

- `r(t)` and dilation event rate
- `zeroRun`/marker frequency
- sparse one-position summaries or counts

Optional Rosetta probes (kept outside core correctness path):

- Binet-style log2 bounds
- modular fingerprints (`N mod p_i`) with explicit dilation semantics

## 7) Rendering budget policy

Renderer must degrade output before it degrades ingest:

- return bounded summaries/ranges first
- return only a small deterministic exemplar set when needed
- never block ingest on expensive interpretation

## 8) Non-goals (for this draft stage)

- no claim of finalized `N(r)` semantic value definition
- no mandatory segmentation interpretation
- no BigFloat-heavy numeric core

Open semantic questions remain in `docs/SPEC.md` and are treated as explicit TODOs, not hidden assumptions.