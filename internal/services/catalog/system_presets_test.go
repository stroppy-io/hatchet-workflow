package catalog

import (
	"testing"

	dag "github.com/stroppy-io/stroppy-cloud/internal/domain/dag"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/deployment"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/domain"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/models"
)

// TestSystemDatabasePresets_NonEmpty guards against an empty seed catalog.
func TestSystemDatabasePresets_NonEmpty(t *testing.T) {
	if len(SystemDatabasePresets()) == 0 {
		t.Fatal("SystemDatabasePresets() returned no presets")
	}
}

// TestSystemDatabasePresets_Valid asserts every seeded preset passes proto
// validation (wrapped as a models.Preset KIND_DATABASE) and that the dag layer
// can compile a preview for it without panicking — i.e. these are real,
// runtime-buildable topologies.
func TestSystemDatabasePresets_Valid(t *testing.T) {
	for _, sp := range SystemDatabasePresets() {
		sp := sp
		t.Run(sp.Name, func(t *testing.T) {
			if sp.DB == nil {
				t.Fatalf("preset %q has nil DB", sp.Name)
			}

			// proto validation: the platform-seeded DatabasePreset must validate
			// exactly as a tenant-stored Preset row would.
			p := &models.Preset{
				Kind:   models.Preset_KIND_DATABASE,
				Preset: &models.Preset_DatabasePreset{DatabasePreset: sp.DB},
			}
			if err := p.ValidateAll(); err != nil {
				t.Fatalf("preset %q failed validation: %v", sp.Name, err)
			}

			// dag preview: wrap the DatabasePreset's Database+Topology into a
			// TestPreset (DOCKER deployment) and compile the static execution dag.
			// CompileTestPresetPreview must not panic and must yield nodes.
			tp := &domain.TestPreset{
				Database:   sp.DB.GetDatabase(),
				Topology:   sp.DB.GetTopology(),
				Deployment: &deployment.DeploymentIntent{Provider: deployment.Provider_PROVIDER_DOCKER},
			}

			var compiled = func() (panicked bool) {
				defer func() {
					if r := recover(); r != nil {
						panicked = true
						t.Errorf("preset %q: CompileTestPresetPreview panicked: %v", sp.Name, r)
					}
				}()
				d := dag.CompileTestPresetPreview(tp)
				if d == nil || len(d.GetNodes()) == 0 {
					t.Errorf("preset %q: compiled dag has no nodes", sp.Name)
				}
				return false
			}()
			_ = compiled

			// BuildInstallDag is the install sub-dag the runtime executes; it must
			// build without error for every seeded preset.
			if _, err := dag.BuildInstallDag(tp, "test-dag"); err != nil {
				t.Errorf("preset %q: BuildInstallDag error: %v", sp.Name, err)
			}
		})
	}
}
