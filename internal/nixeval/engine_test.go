package nixeval

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

var background = context.Background()

type fakeEvaluator struct {
	calls atomic.Int32
	hook  func(context.Context) error
}

func (f *fakeEvaluator) Eval(ctx context.Context, repo, attr string) (json.RawMessage, error) {
	f.calls.Add(1)
	if f.hook != nil {
		if err := f.hook(ctx); err != nil {
			return nil, err
		}
	}
	cmd := exec.CommandContext(ctx, "git", "ls-files", "-z")
	cmd.Dir = repo
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	files := make(map[string]string)
	for _, path := range strings.Split(string(out), "\x00") {
		if path == "" {
			continue
		}
		if s, err := os.Stat(filepath.Join(repo, path)); err == nil && s.IsDir() {
			continue
		}
		b, err := os.ReadFile(filepath.Join(repo, path))
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, err
		}
		files[path] = string(b)
	}
	return json.Marshal(files)
}

type trivialFragmenter struct {
	fragments atomic.Int32
	merges    atomic.Int32
	checks    atomic.Int32
	fail      atomic.Bool
}

func (f *trivialFragmenter) NeedsLeap(ctx context.Context, path string, b []byte, base json.RawMessage) (bool, error) {
	f.checks.Add(1)
	return strings.Contains(string(b), "LEAP"), ctx.Err()
}
func (f *trivialFragmenter) Fragment(ctx context.Context, path string, b []byte, base json.RawMessage) (json.RawMessage, error) {
	f.fragments.Add(1)
	if f.fail.Load() {
		return nil, errors.New("fragment failed")
	}
	return json.Marshal(map[string]string{path: string(b)})
}
func (f *trivialFragmenter) Merge(ctx context.Context, base json.RawMessage, fragments []json.RawMessage) (json.RawMessage, error) {
	f.merges.Add(1)
	var files map[string]string
	if err := json.Unmarshal(base, &files); err != nil {
		return nil, err
	}
	for _, fragment := range fragments {
		var updates map[string]string
		if err := json.Unmarshal(fragment, &updates); err != nil {
			return nil, err
		}
		for k, v := range updates {
			files[k] = v
		}
	}
	return json.Marshal(files)
}

type fixture struct {
	t           *testing.T
	repo, cache string
	e           *Engine
	c           Channel
	f           *trivialFragmenter
	eval        *fakeEvaluator
}

func git(t *testing.T, repo string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = repo
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v: %s", args, err, out)
	}
}

func newFixture(t *testing.T, index int) *fixture {
	t.Helper()
	x := &fixture{t: t, repo: t.TempDir(), cache: t.TempDir(), f: &trivialFragmenter{}, eval: &fakeEvaluator{}}
	git(t, x.repo, "init", "-q")
	for i := 0; i < 12; i++ {
		x.write(fmt.Sprintf("%02d.doc", i), fmt.Sprintf("original %d", i))
	}
	x.write("core.nix", "core")
	git(t, x.repo, "add", "--all")
	x.c = Channel{Repo: x.repo, Attr: ".#fake", Eligible: func(p string) bool { return strings.HasSuffix(p, ".doc") }, Fragmenter: x.f, Evaluator: x.eval, EpochIndex: index}
	x.open()
	t.Cleanup(func() { _ = x.e.Close() })
	return x
}
func (x *fixture) open() {
	x.e = New(x.cache)
	if err := x.e.Register("test", x.c); err != nil {
		x.t.Fatal(err)
	}
}
func (x *fixture) write(path, content string) {
	x.t.Helper()
	if err := os.WriteFile(filepath.Join(x.repo, path), []byte(content), 0600); err != nil {
		x.t.Fatal(err)
	}
}
func (x *fixture) get() json.RawMessage {
	x.t.Helper()
	r, err := x.e.Get(background, "test")
	if err != nil {
		x.t.Fatal(err)
	}
	return r
}
func (x *fixture) status() Status {
	x.t.Helper()
	s, err := x.e.Status(background, "test")
	if err != nil {
		x.t.Fatal(err)
	}
	return s
}
func (x *fixture) parity() {
	x.t.Helper()
	got := x.get()
	want, err := (&fakeEvaluator{}).Eval(background, x.repo, x.c.Attr)
	if err != nil {
		x.t.Fatal(err)
	}
	var a, b any
	if err := json.Unmarshal(got, &a); err != nil {
		x.t.Fatal(err)
	}
	if err := json.Unmarshal(want, &b); err != nil {
		x.t.Fatal(err)
	}
	if !reflect.DeepEqual(a, b) {
		x.t.Fatalf("got %s want %s", got, want)
	}
}

