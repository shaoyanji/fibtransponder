package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"reflect"
	"sort"
	"syscall"

	"github.com/shaoyanji/fibtransponder/internal/nixeval"
	"github.com/shaoyanji/fibtransponder/internal/nixeval/kgdoc"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	if err := run(ctx, os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, "nixeval:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string, out, stderr io.Writer) error {
	flags := flag.NewFlagSet("nixeval", flag.ContinueOnError)
	flags.SetOutput(stderr)
	repo := flags.String("repo", ".", "nixconfig repository directory")
	cache := flags.String("cache", "", "cache root (default: $XDG_CACHE_HOME/fibtransponder/nixeval)")
	epoch := flags.Int("epoch", 10, "epoch Fibonacci index")
	flags.Usage = func() {
		fmt.Fprintln(stderr, "usage: nixeval [-repo DIR] [-cache DIR] [-epoch 10] graph|leap|status|parity")
		flags.PrintDefaults()
	}
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 1 {
		return errors.New("expected graph, leap, status or parity")
	}
	op := flags.Arg(0)
	if op != "graph" && op != "leap" && op != "status" && op != "parity" {
		return fmt.Errorf("unknown command %q", op)
	}
	if *cache == "" {
		root, err := os.UserCacheDir()
		if err != nil {
			return err
		}
		*cache = filepath.Join(root, "fibtransponder", "nixeval")
	}
	root, err := filepath.Abs(*cache)
	if err != nil {
		return err
	}
	e := nixeval.New(root)
	defer e.Close()
	if err := e.Register("kg", nixeval.Channel{Repo: *repo, Attr: ".#knowledgeGraph.graph", CacheVersion: "kgdoc-v1", Eligible: func(string) bool { return true }, Fragmenter: kgdoc.Fragmenter{}, EpochIndex: *epoch}); err != nil {
		return err
	}
	if op == "status" {
		s, err := e.Status(ctx, "kg")
		if err != nil {
			return err
		}
		return json.NewEncoder(out).Encode(s)
	}
	path, err := e.GetPath(ctx, "kg", op == "leap")
	if err != nil {
		return err
	}
	if op != "parity" {
		_, err = fmt.Fprintln(out, path)
		return err
	}
	cached, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	fresh, err := (nixeval.NixEvaluator{}).Eval(ctx, *repo, ".#knowledgeGraph.graph")
	if err != nil {
		return err
	}
	nodes, edges, err := compareGraphs(cached, fresh)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(out, "parity PASS nodes=%d edges=%d\n", nodes, edges)
	return err
}

func sortedGraph(raw []byte) (map[string]any, int, int, error) {
	var g map[string]any
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	if !json.Valid(raw) {
		return nil, 0, 0, errors.New("invalid graph JSON")
	}
	if err := d.Decode(&g); err != nil {
		return nil, 0, 0, err
	}
	counts := make([]int, 0, 2)
	for _, key := range []string{"nodes", "edges"} {
		items, ok := g[key].([]any)
		if !ok {
			return nil, 0, 0, fmt.Errorf("graph %s must be an array", key)
		}
		counts = append(counts, len(items))
		sort.Slice(items, func(i, j int) bool {
			a, _ := json.Marshal(items[i])
			b, _ := json.Marshal(items[j])
			return bytes.Compare(a, b) < 0
		})
	}
	return g, counts[0], counts[1], nil
}

func compareGraphs(cached, fresh []byte) (int, int, error) {
	a, an, ae, err := sortedGraph(cached)
	if err != nil {
		return 0, 0, err
	}
	b, bn, be, err := sortedGraph(fresh)
	if err != nil {
		return 0, 0, err
	}
	if !reflect.DeepEqual(a, b) {
		return 0, 0, fmt.Errorf("parity FAIL: cached nodes=%d edges=%d; fresh nodes=%d edges=%d (content or counts differ)", an, ae, bn, be)
	}
	return an, ae, nil
}
