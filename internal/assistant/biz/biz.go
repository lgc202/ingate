// Package biz 装配运维助手的业务用例。
package biz

import (
	"github.com/google/wire"

	"github.com/lgc202/ingate/internal/assistant/biz/system"
)

// ProviderSet 提供运维助手的业务用例。
var ProviderSet = wire.NewSet(system.NewUsecase)
