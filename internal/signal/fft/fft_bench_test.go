package fft

import "testing"

func BenchmarkFFT1024(b *testing.B) {
	bits := make([]uint8, 1024)
	for i := range bits {
		bits[i] = uint8(i) & 1
	}
	a := make([]complex128, 1024)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		FillBoolToCentered(a, bits)
		FFT(a)
		_ = a[0]
	}
}
