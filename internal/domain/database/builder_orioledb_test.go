package database

import (
	"testing"

	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/domain"
)

func TestBuildTopologySpecOrioledb(t *testing.T) {
	db := &domain.Database{
		Kind: domain.Database_KIND_ORIOLEDB,
		Source: &domain.Database_Params{
			Params: &domain.DatabaseParams{
				Engine: &domain.DatabaseParams_Orioledb{
					Orioledb: &domain.OrioledbParams{},
				},
			},
		},
	}
	spec, err := BuildTopologySpec(db)
	if err != nil {
		t.Fatalf("BuildTopologySpec: %v", err)
	}
	if len(spec.GetComponents()) != 1 || spec.GetComponents()[0].GetEngine() != "orioledb" {
		t.Fatalf("expected one orioledb component, got %+v", spec.GetComponents())
	}
}
