package server

import (
	"net/http"

	"github.com/lgc202/ingate/internal/pkg/requestid"
)

// maxRequestBodyBytes 覆盖现有资源和证书配置，同时限制恶意请求造成的内存占用。
const maxRequestBodyBytes int64 = 4 << 20

func requestIDFilter(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		id := requestid.GetOrCreate(request.Header)
		writer.Header().Set(requestid.Header, id)
		next.ServeHTTP(writer, request)
	})
}

func requestBodyLimitFilter(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		request.Body = http.MaxBytesReader(writer, request.Body, maxRequestBodyBytes)
		next.ServeHTTP(writer, request)
	})
}
