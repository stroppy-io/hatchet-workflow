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
	return []string{region + "-a", region + "-b", region + "-c"}
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
