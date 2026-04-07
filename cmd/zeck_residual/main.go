package main

// cmd/zeck_residual: analyze Zeckendorf residual profiles from stdin input.
//
// Usage:
//
//	echo "hello world" | zeck_residual
//	echo "hello world" | zeck_residual -window=32
//	cat corpus.txt | zeck_residual
//
// Outputs residual statistics and multi-scale residual profile
// for the input bitstream.

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"os"
	"text/tabwriter"

	"github.com/shaoyanji/fibtransponder/internal/zeckendorf"
)

func main() {
	windowSize := flag.Int("window", 128, "sliding window size")
	profile := flag.Bool("profile", true, "compute multi-scale profile")
	jsonOut := flag.Bool("json", false, "output as JSON")
	corpus := flag.String("corpus", "", "compare two files: -corpus=file1,file2")
	flag.Parse()

	var bits []uint8

	if *corpus != "" {
		// Compare two files
		fmt.Fprintln(os.Stderr, "comparing corpus files is not yet implemented; use stdin")
		os.Exit(1)
	}

	reader := bufio.NewReader(os.Stdin)
	input, err := io.ReadAll(reader)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error reading stdin: %v\n", err)
		os.Exit(1)
	}

	// Convert bytes to bits (MSB first)
	bits = make([]uint8, 0, len(input)*8)
	for _, b := range input {
		for i := 7; i >= 0; i-- {
			bits = append(bits, (b>>uint(i))&1)
		}
	}

	if len(bits) < 2 {
		fmt.Fprintf(os.Stderr, "need at least 2 bits, got %d\n", len(bits))
		os.Exit(1)
	}

	// Single residual for entire stream
	globalR := zeckendorf.Residual(bits)

	// Sliding window
	wr := zeckendorf.ResidualWindow(bits, *windowSize)

	if *jsonOut {
		fmt.Printf(`{"total_bits":%d,"global_residual":%.6f,"window_size":%d,"windows":%d,"min":%.6f,"max":%.6f,"mean":%.6f,"stddev":%.6f`,
			len(bits), globalR, *windowSize, wr.Windows, wr.Min, wr.Max, wr.Global, wr.StdDev)
		if *profile {
			sizes := []int{8, 16, 32, 64, 128, 256, 512, 1024}
			prof := zeckendorf.Profile(bits, sizes)
			fmt.Print(`,"profile":[`)
			for i, p := range prof {
				if i > 0 {
					fmt.Print(",")
				}
				fmt.Printf(`{"window":%d,"mean":%.6f,"min":%.6f,"max":%.6f,"stddev":%.6f}`,
					p.WindowSize, p.Mean, p.Min, p.Max, p.StdDev)
			}
			fmt.Print(`]`)
		}
		fmt.Println("}")
		return
	}

	fmt.Printf("Total bits: %d\n", len(bits))
	fmt.Printf("Global residual: %.6f\n\n", globalR)

	// --- Sliding Window ---
	fmt.Println("=== Sliding Residual (window=", *windowSize, ") ===")
	fmt.Printf("  Windows: %d\n", wr.Windows)
	fmt.Printf("  Min:     %.6f\n", wr.Min)
	fmt.Printf("  Max:     %.6f\n", wr.Max)
	fmt.Printf("  Mean:    %.6f\n", wr.Global)
	fmt.Printf("  StdDev:  %.6f\n", wr.StdDev)

	// Distribution histogram
	if wr.Windows > 1 {
		fmt.Println("\n=== Residual Distribution ===")
		bins := make([]int, 10)
		for _, r := range wr.Means {
			bin := int(r * 10)
			if bin >= 10 {
				bin = 9
			}
			bins[bin]++
		}
		maxCount := 0
		for _, c := range bins {
			if c > maxCount {
				maxCount = c
			}
		}
		for i, c := range bins {
			low := float64(i) * 0.1
			high := float64(i+1) * 0.1
			bar := ""
			if maxCount > 0 {
				n := c * 50 / maxCount
				for j := 0; j < n; j++ {
					bar += "\u2588"
				}
			}
			fmt.Printf("  %.1f-%.1f %3d %s\n", low, high, c, bar)
		}
	}

	if *profile {
		prof := zeckendorf.Profile(bits, []int{8, 16, 32, 64, 128, 256, 512, 1024})

		fmt.Println("\n=== Multi-Scale Residual Profile ===")
		tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(tw, "  Window\tMean\tMin\tMax\tStdDev\tRange")
		fmt.Fprintln(tw, "  ------\t----\t---\t---\t------\t-----")
		for _, p := range prof {
			rng := p.Max - p.Min
			fmt.Fprintf(tw, "  %d\t%.4f\t%.4f\t%.4f\t%.4f\t%.4f\n",
				p.WindowSize, p.Mean, p.Min, p.Max, p.StdDev, rng)
		}
		tw.Flush()

		// Scale gradient: how fast does residual change with window size?
		if len(prof) >= 2 {
			fmt.Println("\n=== Scale Gradient ===")
			for i := 1; i < len(prof); i++ {
				dw := float64(prof[i].WindowSize - prof[i-1].WindowSize)
				dr := prof[i].Mean - prof[i-1].Mean
				if dw > 0 {
					fmt.Printf("  %d -> %d:  %+.6f per bit\n",
						prof[i-1].WindowSize, prof[i].WindowSize, dr/dw)
				}
			}
		}
	}

	fmt.Println()
}
