package v1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +genclient
// +genclient:nonNamespaced

// RedisStore 声明 Redis 连接配置
type RedisStore struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   RedisStoreSpec `json:"spec,omitempty"`
	Status ResourceStatus `json:"status,omitempty"`
}

// RedisStoreList 表示 RedisStore 资源列表
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type RedisStoreList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []RedisStore `json:"items"`
}

// RedisStoreSpec 定义 Redis 连接配置
type RedisStoreSpec struct {
	DisplayName string    `json:"displayName"`
	Description string    `json:"description,omitempty"`
	Mode        RedisMode `json:"mode"`
	Address     string    `json:"address"`
	// +listType=atomic
	Addresses            []string `json:"addresses,omitempty"`
	DB                   int      `json:"db,omitempty"`
	TLS                  bool     `json:"tls,omitempty"`
	TLSServerName        string   `json:"tlsServerName,omitempty"`
	Username             string   `json:"username,omitempty"`
	PasswordRef          string   `json:"passwordRef,omitempty"`
	ConnectTimeoutMillis int      `json:"connectTimeoutMillis,omitempty"`
	CommandTimeoutMillis int      `json:"commandTimeoutMillis,omitempty"`
	PoolSize             int      `json:"poolSize,omitempty"`
	MinIdleConns         int      `json:"minIdleConns,omitempty"`
	SentinelMaster       string   `json:"sentinelMaster,omitempty"`
}
