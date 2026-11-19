package main

import (
	"bytes"
	"context"
	"testing"
)

func TestCompareGraphs(t *testing.T) {
	a := []byte(`{"nodes":[{"id":"b"},{"id":"a"}],"edges":[],"stats":{"edges":0}}`)
	b := []byte(`{"stats":{"edges":0},"edges":[],"nodes":[{"id":"a"},{"id":"b"}]}`)
	if n, e, err := compareGraphs(a, b); err != nil || n != 2 || e != 0 {
		t.Fatalf("%d %d %v", n, e, err)
	}
	for _, other := range []string{`{"nodes":[],"edges":[]}`, `{"nodes":[{"id":"b"},{"id":"c"}],"edges":[],"stats":{"edges":0}}`, `{"nodes":[{"id":"a"},{"id":"b"}],"edges":[],"stats":{"edges":1}}`, `invalid`} {
		if _, _, err := compareGraphs(a, []byte(other)); err == nil {
			t.Fatal("mismatch accepted", other)
		}
	}
}

func TestArgumentsAndEmptyStatus(t *testing.T) {
	var out, stderr bytes.Buffer
	if err := run(context.Background(), []string{"-repo", t.TempDir(), "-cache", t.TempDir(), "status"}, &out, &stderr); err != nil {
		t.Fatal(err)
	}
	if out.Len() == 0 {
		t.Fatal("no JSON status")
	}
	for _, args := range [][]string{nil, {"unknown"}, {"graph", "extra"}, {"-epoch", "1", "status"}} {
		if err := run(context.Background(), args, &out, &stderr); err == nil {
			t.Fatal("invalid args accepted", args)
		}
	}
}
