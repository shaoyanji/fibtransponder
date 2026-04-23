package transarray

import (
	"math"

	"github.com/shaoyanji/fibtransponder/internal/fsvm"
)

// Config describes a single transponder's parameters.
type Config struct {
	Width    uint32 // subsampling stride: 1=full, 2=every 2nd, etc.
	ZeroThresh uint32 // marker zero-run threshold (default 8)
}

// DefaultConfigs generates n transponders with varying scale parameters.
// Each transponder observes the bitstream at a different resolution and
// sensitivity, producing a diverse feature vector per window.
func DefaultConfigs(n int) []Config {
	// Multi-scale: width controls granularity, zeroThresh controls sensitivity
	widths := []uint32{1, 2, 3, 4, 5, 6, 7, 8}
	threshes := []uint32{3, 5, 8, 12, 16, 20, 24, 32}

	configs := make([]Config, 0, n)
	for _, w := range widths {
		for _, th := range threshes {
			if len(configs) >= n {
				break
			}
			configs = append(configs, Config{
				Width:      w,
				ZeroThresh: th,
			})
		}
		if len(configs) >= n {
			break
		}
	}
	return configs[:n]
}

// SignalVector is the fixed-dimensional output of one transponder per window.
// Fields are chosen to be small, bounded, and discriminative.
type SignalVector struct {
	Markers      uint32  // violation (power-of-2 zero-run) count
	DilateCount  uint32  // adjacency dilations triggered
	Density      float64 // markers per bit
	DilateRate   float64 // dilations per bit
	AutoCorr     float64 // lag-1 autocorrelation
	RunEntropy   float64 // entropy of run-length distribution
	SpectralFlux float64 // Walsh-Hadamard spectral flux
}

// Dim returns the dimensionality of a single transponder's signal.
func (SignalVector) Dim() int { return 9 }

// Flatten converts a transponder signal to a flat []float64.
func (s SignalVector) Flatten() []float64 {
	return []float64{
		float64(s.Markers),
		float64(s.DilateCount),
		s.Density,
		s.DilateRate,
		s.AutoCorr,
		s.RunEntropy,
		s.SpectralFlux,
	}
}

// ArrayOutput is the concatenation of all transponder signals at a window.
type ArrayOutput struct {
	Window   int       // window index
	Features []float64 // flat feature vector
}

// Array orchestrates multiple transponders in parallel.
type Array struct {
	Transponders []Transponder
	Size         int
}

// Transponder wraps one FSVM instance with scale-specific parameters.
type Transponder struct {
	Config Config
}

// NewArray creates a transponder array from configs.
func NewArray(configs []Config) *Array {
	arr := &Array{
		Transponders: make([]Transponder, len(configs)),
		Size:         len(configs),
	}
	for i, c := range configs {
		arr.Transponders[i] = Transponder{Config: c}
	}
	return arr
}

// Process processes a bitstream and returns per-window feature vectors.
func (a *Array) Process(bits []uint8) []ArrayOutput {
	if len(a.Transponders) == 0 {
		return nil
	}

	// Use largest window for alignment
	maxWin := 16 // default
	for i := range a.Transponders {
		// window not directly stored, derive from config
		w := int(a.Transponders[i].Config.Width)
		if w > maxWin {
			maxWin = w
		}
	}
	if maxWin < 8 {
		maxWin = 8
	}

	var outputs []ArrayOutput
	for start := 0; start+maxWin <= len(bits); start += maxWin / 2 {
		end := start + maxWin
		chunk := bits[start:end]

		features := make([]float64, 0, a.Size*7)
		for i := range a.Transponders {
			sig := a.Transponders[i].analyze(chunk)
			features = append(features, sig.Flatten()...)
		}

		outputs = append(outputs, ArrayOutput{
			Window:   start / (maxWin / 2),
			Features: features,
		})
	}

	return outputs
}

// analyze computes the signal vector for one transponder over a window.
// The transponder subsamples the bits at its configured stride (Width)
// and uses its configured zero-run threshold for markers. This means
// each transponder detects violations at a different scale and sensitivity.
func (t *Transponder) analyze(bits []uint8) SignalVector {
	// Sub-sample at configured stride.
	step := int(t.Config.Width)
	if step < 1 {
		step = 1
	}
	var sampled []uint8
	for i := 0; i < len(bits); i += step {
		sampled = append(sampled, bits[i])
	}

	state := fsvm.New()
	// Override zero-run threshold: the standard FSVM marks at powers of 2 >= 8.
	// For custom thresholds, we manually count zero-runs and compare.
	markerCount := uint32(0)
	dilateCount := uint32(0)
	zeroRun := uint64(0)
	for _, b := range sampled {
		st, evs := fsvm.Step(state, b)
		state = st
		for _, ev := range evs {
			if ev.Kind == fsvm.EventMarker {
				markerCount++
			}
			if ev.Kind == fsvm.EventDilate {
				dilateCount++
			}
		}
		// Also track custom-threshold zero runs from the sampled stream
		if b == 0 {
			zeroRun++
			if zeroRun >= uint64(t.Config.ZeroThresh) && isPowerOfTwo(zeroRun) {
				markerCount++
			}
		} else {
			zeroRun = 0
		}
	}

	n := float64(len(bits))
	if n < 1 {
		n = 1
	}

	return SignalVector{
		Markers:      markerCount,
		DilateCount:  dilateCount,
		Density:      float64(markerCount) / n,
		DilateRate:   float64(dilateCount) / n,
		AutoCorr:     lag1AutoCorr(sampled),
		RunEntropy:   runLengthEntropy(sampled),
		SpectralFlux: spectralFluxWH(sampled),
	}
}

