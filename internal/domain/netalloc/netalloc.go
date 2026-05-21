// Package netalloc assigns per-machine private IPs from a subnet CIDR, avoiding
// addresses already in use (D20). It is pure: the caller supplies the used-IP set
// — it does NOT trust the subnet is empty. The used set must come from the
// provider (live VPC and/or the persisted NetworkAllocation mirror), because the
// docs require provider-side checks before allocating (H33).
package netalloc

import (
	"fmt"
	"net"
)

// reservedHostCount skips the network address + the conventional gateway/reserved
// low addresses (cloud providers reserve the first few host addresses).
const reservedHostCount = 4

// AssignIPs returns one free private IPv4 per machine id, taken from cidr after
// the reserved low addresses and skipping every address in usedIPs. Order follows
// the input id order. Fails if the subnet cannot satisfy all ids.
func AssignIPs(cidr string, ids []string, usedIPs []string) (map[string]string, error) {
	if len(ids) == 0 {
		return map[string]string{}, nil
	}
	_, ipNet, err := net.ParseCIDR(cidr)
	if err != nil {
		return nil, fmt.Errorf("netalloc: parse cidr %q: %w", cidr, err)
	}
	base := ipNet.IP.Mask(ipNet.Mask).To4()
	if base == nil {
		return nil, fmt.Errorf("netalloc: cidr %q is not IPv4", cidr)
	}

	used := make(map[string]struct{}, len(usedIPs))
	for _, s := range usedIPs {
		if ip := net.ParseIP(s); ip != nil {
			used[ip.String()] = struct{}{}
		}
	}

	cur := cloneIP(base)
	for range reservedHostCount {
		cur = inc(cur)
	}
	out := make(map[string]string, len(ids))
	for _, id := range ids {
		for {
			if !ipNet.Contains(cur) {
				return nil, fmt.Errorf("netalloc: subnet %q exhausted assigning %d ips", cidr, len(ids))
			}
			if _, taken := used[cur.String()]; !taken {
				break
			}
			cur = inc(cur)
		}
		out[id] = cur.String()
		used[cur.String()] = struct{}{}
		cur = inc(cur)
	}
	return out, nil
}

func cloneIP(ip net.IP) net.IP {
	c := make(net.IP, len(ip))
	copy(c, ip)
	return c
}

func inc(ip net.IP) net.IP {
	c := cloneIP(ip)
	for i := len(c) - 1; i >= 0; i-- {
		c[i]++
		if c[i] != 0 {
			break
		}
	}
	return c
}
