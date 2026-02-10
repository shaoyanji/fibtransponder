package entropy_estimator

import (
	"fmt"
	"math"
	"strings"

	"github.com/shaoyanji/fibtransponder/internal/extension"
	"github.com/shaoyanji/fibtransponder/internal/fsvm"
)

// EntropyEstimate holds the local entropy calculation.
type EntropyEstimate struct {
	WindowEntropy float64 // Entropy of the current 6-bit window
}

// Estimator is a component that estimates local bitstream entropy.
type Estimator struct {
	latestEstimate EntropyEstimate
	latestOutput   extension.Output
}

// NewEstimator creates a new Estimator.
func NewEstimator() *Estimator {
	e := &Estimator{}
	// Initialize with a default state
	e.latestEstimate = e.estimateInternal(fsvm.New())
	e.latestOutput = e.GetOutput()
	return e
}

// GetTitle returns a short title for this extension.
func (e *Estimator) GetTitle() string {
	return "Entropy Estimate"
}

// ProcessBit is called for each incoming bit, allowing the extension to update its internal state.
func (e *Estimator) ProcessBit(b uint8, fsvmState fsvm.State, zeroRunLength uint64, events []fsvm.Event) {
	e.latestEstimate = e.estimateInternal(fsvmState)
	e.latestOutput = e.GetOutput()
}

// estimateInternal calculates the entropy of the current 6-bit window (fsvmState.W).
func (e *Estimator) estimateInternal(fsvmState fsvm.State) EntropyEstimate {
	window := fsvmState.W
	oneCount := 0
	for i := 0; i < 6; i++ {
		if (window>>i)&1 == 1 {
			oneCount++
		}
	}
	zeroCount := 6 - oneCount

	p0 := float64(zeroCount) / 6.0
	p1 := float64(oneCount) / 6.0

	h := 0.0
	if p0 > 0 {
		h -= p0 * math.Log2(p0)
	}
	if p1 > 0 {
		h -= p1 * math.Log2(p1)
	}

	return EntropyEstimate{
		WindowEntropy: h,
	}
}

// GetOutput returns the current displayable information from the extension.
func (e *Estimator) GetOutput() extension.Output {
	return extension.Output{
		Title: e.GetTitle(),
		Lines: []string{
			fmt.Sprintf("  Window Entropy: %.2f", e.latestEstimate.WindowEntropy),
		},
	}
}