func isPowerOfTwo(n uint64) bool {
	return n > 0 && (n&(n-1)) == 0
}

// lag1AutoCorr computes lag-1 autocorrelation of a bitstream.
func lag1AutoCorr(bits []uint8) float64 {
	if len(bits) < 3 {
		return 0
	}
	n := float64(len(bits) - 1)
	mean := 0.0
	for _, b := range bits {
		mean += float64(b)
	}
	mean /= (n + 1)

	var num, den float64
	for i := 0; i < len(bits)-1; i++ {
		ai := float64(bits[i]) - mean
		bi := float64(bits[i+1]) - mean
		num += ai * bi
		den += ai * ai
	}
	if den == 0 {
		return 0
	}
	return num / den
}

// runLengthEntropy computes entropy of run lengths in a bitstream.
func runLengthEntropy(bits []uint8) float64 {
	if len(bits) == 0 {
		return 0
	}

	runLengths := make(map[int]int)
	currentRun := 1
	for i := 1; i < len(bits); i++ {
		if bits[i] == bits[i-1] {
			currentRun++
		} else {
			runLengths[currentRun]++
			currentRun = 1
		}
	}
	runLengths[currentRun]++

	total := 0
	for _, c := range runLengths {
		total += c
	}

	entropy := 0.0
	for _, c := range runLengths {
		p := float64(c) / float64(total)
		if p > 0 {
			entropy -= p * math.Log2(p)
		}
	}
	return entropy
}

// spectralFluxWH computes spectral flux via fast Walsh-Hadamard transform.
func spectralFluxWH(bits []uint8) float64 {
	n := 1
	for n < len(bits) {
		n *= 2
	}

	signal := make([]float64, n)
	for i, b := range bits {
		if i < len(bits) {
			signal[i] = float64(b)
		}
	}

	for step := 1; step < n; step *= 2 {
		for i := 0; i < n; i += 2 * step {
			for j := i; j < i+step; j++ {
				a, b := signal[j], signal[j+step]
				signal[j] = a + b
				signal[j+step] = a - b
			}
		}
	}

	flux := 0.0
	for i := 1; i < n; i++ {
		flux += math.Abs(signal[i] - signal[i-1])
	}
	return flux / float64(n)
}

// FeatureDim returns total output dimensionality for N transponders + byte features.
// byteFeaturesPerWindow = 3 (ByteEntropy, AlphaRatio, PunctuationRatio)
func FeatureDim(n int) int { return n*7 + 3 }

// ExtractFeatures is a convenience function: process text → features.
func ExtractFeatures(text string, nTransponders int) []ArrayOutput {
	bits := textToBits(text)
	bytes := []byte(text)
	configs := DefaultConfigs(nTransponders)
	arr := NewArray(configs)
	outputs := arr.Process(bits)

	// Append byte-level features. Process uses bit windows of size maxWin
	// with 50% overlap stride. Map this back to char windows.
	// Each char = 8 bits, so char stride = maxWin/(2*8).
	maxWin := 8
	for _, c := range configs {
		if c.Width > uint32(maxWin) {
			maxWin = int(c.Width)
		}
	}
	if maxWin < 8 {
		maxWin = 8
	}
	bitStride := maxWin / 2
	bitWin := bits
	_ = bitWin

	// Simpler: just append the same 3 byte features derived from the
	// full text to every window. The byte features are global for a given
	// text and serve as a reference frame.
	textFeats := appendTextFeatures(bytes)

	for i := range outputs {
		// For window i, take a slice of bytes centered at the window's bit position
		bitPos := i * bitStride
		bytePos := bitPos / 8
		byteWin := 16
		start := bytePos - byteWin/2
		end := start + byteWin
		if start < 0 {
			start = 0
		}
		if end > len(bytes) {
			end = len(bytes)
		}
		if start >= end || len(bytes) == 0 {
			// Use global text features as fallback
			outputs[i].Features = append(outputs[i].Features, textFeats...)
		} else {
			// Use local byte features
			outputs[i].Features = append(outputs[i].Features, appendTextFeatures(bytes[start:end])...)
		}
	}

	return outputs
}

func appendTextFeatures(bytes []byte) []float64 {
	n := float64(len(bytes))
	if n == 0 {
		return []float64{0, 0, 0}
	}

	// Byte entropy
	freq := make(map[byte]int)
	for _, b := range bytes {
		freq[b]++
	}
	byteEntropy := 0.0
	for _, c := range freq {
		p := float64(c) / n
		if p > 0 {
			byteEntropy -= p * math.Log2(p)
		}
	}

	// Alpha ratio (a-z, A-Z, space)
	alphaCount := 0
	puncCount := 0
	for _, b := range bytes {
		if (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || b == ' ' {
			alphaCount++
		}
		if (b >= '!' && b <= '/') || (b >= ':' && b <= '@') ||
			(b >= '[' && b <= '`') || (b >= '{' && b <= '~') {
			puncCount++
		}
	}

	return []float64{byteEntropy, float64(alphaCount) / n, float64(puncCount) / n}
}

// textToBits converts text to MSB-first bits.
func textToBits(s string) []uint8 {
	bits := make([]uint8, 0, len(s)*8)
	for _, c := range s {
		b := byte(c)
		for i := 7; i >= 0; i-- {
			bits = append(bits, (b>>uint(i))&1)
		}
	}
	return bits
}
