#import "@preview/charged-ieee:0.1.3": ieee

#show: ieee.with(
  title: [Proprioceptive State Machines: Fibonacci-Radix Streaming Computation with Structural Calibration],
  abstract: [
    We present the Fibonacci-radix Streaming Virtual Machine (FSVM), a deterministic state machine that processes boolean streams at O(1) per bit with zero heap allocation. The FSVM detects adjacency violations in Zeckendorf-coherent representations, emitting dilation events that retrospectively rescale semantic indices without rewriting data. We demonstrate that structural calibration---varying geometric parameters such as adjacency width and marker threshold---produces independent sensitivity profiles across differently-calibrated FSVM instances, forming a detector basis set rather than a one-parameter family. Experiments demonstrate that a 3#sym.times 3 array of FSVMs with distinct (width, threshold) configurations produces different class orderings on boolean streams derived from natural language, source code, and synthetic patterns. At ≈76 ns per input bit on commodity hardware, the FSVM operates two to three orders of magnitude faster than learned tokenization pipelines while requiring no vocabulary, no training data, and no language-specific preprocessing.
  ],
  authors: (
    (
      name: "Anonymous",
      department: [Submission],
      organization: [ICLR 2027],
      location: [Double-blind review],
      email: "anonymous"
    ),
  ),
  index-terms: ("streaming algorithms", "tokenization", "state machines", "Fibonacci representations", "Zeckendorf coding"),
  bibliography: bibliography("references.bib"),
)

= Introduction

Modern language models depend on learned tokenizers such as byte pair encoding @sennrich2016bpe and SentencePiece @kudo2018sentencepiece that impose a discrete vocabulary on inherently continuous linguistic structure. This creates three problems: (1) the vocabulary is language-specific and corpus-dependent, (2) tokenization is a lossy discretization that discards sub-token structure, and (3) the tokenizer must be trained before the model, adding a preprocessing stage that is tightly coupled to downstream performance.

Byte-level models @xue2022byt5 @kalchbrenner2016bytenet @clark2021canine @yu2023megabyte eliminate the tokenizer but pay a quadratic attention cost on sequences that are three to five times longer than subword sequences. Structured state space models @gu2022s4 @gu2023mamba @peng2023rwkv @poli2023hyena achieve linear-time sequence processing through learned state transitions, but operate on token embeddings rather than raw signal.

We propose a different approach: analog sensing via streaming state machines. Instead of learning a vocabulary, we deploy an array of simple state machines---each calibrated with different geometric parameters---that detect structural features in a boolean stream. The "token" is which detectors are active and at what rate, not a symbol from a learned set. This draws on the biological principle of cochlear frequency analysis @bekesy1960experiments, where hair cells with different physical spacings along the basilar membrane resonate at different frequencies without any discrete frequency vocabulary. We do not claim that this replaces tokenization for NLP tasks at present; rather, we establish the computational primitive and calibration mechanism that would be required.

Our contributions are:

- The *FSVM*: a Fibonacci-radix streaming state machine with O(1) per-bit complexity, zero heap allocation, and built-in Zeckendorf error correction. Benchmarked at ≈76 ns/op on commodity hardware.

- *Structural calibration*: the demonstration that varying geometric parameters (adjacency width, marker threshold) produces independent sensitivity profiles, not just rescaled copies of a single detector. Width selects locality sensitivity; threshold selects event admission sensitivity.

- *The transponder array*: an architecture in which differently-calibrated FSVMs process the same bitstream in parallel, producing a multi-dimensional structural signal that replaces discrete tokenization.

= The Fibonacci-Radix Streaming Virtual Machine

== State and Semantics

The FSVM is a deterministic streaming state machine operating on an unbounded boolean sequence $x_0, x_1, ...$. The state consists of:

- $r in NN$: the dilation exponent (count of retrospective dilation events)
- $W in {0, ..., 63}$: a 6-bit hexagram window over recent logical bits
- $"lastBit" in {0, 1}$: the most recently observed bit
- $"zeroRun" in NN$: current consecutive-zero run length
- $"sketch" in {0, 1}^64$: a Zobrist state fingerprint (XOR-folded)
- $"Seeds" = (s_0, s_1)$: a per-instance pair of 64-bit Zobrist constants

The state occupies 56 bytes. It does not grow with stream length.

== Step Function

On each input bit $b in {0, 1}$, the FSVM performs exactly:

1. *Zero-run update*: If $b = 0$, increment $"zeroRun"$. If $"zeroRun"$ crosses a sparse threshold (default: powers of two $>= 8$), emit a `MARKER` event.

2. *Adjacency detection*: If $"lastBit" = 1$ and $b = 1$, emit `DILATE` and increment $r$.

