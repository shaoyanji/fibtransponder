# HANDOFF — Fibtransponder Research Vision & Next Directions

**Date:** 2026-03-14  
**Purpose:** This document exists because the stateful purpose of the fibtransponder has been at risk of dilution by spec-layer additions that optimize for canonicality over throughput. Read this first before modifying any code.

---

## 0. The One Thing To Remember

**The FSVM IS the contribution. Everything else is optional infrastructure.**

The FSVM at 46.55 ns/op on a Pentium N4200 is a Fibonacci-radix streaming state machine with built-in error correction (Zeckendorf), retrospective dilation (semantic rescaling without data rewrite), and zero-allocation async signal emission. That's the invariant. That's what the paper is about.

If a change makes the FSVM slower, more complex, or less autonomous — it's wrong, no matter how canonical the spec looks.

---

## 1. Architecture (Correct Hierarchy)

```
┌─────────────────────────────────────────────┐
│  Fibtransponder Array (sensor layer)        │  ← multiple FSVMs, different calibrations
│  ┌─────┐ ┌─────┐ ┌─────┐ ┌─────┐          │
│  │ T₁  │ │ T₂  │ │ T₃  │ │ T₄  │ ...      │  ← each ~46ns, independent, parallel
│  └──┬──┘ └──┬──┘ └──┬──┘ └──┬──┘          │
│     │       │       │       │               │
│  ◄──┼───────┼───────┼───────┼──► Zobrist    │  ← XOR fold is CORE, not sidecar
│     │       │       │       │    (Sketch)   │
└─────┼───────┼───────┼───────┼───────────────┘
      │       │       │       │
┌─────▼───────▼───────▼───────▼───────────────┐
│  Analytical Layer (cache + interpreter)      │  ← reads Zobrist, checks coherence
│  • dilation rate detection                   │  ← proprioceptive signal
│  • Zeckendorf coherency validation           │
│  • axial adjustment commands                 │  ← tells physical mover to reposition
│  • meaning caching                           │
└─────────────────────────────────────────────┘
      │
┌─────▼───────────────────────────────────────┐
│  Delta Queue (optional canonical sidecar)    │  ← MemOp log, frontier, revalidation
│  • ONLY if you need replay/audit            │  ← NOT on the hot path
│  • StepsSince, AuxBuckets, classify         │
└─────────────────────────────────────────────┘
```

**Critical rule:** The delta queue sits BELOW the analytical layer, not between the FSVM and the analytical layer. It is an audit log, not a hot-path component.

---

## 2. The Zobrist Mistake (And How To Fix It)

### What went wrong

The delta queue moved Zobrist sketching into the classifier (sidecar). This created:

```
FSVM.Step() → CoreDelta → Classify() → Zobrist fold → DerivedDelta
```

Result: 46.55ns (FSVM) + 50-99ns (classifier) = **2-3x overhead for a sketch that should cost one XOR.**

### The fix: Zobrist in the core

Fold the Zobrist XOR into the FSVM `Step()` function directly:

```go
type State struct {
    R       uint32
    W       uint8
    LastBit uint8
    ZeroRun uint64
    Sketch  uint64  // ← Zobrist sketch lives HERE, not in classifier
    Dilations uint64
    Markers   uint64
}

func Step(s State, b uint8) (State, []Event) {
    b &= 1
    // ... existing logic ...
    
    // One XOR, one register, same cost as W update
    s.Sketch ^= ZobristSeed[b]  // or ZobristSeed[s.W] for window-dependent fold
    
    return s, evs
}
```

The classifier then READS the sketch, it doesn't COMPUTE it. This is the difference between a core invariant and a derived property.

### Performance target

With Zobrist folded into core:
- FSVM step: ~46-48ns (one additional XOR is ~0.1ns, compiler-fused)
- Classifier: ~10-15ns (read sketch, compare, no computation needed)
- Total: ~60ns (vs current ~100-150ns)

---

## 3. The Fibtransponder Array (Sensor Layer)

### Concept

A single FSVM detects one "kind" of signal. An array of FSVMs, each with different **locality spacing** (calibration), detects different structural features — without any tokenization.

| Transponder | Spacing | Detects | Dilation profile |
|---|---|---|---|
| T₁ | 1-2 bits | consonants, stops, edges | frequent DILATE, rapid r drift |
| T₂ | 3-5 bits | vowels, continuants | sparse DILATE, stable r |
| T₃ | 8+ bits | prosody, rhythm, phrases | marker-heavy, almost no DILATE |

### Why this matters

**No tokenizer. No vocabulary. No BPE.**

The "token" is which transponders are firing and at what dilation rate. Language structure *emerges from the sensor array geometry*, just like the cochlea doesn't have a "frequency vocabulary" — it has hair cells with different physical spacings that resonate at different frequencies.

### Implications

