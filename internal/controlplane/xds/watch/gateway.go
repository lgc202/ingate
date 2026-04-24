package watch

import (
	"context"
	"fmt"

	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	toolscache "k8s.io/client-go/tools/cache"

	controllerruntime "github.com/lgc202/ingate/internal/controlplane/controller/runtime"
	"github.com/lgc202/ingate/internal/controlplane/controller/shared"
	gatewaycompiler "github.com/lgc202/ingate/internal/controlplane/controller/gatewaycompiler"
	"github.com/lgc202/ingate/internal/controlplane/xds/translate"
	gatewayv1alpha1 "github.com/lgc202/ingate/pkg/apis/gateway/v1alpha1"
	gatewayinformers "github.com/lgc202/ingate/pkg/generated/informers/externalversions/gateway/v1alpha1"
)

type Publisher interface {
	Publish(ctx context.Context, gateway *gatewayv1alpha1.Gateway, runtimeConfig *translate.RuntimeConfig) error
	Delete(ctx context.Context, key shared.ObjectKey) error
	PublishFailure(ctx context.Context, gateway *gatewayv1alpha1.Gateway, publishErr error) error
}

type GatewayWatcher struct {
	ctx       context.Context
	informer  gatewayinformers.GatewayInformer
	loader    *gatewaycompiler.Loader
	publisher Publisher
}

func NewGatewayWatcher(ctx context.Context, runtimeContext *controllerruntime.Context, publisher Publisher) *GatewayWatcher {
	if ctx == nil {
		ctx = context.Background()
	}
	return &GatewayWatcher{
		ctx:       ctx,
		informer:  runtimeContext.InformerFactory.Gateway().V1alpha1().Gateways(),
		loader:    gatewaycompiler.NewLoader(runtimeContext),
		publisher: publisher,
	}
}

func (w *GatewayWatcher) Register() error {
	if w == nil || w.informer == nil {
		return fmt.Errorf("gateway watcher informer is not initialized")
	}
	if w.publisher == nil {
		return fmt.Errorf("gateway watcher publisher is not initialized")
	}

	_, err := w.informer.Informer().AddEventHandler(toolscache.ResourceEventHandlerFuncs{
		AddFunc:    w.onUpsert,
		UpdateFunc: func(_, newObj interface{}) { w.onUpsert(newObj) },
		DeleteFunc: w.onDelete,
	})
	return err
}

func (w *GatewayWatcher) HasSynced() bool {
	if w == nil || w.informer == nil {
		return false
	}
	return w.informer.Informer().HasSynced()
}

func (w *GatewayWatcher) onUpsert(obj interface{}) {
	gateway, ok := obj.(*gatewayv1alpha1.Gateway)
	if !ok || gateway == nil {
		return
	}

	bundle, err := w.loader.Load(shared.NewObjectKey(gateway.Namespace, gateway.Name))
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("load gateway %q: %w", gateway.Name, err))
		if publishErr := w.publisher.PublishFailure(w.ctx, gateway, err); publishErr != nil {
			utilruntime.HandleError(fmt.Errorf("mark publish failure for gateway %q: %w", gateway.Name, publishErr))
		}
		return
	}
	logical, err := gatewaycompiler.BuildLogicalGateway(bundle)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("build logical gateway %q: %w", gateway.Name, err))
		if publishErr := w.publisher.PublishFailure(w.ctx, gateway, err); publishErr != nil {
			utilruntime.HandleError(fmt.Errorf("mark publish failure for gateway %q: %w", gateway.Name, publishErr))
		}
		return
	}
	config, err := translate.FromLogicalGateway(logical)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("translate gateway %q: %w", gateway.Name, err))
		if publishErr := w.publisher.PublishFailure(w.ctx, gateway, err); publishErr != nil {
			utilruntime.HandleError(fmt.Errorf("mark publish failure for gateway %q: %w", gateway.Name, publishErr))
		}
		return
	}
	if err := w.publisher.Publish(w.ctx, gateway, config); err != nil {
		utilruntime.HandleError(fmt.Errorf("publish gateway %q: %w", gateway.Name, err))
	}
}

func (w *GatewayWatcher) onDelete(obj interface{}) {
	gateway, ok := asGateway(obj)
	if !ok || gateway == nil {
		return
	}
	if err := w.publisher.Delete(w.ctx, shared.NewObjectKey(gateway.Namespace, gateway.Name)); err != nil {
		utilruntime.HandleError(fmt.Errorf("delete gateway %q from publish cache: %w", gateway.Name, err))
	}
}

func asGateway(obj interface{}) (*gatewayv1alpha1.Gateway, bool) {
	switch typed := obj.(type) {
	case *gatewayv1alpha1.Gateway:
		return typed, typed != nil
	case toolscache.DeletedFinalStateUnknown:
		gateway, ok := typed.Obj.(*gatewayv1alpha1.Gateway)
		return gateway, ok
	default:
		return nil, false
	}
}
