package rosetta

import (
	"fmt"
	"math"
	"sort"

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

// fibTable holds every Fibonacci number representable in uint64 (F0..F93,
// 94 entries). Built once at init.
//
// The loop appends F(n), then appends the final term and stops if the
// following addition would overflow. Stopping on the addition (rather than on
// the value about to be appended) is what keeps F93 -- the largest
// Fibonacci representable in uint64 -- in the table.
var fibTable = buildFibTable()

func buildFibTable() []uint64 {
	out := make([]uint64, 0, 94)
	a, b := uint64(0), uint64(1)
	for {
		out = append(out, a)
		if a > math.MaxUint64-b {
			out = append(out, b) // F(n+1) is representable; F(n+2) is not
			break
		}
		a, b = b, a+b
	}
	return out
}

// isFibonacci reports whether n is a Fibonacci number.
//
// This is a binary search over the 94 uint64-representable Fibonacci numbers:
// O(log 94) and independent of n.
//
// The previous implementation tested the standard 5n²±4 perfect-square
// identity, but both the O(√x) square scan and the 5*n*n product are unsafe
// here. isPerfectSquare looped i from 1 upward, so cost grew with the value
// (~2^(k/2) iterations for a marker at zero-run 2^k), and 5*n*n silently
// wrapped uint64 for n > ~1.9e9. Because this is reached from the per-bit
// ingest path on every marker event, a long zero-run stalled ingest for
// seconds -- the opposite of the unDoSable property the spec claims.
func isFibonacci(n uint64) bool {
	i := sort.Search(len(fibTable), func(i int) bool { return fibTable[i] >= n })
	return i < len(fibTable) && fibTable[i] == n
}
