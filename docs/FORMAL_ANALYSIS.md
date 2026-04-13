# Formal Analysis — FSVM Complexity and Coherency

## 1. O(1) Per-Bit Complexity

### Theorem
`Step(s State, b uint8)` executes in O(1) time with respect to all parameters.

### Proof

The `Step` function (internal/fsvm/fsvm.go:64-95) performs exactly:

1. `b &= 1` — one AND operation
2. Zero-run update: one increment + one comparison + one branch
3. Marker check: one comparison (`>= 8`) + one bitwise test (`isPow2`)
   - `isPow2(x)` is `x > 0 && (x & (x-1)) == 0` — two comparisons, one AND, one SUB
4. Adjacency check: one comparison (`s.LastBit == 1 && b == 1`)
5. Dilation update: two increments (R, Dilations) on branch taken
6. Window update: one shift, one OR, one AND (mask 0x3F)
7. Zobrist fold: one XOR, one ADD, two memory reads (Seeds[b], s.W)
8. Return: fixed-size struct copy

**Total operations:** ≤ 20 primitive operations, independent of:
- Stream length (no loops over history)
- Dilation count (r is a counter, not a loop variable)
- Zero-run length (comparison is O(1), not O(zeroRun))
- Sketch value (XOR is constant-time)

**No dynamic dispatch.** No interfaces, no closures, no function pointers.
**No loops.** No iteration over any variable-length structure.
**No recursion.** Pure tail computation.

∴ Step is O(1) per input bit. ∎

### Corollary: Amortized cost equals worst-case cost
Since every step is O(1) (not amortized O(1) with occasional expensive operations),
the amortized cost equals the worst-case cost: 44-48 ns/op on Pentium N4200.

---

## 2. Bounded Memory

### Theorem
The FSVM State occupies a fixed, constant-size memory footprint regardless of
stream length or content.

### Proof

The State struct contains:

| Field      | Type     | Size (bytes) | Bounded by |
|------------|----------|-------------|------------|
| Seeds      | [2]uint64| 16          | constant   |
| Sketch     | uint64   | 8           | constant   |
| ZeroRun    | uint64   | 8           | constant   |
| Dilations  | uint64   | 8           | constant   |
| Markers    | uint64   | 8           | constant   |
| R          | uint32   | 4           | constant   |
| W          | uint8    | 1           | constant   |
| LastBit    | uint8    | 1           | constant   |

