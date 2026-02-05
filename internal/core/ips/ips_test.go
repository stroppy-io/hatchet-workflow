package ips

import (
	"math/big"
	"net"
	"testing"
)

func TestFirstFreeIP(t *testing.T) {
	tests := []struct {
		name    string
		cidr    string
		usedIPs []string
		want    string
		wantErr bool
	}{
		{
			name:    "first IP in empty range (with padding=3, skips .0, .1, .2, .3)",
			cidr:    "192.168.1.0/24",
			usedIPs: []string{},
			want:    "192.168.1.4",
			wantErr: false,
		},
		{
			name:    "skip used IPs after padding",
			cidr:    "192.168.1.0/24",
			usedIPs: []string{"192.168.1.4", "192.168.1.5", "192.168.1.6"},
			want:    "192.168.1.7",
			wantErr: false,
		},
		{
			name:    "skip invalid IPs in used list",
			cidr:    "192.168.1.0/24",
			usedIPs: []string{"invalid", "192.168.1.4", "not-an-ip"},
			want:    "192.168.1.5",
			wantErr: false,
		},
		{
			name:    "skip IPs outside CIDR (returns first after padding)",
			cidr:    "192.168.1.0/24",
			usedIPs: []string{"10.0.0.1", "192.168.2.1"},
			want:    "192.168.1.4",
			wantErr: false,
		},
		{
			name:    "small subnet /30 (no usable IPs after padding)",
			cidr:    "192.168.1.0/30",
			usedIPs: []string{},
			want:    "",
			wantErr: true,
		},
		{
			name:    "small subnet /29 with first IP available",
			cidr:    "192.168.1.0/29",
			usedIPs: []string{},
			want:    "192.168.1.4",
			wantErr: false,
		},
		{
			name:    "small subnet /29 all IPs used",
			cidr:    "192.168.1.0/29",
			usedIPs: []string{"192.168.1.4", "192.168.1.5", "192.168.1.6"},
			want:    "",
			wantErr: true,
		},
		{
			name:    "IPv6 basic (with padding)",
			cidr:    "2001:db8::/120",
			usedIPs: []string{},
			want:    "2001:db8::4",
			wantErr: false,
		},
		{
			name:    "IPv6 with used IPs after padding",
			cidr:    "2001:db8::/120",
			usedIPs: []string{"2001:db8::4", "2001:db8::5"},
			want:    "2001:db8::6",
			wantErr: false,
		},
		{
			name:    "padding respects CIDR - 10.1.0.0/24",
			cidr:    "10.1.0.0/24",
			usedIPs: []string{},
			want:    "10.1.0.4",
			wantErr: false,
		},
		{
			name:    "padding with used IPs in reserved range (should be ignored)",
			cidr:    "10.1.0.0/24",
			usedIPs: []string{"10.1.0.0", "10.1.0.1", "10.1.0.2", "10.1.0.3"},
			want:    "10.1.0.4",
			wantErr: false,
		},
		{
			name:    "find free IP after padding and some used IPs",
			cidr:    "10.1.0.0/24",
			usedIPs: []string{"10.1.0.4", "10.1.0.5"},
			want:    "10.1.0.6",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, ipNet, err := net.ParseCIDR(tt.cidr)
			if err != nil {
				t.Fatalf("failed to parse CIDR: %v", err)
			}

			got, err := FirstFreeIP(ipNet, tt.usedIPs)
			if (err != nil) != tt.wantErr {
				t.Errorf("FirstFreeIP() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got.String() != tt.want {
				t.Errorf("FirstFreeIP() = %v, want %v", got.String(), tt.want)
			}
		})
	}
}

