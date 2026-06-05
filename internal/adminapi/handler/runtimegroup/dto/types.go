package dto

// ListRuntimeGroupsResp 是 RuntimeGroup 列表接口响应
type ListRuntimeGroupsResp struct {
	RuntimeGroups []RuntimeGroupSummary `json:"runtimeGroups"`
}

// RuntimeGroupSummary 是控制台使用的运行组摘要
type RuntimeGroupSummary struct {
	ID          string `json:"id"`
	DisplayName string `json:"displayName"`
	Description string `json:"description"`
	Enabled     bool   `json:"enabled"`
	Target      string `json:"target"`
}
