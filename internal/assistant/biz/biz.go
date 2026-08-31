// Package biz 装配运维助手的业务用例和领域执行器。
package biz

import (
	"github.com/google/wire"

	"github.com/lgc202/ingate/internal/assistant/biz/agent"
	"github.com/lgc202/ingate/internal/assistant/biz/change"
	"github.com/lgc202/ingate/internal/assistant/biz/conversation"
	"github.com/lgc202/ingate/internal/assistant/biz/execution"
	"github.com/lgc202/ingate/internal/assistant/biz/modelconfig"
)

// ProviderSet 汇总运维助手的业务用例和领域执行器。
var ProviderSet = wire.NewSet(
	conversation.NewUsecase,
	change.NewUsecase,
	execution.NewUsecase,
	execution.NewExecutor,
	modelconfig.NewUsecase,
	agent.New,
	wire.Bind(new(execution.Agent), new(*agent.Agent)),
)
