package adapter

import "github.com/shaoyanji/fibtransponder/internal/signal/window"

// Electrical adapters provide convenience constructors for digital/bit streams.
// These are the simplest adapters — no transform, just pass-through with
// optional buffering.

// NewElectricalPipeline creates an adapter for direct digital input.
// Samples are bits (0/1) pushed via PushBit/PushByte and read via Next.
func NewElectricalPipeline(frameSize int) (*Pipeline, error) {
	cfg := DefaultConfig()
	cfg.Medium = MediumElectrical
	cfg.FrameSize = frameSize
	cfg.Transform = TransformNone
	cfg.QuantLevels = 2
	cfg.WindowFn = window.Rectangular
	return NewPipeline(cfg)
}

// NewElectricalWordPipeline creates an adapter that emits 64-bit words
// directly from the input stream without per-bit processing overhead.
// This is the fastest path for already-digital data.
func NewElectricalWordPipeline() (*Pipeline, error) {
	cfg := PipelineConfig{
		Medium:      MediumElectrical,
		FrameSize:   64,
		Overlap:     0,
		Transform:   TransformNone,
		QuantLevels: 2,
		WindowFn:    window.Rectangular,
	}
	return NewPipeline(cfg)
}
