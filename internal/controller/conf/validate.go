// Package conf 定义并校验 ingate-controller 进程配置
package conf

import (
	"errors"
	"net/netip"
	"strings"
)

// Validate 校验 Controller 进程启动所需的配置
func (c *Bootstrap) Validate() error {
	if c.GetServer() == nil || c.GetServer().GetGrpc() == nil || c.GetServer().GetHttp() == nil {
		return errors.New("server gRPC and HTTP config are required")
	}
	xdsAddress, err := netip.ParseAddrPort(strings.TrimSpace(c.GetServer().GetGrpc().GetAddr()))
	if err != nil || !xdsAddress.Addr().Unmap().IsLoopback() {
		return errors.New("server gRPC address must use a loopback IP and port because xDS does not enable mTLS")
	}
	if _, err := netip.ParseAddrPort(strings.TrimSpace(c.GetServer().GetHttp().GetAddr())); err != nil {
		return errors.New("server HTTP address must contain a valid IP and port")
	}
	if c.GetServer().GetHttp().GetTimeout() == nil || c.GetServer().GetHttp().GetTimeout().AsDuration() <= 0 {
		return errors.New("server HTTP timeout must be greater than zero")
	}
	if c.GetServer().GetShutdownTimeout() == nil || c.GetServer().GetShutdownTimeout().AsDuration() <= 0 {
		return errors.New("server shutdown timeout must be greater than zero")
	}

	if c.GetData() == nil || c.GetData().GetApiserver() == nil {
		return errors.New("API Server config is required")
	}
	if strings.TrimSpace(c.GetData().GetApiserver().GetKubeconfig()) == "" {
		return errors.New("API Server kubeconfig must not be empty")
	}
	wasm := c.GetData().GetWasm()
	if wasm == nil {
		return errors.New("wasm module storage config is required")
	}
	if strings.TrimSpace(wasm.GetCacheDir()) == "" {
		return errors.New("wasm module cache directory must not be empty")
	}
	if wasm.GetPullTimeout() == nil || wasm.GetPullTimeout().AsDuration() <= 0 {
		return errors.New("wasm module pull timeout must be greater than zero")
	}
	if wasm.GetMaxModuleBytes() <= 0 {
		return errors.New("wasm maximum module size must be greater than zero")
	}
	if wasm.GetMaxCacheBytes() < wasm.GetMaxModuleBytes() {
		return errors.New("wasm cache size must not be smaller than the maximum module size")
	}
	if c.GetDelivery() == nil || c.GetDelivery().GetCandidateAckTimeout() == nil ||
		c.GetDelivery().GetCandidateAckTimeout().AsDuration() <= 0 {
		return errors.New("delivery candidate ACK timeout must be greater than zero")
	}
	if c.GetDelivery().GetNackRollbackTimeout() == nil ||
		c.GetDelivery().GetNackRollbackTimeout().AsDuration() <= 0 {
		return errors.New("delivery NACK rollback timeout must be greater than zero")
	}
	if c.GetResourceWatch() == nil || c.GetResourceWatch().GetResyncPeriod() == nil ||
		c.GetResourceWatch().GetResyncPeriod().AsDuration() < 0 {
		return errors.New("resource watch resync period must not be negative")
	}

	if c.GetLogging() == nil {
		return errors.New("logging config is required")
	}
	switch strings.ToLower(c.GetLogging().GetFormat()) {
	case "json", "text":
	default:
		return errors.New("logging format must be json or text")
	}
	switch strings.ToLower(c.GetLogging().GetLevel()) {
	case "debug", "info", "warn", "error":
	default:
		return errors.New("logging level must be debug, info, warn or error")
	}
	return nil
}
