package kgdoc

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shaoyanji/fibtransponder/internal/nixeval"
)

type evaluatorFunc func(context.Context, string, string) (json.RawMessage, error)

func (f evaluatorFunc) Eval(ctx context.Context, repo, attr string) (json.RawMessage, error) {
	return f(ctx, repo, attr)
}

func TestChannelLastCitationAndTracking(t *testing.T) {
	repo := t.TempDir()
	write := func(file, content string) {
		t.Helper()
		full := filepath.Join(repo, file)
		if err := os.MkdirAll(filepath.Dir(full), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
	}
	git := func(args ...string) string {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = repo
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %s %v", args, out, err)
		}
		return string(out)
	}
	git("init", "-q")
	original := "# old\n[[flake/policies.nix#real]]\n"
	write("docs/a.md", original)
	write("b.md", "# B\n")
	write("core.nix", "core")
	write(Excluded, "excluded")
	git("add", "--all")
	eval := evaluatorFunc(func(ctx context.Context, repo, attr string) (json.RawMessage, error) {
		base := fixture()
		g, err := parse(base)
		if err != nil {
			return nil, err
		}
		tracked := make(map[string]bool)
		for _, file := range strings.Split(git("ls-files", "-z"), "\x00") {
			tracked[file] = true
		}
		var fs []json.RawMessage
		policy := false
		for _, file := range []string{"docs/a.md", "b.md"} {
			if !tracked[file] {
				continue
			}
			b, err := os.ReadFile(filepath.Join(repo, file))
			if err != nil {
				return nil, err
			}
			f, err := (Fragmenter{}).Fragment(ctx, file, b, base)
			if err != nil {
				return nil, err
			}
			fs = append(fs, f)
			for _, id := range citations(string(b)) {
				if id == "real" {
					policy = true
				}
			}
		}
		merged, err := (Fragmenter{}).Merge(ctx, base, fs)
		if err != nil {
			return nil, err
		}
		g, err = parse(merged)
		if err != nil {
			return nil, err
		}
		nodes := []object{}
		for _, node := range g.nodes {
			if text(node, "type") == "doc" && !tracked[text(node, "path")] {
				continue
			}
			if text(node, "type") == "policy" && !policy {
				continue
			}
			nodes = append(nodes, node)
		}
		g.raw["nodes"] = nodes
		return json.Marshal(g.raw)
	})
	e := nixeval.New(t.TempDir())
	defer e.Close()
	if err := e.Register("kg", nixeval.Channel{Repo: repo, Attr: ".#graph", Eligible: func(string) bool { return true }, Fragmenter: Fragmenter{}, Evaluator: eval}); err != nil {
		t.Fatal(err)
	}
	get := func() json.RawMessage {
		t.Helper()
		raw, err := e.Get(context.Background(), "kg")
		if err != nil {
			t.Fatal(err)
		}
		return raw
	}
	has := func(raw json.RawMessage, id string) bool {
		g, err := parse(raw)
		if err != nil {
			t.Fatal(err)
		}
		for _, node := range g.nodes {
			if text(node, "id") == id {
				return true
			}
		}
		return false
	}
	if !has(get(), "policy:real") {
		t.Fatal("initial policy missing")
	}
	write("docs/a.md", "# old\n")
	if has(get(), "policy:real") {
		t.Fatal("last citation removal retained policy")
	}
	write("docs/a.md", original)
	restored := get()
	if !has(restored, "policy:real") {
		t.Fatal("restoring last citation did not restore policy")
	}
	g, _ := parse(restored)
	if len(g.edges) != 1 || text(g.edges[0], "type") != "cites" {
		t.Fatal(g.edges)
	}
	git("rm", "--cached", "docs/a.md")
	if _, err := os.Stat(filepath.Join(repo, "docs/a.md")); err != nil {
		t.Fatal(err)
	}
	if has(get(), "doc:docs/a.md") {
		t.Fatal("untracked doc remained")
	}
	// Newly tracked docs outside the core eligibility set must invalidate it.
	git("add", "docs/a.md")
	if !has(get(), "doc:docs/a.md") {
		t.Fatal("tracked doc not restored")
	}
	write(Excluded, "excluded text edit")
	get()
	status, err := e.Status(context.Background(), "kg")
	if err != nil || status.LeapReason == "" {
		t.Fatal("excluded doc edit did not invalidate core", status, err)
	}
}

func TestChannelConcurrentDocEditRejectsColdEpoch(t *testing.T) {
	repo := t.TempDir()
	cmd := exec.Command("git", "init", "-q")
	cmd.Dir = repo
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(repo, "b.md")
	if err := os.WriteFile(file, []byte("# B"), 0600); err != nil {
		t.Fatal(err)
	}
	cmd = exec.Command("git", "add", "b.md")
	cmd.Dir = repo
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}
	e := nixeval.New(t.TempDir())
	defer e.Close()
	eval := evaluatorFunc(func(context.Context, string, string) (json.RawMessage, error) {
		return fixture(), os.WriteFile(file, []byte("# changed during eval"), 0600)
	})
	if err := e.Register("kg", nixeval.Channel{Repo: repo, Attr: ".#graph", Eligible: func(string) bool { return true }, Fragmenter: Fragmenter{}, Evaluator: eval}); err != nil {
		t.Fatal(err)
	}
	if _, err := e.Get(context.Background(), "kg"); err == nil || !strings.Contains(err.Error(), "inputs changed") {
		t.Fatal(err)
	}
	s, err := e.Status(context.Background(), "kg")
	if err != nil || s.Epoch != "" {
		t.Fatal("failed cold eval persisted epoch", s, err)
	}
	if _, err := (Fragmenter{}).Merge(context.Background(), nil, nil); err == nil || errors.Is(err, context.Canceled) {
		t.Fatal("missing input accepted", err)
	}
}
