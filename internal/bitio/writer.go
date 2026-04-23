package bitio

import (
	"fmt"
	"io"
)

// BitWriter writes bits to an underlying io.Writer.
type BitWriter struct {
	w           io.Writer
	currentByte byte
	bitOffset   int // 0-7, current bit to write to currentByte
	writtenBits uint64 // Total number of logical bits written
}

// NewBitWriter creates a new BitWriter.
func NewBitWriter(w io.Writer) *BitWriter {
	return &BitWriter{w: w}
}

// WriteBit writes a single bit ('0' or '1') to the underlying writer.
func (bw *BitWriter) WriteBit(bit byte) error {
	if bit == '1' {
		bw.currentByte |= (1 << (7 - bw.bitOffset)) // MSB first
	} else if bit != '0' {
		return fmt.Errorf("invalid bit character '%c'", bit)
	}

	bw.bitOffset++
	bw.writtenBits++

	if bw.bitOffset == 8 { // currentByte is full, write it
		if _, err := bw.w.Write([]byte{bw.currentByte}); err != nil {
			return fmt.Errorf("error writing byte: %w", err)
		}
		bw.currentByte = 0
		bw.bitOffset = 0
	}
	return nil
}

// WriteString writes a string of '0's and '1's as bits.
func (bw *BitWriter) WriteString(s string) error {
	for _, r := range s {
		if err := bw.WriteBit(byte(r)); err != nil {
			return err
		}
	}
	return nil
}

// Flush writes any remaining buffered bits to the underlying writer, padding with zeros if necessary.
// It returns the total number of physical bytes written, including partial bytes.
func (bw *BitWriter) Flush() error {
	if bw.bitOffset > 0 { // There's a partial byte
		if _, err := bw.w.Write([]byte{bw.currentByte}); err != nil {
			return fmt.Errorf("error flushing partial byte: %w", err)
		}
		// Reset for next use if needed, though typically Flush is last operation
		bw.currentByte = 0
		bw.bitOffset = 0
	}
	return nil
}

// TotalWrittenBits returns the total number of logical bits written.
func (bw *BitWriter) TotalWrittenBits() uint64 {
	return bw.writtenBits
}
