package wasmapi

import (
	"encoding/json"
	"math"
	"strings"
	"testing"

	"github.com/shaoyanji/fibtransponder/internal/fib_coder"
)

// Golden tests for the portfolio shim.
//
// The point of this file is that every number the site displays is the number
// the repository's own CLI and tests produce. The values below were captured by
// running the real pipeline (go run ./cmd/... and the WASM build) and are pinned
// here so a change in internal/ that would silently alter the site's claims
// fails the build rather than quietly making the page wrong.

// callOK invokes a method and fails the test if the envelope is not ok.
func callOK(t *testing.T, method string, args any, out any) []string {
	t.Helper()
	raw, err := json.Marshal(args)
	if err != nil {
		t.Fatalf("%s: marshal args: %v", method, err)
	}
	return callOKRaw(t, method, string(raw), out)
}

func callOKRaw(t *testing.T, method, args string, out any) []string {
	t.Helper()
	resp := Call(method, []byte(args))

	var env struct {
		OK       bool            `json:"ok"`
		Data     json.RawMessage `json:"data"`
		Warnings []string        `json:"warnings"`
		Code     string          `json:"code"`
		Error    string          `json:"error"`
	}
	if err := json.Unmarshal(resp, &env); err != nil {
		t.Fatalf("%s: envelope is not JSON: %v\n%s", method, err, resp)
	}
	if !env.OK {
		t.Fatalf("%s: call failed [%s] %s", method, env.Code, env.Error)
	}
	if out != nil {
		if err := json.Unmarshal(env.Data, out); err != nil {
			t.Fatalf("%s: data does not unmarshal into %T: %v", method, out, err)
		}
	}
	return env.Warnings
}

// callErr invokes a method expecting a coded failure.
func callErr(t *testing.T, method, args, wantCode string) {
	t.Helper()
	resp := Call(method, []byte(args))
	var env struct {
		OK    bool   `json:"ok"`
		Code  string `json:"code"`
		Error string `json:"error"`
	}
	if err := json.Unmarshal(resp, &env); err != nil {
		t.Fatalf("%s: envelope is not JSON: %v", method, err)
	}
	if env.OK {
		t.Fatalf("%s: expected failure [%s], got ok", method, wantCode)
	}
	if env.Code != wantCode {
		t.Errorf("%s: expected code %q, got %q (%s)", method, wantCode, env.Code, env.Error)
	}
}

// ── the transport itself ──

// TestU64SurvivesJSONRoundTrip is the load-bearing test in this file.
//
// A uint64 emitted as a JSON number loses precision the moment JavaScript
// parses it: f64 carries 53 mantissa bits and every sketch in this repository
// is larger than 2^53. This walks the actual boundary -- Go value, JSON text,
// parse -- and asserts nothing was lost.
func TestU64SurvivesJSONRoundTrip(t *testing.T) {
	// Deliberately awkward values: f64 cannot represent any of these exactly.
	cases := []uint64{
		0,
		1,
		9_007_199_254_740_993,      // 2^53 + 1, the classic f64 cliff
		7_649_497_944_761_386_324,  // the real sketch from a 10-bit stream
		14_947_234_960_856_675_971, // the collapsed-fold sketch, bit 63 set
		18_261_337_404_106_225_172, // fsvm.Step's value for "hello world"
		math.MaxUint64,
	}
	for _, v := range cases {
		blob, err := json.Marshal(struct{ V U64 }{U(v)})
		if err != nil {
			t.Fatalf("marshal %d: %v", v, err)
		}
		// The wire form must be a quoted string, never a bare number.
		if !strings.HasPrefix(string(blob), `{"V":"`) {
			t.Errorf("U64 %d serialised as a JSON number: %s", v, blob)
		}
		// What the browser's JSON.parse would produce.
		var asJS struct{ V float64 }
		if err := json.Unmarshal(blob, &asJS); err == nil {
			if uint64(asJS.V) != v && asJS.V >= 0 {
				t.Errorf("U64 %d would not survive a float64 parse: got %.0f", v, asJS.V)
			}
		}
		var back struct{ V U64 }
		if err := json.Unmarshal(blob, &back); err != nil {
			t.Fatalf("unmarshal %d: %v", v, err)
		}
		got, err := back.V.Uint64()
		if err != nil {
			t.Fatalf("parse %q: %v", back.V, err)
		}
		if got != v {
			t.Errorf("round trip: want %d, got %d", v, got)
		}
	}
}

// TestU64RejectsJSONNumber guards the other direction: a bare number on the wire
// means it already went through an f64, so it must be refused rather than
// silently rounded.
func TestU64RejectsJSONNumber(t *testing.T) {
	var u U64
	if err := json.Unmarshal([]byte(`12345`), &u); err == nil {
		t.Errorf("U64 accepted a bare JSON number; precision is already lost by then")
	}
}

func TestUnknownMethodIsCoded(t *testing.T) {
	callErr(t, "fsvm.nope", `{}`, CodeUnsupportedMethod)
}

