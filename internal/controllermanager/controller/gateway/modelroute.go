package gateway

import (
	"cmp"
	"fmt"
	"path"
	"slices"
	"strings"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/lgc202/ingate/internal/pkg/aidataplane"
	gatewayv1alpha1 "github.com/lgc202/ingate/pkg/apis/gateway/v1alpha1"
)

func (c *aiCompilation) compileModelRoutes(routes []gatewayv1alpha1.ModelRoute) error {
	ordered := slices.Clone(routes)
	slices.SortFunc(ordered, func(a, b gatewayv1alpha1.ModelRoute) int {
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

		for i, rule := range route.Spec.Rules {
			if len(rule.BackendRefs) != 1 {
				return fmt.Errorf(
					"ModelRoute %q rule %d requires exactly one AIBackend reference",
					client.ObjectKeyFromObject(&route).String(),
					i,
				)
			}

			backendName := c.bindModelBackend(&route, i, rule.BackendRefs[0])
			matches := rule.Matches
			if len(matches) == 0 {
				matches = []gatewayv1alpha1.ModelRouteMatch{{}}
			}

			for j, match := range matches {
				headers := make([]aidataplane.HeaderMatch, len(match.Headers))
				for k, header := range match.Headers {
					headers[k] = aidataplane.HeaderMatch{
						Name:  strings.ToLower(header.Name),
						Type:  aidataplane.HeaderMatchType(cmp.Or(header.Type, gatewayv1alpha1.HeaderMatchExact)),
						Value: header.Value,
					}
				}

				compiled := aidataplane.ModelRoute{
					Name:        fmt.Sprintf("%s/rule/%d/match/%d", path.Join(route.Namespace, route.Name), i, j),
					Headers:     headers,
					BackendName: new(backendName),
				}

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
	route *gatewayv1alpha1.ModelRoute,
	ruleIndex int,
	ref gatewayv1alpha1.AIBackendReference,
) string {
	key := client.ObjectKey{Namespace: route.Namespace, Name: ref.Name}
	backend := c.aiBackends[key]
	if backend == nil {
		return ""
	}

	networkKey := client.ObjectKey{Namespace: backend.Namespace, Name: backend.Spec.BackendRef.Name}
	networkName := c.bindNetworkBackend(networkKey)
	if networkName == "" {
		return ""
	}

	name := fmt.Sprintf("%s/route/%s/rule/%d/ref/0", path.Join(key.Namespace, key.Name), route.Name, ruleIndex)
	prefix := backend.Spec.Schema.Prefix
	if backend.Spec.Schema.Name == gatewayv1alpha1.APISchemaOpenAI && prefix == "" {
		prefix = "/v1"
	}

	c.protocolBackends[name] = aidataplane.Backend{
		Name:               name,
		Schema:             aidataplane.APISchema{Name: aidataplane.APISchemaName(backend.Spec.Schema.Name), Prefix: prefix},
		ModelNameOverride:  new(ref.ModelNameOverride),
		NetworkBackendName: networkName,
	}

	return name
}
