package autocorr

import (
	"math"
	"testing"
)

func TestAtLagZero(t *testing.T) {
	// Any stream autocorrelated at lag 0 should be 1.0 (perfect correlation with itself).
	bits := []byte{1, 0, 1, 1, 0, 0, 1}
	got := AtLag(bits, 0)
	if math.Abs(got-1.0) > 1e-9 {
		t.Errorf("lag 0 should be 1.0, got %f", got)
	}
}

func TestAtLagPeriodic(t *testing.T) {
	// Periodic stream: 101010... period 2.
	bits := []byte{1, 0, 1, 0, 1, 0, 1, 0, 1, 0}
	// Lag 1: perfectly anti-correlated → -1.0
	got1 := AtLag(bits, 1)
	if math.Abs(got1-(-1.0)) > 1e-9 {
		t.Errorf("lag 1 of period-2 stream should be -1.0, got %f", got1)
	}
	// Lag 2: perfectly correlated → 1.0
	got2 := AtLag(bits, 2)
	if math.Abs(got2-1.0) > 1e-9 {
		t.Errorf("lag 2 of period-2 stream should be 1.0, got %f", got2)
	}
}

func TestAtLagRandomish(t *testing.T) {
	// A stream with no obvious pattern should have low autocorrelation at lags > 0.
	bits := []byte{1, 1, 0, 1, 0, 0, 1, 0, 1, 1, 0, 0}
	got := AtLag(bits, 1)
	if math.Abs(got) > 0.5 {
		t.Errorf("lag 1 of unstructured stream should have |corr| < 0.5, got %f", got)
	}
}

func TestAtLags(t *testing.T) {
	bits := []byte{1, 0, 1, 0, 1, 0, 1, 0}
	lags := []int{0, 1, 2, 3}
	dst := make([]float64, len(lags))
	AtLags(dst, bits, lags)
	want := []float64{1.0, -1.0, 1.0, -1.0}
	for i, w := range want {
		if math.Abs(dst[i]-w) > 1e-9 {
			t.Errorf("lag %d: got %f, want %f", lags[i], dst[i], w)
		}
	}
}

func TestAtLagExhausted(t *testing.T) {
	bits := []byte{1, 0, 1}
	got := AtLag(bits, 5)
	if got != 0 {
		t.Errorf("lag beyond length should be 0, got %f", got)
	}
}

func BenchmarkAtLag1(b *testing.B) {
	bits := make([]byte, 4096)
	for i := range bits {
		bits[i] = byte(i & 1)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = AtLag(bits, 1)
	}
}

func BenchmarkAtLags4(b *testing.B) {
	bits := make([]byte, 4096)
	for i := range bits {
		bits[i] = byte(i & 3)
	}
	lags := []int{1, 2, 4, 8}
	dst := make([]float64, len(lags))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		AtLags(dst, bits, lags)
	}
}
