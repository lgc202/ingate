package extproc

import (
	"errors"
	"fmt"
	"strings"

	"github.com/lgc202/ingate/internal/aiproxy/routeconfig"
	"github.com/lgc202/ingate/pkg/llm"
	"github.com/lgc202/ingate/pkg/llm/anthropic"
	"github.com/lgc202/ingate/pkg/llm/gemini"
	"github.com/lgc202/ingate/pkg/llm/openai"
	"github.com/lgc202/ingate/pkg/llm/sse"
)

const chatCompletionsPath = "/v1/chat/completions"

type modelProxy struct {
	requireUsage bool
	upstreams    map[string]routeconfig.Upstream
	models       map[string]routeconfig.Model
}

type preparedRequest struct {
	body      []byte
	path      string
	cluster   string
	authority string
	headers   []routeconfig.Header
	response  responseTransform
}

type responseTransform struct {
	protocol    llm.Protocol
	publicModel string
	stream      bool
}

type localResponse struct {
	statusCode int
	headers    [][2]string
	body       []byte
}

type responseStream interface {
	Push([]byte) ([]byte, error)
	Finish() ([]byte, error)
}

func newModelProxy(config routeconfig.Config) *modelProxy {
	upstreams := make(map[string]routeconfig.Upstream, len(config.Upstreams))
	for _, upstream := range config.Upstreams {
		upstreams[upstream.ID] = upstream
	}
	models := make(map[string]routeconfig.Model, len(config.Models))
	for _, model := range config.Models {
		models[model.Model] = model
	}
	return &modelProxy{
		requireUsage: config.RequireUsage,
		upstreams:    upstreams,
		models:       models,
	}
}

func (p *modelProxy) validateEndpoint(method, path string) *localResponse {
	if method != "POST" {
		response := rejectionResponse(405, "Only POST is supported for this endpoint", "invalid_request_error", "method_not_allowed", nil)
		response.headers = append(response.headers, [2]string{"allow", "POST"})
		return &response
	}
	requestPath, _, _ := strings.Cut(path, "?")
	if requestPath != chatCompletionsPath {
		response := rejectionResponse(404, "The requested endpoint is not supported", "invalid_request_error", "endpoint_not_found", nil)
		return &response
	}
	return nil
}

func (p *modelProxy) prepareRequest(body []byte, allowsModel func(string) bool) (preparedRequest, *localResponse) {
	request, err := openai.DecodeRequest(body)
	if err != nil {
		code := "invalid_request"
		if errors.Is(err, llm.ErrUnsupportedFeature) {
			code = "unsupported_feature"
		}
		response := rejectionResponse(400, err.Error(), "invalid_request_error", code, nil)
		return preparedRequest{}, &response
	}
	if !allowsModel(request.Model) {
		parameter := "model"
		response := rejectionResponse(
			403,
			fmt.Sprintf("The API key is not allowed to access model %q", request.Model),
			"permission_error",
			"model_not_allowed",
			&parameter,
		)
		return preparedRequest{}, &response
	}
	model, exists := p.models[request.Model]
	if !exists {
		parameter := "model"
		response := rejectionResponse(
			404,
			fmt.Sprintf("The model %q does not exist or is not available for this route", request.Model),
			"invalid_request_error",
			"model_not_found",
			&parameter,
		)
		return preparedRequest{}, &response
	}
	upstream, exists := p.upstreams[model.UpstreamID]
	if !exists {
		response := internalErrorResponse()
		return preparedRequest{}, &response
	}

	transformedBody, upstreamPath, err := transformRequest(upstream, model.UpstreamModel, request, p.requireUsage)
	if err != nil {
		if errors.Is(err, llm.ErrInvalidRequest) || errors.Is(err, llm.ErrUnsupportedFeature) {
			response := rejectionResponse(400, err.Error(), "invalid_request_error", "unsupported_request", nil)
			return preparedRequest{}, &response
		}
		response := internalErrorResponse()
		return preparedRequest{}, &response
	}
	headers := make([]routeconfig.Header, 0, len(upstream.Headers)+1)
	headers = append(headers, upstream.Headers...)
	if upstream.APIKey != "" {
		headers = append(headers, routeconfig.Header{
			Name:  upstream.APIKeyHeader,
			Value: upstream.APIKeyPrefix + upstream.APIKey,
		})
	}
	return preparedRequest{
		body:      transformedBody,
		path:      upstreamPath,
		cluster:   upstream.Cluster,
		authority: upstream.Authority,
		headers:   headers,
		response: responseTransform{
			protocol:    upstream.Protocol,
			publicModel: model.Model,
			stream:      request.Streaming(),
		},
	}, nil
}

