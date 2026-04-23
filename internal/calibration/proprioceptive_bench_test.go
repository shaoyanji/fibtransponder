package calibration

import "testing"

func BenchmarkAdaptiveArrayStepWord64(b *testing.B) {
	targets := DefaultTargets()
	arr := NewAdaptiveArray([]string{"t1", "t2", "t3"}, targets)
	arr.CalibrateInterval = 256

	words := make([]uint64, 1024)
	for i := range words {
		words[i] = uint64(i)*0x9e3779b97f4a7c15 + 0x517cc1b727220a95
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		arr.StepWord64(words[i%len(words)])
	}
}

func BenchmarkAdaptiveArrayStepWord64AllZeros(b *testing.B) {
	targets := DefaultTargets()
	arr := NewAdaptiveArray([]string{"t1", "t2", "t3"}, targets)
	arr.CalibrateInterval = 256

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		arr.StepWord64(0)
	}
}

func BenchmarkAdaptiveArrayStepWord64AllOnes(b *testing.B) {
	targets := DefaultTargets()
	arr := NewAdaptiveArray([]string{"t1", "t2", "t3"}, targets)
	arr.CalibrateInterval = 256

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		arr.StepWord64(^uint64(0))
	}
}
