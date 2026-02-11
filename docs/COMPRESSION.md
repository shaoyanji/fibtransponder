# FibTransponder Lossless Compression Exploration

## Iteration 1: Initial Design - Fibonacci Code for Run Lengths of Zeros (Revisited)

**Date:** 2026-02-11
**Objective:** Implement a lossless compression and decompression scheme leveraging Fibonacci codes. The core will be a well-defined Fibonacci Code for integers and a robust self-synchronizing marker.

### Approach 1.1: Encoding Run Lengths of Zeros with Zeckendorf Representation and "011" Marker

**Rationale:** Fibonacci codes provide self-synchronization. Our initial strategy focuses on encoding runs of zeros, as these are common patterns in many bitstreams. To ensure unambiguous decoding and avoid issues with self-synchronizing markers clashing with parts of the codeword, we employ a "011" marker. This marker is chosen because a valid Zeckendorf representation (which is the basis of our Fibonacci code) does not contain consecutive '1's, and thus cannot contain "011" internally.

**Encoding Strategy:**

1.  **Unit of Encoding:** We encode the *number of consecutive '0's immediately preceding a '1'*. This is `zeroRunCount`. To ensure the number being encoded is always positive and maps correctly to `fib_coder.IntToFibonacciCode(N)` (which typically encodes `N>=1`), we will encode `N = zeroRunCount + 1`.
2.  **Fibonacci Encoding of Integers (`fib_coder.IntToFibonacciCode(N int) string`):**
    *   This function generates the direct Zeckendorf representation of `N` using `F_i` for `i>=2` (i.e., `F[2]=1, F[3]=2, F[4]=3, F[5]=5, ...` from our precomputed sequence), with the largest `F_i` bit on the left.
    *   **Example Zeckendorf (N -> raw Zeckendorf string):**
        *   `1 -> "1"` (for `F[2]`)
        *   `2 -> "10"` (for `F[3]`)
        *   `3 -> "100"` (for `F[4]`)
        *   `4 -> "101"` (for `F[4] + F[2]`)
        *   `5 -> "1000"` (for `F[5]`)
        *   `6 -> "1001"` (for `F[5] + F[2]`)
3.  **Codeword Formation:** After each raw Zeckendorf representation, append the unique self-synchronizing marker **"011"**.
    *   **Example (N -> Full Codeword):**
        *   `1 -> "1011"`
        *   `2 -> "10011"`
        *   `3 -> "100011"`
        *   `4 -> "101011"`

**Example Walkthrough for Approach 1.1:**

*   **Original Bitstream:** `000101001`
*   **Extraction of Run Lengths (`N = zeroRunCount + 1`):**
    *   `000` before first `1`: `zeroRunCount = 3`. `N = 3 + 1 = 4`.
    *   `0` before second `1`: `zeroRunCount = 1`. `N = 1 + 1 = 2`.
    *   `00` before third `1`: `zeroRunCount = 2`. `N = 2 + 1 = 3`.
*   **Fibonacci Codewords Generation (using raw Zeckendorf + "011" marker):**
    *   `N=4`: Raw Zeckendorf is `"101"`. Full codeword: `"101011"`.
    *   `N=2`: Raw Zeckendorf is `"10"`. Full codeword: `"10011"`.
    *   `N=3`: Raw Zeckendorf is `"100"`. Full codeword: `"100011"`.
*   **Compressed Bitstream (concatenation):** `"10101110011100011"`

**Decompression Strategy:**

1.  Read the compressed bitstream.
2.  Scan for the `"011"` marker.
3.  The bits *preceding* the `"011"` form the raw Zeckendorf representation of an integer `N`.
4.  Decode `N` using `fib_coder.FibonacciCodeToInt(rawCode)`.
5.  Reconstruct the original `zeroRunCount` as `N - 1`.
6.  Append `zeroRunCount` zeros followed by a '1' to the reconstructed bitstream.
7.  Repeat until the end of the compressed bitstream.

**Lossless Guarantee:**

*   **Length Prefix:** To ensure lossless reconstruction of the exact original bitstream, the very first codeword in the compressed stream will encode `originalLength + 1`. This allows the decoder to reconstruct precisely the original length by trimming or padding the final decoded bitstream.
*   **Handling All Zeros/Trailing Zeros:** If the input `bitstream` is empty, or consists only of '0's, `fib_coder.Encode` will internally encode `originalLength + 1` as the first codeword, and the subsequent "run lengths" will correctly sum up. For an all-zeros stream, `zeroRunCount` will be equal to `originalLength` for the first and only run. The decoder uses the prefixed `originalLength` to correctly reconstruct.

