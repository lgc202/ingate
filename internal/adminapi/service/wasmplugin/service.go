// Package wasmplugin 提供插件安装、升级和卸载 API。
package wasmplugin

import (
	"context"
	"strings"

	"google.golang.org/protobuf/types/known/emptypb"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	"github.com/lgc202/ingate/internal/adminapi/biz/plugin"
	wasmpluginbiz "github.com/lgc202/ingate/internal/adminapi/biz/wasmplugin"
	adminservice "github.com/lgc202/ingate/internal/adminapi/service/protocol"
	resource "github.com/lgc202/ingate/internal/pkg/apis/gateway/v1"
)

// Service 实现 WasmPlugin 管理 API。
type Service struct {
	plugins *wasmpluginbiz.Usecase
}

// NewService 创建 WasmPlugin 协议服务。
func NewService(plugins *wasmpluginbiz.Usecase) *Service {
	return &Service{plugins: plugins}
}

// ListWasmPluginCatalog 返回当前可安装的插件目录。
func (s *Service) ListWasmPluginCatalog(
	context.Context,
	*adminv1.ListWasmPluginCatalogRequest,
) (*adminv1.ListWasmPluginCatalogResponse, error) {
	return catalogResponse(s.plugins.ListCatalog()), nil
}

// ListWasmPlugins 返回满足筛选条件的已安装插件。
func (s *Service) ListWasmPlugins(
	ctx context.Context,
	request *adminv1.ListWasmPluginsRequest,
) (*adminv1.ListWasmPluginsResponse, error) {
	page, err := s.plugins.List(
		ctx,
		adminservice.PageRequest(request.GetLimit(), request.GetCursor()),
		adminservice.ResourceFilter(request.GetQuery(), nil, request.GetState()),
	)
	if err != nil {
		return nil, err
	}
	plugins := make([]*adminv1.WasmPlugin, len(page.Items))
	for i := range page.Items {
		plugins[i] = s.pluginResponse(&page.Items[i], nil)
	}
	return &adminv1.ListWasmPluginsResponse{
		Plugins:    plugins,
		NextCursor: page.NextCursor,
	}, nil
}

// GetWasmPlugin 返回指定已安装插件及其策略引用。
func (s *Service) GetWasmPlugin(
	ctx context.Context,
	request *adminv1.GetWasmPluginRequest,
) (*adminv1.WasmPlugin, error) {
	plugin, err := s.plugins.Get(ctx, request.GetId())
	if err != nil {
		return nil, err
	}
	usages, err := s.plugins.PolicyUsages(ctx, plugin.Spec.Package)
	if err != nil {
		return nil, err
	}
	return s.pluginResponse(plugin, usages), nil
}

// CreateWasmPlugin 安装指定来源的目录插件。
func (s *Service) CreateWasmPlugin(
	ctx context.Context,
	request *adminv1.CreateWasmPluginRequest,
) (*adminv1.WasmPlugin, error) {
	sourceID := strings.TrimSpace(request.GetSourceId())
	if sourceID == "" {
		return nil, adminv1.ErrorInvalidArgument("请选择插件源")
	}
	packageName := strings.TrimSpace(request.GetPackageName())
	if packageName == "" {
		return nil, adminv1.ErrorInvalidArgument("请选择要安装的插件")
	}
	plugin, err := s.plugins.Install(ctx, sourceID, packageName)
	if err != nil {
		return nil, err
	}
	return s.pluginResponse(plugin, nil), nil
}

// UpdateWasmPlugin 将已安装插件升级到当前目录推荐版本。
func (s *Service) UpdateWasmPlugin(
	ctx context.Context,
	request *adminv1.UpdateWasmPluginRequest,
) (*adminv1.WasmPlugin, error) {
	plugin, err := s.plugins.Upgrade(ctx, request.GetId(), request.GetVersion())
	if err != nil {
		return nil, err
	}
	return s.pluginResponse(plugin, nil), nil
}

// DeleteWasmPlugin 卸载指定插件。
func (s *Service) DeleteWasmPlugin(
	ctx context.Context,
	request *adminv1.DeleteWasmPluginRequest,
) (*emptypb.Empty, error) {
	if err := s.plugins.Delete(ctx, request.GetId(), request.GetVersion()); err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

func (s *Service) pluginResponse(
	plugin *resource.WasmPlugin,
	usages []plugin.PluginPolicyUsage,
) *adminv1.WasmPlugin {
	catalog := s.plugins.CatalogInfo(plugin)
	return pluginResponse(plugin, catalog, usages)
}
