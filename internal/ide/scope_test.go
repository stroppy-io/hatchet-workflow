package ide

import "testing"

func TestParseScope_Instance(t *testing.T) {
	s, err := ParseScope("/ide/instance/")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if s.Kind != ScopeInstance || s.Rest != "/" || s.Key() != "instance" {
		t.Fatalf("got %+v", s)
	}
}

func TestParseScope_InstanceWithSubPath(t *testing.T) {
	s, err := ParseScope("/ide/instance/providers/pg-ha/manifest.yaml")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if s.Kind != ScopeInstance || s.Rest != "/providers/pg-ha/manifest.yaml" {
		t.Fatalf("got %+v", s)
	}
}

func TestParseScope_Org(t *testing.T) {
	s, err := ParseScope("/ide/org/acme/")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if s.Kind != ScopeOrg || s.OrgSlug != "acme" || s.Rest != "/" || s.Key() != "org:acme" {
		t.Fatalf("got %+v", s)
	}
}

func TestParseScope_OrgWithSubPath(t *testing.T) {
	s, err := ParseScope("/ide/org/acme/workflows/oltp/cluster.yaml")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if s.Kind != ScopeOrg || s.OrgSlug != "acme" || s.Rest != "/workflows/oltp/cluster.yaml" {
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
	s, err := ParseScope("/ide/org/..%2Finstance/secret.yaml")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if s.Kind != ScopeOrg || s.OrgSlug != "..%2Finstance" {
		t.Fatalf("slug was not treated as an opaque segment: %+v", s)
	}
}