func TestBadArgsIsCoded(t *testing.T) {
	callErr(t, "fsvm.run", `{"version":7}`, CodeBadArgs)
	callErr(t, "sketch.entropy", `{"variant":"sideways"}`, CodeBadArgs)
	callErr(t, "structural.matrix", `{"texts":["a","b"],"labels":["only-one"]}`, CodeBadArgs)
}

func TestInputTooLargeIsCoded(t *testing.T) {
	huge := strings.Repeat("a", MaxTextBytes+1)
	blob, _ := json.Marshal(map[string]string{"text": huge})
	callErr(t, "zeck.profile", string(blob), CodeInputTooLarge)
}

// TestSafeFloatSurvivesJSON is the NaN/Inf guard: encoding/json refuses both,
// and several upstream metrics divide by a count that can be zero.
func TestSafeFloatSurvivesJSON(t *testing.T) {
	for _, v := range []float64{math.NaN(), math.Inf(1), math.Inf(-1), 0.5} {
		if _, err := json.Marshal(safeFloat(v)); err != nil {
			t.Errorf("safeFloat(%v) is not serialisable: %v", v, err)
		}
	}
	if got := safeFloat(math.NaN()); got != 0 {
		t.Errorf("safeFloat(NaN) = %v, want 0", got)
	}
	if got := ratio(1, 0); got != 0 {
		t.Errorf("ratio with zero denominator = %v, want 0", got)
	}
}

// ── fsvm.run ──

// TestFsvmRunMatchesCore is checked against a hand-traceable stream: the bits
// 1,1,0,0,0,0,0,0,0,0 produce one DILATE (the adjacent pair at the start) and
// one MARKER (zeroRun crossing 8), leaving r=1.
func TestFsvmRunMatchesCore(t *testing.T) {
	var out fsvmRunOut
	callOK(t, "fsvm.run", map[string]any{"bits": "1100000000"}, &out)

	if out.BitCount != 10 {
		t.Fatalf("bitCount = %d, want 10", out.BitCount)
	}
	if got := out.Final.Dilations; string(got) != "1" {
		t.Errorf("dilations = %s, want 1", got)
	}
	if got := out.Final.Markers; string(got) != "1" {
		t.Errorf("markers = %s, want 1", got)
	}
	if out.Final.R != 1 {
		t.Errorf("r = %d, want 1", out.Final.R)
	}
	if got, _ := out.Final.Sketch.Uint64(); got != 7649497944761386324 {
		t.Errorf("sketch = %d, want 7649497944761386324 (captured from the real pipeline)", got)
	}
	if got, _ := out.Final.BitsProcessed.Uint64(); got != 10 {
		t.Errorf("bitsProcessed = %d, want 10", got)
	}

	dilates, markers := 0, 0
	for _, e := range out.Events {
		switch e.Kind {
		case "DILATE":
			dilates++
			if e.Pos != 1 {
				t.Errorf("DILATE at pos %d, want 1 (the second bit completes the '11')", e.Pos)
			}
		case "MARKER":
			markers++
			if e.Pos != 9 {
				t.Errorf("MARKER at pos %d, want 9 (zeroRun reaches 8 there)", e.Pos)
			}
		}
	}
	if dilates != 1 || markers != 1 {
		t.Errorf("events: %d DILATE / %d MARKER, want 1 / 1", dilates, markers)
	}
}

func TestFsvmRunStepArraysAreDenseAndAligned(t *testing.T) {
	var out fsvmRunOut
	callOK(t, "fsvm.run", map[string]any{"bits": "1011001110000"}, &out)
	for name, n := range map[string]int{
		"r": len(out.R), "w": len(out.W), "zeroRun": len(out.ZeroRun),
		"sketch": len(out.Sketch), "sketchDelta": len(out.SketchDelta),
	} {
		if n != out.BitCount {
			t.Errorf("%s has %d entries, want %d (one per bit, index-aligned)", name, n, out.BitCount)
		}
	}
	if len(out.Bits) != out.BitCount {
		t.Errorf("bits string is %d chars, want %d", len(out.Bits), out.BitCount)
	}
	// The window must hold the last six bits with the most recent in the LSB,
	// so a short stream is checkable by hand.
	var want int
	for i := len(out.Bits) - 6; i < len(out.Bits); i++ {
		if out.Bits[i] == '1' {
			want |= 1 << uint(len(out.Bits)-1-i)
		}
	}
	if out.Final.W != want {
		t.Errorf("final w = %d, want %d (last six bits of the stream)", out.Final.W, want)
	}
}

