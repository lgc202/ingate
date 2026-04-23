package watch

import (
	"context"
	"fmt"

	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	toolscache "k8s.io/client-go/tools/cache"

	"github.com/lgc202/ingate/internal/controlplane/controller/shared"
	"github.com/lgc202/ingate/internal/controlplane/xds/translate"
	gatewayv1alpha1 "github.com/lgc202/ingate/pkg/apis/gateway/v1alpha1"
	gatewayinformers "github.com/lgc202/ingate/pkg/generated/informers/externalversions/gateway/v1alpha1"
)

type Publisher interface {
	Publish(ctx context.Context, rg *gatewayv1alpha1.ResolvedGateway, runtimeConfig *translate.RuntimeConfig) error
	Delete(ctx context.Context, key shared.ObjectKey) error
	PublishFailure(ctx context.Context, rg *gatewayv1alpha1.ResolvedGateway, publishErr error) error
}

type ResolvedGatewayWatcher struct {
	ctx       context.Context
	informer  gatewayinformers.ResolvedGatewayInformer
	publisher Publisher
}

func NewResolvedGatewayWatcher(ctx context.Context, informer gatewayinformers.ResolvedGatewayInformer, publisher Publisher) *ResolvedGatewayWatcher {
	if ctx == nil {
		ctx = context.Background()
	}
	return &ResolvedGatewayWatcher{
		ctx:       ctx,
		informer:  informer,
		publisher: publisher,
	}
}

func (w *ResolvedGatewayWatcher) Register() error {
	if w == nil || w.informer == nil {
		return fmt.Errorf("resolvedgateway watcher informer is not initialized")
	}
	if w.publisher == nil {
		return fmt.Errorf("resolvedgateway watcher publisher is not initialized")
	}

	_, err := w.informer.Informer().AddEventHandler(toolscache.ResourceEventHandlerFuncs{
		AddFunc:    w.onUpsert,
		UpdateFunc: func(_, newObj interface{}) { w.onUpsert(newObj) },
		DeleteFunc: w.onDelete,
	})
	return err
}

func (w *ResolvedGatewayWatcher) HasSynced() bool {
	if w == nil || w.informer == nil {
		return false
	}
	return w.informer.Informer().HasSynced()
}

func (w *ResolvedGatewayWatcher) onUpsert(obj interface{}) {
	rg, ok := obj.(*gatewayv1alpha1.ResolvedGateway)
	if !ok || rg == nil {
		return
	}

	runtimeConfig, err := translate.FromResolvedGateway(rg)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("translate resolvedgateway %q: %w", rg.Name, err))
		if publishErr := w.publisher.PublishFailure(w.ctx, rg, err); publishErr != nil {
			utilruntime.HandleError(fmt.Errorf("mark publish failure for resolvedgateway %q: %w", rg.Name, publishErr))
		}
		return
	}
	if err := w.publisher.Publish(w.ctx, rg, runtimeConfig); err != nil {
		utilruntime.HandleError(fmt.Errorf("publish resolvedgateway %q: %w", rg.Name, err))
	}
}

func (w *ResolvedGatewayWatcher) onDelete(obj interface{}) {
	rg, ok := asResolvedGateway(obj)
	if !ok || rg == nil {
		return
	}
	if err := w.publisher.Delete(w.ctx, shared.NewObjectKey(rg.Namespace, rg.Name)); err != nil {
		utilruntime.HandleError(fmt.Errorf("delete resolvedgateway %q from publish cache: %w", rg.Name, err))
	}
}

func asResolvedGateway(obj interface{}) (*gatewayv1alpha1.ResolvedGateway, bool) {
	switch typed := obj.(type) {
	case *gatewayv1alpha1.ResolvedGateway:
		return typed, typed != nil
	case toolscache.DeletedFinalStateUnknown:
		rg, ok := typed.Obj.(*gatewayv1alpha1.ResolvedGateway)
		return rg, ok
	default:
		return nil, false
	}
}
