# Applications (exploration / draft)

This file captures *application directions* for the fibtransponder abstraction.

The core abstraction is a deterministic streaming transducer over booleans with:
- Zeckendorf/Fibonacci-radix legality features (e.g., `11` forbidden)
- global retrospective dilation events `r++` (virtual zero-stuffing / upsample-by-2)
- symbolic (regular-language) segmentation opportunities induced by long zero runs
- lazily-computed probes (magnitude/log2 bounds, modular fingerprints, etc.)

## 1) "Weather probe" / environmental stress meter

Observable outputs:
- dilation count `r(t)`
- dilation rate `dr/dt`
- zero-run statistics (distribution, power-of-two crossings)
- hexagram-window opcode frequencies (local pattern histogram)

Interpretation: not "entropy" in the Shannon sense, but a robust *stress/complexity signature* of the observed channel.

## 2) Boolean transport layer for Fourier analysis (signal → boolean → spectrum)

Goal: allow arbitrary sensor signals to be transported/encoded as a boolean stream, while still enabling meaningful downstream spectral analysis.

### Approach A: time-domain transport + downstream FFT
- Treat the boolean stream as a time series `b(t) ∈ {0,1}`.
- Use dilation `r` as a clock-resync / sampling lattice adjustment.
- Segment into windows (allowed segmentation policy) for analysis.
- Convert to centered signal `x(t)=b(t)-mean` or bipolar `x(t)∈{-1,+1}`.
- Compute FFT on windows.

Notes:
- Dilation changes effective sampling rate → spectral results must be annotated with `r`.
- Markers can serve as window boundaries / keyframes.

### Approach B: event (spike) transport
- Carry spike events as `1` separated by zeros.
- Spectral analysis via inter-spike interval transforms, Lomb–Scargle, or treating spikes as impulses.

### Approach C: multichannel boolean bundle
- Carry multiple boolean channels in parallel (vector-valued `b_c(t)`).
- Encode each channel in its own fibtransponder instance, or multiplex channels by time-division markers.

Deliverable concept:
- `internal/signal` module providing windowing + FFT adapters for boolean streams.

## 3) Signal decomposition module

Desired decompositions on boolean streams:
- run-length distribution / renewal process fit
- autocorrelation of boolean signal
- Walsh–Hadamard transform (boolean-native)
- wavelet-like multiscale summaries (dilation events provide natural scales)

A pragmatic initial set:
- rolling mean/variance
- rolling autocorrelation at a small set of lags
- WHT on fixed-size windows (power-of-two) because it matches boolean transport well

## 4) Multi-dimension extension

Interpret the stream as a 2D/3D field by mapping time index `t` to coordinates:
- 2D: `t → (x=t mod W, y=floor(t/W))`
- 3D: add z dimension similarly

Then apply spatial transforms:
- 2D FFT / DCT
- morphological operators
- per-tile hexagram histograms

This can help for:
- visualization dashboards
- detecting structured interference patterns

## 5) Fractal / multiscale analysis

Dilation events naturally create a multiscale hierarchy.

Possible analyses:
- estimate Hurst-like roughness using counts across scales (markers at 2^k zero-run crossings)
- box-counting dimension on 2D embedding of the boolean field
- multiscale entropy proxies (again, not claiming Shannon entropy)

Deliverable concept:
- `internal/signal/fractal` functions for multiscale summaries.

## 6) What matters to keep invariant

Even as applications diversify, the core invariants should remain:
- bounded ingest cost
- no materialization of stuffed zeros
- deterministic transducer
- symbolic ambiguity (regular language) rather than enumerated hypotheses
