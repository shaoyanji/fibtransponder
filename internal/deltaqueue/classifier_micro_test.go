package deltaqueue

import "testing"

// BenchmarkClassifyBaseline: full Classify() with realistic input.
func BenchmarkClassifyBaseline(b *testing.B) {
	cls := &ClassifierState{}
	core := CoreDelta{Tick: 1, StateID: 7, CoreFlags: FlagStateChanged | FlagCheckpointCrossed}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = Classify(core, cls)
		core.Tick++
	}
}

// BenchmarkClassifySketchOnly: what does the sketch read cost?
// This isolates the new read-only sketch path.
func BenchmarkClassifySketchOnly(b *testing.B) {
	core := CoreDelta{Tick: 1, Sketch: 0xDEADBEEF}
	var out uint64
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		out = core.Sketch
	}
	_ = out
}

// BenchmarkClassifyStepsSince: cost of the StepsSince increment loop (4 lanes).
func BenchmarkClassifyStepsSince(b *testing.B) {
	cls := &ClassifierState{}
	core := CoreDelta{Tick: 1}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Just the StepsSince increment loop
		for j := range cls.StepsSince {
			if cls.StepsSince[j] < StepsSinceSaturation {
				cls.StepsSince[j]++
			}
		}
		_ = core
	}
}

// BenchmarkClassifyAux: cost of AuxBuckets computation.
func BenchmarkClassifyAux(b *testing.B) {
	cls := &ClassifierState{
		HasPrev:     true,
		PrevStateID: 7,
		StepsSince:  [4]uint16{10, 20, 30, 40},
	}
	core := CoreDelta{Tick: 50, StateID: 7, CoreFlags: FlagCheckpointCrossed}
	var derived uint32 = FlagSegmentCandidate | FlagMarkerCandidate | FlagPatternRepeat
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = classifyAux(core, *cls, derived)
	}
}

// BenchmarkClassifyFlags: cost of flag derivation logic.
func BenchmarkClassifyFlags(b *testing.B) {
	core := CoreDelta{Tick: 1, StateID: 7, CoreFlags: FlagStateChanged | FlagCheckpointCrossed}
	var derived uint32
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		derived = 0
		if core.CoreFlags&FlagCheckpointCrossed != 0 {
			derived |= FlagSegmentCandidate
		}
		if core.CoreFlags&FlagCheckpointCrossed != 0 {
			derived |= FlagMarkerCandidate
		}
		if core.CoreFlags&FlagStateChanged != 0 {
			derived |= FlagNovelPattern
		}
	}
	_ = derived
}
