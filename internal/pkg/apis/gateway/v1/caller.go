package v1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +genclient
// +genclient:nonNamespaced

// Caller 声明一个调用网关的应用或服务及其访问权限。
type Caller struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   CallerSpec     `json:"spec,omitempty"`
	Status ResourceStatus `json:"status,omitempty"`
}

// CallerList 表示 Caller 资源列表。
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type CallerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []Caller `json:"items"`
}

// CallerSpec 定义调用方是否可用、可访问的 Route 以及访问密钥。
type CallerSpec struct {
	// DisplayName 保存控制台展示名称，不参与鉴权匹配。
	DisplayName string `json:"displayName,omitempty"`
	// Enabled 为 false 时立即拒绝该调用方的全部密钥。
	Enabled bool `json:"enabled"`
	// RouteRefs 保存当前调用方可以访问的 Route ID。
	// +listType=set
	RouteRefs []string `json:"routeRefs,omitempty"`
	// AccessKeys 只保存不可逆摘要，完整密钥仅在签发时返回一次。
	// +listType=map
	// +listMapKey=id
	AccessKeys []AccessKey `json:"accessKeys,omitempty"`
}

// AccessKey 表示 Caller 下的一份可独立停用和到期的访问凭据。
type AccessKey struct {
	// ID 是密钥的公开标识，也用于从请求凭据中定位摘要。
	ID string `json:"id"`
	// DisplayName 用于区分同一调用方在不同环境或客户端中的密钥。
	DisplayName string `json:"displayName"`
	// SecretDigest 是完整密钥的 SHA-256 摘要，不保存可恢复的明文。
	SecretDigest string `json:"secretDigest"`
	// Enabled 为 false 时该密钥立即失效。
	Enabled bool `json:"enabled"`
	// CreatedAt 记录密钥签发时间。
	CreatedAt metav1.Time `json:"createdAt"`
	// ExpiresAt 为空表示密钥不会自动到期。
	ExpiresAt *metav1.Time `json:"expiresAt,omitempty"`
}
