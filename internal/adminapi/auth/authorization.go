package auth

import "strings"

const operationPrefix = "/ingate.admin.v1."

var managedServices = map[string]struct{}{
	"IPRestrictionPolicyService": {},
	"AuthenticationService":      {},
	"CertificateService":         {},
	"ConfigurationService":       {},
	"GatewayService":             {},
	"RateLimitPolicyService":     {},
	"RouteService":               {},
	"UpstreamService":            {},
}

// IsPublicOperation 判断请求是否可以在没有登录状态时访问
func IsPublicOperation(operation string) bool {
	return operation == operationPrefix+"HealthService/Check" ||
		operation == operationPrefix+"AuthenticationService/GetAuthenticationConfiguration"
}

// IsWriteOperation 判断操作是否改变管理面事实，用于审计事件筛选
func IsWriteOperation(operation string) bool {
	_, method, ok := splitOperation(operation)
	return ok && !strings.HasPrefix(method, "Get") && !strings.HasPrefix(method, "List")
}

// Allowed 实现固定三角色模型：查看者只读、操作员管理流量配置、管理员管理全部资源
func Allowed(role Role, operation string) bool {
	service, method, ok := splitOperation(operation)
	if !ok {
		return false
	}
	if service == "HealthService" && method == "Check" {
		return true
	}
	if _, exists := managedServices[service]; !exists {
		return false
	}
	if strings.HasPrefix(method, "Get") || strings.HasPrefix(method, "List") {
		return role == RoleViewer || role == RoleOperator || role == RoleAdmin
	}
	return role == RoleOperator || role == RoleAdmin
}

func splitOperation(operation string) (string, string, bool) {
	value, ok := strings.CutPrefix(operation, operationPrefix)
	if !ok {
		return "", "", false
	}
	service, method, ok := strings.Cut(value, "/")
	return service, method, ok && service != "" && method != ""
}