func TestMergePersistenceAndQuotientHit(t *testing.T) {
	x := newFixture(t, 0)
	x.parity()
	if s := x.status(); s.N != 0 || s.LeapReason != "no epoch" {
		t.Fatalf("cold: %+v", s)
	}
	x.write("03.doc", "edit first, not alphabetically first")
	x.parity()
	x.write("01.doc", "edit second")
	x.parity()
	x.write("03.doc", "re-edit first")
	x.parity()
	if s := x.status(); s.N != 2 {
		t.Fatalf("re-edit appended: %+v", s)
	}
	before := x.get()
	fragments, merges, checks := x.f.fragments.Load(), x.f.merges.Load(), x.f.checks.Load()
	if got := x.get(); string(got) != string(before) {
		t.Fatal("quotient result changed")
	}
	if !x.status().QuotientHit || x.f.fragments.Load() != fragments || x.f.merges.Load() != merges || x.f.checks.Load() != checks {
		t.Fatal("quotient hit did work")
	}
	epoch := x.status().Epoch
	if err := x.e.Close(); err != nil {
		t.Fatal(err)
	}
	x.open()
	if x.status().Epoch != epoch {
		t.Fatal("status not persisted")
	}
	x.parity()
	if x.eval.calls.Load() != 1 || x.f.fragments.Load() != fragments || !x.status().QuotientHit {
		t.Fatal("restart lost cache")
	}
	// The ingest log is first-divergence ordered, not lexicographically reordered.
	identity := digest([]string{"test", x.repo, x.c.Attr, x.c.CacheVersion})
	var m manifest
	if err := readJSON(filepath.Join(x.cache, identity, "manifest.json"), &m); err != nil {
		t.Fatal(err)
	}
	if m.Log[0].Path != "03.doc" || m.Log[1].Path != "01.doc" || len(m.Stats) == 0 {
		t.Fatalf("manifest: %+v", m.Log)
	}
}

func TestFibonacciCarryAndRevertRetainsPosition(t *testing.T) {
	x := newFixture(t, 0)
	x.get()
	for i := 0; i < 6; i++ {
		x.write(fmt.Sprintf("%02d.doc", i), fmt.Sprintf("edit %d", i))
		x.parity()
	}
	six := x.status()
	if !reflect.DeepEqual(six.Terms, []int{5, 1}) {
		t.Fatal(six)
	}
	x.write("06.doc", "edit 6")
	x.parity()
	seven := x.status()
	if !reflect.DeepEqual(seven.Terms, []int{5, 2}) || six.Segments[0].Key != seven.Segments[0].Key || !seven.Segments[0].Hit || seven.Segments[1].Hit {
		t.Fatalf("carry: %+v -> %+v", six, seven)
	}
	fragments := x.f.fragments.Load()
	checks := x.f.checks.Load()
	x.write("00.doc", "original 0")
	x.parity()
	reverted := x.status()
	if reverted.N != 7 || reverted.Segments[1].Key != seven.Segments[1].Key || !reverted.Segments[1].Hit {
		t.Fatalf("revert shifted suffix: %+v", reverted)
	}
	if x.f.fragments.Load()-fragments != 5 {
		t.Fatal("later segment was recomputed")
	}
	if x.f.checks.Load() != checks {
		t.Fatal("reverted content unnecessarily checked for leap")
	}
	x.write("07.doc", "edit 7")
	x.parity()
	if !reflect.DeepEqual(x.status().Terms, []int{8}) {
		t.Fatal("full Fibonacci carry incorrect")
	}
}

