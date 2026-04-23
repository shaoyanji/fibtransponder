// Package deltaqueue contains compile-safe scaffolding for queue-facing runtime
// deltas and their bounded heap ABI seam.
//
// The heap ABI in this package is runtime machinery for surfaced objects only.
// It is not semantic identity, rewrite-equivalence, scheduler policy, or heap
// implementation behavior.
package deltaqueue
