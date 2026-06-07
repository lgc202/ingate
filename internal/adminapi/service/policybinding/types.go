package policybinding

import resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"

// ListResult 表示 PolicyBinding 列表查询结果
type ListResult struct {
	Bindings []resource.PolicyBinding
}

// BindingResult 表示单个 PolicyBinding 查询结果
type BindingResult struct {
	Binding *resource.PolicyBinding
}

// BindingParams 表示 PolicyBinding 可编辑字段
type BindingParams struct {
	Name        string
	Description string
	Enabled     bool
	TargetRef   resource.PolicyTargetRef
	Policies    []resource.PolicyRef
}

// CreateBindingParams 表示创建 PolicyBinding 参数
type CreateBindingParams struct {
	BindingParams
}

// UpdateBindingParams 表示更新 PolicyBinding 参数
type UpdateBindingParams struct {
	Version string
	BindingParams
}
