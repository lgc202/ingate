// Package filterconfig 保存 Controller、Authz 和 ALS 之间的固定协议名称
package filterconfig

const (
	// HTTPFilterName 是 Envoy External Authorization 过滤器的标准名称
	HTTPFilterName = "envoy.filters.http.ext_authz"
	// RouteIDContext 把当前 Ingate Route ID 传给鉴权服务
	RouteIDContext = "route_id"
	// MetadataNamespace 是鉴权成功后写入 Envoy dynamic metadata 的命名空间
	MetadataNamespace = HTTPFilterName
	// CallerIDField 记录本次请求归属的 Caller ID
	CallerIDField = "caller_id"
	// AccessKeyIDField 记录本次请求使用的访问密钥 ID
	AccessKeyIDField = "access_key_id"
)
