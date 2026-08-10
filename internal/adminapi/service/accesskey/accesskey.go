// Package accesskey 实现访问密钥管理 API
package accesskey

import (
	"context"
	"strconv"
	"strings"
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	accesskeybiz "github.com/lgc202/ingate/internal/adminapi/biz/accesskey"
	adminservice "github.com/lgc202/ingate/internal/adminapi/service"
)

// Service 实现客户端访问密钥管理 API
type Service struct {
	usecase *accesskeybiz.Usecase
}

// NewService 创建客户端访问密钥协议服务
func NewService(usecase *accesskeybiz.Usecase) *Service {
	return &Service{usecase: usecase}
}

func (s *Service) ListAccessKeys(ctx context.Context, request *adminv1.ListRequest) (*adminv1.ListAccessKeysReply, error) {
	result, err := s.usecase.List(ctx, adminservice.PageRequest(request.GetPageSize(), request.GetPageToken()))
	if err != nil {
		return nil, err
	}
	reply := &adminv1.ListAccessKeysReply{AccessKeys: make([]*adminv1.AccessKey, 0, len(result.Items)), Page: adminservice.PageInfo(result.NextCursor)}
	for i := range result.Items {
		reply.AccessKeys = append(reply.AccessKeys, newAccessKeyReply(result.Items[i]))
	}
	return reply, nil
}

func (s *Service) CreateAccessKey(ctx context.Context, request *adminv1.CreateAccessKeyRequest) (*adminv1.AccessKeySecretReply, error) {
	name := strings.TrimSpace(request.GetName())
	if name == "" {
		return nil, adminservice.BadRequest("访问密钥名称不能为空")
	}
	expiresAt, err := adminservice.TimeFromProto(request.GetExpiresAt())
	if err != nil {
		return nil, err
	}
	item, secret, err := s.usecase.Create(ctx, name, request.GetAllowedModels(), expiresAt)
	if err != nil {
		return nil, err
	}
	return &adminv1.AccessKeySecretReply{AccessKey: newAccessKeyReply(item), Secret: secret}, nil
}

func (s *Service) UpdateAccessKey(ctx context.Context, request *adminv1.UpdateAccessKeyRequest) (*adminv1.AccessKeyReply, error) {
	name := strings.TrimSpace(request.GetName())
	if name == "" {
		return nil, adminservice.BadRequest("访问密钥名称不能为空")
	}
	expiresAt, err := adminservice.TimeFromProto(request.GetExpiresAt())
	if err != nil {
		return nil, err
	}
	version, err := parseVersion(request.GetVersion())
	if err != nil {
		return nil, err
	}
	item, err := s.usecase.Update(ctx, request.GetId(), version, name, request.GetAllowedModels(), expiresAt)
	if err != nil {
		return nil, err
	}
	return &adminv1.AccessKeyReply{AccessKey: newAccessKeyReply(item)}, nil
}

func (s *Service) SetAccessKeyEnabled(ctx context.Context, request *adminv1.SetAccessKeyEnabledRequest) (*adminv1.AccessKeyReply, error) {
	version, err := parseVersion(request.GetVersion())
	if err != nil {
		return nil, err
	}
	item, err := s.usecase.SetEnabled(ctx, request.GetId(), version, request.GetEnabled())
	if err != nil {
		return nil, err
	}
	return &adminv1.AccessKeyReply{AccessKey: newAccessKeyReply(item)}, nil
}

func (s *Service) RotateAccessKey(ctx context.Context, request *adminv1.AccessKeyActionRequest) (*adminv1.AccessKeySecretReply, error) {
	version, err := parseVersion(request.GetVersion())
	if err != nil {
		return nil, err
	}
	item, secret, err := s.usecase.Rotate(ctx, request.GetId(), version)
	if err != nil {
		return nil, err
	}
	return &adminv1.AccessKeySecretReply{AccessKey: newAccessKeyReply(item), Secret: secret}, nil
}

func (s *Service) DeleteAccessKey(ctx context.Context, request *adminv1.AccessKeyActionRequest) (*adminv1.MutationReply, error) {
	version, err := parseVersion(request.GetVersion())
	if err != nil {
		return nil, err
	}
	if err := s.usecase.Delete(ctx, request.GetId(), version); err != nil {
		return nil, err
	}
	return &adminv1.MutationReply{Success: true, Id: request.GetId()}, nil
}

func newAccessKeyReply(item accesskeybiz.Key) *adminv1.AccessKey {
	return &adminv1.AccessKey{
		Id:            item.ID,
		Version:       strconv.FormatInt(item.Version, 10),
		Name:          item.Name,
		Prefix:        item.SecretPrefix,
		Suffix:        item.SecretSuffix,
		Enabled:       item.Enabled,
		AllowedModels: append([]string(nil), item.AllowedModels...),
		ExpiresAt:     optionalTimestamp(item.ExpiresAt),
		LastUsedAt:    optionalTimestamp(item.LastUsedAt),
		CreatedAt:     adminservice.NewTimestamp(item.CreatedAt),
	}
}

func parseVersion(value string) (int64, error) {
	version, err := strconv.ParseInt(value, 10, 64)
	if err != nil || version < 1 {
		return 0, adminservice.BadRequest("访问密钥版本格式不正确")
	}
	return version, nil
}

func optionalTimestamp(value *time.Time) *timestamppb.Timestamp {
	if value == nil {
		return nil
	}
	return adminservice.NewTimestamp(*value)
}
