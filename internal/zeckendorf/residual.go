package zeckendorf

// Zeckendorf residual measurement for sliding bitstream windows.
//
// The Zeckendorf theorem states every positive integer has a unique
// representation as a sum of non-consecutive Fibonacci numbers. In
// binary terms, a canonical Zeckendorf word has no adjacent 1-bits.
//
// Residual = adj11_count / (bit_count - 1)
//
// Low residual = high structural regularity (prose, well-formed code).
// High residual = unstructured data (noise, random).
//
// This provides a continuous "surprisal" signal that can replace BPE's
// frequency-based merging as a tokenization criterion.

import (
	"math"
)

// Residual computes the fraction of adjacent 1-pairs in a bit window.
// Canonical Zeckendorf form has residual = 0.
func Residual(bits []uint8) float64 {
	if len(bits) < 2 {
		return 0
	}
	adj11 := 0
	for i := 1; i < len(bits); i++ {
		if (bits[i]&1) != 0 && (bits[i-1]&1) != 0 {
			adj11++
		}
	}
	return float64(adj11) / float64(len(bits)-1)
}

// WindowResult holds per-window residual statistics.
type WindowResult struct {
	Means    []float64 // mean residual per window
	Global   float64   // global mean residual
	Min      float64
	Max      float64
	StdDev   float64
	Windows  int
}

// ResidualWindow slides a fixed-size window over a bitstream and returns
// per-window residuals plus aggregate statistics.
func ResidualWindow(bits []uint8, windowSize int) WindowResult {
	if windowSize < 2 {
		windowSize = 2
	}
	n := len(bits)
	if n < windowSize {
		r := Residual(bits)
		return WindowResult{
			Means:   []float64{r},
			Global:  r,
			Min:     r,
			Max:     r,
			Windows: 1,
		}
	}

	step := windowSize / 2 // 50% overlap
	numWindows := 1 + (n-windowSize)/step
	if numWindows < 1 {
		numWindows = 1
	}

	means := make([]float64, 0, numWindows)
	for i := 0; i < numWindows; i++ {
		start := i * step
		end := start + windowSize
		if end > n {
			end = n
			start = end - windowSize
			if start < 0 {
				start = 0
			}
		}
		means = append(means, Residual(bits[start:end]))
	}

	// Aggregate statistics
	var sum, sumSq float64
	min := 1.0
	max := 0.0
	for _, r := range means {
		sum += r
		sumSq += r * r
		if r < min {
			min = r
		}
		if r > max {
			max = r
		}
	}
	mu := sum / float64(len(means))
	variance := sumSq/float64(len(means)) - mu*mu
	if variance < 0 {
		variance = 0
	}

	return WindowResult{
		Means:  means,
		Global: mu,
		Min:    min,
		Max:    max,
		StdDev: math.Sqrt(variance),
		Windows: len(means),
	}
}

// ProfileEntry is a single point in a multi-scale residual profile.
type ProfileEntry struct {
	WindowSize int
	Mean       float64
	Min        float64
	Max        float64
	StdDev     float64
	Windows    int
}

// Profile computes residuals at multiple window sizes to detect
// scale-dependent structure. Different linguistic features manifest
// at different scales (morphemes vs phrases vs discourse).
func Profile(bits []uint8, sizes []int) []ProfileEntry {
	// Filter out sizes larger than input
	valid := make([]int, 0, len(sizes))
	for _, s := range sizes {
		if s >= 2 && s <= len(bits) {
			valid = append(valid, s)
		}
	}
	if len(valid) == 0 {
		s := len(bits) / 2
		if s < 2 {
			s = 2
		}
		valid = []int{s}
	}

	entries := make([]ProfileEntry, 0, len(valid))
	for _, ws := range valid {
		wr := ResidualWindow(bits, ws)
		entries = append(entries, ProfileEntry{
			WindowSize: ws,
			Mean:       wr.Global,
			Min:        wr.Min,
			Max:        wr.Max,
			StdDev:     wr.StdDev,
			Windows:    wr.Windows,
		})
	}
	return entries
}
