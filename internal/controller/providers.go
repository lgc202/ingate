package controller

import (
	"context"
	"log/slog"

	cachev3 "github.com/envoyproxy/go-control-plane/pkg/cache/v3"

	"github.com/lgc202/ingate/internal/controller/biz"
	"github.com/lgc202/ingate/internal/controller/biz/delivery"
	"github.com/lgc202/ingate/internal/controller/conf"
	controllerdata "github.com/lgc202/ingate/internal/controller/data/apiserver"
	controllerstatus "github.com/lgc202/ingate/internal/controller/data/apiserver/status"
	"github.com/lgc202/ingate/internal/controller/server/xds"
	clientset "github.com/lgc202/ingate/pkg/generated/clientset/versioned"
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
	logger *slog.Logger,
) *biz.Controller {
	return biz.NewController(
		resources,
		statusWriter,
		configDelivery,
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
	return xds.NewService(context.Background(), cache, callbacks, xdsLogger)
}
