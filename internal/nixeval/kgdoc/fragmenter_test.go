package kgdoc

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"
)

func fixture() json.RawMessage {
	return json.RawMessage(`{"nodes":[
{"id":"host:z","type":"host","label":"z"},
{"id":"host:a","type":"host","label":"a"},
{"id":"skill:nix-eval","type":"skill","label":"nix-eval"},
{"id":"doc:docs/a.md","type":"doc","path":"docs/a.md","label":"old","cites":[],"extra":9007199254740993},
{"id":"doc:b.md","type":"doc","path":"b.md","label":"B","cites":[]},
{"id":"doc:excluded","type":"doc","path":".agents/skills/router/references/playbooks.md","cites":[]},
{"id":"policy:real","type":"policy","label":"real"}],
"edges":[],"stats":{"edges":0,"recordedEdges":0,"other":42},"extra":"preserved"}`)
}

func TestFragmentVocabularyLinksAndPolicySyntax(t *testing.T) {
	content := "## not first\n# Title  \na z nix-eval nix-eval-extra az\n[x](../b.md#h) [x](<../b.md>) [s](./a.md) [e](https://x/b.md) [r](/b.md)\n[[flake/policies.nix#real]] [[flake/policies.nix#unknown]] [[flake/policies.nix#BAD]]"
	raw, err := (Fragmenter{}).Fragment(context.Background(), "docs/a.md", []byte(content), fixture())
	if err != nil {
		t.Fatal(err)
	}
	var f fragment
	if err := decode(raw, &f); err != nil {
		t.Fatal(err)
	}
	if text(f.Node, "label") != "Title  " {
		t.Fatal(f.Node)
	}
	if !reflect.DeepEqual(f.Node["cites"], []any{"real"}) {
		t.Fatal(f.Node["cites"])
	}
	var targets []string
	for _, edge := range f.Edges {
		targets = append(targets, text(edge, "to"))
	}
	if !reflect.DeepEqual(targets, []string{"host:z", "host:a", "skill:nix-eval", "doc:b.md", "policy:real"}) {
		t.Fatal(targets)
	}
	if f.Node["extra"] != json.Number("9007199254740993") {
		t.Fatal("integer precision lost")
	}
}

func TestEmptyFragmentMergePreservesOtherEdgesAndStats(t *testing.T) {
	g, err := parse(fixture())
	if err != nil {
		t.Fatal(err)
	}
	recorded := object{"from": "doc:docs/a.md", "to": "doc:b.md", "type": "derives", "evidence": nil}
	other := object{"from": "host:z", "to": "host:a", "type": "mentions"}
	g.raw["edges"] = []object{recorded, other, {"from": "doc:docs/a.md", "to": "host:a", "type": "mentions"}, {"from": "doc:docs/a.md", "to": "doc:b.md", "type": "references"}, {"from": "doc:docs/a.md", "to": "policy:real", "type": "cites"}}
	base, err := json.Marshal(g.raw)
	if err != nil {
		t.Fatal(err)
	}
	f, err := (Fragmenter{}).Fragment(context.Background(), "docs/a.md", nil, base)
	if err != nil {
		t.Fatal(err)
	}
	var frag fragment
	if err := decode(f, &frag); err != nil {
		t.Fatal(err)
	}
	if text(frag.Node, "label") != "(untitled)" || len(frag.Edges) != 0 {
		t.Fatal(string(f))
	}
	merged, err := (Fragmenter{}).Merge(context.Background(), base, []json.RawMessage{f})
	if err != nil {
		t.Fatal(err)
	}
	result, err := parse(merged)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(result.edges, []object{recorded, other}) {
		t.Fatal(result.edges)
	}
	if result.stats["edges"] != json.Number("2") || result.stats["recordedEdges"] != json.Number("1") || result.stats["other"] != json.Number("42") || result.raw["extra"] != "preserved" {
		t.Fatal(result.raw)
	}
}

func TestEligibilityAndCitationLeap(t *testing.T) {
	f := Fragmenter{}
	paths, err := f.EligiblePaths(fixture())
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(paths, []string{"docs/a.md", "b.md"}) {
		t.Fatal(paths)
	}
	for _, tt := range []struct {
		text string
		leap bool
	}{
		{"# Text only", false},
		{"[[flake/policies.nix#BAD]]", false},
		{"[[flake/policies.nix#unknown]]", true},
		{"[[flake/policies.nix#real]]", true},
	} {
		leap, err := f.NeedsLeap(context.Background(), "docs/a.md", []byte(tt.text), fixture())
		if err != nil || leap != tt.leap {
			t.Fatalf("%q: %v %v", tt.text, leap, err)
		}
	}
	g, _ := parse(fixture())
	g.nodes[3]["cites"] = []string{"real"}
	g.raw["nodes"] = g.nodes
	base, _ := json.Marshal(g.raw)
	for _, text := range []string{"[[flake/policies.nix#real]] [[flake/policies.nix#real]]", "[[flake/policies.nix#real"} {
		leap, err := f.NeedsLeap(context.Background(), "docs/a.md", []byte(text), base)
		if err != nil || leap {
			t.Fatalf("duplicate or unterminated citation: %v %v", leap, err)
		}
	}
	leap, err := f.NeedsLeap(context.Background(), "docs/a.md", nil, base)
	if err != nil || !leap {
		t.Fatal("last citation removal must leap", err)
	}
}

func TestNormaliseAndMissingInputs(t *testing.T) {
	for input, want := range map[string]string{"../../b.md": "b.md", "docs/./../b.md": "b.md", "/a//b/../c.md": "a/c.md"} {
		if got := normalise(input); got != want {
			t.Fatalf("%s: %s", input, got)
		}
	}
	f := Fragmenter{}
	if _, err := f.Fragment(context.Background(), "missing.md", nil, fixture()); err == nil {
		t.Fatal("missing doc accepted")
	}
	if _, err := f.Fragment(context.Background(), Excluded, nil, fixture()); err == nil {
		t.Fatal("excluded doc accepted")
	}
	if _, err := f.EligiblePaths(json.RawMessage(`{}`)); err == nil {
		t.Fatal("invalid graph accepted")
	}
	if _, err := f.Merge(context.Background(), fixture(), []json.RawMessage{json.RawMessage(`{}`)}); err == nil {
		t.Fatal("invalid fragment accepted")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := f.Fragment(ctx, "docs/a.md", nil, fixture()); err == nil {
		t.Fatal("cancellation ignored")
	}
}
