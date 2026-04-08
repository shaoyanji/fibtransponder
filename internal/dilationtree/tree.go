package dilationtree

import (
	"fmt"
	"math"

	"github.com/shaoyanji/fibtransponder/internal/fsvm"
)

// DilationTree: hierarchical representation from temporal burst structure.
//
// Thesis: the timing of DILATE events across a bitstream carries
// hierarchical information. Bursts of rapid dilations (short inter-event
// intervals) indicate local structural violations; gaps between bursts
// indicate stable regions. A tree built from recursive burst detection
// at multiple scales captures semantic hierarchy — short bursts correspond
// to local features (words, operators), longer bursts to phrases, sustained
// bursts to discourse-level shifts.
//
// This is analogous to Mallat's scattering transform: cascading
// multi-scale grouping produces stable representations invariant to
// small temporal deformations. The dilation event train IS a boolean
// scattering transform when organized hierarchically by burst structure.

// Event is a single dilation event with timing info.
type Event struct {
	BitIndex uint64
	R        uint32
}

// Collector gathers dilation events.
type Collector struct {
	events    []Event
	totalBits uint64
}

// New creates a collector.
func New() *Collector { return &Collector{} }

// Feed records a dilation event.
func (c *Collector) Feed(bitIndex uint64, r uint32) {
	c.events = append(c.events, Event{BitIndex: bitIndex, R: r})
}

// SetTotalBits sets the total bit count.
func (c *Collector) SetTotalBits(n uint64) { c.totalBits = n }

// BuildFromText runs the FSVM on text and builds a dilation tree.
func BuildFromText(text string) (*Node, int) {
	c := New()
	fs := fsvm.New()
	bits := TextToBits(text)

	for i, b := range bits {
		var evs []fsvm.Event
		fs, evs = fsvm.Step(fs, b)
		for _, ev := range evs {
			if ev.Kind == fsvm.EventDilate {
				c.Feed(uint64(i), uint32(ev.Payload))
			}
		}
	}
	c.SetTotalBits(uint64(len(bits)))
	return c.Build(), len(c.events)
}

// Node represents a temporal grouping of dilation events.
type Node struct {
	StartBit uint64  // first event's bit index
	EndBit   uint64  // last event's bit index
	Dilations int    // number of dilations in this node
	Density  float64 // dilations per bit (local rate)
	Scale    float64 // characteristic timescale of this group (mean inter-event interval)
	Children []Node  // sub-groups detected at finer temporal resolution
}

// Depth returns max nesting depth.
func (n *Node) Depth() int {
	if len(n.Children) == 0 {
		return 0
	}
	maxD := 0
	for i := range n.Children {
		d := n.Children[i].Depth()
		if d > maxD {
			maxD = d
		}
	}
	return 1 + maxD
}

// Span returns the bit range covered.
func (n *Node) Span() uint64 {
	if n.EndBit <= n.StartBit {
		return 0
	}
	return n.EndBit - n.StartBit
}

// Balance returns a balance measure [0,1].
// 1.0 = all children have similar depth.
func (n *Node) Balance() float64 {
	if len(n.Children) <= 1 {
		return 1.0
	}
	minD, maxD := n.Children[0].Depth(), n.Children[0].Depth()
	for i := range n.Children {
		d := n.Children[i].Depth()
		if d < minD {
			minD = d
		}
		if d > maxD {
			maxD = d
		}
	}
	if maxD == 0 {
		return 1.0
	}
	return float64(minD+1) / float64(maxD+1)
}

// Size returns total node count in subtree.
func (n *Node) Size() int {
	s := 1
	for i := range n.Children {
		s += n.Children[i].Size()
	}
	return s
}

// LeafCount returns the number of leaf nodes.
func (n *Node) LeafCount() int {
	if len(n.Children) == 0 {
		return 1
	}
	count := 0
	for i := range n.Children {
		count += n.Children[i].LeafCount()
	}
	return count
}

