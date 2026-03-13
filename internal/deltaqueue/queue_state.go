package deltaqueue

// FrontierEntry is the concrete state tracked per frontier item.
type FrontierEntry struct {
	Score        Score
	Materialized bool
}

// MemFrontier is a map-backed FrontierIndex implementation.
// Suitable for initial use and testing; a production heap-backed
// frontier would replace this behind the same interface.
type MemFrontier struct {
	entries map[uint64]FrontierEntry
}

// NewMemFrontier creates an empty in-memory frontier.
func NewMemFrontier() *MemFrontier {
	return &MemFrontier{entries: make(map[uint64]FrontierEntry)}
}

func (f *MemFrontier) Has(id uint64) bool {
	_, ok := f.entries[id]
	return ok
}

func (f *MemFrontier) Score(id uint64) (Score, bool) {
	e, ok := f.entries[id]
	return e.Score, ok
}

func (f *MemFrontier) Materialized(id uint64) bool {
	e, ok := f.entries[id]
	return ok && e.Materialized
}

// Insert adds an item to the frontier. Panics if already present.
func (f *MemFrontier) Insert(id uint64, score Score) {
	if _, exists := f.entries[id]; exists {
		panic("MemFrontier.Insert: duplicate id")
	}
	f.entries[id] = FrontierEntry{Score: score}
}

// Remove removes an item from the frontier.
func (f *MemFrontier) Remove(id uint64) {
	delete(f.entries, id)
}

// SetMaterialized marks an item as materialized.
func (f *MemFrontier) SetMaterialized(id uint64) {
	if e, ok := f.entries[id]; ok {
		e.Materialized = true
		f.entries[id] = e
	}
}

// MemOpLog is a map-backed OpLog implementation.
type MemOpLog struct {
	tombstones map[uint64]struct{}
}

// NewMemOpLog creates an empty in-memory op log.
func NewMemOpLog() *MemOpLog {
	return &MemOpLog{tombstones: make(map[uint64]struct{})}
}

func (l *MemOpLog) HasLiveTombstone(id uint64) bool {
	_, ok := l.tombstones[id]
	return ok
}

// AddTombstone records a live tombstone for the given item.
func (l *MemOpLog) AddTombstone(id uint64) {
	l.tombstones[id] = struct{}{}
}

// RemoveTombstone clears a tombstone (e.g. on merge completion).
func (l *MemOpLog) RemoveTombstone(id uint64) {
	delete(l.tombstones, id)
}

// FixedQuota is a simple counter-based PromotionQuota.
// AllowPromotion returns true until promotions reach Max.
type FixedQuota struct {
	Used uint32
	Max  uint32
}

func (q *FixedQuota) AllowPromotion() bool {
	return q.Used < q.Max
}

// RecordPromotion increments the used counter.
func (q *FixedQuota) RecordPromotion() {
	if q.Used < q.Max {
		q.Used++
	}
}

// NewQueueState constructs a QueueState from its dependencies.
func NewQueueState(frontier FrontierIndex, log OpLog, quota PromotionQuota) QueueState {
	return QueueState{
		Frontier: frontier,
		Log:      log,
		Quota:    quota,
	}
}
