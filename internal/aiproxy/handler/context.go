package handler

import (
	"context"

	"github.com/lgc202/ingate/internal/aiproxy/pkg/aiconfig"
)

type modelEntryKey struct{}

type modelBackendKey struct{}

// modelEntry 是受信任入站适配层确定的本次请求入口，不承载可变配置。
type modelEntry struct {
	port     int32
	hostName string
}

// WithModelEntry 将已确定的 Listener 端口和 Host 完整编译标识关联到本次请求。
// 只能由受信任入站适配层调用，不能直接照抄客户端 Header；本函数本身不执行认证。
// 它只用于进程内传递，不定义 Envoy 与 Go 之间的 Header 或 UDS 协议。
func WithModelEntry(ctx context.Context, port int32, hostName string) context.Context {
	return context.WithValue(ctx, modelEntryKey{}, modelEntry{port: port, hostName: hostName})
}

// ModelBackend 返回模型 Handler 为本次请求选中的后端，结果不含可变引用。
// 后续转发器须使用该绑定，不能用客户端 URL/Host 或自报的后端 Header 重新选目标。
// 未经过请求准备时返回 false；此结果不表示 Envoy 已具备对应出口。
func ModelBackend(ctx context.Context) (aiconfig.ModelBackend, bool) {
	backend, ok := ctx.Value(modelBackendKey{}).(aiconfig.ModelBackend)

	return backend, ok
}
