// Package render holds the render-binding resolver and (later) the config
// renderer. A render.Binding is a typed late-binding hole left in rendered
// content (token + component_ids + attr); at the plan->execute seam the resolver
// replaces the token with the runtime value of attr for the referenced topology
// component(s), read from the provisioning output (TfOperation.Output, H27).
// Anti-leak: content carrying an unresolved binding is invalid.
package render

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/deployment"
	renderpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/runtime/render"
)

// Standard binding attrs (free strings, self-describing — a new attr needs no
// proto change; H4/H46).
const (
	AttrPrivateIP = "private_ip"
	AttrPublicIP  = "public_ip"
	AttrEndpoint  = "endpoint"
)

// Resolved holds runtime values keyed by component id, then by attr.
type Resolved map[string]map[string]string

// Resolver substitutes render.Binding tokens in rendered content.
type Resolver struct {
	values Resolved
}

// NewResolver builds a resolver over the given resolved values.
func NewResolver(values Resolved) *Resolver {
	return &Resolver{values: values}
}

// ResolveText replaces every binding token in content with its runtime value.
// An unresolved binding is an error: content carrying an unresolved binding must
// never reach a WRITE_FILE / agent command (anti-leak, B3 invariant).
func (r *Resolver) ResolveText(content string, bindings []*renderpb.Config_Binding) (string, error) {
	for _, b := range bindings {
		val, err := r.value(b.GetComponentIds(), b.GetAttr())
		if err != nil {
			return "", fmt.Errorf("resolve binding %q: %w", b.GetToken(), err)
		}
		content = strings.ReplaceAll(content, b.GetToken(), val)
	}
	return content, nil
}

// value resolves attr across one or more components, joining multiple values with
// a comma (e.g. an etcd HOSTS list); a single component yields a single value.
func (r *Resolver) value(componentIDs []string, attr string) (string, error) {
	parts := make([]string, 0, len(componentIDs))
	for _, id := range componentIDs {
		attrs, ok := r.values[id]
		if !ok {
			return "", fmt.Errorf("no runtime values for component %q", id)
		}
		v, ok := attrs[attr]
		if !ok || v == "" {
			return "", fmt.Errorf("component %q has no %q", id, attr)
		}
		parts = append(parts, v)
	}
	return strings.Join(parts, ","), nil
}

// vmIP mirrors the per-vm object in the terraform "vm_ips" / "vms" output.
type vmIP struct {
	InternalIP string `json:"internal_ip"`
	NatIP      string `json:"nat_ip"`
	PublicIP   string `json:"public_ip"`
}

// FromTerraform builds Resolved from a terraform outputs_json map and a
// component->machine map. The module's vm_ips output is keyed by machine id; each
// topology component inherits its machine's addresses.
//
// TODO(render): only vm IPs are mapped (private/public/endpoint). Managed-service
// endpoints (ydb_endpoint), ports, and other attrs are not wired. Reported.
// FromDeployment resolves component runtime values from a provisioned
// deployment.Deployment Output (Docker container IPs or Yandex VM IPs), keyed by the
// component->machine map. This is the proto-native binding source for the blueprint
// dag (the deploy node's output), replacing the legacy terraform-output path.
func FromDeployment(dep *deployment.Deployment, componentToMachine map[string]string) Resolved {
	type ip struct{ private, public string }
	byMachine := map[string]ip{}
	for mid, c := range dep.GetDocker().GetOutput().GetContainers() {
		byMachine[mid] = ip{private: c.GetInternalIp()}
	}
	for mid, v := range dep.GetYandex().GetOutput().GetVms() {
		byMachine[mid] = ip{private: v.GetInternalIp(), public: v.GetPublicIp()}
	}
	out := make(Resolved, len(componentToMachine))
	for component, machine := range componentToMachine {
		m, ok := byMachine[machine]
		if !ok {
			continue
		}
		out[component] = map[string]string{
			AttrPrivateIP: m.private,
			AttrEndpoint:  m.private,
			AttrPublicIP:  m.public,
		}
	}
	return out
}

func FromTerraform(outputsJSON map[string][]byte, componentToMachine map[string]string) (Resolved, error) {
	raw, ok := outputsJSON["vm_ips"]
	if !ok {
		raw, ok = outputsJSON["vms"]
	}
	if !ok {
		return Resolved{}, nil // nothing provisioned to resolve (e.g. external DB)
	}
	var vms map[string]vmIP
	if err := json.Unmarshal(raw, &vms); err != nil {
		return nil, fmt.Errorf("render: decode terraform vm output: %w", err)
	}
	out := make(Resolved, len(componentToMachine))
	for component, machine := range componentToMachine {
		vm, ok := vms[machine]
		if !ok {
			continue
		}
		public := vm.PublicIP
		if public == "" {
			public = vm.NatIP
		}
		out[component] = map[string]string{
			AttrPrivateIP: vm.InternalIP,
			AttrEndpoint:  vm.InternalIP,
			AttrPublicIP:  public,
		}
	}
	return out, nil
}
