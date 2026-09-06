package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	authv3 "github.com/envoyproxy/go-control-plane/envoy/service/auth/v3"
	typev3 "github.com/envoyproxy/go-control-plane/envoy/type/v3"
	"github.com/samber/lo"
	"google.golang.org/genproto/googleapis/rpc/code"
	statuspb "google.golang.org/genproto/googleapis/rpc/status"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/structpb"

	"github.com/lgc202/ingate/internal/authz/biz"
	"github.com/lgc202/ingate/internal/authz/biz/ratelimit"
	apivalidation "github.com/lgc202/ingate/internal/pkg/apis/gateway/validation"
	"github.com/lgc202/ingate/internal/pkg/extauthz"
)

const bearerPrefix = "Bearer "

type authorizationCounters struct {
	checks      atomic.Uint64
	allowed     atomic.Uint64
	denied      atomic.Uint64
	rateLimited atomic.Uint64
	failed      atomic.Uint64
}

// Counters 是 Authz 运维指标使用的并发安全计数快照。
type Counters struct {
	Checks      uint64
	Allowed     uint64
	Denied      uint64
	RateLimited uint64
	Failed      uint64
}

type checkInput struct {
	headers        map[string]string
	clientIP       string
	routeID        string
	callerRequired bool
	rateLimitRules []ratelimit.Rule
}

// AuthorizationService 把 Envoy 鉴权请求转换为 Ingate Caller 授权决策。
type AuthorizationService struct {
	authv3.UnimplementedAuthorizationServer
	authorizer  *biz.Authorizer
	rateLimiter *ratelimit.Limiter
	logger      *slog.Logger
	counters    authorizationCounters
}

// NewAuthorizationService 创建 External Authorization 协议服务。
func NewAuthorizationService(
	authorizer *biz.Authorizer,
	rateLimiter *ratelimit.Limiter,
	logger *slog.Logger,
) *AuthorizationService {
	return &AuthorizationService{
		authorizer:  authorizer,
		rateLimiter: rateLimiter,
		logger:      logger,
	}
}

// Check 在 Envoy 转发请求前完成 Caller 授权和共享请求限流。
func (s *AuthorizationService) Check(ctx context.Context, request *authv3.CheckRequest) (*authv3.CheckResponse, error) {
	s.counters.checks.Add(1)
	input, err := parseCheckInput(request)
	if err != nil {
		s.counters.failed.Add(1)
		return nil, err
	}

	identity, response := s.checkCaller(input)
	if response != nil {
		return response, nil
	}
	response, err = s.checkRateLimit(ctx, input, identity)
	if response != nil || err != nil {
		return response, err
	}

	s.counters.allowed.Add(1)
	return allowedResponse(identity, input.callerRequired), nil
}

// Counters 返回鉴权与请求限流结果的累计计数。
func (s *AuthorizationService) Counters() Counters {
	return Counters{
		Checks:      s.counters.checks.Load(),
		Allowed:     s.counters.allowed.Load(),
		Denied:      s.counters.denied.Load(),
		RateLimited: s.counters.rateLimited.Load(),
		Failed:      s.counters.failed.Load(),
	}
}

func (s *AuthorizationService) checkCaller(input checkInput) (biz.Identity, *authv3.CheckResponse) {
	if !input.callerRequired {
		return biz.Identity{}, nil
	}

	identity, err := s.authorizer.Authorize(bearerCredential(input.headers["authorization"]), input.routeID)
	if err == nil {
		return identity, nil
	}

	s.counters.denied.Add(1)
	if errors.Is(err, biz.ErrForbidden) {
		response := deniedResponse(
			typev3.StatusCode_Forbidden,
			code.Code_PERMISSION_DENIED,
			"forbidden",
			"Caller is not authorized for this route.",
		)
		response.DynamicMetadata = identityMetadata(identity)
		return biz.Identity{}, response
	}
	return biz.Identity{}, deniedResponse(
		typev3.StatusCode_Unauthorized,
		code.Code_UNAUTHENTICATED,
		"unauthenticated",
		"Access key is missing or invalid.",
	)
}

func (s *AuthorizationService) checkRateLimit(
	ctx context.Context,
	input checkInput,
	identity biz.Identity,
) (*authv3.CheckResponse, error) {
	if len(input.rateLimitRules) == 0 {
		return nil, nil
	}

	rejection, err := s.rateLimiter.Admit(ctx, input.rateLimitRules, ratelimit.Request{
		ClientIP: input.clientIP,
		Headers:  input.headers,
	})
	if err != nil {
		s.counters.failed.Add(1)
		if ctx.Err() != nil {
			return nil, status.FromContextError(ctx.Err()).Err()
		}
		s.logger.ErrorContext(ctx, "enforce request rate limit failed", "err", err)
		return nil, status.Error(codes.Unavailable, "request rate limit is unavailable")
	}
	if rejection == nil {
		return nil, nil
	}

	s.counters.rateLimited.Add(1)
	response := rateLimitedResponse(rejection.RetryAfter)
	response.DynamicMetadata = identityMetadata(identity)
	return response, nil
}

