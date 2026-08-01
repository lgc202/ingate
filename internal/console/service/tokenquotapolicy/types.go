package tokenquotapolicy

import (
	"github.com/lgc202/ingate/internal/console/service/policytarget"
	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
)

// ListResult 表示 TokenQuotaPolicy 列表查询结果
type ListResult struct {
	Policies    []resource.TokenQuotaPolicy
	TargetNames policytarget.DisplayNames
}

// PolicyResult 表示单个 TokenQuotaPolicy 查询结果
type PolicyResult struct {
	Policy      *resource.TokenQuotaPolicy
	TargetNames policytarget.DisplayNames
}
