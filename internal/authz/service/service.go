// Package service 实现 Envoy External Authorization 协议
package service

import (
	"context"
	"errors"
	"strings"
	"time"

	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	authv3 "github.com/envoyproxy/go-control-plane/envoy/service/auth/v3"
	typev3 "github.com/envoyproxy/go-control-plane/envoy/type/v3"
	"google.golang.org/genproto/googleapis/rpc/code"
	statuspb "google.golang.org/genproto/googleapis/rpc/status"
	"google.golang.org/protobuf/types/known/structpb"

	"github.com/lgc202/ingate/internal/authz/caller"
	"github.com/lgc202/ingate/internal/authz/filterconfig"
)

const bearerPrefix = "Bearer "

// Service 验证 Caller 访问密钥和 Route 权限
type Service struct {
	authv3.UnimplementedAuthorizationServer
	callers *caller.Index
}

// NewService 创建 External Authorization 服务
func NewService(callers *caller.Index) *Service {
	return &Service{callers: callers}
}

// Check 在 Envoy 转发请求前完成 Caller 身份和 Route 权限校验
func (s *Service) Check(_ context.Context, request *authv3.CheckRequest) (*authv3.CheckResponse, error) {
	httpRequest := request.GetAttributes().GetRequest().GetHttp()
	routeID := request.GetAttributes().GetContextExtensions()[filterconfig.RouteIDContext]
	credential := bearerCredential(httpRequest.GetHeaders()["authorization"])
	identity, err := s.callers.Authorize(credential, routeID, time.Now().UTC())
	if errors.Is(err, caller.ErrForbidden) {
		response := denied(typev3.StatusCode_Forbidden, code.Code_PERMISSION_DENIED, "forbidden", "Caller is not authorized for this route.")
		response.DynamicMetadata = identityMetadata(identity)
		return response, nil
	}
	if err != nil {
		return denied(typev3.StatusCode_Unauthorized, code.Code_UNAUTHENTICATED, "unauthenticated", "Access key is missing or invalid."), nil
	}

	return &authv3.CheckResponse{
		Status: &statuspb.Status{Code: int32(code.Code_OK)},
		HttpResponse: &authv3.CheckResponse_OkResponse{OkResponse: &authv3.OkHttpResponse{
			// 调用方凭据由网关消费，普通服务和模型厂商都不应收到该密钥
			HeadersToRemove: []string{"authorization"},
		}},
		DynamicMetadata: identityMetadata(identity),
	}, nil
}

func identityMetadata(identity caller.Identity) *structpb.Struct {
	return &structpb.Struct{Fields: map[string]*structpb.Value{
		filterconfig.CallerIDField:    structpb.NewStringValue(identity.CallerID),
		filterconfig.AccessKeyIDField: structpb.NewStringValue(identity.AccessKeyID),
	}}
}

func bearerCredential(value string) string {
	value = strings.TrimSpace(value)
	if len(value) <= len(bearerPrefix) || !strings.EqualFold(value[:len(bearerPrefix)], bearerPrefix) {
		return ""
	}
	return strings.TrimSpace(value[len(bearerPrefix):])
}

func denied(httpStatus typev3.StatusCode, grpcStatus code.Code, errorCode, message string) *authv3.CheckResponse {
	headers := []*corev3.HeaderValueOption{{
		Header: &corev3.HeaderValue{Key: "content-type", Value: "application/json"},
	}}
	if httpStatus == typev3.StatusCode_Unauthorized {
		headers = append(headers, &corev3.HeaderValueOption{
			Header: &corev3.HeaderValue{Key: "www-authenticate", Value: "Bearer"},
		})
	}
	return &authv3.CheckResponse{
		Status: &statuspb.Status{Code: int32(grpcStatus), Message: message},
		HttpResponse: &authv3.CheckResponse_DeniedResponse{DeniedResponse: &authv3.DeniedHttpResponse{
			Status:  &typev3.HttpStatus{Code: httpStatus},
			Headers: headers,
			Body:    `{"error":{"code":"` + errorCode + `","message":"` + message + `"}}`,
		}},
	}
}
