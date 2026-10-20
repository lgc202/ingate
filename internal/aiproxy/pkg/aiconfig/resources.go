package aiconfig

import (
	"cmp"
	"errors"
	"fmt"
	"maps"
	"slices"

	discoveryv3 "github.com/envoyproxy/go-control-plane/envoy/service/discovery/v3"
	"google.golang.org/protobuf/proto"

	aidataplanev1 "github.com/lgc202/ingate/api/aidataplane/v1"
	"github.com/lgc202/ingate/internal/pkg/aidataplane"
)

// resourceState 只保存已接受的资源与入口索引；更新先构建候选，不修改旧状态。
// 单个订阅循环拥有写入权，查询只持有独立的不可变 entryConfig。
type resourceState struct {
	resources map[string]versionedResource
	byEntry   map[hostKey]map[string]bool
	views     map[hostKey]*entryConfig
}

type versionedResource struct {
	version string
	message *aidataplanev1.Resource
}

type entryConfig struct {
	models []modelCandidate
	mcp    map[string]mcpCandidate
}

type modelCandidate struct {
	model       string
	all         bool
	unavailable bool
	backend     ModelBackend
	origin      *aidataplanev1.Origin
	ruleIndex   uint32
	matchIndex  int
}

type mcpCandidate struct {
	unavailable bool
	backend     MCPBackend
	origin      *aidataplanev1.Origin
}

type ruleKey struct {
	namespace string
	name      string
	index     uint32
}

// apply 把一批更新与删除作为一个候选校验；失败时资源、版本及运行索引全部保旧。
// 顶层表浅复制，未变的 protobuf 和入口视图复用；只重建旧、新引用涉及的入口。
func (s *resourceState) apply(response *discoveryv3.DeltaDiscoveryResponse) (*resourceState, error) {
	if response.TypeUrl != aidataplanev1.ResourceType || response.Nonce == "" {
		return nil, errors.New("AI configuration response requires the supported type and a nonce")
	}

	changes, err := s.decodeChanges(response)
	if err != nil {
		return nil, err
	}

	next := &resourceState{
		resources: make(map[string]versionedResource, len(s.resources)+len(changes)),
		byEntry:   make(map[hostKey]map[string]bool, len(s.byEntry)),
		views:     make(map[hostKey]*entryConfig, len(s.views)),
	}
	maps.Copy(next.resources, s.resources)
	maps.Copy(next.byEntry, s.byEntry)
	maps.Copy(next.views, s.views)

	affected := make(map[hostKey]bool)
	for name, resource := range changes {
		previous := s.resources[name]
		if resource.message != nil {
			next.resources[name] = resource
		} else {
			delete(next.resources, name)
		}

		if proto.Equal(previous.message, resource.message) {
			continue
		}

		for _, key := range resourceEntries(previous.message) {
			next.changeEntry(key, name, false, affected)
		}

		for _, key := range resourceEntries(resource.message) {
			next.changeEntry(key, name, true, affected)
		}
	}

	for key := range affected {
		view, err := next.buildEntry(key)
		if err != nil {
			return nil, err
		}

		if view == nil {
			delete(next.views, key)
			delete(next.byEntry, key)
		} else {
			next.views[key] = view
		}
	}

	return next, nil
}

func (s *resourceState) decodeChanges(response *discoveryv3.DeltaDiscoveryResponse) (map[string]versionedResource, error) {
	changes := make(map[string]versionedResource, len(response.Resources)+len(response.RemovedResources))
	for _, resource := range response.Resources {
		if resource.GetName() == "" || resource.GetVersion() == "" || resource.GetResource() == nil {
			return nil, errors.New("AI configuration update requires a name, version and payload")
		}

		if _, exists := changes[resource.Name]; exists {
			return nil, fmt.Errorf("duplicate AI configuration update %q", resource.Name)
		}

		if resource.Ttl != nil || len(resource.Aliases) != 0 {
			return nil, fmt.Errorf("AI configuration resource %q uses unsupported TTL or aliases", resource.Name)
		}

		message := new(aidataplanev1.Resource)
		if err := resource.Resource.UnmarshalTo(message); err != nil {
			return nil, fmt.Errorf("decode AI configuration resource %q: %w", resource.Name, err)
		}

		if message.Name != resource.Name {
			return nil, fmt.Errorf("AI configuration resource %q has a different payload name", resource.Name)
		}

		if err := validateResource(message); err != nil {
			return nil, fmt.Errorf("validate AI configuration resource %q: %w", resource.Name, err)
		}

		previous := s.resources[resource.Name]
		if proto.Equal(previous.message, message) {
			message = previous.message
		} else if previous.version == resource.Version {
			return nil, fmt.Errorf("AI configuration resource %q changed without a new version", resource.Name)
		}

		changes[resource.Name] = versionedResource{version: resource.Version, message: message}
	}

	for _, name := range response.RemovedResources {
		if _, exists := changes[name]; name == "" || exists {
			return nil, fmt.Errorf("empty or duplicate AI configuration removal %q", name)
		}

		changes[name] = versionedResource{}
	}

	return changes, nil
}

