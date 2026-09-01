package tokenquota

import (
	"time"

	resource "github.com/lgc202/ingate/internal/pkg/apis/gateway/v1"
)

// Usage 表示调用方当前命中的一项 Token 额度及其实时计数。
type Usage struct {
	PolicyID   string
	PolicyName string
	Period     resource.TokenQuotaPeriod
	Used       int64
	Limit      int64
	StartedAt  time.Time
	ResetAt    time.Time
}
