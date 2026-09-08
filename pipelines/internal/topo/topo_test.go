package topo

import (
	"strings"
	"testing"
)

type n struct {
	name string
	deps []string
}

func (x n) NodeName() string   { return x.name }
func (x n) NodeDeps() []string { return x.deps }

func names(ns []n) string {
	var out []string
	for _, x := range ns {
		out = append(out, x.name)
	}
	return strings.Join(out, ",")
}

func TestLayers(t *testing.T) {
	nodes := []n{
		{"haproxy", []string{"pg-1", "pg-2"}},
		{"pg-2", []string{"etcd"}},
		{"pg-1", []string{"etcd"}},
		{"etcd", nil},
		{"exporter", []string{"pg-1"}},
	}
	layers, err := Layers(nodes)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"etcd", "pg-2,pg-1", "haproxy,exporter"}
	if len(layers) != len(want) {
		t.Fatalf("layers = %d, want %d", len(layers), len(want))
	}
	for i, l := range layers {
		if got := names(l); got != want[i] {
			t.Errorf("layer %d = %s, want %s", i, got, want[i])
		}
	}
	order, _ := Order(nodes)
	if got := names(order); got != "etcd,pg-2,pg-1,haproxy,exporter" {
		t.Errorf("order = %s", got)
	}
}

func TestLayersErrors(t *testing.T) {
	if _, err := Layers([]n{{"a", []string{"b"}}, {"b", []string{"a"}}}); err == nil || !strings.Contains(err.Error(), "cycle") {
		t.Errorf("cycle not reported: %v", err)
	}
	if _, err := Layers([]n{{"a", []string{"zzz"}}}); err == nil || !strings.Contains(err.Error(), "unknown") {
		t.Errorf("unknown dep not reported: %v", err)
	}
	if _, err := Layers([]n{{"a", nil}, {"a", nil}}); err == nil || !strings.Contains(err.Error(), "duplicate") {
		t.Errorf("duplicate not reported: %v", err)
	}
	if layers, err := Layers([]n{}); err != nil || len(layers) != 0 {
		t.Errorf("empty input: %v %v", layers, err)
	}
}

func TestGroupBy(t *testing.T) {
	keys, groups := GroupBy([]string{"db-1", "runner-1", "db-2"}, func(s string) string { return strings.Split(s, "-")[0] })
	if strings.Join(keys, ",") != "db,runner" {
		t.Errorf("keys = %v", keys)
	}
	if len(groups["db"]) != 2 || groups["db"][1] != "db-2" {
		t.Errorf("groups = %v", groups)
	}
}

func TestSlug(t *testing.T) {
	for in, want := range map[string]string{
		"Postgres 17 / HA":      "postgres-17-ha",
		"--x--":                 "x",
		"":                      "x",
		"averyveryverylongname": "averyvery",
	} {
		max := 0
		if in == "averyveryverylongname" {
			max = 9
		}
		if got := Slug(in, max); got != want {
			t.Errorf("Slug(%q) = %q, want %q", in, got, want)
		}
	}
}