**Total: 54 bytes** (padded to 56 by Go's alignment rules).

This is independent of:
- Number of bits processed
- Number of DILATE events
- Number of markers
- Sketch value

**Counter overflow note:** Dilations, Markers, and ZeroRun are uint64.
They overflow after 2^64 events. For streams longer than 2^64 bits
(~2.3 × 10^18 exabytes), the caller must checkpoint State externally.
This is a practical non-issue: no physical system produces 2^64 bits.

∴ State memory is O(1). ∎

### Event allocation

Each Step may emit 0, 1, or 2 events (DILATE and/or MARKER).
The event slice is allocated by the caller or by Go's append on a nil slice.
For ≤ 2 elements, Go allocates from the stack (no heap allocation).
The event slice does not grow beyond 2 elements per call.

---

## 3. Zero Heap Allocation

### Theorem
`Step` performs zero heap allocations on every call path.

### Evidence (Go benchmark)

```
BenchmarkStep-4    44.95 ns/op    0 B/op    0 allocs/op
```

### Why the compiler can prove this

1. **Value receiver:** `State` is passed by value (56 bytes, fits in registers
   or stack). No pointer escape.

2. **Bounded append:** `evs` starts as `var evs []Event` (nil slice).
   The first `append` for ≤ 2 elements allocates from the stack-local
   backing array. The Go compiler (1.21+) recognizes this pattern and
   elides the allocation.

3. **No interface values:** No `interface{}` conversions, no reflection.
   All types are concrete.

4. **No closures:** No anonymous functions capturing variables.

5. **No channels/goroutines:** Pure synchronous computation.

**Note:** This analysis applies to the core `Step` function only.
The `Array.Step` method (transponder/array.go) allocates a `[]Result`
slice per call — this is array-level overhead, not FSVM-level.

---

## 4. Zeckendorf Coherency

### Definition
A bitstream is **Zeckendorf-coherent** at position t if it contains no
adjacent 1-bits (no "11" substring) in the canonical Zeckendorf
representation. The FSVM tracks the **dilation exponent** r as the
count of coherence violations observed.

### The Dilation Semantics

When the FSVM observes adjacency "11" at positions (i, i+1):
- It emits DILATE
- It increments r
- Conceptually, the stream is "virtually stuffed" with a zero between
  positions i and i+1: `...1 0 1...`
- But **no data is rewritten**. The dilation is retrospective: the
  effective index mapping becomes `i → i << r` (conceptually)

### Coherency Invariant

After processing n bits with dilation count r:
- The canonical Zeckendorf representation of the "virtual" stream
  (with r zero-stuffings) has no adjacent 1s
- r counts the minimum number of insertions needed to make the
  observed stream Zeckendorf-coherent

### Proof Sketch

Each DILATE event corresponds to exactly one "11" adjacency.
The virtual stuffing operator D(s) = s₀ 0 s₁ 0 s₂ 0 ... 0 s_{n-1}
eliminates exactly one adjacency per application.
Therefore r equals the number of insertions needed.

However: interleaved adjacencies may "cascade" after stuffing.
Example: `111` → stuff at first `11` → `1011` → still has `11` →
needs second stuffing. The FSVM handles this because it processes
bits sequentially: `111` triggers two DILATE events as it sees
`1→1` (DILATE, r=1) then `1→1` again (DILATE, r=2).

**Formal claim:** After n input bits with r total DILATE events,
the virtual stream (with r zero-insertions at the observed positions)
is Zeckendorf-coherent.

**Status:** Empirically verified on all test corpora. Formal proof
of the cascade property (that sequential processing captures all
necessary stuffings) is deferred to the paper.

---

## 5. Zobrist Sketch Properties

### The fold

```
s.Sketch ^= s.Seeds[b] + uint64(s.W)
```

This is a **rolling XOR-add fold** where:
- `Seeds[b]` provides input-dependent variation (bit value)
- `s.W` provides context-dependent variation (6-bit window)
- `^=` accumulates across the entire stream

### Properties

1. **Deterministic:** Same input + same seeds → same sketch.
   No randomness after initialization.

2. **Sensitive:** Flipping any input bit changes `s.W` for all
   subsequent steps, cascading through the sketch. Two streams
   differing in one bit produce different sketches with high
   probability (assuming good seed distribution).

3. **Commutative up to W:** The XOR fold is commutative for the
   seed component: `Seeds[0] ⊕ Seeds[1] = Seeds[1] ⊕ Seeds[0]`.
   But the W-dependent ADD breaks commutativity: the window state
   W depends on bit ordering. Therefore the sketch is
   order-sensitive despite using XOR.

4. **Collision bound:** With a 64-bit sketch, collision probability
   is 2^{-64} for independently-seeded transponders processing
   different inputs. For same-input different-calibration (structural
   array), collisions are possible and observed (tight/wide on prose
   in REPORT_CORPUS.md).

5. **Not a universal hash:** The fold does not satisfy the universal
   hashing property because the ADD introduces linear dependencies.
   It is a practical state fingerprint, not a cryptographic commitment.

### What the sketch provides

- **Cheap divergence detection:** Two transponders processing the same
  input should produce the same sketch (if seeded identically).
  XOR distance `sketch_A ⊕ sketch_B` measures divergence.
- **Convergence detection:** If the sketch stabilizes (no change between
  windows), the input stream has reached a periodic or null state.
- **Identity fingerprinting:** The sketch at any point is a function of
  the entire input history. It serves as a compact state identity.

### What the sketch does NOT provide

- **Class identification:** Sketch collisions between classes are possible.
  The sketch supplements event rates, it does not replace them.
- **Semantic meaning:** The sketch is a structural fingerprint, not a
  semantic encoding. Interpretation belongs in the analytical layer.

---

## 6. Structural Calibration Independence (Formal)

### Definition

Two structural parameters A and B are **independent** if, for a fixed
corpus C = {c₁, c₂, ..., cₖ}:

```
∀a ∈ values(A): rank_B(c₁, c₂, ..., cₖ) is constant across values of B
```

and

```
∀b ∈ values(B): rank_A(c₁, c₂, ..., cₖ) is constant across values of A
```

...producing DIFFERENT rank orderings. That is:
- At fixed width, threshold changes class rankings
- At fixed threshold, width changes class rankings
- The two effects are not redundant

### Proven (from second-axis experiment)

At fixed width w ∈ {1, 2, 3}:
- pow2≥8 ranks: prose+zeros > code+zeros > mixed
- pow3≥9 ranks: code+zeros > prose+zeros > mixed
- lin4 ranks: mixed > code+zeros > prose+zeros

These are different orderings → threshold is independent of width.

### Implication for array design

A transponder array with calibration (width, threshold) spans a
2-dimensional parameter space. Each (w, t) pair defines a unique
detector with a unique sensitivity profile. The array is a basis set,
not a 1-parameter family.

**Open question:** Whether a third axis (e.g., window width W) adds
further independence or is redundant with the existing two axes.
This is future work.

---

## 7. Complexity Summary

| Component | Time | Space | Allocs |
|-----------|------|-------|--------|
| FSVM Step | O(1), 44-48ns | O(1), 56 bytes | 0 |
| StepWidth | O(1), ~same | O(1), 56 bytes | 0 |
| StepFull | O(1), ~same | O(1), 56 bytes | 0 |
| BitRope Append | O(1), 14.78ns | O(n) amortized | 0 |
| Array Step (k transponders) | O(k) | O(k × 56) | O(k) for Result slice |
| Classifier | O(1), 55ns | O(1) | 0 |

**Key insight:** The per-transponder cost is O(1) and allocation-free.
Array-level cost is O(k) where k = number of transponders, with one
allocation per step for the Result slice. For k ≤ 10 (typical array),
total cost is < 500ns per input bit.

---

*Generated: 2026-04-13*
*Benchmarks from: Intel Pentium N4200 @ 1.10GHz, Go 1.25+*
