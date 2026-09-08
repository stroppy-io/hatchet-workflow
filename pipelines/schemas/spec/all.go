package spec

import schemapb "github.com/gopherex/schemapb/go/schemapb"

// All returns every spec.* schema, built fresh.
func All() []*schemapb.Schema {
	return []*schemapb.Schema{
		ProviderVerify(),
		Quotas(),
		ResultProviderVerify(),
		ResultQuotas(),
		ResultRun(),
		Run(),
		Suite(),
	}
}
