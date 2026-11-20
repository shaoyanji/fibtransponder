package nixeval

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"
)

type statSHA struct {
	Size  int64
	Mtime int64
	Mode  os.FileMode
	SHA   string
}

func digest(v any) string {
	b, _ := json.Marshal(v)
	return rawSHA(b)
}

func rawSHA(b []byte) string {
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

func atomicJSON(path string, v any) error {
	b, err := json.Marshal(v)
	if raw, ok := v.(json.RawMessage); ok {
		b = raw
	}
	if err != nil {
		return err
	}
	if err = os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".write-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err = f.Write(b); err != nil {
		f.Close()
		return err
	}
	if err = f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), path)
}

func readJSON(path string, v any) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if err = json.Unmarshal(b, v); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	return nil
}

// A process lock also protects independent one-shot engines sharing a cache.
func (w *worker) lock(ctx context.Context) (func(), error) {
	if err := os.MkdirAll(w.dir, 0700); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(filepath.Join(w.dir, "writer.lock"), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	for {
		if err = ctx.Err(); err != nil {
			f.Close()
			return nil, err
		}
		err = syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
		if err == nil {
			return func() { _ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN); _ = f.Close() }, nil
		}
		if !errors.Is(err, syscall.EWOULDBLOCK) {
			f.Close()
			return nil, err
		}
		timer := time.NewTimer(10 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			f.Close()
			return nil, ctx.Err()
		case <-timer.C:
		}
	}
}

type tree struct {
	Core     string
	All      string
	Eligible map[string]string
}

func (w *worker) fileSHA(ctx context.Context, full string, force bool) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	s, err := os.Lstat(full)
	if errors.Is(err, os.ErrNotExist) {
		delete(w.stats, full)
		return "deleted", nil
	}
	if err != nil {
		return "", err
	}
	if s.Mode()&os.ModeSymlink != 0 {
		target, err := os.Readlink(full)
		return digest(target), err
	}
	if s.IsDir() {
		if _, err := os.Stat(filepath.Join(full, ".git")); errors.Is(err, os.ErrNotExist) {
			return "uninitialised gitlink", nil
		}
		t, err := w.scanRepo(ctx, full, false, force)
		return t.All, err
	}
	if !s.Mode().IsRegular() {
		return "", fmt.Errorf("unsupported tracked file: %s", full)
	}
	old, ok := w.stats[full]
	if !force && ok && old.Size == s.Size() && old.Mtime == s.ModTime().UnixNano() && old.Mode == s.Mode() {
		return old.SHA, nil
	}
	b, err := os.ReadFile(full)
	if err != nil {
		return "", err
	}
	h := sha256.Sum256(b)
	sha := hex.EncodeToString(h[:])
	w.stats[full] = statSHA{Size: s.Size(), Mtime: s.ModTime().UnixNano(), Mode: s.Mode(), SHA: sha}
	return sha, nil
}

func (w *worker) scanRepo(ctx context.Context, repo string, eligible, force bool) (tree, error) {
	var allowed map[string]bool
	if restriction, ok := w.config.Fragmenter.(BaseEligibility); ok && eligible && len(w.base) != 0 {
		paths, err := restriction.EligiblePaths(w.base)
		if err != nil {
			return tree{}, err
		}
		allowed = make(map[string]bool, len(paths))
		for _, path := range paths {
			allowed[path] = true
		}
	}
	cmd := exec.CommandContext(ctx, "git", "ls-files", "--stage", "-z")
	cmd.Dir = repo
	out, err := cmd.Output()
	if err != nil {
		return tree{}, fmt.Errorf("git ls-files: %w", err)
	}
	entries := strings.Split(strings.TrimSuffix(string(out), "\x00"), "\x00")
	sort.Slice(entries, func(i, j int) bool {
		_, a, _ := strings.Cut(entries[i], "\t")
		_, b, _ := strings.Cut(entries[j], "\t")
		return a < b
	})
	t := tree{Eligible: make(map[string]string)}
	var core, all [][]string
	var paths []string
	for _, entry := range entries {
		if entry == "" {
			continue
		}
		meta, path, ok := strings.Cut(entry, "\t")
		fields := strings.Fields(meta)
		if !ok || len(fields) != 3 || fields[2] != "0" {
			return tree{}, errors.New("unmerged or invalid git index")
		}
		sha, err := w.fileSHA(ctx, filepath.Join(repo, path), force)
		if err != nil {
			return tree{}, err
		}
		record := []string{path, fields[0], sha}
		all = append(all, record)
		if eligible && w.config.Eligible(path) && (allowed == nil || allowed[path]) {
			paths = append(paths, path)
			t.Eligible[path] = sha
		} else {
			core = append(core, record)
		}
	}
	sort.Strings(paths)
	t.Core = digest([]any{core, paths})
	t.All = digest(all)
	return t, nil
}
