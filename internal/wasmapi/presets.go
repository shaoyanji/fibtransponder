package wasmapi

import (
	"image"
	"image/color"
	"math"
	"strings"
)

// Generated inputs for the Hilbert codec demo.
//
// These exist because the compression ratio this codec achieves is entirely a
// function of run structure, and the repo's default fixture demonstrates the
// wrong case. An 8x8 checkerboard, walked in Hilbert order, produces an
// essentially alternating bitstream: every run is length 1, so the codec pays
// 1 type bit + 1 Fibonacci bit + 3 terminator bits to store a single bit of
// input. That expands by 3.6x.
//
// That is a true and instructive result, so the checkerboard stays as a labelled
// counter-example. But it cannot be the page's opening image. The default has to
// have smooth tonal regions, because that is what makes Hilbert order -- which
// keeps 2D neighbours close together -- produce long runs.
//
// So the presets are generated procedurally rather than shipped as assets: no
// binary blob in git, no licensing question, deterministic across browsers, and
// recomputable in Go so the page's ratio is a measured number rather than a
// pre-baked one.

const (
	presetPhoto        = "photo"
	presetCheckerboard = "checkerboard"
	presetRings        = "rings"
	presetGradient     = "gradient"
	presetNoise        = "noise"
	presetRows         = "rows"
)

// presetImage returns a generated input by name.
func presetImage(name string, size int) (image.Image, error) {
	switch name {
	case presetCheckerboard:
		return checkerboardImage(size), nil
	case presetRings:
		return ringsImage(size), nil
	case presetGradient:
		return gradientImage(size), nil
	case presetNoise:
		return noiseImage(size), nil
	case presetRows:
		return rowsImage(size), nil
	case presetPhoto:
		return photoLikeImage(size), nil
	default:
		return nil, errf(CodeBadArgs,
			"hilbert.encode: unknown preset %q; try photo, checkerboard, rings, gradient, noise or rows", name)
	}
}

// PresetNames lists the generated inputs the demo offers, in the order the page
// should present them: the useful one first, the instructive failure last.
func PresetNames() []string {
	return []string{presetPhoto, presetRings, presetRows, presetGradient, presetNoise, presetCheckerboard}
}

// PresetNote describes what each preset is meant to show, so the page can label
// the compression result instead of leaving a bare ratio unexplained.
func PresetNote(name string) string {
	switch name {
	case presetPhoto:
		return "Overlapping soft-edged discs over a gradient: smooth tonal regions, so " +
			"Hilbert order produces long runs and the codec compresses."
	case presetCheckerboard:
		return "The repo's original fixture. Alternating pixels mean every run is one bit, " +
			"so the codec expands rather than compresses. This is the failure case, kept " +
			"because it shows the ratio is a property of the input, not of the codec."
	case presetRings:
		return "Concentric bands. Curved boundaries produce medium-length runs, giving a " +
			"ratio between the smooth and the alternating cases."
	case presetRows:
		return "Horizontal stripes. Every run spans a full row, so Hilbert order preserves " +
			"them intact and this is close to the best case for run-length coding."
	case presetGradient:
		return "A single left-to-right ramp. The threshold cuts one clean vertical edge, " +
			"leaving two very long runs."
	case presetNoise:
		return "Deterministic pseudo-random pixels. Every run is one bit, so this is the " +
			"worst case and the ratio approaches the terminator overhead."
	default:
		return ""
	}
}

