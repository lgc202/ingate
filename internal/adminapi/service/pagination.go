package service

import (
	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	"github.com/lgc202/ingate/internal/adminapi/biz"
)

const defaultPageSize = 100

// PageRequest 把控制台分页参数转换为不依赖存储实现的用例参数
func PageRequest(request *adminv1.ListRequest) biz.PageRequest {
	size := int64(request.GetPageSize())
	if size == 0 {
		size = defaultPageSize
	}
	return biz.PageRequest{Size: size, Token: request.GetPageToken()}
}

// PageInfo 构造控制台分页响应
func PageInfo(nextToken string) *adminv1.PageInfo {
	return &adminv1.PageInfo{NextPageToken: nextToken}
}