func TestFsvmRunWidthNarrowsDilation(t *testing.T) {
	var w1, w3 fsvmRunOut
	callOK(t, "fsvm.run", map[string]any{"bits": "1111111", "width": 1}, &w1)
	callOK(t, "fsvm.run", map[string]any{"bits": "1111111", "width": 3}, &w3)

	d1, _ := w1.Final.Dilations.Uint64()
	d3, _ := w3.Final.Dilations.Uint64()
	if d1 != 6 {
		t.Errorf("w=1 dilations over 1111111 = %d, want 6", d1)
	}
	if d3 > d1 {
		t.Errorf("w=3 dilations (%d) exceeded w=1 (%d); the sets must nest", d3, d1)
	}
	// The fold does not see width, so the fingerprints must be identical.
	if w1.Final.Sketch != w3.Final.Sketch {
		t.Errorf("sketch changed with width: %s vs %s", w1.Final.Sketch, w3.Final.Sketch)
	}
}

func TestFsvmMetaReportsFullRange(t *testing.T) {
	var out fsvmMetaOut
	callOK(t, "fsvm.meta", map[string]any{}, &out)
	if out.WindowMask != 0x3F || out.WindowBits != 6 {
		t.Errorf("window is %d bits masked %x, want 6 bits masked 0x3f", out.WindowBits, out.WindowMask)
	}
	if out.MarkerFrom != 8 {
		t.Errorf("markerFrom = %d, want 8", out.MarkerFrom)
	}
	if len(out.Families) != 8 {
		t.Fatalf("%d hash families, want 8", len(out.Families))
	}
	for _, f := range out.Families {
		if !f.Odd {
			t.Errorf("family %d multiplier is even; the mixer would stop being bijective", f.ID)
		}
	}
	if len(out.Families) > 1 {
		for i := 1; i < len(out.Families); i++ {
			gap := out.Families[i].R - out.Families[i-1].R
			if gap < 6 {
				t.Errorf("family rotations %d..%d are %d apart, want >= 6",
					out.Families[i-1].R, out.Families[i].R, gap)
			}
		}
	}
}

// ── zeck.profile ──

// TestZeckProfileMatchesCLI pins the value cmd/zeck_residual prints for
// "hello world": 0.264368.
func TestZeckProfileMatchesCLI(t *testing.T) {
	var out zeckOut
	callOK(t, "zeck.profile", map[string]any{"text": "hello world"}, &out)

	const want = 0.26436781609195403
	if math.Abs(out.Residual-want) > 1e-12 {
		t.Errorf("residual = %.15f, want %.15f (cmd/zeck_residual prints 0.264368)", out.Residual, want)
	}
	if out.AdjacencyPairs == 0 {
		t.Errorf("adjacencyPairs = 0 but residual is %v; the numerator is not being counted", out.Residual)
	}
	if got, want := out.AdjacencyPairs, 23; got != want {
		t.Errorf("adjacencyPairs = %d, want %d (adj11 over len-1 for %q)", got, want, "hello world")
	}
}

// TestZeckResidualEdgeCases covers the cases the docs pin: alternating is 0,
// all ones is 1, and streams shorter than two bits are defined as 0.
func TestZeckResidualEdgeCases(t *testing.T) {
	cases := []struct {
		bits string
		want float64
	}{
		{"10101010", 0},
		{"11111111", 1},
		{"0", 0},
		{"1", 0},
		{"", 0},
		{"110010110", 0.25}, // 2 adjacent pairs over 8 gaps
	}
	for _, c := range cases {
		var out zeckOut
		callOK(t, "zeck.profile", map[string]any{"bits": c.bits, "sizes": []int{8}}, &out)
		if math.Abs(out.Residual-c.want) > 1e-12 {
			t.Errorf("residual(%q) = %v, want %v", c.bits, out.Residual, c.want)
		}
	}
}

// ── tree.build ──

// TestTreeBuildMatchesCLI pins the whole report for the fox sentence, which
// `go run ./cmd/dilation_tree -json` reports as
//
//	total_bits 360, dilations 77, depth 1, nodes 34, leaves 33,
//	balance 1.0000, skew 0.0303, entropy 0.1914
func TestTreeBuildMatchesCLI(t *testing.T) {
	const text = "The quick brown fox jumps over the lazy dog. "
	var out treeBuildOut
	callOK(t, "tree.build", map[string]any{"text": text}, &out)

	if out.TotalBits != 360 {
		t.Errorf("totalBits = %d, want 360 (45 runes x 8)", out.TotalBits)
	}
	if out.TotalDilations != 77 {
		t.Errorf("totalDilations = %d, want 77", out.TotalDilations)
	}
	if out.MaxDepth != 1 {
		t.Errorf("maxDepth = %d, want 1", out.MaxDepth)
	}
	if out.TotalNodes != 34 {
		t.Errorf("totalNodes = %d, want 34", out.TotalNodes)
	}
	if out.LeafCount != 33 {
		t.Errorf("leafCount = %d, want 33", out.LeafCount)
	}
	if math.Abs(out.Balance-1.0) > 5e-5 {
		t.Errorf("balance = %v, want 1.0000", out.Balance)
	}
	if math.Abs(out.SkewRatio-0.0303) > 5e-5 {
		t.Errorf("skewRatio = %v, want 0.0303", out.SkewRatio)
	}
	if math.Abs(out.DepthEntropy-0.1914) > 5e-5 {
		t.Errorf("depthEntropy = %v, want 0.1914", out.DepthEntropy)
	}
	want := "dilations=77 depth=1 nodes=34 leaves=33 balance=1.0000 skew=0.0303 entropy=0.1914 scale=2.3"
	if out.Report != want {
		t.Errorf("report =\n  %q\nwant\n  %q", out.Report, want)
	}
	if out.Truncated {
		t.Errorf("tree was truncated for a 45-rune input; the cap is wrong")
	}
}

