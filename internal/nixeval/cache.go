package nixeval

import (
	"context"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"

	"github.com/shaoyanji/fibtransponder/internal/deltaqueue"
)

type entry struct{ Path, SHA string }
type manifest struct {
	Epoch, CoreKey string
	Files          map[string]string
	Log            []entry
	Base           json.RawMessage
	Status         Status
	Stats          map[string]statSHA
}
type worker struct {
	config     Channel
	dir        string
	stats      map[string]statSHA
	base       json.RawMessage
	resultPath string
}

func mix64(z uint64) uint64 {
	z = (z ^ (z >> 30)) * 0xBF58476D1CE4E5B9
	z = (z ^ (z >> 27)) * 0x94D049BB133111EB
	return z ^ (z >> 31)
}

// Length-delimited strings avoid ambiguous boundaries; chaining preserves order.
func wordKey(parts ...string) string {
	h := uint64(0x9E3779B97F4A7C15)
	for _, p := range parts {
		h = mix64(h ^ uint64(len(p)))
		b := []byte(p)
		for len(b) >= 8 {
			h = mix64(h ^ binary.LittleEndian.Uint64(b))
			b = b[8:]
		}
		var tail [8]byte
		copy(tail[:], b)
		h = mix64(h ^ binary.LittleEndian.Uint64(tail[:]))
	}
	var b [8]byte
	binary.LittleEndian.PutUint64(b[:], mix64(h^uint64(len(parts))))
	return hex.EncodeToString(b[:])
}

type segment struct {
	SegmentStatus
	Entries []entry
}

func segments(m manifest) []segment {
	var out []segment
	pos := 0
	for _, term := range deltaqueue.ZeckendorfTerms(len(m.Log)) {
		block := m.Log[pos : pos+term]
		parts := []string{m.Epoch, fmt.Sprint(pos)}
		for _, e := range block {
			parts = append(parts, e.Path, e.SHA)
		}
		out = append(out, segment{SegmentStatus: SegmentStatus{Term: term, Key: wordKey(parts...)}, Entries: block})
		pos += term
	}
	return out
}

func (w *worker) load() (manifest, error) {
	var m manifest
	err := readJSON(filepath.Join(w.dir, "manifest.json"), &m)
	if err == nil {
		if m.Epoch == "" || m.Files == nil || !json.Valid(m.Base) {
			return m, errors.New("invalid epoch manifest")
		}
		w.stats = m.Stats
		w.base = m.Base
		if w.stats == nil {
			w.stats = make(map[string]statSHA)
		}
	}
	return m, err
}

func (w *worker) status(ctx context.Context) (Status, error) {
	unlock, err := w.lock(ctx)
	if err != nil {
		return Status{}, err
	}
	defer unlock()
	m, err := w.load()
	if errors.Is(err, os.ErrNotExist) {
		return Status{}, nil
	}
	return m.Status, err
}

func threshold(index int) int {
	a, b := 0, 1
	for i := 0; i < index; i++ {
		a, b = b, a+b
	}
	return a
}

func (w *worker) leap(ctx context.Context, before tree, reason string) (manifest, error) {
	base, err := w.config.Evaluator.Eval(ctx, w.config.Repo, w.config.Attr)
	if err != nil {
		return manifest{}, err
	}
	if !json.Valid(base) {
		return manifest{}, errors.New("evaluator returned invalid JSON")
	}
	w.base = base
	after, err := w.scanRepo(ctx, w.config.Repo, true, true)
	if err != nil {
		return manifest{}, err
	}
	if before.All != after.All {
		return manifest{}, errors.New("inputs changed during full evaluation; retry")
	}
	epoch := digest([]any{after.Core, after.Eligible, rawSHA(base)})
	return manifest{Epoch: epoch, CoreKey: after.Core, Files: after.Eligible, Base: base, Status: Status{LeapReason: reason}}, nil
}

func (w *worker) content(e entry) ([]byte, error) {
	b, err := os.ReadFile(filepath.Join(w.config.Repo, e.Path))
	if err != nil {
		return nil, err
	}
	// Eligible files must be regular files, not symlinks or gitlinks.
	s, err := os.Lstat(filepath.Join(w.config.Repo, e.Path))
	if err != nil {
		return nil, err
	}
	if !s.Mode().IsRegular() {
		return nil, fmt.Errorf("eligible file is not regular: %s", e.Path)
	}
	if rawSHA(b) != e.SHA {
		return nil, errors.New("inputs changed during fragment evaluation; retry")
	}
	return b, nil
}

