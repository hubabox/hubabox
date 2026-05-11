package mdns

import (
	"fmt"
	"net"
	"strings"

	"github.com/grandcat/zeroconf"
)

// ListenPort parses TCP listen address like ":8787" or "0.0.0.0:8787".
func ListenPort(addr string) int {
	addr = strings.TrimSpace(addr)
	if addr == "" {
		return 8787
	}
	if strings.HasPrefix(addr, ":") {
		var p int
		_, _ = fmt.Sscanf(addr, ":%d", &p)
		if p > 0 {
			return p
		}
		return 8787
	}
	_, sport, err := net.SplitHostPort(addr)
	if err != nil {
		return 8787
	}
	var p int
	_, _ = fmt.Sscanf(sport, "%d", &p)
	if p <= 0 {
		return 8787
	}
	return p
}

// Register announces an _http._tcp service on the local network (Bonjour / mDNS).
func Register(instance string, port int) (*zeroconf.Server, error) {
	txt := []string{"app=hubabox", "path=/", "library=/library"}
	return zeroconf.Register(instance, "_http._tcp", "local.", port, txt, nil)
}
