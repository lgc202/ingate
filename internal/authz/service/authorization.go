// Package service 适配 Envoy External Authorization 协议
package service

import (
	"context"
	"errors"
	"strings"

	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	authv3 "github.com/envoyproxy/go-control-plane/envoy/service/auth/v3"
	typev3 "github.com/envoyproxy/go-control-plane/envoy/type/v3"
	"google.golang.org/genproto/googleapis/rpc/code"
	statuspb "google.golang.org/genproto/googleapis/rpc/status"
	"google.golang.org/protobuf/types/known/structpb"

	"github.com/lgc202/ingate/internal/authz/biz"
	"github.com/lgc202/ingate/internal/pkg/extauthz"
)

const bearerPrefix = "Bearer "

// AuthorizationService 把 Envoy 鉴权请求转换为 Ingate Caller 授权决策
type AuthorizationService struct {
	authv3.UnimplementedAuthorizationServer
	authorizer *biz.Authorizer
}

// NewAuthorizationService 创建 External Authorization 协议服务
func NewAuthorizationService(authorizer *biz.Authorizer) *AuthorizationService {
	return &AuthorizationService{authorizer: authorizer}
}

// Check 在 Envoy 转发请求前完成 Caller 身份和 Route 权限校验
func (s *AuthorizationService) Check(_ context.Context, request *authv3.CheckRequest) (*authv3.CheckResponse, error) {
	httpRequest := request.GetAttributes().GetRequest().GetHttp()
	routeID := request.GetAttributes().GetContextExtensions()[extauthz.RouteIDContext]
	credential := bearerCredential(httpRequest.GetHeaders()["authorization"])
	identity, err := s.authorizer.Authorize(credential, routeID)
	if errors.Is(err, biz.ErrForbidden) {
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

func identityMetadata(identity biz.Identity) *structpb.Struct {
	return &structpb.Struct{Fields: map[string]*structpb.Value{
		extauthz.CallerIDField:    structpb.NewStringValue(identity.CallerID),
		extauthz.AccessKeyIDField: structpb.NewStringValue(identity.AccessKeyID),
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
