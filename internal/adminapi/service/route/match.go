package route

import (
	"net/http"
	"strings"

	"github.com/go-kratos/kratos/v3/errors"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	resource "github.com/lgc202/ingate/internal/pkg/apis/gateway/v1"
	"github.com/lgc202/ingate/internal/pkg/httpheader"
	"github.com/lgc202/ingate/internal/pkg/routeconfig"
)

func parseRouteMatch(config *adminv1.RouteMatch) (resource.RouteMatch, error) {
	if config == nil || config.GetPath() == nil {
		return resource.RouteMatch{}, errors.BadRequest(
			adminv1.ErrorReason_INVALID_ARGUMENT.String(),
			"必须配置路由路径",
		)
	}
	path := config.GetPath()
	pathType, err := parsePathMatchType(path.GetType())
	if err != nil {
		return resource.RouteMatch{}, err
	}
	pathValue := strings.TrimSpace(path.GetValue())
	if !routeconfig.IsValidPath(pathValue) {
		return resource.RouteMatch{}, errors.BadRequest(
			adminv1.ErrorReason_INVALID_ARGUMENT.String(),
			"路由路径必须是以 / 开头且不包含查询参数或片段的请求路径",
		)
	}
	methods := config.GetMethods()
	if len(methods) > routeconfig.MaxHTTPMethods {
		return resource.RouteMatch{}, errors.BadRequest(
			adminv1.ErrorReason_INVALID_ARGUMENT.String(),
			"路由匹配的 HTTP 方法过多",
		)
	}
	headers := config.GetHeaders()
	if len(headers) > routeconfig.MaxHeaderMatches {
		return resource.RouteMatch{}, errors.BadRequest(
			adminv1.ErrorReason_INVALID_ARGUMENT.String(),
			"路由匹配的 Header 条件过多",
		)
	}

	match := resource.RouteMatch{
		Path:    resource.PathMatch{Type: pathType, Value: pathValue},
		Methods: make([]string, len(methods)),
		Headers: make([]resource.HeaderMatch, len(headers)),
	}
	seenMethods := make(map[string]bool, len(methods))
	for i, method := range methods {
		httpMethod, err := parseHTTPMethod(method)
		if err != nil {
			return resource.RouteMatch{}, err
		}
		if seenMethods[httpMethod] {
			return resource.RouteMatch{}, errors.BadRequest(
				adminv1.ErrorReason_INVALID_ARGUMENT.String(),
				"HTTP 方法不能重复",
			)
		}
		seenMethods[httpMethod] = true
		match.Methods[i] = httpMethod
	}

	seenHeaders := make(map[string]bool, len(headers))
	for i, header := range headers {
		if header == nil {
			return resource.RouteMatch{}, errors.BadRequest(
				adminv1.ErrorReason_INVALID_ARGUMENT.String(),
				"Header 匹配条件不能为空",
			)
		}
		name := httpheader.NormalizeName(header.GetName())
		value := httpheader.NormalizeValue(header.GetValue())
		if !httpheader.IsValidName(name) || value == "" || !httpheader.IsValidValue(value) {
			return resource.RouteMatch{}, errors.BadRequest(
				adminv1.ErrorReason_INVALID_ARGUMENT.String(),
				"Header 匹配条件的名称或值格式不正确",
			)
		}
		if seenHeaders[name] {
			return resource.RouteMatch{}, errors.BadRequest(
				adminv1.ErrorReason_INVALID_ARGUMENT.String(),
				"同一个 Header 只能匹配一次",
			)
		}
		seenHeaders[name] = true
		match.Headers[i] = resource.HeaderMatch{Name: name, Value: value}
	}
	return match, nil
}

func parsePathMatchType(matchType adminv1.RoutePathMatchType) (resource.PathMatchType, error) {
	switch matchType {
	case adminv1.RoutePathMatchType_ROUTE_PATH_MATCH_TYPE_PREFIX:
		return resource.PathMatchPrefix, nil
	case adminv1.RoutePathMatchType_ROUTE_PATH_MATCH_TYPE_EXACT:
		return resource.PathMatchExact, nil
	default:
		return "", errors.BadRequest(
			adminv1.ErrorReason_INVALID_ARGUMENT.String(),
			"路由路径匹配方式不正确",
		)
	}
}

func parseHTTPMethod(method adminv1.HTTPMethod) (string, error) {
	switch method {
	case adminv1.HTTPMethod_HTTP_METHOD_GET:
		return http.MethodGet, nil
	case adminv1.HTTPMethod_HTTP_METHOD_HEAD:
		return http.MethodHead, nil
	case adminv1.HTTPMethod_HTTP_METHOD_POST:
		return http.MethodPost, nil
	case adminv1.HTTPMethod_HTTP_METHOD_PUT:
		return http.MethodPut, nil
	case adminv1.HTTPMethod_HTTP_METHOD_PATCH:
		return http.MethodPatch, nil
	case adminv1.HTTPMethod_HTTP_METHOD_DELETE:
		return http.MethodDelete, nil
	case adminv1.HTTPMethod_HTTP_METHOD_OPTIONS:
		return http.MethodOptions, nil
	default:
		return "", errors.BadRequest(
			adminv1.ErrorReason_INVALID_ARGUMENT.String(),
			"HTTP 方法不正确",
		)
	}
}

func matchResponse(match resource.RouteMatch) *adminv1.RouteMatch {
	response := &adminv1.RouteMatch{
		Path: &adminv1.RoutePathMatch{
			Type:  pathMatchTypeResponse(match.Path.Type),
			Value: match.Path.Value,
		},
		Methods: make([]adminv1.HTTPMethod, len(match.Methods)),
		Headers: make([]*adminv1.HeaderMatch, len(match.Headers)),
	}
	for i, method := range match.Methods {
		response.Methods[i] = httpMethodResponse(method)
	}
	for i, header := range match.Headers {
		response.Headers[i] = &adminv1.HeaderMatch{
			Name:  header.Name,
			Value: header.Value,
		}
	}
	return response
}

func pathMatchTypeResponse(matchType resource.PathMatchType) adminv1.RoutePathMatchType {
	switch matchType {
	case resource.PathMatchPrefix:
		return adminv1.RoutePathMatchType_ROUTE_PATH_MATCH_TYPE_PREFIX
	case resource.PathMatchExact:
		return adminv1.RoutePathMatchType_ROUTE_PATH_MATCH_TYPE_EXACT
	default:
		return adminv1.RoutePathMatchType_ROUTE_PATH_MATCH_TYPE_UNSPECIFIED
	}
}

func httpMethodResponse(method string) adminv1.HTTPMethod {
	switch method {
	case http.MethodGet:
		return adminv1.HTTPMethod_HTTP_METHOD_GET
	case http.MethodHead:
		return adminv1.HTTPMethod_HTTP_METHOD_HEAD
	case http.MethodPost:
		return adminv1.HTTPMethod_HTTP_METHOD_POST
	case http.MethodPut:
		return adminv1.HTTPMethod_HTTP_METHOD_PUT
	case http.MethodPatch:
		return adminv1.HTTPMethod_HTTP_METHOD_PATCH
	case http.MethodDelete:
		return adminv1.HTTPMethod_HTTP_METHOD_DELETE
	case http.MethodOptions:
		return adminv1.HTTPMethod_HTTP_METHOD_OPTIONS
	default:
		return adminv1.HTTPMethod_HTTP_METHOD_UNSPECIFIED
	}
}