func TestFirstFreeIP_Padding(t *testing.T) {
	// This test specifically validates that the padding constant is respected
	// and the first (padding+1) IPs are always skipped
	tests := []struct {
		name          string
		cidr          string
		expectedFirst string
		description   string
	}{
		{
			name:          "IPv4 /24 padding validation",
			cidr:          "10.1.0.0/24",
			expectedFirst: "10.1.0.4",
			description:   "Should skip 10.1.0.0, 10.1.0.1, 10.1.0.2, 10.1.0.3 (padding=3)",
		},
		{
			name:          "IPv4 /16 padding validation",
			cidr:          "172.16.0.0/16",
			expectedFirst: "172.16.0.4",
			description:   "Should skip 172.16.0.0, 172.16.0.1, 172.16.0.2, 172.16.0.3",
		},
		{
			name:          "IPv4 /28 small subnet",
			cidr:          "192.168.1.16/28",
			expectedFirst: "192.168.1.20",
			description:   "Network address is 192.168.1.16, should skip .16, .17, .18, .19",
		},
		{
			name:          "IPv6 /64 padding validation",
			cidr:          "2001:db8::/64",
			expectedFirst: "2001:db8::4",
			description:   "Should skip ::0, ::1, ::2, ::3",
		},
		{
			name:          "IPv6 /120 padding validation",
			cidr:          "fd00:1234:5678::/120",
			expectedFirst: "fd00:1234:5678::4",
			description:   "Should skip ::0, ::1, ::2, ::3",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, ipNet, err := net.ParseCIDR(tt.cidr)
			if err != nil {
				t.Fatalf("failed to parse CIDR: %v", err)
			}

			// Get first free IP with no used IPs
			got, err := FirstFreeIP(ipNet, []string{})
			if err != nil {
				t.Fatalf("FirstFreeIP() error = %v", err)
			}

			if got.String() != tt.expectedFirst {
				t.Errorf("%s: FirstFreeIP() = %v, want %v", tt.description, got.String(), tt.expectedFirst)
			}

			// Verify that the reserved IPs (network + padding) are actually skipped
			networkAddr := ipNet.IP.Mask(ipNet.Mask)
			for i := 0; i <= padding; i++ {
				testIP := cloneIP(networkAddr)
				for j := 0; j < i; j++ {
					testIP = incIP(testIP)
				}
				if got.Equal(testIP) {
					t.Errorf("FirstFreeIP() returned reserved IP %v (index %d in padding range)", testIP, i)
				}
			}
		})
	}
}

func TestFirstFreeIP_EdgeCases(t *testing.T) {
	tests := []struct {
		name    string
		cidr    string
		usedIPs []string
		wantErr bool
		desc    string
	}{
		{
			name:    "very small subnet /31 - no usable IPs after padding",
			cidr:    "192.168.1.0/31",
			usedIPs: []string{},
			wantErr: true,
			desc:    "/31 has only 2 IPs total, padding requires 4 to be skipped",
		},
		{
			name:    "very small subnet /32 - single IP",
			cidr:    "192.168.1.1/32",
			usedIPs: []string{},
			wantErr: true,
			desc:    "/32 has only 1 IP, cannot satisfy padding requirement",
		},
		{
			name:    "nearly full subnet",
			cidr:    "192.168.1.0/29",
			usedIPs: []string{"192.168.1.4", "192.168.1.5", "192.168.1.6"},
			wantErr: true,
			desc:    "/29 has 8 IPs: .0-.7, broadcast is .7, padding reserves .0-.3, only .4-.6 usable, all taken",
		},
		{
			name:    "last IP available after padding",
			cidr:    "192.168.1.0/29",
			usedIPs: []string{"192.168.1.4", "192.168.1.5"},
			wantErr: false,
			desc:    ".6 should still be available (broadcast is .7)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, ipNet, err := net.ParseCIDR(tt.cidr)
			if err != nil {
				t.Fatalf("failed to parse CIDR: %v", err)
			}

			_, err = FirstFreeIP(ipNet, tt.usedIPs)
			if (err != nil) != tt.wantErr {
				t.Errorf("%s: FirstFreeIP() error = %v, wantErr %v", tt.desc, err, tt.wantErr)
			}
		})
	}
}

