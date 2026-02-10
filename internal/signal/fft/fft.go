package fft

// Minimal radix-2 FFT (complex128) for boolean windows.
//
// This is an applications-layer transform. It is not intended to be the fastest
// possible implementation; it provides a baseline without extra deps.

import (
	"math"
	"math/cmplx"
)

// FillBoolToCentered writes a centered complex window into dst.
// Mapping: 0->-0.5, 1->+0.5.
func FillBoolToCentered(dst []complex128, bits []uint8) {
	n := len(bits)
	if len(dst) < n {
		panic("dst too small")
	}
	for i := 0; i < n; i++ {
		if bits[i]&1 == 1 {
			dst[i] = complex(+0.5, 0)
		} else {
			dst[i] = complex(-0.5, 0)
		}
	}
}

// FFT performs an in-place radix-2 Cooley–Tukey FFT.
// len(a) must be power of two.
func FFT(a []complex128) {
	n := len(a)
	// bit reversal
	j := 0
	for i := 1; i < n; i++ {
		bit := n >> 1
		for ; j&bit != 0; bit >>= 1 {
			j &= ^bit
		}
		j |= bit
		if i < j {
			a[i], a[j] = a[j], a[i]
		}
	}

	for length := 2; length <= n; length <<= 1 {
		ang := -2 * math.Pi / float64(length)
		wlen := cmplx.Exp(complex(0, ang))
		for i := 0; i < n; i += length {
			w := complex(1, 0)
			for j := 0; j < length/2; j++ {
				u := a[i+j]
				v := a[i+j+length/2] * w
				a[i+j] = u + v
				a[i+j+length/2] = u - v
				w *= wlen
			}
		}
	}
}

func PowerInto(dst []float64, a []complex128) {
	if len(dst) < len(a) {
		panic("dst too small")
	}
	for i, x := range a {
		dst[i] = real(x)*real(x) + imag(x)*imag(x)
	}
}