// TestTreeBuildCountsRunesNotBytes is the UTF-8 caveat made testable:
// TextToBits emits 8 bits per rune, so a 3-rune em-dash string is 24 bits even
// though it is 3 bytes wide.
func TestTreeBuildCountsRunesNotBytes(t *testing.T) {
	var out treeBuildOut
	warnings := callOK(t, "tree.build", map[string]any{"text": "a—b"}, &out)
	if out.TotalBits != 24 {
		t.Errorf("totalBits = %d, want 24 (3 runes x 8, not 5 bytes x 8)", out.TotalBits)
	}
	if len(warnings) == 0 {
		t.Errorf("non-ASCII input produced no warning; the rune/byte divergence would be silent")
	}
	if out.NonASCII != "—" {
		t.Errorf("nonAsciiRune = %q, want the em dash", out.NonASCII)
	}
}

func TestTreeBuildRespectsNodeCap(t *testing.T) {
	var out treeBuildOut
	callOK(t, "tree.build", map[string]any{
		"text": strings.Repeat("the quick brown fox. ", 40), "maxNodes": 5,
	}, &out)
	if !out.Truncated {
		t.Errorf("a 5-node cap over 880 runes did not report truncation")
	}
}

// ── structural.matrix / structural.nesting ──

// TestStructuralNestingHolds is the retraction demo's assertion, run live. It
// must never report a violation: the widths are ordered thresholds, so
// dilation counts are non-increasing in width for every possible input.
func TestStructuralNestingHolds(t *testing.T) {
	var out nestingOut
	callOK(t, "structural.nesting", map[string]any{"streams": 500, "seed": 20240914}, &out)
	if out.Violations != 0 {
		t.Errorf("%d violations of the nested-width invariant; the site would be "+
			"advertising a claim the code contradicts", out.Violations)
	}
	if out.FirstFail != "" {
		t.Errorf("first failing stream reported as %q", out.FirstFail)
	}
	if len(out.MeanRate) != 3 {
		t.Fatalf("meanRatePerWidth has %d entries, want 3", len(out.MeanRate))
	}
	if !(out.MeanRate[0] >= out.MeanRate[1] && out.MeanRate[1] >= out.MeanRate[2]) {
		t.Errorf("mean rates are not nested: %v", out.MeanRate)
	}
}

func TestStructuralNestingIsDeterministic(t *testing.T) {
	var a, b nestingOut
	args := map[string]any{"streams": 200, "seed": 777}
	callOK(t, "structural.nesting", args, &a)
	callOK(t, "structural.nesting", args, &b)
	for i := range a.MeanRate {
		if a.MeanRate[i] != b.MeanRate[i] {
			t.Fatalf("run %d differs between identical calls: %v vs %v", i, a.MeanRate[i], b.MeanRate[i])
		}
	}
}

func TestStructuralMatrixSketchIndependentOfWidth(t *testing.T) {
	var out structuralMatrixOut
	callOK(t, "structural.matrix", map[string]any{
		"texts":  []string{"the quick brown fox jumps over the lazy dog"},
		"labels": []string{"prose"},
	}, &out)

	if len(out.Rows) != 1 {
		t.Fatalf("%d rows, want 1", len(out.Rows))
	}
	row := out.Rows[0]
	if len(row.Cells) != 3 {
		t.Fatalf("%d width cells, want 3", len(row.Cells))
	}
	// Dilation rate must not increase with width.
	for i := 1; i < len(row.Cells); i++ {
		if row.Cells[i].DilRate > row.Cells[i-1].DilRate {
			t.Errorf("width %d rate %v exceeds width %d rate %v",
				row.Cells[i].Width, row.Cells[i].DilRate,
				row.Cells[i-1].Width, row.Cells[i-1].DilRate)
		}
	}
	// Exactly one cell is flagged most-sensitive.
	flagged := 0
	for _, c := range row.Cells {
		if c.MostSensitive {
			flagged++
		}
	}
	if flagged != 1 {
		t.Errorf("%d cells flagged most-sensitive, want exactly 1", flagged)
	}
	// Sketch must not depend on width.
	if len(out.SketchPerWidth) == 3 {
		if out.SketchPerWidth[0] != out.SketchPerWidth[1] || out.SketchPerWidth[1] != out.SketchPerWidth[2] {
			t.Errorf("sketch varied with width: %v", out.SketchPerWidth)
		}
	}
}

