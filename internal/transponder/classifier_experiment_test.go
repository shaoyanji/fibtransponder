package transponder

import (
	"fmt"
	"math"
	"testing"

	"github.com/shaoyanji/fibtransponder/internal/fsvm"
)

// ── Classification Experiment (Path A proof-of-concept) ──
//
// Goal: show that FSVM array features can classify text type
// (prose vs code vs structured) with higher accuracy than
// simple byte-frequency features.
//
// Method:
//   1. Split corpora into chunks (~200 bytes each)
//   2. Extract two feature sets per chunk:
//      a. FSVM features: dilate rates across transponders + temporal stats
//      b. Byte features: ASCII class frequencies (letters, digits, space, punct, newline)
//   3. Train nearest-centroid classifier on half, test on half
//   4. Compare accuracy

const chunkSize = 200 // bytes per sample

// FeatureSet holds features for one chunk.
type FeatureSet struct {
	Label    string
	Features []float64
}

// splitIntoChunks splits a byte slice into non-overlapping chunks.
func splitIntoChunks(data []byte, size int) [][]byte {
	var chunks [][]byte
	for i := 0; i < len(data); i += size {
		end := i + size
		if end > len(data) {
			end = len(data)
		}
		chunk := make([]byte, end-i)
		copy(chunk, data[i:end])
		chunks = append(chunks, chunk)
	}
	return chunks
}

// extractFSVMFeatures runs the chunk through a multi-transponder array
// and returns a feature vector.
func extractFSVMFeatures(data []byte) []float64 {
	bits := BytesToBits(data)

	// Use 6 transponders: 3 widths × 2 thresholds (pow2, lin8)
	cals := []JointCalibration{
		{"w1+pow2", Width1, ThresholdDefault},
		{"w1+lin8", Width1, ThresholdLinear8},
		{"w2+pow2", Width2, ThresholdDefault},
		{"w2+lin8", Width2, ThresholdLinear8},
		{"w3+pow2", Width3, ThresholdDefault},
		{"w3+lin8", Width3, ThresholdLinear8},
	}

	arr := NewJointArray(cals)

	// Windowed temporal tracking
	const winSize = 64 // small windows for fine-grained profiles
	nWin := (len(bits) + winSize - 1) / winSize
	if nWin < 1 {
		nWin = 1
	}

	windowDil := make([][]int, len(cals))
	for i := range windowDil {
		windowDil[i] = make([]int, nWin)
	}

	for bitIdx, b := range bits {
		wi := bitIdx / winSize
		if wi >= nWin {
			wi = nWin - 1
		}
		for ti := range arr.Transponders {
			t := &arr.Transponders[ti]
			prevDil := t.State.Dilations
			var evs []fsvm.Event
			t.State, evs = StepFull(t.State, b, t.Width, t.Thresh)
			_ = evs
			windowDil[ti][wi] += int(t.State.Dilations - prevDil)
		}
	}

	// Build feature vector:
	// - 6 aggregate dilate rates (one per transponder)
	// - 6 aggregate marker rates
	// - 6 temporal variances (one per transponder)
	// - 6 temporal skewness values
	// Total: 24 features
	features := make([]float64, 0, 24)

	for ti := range arr.Transponders {
		t := arr.Transponders[ti]
		dilRate := float64(t.State.Dilations) / float64(len(bits))
		markRate := float64(t.State.Markers) / float64(len(bits))
		features = append(features, dilRate, markRate)
	}

	for ti := range windowDil {
		rates := make([]float64, nWin)
		bitsInWin := winSize
		for w := 0; w < nWin; w++ {
			if w == nWin-1 && len(bits)%winSize != 0 {
				bitsInWin = len(bits) % winSize
			}
			if bitsInWin > 0 {
				rates[w] = float64(windowDil[ti][w]) / float64(bitsInWin)
			}
		}

		mean := 0.0
		for _, r := range rates {
			mean += r
		}
		mean /= float64(len(rates))

		variance := 0.0
		skewness := 0.0
		for _, r := range rates {
			d := r - mean
			variance += d * d
		}
		variance /= float64(len(rates))
		stddev := math.Sqrt(variance)
		if stddev > 0 {
			for _, r := range rates {
				d := (r - mean) / stddev
				skewness += d * d * d
			}
			skewness /= float64(len(rates))
		}

		features = append(features, variance, skewness)
	}

	return features
}