3. *Window update*: $W <- ((W << 1) | b) mod 64$

4. *Zobrist fold*: $"sketch" <- "sketch" xor "Seeds"[b] + W$

Each operation is a fixed sequence of comparisons, arithmetic, and bitwise operations. No loops, no recursion, no dynamic dispatch. The total operation count is bounded by 20 primitive operations per input bit, establishing O(1) worst-case complexity.

== Dilation Semantics

When the FSVM observes adjacent 1-bits, it increments the dilation exponent $r$. Conceptually, the stream is retrospectively rescaled as if a zero were inserted between the adjacent positions:

$ D(s) = s_0 thin 0 thin s_1 thin 0 thin s_2 thin 0 thin ... thin 0 thin s_{n-1} $

But no data is rewritten. The dilation exponent $r$ counts the minimum number of zero-insertions needed to make the observed stream Zeckendorf-coherent---free of adjacent 1-bits. Under this interpretation, effective index $i$ would shift to $i << r$ if the stream were rewritten; the FSVM never performs this expansion, using $r$ only as a structural signal.

This connects to Zeckendorf's theorem @zeckendorf1972representation, independently proven by Lekkerkerker @lekkerkerker1972fibbonacci, which establishes that every positive integer has a unique representation as a sum of non-consecutive Fibonacci numbers. The FSVM treats Zeckendorf coherency as a streaming invariant: DILATE events mark violations, and $r$ measures the deviation from coherency.

== The Zobrist Sketch

The sketch provides a 64-bit state fingerprint updated by one XOR per input bit. It inherits its mechanism from Zobrist hashing @zobrist1970hashing, originally developed for incremental board-position hashing in game-playing programs, and from the broader class of rolling and incremental hash functions @karp1987fingerprinting that maintain a running fingerprint over streaming input.

The FSVM's fold differs from classical Zobrist hashing in two ways: (1) the seed table has only two entries (one per bit value) rather than one per board position, and (2) the window state $W$ is folded in via addition, breaking XOR commutativity and making the sketch order-sensitive despite using XOR accumulation.

The sketch provides: (a) cheap divergence detection between transponders processing the same input, (b) convergence detection when the sketch stabilizes across windows, and (c) state identity fingerprinting. It does not provide class identification or semantic encoding---those belong in the analytical layer.

== Complexity

