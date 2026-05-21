package render

import (
	"testing"

	renderpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/runtime/render"
)

func TestResolveTextSubstitutesTokens(t *testing.T) {
	r := NewResolver(Resolved{
		"db-1": {AttrPrivateIP: "10.0.0.5", AttrPublicIP: "203.0.113.5"},
	})
	out, err := r.ResolveText(
		"host=__DB_IP__ port=5432",
		[]*renderpb.Config_Binding{{Token: "__DB_IP__", ComponentIds: []string{"db-1"}, Attr: AttrPrivateIP}},
	)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if out != "host=10.0.0.5 port=5432" {
		t.Errorf("got %q", out)
	}
}

func TestResolveTextMultiComponentJoins(t *testing.T) {
	r := NewResolver(Resolved{
		"etcd-1": {AttrPrivateIP: "10.0.0.1"},
		"etcd-2": {AttrPrivateIP: "10.0.0.2"},
	})
	out, err := r.ResolveText(
		"hosts=__ETCD__",
		[]*renderpb.Config_Binding{{Token: "__ETCD__", ComponentIds: []string{"etcd-1", "etcd-2"}, Attr: AttrPrivateIP}},
	)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if out != "hosts=10.0.0.1,10.0.0.2" {
		t.Errorf("got %q", out)
	}
}

func TestResolveTextUnresolvedIsError(t *testing.T) {
	r := NewResolver(Resolved{})
	if _, err := r.ResolveText(
		"x=__MISSING__",
		[]*renderpb.Config_Binding{{Token: "__MISSING__", ComponentIds: []string{"nope"}, Attr: AttrPrivateIP}},
	); err == nil {
		t.Fatal("expected error for unresolved binding")
	}
}

func TestFromTerraformMapsVmIps(t *testing.T) {
	outputs := map[string][]byte{
		"vm_ips": []byte(`{"db-1":{"id":"x","nat_ip":"203.0.113.5","internal_ip":"10.0.0.5"}}`),
	}
	resolved, err := FromTerraform(outputs, map[string]string{"pg": "db-1"})
	if err != nil {
		t.Fatalf("from terraform: %v", err)
	}
	if resolved["pg"][AttrPrivateIP] != "10.0.0.5" {
		t.Errorf("private_ip = %q", resolved["pg"][AttrPrivateIP])
	}
	if resolved["pg"][AttrPublicIP] != "203.0.113.5" {
		t.Errorf("public_ip = %q", resolved["pg"][AttrPublicIP])
	}
}
