package gateway

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Certificate 声明一个可由多个 HTTPS Gateway 监听器复用的 TLS 证书
type Certificate struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   CertificateSpec `json:"spec,omitempty"`
	Status ResourceStatus  `json:"status,omitempty"`
}

// CertificateList 表示 Certificate 资源列表
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type CertificateList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []Certificate `json:"items"`
}

// CertificateSpec 定义 TLS 证书及其私钥
type CertificateSpec struct {
	// DisplayName 保存控制台展示名称，不参与 Gateway 引用
	DisplayName string `json:"displayName,omitempty"`
	// CertificatePEM 保存叶子证书以及可选的中间证书链
	CertificatePEM string `json:"certificatePEM"`
	// PrivateKeyPEM 保存与叶子证书匹配的私钥
	PrivateKeyPEM string `json:"privateKeyPEM"`
}
