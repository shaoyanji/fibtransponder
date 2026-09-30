// Package wasmapi is a JSON transport over the fibtransponder internal
// packages, for the portfolio site's interactive demos.
//
// It exists so the browser runs this repository's real code rather than a
// JavaScript re-implementation of it: every number the site displays is
// produced by the same function the Go tests assert on.
//
// Design constraints, all deliberate:
//
//   - No syscall/js anywhere in this package. The js/wasm glue lives in
//     cmd/fibwasm. That keeps this package natively testable, so the golden
//     values in wasmapi_test.go run under `go test` in normal CI.
//
//   - One generic dispatcher, Call(method, argsJSON), instead of per-method
//     exports. syscall/js cannot pass strings through //go:wasmexport, and a
//     single surface keeps the JS side to one function.
//
//   - Every uint64 crosses the boundary as a decimal *string*, never a JSON
//     number. JavaScript numbers are float64 and silently lose integer
//     precision above 2^53, which every sketch here exceeds. MarshalU64 and
//     the U64 type exist solely to make that impossible to get wrong; the
//     round-trip test in wasmapi_test.go guards it.
//
//   - Input is capped rather than trusted. Ingest must not be a way to hang
//     the browser, which is the same unDoSable constraint docs/SPEC.md §5
//     places on the core.
package wasmapi

import (
	"encoding/json"
	"fmt"
	"math"
	"math/bits"
	"strconv"

	"github.com/shaoyanji/fibtransponder/internal/zeckendorf"
)

// Error codes. These are part of the wire contract: the client switches on
// them, so treat them as stable.
const (
	CodeBadArgs           = "bad_args"
	CodeUnsupportedMethod = "unsupported_method"
	CodeInputTooLarge     = "input_too_large"
	CodeInternal          = "internal"
)

// Input caps. Chosen to sit well inside a browser tab's comfort zone while
// staying far above anything the demos display.
const (
	MaxTextBytes = 64 << 10  // 64 KiB of text
	MaxStreamBit = 512 << 10 // 512 Kibi of raw bits
	MaxTraceBit  = 64 << 10  // 64 Kibi of per-bit trace
	MaxTreeNodes = 20000     // burst-hierarchy nodes before truncation
	MaxSketchRun = 200000    // random streams per sketch.entropy call
)

// envelope is the wire format for every reply.
type envelope struct {
	OK       bool            `json:"ok"`
	Data     json.RawMessage `json:"data,omitempty"`
	Warnings []string        `json:"warnings,omitempty"`
	Code     string          `json:"code,omitempty"`
	Error    string          `json:"error,omitempty"`
}

// result is what a handler returns: payload plus any caveats the client should
// surface rather than swallow. Warnings exist because some upstream functions
// are lossy in ways a visitor pasting their own text would otherwise hit
// silently — see the non-ASCII note in input.go.
type result struct {
	Data     any
	Warnings []string
}

func (r result) warn(format string, args ...any) result {
	r.Warnings = append(r.Warnings, fmt.Sprintf(format, args...))
	return r
}

// with attaches the payload to a result that may already carry warnings.
func (r result) with(v any) result {
	r.Data = v
	return r
}

type apiError struct {
	code string
	msg  string
}

func (e *apiError) Error() string { return e.msg }

func errf(code, format string, args ...any) *apiError {
	return &apiError{code: code, msg: fmt.Sprintf(format, args...)}
}

// Call dispatches one method and returns a JSON envelope. It never panics out
// to the caller: a bad method or bad args comes back as a coded error, because
// in wasm a panic that escapes main tears down the module and the page has to
// be reloaded to recover.
func Call(method string, argsJSON []byte) []byte {
	res, err := dispatch(method, argsJSON)
	if err != nil {
		ae := &apiError{code: CodeInternal, msg: err.Error()}
		var typed *apiError
		if as(err, &typed) {
			ae = typed
		}
		return mustMarshal(envelope{OK: false, Code: ae.code, Error: ae.msg})
	}
	raw, mErr := json.Marshal(res.Data)
	if mErr != nil {
		return mustMarshal(envelope{
			OK:    false,
			Code:  CodeInternal,
			Error: "result is not JSON-serialisable: " + mErr.Error(),
		})
	}
	return mustMarshal(envelope{OK: true, Data: raw, Warnings: res.Warnings})
}

func mustMarshal(v envelope) []byte {
	// envelope is a fixed shape of string/bool/RawMessage, so this cannot
	// fail. Falling back keeps Call total.
	b, err := json.Marshal(v)
	if err != nil {
		return []byte(`{"ok":false,"code":"internal","error":"envelope marshal failed"}`)
	}
	return b
}

