package rosetta

import (
	"fmt"
	"strings"

	"github.com/shaoyanji/fibtransponder/internal/extension"
	"github.com/shaoyanji/fibtransponder/internal/fsvm"
)

// Probe represents an interpretation or analysis of a marker.
type Probe struct {
	Type        string
	Value       interface{}
	Description string
}

// Rosetta is a component that interprets FSVM markers.
type Rosetta struct {
	rosettaProbes []Probe // History of probes
	latestOutput  extension.Output
}

// New creates a new Rosetta interpreter.
func New() *Rosetta {
	r := &Rosetta{
		rosettaProbes: make([]Probe, 0),
	}
	r.latestOutput = r.GetOutput() // Initialize output
	return r
}

// GetTitle returns a short title for this extension.
func (r *Rosetta) GetTitle() string {
	return "Rosetta Probes"
}

// ProcessBit is called for each incoming bit, allowing the extension to update its internal state
// and potentially react to FSVM events.
func (r *Rosetta) ProcessBit(b uint8, fsvmState fsvm.State, zeroRunLength uint64, events []fsvm.Event) {
	var currentProbes []Probe
	for _, ev := range events {
		switch ev.Kind {
		case fsvm.EventMarker:
			currentProbes = append(currentProbes, Probe{
				Type:        "ZeroRunMarker",
				Value:       zeroRunLength,
				Description: fmt.Sprintf("Zero-run length at marker: %d", zeroRunLength),
			})

			if zeroRunLength > 0 && isFibonacci(zeroRunLength) {
				currentProbes = append(currentProbes, Probe{
					Type:        "FibonacciZeroRun",
					Value:       zeroRunLength,
					Description: fmt.Sprintf("Significant marker: Zero-run length %d is a Fibonacci number", zeroRunLength),
				})
			}

			if fsvmState.W == 0b000000 {
				currentProbes = append(currentProbes, Probe{
					Type:        "AllZerosWindowAtMarker",
					Value:       fsvmState.W,
					Description: "FSVM window was all zeros at marker event",
				})
			}
		}
	}
	// Append current probes to history, keep manageable size
	r.rosettaProbes = append(r.rosettaProbes, currentProbes...)
	if len(r.rosettaProbes) > 20 { // Limit log to last 20 entries
		r.rosettaProbes = r.rosettaProbes[len(r.rosettaProbes)-20:]
	}

	r.latestOutput = r.GetOutput()
}

// GetOutput returns the current displayable information from the extension.
func (r *Rosetta) GetOutput() extension.Output {
	var lines []string
	if len(r.rosettaProbes) == 0 {
		lines = append(lines, "  No events yet...")
	} else {
		for _, probe := range r.rosettaProbes {
			lines = append(lines, fmt.Sprintf("  (%s): %s", probe.Type, probe.Description))
		}
	}
	return extension.Output{Title: r.GetTitle(), Lines: lines}
}

// isPerfectSquare checks if x is a perfect square.
func isPerfectSquare(x uint64) bool {
	if x == 0 {
		return true
	}
	var i uint64 = 1
	for i*i <= x {
		if i*i == x {
			return true
		}
		i++
	}
	return false
}

// isFibonacci checks if n is a Fibonacci number.
func isFibonacci(n uint64) bool {
	return isPerfectSquare(5*n*n + 4) || isPerfectSquare(5*n*n - 4)
}