// ── structural.corpus ──

// TestStructuralCorpusReproducesPublishedTable is the check that keeps the site
// honest. REPORT_STRUCTURAL.md §1 publishes a 3x3 dilation-rate table; this
// runs the experiment live and diffs it against that transcription, so if a
// change in internal/ would invalidate the published document, the build fails
// instead of the page quietly disagreeing with the repo's own report.
//
//	| width | prose | code    | synthetic |
//	| w=1   | 0.1992| 0.1763  | 0.0313    |
//	| w=2   | 0.0582| 0.0609  | 0.0000    |
//	| w=3   | 0.0079| 0.0150  | 0.0000    |
func TestStructuralCorpusReproducesPublishedTable(t *testing.T) {
	var out structuralCorpusOut
	callOK(t, "structural.corpus", map[string]any{}, &out)

	if len(out.Classes) != 3 {
		t.Fatalf("%d classes, want 3", len(out.Classes))
	}
	// Byte counts must match REPORT_CORPUS.md.
	wantBytes := map[string]int{"prose": 1007, "code": 1185, "synthetic": 1536}
	for _, row := range out.Classes {
		if got, want := out.CorpusBytes[row.Label], wantBytes[row.Label]; got != want {
			t.Errorf("%s is %d bytes, REPORT_CORPUS.md says %d", row.Label, got, want)
		}
	}

	for _, row := range out.Classes {
		want := publishedCorpusRates[row.Label]
		for _, c := range row.Cells {
			w, ok := want[c.Width]
			if !ok {
				continue
			}
			// The report quotes four decimals, so compare at that precision.
			if math.Abs(round4(c.DilRate)-w) > 5e-5 {
				t.Errorf("%s w=%d live rate %.4f, REPORT_STRUCTURAL.md publishes %.4f",
					row.Label, c.Width, c.DilRate, w)
			}
		}
	}

	// The synthetic control must vanish entirely at w>=2: it never produces
	// three or more consecutive 1-bits, so a wider window suppresses it to zero.
	syn := rowByLabel(t, out.Classes, "synthetic")
	for _, c := range syn.Cells {
		if c.Width >= 2 && c.Dilations != "0" {
			t.Errorf("synthetic w=%d produced %s dilations, want 0", c.Width, c.Dilations)
		}
	}

	// The sensitivity ranking shift is the observation that survived the
	// retraction: w=1 favours prose, w>=2 favours code.
	prose := rowByLabel(t, out.Classes, "prose")
	code := rowByLabel(t, out.Classes, "code")
	if !(dilRate(prose, 1) > dilRate(code, 1)) {
		t.Errorf("at w=1 prose (%.4f) should be the more sensitive class, ahead of code (%.4f)",
			dilRate(prose, 1), dilRate(code, 1))
	}
	if !(dilRate(code, 2) > dilRate(prose, 2)) {
		t.Errorf("at w=2 code (%.4f) should be the more sensitive class, ahead of prose (%.4f)",
			dilRate(code, 2), dilRate(prose, 2))
	}

	// Second-axis retraction: no marker fires anywhere in the 3x3 grid.
	if !out.MarkersAllZero {
		t.Errorf("a marker fired on the published corpus; the second-axis retraction " +
			"states none do, so the page's claim would now be wrong")
	}
}

func TestStructuralCorpusNestsAcrossClasses(t *testing.T) {
	var out structuralCorpusOut
	callOK(t, "structural.corpus", map[string]any{}, &out)
	for _, row := range out.Classes {
		for i := 1; i < len(row.Cells); i++ {
			if row.Cells[i].DilRate > row.Cells[i-1].DilRate {
				t.Errorf("%s: w=%d rate %.6f exceeds w=%d rate %.6f; the sets must nest",
					row.Label, row.Cells[i].Width, row.Cells[i].DilRate,
					row.Cells[i-1].Width, row.Cells[i-1].DilRate)
			}
		}
	}
}

func rowByLabel(t *testing.T, rows []structuralRow, label string) structuralRow {
	t.Helper()
	for _, r := range rows {
		if r.Label == label {
			return r
		}
	}
	t.Fatalf("no row labelled %q", label)
	return structuralRow{}
}

func dilRate(row structuralRow, width int) float64 {
	for _, c := range row.Cells {
		if c.Width == width {
			return c.DilRate
		}
	}
	return math.NaN()
}

func round4(v float64) float64 { return math.Round(v*10000) / 10000 }

