package agent

import (
	"github.com/google/wire"

	executionbiz "github.com/lgc202/ingate/internal/assistant/biz/execution"
)

// ProviderSet 提供 Agent 与只读工具装配。
var ProviderSet = wire.NewSet(
	NewAgent,
	wire.Bind(new(executionbiz.Agent), new(*Agent)),
)
