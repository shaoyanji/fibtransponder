package wasmapi

import (
	"encoding/json"
	"math/rand"
	"sync"

	"github.com/shaoyanji/fibtransponder/internal/fsvm"
)

// sketchAPI serves the sketch-entropy A/B demo, which is the single most
// useful artifact on the page: it shows a bug that was found, understood,
// fixed, and regression-tested, side by side with the version that caused it.
//
// The two folds differ only in whether the 6-bit window is spread across the
// word:
//
//	naive: seeds[b] + W                       <= 4*256 = 1024 distinct values
//	fixed: seeds[b] + W*0x9E3779B97F4A7C15    <= 2^64 in practice
//
// The naive form is the retracted one. It lives here, in the shim, and nowhere
// in internal/ -- the repo's own fold is the fixed form.
type sketchAPI struct{}

type sketchEntropyArgs struct {
	Variant string `json:"variant"` // "fixed" | "naive"
	// Streams is the size of the whole sample, not the size of this call.
	Streams int   `json:"streams"`
	Seed    int64 `json:"seed"`
	MinLen  int   `json:"minLen"`
	MaxLen  int   `json:"maxLen"`
	// Offset is how many streams previous calls already consumed, so repeated
	// calls cover one deterministic sample rather than drawing fresh randomness
	// each time.
	Offset int `json:"offset"`
	// Count is how many streams this call processes. 0 means "all remaining",
	// which is fine for a small sample but blocks the browser's event loop for
	// a large one -- Go/WASM runs synchronously on it. The client should pass
	// something like 2000, yield, and call again with Offset advanced, so the
	// heavy loop stays in Go while the tab stays responsive.
	Count int `json:"count"`
	// Session makes the distinct count cumulative across calls.
	//
	// Without it, `distinct` is per-chunk, and a client that sweeps in chunks
	// and adds the results up over-counts: the naive fold saturates at 256
	// per chunk, so three chunks report 768 and the arithmetic is nonsense.
	// Carrying the seen-set across calls is the only way the number means what
	// it says. Pass the same session id for every chunk of one sweep, and a
	// different one to start over.
	Session string `json:"session"`
	// ResetSession discards any accumulated state for Session before running.
	Reset bool `json:"reset"`
}

type sketchEntropyOut struct {
	Variant string `json:"variant"`
	// Distinct is cumulative across the session when one is supplied, and
	// per-chunk otherwise. The client must not add per-chunk distinct counts
	// together; with a session it does not have to.
	Distinct   int `json:"distinct"`
	Total      int `json:"total"`
	Collisions int `json:"collisions"`
	// ChunkDistinct and ChunkTotal describe just this call, for progress bars.
	ChunkDistinct int `json:"chunkDistinct"`
	ChunkTotal    int `json:"chunkTotal"`
	// FirstCollisionInChunk is the 1-based absolute index of the first stream
	// in this chunk whose sketch had already been seen, either in this chunk or
	// earlier in the session. 0 means none.
	FirstCollisionInChunk int `json:"firstCollisionInChunk"`
	// Cap is the theoretical maximum distinct value count for this fold: 256
	// for the naive one. It makes the collapse legible even when the measured
	// number has not saturated yet.
	Cap          int     `json:"cap"`
	MeanPopcount float64 `json:"meanPopcount"`
	Seed         int64   `json:"seed"`
	MinLen       int     `json:"minLen"`
	MaxLen       int     `json:"maxLen"`
	Done         bool    `json:"done"`
	Offset       int     `json:"offset"`
	Session      string  `json:"session,omitempty"`
}

// sketchSession is the accumulated state for one chunked sweep.
type sketchSession struct {
	seen     map[uint64]struct{}
	distinct int
	total    int
	popSum   int64
	firstHit int
}

var (
	sessionsMu sync.Mutex
	sessions   = map[string]*sketchSession{}
)

// sessionFor returns the accumulator for a session id, or a fresh one when the
// id is empty.
//
// Sessions are bounded by MaxSketchRun distinct values, and each entry is 8
// bytes plus map overhead, so a full sweep costs on the order of a megabyte. The
// map is package-level because the wasm module has no other place to keep it,
// which is also why a long-lived page should reuse session ids rather than
// minting one per sweep.
func sessionFor(id string, reset bool) *sketchSession {
	if id == "" {
		return &sketchSession{seen: map[uint64]struct{}{}}
	}
	sessionsMu.Lock()
	defer sessionsMu.Unlock()
	if reset {
		delete(sessions, id)
	}
	s, ok := sessions[id]
	if !ok {
		s = &sketchSession{seen: make(map[uint64]struct{}, MaxSketchRun)}
		sessions[id] = s
	}
	return s
}

