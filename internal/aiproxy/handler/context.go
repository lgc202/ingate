package handler

import "context"

type modelEntryKey struct{}

// modelEntry 是受信任入站适配层确定的本次请求入口，不承载可变配置。
type modelEntry struct {
	port     int32
	hostName string
}

// WithModelEntry 将已确定的 Listener 端口和 Host 完整编译标识关联到本次请求。
// 只能由受信任入站适配层调用，不能直接照抄客户端 Header；本函数本身不执行认证。
// Server 在受保护的 Unix 入口解析 aidataplane 定义的 Header 后调用本函数。
func WithModelEntry(ctx context.Context, port int32, hostName string) context.Context {
	return context.WithValue(ctx, modelEntryKey{}, modelEntry{port: port, hostName: hostName})
}
