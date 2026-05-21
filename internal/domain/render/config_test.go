package render

import (
	"strings"
	"testing"

	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/domain"
)

func TestRenderDatabasePostgres(t *testing.T) {
	cfg, err := RenderDatabase(&domain.Database{Kind: domain.Database_KIND_POSTGRES, Version: "16"}, 4096)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if len(cfg.GetItems()) != 2 {
		t.Fatalf("want 2 items, got %d", len(cfg.GetItems()))
	}
	var conf string
	for _, it := range cfg.GetItems() {
		if it.GetId() == "postgresql.conf" {
			conf = it.GetFile().GetContent().GetText()
		}
	}
	if conf == "" {
		t.Fatal("postgresql.conf not rendered")
	}
	// 25% of 4096MB = 1024MB.
	if !strings.Contains(conf, "shared_buffers = 1024MB") {
		t.Errorf("shared_buffers not resolved from percent:\n%s", conf)
	}
	if !strings.Contains(conf, "listen_addresses = '*'") {
		t.Error("missing listen_addresses")
	}
}

func TestRenderDatabaseUnsupported(t *testing.T) {
	if _, err := RenderDatabase(&domain.Database{Kind: domain.Database_KIND_COCKROACH}, 1024); err == nil {
		t.Fatal("expected error for unsupported kind")
	}
}
