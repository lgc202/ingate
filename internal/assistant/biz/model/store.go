package model

import "context"

// Store 持久化运维助手当前生效的唯一模型连接。
type Store interface {
	GetModelConnection(context.Context) (Connection, error)
	UpdateModelConnection(context.Context, Update) (Connection, error)
}
