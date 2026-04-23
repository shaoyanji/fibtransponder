package typing_analyzer

import (
	"fmt"

	"github.com/shaoyanji/fibtransponder/internal/extension"
	"github.com/shaoyanji/fibtransponder/internal/fsvm"
)

// AnalysisResult holds metrics derived from typing patterns.
type AnalysisResult struct {
	TotalKeyPresses uint64
	TotalPauses     uint64
	RapidSequences  uint64
	AvgZeroRun      float64 // Average length of '0' runs (pauses)
}

// Analyzer is a component that interprets FSVM state for typing patterns.
type Analyzer struct {
	totalKeyPresses uint64
	totalPauses     uint64
	zeroRunLengths  []uint64 // Store recent zero run lengths to calculate average
	latestAnalysis  AnalysisResult
	latestOutput    extension.Output
}

// NewAnalyzer creates a new Analyzer.
func NewAnalyzer() *Analyzer {
	a := &Analyzer{
		zeroRunLengths: make([]uint64, 0, 100), // Keep a history of up to 100 zero-run lengths
	}
	// Initialize with a default state
	a.latestAnalysis = a.analyzeInternal(fsvm.New(), 0, 0) // Pass dummy values for initial state
	a.latestOutput = a.GetOutput() // Generate initial output
	return a
}

// GetTitle returns a short title for this extension.
func (a *Analyzer) GetTitle() string {
	return "Typing Analysis"
}

// ProcessBit is called for each incoming bit, allowing the extension to update its internal state.
func (a *Analyzer) ProcessBit(b uint8, fsvmState fsvm.State, zeroRunLength uint64, events []fsvm.Event) {
	a.latestAnalysis = a.analyzeInternal(fsvmState, b, zeroRunLength)
	a.latestOutput = a.GetOutput()
}

// analyzeInternal processes the current FSVM state and a new bit, updating metrics.
func (a *Analyzer) analyzeInternal(fsvmState fsvm.State, latestBit uint8, zeroRunLength uint64) AnalysisResult {
	if latestBit == 1 {
		a.totalKeyPresses++
	} else {
		a.totalPauses++
	}

	// Track zero-run lengths for averaging
	// Only consider zero-runs that just started (latestBit is 0, but previous was 1)
	if latestBit == 0 && fsvmState.LastBit == 1 && zeroRunLength > 0 {
		a.zeroRunLengths = append(a.zeroRunLengths, zeroRunLength)
		if len(a.zeroRunLengths) > 100 { // Keep history manageable
			a.zeroRunLengths = a.zeroRunLengths[1:]
		}
	}

	// Calculate average zero-run
	var sumZeroRuns uint64
	for _, l := range a.zeroRunLengths {
		sumZeroRuns += l
	}
	avgZeroRun := 0.0
	if len(a.zeroRunLengths) > 0 {
		avgZeroRun = float64(sumZeroRuns) / float64(len(a.zeroRunLengths))
	}

	// Detect rapid sequences (e.g., two '1's adjacent in the window)
	rapidSequences := uint64(0)
	if (fsvmState.W&0b11) == 0b11 { // Check for '11' at the end of the window
		rapidSequences++
	}
	// (More complex pattern detection could go here)

	return AnalysisResult{
		TotalKeyPresses: a.totalKeyPresses,
		TotalPauses:     a.totalPauses,
		RapidSequences:  rapidSequences,
		AvgZeroRun:      avgZeroRun,
	}
}

// GetOutput returns the current displayable information from the extension.
func (a *Analyzer) GetOutput() extension.Output {
	return extension.Output{
		Title: a.GetTitle(),
		Lines: []string{
			fmt.Sprintf("  Keys: %d | Pauses: %d", a.latestAnalysis.TotalKeyPresses, a.latestAnalysis.TotalPauses),
			fmt.Sprintf("  Avg Pause Length: %.2f | Bursts: %d", a.latestAnalysis.AvgZeroRun, a.latestAnalysis.RapidSequences),
		},
	}
}
