package deltaqueue

const (
	// StepsSince lane indexes as frozen in the constants appendix.
	LaneStateChange      = 0
	LaneDilation         = 1
	LaneSegmentCandidate = 2
	LaneMarkerCandidate  = 3
)

const (
	// StepsSince saturates at 0xFFFF and never wraps.
	StepsSinceSaturation uint16 = 0xFFFF
)

// CoreDelta is emitted by fsvm after each Step().
// All fields reflect the state of the machine after the step completes.
// CoreFlags contains only step-truths. See §I.5 for flag definitions.
type CoreDelta struct {
	Tick      uint64 // monotonic step counter, never reset
	StateID   uint32 // FSVM state identifier after this step
	SegEpoch  uint32 // coarse segment epoch; incremented by fsvm on major boundaries
	CoreFlags uint32 // see CoreFlag constants, §I.5.1
}

// DerivedDelta is produced by the classifier immediately after each Step().
// It embeds CoreDelta by value. No heap allocation is permitted.
type DerivedDelta struct {
	Core         CoreDelta
	DerivedFlags uint32 // see DerivedFlag constants, §I.5.2
	Aux          uint32 // packed score buckets; see §I.5.3
	// TODO(spec): delta-queue spec omits this field, but frozen constants appendix
	// conformance text references DerivedDelta.StepsSince[0].
	StepsSince [4]uint16
}

// ClassifierState is the classifier's local residue between steps.
// It is owned by the classifier and must not be read by any other layer.
type ClassifierState struct {
	HasPrev     bool
	SuppressFor uint8
	PrevFlags   uint32
	PrevStateID uint32
	StepsSince  [4]uint16 // lanes: [StateChange, Dilation, SegmentCandidate, MarkerCandidate]
	Sketch      uint64
}

// AuxBuckets is packed into DerivedDelta.Aux (uint32).
// Buckets are ordinal hints.
type AuxBuckets struct {
	Recency   uint8 // log-scale; higher = more recent
	Novelty   uint8
	Stability uint8
	Urgency   uint8
}

// Subscription defines which DerivedDeltas wake an extension.
// Any: wake if (flags & Any) != 0
// All: wake only if (flags & All) == All
// (0 means no All requirement).
type Subscription struct {
	CoreAny    uint32
	CoreAll    uint32
	DerivedAny uint32
	DerivedAll uint32
}

// MemOpKind is the operation kind in the append-only queue log.
type MemOpKind uint8

const (
	MemOpAdd MemOpKind = iota
	MemOpPromote
	MemOpDemote
	MemOpTombstone
	MemOpMerge
	MemOpMaterializeEligible
)

// MemOp is an append-only record in the operation log.
// It is never mutated after append.
type MemOp struct {
	Tick    uint64
	Kind    MemOpKind
	Target  uint64
	ParentA uint64
	ParentB uint64
	Score   uint32 // snapshot at append time only
}

// Score is the queue frontier score vector.
type Score struct {
	Recency   uint32
	Stability uint32
	Reuse     uint32
	Novelty   uint32
}

// FrontierIndex is intentionally left as TODO until interface details are specified.
// TODO(spec): define exact frontier index contract.
type FrontierIndex interface{}

// OpLog is intentionally left as TODO until interface details are specified.
// TODO(spec): define exact operation log contract.
type OpLog interface{}

// PromotionQuota is intentionally left as TODO until interface details are specified.
// TODO(spec): define quota shape and accounting semantics.
type PromotionQuota interface{}

// QueueState is intentionally left as TODO until state layout is specified.
// TODO(spec): define queue state fields and index ownership.
type QueueState struct{}
