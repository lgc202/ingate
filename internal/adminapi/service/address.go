package service

import (
	"net/netip"
	"strings"
)

// validEndpointAddress 接受 IP 或 DNS 名称，供 Route Host 和 Upstream 地址共用
func validEndpointAddress(address string) bool {
	if _, err := netip.ParseAddr(address); err == nil {
		return true
	}
	if address == "" || len(address) > 253 {
		return false
	}
	for label := range strings.SplitSeq(strings.ToLower(address), ".") {
		if label == "" || len(label) > 63 || strings.HasPrefix(label, "-") || strings.HasSuffix(label, "-") {
			return false
		}
		for _, char := range label {
			if (char < 'a' || char > 'z') && (char < '0' || char > '9') && char != '-' {
				return false
			}
		}
	}
	return true
}
