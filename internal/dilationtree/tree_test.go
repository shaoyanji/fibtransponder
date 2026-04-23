package dilationtree

import "testing"

func TestBuildEmpty(t *testing.T) {
	c := New()
	tree := c.Build()
	if tree == nil {
		t.Fatal("expected non-nil tree")
	}
	if tree.Depth() != 0 {
		t.Errorf("root depth should be 0, got %d", tree.Depth())
	}
}

func TestBuildSimpleSequence(t *testing.T) {
	c := New()
	// Simulate dilations at r=1, r=2, r=1
	c.Feed(0, 1)
	c.Feed(10, 1)
	c.Feed(20, 2)
	c.Feed(30, 1)
	c.SetTotalBits(40)

	tree := c.Build()
	if tree == nil {
		t.Fatal("expected non-nil tree")
	}
	// Root should have children at level 1
	if len(tree.Children) == 0 {
		t.Error("expected children at root")
	}
	t.Logf("tree size=%d depth=%d children=%d", tree.Size(), tree.Depth(), len(tree.Children))
}

func TestAnalyze(t *testing.T) {
	c := New()
	for i := 0; i < 50; i++ {
		r := uint32(i%4) + 1
		c.Feed(uint64(i*10), r)
	}
	c.SetTotalBits(500)

	tree := c.Build()
	rpt := Analyze(tree, len(c.events))

	if rpt.TotalDilations != len(c.events) {
		t.Errorf("expected %d dilations, got %d", len(c.events), rpt.TotalDilations)
	}
	if rpt.MaxDepth < 0 {
		t.Error("negative depth")
	}
	if rpt.TotalNodes < 1 {
		t.Error("expected at least root node")
	}
	if rpt.Balance < 0 || rpt.Balance > 1 {
		t.Errorf("balance out of range [0,1]: %f", rpt.Balance)
	}

	t.Logf("Report: %s", rpt.String())
	t.Logf("Depth distribution: %v", rpt.DepthDist)
}

func TestProseVsCodeVsNoise(t *testing.T) {
	// Compare tree structures for different content types
	proseTree, proseDil := BuildFromText(
		"The quick brown fox jumps over the lazy dog. " +
			"Pack my box with five dozen liquor jugs. " +
			"How vexingly quick daft zebras jump. " +
			"The five boxing wizards jump quickly at dawn.")

	codeTree, codeDil := BuildFromText(
		"func main() { fmt.Println(\"hello\"); os.Exit(0) }\n" +
			"type T struct { X int; Y string }\n" +
			"func (t T) F() int { return t.X + 1 }\n" +
			"var items = []string{\"a\", \"b\", \"c\"}")

	noiseTree, noiseDil := BuildFromText(
		"\x80\xff\x00\x7f\xaa\x55\xcc\x33\x99\x66\xff\x00\x80\x00\x11\x22" +
			"\xaa\xbb\xcc\xdd\xee\xff\x00\x11\x22\x33\x44\x55\x66\x77\x88\x99")

	proseRpt := Analyze(proseTree, proseDil)
	codeRpt := Analyze(codeTree, codeDil)
	noiseRpt := Analyze(noiseTree, noiseDil)

	t.Logf("Prose:  dilations=%d depth=%d balance=%.4f skew=%.4f entropy=%.4f",
		proseRpt.TotalDilations, proseRpt.MaxDepth, proseRpt.Balance,
		proseRpt.SkewRatio, proseRpt.DepthEntropy)
	t.Logf("Code:   dilations=%d depth=%d balance=%.4f skew=%.4f entropy=%.4f",
		codeRpt.TotalDilations, codeRpt.MaxDepth, codeRpt.Balance,
		codeRpt.SkewRatio, codeRpt.DepthEntropy)
	t.Logf("Noise:  dilations=%d depth=%d balance=%.4f skew=%.4f entropy=%.4f",
		noiseRpt.TotalDilations, noiseRpt.MaxDepth, noiseRpt.Balance,
		noiseRpt.SkewRatio, noiseRpt.DepthEntropy)
}

func TestTextToBits(t *testing.T) {
	bits := TextToBits("A") // 0x41 = 01000001
	expected := []uint8{0, 1, 0, 0, 0, 0, 0, 1}
	if len(bits) != len(expected) {
		t.Fatalf("expected %d bits, got %d", len(expected), len(bits))
	}
	for i := range expected {
		if bits[i] != expected[i] {
			t.Errorf("bit %d: expected %d, got %d", i, expected[i], bits[i])
		}
	}
}

func BenchmarkBuildFromText(b *testing.B) {
	text := "The quick brown fox jumps over the lazy dog. " +
		"Pack my box with five dozen liquor jugs. " +
		"How vexingly quick daft zebras jump. "
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = BuildFromText(text)
	}
}
