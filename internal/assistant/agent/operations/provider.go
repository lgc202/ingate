package operations

import (
	"github.com/google/wire"

	"github.com/lgc202/ingate/internal/assistant/biz/execution"
)

// ProviderSet 提供面向 Ingate 运维场景的 Agent 实现。
var ProviderSet = wire.NewSet(
	New,
	wire.Bind(new(execution.Agent), new(*Agent)),
)
