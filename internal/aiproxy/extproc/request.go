package extproc

import (
	"context"
	"strings"

	extprocv3 "github.com/envoyproxy/go-control-plane/envoy/service/ext_proc/v3"

	"github.com/lgc202/ingate/internal/pkg/aiproxyconfig"
	"github.com/lgc202/ingate/internal/pkg/bearer"
)

func (s *Server) processRequestHeaders(
	ctx context.Context,
	state *requestState,
	headers *extprocv3.HttpHeaders,
) (*extprocv3.ProcessingResponse, bool, error) {
	if state.configErr != nil {
		s.logger.Error("decode AI route config failed", "err", state.configErr)
		return immediateResponse(internalErrorResponse()), true, nil
	}
	state.proxy = newModelProxy(state.config)
	if response := state.proxy.validateEndpoint(
		headerValue(headers.GetHeaders(), ":method"),
		headerValue(headers.GetHeaders(), ":path"),
	); response != nil {
		return immediateResponse(*response), true, nil
	}

	secret, ok := bearerSecret(headerValue(headers.GetHeaders(), authorizationHeader))
	if !ok {
		return immediateResponse(unauthorizedLocalResponse()), true, nil
	}
	currentGrant, authorized, err := s.authenticator.Authenticate(ctx, secret)
	if err != nil {
		s.logger.Error("authenticate AI access key failed", "err", err)
		return immediateResponse(authenticationUnavailableLocalResponse()), true, nil
	}
	if !authorized {
		return immediateResponse(unauthorizedLocalResponse()), true, nil
	}
	if headers.GetEndOfStream() {
		response := rejectionResponse(400, "Request body is required", "invalid_request_error", "invalid_request", nil)
		return immediateResponse(response), true, nil
	}
	state.grant = currentGrant
	return requestHeadersResponse(), false, nil
}

func (s *Server) processRequestBody(
	state *requestState,
	body *extprocv3.HttpBody,
) (*extprocv3.ProcessingResponse, bool, error) {
	if len(body.GetBody()) > aiproxyconfig.MaxRequestBodyBytes {
		response := rejectionResponse(413, "Request body is too large", "invalid_request_error", "request_too_large", nil)
		return immediateResponse(response), true, nil
	}
	prepared, response := state.proxy.prepareRequest(body.GetBody(), state.grant.Allows)
	if response != nil {
		return immediateResponse(*response), true, nil
	}
	state.requestPrepared = true
	state.responseTransform = prepared.response
	return preparedRequestResponse(prepared), false, nil
}

func bearerSecret(authorization string) (string, bool) {
	parts := strings.Fields(authorization)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || !bearer.ValidToken(parts[1]) {
		return "", false
	}
	return parts[1], true
}
