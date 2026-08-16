// Package request 提供控制台请求记录查询 API
package request

import (
	"context"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	requestbiz "github.com/lgc202/ingate/internal/adminapi/biz/request"
)

// Service 实现请求记录管理 API
type Service struct {
	business *requestbiz.Service
}

// NewService 创建请求记录协议服务
func NewService(business *requestbiz.Service) *Service {
	return &Service{business: business}
}

// ListRequestRecords 按时间倒序查询请求记录
func (s *Service) ListRequestRecords(
	ctx context.Context,
	request *adminv1.ListRequestRecordsRequest,
) (*adminv1.ListRequestRecordsResponse, error) {
	options, err := listOptions(request)
	if err != nil {
		return nil, err
	}
	page, err := s.business.List(ctx, options)
	if err != nil {
		return nil, err
	}
	records := make([]*adminv1.RequestRecord, 0, len(page.Records))
	for i := range page.Records {
		records = append(records, recordResponse(&page.Records[i]))
	}
	return &adminv1.ListRequestRecordsResponse{
		Records:       records,
		NextPageToken: page.NextPageToken,
	}, nil
}

// GetRequestRecord 查询单次请求记录
func (s *Service) GetRequestRecord(
	ctx context.Context,
	request *adminv1.GetRequestRecordRequest,
) (*adminv1.RequestRecord, error) {
	startedAt, err := requiredTimestamp(request.GetStartedAt(), "请选择请求开始时间")
	if err != nil {
		return nil, err
	}
	record, err := s.business.Get(ctx, request.GetId(), startedAt)
	if err != nil {
		return nil, err
	}
	return recordResponse(record), nil
}