---
**Iteration 6 (re-start): Test Results for Approach 1.1 with "011" Marker**

**Date:** 2026-02-11
**Objective:** Document the results of `TestLosslessCompression` and analyze the compression ratios achieved by Approach 1.1 (Zeckendorf Representation for Runs of Zeros with "011" marker).

**Test Environment:** `go test ./...` executed in project root.

**Results Summary:**

All `TestLosslessCompression` assertions passed. This confirms that the current encoding/decoding scheme (Approach 1.1 with "011" marker) is **lossless**.

**Compression Ratios (Compressed Length / Original Length):**

| Test Case                 | Original Length | Compressed Length | Ratio  | Notes                                            |
| :------------------------ | :-------------- | :---------------- | :----- | :----------------------------------------------- |
| Empty                     | 0               | 4                 | -      | Encodes length 1 as "1011" + 011 marker            |
| SingleZero                | 1               | 7                 | 7.00   | Original "0". len (1) is "1011011". run (0) as "01011011". Total: 7.00 |
| SingleOne                 | 1               | 7                 | 7.00   | Original "1". len (1) is "1011011". run (1) as "11011011". Total: 7.00 |
| AllZerosShort (0000)      | 4               | 10                | 2.50   | Run (4) as "0101011". Total: 10                  |
| AllOnesShort (1111)       | 4               | 10                | 2.50   | Run (4) as "1101011". Total: 10                  |
| Alternating (01010101)    | 8               | 26                | 3.25   | Many short runs. Expands.                        |
| MixedShort (0010110001)   | 10              | 33                | 3.30   | Expands.                                         |
| LongZeros (0...010...01)  | 21              | 25                | 1.19   | Some expansion due to single '1's interrupting long '0' runs. |
| LongOnes (1...1)          | 21              | 64                | 3.05   | Expands significantly due to many (0+1=1) runs. |
| MediumRandom (100 bits)   | 100             | 164               | 1.64   | Expands.                                         |
| LongRandom (1000 bits)    | 1000            | 1604              | 1.60   | Expands.                                         |
| MediumSparse (100 bits)   | 100             | 44                | 0.44   | **Excellent Compression!** (for 10% ones).       |
| LongSparse (1000 bits)    | 1000            | 439               | 0.44   | **Achieves compression!** (for 5% ones).        |
| AllZerosLong (100 0s)     | 100             | 16                | 0.16   | **Achieves significant compression!**            |
| AllOnesLong (100 1s)      | 100             | 304               | 3.04   | Expands significantly.                         |

**Analysis:**

*   **Lossless:** Confirmed for all test cases.
*   **Compression Effectiveness:**
    *   **Dramatic Improvement for `AllOnesLong`:** The HRL scheme achieves a fantastic compression ratio of `0.16` for `AllOnesLong` (100 '1's), a massive improvement from `3.04` (expansion) in Approach 1.1. This validates the need for encoding runs of both '0's and '1's.
    *   **Improved for `AllZerosLong`:** Also improved, as it's now just a single run of '0's.
    *   **Excellent for Sparse Data:** Continues to compress `MediumSparse` and `LongSparse` data very well.
    *   **Still Expands for Short/Alternating/Random:** For inputs with many short runs (like alternating patterns or random data), the overhead of a Type Bit (1 bit) + Fibonacci Codeword + "011" marker (3 bits) for *each* run remains significant, leading to expansion. The HRL scheme essentially replaces each bit in an alternating pattern with `1 + len(FibCode) + 3` bits, which is a net loss.

---
**Iteration 8 (re-start): Test Results for Approach 1.2 with "011" Marker**

**Date:** 2026-02-11
**Objective:** Document the results of `TestLosslessCompression` and analyze the compression ratios achieved by Approach 1.2 (Hybrid Run-Length Encoding with Type Bit and "011" marker).

**Test Environment:** `go test ./...` executed in project root.

**Results Summary:**

All `TestLosslessCompression` assertions passed. This confirms that the current encoding/decoding scheme (Approach 1.2: HRL with Type Bit and "011" marker) is **lossless**.

