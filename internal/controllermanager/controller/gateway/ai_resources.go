package gateway

import (
	"fmt"

	"google.golang.org/protobuf/types/known/timestamppb"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	aidataplanev1 "github.com/lgc202/ingate/api/aidataplane/v1"
	"github.com/lgc202/ingate/internal/pkg/aidataplane"
)

// aiResourceCompilation 将同一份已收敛的入口结果转换为独立 AI Proxy 资源。
// 目标名称复用 Envoy 出口的计算规则，不从路由名称反解析来源或规则位置。
type aiResourceCompilation struct {
	sources      map[string]aiResourceSource
	modelTargets map[string]aiModelTargetTranslation
	mcpTargets   map[string]aiMCPTargetTranslation
}

func newAIResourceCompilation(
	snapshot publication,
	config aidataplane.Config,
	previous *aiResourceTranslation,
) (*aiResourceCompilation, error) {
	c := &aiResourceCompilation{
		sources:      make(map[string]aiResourceSource),
		modelTargets: make(map[string]aiModelTargetTranslation, len(config.Backends)),
		mcpTargets:   make(map[string]aiMCPTargetTranslation, len(config.NetworkBackends)),
	}
	networks := make(map[string]aidataplane.NetworkBackend, len(config.NetworkBackends))
	for _, network := range config.NetworkBackends {
		networks[network.Name] = network
		current := snapshot.networks[network.Name]
		if target := previous.mcpTargets[network.Name]; target.name != "" && target.network == current {
			c.mcpTargets[network.Name] = target
			continue
		}

		name, err := aidataplane.MCPTargetName(network)
		if err != nil {
			return nil, err
		}

		c.mcpTargets[network.Name] = aiMCPTargetTranslation{network: current, name: name}
	}

	for _, backend := range config.Backends {
		current := snapshot.networks[backend.NetworkBackendName]
		target := previous.modelTargets[backend.Name]
		if target.resource != nil && target.network == current && target.schema == backend.Schema &&
			target.modelNameOverride == *backend.ModelNameOverride {
			c.modelTargets[backend.Name] = target
			continue
		}

		name, err := aidataplane.ModelTargetName(backend, networks[backend.NetworkBackendName])
		if err != nil {
			return nil, err
		}

		resource := &aidataplanev1.ModelTarget{
			BindingName: backend.Name,
			Schema: &aidataplanev1.APISchema{
				Name:   aidataplanev1.APISchemaName_API_SCHEMA_NAME_OPENAI,
				Prefix: backend.Schema.Prefix,
			},
			ModelNameOverride:  *backend.ModelNameOverride,
			NetworkBackendName: backend.NetworkBackendName,
			TargetName:         name,
		}
		c.modelTargets[backend.Name] = aiModelTargetTranslation{
			schema:            backend.Schema,
			modelNameOverride: *backend.ModelNameOverride,
			network:           current,
			resource:          resource,
		}
	}

	return c, nil
}

func (c *aiResourceCompilation) addEntry(table *entryTable) error {
	port, hostName := table.entry.listener.Port, table.ai.Name
	name := fmt.Sprintf("entry/%d/%s", port, hostName)
	c.sources[name] = aiResourceSource{entry: table.entry}

	models := make(map[string]aidataplane.ModelRoute, len(table.ai.ModelRoutes))
	for _, match := range table.ai.ModelRoutes {
		models[match.Name] = match
	}

	for _, version := range table.versions {
		if version.model == nil {
			continue
		}

		for i, rule := range version.model.rules {
			match, ok := models[rule.matches[0].Name]
			if !ok {
				return fmt.Errorf("ModelRoute %q rule %d is missing from entry %q", version.model.ResourceName(), i, hostName)
			}

			c.addModelRule(version.model, i, *match.BackendName, table.entry)
		}
	}

	for _, match := range table.ai.MCPRoutes {
		version := table.versions["mcp/"+match.Name]
		c.addMCPRoute(version.mcp, match, table.entry)
	}

	return nil
}

func (c *aiResourceCompilation) addModelRule(
	route *modelRoute,
	index int,
	backendName string,
	entry *listenerEntry,
) {
	name := fmt.Sprintf("model/%s/rule/%d", route.ResourceName(), index)
	source := c.sources[name]
	source.model = route
	source.ruleIndex = index
	source.modelTarget = c.modelTargets[backendName].resource
	source.refs = append(source.refs, aiEntryReference{port: entry.listener.Port, name: entry.ResourceName()})
	c.sources[name] = source
}

func (c *aiResourceCompilation) addMCPRoute(route *mcpRoute, match aidataplane.MCPRoute, entry *listenerEntry) {
	name := "mcp/" + route.ResourceName()
	source := c.sources[name]
	source.mcp = route
	source.mcpBackend = *match.BackendName
	source.mcpTarget = c.mcpTargets[source.mcpBackend].name
	source.refs = append(source.refs, aiEntryReference{port: entry.listener.Port, name: entry.ResourceName()})
	c.sources[name] = source
}

func aiResourceOrigin(meta metav1.ObjectMeta) *aidataplanev1.Origin {
	return &aidataplanev1.Origin{
		Namespace:    meta.Namespace,
		Name:         meta.Name,
		CreationTime: timestamppb.New(meta.CreationTimestamp.Time),
	}
}
