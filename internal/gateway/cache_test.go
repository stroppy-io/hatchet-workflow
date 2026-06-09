package gateway

import "testing"

func TestCockroachBinaryUpstreamUsesReleaseStorage(t *testing.T) {
	tmpl, ok := binaryUpstreams["cockroach"]
	if !ok {
		t.Fatal("cockroach upstream is not configured")
	}

	got := resolveBinaryUpstream(tmpl, "23.2.5", "cockroach-v23.2.5.linux-amd64.tgz")
	want := "https://storage.googleapis.com/cockroach-release-artifacts-prod/cockroach-v23.2.5.linux-amd64.tgz"
	if got != want {
		t.Fatalf("cockroach upstream = %q, want %q", got, want)
	}
}
