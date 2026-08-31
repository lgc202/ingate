// Package iprestrictionconfig 定义 IP 访问限制策略各信任边界共享的稳定领域约束。
package iprestrictionconfig

import (
	"net/netip"
	"strings"
)

// MaxRanges 限制一条策略可以声明的 IP 地址或 CIDR 数量。
const MaxRanges = 1_024

// NormalizeRange 校验 IP 地址或 CIDR，并返回等价的规范 CIDR。
func NormalizeRange(value string) (string, bool) {
	value = strings.TrimSpace(value)
	if address, err := netip.ParseAddr(value); err == nil {
		return netip.PrefixFrom(address, address.BitLen()).String(), true
	}
	prefix, err := netip.ParsePrefix(value)
	if err != nil {
		return "", false
	}
	return prefix.Masked().String(), true
}
