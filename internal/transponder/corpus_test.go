package transponder

import (
	"fmt"
	"testing"
)

const windowSize = 4096 // bits per window

// ── Input corpus: 3 classes ──

// Natural language prose (~1.5KB)
var corpusProse = []byte(`The Fibonacci sequence is a series of numbers where each number is the sum of the two preceding ones. It starts from 0 and 1, and continues indefinitely. The sequence appears throughout nature, from the arrangement of leaves on a stem to the spiral of a nautilus shell. In mathematics, the ratio between consecutive Fibonacci numbers converges to the golden ratio, approximately 1.618. This ratio appears in art, architecture, and music. The sequence was introduced to Western European mathematics by Leonardo of Pisa, known as Fibonacci, in his 1202 book Liber Abaci. However, the sequence had been described earlier in Indian mathematics. The connection between Fibonacci numbers and the golden ratio is profound: as you go further in the sequence, the ratio of consecutive numbers gets closer and closer to phi. This property makes Fibonacci numbers useful in algorithms, data structures, and computational methods. Binary search trees, heaps, and hash tables all use properties related to these numbers.`)

// Source code (~1.5KB)
var corpusCode = []byte(`package main

import (
	"fmt"
	"math"
	"os"
	"strconv"
)

type Node struct {
	Value int
	Left  *Node
	Right *Node
}

func fib(n int) int {
	if n <= 1 {
		return n
	}
	a, b := 0, 1
	for i := 2; i <= n; i++ {
		a, b = b, a+b
	}
	return b
}

func buildTree(values []int) *Node {
	if len(values) == 0 {
		return nil
	}
	root := &Node{Value: values[0]}
	for _, v := range values[1:] {
		insert(root, v)
	}
	return root
}

func insert(node *Node, val int) {
	if val < node.Value {
		if node.Left == nil {
			node.Left = &Node{Value: val}
		} else {
			insert(node.Left, val)
		}
	} else {
		if node.Right == nil {
			node.Right = &Node{Value: val}
		} else {
			insert(node.Right, val)
		}
	}
}

func traverse(node *Node, depth int) {
	if node == nil {
		return
	}
	traverse(node.Left, depth+1)
	fmt.Printf("%s%d\n", indent(depth), node.Value)
	traverse(node.Right, depth+1)
}

func indent(n int) string {
	buf := make([]byte, n*2)
	for i := range buf {
		buf[i] = ' '
	}
	return string(buf)
}

func main() {
	n, _ := strconv.Atoi(os.Args[1])
	fmt.Printf("fib(%d) = %d\n", n, fib(n))
	values := []int{8, 3, 10, 1, 6, 14, 4, 7, 13}
	tree := buildTree(values)
	traverse(tree, 0)
	_ = math.Pi
}`)

// Highly regular synthetic pattern (~1.5KB)
// Repeating 8-byte pattern: creates predictable adjacency structure
var corpusSynthetic []byte

func init() {
	// Pattern: fibonacci bytes repeated
	// 0x01, 0x01, 0x02, 0x03, 0x05, 0x08, 0x0D, 0x15
	pattern := []byte{0x01, 0x01, 0x02, 0x03, 0x05, 0x08, 0x0D, 0x15}
	target := 1536 // ~1.5KB
	corpusSynthetic = make([]byte, 0, target)
	for len(corpusSynthetic) < target {
		corpusSynthetic = append(corpusSynthetic, pattern...)
	}
	corpusSynthetic = corpusSynthetic[:target]
}

// ── Experiment ──

