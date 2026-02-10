package rosetta

// Package rosetta will contain marker checkpoint logic bridging:
// - fib radix structure
// - log2/Binet magnitude bounds
// - modular fingerprints
//
// This is deliberately left as TODO because it depends on deciding the exact
// semantic meaning of N(r) under retrospective dilation.

type Marker struct {
	Cursor uint64
	R      uint32
	// TODO: payloads: k_max, residue snapshots, window state, etc.
}
