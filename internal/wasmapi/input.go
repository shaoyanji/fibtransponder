package wasmapi

import (
	"unicode/utf8"
)

// Input handling for the demos.
//
// The demos all take text and turn it into a bitstream, but they do not agree
// on how, and the difference is worth being explicit about rather than hiding:
//
//   - The FSVM ingests true UTF-8 bytes. internal/transponder.BytesToBits is
//     the function the corpus experiments use (RunCorpusExperiment), so that
//     is what the shim uses for every bit-exact demo.
//
//   - internal/dilationtree.TextToBits iterates runes and emits 8 bits per
//     *rune*, truncating the code point to its low byte: for c in s { byte(c) }.
//     For ASCII that is identical to UTF-8 byte expansion. For anything else
//     it is lossy in a way the repo already acknowledges (a French corpus
//     collapses to 907 bytes).
//
// Since tree.build must call the real BuildFromText to stay faithful to
// cmd/dilation_tree, pasting an em-dash would silently change the result
// relative to the CLI. So it is reported as a warning rather than swallowed.

// capText enforces MaxTextBytes and returns the text plus whether it is
// ASCII-only.
func capText(text string) (string, bool, error) {
	if len(text) > MaxTextBytes {
		return "", false, errf(CodeInputTooLarge,
			"input is %d bytes, limit is %d", len(text), MaxTextBytes)
	}
	return text, isASCII(text), nil
}

func isASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] >= utf8.RuneSelf {
			return false
		}
	}
	return true
}

// describeNonASCII names the first offending rune so a warning is actionable
// instead of a vague "unsupported characters".
func describeNonASCII(s string) string {
	for _, r := range s {
		if r >= utf8.RuneSelf {
			return string(r)
		}
	}
	return ""
}

// defaultText is the shared demo input: the same Fibonacci essay the corpus
// experiments use, trimmed to something that reads as prose and fits a
// textarea without scrolling.
const defaultText = "The quick brown fox jumps over the lazy dog. " +
	"Every natural number is uniquely a sum of non-consecutive Fibonacci numbers."

// countNonASCII reports how many runes are outside ASCII, used in the warning
// text for the demos that take true UTF-8.
func countNonASCII(s string) int {
	n := 0
	for _, r := range s {
		if r >= utf8.RuneSelf {
			n++
		}
	}
	return n
}

// utf8RuneCount is the unit dilationtree.TextToBits actually iterates: one
// 8-bit group per rune, not per byte.
func utf8RuneCount(s string) int { return utf8.RuneCountInString(s) }