// Build constructs the tree from collected events using recursive
// temporal burst detection.
//
// Algorithm:
// 1. Compute inter-event intervals (IEIs): delta[i] = events[i+1].BitIndex - events[i].BitIndex
// 2. Compute mean and std of IEIs
// 3. Events with IEI < (mean - std/2) form a "burst" — group them
// 4. Recursively process each burst at a finer scale
// 5. Non-burst events become siblings at the current level
func (c *Collector) Build() *Node {
	n := len(c.events)
	if n == 0 {
		return &Node{EndBit: c.totalBits}
	}

	// Compute IEIs
	ieis := make([]float64, n-1)
	var sumIEI float64
	for i := 0; i < n-1; i++ {
		ieis[i] = float64(c.events[i+1].BitIndex - c.events[i].BitIndex)
		sumIEI += ieis[i]
	}

	meanIEI := 0.0
	varieis := 0.0
	if n > 1 {
		meanIEI = sumIEI / float64(n-1)
		for _, v := range ieis {
			diff := v - meanIEI
			varieis += diff * diff
		}
		varieis /= float64(n - 1)
		if varieis < 0 {
			varieis = 0
		}
	}
	stdIEI := math.Sqrt(varieis)

	// Burst threshold: IEI < mean - k*std is "fast" (within a burst)
	// Use k=0.5 for moderate sensitivity
	k := 0.5
	threshold := meanIEI - k*stdIEI
	if threshold < 1 {
		threshold = 1
	}

	// Group events into bursts
	type group struct {
		startIdx int
		endIdx   int // inclusive
		isBurst  bool
	}

	var groups []group
	inBurst := false
	gStart := 0

	for i := 0; i < n-1; i++ {
		if ieis[i] <= threshold {
			if !inBurst {
				gStart = i
				inBurst = true
			}
		} else if inBurst {
			groups = append(groups, group{gStart, i, true})
			inBurst = false
		}
	}
	// Close any open burst
	if inBurst && gStart < n-1 {
		groups = append(groups, group{gStart, n - 1, true})
	}

	// Build non-burst segments between bursts
	if len(groups) == 0 {
		// No bursts detected — single node with all events
		return &Node{
			StartBit:  c.events[0].BitIndex,
			EndBit:    c.events[n-1].BitIndex,
			Dilations: n,
			Density:   float64(n) / float64(c.totalBits),
			Scale:     meanIEI,
		}
	}

	// Build node
	root := &Node{
		StartBit: c.events[0].BitIndex,
		EndBit:   c.events[n-1].BitIndex,
	}

	var children []Node

	// Process segments
	lastEnd := 0
	for _, g := range groups {
		// Add non-burst segment before this burst (if any)
		if g.startIdx > lastEnd {
			child := makeLeaf(c.events[lastEnd:g.startIdx], c.totalBits)
			children = append(children, child)
		}
		// Make burst group
		burstEvents := c.events[g.startIdx : g.endIdx+1]
		if g.isBurst {
			subc := &Collector{events: burstEvents, totalBits: c.totalBits}
			child := subc.buildSubTree()
			children = append(children, child)
		}
		lastEnd = g.endIdx + 1
	}
	// Remaining non-burst segment
	if lastEnd < n {
		child := makeLeaf(c.events[lastEnd:n], c.totalBits)
		children = append(children, child)
	}

	root.Children = children
	root.Dilations = n
	root.Density = float64(n) / float64(c.totalBits)
	root.Scale = meanIEI

	return root
}

// buildSubTree recursively builds a subtree from a burst group.
func (c *Collector) buildSubTree() Node {
	n := len(c.events)
	if n <= 1 {
		if n == 0 {
			return Node{}
		}
		return Node{
			StartBit:  c.events[0].BitIndex,
			EndBit:    c.events[0].BitIndex,
			Dilations: 1,
		}
	}

	ieis := make([]float64, n-1)
	var sumIEI float64
	for i := 0; i < n-1; i++ {
		ieis[i] = float64(c.events[i+1].BitIndex - c.events[i].BitIndex)
		sumIEI += ieis[i]
	}
	meanIEI := sumIEI / float64(n-1)

	varieis := 0.0
	for _, v := range ieis {
		diff := v - meanIEI
		varieis += diff * diff
	}
	varieis /= float64(n - 1)
	if varieis < 0 {
		varieis = 0
	}
	stdIEI := math.Sqrt(varieis)

	// Tighter threshold for sub-level
	k := 0.3
	threshold := meanIEI - k*stdIEI
	if threshold < 1 {
		threshold = 1
	}

	// Check if there's any variability to detect
	if stdIEI < meanIEI*0.1 || n < 4 {
		// Not enough variation to subdivide — make leaf
		return makeLeaf(c.events, c.totalBits)
	}

	// Find sub-bursts
	type group struct {
		startIdx int
		endIdx   int
	}

	var subGroups []group
	inBurst := false
	gStart := 0

	for i := 0; i < n-1; i++ {
		if ieis[i] <= threshold {
			if !inBurst {
				gStart = i
				inBurst = true
			}
		} else if inBurst {
			subGroups = append(subGroups, group{gStart, i})
			inBurst = false
		}
	}
	if inBurst {
		subGroups = append(subGroups, group{gStart, n - 1})
	}

	node := Node{
		StartBit:  c.events[0].BitIndex,
		EndBit:    c.events[n-1].BitIndex,
		Dilations: n,
		Density:   float64(n) / float64(c.totalBits),
		Scale:     meanIEI,
	}

	if len(subGroups) <= 1 {
		// Not enough sub-structure detected
		return node
	}

	var children []Node
	lastEnd := 0
	for _, sg := range subGroups {
		if sg.startIdx > lastEnd {
			child := makeLeaf(c.events[lastEnd:sg.startIdx], c.totalBits)
			children = append(children, child)
		}
		subc := &Collector{events: c.events[sg.startIdx : sg.endIdx+1], totalBits: c.totalBits}
		subNode := subc.buildSubTree()
		children = append(children, subNode)
		lastEnd = sg.endIdx + 1
	}
	if lastEnd < n {
		child := makeLeaf(c.events[lastEnd:n], c.totalBits)
		children = append(children, child)
	}

	node.Children = children
	return node
}

