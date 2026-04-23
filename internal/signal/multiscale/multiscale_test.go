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

// ------------------------------------------------------------------
// Overlapping window tests
// ------------------------------------------------------------------

func TestSummarizeWindowsNonOverlapping(t *testing.T) {
	bits := []byte{1, 1, 0, 0, 1, 1, 0, 0}
	sums, err := SummarizeWindows(bits, 4, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sums) != 2 {
		t.Fatalf("expected 2 windows, got %d", len(sums))
	}
	if sums[0].WindowBits != 4 || sums[0].OneDensity != 0.5 {
		t.Errorf("window 0: got %+v", sums[0])
	}
	if sums[1].WindowBits != 4 || sums[1].OneDensity != 0.5 {
		t.Errorf("window 1: got %+v", sums[1])
	}
}

func TestSummarizeWindowsOverlapping(t *testing.T) {
	// 8 bits, window=4, overlap=2 → hop=2
	// windows: [0:4], [2:6], [4:8]
	bits := []byte{1, 1, 0, 0, 1, 1, 0, 0}
	sums, err := SummarizeWindows(bits, 4, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sums) != 3 {
		t.Fatalf("expected 3 windows, got %d", len(sums))
	}
	// Window 0: [1,1,0,0] → density 0.5
	if sums[0].OneDensity != 0.5 {
		t.Errorf("window 0 density: got %f, want 0.5", sums[0].OneDensity)
	}
	// Window 1: [0,0,1,1] → density 0.5
	if sums[1].OneDensity != 0.5 {
		t.Errorf("window 1 density: got %f, want 0.5", sums[1].OneDensity)
	}
	// Window 2: [1,1,0,0] → density 0.5
	if sums[2].OneDensity != 0.5 {
		t.Errorf("window 2 density: got %f, want 0.5", sums[2].OneDensity)
	}
}

func TestSummarizeWindowsPartialFinal(t *testing.T) {
	// 7 bits, window=4, overlap=0 → hop=4
	// windows: [0:4], [4:8] but 8 > 7, so only [0:4]
	bits := []byte{1, 1, 1, 1, 0, 0, 0}
	sums, err := SummarizeWindows(bits, 4, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sums) != 1 {
		t.Fatalf("expected 1 window, got %d", len(sums))
	}
	if sums[0].OneDensity != 1.0 {
		t.Errorf("expected density 1.0, got %f", sums[0].OneDensity)
	}
}

func TestSummarizeWindowsInvalidOverlap(t *testing.T) {
	_, err := SummarizeWindows([]byte{1}, 4, 4)
	if err == nil {
		t.Error("expected error for overlap == windowSize")
	}
	_, err = SummarizeWindows([]byte{1}, 4, -1)
	if err == nil {
		t.Error("expected error for negative overlap")
	}
	_, err = SummarizeWindows([]byte{1}, 0, 0)
	if err == nil {
		t.Error("expected error for windowSize == 0")
	}
}

