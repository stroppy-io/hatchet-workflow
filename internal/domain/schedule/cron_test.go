package schedule

import (
	"testing"
	"time"
)

func TestCronNext(t *testing.T) {
	loc := time.UTC
	base := time.Date(2026, 9, 10, 15, 30, 20, 0, loc)
	cases := []struct{ expr, want string }{
		{"*/15 * * * *", "2026-09-10T15:45:00Z"},
		{"0 3 * * *", "2026-09-11T03:00:00Z"},
		{"@hourly", "2026-09-10T16:00:00Z"},
		{"30 9 * * mon-fri", "2026-09-11T09:30:00Z"},
		{"0 0 1 1 *", "2027-01-01T00:00:00Z"},
		{"0 12 15 * *", "2026-09-15T12:00:00Z"},
		{"0 0 * * 0", "2026-09-13T00:00:00Z"},
		{"5,35 * * * *", "2026-09-10T15:35:00Z"},
	}
	for _, c := range cases {
		cr, err := ParseCron(c.expr)
		if err != nil {
			t.Fatalf("%s: %v", c.expr, err)
		}
		got := cr.Next(base).Format(time.RFC3339)
		if got != c.want {
			t.Errorf("%s: next = %s, want %s", c.expr, got, c.want)
		}
	}
	for _, bad := range []string{"", "* * *", "60 * * * *", "* 24 * * *", "a b c d e"} {
		if _, err := ParseCron(bad); err == nil {
			t.Errorf("%q parsed", bad)
		}
	}
}
