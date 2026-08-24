// Package biz 装配运维助手的业务服务。
package biz

import (
	"github.com/google/wire"

	"github.com/lgc202/ingate/internal/assistant/biz/conversation"
)

// ProviderSet 汇总运维助手的业务服务。
var ProviderSet = wire.NewSet(conversation.NewService)
