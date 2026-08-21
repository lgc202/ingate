package route

import (
	"net/http"
	"net/url"
	"strings"

	"golang.org/x/net/http/httpguts"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	adminservice "github.com/lgc202/ingate/internal/adminapi/service"
	resource "github.com/lgc202/ingate/internal/pkg/apis/gateway/v1"
)

func routeMatch(input *adminv1.RouteMatch) (resource.RouteMatch, error) {
	if input == nil || input.GetPath() == nil {
		return resource.RouteMatch{}, adminservice.BadRequest("必须配置路由路径")
	}
	pathType, err := pathMatchType(input.GetPath().GetType())
	if err != nil {
		return resource.RouteMatch{}, err
	}
	pathValue := strings.TrimSpace(input.GetPath().GetValue())
	if !validPath(pathValue) {
		return resource.RouteMatch{}, adminservice.BadRequest("路由路径必须是以 / 开头且不包含查询参数或片段的请求路径")
	}

	match := resource.RouteMatch{Path: resource.PathMatch{Type: pathType, Value: pathValue}}
	seenMethods := make(map[string]struct{}, len(input.GetMethods()))
	for _, inputMethod := range input.GetMethods() {
		method, err := httpMethod(inputMethod)
		if err != nil {
			return resource.RouteMatch{}, err
		}
		if _, exists := seenMethods[method]; exists {
			return resource.RouteMatch{}, adminservice.BadRequest("HTTP 方法不能重复")
		}
		seenMethods[method] = struct{}{}
		match.Methods = append(match.Methods, method)
	}

	seenHeaders := make(map[string]struct{}, len(input.GetHeaders()))
	for _, inputHeader := range input.GetHeaders() {
		if inputHeader == nil {
			return resource.RouteMatch{}, adminservice.BadRequest("Header 匹配条件不能为空")
		}
		name := strings.ToLower(strings.TrimSpace(inputHeader.GetName()))
		value := inputHeader.GetValue()
		if !httpguts.ValidHeaderFieldName(name) || value == "" || !httpguts.ValidHeaderFieldValue(value) {
			return resource.RouteMatch{}, adminservice.BadRequest("Header 匹配条件的名称或值格式不正确")
		}
		if _, exists := seenHeaders[name]; exists {
			return resource.RouteMatch{}, adminservice.BadRequest("同一个 Header 只能匹配一次")
		}
		seenHeaders[name] = struct{}{}
		match.Headers = append(match.Headers, resource.HeaderMatch{Name: name, Value: value})
	}
	return match, nil
}

func pathMatchType(matchType adminv1.RoutePathMatchType) (resource.PathMatchType, error) {
	switch matchType {
	case adminv1.RoutePathMatchType_ROUTE_PATH_MATCH_PREFIX:
		return resource.PathMatchPrefix, nil
	case adminv1.RoutePathMatchType_ROUTE_PATH_MATCH_EXACT:
		return resource.PathMatchExact, nil
	default:
		return "", adminservice.BadRequest("路由路径匹配方式不正确")
	}
}

func httpMethod(method adminv1.HTTPMethod) (string, error) {
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
		return "", adminservice.BadRequest("HTTP 方法不正确")
	}
}

func validPath(value string) bool {
	if !strings.HasPrefix(value, "/") || strings.ContainsAny(value, "?#") {
		return false
	}
	_, err := url.ParseRequestURI(value)
	return err == nil
}
