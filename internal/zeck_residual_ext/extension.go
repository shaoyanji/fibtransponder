package zkrext

import (
	"fmt"

	"github.com/shaoyanji/fibtransponder/internal/extension"
	"github.com/shaoyanji/fibtransponder/internal/fsvm"
)

// Estimator tracks the Zeckendorf residual using a FIFO ring buffer.
// Low residual = structured content. High = noise.
type Estimator struct {
	buf   []uint8
	size  int
	pos   int // next write position
	count int // how many slots are filled (capped at size)
	resid float64
	out   extension.Output
}

// NewEstimator creates an estimator with a default 128-bit window.
func NewEstimator() *Estimator {
	return NewEstimatorWithSize(128)
}

// NewEstimatorWithSize creates an estimator with a custom window size.
func NewEstimatorWithSize(windowSize int) *Estimator {
	if windowSize < 2 {
		windowSize = 2
	}
	return &Estimator{
		buf:  make([]uint8, windowSize),
		size: windowSize,
	}
}

func (e *Estimator) GetTitle() string {
	return "Zeckendorf Residual"
}

func (e *Estimator) ProcessBit(b uint8, fsvmState fsvm.State, zeroRunLength uint64, events []fsvm.Event) {
	e.buf[e.pos] = b
	e.pos = (e.pos + 1) % e.size
	if e.count < e.size {
		e.count++
	}

	// Recompute residual for the linear portion of the ring
	if e.count >= 2 {
		adj11 := 0
		for i := 1; i < e.count; i++ {
			prev := (e.pos - e.count + i + e.size) % e.size
			curr := (e.pos - e.count + i + 1 + e.size) % e.size
			if e.buf[prev]&1 != 0 && e.buf[curr]&1 != 0 {
				adj11++
			}
		}
		e.resid = float64(adj11) / float64(e.count-1)
	} else {
		e.resid = 0
	}

	e.out = e.GetOutput()
}

func (e *Estimator) Residual() float64 {
	return e.resid
}

func (e *Estimator) GetOutput() extension.Output {
	return extension.Output{
		Title: e.GetTitle(),
		Lines: []string{
			fmt.Sprintf("  Residual: %.4f", e.resid),
			fmt.Sprintf("  Window:   %d/%d", e.count, e.size),
		},
	}
}
