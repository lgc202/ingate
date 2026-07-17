// Package systemstatus 承载单配置域运行状态查询用例
package systemstatus

import (
	"context"

	controllerclient "github.com/lgc202/ingate/internal/adminapi/client/controller"
)

const (
	readyMessage       = "Controller 正常提供配置编译和 xDS 交付服务"
	reconcilingMessage = "Controller 已连接，正在等待首次配置收敛"
)

// Service 查询 Controller 与 Envoy 的实时运行状态
type Service struct {
	client *controllerclient.Client
}

// New 创建单配置域运行状态 service
func New(client *controllerclient.Client) *Service {
	return &Service{client: client}
}

// Get 返回当前单配置域的编译和 Envoy 配置交付状态
func (s *Service) Get(ctx context.Context) (*Result, error) {
	status, err := s.client.GetStatus(ctx)
	if err != nil {
		return nil, err
	}

	message := readyMessage
	if !status.Reconciled {
		message = reconcilingMessage
	}
	result := &Result{
		Available:        true,
		Message:          message,
		ConfigReady:      status.ConfigReady,
		DeliveryState:    string(status.DeliveryState),
		CandidateVersion: status.CandidateVersion,
		ActiveVersion:    status.ActiveVersion,
		ConnectedEnvoys:  status.ConnectedEnvoys,
		ACK: ACKSummary{
			Required: status.ACKs.Required,
			Received: status.ACKs.Received,
		},
	}
	if status.LastNACK != nil {
		result.LastNACK = &NACK{
			NodeID:  status.LastNACK.NodeID,
			TypeURL: status.LastNACK.TypeURL,
			Version: status.LastNACK.Version,
			Time:    status.LastNACK.Time,
			Message: status.LastNACK.Message,
		}
	}
	return result, nil
}
