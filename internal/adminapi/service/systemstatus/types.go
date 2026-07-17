package systemstatus

import "time"

// Result 是控制台读取单配置域运行状态的用例结果
type Result struct {
	Available        bool
	Message          string
	ConfigReady      bool
	DeliveryState    string
	CandidateVersion string
	ActiveVersion    string
	ConnectedEnvoys  int
	ACK              ACKSummary
	LastNACK         *NACK
}

// ACKSummary 是控制台展示的 Envoy ACK 进度
type ACKSummary struct {
	Required int
	Received int
}

// NACK 是控制台展示的最近一次 Envoy 配置拒绝
type NACK struct {
	NodeID  string
	TypeURL string
	Version string
	Time    time.Time
	Message string
}