func makeLeaf(events []Event, totalBits uint64) Node {
	n := len(events)
	if n == 0 {
		return Node{}
	}
	start := events[0].BitIndex
	end := events[n-1].BitIndex

	var meanInterval float64
	if n > 1 {
		var sum float64
		for i := 1; i < n; i++ {
			sum += float64(events[i].BitIndex - events[i-1].BitIndex)
		}
		meanInterval = sum / float64(n-1)
	}

	return Node{
		StartBit:  start,
		EndBit:    end,
		Dilations: n,
		Density:   float64(n) / float64(totalBits),
		Scale:     meanInterval,
	}
}

// Report captures structural properties.
type Report struct {
	TotalDilations int
	MaxDepth       int
	TotalNodes     int
	Balance        float64
	LeafCount      int
	SkewRatio      float64 // maxDepth / leafCount; high = pathological
	DepthEntropy   float64 // Shannon entropy of depth distribution
	DepthDist      []int   // node count per depth level
	MeanScale      float64 // mean characteristic timescale across nodes
}

// Analyze generates a report.
func Analyze(tree *Node, totalDilations int) Report {
	if tree == nil {
		return Report{}
	}

	var leaves int
	depthCount := make(map[int]int)
	var totalScale float64
	var scaleCount int

	var walk func(*Node, int)
	walk = func(n *Node, d int) {
		depthCount[d]++
		totalScale += n.Scale
		scaleCount++
		if len(n.Children) == 0 {
			leaves++
			return
		}
		for i := range n.Children {
			walk(&n.Children[i], d+1)
		}
	}
	walk(tree, 0)

	depth := tree.Depth()
	skew := 0.0
	if leaves > 0 {
		skew = float64(depth) / float64(leaves)
	}

	maxDepth := 0
	for d := range depthCount {
		if d > maxDepth {
			maxDepth = d
		}
	}
	depthDist := make([]int, maxDepth+1)
	for d, c := range depthCount {
		depthDist[d] = c
	}

	var entropy float64
	total := 0
	for _, c := range depthCount {
		total += c
	}
	if total > 0 {
		for _, c := range depthCount {
			p := float64(c) / float64(total)
			if p > 0 {
				entropy -= p * math.Log2(p)
			}
		}
	}

	meanScale := 0.0
	if scaleCount > 0 {
		meanScale = totalScale / float64(scaleCount)
	}

	return Report{
		TotalDilations: totalDilations,
		MaxDepth:       depth,
		TotalNodes:     tree.Size(),
		Balance:        tree.Balance(),
		LeafCount:      leaves,
		SkewRatio:      skew,
		DepthEntropy:   entropy,
		DepthDist:      depthDist,
		MeanScale:      meanScale,
	}
}

// TextToBits converts text to MSB-first bits.
func TextToBits(s string) []uint8 {
	bits := make([]uint8, 0, len(s)*8)
	for _, c := range s {
		b := byte(c)
		for i := 7; i >= 0; i-- {
			bits = append(bits, (b>>uint(i))&1)
		}
	}
	return bits
}

// String formats a report.
func (r Report) String() string {
	return fmt.Sprintf(
		"dilations=%d depth=%d nodes=%d leaves=%d balance=%.4f skew=%.4f entropy=%.4f scale=%.1f",
		r.TotalDilations, r.MaxDepth, r.TotalNodes, r.LeafCount,
		r.Balance, r.SkewRatio, r.DepthEntropy, r.MeanScale,
	)
}
