package modelproxy

import (
	"context"

	"github.com/lgc202/ingate/internal/aiproxy/pkg/aiconfig"
)

type backendKey struct{}

// WithBackend 关联 Service 已选定的按值后端；只允许受信任的进程内请求准备代码调用。
// 不从客户端 Header 生成这个值，不重新读取配置，也不表示 Envoy 已接收对应出口。
func WithBackend(ctx context.Context, backend aiconfig.ModelBackend) context.Context {
	return context.WithValue(ctx, backendKey{}, backend)
}

// Backend 返回本次请求选中的按值绑定，不包含共享可变配置引用。
func Backend(ctx context.Context) (aiconfig.ModelBackend, bool) {
	backend, ok := ctx.Value(backendKey{}).(aiconfig.ModelBackend)

	return backend, ok
}
