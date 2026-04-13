package transponder

import (
	"fmt"
	"math"
	"strings"
	"testing"
	"unicode/utf8"
)

// ── Language Sensing Experiment (Path A) ──
//
// Goal: demonstrate that a fibtransponder array operating on UTF-8
// bitstreams produces structural signals that distinguish natural
// language text from code and from each other — without any learned
// vocabulary, without BPE, without tokenization.
//
// Method:
//   1. Encode text as UTF-8 bytes → bitstream
//   2. Feed through array with varied (width, threshold) calibrations
//   3. Extract per-window event profiles as "structural tokens"
//   4. Measure inter-class separability vs intra-class stability
//
// This is the experiment that connects the FSVM to NLP.

// ── Real-world text corpora ──

// English prose (Wikipedia-style)
var corpusEnglish = []byte(`The Fibonacci sequence is a series of numbers where each number is the sum of the two preceding ones. It starts from 0 and 1, and continues indefinitely. The sequence appears throughout nature, from the arrangement of leaves on a stem to the spiral of a nautilus shell. In mathematics, the ratio between consecutive Fibonacci numbers converges to the golden ratio, approximately 1.618. This ratio appears in art, architecture, and music. The sequence was introduced to Western European mathematics by Leonardo of Pisa, known as Fibonacci, in his 1202 book Liber Abaci. However, the sequence had been described earlier in Indian mathematics. The connection between Fibonacci numbers and the golden ratio is profound: as you go further in the sequence, the ratio of consecutive numbers gets closer and closer to phi. This property makes Fibonacci numbers useful in algorithms, data structures, and computational methods. Binary search trees, heaps, and hash tables all use properties related to these numbers.`)

// French prose (similar topic, different language structure)
var corpusFrench = []byte(`La suite de Fibonacci est une suite d'entiers dans laquelle chaque terme est la somme des deux termes qui le précèdent. Elle commence par 0 et 1, et se poursuit indéfiniment. La suite apparaît dans toute la nature, de la disposition des feuilles sur une tige à la spirale d'un coquillage nautile. En mathématiques, le rapport entre les nombres de Fibonacci consécutifs converge vers le nombre d'or, environ 1,618. Ce rapport apparaît dans l'art, l'architecture et la musique. La suite a été introduite dans les mathématiques européennes par Leonardo de Pise, connu sous le nom de Fibonacci, dans son livre de 1202 Liber Abaci. Cependant, la suite avait été décrite plus tôt dans les mathématiques indiennes. La connexion entre les nombres de Fibonacci et le nombre d'or est profonde. Cette propriété rend les nombres de Fibonacci utiles dans les algorithmes et les structures de données.`)

// Python source code
var corpusPython = []byte(`def fibonacci(n):
    if n <= 1:
        return n
    a, b = 0, 1
    for i in range(2, n + 1):
        a, b = b, a + b
    return b

def build_tree(values):
    if not values:
        return None
    root = Node(values[0])
    for v in values[1:]:
        insert(root, v)
    return root

def insert(node, val):
    if val < node.value:
        if node.left is None:
            node.left = Node(val)
        else:
            insert(node.left, val)
    else:
        if node.right is None:
            node.right = Node(val)
        else:
            insert(node.right, val)

def traverse(node, depth=0):
    if node is None:
        return
    traverse(node.left, depth + 1)
    print(f"{'  ' * depth}{node.value}")
    traverse(node.right, depth + 1)

if __name__ == "__main__":
    import sys
    n = int(sys.argv[1])
    print(f"fib({n}) = {fibonacci(n)}")
    values = [8, 3, 10, 1, 6, 14, 4, 7, 13]
    tree = build_tree(values)
    traverse(tree)`)

// JSON data
var corpusJSON = []byte(`{"name":"Fibonacci","description":"A mathematical sequence","properties":{"first_terms":[0,1,1,2,3,5,8,13,21,34],"ratio_limit":1.618033988749895,"discovered_by":"Leonardo of Pisa","year":1202,"applications":["algorithms","data_structures","nature","art","architecture"],"related_concepts":["golden_ratio","zeckendorf_theorem","lucas_numbers","pascals_triangle"]},"metadata":{"version":"1.0","author":"research","tags":["math","sequences","fibonacci","number_theory"],"created":"2026-01-01T00:00:00Z","modified":"2026-04-13T12:00:00Z"},"statistics":{"total_terms":10,"even_terms":[0,2,8,34],"odd_terms":[1,1,3,5,13,21],"sum":88,"mean":8.8,"median":4.0}}`)

// HTML markup
var corpusHTML = []byte(`<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Fibonacci Sequence</title>
    <style>
        body { font-family: serif; margin: 2rem; }
        .sequence { font-family: monospace; }
        .highlight { color: goldenrod; }
    </style>
</head>
<body>
    <h1>The Fibonacci Sequence</h1>
    <p class="sequence">0, 1, 1, 2, 3, 5, 8, 13, 21, 34, 55, 89, 144, 233, 377</p>
    <p>The <span class="highlight">golden ratio</span> &phi; &asymp; 1.618</p>
    <script>
        function fib(n) {
            let a = 0, b = 1;
            for (let i = 2; i <= n; i++) {
                [a, b] = [b, a + b];
            }
            return b;
        }
        console.log(fib(10));
    </script>
</body>
</html>`)