func TestRandomIP(t *testing.T) {
	tests := []struct {
		name    string
		cidr    string
		wantErr bool
	}{
		{
			name:    "IPv4 /24",
			cidr:    "192.168.1.0/24",
			wantErr: false,
		},
		{
			name:    "IPv4 /16",
			cidr:    "10.0.0.0/16",
			wantErr: false,
		},
		{
			name:    "IPv4 /30 (small subnet)",
			cidr:    "192.168.1.0/30",
			wantErr: false,
		},
		{
			name:    "IPv4 /32 (no hosts)",
			cidr:    "192.168.1.1/32",
			wantErr: true,
		},
		{
			name:    "IPv4 /31 (no hosts)",
			cidr:    "192.168.1.0/31",
			wantErr: true,
		},
		{
			name:    "IPv6 /120",
			cidr:    "2001:db8::/120",
			wantErr: false,
		},
		{
			name:    "IPv6 /64",
			cidr:    "2001:db8::/64",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, ipNet, err := net.ParseCIDR(tt.cidr)
			if err != nil {
				t.Fatalf("failed to parse CIDR: %v", err)
			}

			got, err := RandomIP(ipNet)
			if (err != nil) != tt.wantErr {
				t.Errorf("RandomIP() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				// Verify IP is within range
				if !ipNet.Contains(got) {
					t.Errorf("RandomIP() = %v is not in network %v", got, ipNet)
				}

				// Verify it's not network or broadcast address
				networkAddr := ipNet.IP.Mask(ipNet.Mask)
				broadcast := lastIP(ipNet)

				if got.Equal(networkAddr) {
					t.Errorf("RandomIP() returned network address %v", got)
				}
				if got.Equal(broadcast) {
					t.Errorf("RandomIP() returned broadcast address %v", got)
				}
			}
		})
	}
}

func TestRandomIP_Distribution(t *testing.T) {
	// Test that RandomIP generates different IPs
	_, ipNet, err := net.ParseCIDR("192.168.1.0/24")
	if err != nil {
		t.Fatalf("failed to parse CIDR: %v", err)
	}

	seen := make(map[string]bool)
	iterations := 100

	for i := 0; i < iterations; i++ {
		ip, err := RandomIP(ipNet)
		if err != nil {
			t.Fatalf("RandomIP() failed: %v", err)
		}
		seen[ip.String()] = true
	}

	// We should see more than one unique IP in 100 iterations
	if len(seen) <= 1 {
		t.Errorf("RandomIP() generated only %d unique IP(s) in %d iterations", len(seen), iterations)
	}
}

