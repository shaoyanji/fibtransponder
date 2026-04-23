package signal

// Package signal is an applications layer that treats the boolean stream as a signal.
//
// It is intentionally separated from the fibtransponder core.
//
// Planned features:
// - windowing over the append-only rope
// - boolean→bipolar conversion
// - FFT adapters (requires external deps or a simple radix-2 implementation)
// - Walsh–Hadamard transform (boolean-native)
// - autocorrelation / run-length statistics
// - multiscale / fractal summaries
//
// TODO: implement once Go toolchain is available.
