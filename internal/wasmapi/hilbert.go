package wasmapi

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"io"

	"github.com/shaoyanji/fibtransponder/internal/fib_coder"
	"github.com/shaoyanji/fibtransponder/internal/image_hilbert"
)

// hilbertAPI serves the image-codec demo.
//
// The upstream entry point is image_hilbert.GenerateBitstream, which takes a
// filesystem path and so cannot run in a browser. Both of its halves are
// reachable from here instead: the traversal is the exported
// image_hilbert.D2XY, and the codec is fib_coder.Encode, which works on
// io.Reader/io.Writer and therefore accepts a bytes.Buffer. The thresholding
// step is reproduced from GenerateBitstream and is identical to it:
//
//	gray = uint8((0.299*r + 0.587*g + 0.114*b) / 256)   // Rec.601, /257
//	bit  = gray > threshold
type hilbertAPI struct{}

type hilbertEncodeArgs struct {
	// PNG is a base64-encoded PNG. Preset, Checkerboard or Pattern is used when
	// none is given, so the demo works with no upload.
	PNG          string `json:"png"`
	Preset       string `json:"preset"`
	Pattern      string `json:"pattern"`
	Order        int    `json:"order"`
	Threshold    int    `json:"threshold"`
	Checkerboard bool   `json:"checkerboard"`
	// Length is the bit count for Pattern input. Ignored for images, which are
	// always order^2 bits.
	Length int `json:"length"`
}

type runRecord struct {
	Bit         int    `json:"bit"`
	Value       int    `json:"value"`
	Length      int    `json:"length"`
	Codeword    string `json:"codeword"`
	StartOutBit int    `json:"startOutBit"`
	StartCoded  int    `json:"startCodedBit"`
	CodedLen    int    `json:"codedLength"`
}
type hilbertEncodeOut struct {
	Order     int `json:"order"`
	Size      int `json:"size"`
	Threshold int `json:"threshold"`
	// Preset names the generated input, "upload" for a visitor's own image, or
	// the pattern name for raw bits. It is carried through because the
	// compression ratio is entirely a function of run structure, so the page
	// must always say which input produced it.
	Preset     string `json:"preset"`
	BitCount   int    `json:"bitCount"`
	Ones       int    `json:"ones"`
	LongestRun int    `json:"longestRun"`
	// MeanRunLength is the number that actually predicts the ratio: the codec
	// beats even at about 9 bits per run.
	MeanRunLength int `json:"meanRunLength"`
	// Bits is the thresholded stream in Hilbert-traversal order, as '0'/'1'.
	Bits string `json:"bits"`
	// CodedBase64 is the HRL payload: fib_coder.Encode's output with its own
	// 8-byte length header stripped, since that is reported separately as part
	// of the container.
	CodedBase64  string  `json:"codedBase64"`
	CodedBits    int     `json:"codedBits"`
	ExactCoded   int     `json:"exactCodedBits"`
	OriginalLen  int     `json:"originalLenInBits"`
	HeaderHex    string  `json:"containerHeaderHex"`
	Ratio        float64 `json:"ratio"`
	SourcePixels int     `json:"sourcePixels"`
	// Runs is the per-run display table. StartCoded/CodedLen are exact, so any
	// offset into the compressed file maps to the output bit range it produced.
	// That mapping is what lets the reveal animation show the image emerging
	// from a partially consumed file without the client decoding anything.
	Runs    []runRecord `json:"runs"`
	RunNote string      `json:"runNote"`
}

func (*hilbertAPI) encode(raw json.RawMessage) (result, error) {
	var a hilbertEncodeArgs
	if err := json.Unmarshal(raw, &a); err != nil {
		return result{}, errf(CodeBadArgs, "hilbert.encode: %v", err)
	}
	order := clampInt(defaultIfZero(a.Order, 3), 1, 7)
	size := 1 << order
	threshold := clampInt(a.Threshold, 0, 255)

	// A raw bit pattern bypasses the image path entirely. This matters because
	// the codec's actual domain is bitstreams with long runs, not images --
	// see the note on PatternNames. Without this the page can only show the
	// expanding cases.
	if a.Pattern != "" && a.PNG == "" && !a.Checkerboard && a.Preset == "" {
		bits, err := patternBits(a.Pattern, clampInt(defaultIfZero(a.Length, 1024), 2, 1<<20))
		if err != nil {
			return result{}, err
		}
		// 0x0: a raw pattern has no pixel dimensions, so the width and height
		// fields are zero and only the bit length is meaningful.
		return encodeBits(bits, 0, 0, order, threshold, a.Pattern)
	}

	img, preset, err := resolveImage(a, size)
	if err != nil {
		return result{}, err
	}
	return encodeBits(thresholdToBits(img, size, uint8(threshold)), size, size, order, threshold, preset)
}

