package ide

import "testing"

func TestParseScope_Instance(t *testing.T) {
	s, err := ParseScope("/ide/instance/provider/docker/")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if s.Kind != ScopeInstance || s.EntryKind != EntryKindProvider || s.EntrySlug != "docker" || s.Rest != "/" {
		t.Fatalf("got %+v", s)
	}
	if s.Key() != "instance:provider:docker" {
		t.Fatalf("key = %q", s.Key())
	}
}

func TestParseScope_InstanceWithSubPath(t *testing.T) {
	s, err := ParseScope("/ide/instance/provider/pg-ha/manifest.yaml")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if s.Kind != ScopeInstance || s.EntrySlug != "pg-ha" || s.Rest != "/manifest.yaml" {
		t.Fatalf("got %+v", s)
	}
}

func TestParseScope_InstanceWorkflow(t *testing.T) {
	s, err := ParseScope("/ide/instance/workflow/postgres-ha")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if s.Kind != ScopeInstance || s.EntryKind != EntryKindWorkflow || s.EntrySlug != "postgres-ha" || s.Rest != "/" {
		t.Fatalf("got %+v", s)
	}
}

func TestParseScope_Org(t *testing.T) {
	s, err := ParseScope("/ide/org/acme/workflow/oltp/")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if s.Kind != ScopeOrg || s.OrgSlug != "acme" || s.EntryKind != EntryKindWorkflow || s.EntrySlug != "oltp" || s.Rest != "/" {
		t.Fatalf("got %+v", s)
	}
	if s.Key() != "org:acme:workflow:oltp" {
		t.Fatalf("key = %q", s.Key())
	}
}

func TestParseScope_OrgWithSubPath(t *testing.T) {
	s, err := ParseScope("/ide/org/acme/workflow/oltp/cluster.yaml")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if s.Kind != ScopeOrg || s.OrgSlug != "acme" || s.EntrySlug != "oltp" || s.Rest != "/cluster.yaml" {
		t.Fatalf("got %+v", s)
	}
}

func TestParseScope_RejectsMissingOrgSlug(t *testing.T) {
	if _, err := ParseScope("/ide/org/"); err == nil {
		t.Fatal("expected error for missing org slug")
	}
	if _, err := ParseScope("/ide/org"); err == nil {
		t.Fatal("expected error for missing org slug")
	}
}

func TestParseScope_RejectsMissingEntryKindOrSlug(t *testing.T) {
	cases := []string{
		"/ide/instance",
		"/ide/instance/",
		"/ide/instance/provider",
		"/ide/instance/provider/",
		"/ide/org/acme",
		"/ide/org/acme/",
		"/ide/org/acme/provider",
		"/ide/org/acme/provider/",
	}
	for _, p := range cases {
		if _, err := ParseScope(p); err == nil {
			t.Fatalf("ParseScope(%q): expected error for missing kind/slug", p)
		}
	}
}

func TestParseScope_RejectsUnrecognizedEntryKind(t *testing.T) {
	if _, err := ParseScope("/ide/instance/database/docker"); err == nil {
		t.Fatal("expected error for an entry kind other than provider/workflow")
	}
	if _, err := ParseScope("/ide/org/acme/database/docker"); err == nil {
		t.Fatal("expected error for an entry kind other than provider/workflow")
	}
}

func TestParseScope_RejectsUnknownScope(t *testing.T) {
	if _, err := ParseScope("/ide/whatever/acme"); err == nil {
		t.Fatal("expected error for unrecognized scope segment")
	}
}

func TestParseScope_RejectsNonIdePath(t *testing.T) {
	if _, err := ParseScope("/grafana/"); err == nil {
		t.Fatal("expected error for a path not under /ide/")
	}
}

// TestParseScope_OrgSlugCannotEscapeIntoAnotherScope proves the parser
// treats the org slug as an opaque path segment — a slug crafted to look
// like a path-traversal ("..", or containing "/instance") can never widen
// the parsed scope to ScopeInstance or a different org, because SplitN
// caps the org slug at exactly one segment.
func TestParseScope_OrgSlugCannotEscapeIntoAnotherScope(t *testing.T) {
	s, err := ParseScope("/ide/org/..%2Finstance/provider/docker")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if s.Kind != ScopeOrg || s.OrgSlug != "..%2Finstance" {
		t.Fatalf("slug was not treated as an opaque segment: %+v", s)
	}
}

// TestParseScope_EntrySlugCannotEscapeEntryKind proves a hostile entry slug
// (path traversal, or a slug that looks like another kind segment) is still
// treated as an opaque single path segment, never reinterpreted.
func TestParseScope_EntrySlugCannotEscapeEntryKind(t *testing.T) {
	s, err := ParseScope("/ide/instance/provider/..%2Fworkflow%2Fsecret")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if s.EntryKind != EntryKindProvider || s.EntrySlug != "..%2Fworkflow%2Fsecret" {
		t.Fatalf("entry slug was not treated as opaque: %+v", s)
	}
}

func TestScope_KeyDistinguishesEveryDimension(t *testing.T) {
	a, _ := ParseScope("/ide/org/acme/provider/docker")
	b, _ := ParseScope("/ide/org/beta/provider/docker")
	c, _ := ParseScope("/ide/org/acme/workflow/docker")
	d, _ := ParseScope("/ide/instance/provider/docker")
	keys := map[string]bool{}
	for _, s := range []Scope{a, b, c, d} {
		if keys[s.Key()] {
			t.Fatalf("key collision: %q", s.Key())
		}
		keys[s.Key()] = true
	}
}
