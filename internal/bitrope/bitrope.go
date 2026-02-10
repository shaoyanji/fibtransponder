package bitrope

// Append-only packed bit rope with fixed-size blocks.
//
// Primary goal: predictable allocations + cheap appends + cheap window reads.
// This is a pragmatic building block for unbounded boolean streams.

import "fmt"

type Cursor struct {
	Block int
	Off   int // bit offset within block
}

type Rope struct {
	BlockBits int
	blocks    [][]uint64
	nBitsLast int
	lenBits   uint64
}

func New(blockBits int) *Rope {
	if blockBits <= 0 {
		blockBits = 1 << 16 // 65536 bits
	}
	// round up to multiple of 64
	if blockBits%64 != 0 {
		blockBits = ((blockBits + 63) / 64) * 64
	}
	return &Rope{BlockBits: blockBits}
}

func (r *Rope) LenBits() uint64 { return r.lenBits }

func (r *Rope) Blocks() (numBlocks int, blockBits int) { return len(r.blocks), r.BlockBits }

func (r *Rope) ensureBlock() {
	if len(r.blocks) == 0 || r.nBitsLast >= r.BlockBits {
		nWords := r.BlockBits / 64
		r.blocks = append(r.blocks, make([]uint64, nWords))
		r.nBitsLast = 0
	}
}

// AppendBit appends a single bit and returns its cursor.
func (r *Rope) AppendBit(b uint8) Cursor {
	b &= 1
	r.ensureBlock()
	bi := len(r.blocks) - 1
	off := r.nBitsLast
	wi := off >> 6
	bit := off & 63
	if b == 1 {
		r.blocks[bi][wi] |= 1 << bit
	}
	r.nBitsLast++
	r.lenBits++
	return Cursor{Block: bi, Off: off}
}

// Get returns the bit at absolute position i; out-of-range reads return 0.
func (r *Rope) Get(i uint64) uint8 {
	if i >= r.lenBits {
		return 0
	}
	bi := int(i / uint64(r.BlockBits))
	off := int(i) - bi*r.BlockBits
	wi := off >> 6
	bit := off & 63
	return uint8((r.blocks[bi][wi] >> bit) & 1)
}

// ReadBits returns n bits starting at start. Out-of-range bits are 0.
func (r *Rope) ReadBits(start uint64, n int) []uint8 {
	if n <= 0 {
		return nil
	}
	out := make([]uint8, n)
	for j := 0; j < n; j++ {
		out[j] = r.Get(start + uint64(j))
	}
	return out
}

// ReadU64Window reads up to 64 bits into an LSB-first uint64.
func (r *Rope) ReadU64Window(start uint64, n int) uint64 {
	if n <= 0 {
		return 0
	}
	if n > 64 {
		n = 64
	}
	var x uint64
	for j := 0; j < n; j++ {
		x |= (uint64(r.Get(start+uint64(j))) & 1) << uint(j)
	}
	return x
}

func (r *Rope) String() string {
	return fmt.Sprintf("Rope{lenBits=%d blocks=%d blockBits=%d}", r.lenBits, len(r.blocks), r.BlockBits)
}
