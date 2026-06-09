package gateway

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// AccessControlAction 表示访问控制规则命中后的处理动作
type AccessControlAction string

const (
	// AccessControlActionAllow 表示允许请求继续
	AccessControlActionAllow AccessControlAction = "Allow"
	// AccessControlActionDeny 表示拒绝请求
	AccessControlActionDeny AccessControlAction = "Deny"
)

// AccessControlConditionType 表示访问控制匹配维度
type AccessControlConditionType string

const (
	// AccessControlConditionTypeIP 表示按客户端 IP 匹配
	AccessControlConditionTypeIP AccessControlConditionType = "IP"
	// AccessControlConditionTypeHeader 表示按请求 Header 匹配
	AccessControlConditionTypeHeader AccessControlConditionType = "Header"
	// AccessControlConditionTypeConsumer 表示按认证后的 consumer 匹配
	AccessControlConditionTypeConsumer AccessControlConditionType = "Consumer"
	// AccessControlConditionTypeTenant 表示按租户匹配
	AccessControlConditionTypeTenant AccessControlConditionType = "Tenant"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// AccessControlPolicy 声明访问控制策略
type AccessControlPolicy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   AccessControlPolicySpec `json:"spec,omitempty"`
	Status ResourceStatus          `json:"status,omitempty"`
}

// AccessControlPolicyList 表示 AccessControlPolicy 资源列表
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type AccessControlPolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []AccessControlPolicy `json:"items"`
}

// AccessControlPolicySpec 定义访问控制策略配置
type AccessControlPolicySpec struct {
	DisplayName   string              `json:"displayName"`
	Description   string              `json:"description,omitempty"`
	Enabled       bool                `json:"enabled"`
	DefaultAction AccessControlAction `json:"defaultAction,omitempty"`
	// +listType=atomic
	Rules    []AccessControlRule       `json:"rules,omitempty"`
	Response AccessControlDenyResponse `json:"response,omitempty"`
}

// AccessControlRule 定义一条访问控制规则
type AccessControlRule struct {
	Name   string              `json:"name"`
	Action AccessControlAction `json:"action"`
	// +listType=atomic
	Conditions []AccessControlCondition `json:"conditions,omitempty"`
}

// AccessControlCondition 定义访问控制规则中的一个匹配条件
type AccessControlCondition struct {
	Type  AccessControlConditionType `json:"type"`
	Name  string                     `json:"name,omitempty"`
	Value string                     `json:"value"`
}

// AccessControlDenyResponse 定义访问控制拒绝响应
type AccessControlDenyResponse struct {
	StatusCode int    `json:"statusCode,omitempty"`
	Message    string `json:"message,omitempty"`
}
