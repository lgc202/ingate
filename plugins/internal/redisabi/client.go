package redisabi

import (
	"errors"
	"sync"
	"time"
)

const (
	// SystemCluster 是 Envoy bootstrap 中系统 Redis 的固定 cluster 名称
	SystemCluster = "ingate-system-redis"
	// CommandTimeout 是 Redis hostcall 的系统级固定超时
	CommandTimeout = 50 * time.Millisecond

	clusterInitialization = SystemCluster + "?buffer_flush_timeout=0&max_buffer_size_before_flush=0"
)

// Result 表示一次 Redis 异步调用的结果
type Result struct {
	Status RedisStatus
	Data   []byte
}

// Callback 在 Redis 异步调用完成后执行
type Callback func(Result)

var (
	initializeOnce sync.Once
	initializeErr  error
	callbacks      = newCallbackRegistry()
)

// Init 初始化当前 Wasm VM 使用的固定系统 Redis cluster
func Init() error {
	initializeOnce.Do(func() {
		initializeErr = hostInit(clusterInitialization, CommandTimeout)
	})
	return initializeErr
}

// Dispatch 向系统 Redis 发送已经编码的 RESP 命令
func Dispatch(pluginContextID uint32, query []byte, callback Callback) (uint32, error) {
	if len(query) == 0 {
		return 0, errors.New("redis ABI query is empty")
	}
	if callback == nil {
		return 0, errors.New("redis ABI callback is required")
	}

	calloutID, err := hostCall(SystemCluster, query)
	if err != nil {
		return 0, err
	}
	if err := callbacks.add(callKey{pluginContextID: pluginContextID, calloutID: calloutID}, callback); err != nil {
		return 0, err
	}
	return calloutID, nil
}

func handleCallback(pluginContextID, calloutID uint32, status RedisStatus, responseSize int32) {
	callback, exists := callbacks.take(callKey{pluginContextID: pluginContextID, calloutID: calloutID})
	if !exists {
		return
	}

	result := Result{Status: status}
	if status == 0 && responseSize > 0 {
		data, err := hostResponse(responseSize)
		if err != nil {
			result.Status = -1
		} else {
			result.Data = data
		}
	}
	callback(result)
}
