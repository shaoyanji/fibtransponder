package multiscale

import (
	"testing"
)

func TestRunLengths(t *testing.T) {
	bits := []byte{1, 1, 0, 0, 0, 1, 1, 1, 1, 0}
	runs := RunLengths(bits)
	want := []int{2, 3, 4, 1}
	if len(runs) != len(want) {
		t.Fatalf("expected %d runs, got %d", len(want), len(runs))
	}
	for i, w := range want {
		if runs[i] != w {
			t.Errorf("run %d: got %d, want %d", i, runs[i], w)
		}
	}
}

func TestRunLengthsSingleBit(t *testing.T) {
	bits := []byte{1}
	runs := RunLengths(bits)
	if len(runs) != 1 || runs[0] != 1 {
		t.Errorf("expected [1], got %v", runs)
	}
}

func TestRunLengthHistogram(t *testing.T) {
	runs := []int{1, 2, 3, 4, 5, 8, 9, 16}
	hist := RunLengthHistogram(runs)
	// bucket 0: [1] → 1
	// bucket 1: [2] → 1
	// bucket 2: [3-4] → 2
	// bucket 3: [5-8] → 2
	// bucket 4: [9-16] → 2
	want := []int{1, 1, 2, 2, 2}
	if len(hist) != len(want) {
		t.Fatalf("expected %d buckets, got %d: %v", len(want), len(hist), hist)
	}
	for i, w := range want {
		if hist[i] != w {
			t.Errorf("bucket %d: got %d, want %d", i, hist[i], w)
		}
	}
}

func TestTransitionDensity(t *testing.T) {
	// Alternating: max transitions
	alt := []byte{1, 0, 1, 0, 1, 0}
	if td := TransitionDensity(alt); td != 1.0 {
		t.Errorf("alternating should have td=1.0, got %f", td)
	}
	// Constant: zero transitions
	constant := []byte{1, 1, 1, 1}
	if td := TransitionDensity(constant); td != 0 {
		t.Errorf("constant should have td=0, got %f", td)
	}
	// Half transitions
	half := []byte{1, 1, 0, 0, 1, 1}
	if td := TransitionDensity(half); td != 0.4 {
		t.Errorf("half should have td=0.4, got %f", td)
	}
}

func TestOneDensity(t *testing.T) {
	bits := []byte{1, 0, 1, 0, 1, 1}
	if od := OneDensity(bits); od != 4.0/6.0 {
		t.Errorf("expected one density 4/6, got %f", od)
	}
}

func TestComputeSummary(t *testing.T) {
	bits := []byte{1, 1, 0, 0, 0, 1}
	s := ComputeSummary(bits)
	if s.WindowBits != 6 {
		t.Errorf("window bits: got %d, want 6", s.WindowBits)
	}
	if s.OneDensity != 3.0/6.0 {
		t.Errorf("one density: got %f, want 0.5", s.OneDensity)
	}
	if len(s.RunLengths) != 3 {
		t.Errorf("run lengths: got %d, want 3", len(s.RunLengths))
	}
}

func BenchmarkComputeSummary(b *testing.B) {
	bits := make([]byte, 4096)
	for i := range bits {
		bits[i] = byte(i & 1)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ComputeSummary(bits)
	}
}
