package transponder

import (
	"github.com/shaoyanji/fibtransponder/internal/fsvm"
)

// MarkerThresholdFamily defines how marker thresholds are computed
// from the current zero-run length.
type MarkerThresholdFamily int

const (
	// PowersOf2 fires at zero runs of 8, 16, 32, ... (default FSVM behavior)
	PowersOf2 MarkerThresholdFamily = iota
	// PowersOf3 fires at zero runs of 9, 27, 81, ...
	PowersOf3
	// Linear fires every Min zeros (e.g., every 12 zeros starting at 12)
	Linear
)

func (f MarkerThresholdFamily) String() string {
	switch f {
	case PowersOf2:
		return "pow2>=8"
	case PowersOf3:
		return "pow3>=9"
	case Linear:
		return "lin"
	default:
		return "unknown"
	}
}

// MarkerThresholdConfig defines when markers fire.
type MarkerThresholdConfig struct {
	Family MarkerThresholdFamily
	Min    uint64 // minimum zero run length (only meaningful for Linear)
}

// Default threshold configs matching HANDOFF_VISION.md section 12.
var (
	ThresholdDefault = MarkerThresholdConfig{Family: PowersOf2}
	ThresholdPow3    = MarkerThresholdConfig{Family: PowersOf3}
	ThresholdLinear  = MarkerThresholdConfig{Family: Linear, Min: 12}
	ThresholdLinear8 = MarkerThresholdConfig{Family: Linear, Min: 8}
)

// thresholdFired returns true if a marker should fire at this zero-run length.
func thresholdFired(zeroRun uint64, cfg MarkerThresholdConfig) bool {
	switch cfg.Family {
	case PowersOf2:
		return zeroRun >= 8 && isPow2(zeroRun)
	case PowersOf3:
		return zeroRun >= 9 && isPow3(zeroRun)
	case Linear:
		min := cfg.Min
		if min == 0 {
			min = 12
		}
		return zeroRun >= min && zeroRun%min == 0
	default:
		return false
	}
}

func isPow3(x uint64) bool {
	if x == 0 {
		return false
	}
	for x > 1 {
		if x%3 != 0 {
			return false
		}
		x /= 3
	}
	return true
}

// StepFull runs one FSVM step with configurable adjacency width AND
// marker threshold. Seeds are always DefaultSeeds (structural calibration only).
func StepFull(s fsvm.State, b uint8, width AdjacencyWidth, thresh MarkerThresholdConfig) (fsvm.State, []fsvm.Event) {
	b &= 1
	var evs []fsvm.Event

	// update zero run + marker (threshold-aware)
	if b == 0 {
		s.ZeroRun++
		if thresholdFired(s.ZeroRun, thresh) {
			s.Markers++
			evs = append(evs, fsvm.Event{Kind: fsvm.EventMarker, Payload: s.ZeroRun})
		}
	} else {
		s.ZeroRun = 0
	}

	// adjacency detection with configurable width (same as StepWidth)
	adjacent := false
	switch width {
	case Width1:
		adjacent = s.LastBit == 1 && b == 1
	case Width2:
		adjacent = (s.W&0x03) == 0x03 && b == 1
	case Width3:
		adjacent = (s.W&0x07) == 0x07 && b == 1
	}

	if adjacent {
		s.R++
		s.Dilations++
		evs = append(evs, fsvm.Event{Kind: fsvm.EventDilate, Payload: uint64(s.R)})
	}

	s.LastBit = b
	s.W = ((s.W << 1) | b) & 0x3F

	// Zobrist fold (same seeds for all transponders)
	s.Sketch ^= s.Seeds[b] + uint64(s.W)

	return s, evs
}

// JointCalibration pairs a width with a threshold family.
type JointCalibration struct {
	Name      string
	Width     AdjacencyWidth
	Threshold MarkerThresholdConfig
}

// JointTransponder is one detector with both width and threshold calibration.
type JointTransponder struct {
	Name   string
	Width  AdjacencyWidth
	Thresh MarkerThresholdConfig
	State  fsvm.State
}

// JointArray holds transponders with different (width, threshold) pairs.
type JointArray struct {
	Transponders []JointTransponder
}

// NewJointArray creates transponders from joint calibrations, all using DefaultSeeds.
func NewJointArray(cals []JointCalibration) *JointArray {
	arr := &JointArray{Transponders: make([]JointTransponder, len(cals))}
	for i, c := range cals {
		arr.Transponders[i] = JointTransponder{
			Name:   c.Name,
			Width:  c.Width,
			Thresh: c.Threshold,
			State:  fsvm.New(),
		}
	}
	return arr
}

// Step feeds one bit to all joint transponders.
func (a *JointArray) Step(b uint8) []Result {
	results := make([]Result, len(a.Transponders))
	for i := range a.Transponders {
		t := &a.Transponders[i]
		var evs []fsvm.Event
		t.State, evs = StepFull(t.State, b, t.Width, t.Thresh)
		_ = evs
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

// ProcessStream feeds entire bitstream through the joint array.
func (a *JointArray) ProcessStream(bits []uint8) []Result {
	var last []Result
	for _, b := range bits {
		last = a.Step(b)
	}
	return last
}
