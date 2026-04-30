package resource

// ResourceListView 表示前端使用的资源列表视图
type ResourceListView struct {
	Kind  Kind           `json:"kind"`
	Items []ResourceView `json:"items"`
}

// ResourceView 表示前端使用的资源详情视图
type ResourceView struct {
	Kind     Kind           `json:"kind"`
	Metadata MetadataView   `json:"metadata"`
	Spec     map[string]any `json:"spec"`
	Status   StatusView     `json:"status"`
}

// MetadataView 表示 admin-api 暴露给前端的资源元信息
type MetadataView struct {
	Name            string `json:"name"`
	Generation      int64  `json:"generation"`
	ResourceVersion string `json:"resourceVersion,omitempty"`
	CreatedAt       string `json:"createdAt,omitempty"`
}

// StatusView 表示 admin-api 暴露给前端的资源状态
type StatusView struct {
	Conditions []ConditionView `json:"conditions,omitempty"`
}

// ConditionView 表示资源状态条件
type ConditionView struct {
	Type               string `json:"type"`
	Status             string `json:"status"`
	ObservedGeneration int64  `json:"observedGeneration,omitempty"`
	LastTransitionTime string `json:"lastTransitionTime,omitempty"`
	Reason             string `json:"reason,omitempty"`
	Message            string `json:"message,omitempty"`
}
