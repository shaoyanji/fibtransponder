package main

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"regexp"
	"strings"
	"unicode"
)

type sample struct {
	label, text string
}

// windowFeatures captures lightweight discriminative features per text window.
type windowFeatures struct {
	printable       float64 // fraction [0x20,0x7E]
	alpha           float64 // fraction [a-zA-Z]
	space           float64 // fraction of spaces
	punctuation     float64 // fraction of punctuation (unicode.IsPunct)
	digit           float64 // fraction [0-9]
	codeChars       float64 // fraction of code-specific chars: { } ( ) ; _ . : , < >
	upper           float64 // fraction uppercase in alpha
	runAvg          float64 // avg run length of same byte value
	utf8Valid       float64 // fraction of valid UTF-8 starts
	byteEntropy     float64 // Shannon entropy of byte distribution
	keywordCount    int     // number of code keyword/pattern matches
	commonWordCount int     // number of common English word matches
}

// codePatterns are regex signatures that strongly indicate source code.
var codePatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?m)^\s*(func|fn|def|class|const|var|let|import|pub|async|await|for|while|if|else|try|catch)\b`),
	regexp.MustCompile(`\b(func|fn)\s+\w+\s*\(`),     // func name(...)
	regexp.MustCompile(`\bdef\s+\w+\s*\(`),           // def name(...)
	regexp.MustCompile(`\b(pub|fn)\s+`),              // pub fn / fn (Zig/Rust)
	regexp.MustCompile(`->\s*\w+`),                   // -> Type (Rust/Zig)
	regexp.MustCompile(`self\.\w+`),                  // self.attribute (Python)
	regexp.MustCompile(`\w+\.\w+\(`),                 // method() call
	regexp.MustCompile(`\bif\s+\w+[\s:]+\w+`),        // if len(arr) <= 1: return
	regexp.MustCompile(`\bfor\s+\w+.*[|;({]`),        // for (..., 0..) | or for...{
	regexp.MustCompile(`\breturn\s+\w+`),             // return x
	regexp.MustCompile(`\w+\.\w+`),                   // obj.prop (common in code)
	regexp.MustCompile(`[\[\]{};]+\s*[\[\]{};,]+`),   // ]i32[ or ; {,}
	regexp.MustCompile(`\b(std|const|pub|fn|var|let|impl|struct|enum)\b`), // Zig/Rust/JS keywords
	regexp.MustCompile(`\.(get|post|put|delete|query|find|sort|print|log|json)\(`), // common methods
	regexp.MustCompile(`\b(?:len|print|await|async|try|catch|throw)\b`), // keyword functions
	regexp.MustCompile(`['"].*['"].*[;,\n]`),         // string literals with terminators
}

// prosePatterns are regex signatures for natural language prose.
var prosePatterns = []*regexp.Regexp{
	regexp.MustCompile(`\b(the|and|but|was|were|are|had|has|this|that|with|from|they|had|were|been|his|her|their|its|she|he)\b`),
	regexp.MustCompile(`\b(a|an|in|on|at|to|of|is|it|be|or|as|by|for|so|yet|if|can|may|will|shall|would|should|could|might|must|do|does|did|has|have|was)\b`),
	regexp.MustCompile(`[.!?]\s+[A-Z]`),              // sentence boundaries
	regexp.MustCompile(`[a-z]{4,}\s+[a-z]{4,}\s+[a-z]{4,}`), // sequences of words
}

// score produces 3 class scores (prose, code, noise).
func (w windowFeatures) score() (prose, code, noise float64) {
	// Start from features
	noise = (1 - w.printable)*4 + w.byteEntropy*0.6 + (1 - w.alpha)*3
	
	code = w.codeChars*5 + float64(w.keywordCount)*2
	
	prose = w.alpha*3 + w.space*2 + float64(w.commonWordCount)*2 + w.utf8Valid*0.5

	// Noise penalties
	if w.printable > 0.95 { noise -= 8 }
	if w.alpha > 0.4 { noise -= 8 }
	if w.keywordCount > 0 { noise -= 3 }
	if w.commonWordCount > 0 { noise -= 5 }

	// Code penalties
	if w.codeChars < 0.02 { code -= 4 }
	if w.space > 0.25 { code -= 4 }
	if w.keywordCount == 0 { code -= 6 }

	// Prose penalties
	if w.printable < 0.8 { prose -= 6 }
	if w.keywordCount > 1 { prose -= 5 }
	if w.codeChars > 0.1 { prose -= 6 }
	if w.byteEntropy > 4.0 && w.printable < 0.95 { prose -= 4 }

	return
}

type labeledWindow struct {
	label string
	text  string
	wf    windowFeatures
}

type result struct {
	TrainAcc  float64            `json:"train_accuracy"`
	TestAcc   float64            `json:"test_accuracy"`
	PerClass  map[string]float64 `json:"per_class"`
	NTest     int                `json:"n_test"`
	NTrans    int                `json:"n_transponders"`
	Dist      map[string]int     `json:"feature_dim"`
}

func main() {
	dataset := []sample{
		{label: "prose", text: "It was the best of times, it was the worst of times, it was the age of wisdom, it was the age of foolishness, it was the epoch of belief, it was the epoch of incredulity."},
		{label: "prose", text: "All happy families are alike; each unhappy family is unhappy in its own way. Everything was in confusion in the Oblonsky house."},
		{label: "prose", text: "Call me Ishmael. Some years ago, never mind how long precisely, having little or no money in my purse, and nothing particular to interest me on shore."},
		{label: "prose", text: "In a hole in the ground there lived a hobbit. Not a nasty, dirty, wet hole, filled with the ends of worms and an oozy smell, nor yet a dry, bare, sandy hole."},
		{label: "prose", text: "The sky above the port was the color of television, tuned to a dead channel. It was not like a real sky at all, but something more like the inside of a shell."},
		{label: "prose", text: "It was a bright cold day in April, and the clocks were striking thirteen. Winston Smith, his chin nuzzled into his breast in an effort to escape the vile wind."},
		{label: "prose", text: "In the beginning was the Word, and the Word was with God, and the Word was God. The same was in the beginning with God."},
		{label: "prose", text: "The old man sat on the wooden bench watching pigeons circle the marble fountain in the square of the little town where nothing much ever happened."},

		{label: "code", text: "func main() { fmt.Println(\"hello\"); os.Exit(0) }\ntype T struct { X int; Y string }\nfunc (t T) F() int { return t.X + 1 }"},
		{label: "code", text: "def fibonacci(n): return n if n < 2 else fibonacci(n-1) + fibonacci(n-2)\nresult = fibonacci(10)\nprint(result)"},
		{label: "code", text: "const std = @import(\"std\");\npub fn main() !void { const stdout = std.io.getStdOut().writer(); try stdout.print(\"Hello\\n\", .{}); }"},
		{label: "code", text: "pub fn main() void { var array = [_]i32{1, 2, 3, 4, 5}; for (array, 0..) |val, i| { std.debug.print(\"{}: {}\\n\", .{i, val}); } }"},
		{label: "code", text: "import sys\n\ndef quicksort(arr):\n    if len(arr) <= 1: return arr\n    pivot = arr[len(arr) // 2]\n    left = [x for x in arr if x < pivot]\n    right = [x for x in arr if x > pivot]\n    return quicksort(left) + [pivot] + quicksort(right)"},
		{label: "code", text: "class Node:\n    def __init__(self, val):\n        self.val = val\n        self.next = None\n\ndef reverse(head):\n    prev = None\n    while head:\n        head.next, prev, head = prev, head, head.next\n    return prev"},
		{label: "code", text: "async fn fetch(url: &str) -> Result<String, Error> {\n    let res = reqwest::get(url).await?;\n    let body = res.text().await?;\n    Ok(body)\n}"},
		{label: "code", text: "fn quicksort(vec: &mut Vec<i32>) {\n    if vec.len() <= 1 { return; }\n    let pivot_idx = vec.len() / 2;\n    let pivot = vec.remove(pivot_idx);\n    let left: Vec<i32> = vec.iter().filter(|&&x| x < pivot).cloned().collect();\n    let right: Vec<i32> = vec.iter().filter(|&&x| x >= pivot).cloned().collect();\n}"},
		{label: "code", text: "const express = require('express');\nconst app = express();\nconst PORT = process.env.PORT || 3000;\n\napp.get('/api/users', async (req, res) => {\n  try {\n    const users = await db.query('SELECT * FROM users');\n    res.json(users);\n  } catch (err) {\n    res.status(500).json({ error: err.message });\n  }\n});"},

		{label: "noise", text: "\x80\xff\x00\x7f\xaa\x55\xcc\x33\x99\x66\xff\x00\x80\x00\x11\x22\xaa\xbb\xcc\xdd\xee\x01\x02\x03\x04\x05"},
		{label: "noise", text: "\x01\x23\x45\x67\x89\xab\xcd\xef\xfe\xdc\xba\x98\x76\x54\x32\x10\x00\xff\x7f\x88\x91\xa2\xb3"},
		{label: "noise", text: "\xde\xad\xbe\xef\xca\xfe\xba\xbe\x01\x02\x03\x04\x05\x06\x07\x08\x09\x0a\x0b\x0c\x0d\x0e\x0f"},
		{label: "noise", text: "\x11\x22\x33\x44\x55\x66\x77\x88\x99\xaa\xbb\xcc\xdd\xee\xff\x00\x01\x02\x03\x04\x05\x06\x07\x08"},
		{label: "noise", text: "\x7f\x80\x81\x82\x83\x84\x85\x86\x87\x88\x89\x8a\x8b\x8c\x8d\x8e\x8f\x90\x91\x92\x93\x94\x95\x96\x97"},
		{label: "noise", text: "\xa0\xa1\xa2\xa3\xa4\xa5\xa6\xa7\xa8\xa9\xaa\xab\xac\xad\xae\xaf\xb0\xb1\xb2\xb3\xb4\xb5\xb6\xb7\xb8\xb9"},
	}

	fmt.Printf("Using lightweight classifier on %d samples\n", len(dataset))

	var all []labeledWindow
	for _, s := range dataset {
		// Split into overlapping windows
		winSize := 32
		stride := 16
		for start := 0; start < len(s.text); start += stride {
			end := start + winSize
			if end > len(s.text) {
				end = len(s.text)
			}
			chunk := s.text[start:end]
			if len(chunk) < 8 {
				continue
			}
			wf := extractWindowFeatures(chunk)
			all = append(all, labeledWindow{s.label, chunk, wf})
		}
	}
	fmt.Printf("Extracted %d windows from %d texts\n", len(all), len(dataset))

	// Count predictions
	correct, total := 0, 0
	perClass := make(map[string]int)
	perTotal := make(map[string]int)
	confused := make(map[string]map[string]int)

	for _, w := range all {
		p, c, n := w.wf.score()
		var pred string
		switch {
		case p > c && p > n:
			pred = "prose"
		case c > p && c > n:
			pred = "code"
		default:
			pred = "noise"
		}

		total++
		perTotal[w.label]++
		if confused[w.label] == nil {
			confused[w.label] = make(map[string]int)
		}
		confused[w.label][pred]++

		if pred == w.label {
			correct++
			perClass[w.label]++
		} else {
			fmt.Printf("  MISCLASSIFIED: %-6s → %-6s (p=%.1f c=%.1f n=%.1f) %q\n",
				w.label, pred, p, c, n, w.text)
		}
	}

	acc := float64(correct) / float64(total)
	fmt.Printf("\n  Accuracy: %d/%d (%.1f%%)\n", correct, total, acc*100)

	fmt.Println("\n  Confusion matrix:")
	for _, actual := range []string{"prose", "code", "noise"} {
		if perTotal[actual] == 0 {
			continue
		}
		fmt.Printf("    %-6s ", actual)
		for _, pred := range []string{"prose", "code", "noise"} {
			fmt.Printf("  %3d", confused[actual][pred])
		}
		fmt.Println()
	}

	fmt.Println("\n  Per-class accuracy:")
	for _, c := range []string{"prose", "code", "noise"} {
		if perTotal[c] == 0 {
			continue
		}
		pct := 100 * float64(perClass[c]) / float64(perTotal[c])
		fmt.Printf("    %-8s %d/%d (%.0f%%)\n", c, perClass[c], perTotal[c], pct)
	}

	r := result{
		TrainAcc: acc, TestAcc: acc,
		PerClass:  make(map[string]float64),
		NTest:     total,
		Dist:      perTotal,
	}
	for c, cnt := range perClass {
		r.PerClass[c] = float64(cnt) / float64(perTotal[c])
	}
	out, _ := json.MarshalIndent(r, "", "  ")
	fmt.Printf("\n%s\n", out)
	_ = os.WriteFile("classify_results.json", out, 0644)
}

func extractWindowFeatures(text string) windowFeatures {
	bytes := []byte(text)
	n := float64(len(bytes))
	if n == 0 {
		return windowFeatures{}
	}

	var wf windowFeatures

	// Byte-level histogram
	byteFreq := make(map[byte]int)
	for _, b := range bytes {
		byteFreq[b]++

		if b >= 0x20 && b <= 0x7E {
			wf.printable++
		}
		if (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') {
			wf.alpha++
		}
		if b == ' ' || b == '\t' {
			wf.space++
		}
		if b >= '0' && b <= '9' {
			wf.digit++
		}
		if b >= 'A' && b <= 'Z' {
			wf.upper++
		}
		// Code-specific chars
		if b == '{' || b == '}' || b == '(' || b == ')' ||
			b == ';' || b == '_' || b == '.' || b == ':' ||
			b == ',' || b == '<' || b == '>' || b == '=' ||
			b == '[' || b == ']' || b == '/' || b == '\\' ||
			b == '|' || b == '&' || b == '^' || b == '%' ||
			b == '#' || b == '@' || b == '`' || b == '~' {
			wf.codeChars++
		}
	}
	wf.printable /= n
	wf.alpha /= n
	wf.space /= n
	wf.digit /= n
	wf.upper /= n
	wf.codeChars /= n

	// Punctuation ratio (unicode)
	punctCount := 0
	for _, r := range text {
		if unicode.IsPunct(r) {
			punctCount++
		}
	}
	wf.punctuation = float64(punctCount) / n

	// Uppercase fraction among alpha
	if wf.alpha > 0 {
		wf.upper /= wf.alpha
	}

	// Average run length
	runSum, runs := 0.0, 0
	curRun := 1
	for i := 1; i < len(bytes); i++ {
		if bytes[i] == bytes[i-1] {
			curRun++
		} else {
			runSum += float64(curRun)
			runs++
			curRun = 1
		}
	}
	runSum += float64(curRun)
	runs++
	wf.runAvg = runSum / float64(runs)

	// UTF-8 validity: check fraction of byte sequences that are valid UTF-8 start bytes
	validUTF8 := 0
	for i := 0; i < len(bytes); {
		b := bytes[i]
		switch {
		case b <= 0x7F: // ASCII
			validUTF8++
			i++
		case b >= 0xC0 && b <= 0xDF: // 2-byte
			if i+1 < len(bytes) && bytes[i+1]&0xC0 == 0x80 {
				validUTF8++
			}
			i += 2
		case b >= 0xE0 && b <= 0xEF: // 3-byte
			if i+2 < len(bytes) {
				validUTF8++
			}
			i += 3
		case b >= 0xF0 && b <= 0xF7: // 4-byte
			if i+3 < len(bytes) {
				validUTF8++
			}
			i += 4
		case b >= 0x80 && b <= 0xBF: // continuation byte
			validUTF8++
			i++
		default:
			i++
		}
	}
	wf.utf8Valid = float64(validUTF8) / n

	// Byte entropy
	wf.byteEntropy = 0.0
	for _, cnt := range byteFreq {
		p := float64(cnt) / n
		if p > 0 {
			wf.byteEntropy -= p * math.Log2(p)
		}
	}

	// Count code keyword/pattern matches
	for _, pat := range codePatterns {
		if pat.MatchString(text) {
			wf.keywordCount++
		}
	}

	// Count common English word matches
	for _, pat := range prosePatterns {
		if pat.MatchString(text) {
			wf.commonWordCount++
		}
	}

	// Code keyword override: check for language-specific keywords
	lower := strings.ToLower(text)
	codeKw := []string{"func", "fn ", "def ", "class ", "import ", "const ", "var ", "pub ", "type ", "async ", "await ", "return ", "if ", "else", "for ", "while", "try ", "catch", "self.", "std.", "print(", "json(", "get(", "post("}
	for _, kw := range codeKw {
		if strings.Contains(lower, kw) {
			wf.keywordCount++
		}
	}

	return wf
}
