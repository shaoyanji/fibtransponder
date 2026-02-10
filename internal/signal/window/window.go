package window

// Package window provides small helpers for choosing and extracting windows
// from a bit rope / bit slice.

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
