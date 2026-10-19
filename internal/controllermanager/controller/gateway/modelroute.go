package gateway

import (
	"fmt"
	"path"
	"slices"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/lgc202/ingate/internal/pkg/aidataplane"
	gatewayv1alpha1 "github.com/lgc202/ingate/pkg/apis/gateway/v1alpha1"
)

func (c *aiCompilation) compileModelRoutes(routes []*modelRoute) error {
	ordered := slices.Clone(routes)
	slices.SortFunc(ordered, func(a, b *modelRoute) int {
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

		for i, rule := range route.rules {
			if len(rule.refs) != 1 {
				return fmt.Errorf(
					"ModelRoute %q rule %d requires exactly one AIBackend reference",
					path.Join(route.Namespace, route.Name),
					i,
				)
			}

			backendName := c.bindModelBackend(route, i, rule.refs[0])
			for _, match := range rule.matches {
				compiled := match
				compiled.Headers = slices.Clone(match.Headers)
				compiled.BackendName = new(backendName)

				for _, listener := range listeners {
					c.modelRoutes[listener] = append(c.modelRoutes[listener], compiled)
				}
			}
		}
	}

	return nil
}

// bindModelBackend 按路由规则建立协议绑定；同一 AIBackend 的不同 override 不合并。
func (c *aiCompilation) bindModelBackend(
	route *modelRoute,
	ruleIndex int,
	ref gatewayv1alpha1.AIBackendReference,
) string {
	key := client.ObjectKey{Namespace: route.Namespace, Name: ref.Name}
	binding := c.aiBackends[key]
	if binding == nil || binding.network == nil {
		return ""
	}

	backend := binding.backend
	networkName := c.bindNetworkBackend(backend.networkKey)
	if networkName == "" {
		return ""
	}

	name := fmt.Sprintf("%s/route/%s/rule/%d/ref/0", path.Join(key.Namespace, key.Name), route.Name, ruleIndex)
	c.protocolBackends[name] = aidataplane.Backend{
		Name:               name,
		Schema:             backend.schema,
		ModelNameOverride:  new(ref.ModelNameOverride),
		NetworkBackendName: networkName,
	}

	return name
}
