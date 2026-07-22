//go:build wasm

package redisabi

import (
	"encoding/binary"
	"fmt"
	"sort"
	"time"
	"unsafe"
)

const (
	warningLogLevel = 3
	requestStream   = 0
	responseStream  = 1
)

//go:wasmimport env proxy_redis_init
func proxyRedisInit(
	clusterData *byte,
	clusterSize int32,
	usernameData *byte,
	usernameSize int32,
	passwordData *byte,
	passwordSize int32,
	timeoutMilliseconds uint32,
) HostStatus

//go:wasmimport env proxy_redis_call
func proxyRedisCall(
	clusterData *byte,
	clusterSize int32,
	queryData *byte,
	querySize int32,
	calloutID *uint32,
) HostStatus

//go:wasmimport env proxy_get_buffer_bytes
func proxyGetBufferBytes(
	bufferType BufferType,
	start int32,
	maxSize int32,
	returnBufferData unsafe.Pointer,
	returnBufferSize *int32,
) HostStatus

//go:wasmimport env proxy_set_effective_context
func proxySetEffectiveContext(contextID uint32) HostStatus

//go:wasmimport env proxy_continue_stream
func proxyContinueStream(streamType uint32) HostStatus

//go:wasmimport env proxy_send_local_response
func proxySendLocalResponse(
	statusCode uint32,
	statusCodeDetailData *byte,
	statusCodeDetailsSize int32,
	bodyData *byte,
	bodySize int32,
	headersData *byte,
	headersSize int32,
	grpcStatus int32,
) HostStatus

//go:wasmimport env proxy_log
func proxyLog(level uint32, messageData *byte, messageSize int32) HostStatus

func hostInit(cluster string, timeout time.Duration) error {
	clusterBytes := []byte(cluster)
	status := proxyRedisInit(
		&clusterBytes[0],
		int32(len(clusterBytes)),
		nil,
		0,
		nil,
		0,
		uint32(timeout/time.Millisecond),
	)
	return checkHostStatus("init", status)
}

func hostCall(cluster string, query []byte) (uint32, error) {
	clusterBytes := []byte(cluster)
	var calloutID uint32
	status := proxyRedisCall(
		&clusterBytes[0],
		int32(len(clusterBytes)),
		&query[0],
		int32(len(query)),
		&calloutID,
	)
	if err := checkHostStatus("call", status); err != nil {
		return 0, err
	}
	return calloutID, nil
}

func hostResponse(responseSize int32) ([]byte, error) {
	var data *byte
	var size int32
	status := proxyGetBufferBytes(
		RedisCallResponseBuffer,
		0,
		responseSize,
		unsafe.Pointer(&data),
		&size,
	)
	if err := checkHostStatus("get response", status); err != nil {
		return nil, err
	}
	if size < 0 || size > responseSize {
		return nil, fmt.Errorf("redis ABI response size %d is invalid for callback size %d", size, responseSize)
	}
	if size == 0 {
		return nil, nil
	}
	if data == nil {
		return nil, fmt.Errorf("redis ABI response buffer is nil for size %d", size)
	}
	return append([]byte(nil), unsafe.Slice(data, size)...), nil
}

func hostSetEffectiveContext(contextID uint32) error {
	return checkHostStatus("set effective context", proxySetEffectiveContext(contextID))
}

func hostResumeHTTPRequest() error {
	return checkHostStatus("resume HTTP request", proxyContinueStream(requestStream))
}

func hostResumeHTTPResponse() error {
	return checkHostStatus("resume HTTP response", proxyContinueStream(responseStream))
}

func hostSendHTTPResponse(statusCode uint32, headers map[string]string, body []byte) error {
	serializedHeaders := serializeHeaders(headers)
	var bodyData *byte
	if len(body) > 0 {
		bodyData = &body[0]
	}
	status := proxySendLocalResponse(
		statusCode,
		nil,
		0,
		bodyData,
		int32(len(body)),
		&serializedHeaders[0],
		int32(len(serializedHeaders)),
		-1,
	)
	return checkHostStatus("send HTTP response", status)
}

func hostLogWarning(message string) {
	if message == "" {
		return
	}
	data := []byte(message)
	_ = proxyLog(warningLogLevel, &data[0], int32(len(data)))
}

func serializeHeaders(headers map[string]string) []byte {
	names := make([]string, 0, len(headers))
	for name := range headers {
		names = append(names, name)
	}
	sort.Strings(names)

	size := 4
	for _, name := range names {
		value := headers[name]
		size += len(name) + len(value) + 10
	}
	result := make([]byte, size)
	binary.LittleEndian.PutUint32(result[:4], uint32(len(headers)))

	offset := 4
	for _, name := range names {
		value := headers[name]
		binary.LittleEndian.PutUint32(result[offset:offset+4], uint32(len(name)))
		offset += 4
		binary.LittleEndian.PutUint32(result[offset:offset+4], uint32(len(value)))
		offset += 4
	}
	for _, name := range names {
		value := headers[name]
		copy(result[offset:], name)
		offset += len(name) + 1
		copy(result[offset:], value)
		offset += len(value) + 1
	}
	return result
}
