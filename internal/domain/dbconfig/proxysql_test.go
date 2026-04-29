package dbconfig

import (
	"fmt"
	"strings"
	"testing"
)

func TestRenderProxySQLConf_PlaceholdersForEachBackend(t *testing.T) {
	out := RenderProxySQLConf(RenderProxySQLConfOpts{BackendCount: 3})
	for i := 0; i < 3; i++ {
		want := proxysqlBackendPlaceholder(i)
		if !strings.Contains(out, want) {
			t.Errorf("backend %d placeholder missing:\n%s", i, out)
		}
	}
}

func TestRenderProxySQLConf_FirstBackendIsWriter(t *testing.T) {
	out := RenderProxySQLConf(RenderProxySQLConfOpts{BackendCount: 2, WriterHostgroup: 10, ReaderHostgroup: 20})
	// First backend lands in hostgroup 10, second in 20.
	if !strings.Contains(out, fmt.Sprintf("address=\"%s\", port=3306, hostgroup=10", proxysqlBackendPlaceholder(0))) {
		t.Errorf("backend 0 not in writer hostgroup:\n%s", out)
	}
	if !strings.Contains(out, fmt.Sprintf("address=\"%s\", port=3306, hostgroup=20", proxysqlBackendPlaceholder(1))) {
		t.Errorf("backend 1 not in reader hostgroup:\n%s", out)
	}
}

func TestRenderProxySQLConf_DefaultsApplied(t *testing.T) {
	out := RenderProxySQLConf(RenderProxySQLConfOpts{BackendCount: 1})
	if !strings.Contains(out, "0.0.0.0:6033") {
		t.Errorf("default ListenPort 6033 missing:\n%s", out)
	}
	if !strings.Contains(out, "0.0.0.0:6032") {
		t.Errorf("default AdminPort 6032 missing:\n%s", out)
	}
}

func TestSubstituteProxySQLBackends(t *testing.T) {
	body := RenderProxySQLConf(RenderProxySQLConfOpts{BackendCount: 2})
	got := SubstituteProxySQLBackends(body, []string{"db-primary.local:3306", "db-replica.local:3306"})
	if !strings.Contains(got, "address=\"db-primary.local\"") {
		t.Errorf("primary host not substituted:\n%s", got)
	}
	if !strings.Contains(got, "address=\"db-replica.local\"") {
		t.Errorf("replica host not substituted:\n%s", got)
	}
	if strings.Contains(got, proxysqlBackendPlaceholder(0)) || strings.Contains(got, proxysqlBackendPlaceholder(1)) {
		t.Errorf("placeholders still present after substitution:\n%s", got)
	}
}