func TestSliderNonOverlapping(t *testing.T) {
	s, err := NewSlider(4, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	s.Push([]byte{1, 1, 0, 0, 1, 1, 0, 0})
	sums := s.Summaries()
	if len(sums) != 2 {
		t.Fatalf("expected 2 summaries, got %d", len(sums))
	}
	if sums[0].OneDensity != 0.5 || sums[1].OneDensity != 0.5 {
		t.Errorf("unexpected densities: %+v", sums)
	}
}

func TestSliderOverlapping(t *testing.T) {
	// window=4, overlap=2 → hop=2
	s, err := NewSlider(4, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// 8 bits → windows at bits 3, 5, 7 (0-indexed)
	// Actually: first emit after 4 bits (0..3), then every 2 bits
	// So at bit 3: emit [0..3], at bit 5: emit [2..5], at bit 7: emit [4..7]
	s.Push([]byte{1, 1, 0, 0, 1, 1, 0, 0})
	sums := s.Summaries()
	if len(sums) != 3 {
		t.Fatalf("expected 3 summaries, got %d", len(sums))
	}
	// [1,1,0,0] → density 0.5
	if sums[0].OneDensity != 0.5 {
		t.Errorf("summary 0 density: got %f, want 0.5", sums[0].OneDensity)
	}
	// [0,0,1,1] → density 0.5
	if sums[1].OneDensity != 0.5 {
		t.Errorf("summary 1 density: got %f, want 0.5", sums[1].OneDensity)
	}
	// [1,1,0,0] → density 0.5
	if sums[2].OneDensity != 0.5 {
		t.Errorf("summary 2 density: got %f, want 0.5", sums[2].OneDensity)
	}
}

func TestSliderStreaming(t *testing.T) {
	s, err := NewSlider(4, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Push first 4 bits → first summary
	s.Push([]byte{1, 1, 1, 1})
	if len(s.Summaries()) != 1 {
		t.Fatalf("expected 1 summary after 4 bits, got %d", len(s.Summaries()))
	}
	// Push 2 more bits → second summary
	s.Push([]byte{0, 0})
	if len(s.Summaries()) != 2 {
		t.Fatalf("expected 2 summaries after 6 bits, got %d", len(s.Summaries()))
	}
	// Push 2 more bits → third summary
	s.Push([]byte{0, 0})
	if len(s.Summaries()) != 3 {
		t.Fatalf("expected 3 summaries after 8 bits, got %d", len(s.Summaries()))
	}
}

func TestSliderFlush(t *testing.T) {
	s, err := NewSlider(4, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// 5 bits: fill window (4) + 1 extra
	s.Push([]byte{1, 1, 1, 1, 0})
	// Automatic summaries: after bit 3 (window 0..3), after bit 5 would be next
	// but we only have 5 bits, so only 1 automatic summary
	auto := len(s.Summaries())
	if auto != 1 {
		t.Fatalf("expected 1 automatic summary, got %d", auto)
	}
	// Flush should emit the current window regardless of hop
	s.Flush()
	if len(s.Summaries()) != 2 {
		t.Fatalf("expected 2 summaries after flush, got %d", len(s.Summaries()))
	}
}

func TestSliderReset(t *testing.T) {
	s, err := NewSlider(4, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	s.Push([]byte{1, 1, 1, 1})
	if len(s.Summaries()) != 1 {
		t.Fatalf("expected 1 summary, got %d", len(s.Summaries()))
	}
	s.Reset()
	if len(s.Summaries()) != 0 {
		t.Errorf("expected 0 summaries after reset, got %d", len(s.Summaries()))
	}
	// Should be able to push again
	s.Push([]byte{0, 0, 0, 0})
	if len(s.Summaries()) != 1 {
		t.Fatalf("expected 1 summary after re-push, got %d", len(s.Summaries()))
	}
	if s.Summaries()[0].OneDensity != 0 {
		t.Errorf("expected density 0 after reset, got %f", s.Summaries()[0].OneDensity)
	}
}

func TestSliderInvalidConfig(t *testing.T) {
	_, err := NewSlider(0, 0)
	if err == nil {
		t.Error("expected error for windowSize == 0")
	}
	_, err = NewSlider(4, 4)
	if err == nil {
		t.Error("expected error for overlap == windowSize")
	}
	_, err = NewSlider(4, -1)
	if err == nil {
		t.Error("expected error for negative overlap")
	}
}

func BenchmarkSummarizeWindowsOverlapping(b *testing.B) {
	bits := make([]byte, 4096)
	for i := range bits {
		bits[i] = byte(i & 1)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = SummarizeWindows(bits, 64, 32)
	}
}

func BenchmarkSliderOverlapping(b *testing.B) {
	bits := make([]byte, 4096)
	for i := range bits {
		bits[i] = byte(i & 1)
	}
	s, _ := NewSlider(64, 32)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.Reset()
		s.Push(bits)
	}
}
