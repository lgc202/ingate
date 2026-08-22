package controller

import (
	"log/slog"
	"net/http"

	cachev3 "github.com/envoyproxy/go-control-plane/pkg/cache/v3"

	"github.com/lgc202/ingate/internal/controller/biz"
	"github.com/lgc202/ingate/internal/controller/biz/delivery"
	"github.com/lgc202/ingate/internal/controller/conf"
	controllerdata "github.com/lgc202/ingate/internal/controller/data/apiserver"
	controllerstatus "github.com/lgc202/ingate/internal/controller/data/apiserver/status"
	controllerwasm "github.com/lgc202/ingate/internal/controller/data/wasm"
	"github.com/lgc202/ingate/internal/controller/server/xds"
	clientset "github.com/lgc202/ingate/internal/pkg/generated/clientset/versioned"
)

func newAPIClient(config *conf.Data_APIServer) (clientset.Interface, error) {
	return controllerdata.NewClient(config.GetMaster(), config.GetKubeconfig())
}

func newResourceWatcher(
	config *conf.ResourceWatch,
	client clientset.Interface,
) (*controllerdata.ResourceWatcher, error) {
	return controllerdata.NewResourceWatcher(client, config.GetResyncPeriod().AsDuration())
}

func newStatusWriter(client clientset.Interface) *controllerstatus.Writer {
	return controllerstatus.NewWriter(client.GatewayV1())
}

func newWasmModuleStore(config *conf.Data_Wasm) (*controllerwasm.Store, error) {
	return controllerwasm.NewStore(config)
}

func asWasmModuleStore(store *controllerwasm.Store) biz.WasmModuleStore {
	return store
}

func newWasmModuleHandler(store *controllerwasm.Store) http.Handler {
	return store
}

func newSnapshotCache(logger *slog.Logger) cachev3.SnapshotCache {
	return xds.NewSnapshotCache(xds.NewSlogLogger(logger.With("component", "xds")))
}

func newXDSPublisher(cache cachev3.SnapshotCache) *xds.Publisher {
	return xds.NewPublisher(cache)
}

func newDelivery(config *conf.Delivery, publisher *xds.Publisher) (*delivery.Delivery, error) {
	return delivery.New(publisher, delivery.Options{
		ACKTimeout:          config.GetCandidateAckTimeout().AsDuration(),
		NACKRollbackTimeout: config.GetNackRollbackTimeout().AsDuration(),
	})
}

func newController(
	resources *controllerdata.ResourceWatcher,
	statusWriter *controllerstatus.Writer,
	configDelivery *delivery.Delivery,
	wasmModules biz.WasmModuleStore,
	logger *slog.Logger,
) *biz.Controller {
	return biz.NewController(
		resources,
		statusWriter,
		configDelivery,
		wasmModules,
		logger.With("component", "controller"),
	)
}

func newXDSService(
	cache cachev3.SnapshotCache,
	configDelivery *delivery.Delivery,
	logger *slog.Logger,
) *xds.Service {
	xdsLogger := xds.NewSlogLogger(logger.With("component", "xds"))
	callbacks := xds.NewCallbacks(configDelivery.HandleXDSEvent)
	return xds.NewService(cache, callbacks, xdsLogger)
}