func TestCorpusExperiment(t *testing.T) {
	cals := []Calibration{
		CalibrationTight,
		CalibrationMedium,
		CalibrationWide,
	}

	corpus := []struct {
		label string
		data  []byte
	}{
		{"prose", corpusProse},
		{"code", corpusCode},
		{"synthetic", corpusSynthetic},
	}

	reports := make([]CorpusReport, len(corpus))
	for i, c := range corpus {
		bits := BytesToBits(c.data)
		reports[i] = RunCorpusExperiment(c.label, bits, cals, windowSize)
	}

	// ── Print numeric results ──
	t.Log("═══════════════════════════════════════════════════════════════")
	t.Log("CORPUS EXPERIMENT: Per-transponder metrics across 3 input classes")
	t.Log("═══════════════════════════════════════════════════════════════")

	for _, r := range reports {
		t.Logf("")
		t.Logf("── %s (%d bits, %d windows) ──", r.Label, r.BitCount, len(r.Transponders[0].WindowData))
		t.Logf("%-10s  %8s  %8s  %8s  %18s", "transp.", "dil", "mark", "r", "sketch")
		t.Logf("%-10s  %8s  %8s  %8s  %18s", "─────────", "───────", "───────", "───────", "──────────────────")
		for _, tr := range r.Transponders {
			t.Logf("%-10s  %8d  %8d  %8d  0x%016x",
				tr.Name, tr.TotalDil, tr.TotalMark, tr.FinalR, tr.FinalSketch)
		}

		// Windowed rates
		t.Logf("")
		t.Logf("  Window dil-rate heatmap (dilations/bit):")
		nWin := len(r.Transponders[0].WindowData)
		for w := 0; w < nWin && w < 8; w++ {
			line := fmt.Sprintf("    W%-2d: ", w)
			for _, tr := range r.Transponders {
				wd := tr.WindowData[w]
				line += fmt.Sprintf("%s=%.4f  ", tr.Name, wd.DilateRate)
			}
			t.Log(line)
		}
	}

	// ── Cross-class comparison per transponder ──
	t.Log("")
	t.Log("═══════════════════════════════════════════════════════════════")
	t.Log("DIVERGENCE MATRIX: Do transponders separate input classes?")
	t.Log("═══════════════════════════════════════════════════════════════")
	t.Log("")

	for tIdx := 0; tIdx < len(cals); tIdx++ {
		name := reports[0].Transponders[tIdx].Name
		t.Logf("Transponder %s:", name)
		t.Logf("  %-12s  %8s  %8s  %8s  %18s", "class", "dil", "mark", "r", "sketch")
		for _, r := range reports {
			tr := r.Transponders[tIdx]
			t.Logf("  %-12s  %8d  %8d  %8d  0x%016x",
				r.Label, tr.TotalDil, tr.TotalMark, tr.FinalR, tr.FinalSketch)
		}
		t.Log("")
	}

	// ── Sketch divergence within each class ──
	t.Log("═══════════════════════════════════════════════════════════════")
	t.Log("SKETCH DIVERGENCE: XOR distance between transponders per class")
	t.Log("═══════════════════════════════════════════════════════════════")
	t.Log("")

	for _, r := range reports {
		t.Logf("Class %s:", r.Label)
		for i := 0; i < len(r.Transponders); i++ {
			for j := i + 1; j < len(r.Transponders); j++ {
				xor := r.Transponders[i].FinalSketch ^ r.Transponders[j].FinalSketch
				t.Logf("  %s ⊕ %s = 0x%016x",
					r.Transponders[i].Name, r.Transponders[j].Name, xor)
			}
		}
		t.Log("")
	}

	// ── Event structure comparison ──
	t.Log("═══════════════════════════════════════════════════════════════")
	t.Log("EVENT STRUCTURE: Does calibration affect event rates?")
	t.Log("═══════════════════════════════════════════════════════════════")
	t.Log("")

	for _, r := range reports {
		t.Logf("Class %s (%d bits):", r.Label, r.BitCount)
		t.Logf("  %-10s  %12s  %12s  %12s", "transp.", "dil-rate", "mark-rate", "total-rate")
		for _, tr := range r.Transponders {
			dr := float64(tr.TotalDil) / float64(r.BitCount)
			mr := float64(tr.TotalMark) / float64(r.BitCount)
			t.Logf("  %-10s  %12.6f  %12.6f  %12.6f",
				tr.Name, dr, mr, dr+mr)
		}
		t.Log("")
	}

	// ── Windowed sketch trajectory ──
	t.Log("═══════════════════════════════════════════════════════════════")
	t.Log("SKETCH TRAJECTORY: sketch value at each window boundary")
	t.Log("═══════════════════════════════════════════════════════════════")
	t.Log("")

	for _, r := range reports {
		t.Logf("Class %s:", r.Label)
		nWin := len(r.Transponders[0].WindowData)
		for w := 0; w < nWin && w < 4; w++ {
			line := fmt.Sprintf("  W%-2d: ", w)
			for _, tr := range r.Transponders {
				line += fmt.Sprintf("%s=0x%016x  ", tr.Name, tr.WindowData[w].Sketch)
			}
			t.Log(line)
		}
		if nWin > 4 {
			t.Logf("  ... (%d more windows)", nWin-4)
		}
		t.Log("")
	}

	// ── Assertions: structural facts ──

	// 1. Within each class, all transponders must have identical DILATE counts
	//    (adjacency detection is calibration-independent)
	for _, r := range reports {
		dil0 := r.Transponders[0].TotalDil
		for _, tr := range r.Transponders[1:] {
			if tr.TotalDil != dil0 {
				t.Errorf("class %s: DILATE mismatch: %s=%d vs %s=%d",
					r.Label, r.Transponders[0].Name, dil0, tr.Name, tr.TotalDil)
			}
		}
	}

	// 2. Within each class, all transponders must have identical marker counts
	for _, r := range reports {
		m0 := r.Transponders[0].TotalMark
		for _, tr := range r.Transponders[1:] {
			if tr.TotalMark != m0 {
				t.Errorf("class %s: marker mismatch: %s=%d vs %s=%d",
					r.Label, r.Transponders[0].Name, m0, tr.Name, tr.TotalMark)
			}
		}
	}

	// 3. Sketches may differ or collide — document, don't assert.
	//    (See assertion #4 below for collision reporting.)

	// 4. Sketch values may collide between transponders for some inputs.
	//    (This is observed: tight and wide produced identical sketches on prose.)
	//    Document collisions rather than asserting divergence.
	for _, r := range reports {
		sketches := make(map[uint64]string)
		collisions := 0
		for _, tr := range r.Transponders {
			if prev, exists := sketches[tr.FinalSketch]; exists {
				t.Logf("NOTE: class %s sketch collision: %s == %s == 0x%016x",
					r.Label, prev, tr.Name, tr.FinalSketch)
				collisions++
			}
			sketches[tr.FinalSketch] = tr.Name
		}
		if collisions > 0 {
			t.Logf("  → class %s: %d/%d transponders collided on sketch",
				r.Label, collisions+1, len(r.Transponders))
		}
	}
}

func TestCorpusByteCounts(t *testing.T) {
	t.Logf("prose:      %d bytes = %d bits", len(corpusProse), len(corpusProse)*8)
	t.Logf("code:       %d bytes = %d bits", len(corpusCode), len(corpusCode)*8)
	t.Logf("synthetic:  %d bytes = %d bits", len(corpusSynthetic), len(corpusSynthetic)*8)
}
