package fib_coder

import (
	"encoding/binary"
	"fmt"
	"io"
	"strings"

	"github.com/shaoyanji/fibtransponder/internal/bitio"
)

// Encode takes a binary stream (from io.Reader) and compresses it
// using a Hybrid Run-Length Encoding (HRL) with Fibonacci coding.
// The compressed output is written to io.Writer.
// The originalLenInBits is the total number of bits in the original (uncompressed) data.
func Encode(input io.Reader, output io.Writer, originalLenInBits uint64) error {
	// 1. Write Header: originalLengthInBits
	headerBuf := make([]byte, 8)
	binary.BigEndian.PutUint64(headerBuf, originalLenInBits)
	if _, err := output.Write(headerBuf); err != nil {
		return fmt.Errorf("failed to write header: %w", err)
	}

	br := bitio.NewBitReader(input)
	bw := bitio.NewBitWriter(output)
	defer bw.Flush() // Ensure any partial byte is written at the end

	if originalLenInBits == 0 { // Handle empty original file
		// Encode length 1 (for N=0+1)
		encodedLength, err := IntToFibonacciCode(1) // Code for 1
		if err != nil {
			return fmt.Errorf("failed to encode length for empty bitstream: %w", err)
		}
		if err := bw.WriteString(encodedLength); err != nil {
			return err
		}
		if err := bw.WriteString("011"); err != nil { // Suffix with marker
			return err
		}
		return nil
	}

	// Read first bit to initialize currentRunType
	firstBit, err := br.ReadBit()
	if err != nil {
		return fmt.Errorf("failed to read first bit: %w", err)
	}

	currentRunType := firstBit
	runLength := 1
	bitsRead := uint64(1)

	for bitsRead < originalLenInBits {
		nextBit, err := br.ReadBit()
		if err != nil {
			return fmt.Errorf("error reading bit from input: %w", err)
		}
		bitsRead++

		if nextBit == currentRunType {
			runLength++
		} else {
			// Encode the completed run
			if err := encodeRun(bw, currentRunType, runLength); err != nil {
				return err
			}
			// Start new run
			currentRunType = nextBit
			runLength = 1
		}
	}

	// Encode the last run after loop finishes
	if runLength > 0 {
		if err := encodeRun(bw, currentRunType, runLength); err != nil {
			return err
		}
	}

	return nil
}

// encodeRun is a helper to write a single encoded run to the BitWriter.
func encodeRun(bw *bitio.BitWriter, runType byte, length int) error {
	// 1. Type Bit: Prepend '0' for a run of '0's, '1' for a run of '1's
	if err := bw.WriteBit(runType); err != nil {
		return err
	}

	// 2. Run Length: Encode runLength + 1
	encodedRun, err := IntToFibonacciCode(length + 1)
	if err != nil {
		return err
	}
	if err := bw.WriteString(encodedRun); err != nil {
		return err
	}
	if err := bw.WriteString("011"); err != nil { // Suffix the run length codeword with "011"
		return err
	}
	return nil
}

// Decode reads a compressed stream from io.Reader and writes the decompressed data to io.Writer.
// It returns the original number of bits and any error encountered.
func Decode(input io.Reader, output io.Writer) (uint64, error) {
	// 1. Read Header: originalLengthInBits
	headerBuf := make([]byte, 8)
	if _, err := io.ReadFull(input, headerBuf); err != nil {
		return 0, fmt.Errorf("failed to read header: %w", err)
	}
	originalLengthInBits := binary.BigEndian.Uint64(headerBuf)
	if originalLengthInBits == 0 { // Handle empty original file
		return 0, nil
	}

	br := bitio.NewBitReader(input)
	bw := bitio.NewBitWriter(output)
	defer bw.Flush() // Ensure any partial byte is written at the end

	decodedBitsCount := uint64(0)
	for decodedBitsCount < originalLengthInBits {
		// Read Type Bit
		runTypeBit, err := br.ReadBit()
		if err != nil {
			if err == io.EOF && decodedBitsCount == originalLengthInBits {
				break // Reached expected end (should be clean)
			}
			return 0, fmt.Errorf("failed to read run type bit: %w", err)
		}

		// Read Run Length Fibonacci Code
		var rawCodeBuilder strings.Builder
		for {
			bit, err := br.ReadBit()
			if err != nil {
				if err == io.EOF && decodedBitsCount == originalLengthInBits {
					break // Reached expected end (should be clean)
				}
				return 0, fmt.Errorf("failed to read fibonacci code bit: %w", err)
			}
			rawCodeBuilder.WriteByte(bit)
			if strings.HasSuffix(rawCodeBuilder.String(), "011") {
				break
			}
		}
		codewordWithMarker := rawCodeBuilder.String()
		rawCode := codewordWithMarker[:len(codewordWithMarker)-3] // Remove "011" marker

		// Decode N
		runCountPlusOne, err := FibonacciCodeToInt(rawCode)
		if err != nil {
			return 0, fmt.Errorf("failed to decode run length from '%s': %w", rawCode, err)
		}
		if runCountPlusOne == 0 {
			return 0, fmt.Errorf("decoded run length (N+1) cannot be 0")
		}
		runCount := runCountPlusOne - 1

		// Append bits based on type and length
		for i := 0; i < runCount; i++ {
			if err := bw.WriteBit(runTypeBit); err != nil {
				return 0, err
			}
			decodedBitsCount++
			if decodedBitsCount == originalLengthInBits { // Stop if we've reached original length
				break
			}
		}
	}

	return originalLengthInBits, nil
}
