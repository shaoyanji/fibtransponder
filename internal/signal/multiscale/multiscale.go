package multiscale

// Multiscale summaries over boolean streams.
//
// Operates on raw bit slices (not FSVM state) so it can be applied to
// any window of the stream.  All computations use integer arithmetic.

// RunLengths returns the lengths of consecutive equal-bit runs.
// For bits [1,1,0,0,0,1], returns [2,3,1].
func RunLengths(bits []byte) []int {
	if len(bits) == 0 {
		return nil
	}
	out := make([]int, 0, 16)
	current := bits[0] & 1
	length := 1
	for i := 1; i < len(bits); i++ {
		b := bits[i] & 1
		if b == current {
			length++
		} else {
			out = append(out, length)
			current = b
			length = 1
		}
	}
	out = append(out, length)
	return out
}

// RunLengthHistogram bins run lengths into power-of-two buckets.
// Returns counts for buckets [1], [2], [3-4], [5-8], [9-16], ...
// up to the smallest bucket that covers maxRun.
func RunLengthHistogram(runs []int) []int {
	if len(runs) == 0 {
		return nil
	}
	maxRun := 0
	for _, r := range runs {
		if r > maxRun {
			maxRun = r
		}
	}
	// Determine number of buckets: bucket 0 = [1], bucket 1 = [2],
	// bucket k = [2^{k-1}+1, 2^k] for k>=2.
	nBuckets := 1
	for (1 << (nBuckets - 1)) < maxRun {
		nBuckets++
	}
	counts := make([]int, nBuckets)
	for _, r := range runs {
		bucket := runBucket(r)
		counts[bucket]++
	}
	return counts
}

func runBucket(r int) int {
	if r == 1 {
		return 0
	}
	if r == 2 {
		return 1
	}
	b := 2
	for (1 << (b - 1)) < r {
		b++
	}
	return b - 1
}

// TransitionDensity returns the fraction of positions where bits[t] != bits[t+1].
func TransitionDensity(bits []byte) float64 {
	if len(bits) < 2 {
		return 0
	}
	trans := 0
	for i := 1; i < len(bits); i++ {
		if (bits[i] & 1) != (bits[i-1] & 1) {
			trans++
		}
	}
	return float64(trans) / float64(len(bits)-1)
}

// OneDensity returns the fraction of 1-bits.
func OneDensity(bits []byte) float64 {
	if len(bits) == 0 {
		return 0
	}
	ones := 0
	for _, b := range bits {
		if b&1 == 1 {
			ones++
		}
	}
	return float64(ones) / float64(len(bits))
}

// Summary collects all multiscale statistics for a window.
type Summary struct {
	WindowBits        int
	OneDensity        float64
	TransitionDensity float64
	RunLengths        []int
	RunHistogram      []int
}

// ComputeSummary returns a full multiscale summary for a bit window.
func ComputeSummary(bits []byte) Summary {
	runs := RunLengths(bits)
	return Summary{
		WindowBits:        len(bits),
		OneDensity:        OneDensity(bits),
		TransitionDensity: TransitionDensity(bits),
		RunLengths:        runs,
		RunHistogram:      RunLengthHistogram(runs),
	}
}
