package execution

import (
	"context"

	agentprotocol "github.com/lgc202/ingate/internal/assistant/agent"
	"github.com/lgc202/ingate/internal/assistant/biz/conversation"
)

// Agent 表示后台执行器需要的推理能力。
// 接口定义在调用方，业务编排不依赖 Eino、模型 SDK 或具体工具实现。
type Agent interface {
	Execute(context.Context, agentprotocol.Request, agentprotocol.EventSink) (agentprotocol.Response, error)
}

// Completion 是成功执行提交到会话存储的最终内容。
// 它属于持久化命令，不携带模型客户端、工具调用或流式事件等临时状态。
type Completion struct {
	Content          string
	ReasoningContent string
}

func newAgentRequest(messages []conversation.Message) agentprotocol.Request {
	request := agentprotocol.Request{
		Messages: make([]agentprotocol.Message, 0, len(messages)),
	}
	for _, message := range messages {
		var role agentprotocol.Role
		switch message.Role {
		case conversation.RoleUser:
			role = agentprotocol.RoleUser
		case conversation.RoleAssistant:
			role = agentprotocol.RoleAssistant
		default:
			// 持久层可能在后续加入系统通知等非模型消息。执行上下文只接收 Agent
			// 明确定义的角色，避免新存储类型未经设计就悄悄改变模型输入。
			continue
		}
		request.Messages = append(request.Messages, agentprotocol.Message{
			Role:    role,
			Content: message.Content,
		})
	}
	return request
}