func TestLeapTriggers(t *testing.T) {
	tests := []struct {
		name, reason string
		edit         func(*fixture)
	}{
		{"core", "core key", func(x *fixture) { x.write("core.nix", "new core") }},
		{"add eligible", "eligible path set", func(x *fixture) { x.write("new.doc", "new"); git(t, x.repo, "add", "new.doc") }},
		{"remove eligible", "eligible path set", func(x *fixture) { git(t, x.repo, "rm", "--cached", "00.doc") }},
		{"delete eligible", "eligible file deleted", func(x *fixture) {
			if err := os.Remove(filepath.Join(x.repo, "00.doc")); err != nil {
				t.Fatal(err)
			}
		}},
		{"fragmenter", "fragmenter requested leap", func(x *fixture) { x.write("00.doc", "LEAP please") }},
		{"threshold", "epoch threshold", func(x *fixture) { x.write("00.doc", "edit") }},
		{"explicit", "explicit leap", func(x *fixture) {
			if _, err := x.e.Leap(background, "test"); err != nil {
				t.Fatal(err)
			}
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			index := 0
			if tt.name == "threshold" {
				index = 2
			}
			x := newFixture(t, index)
			x.get()
			tt.edit(x)
			if tt.name != "explicit" {
				x.parity()
			}
			s := x.status()
			if s.N != 0 || !strings.Contains(s.LeapReason, tt.reason) || x.eval.calls.Load() != 2 {
				t.Fatalf("leap: %+v evals=%d", s, x.eval.calls.Load())
			}
		})
	}
}

func TestUntrackedIgnoredAndEmptyLog(t *testing.T) {
	x := newFixture(t, 0)
	before := x.get()
	x.write("untracked.doc", "not visible")
	if got := x.get(); string(got) != string(before) {
		t.Fatal("untracked content leaked")
	}
	if x.eval.calls.Load() != 1 || x.f.fragments.Load() != 0 || x.f.merges.Load() != 0 || !x.status().QuotientHit {
		t.Fatal("empty log did work")
	}
	if threshold(10) != 55 || threshold(2) != 1 {
		t.Fatal("Fibonacci index convention")
	}
}

func TestErrorsDoNotPublishManifest(t *testing.T) {
	x := newFixture(t, 0)
	x.eval.hook = func(context.Context) error { return errors.New("eval failed") }
	if _, err := x.e.Get(background, "test"); err == nil || !strings.Contains(err.Error(), "eval failed") {
		t.Fatal(err)
	}
	x.eval.hook = nil
	x.get()
	epoch := x.status().Epoch
	x.write("00.doc", "edit")
	x.f.fail.Store(true)
	if _, err := x.e.Get(background, "test"); err == nil || !strings.Contains(err.Error(), "fragment failed") {
		t.Fatal(err)
	}
	if s := x.status(); s.N != 0 || s.Epoch != epoch {
		t.Fatal("failed call published state")
	}
	x.f.fail.Store(false)
	x.parity()
	x.write("core.nix", "changed while evaluating")
	x.eval.hook = func(context.Context) error { x.write("core.nix", "another change during evaluation"); return nil }
	if _, err := x.e.Get(background, "test"); err == nil || !strings.Contains(err.Error(), "inputs changed") {
		t.Fatal(err)
	}
}

