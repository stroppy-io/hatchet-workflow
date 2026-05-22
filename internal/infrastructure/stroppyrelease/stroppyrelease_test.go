package stroppyrelease

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// sampleReleases is a captured-shape subset of the real
// GET https://api.github.com/repos/stroppy-io/stroppy/releases response: a mix
// of stable, prerelease, draft and nightly tags, intentionally out of order to
// exercise newest-first sorting.
const sampleReleases = `[
  {
    "tag_name": "v4.1.0",
    "name": "v4.1.0",
    "draft": false,
    "prerelease": false,
    "assets": [
      {"name": "stroppy_linux_amd64.tar.gz", "browser_download_url": "https://github.com/stroppy-io/stroppy/releases/download/v4.1.0/stroppy_linux_amd64.tar.gz"}
    ]
  },
  {
    "tag_name": "v4.3.0",
    "name": "v4.3.0",
    "draft": false,
    "prerelease": false,
    "assets": [
      {"name": "stroppy_linux_amd64.tar.gz", "browser_download_url": "https://github.com/stroppy-io/stroppy/releases/download/v4.3.0/stroppy_linux_amd64.tar.gz"},
      {"name": "stroppy_darwin_arm64.tar.gz", "browser_download_url": "https://github.com/stroppy-io/stroppy/releases/download/v4.3.0/stroppy_darwin_arm64.tar.gz"}
    ]
  },
  {
    "tag_name": "v4.4.0-rc1",
    "name": "v4.4.0-rc1",
    "draft": false,
    "prerelease": true,
    "assets": []
  },
  {
    "tag_name": "v4.2.1",
    "name": "v4.2.1",
    "draft": false,
    "prerelease": false,
    "assets": []
  },
  {
    "tag_name": "v9.9.9-draft",
    "name": "draft",
    "draft": true,
    "prerelease": false,
    "assets": []
  },
  {
    "tag_name": "nightly-abc1234",
    "name": "nightly-abc1234",
    "draft": false,
    "prerelease": true,
    "assets": [
      {"name": "stroppy", "browser_download_url": "https://github.com/stroppy-io/stroppy/releases/download/nightly-abc1234/stroppy"}
    ]
  }
]`

func newTestClient(t *testing.T, handler http.HandlerFunc) (*Client, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return New(WithBaseURL(srv.URL), WithHTTPClient(srv.Client())), srv
}

func TestVersions_ParsesAndSortsNewestFirst(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Accept"); got != "application/vnd.github+json" {
			t.Errorf("Accept header = %q, want application/vnd.github+json", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(sampleReleases))
	})

	versions, err := c.Versions(context.Background())
	if err != nil {
		t.Fatalf("Versions: unexpected error: %v", err)
	}
	want := []string{"v4.4.0-rc1", "v4.3.0", "v4.2.1", "v4.1.0"}
	if len(versions) != len(want) {
		t.Fatalf("Versions = %v, want %v", versions, want)
	}
	for i := range want {
		if versions[i] != want[i] {
			t.Fatalf("Versions = %v, want %v", versions, want)
		}
	}
}

func TestLatest_PrefersNewestStable(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(sampleReleases))
	})

	latest, err := c.Latest(context.Background())
	if err != nil {
		t.Fatalf("Latest: unexpected error: %v", err)
	}
	// v4.4.0-rc1 is newer numerically but is a prerelease; latest stable is v4.3.0.
	if latest != "v4.3.0" {
		t.Fatalf("Latest = %q, want v4.3.0", latest)
	}
}

func TestAssetURL_FindsAssetWithOrWithoutVPrefix(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(sampleReleases))
	})

	url, err := c.AssetURL(context.Background(), "4.3.0", "stroppy_darwin_arm64.tar.gz")
	if err != nil {
		t.Fatalf("AssetURL: unexpected error: %v", err)
	}
	want := "https://github.com/stroppy-io/stroppy/releases/download/v4.3.0/stroppy_darwin_arm64.tar.gz"
	if url != want {
		t.Fatalf("AssetURL = %q, want %q", url, want)
	}

	if _, err := c.AssetURL(context.Background(), "v4.3.0", "nope.tar.gz"); err == nil {
		t.Fatal("AssetURL: expected error for missing asset, got nil")
	}
	if _, err := c.AssetURL(context.Background(), "v0.0.0", "stroppy_linux_amd64.tar.gz"); err == nil {
		t.Fatal("AssetURL: expected error for missing release, got nil")
	}
}

func TestCache_AvoidsRepeatHits(t *testing.T) {
	var hits int
	c, _ := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		hits++
		_, _ = w.Write([]byte(sampleReleases))
	})
	c.ttl = time.Hour

	for i := 0; i < 3; i++ {
		if _, err := c.Versions(context.Background()); err != nil {
			t.Fatalf("Versions[%d]: %v", i, err)
		}
	}
	if hits != 1 {
		t.Fatalf("github hit %d times, want 1 (cache should collapse repeats)", hits)
	}
}

func TestNetworkFailure_DegradesGracefully(t *testing.T) {
	// First serve a good response to warm the cache, then fail and force a refresh.
	var fail bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if fail {
			http.Error(w, "boom", http.StatusInternalServerError)
			return
		}
		_, _ = w.Write([]byte(sampleReleases))
	}))
	t.Cleanup(srv.Close)
	c := New(WithBaseURL(srv.URL), WithHTTPClient(srv.Client()), WithTTL(0))

	warm, err := c.Versions(context.Background())
	if err != nil || len(warm) == 0 {
		t.Fatalf("warm Versions: err=%v len=%d", err, len(warm))
	}

	fail = true
	// TTL=0 forces a refresh; the refresh fails, so we expect the stale cache + error.
	got, err := c.Versions(context.Background())
	if err == nil {
		t.Fatal("expected error on failed refresh, got nil")
	}
	if len(got) != len(warm) {
		t.Fatalf("on failure expected stale cache (%d versions), got %d", len(warm), len(got))
	}
}

func TestNetworkFailure_NoCacheReturnsEmptyPlusError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "boom", http.StatusBadGateway)
	}))
	t.Cleanup(srv.Close)
	c := New(WithBaseURL(srv.URL), WithHTTPClient(srv.Client()))

	got, err := c.Versions(context.Background())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if len(got) != 0 {
		t.Fatalf("expected empty versions on cold failure, got %v", got)
	}
}
