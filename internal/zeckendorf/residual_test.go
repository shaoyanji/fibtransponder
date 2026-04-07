package zeckendorf

import (
	"math/rand"
	"testing"
)

func TestResidualValidZeckendorf(t *testing.T) {
	// 10101010 has no adjacent 1s => residual = 0
	bits := []uint8{1, 0, 1, 0, 1, 0, 1, 0}
	if r := Residual(bits); r != 0 {
		t.Errorf("expected 0, got %f", r)
	}
}

func TestResidualAllOnes(t *testing.T) {
	// 11111111 => adj11 at every position
	bits := []uint8{1, 1, 1, 1, 1, 1, 1, 1}
	if r := Residual(bits); r != 1.0 {
		t.Errorf("expected 1.0, got %f", r)
	}
}

func TestResidualSingleBit(t *testing.T) {
	if r := Residual([]uint8{1}); r != 0 {
		t.Errorf("expected 0 for single bit, got %f", r)
	}
}

func TestResidualEmpty(t *testing.T) {
	if r := Residual([]uint8{}); r != 0 {
		t.Errorf("expected 0 for empty, got %f", r)
	}
}

func TestResidualMixed(t *testing.T) {
	// 110010110 => adj11 at indices [0,1] and [6,7] = 2/8
	bits := []uint8{1, 1, 0, 0, 1, 0, 1, 1, 0}
	if r := Residual(bits); r != 2.0/8.0 {
		t.Errorf("expected %f, got %f", 2.0/8.0, r)
	}
}

func TestResidualWindow(t *testing.T) {
	// All ones => every window should have residual 1.0
	bits := make([]uint8, 64)
	for i := range bits {
		bits[i] = 1
	}
	wr := ResidualWindow(bits, 16)
	if wr.Global != 1.0 {
		t.Errorf("expected mean 1.0, got %f", wr.Global)
	}
	if wr.Windows < 1 {
		t.Errorf("expected at least 1 window, got %d", wr.Windows)
	}
}

func TestResidualWindowEmpty(t *testing.T) {
	wr := ResidualWindow([]uint8{}, 16)
	if wr.Global != 0 {
		t.Errorf("expected mean 0, got %f", wr.Global)
	}
}

func TestProfile(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	bits := make([]uint8, 512)
	for i := range bits {
		bits[i] = uint8(rng.Intn(2))
	}

	prof := Profile(bits, []int{8, 16, 32, 64, 128})
	if len(prof) != 5 {
		t.Errorf("expected 5 entries, got %d", len(prof))
	}
	for _, p := range prof {
		if p.Mean < 0 || p.Mean > 1 {
			t.Errorf("residual out of range: %f", p.Mean)
		}
	}
}

func TestStructuredVsRandom(t *testing.T) {
	rng := rand.New(rand.NewSource(99))
	n := 2048

	// Structured: no adjacent 1s
	structured := make([]uint8, n)
	last := 0
	for i := range structured {
		if last == 1 {
			structured[i] = 0
		} else {
			structured[i] = uint8(rng.Intn(2))
		}
		last = int(structured[i])
	}

	// Random
	random := make([]uint8, n)
	for i := range random {
		random[i] = uint8(rng.Intn(2))
	}

	sProf := Profile(structured, []int{64})
	rProf := Profile(random, []int{64})

	// Structured should have strictly lower residual
	if sProf[0].Mean >= rProf[0].Mean {
		t.Errorf("structured residual (%.4f) should be < random (%.4f)",
			sProf[0].Mean, rProf[0].Mean)
	}
}

func BenchmarkResidual(b *testing.B) {
	rng := rand.New(rand.NewSource(1))
	bits := make([]uint8, 4096)
	for i := range bits {
		bits[i] = uint8(rng.Intn(2))
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Residual(bits)
	}
}

func BenchmarkResidualWindow(b *testing.B) {
	rng := rand.New(rand.NewSource(1))
	bits := make([]uint8, 4096)
	for i := range bits {
		bits[i] = uint8(rng.Intn(2))
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ResidualWindow(bits, 128)
	}
}
