// Package nixeval caches full Nix evaluations and ordered, file-local updates.
package nixeval

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"sync"
)

// Evaluator returns the complete result for the current tracked working tree.
type Evaluator interface {
	Eval(context.Context, string, string) (json.RawMessage, error)
}

// Fragmenter implementations must be deterministic. Fragments are relative to
// base, not to preceding fragments. Merge receives them in ingest-log order.
// Changing an implementation requires a new CacheVersion.
type Fragmenter interface {
	Fragment(context.Context, string, []byte, json.RawMessage) (json.RawMessage, error)
	NeedsLeap(context.Context, string, []byte, json.RawMessage) (bool, error)
	Merge(context.Context, json.RawMessage, []json.RawMessage) (json.RawMessage, error)
}

// BaseEligibility optionally restricts eligible paths to those represented in
// the full result. Paths newly tracked outside this set invalidate the core.
type BaseEligibility interface {
	EligiblePaths(json.RawMessage) ([]string, error)
}

type NixEvaluator struct{}

func (NixEvaluator) Eval(ctx context.Context, repo, attr string) (json.RawMessage, error) {
	var last error
	for _, args := range [][]string{{"eval", "--json", attr}, {"eval", "--impure", "--json", attr}} {
		cmd := exec.CommandContext(ctx, "nix", args...)
		cmd.Dir = repo
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		out, err := cmd.Output()
		if err == nil && json.Valid(out) {
			return out, nil
		}
		last = fmt.Errorf("nix %v: %w: %s", args, err, stderr.String())
		if err == nil {
			last = errors.New("nix returned invalid JSON")
		}
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
	}
	return nil, last
}

// Channel describes one evaluation target. Eligible receives slash-separated
// repository-relative paths. EpochIndex defaults to 10 (F_10 = 55).
type Channel struct {
	Repo, Attr, CacheVersion string
	Eligible                 func(string) bool
	Fragmenter               Fragmenter
	Evaluator                Evaluator
	EpochIndex               int
}

type SegmentStatus struct {
	Term int    `json:"term"`
	Key  string `json:"key"`
	Hit  bool   `json:"hit"`
}

// Status describes the last successful Get or Leap, not a speculative scan.
type Status struct {
	Epoch       string          `json:"epoch"`
	N           int             `json:"n"`
	Terms       []int           `json:"terms"`
	Segments    []SegmentStatus `json:"segments"`
	QuotientHit bool            `json:"quotient_hit"`
	LeapReason  string          `json:"leap_reason,omitempty"`
}

type request struct {
	ctx   context.Context
	op    string
	reply chan response
}
type response struct {
	data   json.RawMessage
	path   string
	status Status
	err    error
}

// Engine owns one request-processing goroutine per registered channel. Close
// cancels outstanding work and waits for workers; callers must close engines.
type Engine struct {
	root    string
	mu      sync.RWMutex
	workers map[string]chan request
	ctx     context.Context
	cancel  context.CancelFunc
	wg      sync.WaitGroup
	closed  bool
}

func New(cacheDir string) *Engine {
	ctx, cancel := context.WithCancel(context.Background())
	return &Engine{root: cacheDir, workers: make(map[string]chan request), ctx: ctx, cancel: cancel}
}

func (e *Engine) Register(name string, c Channel) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.closed {
		return errors.New("engine closed")
	}
	if _, ok := e.workers[name]; ok {
		return fmt.Errorf("channel %q already registered", name)
	}
	if name == "" || c.Eligible == nil || c.Fragmenter == nil || c.Repo == "" || c.Attr == "" {
		return errors.New("channel requires name, repo, attr, eligible and fragmenter")
	}
	if c.EpochIndex == 0 {
		c.EpochIndex = 10
	}
	// The shared Zeckendorf helper represents all logs below F_34.
	if c.EpochIndex < 2 || c.EpochIndex > 34 {
		return errors.New("epoch index must be between 2 and 34")
	}
	var err error
	c.Repo, err = filepath.Abs(c.Repo)
	if err != nil {
		return err
	}
	if c.Evaluator == nil {
		c.Evaluator = NixEvaluator{}
	}
	identity := digest([]string{name, c.Repo, c.Attr, c.CacheVersion})
	w := &worker{config: c, dir: filepath.Join(e.root, identity), stats: make(map[string]statSHA)}
	ch := make(chan request)
	e.workers[name] = ch
	e.wg.Add(1)
	go func() {
		defer e.wg.Done()
		for {
			select {
			case <-e.ctx.Done():
				return
			case r := <-ch:
				ctx, cancel := context.WithCancel(r.ctx)
				stop := context.AfterFunc(e.ctx, cancel)
				var out response
				if r.op == "status" {
					out.status, out.err = w.status(ctx)
				} else {
					out.data, out.err = w.get(ctx, r.op == "leap")
					out.path = w.resultPath
				}
				stop()
				cancel()
				r.reply <- out
			}
		}
	}()
	return nil
}

func (e *Engine) call(ctx context.Context, name, op string) (response, error) {
	e.mu.RLock()
	ch, ok := e.workers[name]
	closed := e.closed
	e.mu.RUnlock()
	if closed {
		return response{}, errors.New("engine closed")
	}
	if !ok {
		return response{}, fmt.Errorf("unknown channel %q", name)
	}
	r := request{ctx: ctx, op: op, reply: make(chan response, 1)}
	select {
	case <-ctx.Done():
		return response{}, ctx.Err()
	case <-e.ctx.Done():
		return response{}, errors.New("engine closed")
	case ch <- r:
	}
	select {
	case <-ctx.Done():
		return response{}, ctx.Err()
	case <-e.ctx.Done():
		return response{}, errors.New("engine closed")
	case out := <-r.reply:
		return out, out.err
	}
}

func (e *Engine) Get(ctx context.Context, name string) (json.RawMessage, error) {
	r, err := e.call(ctx, name, "get")
	return r.data, err
}

func (e *Engine) Leap(ctx context.Context, name string) (json.RawMessage, error) {
	r, err := e.call(ctx, name, "leap")
	return r.data, err
}

// GetPath returns the persistent JSON artifact instead of copying its contents
// to the caller. Force requests a full evaluation before resolving the path.
func (e *Engine) GetPath(ctx context.Context, name string, force bool) (string, error) {
	op := "get"
	if force {
		op = "leap"
	}
	r, err := e.call(ctx, name, op)
	return r.path, err
}

func (e *Engine) Status(ctx context.Context, name string) (Status, error) {
	r, err := e.call(ctx, name, "status")
	return r.status, err
}

func (e *Engine) Close() error {
	e.mu.Lock()
	e.closed = true
	e.cancel()
	e.mu.Unlock()
	e.wg.Wait()
	return nil
}
