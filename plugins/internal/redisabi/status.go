package redisabi

import "fmt"

// HostStatus 表示 Proxy-Wasm hostcall 的返回状态
type HostStatus uint32

// RedisStatus 表示 Redis 异步调用的执行状态
type RedisStatus int32

// BufferType 表示 Proxy-Wasm buffer 类型
type BufferType uint32

const (
	// HostStatusOK 表示 hostcall 已被 Envoy 接受
	HostStatusOK HostStatus = 0
	// RedisStatusOK 表示 Redis 异步调用执行成功
	RedisStatusOK RedisStatus = 0
	// RedisCallResponseBuffer 表示 Redis call response buffer
	RedisCallResponseBuffer BufferType = 9
)

type hostError struct {
	operation string
	status    HostStatus
}

func (e *hostError) Error() string {
	return fmt.Sprintf("redis ABI %s failed with host status %d", e.operation, e.status)
}

func checkHostStatus(operation string, status HostStatus) error {
	if status == HostStatusOK {
		return nil
	}
	return &hostError{operation: operation, status: status}
}
