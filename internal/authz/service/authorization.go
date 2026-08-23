// Package service 适配 Envoy External Authorization 协议
package service

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	authv3 "github.com/envoyproxy/go-control-plane/envoy/service/auth/v3"
	typev3 "github.com/envoyproxy/go-control-plane/envoy/type/v3"
	"google.golang.org/genproto/googleapis/rpc/code"
	statuspb "google.golang.org/genproto/googleapis/rpc/status"
	"google.golang.org/protobuf/types/known/structpb"

	"github.com/lgc202/ingate/internal/authz/biz"
	"github.com/lgc202/ingate/internal/authz/biz/ratelimit"
	"github.com/lgc202/ingate/internal/pkg/extauthz"
)

const bearerPrefix = "Bearer "

// AuthorizationService 把 Envoy 鉴权请求转换为 Ingate Caller 授权决策
type AuthorizationService struct {
	authv3.UnimplementedAuthorizationServer
	authorizer *biz.Authorizer
	rateLimits *ratelimit.Service
	counters   authorizationCounters
}

type authorizationCounters struct {
	checks      atomic.Uint64
	allowed     atomic.Uint64
	denied      atomic.Uint64
	rateLimited atomic.Uint64
	errors      atomic.Uint64
}

// Counters 是 Authz 运维指标使用的并发安全计数快照
type Counters struct {
	Checks      uint64
	Allowed     uint64
	Denied      uint64
	RateLimited uint64
	Errors      uint64
}

// NewAuthorizationService 创建 External Authorization 协议服务
func NewAuthorizationService(authorizer *biz.Authorizer, rateLimits *ratelimit.Service) *AuthorizationService {
	return &AuthorizationService{authorizer: authorizer, rateLimits: rateLimits}
}

// Check 在 Envoy 转发请求前完成 Caller 授权和共享请求限流
func (s *AuthorizationService) Check(ctx context.Context, request *authv3.CheckRequest) (*authv3.CheckResponse, error) {
	s.counters.checks.Add(1)
	httpRequest := request.GetAttributes().GetRequest().GetHttp()
	contextExtensions := request.GetAttributes().GetContextExtensions()
	callerRequired := contextExtensions[extauthz.CallerRequiredContext] == "true"
	var identity biz.Identity
	if callerRequired {
		routeID := contextExtensions[extauthz.RouteIDContext]
		credential := bearerCredential(httpRequest.GetHeaders()["authorization"])
		var err error
		identity, err = s.authorizer.Authorize(credential, routeID)
		if errors.Is(err, biz.ErrForbidden) {
			s.counters.denied.Add(1)
			response := denied(typev3.StatusCode_Forbidden, code.Code_PERMISSION_DENIED, "forbidden", "Caller is not authorized for this route.")
			response.DynamicMetadata = identityMetadata(identity)
			return response, nil
		}
		if err != nil {
			s.counters.denied.Add(1)
			return denied(typev3.StatusCode_Unauthorized, code.Code_UNAUTHENTICATED, "unauthenticated", "Access key is missing or invalid."), nil
		}
	}

	rules, err := rateLimitRules(contextExtensions[extauthz.RateLimitsContext])
	if err != nil {
		s.counters.errors.Add(1)
		return nil, err
	}
	if len(rules) > 0 {
		exceeded, err := s.rateLimits.Admit(ctx, rules, ratelimit.Request{
			ClientIP: request.GetAttributes().GetSource().GetAddress().GetSocketAddress().GetAddress(),
			Headers:  httpRequest.GetHeaders(),
		}, time.Now())
		if err != nil {
			s.counters.errors.Add(1)
			return nil, fmt.Errorf("enforce request rate limit: %w", err)
		}
		if exceeded != nil {
			s.counters.rateLimited.Add(1)
			response := rateLimited(exceeded.RetryAfter)
			response.DynamicMetadata = identityMetadata(identity)
			return response, nil
		}
	}

	headersToRemove := []string(nil)
	if callerRequired {
		// Caller 密钥由网关消费；公开 Route 上的 Authorization 则属于上游业务，必须原样保留
		headersToRemove = []string{"authorization"}
	}
	s.counters.allowed.Add(1)
	return &authv3.CheckResponse{
		Status: &statuspb.Status{Code: int32(code.Code_OK)},
		HttpResponse: &authv3.CheckResponse_OkResponse{OkResponse: &authv3.OkHttpResponse{
			HeadersToRemove: headersToRemove,
		}},
		DynamicMetadata: identityMetadata(identity),
	}, nil
}

// Counters 返回鉴权与请求限流结果的累计计数
func (s *AuthorizationService) Counters() Counters {
	return Counters{
		Checks:      s.counters.checks.Load(),
		Allowed:     s.counters.allowed.Load(),
		Denied:      s.counters.denied.Load(),
		RateLimited: s.counters.rateLimited.Load(),
		Errors:      s.counters.errors.Load(),
	}
}

func identityMetadata(identity biz.Identity) *structpb.Struct {
	if identity.CallerID == "" && identity.AccessKeyID == "" {
		return nil
	}
	return &structpb.Struct{Fields: map[string]*structpb.Value{
		extauthz.CallerIDField:    structpb.NewStringValue(identity.CallerID),
		extauthz.AccessKeyIDField: structpb.NewStringValue(identity.AccessKeyID),
	}}
}

func rateLimitRules(encoded string) ([]ratelimit.Rule, error) {
	compiled, err := extauthz.DecodeRateLimitRules(encoded)
	if err != nil {
		return nil, err
	}
	rules := make([]ratelimit.Rule, 0, len(compiled))
	for _, rule := range compiled {
		rules = append(rules, ratelimit.Rule{
			PolicyID:      rule.PolicyID,
			Scope:         rule.Scope,
			Subject:       ratelimit.Subject(rule.Subject),
			HeaderName:    rule.HeaderName,
			Requests:      rule.Requests,
			WindowSeconds: rule.WindowSeconds,
		})
	}
	return rules, nil
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

func rateLimited(retryAfter time.Duration) *authv3.CheckResponse {
	seconds := int64((retryAfter + time.Second - 1) / time.Second)
	response := denied(
		typev3.StatusCode_TooManyRequests,
		code.Code_RESOURCE_EXHAUSTED,
		"rate_limit_exceeded",
		"Request rate limit exceeded.",
	)
	response.GetDeniedResponse().Headers = append(response.GetDeniedResponse().Headers, &corev3.HeaderValueOption{
		Header: &corev3.HeaderValue{Key: "retry-after", Value: strconv.FormatInt(seconds, 10)},
	})
	return response
}
