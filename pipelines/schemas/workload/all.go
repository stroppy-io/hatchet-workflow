package workload

import schemapb "github.com/gopherex/schemapb/go/schemapb"

// All returns every workload.* schema, built fresh.
func All() []*schemapb.Schema {
	return []*schemapb.Schema{
		Segment(),
		Stroppy(),
	}
}
