// Package protocol 提供 Admin API 协议层复用的纯转换函数。
package protocol

import (
	"cmp"

	"github.com/lgc202/ingate/internal/adminapi/biz/pagination"
)

const (
	defaultPageLimit = 100
	maxPageLimit     = 200
)

// PageRequest 把控制台分页参数转换为不依赖存储实现的 biz 参数。
func PageRequest(limit int32, cursor string) pagination.Request {
	normalizedLimit := min(cmp.Or(int64(limit), defaultPageLimit), maxPageLimit)
	return pagination.Request{Limit: normalizedLimit, Cursor: cursor}
}
