// Package pluginsource 提供插件源管理和目录同步 API。
package pluginsource

import (
	"context"

	"google.golang.org/protobuf/types/known/emptypb"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	pluginsourcebiz "github.com/lgc202/ingate/internal/adminapi/biz/pluginsource"
	adminservice "github.com/lgc202/ingate/internal/adminapi/service/protocol"
)

// Service 实现插件源管理 API。
type Service struct {
	sources *pluginsourcebiz.Usecase
}

// NewService 创建插件源协议服务。
func NewService(sources *pluginsourcebiz.Usecase) *Service {
	return &Service{sources: sources}
}

// ListPluginSources 返回满足筛选条件的插件源。
func (s *Service) ListPluginSources(
	ctx context.Context,
	request *adminv1.ListPluginSourcesRequest,
) (*adminv1.ListPluginSourcesResponse, error) {
	filter := pluginsourcebiz.ListFilter{
		Query:   request.GetQuery(),
		Enabled: request.Enabled,
		State:   syncStateFilter(request.GetSyncState()),
	}
	page, err := s.sources.List(
		ctx,
		adminservice.PageRequest(request.GetLimit(), request.GetCursor()),
		filter,
	)
	if err != nil {
		return nil, err
	}
	sources := make([]*adminv1.PluginSource, len(page.Items))
	for i := range page.Items {
		sources[i] = sourceResponse(page.Items[i])
	}
	return &adminv1.ListPluginSourcesResponse{
		Sources:    sources,
		NextCursor: page.NextCursor,
	}, nil
}

// GetPluginSource 返回指定插件源。
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

// CreatePluginSource 创建自定义插件源。
func (s *Service) CreatePluginSource(
	ctx context.Context,
	request *adminv1.CreatePluginSourceRequest,
) (*adminv1.PluginSource, error) {
	spec, err := parsePluginSourceSpec(
		request.GetName(),
		request.GetUrl(),
		request.GetEnabled(),
	)
	if err != nil {
		return nil, err
	}
	source, err := s.sources.Create(ctx, spec)
	if err != nil {
		return nil, err
	}
	return sourceResponse(source), nil
}

// UpdatePluginSource 完整替换自定义插件源配置。
func (s *Service) UpdatePluginSource(
	ctx context.Context,
	request *adminv1.UpdatePluginSourceRequest,
) (*adminv1.PluginSource, error) {
	spec, err := parsePluginSourceSpec(
		request.GetName(),
		request.GetUrl(),
		request.GetEnabled(),
	)
	if err != nil {
		return nil, err
	}
	source, err := s.sources.Replace(
		ctx,
		request.GetId(),
		request.GetVersion(),
		spec,
	)
	if err != nil {
		return nil, err
	}
	return sourceResponse(source), nil
}

// DeletePluginSource 删除自定义插件源。
func (s *Service) DeletePluginSource(
	ctx context.Context,
	request *adminv1.DeletePluginSourceRequest,
) (*emptypb.Empty, error) {
	if err := s.sources.Delete(ctx, request.GetId(), request.GetVersion()); err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

// SyncPluginSource 立即同步指定插件源。
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

func syncStateFilter(state adminv1.PluginSourceSyncState) pluginsourcebiz.SyncState {
	switch state {
	case adminv1.PluginSourceSyncState_PLUGIN_SOURCE_SYNC_STATE_READY:
		return pluginsourcebiz.SyncStateReady
	case adminv1.PluginSourceSyncState_PLUGIN_SOURCE_SYNC_STATE_ERROR:
		return pluginsourcebiz.SyncStateError
	case adminv1.PluginSourceSyncState_PLUGIN_SOURCE_SYNC_STATE_DISABLED:
		return pluginsourcebiz.SyncStateDisabled
	case adminv1.PluginSourceSyncState_PLUGIN_SOURCE_SYNC_STATE_NOT_SYNCED:
		return pluginsourcebiz.SyncStateNotSynced
	default:
		return ""
	}
}