- **Different languages** → different transponder calibrations, not retrained weights
- **Multimodal** → same array, different spacings (vision = wider, audio = tighter)
- **Model weights are fixed** → only sensor geometry changes
- **Parallel at 55ns each** → the whole array processes in one FSVM step cycle

### Calibration protocol

Each transponder's `ZobristSeed[]` table is its calibration. Different seeds = different detection sensitivity. The seed table IS the "trained parameter," but it's a lookup table, not a weight matrix.

---

## 4. Proprioceptive Computation (The Biological Analogy)

### The loop

```
FSVM detects DILATE events
    → Analytical layer measures DILATE rate (proprioceptive signal)
        → Rate informs axial adjustment of sensor position
            → New position changes input geometry
                → Different bits become "adjacent"
                    → Different DILATE profile
                        → Loop continues until convergence
```

This is **proprioception**, not control. There is no central controller. The system stabilizes through feedback, just like biological organisms don't "decide" to balance — they ARE balancing.

### Convergence ≠ termination

In turn-based systems (current GPTs), you stop at max_tokens. In a proprioceptive FSVM array, you converge when:

1. DILATE rate → 0 (no more adjacency violations, Zeckendorf coherency achieved)
2. Zobrist sketch stabilizes (no state transitions detected)
3. Marker frequency drops below threshold (silence)

The system doesn't "finish." It **settles**. Like a ball rolling to the bottom of a bowl.

### Axial adjustment math

The physical mover shifts the sensor position, which changes the locality spacing:

```
spacing_effective = spacing_base × cos(θ)     // θ = axial angle
```

At θ = 0 (aligned), you get base spacing. As θ increases, effective spacing changes, which changes which patterns trigger DILATE. The analytical layer controls θ based on dilation rate feedback.

This is a **multidimensional coordinate system** for interpretation. Not a pipeline. Not a stack. A space the sensor moves through.

---

## 5. What GPTs Get Wrong (And What FSVM Fixes)

### Current GPT paradigm (stepwise)

```
prompt → token → token → token → stop token / max_tokens
```

- State is KV cache (memory, not a state machine)
- Each step is a prediction, not a transition
- No sense of "where am I in the computation"
- Halting is external (truncation), not intrinsic (convergence)
- Tokenization is discrete, lossy, language-specific

### FSVM paradigm (continuous)

```
state → transition → transition → [dilate] → transition → converged (Zobrist stable)
```

- State IS the computation (Sketch, R, W are the machine)
- Each step is a state transition, not a prediction
- The machine knows its own coherence (Zobrist sketch)
- Halting is intrinsic (Zeckendorf coherency + sketch stability)
- Tokenization is analog, emergent, language-agnostic

### The translation to transformers

You don't need to retrain transformer weights. You need:

1. **FSVM array as input layer** — replaces tokenizer + embedding
2. **Zobrist sketch as state signal** — replaces positional encoding
3. **Dilation rate as proprioceptive feedback** — replaces fixed context window
4. **Convergence detection as halting** — replaces max_tokens

The transformer becomes a semantic interpreter operating on FSVM state vectors, not token embeddings. The "vocabulary" is the transponder array calibration. The "context window" is the dilation history. The "position encoding" is the Fibonacci indexing.

---

## 6. The Paper

### Title (working)

**"Proprioceptive State Machines: Fibonacci-Radix Streaming Computation with Continuous Convergence"**

Or shorter: **"The Language Ear: Analog Tokenization via Fibtransponder Arrays"**

### Core claims

1. A Fibonacci-radix streaming state machine (FSVM) achieves O(1) per-bit processing with built-in error correction via Zeckendorf coherency, at 46.55ns/op on commodity hardware.

2. Retrospective dilation enables semantic rescaling without data rewrite — the machine can adjust its interpretation of past data based on current signal, like biological proprioception.

3. An array of differently-calibrated FSVMs ("fibtransponder array") replaces discrete tokenization with analog sensing — language structure emerges from sensor geometry, not learned vocabulary.

4. Zobrist state sketching, folded into the core state machine (not bolted on as a sidecar), provides O(1) convergence detection via XOR-based coherence checking.

5. This architecture enables continuous-runtime computation models that converge intrinsically (homeostasis) rather than terminating extrinsically (truncation), breaking the stepwise turn-based paradigm of current transformer architectures.

### Key numbers

| Metric | Value | Notes |
|---|---|---|
| FSVM step | 46.55 ns/op | 0 allocs, Pentium N4200 |
| BitRope append | 14.78 ns/op | 0 allocs, storage substrate |
| Target (Zobrist-in-core) | ~48 ns/op | FSVM + one XOR |
| Classifier (sidecar) | 50-99 ns/op | Current, should be ~10-15ns after fix |
| Dilations per million bits | depends on signal | More adjacency = more DILATE = more adjustment |

