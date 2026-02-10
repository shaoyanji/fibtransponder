# Natural explorations (15-minute burst)

This doc is intentionally speculative: small, concrete exploration threads that can turn into modules.

## 1) Boolean transport → WHT-first over FFT

For boolean streams, WHT often beats FFT as a first-line spectral-ish decomposition:
- the domain is naturally ±1
- no complex arithmetic
- robust to time-domain aliasing in a way that can be easier to interpret

Actionable:
- Provide a `WindowSource` that emits power-of-two windows (prefer marker-aligned, fall back to fixed-size)
- Compute WHT spectrum and expose top-K coefficients as features

## 2) Dilation as a multiscale tree (fractal-ish)

Each DILATE event `r++` defines a new scale. A stream with frequent DILATE resembles a multiscale branching process:
- raw bits at base scale
- virtual stuffed zeros at higher r

Exploration: define features on the dilation event process:
- `r(t)`
- inter-dilation intervals
- power spectral density of the dilation event train itself

## 3) Triplet grammar view (regular language)

Canonical Zeckendorf strings are exactly binary strings without `11`.
That language is regular; its DFA is 2-state.

Exploration: treat segmentation ambiguity + dilation triggers as a *product automaton*:
- DFA for `no-11` legality
- NFA for allowed segmentation at sparse markers
- scalar r counter

Deliverable direction: `internal/segauto` implementing bitset NFA update + exemplar extraction.

## 4) Multi-dimensional embeddings for interference diagnosis

Embedding the last N bits into a 2D grid gives a fast visual fingerprint of structured interference.
Simple box-counting is a crude but cheap multiscale measure.

Exploration:
- tile-wise hexagram histograms (local pattern counts) per row/column
- 2D autocorrelation on the grid (still integer ops)

## 5) Fourier via boolean transport: window metadata matters

If dilation changes the effective sampling lattice, any Fourier feature must be paired with:
- current r
- marker scale
- window duration in physical time (if available)

So the feature vector should be `(r, window_len, top_wht_components, box_counts, ...)`.

## 6) Fractal / multiscale summaries without floating point

You can get surprisingly far with integer-only multiscale summaries:
- box counts at sizes 1,2,4,8,...
- run-length counts at thresholds 2^k
- WHT coefficient tail statistics

These play nicely with DoS resistance.

## 7) A pragmatic 'boolean PLL'

Even if we avoid inferred priors, we can define a deterministic control law:
- if DILATE frequency crosses thresholds, raise an "undersampling" alarm
- if zeroRun distributions drift, raise a "silence bias" alarm

This is still measurement-first; it’s just turning counters into alerts.
