// Package kgdoc implements nixconfig's document-local knowledge graph projection.
package kgdoc

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path"
	"regexp"
	"strings"
)

const Excluded = ".agents/skills/router/references/playbooks.md"

type Fragmenter struct{}

type object = map[string]any

type graph struct {
	raw          object
	nodes, edges []object
	stats        object
}

type fragment struct {
	Node  object   `json:"node"`
	Edges []object `json:"edges"`
}

func decode(b []byte, v any) error {
	if !json.Valid(b) {
		return errors.New("invalid graph JSON")
	}
	d := json.NewDecoder(bytes.NewReader(b))
	d.UseNumber()
	return d.Decode(v)
}

func objects(v any) ([]object, error) {
	array, ok := v.([]any)
	if !ok {
		return nil, errors.New("graph nodes and edges must be arrays")
	}
	out := make([]object, 0, len(array))
	for _, item := range array {
		o, ok := item.(map[string]any)
		if !ok {
			return nil, errors.New("graph entries must be objects")
		}
		out = append(out, o)
	}
	return out, nil
}

func parse(base json.RawMessage) (graph, error) {
	var g graph
	if err := decode(base, &g.raw); err != nil {
		return g, err
	}
	var err error
	if g.nodes, err = objects(g.raw["nodes"]); err != nil {
		return g, err
	}
	if g.edges, err = objects(g.raw["edges"]); err != nil {
		return g, err
	}
	var ok bool
	if g.stats, ok = g.raw["stats"].(map[string]any); !ok {
		return g, errors.New("graph stats must be an object")
	}
	return g, nil
}

func text(o object, key string) string { s, _ := o[key].(string); return s }

func (Fragmenter) EligiblePaths(base json.RawMessage) ([]string, error) {
	g, err := parse(base)
	if err != nil {
		return nil, err
	}
	var paths []string
	for _, node := range g.nodes {
		if text(node, "type") == "doc" && text(node, "path") != Excluded {
			paths = append(paths, text(node, "path"))
		}
	}
	return paths, nil
}

var policySyntax = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)
var tokenSplit = regexp.MustCompile(`[^A-Za-z0-9_-]+`)

func citations(content string) []string {
	out := []string{}
	seen := make(map[string]bool)
	for _, chunk := range strings.Split(content, "[[flake/policies.nix#")[1:] {
		id := strings.SplitN(chunk, "]]", 2)[0]
		if policySyntax.MatchString(id) && !seen[id] {
			out = append(out, id)
			seen[id] = true
		}
	}
	return out
}

func findDoc(g graph, file string) (object, error) {
	for _, n := range g.nodes {
		if text(n, "type") == "doc" && text(n, "path") == file && file != Excluded {
			return n, nil
		}
	}
	return nil, fmt.Errorf("not an eligible core doc: %s", file)
}

func (Fragmenter) NeedsLeap(ctx context.Context, file string, content []byte, base json.RawMessage) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	g, err := parse(base)
	if err != nil {
		return false, err
	}
	node, err := findDoc(g, file)
	if err != nil {
		return false, err
	}
	old := make(map[string]bool)
	if cites, ok := node["cites"].([]any); ok {
		for _, id := range cites {
			if s, ok := id.(string); ok {
				old[s] = true
			}
		}
	}
	fresh := citations(string(content))
	if len(old) != len(fresh) {
		return true, nil
	}
	for _, id := range fresh {
		if !old[id] {
			return true, nil
		}
	}
	return false, nil
}

func normalise(p string) string {
	var parts []string
	for _, s := range strings.Split(p, "/") {
		switch s {
		case "", ".":
		case "..":
			if len(parts) > 0 {
				parts = parts[:len(parts)-1]
			}
		default:
			parts = append(parts, s)
		}
	}
	return strings.Join(parts, "/")
}

func (Fragmenter) Fragment(ctx context.Context, file string, content []byte, base json.RawMessage) (json.RawMessage, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	g, err := parse(base)
	if err != nil {
		return nil, err
	}
	node, err := findDoc(g, file)
	if err != nil {
		return nil, err
	}
	s := string(content)
	tokens := make(map[string]bool)
	for _, token := range tokenSplit.Split(s, -1) {
		tokens[token] = true
	}
	docs, policies := make(map[string]bool), make(map[string]bool)
	for _, n := range g.nodes {
		if text(n, "type") == "doc" {
			docs[text(n, "path")] = true
		}
		if text(n, "type") == "policy" {
			policies[strings.TrimPrefix(text(n, "id"), "policy:")] = true
		}
	}
	cites := []string{}
	for _, id := range citations(s) {
		if policies[id] {
			cites = append(cites, id)
		}
	}
	references := []string{}
	seen := make(map[string]bool)
	for _, chunk := range strings.Split(s, "](")[1:] {
		ref := strings.SplitN(chunk, ")", 2)[0]
		ref = strings.SplitN(ref, "#", 2)[0]
		ref = strings.TrimSuffix(strings.TrimPrefix(ref, "<"), ">")
		if !strings.HasSuffix(ref, ".md") || strings.HasPrefix(ref, "/") || strings.HasPrefix(ref, "http") || strings.HasPrefix(ref, "mailto:") {
			continue
		}
		ref = normalise(path.Dir(file) + "/" + ref)
		if ref != file && docs[ref] && !seen[ref] {
			references = append(references, ref)
			seen[ref] = true
		}
	}
	label := "(untitled)"
	for _, line := range strings.Split(s, "\n") {
		if strings.HasPrefix(line, "# ") {
			label = strings.TrimPrefix(line, "# ")
			break
		}
	}
	node["label"], node["cites"] = label, cites
	f := fragment{Node: node, Edges: []object{}}
	add := func(kind, prefix string, ids []string) {
		for _, id := range ids {
			f.Edges = append(f.Edges, object{"from": text(node, "id"), "to": prefix + ":" + id, "type": kind})
		}
	}
	for _, kind := range []string{"host", "skill"} {
		var names []string
		for _, n := range g.nodes {
			if text(n, "type") == kind && tokens[text(n, "label")] {
				names = append(names, text(n, "label"))
			}
		}
		add("mentions", kind, names)
	}
	add("references", "doc", references)
	add("cites", "policy", cites)
	return json.Marshal(f)
}

func (Fragmenter) Merge(ctx context.Context, base json.RawMessage, fragments []json.RawMessage) (json.RawMessage, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	g, err := parse(base)
	if err != nil {
		return nil, err
	}
	replacements := make(map[string]object)
	var added []object
	for _, raw := range fragments {
		var f fragment
		if err := decode(raw, &f); err != nil {
			return nil, err
		}
		if text(f.Node, "id") == "" {
			return nil, errors.New("fragment node requires id")
		}
		replacements[text(f.Node, "id")] = f.Node
		added = append(added, f.Edges...)
	}
	edges := []object{}
	for _, e := range g.edges {
		_, replace := replacements[text(e, "from")]
		kind := text(e, "type")
		if !replace || (kind != "mentions" && kind != "references" && kind != "cites") {
			edges = append(edges, e)
		}
	}
	edges = append(edges, added...)
	for i, n := range g.nodes {
		if replacement, ok := replacements[text(n, "id")]; ok {
			g.nodes[i] = replacement
		}
	}
	recorded := 0
	for _, e := range edges {
		if _, ok := e["evidence"]; ok {
			recorded++
		}
	}
	g.stats["edges"], g.stats["recordedEdges"] = len(edges), recorded
	g.raw["nodes"], g.raw["edges"] = g.nodes, edges
	return json.Marshal(g.raw)
}
