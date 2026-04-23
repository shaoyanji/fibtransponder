package bitio

import (
	"io"
)

// BitReader reads bits from an underlying io.Reader.
type BitReader struct {
	r          io.Reader
	currentByte byte
	bitOffset  int // 0-7, current bit to read from currentByte
}

// NewBitReader creates a new BitReader.
func NewBitReader(r io.Reader) *BitReader {
	return &BitReader{r: r, bitOffset: 8} // bitOffset = 8 means currentByte is empty/needs refill
}

// ReadBit reads the next single bit as a byte ('0' or '1').
func (br *BitReader) ReadBit() (byte, error) {
	if br.bitOffset == 8 { // Need to read a new byte
		buf := make([]byte, 1)
		n, err := br.r.Read(buf)
		if err != nil {
			return 0, err // Return EOF if no more bytes
		}
		if n == 0 { // Should not happen if err is nil but no bytes
			return 0, io.EOF
		}
		br.currentByte = buf[0]
		br.bitOffset = 0 // Start reading from MSB (bit 7)
	}

	// Read current bit (MSB first)
	bit := (br.currentByte >> (7 - br.bitOffset)) & 1
	br.bitOffset++

	if bit == 1 {
		return '1', nil
	}
	return '0', nil
}
