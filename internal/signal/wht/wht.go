package wht

// Walsh–Hadamard transform utilities.
//
// WHT is a pragmatic first transform for boolean transport:
// - input mapped to +/-1
// - O(N log N) with only adds/subtracts
// - power-of-two windows

// FillBoolToBipolar writes 0->-1, 1->+1 into dst.
func FillBoolToBipolar(dst []int, bits []uint8) {
	n := len(bits)
	if len(dst) < n {
		panic("dst too small")
	}
	for i := 0; i < n; i++ {
		if bits[i]&1 == 1 {
			dst[i] = 1
		} else {
			dst[i] = -1
		}
	}
}

// FWHT performs an in-place, unnormalized fast Walsh–Hadamard transform.
// len(a) must be a power of two.
func FWHT(a []int) {
	n := len(a)
	for h := 1; h < n; h <<= 1 {
		step := h << 1
		for i := 0; i < n; i += step {
			for j := i; j < i+h; j++ {
				x := a[j]
				y := a[j+h]
				a[j] = x + y
				a[j+h] = x - y
			}
		}
	}
}

func PowerInto(dst []int64, a []int) {
	if len(dst) < len(a) {
		panic("dst too small")
	}
	for i, x := range a {
		dst[i] = int64(x) * int64(x)
	}
}
