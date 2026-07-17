package dto

import systemstatusservice "github.com/lgc202/ingate/internal/adminapi/service/systemstatus"

const unavailableMessage = "暂时无法获取控制器运行状态，请稍后重试"

// NewGetSystemStatusResp 转换运行状态用例结果为控制台响应
func NewGetSystemStatusResp(result *systemstatusservice.Result) GetSystemStatusResp {
	response := GetSystemStatusResp{
		Available:        result.Available,
		Message:          result.Message,
		ConfigReady:      result.ConfigReady,
		DeliveryState:    result.DeliveryState,
		CandidateVersion: result.CandidateVersion,
		ActiveVersion:    result.ActiveVersion,
		ConnectedEnvoys:  result.ConnectedEnvoys,
		ACK: ACKSummary{
			Required: result.ACK.Required,
			Received: result.ACK.Received,
		},
	}
	if result.LastNACK != nil {
		response.LastNACK = &NACK{
			NodeID:  result.LastNACK.NodeID,
			TypeURL: result.LastNACK.TypeURL,
			Version: result.LastNACK.Version,
			Time:    result.LastNACK.Time,
			Message: result.LastNACK.Message,
		}
	}
	return response
}

// NewUnavailableSystemStatusResp 返回不泄漏内部错误的稳定降级响应
func NewUnavailableSystemStatusResp() GetSystemStatusResp {
	return GetSystemStatusResp{
		Available:       false,
		Message:         unavailableMessage,
		ConfigReady:     false,
		DeliveryState:   "NoConfig",
		ConnectedEnvoys: 0,
		ACK:             ACKSummary{},
	}
}
