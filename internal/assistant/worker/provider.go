package worker

import "github.com/google/wire"

// ProviderSet 提供后台任务。
var ProviderSet = wire.NewSet(NewExecutionWorker)
