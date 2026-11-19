package deltaqueue

// Epoch replay cache: Zeckendorf-segmented, content-addressed replay of the
// MemOp log. Ported from the measurement probe in epoch_probe_test.go after
// its save>0 gate passed (see HANDOFF.md "Epoch-Replay Probe").
//
//   epoch    = content address of the replay boundary (caller-supplied; e.g.
//              a Zobrist Sketch at a marker cut, or a constant for genesis)
//   segments = the op log carved into greedy non-consecutive Fibonacci blocks
//   blockKey = word-mixed hash over (epoch, position, op contents)
//   quotient = word-mixed hash over (epoch, ordered block keys)
//
// A BlockDelta is one block's net effect: absolute final entry state for each
// touched item, removals, and the touched items' tombstone membership. Because
// the key binds (epoch, position, ops), a stored delta is only ever applied
// where it is valid; absolute states make application order-safe for the
// items it covers.
//
// Addressing note (measured): hashing must be word-wise. A byte-at-a-time
// FNV walk over the log costs ~750µs per call at 10k ops — slower than the
// replay it replaces (~364µs). The splitmix-style mixer below costs roughly
// one mix per op (~50µs at 10k ops), keeping the warm path well under full
// replay. A shortcut that addresses blocks only by (position, length,
// endpoint ticks) was considered and rejected: it is not content-complete —
// interior ops can differ under identical endpoints, and multiple MemOps can
// share one Tick.
//
// Not concurrency-safe — matches the rest of the package's ownership model.
// Replay is not on the classifier hot path; the allocation-free invariant
// does not apply here.

// ---------------------------------------------------------------------------
// Zeckendorf decomposition (greedy, non-consecutive, largest term first —
// same orientation as kg-zeck's segments()).
// ---------------------------------------------------------------------------

var fibTerms = []int{1, 2, 3, 5, 8, 13, 21, 34, 55, 89, 144, 233, 377, 610, 987,
	1597, 2584, 4181, 6765, 10946, 17711, 28657, 46368, 75025, 121393,
	196418, 317811, 514229, 832040, 1346269, 2178309, 3524578, 5702887}

// ZeckendorfTerms returns n's greedy non-consecutive Fibonacci decomposition,
// largest term first. n must be <= the representable maximum (~9.4M).
func ZeckendorfTerms(n int) []int {
	var out []int
	for i := len(fibTerms) - 1; i >= 0 && n > 0; i-- {
		if fibTerms[i] <= n {
			out = append(out, fibTerms[i])
			n -= fibTerms[i]
		}
	}
	return out
}

// ZeckendorfBlocks carves ops into contiguous blocks of Zeckendorf size,
// largest term first. The blocks partition ops: concatenating them in order
// reproduces ops exactly.
func ZeckendorfBlocks(ops []MemOp) [][]MemOp {
	terms := ZeckendorfTerms(len(ops))
	blocks := make([][]MemOp, 0, len(terms))
	rest := ops
	for _, sz := range terms {
		blocks = append(blocks, rest[:sz])
		rest = rest[sz:]
	}
	return blocks
}

// ---------------------------------------------------------------------------
// Content addressing.
// ---------------------------------------------------------------------------

// mix64 is the splitmix64 finalizer: cheap, well-avalanched, and used for all
// content addressing here.
func mix64(z uint64) uint64 {
	z = (z ^ (z >> 30)) * 0xBF58476D1CE4E5B9
	z = (z ^ (z >> 27)) * 0x94D049BB133111EB
	return z ^ (z >> 31)
}

func rotl64(x uint64, k uint) uint64 { return (x << k) | (x >> (64 - k)) }

// opWord packs one MemOp's content into a single word for mixing.
func opWord(op MemOp) uint64 {
	return op.Tick ^ rotl64(op.Target, 11) ^ rotl64(op.ParentA, 29) ^
		rotl64(op.ParentB, 47) ^ (uint64(op.Kind) << 56) ^ uint64(op.Score)
}

// BlockKey is the content address of one replay segment: a word-mixed hash
// over (epoch, position, op contents). O(len(ops)) and order-sensitive —
// the fold must chain: ops sharing one Tick can legitimately appear in
// either order and produce different deltas, so a commutative fold would
// give them the same address.
func BlockKey(epoch uint64, pos int, ops []MemOp) uint64 {
	h := mix64(epoch) ^ rotl64(uint64(pos), 23)
	for _, op := range ops {
		h = mix64(h ^ opWord(op))
	}
	return mix64(h ^ uint64(len(ops)))
}

// QuotientKey is the content address of the whole frontier state after
// replaying a block-keyed log: a word-mixed hash over (epoch, ordered block
// keys). Identical op-window multiset → identical quotient → a stored
// snapshot is a hit.
func QuotientKey(epoch uint64, blockKeys []uint64) uint64 {
	h := mix64(epoch)
	for _, k := range blockKeys {
		h = mix64(h ^ k)
	}
	return mix64(h ^ uint64(len(blockKeys)))
}

// ---------------------------------------------------------------------------
// Block deltas.
// ---------------------------------------------------------------------------