### What NOT to claim

- This does not replace transformers. It replaces the INPUT LAYER of transformers (tokenizer + embedding + positional encoding).
- This does not eliminate training. It eliminates VOCABULARY TRAINING (BPE, etc.). The transponder calibrations (ZobristSeed tables) still need tuning.
- This is not proven on NLP tasks yet. The FSVM is proven on boolean streams. The transponder array for language is a proposed architecture.

---

## 7. Next Steps (Concrete)

### Immediate: Fix the Zobrist placement

**Goal:** Move Zobrist sketch from classifier sidecar into FSVM core.

**Files:**
- `internal/fsvm/fsvm.go` — add `Sketch uint64` to State, add `ZobristSeed [2]uint64` constant, fold XOR into Step()
- `internal/deltaqueue/classifier.go` — remove sketch computation, read from CoreDelta.Sketch instead
- `internal/deltaqueue/types.go` — add Sketch field to CoreDelta

**Proof:** FSVM benchmark stays at ~46-48ns (one XOR is negligible). Classifier benchmark drops to ~10-15ns (read-only).

### Near-term: Transponder array prototype

**Goal:** Implement 2-3 FSVM instances with different ZobristSeed calibrations, verify they detect different signal patterns.

**Approach:**
- Create `internal/transponder/array.go` with N FSVM instances
- Each has a different seed table (calibration)
- Feed the same bitstream to all, compare DILATE rates and Sketch values
- Verify that different calibrations produce different event profiles

**Benchmark:** Array of 3 transponders should process at ~50ns (parallel, not sequential — they're independent).

### Medium-term: Axial adjustment loop

**Goal:** Implement the proprioceptive feedback loop — measure DILATE rate, adjust locality spacing, observe signal change.

**Approach:**
- Add `Theta float64` to transponder state (axial angle)
- Dilation rate → theta adjustment: `theta += k * (dilate_rate - target_rate)`
- Effective spacing: `spacing = base_spacing * cos(theta)`
- Run for 1000 steps, verify convergence (DILATE rate → 0 or stable)

### Long-term: Language sensing prototype

**Goal:** Replace BPE tokenization with fibtransponder array for a real text corpus.

**Approach:**
- Encode text as UTF-8 bitstream
- Feed to transponder array with vowel/consonant/prosody calibrations
- Extract event profiles as "tokens"
- Compare compression ratio and information content vs BPE

---

## 8. Anti-Patterns (What Went Wrong Before)

### ❌ Sidecar Zobrist
Don't put derived properties in the classifier that should be core invariants. If it's computed every step, it belongs in Step().

### ❌ Spec-before-measurement
Don't write conformance targets before benchmarking the core. The spec should describe what's fast, not prescribe what's canonical.

### ❌ Layer accumulation
Every layer between FSVM and the analytical interpreter is overhead. If a layer doesn't read signals or cache meaning, it doesn't belong.

### ❌ Allocting on the hot path
Zero allocation is non-negotiable for the FSVM. One alloc per step is 5-10ns. Over a million bits, that's 5-10ms of garbage.

### ❌ Treating tokens as fundamental
Tokens are a discretization artifact. The FSVM operates on bits. Any token-level thinking should be in the analytical layer, never in the core.

---

## 9. Reading Order for New Contributors

1. This document (HANDOFF_VISION.md) — understand the WHY
2. `docs/SPEC.md` — the core state machine contract
3. `internal/fsvm/fsvm.go` — the implementation (small, read the whole thing)
4. `docs/BENCHMARKS.md` — the numbers that matter
5. `docs/EXPLORATIONS.md` — speculative directions (WHT, dilation trees, etc.)
6. `internal/deltaqueue/` — the sidecar (understand what NOT to put on the hot path)

Do NOT read the delta queue before understanding the FSVM. The delta queue is an implementation detail. The FSVM is the contribution.

---

## 10. The Stateful Purpose (Why This Document Exists)

Spec writers have a natural tendency to add layers: more types, more invariants, more conformance tests. This is good for correctness but bad for performance.

The fibtransponder's value proposition is speed and simplicity. A 46ns state machine that self-corrects via Zeckendorf mathematics. Every layer added on top should earn its cost in functionality.

The delta queue is useful. But it should never slow down the FSVM, and it should never be confused with the FSVM. The state machine is the thing. The queue is a thing that watches the state machine. Don't invert that relationship.

If a spec change makes the FSVM slower, the spec is wrong.
If a conformance test requires the classifier to do work the FSVM should do, the test is wrong.
If a type system makes the code more correct but twice as slow, the type system is wrong.

Speed is the invariant. Correctness is the constraint. Canonicality is optional.

---

*Generated: 2026-03-14 08:58 CET*  
*Context: Session discussing FSVM → transponder array → proprioceptive computation → language ear architecture*
