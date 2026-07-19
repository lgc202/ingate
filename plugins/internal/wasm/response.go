package wasm

import "github.com/proxy-wasm/proxy-wasm-go-sdk/proxywasm"

// ResponseHeader 读取 HTTP 响应 header，读取失败时返回空字符串
func ResponseHeader(name string) string {
	value, err := proxywasm.GetHttpResponseHeader(name)
	if err != nil {
		return ""
	}
	return value
}

// ReplaceResponseHeaders 覆盖 HTTP 响应 header
func ReplaceResponseHeaders(headers map[string]string) {
	for name, value := range headers {
		_ = proxywasm.ReplaceHttpResponseHeader(name, value)
	}
}

// SendResponse 直接返回 HTTP 响应并终止当前请求
func SendResponse(statusCode int, headers map[string]string, body string) {
	_ = proxywasm.SendHttpResponse(
		uint32(statusCode),
		headerPairs(headers),
		[]byte(body),
		-1,
	)
}

func headerPairs(headers map[string]string) [][2]string {
	if len(headers) == 0 {
		return nil
	}
	result := make([][2]string, 0, len(headers))
	for name, value := range headers {
		result = append(result, [2]string{name, value})
	}
	return result
}