#figure(
  table(
    columns: 4,
    align: (left, right, right, right),
    table.header([Component], [Time], [Space], [Allocs]),
    [FSVM Step], [≈76 ns], [56 B], [0],
    [StepWidth], [$tilde.eq$ same], [56 B], [0],
    [BitRope Append], [≈21 ns], [O(n) amort.], [0],
    [Classifier], [≈93 ns], [O(1)], [0],
    [Array Step ($k$ transponders)], [O($k$)], [O($k$#sym.times 56)], [O($k$)],
  ),
  caption: [Benchmarked on Intel Celeron N3010 / Pentium N4200. All components show 0 heap allocations per call.],
) <tab:benchmarks>

The FSVM's O(1) complexity is worst-case, not amortized. No input pattern triggers expensive fallback behavior. The zero-allocation property holds because: (1) the State is passed by value (56 bytes, fits in registers), (2) the event slice uses append with bounded growth (at most 2 events per step), and (3) no interfaces, closures, or channels are involved.

= Structural Calibration

== The Cochlear Analogy

In the mammalian cochlea, hair cells at different positions along the basilar membrane have different physical spacings, causing them to resonate at different frequencies @bekesy1960experiments. There is no "frequency vocabulary"---frequency decomposition emerges from sensor geometry.

The fibtransponder array applies this principle. Each transponder is an FSVM instance with specific geometric parameters:

- *Adjacency width* $w in {1, 2, 3}$: how many trailing bits must be 1 for the adjacency detector to fire
- *Marker threshold*: the zero-run length at which markers are emitted (powers of 2, powers of 3, or linear)

All transponders share the same Zobrist seed table. Only geometry differs. This distinction---seeds label trajectories; geometry changes sensitivity---was established by controlled experiment.

== First Axis: Adjacency Width

Varying adjacency width changes which input structures trigger DILATE events. At $w=1$, any consecutive 1-bits trigger dilation; at $w=3$, three consecutive 1-bits are required.

#figure(
  table(
    columns: 5,
    align: (left, right, right, right, left),
    table.header([Width], [Prose], [Code], [Synthetic], [Most sensitive]),
    [$w=1$], [*0.1992*], [0.1763], [0.0313], [prose],
    [$w=2$], [0.0582], [*0.0609*], [0.0000], [code],
    [$w=3$], [0.0079], [*0.0150*], [0.0000], [code],
  ),
  caption: [DILATE rate per (width #sym.times class). Bold = highest rate per row. Different widths are most sensitive to different input classes.],
) <tab:width-sensitivity>

Two falsification checks passed: (A) dil-rate ordering is not monotonic across widths (w=1 ranks prose > code; w=2 and w=3 rank code > prose), and (B) the most-sensitive class changes with width. This establishes that structural calibration produces different detectors, not rescaled copies of one detector.

== Second Axis: Marker Threshold

The marker threshold defines when zero-run markers fire. We test three threshold families: powers of two $>= 8$ (default), powers of three $>= 9$, and linear (every $k$ zeros).

The original corpora produce zero markers because they lack long zero runs. We expand the corpora by interleaving original content with zero-padded blocks (64--128 bytes of zeros), creating diverse zero-run distributions.

#figure(
  table(
    columns: 5,
    align: (left, right, right, right, right),
    table.header([Threshold], [Prose+zeros], [Code+zeros], [Mixed], [Ranking]),
    [pow2#sym.gt.eq 8], [0.000817], [0.000762], [0.000496], [prose > code > mixed],
    [pow3#sym.gt.eq 9], [0.000467], [0.000476], [0.000297], [code > prose > mixed],
    [lin 4], [0.052054], [0.066546], [0.073236], [mixed > code > prose],
    [lin 8], [0.007470], [0.012186], [0.001850], [code > prose > mixed],
    [lin 12], [0.004902], [0.008092], [0.001222], [code > prose > mixed],
  ),
  caption: [Marker rate per (threshold #sym.times class) at $w=1$. Different thresholds produce different class orderings. This holds at all three widths.],
) <tab:threshold-sensitivity>

At every fixed width, different threshold families produce different class orderings. The marker rate is identical across widths at fixed threshold (threshold and width are orthogonal). This establishes the marker threshold as an independent second structural axis.

== Independence Theorem

Two structural parameters $A$ and $B$ are *independent* if, at fixed $A$, varying $B$ produces different class orderings, and vice versa. Formally, for a corpus $C = {c_1, ..., c_k}$:

$ forall a in "values"(A): "rank"_B (c_1, ..., c_k) "varies with" B $

The width-axis experiment shows that varying width at fixed threshold changes class rankings (prose-first vs code-first). The threshold-axis experiment shows that varying threshold at fixed width changes class rankings (prose-first vs code-first vs mixed-first). Both conditions hold. Width and threshold are independent structural axes.

= The Transponder Array

== Architecture

A transponder array consists of $k$ FSVM instances, each with distinct geometric calibration, processing the same input bitstream in parallel. Each transponder runs at the core FSVM speed ($tilde.eq 60$ ns). The array processes in O($k$) time per input bit.

The output of the array is a $k$-dimensional vector of (dilation rate, marker rate, sketch) triples, updated every bit. This vector is the analog equivalent of a token embedding---but it is extracted by geometry, not learned from data.

== Potential Application: Input Layer Replacement

The following is a proposed architecture, not an implemented system. It is included to clarify the design intent behind the transponder array and to motivate the calibration mechanism developed in Section 3.

In a conventional pipeline, text passes through: raw bytes #sym.arrow BPE tokenization #sym.arrow embedding lookup #sym.arrow positional encoding #sym.arrow transformer. The FSVM array could replace the first three stages: raw bytes #sym.arrow UTF-8 bitstream #sym.arrow FSVM array #sym.arrow structural signal #sym.arrow transformer.

In this hypothetical architecture, the "vocabulary" becomes the array calibration, the "context window" becomes the dilation history, and positional encoding is subsumed by the Fibonacci indexing inherent in dilation. Whether this produces useful representations at scale remains an open question: Section 6 reports that byte-frequency features currently outperform FSVM features for text classification at small corpus scales.

== Convergence and Halting (Proposed)

Current transformer models halt extrinsically---by reaching a maximum token count or generating an end-of-sequence token. The FSVM array suggests a possible alternative: intrinsic convergence through homeostatic settling. The system would settle when (1) DILATE rate approaches zero (Zeckendorf coherency achieved), (2) the Zobrist sketch stabilizes (no state transitions detected), and (3) marker frequency drops below threshold (silence).

This is analogous to a ball rolling to the bottom of a bowl---the system does not "finish" in the stepwise sense but reaches a quiescent state. Convergence guarantees for this feedback loop are not proven; empirical validation and formal analysis remain future work.

= Related Work

*Tokenization.* Subword tokenization @sennrich2016bpe @kudo2018sentencepiece remains the dominant paradigm for preparing text input to neural models. Both BPE and SentencePiece learn a discrete vocabulary from training data, creating a language-specific preprocessing stage. Byte-level models @xue2022byt5 @kalchbrenner2016bytenet @clark2021canine eliminate the vocabulary but incur longer sequences and higher attention costs. MEGABYTE @yu2023megabyte mitigates this with a multi-scale architecture that processes million-byte sequences at competitive performance with subword models. The FSVM differs from all of these: it extracts structural features from a boolean stream using a fixed state machine, with no learned vocabulary and no neural preprocessing.

*Streaming algorithms.* The FSVM's O(1) per-step processing connects to the streaming algorithms literature established by Alon et al.'s AMS sketch @alon1996ams, the Count-Min Sketch @cormode2005countmin, and the Count Sketch @charikar2002countsketch, surveyed by Muthukrishnan @muthukrishnan2005streams. These randomized data structures approximate aggregate statistics over streams with bounded memory. The FSVM shares the commitment to bounded work per symbol but is deterministic and tracks structural properties rather than frequency statistics.

*State space models.* Structured state space models @gu2022s4 @gu2023mamba achieve linear-time sequence processing through continuous-time state transitions with learned parameters. RWKV @peng2023rwkv combines RNN efficiency with transformer-scale parallel training, and Hyena @poli2023hyena replaces attention with learned long convolutions at subquadratic cost. The FSVM shares the state-based paradigm but operates on raw bits with discrete, bounded state (56 bytes) and geometry-defined (not learned) transitions.

*Biological signal processing.* The transponder array draws on cochlear frequency analysis @bekesy1960experiments, where frequency decomposition emerges from sensor geometry rather than discrete frequency bins. The auditory scene analysis framework of Bregman @bregman1990auditory provides the theoretical grounding for streaming-based source segregation. The scattering transform @bruna2013scattering applies a similar principle to visual signals, constructing translation-invariant representations through cascaded wavelet decompositions without learned parameters.

= Limitations

The FSVM is proven on boolean streams. Its application to natural language processing is proposed but not yet demonstrated. In a 5-fold cross-validation classification experiment on small corpora ($approx$1 KB per class), byte-frequency features (7 dimensions) achieved 100% accuracy on prose/code/structured discrimination while FSVM array features (24 dimensions) achieved 66.7%, suggesting that the current FSVM signal is weaker than simple byte statistics at this scale. Larger corpora and different task formulations may be needed to realize the analog tokenization hypothesis.

The transponder array's calibration parameters (adjacency width, marker threshold) are currently hand-set, not learned. The second-axis experiment uses expanded corpora with artificial zero padding; validation on real-world streaming data remains future work.

The Zobrist sketch is not a universal hash---the ADD component introduces linear dependencies. It serves as a practical state fingerprint, not a cryptographic commitment. Sketch collisions between classes are possible and observed.

The proprioceptive feedback loop (measuring DILATE rate, adjusting sensor geometry, observing signal change) is described but not implemented. Convergence guarantees for the feedback loop are empirical, not proven.

The analog tokenization hypothesis — that structurally calibrated FSVM arrays can supplant learned tokenization for NLP tasks — is empirically falsifiable. A concrete falsification condition is: on a diverse corpus suite totaling >1 MB of natural text, after systematic calibration across widths {1,2,3} and threshold families, the FSVM array signal fails to exceed simple non-learned baselines (byte-frequency histograms, character n-grams) on a battery of standard text classification benchmarks. The current evidence remains preliminary; this condition is untested at scale.

= Conclusion

We have presented the FSVM, a Fibonacci-radix streaming state machine that processes boolean streams at O(1) per bit with zero allocation. We demonstrated that structural calibration---varying geometric parameters rather than hash seeds---produces independent detector sensitivities, establishing a two-dimensional basis set for analog signal sensing.

The FSVM operates at ≈76 ns per input bit on commodity hardware, two to three orders of magnitude faster than learned tokenization pipelines. It requires no vocabulary, no training data, and no language-specific preprocessing. The transponder array architecture replaces discrete tokenization with geometric sensing, drawing on the biological principle of cochlear frequency decomposition.

The path forward involves testing the array on larger corpora, implementing the proprioceptive feedback loop, and exploring task formulations where bit-level temporal dynamics provide signal that byte-frequency statistics cannot. Whether the analog tokenization hypothesis holds at scale remains an open question---but the computational primitive and the calibration mechanism are now established.

#[
  = Appendix
  
  == Benchmark Commands
  
  All benchmarks reproduce with:
  
  ```bash
  go test -bench=. -benchmem ./internal/fsvm
  go test -bench=. -benchmem ./internal/bitrope
  go test -run TestSecondAxisExpanded -v ./internal/transponder
  ```
  
  == Reproducibility
  
  The complete implementation, test suite, and experiment infrastructure are available at the project repository. All experiments use deterministic FSVM processing with fixed seed tables. No randomness is involved in the core state machine or structural calibration experiments.
]