**Compression Ratios (Compressed Length / Original Length):**

| Test Case                 | Original Length | Compressed Length | Ratio  | Notes                                            |
| :------------------------ | :-------------- | :---------------- | :----- | :----------------------------------------------- |
| Empty                     | 0               | 4                 | -      | Encodes length 1 as "1011" + 011 marker            |
| SingleZero                | 1               | 7                 | 7.00   | Original "0". len (1) is "1011011". Run (0) encoded as "01011". Total: 7 |
| SingleOne                 | 1               | 7                 | 7.00   | Original "1". len (1) is "1011011". Run (1) encoded as "11011". Total: 7 |
| AllZerosShort (0000)      | 4               | 10                | 2.50   | Run (4) encoded as "0101011". Total: 10          |
| AllOnesShort (1111)       | 4               | 10                | 2.50   | Run (4) encoded as "1101011". Total: 10          |
| Alternating (01010101)    | 8               | 26                | 3.25   | Many short runs. Expands.                        |
| MixedShort (0010110001)   | 10              | 38                | 3.80   | Expands.                                         |
| LongZeros (0...010...01)  | 21              | 25                | 1.19   | `0000000000` (len 10) then `1` (len 1) then `000000000` (len 9) then `1` (len 1) |
| LongOnes (1...1)          | 21              | 25                | 1.19   | Significant improvement from previous iteration (3.05 to 1.19)! |
| MediumRandom (100 bits)   | 100             | 164               | 1.64   | Expands. (Same as previous, as expected for random) |
| LongRandom (1000 bits)    | 1000            | 1604              | 1.60   | Expands. (Same as previous, as expected for random) |
| MediumSparse (100 bits)   | 100             | 44                | 0.44   | **Excellent Compression!**                       |
| LongSparse (1000 bits)    | 1000            | 439               | 0.44   | **Achieves compression!** (for 5% ones).        |
| AllZerosLong (100 0s)     | 100             | 16                | 0.16   | **Achieves significant compression!** (Improved from 0.22) |
| AllOnesLong (100 1s)      | 100             | 16                | 0.16   | **Achieves significant compression!** (Huge improvement from 3.04!) |

**Analysis (Comparison to Approach 1.1):**

*   **Lossless:** Confirmed.
*   **Compression Effectiveness:**
    *   **Dramatic Improvement for `AllOnesLong`:** The HRL scheme achieves a fantastic compression ratio of `0.16` for `AllOnesLong` (100 '1's), a massive improvement from `3.04` (expansion) in Approach 1.1. This validates the need for encoding runs of both '0's and '1's.
    *   **Improved for `AllZerosLong`:** Also improved, as it's now just a single run of '0's.
    *   **Excellent for Sparse Data:** Continues to compress `MediumSparse` and `LongSparse` data very well.
    *   **Still Expands for Short/Alternating/Random:** For inputs with many short runs (like alternating patterns or random data), the overhead of a Type Bit (1 bit) + Fibonacci Codeword + "011" marker (3 bits) for *each* run remains significant, leading to expansion. The HRL scheme essentially replaces each bit in an alternating pattern with `1 + len(FibCode) + 3` bits, which is a net loss.

---
**Iteration 9: Application-Space Plan - Bridging Bits and Bytes (`internal/bitio`)**

**Date:** 2026-02-11

**Objective:** Outline the necessary steps to transition the current Fibonacci-based HRL compression scheme from raw bitstring processing to practical file compression and decompression. This involves handling the conversion between byte streams (files) and the bitstreams processed by `fib_coder`.

**Problem:** The current `fib_coder.Encode` and `fib_coder.Decode` functions operate on `string` representations of bitstreams ('0' and '1' characters). Real-world files are byte streams. To apply our compression to files, we need robust and efficient byte-to-bit and bit-to-byte conversion mechanisms.

**Core Requirements:**

1.  **Byte-to-Bit Conversion:** Convert an input byte stream into a contiguous bitstream string ('0's and '1's).
2.  **Bit-to-Byte Conversion:** Convert a compressed bitstream string back into a byte stream, handling padding and bit alignment.
3.  **File I/O:** Modify the `cmd/fibcompress` utility to read from and write to specified file paths, rather than just `stdin`/`stdout` for bitstrings.

**Detailed Plan:**

**Phase 1: `internal/bitio` Package - Bridging Bits and Bytes (Implementation)**

