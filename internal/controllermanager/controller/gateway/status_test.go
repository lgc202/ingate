package gateway

import (
	"context"
	"testing"

	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	gatewayv1alpha1 "github.com/lgc202/ingate/pkg/apis/gateway/v1alpha1"
)

func TestStatusUpdatesConfirmOnlyCompiledGeneration(t *testing.T) {
	tests := []struct {
		name       string
		change     func(context.Context, client.Client, client.Object) error
		wantWrites int
	}{
		{name: "same instance and generation", wantWrites: 2},
		{
			name: "newer generation",
			change: func(ctx context.Context, apiClient client.Client, object client.Object) error {
				object.SetGeneration(object.GetGeneration() + 1)

				return apiClient.Update(ctx, object)
			},
		},
		{
			name: "replacement instance",
			change: func(ctx context.Context, apiClient client.Client, object client.Object) error {
				if err := apiClient.Delete(ctx, object); err != nil {
					return err
				}

				object.SetUID(types.UID("replacement-" + object.GetName()))
				object.SetResourceVersion("")

				return apiClient.Create(ctx, object)
			},
		},
		{
			name: "already deleted",
			change: func(ctx context.Context, apiClient client.Client, object client.Object) error {
				return apiClient.Delete(ctx, object)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gateway := gatewayResource("edge", 1, listener("http", 8080, ""))
			route := routeResource("orders", 1, "edge", "http", "orders", gatewayv1alpha1.PathMatchPrefix)
			gateway.UID, route.UID = "old-gateway", "old-route"
			gateway.Generation, route.Generation = 1, 1
			compiledGateway := *projectGateway(&gateway)
			compiledRoute := compileHTTPRoute(&route)
			apiClient := newFakeClient(t, &gateway, &route)
			for _, object := range []client.Object{&gateway, &route} {
				if err := apiClient.Get(t.Context(), client.ObjectKeyFromObject(object), object); err != nil {
					t.Fatalf("Get(%T) error = %v, want nil", object, err)
				}

				if tt.change != nil {
					if err := tt.change(t.Context(), apiClient, object); err != nil {
						t.Fatalf("prepare %s(%T) error = %v, want nil", tt.name, object, err)
					}
				}
			}

			var writes int
			statusClient := interceptor.NewClient(apiClient, interceptor.Funcs{
				SubResourceUpdate: func(
					ctx context.Context,
					underlying client.Client,
					name string,
					object client.Object,
					options ...client.SubResourceUpdateOption,
				) error {
					writes++

					return underlying.SubResource(name).Update(ctx, object, options...)
				},
			})
			r := New(statusClient, nil)
			parents := []httpRouteParentResolution{{
				routeKey:  client.ObjectKeyFromObject(&route),
				parentRef: route.Spec.ParentRefs[0],
				accepted:  true,
				reason:    "Accepted",
				message:   "attached",
			}}

			if err := r.updateGatewayStatuses(t.Context(), []gatewayv1alpha1.Gateway{compiledGateway}, nil); err != nil {
				t.Fatalf("updateGatewayStatuses(%s) error = %v, want nil", tt.name, err)
			}

			if err := r.updateHTTPRouteStatuses(t.Context(), []*httpRoute{compiledRoute}, parents); err != nil {
				t.Fatalf("updateHTTPRouteStatuses(%s) error = %v, want nil", tt.name, err)
			}

			if writes != tt.wantWrites {
				t.Errorf("status writes(%s) = %d, want %d", tt.name, writes, tt.wantWrites)
			}
		})
	}
}
