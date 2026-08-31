package execution

import (
	"context"
	"fmt"

	agentbiz "github.com/lgc202/ingate/internal/assistant/biz/agent"
	agenttool "github.com/lgc202/ingate/internal/assistant/biz/agent/tool"
	"github.com/lgc202/ingate/internal/assistant/biz/conversation"
)

// Agent 表示后台执行器需要的推理能力。
// 接口定义在调用方，业务编排不依赖 Eino、模型 SDK 或具体工具实现。
type Agent interface {
	Execute(context.Context, agentbiz.Request, agentbiz.EventSink) (agentbiz.Response, error)
}

// Completion 是成功执行提交到会话存储的最终内容。
// 它属于持久化命令，不携带模型客户端、工具调用或流式事件等临时状态。
type Completion struct {
	Content          string
	ReasoningContent string
}

func agentRequest(
	executionID string,
	resume *Resume,
	messages []conversation.HistoryMessage,
) (agentbiz.Request, error) {
	request := agentbiz.Request{
		CheckpointID: executionID,
		Messages:     make([]agentbiz.Message, 0, len(messages)),
	}
	if resume != nil {
		request.Resume = &agentbiz.Resume{
			InterruptID: resume.InterruptID,
			Result: &agenttool.ApprovalResult{
				Approved: resume.Approved,
				Feedback: resume.Feedback,
			},
		}
	}
	for _, message := range messages {
		var role agentbiz.Role
		switch message.Role {
		case conversation.RoleUser:
			role = agentbiz.RoleUser
		case conversation.RoleAssistant:
			role = agentbiz.RoleAssistant
		default:
			return agentbiz.Request{}, fmt.Errorf(
				"unsupported persistent message role %q",
				message.Role,
			)
		}
		request.Messages = append(request.Messages, agentbiz.Message{
			Role:    role,
			Content: message.Content,
		})
	}
	return request, nil
}
