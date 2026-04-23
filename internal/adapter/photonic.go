package adapter

import "github.com/shaoyanji/fibtransponder/internal/signal/window"

// Photonic adapters bridge light intensity measurements to FSVM bit streams.
//
// Photons arrive asynchronously; the adapter must handle:
//   - Thresholding: intensity above/below a cutoff → 1/0
//   - Statistical sampling: photon counts in a window → bit
//   - Quantum noise: Poisson statistics affect reliability
//
// The default configuration uses direct thresholding with a configurable
// intensity cutoff.  For photon-counting modes, use PushFloat with the
// count and a higher frame size to average over more detection events.

// NewPhotonicPipeline creates an adapter for photonic input.
//
// threshold is the intensity cutoff: samples >= threshold → 1, else 0.
// frameSize determines how many samples are batched before quantization.
// For single-photon detection, frameSize=1 is appropriate.
// For analog photodiodes, frameSize=8..64 reduces noise.
func NewPhotonicPipeline(threshold float64, frameSize int) (*Pipeline, error) {
	cfg := PipelineConfig{
		Medium:      MediumPhotonic,
		FrameSize:   frameSize,
		Overlap:     0,
		Transform:   TransformNone,
		QuantLevels: 2,
		WindowFn:    window.Rectangular,
		Threshold:   threshold,
	}
	return NewPipeline(cfg)
}

// NewPhotonicCountingPipeline creates an adapter for photon-counting
// detectors.  It uses statistical sampling: if the mean photon count
// in a frame exceeds threshold, the frame encodes to 1.
func NewPhotonicCountingPipeline(threshold float64) (*Pipeline, error) {
	cfg := PipelineConfig{
		Medium:      MediumPhotonic,
		FrameSize:   16, // accumulate 16 detection windows
		Overlap:     0,
		Transform:   TransformNone,
		QuantLevels: 2,
		WindowFn:    window.Rectangular,
		Threshold:   threshold,
	}
	return NewPipeline(cfg)
}
