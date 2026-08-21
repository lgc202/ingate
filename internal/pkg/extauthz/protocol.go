// Package extauthz 定义 Controller、Authz 和 ALS 共享的 Envoy External Authorization 协议名称
package extauthz

const (
	// FilterName 是 Envoy External Authorization 过滤器的标准名称
	FilterName = "envoy.filters.http.ext_authz"
	// RouteIDContext 把当前 Ingate Route ID 传给鉴权服务
	RouteIDContext = "route_id"
	// MetadataNamespace 是鉴权结果写入 Envoy dynamic metadata 的命名空间
	MetadataNamespace = FilterName
	// CallerIDField 记录本次请求归属的 Caller ID
	CallerIDField = "caller_id"
	// AccessKeyIDField 记录本次请求使用的访问密钥 ID
	AccessKeyIDField = "access_key_id"
)
