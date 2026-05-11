package netutil

import (
	"net"
	"sort"
	"strings"
)

// skipIface reports virtual / bridge interfaces whose addresses are usually not the LAN you want for guests.
func skipIface(name string) bool {
	l := strings.ToLower(strings.TrimSpace(name))
	prefixes := []string{
		"docker", "br-", "veth", "virbr", "vmnet", "vboxnet",
		"cni", "flannel", "calico", "wg", // common overlay / CNI
	}
	for _, p := range prefixes {
		if strings.HasPrefix(l, p) {
			return true
		}
	}
	return false
}

// isLikelyLANIPv4 is RFC1918 / CGNAT space (common on Wi‑Fi and Tailscale); excludes loopback and IPv6.
func isLikelyLANIPv4(ip net.IP) bool {
	ip4 := ip.To4()
	if ip4 == nil || ip4.IsLoopback() {
		return false
	}
	switch {
	case ip4[0] == 10:
		return true
	case ip4[0] == 172 && ip4[1] >= 16 && ip4[1] <= 31:
		return true
	case ip4[0] == 192 && ip4[1] == 168:
		return true
	case ip4[0] == 100 && ip4[1] >= 64 && ip4[1] <= 127: // CGNAT (incl. some overlay nets)
		return true
	default:
		return false
	}
}

// LANIPv4Strings returns private IPv4 addresses on up, non-loopback interfaces (best effort for “share this on Wi‑Fi”).
func LANIPv4Strings() ([]string, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}
	seen := make(map[string]struct{})
	var ips []string
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 {
			continue
		}
		if iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		if skipIface(iface.Name) {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, a := range addrs {
			ipnet, ok := a.(*net.IPNet)
			if !ok {
				continue
			}
			ip := ipnet.IP.To4()
			if ip == nil || !isLikelyLANIPv4(ip) {
				continue
			}
			s := ip.String()
			if _, dup := seen[s]; dup {
				continue
			}
			seen[s] = struct{}{}
			ips = append(ips, s)
		}
	}
	sort.Strings(ips)
	return ips, nil
}
