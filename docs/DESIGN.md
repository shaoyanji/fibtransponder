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

## G. Architectural Flexibility and Extensibility

A core design principle of the fibtransponder is its modular and extensible architecture. The "Fibonacci State Vector Machine" (FSVM) at its heart acts as a robust, deterministic event source, processing the incoming bitstream and emitting fundamental events like `DILATE` and `MARKER`. This approach ensures that the core state update is always O(1) and free from side effects that might complicate further analysis.

Building upon this stable foundation, various "interpretation layers" can be dynamically attached without modifying the core FSVM. For instance, the Segmentation Automaton (NFA) processes `MARKER` events to symbolically represent segmentation possibilities, providing an unDoSable way to manage ambiguities. Similarly, the newly introduced "Rosetta Layer" demonstrates how `MARKER` events can trigger deeper, application-specific analyses—such as identifying Fibonacci zero-run lengths or specific FSVM window patterns—without burdening the primary bit-processing pipeline.

This clear separation of concerns allows for:
- **Unbounded extensibility:** New analysis modules can be easily integrated by simply subscribing to FSVM events.
- **Maintainability:** Core logic remains isolated and simple.
- **Experimentation:** Different interpretative models (e.g., for various encoding schemes or cryptographic probes) can be developed and tested in parallel.

This flexibility is key to exploring the rich mathematical properties of Fibonacci radix representations in diverse applications, from data compression to secure communication protocols.
