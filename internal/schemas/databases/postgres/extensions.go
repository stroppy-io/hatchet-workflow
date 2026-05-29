package postgres

import (
	"github.com/stroppy-io/schemapb/schemapb"

	"github.com/stroppy-io/stroppy-cloud/internal/schemas/utils"
)

// extensionsSection is the multi-select of PostgreSQL extensions to install
// (pure DB). shared_preload / restart wiring is handled downstream.
func extensionsSection() schemapb.FieldDef {
	return schemapb.Object(FieldExtensions,
		schemapb.List(FieldEnabled,
			schemapb.Object(FieldExt,
				utils.StrEnum(FieldName, ExtensionNameValues...).Required().Title("Extension"),
			),
		).Title("Enabled extensions"),
	).Title("Extensions")
}
