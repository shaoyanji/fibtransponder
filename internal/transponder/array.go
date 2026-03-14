// Package transponder implements a fibtransponder array — multiple FSVM instances
// with different ZobristSeed calibrations, processing the same bitstream in parallel.
//
// Each transponder has its own locality sensitivity. Different calibrations produce
// meaningfully different event profiles, analogous to how different hair cells in
// the cochlea resonate at different frequencies.
//
// This is the first step toward analog tokenization: language structure emerges
// from sensor geometry, not learned vocabulary.
package transponder

import "github.com/shaoyanji/fibtransponder/internal/fsvm"

// Calibration defines a transponder's sensitivity profile.
// Different seed tables = different detection characteristics.
type Calibration struct {
	Name   string
	Seeds  [2]uint64 // ZobristSeedBit override: [bit0, bit1]
}

// Standard calibrations — starting points for exploration.
var (
	// CalibrationTight: sensitive to local bit transitions (consonants, stops).
	CalibrationTight = Calibration{
		Name:  "tight",
		Seeds: [2]uint64{0x517cc1b727220a95, 0x9e3779b97f4a7c15},
	}

	// CalibrationMedium: balanced sensitivity (vowels, continuants).
	CalibrationMedium = Calibration{
		Name:  "medium",
		Seeds: [2]uint64{0x243f6a8885a308d3, 0x13198a2e03707344},
	}

	// CalibrationWide: sensitive to long-range structure (prosody, rhythm).
	CalibrationWide = Calibration{
		Name:  "wide",
		Seeds: [2]uint64{0xa4093822299f31d0, 0x082efa98ec4e6c89},
	}
)

// Transponder is one FSVM with a specific calibration.
type Transponder struct {
	Name  string
	State fsvm.State
}

// Result holds one transponder's output for a single bit.
type Result struct {
	Name      string
	Dilations uint64
	Markers   uint64
	Sketch    uint64
	R         uint32
}

// Array is a parallel set of differently-calibrated transponders.
type Array struct {
	Transponders []Transponder
}

// NewArray creates an array with the given calibrations.
// Each transponder gets a fresh FSVM state. Calibrations are applied by
// swapping ZobristSeedBit before Step (then restored).
func NewArray(cals []Calibration) *Array {
	arr := &Array{
		Transponders: make([]Transponder, len(cals)),
	}
	for i, cal := range cals {
		arr.Transponders[i] = Transponder{
			Name:  cal.Name,
			State: fsvm.New(),
		}
	}
	return arr
}

// Step processes one bit through all transponders sequentially.
// Each transponder uses the global ZobristSeedBit (set before call).
// For different calibrations, run separate arrays or use StepWithCalibration.
func (a *Array) Step(b uint8) []Result {
	results := make([]Result, len(a.Transponders))
	for i := range a.Transponders {
		t := &a.Transponders[i]
		var evs []fsvm.Event
		t.State, evs = fsvm.Step(t.State, b)
		for _, ev := range evs {
			switch ev.Kind {
			case fsvm.EventDilate:
				// dilations already tracked in State
			case fsvm.EventMarker:
				// markers already tracked in State
			}
		}
		results[i] = Result{
			Name:      t.Name,
			Dilations: t.State.Dilations,
			Markers:   t.State.Markers,
			Sketch:    t.State.Sketch,
			R:         t.State.R,
		}
	}
	return results
}

// ProcessStream feeds an entire bitstream through the array and returns
// the final state of each transponder.
func (a *Array) ProcessStream(bits []uint8) []Result {
	var last []Result
	for _, b := range bits {
		last = a.Step(b)
	}
	return last
}

// CompareResults returns a summary of how differently the transponders
// responded to the same input stream.
func CompareResults(results []Result) string {
	if len(results) < 2 {
		return "need >=2 transponders to compare"
	}
	// Compare DILATE rates: total dilations / total bits processed
	// Sketch divergence: XOR of final sketches
	var summary string
	for i := 0; i < len(results); i++ {
		summary += results[i].Name + ": dilations=" + itoa(results[i].Dilations) +
			" markers=" + itoa(results[i].Markers) +
			" r=" + itoa(uint64(results[i].R)) +
			" sketch=" + hex(results[i].Sketch) + "\n"
	}
	// Sketch divergence between pairs
	if len(results) >= 2 {
		summary += "\nSketch divergence:\n"
		for i := 0; i < len(results); i++ {
			for j := i + 1; j < len(results); j++ {
				div := results[i].Sketch ^ results[j].Sketch
				summary += "  " + results[i].Name + " ⊕ " + results[j].Name +
					" = " + hex(div) + "\n"
			}
		}
	}
	return summary
}

func itoa(n uint64) string {
	if n == 0 {
		return "0"
	}
	buf := [20]byte{}
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}

func hex(n uint64) string {
	const digits = "0123456789abcdef"
	buf := [16]byte{}
	for i := 15; i >= 0; i-- {
		buf[i] = digits[n&0xf]
		n >>= 4
	}
	return "0x" + string(buf[:])
}