// extractByteFeatures computes simple byte-class frequency features.
func extractByteFeatures(data []byte) []float64 {
	letters := 0
	digits := 0
	spaces := 0
	newlines := 0
	punctuation := 0
	upper := 0
	other := 0
	total := len(data)

	for _, b := range data {
		switch {
		case b >= 'a' && b <= 'z':
			letters++
		case b >= 'A' && b <= 'Z':
			letters++
			upper++
		case b >= '0' && b <= '9':
			digits++
		case b == ' ':
			spaces++
		case b == '\n' || b == '\r':
			newlines++
		case b == '.' || b == ',' || b == ';' || b == ':' || b == '!' || b == '?':
			punctuation++
		case b == '(' || b == ')' || b == '{' || b == '}' || b == '[' || b == ']' || b == '<' || b == '>':
			punctuation++
		case b == '+' || b == '-' || b == '*' || b == '=' || b == '/' || b == '\\' || b == '#' || b == '@':
			punctuation++
		default:
			other++
		}
	}

	n := float64(total)
	return []float64{
		float64(letters) / n,
		float64(digits) / n,
		float64(spaces) / n,
		float64(newlines) / n,
		float64(punctuation) / n,
		float64(upper) / n,
		float64(other) / n,
	}
}

// nearestCentroid classifies by closest class centroid.
type centroidClassifier struct {
	centroids map[string][]float64
}

func trainCentroidClassifier(samples []FeatureSet) centroidClassifier {
	classSums := make(map[string][]float64)
	classCounts := make(map[string]int)

	for _, s := range samples {
		if _, ok := classSums[s.Label]; !ok {
			classSums[s.Label] = make([]float64, len(s.Features))
		}
		for i, f := range s.Features {
			classSums[s.Label][i] += f
		}
		classCounts[s.Label]++
	}

	centroids := make(map[string][]float64)
	for label, sums := range classSums {
		centroid := make([]float64, len(sums))
		for i, s := range sums {
			centroid[i] = s / float64(classCounts[label])
		}
		centroids[label] = centroid
	}

	return centroidClassifier{centroids: centroids}
}

func (c centroidClassifier) classify(features []float64) string {
	bestDist := math.MaxFloat64
	bestLabel := ""
	for label, centroid := range c.centroids {
		dist := 0.0
		for i := range features {
			if i < len(centroid) {
				d := features[i] - centroid[i]
				dist += d * d
			}
		}
		dist = math.Sqrt(dist)
		if dist < bestDist {
			bestDist = dist
			bestLabel = label
		}
	}
	return bestLabel
}

// normalizeFeatures z-normalizes features across all samples.
func normalizeFeatures(samples []FeatureSet) {
	if len(samples) == 0 {
		return
	}
	nFeat := len(samples[0].Features)
	means := make([]float64, nFeat)
	stddevs := make([]float64, nFeat)

	for _, s := range samples {
		for i, f := range s.Features {
			means[i] += f
		}
	}
	for i := range means {
		means[i] /= float64(len(samples))
	}

	for _, s := range samples {
		for i, f := range s.Features {
			d := f - means[i]
			stddevs[i] += d * d
		}
	}
	for i := range stddevs {
		stddevs[i] = math.Sqrt(stddevs[i] / float64(len(samples)))
		if stddevs[i] == 0 {
			stddevs[i] = 1
		}
	}

	for i := range samples {
		for j := range samples[i].Features {
			samples[i].Features[j] = (samples[i].Features[j] - means[j]) / stddevs[j]
		}
	}
}

