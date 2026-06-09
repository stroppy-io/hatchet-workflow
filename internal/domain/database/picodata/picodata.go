package picodata

import (
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/database/dbspec"
	deploymentbuilder "github.com/stroppy-io/stroppy-cloud/internal/domain/deployment"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/domain"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/topology"
)

const (
	picodataEngine = "picodata"

	picodataRoleInstance = "instance"
	picodataRoleHaproxy  = "haproxy"

	// pgprotoPort is the PostgreSQL wire-protocol port Picodata exposes.
	pgprotoPort = 5432
	// iprotoPort is the Tarantool iproto / raft peer port.
	iprotoPort = 3301
	// httpPort is the Picodata HTTP (monitoring) port.
	httpPort = 8081

	defaultTierName = "default"
)

type Database struct{}

func (p *Database) ValidateInput(input *domain.PicodataParams) error {
	if input == nil {
		return errors.New("picodata params are required")
	}
	if err := input.Validate(); err != nil {
		return err
	}
	for _, tier := range input.GetTiers() {
		if tier.GetCount() == 0 {
			return fmt.Errorf("picodata tier %q must have count >= 1", tier.GetName())
		}
	}
	return nil
}

func (p *Database) BuildTopologySpec(input *domain.PicodataParams) (*topology.TopologySpec, error) {
	if err := p.ValidateInput(input); err != nil {
		return nil, err
	}
	return newPicodataSpecBuilder(input).build(), nil
}

type instanceRef struct {
	id   string
	tier string
}

type picodataSpecBuilder struct {
	input     *domain.PicodataParams
	spec      *topology.TopologySpec
	instances []instanceRef
}

func newPicodataSpecBuilder(input *domain.PicodataParams) *picodataSpecBuilder {
	return &picodataSpecBuilder{
		input: input,
		spec: &topology.TopologySpec{
			Nodes:       make([]*topology.Node, 0, picodataInstanceCount(input)+input.GetHaproxy()),
			Components:  make([]*topology.Component, 0, picodataInstanceCount(input)+input.GetHaproxy()),
			Connections: make([]*topology.Connection, 0),
			Labels: map[string]string{
				"kind":               "database",
				"engine":             picodataEngine,
				"instances":          strconv.FormatUint(uint64(picodataInstanceCount(input)), 10),
				"haproxy":            strconv.FormatUint(uint64(input.GetHaproxy()), 10),
				"replication_factor": strconv.FormatUint(uint64(input.GetReplicationFactor()), 10),
				"shards":             strconv.FormatUint(uint64(input.GetShards()), 10),
				"tiers":              strconv.Itoa(len(input.GetTiers())),
			},
			Tags: dbspec.Tags(picodataEngine),
		},
	}
}

func (b *picodataSpecBuilder) build() *topology.TopologySpec {
	for _, ref := range picodataInstanceRefs(b.input) {
		b.addInstance(ref)
	}

	// Every non-bootstrap instance joins the first instance over iproto.
	if len(b.instances) > 0 {
		bootstrap := b.instances[0].id
		for _, ref := range b.instances[1:] {
			b.spec.Connections = append(b.spec.Connections, dbspec.Connection(
				picodataEngine, ref.id, bootstrap,
				topology.Connection_KIND_COORDINATION, topology.Connection_PROTOCOL_CONTROL, topology.Connection_MODE_SYNC,
				"iproto", iprotoPort, false,
			))
		}
	}

	b.addHaproxyNodes()
	return b.spec
}

func (b *picodataSpecBuilder) addInstance(ref instanceRef) {
	ordinal := uint32(len(b.instances))
	b.spec.Components = append(b.spec.Components, dbspec.Component(
		picodataEngine, ref.id, topology.Component_KIND_DATABASE, picodataRoleInstance, ref.id,
	))
	node := dbspec.Node(picodataEngine, ref.id, picodataRoleInstance, ordinal, []string{ref.id})
	node.Labels["tier"] = ref.tier
	b.spec.Nodes = append(b.spec.Nodes, node)
	b.instances = append(b.instances, ref)
}

func (b *picodataSpecBuilder) addHaproxyNodes() {
	for i := uint32(1); i <= b.input.GetHaproxy(); i++ {
		haproxyID := fmt.Sprintf("haproxy-%d", i)
		b.spec.Components = append(b.spec.Components, dbspec.Component(
			picodataEngine, haproxyID, topology.Component_KIND_PROXY, picodataRoleHaproxy, haproxyID,
		))
		b.spec.Nodes = append(b.spec.Nodes, dbspec.Node(picodataEngine, haproxyID, picodataRoleHaproxy, i, []string{haproxyID}))

		for _, ref := range b.instances {
			b.spec.Connections = append(b.spec.Connections, dbspec.Connection(
				picodataEngine, haproxyID, ref.id,
				topology.Connection_KIND_PROXY, topology.Connection_PROTOCOL_TCP, topology.Connection_MODE_REQUEST,
				"pgproto", pgprotoPort, false,
			))
		}
	}
}

