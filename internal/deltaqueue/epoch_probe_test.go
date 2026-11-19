package deltaqueue

// epoch_probe_test.go — measurement probe for gap #9 (frontier derivation /
// replay from the append-only MemOp log).
//
// This file contains no production code and changes no semantics. It answers
// one question: does Zeckendorf-decomposed, content-addressed segment caching
// beat full sequential replay of an op log — including snapshot amortisation?
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
	"encoding/binary"
	"fmt"
	"hash/fnv"
	"math/rand"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// Probe-scoped replay semantics.
//
// Not the queue spec: a minimal deterministic kernel whose per-op cost is
// representative (map read/write + tombstone bookkeeping). Add is an upsert;
// Promote/Demote adjust Recency by the op's snapshot score; Merge folds the
// secondary parent into the survivor.
// ---------------------------------------------------------------------------

func replayOp(op MemOp, f *MemFrontier, l *MemOpLog, touched map[uint64]struct{}) {
	switch op.Kind {
	case MemOpAdd:
		if touched != nil {
			touched[op.Target] = struct{}{}
		}
		f.entries[op.Target] = FrontierEntry{Score: Score{Recency: op.Score}}
	case MemOpPromote:
		if e, ok := f.entries[op.Target]; ok {
			if touched != nil {
				touched[op.Target] = struct{}{}
			}
			e.Score.Recency += op.Score
			f.entries[op.Target] = e
		}
	case MemOpDemote:
		if e, ok := f.entries[op.Target]; ok {
			if touched != nil {
				touched[op.Target] = struct{}{}
			}
			e.Score.Recency -= min64u32(e.Score.Recency, op.Score)
			f.entries[op.Target] = e
		}
	case MemOpTombstone:
		if _, ok := f.entries[op.Target]; ok {
			if touched != nil {
				touched[op.Target] = struct{}{}
			}
			delete(f.entries, op.Target)
			l.tombstones[op.Target] = struct{}{}
		}
	case MemOpMerge:
		a, oka := f.entries[op.ParentA]
		if bv, okb := f.entries[op.ParentB]; oka && okb {
			if touched != nil {
				touched[op.ParentA] = struct{}{}
				touched[op.ParentB] = struct{}{}
			}
			a.Score.Recency += bv.Score.Recency
			f.entries[op.ParentA] = a
			delete(f.entries, op.ParentB)
		}
	case MemOpMaterializeEligible:
		if e, ok := f.entries[op.Target]; ok {
			if touched != nil {
				touched[op.Target] = struct{}{}
			}
			e.Materialized = true
			f.entries[op.Target] = e
		}
	}
}

func min64u32(a, b uint32) uint32 {
	if a < b {
		return a
	}
	return b
}

// ---------------------------------------------------------------------------
// Zeckendorf terms: greedy non-consecutive Fibonacci decomposition.
// Sequence 1,2,3,5,... (single 1, as in kg-zeck's terms()).
// ---------------------------------------------------------------------------

var fibSeq = []int{1, 2, 3, 5, 8, 13, 21, 34, 55, 89, 144, 233, 377, 610, 987,
	1597, 2584, 4181, 6765, 10946, 17711, 28657, 46368, 75025, 121393,
	196418, 317811, 514229, 832040, 1346269, 2178309, 3524578, 5702887}

func zeckTerms(n int) []int {
	var out []int
	for i := len(fibSeq) - 1; i >= 0 && n > 0; i-- {
		if fibSeq[i] <= n {
			out = append(out, fibSeq[i])
			n -= fibSeq[i]
		}
	}
	return out
}

// Carve ops into contiguous blocks of Zeckendorf size, largest term first
// (same orientation as kg-zeck segments()).
func zeckBlocks(ops []MemOp) [][]MemOp {
	terms := zeckTerms(len(ops))
	blocks := make([][]MemOp, 0, len(terms))
	rest := ops
	for _, sz := range terms {
		blocks = append(blocks, rest[:sz])
		rest = rest[sz:]
	}
	return blocks
}

// ---------------------------------------------------------------------------
// Content addressing (FNV-1a; the probe's stand-in for the Zobrist sketch on
// the epoch boundary — the real scheme would carry fsvm Sketch at the cut).
// ---------------------------------------------------------------------------

func hashWrite(h interface{ Write([]byte) (int, error) }, v uint64) {
	var buf [8]byte
	binary.LittleEndian.PutUint64(buf[:], v)
	h.Write(buf[:])
}

func blockKey(epoch uint64, pos int, ops []MemOp) uint64 {
	h := fnv.New64a()
	hashWrite(h, epoch)
	hashWrite(h, uint64(pos))
	for _, op := range ops {
		hashWrite(h, op.Tick)
		hashWrite(h, uint64(op.Kind))
		hashWrite(h, op.Target)
		hashWrite(h, op.ParentA)
		hashWrite(h, op.ParentB)
		hashWrite(h, uint64(op.Score))
	}
	return h.Sum64()
}

func quotientKey(epoch uint64, keys []uint64) uint64 {
	h := fnv.New64a()
	hashWrite(h, epoch)
	for _, k := range keys {
		hashWrite(h, k)
	}
	return h.Sum64()
}