// encodeBits is the shared tail: thresholded-or-raw bits in, the full codec
// report out. width and height are the container's dimension fields: the actual
// image side for image input, zero for a raw pattern.
func encodeBits(bits []uint8, width, height, order, threshold int, preset string) (result, error) {
	ones := onesCount(bits)

	runs, exactCoded := encodeWithRuns(bits)

	// The real encoder is the source of truth for the artifact on disk.
	var encoded bytes.Buffer
	if err := fib_coder.Encode(newByteReader(packBits(bits)), &encoded, uint64(len(bits))); err != nil {
		return result{}, errf(CodeInternal, "hilbert.encode: %v", err)
	}
	payload := encoded.Bytes()
	if len(payload) < 8 {
		return result{}, errf(CodeInternal,
			"hilbert.encode: encoded stream is %d bytes, shorter than its 8-byte header", len(payload))
	}
	body := payload[8:]
	codedBits := len(body) * 8

	// Container header, matching cmd/hilbert_gen's writer: two big-endian
	// uint32 dimensions, then fib_coder.Encode's own big-endian uint64 length.
	header := make([]byte, 0, 16)
	header = appendBE32(header, uint32(width))
	header = appendBE32(header, uint32(height))
	header = appendBE64(header, uint64(len(bits)))

	longest := 0
	for _, r := range runs {
		if r.Length > longest {
			longest = r.Length
		}
	}
	meanRun := 0
	if len(runs) > 0 {
		meanRun = len(bits) / len(runs)
	}

	return result{Data: hilbertEncodeOut{
		Order:         order,
		Size:          width,
		Threshold:     threshold,
		Preset:        preset,
		BitCount:      len(bits),
		Ones:          ones,
		Bits:          bitsToString(bits),
		CodedBase64:   base64.StdEncoding.EncodeToString(body),
		CodedBits:     codedBits,
		ExactCoded:    exactCoded,
		OriginalLen:   len(bits),
		HeaderHex:     hexOf(header),
		Ratio:         ratio(codedBits, len(bits)),
		SourcePixels:  width * height,
		Runs:          runs,
		LongestRun:    longest,
		MeanRunLength: meanRun,
		RunNote: "overhead is 1 type bit + len(Fibonacci code of length+1) + 3 terminator " +
			"bits per run, so a run of length L costs about log_phi(L) + 4 bits. That only " +
			"beats L once L is roughly 9 or more, which is why the ratio tracks mean run " +
			"length far more closely than it tracks anything about the codec.",
	}}, nil
}

func resolveImage(a hilbertEncodeArgs, size int) (image.Image, string, error) {
	switch {
	case a.Checkerboard:
		return checkerboardImage(size), presetCheckerboard, nil
	case a.PNG != "":
		rawPNG, err := base64.StdEncoding.DecodeString(a.PNG)
		if err != nil {
			return nil, "", errf(CodeBadArgs, "hilbert.encode: png is not base64: %v", err)
		}
		decoded, err := png.Decode(bytes.NewReader(rawPNG))
		if err != nil {
			return nil, "", errf(CodeBadArgs, "hilbert.encode: png decode failed: %v", err)
		}
		return decoded, "upload", nil
	case a.Preset != "":
		img, err := presetImage(a.Preset, size)
		if err != nil {
			return nil, "", err
		}
		return img, a.Preset, nil
	default:
		return photoLikeImage(size), presetPhoto, nil
	}
}

// ── hilbert.path ──

type hilbertPathArgs struct {
	Order  int `json:"order"`
	Stride int `json:"stride"`
}

type hilbertPathOut struct {
	Order      int `json:"order"`
	Size       int `json:"size"`
	Stride     int `json:"stride"`
	PointCount int `json:"pointCount"`
	// Pairs is a flat [x0,y0, x1,y1, ...] list. The client draws consecutive
	// pairs as the traversal path.
	Pairs []int `json:"pairs"`
}

func (*hilbertAPI) path(raw json.RawMessage) (result, error) {
	var a hilbertPathArgs
	if err := json.Unmarshal(raw, &a); err != nil {
		return result{}, errf(CodeBadArgs, "hilbert.path: %v", err)
	}
	order := clampInt(defaultIfZero(a.Order, 3), 1, 7)
	size := 1 << order
	stride := clampInt(defaultIfZero(a.Stride, 1), 1, size)

	total := size * size
	out := hilbertPathOut{
		Order:  order,
		Size:   size,
		Stride: stride,
		Pairs:  make([]int, 0, (total/stride+1)*2),
	}
	for d := 0; d < total; d += stride {
		x, y := image_hilbert.D2XY(uint32(size), uint32(d))
		out.Pairs = append(out.Pairs, int(x), int(y))
	}
	out.PointCount = len(out.Pairs) / 2
	return result{Data: out}, nil
}