func parseCheckInput(request *authv3.CheckRequest) (checkInput, error) {
	attributes := request.GetAttributes()
	if attributes == nil {
		return checkInput{}, status.Error(codes.InvalidArgument, "authorization request attributes are required")
	}
	httpAttributes := attributes.GetRequest().GetHttp()
	if httpAttributes == nil {
		return checkInput{}, status.Error(codes.InvalidArgument, "authorization HTTP request attributes are required")
	}
	contextExtensions := attributes.GetContextExtensions()
	requiresCaller, err := parseCallerRequirement(contextExtensions)
	if err != nil {
		return checkInput{}, status.Error(codes.FailedPrecondition, "caller authorization context is invalid")
	}
	rateLimitRules, err := parseRateLimitRules(contextExtensions[extauthz.RateLimitsContext])
	if err != nil {
		return checkInput{}, status.Error(codes.FailedPrecondition, "rate limit authorization context is invalid")
	}
	if !requiresCaller && len(rateLimitRules) == 0 {
		return checkInput{}, status.Error(codes.FailedPrecondition, "authorization rule is required")
	}

	routeID := contextExtensions[extauthz.RouteIDContext]
	if requiresCaller {
		if !apivalidation.IsCanonicalID(routeID) {
			return checkInput{}, status.Error(codes.FailedPrecondition, "authorization Route ID is invalid")
		}
	}
	return checkInput{
		headers:        httpAttributes.GetHeaders(),
		clientIP:       attributes.GetSource().GetAddress().GetSocketAddress().GetAddress(),
		routeID:        routeID,
		callerRequired: requiresCaller,
		rateLimitRules: rateLimitRules,
	}, nil
}

func allowedResponse(identity biz.Identity, callerRequired bool) *authv3.CheckResponse {
	var headersToRemove []string
	if callerRequired {
		// Caller 密钥由网关消费；公开 Route 上的 Authorization 则属于上游业务，必须原样保留。
		headersToRemove = []string{"authorization"}
	}
	return &authv3.CheckResponse{
		Status: &statuspb.Status{Code: int32(code.Code_OK)},
		HttpResponse: &authv3.CheckResponse_OkResponse{OkResponse: &authv3.OkHttpResponse{
			HeadersToRemove: headersToRemove,
		}},
		DynamicMetadata: identityMetadata(identity),
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

func parseCallerRequirement(extensions map[string]string) (bool, error) {
	value, exists := extensions[extauthz.CallerRequiredContext]
	if !exists {
		return false, nil
	}
	if value != "true" {
		return false, fmt.Errorf("unsupported caller requirement %q", value)
	}
	return true, nil
}

func parseRateLimitRules(encoded string) ([]ratelimit.Rule, error) {
	compiled, err := extauthz.DecodeRateLimitRules(encoded)
	if err != nil {
		return nil, err
	}
	return lo.Map(compiled, func(rule extauthz.RateLimitRule, _ int) ratelimit.Rule {
		return ratelimit.Rule{
			PolicyID:      rule.PolicyID,
			Scope:         rule.Scope,
			Subject:       ratelimit.Subject(rule.Subject),
			HeaderName:    rule.HeaderName,
			Requests:      rule.Requests,
			WindowSeconds: rule.WindowSeconds,
		}
	}), nil
}

func bearerCredential(headerValue string) string {
	headerValue = strings.TrimSpace(headerValue)
	if len(headerValue) <= len(bearerPrefix) ||
		!strings.EqualFold(headerValue[:len(bearerPrefix)], bearerPrefix) {
		return ""
	}
	return strings.TrimSpace(headerValue[len(bearerPrefix):])
}

func deniedResponse(httpCode typev3.StatusCode, rpcCode code.Code, errorCode, message string) *authv3.CheckResponse {
	headers := []*corev3.HeaderValueOption{{
		Header:       &corev3.HeaderValue{Key: "content-type", RawValue: []byte("application/json")},
		AppendAction: corev3.HeaderValueOption_OVERWRITE_IF_EXISTS_OR_ADD,
	}}
	if httpCode == typev3.StatusCode_Unauthorized {
		headers = append(headers, &corev3.HeaderValueOption{
			Header:       &corev3.HeaderValue{Key: "www-authenticate", RawValue: []byte("Bearer")},
			AppendAction: corev3.HeaderValueOption_OVERWRITE_IF_EXISTS_OR_ADD,
		})
	}
	return &authv3.CheckResponse{
		Status: &statuspb.Status{Code: int32(rpcCode), Message: message},
		HttpResponse: &authv3.CheckResponse_DeniedResponse{DeniedResponse: &authv3.DeniedHttpResponse{
			Status:  &typev3.HttpStatus{Code: httpCode},
			Headers: headers,
			Body: `{"error":{"code":` + strconv.Quote(errorCode) +
				`,"message":` + strconv.Quote(message) + `}}`,
		}},
	}
}

func rateLimitedResponse(retryAfter time.Duration) *authv3.CheckResponse {
	seconds := int64(retryAfter / time.Second)
	if retryAfter%time.Second != 0 {
		seconds++
	}
	seconds = max(1, seconds)
	response := deniedResponse(
		typev3.StatusCode_TooManyRequests,
		code.Code_RESOURCE_EXHAUSTED,
		"rate_limit_exceeded",
		"Request rate limit exceeded.",
	)
	response.GetDeniedResponse().Headers = append(response.GetDeniedResponse().Headers, &corev3.HeaderValueOption{
		Header: &corev3.HeaderValue{
			Key:      "retry-after",
			RawValue: []byte(strconv.FormatInt(seconds, 10)),
		},
		AppendAction: corev3.HeaderValueOption_OVERWRITE_IF_EXISTS_OR_ADD,
	})
	return response
}
