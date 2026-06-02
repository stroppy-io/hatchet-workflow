package database

import "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/topology"

type Database[T any] interface {
	ValidateInput(input T) error
	BuildTopologySpec(input T) (*topology.TopologySpec, error)
}
