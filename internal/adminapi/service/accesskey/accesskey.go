// Package accesskey 实现访问密钥管理 API
package accesskey

import (
	"context"
	"strings"
	"time"

	"github.com/google/wire"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	accesskeybiz "github.com/lgc202/ingate/internal/adminapi/biz/accesskey"
	adminservice "github.com/lgc202/ingate/internal/adminapi/service"
)

// ProviderSet 提供访问密钥协议服务
var ProviderSet = wire.NewSet(NewService)

// Service 实现客户端访问密钥管理 API
type Service struct {
	usecase *accesskeybiz.Usecase
}

// NewService 创建客户端访问密钥协议服务
func NewService(usecase *accesskeybiz.Usecase) *Service {
	return &Service{usecase: usecase}
}

func (s *Service) ListAccessKeys(ctx context.Context, _ *emptypb.Empty) (*adminv1.ListAccessKeysReply, error) {
	items, err := s.usecase.List(ctx)
	if err != nil {
		return nil, adminservice.OperationError(err, "查询访问密钥失败")
	}
	reply := &adminv1.ListAccessKeysReply{AccessKeys: make([]*adminv1.AccessKey, 0, len(items))}
	for i := range items {
		reply.AccessKeys = append(reply.AccessKeys, accessKeyReply(items[i]))
	}
	return reply, nil
}

func (s *Service) CreateAccessKey(ctx context.Context, request *adminv1.CreateAccessKeyRequest) (*adminv1.AccessKeySecretReply, error) {
	name := strings.TrimSpace(request.GetName())
	if name == "" {
		return nil, adminservice.BadRequest("访问密钥名称不能为空")
	}
	expiresAt, err := adminservice.OptionalTime(request.GetExpiresAt())
	if err != nil {
		return nil, err
	}
	item, secret, err := s.usecase.Create(ctx, name, request.GetAllowedModels(), expiresAt)
	if err != nil {
		return nil, adminservice.OperationError(err, "创建访问密钥失败")
	}
	return &adminv1.AccessKeySecretReply{AccessKey: accessKeyReply(item), Secret: secret}, nil
}

func (s *Service) UpdateAccessKey(ctx context.Context, request *adminv1.UpdateAccessKeyRequest) (*adminv1.AccessKeyReply, error) {
	name := strings.TrimSpace(request.GetName())
	if name == "" {
		return nil, adminservice.BadRequest("访问密钥名称不能为空")
	}
	expiresAt, err := adminservice.OptionalTime(request.GetExpiresAt())
	if err != nil {
		return nil, err
	}
	item, err := s.usecase.Update(ctx, request.GetId(), name, request.GetAllowedModels(), expiresAt)
	if err != nil {
		return nil, adminservice.OperationError(err, "更新访问密钥失败")
	}
	return &adminv1.AccessKeyReply{AccessKey: accessKeyReply(item)}, nil
}

func (s *Service) SetAccessKeyEnabled(ctx context.Context, request *adminv1.SetEnabledRequest) (*adminv1.AccessKeyReply, error) {
	item, err := s.usecase.SetEnabled(ctx, request.GetId(), request.GetEnabled())
	if err != nil {
		return nil, adminservice.OperationError(err, "更新访问密钥状态失败")
	}
	return &adminv1.AccessKeyReply{AccessKey: accessKeyReply(item)}, nil
}

func (s *Service) RotateAccessKey(ctx context.Context, request *adminv1.ResourceRequest) (*adminv1.AccessKeySecretReply, error) {
	item, secret, err := s.usecase.Rotate(ctx, request.GetId())
	if err != nil {
		return nil, adminservice.OperationError(err, "轮换访问密钥失败")
	}
	return &adminv1.AccessKeySecretReply{AccessKey: accessKeyReply(item), Secret: secret}, nil
}

func (s *Service) DeleteAccessKey(ctx context.Context, request *adminv1.ResourceRequest) (*adminv1.MutationReply, error) {
	if err := s.usecase.Delete(ctx, request.GetId()); err != nil {
		return nil, adminservice.OperationError(err, "删除访问密钥失败")
	}
	return &adminv1.MutationReply{Success: true, Id: request.GetId()}, nil
}

func accessKeyReply(item accesskeybiz.Key) *adminv1.AccessKey {
	return &adminv1.AccessKey{
		Id:            item.ID,
		Name:          item.Name,
		Prefix:        item.SecretPrefix,
		Suffix:        item.SecretSuffix,
		Enabled:       item.Enabled,
		AllowedModels: append([]string(nil), item.AllowedModels...),
		ExpiresAt:     optionalTimestamp(item.ExpiresAt),
		LastUsedAt:    optionalTimestamp(item.LastUsedAt),
		CreatedAt:     adminservice.Timestamp(item.CreatedAt),
	}
}

func optionalTimestamp(value *time.Time) *timestamppb.Timestamp {
	if value == nil {
		return nil
	}
	return adminservice.Timestamp(*value)
}
