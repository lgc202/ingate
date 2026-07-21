package accesscontrolpolicy

import (
	"github.com/lgc202/ingate/internal/adminapi/service/policytarget"
	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
)

// ListResult 表示 AccessControlPolicy 列表结果
type ListResult struct {
	Policies    []resource.AccessControlPolicy
	TargetNames policytarget.DisplayNames
}

// PolicyResult 表示单个 AccessControlPolicy 查询结果
type PolicyResult struct {
	Policy      *resource.AccessControlPolicy
	TargetNames policytarget.DisplayNames
}