func (w *worker) get(ctx context.Context, force bool) (json.RawMessage, error) {
	unlock, err := w.lock(ctx)
	if err != nil {
		return nil, err
	}
	defer unlock()
	m, err := w.load()
	reason := ""
	if errors.Is(err, os.ErrNotExist) {
		reason = "no epoch"
	} else if err != nil {
		return nil, err
	}
	t, err := w.scanRepo(ctx, w.config.Repo, true, false)
	if err != nil {
		return nil, err
	}
	if force {
		reason = "explicit leap"
	}
	if reason == "" {
		if m.CoreKey != t.Core {
			reason = "core key or eligible path set changed"
		}
		for path, sha := range t.Eligible {
			if sha != "deleted" && m.Files[path] == "deleted" {
				reason = "eligible file restored"
			}
			if sha == "deleted" {
				if m.Files[path] != sha {
					reason = "eligible file deleted"
				}
			}
		}
	}
	if reason == "" {
		previous := make(map[string]string)
		for i := range m.Log {
			previous[m.Log[i].Path] = m.Log[i].SHA
			m.Log[i].SHA = t.Eligible[m.Log[i].Path]
		}
		var paths []string
		for path, sha := range t.Eligible {
			if sha != m.Files[path] {
				paths = append(paths, path)
			}
		}
		sort.Strings(paths)
		for _, path := range paths {
			if _, ok := previous[path]; !ok {
				m.Log = append(m.Log, entry{path, t.Eligible[path]})
			}
		}
		if len(m.Log) >= threshold(w.config.EpochIndex) {
			reason = "epoch threshold"
		}
		if reason == "" {
			for _, e := range m.Log {
				if previous[e.Path] == e.SHA || m.Files[e.Path] == e.SHA {
					continue
				}
				b, err := w.content(e)
				if err != nil {
					return nil, err
				}
				leap, err := w.config.Fragmenter.NeedsLeap(ctx, e.Path, b, append(json.RawMessage(nil), m.Base...))
				if err != nil {
					return nil, err
				}
				if leap {
					reason = "fragmenter requested leap"
					break
				}
			}
		}
	}
	if reason != "" {
		// A leap verifies fresh digests on both sides of the full evaluation.
		t, err = w.scanRepo(ctx, w.config.Repo, true, true)
		if err != nil {
			return nil, err
		}
		m, err = w.leap(ctx, t, reason)
		if err != nil {
			return nil, err
		}
		t.Eligible = m.Files
	}
	blocks := segments(m)
	keys := []string{m.Epoch}
	status := Status{Epoch: m.Epoch, N: len(m.Log), LeapReason: reason}
	for _, b := range blocks {
		keys = append(keys, b.Key)
		status.Terms = append(status.Terms, b.Term)
		_, err := os.Stat(filepath.Join(w.dir, "segments", b.Key+".json"))
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return nil, err
		}
		b.Hit = err == nil
		status.Segments = append(status.Segments, b.SegmentStatus)
	}
	qpath := filepath.Join(w.dir, "results", wordKey(keys...)+".json")
	stored, err := os.ReadFile(qpath)
	result := json.RawMessage(stored)
	if err == nil {
		if !json.Valid(result) {
			return nil, errors.New("invalid cached result JSON")
		}
		status.QuotientHit = true
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	} else {
		var fragments []json.RawMessage
		for _, b := range blocks {
			path := filepath.Join(w.dir, "segments", b.Key+".json")
			var fs []json.RawMessage
			err := readJSON(path, &fs)
			if errors.Is(err, os.ErrNotExist) {
				for _, e := range b.Entries {
					content, err := w.content(e)
					if err != nil {
						return nil, err
					}
					f, err := w.config.Fragmenter.Fragment(ctx, e.Path, content, append(json.RawMessage(nil), m.Base...))
					if err != nil {
						return nil, err
					}
					if !json.Valid(f) {
						return nil, errors.New("fragmenter returned invalid JSON")
					}
					fs = append(fs, f)
				}
				if err = atomicJSON(path, fs); err != nil {
					return nil, err
				}
			} else if err != nil {
				return nil, err
			}
			fragments = append(fragments, fs...)
		}
		if len(blocks) == 0 {
			result = append(json.RawMessage(nil), m.Base...)
		} else {
			result, err = w.config.Fragmenter.Merge(ctx, append(json.RawMessage(nil), m.Base...), fragments)
			if err != nil {
				return nil, err
			}
		}
		if !json.Valid(result) {
			return nil, errors.New("merge returned invalid JSON")
		}
		if err = atomicJSON(qpath, result); err != nil {
			return nil, err
		}
	}
	final, err := w.scanRepo(ctx, w.config.Repo, true, false)
	if err != nil {
		return nil, err
	}
	if !reflect.DeepEqual(t.Eligible, final.Eligible) || t.All != final.All {
		return nil, errors.New("inputs changed during evaluation; retry")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	m.Status, m.Stats = status, w.stats
	if err = atomicJSON(filepath.Join(w.dir, "manifest.json"), m); err != nil {
		return nil, err
	}
	w.resultPath = qpath
	return result, nil
}
