package gateway

import (
	"cmp"
	"fmt"
	"maps"
	"slices"

	"buf.build/go/protovalidate"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	aidataplanev1 "github.com/lgc202/ingate/api/aidataplane/v1"
	"github.com/lgc202/ingate/internal/pkg/aidataplane"
)

// aiResourceCompilation 将同一份已收敛的入口结果转换为独立 Go 资源。
// 目标名称复用 Envoy 出口的计算规则，不从路由名称反解析来源或规则位置。
type aiResourceCompilation struct {
	resources    map[string]*aidataplanev1.Resource
	modelTargets map[string]*aidataplanev1.ModelTarget
	mcpTargets   map[string]string
}

// aiResources 只构造候选，不更新分发缓存；失败时不返回可部分发布的结果。
// 当前仍扫描完整发布结果，后续缓存按资源内容识别更新和删除。
func (p publication) aiResources() ([]*aidataplanev1.Resource, error) {
	config, err := p.aiConfig()
	if err != nil {
		return nil, err
	}

	c, err := newAIResourceCompilation(config)
	if err != nil {
		return nil, err
	}

	for _, table := range p.tables {
		if table.ai.Name == "" {
			continue
		}

		if err := c.addEntry(table); err != nil {
			return nil, err
		}
	}

	resources := make([]*aidataplanev1.Resource, 0, len(c.resources))
	for _, name := range slices.Sorted(maps.Keys(c.resources)) {
		resource := c.resources[name]
		sortAIEntryReferences(resource)
		if err := protovalidate.Validate(resource); err != nil {
			return nil, fmt.Errorf("validate Go data-plane resource %q: %w", name, err)
		}

		resources = append(resources, resource)
	}

	return resources, nil
}

func newAIResourceCompilation(config aidataplane.Config) (*aiResourceCompilation, error) {
	c := &aiResourceCompilation{
		resources:    make(map[string]*aidataplanev1.Resource),
		modelTargets: make(map[string]*aidataplanev1.ModelTarget),
		mcpTargets:   make(map[string]string),
	}
	networks := make(map[string]aidataplane.NetworkBackend, len(config.NetworkBackends))
	for _, network := range config.NetworkBackends {
		networks[network.Name] = network
		name, err := aidataplane.MCPTargetName(network)
		if err != nil {
			return nil, err
		}

		c.mcpTargets[network.Name] = name
	}

	for _, backend := range config.Backends {
		name, err := aidataplane.ModelTargetName(backend, networks[backend.NetworkBackendName])
		if err != nil {
			return nil, err
		}

		// aiConfig 已校验当前唯一支持的 OpenAI 协议及默认路径。
		c.modelTargets[backend.Name] = &aidataplanev1.ModelTarget{
			BindingName: backend.Name,
			Schema: &aidataplanev1.APISchema{
				Name:   aidataplanev1.APISchemaName_API_SCHEMA_NAME_OPENAI,
				Prefix: backend.Schema.Prefix,
			},
			ModelNameOverride:  *backend.ModelNameOverride,
			NetworkBackendName: backend.NetworkBackendName,
			TargetName:         name,
		}
	}

	return c, nil
}

func (c *aiResourceCompilation) addEntry(table *entryTable) error {
	port, hostName := table.entry.listener.Port, table.ai.Name
	name := fmt.Sprintf("entry/%d/%s", port, hostName)
	c.resources[name] = &aidataplanev1.Resource{
		Name: name,
		Payload: &aidataplanev1.Resource_Entry{Entry: &aidataplanev1.Entry{
			Port: port, HostName: hostName, Hostname: *table.ai.Hostname,
		}},
	}

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
	resource := c.resources[name]
	if resource == nil {
		compiled := &aidataplanev1.ModelRule{Origin: aiResourceOrigin(route.ObjectMeta), RuleIndex: uint32(index)}
		for _, match := range route.rules[index].matches {
			condition := &aidataplanev1.ModelMatch{Condition: &aidataplanev1.ModelMatch_All{All: &emptypb.Empty{}}}
			if len(match.Headers) != 0 {
				condition.Condition = &aidataplanev1.ModelMatch_Exact{Exact: match.Headers[0].Value}
			}

			compiled.Matches = append(compiled.Matches, condition)
		}

		compiled.Outcome = &aidataplanev1.ModelRule_Unavailable{Unavailable: &emptypb.Empty{}}
		if backendName != "" {
			compiled.Outcome = &aidataplanev1.ModelRule_Target{Target: c.modelTargets[backendName]}
		}

		resource = &aidataplanev1.Resource{Name: name, Payload: &aidataplanev1.Resource_ModelRule{ModelRule: compiled}}
		c.resources[name] = resource
	}

	compiled := resource.GetModelRule()
	compiled.EntryRefs = append(compiled.EntryRefs, &aidataplanev1.EntryReference{
		Port: entry.listener.Port, HostName: entry.ResourceName(),
	})
}

func (c *aiResourceCompilation) addMCPRoute(route *mcpRoute, match aidataplane.MCPRoute, entry *listenerEntry) {
	name := "mcp/" + route.ResourceName()
	resource := c.resources[name]
	if resource == nil {
		compiled := &aidataplanev1.MCPRoute{Origin: aiResourceOrigin(route.ObjectMeta), Path: match.Path}
		compiled.Outcome = &aidataplanev1.MCPRoute_Unavailable{Unavailable: &emptypb.Empty{}}
		if backendName := *match.BackendName; backendName != "" {
			compiled.Outcome = &aidataplanev1.MCPRoute_Target{Target: &aidataplanev1.MCPTarget{
				NetworkBackendName: backendName,
				BackendPath:        match.BackendPath,
				TargetName:         c.mcpTargets[backendName],
			}}
		}

		resource = &aidataplanev1.Resource{Name: name, Payload: &aidataplanev1.Resource_McpRoute{McpRoute: compiled}}
		c.resources[name] = resource
	}

	compiled := resource.GetMcpRoute()
	compiled.EntryRefs = append(compiled.EntryRefs, &aidataplanev1.EntryReference{
		Port: entry.listener.Port, HostName: entry.ResourceName(),
	})
}

func aiResourceOrigin(meta metav1.ObjectMeta) *aidataplanev1.Origin {
	return &aidataplanev1.Origin{
		Namespace:    meta.Namespace,
		Name:         meta.Name,
		CreationTime: timestamppb.New(meta.CreationTimestamp.Time),
	}
}

func sortAIEntryReferences(resource *aidataplanev1.Resource) {
	var refs []*aidataplanev1.EntryReference
	switch payload := resource.Payload.(type) {
	case *aidataplanev1.Resource_ModelRule:
		refs = payload.ModelRule.EntryRefs
	case *aidataplanev1.Resource_McpRoute:
		refs = payload.McpRoute.EntryRefs
	}

	slices.SortFunc(refs, func(a, b *aidataplanev1.EntryReference) int {
		if order := cmp.Compare(a.Port, b.Port); order != 0 {
			return order
		}

		return cmp.Compare(a.HostName, b.HostName)
	})
}
