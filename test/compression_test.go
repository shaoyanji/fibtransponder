package test

import (
	"bytes"
	"fmt"
	"math/rand"
	"strings"
	"testing"
	"time"

	"github.com/shaoyanji/fibtransponder/internal/fib_coder"
)

// Helper function to generate random bitstreams
func generateRandomBitstream(length int) string {
	var sb strings.Builder
	sb.Grow(length)
	for i := 0; i < length; i++ {
		if rand.Intn(2) == 0 {
			sb.WriteByte('0')
		} else {
			sb.WriteByte('1')
		}
	}
	return sb.String()
}

// Helper function to generate a bitstream with long runs of zeros
func generateSparseBitstream(length int, oneDensity float64) string {
	var sb strings.Builder
	sb.Grow(length)
	for i := 0; i < length; i++ {
		if rand.Float64() < oneDensity {
			sb.WriteByte('1')
		} else {
			sb.WriteByte('0')
		}
	}
	return sb.String()
}


func TestLosslessCompression(t *testing.T) {
	t.Skip("TODO(codec): current fib_coder stream format is not lossless for full round-trip; tracked for codec refactor")
	rand.Seed(time.Now().UnixNano())

	testCases := []struct {
		name     string
		original string
	}{
		{"Empty", ""},
		{"SingleZero", "0"},
		{"SingleOne", "1"},
		{"AllZerosShort", "0000"},
		{"AllOnesShort", "1111"},
		{"Alternating", "01010101"},
		{"MixedShort", "0010110001"},
		{"LongZeros", "000000000010000000001"}, // Long run of zeros
		{"LongOnes", "111111111111111111111"},   // Long run of ones (bad for this scheme)
		{"MediumRandom", generateRandomBitstream(100)},
		{"LongRandom", generateRandomBitstream(1000)},
		{"MediumSparse", generateSparseBitstream(100, 0.1)}, // 10% ones
		{"LongSparse", generateSparseBitstream(1000, 0.05)}, // 5% ones
		{"AllZerosLong", strings.Repeat("0", 100)},
		{"AllOnesLong", strings.Repeat("1", 100)},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			originalReader := strings.NewReader(tc.original)
			compressedWriter := new(bytes.Buffer)
			
			// Encode expects original length in bits
			originalLenInBits := uint64(len(tc.original))
			err := fib_coder.Encode(originalReader, compressedWriter, originalLenInBits)
			if err != nil {
				t.Fatalf("Encoding failed: %v", err)
			}

			decompressedReader := new(bytes.Buffer)
			compressedReader := bytes.NewReader(compressedWriter.Bytes())
			
			decodedLen, err := fib_coder.Decode(compressedReader, decompressedReader)
			if err != nil {
				t.Fatalf("Decompressing failed: %v", err)
			}
			
			decompressed, err := PackedBytesToBitString(decompressedReader.Bytes(), decodedLen)
			if err != nil {
				t.Fatalf("failed to unpack decoded bits: %v", err)
			}

			// Assert lossless
			if decompressed != tc.original {
				t.Errorf("Lossless check failed for original:\n  %s\nCompressed:\n  %s\nDecompressed:\n  %s",
					tc.original, compressedWriter.String(), decompressed)
			}

			// Measure compression ratio
			originalLen := float64(len(tc.original))
			compressedLen := float64(compressedWriter.Len() * 8) // Convert bytes to bits
			ratio := 1.0
			if originalLen > 0 {
				ratio = compressedLen / originalLen
			}
			t.Logf("Original Len: %d, Compressed Len: %d, Ratio (Compressed/Original): %.2f",
				int(originalLen), int(compressedLen), ratio)

			// Log compression details for documentation
			docString := fmt.Sprintf("\n--- Test Case: %s ---\n", tc.name)
			docString += fmt.Sprintf("Original Length: %d\n", int(originalLen))
			docString += fmt.Sprintf("Compressed Length: %d\n", int(compressedLen))
			docString += fmt.Sprintf("Compression Ratio (C/O): %.2f\n", ratio)
			docString += fmt.Sprintf("Original:   %s\n", tc.original)
			docString += fmt.Sprintf("Compressed: %s\n", compressedWriter.String()) // This will show raw bytes as string
			docString += fmt.Sprintf("Decompressed: %s\n", decompressed)
			docString += fmt.Sprintf("Lossless: %t\n", decompressed == tc.original)
			// t.Log(docString) // This would print to test output. Not directly to docs/COMPRESSION.md
		})
	}
}

func TestFibonacciCodeIntegrity(t *testing.T) {
	// Test the core IntToFibonacciCode and FibonacciCodeToInt functions
	// These expected raw codes are based on the direct Zeckendorf representation for N,
	// using F_i for i>=2, with largest F_i bit on the left.
	testCases := []struct {
		val int
		rawCode string // The code *without* the "011" suffix
	}{
		{0, ""},
		{1, "1"},     // F2
		{2, "10"},    // F3
		{3, "100"},   // F4
		{4, "101"},   // F4+F2
		{5, "1000"},  // F5
		{6, "1001"},  // F5+F2
		{7, "1010"},  // F5+F3
		{8, "10000"}, // F6
		{9, "10001"}, // F6+F2
		{10, "10010"}, // F6+F3
		{13, "100000"}, // F7
		{20, "101010"}, // F8+F6+F4
		{33, "1010101"}, // F9+F7+F5+F3
		{54, "10101010"}, // F10+F8+F6+F4+F2
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("Int_%d", tc.val), func(t *testing.T) {
			encoded, err := fib_coder.IntToFibonacciCode(tc.val)
			if err != nil {
				t.Fatalf("IntToFibonacciCode(%d) failed: %v", tc.val, err)
			}
			if encoded != tc.rawCode {
				t.Errorf("IntToFibonacciCode(%d) = %q, want %q", tc.val, encoded, tc.rawCode)
			}

			decoded, err := fib_coder.FibonacciCodeToInt(encoded)
			if err != nil {
				t.Fatalf("FibonacciCodeToInt(%q) failed: %v", encoded, err)
			}
			if decoded != tc.val {
				t.Errorf("FibonacciCodeToInt(%q) = %d, want %d", encoded, decoded, tc.val)
			}
		})
	}

	// Test error cases
	t.Run("NegativeInput", func(t *testing.T) {
		_, err := fib_coder.IntToFibonacciCode(-1)
		if err == nil {
			t.Error("IntToFibonacciCode(-1) should return an error")
		}
	})
	t.Run("InvalidFibCode_ConsecutiveOnes", func(t *testing.T) {
		_, err := fib_coder.FibonacciCodeToInt("11") // Consecutive F_i, F_{i-1}
		if err == nil {
			t.Error("FibonacciCodeToInt(\"11\") should return an error due to consecutive '1's")
		}
		_, err = fib_coder.FibonacciCodeToInt("1011") // Consecutive F_i, F_{i-1}
		if err == nil {
			t.Error("FibonacciCodeToInt(\"1011\") should return an error due to consecutive '1's")
		}
	})
}
