package main

// cmd/dilation_tree: build and analyze dilation history trees from
// text input. Trees capture hierarchical structure in the input stream.
//
// Usage:
//	echo "hello world" | dilation_tree
//	cat corpus.txt | dilation_tree -json > tree.json
//	dilation_tree -compare=prose.txt,code.txt,noise.txt

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"os"
	"text/tabwriter"

	"github.com/shaoyanji/fibtransponder/internal/dilationtree"
)

func main() {
	jsonOut := flag.Bool("json", false, "output as JSON")
	compare := flag.String("compare", "", "CSV of files to compare")
	flag.Parse()

	if *compare != "" {
		runComparison(*compare, *jsonOut)
		return
	}

	reader := bufio.NewReader(os.Stdin)
	input, err := io.ReadAll(reader)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error reading stdin: %v\n", err)
		os.Exit(1)
	}

	tree, dilCount := dilationtree.BuildFromText(string(input))
	rpt := dilationtree.Analyze(tree, dilCount)

	if *jsonOut {
		fmt.Printf(`{"total_bits":%d,"dilations":%d,"depth":%d,"nodes":%d,"leaves":%d,"balance":%.4f,"skew":%.4f,"entropy":%.4f,"depth_dist":[`,
			len(input)*8, rpt.TotalDilations, rpt.MaxDepth, rpt.TotalNodes,
			rpt.LeafCount, rpt.Balance, rpt.SkewRatio, rpt.DepthEntropy)
		for i, d := range rpt.DepthDist {
			if i > 0 {
				fmt.Print(",")
			}
			fmt.Print(d)
		}
		fmt.Println("]}")
		return
	}

	fmt.Println("=== Dilation Tree Structure ===")
	fmt.Printf("  Total bits:      %d\n", len(input)*8)
	fmt.Printf("  Dilations:       %d\n", rpt.TotalDilations)
	fmt.Printf("  Max depth:       %d\n", rpt.MaxDepth)
	fmt.Printf("  Total nodes:     %d\n", rpt.TotalNodes)
	fmt.Printf("  Leaf count:      %d\n", rpt.LeafCount)
	fmt.Printf("  Balance:         %.4f\n", rpt.Balance)
	fmt.Printf("  Skew ratio:      %.4f\n", rpt.SkewRatio)
	fmt.Printf("  Depth entropy:   %.4f\n", rpt.DepthEntropy)

	if len(rpt.DepthDist) > 0 {
		fmt.Println("\n=== Depth Distribution ===")
		tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(tw, "  Depth\tNodes\tBar")
		fmt.Fprintln(tw, "  -----\t-----\t---")
		maxCount := 0
		for _, c := range rpt.DepthDist {
			if c > maxCount {
				maxCount = c
			}
		}
		for d, c := range rpt.DepthDist {
			bar := ""
			if maxCount > 0 {
				n := c * 50 / maxCount
				for j := 0; j < n; j++ {
					bar += "\u2588"
				}
			}
			fmt.Fprintf(tw, "  %d\t%d\t%s\n", d, c, bar)
		}
		tw.Flush()
	}

	fmt.Println()
}

func runComparison(files string, jsonOut bool) {
	parts := splitCSV(files)

	tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "  File\tDilations\tDepth\tNodes\tBalance\tSkew\tEntropy")
	fmt.Fprintln(tw, "  ----\t---------\t-----\t-----\t-------\t----\t-------")

	for _, f := range parts {
		data, err := os.ReadFile(f)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error reading %s: %v\n", f, err)
			continue
		}

		tree, dilCount := dilationtree.BuildFromText(string(data))
		rpt := dilationtree.Analyze(tree, dilCount)

		if jsonOut {
			fmt.Printf(`{"file":"%s","dilations":%d,"depth":%d,"nodes":%d,"balance":%.4f,"skew":%.4f,"entropy":%.4f}`,
				f, rpt.TotalDilations, rpt.MaxDepth, rpt.TotalNodes,
				rpt.Balance, rpt.SkewRatio, rpt.DepthEntropy)
			fmt.Println()
		} else {
			fmt.Fprintf(tw, "  %s\t%d\t%d\t%d\t%.4f\t%.4f\t%.4f\n",
				f, rpt.TotalDilations, rpt.MaxDepth, rpt.TotalNodes,
				rpt.Balance, rpt.SkewRatio, rpt.DepthEntropy)
		}
	}

	if !jsonOut {
		tw.Flush()
	}
}

func splitCSV(s string) []string {
	var parts []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == ',' {
			parts = append(parts, s[start:i])
			start = i + 1
		}
	}
	parts = append(parts, s[start:])
	return parts
}
