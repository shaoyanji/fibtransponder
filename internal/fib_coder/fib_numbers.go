package fib_coder

import (
	"fmt"
	"strings"
)

var fibNumbers []int // Precomputed Fibonacci numbers (0-indexed F[0]=0, F[1]=1, F[2]=1, F[3]=2...)

func init() {
	// Generate Fibonacci numbers up to a reasonable limit.
	// F(50) is ~1.2e10, large enough for run lengths up to ~1.2e10.
	// F(93) is ~7.5e18 (largest fit in int64)
	fibNumbers = make([]int, 51) // Max index 50, so F_50
	fibNumbers[0] = 0
	fibNumbers[1] = 1
	for i := 2; i <= 50; i++ {
		fibNumbers[i] = fibNumbers[i-1] + fibNumbers[i-2]
	}
}

// IntToFibonacciCode converts an integer N (N >= 0) into its raw Zeckendorf representation.
// The representation is a bit string using F_i for i >= 2, ordered with the largest Fibonacci number's bit on the left.
// If N=0, it returns an empty string.
// Example: IntToFibonacciCode(6) returns "1001" (F_5 + F_2)
func IntToFibonacciCode(n int) (string, error) {
	if n < 0 {
		return "", fmt.Errorf("cannot encode negative number %d", n)
	}
	if n == 0 {
		return "", nil // Special case for 0 (no '1's in Zeckendorf representation)
	}

	// Find largest fibNumbers[idx] <= n. This corresponds to F_idx.
	largestFibIdx := 0
	for i := len(fibNumbers) - 1; i >= 2; i-- { // Iterate downwards from largest F_k (index `i`) to F_2
		if fibNumbers[i] <= n {
			largestFibIdx = i
			break
		}
	}
	if largestFibIdx == 0 { // This case should only happen if n=0, which is handled above.
		return "", fmt.Errorf("could not find largest fib number for %d (n>0 but no F_i >=2 <= n)", n)
	}

	codeBuilder := strings.Builder{}
	currentValue := n
	lastSetIndex := -1 // To enforce non-consecutive rule, stores fibNumbers index of last '1'

	// Iterate downwards from highestFibIdx to 2 (F_2)
	for i := largestFibIdx; i >= 2; i-- {
		// Standard greedy algorithm for Zeckendorf
		if currentValue >= fibNumbers[i] && (lastSetIndex == -1 || i < lastSetIndex-1) { // lastSetIndex check not strictly needed for greedy
			codeBuilder.WriteByte('1')
			currentValue -= fibNumbers[i]
			lastSetIndex = i // Record the index of the Fibonacci number just used
		} else {
			codeBuilder.WriteByte('0')
		}
	}
	
	// The generated code is highest F_i first. For N=6, this will be "1001" (F5,F4,F3,F2).
	// This matches the test cases for raw code.
	return codeBuilder.String(), nil
}

// FibonacciCodeToInt converts a raw Zeckendorf representation back to an integer.
// The code is expected to be ordered with the largest Fibonacci number's bit on the left.
// An empty string decodes to 0.
// Example: FibonacciCodeToInt("1001") returns 6.
func FibonacciCodeToInt(code string) (int, error) {
	if code == "" {
		return 0, nil
	}

	n := 0
	lastSetFibIdx := -1 // To check non-consecutive rule during decoding

	// The code string represents F_k, F_{k-1}, ..., F_2
	// So, the length L of the code string means the first bit corresponds to F_{L+1}.
	// The `i`-th bit (0-indexed from left) corresponds to F_{L+1-i}.
	
	codeLen := len(code)
	for i := 0; i < codeLen; i++ {
		if code[i] == '1' {
			fibIdx := codeLen + 1 - i // Map string index to Fibonacci index (F_2 to F_{codeLen+1})
			
			// Ensure we don't exceed our precomputed Fibonacci numbers
			if fibIdx >= len(fibNumbers) {
				return 0, fmt.Errorf("fibonacci index %d out of bounds for code '%s' at bit %d", fibIdx, code, i)
			}

			// Check for non-consecutive rule (encoded bit should not be adjacent to previously set bit)
			if lastSetFibIdx != -1 && fibIdx == lastSetFibIdx-1 {
				return 0, fmt.Errorf("invalid fibonacci code: consecutive '1's in Zeckendorf representation (F_%d and F_%d)", lastSetFibIdx, fibIdx)
			}
			
			n += fibNumbers[fibIdx]
			lastSetFibIdx = fibIdx
		}
	}

	return n, nil
}
