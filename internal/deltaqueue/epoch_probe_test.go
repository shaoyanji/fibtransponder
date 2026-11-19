package deltaqueue

// epoch_probe_test.go — measurement probe for gap #9 (frontier derivation /
// replay from the append-only MemOp log).
//
// This file contains no production code and changes no semantics. It answers
// one question: does Zeckendorf-decomposed, content-addressed segment caching
// beat full sequential replay of an op log — including snapshot amortisation?
// Since the gate passed, it now measures the production implementation in
// replay.go / epoch_cache.go (same kernel it prototyped).
//
// The scheme under test (ported from the kg-zeck epoch cache probe):
//
//   epoch    = content address of the replay boundary (genesis here)
//   segments = the op log carved into greedy non-consecutive Fibonacci blocks
//   blockKey = FNV-1a over (epoch, position, op contents) -> cached block delta
//   quotient = FNV-1a over (epoch, ordered block keys) -> frontier state id
//
// A cached block delta is the block's net effect on the frontier: absolute
// final entry states for items it touches, plus removals and tombstones.
// Because replay is deterministic from genesis and the key binds (epoch,
// position, ops), a delta is only ever applied where it is valid.
//
// Gate (same Kelly discipline as kg-zeck): the cache is only worth building if
//   save = T_full - (T_segmented(p) + T_snapshot/R)  >  0
// at a realistic cache-hit fraction p and reads-per-epoch R.

import (
	"fmt"
	"math/rand"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// Synthetic log: realistic op mix over a hot working set plus a cold tail.
// ---------------------------------------------------------------------------

func synthLog(nOps, hot, cold int, seed int64) []MemOp {
	rng := rand.New(rand.NewSource(seed))
	ops := make([]MemOp, 0, nOps)
	item := func() uint64 {
		if rng.Intn(100) < 80 || cold == 0 {
			return uint64(rng.Intn(hot) + 1)
		}
		return uint64(hot + rng.Intn(cold) + 1)
	}
	var nextID uint64 = uint64(hot + cold + 1)
	for i := 0; i < nOps; i++ {
		tick := uint64(i + 1)
		switch r := rng.Intn(100); {
		case r < 30:
			ops = append(ops, MemOp{Tick: tick, Kind: MemOpPromote, Target: item(), Score: uint32(rng.Intn(90) + 10)})
		case r < 55:
			ops = append(ops, MemOp{Tick: tick, Kind: MemOpDemote, Target: item(), Score: uint32(rng.Intn(90) + 10)})
		case r < 70:
			ops = append(ops, MemOp{Tick: tick, Kind: MemOpAdd, Target: nextID, Score: uint32(rng.Intn(90) + 10)})
			nextID++
		case r < 80:
			ops = append(ops, MemOp{Tick: tick, Kind: MemOpTombstone, Target: item()})
		case r < 90:
			a, b2 := item(), item()
			ops = append(ops, MemOp{Tick: tick, Kind: MemOpMerge, Target: a, ParentA: a, ParentB: b2})
		default:
			ops = append(ops, MemOp{Tick: tick, Kind: MemOpMaterializeEligible, Target: item()})
		}
	}
	return ops
}

func fullReplay(ops []MemOp) *MemFrontier {
	f, _ := ReplayLog(ops)
	return f
}

// ---------------------------------------------------------------------------
// Benchmarks
// ---------------------------------------------------------------------------

func benchFullReplay(b *testing.B, nOps int) {
	ops := synthLog(nOps, 64, 512, 42)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		fullReplay(ops)
	}
}

func benchSegmented(b *testing.B, nOps int, hitPct int) {
	ops := synthLog(nOps, 64, 512, 42)
	blocks := ZeckendorfBlocks(ops)
	const epoch = 0x9E3779B97F4A7C15
	cached := len(blocks) * hitPct / 100

	// Pre-compute the cached-prefix deltas the way a live epoch cache would:
	// the first pass replays and stores; later passes reuse.
	preF := NewMemFrontier()
	preL := NewMemOpLog()
	deltas := make([]BlockDelta, cached)
	for i := 0; i < cached; i++ {
		deltas[i] = ComputeBlockDelta(blocks[i], preF, preL)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		f := NewMemFrontier()
		l := NewMemOpLog()
		for j, blk := range blocks {
			if j < cached {
				deltas[j].ApplyTo(f, l)
			} else {
				ComputeBlockDelta(blk, f, l)
			}
		}
	}
}

// The production API path: one EpochCache living across replays, the way a
// real deployment uses it.
func benchEpochCacheReplay(b *testing.B, nOps int) {
	ops := synthLog(nOps, 64, 512, 42)
	c := NewEpochCache(0x9E3779B97F4A7C15)
	c.Replay(ops) // warm
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.Replay(ops)
	}
}

