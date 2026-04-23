package adapter

import "github.com/shaoyanji/fibtransponder/internal/signal/window"

// Acoustic adapters bridge audio amplitude samples to FSVM bit streams.
//
// Typical pipeline:
//   float64 samples (audio amplitude) → frame buffer → Hann window → FFT
//   → spectral energy bands → quantization → bits
//
// The default configuration uses 256-sample frames with 50% overlap,
// FFT transform, and 2-level quantization (energy above/below median).

// NewAcousticPipeline creates an adapter for audio/sound input.
func NewAcousticPipeline() (*Pipeline, error) {
	cfg := PipelineConfig{
		Medium:      MediumAcoustic,
		FrameSize:   256,
		Overlap:     128, // 50% overlap
		Transform:   TransformFFT,
		QuantLevels: 2,
		WindowFn:    window.Hann,
	}
	return NewPipeline(cfg)
}

// NewAcousticPipelineWithTransform creates an acoustic adapter with a
// custom transform (e.g. WHT for impulsive signals, autocorr for pitch).
func NewAcousticPipelineWithTransform(t Transform) (*Pipeline, error) {
	cfg := PipelineConfig{
		Medium:      MediumAcoustic,
		FrameSize:   256,
		Overlap:     128,
		Transform:   t,
		QuantLevels: 2,
		WindowFn:    window.Hann,
	}
	return NewPipeline(cfg)
}
