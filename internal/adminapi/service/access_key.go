package service

import (
	"context"
	"strings"
	"time"

	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	"github.com/lgc202/ingate/internal/adminapi/biz"
)

// AccessKeyService 实现客户端访问密钥管理 API
type AccessKeyService struct {
	usecase *biz.AccessKeyUsecase
}

// NewAccessKeyService 创建客户端访问密钥协议服务
func NewAccessKeyService(usecase *biz.AccessKeyUsecase) *AccessKeyService {
	return &AccessKeyService{usecase: usecase}
}

func (s *AccessKeyService) ListAccessKeys(ctx context.Context, _ *emptypb.Empty) (*adminv1.ListAccessKeysReply, error) {
	items, err := s.usecase.List(ctx)
	if err != nil {
		return nil, operationError(err, "查询访问密钥失败")
	}
	reply := &adminv1.ListAccessKeysReply{AccessKeys: make([]*adminv1.AccessKey, 0, len(items))}
	for i := range items {
		reply.AccessKeys = append(reply.AccessKeys, accessKeyReply(items[i]))
	}
	return reply, nil
}

func (s *AccessKeyService) CreateAccessKey(ctx context.Context, request *adminv1.CreateAccessKeyRequest) (*adminv1.AccessKeySecretReply, error) {
	name := strings.TrimSpace(request.GetName())
	if name == "" {
		return nil, badRequest("访问密钥名称不能为空")
	}
	expiresAt, err := optionalTime(request.GetExpiresAt())
	if err != nil {
		return nil, err
	}
	item, secret, err := s.usecase.Create(ctx, name, request.GetAllowedModels(), expiresAt)
	if err != nil {
		return nil, operationError(err, "创建访问密钥失败")
	}
	return &adminv1.AccessKeySecretReply{AccessKey: accessKeyReply(item), Secret: secret}, nil
}

func (s *AccessKeyService) UpdateAccessKey(ctx context.Context, request *adminv1.UpdateAccessKeyRequest) (*adminv1.AccessKeyReply, error) {
	if err := validateID(request.GetId()); err != nil {
		return nil, err
	}
	name := strings.TrimSpace(request.GetName())
	if name == "" {
		return nil, badRequest("访问密钥名称不能为空")
	}
	expiresAt, err := optionalTime(request.GetExpiresAt())
	if err != nil {
		return nil, err
	}
	item, err := s.usecase.Update(ctx, request.GetId(), name, request.GetAllowedModels(), expiresAt)
	if err != nil {
		return nil, operationError(err, "更新访问密钥失败")
	}
	return &adminv1.AccessKeyReply{AccessKey: accessKeyReply(item)}, nil
}

func (s *AccessKeyService) SetAccessKeyEnabled(ctx context.Context, request *adminv1.SetEnabledRequest) (*adminv1.AccessKeyReply, error) {
	if err := validateID(request.GetId()); err != nil {
		return nil, err
	}
	if request.Enabled == nil {
		return nil, badRequest("启用状态不能为空")
	}
	item, err := s.usecase.SetEnabled(ctx, request.GetId(), request.GetEnabled())
	if err != nil {
		return nil, operationError(err, "更新访问密钥状态失败")
	}
	return &adminv1.AccessKeyReply{AccessKey: accessKeyReply(item)}, nil
}

func (s *AccessKeyService) RotateAccessKey(ctx context.Context, request *adminv1.ResourceRequest) (*adminv1.AccessKeySecretReply, error) {
	if err := validateID(request.GetId()); err != nil {
		return nil, err
	}
	item, secret, err := s.usecase.Rotate(ctx, request.GetId())
	if err != nil {
		return nil, operationError(err, "轮换访问密钥失败")
	}
	return &adminv1.AccessKeySecretReply{AccessKey: accessKeyReply(item), Secret: secret}, nil
}

func (s *AccessKeyService) DeleteAccessKey(ctx context.Context, request *adminv1.ResourceRequest) (*adminv1.MutationReply, error) {
	if err := validateID(request.GetId()); err != nil {
		return nil, err
	}
	if err := s.usecase.Delete(ctx, request.GetId()); err != nil {
		return nil, operationError(err, "删除访问密钥失败")
	}
	return &adminv1.MutationReply{Success: true, Id: request.GetId()}, nil
}

func accessKeyReply(item biz.AccessKey) *adminv1.AccessKey {
	return &adminv1.AccessKey{
		Id:            item.ID,
		Name:          item.Name,
		Prefix:        item.SecretPrefix,
		Suffix:        item.SecretSuffix,
		Enabled:       item.Enabled,
		AllowedModels: append([]string(nil), item.AllowedModels...),
		ExpiresAt:     optionalTimestamp(item.ExpiresAt),
		LastUsedAt:    optionalTimestamp(item.LastUsedAt),
		CreatedAt:     timestamp(item.CreatedAt),
	}
}

func optionalTimestamp(value *time.Time) *timestamppb.Timestamp {
	if value == nil {
		return nil
	}
	return timestamp(*value)
}
