package render

import (
	"encoding/json"

	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/runtime/primitive"
	renderpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/runtime/render"
)

// Dag node + Dag metadata keys for late-binding resolution. The Dag primitive is
// domain-agnostic, so render carries its binding contract in metadata:
//   - a node's render bindings live in node.Metadata[BindingsMetaKey] (JSON);
//   - the run-wide component->machine map lives in dag.Metadata[ComponentMachineMetaKey].
const (
	BindingsMetaKey         = "render.bindings"
	ComponentMachineMetaKey = "render.component_machine"
)

type metaBinding struct {
	Token        string   `json:"token"`
	ComponentIDs []string `json:"component_ids"`
	Attr         string   `json:"attr"`
}

// AttachBindings stores the node's render bindings in node metadata so the
// execute-seam resolver can act on them. No-op for an empty list.
func AttachBindings(node *primitive.Dag_Node, bindings []*renderpb.Config_Binding) {
	if len(bindings) == 0 {
		return
	}
	mb := make([]metaBinding, 0, len(bindings))
	for _, b := range bindings {
		mb = append(mb, metaBinding{Token: b.GetToken(), ComponentIDs: b.GetComponentIds(), Attr: b.GetAttr()})
	}
	data, err := json.Marshal(mb)
	if err != nil {
		return
	}
	if node.Metadata == nil {
		node.Metadata = map[string]string{}
	}
	node.Metadata[BindingsMetaKey] = string(data)
}

// NodeBindings returns the render bindings attached to the node (nil if none).
func NodeBindings(node *primitive.Dag_Node) ([]*renderpb.Config_Binding, error) {
	raw := node.GetMetadata()[BindingsMetaKey]
	if raw == "" {
		return nil, nil
	}
	var mb []metaBinding
	if err := json.Unmarshal([]byte(raw), &mb); err != nil {
		return nil, err
	}
	out := make([]*renderpb.Config_Binding, 0, len(mb))
	for _, b := range mb {
		out = append(out, &renderpb.Config_Binding{Token: b.Token, ComponentIds: b.ComponentIDs, Attr: b.Attr})
	}
	return out, nil
}

// EncodeComponentMachine serializes the run-wide component->machine map for
// dag.Metadata[ComponentMachineMetaKey].
func EncodeComponentMachine(m map[string]string) string {
	data, err := json.Marshal(m)
	if err != nil {
		return ""
	}
	return string(data)
}

// DecodeComponentMachine reads the component->machine map from a dag metadata value.
func DecodeComponentMachine(raw string) map[string]string {
	if raw == "" {
		return map[string]string{}
	}
	var m map[string]string
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		return map[string]string{}
	}
	return m
}
