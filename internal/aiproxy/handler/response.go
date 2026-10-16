package handler

import (
	"context"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"slices"
	"strings"
)

// streamWriter 在每次正文写入后刷新，不解析或改写 SSE 事件。
type streamWriter struct {
	response   http.ResponseWriter
	controller *http.ResponseController
}

// writeUpstreamResponse 接管响应 Body，只负责 HTTP 写回，不重新选路或发起网络请求。
// 正文及 Flush 失败后响应已经开始，必须中断连接，不能返回给 Kratos 追加错误 JSON。
func writeUpstreamResponse(
	ctx context.Context,
	response http.ResponseWriter,
	upstream *http.Response,
	logger *slog.Logger,
) {
	defer func() {
		if err := upstream.Body.Close(); err != nil && ctx.Err() == nil {
			logger.WarnContext(ctx, "Failed to close upstream response body", "err", err)
		}
	}()

	headers := endToEndHeaders(upstream.Header)
	for name, values := range headers {
		for _, value := range values {
			response.Header().Add(name, value)
		}
	}

	trailers := endToEndHeaders(upstream.Trailer)
	for name := range trailers {
		response.Header().Add("Trailer", name)
	}

	response.WriteHeader(upstream.StatusCode)

	controller := http.NewResponseController(response)
	var destination io.Writer = response
	mediaType, _, mediaErr := mime.ParseMediaType(upstream.Header.Get("Content-Type"))
	if (mediaErr == nil && strings.EqualFold(mediaType, "text/event-stream")) || upstream.ContentLength < 0 {
		// 先交付响应头，即使上游尚未发送第一个事件也不会等待整条流结束。
		if err := controller.Flush(); err != nil {
			abortResponse(ctx, err, logger)
		}

		destination = streamWriter{response: response, controller: controller}
	}

	if _, err := io.Copy(destination, upstream.Body); err != nil {
		abortResponse(ctx, err, logger)
	}

	// Trailer 在读取到 EOF 后才完整；未提前声明的字段使用原生 TrailerPrefix。
	for name, values := range endToEndHeaders(upstream.Trailer) {
		key := name
		if _, announced := trailers[name]; !announced {
			key = http.TrailerPrefix + name
		}

		response.Header()[key] = slices.Clone(values)
	}
}

func (w streamWriter) Write(data []byte) (int, error) {
	n, err := w.response.Write(data)
	if err != nil {
		return n, err
	}

	return n, w.controller.Flush()
}

// endToEndHeaders 只复制端到端响应字段，不修改 Client 返回的 Header。
func endToEndHeaders(source http.Header) http.Header {
	headers := source.Clone()
	for _, connection := range source.Values("Connection") {
		for name := range strings.SplitSeq(connection, ",") {
			headers.Del(strings.TrimSpace(name))
		}
	}

	for _, name := range []string{
		"Connection", "Proxy-Connection", "Keep-Alive", "Proxy-Authenticate",
		"Proxy-Authorization", "Te", "Trailer", "Transfer-Encoding", "Upgrade",
	} {
		headers.Del(name)
	}

	return headers
}

func abortResponse(ctx context.Context, err error, logger *slog.Logger) {
	if ctx.Err() == nil {
		logger.ErrorContext(ctx, "Failed to write upstream response", "err", err)
	}

	// ErrAbortHandler 是 net/http 的连接/HTTP2 流中断约定，不是业务 panic 恢复机制。
	panic(http.ErrAbortHandler)
}
