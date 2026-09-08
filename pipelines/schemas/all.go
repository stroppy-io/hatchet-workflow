// Package schemas is the product's schemapb schema registry: every form,
// config and pipeline spec, keyed by public id "<ns>.<name>@<major>".
package schemas

import (
	"context"
	"sort"

	schemapb "github.com/gopherex/schemapb/go/schemapb"

	"github.com/stroppy-io/stroppy-cloud/pipelines/schemas/cfg"
	"github.com/stroppy-io/stroppy-cloud/pipelines/schemas/dbparams"
	"github.com/stroppy-io/stroppy-cloud/pipelines/schemas/ids"
	"github.com/stroppy-io/stroppy-cloud/pipelines/schemas/provider"
	"github.com/stroppy-io/stroppy-cloud/pipelines/schemas/spec"
	"github.com/stroppy-io/stroppy-cloud/pipelines/schemas/system"
	"github.com/stroppy-io/stroppy-cloud/pipelines/schemas/workload"
)

// All returns every schema, built fresh, sorted by public id.
func All() []*schemapb.Schema {
	var list []*schemapb.Schema
	list = append(list, dbparams.All()...)
	list = append(list, cfg.All()...)
	list = append(list, workload.All()...)
	list = append(list, provider.All()...)
	list = append(list, spec.All()...)
	list = append(list, system.All()...)
	sort.Slice(list, func(i, j int) bool { return ids.Public(list[i].GetId()) < ids.Public(list[j].GetId()) })
	return list
}

// Registry puts All() into a fresh in-memory schemapb registry.
func Registry(ctx context.Context) (schemapb.Registry, error) {
	reg := schemapb.NewInMemoryRegistry()
	for _, s := range All() {
		if err := reg.Put(ctx, s); err != nil {
			return nil, err
		}
	}
	return reg, nil
}

// ByID indexes All() by public id.
func ByID() map[string]*schemapb.Schema {
	out := map[string]*schemapb.Schema{}
	for _, s := range All() {
		out[ids.Public(s.GetId())] = s
	}
	return out
}