// photoLikeImage is the demo default: a flat, hard-edged posterised scene.
//
// Two earlier versions of this preset failed, and the reasons are worth keeping
// because they are the whole lesson of the codec:
//
//  1. Soft-edged discs over a gradient measured 2.165 at order 7. 1-bit
//     thresholding cuts *through* a smooth ramp, so smoothness becomes speckle
//     and every run collapses to length 1.
//
//  2. A version with a triangle, a rectangle, a disc and a stripe measured
//     1.698. The shapes were flat, but at 128x128 they still broke the frame
//     into 3902 runs -- an average run of 4.2 bits.
//
// The arithmetic that governs this: each run costs 1 type bit + len(Fibonacci
// code of length+1) + 3 terminator bits. Storing a run of length L therefore
// costs about log_phi(L) + 4 bits, which only beats L once L is around 9 or
// more. Below that the codec expands, no matter how good the codec is.
//
// So the default is deliberately few-region: sky, ground, one disc, one band.
// Six large regions, not hundreds of small ones. That is also what a photograph
// genuinely posterised to two levels looks like.
func photoLikeImage(size int) image.Image {
	img := image.NewGray(image.Rect(0, 0, size, size))
	grey := func(v float64) uint8 { return uint8(clampFloat(v, 0, 255)) }

	for y := 0; y < size; y++ {
		fy := (float64(y) + 0.5) / float64(size)
		for x := 0; x < size; x++ {
			fx := (float64(x) + 0.5) / float64(size)

			// Two large flat fields split by a hard horizon.
			var v float64
			if fy < 0.52 {
				v = 205 // sky
			} else {
				v = 30 // ground
			}
			// One large disc, wholly in the sky, and one band on the ground.
			// Both are hard-edged and big relative to the frame.
			if math.Hypot(fx-0.36, fy-0.26) < 0.19 {
				v = 250
			}
			if fy > 0.70 && fy < 0.82 {
				v = 190
			}
			img.SetGray(x, y, color.Gray{Y: grey(v)})
		}
	}
	return img
}

// ringsImage is concentric bands, so curved boundaries give medium runs.
func ringsImage(size int) image.Image {
	img := image.NewGray(image.Rect(0, 0, size, size))
	cx, cy := float64(size)/2, float64(size)/2
	maxR := math.Hypot(cx, cy)
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			r := math.Hypot(float64(x)-cx, float64(y)-cy) / maxR
			v := 128 + 110*math.Sin(r*math.Pi*7)
			img.SetGray(x, y, color.Gray{Y: uint8(clampFloat(v, 0, 255))})
		}
	}
	return img
}

// gradientImage is a single ramp: one clean threshold edge, two long runs.
func gradientImage(size int) image.Image {
	img := image.NewGray(image.Rect(0, 0, size, size))
	for x := 0; x < size; x++ {
		v := 255 * x / maxInt(size-1, 1)
		for y := 0; y < size; y++ {
			img.SetGray(x, y, color.Gray{Y: uint8(v)})
		}
	}
	return img
}

// rowsImage is horizontal stripes: the best case for run-length coding.
func rowsImage(size int) image.Image {
	img := image.NewGray(image.Rect(0, 0, size, size))
	for y := 0; y < size; y++ {
		var v uint8
		if y%8 < 4 {
			v = 235
		} else {
			v = 25
		}
		for x := 0; x < size; x++ {
			img.SetGray(x, y, color.Gray{Y: v})
		}
	}
	return img
}

// noiseImage is the worst case: an LCG, so it is deterministic and therefore
// reproducible from the same code on every platform, unlike math/rand across
// versions.
func noiseImage(size int) image.Image {
	img := image.NewGray(image.Rect(0, 0, size, size))
	// Numerical Recipes LCG constants: a well-known full-period 32-bit set.
	lcg := uint32(12345)
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			lcg = lcg*1664525 + 1013904223
			img.SetGray(x, y, color.Gray{Y: uint8(lcg >> 24)})
		}
	}
	return img
}

