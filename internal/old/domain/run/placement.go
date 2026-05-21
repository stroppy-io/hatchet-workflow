package run

import (
	"strings"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/types"
)

var logicalYDBZones = []string{"zone-a", "zone-b", "zone-c"}

func placementZones(p *types.PlacementSpec) []string {
	if p != nil && len(p.Zones) > 0 {
		zones := make([]string, 0, len(p.Zones))
		for _, z := range p.Zones {
			if z != "" {
				zones = append(zones, z)
			}
		}
		if len(zones) > 0 {
			return zones
		}
	}
	return logicalYDBZones
}

// yandexDefaultRoundRobinSuffixes is the set of YC zone suffixes used when
// PlacementSpec asks for round-robin without an explicit Zones list.
//
// Only a/b/d are usable for stroppy clusters today:
//   - `c` was decommissioned (subnet → "Illegal argument zone_id", disk →
//     "Zone is down")
//   - `e` is a separate region not exposed for general compute
//   - `m` is the Yandex BareMetal zone — different SKU, no general VMs
//
// Yandex docs: https://yandex.cloud/en/docs/overview/concepts/geo-scope
var yandexDefaultRoundRobinSuffixes = []string{"-a", "-b", "-d"}

func yandexPlacementZones(p *types.PlacementSpec, defaultZone string) []string {
	if p != nil && len(p.Zones) > 0 {
		return placementZones(p)
	}
	if p == nil || p.Strategy != "round-robin" {
		if defaultZone != "" {
			return []string{defaultZone}
		}
		return placementZones(p)
	}
	if defaultZone == "" {
		return placementZones(p)
	}
	idx := strings.LastIndex(defaultZone, "-")
	if idx <= 0 || idx == len(defaultZone)-1 {
		return []string{defaultZone}
	}
	region := defaultZone[:idx]
	out := make([]string, 0, len(yandexDefaultRoundRobinSuffixes))
	for _, suffix := range yandexDefaultRoundRobinSuffixes {
		out = append(out, region+suffix)
	}
	return out
}

func zoneForInstance(p *types.PlacementSpec, zones []string, idx int) string {
	if len(zones) == 0 {
		return ""
	}
	if p != nil && p.Strategy == "single" {
		return zones[0]
	}
	return zones[idx%len(zones)]
}
