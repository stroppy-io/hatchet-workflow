package ips

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"math/big"
	"net"
)

const padding = 3

// FirstFreeIP returns the first free IP in cidr that is not in usedIPs.
func FirstFreeIP(ipNet *net.IPNet, usedIPs []string) (net.IP, error) {

	// Build a set of used IPs (string form).
	used := make(map[string]struct{}, len(usedIPs))
	for _, s := range usedIPs {
		ip := net.ParseIP(s)
		if ip == nil {
			continue
		}
		// Normalize to the same family/format as ipNet.IP
		ip = normalizeIPFamily(ip, ipNet.IP)
		if ipNet.Contains(ip) {
			used[ip.String()] = struct{}{}
		}
	}

	// Start from network address.
	start := ipNet.IP.Mask(ipNet.Mask)

	// Compute broadcast (last IP in range).
	broadcast := lastIP(ipNet)

	// Apply padding: skip network address + padding IPs (e.g., 10.1.0.0 - 10.1.0.3)
	paddedStart := cloneIP(start)
	for i := 0; i <= padding; i++ {
		paddedStart = incIP(paddedStart)
	}

	// Check if paddedStart has exceeded the broadcast address
	// This can happen in very small subnets where padding consumes all available IPs
	if !ipNet.Contains(paddedStart) || paddedStart.Equal(broadcast) || isGreaterOrEqual(paddedStart, broadcast) {
		return nil, fmt.Errorf("no free IPs in %s (padding exceeds available range)", ipNet.String())
	}

	// Iterate from paddedStart to last host-1.
	for ip := cloneIP(paddedStart); !ip.Equal(broadcast); ip = incIP(ip) {
		if _, taken := used[ip.String()]; !taken {
			return ip, nil
		}
	}

	return nil, fmt.Errorf("no free IPs in %s", ipNet.String())
}

// incIP increments an IP (IPv4 or IPv6) in-place and also returns it.
func incIP(ip net.IP) net.IP {
	for i := len(ip) - 1; i >= 0; i-- {
		ip[i]++
		if ip[i] != 0 {
			break
		}
	}
	return ip
}

// lastIP calculates the last IP in the subnet.
func lastIP(ipNet *net.IPNet) net.IP {
	ip := cloneIP(ipNet.IP)
	mask := ipNet.Mask

	for i := 0; i < len(ip); i++ {
		ip[i] |= ^mask[i]
	}
	return ip
}

func cloneIP(ip net.IP) net.IP {
	cp := make(net.IP, len(ip))
	copy(cp, ip)
	return cp
}

// normalizeIPFamily ensures ip has same length/family as base (handles v4-in-v6).
func normalizeIPFamily(ip, base net.IP) net.IP {
	if ip.To4() != nil && base.To4() != nil {
		return ip.To4()
	}
	return ip
}

// isGreaterOrEqual returns true if ip1 >= ip2 (bytewise comparison).
func isGreaterOrEqual(ip1, ip2 net.IP) bool {
	// Ensure both IPs have the same length for comparison
	if len(ip1) != len(ip2) {
		// Try to normalize to the same format
		if ip1.To4() != nil && ip2.To4() != nil {
			ip1 = ip1.To4()
			ip2 = ip2.To4()
		}
	}

	for i := 0; i < len(ip1) && i < len(ip2); i++ {
		if ip1[i] > ip2[i] {
			return true
		}
		if ip1[i] < ip2[i] {
			return false
		}
	}
	return true // Equal
}

// RandomIP generates a random IP address within the given CIDR range.
func RandomIP(ipNet *net.IPNet) (net.IP, error) {
	// Calculate the number of available IPs efficiently
	ones, bits := ipNet.Mask.Size()
	if bits == 0 {
		return nil, fmt.Errorf("invalid network mask")
	}

	// Calculate host bits
	hostBits := bits - ones

	// Calculate total number of IPs in the range: 2^hostBits
	totalIPs := new(big.Int).Lsh(big.NewInt(1), uint(hostBits))

	// Exclude network and broadcast addresses (2 IPs)
	// For /31 and /32 (IPv4) or /127 and /128 (IPv6), handle specially
	availableIPs := new(big.Int).Sub(totalIPs, big.NewInt(2))

	if availableIPs.Cmp(big.NewInt(0)) <= 0 {
		return nil, fmt.Errorf("no available IPs in %s", ipNet.String())
	}

	// Generate random offset
	randomOffset, err := rand.Int(rand.Reader, availableIPs)
	if err != nil {
		return nil, fmt.Errorf("failed to generate random number: %w", err)
	}

	// Start from network address
	start := ipNet.IP.Mask(ipNet.Mask)

	// Skip first IP (network address) and add random offset
	offset := new(big.Int).Add(randomOffset, big.NewInt(1))

	// Apply offset to start IP
	ip := addBigIntToIP(cloneIP(start), offset)

	return ip, nil
}

// addBigIntToIP adds a big.Int offset to an IP address
func addBigIntToIP(ip net.IP, offset *big.Int) net.IP {
	// Convert IP to big.Int
	ipInt := new(big.Int).SetBytes(ip)

	// Add offset
	ipInt.Add(ipInt, offset)

	// Convert back to IP
	ipBytes := ipInt.Bytes()

	// Ensure correct length (pad with zeros if needed)
	result := make(net.IP, len(ip))
	copy(result[len(result)-len(ipBytes):], ipBytes)

	return result
}

