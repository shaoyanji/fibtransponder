package deltaqueue

// Heap ABI scaffolding for packet-based runtime coordination.
//
// This file intentionally exposes only a bounded operation contract for heap
// interaction. It does not expose structural heap internals, semantic identity
// truth, scheduler behavior, or Fibonacci-specific rewrite logic.
//
// TODO(spec): confirm whether this ABI remains package-local to deltaqueue or
// moves behind a narrower runtime-facing package once a concrete heap runtime
// exists.

// HeapInvariant names a runtime property that cross-agent packets may preserve
// or verify. These are packet-level labels only; they do not imply behavior.
type HeapInvariant string

const (
	HeapInvariantRuntimeOwnsStructure HeapInvariant = "runtime_owns_structure"
	HeapInvariantHandleStability      HeapInvariant = "handle_stability"
	HeapInvariantSurfaceOnly          HeapInvariant = "surface_only"
	HeapInvariantExplicitLaziness     HeapInvariant = "explicit_laziness"
	HeapInvariantNoRawPointerTalk     HeapInvariant = "no_raw_pointer_talk"
	HeapInvariantHeapOrder            HeapInvariant = "heap_order"
	HeapInvariantHandleValidity       HeapInvariant = "handle_validity"
)

// HeapOpKind is the bounded heap operation kind exposed to packets.
// Agents may propose these operations, but only the runtime may perform the
// underlying structural mutation.
type HeapOpKind string

const (
	HeapOpPeekMin      HeapOpKind = "PEEK_MIN"
	HeapOpHandleExists HeapOpKind = "HANDLE_EXISTS"
	HeapOpGetKey       HeapOpKind = "GET_KEY"
	HeapOpGetSummary   HeapOpKind = "GET_SUMMARY"

	HeapOpInsert      HeapOpKind = "INSERT"
	HeapOpDecreaseKey HeapOpKind = "DECREASE_KEY"
	HeapOpExtractMin  HeapOpKind = "EXTRACT_MIN"
	HeapOpMeld        HeapOpKind = "MELD"
	HeapOpDeleteHandle HeapOpKind = "DELETE_HANDLE"
)

// HeapHandle is a stable runtime reference to a surfaced object.
// semantic_ref links outward to semantic identity layers but is not heap truth.
// generation may be used by the runtime to reject stale operations.
type HeapHandle struct {
	HandleID    string
	HeapID      string
	SemanticRef string
	Key         int64
	Generation  uint32
}

// HeapSummary is the bounded read model exposed for inspection.
// dirty makes deferred/lazy runtime work explicit without exposing internals.
type HeapSummary struct {
	HeapID           string
	MinHandle        string
	NodeCount        uint32
	RootCount        uint32
	Dirty            bool
	TopK             []string
	LastOp           string
	LastMutationTick uint64
}

// HeapProof captures the packet-level pre/effect/post proof shape for a heap op.
type HeapProof struct {
	Pre    []string
	Effect []string
	Post   []string
}

// HeapPacket is the structured packet form for bounded heap coordination.
// This is runtime-only scaffolding for surfaced objects; it must not become the
// semantic source of truth for identity, rewrite equivalence, or tombstoning.
type HeapPacket struct {
	HeapID      string
	Op          HeapOpKind
	HandleID    string
	SemanticRef string
	OldKey      *int64
	NewKey      *int64
	Preserve    []HeapInvariant
	ProofTarget []string
	Proof       HeapProof
}

// HeapRuntime defines the narrow runtime seam acknowledged by the packet ABI.
// Implementations own structural mutation and may apply cuts, cascades,
// consolidation, or other internal mechanics behind these bounded calls.
//
// TODO(spec): decide whether MELD and DELETE_HANDLE return richer receipts once
// concrete queue integration exists.
type HeapRuntime interface {
	PeekMin(heapID string) (HeapHandle, bool)
	HandleExists(heapID, handleID string) bool
	GetKey(heapID, handleID string) (int64, bool)
	GetSummary(heapID string) HeapSummary
	ApplyHeapPacket(pkt HeapPacket) error
}