// checkerboardImage builds the (x+y)%2 fixture the repo's integration tests
// use, so the demo's default has a known-good expected value.
func checkerboardImage(size int) image.Image {
	img := image.NewGray(image.Rect(0, 0, size, size))
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			if (x+y)%2 == 0 {
				img.SetGray(x, y, color.Gray{Y: 0})
			} else {
				img.SetGray(x, y, color.Gray{Y: 255})
			}
		}
	}
	return img
}

// thresholdToBits is GenerateBitstream's loop over an already-decoded image:
// walk d = 0..n*n-1, map d through the Hilbert curve, read that pixel.
func thresholdToBits(img image.Image, size int, threshold uint8) []uint8 {
	bits := make([]uint8, 0, size*size)
	for d := 0; d < size*size; d++ {
		x, y := image_hilbert.D2XY(uint32(size), uint32(d))
		r, g, b, _ := img.At(int(x), int(y)).RGBA()
		gray := uint8((0.299*float64(r) + 0.587*float64(g) + 0.114*float64(b)) / 256)
		if gray > threshold {
			bits = append(bits, 1)
		} else {
			bits = append(bits, 0)
		}
	}
	return bits
}

// ── run table ──

// encodeWithRuns builds the HRL bitstream and the per-run display table.
//
// The scheme mirrors fib_coder's Hybrid Run-Length Encoding: a type bit, then
// the Fibonacci code of (runLength+1), then the "011" terminator. A canonical
// Zeckendorf word has no adjacent 1-bits, so it can never contain "011"
// internally, which is exactly what makes the terminator self-synchronising
// when scanning forward from the start of a codeword.
//
// The bit count this reports is the exact one; the padded, on-disk figure comes
// from fib_coder.Encode above.
func encodeWithRuns(bits []uint8) ([]runRecord, int) {
	sink := &bitSink{}
	runs := make([]runRecord, 0, 64)
	codedPos, outPos := 0, 0
	for i := 0; i < len(bits); {
		v := bits[i] & 1
		j := i
		for j < len(bits) && bits[j]&1 == v {
			j++
		}
		runLen := j - i
		code, err := fib_coder.IntToFibonacciCode(runLen + 1)
		if err != nil {
			// Run length beyond the Fibonacci table; stop rather than emit a
			// stream that would not round-trip.
			break
		}
		rec := runRecord{
			Bit:         i,
			Value:       int(v),
			Length:      runLen,
			Codeword:    code,
			StartOutBit: outPos,
			StartCoded:  codedPos,
		}
		sink.writeBit(v)
		sink.writeString(code)
		sink.writeString("011")
		rec.CodedLen = sink.n - codedPos
		codedPos = sink.n
		outPos += runLen
		runs = append(runs, rec)
		i = j
	}
	return runs, codedPos
}

// bitSink writes MSB-first with a zero-padded trailing byte, matching
// bitio.BitWriter's order and its Flush.
type bitSink struct {
	buf bytes.Buffer
	n   int
	cur byte
	pos uint
}

func (s *bitSink) writeBit(b byte) {
	s.cur |= (b & 1) << (7 - s.pos)
	s.pos++
	s.n++
	if s.pos == 8 {
		s.buf.WriteByte(s.cur)
		s.cur, s.pos = 0, 0
	}
}

func (s *bitSink) writeString(v string) {
	for i := 0; i < len(v); i++ {
		switch v[i] {
		case '0':
			s.writeBit(0)
		case '1':
			s.writeBit(1)
		}
	}
}

// ── bit <-> byte plumbing ──

// packBits converts a bit slice to raw bytes, MSB-first within each byte. This
// is the form bitio.BitReader expects: it does its own MSB-first extraction,
// so handing it ASCII '0'/'1' bytes would shift every bit by one position.
func packBits(bits []uint8) []byte {
	out := make([]byte, (len(bits)+7)/8)
	for i, b := range bits {
		if b&1 == 1 {
			out[i/8] |= 1 << uint(7-i%8)
		}
	}
	return out
}

// newByteReader wraps a byte slice as an io.Reader, the shape
// bitio.BitReader refills from one byte at a time.
func newByteReader(b []byte) io.Reader { return bytes.NewReader(b) }

func appendBE32(dst []byte, v uint32) []byte {
	return append(dst, byte(v>>24), byte(v>>16), byte(v>>8), byte(v))
}

func appendBE64(dst []byte, v uint64) []byte {
	return append(dst,
		byte(v>>56), byte(v>>48), byte(v>>40), byte(v>>32),
		byte(v>>24), byte(v>>16), byte(v>>8), byte(v))
}

func hexOf(b []byte) string {
	const digits = "0123456789abcdef"
	out := make([]byte, 0, len(b)*2)
	for _, c := range b {
		out = append(out, digits[c>>4], digits[c&0x0F])
	}
	return string(out)
}
