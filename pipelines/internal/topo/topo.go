// Package topo holds the pure, deterministic helpers the pipelines order
// their work with: topological sorting of dependency graphs, grouping by
// role, stable naming. Nothing here touches Temporal, so everything is
// unit-tested and safe to call from workflow code.
package topo

import (
	"fmt"
	"sort"
	"strings"
)

// Node is anything with a name and dependencies on other names.
type Node interface {
	NodeName() string
	NodeDeps() []string
}

// Layers sorts nodes into dependency layers: every node of layer i depends
// only on nodes of layers < i. Nodes inside a layer are ordered by their
// input position, so the result is deterministic — a requirement for
// workflow code. Unknown dependencies and cycles are errors that name the
// culprit.
func Layers[N Node](nodes []N) ([][]N, error) {
	index := make(map[string]int, len(nodes))
	for i, n := range nodes {
		name := n.NodeName()
		if _, dup := index[name]; dup {
			return nil, fmt.Errorf("topo: duplicate node %q", name)
		}
		index[name] = i
	}
	for _, n := range nodes {
		for _, d := range n.NodeDeps() {
			if _, ok := index[d]; !ok {
				return nil, fmt.Errorf("topo: %q depends on unknown %q", n.NodeName(), d)
			}
		}
	}
	placed := make([]int, len(nodes)) // layer+1; 0 = not placed
	var layers [][]N
	remaining := len(nodes)
	for remaining > 0 {
		var layer []N
		for i, n := range nodes {
			if placed[i] != 0 {
				continue
			}
			ready := true
			for _, d := range n.NodeDeps() {
				if placed[index[d]] == 0 {
					ready = false
					break
				}
			}
			if ready {
				layer = append(layer, n)
			}
		}
		if len(layer) == 0 {
			var stuck []string
			for i, n := range nodes {
				if placed[i] == 0 {
					stuck = append(stuck, n.NodeName())
				}
			}
			sort.Strings(stuck)
			return nil, fmt.Errorf("topo: dependency cycle among %s", strings.Join(stuck, ", "))
		}
		for _, n := range layer {
			placed[index[n.NodeName()]] = len(layers) + 1
		}
		layers = append(layers, layer)
		remaining -= len(layer)
	}
	return layers, nil
}

// Order flattens Layers into one dependency-respecting sequence.
func Order[N Node](nodes []N) ([]N, error) {
	layers, err := Layers(nodes)
	if err != nil {
		return nil, err
	}
	out := make([]N, 0, len(nodes))
	for _, l := range layers {
		out = append(out, l...)
	}
	return out, nil
}

// GroupBy buckets items by key, keeping the input order inside a bucket,
// and returns the keys in first-seen order — deterministic iteration for
// workflow code (a Go map alone is not).
func GroupBy[T any](items []T, key func(T) string) (keys []string, groups map[string][]T) {
	groups = map[string][]T{}
	for _, it := range items {
		k := key(it)
		if _, seen := groups[k]; !seen {
			keys = append(keys, k)
		}
		groups[k] = append(groups[k], it)
	}
	return keys, groups
}

// SortedKeys returns the keys of a map in sorted order.
func SortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// Slug turns any string into a DNS-ish label: lowercase, [a-z0-9-], no
// leading/trailing dashes, at most max characters. Empty input gives "x".
func Slug(s string, limit int) string {
	var b strings.Builder
	lastDash := true
	for _, r := range strings.ToLower(s) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			lastDash = false
		default:
			if !lastDash {
				b.WriteByte('-')
				lastDash = true
			}
		}
	}
	out := strings.TrimRight(b.String(), "-")
	if limit > 0 && len(out) > limit {
		out = strings.TrimRight(out[:limit], "-")
	}
	if out == "" {
		return "x"
	}
	return out
}