func picodataInstanceRefs(input *domain.PicodataParams) []instanceRef {
	tiers := input.GetTiers()
	if len(tiers) == 0 {
		count := picodataInstanceCount(input)
		refs := make([]instanceRef, 0, count)
		for i := uint32(1); i <= count; i++ {
			refs = append(refs, instanceRef{id: fmt.Sprintf("picodata-instance-%d", i), tier: defaultTierName})
		}
		return refs
	}

	var refs []instanceRef
	for _, tier := range tiers {
		for i := uint32(1); i <= tier.GetCount(); i++ {
			refs = append(refs, instanceRef{
				id:   fmt.Sprintf("picodata-%s-%d", tier.GetName(), i),
				tier: tier.GetName(),
			})
		}
	}
	return refs
}

func picodataInstanceCount(input *domain.PicodataParams) uint32 {
	if tiers := input.GetTiers(); len(tiers) > 0 {
		var total uint32
		for _, tier := range tiers {
			total += tier.GetCount()
		}
		return total
	}
	if input.GetInstances() == 0 {
		return 1
	}
	return input.GetInstances()
}

// picodataConfigContent renders a minimal picodata.yaml in the current config
// shape: cluster tiers plus per-instance sockets and data directory.
func picodataConfigContent(componentID string, input *domain.PicodataParams, wiring picodataWiring) string {
	advertise := wiring.advertise
	if advertise == "" {
		advertise = fmt.Sprintf("127.0.0.1:%d", iprotoPort)
	}
	peer := wiring.peer
	if peer == "" {
		peer = advertise
	}
	dataDir := deploymentbuilder.DataDir(componentID)

	var b strings.Builder
	b.WriteString("cluster:\n")
	b.WriteString("  name: stroppy-cluster\n")
	b.WriteString("  tier:\n")
	for _, tier := range picodataTierSpecs(input) {
		fmt.Fprintf(&b, "    %s:\n", tier.name)
		fmt.Fprintf(&b, "      replication_factor: %d\n", tier.replicationFactor)
		fmt.Fprintf(&b, "      can_vote: %t\n", tier.canVote)
	}
	b.WriteString("instance:\n")
	fmt.Fprintf(&b, "  instance_dir: %s\n", shellYAMLString(dataDir))
	fmt.Fprintf(&b, "  name: %s\n", shellYAMLString(picodataInstanceName(componentID)))
	b.WriteString("  tier: default\n")
	fmt.Fprintf(&b, "  peer:\n    - %s\n", shellYAMLString(peer))
	b.WriteString("  iproto:\n")
	b.WriteString("    enabled: true\n")
	fmt.Fprintf(&b, "    listen: %s\n", shellYAMLString(fmt.Sprintf("0.0.0.0:%d", iprotoPort)))
	fmt.Fprintf(&b, "    advertise: %s\n", shellYAMLString(advertise))
	b.WriteString("  http:\n")
	b.WriteString("    enabled: true\n")
	b.WriteString("    kubernetes_probes: true\n")
	fmt.Fprintf(&b, "    listen: %s\n", shellYAMLString(fmt.Sprintf("0.0.0.0:%d", httpPort)))
	b.WriteString("  pgproto:\n")
	b.WriteString("    enabled: true\n")
	fmt.Fprintf(&b, "    listen: %s\n", shellYAMLString(fmt.Sprintf("0.0.0.0:%d", pgprotoPort)))

	keys := make([]string, 0, len(input.GetInstanceOptions()))
	for key := range input.GetInstanceOptions() {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		fmt.Fprintf(&b, "  %s: %s\n", key, input.GetInstanceOptions()[key])
	}
	return b.String()
}

func picodataInstanceName(componentID string) string {
	name := strings.NewReplacer("-", "_", ".", "_", "/", "_").Replace(componentID)
	if name == "" {
		return "picodata_instance"
	}
	return name
}

func shellYAMLString(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "''") + "'"
}

type tierSpec struct {
	name              string
	replicationFactor uint32
	canVote           bool
}

func picodataTierSpecs(input *domain.PicodataParams) []tierSpec {
	if tiers := input.GetTiers(); len(tiers) > 0 {
		specs := make([]tierSpec, 0, len(tiers))
		for _, tier := range tiers {
			rf := tier.GetReplicationFactor()
			if rf == 0 {
				rf = 1
			}
			specs = append(specs, tierSpec{name: tier.GetName(), replicationFactor: rf, canVote: tier.GetCanVote()})
		}
		return specs
	}
	rf := input.GetReplicationFactor()
	if rf == 0 {
		rf = 1
	}
	return []tierSpec{{name: defaultTierName, replicationFactor: rf, canVote: true}}
}
