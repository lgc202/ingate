package gateway

import resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"

// CreateGatewayParams 是创建 Gateway 用例参数
type CreateGatewayParams struct {
	GatewayParams
}

// UpdateGatewayParams 是更新 Gateway 用例参数
type UpdateGatewayParams struct {
	Version string
	GatewayParams
}

// GatewayParams 是创建和更新 Gateway 共用的配置参数
type GatewayParams struct {
	Name        string
	Description string
	Listeners   []ListenerParams
	Hostnames   []string
}

// ListenerParams 是固定数据面入口参数
type ListenerParams struct {
	Protocol      resource.Protocol
	Port          int
	CertificateID string
}
