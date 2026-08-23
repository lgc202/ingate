// Package caller 提供 Caller 管理和访问密钥签发 API
package caller

import (
	"context"

	"google.golang.org/protobuf/types/known/emptypb"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	callerbiz "github.com/lgc202/ingate/internal/adminapi/biz/caller"
	adminservice "github.com/lgc202/ingate/internal/adminapi/service"
)

// Service 实现调用方管理 API
type Service struct {
	callers *callerbiz.Service
}

// NewService 创建调用方协议服务
func NewService(callers *callerbiz.Service) *Service {
	return &Service{callers: callers}
}

func (s *Service) ListCallers(ctx context.Context, request *adminv1.ListCallersRequest) (*adminv1.ListCallersResponse, error) {
	page, err := s.callers.List(
		ctx,
		adminservice.PageRequest(request.GetLimit(), request.GetCursor()),
		adminservice.ResourceFilter(request.GetQuery(), request.Enabled, adminv1.ResourceState_RESOURCE_STATE_UNSPECIFIED),
	)
	if err != nil {
		return nil, err
	}
	response := &adminv1.ListCallersResponse{
		Callers:    make([]*adminv1.Caller, 0, len(page.Items)),
		NextCursor: page.NextCursor,
	}
	for i := range page.Items {
		response.Callers = append(response.Callers, callerResponse(&page.Items[i]))
	}
	return response, nil
}

func (s *Service) GetCaller(ctx context.Context, request *adminv1.GetCallerRequest) (*adminv1.Caller, error) {
	caller, err := s.callers.Get(ctx, request.GetId())
	if err != nil {
		return nil, err
	}
	return callerResponse(caller), nil
}

func (s *Service) CreateCaller(ctx context.Context, request *adminv1.CreateCallerRequest) (*adminv1.CreateCallerResponse, error) {
	spec, err := createSpec(request)
	if err != nil {
		return nil, err
	}
	keyName, expiresAt, err := accessKey(request.GetAccessKeyName(), request.GetAccessKeyExpiresAt())
	if err != nil {
		return nil, err
	}
	caller, issued, err := s.callers.Create(ctx, callerbiz.CreateInput{
		Spec:             spec,
		AccessKeyName:    keyName,
		AccessKeyExpires: expiresAt,
	})
	if err != nil {
		return nil, err
	}
	return &adminv1.CreateCallerResponse{
		Caller:          callerResponse(caller),
		IssuedAccessKey: issuedKeyResponse(issued),
	}, nil
}

func (s *Service) UpdateCaller(ctx context.Context, request *adminv1.UpdateCallerRequest) (*adminv1.Caller, error) {
	spec, err := updateSpec(request)
	if err != nil {
		return nil, err
	}
	caller, err := s.callers.Update(ctx, request.GetId(), request.GetVersion(), spec)
	if err != nil {
		return nil, err
	}
	return callerResponse(caller), nil
}

func (s *Service) DeleteCaller(ctx context.Context, request *adminv1.DeleteCallerRequest) (*emptypb.Empty, error) {
	if err := s.callers.Delete(ctx, request.GetId(), request.GetVersion()); err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

func (s *Service) IssueAccessKey(ctx context.Context, request *adminv1.IssueAccessKeyRequest) (*adminv1.IssuedAccessKey, error) {
	name, expiresAt, err := accessKey(request.GetName(), request.GetExpiresAt())
	if err != nil {
		return nil, err
	}
	issued, err := s.callers.IssueAccessKey(ctx, request.GetCallerId(), request.GetVersion(), name, expiresAt)
	if err != nil {
		return nil, err
	}
	return issuedKeyResponse(issued), nil
}

func (s *Service) DisableAccessKey(ctx context.Context, request *adminv1.DisableAccessKeyRequest) (*adminv1.Caller, error) {
	caller, err := s.callers.DisableAccessKey(
		ctx,
		request.GetCallerId(),
		request.GetAccessKeyId(),
		request.GetVersion(),
	)
	if err != nil {
		return nil, err
	}
	return callerResponse(caller), nil
}