// TestSketchEntropyFixedSpread is the bug-fix demo's headline: with the window
// spread across the word, distinct sketches track the stream count closely.
func TestSketchEntropyFixedSpread(t *testing.T) {
	const streams = 5000
	var out sketchEntropyOut
	callOK(t, "sketch.entropy", map[string]any{
		"variant": "fixed", "streams": streams, "minLen": 1, "maxLen": 400, "seed": 1,
	}, &out)

	if out.Variant != "fixed" {
		t.Fatalf("variant = %q", out.Variant)
	}
	// docs report ~19713 distinct over 20000; allow the same shape here.
	if out.Distinct < streams*95/100 {
		t.Errorf("fixed fold produced only %d distinct sketches over %d streams; "+
			"the window spread is not working", out.Distinct, streams)
	}
	if out.Cap != 0 {
		t.Errorf("the fixed fold should have no structural cap, got %d", out.Cap)
	}
	// A sketch that is not roughly half-ones is a red flag.
	if out.MeanPopcount < 24 || out.MeanPopcount > 40 {
		t.Errorf("mean popcount %.1f/64, want roughly 32", out.MeanPopcount)
	}
}

// TestSketchEntropyNaiveCollapses is the retracted fold. It cannot exceed 256
// distinct values, which is the whole point of the comparison.
func TestSketchEntropyNaiveCollapses(t *testing.T) {
	const streams = 5000
	var out sketchEntropyOut
	callOK(t, "sketch.entropy", map[string]any{
		"variant": "naive", "streams": streams, "minLen": 1, "maxLen": 400, "seed": 1,
	}, &out)

	if out.Cap != 256 {
		t.Errorf("naive cap = %d, want 256 (2 seeds x 64 window values)", out.Cap)
	}
	if out.Distinct > 256 {
		t.Errorf("naive fold produced %d distinct sketches, which exceeds its own 256 cap",
			out.Distinct)
	}
	if out.Distinct > streams/10 {
		t.Errorf("naive fold produced %d distinct over %d streams; expected saturation "+
			"near 256", out.Distinct, streams)
	}
	if out.Collisions <= 0 {
		t.Errorf("naive fold reported no collisions, which cannot be right over %d streams", streams)
	}
}

// TestSketchSessionAccumulatesDistinctCorrectly pins the fix for a real bug.
//
// The distinct count used to be per-chunk, so a client sweeping in chunks and
// adding the results up over-counted. The naive fold saturates at 256 per
// chunk, so three chunks reported 768 distinct values for a fold whose
// mathematical ceiling is 256. A session carries the seen-set across calls so
// the number means what it says.
func TestSketchSessionAccumulatesDistinctCorrectly(t *testing.T) {
	const total = 6000
	const chunk = 2000

	// Chunked, with a session: the reported count must never exceed the fold's
	// own ceiling, and must equal the single-call result.
	var sessioned sketchEntropyOut
	for off := 0; off < total; off += chunk {
		callOK(t, "sketch.entropy", map[string]any{
			"variant": "naive", "streams": total, "offset": off, "count": chunk,
			"seed": 1, "minLen": 1, "maxLen": 400,
			"session": "test-naive", "reset": off == 0,
		}, &sessioned)
		if sessioned.Distinct > 256 {
			t.Fatalf("sessioned distinct = %d after %d streams; the naive fold "+
				"cannot exceed its 256 ceiling", sessioned.Distinct, sessioned.Total)
		}
	}
	if sessioned.Total != total {
		t.Errorf("session accumulated %d streams, want %d", sessioned.Total, total)
	}

	// The naive fold must actually saturate, not merely stay under the cap.
	if sessioned.Distinct < 200 {
		t.Errorf("naive fold produced only %d distinct over %d streams; expected it to "+
			"saturate near 256", sessioned.Distinct, sessioned.Total)
	}

	// One call, no session, same sample: identical distinct count.
	var whole sketchEntropyOut
	callOK(t, "sketch.entropy", map[string]any{
		"variant": "naive", "streams": total, "seed": 1, "minLen": 1, "maxLen": 400,
	}, &whole)
	if whole.Distinct != sessioned.Distinct {
		t.Errorf("chunked-with-session gave %d distinct, single call gave %d; the "+
			"two must agree because the sample is identical", sessioned.Distinct, whole.Distinct)
	}
	if whole.Collisions != total-whole.Distinct {
		t.Errorf("collisions = %d, want total-distinct = %d", whole.Collisions, total-whole.Distinct)
	}

	// Chunk fields describe this call only, so a progress bar can use them.
	if whole.ChunkTotal != total || whole.ChunkDistinct != whole.Distinct {
		t.Errorf("single call: chunkTotal %d chunkDistinct %d, want %d and %d",
			whole.ChunkTotal, whole.ChunkDistinct, total, whole.Distinct)
	}
}

