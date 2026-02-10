package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"sort"

	"github.com/shaoyanji/fibtransponder/internal/bitrope"
	"github.com/shaoyanji/fibtransponder/internal/fsvm"
	"github.com/shaoyanji/fibtransponder/internal/segauto"
	"github.com/shaoyanji/fibtransponder/internal/signal/embed2d"
	"github.com/shaoyanji/fibtransponder/internal/signal/fft"
	"github.com/shaoyanji/fibtransponder/internal/signal/wht"
)

// CLI demo:
// - reads 0/1 from stdin
// - runs FSVM (dilation + marker emission)
// - keeps a bit rope
// - updates a symbolic segmentation NFA on marker events
// - on EOF prints a summary + WHT(top components) + 2D box-count sketch
func main() {
	window := flag.Int("window", 64, "analysis window size (uses largest pow2 <= window)")
	width2d := flag.Int("width2d", 16, "2D embed width")
	blockBits := flag.Int("blockBits", 1<<16, "rope block size in bits")
	doFFT := flag.Bool("fft", false, "also compute FFT power spectrum (baseline)")
	jsonOut := flag.Bool("json", false, "emit final summary as JSON")
	flag.Parse()

	rope := bitrope.New(*blockBits)
	st := fsvm.New()
	nfa := segauto.New()

	in := bufio.NewReader(os.Stdin)
	var i uint64
	for {
		c, err := in.ReadByte()
		if err != nil {
			break
		}
		var b uint8
		switch c {
		case '0':
			b = 0
		case '1':
			b = 1
		case '\n', '\r', '\t', ' ':
			continue
		default:
			continue
		}
		rope.AppendBit(b)
		var evs []fsvm.Event
		st, evs = fsvm.Step(st, b)
		nfa = segauto.Step(nfa, b)
		for _, ev := range evs {
			switch ev.Kind {
			case fsvm.EventDilate:
				fmt.Printf("i=%d DILATE r=%d\n", i, st.R)
			case fsvm.EventMarker:
				// marker is a segmentation opportunity; allowed cut
				nfa = segauto.MarkerCut(nfa)
				fmt.Printf("i=%d MARKER zero_run=%d nfa_mask=0x%02x\n", i, ev.Payload, nfa.Mask)
			}
		}
		i++
	}

	type outSummary struct {
		LenBits   uint64 `json:"len_bits"`
		R         uint32 `json:"r"`
		Dilations uint64 `json:"dilations"`
		Markers   uint64 `json:"markers"`
		ZeroRun   uint64 `json:"zero_run"`
		W         string `json:"w"`
		NFAMask   uint8  `json:"nfa_mask"`
	}
	out := outSummary{
		LenBits:   rope.LenBits(),
		R:         st.R,
		Dilations: st.Dilations,
		Markers:   st.Markers,
		ZeroRun:   st.ZeroRun,
		W:         fmt.Sprintf("%06b", st.W),
		NFAMask:   nfa.Mask,
	}
	if *jsonOut {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(out)
	} else {
		fmt.Printf("\n=== summary ===\n")
		fmt.Printf("len_bits=%d r=%d dilations=%d markers=%d zero_run=%d w=%06b nfa_mask=0x%02x\n", rope.LenBits(), st.R, st.Dilations, st.Markers, st.ZeroRun, st.W, nfa.Mask)
	}

	// Windowed transforms (WHT and optional FFT baseline)
	n := int(rope.LenBits())
	if n > *window {
		n = *window
	}
	pow2 := 1
	for pow2*2 <= n {
		pow2 *= 2
	}
	if pow2 >= 2 {
		start := rope.LenBits() - uint64(pow2)
		bits := rope.ReadBits(start, pow2)

		// WHT (buffer reuse)
		a := make([]int, pow2)
		ps := make([]int64, pow2)
		wht.FillBoolToBipolar(a, bits)
		wht.FWHT(a)
		wht.PowerInto(ps, a)
		type kv struct{ idx int; p int64 }
		arr := make([]kv, len(ps))
		for i, p := range ps {
			arr[i] = kv{i, p}
		}
		sort.Slice(arr, func(i, j int) bool { return arr[i].p > arr[j].p })
		fmt.Printf("\n=== WHT ===\nwindow=%d top:\n", pow2)
		for j := 0; j < 8 && j < len(arr); j++ {
			fmt.Printf("  %4d %d\n", arr[j].idx, arr[j].p)
		}

		// FFT baseline
		if *doFFT {
			c := make([]complex128, pow2)
			p2 := make([]float64, pow2)
			fft.FillBoolToCentered(c, bits)
			fft.FFT(c)
			fft.PowerInto(p2, c)
			type kvf struct{ idx int; p float64 }
			arr2 := make([]kvf, len(p2))
			for i, p := range p2 {
				arr2[i] = kvf{i, p}
			}
			sort.Slice(arr2, func(i, j int) bool { return arr2[i].p > arr2[j].p })
			fmt.Printf("\n=== FFT (baseline) ===\nwindow=%d top:\n", pow2)
			for j := 0; j < 8 && j < len(arr2); j++ {
				fmt.Printf("  %4d %.6f\n", arr2[j].idx, arr2[j].p)
			}
		}
	}

	// 2D embed + box count (last up to 256 bits)
	take := 256
	if int(rope.LenBits()) < take { take = int(rope.LenBits()) }
	if take > 0 {
		start := rope.LenBits() - uint64(take)
		bits := rope.ReadBits(start, take)
		grid := embed2d.Embed(bits, *width2d)
		pairs := embed2d.MultiScaleBoxCounts(grid, []int{1,2,4,8})
		fmt.Printf("\n=== 2D box counts ===\nembedded last %d bits into W=%d H=%d\n", take, *width2d, len(grid))
		for _, p := range pairs {
			fmt.Printf("  box=%2d count=%d\n", p.Box, p.Count)
		}
	}
}
