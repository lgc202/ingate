// Package caller 提供 Caller 管理和访问密钥签发 API。
package caller

import (
	"context"

	"google.golang.org/protobuf/types/known/emptypb"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	callerbiz "github.com/lgc202/ingate/internal/adminapi/biz/caller"
	adminservice "github.com/lgc202/ingate/internal/adminapi/service"
)

// Service 实现调用方管理 API。
type Service struct {
	callers *callerbiz.Usecase
}

// NewService 创建调用方协议服务。
func NewService(callers *callerbiz.Usecase) *Service {
	return &Service{callers: callers}
}

// ListCallers 返回满足筛选条件的调用方列表。
func (s *Service) ListCallers(
	ctx context.Context,
	request *adminv1.ListCallersRequest,
) (*adminv1.ListCallersResponse, error) {
	page, err := s.callers.List(
		ctx,
		adminservice.PageRequest(request.GetLimit(), request.GetCursor()),
		adminservice.ResourceFilter(
			request.GetQuery(),
			request.Enabled,
			adminv1.ResourceState_RESOURCE_STATE_UNSPECIFIED,
		),
	)
	if err != nil {
		return nil, err
	}
	callers := make([]*adminv1.Caller, len(page.Items))
	for i := range page.Items {
		callers[i] = callerResponse(&page.Items[i])
	}
	return &adminv1.ListCallersResponse{
		Callers:    callers,
		NextCursor: page.NextCursor,
	}, nil
}

// GetCaller 返回指定调用方。
func (s *Service) GetCaller(
	ctx context.Context,
	request *adminv1.GetCallerRequest,
) (*adminv1.Caller, error) {
	caller, err := s.callers.Get(ctx, request.GetId())
	if err != nil {
		return nil, err
	}
	return callerResponse(caller), nil
}

// CreateCaller 创建调用方并签发首个访问密钥。
func (s *Service) CreateCaller(
	ctx context.Context,
	request *adminv1.CreateCallerRequest,
) (*adminv1.CreateCallerResponse, error) {
	spec, err := parseCallerSpec(
		request.GetName(),
		request.GetEnabled(),
		request.GetRouteIds(),
	)
	if err != nil {
		return nil, err
	}
	accessKeyDisplayName, accessKeyExpiresAt, err := parseAccessKey(
		request.GetAccessKeyName(),
		request.GetAccessKeyExpiresAt(),
	)
	if err != nil {
		return nil, err
	}
	input := callerbiz.CreateInput{
		Spec:                 spec,
		AccessKeyDisplayName: accessKeyDisplayName,
		AccessKeyExpiresAt:   accessKeyExpiresAt,
	}
	caller, issuedAccessKey, err := s.callers.Create(ctx, input)
	if err != nil {
		return nil, err
	}
	return &adminv1.CreateCallerResponse{
		Caller:          callerResponse(caller),
		IssuedAccessKey: issuedAccessKeyResponse(issuedAccessKey),
	}, nil
}

// UpdateCaller 完整替换调用方名称、启用状态和 Route 权限，并保留已有访问密钥。
func (s *Service) UpdateCaller(
	ctx context.Context,
	request *adminv1.UpdateCallerRequest,
) (*adminv1.Caller, error) {
	spec, err := parseCallerSpec(
		request.GetName(),
		request.GetEnabled(),
		request.GetRouteIds(),
	)
	if err != nil {
		return nil, err
	}
	caller, err := s.callers.Replace(
		ctx,
		request.GetId(),
		request.GetVersion(),
		spec,
	)
	if err != nil {
		return nil, err
	}
	return callerResponse(caller), nil
}

// DeleteCaller 删除调用方。
func (s *Service) DeleteCaller(
	ctx context.Context,
	request *adminv1.DeleteCallerRequest,
) (*emptypb.Empty, error) {
	if err := s.callers.Delete(ctx, request.GetId(), request.GetVersion()); err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

// IssueAccessKey 为调用方签发一份新的访问密钥。
func (s *Service) IssueAccessKey(
	ctx context.Context,
	request *adminv1.IssueAccessKeyRequest,
) (*adminv1.IssuedAccessKey, error) {
	displayName, expiresAt, err := parseAccessKey(
		request.GetName(),
		request.GetExpiresAt(),
	)
	if err != nil {
		return nil, err
	}
	input := callerbiz.IssueAccessKeyInput{
		ExpectedGeneration: request.GetVersion(),
		DisplayName:        displayName,
		ExpiresAt:          expiresAt,
	}
	issuedAccessKey, err := s.callers.IssueAccessKey(ctx, request.GetCallerId(), input)
	if err != nil {
		return nil, err
	}
	return issuedAccessKeyResponse(issuedAccessKey), nil
}

// DisableAccessKey 停用调用方的一份访问密钥。
func (s *Service) DisableAccessKey(
	ctx context.Context,
	request *adminv1.DisableAccessKeyRequest,
) (*adminv1.Caller, error) {
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
