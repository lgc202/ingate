package redisabi

import (
	"errors"
	"fmt"
	"sync"
	"time"
)

const (
	// SystemCluster 是 Envoy bootstrap 中系统 Redis 的固定 cluster 名称
	SystemCluster = "ingate-system-redis"
	// CommandTimeout 是请求路径上单次 Redis 操作的固定超时
	CommandTimeout = 50 * time.Millisecond

	clusterInitialization = SystemCluster + "?buffer_flush_timeout=0&max_buffer_size_before_flush=0"
	lateCallbackLog       = "late_callback_ignored"
)

// Result 表示一次 Redis 异步调用的结果
type Result struct {
	Status RedisStatus
	Data   []byte
	Err    error
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

// RegisterHTTPContext 登记属于指定插件实例的 HTTP context 存活状态
func RegisterHTTPContext(pluginContextID, httpContextID uint32) {
	callbacks.registerContext(contextKey{pluginContextID: pluginContextID, httpContextID: httpContextID})
}

// CloseHTTPContext 标记 HTTP context 已销毁，但保留仍在飞行的 callback 记录
func CloseHTTPContext(pluginContextID, httpContextID uint32) {
	callbacks.closeContext(contextKey{pluginContextID: pluginContextID, httpContextID: httpContextID})
}

// Dispatch 在显式 HTTP context 中向系统 Redis 发送 RESP 命令
func Dispatch(pluginContextID, httpContextID uint32, query []byte, callback Callback) (uint32, error) {
	if len(query) == 0 {
		return 0, errors.New("redis ABI query is empty")
	}
	if callback == nil {
		return 0, errors.New("redis ABI callback is required")
	}
	context := contextKey{pluginContextID: pluginContextID, httpContextID: httpContextID}
	if !callbacks.contextAlive(context) {
		return 0, errors.New("redis ABI HTTP context is closed")
	}
	if err := hostSetEffectiveContext(httpContextID); err != nil {
		return 0, err
	}

	calloutID, err := hostCall(SystemCluster, query)
	if err != nil {
		return 0, err
	}
	record := callbackRecord{
		pluginContextID: pluginContextID,
		httpContextID:   httpContextID,
		callback:        callback,
	}
	if err := callbacks.add(callKey{pluginContextID: pluginContextID, calloutID: calloutID}, record); err != nil {
		return 0, err
	}
	return calloutID, nil
}

// ResumeHTTPRequest 在显式 HTTP context 中恢复暂停的请求流
func ResumeHTTPRequest(httpContextID uint32) error {
	if err := hostSetEffectiveContext(httpContextID); err != nil {
		return err
	}
	return hostResumeHTTPRequest()
}

// SendHTTPResponse 在显式 HTTP context 中返回本地响应
func SendHTTPResponse(httpContextID uint32, statusCode int, headers map[string]string, body string) error {
	if statusCode <= 0 || uint64(statusCode) > uint64(^uint32(0)) {
		return errors.New("HTTP response status code is out of range")
	}
	if err := hostSetEffectiveContext(httpContextID); err != nil {
		return err
	}
	return hostSendHTTPResponse(uint32(statusCode), headers, []byte(body))
}

func handleCallback(pluginContextID, calloutID uint32, status RedisStatus, responseSize int32) {
	key := callKey{pluginContextID: pluginContextID, calloutID: calloutID}
	if err := hostSetEffectiveContext(pluginContextID); err != nil {
		callbacks.remove(key)
		hostLogWarning(lateCallbackLog)
		return
	}

	record, exists := callbacks.get(key)
	if !exists || record.pluginContextID != pluginContextID {
		hostLogWarning(lateCallbackLog)
		return
	}
	context := contextKey{pluginContextID: record.pluginContextID, httpContextID: record.httpContextID}
	if !callbacks.contextAlive(context) {
		callbacks.remove(key)
		hostLogWarning(lateCallbackLog)
		return
	}
	if err := hostSetEffectiveContext(record.httpContextID); err != nil {
		callbacks.remove(key)
		hostLogWarning(lateCallbackLog)
		return
	}
	callbacks.remove(key)

	result := Result{Status: status}
	if status == RedisStatusOK && responseSize > 0 {
		result.Data, result.Err = hostResponse(responseSize)
	} else if status == RedisStatusOK && responseSize < 0 {
		result.Err = fmt.Errorf("redis ABI response size is %d", responseSize)
	}
	record.callback(result)
}