func (s *resourceState) changeEntry(key hostKey, name string, add bool, affected map[hostKey]bool) {
	if !affected[key] {
		s.byEntry[key] = maps.Clone(s.byEntry[key])
		if s.byEntry[key] == nil {
			s.byEntry[key] = make(map[string]bool)
		}

		affected[key] = true
	}

	if add {
		s.byEntry[key][name] = true
	} else {
		delete(s.byEntry[key], name)
	}
}

func (s *resourceState) buildEntry(key hostKey) (*entryConfig, error) {
	names := s.byEntry[key]
	if len(names) == 0 {
		return nil, nil
	}

	view := &entryConfig{mcp: make(map[string]mcpCandidate)}
	entries := 0
	models := make(map[ruleKey]bool)
	mcp := make(map[ruleKey]bool)

	for name := range names {
		resource := s.resources[name].message
		switch payload := resource.Payload.(type) {
		case *aidataplanev1.Resource_Entry:
			entries++
		case *aidataplanev1.Resource_ModelRule:
			rule := payload.ModelRule
			identity := ruleKey{namespace: rule.Origin.Namespace, name: rule.Origin.Name, index: rule.RuleIndex}
			if models[identity] {
				return nil, fmt.Errorf("duplicate model rule identity in entry %d/%q", key.port, key.name)
			}

			models[identity] = true
			view.addModelRule(payload.ModelRule)
		case *aidataplanev1.Resource_McpRoute:
			route := payload.McpRoute
			identity := ruleKey{namespace: route.Origin.Namespace, name: route.Origin.Name}
			if mcp[identity] {
				return nil, fmt.Errorf("duplicate MCP route identity in entry %d/%q", key.port, key.name)
			}

			mcp[identity] = true
			view.addMCPRoute(payload.McpRoute)
		}
	}

	if entries != 1 {
		return nil, fmt.Errorf("AI configuration entry %d/%q requires exactly one Entry, got %d", key.port, key.name, entries)
	}

	slices.SortFunc(view.models, compareModelCandidates)

	return view, nil
}

func (v *entryConfig) addModelRule(rule *aidataplanev1.ModelRule) {
	for index, match := range rule.Matches {
		candidate := modelCandidate{
			model: match.GetExact(), all: match.GetAll() != nil, unavailable: rule.GetUnavailable() != nil,
			origin: rule.Origin, ruleIndex: rule.RuleIndex, matchIndex: index,
			backend: ModelBackend{RouteName: fmt.Sprintf("%s/%s/rule/%d/match/%d", rule.Origin.Namespace, rule.Origin.Name, rule.RuleIndex, index)},
		}
		if target := rule.GetTarget(); target != nil {
			candidate.backend.Name = target.BindingName
			candidate.backend.Schema = aidataplane.APISchema{Name: aidataplane.APISchemaOpenAI, Prefix: target.Schema.Prefix}
			candidate.backend.ModelNameOverride = target.ModelNameOverride
			candidate.backend.NetworkBackendName = target.NetworkBackendName
			candidate.backend.TargetName = target.TargetName
		}

		v.models = append(v.models, candidate)
	}
}

func (v *entryConfig) addMCPRoute(route *aidataplanev1.MCPRoute) {
	candidate := mcpCandidate{
		unavailable: route.GetUnavailable() != nil,
		origin:      route.Origin,
		backend:     MCPBackend{RouteName: route.Origin.Namespace + "/" + route.Origin.Name},
	}
	if target := route.GetTarget(); target != nil {
		candidate.backend.NetworkBackendName = target.NetworkBackendName
		candidate.backend.BackendPath = target.BackendPath
		candidate.backend.TargetName = target.TargetName
	}

	previous, exists := v.mcp[route.Path]
	if !exists || compareOrigins(candidate.origin, previous.origin) < 0 {
		v.mcp[route.Path] = candidate
	}
}

func resourceEntries(resource *aidataplanev1.Resource) []hostKey {
	var refs []*aidataplanev1.EntryReference
	switch payload := resource.GetPayload().(type) {
	case *aidataplanev1.Resource_Entry:
		return []hostKey{{port: payload.Entry.Port, name: payload.Entry.HostName}}
	case *aidataplanev1.Resource_ModelRule:
		refs = payload.ModelRule.EntryRefs
	case *aidataplanev1.Resource_McpRoute:
		refs = payload.McpRoute.EntryRefs
	}

	keys := make([]hostKey, 0, len(refs))
	for _, ref := range refs {
		keys = append(keys, hostKey{port: ref.Port, name: ref.HostName})
	}

	return keys
}

func compareModelCandidates(a, b modelCandidate) int {
	if a.all != b.all {
		if a.all {
			return 1
		}

		return -1
	}

	if order := compareOrigins(a.origin, b.origin); order != 0 {
		return order
	}

	if order := cmp.Compare(a.ruleIndex, b.ruleIndex); order != 0 {
		return order
	}

	return cmp.Compare(a.matchIndex, b.matchIndex)
}

func compareOrigins(a, b *aidataplanev1.Origin) int {
	if order := a.CreationTime.AsTime().Compare(b.CreationTime.AsTime()); order != 0 {
		return order
	}

	if order := cmp.Compare(a.Namespace, b.Namespace); order != 0 {
		return order
	}

	return cmp.Compare(a.Name, b.Name)
}
