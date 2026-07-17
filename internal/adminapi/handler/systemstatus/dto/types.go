package dto

import "time"

// GetSystemStatusResp 是控制台使用的单配置域运行状态
type GetSystemStatusResp struct {
	Available        bool       `json:"available"`
	Message          string     `json:"message"`
	ConfigReady      bool       `json:"configReady"`
	DeliveryState    string     `json:"deliveryState"`
	CandidateVersion string     `json:"candidateVersion,omitempty"`
	ActiveVersion    string     `json:"activeVersion,omitempty"`
	ConnectedEnvoys  int        `json:"connectedEnvoys"`
	ACK              ACKSummary `json:"ack"`
	LastNACK         *NACK      `json:"lastNack,omitempty"`
}

// ACKSummary 是控制台展示的 Envoy ACK 进度
type ACKSummary struct {
	Required int `json:"required"`
	Received int `json:"received"`
}

// NACK 是控制台展示的最近一次 Envoy 配置拒绝
type NACK struct {
	NodeID  string    `json:"nodeID"`
	TypeURL string    `json:"typeURL"`
	Version string    `json:"version"`
	Time    time.Time `json:"time"`
	Message string    `json:"message"`
}
