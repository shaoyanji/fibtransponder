package signal

import (
	"fmt"
	"strings"

	"github.com/shaoyanji/fibtransponder/internal/extension"
	"github.com/shaoyanji/fibtransponder/internal/fsvm"
)

// Features represents a collection of extracted signal features.
type Features struct {
	OneDensityInWindow float64
	DilationFactor     uint32
}

// FeatureExtractor is a component that extracts signal features from the FSVM state.
type FeatureExtractor struct {
	latestFeatures Features
	latestOutput   extension.Output
}

// NewFeatureExtractor creates a new FeatureExtractor.
func NewFeatureExtractor() *FeatureExtractor {
	fe := &FeatureExtractor{}
	// Initialize with a default state to avoid nil dereference before first bit
	fe.latestFeatures = fe.extractInternal(fsvm.New())
	fe.latestOutput = fe.GetOutput()
	return fe
}

// GetTitle returns a short title for this extension.
func (fe *FeatureExtractor) GetTitle() string {
	return "Signal Features"
}

// ProcessBit is called for each incoming bit, allowing the extension to update its internal state.
func (fe *FeatureExtractor) ProcessBit(b uint8, fsvmState fsvm.State, zeroRunLength uint64, events []fsvm.Event) {
	fe.latestFeatures = fe.extractInternal(fsvmState)
	fe.latestOutput = fe.GetOutput()
}

// extractInternal extracts features from the FSVM state.
func (fe *FeatureExtractor) extractInternal(fsvmState fsvm.State) Features {
	oneCount := 0
	for i := 0; i < 6; i++ {
		if (fsvmState.W>>i)&1 == 1 {
			oneCount++
		}
	}
	oneDensity := float64(oneCount) / 6.0

	return Features{
		OneDensityInWindow: oneDensity,
		DilationFactor:     fsvmState.R,
	}
}

// GetOutput returns the current displayable information from the extension.
func (fe *FeatureExtractor) GetOutput() extension.Output {
	return extension.Output{
		Title: fe.GetTitle(),
		Lines: []string{
			fmt.Sprintf("  Window One Density: %.2f (Window: %06b)", fe.latestFeatures.OneDensityInWindow, fe.latestFeatures.DilationFactor),
			fmt.Sprintf("  Dilation: %d", fe.latestFeatures.DilationFactor),
		},
	}
}