# Fourier on boolean streams (notes)

This is a placeholder.

Two pragmatic transforms for boolean transport:

1) FFT on windowed boolean→bipolar (`0→-1, 1→+1`) or centered (`b-mean(b)`)
2) Walsh–Hadamard transform (WHT), which is naturally defined for ±1 vectors and can be faster/simpler than FFT for some boolean patterns.

Dilation events `r` must be included in spectral metadata because they alter the effective sampling lattice.
