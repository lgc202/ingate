package gateway

import (
	"fmt"
	"path"
	"slices"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/lgc202/ingate/internal/pkg/aidataplane"
)

func (c *aiCompilation) compileMCPRoutes(routes []*mcpRoute) error {
	ordered := slices.Clone(routes)
	slices.SortFunc(ordered, func(a, b *mcpRoute) int {
		return compareMeta(a.ObjectMeta, b.ObjectMeta)
	})

	for _, route := range ordered {
		if !route.DeletionTimestamp.IsZero() {
			continue
		}

		listeners := c.matchingListeners(route.Namespace, route.parents)
		if len(listeners) == 0 {
			continue
		}

		if len(route.refs) != 1 {
			return fmt.Errorf("MCPRoute %q requires exactly one Backend reference", path.Join(route.Namespace, route.Name))
		}

		entryPath := route.match.Path
		for _, listener := range listeners {
			if len(c.modelRoutes[listener]) != 0 && (entryPath == aidataplane.ModelChatCompletionsPath || entryPath == aidataplane.ModelResponsesPath) {
				return fmt.Errorf("MCPRoute %q path %q conflicts with model operations on listener %q", path.Join(route.Namespace, route.Name), entryPath, listener.listenerName)
			}
		}

		ref := route.refs[0]
		backendName := c.bindNetworkBackend(client.ObjectKey{Namespace: route.Namespace, Name: ref.Name})
		compiled := route.match
		compiled.BackendName = new(backendName)

		for _, listener := range listeners {
			c.mcpRoutes[listener] = append(c.mcpRoutes[listener], compiled)
		}
	}

	return nil
}