// NextSubnetByCount returns the next subnet in the given parent CIDR with the specified prefix,
// after the given number of subnets have been created.
// Работает только с IPv4.
func NextSubnetByCount(parentCIDR string, newPrefix int, createdCount int) (*net.IPNet, error) {
	parentIP, parentNet, err := net.ParseCIDR(parentCIDR)
	if err != nil {
		return nil, fmt.Errorf("invalid parent CIDR: %w", err)
	}

	parentIP = parentIP.To4()
	if parentIP == nil {
		return nil, fmt.Errorf("only IPv4 is supported")
	}

	parentOnes, parentBits := parentNet.Mask.Size()
	if parentBits != 32 {
		return nil, fmt.Errorf("only IPv4 /32-based masks supported")
	}
	if newPrefix < parentOnes || newPrefix > 32 {
		return nil, fmt.Errorf("newPrefix must be between %d and 32", parentOnes)
	}
	if createdCount < 0 {
		return nil, fmt.Errorf("createdCount must be >= 0")
	}

	parentStart := ipToUint32(parentIP)
	parentSize := uint32(1) << uint32(32-parentOnes)
	parentEnd := parentStart + parentSize - 1

	subnetSize := uint32(1) << uint32(32-newPrefix)

	// Сколько подсетей данного префикса влезет в родительскую сеть
	totalSubnets := parentSize / subnetSize

	if uint32(createdCount) >= totalSubnets {
		return nil, fmt.Errorf("no more free subnets: created=%d, total=%d",
			createdCount, totalSubnets)
	}

	// Индекс следующей подсети = createdCount (0-based)
	start := parentStart + uint32(createdCount)*subnetSize
	end := start + subnetSize - 1
	if end > parentEnd {
		return nil, fmt.Errorf("next subnet would overflow parent range")
	}

	ip := uint32ToIP(start)
	mask := net.CIDRMask(newPrefix, 32)

	return &net.IPNet{
		IP:   ip,
		Mask: mask,
	}, nil
}

// NextSubnetWithIPs finds a free subnet of size newPrefix within parentCIDR, avoiding existingSubnets.
// It returns the new subnet and a list of ipCount IPs from that subnet.
func NextSubnetWithIPs(parentCIDR string, newPrefix int, existingSubnets []string, ipCount int) (*net.IPNet, []net.IP, error) {
	_, parentNet, err := net.ParseCIDR(parentCIDR)
	if err != nil {
		return nil, nil, fmt.Errorf("invalid parent CIDR: %w", err)
	}

	// Ensure IPv4
	if parentNet.IP.To4() == nil {
		return nil, nil, fmt.Errorf("only IPv4 is supported")
	}

	parentOnes, parentBits := parentNet.Mask.Size()
	if parentBits != 32 {
		return nil, nil, fmt.Errorf("only IPv4 /32-based masks supported")
	}
	if newPrefix < parentOnes || newPrefix > 32 {
		return nil, nil, fmt.Errorf("newPrefix must be between %d and 32", parentOnes)
	}

	// Parse existing subnets into ranges
	var usedRanges [][2]uint32
	for _, s := range existingSubnets {
		_, n, err := net.ParseCIDR(s)
		if err != nil {
			continue
		}
		if n.IP.To4() == nil {
			continue
		}
		start := ipToUint32(n.IP)
		ones, _ := n.Mask.Size()
		size := uint32(1) << uint32(32-ones)
		end := start + size - 1
		usedRanges = append(usedRanges, [2]uint32{start, end})
	}

	parentStart := ipToUint32(parentNet.IP)
	parentSize := uint32(1) << uint32(32-parentOnes)
	subnetSize := uint32(1) << uint32(32-newPrefix)
	totalSubnets := parentSize / subnetSize

	var foundSubnet *net.IPNet

	for i := uint32(0); i < totalSubnets; i++ {
		candStart := parentStart + i*subnetSize
		candEnd := candStart + subnetSize - 1

		collision := false
		for _, r := range usedRanges {
			// Check overlap: start1 <= end2 && start2 <= end1
			if candStart <= r[1] && r[0] <= candEnd {
				collision = true
				break
			}
		}

		if !collision {
			foundSubnet = &net.IPNet{
				IP:   uint32ToIP(candStart),
				Mask: net.CIDRMask(newPrefix, 32),
			}
			break
		}
	}

	if foundSubnet == nil {
		return nil, nil, fmt.Errorf("no available subnet of size /%d in %s", newPrefix, parentCIDR)
	}

	// Generate IPs
	var ips []net.IP
	// Start from network address
	currentIP := cloneIP(foundSubnet.IP)

	// Apply padding: skip network address + padding IPs
	for i := 0; i <= padding; i++ {
		currentIP = incIP(currentIP)
	}

	broadcast := lastIP(foundSubnet)

	for k := 0; k < ipCount; k++ {
		// Check if we reached broadcast or went beyond
		if !foundSubnet.Contains(currentIP) || currentIP.Equal(broadcast) || isGreaterOrEqual(currentIP, broadcast) {
			return nil, nil, fmt.Errorf("not enough IPs in subnet %s (requested %d)", foundSubnet.String(), ipCount)
		}

		ips = append(ips, cloneIP(currentIP))
		currentIP = incIP(currentIP)
	}

	return foundSubnet, ips, nil
}

func ipToUint32(ip net.IP) uint32 {
	ip = ip.To4()
	if ip == nil {
		return 0
	}
	return binary.BigEndian.Uint32(ip)
}

func uint32ToIP(n uint32) net.IP {
	ip := make(net.IP, 4)
	binary.BigEndian.PutUint32(ip, n)
	return ip
}

// IsIPInCIDR checks if the given IP address belongs to the specified CIDR range.
func IsIPInCIDR(ipStr, cidrStr string) (bool, error) {
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return false, fmt.Errorf("invalid IP address: %s", ipStr)
	}

	_, ipNet, err := net.ParseCIDR(cidrStr)
	if err != nil {
		return false, fmt.Errorf("invalid CIDR: %w", err)
	}

	return ipNet.Contains(ip), nil
}
