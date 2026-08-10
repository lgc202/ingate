package policy

import (
	"net"
	"net/netip"
	"strings"
)

// Allows 判断请求来源 IP 是否通过当前 Route 上的全部限制策略
func (r Route) Allows(remoteAddress string) bool {
	address, err := netip.ParseAddr(clientIP(remoteAddress))
	if err != nil {
		return false
	}
	for _, policy := range r.policies {
		if !policy.allows(address) {
			return false
		}
	}
	return true
}

func (r restriction) allows(address netip.Addr) bool {
	if len(r.allow) > 0 {
		return contains(r.allow, address)
	}
	return !contains(r.deny, address)
}

func contains(prefixes []netip.Prefix, address netip.Addr) bool {
	for _, prefix := range prefixes {
		if prefix.Contains(address) {
			return true
		}
	}
	return false
}

func clientIP(remoteAddress string) string {
	if host, _, err := net.SplitHostPort(remoteAddress); err == nil {
		return strings.Trim(host, "[]")
	}
	return strings.Trim(remoteAddress, "[]")
}