func TestClassificationExperiment(t *testing.T) {
	// Use 3 classes for clearer signal
	corpus := []struct {
		label string
		data  []byte
	}{
		{"prose", corpusEnglish},
		{"code", corpusPython},
		{"structured", corpusJSON},
	}

	// Split into chunks and extract features
	var fsvmSamples []FeatureSet
	var byteSamples []FeatureSet

	for _, c := range corpus {
		chunks := splitIntoChunks(c.data, chunkSize)
		for _, chunk := range chunks {
			if len(chunk) < chunkSize/2 {
				continue // skip tiny trailing chunks
			}
			fsvmFeat := extractFSVMFeatures(chunk)
			byteFeat := extractByteFeatures(chunk)
			fsvmSamples = append(fsvmSamples, FeatureSet{Label: c.label, Features: fsvmFeat})
			byteSamples = append(byteSamples, FeatureSet{Label: c.label, Features: byteFeat})
		}
	}

	t.Logf("Total samples: %d (per class: ~%d bytes / %d bytes per chunk)",
		len(fsvmSamples), len(corpus[0].data), chunkSize)

	// Print sample distribution
	classCounts := make(map[string]int)
	for _, s := range fsvmSamples {
		classCounts[s.Label]++
	}
	for label, count := range classCounts {
		t.Logf("  %s: %d chunks", label, count)
	}

	// Normalize features
	normalizeFeatures(fsvmSamples)
	normalizeFeatures(byteSamples)

	// Train/test split: first half of each class for training, second half for test
	// (preserves temporal order within each corpus)
	trainByLabel := make(map[string][]FeatureSet)
	testByLabel := make(map[string][]FeatureSet)

	for label := range classCounts {
		var fsvmForLabel []FeatureSet
		for _, s := range fsvmSamples {
			if s.Label == label {
				fsvmForLabel = append(fsvmForLabel, s)
			}
		}
		split := len(fsvmForLabel) / 2
		if split < 1 {
			split = 1
		}
		trainByLabel[label] = fsvmForLabel[:split]
		testByLabel[label] = fsvmForLabel[split:]
	}

	// Flatten for training
	var fsvmTrain, fsvmTest []FeatureSet
	for _, samples := range trainByLabel {
		fsvmTrain = append(fsvmTrain, samples...)
	}
	for _, samples := range testByLabel {
		fsvmTest = append(fsvmTest, samples...)
	}

	// Same split for byte features
	trainByLabelB := make(map[string][]FeatureSet)
	testByLabelB := make(map[string][]FeatureSet)
	for label := range classCounts {
		var byteForLabel []FeatureSet
		for _, s := range byteSamples {
			if s.Label == label {
				byteForLabel = append(byteForLabel, s)
			}
		}
		split := len(byteForLabel) / 2
		if split < 1 {
			split = 1
		}
		trainByLabelB[label] = byteForLabel[:split]
		testByLabelB[label] = byteForLabel[split:]
	}

	var byteTrain, byteTest []FeatureSet
	for _, samples := range trainByLabelB {
		byteTrain = append(byteTrain, samples...)
	}
	for _, samples := range testByLabelB {
		byteTest = append(byteTest, samples...)
	}

	// Train classifiers
	fsvmClf := trainCentroidClassifier(fsvmTrain)
	byteClf := trainCentroidClassifier(byteTrain)

	// Evaluate
	t.Log("")
	t.Log("═══════════════════════════════════════════════════════════════")
	t.Log("CLASSIFICATION RESULTS")
	t.Log("═══════════════════════════════════════════════════════════════")

	// FSVM accuracy
	fsvmCorrect := 0
	fsvmConfusion := make(map[string]map[string]int)
	for _, s := range fsvmTest {
		pred := fsvmClf.classify(s.Features)
		if pred == s.Label {
			fsvmCorrect++
		}
		if fsvmConfusion[s.Label] == nil {
			fsvmConfusion[s.Label] = make(map[string]int)
		}
		fsvmConfusion[s.Label][pred]++
	}
	fsvmAcc := float64(fsvmCorrect) / float64(len(fsvmTest))

	// Byte accuracy
	byteCorrect := 0
	byteConfusion := make(map[string]map[string]int)
	for _, s := range byteTest {
		pred := byteClf.classify(s.Features)
		if pred == s.Label {
			byteCorrect++
		}
		if byteConfusion[s.Label] == nil {
			byteConfusion[s.Label] = make(map[string]int)
		}
		byteConfusion[s.Label][pred]++
	}
	byteAcc := float64(byteCorrect) / float64(len(byteTest))

	t.Log("")
	t.Logf("  FSVM features:  %d/%d correct = %.1f%% accuracy", fsvmCorrect, len(fsvmTest), fsvmAcc*100)
	t.Logf("  Byte features:  %d/%d correct = %.1f%% accuracy", byteCorrect, len(byteTest), byteAcc*100)
	t.Logf("  Features used:  FSVM=%d, Byte=%d", len(fsvmSamples[0].Features), len(byteSamples[0].Features))

	// Confusion matrices
	t.Log("")
	t.Log("FSVM confusion matrix:")
	labels := []string{"prose", "code", "structured"}
	header := fmt.Sprintf("%-12s", "true\\pred")
	for _, l := range labels {
		header += fmt.Sprintf("  %10s", l)
	}
	t.Log(header)
	for _, trueLabel := range labels {
		row := fmt.Sprintf("%-12s", trueLabel)
		for _, predLabel := range labels {
			count := fsvmConfusion[trueLabel][predLabel]
			row += fmt.Sprintf("  %10d", count)
		}
		t.Log(row)
	}

	t.Log("")
	t.Log("Byte confusion matrix:")
	header = fmt.Sprintf("%-12s", "true\\pred")
	for _, l := range labels {
		header += fmt.Sprintf("  %10s", l)
	}
	t.Log(header)
	for _, trueLabel := range labels {
		row := fmt.Sprintf("%-12s", trueLabel)
		for _, predLabel := range labels {
			count := byteConfusion[trueLabel][predLabel]
			row += fmt.Sprintf("  %10d", count)
		}
		t.Log(row)
	}

	// Feature importance: which FSVM features vary most across classes?
	t.Log("")
	t.Log("── FSVM feature variance across class centroids ──")
	featureNames := []string{
		"w1+dil", "w1+mark", "w2+dil", "w2+mark", "w3+dil", "w3+mark",
		"w1+var", "w1+skew", "w1+var", "w1+skew", // temporal
		"w2+var", "w2+skew",
		"w3+var", "w3+skew",
		"w1l8+dil", "w1l8+mark", "w2l8+dil", "w2l8+mark", "w3l8+dil", "w3l8+mark",
		"w1l8+var", "w1l8+skew",
		"w2l8+var", "w2l8+skew",
	}
	for i, name := range featureNames {
		if i >= len(fsvmClf.centroids["prose"]) {
			break
		}
		vals := make([]float64, 0)
		for _, label := range labels {
			if i < len(fsvmClf.centroids[label]) {
				vals = append(vals, fsvmClf.centroids[label][i])
			}
		}
		// Compute range
		min, max := vals[0], vals[0]
		for _, v := range vals[1:] {
			if v < min {
				min = v
			}
			if v > max {
				max = v
			}
		}
		t.Logf("  %-12s  range=%.4f  vals=%v", name, max-min, vals)
	}

	// Statistical significance: repeat with different random splits
	t.Log("")
	t.Log("── Stability check: 5-fold sequential cross-validation ──")
	nFolds := 5

	// Rebuild full sample lists
	var allFSVM []FeatureSet
	for _, c := range corpus {
		chunks := splitIntoChunks(c.data, chunkSize)
		for _, chunk := range chunks {
			if len(chunk) < chunkSize/2 {
				continue
			}
			feat := extractFSVMFeatures(chunk)
			allFSVM = append(allFSVM, FeatureSet{Label: c.label, Features: feat})
		}
	}
	normalizeFeatures(allFSVM)

	// Group by label
	byLabel := make(map[string][]FeatureSet)
	for _, s := range allFSVM {
		byLabel[s.Label] = append(byLabel[s.Label], s)
	}

	totalCorrect := 0
	totalTest := 0
	for fold := 0; fold < nFolds; fold++ {
		var trainSet, testSet []FeatureSet
		for _, samples := range byLabel {
			n := len(samples)
			foldSize := n / nFolds
			if foldSize < 1 {
				foldSize = 1
			}
			start := fold * foldSize
			end := start + foldSize
			if fold == nFolds-1 {
				end = n
			}
			if start >= n {
				start = n - 1
			}
			if end > n {
				end = n
			}
			testSet = append(testSet, samples[start:end]...)
			trainSet = append(trainSet, samples[:start]...)
			trainSet = append(trainSet, samples[end:]...)
		}

		if len(trainSet) == 0 || len(testSet) == 0 {
			continue
		}

		clf := trainCentroidClassifier(trainSet)
		correct := 0
		for _, s := range testSet {
			if clf.classify(s.Features) == s.Label {
				correct++
			}
		}
		acc := float64(correct) / float64(len(testSet))
		totalCorrect += correct
		totalTest += len(testSet)
		t.Logf("  fold %d: %d/%d = %.1f%%", fold+1, correct, len(testSet), acc*100)
	}
	overallAcc := float64(totalCorrect) / float64(totalTest)
	t.Logf("  Overall: %d/%d = %.1f%%", totalCorrect, totalTest, overallAcc*100)

	// Random baseline
	t.Log("")
	t.Log("── Random baseline ──")
	correctByChance := 1.0 / float64(len(labels))
	t.Logf("  Random classifier (uniform): %.1f%%", correctByChance*100)

	// Summary
	t.Log("")
	t.Log("═══════════════════════════════════════════════════════════════")
	t.Log("SUMMARY")
	t.Log("═══════════════════════════════════════════════════════════════")
	t.Logf("  FSVM nearest-centroid:  %.1f%%", fsvmAcc*100)
	t.Logf("  Byte nearest-centroid:  %.1f%%", byteAcc*100)
	t.Logf("  Random baseline:        %.1f%%", correctByChance*100)
	if fsvmAcc > byteAcc {
		t.Log("  → FSVM features OUTPERFORM byte features")
	} else if fsvmAcc == byteAcc {
		t.Log("  → FSVM and byte features TIED")
	} else {
		t.Log("  → Byte features outperform FSVM features")
	}
}
