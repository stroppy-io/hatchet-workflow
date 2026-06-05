package deployment

import (
	"strings"
	"testing"
)

func TestEnableStartServiceCommandPrintsDiagnosticsOnFailure(t *testing.T) {
	cmd := EnableStartServiceCommand("postgres-master")

	for _, want := range []string{
		"systemctl enable --now 'stroppy-postgres-master'",
		`echo "--- systemctl status 'stroppy-postgres-master' ---"`,
		"systemctl status --no-pager -l 'stroppy-postgres-master'",
		`echo "--- journalctl 'stroppy-postgres-master' ---"`,
		"journalctl --no-pager --output=short-iso-precise -u 'stroppy-postgres-master' -n 200",
		"exit 1",
	} {
		if !strings.Contains(cmd, want) {
			t.Fatalf("command missing %q:\n%s", want, cmd)
		}
	}
}
