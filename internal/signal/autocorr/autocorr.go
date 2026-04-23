package autocorr

// Small-lag autocorrelation for boolean streams.
//
// The input is treated as bipolar (-1 for 0, +1 for 1) so that
// autocorrelation at lag 0 equals the stream length (normalized
// to 1.0) and random noise decays to 0.

// AtLags computes autocorrelation coefficients for a set of lags.
// bits[i] should be 0 or 1.  dst must have len >= len(lags).
//
// The result at lag L is:
//   (1/N) * Σ_t bipolar(bits[t]) * bipolar(bits[t+L])
// where N = len(bits) - max(lag).
//
// Complexity: O(N * len(lags)).
func AtLags(dst []float64, bits []byte, lags []int) {
	if len(dst) < len(lags) {
		panic("dst too small")
	}
	if len(bits) == 0 || len(lags) == 0 {
		return
	}
	maxLag := 0
	for _, L := range lags {
		if L > maxLag {
			maxLag = L
		}
	}
	n := len(bits) - maxLag
	if n <= 0 {
		return
	}
	invN := 1.0 / float64(n)

	for i, L := range lags {
		sum := 0.0
		for t := 0; t < n; t++ {
			sum += bipolar(bits[t]) * bipolar(bits[t+L])
		}
		dst[i] = sum * invN
	}
}

// Single lag convenience function.
func AtLag(bits []byte, lag int) float64 {
	if lag >= len(bits) || len(bits) == 0 {
		return 0
	}
	n := len(bits) - lag
	sum := 0.0
	for t := 0; t < n; t++ {
		sum += bipolar(bits[t]) * bipolar(bits[t+lag])
	}
	return sum / float64(n)
}

// bipolar maps 0 -> -1.0, 1 -> +1.0.
func bipolar(b byte) float64 {
	if b&1 == 1 {
		return 1.0
	}
	return -1.0
}
