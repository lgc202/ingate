// Package service 装配运维助手的协议服务。
package service

import (
	"github.com/google/wire"

	changeservice "github.com/lgc202/ingate/internal/assistant/service/change"
	conversationservice "github.com/lgc202/ingate/internal/assistant/service/conversation"
	executionservice "github.com/lgc202/ingate/internal/assistant/service/execution"
	modelconfigservice "github.com/lgc202/ingate/internal/assistant/service/modelconfig"
)

// ProviderSet 汇总运维助手的协议实现。
var ProviderSet = wire.NewSet(
	conversationservice.NewService,
	changeservice.NewService,
	executionservice.NewService,
	modelconfigservice.NewService,
)
