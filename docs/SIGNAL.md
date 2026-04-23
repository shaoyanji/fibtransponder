# Signal & decomposition module (design)

This module treats the boolean stream as a signal and provides transforms and decompositions.

## Goals
- Keep the FSVM core deterministic and bounded.
- Perform heavier analysis lazily on windows (marker-aligned or fixed-size).
- Provide multi-scale analysis tied to dilation events and zero-run markers.

## Transform options

### 1) Walsh–Hadamard transform (WHT)
- Works on ±1 signals.
- O(N log N) with only adds/subtracts.
- Natural for boolean patterns and fast in integer arithmetic.
- Implemented in Go: `internal/signal/wht`.

### 2) FFT on boolean windows
- Convert 0/1 to centered float or ±1.
- Apply radix-2 FFT per window.
- Must annotate outputs with effective sampling lattice (dilation counter r).

### 3) Autocorrelation / run-length statistics
- Cheap and stable.
- Useful under interference and superposition.

## Multidimensional embedding
Map time index to a grid (2D/3D) and apply spatial transforms:
- 2D FFT / DCT
- box-counting / fractal proxies
- tile-wise hexagram histograms

Implemented (sketch) in Go: `internal/signal/embed2d` with box-counting.

## Fractal/multiscale
Use markers (powers of two) as natural scale boundaries:
- counts across scales
- box-counting on 2D embedding

## Interface concept
- `WindowSource`: yields windows of bits with metadata `{cursorStart, cursorEnd, r, zeroRunStats}`
- `Transform`: consumes windows and emits feature vectors

This remains a separate layer from the transponder core.
