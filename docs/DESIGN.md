# DESIGN — notes & implementation plan

## A. Core: deterministic transducer + unbounded dilation counter

Think of the system as:

- a **finite-state** machine for local legality/segmentation ambiguity
- times a single monotone counter `r` ("clock dilation" / retrospective upsample exponent)

The DoS guarantee comes from bounding work per input symbol: bitset state updates + scalar counter increments.

## B. Virtual stuffing without rewriting

We never apply `D(s)` to stored data.

Instead:
- store observed bits in an append-only rope
- interpret index mapping through `r` for any semantic probe

If we store **base indices of ones** (sparse representation), retrospective dilation is trivial:

- effectiveIndex = baseIndex << r

This avoids touching all stored indices on each dilation event.

## C. Allowed segmentation via regular language

Segmentation should not spawn unbounded hypotheses.

Approach:
- emit sparse candidate boundary markers at zero-run threshold crossings (2^k)
- maintain an NFA state-set (bitset) representing cut/no-cut possibilities
- render representative segmentations by extracting exemplars from the automaton under a budget

## D. Hexagram window (6-bit local context)

Use a 6-bit sliding window as a local opcode / feature:
- detects adjacency and other forbidden local patterns
- feeds LUTs for potential future rewrite logic

In canonical Zeckendorf, only 21 of 64 internal patterns are valid (no adjacent 1s), which is useful as an invariant.

## E. Marker / Rosetta layer (future)

Markers are checkpoints anchoring translations between:
- fib radix structure
- log2/Binet magnitude
- modular fingerprints

Key unresolved technical item: define probes under retrospective dilation consistently and cheaply.

## F. Renderer (unDoSable)

Renderer never enumerates all hypotheses. It returns:
- scalar summaries (r, rates, counts, ranges)
- K exemplars chosen deterministically (lexicographically min/max; min/max cuts; etc.)

If budget exceeded, degrade output, not ingestion.
