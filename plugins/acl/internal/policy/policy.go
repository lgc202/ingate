// Package policy 执行 ACL 插件的访问控制判断
package policy

import config "github.com/lgc202/ingate/pkg/plugin/acl"

const (
	consumerHeader = "x-ingate-consumer"
	tenantHeader   = "x-ingate-tenant"
)

// Request 表示 ACL 判断需要读取的请求信息
type Request struct {
	GatewayName string
	RouteName   string
	RuleName    string
	RemoteAddr  string
	Headers     map[string]string
}

// Decision 表示 ACL 对一次请求的访问控制结果
type Decision struct {
	Allowed    bool
	StatusCode int
	Message    string
	Rule       config.Rule
}

// Runner 应用 ACL 规则，产出 allow 或 deny 决策
type Runner struct{}

// NewRunner 创建 ACL 策略执行器
func NewRunner() *Runner {
	return &Runner{}
}
