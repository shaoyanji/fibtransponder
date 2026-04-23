package transponder

import (
	"github.com/shaoyanji/fibtransponder/internal/fsvm"
)

// WindowMetrics captures per-transponder state over a fixed-size bit window.
type WindowMetrics struct {
	WindowIndex int
	BitCount    int
	Dilations   uint64
	Markers     uint64
	DilateRate  float64 // dilations / bitCount
	MarkerRate  float64 // markers / bitCount
	Sketch      uint64  // snapshot at end of window
	R           uint32
}

// CorpusReport holds per-transponder metrics for one input stream.
type CorpusReport struct {
	Label        string
	BitCount     int
	Transponders []TransponderReport
}

// TransponderReport holds one transponder's full metrics for one input.
type TransponderReport struct {
	Name         string
	TotalDil     uint64
	TotalMark    uint64
	FinalR       uint32
	FinalSketch  uint64
	WindowData   []WindowMetrics
}

// BytesToBits converts a byte slice to a bit slice (MSB first per byte).
func BytesToBits(data []byte) []uint8 {
	bits := make([]uint8, len(data)*8)
	for i, byt := range data {
		for j := 7; j >= 0; j-- {
			bits[i*8+(7-j)] = (byt >> uint(j)) & 1
		}
	}
	return bits
}

// RunCorpusExperiment feeds a bitstream through transponders, collecting
// windowed metrics. windowBits defines the window size for rate histograms.
func RunCorpusExperiment(label string, bits []uint8, cals []Calibration, windowBits int) CorpusReport {
	arr := NewArray(cals)
	nTrans := len(cals)
	nWindows := (len(bits) + windowBits - 1) / windowBits

	report := CorpusReport{
		Label:        label,
		BitCount:     len(bits),
		Transponders: make([]TransponderReport, nTrans),
	}

	for i, cal := range cals {
		report.Transponders[i].Name = cal.Name
		report.Transponders[i].WindowData = make([]WindowMetrics, nWindows)
	}

	// Track window-local counters
	windowDil := make([]uint64, nTrans)
	windowMark := make([]uint64, nTrans)

	for bitIdx, b := range bits {
		windowIdx := bitIdx / windowBits

		for tIdx := range arr.Transponders {
			t := &arr.Transponders[tIdx]
			prevDil := t.State.Dilations
			prevMark := t.State.Markers

			var evs []fsvm.Event
			t.State, evs = fsvm.Step(t.State, b)
			_ = evs

			windowDil[tIdx] += t.State.Dilations - prevDil
			windowMark[tIdx] += t.State.Markers - prevMark
		}

		// At window boundary, snapshot
		atWindowEnd := (bitIdx+1)%windowBits == 0
		atStreamEnd := bitIdx == len(bits)-1

		if atWindowEnd || atStreamEnd {
			bitsInWindow := windowBits
			if atStreamEnd && !atWindowEnd {
				bitsInWindow = (bitIdx % windowBits) + 1
			}

			for tIdx := range arr.Transponders {
				t := &arr.Transponders[tIdx]
				report.Transponders[tIdx].WindowData[windowIdx] = WindowMetrics{
					WindowIndex: windowIdx,
					BitCount:    bitsInWindow,
					Dilations:   windowDil[tIdx],
					Markers:     windowMark[tIdx],
					DilateRate:  float64(windowDil[tIdx]) / float64(bitsInWindow),
					MarkerRate:  float64(windowMark[tIdx]) / float64(bitsInWindow),
					Sketch:      t.State.Sketch,
					R:           t.State.R,
				}
				windowDil[tIdx] = 0
				windowMark[tIdx] = 0
			}
		}
	}

	// Fill totals
	for i := range arr.Transponders {
		t := arr.Transponders[i]
		report.Transponders[i].TotalDil = t.State.Dilations
		report.Transponders[i].TotalMark = t.State.Markers
		report.Transponders[i].FinalR = t.State.R
		report.Transponders[i].FinalSketch = t.State.Sketch
	}

	return report
}