// ── Window-level structural profile ──

// StructuralProfile captures what one transponder "sees" in a window.
type StructuralProfile struct {
	TransponderName string
	WindowBits      int
	Dilations       uint64
	Markers         uint64
	DilateRate      float64
	MarkerRate      float64
	Sketch          uint64
}

// CorpusProfile is the full array's view of one corpus.
type CorpusProfile struct {
	Label     string
	ByteCount int
	BitCount  int
	Profiles  []StructuralProfile // one per transponder
}

func computeCorpusProfile(label string, data []byte, cals []JointCalibration) CorpusProfile {
	bits := BytesToBits(data)
	arr := NewJointArray(cals)
	arr.ProcessStream(bits)

	profiles := make([]StructuralProfile, len(cals))
	for i, t := range arr.Transponders {
		profiles[i] = StructuralProfile{
			TransponderName: t.Name,
			WindowBits:      len(bits),
			Dilations:       t.State.Dilations,
			Markers:         t.State.Markers,
			DilateRate:      float64(t.State.Dilations) / float64(len(bits)),
			MarkerRate:      float64(t.State.Markers) / float64(len(bits)),
			Sketch:          t.State.Sketch,
		}
	}

	return CorpusProfile{
		Label:     label,
		ByteCount: len(data),
		BitCount:  len(bits),
		Profiles:  profiles,
	}
}

// cosineSimilarity computes cosine similarity between two rate vectors.
func cosineSimilarity(a, b []float64) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}
	var dot, normA, normB float64
	for i := range a {
		dot += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}
	if normA == 0 || normB == 0 {
		return 0
	}
	return dot / (math.Sqrt(normA) * math.Sqrt(normB))
}

