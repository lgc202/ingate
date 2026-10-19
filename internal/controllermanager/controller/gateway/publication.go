package gateway

import (
	"cmp"
	"errors"
	"fmt"
	"maps"
	"path"
	"slices"

	"istio.io/istio/pkg/kube/krt"

	"github.com/lgc202/ingate/internal/controllermanager/dataplane"
	"github.com/lgc202/ingate/internal/pkg/aidataplane"
)

// publication 是一次发布捕获的只读结果；校验中间集合的收敛后才允许发布和确认状态。
// 读取不构成跨资源事务；校验排除已观察到的不匹配，后续变化仍由事件驱动协调。
type publication struct {
	selection *gatewaySelection
	tables    []*entryTable
	statuses  []*httpRouteStatus
	networks  map[string]*networkBackend
	err       error
}

var errCompilationPending = errors.New("entry compilation is still converging")

func (s *compilationSource) readPublication() publication {
	p := publication{
		selection: s.selection.Get(),
		tables:    s.tables.List(),
		statuses:  s.httpStatuses.List(),
		networks:  make(map[string]*networkBackend),
	}
	for _, backend := range s.backends.projected.List() {
		p.networks[backend.ResourceName()] = backend
	}

	slices.SortFunc(p.tables, func(a, b *entryTable) int {
		if order := compareMeta(a.entry.origin, b.entry.origin); order != 0 {
			return order
		}

		return cmp.Compare(a.entry.index, b.entry.index)
	})

	p.err = s.checkEntryVersions(p)

	return p
}

// checkEntryVersions 防止异步的裁决、附着和聚合混成半新半旧的候选。
// 不等待或锁住计算图；返回可重试错误，保留已发布配置，后续集合事件会再次入队。
func (s *compilationSource) checkEntryVersions(p publication) error {
	expectedEntries := make(map[string]*listenerEntry)
	byGateway := make(map[string][]*listenerEntry)
	for _, gateway := range p.selection.accepted {
		for i, listener := range gateway.Spec.Listeners {
			entry := &listenerEntry{origin: projectForwardingMeta(gateway), listener: listener, index: i}
			expectedEntries[entry.ResourceName()] = entry
			key := path.Join(entry.origin.Namespace, entry.origin.Name)
			byGateway[key] = append(byGateway[key], entry)
		}
	}

	if len(expectedEntries) != len(p.tables) {
		return errCompilationPending
	}

	versions := make(map[string]compiledVersion)
	httpRoutes := s.httpRoutes.projected.List()
	if len(httpRoutes) != len(p.statuses) {
		return errCompilationPending
	}

	for _, route := range httpRoutes {
		versions["http/"+route.ResourceName()] = compiledVersion{http: route}
	}

	for _, route := range s.modelRoutes.projected.List() {
		versions["model/"+route.ResourceName()] = compiledVersion{model: route}
	}

	for _, route := range s.mcpRoutes.projected.List() {
		versions["mcp/"+route.ResourceName()] = compiledVersion{mcp: route}
	}

	attached := make(map[string]map[string]compiledVersion)
	seen := make(map[string]bool)
	for prefix, collection := range map[string]krt.Collection[*routeAttachment]{
		"http/": s.httpAttachments, "model/": s.modelAttachments, "mcp/": s.mcpAttachments,
	} {
		for _, route := range collection.List() {
			key := prefix + route.ResourceName()
			version := versions[key]
			if version == (compiledVersion{}) {
				return errCompilationPending
			}

			parents := version.parentRefs()
			if parents.deleting != route.route.deleting || !slices.Equal(parents.parents, route.route.parents) ||
				!parents.attachmentsCurrent(route.entries, byGateway) {
				return errCompilationPending
			}

			seen[key] = true
			for _, entry := range route.entries {
				if attached[entry] == nil {
					attached[entry] = make(map[string]compiledVersion)
				}

				attached[entry][key] = version
			}
		}
	}

	if len(seen) != len(versions) {
		return errCompilationPending
	}

	for _, table := range p.tables {
		key := table.ResourceName()
		expected := expectedEntries[key]
		if expected == nil || expected.listener != table.entry.listener || expected.index != table.entry.index ||
			compareMeta(expected.origin, table.entry.origin) != 0 {
			return errCompilationPending
		}

		if len(table.versions) != len(attached[key]) {
			return errCompilationPending
		}

		for route, version := range table.versions {
			if version != attached[key][route] {
				return errCompilationPending
			}
		}
	}

	for _, status := range p.statuses {
		if versions["http/"+status.route.ResourceName()].http != status.route {
			return errCompilationPending
		}

		parents := (compiledVersion{http: status.route}).parentRefs()
		if !parents.entriesCurrent(status.entries, byGateway) {
			return errCompilationPending
		}
	}

	return nil
}

