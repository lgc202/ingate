// Package service 装配运维助手的协议服务。
package service

import (
	"github.com/google/wire"

	serviceSystem "github.com/lgc202/ingate/internal/assistant/service/system"
)

// ProviderSet 提供运维助手的协议实现。
var ProviderSet = wire.NewSet(serviceSystem.NewService)