// blockDelta is the cached net effect of one block on the frontier.
type blockDelta struct {
	set   map[uint64]FrontierEntry // absolute final state per touched item
	del   []uint64                 // removals (tombstone/merge secondary)
	tomb  []uint64                 // tombstone additions
	apply int                      // entries written on apply (cost report)
}

// replayBlock replays ops, records the net delta, and returns it. Frontier
// mutations are applied for real; the delta records what changed.
func replayBlock(ops []MemOp, f *MemFrontier, l *MemOpLog) blockDelta {
	touched := make(map[uint64]struct{})
	for _, op := range ops {
		replayOp(op, f, l, touched)
	}
	d := blockDelta{set: make(map[uint64]FrontierEntry, len(touched))}
	for id := range touched {
		if e, ok := f.entries[id]; ok {
			d.set[id] = e
		} else {
			d.del = append(d.del, id)
		}
	}
	return d
}

func applyDelta(d blockDelta, f *MemFrontier, l *MemOpLog) {
	for _, id := range d.del {
		delete(f.entries, id)
	}
	for id, e := range d.set {
		f.entries[id] = e
	}
	for _, id := range d.tomb {
		l.tombstones[id] = struct{}{}
	}
}

func snapshotFrontier(f *MemFrontier) map[uint64]FrontierEntry {
	snap := make(map[uint64]FrontierEntry, len(f.entries))
	for id, e := range f.entries {
		snap[id] = e
	}
	return snap
}

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
	f := NewMemFrontier()
	l := NewMemOpLog()
	for _, op := range ops {
		replayOp(op, f, l, nil)
	}
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
	blocks := zeckBlocks(ops)
	const epoch = 0x9E3779B97F4A7C15
	keys := make([]uint64, len(blocks))
	for i, blk := range blocks {
		keys[i] = blockKey(epoch, i, blk)
	}
	cached := len(blocks) * hitPct / 100

	// Pre-compute the cached-prefix deltas the way a live epoch cache would:
	// the first pass replays and stores; later passes reuse.
	preF := NewMemFrontier()
	preL := NewMemOpLog()
	deltas := make([]blockDelta, cached)
	for i := 0; i < cached; i++ {
		deltas[i] = replayBlock(blocks[i], preF, preL)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		f := NewMemFrontier()
		l := NewMemOpLog()
		for j, blk := range blocks {
			if j < cached {
				applyDelta(deltas[j], f, l)
			} else {
				replayBlock(blk, f, l)
			}
		}
	}
}

func benchQuotientHit(b *testing.B, nOps int) {
	ops := synthLog(nOps, 64, 512, 42)
	blocks := zeckBlocks(ops)
	const epoch = 0x9E3779B97F4A7C15
	keys := make([]uint64, len(blocks))
	for i, blk := range blocks {
		keys[i] = blockKey(epoch, i, blk)
	}
	final := fullReplay(ops)
	snap := snapshotFrontier(final)
	cache := map[uint64]map[uint64]FrontierEntry{quotientKey(epoch, keys): snap}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = cache[quotientKey(epoch, keys)]
	}
}

func benchSnapshot(b *testing.B, nOps int) {
	f := fullReplay(synthLog(nOps, 64, 512, 42))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		snapshotFrontier(f)
	}
}

func BenchmarkEpochProbe(b *testing.B) {
	for _, n := range []int{1000, 10000} {
		b.Run(fmt.Sprintf("full_replay_n%d", n), func(b *testing.B) { benchFullReplay(b, n) })
		for _, p := range []int{50, 90, 100} {
			b.Run(fmt.Sprintf("segmented_n%d_p%d", n, p), func(b *testing.B) { benchSegmented(b, n, p) })
		}
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
	blocks := zeckBlocks(ops)
	keys := make([]uint64, len(blocks))
	for i, blk := range blocks {
		keys[i] = blockKey(epoch, i, blk)
	}
	terms := make([]int, len(blocks))
	for i, blk := range blocks {
		terms[i] = len(blk)
	}
	t.Logf("log: %d ops -> %d zeckendorf blocks %v", nOps, len(blocks), terms)

	full := timeit(func() { fullReplay(ops) }, reps)
	t.Logf("full replay:        %v  (%.1f ns/op)", full, float64(full)/nOps)

	finalFrontier := fullReplay(ops)
	snapCost := timeit(func() { snapshotFrontier(finalFrontier) }, reps)
	t.Logf("epoch snapshot:     %v", snapCost)

	// quotient-hit read: hash blocks + map lookup
	qhit := timeit(func() {
		qk := quotientKey(epoch, keys)
		_ = qk
	}, reps)
	t.Logf("quotient key read:  %v", qhit)

	pass := false
	for _, p := range []int{25, 50, 75, 90, 100} {
		cached := len(blocks) * p / 100
		preF := NewMemFrontier()
		preL := NewMemOpLog()
		deltas := make([]blockDelta, cached)
		touchedTotal := 0
		for i := 0; i < cached; i++ {
			deltas[i] = replayBlock(blocks[i], preF, preL)
			touchedTotal += len(deltas[i].set) + len(deltas[i].del)
		}

		seg := timeit(func() {
			f := NewMemFrontier()
			l := NewMemOpLog()
			for j, blk := range blocks {
				if j < cached {
					applyDelta(deltas[j], f, l)
				} else {
					replayBlock(blk, f, l)
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