func (p publication) httpConfig() (dataplane.Config, error) {
	if p.err != nil {
		return dataplane.Config{}, p.err
	}

	var config dataplane.Config
	listeners := make(map[int32]*dataplane.Listener)
	used := make(map[string]bool)
	for _, table := range p.tables {
		port := table.entry.listener.Port
		if listeners[port] == nil {
			listeners[port] = &dataplane.Listener{Port: port}
		}

		host := table.http
		host.Routes = slices.Clone(host.Routes)
		listeners[port].Hosts = append(listeners[port].Hosts, host)
		for _, name := range table.httpNetworks {
			used[name] = true
		}
	}

	for _, port := range slices.Sorted(maps.Keys(listeners)) {
		config.Listeners = append(config.Listeners, *listeners[port])
	}

	for _, name := range slices.Sorted(maps.Keys(used)) {
		backend := p.networks[name]
		if backend == nil {
			return dataplane.Config{}, errCompilationPending
		}

		config.Backends = append(config.Backends, backend.httpBackend())
	}

	return config, nil
}

func (p publication) aiConfig() (aidataplane.Config, error) {
	if p.err != nil {
		return aidataplane.Config{}, p.err
	}

	config := aidataplane.Config{
		FormatVersion:   aidataplane.FormatVersionV1,
		Listeners:       []aidataplane.Listener{},
		Backends:        []aidataplane.Backend{},
		NetworkBackends: []aidataplane.NetworkBackend{},
	}
	listeners := make(map[int32]*aidataplane.Listener)
	protocols := make(map[string]aidataplane.Backend)
	used := make(map[string]bool)
	for _, table := range p.tables {
		if table.aiErr != nil {
			return aidataplane.Config{}, table.aiErr
		}

		if table.ai.Name == "" {
			continue
		}

		port := table.entry.listener.Port
		if listeners[port] == nil {
			listeners[port] = &aidataplane.Listener{Port: port, Hosts: []aidataplane.Host{}}
		}

		listeners[port].Hosts = append(listeners[port].Hosts, table.aiHost())
		for _, backend := range table.protocols {
			backend.ModelNameOverride = new(*backend.ModelNameOverride)
			protocols[backend.Name] = backend
		}

		for _, name := range table.aiNetworks {
			used[name] = true
		}
	}

	for _, port := range slices.Sorted(maps.Keys(listeners)) {
		config.Listeners = append(config.Listeners, *listeners[port])
	}

	for _, name := range slices.Sorted(maps.Keys(protocols)) {
		config.Backends = append(config.Backends, protocols[name])
	}

	for _, name := range slices.Sorted(maps.Keys(used)) {
		backend := p.networks[name]
		if backend == nil || backend.deleting {
			return aidataplane.Config{}, errCompilationPending
		}

		config.NetworkBackends = append(config.NetworkBackends, backend.aiBackend())
	}

	if err := aidataplane.Validate(config); err != nil {
		return aidataplane.Config{}, fmt.Errorf("validate compiled AI configuration: %w", err)
	}

	return config, nil
}

func (t *entryTable) aiHost() aidataplane.Host {
	host := t.ai
	host.Hostname = new(*host.Hostname)
	host.ModelRoutes = slices.Clone(host.ModelRoutes)
	for i := range host.ModelRoutes {
		route := &host.ModelRoutes[i]
		route.Headers = slices.Clone(route.Headers)
		route.BackendName = new(*route.BackendName)
	}

	host.MCPRoutes = slices.Clone(host.MCPRoutes)
	for i := range host.MCPRoutes {
		host.MCPRoutes[i].BackendName = new(*host.MCPRoutes[i].BackendName)
	}

	return host
}
