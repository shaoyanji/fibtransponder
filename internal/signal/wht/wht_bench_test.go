package wht

import "testing"

func BenchmarkFWHT1024(b *testing.B) {
	bits := make([]uint8, 1024)
	for i := range bits {
		bits[i] = uint8(i) & 1
	}
	a := make([]int, 1024)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		FillBoolToBipolar(a, bits)
		FWHT(a)
		_ = a[0]
	}
}
