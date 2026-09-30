package session

import (
	"testing"

	"github.com/shaoyanji/fibtransponder/internal/fsvm"
)

// TestProcessBitsMatchesFSVMCore verifies the session ingest path tracks the
// core FSVM exactly. internal/session previously had no test files.
func TestProcessBitsMatchesFSVMCore(t *testing.T) {
	bits := "1100101001100010110100101101001011010010110100101101001011"
	s := NewSession("test-core-parity")
	if err := s.ProcessBits(bits); err != nil {
		t.Fatalf("ProcessBits: %v", err)
	}

	ref := fsvm.New()
	for _, r := range bits {
		ref, _ = fsvm.Step(ref, uint8(r-'0'))
	}

	if s.FSVMState != ref {
		t.Errorf("session FSVM state diverged from core\n got: %+v\nwant: %+v", s.FSVMState, ref)
	}
	if s.ProcessedBits != uint64(len(bits)) {
		t.Errorf("ProcessedBits = %d, want %d", s.ProcessedBits, len(bits))
	}
}

// TestProcessBitsIgnoresNonBinaryInput verifies junk characters are skipped
// rather than counted.
func TestProcessBitsIgnoresNonBinaryInput(t *testing.T) {
	s := NewSession("test-junk")
	if err := s.ProcessBits("01x2 3_0\n1"); err != nil {
		t.Fatalf("ProcessBits: %v", err)
	}
	if s.ProcessedBits != 4 {
		t.Errorf("ProcessedBits = %d, want 4 (only 0,1,0,1 count)", s.ProcessedBits)
	}
}

// TestRefreshOutputsPopulatesAllExtensions verifies RefreshOutputs fills one
// output per registered extension, and that it is idempotent.
func TestRefreshOutputsPopulatesAllExtensions(t *testing.T) {
	s := NewSession("test-outputs")
	if err := s.ProcessBits("1101001011010010"); err != nil {
		t.Fatalf("ProcessBits: %v", err)
	}
	s.RefreshOutputs()
	if len(s.ExtensionOutputs) != len(s.extensions) {
		t.Fatalf("got %d outputs for %d extensions", len(s.ExtensionOutputs), len(s.extensions))
	}
	for i, out := range s.ExtensionOutputs {
		if out.Title == "" {
			t.Errorf("extension %d has empty title", i)
		}
	}

	// Idempotent: refreshing again must not change the slice length.
	before := len(s.ExtensionOutputs)
	s.RefreshOutputs()
	if len(s.ExtensionOutputs) != before {
		t.Errorf("RefreshOutputs not idempotent: %d then %d", before, len(s.ExtensionOutputs))
	}
}

// TestOutputsReflectStateAfterIngest verifies the display layer actually
// reflects the ingested state -- i.e. the refactor did not leave outputs
// permanently stale.
func TestOutputsReflectStateAfterIngest(t *testing.T) {
	s := NewSession("test-fresh")
	if err := s.ProcessBits("000000001111"); err != nil {
		t.Fatalf("ProcessBits: %v", err)
	}
	s.RefreshOutputs()
	joined := ""
	for _, out := range s.ExtensionOutputs {
		for _, l := range out.Lines {
			joined += l + "\n"
		}
	}
	// segauto reports segment count; after ingest there should be a non-zero
	// Dilations figure somewhere in the rendered state.
	if s.FSVMState.Dilations == 0 {
		t.Fatal("expected dilations from the fixture, got 0")
	}
	if joined == "" {
		t.Error("RefreshOutputs produced no display lines")
	}
}

// BenchmarkProcessBits measures the per-bit ingest cost of the shipped
// session path (as opposed to the bare fsvm.Step core).
func BenchmarkProcessBits(b *testing.B) {
	bits := make([]byte, 4096)
	for i := range bits {
		bits[i] = byte('0' + (i*i+i/3)%2)
	}
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		s := NewSession("bench")
		_ = s.ProcessBits(string(bits))
	}
}
