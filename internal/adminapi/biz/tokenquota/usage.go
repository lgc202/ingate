package tokenquota

import (
	"time"

	"github.com/go-kratos/kratos/v3/errors"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	resource "github.com/lgc202/ingate/internal/pkg/apis/gateway/v1"
)

// ErrUnavailable 表示 AI ExtProc 当前无法提供实时额度。
var ErrUnavailable = errors.ServiceUnavailable(
	adminv1.ErrorReason_DEPENDENCY_UNAVAILABLE.String(),
	"实时额度暂时不可用，请稍后重试",
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

// Unavailable 保留 AI ExtProc 返回的底层原因，同时向控制台暴露稳定错误语义。
func Unavailable(cause error) error {
	return ErrUnavailable.WithCause(cause)
}
