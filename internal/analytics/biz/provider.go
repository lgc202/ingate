package biz

import (
	"github.com/google/wire"

	"github.com/lgc202/ingate/internal/analytics/biz/request"
	"github.com/lgc202/ingate/internal/analytics/biz/traffic"
)

// ProviderSet 汇总 Analytics 的业务用例
var ProviderSet = wire.NewSet(
	request.NewRecorder,
	request.NewQueries,
	traffic.NewQueries,
)