// TestSketchSessionResetClears is what lets the page re-run a sweep.
func TestSketchSessionResetClears(t *testing.T) {
	args := map[string]any{
		"variant": "naive", "streams": 2000, "seed": 1,
		"minLen": 1, "maxLen": 400, "session": "test-reset",
	}
	var first sketchEntropyOut
	callOK(t, "sketch.entropy", args, &first)
	if first.Distinct == 0 || first.Total != 2000 {
		t.Fatalf("first run: distinct %d total %d", first.Distinct, first.Total)
	}

	// Without a reset the session keeps accumulating stream counts. The naive
	// fold is already saturated at 256 after the first 2000, so distinct cannot
	// rise -- which is itself the point of the demo.
	var second sketchEntropyOut
	callOK(t, "sketch.entropy", args, &second)
	if second.Total != 4000 {
		t.Errorf("second run without reset accumulated %d streams, want 4000", second.Total)
	}
	if second.Distinct != first.Distinct {
		t.Errorf("distinct changed from %d to %d across 2000 more streams of a saturated "+
			"fold; it should not have", first.Distinct, second.Distinct)
	}
	if second.Collisions != second.Total-second.Distinct {
		t.Errorf("collisions = %d, want total-distinct = %d",
			second.Collisions, second.Total-second.Distinct)
	}

	// With a reset it starts clean.
	var third sketchEntropyOut
	resetArgs := map[string]any{}
	for k, v := range args {
		resetArgs[k] = v
	}
	resetArgs["reset"] = true
	callOK(t, "sketch.entropy", resetArgs, &third)
	if third.Total != 2000 || third.Distinct != first.Distinct {
		t.Errorf("after reset: total %d distinct %d, want 2000 and %d",
			third.Total, third.Distinct, first.Distinct)
	}

	// The bulk-clear endpoint exists so a page can reclaim memory without
	// minting a new session id.
	var cleared struct {
		Cleared bool `json:"cleared"`
	}
	callOK(t, "sketch.sessions.reset", map[string]any{}, &cleared)
	if !cleared.Cleared {
		t.Errorf("reset reported cleared=false")
	}
	var afterClear sketchEntropyOut
	callOK(t, "sketch.entropy", args, &afterClear)
	if afterClear.Total != 2000 {
		t.Errorf("after a bulk reset the session should be empty, but it reported %d "+
			"accumulated streams", afterClear.Total)
	}
}
func TestSketchEntropyChunkingIsDeterministic(t *testing.T) {
	args := map[string]any{
		"variant": "fixed", "streams": 900, "offset": 300, "count": 300,
		"seed": 42, "minLen": 1, "maxLen": 120,
	}
	var a, b sketchEntropyOut
	callOK(t, "sketch.entropy", args, &a)
	callOK(t, "sketch.entropy", args, &b)
	if a.Distinct != b.Distinct || a.MeanPopcount != b.MeanPopcount {
		t.Errorf("identical chunk requests differ: %d/%v vs %d/%v",
			a.Distinct, a.MeanPopcount, b.Distinct, b.MeanPopcount)
	}
	if a.Offset != 300 || a.Total != 300 {
		t.Errorf("chunk covered offset %d size %d, want 300/300", a.Offset, a.Total)
	}
	if a.Done {
		t.Errorf("a chunk ending at 600 of a 900-stream sample reported done")
	}

	// The last chunk of the same sweep must report done and cover the rest.
	var tail sketchEntropyOut
	callOK(t, "sketch.entropy", map[string]any{
		"variant": "fixed", "streams": 900, "offset": 600, "count": 300,
		"seed": 42, "minLen": 1, "maxLen": 120,
	}, &tail)
	if !tail.Done {
		t.Errorf("the chunk that reaches 900 of 900 did not report done")
	}
	if a.Distinct == tail.Distinct && a.MeanPopcount == tail.MeanPopcount {
		t.Errorf("two different chunks of the sweep returned identical statistics; " +
			"the offset is not selecting different streams")
	}

	// One call with no count must cover the whole remainder in a single go.
	var whole sketchEntropyOut
	callOK(t, "sketch.entropy", map[string]any{
		"variant": "fixed", "streams": 600, "offset": 0, "seed": 42, "minLen": 1, "maxLen": 120,
	}, &whole)
	if whole.Total != 600 {
		t.Fatalf("full run covered %d streams, want 600", whole.Total)
	}
	if !whole.Done {
		t.Errorf("a run that reached the requested total did not report done")
	}
	if whole.Distinct <= 300 {
		t.Errorf("600 streams gave %d distinct, no better than a 300-stream chunk", whole.Distinct)
	}
}

// ── hilbert ──

// TestHilbertEncodeCheckerboard pins the deterministic default: an 8x8
// checkerboard at order 3 is 64 bits, half set.
func TestHilbertEncodeCheckerboard(t *testing.T) {
	var out hilbertEncodeOut
	callOK(t, "hilbert.encode", map[string]any{
		"order": 3, "threshold": 128, "checkerboard": true,
	}, &out)

	if out.Size != 8 || out.BitCount != 64 {
		t.Fatalf("size %d, bitCount %d; want 8 and 64", out.Size, out.BitCount)
	}
	if out.Ones != 32 {
		t.Errorf("ones = %d, want 32 for an 8x8 checkerboard", out.Ones)
	}
	// The 16-byte container header: two uint32 dimensions then a uint64 length.
	if len(out.HeaderHex) != 32 {
		t.Errorf("container header is %d hex chars, want 32", len(out.HeaderHex))
	}
	if out.HeaderHex[:8] != "00000008" || out.HeaderHex[8:16] != "00000008" {
		t.Errorf("header dimensions = %s", out.HeaderHex[:16])
	}
	if out.HeaderHex[16:] != "0000000000000040" {
		t.Errorf("header bit length = %s, want 0x40 = 64", out.HeaderHex[16:])
	}
	if len(out.Runs) == 0 {
		t.Errorf("checkerboard produced no runs; the display table is empty")
	}
}

