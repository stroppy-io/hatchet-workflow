package providers

import (
	"encoding/json"
	"testing"

	"google.golang.org/protobuf/types/known/structpb"
)

func values(t *testing.T, jsonStr string) *structpb.Struct {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal([]byte(jsonStr), &m); err != nil {
		t.Fatalf("bad json: %v", err)
	}
	st, err := structpb.NewStruct(m)
	if err != nil {
		t.Fatalf("struct: %v", err)
	}
	return st
}

func TestYandexBuildsAndValidates(t *testing.T) {
	s := YandexProviderSchema()
	if s.GetId().GetName() != nameYandex {
		t.Fatalf("name=%q", s.GetId().GetName())
	}
	errs := s.ValidateStruct(values(t, `{
		"cloud_id":"c1","folder_id":"f1",
		"zones":["ru-central1-a","ru-central1-b"],
		"platform_id":"standard-v3",
		"disk_types":["network-ssd","network-ssd-io-m3"],
		"limits":{"max_nodes":10,"max_cores_per_node":96}
	}`))
	for _, e := range errs {
		if e.GetSeverity().String() == "ERROR" {
			t.Errorf("unexpected error: %s: %s", e.GetField(), e.GetMessage())
		}
	}
}

func TestDockerBuildsAndValidates(t *testing.T) {
	s := DockerProviderSchema()
	if s.GetId().GetName() != nameDocker {
		t.Fatalf("name=%q", s.GetId().GetName())
	}
	errs := s.ValidateStruct(values(t, `{"image":"stroppy-agent:latest","privileged":true}`))
	for _, e := range errs {
		if e.GetSeverity().String() == "ERROR" {
			t.Errorf("unexpected error: %s: %s", e.GetField(), e.GetMessage())
		}
	}
}

// TestYandexRejectsBadDiskType: a disk class outside the catalog must error.
func TestYandexRejectsBadDiskType(t *testing.T) {
	s := YandexProviderSchema()
	errs := s.ValidateStruct(values(t, `{
		"cloud_id":"c1","folder_id":"f1","zones":["ru-central1-a"],
		"disk_types":["nvme-from-mars"]
	}`))
	found := false
	for _, e := range errs {
		if e.GetSeverity().String() == "ERROR" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected an error for invalid disk type")
	}
}
