// Package gateway 将 Gateway、Route 和 Upstream 注册到 Generic API Server 的 API Group。
package gateway

import (
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/registry/generic"
	"k8s.io/apiserver/pkg/registry/rest"

	gatewaystorage "github.com/lgc202/ingate/internal/apiserver/registry/gateway/gateway"
	routestorage "github.com/lgc202/ingate/internal/apiserver/registry/gateway/route"
	upstreamstorage "github.com/lgc202/ingate/internal/apiserver/registry/gateway/upstream"
)

// NewStorage 创建 gateway.ingate.io API Group 中已实现资源的存储映射。
func NewStorage(
	scheme *runtime.Scheme,
	optionsGetter generic.RESTOptionsGetter,
) (map[string]rest.Storage, error) {
	gatewayREST, statusREST, err := gatewaystorage.NewREST(scheme, optionsGetter)
	if err != nil {
		return nil, fmt.Errorf("create Gateway REST storage: %w", err)
	}

	routeREST, routeStatusREST, err := routestorage.NewREST(scheme, optionsGetter)
	if err != nil {
		gatewayREST.Destroy()
		return nil, fmt.Errorf("create Route REST storage: %w", err)
	}

	upstreamREST, upstreamStatusREST, err := upstreamstorage.NewREST(scheme, optionsGetter)
	if err != nil {
		routeREST.Destroy()
		gatewayREST.Destroy()
		return nil, fmt.Errorf("create Upstream REST storage: %w", err)
	}

	return map[string]rest.Storage{
		"gateways":         gatewayREST,
		"gateways/status":  statusREST,
		"routes":           routeREST,
		"routes/status":    routeStatusREST,
		"upstreams":        upstreamREST,
		"upstreams/status": upstreamStatusREST,
	}, nil
}
