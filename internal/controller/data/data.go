// Package data 装配 Controller 使用的数据访问实现。
package data

import (
	"github.com/google/wire"

	"github.com/lgc202/ingate/internal/controller/biz"
	"github.com/lgc202/ingate/internal/controller/conf"
	"github.com/lgc202/ingate/internal/controller/data/apiserver"
	"github.com/lgc202/ingate/internal/controller/data/apiserver/status"
	"github.com/lgc202/ingate/internal/controller/data/wasm"
	"github.com/lgc202/ingate/internal/pkg/apiserverclient"
	clientset "github.com/lgc202/ingate/internal/pkg/generated/clientset/versioned"
)

// ProviderSet 汇总 Controller 的资源读取、状态写入和 Wasm 模块存储实现。
var ProviderSet = wire.NewSet(
	NewAPIServerClient,
	NewResourceWatcher,
	NewStatusWriter,
	NewWasmModuleStore,
	wire.Bind(new(biz.ResourceWatcher), new(*apiserver.ResourceWatcher)),
	wire.Bind(new(biz.StatusWriter), new(*status.Writer)),
	wire.Bind(new(biz.WasmModuleStore), new(*wasm.Store)),
)

// NewAPIServerClient 创建 Controller 使用的声明式资源客户端。
func NewAPIServerClient(config *conf.Data_APIServer) (clientset.Interface, error) {
	return apiserver.NewClient(apiserverclient.Options{
		MasterURL:      config.GetMaster(),
		KubeconfigPath: config.GetKubeconfig(),
		BearerToken:    config.GetBearerToken(),
	})
}

// NewResourceWatcher 创建 Controller 使用的全配置域资源监听器。
func NewResourceWatcher(
	config *conf.ResourceWatch,
	client clientset.Interface,
) (*apiserver.ResourceWatcher, error) {
	return apiserver.NewResourceWatcher(client, config.GetResyncPeriod().AsDuration())
}

// NewStatusWriter 创建声明式资源状态写入器。
func NewStatusWriter(client clientset.Interface) *status.Writer {
	return status.NewWriter(client.GatewayV1())
}

// NewWasmModuleStore 创建 Controller 使用的 Wasm 模块存储。
func NewWasmModuleStore(config *conf.Data_Wasm) (*wasm.Store, error) {
	return wasm.NewStore(config)
}
