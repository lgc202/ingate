// Package wasmplugin 提供插件安装、升级和卸载 API
package wasmplugin

import (
	"context"
	"strings"

	"google.golang.org/protobuf/types/known/emptypb"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	wasmpluginbiz "github.com/lgc202/ingate/internal/adminapi/biz/wasmplugin"
	adminservice "github.com/lgc202/ingate/internal/adminapi/service"
	resource "github.com/lgc202/ingate/internal/pkg/apis/gateway/v1"
)

// Service 实现 WasmPlugin 管理 API
type Service struct {
	plugins *wasmpluginbiz.Service
}

// NewService 创建 WasmPlugin 协议服务
func NewService(plugins *wasmpluginbiz.Service) *Service {
	return &Service{plugins: plugins}
}

func (s *Service) ListWasmPluginCatalog(
	context.Context,
	*adminv1.ListWasmPluginCatalogRequest,
) (*adminv1.ListWasmPluginCatalogResponse, error) {
	return catalogResponse(s.plugins.Catalog()), nil
}

func catalogResponse(snapshot wasmpluginbiz.CatalogSnapshot) *adminv1.ListWasmPluginCatalogResponse {
	response := &adminv1.ListWasmPluginCatalogResponse{
		Plugins: make([]*adminv1.WasmPluginCatalogItem, 0, len(snapshot.Items)),
	}
	for _, item := range snapshot.Items {
		response.Plugins = append(response.Plugins, catalogItemResponse(item))
	}
	return response
}

func (s *Service) ListWasmPlugins(
	ctx context.Context,
	request *adminv1.ListWasmPluginsRequest,
) (*adminv1.ListWasmPluginsResponse, error) {
	page, err := s.plugins.List(ctx, adminservice.PageRequest(request.GetLimit(), request.GetCursor()))
	if err != nil {
		return nil, err
	}
	response := &adminv1.ListWasmPluginsResponse{
		Plugins:    make([]*adminv1.WasmPlugin, 0, len(page.Items)),
		NextCursor: page.NextCursor,
	}
	for i := range page.Items {
		response.Plugins = append(response.Plugins, s.pluginResponse(&page.Items[i]))
	}
	return response, nil
}

func (s *Service) GetWasmPlugin(
	ctx context.Context,
	request *adminv1.GetWasmPluginRequest,
) (*adminv1.WasmPlugin, error) {
	plugin, err := s.plugins.Get(ctx, request.GetId())
	if err != nil {
		return nil, err
	}
	return s.pluginResponse(plugin), nil
}

func (s *Service) CreateWasmPlugin(
	ctx context.Context,
	request *adminv1.CreateWasmPluginRequest,
) (*adminv1.WasmPlugin, error) {
	packageName := strings.TrimSpace(request.GetPackageName())
	if packageName == "" {
		return nil, adminservice.BadRequest("请选择要安装的插件")
	}
	plugin, err := s.plugins.Install(ctx, packageName)
	if err != nil {
		return nil, err
	}
	return s.pluginResponse(plugin), nil
}

func (s *Service) UpdateWasmPlugin(
	ctx context.Context,
	request *adminv1.UpdateWasmPluginRequest,
) (*adminv1.WasmPlugin, error) {
	plugin, err := s.plugins.Upgrade(ctx, request.GetId(), request.GetVersion())
	if err != nil {
		return nil, err
	}
	return s.pluginResponse(plugin), nil
}

func (s *Service) DeleteWasmPlugin(
	ctx context.Context,
	request *adminv1.DeleteWasmPluginRequest,
) (*emptypb.Empty, error) {
	if err := s.plugins.Delete(ctx, request.GetId(), request.GetVersion()); err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

func (s *Service) pluginResponse(plugin *resource.WasmPlugin) *adminv1.WasmPlugin {
	latestVersion, upgradeAvailable := s.plugins.UpgradeVersion(plugin.Spec.Package, plugin.Spec.Version)
	return pluginResponse(plugin, latestVersion, upgradeAvailable)
}