func (s *sketchSession) drop() {
	if len(sessions) == 0 {
		return
	}
	sessionsMu.Lock()
	defer sessionsMu.Unlock()
	for k := range sessions {
		delete(sessions, k)
	}
}

// naiveSketchTerm is the retracted fold, reproduced verbatim so the demo can
// A/B it. Do not copy this into internal/: the repository's own
// fsvm.SketchTerm is the corrected form, and sketch_fold_test.go exists to
// stop the naive version coming back.
func naiveSketchTerm(seeds [2]uint64, b, w uint8) uint64 {
	return seeds[b&1] + uint64(w&0x3F)
}

func (*sketchAPI) entropy(raw json.RawMessage) (result, error) {
	var a sketchEntropyArgs
	if err := json.Unmarshal(raw, &a); err != nil {
		return result{}, errf(CodeBadArgs, "sketch.entropy: %v", err)
	}

	term := fsvm.SketchTerm
	variant, cap := "fixed", 0
	switch a.Variant {
	case "", "fixed":
		term = fsvm.SketchTerm
	case "naive":
		term = naiveSketchTerm
		// seeds[b] contributes one of 2 high words, W contributes 0..63:
		// at most 2*64 = 128 low words, so 2*128 = 256. The window's 6 bits
		// cannot touch the upper 56 bits at all.
		cap = 256
	default:
		return result{}, errf(CodeBadArgs, "sketch.entropy: variant must be \"fixed\" or \"naive\", got %q", a.Variant)
	}

	total := clampInt(defaultIfZero(a.Streams, 20000), 1, MaxSketchRun)
	minLen := clampInt(defaultIfZero(a.MinLen, 1), 1, 4096)
	maxLen := clampInt(defaultIfZero(a.MaxLen, 400), minLen, 4096)
	offset := clampInt(a.Offset, 0, total)
	seed := a.Seed
	if seed == 0 {
		seed = 1
	}

	// Stream i always uses the same derived seed, so a chunked sweep and a
	// single-call sweep see an identical sample.
	count := minInt(total-offset, clampInt(defaultIfZero(a.Count, MaxSketchRun), 1, MaxSketchRun))
	acc := sessionFor(a.Session, a.Reset)
	firstCollisionInChunk := 0
	chunkDistinct := 0

	for i := offset; i < offset+count; i++ {
		rng := rand.New(rand.NewSource(seed + int64(i)*7919))
		n := minLen + rng.Intn(maxLen-minLen+1)
		sketch := uint64(0)
		w := uint8(0)
		for j := 0; j < n; j++ {
			var b uint8
			if rng.Intn(2) == 1 {
				b = 1
			}
			w = ((w << 1) | b) & 0x3F
			sketch ^= term(fsvm.DefaultSeeds, b, w)
		}
		if _, dup := acc.seen[sketch]; dup {
			if firstCollisionInChunk == 0 {
				firstCollisionInChunk = i + 1
			}
		} else {
			acc.seen[sketch] = struct{}{}
			acc.distinct++
			chunkDistinct++
		}
		acc.total++
		acc.popSum += int64(popcount(sketch))
		if acc.firstHit == 0 && firstCollisionInChunk > 0 {
			acc.firstHit = firstCollisionInChunk
		}
	}

	// A session accumulates memory that is never reclaimed until it is reset,
	// so cap it rather than letting a runaway client grow the map without
	// bound. The cap is on distinct values, which is the dominant term.
	if len(acc.seen) > MaxSketchRun {
		acc.drop()
		return result{}, errf(CodeInputTooLarge,
			"sketch session %q exceeded %d distinct sketches; start a new session id or set reset",
			a.Session, MaxSketchRun)
	}

	out := sketchEntropyOut{
		Variant:               variant,
		Distinct:              acc.distinct,
		Total:                 acc.total,
		Collisions:            acc.total - acc.distinct,
		ChunkDistinct:         chunkDistinct,
		ChunkTotal:            count,
		FirstCollisionInChunk: firstCollisionInChunk,
		Cap:                   cap,
		Seed:                  seed,
		MinLen:                minLen,
		MaxLen:                maxLen,
		Offset:                offset,
		Done:                  offset+count >= total,
		Session:               a.Session,
	}
	if acc.total > 0 {
		out.MeanPopcount = safeFloat(float64(acc.popSum) / float64(acc.total))
	}
	return result{Data: out}, nil
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// reset drops every accumulated sketch session. Exposed so a page that has
// finished a sweep can reclaim the memory without minting a new session id,
// and so a long-lived tab does not carry the maps forever.
func (*sketchAPI) resetSessions(json.RawMessage) (result, error) {
	(&sketchSession{}).drop()
	return result{Data: map[string]any{"cleared": true}}, nil
}
