package transponder

import (
	"testing"

	"github.com/shaoyanji/fibtransponder/internal/fsvm"
)

func BenchmarkArrayStepBitByBit(b *testing.B) {
	cals := []Calibration{CalibrationTight, CalibrationMedium, CalibrationWide}
	arr := NewArray(cals)
	words := make([]uint64, 1024)
	for i := range words {
		words[i] = uint64(i)*0x9e3779b97f4a7c15 + 0x517cc1b727220a95
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		word := words[i%len(words)]
		for bit := 0; bit < 64; bit++ {
			b := uint8((word >> bit) & 1)
			arr.Step(b)
		}
	}
}

func BenchmarkArrayStepWord64(b *testing.B) {
	cals := []Calibration{CalibrationTight, CalibrationMedium, CalibrationWide}
	arr := NewArray(cals)
	words := make([]uint64, 1024)
	for i := range words {
		words[i] = uint64(i)*0x9e3779b97f4a7c15 + 0x517cc1b727220a95
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		arr.StepWord64(words[i%len(words)])
	}
}

func BenchmarkArrayStepWord64AllZeros(b *testing.B) {
	cals := []Calibration{CalibrationTight, CalibrationMedium, CalibrationWide}
	arr := NewArray(cals)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		arr.StepWord64(0)
	}
}

func BenchmarkArrayStepWord64AllOnes(b *testing.B) {
	cals := []Calibration{CalibrationTight, CalibrationMedium, CalibrationWide}
	arr := NewArray(cals)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		arr.StepWord64(^uint64(0))
	}
}

// BenchmarkFSVMStepWord64Single isolates the core FSVM word step.
func BenchmarkFSVMStepWord64Single(b *testing.B) {
	st := fsvm.New()
	words := make([]uint64, 1024)
	for i := range words {
		words[i] = uint64(i)*0x9e3779b97f4a7c15 + 0x517cc1b727220a95
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		st, _ = fsvm.StepWord64(st, words[i%len(words)])
	}
	_ = st
}