// BlockDelta is a block's net effect on replay state. Applying it produces
// the same frontier/log state as replaying the block's ops at the position
// its key binds.
type BlockDelta struct {
	Set  map[uint64]FrontierEntry // absolute final state per touched item
	Del  []uint64                 // touched items absent from the frontier at block end
	Tomb []uint64                 // touched items holding a live tombstone at block end
}

// ApplyTo applies the delta's net effect to frontier and log state.
func (d BlockDelta) ApplyTo(f *MemFrontier, l *MemOpLog) {
	for _, id := range d.Del {
		delete(f.entries, id)
	}
	for id, e := range d.Set {
		f.entries[id] = e
	}
	for _, id := range d.Tomb {
		l.tombstones[id] = struct{}{}
	}
}

// ComputeBlockDelta replays a block's ops for real (mutating f and l) and
// returns the net delta for caching. Tombstone membership is recorded for
// every touched item, not only absent ones: an item tombstoned mid-block and
// re-added still holds a live tombstone at block end, and missing that would
// diverge from sequential replay.
func ComputeBlockDelta(ops []MemOp, f *MemFrontier, l *MemOpLog) BlockDelta {
	touched := make(map[uint64]struct{})
	for _, op := range ops {
		replayMemOp(op, f, l, touched)
	}
	d := BlockDelta{Set: make(map[uint64]FrontierEntry, len(touched))}
	for id := range touched {
		if e, ok := f.entries[id]; ok {
			d.Set[id] = e
		} else {
			d.Del = append(d.Del, id)
		}
		if _, ok := l.tombstones[id]; ok {
			d.Tomb = append(d.Tomb, id)
		}
	}
	return d
}

// ---------------------------------------------------------------------------
// The cache.
// ---------------------------------------------------------------------------

type epochSnapshot struct {
	entries map[uint64]FrontierEntry
	tombs   map[uint64]struct{}
}

// EpochCache is a content-addressed store of block deltas and whole-state
// snapshots for one epoch boundary.
type EpochCache struct {
	epoch    uint64
	deltas   map[uint64]BlockDelta
	snaps    map[uint64]epochSnapshot
	hits     int // block-delta cache hits
	misses   int // block-delta cache misses
	snapHits int // quotient-addressed snapshot lookups that hit
}

// NewEpochCache creates an empty cache bound to an epoch address.
func NewEpochCache(epoch uint64) *EpochCache {
	return &EpochCache{
		epoch:  epoch,
		deltas: make(map[uint64]BlockDelta),
		snaps:  make(map[uint64]epochSnapshot),
	}
}

// Replay derives frontier and log state from genesis using Zeckendorf
// segmentation: blocks whose key has a cached delta are applied directly;
// misses are replayed and their delta stored. The result is identical to
// ReplayLog(ops).
func (c *EpochCache) Replay(ops []MemOp) (*MemFrontier, *MemOpLog) {
	blocks := ZeckendorfBlocks(ops)
	f := NewMemFrontier()
	l := NewMemOpLog()
	for i, blk := range blocks {
		key := BlockKey(c.epoch, i, blk)
		if d, ok := c.deltas[key]; ok {
			c.hits++
			d.ApplyTo(f, l)
		} else {
			c.misses++
			c.deltas[key] = ComputeBlockDelta(blk, f, l)
		}
	}
	return f, l
}

// Quotient returns the content address of the frontier state that replaying
// ops would produce, without replaying. O(ops) hashing, no mutation.
func (c *EpochCache) Quotient(ops []MemOp) uint64 {
	blocks := ZeckendorfBlocks(ops)
	keys := make([]uint64, len(blocks))
	for i, blk := range blocks {
		keys[i] = BlockKey(c.epoch, i, blk)
	}
	return QuotientKey(c.epoch, keys)
}

// Snapshot returns a stored whole-state snapshot — frontier entries and live
// tombstone set — for the op sequence's quotient, or nil,nil,false on a miss.
// Both maps are clones; mutating them does not corrupt the cache.
func (c *EpochCache) Snapshot(ops []MemOp) (map[uint64]FrontierEntry, map[uint64]struct{}, bool) {
	snap, ok := c.snaps[c.Quotient(ops)]
	if !ok {
		return nil, nil, false
	}
	c.snapHits++
	entries := make(map[uint64]FrontierEntry, len(snap.entries))
	for id, e := range snap.entries {
		entries[id] = e
	}
	tombs := make(map[uint64]struct{}, len(snap.tombs))
	for id := range snap.tombs {
		tombs[id] = struct{}{}
	}
	return entries, tombs, true
}

// StoreSnapshot records a whole-state snapshot under the op sequence's
// quotient. Call it at the epoch boundary (marker cut) so subsequent reads
// of the same window resolve without replay.
func (c *EpochCache) StoreSnapshot(ops []MemOp, f *MemFrontier, l *MemOpLog) {
	tombs := make(map[uint64]struct{}, len(l.tombstones))
	for id := range l.tombstones {
		tombs[id] = struct{}{}
	}
	c.snaps[c.Quotient(ops)] = epochSnapshot{
		entries: SnapshotFrontier(f),
		tombs:   tombs,
	}
}

// Stats reports block-delta hits/misses and quotient snapshot hits since
// the cache was created.
func (c *EpochCache) Stats() (hits, misses, snapHits int) {
	return c.hits, c.misses, c.snapHits
}
