package rosetta

import (
	"math"
	"testing"
	"time"
)

// TestIsFibonacciExact verifies isFibonacci against the known sequence and
// against the previous 5n²±4 perfect-square characterization on the range
// where that identity does not overflow.
func TestIsFibonacciExact(t *testing.T) {
	known := []uint64{0, 1, 2, 3, 5, 8, 13, 21, 34, 55, 89, 144, 233, 377,
		610, 987, 1597, 2584, 4181, 6765, 10946, 17711, 28657, 46368, 75025}
	for _, n := range known {
		if !isFibonacci(n) {
			t.Errorf("isFibonacci(%d) = false, want true", n)
		}
	}
	nonFib := []uint64{4, 6, 7, 9, 10, 11, 12, 14, 15, 100, 1000, 12345, 99999}
	for _, n := range nonFib {
		if isFibonacci(n) {
			t.Errorf("isFibonacci(%d) = true, want false", n)
		}
	}
}

// TestIsFibonacciAgreesWithSquareIdentity cross-checks the table lookup
// against the original 5n²±4 test over the non-overflowing range.
//
// The reference is O(√x), so the sweep is deliberately bounded: 20k values is
// ample to cover every Fibonacci boundary below that point while keeping the
// test fast.
func TestIsFibonacciAgreesWithSquareIdentity(t *testing.T) {
	const limit = uint64(20000)
	for n := uint64(0); n <= limit; n++ {
		want := isPerfectSquareRef(5*n*n+4) || isPerfectSquareRef(5*n*n-4)
		if got := isFibonacci(n); got != want {
			t.Fatalf("n=%d: isFibonacci=%v, 5n²±4 identity=%v", n, got, want)
		}
	}
}

// TestFibTableCoversAllRepresentable verifies the table contains every
// Fibonacci number that fits in uint64, including the largest one
// (F93 = 12200160415121876738), and stops cleanly without overflowing.
func TestFibTableCoversAllRepresentable(t *testing.T) {
	const wantLen = 94
	if len(fibTable) != wantLen {
		t.Errorf("fibTable has %d entries, want %d (F0..F93)", len(fibTable), wantLen)
	}
	const wantLast = uint64(12200160415121876738) // F93
	if got := fibTable[len(fibTable)-1]; got != wantLast {
		t.Errorf("fibTable last entry = %d, want %d (F93)", got, wantLast)
	}
	// Non-decreasing; F1 == F2 == 1 is the only equal adjacent pair.
	for i := 1; i < len(fibTable); i++ {
		if fibTable[i] < fibTable[i-1] {
			t.Fatalf("fibTable not sorted at %d: %d then %d", i, fibTable[i-1], fibTable[i])
		}
	}
	// The term after the last one must not be representable.
	last, prev := fibTable[len(fibTable)-1], fibTable[len(fibTable)-2]
	if last <= math.MaxUint64-prev {
		t.Errorf("fibTable stops early: %d+%d = %d still fits in uint64", prev, last, prev+last)
	}
}

// TestIsFibonacciHandlesLargeValues exercises the range where 5*n*n wraps
// uint64 (n > ~1.9e9), which the previous implementation answered incorrectly.
func TestIsFibonacciHandlesLargeValues(t *testing.T) {
	cases := []struct {
		n    uint64
		want bool
	}{
		{2971215073, true},           // F47
		{4807526976, true},           // F48
		{7778742049, true},           // F49
		{12586269025, true},          // F50
		{20365011074, true},          // F51
		{10610209857723, true},       // F68
		{7540113804746346429, true},  // F92
		{12200160415121876738, true}, // F93, largest representable
		{2971215072, false},          // just below F47
		{4807526975, false},          // just below F48
		{10610209857722, false},      // just below F68
		{3416454622901757, false},
		{math.MaxUint64, false},
	}
	for _, c := range cases {
		if got := isFibonacci(c.n); got != c.want {
			t.Errorf("isFibonacci(%d) = %v, want %v", c.n, got, c.want)
		}
	}
}

// TestIsFibonacciConstantTime is the regression guard for the DoS: the old
// O(√x) square scan took ~60ms at n=2^25 and grew 4x per doubling. A binary
// search over 93 entries must be effectively instant at any magnitude.
func TestIsFibonacciConstantTime(t *testing.T) {
	// Warm up.
	for i := 0; i < 1000; i++ {
		_ = isFibonacci(uint64(i))
	}
	for _, n := range []uint64{1 << 20, 1 << 25, 1 << 40, 1 << 62, math.MaxUint64} {
		start := time.Now()
		for i := 0; i < 1000; i++ {
			_ = isFibonacci(n + uint64(i))
		}
		el := time.Since(start)
		if el > 50*time.Millisecond {
			t.Errorf("isFibonacci near n=%d took %v for 1000 calls; work must not scale with n", n, el)
		}
	}
}

// isPerfectSquareRef is the original O(sqrt(x)) reference implementation,
// kept only to cross-check isFibonacci on the non-overflowing range.
func isPerfectSquareRef(x uint64) bool {
	if x == 0 {
		return true
	}
	var i uint64 = 1
	for i*i <= x {
		if i*i == x {
			return true
		}
		i++
	}
	return false
}