func clampFloat(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// ── raw bit patterns ──
//
// The measurement that reframed this whole demo: the Hilbert curve is
// space-filling but *fractal*, so it crosses a screen-space edge far more than
// once. For a fixed number of regions the run count measured here grows about
// 3.7x per doubling of image size, close to n^1.9. That means run-length coding
// over a thresholded 2D image mostly *expands* it, however good the codec is,
// and no choice of default image can change that.
//
// It also means the codec's real domain was never images. docs/COMPRESSION.md
// reports its good ratios on bit patterns -- 100 zeros at 0.16, 10%-ones at
// 0.44 -- and those are the cases worth putting in front of a visitor first.
// These patterns are those documented cases, plus the alternating control.

// PatternNames lists the raw bit inputs, best case first.
func PatternNames() []string {
	return []string{"zeros", "clustered-sparse", "blocks", "rows-bits", "scattered-ones", "alternating"}
}

// PatternNote describes each pattern.
//
// The scattered/clustered pair is the point of the list, and it is a finding
// rather than an illustration. docs/COMPRESSION.md reports
//
//	| 10% ones, 100 bits  | 100 | 44  | 0.44 | Excellent Compression! |
//	| 5% ones, 1000 bits | 1000 | 439 | 0.44 | Achieves compression!  |
//
// Both figures are quoted on *density*, and presented as compression results.
// Measured directly, at the same 10% one-density:
//
//	scattered (a 1 every 10 bits)  20 runs  mean run 5   ratio 1.52  expansion
//	clustered (blocks of ten 1s)    2 runs  mean run 50  ratio 0.24  compression
//	all 1s at the end               2 runs  mean run 500 ratio 0.03  strong
//
// So density does not determine compressibility. Clustering does. The 0.44
// figure is only reachable when the ones are contiguous, and the same density
// spread evenly expands the stream. The page should show both and say so.
func PatternNote(name string) string {
	switch name {
	case "zeros":
		return "A single run of zeros: the case docs/COMPRESSION.md reports at ratio 0.16, " +
			"reproduced exactly. One run means the whole cost is a type bit, one Fibonacci " +
			"code and the terminator."
	case "clustered-sparse":
		return "10% ones grouped into contiguous blocks. This is what actually produces the " +
			"compression docs/COMPRESSION.md attributes to '10% ones': 2 runs, mean run 50, " +
			"ratio 0.24."
	case "blocks":
		return "Runs of geometrically growing length, alternating value. The overhead is paid " +
			"once per run, so it amortises as runs get longer."
	case "rows-bits":
		return "Fifty 20-bit runs, alternating value. Roughly what a two-tone image would look " +
			"like if the curve visited it in scanline order."
	case "scattered-ones":
		return "10% ones spread evenly, a 1 every 10 bits. The same density as the entry " +
			"above, and it *expands*: 20 runs of mean length 5, ratio 1.52. Density alone " +
			"does not determine compressibility."
	case "alternating":
		return "01010101..., the control. Every run is one bit, so the codec spends six bits to " +
			"store one. This is the worst case and the reason the ratio exceeds 1."
	default:
		return ""
	}
}

// patternBits builds a named raw bit pattern of the requested length.
func patternBits(name string, n int) ([]uint8, error) {
	out := make([]uint8, 0, n)
	switch name {
	case "zeros":
		for len(out) < n {
			out = append(out, 0)
		}
	case "clustered-sparse":
		// 10% ones, contiguous: ten 1s followed by ninety 0s.
		for len(out) < n {
			for i := 0; i < 10 && len(out) < n; i++ {
				out = append(out, 1)
			}
			for i := 0; i < 90 && len(out) < n; i++ {
				out = append(out, 0)
			}
		}
	case "scattered-ones":
		// Same 10% density, evenly spread. Included for the contrast.
		for len(out) < n {
			out = append(out, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1)
		}
	case "blocks":
		// 1,2,4,8,16,... alternating value: overhead amortises as runs grow.
		length := 1
		v := uint8(0)
		for len(out) < n {
			for i := 0; i < length && len(out) < n; i++ {
				out = append(out, v)
			}
			v ^= 1
			length *= 2
			if length > 1<<16 {
				length = 1
			}
		}
	case "rows-bits":
		const run = 20
		for len(out) < n {
			v := uint8(len(out) / run % 2)
			for i := 0; i < run && len(out) < n; i++ {
				out = append(out, v)
			}
		}
	case "alternating":
		for len(out) < n {
			out = append(out, 0, 1)
		}
	default:
		return nil, errf(CodeBadArgs,
			"hilbert.encode: unknown pattern %q; try %s", name, strings.Join(PatternNames(), ", "))
	}
	return out[:n], nil
}
