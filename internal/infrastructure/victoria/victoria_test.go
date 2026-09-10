package victoria

import "testing"

func TestScopePromQL(t *testing.T) {
	cases := map[string]string{
		`up`: `up{r="x"}`,
		`rate(node_cpu_seconds_total{mode="idle"}[5m])`: `rate(node_cpu_seconds_total{r="x",mode="idle"}[5m])`,
		`sum by (machine) (up) / count(up{a="b"})`:      `sum by (machine) (up{r="x"}) / count(up{r="x",a="b"})`,
		`avg_over_time(tps[1h30m]) > 100`:               `avg_over_time(tps{r="x"}[1h30m]) > 100`,
		`label_replace(up, "m", "$1", "x", "(.*)")`:     `label_replace(up{r="x"}, "m", "$1", "x", "(.*)")`,
	}
	for in, want := range cases {
		if got := scopePromQL(in, "r", "x"); got != want {
			t.Errorf("%s\n got %s\nwant %s", in, got, want)
		}
	}
}
