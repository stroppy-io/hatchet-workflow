package preset

import (
	workloadbuilder "github.com/stroppy-io/stroppy-cloud/internal/domain/workload"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/models"
)

func fillDatabasePresetSummary(p *models.DatabasePresetRecord) {
	if p == nil {
		return
	}
	p.Summary = &models.DatabasePresetRecord_Summary{
		DbKind:   p.GetDatabase().GetKind(),
		Version:  p.GetDatabase().GetParams().GetVersion(),
		External: p.GetDatabase().GetExternal() != nil,
	}
}

func fillWorkloadPresetSummary(p *models.WorkloadPresetRecord) {
	if p == nil {
		return
	}
	p.Summary = &models.WorkloadPresetRecord_Summary{
		Protocol:       p.GetWorkload().GetProtocol(),
		StroppyVersion: p.GetWorkload().GetStroppyVersion(),
		Script:         workloadbuilder.PrimarySegment(p.GetWorkload()).GetScript(),
	}
}

func fillTestPresetSummary(p *models.TestPresetRecord) {
	if p == nil {
		return
	}
	p.Summary = &models.TestPresetRecord_Summary{
		DbKind:         p.GetTest().GetDatabase().GetKind(),
		Protocol:       p.GetTest().GetWorkload().GetProtocol(),
		StroppyVersion: p.GetTest().GetWorkload().GetStroppyVersion(),
	}
}
