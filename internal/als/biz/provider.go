package biz

import "github.com/google/wire"

// ProviderSet 汇总 ALS 的请求记录投递用例
var ProviderSet = wire.NewSet(NewRecorder)
