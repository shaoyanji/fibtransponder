package wasmapi

import (
	"encoding/json"

	"github.com/shaoyanji/fibtransponder/internal/dilationtree"
)

// treeAPI serves the burst-hierarchy demo. It calls the real
// dilationtree.BuildFromText and Analyze, so the numbers match
// `go run ./cmd/dilation_tree -json` exactly.
type treeAPI struct{}

type treeBuildArgs struct {
	Text     string `json:"text"`
	MaxNodes int    `json:"maxNodes"`
	MaxDepth int    `json:"maxDepth"`
}

type treeNodeOut struct {
	StartBit  U64           `json:"startBit"`
	EndBit    U64           `json:"endBit"`
	Dilations int           `json:"dilations"`
	Density   float64       `json:"density"`
	Scale     float64       `json:"scale"`
	Children  []treeNodeOut `json:"children,omitempty"`
	Depth     int           `json:"depth"`
	SpanBits  U64           `json:"spanBits"`
	Truncated bool          `json:"truncated,omitempty"`
}

type treeBuildOut struct {
	TotalBits      int          `json:"totalBits"`
	TotalDilations int          `json:"totalDilations"`
	MaxDepth       int          `json:"maxDepth"`
	TotalNodes     int          `json:"totalNodes"`
	LeafCount      int          `json:"leafCount"`
	Balance        float64      `json:"balance"`
	SkewRatio      float64      `json:"skewRatio"`
	DepthEntropy   float64      `json:"depthEntropy"`
	MeanScale      float64      `json:"meanScale"`
	DepthDist      []int        `json:"depthDist"`
	Root           *treeNodeOut `json:"root"`
	// Report is the upstream String() form, so the page can quote the CLI
	// verbatim instead of reformatting the numbers its own way.
	Report    string `json:"report"`
	Truncated bool   `json:"truncated"`
	NonASCII  string `json:"nonAsciiRune,omitempty"`
}

func (*treeAPI) build(raw json.RawMessage) (result, error) {
	var a treeBuildArgs
	if err := json.Unmarshal(raw, &a); err != nil {
		return result{}, errf(CodeBadArgs, "tree.build: %v", err)
	}

	text := a.Text
	if text == "" {
		text = defaultText
	}
	text, ascii, err := capText(text)
	if err != nil {
		return result{}, err
	}

	tree, dilCount := dilationtree.BuildFromText(text)
	if tree == nil {
		return result{Data: treeBuildOut{Report: "", NonASCII: nonASCIIIfAny(text, ascii)}}, nil
	}

	maxNodes := clampInt(defaultIfZero(a.MaxNodes, MaxTreeNodes), 1, MaxTreeNodes)
	maxDepth := clampInt(defaultIfZero(a.MaxDepth, 32), 1, 64)

	truncated := false
	nodeCount := 0
	var convert func(n *dilationtree.Node, depth int) *treeNodeOut
	convert = func(n *dilationtree.Node, depth int) *treeNodeOut {
		if nodeCount >= maxNodes || depth > maxDepth {
			truncated = true
			return nil
		}
		nodeCount++
		out := &treeNodeOut{
			StartBit:  U(n.StartBit),
			EndBit:    U(n.EndBit),
			Dilations: n.Dilations,
			Density:   safeFloat(n.Density),
			Scale:     safeFloat(n.Scale),
			Depth:     depth,
			SpanBits:  U(n.Span()),
		}
		for i := range n.Children {
			if child := convert(&n.Children[i], depth+1); child != nil {
				out.Children = append(out.Children, *child)
			} else {
				truncated = true
			}
		}
		return out
	}

	// Serialize the original tree; Analyze walks the untouched original so the
	// report metrics are never affected by the display cap.
	root := convert(tree, 0)
	rep := dilationtree.Analyze(tree, dilCount)

	out := treeBuildOut{
		// TextToBits emits 8 bits per rune, so the bit count is runeCount*8,
		// not byteCount*8. These differ for any non-ASCII input.
		TotalBits:      utf8RuneCount(text) * 8,
		TotalDilations: rep.TotalDilations,
		MaxDepth:       rep.MaxDepth,
		TotalNodes:     rep.TotalNodes,
		LeafCount:      rep.LeafCount,
		Balance:        safeFloat(rep.Balance),
		SkewRatio:      safeFloat(rep.SkewRatio),
		DepthEntropy:   safeFloat(rep.DepthEntropy),
		MeanScale:      safeFloat(rep.MeanScale),
		DepthDist:      rep.DepthDist,
		Root:           root,
		Report:         rep.String(),
		Truncated:      truncated,
		NonASCII:       nonASCIIIfAny(text, ascii),
	}

	var res result
	if !ascii {
		res = res.warn("dilationtree.TextToBits emits 8 bits per rune and truncates each code "+
			"point to its low byte, so this result uses %d bytes for %d runes; the "+
			"FSVM demos on the same text use full UTF-8 and will disagree",
			len([]byte(text)), utf8RuneCount(text))
	}
	return res.with(out), nil
}

func nonASCIIIfAny(text string, ascii bool) string {
	if ascii {
		return ""
	}
	return describeNonASCII(text)
}
