package image_analyzer

import (
	"fmt"

	"github.com/shaoyanji/fibtransponder/internal/extension"
	"github.com/shaoyanji/fibtransponder/internal/fsvm"
)

// AnalysisResult holds metrics derived from image bitstream patterns.
type AnalysisResult struct {
	EdgeDetections   uint64 // Count of rapid changes in window (potential edges)
	TextureVariations uint64 // Count of higher entropy windows (i.e., not all 0s or all 1s)
	// SmoothnessScore float64 // TBD: This would require more sophisticated logic
}

// Analyzer is a component that interprets FSVM state for image patterns.
type Analyzer struct {
	previousWindowValue uint8 // Keep track of the previous window's value to detect changes
	latestAnalysis      AnalysisResult
	latestOutput        extension.Output
}

// NewAnalyzer creates a new Analyzer.
func NewAnalyzer() *Analyzer {
	a := &Analyzer{}
	// Initialize with a default state
	// Note: initial fsvm.New() state has W=0, so previousWindowValue will be 0 initially
	a.latestAnalysis = AnalysisResult{} // Start with zero counts
	a.latestOutput = a.GetOutput()
	return a
}

// GetTitle returns a short title for this extension.
func (a *Analyzer) GetTitle() string {
	return "Image Analysis"
}

// ProcessBit is called for each incoming bit, allowing the extension to update its internal state.
func (a *Analyzer) ProcessBit(b uint8, fsvmState fsvm.State, zeroRunLength uint64, events []fsvm.Event) {
	a.latestAnalysis = a.analyzeInternal(fsvmState, b, zeroRunLength, events)
	a.latestOutput = a.GetOutput()
}

// analyzeInternal processes the current FSVM state and bit, updating image metrics.
func (a *Analyzer) analyzeInternal(fsvmState fsvm.State, latestBit uint8, zeroRunLength uint64, events []fsvm.Event) AnalysisResult {
	currentWindow := fsvmState.W
	
	// Edge detection: if window value changes significantly from previous
	// A simple heuristic: if a certain number of bits change.
	// For a 6-bit window, if more than 3 bits flipped, it's a "significant" change.
	// (XORing shows which bits changed, then count set bits)
	diffBits := currentWindow ^ a.previousWindowValue
	changedBitCount := 0
	for i := 0; i < 6; i++ {
		if (diffBits>>i)&1 == 1 {
			changedBitCount++
		}
	}
	if changedBitCount >= 3 { // Threshold for "edge"
		a.latestAnalysis.EdgeDetections++
	}
	a.previousWindowValue = currentWindow

	// Texture variations: count windows that are not uniformly black or white.
	// A window with many 0s and 1s (e.g., 000111 or 010101) suggests "texture."
	oneCount := 0
	for i := 0; i < 6; i++ {
		if (currentWindow>>i)&1 == 1 {
			oneCount++
		}
	}
	// If the window is not all zeros (0 ones) and not all ones (6 ones)
	if oneCount > 0 && oneCount < 6 {
		a.latestAnalysis.TextureVariations++
	}

	return a.latestAnalysis
}

// GetOutput returns the current displayable information from the extension.
func (a *Analyzer) GetOutput() extension.Output {
	return extension.Output{
		Title: a.GetTitle(),
		Lines: []string{
			fmt.Sprintf("  Edges Detected: %d", a.latestAnalysis.EdgeDetections),
			fmt.Sprintf("  Texture Variations: %d", a.latestAnalysis.TextureVariations),
		},
	}
}
