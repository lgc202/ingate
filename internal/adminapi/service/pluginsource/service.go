// Package pluginsource 提供插件源管理和目录同步 API
package pluginsource

import (
	"context"

	"google.golang.org/protobuf/types/known/emptypb"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	pluginsourcebiz "github.com/lgc202/ingate/internal/adminapi/biz/pluginsource"
)

// Service 实现插件源管理 API
type Service struct {
	sources *pluginsourcebiz.Service
}

// NewService 创建插件源协议服务
func NewService(sources *pluginsourcebiz.Service) *Service {
	return &Service{sources: sources}
}

func (s *Service) ListPluginSources(
	ctx context.Context,
	_ *adminv1.ListPluginSourcesRequest,
) (*adminv1.ListPluginSourcesResponse, error) {
	sources, err := s.sources.List(ctx)
	if err != nil {
		return nil, err
	}
	response := &adminv1.ListPluginSourcesResponse{Sources: make([]*adminv1.PluginSource, 0, len(sources))}
	for _, source := range sources {
		response.Sources = append(response.Sources, sourceResponse(source))
	}
	return response, nil
}

func (s *Service) GetPluginSource(
	ctx context.Context,
	request *adminv1.GetPluginSourceRequest,
) (*adminv1.PluginSource, error) {
	source, err := s.sources.Get(ctx, request.GetId())
	if err != nil {
		return nil, err
	}
	return sourceResponse(source), nil
}

func (s *Service) CreatePluginSource(
	ctx context.Context,
	request *adminv1.CreatePluginSourceRequest,
) (*adminv1.PluginSource, error) {
	spec, err := createSpec(request)
	if err != nil {
		return nil, err
	}
	source, err := s.sources.Create(ctx, spec)
	if err != nil {
		return nil, err
	}
	return sourceResponse(source), nil
}

func (s *Service) UpdatePluginSource(
	ctx context.Context,
	request *adminv1.UpdatePluginSourceRequest,
) (*adminv1.PluginSource, error) {
	spec, err := updateSpec(request)
	if err != nil {
		return nil, err
	}
	source, err := s.sources.Update(ctx, request.GetId(), request.GetVersion(), spec)
	if err != nil {
		return nil, err
	}
	return sourceResponse(source), nil
}

func (s *Service) DeletePluginSource(
	ctx context.Context,
	request *adminv1.DeletePluginSourceRequest,
) (*emptypb.Empty, error) {
	if err := s.sources.Delete(ctx, request.GetId(), request.GetVersion()); err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

func (s *Service) SyncPluginSource(
	ctx context.Context,
	request *adminv1.SyncPluginSourceRequest,
) (*adminv1.PluginSource, error) {
	source, err := s.sources.Sync(ctx, request.GetId())
	if err != nil {
		return nil, err
	}
	return sourceResponse(source), nil
}
