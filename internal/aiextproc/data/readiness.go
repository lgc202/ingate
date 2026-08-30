package data

import (
	dataapiserver "github.com/lgc202/ingate/internal/aiextproc/data/apiserver"
	dataredis "github.com/lgc202/ingate/internal/aiextproc/data/redis"
)

// Readiness 汇总 AI 请求执行依赖的配置同步和 Redis 连接状态。
type Readiness struct {
	configs *dataapiserver.ConfigCache
	quotas  *dataredis.TokenCounter
}

// NewReadiness 创建 AI ExtProc 外部依赖就绪检查。
func NewReadiness(configs *dataapiserver.ConfigCache, quotas *dataredis.TokenCounter) *Readiness {
	return &Readiness{configs: configs, quotas: quotas}
}

// Ready 仅在配置已同步且 Token 额度存储可用时返回 true。
func (r *Readiness) Ready() bool {
	return r.configs.Ready() && r.quotas.Ready()
}
