// Package data 提供 AI ExtProc 依赖的外部数据来源
package data

import (
	"github.com/google/wire"

	"github.com/lgc202/ingate/internal/aiextproc/data/apiserver"
	aiextprocservice "github.com/lgc202/ingate/internal/aiextproc/service"
)

// ProviderSet 提供 API Server 模型服务配置缓存
var ProviderSet = wire.NewSet(
	apiserver.NewModelServiceCache,
	wire.Bind(new(aiextprocservice.ModelAPIKeySource), new(*apiserver.ModelServiceCache)),
)
