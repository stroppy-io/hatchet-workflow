package dbconfig

import (
	"strings"
	"testing"
)

func TestRenderMySQLConf_PlaceholdersPresent(t *testing.T) {
	out := RenderMySQLConf(RenderMySQLConfOpts{Version: "8.0", Role: "primary", TotalMemoryMB: 4096})
	if !strings.Contains(out, "server-id = "+MySQLServerIDPlaceholder) {
		t.Errorf("server-id placeholder missing:\n%s", out)
	}
}

func TestRenderMySQLConf_GroupReplEmitsReportHostPlaceholder(t *testing.T) {
	out := RenderMySQLConf(RenderMySQLConfOpts{Version: "8.0", Role: "primary", GroupRepl: true, TotalMemoryMB: 4096})
	if !strings.Contains(out, "report_host = "+MySQLLocalHostPlaceholder) {
		t.Errorf("report_host placeholder missing under GR:\n%s", out)
	}
	if !strings.Contains(out, "plugin-load-add = group_replication.so") {
		t.Errorf("GR plugin-load-add missing:\n%s", out)
	}
}

func TestRenderMySQLConf_OptionsCannotOverrideServerID(t *testing.T) {
	out := RenderMySQLConf(RenderMySQLConfOpts{
		Version: "8.0", Role: "primary", TotalMemoryMB: 4096,
		Options: map[string]string{"server-id": "9999", "max_connections": "500"},
	})
	if strings.Contains(out, "server-id = 9999") {
		t.Errorf("server-id override leaked into render:\n%s", out)
	}
	if !strings.Contains(out, "max_connections = 500") {
		t.Errorf("benign user option dropped:\n%s", out)
	}
}

func TestRenderMySQLConf_SuffixTranslation(t *testing.T) {
	// "25%" of 4096 MB resolves to 1024MB → "1024M" in my.cnf.
	out := RenderMySQLConf(RenderMySQLConfOpts{
		Version: "8.0", Role: "primary", TotalMemoryMB: 4096,
		Options: map[string]string{"innodb_buffer_pool_size": "25%"},
	})
	if !strings.Contains(out, "innodb_buffer_pool_size = 1024M") {
		t.Errorf("expected MB→M translation; got:\n%s", out)
	}
}

func TestSubstituteMySQLPlaceholders(t *testing.T) {
	body := "server-id = " + MySQLServerIDPlaceholder + "\nreport_host = " + MySQLLocalHostPlaceholder + "\n"
	got := SubstituteMySQLPlaceholders(body, 4, "10.0.0.5")
	if !strings.Contains(got, "server-id = 5") { // NodeIndex+1
		t.Errorf("server-id substitution failed:\n%s", got)
	}
	if !strings.Contains(got, "report_host = 10.0.0.5") {
		t.Errorf("report_host substitution failed:\n%s", got)
	}
}