func (p *modelProxy) transformResponse(transform responseTransform, statusCode int, body []byte) ([]byte, error) {
	if statusCode >= 400 {
		switch transform.protocol {
		case llm.ProtocolOpenAIChatCompletions:
			return openai.TransformError(body, statusCode), nil
		case llm.ProtocolAnthropicMessages:
			return anthropic.TransformError(body, statusCode), nil
		case llm.ProtocolGeminiGenerateContent:
			return gemini.TransformError(body, statusCode), nil
		default:
			return nil, fmt.Errorf("unsupported response protocol %q", transform.protocol)
		}
	}
	switch transform.protocol {
	case llm.ProtocolOpenAIChatCompletions:
		return openai.TransformResponse(body, transform.publicModel)
	case llm.ProtocolAnthropicMessages:
		return anthropic.TransformResponse(body, transform.publicModel)
	case llm.ProtocolGeminiGenerateContent:
		return gemini.TransformResponse(body, transform.publicModel)
	default:
		return nil, fmt.Errorf("unsupported response protocol %q", transform.protocol)
	}
}

func (p *modelProxy) newResponseStream(transform responseTransform) (responseStream, error) {
	switch transform.protocol {
	case llm.ProtocolOpenAIChatCompletions:
		return openai.NewStream(transform.publicModel)
	case llm.ProtocolAnthropicMessages:
		return anthropic.NewStream(transform.publicModel)
	case llm.ProtocolGeminiGenerateContent:
		return gemini.NewStream(transform.publicModel)
	default:
		return nil, fmt.Errorf("unsupported response protocol %q", transform.protocol)
	}
}

func transformRequest(
	upstream routeconfig.Upstream,
	upstreamModel string,
	request openai.Request,
	requireUsage bool,
) ([]byte, string, error) {
	var (
		transformed []byte
		endpoint    string
		err         error
	)
	switch upstream.Protocol {
	case llm.ProtocolOpenAIChatCompletions:
		if requireUsage && request.Streaming() {
			transformed, err = openai.TransformRequestWithStreamUsage(request, upstreamModel)
		} else {
			transformed, err = openai.TransformRequest(request, upstreamModel)
		}
		endpoint = openai.ChatCompletionsPath
	case llm.ProtocolAnthropicMessages:
		transformed, err = anthropic.TransformRequest(request, upstreamModel)
		endpoint = anthropic.MessagesPath
	case llm.ProtocolGeminiGenerateContent:
		transformed, err = gemini.TransformRequest(request)
		if err == nil {
			endpoint, err = gemini.EndpointPath(upstreamModel, request.Streaming())
		}
	default:
		err = fmt.Errorf("unsupported request protocol %q", upstream.Protocol)
	}
	if err != nil {
		return nil, "", err
	}
	return transformed, joinPath(upstream.BasePath, endpoint), nil
}

func joinPath(basePath, endpoint string) string {
	if basePath == "/" {
		return endpoint
	}
	return strings.TrimSuffix(basePath, "/") + endpoint
}

func rejectionResponse(statusCode int, message, errorType, code string, parameter *string) localResponse {
	return localResponse{
		statusCode: statusCode,
		headers:    [][2]string{{"content-type", jsonContentType}},
		body: openai.EncodeError(openai.ErrorDetail{
			Message: message,
			Type:    errorType,
			Param:   parameter,
			Code:    code,
		}),
	}
}

func internalErrorResponse() localResponse {
	return rejectionResponse(500, "The request could not be processed", "server_error", "internal_error", nil)
}

func responseErrorBody() []byte {
	return openai.EncodeError(openai.DefaultError(502, "The upstream response could not be processed"))
}

func streamErrorBody() []byte {
	detail := openai.DefaultError(502, "The upstream stream could not be processed")
	body := sse.EncodeData(openai.EncodeError(detail))
	return append(body, sse.EncodeData([]byte("[DONE]"))...)
}
