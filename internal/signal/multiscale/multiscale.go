package multiscale

import "errors"

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

// ------------------------------------------------------------------
// Overlapping windows
// ------------------------------------------------------------------

// SummarizeWindows computes a Summary for every window of size windowSize
// sliding across bits with the given overlap.
//
// hop = windowSize - overlap.  The first window covers bits[0:windowSize],
// the second covers bits[hop:hop+windowSize], and so on.
// Windows that would extend past the end of the slice are omitted — only
// full windows are emitted.
//
// Setting overlap = 0 yields non-overlapping windows.
func SummarizeWindows(bits []byte, windowSize, overlap int) ([]Summary, error) {
	if windowSize <= 0 {
		return nil, errors.New("window size must be > 0")
	}
	if overlap < 0 || overlap >= windowSize {
		return nil, errors.New("overlap must be in [0, windowSize)")
	}
	hop := windowSize - overlap
	var out []Summary
	for start := 0; start+windowSize <= len(bits); start += hop {
		out = append(out, ComputeSummary(bits[start:start+windowSize]))
	}
	return out, nil
}

// Slider computes multiscale summaries over a sliding overlapping window
// on a streaming bit source.
//
// Create with NewSlider, push bits with Push, and read summaries with
// Summaries.  The zero value is NOT usable.
type Slider struct {
	windowSize int
	overlap    int
	hop        int

	buf    []byte
	write  int
	filled bool

	sinceLast int      // samples since last emitted summary
	sums      []Summary
}

// NewSlider creates a sliding-window analyzer.
//
// windowSize is the number of bits in each summary window.
// overlap is the number of bits shared between consecutive windows.
// hop = windowSize - overlap.
func NewSlider(windowSize, overlap int) (*Slider, error) {
	if windowSize <= 0 {
		return nil, errors.New("window size must be > 0")
	}
	if overlap < 0 || overlap >= windowSize {
		return nil, errors.New("overlap must be in [0, windowSize)")
	}
	return &Slider{
		windowSize: windowSize,
		overlap:    overlap,
		hop:        windowSize - overlap,
		buf:        make([]byte, windowSize),
	}, nil
}

// WindowSize returns the configured window size.
func (s *Slider) WindowSize() int { return s.windowSize }

// Overlap returns the configured overlap.
func (s *Slider) Overlap() int { return s.overlap }

// Hop returns the advance between consecutive windows.
func (s *Slider) Hop() int { return s.hop }

// Push feeds bits into the sliding window.
// Summaries are computed automatically once the window is full.
func (s *Slider) Push(bits []byte) {
	for _, b := range bits {
		s.buf[s.write] = b & 1
		s.write++
		if s.write >= s.windowSize {
			s.write = 0
			if !s.filled {
				s.filled = true
				s.emit()
				s.sinceLast = 0
				continue
			}
		}
		if s.filled {
			s.sinceLast++
			if s.sinceLast >= s.hop {
				s.emit()
				s.sinceLast -= s.hop
			}
		}
	}
}

// Summaries returns all summaries computed so far.
// The caller may slice the result to process incrementally.
func (s *Slider) Summaries() []Summary { return s.sums }

// Flush forces emission of a summary from the current window contents,
// regardless of hop position.  If the window has not yet filled, Flush
// is a no-op.
func (s *Slider) Flush() {
	if s.filled {
		s.emit()
		s.sinceLast = 0
	}
}

// Reset clears all state, including buffered bits and emitted summaries.
func (s *Slider) Reset() {
	s.write = 0
	s.filled = false
	s.sinceLast = 0
	s.sums = s.sums[:0]
	for i := range s.buf {
		s.buf[i] = 0
	}
}

func (s *Slider) emit() {
	frame := make([]byte, s.windowSize)
	for i := 0; i < s.windowSize; i++ {
		frame[i] = s.buf[(s.write+i)%s.windowSize]
	}
	s.sums = append(s.sums, ComputeSummary(frame))
}