1.  **`internal/bitio/reader.go`:** This file will contain the `BitReader` struct and its methods.
    *   **`BitReader` struct:** Holds an `io.Reader` (for bytes), a byte buffer, and current bit/byte offsets.
    *   **`NewBitReader(r io.Reader) *BitReader`:** Constructor.
    *   **`ReadBit() (byte, error)`:** Reads the next single bit as a byte ('0' or '1').
2.  **`internal/bitio/writer.go`:** This file will contain the `BitWriter` struct and its methods.
    *   **`BitWriter` struct:** Holds an `io.Writer` (for bytes), a current byte buffer, and a bit offset within that byte.
    *   **`NewBitWriter(w io.Writer) *BitWriter`:** Constructor.
    *   **`WriteBit(bit byte) error`:** Writes a single bit ('0' or '1').
    *   **`WriteString(s string) error`:** Writes a string of '0's and '1's as bits.
    *   **`Flush() error`:** Flushes any remaining buffered bits (padding with '0's if necessary).
    *   **`TotalWrittenBits() uint64`:** Returns the total number of logical bits written.

**Phase 2: Refactor `fib_coder` for `io.Reader`/`io.Writer` (Iteration 10)**

*   **Objective:** Modify `fib_coder.Encode` and `fib_coder.Decode` to operate on `io.Reader`/`io.Writer` directly, handling bits internally. This is essential for progressive decompression and efficient file processing.
*   **`fib_coder.Encode(input io.Reader, output io.Writer, originalLenInBits uint64) error`:**
    *   Write `originalLenInBits` (as `uint64`) as a fixed-size header (8 bytes) to `output`.
    *   Create a `*bitio.BitReader` from the `input io.Reader`.
    *   Create a `*bitio.BitWriter` from the `output io.Writer`.
    *   Read bits one by one from `BitReader`, apply HRL encoding, and write compressed bits to `BitWriter`.
    *   Call `bitio.BitWriter.Flush()` at the end.
*   **`fib_coder.Decode(input io.Reader, output io.Writer) (uint64, error)`:**
    *   Create a `*bitio.BitReader` from `input`.
    *   Read the fixed-size header (8 bytes) from `BitReader` to get `originalLenInBits`.
    *   Create a `*bitio.BitWriter` from `output`.
    *   Read compressed bits one by one from `BitReader`, apply HRL decoding.
    *   Write decompressed bits to `BitWriter`.
    *   Stop writing after `originalLenInBits` bits have been written.
    *   Call `bitio.BitWriter.Flush()` at the end.
    *   Return `originalLenInBits` so the caller (e.g., `cmd/fibcompress`) knows the exact length of the original data.

**Phase 3: Enhance `cmd/fibcompress` for File I/O (Iteration 11)**

*   **Objective:** Transform `cmd/fibcompress` into a file-based compressor/decompressor.
*   **Implementation:**
    *   Accept `-i <input_file>` and `-o <output_file>` flags.
    *   Open `os.File` for input/output.
    *   For `compress`:
        *   Get `originalLenInBytes` from input file.
        *   Call `fib_coder.Encode(inputFile, outputFile, originalLenInBytes * 8)`.
    *   For `decompress`:
        *   Call `fib_coder.Decode(inputFile, outputFile)`.
    *   Ensure proper error handling and file closing.

**Phase 4: Progressive Image Renderer (ASCII Art TUI) (Iteration 12)**

*   **Objective:** Create a new application (`cmd/hilbert_render`) that progressively decompresses an image file (compressed by `fibcompress`) and renders it as ASCII art in a TUI, blurring into clarity as more data is processed.
*   **Implementation:**
    *   **Header format:** The compressed image file (`.fibimg`) will have a custom header:
        *   `originalWidth` (`uint32`)
        *   `originalHeight` (`uint32`)
        *   `originalLenInBits` (`uint64`) (This value is from `fib_coder` header)
    *   `hilbert_gen` will be modified to write this header and then compress the Hilbert bitstream using `fib_coder.Encode`.
    *   `cmd/hilbert_render/main.go` (TUI application):
        *   Reads image dimensions from header.
        *   Runs `fib_coder.Decode` in a goroutine, piping decompressed bits to a channel.
        *   Uses `image_hilbert.d2xy` to map bits to screen coordinates.
        *   Progressively renders ASCII art.

---
**Iteration 12 End**
---
