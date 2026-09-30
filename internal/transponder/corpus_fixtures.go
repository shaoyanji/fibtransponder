package transponder

// Corpus fixtures for the published reports.
//
// These live in the package rather than in a test file because REPORT_CORPUS.md
// and REPORT_STRUCTURAL.md are published artifacts derived from them, and the
// portfolio site reproduces those tables live via internal/wasmapi. A fixture
// that lives only in a test file cannot be the input to a published number
// without risking silent drift.
//
// Nothing here is synthetic-looking on purpose: prose and source code are the
// two classes the width axis was claimed to separate, and the repeating
// Fibonacci byte pattern is the control that shows a regular input vanishing
// entirely at wider adjacency windows.

// CorpusProse is natural-language text: the Fibonacci essay.
var CorpusProse = []byte(`The Fibonacci sequence is a series of numbers where each number is the sum of the two preceding ones. It starts from 0 and 1, and continues indefinitely. The sequence appears throughout nature, from the arrangement of leaves on a stem to the spiral of a nautilus shell. In mathematics, the ratio between consecutive Fibonacci numbers converges to the golden ratio, approximately 1.618. This ratio appears in art, architecture, and music. The sequence was introduced to Western European mathematics by Leonardo of Pisa, known as Fibonacci, in his 1202 book Liber Abaci. However, the sequence had been described earlier in Indian mathematics. The connection between Fibonacci numbers and the golden ratio is profound: as you go further in the sequence, the ratio of consecutive numbers gets closer and closer to phi. This property makes Fibonacci numbers useful in algorithms, data structures, and computational methods. Binary search trees, heaps, and hash tables all use properties related to these numbers.`)

// CorpusCode is Go source exercising the same tree operations as the prose.
var CorpusCode = []byte(`package main

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

// CorpusSynthetic repeats the first eight Fibonacci bytes, 1536 bytes total.
//
// The point of this control is that it never produces three or more
// consecutive 1-bits, so its dilation rate goes to exactly zero at w>=2. That
// is what shows a wider adjacency window suppressing a regular signal rather
// than merely being less sensitive to it.
var CorpusSynthetic = buildSyntheticCorpus()

func buildSyntheticCorpus() []byte {
	pattern := []byte{0x01, 0x01, 0x02, 0x03, 0x05, 0x08, 0x0D, 0x15}
	const target = 1536
	out := make([]byte, 0, target+len(pattern))
	for len(out) < target {
		out = append(out, pattern...)
	}
	return out[:target]
}

// CorpusClass is one labelled fixture.
type CorpusClass struct {
	Label string
	Data  []byte
}

// CorpusClasses returns the three published classes in report order.
func CorpusClasses() []CorpusClass {
	return []CorpusClass{
		{"prose", CorpusProse},
		{"code", CorpusCode},
		{"synthetic", CorpusSynthetic},
	}
}
