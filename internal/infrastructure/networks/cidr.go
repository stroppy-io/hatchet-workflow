package networks

import (
	"encoding/binary"
	"fmt"
	"hash/fnv"
	"net/netip"
	"sort"
)

const runCIDRBits = 20

func SelectRunCIDR(runID, baseCIDR string, usedCIDRs []string) (string, error) {
	if baseCIDR == "" {
		baseCIDR = defaultYandexCIDRPool
	}
	base, err := netip.ParsePrefix(baseCIDR)
	if err != nil {
		return "", fmt.Errorf("parse network cidr %q: %w", baseCIDR, err)
	}
	base = base.Masked()
	if !base.Addr().Is4() {
		return "", fmt.Errorf("network cidr %q must be IPv4", baseCIDR)
	}
	if base.Bits() > runCIDRBits {
		return "", fmt.Errorf("network cidr %q must not be narrower than /%d for run allocations", baseCIDR, runCIDRBits)
	}

	used, err := parseCIDRs(usedCIDRs)
	if err != nil {
		return "", err
	}
	candidates, err := childPrefixes(base, runCIDRBits)
	if err != nil {
		return "", err
	}
	if len(candidates) == 0 {
		return "", fmt.Errorf("network cidr %q has no allocatable subnets", baseCIDR)
	}

	offset := 0
	if runID != "" {
		offset = int(hashString(runID) % uint64(len(candidates)))
	}
	for i := 0; i < len(candidates); i++ {
		candidate := candidates[(offset+i)%len(candidates)]
		if !overlapsAny(candidate, used) {
			return candidate.String(), nil
		}
	}
	return "", fmt.Errorf("no free /%d CIDR left in %s; choose a wider non-overlapping subnet_cidr such as 10.0.0.0/8, 172.16.0.0/12, or 192.168.0.0/16, or remove stale provider subnets/reservations", runCIDRBits, base.String())
}

func ZoneCIDRs(baseCIDR string, zones []string) (map[string]string, error) {
	if len(zones) == 0 {
		return nil, nil
	}
	base, err := netip.ParsePrefix(baseCIDR)
	if err != nil {
		return nil, fmt.Errorf("parse network cidr %q: %w", baseCIDR, err)
	}
	base = base.Masked()
	if !base.Addr().Is4() {
		return nil, fmt.Errorf("network cidr %q must be IPv4", baseCIDR)
	}
	childBits := base.Bits()
	for capacity := 1; capacity < len(zones); capacity <<= 1 {
		childBits++
	}
	if childBits > 28 {
		return nil, fmt.Errorf("network cidr %s cannot be split into %d YC subnets with /28 minimum", baseCIDR, len(zones))
	}
	children, err := childPrefixes(base, childBits)
	if err != nil {
		return nil, err
	}
	sort.Strings(zones)
	out := make(map[string]string, len(zones))
	for i, zone := range zones {
		out[zone] = children[i].String()
	}
	return out, nil
}

func parseCIDRs(values []string) ([]netip.Prefix, error) {
	out := make([]netip.Prefix, 0, len(values))
	for _, value := range values {
		if value == "" {
			continue
		}
		prefix, err := netip.ParsePrefix(value)
		if err != nil {
			return nil, fmt.Errorf("parse used network cidr %q: %w", value, err)
		}
		if !prefix.Addr().Is4() {
			continue
		}
		out = append(out, prefix.Masked())
	}
	return out, nil
}

func childPrefixes(base netip.Prefix, bits int) ([]netip.Prefix, error) {
	if bits < base.Bits() || bits > 32 {
		return nil, fmt.Errorf("invalid child prefix /%d for %s", bits, base.String())
	}
	start := ipv4ToUint32(base.Addr())
	count := uint64(1) << uint(bits-base.Bits())
	size := uint64(1) << uint(32-bits)
	out := make([]netip.Prefix, 0, count)
	for i := uint64(0); i < count; i++ {
		addr := uint32ToIPv4(uint32(uint64(start) + i*size))
		out = append(out, netip.PrefixFrom(addr, bits).Masked())
	}
	return out, nil
}

func overlapsAny(candidate netip.Prefix, used []netip.Prefix) bool {
	for _, prefix := range used {
		if overlaps(candidate, prefix) {
			return true
		}
	}
	return false
}

func overlaps(left, right netip.Prefix) bool {
	left = left.Masked()
	right = right.Masked()
	return left.Contains(right.Addr()) || right.Contains(left.Addr())
}

func ipv4ToUint32(addr netip.Addr) uint32 {
	raw := addr.As4()
	return binary.BigEndian.Uint32(raw[:])
}

func uint32ToIPv4(value uint32) netip.Addr {
	var raw [4]byte
	binary.BigEndian.PutUint32(raw[:], value)
	return netip.AddrFrom4(raw)
}

func hashString(value string) uint64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte(value))
	return h.Sum64()
}
