// Package biz 装配运维助手的业务服务。
package biz

import (
	"github.com/google/wire"

	"github.com/lgc202/ingate/internal/assistant/biz/conversation"
	"github.com/lgc202/ingate/internal/assistant/biz/execution"
	"github.com/lgc202/ingate/internal/assistant/biz/modelconfig"
)

// ProviderSet 汇总运维助手的业务服务。
var ProviderSet = wire.NewSet(
	conversation.NewService,
	execution.NewService,
	execution.NewExecutor,
	modelconfig.NewService,
)
