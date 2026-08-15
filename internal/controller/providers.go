package controller

import (
	"context"
	"fmt"
	"log/slog"

	cachev3 "github.com/envoyproxy/go-control-plane/pkg/cache/v3"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/lgc202/ingate/internal/controller/conf"
	"github.com/lgc202/ingate/internal/controller/delivery"
	"github.com/lgc202/ingate/internal/controller/reconcile"
	"github.com/lgc202/ingate/internal/controller/xds"
	clientset "github.com/lgc202/ingate/pkg/generated/clientset/versioned"
)

func newAPIClient(config *conf.Data_APIServer) (clientset.Interface, error) {
	restConfig, err := clientcmd.BuildConfigFromFlags(config.GetMaster(), config.GetKubeconfig())
	if err != nil {
		return nil, fmt.Errorf("build API Server client config: %w", err)
	}
	client, err := clientset.NewForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("create API Server resource client: %w", err)
	}
	return client, nil
}

func newSnapshotCache(logger *slog.Logger) cachev3.SnapshotCache {
	return xds.NewSnapshotCache(xds.NewSlogLogger(logger.With("component", "xds")))
}

func newDelivery(config *conf.Delivery, cache cachev3.SnapshotCache) (*delivery.Delivery, error) {
	return delivery.New(cache, delivery.Options{
		ACKTimeout:          config.GetCandidateAckTimeout().AsDuration(),
		NACKRollbackTimeout: config.GetNackRollbackTimeout().AsDuration(),
	})
}

func newReconciler(
	config *conf.ResourceWatch,
	client clientset.Interface,
	configDelivery *delivery.Delivery,
	logger *slog.Logger,
) (*reconcile.Reconciler, error) {
	return reconcile.New(
		client,
		config.GetResyncPeriod().AsDuration(),
		configDelivery,
		logger.With("component", "reconcile"),
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
