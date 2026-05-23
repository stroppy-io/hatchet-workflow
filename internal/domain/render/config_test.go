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

func TestRenderDatabaseMySQL(t *testing.T) {
	cfg, err := RenderDatabase(&domain.Database{Kind: domain.Database_KIND_MYSQL, Version: "8.4"}, 4096)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	item := cfg.GetItems()[0].GetFile()
	if got := item.GetInfo().GetPath(); got != "/etc/mysql/mysql.conf.d/zz-stroppy.cnf" {
		t.Fatalf("unexpected mysql config path: %s", got)
	}
	conf := item.GetContent().GetText()
	if !strings.Contains(conf, "[mysqld]") || !strings.Contains(conf, "innodb_buffer_pool_size = 2048M") {
		t.Errorf("unexpected my.cnf:\n%s", conf)
	}
}

func TestRenderDatabaseMariaDB(t *testing.T) {
	cfg, err := RenderDatabase(&domain.Database{Kind: domain.Database_KIND_MARIADB, Version: "11.4"}, 4096)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	item := cfg.GetItems()[0].GetFile()
	if got := item.GetInfo().GetPath(); got != "/etc/mysql/mariadb.conf.d/zz-stroppy.cnf" {
		t.Fatalf("unexpected mariadb config path: %s", got)
	}
	conf := item.GetContent().GetText()
	if !strings.Contains(conf, "[mysqld]") || !strings.Contains(conf, "bind-address = 0.0.0.0") {
		t.Errorf("unexpected mariadb my.cnf:\n%s", conf)
	}
}

func TestRenderDatabaseUnsupported(t *testing.T) {
	if _, err := RenderDatabase(&domain.Database{Kind: domain.Database_KIND_UNSPECIFIED}, 1024); err == nil {
		t.Fatal("expected error for unspecified kind")
	}
}