func dispatch(method string, argsJSON []byte) (result, error) {
	h, ok := methods()[method]
	if !ok {
		return result{}, errf(CodeUnsupportedMethod, "unknown method %q", method)
	}
	args := json.RawMessage(argsJSON)
	if len(args) == 0 {
		args = json.RawMessage(`{}`)
	}
	return h(args)
}

type handler func(args json.RawMessage) (result, error)

// Zero-size receivers, bound once. The APIs carry no state: every call is a
// pure function of its arguments, which is what lets the client re-run an
// experiment with no session to keep alive.
var (
	apiFSVM       = &fsvmAPI{}
	apiZeck       = &zeckAPI{}
	apiTree       = &treeAPI{}
	apiStructural = &structuralAPI{}
	apiSketch     = &sketchAPI{}
	apiHilbert    = &hilbertAPI{}
)

var methodTable map[string]handler

func methods() map[string]handler {
	if methodTable == nil {
		methodTable = map[string]handler{
			"fsvm.meta":             apiFSVM.meta,
			"fsvm.run":              apiFSVM.run,
			"zeck.profile":          apiZeck.profile,
			"tree.build":            apiTree.build,
			"structural.matrix":     apiStructural.matrix,
			"structural.nesting":    apiStructural.nesting,
			"structural.corpus":     apiStructural.corpus,
			"sketch.entropy":        apiSketch.entropy,
			"sketch.sessions.reset": apiSketch.resetSessions,
			"hilbert.encode":        apiHilbert.encode,
			"hilbert.path":          apiHilbert.path,
		}
	}
	return methodTable
}

// ── uint64 across an f64 boundary ──

// U64 is a uint64 serialised as a decimal string.
//
// JavaScript cannot represent a uint64 as a number: f64 carries 53 bits of
// mantissa, and the smallest sketch in this repository is already ~7.6e18.
// A string is exact, parses losslessly via BigInt in the client, and reads
// better on the page than a hex literal.
type U64 string

func U(v uint64) U64 { return U64(strconv.FormatUint(v, 10)) }

// MarshalJSON emits the decimal string, never a number.
func (u U64) MarshalJSON() ([]byte, error) {
	return json.Marshal(string(u))
}

// UnmarshalJSON rejects a bare JSON number on purpose. Accepting one would
// mean it had already been through an f64 and lost precision, so the honest
// response is to refuse rather than round-trip corrupted data.
func (u *U64) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return fmt.Errorf("U64 must be a JSON string, not a number: %s", string(b))
	}
	*u = U64(s)
	return nil
}

// Uint64 parses the decimal string back, for the round-trip test.
func (u U64) Uint64() (uint64, error) { return strconv.ParseUint(string(u), 10, 64) }

// ── helpers ──

// safeFloat renders a possibly non-finite float in a form that survives JSON.
// encoding/json refuses NaN and ±Inf, and several of the upstream metrics
// divide by a count that can legitimately be zero (Analyze's Density over an
// empty stream, Residual over fewer than two bits).
func safeFloat(v float64) float64 {
	if math.IsNaN(v) {
		return 0
	}
	if math.IsInf(v, 1) {
		return math.MaxFloat64
	}
	if math.IsInf(v, -1) {
		return -math.MaxFloat64
	}
	return v
}

// ratio guards a division that can hit a zero denominator.
func ratio(num, den int) float64 {
	if den == 0 {
		return 0
	}
	return safeFloat(float64(num) / float64(den))
}

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// as is a tiny stand-in for errors.As, kept local so the wasm build does not
// pull in reflect-heavy paths for one call site.
func as(err error, target **apiError) bool {
	if e, ok := err.(*apiError); ok {
		*target = e
		return true
	}
	return false
}

// popcount is the sketch's own divergence metric: segauto scores segments by
// popcount(startSketch ^ endSketch), and the stepper shows sketch bit-flips
// per step, so the page needs the same primitive.
func popcount(v uint64) int { return bits.OnesCount64(v) }

// residualOf lets the structural demo report adjacency density alongside the
// dilation rate, so a high rate can be attributed to structure rather than to
// a stream that is simply dense in ones.
func residualOf(b []uint8) float64 { return zeckendorf.Residual(b) }

// itoa is strconv.Itoa, named short because it appears inside struct literals
// on nearly every line of the experiment methods.
func itoa(v int) string { return strconv.Itoa(v) }