func benchQuotientHit(b *testing.B, nOps int) {
	ops := synthLog(nOps, 64, 512, 42)
	blocks := ZeckendorfBlocks(ops)
	const epoch = 0x9E3779B97F4A7C15
	keys := make([]uint64, len(blocks))
	for i, blk := range blocks {
		keys[i] = BlockKey(epoch, i, blk)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = QuotientKey(epoch, keys)
	}
}

func benchSnapshot(b *testing.B, nOps int) {
	f := fullReplay(synthLog(nOps, 64, 512, 42))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		SnapshotFrontier(f)
	}
}

func BenchmarkEpochProbe(b *testing.B) {
	for _, n := range []int{1000, 10000} {
		b.Run(fmt.Sprintf("full_replay_n%d", n), func(b *testing.B) { benchFullReplay(b, n) })
		for _, p := range []int{50, 90, 100} {
			b.Run(fmt.Sprintf("segmented_n%d_p%d", n, p), func(b *testing.B) { benchSegmented(b, n, p) })
		}
		b.Run(fmt.Sprintf("epoch_cache_replay_n%d", n), func(b *testing.B) { benchEpochCacheReplay(b, n) })
		b.Run(fmt.Sprintf("quotient_hit_n%d", n), func(b *testing.B) { benchQuotientHit(b, n) })
		b.Run(fmt.Sprintf("snapshot_n%d", n), func(b *testing.B) { benchSnapshot(b, n) })
	}
}

// ---------------------------------------------------------------------------
// Gate: one deterministic pass at fixed scale, reporting the verdict.
// T_full vs T_segmented(p) + T_snapshot/R for a sweep of p and R.
// ---------------------------------------------------------------------------

func timeit(f func(), reps int) time.Duration {
	best := time.Duration(1<<63 - 1)
	for i := 0; i < reps; i++ {
		t0 := time.Now()
		f()
		if d := time.Since(t0); d < best {
			best = d
		}
	}
	return best
}

func TestEpochProbeGate(t *testing.T) {
	const (
		nOps  = 10000
		reps  = 7
		epoch = 0x9E3779B97F4A7C15
	)
	ops := synthLog(nOps, 64, 512, 42)
	blocks := ZeckendorfBlocks(ops)
	keys := make([]uint64, len(blocks))
	for i, blk := range blocks {
		keys[i] = BlockKey(epoch, i, blk)
	}
	terms := make([]int, len(blocks))
	for i, blk := range blocks {
		terms[i] = len(blk)
	}
	t.Logf("log: %d ops -> %d zeckendorf blocks %v", nOps, len(blocks), terms)

	full := timeit(func() { fullReplay(ops) }, reps)
	t.Logf("full replay:        %v  (%.1f ns/op)", full, float64(full)/nOps)

	finalFrontier := fullReplay(ops)
	snapCost := timeit(func() { SnapshotFrontier(finalFrontier) }, reps)
	t.Logf("epoch snapshot:     %v", snapCost)

	// quotient-hit read: hash blocks + map lookup
	qhit := timeit(func() {
		qk := QuotientKey(epoch, keys)
		_ = qk
	}, reps)
	t.Logf("quotient key read:  %v", qhit)

	pass := false
	for _, p := range []int{25, 50, 75, 90, 100} {
		cached := len(blocks) * p / 100
		preF := NewMemFrontier()
		preL := NewMemOpLog()
		deltas := make([]BlockDelta, cached)
		touchedTotal := 0
		for i := 0; i < cached; i++ {
			deltas[i] = ComputeBlockDelta(blocks[i], preF, preL)
			touchedTotal += len(deltas[i].Set) + len(deltas[i].Del)
		}

		seg := timeit(func() {
			f := NewMemFrontier()
			l := NewMemOpLog()
			for j, blk := range blocks {
				if j < cached {
					deltas[j].ApplyTo(f, l)
				} else {
					ComputeBlockDelta(blk, f, l)
				}
			}
		}, reps)

		// Amortised snapshot: one snapshot per epoch, spread over R reads.
		for _, R := range []int{1, 4, 16} {
			amortised := snapCost / time.Duration(R)
			save := full - (seg + amortised)
			verdict := "FAIL"
			if save > 0 {
				verdict = "PASS"
				pass = true
			}
			t.Logf("p=%3d%% R=%2d  segmented %v (+snap/R %v)  save %v  %s",
				p, R, seg, amortised, save, verdict)
		}
		t.Logf("  (cached prefix touched %d items across %d blocks)", touchedTotal, cached)
	}

	if !pass {
		t.Logf("GATE VERDICT: no (p,R) clears save > 0 — do not build the epoch cache")
	} else {
		t.Logf("GATE VERDICT: save > 0 holds for at least one (p,R) — cache earns its keep")
	}
}
