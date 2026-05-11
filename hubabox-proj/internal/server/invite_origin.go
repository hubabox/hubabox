package server

import (
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/kros/hubabox/internal/mdns"
)

// normalizePublicOrigin returns a canonical http(s)://host:port base with no path, or "" if invalid.
func normalizePublicOrigin(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	if !strings.Contains(s, "://") {
		s = "http://" + s
	}
	u, err := url.Parse(s)
	if err != nil || u.Host == "" {
		return ""
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return ""
	}
	u.Path, u.RawQuery, u.Fragment = "", "", ""
	return u.Scheme + "://" + u.Host
}

func stripIPBrackets(host string) string {
	if strings.HasPrefix(host, "[") && strings.HasSuffix(host, "]") {
		return host[1 : len(host)-1]
	}
	return host
}

// requestHostUnusableForInvite is true when r.Host should not be used as the guest-facing base
// (e.g. browser opened admin at localhost — net.ParseIP("localhost") is nil, so we must not treat it as a public hostname).
func requestHostUnusableForInvite(host string) bool {
	h := strings.TrimSpace(stripIPBrackets(host))
	if h == "" {
		return true
	}
	if strings.EqualFold(h, "localhost") {
		return true
	}
	if ip := net.ParseIP(h); ip != nil {
		return ip.IsLoopback() || ip.IsUnspecified()
	}
	return false
}

// libraryInviteOrigin returns scheme://host:port (no path) for guest-facing invite URLs.
// Order: HUBABOX_PUBLIC_ORIGIN / -public-origin; non-loopback Host; first LAN IPv4;
// http://<os-hostname>.local:port when hostname looks like a single DNS label (e.g. pop-os).
func libraryInviteOrigin(r *http.Request, lanIPs []string, listenAddr, osHostname, publicOrigin string) string {
	if o := normalizePublicOrigin(publicOrigin); o != "" {
		return o
	}
	cfgPort := mdns.ListenPort(listenAddr)
	hostport := strings.TrimSpace(r.Host)
	if hostport != "" {
		rawHost, rawPort, err := net.SplitHostPort(hostport)
		if err == nil {
			port := cfgPort
			if p, err := strconv.Atoi(rawPort); err == nil && p > 0 {
				port = p
			}
			host := stripIPBrackets(rawHost)
			if requestHostUnusableForInvite(host) {
				// fall through to LAN / hostname.local
			} else if ip := net.ParseIP(host); ip != nil {
				return "http://" + net.JoinHostPort(host, strconv.Itoa(port))
			} else {
				return "http://" + net.JoinHostPort(host, strconv.Itoa(port))
			}
		}
	}
	if o := fallbackLANOrigin(lanIPs, cfgPort); o != "" {
		return o
	}
	return fallbackHostnameLocalOrigin(osHostname, cfgPort)
}

func fallbackLANOrigin(lanIPs []string, port int) string {
	if len(lanIPs) == 0 {
		return ""
	}
	return "http://" + net.JoinHostPort(lanIPs[0], strconv.Itoa(port))
}

// fallbackHostnameLocalOrigin uses hostname.local for typical single-label OS hostnames (pop-os → pop-os.local).
func fallbackHostnameLocalOrigin(hostname string, port int) string {
	h := strings.TrimSpace(hostname)
	if h == "" {
		return ""
	}
	if strings.HasSuffix(strings.ToLower(h), ".local") {
		return "http://" + net.JoinHostPort(h, strconv.Itoa(port))
	}
	// Avoid turning "host.example.com" into "host.example.com.local"
	if strings.Contains(h, ".") {
		return "http://" + net.JoinHostPort(h, strconv.Itoa(port))
	}
	return "http://" + net.JoinHostPort(h+".local", strconv.Itoa(port))
}
