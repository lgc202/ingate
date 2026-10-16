package gateway

import (
	"fmt"
	"path"
	"slices"

	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/lgc202/ingate/internal/pkg/aidataplane"
	gatewayv1alpha1 "github.com/lgc202/ingate/pkg/apis/gateway/v1alpha1"
)

func (c *aiCompilation) compileMCPRoutes(routes []gatewayv1alpha1.MCPRoute) error {
	ordered := slices.Clone(routes)
	slices.SortFunc(ordered, func(a, b gatewayv1alpha1.MCPRoute) int {
		return compareMeta(a.ObjectMeta, b.ObjectMeta)
	})

	for _, route := range ordered {
		if !route.DeletionTimestamp.IsZero() {
			continue
		}

		listeners := c.matchingListeners(route.Namespace, route.Spec.ParentRefs)
		if len(listeners) == 0 {
			continue
		}

		if len(route.Spec.BackendRefs) != 1 {
			return fmt.Errorf("MCPRoute %q requires exactly one Backend reference", client.ObjectKeyFromObject(&route).String())
		}

		ref := route.Spec.BackendRefs[0]
		backendName := c.bindNetworkBackend(client.ObjectKey{Namespace: route.Namespace, Name: ref.Name})
		compiled := aidataplane.MCPRoute{
			Name:        path.Join(route.Namespace, route.Name),
			Path:        ptr.Deref(route.Spec.Path, "/mcp"),
			BackendName: new(backendName),
			BackendPath: ptr.Deref(ref.Path, "/mcp"),
		}

		for _, listener := range listeners {
			c.mcpRoutes[listener] = append(c.mcpRoutes[listener], compiled)
		}
	}

	return nil
}