func TestConcurrentChannelsAndCancellation(t *testing.T) {
	x := newFixture(t, 0)
	started := make(chan struct{}, 2)
	release := make(chan struct{})
	x.eval.hook = func(ctx context.Context) error {
		started <- struct{}{}
		select {
		case <-release:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	other := x.c
	other.Attr = ".#other"
	if err := x.e.Register("other", other); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	errs := make(chan error, 12)
	for _, name := range []string{"test", "other"} {
		wg.Add(1)
		go func(name string) { defer wg.Done(); _, err := x.e.Get(background, name); errs <- err }(name)
	}
	for i := 0; i < 2; i++ {
		select {
		case <-started:
		case <-time.After(5 * time.Second):
			t.Fatal("channels did not evaluate in parallel")
		}
	}
	ctx, cancel := context.WithCancel(background)
	cancel()
	if _, err := x.e.Get(ctx, "test"); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	close(release)
	wg.Wait()
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			name := "test"
			if i%2 == 1 {
				name = "other"
			}
			_, err := x.e.Get(background, name)
			errs <- err
		}(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	if x.eval.calls.Load() != 2 {
		t.Fatalf("calls not serialized: %d", x.eval.calls.Load())
	}
}

func TestCloseCancelsActiveEvaluation(t *testing.T) {
	x := newFixture(t, 0)
	started := make(chan struct{})
	x.eval.hook = func(ctx context.Context) error { close(started); <-ctx.Done(); return ctx.Err() }
	done := make(chan struct{})
	go func() { defer close(done); _, _ = x.e.Get(background, "test") }()
	<-started
	if err := x.e.Close(); err != nil {
		t.Fatal(err)
	}
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Get did not stop")
	}
	if _, err := x.e.Get(background, "test"); err == nil {
		t.Fatal("closed engine accepted Get")
	}
}

func TestValidationAndCorruptCache(t *testing.T) {
	e := New(t.TempDir())
	defer e.Close()
	if err := e.Register("", Channel{}); err == nil {
		t.Fatal("invalid channel accepted")
	}
	if _, err := e.Get(background, "missing"); err == nil {
		t.Fatal("missing channel accepted")
	}
	x := newFixture(t, 0)
	if s := x.status(); s.Epoch != "" {
		t.Fatal("unexpected initial status")
	}
	if err := x.e.Register("test", x.c); err == nil {
		t.Fatal("duplicate accepted")
	}
	x.get()
	identity := digest([]string{"test", x.repo, x.c.Attr, x.c.CacheVersion})
	if err := os.WriteFile(filepath.Join(x.cache, identity, "manifest.json"), []byte("broken"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := x.e.Get(background, "test"); err == nil {
		t.Fatal("corrupt manifest silently accepted")
	}
}

func TestNixEvaluatorFallback(t *testing.T) {
	dir := t.TempDir()
	script := `#!/bin/sh
printf '%s\n' "$*" >> "$NIX_EVAL_TEST_LOG"
if [ "$NIX_EVAL_TEST_MODE" = fail ]; then printf 'failure detail' >&2; exit 1; fi
if [ "$NIX_EVAL_TEST_MODE" = invalid ]; then printf 'invalid'; exit 0; fi
if [ "$NIX_EVAL_TEST_MODE" = fallback ] && [ "$2" != --impure ]; then exit 1; fi
printf ' { "ok": true } '
`
	if err := os.WriteFile(filepath.Join(dir, "nix"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	for _, mode := range []string{"pure", "fallback", "fail", "invalid"} {
		t.Run(mode, func(t *testing.T) {
			log := filepath.Join(t.TempDir(), "calls")
			t.Setenv("NIX_EVAL_TEST_LOG", log)
			t.Setenv("NIX_EVAL_TEST_MODE", mode)
			out, err := (NixEvaluator{}).Eval(background, dir, ".#target")
			if mode == "fail" || mode == "invalid" {
				if err == nil {
					t.Fatal("expected error")
				}
			} else if err != nil || !json.Valid(out) {
				t.Fatalf("%s %v", out, err)
			}
			calls, readErr := os.ReadFile(log)
			if readErr != nil {
				t.Fatal(readErr)
			}
			want := "eval --json .#target\n"
			if mode != "pure" {
				want += "eval --impure --json .#target\n"
			}
			if string(calls) != want {
				t.Fatalf("calls %q want %q", calls, want)
			}
		})
	}
}

func TestStagingContentDoesNotChangeEpoch(t *testing.T) {
	x := newFixture(t, 0)
	x.get()
	x.write("00.doc", "changed")
	x.get()
	epoch := x.status().Epoch
	git(t, x.repo, "add", "00.doc")
	x.get()
	if x.status().Epoch != epoch || x.eval.calls.Load() != 1 || !x.status().QuotientHit {
		t.Fatal("index blob hash affected working-tree key")
	}
}

func TestIndependentEnginesSerializeCacheWriters(t *testing.T) {
	x := newFixture(t, 0)
	other := New(x.cache)
	defer other.Close()
	if err := other.Register("test", x.c); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for _, e := range []*Engine{x.e, other} {
		wg.Add(1)
		go func(e *Engine) { defer wg.Done(); _, err := e.Get(background, "test"); errs <- err }(e)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	if x.eval.calls.Load() != 1 {
		t.Fatal("independent engines evaluated concurrently")
	}
}

func TestStatSHACache(t *testing.T) {
	repo := t.TempDir()
	path := filepath.Join(repo, "file")
	if err := os.WriteFile(path, []byte("content"), 0600); err != nil {
		t.Fatal(err)
	}
	w := worker{stats: make(map[string]statSHA)}
	sha, err := w.fileSHA(background, path, false)
	if err != nil {
		t.Fatal(err)
	}
	// A sentinel digest shows the matching stat tuple bypasses file hashing.
	stat := w.stats[path]
	stat.SHA = "cached"
	w.stats[path] = stat
	if got, err := w.fileSHA(background, path, false); err != nil || got != "cached" {
		t.Fatalf("%s %v", got, err)
	}
	if got, err := w.fileSHA(background, path, true); err != nil || got != sha {
		t.Fatalf("%s %v", got, err)
	}
}
