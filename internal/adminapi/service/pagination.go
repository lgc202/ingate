package service

import (
	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	"github.com/lgc202/ingate/internal/adminapi/biz"
)

const (
	defaultPageLimit = 100
	maxPageLimit     = 200
)

// PageRequest 把控制台分页参数转换为不依赖存储实现的 biz 参数
func PageRequest(limit int32, cursor string) biz.PageRequest {
	normalizedLimit := int64(limit)
	if normalizedLimit == 0 {
		normalizedLimit = defaultPageLimit
	}
	if normalizedLimit > maxPageLimit {
		normalizedLimit = maxPageLimit
	}
	return biz.PageRequest{Limit: normalizedLimit, Cursor: cursor}
}

// PageInfo 构造控制台分页响应
func PageInfo(nextCursor string) *adminv1.PageInfo {
	return &adminv1.PageInfo{NextPageToken: nextCursor}
}
