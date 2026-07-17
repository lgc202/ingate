//go:build wasm

package redisabi

import (
	"time"
	"unsafe"
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
	if data == nil || size <= 0 {
		return nil, nil
	}
	return append([]byte(nil), unsafe.Slice(data, size)...), nil
}