// TestHilbertEncodeRunTableMatchesEncoder is the consistency check that makes
// the reveal animation trustworthy: the exact bit count the run table claims
// must equal what fib_coder.Encode actually produced, modulo final padding.
func TestHilbertEncodeRunTableMatchesEncoder(t *testing.T) {
	for _, order := range []int{2, 3, 4, 5} {
		var out hilbertEncodeOut
		callOK(t, "hilbert.encode", map[string]any{
			"order": order, "threshold": 128, "checkerboard": true,
		}, &out)
		if out.ExactCoded <= 0 {
			t.Errorf("order %d: exactCoded = %d", order, out.ExactCoded)
		}
		if out.CodedBits < out.ExactCoded {
			t.Errorf("order %d: on-disk coded bits %d is fewer than the exact count %d; "+
				"the encoder should only pad upward", order, out.CodedBits, out.ExactCoded)
		}
		if out.CodedBits-out.ExactCoded > 7 {
			t.Errorf("order %d: %d bits of padding, want at most 7", order, out.CodedBits-out.ExactCoded)
		}
		// Runs must tile the input exactly.
		covered := 0
		for _, r := range out.Runs {
			if r.Bit != covered {
				t.Errorf("order %d: run starts at %d, want %d (gap or overlap)", order, r.Bit, covered)
				break
			}
			covered += r.Length
		}
		if covered != out.BitCount {
			t.Errorf("order %d: runs cover %d bits, want %d", order, covered, out.BitCount)
		}
	}
}

// TestHilbertTerminatorNeverAppearsInternally is the self-synchronising
// property the compression design rests on: a Zeckendorf word has no adjacent
// 1-bits, so "011" can only ever be the terminator.
func TestHilbertTerminatorNeverAppearsInternally(t *testing.T) {
	for _, n := range []int{1, 2, 3, 4, 5, 10, 20, 33, 54, 100, 1000, 12586269025} {
		code, err := fib_coder.IntToFibonacciCode(n)
		if err != nil {
			t.Fatalf("code(%d): %v", n, err)
		}
		if code == "" {
			continue
		}
		if strings.Contains(code, "011") {
			t.Errorf("code(%d) = %q contains the terminator internally", n, code)
		}
		// And it must round-trip, since the run table quotes these verbatim.
		back, err := fib_coder.FibonacciCodeToInt(code)
		if err != nil {
			t.Errorf("decode(%q): %v", code, err)
			continue
		}
		if back != n {
			t.Errorf("round trip: code(%d) = %q decoded to %d", n, code, back)
		}
	}
}

func TestHilbertPathVisitsDistinctPoints(t *testing.T) {
	var out hilbertPathOut
	callOK(t, "hilbert.path", map[string]any{"order": 4}, &out)

	n := 16
	if out.Size != n {
		t.Fatalf("size = %d, want 16", out.Size)
	}
	if out.PointCount != n*n {
		t.Errorf("pointCount = %d, want %d", out.PointCount, n*n)
	}
	seen := make(map[int]bool, n*n)
	for i := 0; i < len(out.Pairs); i += 2 {
		x, y := out.Pairs[i], out.Pairs[i+1]
		if x < 0 || x >= n || y < 0 || y >= n {
			t.Fatalf("point (%d,%d) is outside the %dx%d grid", x, y, n, n)
		}
		key := y*n + x
		if seen[key] {
			t.Fatalf("point (%d,%d) visited twice; the curve is not a bijection", x, y)
		}
		seen[key] = true
	}
	// d=0 must be the origin.
	if out.Pairs[0] != 0 || out.Pairs[1] != 0 {
		t.Errorf("traversal starts at (%d,%d), want (0,0)", out.Pairs[0], out.Pairs[1])
	}
}

// ── helpers ──

// TestEveryMethodIsReachable keeps the method table from drifting out of sync
// with the documented surface.
func TestEveryMethodIsReachable(t *testing.T) {
	want := []string{
		"fsvm.meta", "fsvm.run", "zeck.profile", "tree.build",
		"structural.matrix", "structural.nesting", "structural.corpus",
		"sketch.entropy", "sketch.sessions.reset",
		"hilbert.encode", "hilbert.path",
	}
	for _, m := range want {
		if _, ok := methods()[m]; !ok {
			t.Errorf("method %q is documented but not in the table", m)
		}
	}
	if len(methods()) != len(want) {
		t.Errorf("method table has %d entries, want %d; an undocumented method may have crept in",
			len(methods()), len(want))
	}
}
