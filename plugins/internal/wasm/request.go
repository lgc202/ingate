package wasm

import "github.com/proxy-wasm/proxy-wasm-go-sdk/proxywasm"

// RequestHeader 读取 HTTP 请求 header，读取失败时返回空字符串
func RequestHeader(name string) string {
	value, err := proxywasm.GetHttpRequestHeader(name)
	if err != nil {
		return ""
	}
	return value
}

// RequestHeaders 读取一组 HTTP 请求 header，忽略读取失败和空值
func RequestHeaders(names []string) map[string]string {
	headers := make(map[string]string)
	for _, name := range names {
		value := RequestHeader(name)
		if value != "" {
			headers[name] = value
		}
	}
	return headers
}

// SourceAddress 读取当前请求的源地址
func SourceAddress() string {
	value, err := proxywasm.GetProperty([]string{"source", "address"})
	if err != nil {
		return ""
	}
	return string(value)
}
