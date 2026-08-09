package extproc

import (
	"strconv"

	extprochttpv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/ext_proc/v3"
	extprocv3 "github.com/envoyproxy/go-control-plane/envoy/service/ext_proc/v3"

	"github.com/lgc202/ingate/internal/pkg/aiproxyconfig"
)

func (s *Server) processResponseHeaders(
	state *requestState,
	headers *extprocv3.HttpHeaders,
) (*extprocv3.ProcessingResponse, bool, error) {
	state.responseStarted = true
	statusCode, err := strconv.Atoi(headerValue(headers.GetHeaders(), ":status"))
	if err != nil {
		s.logger.Error("parse AI upstream response status failed", "err", err)
		statusCode = 502
	}
	state.responseStatus = statusCode

	streaming := state.responseTransform.stream && statusCode < 400
	contentType := jsonContentType
	bodyMode := extprochttpv3.ProcessingMode_BUFFERED
	if streaming {
		contentType = sseContentType
		bodyMode = extprochttpv3.ProcessingMode_STREAMED
		state.responseStream, err = state.proxy.newResponseStream(state.responseTransform)
		if err != nil {
			s.logger.Error("create AI response stream failed", "err", err)
			return replaceResponseHeaders(jsonContentType, responseErrorBody(), 502), true, nil
		}
	}

	if !headers.GetEndOfStream() {
		return responseHeadersResponse(contentType, bodyMode), false, nil
	}
	if streaming {
		body, finishErr := state.responseStream.Finish()
		if finishErr != nil {
			s.logger.Error("finish empty AI response stream failed", "err", finishErr)
			body = streamErrorBody()
		}
		return replaceResponseHeaders(contentType, body, 0), true, nil
	}
	body, transformErr := state.proxy.transformResponse(state.responseTransform, statusCode, nil)
	if transformErr != nil {
		s.logger.Error("transform empty AI response failed", "err", transformErr)
		return replaceResponseHeaders(jsonContentType, responseErrorBody(), 502), true, nil
	}
	return replaceResponseHeaders(contentType, body, 0), true, nil
}

func (s *Server) processResponseBody(
	state *requestState,
	body *extprocv3.HttpBody,
) (*extprocv3.ProcessingResponse, bool, error) {
	if state.responseTransform.stream && state.responseStatus < 400 {
		return s.processStreamingResponse(state, body)
	}
	if len(body.GetBody()) > aiproxyconfig.MaxResponseBodyBytes {
		s.logger.Error("AI upstream response body is too large", "bytes", len(body.GetBody()))
		return replaceResponseBody(502, responseErrorBody()), body.GetEndOfStream(), nil
	}
	transformed, err := state.proxy.transformResponse(state.responseTransform, state.responseStatus, body.GetBody())
	if err != nil {
		s.logger.Error("transform AI upstream response failed", "err", err)
		return replaceResponseBody(502, responseErrorBody()), body.GetEndOfStream(), nil
	}
	return replaceResponseBody(0, transformed), body.GetEndOfStream(), nil
}

func (s *Server) processStreamingResponse(
	state *requestState,
	body *extprocv3.HttpBody,
) (*extprocv3.ProcessingResponse, bool, error) {
	if state.responseClosed {
		return replaceResponseBody(0, nil), body.GetEndOfStream(), nil
	}
	transformed, err := state.responseStream.Push(body.GetBody())
	if err != nil {
		s.logger.Error("transform AI upstream stream failed", "err", err)
		state.responseClosed = true
		return replaceResponseBody(0, streamErrorBody()), body.GetEndOfStream(), nil
	}
	if body.GetEndOfStream() {
		tail, finishErr := state.responseStream.Finish()
		if finishErr != nil {
			s.logger.Error("finish AI upstream stream failed", "err", finishErr)
			tail = streamErrorBody()
		}
		transformed = append(transformed, tail...)
	}
	return replaceResponseBody(0, transformed), body.GetEndOfStream(), nil
}