func TestLanguageSensing(t *testing.T) {
	// Use the expanded threshold families for richer signal
	cals := []JointCalibration{
		{"w1+pow2", Width1, ThresholdDefault},
		{"w1+pow3", Width1, ThresholdPow3},
		{"w1+lin8", Width1, ThresholdLinear8},
		{"w2+pow2", Width2, ThresholdDefault},
		{"w2+pow3", Width2, ThresholdPow3},
		{"w2+lin8", Width2, ThresholdLinear8},
		{"w3+pow2", Width3, ThresholdDefault},
		{"w3+pow3", Width3, ThresholdPow3},
		{"w3+lin8", Width3, ThresholdLinear8},
	}

	corpus := []struct {
		label string
		data  []byte
	}{
		{"english", corpusEnglish},
		{"french", corpusFrench},
		{"python", corpusPython},
		{"json", corpusJSON},
		{"html", corpusHTML},
	}

	// Compute profiles
	profiles := make([]CorpusProfile, len(corpus))
	for i, c := range corpus {
		profiles[i] = computeCorpusProfile(c.label, c.data, cals)
	}

	// ══════════════════════════════════════════════════════════════
	t.Log("═══════════════════════════════════════════════════════════════")
	t.Log("LANGUAGE SENSING: structural profiles across text types")
	t.Log("═══════════════════════════════════════════════════════════════")

	// Print corpus sizes
	t.Log("")
	t.Log("Corpus sizes:")
	for _, p := range profiles {
		t.Logf("  %-10s  %5d bytes = %6d bits (UTF-8)", p.Label, p.ByteCount, p.BitCount)
	}

	// DILATE rate matrix
	t.Log("")
	t.Log("── DILATE rate per (transponder × corpus) ──")
	header := fmt.Sprintf("%-12s", "transponder")
	for _, p := range profiles {
		header += fmt.Sprintf("  %10s", p.Label)
	}
	t.Log(header)

	for ti := range cals {
		row := fmt.Sprintf("%-12s", cals[ti].Name)
		for _, p := range profiles {
			row += fmt.Sprintf("  %10.6f", p.Profiles[ti].DilateRate)
		}
		t.Log(row)
	}

	// MARKER rate matrix
	t.Log("")
	t.Log("── MARKER rate per (transponder × corpus) ──")
	header = fmt.Sprintf("%-12s", "transponder")
	for _, p := range profiles {
		header += fmt.Sprintf("  %10s", p.Label)
	}
	t.Log(header)

	for ti := range cals {
		row := fmt.Sprintf("%-12s", cals[ti].Name)
		for _, p := range profiles {
			row += fmt.Sprintf("  %10.6f", p.Profiles[ti].MarkerRate)
		}
		t.Log(row)
	}

	// ══════════════════════════════════════════════════════════════
	// SEPARABILITY ANALYSIS
	// ══════════════════════════════════════════════════════════════

	t.Log("")
	t.Log("═══════════════════════════════════════════════════════════════")
	t.Log("SEPARABILITY: cosine distance between corpus profiles")
	t.Log("═══════════════════════════════════════════════════════════════")

	// Build rate vectors for each corpus (concatenate dil-rate + mark-rate across transponders)
	rateVectors := make(map[string][]float64)
	for _, p := range profiles {
		vec := make([]float64, 0, len(cals)*2)
		for _, prof := range p.Profiles {
			vec = append(vec, prof.DilateRate)
			vec = append(vec, prof.MarkerRate)
		}
		rateVectors[p.Label] = vec
	}

	// Compute pairwise cosine similarities
	t.Log("")
	t.Log("Pairwise cosine similarity (1.0 = identical, 0.0 = orthogonal):")
	t.Logf("%-12s", "")
	for _, p := range profiles {
		t.Logf("  %10s", p.Label)
	}
	t.Log("")

	for i, pi := range profiles {
		row := fmt.Sprintf("%-12s", pi.Label)
		for j, pj := range profiles {
			sim := cosineSimilarity(rateVectors[pi.Label], rateVectors[pj.Label])
			if i == j {
				row += fmt.Sprintf("  %10s", "---")
			} else {
				row += fmt.Sprintf("  %10.4f", sim)
			}
		}
		t.Log(row)
	}

	// Key comparisons
	t.Log("")
	t.Log("Key comparisons:")
	engFr := cosineSimilarity(rateVectors["english"], rateVectors["french"])
	engPy := cosineSimilarity(rateVectors["english"], rateVectors["python"])
	frPy := cosineSimilarity(rateVectors["french"], rateVectors["python"])
	engJSON := cosineSimilarity(rateVectors["english"], rateVectors["json"])
	pyJSON := cosineSimilarity(rateVectors["python"], rateVectors["json"])

	t.Logf("  English vs French:  %.4f  (same language family, similar structure)", engFr)
	t.Logf("  English vs Python:  %.4f  (natural language vs code)", engPy)
	t.Logf("  French vs Python:   %.4f  (natural language vs code)", frPy)
	t.Logf("  English vs JSON:    %.4f  (prose vs structured data)", engJSON)
	t.Logf("  Python vs JSON:     %.4f  (code vs structured data)", pyJSON)

	// ══════════════════════════════════════════════════════════════
	// BPE COMPARISON (information-theoretic)
	// ══════════════════════════════════════════════════════════════

	t.Log("")
	t.Log("═══════════════════════════════════════════════════════════════")
	t.Log("INFORMATION DENSITY: bytes per structural event")
	t.Log("═══════════════════════════════════════════════════════════════")

	t.Log("")
	t.Log("BPE baseline: ~4-5 bytes per token for English (typical GPT vocab)")
	t.Log("")
	t.Logf("%-10s  %8s  %8s  %8s  %12s", "corpus", "bytes", "events", "dil-only", "bytes/event")
	for _, p := range profiles {
		// Use w1+pow2 transponder as representative
		prof := p.Profiles[0] // first transponder = w1+pow2
		events := prof.Dilations + prof.Markers
		bpeBytesPerToken := 4.5 // typical BPE
		var fsvmBytesPerEvent float64
		if events > 0 {
			fsvmBytesPerEvent = float64(p.ByteCount) / float64(events)
		}
		t.Logf("%-10s  %8d  %8d  %8d  %12.2f  (BPE: ~%.1f)",
			p.Label, p.ByteCount, events, prof.Dilations,
			fsvmBytesPerEvent, bpeBytesPerToken)
	}

	// ══════════════════════════════════════════════════════════════
	// UTF-8 STRUCTURE ANALYSIS
	// ══════════════════════════════════════════════════════════════

	t.Log("")
	t.Log("═══════════════════════════════════════════════════════════════")
	t.Log("UTF-8 STRUCTURE: why different text types produce different signals")
	t.Log("═══════════════════════════════════════════════════════════════")

	for _, c := range corpus {
		asciiCount := 0
		utf8MultiByte := 0
		spaceCount := 0
		newlineCount := 0
		punctuationCount := 0

		for _, r := range string(c.data) {
			if r < 128 {
				asciiCount++
			} else {
				utf8MultiByte++
			}
			if r == ' ' {
				spaceCount++
			}
			if r == '\n' {
				newlineCount++
			}
			if strings.ContainsRune(".,;:!?(){}[]\"'<>", r) {
				punctuationCount++
			}
		}

		totalRunes := utf8.RuneCount(c.data)
		t.Logf("")
		t.Logf("  %s:", c.label)
		t.Logf("    %d runes (%d ASCII, %d multi-byte UTF-8)", totalRunes, asciiCount, utf8MultiByte)
		t.Logf("    %d spaces (%.1f%%), %d newlines, %d punctuation",
			spaceCount, 100*float64(spaceCount)/float64(totalRunes),
			newlineCount, punctuationCount)
		t.Logf("    UTF-8 overhead: %.1f%% bytes beyond ASCII",
			100*float64(len(c.data)-asciiCount)/float64(len(c.data)))
	}
}
