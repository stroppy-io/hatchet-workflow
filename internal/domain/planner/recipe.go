package planner

import (
	"github.com/stroppy-io/stroppy-cloud/internal/domain/render"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/domain"
	renderpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/runtime/render"
)

// stroppyDBHostToken is the late-binding hole in the run_stroppy command,
// resolved to the database component's private ip at the plan->execute seam.
const stroppyDBHostToken = "__STROPPY_DB_HOST__"

// topologyView extracts what the planner needs from a topology: the
// component->machine map (for binding resolution) and the primary DATABASE
// component (+ its machine memory for config sizing).
type topologyView struct {
	componentToMachine map[string]string
	dbComponentID      string
	dbMemoryMB         int
}

func viewTopology(topo *domain.Topology) topologyView {
	v := topologyView{componentToMachine: map[string]string{}}
	for _, m := range topo.GetMachines() {
		for _, c := range m.GetComponents() {
			v.componentToMachine[c.GetId()] = m.GetId()
			if c.GetKind() == domain.Topology_Component_KIND_DATABASE && v.dbComponentID == "" {
				v.dbComponentID = c.GetId()
				v.dbMemoryMB = int(m.GetMemoryGb()) * 1024
			}
		}
	}
	return v
}

// databaseConfig returns the rendered DB config: the intent's pre-rendered
// Database.config when present (wizard, preview==execution), else rendered on the
// fly. Returns an empty Config (no config writes) for engines the renderer does
// not support yet.
func databaseConfig(db *domain.Database, memoryMB int) *renderpb.Config {
	if cfg := db.GetConfig(); len(cfg.GetItems()) > 0 {
		return cfg
	}
	cfg, err := render.RenderDatabase(db, memoryMB)
	if err != nil {
		return &renderpb.Config{} // TODO(planner): non-postgres engines
	}
	return cfg
}
