# Related Work — fibtransponder

## Tokenization

Subword tokenization is the dominant paradigm for preparing text input to neural models. BPE adapted from compression to NMT by Sennrich et al. [sennrich2016bpe] iteratively merges frequent character pairs into a fixed vocabulary. SentencePiece [kudo2018sentencepiece] extends this with language-independent training on raw sentences, removing the need for word-boundary pre-tokenization. Both methods are discrete, learned, and language-specific — the vocabulary is a function of the training corpus.

Byte-level models eliminate tokenization entirely. Kalchbrenner et al. [kalchbrenner2016bytenet] proposed ByteNet, a dilated convolutional network operating on raw bytes with linear-time decoding. Xue et al. [xue2022byt5] showed that standard Transformers can process byte sequences directly (ByT5), achieving competitive performance with subword models while gaining robustness to noise and multilingual coverage. However, byte-level models pay a quadratic attention cost on longer sequences, and byte sequences are 3-5x longer than subword sequences for equivalent text.

The FSVM differs from both approaches. It does not learn a vocabulary (unlike BPE), and it does not process bytes through a neural network (unlike ByT5). Instead, it extracts structural features from the boolean stream using a fixed state machine — the "token" is which transponders are firing and at what dilation rate, not a discrete symbol from a learned set.

## Streaming Algorithms

The FSVM's O(1) per-step processing connects to the streaming algorithms literature. Alon, Matias, and Szegedy [alon1996ams] introduced the AMS sketch for estimating frequency moments in a single pass over a data stream, establishing the theoretical framework for bounded-memory stream processing. The Count-Min Sketch [cormode2005countmin] provides sublinear-space summaries for point, range, and inner product queries. Muthukrishnan's survey [muthukrishnan2005streams] catalogs the field's core results.

These sketches share the FSVM's commitment to bounded work per input symbol and fixed memory. The key difference: AMS and Count-Min are randomized approximations of aggregate statistics over a stream, while the FSVM is a deterministic state machine that tracks structural properties (dilation, markers, zero runs) without approximation. The Zobrist sketch in the FSVM is more akin to a state fingerprint than a frequency estimator — it provides identity and divergence detection, not frequency counts.

## Fibonacci Representations

Zeckendorf's theorem [zeckendorf1972representation] proves that every positive integer has a unique representation as a sum of non-consecutive Fibonacci numbers. This representation has error-correcting properties: the no-adjacency constraint means single-bit errors produce detectable violations. Lekkerkerker [lekkerkerker1972fibbonacci] independently proved related results on Fibonacci representations.

The FSVM uses Zeckendorf coherency as a structural invariant. DILATE events mark violations of the no-adjacency constraint, and the dilation exponent r counts the minimum number of zero-insertions needed to restore coherency. This connects the FSVM to the combinatorial properties of Fibonacci representations, but applies them as a streaming detection mechanism rather than a static encoding.

## Zobrist Hashing

Zobrist [zobrist1970hashing] introduced incremental XOR-based hashing for game-playing programs, enabling O(1) hash updates when a single board element changes. The FSVM adapts this technique: the sketch updates by one XOR per input bit, folding in both the bit value and the window state. The original Zobrist scheme uses random precomputed tables for each board position; the FSVM uses a 2-element seed table (one seed per bit value) with window-state modulation, making it a rolling state fingerprint rather than a board-position hash.

## State Space Models

Structured state space models (SSMs) such as S4 [gu2022s4] and Mamba [gu2023mamba] process sequences through continuous-time state transitions with linear complexity in sequence length. Mamba introduces input-dependent state transitions (selective SSMs) that allow the model to propagate or forget information based on content.

The FSVM shares SSMs' commitment to state-based processing and O(1) per-step cost, but differs in three ways:
1. The FSVM state is discrete and bounded (56 bytes), not a high-dimensional continuous vector
2. The FSVM's transitions are input-dependent through adjacency detection, not learned
3. The FSVM operates on raw bits, not token embeddings

The dilation mechanism in the FSVM has a loose analogy to Mamba's selective propagation: both adjust their "reception" of past information based on current input. But the FSVM's mechanism is geometric (Fibonacci-index rescaling), not learned.

## Biological Signal Processing

The fibtransponder array draws explicit analogy to cochlear frequency analysis. Von Békésy [bekesy1960experiments] demonstrated that the basilar membrane performs spatial frequency decomposition: different positions along the membrane resonate at different frequencies, with no discrete frequency vocabulary. Bregman [bregman1990auditory] extended this into auditory scene analysis, showing how the auditory system segregates sound sources through streaming.

The transponder array applies this principle to boolean streams: different transponders, calibrated with different adjacency widths and marker thresholds, resonate at different structural frequencies. Just as hair cells with different physical spacings detect different sound frequencies, transponders with different geometric parameters detect different bit-pattern structures. The calibration is the geometry, not the seed — this distinction was empirically validated by the seed-only falsification experiment and the structural calibration demonstration.
