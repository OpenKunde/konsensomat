package main

import (
	"fmt"
	"net"
	"strings"
)

// trustedProxies holds the reverse proxies (single IPs or CIDR ranges)
// configured via KONSENSOMAT_TRUSTED_PROXIES that are allowed to set
// X-Forwarded-For/X-Real-IP. Empty (the default) means no proxy is trusted,
// and clientIP always uses the raw connecting address - X-Forwarded-For is
// just a request header, trivially set by anyone who can reach the server
// directly, so it must never be trusted unless the request is known to have
// actually come through one of these proxies.
var trustedProxies []*net.IPNet

// parseTrustedProxies parses a comma-separated list of IPs (treated as a
// single-address /32 or /128) and/or CIDR ranges, e.g.
// "10.0.0.1,172.16.0.0/12,::1". An empty string yields no trusted proxies,
// which is the safe default.
func parseTrustedProxies(raw string) ([]*net.IPNet, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}

	var nets []*net.IPNet
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		if !strings.Contains(part, "/") {
			ip := net.ParseIP(part)
			if ip == nil {
				return nil, fmt.Errorf("ungültige IP-Adresse oder CIDR: %q", part)
			}
			if ip.To4() != nil {
				part += "/32"
			} else {
				part += "/128"
			}
		}

		_, ipNet, err := net.ParseCIDR(part)
		if err != nil {
			return nil, fmt.Errorf("ungültige IP-Adresse oder CIDR: %q", part)
		}
		nets = append(nets, ipNet)
	}
	return nets, nil
}

// isTrustedProxy reports whether ip falls within one of the configured
// trusted proxy ranges.
func isTrustedProxy(ip net.IP) bool {
	for _, n := range trustedProxies {
		if n.Contains(ip) {
			return true
		}
	}
	return false
}
