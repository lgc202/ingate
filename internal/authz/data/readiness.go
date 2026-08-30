package data

import (
	"github.com/lgc202/ingate/internal/authz/data/apiserver"
	dataredis "github.com/lgc202/ingate/internal/authz/data/redis"
)

// Readiness 汇总 Caller 配置同步和请求限流存储状态。
type Readiness struct {
	credentials *apiserver.CredentialCache
	rates       *dataredis.RateCounter
}

// NewReadiness 创建 Authz 外部依赖就绪检查。
func NewReadiness(credentials *apiserver.CredentialCache, rates *dataredis.RateCounter) *Readiness {
	return &Readiness{credentials: credentials, rates: rates}
}

// Ready 只在凭据首次同步完成且 Redis 可用时返回 true。
func (r *Readiness) Ready() bool {
	return r.credentials.Ready() && r.rates.Ready()
}
