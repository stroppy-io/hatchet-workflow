package ast

import (
	"fmt"

	"gopkg.in/yaml.v3"

	"github.com/stroppy-io/stroppy-cloud/internal/dsl/diag"
)

// posOf converts a yaml.Node's Line/Column into a diag.Pos. A nil node
// yields the zero Pos.
func posOf(n *yaml.Node) diag.Pos {
	if n == nil {
		return diag.Pos{}
	}
	return diag.Pos{Line: n.Line, Col: n.Column}
}

// nodeKindName renders a yaml.Kind as a short human-readable word, used in
// diagnostic and error messages.
func nodeKindName(k yaml.Kind) string {
	switch k {
	case yaml.DocumentNode:
		return "document"
	case yaml.MappingNode:
		return "mapping"
	case yaml.SequenceNode:
		return "sequence"
	case yaml.ScalarNode:
		return "scalar"
	case yaml.AliasNode:
		return "alias"
	default:
		return "unknown"
	}
}

// decodeMapping walks a YAML mapping node key by key. For every key present
// in handlers, the matching function is called with that key's value node;
// a non-nil error returned by a handler aborts the walk (handlers that want
// to keep collecting sibling diagnostics should record them directly, e.g.
// via a closed-over diag.List, and return nil).
//
// Any key absent from handlers is reported through onUnknown, which
// receives both the key node (for its exact source position) and the value
// node (so callers can, for example, capture the block into an ext/extra
// map instead of erroring). onUnknown may be nil, in which case unknown
// keys are silently skipped by the caller's own contract — callers wanting
// strict decoding must pass one that records a diagnostic.
//
// decodeMapping is intentionally generic (not tied to the cluster.yaml
// schema) so Tasks 3/4 can reuse it for the workflow/component documents.
func decodeMapping(
	node *yaml.Node,
	handlers map[string]func(*yaml.Node) error,
	onUnknown func(key string, keyNode, valNode *yaml.Node),
) error {
	if node == nil {
		return nil
	}
	if node.Kind != yaml.MappingNode {
		return fmt.Errorf("expected a mapping, got %s", nodeKindName(node.Kind))
	}
	for i := 0; i+1 < len(node.Content); i += 2 {
		keyNode := node.Content[i]
		valNode := node.Content[i+1]

		handler, ok := handlers[keyNode.Value]
		if !ok {
			if onUnknown != nil {
				onUnknown(keyNode.Value, keyNode, valNode)
			}
			continue
		}
		if err := handler(valNode); err != nil {
			return fmt.Errorf("%s: %w", keyNode.Value, err)
		}
	}
	return nil
}
