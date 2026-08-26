// Package service 装配运维助手的协议服务。
package service

import (
	"github.com/google/wire"

	conversationservice "github.com/lgc202/ingate/internal/assistant/service/conversation"
	executionservice "github.com/lgc202/ingate/internal/assistant/service/execution"
	modelservice "github.com/lgc202/ingate/internal/assistant/service/model"
)

// ProviderSet 汇总运维助手的协议实现。
var ProviderSet = wire.NewSet(
	conversationservice.NewService,
	executionservice.NewService,
	modelservice.NewService,
)
