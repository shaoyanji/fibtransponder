package window

import "math"

// Package window provides small helpers for choosing and extracting windows
// from a bit rope / bit slice.

// Kind identifies a window function.
type Kind uint8

const (
	Rectangular Kind = iota
	Hann
	Hamming
	Blackman
)

// Apply applies a window function in-place to a float64 slice.
func Apply(s []float64, k Kind) {
	switch k {
	case Hann:
		hann(s)
	case Hamming:
		hamming(s)
	case Blackman:
		blackman(s)
	default:
		// rectangular: no-op
	}
}

func hann(s []float64) {
	n := float64(len(s))
	for i := range s {
		w := 0.5 * (1 - math.Cos(2*math.Pi*float64(i)/(n-1)))
		s[i] *= w
	}
}

func hamming(s []float64) {
	n := float64(len(s))
	for i := range s {
		w := 0.54 - 0.46*math.Cos(2*math.Pi*float64(i)/(n-1))
		s[i] *= w
	}
}

func blackman(s []float64) {
	n := float64(len(s))
	for i := range s {
		w := 0.42 - 0.5*math.Cos(2*math.Pi*float64(i)/(n-1)) + 0.08*math.Cos(4*math.Pi*float64(i)/(n-1))
		s[i] *= w
	}
}

func LargestPow2LE(n int) int {
	if n < 2 {
		return 0
	}
	p := 1
	for p<<1 <= n {
		p <<= 1
	}
	return p
}