func TestIncIP(t *testing.T) {
	tests := []struct {
		name string
		ip   string
		want string
	}{
		{
			name: "IPv4 simple increment",
			ip:   "192.168.1.1",
			want: "192.168.1.2",
		},
		{
			name: "IPv4 overflow octet",
			ip:   "192.168.1.255",
			want: "192.168.2.0",
		},
		{
			name: "IPv4 overflow multiple octets",
			ip:   "192.168.255.255",
			want: "192.169.0.0",
		},
		{
			name: "IPv6 simple increment",
			ip:   "2001:db8::1",
			want: "2001:db8::2",
		},
		{
			name: "IPv6 overflow",
			ip:   "2001:db8::ffff",
			want: "2001:db8::1:0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ip := net.ParseIP(tt.ip)
			if ip == nil {
				t.Fatalf("failed to parse IP: %s", tt.ip)
			}

			got := incIP(cloneIP(ip))
			if got.String() != tt.want {
				t.Errorf("incIP() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestLastIP(t *testing.T) {
	tests := []struct {
		name string
		cidr string
		want string
	}{
		{
			name: "IPv4 /24",
			cidr: "192.168.1.0/24",
			want: "192.168.1.255",
		},
		{
			name: "IPv4 /16",
			cidr: "10.0.0.0/16",
			want: "10.0.255.255",
		},
		{
			name: "IPv4 /30",
			cidr: "192.168.1.0/30",
			want: "192.168.1.3",
		},
		{
			name: "IPv4 /32",
			cidr: "192.168.1.1/32",
			want: "192.168.1.1",
		},
		{
			name: "IPv6 /120",
			cidr: "2001:db8::/120",
			want: "2001:db8::ff",
		},
		{
			name: "IPv6 /64",
			cidr: "2001:db8::/64",
			want: "2001:db8::ffff:ffff:ffff:ffff",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, ipNet, err := net.ParseCIDR(tt.cidr)
			if err != nil {
				t.Fatalf("failed to parse CIDR: %v", err)
			}

			got := lastIP(ipNet)
			if got.String() != tt.want {
				t.Errorf("lastIP() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCloneIP(t *testing.T) {
	original := net.ParseIP("192.168.1.1")
	clone := cloneIP(original)

	// Verify they are equal
	if !clone.Equal(original) {
		t.Errorf("cloneIP() = %v, want %v", clone, original)
	}

	// Modify clone and verify original is unchanged
	clone[len(clone)-1] = 99
	if original[len(original)-1] == 99 {
		t.Errorf("cloneIP() did not create a true copy - original was modified")
	}
}

func TestNormalizeIPFamily(t *testing.T) {
	tests := []struct {
		name string
		ip   string
		base string
		want string
	}{
		{
			name: "both IPv4",
			ip:   "192.168.1.1",
			base: "10.0.0.1",
			want: "192.168.1.1",
		},
		{
			name: "IPv4-mapped to IPv4 base",
			ip:   "::ffff:192.168.1.1",
			base: "10.0.0.1",
			want: "192.168.1.1",
		},
		{
			name: "both IPv6",
			ip:   "2001:db8::1",
			base: "2001:db8::2",
			want: "2001:db8::1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ip := net.ParseIP(tt.ip)
			base := net.ParseIP(tt.base)

			got := normalizeIPFamily(ip, base)
			if got.String() != tt.want {
				t.Errorf("normalizeIPFamily() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAddBigIntToIP(t *testing.T) {
	tests := []struct {
		name   string
		ip     string
		offset int64
		want   string
	}{
		{
			name:   "IPv4 add 1",
			ip:     "192.168.1.1",
			offset: 1,
			want:   "192.168.1.2",
		},
		{
			name:   "IPv4 add 100",
			ip:     "192.168.1.1",
			offset: 100,
			want:   "192.168.1.101",
		},
		{
			name:   "IPv4 add with overflow",
			ip:     "192.168.1.200",
			offset: 100,
			want:   "192.168.2.44",
		},
		{
			name:   "IPv6 add 1",
			ip:     "2001:db8::1",
			offset: 1,
			want:   "2001:db8::2",
		},
		{
			name:   "IPv6 add large number",
			ip:     "2001:db8::1",
			offset: 1000,
			want:   "2001:db8::3e9",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ip := net.ParseIP(tt.ip)
			if ip == nil {
				t.Fatalf("failed to parse IP: %s", tt.ip)
			}

			offset := big.NewInt(tt.offset)
			got := addBigIntToIP(ip, offset)

			if got.String() != tt.want {
				t.Errorf("addBigIntToIP() = %v, want %v", got, tt.want)
			}
		})
	}
}

func BenchmarkFirstFreeIP(b *testing.B) {
	_, ipNet, _ := net.ParseCIDR("192.168.1.0/24")
	usedIPs := []string{"192.168.1.1", "192.168.1.2", "192.168.1.3"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = FirstFreeIP(ipNet, usedIPs)
	}
}

func BenchmarkRandomIP(b *testing.B) {
	_, ipNet, _ := net.ParseCIDR("192.168.1.0/24")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = RandomIP(ipNet)
	}
}

func BenchmarkRandomIP_LargeNetwork(b *testing.B) {
	_, ipNet, _ := net.ParseCIDR("10.0.0.0/16")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = RandomIP(ipNet)
	}
}

func TestNextSubnetByCount(t *testing.T) {
	parent := "10.1.0.0/16"
	newPrefix := 24

	// Пример: уже создано 5 подсетей /24, хотим сгенерировать следующую
	createdCount := 5

	next, err := NextSubnetByCount(parent, newPrefix, createdCount)
	if err != nil {
		t.Fatalf("NextSubnetByCount() failed: %v", err)
		return
	}
	t.Logf("Next subnet after %d created: %s\n", createdCount, next.String())
}

func TestNextSubnetWithIPs(t *testing.T) {
	tests := []struct {
		name            string
		parentCIDR      string
		newPrefix       int
		existingSubnets []string
		ipCount         int
		wantSubnet      string
		wantIPCount     int
		wantErr         bool
	}{
		{
			name:            "first subnet in empty parent",
			parentCIDR:      "10.0.0.0/16",
			newPrefix:       24,
			existingSubnets: []string{},
			ipCount:         5,
			wantSubnet:      "10.0.0.0/24",
			wantIPCount:     5,
			wantErr:         false,
		},
		{
			name:            "skip existing subnet at start",
			parentCIDR:      "10.0.0.0/16",
			newPrefix:       24,
			existingSubnets: []string{"10.0.0.0/24"},
			ipCount:         5,
			wantSubnet:      "10.0.1.0/24",
			wantIPCount:     5,
			wantErr:         false,
		},
		{
			name:            "skip multiple existing subnets",
			parentCIDR:      "10.0.0.0/16",
			newPrefix:       24,
			existingSubnets: []string{"10.0.0.0/24", "10.0.2.0/24"},
			ipCount:         5,
			wantSubnet:      "10.0.1.0/24",
			wantIPCount:     5,
			wantErr:         false,
		},
		{
			name:            "skip overlapping larger subnet",
			parentCIDR:      "10.0.0.0/16",
			newPrefix:       24,
			existingSubnets: []string{"10.0.0.0/23"}, // Covers 10.0.0.0/24 and 10.0.1.0/24
			ipCount:         5,
			wantSubnet:      "10.0.2.0/24",
			wantIPCount:     5,
			wantErr:         false,
		},
		{
			name:            "no space left",
			parentCIDR:      "10.0.0.0/24",
			newPrefix:       25,
			existingSubnets: []string{"10.0.0.0/25", "10.0.0.128/25"},
			ipCount:         5,
			wantSubnet:      "",
			wantIPCount:     0,
			wantErr:         true,
		},
		{
			name:            "not enough IPs in subnet (padding)",
			parentCIDR:      "10.0.0.0/24",
			newPrefix:       30, // 4 IPs total, 2 usable? padding=3 -> 0 usable?
			existingSubnets: []string{},
			ipCount:         1,
			wantSubnet:      "",
			wantIPCount:     0,
			wantErr:         true,
		},
		{
			name:            "enough IPs in subnet",
			parentCIDR:      "10.0.0.0/24",
			newPrefix:       29, // 8 IPs total. .0 network, .7 broadcast. padding .0-.3. usable .4, .5, .6
			existingSubnets: []string{},
			ipCount:         3,
			wantSubnet:      "10.0.0.0/29",
			wantIPCount:     3,
			wantErr:         false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			subnet, ips, err := NextSubnetWithIPs(tt.parentCIDR, tt.newPrefix, tt.existingSubnets, tt.ipCount)
			if (err != nil) != tt.wantErr {
				t.Errorf("NextSubnetWithIPs() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if subnet.String() != tt.wantSubnet {
					t.Errorf("NextSubnetWithIPs() subnet = %v, want %v", subnet.String(), tt.wantSubnet)
				}
				if len(ips) != tt.wantIPCount {
					t.Errorf("NextSubnetWithIPs() ip count = %v, want %v", len(ips), tt.wantIPCount)
				}
				// Verify IPs are in subnet
				for _, ip := range ips {
					if !subnet.Contains(ip) {
						t.Errorf("Returned IP %v not in subnet %v", ip, subnet)
					}
				}
				// Verify IPs are distinct
				seen := make(map[string]bool)
				for _, ip := range ips {
					if seen[ip.String()] {
						t.Errorf("Duplicate IP returned: %v", ip)
					}
					seen[ip.String()] = true
				}
			}
		})
	}
}

func TestIsIPInCIDR(t *testing.T) {
	tests := []struct {
		name    string
		ip      string
		cidr    string
		want    bool
		wantErr bool
	}{
		{
			name:    "IP in CIDR",
			ip:      "192.168.1.5",
			cidr:    "192.168.1.0/24",
			want:    true,
			wantErr: false,
		},
		{
			name:    "IP not in CIDR",
			ip:      "192.168.2.5",
			cidr:    "192.168.1.0/24",
			want:    false,
			wantErr: false,
		},
		{
			name:    "Invalid IP",
			ip:      "invalid-ip",
			cidr:    "192.168.1.0/24",
			want:    false,
			wantErr: true,
		},
		{
			name:    "Invalid CIDR",
			ip:      "192.168.1.5",
			cidr:    "invalid-cidr",
			want:    false,
			wantErr: true,
		},
		{
			name:    "IPv6 in CIDR",
			ip:      "2001:db8::1",
			cidr:    "2001:db8::/64",
			want:    true,
			wantErr: false,
		},
		{
			name:    "IPv6 not in CIDR",
			ip:      "2001:db9::1",
			cidr:    "2001:db8::/64",
			want:    false,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := IsIPInCIDR(tt.ip, tt.cidr)
			if (err != nil) != tt.wantErr {
				t.Errorf("IsIPInCIDR() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("IsIPInCIDR() = %v, want %v", got, tt.want)
			}
		})
	}
}
