//go:build wasm

package redisabi

//go:wasmexport proxy_on_redis_call_response
func proxyOnRedisCallResponse(pluginContextID, calloutID uint32, status, responseSize int32) {
	handleCallback(pluginContextID, calloutID, RedisStatus(status), responseSize)
}
