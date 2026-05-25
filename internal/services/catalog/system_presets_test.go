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

// TestSystemWorkloadPresets_Valid asserts every seeded workload preset is non-empty
// and passes proto validation wrapped as a models.Preset KIND_WORKLOAD (the same way
// a tenant-stored row validates). Unique names guard the seed's idempotency key.
func TestSystemWorkloadPresets_Valid(t *testing.T) {
	wps := SystemWorkloadPresets()
	if len(wps) == 0 {
		t.Fatal("SystemWorkloadPresets() returned no presets")
	}
	seen := map[string]bool{}
	for _, wp := range wps {
		wp := wp
		t.Run(wp.Name, func(t *testing.T) {
			if seen[wp.Name] {
				t.Fatalf("duplicate workload preset name %q", wp.Name)
			}
			seen[wp.Name] = true
			if wp.WL == nil {
				t.Fatalf("preset %q has nil WL", wp.Name)
			}
			p := &models.Preset{
				Kind:   models.Preset_KIND_WORKLOAD,
				Preset: &models.Preset_WorkloadPreset{WorkloadPreset: wp.WL},
			}
			if err := p.ValidateAll(); err != nil {
				t.Fatalf("preset %q failed validation: %v", wp.Name, err)
			}
		})
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
