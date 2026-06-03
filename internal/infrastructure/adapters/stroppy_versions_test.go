package adapters

import (
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"
)

func TestGitHubStroppyVersionSourceListFiltersNormalizesSortsAndCaches(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if got, want := r.URL.Path, "/repos/stroppy-io/stroppy/releases"; got != want {
			t.Fatalf("path = %s, want %s", got, want)
		}
		if got, want := r.Header.Get("Authorization"), "Bearer gh-token"; got != want {
			t.Fatalf("authorization = %s, want %s", got, want)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[
			{"tag_name":"v1.10.0","draft":false,"prerelease":false},
			{"tag_name":"v1.2.0","draft":false,"prerelease":false},
			{"tag_name":"v1.1.9","draft":false,"prerelease":false},
			{"tag_name":"v2.0.0-rc.1","draft":false,"prerelease":true},
			{"tag_name":"v3.0.0","draft":true,"prerelease":false},
			{"tag_name":"not-a-version","draft":false,"prerelease":false}
		]`))
	}))
	defer server.Close()

	source, err := NewGitHubStroppyVersionSource(GitHubStroppyVersionConfig{
		Repo:       "https://github.com/stroppy-io/stroppy.git",
		MinVersion: "v1.2.0",
		Token:      "gh-token",
		TTL:        time.Hour,
		BaseURL:    server.URL,
	})
	if err != nil {
		t.Fatalf("new source: %v", err)
	}

	got, err := source.List(context.Background())
	if err != nil {
		t.Fatalf("list versions: %v", err)
	}
	want := []string{"1.10.0", "1.2.0"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("versions = %#v, want %#v", got, want)
	}

	got, err = source.List(context.Background())
	if err != nil {
		t.Fatalf("list cached versions: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("cached versions = %#v, want %#v", got, want)
	}
	if requests != 1 {
		t.Fatalf("requests = %d, want 1", requests)
	}
}
